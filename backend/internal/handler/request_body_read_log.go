package handler

import (
	"net/http"

	pkghttputil "github.com/BrandonVee/TokenRouter/internal/pkg/httputil"
	"go.uber.org/zap"
)

// requestBodyReadFailure 将读取失败映射为稳定的状态和消息；超时返回 408，其他读取错误保留通用 400。除单请求超限外记录有限进度指标，不记录正文或原始错误。
func requestBodyReadFailure(reqLog *zap.Logger, req *http.Request, err error) (int, string) {
	failure := pkghttputil.DescribeRequestBodyReadFailure(req, err)
	if failure.Kind == pkghttputil.RequestBodyReadErrorMaxBytes {
		return failure.Status, buildBodyTooLargeMessage(failure.MaxBytesLimit)
	}
	logRequestBodyReadFailure(reqLog, failure)
	return failure.Status, failure.Message
}

func logRequestBodyReadFailure(reqLog *zap.Logger, failure pkghttputil.RequestBodyReadFailure) {
	if reqLog != nil {
		reqLog.Warn("read request body failed", failure.LogFields()...)
	}
}
