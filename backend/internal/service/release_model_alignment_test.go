package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// 新模型必须保持独立的路由及价格，不得被旧 GPT/Claude 模型族吞并。
func TestReleaseModelAlignmentRoutingAndPricing(t *testing.T) {
	for _, tc := range []struct {
		model, canonical string
		input, output    float64
	}{
		{"openai/gpt-6.1-terra-max", "gpt-6.1-terra", 1e-6, 5e-6},
		{"gpt-6.1-sol-high", "gpt-6.1-sol", 2e-6, 10e-6},
		{"gpt-6-sol-max", "gpt-6-sol", 2e-6, 10e-6},
		{"gpt-6-luna-low", "gpt-6-luna", 0.1e-6, 0.5e-6},
	} {
		t.Run(tc.model, func(t *testing.T) {
			require.Equal(t, tc.canonical, normalizeCodexModel(tc.model))
			pricing := (&PricingService{}).GetModelPricing(tc.model)
			require.NotNil(t, pricing)
			require.InDelta(t, tc.input, pricing.InputCostPerToken, 1e-12)
			require.InDelta(t, tc.output, pricing.OutputCostPerToken, 1e-12)
		})
	}
	svc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{"gpt-6.1-sol": {InputCostPerToken: 7e-6}}}
	require.InDelta(t, 7e-6, svc.GetModelPricing("gpt-6.1-sol-max").InputCostPerToken, 1e-12, "别名应使用已配置基础价卡")
}

// Chat 降级不得丢失新模型的推理工具协议或静默提升被关闭的推理。
func TestReleaseModelChatFallbackPolicy(t *testing.T) {
	require.Error(t, validateOpenAIChatToolCompatibility("gpt-6.1-sol", false, "none"))
	require.Error(t, validateOpenAIChatToolCompatibility("gpt-6.1-terra", true, "none"))
	require.Error(t, validateOpenAIChatToolCompatibility("gpt-6-sol", true, "high"))
	require.NoError(t, validateOpenAIChatToolCompatibility("gpt-6-sol", true, "none"))
	require.NoError(t, validateOpenAIChatToolCompatibility("deepseek-v4-pro", true, "high"))
}
