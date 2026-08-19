package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/singleflight"
)

const (
	cnBalanceUpstreamTimeout = 15 * time.Second
	cnBalanceMaxBodyBytes    = 256 * 1024
	cnBalanceSuffixBalance   = "balance"
	cnBalanceSuffixCurrency  = "balance_currency"
	cnBalanceSuffixAvailable = "balance_available"
	cnBalanceSuffixUpdated   = "balance_updated_at"
	cnBalanceSuffixBalances  = "balances"
)

// CNProviderBalanceEntry 表示一种币种的余额。
type CNProviderBalanceEntry struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
}

// CNProviderBalanceResult 是管理端余额探测结果。
type CNProviderBalanceResult struct {
	Provider   string                   `json:"provider"`
	Success    bool                     `json:"success"`
	Balance    float64                  `json:"balance"`
	Currency   string                   `json:"currency,omitempty"`
	Balances   []CNProviderBalanceEntry `json:"balances,omitempty"`
	Available  bool                     `json:"available"`
	StatusCode int                      `json:"status_code,omitempty"`
	FetchedAt  int64                    `json:"fetched_at"`
	Persisted  bool                     `json:"persisted"`
	Error      string                   `json:"error,omitempty"`
}

// CNProviderBalanceService 查询 Kimi/DeepSeek 按量账号余额。
type CNProviderBalanceService struct {
	accountRepo  AccountRepository
	proxyRepo    ProxyRepository
	httpUpstream HTTPUpstream
	cfg          *config.Config
	flight       singleflight.Group
}

// NewCNProviderBalanceService 创建国产供应商余额探测服务。
func NewCNProviderBalanceService(accountRepo AccountRepository, proxyRepo ProxyRepository, httpUpstream HTTPUpstream, cfg *config.Config) *CNProviderBalanceService {
	return &CNProviderBalanceService{accountRepo: accountRepo, proxyRepo: proxyRepo, httpUpstream: httpUpstream, cfg: cfg}
}

// QueryBalance 查询余额并保存成功快照。
func (s *CNProviderBalanceService) QueryBalance(ctx context.Context, accountID int64) (*CNProviderBalanceResult, error) {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "CN_BALANCE_NOT_CONFIGURED", "cn provider balance service is not configured")
	}
	resultCh := s.flight.DoChan("cn_balance:"+strconv.FormatInt(accountID, 10), func() (any, error) {
		probeCtx, cancel := context.WithTimeout(context.Background(), cnBalanceUpstreamTimeout+5*time.Second)
		defer cancel()
		return s.queryBalance(probeCtx, accountID)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		probe, ok := result.Val.(*CNProviderBalanceResult)
		if !ok || probe == nil {
			return nil, infraerrors.New(http.StatusInternalServerError, "CN_BALANCE_PROBE_RESULT_INVALID", "invalid cn provider balance probe result")
		}
		clone := *probe
		return &clone, nil
	}
}

func (s *CNProviderBalanceService) queryBalance(ctx context.Context, accountID int64) (*CNProviderBalanceResult, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, infraerrors.Newf(http.StatusNotFound, "CN_BALANCE_ACCOUNT_NOT_FOUND", "account not found: %v", err)
	}
	if !account.IsCNProvider() || account.IsCodingPlan() {
		return nil, infraerrors.New(http.StatusBadRequest, "CN_BALANCE_INVALID_ACCOUNT", "account is not a payg CN provider account")
	}
	provider := account.Platform
	if provider != PlatformKimi && provider != PlatformDeepseek {
		return nil, infraerrors.New(http.StatusBadRequest, "CN_BALANCE_NO_ENDPOINT", "account provider has no balance endpoint")
	}
	apiKey := strings.TrimSpace(account.GetCNAPIKey())
	if apiKey == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "CN_BALANCE_NO_APIKEY", "account api_key is empty")
	}
	validatedURL, err := cnValidateProbeURL(s.cfg, cnBalanceURL(account))
	if err != nil {
		return nil, infraerrors.New(http.StatusForbidden, "CN_BALANCE_URL_REJECTED", err.Error())
	}
	callCtx, cancel := context.WithTimeout(ctx, cnBalanceUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "CN_BALANCE_REQUEST_BUILD_FAILED", "build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	} else if account.ProxyID != nil && s.proxyRepo != nil {
		if proxy, proxyErr := s.proxyRepo.GetByID(ctx, *account.ProxyID); proxyErr == nil && proxy != nil {
			account.Proxy = proxy
			proxyURL = proxy.URL()
		}
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, maxInt(account.Concurrency, 1))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "CN_BALANCE_REQUEST_FAILED", "upstream request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Debug("cn_balance_response_close_failed", "account_id", account.ID, "provider", provider, "error", closeErr)
		}
	}()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, cnBalanceMaxBodyBytes))
	now := time.Now().UTC()
	result := &CNProviderBalanceResult{Provider: provider, FetchedAt: now.Unix(), StatusCode: resp.StatusCode, Available: true}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		result.Error = fmt.Sprintf("Authentication failed (HTTP %d)", resp.StatusCode)
		return result, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("API error (HTTP %d): %s", resp.StatusCode, truncate(strings.TrimSpace(string(body)), 240))
		return result, nil
	}
	entries := make([]CNProviderBalanceEntry, 0, 2)
	available := true
	if provider == PlatformKimi {
		balance, _ := cnParseF64(gjson.GetBytes(body, "data.available_balance").Value())
		entries = append(entries, CNProviderBalanceEntry{Currency: "CNY", Balance: balance})
	} else {
		if value := gjson.GetBytes(body, "is_available"); value.Exists() {
			available = value.Bool()
		}
		gjson.GetBytes(body, "balance_infos").ForEach(func(_, item gjson.Result) bool {
			currency := strings.ToUpper(strings.TrimSpace(item.Get("currency").String()))
			if currency == "" {
				currency = "CNY"
			}
			balance, _ := cnParseF64(item.Get("total_balance").Value())
			entries = append(entries, CNProviderBalanceEntry{Currency: currency, Balance: balance})
			return true
		})
		if len(entries) == 0 {
			entries = append(entries, CNProviderBalanceEntry{Currency: "CNY"})
		}
	}
	result.Balances = entries
	result.Balance = entries[0].Balance
	result.Currency = entries[0].Currency
	result.Available = available
	result.Success = true
	details := make([]any, 0, len(entries))
	for _, entry := range entries {
		details = append(details, map[string]any{"currency": entry.Currency, "balance": entry.Balance})
	}
	updates := map[string]any{
		cnExtraKey(provider, cnBalanceSuffixBalance):   result.Balance,
		cnExtraKey(provider, cnBalanceSuffixCurrency):  result.Currency,
		cnExtraKey(provider, cnBalanceSuffixAvailable): available,
		cnExtraKey(provider, cnBalanceSuffixUpdated):   now.Format(time.RFC3339),
		cnExtraKey(provider, cnBalanceSuffixBalances):  details,
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		slog.Warn("cn_balance_persist_failed", "account_id", account.ID, "provider", provider, "error", err)
	} else {
		result.Persisted = true
	}
	return result, nil
}

func cnBalanceURL(account *Account) string {
	switch account.Platform {
	case PlatformKimi:
		return "https://api.moonshot.cn/v1/users/me/balance"
	case PlatformDeepseek:
		return strings.TrimRight(account.GetOpenAIFormatBaseURL(), "/") + "/user/balance"
	default:
		return ""
	}
}
