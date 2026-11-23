package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type furnitureTestBus struct {
	receive func([]byte)
	fail    bool
}

func (b *furnitureTestBus) SubscribeBytes(_ string, receive func([]byte)) error {
	b.receive = receive
	return nil
}
func (b *furnitureTestBus) PublishJSON(_ context.Context, _ string, value any) error {
	if b.fail {
		return errors.New("bus unavailable")
	}
	raw, _ := json.Marshal(value)
	b.receive(raw)
	return nil
}

func TestFurnitureCacheInvalidationBusIsAdvisoryAndPlayerScoped(t *testing.T) {
	s := &Store{}
	bus := &furnitureTestBus{}
	if err := s.BindFurnitureInvalidations(bus); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	tx := &furnitureCacheTestTx{}
	load := func() {
		t.Helper()
		if _, err := s.furnitureWindowsTx(context.Background(), tx, []int64{1, 2}, at, at); err != nil {
			t.Fatal(err)
		}
	}
	load()
	for _, raw := range []string{`bad`, `{"userId":0,"revision":1}`, `{"userId":1,"revision":0}`} {
		bus.receive([]byte(raw))
	}
	load()
	if tx.queries.Load() != 1 {
		t.Fatal("invalid event flushed cache")
	}
	s.furnitureChanged(context.Background(), 1, 2)
	if s.furnitureEffects.entries[2] == nil || s.furnitureEffects.entries[1] != nil {
		t.Fatal("invalidation crossed player boundary")
	}
	load()
	bus.fail = true
	s.furnitureChanged(context.Background(), 1, 3)
	load()
	if tx.queries.Load() != 2 {
		t.Fatal("failed delivery corrupted valid lease")
	}
	bus.receive([]byte(`{"userId":1,"revision":1}`))
	load()
	if tx.queries.Load() != 3 {
		t.Fatal("late event must evict, never restore older data")
	}
}
