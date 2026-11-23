package discovery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisNodeRegistry struct {
	redis    *redis.Client
	key      string
	value    string
	ttl      time.Duration
	interval time.Duration
}

func NewRedisNodeRegistry(client *redis.Client, keyPrefix, nodeAddr string, ttl, interval time.Duration) (*RedisNodeRegistry, error) {
	if client == nil {
		return nil, errors.New("redis client is nil")
	}
	if nodeAddr == "" {
		return nil, errors.New("node advertise address is required")
	}
	if keyPrefix == "" {
		return nil, errors.New("node registry key prefix is required")
	}
	if ttl <= 0 {
		return nil, errors.New("node registry ttl must be positive")
	}
	if interval <= 0 {
		return nil, errors.New("node registry interval must be positive")
	}
	if interval >= ttl {
		return nil, errors.New("node registry interval must be smaller than ttl")
	}
	return &RedisNodeRegistry{
		redis:    client,
		key:      fmt.Sprintf("%s%s", keyPrefix, nodeAddr),
		value:    nodeAddr,
		ttl:      ttl,
		interval: interval,
	}, nil
}

func (r *RedisNodeRegistry) Start(ctx context.Context) error {
	if err := r.heartbeat(ctx); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.heartbeat(context.Background())
			}
		}
	}()
	return nil
}

func (r *RedisNodeRegistry) Stop(ctx context.Context) error {
	return r.redis.Del(ctx, r.key).Err()
}

func (r *RedisNodeRegistry) heartbeat(ctx context.Context) error {
	return r.redis.Set(ctx, r.key, r.value, r.ttl).Err()
}
