package service

// 分组利润控制只筛选账号，不改变合格候选之间的排序、评分与熔断。
// U 是账号手工成本倍率，D 是付款主体在真实计费分组的可用资金最低倍率：
// U <= D ×（1 - 最低毛利率 - 安全缓冲）。D 叠加请求冻结时点的分组高峰因子；
// 同组 failover 复用阈值，WebSocket 每轮重新冻结。明确按兜底分组重新计费时
// 同时更新计费分组快照；复合路由不能用调度分组替代真实售价分组。
//
// nil、负数与非有限账号倍率拒绝，显式 0 是免费上游。独立媒体、免费统计与
// 元数据路径显式抑制利润门。账号排队结束后复核最新快照，否决则释放槽位
// 并在有界预算内重选；旧粘性绑定保留，新绑定只在终检通过后写入。
// 分组读取失败放行并告警；已启用策略的套餐资金读取失败使用零阈值，仅允许
// 免费上游。观测按平台、分组累计并限频输出，不逐请求打印。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/ctxkey"
	"github.com/BrandonVee/TokenRouter/internal/pkg/timezone"
)

const (
	// profitControlRateEpsilon 吸收 decimal(10,4) 落库与浮点乘法的边界误差：
	// U 与阈值的相对差在该量级内视为相等（即 U == 阈值 判定为合格）。
	profitControlRateEpsilon = 1e-9

	// 进入 openAISelectionFilterStats 的排除原因，全排除时出现在
	// "no available accounts" 的内部统计摘要里（与 quota_auto_pause 等同通道）。
	openAIProfitFilterReasonThreshold          = "profit_threshold"
	openAIProfitFilterReasonInvalidAccountRate = "profit_invalid_account_rate"

	// profitControlActivityLogInterval 是按分组采样输出累计计数的最小间隔。
	profitControlActivityLogInterval = 5 * time.Minute
)

type openAIProfitControlGateCtxKey struct{}

// openAIProfitControlSuppressCtxKey 标记本请求显式跳过利润门（独立图片/视频
// 端点、Grok 媒体、count_tokens、live 等利润门范围外流量）。所有装门点看到该
// 标记后一律不装门，防止 service 层防御性装门把边界外流量重新拉回利润过滤。
type openAIProfitControlSuppressCtxKey struct{}

// openAIPricingAtCtxKey 携带请求级定价时刻 pricingAt：门的 D 与 RecordUsage
// 的高峰因子共用，保证一个请求从准入到扣费不中途变价。
type openAIPricingAtCtxKey struct{}

// clampProfitControlThreshold 归一化利润门阈值。Validate/Normalize 已保证
// margin+buffer < 1，这里只对存量脏数据兜底：阈值非有限或为负时按 0 处理
// （等价于只放行免费上游）。装门点与 profit-preview 共用，避免口径漂移。
func clampProfitControlThreshold(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 {
		return 0
	}
	return threshold
}

// profitControlOverThreshold 是"上游倍率越线"的唯一定义：相对 epsilon 吸收
// decimal(10,4) 落库与浮点乘法的边界误差，U == 阈值 判定为合格。
// 线上否决点与 profit-preview 共用，两者不得各自实现。
func profitControlOverThreshold(upstream, threshold float64) bool {
	return upstream-threshold > profitControlRateEpsilon*math.Max(1, math.Abs(threshold))
}

// openAIProfitControlGate 是一个请求的利润准入门。除 pricingAt 外全部为预计算
// 标量：候选过滤热路径上每账号只做一次快照解码与一次浮点比较。
type openAIProfitControlGate struct {
	// groupID 是门配置来源的被调度分组；请求内按分组复用（failover 阈值稳定），
	// composite 等跨分组调度切换分组时重新解析。
	groupID int64
	// platform 是利润配置所在分组的平台，用于按平台观测门是否真实生效。
	platform string
	// threshold = D(pricingAt) × (1 − margin − buffer)，账号倍率必须 <= 它。
	threshold float64
	// pricingAt 是本请求的统一定价时刻（D 侧）。
	pricingAt time.Time
}

// WithOpenAIRequestPricingContext 在请求开始处装配请求级定价上下文：固定
// pricingAt（返回给调用方，供 RecordUsage 入参共用同一时刻），并按分组安装
// 利润门。ctx 携带 WithOpenAIProfitControlSuppressed 标记（门范围外流量）时
// 只固定 pricingAt、不装门。handler 各文本入口应在选号循环前调用一次。
func (s *OpenAIGatewayService) WithOpenAIRequestPricingContext(ctx context.Context, groupID *int64) (context.Context, time.Time) {
	pricingAt := GatewayTokenRequestPricingAtFromContext(ctx)
	if pricingAt.IsZero() {
		ctx, pricingAt = WithGatewayTokenRequestPricing(ctx)
	}
	ctx = context.WithValue(ctx, openAIPricingAtCtxKey{}, pricingAt)
	return s.withOpenAIProfitControlGate(ctx, groupID), pricingAt
}

// WithOpenAIProfitControlSuppressed 标记本请求在利润门范围之外（独立图片/视频
// 端点、Grok 媒体、count_tokens、live）。所有装门点（含 service 层防御性装门）
// 都尊重该标记；它只关闭利润准入过滤，不影响定价上下文与计费。
func WithOpenAIProfitControlSuppressed(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, openAIProfitControlGateCtxKey{}, (*openAIProfitControlGate)(nil))
	return context.WithValue(ctx, openAIProfitControlSuppressCtxKey{}, struct{}{})
}

// WithOpenAITurnPricingContext 在长连接（Responses WS）的每个 turn 开始重新
// 冻结 pricingAt 并按当前配置重装利润门，使 turn 的准入与计费同源：峰前建连
// 保活不再让后续 turn 继续按建连时刻的谷价定价。连接可能被调度到与入口分组
// 不同的分组（composite 成员分组），turn 级重装以连接上已装门的调度分组为准；
// 连接从未装门时才回退入口分组。抑制标记下只刷新 pricingAt。
func (s *OpenAIGatewayService) WithOpenAITurnPricingContext(ctx context.Context, groupID *int64) (context.Context, time.Time) {
	pricingAt := timezone.Now()
	ctx = WithGatewayTokenRequestPricingAt(ctx, pricingAt)
	ctx = context.WithValue(ctx, openAIPricingAtCtxKey{}, pricingAt)
	if _, suppressed := ctx.Value(openAIProfitControlSuppressCtxKey{}).(struct{}); suppressed {
		return ctx, pricingAt
	}
	if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil {
		gid := existing.groupID
		groupID = &gid
	}
	gate := s.resolveOpenAIProfitControlGate(ctx, groupID)
	if gate == nil {
		// 分组已关门（或配置读取失败 fail-open）：清除旧 turn 的门，后续 turn
		// 按无门放行，与 HTTP 路径的开关语义一致。
		if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil {
			return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, (*openAIProfitControlGate)(nil)), pricingAt
		}
		return ctx, pricingAt
	}
	openAIProfitControlObserverInstance.recordInstall(gate.groupID, gate.platform, gate.threshold)
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, gate), pricingAt
}

// openAIPricingAtFromContext 返回请求级定价时刻；未装配（内部调用、非文本
// 入口）时 ok=false，调用方回退 timezone.Now() 保持既有行为。
func openAIPricingAtFromContext(ctx context.Context) (time.Time, bool) {
	pricingAt, ok := ctx.Value(openAIPricingAtCtxKey{}).(time.Time)
	return pricingAt, ok && !pricingAt.IsZero()
}

// OpenAIPricingAtFromContext 是 handler 侧读取请求级定价时刻的公开入口（未装配
// 时为零值，RecordUsage 对零值回退记录时刻）。供跨函数传递 pricingAt 不便的
// 记录路径直接从请求 ctx 取值。
func OpenAIPricingAtFromContext(ctx context.Context) time.Time {
	pricingAt, _ := openAIPricingAtFromContext(ctx)
	return pricingAt
}

// @project-doc docs/architecture/account_scheduling_and_cache.md#profit_admission
// withOpenAIProfitControlGate 解析分组利润控制配置；启用时把预计算好的准入门
// 装进 ctx。抑制标记、未启用/非 openai 分组/无法取到分组配置时原样返回 ctx
// （门不存在，全部否决点自动放行，既有行为零变化）。ctx 已有同分组门时直接
// 复用：同一请求的全部 failover 重入共享同一阈值。
func (s *OpenAIGatewayService) withOpenAIProfitControlGate(ctx context.Context, groupID *int64) context.Context {
	if _, suppressed := ctx.Value(openAIProfitControlSuppressCtxKey{}).(struct{}); suppressed {
		return ctx
	}
	if groupID != nil {
		if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil && existing.groupID == *groupID {
			return ctx
		}
	}
	gate := s.resolveOpenAIProfitControlGate(ctx, groupID)
	if gate == nil {
		// 被调度分组无门（未启用/非 openai/配置读取失败）而 ctx 带着其他分组的
		// 请求门时清除之：门配置取被调度分组，父分组阈值不得泄漏到成员分组
		//（composite/模型路由等跨分组调度）。typed-nil 覆盖值由否决点按无门放行。
		if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil && groupID != nil && existing.groupID != *groupID {
			return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, (*openAIProfitControlGate)(nil))
		}
		return ctx
	}
	openAIProfitControlObserverInstance.recordInstall(gate.groupID, gate.platform, gate.threshold)
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, gate)
}

func (s *OpenAIGatewayService) resolveOpenAIProfitControlGate(ctx context.Context, groupID *int64) *openAIProfitControlGate {
	if s == nil || groupID == nil || *groupID <= 0 {
		return nil
	}
	// 门配置取被调度分组。直连请求（ctx 认证分组即调度分组，生产绝大多数流量）
	// 直接复用 auth cache 分组，热路径零额外查询；composite 父分组路由到成员
	// 分组等 ID 不一致场景才回源仓库读取。auth 快照的分组字段完备性由
	// GetByKeyForAuth 投影 + 集成测试保证（防投影漏列导致门静默失效）。
	var group *Group
	if ctxGroup, ok := ctx.Value(ctxkey.Group).(*Group); ok && IsGroupContextValid(ctxGroup) && ctxGroup.ID == *groupID {
		group = ctxGroup
	} else if s.schedulerSnapshot != nil {
		// Lite 读取：门只用平台/倍率/利润/高峰字段，不需要账号计数聚合。
		loaded, err := s.schedulerSnapshot.GetGroupByIDLite(ctx, *groupID)
		if err != nil {
			// fail-open：配置系统故障时可用性优先，该窗口内利润保证不成立，
			// 依赖 WARN 暴露；不把瞬时 DB 抖动放大成全站不可调度。
			slog.Warn("profit_control_group_load_failed", "group_id", *groupID, "error", err)
			return nil
		}
		group = loaded
	}
	if group == nil || !group.ProfitControlEnabled ||
		(group.Platform != PlatformOpenAI && group.Platform != PlatformGrok) {
		return nil
	}

	pricingAt, ok := openAIPricingAtFromContext(ctx)
	if !ok {
		pricingAt = GatewayTokenRequestPricingAtFromContext(ctx)
		if pricingAt.IsZero() {
			pricingAt = timezone.Now()
		}
	}
	// D 与计费完全同源（RecordUsage 组合）：计费永远按 apiKey 自身分组
	//（composite 请求即父分组）的"用户覆盖 ?? 分组默认 × 高峰因子"计算，
	// 因此优先取认证中间件放入 ctx 的分组；ctx 中无有效分组（内部调用）时
	// 退回调度分组组合，直连 openai 分组场景两者等价。
	billingGroup := group
	if canonical := gatewayTokenRequestBillingGroupFromContext(ctx); canonical != nil {
		billingGroup = canonical
	} else if ctxGroup, ok := ctx.Value(ctxkey.Group).(*Group); ok && IsGroupContextValid(ctxGroup) {
		billingGroup = ctxGroup
	}
	downstream := profitControlDownstreamRate(ctx, billingGroup, pricingAt, s.userSubRepo, s.resolveUserGroupRateMultiplier)

	deduction := group.ProfitMinMargin + group.ProfitSafetyBuffer
	threshold := clampProfitControlThreshold(downstream * (1 - deduction))
	return &openAIProfitControlGate{
		groupID:   *groupID,
		platform:  group.Platform,
		threshold: threshold,
		pricingAt: pricingAt,
	}
}

// attachSelectionProfitGate 把调度上下文里生效的利润门记录到选号结果上。门在
// 选号函数内部的局部 ctx 上安装（composite/fallback 路由还可能解析出与入口
// 分组不同的门），不随返回值离开调度栈；结果携带后 handler 才能对"真实过滤了
// 候选的那个门"做抢槽后终检与准入后绑定。
func attachSelectionProfitGate(ctx context.Context, sel *AccountSelectionResult) *AccountSelectionResult {
	if sel == nil {
		return nil
	}
	if gate, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && gate != nil {
		sel.profitGate = gate
	}
	return sel
}

// ContextWithSelectionProfitGate 把选号时真实生效的利润门重放到 ctx 上。
// handler 在拿到选号结果后必须用返回的 ctx 做抢槽后终检
// （ProfitControlVetoLatest / GatewayProfitControlVetoLatest）与准入后粘性
// 绑定，否则这两步会因为看不到调度栈内安装的门而退化为空操作。
func ContextWithSelectionProfitGate(ctx context.Context, sel *AccountSelectionResult) context.Context {
	if sel == nil || sel.profitGate == nil {
		return ctx
	}
	if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing == sel.profitGate {
		return ctx
	}
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, sel.profitGate)
}

// openAIProfitControlVetoReason 报告利润门是否否决该账号。ctx 中没有门
// （分组未启用利润控制或本请求跳门）或账号为 nil 时一律放行。
func openAIProfitControlVetoReason(ctx context.Context, account *Account) (bool, string) {
	gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	if gate == nil || account == nil {
		return false, ""
	}
	if account.RateMultiplier == nil ||
		math.IsNaN(*account.RateMultiplier) ||
		math.IsInf(*account.RateMultiplier, 0) ||
		*account.RateMultiplier < 0 {
		openAIProfitControlObserverInstance.recordVeto(gate.groupID, gate.platform, gate.threshold, openAIProfitFilterReasonInvalidAccountRate)
		return true, openAIProfitFilterReasonInvalidAccountRate
	}
	upstream := *account.RateMultiplier
	if profitControlOverThreshold(upstream, gate.threshold) {
		openAIProfitControlObserverInstance.recordVeto(gate.groupID, gate.platform, gate.threshold, openAIProfitFilterReasonThreshold)
		return true, openAIProfitFilterReasonThreshold
	}
	return false, ""
}

// OpenAIProfitControlVeto 是 handler 层槽位获取后终检的公开入口：语义与调度
// 内否决点完全一致。ctx 必须是经 WithOpenAIRequestPricingContext 装配过的
// 请求上下文，否则视为无门放行。
func OpenAIProfitControlVeto(ctx context.Context, account *Account) (bool, string) {
	return openAIProfitControlVetoReason(ctx, account)
}

// ProfitControlVetoLatest 在获取并发槽后按最新账号倍率终检，防止排队期间的调价使用旧快照。
func (s *OpenAIGatewayService) ProfitControlVetoLatest(ctx context.Context, selected *Account) (*Account, bool, string) {
	if s == nil {
		return selected, false, ""
	}
	return profitControlVetoLatest(ctx, selected, s.schedulerSnapshot)
}

// bindOpenAIStickySessionDuringSelection 在启用利润门时延迟绑定，未启用时保持原有时机。
func (s *OpenAIGatewayService) bindOpenAIStickySessionDuringSelection(ctx context.Context, groupID *int64, sessionHash string, accountID int64) error {
	if gatewayProfitControlGateActive(ctx) {
		return nil
	}
	return s.BindStickySession(ctx, groupID, sessionHash, accountID)
}

// BindStickySessionAfterProfitAdmission 只为通过终检的新会话绑定账号，保留已有不同绑定以便倍率恢复后复用。
func (s *OpenAIGatewayService) BindStickySessionAfterProfitAdmission(ctx context.Context, groupID *int64, sessionHash string, accountID int64) error {
	if sessionHash == "" || accountID <= 0 {
		return nil
	}

	if !gatewayProfitControlGateActive(ctx) {
		return s.BindStickySession(ctx, groupID, sessionHash, accountID)
	}
	existingAccountID, err := s.getStickySessionAccountID(ctx, groupID, sessionHash)
	if err != nil && !errors.Is(err, ErrStickySessionNotFound) {
		slog.Warn("profit_control_sticky_binding_read_failed", "group_id", derefGroupID(groupID), "account_id", accountID, "error", err)
		return nil
	}
	if existingAccountID > 0 && existingAccountID != accountID {
		return nil
	}
	return s.BindStickySession(ctx, groupID, sessionHash, accountID)
}

// ---- 可观测性：按分组累计计数 + 采样日志（无逐请求输出） ----
//
// 计数按"每次准入评估"累计，而非每请求：粘性层校验与候选池过滤可能对同一账号
// 各评估一次，failover 重入也会再次计数。计数用于确认门在真实流量上生效及否决
// 构成，不能当作精确的请求数或账号数。

type openAIProfitControlGroupStats struct {
	installs         atomic.Int64
	vetoThreshold    atomic.Int64
	vetoInvalidRate  atomic.Int64
	refreshFailures  atomic.Int64
	lastLogUnixMilli atomic.Int64
}

type openAIProfitControlObserver struct {
	groups sync.Map // 按平台与分组定位累计计数。
}

var openAIProfitControlObserverInstance = &openAIProfitControlObserver{}

func profitControlObserverKey(groupID int64, platform string) string {
	return platform + ":" + fmt.Sprintf("%d", groupID)
}

func (o *openAIProfitControlObserver) stats(groupID int64, platform string) *openAIProfitControlGroupStats {
	key := profitControlObserverKey(groupID, platform)
	if v, ok := o.groups.Load(key); ok {
		if s, ok := v.(*openAIProfitControlGroupStats); ok {
			return s
		}
	}
	v, _ := o.groups.LoadOrStore(key, &openAIProfitControlGroupStats{})
	if s, ok := v.(*openAIProfitControlGroupStats); ok {
		return s
	}
	// 不可达：map 中只存 *openAIProfitControlGroupStats；兜底返回独立实例避免 panic。
	return &openAIProfitControlGroupStats{}
}

func (o *openAIProfitControlObserver) recordInstall(groupID int64, platform string, threshold float64) {
	s := o.stats(groupID, platform)
	s.installs.Add(1)
	o.maybeLog(groupID, platform, threshold, s)
}

func (o *openAIProfitControlObserver) recordVeto(groupID int64, platform string, threshold float64, reason string) {
	s := o.stats(groupID, platform)
	switch reason {
	case openAIProfitFilterReasonThreshold:
		s.vetoThreshold.Add(1)
	case openAIProfitFilterReasonInvalidAccountRate:
		s.vetoInvalidRate.Add(1)
	}
	o.maybeLog(groupID, platform, threshold, s)
}

func (o *openAIProfitControlObserver) recordRefreshFailure(groupID int64, platform string, threshold float64) {
	s := o.stats(groupID, platform)
	s.refreshFailures.Add(1)
	o.maybeLog(groupID, platform, threshold, s)
}

// maybeLog 以 CAS 保证同分组 ≥ profitControlActivityLogInterval 才输出一条
// 累计计数 Info；计数为进程内累计值，用于确认门在真实流量上生效及否决构成。
func (o *openAIProfitControlObserver) maybeLog(groupID int64, platform string, threshold float64, s *openAIProfitControlGroupStats) {
	now := time.Now().UnixMilli()
	last := s.lastLogUnixMilli.Load()
	if last != 0 && now-last < profitControlActivityLogInterval.Milliseconds() {
		return
	}
	if !s.lastLogUnixMilli.CompareAndSwap(last, now) {
		return
	}
	slog.Info("profit_control_activity",
		"group_id", groupID,
		"platform", platform,
		"threshold", threshold,
		"installs_total", s.installs.Load(),
		"veto_threshold_total", s.vetoThreshold.Load(),
		"veto_invalid_account_rate_total", s.vetoInvalidRate.Load(),
		"refresh_failure_total", s.refreshFailures.Load(),
	)
}
