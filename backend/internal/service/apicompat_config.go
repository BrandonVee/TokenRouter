package service

import (
	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/apicompat"
)

// @project-doc docs/interfaces/configuration.md#configuration_sources
// configuredMinOutputTokens 兼容未注入配置的旧调用方，并裁定协议转换下限。
func configuredMinOutputTokens(cfg *config.Config) int {
	if cfg != nil && cfg.Gateway.MinOutputTokens >= 16 {
		return cfg.Gateway.MinOutputTokens
	}
	return apicompat.DefaultMinMaxOutputTokens
}

func chatCompletionsToResponses(cfg *config.Config, req *apicompat.ChatCompletionsRequest) (*apicompat.ResponsesRequest, error) {
	return apicompat.ChatCompletionsToResponsesWithMinOutputTokens(req, configuredMinOutputTokens(cfg))
}

func anthropicToResponses(cfg *config.Config, req *apicompat.AnthropicRequest) (*apicompat.ResponsesRequest, error) {
	return apicompat.AnthropicToResponsesWithMinOutputTokens(req, configuredMinOutputTokens(cfg))
}

func settingServiceConfig(settingService *SettingService) *config.Config {
	if settingService == nil {
		return nil
	}
	return settingService.cfg
}
