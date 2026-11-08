package httputil

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// WithWriteTimeout 限制每次写入或刷新的阻塞时间，完成后清除截止，允许上游长时间推理；位于 Gin 下方以覆盖所有写入包装，HTTP/2 截止仅作用于当前流。
func WithWriteTimeout(next http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		writer := &writeTimeoutWriter{ResponseWriter: w, timeout: timeout, cancel: cancel}
		next.ServeHTTP(writer, r.WithContext(ctx))
	})
}

type writeTimeoutWriter struct {
	http.ResponseWriter
	timeout time.Duration
	cancel  context.CancelFunc
	err     error
}

func (w *writeTimeoutWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *writeTimeoutWriter) start() error {
	if w.err != nil {
		return w.err
	}
	err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Now().Add(w.timeout))
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		w.fail(err)
	}
	return w.err
}

func (w *writeTimeoutWriter) finish() {
	_ = http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Time{})
}

func (w *writeTimeoutWriter) fail(err error) {
	if err != nil {
		w.err = err
		w.cancel()
	}
}

func (w *writeTimeoutWriter) Write(p []byte) (int, error) {
	if err := w.start(); err != nil {
		return 0, err
	}
	defer w.finish()
	n, err := w.ResponseWriter.Write(p)
	w.fail(err)
	return n, err
}

func (w *writeTimeoutWriter) WriteString(s string) (int, error) {
	if err := w.start(); err != nil {
		return 0, err
	}
	defer w.finish()
	n, err := io.WriteString(w.ResponseWriter, s)
	w.fail(err)
	return n, err
}

func (w *writeTimeoutWriter) WriteHeader(status int) {
	if w.start() != nil {
		return
	}
	defer w.finish()
	w.ResponseWriter.WriteHeader(status)
}

func (w *writeTimeoutWriter) FlushError() error {
	if err := w.start(); err != nil {
		return err
	}
	defer w.finish()
	err := http.NewResponseController(w.ResponseWriter).Flush()
	w.fail(err)
	return err
}

func (w *writeTimeoutWriter) Flush() { _ = w.FlushError() }

func (w *writeTimeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *writeTimeoutWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (w *writeTimeoutWriter) CloseNotify() <-chan bool {
	if notifier, ok := w.ResponseWriter.(http.CloseNotifier); ok { //nolint:staticcheck // Gin 仍要求转发旧版 CloseNotify 接口。
		return notifier.CloseNotify()
	}
	return make(chan bool)
}
