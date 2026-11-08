package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	grokMissingUsageErrorCode     = "grok_missing_usage"
	grokMissingUsageMessage       = "xAI upstream returned a successful chat completion without billable usage"
	deepseekMissingUsageErrorCode = "deepseek_missing_usage"
	deepseekMissingUsageMessage   = "DeepSeek upstream returned a completion without billable usage"
)

// hasBillableGrokChatUsage 只检查聊天结算实际使用的聚合 token 桶。
// 明细字段本身不能证明响应可安全结算，至少一个聚合桶必须为正数。
func hasBillableGrokChatUsage(usage OpenAIUsage) bool {
	return hasBillableOpenAIChatUsage(usage)
}

// hasBillableOpenAIChatUsage 判断文本结算是否至少观测到一个实际参与计费的 token 桶。
func hasBillableOpenAIChatUsage(usage OpenAIUsage) bool {
	return usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0
}

// requiresBillableGrokChatUsage 根据实际账号平台和最终模型身份识别 Grok 流量。
// Grok 可由通用 OpenAI 兼容账号承载，因此不能只检查 account.Platform；同时不使用
// 未映射的客户端模型，避免 Grok 命名别名映射到非 Grok 上游时被误判。
func requiresBillableGrokChatUsage(account *Account, models ...string) bool {
	if account != nil && account.Platform == PlatformGrok {
		return true
	}
	for _, model := range models {
		normalized := strings.ToLower(strings.TrimSpace(model))
		if separator := strings.LastIndex(normalized, "/"); separator >= 0 {
			normalized = strings.TrimSpace(normalized[separator+1:])
		}
		if normalized == "grok" || strings.HasPrefix(normalized, "grok-") {
			return true
		}
	}
	return false
}

// requiresBillableDeepseekUsage 按实际平台和最终模型识别 DeepSeek，兼容第三方 OpenAI 账号。
func requiresBillableDeepseekUsage(account *Account, models ...string) bool {
	if account != nil && account.Platform == PlatformDeepseek {
		return true
	}
	for _, model := range models {
		normalized := strings.ToLower(strings.TrimSpace(model))
		if separator := strings.LastIndex(normalized, "/"); separator >= 0 {
			normalized = strings.TrimSpace(normalized[separator+1:])
		}
		if normalized == "deepseek" || strings.HasPrefix(normalized, "deepseek-") {
			return true
		}
	}
	return false
}

// validateDeepseekUsage 不把未返回 token 的响应作为成功结算，也不根据请求内容猜测扣费。
// 调用方必须在发送正常结束事件前校验，已写出的流由 handler 的边界检查禁止切号重放。
// @project-doc docs/interfaces/openai_upstream.md#openai_protocol_dispatch
func validateDeepseekUsage(c *gin.Context, account *Account, requestID string, usage OpenAIUsage, models ...string) error {
	if !requiresBillableDeepseekUsage(account, models...) || hasBillableOpenAIChatUsage(usage) {
		return nil
	}
	platform := PlatformDeepseek
	if account != nil {
		platform = account.Platform
	}
	return newChatMissingUsageFailoverError(c, account, requestID, platform, deepseekMissingUsageErrorCode, deepseekMissingUsageMessage)
}

// newGrokMissingUsageFailoverError 构造稳定的缺失用量故障转移错误并写入 Grok Ops 诊断。
func newGrokMissingUsageFailoverError(c *gin.Context, account *Account, upstreamRequestID string) *UpstreamFailoverError {
	return newChatMissingUsageFailoverError(c, account, upstreamRequestID, PlatformGrok, grokMissingUsageErrorCode, grokMissingUsageMessage)
}

// newChatMissingUsageFailoverError 为缺失用量保留可关联的上游诊断，不记录响应正文或用户内容。
func newChatMissingUsageFailoverError(c *gin.Context, account *Account, upstreamRequestID, platform, code, message string) *UpstreamFailoverError {
	accountID := int64(0)
	accountName := ""
	if account != nil {
		accountID = account.ID
		accountName = account.Name
	}

	setOpsUpstreamError(c, http.StatusBadGateway, message, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           platform,
		AccountID:          accountID,
		AccountName:        accountName,
		UpstreamStatusCode: http.StatusBadGateway,
		UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
		Kind:               "failover",
		Message:            message,
	})

	body, _ := json.Marshal(gin.H{
		"error": gin.H{
			"type":    "upstream_error",
			"code":    code,
			"message": message,
		},
	})
	headers := http.Header{}
	if requestID := strings.TrimSpace(upstreamRequestID); requestID != "" {
		headers.Set("x-request-id", requestID)
	}
	return &UpstreamFailoverError{
		StatusCode:      http.StatusBadGateway,
		ResponseBody:    body,
		ResponseHeaders: headers,
	}
}
