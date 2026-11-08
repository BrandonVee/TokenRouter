package handler

import (
	"net/http"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// profitControlledRequest 明确限定 token 生成及缓存创建，媒体、元数据与免费统计不安装利润门。
func profitControlledRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	path := strings.TrimRight(strings.ToLower(c.Request.URL.Path), "/")
	for _, suffix := range []string{"/messages", "/chat/completions", "/responses", "/responses/compact", "/systemone"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	if c.Request.Method == http.MethodPost && strings.HasSuffix(path, "/cachedcontents") {
		return true
	}
	return strings.Contains(path, "/models/") && (strings.HasSuffix(path, ":generatecontent") || strings.HasSuffix(path, ":streamgeneratecontent"))
}

// admitGatewayProfitSelection 在槽位到手后重放实际调度分组的门，失败方负责记录否决并重选。
func admitGatewayProfitSelection(c *gin.Context, gateway *service.GatewayService, groupID *int64, session string, selection *service.AccountSelectionResult, release func(), log *zap.Logger) (*service.Account, bool) {
	if selection == nil || selection.Account == nil {
		return nil, false
	}
	if !selection.ProfitGateActive() {
		return selection.Account, true
	}
	ctx := service.ContextWithSelectionProfitGate(c.Request.Context(), selection)
	latest, vetoed, reason := gateway.GatewayProfitControlVetoLatest(ctx, selection.Account)
	if vetoed {
		if release != nil {
			release()
		}
		log.Debug("gateway.account_profit_vetoed", zap.Int64("account_id", selection.Account.ID), zap.String("reason", reason))
		return latest, false
	}
	selection.Account = latest
	if err := gateway.BindStickySessionAfterProfitAdmission(ctx, groupID, session, latest.ID); err != nil {
		log.Warn("gateway.bind_after_profit_admission_failed", zap.Error(err))
	}
	return latest, true
}
