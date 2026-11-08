package service

import "context"

// cachedContentBillingModel 沿用网关请求的模型来源和无价别名兜底，续期使用创建时保存的来源。
func (s *GatewayService) cachedContentBillingModel(ctx context.Context, input *RecordUsageInput) string {
	result := input.Result
	model := forwardResultBillingModel(result.Model, result.UpstreamModel)
	switch input.BillingModelSource {
	case BillingModelSourceUpstream:
		if result.UpstreamModel != "" {
			model = result.UpstreamModel
		}
	case BillingModelSourceChannelMapped:
		if input.ChannelMappedModel != "" {
			model = input.ChannelMappedModel
		}
	case BillingModelSourceRequested:
		if input.OriginalModel != "" {
			model = input.OriginalModel
		}
	}
	return s.billableModelWithFallback(ctx, input.APIKey, model, result.UpstreamModel, input.OriginalModel)
}

// CacheStoragePriced 要求存储价可验证，不能把未知模型按零元创建长期缓存。
func (s *GatewayService) CacheStoragePriced(ctx context.Context, input *RecordUsageInput) bool {
	return input != nil && input.Result != nil && s.cacheStoragePrice(input.Result, s.cachedContentBillingModel(ctx, input)) > 0
}

// QuoteUsageCost 与真实结算共用价格、资金倍率和命令归一化，保留跨套餐重新分配所需的基础费用。
func (s *GatewayService) QuoteUsageCost(ctx context.Context, input *RecordUsageInput) FixedChargeQuote {
	if input == nil || input.Result == nil || input.APIKey == nil {
		return FixedChargeQuote{}
	}
	ctx = WithGatewayTokenRequestPricingAt(ctx, input.PricingAt)
	ctx = withUsageReasoningEffort(ctx, input.Result.ReasoningEffort)
	user := input.User
	if user == nil {
		user = input.APIKey.User
	}
	if user == nil {
		return FixedChargeQuote{}
	}
	sub, multiplier, imageMultiplier, subMultiplier, balanceMultiplier, subScale := s.recordUsageRateMultipliers(ctx, input.APIKey, user, input.Subscription)
	model := s.cachedContentBillingModel(ctx, input)
	requested := input.OriginalModel
	if requested == "" {
		requested = input.Result.Model
	}
	account := input.Account
	if account == nil {
		account = &Account{}
	}
	cost := s.calculateRecordUsageCost(ctx, input.Result, input.APIKey, account, model, requested, input.BillingModelSource, input.ChannelMappedModel, multiplier, imageMultiplier, nil)
	cmd := buildUsageBillingCommand("quote", nil, &usageBillingParams{Cost: cost, User: user, APIKey: input.APIKey, Account: account, Subscription: sub, SubscriptionRateMultiplier: subMultiplier, SubscriptionRateMultiplierScale: subScale, BalanceRateMultiplier: balanceMultiplier})
	if cmd == nil {
		return FixedChargeQuote{}
	}
	return FixedChargeQuote{AmountUSD: cost.ActualCost, BaseAmountUSD: cmd.BaseAmountUSD, BalanceRateMultiplier: cmd.BalanceRateMultiplier, SubscriptionRateMultiplier: cmd.SubscriptionRateMultiplier, SubscriptionRateMultiplierScale: cmd.SubscriptionRateMultiplierScale, DisablePlanGroupRateMultiplier: cmd.DisablePlanGroupRateMultiplier}
}
