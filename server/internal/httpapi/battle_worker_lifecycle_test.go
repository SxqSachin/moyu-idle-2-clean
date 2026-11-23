package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBattleWorkerShutdownWaitsForInFlightTask(t *testing.T) {
	s := &Server{battleLoopEnabled: true, battleStopCh: make(chan struct{})}
	started := make(chan struct{})
	release := make(chan struct{})
	if !s.launchBattleTask(func() {
		close(started)
		<-release
	}) {
		t.Fatal("task was not launched")
	}
	<-started

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		done <- s.ShutdownBattleWorker(ctx)
	}()
	select {
	case err := <-done:
		t.Fatalf("shutdown returned before task completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	if s.launchBattleTask(func() {}) {
		t.Fatal("shutdown accepted a new worker task")
	}
}

func TestBattleWorkerShutdownHonorsContext(t *testing.T) {
	s := &Server{battleLoopEnabled: true, battleStopCh: make(chan struct{})}
	release := make(chan struct{})
	if !s.launchBattleTask(func() { <-release }) {
		t.Fatal("task was not launched")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err := s.ShutdownBattleWorker(ctx)
	if !errors.Is(err, errBattleWorkerShutdownTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error=%v", err)
	}
	close(release)
	if err := s.ShutdownBattleWorker(context.Background()); err != nil {
		t.Fatalf("second shutdown did not join the released task: %v", err)
	}
}

func TestBattleWorkerShutdownStopsMutationSyncWorker(t *testing.T) {
	s := &Server{
		battleLoopEnabled: true,
		battleStopCh:      make(chan struct{}),
		mutationSyncQueue: make(chan mutationSyncScope, 1),
	}
	s.startMutationSyncWorker()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.ShutdownBattleWorker(ctx); err != nil {
		t.Fatalf("shutdown mutation sync worker: %v", err)
	}
}
