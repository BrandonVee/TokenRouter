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
	// 与其它 OpenAI 兼容转发路径一致，客户端断开后仍读取上游最终用量用于结算。
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, apiURL, bytes.NewReader(body))
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
	isStream := isEventStreamResponse(resp.Header)
	usage := parseCNAdaptiveAnthropicUsage(responseBody, isStream)
	if err := validateDeepseekUsage(c, account, resp.Header.Get("x-request-id"), usage, model); err != nil {
		return nil, err
	}
	if isStream {
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
		RequestID: resp.Header.Get("x-request-id"), Usage: usage,
		Model: model, BillingModel: model, UpstreamModel: model,
		UpstreamEndpoint: "/v1/messages", ResponseHeaders: resp.Header,
		ResponseBody: responseBody, Stream: isStream,
		Duration: time.Since(startTime),
	}, nil
}

// parseCNAdaptiveAnthropicUsage 解析 JSON 或 SSE 的原生 Messages 用量并转换为统一结算口径。
// @project-doc docs/interfaces/upstream_account_matrix.md#cn_adaptive_usage
func parseCNAdaptiveAnthropicUsage(body []byte, stream bool) OpenAIUsage {
	usage := &ClaudeUsage{}
	if stream {
		// message_start 给出输入用量，message_delta 给出累计输出；不能用末尾事件覆盖整份用量。
		for line := range bytes.SplitSeq(body, []byte("\n")) {
			if payload, ok := extractAnthropicSSEDataLine(string(line)); ok {
				parseClaudeUsageFromSSEEvent(payload, usage)
			}
		}
	} else {
		usage = parseClaudeUsageFromResponseBody(body)
	}
	// Anthropic 输入不包含缓存桶，而 RecordUsage 会从 OpenAI 总输入中扣除缓存桶，
	// 因此必须先合并总输入，避免普通输入被重复扣减而少计费。
	return OpenAIUsage{
		InputTokens:              usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
	}
}
