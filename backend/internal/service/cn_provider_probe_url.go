package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/util/urlvalidator"
)

// cnValidateProbeURL 校验国产供应商余额与额度探测使用的出站地址。
// 探测会携带账号 API Key，因此必须复用网关的 URL 安全策略。
func cnValidateProbeURL(cfg *config.Config, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("probe url is required")
	}
	if cfg != nil && cfg.Security.URLAllowlist.Enabled {
		normalized, err := urlvalidator.ValidateHTTPSURL(trimmed, urlvalidator.ValidationOptions{
			AllowedHosts:     cfg.Security.URLAllowlist.UpstreamHosts,
			RequireAllowlist: true,
			AllowPrivate:     cfg.Security.URLAllowlist.AllowPrivateHosts,
		})
		if err != nil {
			return "", fmt.Errorf("probe target rejected by URL security policy: %w", err)
		}
		return normalized, nil
	}
	allowInsecureHTTP := cfg != nil && cfg.Security.URLAllowlist.AllowInsecureHTTP
	normalized, err := urlvalidator.ValidateURLFormat(trimmed, allowInsecureHTTP)
	if err != nil {
		return "", fmt.Errorf("probe target rejected by URL security policy: %w", err)
	}
	return normalized, nil
}
