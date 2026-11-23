package app

import (
	"context"

	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/auth"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/game"
	"mysecondwebgame/server/internal/market"
	"mysecondwebgame/server/internal/moderation"
	"mysecondwebgame/server/internal/mq"
	"mysecondwebgame/server/internal/risk"
	"mysecondwebgame/server/internal/riskinsight"
	"mysecondwebgame/server/internal/scheduler"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/ws"
)

type AuthGate interface {
	Health(ctx context.Context) error
	PoolStats() (acquired, idle, maxConns int32)
	GetAuthGateState(ctx context.Context, userID int64) (store.AuthGateState, error)
}

type EmailPolicy interface {
	IsBlocked(email string) bool
}

type Dependencies struct {
	Config      config.Config
	GameData    catalog.Data
	Store       *store.Store
	AuthGate    AuthGate
	Redis       *redis.Client
	Auth        *auth.Service
	EmailPolicy EmailPolicy
	Moderation  *moderation.Service
	MQ          mq.Publisher
	Game        *game.Service
	Market      *market.Service
	Risk        *risk.Service
	RiskInsight *riskinsight.Service
	Hub         *ws.Hub
	Scheduler   *scheduler.Scheduler
}
