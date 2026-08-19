package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCompositeTargetPlatformAllowedResolvesKnownAllowedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/embeddings", nil)
	apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformComposite}}

	require.True(t, compositeTargetPlatformAllowed(c, apiKey, "text-embedding-3-large", service.PlatformOpenAI))
	platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, service.PlatformOpenAI, platform)
}

func TestOpenAICompatibleTextTargetAllowsCompositeProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	providers := []struct {
		model    string
		platform string
	}{
		{model: "grok-4.3", platform: service.PlatformGrok},
		{model: "kimi-k2-thinking", platform: service.PlatformKimi},
		{model: "glm-5.2", platform: service.PlatformZhipu},
		{model: "deepseek-v3.2", platform: service.PlatformDeepseek},
	}
	for _, path := range []string{"/v1/messages", "/v1/chat/completions", "/v1/responses"} {
		for _, provider := range providers {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, nil)
			apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformComposite}}

			require.True(t, openAICompatibleTextTargetAllowed(c, apiKey, provider.model), "path=%s model=%s", path, provider.model)
			platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
			require.True(t, ok, "path=%s model=%s", path, provider.model)
			require.Equal(t, provider.platform, platform, "path=%s model=%s", path, provider.model)
		}
	}
}

func TestResponsesWebSocketCompositePlatformGuardAllowsOpenAICompatibleProviders(t *testing.T) {
	for _, platform := range []string{
		service.PlatformOpenAI, service.PlatformGrok, service.PlatformKimi,
		service.PlatformZhipu, service.PlatformDeepseek,
	} {
		require.True(t, isOpenAICompatibleTextPlatform(platform), "platform=%s", platform)
	}
	require.False(t, isOpenAICompatibleTextPlatform(service.PlatformAnthropic))
	require.False(t, isOpenAICompatibleTextPlatform(service.PlatformGemini))
}
func TestCompositeTargetPlatformAllowedRejectsWrongOrUnknownModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name  string
		model string
	}{
		{name: "wrong provider", model: "claude-sonnet-4-5"},
		{name: "unknown provider", model: "llama-4-maverick"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/embeddings", nil)
			apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformComposite}}

			require.False(t, compositeTargetPlatformAllowed(c, apiKey, tc.model, service.PlatformOpenAI))
		})
	}
}
