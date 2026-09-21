package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseUsageErrorFilterTime_DateTimeKeepsSelectedMinute(t *testing.T) {
	parsed, err := parseUsageErrorFilterTime("2026-09-21T14:35:00", "Asia/Shanghai", true)

	require.NoError(t, err)
	require.Equal(t, "2026-09-21T14:35:00+08:00", parsed.Format(time.RFC3339))
}

func TestParseUsageErrorFilterTime_DateOnlyEndIncludesWholeDay(t *testing.T) {
	parsed, err := parseUsageErrorFilterTime("2026-09-21", "Asia/Shanghai", true)

	require.NoError(t, err)
	require.Equal(t, "2026-09-22T00:00:00+08:00", parsed.Format(time.RFC3339))
}
