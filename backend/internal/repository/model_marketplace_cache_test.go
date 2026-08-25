package repository

import (
	"context"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceAvailabilityCacheSharesValuesAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cache := NewMarketplaceAvailabilityCache(rdb)
	ctx := context.Background()
	key := "sub2api:model_marketplace:availability:test"

	require.NoError(t, cache.Set(ctx, key, `{"7":{"availability_rate":1}}`, time.Minute))
	value, err := cache.Get(ctx, key)
	require.NoError(t, err)
	require.JSONEq(t, `{"7":{"availability_rate":1}}`, value)

	mr.FastForward(2 * time.Minute)
	_, err = cache.Get(ctx, key)
	require.ErrorIs(t, err, service.ErrMarketplaceAvailabilityCacheMiss)
}
