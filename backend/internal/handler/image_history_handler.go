package handler

import (
	"mime"
	"net/http"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/pkg/pagination"
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	middleware2 "github.com/BrandonVee/TokenRouter/internal/server/middleware"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ImageHistoryHandler 提供当前用户的生图留存设置和私有历史管理接口。
type ImageHistoryHandler struct {
	service *service.ImageHistoryService
}

// NewImageHistoryHandler 创建生图历史处理器。
func NewImageHistoryHandler(imageHistoryService *service.ImageHistoryService) *ImageHistoryHandler {
	return &ImageHistoryHandler{service: imageHistoryService}
}

// GetSettings 返回当前用户的保存选择和部署可用性。
func (h *ImageHistoryHandler) GetSettings(c *gin.Context) {
	subject, ok := imageHistorySubject(c)
	if !ok {
		return
	}
	settings, err := h.service.GetSettings(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateSettings 更新当前用户的保存选择。
func (h *ImageHistoryHandler) UpdateSettings(c *gin.Context) {
	subject, ok := imageHistorySubject(c)
	if !ok {
		return
	}
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid image history settings")
		return
	}
	settings, err := h.service.UpdateSettings(c.Request.Context(), subject.UserID, request.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// List 返回当前用户的生图历史。
func (h *ImageHistoryHandler) List(c *gin.Context) {
	subject, ok := imageHistorySubject(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.List(c.Request.Context(), subject.UserID, service.ImageHistoryListParams{
		PaginationParams: pagination.PaginationParams{
			Page:     page,
			PageSize: pageSize,
		},
		Search: strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// Download 在用户归属校验后代理私有对象下载。
func (h *ImageHistoryHandler) Download(c *gin.Context) {
	subject, id, ok := imageHistoryTarget(c)
	if !ok {
		return
	}
	content, err := h.service.OpenContent(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = content.Body.Close() }()
	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": service.ImageHistoryDownloadName(content.Record),
	})
	c.Header("Content-Disposition", disposition)
	c.DataFromReader(http.StatusOK, content.Record.SizeBytes, content.Record.MimeType, content.Body, nil)
}

// Delete 删除当前用户的一条历史及其私有对象。
func (h *ImageHistoryHandler) Delete(c *gin.Context) {
	subject, id, ok := imageHistoryTarget(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func imageHistorySubject(c *gin.Context) (middleware2.AuthSubject, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return middleware2.AuthSubject{}, false
	}
	return subject, true
}

func imageHistoryTarget(c *gin.Context) (middleware2.AuthSubject, string, bool) {
	subject, ok := imageHistorySubject(c)
	if !ok {
		return middleware2.AuthSubject{}, "", false
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		response.BadRequest(c, "Invalid image history ID")
		return middleware2.AuthSubject{}, "", false
	}
	return subject, id, true
}
