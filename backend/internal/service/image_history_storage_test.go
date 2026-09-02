package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/stretchr/testify/require"
)

type imageHistoryStorageSettingRepo struct {
	values map[string]string
}

func newImageHistoryStorageSettingRepo() *imageHistoryStorageSettingRepo {
	return &imageHistoryStorageSettingRepo{values: make(map[string]string)}
}

func (r *imageHistoryStorageSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *imageHistoryStorageSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *imageHistoryStorageSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *imageHistoryStorageSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (r *imageHistoryStorageSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *imageHistoryStorageSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *imageHistoryStorageSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

type imageHistoryStorageEncryptor struct{}

func (imageHistoryStorageEncryptor) Encrypt(value string) (string, error) {
	return "ciphertext", nil
}

func (imageHistoryStorageEncryptor) Decrypt(value string) (string, error) {
	if value != "ciphertext" {
		return "", errors.New("invalid ciphertext")
	}
	return "stored-secret", nil
}

type imageHistoryStorageTestStore struct {
	*fakeImageHistoryObjectStore
	testErr error
}

func (s *imageHistoryStorageTestStore) TestConnection(context.Context) error {
	return s.testErr
}

func imageHistoryStorageTestFactory(created *[]config.ImageHistoryConfig) ImageHistoryObjectStoreFactory {
	return func(cfg config.ImageHistoryConfig) (ImageHistoryObjectStore, error) {
		*created = append(*created, cfg)
		if !cfg.Enabled {
			return nil, nil
		}
		return &imageHistoryStorageTestStore{fakeImageHistoryObjectStore: newFakeImageHistoryObjectStore()}, nil
	}
}

func TestImageHistoryStorageConfigFallsBackToDeployment(t *testing.T) {
	repo := newImageHistoryStorageSettingRepo()
	created := make([]config.ImageHistoryConfig, 0)
	cfg := &config.Config{
		Totp: config.TotpConfig{EncryptionKeyConfigured: true},
		ImageHistory: config.ImageHistoryConfig{
			Enabled:                 true,
			Region:                  "auto",
			Bucket:                  "legacy-bucket",
			AccessKeyID:             "legacy-key",
			SecretAccessKey:         "legacy-secret",
			Prefix:                  "legacy-images",
			MaxObjectBytes:          1024,
			PreviewURLExpiryMinutes: 15,
		},
	}
	svc, err := ProvideImageHistoryService(&fakeImageHistoryRepository{}, repo, cfg, imageHistoryStorageEncryptor{}, imageHistoryStorageTestFactory(&created))
	require.NoError(t, err)

	legacy := svc.GetStorageConfig()
	require.Equal(t, ImageHistoryStorageSourceDeployment, legacy.Source)
	require.True(t, legacy.Available)
	require.True(t, legacy.SecretConfigured)
	require.Empty(t, legacy.SecretAccessKey)
	require.Equal(t, "legacy-images", legacy.Prefix)

	updated, err := svc.UpdateStorageConfig(context.Background(), ImageHistoryStorageConfig{
		Enabled:         true,
		Endpoint:        "https://s3.example.com",
		Region:          "us-east-1",
		Bucket:          "page-bucket",
		AccessKeyID:     "page-key",
		SecretAccessKey: "page-secret",
		Prefix:          "/generated/images/",
		ForcePathStyle:  true,
	})
	require.NoError(t, err)
	require.Equal(t, ImageHistoryStorageSourceDatabase, updated.Source)
	require.Equal(t, "generated/images", updated.Prefix)
	require.True(t, updated.Available)
	require.NotContains(t, repo.values[settingKeyImageHistoryStorageConfig], "page-secret")

	var stored imageHistoryStoredStorageConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values[settingKeyImageHistoryStorageConfig]), &stored))
	require.Equal(t, "ciphertext", stored.SecretAccessKey)
	require.Equal(t, "page-secret", created[len(created)-1].SecretAccessKey)

}

func TestImageHistoryStorageConfigLoadsPersistedOverrideAndPreservesSecret(t *testing.T) {
	repo := newImageHistoryStorageSettingRepo()
	stored, err := json.Marshal(imageHistoryStoredStorageConfig{
		Enabled:         true,
		Region:          "auto",
		Bucket:          "stored-bucket",
		AccessKeyID:     "stored-key",
		SecretAccessKey: "ciphertext",
		Prefix:          "stored-images",
	})
	require.NoError(t, err)
	repo.values[settingKeyImageHistoryStorageConfig] = string(stored)
	created := make([]config.ImageHistoryConfig, 0)
	cfg := &config.Config{
		Totp: config.TotpConfig{EncryptionKeyConfigured: true},
		ImageHistory: config.ImageHistoryConfig{
			Prefix:                  "deployment-images",
			MaxObjectBytes:          2048,
			PreviewURLExpiryMinutes: 15,
		},
	}
	svc, err := ProvideImageHistoryService(&fakeImageHistoryRepository{}, repo, cfg, imageHistoryStorageEncryptor{}, imageHistoryStorageTestFactory(&created))
	require.NoError(t, err)
	require.Equal(t, ImageHistoryStorageSourceDatabase, svc.GetStorageConfig().Source)
	require.Equal(t, "stored-secret", created[len(created)-1].SecretAccessKey)

	updated, err := svc.UpdateStorageConfig(context.Background(), ImageHistoryStorageConfig{
		Enabled:     true,
		Region:      "auto",
		Bucket:      "stored-bucket",
		AccessKeyID: "stored-key",
		Prefix:      "new-prefix",
	})
	require.NoError(t, err)
	require.True(t, updated.SecretConfigured)
	require.Equal(t, "stored-secret", created[len(created)-1].SecretAccessKey)
	require.Equal(t, int64(2048), created[len(created)-1].MaxObjectBytes)
}

func TestImageHistoryStorageConfigRequiresStableEncryptionKeyBeforeSave(t *testing.T) {
	repo := newImageHistoryStorageSettingRepo()
	created := make([]config.ImageHistoryConfig, 0)
	svc, err := ProvideImageHistoryService(&fakeImageHistoryRepository{}, repo, &config.Config{}, imageHistoryStorageEncryptor{}, imageHistoryStorageTestFactory(&created))
	require.NoError(t, err)

	_, err = svc.UpdateStorageConfig(context.Background(), ImageHistoryStorageConfig{
		Enabled:         false,
		Region:          "auto",
		Bucket:          "bucket",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Prefix:          "images",
	})
	require.ErrorIs(t, err, ErrImageHistoryStorageKeyRequired)
	_, exists := repo.values[settingKeyImageHistoryStorageConfig]
	require.False(t, exists)
}

func TestImageHistoryStorageConnectionUsesSavedSecret(t *testing.T) {
	repo := newImageHistoryStorageSettingRepo()
	created := make([]config.ImageHistoryConfig, 0)
	cfg := &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: false}, ImageHistory: config.ImageHistoryConfig{
		Enabled:         true,
		Region:          "auto",
		Bucket:          "bucket",
		AccessKeyID:     "key",
		SecretAccessKey: "deployment-secret",
		Prefix:          "images",
	}}
	svc, err := ProvideImageHistoryService(&fakeImageHistoryRepository{}, repo, cfg, imageHistoryStorageEncryptor{}, imageHistoryStorageTestFactory(&created))
	require.NoError(t, err)

	err = svc.TestStorageConnection(context.Background(), ImageHistoryStorageConfig{
		Region:      "auto",
		Bucket:      "bucket",
		AccessKeyID: "key",
		Prefix:      "images",
	})
	require.NoError(t, err)
	require.Equal(t, "deployment-secret", created[len(created)-1].SecretAccessKey)
}
