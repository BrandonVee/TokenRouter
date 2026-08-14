package apicompat

import "testing"

// TestAnthropicEventToResponses_TextEmitsContentPart 验证每个文本分片先发送
// content_part.added，再发送 output_text.delta，避免 SDK 累积流时索引空数组。
func TestAnthropicEventToResponses_TextEmitsContentPart(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var types []string
	feed := func(evt *AnthropicStreamEvent) {
		for _, out := range AnthropicEventToResponsesEvents(evt, state) {
			types = append(types, out.Type)
		}
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1", Model: "claude-sonnet-4-5"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "Hel"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "lo"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	posOf := func(target string) int {
		for i, ty := range types {
			if ty == target {
				return i
			}
		}
		return -1
	}

	partAdded := posOf("response.content_part.added")
	firstDelta := posOf("response.output_text.delta")

	if partAdded < 0 {
		t.Fatalf("response.content_part.added was not emitted; got %v", types)
	}
	if firstDelta < 0 {
		t.Fatalf("response.output_text.delta was not emitted; got %v", types)
	}
	if partAdded > firstDelta {
		t.Errorf("content_part.added must precede the first output_text.delta; got %v", types)
	}
	if posOf("response.content_part.done") < 0 {
		t.Errorf("response.content_part.done was not emitted; got %v", types)
	}
}

// TestAnthropicEventToResponses_DoneEventsCarryFullText 验证完成事件携带完整文本。
func TestAnthropicEventToResponses_DoneEventsCarryFullText(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "Hello "}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "world"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})

	const want = "Hello world"
	var sawTextDone, sawPartDone bool
	for _, e := range events {
		switch e.Type {
		case "response.output_text.done":
			sawTextDone = true
			if e.Text != want {
				t.Errorf("output_text.done text = %q, want %q", e.Text, want)
			}
		case "response.content_part.done":
			sawPartDone = true
			if e.Part == nil || e.Part.Text != want {
				t.Errorf("content_part.done part = %+v, want text %q", e.Part, want)
			}
		}
	}
	if !sawTextDone || !sawPartDone {
		t.Errorf("missing done events: output_text.done=%v content_part.done=%v", sawTextDone, sawPartDone)
	}
}

// TestAnthropicEventToResponses_CompletedCarriesOutput 验证终止事件携带完整输出，
// 供 SDK 与链路追踪组件直接重建最终响应。
func TestAnthropicEventToResponses_CompletedCarriesOutput(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "4826"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	if completed == nil || completed.Response == nil {
		t.Fatalf("response.completed was not emitted")
	}
	if len(completed.Response.Output) == 0 {
		t.Fatalf("response.completed carries an empty output; clients would see no result")
	}
	msg := completed.Response.Output[0]
	if msg.Type != "message" || len(msg.Content) == 0 {
		t.Fatalf("output[0] = %+v, want a message with content", msg)
	}
	if msg.Content[0].Text != "4826" {
		t.Errorf("output[0].content[0].text = %q, want %q", msg.Content[0].Text, "4826")
	}
}

// TestAnthropicEventToResponses_ToolCallCompletedCarriesArguments 验证累积后的
// 函数参数会进入 output_item.done 与 response.completed。
func TestAnthropicEventToResponses_ToolCallCompletedCarriesArguments(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{
		Type: "tool_use", ID: "toolu_1", Name: "get_weather",
	}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{
		Type: "input_json_delta", PartialJSON: `{"city":`,
	}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{
		Type: "input_json_delta", PartialJSON: `"SH"}`,
	}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	if completed == nil || completed.Response == nil || len(completed.Response.Output) == 0 {
		t.Fatalf("response.completed carries no output")
	}
	fc := completed.Response.Output[0]
	if fc.Type != "function_call" {
		t.Fatalf("output[0].type = %q, want function_call", fc.Type)
	}
	if fc.Arguments != `{"city":"SH"}` {
		t.Errorf("arguments = %q, want %q", fc.Arguments, `{"city":"SH"}`)
	}
	if fc.Name != "get_weather" {
		t.Errorf("name = %q, want get_weather", fc.Name)
	}
}

// TestAnthropicEventToResponses_MultipleTextPartsUseDistinctIndices 验证同一
// message 中的连续文本块使用递增的 content_index，并完整进入终止事件。
func TestAnthropicEventToResponses_MultipleTextPartsUseDistinctIndices(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	for blockIndex, text := range []string{"first", "second"} {
		index := blockIndex
		feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &index, ContentBlock: &AnthropicContentBlock{Type: "text"}})
		feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &index, Delta: &AnthropicDelta{Type: "text_delta", Text: text}})
		feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &index})
	}
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var addedIndices []int
	var completed *ResponsesStreamEvent
	for i := range events {
		switch events[i].Type {
		case "response.content_part.added":
			addedIndices = append(addedIndices, events[i].ContentIndex)
		case "response.completed":
			completed = &events[i]
		}
	}
	if len(addedIndices) != 2 || addedIndices[0] != 0 || addedIndices[1] != 1 {
		t.Fatalf("content_part.added indices = %v, want [0 1]", addedIndices)
	}
	if completed == nil || completed.Response == nil || len(completed.Response.Output) != 1 {
		t.Fatalf("response.completed output = %+v, want one message", completed)
	}
	content := completed.Response.Output[0].Content
	if len(content) != 2 || content[0].Text != "first" || content[1].Text != "second" {
		t.Fatalf("completed message content = %+v, want first and second parts", content)
	}
}
