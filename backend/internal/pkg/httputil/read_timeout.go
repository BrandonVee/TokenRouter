package httputil

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// WithBodyReadTimeout 仅限制接收正文时每次套接字读取的等待，不设置全请求截止，也不限制 SSE 响应总时长。
func WithBodyReadTimeout(next http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody {
			body := &bodyReadTimeout{ReadCloser: r.Body, controller: http.NewResponseController(w), timeout: timeout}
			r.Body = body
			if r.ProtoMajor == 1 {
				writer := &bodyDrainTimeoutWriter{ResponseWriter: w, body: body}
				defer func() {
					if !writer.hijacked.Load() {
						// net/http 在处理器退出后会关闭原始正文，绕过本包装的 Read 和 Close。
						body.startDrain()
					}
				}()
				next.ServeHTTP(writer, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type bodyReadTimeout struct {
	io.ReadCloser
	controller *http.ResponseController
	timeout    time.Duration
	finished   atomic.Bool
	failed     atomic.Bool
}

func (r *bodyReadTimeout) Read(p []byte) (int, error) {
	err := r.controller.SetReadDeadline(time.Now().Add(r.timeout))
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		return 0, err
	}
	n, readErr := r.ReadCloser.Read(p)
	if errors.Is(readErr, io.EOF) {
		r.finished.Store(true)
	} else if readErr != nil {
		r.failed.Store(true)
	}
	if readErr == nil || errors.Is(readErr, io.EOF) {
		_ = r.controller.SetReadDeadline(time.Time{})
	}
	// 失败时保留读取截止；net/http 可能在处理器退出后排空 HTTP/1 剩余正文，清除超时会再次阻塞。
	return n, readErr
}

func (r *bodyReadTimeout) Close() error {
	armed := r.startDrain()
	err := r.ReadCloser.Close()
	if err == nil {
		r.finished.Store(true)
	} else {
		r.failed.Store(true)
	}
	r.finishDrain(armed)
	return err
}

func (r *bodyReadTimeout) startDrain() bool {
	if r.finished.Load() || r.failed.Load() {
		return false
	}
	return r.controller.SetReadDeadline(time.Now().Add(r.timeout)) == nil
}

func (r *bodyReadTimeout) finishDrain(armed bool) {
	if armed && !r.failed.Load() {
		_ = r.controller.SetReadDeadline(time.Time{})
	}
}

// HTTP/1 在首次写响应、刷新和处理器退出时都可能隐式排空未读正文，必须保护这些操作；HTTP/2 不会这样排空。
type bodyDrainTimeoutWriter struct {
	http.ResponseWriter
	body       *bodyReadTimeout
	hijacked   atomic.Bool
	fullDuplex atomic.Bool
}

func (w *bodyDrainTimeoutWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *bodyDrainTimeoutWriter) startDrain() bool {
	return !w.fullDuplex.Load() && w.body.startDrain()
}

func (w *bodyDrainTimeoutWriter) Write(p []byte) (int, error) {
	armed := w.startDrain()
	defer w.body.finishDrain(armed)
	return w.ResponseWriter.Write(p)
}

func (w *bodyDrainTimeoutWriter) WriteString(s string) (int, error) {
	armed := w.startDrain()
	defer w.body.finishDrain(armed)
	return io.WriteString(w.ResponseWriter, s)
}

func (w *bodyDrainTimeoutWriter) WriteHeader(status int) {
	armed := w.startDrain()
	defer w.body.finishDrain(armed)
	w.ResponseWriter.WriteHeader(status)
}

func (w *bodyDrainTimeoutWriter) FlushError() error {
	armed := w.startDrain()
	defer w.body.finishDrain(armed)
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *bodyDrainTimeoutWriter) Flush() { _ = w.FlushError() }

func (w *bodyDrainTimeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked.Store(true)
		_ = conn.SetReadDeadline(time.Time{})
	}
	return conn, rw, err
}

func (w *bodyDrainTimeoutWriter) EnableFullDuplex() error {
	if err := http.NewResponseController(w.ResponseWriter).EnableFullDuplex(); err != nil {
		return err
	}
	w.fullDuplex.Store(true)
	return nil
}

func (w *bodyDrainTimeoutWriter) CloseNotify() <-chan bool {
	if notifier, ok := w.ResponseWriter.(http.CloseNotifier); ok { //nolint:staticcheck // Gin 仍要求转发旧版 CloseNotify 接口。
		return notifier.CloseNotify()
	}
	return make(chan bool)
}
