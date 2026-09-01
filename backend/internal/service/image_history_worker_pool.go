package service

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultImageHistoryWorkerCount = 4
	defaultImageHistoryQueueSize   = 256
	defaultImageHistorySaveTimeout = 45 * time.Second
	imageHistoryDropLogInterval    = 5 * time.Second
)

// ImageHistorySaveWorkerPool 使用有界队列异步转存图片，避免网关请求创建无界 goroutine。
type ImageHistorySaveWorkerPool struct {
	mu               sync.RWMutex
	service          *ImageHistoryService
	queue            chan SaveImageHistoryInput
	timeout          time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	accepting        bool
	stopped          bool
	lastDropLogNanos atomic.Int64
}

// NewImageHistorySaveWorkerPool 按静态配置创建并启动生图历史转存 worker。
func NewImageHistorySaveWorkerPool(imageHistoryService *ImageHistoryService, cfg *config.Config) *ImageHistorySaveWorkerPool {
	workerCount := defaultImageHistoryWorkerCount
	queueSize := defaultImageHistoryQueueSize
	timeout := defaultImageHistorySaveTimeout
	if cfg != nil {
		if cfg.ImageHistory.WorkerCount > 0 {
			workerCount = cfg.ImageHistory.WorkerCount
		}
		if cfg.ImageHistory.QueueSize > 0 {
			queueSize = cfg.ImageHistory.QueueSize
		}
		if cfg.ImageHistory.SaveTimeoutSeconds > 0 {
			timeout = time.Duration(cfg.ImageHistory.SaveTimeoutSeconds) * time.Second
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool := &ImageHistorySaveWorkerPool{
		service: imageHistoryService,
		queue:   make(chan SaveImageHistoryInput, queueSize),
		timeout: timeout,
		ctx:     ctx,
		cancel:  cancel,
	}
	if imageHistoryService == nil {
		return pool
	}
	// worker 始终启动，使管理员从页面启用存储后无需重启进程。
	pool.accepting = true
	pool.wg.Add(workerCount)
	for workerID := 0; workerID < workerCount; workerID++ {
		go pool.runWorker(workerID)
	}
	return pool
}

// Submit 非阻塞提交转存任务；队列满或功能不可用时丢弃，不改变生图响应。
func (p *ImageHistorySaveWorkerPool) Submit(input SaveImageHistoryInput) bool {
	if p == nil || p.service == nil || !p.service.Available() || len(input.Images) == 0 {
		return false
	}
	p.mu.RLock()
	if !p.accepting || p.stopped {
		p.mu.RUnlock()
		return false
	}
	select {
	case p.queue <- input:
		p.mu.RUnlock()
		return true
	default:
		p.mu.RUnlock()
		p.logDrop(input)
		return false
	}
}

// Stop 停止接收新任务并取消正在执行的转存。
func (p *ImageHistorySaveWorkerPool) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	p.accepting = false
	p.cancel()
	close(p.queue)
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *ImageHistorySaveWorkerPool) runWorker(workerID int) {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		select {
		case <-p.ctx.Done():
			return
		case input, ok := <-p.queue:
			if !ok {
				return
			}
			p.execute(workerID, input)
		}
	}
}

func (p *ImageHistorySaveWorkerPool) execute(workerID int, input SaveImageHistoryInput) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "service.image_history_worker"),
				zap.Int("worker_id", workerID),
				zap.Any("panic", recovered),
			).Error("image_history.save_panic_recovered")
		}
	}()
	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()
	if err := p.service.SaveCapturedImages(ctx, input); err != nil {
		logger.L().With(
			zap.String("component", "service.image_history_worker"),
			zap.Int("worker_id", workerID),
			zap.Int64("user_id", input.UserID),
			zap.Int64("api_key_id", input.APIKeyID),
			zap.String("request_id", strings.TrimSpace(input.RequestID)),
		).Warn("image_history.save_failed", zap.Error(err))
	}
}

func (p *ImageHistorySaveWorkerPool) logDrop(input SaveImageHistoryInput) {
	now := time.Now().UnixNano()
	previous := p.lastDropLogNanos.Load()
	if previous != 0 && time.Duration(now-previous) < imageHistoryDropLogInterval {
		return
	}
	if !p.lastDropLogNanos.CompareAndSwap(previous, now) {
		return
	}
	logger.L().With(
		zap.String("component", "service.image_history_worker"),
		zap.Int64("user_id", input.UserID),
		zap.Int64("api_key_id", input.APIKeyID),
		zap.String("request_id", strings.TrimSpace(input.RequestID)),
	).Warn("image_history.save_dropped_queue_full")
}
