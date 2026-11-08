package httputil

import (
	"context"
	"errors"
	"sync"
)

// ErrRequestBodyMemoryBudget 表示实例暂时无力缓冲新请求，是容量不足而非正文格式或单请求大小错误。
var ErrRequestBodyMemoryBudget = errors.New("request body memory budget exhausted")

// BodyMemoryBudget 跨请求统计分块、连续副本、解压结果及解码窗口；不代表整个应用堆或全部协议转换的上限。
type BodyMemoryBudget struct {
	mu    sync.Mutex
	limit int64
	used  int64
}

func NewBodyMemoryBudget(limit int64) *BodyMemoryBudget {
	return &BodyMemoryBudget{limit: limit}
}

type bodyMemoryReservation struct {
	mu     sync.Mutex
	budget *BodyMemoryBudget
	bytes  int64
	closed bool
}

type bodyMemoryReservationKey struct{}

// WithRequestBodyMemoryBudget 将最终正文预算保留到 HTTP 处理器退出，包括等待用户并发槽的时间；临时分配及时退还，预算不足不创建等待队列或协程。
func WithRequestBodyMemoryBudget(ctx context.Context, budget *BodyMemoryBudget) (context.Context, func()) {
	if budget == nil || budget.limit <= 0 || ctx.Value(bodyMemoryReservationKey{}) != nil {
		return ctx, func() {}
	}
	r := &bodyMemoryReservation{budget: budget}
	return context.WithValue(ctx, bodyMemoryReservationKey{}, r), r.release
}

// WithRequestBodyMemoryScope 为单次上游改写等短生命周期表示创建独立子预算；释放幂等且只退还自身字节。无父预算时不操作，父请求已经释放时拒绝继续申请。
func WithRequestBodyMemoryScope(ctx context.Context) (context.Context, func()) {
	parent, _ := ctx.Value(bodyMemoryReservationKey{}).(*bodyMemoryReservation)
	if parent == nil {
		return ctx, func() {}
	}
	parent.mu.Lock()
	closed := parent.closed
	parent.mu.Unlock()
	r := &bodyMemoryReservation{budget: parent.budget, closed: closed}
	return context.WithValue(ctx, bodyMemoryReservationKey{}, r), r.release
}

// DetachRequestBodyMemory 为异步任务保留请求派生缓冲区：接管父请求已持有的字节，只为差额申请，不重复计数。调用方必须在任务结束时释放，父处理器退出不影响该预算。
func DetachRequestBodyMemory(ctx context.Context, n int64) (context.Context, func(), error) {
	parent, _ := ctx.Value(bodyMemoryReservationKey{}).(*bodyMemoryReservation)
	if parent == nil {
		return ctx, func() {}, nil
	}
	detached, release := WithRequestBodyMemoryScope(ctx)
	r, _ := detached.Value(bodyMemoryReservationKey{}).(*bodyMemoryReservation)
	parent.mu.Lock()
	moved := int64(0)
	if !parent.closed && !r.closed {
		moved = min(max(n, 0), parent.bytes)
		parent.bytes -= moved
	}
	parent.mu.Unlock()
	r.mu.Lock()
	r.bytes += moved
	r.mu.Unlock()
	if err := reserveRequestBodyMemory(detached, n-moved); err != nil {
		// 接管失败时把正文预算交回仍持有切片的请求处理器。
		r.mu.Lock()
		r.bytes -= moved
		r.mu.Unlock()
		parent.mu.Lock()
		if parent.closed {
			parent.budget.mu.Lock()
			parent.budget.used -= moved
			parent.budget.mu.Unlock()
		} else {
			parent.bytes += moved
		}
		parent.mu.Unlock()
		release()
		return ctx, func() {}, err
	}
	return detached, release, nil
}

func (r *bodyMemoryReservation) release() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.budget.mu.Lock()
	r.budget.used -= r.bytes
	r.budget.mu.Unlock()
	r.bytes = 0
}

// ReserveRequestBodyMemory 在创建新缓冲表示前申请预算，保持到当前作用域或处理器退出。容量不足必须向上传播，不能改走无预算路径；是否继续处理已断开的客户端由调用方决定。
func ReserveRequestBodyMemory(ctx context.Context, n int64) error {
	return reserveBodyMemory(ctx, n)
}

func reserveRequestBodyMemory(ctx context.Context, n int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return reserveBodyMemory(ctx, n)
}

func reserveBodyMemory(ctx context.Context, n int64) error {
	r, _ := ctx.Value(bodyMemoryReservationKey{}).(*bodyMemoryReservation)
	if r == nil || n <= 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRequestBodyMemoryBudget
	}
	r.budget.mu.Lock()
	defer r.budget.mu.Unlock()
	if n > r.budget.limit-r.budget.used {
		return ErrRequestBodyMemoryBudget
	}
	r.budget.used += n
	r.bytes += n
	return nil
}

// ReleaseRequestBodyMemory 退还当前请求或作用域预留但未使用的字节，不能退还其他请求的份额。
func ReleaseRequestBodyMemory(ctx context.Context, n int64) {
	releaseRequestBodyMemory(ctx, n)
}

// releaseRequestBodyMemory 退还已不再持有的分配；每份只释放一次。取消读取可能与清理并发，已关闭的作用域不再重复释放。
func releaseRequestBodyMemory(ctx context.Context, n int64) {
	r, _ := ctx.Value(bodyMemoryReservationKey{}).(*bodyMemoryReservation)
	if r == nil || n <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	n = min(n, r.bytes)
	r.budget.mu.Lock()
	r.budget.used -= n
	r.budget.mu.Unlock()
	r.bytes -= n
}
