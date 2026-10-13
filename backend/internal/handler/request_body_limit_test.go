package handler

import (
	"bytes"
	"errors"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := int64(16)
	router := gin.New()
	router.Use(middleware.RequestBodyLimit(limit))
	router.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": buildBodyTooLargeMessage(maxErr.Limit),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "read_failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	payload := bytes.Repeat([]byte("a"), int(limit+1))
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), buildBodyTooLargeMessage(limit))
}

func TestReadLenientJSONRequestBodyWithPreallocUsesGatewayLimit(t *testing.T) {
	body := []byte("{\"input\":\"\x00\x00\"}")
	limit := int64(len(body) + 5)
	cfg := &config.Config{}
	cfg.Gateway.MaxBodySize = limit
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	_, err := readLenientJSONRequestBodyWithPrealloc(req, cfg)
	var maxErr *http.MaxBytesError
	require.True(t, errors.As(err, &maxErr))
	// 归一化膨胀后必须继续使用网关配置的上限。
	require.Equal(t, limit, maxErr.Limit)
}

// TestGatewayJSONRoutingOverrides 覆盖平台专用 JSON 入口的策略上下文与实际转发正文。
func TestGatewayJSONRoutingOverrides(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/v1/chat/completions", "/v1/messages"} {
		for _, tc := range []struct{ name, body, want string }{
			{"key default", `{"model":"gpt-5"}`, service.APIKeyRoutingStrategySuccessRate},
			{"speed suffix", `{"model":"gpt-5:nitro"}`, service.APIKeyRoutingStrategySpeed},
			{"price suffix", `{"model":"gpt-5:floor"}`, service.APIKeyRoutingStrategyPrice},
			{"provider wins", `{"model":"gpt-5:nitro","provider":{"sort":"price"}}`, service.APIKeyRoutingStrategyPrice},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(tc.body))
				req = req.WithContext(service.WithAPIKeyRoutingStrategy(req.Context(), service.APIKeyRoutingStrategySuccessRate))
				body, err := readLenientJSONRequestBodyWithPrealloc(req, nil)
				require.NoError(t, err)
				require.Equal(t, tc.want, service.APIKeyRoutingStrategyFromContext(req.Context()))
				require.Equal(t, "gpt-5", gjson.GetBytes(body, "model").String())
				require.False(t, gjson.GetBytes(body, "provider").Exists())
				// 通用处理器再次解析时，已消费的扩展不能把请求上下文重置为手动模式。
				parsed, err := service.ParseGatewayRequest(service.NewRequestBodyRef(body), "responses")
				require.NoError(t, err)
				ctx := service.WithAPIKeyRoutingStrategy(req.Context(), parsed.RoutingStrategy)
				require.Equal(t, tc.want, service.APIKeyRoutingStrategyFromContext(ctx))
			})
		}
	}
}
