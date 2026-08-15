package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newCodexTurnStateTestContext(t *testing.T, apiKeyID int64, sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session_id", sessionID)
	c.Set("api_key", &APIKey{ID: apiKeyID})
	return c, recorder
}

func TestOpenAICodexTurnStateRelayAndCrossAccountGuard(t *testing.T) {
	svc := &OpenAIGatewayService{}
	c, _ := newCodexTurnStateTestContext(t, 7, "session-a")
	upstream := http.Header{"X-Codex-Turn-State": []string{"state-a"}}

	svc.relayOpenAICodexTurnState(c, &Account{ID: 42}, upstream)
	require.Equal(t, "state-a", c.Writer.Header().Get(openAICodexTurnStateHeader))

	sameAccount := http.Header{"X-Codex-Turn-State": []string{"state-a"}}
	svc.guardOpenAICodexTurnStateEcho(c, &Account{ID: 42}, sameAccount)
	require.Equal(t, "state-a", sameAccount.Get(openAICodexTurnStateHeader))

	otherAccount := http.Header{"X-Codex-Turn-State": []string{"state-a"}}
	svc.guardOpenAICodexTurnStateEcho(c, &Account{ID: 43}, otherAccount)
	require.Empty(t, otherAccount.Get(openAICodexTurnStateHeader))
}

func TestOpenAICodexTurnStateExpiredOriginDoesNotStrip(t *testing.T) {
	svc := &OpenAIGatewayService{}
	c, _ := newCodexTurnStateTestContext(t, 8, "session-b")
	seed := openAICodexTurnStateSeed(c)
	svc.openaiCodexTurnStateOrigins.Store(seed, openAICodexTurnStateOrigin{accountID: 42, expiresAt: time.Now().Add(-time.Minute)})

	headers := http.Header{"X-Codex-Turn-State": []string{"state-old"}}
	svc.guardOpenAICodexTurnStateEcho(c, &Account{ID: 43}, headers)
	require.Equal(t, "state-old", headers.Get(openAICodexTurnStateHeader))
	_, exists := svc.openaiCodexTurnStateOrigins.Load(seed)
	require.False(t, exists)
}

func TestWriteOpenAIPassthroughResponseHeadersRelaysAndClearsTurnState(t *testing.T) {
	destination := http.Header{}
	writeOpenAIPassthroughResponseHeaders(destination, http.Header{"X-Codex-Turn-State": []string{"state-a"}}, nil)
	require.Equal(t, "state-a", destination.Get(openAICodexTurnStateHeader))

	writeOpenAIPassthroughResponseHeaders(destination, http.Header{"Content-Type": []string{"application/json"}}, nil)
	require.Empty(t, destination.Get(openAICodexTurnStateHeader))
}
