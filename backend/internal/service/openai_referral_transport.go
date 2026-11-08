package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
)

// quotaReferralTransport 沿用配额请求的账号隔离与指纹；发送没有幂等键，任何失败都不自动重放。
type quotaReferralTransport struct{ service *OpenAIQuotaService }

func (t *quotaReferralTransport) request(ctx context.Context, call OpenAIReferralCall, email string) ([]byte, error) {
	account := call.accountContext
	if account == nil {
		return nil, infraerrors.ServiceUnavailable("OPENAI_REFERRAL_NOT_CONFIGURED", "referral transport is unavailable")
	}
	endpoint := "https://chatgpt.com/backend-api/referrals/invite"
	method := http.MethodGet
	var body []byte
	if email == "" {
		endpoint += "/eligibility?" + url.Values{"program_id": {call.ProgramID}, "entrypoint": {"persistent"}}.Encode()
	} else {
		method = http.MethodPost
		body, _ = json.Marshal(map[string]any{"program_id": call.ProgramID, "entrypoint": "persistent", "emails": []string{email}})
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if _, err = t.service.applyHeaders(req, account); err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "OPENAI_REFERRAL_AUTH_ERROR", "failed to authenticate referral request")
	}
	if email != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.service.httpUpstream.DoWithTLS(req, account.proxyURL, account.account.ID, account.account.Concurrency, account.tlsProfile)
	if err != nil {
		return nil, referralTransportUnknown(email)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 上游正文可能带个人资料或凭据，错误与日志都只使用稳定分类。
		status, reason := http.StatusBadGateway, "OPENAI_REFERRAL_UPSTREAM_ERROR"
		switch resp.StatusCode {
		case 400, 422:
			status, reason = 400, "OPENAI_REFERRAL_REJECTED"
		case 403:
			status, reason = 403, "OPENAI_REFERRAL_FORBIDDEN"
		case 409:
			status, reason = 409, "OPENAI_REFERRAL_ALREADY_EXISTS"
		case 429:
			status, reason = 429, "OPENAI_REFERRAL_RATE_LIMITED"
		case 401:
			reason = "OPENAI_REFERRAL_AUTH_ERROR"
		}
		return nil, infraerrors.New(status, reason, "invitation service request failed")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, openAIUpstreamErrorBodyReadLimit+1))
	if err != nil || int64(len(data)) > openAIUpstreamErrorBodyReadLimit {
		return nil, referralTransportUnknown(email)
	}
	return data, nil
}

func referralTransportUnknown(email string) error {
	if email != "" {
		return infraerrors.New(502, "OPENAI_REFERRAL_SEND_UNKNOWN", "invitation outcome is unknown; check Codex before sending again")
	}
	return infraerrors.New(502, "OPENAI_REFERRAL_QUERY_FAILED", "failed to query invitation eligibility")
}

func (t *quotaReferralTransport) QueryEligibility(ctx context.Context, call OpenAIReferralCall) (*OpenAIReferralEligibility, error) {
	data, err := t.request(ctx, call, "")
	if err != nil {
		return nil, err
	}
	var result *OpenAIReferralEligibility
	if json.Unmarshal(data, &result) != nil || result == nil {
		return nil, infraerrors.New(502, "OPENAI_REFERRAL_INVALID_RESPONSE", "invalid invitation eligibility response")
	}
	return result, nil
}

func (t *quotaReferralTransport) SendInvite(ctx context.Context, call OpenAIReferralCall, email string) error {
	data, err := t.request(ctx, call, email)
	if err != nil {
		return err
	}
	var result struct {
		Invites []json.RawMessage `json:"invites"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Invites) != 1 || string(result.Invites[0]) == "null" {
		return referralTransportUnknown(email)
	}
	return nil
}
