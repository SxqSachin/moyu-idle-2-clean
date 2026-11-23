package httpapi

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

const (
	battleRunHistoryCleanupQueueCapacity = 4096
	battleRunHistoryCleanupRoomBatch     = 64
	battleRunHistoryCleanupDeleteBatch   = 64
	battleRunHistoryCleanupCoalesceDelay = 500 * time.Millisecond
	battleRunHistoryCleanupRetryDelay    = 2 * time.Second
	battleRunHistoryCleanupTimeout       = 3 * time.Second
	battleRunHistoryCleanupSlowThreshold = 250 * time.Millisecond
)

type battleRunHistoryCleanupQueue struct {
	mu       sync.Mutex
	pending  map[int64]struct{}
	wake     chan struct{}
	capacity int
	dropped  uint64
}

func newBattleRunHistoryCleanupQueue(capacity int) *battleRunHistoryCleanupQueue {
	if capacity <= 0 {
		capacity = battleRunHistoryCleanupQueueCapacity
	}
	return &battleRunHistoryCleanupQueue{
		pending:  make(map[int64]struct{}, capacity),
		wake:     make(chan struct{}, 1),
		capacity: capacity,
	}
}

func (q *battleRunHistoryCleanupQueue) signal() {
	if q == nil {
		return
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *battleRunHistoryCleanupQueue) enqueue(roomID int64) bool {
	if q == nil || roomID <= 0 {
		return false
	}
	q.mu.Lock()
	if _, exists := q.pending[roomID]; exists {
		q.mu.Unlock()
		return true
	}
	if len(q.pending) >= q.capacity {
		q.dropped++
		q.mu.Unlock()
		return false
	}
	q.pending[roomID] = struct{}{}
	q.mu.Unlock()
	q.signal()
	return true
}

func (q *battleRunHistoryCleanupQueue) take(limit int) ([]int64, uint64) {
	if q == nil {
		return nil, 0
	}
	q.mu.Lock()
	if limit <= 0 || limit > len(q.pending) {
		limit = len(q.pending)
	}
	roomIDs := make([]int64, 0, limit)
	for roomID := range q.pending {
		roomIDs = append(roomIDs, roomID)
		delete(q.pending, roomID)
		if len(roomIDs) == limit {
			break
		}
	}
	dropped := q.dropped
	q.dropped = 0
	remaining := len(q.pending) > 0
	q.mu.Unlock()
	sort.Slice(roomIDs, func(i, j int) bool { return roomIDs[i] < roomIDs[j] })
	if remaining {
		q.signal()
	}
	return roomIDs, dropped
}

func (q *battleRunHistoryCleanupQueue) requeue(roomIDs []int64) {
	for _, roomID := range roomIDs {
		q.enqueue(roomID)
	}
}

// scheduleBattleRunHistoryCleanup is deliberately best-effort: a missed signal
// can only leave surplus history, and a later run or admin purge can retry it.
func (s *Server) scheduleBattleRunHistoryCleanup(roomID int64) bool {
	if s == nil || !s.battleLoopEnabled || s.battleRunHistoryCleanup == nil || roomID <= 0 {
		return false
	}
	if s.battleStopCh != nil {
		select {
		case <-s.battleStopCh:
			return false
		default:
		}
	}
	return s.battleRunHistoryCleanup.enqueue(roomID)
}

func (s *Server) startBattleRunHistoryCleanupLoop() {
	if s.battleRunHistoryCleanup == nil {
		s.battleRunHistoryCleanup = newBattleRunHistoryCleanupQueue(battleRunHistoryCleanupQueueCapacity)
	}
	if !s.launchBattleTask(s.runBattleRunHistoryCleanupLoop) {
		return
	}
	log.Printf(
		"battle run history cleanup worker started queue_capacity=%d room_batch=%d delete_batch=%d coalesce_ms=%d",
		s.battleRunHistoryCleanup.capacity,
		battleRunHistoryCleanupRoomBatch,
		battleRunHistoryCleanupDeleteBatch,
		battleRunHistoryCleanupCoalesceDelay.Milliseconds(),
	)
}

func (s *Server) runBattleRunHistoryCleanupLoop() {
	queue := s.battleRunHistoryCleanup
	for {
		select {
		case <-queue.wake:
		case <-s.battleStopCh:
			return
		}
		if !waitForBattleRunHistoryCleanup(s.battleStopCh, battleRunHistoryCleanupCoalesceDelay) {
			return
		}
		drainBattleRunHistoryCleanupWake(queue.wake)
		roomIDs, dropped := queue.take(battleRunHistoryCleanupRoomBatch)
		if dropped > 0 {
			log.Printf("battle run history cleanup queue saturated dropped=%d capacity=%d", dropped, queue.capacity)
		}
		if len(roomIDs) == 0 {
			continue
		}

		startedAt := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), battleRunHistoryCleanupTimeout)
		deleted, err := s.deps.Store.CompactBattleRunHistories(
			ctx,
			roomIDs,
			s.deps.Config.GlobalSettings.Battle.RunHistoryKeepCount,
			battleRunHistoryCleanupDeleteBatch,
		)
		cancel()
		elapsed := time.Since(startedAt)
		if err != nil {
			queue.requeue(roomIDs)
			log.Printf("battle run history cleanup failed rooms=%d cleanup_ms=%d err=%v", len(roomIDs), elapsed.Milliseconds(), err)
			if !waitForBattleRunHistoryCleanup(s.battleStopCh, battleRunHistoryCleanupRetryDelay) {
				return
			}
			continue
		}
		if deleted >= battleRunHistoryCleanupDeleteBatch {
			queue.requeue(roomIDs)
		}
		if elapsed >= battleRunHistoryCleanupSlowThreshold {
			log.Printf("battle run history cleanup slow rooms=%d deleted=%d cleanup_ms=%d", len(roomIDs), deleted, elapsed.Milliseconds())
		}
	}
}

func drainBattleRunHistoryCleanupWake(wake <-chan struct{}) {
	for {
		select {
		case <-wake:
		default:
			return
		}
	}
}

func waitForBattleRunHistoryCleanup(stop <-chan struct{}, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-stop:
		return false
	}
}
