//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/adminaccess"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/BrandonVee/TokenRouter/internal/pkg/pagination"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/stretchr/testify/require"
)

// 真实迁移与仓储验证：共享关联保留、凭证补丁、白名单投影、回滚及撤销版本都必须同源。
func TestScopedAdminForkTransactionsAndRevocation(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	creator := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("scoped-admin-%d@example.com", suffix), Role: service.RoleAdmin})
	groups := NewAdminGroupRepository(integrationEntClient, integrationDB)
	accounts := NewAdminAccountRepository(integrationEntClient, integrationDB, nil)
	admin := service.NewAdminService(nil, groups, accounts, nil, nil, nil, nil, nil, nil, nil, nil, nil, integrationEntClient, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	g1 := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("scoped-visible-%d", suffix), Platform: service.PlatformOpenAI, RateMultiplier: 1})
	g2 := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("scoped-hidden-%d", suffix), Platform: service.PlatformOpenAI, RateMultiplier: 1})
	account, err := admin.CreateAccount(ctx, &service.CreateAccountInput{Name: fmt.Sprintf("scoped-shared-%d", suffix), Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "fixture-private-token"}, GroupIDs: []int64{g1.ID, g2.ID}, Concurrency: 1, SkipMixedChannelCheck: true})
	require.NoError(t, err)
	var originalTime time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT created_at FROM account_groups WHERE account_id=$1 AND group_id=$2`, account.ID, g2.ID).Scan(&originalTime))
	settings := service.NewSettingService(nil, nil)
	settings.SetScopedAdminKeyRepository(NewScopedAdminKeyRepository(integrationDB))
	policy := adminaccess.Policy{Rules: []adminaccess.Rule{{Permissions: []string{"accounts.read", "accounts.update", "accounts.credentials", "accounts.status", "accounts.groups", "accounts.models.write", "groups.models.write"}, GroupIDs: []int64{g1.ID}, AccountTypes: []string{service.AccountTypeAPIKey}, Platforms: []string{service.PlatformOpenAI}}}}
	secret, key, err := settings.CreateScopedAdminKey(ctx, service.ScopedAdminKeyInput{Name: "integration", Enabled: true, Policy: policy}, creator.ID)
	require.NoError(t, err)
	stored, err := settings.AuthenticateScopedAdminKey(ctx, secret, "127.0.0.1")
	require.NoError(t, err)
	require.NotEqual(t, secret, stored.Hash)
	require.NotEmpty(t, stored.Hash)
	p := &adminaccess.Principal{KeyID: stored.ID, KeyVersion: stored.UpdatedAt, CreatorID: creator.ID, Policy: stored.Policy}
	scopedCtx := adminaccess.WithPrincipal(ctx, p)
	api := admin.(service.ScopedAdminAccounts)

	updated, err := api.ScopedAdminWrite(scopedCtx, account.ID, "", map[string]json.RawMessage{"credentials": json.RawMessage(`{"model_mapping":{"review":"gpt-5.4"},"model_whitelist":["review"]}`)}, nil)
	require.NoError(t, err)
	require.Equal(t, "fixture-private-token", updated.GetCredential("api_key"))
	listed, _, err := api.ScopedAdminList(scopedCtx, pagination.PaginationParams{Page: 1, PageSize: 20}, "", "", "", account.Name, 0)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotContains(t, listed[0].Credentials, "api_key")
	require.Equal(t, []string{"review"}, listed[0].Credentials["model_whitelist"])

	_, err = api.ScopedAdminWrite(scopedCtx, account.ID, "", map[string]json.RawMessage{"group_ids": json.RawMessage(`[]`)}, nil)
	require.NoError(t, err)
	loaded, err := accounts.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{g2.ID}, loaded.GroupIDs)
	var keptTime time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT created_at FROM account_groups WHERE account_id=$1 AND group_id=$2`, account.ID, g2.ID).Scan(&keptTime))
	require.Equal(t, originalTime, keptTime)
	// 重新绑定供后续回滚与撤销检查；完整管理员保持 fork 原有操作能力。
	require.NoError(t, accounts.BindGroups(ctx, account.ID, []int64{g1.ID, g2.ID}))
	_, err = api.ScopedAdminWrite(scopedCtx, account.ID, "accounts.status", nil, func(txctx context.Context, current *service.Account) (*service.Account, error) {
		_, updateErr := admin.UpdateAccount(txctx, current.ID, &service.UpdateAccountInput{Status: "inactive"})
		require.NoError(t, updateErr)
		return nil, errors.New("fixture rollback")
	})
	require.ErrorContains(t, err, "fixture rollback")
	loaded, err = accounts.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusActive, loaded.Status)

	require.NoError(t, settings.RevokeScopedAdminKey(ctx, key.ID))
	_, err = api.ScopedAdminWrite(scopedCtx, account.ID, "", map[string]json.RawMessage{"status": json.RawMessage(`"inactive"`)}, nil)
	require.Equal(t, 401, infraerrors.Code(err), "认证后发生撤销也不得提交旧授权写入")
	_, err = groups.(interface {
		ScopedAdminUpdateGroupModels(context.Context, int64, service.GroupModelAllowlist) (*service.Group, error)
	}).ScopedAdminUpdateGroupModels(scopedCtx, g1.ID, service.GroupModelAllowlist{Enabled: true, Models: []string{"review"}})
	require.Equal(t, 401, infraerrors.Code(err))
}
