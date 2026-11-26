package platform

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func openCache(address string) *Cache {
	if address == "" {
		return nil
	}
	return &Cache{client: redis.NewClient(&redis.Options{
		Addr:                  address,
		DialTimeout:           150 * time.Millisecond,
		ReadTimeout:           150 * time.Millisecond,
		WriteTimeout:          150 * time.Millisecond,
		MaxRetries:            -1,
		PoolSize:              8,
		ContextTimeoutEnabled: true,
	})}
}

func (c *Cache) Get(ctx context.Context, key string, extendTTL time.Duration) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	var command *redis.StringCmd
	if extendTTL > 0 {
		command = c.client.GetEx(ctx, key, extendTTL)
	} else {
		command = c.client.Get(ctx, key)
	}
	value, err := command.Bytes()
	return value, err == nil
}

func (c *Cache) Put(ctx context.Context, key string, value []byte, ttl time.Duration) {
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_ = c.client.Set(ctx, key, value, ttl).Err()
}
