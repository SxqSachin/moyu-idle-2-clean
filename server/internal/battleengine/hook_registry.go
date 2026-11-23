package battleengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"mysecondwebgame/server/internal/wsbus"
)

var ErrHookNotRegistered = errors.New("battle hook not registered")

type HookContext struct {
	RunID      int64
	RoomID     int64
	Seed       int64
	HookID     string
	TriggerSeq int64
	Input      map[string]any
}

type HookResult struct {
	TimelineEvents []wsbus.BattleTimelineEvent
}

type HookHandler func(ctx context.Context, input HookContext) (HookResult, error)

type HookRegistry struct {
	mu       sync.RWMutex
	handlers map[string]HookHandler
}

func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		handlers: make(map[string]HookHandler),
	}
}

func (r *HookRegistry) Register(id string, handler HookHandler) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("hook id is required")
	}
	if handler == nil {
		return fmt.Errorf("hook handler is required")
	}
	r.mu.Lock()
	r.handlers[id] = handler
	r.mu.Unlock()
	return nil
}

func (r *HookRegistry) Execute(ctx context.Context, input HookContext) (HookResult, error) {
	id := strings.TrimSpace(input.HookID)
	if id == "" {
		return HookResult{}, fmt.Errorf("hook id is required")
	}
	r.mu.RLock()
	handler, ok := r.handlers[id]
	r.mu.RUnlock()
	if !ok {
		return HookResult{}, ErrHookNotRegistered
	}
	return handler(ctx, input)
}
