//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// openAITransportAccountRepoStub 只记录临时不可调度调用，其他仓储方法不应被触达。
type openAITransportAccountRepoStub struct {
	AccountRepository
	tempUnschedCalls []tempUnschedCall
}

func (r *openAITransportAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempUnschedCalls = append(r.tempUnschedCalls, tempUnschedCall{accountID: id, until: until, reason: reason})
	return nil
}

func newOpenAITransportErrTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, rec
}

func TestHandleOpenAIUpstreamTransportError_PersistentEvictsAndFailsOver(t *testing.T) {
	repo := &openAITransportAccountRepoStub{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 4627, Name: "proxy-expired", Platform: PlatformOpenAI}
	c, rec := newOpenAITransportErrTestContext()

	before := time.Now()
	retErr := svc.handleOpenAIUpstreamTransportError(context.Background(), c, account,
		errors.New(`Post "https://chatgpt.com/backend-api/codex/responses": socks connect tcp 85.255.176.68:12324->chatgpt.com:443: username/password authentication failed`), false)
	after := time.Now()

	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(retErr, &failoverErr), "persistent error must return *UpstreamFailoverError")
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Len(t, repo.tempUnschedCalls, 1)
	require.Equal(t, int64(4627), repo.tempUnschedCalls[0].accountID)
	require.Contains(t, repo.tempUnschedCalls[0].reason, "authentication failed")
	require.True(t, repo.tempUnschedCalls[0].until.After(before.Add(openAITransportErrorTempUnschedDuration-time.Second)))
	require.True(t, repo.tempUnschedCalls[0].until.Before(after.Add(openAITransportErrorTempUnschedDuration+time.Second)))
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 0, rec.Body.Len())
}

func TestHandleOpenAIUpstreamTransportError_TransientFailsOverWithoutEviction(t *testing.T) {
	repo := &openAITransportAccountRepoStub{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 99, Name: "flaky", Platform: PlatformOpenAI}
	c, rec := newOpenAITransportErrTestContext()

	err := svc.handleOpenAIUpstreamTransportError(context.Background(), c, account,
		errors.New(`Post "https://chatgpt.com/...": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`), false)

	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "transient error must return *UpstreamFailoverError")
	require.Empty(t, repo.tempUnschedCalls)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 0, rec.Body.Len())
}

func TestHandleOpenAIUpstreamTransportError_ContextCanceledNoFailover(t *testing.T) {
	repo := &openAITransportAccountRepoStub{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 77, Name: "healthy", Platform: PlatformOpenAI}
	c, rec := newOpenAITransportErrTestContext()

	err := svc.handleOpenAIUpstreamTransportError(context.Background(), c, account, context.Canceled, false)

	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "context.Canceled must not return *UpstreamFailoverError")
	require.Empty(t, repo.tempUnschedCalls)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 0, rec.Body.Len())
}

func TestHandleOpenAIUpstreamTransportError_WrappedContextCanceledNoFailover(t *testing.T) {
	repo := &openAITransportAccountRepoStub{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 78, Name: "healthy2", Platform: PlatformOpenAI}
	c, _ := newOpenAITransportErrTestContext()

	err := svc.handleOpenAIUpstreamTransportError(context.Background(), c, account, fmt.Errorf("http request failed: %w", context.Canceled), false)

	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "wrapped context.Canceled must not return *UpstreamFailoverError")
	require.Empty(t, repo.tempUnschedCalls)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

// TestHandleOpenAIUpstreamTransportError_RecordsOllamaActivityOnly 验证传输错误只记录 Ollama Cloud 账号活动。
func TestHandleOpenAIUpstreamTransportError_RecordsOllamaActivityOnly(t *testing.T) {
	deferred := NewDeferredService(nil, nil, time.Second)
	svc := &OpenAIGatewayService{
		accountRepo:     &openAITransportAccountRepoStub{},
		deferredService: deferred,
	}
	ollama := &Account{
		ID: 501, Name: "ollama-cloud", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "k-ollama", "base_url": "https://ollama.com"},
	}
	other := &Account{
		ID: 502, Name: "openai-official", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "k-openai", "base_url": "https://api.openai.com"},
	}
	c, _ := newOpenAITransportErrTestContext()

	_ = svc.handleOpenAIUpstreamTransportError(context.Background(), c, ollama, errors.New("connection reset"), false)
	_ = svc.handleOpenAIUpstreamTransportError(context.Background(), c, other, errors.New("connection reset"), false)

	_, ok := deferred.lastUsedUpdates.Load(int64(501))
	require.True(t, ok, "Ollama Cloud transport error must schedule last_used activity")
	_, ok = deferred.lastUsedUpdates.Load(int64(502))
	require.False(t, ok, "non-Ollama transport error must not schedule Ollama activity")
}

// TestHandleOpenAIUpstreamTransportError_ContextCanceledSkipsOllamaActivity 验证客户端取消不会记录活动。
func TestHandleOpenAIUpstreamTransportError_ContextCanceledSkipsOllamaActivity(t *testing.T) {
	deferred := NewDeferredService(nil, nil, time.Second)
	svc := &OpenAIGatewayService{
		accountRepo:     &openAITransportAccountRepoStub{},
		deferredService: deferred,
	}
	ollama := &Account{
		ID: 503, Name: "ollama-canceled", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "k-ollama", "base_url": "https://ollama.com"},
	}
	c, _ := newOpenAITransportErrTestContext()

	err := svc.handleOpenAIUpstreamTransportError(context.Background(), c, ollama, context.Canceled, false)

	require.ErrorIs(t, err, context.Canceled)
	_, ok := deferred.lastUsedUpdates.Load(int64(503))
	require.False(t, ok, "context.Canceled is client disconnect before a fault; do not count as Ollama activity")
}

// TestHandleOpenAIAccountUpstreamError_RecordsOllamaActivityOnly 验证非 2xx 响应只记录 Ollama Cloud 账号活动。
func TestHandleOpenAIAccountUpstreamError_RecordsOllamaActivityOnly(t *testing.T) {
	deferred := NewDeferredService(nil, nil, time.Second)
	svc := &OpenAIGatewayService{deferredService: deferred}
	ollama := &Account{
		ID: 504, Name: "ollama-429", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "k-ollama", "base_url": "https://ollama.com"},
	}
	other := &Account{
		ID: 505, Name: "openai-429", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "k-openai", "base_url": "https://api.openai.com"},
	}

	_ = svc.handleOpenAIAccountUpstreamError(context.Background(), ollama, http.StatusTooManyRequests, http.Header{}, []byte(`{"error":{"message":"rate"}}`), "gpt-test")
	_ = svc.handleOpenAIAccountUpstreamError(context.Background(), other, http.StatusTooManyRequests, http.Header{}, []byte(`{"error":{"message":"rate"}}`), "gpt-test")

	_, ok := deferred.lastUsedUpdates.Load(int64(504))
	require.True(t, ok, "Ollama Cloud non-2xx must schedule last_used activity")
	_, ok = deferred.lastUsedUpdates.Load(int64(505))
	require.False(t, ok, "non-Ollama non-2xx must not schedule Ollama activity")
}
