package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 普通 Key 在鉴权内也会改写模型，准入仍必须看到全部原始公开名称。
func TestGroupModelAllowlistOrdinaryUsesOriginalModels(t *testing.T) {
	for _, tc := range []struct {
		body   string
		models []string
		want   int
	}{
		{`{"model":"review"}`, []string{"review"}, 200},
		{`{"model":"review"}`, []string{"gpt-5.4"}, 404},
		{`{"model":"review","Model":"blocked"}`, []string{"review"}, 404},
	} {
		key := &service.APIKey{Group: &service.Group{ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: tc.models}}, ModelMapping: map[string]string{"review": "gpt-5.4"}}
		r := gin.New()
		r.POST("/v1/responses", func(c *gin.Context) {
			require.True(t, captureGroupAllowlistModels(c, key))
			applyAPIKeyModelRedirect(c, key)
			c.Set(string(ContextKeyAPIKey), key)
			c.Next()
		}, GroupModelAllowlist(), func(c *gin.Context) { c.Status(200) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, tc.want, w.Code, w.Body.String())
	}
}

// 复合 Key 的前缀改写不能让公开别名用内部目标名称绕过白名单。
func TestGroupModelAllowlistCompositeUsesOriginalModels(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		models     []string
		want       int
	}{
		{name: "公开别名允许", body: `{"model":"GPT/review"}`, models: []string{"review"}, want: 200},
		{name: "内部目标不代替别名授权", body: `{"model":"GPT/review"}`, models: []string{"gpt-5.4"}, want: 404},
		{name: "重复字段不能跨过已选分组", body: `{"model":"GPT/review","Model":"Claude/claude-sonnet-4-6"}`, models: []string{"*"}, want: 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := compositeMiddlewareMultiGroupTestKey()
			key.CompositeGroups[0].Group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: tc.models}
			key.ModelMapping = map[string]string{"review": "gpt-5.4"}
			r := gin.New()
			r.POST("/v1/responses", func(c *gin.Context) {
				if !captureGroupAllowlistModels(c, key) {
					return
				}
				selected, err := resolveCompositeAPIKeyRequest(c, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil), key)
				require.NoError(t, err)
				c.Set(string(ContextKeyAPIKey), selected)
				applyAPIKeyModelRedirect(c, selected)
				c.Next()
			}, GroupModelAllowlist(), func(c *gin.Context) { c.Status(http.StatusOK) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

// 合法的默认分组回退仍校验公开模型，不把回退组 ID 与原前缀所属组误当作跨组请求。
func TestGroupModelAllowlistCompositePreservesFallback(t *testing.T) {
	key := compositeMiddlewareTestKey()
	key.CompositeGroups[0].Group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"review"}}
	r := gin.New()
	r.POST("/v1/responses", func(c *gin.Context) {
		require.True(t, captureGroupAllowlistModels(c, key))
		selected, err := resolveCompositeAPIKeyRequest(c, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil), key)
		require.NoError(t, err)
		fallback := *selected.Group
		fallback.ID = 20
		selected.Group = &fallback
		selected.GroupID = &fallback.ID
		c.Set(string(ContextKeyAPIKey), selected)
		c.Next()
	}, GroupModelAllowlist(), func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"GPT/review"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
}
