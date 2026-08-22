package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// forwardCNAdaptiveAnthropic 将自适应账号的 Messages 请求原样发送到供应商原生端点。
// 该路径保留 Anthropic SSE/JSON 响应格式，避免经过 Responses 中间格式丢失协议字段。
func (s *OpenAIGatewayService) forwardCNAdaptiveAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	baseURL := account.GetCNProtocolBaseURL(APIProtocolAnthropic)
	if baseURL == "" {
		return nil, fmt.Errorf("adaptive Anthropic endpoint is not configured")
	}
	normalized, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid adaptive Anthropic base URL: %w", err)
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if mapped := strings.TrimSpace(account.GetMappedModel(model)); mapped != "" {
		model = mapped
	}
	if defaultMappedModel != "" && model == "" {
		model = defaultMappedModel
	}
	if model != "" {
		if rewritten, rewriteErr := sjson.SetBytes(body, "model", model); rewriteErr == nil {
			body = rewritten
		}
	}
	apiURL := strings.TrimRight(normalized, "/") + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	setAnthropicAPIKeyAuthHeader(req.Header, account, account.GetOpenAIProtocolAPIKey())
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.resolveOpenAITLSProfile(account))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode >= http.StatusBadRequest {
		c.Data(resp.StatusCode, "application/json", responseBody)
		return nil, fmt.Errorf("adaptive Anthropic endpoint returned %d", resp.StatusCode)
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		_, _ = c.Writer.Write(responseBody)
		c.Writer.Flush()
	} else {
		c.Data(http.StatusOK, "application/json", responseBody)
	}
	return &OpenAIForwardResult{
		Model: model, BillingModel: model, UpstreamModel: model,
		UpstreamEndpoint: "/v1/messages", ResponseHeaders: resp.Header,
		ResponseBody: responseBody, Stream: strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream"),
		Duration: time.Since(startTime),
	}, nil
}
