package admin

import (
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
)

// ImageHistoryHandler 提供管理员生图历史对象存储配置接口。
type ImageHistoryHandler struct {
	service *service.ImageHistoryService
}

// NewImageHistoryHandler 创建管理员生图历史存储处理器。
func NewImageHistoryHandler(imageHistoryService *service.ImageHistoryService) *ImageHistoryHandler {
	return &ImageHistoryHandler{service: imageHistoryService}
}

// GetStorageConfig 返回脱敏后的当前有效配置。
func (h *ImageHistoryHandler) GetStorageConfig(c *gin.Context) {
	response.Success(c, h.service.GetStorageConfig())
}

// UpdateStorageConfig 保存数据库覆盖配置并即时生效。
func (h *ImageHistoryHandler) UpdateStorageConfig(c *gin.Context) {
	var req service.ImageHistoryStorageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.service.UpdateStorageConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// TestStorageConnection 验证表单中的 S3 连接参数。
func (h *ImageHistoryHandler) TestStorageConnection(c *gin.Context) {
	var req service.ImageHistoryStorageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.TestStorageConnection(c.Request.Context(), req); err != nil {
		response.Success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	response.Success(c, gin.H{"ok": true, "message": "connection successful"})
}

// ResetStorageConfig 删除页面覆盖并恢复 YAML/环境变量配置。
func (h *ImageHistoryHandler) ResetStorageConfig(c *gin.Context) {
	cfg, err := h.service.ResetStorageConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
