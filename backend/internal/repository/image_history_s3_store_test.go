package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageHistoryS3StorePrivateObjectLifecycle(t *testing.T) {
	payload := []byte("image-bytes")
	var mu sync.Mutex
	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.Method {
		case http.MethodHead:
			// 连接测试只检查桶，不访问具体对象。
			require.Equal(t, "/history-bucket", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			require.Equal(t, "/history-bucket/image-history/7/test.png", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, payload, body)
			require.Equal(t, "image/png", r.Header.Get("Content-Type"))
			w.Header().Set("ETag", `"stored"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			require.Equal(t, "/history-bucket/image-history/7/test.png", r.URL.Path)
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", "11")
			_, _ = w.Write(payload)
		case http.MethodDelete:
			require.Equal(t, "/history-bucket/image-history/7/test.png", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	store, err := NewImageHistoryS3Store(&config.Config{ImageHistory: config.ImageHistoryConfig{
		Enabled:         true,
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "history-bucket",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	}})
	require.NoError(t, err)
	require.NotNil(t, store)

	ctx := context.Background()
	key := "image-history/7/test.png"
	tester, ok := store.(interface{ TestConnection(context.Context) error })
	require.True(t, ok)
	require.NoError(t, tester.TestConnection(ctx))
	require.NoError(t, store.Put(ctx, key, "image/png", payload))
	body, err := store.Open(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.NoError(t, body.Close())
	require.Equal(t, payload, got)

	presigned, err := store.PresignGet(ctx, key, 5*time.Minute)
	require.NoError(t, err)
	require.Contains(t, presigned, "/history-bucket/image-history/7/test.png")
	require.Contains(t, strings.ToLower(presigned), "x-amz-signature=")
	require.NoError(t, store.Delete(ctx, key))

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{
		"HEAD /history-bucket",
		"PUT /history-bucket/image-history/7/test.png",
		"GET /history-bucket/image-history/7/test.png",
		"DELETE /history-bucket/image-history/7/test.png",
	}, requests)
}

func TestImageHistoryS3StoreDisabledReturnsNoStore(t *testing.T) {
	store, err := NewImageHistoryS3Store(&config.Config{})
	require.NoError(t, err)
	require.Nil(t, store)
}
