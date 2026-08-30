package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/timezone"
)

func init() {
	// 测试固定全局时区为 UTC，确保判定可复现。
	_ = timezone.Init("UTC")
}

func newPeakGroup(enabled bool, start, end string, mult float64) *Group {
	return &Group{
		PeakRateEnabled:    enabled,
		PeakStart:          start,
		PeakEnd:            end,
		PeakRateMultiplier: mult,
	}
}

func at(hour, min int) time.Time {
	return time.Date(2026, 6, 29, hour, min, 0, 0, time.UTC)
}

func TestPeakMultiplierAt_DisabledOrUnconfigured(t *testing.T) {
	cases := []struct {
		name string
		g    *Group
	}{
		{"disabled", newPeakGroup(false, "14:00", "18:00", 3.0)},
		{"empty start", newPeakGroup(true, "", "18:00", 3.0)},
		{"empty end", newPeakGroup(true, "14:00", "", 3.0)},
		{"invalid start>=end", newPeakGroup(true, "18:00", "14:00", 3.0)},
		{"equal start==end", newPeakGroup(true, "14:00", "14:00", 3.0)},
		{"malformed start", newPeakGroup(true, "99:99", "18:00", 3.0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.g.PeakMultiplierAt(at(15, 0)); got != 1.0 {
				t.Fatalf("expect 1.0, got %v", got)
			}
		})
	}
}

func TestPeakMultiplierAt_NilReceiver(t *testing.T) {
	var g *Group
	if got := g.PeakMultiplierAt(at(15, 0)); got != 1.0 {
		t.Fatalf("expect 1.0, got %v", got)
	}
}

func TestPeakMultiplierAt_Boundaries(t *testing.T) {
	g := newPeakGroup(true, "14:00", "18:00", 3.0)
	cases := []struct {
		t    time.Time
		want float64
	}{
		{at(13, 59), 1.0},
		{at(14, 0), 3.0},
		{at(15, 30), 3.0},
		{at(17, 59), 3.0},
		{at(18, 0), 1.0},
		{at(23, 0), 1.0},
	}
	for _, c := range cases {
		t.Run(c.t.Format("15:04"), func(t *testing.T) {
			if got := g.PeakMultiplierAt(c.t); got != c.want {
				t.Fatalf("at %s: expect %v, got %v", c.t.Format("15:04"), c.want, got)
			}
		})
	}
}

func TestPeakMultiplierAt_RespectsTimezoneLocation(t *testing.T) {
	// 全局时区为 UTC。北京 15:00 = UTC 07:00，不在 [14:00,18:00)。
	nonUTC := time.Date(2026, 6, 29, 15, 0, 0, 0, mustLoad("Asia/Shanghai"))
	g := newPeakGroup(true, "14:00", "18:00", 3.0)
	if got := g.PeakMultiplierAt(nonUTC); got != 1.0 {
		t.Fatalf("expect 1.0 (converted to UTC 07:00), got %v", got)
	}
}

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func TestValidatePeakRateConfig(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		start   string
		end     string
		mult    float64
		wantErr bool
	}{
		{"disabled passes through", false, "", "", 0, false},
		{"enabled valid", true, "14:00", "18:00", 3.0, false},
		{"enabled valid single digit hour", true, "1:00", "2:00", 3.0, false},
		{"enabled empty start", true, "", "18:00", 1.0, true},
		{"enabled empty end", true, "14:00", "", 1.0, true},
		{"enabled malformed start", true, "99:99", "18:00", 1.0, true},
		{"enabled malformed end", true, "14:00", "25:00", 1.0, true},
		{"enabled equal start==end", true, "14:00", "14:00", 1.0, true},
		{"enabled cross-day rejected", true, "22:00", "02:00", 1.0, true},
		{"enabled negative multiplier", true, "14:00", "18:00", -0.5, true},
		{"enabled zero multiplier allowed", true, "14:00", "18:00", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePeakRateConfig(c.enabled, c.start, c.end, c.mult)
			if c.wantErr && err == nil {
				t.Fatalf("expect error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
		})
	}
}

func TestChannelModelPricingPeakMultiplierAt(t *testing.T) {
	pricing := &ChannelModelPricing{
		PeakRateEnabled:    true,
		PeakStart:          "14:00",
		PeakEnd:            "18:00",
		PeakRateMultiplier: 2.5,
	}
	if got := pricing.PeakMultiplierAt(at(14, 0)); got != 2.5 {
		t.Fatalf("高峰开始时倍率 = %v，期望 2.5", got)
	}
	if got := pricing.PeakMultiplierAt(at(18, 0)); got != 1.0 {
		t.Fatalf("高峰结束时倍率 = %v，期望 1.0", got)
	}
	if pricing.HasExplicitPriceFields() {
		t.Fatal("仅峰谷配置不能被视为显式基础价格")
	}
}

func TestChannelModelPricingPeakMultiplierAt_WeeklyWindows(t *testing.T) {
	pricing := &ChannelModelPricing{
		PeakRateEnabled: true,
		PeakRateWindows: []PeakRateWindow{
			{Weekdays: []int{0, 2}, Start: "09:00", End: "12:00", Multiplier: 1.8},
			{Weekdays: []int{4}, Start: "22:00", End: "02:00", Multiplier: 2.5},
		},
	}
	cases := []struct {
		name string
		at   time.Time
		want float64
	}{
		{"monday configured", time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC), 1.8},
		{"tuesday excluded", time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC), 1.0},
		{"wednesday configured", time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), 1.8},
		{"friday before midnight", time.Date(2026, 7, 3, 23, 0, 0, 0, time.UTC), 2.5},
		{"saturday after midnight", time.Date(2026, 7, 4, 1, 0, 0, 0, time.UTC), 2.5},
		{"saturday end boundary", time.Date(2026, 7, 4, 2, 0, 0, 0, time.UTC), 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pricing.PeakMultiplierAt(tc.at); got != tc.want {
				t.Fatalf("倍率 = %v，期望 %v", got, tc.want)
			}
		})
	}
}

func TestValidatePeakRateWindows(t *testing.T) {
	validWindows := []PeakRateWindow{
		{Weekdays: []int{0, 1, 2, 3, 4}, Start: "09:00", End: "18:00", Multiplier: 1.5},
		{Weekdays: []int{5, 6}, Start: "22:00", End: "02:00", Multiplier: 0.8},
	}
	if err := ValidatePeakRateWindows(validWindows); err != nil {
		t.Fatalf("有效配置返回错误: %v", err)
	}

	cases := []struct {
		name    string
		windows []PeakRateWindow
	}{
		{"empty", nil},
		{"no weekday", []PeakRateWindow{{Start: "09:00", End: "18:00", Multiplier: 1}}},
		{"duplicate weekday", []PeakRateWindow{{Weekdays: []int{0, 0}, Start: "09:00", End: "18:00", Multiplier: 1}}},
		{"invalid weekday", []PeakRateWindow{{Weekdays: []int{7}, Start: "09:00", End: "18:00", Multiplier: 1}}},
		{"same time", []PeakRateWindow{{Weekdays: []int{0}, Start: "09:00", End: "09:00", Multiplier: 1}}},
		{"same day overlap", []PeakRateWindow{
			{Weekdays: []int{0}, Start: "09:00", End: "12:00", Multiplier: 1},
			{Weekdays: []int{0}, Start: "11:00", End: "13:00", Multiplier: 1},
		}},
		{"overnight overlap", []PeakRateWindow{
			{Weekdays: []int{6}, Start: "22:00", End: "02:00", Multiplier: 1},
			{Weekdays: []int{0}, Start: "01:00", End: "03:00", Multiplier: 1},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePeakRateWindows(tc.windows); err == nil {
				t.Fatal("期望校验错误")
			}
		})
	}
}

func TestChannelModelPeakRateDoesNotSatisfyPriceMultiplierValidation(t *testing.T) {
	priceMultiplier := 1.2
	err := validatePricingBillingMode([]ChannelModelPricing{{
		BillingMode:        BillingModeToken,
		PeakRateEnabled:    true,
		PeakStart:          "14:00",
		PeakEnd:            "18:00",
		PeakRateMultiplier: 2,
		PriceMultiplier:    &priceMultiplier,
	}})
	if err == nil {
		t.Fatal("仅峰谷配置不应满足普通价格倍率的显式价格校验")
	}
}

func TestChannelModelPeakRateAffectsOnlyTokenCost(t *testing.T) {
	billing := NewBillingService(nil, nil)
	resolver := NewModelPricingResolver(nil, billing)
	pricing := &ChannelModelPricing{
		PeakRateEnabled:    true,
		PeakStart:          "14:00",
		PeakEnd:            "18:00",
		PeakRateMultiplier: 2,
	}
	resolved := &ResolvedPricing{
		Mode:           BillingModeToken,
		Source:         PricingSourceChannel,
		BasePricing:    &ModelPricing{InputPricePerToken: 1e-6, OutputPricePerToken: 2e-6},
		channelPricing: pricing,
	}
	baseInput := CostInput{
		Ctx:            context.Background(),
		Model:          "test-model",
		Tokens:         UsageTokens{InputTokens: 100, OutputTokens: 50},
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	}
	lowInput := baseInput
	lowInput.PricingAt = at(13, 59)
	low, err := billing.CalculateCostUnified(lowInput)
	if err != nil {
		t.Fatalf("计算低谷价格失败: %v", err)
	}
	peakInput := baseInput
	peakInput.PricingAt = at(14, 0)
	peak, err := billing.CalculateCostUnified(peakInput)
	if err != nil {
		t.Fatalf("计算高峰价格失败: %v", err)
	}
	if math.Abs(peak.TotalCost-low.TotalCost*2) > 1e-12 {
		t.Fatalf("高峰 token 价格 = %v，期望低谷价格 %v 的 2 倍", peak.TotalCost, low.TotalCost)
	}

	perRequest := &ResolvedPricing{
		Mode:                   BillingModePerRequest,
		Source:                 PricingSourceChannel,
		DefaultPerRequestPrice: 3,
		channelPricing:         pricing,
	}
	requestCost, err := billing.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "test-model", RequestCount: 1, RateMultiplier: 1,
		PricingAt: at(14, 0), Resolver: resolver, Resolved: perRequest,
	})
	if err != nil {
		t.Fatalf("计算按次价格失败: %v", err)
	}
	if math.Abs(requestCost.TotalCost-3) > 1e-12 {
		t.Fatalf("按次价格 = %v，期望 3", requestCost.TotalCost)
	}
}

func TestParseMinutesMatchesLegacyTimeParseShape(t *testing.T) {
	cases := []struct {
		value string
		want  int
		ok    bool
	}{
		{"0:00", 0, true},
		{"00:00", 0, true},
		{"1:30", 90, true},
		{"01:30", 90, true},
		{"23:59", 1439, true},
		{"001:30", 0, false},
		{"9:3", 0, false},
		{"24:00", 0, false},
		{"1:030", 0, false},
		{" 1:30", 0, false},
		{"1:30 ", 0, false},
	}

	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			got, ok := parseMinutes(c.value)
			if ok != c.ok {
				t.Fatalf("ok: got %v, want %v", ok, c.ok)
			}
			if ok && got != c.want {
				t.Fatalf("minutes: got %v, want %v", got, c.want)
			}
		})
	}
}

func TestNormalizePeakRateConfig(t *testing.T) {
	enabled, start, end, multiplier := NormalizePeakRateConfig(false, "bad", "18:00", -2)
	if enabled || start != "" || end != "18:00" || multiplier != 1.0 {
		t.Fatalf("disabled cleanup mismatch: enabled=%v start=%q end=%q multiplier=%v", enabled, start, end, multiplier)
	}

	enabled, start, end, multiplier = NormalizePeakRateConfig(false, "14:00", "18:00", 3)
	if enabled || start != "14:00" || end != "18:00" || multiplier != 3 {
		t.Fatalf("disabled valid config should be preserved: enabled=%v start=%q end=%q multiplier=%v", enabled, start, end, multiplier)
	}

	enabled, start, end, multiplier = NormalizePeakRateConfig(true, "bad", "18:00", -2)
	if !enabled || start != "bad" || end != "18:00" || multiplier != -2 {
		t.Fatalf("enabled config should be left for validation: enabled=%v start=%q end=%q multiplier=%v", enabled, start, end, multiplier)
	}
}

func TestPeakMultiplierAt_EnabledGroupUsesConfiguredWindow(t *testing.T) {
	g := newPeakGroup(true, "14:00", "18:00", 3.0)
	if got := g.PeakMultiplierAt(at(15, 30)); got != 3.0 {
		t.Fatalf("enabled group peak multiplier: got %v, want 3.0", got)
	}
}

// TestPeakMultiplier_GatewayBillingSequence 调用 gateway_service.recordUsageCore 与
// openai_gateway_service.RecordUsage 共用的 computePeakAwareMultipliers，验证计费叠加顺序：
// 图片按次倍率基于基础倍率算出且不受高峰影响，高峰因子只乘入 token 倍率。
// 若有人调换叠加顺序或把高峰并入 imageMultiplier，此测试会失败。
func TestPeakMultiplier_GatewayBillingSequence(t *testing.T) {
	const baseMultiplier = 0.8
	apiKey := &APIKey{Group: newPeakGroup(true, "14:00", "18:00", 3.0)}
	approxEq := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

	t.Run("peak hour amplifies token multiplier only", func(t *testing.T) {
		now := at(15, 30) // 处于 [14:00, 18:00)
		tokenMultiplier, imageMultiplier := computePeakAwareMultipliers(apiKey, baseMultiplier, now)
		if !approxEq(imageMultiplier, baseMultiplier) {
			t.Fatalf("image multiplier must not be affected by peak: got %v, want %v", imageMultiplier, baseMultiplier)
		}
		if want := baseMultiplier * 3.0; !approxEq(tokenMultiplier, want) {
			t.Fatalf("token multiplier should include peak factor: got %v, want %v", tokenMultiplier, want)
		}
	})

	t.Run("off-peak leaves both multipliers at base", func(t *testing.T) {
		now := at(20, 0)
		tokenMultiplier, imageMultiplier := computePeakAwareMultipliers(apiKey, baseMultiplier, now)
		if !approxEq(imageMultiplier, baseMultiplier) {
			t.Fatalf("image multiplier: got %v, want %v", imageMultiplier, baseMultiplier)
		}
		if !approxEq(tokenMultiplier, baseMultiplier) {
			t.Fatalf("token multiplier should equal base off-peak: got %v, want %v", tokenMultiplier, baseMultiplier)
		}
	})

	t.Run("image independent mode decoupled from peak", func(t *testing.T) {
		indGroup := newPeakGroup(true, "14:00", "18:00", 3.0)
		indGroup.ImageRateIndependent = true
		indGroup.ImageRateMultiplier = 0.5
		indKey := &APIKey{Group: indGroup}
		now := at(15, 30)
		tokenMultiplier, imageMultiplier := computePeakAwareMultipliers(indKey, baseMultiplier, now)
		if !approxEq(imageMultiplier, 0.5) {
			t.Fatalf("independent image multiplier: got %v, want 0.5", imageMultiplier)
		}
		if want := baseMultiplier * 3.0; !approxEq(tokenMultiplier, want) {
			t.Fatalf("token multiplier should include peak factor: got %v, want %v", tokenMultiplier, want)
		}
	})

	t.Run("nil api key degrades to base multipliers", func(t *testing.T) {
		now := at(15, 30)
		tokenMultiplier, imageMultiplier := computePeakAwareMultipliers(nil, baseMultiplier, now)
		if !approxEq(tokenMultiplier, baseMultiplier) {
			t.Fatalf("nil group token multiplier: got %v, want %v", tokenMultiplier, baseMultiplier)
		}
		if !approxEq(imageMultiplier, baseMultiplier) {
			t.Fatalf("nil group image multiplier: got %v, want %v", imageMultiplier, baseMultiplier)
		}
	})
}

// TestPeakMultiplier_SnapshotRoundTrip 防回归：认证缓存快照（APIKeyAuthGroupSnapshot）
// 必须携带高峰倍率 4 字段，否则扣费路径拿到的 apiKey.Group 会缺字段、PeakMultiplierAt 恒降级为 1.0。
// 调用真实链路 snapshotFromAPIKey → snapshotToAPIKey，验证 peak 配置经快照往返后仍生效。
func TestPeakMultiplier_SnapshotRoundTrip(t *testing.T) {
	apiKey := &APIKey{
		User:  &User{ID: 1, Status: StatusActive, Role: RoleUser},
		Group: newPeakGroup(true, "14:00", "18:00", 3.0),
	}
	svc := &APIKeyService{}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	if snapshot == nil || snapshot.Group == nil {
		t.Fatalf("snapshot or snapshot.Group must not be nil")
	}
	restored := svc.snapshotToAPIKey("k", snapshot)
	if restored.Group == nil {
		t.Fatalf("restored.Group must not be nil")
	}

	if !restored.Group.PeakRateEnabled ||
		restored.Group.PeakStart != "14:00" ||
		restored.Group.PeakEnd != "18:00" ||
		restored.Group.PeakRateMultiplier != 3.0 {
		t.Fatalf("peak fields lost in snapshot round-trip: %+v", restored.Group)
	}
	if got := restored.Group.PeakMultiplierAt(at(15, 30)); got != 3.0 {
		t.Fatalf("peak hour multiplier after round-trip: got %v, want 3.0", got)
	}
	if got := restored.Group.PeakMultiplierAt(at(20, 0)); got != 1.0 {
		t.Fatalf("off-peak multiplier after round-trip: got %v, want 1.0", got)
	}
}
