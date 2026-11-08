package admin

import (
	"context"
	"github.com/BrandonVee/TokenRouter/internal/pkg/adminaccess"
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

type claudeResetReader interface {
	Query(context.Context, int64) (*service.ClaudeResetCredits, error)
	Redeem(context.Context, int64, string) (*service.ClaudeResetOutcome, error)
}

func (h *AccountHandler) SetClaudeResetCreditService(s *service.ClaudeResetCreditService) {
	h.claudeResetCredits = s
}
func (h *AccountHandler) ClaudeResetCredits(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if h.claudeResetCredits == nil {
		response.Error(c, 503, "Claude reset service unavailable")
		return
	}
	status, err := h.claudeResetCredits.Query(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// RedeemClaudeResetCredit 消费服务端选中的下一次重置机会；Idempotency-Key 对应一次确认，重试仅重放结果。
func (h *AccountHandler) RedeemClaudeResetCredit(c *gin.Context) {
	c.Request = c.Request.WithContext(service.WithScopedAdminAccountOperation(c.Request.Context(), "claude.quota.reset"))
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if h.claudeResetCredits == nil {
		response.Error(c, 503, "Claude reset service unavailable")
		return
	}
	result, err := h.claudeResetCredits.Redeem(c.Request.Context(), id, c.GetHeader("Idempotency-Key"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 消费权限不隐式授予完整次数读取；仅保留操作结果与已清理窗口。
	if p := adminaccess.RestrictedFromContext(c.Request.Context()); p != nil && result != nil {
		a, err := h.adminService.GetAccount(c.Request.Context(), id)
		if err != nil || a == nil || !p.Policy.AllowsAccount("claude.quota.read", scopedAccountResource(a)) {
			safe := *result
			safe.Credits = nil
			result = &safe
		}
	}
	response.Success(c, result)
}
