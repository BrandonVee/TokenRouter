package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dashboardAdRepositoryStub struct{}

func (dashboardAdRepositoryStub) List(context.Context) ([]DashboardAd, error) {
	return []DashboardAd{}, nil
}
func (dashboardAdRepositoryStub) Replace(context.Context, []DashboardAd) error { return nil }

func TestNormalizeDashboardAdsAssignsIDsAndKeepsOrder(t *testing.T) {
	start := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	ads, err := normalizeDashboardAds([]DashboardAd{
		{ImageURL: " https://cdn.example/ad.png ", LinkURL: " https://example.com ", StartsAt: &start, EndsAt: &end},
		{ID: "second", Enabled: true},
	})

	require.NoError(t, err)
	require.Len(t, ads, 2)
	require.NotEmpty(t, ads[0].ID)
	require.Equal(t, "second", ads[1].ID)
	require.Equal(t, "https://cdn.example/ad.png", ads[0].ImageURL)
	require.Equal(t, "https://example.com", ads[0].LinkURL)
}

func TestNormalizeDashboardAdsRejectsDuplicateAndInvalidSchedule(t *testing.T) {
	start := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	end := start.Add(-time.Minute)

	_, err := normalizeDashboardAds([]DashboardAd{{ID: "same"}, {ID: "same"}})
	require.Error(t, err)

	_, err = normalizeDashboardAds([]DashboardAd{{ID: "bad-schedule", StartsAt: &start, EndsAt: &end}})
	require.Error(t, err)
}

func TestBuildSystemSettingsUpdatesOmitsLegacyDashboardAdsWithRepository(t *testing.T) {
	svc := &SettingService{dashboardAdRepo: dashboardAdRepositoryStub{}}
	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{})

	require.NoError(t, err)
	_, exists := updates[SettingKeyDashboardAds]
	require.False(t, exists)
}

func TestParseDashboardAdsSkipsMalformedItems(t *testing.T) {
	ads := parseDashboardAds(`[{
    "id":"valid",
    "image_url":"https://cdn.example/valid.png"
  }, {
    "id":"invalid",
    "starts_at":"not-a-timestamp"
  }]`)

	require.Len(t, ads, 1)
	require.Equal(t, "valid", ads[0].ID)
}
