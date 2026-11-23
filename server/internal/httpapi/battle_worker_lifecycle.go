package httpapi

import (
	"context"
	"errors"
)

var errBattleWorkerShutdownTimeout = errors.New("battle worker shutdown timed out")

// launchBattleTask registers every worker-owned goroutine before it starts.
// The lifecycle mutex prevents WaitGroup.Add from racing with shutdown Wait.
func (s *Server) launchBattleTask(task func()) bool {
	if s == nil || task == nil {
		return false
	}
	s.battleLifecycleMu.Lock()
	if s.battleStopping {
		s.battleLifecycleMu.Unlock()
		return false
	}
	s.battleWorkerWG.Add(1)
	s.battleLifecycleMu.Unlock()
	go func() {
		defer s.battleWorkerWG.Done()
		task()
	}()
	return true
}

// launchBattleTaskOrRun preserves durable follow-up work during shutdown. If
// shutdown has already started, the tracked caller finishes the work inline
// before it can release the database pool.
func (s *Server) launchBattleTaskOrRun(task func()) {
	if task == nil {
		return
	}
	if !s.launchBattleTask(task) {
		task()
	}
}

// ShutdownBattleWorker stops new dispatch, then waits until every in-flight
// battle task has stopped using PostgreSQL, Redis, and NATS.
func (s *Server) ShutdownBattleWorker(ctx context.Context) error {
	if s == nil || !s.battleLoopEnabled {
		return nil
	}
	s.battleLifecycleMu.Lock()
	if !s.battleStopping {
		s.battleStopping = true
		close(s.battleStopCh)
	}
	s.battleLifecycleMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.battleWorkerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.Join(errBattleWorkerShutdownTimeout, ctx.Err())
	}
}
