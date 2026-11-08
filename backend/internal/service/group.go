package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/domain"
	"github.com/BrandonVee/TokenRouter/internal/pkg/timezone"
)

type OpenAIMessagesDispatchModelConfig = domain.OpenAIMessagesDispatchModelConfig
type GroupModelsListConfig = domain.GroupModelsListConfig
type GroupAvailabilityProbeConfig = domain.GroupAvailabilityProbeConfig
type ReasoningEffortMapping = domain.ReasoningEffortMapping
type GroupClientProtocol = domain.GroupClientProtocol
type GroupAdvancedSchedulerOverrides = domain.GroupAdvancedSchedulerOverrides

// GroupSchedulerType 表示分组使用的账号调度器类型。
type GroupSchedulerType string

const (
	// GroupSchedulerTypeBasic 保持当前默认调度路径。
	GroupSchedulerTypeBasic GroupSchedulerType = "basic"
	// GroupSchedulerTypeAdvanced 使用通用高级调度器。
	GroupSchedulerTypeAdvanced GroupSchedulerType = "advanced"
)

// NormalizeGroupSchedulerType 归一化并校验调度器类型。
func NormalizeGroupSchedulerType(value string) (GroupSchedulerType, error) {
	normalized := GroupSchedulerType(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case "", GroupSchedulerTypeBasic:
		return GroupSchedulerTypeBasic, nil
	case GroupSchedulerTypeAdvanced:
		return GroupSchedulerTypeAdvanced, nil
	default:
		return "", fmt.Errorf("scheduler_type must be basic or advanced")
	}
}

type Group struct {
	ID          int64
	Name        string
	Description string
	Platform    string
	// SchedulerType 决定该分组使用基础或高级调度器。
	SchedulerType GroupSchedulerType
	// AdvancedSchedulerOverrides 仅对高级调度分组生效，未设置字段继承网关通用设置。
	AdvancedSchedulerOverrides GroupAdvancedSchedulerOverrides
	DisplayBrand               string
	RateMultiplier             float64
	// 高峰时段倍率：peak_rate_enabled 为 true 且当前时刻处于 [PeakStart, PeakEnd) 时，
	// token 计费倍率额外乘以 PeakRateMultiplier。详见 PeakMultiplierAt。
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	// 账号成本倍率的利润准入配置，仅管理端可见。
	ProfitControlEnabled bool
	ProfitMinMargin      float64
	ProfitSafetyBuffer   float64
	IsExclusive          bool
	IsDefault            bool
	Status               string
	Hydrated             bool // indicates the group was loaded from a trusted repository source
	// DuplicateOperationID 仅用于恢复已提交的一键复制结果，不得映射到 API DTO。
	DuplicateOperationID string

	// DataSharingEnabled 表示该分组产生的 Agent session 是否进入数据共享采集流程。
	DataSharingEnabled bool
	// SessionIsolationEnabled 表示目标分组是否拒绝其它分组已归属的显式会话切入。
	SessionIsolationEnabled bool

	// 图片生成计费配置（antigravity 和 gemini 平台使用）
	AllowImageGeneration         bool
	AllowBatchImageGeneration    bool
	ImageCountBilling            bool
	ImageRateIndependent         bool
	ImageRateMultiplier          float64
	ImagePrice1K                 *float64
	ImagePrice2K                 *float64
	ImagePrice4K                 *float64
	BatchImageDiscountMultiplier float64
	BatchImageHoldMultiplier     float64
	VideoRateIndependent         bool
	VideoRateMultiplier          float64
	VideoPrice480P               *float64
	VideoPrice720P               *float64
	VideoPrice1080P              *float64
	// VideoModelPrices 是按模型族和分辨率保存的可选每秒美元价格，
	// 命中时仅对该模型优先于 VideoPrice* 平铺列。
	VideoModelPrices map[string]map[string]float64
	// Codex alpha/search 网页搜索单次价格（USD/次，仅 openai 平台使用）；
	// nil 表示使用默认价 defaultWebSearchPricePerCall（官方 $10/1000 次）。
	WebSearchPricePerCall *float64

	// 搜索工具每千次调用的显式定价。
	SearchPricePer1k *float64
	// Grok Voice 显式定价（分组级，不按文本 RateMultiplier）。
	AudioRealtimePricePerMin     *float64
	AudioTTSPricePerMillionChars *float64
	AudioSTTPricePerHour         *float64

	// ModelPricing 为命中模型覆盖渠道与内置基础价格。
	// LongContextPricingEnabled 仅控制内置长上下文倍率，不改变渠道自定义区间。
	LongContextPricingEnabled bool
	ModelPricing              []ChannelModelPricing

	// Claude Code 客户端限制
	ClaudeCodeOnly  bool
	FallbackGroupID *int64
	// 无效请求兜底分组（仅 anthropic 平台使用）
	FallbackGroupIDOnInvalidRequest *int64
	// UnavailableFallbackGroupID 表示当前分组停用时 API Key 优先回退到的分组。
	UnavailableFallbackGroupID *int64

	// 模型路由配置
	// key: 模型匹配模式（支持 * 通配符，如 "claude-opus-*"）
	// value: 优先账号 ID 列表
	ModelRouting        map[string][]int64
	ModelRoutingEnabled bool

	// MCP XML 协议注入开关（仅 antigravity 平台使用）
	MCPXMLInject bool

	// 支持的模型系列（仅 antigravity 平台使用）
	// 可选值: claude, gemini_text, gemini_image
	SupportedModelScopes []string

	// 分组排序
	SortOrder int

	// AllowedClientProtocols 是分组允许的完整客户端文本协议集合，空集合表示全部关闭。
	AllowedClientProtocols []GroupClientProtocol
	// AllowMessagesDispatch 是从协议集合派生并持久化的弃用兼容镜像。
	AllowMessagesDispatch       bool
	AllowLive                   bool
	RequireOAuthOnly            bool // 仅允许非 apikey 类型账号关联（OpenAI/Antigravity/Anthropic/Gemini）
	RequirePrivacySet           bool // 调度时仅允许 privacy 已成功设置的账号（OpenAI/Antigravity/Anthropic/Gemini）
	DefaultMappedModel          string
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
	ModelAllowlist              GroupModelAllowlist
	ModelsListConfig            GroupModelsListConfig
	// AvailabilityProbeConfig 控制该分组的主动可用性探测。
	AvailabilityProbeConfig GroupAvailabilityProbeConfig

	// RPMLimit 分组级每分钟请求数上限（0 = 不限制）。
	// 一旦设置即接管该分组用户的限流（覆盖用户级 rpm_limit），可被 user-group rpm_override 进一步覆盖。
	RPMLimit int

	// MaxReasoningEffort 限制实际生效的 OpenAI/Codex 推理强度。
	// 空字符串表示不限制；支持 minimal/low/medium/high/xhigh/max。
	MaxReasoningEffort string
	// ReasoningEffortMappings 在应用上限前改写请求中显式指定的值。
	ReasoningEffortMappings []ReasoningEffortMapping

	CreatedAt time.Time
	UpdatedAt time.Time

	AccountGroups           []AccountGroup
	AccountCount            int64
	ActiveAccountCount      int64
	RateLimitedAccountCount int64
}

// UsesAdvancedScheduler 返回分组是否启用通用高级调度器。
func (g *Group) UsesAdvancedScheduler() bool {
	return g != nil && g.SchedulerType == GroupSchedulerTypeAdvanced
}

func (g *Group) IsActive() bool {
	return g.Status == StatusActive
}

// GetImagePrice 根据 image_size 返回对应的图片生成价格
// 如果分组未配置价格，返回 nil（调用方应使用默认值）
func (g *Group) GetImagePrice(imageSize string) *float64 {
	switch imageSize {
	case "1K":
		return g.ImagePrice1K
	case "2K":
		return g.ImagePrice2K
	case "4K":
		return g.ImagePrice4K
	default:
		// 未知尺寸默认按 2K 计费
		return g.ImagePrice2K
	}
}

// GetVideoPrice 根据 resolution 返回对应的视频生成价格。
// 如果分组未配置价格，返回 nil（调用方应使用默认值）。
func (g *Group) GetVideoPrice(resolution string) *float64 {
	switch NormalizeVideoBillingResolutionOrDefault(resolution) {
	case VideoBillingResolution480P:
		return g.VideoPrice480P
	case VideoBillingResolution720P:
		return g.VideoPrice720P
	case VideoBillingResolution1080P:
		return g.VideoPrice1080P
	default:
		return g.VideoPrice480P
	}
}

// GetVideoPriceForModel 优先读取模型族价格，再回退到平铺分辨率列。
func (g *Group) GetVideoPriceForModel(model, resolution string) *float64 {
	if g == nil {
		return nil
	}
	if price := LookupVideoModelPrice(g.VideoModelPrices, model, resolution); price != nil {
		return price
	}
	return g.GetVideoPrice(resolution)
}

// VideoPriceConfig 构造包含可选模型价格映射的计费配置。
func (g *Group) VideoPriceConfig() *VideoPriceConfig {
	if g == nil {
		return nil
	}
	return &VideoPriceConfig{
		Price480P:   g.VideoPrice480P,
		Price720P:   g.VideoPrice720P,
		Price1080P:  g.VideoPrice1080P,
		ModelPrices: NormalizeVideoModelPrices(g.VideoModelPrices),
	}
}

// IsGroupContextValid reports whether a group from context has the fields required for routing decisions.
func IsGroupContextValid(group *Group) bool {
	if group == nil {
		return false
	}
	if group.ID <= 0 {
		return false
	}
	if !group.Hydrated {
		return false
	}
	if group.Platform == "" || group.Status == "" {
		return false
	}
	return true
}

// GetRoutingAccountIDs 根据请求模型获取路由账号 ID 列表
// 返回匹配的优先账号 ID 列表，如果没有匹配规则则返回 nil
func (g *Group) GetRoutingAccountIDs(requestedModel string) []int64 {
	if !g.ModelRoutingEnabled || len(g.ModelRouting) == 0 || requestedModel == "" {
		return nil
	}

	// 1. 精确匹配优先
	if accountIDs, ok := g.ModelRouting[requestedModel]; ok && len(accountIDs) > 0 {
		return accountIDs
	}

	// 2. 通配符匹配（前缀匹配）
	for pattern, accountIDs := range g.ModelRouting {
		if matchModelPattern(pattern, requestedModel) && len(accountIDs) > 0 {
			return accountIDs
		}
	}

	return nil
}

// matchModelPattern 检查模型是否匹配模式
// 支持 * 通配符，如 "claude-opus-*" 匹配 "claude-opus-4-20250514"
func matchModelPattern(pattern, model string) bool {
	if pattern == model {
		return true
	}

	// 处理 * 通配符（仅支持末尾通配符）
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}

	return false
}

// parseMinutes 把 "HH:MM" 解析为当日分钟数（0..1439），格式非法返回 (0,false)。
func parseMinutes(hhmm string) (int, bool) {
	// 手工解析避免计费热路径反复走 time.Parse；接受集保持与 time.Parse("15:04", s) 一致：
	// 小时允许 1-2 位数字（0..23），分钟必须是 2 位数字（00..59）。
	colon := strings.IndexByte(hhmm, ':')
	if (colon != 1 && colon != 2) || len(hhmm)-colon-1 != 2 {
		return 0, false
	}
	hour := 0
	for i := 0; i < colon; i++ {
		digit := hhmm[i] - '0'
		if digit > 9 {
			return 0, false
		}
		hour = hour*10 + int(digit)
	}
	minuteTens, minuteOnes := hhmm[colon+1]-'0', hhmm[colon+2]-'0'
	if minuteTens > 9 || minuteOnes > 9 {
		return 0, false
	}
	minute := int(minuteTens)*10 + int(minuteOnes)
	if hour > 23 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

// PeakMultiplierAt 返回指定时刻 now 的高峰因子。
//   - 未启用 / 未配置 / 配置非法（start>=end 或格式错误） / 非高峰时段 → 返回 1.0（安全降级）
//   - 区间为左闭右开 [PeakStart, PeakEnd)，仅支持当日区间，不支持跨天（如 22:00-次日02:00）
//   - 时刻基于全局系统时区（timezone.Location）判定
//
// 该方法是纯函数，不读取任何外部状态，便于单测。
func (g *Group) PeakMultiplierAt(now time.Time) float64 {
	if g == nil || !g.PeakRateEnabled || g.PeakStart == "" || g.PeakEnd == "" {
		return 1.0
	}
	start, ok1 := parseMinutes(g.PeakStart)
	end, ok2 := parseMinutes(g.PeakEnd)
	if !ok1 || !ok2 || start >= end {
		return 1.0
	}
	t := now.In(timezone.Location())
	cur := t.Hour()*60 + t.Minute()
	if cur >= start && cur < end {
		return g.PeakRateMultiplier
	}
	return 1.0
}

// ValidatePeakRateConfig 是高峰倍率配置的唯一校验来源，供 handler 与 service 层共用。
// enabled=true 时要求 start/end 合法且 end>start（不支持跨天），multiplier>=0。
// multiplier=0 是允许的，表示高峰 token 请求按 0 倍计费，可用于折扣/免费策略。
// enabled=false 时放行。
func ValidatePeakRateConfig(enabled bool, start, end string, multiplier float64) error {
	if !enabled {
		return nil
	}
	if start == "" || end == "" {
		return errors.New("peak_rate_enabled 为 true 时 peak_start 与 peak_end 必填")
	}
	st, okStart := parseMinutes(start)
	if !okStart {
		return fmt.Errorf("peak_start 格式应为 HH:MM，got %q", start)
	}
	en, okEnd := parseMinutes(end)
	if !okEnd {
		return fmt.Errorf("peak_end 格式应为 HH:MM，got %q", end)
	}
	if st >= en {
		return errors.New("peak_end 必须大于 peak_start（不支持跨天区间，如 22:00-02:00）")
	}
	if multiplier < 0 {
		return errors.New("peak_rate_multiplier 不能为负")
	}
	return nil
}

// NormalizePeakRateConfig 归一化最终落库的高峰倍率配置，供 CreateGroup 与 UpdateGroup 共用。
// 启用时保持原值并交给 ValidatePeakRateConfig 严格校验；停用时保留合法窗口与非负倍率，
// 仅清理脏窗口与负倍率，便于管理员临时停用后按原配置重新启用。
func NormalizePeakRateConfig(enabled bool, start, end string, multiplier float64) (bool, string, string, float64) {
	if enabled {
		return enabled, start, end, multiplier
	}
	if _, ok := parseMinutes(start); !ok {
		start = ""
	}
	if _, ok := parseMinutes(end); !ok {
		end = ""
	}
	if multiplier < 0 {
		multiplier = 1.0
	}
	return false, start, end, multiplier
}

// computePeakAwareMultipliers 把"基础 token 倍率 base"（已含系统/分组/用户级倍率，但不含高峰）
// 拆分为最终 token 倍率与图片按次倍率：图片按次倍率基于 base 现算、不受高峰影响；token 倍率在 base 上叠加高峰因子。
// gateway_service.recordUsageCore 与 openai_gateway_service.RecordUsage 共用此函数，
// 锁死"高峰因子只乘入 token 倍率、图片按次倍率不受影响"这一叠加顺序——任何调换都会被 group_peak_rate_test 覆盖。
func computePeakAwareMultipliers(apiKey *APIKey, base float64, now time.Time) (text, image float64) {
	image = resolveImageRateMultiplier(apiKey, base)
	peak := 1.0
	if apiKey != nil && apiKey.Group != nil {
		peak = apiKey.Group.PeakMultiplierAt(now)
	}
	text = base * peak
	return
}

// GetSearchPricePer1k 返回分组显式配置的搜索工具每千次价格。
func (g *Group) GetSearchPricePer1k() *float64 {
	if g == nil {
		return nil
	}
	return g.SearchPricePer1k
}

// validProfitControlRatio 判定 margin/buffer 是否为可落库的合法小数：[0,1) 且非 NaN/Inf。
func validProfitControlRatio(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < 1
}

// NormalizeGroupPlatform 把创建分组时省略的 platform 归一化为默认平台。
// handler 的入参预校验必须与 CreateGroup 落库时用同一个归一化结果，否则
// 「省略 platform + 启用利润控制」会被 handler 以「平台不支持」400 掉，
// 而该分组本会被建成受支持的 anthropic 分组。
func NormalizeGroupPlatform(platform string) string {
	if platform == "" {
		return PlatformAnthropic
	}
	return platform
}

// ValidateProfitControlConfig 是分组利润控制配置的唯一校验来源，handler 与 service 层共用。
// enabled=true 时仅允许五个可计费平台分组；margin/buffer 各自 ∈ [0,1)，且 margin+buffer < 1
// （相加 >=1 时阈值 <=0，所有可核价账号都会被排除，视为配置错误而不是静默全黑）。
// enabled=false 时放行（不关心平台），由 Normalize 兜底清洗数值。
func ValidateProfitControlConfig(platform string, enabled bool, minMargin, safetyBuffer float64) error {
	if !enabled {
		return nil
	}
	if !profitControlPlatformSupported(platform) {
		return errors.New("利润控制仅支持 openai、anthropic、gemini、grok、antigravity 平台分组")
	}
	if !validProfitControlRatio(minMargin) || !validProfitControlRatio(math.Round(minMargin*1e4)/1e4) {
		return fmt.Errorf("profit_min_margin 应为 [0,1) 的小数，got %v", minMargin)
	}
	if !validProfitControlRatio(safetyBuffer) || !validProfitControlRatio(math.Round(safetyBuffer*1e4)/1e4) {
		return fmt.Errorf("profit_safety_buffer 应为 [0,1) 的小数，got %v", safetyBuffer)
	}
	// 按数据库 decimal(10,4) 精度判断，防止校验后进位成 100%。
	if math.Round(minMargin*1e4)+math.Round(safetyBuffer*1e4) >= 1e4 {
		return errors.New("profit_min_margin 与 profit_safety_buffer 之和必须小于 1，否则将排除全部账号")
	}
	return nil
}

// NormalizeProfitControlConfig 归一化最终落库的利润控制配置，CreateGroup 与 UpdateGroup 共用（唯一收口）：
//   - 非五个平台分组不携带利润控制，一律重置为默认（关、0、0）；
//   - 支持平台关闭开关时保留合法数值（便于再次启用），清洗 NaN/Inf/越界脏值。
//
// 与 ValidateProfitControlConfig 的分工同高峰倍率：先归一化、后校验，
// 使"openai 转其他平台"这类更新能静默清空利润配置而不是被校验拒绝。
func NormalizeProfitControlConfig(platform string, enabled bool, minMargin, safetyBuffer float64) (bool, float64, float64) {
	if !profitControlPlatformSupported(platform) {
		return false, 0, 0
	}
	if !enabled {
		if !validProfitControlRatio(minMargin) || !validProfitControlRatio(math.Round(minMargin*1e4)/1e4) {
			minMargin = 0
		}
		if !validProfitControlRatio(safetyBuffer) || !validProfitControlRatio(math.Round(safetyBuffer*1e4)/1e4) {
			safetyBuffer = 0
		}
	}
	if validProfitControlRatio(minMargin) {
		minMargin = math.Round(minMargin*1e4) / 1e4
	}
	if validProfitControlRatio(safetyBuffer) {
		safetyBuffer = math.Round(safetyBuffer*1e4) / 1e4
	}
	return enabled, minMargin, safetyBuffer
}

func profitControlPlatformSupported(platform string) bool {
	switch platform {
	case PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformGrok, PlatformAntigravity:
		return true
	default:
		return false
	}
}
