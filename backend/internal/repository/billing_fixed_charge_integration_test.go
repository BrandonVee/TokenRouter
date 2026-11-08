//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// 已知费用准入与真实数据库事务对同一组套餐、倍率和余额产生相同的资金分配。
func TestFixedChargeAdmissionMatchesForkSettlement(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "fixed-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 12})
	group := mustCreateGroup(t, client, &service.Group{Name: "fixed-" + uuid.NewString(), Platform: service.PlatformGemini, RateMultiplier: 3})
	first := mustCreatePlan(t, client, &service.SubscriptionPlan{Name: "first-" + uuid.NewString(), ValidityDays: 30, ValidityUnit: "day", DailyLimitUSD: float64Ptr(4), GroupIDs: []int64{group.ID}, GroupRateMultipliers: map[int64]float64{group.ID: 2}})
	second := mustCreatePlan(t, client, &service.SubscriptionPlan{Name: "second-" + uuid.NewString(), ValidityDays: 30, ValidityUnit: "day", DailyLimitUSD: float64Ptr(2), GroupIDs: []int64{group.ID}, GroupRateMultipliers: map[int64]float64{group.ID: .5}})
	window := time.Now().Add(-time.Hour)
	a := mustCreateSubscription(t, client, &service.UserSubscription{UserID: user.ID, PlanID: first.ID, StartsAt: window, ExpiresAt: time.Now().Add(2 * 24 * time.Hour), Status: service.SubscriptionStatusActive, DailyWindowStart: &window, DailyLimitUSD: float64Ptr(4)})
	b := mustCreateSubscription(t, client, &service.UserSubscription{UserID: user.ID, PlanID: second.ID, StartsAt: window, ExpiresAt: time.Now().Add(3 * 24 * time.Hour), Status: service.SubscriptionStatusActive, DailyWindowStart: &window, DailyLimitUSD: float64Ptr(2)})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-fixed-" + uuid.NewString(), Name: "fixed", Quota: 18, BillingMode: service.APIKeyBillingModeAuto})
	key.Group = group
	key.User = user
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	cfg.Billing.InflightReservation.MaxReservationUSD = .01
	cache := NewBillingCache(integrationRedis)
	billing := service.NewBillingCacheService(cache, NewUserRepository(client, integrationDB), NewUserSubscriptionRepository(client), NewAPIKeyRepository(client, integrationDB), nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	quote := service.FixedChargeQuote{BaseAmountUSD: 10, SubscriptionRateMultiplier: 3, SubscriptionRateMultiplierScale: 1, BalanceRateMultiplier: 3}
	reservation, err := billing.ReserveFixedCharge(ctx, user, key, group, quote)
	require.NoError(t, err)
	require.InDelta(t, 12, reservation.Amount(), 1e-9)
	defer reservation.HandlerDone()
	_, err = billing.ReserveFixedCharge(ctx, user, key, group, quote)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)

	result, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, &service.UsageBillingCommand{RequestID: "gcache:create:" + uuid.NewString(), APIKeyID: key.ID, UserID: user.ID, GroupID: &group.ID, APIKeyBillingMode: service.APIKeyBillingModeAuto, BaseAmountUSD: 10, BillableAmountUSD: 18, SubscriptionRateMultiplier: 3, SubscriptionRateMultiplierScale: 1, BalanceRateMultiplier: 3, APIKeyQuotaCost: 18})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.InDelta(t, 12, result.BalanceAmountUSD, 1e-9)
	require.InDelta(t, 6, result.SubscriptionAmountUSD, 1e-9)
	var balance, usedA, usedB, keyUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", user.ID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id=$1", a.ID).Scan(&usedA))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id=$1", b.ID).Scan(&usedB))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id=$1", key.ID).Scan(&keyUsed))
	require.InDelta(t, 0, balance, 1e-9)
	require.InDelta(t, 4, usedA, 1e-9)
	require.InDelta(t, 2, usedB, 1e-9)
	require.InDelta(t, 18, keyUsed, 1e-9)
}
