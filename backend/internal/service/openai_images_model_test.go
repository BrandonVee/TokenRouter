//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIImagesResponsesDriverAndImageModels(t *testing.T) {
	for _, override := range []string{"", "  ", " gpt-5.6-sol "} {
		t.Run(fmt.Sprintf("override=%q", override), func(t *testing.T) {
			t.Setenv("SUB2API_IMAGES_MAIN_MODEL", override)
			driver := strings.TrimSpace(override)
			if driver == "" {
				driver = "gpt-5.6-luna"
			}
			for _, model := range []string{"gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst"} {
				parsed := &OpenAIImagesRequest{
					Endpoint: openAIImagesGenerationsEndpoint,
					Model:    model,
					Prompt:   "draw a red cup",
					Quality:  "xhigh",
					Size:     "1536x864",
					N:        1,
				}
				body, err := buildOpenAIImagesResponsesRequest(parsed, model)
				require.NoError(t, err)
				require.Equal(t, driver, gjson.GetBytes(body, "model").String())
				require.Equal(t, model, gjson.GetBytes(body, "tools.0.model").String())
				require.Equal(t, openAIImagesVerbatimPromptInstructions, gjson.GetBytes(body, "instructions").String())
			}
		})
	}
}

func TestDoubaoSeedreamModelsAreAcceptedByImagesEndpoint(t *testing.T) {
	for _, model := range []string{
		"doubao-seedream-3-0-t2i-250415",
		"doubao-seedream-4-0-250828",
		"doubao-seedream-4-5-251128",
		"doubao-seedream-5-0-lite",
	} {
		require.NoError(t, validateOpenAIImagesModel(model), model)
	}
	require.Error(t, validateOpenAIImagesModel("doubao-seed-2-0-pro"))
}

func TestDoubaoSeedreamAllowsArkInferenceEndpointMapping(t *testing.T) {
	require.True(t, isDoubaoSeedreamModel("Doubao-Seedream-4-5-251128"))
	require.True(t, isArkInferenceEndpointModel("ep-20260920-example"))
	require.False(t, isArkInferenceEndpointModel("seedream-endpoint"))
}

func TestParseOpenAIImagesRequestCollectsSeedreamReferenceImages(t *testing.T) {
	req := &OpenAIImagesRequest{Endpoint: openAIImagesGenerationsEndpoint}
	err := parseOpenAIImagesJSONRequest([]byte(`{
		"model":"doubao-seedream-4-5-251128",
		"prompt":"融合参考图",
		"image":["https://example.com/a.png",{"url":"https://example.com/b.png"}],
		"reference_images":{"image_url":{"url":"https://example.com/c.png"}}
	}`), req)
	require.NoError(t, err)
	require.Equal(t, []string{
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
	}, req.InputImageURLs)
}

func TestNormalizeOpenAIResponsesImageOnlyModelUsesConfiguredDriver(t *testing.T) {
	t.Setenv("SUB2API_IMAGES_MAIN_MODEL", "gpt-5.6-sol")
	reqBody := map[string]any{
		"model":  "gpt-image-2.5-flare",
		"prompt": "draw a red cup",
	}

	require.True(t, normalizeOpenAIResponsesImageOnlyModel(reqBody))
	require.Equal(t, "gpt-5.6-sol", reqBody["model"])
	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	require.Equal(t, "gpt-image-2.5-flare", tools[0].(map[string]any)["model"])
}

func TestOpenAIImagesRejectedDriverDoesNotCoolImageModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_IMAGES_MAIN_MODEL", "gpt-5.4-mini")
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		for _, rejected := range []string{"gpt-5.4-mini", "gpt-image-2.5-flare"} {
			t.Run(accountType+"/"+rejected, func(t *testing.T) {
				repo := &modelNotFoundAccountRepoStub{}
				svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, nil)
				body := fmt.Sprintf(`{"error":{"message":"The '%s' model is not supported when using Codex with a ChatGPT account.","type":"invalid_request_error"}}`, rejected)
				resp := &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(body)),
				}
				account := openAICodexPlanGatedOAuthAccount()
				account.Type = accountType

				_, err := svc.handleOpenAIImagesErrorResponse(
					WithOpenAIImagesEndpoint(context.Background()),
					resp,
					c,
					account,
					nil,
					"gpt-image-2.5-flare",
				)
				require.Error(t, err)
				if rejected == "gpt-5.4-mini" {
					var upstreamErr *OpenAIImagesUpstreamError
					require.ErrorAs(t, err, &upstreamErr)
					require.Empty(t, repo.modelRateLimitCalls)
					require.Zero(t, repo.tempCalls)
				} else if accountType == AccountTypeOAuth {
					require.Len(t, repo.modelRateLimitCalls, 1)
				} else {
					// Setup Token 不参与可刷新 OAuth 的模型级冷却生命周期。
					require.Empty(t, repo.modelRateLimitCalls)
				}
			})
		}
	}
}

func TestGPTImage25PricingDoesNotUseLegacyImageRates(t *testing.T) {
	for _, model := range []string{"gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare-2026-09-08", "gpt-image-2.5-sunburst-2026-09-08"} {
		svc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
			"gpt-image-2": {InputCostPerToken: 2.5e-6, OutputCostPerImageToken: 15e-6},
		}}
		pricing := svc.GetModelPricing(model)
		require.NotNil(t, pricing)
		require.Equal(t, 5e-6, pricing.InputCostPerToken)
		require.Equal(t, 1.25e-6, pricing.CacheReadInputTokenCost)
		require.Equal(t, 8e-6, pricing.InputCostPerImageToken)
		require.Equal(t, 30e-6, pricing.OutputCostPerImageToken)
		require.Zero(t, pricing.OutputCostPerToken)
		custom := &LiteLLMModelPricing{InputCostPerToken: 7e-6}
		svc.pricingData[model] = custom
		require.Same(t, custom, svc.GetModelPricing(model))
	}
}

func TestGPTImage25UsageCapsImageInputTokens(t *testing.T) {
	usage, ok := openAIImagesToolUsageFromGJSON(gjson.Parse(`{"input_tokens":10,"input_tokens_details":{"image_tokens":12},"output_tokens":5,"output_tokens_details":{"image_tokens":5}}`))
	require.True(t, ok)
	require.Equal(t, 10, usage.ImageInputTokens)
}
