package repository

import (
	"context"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPassiveWeightedAvailabilityRateUsesErrorWeights(t *testing.T) {
	noDataRate := passiveWeightedAvailabilityRate(0, 0, 0)
	require.NotNil(t, noDataRate)
	require.Equal(t, 1.0, *noDataRate)

	rate := passiveWeightedAvailabilityRate(9, 1, 10)
	require.NotNil(t, rate)
	require.InDelta(t, 0.875, *rate, 1e-9)
}

func TestGetPassiveSummaryBuildsSixtyTimeBuckets(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"group_id", "bucket_index", "success_count", "slow_stream_count",
		"total_count", "last_status", "last_checked_at",
	}).
		AddRow(int64(7), 58, int64(9), int64(1), int64(10), service.GroupAvailabilityRequestStatusSlowStream, now.Add(-2*time.Hour)).
		AddRow(int64(7), 59, int64(6), int64(0), int64(10), service.GroupAvailabilityRequestStatusUpstreamError, now.Add(-time.Hour))

	mock.ExpectQuery(`(?s)WITH events AS .*first_token_ms.*30000.*billing_mode.*image.*image_count.*bucket_index.*COUNT.*slow_stream_count.*GROUP BY group_id, bucket_index`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 120).
		WillReturnRows(rows)

	repo := &groupAvailabilityProbeRepository{db: db}
	summaries, err := repo.GetPassiveSummaryByGroupIDs(context.Background(), []int64{7}, 7, 120, "UTC", now)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	summary := summaries[7]
	require.NotNil(t, summary)
	require.Equal(t, int64(15), summary.SuccessCount)
	require.Equal(t, int64(0), summary.PressureCount)
	require.Equal(t, int64(20), summary.TotalCount)
	require.Len(t, summary.Days, service.PassiveAvailabilityBucketCount)
	require.Equal(t, int64(1), summary.Days[58].SlowStreamCount)
	require.InDelta(t, 0.875, *summary.Days[58].AvailabilityRate, 1e-9)
	require.InDelta(t, 0.6, *summary.Days[59].AvailabilityRate, 1e-9)
	require.NotNil(t, summary.AvailabilityRate)
	require.InDelta(t, 0.7375, *summary.AvailabilityRate, 1e-9)
}
