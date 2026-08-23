package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/singleflight"
)

const (
	cnQuotaUpstreamTimeout = 15 * time.Second
	cnQuotaMaxBodyBytes    = 256 * 1024
	cnExtraSuffix5hUsed    = "5h_used_percent"
	cnExtraSuffix5hReset   = "5h_reset_at"
	cnExtraSuffixWeekUsed  = "weekly_used_percent"
	cnExtraSuffixWeekReset = "weekly_reset_at"
	cnExtraSuffixUpdated   = "usage_updated_at"
)

func cnExtraKey(provider, suffix string) string { return provider + "_" + suffix }

// CNQuotaTier 表示 Coding Plan 的一个滚动用量窗口。
type CNQuotaTier struct {
	Window      string  `json:"window"`
	UsedPercent float64 `json:"used_percent"`
	ResetAt     string  `json:"reset_at,omitempty"`
}

// CNProviderQuotaProbeResult 是管理端额度探测结果。
type CNProviderQuotaProbeResult struct {
	Provider        string        `json:"provider"`
	Source          string        `json:"source"`
	Success         bool          `json:"success"`
	CredentialValid bool          `json:"credential_valid"`
	Tiers           []CNQuotaTier `json:"tiers,omitempty"`
	PlanLevel       string        `json:"plan_level,omitempty"`
	StatusCode      int           `json:"status_code,omitempty"`
	FetchedAt       int64         `json:"fetched_at"`
	Persisted       bool          `json:"persisted"`
	Error           string        `json:"error,omitempty"`
}

// CNProviderQuotaService 查询 Kimi/智谱 Coding Plan 的滚动窗口额度。
type CNProviderQuotaService struct {
	accountRepo  AccountRepository
	proxyRepo    ProxyRepository
	httpUpstream HTTPUpstream
	cfg          *config.Config
	flight       singleflight.Group
}

// NewCNProviderQuotaService 创建国产供应商额度探测服务。
func NewCNProviderQuotaService(accountRepo AccountRepository, proxyRepo ProxyRepository, httpUpstream HTTPUpstream, cfg *config.Config) *CNProviderQuotaService {
	return &CNProviderQuotaService{accountRepo: accountRepo, proxyRepo: proxyRepo, httpUpstream: httpUpstream, cfg: cfg}
}

// QueryUsage 查询额度并将成功快照保存到账号 Extra。
func (s *CNProviderQuotaService) QueryUsage(ctx context.Context, accountID int64) (*CNProviderQuotaProbeResult, error) {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "CN_QUOTA_NOT_CONFIGURED", "cn provider quota service is not configured")
	}
	resultCh := s.flight.DoChan("cn_quota:"+strconv.FormatInt(accountID, 10), func() (any, error) {
		probeCtx, cancel := context.WithTimeout(context.Background(), cnQuotaUpstreamTimeout+5*time.Second)
		defer cancel()
		return s.queryUsage(probeCtx, accountID)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		probe, ok := result.Val.(*CNProviderQuotaProbeResult)
		if !ok || probe == nil {
			return nil, infraerrors.New(http.StatusInternalServerError, "CN_QUOTA_PROBE_RESULT_INVALID", "invalid cn provider quota probe result")
		}
		clone := *probe
		return &clone, nil
	}
}

func (s *CNProviderQuotaService) queryUsage(ctx context.Context, accountID int64) (*CNProviderQuotaProbeResult, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, infraerrors.Newf(http.StatusNotFound, "CN_QUOTA_ACCOUNT_NOT_FOUND", "account not found: %v", err)
	}
	if !account.IsCNProvider() || !account.IsCodingPlan() {
		return nil, infraerrors.New(http.StatusBadRequest, "CN_QUOTA_NOT_CODING_PLAN", "account is not a CN provider coding plan account")
	}
	provider := account.GetCodingPlanProvider()
	if provider != PlatformKimi && provider != PlatformZhipu {
		return nil, infraerrors.New(http.StatusBadRequest, "CN_QUOTA_UNSUPPORTED_PROVIDER", "account is not a kimi/zhipu coding plan account")
	}
	apiKey := strings.TrimSpace(account.GetCNAPIKey())
	if apiKey == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "CN_QUOTA_NO_APIKEY", "account api_key is empty")
	}
	baseURL := account.GetOpenAIBaseURL()
	targetURL := kimiQuotaURL(baseURL)
	authHeader := "Bearer " + apiKey
	if provider == PlatformZhipu {
		targetURL = zhipuQuotaURL(baseURL)
		authHeader = apiKey
	}
	validatedURL, err := cnValidateProbeURL(s.cfg, targetURL)
	if err != nil {
		return nil, infraerrors.New(http.StatusForbidden, "CN_QUOTA_URL_REJECTED", err.Error())
	}
	callCtx, cancel := context.WithTimeout(ctx, cnQuotaUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "CN_QUOTA_REQUEST_BUILD_FAILED", "build request: %v", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	if provider == PlatformZhipu {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Language", "en-US,en")
	}
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
		return nil, infraerrors.Newf(http.StatusBadGateway, "CN_QUOTA_REQUEST_FAILED", "upstream request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Debug("cn_quota_response_close_failed", "account_id", account.ID, "provider", provider, "error", closeErr)
		}
	}()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, cnQuotaMaxBodyBytes))
	now := time.Now().UTC()
	result := &CNProviderQuotaProbeResult{Provider: provider, Source: "coding_plan", FetchedAt: now.Unix(), StatusCode: resp.StatusCode}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		result.Error = fmt.Sprintf("Authentication failed (HTTP %d)", resp.StatusCode)
		return result, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("API error (HTTP %d): %s", resp.StatusCode, truncate(strings.TrimSpace(string(body)), 240))
		return result, nil
	}
	if provider == PlatformZhipu && gjson.GetBytes(body, "success").Exists() && !gjson.GetBytes(body, "success").Bool() {
		result.Error = "API error: " + strings.TrimSpace(gjson.GetBytes(body, "msg").String())
		if result.Error == "API error: " {
			result.Error = "API error: unknown zhipu quota error"
		}
		return result, nil
	}
	if provider == PlatformKimi {
		result.Tiers = parseKimiUsageTiers(body)
	} else {
		result.Tiers = parseZhipuTokenTiers(gjson.GetBytes(body, "data"))
		result.PlanLevel = strings.TrimSpace(gjson.GetBytes(body, "data.level").String())
	}
	result.Success = true
	result.CredentialValid = true
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, cnQuotaExtraUpdates(provider, result.Tiers, now)); err != nil {
		slog.Warn("cn_quota_persist_failed", "account_id", account.ID, "provider", provider, "error", err)
	} else {
		result.Persisted = true
	}
	return result, nil
}

func kimiQuotaURL(baseURL string) string {
	base := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
	return base + "/v1/usages"
}

func zhipuQuotaURL(baseURL string) string {
	return zhipuQuotaHost(baseURL) + "/api/monitor/usage/quota/limit"
}

func zhipuQuotaHost(baseURL string) string {
	if strings.Contains(strings.ToLower(baseURL), "z.ai") {
		return "https://api.z.ai"
	}
	return "https://open.bigmodel.cn"
}

func parseKimiUsageTiers(body []byte) []CNQuotaTier {
	var tiers []CNQuotaTier
	if limits := gjson.GetBytes(body, "limits"); limits.IsArray() {
		limits.ForEach(func(_, item gjson.Result) bool {
			detail := item.Get("detail")
			if !detail.Exists() {
				return true
			}
			limit, _ := cnParseF64(detail.Get("limit").Value())
			remaining, _ := cnParseF64(detail.Get("remaining").Value())
			tiers = append(tiers, CNQuotaTier{Window: "5h", UsedPercent: percentUsed(limit, remaining), ResetAt: cnNormalizeResetTime(detail.Get("resetTime").Value())})
			return false
		})
	}
	if usage := gjson.GetBytes(body, "usage"); usage.Exists() {
		limit, _ := cnParseF64(usage.Get("limit").Value())
		remaining, _ := cnParseF64(usage.Get("remaining").Value())
		tiers = append(tiers, CNQuotaTier{Window: "weekly", UsedPercent: percentUsed(limit, remaining), ResetAt: cnNormalizeResetTime(usage.Get("resetTime").Value())})
	}
	return tiers
}

func percentUsed(limit, remaining float64) float64 {
	if limit <= 0 {
		return 0
	}
	used := limit - remaining
	if used < 0 {
		used = 0
	}
	return used / limit * 100
}

func parseZhipuTokenTiers(data gjson.Result) []CNQuotaTier {
	type entry struct {
		unit       int64
		reset      int64
		hasReset   bool
		percentage float64
	}
	var token, credit []entry
	data.Get("limits").ForEach(func(_, item gjson.Result) bool {
		kind := strings.ToUpper(strings.TrimSpace(item.Get("type").String()))
		percentage, _ := cnParseF64(item.Get("percentage").Value())
		rawReset := item.Get("nextResetTime")
		e := entry{unit: item.Get("unit").Int(), percentage: percentage}
		if rawReset.Exists() {
			switch rawReset.Type {
			case gjson.Number:
				e.reset = rawReset.Int()
			case gjson.String:
				if parsed, parseErr := parseSchedulingTime(rawReset.String()); parseErr == nil {
					e.reset = parsed.UnixMilli()
				}
			}
			e.hasReset = e.reset > 0
		}
		switch kind {
		case "TOKENS_LIMIT":
			token = append(token, e)
		case "CREDIT_LIMIT":
			credit = append(credit, e)
		}
		return true
	})
	if len(token) == 0 {
		token = credit
	}
	var five, week *entry
	for i := range token {
		e := &token[i]
		switch e.unit {
		case 3:
			if five == nil {
				five = e
			}
		case 6:
			if week == nil {
				week = e
			}
		}
	}
	unclassified := make([]entry, 0, len(token))
	for _, e := range token {
		if (e.unit != 3 || five == nil) && (e.unit != 6 || week == nil) {
			unclassified = append(unclassified, e)
		}
	}
	sort.SliceStable(unclassified, func(i, j int) bool {
		if unclassified[i].hasReset != unclassified[j].hasReset {
			return !unclassified[i].hasReset
		}
		return unclassified[i].reset < unclassified[j].reset
	})
	for i := range unclassified {
		if five == nil {
			five = &unclassified[i]
		} else if week == nil {
			week = &unclassified[i]
		}
	}
	result := make([]CNQuotaTier, 0, 2)
	if five != nil {
		result = append(result, CNQuotaTier{Window: "5h", UsedPercent: five.percentage, ResetAt: cnMillisToRFC3339(five.reset)})
	}
	if week != nil {
		result = append(result, CNQuotaTier{Window: "weekly", UsedPercent: week.percentage, ResetAt: cnMillisToRFC3339(week.reset)})
	}
	return result
}

func cnQuotaExtraUpdates(provider string, tiers []CNQuotaTier, now time.Time) map[string]any {
	updates := map[string]any{cnExtraKey(provider, cnExtraSuffixUpdated): now.Format(time.RFC3339)}
	for _, tier := range tiers {
		switch tier.Window {
		case "5h":
			updates[cnExtraKey(provider, cnExtraSuffix5hUsed)] = tier.UsedPercent
			updates[cnExtraKey(provider, cnExtraSuffix5hReset)] = tier.ResetAt
		case "weekly":
			updates[cnExtraKey(provider, cnExtraSuffixWeekUsed)] = tier.UsedPercent
			updates[cnExtraKey(provider, cnExtraSuffixWeekReset)] = tier.ResetAt
		}
	}
	return updates
}

func cnParseF64(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func cnNormalizeResetTime(raw any) string {
	switch value := raw.(type) {
	case string:
		parsed, err := parseSchedulingTime(strings.TrimSpace(value))
		if err != nil {
			return ""
		}
		return parsed.UTC().Format(time.RFC3339)
	case float64:
		return cnMillisToRFC3339(int64(value))
	case int64:
		return cnMillisToRFC3339(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return cnMillisToRFC3339(parsed)
		}
	}
	return ""
}

func cnMillisToRFC3339(value int64) string {
	if value <= 0 {
		return ""
	}
	if value < 1_000_000_000_000 {
		value *= 1000
	}
	return time.UnixMilli(value).UTC().Format(time.RFC3339)
}
