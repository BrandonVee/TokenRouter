package apicompat

import "strings"

// responsesReasoningText 优先保留可见摘要，兼容仅在 content 中返回推理文本的上游。
// 两种载体同时存在时不重复拼接，也不把加密内容当作可见文本。
func responsesReasoningText(item ResponsesOutput) string {
	var text strings.Builder
	for _, part := range item.Summary {
		if part.Type == "summary_text" {
			_, _ = text.WriteString(part.Text)
		}
	}
	if text.Len() == 0 {
		for _, part := range item.Content {
			if part.Type == "reasoning_text" {
				_, _ = text.WriteString(part.Text)
			}
		}
	}
	return text.String()
}
