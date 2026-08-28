package admin

import (
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
)

// UpdateDashboardAdsRequest 是广告列表整体替换请求。
type UpdateDashboardAdsRequest struct {
	Ads []service.DashboardAd `json:"ads"`
}

// GetDashboardAds 返回独立表中的广告配置。
func (h *SettingHandler) GetDashboardAds(c *gin.Context) {
	ads, err := h.settingService.GetDashboardAds(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ads)
}

// UpdateDashboardAds 整体替换广告配置，不再经过普通 settings 写入链路。
func (h *SettingHandler) UpdateDashboardAds(c *gin.Context) {
	var req UpdateDashboardAdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingService.ReplaceDashboardAds(c.Request.Context(), req.Ads); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	ads, err := h.settingService.GetDashboardAds(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ads)
}
