package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	imageHistoryRetentionInterval = 10 * time.Minute
	imageHistoryRetentionTimeout  = time.Minute
	imageHistoryRetentionBatch    = 100
)

func imageHistoryRetentionCutoff(now time.Time, cfg config.ImageHistoryConfig) time.Time {
	return now.Add(-time.Duration(cfg.EffectiveRetentionDays()) * 24 * time.Hour)
}

// CleanupExpired 对记录和云端图片执行同一期限，用户关闭个人保存开关后仍会清理旧图片。
// @project-doc docs/domains/image_history.md#retention
func (s *ImageHistoryService) CleanupExpired(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil {
		return nil
	}
	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()
	store, cfg := s.storageSnapshot()
	if store == nil || !cfg.Enabled {
		return nil
	}
	cutoff := imageHistoryRetentionCutoff(now, cfg)
	var firstErr error
	afterID := s.retentionAfterID
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		records, err := s.repo.ListExpired(ctx, cutoff, afterID, imageHistoryRetentionBatch)
		if err != nil {
			return fmt.Errorf("list expired image history: %w", err)
		}
		for _, record := range records {
			if err := ctx.Err(); err != nil {
				return err
			}
			afterID = record.ID
			s.retentionAfterID = record.ID
			// 先删除对象，再删除元数据；对象故障时保留重试依据。
			deleteCtx, cancel := context.WithTimeout(ctx, imageHistoryCleanupTimeout)
			err := store.Delete(deleteCtx, record.ObjectKey)
			if err == nil {
				err = s.repo.Delete(deleteCtx, record.UserID, record.ID)
			}
			cancel()
			// 多实例或用户手动删除可能先完成，同一记录重复清理视为成功。
			if err != nil && !errors.Is(err, ErrImageHistoryNotFound) && firstErr == nil {
				firstErr = fmt.Errorf("delete expired image history %s: %w", record.ID, err)
			}
		}
		if len(records) < imageHistoryRetentionBatch {
			if err := ctx.Err(); err != nil {
				return err
			}
			// 一轮扫描结束后回到起点，使失败记录在后续周期重试。
			s.retentionAfterID = ""
			return firstErr
		}
	}
}

// runRetentionCleanup 复用转存 worker 的生命周期，启动时执行一次，之后定时清理。
func (p *ImageHistorySaveWorkerPool) runRetentionCleanup() {
	defer p.wg.Done()
	ticker := time.NewTicker(imageHistoryRetentionInterval)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(p.ctx, imageHistoryRetentionTimeout)
		err := p.service.CleanupExpired(ctx, time.Now())
		cancel()
		if err != nil && p.ctx.Err() == nil {
			logger.L().With(zap.String("component", "service.image_history_worker")).Warn("image_history.cleanup_failed", zap.Error(err))
		}
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
