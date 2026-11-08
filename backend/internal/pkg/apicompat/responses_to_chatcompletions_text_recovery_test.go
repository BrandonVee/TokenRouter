// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupplementResponseOutput_RecoversTextWhenTerminalMessageIsEmpty(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "Hello, "})
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "world"})

	resp := &ResponsesResponse{
		ID:     "resp_empty_msg",
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:    "message",
			Role:    "assistant",
			Content: []ResponsesContentPart{},
		}},
		Usage: &ResponsesUsage{InputTokens: 13168, OutputTokens: 3329},
	}

	acc.SupplementResponseOutput(resp)

	chat := ResponsesToChatCompletions(resp, "gpt-5.5")
	require.Len(t, chat.Choices, 1)
	require.NotNil(t, chat.Choices[0].Message.Content, "the client must not receive an empty reply")

	var got string
	require.NoError(t, json.Unmarshal(chat.Choices[0].Message.Content, &got))
	assert.Equal(t, "Hello, world", got)
}

func TestSupplementResponseOutput_RecoversTextWhenTerminalTextIsBlank(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "real text"})

	resp := &ResponsesResponse{
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:    "message",
			Content: []ResponsesContentPart{{Type: "output_text", Text: "   "}},
		}},
	}

	acc.SupplementResponseOutput(resp)

	chat := ResponsesToChatCompletions(resp, "m")
	var got string
	require.NotNil(t, chat.Choices[0].Message.Content)
	require.NoError(t, json.Unmarshal(chat.Choices[0].Message.Content, &got))
	assert.Equal(t, "real text", got)
}

func TestSupplementResponseOutput_AppendsMessageWhenTerminalHasNoMessage(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "only in the stream"})

	resp := &ResponsesResponse{
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "verify",
		}},
	}

	acc.SupplementResponseOutput(resp)

	var text string
	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}
		for _, p := range item.Content {
			text += p.Text
		}
	}
	assert.Equal(t, "only in the stream", text)

	require.Len(t, resp.Output, 2)
	assert.Equal(t, "function_call", resp.Output[0].Type)
	assert.Equal(t, "verify", resp.Output[0].Name)
}

func TestSupplementResponseOutput_KeepsTerminalTextAuthoritative(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "from the stream"})

	resp := &ResponsesResponse{
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:    "message",
			Content: []ResponsesContentPart{{Type: "output_text", Text: "from the terminal event"}},
		}},
	}

	acc.SupplementResponseOutput(resp)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, "from the terminal event", resp.Output[0].Content[0].Text)
}

func TestSupplementResponseOutput_NoSyntheticMessageWithoutStreamText(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	resp := &ResponsesResponse{
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:    "message",
			Content: []ResponsesContentPart{},
		}},
	}

	acc.SupplementResponseOutput(resp)

	require.Len(t, resp.Output, 1)
	assert.Empty(t, resp.Output[0].Content)
}
