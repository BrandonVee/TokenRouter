package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BrandonVee/TokenRouter/internal/config"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
)

const (
	settingKeyFileStorageConfig           = "file_storage_config"
	FileStorageDirectoryInvoiceAttachment = "invoice_attachments"
	FileStorageTypeLocal                  = "local"
	FileStorageTypeS3                     = "s3"
	localFileStorageProfileID             = "local-default"
)

// FileObjectStore 统一受保护业务文件的存取接口，避免业务代码绑定某个后端。
type FileObjectStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	TestConnection(ctx context.Context) error
}

// FileStorageS3Factory 创建 S3 兼容的文件存储后端。
type FileStorageS3Factory func(ctx context.Context, cfg *BackupS3Config) (BackupObjectStore, error)

// FileStorageProfile 是一个不可变的存储目的地。更新目录配置会创建新档案。
type FileStorageProfile struct {
	ID                 string         `json:"id"`
	Type               string         `json:"type"`
	LocalPath          string         `json:"local_path"`
	S3                 BackupS3Config `json:"s3"`
	SecretConfigured   bool           `json:"secret_configured"`
	EncryptionKeyReady bool           `json:"encryption_key_ready"`
}

// FileStorageDirectoryConfig 表示一个业务目录当前用于新对象的存储档案。
type FileStorageDirectoryConfig struct {
	Directory string             `json:"directory"`
	Profile   FileStorageProfile `json:"profile"`
}

type storedFileStorageProfile struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	LocalPath string         `json:"local_path"`
	S3        BackupS3Config `json:"s3"`
}

type storedFileStorageConfig struct {
	Assignments map[string]string                   `json:"assignments"`
	Profiles    map[string]storedFileStorageProfile `json:"profiles"`
}

// FileStorageService 管理按业务目录分配的版本化本地/S3 存储档案。
type FileStorageService struct {
	settingRepo             SettingRepository
	encryptor               SecretEncryptor
	s3Factory               FileStorageS3Factory
	encryptionKeyConfigured bool
	localPath               string

	mu     sync.RWMutex
	config storedFileStorageConfig
	stores map[string]FileObjectStore
}

// ProvideFileStorageService 初始化统一文件存储；损坏的管理配置只会回退到本地默认档案。
func ProvideFileStorageService(settingRepo SettingRepository, cfg *config.Config, encryptor SecretEncryptor, factory BackupObjectStoreFactory) *FileStorageService {
	service := &FileStorageService{
		settingRepo: settingRepo,
		encryptor:   encryptor,
		s3Factory: func(ctx context.Context, cfg *BackupS3Config) (BackupObjectStore, error) {
			return factory(ctx, cfg)
		},
		encryptionKeyConfigured: cfg != nil && cfg.Totp.EncryptionKeyConfigured,
		localPath:               filepath.Join(defaultDataShareExportDataDir(), "invoice-attachments"),
		stores:                  make(map[string]FileObjectStore),
	}
	service.config = service.defaultConfig()
	if err := service.load(context.Background()); err != nil {
		// 存储配置不能阻断主服务启动，管理员可通过页面修复。
		service.config = service.defaultConfig()
	}
	return service
}

func (s *FileStorageService) defaultConfig() storedFileStorageConfig {
	return storedFileStorageConfig{
		Assignments: map[string]string{FileStorageDirectoryInvoiceAttachment: localFileStorageProfileID},
		Profiles: map[string]storedFileStorageProfile{
			localFileStorageProfileID: {ID: localFileStorageProfileID, Type: FileStorageTypeLocal, LocalPath: s.localPath},
		},
	}
}

// GetInvoiceAttachmentConfig 返回当前用于新发票附件的档案，敏感密钥不会回显。
func (s *FileStorageService) GetInvoiceAttachmentConfig() FileStorageDirectoryConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.directoryConfigLocked(FileStorageDirectoryInvoiceAttachment)
}

// UpdateInvoiceAttachmentConfig 创建并切换到新档案，已存对象继续引用其旧档案。
func (s *FileStorageService) UpdateInvoiceAttachmentConfig(ctx context.Context, input FileStorageDirectoryConfig) (FileStorageDirectoryConfig, error) {
	if input.Directory != "" && input.Directory != FileStorageDirectoryInvoiceAttachment {
		return FileStorageDirectoryConfig{}, infraerrors.BadRequest("FILE_STORAGE_DIRECTORY_INVALID", "unsupported file storage directory")
	}
	profile, err := s.prepareProfile(ctx, input.Profile)
	if err != nil {
		return FileStorageDirectoryConfig{}, err
	}
	if _, err := s.storeForProfile(ctx, profile); err != nil {
		return FileStorageDirectoryConfig{}, fmt.Errorf("create file storage: %w", err)
	}

	s.mu.Lock()
	next := s.cloneConfigLocked()
	next.Profiles[profile.ID] = profile
	next.Assignments[FileStorageDirectoryInvoiceAttachment] = profile.ID
	if err := s.saveLocked(ctx, next); err != nil {
		s.mu.Unlock()
		return FileStorageDirectoryConfig{}, err
	}
	s.config = next
	s.mu.Unlock()
	return s.GetInvoiceAttachmentConfig(), nil
}

// TestInvoiceAttachmentStorageConnection 验证待保存的 S3 桶权限，本地目录则验证可写性。
func (s *FileStorageService) TestInvoiceAttachmentStorageConnection(ctx context.Context, input FileStorageDirectoryConfig) error {
	profile, err := s.prepareProfile(ctx, input.Profile)
	if err != nil {
		return err
	}
	store, err := s.storeForProfile(ctx, profile)
	if err != nil {
		return fmt.Errorf("create file storage: %w", err)
	}
	return store.TestConnection(ctx)
}

// ResolveInvoiceAttachmentStore 按附件保存的档案 ID 查找后端，使切换配置不影响历史文件。
func (s *FileStorageService) ResolveInvoiceAttachmentStore(ctx context.Context, profileID string) (FileObjectStore, error) {
	if strings.TrimSpace(profileID) == "" {
		profileID = localFileStorageProfileID
	}
	s.mu.RLock()
	profile, ok := s.config.Profiles[profileID]
	s.mu.RUnlock()
	if !ok {
		return nil, infraerrors.NotFound("FILE_STORAGE_PROFILE_NOT_FOUND", "file storage profile is no longer available")
	}
	return s.storeForProfile(ctx, profile)
}

func (s *FileStorageService) CurrentInvoiceAttachmentProfileID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Assignments[FileStorageDirectoryInvoiceAttachment]
}

// InvoiceAttachmentStorageType 返回档案的后端类型，供附件元数据审计与迁移使用。
func (s *FileStorageService) InvoiceAttachmentStorageType(profileID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if profile, ok := s.config.Profiles[profileID]; ok {
		return profile.Type
	}
	return FileStorageTypeLocal
}

func (s *FileStorageService) directoryConfigLocked(directory string) FileStorageDirectoryConfig {
	profile := s.config.Profiles[s.config.Assignments[directory]]
	return FileStorageDirectoryConfig{Directory: directory, Profile: fileStorageProfileResponse(profile, s.encryptionKeyConfigured)}
}

func (s *FileStorageService) prepareProfile(_ context.Context, input FileStorageProfile) (storedFileStorageProfile, error) {
	typeName := strings.ToLower(strings.TrimSpace(input.Type))
	if typeName == "" {
		typeName = FileStorageTypeLocal
	}
	switch typeName {
	case FileStorageTypeLocal:
		return storedFileStorageProfile{ID: localFileStorageProfileID, Type: FileStorageTypeLocal, LocalPath: s.localPath}, nil
	case FileStorageTypeS3:
		if !s.encryptionKeyConfigured || s.encryptor == nil {
			return storedFileStorageProfile{}, infraerrors.BadRequest("FILE_STORAGE_ENCRYPTION_KEY_REQUIRED", "stable TOTP encryption key is not ready; restart after database security-secret initialization")
		}
		cfg := input.S3
		cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
		cfg.Region = strings.TrimSpace(cfg.Region)
		if cfg.Region == "" {
			cfg.Region = "auto"
		}
		cfg.Bucket = strings.TrimSpace(cfg.Bucket)
		cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
		cfg.Prefix = strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
		if cfg.Prefix == "" || cfg.Prefix == "." || strings.Contains(cfg.Prefix, "..") {
			return storedFileStorageProfile{}, infraerrors.BadRequest("FILE_STORAGE_PREFIX_INVALID", "storage prefix must be a safe non-empty path")
		}
		if cfg.Bucket == "" || cfg.AccessKeyID == "" {
			return storedFileStorageProfile{}, infraerrors.BadRequest("FILE_STORAGE_S3_CONFIG_INCOMPLETE", "bucket and access_key_id are required")
		}
		if strings.TrimSpace(cfg.SecretAccessKey) == "" {
			s.mu.RLock()
			current := s.config.Profiles[s.config.Assignments[FileStorageDirectoryInvoiceAttachment]]
			s.mu.RUnlock()
			if current.Type == FileStorageTypeS3 && current.S3.Bucket == cfg.Bucket && current.S3.AccessKeyID == cfg.AccessKeyID && current.S3.Endpoint == cfg.Endpoint {
				plain, err := s.decryptSecret(current.S3.SecretAccessKey)
				if err != nil {
					return storedFileStorageProfile{}, err
				}
				cfg.SecretAccessKey = plain
			}
		}
		if strings.TrimSpace(cfg.SecretAccessKey) == "" {
			return storedFileStorageProfile{}, infraerrors.BadRequest("FILE_STORAGE_S3_CONFIG_INCOMPLETE", "secret_access_key is required")
		}
		plainSecret := cfg.SecretAccessKey
		encrypted, err := s.encryptor.Encrypt(plainSecret)
		if err != nil {
			return storedFileStorageProfile{}, fmt.Errorf("encrypt file storage secret: %w", err)
		}
		cfg.SecretAccessKey = encrypted
		id, err := newFileStorageProfileID()
		if err != nil {
			return storedFileStorageProfile{}, err
		}
		return storedFileStorageProfile{ID: id, Type: FileStorageTypeS3, S3: cfg}, nil
	default:
		return storedFileStorageProfile{}, infraerrors.BadRequest("FILE_STORAGE_TYPE_INVALID", "storage type must be local or s3")
	}
}

func (s *FileStorageService) storeForProfile(ctx context.Context, profile storedFileStorageProfile) (FileObjectStore, error) {
	s.mu.RLock()
	store := s.stores[profile.ID]
	s.mu.RUnlock()
	if store != nil {
		return store, nil
	}
	var created FileObjectStore
	switch profile.Type {
	case FileStorageTypeLocal:
		created = &localFileObjectStore{root: profile.LocalPath}
	case FileStorageTypeS3:
		if s.s3Factory == nil {
			return nil, fmt.Errorf("S3 factory is not configured")
		}
		cfg := profile.S3
		secret, err := s.decryptSecret(cfg.SecretAccessKey)
		if err != nil {
			return nil, err
		}
		cfg.SecretAccessKey = secret
		remote, err := s.s3Factory(ctx, &cfg)
		if err != nil {
			return nil, err
		}
		created = &s3FileObjectStore{store: remote, prefix: cfg.Prefix}
	default:
		return nil, fmt.Errorf("unsupported storage type %q", profile.Type)
	}
	s.mu.Lock()
	if existing := s.stores[profile.ID]; existing != nil {
		s.mu.Unlock()
		return existing, nil
	}
	s.stores[profile.ID] = created
	s.mu.Unlock()
	return created, nil
}

func (s *FileStorageService) decryptSecret(value string) (string, error) {
	if s.encryptor == nil {
		return "", fmt.Errorf("file storage encryptor is not configured")
	}
	return s.encryptor.Decrypt(value)
}

func (s *FileStorageService) load(ctx context.Context) error {
	if s.settingRepo == nil {
		return nil
	}
	raw, err := s.settingRepo.GetValue(ctx, settingKeyFileStorageConfig)
	if err != nil {
		if err == ErrSettingNotFound {
			return nil
		}
		return err
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var stored storedFileStorageConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return fmt.Errorf("decode file storage config: %w", err)
	}
	if stored.Assignments == nil || stored.Profiles == nil {
		return fmt.Errorf("file storage config is incomplete")
	}
	if _, ok := stored.Profiles[localFileStorageProfileID]; !ok {
		stored.Profiles[localFileStorageProfileID] = s.defaultConfig().Profiles[localFileStorageProfileID]
	}
	if _, ok := stored.Assignments[FileStorageDirectoryInvoiceAttachment]; !ok {
		stored.Assignments[FileStorageDirectoryInvoiceAttachment] = localFileStorageProfileID
	}
	s.config = stored
	return nil
}

func (s *FileStorageService) saveLocked(ctx context.Context, config storedFileStorageConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal file storage config: %w", err)
	}
	if s.settingRepo == nil {
		return fmt.Errorf("file storage setting repository is not configured")
	}
	if err := s.settingRepo.Set(ctx, settingKeyFileStorageConfig, string(data)); err != nil {
		return fmt.Errorf("save file storage config: %w", err)
	}
	return nil
}

func (s *FileStorageService) cloneConfigLocked() storedFileStorageConfig {
	next := storedFileStorageConfig{Assignments: make(map[string]string, len(s.config.Assignments)), Profiles: make(map[string]storedFileStorageProfile, len(s.config.Profiles))}
	for key, value := range s.config.Assignments {
		next.Assignments[key] = value
	}
	for key, value := range s.config.Profiles {
		next.Profiles[key] = value
	}
	return next
}

func fileStorageProfileResponse(profile storedFileStorageProfile, encryptionReady bool) FileStorageProfile {
	response := FileStorageProfile{ID: profile.ID, Type: profile.Type, LocalPath: profile.LocalPath, S3: profile.S3, EncryptionKeyReady: encryptionReady}
	response.SecretConfigured = strings.TrimSpace(response.S3.SecretAccessKey) != ""
	response.S3.SecretAccessKey = ""
	return response
}

func newFileStorageProfileID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate file storage profile ID: %w", err)
	}
	return "fsp_" + hex.EncodeToString(data), nil
}

type localFileObjectStore struct{ root string }

func (s *localFileObjectStore) Put(_ context.Context, key, _ string, data []byte) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}
func (s *localFileObjectStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s *localFileObjectStore) Delete(_ context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
func (s *localFileObjectStore) TestConnection(_ context.Context) error {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.root, ".write-test-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}
func (s *localFileObjectStore) path(key string) (string, error) {
	clean := filepath.Clean(key)
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe local storage key")
	}
	return filepath.Join(s.root, clean), nil
}

type s3FileObjectStore struct {
	store  BackupObjectStore
	prefix string
}

func (s *s3FileObjectStore) Put(ctx context.Context, key, contentType string, data []byte) error {
	key = s.objectKey(key)
	if sized, ok := s.store.(BackupObjectStoreSizedUploader); ok {
		_, err := sized.UploadSized(ctx, key, bytes.NewReader(data), contentType, int64(len(data)))
		return err
	}
	_, err := s.store.Upload(ctx, key, bytes.NewReader(data), contentType)
	return err
}
func (s *s3FileObjectStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.store.Download(ctx, s.objectKey(key))
}
func (s *s3FileObjectStore) Delete(ctx context.Context, key string) error {
	return s.store.Delete(ctx, s.objectKey(key))
}
func (s *s3FileObjectStore) TestConnection(ctx context.Context) error { return s.store.HeadBucket(ctx) }
func (s *s3FileObjectStore) objectKey(key string) string {
	return strings.Trim(s.prefix, "/") + "/" + strings.TrimLeft(key, "/")
}
