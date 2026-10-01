//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyDirectionalRoutingSelectsActualAccount 覆盖实际选择入口，防止策略只改变展示分数。
func TestAPIKeyDirectionalRoutingSelectsActualAccount(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini} {
		for _, mode := range []GroupSchedulerType{GroupSchedulerTypeBasic, GroupSchedulerTypeAdvanced, "ungrouped"} {
			for _, strategy := range []string{APIKeyRoutingStrategySpeed, APIKeyRoutingStrategyPrice, APIKeyRoutingStrategySuccessRate} {
				t.Run(platform+"/"+string(mode)+"/"+strategy, func(t *testing.T) {
					cheap, expensive := 0.5, 1.5
					accounts := []Account{
						{ID: 1, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, Priority: 0, RateMultiplier: &cheap},
						{ID: 2, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, Priority: 100, RateMultiplier: &expensive},
					}
					group := advancedSchedulerRegressionGroup(9001, platform, advancedSchedulerRegressionOverrides())
					group.SchedulerType = mode
					// 放大反向优先级和粘性权重，明确目标仍应选中指标最佳的账号。
					group.AdvancedSchedulerOverrides.WeightPriority = advancedSchedulerRegressionFloat(1000)
					group.AdvancedSchedulerOverrides.WeightSessionSticky = advancedSchedulerRegressionFloat(1000)
					group.AdvancedSchedulerOverrides.StickyWeightedEnabled = advancedSchedulerRegressionBool(false)
					group.AdvancedSchedulerOverrides.SubscriptionPriorityEnabled = advancedSchedulerRegressionBool(true)
					var groupID *int64
					ctx := WithAPIKeyRoutingStrategy(context.Background(), strategy)
					if mode != "ungrouped" {
						groupID = &group.ID
						ctx = context.WithValue(ctx, ctxkey.Group, group)
						for i := range accounts {
							accounts[i].GroupIDs = []int64{group.ID}
						}
					}
					stats := newAdvancedAccountRuntimeStats()
					slow, fast := 2000, 100
					stats.report(1, false, &slow)
					stats.report(2, true, &fast)
					want, oldSticky := int64(2), int64(1)
					if strategy == APIKeyRoutingStrategyPrice {
						want, oldSticky = 1, 2
					}
					cfg := testConfig()
					cfg.Gateway.AdvancedScheduler.LBTopK = 1
					cfg.Gateway.Scheduling.LoadBatchEnabled = true
					if platform == PlatformOpenAI {
						svc := &OpenAIGatewayService{
							accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: cfg,
							cache:              &schedulerTestGatewayCache{sessionBindings: map[string]int64{"session": oldSticky}},
							concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}), openaiAccountStats: stats,
						}
						selection, _, err := svc.SelectAccountWithScheduler(ctx, groupID, "", "session", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
						require.NoError(t, err)
						require.NotNil(t, selection)
						require.Equal(t, want, selection.Account.ID)
						require.True(t, selection.AdvancedScheduler)
						if selection.ReleaseFunc != nil {
							selection.ReleaseFunc()
						}
					} else if platform == PlatformGemini {
						repo := &mockAccountRepoForGemini{accounts: accounts, accountsByID: map[int64]*Account{1: &accounts[0], 2: &accounts[1]}}
						svc := &GeminiMessagesCompatService{
							accountRepo: repo, cfg: cfg, advancedAccountStats: stats,
							groupRepo: &mockGroupRepoForGemini{groups: map[int64]*Group{group.ID: group}},
							cache:     &mockGatewayCacheForGemini{sessionBindings: map[string]int64{"gemini:session": oldSticky}},
						}
						account, err := svc.SelectAccountForModel(ctx, groupID, "session", "gemini-2.5-pro")
						require.NoError(t, err)
						require.NotNil(t, account)
						require.Equal(t, want, account.ID)
					} else {
						svc := &GatewayService{
							accountRepo: advancedSchedulerRegressionAccountRepo(accounts), cfg: cfg, advancedAccountStats: stats,
							groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{group.ID: group}},
							cache:              &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"session": oldSticky}},
							concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
						}
						selection, err := svc.SelectAccountWithLoadAwareness(ctx, groupID, "session", "claude-sonnet-4", nil, "", 0)
						require.NoError(t, err)
						require.NotNil(t, selection)
						require.Equal(t, want, selection.Account.ID)
						require.True(t, selection.AdvancedScheduler)
						if selection.ReleaseFunc != nil {
							selection.ReleaseFunc()
						}
					}
				})
			}
		}
	}
}

// TestAPIKeyPriceRoutingFallsBackBeyondTopK 最低成本账号满槽时应尝试其余账号，而非只等待第一名。
func TestAPIKeyPriceRoutingFallsBackBeyondTopK(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			cheap, expensive := 0.5, 1.5
			accounts := []Account{
				{ID: 1, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, RateMultiplier: &cheap},
				{ID: 2, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, RateMultiplier: &expensive},
			}
			ctx := WithAPIKeyRoutingStrategy(context.Background(), APIKeyRoutingStrategyPrice)
			cfg := testConfig()
			cfg.Gateway.AdvancedScheduler.LBTopK = 1
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			var selection *AccountSelectionResult
			var err error
			if platform == PlatformOpenAI {
				svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: cfg,
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{1: false, 2: true}})}
				selection, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
			} else {
				svc := &GatewayService{accountRepo: advancedSchedulerRegressionAccountRepo(accounts), cfg: cfg,
					concurrencyService: NewConcurrencyService(&mockConcurrencyCache{acquireResults: map[int64]bool{1: false, 2: true}})}
				selection, err = svc.SelectAccountWithLoadAwareness(ctx, nil, "", "claude-sonnet-4", nil, "", 0)
			}
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, int64(2), selection.Account.ID)
			require.True(t, selection.Acquired)
			require.Nil(t, selection.WaitPlan)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}

// TestAPIKeySmartRoutingSettings 保证手动模式继承管理员设置，自动模式不会被硬粘性短路。
func TestAPIKeySmartRoutingSettings(t *testing.T) {
	group := advancedSchedulerRegressionGroup(9002, PlatformOpenAI, advancedSchedulerRegressionOverrides())
	svc := &OpenAIGatewayService{cfg: testConfig()}
	manual := svc.advancedSchedulerEffectiveSettingsForGroup(context.Background(), group)
	require.Equal(t, 1, manual.topK)
	group.AdvancedSchedulerOverrides.StickyWeightedEnabled = advancedSchedulerRegressionBool(false)
	manual = svc.advancedSchedulerEffectiveSettingsForGroup(context.Background(), group)
	require.False(t, manual.stickyWeightedEnabled)
	auto := svc.advancedSchedulerEffectiveSettingsForGroup(WithAPIKeyRoutingStrategy(context.Background(), APIKeyRoutingStrategyAuto), group)
	require.True(t, auto.stickyWeightedEnabled)
	require.Equal(t, manual.weights, auto.weights)
	require.Equal(t, manual.topK, auto.topK)
	require.False(t, *group.AdvancedSchedulerOverrides.StickyWeightedEnabled, "请求级策略不能修改分组配置")
}

// TestAPIKeyRoutingKeepsImmovablePreviousResponse 不可迁移的响应续链必须保持上游账号绑定。
func TestAPIKeyRoutingKeepsImmovablePreviousResponse(t *testing.T) {
	ctx := WithAPIKeyRoutingStrategy(context.Background(), APIKeyRoutingStrategyPrice)
	groupID := int64(9003)
	cheap, expensive := 0.5, 1.5
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, GroupIDs: []int64{groupID}, RateMultiplier: &expensive},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, GroupIDs: []int64{groupID}, RateMultiplier: &cheap},
	}
	accounts[0].Extra = map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: newSchedulerTestOpenAIWSV2Config(),
		cache: &schedulerTestGatewayCache{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, groupID, "resp_bound", 1, stickySessionTTL))
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "resp_bound", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(1), selection.Account.ID)
	require.True(t, decision.StickyPreviousHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}
