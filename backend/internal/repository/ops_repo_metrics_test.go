package repository

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryInsertSystemMetricsPreservesZeroDBPoolCounts(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	createdAt := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	active := 0
	idle := 10

	args := make([]driver.Value, 43)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	// 参数索引 38、39、40 分别对应活跃、空闲和等待连接数，等待连接缺失时仍应写入 NULL。
	args[38] = int64(0)
	args[39] = int64(10)
	args[40] = nil

	mock.ExpectExec("INSERT INTO ops_system_metrics").
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.InsertSystemMetrics(context.Background(), &service.OpsInsertSystemMetricsInput{
		CreatedAt:     createdAt,
		WindowMinutes: 1,
		DBConnActive:  &active,
		DBConnIdle:    &idle,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInsertSystemMetricsNullableIntegerMetrics 验证零值指标按有效观测值写入，
// 只有指针缺失时才写 NULL（移植上游 9ddf69698）。
func TestInsertSystemMetricsNullableIntegerMetrics(t *testing.T) {
	createdAt := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	zero := 0

	tests := []struct {
		name         string
		dbConnActive *int
		wantDBActive driver.Value
	}{
		{name: "explicit zero is preserved", dbConnActive: &zero, wantDBActive: int64(0)},
		{name: "unavailable metric remains null", dbConnActive: nil, wantDBActive: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newSQLMock(t)

			args := make([]driver.Value, 43)
			for i := range args {
				args[i] = sqlmock.AnyArg()
			}
			args[0] = createdAt
			// 时延分位、TTFT、goroutine 数等零值都必须写入 0 而不是 NULL。
			args[15] = int64(0) // duration_p50_ms
			args[20] = int64(0) // duration_max_ms
			args[21] = int64(0) // ttft_p50_ms
			args[26] = int64(0) // ttft_max_ms
			args[36] = int64(0) // redis_conn_total
			args[38] = tt.wantDBActive
			args[41] = int64(0) // goroutine_count

			mock.ExpectExec("INSERT INTO ops_system_metrics").
				WithArgs(args...).
				WillReturnResult(sqlmock.NewResult(1, 1))

			repo := &opsRepository{db: db}
			err := repo.InsertSystemMetrics(context.Background(), &service.OpsInsertSystemMetricsInput{
				CreatedAt:      createdAt,
				DurationP50Ms:  &zero,
				DurationMaxMs:  &zero,
				TTFTP50Ms:      &zero,
				TTFTMaxMs:      &zero,
				RedisConnTotal: &zero,
				DBConnActive:   tt.dbConnActive,
				GoroutineCount: &zero,
			})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
