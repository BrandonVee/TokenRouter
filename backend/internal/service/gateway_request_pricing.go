package service

import (
	"context"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/timezone"
)

type gatewayTokenRequestPricingAtCtxKey struct{}

// WithGatewayTokenRequestPricing 在首次准入时冻结价格时点，后续重试与异步结算复用该值。
func WithGatewayTokenRequestPricing(ctx context.Context) (context.Context, time.Time) {
	if ctx == nil {
		ctx = context.Background()
	}
	pricingAt := timezone.Now()
	ctx = context.WithValue(ctx, gatewayTokenRequestPricingAtCtxKey{}, pricingAt)

	return ctx, pricingAt
}

func gatewayTokenRequestPricingAtFromContext(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	pricingAt, ok := ctx.Value(gatewayTokenRequestPricingAtCtxKey{}).(time.Time)
	return pricingAt, ok && !pricingAt.IsZero()
}

func GatewayTokenRequestPricingAtFromContext(ctx context.Context) time.Time {
	pricingAt, _ := gatewayTokenRequestPricingAtFromContext(ctx)
	return pricingAt
}

// WithGatewayTokenRequestPricingAt 将已冻结的时刻交给异步结算，避免跨峰谷重取当前时间。
func WithGatewayTokenRequestPricingAt(ctx context.Context, at time.Time) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if at.IsZero() {
		return ctx
	}
	return context.WithValue(ctx, gatewayTokenRequestPricingAtCtxKey{}, at)
}

type usageReasoningEffortCtxKey struct{}

// withUsageReasoningEffort 传递最终上游档位，不沿用被策略改写前的请求值。
func withUsageReasoningEffort(ctx context.Context, effort *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if effort == nil {
		return ctx
	}
	return context.WithValue(ctx, usageReasoningEffortCtxKey{}, *effort)
}

func usageReasoningEffort(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	effort, _ := ctx.Value(usageReasoningEffortCtxKey{}).(string)
	return effort
}
