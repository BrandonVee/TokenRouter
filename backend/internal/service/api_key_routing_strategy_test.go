package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNormalizeAPIKeyRoutingStrategy 覆盖存量默认值和全部公开策略。
func TestNormalizeAPIKeyRoutingStrategy(t *testing.T) {
	t.Parallel()

	for input, expected := range map[string]string{
		"":             APIKeyRoutingStrategyManual,
		" MANUAL ":     APIKeyRoutingStrategyManual,
		"AUTO":         APIKeyRoutingStrategyAuto,
		"speed":        APIKeyRoutingStrategySpeed,
		"price":        APIKeyRoutingStrategyPrice,
		"success_rate": APIKeyRoutingStrategySuccessRate,
	} {
		actual, ok := NormalizeAPIKeyRoutingStrategy(input)
		require.True(t, ok, input)
		require.Equal(t, expected, actual, input)
	}

	_, ok := NormalizeAPIKeyRoutingStrategy("unknown")
	require.False(t, ok)
}

// TestWithAPIKeyRoutingStrategyOverridesRequestOnly 确认覆盖只存在于当前请求上下文。
func TestWithAPIKeyRoutingStrategyOverridesRequestOnly(t *testing.T) {
	t.Parallel()

	ctx := WithAPIKeyRoutingStrategy(context.Background(), APIKeyRoutingStrategySpeed)
	require.Equal(t, APIKeyRoutingStrategySpeed, APIKeyRoutingStrategyFromContext(ctx))
	require.Equal(t, APIKeyRoutingStrategyManual, APIKeyRoutingStrategyFromContext(context.Background()))
}

// TestParseGatewayRequestProviderRoutingOverrides 验证模型后缀与 provider.sort 的优先级。
func TestParseGatewayRequestProviderRoutingOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		body             string
		expectedModel    string
		expectedStrategy string
	}{
		{name: "nitro suffix", body: `{"model":"gpt-5:nitro"}`, expectedModel: "gpt-5", expectedStrategy: APIKeyRoutingStrategySpeed},
		{name: "floor suffix", body: `{"model":"gpt-5:floor"}`, expectedModel: "gpt-5", expectedStrategy: APIKeyRoutingStrategyPrice},
		{name: "provider sort wins", body: `{"model":"gpt-5:nitro","provider":{"sort":"success_rate"}}`, expectedModel: "gpt-5", expectedStrategy: APIKeyRoutingStrategySuccessRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(tt.body)), "responses")
			require.NoError(t, err)
			require.Equal(t, tt.expectedModel, parsed.Model)
			require.Equal(t, tt.expectedStrategy, parsed.RoutingStrategy)
			if tt.name == "provider sort wins" {
				require.NotContains(t, string(parsed.Body.Bytes()), "provider")
			}
		})
	}
}

// TestApplyAPIKeyPriceRoutingScores 确认价格策略使用账号成本倍率而不是原综合分。
func TestApplyAPIKeyPriceRoutingScores(t *testing.T) {
	t.Parallel()

	cheapRate, expensiveRate := 0.5, 1.5
	candidates := []advancedSchedulerCandidateScore{
		{account: &Account{ID: 1, RateMultiplier: &expensiveRate}, score: 100},
		{account: &Account{ID: 2, RateMultiplier: &cheapRate}, score: 1},
	}
	ctx := WithAPIKeyRoutingStrategy(context.Background(), APIKeyRoutingStrategyPrice)
	applyAPIKeyPriceRoutingScores(ctx, candidates)

	require.True(t, isAdvancedSchedulerCandidateBetter(candidates[1], candidates[0]))
}
