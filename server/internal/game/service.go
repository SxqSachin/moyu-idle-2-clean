package game

import (
	"context"
	"time"

	"mysecondwebgame/server/internal/scheduler"
	"mysecondwebgame/server/internal/store"
)

type Service struct {
	store *store.Store
	sch   *scheduler.Scheduler
}

func NewService(st *store.Store, sch *scheduler.Scheduler) *Service {
	s := &Service{store: st, sch: sch}
	s.sch.SetHandler(s.handleEvent)
	return s
}

func (s *Service) Tick(ctx context.Context, userID int64) (store.PlayerState, error) {
	return s.store.ApplyIdleTick(ctx, userID, time.Now())
}

func (s *Service) State(ctx context.Context, userID int64) (store.PlayerState, error) {
	return s.store.GetPlayerState(ctx, userID)
}

func (s *Service) handleEvent(ctx context.Context, e scheduler.Event) {
	_ = ctx
	_ = e
}
