package service

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/stretchr/testify/require"
)

type fileStorageFakeBackupStore struct {
	objects map[string][]byte
}

func (s *fileStorageFakeBackupStore) Upload(_ context.Context, key string, body io.Reader, _ string) (int64, error) {
	data, err := io.ReadAll(body)
	if err == nil {
		s.objects[key] = data
	}
	return int64(len(data)), err
}
func (s *fileStorageFakeBackupStore) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (int64, error) {
	return s.Upload(ctx, key, body, contentType)
}
func (s *fileStorageFakeBackupStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.objects[key])), nil
}
func (s *fileStorageFakeBackupStore) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}
func (s *fileStorageFakeBackupStore) PresignURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s *fileStorageFakeBackupStore) HeadBucket(context.Context) error { return nil }

// TestFileStorageProfilesRemainResolvable 验证切换档案不会覆盖已经写入的 S3 目的地。
func TestFileStorageProfilesRemainResolvable(t *testing.T) {
	repo := newImageHistoryStorageSettingRepo()
	created := make(map[string]*fileStorageFakeBackupStore)
	factory := func(_ context.Context, cfg *BackupS3Config) (BackupObjectStore, error) {
		store := &fileStorageFakeBackupStore{objects: make(map[string][]byte)}
		created[cfg.Bucket] = store
		return store, nil
	}
	service := ProvideFileStorageService(repo, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}}, imageHistoryStorageEncryptor{}, factory)

	first, err := service.UpdateInvoiceAttachmentConfig(context.Background(), FileStorageDirectoryConfig{
		Directory: FileStorageDirectoryInvoiceAttachment,
		Profile:   FileStorageProfile{Type: FileStorageTypeS3, S3: BackupS3Config{Bucket: "first", AccessKeyID: "key", SecretAccessKey: "secret", Prefix: "invoices"}},
	})
	require.NoError(t, err)
	require.Empty(t, first.Profile.S3.SecretAccessKey)
	require.True(t, first.Profile.SecretConfigured)

	oldStore, err := service.ResolveInvoiceAttachmentStore(context.Background(), first.Profile.ID)
	require.NoError(t, err)
	require.NoError(t, oldStore.Put(context.Background(), "a.pdf", "application/pdf", []byte("first")))

	second, err := service.UpdateInvoiceAttachmentConfig(context.Background(), FileStorageDirectoryConfig{
		Directory: FileStorageDirectoryInvoiceAttachment,
		Profile:   FileStorageProfile{Type: FileStorageTypeS3, S3: BackupS3Config{Bucket: "second", AccessKeyID: "key", SecretAccessKey: "secret", Prefix: "invoices"}},
	})
	require.NoError(t, err)
	require.NotEqual(t, first.Profile.ID, second.Profile.ID)

	reader, err := oldStore.Open(context.Background(), "a.pdf")
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.Equal(t, []byte("first"), data)
	require.Contains(t, created["first"].objects, "invoices/a.pdf")
}
