package repository

import (
	"context"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/redis/go-redis/v9"
)

type modelMarketplaceAvailabilityCache struct {
	rdb *redis.Client
}

// NewMarketplaceAvailabilityCache 创建模型广场可用性摘要的 Redis 共享缓存。
func NewMarketplaceAvailabilityCache(rdb *redis.Client) service.MarketplaceAvailabilityCache {
	return &modelMarketplaceAvailabilityCache{rdb: rdb}
}

func (c *modelMarketplaceAvailabilityCache) Get(ctx context.Context, key string) (string, error) {
	if c == nil || c.rdb == nil {
		return "", service.ErrMarketplaceAvailabilityCacheMiss
	}
	value, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", service.ErrMarketplaceAvailabilityCacheMiss
	}
	return value, err
}

func (c *modelMarketplaceAvailabilityCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, key, value, ttl).Err()
}
