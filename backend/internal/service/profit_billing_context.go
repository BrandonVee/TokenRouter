package service

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/ctxkey"
)

type profitBillingContextKey struct{}

// profitBillingContext 只携带资金来源标量，不能把完整 Key 或请求正文挂给异步任务。
type profitBillingContext struct {
	userID          int64
	mode            string
	preferredID     int64
	hasSubscription bool
}

// WithProfitBillingContext 固定鉴权付款主体和本请求资金模式，供各调度器共用。
func WithProfitBillingContext(ctx context.Context, key *APIKey, sub *UserSubscription) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if key == nil || key.User == nil {
		return ctx
	}
	input := profitBillingContext{userID: key.User.ID, mode: APIKeyEffectiveBillingMode(key), hasSubscription: sub != nil}
	if key.PreferredSubscriptionID != nil {
		input.preferredID = *key.PreferredSubscriptionID
	}
	return context.WithValue(ctx, profitBillingContextKey{}, input)
}

// profitControlDownstreamRate 在可能参与本请求结算的资金倍率中取最低值。
// 这会保守覆盖多套餐以及余额溢出，不能以最贵的第一个套餐为整个请求承诺利润。
func profitControlDownstreamRate(ctx context.Context, group *Group, at time.Time, repo UserSubscriptionRepository, resolve func(context.Context, int64, int64, float64) float64) float64 {
	if group == nil {
		return 0
	}
	userID, _ := ctx.Value(ctxkey.UserID).(int64)
	input, hasInput := ctx.Value(profitBillingContextKey{}).(profitBillingContext)
	if hasInput {
		userID = input.userID
	}
	rate := group.RateMultiplier
	if userID > 0 && resolve != nil && (!hasInput || !input.hasSubscription || input.mode == APIKeyBillingModeBalance) {
		rate = resolve(ctx, userID, group.ID, rate)
	}
	if hasInput && input.mode != APIKeyBillingModeBalance && repo != nil {
		subs, err := repo.ListActiveByUserID(ctx, userID)
		if err != nil {
			// 权益来源读取失败不能默默用较高倍率继续过滤；按零阈值仅允许免费上游。
			slog.Warn("profit_control_subscription_load_failed", "user_id", userID, "group_id", group.ID, "error", err)
			return 0
		}
		for i := range subs {
			sub := &subs[i]
			if input.mode == APIKeyBillingModeSubscription && sub.ID != input.preferredID {
				continue
			}
			if !SubscriptionAllowsGroup(sub, group.ID) || checkEffectiveSubscriptionEligibility(sub) != nil {
				continue
			}
			subRate := group.RateMultiplier
			if planRate, ok := subscriptionPlanGroupRateMultiplier(sub.Plan, group.ID); ok {
				subRate = planRate
			}
			if subRate < rate {
				rate = subRate
			}
		}
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate * group.PeakMultiplierAt(at)
}
