package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	battlecore "mysecondwebgame/server/internal/battle/core"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

const (
	battleKindArena         = "arena"
	arenaMatchupHPBuffID    = "arena_matchup_hp_adjustment"
	arenaMatchupHPRuleID    = "arena_matchup_hp_adjustment_while_active"
	arenaOutputWarmupBuffID = "arena_output_warmup"
)

func battleArenaSummonBaseStats(state *battleRuntimeState, source *battleUnitState, stats map[string]float64) map[string]float64 {
	if state == nil || source == nil || !strings.EqualFold(strings.TrimSpace(state.BattleKind), battleKindArena) || stats == nil {
		return stats
	}
	bonus := source.StatSources["hp"][arenaMatchupHPRuleID]
	if bonus <= 0 {
		return stats
	}
	if hp, exists := stats["hp"]; exists {
		stats["hp"] = maxFloat(1, roundTo4(hp-bonus))
	}
	return stats
}

func battleArenaSummonFormulaSource(state *battleRuntimeState, source *battleUnitState) *battleUnitState {
	if state == nil || source == nil || !strings.EqualFold(strings.TrimSpace(state.BattleKind), battleKindArena) {
		return source
	}
	bonus := source.StatSources["hp"][arenaMatchupHPRuleID]
	if bonus <= 0 {
		return source
	}
	clone := *source
	clone.Stats = cloneFloatMap(source.Stats)
	clone.Stats["hp"] = maxFloat(1, roundTo4(clone.Stats["hp"]-bonus))
	baseMaxHP := maxFloat(1, roundTo4(source.MaxHP-bonus))
	hpRatio := 0.0
	if source.MaxHP > 0 {
		hpRatio = clampFloat(source.HP/source.MaxHP, 0, 1)
	}
	clone.MaxHP = baseMaxHP
	clone.HP = roundTo4(baseMaxHP * hpRatio)
	return &clone
}

func (s *Server) initArenaRuntimeState(challenge store.ArenaChallenge, now time.Time) (battleRuntimeState, error) {
	players, err := competitiveRuntimeLineup(challenge.AttackerLineup, "player", "arena:attacker")
	if err != nil {
		return battleRuntimeState{}, err
	}
	enemies, err := competitiveRuntimeLineup(challenge.DefenderLineup, "enemy", "arena:defender")
	if err != nil {
		return battleRuntimeState{}, err
	}
	format, err := decodeArenaFormatSnapshot(challenge.FormatSnapshot)
	if err != nil {
		return battleRuntimeState{}, err
	}
	state := battleRuntimeState{
		BattleKind:                 battleKindArena,
		Seed:                       challenge.Seed,
		StartedAt:                  now.UnixMilli(),
		LastActionAt:               now.UnixMilli(),
		Wave:                       1,
		Players:                    players,
		Enemies:                    enemies,
		Order:                      []string{},
		FullOrder:                  []string{},
		PreparationOrder:           []string{},
		PreparationSeedOrder:       []string{},
		ActedUnits:                 []string{},
		Events:                     []battleEvent{},
		RetainCompleteEventHistory: true,
		ArenaFormat:                format,
	}
	ensureSliceSpareCap(&state.Players, 40)
	ensureSliceSpareCap(&state.Enemies, 40)
	battleStartActions := s.battleApplyArenaFormatBattleStart(&state)
	battleStartActions = append(battleStartActions, s.battleTriggerBattleStartEquipmentHooks(&state, now)...)
	battleStartActions = append(battleStartActions, s.battleTriggerCompetitiveWaveStartHooks(&state, now)...)
	battleStartActions = append(battleStartActions, s.battleFlushPendingUnitDeathEquipmentHooks(&state, now)...)
	battleStartActions = append(battleStartActions, s.battleDrainArenaFormatConsequences(&state, now)...)
	battleStartActions = append(battleStartActions, s.battleFinalizeArenaFormatTurn(&state)...)
	for index := range state.Players {
		s.battleRecomputeUnitStats(&state.Players[index])
	}
	for index := range state.Enemies {
		s.battleRecomputeUnitStats(&state.Enemies[index])
	}
	s.rebuildBattleOrder(&state)
	state.emit("battle.wave.spawn", now, map[string]any{
		"wave":               state.Wave,
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   []string{},
		"players":            competitivePublicUnitSlice(state.Players),
		"enemies":            competitivePublicUnitSlice(state.Enemies),
		"battlefieldEffects": []map[string]any{},
		"battleStartActions": battleStartActions,
		"arenaFormat":        arenaPublicFormat(state.ArenaFormat),
	})
	return state, nil
}

func arenaPublicState(state battleRuntimeState) map[string]any {
	return map[string]any{
		"battleKind":         battleKindArena,
		"seed":               state.Seed,
		"wave":               state.Wave,
		"completedBattles":   state.CompletedBattles,
		"seq":                state.Seq,
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   []string{},
		"players":            competitivePublicUnitSlice(state.Players),
		"enemies":            competitivePublicUnitSlice(state.Enemies),
		"battlefieldEffects": []map[string]any{},
		"arenaFormat":        arenaPublicFormat(state.ArenaFormat),
		"events":             []battleEvent{},
	}
}

func (s *Server) compileArenaChallenge(ctx context.Context, challenge store.ArenaChallenge, now time.Time) (string, json.RawMessage, json.RawMessage, json.RawMessage, []wsbus.BattleTimelineEvent, error) {
	state, err := s.initArenaRuntimeState(challenge, now)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	initialState := arenaPublicState(state)
	turnInterval := time.Duration(s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs) * time.Millisecond
	if turnInterval <= 0 {
		turnInterval = time.Second
	}
	formalUnitCount := len(state.Players) + len(state.Enemies)
	perUnit := s.deps.Config.GlobalSettings.Battle.CompileTurnsPerPlayerUnit
	if perUnit <= 0 {
		perUnit = 100
	}
	rewardPlan := map[int64]store.BattleWaveReward{}
	orchestrated, err := battlecore.RunCompileOrchestration(battlecore.CompileOrchestratorInput{
		Now:             now,
		TurnInterval:    turnInterval,
		MaxSteps:        maxInt(1, formalUnitCount) * perUnit,
		MaxExecDuration: time.Duration(s.deps.Config.GlobalSettings.Battle.CompileMaxExecMs) * time.Millisecond,
		ProcessTurnStep: func(simNow time.Time) (string, bool, error) {
			return s.processBattleTurn(ctx, &state, battleWorldConfig{}, nil, simNow, rewardPlan)
		},
	})
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	if orchestrated.TimedOut || orchestrated.StepLimitReached {
		return "", nil, nil, nil, nil, fmt.Errorf("arena compile incomplete timed_out=%t step_limited=%t", orchestrated.TimedOut, orchestrated.StepLimitReached)
	}
	outcome := store.ArenaChallengeDefenderWin
	if orchestrated.Outcome == store.BattleRunStatusEnded {
		outcome = store.ArenaChallengeAttackerWin
	} else if orchestrated.Outcome != store.BattleRunStatusFailed {
		return "", nil, nil, nil, nil, fmt.Errorf("arena compile returned unexpected outcome %q", orchestrated.Outcome)
	}
	timeline, playbackTimeline := encodeBattleTimelineWithPlayback(state.Events, state.StartedAt, turnInterval.Milliseconds())
	finalResult := map[string]any{
		"battleKind":       battleKindArena,
		"outcome":          outcome,
		"seed":             state.Seed,
		"totalEvents":      len(timeline),
		"totalTurns":       countBattleTimelineTurns(state.Events),
		"totalSteps":       orchestrated.Steps,
		"compileTimedOut":  false,
		"compileStepLimit": false,
		"finalState":       arenaPublicState(state),
		"battleStats":      battleAchievementStatsFromState(state),
		"compiledAtMs":     time.Now().UnixMilli(),
	}
	initialRaw, err := json.Marshal(initialState)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	timelineRaw, err := json.Marshal(timeline)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	finalRaw, err := json.Marshal(finalResult)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return outcome, initialRaw, timelineRaw, finalRaw, playbackTimeline, nil
}

func (s *Server) startArenaChallengeLoop(workerRuntimeID string) {
	cfg := s.deps.Config.GlobalSettings.Arena
	if !cfg.Enabled {
		return
	}
	for index := 0; index < cfg.WorkerCount; index++ {
		workerID := fmt.Sprintf("%s:arena:%d", workerRuntimeID, index+1)
		s.launchBattleTask(func() { s.runArenaChallengeWorker(workerID) })
	}
	log.Printf("arena battle workers started count=%d history_keep=%d", cfg.WorkerCount, cfg.HistoryKeepCount)
}

func (s *Server) runArenaChallengeWorker(workerID string) {
	cfg := s.deps.Config.GlobalSettings.Arena
	poll := time.Duration(cfg.WorkerPollIntervalMs) * time.Millisecond
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	lease := time.Duration(cfg.WorkerLeaseSeconds) * time.Second
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		claimCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		challenge, err := s.deps.Store.ClaimArenaChallenge(claimCtx, workerID, lease, time.Now())
		cancel()
		if errors.Is(err, pgx.ErrNoRows) {
			select {
			case <-time.After(poll):
				continue
			case <-s.battleStopCh:
				return
			}
		}
		if err != nil {
			log.Printf("arena challenge claim failed worker=%s err=%v", workerID, err)
			select {
			case <-time.After(poll):
				continue
			case <-s.battleStopCh:
				return
			}
		}
		s.processClaimedArenaChallenge(workerID, challenge)
	}
}

func (s *Server) processClaimedArenaChallenge(workerID string, challenge store.ArenaChallenge) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s.pushBattleRoomEventToUsers([]int64{challenge.AttackerID}, "battle.run.started", fiber.Map{
		"battleKind":  battleKindArena,
		"roomId":      challenge.ID,
		"runId":       challenge.ID,
		"challengeId": challenge.ID,
	})
	outcome, initialRaw, timelineRaw, finalRaw, playbackTimeline, err := s.compileArenaChallenge(ctx, challenge, time.Now())
	if err != nil {
		log.Printf("arena challenge compile failed challenge=%d worker=%s attempt=%d err=%v", challenge.ID, workerID, challenge.Attempts, err)
		failCtx, failCancel := context.WithTimeout(context.Background(), 3*time.Second)
		arenaCfg := s.deps.Config.GlobalSettings.Arena
		terminal, failErr := s.deps.Store.FailArenaChallenge(failCtx, challenge.ID, workerID, "compile_failed", arenaCfg.MaxAttempts, arenaChallengeChargePolicy(arenaCfg))
		failCancel()
		if failErr != nil && !errors.Is(failErr, store.ErrArenaChallengeLeaseLost) {
			log.Printf("arena challenge failure save failed challenge=%d worker=%s err=%v", challenge.ID, workerID, failErr)
		}
		if terminal && failErr == nil {
			s.pruneArenaChallengeHistory(challenge.ID)
			s.pushArenaChallengeFailed(challenge)
		}
		return
	}
	cfg := s.deps.Config.GlobalSettings.Arena
	settlement, err := s.deps.Store.CompleteArenaChallenge(ctx, store.CompleteArenaChallengeInput{
		ChallengeID:  challenge.ID,
		WorkerID:     workerID,
		Outcome:      outcome,
		InitialState: initialRaw,
		Timeline:     timelineRaw,
		FinalResult:  finalRaw,
		Policy: store.ArenaSettlementPolicy{
			DefaultRating:             cfg.DefaultRating,
			AttackerWinTokens:         cfg.AttackerWinTokens,
			AttackerLossTokens:        cfg.AttackerLossTokens,
			DefenderWinTokens:         cfg.DefenderWinTokens,
			BountyPerDefenseWin:       cfg.BountyPerDefenseWin,
			BountyCap:                 cfg.BountyCap,
			AttackerWinRating:         cfg.AttackerWinRating,
			AttackerLossRatingPenalty: cfg.AttackerLossRatingPenalty,
			DefenderWinRating:         cfg.DefenderWinRating,
			DefenderLossRatingPenalty: cfg.DefenderLossRatingPenalty,
		},
	})
	if err != nil {
		log.Printf("arena challenge settlement failed challenge=%d worker=%s err=%v", challenge.ID, workerID, err)
		failCtx, failCancel := context.WithTimeout(context.Background(), 3*time.Second)
		terminal, failErr := s.deps.Store.FailArenaChallenge(failCtx, challenge.ID, workerID, "settlement_failed", cfg.MaxAttempts, arenaChallengeChargePolicy(cfg))
		failCancel()
		if failErr != nil && !errors.Is(failErr, store.ErrArenaChallengeLeaseLost) {
			log.Printf("arena challenge settlement failure save failed challenge=%d worker=%s err=%v", challenge.ID, workerID, failErr)
		}
		if terminal && failErr == nil {
			s.pruneArenaChallengeHistory(challenge.ID)
			s.pushArenaChallengeFailed(challenge)
		}
		return
	}
	s.pruneArenaChallengeHistory(challenge.ID)
	s.pushArenaChallengePlayback(challenge, outcome, initialRaw, playbackTimeline)
	log.Printf("arena challenge settled challenge=%d outcome=%s attacker=%d defender=%d", settlement.ChallengeID, outcome, settlement.AttackerID, settlement.DefenderID)
}

func (s *Server) pushArenaChallengeFailed(challenge store.ArenaChallenge) {
	if challenge.ID <= 0 || challenge.AttackerID <= 0 {
		return
	}
	s.pushBattleRoomEventToUsers([]int64{challenge.AttackerID}, "battle.run.finished", fiber.Map{
		"battleKind":  battleKindArena,
		"roomId":      challenge.ID,
		"runId":       challenge.ID,
		"challengeId": challenge.ID,
		"outcome":     store.ArenaChallengeStatusFailed,
	})
}

func (s *Server) pushArenaChallengePlayback(challenge store.ArenaChallenge, outcome string, initialRaw json.RawMessage, timeline []wsbus.BattleTimelineEvent) {
	if challenge.ID <= 0 || challenge.AttackerID <= 0 {
		return
	}
	var snapshot map[string]any
	if err := json.Unmarshal(initialRaw, &snapshot); err != nil {
		log.Printf("arena playback snapshot decode failed challenge=%d err=%v", challenge.ID, err)
		return
	}
	targetUsers := []int64{challenge.AttackerID}
	startedAtMs := time.Now().UnixMilli()
	s.pushBattleRoomEventToUsers(targetUsers, "battle.run.compiled", fiber.Map{
		"battleKind":  battleKindArena,
		"roomId":      challenge.ID,
		"runId":       challenge.ID,
		"challengeId": challenge.ID,
	})
	if !s.pushBattleRoomEventToUsers(targetUsers, "battle.run.batch", toArenaBattleEventBatchWirePayload(challenge.ID, wsbus.BattleEventBatchPayload{
		RunID:         challenge.ID,
		WindowStartMS: startedAtMs,
		WindowEndMS:   startedAtMs,
		Events:        []wsbus.BattleTimelineEvent{},
		Snapshot:      snapshot,
	})) {
		log.Printf("arena playback snapshot publish failed challenge=%d", challenge.ID)
		return
	}
	timelineCopy := append([]wsbus.BattleTimelineEvent(nil), timeline...)
	if !s.launchBattleTask(func() {
		s.streamArenaChallengeTimeline(challenge, outcome, startedAtMs, timelineCopy)
	}) {
		log.Printf("arena playback launch skipped during shutdown challenge=%d", challenge.ID)
	}
}

func (s *Server) streamArenaChallengeTimeline(challenge store.ArenaChallenge, outcome string, startedAtMs int64, timeline []wsbus.BattleTimelineEvent) {
	targetUsers := []int64{challenge.AttackerID}
	if len(timeline) > 0 && !s.waitBattlePlaybackDelay(battlePlaybackFirstEventDelay) {
		return
	}
	maxEvents := s.deps.Config.GlobalSettings.Battle.PlaybackMaxEventsBatch
	if maxEvents <= 0 {
		maxEvents = 100
	}
	intervalMs := s.deps.Config.GlobalSettings.Battle.PlaybackFixedIntervalMs
	if intervalMs <= 0 {
		intervalMs = s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs
	}
	if intervalMs <= 0 {
		intervalMs = 1000
	}
	for start := 0; start < len(timeline); {
		events, next, _ := collectPlaybackEventsAtCurrentOffset(timeline, start, maxEvents)
		if len(events) == 0 {
			break
		}
		fromSeq := events[0].Seq
		toSeq := events[len(events)-1].Seq
		windowStart := startedAtMs + maxInt64(0, events[0].OffsetMS)
		windowEnd := startedAtMs + maxInt64(0, events[len(events)-1].OffsetMS)
		if !s.pushBattleRoomEventToUsers(targetUsers, "battle.run.batch", toArenaBattleEventBatchWirePayload(challenge.ID, wsbus.BattleEventBatchPayload{
			RunID:         challenge.ID,
			FromSeq:       fromSeq,
			ToSeq:         toSeq,
			WindowStartMS: windowStart,
			WindowEndMS:   windowEnd,
			Events:        events,
		})) {
			log.Printf("arena playback batch publish failed challenge=%d from_seq=%d to_seq=%d", challenge.ID, fromSeq, toSeq)
			return
		}
		start = next
		if start < len(timeline) && !s.waitBattlePlaybackDelay(time.Duration(intervalMs)*time.Millisecond) {
			return
		}
	}
	s.pushBattleRoomEventToUsers(targetUsers, "battle.run.finished", fiber.Map{
		"battleKind":  battleKindArena,
		"roomId":      challenge.ID,
		"runId":       challenge.ID,
		"challengeId": challenge.ID,
		"outcome":     outcome,
	})
}

func (s *Server) pruneArenaChallengeHistory(challengeID int64) {
	cfg := s.deps.Config.GlobalSettings.Arena
	pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := s.deps.Store.PruneArenaChallengeHistory(pruneCtx, cfg.HistoryKeepCount); err != nil {
		log.Printf("arena challenge history prune failed challenge=%d err=%v", challengeID, err)
	}
	pruneCancel()
}
