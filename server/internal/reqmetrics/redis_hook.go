package reqmetrics

import (
	"context"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisMetricsHook struct{}

func NewRedisHook() redis.Hook {
	return redisMetricsHook{}
}

func (redisMetricsHook) DialHook(hook redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return hook(ctx, network, addr)
	}
}

func (redisMetricsHook) ProcessHook(hook redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		startAt := time.Now()
		err := hook(ctx, cmd)
		if collector := FromContext(ctx); collector != nil {
			collector.AddRedis(time.Since(startAt))
		}
		return err
	}
}

func (redisMetricsHook) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		startAt := time.Now()
		err := hook(ctx, cmds)
		if collector := FromContext(ctx); collector != nil {
			collector.AddRedis(time.Since(startAt))
		}
		return err
	}
}
