package repository

import (
	"context"

	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/ent/dashboardad"
	"github.com/BrandonVee/TokenRouter/internal/service"
)

type dashboardAdRepository struct {
	client *dbent.Client
}

// NewDashboardAdRepository 创建仪表盘广告仓储。
func NewDashboardAdRepository(client *dbent.Client) service.DashboardAdRepository {
	return &dashboardAdRepository{client: client}
}

func (r *dashboardAdRepository) List(ctx context.Context) ([]service.DashboardAd, error) {
	items, err := r.client.DashboardAd.Query().
		Order(dashboardad.BySortOrder(), dashboardad.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]service.DashboardAd, 0, len(items))
	for _, item := range items {
		result = append(result, dashboardAdEntityToService(item))
	}
	return result, nil
}

func (r *dashboardAdRepository) Replace(ctx context.Context, ads []service.DashboardAd) error {
	client := clientFromContext(ctx, r.client)
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	if _, err := tx.DashboardAd.Delete().Exec(ctx); err != nil {
		rollback()
		return err
	}
	for i := range ads {
		ad := ads[i]
		builder := tx.DashboardAd.Create().
			SetID(ad.ID).
			SetImageURL(ad.ImageURL).
			SetLinkURL(ad.LinkURL).
			SetFitMode(dashboardad.FitMode(normalizeDashboardAdFitModeForRepository(ad.FitMode))).
			SetEnabled(ad.Enabled).
			SetSortOrder(i)
		if ad.StartsAt != nil {
			builder.SetStartsAt(*ad.StartsAt)
		}
		if ad.EndsAt != nil {
			builder.SetEndsAt(*ad.EndsAt)
		}
		if _, err := builder.Save(ctx); err != nil {
			rollback()
			return err
		}
	}
	return tx.Commit()
}

func dashboardAdEntityToService(item *dbent.DashboardAd) service.DashboardAd {
	return service.DashboardAd{
		ID:       item.ID,
		ImageURL: item.ImageURL,
		LinkURL:  item.LinkURL,
		FitMode:  string(item.FitMode),
		StartsAt: item.StartsAt,
		EndsAt:   item.EndsAt,
		Enabled:  item.Enabled,
	}
}

// normalizeDashboardAdFitModeForRepository 保证仓储写入值符合 Ent 枚举约束。
func normalizeDashboardAdFitModeForRepository(mode string) string {
	switch mode {
	case service.DashboardAdFitModeCover, service.DashboardAdFitModeFill:
		return mode
	default:
		return service.DashboardAdFitModeAdaptive
	}
}
