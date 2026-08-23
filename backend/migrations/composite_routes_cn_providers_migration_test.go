package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompositeRoutesCNProvidersMigration(t *testing.T) {
	content, err := FS.ReadFile("253_composite_model_routes.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CONSTRAINT composite_model_routes_target_platform_check")
	require.Contains(t, sql,
		"CHECK (target_platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek'))")
}
