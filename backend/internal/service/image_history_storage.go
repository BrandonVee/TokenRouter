package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/config"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/BrandonVee/TokenRouter/internal/pkg/logger"
)

const (
	settingKeyImageHistoryStorageConfig = "image_history_storage_config"

	ImageHistoryStorageSourceDeployment = "deployment"
	ImageHistoryStorageSourceDatabase   = "database"
)

var (
	ErrImageHistoryStorageConfigInvalid = infraerrors.BadRequest("IMAGE_HISTORY_STORAGE_CONFIG_INVALID", "image history storage config is invalid")
	ErrImageHistoryStorageKeyRequired   = infraerrors.BadRequest(
		"IMAGE_HISTORY_STORAGE_ENCRYPTION_KEY_REQUIRED",
		"set a fixed TOTP_ENCRYPTION_KEY before saving image history storage credentials",
	)
)

// ImageHistoryObjectStoreFactory 根据运行时配置创建生图历史私有对象存储。
type ImageHistoryObjectStoreFactory func(cfg config.ImageHistoryConfig) (ImageHistoryObjectStore, error)

type imageHistoryObjectStoreConnectionTester interface {
	TestConnection(ctx context.Context) error
}

// ImageHistoryStorageConfig 是管理员页面使用的生图历史存储配置。
type ImageHistoryStorageConfig struct {
	Enabled            bool   `json:"enabled"`
	Endpoint           string `json:"endpoint"`
	Region             string `json:"region"`
	Bucket             string `json:"bucket"`
	AccessKeyID        string `json:"access_key_id"`
	SecretAccessKey    string `json:"secret_access_key,omitempty"` //nolint:revive // 字段名沿用 S3 约定。
	Prefix             string `json:"prefix"`
	ForcePathStyle     bool   `json:"force_path_style"`
	SecretConfigured   bool   `json:"secret_configured"`
	Available          bool   `json:"available"`
	Source             string `json:"source"`
	EncryptionKeyReady bool   `json:"encryption_key_ready"`
}

type imageHistoryStoredStorageConfig struct {
	Enabled         bool   `json:"enabled"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key,omitempty"` //nolint:revive // 字段名沿用 S3 约定。
	Prefix          string `json:"prefix"`
	ForcePathStyle  bool   `json:"force_path_style"`
}

// ProvideImageHistoryService 组装生图历史服务，并让数据库页面配置覆盖旧部署配置。
func ProvideImageHistoryService(
	repo ImageHistoryRepository,
	settingRepo SettingRepository,
	cfg *config.Config,
	encryptor SecretEncryptor,
	storeFactory ImageHistoryObjectStoreFactory,
) (*ImageHistoryService, error) {
	deploymentCfg := config.ImageHistoryConfig{}
	if cfg != nil {
		deploymentCfg = cfg.ImageHistory
	}
	deploymentStore, err := storeFactory(deploymentCfg)
	if err != nil {
		return nil, err
	}
	service := NewImageHistoryService(repo, deploymentStore, cfg)
	service.settingRepo = settingRepo
	service.encryptor = encryptor
	service.storeFactory = storeFactory
	service.encryptionKeyConfigured = cfg != nil && cfg.Totp.EncryptionKeyConfigured
	if err := service.loadPersistedStorageConfig(context.Background()); err != nil {
		// 页面配置损坏时继续使用原 YAML/环境变量，避免管理员失去修复入口。
		logger.LegacyPrintf("service.image_history", "load persisted image history storage config failed; using deployment config: %v", err)
	}
	return service, nil
}

func (s *ImageHistoryService) storageSnapshot() (ImageHistoryObjectStore, config.ImageHistoryConfig) {
	if s == nil {
		return nil, config.ImageHistoryConfig{}
	}
	s.storageMu.RLock()
	defer s.storageMu.RUnlock()
	cfg := s.storageCfg
	// 部署配置模式继续反映调用方持有的 Config，保持旧构造函数的行为兼容。
	if s.storageSource == ImageHistoryStorageSourceDeployment && s.cfg != nil {
		cfg = s.cfg.ImageHistory
	}
	return s.store, cfg
}

func (s *ImageHistoryService) applyStorageConfig(store ImageHistoryObjectStore, cfg config.ImageHistoryConfig, source string) {
	s.storageMu.Lock()
	s.store = store
	s.storageCfg = cfg
	s.storageSource = source
	s.storageMu.Unlock()
}

// GetStorageConfig 返回脱敏后的当前有效配置。
func (s *ImageHistoryService) GetStorageConfig() ImageHistoryStorageConfig {
	if s == nil {
		return ImageHistoryStorageConfig{Source: ImageHistoryStorageSourceDeployment}
	}
	s.storageMu.RLock()
	defer s.storageMu.RUnlock()
	return imageHistoryStorageConfigResponse(
		s.storageCfg,
		s.storageSource,
		s.store != nil && s.storageCfg.Enabled,
		s.encryptionKeyConfigured,
	)
}

// UpdateStorageConfig 加密保存页面配置，并立即替换后续请求使用的对象存储。
func (s *ImageHistoryService) UpdateStorageConfig(ctx context.Context, input ImageHistoryStorageConfig) (ImageHistoryStorageConfig, error) {
	if s == nil || s.settingRepo == nil || s.encryptor == nil || s.storeFactory == nil {
		return ImageHistoryStorageConfig{}, ErrImageHistoryStorageConfigInvalid
	}
	runtimeCfg, err := s.runtimeStorageConfigFromInput(input)
	if err != nil {
		return ImageHistoryStorageConfig{}, err
	}
	if strings.TrimSpace(runtimeCfg.SecretAccessKey) == "" {
		_, current := s.storageSnapshot()
		runtimeCfg.SecretAccessKey = current.SecretAccessKey
	}
	if runtimeCfg.Enabled && !s.encryptionKeyConfigured {
		return ImageHistoryStorageConfig{}, ErrImageHistoryStorageKeyRequired
	}
	if err := validateImageHistoryRuntimeStorageConfig(runtimeCfg); err != nil {
		return ImageHistoryStorageConfig{}, err
	}

	store, err := s.storeFactory(runtimeCfg)
	if err != nil {
		return ImageHistoryStorageConfig{}, fmt.Errorf("create image history object store: %w", err)
	}
	stored := imageHistoryStoredStorageConfigFromRuntime(runtimeCfg)
	if strings.TrimSpace(runtimeCfg.SecretAccessKey) != "" {
		if !s.encryptionKeyConfigured {
			// 关闭状态允许清空凭据，但不会把临时密钥加密的值写入数据库。
			stored.SecretAccessKey = ""
			runtimeCfg.SecretAccessKey = ""
		} else {
			stored.SecretAccessKey, err = s.encryptor.Encrypt(runtimeCfg.SecretAccessKey)
			if err != nil {
				return ImageHistoryStorageConfig{}, fmt.Errorf("encrypt image history storage secret: %w", err)
			}
		}
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return ImageHistoryStorageConfig{}, fmt.Errorf("marshal image history storage config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, settingKeyImageHistoryStorageConfig, string(data)); err != nil {
		return ImageHistoryStorageConfig{}, fmt.Errorf("save image history storage config: %w", err)
	}
	s.applyStorageConfig(store, runtimeCfg, ImageHistoryStorageSourceDatabase)
	return s.GetStorageConfig(), nil
}

// TestStorageConnection 使用表单参数或已保存 Secret 验证桶是否可访问。
func (s *ImageHistoryService) TestStorageConnection(ctx context.Context, input ImageHistoryStorageConfig) error {
	if s == nil || s.storeFactory == nil {
		return ErrImageHistoryStorageConfigInvalid
	}
	runtimeCfg, err := s.runtimeStorageConfigFromInput(input)
	if err != nil {
		return err
	}
	if strings.TrimSpace(runtimeCfg.SecretAccessKey) == "" {
		_, current := s.storageSnapshot()
		runtimeCfg.SecretAccessKey = current.SecretAccessKey
	}
	runtimeCfg.Enabled = true
	if err := validateImageHistoryRuntimeStorageConfig(runtimeCfg); err != nil {
		return err
	}
	store, err := s.storeFactory(runtimeCfg)
	if err != nil {
		return fmt.Errorf("create image history object store: %w", err)
	}
	tester, ok := store.(imageHistoryObjectStoreConnectionTester)
	if !ok {
		return fmt.Errorf("image history object store does not support connection testing")
	}
	return tester.TestConnection(ctx)
}

// ResetStorageConfig 删除页面覆盖并恢复 YAML/环境变量配置。
func (s *ImageHistoryService) ResetStorageConfig(ctx context.Context) (ImageHistoryStorageConfig, error) {
	if s == nil || s.settingRepo == nil || s.storeFactory == nil {
		return ImageHistoryStorageConfig{}, ErrImageHistoryStorageConfigInvalid
	}
	if err := s.settingRepo.Delete(ctx, settingKeyImageHistoryStorageConfig); err != nil {
		return ImageHistoryStorageConfig{}, fmt.Errorf("delete image history storage config: %w", err)
	}
	store, err := s.storeFactory(s.deploymentStorageCfg)
	if err != nil {
		return ImageHistoryStorageConfig{}, fmt.Errorf("restore deployment image history storage config: %w", err)
	}
	s.applyStorageConfig(store, s.deploymentStorageCfg, ImageHistoryStorageSourceDeployment)
	return s.GetStorageConfig(), nil
}

func (s *ImageHistoryService) loadPersistedStorageConfig(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	raw, err := s.settingRepo.GetValue(ctx, settingKeyImageHistoryStorageConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var stored imageHistoryStoredStorageConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return fmt.Errorf("decode image history storage config: %w", err)
	}
	runtimeCfg := imageHistoryRuntimeStorageConfig(s.deploymentStorageCfg, stored)
	if strings.TrimSpace(runtimeCfg.SecretAccessKey) != "" {
		decrypted, err := s.encryptor.Decrypt(runtimeCfg.SecretAccessKey)
		if err != nil {
			return fmt.Errorf("decrypt image history storage secret: %w", err)
		}
		runtimeCfg.SecretAccessKey = decrypted
	}
	if err := validateImageHistoryRuntimeStorageConfig(runtimeCfg); err != nil {
		return err
	}
	store, err := s.storeFactory(runtimeCfg)
	if err != nil {
		return fmt.Errorf("create persisted image history object store: %w", err)
	}
	s.applyStorageConfig(store, runtimeCfg, ImageHistoryStorageSourceDatabase)
	return nil
}

func (s *ImageHistoryService) runtimeStorageConfigFromInput(input ImageHistoryStorageConfig) (config.ImageHistoryConfig, error) {
	base := s.deploymentStorageCfg
	stored := imageHistoryStoredStorageConfig{
		Enabled:         input.Enabled,
		Endpoint:        input.Endpoint,
		Region:          input.Region,
		Bucket:          input.Bucket,
		AccessKeyID:     input.AccessKeyID,
		SecretAccessKey: input.SecretAccessKey,
		Prefix:          input.Prefix,
		ForcePathStyle:  input.ForcePathStyle,
	}
	runtimeCfg := imageHistoryRuntimeStorageConfig(base, stored)
	return runtimeCfg, nil
}

func imageHistoryRuntimeStorageConfig(base config.ImageHistoryConfig, stored imageHistoryStoredStorageConfig) config.ImageHistoryConfig {
	base.Enabled = stored.Enabled
	base.Endpoint = strings.TrimSpace(stored.Endpoint)
	base.Region = strings.TrimSpace(stored.Region)
	if base.Region == "" {
		base.Region = "auto"
	}
	base.Bucket = strings.TrimSpace(stored.Bucket)
	base.AccessKeyID = strings.TrimSpace(stored.AccessKeyID)
	base.SecretAccessKey = strings.TrimSpace(stored.SecretAccessKey)
	base.Prefix = strings.Trim(strings.TrimSpace(stored.Prefix), "/")
	base.ForcePathStyle = stored.ForcePathStyle
	return base
}

func imageHistoryStoredStorageConfigFromRuntime(cfg config.ImageHistoryConfig) imageHistoryStoredStorageConfig {
	return imageHistoryStoredStorageConfig{
		Enabled:         cfg.Enabled,
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		Bucket:          cfg.Bucket,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		Prefix:          cfg.Prefix,
		ForcePathStyle:  cfg.ForcePathStyle,
	}
}

func imageHistoryStorageConfigResponse(cfg config.ImageHistoryConfig, source string, available, encryptionKeyReady bool) ImageHistoryStorageConfig {
	return ImageHistoryStorageConfig{
		Enabled:            cfg.Enabled,
		Endpoint:           cfg.Endpoint,
		Region:             cfg.Region,
		Bucket:             cfg.Bucket,
		AccessKeyID:        cfg.AccessKeyID,
		Prefix:             cfg.Prefix,
		ForcePathStyle:     cfg.ForcePathStyle,
		SecretConfigured:   strings.TrimSpace(cfg.SecretAccessKey) != "",
		Available:          available,
		Source:             source,
		EncryptionKeyReady: encryptionKeyReady,
	}
}

func validateImageHistoryRuntimeStorageConfig(cfg config.ImageHistoryConfig) error {
	if cfg.Prefix == "" || cfg.Prefix == "." || strings.Contains(cfg.Prefix, "..") {
		return infraerrors.BadRequest("IMAGE_HISTORY_PREFIX_INVALID", "image history prefix must be a safe non-empty object prefix")
	}
	if !cfg.Enabled {
		return nil
	}
	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return infraerrors.BadRequest("IMAGE_HISTORY_S3_CONFIG_INCOMPLETE", "bucket, access_key_id and secret_access_key are required")
	}
	return nil
}
