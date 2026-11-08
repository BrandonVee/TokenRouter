// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func driveToolUseThroughRestorer(
	blockInput json.RawMessage,
	partials []string,
	mapping ResponsesClientToolMapping,
) []ResponsesStreamEvent {
	state := NewAnthropicEventToResponsesState()
	restorer := NewResponsesClientToolStreamRestorer(mapping)

	var clientEvents []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		for _, converted := range AnthropicEventToResponsesEvents(evt, state) {
			clientEvents = append(clientEvents, restorer.Restore(converted)...)
		}
	}

	feed(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_chain",
			Model: "gemini-3.7-flash",
			Usage: AnthropicUsage{InputTokens: 11},
		},
	})
	feed(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "tool_use",
			ID:    "toolu_chain",
			Name:  "eval",
			Input: blockInput,
		},
	})
	for _, partial := range partials {
		feed(&AnthropicStreamEvent{
			Type:  "content_block_delta",
			Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: partial},
		})
	}
	feed(&AnthropicStreamEvent{Type: "content_block_stop"})
	feed(&AnthropicStreamEvent{Type: "message_delta", Usage: &AnthropicUsage{OutputTokens: 6}})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	return clientEvents
}

func firstEventOfType(events []ResponsesStreamEvent, typ string) *ResponsesStreamEvent {
	for i := range events {
		if events[i].Type == typ {
			return &events[i]
		}
	}
	return nil
}

func TestToolInputOnContentBlockStart_SurvivesClientToolRestore(t *testing.T) {
	mapping := ResponsesClientToolMapping{CustomTools: map[string]bool{"eval": true}}

	events := driveToolUseThroughRestorer(
		json.RawMessage(`{"input":"print(1)"}`), nil, mapping)

	inputDone := firstEventOfType(events, "response.custom_tool_call_input.done")
	require.NotNil(t, inputDone, "custom_tool_call_input.done must be emitted")
	assert.Equal(t, "print(1)", inputDone.Input,
		"inline tool input must survive conversion and client-tool restoration")

	var itemDone *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.output_item.done" && events[i].Item != nil &&
			events[i].Item.Type == "custom_tool_call" {
			itemDone = &events[i]
		}
	}
	require.NotNil(t, itemDone, "a custom_tool_call item must be closed")
	assert.Equal(t, "print(1)", itemDone.Item.Input)
}

func TestToolInputOnContentBlockStart_PlainFunctionToolChain(t *testing.T) {
	const args = `{"language":"py","code":"print(1)"}`

	events := driveToolUseThroughRestorer(
		json.RawMessage(args), nil, ResponsesClientToolMapping{})

	done := firstEventOfType(events, "response.function_call_arguments.done")
	require.NotNil(t, done)
	assert.Equal(t, args, done.Arguments)

	var itemDone *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.output_item.done" && events[i].Item != nil &&
			events[i].Item.Type == "function_call" {
			itemDone = &events[i]
		}
	}
	require.NotNil(t, itemDone)
	assert.Equal(t, args, itemDone.Item.Arguments)
}

func TestToolInputFromDeltas_SurvivesClientToolRestore(t *testing.T) {
	mapping := ResponsesClientToolMapping{CustomTools: map[string]bool{"eval": true}}

	events := driveToolUseThroughRestorer(
		json.RawMessage(`{}`),
		[]string{`{"input":"pri`, `nt(1)"}`},
		mapping)

	inputDone := firstEventOfType(events, "response.custom_tool_call_input.done")
	require.NotNil(t, inputDone)
	assert.Equal(t, "print(1)", inputDone.Input)
}
