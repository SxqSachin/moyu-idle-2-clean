package httpapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	battlecore "mysecondwebgame/server/internal/battle/core"
	"mysecondwebgame/server/internal/store"
)

func battleRunResultSummary(finalResult map[string]any) map[string]any {
	if len(finalResult) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	for _, key := range []string{
		"outcome",
		"rewardPlan",
		"nextPreparationOrder",
		"totalEvents",
		"totalTurns",
		"totalSteps",
		"compilePlayerUnits",
		"compileBudgetSteps",
		"compileTimedOut",
		"compileStepLimit",
		"battleStats",
		"turnProfileMs",
		"compiledAtMs",
	} {
		if value, ok := finalResult[key]; ok {
			out[key] = value
		}
	}
	if manifest, ok := buildBattleSettlementManifestV1(finalResult); ok {
		out["settlementManifestV1"] = manifest
	}
	return out
}

func battleCompileInputSnapshot(rows []battleUnitState) map[string]any {
	unitIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		unitID := strings.TrimSpace(row.UnitID)
		if unitID == "" {
			continue
		}
		unitIDs = append(unitIDs, unitID)
	}
	return map[string]any{
		"count":   len(rows),
		"unitIds": unitIDs,
	}
}

func (s *Server) battleCompilePlayerUnitBudget(players []battleUnitState) (int, int) {
	perUnit := s.deps.Config.GlobalSettings.Battle.CompileTurnsPerPlayerUnit
	if perUnit <= 0 {
		perUnit = 100
	}
	unitCount := 0
	for _, unit := range players {
		unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
		switch unitType {
		case "player", "kitty":
			unitCount++
		}
	}
	if unitCount <= 0 {
		unitCount = 1
	}
	return unitCount, unitCount * perUnit
}

func (s *Server) compileBattleRun(
	ctx context.Context,
	detail store.BattleRoomDetail,
	run store.BattleRun,
	worldCfg battleWorldConfig,
	enemyCatalog map[string]battleEnemyConfig,
	now time.Time,
) (battleRunCompileResult, error) {
	state := decodeBattleRuntimeState(run.State)
	if len(state.Players) == 0 {
		initialized, initErr := s.initBattleRuntimeState(ctx, detail, run, worldCfg, enemyCatalog, now)
		if initErr != nil {
			return battleRunCompileResult{}, initErr
		}
		state = initialized
	}
	state.battlefieldEffectDefinitions = append([]battleWorldBattlefieldEffect(nil), worldCfg.BattlefieldEffects...)
	state.battlefieldSpecialEnemyIDs = stringSliceSet(worldCfg.SpecialEnemyIDs)
	if len(state.BattlefieldEffects) == 0 && len(worldCfg.BattlefieldEffects) > 0 {
		state.BattlefieldEffects = buildBattleMissingInitialBattlefieldEffects(
			worldCfg,
			state.Players,
			run.Wave,
			func(expectedWave int) (battlePreviousWaveCarryState, error) {
				return s.loadBattlePreviousWaveCarryState(ctx, detail.Room.ID, run.ID, expectedWave)
			},
		)
	}

	if run.Wave <= 1 {
		s.updateBattlePlayerRewardEligibility(ctx, &state, now)
	}
	s.rebuildBattleOrder(&state)
	initialSnapshot := map[string]any{
		"wave":               state.Wave,
		"completedBattles":   state.CompletedBattles,
		"seq":                int64(0),
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   append([]string(nil), state.PreparationOrder...),
		"players":            battlePublicUnitSlice(state.Players),
		"enemies":            battlePublicUnitSlice(state.Enemies),
		"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
		"events":             []battleEvent{},
	}
	turnInterval := time.Duration(s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs) * time.Millisecond
	if turnInterval <= 0 {
		turnInterval = time.Second
	}
	interBattleGap := time.Duration(s.deps.Config.GlobalSettings.Battle.InterBattleIntervalMs) * time.Millisecond
	if interBattleGap < 0 {
		interBattleGap = 0
	}
	playerUnitCount, maxCompileSteps := s.battleCompilePlayerUnitBudget(state.Players)
	rewardPlan := map[int64]store.BattleWaveReward{}
	orchestrated, orchestrationErr := battlecore.RunCompileOrchestration(battlecore.CompileOrchestratorInput{
		Now:             now,
		TurnInterval:    turnInterval,
		InterBattleGap:  interBattleGap,
		MaxSteps:        maxCompileSteps,
		MaxExecDuration: time.Duration(s.deps.Config.GlobalSettings.Battle.CompileMaxExecMs) * time.Millisecond,
		ProcessTurnStep: func(simNow time.Time) (status string, waveWon bool, err error) {
			return s.processBattleTurn(ctx, &state, worldCfg, enemyCatalog, simNow, rewardPlan)
		},
	})
	if orchestrationErr != nil {
		return battleRunCompileResult{}, fmt.Errorf("%w run_id=%d", orchestrationErr, run.ID)
	}
	outcome := orchestrated.Outcome
	timeline, playbackTimeline := encodeBattleTimelineWithPlayback(state.Events, state.StartedAt, turnInterval.Milliseconds())
	totalTurns := countBattleTimelineTurns(state.Events)
	battleStats := battleAchievementStatsFromState(state)
	finalResult := map[string]any{
		"outcome":              outcome,
		"rewardPlan":           encodeBattleRewardPlan(rewardPlan),
		"achievementStats":     battleStats,
		"battleStats":          battleStats,
		"finalState":           battleSettlementStatePayload(state, outcome),
		"carryState":           battleCarryStatePayload(state, outcome),
		"nextPreparationOrder": battleNextPreparationOrderFromState(state),
		"totalEvents":          len(timeline),
		"totalTurns":           totalTurns,
		"totalSteps":           orchestrated.Steps,
		"compilePlayerUnits":   playerUnitCount,
		"compileBudgetSteps":   maxCompileSteps,
		"compileTimedOut":      orchestrated.TimedOut,
		"compileStepLimit":     orchestrated.StepLimitReached,
		"turnProfileMs":        state.turnProfile.toMap(),
		"compiledAtMs":         now.UnixMilli(),
	}
	finalResult = battlePersistableFinalResult(finalResult)
	compileInput := map[string]any{
		"passiveSources": state.PassiveSources,
		"runId":          run.ID,
		"roomId":         detail.Room.ID,
		"wave":           state.Wave,
		"seed":           state.Seed,
		"compiledAt":     now.UnixMilli(),
		"snapshot": map[string]any{
			"players":            battleCompileInputSnapshot(state.Players),
			"enemies":            battleCompileInputSnapshot(state.Enemies),
			"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
		},
		"schemaVersion": 2,
	}
	checkpoint := map[string]any{
		"passiveSources":      state.PassiveSources,
		"playbackStartedAtMs": now.UnixMilli(),
		"compileStartedAtMs":  now.UnixMilli(),
		"compileFinishedAtMs": time.Now().UnixMilli(),
		"lastDispatchAtMs":    int64(0),
		"lastPushedSeq":       int64(0),
		"lastPushedOffsetMs":  int64(-1),
		"totalEvents":         len(timeline),
		"finalSeq":            timelineLastSeq(timeline),
		"turnProfileMs":       state.turnProfile.toMap(),
		"stage":               "compiled",
		"outcome":             outcome,
		"wave":                initialSnapshot["wave"],
		"completedBattles":    initialSnapshot["completedBattles"],
		"seq":                 initialSnapshot["seq"],
		"order":               initialSnapshot["order"],
		"preparationOrder":    initialSnapshot["preparationOrder"],
		"players":             initialSnapshot["players"],
		"enemies":             initialSnapshot["enemies"],
		"battlefieldEffects":  initialSnapshot["battlefieldEffects"],
		"events":              initialSnapshot["events"],
	}
	return battleRunCompileResult{
		CompileInput:     compileInput,
		Wave:             state.Wave,
		Timeline:         timeline,
		PlaybackTimeline: playbackTimeline,
		FinalResult:      finalResult,
		Checkpoint:       checkpoint,
		PlayerUnits:      playerUnitCount,
		MaxSteps:         maxCompileSteps,
		TotalTurns:       totalTurns,
		TotalSteps:       orchestrated.Steps,
		TimedOut:         orchestrated.TimedOut,
		StepLimited:      orchestrated.StepLimitReached,
	}, nil
}

func countBattleTimelineTurns(events []battleEvent) int64 {
	var turns int64
	for _, event := range events {
		if event.Type == "battle.timeline.node" {
			turns++
		}
	}
	return turns
}

func timelineLastSeq(timeline []map[string]any) int64 {
	if len(timeline) == 0 {
		return 0
	}
	return toInt64Loose(timeline[len(timeline)-1]["seq"], 0)
}
