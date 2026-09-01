package service

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

const imageHistoryValidPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M/wHwAF/gL+Z8dZAAAAAElFTkSuQmCC"

type fakeImageHistoryRepository struct {
	mu           sync.Mutex
	enabled      bool
	setUserID    int64
	setEnabled   bool
	createErr    error
	onCreate     func()
	records      []ImageHistoryRecord
	deletedIDs   []string
	lastListUser int64
}

func (r *fakeImageHistoryRepository) GetSavingEnabled(context.Context, int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled, nil
}

func (r *fakeImageHistoryRepository) SetSavingEnabled(_ context.Context, userID int64, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
	r.setUserID = userID
	r.setEnabled = enabled
	return nil
}

func (r *fakeImageHistoryRepository) Create(_ context.Context, record ImageHistoryRecord) error {
	if r.onCreate != nil {
		r.onCreate()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	r.records = append(r.records, record)
	return nil
}

func (r *fakeImageHistoryRepository) List(_ context.Context, userID int64, params pagination.PaginationParams) ([]ImageHistoryRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastListUser = userID
	items := append([]ImageHistoryRecord(nil), r.records...)
	return items, int64(len(items)), nil
}

func (r *fakeImageHistoryRepository) Get(_ context.Context, userID int64, id string) (*ImageHistoryRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.records {
		if record.UserID == userID && record.ID == id {
			copyRecord := record
			return &copyRecord, nil
		}
	}
	return nil, ErrImageHistoryNotFound
}

func (r *fakeImageHistoryRepository) Delete(_ context.Context, userID int64, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index, record := range r.records {
		if record.UserID == userID && record.ID == id {
			r.records = append(r.records[:index], r.records[index+1:]...)
			r.deletedIDs = append(r.deletedIDs, id)
			return nil
		}
	}
	return ErrImageHistoryNotFound
}

type fakeImageHistoryObjectStore struct {
	mu               sync.Mutex
	objects          map[string][]byte
	deletedKeys      []string
	presignedKeys    []string
	putStarted       chan struct{}
	putStartedOnce   sync.Once
	blockPut         chan struct{}
	deleteContextErr error
}

func newFakeImageHistoryObjectStore() *fakeImageHistoryObjectStore {
	return &fakeImageHistoryObjectStore{objects: make(map[string][]byte)}
}

func (s *fakeImageHistoryObjectStore) Put(ctx context.Context, key, _ string, data []byte) error {
	if s.putStarted != nil {
		s.putStartedOnce.Do(func() { close(s.putStarted) })
	}
	if s.blockPut != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.blockPut:
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = append([]byte(nil), data...)
	return nil
}

func (s *fakeImageHistoryObjectStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (s *fakeImageHistoryObjectStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteContextErr = ctx.Err()
	delete(s.objects, key)
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

func (s *fakeImageHistoryObjectStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presignedKeys = append(s.presignedKeys, key)
	return "https://s3.example.com/" + key, nil
}

func newTestImageHistoryService(repo ImageHistoryRepository, store ImageHistoryObjectStore) *ImageHistoryService {
	return NewImageHistoryService(repo, store, &config.Config{ImageHistory: config.ImageHistoryConfig{
		Enabled:                 true,
		Prefix:                  "image-history",
		MaxObjectBytes:          1024 * 1024,
		DownloadTimeoutSeconds:  2,
		PreviewURLExpiryMinutes: 15,
	}})
}

func TestImageHistoryServiceSettingsRequireGlobalAvailability(t *testing.T) {
	repo := &fakeImageHistoryRepository{}
	unavailable := NewImageHistoryService(repo, nil, &config.Config{})

	settings, err := unavailable.GetSettings(context.Background(), 9)
	require.NoError(t, err)
	require.False(t, settings.Available)
	require.False(t, settings.Enabled)
	_, err = unavailable.UpdateSettings(context.Background(), 9, true)
	require.ErrorIs(t, err, ErrImageHistoryUnavailable)

	available := newTestImageHistoryService(repo, newFakeImageHistoryObjectStore())
	settings, err = available.UpdateSettings(context.Background(), 9, true)
	require.NoError(t, err)
	require.True(t, settings.Available)
	require.True(t, settings.Enabled)
	require.True(t, available.ShouldCapture(context.Background(), 9))
	require.Equal(t, int64(9), repo.setUserID)
}

func TestImageHistoryServiceSavesValidatedImageAndTruncatesMetadata(t *testing.T) {
	repo := &fakeImageHistoryRepository{enabled: true}
	store := newFakeImageHistoryObjectStore()
	svc := newTestImageHistoryService(repo, store)

	err := svc.SaveCapturedImages(context.Background(), SaveImageHistoryInput{
		UserID:    11,
		APIKeyID:  22,
		RequestID: strings.Repeat("r", 300),
		Source:    strings.Repeat("源", 40),
		Endpoint:  "/v1/images/generations",
		Model:     strings.Repeat("模", 300),
		Prompt:    "画一张测试图",
		Images: []GeneratedImageCapture{{
			Base64:        imageHistoryValidPNGBase64,
			MimeType:      "image/jpeg",
			RevisedPrompt: "修订提示词",
		}},
	})
	require.NoError(t, err)
	require.Len(t, repo.records, 1)
	record := repo.records[0]
	require.Equal(t, "image/png", record.MimeType)
	require.Equal(t, 1, record.Width)
	require.Equal(t, 1, record.Height)
	require.Equal(t, int64(11), record.UserID)
	require.Equal(t, int64(22), *record.APIKeyID)
	require.Equal(t, 255, utf8.RuneCountInString(record.RequestID))
	require.Equal(t, 32, utf8.RuneCountInString(record.Source))
	require.Equal(t, 255, utf8.RuneCountInString(record.Model))
	require.Contains(t, record.ObjectKey, "image-history/11/")
	require.NotEmpty(t, record.SHA256)
	require.Empty(t, record.Parameters)
	require.Len(t, store.objects, 1)
}

func TestImageHistoryServiceRejectsSpoofedOrOversizedBase64(t *testing.T) {
	repo := &fakeImageHistoryRepository{enabled: true}
	store := newFakeImageHistoryObjectStore()
	svc := newTestImageHistoryService(repo, store)

	err := svc.SaveCapturedImages(context.Background(), SaveImageHistoryInput{
		UserID: 1,
		Images: []GeneratedImageCapture{{
			Base64:   base64.StdEncoding.EncodeToString([]byte("not an image")),
			MimeType: "image/png",
		}},
	})
	require.Error(t, err)
	require.Empty(t, store.objects)

	svc.cfg.ImageHistory.MaxObjectBytes = 4
	err = svc.SaveCapturedImages(context.Background(), SaveImageHistoryInput{
		UserID: 1,
		Images: []GeneratedImageCapture{{Base64: imageHistoryValidPNGBase64}},
	})
	require.ErrorContains(t, err, "exceeds configured history limit")
	require.Empty(t, store.objects)
}

func TestImageHistoryServiceCleansObjectWhenMetadataWriteFailsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &fakeImageHistoryRepository{createErr: errors.New("database unavailable"), onCreate: cancel}
	store := newFakeImageHistoryObjectStore()
	svc := newTestImageHistoryService(repo, store)

	err := svc.SaveCapturedImages(ctx, SaveImageHistoryInput{
		UserID: 1,
		Images: []GeneratedImageCapture{{Base64: imageHistoryValidPNGBase64}},
	})
	require.ErrorContains(t, err, "database unavailable")
	require.Empty(t, store.objects)
	require.Len(t, store.deletedKeys, 1)
	require.NoError(t, store.deleteContextErr)
}

func TestImageHistoryServiceListOpenAndDeleteKeepUserScope(t *testing.T) {
	store := newFakeImageHistoryObjectStore()
	record := ImageHistoryRecord{ID: "record-1", UserID: 7, ObjectKey: "image-history/7/record-1.png", MimeType: "image/png", SizeBytes: 3}
	repo := &fakeImageHistoryRepository{records: []ImageHistoryRecord{record}}
	store.objects[record.ObjectKey] = []byte("png")
	svc := newTestImageHistoryService(repo, store)

	list, err := svc.List(context.Background(), 7, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Contains(t, list.Items[0].PreviewURL, record.ObjectKey)
	require.Equal(t, int64(7), repo.lastListUser)

	content, err := svc.OpenContent(context.Background(), 8, record.ID)
	require.Nil(t, content)
	require.ErrorIs(t, err, ErrImageHistoryNotFound)
	require.ErrorIs(t, svc.Delete(context.Background(), 8, record.ID), ErrImageHistoryNotFound)
	require.NoError(t, svc.Delete(context.Background(), 7, record.ID))
	require.Equal(t, []string{record.ObjectKey}, store.deletedKeys)
	require.Equal(t, []string{record.ID}, repo.deletedIDs)
}
