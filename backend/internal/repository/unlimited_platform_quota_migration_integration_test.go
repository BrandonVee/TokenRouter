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

// 全空配置只是额度镜像，清理不影响真实限额；显式零和正数都必须保留。
func TestMigration276OnlyPurgesUnlimitedQuotaRows(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("unlimited-%d@example.com", time.Now().UnixNano())})
	tx := testTx(t)
	_, err := tx.ExecContext(ctx, `INSERT INTO user_platform_quotas(user_id,platform,daily_limit_usd,daily_usage_usd) VALUES ($1,'openai',NULL,20),($1,'qoder',0,0),($1,'gemini',10,3)`, user.ID)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("276_purge_unlimited_user_platform_quotas.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_platform_quotas WHERE user_id=$1`, user.ID).Scan(&count))
	require.Equal(t, 2, count)
	var zero, positive float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT daily_limit_usd FROM user_platform_quotas WHERE user_id=$1 AND platform='qoder'`, user.ID).Scan(&zero))
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT daily_limit_usd FROM user_platform_quotas WHERE user_id=$1 AND platform='gemini'`, user.ID).Scan(&positive))
	require.Zero(t, zero)
	require.Equal(t, 10.0, positive)
}
