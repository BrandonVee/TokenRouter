//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/stretchr/testify/require"
)

// 第二次状态写入失败必须回滚首次延期；重新提交批次时去重，不能把失败项多延期一次。
func TestSubscriptionBulkForkRollbackAndRetry(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("bulk-sub-%d@example.com", suffix)})
	plan := mustCreatePlan(t, integrationEntClient, &service.SubscriptionPlan{Name: fmt.Sprintf("bulk-plan-%d", suffix), ValidityDays: 30, Price: 1})
	sub := mustCreateSubscription(t, integrationEntClient, &service.UserSubscription{UserID: user.ID, PlanID: plan.ID})
	repo := NewUserSubscriptionRepository(integrationEntClient)
	svc := service.NewSubscriptionService(NewGroupRepository(integrationEntClient, integrationDB), repo, nil, integrationEntClient, nil)
	stored, err := repo.GetByID(ctx, sub.ID)
	require.NoError(t, err)
	oldExpiry := stored.ExpiresAt
	function := fmt.Sprintf("reject_bulk_status_%d", suffix)
	trigger := fmt.Sprintf("reject_bulk_trigger_%d", suffix)
	_, err = integrationDB.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture second write failure'; RETURN NULL; END $$;
CREATE TRIGGER %s AFTER UPDATE OF status ON user_subscriptions FOR EACH ROW WHEN (NEW.id=%d) EXECUTE FUNCTION %s()`, function, trigger, sub.ID, function))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON user_subscriptions; DROP FUNCTION IF EXISTS %s()`, trigger, function))
		require.NoError(t, cleanupErr)
	})
	input := &service.BulkSubscriptionActionInput{SubscriptionIDs: []int64{sub.ID}, Action: "extend", Days: 7}
	result, err := svc.BulkSubscriptionAction(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1, result.FailedCount)
	stored, err = repo.GetByID(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, oldExpiry, stored.ExpiresAt)
	_, err = integrationDB.ExecContext(ctx, fmt.Sprintf(`DROP TRIGGER %s ON user_subscriptions`, trigger))
	require.NoError(t, err)
	input.SubscriptionIDs = []int64{sub.ID, sub.ID, 999999999}
	result, err = svc.BulkSubscriptionAction(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 1, result.FailedCount)
	require.Len(t, result.Results, 2)
	stored, err = repo.GetByID(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, oldExpiry.AddDate(0, 0, 7), stored.ExpiresAt)
}
