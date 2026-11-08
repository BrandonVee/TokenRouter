//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// 利润门的 D 使用实际付款主体，并覆盖 fork 的余额、指定订阅及多套餐混合分配。
func TestProfitControlForkFundingSources(t *testing.T) {
	group := &Group{ID: 9, Platform: PlatformOpenAI, Hydrated: true, Status: StatusActive, RateMultiplier: 3, ProfitControlEnabled: true, ProfitMinMargin: .2}
	first, second := fixedChargePlan(1, 10, 2), fixedChargePlan(2, 10, .5)
	base := context.WithValue(context.Background(), ctxkey.Group, group)
	base = context.WithValue(base, ctxkey.UserID, int64(900))
	ownerID := int64(0)
	svc := &GatewayService{userSubRepo: &fixedChargeSubRepo{subs: []UserSubscription{*first, *second}}}
	resolve := func(_ context.Context, userID, groupID int64, fallback float64) float64 { ownerID = userID; return 1.2 }
	for _, tc := range []struct {
		mode      string
		sub       *UserSubscription
		preferred *int64
		want      float64
	}{
		{mode: APIKeyBillingModeBalance, sub: first, want: 1.2},
		{mode: APIKeyBillingModeAuto, sub: first, want: .5},
		{mode: APIKeyBillingModeSubscription, sub: first, preferred: &first.ID, want: 2},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			key := &APIKey{ID: 7, UserID: 900, User: &User{ID: 1}, BillingMode: tc.mode, PreferredSubscriptionID: tc.preferred}
			ctx := WithProfitBillingContext(base, key, tc.sub)
			ctx, at := WithGatewayTokenRequestPricing(ctx)
			require.InDelta(t, tc.want, profitControlDownstreamRate(ctx, group, at, svc.userSubRepo, resolve), 1e-12)
			if tc.mode == APIKeyBillingModeBalance {
				require.Equal(t, int64(1), ownerID, "团队成员不使用自己的专属倍率代替 owner")
			}
		})
	}
}

type profitFailingSubscriptionRepo struct{ UserSubscriptionRepository }

func (profitFailingSubscriptionRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, errors.New("database unavailable")
}

// 已启用的利润策略不能在套餐数据不可读时使用可能过高的现金倍率放行付费上游。
func TestProfitControlForkFundingReadFailure(t *testing.T) {
	group := &Group{ID: 9, RateMultiplier: 3}
	ctx := WithProfitBillingContext(context.Background(), &APIKey{User: &User{ID: 1}, BillingMode: APIKeyBillingModeAuto}, nil)
	ctx, at := WithGatewayTokenRequestPricing(ctx)
	require.Zero(t, profitControlDownstreamRate(ctx, group, at, profitFailingSubscriptionRepo{}, nil))
}

// 请求转入独立媒体范围时，先前装过的门也必须清除，不能只阻止后续重新装门。
func TestProfitControlSuppressionClearsInheritedGate(t *testing.T) {
	ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{threshold: .1})
	ctx = WithOpenAIProfitControlSuppressed(ctx)
	account := manualProfitTestAccount(1, 2)
	vetoed, _ := OpenAIProfitControlVeto(ctx, account)
	require.False(t, vetoed)
	require.False(t, gatewayProfitControlGateActive(ctx))
}

// API 写入同样校验落库精度，不能绕过前端的百分比边界保护。
func TestProfitControlStoredPrecision(t *testing.T) {
	require.Error(t, ValidateProfitControlConfig(PlatformOpenAI, true, .99996, 0))
	require.Error(t, ValidateProfitControlConfig(PlatformOpenAI, true, .6, .39996))
	enabled, margin, buffer := NormalizeProfitControlConfig(PlatformOpenAI, true, .333333, .111111)
	require.True(t, enabled)
	require.Equal(t, .3333, margin)
	require.Equal(t, .1111, buffer)
}
