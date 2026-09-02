package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/BrandonVee/TokenRouter/internal/pkg/logger"
	"github.com/BrandonVee/TokenRouter/internal/pkg/pagination"
	"github.com/BrandonVee/TokenRouter/internal/util/urlvalidator"
	"github.com/google/uuid"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"
)

const (
	imageHistoryRequestIDMaxRunes     = 255
	imageHistorySearchMaxRunes        = 255
	imageHistorySourceMaxRunes        = 32
	imageHistoryEndpointMaxRunes      = 64
	imageHistoryModelMaxRunes         = 255
	imageHistoryParametersMaxRunes    = 65536
	imageHistoryPromptMaxRunes        = 32768
	imageHistoryRevisedPromptMaxRunes = 32768
	imageHistoryCleanupTimeout        = 10 * time.Second
)

var (
	ErrImageHistoryUnavailable = infraerrors.ServiceUnavailable("IMAGE_HISTORY_UNAVAILABLE", "image history storage is not configured")
	ErrImageHistoryNotFound    = infraerrors.NotFound("IMAGE_HISTORY_NOT_FOUND", "image history record not found")
)

// ImageHistoryRecord 是历史元数据的领域结构，对象键只在服务端内部流转。
type ImageHistoryRecord struct {
	ID            string    `json:"id"`
	UserID        int64     `json:"-"`
	APIKeyID      *int64    `json:"api_key_id,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	Source        string    `json:"source"`
	Endpoint      string    `json:"endpoint"`
	Model         string    `json:"model"`
	Prompt        string    `json:"prompt"`
	RevisedPrompt string    `json:"revised_prompt,omitempty"`
	Parameters    string    `json:"parameters,omitempty"`
	ObjectKey     string    `json:"-"`
	MimeType      string    `json:"mime_type"`
	SizeBytes     int64     `json:"size_bytes"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	SHA256        string    `json:"sha256"`
	PreviewURL    string    `json:"preview_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ImageHistorySettings 返回全局可用性和当前用户选择。
type ImageHistorySettings struct {
	Available bool `json:"available"`
	Enabled   bool `json:"enabled"`
}

// ImageHistoryList 是用户历史分页响应。
type ImageHistoryList struct {
	Items    []ImageHistoryRecord `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Pages    int                  `json:"pages"`
}

// ImageHistoryListParams 描述生图历史的分页和可选搜索条件。
type ImageHistoryListParams struct {
	pagination.PaginationParams
	Search string
}

// SaveImageHistoryInput 描述一次成功生图请求及其最终图片。
type SaveImageHistoryInput struct {
	UserID     int64
	APIKeyID   int64
	RequestID  string
	Source     string
	Endpoint   string
	Model      string
	Prompt     string
	Parameters string
	Images     []GeneratedImageCapture
}

// ImageHistoryRepository 负责用户偏好和历史元数据，图片字节由对象存储负责。
type ImageHistoryRepository interface {
	GetSavingEnabled(ctx context.Context, userID int64) (bool, error)
	SetSavingEnabled(ctx context.Context, userID int64, enabled bool) error
	Create(ctx context.Context, record ImageHistoryRecord) error
	List(ctx context.Context, userID int64, params ImageHistoryListParams) ([]ImageHistoryRecord, int64, error)
	Get(ctx context.Context, userID int64, id string) (*ImageHistoryRecord, error)
	Delete(ctx context.Context, userID int64, id string) error
}

// ImageHistoryObjectStore 是私有图片对象所需的最小 S3 能力。
type ImageHistoryObjectStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// ImageHistoryContent 包含鉴权下载所需的对象流和元数据。
type ImageHistoryContent struct {
	Record *ImageHistoryRecord
	Body   io.ReadCloser
}

// ImageHistoryService 协调用户 opt-in、图片转存和元数据生命周期。
type ImageHistoryService struct {
	repo ImageHistoryRepository
	// cfg 保留部署配置指针，兼容现有测试和依赖注入调用方；运行时读写使用 storageCfg 快照。
	cfg *config.Config

	storageMu            sync.RWMutex
	store                ImageHistoryObjectStore
	storageCfg           config.ImageHistoryConfig
	deploymentStorageCfg config.ImageHistoryConfig
	storageSource        string

	settingRepo             SettingRepository
	encryptor               SecretEncryptor
	storeFactory            ImageHistoryObjectStoreFactory
	encryptionKeyConfigured bool
	httpClient              *http.Client
}

// NewImageHistoryService 创建生图历史服务。
func NewImageHistoryService(repo ImageHistoryRepository, store ImageHistoryObjectStore, cfg *config.Config) *ImageHistoryService {
	timeout := 30 * time.Second
	storageCfg := config.ImageHistoryConfig{}
	if cfg != nil && cfg.ImageHistory.DownloadTimeoutSeconds > 0 {
		timeout = time.Duration(cfg.ImageHistory.DownloadTimeoutSeconds) * time.Second
	}
	if cfg != nil {
		storageCfg = cfg.ImageHistory
	}
	return &ImageHistoryService{
		repo:                 repo,
		cfg:                  cfg,
		store:                store,
		storageCfg:           storageCfg,
		deploymentStorageCfg: storageCfg,
		storageSource:        ImageHistoryStorageSourceDeployment,
		httpClient:           newImageHistoryHTTPClient(timeout),
	}
}

// Available 表示 S3 已由部署者完整启用。
func (s *ImageHistoryService) Available() bool {
	if s == nil || s.repo == nil {
		return false
	}
	store, cfg := s.storageSnapshot()
	return store != nil && cfg.Enabled
}

// GetSettings 获取当前用户的保存选择；全局未配置时始终显示未启用。
func (s *ImageHistoryService) GetSettings(ctx context.Context, userID int64) (ImageHistorySettings, error) {
	settings := ImageHistorySettings{Available: s.Available()}
	if s == nil || s.repo == nil {
		return settings, nil
	}
	enabled, err := s.repo.GetSavingEnabled(ctx, userID)
	if err != nil {
		return settings, err
	}
	settings.Enabled = settings.Available && enabled
	return settings, nil
}

// UpdateSettings 更新用户保存选择；未配置 S3 时不允许开启。
func (s *ImageHistoryService) UpdateSettings(ctx context.Context, userID int64, enabled bool) (ImageHistorySettings, error) {
	if enabled && !s.Available() {
		return ImageHistorySettings{Available: false}, ErrImageHistoryUnavailable
	}
	if s == nil || s.repo == nil {
		return ImageHistorySettings{}, ErrImageHistoryUnavailable
	}
	if err := s.repo.SetSavingEnabled(ctx, userID, enabled); err != nil {
		return ImageHistorySettings{Available: s.Available()}, err
	}
	return ImageHistorySettings{Available: s.Available(), Enabled: enabled && s.Available()}, nil
}

// ShouldCapture 只在全局存储可用且用户已主动开启时启用响应捕获。
func (s *ImageHistoryService) ShouldCapture(ctx context.Context, userID int64) bool {
	if !s.Available() {
		return false
	}
	enabled, err := s.repo.GetSavingEnabled(ctx, userID)
	if err != nil {
		// 生产迁移或数据库异常不能静默伪装成用户关闭，保留可检索的诊断信号。
		logger.L().With(
			zap.String("component", "service.image_history"),
			zap.Int64("user_id", userID),
		).Warn("image_history.capture_preference_load_failed", zap.Error(err))
		return false
	}
	return enabled
}

// SaveCapturedImages 逐张提交对象和元数据；单张失败不会删除同请求中已成功保存的图片。
func (s *ImageHistoryService) SaveCapturedImages(ctx context.Context, input SaveImageHistoryInput) error {
	if s == nil || s.repo == nil || len(input.Images) == 0 {
		return nil
	}
	store, cfg := s.storageSnapshot()
	if store == nil || !cfg.Enabled {
		return nil
	}
	var firstErr error
	for _, captured := range input.Images {
		if err := s.saveCapturedImage(ctx, input, captured, store, cfg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// List 返回当前用户的历史，并为每个私有对象生成短期预览地址。
func (s *ImageHistoryService) List(ctx context.Context, userID int64, params ImageHistoryListParams) (*ImageHistoryList, error) {
	if s == nil || s.repo == nil {
		return nil, ErrImageHistoryUnavailable
	}
	store, cfg := s.storageSnapshot()
	if store == nil || !cfg.Enabled {
		return nil, ErrImageHistoryUnavailable
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	// 限制搜索词长度，避免异常长查询拖慢历史列表。
	params.Search = truncateImageHistoryText(strings.TrimSpace(params.Search), imageHistorySearchMaxRunes)
	items, total, err := s.repo.List(ctx, userID, params)
	if err != nil {
		return nil, err
	}
	expiry := time.Duration(cfg.PreviewURLExpiryMinutes) * time.Minute
	for i := range items {
		previewURL, signErr := store.PresignGet(ctx, items[i].ObjectKey, expiry)
		if signErr == nil {
			items[i].PreviewURL = previewURL
		}
	}
	pages := 0
	if total > 0 {
		pages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}
	return &ImageHistoryList{Items: items, Total: total, Page: params.Page, PageSize: params.PageSize, Pages: pages}, nil
}

// OpenContent 在校验记录归属后打开私有对象。
func (s *ImageHistoryService) OpenContent(ctx context.Context, userID int64, id string) (*ImageHistoryContent, error) {
	if s == nil || s.repo == nil {
		return nil, ErrImageHistoryUnavailable
	}
	store, cfg := s.storageSnapshot()
	if store == nil || !cfg.Enabled {
		return nil, ErrImageHistoryUnavailable
	}
	record, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	body, err := store.Open(ctx, record.ObjectKey)
	if err != nil {
		return nil, err
	}
	return &ImageHistoryContent{Record: record, Body: body}, nil
}

// Delete 先删除 S3 对象，再删除元数据，避免留下用户看不见但仍计费的对象。
func (s *ImageHistoryService) Delete(ctx context.Context, userID int64, id string) error {
	if s == nil || s.repo == nil {
		return ErrImageHistoryUnavailable
	}
	store, cfg := s.storageSnapshot()
	if store == nil || !cfg.Enabled {
		return ErrImageHistoryUnavailable
	}
	record, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, record.ObjectKey); err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, id)
}

func (s *ImageHistoryService) saveCapturedImage(ctx context.Context, input SaveImageHistoryInput, captured GeneratedImageCapture, store ImageHistoryObjectStore, cfg config.ImageHistoryConfig) error {
	data, _, err := s.resolveCapturedImage(ctx, captured, cfg.MaxObjectBytes)
	if err != nil {
		return err
	}
	maxBytes := cfg.MaxObjectBytes
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("generated image exceeds configured history limit")
	}
	mimeType, width, height := inspectImageHistoryData(data)
	if mimeType == "" {
		return fmt.Errorf("generated output is not a supported image")
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	id := uuid.NewString()
	createdAt := time.Now()
	objectKey := path.Join(
		strings.Trim(strings.TrimSpace(cfg.Prefix), "/"),
		strconv.FormatInt(input.UserID, 10),
		createdAt.UTC().Format("2006/01"),
		id+imageHistoryExtension(mimeType),
	)
	if err := store.Put(ctx, objectKey, mimeType, data); err != nil {
		return err
	}
	apiKeyID := input.APIKeyID
	record := ImageHistoryRecord{
		ID:            id,
		UserID:        input.UserID,
		APIKeyID:      &apiKeyID,
		RequestID:     truncateImageHistoryText(input.RequestID, imageHistoryRequestIDMaxRunes),
		Source:        truncateImageHistoryText(input.Source, imageHistorySourceMaxRunes),
		Endpoint:      truncateImageHistoryText(input.Endpoint, imageHistoryEndpointMaxRunes),
		Model:         truncateImageHistoryText(input.Model, imageHistoryModelMaxRunes),
		Prompt:        truncateImageHistoryText(input.Prompt, imageHistoryPromptMaxRunes),
		RevisedPrompt: truncateImageHistoryText(captured.RevisedPrompt, imageHistoryRevisedPromptMaxRunes),
		Parameters:    truncateImageHistoryText(input.Parameters, imageHistoryParametersMaxRunes),
		ObjectKey:     objectKey,
		MimeType:      mimeType,
		SizeBytes:     int64(len(data)),
		Width:         width,
		Height:        height,
		SHA256:        digest,
		CreatedAt:     createdAt,
	}
	if input.APIKeyID <= 0 {
		record.APIKeyID = nil
	}
	if err := s.repo.Create(ctx, record); err != nil {
		// 元数据写入失败时使用独立短超时清理，避免请求 Context 已取消后留下孤儿对象。
		cleanupCtx, cancel := context.WithTimeout(context.Background(), imageHistoryCleanupTimeout)
		_ = store.Delete(cleanupCtx, objectKey)
		cancel()
		return err
	}
	return nil
}

func (s *ImageHistoryService) resolveCapturedImage(ctx context.Context, captured GeneratedImageCapture, maxObjectBytes int64) ([]byte, string, error) {
	if encoded := strings.TrimSpace(captured.Base64); encoded != "" {
		data, mimeType, err := decodeImageHistoryBase64(encoded, captured.MimeType, maxObjectBytes)
		return data, mimeType, err
	}
	rawURL := strings.TrimSpace(captured.URL)
	if strings.HasPrefix(strings.ToLower(rawURL), "data:image/") {
		return decodeImageHistoryBase64(rawURL, captured.MimeType, maxObjectBytes)
	}
	if rawURL == "" {
		return nil, "", fmt.Errorf("generated image response has no content")
	}
	validated, err := urlvalidator.ValidateHTTPSURL(rawURL, urlvalidator.ValidationOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("validate generated image URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validated, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download generated image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download generated image: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxObjectBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxObjectBytes {
		return nil, "", fmt.Errorf("generated image exceeds configured history limit")
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func decodeImageHistoryBase64(raw, fallbackMIME string, maxBytes int64) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	mimeType := strings.TrimSpace(fallbackMIME)
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		comma := strings.IndexByte(raw, ',')
		if comma < 0 {
			return nil, "", fmt.Errorf("invalid generated image data URL")
		}
		header := raw[5:comma]
		if semicolon := strings.IndexByte(header, ';'); semicolon >= 0 {
			mimeType = header[:semicolon]
		} else if header != "" {
			mimeType = header
		}
		raw = raw[comma+1:]
	}
	raw = strings.TrimSpace(raw)
	if maxBytes > 0 && int64(len(raw)) > imageHistoryBase64EncodedLimit(maxBytes) {
		return nil, "", fmt.Errorf("generated image exceeds configured history limit")
	}
	raw = strings.TrimRight(raw, "=") + strings.Repeat("=", (4-len(raw)%4)%4)
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("decode generated image: %w", err)
	}
	return data, mimeType, nil
}

func inspectImageHistoryData(data []byte) (string, int, int) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0
	}
	var mimeType string
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		mimeType = "image/png"
	case "jpeg":
		mimeType = "image/jpeg"
	case "webp":
		mimeType = "image/webp"
	case "gif":
		mimeType = "image/gif"
	default:
		return "", 0, 0
	}
	return mimeType, cfg.Width, cfg.Height
}

func imageHistoryBase64EncodedLimit(maxBytes int64) int64 {
	if maxBytes <= 0 || maxBytes > (1<<62)-2 {
		return 1<<63 - 1
	}
	return ((maxBytes + 2) / 3 * 4) + 4
}

func truncateImageHistoryText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return ""
	}
	count := 0
	for index := range value {
		if count == maxRunes {
			return value[:index]
		}
		count++
	}
	return value
}

func imageHistoryExtension(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func newImageHistoryHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, resolved := range addresses {
				if imageHistorySafeIP(resolved.IP) {
					return dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
				}
			}
			return nil, fmt.Errorf("generated image host resolved only to blocked addresses")
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many generated image redirects")
			}
			_, err := urlvalidator.ValidateHTTPSURL(req.URL.String(), urlvalidator.ValidationOptions{})
			return err
		},
	}
}

func imageHistorySafeIP(ip net.IP) bool {
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}

// imageHistoryFileName 为鉴权下载生成稳定且不含用户输入的文件名。
func imageHistoryFileName(record *ImageHistoryRecord) string {
	if record == nil {
		return "image.png"
	}
	return record.ID + imageHistoryExtension(record.MimeType)
}

// ImageHistoryDownloadName 返回历史下载的安全文件名。
func ImageHistoryDownloadName(record *ImageHistoryRecord) string {
	return path.Base(imageHistoryFileName(record))
}
