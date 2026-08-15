package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAINativeCompactionV2Key     = "openai_native_compaction_v2"
	openAIRemoteCompactionV2Feature = "remote_compaction_v2"
)

// MarkOpenAINativeCompactionV2 标记当前请求采用原生 remote compaction v2。
func MarkOpenAINativeCompactionV2(c *gin.Context) {
	if c != nil {
		c.Set(openAINativeCompactionV2Key, true)
	}
}

func isOpenAINativeCompactionV2(c *gin.Context) bool {
	return c != nil && c.GetBool(openAINativeCompactionV2Key)
}

// ensureOpenAIRemoteCompactionV2BetaFeature 确保协商头包含 v2 功能标识。
func ensureOpenAIRemoteCompactionV2BetaFeature(headers http.Header) {
	if headers == nil {
		return
	}
	tokens := make([]string, 0, 4)
	for _, value := range headers.Values("x-codex-beta-features") {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if token == openAIRemoteCompactionV2Feature {
				return
			}
			tokens = append(tokens, token)
		}
	}
	tokens = append(tokens, openAIRemoteCompactionV2Feature)
	headers.Set("x-codex-beta-features", strings.Join(tokens, ","))
}

func hasOpenAICodexBetaFeaturesHeader(headers http.Header) bool {
	if headers == nil {
		return false
	}
	for _, value := range headers.Values("x-codex-beta-features") {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// applyOpenAICodexBetaFeatures 对齐 Codex 会话级 beta 头行为。
// OAuth 普通请求在客户端未声明时补默认值；原生 v2 请求始终确保功能标识存在。
func applyOpenAICodexBetaFeatures(c *gin.Context, account *Account, headers http.Header) {
	if headers == nil {
		return
	}
	if isOpenAINativeCompactionV2(c) {
		ensureOpenAIRemoteCompactionV2BetaFeature(headers)
		return
	}
	if account == nil || !account.IsOpenAIOAuth() || hasOpenAICodexBetaFeaturesHeader(headers) {
		return
	}
	headers.Set("x-codex-beta-features", openAIRemoteCompactionV2Feature)
}

// HasCompactionTriggerInInput 检测 input 中 type="compaction_trigger" 的条目。
// handler 会结合请求路径、stream 字段和 Codex beta feature 请求头，区分原生
// remote compaction v2 流式协议与旧的 /responses/compact 桥接协议。
func HasCompactionTriggerInInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "compaction_trigger" {
			found = true
			return false
		}
		return true
	})
	return found
}
