package handler

import (
	"context"

	"github.com/BrandonVee/TokenRouter/internal/service"
)

// prepareImageHistoryCapture 仅为已主动开启历史的用户创建请求级捕获器。
func (h *OpenAIGatewayHandler) prepareImageHistoryCapture(ctx context.Context, userID int64) (context.Context, *service.GeneratedImageCaptureCollector) {
	if h == nil || h.imageHistoryService == nil || !h.imageHistoryService.ShouldCapture(ctx, userID) {
		return ctx, nil
	}
	collector := service.NewGeneratedImageCaptureCollector()
	return service.WithGeneratedImageCaptureCollector(ctx, collector), collector
}

// saveImageHistoryAsync 把最终图片提交到有界队列，失败或队列满都不改变原生图响应。
func (h *OpenAIGatewayHandler) saveImageHistoryAsync(input service.SaveImageHistoryInput, collector *service.GeneratedImageCaptureCollector) {
	if h == nil || h.imageHistorySaveWorkerPool == nil || collector == nil {
		return
	}
	input.Images = collector.Items()
	h.saveImageHistoryItemsAsync(input, input.Images)
}

// saveImageHistoryItemsAsync 提交指定图片，供多轮 WebSocket 请求避免重复保存历史图片。
func (h *OpenAIGatewayHandler) saveImageHistoryItemsAsync(input service.SaveImageHistoryInput, images []service.GeneratedImageCapture) bool {
	if h == nil || h.imageHistorySaveWorkerPool == nil || len(images) == 0 {
		return false
	}
	input.Images = images
	return h.imageHistorySaveWorkerPool.Submit(input)
}
