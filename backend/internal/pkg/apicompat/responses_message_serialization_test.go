// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func marshalMessageWire(t *testing.T, value any) gjson.Result {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return gjson.ParseBytes(raw)
}

// 完整响应、流事件重建与历史回放都必须保留密封调用元数据，不把它变成可见工具参数。
func TestFunctionCallEncryptedArgumentsSurviveWireAndReplay(t *testing.T) {
	for _, marker := range []string{`true`, `"opaque-arguments"`, `{"format":"opaque"}`} {
		item := ResponsesOutput{Type: "function_call", ID: "fc_1", CallID: "call_1", Name: "delegate", Arguments: "sealed", EncryptedFunctionArgs: json.RawMessage(marker)}
		for _, value := range []any{item, ResponsesStreamEvent{Type: "response.output_item.done", Item: &item}} {
			body, err := json.Marshal(value)
			require.NoError(t, err)
			path := "encrypted_function_args"
			if _, event := value.(ResponsesStreamEvent); event {
				path = "item." + path
			}
			require.JSONEq(t, marker, gjson.GetBytes(body, path).Raw)
		}
		body, err := json.Marshal(item)
		require.NoError(t, err)
		var replay ResponsesInputItem
		require.NoError(t, json.Unmarshal(body, &replay))
		require.JSONEq(t, marker, string(replay.EncryptedFunctionArgs))
		require.Equal(t, "sealed", replay.Arguments)
	}
}

func TestMessageContentWireConsistent(t *testing.T) {
	for _, text := range []string{"", "search result"} {
		t.Run(text, func(t *testing.T) {
			part := ResponsesContentPart{Type: "output_text", Text: text}
			item := ResponsesOutput{Type: "message", ID: "msg_1", Role: "assistant", Status: "completed", Content: []ResponsesContentPart{part}}
			response := ResponsesResponse{ID: "resp_1", Object: "response", Status: "completed", Output: []ResponsesOutput{item}}
			parts := []gjson.Result{
				marshalMessageWire(t, ResponsesStreamEvent{Type: "response.content_part.done", Part: &part}).Get("part"),
				marshalMessageWire(t, ResponsesStreamEvent{Type: "response.output_item.done", Item: &item}).Get("item.content.0"),
				marshalMessageWire(t, ResponsesStreamEvent{Type: "response.completed", Response: &response}).Get("response.output.0.content.0"),
				marshalMessageWire(t, response).Get("output.0.content.0"),
			}
			for _, encoded := range parts {
				require.Equal(t, "output_text", encoded.Get("type").String())
				require.True(t, encoded.Get("text").Exists())
				require.Equal(t, text, encoded.Get("text").String())
				require.Equal(t, "[]", encoded.Get("annotations").Raw)
				require.Equal(t, "[]", encoded.Get("logprobs").Raw)
			}
		})
	}
}

func TestMessageRefusalPreserved(t *testing.T) {
	item := ResponsesOutput{Type: "message", ID: "msg_1", Role: "assistant", Content: []ResponsesContentPart{{Type: "refusal", Refusal: "Cannot provide this."}}}
	for _, encoded := range []gjson.Result{
		marshalMessageWire(t, item),
		marshalMessageWire(t, ResponsesStreamEvent{Type: "response.output_item.done", Item: &item}).Get("item"),
	} {
		require.Equal(t, "refusal", encoded.Get("content.0.type").String())
		require.Equal(t, "Cannot provide this.", encoded.Get("content.0.refusal").String())
		require.False(t, encoded.Get("content.0.text").Exists())
	}
}

func TestEmptyMessageContentArray(t *testing.T) {
	item := marshalMessageWire(t, ResponsesOutput{Type: "message", ID: "msg_1", Role: "assistant"})
	require.Equal(t, "[]", item.Get("content").Raw)
}
