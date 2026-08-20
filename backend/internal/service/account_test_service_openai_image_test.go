package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestIsOpenAIImageModel_AllowsGeminiCompatibleImageModel 验证账号测试会把 Gemini 生图模型路由到 Images 端点。
func TestIsOpenAIImageModel_AllowsGeminiCompatibleImageModel(t *testing.T) {
	require.True(t, isOpenAIImageModel("gemini-3-pro-image-c"))
	require.False(t, isOpenAIImageModel("gemini-3-pro"))
}

func TestAccountTestService_OpenAIImageOAuthHandlesOutputItemDoneFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_123\",\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"revised_prompt\":\"draw a cat\",\"output_format\":\"png\"}}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000006,\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:       53,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	err := svc.testOpenAIImageOAuth(c, context.Background(), account, "gpt-image-2", "draw a cat")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Contains(t, rec.Body.String(), "Calling Codex /responses image tool")
	require.Contains(t, rec.Body.String(), "data:image/png;base64,aGVsbG8=")
	require.Contains(t, rec.Body.String(), "\"success\":true")
}

func TestAccountTestService_OpenAIImageOAuthForwardsTLSProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_456\",\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"output_format\":\"png\"}}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc := &AccountTestService{
		httpUpstream:        upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:       55,
		Name:     "openai-oauth-tls",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint": true,
		},
	}

	err := svc.testOpenAIImageOAuth(c, context.Background(), account, "gpt-image-2", "draw a cat")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastTLSProfile)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
}

func TestAccountTestService_OpenAIImageAPIKeyUsesConfiguredV1BaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aGVsbG8=","revised_prompt":"draw a cat"}]}`)),
		},
	}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	account := &Account{
		ID:       54,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	err := svc.testOpenAIImageAPIKey(c, context.Background(), account, "gpt-image-2", "draw a cat")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Equal(t, "https://image-upstream.example/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer test-api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, rec.Body.String(), "data:image/png;base64,aGVsbG8=")
	require.Contains(t, rec.Body.String(), "\"success\":true")
}

// TestAccountTestService_OpenAICompatibleGeminiImageUsesImagesEndpoint 验证完整账号测试分派使用生图端点。
func TestAccountTestService_OpenAICompatibleGeminiImageUsesImagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aGVsbG8="}]}`)),
		},
	}
	svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:       56,
		Name:     "openai-gemini-compatible",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	// 即使客户端残留 compact 模式，图片模型也必须优先进入 Images 测试链路。
	err := svc.testOpenAIAccountConnection(c, account, "gemini-3-pro-image-c", "draw a cat", AccountTestModeCompact)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://image-upstream.example/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "gemini-3-pro-image-c", gjson.GetBytes(upstream.lastBody, "model").String())
}
