package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 每次返回新的响应体，避免已关闭响应干扰真实的重试次数断言。
type configuredGeminiRetryUpstream struct{ calls int }

func (s *configuredGeminiRetryUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	body := `{"error":{"code":503,"message":"overloaded","status":"UNAVAILABLE"}}`
	return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}
func (s *configuredGeminiRetryUpstream) DoWithTLS(r *http.Request, p string, a int64, c int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(r, p, a, c)
}

func TestConfiguredGeminiSameAccountRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, retries := range []int{0, 1} {
		for _, entry := range []string{"messages", "native", "chat"} {
			t.Run(entry+string(rune('0'+retries)), func(t *testing.T) {
				upstream := &configuredGeminiRetryUpstream{}
				svc := &GeminiMessagesCompatService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxSameAccountRetriesGemini: retries}}, httpUpstream: upstream, tokenProvider: &GeminiTokenProvider{}}
				account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
				var err error
				switch entry {
				case "messages":
					_, err = svc.Forward(context.Background(), c, account, []byte(`{"model":"gemini-3.5-flash","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
				case "native":
					_, err = svc.ForwardNative(context.Background(), c, account, "gemini-3.5-flash", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
				case "chat":
					_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(`{"model":"gemini-3.5-flash","messages":[{"role":"user","content":"hi"}]}`))
				}
				require.Error(t, err)
				require.Equal(t, retries+1, upstream.calls)
			})
		}
	}
}
