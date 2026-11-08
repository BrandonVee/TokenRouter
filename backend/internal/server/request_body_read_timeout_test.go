//go:build unit

package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/handler"
	pkghttputil "github.com/BrandonVee/TokenRouter/internal/pkg/httputil"
	"github.com/BrandonVee/TokenRouter/internal/server/middleware"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testBodyReadTimeout = 300 * time.Millisecond

// bodyKindHandler 通过共享读取器读取正文并返回失败类别及状态。
func bodyKindHandler(kinds chan<- string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(r)
		if err != nil {
			failure := pkghttputil.DescribeRequestBodyReadFailure(r, err)
			if kinds != nil {
				kinds <- failure.Kind
			}
			w.Header().Set("X-Body-Error", failure.Kind)
			w.WriteHeader(failure.Status)
			return
		}
		_, _ = io.WriteString(w, strconv.Itoa(len(body)))
	})
}

func newBodyReadTimeoutTestServer(t *testing.T, h http.Handler, h2c bool) (*httptest.Server, *http.Client) {
	t.Helper()
	ts := httptest.NewUnstartedServer(pkghttputil.WithBodyReadTimeout(h, testBodyReadTimeout))
	transport := &http.Transport{}
	if h2c {
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		protocols.SetUnencryptedHTTP2(true)
		ts.Config.Protocols = protocols
		clientProtocols := new(http.Protocols)
		clientProtocols.SetUnencryptedHTTP2(true)
		transport.Protocols = clientProtocols
	}
	ts.Start()
	t.Cleanup(ts.Close)
	t.Cleanup(transport.CloseIdleConnections)
	return ts, &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

// HTTP/1 和 HTTP/2 的停滞上传都应返回 408；HTTP/1 同时取消上下文时仍优先按读取超时分类。
func TestBodyReadTimeoutClassifiedAsTimeout(t *testing.T) {
	for _, h2c := range []bool{false, true} {
		name := "http1"
		if h2c {
			name = "h2c"
		}
		t.Run(name, func(t *testing.T) {
			ts, client := newBodyReadTimeoutTestServer(t, bodyKindHandler(nil), h2c)
			pr, pw := io.Pipe()
			defer func() { _ = pw.Close() }()
			go func() { _, _ = pw.Write([]byte(`{"model":"x","messages":[`)) }()
			req, err := http.NewRequest(http.MethodPost, ts.URL, pr)
			require.NoError(t, err)
			req.ContentLength = 1 << 20

			start := time.Now()
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equal(t, http.StatusRequestTimeout, resp.StatusCode)
			require.Equal(t, pkghttputil.RequestBodyReadErrorTimeout, resp.Header.Get("X-Body-Error"))
			elapsed := time.Since(start)
			require.GreaterOrEqual(t, elapsed, testBodyReadTimeout)
			require.Less(t, elapsed, 5*time.Second)
		})
	}
}

// 使用真实 net/http 错误和上下文验证上传中途断开仍按客户端取消分类。
func TestBodyReadClassifiesHTTP1ClientDisconnect(t *testing.T) {
	kinds := make(chan string, 1)
	ts, _ := newBodyReadTimeoutTestServer(t, bodyKindHandler(kinds), false)
	conn, err := net.DialTimeout("tcp", ts.Listener.Addr().String(), time.Second)
	require.NoError(t, err)
	_, err = io.WriteString(conn, "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 1000\r\n\r\nabc")
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, conn.Close())

	select {
	case kind := <-kinds:
		require.Equal(t, pkghttputil.RequestBodyReadErrorClientDisconnect, kind)
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not observe the disconnect")
	}
}

// 真实 HTTP/1 连接中，Anthropic 停滞上传必须返回对应协议的 408，不能误判为 499 或通用 400。
func TestHTTPServerBodyReadTimeoutAnthropicMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	groupID := int64(1)
	r.POST("/v1/messages", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 1, UserID: 1, GroupID: &groupID})
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
		c.Next()
	}, (&handler.GatewayHandler{}).Messages)
	cfg := ingressTestConfig()
	cfg.Server.MaxRequestBodySize = 0
	cfg.Gateway.MaxBodySize = 0
	cfg.Gateway.RequestBodyReadTimeoutSeconds = 1
	addr, stop := serveIngressTestServer(t, ProvideHTTPServer(cfg, r))
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	partial := `{"model":"claude-sonnet-4-5","max_tokens":1,"messages":[`
	_, err = io.WriteString(conn, "POST /v1/messages HTTP/1.1\r\nHost: test\r\nContent-Type: application/json\r\nContent-Length: 1048576\r\n\r\n"+partial)
	require.NoError(t, err)

	start := time.Now()
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	require.Equal(t, http.StatusRequestTimeout, resp.StatusCode)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
	require.Less(t, elapsed, 5*time.Second)
	var payload struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "invalid_request_error", payload.Error.Type)
	require.Equal(t, "Timed out reading request body", payload.Error.Message)
	require.True(t, resp.Close, "connection with an unread body must not be reused")
}
