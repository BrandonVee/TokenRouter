package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

// HistoryParametersFromResponsesJSON 提取 Responses 生图请求中的非敏感参数。
// 输入消息、工具上下文和内联图片不会被原样持久化。
func HistoryParametersFromResponsesJSON(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	root := gjson.ParseBytes(body)
	params := map[string]any{}
	for _, key := range []string{"model", "stream", "parallel_tool_calls", "temperature", "top_p", "max_output_tokens", "reasoning", "text"} {
		if value := root.Get(key); value.Exists() {
			params[key] = value.Value()
		}
	}
	if tools := root.Get("tools"); tools.IsArray() {
		imageTools := make([]any, 0)
		tools.ForEach(func(_, tool gjson.Result) bool {
			if strings.Contains(strings.ToLower(tool.Get("type").String()), "image_generation") {
				imageTools = append(imageTools, tool.Value())
			}
			return true
		})
		if len(imageTools) > 0 {
			params["image_generation_tools"] = imageTools
		}
	}
	data, err := json.Marshal(params)
	if err != nil || len(params) == 0 {
		return ""
	}
	return string(data)
}

// HistoryPromptFromResponsesJSON 提取 Responses 请求中的简短提示词摘要。
func HistoryPromptFromResponsesJSON(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	root := gjson.ParseBytes(body)
	for _, key := range []string{"instructions", "prompt"} {
		value := root.Get(key)
		if value.Type == gjson.String && strings.TrimSpace(value.String()) != "" {
			return strings.TrimSpace(value.String())
		}
	}
	var promptParts []string
	collectResponsePromptText(root.Get("input"), &promptParts)
	if len(promptParts) > 0 {
		return strings.Join(promptParts, "\n")
	}
	return ""
}

// HistoryParametersFromGeminiJSON 返回 Gemini 生图请求的安全参数快照。
func HistoryParametersFromGeminiJSON(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	root := gjson.ParseBytes(body)
	params := map[string]any{}
	for _, key := range []string{"generationConfig", "generation_config", "safetySettings", "safety_settings", "tools", "toolConfig", "tool_config"} {
		if value := root.Get(key); value.Exists() {
			params[key] = value.Value()
		}
	}
	data, err := json.Marshal(params)
	if err != nil || len(params) == 0 {
		return ""
	}
	return string(data)
}

// HistoryPromptFromGeminiJSON 提取 Gemini contents 中的文本提示词，忽略图片数据。
func HistoryPromptFromGeminiJSON(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	var parts []string
	root := gjson.ParseBytes(body)
	root.Get("contents").ForEach(func(_, content gjson.Result) bool {
		content.Get("parts").ForEach(func(_, part gjson.Result) bool {
			collectResponsePromptText(part.Get("text"), &parts)
			return len(parts) < 32
		})
		return len(parts) < 32
	})
	return strings.Join(parts, "\n")
}

func collectResponsePromptText(value gjson.Result, parts *[]string) {
	if parts == nil || !value.Exists() {
		return
	}
	if value.Type == gjson.String {
		text := strings.TrimSpace(value.String())
		if text != "" && !strings.HasPrefix(strings.ToLower(text), "data:image/") {
			*parts = append(*parts, text)
		}
		return
	}
	if value.IsArray() {
		value.ForEach(func(_, item gjson.Result) bool {
			collectResponsePromptText(item, parts)
			return len(*parts) < 32
		})
		return
	}
	if !value.IsObject() {
		return
	}
	for _, key := range []string{"text", "input_text", "instructions", "prompt", "content"} {
		if child := value.Get(key); child.Exists() {
			collectResponsePromptText(child, parts)
		}
	}
}

// GeneratedImageCapture 是从成功生图响应中提取的最终图片引用。
type GeneratedImageCapture struct {
	Base64        string
	URL           string
	MimeType      string
	RevisedPrompt string
	Size          string
}

type generatedImageCaptureContextKey struct{}

// GeneratedImageCaptureCollector 在单次请求内并发安全地去重最终图片。
type GeneratedImageCaptureCollector struct {
	mu    sync.Mutex
	seen  map[string]struct{}
	items []GeneratedImageCapture
}

// NewGeneratedImageCaptureCollector 创建单次生图请求使用的捕获器。
func NewGeneratedImageCaptureCollector() *GeneratedImageCaptureCollector {
	return &GeneratedImageCaptureCollector{seen: make(map[string]struct{})}
}

// WithGeneratedImageCaptureCollector 只在用户已主动开启历史时把捕获器挂到请求上下文。
func WithGeneratedImageCaptureCollector(ctx context.Context, collector *GeneratedImageCaptureCollector) context.Context {
	if collector == nil {
		return ctx
	}
	return context.WithValue(ctx, generatedImageCaptureContextKey{}, collector)
}

// CaptureGeneratedImagesFromJSON 提取普通 JSON 响应中的最终图片。
func CaptureGeneratedImagesFromJSON(ctx context.Context, body []byte) {
	collector := generatedImageCollectorFromContext(ctx)
	if collector == nil || len(body) == 0 || !gjson.ValidBytes(body) {
		return
	}
	root := gjson.ParseBytes(body)
	collector.addDataArray(root.Get("data"))
	collector.addOutputArray(root.Get("output"))
	collector.addOutputArray(root.Get("response.output"))
}

// CaptureGeneratedImagesFromSSE 提取 SSE data 载荷中的最终图片，忽略 partial_image。
func CaptureGeneratedImagesFromSSE(ctx context.Context, data []byte) {
	collector := generatedImageCollectorFromContext(ctx)
	if collector == nil || len(data) == 0 || strings.TrimSpace(string(data)) == "[DONE]" || !gjson.ValidBytes(data) {
		return
	}
	root := gjson.ParseBytes(data)
	collector.addDataArray(root.Get("data"))
	switch strings.TrimSpace(root.Get("type").String()) {
	case "response.output_item.done", "response.image_generation_call.completed":
		collector.addOutputItem(root.Get("item"))
		// 部分上游直接把最终图片字段放在 completed 事件根节点。
		if !root.Get("item").Exists() {
			collector.addOutputItem(root)
		}
	case "response.completed", "response.done":
		collector.addOutputArray(root.Get("response.output"))
	case "image_generation.completed", "image_edit.completed":
		if item := root.Get("item"); item.Exists() {
			collector.addOutputItem(item)
		} else if output := root.Get("output"); output.Exists() {
			collector.addOutputItem(output)
		} else {
			collector.addOutputItem(root)
		}
	}
}

// CaptureGeneratedImagesFromGeminiJSON 提取 Gemini 原生响应中的 inlineData 图片。
// 支持 camelCase/snake_case，并按图片数据去重，避免流式累计响应重复保存。
func CaptureGeneratedImagesFromGeminiJSON(ctx context.Context, body []byte) {
	collector := generatedImageCollectorFromContext(ctx)
	if collector == nil || len(body) == 0 || !gjson.ValidBytes(body) {
		return
	}
	root := gjson.ParseBytes(body)
	root.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			inline := part.Get("inlineData")
			if !inline.Exists() {
				inline = part.Get("inline_data")
			}
			encoded := strings.TrimSpace(inline.Get("data").String())
			if encoded == "" {
				return true
			}
			mimeType := strings.TrimSpace(inline.Get("mimeType").String())
			if mimeType == "" {
				mimeType = strings.TrimSpace(inline.Get("mime_type").String())
			}
			collector.add(GeneratedImageCapture{Base64: encoded, MimeType: mimeType})
			return true
		})
		return true
	})
}

// Items 返回与捕获器内部存储隔离的结果快照。
func (c *GeneratedImageCaptureCollector) Items() []GeneratedImageCapture {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]GeneratedImageCapture, len(c.items))
	copy(out, c.items)
	return out
}

func generatedImageCollectorFromContext(ctx context.Context) *GeneratedImageCaptureCollector {
	if ctx == nil {
		return nil
	}
	collector, _ := ctx.Value(generatedImageCaptureContextKey{}).(*GeneratedImageCaptureCollector)
	return collector
}

func (c *GeneratedImageCaptureCollector) addDataArray(data gjson.Result) {
	if c == nil || !data.IsArray() {
		return
	}
	data.ForEach(func(_, item gjson.Result) bool {
		c.addOutputItem(item)
		return true
	})
}

func (c *GeneratedImageCaptureCollector) addOutputArray(output gjson.Result) {
	if c == nil || !output.IsArray() {
		return
	}
	output.ForEach(func(_, item gjson.Result) bool {
		c.addOutputItem(item)
		return true
	})
}

func (c *GeneratedImageCaptureCollector) addOutputItem(item gjson.Result) {
	if c == nil || !item.Exists() || !item.IsObject() || strings.Contains(strings.ToLower(item.Raw), "partial_image") {
		return
	}
	itemType := strings.TrimSpace(item.Get("type").String())
	if itemType != "" && itemType != "image_generation_call" && itemType != "image_generation.completed" && itemType != "image_edit.completed" && itemType != "response.image_generation_call.completed" {
		// Images API 的 data 数组通常没有 type；带 type 时只接受明确的最终图片对象。
		if strings.TrimSpace(item.Get("b64_json").String()) == "" && strings.TrimSpace(item.Get("url").String()) == "" {
			return
		}
	}

	encoded := strings.TrimSpace(item.Get("b64_json").String())
	imageURL := strings.TrimSpace(item.Get("url").String())
	if encoded == "" && imageURL == "" {
		result := strings.TrimSpace(item.Get("result").String())
		if strings.HasPrefix(strings.ToLower(result), "http://") || strings.HasPrefix(strings.ToLower(result), "https://") || strings.HasPrefix(strings.ToLower(result), "data:image/") {
			imageURL = result
		} else {
			encoded = result
		}
	}
	if encoded == "" && imageURL == "" {
		return
	}

	c.add(GeneratedImageCapture{
		Base64:        encoded,
		URL:           imageURL,
		MimeType:      firstNonEmpty(item.Get("mime_type").String(), item.Get("content_type").String(), mimeTypeFromOutputFormat(item.Get("output_format").String())),
		RevisedPrompt: firstNonEmpty(item.Get("revised_prompt").String(), item.Get("prompt").String()),
		Size:          strings.TrimSpace(item.Get("size").String()),
	})
}

func (c *GeneratedImageCaptureCollector) add(item GeneratedImageCapture) {
	if c == nil {
		return
	}
	identity := item.Base64
	if identity == "" {
		identity = item.URL
	}
	sum := sha256.Sum256([]byte(identity))
	key := hex.EncodeToString(sum[:])
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.seen[key]; exists {
		return
	}
	c.seen[key] = struct{}{}
	c.items = append(c.items, item)
}

func mimeTypeFromOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "png":
		return "image/png"
	default:
		return ""
	}
}
