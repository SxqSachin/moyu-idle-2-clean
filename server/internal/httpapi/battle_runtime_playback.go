package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

const battlePlaybackWorkerStuckRetryThreshold = 3
const battlePlaybackWorkerStuckFinalizeThreshold = 6
const battlePlaybackMaxFutureSleep = 5 * time.Second
const battlePlaybackPublishRetryDelay = 2 * time.Second
const battlePlaybackTickOperationTimeout = 3 * time.Second
const battlePlaybackLeaseRenewRetryDelay = 200 * time.Millisecond

var (
	errBattlePlaybackPublishUnavailable = errors.New("battle playback publish unavailable")
	errBattlePlaybackLeaseLost          = errors.New("battle playback lease lost")
)

type battlePlaybackWorkerReservation struct {
	RoomID     int64
	RunID      int64
	Generation uint64
	OwnerToken string
}

type battlePlaybackTickResult struct {
	NextEventIdx  int
	LastPushedSeq int64
	TotalEvents   int
}

func (result battlePlaybackTickResult) progressedFrom(previous battlePlaybackWorkerState) bool {
	return result.NextEventIdx != previous.LastNextEventIdx || result.LastPushedSeq != previous.LastPushedSeq
}

func (s *Server) playbackBattleRunTick(ctx context.Context, detail store.BattleRoomDetail, run store.BattleRun, now time.Time) (time.Time, battlePlaybackTickResult, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return time.Time{}, battlePlaybackTickResult{}, err
		}
	}
	timeline, err := s.loadBattleRunPlaybackCache(ctx, run.ID)
	if err != nil {
		return time.Time{}, battlePlaybackTickResult{}, err
	}
	result := battlePlaybackTickResult{TotalEvents: len(timeline)}
	if len(timeline) == 0 {
		expectedTotalEvents := int(toInt64Loose(run.State["totalEvents"], 0))
		expectedFinalSeq := toInt64Loose(run.State["finalSeq"], 0)
		if expectedTotalEvents > 0 || expectedFinalSeq > 0 {
			return time.Time{}, result, fmt.Errorf(
				"battle playback payload missing room_id=%d run_id=%d expected_total_events=%d expected_final_seq=%d",
				detail.Room.ID,
				run.ID,
				expectedTotalEvents,
				expectedFinalSeq,
			)
		}
		if err := s.finishBattleRunPlayback(ctx, detail, run, timeline, now); err != nil {
			return time.Time{}, result, err
		}
		return now.Add(battleLoopMinSleep), result, errBattlePlaybackFinished
	}
	interval := time.Duration(s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs) * time.Millisecond
	if fixed := s.deps.Config.GlobalSettings.Battle.PlaybackFixedIntervalMs; fixed > 0 {
		interval = time.Duration(fixed) * time.Millisecond
	}
	if interval <= 0 {
		interval = time.Second
	}
	intervalMs := interval.Milliseconds()
	checkpoint := s.loadBattleRunPlaybackCheckpoint(run.ID, run.State, now)
	checkpoint = s.rebuildBattlePlaybackCheckpoint(run.ID, checkpoint, run.State, timeline, now)
	nowMs := now.UnixMilli()
	targetUsers := battleRoomMemberUserIDs(detail.Members)
	if !checkpoint.SnapshotPushed {
		checkpoint.Stage = "playback_streaming"
		if checkpoint.PlaybackStartedAtMs <= 0 {
			checkpoint.PlaybackStartedAtMs = nowMs
		}
		published := s.pushBattleRoomEventToUsers(targetUsers, "battle.run.batch", toBattleEventBatchWirePayload(detail.Room.ID, wsbus.BattleEventBatchPayload{
			RunID:         run.ID,
			FromSeq:       0,
			ToSeq:         0,
			WindowStartMS: checkpoint.PlaybackStartedAtMs,
			WindowEndMS:   checkpoint.PlaybackStartedAtMs,
			Events:        []wsbus.BattleTimelineEvent{},
			Snapshot:      checkpoint.snapshot(detail.Room.ID),
		}), ctx)
		if !published {
			return time.Time{}, result, fmt.Errorf("%w: room_id=%d run_id=%d snapshot=true", errBattlePlaybackPublishUnavailable, detail.Room.ID, run.ID)
		}
		compileStartedAtMs := checkpoint.CompileStartedAtMs
		compileFinishedAtMs := checkpoint.CompileFinishedAtMs
		if compileFinishedAtMs <= 0 {
			compileFinishedAtMs = checkpoint.PlaybackStartedAtMs
		}
		runStartedAtMs := run.StartedAt.UnixMilli()
		runStartToFirstPushMs := maxInt64(0, nowMs-runStartedAtMs)
		if runStartToFirstPushMs >= battleRunFirstPushSlowLogThresholdMs {
			log.Printf(
				"battle first batch snapshot slow room_id=%d run_id=%d sent_at_ms=%d playback_started_at_ms=%d first_batch_delay_ms=%d compile_started_at_ms=%d compile_finished_at_ms=%d compile_to_first_push_ms=%d run_started_at_ms=%d run_start_to_first_push_ms=%d",
				detail.Room.ID,
				run.ID,
				nowMs,
				checkpoint.PlaybackStartedAtMs,
				maxInt64(0, nowMs-checkpoint.PlaybackStartedAtMs),
				compileStartedAtMs,
				compileFinishedAtMs,
				maxInt64(0, nowMs-compileFinishedAtMs),
				runStartedAtMs,
				runStartToFirstPushMs,
			)
		}
		checkpoint.SnapshotPushed = true
		checkpoint.LastDispatchAtMs = nowMs
		firstDelayMs := battlePlaybackFirstEventDelay.Milliseconds()
		if firstDelayMs <= 0 || firstDelayMs > intervalMs {
			firstDelayMs = intervalMs
		}
		checkpoint.NextPushAtMs = nowMs + firstDelayMs
		s.setBattleRunPlaybackCheckpoint(run.ID, checkpoint)
		result.NextEventIdx = checkpoint.NextEventIdx
		result.LastPushedSeq = checkpoint.LastPushedSeq
		return s.normalizeBattlePlaybackNextTickAt(detail.Room.ID, run.ID, now, checkpoint.NextPushAtMs), result, nil
	}
	if checkpoint.NextPushAtMs <= 0 {
		checkpoint.NextPushAtMs = nowMs + intervalMs
		s.setBattleRunPlaybackCheckpoint(run.ID, checkpoint)
		result.NextEventIdx = checkpoint.NextEventIdx
		result.LastPushedSeq = checkpoint.LastPushedSeq
		return s.normalizeBattlePlaybackNextTickAt(detail.Room.ID, run.ID, now, checkpoint.NextPushAtMs), result, nil
	}
	if nowMs < checkpoint.NextPushAtMs {
		result.NextEventIdx = checkpoint.NextEventIdx
		result.LastPushedSeq = checkpoint.LastPushedSeq
		return s.normalizeBattlePlaybackNextTickAt(detail.Room.ID, run.ID, now, checkpoint.NextPushAtMs), result, nil
	}
	checkpoint.ensureTimelineCursor(timeline)
	if playbackCheckpointAtTimelineEnd(checkpoint, timeline) {
		if err := s.finishBattleRunPlayback(ctx, detail, run, timeline, now); err != nil {
			return time.Time{}, result, err
		}
		result.NextEventIdx = checkpoint.NextEventIdx
		result.LastPushedSeq = checkpoint.LastPushedSeq
		return now.Add(battleLoopMinSleep), result, errBattlePlaybackFinished
	}
	maxEventsBatch := s.deps.Config.GlobalSettings.Battle.PlaybackMaxEventsBatch
	nextEvents, nextEventIdx, hasOffsetRemainder := collectPlaybackEventsAtCurrentOffset(timeline, checkpoint.NextEventIdx, maxEventsBatch)
	if len(nextEvents) == 0 {
		checkpoint.NextEventIdx = nextEventIdx
		checkpoint.NextPushAtMs = nowMs + intervalMs
		s.setBattleRunPlaybackCheckpoint(run.ID, checkpoint)
		result.NextEventIdx = checkpoint.NextEventIdx
		result.LastPushedSeq = checkpoint.LastPushedSeq
		return s.normalizeBattlePlaybackNextTickAt(detail.Room.ID, run.ID, now, checkpoint.NextPushAtMs), result, nil
	}
	fromSeq := nextEvents[0].Seq
	toSeq := nextEvents[len(nextEvents)-1].Seq
	prevOffset := checkpoint.LastPushedOffsetMs
	scheduledPushAtMs := checkpoint.NextPushAtMs
	dispatchIntervalMs := int64(0)
	if checkpoint.LastDispatchAtMs > 0 {
		dispatchIntervalMs = nowMs - checkpoint.LastDispatchAtMs
	}
	nextPushedOffsetMs := nextEvents[len(nextEvents)-1].OffsetMS
	nextDelayMs := playbackNextDelayMs(intervalMs, s.deps.Config.GlobalSettings.Battle.PlaybackMaxGapMs, timeline, nextEventIdx, nextPushedOffsetMs)
	windowStartOffset := maxInt64(0, prevOffset)
	windowEndOffset := maxInt64(0, nextPushedOffsetMs)
	offsetGapMs := int64(0)
	if prevOffset < 0 {
		offsetGapMs = maxInt64(0, nextEvents[0].OffsetMS)
	} else {
		offsetGapMs = maxInt64(0, nextEvents[0].OffsetMS-prevOffset)
	}
	scheduleLatenessMs := maxInt64(0, nowMs-scheduledPushAtMs)
	batchPayload := wsbus.BattleEventBatchPayload{
		RunID:         run.ID,
		FromSeq:       fromSeq,
		ToSeq:         toSeq,
		WindowStartMS: checkpoint.PlaybackStartedAtMs + windowStartOffset,
		WindowEndMS:   checkpoint.PlaybackStartedAtMs + windowEndOffset,
		Events:        nextEvents,
	}
	if !s.pushBattleRoomEventToUsers(targetUsers, "battle.run.batch", toBattleEventBatchWirePayload(detail.Room.ID, batchPayload), ctx) {
		return time.Time{}, result, fmt.Errorf(
			"%w: room_id=%d run_id=%d from_seq=%d to_seq=%d",
			errBattlePlaybackPublishUnavailable,
			detail.Room.ID,
			run.ID,
			fromSeq,
			toSeq,
		)
	}
	checkpoint.NextEventIdx = nextEventIdx
	checkpoint.LastPushedSeq = toSeq
	checkpoint.LastPushedOffsetMs = nextPushedOffsetMs
	checkpoint.LastDispatchAtMs = nowMs
	applyPlaybackEventsToCheckpoint(&checkpoint, nextEvents)
	checkpoint.NextPushAtMs = nowMs + nextDelayMs
	if shouldSampleBattlePlaybackCadenceLog(run.ID, now) &&
		(dispatchIntervalMs > intervalMs*3/2 || scheduleLatenessMs > intervalMs/2) {
		log.Printf(
			"battle playback cadence delayed room_id=%d run_id=%d dispatch_interval_ms=%d base_interval_ms=%d schedule_lateness_ms=%d offset_gap_ms=%d next_delay_ms=%d events=%d next_event_idx=%d total_events=%d has_offset_remainder=%t",
			detail.Room.ID,
			run.ID,
			dispatchIntervalMs,
			intervalMs,
			scheduleLatenessMs,
			offsetGapMs,
			nextDelayMs,
			len(nextEvents),
			checkpoint.NextEventIdx,
			len(timeline),
			hasOffsetRemainder,
		)
	}
	s.battleMetrics.AddPlaybackDispatch(battlePlaybackDispatchMetricSample{
		DispatchInterval: time.Duration(maxInt64(0, dispatchIntervalMs)) * time.Millisecond,
		BaseInterval:     time.Duration(maxInt64(0, intervalMs)) * time.Millisecond,
		OffsetGap:        time.Duration(offsetGapMs) * time.Millisecond,
		BatchSpan:        time.Duration(maxInt64(0, windowEndOffset-windowStartOffset)) * time.Millisecond,
		NextDelay:        time.Duration(maxInt64(0, nextDelayMs)) * time.Millisecond,
		Events:           len(nextEvents),
	})
	s.setBattleRunPlaybackCheckpoint(run.ID, checkpoint)
	result.NextEventIdx = checkpoint.NextEventIdx
	result.LastPushedSeq = checkpoint.LastPushedSeq
	if checkpoint.NextEventIdx < len(timeline) {
		return s.normalizeBattlePlaybackNextTickAt(detail.Room.ID, run.ID, now, checkpoint.NextPushAtMs), result, nil
	}
	checkpoint.Stage = "playback_finished"
	checkpoint.NextPushAtMs = 0
	checkpoint.TotalEvents = len(timeline)
	checkpoint.FinalSeq = timeline[len(timeline)-1].Seq
	s.setBattleRunPlaybackCheckpoint(run.ID, checkpoint)
	if err := s.finishBattleRunPlayback(ctx, detail, run, timeline, now); err != nil {
		return time.Time{}, result, err
	}
	result.NextEventIdx = checkpoint.NextEventIdx
	result.LastPushedSeq = checkpoint.LastPushedSeq
	return now.Add(battleLoopMinSleep), result, errBattlePlaybackFinished
}

func (s *Server) newBattlePlaybackWorkerReservation(roomID int64, runID int64, workerID string) (battlePlaybackWorkerReservation, bool) {
	workerID = strings.TrimSpace(workerID)
	if roomID <= 0 || runID <= 0 || workerID == "" {
		return battlePlaybackWorkerReservation{}, false
	}
	s.battlePlaybackRunMu.Lock()
	s.battlePlaybackRunSeq++
	if s.battlePlaybackRunSeq == 0 {
		s.battlePlaybackRunSeq++
	}
	generation := s.battlePlaybackRunSeq
	s.battlePlaybackRunMu.Unlock()
	return battlePlaybackWorkerReservation{
		RoomID:     roomID,
		RunID:      runID,
		Generation: generation,
		OwnerToken: fmt.Sprintf("%s:playback:%d:%d", workerID, runID, generation),
	}, true
}

func (s *Server) activateBattlePlaybackWorkerForRun(detail store.BattleRoomDetail, reservation battlePlaybackWorkerReservation, now time.Time) bool {
	if reservation.RoomID != detail.Room.ID || reservation.RunID <= 0 || reservation.Generation == 0 || strings.TrimSpace(reservation.OwnerToken) == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.battlePlaybackRunMu.Lock()
	if s.battlePlaybackRuns == nil {
		s.battlePlaybackRuns = make(map[int64]battlePlaybackWorkerState)
	}
	existing, exists := s.battlePlaybackRuns[reservation.RunID]
	if exists && existing.Generation >= reservation.Generation {
		s.battlePlaybackRunMu.Unlock()
		return existing.Generation == reservation.Generation && existing.OwnerToken == reservation.OwnerToken
	}
	restartCount := int64(0)
	if exists {
		restartCount = existing.RestartCount + 1
	}
	s.battlePlaybackRuns[reservation.RunID] = battlePlaybackWorkerState{
		RoomID:         reservation.RoomID,
		RunID:          reservation.RunID,
		Generation:     reservation.Generation,
		OwnerToken:     reservation.OwnerToken,
		StartedAt:      now,
		LastTickAt:     now,
		LastProgressAt: now,
		LastNextTickAt: now,
		RestartCount:   restartCount,
	}
	s.battlePlaybackRunMu.Unlock()
	if exists {
		log.Printf("battle playback worker fenced room_id=%d run_id=%d old_generation=%d generation=%d restart_count=%d", reservation.RoomID, reservation.RunID, existing.Generation, reservation.Generation, restartCount)
		s.battleMetrics.AddPlaybackWorkerRestart()
	}
	if battleRoomUsesLegacyDueQueue(detail.Room) {
		// V1 uses Redis due-room filtering; V2 dispatches playback by DB phase.
		s.addPlaybackActiveRoom(reservation.RoomID)
	}
	return true
}

func (s *Server) releaseBattlePlaybackWorkerForRun(detail store.BattleRoomDetail, reservation battlePlaybackWorkerReservation) {
	if !s.releaseBattlePlaybackWorker(reservation) {
		return
	}
	if battleRoomUsesLegacyDueQueue(detail.Room) {
		s.removePlaybackActiveRoom(detail.Room.ID)
	}
}

func (s *Server) startReservedBattlePlaybackWorker(detail store.BattleRoomDetail, run store.BattleRun, reservation battlePlaybackWorkerReservation, nextTickAt time.Time, leaseUntil time.Time) {
	s.launchBattleTaskOrRun(func() { s.runBattlePlaybackWorker(detail, run, reservation, nextTickAt, leaseUntil) })
}

func battlePlaybackLeaseNeedsRenewal(now time.Time, nextTickAt time.Time, leaseUntil time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	if leaseUntil.IsZero() {
		return true
	}
	wakeAt := nextTickAt
	if wakeAt.IsZero() || wakeAt.Before(now) {
		wakeAt = now
	}
	return !leaseUntil.After(wakeAt.Add(battlePlaybackLeaseRenewThreshold))
}

func battlePlaybackLeaseCoversTick(ctx context.Context, now time.Time, leaseUntil time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	tickDeadline := now.Add(battlePlaybackTickOperationTimeout)
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			tickDeadline = deadline
		}
	}
	return leaseUntil.After(tickDeadline)
}

func battlePlaybackWorkerWakeAt(nextTickAt time.Time, leaseUntil time.Time, renewRetryNotBefore time.Time) time.Time {
	wakeAt := nextTickAt
	if leaseUntil.IsZero() {
		if !renewRetryNotBefore.IsZero() && (wakeAt.IsZero() || wakeAt.Before(renewRetryNotBefore)) {
			return renewRetryNotBefore
		}
		return wakeAt
	}
	renewAt := leaseUntil.Add(-battlePlaybackLeaseRenewThreshold)
	if wakeAt.IsZero() || renewAt.Before(wakeAt) {
		wakeAt = renewAt
	}
	if !renewRetryNotBefore.IsZero() && wakeAt.Before(renewRetryNotBefore) {
		wakeAt = renewRetryNotBefore
	}
	return wakeAt
}

func (s *Server) renewBattlePlaybackLease(ctx context.Context, reservation battlePlaybackWorkerReservation, now time.Time) (time.Time, error) {
	if now.IsZero() {
		now = time.Now()
	}
	nextLeaseUntil := now.Add(battlePlaybackLeaseTTL)
	renewed, err := s.deps.Store.ExtendBattleRoomPlaybackLease(ctx, reservation.RoomID, reservation.RunID, reservation.OwnerToken, nextLeaseUntil)
	if err != nil {
		return time.Time{}, err
	}
	if err := battlePlaybackLeaseRenewalError(renewed); err != nil {
		return time.Time{}, err
	}
	return nextLeaseUntil, nil
}

func battlePlaybackLeaseRenewalError(renewed bool) error {
	if !renewed {
		return errBattlePlaybackLeaseLost
	}
	return nil
}

func (s *Server) ensureBattlePlaybackLeaseBeforeStart(ctx context.Context, reservation battlePlaybackWorkerReservation, nextTickAt time.Time, leaseUntil time.Time) (time.Time, error) {
	if _, ok := s.battlePlaybackWorkerState(reservation); !ok {
		return time.Time{}, errBattlePlaybackLeaseLost
	}
	now := time.Now()
	if !battlePlaybackLeaseNeedsRenewal(now, nextTickAt, leaseUntil) {
		return leaseUntil, nil
	}
	renewCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	return s.renewBattlePlaybackLease(renewCtx, reservation, now)
}

func (s *Server) ensureBattlePlaybackLeaseForTick(ctx context.Context, reservation battlePlaybackWorkerReservation, leaseUntil time.Time) (time.Time, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if _, ok := s.battlePlaybackWorkerState(reservation); !ok {
		return time.Time{}, errBattlePlaybackLeaseLost
	}
	if battlePlaybackLeaseCoversTick(ctx, time.Now(), leaseUntil) {
		return leaseUntil, nil
	}
	renewCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	nextLeaseUntil, err := s.renewBattlePlaybackLease(renewCtx, reservation, time.Now())
	if err != nil {
		return time.Time{}, err
	}
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if !battlePlaybackLeaseCoversTick(ctx, time.Now(), nextLeaseUntil) {
		return time.Time{}, fmt.Errorf("battle playback renewed lease cannot cover tick window")
	}
	return nextLeaseUntil, nil
}

func (s *Server) runBattlePlaybackWorker(detail store.BattleRoomDetail, run store.BattleRun, reservation battlePlaybackWorkerReservation, nextTickAt time.Time, leaseUntil time.Time) {
	roomID := reservation.RoomID
	runID := reservation.RunID
	defer s.releaseBattlePlaybackWorkerForRun(detail, reservation)
	if run.ID != runID || detail.Room.ID != roomID {
		return
	}
	// Cache total timeline events for stuck detection (avoids re-reading timeline).
	// The timeline is immutable during playback — set once at compile time.
	totalEvents := 0
	if timeline, err := s.loadBattleRunPlaybackCache(context.Background(), runID); err == nil {
		totalEvents = len(timeline)
	}
	renewRetryNotBefore := time.Time{}
	for {
		if !s.updateBattlePlaybackWorkerState(reservation, func(state *battlePlaybackWorkerState) {
			state.LastNextTickAt = nextTickAt
		}) {
			return
		}
		wakeAt := battlePlaybackWorkerWakeAt(nextTickAt, leaseUntil, renewRetryNotBefore)
		wait := time.Until(wakeAt)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-s.battleStopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
		if !s.updateBattlePlaybackWorkerState(reservation, func(state *battlePlaybackWorkerState) {
			state.LastTickAt = time.Now()
		}) {
			return
		}
		if !nextTickAt.IsZero() && time.Now().Before(nextTickAt) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			nextLeaseUntil, renewErr := s.renewBattlePlaybackLease(ctx, reservation, time.Now())
			cancel()
			if renewErr != nil {
				if errors.Is(renewErr, errBattlePlaybackLeaseLost) {
					log.Printf("battle playback worker lease lost room_id=%d run_id=%d generation=%d", roomID, runID, reservation.Generation)
					return
				}
				if shouldSampleBattlePlaybackFailureLog(runID, time.Now()) {
					log.Printf("battle playback worker renew lease failed room_id=%d run_id=%d generation=%d err=%v", roomID, runID, reservation.Generation, renewErr)
				}
				renewRetryNotBefore = time.Now().Add(battlePlaybackLeaseRenewRetryDelay)
				continue
			}
			leaseUntil = nextLeaseUntil
			renewRetryNotBefore = time.Time{}
			continue
		}
		tickCtx, tickCancel := context.WithTimeout(context.Background(), battlePlaybackTickOperationTimeout)
		nextLeaseUntil, renewErr := s.ensureBattlePlaybackLeaseForTick(tickCtx, reservation, leaseUntil)
		if renewErr != nil {
			tickCancel()
			if errors.Is(renewErr, errBattlePlaybackLeaseLost) {
				log.Printf("battle playback worker lease lost before tick room_id=%d run_id=%d generation=%d", roomID, runID, reservation.Generation)
				return
			}
			if shouldSampleBattlePlaybackFailureLog(runID, time.Now()) {
				log.Printf("battle playback worker tick lease unavailable room_id=%d run_id=%d generation=%d err=%v", roomID, runID, reservation.Generation, renewErr)
			}
			renewRetryNotBefore = time.Now().Add(battlePlaybackLeaseRenewRetryDelay)
			continue
		}
		leaseUntil = nextLeaseUntil
		renewRetryNotBefore = time.Time{}
		previousState, ok := s.battlePlaybackWorkerState(reservation)
		if !ok {
			tickCancel()
			return
		}
		next, tickResult, tickErr := s.playbackBattleRunTick(tickCtx, detail, run, time.Now())
		tickCancel()
		if tickErr != nil {
			if errors.Is(tickErr, errBattlePlaybackFinished) {
				return
			}
			retryDelay := 200 * time.Millisecond
			if errors.Is(tickErr, errBattlePlaybackPublishUnavailable) {
				retryDelay = battlePlaybackPublishRetryDelay
				if shouldSampleBattlePlaybackFailureLog(runID, time.Now()) {
					log.Printf("battle playback worker publish delayed room_id=%d run_id=%d retry_ms=%d err=%v", roomID, runID, retryDelay.Milliseconds(), tickErr)
				}
			} else {
				log.Printf("battle playback worker tick failed room_id=%d run_id=%d err=%v", roomID, runID, tickErr)
			}
			nextTickAt = time.Now().Add(retryDelay)
			continue
		}
		progressed := tickResult.progressedFrom(previousState)
		if !s.updateBattlePlaybackWorkerState(reservation, func(state *battlePlaybackWorkerState) {
			state.LastTickAt = time.Now()
			state.LastPushedSeq = tickResult.LastPushedSeq
			state.LastNextEventIdx = tickResult.NextEventIdx
			if progressed {
				state.LastProgressAt = time.Now()
				state.StuckCount = 0
			} else {
				state.StuckCount++
			}
		}) {
			return
		}
		if tickResult.TotalEvents > 0 {
			totalEvents = tickResult.TotalEvents
		}
		if !progressed && tickResult.NextEventIdx < totalEvents {
			if state, ok := s.battlePlaybackWorkerState(reservation); ok && state.StuckCount >= battlePlaybackWorkerStuckFinalizeThreshold {
				log.Printf("battle playback recovery failed room_id=%d run_id=%d stuck_count=%d next_event_idx=%d last_pushed_seq=%d", roomID, runID, state.StuckCount, tickResult.NextEventIdx, tickResult.LastPushedSeq)
				_ = s.forceFinishBattlePlayback(context.Background(), detail, run)
				return
			}
			if state, ok := s.battlePlaybackWorkerState(reservation); ok && state.StuckCount >= battlePlaybackWorkerStuckRetryThreshold {
				log.Printf("battle playback recovery triggered room_id=%d run_id=%d stuck_count=%d next_event_idx=%d last_pushed_seq=%d", roomID, runID, state.StuckCount, tickResult.NextEventIdx, tickResult.LastPushedSeq)
				s.battleMetrics.AddPlaybackStuckRecovery()
				s.clearBattleRunPlaybackCache(runID)
				s.clearBattleRunPlaybackCheckpoint(runID)
				nextTickAt = time.Now().Add(200 * time.Millisecond)
				continue
			}
		}
		if next.IsZero() {
			nextTickAt = time.Now().Add(battleLoopMinSleep)
			continue
		}
		nextTickAt = next
	}
}

func (s *Server) releaseBattlePlaybackWorker(reservation battlePlaybackWorkerReservation) bool {
	s.battlePlaybackRunMu.Lock()
	defer s.battlePlaybackRunMu.Unlock()
	state, ok := s.battlePlaybackRuns[reservation.RunID]
	if !ok || state.Generation != reservation.Generation || state.OwnerToken != reservation.OwnerToken {
		return false
	}
	delete(s.battlePlaybackRuns, reservation.RunID)
	return true
}

func (s *Server) updateBattlePlaybackWorkerState(reservation battlePlaybackWorkerReservation, update func(state *battlePlaybackWorkerState)) bool {
	if reservation.RunID <= 0 || reservation.Generation == 0 || update == nil {
		return false
	}
	s.battlePlaybackRunMu.Lock()
	defer s.battlePlaybackRunMu.Unlock()
	state, ok := s.battlePlaybackRuns[reservation.RunID]
	if !ok || state.Generation != reservation.Generation || state.OwnerToken != reservation.OwnerToken {
		return false
	}
	update(&state)
	s.battlePlaybackRuns[reservation.RunID] = state
	return true
}

func (s *Server) battlePlaybackWorkerState(reservation battlePlaybackWorkerReservation) (battlePlaybackWorkerState, bool) {
	s.battlePlaybackRunMu.Lock()
	defer s.battlePlaybackRunMu.Unlock()
	state, ok := s.battlePlaybackRuns[reservation.RunID]
	if !ok || state.Generation != reservation.Generation || state.OwnerToken != reservation.OwnerToken {
		return battlePlaybackWorkerState{}, false
	}
	return state, true
}

func (s *Server) normalizeBattlePlaybackNextTickAt(roomID int64, runID int64, now time.Time, nextPushAtMs int64) time.Time {
	nextTickAt := playbackCheckpointNextTickAt(now, nextPushAtMs)
	maxAllowed := now.Add(battlePlaybackMaxFutureSleep)
	if nextTickAt.After(maxAllowed) {
		log.Printf("battle playback invalid schedule room_id=%d run_id=%d now_ms=%d next_push_at_ms=%d next_tick_at_ms=%d clamped_to_ms=%d", roomID, runID, now.UnixMilli(), nextPushAtMs, nextTickAt.UnixMilli(), maxAllowed.UnixMilli())
		return maxAllowed
	}
	return nextTickAt
}

func (s *Server) mustLoadBattlePlaybackTimeline(ctx context.Context, runID int64) []wsbus.BattleTimelineEvent {
	timeline, err := s.loadBattleRunPlaybackCache(ctx, runID)
	if err != nil {
		return nil
	}
	return timeline
}

func (s *Server) forceFinishBattlePlayback(ctx context.Context, detail store.BattleRoomDetail, run store.BattleRun) error {
	timeline, err := s.loadBattleRunPlaybackCache(ctx, run.ID)
	if err != nil {
		return err
	}
	return s.finishBattleRunPlayback(ctx, detail, run, timeline, time.Now())
}
