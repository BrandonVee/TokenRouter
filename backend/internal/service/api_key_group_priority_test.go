//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/stretchr/testify/require"
)

// TestGetByKeyKeepsLegacyPrimaryGroup 防止历史候选列表继续在鉴权时切换分组。
func TestGetByKeyKeepsLegacyPrimaryGroup(t *testing.T) {
	primaryID := int64(10)
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		return &APIKey{
			ID: 1, UserID: 7, Status: StatusActive,
			GroupID: &primaryID, GroupIDs: []int64{10, 20},
			User:  &User{ID: 7, Status: StatusActive, GroupRestrictionsLoaded: true},
			Group: &Group{ID: primaryID, Status: StatusDisabled, Platform: PlatformOpenAI, Hydrated: true},
		}, nil
	}}
	groupRepo := &authGroupRepoStub{groupsByID: map[int64]Group{
		20: {ID: 20, Status: StatusActive, Platform: PlatformOpenAI},
	}}
	svc := NewAPIKeyService(repo, nil, groupRepo, nil, nil, nil, &config.Config{})

	key, err := svc.GetByKey(context.Background(), "sk-legacy")

	require.NoError(t, err)
	require.Equal(t, primaryID, *key.GroupID)
	require.Equal(t, StatusDisabled, key.Group.Status)
}

// TestRetiredAPIKeyRoutingRejected 确认创建与更新入口均拒绝已下线的配置。
func TestRetiredAPIKeyRoutingRejected(t *testing.T) {
	ids := []int64{10, 20}
	require.ErrorIs(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{GroupIDs: ids}), ErrAPIKeyGroupPrioritiesInvalid)
	require.ErrorIs(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{GroupIDs: &ids}), ErrAPIKeyGroupPrioritiesInvalid)
	for _, strategy := range []string{"auto", "speed", "price", "success_rate"} {
		require.ErrorIs(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{RoutingStrategy: strategy}), ErrInvalidAPIKeyRoutingStrategy)
		require.ErrorIs(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{RoutingStrategy: &strategy}), ErrInvalidAPIKeyRoutingStrategy)
	}
	require.NoError(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{GroupIDs: []int64{10}}))
	require.NoError(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{}))
}

func TestPrepareAPIKeySingleGroupRejectsMultipleBeforeRepositoryRead(t *testing.T) {
	service := &APIKeyService{}

	_, _, err := service.prepareAPIKeySingleGroup(context.Background(), &User{}, []int64{10, 10})

	require.ErrorIs(t, err, ErrAPIKeyGroupPrioritiesInvalid)
}

func TestAPIKeyAuthSnapshotPreservesGroupPriority(t *testing.T) {
	service := &APIKeyService{}
	key := &APIKey{ID: 1, GroupIDs: []int64{30, 20, 10}, User: &User{ID: 7}}

	snapshot := service.snapshotFromAPIKey(context.Background(), key)
	restored := service.snapshotToAPIKey("sk-test", snapshot)

	require.Equal(t, []int64{30, 20, 10}, snapshot.GroupIDs)
	require.Equal(t, []int64{30, 20, 10}, restored.GroupIDs)
}
