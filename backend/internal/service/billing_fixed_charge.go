package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/logger"
	"github.com/google/uuid"
)

// FixedChargeQuote 保留基础费用和资金分配倍率，避免把首个套餐的实扣金额套给所有套餐。
// 字段沿用 UsageBillingCommand 的实际分配口径，保留套餐覆盖开关。
type FixedChargeQuote struct {
	AmountUSD                       float64 // 初选资金来源的报价展示值，准入分配使用基础费用重新计算。
	BaseAmountUSD                   float64
	BalanceRateMultiplier           float64
	SubscriptionRateMultiplier      float64
	SubscriptionRateMultiplierScale float64
	DisablePlanGroupRateMultiplier  bool
}

// @project-doc docs/domains/routing_and_billing.md#fixed_charge_admission
// ReserveFixedCharge 按到期顺序模拟真实多套餐分配，并校验余额及 Key 的总额、滚动限额。
// 已知费用不能依靠普通估算的首请求放行或金额封顶；登记后再读数据库避免并发读旧用量。
func (s *BillingCacheService) ReserveFixedCharge(ctx context.Context, user *User, key *APIKey, group *Group, quote FixedChargeQuote) (*InflightReservation, error) {
	if s == nil || user == nil || key == nil {
		return nil, ErrBillingServiceUnavailable
	}
	for _, value := range []float64{quote.BaseAmountUSD, quote.BalanceRateMultiplier, quote.SubscriptionRateMultiplier, quote.SubscriptionRateMultiplierScale} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, ErrBillingServiceUnavailable
		}
	}
	if (s.cfg != nil && s.cfg.RunMode == "simple") || quote.BaseAmountUSD == 0 {
		return nil, nil
	}
	if quote.SubscriptionRateMultiplierScale == 0 {
		quote.SubscriptionRateMultiplierScale = 1
	}
	res := &InflightReservation{userID: user.ID, requestID: uuid.NewString(), ttl: defaultInflightReservationTTL}
	res.refs.Store(1)
	if cfg, enabled := s.inflightReservationConfig(); enabled {
		if cfg.TTLSeconds > 0 {
			res.ttl = time.Duration(cfg.TTLSeconds) * time.Second
		}
		res.scopeCache, _ = s.cache.(InflightScopeReservationCache)
	}
	remaining, total := quote.BaseAmountUSD, 0.0
	mode := APIKeyEffectiveBillingMode(key)
	if mode != APIKeyBillingModeBalance {
		if s.subRepo == nil {
			return nil, ErrBillingServiceUnavailable
		}
		subs, err := s.subRepo.ListActiveByUserID(ctx, user.ID)
		if err != nil {
			return nil, ErrBillingServiceUnavailable.WithCause(err)
		}
		sort.Slice(subs, func(i, j int) bool {
			if !subs[i].ExpiresAt.Equal(subs[j].ExpiresAt) {
				return subs[i].ExpiresAt.Before(subs[j].ExpiresAt)
			}
			if !subs[i].StartsAt.Equal(subs[j].StartsAt) {
				return subs[i].StartsAt.Before(subs[j].StartsAt)
			}
			return subs[i].ID < subs[j].ID
		})
		for _, candidate := range subs {
			if remaining <= 0 {
				break
			}
			if mode == APIKeyBillingModeSubscription && (key.PreferredSubscriptionID == nil || candidate.ID != *key.PreferredSubscriptionID) {
				continue
			}
			if group == nil || candidate.UserID != user.ID || !candidate.IsEffective() || !SubscriptionAllowsGroup(&candidate, group.ID) {
				continue
			}
			rate := quote.SubscriptionRateMultiplier
			if !quote.DisablePlanGroupRateMultiplier {
				if planRate, ok := subscriptionPlanGroupRateMultiplier(candidate.Plan, group.ID); ok {
					rate = planRate * quote.SubscriptionRateMultiplierScale
				}
			}
			if rate == 0 {
				remaining = 0
				break
			}
			desired := remaining * rate
			if !validCost(desired) {
				res.Release()
				return nil, ErrBillingServiceUnavailable
			}
			scope := "subscription:" + strconv.FormatInt(candidate.ID, 10)
			pending := s.registerFixedChargeScope(ctx, res, scope, desired)
			// 排序快照只定位订阅；权威用量必须在预留登记后重读。
			fresh, err := s.subRepo.GetByID(ctx, candidate.ID)
			if err != nil {
				res.Release()
				return nil, ErrBillingServiceUnavailable.WithCause(err)
			}
			if fresh == nil || fresh.UserID != user.ID || !fresh.IsEffective() || !SubscriptionAllowsGroup(fresh, group.ID) {
				s.registerFixedChargeScope(ctx, res, scope, 0)
				continue
			}
			available := desired
			if fresh.HasQuotaLimit() {
				normalized := []UserSubscription{*fresh}
				normalizeExpiredWindows(normalized)
				available = math.Max(0, normalized[0].AvailableQuotaUSD()-pending)
			}
			allocated := math.Min(desired, available)
			s.registerFixedChargeScope(ctx, res, scope, allocated)
			total += allocated
			remaining = math.Max(0, remaining-allocated/rate)
		}
		if mode == APIKeyBillingModeSubscription && remaining > 1e-12 {
			res.Release()
			return nil, ErrPreferredSubscriptionInsufficient
		}
	}
	if remaining > 0 {
		amount := remaining * quote.BalanceRateMultiplier
		if amount > 0 {
			if !validCost(amount) {
				res.Release()
				return nil, ErrBillingServiceUnavailable
			}
			pending := 0.0
			if s.InflightReservationEnabled() {
				if cache, ok := s.cache.(InflightFixedBalanceReservationCache); ok {
					value, err := cache.RegisterInflightBalance(ctx, user.ID, res.requestID, amount, res.ttl)
					if err == nil {
						pending = value
						res.cache, _ = s.cache.(InflightBalanceReservationCache)
						res.amount = amount
					} else {
						logger.LegacyPrintf("service.billing_cache", "fixed balance reservation failed (fail-open): %v", err)
					}
				}
			}
			if s.userRepo == nil {
				res.Release()
				return nil, ErrBillingServiceUnavailable
			}
			fresh, err := s.userRepo.GetByID(ctx, user.ID)
			if err != nil || fresh == nil {
				res.Release()
				return nil, ErrBillingServiceUnavailable.WithCause(err)
			}
			if fresh.Balance-pending+1e-12 < amount {
				res.Release()
				return nil, ErrInsufficientBalance
			}
			total += amount
		}
	}
	if math.IsNaN(total) || math.IsInf(total, 0) {
		res.Release()
		return nil, ErrBillingServiceUnavailable
	}
	pending := s.registerFixedChargeScope(ctx, res, "apikey:"+strconv.FormatInt(key.ID, 10), total)
	if err := s.checkFixedChargeAPIKey(ctx, key, total+pending); err != nil {
		res.Release()
		return nil, err
	}
	return res, nil
}

// registerFixedChargeScope 替换本请求在同一资金范围内的金额，返回其他请求占用；Redis 故障沿用放行策略。
func (s *BillingCacheService) registerFixedChargeScope(ctx context.Context, res *InflightReservation, scope string, amount float64) float64 {
	if res.scopeCache == nil {
		return 0
	}
	pending, err := res.scopeCache.ReserveInflightScope(ctx, scope, res.requestID, amount, res.ttl)
	if err != nil {
		logger.LegacyPrintf("service.billing_cache", "fixed charge scope reservation failed (fail-open): %v", err)
		return 0
	}
	for _, existing := range res.scopes {
		if existing == scope {
			return pending
		}
	}
	res.scopes = append(res.scopes, scope)
	return pending
}

// checkFixedChargeAPIKey 从仓储读取总配额及全部滚动窗口，不能用异步更新的 Redis 用量代替权威值。
func (s *BillingCacheService) checkFixedChargeAPIKey(ctx context.Context, key *APIKey, pending float64) error {
	if key.Quota <= 0 && !key.HasRateLimits() {
		return nil
	}
	reader, ok := s.apiKeyRateLimitLoader.(interface {
		GetByID(context.Context, int64) (*APIKey, error)
	})
	if !ok {
		return ErrBillingServiceUnavailable
	}
	fresh, err := reader.GetByID(ctx, key.ID)
	if err != nil || fresh == nil {
		return ErrBillingServiceUnavailable.WithCause(err)
	}
	exceeds := func(usage, limit float64) bool { return limit > 0 && usage+pending > limit+1e-12 }
	switch {
	case exceeds(fresh.QuotaUsed, key.Quota):
		return ErrAPIKeyQuotaExhausted
	case exceeds(fresh.EffectiveUsage5h(), key.RateLimit5h):
		return ErrAPIKeyRateLimit5hExceeded
	case exceeds(fresh.EffectiveUsage1d(), key.RateLimit1d):
		return ErrAPIKeyRateLimit1dExceeded
	case exceeds(fresh.EffectiveUsage7d(), key.RateLimit7d):
		return ErrAPIKeyRateLimit7dExceeded
	case exceeds(fresh.EffectiveUsage30d(), key.RateLimit30d):
		return ErrAPIKeyRateLimit30dExceeded
	}
	return nil
}
