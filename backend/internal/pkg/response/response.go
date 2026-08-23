// Package response provides standardized HTTP response helpers.
package response

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"strings"

	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/BrandonVee/TokenRouter/internal/util/logredact"
	"github.com/gin-gonic/gin"
)

const statusClientClosedRequest = 499

// Response 标准API响应格式
type Response struct {
	Code     int               `json:"code"`
	Message  string            `json:"message"`
	Reason   string            `json:"reason,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Data     any               `json:"data,omitempty"`
}

// PaginatedData 分页数据格式（匹配前端期望）
type PaginatedData struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Pages    int   `json:"pages"`
}

// Success 返回成功响应
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Created 返回创建成功响应
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Accepted 返回异步接受响应 (HTTP 202)
func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, Response{
		Code:    0,
		Message: "accepted",
		Data:    data,
	})
}

// Error 返回错误响应
func Error(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, Response{
		Code:     statusCode,
		Message:  message,
		Reason:   "",
		Metadata: nil,
	})
}

// ErrorWithDetails returns an error response compatible with the existing envelope while
// optionally providing structured error fields (reason/metadata).
func ErrorWithDetails(c *gin.Context, statusCode int, message, reason string, metadata map[string]string) {
	c.JSON(statusCode, Response{
		Code:     statusCode,
		Message:  message,
		Reason:   reason,
		Metadata: metadata,
	})
}

// ErrorFrom 将应用错误转换为统一响应；请求已取消时仅标记 499，不再写错误响应体或错误日志。
// 返回值表示错误是否已处理，调用方收到 true 后应立即结束当前请求。
func ErrorFrom(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if isRequestCanceled(c, err) {
		if !c.Writer.Written() {
			c.Status(statusClientClosedRequest)
		}
		return true
	}

	statusCode, status := infraerrors.ToHTTP(err)

	// 内部错误保留完整诊断信息，客户端取消则已在上方提前结束。
	if statusCode >= 500 && c.Request != nil {
		log.Printf("[ERROR] %s %s\n  Error: %s", c.Request.Method, c.Request.URL.Path, logredact.RedactText(err.Error()))
	}

	ErrorWithDetails(c, statusCode, status.Message, status.Reason, status.Metadata)
	return true
}

// isRequestCanceled 兼容 lib/pq 未保留 context.Canceled 包装的取消错误。
func isRequestCanceled(c *gin.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if c != nil && c.Request != nil {
		if errors.Is(c.Request.Context().Err(), context.Canceled) {
			return true
		}
	}
	return strings.Contains(err.Error(), "canceling statement due to user request")
}

// BadRequest 返回400错误
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized 返回401错误
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden 返回403错误
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound 返回404错误
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// InternalError 返回500错误
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}

// Paginated 返回分页数据
func Paginated(c *gin.Context, items any, total int64, page, pageSize int) {
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}

	Success(c, PaginatedData{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	})
}

// PaginationResult 分页结果（与pagination.PaginationResult兼容）
type PaginationResult struct {
	Total    int64
	Page     int
	PageSize int
	Pages    int
}

// PaginatedWithResult 使用PaginationResult返回分页数据
func PaginatedWithResult(c *gin.Context, items any, pagination *PaginationResult) {
	if pagination == nil {
		Success(c, PaginatedData{
			Items:    items,
			Total:    0,
			Page:     1,
			PageSize: 20,
			Pages:    1,
		})
		return
	}

	Success(c, PaginatedData{
		Items:    items,
		Total:    pagination.Total,
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Pages:    pagination.Pages,
	})
}

// ParsePagination 解析分页参数
func ParsePagination(c *gin.Context) (page, pageSize int) {
	page = 1
	pageSize = 20

	if p := c.Query("page"); p != "" {
		if val, err := parseInt(p); err == nil && val > 0 {
			page = val
		}
	}

	// 支持 page_size 和 limit 两种参数名
	if ps := c.Query("page_size"); ps != "" {
		if val, err := parseInt(ps); err == nil && val > 0 && val <= 1000 {
			pageSize = val
		}
	} else if l := c.Query("limit"); l != "" {
		if val, err := parseInt(l); err == nil && val > 0 && val <= 1000 {
			pageSize = val
		}
	}

	return page, pageSize
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}
