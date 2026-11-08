package handler

import (
	"context"
	"fmt"
	"github.com/BrandonVee/TokenRouter/internal/config"
	middleware "github.com/BrandonVee/TokenRouter/internal/server/middleware"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
)

func wsAllowlistGroup(enabled bool, models ...string) *service.Group {
	return &service.Group{
		ID:       4201,
		Platform: service.PlatformOpenAI,
		ModelAllowlist: service.GroupModelAllowlist{
			Enabled: enabled,
			Models:  models,
		},
	}
}

// 首帧拒绝：模型不在白名单时连接被 1008 关闭（passthrough relay 模式）。
func TestOpenAIResponsesWebSocket_FirstFrameModelNotAllowedCloses_Passthrough(t *testing.T) {
	runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:            `{"type":"response.create","model":"gpt-4.1","stream":false}`,
		group:                   wsAllowlistGroup(true, "gpt-5.4"),
		ingressMode:             service.OpenAIWSIngressModePassthrough,
		firstFrameCloseExpected: true,
	})
}

// 首帧拒绝：原生 ingress 模式。
func TestOpenAIResponsesWebSocket_FirstFrameModelNotAllowedCloses_NativeIngress(t *testing.T) {
	runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:            `{"type":"response.create","model":"gpt-4.1","stream":false}`,
		group:                   wsAllowlistGroup(true, "gpt-5.4"),
		ingressMode:             service.OpenAIWSIngressModeDedicated,
		firstFrameCloseExpected: true,
	})
}

// 首帧命中白名单：连接正常建立并收到 response.completed（passthrough）。
func TestOpenAIResponsesWebSocket_FirstFrameAllowlistedModelProceeds(t *testing.T) {
	got := runAllowlistWSCase(t, allowlistWSCase{
		firstPayload: `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		group:        wsAllowlistGroup(true, "gpt-5.4"),
	})
	if len(got.clientEvents) != 1 {
		t.Fatalf("expected one completed event, got %d", len(got.clientEvents))
	}
}

// 后续 turn 切换到白名单外的模型：连接被 1008 关闭（passthrough relay 模式）。
func TestOpenAIResponsesWebSocket_SubsequentTurnModelNotAllowedCloses_Passthrough(t *testing.T) {
	runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		secondPayload:           `{"type":"response.create","model":"gpt-4.1","stream":false}`,
		group:                   wsAllowlistGroup(true, "gpt-5.4"),
		ingressMode:             service.OpenAIWSIngressModePassthrough,
		secondTurnCloseExpected: true,
	})
}

// 后续 turn 切换到白名单外的模型：连接被 1008 关闭（原生 ingress 模式）。
func TestOpenAIResponsesWebSocket_SubsequentTurnModelNotAllowedCloses_NativeIngress(t *testing.T) {
	runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		secondPayload:           `{"type":"response.create","model":"gpt-4.1","stream":false}`,
		group:                   wsAllowlistGroup(true, "gpt-5.4"),
		ingressMode:             service.OpenAIWSIngressModeDedicated,
		secondTurnCloseExpected: true,
	})
}

// 后续 turn 省略 model：沿用会话初始模型，白名单校验通过。
func TestOpenAIResponsesWebSocket_SubsequentTurnOmittedModelUsesSessionModel(t *testing.T) {
	got := runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:  `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		secondPayload: `{"type":"response.create","stream":false}`,
		group:         wsAllowlistGroup(true, "gpt-5.4"),
	})
	if len(got.clientEvents) != 2 {
		t.Fatalf("expected two completed events, got %d", len(got.clientEvents))
	}
	if len(got.upstreamPayloads) != 2 {
		t.Fatalf("expected two upstream frames, got %d", len(got.upstreamPayloads))
	}
}

// 白名单关闭：不受任何影响。
func TestOpenAIResponsesWebSocket_DisabledAllowlistDoesNotInterfere(t *testing.T) {
	got := runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:  `{"type":"response.create","model":"gpt-4.1","stream":false}`,
		secondPayload: `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		group:         wsAllowlistGroup(false, "gpt-5.4"),
	})
	if len(got.clientEvents) != 2 {
		t.Fatalf("expected two completed events, got %d", len(got.clientEvents))
	}
}

// 首帧含重复 model 键（首个在白名单、末个不在）：passthrough 会原样转发两个字段，
// 上游按末值绑定时得到禁用模型，必须在入口拒绝。
func TestOpenAIResponsesWebSocket_FirstFrameDuplicateModelKeysRejected(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			runAllowlistWSCase(t, allowlistWSCase{
				firstPayload:            `{"type":"response.create","model":"gpt-5.4","model":"gpt-4.1","stream":false}`,
				group:                   wsAllowlistGroup(true, "gpt-5.4"),
				ingressMode:             mode,
				firstFrameCloseExpected: true,
			})
		})
	}
}

// 首帧混用大小写变体键（{"model":"allowed","Model":"blocked"}）：绑定系上游
// 大小写不敏感且末值生效会绑定 blocked，必须在入口拒绝。仅含变体键的帧则会在
// 「model is required」处更早被关闭。
func TestOpenAIResponsesWebSocket_FirstFrameCaseVariantModelKeyRejected(t *testing.T) {
	runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","Model":"gpt-4.1","stream":false}`,
		group:                   wsAllowlistGroup(true, "gpt-5.4"),
		ingressMode:             service.OpenAIWSIngressModePassthrough,
		firstFrameCloseExpected: true,
	})
}

// 后续 turn 含重复 model 键（首个在白名单、末个不在）：拒绝并关闭连接。
func TestOpenAIResponsesWebSocket_SubsequentTurnDuplicateModelKeysRejected(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			runAllowlistWSCase(t, allowlistWSCase{
				firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
				secondPayload:           `{"type":"response.create","model":"gpt-5.4","model":"gpt-4.1","stream":false}`,
				group:                   wsAllowlistGroup(true, "gpt-5.4"),
				ingressMode:             mode,
				secondTurnCloseExpected: true,
			})
		})
	}
}

// 重复但同值的 model 键不误伤，连接正常完成两个 turn。
func TestOpenAIResponsesWebSocket_DuplicateIdenticalModelKeysAllowed(t *testing.T) {
	got := runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:  `{"type":"response.create","model":"gpt-5.4","model":"gpt-5.4","stream":false}`,
		secondPayload: `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		group:         wsAllowlistGroup(true, "gpt-5.4"),
	})
	if len(got.clientEvents) != 2 {
		t.Fatalf("expected two completed events, got %d", len(got.clientEvents))
	}
}

// session.update 轮换绕过：首帧用白名单内模型建立会话，session.update 把会话
// 模型改为白名单外的模型，随后 response.create 携带嵌套 session.model（白名单
// 内）让帧内候选非空。实际生效模型（轮换后的会话模型）必须始终参与校验。
func TestOpenAIResponsesWebSocket_SessionUpdateRotationBypassRejected(t *testing.T) {
	runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		midPayload:              `{"type":"session.update","session":{"model":"gpt-4.1"}}`,
		secondPayload:           `{"type":"response.create","session":{"model":"gpt-5.4"},"stream":false}`,
		group:                   wsAllowlistGroup(true, "gpt-5.4"),
		ingressMode:             service.OpenAIWSIngressModePassthrough,
		secondTurnCloseExpected: true,
	})
}

// 轮换后的会话模型本身在白名单内时，省略 model 的后续 turn 正常放行。
func TestOpenAIResponsesWebSocket_SessionUpdateToAllowedModelStillWorks(t *testing.T) {
	got := runAllowlistWSCase(t, allowlistWSCase{
		firstPayload:  `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		midPayload:    `{"type":"session.update","session":{"model":"gpt-5.4"}}`,
		secondPayload: `{"type":"response.create","stream":false}`,
		group:         wsAllowlistGroup(true, "gpt-5.4"),
	})
	if len(got.clientEvents) < 2 {
		t.Fatalf("expected at least two completed events, got %d", len(got.clientEvents))
	}
}

// allowlistWSCase 用实际上下游 WebSocket 验证帧准入，不修改原有单轮用量测试夹具。
type allowlistWSCase struct {
	firstPayload, secondPayload, midPayload, ingressMode string
	group                                                *service.Group
	firstFrameCloseExpected, secondTurnCloseExpected     bool
	userAgent                                            *string
	channelMapping                                       map[string]string
}
type allowlistWSResult struct{ upstreamPayloads, clientEvents [][]byte }

func runAllowlistWSCase(t *testing.T, tc allowlistWSCase) allowlistWSResult {
	t.Helper()
	gin.SetMode(gin.TestMode)

	turnCount := 1
	if tc.midPayload != "" {
		turnCount++
	}
	if tc.secondPayload != "" {
		turnCount++
	}
	upstreamPayloadCh := make(chan []byte, turnCount)
	upstreamErrCh := make(chan error, 1)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{
			CompressionMode: coderws.CompressionContextTakeover,
		})
		if err != nil {
			upstreamErrCh <- err
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		for turn := 1; turn <= turnCount; turn++ {
			readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			_, payload, err := conn.Read(readCtx)
			cancel()
			if err != nil {
				upstreamErrCh <- err
				return
			}
			upstreamPayloadCh <- payload
			response := fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_allowlist_%d","model":%q,"usage":{"input_tokens":2,"output_tokens":1}}}`, turn, gjson.GetBytes(payload, "model").String())
			writeCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			err = conn.Write(writeCtx, coderws.MessageText, []byte(response))
			cancel()
			if err != nil {
				upstreamErrCh <- err
				return
			}
		}

		_ = conn.Close(coderws.StatusNormalClosure, "done")
		upstreamErrCh <- nil
	}))
	defer upstreamServer.Close()

	groupID := int64(4201)
	account := service.Account{
		ID:          9901,
		Name:        "openai-ws-passthrough-usage-e2e",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": upstreamServer.URL,
		},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
		},
	}

	if tc.ingressMode != "" {
		account.Extra["openai_apikey_responses_websockets_v2_mode"] = tc.ingressMode
	}

	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	accountRepo := &openAIWSUsageHandlerAccountRepoStub{account: account}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, turnCount)}

	var channelSvc *service.ChannelService
	if len(tc.channelMapping) > 0 {
		channelSvc = service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{
			channels: []service.Channel{{
				ID:           7701,
				Name:         "openai-ws-e2e-channel",
				Status:       service.StatusActive,
				GroupIDs:     []int64{groupID},
				ModelMapping: map[string]map[string]string{service.PlatformOpenAI: tc.channelMapping},
			}},
			groupPlatforms: map[int64]string{groupID: service.PlatformOpenAI},
		}, nil)
	}

	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheSvc.Stop)
	gatewaySvc := service.NewOpenAIGatewayService(
		accountRepo,
		usageRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		nil,
		service.NewBillingService(cfg, nil),
		nil,
		billingCacheSvc,
		nil,
		nil,
		&service.DeferredService{},
		nil,
		nil,
		nil,
		channelSvc,
		nil,
		nil,
		nil, // 用户平台配额仓库
		nil, // 数据共享服务
	)

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
	}
	h := &OpenAIGatewayHandler{
		cfg:                 cfg,
		gatewayService:      gatewaySvc,
		billingCacheService: billingCacheSvc,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}

	apiKey := &service.APIKey{
		ID:      1801,
		GroupID: &groupID,
		User:    &service.User{ID: 1701, Status: service.StatusActive},
	}
	apiKey.Group = tc.group

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.GET("/openai/v1/responses", h.ResponsesWebSocket)
	handlerServer := httptest.NewServer(router)
	defer handlerServer.Close()

	headers := http.Header{}
	if tc.userAgent != nil {
		headers.Set("User-Agent", *tc.userAgent)
	}
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(
		dialCtx,
		"ws"+strings.TrimPrefix(handlerServer.URL, "http")+"/openai/v1/responses",
		&coderws.DialOptions{HTTPHeader: headers, CompressionMode: coderws.CompressionContextTakeover},
	)
	cancelDial()
	require.NoError(t, err)
	defer func() {
		_ = clientConn.CloseNow()
	}()

	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	err = clientConn.Write(writeCtx, coderws.MessageText, []byte(tc.firstPayload))
	cancelWrite()
	require.NoError(t, err)

	events := make([][]byte, 0, turnCount)
	readEvent := func(expectClose bool) {
		readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, event, readErr := clientConn.Read(readCtx)
		cancel()
		if expectClose {
			var closeErr coderws.CloseError
			require.ErrorAs(t, readErr, &closeErr)
			require.Equal(t, coderws.StatusPolicyViolation, closeErr.Code)
			require.Contains(t, closeErr.Reason, "not available for this group")
			return
		}
		require.NoError(t, readErr)
		require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
		events = append(events, append([]byte(nil), event...))
	}
	if tc.firstFrameCloseExpected {
		readEvent(true)
		require.Empty(t, upstreamPayloadCh)
		return allowlistWSResult{}
	}
	readEvent(false)
	send := func(payload string) {
		writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := clientConn.Write(writeCtx, coderws.MessageText, []byte(payload))
		cancel()
		require.NoError(t, err)
	}
	if tc.midPayload != "" {
		send(tc.midPayload)
		readEvent(false)
	}
	if tc.secondPayload != "" {
		send(tc.secondPayload)
		readEvent(tc.secondTurnCloseExpected)
	}
	_ = clientConn.CloseNow()
	count := turnCount
	if tc.secondTurnCloseExpected {
		count--
	}
	payloads := make([][]byte, 0, count)
	for len(payloads) < count {
		select {
		case payload := <-upstreamPayloadCh:
			payloads = append(payloads, payload)
		case <-time.After(3 * time.Second):
			t.Fatal("等待上游白名单测试帧超时")
		}
	}
	require.Empty(t, upstreamPayloadCh, "拒绝的帧不能抵达上游")
	return allowlistWSResult{upstreamPayloads: payloads, clientEvents: events}
}
