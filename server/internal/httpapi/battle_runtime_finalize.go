package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func (s *Server) finishBattleRunPlayback(
	ctx context.Context,
	detail store.BattleRoomDetail,
	run store.BattleRun,
	timeline []wsbus.BattleTimelineEvent,
	now time.Time,
) error {
	checkpoint := s.loadBattleRunPlaybackCheckpoint(run.ID, run.State, now)
	checkpoint.Stage = "playback_finished"
	checkpoint.NextPushAtMs = 0
	checkpoint.TotalEvents = len(timeline)
	if len(timeline) > 0 {
		checkpoint.FinalSeq = timeline[len(timeline)-1].Seq
	}
	s.setBattleRunPlaybackCheckpoint(run.ID, checkpoint)
	if err := s.deps.Store.SaveBattleRunState(ctx, run.ID, run.Wave, store.BattleRunStatusRunning, checkpoint.cursorMap(), nil); err != nil {
		return err
	}
	return s.deps.Store.EnqueueBattleRunSettlement(ctx, detail.Room.ID, run.ID, now, now.Add(30*time.Second))
}

func (s *Server) settleBattleRunPlayback(
	ctx context.Context,
	detail store.BattleRoomDetail,
	run store.BattleRun,
	timeline []wsbus.BattleTimelineEvent,
	finalResult map[string]any,
	now time.Time,
) error {
	finalizeWaitStartedAt := time.Now()
	releaseFinalize, err := s.acquireBattleFinalizeSlot(context.WithoutCancel(ctx))
	if err != nil {
		return err
	}
	defer releaseFinalize()
	finalizeWaitElapsed := time.Since(finalizeWaitStartedAt)
	finalizeCtx, finalizeCancel := context.WithTimeout(context.WithoutCancel(ctx), battleFinalizeOperationTimeout)
	defer finalizeCancel()
	ctx = finalizeCtx
	finalizeStartedAt := time.Now()
	outcome := battleOutcomeFromFinalResult(finalResult)
	rewards := decodeBattleRewardPlan(finalResult)
	attrExpJobPayload, err := battleAttrExpJobPayload(rewards)
	if err != nil {
		return err
	}
	if outcome != store.BattleRunStatusEnded && outcome != store.BattleRunStatusFailed {
		attrExpJobPayload = nil
	}
	skillExpSettleElapsed := time.Duration(0)
	skillExpSettleStats := battleSkillExpSettlementStats{}
	skillExpJobPayload := []byte(nil)
	if outcome == store.BattleRunStatusEnded || outcome == store.BattleRunStatusFailed {
		skillExpStartedAt := time.Now()
		var skillExpRows []battleSkillExpRewardRow
		skillExpSettleStats, skillExpRows, err = s.settleBattleSkillExpFromTimeline(ctx, timeline, finalResult)
		if err == nil {
			skillExpJobPayload, err = battleSkillExpJobPayload(skillExpRows)
		}
		skillExpSettleElapsed = time.Since(skillExpStartedAt)
		if err != nil {
			return err
		}
	}
	killAchievementBatches := []store.AchievementEventBatch(nil)
	if outcome == store.BattleRunStatusEnded && len(rewards) > 0 {
		killAchievementBatches = battleKillAchievementEventBatches(run.ID, rewards, finalResult, timeline)
	}
	itemBagSettleElapsed := time.Duration(0)
	itemBagSettleStats := battleItemBagSettlementStats{}
	itemBagUpdates := []store.BattleItemBagQuantityUpdate(nil)
	if outcome == store.BattleRunStatusEnded || outcome == store.BattleRunStatusFailed {
		// The store commits this absolute snapshot in the room/run-fenced finalization
		// transaction so an orphan settlement job cannot overwrite a newer wave.
		itemBagStartedAt := time.Now()
		itemBagUpdates, itemBagSettleStats = battleItemBagUpdatesFromFinalResult(finalResult)
		itemBagSettleElapsed = time.Since(itemBagStartedAt)
	}
	var nextDetail store.BattleRoomDetail
	finalizedByThisCall := false
	interBattleDelay := time.Duration(s.deps.Config.GlobalSettings.Battle.InterBattleIntervalMs) * time.Millisecond
	recoveryDuration := time.Duration(s.deps.Config.GlobalSettings.Battle.RecoveryDurationMs) * time.Millisecond
	finalizeTxnStartedAt := time.Now()
	const finalizeMaxAttempts = 3
	for attempt := 1; attempt <= finalizeMaxAttempts; attempt++ {
		if outcome == store.BattleRunStatusEnded {
			continued, _, continueErr := s.deps.Store.CompleteBattleRunAndMaybeContinue(ctx, detail.Room.ID, run.ID, rewards, itemBagUpdates, attrExpJobPayload, skillExpJobPayload, killAchievementBatches, now, interBattleDelay)
			if continueErr != nil {
				alreadyDone, doneErr := s.battleRunFinalizeAlreadyApplied(ctx, detail.Room.ID, run.ID, outcome)
				if doneErr != nil {
					return doneErr
				}
				if alreadyDone {
					nextDetail = detail
					finalizedByThisCall = false
					break
				}
				if !battleFinalizeRetryable(continueErr) || attempt == finalizeMaxAttempts {
					return continueErr
				}
				log.Printf("battle run finalize retry room_id=%d run_id=%d outcome=%s attempt=%d err=%v", detail.Room.ID, run.ID, outcome, attempt, continueErr)
				time.Sleep(20 * time.Millisecond)
				continue
			}
			nextDetail = continued
			finalizedByThisCall = true
		} else if outcome == store.BattleRunStatusFailed {
			recovered, recoverErr := s.deps.Store.HandleBattleRunFailure(ctx, detail.Room.ID, run.ID, rewards, itemBagUpdates, attrExpJobPayload, skillExpJobPayload, killAchievementBatches, now, recoveryDuration)
			if recoverErr != nil {
				alreadyDone, doneErr := s.battleRunFinalizeAlreadyApplied(ctx, detail.Room.ID, run.ID, outcome)
				if doneErr != nil {
					return doneErr
				}
				if alreadyDone {
					nextDetail = detail
					finalizedByThisCall = false
					break
				}
				if !battleFinalizeRetryable(recoverErr) || attempt == finalizeMaxAttempts {
					return recoverErr
				}
				log.Printf("battle run finalize retry room_id=%d run_id=%d outcome=%s attempt=%d err=%v", detail.Room.ID, run.ID, outcome, attempt, recoverErr)
				time.Sleep(20 * time.Millisecond)
				continue
			}
			nextDetail = recovered
			finalizedByThisCall = true
		} else {
			finished, finishErr := s.deps.Store.FinishBattleRoom(ctx, detail.Room.ID, run.ID, outcome, rewards, itemBagUpdates, attrExpJobPayload, skillExpJobPayload, killAchievementBatches, now)
			if finishErr != nil {
				alreadyDone, doneErr := s.battleRunFinalizeAlreadyApplied(ctx, detail.Room.ID, run.ID, outcome)
				if doneErr != nil {
					return doneErr
				}
				if alreadyDone {
					nextDetail = detail
					finalizedByThisCall = false
					break
				}
				if !battleFinalizeRetryable(finishErr) || attempt == finalizeMaxAttempts {
					return finishErr
				}
				log.Printf("battle run finalize retry room_id=%d run_id=%d outcome=%s attempt=%d err=%v", detail.Room.ID, run.ID, outcome, attempt, finishErr)
				time.Sleep(20 * time.Millisecond)
				continue
			}
			nextDetail = finished
			finalizedByThisCall = true
		}
		break
	}
	if finalizedByThisCall {
		itemBagSettleStats.RowsUpdated = len(itemBagUpdates)
	}
	finalizeTxnElapsed := time.Since(finalizeTxnStartedAt)
	if !finalizedByThisCall {
		// A concurrent claimant may have committed the core room transaction and
		// then stopped before post-finalize outboxes. Reconcile every idempotent
		// deferred reward before this claimant is allowed to retire the job.
		reconciledRun := run
		reconciledRun.Status = outcome
		if err := s.reconcileBattleSettlementPostFinalize(ctx, reconciledRun, timeline, finalResult, now); err != nil {
			return err
		}
		freshDetail, detailErr := s.loadBattleRoomDetailFresh(ctx, detail.Room.ID)
		if detailErr == nil {
			nextDetail = freshDetail
		}
	}
	if outcome == store.BattleRunStatusEnded && nextDetail.Room.Status == store.BattleRoomStatusPlaying && nextDetail.Room.CurrentRunID == nil {
		readyAtMs := now.Add(interBattleDelay).UnixMilli()
		s.markBattleRoomReadyForNext(nextDetail.Room.ID, readyAtMs)
		s.pushBattleRoomEventByMembers(nextDetail.Members, "battle.run.preparing_next", fiber.Map{
			"roomId":             detail.Room.ID,
			"completedRuns":      nextDetail.Room.CompletedRuns,
			"nextBattleStartsAt": now.Add(interBattleDelay).UnixMilli(),
		}, ctx)
	}
	s.pushBattleRoomUpdated(nextDetail, ctx)
	compactElapsed := time.Duration(0)
	bagSettleElapsed := itemBagSettleElapsed
	s.clearBattleRunPlaybackCache(run.ID)
	shouldLogFinalize := false
	if !finalizedByThisCall {
		shouldLogFinalize = time.Since(finalizeStartedAt) >= battleFinalizeSlowLogThreshold
		if shouldLogFinalize {
			log.Printf(
				"battle run finalize slow room_id=%d run_id=%d outcome=%s finalized=%t finalize_wait_ms=%d finalize_tx_ms=%d compact_ms=%d bag_settle_ms=%d skill_exp_settle_ms=%d skill_exp_users=%d skill_exp_skills=%d skill_exp_total_exp=%d item_bag_settle_ms=%d item_bag_users=%d item_bag_entries=%d item_bag_rows=%d total_ms=%d",
				detail.Room.ID,
				run.ID,
				outcome,
				finalizedByThisCall,
				finalizeWaitElapsed.Milliseconds(),
				finalizeTxnElapsed.Milliseconds(),
				compactElapsed.Milliseconds(),
				bagSettleElapsed.Milliseconds(),
				skillExpSettleElapsed.Milliseconds(),
				skillExpSettleStats.Users,
				skillExpSettleStats.Skills,
				skillExpSettleStats.Exp,
				itemBagSettleElapsed.Milliseconds(),
				itemBagSettleStats.UsersTouched,
				itemBagSettleStats.EntriesSeen,
				itemBagSettleStats.RowsUpdated,
				time.Since(finalizeStartedAt).Milliseconds(),
			)
		}
		return nil
	}
	compileTimedOut := battleFinalResultFlag(finalResult, "compileTimedOut")
	compileStepLimit := battleFinalResultFlag(finalResult, "compileStepLimit")
	failureReason := battleFailureReasonFromFinalResult(finalResult)
	s.pushBattleRoomEventByMembers(nextDetail.Members, "battle.run.finished", fiber.Map{
		"roomId":                detail.Room.ID,
		"runId":                 run.ID,
		"result":                outcome,
		"failureReason":         failureReason,
		"compileTimedOut":       compileTimedOut,
		"compileStepLimit":      compileStepLimit,
		"interBattleIntervalMs": s.deps.Config.GlobalSettings.Battle.InterBattleIntervalMs,
	}, ctx)
	s.markBattleRunFinished(detail.Room.ID, now.UnixMilli())
	if outcome != store.BattleRunStatusEnded || compileTimedOut || compileStepLimit {
		log.Printf(
			"battle run finished room_id=%d run_id=%d finished_at_ms=%d outcome=%s failure_reason=%s compile_timed_out=%t compile_step_limited=%t",
			detail.Room.ID,
			run.ID,
			now.UnixMilli(),
			outcome,
			failureReason,
			compileTimedOut,
			compileStepLimit,
		)
	}
	s.scheduleBattleRunHistoryCleanup(detail.Room.ID)
	shouldLogFinalize = time.Since(finalizeStartedAt) >= battleFinalizeSlowLogThreshold
	if shouldLogFinalize {
		log.Printf(
			"battle run finalize slow room_id=%d run_id=%d outcome=%s finalized=%t finalize_wait_ms=%d finalize_tx_ms=%d compact_ms=%d bag_settle_ms=%d skill_exp_settle_ms=%d skill_exp_users=%d skill_exp_skills=%d skill_exp_total_exp=%d item_bag_settle_ms=%d item_bag_users=%d item_bag_entries=%d item_bag_rows=%d total_ms=%d",
			detail.Room.ID,
			run.ID,
			outcome,
			finalizedByThisCall,
			finalizeWaitElapsed.Milliseconds(),
			finalizeTxnElapsed.Milliseconds(),
			compactElapsed.Milliseconds(),
			bagSettleElapsed.Milliseconds(),
			skillExpSettleElapsed.Milliseconds(),
			skillExpSettleStats.Users,
			skillExpSettleStats.Skills,
			skillExpSettleStats.Exp,
			itemBagSettleElapsed.Milliseconds(),
			itemBagSettleStats.UsersTouched,
			itemBagSettleStats.EntriesSeen,
			itemBagSettleStats.RowsUpdated,
			time.Since(finalizeStartedAt).Milliseconds(),
		)
	}
	return nil
}

func (s *Server) applyBattleKillAchievementEvents(ctx context.Context, runID int64, rewards map[int64]store.BattleWaveReward, finalResult map[string]any, timeline []wsbus.BattleTimelineEvent) error {
	for _, batch := range battleKillAchievementEventBatches(runID, rewards, finalResult, timeline) {
		if err := s.deps.Store.ApplyAchievementEventsNoReconcile(ctx, batch.UserID, batch.Events); err != nil {
			return err
		}
	}
	return nil
}

func battleKillAchievementEventBatches(runID int64, rewards map[int64]store.BattleWaveReward, finalResult map[string]any, timeline []wsbus.BattleTimelineEvent) []store.AchievementEventBatch {
	events := battleKillAchievementEventsFromResult(runID, finalResult, timeline)
	if len(events) == 0 {
		return nil
	}
	userIDs := make([]int64, 0, len(rewards))
	for userID := range rewards {
		if userID > 0 {
			userIDs = append(userIDs, userID)
		}
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	batches := make([]store.AchievementEventBatch, 0, len(userIDs))
	for _, userID := range userIDs {
		batches = append(batches, store.AchievementEventBatch{UserID: userID, Events: events})
	}
	return batches
}

func battleKillAchievementEventsFromFinalResult(runID int64, finalResult map[string]any) []store.AchievementEvent {
	return battleKillAchievementEventsFromResult(runID, finalResult, nil)
}

func battleKillAchievementEventsFromResult(runID int64, finalResult map[string]any, timeline []wsbus.BattleTimelineEvent) []store.AchievementEvent {
	counts := battleKillCountsFromAchievementStats(finalResult)
	if len(counts) == 0 {
		counts = battleKillCountsFromTimeline(timeline)
	}
	if len(counts) == 0 {
		counts = battleKillCountsFromFinalResultEnemies(finalResult)
	}
	if len(counts) == 0 {
		return nil
	}
	enemyIDs := make([]string, 0, len(counts))
	for enemyID := range counts {
		enemyIDs = append(enemyIDs, enemyID)
	}
	sort.Strings(enemyIDs)
	events := make([]store.AchievementEvent, 0, len(enemyIDs))
	for _, enemyID := range enemyIDs {
		events = append(events, store.AchievementEvent{
			Kind:      store.AchievementEventKillEnemy,
			Count:     counts[enemyID],
			SourceKey: fmt.Sprintf("battle_run:%d:kill:%s", runID, enemyID),
			SubjectID: enemyID,
			EnemyID:   enemyID,
		})
	}
	return events
}

func battleKillCountsFromAchievementStats(finalResult map[string]any) map[string]int64 {
	for _, rawStats := range []any{finalResult["battleStats"], finalResult["achievementStats"]} {
		stats := normalizeAnyMap(rawStats)
		kills := normalizeAnyMap(firstNonNil(stats["enemyKills"], stats["enemyKillCounts"]))
		if len(kills) == 0 {
			continue
		}
		out := map[string]int64{}
		for enemyID, raw := range kills {
			enemyID = strings.TrimSpace(enemyID)
			count := toInt64Loose(raw, 0)
			if enemyID != "" && count > 0 {
				out[enemyID] += count
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func battleKillCountsFromFinalResultEnemies(finalResult map[string]any) map[string]int64 {
	enemiesRaw, ok := finalResult["enemies"]
	if !ok {
		return nil
	}
	buf, err := json.Marshal(enemiesRaw)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(buf, &rows); err != nil {
		return nil
	}
	counts := map[string]int64{}
	for _, row := range rows {
		if battleResultUnitAlive(row) {
			continue
		}
		enemyID := strings.TrimSpace(fmt.Sprint(row["enemyId"]))
		if enemyID == "" || enemyID == "<nil>" {
			enemyID = battleEnemyIDFromUnitID(strings.TrimSpace(fmt.Sprint(row["unitId"])))
		}
		if enemyID == "" || enemyID == "<nil>" {
			continue
		}
		counts[enemyID]++
	}
	return counts
}

func battleKillCountsFromTimeline(timeline []wsbus.BattleTimelineEvent) map[string]int64 {
	if len(timeline) == 0 {
		return nil
	}
	counts := map[string]int64{}
	seenUnits := map[string]struct{}{}
	for _, event := range timeline {
		payload := map[string]any{}
		if len(event.Payload) == 0 || json.Unmarshal(event.Payload, &payload) != nil {
			continue
		}
		eventType := strings.TrimSpace(fmt.Sprint(payload["eventType"]))
		data, _ := payload["data"].(map[string]any)
		switch eventType {
		case "battle.turn.action":
			for _, row := range battleAnyMapSlice(data["actionLog"]) {
				if !parseBooleanLoose(row["beforeAlive"], false) || parseBooleanLoose(row["afterAlive"], true) {
					continue
				}
				battleAccumulateKilledEnemy(counts, seenUnits, strings.TrimSpace(fmt.Sprint(row["targetId"])))
			}
		case "battle.timeline.node":
			for _, step := range battleAnyMapSlice(data["p"]) {
				for _, target := range battleAnyMapSlice(step["t"]) {
					if parseBooleanLoose(target["l"], true) {
						continue
					}
					battleAccumulateKilledEnemy(counts, seenUnits, strings.TrimSpace(fmt.Sprint(target["u"])))
				}
			}
		}
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func battleAccumulateKilledEnemy(counts map[string]int64, seenUnits map[string]struct{}, unitID string) {
	unitID = strings.TrimSpace(unitID)
	if unitID == "" || !strings.HasPrefix(unitID, "enemy:") {
		return
	}
	if _, seen := seenUnits[unitID]; seen {
		return
	}
	enemyID := battleEnemyIDFromUnitID(unitID)
	if enemyID == "" {
		return
	}
	seenUnits[unitID] = struct{}{}
	counts[enemyID]++
}

func battleEnemyIDFromUnitID(unitID string) string {
	unitID = strings.TrimSpace(unitID)
	if unitID == "" {
		return ""
	}
	parts := strings.Split(unitID, ":")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func battleResultUnitAlive(row map[string]any) bool {
	if alive, ok := row["alive"].(bool); ok {
		return alive
	}
	if alive, ok := row["a"].(bool); ok {
		return alive
	}
	if hp, ok := battleResultFloat(row["hp"]); ok {
		return hp > 0
	}
	if hp, ok := battleResultFloat(row["h"]); ok {
		return hp > 0
	}
	return true
}

func battleResultFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}

func (s *Server) enqueueBattleSkillExpJob(ctx context.Context, runID int64, rows []battleSkillExpRewardRow, now time.Time) error {
	if runID <= 0 || len(rows) == 0 {
		return nil
	}
	payload, err := battleSkillExpJobPayload(rows)
	if err != nil {
		return err
	}
	return s.deps.Store.EnqueueBattleSkillExpJob(ctx, runID, payload, now)
}

func battleSkillExpJobPayload(rows []battleSkillExpRewardRow) ([]byte, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	return json.Marshal(rows)
}

func (s *Server) acquireBattleFinalizeSlot(ctx context.Context) (func(), error) {
	if s.battleFinalizeSlots == nil {
		return func() {}, nil
	}
	select {
	case s.battleFinalizeSlots <- struct{}{}:
		return func() { <-s.battleFinalizeSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.battleStopCh:
		return nil, context.Canceled
	}
}

func (s *Server) battleRunFinalizeAlreadyApplied(ctx context.Context, roomID, runID int64, expectedOutcome string) (bool, error) {
	if roomID <= 0 || runID <= 0 {
		return false, nil
	}
	run, err := s.deps.Store.GetBattleRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, store.ErrBattleRoomNotFound) {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(strings.ToLower(run.Status)) != strings.TrimSpace(strings.ToLower(expectedOutcome)) {
		return false, nil
	}
	detail, detailErr := s.loadBattleRoomDetailFresh(ctx, roomID)
	if detailErr != nil {
		if errors.Is(detailErr, store.ErrBattleRoomNotFound) {
			return false, nil
		}
		return false, detailErr
	}
	if detail.Room.CurrentRunID != nil && *detail.Room.CurrentRunID == runID {
		return false, nil
	}
	return true, nil
}
