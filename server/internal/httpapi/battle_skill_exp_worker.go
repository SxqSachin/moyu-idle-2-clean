package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"
)

const battleSkillExpJobLease = 30 * time.Second
const battleSkillExpMaxJobRowExp = battleSkillExpMaxCastsPerUnit * store.BattleSkillExpPerUse

// Each worker processes claims serially. A single-item claim keeps queued jobs
// from expiring locally before their lease-fenced transaction starts.
const battleSkillExpClaimBatchSize = 1
const battleSkillExpSlowLogThreshold = 6 * time.Second

var errBattleSkillExpPayloadInvalid = errors.New("battle skill experience job payload is invalid")

type battleSkillExpRewardRow struct {
	UserID    int64  `json:"userId"`
	ActorType string `json:"actorType,omitempty"`
	ActorID   int64  `json:"actorId,omitempty"`
	SkillID   string `json:"skillId"`
	Exp       int64  `json:"exp"`
}

func (s *Server) startBattleSkillExpLoop(workerID string) {
	workers := s.deps.Config.GlobalSettings.Battle.SkillExpWorkers
	if workers <= 0 {
		workers = 1
	}
	for idx := 0; idx < workers; idx++ {
		expWorkerID := fmt.Sprintf("%s:skill-exp:%d", workerID, idx)
		s.launchBattleTask(func() { s.runBattleSkillExpWorker(expWorkerID) })
	}
}

func (s *Server) runBattleSkillExpWorker(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		jobs, err := s.deps.Store.ClaimBattleSkillExpJobs(ctx, workerID, battleSkillExpClaimBatchSize, battleSkillExpJobLease, time.Now())
		cancel()
		if err != nil {
			log.Printf("battle skill exp claim failed worker=%s err=%v", workerID, err)
			s.sleepBattleSettlementWorker(battleSettlementWorkerIdleSleep)
			continue
		}
		if len(jobs) == 0 {
			s.sleepBattleSettlementWorker(battleSettlementWorkerIdleSleep)
			continue
		}
		for _, job := range jobs {
			s.processBattleSkillExpJob(workerID, job)
		}
	}
}

func (s *Server) processBattleSkillExpJob(workerID string, job store.BattleSkillExpJob) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, rewards, kittyRewards, err := battleSkillExpJobRewards(job.Payload)
	if err != nil {
		s.quarantineBattleSkillExpJob(ctx, workerID, job, err)
		return
	}
	if err := s.deps.Store.ApplyBattleSkillExpJob(ctx, job.ID, workerID, job.Attempts, rewards, kittyRewards); err != nil {
		if errors.Is(err, store.ErrBattleSkillExpJobLeaseLost) {
			log.Printf("battle skill exp lease lost worker=%s job_id=%d run_id=%d attempts=%d", workerID, job.ID, job.RunID, job.Attempts)
			return
		}
		if errors.Is(err, store.ErrSkillExpOverflow) {
			s.quarantineBattleSkillExpJob(ctx, workerID, job, err)
			return
		}
		s.failBattleSkillExpJob(ctx, workerID, job, err)
		return
	}
	elapsed := time.Since(startedAt)
	queueWait := startedAt.Sub(job.CreatedAt)
	if queueWait < 0 {
		queueWait = 0
	}
	if elapsed >= battleSkillExpSlowLogThreshold || queueWait >= battleSkillExpSlowLogThreshold {
		log.Printf("battle skill exp job slow worker=%s job_id=%d run_id=%d attempts=%d queue_wait_ms=%d elapsed_ms=%d rows=%d", workerID, job.ID, job.RunID, job.Attempts, queueWait.Milliseconds(), elapsed.Milliseconds(), len(rows))
	}
}

func battleSkillExpJobRewards(payload []byte) (
	[]battleSkillExpRewardRow,
	map[int64]map[string]int64,
	map[int64]map[string]int64,
	error,
) {
	rows := []battleSkillExpRewardRow{}
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %v", errBattleSkillExpPayloadInvalid, err)
	}
	if len(rows) == 0 {
		return nil, nil, nil, fmt.Errorf("%w: reward rows are empty", errBattleSkillExpPayloadInvalid)
	}
	rewards := map[int64]map[string]int64{}
	kittyRewards := map[int64]map[string]int64{}
	for index, row := range rows {
		actorType := strings.TrimSpace(strings.ToLower(row.ActorType))
		skillID := strings.TrimSpace(row.SkillID)
		if row.Exp <= 0 || row.Exp > battleSkillExpMaxJobRowExp || skillID == "" {
			return nil, nil, nil, fmt.Errorf("%w: row %d has invalid skill or exp", errBattleSkillExpPayloadInvalid, index)
		}
		switch actorType {
		case "kitty":
			if row.ActorID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: row %d has invalid kitty id", errBattleSkillExpPayloadInvalid, index)
			}
			if _, ok := kittyRewards[row.ActorID]; !ok {
				kittyRewards[row.ActorID] = map[string]int64{}
			}
			current := kittyRewards[row.ActorID][skillID]
			if current > math.MaxInt64-row.Exp {
				return nil, nil, nil, fmt.Errorf("%w: row %d overflows kitty skill exp", errBattleSkillExpPayloadInvalid, index)
			}
			kittyRewards[row.ActorID][skillID] = current + row.Exp
		case "", "player":
			if row.UserID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: row %d has invalid user id", errBattleSkillExpPayloadInvalid, index)
			}
			if _, ok := rewards[row.UserID]; !ok {
				rewards[row.UserID] = map[string]int64{}
			}
			current := rewards[row.UserID][skillID]
			if current > math.MaxInt64-row.Exp {
				return nil, nil, nil, fmt.Errorf("%w: row %d overflows player skill exp", errBattleSkillExpPayloadInvalid, index)
			}
			rewards[row.UserID][skillID] = current + row.Exp
		default:
			return nil, nil, nil, fmt.Errorf("%w: row %d has invalid actor type", errBattleSkillExpPayloadInvalid, index)
		}
	}
	return rows, rewards, kittyRewards, nil
}

func (s *Server) failBattleSkillExpJob(ctx context.Context, workerID string, job store.BattleSkillExpJob, jobErr error) {
	failCtx, failCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer failCancel()
	retryDelay := time.Duration(job.Attempts) * time.Second
	if retryDelay < time.Second {
		retryDelay = time.Second
	}
	if retryDelay > 30*time.Second {
		retryDelay = 30 * time.Second
	}
	retryAt := time.Now().Add(retryDelay)
	marked, err := s.deps.Store.MarkBattleSkillExpJobFailed(failCtx, job.ID, workerID, job.Attempts, jobErr.Error(), retryAt)
	if err != nil {
		log.Printf("battle skill exp mark failed failed worker=%s job_id=%d run_id=%d err=%v original_err=%v", workerID, job.ID, job.RunID, err, jobErr)
		return
	}
	if !marked {
		log.Printf("battle skill exp failure ignored after lease loss worker=%s job_id=%d run_id=%d attempts=%d original_err=%v", workerID, job.ID, job.RunID, job.Attempts, jobErr)
		return
	}
	log.Printf("battle skill exp job failed worker=%s job_id=%d run_id=%d attempts=%d retry_at=%s err=%v", workerID, job.ID, job.RunID, job.Attempts, retryAt.Format(time.RFC3339), jobErr)
}

func (s *Server) quarantineBattleSkillExpJob(ctx context.Context, workerID string, job store.BattleSkillExpJob, jobErr error) {
	quarantineCtx, quarantineCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer quarantineCancel()
	marked, err := s.deps.Store.MarkBattleSkillExpJobQuarantined(quarantineCtx, job.ID, workerID, job.Attempts, jobErr.Error())
	if err != nil {
		log.Printf("battle skill exp quarantine failed worker=%s job_id=%d run_id=%d err=%v original_err=%v", workerID, job.ID, job.RunID, err, jobErr)
		return
	}
	if !marked {
		log.Printf("battle skill exp quarantine ignored after lease loss worker=%s job_id=%d run_id=%d attempts=%d original_err=%v", workerID, job.ID, job.RunID, job.Attempts, jobErr)
		return
	}
	log.Printf("battle skill exp job quarantined worker=%s job_id=%d run_id=%d attempts=%d err=%v", workerID, job.ID, job.RunID, job.Attempts, jobErr)
}
