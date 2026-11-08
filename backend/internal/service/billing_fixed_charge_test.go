//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/stretchr/testify/require"
)

// fixedChargeLedgerCache 用同一金额池模拟 Redis 的替换、并发读取和幂等释放。
type fixedChargeLedgerCache struct {
	BillingCache
	mu      sync.Mutex
	amounts map[string]map[string]float64
}

func (c *fixedChargeLedgerCache) ReserveInflightScope(_ context.Context, scope, id string, amount float64, _ time.Duration) (float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.amounts == nil {
		c.amounts = map[string]map[string]float64{}
	}
	if c.amounts[scope] == nil {
		c.amounts[scope] = map[string]float64{}
	}
	var pending float64
	for other, value := range c.amounts[scope] {
		if other != id {
			pending += value
		}
	}
	c.amounts[scope][id] = amount
	return pending, nil
}
func (c *fixedChargeLedgerCache) ReleaseInflightScope(_ context.Context, scope, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.amounts[scope], id)
	return nil
}
func (c *fixedChargeLedgerCache) RegisterInflightBalance(ctx context.Context, userID int64, id string, amount float64, ttl time.Duration) (float64, error) {
	return c.ReserveInflightScope(ctx, "balance", id, amount, ttl)
}
func (c *fixedChargeLedgerCache) ReleaseInflightBalance(ctx context.Context, _ int64, id string) error {
	return c.ReleaseInflightScope(ctx, "balance", id)
}
func (c *fixedChargeLedgerCache) ReserveInflightBalance(context.Context, int64, string, float64, float64, time.Duration) (bool, float64, error) {
	panic("确定费用不使用普通估算的首请求豁免")
}
func (c *fixedChargeLedgerCache) sum(scope string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var total float64
	for _, v := range c.amounts[scope] {
		total += v
	}
	return total
}

type fixedChargeUserRepo struct {
	UserRepository
	balance float64
}

func (r *fixedChargeUserRepo) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 1, Balance: r.balance}, nil
}

type fixedChargeSubRepo struct {
	UserSubscriptionRepository
	subs []UserSubscription
}

func (r *fixedChargeSubRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return append([]UserSubscription{}, r.subs...), nil
}
func (r *fixedChargeSubRepo) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	for _, sub := range r.subs {
		if sub.ID == id {
			return &sub, nil
		}
	}
	return nil, ErrSubscriptionInvalid
}

type fixedChargeKeyRepo struct {
	APIKeyRepository
	key *APIKey
}

func (r *fixedChargeKeyRepo) GetByID(context.Context, int64) (*APIKey, error) {
	copy := *r.key
	return &copy, nil
}

func newForkFixedChargeEnv(balance float64, subs ...UserSubscription) (*BillingCacheService, *APIKey, *fixedChargeLedgerCache) {
	cache := &fixedChargeLedgerCache{}
	key := &APIKey{ID: 8, UserID: 1, BillingMode: APIKeyBillingModeAuto, User: &User{ID: 1}, Group: &Group{ID: 9, RateMultiplier: 1}}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	cfg.Billing.InflightReservation.MaxReservationUSD = .01
	s := &BillingCacheService{cache: cache, cfg: cfg, userRepo: &fixedChargeUserRepo{balance: balance}, subRepo: &fixedChargeSubRepo{subs: subs}, apiKeyRateLimitLoader: &fixedChargeKeyRepo{key: key}}
	return s, key, cache
}

func fixedChargePlan(id int64, limit, rate float64) *UserSubscription {
	start := time.Now().Add(-time.Hour)
	return &UserSubscription{ID: id, UserID: 1, PlanID: id, StartsAt: start, ExpiresAt: time.Now().Add(time.Duration(id) * 24 * time.Hour), Status: SubscriptionStatusActive, DailyWindowStart: &start, DailyLimitUSD: &limit, Plan: &SubscriptionPlan{ID: id, GroupIDs: []int64{9}, GroupRateMultipliers: map[int64]float64{9: rate}}}
}

// 基础费用 10：第一套餐用 4（倍率 2 覆盖基础 2），第二套餐用 2（倍率 .5 覆盖基础 4），余额用 12。
func TestReserveFixedCharge_ForkMixedRatesAndUncappedAmount(t *testing.T) {
	a, b := fixedChargePlan(1, 4, 2), fixedChargePlan(2, 2, .5)
	s, key, cache := newForkFixedChargeEnv(12, *b, *a)
	quote := FixedChargeQuote{BaseAmountUSD: 10, BalanceRateMultiplier: 3, SubscriptionRateMultiplier: 1, SubscriptionRateMultiplierScale: 1}
	r, err := s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
	require.NoError(t, err)
	require.InDelta(t, 4, cache.sum("subscription:1"), 1e-9)
	require.InDelta(t, 2, cache.sum("subscription:2"), 1e-9)
	require.InDelta(t, 12, r.Amount(), 1e-9)
	require.InDelta(t, 18, cache.sum("apikey:8"), 1e-9)
	_, err = s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	r.HandlerDone()
	require.Zero(t, cache.sum("balance"))
	require.Zero(t, cache.sum("subscription:1"))
}

// 指定订阅不能借另一套餐或余额完成固定费用；无限套餐不被 AvailableQuotaUSD 的零值误拒绝。
func TestReserveFixedCharge_ForkPreferredAndUnlimited(t *testing.T) {
	a, b := fixedChargePlan(1, 4, 2), fixedChargePlan(2, 100, 1)
	s, key, cache := newForkFixedChargeEnv(100, *a, *b)
	key.BillingMode = APIKeyBillingModeSubscription
	key.PreferredSubscriptionID = &a.ID
	quote := FixedChargeQuote{BaseAmountUSD: 3, BalanceRateMultiplier: 1, SubscriptionRateMultiplier: 1, SubscriptionRateMultiplierScale: 1}
	_, err := s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
	require.ErrorIs(t, err, ErrPreferredSubscriptionInsufficient)
	require.Zero(t, cache.sum("subscription:1"))
	a.DailyLimitUSD = nil
	s.subRepo = &fixedChargeSubRepo{subs: []UserSubscription{*a}}
	r, err := s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
	require.NoError(t, err)
	defer r.HandlerDone()
	require.InDelta(t, 6, cache.sum("subscription:1"), 1e-9)
	require.Zero(t, r.Amount())
}

// 纯余额忽略用户套餐，Key 的 30 天限额同样约束确定费用。
func TestReserveFixedCharge_ForkBalanceAndMonthlyKey(t *testing.T) {
	a := fixedChargePlan(1, 100, 1)
	s, key, cache := newForkFixedChargeEnv(10, *a)
	key.BillingMode = APIKeyBillingModeBalance
	key.RateLimit30d = 5
	quote := FixedChargeQuote{BaseAmountUSD: 3, BalanceRateMultiplier: 1}
	r, err := s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
	require.NoError(t, err)
	require.Zero(t, cache.sum("subscription:1"))
	_, err = s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
	require.ErrorIs(t, err, ErrAPIKeyRateLimit30dExceeded)
	require.InDelta(t, 3, cache.sum("balance"), 1e-9)
	r.HandlerDone()
}

// 并发登记必须在读取权威 Key 用量前完成，总额度仅容纳三笔确定费用。
func TestReserveFixedCharge_ForkConcurrentKeyQuota(t *testing.T) {
	s, key, _ := newForkFixedChargeEnv(100)
	key.BillingMode = APIKeyBillingModeBalance
	key.Quota = 10
	quote := FixedChargeQuote{BaseAmountUSD: 3, BalanceRateMultiplier: 1}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var admitted []*InflightReservation
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.ReserveFixedCharge(context.Background(), key.User, key, key.Group, quote)
			if err == nil {
				mu.Lock()
				admitted = append(admitted, r)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	for _, r := range admitted {
		r.HandlerDone()
	}
	require.Len(t, admitted, 3)
}
