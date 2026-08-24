package repository

import (
	"context"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPassiveAvailabilityRateUsesPressureWeightAndMinimumSamples(t *testing.T) {
	require.Nil(t, passiveAvailabilityRate(9, 0, 9))

	rate := passiveAvailabilityRate(8, 1, 10)
	require.NotNil(t, rate)
	require.InDelta(t, 0.85, *rate, 1e-9)
}

func TestGetPassiveSummaryFiltersNoiseAndUsesLatestThreeHundredSamples(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"group_id", "status", "success", "created_at"})
	for i := 0; i < 8; i++ {
		rows.AddRow(int64(7), service.GroupAvailabilityProbeStatusSuccess, true, now.Add(time.Duration(i-10)*time.Minute))
	}
	rows.AddRow(int64(7), service.GroupAvailabilityRequestStatusPressure, false, now.Add(-2*time.Minute))
	rows.AddRow(int64(7), service.GroupAvailabilityRequestStatusUpstreamError, false, now.Add(-time.Minute))

	mock.ExpectQuery(`(?s)WITH events AS .*NOT COALESCE\(oe\.is_business_limited, false\).*NOT COALESCE\(oe\.is_count_tokens, false\).*ORDER BY source_priority DESC, created_at DESC, id DESC.*WHERE group_rank <= \$4`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), service.PassiveAvailabilitySampleLimit).
		WillReturnRows(rows)

	repo := &groupAvailabilityProbeRepository{db: db}
	summaries, err := repo.GetPassiveSummaryByGroupIDs(context.Background(), []int64{7}, 7, 120, "UTC", now)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	summary := summaries[7]
	require.NotNil(t, summary)
	require.Equal(t, int64(8), summary.SuccessCount)
	require.Equal(t, int64(1), summary.PressureCount)
	require.Equal(t, int64(10), summary.TotalCount)
	require.Len(t, summary.Requests, 10)
	require.NotNil(t, summary.AvailabilityRate)
	require.InDelta(t, 0.85, *summary.AvailabilityRate, 1e-9)
}
