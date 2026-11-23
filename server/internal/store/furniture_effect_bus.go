package store

import (
	"context"
	"encoding/json"
	"log"
)

type furnitureInvalidationBus interface {
	PublishJSON(context.Context, string, any) error
	SubscribeBytes(string, func([]byte)) error
}

// Bind before serving requests. Delivery accelerates refresh; fixed leases
// remain the safety boundary. The bus owns subscription shutdown.
func (s *Store) BindFurnitureInvalidations(bus furnitureInvalidationBus) error {
	const topic = "runtime.furniture.changed"
	type event struct {
		UserID   int64 `json:"userId"`
		Revision int64 `json:"revision"`
	}
	if err := bus.SubscribeBytes(topic, func(raw []byte) {
		var e event
		if len(raw) > 1024 || json.Unmarshal(raw, &e) != nil || e.UserID <= 0 || e.Revision <= 0 {
			return
		}
		s.InvalidateFurnitureEffects(e.UserID)
	}); err != nil {
		return err
	}
	s.furnitureChanged = func(ctx context.Context, userID, revision int64) {
		if err := bus.PublishJSON(ctx, topic, event{UserID: userID, Revision: revision}); err != nil {
			log.Printf("furniture cache invalidation deferred to lease: %v", err)
		}
	}
	return nil
}
