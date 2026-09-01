package service

import (
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageHistorySaveWorkerPoolIsBounded(t *testing.T) {
	repo := &fakeImageHistoryRepository{enabled: true}
	store := newFakeImageHistoryObjectStore()
	store.putStarted = make(chan struct{})
	store.blockPut = make(chan struct{})
	svc := newTestImageHistoryService(repo, store)
	pool := NewImageHistorySaveWorkerPool(svc, &config.Config{ImageHistory: config.ImageHistoryConfig{
		Enabled:            true,
		WorkerCount:        1,
		QueueSize:          1,
		SaveTimeoutSeconds: 2,
	}})
	t.Cleanup(pool.Stop)

	job := SaveImageHistoryInput{UserID: 1, Images: []GeneratedImageCapture{{Base64: imageHistoryValidPNGBase64}}}
	require.True(t, pool.Submit(job))
	select {
	case <-store.putStarted:
	case <-time.After(time.Second):
		t.Fatal("转存 worker 未开始执行")
	}
	require.True(t, pool.Submit(job))
	require.False(t, pool.Submit(job))
}

func TestImageHistorySaveWorkerPoolExecutesQueuedSave(t *testing.T) {
	repo := &fakeImageHistoryRepository{enabled: true}
	store := newFakeImageHistoryObjectStore()
	svc := newTestImageHistoryService(repo, store)
	pool := NewImageHistorySaveWorkerPool(svc, &config.Config{ImageHistory: config.ImageHistoryConfig{
		Enabled:            true,
		WorkerCount:        1,
		QueueSize:          2,
		SaveTimeoutSeconds: 2,
	}})
	t.Cleanup(pool.Stop)

	require.True(t, pool.Submit(SaveImageHistoryInput{
		UserID: 2,
		Images: []GeneratedImageCapture{{Base64: imageHistoryValidPNGBase64}},
	}))
	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.records) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestImageHistorySaveWorkerPoolDisabledDoesNotAcceptJobs(t *testing.T) {
	pool := NewImageHistorySaveWorkerPool(NewImageHistoryService(nil, nil, nil), nil)
	t.Cleanup(pool.Stop)
	require.False(t, pool.Submit(SaveImageHistoryInput{
		UserID: 3,
		Images: []GeneratedImageCapture{{Base64: imageHistoryValidPNGBase64}},
	}))
	require.NotPanics(t, func() { pool.Stop() })
}
