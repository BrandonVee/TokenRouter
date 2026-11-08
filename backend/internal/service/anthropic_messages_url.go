package service

import (
	"net/url"
	"strings"
)

// buildAnthropicMessagesURL 为已验证的 base URL 拼接 Messages 端点。
// 仅 Cloudflare 账号 REST 端点拒绝 beta 参数，其原生网关和其他上游仍保留该参数。
func buildAnthropicMessagesURL(baseURL string) string {
	targetURL := strings.TrimRight(baseURL, "/") + "/v1/messages"
	parsed, err := url.Parse(targetURL)
	if err == nil && strings.EqualFold(parsed.Scheme, "https") && parsed.User == nil &&
		parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" && parsed.RawPath == "" {
		host := strings.ToLower(parsed.Host)
		parts := strings.Split(parsed.Path, "/")
		if (host == "api.cloudflare.com" || host == "api.cloudflare.com:443") &&
			len(parts) == 8 && parts[0] == "" && parts[1] == "client" && parts[2] == "v4" &&
			parts[3] == "accounts" && parts[4] != "" && parts[4] != "." && parts[4] != ".." &&
			parts[5] == "ai" && parts[6] == "v1" && parts[7] == "messages" {
			return targetURL
		}
	}
	return targetURL + "?beta=true"
}
