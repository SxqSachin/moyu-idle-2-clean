package httpapi

import (
	"context"
	"log"
	"time"

	"mysecondwebgame/server/internal/store"
)

const (
	battleSlowRunArchivePerRoomKeepCount = 5
	battleSlowRunArchiveKeepDaysDefault  = 30
)

func shouldArchiveBattleSlowRun(queueSinceStartMs, compileElapsedMs int64, compiled battleRunCompileResult) bool {
	return queueSinceStartMs >= battleCompileQueueSlowLogThresholdMs ||
		time.Duration(compileElapsedMs)*time.Millisecond >= battleCompileSlowLogThreshold ||
		compiled.TimedOut ||
		compiled.StepLimited
}

func battleSlowRunArchiveTriggerFlags(queueSinceStartMs, compileElapsedMs int64, compiled battleRunCompileResult) map[string]any {
	return map[string]any{
		"queueSlow":        queueSinceStartMs >= battleCompileQueueSlowLogThresholdMs,
		"compileSlow":      time.Duration(compileElapsedMs)*time.Millisecond >= battleCompileSlowLogThreshold,
		"guardTimedOut":    compiled.TimedOut,
		"guardStepLimited": compiled.StepLimited,
	}
}

func battlePersistableFinalResult(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if key == "playbackKeyframes" {
			continue
		}
		out[key] = value
	}
	return out
}

func (s *Server) captureBattleSlowRunArchive(
	ctx context.Context,
	workerID string,
	detail store.BattleRoomDetail,
	run store.BattleRun,
	task battleCompileKickoffTask,
	compiled battleRunCompileResult,
	queueSinceStartMs int64,
	compileElapsedMs int64,
	totalMs int64,
) {
	if s == nil || s.deps.Store == nil {
		return
	}
	if !shouldArchiveBattleSlowRun(queueSinceStartMs, compileElapsedMs, compiled) {
		return
	}
	turnProfile := normalizeAnyMap(compiled.Checkpoint["turnProfileMs"])
	if len(turnProfile) == 0 {
		turnProfile = map[string]any{}
	}
	input := store.BattleSlowRunArchiveInput{
		RoomID:            detail.Room.ID,
		RunID:             run.ID,
		DungeonID:         detail.Room.DungeonID,
		WorkerID:          workerID,
		Source:            task.Source,
		Wave:              compiled.Wave,
		Outcome:           battleOutcomeFromFinalResult(compiled.FinalResult),
		QueueSinceStartMs: queueSinceStartMs,
		CompileElapsedMs:  compileElapsedMs,
		TotalMs:           totalMs,
		TotalSteps:        compiled.TotalSteps,
		TotalTurns:        compiled.TotalTurns,
		TimelineEvents:    len(compiled.Timeline),
		FinalSeq:          toInt64Loose(compiled.Checkpoint["finalSeq"], 0),
		PlayerUnits:       compiled.PlayerUnits,
		GuardTimedOut:     compiled.TimedOut,
		GuardStepLimited:  compiled.StepLimited,
		TriggerFlags:      battleSlowRunArchiveTriggerFlags(queueSinceStartMs, compileElapsedMs, compiled),
		TurnProfile:       turnProfile,
		CompileInput:      map[string]any{},
		Checkpoint:        map[string]any{},
		Timeline:          []map[string]any{},
		FinalResult:       map[string]any{},
		RunStartedAt:      &run.StartedAt,
	}
	s.launchBattleTaskOrRun(func() {
		archiveCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.deps.Store.UpsertBattleSlowRunArchive(archiveCtx, input); err != nil {
			log.Printf("battle slow run archive upsert failed room_id=%d run_id=%d source=%s err=%v", detail.Room.ID, run.ID, task.Source, err)
			return
		}
		if err := s.deps.Store.TrimBattleSlowRunArchivesByRoom(archiveCtx, detail.Room.ID, battleSlowRunArchivePerRoomKeepCount); err != nil {
			log.Printf("battle slow run archive trim failed room_id=%d run_id=%d err=%v", detail.Room.ID, run.ID, err)
		}
	})
}
