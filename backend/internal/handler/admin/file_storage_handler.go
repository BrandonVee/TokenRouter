package admin

import (
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
)

// FileStorageHandler 提供统一文件存储目录的管理接口。
type FileStorageHandler struct{ service *service.FileStorageService }

// NewFileStorageHandler 创建统一文件存储处理器。
func NewFileStorageHandler(fileStorage *service.FileStorageService) *FileStorageHandler {
	return &FileStorageHandler{service: fileStorage}
}

// GetInvoiceAttachmentConfig 返回新发票附件当前使用的存储档案。
func (h *FileStorageHandler) GetInvoiceAttachmentConfig(c *gin.Context) {
	response.Success(c, h.service.GetInvoiceAttachmentConfig())
}

// UpdateInvoiceAttachmentConfig 保存新的不可变档案并切换后续上传位置。
func (h *FileStorageHandler) UpdateInvoiceAttachmentConfig(c *gin.Context) {
	var input service.FileStorageDirectoryConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	config, err := h.service.UpdateInvoiceAttachmentConfig(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

// TestInvoiceAttachmentStorageConnection 验证表单中的本地目录或 S3 桶。
func (h *FileStorageHandler) TestInvoiceAttachmentStorageConnection(c *gin.Context) {
	var input service.FileStorageDirectoryConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.TestInvoiceAttachmentStorageConnection(c.Request.Context(), input); err != nil {
		response.Success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	response.Success(c, gin.H{"ok": true, "message": "connection successful"})
}
