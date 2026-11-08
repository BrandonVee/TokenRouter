package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/pkg/httputil"
	"github.com/BrandonVee/TokenRouter/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RejectRequestBodyReadError 区分客户端取消、读取超时与实例容量不足，避免误报无效 JSON。
func RejectRequestBodyReadError(c *gin.Context, component string, err error) bool {
	if err != nil && c != nil && c.Request != nil && c.Request.Context().Err() != nil && !httputil.IsRequestBodyReadTimeout(err) {
		logRequestBodyReadFailure(c, component, httputil.DescribeRequestBodyReadFailure(c.Request, err))
		if !c.Writer.Written() {
			c.Status(httputil.StatusClientClosedRequest)
		}
		c.Abort()
		return true
	}
	return RejectRequestBodyMemoryOverload(c, err)
}

// RejectRequestBodyMemoryOverload 用可重试的 503 表示共享预算不足，不与单请求超限混淆。
func RejectRequestBodyMemoryOverload(c *gin.Context, err error) bool {
	if !errors.Is(err, httputil.ErrRequestBodyMemoryBudget) {
		return false
	}
	c.Header("Retry-After", "1")
	writeRequestBodyFailure(c, http.StatusServiceUnavailable, "Request body buffering capacity is exhausted; retry later")
	return true
}

// AbortWithRequestBodyReadFailure 供读取正文的路由/鉴权中间件复用无敏感内容的错误分类。
func AbortWithRequestBodyReadFailure(c *gin.Context, component string, err error) {
	failure := httputil.DescribeRequestBodyReadFailure(c.Request, err)
	if failure.Kind != httputil.RequestBodyReadErrorMaxBytes {
		logRequestBodyReadFailure(c, component, failure)
	}
	writeRequestBodyFailure(c, failure.Status, failure.Message)
}

func writeRequestBodyFailure(c *gin.Context, status int, message string) {
	path := c.Request.URL.Path
	switch {
	case strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/antigravity/v1beta"):
		GoogleErrorWriter(c, status, message)
	case strings.Contains(path, "/messages"):
		c.JSON(status, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": message}})
	default:
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
	}
	c.Abort()
}

func logRequestBodyReadFailure(c *gin.Context, component string, failure httputil.RequestBodyReadFailure) {
	fields := append([]zap.Field{zap.String("component", component)}, failure.LogFields()...)
	logger.FromContext(c.Request.Context()).Warn("read request body failed", fields...)
}
