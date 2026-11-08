//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 从三个实际转发入口验证累计用量，防止只修复账本而让协议转换仍然把用量清零。
func TestDeepseekChatStreamPreservesUsageAcrossProtocols(t *testing.T) {
	const full = `{"prompt_tokens":100,"completion_tokens":12,"prompt_tokens_details":{"cached_tokens":80,"cache_write_tokens":5}}`
	const initial = `{"prompt_tokens":100,"completion_tokens":4,"prompt_tokens_details":{"cached_tokens":80,"cache_write_tokens":5}}`
	tests := []struct {
		name      string
		snapshots []string
	}{
		{name: "repeated cumulative snapshot", snapshots: []string{`"usage":` + full, `"usage":` + full}},
		{name: "empty final snapshot", snapshots: []string{`"usage":` + full, `"usage":{}`}},
		{name: "zero final snapshot", snapshots: []string{`"usage":` + full, `"usage":{"prompt_tokens":0,"completion_tokens":0}`}},
		{name: "partial output snapshot", snapshots: []string{`"usage":` + initial, `"usage":{"completion_tokens":12}`}},
		{name: "wrapped usage", snapshots: []string{`"data":{"usage":` + full + `}`}},
	}
	for _, protocol := range []string{"chat", "responses", "messages"} {
		for _, tt := range tests {
			t.Run(protocol+"/"+tt.name, func(t *testing.T) {
				upstreamBody := "data: {\"id\":\"chatcmpl_usage\",\"model\":\"deepseek-v4-pro\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}\n\n"
				for _, snapshot := range tt.snapshots {
					upstreamBody += `data: {"choices":[],` + snapshot + "}\n\n"
				}
				upstreamBody += "data: [DONE]\n\n"
				result, rec, err := forwardDeepseekUsageTest(t, protocol, true, upstreamBody)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, OpenAIUsage{
					InputTokens: 100, OutputTokens: 12, CacheReadInputTokens: 80, CacheCreationInputTokens: 5,
				}, result.Usage)
				if protocol == "chat" {
					require.Equal(t, int64(100), gjson.GetBytes(result.ResponseBody, "usage.prompt_tokens").Int())
					require.Equal(t, int64(12), gjson.GetBytes(result.ResponseBody, "usage.completion_tokens").Int())
					return
				}
				var terminalUsage gjson.Result
				for _, line := range strings.Split(rec.Body.String(), "\n") {
					payload, ok := extractOpenAISSEDataLine(line)
					if !ok {
						continue
					}
					event := gjson.Parse(payload)
					if event.Get("type").String() == "response.completed" {
						terminalUsage = event.Get("response.usage")
					}
					if event.Get("type").String() == "message_delta" {
						terminalUsage = event.Get("usage")
					}
				}
				require.True(t, terminalUsage.Exists())
				require.Equal(t, int64(12), terminalUsage.Get("output_tokens").Int())
				if protocol == "responses" {
					require.Equal(t, int64(100), terminalUsage.Get("input_tokens").Int())
					require.Equal(t, int64(80), terminalUsage.Get("input_tokens_details.cached_tokens").Int())
				} else {
					require.Equal(t, int64(15), terminalUsage.Get("input_tokens").Int())
					require.Equal(t, int64(80), terminalUsage.Get("cache_read_input_tokens").Int())
				}
			})
		}
	}
}

// 缺失用量必须在正常结束标记之前暴露错误，不能继续生成正常的零费用成功记录。
func TestDeepseekMissingUsageRejectsSuccessAcrossProtocols(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "messages"} {
		for _, stream := range []bool{false, true} {
			name := protocol + "/json"
			upstreamBody := `{"id":"chatcmpl_missing","model":"deepseek-v4-pro","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`
			if stream {
				name = protocol + "/stream"
				upstreamBody = "data: {\"model\":\"deepseek-v4-pro\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n"
			}
			t.Run(name, func(t *testing.T) {
				result, rec, err := forwardDeepseekUsageTest(t, protocol, stream, upstreamBody)
				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.Nil(t, result)
				require.Equal(t, deepseekMissingUsageErrorCode, gjson.GetBytes(failoverErr.ResponseBody, "error.code").String())
				require.Equal(t, "upstream-usage-test", failoverErr.ResponseHeaders.Get("x-request-id"))
				require.NotContains(t, rec.Body.String(), "[DONE]")
				require.NotContains(t, rec.Body.String(), "response.completed")
				require.NotContains(t, rec.Body.String(), "message_stop")
				if !stream {
					require.Zero(t, rec.Body.Len())
				}
			})
		}
	}
}

func TestDeepseekChatStreamMissingUsageWithoutOutputCanFailOver(t *testing.T) {
	for _, upstreamBody := range []string{"data: [DONE]\n\n", ""} {
		result, rec, err := forwardDeepseekUsageTest(t, "chat", true, upstreamBody)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.Nil(t, result)
		require.Zero(t, rec.Body.Len())
	}
}

// 通过第三方 OpenAI 账号复现线上路径，不依赖账号平台恰好是 deepseek。
func forwardDeepseekUsageTest(t *testing.T, protocol string, stream bool, upstreamBody string) (*OpenAIForwardResult, *httptest.ResponseRecorder, error) {
	t.Helper()
	body := `{"model":"deepseek-v4-pro","messages":[{"role":"user","content":"hello"}],"max_tokens":64}`
	if protocol == "responses" {
		body = `{"model":"deepseek-v4-pro","input":"hello"}`
	}
	if stream {
		body = strings.TrimSuffix(body, "}") + `,"stream":true}`
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, bytes.NewBufferString(body))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	if stream {
		headers.Set("Content-Type", "text/event-stream")
	}
	headers.Set("x-request-id", "upstream-usage-test")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: headers, Body: io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()
	var result *OpenAIForwardResult
	var err error
	switch protocol {
	case "chat":
		result, err = svc.forwardAsRawChatCompletions(context.Background(), c, account, []byte(body), "")
	case "responses":
		result, err = svc.forwardResponsesViaRawChatCompletions(context.Background(), c, account, []byte(body))
	case "messages":
		result, err = svc.forwardAnthropicViaRawChatCompletions(context.Background(), c, account, []byte(body), "")
	default:
		t.Fatalf("unknown test protocol: %s", protocol)
	}
	return result, rec, err
}

// 原生 Messages 的普通输入与缓存输入互斥，进入统一结算前必须补成 OpenAI 总输入口径。
func TestDeepseekAdaptiveAnthropicRecordsNativeUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		name := "json"
		upstreamBody := `{"id":"msg_native","type":"message","usage":{"input_tokens":25,"output_tokens":5,"cache_read_input_tokens":10,"cache_creation_input_tokens":5}}`
		contentType := "application/json"
		if stream {
			name = "stream"
			contentType = "text/event-stream"
			upstreamBody = "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":25,\"cache_read_input_tokens\":10,\"cache_creation_input_tokens\":5}}}\r\n\r\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\r\n\r\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\r\n\r\ndata: {\"type\":\"message_stop\"}\r\n\r\n"
		}
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"model":"deepseek-v4-pro","messages":[{"role":"user","content":"hello"}]}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			headers := http.Header{}
			headers.Set("Content-Type", contentType)
			headers.Set("x-request-id", "native-request")
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK, Header: headers, Body: io.NopCloser(strings.NewReader(upstreamBody)),
			}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			account := rawChatCompletionsTestAccount()
			account.Platform = PlatformDeepseek
			account.Credentials["api_protocol"] = APIProtocolAdaptive
			account.Credentials["api_base_urls"] = map[string]any{APIProtocolAnthropic: "http://upstream.example/anthropic"}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			result, err := svc.ForwardAsAnthropic(ctx, c, account, body, "", "")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, OpenAIUsage{InputTokens: 40, OutputTokens: 5, CacheReadInputTokens: 10, CacheCreationInputTokens: 5}, result.Usage)
			require.Equal(t, "native-request", result.RequestID)
			require.NoError(t, upstream.lastReq.Context().Err())
			require.Equal(t, upstreamBody, rec.Body.String())
		})
	}
}

func TestRequiresBillableDeepseekUsageUsesFinalModelIdentity(t *testing.T) {
	require.True(t, requiresBillableDeepseekUsage(&Account{Platform: PlatformDeepseek}, "alias"))
	require.True(t, requiresBillableDeepseekUsage(&Account{Platform: PlatformOpenAI}, "provider/DeepSeek-V4-Pro"))
	require.False(t, requiresBillableDeepseekUsage(&Account{Platform: PlatformOpenAI}, "gpt-5.4"))
	require.False(t, requiresBillableDeepseekUsage(nil, "deepseekcoder"))
}
