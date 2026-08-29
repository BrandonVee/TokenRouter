//go:build unit

package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestDeepseekPeakMultiplierAt(t *testing.T) {
	weekday := func(hour, minute int) time.Time {
		return time.Date(2026, 8, 24, hour, minute, 0, 0, time.UTC)
	}
	tests := []struct {
		name string
		now  time.Time
		want float64
	}{
		{"工作日第一个峰值窗口起点", weekday(1, 0), 2},
		{"工作日第一个峰值窗口终点", weekday(3, 59), 2},
		{"第一个峰值窗口结束", weekday(4, 0), 1},
		{"工作日第二个峰值窗口起点", weekday(6, 0), 2},
		{"第二个峰值窗口结束", weekday(10, 0), 1},
		{"工作日普通时段", weekday(12, 0), 1},
		{"北京时间周六全天低谷", time.Date(2026, 8, 22, 2, 0, 0, 0, time.UTC), 1},
		{"UTC 周六跨日到北京时间周日", time.Date(2026, 8, 22, 16, 30, 0, 0, time.UTC), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, deepseekPeakMultiplierAt(tt.now))
		})
	}
}

func TestIsDeepSeekModel(t *testing.T) {
	for _, model := range []string{
		"deepseek-v4-flash", "deepseek-v4-pro", "deepseek-chat", "deepseek-unknown",
		" DEEPSEEK-V4-PRO ",
	} {
		require.True(t, isDeepSeekModel(model), model)
	}
	for _, model := range []string{"", "deepseek", "deepseekcoder", "gpt-5.6-sol"} {
		require.False(t, isDeepSeekModel(model), model)
	}
}

func TestCalculateCostUnified_DeepseekDefaultCardPeakMultiplier(t *testing.T) {
	service := newTestBillingService()
	resolver := NewModelPricingResolver(nil, service)
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 1000}
	offPeak := 1000*deepseekFlashOffPeakInputPrice + 500*deepseekFlashOffPeakOutputPrice + 1000*deepseekFlashOffPeakCacheRead

	base := CostInput{
		Ctx: context.Background(), Model: "deepseek-v4-flash", Tokens: tokens,
		RateMultiplier: 1, Resolver: resolver,
	}
	low, err := service.CalculateCostUnified(CostInput{
		Ctx: base.Ctx, Model: base.Model, Tokens: base.Tokens, RateMultiplier: base.RateMultiplier,
		Resolver: base.Resolver, PricingAt: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.InDelta(t, offPeak, low.TotalCost, 1e-12)

	peak, err := service.CalculateCostUnified(CostInput{
		Ctx: base.Ctx, Model: base.Model, Tokens: base.Tokens, RateMultiplier: base.RateMultiplier,
		Resolver: base.Resolver, PricingAt: time.Date(2026, 8, 24, 2, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.InDelta(t, offPeak*2, peak.TotalCost, 1e-12)
}

func TestCalculateCostUnified_DeepseekCustomGroupPriceIgnoresPeak(t *testing.T) {
	service := newTestBillingService()
	resolver := NewModelPricingResolver(nil, service)
	inputPrice, outputPrice := 1e-6, 2e-6
	group := &Group{ID: 1, ModelPricing: []ChannelModelPricing{{
		Models: []string{"deepseek-v4-flash"}, BillingMode: BillingModeToken,
		InputPrice: &inputPrice, OutputPrice: &outputPrice,
	}}}
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
	want := 1000*inputPrice + 500*outputPrice

	for _, pricingAt := range []time.Time{
		time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 24, 2, 0, 0, 0, time.UTC),
	} {
		resolved := resolver.Resolve(context.Background(), PricingInput{Model: "deepseek-v4-flash", Group: group})
		cost, err := service.CalculateCostUnified(CostInput{
			Ctx: context.Background(), Model: "deepseek-v4-flash", Group: group,
			Tokens: tokens, RateMultiplier: 1, Resolver: resolver, Resolved: resolved, PricingAt: pricingAt,
		})
		require.NoError(t, err)
		require.InDelta(t, want, cost.TotalCost, 1e-12)
	}
}

func TestGetModelPricing_DeepseekUnknownFallsBackToFlash(t *testing.T) {
	service := newTestBillingService()
	for _, model := range []string{"deepseek-v4-flash-0731", "deepseek-chat", "deepseek-reasoner", "deepseek-new-model"} {
		pricing, err := service.GetModelPricing(model)
		require.NoError(t, err)
		require.Equal(t, deepseekFlashOffPeakInputPrice, pricing.InputPricePerToken)
		require.Equal(t, deepseekFlashOffPeakOutputPrice, pricing.OutputPricePerToken)
		require.Equal(t, deepseekFlashOffPeakCacheRead, pricing.CacheReadPricePerToken)
	}
}

func TestGetModelPricing_DeepseekOverridesStaleDynamicPrice(t *testing.T) {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"deepseek-v4-flash": {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6, CacheReadInputTokenCost: 3e-8},
		"deepseek-v4-pro":   {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6, CacheReadInputTokenCost: 3e-8},
	}}
	service := NewBillingService(&config.Config{}, pricingService)

	flash, err := service.GetModelPricing("deepseek-v4-flash")
	require.NoError(t, err)
	require.Equal(t, deepseekFlashOffPeakInputPrice, flash.InputPricePerToken)
	pro, err := service.GetModelPricing("deepseek-v4-pro")
	require.NoError(t, err)
	require.Equal(t, deepseekProOffPeakInputPrice, pro.InputPricePerToken)
}

func TestDeepseekPricingFileUsesOfficialModels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	service := &PricingService{}
	parsed, err := service.parsePricingData(data)
	require.NoError(t, err)
	for _, oldModel := range []string{"deepseek-chat", "deepseek-reasoner", "deepseek-v3-2-251201"} {
		_, exists := parsed[oldModel]
		require.False(t, exists, oldModel)
	}
	for _, model := range []string{"deepseek-v4-flash", "deepseek-v4-flash-vision-exp", "deepseek-v4-pro"} {
		entry, exists := parsed[model]
		require.True(t, exists, model)
		require.NotNil(t, entry)
	}
}

func TestDeepseekPricingAtZeroUsesServerClock(t *testing.T) {
	service := newTestBillingService()
	resolver := NewModelPricingResolver(nil, service)
	input := CostInput{Ctx: context.Background(), Model: "deepseek-v4-flash", Tokens: UsageTokens{InputTokens: 10}, RateMultiplier: 1, Resolver: resolver}
	zero, err := service.CalculateCostUnified(input)
	require.NoError(t, err)
	explicit, err := service.CalculateCostUnified(CostInput{
		Ctx: input.Ctx, Model: input.Model, Tokens: input.Tokens, RateMultiplier: input.RateMultiplier,
		Resolver: input.Resolver, PricingAt: timezone.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, zero.TotalCost, explicit.TotalCost)
}
