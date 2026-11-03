package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// retentionFailingStore 模拟云端删除故障，验证失败后仍保留元数据以便重试。
type retentionFailingStore struct {
	*fakeImageHistoryObjectStore
	failKey string
}

func (s *retentionFailingStore) Delete(ctx context.Context, key string) error {
	if key == s.failKey {
		return errors.New("cloud delete failed")
	}
	return s.fakeImageHistoryObjectStore.Delete(ctx, key)
}

// TestImageHistoryRetentionCleansCloudAndMetadata 覆盖七天边界与不同用户的统一清理。
func TestImageHistoryRetentionCleansCloudAndMetadata(t *testing.T) {
	now := time.Now()
	repo := &fakeImageHistoryRepository{enabled: false, records: []ImageHistoryRecord{
		{ID: "a", UserID: 1, ObjectKey: "expired", CreatedAt: now.Add(-7 * 24 * time.Hour)},
		{ID: "b", UserID: 2, ObjectKey: "recent", CreatedAt: now.Add(-6 * 24 * time.Hour)},
	}}
	store := newFakeImageHistoryObjectStore()
	store.objects["expired"] = []byte("old")
	store.objects["recent"] = []byte("new")
	svc := newTestImageHistoryService(repo, store)

	require.NoError(t, svc.CleanupExpired(context.Background(), now))
	require.Equal(t, []string{"a"}, repo.deletedIDs)
	require.Equal(t, []string{"expired"}, store.deletedKeys)
	require.Len(t, repo.records, 1)
	require.Contains(t, store.objects, "recent")
}

// TestImageHistoryRetentionRetriesWithoutStarvingLaterBatches 覆盖超出单批的过期记录和云端失败重试。
func TestImageHistoryRetentionRetriesWithoutStarvingLaterBatches(t *testing.T) {
	now := time.Now()
	repo := &fakeImageHistoryRepository{}
	store := &retentionFailingStore{fakeImageHistoryObjectStore: newFakeImageHistoryObjectStore(), failKey: "image-000"}
	for i := 0; i < imageHistoryRetentionBatch+1; i++ {
		id := fmt.Sprintf("image-%03d", i)
		repo.records = append(repo.records, ImageHistoryRecord{ID: id, UserID: 1, ObjectKey: id, CreatedAt: now.Add(-8 * 24 * time.Hour)})
		store.objects[id] = []byte("image")
	}
	svc := newTestImageHistoryService(repo, store)
	require.ErrorContains(t, svc.CleanupExpired(context.Background(), now), "cloud delete failed")
	require.Len(t, repo.records, 1)
	require.Equal(t, store.failKey, repo.records[0].ObjectKey)
	require.Contains(t, store.objects, store.failKey)

	store.failKey = ""
	require.NoError(t, svc.CleanupExpired(context.Background(), now))
	require.Empty(t, repo.records)
	require.Empty(t, store.objects)
}

// TestImageHistoryRetentionDeniesExpiredContent 过期即不可列出或下载，不依赖后台清理恰好执行。
func TestImageHistoryRetentionDeniesExpiredContent(t *testing.T) {
	repo := &fakeImageHistoryRepository{records: []ImageHistoryRecord{{ID: "expired", UserID: 7, ObjectKey: "expired", CreatedAt: time.Now().Add(-8 * 24 * time.Hour)}}}
	store := newFakeImageHistoryObjectStore()
	store.objects["expired"] = []byte("image")
	svc := newTestImageHistoryService(repo, store)
	list, err := svc.List(context.Background(), 7, ImageHistoryListParams{})
	require.NoError(t, err)
	require.Zero(t, list.Total)
	require.Empty(t, list.Items)
	require.Empty(t, store.presignedKeys)
	_, err = svc.OpenContent(context.Background(), 7, "expired")
	require.ErrorIs(t, err, ErrImageHistoryNotFound)
	require.Contains(t, store.objects, "expired")
}

// TestImageHistoryRetentionWorkerStartsWithPool 确保启动时清理且随 worker 一起退出。
func TestImageHistoryRetentionWorkerStartsWithPool(t *testing.T) {
	repo := &fakeImageHistoryRepository{records: []ImageHistoryRecord{{ID: "old", UserID: 1, ObjectKey: "old", CreatedAt: time.Now().Add(-8 * 24 * time.Hour)}}}
	store := newFakeImageHistoryObjectStore()
	store.objects["old"] = []byte("image")
	pool := NewImageHistorySaveWorkerPool(newTestImageHistoryService(repo, store), nil)
	t.Cleanup(pool.Stop)
	require.Eventually(t, func() bool { repo.mu.Lock(); defer repo.mu.Unlock(); return len(repo.records) == 0 }, time.Second, 10*time.Millisecond)
	pool.Stop()
	require.Empty(t, store.objects)
}

// retentionCancellingStore 模拟云端删除耗尽单轮预算，不阻塞后续周期清理其它对象。
type retentionCancellingStore struct {
	*fakeImageHistoryObjectStore
	cancel context.CancelFunc
}

func (s *retentionCancellingStore) Delete(ctx context.Context, key string) error {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		return ctx.Err()
	}
	return s.fakeImageHistoryObjectStore.Delete(ctx, key)
}

func TestImageHistoryRetentionResumesAfterTimeout(t *testing.T) {
	now := time.Now()
	repo := &fakeImageHistoryRepository{records: []ImageHistoryRecord{
		{ID: "a", UserID: 1, ObjectKey: "a", CreatedAt: now.Add(-8 * 24 * time.Hour)},
		{ID: "b", UserID: 1, ObjectKey: "b", CreatedAt: now.Add(-8 * 24 * time.Hour)},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &retentionCancellingStore{fakeImageHistoryObjectStore: newFakeImageHistoryObjectStore(), cancel: cancel}
	store.objects["a"] = []byte("a")
	store.objects["b"] = []byte("b")
	svc := newTestImageHistoryService(repo, store)
	require.ErrorIs(t, svc.CleanupExpired(ctx, now), context.Canceled)
	require.NoError(t, svc.CleanupExpired(context.Background(), now))
	require.Len(t, repo.records, 1)
	require.Equal(t, "a", repo.records[0].ID)
	require.NoError(t, svc.CleanupExpired(context.Background(), now))
	require.Empty(t, repo.records)
}
