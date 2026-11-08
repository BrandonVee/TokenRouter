//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	dbmigrations "github.com/BrandonVee/TokenRouter/migrations"
	"github.com/stretchr/testify/require"
)

// 新准入列与旧展示列必须独立，迁移重放不能把存量展示配置变成调用限制。
func TestMigration274PreservesDisplayConfig(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO groups(name, platform, models_list_config) VALUES ($1, 'openai', '{"enabled":true,"models":["display-only"]}') RETURNING id`, fmt.Sprintf("allowlist-migration-%d", time.Now().UnixNano())).Scan(&id))
	migration, err := dbmigrations.FS.ReadFile("274_group_model_allowlist.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	var display, policy string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT models_list_config::text, model_allowlist::text FROM groups WHERE id=$1`, id).Scan(&display, &policy))
	require.JSONEq(t, `{"enabled":true,"models":["display-only"]}`, display)
	require.JSONEq(t, `{}`, policy)
}

// ORM、认证专用查询及复合关系必须携带新策略；直改策略也要失效两类 Key。
func TestGroupModelAllowlistAuthProjectionAndInvalidation(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	repo := NewGroupRepository(integrationEntClient, integrationDB)
	g := &service.Group{Name: fmt.Sprintf("allowlist-%d", suffix), Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1,
		ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"display-only"}},
		ModelAllowlist:   service.GroupModelAllowlist{Enabled: true, Models: []string{"review"}}}
	require.NoError(t, repo.Create(ctx, g))
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("allowlist-%d@example.com", suffix)})
	keys := NewAPIKeyRepository(integrationEntClient, integrationDB)
	normal := &service.APIKey{UserID: user.ID, GroupID: &g.ID, Key: fmt.Sprintf("sk-allowlist-normal-%d", suffix), Name: "normal", Status: service.StatusActive}
	composite := &service.APIKey{UserID: user.ID, IsComposite: true, Key: fmt.Sprintf("sk-allowlist-composite-%d", suffix), Name: "composite", Status: service.StatusActive,
		CompositeGroups: []service.APIKeyCompositeGroup{{GroupID: g.ID, Prefix: "vip", NormalizedPrefix: "vip"}}}
	for _, key := range []*service.APIKey{normal, composite} {
		require.NoError(t, keys.Create(ctx, key))
	}
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE id IN ($1,$2)`, normal.ID, composite.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, g.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM auth_cache_invalidation_outbox WHERE cache_key IN (encode(sha256(convert_to($1,'UTF8')),'hex'),encode(sha256(convert_to($2,'UTF8')),'hex'))`, normal.Key, composite.Key)
		require.NoError(t, err)
	})
	for _, update := range []bool{false, true} {
		if update {
			g.ModelAllowlist.Models = []string{"review", "gpt-*"}
			require.NoError(t, repo.Update(ctx, g))
		}
		loaded, err := repo.GetByID(ctx, g.ID)
		require.NoError(t, err)
		require.Equal(t, g.ModelAllowlist, loaded.ModelAllowlist)
		require.Equal(t, g.ModelsListConfig, loaded.ModelsListConfig)
		for _, key := range []*service.APIKey{normal, composite} {
			auth, err := keys.GetByKeyForAuth(ctx, key.Key)
			require.NoError(t, err)
			group := auth.Group
			if key.IsComposite {
				require.Len(t, auth.CompositeGroups, 1)
				group = auth.CompositeGroups[0].Group
			}
			require.NotNil(t, group)
			require.Equal(t, g.ModelAllowlist, group.ModelAllowlist)
			require.Equal(t, g.ModelsListConfig, group.ModelsListConfig)
		}
	}
	_, err := integrationDB.ExecContext(ctx, `DELETE FROM auth_cache_invalidation_outbox WHERE cache_key IN (encode(sha256(convert_to($1,'UTF8')),'hex'),encode(sha256(convert_to($2,'UTF8')),'hex'))`, normal.Key, composite.Key)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET model_allowlist='{"enabled":true,"models":["new"]}' WHERE id=$1`, g.ID)
	require.NoError(t, err)
	for _, key := range []*service.APIKey{normal, composite} {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')`, key.Key).Scan(&count))
		require.Equal(t, 1, count)
	}
}
