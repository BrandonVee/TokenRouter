package repository

import (
	"context"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/lib/pq"
)

// lockLiveGroups 锁定仍存在的目标分组，避免并发删除后新增孤立关联。
func lockLiveGroups(ctx context.Context, exec sqlExecutor, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		unique[id] = struct{}{}
	}
	rows, err := exec.QueryContext(ctx, `/* account_group_live_group_lock */
		SELECT id FROM groups
		WHERE id = ANY($1) AND deleted_at IS NULL
		ORDER BY id
		FOR SHARE`, pq.Array(groupIDs))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	locked := 0
	for rows.Next() {
		locked++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if locked != len(unique) {
		return service.ErrGroupNotFound
	}
	return nil
}
