package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

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
	case "response.output_item.done":
		collector.addOutputItem(root.Get("item"))
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
	if itemType != "" && itemType != "image_generation_call" && itemType != "image_generation.completed" && itemType != "image_edit.completed" {
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

	identity := encoded
	if identity == "" {
		identity = imageURL
	}
	sum := sha256.Sum256([]byte(identity))
	key := hex.EncodeToString(sum[:])

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.seen[key]; exists {
		return
	}
	c.seen[key] = struct{}{}
	c.items = append(c.items, GeneratedImageCapture{
		Base64:        encoded,
		URL:           imageURL,
		MimeType:      firstNonEmpty(item.Get("mime_type").String(), item.Get("content_type").String(), mimeTypeFromOutputFormat(item.Get("output_format").String())),
		RevisedPrompt: firstNonEmpty(item.Get("revised_prompt").String(), item.Get("prompt").String()),
		Size:          strings.TrimSpace(item.Get("size").String()),
	})
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
