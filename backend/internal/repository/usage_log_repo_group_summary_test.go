package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetAllGroupUsageSummaryRawFallbackIncludesYesterday(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	todayStart := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT source_oldest_at").
		WillReturnError(errors.New("aggregation unavailable"))
	mock.ExpectQuery("(?s)SELECT.*yesterday_cost.*FROM groups").
		WithArgs(todayStart, todayStart.AddDate(0, 0, -1)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "total_cost", "today_cost", "yesterday_cost"}).
			AddRow(7, 12.5, 3.25, 2.75))

	result, err := repo.GetAllGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(7), result[0].GroupID)
	require.InDelta(t, 12.5, result[0].TotalCost, 1e-9)
	require.InDelta(t, 3.25, result[0].TodayCost, 1e-9)
	require.InDelta(t, 2.75, result[0].YesterdayCost, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}
