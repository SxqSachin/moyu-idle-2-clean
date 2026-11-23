package httpapi

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/store"
)

// One bounded dispatcher per gateway, never one goroutine per reconnect. The
// database user lock and recovery revision fence competing gateway instances.
func (s *Server) startOfflineRecoveryWorker() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.Hooks().OnShutdown(func() error { cancel(); <-done; return nil })
	specs := store.BuildActionSpecMap(s.deps.GameData)
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		scanDelay := 100 * time.Millisecond
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			scanCtx, stopScan := context.WithTimeout(ctx, time.Second)
			jobs, err := s.deps.Store.ListDueOfflineRecoveries(scanCtx, 4)
			stopScan()
			if err != nil {
				log.Printf("offline recovery scan failed err=%v", err)
				scanDelay = min(scanDelay*2, 5*time.Second)
				ticker.Reset(scanDelay)
				continue
			}
			if scanDelay != 100*time.Millisecond {
				scanDelay = 100 * time.Millisecond
				ticker.Reset(scanDelay)
			}
			for _, job := range jobs {
				if ctx.Err() != nil {
					return
				}
				started := time.Now()
				chunkCtx, stopChunk := context.WithTimeout(store.WithAuditSource(ctx, "offline_recovery"), 3*time.Second)
				err := s.deps.Store.AdvanceOfflineRecovery(chunkCtx, job, specs)
				stopChunk()
				if err != nil {
					retryCtx, stopRetry := context.WithTimeout(ctx, time.Second)
					retryErr := s.deps.Store.RetryOfflineRecoveryLater(retryCtx, job, err)
					stopRetry()
					log.Printf("offline recovery retry user_id=%d recovery_id=%d revision=%d elapsed_ms=%d err=%v retry_err=%v", job.UserID, job.ID, job.Revision, time.Since(started).Milliseconds(), err, retryErr)
				} else {
					log.Printf("offline recovery chunk user_id=%d recovery_id=%d revision=%d elapsed_ms=%d", job.UserID, job.ID, job.Revision, time.Since(started).Milliseconds())
				}
			}
			scheduleCtx, stopSchedule := context.WithTimeout(ctx, time.Second)
			s.restoreOfflineRecoverySchedules(scheduleCtx)
			stopSchedule()
		}
	}()
}

func (s *Server) restoreOfflineRecoverySchedules(ctx context.Context) {
	if s.deps.Redis == nil {
		return
	}
	jobs, err := s.deps.Store.OfflineRecoverySchedules(ctx)
	if err != nil {
		log.Printf("offline recovery schedule scan failed err=%v", err)
		return
	}
	for id, userID := range jobs {
		if s.deps.Redis != nil {
			if err = s.deps.Redis.ZAdd(ctx, actionruntime.QueueDueKey(), redis.Z{Member: strconv.FormatInt(userID, 10), Score: float64(time.Now().UnixMilli())}).Err(); err != nil {
				continue
			}
		}
		if err = s.deps.Store.MarkOfflineRecoveryScheduled(ctx, id); err != nil {
			log.Printf("offline recovery schedule acknowledge failed id=%d err=%v", id, err)
			continue
		}
		// This is a wake-up hint only. The client still reads the durable receipt;
		// a lost notification is recovered by its bounded polling loop.
		s.pushUserEventCluster(userID, "action.offline.recovery.ready", map[string]any{"taskId": strconv.FormatInt(id, 10)})
	}
}
