// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectToolCallStreamEvents(t *testing.T, blockInput json.RawMessage, partials []string) []ResponsesStreamEvent {
	t.Helper()

	state := NewAnthropicEventToResponsesState()
	var all []ResponsesStreamEvent

	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_tool_input",
			Model: "gemini-3.7-flash",
			Usage: AnthropicUsage{InputTokens: 12},
		},
	}, state)...)

	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "tool_use",
			ID:    "toolu_01abc",
			Name:  "eval",
			Input: blockInput,
		},
	}, state)...)

	for _, partial := range partials {
		all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
			Type:  "content_block_delta",
			Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: partial},
		}, state)...)
	}

	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "content_block_stop"}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type:  "message_delta",
		Usage: &AnthropicUsage{OutputTokens: 9},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)...)

	return all
}

func concatArgumentDeltas(events []ResponsesStreamEvent) string {
	out := ""
	for _, e := range events {
		if e.Type == "response.function_call_arguments.delta" {
			out += e.Delta
		}
	}
	return out
}

func findFunctionCallOutput(events []ResponsesStreamEvent) *ResponsesOutput {
	for i := range events {
		if events[i].Type != "response.output_item.done" || events[i].Item == nil {
			continue
		}
		if events[i].Item.Type == "function_call" {
			return events[i].Item
		}
	}
	return nil
}

func TestAnthropicEventToResponses_ToolInputOnContentBlockStart(t *testing.T) {
	const args = `{"language":"py","code":"print(1)"}`

	events := collectToolCallStreamEvents(t, json.RawMessage(args), nil)

	item := findFunctionCallOutput(events)
	require.NotNil(t, item, "a function_call output item must be emitted")
	assert.Equal(t, args, item.Arguments,
		"arguments carried on content_block_start must survive to output_item.done")
	assert.Equal(t, "eval", item.Name)

	assert.Equal(t, args, concatArgumentDeltas(events),
		"argument deltas must reconstruct the full arguments JSON")

	done := findEvent(events, "response.function_call_arguments.done")
	require.NotNil(t, done, "function_call_arguments.done must be emitted")
	assert.Equal(t, args, done.Arguments,
		"function_call_arguments.done must carry the complete arguments")

	completed := findEvent(events, "response.completed")
	require.NotNil(t, completed)
	require.NotNil(t, completed.Response)
	require.Len(t, completed.Response.Output, 1)
	assert.Equal(t, args, completed.Response.Output[0].Arguments,
		"response.completed must carry the same arguments")
}

func TestAnthropicEventToResponses_ToolInputFromDeltasUnchanged(t *testing.T) {
	const args = `{"language":"py","code":"print(1)"}`

	events := collectToolCallStreamEvents(t, json.RawMessage(`{}`),
		[]string{`{"language":"py",`, `"code":"print(1)"}`})

	item := findFunctionCallOutput(events)
	require.NotNil(t, item)
	assert.Equal(t, args, item.Arguments)

	var deltas []string
	for _, e := range events {
		if e.Type == "response.function_call_arguments.delta" {
			deltas = append(deltas, e.Delta)
		}
	}
	assert.Equal(t, []string{`{"language":"py",`, `"code":"print(1)"}`}, deltas,
		"canonical delta streaming must not gain or lose events")
}

func TestAnthropicEventToResponses_ToolInputSeedNotDuplicatedByDeltas(t *testing.T) {
	const args = `{"language":"py","code":"print(1)"}`

	events := collectToolCallStreamEvents(t, json.RawMessage(args),
		[]string{`{"language":"py",`, `"code":"print(1)"}`})

	item := findFunctionCallOutput(events)
	require.NotNil(t, item)
	assert.Equal(t, args, item.Arguments, "deltas win over the content_block_start seed")
	assert.True(t, json.Valid([]byte(item.Arguments)), "arguments must stay valid JSON")
	assert.Equal(t, args, concatArgumentDeltas(events), "seed must not be replayed alongside deltas")
}

func TestAnthropicEventToResponses_ToolInputEmptyStaysEmptyObject(t *testing.T) {
	for name, input := range map[string]json.RawMessage{
		"absent": nil,
		"empty":  json.RawMessage(`{}`),
	} {
		t.Run(name, func(t *testing.T) {
			events := collectToolCallStreamEvents(t, input, nil)

			item := findFunctionCallOutput(events)
			require.NotNil(t, item)
			assert.Equal(t, "{}", item.Arguments)
			assert.Empty(t, concatArgumentDeltas(events),
				"no argument delta should be synthesized when there are no arguments")
		})
	}
}

func findEvent(events []ResponsesStreamEvent, eventType string) *ResponsesStreamEvent {
	for i := range events {
		if events[i].Type == eventType {
			return &events[i]
		}
	}
	return nil
}
