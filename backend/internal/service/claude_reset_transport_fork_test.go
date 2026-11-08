package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type resetForkUpstream struct {
	HTTPUpstream
	t     *testing.T
	calls int
}

// 共享传输必须携带真实账号与安全标记，不能在新功能里绕开 fork 现有隔离。
func (s *resetForkUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	s.calls++
	require.Equal(s.t, int64(42), id)
	require.Equal(s.t, 3, concurrency)
	require.Empty(s.t, proxy)
	require.Equal(s.t, "Bearer synthetic-token", req.Header.Get("Authorization"))
	require.True(s.t, HTTPUpstreamRedirectsDisabled(req.Context()))
	require.True(s.t, HTTPUpstreamPublicHostsOnly(req.Context()))
	deadline, ok := req.Context().Deadline()
	require.True(s.t, ok)
	require.LessOrEqual(s.t, time.Until(deadline), 25*time.Second)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"cedar_ember":null}`))}, nil
}

func TestClaudeResetUsesForkAccountTransport(t *testing.T) {
	upstream := &resetForkUpstream{t: t}
	s := &ClaudeResetCreditService{accounts: resetAccountStub{&Account{ID: 42, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 3, Credentials: map[string]any{"scope": "user:profile"}}}, tokens: resetTokenStub{}, now: time.Now}
	s.SetHTTPUpstream(upstream, nil)
	result, err := s.Query(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls)
}
