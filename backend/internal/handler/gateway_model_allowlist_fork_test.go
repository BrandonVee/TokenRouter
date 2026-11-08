package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/pkg/gemini"
	"github.com/BrandonVee/TokenRouter/internal/server/middleware"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 列表先扩展 Key 别名再检查公开名称，目标不在白名单时仍能展示合法别名。
func TestGatewayModelsAllowlistKeepsIndependentKeyAlias(t *testing.T) {
	groupID := int64(42)
	key := &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"review"}}}, ModelMapping: map[string]string{"review": "gpt-5.4"}}
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {{ID: 7, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true}}}})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), key)
	h.Models(c)
	require.Equal(t, 200, w.Code)
	data := gjson.Get(w.Body.String(), "data").Array()
	require.Len(t, data, 1, w.Body.String())
	require.Equal(t, "review", data[0].Get("id").String())
}

// 原生 Gemini 列表保留返回对象的元数据，禁止模型不会通过别名追加泄露到列表。
func TestGeminiModelsAllowlistPreservesAliasMetadata(t *testing.T) {
	key := &service.APIKey{Group: &service.Group{ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"review"}}}, ModelMapping: map[string]string{"review": "gemini-3.5-flash"}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	writeGeminiModelsListWithAPIKeyAliases(c, gemini.ModelsListResponse{Models: []gemini.Model{gemini.FallbackModel("gemini-3.5-flash")}}, key)
	models := gjson.Get(w.Body.String(), "models").Array()
	require.Len(t, models, 1, w.Body.String())
	require.Equal(t, "models/review", models[0].Get("name").String())
	require.True(t, models[0].Get("supportedGenerationMethods").IsArray())
}
