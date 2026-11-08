package service

import (
	"context"
	"github.com/BrandonVee/TokenRouter/internal/pkg/ctxkey"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/timezone"
)

type gatewayTokenRequestPricingAtCtxKey struct{}
type gatewayTokenRequestBillingGroupCtxKey struct{}

// WithGatewayTokenRequestPricing 在首次准入时冻结价格时点，后续重试与异步结算复用该值。
func WithGatewayTokenRequestPricing(ctx context.Context) (context.Context, time.Time) {
	if ctx == nil {
		ctx = context.Background()
	}
	pricingAt := timezone.Now()
	ctx = context.WithValue(ctx, gatewayTokenRequestPricingAtCtxKey{}, pricingAt)

	if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && IsGroupContextValid(group) {
		copy := *group
		ctx = context.WithValue(ctx, gatewayTokenRequestBillingGroupCtxKey{}, &copy)
	}
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

// gatewayTokenRequestBillingGroupFromContext 保留计费分组，避免调度 fallback 覆盖 Group 后改变售价。
func gatewayTokenRequestBillingGroupFromContext(ctx context.Context) *Group {
	if ctx == nil {
		return nil
	}
	group, _ := ctx.Value(gatewayTokenRequestBillingGroupCtxKey{}).(*Group)
	if IsGroupContextValid(group) {
		return group
	}
	return nil
}

// WithGatewayBillingGroupSnapshot 用于 fork 明确按兜底分组重新计费的路径，不改动冻结时点。
func WithGatewayBillingGroupSnapshot(ctx context.Context, group *Group) context.Context {
	if !IsGroupContextValid(group) {
		return ctx
	}
	copy := *group
	ctx = context.WithValue(ctx, gatewayTokenRequestBillingGroupCtxKey{}, &copy)
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, (*openAIProfitControlGate)(nil))
}
