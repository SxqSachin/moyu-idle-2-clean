package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerSerializesSamePlayerEvents(t *testing.T) {
	s := New(Options{
		Engine:     EngineMinHeap,
		ShardCount: 2,
		Tick:       5 * time.Millisecond,
		WheelSize:  64,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const total = 6
	done := make(chan struct{}, total)
	var active int32
	var maxActive int32

	s.SetHandler(func(_ context.Context, _ Event) {
		current := atomic.AddInt32(&active, 1)
		for {
			prev := atomic.LoadInt32(&maxActive)
			if current <= prev || atomic.CompareAndSwapInt32(&maxActive, prev, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		done <- struct{}{}
	})
	s.Start(ctx)
	defer s.Stop()

	triggerAt := time.Now().Add(15 * time.Millisecond)
	for i := 0; i < total; i++ {
		s.Add(Event{
			ID:        "event-same-player",
			PlayerID:  1001,
			Type:      "combat.event",
			TriggerAt: triggerAt,
		})
	}

	timeout := time.After(2 * time.Second)
	for i := 0; i < total; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatalf("timed out waiting for event #%d", i+1)
		}
	}

	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("expected same-player events to run serially, max concurrency = %d", got)
	}
}
