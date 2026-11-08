//go:build unit

package service

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

// 原生桥接不能把完整但缺少账单用量的 DeepSeek 流声明为成功。
func TestDeepseekNativeBridgesRequireBillableUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ingress := range routingMatrixIngresses() {
		if ingress.name == "chat_responses_shape" || ingress.name == "messages" {
			continue
		}
		for _, stream := range []bool{false, true} {
			for _, billable := range []bool{false, true} {
				t.Run(ingress.name+"/"+boolLabel(stream)+"/"+boolLabel(billable), func(t *testing.T) {
					resp := nativeAnthropicStreamResponse()
					if !billable {
						data, err := io.ReadAll(resp.Body)
						require.NoError(t, err)
						body := strings.NewReplacer(`"input_tokens":93`, `"input_tokens":0`, `"output_tokens":16`, `"output_tokens":0`, `"output_tokens":1`, `"output_tokens":0`).Replace(string(data))
						resp.Body = io.NopCloser(strings.NewReader(body))
					}
					account := nativeAnthropicTestAccount()
					account.Platform = PlatformDeepseek
					body := routingMatrixCase{ingress: ingress, model: "mapped-model"}.body()
					body, err := sjson.SetBytes(body, "stream", stream)
					require.NoError(t, err)
					w := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(w)
					c.Request = httptest.NewRequest("POST", ingress.path, strings.NewReader(string(body))).WithContext(context.Background())
					svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: &httpUpstreamRecorder{resp: resp}}
					err = ingress.forward(svc, c, account, body)
					if billable {
						require.NoError(t, err)
						return
					}
					var failure *UpstreamFailoverError
					require.ErrorAs(t, err, &failure)
					require.Contains(t, string(failure.ResponseBody), deepseekMissingUsageErrorCode)
					require.NotContains(t, w.Body.String(), "response.completed")
					require.NotContains(t, w.Body.String(), "data: [DONE]")
				})
			}
		}
	}
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
