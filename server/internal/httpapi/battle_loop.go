package httpapi

// TODO(phase3-extract): move battle worker main loop from httpapi into
// internal/battle/runtime/worker_loop.go and keep httpapi transport-focused.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const battleRoomLeaseDuration = 3 * time.Second
const battleLoopMinSleep = 20 * time.Millisecond
const battleLoopMaxSleep = 2 * time.Second
const battleLoopIdleSleep = 300 * time.Millisecond
const battleTickBatchSlowLogThreshold = 250 * time.Millisecond
const battleTickRoomSlowLogThreshold = 200 * time.Millisecond
const battleRunStartSlowLogThresholdMs = int64(4500)

var battleDuePopScript = redis.NewScript(`
local ids = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, ARGV[2])
if #ids > 0 then
	redis.call("ZREM", KEYS[1], unpack(ids))
end
return ids
`)

type battleTickRoomMetrics struct {
	CompiledRuns       int64
	CompiledTurns      int64
	LoadDetailElapsed  time.Duration
	EnsureRunElapsed   time.Duration
	LoadRuntimeElapsed time.Duration
	CompileElapsed     time.Duration
	HoldLeaseElapsed   time.Duration
	PlaybackElapsed    time.Duration
	ScheduleElapsed    time.Duration
	TotalElapsed       time.Duration
	AsyncPlayback      bool
	CompileTimedOut    bool
	CompileStepLimit   bool
}

type battleTickClaimPolicy struct {
	MinRooms           int
	MaxRooms           int
	TargetLeaseUtilPct int
}

func (s *Server) startBattleLoop() {
	workerID := battleWorkerRuntimeID()
	cfg := s.deps.Config.GlobalSettings.Battle
	// V1 dispatch is retired. Existing V1 rooms are intentionally left untouched;
	// user/admin transitions may still normalize them onto the current V2 schema.
	s.deps.Store.SetBattleStateMachineV2(true)
	log.Printf(
		"battle worker config snapshot worker=%s state_machine_v2=true inter_battle_interval_ms=%d recovery_duration_ms=%d playback_fixed_interval_ms=%d timeline_push_interval_ms=%d tick_claim_rooms_min=%d tick_claim_rooms_max=%d tick_shard_workers=%d compile_kickoff_slots=%d compile_first_queue_capacity=%d compile_turns_per_player_unit=%d settlement_workers=%d skill_exp_workers=%d attr_exp_workers=%d finalize_slots=%d claim_playback_dispatch_limit=%d ws_battle_batch_compression=%t",
		workerID,
		cfg.InterBattleIntervalMs,
		cfg.RecoveryDurationMs,
		cfg.PlaybackFixedIntervalMs,
		cfg.TimelinePushIntervalMs,
		cfg.TickClaimRoomsMin,
		cfg.TickClaimRoomsMax,
		cfg.TickShardWorkers,
		cfg.CompileKickoffSlots,
		cap(s.battleCompileFirstQ),
		cfg.CompileTurnsPerPlayerUnit,
		cfg.SettlementWorkers,
		cfg.SkillExpWorkers,
		cfg.AttrExpWorkers,
		cfg.FinalizeSlots,
		battleV2ClaimLimit,
		s.deps.Config.WSBattleBatchCompression,
	)
	s.startBattleCompileKickoffWorkers(workerID)
	s.startBattleSettlementLoop(workerID)
	s.startBattleSkillExpLoop(workerID)
	s.startBattleAttrExpLoop(workerID)
	s.startBattleWorkerHeartbeatLoop(workerID)
	s.startBattleRunHistoryCleanupLoop()
	s.startBattleV2Loops(workerID)
	s.startArenaChallengeLoop(workerID)
	s.startCombatTrialLoop(workerID)
	log.Printf("battle v1 legacy dispatch disabled worker=%s", workerID)
}

func (s *Server) startBattleWorkerHeartbeatLoop(workerID string) {
	refreshInterval := time.Duration(s.deps.Config.GlobalSettings.Battle.WorkerHeartbeat.RefreshIntervalSeconds) * time.Second
	if refreshInterval <= 0 {
		refreshInterval = 5 * time.Second
	}
	s.launchBattleTask(func() {
		s.publishBattleWorkerHeartbeat(workerID, time.Now())
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				s.publishBattleWorkerHeartbeat(workerID, now)
			case <-s.battleStopCh:
				return
			}
		}
	})
}

func (s *Server) tickBattleRooms(workerID string, now time.Time, execPerRoomEWMA time.Duration) (time.Duration, time.Duration) {
	s.battleTickMu.Lock()
	defer s.battleTickMu.Unlock()
	claimCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	statuses := []string{store.BattleRoomStatusEntering, store.BattleRoomStatusPlaying, store.BattleRoomStatusRecovering}
	claimPolicy := battleTickClaimPolicy{
		MinRooms:           s.deps.Config.GlobalSettings.Battle.TickClaimRoomsMin,
		MaxRooms:           s.deps.Config.GlobalSettings.Battle.TickClaimRoomsMax,
		TargetLeaseUtilPct: s.deps.Config.GlobalSettings.Battle.TickTargetLeaseUtilPct,
	}
	claimLimit := battleTickClaimRoomLimit(claimPolicy, battleRoomLeaseDuration, execPerRoomEWMA)
	claimLimit = s.battleCompileQueuePressure().applyClaimLimit(claimLimit)
	// State-advance workers should not be throttled by playback cadence.
	// Playback dispatch is handled by a dedicated loop, and over-throttling here
	// causes long gaps (10s+) before rooms with current_run_id=NULL can start next run.
	playbackClaimCap := claimLimit
	claimLookahead := battleTickClaimLookahead(execPerRoomEWMA, claimLimit, battleRoomLeaseDuration)
	claimCutoff := now.Add(claimLookahead)
	claimStartedAt := time.Now()
	readyFastLaneLimit := maxInt(claimPolicy.MinRooms, claimLimit/2)
	if readyFastLaneLimit > claimLimit {
		readyFastLaneLimit = claimLimit
	}
	readyFastLaneRooms, readyErr := s.deps.Store.ClaimBattleReadyNextRooms(claimCtx, workerID, readyFastLaneLimit, battleRoomLeaseDuration, now)
	if readyErr != nil {
		log.Printf("battle loop claim ready-next rooms failed worker=%s err=%v", workerID, readyErr)
	}
	duePopLimit := claimLimit * 4
	if duePopLimit < claimLimit {
		duePopLimit = claimLimit
	}
	if duePopLimit > 512 {
		duePopLimit = 512
	}
	dueRoomIDs := s.popBattleDueRoomIDs(claimCtx, now, duePopLimit)
	// Filter out rooms with active playback workers. Playback rooms are handled by
	// the dedicated playback dispatch loop and don't need state-advance processing.
	// This avoids the expensive full-table-scan fallback when the due queue is
	// dominated by playback-room IDs.
	filteredDueRoomIDs := s.filterPlaybackRoomsFromDueIDs(claimCtx, dueRoomIDs)
	var rooms []store.BattleRoom
	var err error
	if len(filteredDueRoomIDs) > 0 {
		rooms, err = s.deps.Store.ClaimBattleStateAdvanceRoomsByIDs(claimCtx, workerID, statuses, filteredDueRoomIDs, battleRoomLeaseDuration, now)
		// Fallback: if by-ID claim returned nothing (e.g., all filtered rooms are in
		// a state not eligible for claiming), fall back to DB-wide claim.
		if err == nil && len(rooms) == 0 {
			rooms, err = s.deps.Store.ClaimBattleStateAdvanceRooms(claimCtx, workerID, statuses, claimLimit, battleRoomLeaseDuration, claimCutoff)
			if len(filteredDueRoomIDs) > 0 && len(rooms) > 0 {
				log.Printf(
					"battle state-advance fallback claim worker=%s due_ids=%d filtered_ids=%d claimed=%d claim_limit=%d lookahead=%s",
					workerID,
					len(dueRoomIDs),
					len(filteredDueRoomIDs),
					len(rooms),
					claimLimit,
					claimLookahead,
				)
			}
		}
	} else if len(dueRoomIDs) == 0 {
		// No due rooms at all — normal DB-wide claim.
		rooms, err = s.deps.Store.ClaimBattleStateAdvanceRooms(claimCtx, workerID, statuses, claimLimit, battleRoomLeaseDuration, claimCutoff)
	}
	// If len(dueRoomIDs) > 0 but len(filteredDueRoomIDs) == 0, all due rooms are in
	// active playback — skip claiming entirely. No fallback to full-table-scan.
	claimElapsed := time.Since(claimStartedAt)
	if err != nil {
		log.Printf("battle loop claim rooms failed worker=%s err=%v", workerID, err)
		return battleLoopIdleSleep, execPerRoomEWMA
	}
	if len(readyFastLaneRooms) > 0 {
		rooms = append(readyFastLaneRooms, rooms...)
	}
	if len(rooms) > 1 {
		seen := make(map[int64]struct{}, len(rooms))
		dedup := make([]store.BattleRoom, 0, len(rooms))
		for _, room := range rooms {
			if _, ok := seen[room.ID]; ok {
				continue
			}
			seen[room.ID] = struct{}{}
			dedup = append(dedup, room)
		}
		rooms = dedup
	}
	if len(rooms) > 0 {
		concurrency := battleTickShardWorkerCount(len(rooms), s.deps.Config.GlobalSettings.Battle.TickShardWorkers)
		shards := make([][]store.BattleRoom, concurrency)
		for _, room := range rooms {
			shardIdx := int(room.ID % int64(concurrency))
			if shardIdx < 0 {
				shardIdx = -shardIdx
			}
			shards[shardIdx] = append(shards[shardIdx], room)
		}
		type shardResult struct {
			metrics           battleTickRoomMetrics
			stateInvalidCount int
		}
		shardResults := make([]shardResult, concurrency)
		execStartedAt := time.Now()
		var wg sync.WaitGroup
		for shardIdx := range shards {
			shardRooms := shards[shardIdx]
			if len(shardRooms) == 0 {
				continue
			}
			wg.Add(1)
			go func(idx int, roomsInShard []store.BattleRoom) {
				defer wg.Done()
				local := &shardResults[idx]
				for _, room := range roomsInShard {
					roomMetrics, err := s.processClaimedBattleRoomTick(workerID, room)
					if err != nil {
						if errors.Is(err, store.ErrBattleRoomStateInvalid) {
							local.stateInvalidCount++
							continue
						}
						log.Printf("battle room shard tick failed room_id=%d worker=%s err=%v", room.ID, workerID, err)
						continue
					}
					local.metrics.CompiledRuns += roomMetrics.CompiledRuns
					local.metrics.CompiledTurns += roomMetrics.CompiledTurns
					local.metrics.LoadDetailElapsed += roomMetrics.LoadDetailElapsed
					local.metrics.EnsureRunElapsed += roomMetrics.EnsureRunElapsed
					local.metrics.LoadRuntimeElapsed += roomMetrics.LoadRuntimeElapsed
					local.metrics.CompileElapsed += roomMetrics.CompileElapsed
					local.metrics.HoldLeaseElapsed += roomMetrics.HoldLeaseElapsed
					local.metrics.PlaybackElapsed += roomMetrics.PlaybackElapsed
					local.metrics.ScheduleElapsed += roomMetrics.ScheduleElapsed
					local.metrics.TotalElapsed += roomMetrics.TotalElapsed
					local.metrics.AsyncPlayback = local.metrics.AsyncPlayback || roomMetrics.AsyncPlayback
					local.metrics.CompileTimedOut = local.metrics.CompileTimedOut || roomMetrics.CompileTimedOut
					local.metrics.CompileStepLimit = local.metrics.CompileStepLimit || roomMetrics.CompileStepLimit
					if roomMetrics.CompileElapsed > 0 {
						s.battleMetrics.AddCompile(battleStageMetricSample{
							Elapsed:     roomMetrics.CompileElapsed,
							Turns:       roomMetrics.CompiledTurns,
							TimedOut:    roomMetrics.CompileTimedOut,
							StepLimited: roomMetrics.CompileStepLimit,
						})
					}
					if roomMetrics.PlaybackElapsed > 0 {
						s.battleMetrics.AddPlayback(battleStageMetricSample{
							Elapsed: roomMetrics.PlaybackElapsed,
						})
					}
				}
			}(shardIdx, shardRooms)
		}
		wg.Wait()
		batchMetrics := battleTickRoomMetrics{}
		stateInvalidCount := 0
		for _, result := range shardResults {
			batchMetrics.CompiledRuns += result.metrics.CompiledRuns
			batchMetrics.CompiledTurns += result.metrics.CompiledTurns
			batchMetrics.LoadDetailElapsed += result.metrics.LoadDetailElapsed
			batchMetrics.EnsureRunElapsed += result.metrics.EnsureRunElapsed
			batchMetrics.LoadRuntimeElapsed += result.metrics.LoadRuntimeElapsed
			batchMetrics.CompileElapsed += result.metrics.CompileElapsed
			batchMetrics.HoldLeaseElapsed += result.metrics.HoldLeaseElapsed
			batchMetrics.PlaybackElapsed += result.metrics.PlaybackElapsed
			batchMetrics.ScheduleElapsed += result.metrics.ScheduleElapsed
			batchMetrics.TotalElapsed += result.metrics.TotalElapsed
			batchMetrics.AsyncPlayback = batchMetrics.AsyncPlayback || result.metrics.AsyncPlayback
			batchMetrics.CompileTimedOut = batchMetrics.CompileTimedOut || result.metrics.CompileTimedOut
			batchMetrics.CompileStepLimit = batchMetrics.CompileStepLimit || result.metrics.CompileStepLimit
			stateInvalidCount += result.stateInvalidCount
		}
		execElapsed := time.Since(execStartedAt)
		execPerRoomEWMA = battleTickUpdateExecPerRoomEWMA(execPerRoomEWMA, execElapsed, len(rooms))
		batchElapsed := claimElapsed + execElapsed
		if batchElapsed >= battleRoomLeaseDuration*8/10 {
			utilizationPct := int64(0)
			if battleRoomLeaseDuration > 0 {
				utilizationPct = batchElapsed.Milliseconds() * 100 / battleRoomLeaseDuration.Milliseconds()
			}
			log.Printf(
				"battle tick batch near lease budget worker=%s rooms=%d ready_fast_lane_claimed=%d concurrency=%d claim_limit=%d claim_playback_cap=%d claim_lookahead=%s claim_elapsed=%s exec_elapsed=%s elapsed=%s lease_budget=%s utilization=%d%% exec_per_room_ewma=%s due_popped=%d due_claimed=%d state_invalid=%d stages_ms=load_detail:%d ensure_run:%d load_runtime:%d compile:%d hold_lease:%d playback:%d schedule:%d total:%d",
				workerID,
				len(rooms),
				len(readyFastLaneRooms),
				concurrency,
				claimLimit,
				playbackClaimCap,
				claimLookahead,
				claimElapsed,
				execElapsed,
				batchElapsed,
				battleRoomLeaseDuration,
				utilizationPct,
				execPerRoomEWMA,
				len(dueRoomIDs),
				minInt(len(dueRoomIDs), len(rooms)),
				stateInvalidCount,
				batchMetrics.LoadDetailElapsed.Milliseconds(),
				batchMetrics.EnsureRunElapsed.Milliseconds(),
				batchMetrics.LoadRuntimeElapsed.Milliseconds(),
				batchMetrics.CompileElapsed.Milliseconds(),
				batchMetrics.HoldLeaseElapsed.Milliseconds(),
				batchMetrics.PlaybackElapsed.Milliseconds(),
				batchMetrics.ScheduleElapsed.Milliseconds(),
				batchMetrics.TotalElapsed.Milliseconds(),
			)
			if batchMetrics.CompiledRuns > 0 {
				log.Printf(
					"battle tick batch compile load worker=%s rooms=%d compiled_runs=%d compiled_turns=%d",
					workerID,
					len(rooms),
					batchMetrics.CompiledRuns,
					batchMetrics.CompiledTurns,
				)
			}
		} else if batchElapsed > battleTickBatchSlowLogThreshold {
			estimatedSweepMs := int64(0)
			if len(rooms) > 0 && execPerRoomEWMA > 0 {
				estimatedSweepMs = (execPerRoomEWMA * time.Duration(claimLimit)).Milliseconds()
			}
			log.Printf(
				"battle tick batch finished worker=%s rooms=%d ready_fast_lane_claimed=%d concurrency=%d claim_limit=%d claim_playback_cap=%d claim_lookahead=%s claim_elapsed=%s exec_elapsed=%s elapsed=%s compiled_runs=%d compiled_turns=%d exec_per_room_ewma=%s est_sweep_ms=%d due_popped=%d due_claimed=%d state_invalid=%d stages_ms=load_detail:%d ensure_run:%d load_runtime:%d compile:%d hold_lease:%d playback:%d schedule:%d total:%d",
				workerID,
				len(rooms),
				len(readyFastLaneRooms),
				concurrency,
				claimLimit,
				playbackClaimCap,
				claimLookahead,
				claimElapsed,
				execElapsed,
				batchElapsed,
				batchMetrics.CompiledRuns,
				batchMetrics.CompiledTurns,
				execPerRoomEWMA,
				estimatedSweepMs,
				len(dueRoomIDs),
				minInt(len(dueRoomIDs), len(rooms)),
				stateInvalidCount,
				batchMetrics.LoadDetailElapsed.Milliseconds(),
				batchMetrics.EnsureRunElapsed.Milliseconds(),
				batchMetrics.LoadRuntimeElapsed.Milliseconds(),
				batchMetrics.CompileElapsed.Milliseconds(),
				batchMetrics.HoldLeaseElapsed.Milliseconds(),
				batchMetrics.PlaybackElapsed.Milliseconds(),
				batchMetrics.ScheduleElapsed.Milliseconds(),
				batchMetrics.TotalElapsed.Milliseconds(),
			)
		}
		s.battleMetrics.AddBatch(battleBatchMetricSample{
			Elapsed:       batchElapsed,
			ClaimElapsed:  claimElapsed,
			ExecElapsed:   execElapsed,
			LeaseUtilPct:  float64(batchElapsed) * 100 / float64(battleRoomLeaseDuration),
			Rooms:         len(rooms),
			CompiledRuns:  batchMetrics.CompiledRuns,
			CompiledTurns: batchMetrics.CompiledTurns,
		})
		return battleLoopMinSleep, execPerRoomEWMA
	}
	nextTickAt, hasNext, err := s.deps.Store.NextBattleRoomTickAt(claimCtx, statuses)
	if err != nil {
		log.Printf("battle loop peek next tick failed worker=%s err=%v", workerID, err)
		return battleLoopIdleSleep, execPerRoomEWMA
	}
	if !hasNext {
		return battleLoopIdleSleep, execPerRoomEWMA
	}
	wait := time.Until(nextTickAt)
	if wait < battleLoopMinSleep {
		return battleLoopMinSleep, execPerRoomEWMA
	}
	if wait > battleLoopMaxSleep {
		return battleLoopMaxSleep, execPerRoomEWMA
	}
	return wait, execPerRoomEWMA
}

func battleTickShardWorkerCount(roomCount int, configuredMax int) int {
	if roomCount <= 0 {
		return 1
	}
	workers := configuredMax
	if workers <= 0 {
		workers = 8
	}
	if workers > roomCount {
		workers = roomCount
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

func expectedBattleRunStartGapMs(startPath string, interBattleIntervalMs, recoveryDurationMs int64) int64 {
	switch normalizeBattleCompileSource(startPath) {
	case battleCompileSourceRecoveryAutoReenter:
		return maxInt64(0, recoveryDurationMs)
	case battleCompileSourceReadyDispatch, battleCompileSourceEnsureRun:
		return maxInt64(0, interBattleIntervalMs)
	default:
		return 0
	}
}

func battleRunStartLateMs(gapSincePrevFinishMs, expectedGapMs int64) int64 {
	if gapSincePrevFinishMs < 0 {
		return -1
	}
	return maxInt64(0, gapSincePrevFinishMs-maxInt64(0, expectedGapMs))
}

func shouldLogBattleRunStart(gapLateMs int64, waitSinceReadyMs int64) bool {
	return gapLateMs >= battleRunStartSlowLogThresholdMs || waitSinceReadyMs >= battleRunStartSlowLogThresholdMs
}

func battleTickClaimRoomLimit(policy battleTickClaimPolicy, leaseDuration, execPerRoomEWMA time.Duration) int {
	minRooms := policy.MinRooms
	if minRooms <= 0 {
		minRooms = 1
	}
	maxRooms := policy.MaxRooms
	if maxRooms < minRooms {
		maxRooms = minRooms
	}
	targetUtilPct := policy.TargetLeaseUtilPct
	if targetUtilPct <= 0 || targetUtilPct >= 100 {
		targetUtilPct = 60
	}
	if execPerRoomEWMA <= 0 || leaseDuration <= 0 {
		return maxRooms
	}
	targetBudget := time.Duration(int64(leaseDuration) * int64(targetUtilPct) / 100)
	if targetBudget <= 0 {
		return minRooms
	}
	limit := int(targetBudget / execPerRoomEWMA)
	if limit < minRooms {
		return minRooms
	}
	if limit > maxRooms {
		return maxRooms
	}
	return limit
}

func battleTickClaimLookahead(execPerRoomEWMA time.Duration, claimLimit int, leaseDuration time.Duration) time.Duration {
	if execPerRoomEWMA <= 0 || claimLimit <= 0 {
		return 0
	}
	predictedBatch := execPerRoomEWMA * time.Duration(claimLimit)
	lookahead := predictedBatch / 2
	if lookahead < 100*time.Millisecond {
		lookahead = 100 * time.Millisecond
	}
	maxLookahead := leaseDuration / 2
	if maxLookahead <= 0 {
		maxLookahead = 1200 * time.Millisecond
	}
	if lookahead > maxLookahead {
		lookahead = maxLookahead
	}
	return lookahead
}

func battleTickUpdateExecPerRoomEWMA(prevEWMA, execElapsed time.Duration, rooms int) time.Duration {
	if rooms <= 0 || execElapsed <= 0 {
		return prevEWMA
	}
	sample := execElapsed / time.Duration(rooms)
	if sample <= 0 {
		return prevEWMA
	}
	if prevEWMA <= 0 {
		return sample
	}
	const alphaNum int64 = 3
	const alphaDen int64 = 10
	return time.Duration((int64(prevEWMA)*(alphaDen-alphaNum) + int64(sample)*alphaNum) / alphaDen)
}

func (s *Server) processClaimedBattleRoomTick(workerID string, room store.BattleRoom) (battleTickRoomMetrics, error) {
	metrics := battleTickRoomMetrics{}
	tickStartedAt := time.Now()
	roomNow := time.Now()
	roomCtx, roomCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer roomCancel()
	nextTickAt := roomNow.Add(battleLoopMinSleep)
	switch room.Status {
	case store.BattleRoomStatusEntering:
		targetTick, tickErr := s.tryStartPreparingRoom(roomCtx, room, roomNow)
		if tickErr != nil {
			return metrics, tickErr
		}
		if !targetTick.IsZero() {
			nextTickAt = targetTick
		}
	case store.BattleRoomStatusRecovering:
		targetTick, tickErr := s.tryAdvanceRecoveringRoom(roomCtx, room, roomNow)
		if tickErr != nil {
			return metrics, tickErr
		}
		if !targetTick.IsZero() {
			nextTickAt = targetTick
		}
	case store.BattleRoomStatusPlaying:
		tickMetrics, targetTick, asyncPlayback, tickErr := s.processBattleRoomTick(roomCtx, workerID, room, roomNow)
		if tickErr != nil {
			return metrics, tickErr
		}
		metrics = tickMetrics
		if asyncPlayback {
			metrics.AsyncPlayback = true
			metrics.TotalElapsed = time.Since(tickStartedAt)
			s.logSlowBattleRoomTick(workerID, room, metrics, time.Time{})
			return metrics, nil
		}
		if !targetTick.IsZero() {
			nextTickAt = targetTick
		}
	}
	scheduleStartedAt := time.Now()
	if err := s.deps.Store.ScheduleBattleRoomNextTick(roomCtx, room.ID, workerID, nextTickAt, roomNow.Add(battleRoomLeaseDuration)); err != nil {
		return metrics, err
	}
	metrics.ScheduleElapsed = time.Since(scheduleStartedAt)
	metrics.TotalElapsed = time.Since(tickStartedAt)
	s.logSlowBattleRoomTick(workerID, room, metrics, nextTickAt)
	s.enqueueBattleDueRoom(room.ID, nextTickAt)
	return metrics, nil
}

func (s *Server) logSlowBattleRoomTick(workerID string, room store.BattleRoom, metrics battleTickRoomMetrics, nextTickAt time.Time) {
	if metrics.TotalElapsed < battleTickRoomSlowLogThreshold &&
		metrics.LoadDetailElapsed < battleTickRoomSlowLogThreshold &&
		metrics.EnsureRunElapsed < battleTickRoomSlowLogThreshold &&
		metrics.LoadRuntimeElapsed < battleTickRoomSlowLogThreshold &&
		metrics.CompileElapsed < battleTickRoomSlowLogThreshold &&
		metrics.HoldLeaseElapsed < battleTickRoomSlowLogThreshold &&
		metrics.PlaybackElapsed < battleTickRoomSlowLogThreshold &&
		metrics.ScheduleElapsed < battleTickRoomSlowLogThreshold {
		return
	}
	nextTickMs := int64(0)
	if !nextTickAt.IsZero() {
		nextTickMs = nextTickAt.UnixMilli()
	}
	log.Printf(
		"battle room tick stages room_id=%d worker=%s status=%s run_id=%d async_playback=%t next_tick_at_ms=%d total_ms=%d load_detail_ms=%d ensure_run_ms=%d load_runtime_ms=%d compile_ms=%d hold_lease_ms=%d playback_ms=%d schedule_ms=%d compiled_runs=%d compiled_turns=%d",
		room.ID,
		workerID,
		room.Status,
		battleRoomCurrentRunIDValue(room),
		metrics.AsyncPlayback,
		nextTickMs,
		metrics.TotalElapsed.Milliseconds(),
		metrics.LoadDetailElapsed.Milliseconds(),
		metrics.EnsureRunElapsed.Milliseconds(),
		metrics.LoadRuntimeElapsed.Milliseconds(),
		metrics.CompileElapsed.Milliseconds(),
		metrics.HoldLeaseElapsed.Milliseconds(),
		metrics.PlaybackElapsed.Milliseconds(),
		metrics.ScheduleElapsed.Milliseconds(),
		metrics.CompiledRuns,
		metrics.CompiledTurns,
	)
}

func battleRoomCurrentRunIDValue(room store.BattleRoom) int64 {
	if room.CurrentRunID == nil {
		return 0
	}
	return *room.CurrentRunID
}

func (s *Server) tryStartPreparingRoom(ctx context.Context, room store.BattleRoom, now time.Time) (time.Time, error) {
	world, ok := s.battleWorldConfigByID(room.DungeonID)
	if !ok {
		return now.Add(2 * time.Second), nil
	}

	entryStartedAt := room.UpdatedAt
	if room.EnteringStartedAt != nil && !room.EnteringStartedAt.IsZero() {
		entryStartedAt = *room.EnteringStartedAt
	}
	waitUntil := entryStartedAt.Add(time.Duration(world.EntryDurationSeconds) * time.Second)
	if now.Before(waitUntil) {
		remaining := waitUntil.Sub(now)
		if remaining >= 2*time.Second {
			log.Printf("battle entering wait room_id=%d remaining_ms=%d entry_duration_s=%d entering_started_at_ms=%d", room.ID, remaining.Milliseconds(), world.EntryDurationSeconds, entryStartedAt.UnixMilli())
		}
		return waitUntil, nil
	}
	fogItemID := ""
	if room.IsFogMode {
		fogItemID = world.FogMapItemID
	}
	detail, seed, fogDisabled, err := s.deps.Store.StartBattleRoomFight(ctx, room.ID, world.EntryCosts, fogItemID, now)
	if err != nil {
		if errors.Is(err, store.ErrInsufficientItem) {
			s.pushBattleRoomEvent(room.ID, "battle.room.entry_cost_failed", fiber.Map{"roomId": room.ID}, ctx)
		}
		return now.Add(2 * time.Second), nil
	}
	if fogDisabled {
		s.pushFogModeDisabledEvent(detail, ctx)
	} else if detail.Room.IsFogMode {
		_ = s.writeFogMemberSnapshots(ctx, detail)
	}
	// Push inventory update to owner whenever a fog item was consumed or fog mode changed.
	if fogItemID != "" {
		s.launchBattleTaskOrRun(func() { s.pushInventoryInstances(detail.Room.OwnerID) })
	}
	s.pushBattleRoomUpdated(detail, ctx)
	s.handleBattleRunStarted(detail, seed, now, battleCompileSourcePrepare, battleCompileSourcePrepare, false, ctx)
	return now, nil
}

func (s *Server) tryAdvanceRecoveringRoom(ctx context.Context, room store.BattleRoom, now time.Time) (time.Time, error) {
	detail, transitioned, err := s.deps.Store.AdvanceBattleRoomRecovery(ctx, room.ID, now)
	if err != nil {
		return now.Add(2 * time.Second), err
	}
	if !transitioned {
		if detail.Room.RecoveryEndsAt != nil && !detail.Room.RecoveryEndsAt.IsZero() {
			remaining := detail.Room.RecoveryEndsAt.Sub(now)
			if remaining >= 2*time.Second {
				log.Printf("battle recovering wait room_id=%d remaining_ms=%d recovery_ends_at_ms=%d", room.ID, remaining.Milliseconds(), detail.Room.RecoveryEndsAt.UnixMilli())
			}
			return *detail.Room.RecoveryEndsAt, nil
		}
		return now.Add(200 * time.Millisecond), nil
	}
	s.pushBattleRoomUpdated(detail, ctx)
	if detail.Room.Status == store.BattleRoomStatusEntering {
		world, ok := s.battleWorldConfigByID(detail.Room.DungeonID)
		if ok {
			reenterFogItemID := ""
			if detail.Room.IsFogMode {
				reenterFogItemID = world.FogMapItemID
			}
			startedDetail, seed, reenterFogDisabled, startErr := s.deps.Store.StartBattleRoomFight(ctx, detail.Room.ID, world.EntryCosts, reenterFogItemID, now)
			if startErr == nil {
				if reenterFogDisabled {
					s.pushFogModeDisabledEvent(startedDetail, ctx)
				} else if startedDetail.Room.IsFogMode {
					_ = s.writeFogMemberSnapshots(ctx, startedDetail)
				}
				if reenterFogItemID != "" {
					s.launchBattleTaskOrRun(func() { s.pushInventoryInstances(startedDetail.Room.OwnerID) })
				}
				s.pushBattleRoomUpdated(startedDetail, ctx)
				s.handleBattleRunStarted(startedDetail, seed, now, battleCompileSourceRecoveryAutoReenter, battleCompileSourceRecoveryAutoReenter, false, ctx)
				return now, nil
			}
			if errors.Is(startErr, store.ErrInsufficientItem) {
				s.pushBattleRoomEvent(detail.Room.ID, "battle.room.entry_cost_failed", fiber.Map{"roomId": detail.Room.ID}, ctx)
			}
		}
		s.pushBattleRoomEventByMembers(detail.Members, "battle.room.recovered", fiber.Map{
			"roomId": detail.Room.ID,
			"status": detail.Room.Status,
		}, ctx)
	}
	return now, nil
}

func battleWorkerRuntimeID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}

func (s *Server) publishBattleWorkerHeartbeat(workerID string, now time.Time) {
	if strings.TrimSpace(workerID) == "" || s.deps.Redis == nil {
		return
	}
	ttl := time.Duration(s.deps.Config.GlobalSettings.Battle.WorkerHeartbeat.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	payload, err := json.Marshal(fiber.Map{
		"workerId":       workerID,
		"ts":             now.UnixMilli(),
		"refreshedAt":    now.UTC().Format(time.RFC3339Nano),
		"specialization": s.specializationRuntimeVersionPayload(),
	})
	if err != nil {
		return
	}
	key := s.battleWorkerHeartbeatKey(workerID)
	if err := s.deps.Redis.Set(context.Background(), key, string(payload), ttl).Err(); err != nil {
		log.Printf("battle worker heartbeat failed worker=%s err=%v", workerID, err)
	}
}

func (s *Server) battleWorkerHeartbeatKey(workerID string) string {
	prefix := strings.TrimSpace(s.deps.Config.MQTopicPrefix)
	if prefix == "" {
		prefix = "game"
	}
	return fmt.Sprintf("%s:battle:worker:heartbeat:%s", prefix, strings.TrimSpace(workerID))
}

func (s *Server) battleWorkerHeartbeatPattern() string {
	prefix := strings.TrimSpace(s.deps.Config.MQTopicPrefix)
	if prefix == "" {
		prefix = "game"
	}
	return fmt.Sprintf("%s:battle:worker:heartbeat:*", prefix)
}

func (s *Server) battleDueQueueKey() string {
	prefix := strings.TrimSpace(s.deps.Config.MQTopicPrefix)
	if prefix == "" {
		prefix = "game"
	}
	return fmt.Sprintf("%s:battle:due_rooms", prefix)
}

func (s *Server) battlePlaybackActiveRoomKey(roomID int64) string {
	prefix := strings.TrimSpace(s.deps.Config.MQTopicPrefix)
	if prefix == "" {
		prefix = "game"
	}
	return fmt.Sprintf("%s:battle:playback:active:%d", prefix, roomID)
}

func (s *Server) battlePlaybackActiveRoomsPattern() string {
	prefix := strings.TrimSpace(s.deps.Config.MQTopicPrefix)
	if prefix == "" {
		prefix = "game"
	}
	return fmt.Sprintf("%s:battle:playback:active:*", prefix)
}

// addPlaybackActiveRoom tracks a room with an active playback worker in Redis.
// Uses per-room key with TTL so stale entries auto-expire if SREM fails.
// This is used by tickBattleRooms to filter out playback rooms from the due queue,
// avoiding expensive full-table-scan fallback claims.
func (s *Server) addPlaybackActiveRoom(roomID int64) {
	if s.deps.Redis == nil || roomID <= 0 {
		return
	}
	// TTL 30s — safety net for SREM failures. Playback lease is 4s, so 30s is generous.
	if err := s.deps.Redis.Set(context.Background(), s.battlePlaybackActiveRoomKey(roomID), "1", 30*time.Second).Err(); err != nil {
		log.Printf("battle playback active room set failed room_id=%d err=%v", roomID, err)
	}
}

// removePlaybackActiveRoom removes a room from the active playback tracking.
func (s *Server) removePlaybackActiveRoom(roomID int64) {
	if s.deps.Redis == nil || roomID <= 0 {
		return
	}
	if err := s.deps.Redis.Del(context.Background(), s.battlePlaybackActiveRoomKey(roomID)).Err(); err != nil {
		log.Printf("battle playback active room del failed room_id=%d err=%v", roomID, err)
	}
}

// getPlaybackActiveRoomIDs returns all room IDs with active playback workers.
// Uses SCAN to find all matching keys (per-room keys with pattern).
func (s *Server) getPlaybackActiveRoomIDs(ctx context.Context) map[int64]struct{} {
	result := make(map[int64]struct{})
	if s.deps.Redis == nil {
		return result
	}
	pattern := s.battlePlaybackActiveRoomsPattern()
	var cursor uint64
	for {
		keys, nextCursor, err := s.deps.Redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			log.Printf("battle playback active rooms scan failed err=%v", err)
			return result
		}
		for _, key := range keys {
			// Extract roomID from key: {prefix}:battle:playback:active:{roomID}
			parts := strings.Split(key, ":")
			if len(parts) > 0 {
				roomIDStr := parts[len(parts)-1]
				if id, err := strconv.ParseInt(roomIDStr, 10, 64); err == nil && id > 0 {
					result[id] = struct{}{}
				}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return result
}

// filterPlaybackRoomsFromDueIDs removes room IDs that have active playback workers.
// Playback rooms don't need state-advance processing; filtering them avoids the
// expensive full-table-scan fallback when the due queue is dominated by playback rooms.
func (s *Server) filterPlaybackRoomsFromDueIDs(ctx context.Context, dueRoomIDs []int64) []int64 {
	if len(dueRoomIDs) == 0 {
		return dueRoomIDs
	}
	if s.deps.Redis == nil {
		return dueRoomIDs
	}
	keys := make([]string, 0, len(dueRoomIDs))
	for _, id := range dueRoomIDs {
		if id <= 0 {
			continue
		}
		keys = append(keys, s.battlePlaybackActiveRoomKey(id))
	}
	if len(keys) == 0 {
		return dueRoomIDs
	}
	values, err := s.deps.Redis.MGet(ctx, keys...).Result()
	if err != nil {
		log.Printf("battle playback active rooms mget failed due_ids=%d err=%v", len(dueRoomIDs), err)
		return dueRoomIDs
	}
	filtered := make([]int64, 0, len(dueRoomIDs))
	keyIndex := 0
	for _, id := range dueRoomIDs {
		if id <= 0 {
			filtered = append(filtered, id)
			continue
		}
		if keyIndex >= len(values) || values[keyIndex] == nil {
			filtered = append(filtered, id)
		}
		keyIndex++
	}
	return filtered
}

func (s *Server) enqueueBattleDueRoom(roomID int64, nextTickAt time.Time) {
	if s.deps.Redis == nil || roomID <= 0 {
		return
	}
	scoreAt := nextTickAt
	if scoreAt.IsZero() {
		scoreAt = time.Now()
	}
	if err := s.deps.Redis.ZAdd(context.Background(), s.battleDueQueueKey(), redis.Z{
		Score:  float64(scoreAt.UnixMilli()),
		Member: strconv.FormatInt(roomID, 10),
	}).Err(); err != nil {
		log.Printf("battle due enqueue failed room_id=%d err=%v", roomID, err)
	}
}

func battleRoomUsesLegacyDueQueue(room store.BattleRoom) bool {
	return room.SchemaVersion != BattleRoomSchemaV2
}

func (s *Server) enqueueBattleDueRoomForRoom(room store.BattleRoom, nextTickAt time.Time) {
	if !battleRoomUsesLegacyDueQueue(room) {
		return
	}
	s.enqueueBattleDueRoom(room.ID, nextTickAt)
}

func (s *Server) popBattleDueRoomIDs(ctx context.Context, now time.Time, limit int) []int64 {
	if s.deps.Redis == nil || limit <= 0 {
		return nil
	}
	raw, err := battleDuePopScript.Run(
		ctx,
		s.deps.Redis,
		[]string{s.battleDueQueueKey()},
		now.UnixMilli(),
		limit,
	).Result()
	if err != nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]int64, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text == "" {
			continue
		}
		roomID, parseErr := strconv.ParseInt(text, 10, 64)
		if parseErr != nil || roomID <= 0 {
			continue
		}
		out = append(out, roomID)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
