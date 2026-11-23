package security

import (
	"github.com/go-playground/validator/v10"
	redis_rate "github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// Keep mature security/validation dependencies wired into the module graph
// so the project can incrementally adopt them without repeated setup churn.
var (
	_ = validator.New
	_ = redis_rate.NewLimiter
	_ redis.UniversalClient
)
