package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 缓存序列化必须保留独立的展示与准入策略，同时淘汰不含准入列的旧版本。
func TestAPIKeyAuthSnapshotIndependentModelAllowlist(t *testing.T) {
	svc := &APIKeyService{}
	for _, composite := range []bool{false, true} {
		key := profitAuthTestAPIKey()
		key.Group.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"display"}}
		key.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"review"}}
		group := key.Group
		if composite {
			key.IsComposite, key.Group, key.GroupID = true, nil, nil
			key.CompositeGroups = []APIKeyCompositeGroup{{GroupID: group.ID, Group: group, Prefix: "vip", NormalizedPrefix: "vip"}}
		}
		data, err := json.Marshal(svc.snapshotFromAPIKey(context.Background(), key))
		require.NoError(t, err)
		var snapshot APIKeyAuthSnapshot
		require.NoError(t, json.Unmarshal(data, &snapshot))
		restored, used, err := svc.applyAuthCacheEntry(key.Key, &APIKeyAuthCacheEntry{Snapshot: &snapshot})
		require.NoError(t, err)
		require.True(t, used)
		got := restored.Group
		if composite {
			got = restored.CompositeGroups[0].Group
		}
		require.Equal(t, group.ModelsListConfig, got.ModelsListConfig)
		require.Equal(t, group.ModelAllowlist, got.ModelAllowlist)
		require.True(t, got.ProfitControlEnabled)
		snapshot.Version = 37
		_, used, err = svc.applyAuthCacheEntry(key.Key, &APIKeyAuthCacheEntry{Snapshot: &snapshot})
		require.NoError(t, err)
		require.False(t, used)
	}
}
