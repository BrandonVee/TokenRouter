package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/pkg/httputil"
	"github.com/BrandonVee/TokenRouter/internal/pkg/requestmodel"
	"github.com/BrandonVee/TokenRouter/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupModelAllowlist 是分组级模型白名单准入中间件。
//
// 挂载位置：每条网关链的 apiKeyAuth 之后、compositeTarget 之前——
// 保证校验发生在合成路由改写与调度之前，且只看客户端书写的公开模型名。
//
// 行为：
//   - 快速路径：未绑定分组或白名单未开启时直接放行，不读请求体。
//   - Responses WebSocket 入口跳过（首帧与后续 turn 由 ResponsesWebSocket 逐帧
//     校验）；Grok Realtime 的升级请求模型固定在查询参数里，仍走中间件校验，
//     其他路由伪造 Upgrade 头不得绕过校验。
//   - GET/DELETE：从路由参数（model / modelAction）或查询参数（/realtime 的
//     model）提取模型；提取不到放行，由 handler 决定是否报「model is required」。
//   - 带请求体的方法：读体（PrereadBody 回填，下游零拷贝）、提取 JSON
//     `model`/`session.model` 或 multipart `model`/`session` 后回填请求体。
//   - 拒绝：按入口协议格式返回 404，并标记运维业务限流原因
//     local_model_configuration 与 ingress 拒绝原因 model_not_allowed。
//
// @project-doc docs/domains/gateway_policy_controls.md#group_model_allowlist
func GroupModelAllowlist() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !apiKey.Group.ModelAllowlistEnabled() {
			c.Next()
			return
		}
		allowlist := apiKey.Group.ModelAllowlist
		if c.Request == nil {
			c.Next()
			return
		}

		if isResponsesWebSocketRoute(c) {
			// Responses WS 长连接由 ResponsesWebSocket 校验首帧与每个 response.create。
			c.Next()
			return
		}
		// models 收集该请求全部可被下游解析器绑定到的模型值（路径参数/查询参数
		// 单值；请求体候选集含重复键与大小写变体），逐一校验，任一未命中即拒绝。
		models, captured, invalid := capturedGroupAllowlistModels(c, apiKey)
		if !captured {
			if model := groupModelAllowlistModelFromParams(c); model != "" {
				models = []string{model}
			} else {
				switch c.Request.Method {
				case http.MethodPost, http.MethodPut, http.MethodPatch:
					candidates, done := groupModelAllowlistModelsFromBody(c)
					if !done {
						// 请求体读取失败（如超限 413）已写出响应。
						return
					}
					models = candidates
				}
				if len(models) == 0 {
					// Grok Realtime 升级请求把模型固定在查询参数里。
					if model := strings.TrimSpace(c.Query("model")); model != "" {
						models = []string{model}
					}
				}
			}

		}
		blocked := invalid
		for _, candidate := range models {
			if blocked != "" {
				break
			}
			if !allowlist.Allows(candidate) {
				blocked = candidate
				break
			}
		}
		if blocked == "" {
			c.Next()
			return
		}

		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		service.MarkOpsRequestScopedError(c, "not_found_error")
		MarkIngressRejected(c, IngressRejectModelNotAllowed)
		groupModelAllowlistErrorWriter(c)(c, http.StatusNotFound, fmt.Sprintf("Model %q is not available for this group", blocked))
		c.Abort()
	}
}

// isResponsesWebSocketRoute 判断当前请求是否命中 OpenAI Responses WebSocket
// 入口。只有这三条路由的模型在升级后的帧里，交给 handler 逐帧校验；其他路由
// 即使携带 WebSocket Upgrade 头也必须经过本中间件校验。
func isResponsesWebSocketRoute(c *gin.Context) bool {
	if c.Request == nil || c.Request.Method != http.MethodGet {
		return false
	}
	switch c.FullPath() {
	case "/v1/responses", "/responses", "/backend-api/codex/responses":
		return true
	}
	return false
}

// groupModelAllowlistModelsFromBody 读取请求体并提取客户端模型名，随后把请求体
// 回填（PrereadBody），保证后续 handler 零拷贝重读。读取失败按现有合成中间件
// 的方式返回 400/413（返回 false 表示已写出响应并 Abort）。
//
// 下游同时存在 gjson（首个、大小写敏感）、encoding/json 绑定（末值、大小写
// 不敏感）与 multipart 表单（首/末字段）三类解析器，这里返回「任一解析器可能
// 绑定到的全部模型值」，调用方必须逐一校验，任一未命中即拒绝。
func groupModelAllowlistModelsFromBody(c *gin.Context) ([]string, bool) {
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if RejectRequestBodyReadError(c, "middleware.group_model_allowlist", err) {
			return nil, false
		}
		AbortWithRequestBodyReadFailure(c, "middleware.group_model_allowlist", err)
		return nil, false
	}
	requestmodel.ResetRequestBody(c.Request, body)
	return requestmodel.FromBodyCandidates(c.FullPath(), c.GetHeader("Content-Type"), body), true
}

// groupModelAllowlistModelFromParams 从路由参数提取模型名：Gemini 原生 URL 的
// `model` / `modelAction`（去掉 `:action` 后缀）。提取不到返回空串，
// 由调用方决定是否继续读请求体或查询参数。
func groupModelAllowlistModelFromParams(c *gin.Context) string {
	if model := strings.TrimSpace(c.Param("model")); model != "" {
		return strings.TrimPrefix(model, "/")
	}
	if modelAction := strings.TrimSpace(c.Param("modelAction")); modelAction != "" {
		modelAction = strings.TrimPrefix(modelAction, "/")
		if idx := strings.LastIndex(modelAction, ":"); idx >= 0 {
			return strings.TrimSpace(modelAction[:idx])
		}
		return modelAction
	}
	return ""
}

// groupModelAllowlistErrorWriter 按入口协议选择错误格式：
// Gemini 原生（/v1beta、/antigravity/v1beta）用 Google 格式；
// Messages 入口（含根路径别名与 /antigravity/v1）用 Anthropic 格式；
// 其余（OpenAI 兼容入口）用 OpenAI 格式。
func groupModelAllowlistErrorWriter(c *gin.Context) GatewayErrorWriter {
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	switch {
	case strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/antigravity/v1beta"):
		return GoogleErrorWriter
	case strings.Contains(path, "/messages"):
		return AnthropicErrorWriter
	default:
		return OpenAIErrorWriter
	}
}

type compositeAllowlistCandidates struct {
	models []string
	source *service.APIKey
}

const compositeAllowlistCandidatesKey = "composite_allowlist_original_models"

// captureGroupAllowlistModels 在前缀和 Key 别名改写前保留全部候选，避免公开准入依赖内部名称。
func captureGroupAllowlistModels(c *gin.Context, key *service.APIKey) bool {
	if key == nil || c == nil || c.Request == nil {
		return true
	}
	// 复合 Key 本来就需要解析模型；同时覆盖禁用组回退到启用白名单组的情况。
	needed := key.IsComposite || key.Group.ModelAllowlistEnabled() || len(key.ModelMapping) > 0
	if !needed || isResponsesWebSocketRoute(c) {
		return true
	}
	var models []string
	if model := groupModelAllowlistModelFromParams(c); model != "" {
		models = []string{model}
	} else if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
		var ok bool
		models, ok = groupModelAllowlistModelsFromBody(c)
		if !ok {
			return false
		}
	}
	c.Set(compositeAllowlistCandidatesKey, &compositeAllowlistCandidates{models: models, source: key})
	return true
}

// capturedGroupAllowlistModels 用选中分组的公开模型校验，不让其它前缀或重定向后的名字绕过策略。
func capturedGroupAllowlistModels(c *gin.Context, selected *service.APIKey) ([]string, bool, string) {
	value, exists := c.Get(compositeAllowlistCandidatesKey)
	if !exists {
		return nil, false, ""
	}
	captured, ok := value.(*compositeAllowlistCandidates)
	if !ok || captured == nil {
		return nil, false, ""
	}
	if !captured.source.IsComposite {
		return captured.models, true, ""
	}
	models := make([]string, 0, len(captured.models))
	expectedGroupID := int64(0)
	if original, _, ok := GetCompositeModelFromContext(c); ok {
		if binding, _, err := captured.source.ResolveCompositeModel(original); err == nil && binding != nil {
			expectedGroupID = binding.GroupID
		}
	}
	for _, candidate := range captured.models {
		binding, model, err := captured.source.ResolveCompositeModel(candidate)
		if err != nil || binding == nil || selected.Group == nil || (expectedGroupID > 0 && binding.GroupID != expectedGroupID) {
			return nil, true, candidate
		}
		if expectedGroupID == 0 {
			expectedGroupID = binding.GroupID
		}
		models = append(models, model)
	}
	return models, true, ""
}
