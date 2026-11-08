package httputil

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// 请求体读取失败使用稳定且不包含正文的分类标签。
const (
	RequestBodyReadErrorMaxBytes                   = "max_bytes"
	RequestBodyReadErrorTimeout                    = "read_timeout"
	RequestBodyReadErrorClientDisconnect           = "client_disconnect"
	RequestBodyReadErrorTruncated                  = "truncated_body"
	RequestBodyReadErrorDecodeContentEncoding      = "decode_content_encoding"
	RequestBodyReadErrorUnsupportedContentEncoding = "unsupported_content_encoding"
	RequestBodyReadErrorTransport                  = "transport"
	RequestBodyReadErrorIO                         = "io_read"
)

// StatusClientClosedRequest 表示客户端在响应前离开的非标准 499 状态。
const StatusClientClosedRequest = 499

// RequestBodyReadError 记录失败读取的进度计数，不携带请求正文。
type RequestBodyReadError struct {
	Err       error
	BytesRead int64
	Elapsed   time.Duration
}

func (e *RequestBodyReadError) Error() string { return e.Err.Error() }

func (e *RequestBodyReadError) Unwrap() error { return e.Err }

// RequestBodyReadFailure 保存读取失败的响应分类与无正文诊断信息。
type RequestBodyReadFailure struct {
	Kind    string
	Status  int
	Message string
	// MaxBytesLimit 仅在单请求大小超限时设置。
	MaxBytesLimit int64

	contentLength   int64
	contentEncoding string
	protocol        string
	chunked         bool
	bytesRead       int64
	elapsed         time.Duration
}

// DescribeRequestBodyReadFailure 将超时、客户端取消和普通格式错误分别映射为 408、499 和 400。
func DescribeRequestBodyReadFailure(req *http.Request, err error) RequestBodyReadFailure {
	failure := RequestBodyReadFailure{
		Kind:            classifyRequestBodyReadError(req, err),
		contentLength:   -1,
		contentEncoding: "identity",
		bytesRead:       -1,
	}
	if req != nil {
		failure.contentLength = req.ContentLength
		failure.contentEncoding = RequestContentEncodingCategory(req.Header.Get("Content-Encoding"))
		failure.protocol = req.Proto
		failure.chunked = len(req.TransferEncoding) > 0
	}
	var readErr *RequestBodyReadError
	if errors.As(err, &readErr) {
		failure.bytesRead = readErr.BytesRead
		failure.elapsed = readErr.Elapsed
	}

	switch failure.Kind {
	case RequestBodyReadErrorMaxBytes:
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			failure.MaxBytesLimit = maxErr.Limit
		}
		failure.Status = http.StatusRequestEntityTooLarge
		failure.Message = "Request body is too large"
	case RequestBodyReadErrorTimeout:
		failure.Status = http.StatusRequestTimeout
		failure.Message = "Timed out reading request body"
	case RequestBodyReadErrorClientDisconnect:
		failure.Status = StatusClientClosedRequest
		failure.Message = "Client closed the connection before the request body was received"
	default:
		failure.Status = http.StatusBadRequest
		failure.Message = "Failed to read request body"
	}
	return failure
}

// LogFields 不返回请求内容或可能包含对端地址、解码输入的原始错误文本。
func (f RequestBodyReadFailure) LogFields() []zap.Field {
	fields := []zap.Field{
		zap.String("error_kind", f.Kind),
		zap.Int("status", f.Status),
		zap.String("protocol", f.protocol),
		zap.String("content_encoding", f.contentEncoding),
		zap.Int64("content_length", f.contentLength),
		zap.Bool("chunked", f.chunked),
	}
	if f.bytesRead >= 0 {
		fields = append(fields,
			zap.Int64("bytes_read", f.bytesRead),
			zap.Int64("read_ms", f.elapsed.Milliseconds()),
		)
	}
	return fields
}

func classifyRequestBodyReadError(req *http.Request, err error) string {
	if err == nil {
		return RequestBodyReadErrorIO
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return RequestBodyReadErrorMaxBytes
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "decode content-encoding") {
		if strings.Contains(lower, "unsupported content-encoding") {
			return RequestBodyReadErrorUnsupportedContentEncoding
		}
		return RequestBodyReadErrorDecodeContentEncoding
	}
	// HTTP/1 的读取超时也会取消 Context，必须优先识别超时，避免误归类为客户端断开。
	if IsRequestBodyReadTimeout(err) {
		return RequestBodyReadErrorTimeout
	}
	if requestBodyClientGone(req, err, lower) {
		return RequestBodyReadErrorClientDisconnect
	}
	// HTTP/2 在正文少于声明长度时会报告提前 END_STREAM，应按截断正文归类。
	if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(lower, "request declared a content-length") {
		return RequestBodyReadErrorTruncated
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return RequestBodyReadErrorTransport
	}
	return RequestBodyReadErrorIO
}

// IsRequestBodyReadTimeout 识别逐次正文读取截止导致的超时。
func IsRequestBodyReadTimeout(err error) bool {
	var netErr net.Error
	return errors.Is(err, os.ErrDeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())
}

func requestBodyClientGone(req *http.Request, err error, lowerErr string) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	// HTTP/2 通过该固定错误信息报告客户端连接关闭。
	if strings.Contains(lowerErr, "client disconnected") {
		return true
	}
	// 连接失败会取消 Context；此时提前 EOF 应归类为客户端断开。
	return req != nil && errors.Is(req.Context().Err(), context.Canceled)
}

// RequestContentEncodingCategory 将压缩方式归一到有限标签，避免日志高基数。
func RequestContentEncodingCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "identity":
		return "identity"
	case "gzip", "x-gzip":
		return "gzip"
	case "zstd":
		return "zstd"
	case "deflate":
		return "deflate"
	default:
		return "other"
	}
}
