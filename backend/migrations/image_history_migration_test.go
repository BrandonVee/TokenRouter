package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageHistoryMigrationKeepsOptInAndPrivateObjectMetadata(t *testing.T) {
	content, err := FS.ReadFile("260_image_history.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	// 用户保存选择必须默认关闭，升级不能自动留存已有用户的图片。
	require.Contains(t, sql, "save_image_history boolean not null default false")
	require.Contains(t, sql, "create table if not exists image_histories")
	require.Contains(t, sql, "user_id bigint not null references users(id) on delete cascade")
	require.Contains(t, sql, "api_key_id bigint references api_keys(id) on delete set null")
	require.Contains(t, sql, "object_key text not null unique")
	require.Contains(t, sql, "sha256 varchar(64) not null")
	require.Contains(t, sql, "idx_image_histories_user_created_at")

	// 数据库只保存私有对象键和元数据，不应重新引入公开 URL 或图片字节列。
	require.NotContains(t, sql, "preview_url")
	require.NotContains(t, sql, "public_url")
	require.NotContains(t, sql, "base64")
}
