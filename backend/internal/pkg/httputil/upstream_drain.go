package httputil

import (
	"context"
	"sync"
	"time"
)

const DefaultUpstreamDrainTimeout = 180 * time.Second

type upstreamDrainKey struct{}
type upstreamDrainPolicy struct {
	client  context.Context
	timeout time.Duration
}

// DetachUpstreamContext 在客户端离开后保留计费读取，仅记录原取消信号，不在构造阶段启动计时；实际传输负责单次请求和有界排空的生命周期。
func DetachUpstreamContext(ctx context.Context, timeout ...time.Duration) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	policy := upstreamDrainPolicy{client: ctx}
	if previous, ok := ctx.Value(upstreamDrainKey{}).(upstreamDrainPolicy); ok {
		policy = previous
	}
	if len(timeout) > 0 && timeout[0] > 0 {
		policy.timeout = timeout[0]
	}
	return context.WithValue(context.WithoutCancel(ctx), upstreamDrainKey{}, policy)
}

// NewUpstreamAttemptContext 在原客户端取消时启动总排空预算，后续字节不续期；此前请求可长期流式执行。错误或响应体关闭时必须释放，确保末尾用量仍有机会读取。
func NewUpstreamAttemptContext(parent context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	policy, ok := parent.Value(upstreamDrainKey{}).(upstreamDrainPolicy)
	if !ok || policy.client.Done() == nil {
		return ctx, cancel
	}
	timeout := policy.timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout <= 0 {
		timeout = DefaultUpstreamDrainTimeout
	}
	var mu sync.Mutex
	var timer *time.Timer
	closed := false
	startDrain := func() {
		mu.Lock()
		defer mu.Unlock()
		if !closed {
			timer = time.AfterFunc(timeout, cancel)
		}
	}
	stop := context.AfterFunc(policy.client, startDrain)
	return ctx, func() {
		stop()
		mu.Lock()
		closed = true
		if timer != nil {
			timer.Stop()
		}
		mu.Unlock()
		cancel()
	}
}
