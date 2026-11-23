package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"
)

const battleAttrExpJobLease = 30 * time.Second

// A worker processes claimed jobs serially. Claiming more than one lets later
// jobs lose their lease before execution and creates avoidable reclaim races.
const battleAttrExpClaimBatchSize = 1
const battleAttrExpDiagnosticThreshold = 250 * time.Millisecond
const battleAttrExpBacklogThreshold = 6 * time.Second
const battleAttrExpBacklogLogInterval = 5 * time.Second
const battleAttrExpBusyRetryMin = 2 * time.Second
const battleAttrExpBusyRetrySpread = 3 * time.Second

var errBattleAttrExpPayloadInvalid = errors.New("battle attribute experience job payload is invalid")

type battleAttrExpRewardRow struct {
	UserID       int64                        `json:"userId"`
	AttrExp      map[string]float64           `json:"attrExp,omitempty"`
	KittyAttrExp map[int64]map[string]float64 `json:"kittyAttrExp,omitempty"`
}

func battleAttrExpRewardRows(rewards map[int64]store.BattleWaveReward) []battleAttrExpRewardRow {
	userIDs := make([]int64, 0, len(rewards))
	for userID := range rewards {
		if userID > 0 {
			userIDs = append(userIDs, userID)
		}
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	rows := make([]battleAttrExpRewardRow, 0, len(userIDs))
	for _, userID := range userIDs {
		reward := rewards[userID]
		attrExp := make(map[string]float64, len(reward.AttrExp))
		for key, value := range reward.AttrExp {
			if strings.TrimSpace(key) == "" || value == 0 {
				continue
			}
			attrExp[key] = value
		}
		kittyAttrExp := make(map[int64]map[string]float64, len(reward.KittyAttrExp))
		for kittyID, kittyRewards := range reward.KittyAttrExp {
			if kittyID <= 0 {
				continue
			}
			attrs := make(map[string]float64, len(kittyRewards))
			for key, value := range kittyRewards {
				if strings.TrimSpace(key) == "" || value == 0 {
					continue
				}
				attrs[key] += value
			}
			if len(attrs) > 0 {
				kittyAttrExp[kittyID] = attrs
			}
		}
		if len(attrExp) == 0 && len(kittyAttrExp) == 0 {
			continue
		}
		rows = append(rows, battleAttrExpRewardRow{
			UserID:       userID,
			AttrExp:      attrExp,
			KittyAttrExp: kittyAttrExp,
		})
	}
	return rows
}

func battleAttrExpRewardsFromRows(rows []battleAttrExpRewardRow) (map[int64]store.BattleWaveReward, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: reward rows are empty", errBattleAttrExpPayloadInvalid)
	}
	rewards := make(map[int64]store.BattleWaveReward, len(rows))
	for index, row := range rows {
		if row.UserID <= 0 {
			return nil, fmt.Errorf("%w: row %d has invalid user id", errBattleAttrExpPayloadInvalid, index)
		}
		if _, exists := rewards[row.UserID]; exists {
			return nil, fmt.Errorf("%w: row %d duplicates user %d", errBattleAttrExpPayloadInvalid, index, row.UserID)
		}
		reward := store.BattleWaveReward{
			AttrExp:      make(map[string]float64, len(row.AttrExp)),
			KittyAttrExp: make(map[int64]map[string]float64, len(row.KittyAttrExp)),
		}
		for key, value := range row.AttrExp {
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("%w: row %d has an empty player attribute key", errBattleAttrExpPayloadInvalid, index)
			}
			if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
				return nil, fmt.Errorf("%w: row %d has invalid player attribute experience", errBattleAttrExpPayloadInvalid, index)
			}
			reward.AttrExp[key] = value
		}
		for kittyID, attrs := range row.KittyAttrExp {
			if kittyID <= 0 {
				return nil, fmt.Errorf("%w: row %d has invalid kitty id", errBattleAttrExpPayloadInvalid, index)
			}
			if len(attrs) == 0 {
				return nil, fmt.Errorf("%w: row %d has empty kitty attribute rewards", errBattleAttrExpPayloadInvalid, index)
			}
			kittyReward := make(map[string]float64, len(attrs))
			for key, value := range attrs {
				if strings.TrimSpace(key) == "" {
					return nil, fmt.Errorf("%w: row %d has an empty kitty attribute key", errBattleAttrExpPayloadInvalid, index)
				}
				if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
					return nil, fmt.Errorf("%w: row %d has invalid kitty attribute experience", errBattleAttrExpPayloadInvalid, index)
				}
				kittyReward[key] = value
			}
			reward.KittyAttrExp[kittyID] = kittyReward
		}
		if len(reward.AttrExp) == 0 && len(reward.KittyAttrExp) == 0 {
			return nil, fmt.Errorf("%w: row %d has no attribute rewards", errBattleAttrExpPayloadInvalid, index)
		}
		rewards[row.UserID] = reward
	}
	return rewards, nil
}

func battleAttrExpJobRewards(payload []byte) (
	[]battleAttrExpRewardRow,
	map[int64]store.BattleWaveReward,
	error,
) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	rows := []battleAttrExpRewardRow{}
	if err := decoder.Decode(&rows); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errBattleAttrExpPayloadInvalid, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing JSON value")
		}
		return nil, nil, fmt.Errorf("%w: %v", errBattleAttrExpPayloadInvalid, err)
	}
	rewards, err := battleAttrExpRewardsFromRows(rows)
	if err != nil {
		return nil, nil, err
	}
	return rows, rewards, nil
}

func (s *Server) enqueueBattleAttrExpJob(ctx context.Context, runID int64, rewards map[int64]store.BattleWaveReward, now time.Time) error {
	payload, err := battleAttrExpJobPayload(rewards)
	if err != nil {
		return err
	}
	if runID <= 0 || len(payload) == 0 {
		return nil
	}
	return s.deps.Store.EnqueueBattleAttrExpJob(ctx, runID, payload, now)
}

func battleAttrExpJobPayload(rewards map[int64]store.BattleWaveReward) ([]byte, error) {
	if len(rewards) == 0 {
		return nil, nil
	}
	rows := battleAttrExpRewardRows(rewards)
	if len(rows) == 0 {
		return nil, nil
	}
	return json.Marshal(rows)
}

func (s *Server) startBattleAttrExpLoop(workerID string) {
	workers := s.deps.Config.GlobalSettings.Battle.AttrExpWorkers
	if workers <= 0 {
		workers = 1
	}
	for idx := 0; idx < workers; idx++ {
		expWorkerID := fmt.Sprintf("%s:attr-exp:%d", workerID, idx)
		s.launchBattleTask(func() { s.runBattleAttrExpWorker(expWorkerID) })
	}
}

func (s *Server) runBattleAttrExpWorker(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		claimStartedAt := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		jobs, err := s.deps.Store.ClaimBattleAttrExpJobs(ctx, workerID, battleAttrExpClaimBatchSize, battleAttrExpJobLease, time.Now())
		cancel()
		claimElapsed := time.Since(claimStartedAt)
		if err != nil {
			log.Printf("battle attr exp claim failed worker=%s err=%v", workerID, err)
			s.sleepBattleSettlementWorker(battleSettlementWorkerIdleSleep)
			continue
		}
		if len(jobs) == 0 {
			s.sleepBattleSettlementWorker(battleSettlementWorkerIdleSleep)
			continue
		}
		if claimElapsed >= battleAttrExpDiagnosticThreshold && s.shouldLogBattleAttrExpBacklog(time.Now()) {
			log.Printf("battle attr exp claim slow worker=%s elapsed_ms=%d jobs=%d", workerID, claimElapsed.Milliseconds(), len(jobs))
		}
		for _, job := range jobs {
			s.processBattleAttrExpJob(workerID, job)
		}
	}
}

func (s *Server) processBattleAttrExpJob(workerID string, job store.BattleAttrExpJob) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, rewards, err := battleAttrExpJobRewards(job.Payload)
	if err != nil {
		s.quarantineBattleAttrExpJob(ctx, workerID, job, err)
		return
	}
	applyMetrics, err := s.deps.Store.ApplyBattleAttrExpJob(ctx, job.ID, workerID, job.Attempts, rewards)
	if err != nil {
		if errors.Is(err, store.ErrBattleAttrExpJobLeaseLost) {
			log.Printf("battle attr exp lease lost worker=%s job_id=%d run_id=%d attempts=%d", workerID, job.ID, job.RunID, job.Attempts)
			return
		}
		if errors.Is(err, store.ErrBattleAttrExpUserBusy) {
			s.deferBattleAttrExpJobForBusy(ctx, workerID, job)
			return
		}
		s.failBattleAttrExpJob(ctx, workerID, job, err)
		return
	}
	elapsed := time.Since(startedAt)
	queueWait := startedAt.Sub(job.CreatedAt)
	if queueWait < 0 {
		queueWait = 0
	}
	if (elapsed >= battleAttrExpDiagnosticThreshold || queueWait >= battleAttrExpBacklogThreshold) && s.shouldLogBattleAttrExpBacklog(startedAt) {
		reason := "slow"
		if queueWait >= battleAttrExpBacklogThreshold {
			reason = "backlog"
		}
		log.Printf(
			"battle attr exp diagnostic reason=%s worker=%s job_id=%d run_id=%d attempts=%d queue_wait_ms=%d elapsed_ms=%d tx_ms=%d job_lock_ms=%d reward_ms=%d user_lock_ms=%d player_attr_ms=%d kitty_attr_ms=%d done_ms=%d rows=%d users=%d kitties=%d player_attr_keys=%d kitty_attr_keys=%d player_progress_changes=%d kitty_progress_changes=%d player_level_changes=%d kitty_level_changes=%d",
			reason,
			workerID,
			job.ID,
			job.RunID,
			job.Attempts,
			queueWait.Milliseconds(),
			elapsed.Milliseconds(),
			applyMetrics.TransactionElapsed.Milliseconds(),
			applyMetrics.JobLockElapsed.Milliseconds(),
			applyMetrics.RewardApplyElapsed.Milliseconds(),
			applyMetrics.UserLockElapsed.Milliseconds(),
			applyMetrics.PlayerAttrElapsed.Milliseconds(),
			applyMetrics.KittyAttrElapsed.Milliseconds(),
			applyMetrics.DoneMarkElapsed.Milliseconds(),
			len(rows),
			applyMetrics.Users,
			applyMetrics.Kitties,
			applyMetrics.PlayerAttrKeys,
			applyMetrics.KittyAttrKeys,
			applyMetrics.PlayerProgressChanges,
			applyMetrics.KittyProgressChanges,
			applyMetrics.PlayerLevelChanges,
			applyMetrics.KittyLevelChanges,
		)
	}
}

func battleAttrExpBusyRetryDelay(job store.BattleAttrExpJob) time.Duration {
	seed := uint64(job.ID)*31 + uint64(job.Attempts)*17
	jitterSlots := uint64(battleAttrExpBusyRetrySpread / time.Millisecond)
	return battleAttrExpBusyRetryMin + time.Duration(seed%jitterSlots)*time.Millisecond
}

func (s *Server) deferBattleAttrExpJobForBusy(ctx context.Context, workerID string, job store.BattleAttrExpJob) {
	failCtx, failCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer failCancel()
	retryDelay := battleAttrExpBusyRetryDelay(job)
	retryAt := time.Now().Add(retryDelay)
	marked, err := s.deps.Store.MarkBattleAttrExpJobFailed(
		failCtx,
		job.ID,
		workerID,
		job.Attempts,
		store.ErrBattleAttrExpUserBusy.Error(),
		retryAt,
	)
	if err != nil {
		log.Printf("battle attr exp busy retry schedule failed worker=%s job_id=%d run_id=%d err=%v", workerID, job.ID, job.RunID, err)
		return
	}
	if marked && s.shouldLogBattleAttrExpBacklog(time.Now()) {
		log.Printf("battle attr exp diagnostic reason=user_lock_busy worker=%s job_id=%d run_id=%d attempts=%d retry_delay_ms=%d", workerID, job.ID, job.RunID, job.Attempts, retryDelay.Milliseconds())
	}
}

func (s *Server) shouldLogBattleAttrExpBacklog(now time.Time) bool {
	s.battleAttrExpLogMu.Lock()
	defer s.battleAttrExpLogMu.Unlock()
	if !s.battleAttrExpLastLog.IsZero() && now.Sub(s.battleAttrExpLastLog) < battleAttrExpBacklogLogInterval {
		return false
	}
	s.battleAttrExpLastLog = now
	return true
}

func (s *Server) failBattleAttrExpJob(ctx context.Context, workerID string, job store.BattleAttrExpJob, jobErr error) {
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
	marked, err := s.deps.Store.MarkBattleAttrExpJobFailed(failCtx, job.ID, workerID, job.Attempts, jobErr.Error(), retryAt)
	if err != nil {
		log.Printf("battle attr exp mark failed failed worker=%s job_id=%d run_id=%d err=%v original_err=%v", workerID, job.ID, job.RunID, err, jobErr)
		return
	}
	if !marked {
		log.Printf("battle attr exp failure ignored after lease loss worker=%s job_id=%d run_id=%d attempts=%d original_err=%v", workerID, job.ID, job.RunID, job.Attempts, jobErr)
		return
	}
	log.Printf("battle attr exp job failed worker=%s job_id=%d run_id=%d attempts=%d retry_at=%s err=%v", workerID, job.ID, job.RunID, job.Attempts, retryAt.Format(time.RFC3339), jobErr)
}

func (s *Server) quarantineBattleAttrExpJob(ctx context.Context, workerID string, job store.BattleAttrExpJob, jobErr error) {
	quarantineCtx, quarantineCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer quarantineCancel()
	marked, err := s.deps.Store.MarkBattleAttrExpJobQuarantined(quarantineCtx, job.ID, workerID, job.Attempts, jobErr.Error())
	if err != nil {
		log.Printf("battle attr exp quarantine failed worker=%s job_id=%d run_id=%d err=%v original_err=%v", workerID, job.ID, job.RunID, err, jobErr)
		return
	}
	if !marked {
		log.Printf("battle attr exp quarantine ignored after lease loss worker=%s job_id=%d run_id=%d attempts=%d original_err=%v", workerID, job.ID, job.RunID, job.Attempts, jobErr)
		return
	}
	log.Printf("battle attr exp job quarantined worker=%s job_id=%d run_id=%d attempts=%d err=%v", workerID, job.ID, job.RunID, job.Attempts, jobErr)
}
