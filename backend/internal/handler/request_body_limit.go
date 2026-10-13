package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/BrandonVee/TokenRouter/internal/config"
	pkghttputil "github.com/BrandonVee/TokenRouter/internal/pkg/httputil"
	"github.com/BrandonVee/TokenRouter/internal/service"
)

func extractMaxBytesError(err error) (*http.MaxBytesError, bool) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return maxErr, true
	}
	return nil, false
}

func formatBodyLimit(limit int64) string {
	const mb = 1024 * 1024
	if limit >= mb {
		return fmt.Sprintf("%dMB", limit/mb)
	}
	return fmt.Sprintf("%dB", limit)
}

func buildBodyTooLargeMessage(limit int64) string {
	return fmt.Sprintf("Request body too large, limit is %s", formatBodyLimit(limit))
}

// readLenientJSONRequestBodyWithPrealloc 按网关请求体上限读取并规范化 JSON。
func readLenientJSONRequestBodyWithPrealloc(req *http.Request, cfg *config.Config) ([]byte, error) {
	body, err := pkghttputil.ReadLenientJSONRequestBodyWithPrealloc(req, gatewayMaxBodySize(cfg))
	if err != nil {
		return nil, err
	}
	return normalizeGatewayRoutingRequest(req, body)
}

// normalizeGatewayRoutingRequest 在模型校验和账号选择前消费路由扩展。
func normalizeGatewayRoutingRequest(req *http.Request, body []byte) ([]byte, error) {
	body, strategy, err := service.NormalizeGatewayRoutingOverrides(body)
	if err != nil {
		return nil, err
	}
	*req = *req.WithContext(service.WithAPIKeyRoutingStrategy(req.Context(), strategy))
	return body, nil
}

// gatewayMaxBodySize 读取网关配置的请求体上限，空配置由 httputil 使用保守默认值。
func gatewayMaxBodySize(cfg *config.Config) int64 {
	if cfg == nil {
		return 0
	}
	return cfg.Gateway.MaxBodySize
}
