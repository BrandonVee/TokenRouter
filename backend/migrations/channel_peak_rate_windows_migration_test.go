package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRestoreChannelPeakRateWindowsMigrationIsForwardCompatible(t *testing.T) {
	content, err := FS.ReadFile("261_restore_channel_peak_rate_windows.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	// 补偿迁移必须同时兼容缺列的历史数据库和已具备该列的新数据库。
	require.Contains(t, sql, "alter table channel_model_pricing")
	require.Contains(t, sql, "add column if not exists peak_rate_windows jsonb not null default '[]'::jsonb")
	require.Contains(t, sql, "comment on column channel_model_pricing.peak_rate_windows")
	require.NotContains(t, sql, "drop column")
}
