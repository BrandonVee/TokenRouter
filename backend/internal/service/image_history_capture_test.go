package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaptureGeneratedImagesFromJSON(t *testing.T) {
	collector := NewGeneratedImageCaptureCollector()
	ctx := WithGeneratedImageCaptureCollector(context.Background(), collector)

	CaptureGeneratedImagesFromJSON(ctx, []byte(`{
		"data": [
			{"b64_json":"first-image","revised_prompt":"revised"},
			{"url":"https://cdn.example.com/second.png"},
			{"b64_json":"first-image"}
		]
	}`))

	items := collector.Items()
	require.Len(t, items, 2)
	require.Equal(t, "first-image", items[0].Base64)
	require.Equal(t, "revised", items[0].RevisedPrompt)
	require.Equal(t, "https://cdn.example.com/second.png", items[1].URL)
}

func TestCaptureGeneratedImagesFromSSEIgnoresPartialAndDeduplicatesFinalImage(t *testing.T) {
	collector := NewGeneratedImageCaptureCollector()
	ctx := WithGeneratedImageCaptureCollector(context.Background(), collector)

	// partial_image 只用于进度展示，不能进入用户历史。
	CaptureGeneratedImagesFromSSE(ctx, []byte(`{"type":"response.image_generation_call.partial_image","partial_image_b64":"partial"}`))
	CaptureGeneratedImagesFromSSE(ctx, []byte(`{"type":"image_generation.partial_image","b64_json":"partial"}`))
	CaptureGeneratedImagesFromSSE(ctx, []byte(`{
		"type":"response.output_item.done",
		"item":{"type":"image_generation_call","status":"completed","result":"final-image"}
	}`))
	CaptureGeneratedImagesFromSSE(ctx, []byte(`{
		"type":"response.completed",
		"response":{"output":[{"type":"image_generation_call","status":"completed","result":"final-image"}]}
	}`))

	items := collector.Items()
	require.Len(t, items, 1)
	require.Equal(t, "final-image", items[0].Base64)
}

func TestCaptureGeneratedImagesWithoutCollectorIsNoop(t *testing.T) {
	require.NotPanics(t, func() {
		CaptureGeneratedImagesFromJSON(context.Background(), []byte(`{"data":[{"b64_json":"image"}]}`))
		CaptureGeneratedImagesFromSSE(context.Background(), []byte(`not-json`))
	})
}
