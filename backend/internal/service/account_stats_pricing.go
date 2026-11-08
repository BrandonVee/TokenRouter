package service

import (
	"context"
	"strings"
	"time"
)

// resolveAccountStatsCost 计算账号统计定价费用。
// 返回 nil 表示不覆盖，使用默认公式（total_cost × account_rate_multiplier）。
//
// 优先级（先命中为准）：
//  1. 自定义规则（始终尝试，不依赖 ApplyPricingToAccountStats 开关）
//  2. ApplyPricingToAccountStats 启用时，直接使用本次请求的客户计费（倍率前的 totalCost）
//  3. 模型定价文件（LiteLLM）中的默认价格（Qoder 按 requested → upstream 尝试，但已知 alias/route key 不走模型价兜底）
//  4. nil → 走默认公式（total_cost × account_rate_multiplier）
//
// upstreamModel 是最终发往上游的模型 ID。
// requestedModel 是渠道映射前的请求模型 ID；channelMappedModel 是渠道映射后的 route key。
// Qoder 这类上游 route key 与公开 alias 分离的平台会按 requested → channelMapped → upstream 尝试。
// totalCost 是本次请求的客户计费（倍率前），用于优先级 2。
// serviceTier 是最终参与用户计费的服务层级，仅用于优先级 3。
//
//nolint:unused // 兼容旧测试入口；生产路径会传入 channelMappedModel 并调用 WithMapped 版本。
func resolveAccountStatsCost(
	ctx context.Context,
	channelService *ChannelService,
	billingService *BillingService,
	accountID int64,
	groupID int64,
	upstreamModel string,
	requestedModel string,
	tokens UsageTokens,
	requestCount int,
	totalCost float64,
	serviceTier string,
) *float64 {
	return resolveAccountStatsCostWithMapped(ctx, channelService, billingService, accountID, groupID, upstreamModel, requestedModel, "", tokens, requestCount, totalCost, serviceTier)
}

func resolveAccountStatsCostWithMapped(
	ctx context.Context,
	channelService *ChannelService,
	billingService *BillingService,
	accountID int64,
	groupID int64,
	upstreamModel string,
	requestedModel string,
	channelMappedModel string,
	tokens UsageTokens,
	requestCount int,
	totalCost float64,
	serviceTier string,
) *float64 {
	if channelService == nil || upstreamModel == "" {
		return nil
	}
	channel, err := channelService.GetChannelForGroup(ctx, groupID)
	if err != nil || channel == nil {
		return nil
	}

	platform := channelService.GetGroupPlatform(ctx, groupID)

	// 优先级 1：自定义规则（始终尝试）
	for _, customRuleModel := range accountStatsCustomRuleModels(platform, upstreamModel, requestedModel, channelMappedModel) {
		if cost := tryCustomRules(channel, accountID, groupID, platform, customRuleModel, tokens, requestCount, ctx); cost != nil {
			return cost
		}
	}

	// 优先级 2：渠道开启"应用模型定价到账号统计"时，直接使用客户计费（倍率前）
	if channel.ApplyPricingToAccountStats {
		cost := totalCost
		if cost <= 0 {
			return nil
		}
		return &cost
	}

	// 优先级 3：模型定价文件（LiteLLM）默认价格
	if billingService != nil {
		if platform == PlatformQoder {
			if qoderAliasRequiresManualPricingAny(requestedModel) {
				return nil
			}
			for _, model := range accountStatsModelFilePricingModels(platform, upstreamModel, requestedModel, channelMappedModel) {
				if qoderAliasRequiresManualPricingAny(model) {
					continue
				}
				if cost := tryModelFilePricing(billingService, model, tokens, serviceTier, ctx); cost != nil {
					return cost
				}
			}
			return nil
		}
		return tryModelFilePricing(billingService, upstreamModel, tokens, serviceTier, ctx)
	}

	return nil
}

func accountStatsCustomRuleModels(platform, upstreamModel, requestedModel string, channelMappedModel ...string) []string {
	upstreamModel = strings.TrimSpace(upstreamModel)
	requestedModel = strings.TrimSpace(requestedModel)
	mappedModel := ""
	if len(channelMappedModel) > 0 {
		mappedModel = strings.TrimSpace(channelMappedModel[0])
	}
	if platform != PlatformQoder || requestedModel == "" ||
		(requestedModel == upstreamModel && (mappedModel == "" || mappedModel == requestedModel)) {
		if upstreamModel == "" {
			return nil
		}
		return []string{upstreamModel}
	}
	models := []string{requestedModel}
	models = append(models, mappedModel)
	models = append(models, upstreamModel)
	return uniqueNonEmptyAccountStatsModels(models)
}

func accountStatsModelFilePricingModels(platform, upstreamModel, requestedModel string, channelMappedModel ...string) []string {
	upstreamModel = strings.TrimSpace(upstreamModel)
	requestedModel = strings.TrimSpace(requestedModel)
	mappedModel := ""
	if len(channelMappedModel) > 0 {
		mappedModel = strings.TrimSpace(channelMappedModel[0])
	}
	if platform != PlatformQoder || requestedModel == "" ||
		(requestedModel == upstreamModel && (mappedModel == "" || mappedModel == requestedModel)) {
		if upstreamModel == "" {
			return nil
		}
		return []string{upstreamModel}
	}
	models := []string{requestedModel}
	models = append(models, mappedModel)
	models = append(models, upstreamModel)
	return uniqueNonEmptyAccountStatsModels(models)
}

func uniqueNonEmptyAccountStatsModels(models []string) []string {
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, model)
	}
	return out
}

// tryModelFilePricing 使用模型定价文件（LiteLLM/fallback）中的价格计算费用。
func tryModelFilePricing(billingService *BillingService, model string, tokens UsageTokens, serviceTier string, contexts ...context.Context) *float64 {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	cost, err := billingService.CalculateCostUnified(CostInput{
		Ctx: ctx, Model: model, Tokens: tokens, RateMultiplier: 1,
		ServiceTier: normalizeBillingServiceTier(serviceTier), Resolver: NewModelPricingResolver(nil, billingService),
	})
	if err != nil || cost == nil || cost.TotalCost <= 0 {
		return nil
	}
	return &cost.TotalCost
}

// tryCustomRules 遍历自定义规则，按数组顺序先命中为准。
func tryCustomRules(
	channel *Channel, accountID, groupID int64,
	platform, model string, tokens UsageTokens, requestCount int, contexts ...context.Context,
) *float64 {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	modelLower := strings.ToLower(model)
	for _, rule := range channel.AccountStatsPricingRules {
		if !matchAccountStatsRule(&rule, accountID, groupID) {
			continue
		}
		pricing := findEffectivePricingForModel(rule.Pricing, platform, modelLower)
		if pricing == nil {
			continue // 规则匹配但模型不在规则定价中，继续下一条
		}
		if cost := calculateStatsCost(pricing, tokens, requestCount); cost != nil {
			*cost *= reasoningEffortBillingMultiplier(usageReasoningEffort(ctx), pricing.ReasoningEffortMultipliers)
			at := GatewayTokenRequestPricingAtFromContext(ctx)
			if at.IsZero() {
				at = time.Now()
			}
			if pricing.BillingMode == "" || pricing.BillingMode == BillingModeToken {
				*cost *= pricing.PeakMultiplierAt(at)
			}
			return cost
		}
	}
	return nil
}

// matchAccountStatsRule 检查规则是否匹配指定的 accountID 和 groupID。
// 匹配条件：accountID ∈ rule.AccountIDs 或 groupID ∈ rule.GroupIDs。
// 如果规则的 AccountIDs 和 GroupIDs 都为空，视为不匹配。
func matchAccountStatsRule(rule *AccountStatsPricingRule, accountID, groupID int64) bool {
	if len(rule.AccountIDs) == 0 && len(rule.GroupIDs) == 0 {
		return false
	}
	for _, id := range rule.AccountIDs {
		if id == accountID {
			return true
		}
	}
	for _, id := range rule.GroupIDs {
		if id == groupID {
			return true
		}
	}
	return false
}

// findPricingForModel 在定价列表中查找匹配的模型定价。
// 先精确匹配，再通配符匹配（按配置顺序，先匹配先使用）。
//
//nolint:unused // 兼容旧测试入口；生产路径需要过滤空定价行并调用 findEffectivePricingForModel。
func findPricingForModel(pricingList []ChannelModelPricing, platform, modelLower string) *ChannelModelPricing {
	return findPricingForModelByPredicate(pricingList, platform, modelLower, nil)
}

// findEffectivePricingForModel 用于账号统计成本规则。
// 空定价行只是配置占位，不是成本规则；显式 0 指针仍视为有效，返回 0 成本覆盖。
func findEffectivePricingForModel(pricingList []ChannelModelPricing, platform, modelLower string) *ChannelModelPricing {
	return findPricingForModelByPredicate(pricingList, platform, modelLower, func(p *ChannelModelPricing) bool {
		return p != nil && p.HasEffectivePricing()
	})
}

func findPricingForModelByPredicate(pricingList []ChannelModelPricing, platform, modelLower string, include func(*ChannelModelPricing) bool) *ChannelModelPricing {
	if include == nil {
		include = func(*ChannelModelPricing) bool { return true }
	}
	// 精确匹配优先
	for i := range pricingList {
		p := &pricingList[i]
		if !include(p) || !isPlatformMatch(platform, p.Platform) {
			continue
		}
		for _, m := range p.Models {
			if strings.ToLower(m) == modelLower {
				return p
			}
		}
	}
	// 通配符匹配：按配置顺序，先匹配先使用
	for i := range pricingList {
		p := &pricingList[i]
		if !include(p) || !isPlatformMatch(platform, p.Platform) {
			continue
		}
		for _, m := range p.Models {
			ml := strings.ToLower(m)
			if !strings.HasSuffix(ml, "*") {
				continue
			}
			prefix := strings.TrimSuffix(ml, "*")
			if strings.HasPrefix(modelLower, prefix) {
				return p
			}
		}
	}
	return nil
}

// isPlatformMatch 判断平台是否匹配（空平台视为不限平台）。
func isPlatformMatch(queryPlatform, pricingPlatform string) bool {
	if queryPlatform == "" || pricingPlatform == "" {
		return true
	}
	return queryPlatform == pricingPlatform
}

// calculateStatsCost 使用给定的定价计算费用，并在最后应用可选的定价倍率。
func calculateStatsCost(pricing *ChannelModelPricing, tokens UsageTokens, requestCount int) *float64 {
	if pricing == nil {
		return nil
	}
	var cost *float64
	switch pricing.BillingMode {
	case BillingModePerRequest, BillingModeImage, BillingModeVideo:
		cost = calculatePerRequestStatsCost(pricing, requestCount)
	default:
		cost = calculateTokenStatsCost(pricing, tokens)
	}
	if cost == nil {
		return nil
	}
	// 账号统计规则与实际渠道计费共用同一倍率语义。
	if multiplier, configured := normalizedPriceMultiplier(pricing); configured {
		scaled := *cost * multiplier
		return &scaled
	}
	return cost
}

// calculatePerRequestStatsCost 按次/图片计费。
func calculatePerRequestStatsCost(pricing *ChannelModelPricing, requestCount int) *float64 {
	if pricing.PerRequestPrice == nil {
		return nil
	}
	if requestCount <= 0 {
		requestCount = 1
	}
	cost := *pricing.PerRequestPrice * float64(requestCount)
	if cost < 0 {
		return nil
	}
	return &cost
}

// calculateTokenStatsCost Token 计费。
// If the pricing has intervals, find the matching interval by total token count
// and use its prices instead of the flat pricing fields.
func calculateTokenStatsCost(pricing *ChannelModelPricing, tokens UsageTokens) *float64 {
	if !hasAnyTokenStatsPrice(pricing) || !hasAnyStatsTokenUsage(tokens) {
		return nil
	}
	base := &ModelPricing{}
	applyChannelTokenPriceOverrides(base, pricing)
	if pricing.ImageInputPrice != nil {
		base.ImageInputPricePerToken = *pricing.ImageInputPrice
	}
	if pricing.ImageOutputPrice != nil {
		base.ImageOutputPricePerToken = *pricing.ImageOutputPrice
		base.ImageOutputPriceExplicit = true
	}
	// 自定义统计价只继承本条规则的基础价格，不从模型目录补未知字段。
	if validIntervals := filterValidTokenIntervals(pricing.Intervals); len(validIntervals) > 0 {
		total := tokens.InputTokens + tokens.OutputTokens + tokens.CacheCreationTokens + tokens.CacheReadTokens
		if iv := FindMatchingInterval(validIntervals, total); iv != nil {
			base = intervalToModelPricingWithBase(iv, base.SupportsCacheBreakdown, pricing, base)
		}
	}
	if tokens.CacheCreation5mTokens+tokens.CacheCreation1hTokens > 0 {
		base.SupportsCacheBreakdown = true
	}
	// 与用户计费共用互斥的文本、图片和缓存桶，避免图片输出被重复计入文本输出。
	cost := (&BillingService{}).computeTokenBreakdown(base, tokens, 1, "", false).TotalCost
	if cost < 0 {
		return nil
	}
	return &cost
}

func hasAnyTokenStatsPrice(pricing *ChannelModelPricing) bool {
	if pricing == nil {
		return false
	}
	if pricing.InputPrice != nil || pricing.ImageInputPrice != nil || pricing.OutputPrice != nil ||
		pricing.CacheWritePrice != nil || pricing.CacheWrite1hPrice != nil || pricing.CacheReadPrice != nil || pricing.ImageOutputPrice != nil {
		return true
	}
	for _, interval := range filterValidTokenIntervals(pricing.Intervals) {
		if interval.InputPrice != nil || interval.OutputPrice != nil || interval.CacheWritePrice != nil || interval.CacheWrite1hPrice != nil || interval.CacheReadPrice != nil {
			return true
		}
	}
	return false
}

func hasAnyStatsTokenUsage(tokens UsageTokens) bool {
	return tokens.InputTokens > 0 || tokens.ImageInputTokens > 0 ||
		tokens.OutputTokens > 0 ||
		tokens.CacheCreationTokens > 0 || tokens.CacheCreation5mTokens > 0 || tokens.CacheCreation1hTokens > 0 ||
		tokens.CacheReadTokens > 0 ||
		tokens.ImageOutputTokens > 0
}

// applyAccountStatsCost resolves the account stats cost for a usage log entry.
// It resolves the upstream model (falling back to the requested model) and calls
// the 4-level priority chain via resolveAccountStatsCost.
func applyAccountStatsCost(
	ctx context.Context,
	usageLog *UsageLog,
	cs *ChannelService, bs *BillingService,
	accountID int64, groupID int64,
	upstreamModel, requestedModel, channelMappedModel string,
	tokens UsageTokens,
	totalCost float64,
) {
	model := upstreamModel
	if model == "" {
		model = requestedModel
	}
	requestCount := 1
	if usageLog != nil && usageLog.ImageCount > 0 {
		requestCount = usageLog.ImageCount
	}
	serviceTier := ""
	if usageLog != nil && usageLog.ServiceTier != nil {
		serviceTier = *usageLog.ServiceTier
	}
	usageLog.AccountStatsCost = resolveAccountStatsCostWithMapped(
		ctx, cs, bs, accountID, groupID, model, requestedModel, channelMappedModel, tokens, requestCount, totalCost, serviceTier,
	)
}
