package admin

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/pkg/adminaccess"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type resetScopeAdmin struct{ service.AdminService }

func (resetScopeAdmin) GetAccount(context.Context, int64) (*service.Account, error) {
	return &service.Account{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, GroupIDs: []int64{2}}, nil
}

type resetScopeReader struct{ claudeResetHandlerStub }

func (resetScopeReader) Redeem(context.Context, int64, string) (*service.ClaudeResetOutcome, error) {
	return &service.ClaudeResetOutcome{Outcome: service.ClaudeResetOutcomeReset, Credits: &service.ClaudeResetCredits{AvailableCount: 2}}, nil
}

// 只授予消费动作的集成不能借成功响应读取完整次数；读取权限须单独授予。
func TestClaudeResetScopedWriteDoesNotGrantRead(t *testing.T) {
	for _, read := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/accounts/1/claude/reset-credits/redeem", nil)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		permissions := []string{"claude.quota.reset"}
		if read {
			permissions = append(permissions, "claude.quota.read")
		}
		policy := adminaccess.Policy{Rules: []adminaccess.Rule{{Permissions: permissions, GroupIDs: []int64{2}, AccountTypes: []string{service.AccountTypeOAuth}}}}
		c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{Policy: policy}))
		h := &AccountHandler{adminService: resetScopeAdmin{}, claudeResetCredits: &resetScopeReader{}}
		h.RedeemClaudeResetCredit(c)
		require.Equal(t, 200, w.Code)
		if read {
			require.Contains(t, w.Body.String(), "available_count")
		} else {
			require.NotContains(t, w.Body.String(), "available_count")
		}
	}
}
