package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/google/uuid"
)

// ErrDashboardAdInvalid 表示广告列表存在重复 ID 或非法时间范围。
var ErrDashboardAdInvalid = infraerrors.BadRequest("DASHBOARD_AD_INVALID", "dashboard ad configuration is invalid")

const (
	// DashboardAdFitModeAdaptive 保持图片比例并根据容器宽度自适应。
	DashboardAdFitModeAdaptive = "adaptive"
	// DashboardAdFitModeCover 填满展示区域，超出部分裁剪。
	DashboardAdFitModeCover = "cover"
	// DashboardAdFitModeFill 拉伸图片填满展示区域。
	DashboardAdFitModeFill = "fill"
)

// normalizeDashboardAdFitMode 兼容历史广告数据，并将非法值回退为自适应。
func normalizeDashboardAdFitMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case DashboardAdFitModeCover:
		return DashboardAdFitModeCover
	case DashboardAdFitModeFill:
		return DashboardAdFitModeFill
	default:
		return DashboardAdFitModeAdaptive
	}
}

// DashboardAdRepository 负责独立广告表的查询和整体替换。
type DashboardAdRepository interface {
	List(ctx context.Context) ([]DashboardAd, error)
	Replace(ctx context.Context, ads []DashboardAd) error
}

// SetDashboardAdRepository 注入独立广告仓储，保持旧版 SettingService 构造函数兼容。
func (s *SettingService) SetDashboardAdRepository(repo DashboardAdRepository) {
	if s != nil {
		s.dashboardAdRepo = repo
	}
}

// loadDashboardAds 从独立表读取广告，并在历史数据未成功回填时回退旧键。
func (s *SettingService) loadDashboardAds(ctx context.Context, legacyRaw string) ([]DashboardAd, error) {
	if s.dashboardAdRepo == nil {
		return parseDashboardAds(legacyRaw), nil
	}
	ads, err := s.dashboardAdRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dashboard ads: %w", err)
	}
	if len(ads) > 0 {
		return ads, nil
	}
	if strings.TrimSpace(legacyRaw) != "" {
		return parseDashboardAds(legacyRaw), nil
	}
	return ads, nil
}

// GetDashboardAds 优先读取独立广告表，迁移未完成时从 settings 回退读取。
func (s *SettingService) GetDashboardAds(ctx context.Context) ([]DashboardAd, error) {
	if s == nil {
		return []DashboardAd{}, nil
	}
	if s.dashboardAdRepo != nil {
		ads, err := s.dashboardAdRepo.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("get dashboard ads: %w", err)
		}
		if len(ads) > 0 {
			return ads, nil
		}
		if s.settingRepo == nil {
			return ads, nil
		}
		// 迁移遇到历史脏数据时保留 settings 回退，直到管理员首次保存独立表。
		legacy, legacyErr := s.settingRepo.GetValue(ctx, SettingKeyDashboardAds)
		if legacyErr == nil {
			return parseDashboardAds(legacy), nil
		}
		if legacyErr != ErrSettingNotFound {
			return nil, fmt.Errorf("get legacy dashboard ads: %w", legacyErr)
		}
		return ads, nil
	}
	if s.settingRepo == nil {
		return []DashboardAd{}, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyDashboardAds)
	if err != nil {
		if err == ErrSettingNotFound {
			return []DashboardAd{}, nil
		}
		return nil, fmt.Errorf("get legacy dashboard ads: %w", err)
	}
	return parseDashboardAds(raw), nil
}

// ReplaceDashboardAds 校验并整体替换广告列表，避免普通设置保存覆盖广告数据。
func (s *SettingService) ReplaceDashboardAds(ctx context.Context, ads []DashboardAd) error {
	if s == nil {
		return fmt.Errorf("setting service is nil")
	}
	if s.settingRepo == nil {
		return fmt.Errorf("setting repository is nil")
	}
	normalized, err := normalizeDashboardAds(ads)
	if err != nil {
		return err
	}
	if s.dashboardAdRepo != nil {
		if err := s.dashboardAdRepo.Replace(ctx, normalized); err != nil {
			return fmt.Errorf("replace dashboard ads: %w", err)
		}
		// 清理迁移前的键，避免清空独立表后又从旧值恢复广告。
		if err := s.settingRepo.Delete(ctx, SettingKeyDashboardAds); err != nil && err != ErrSettingNotFound {
			return fmt.Errorf("delete legacy dashboard ads: %w", err)
		}
	} else {
		// 旧版测试或未迁移实例仍可通过 settings 键保存广告。
		payload, err := json.Marshal(normalized)
		if err != nil {
			return fmt.Errorf("marshal legacy dashboard ads: %w", err)
		}
		if err := s.settingRepo.Set(ctx, SettingKeyDashboardAds, string(payload)); err != nil {
			return fmt.Errorf("replace legacy dashboard ads: %w", err)
		}
	}
	// 广告更新也会触发设置缓存失效，但必须用数据库完整快照，不能用只含广告的零值结构覆盖其它缓存。
	if stored, err := s.GetAllSettings(ctx); err == nil {
		s.refreshCachedSettings(stored)
	} else {
		return fmt.Errorf("refresh settings after replacing dashboard ads: %w", err)
	}
	return nil
}

func normalizeDashboardAds(ads []DashboardAd) ([]DashboardAd, error) {
	result := make([]DashboardAd, 0, len(ads))
	seen := make(map[string]struct{}, len(ads))
	for i, ad := range ads {
		ad.ID = strings.TrimSpace(ad.ID)
		if ad.ID == "" {
			ad.ID = uuid.NewString()
		}
		if len(ad.ID) > 100 {
			return nil, fmt.Errorf("%w: ad %d id is too long", ErrDashboardAdInvalid, i)
		}
		if _, exists := seen[ad.ID]; exists {
			return nil, fmt.Errorf("%w: duplicated id %s", ErrDashboardAdInvalid, ad.ID)
		}
		seen[ad.ID] = struct{}{}
		ad.ImageURL = strings.TrimSpace(ad.ImageURL)
		ad.LinkURL = strings.TrimSpace(ad.LinkURL)
		ad.FitMode = normalizeDashboardAdFitMode(ad.FitMode)
		if ad.StartsAt != nil && ad.EndsAt != nil && !ad.StartsAt.Before(*ad.EndsAt) {
			return nil, fmt.Errorf("%w: ad %d has invalid schedule", ErrDashboardAdInvalid, i)
		}
		result = append(result, ad)
	}
	return result, nil
}
