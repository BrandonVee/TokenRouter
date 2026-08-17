//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyPriorityGroupSelectsFirstActiveAccessibleGroup(t *testing.T) {
	service := &APIKeyService{groupRepo: &authGroupRepoStub{groupsByID: map[int64]Group{
		10: {ID: 10, Status: "inactive"},
		20: {ID: 20, Status: StatusActive, Platform: PlatformOpenAI},
		30: {ID: 30, Status: StatusActive, Platform: PlatformAnthropic},
	}}}
	key := &APIKey{
		GroupIDs: []int64{10, 20, 30},
		User:     &User{ID: 7, GroupRestrictionsLoaded: true},
	}

	selected := service.applyPriorityGroup(context.Background(), key)

	require.NotNil(t, selected.GroupID)
	require.Equal(t, int64(20), *selected.GroupID)
	require.Equal(t, int64(20), selected.Group.ID)
	require.Equal(t, []int64{10, 20, 30}, selected.GroupIDs)
}

func TestPrepareAPIKeyPriorityGroupsRejectsDuplicateBeforeRepositoryRead(t *testing.T) {
	service := &APIKeyService{}

	_, _, err := service.prepareAPIKeyPriorityGroups(context.Background(), &User{}, []int64{10, 10})

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
