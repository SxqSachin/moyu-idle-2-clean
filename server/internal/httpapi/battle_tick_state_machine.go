package httpapi

import (
	"context"
	"errors"
	"time"

	"mysecondwebgame/server/internal/store"
)

type battleTickExecutionMode string

const (
	battleTickExecutionNoop                battleTickExecutionMode = "noop"
	battleTickExecutionWaitCompileKickoff  battleTickExecutionMode = "wait_compile_kickoff"
	battleTickExecutionStartPlaybackWorker battleTickExecutionMode = "start_playback_worker"
	battleTickExecutionRunPlaybackTick     battleTickExecutionMode = "run_playback_tick"
)

type battleTickExecutionPlan struct {
	Mode       battleTickExecutionMode
	NextTickAt time.Time
}

func (s *Server) buildBattleTickExecutionPlan(pipe *battleTickPipeline, now time.Time) battleTickExecutionPlan {
	if pipe == nil || !pipe.worldReady {
		return battleTickExecutionPlan{Mode: battleTickExecutionNoop}
	}
	if pipe.run.Status == store.BattleRunStatusSettling {
		return battleTickExecutionPlan{
			Mode:       battleTickExecutionNoop,
			NextTickAt: now.Add(battlePlaybackLeaseTTL),
		}
	}
	if pipe.detail.Room.Status != store.BattleRoomStatusPlaying {
		return battleTickExecutionPlan{Mode: battleTickExecutionNoop}
	}
	if pipe.run.Status == store.BattleRunStatusCompiled {
		return battleTickExecutionPlan{
			Mode:       battleTickExecutionWaitCompileKickoff,
			NextTickAt: now.Add(battleCompilePendingRetryDelay),
		}
	}
	if pipe.run.Status == store.BattleRunStatusRunning {
		return battleTickExecutionPlan{Mode: battleTickExecutionStartPlaybackWorker}
	}
	return battleTickExecutionPlan{Mode: battleTickExecutionRunPlaybackTick}
}

func (s *Server) executeBattleTickExecutionPlan(
	ctx context.Context,
	workerID string,
	pipe *battleTickPipeline,
	plan battleTickExecutionPlan,
	now time.Time,
	metrics *battleTickRoomMetrics,
) (time.Time, bool, error) {
	switch plan.Mode {
	case battleTickExecutionNoop, battleTickExecutionWaitCompileKickoff:
		return plan.NextTickAt, false, nil
	case battleTickExecutionStartPlaybackWorker:
		if err := s.loadBattleTickMembersIfNeeded(ctx, pipe); err != nil {
			return time.Time{}, false, err
		}
		reservation, ok := s.newBattlePlaybackWorkerReservation(pipe.detail.Room.ID, pipe.run.ID, workerID)
		if !ok {
			return time.Time{}, false, store.ErrBattleRoomStateInvalid
		}
		leaseUntil := now.Add(battlePlaybackLeaseTTL)
		holdLeaseStartedAt := time.Now()
		if err := s.deps.Store.HoldBattleRoomPlaybackLease(ctx, pipe.detail.Room.ID, workerID, reservation.OwnerToken, leaseUntil); err != nil {
			return time.Time{}, false, err
		}
		if metrics != nil {
			metrics.HoldLeaseElapsed = time.Since(holdLeaseStartedAt)
		}
		if !s.activateBattlePlaybackWorkerForRun(pipe.detail, reservation, now) {
			return time.Time{}, false, store.ErrBattleRoomStateInvalid
		}
		leaseUntil, err := s.ensureBattlePlaybackLeaseBeforeStart(ctx, reservation, now, leaseUntil)
		if err != nil {
			s.releaseBattlePlaybackWorkerForRun(pipe.detail, reservation)
			return time.Time{}, false, err
		}
		s.startReservedBattlePlaybackWorker(pipe.detail, pipe.run, reservation, now, leaseUntil)
		return time.Time{}, true, nil
	case battleTickExecutionRunPlaybackTick:
		if err := s.loadBattleTickMembersIfNeeded(ctx, pipe); err != nil {
			return time.Time{}, false, err
		}
		playbackStartedAt := time.Now()
		nextTickAt, _, err := s.playbackBattleRunTick(ctx, pipe.detail, pipe.run, now)
		if metrics != nil {
			metrics.PlaybackElapsed = time.Since(playbackStartedAt)
		}
		if err != nil {
			if errors.Is(err, errBattlePlaybackFinished) {
				return nextTickAt, false, nil
			}
			return time.Time{}, false, err
		}
		return nextTickAt, false, nil
	default:
		return time.Time{}, false, nil
	}
}
