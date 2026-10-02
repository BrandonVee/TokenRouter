//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/BrandonVee/TokenRouter/internal/service"
)

const (
	// 已受理请求在结算前被删除 Key 时，必须仍能按原 ID 更新保留行完成整笔结算：
	// WHERE 不再过滤 deleted_at，但状态改写与“新耗尽”判定都要排除已删除记录。
	incrementAPIKeyQuotaSQL  = `(?s)UPDATE api_keys\s+SET quota_used = quota_used \+ \$1,.*WHEN deleted_at IS NULL AND quota > 0.*WHERE id = \$2\s+RETURNING deleted_at IS NULL AND quota > 0`
	incrementAPIKeyWindowSQL = `(?s)UPDATE api_keys SET\s+usage_5h = .*WHERE id = \$2\s*$`
)

// 软删除 Key 的用量更新不再被 deleted_at 过滤，但已删除记录不能被改状态或报为新耗尽。
func TestIncrementUsageBillingAPIKeyQuota_IncludesSoftDeletedRow(t *testing.T) {
	dbFailure := errors.New("database unavailable")
	for _, tc := range []struct {
		name     string
		rows     *sqlmock.Rows
		queryErr error
		want     bool
		wantErr  error
	}{
		{name: "deleted_row_not_exhausted", rows: sqlmock.NewRows([]string{"?column?"}).AddRow(false), want: false},
		{name: "active_row_exhausted", rows: sqlmock.NewRows([]string{"?column?"}).AddRow(true), want: true},
		{name: "missing_row", queryErr: sql.ErrNoRows, wantErr: service.ErrAPIKeyNotFound},
		{name: "database_error", queryErr: dbFailure, wantErr: dbFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			mock.ExpectBegin()
			tx, err := db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			query := mock.ExpectQuery(incrementAPIKeyQuotaSQL).
				WithArgs(1.25, int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted)
			if tc.queryErr != nil {
				query.WillReturnError(tc.queryErr)
			} else {
				query.WillReturnRows(tc.rows)
			}
			mock.ExpectCommit()

			exhausted, err := incrementUsageBillingAPIKeyQuota(context.Background(), tx, 7, 1.25)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.False(t, exhausted)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, exhausted)
			}
			require.NoError(t, tx.Commit())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 滚动窗口用量同样按 ID 命中保留行；真正缺失的行仍必须报错并让整笔事务回滚。
func TestIncrementUsageBillingAPIKeyRateLimit_IncludesSoftDeletedRow(t *testing.T) {
	dbFailure := errors.New("database unavailable")
	for _, tc := range []struct {
		name     string
		affected int64
		execErr  error
		wantErr  error
	}{
		{name: "deleted_row_updated", affected: 1},
		{name: "missing_row", affected: 0, wantErr: service.ErrAPIKeyNotFound},
		{name: "database_error", execErr: dbFailure, wantErr: dbFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			mock.ExpectBegin()
			tx, err := db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			exec := mock.ExpectExec(incrementAPIKeyWindowSQL).WithArgs(1.25, int64(7))
			if tc.execErr != nil {
				exec.WillReturnError(tc.execErr)
			} else {
				exec.WillReturnResult(sqlmock.NewResult(0, tc.affected))
			}
			mock.ExpectCommit()

			err = incrementUsageBillingAPIKeyRateLimit(context.Background(), tx, 7, 1.25)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, tx.Commit())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
