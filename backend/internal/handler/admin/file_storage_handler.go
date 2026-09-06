package admin

import (
	"errors"

	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
)

// connectionTestErrorMessage 把连接测试失败转换为用户可读消息：
// 业务校验错误（如 FILE_STORAGE_PREFIX_INVALID）只返回干净的 message，
// 避免把 "error: code=... reason=..." 的内部错误串直接透给前端；
// 其余网络/IO 错误保留原始错误串以便排障。
func connectionTestErrorMessage(err error) string {
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return err.Error()
}

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
		response.Success(c, gin.H{"ok": false, "message": connectionTestErrorMessage(err)})
		return
	}
	response.Success(c, gin.H{"ok": true, "message": "connection successful"})
}
