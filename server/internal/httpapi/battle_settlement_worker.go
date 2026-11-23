package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

const (
	battleSettlementWorkerIdleSleep  = 200 * time.Millisecond
	battleSettlementJobLease         = 30 * time.Second
	battleSettlementSlowLogThreshold = 8 * time.Second
)

// A worker finalizes claims serially. Claim one at a time so later jobs cannot
// expire locally while an earlier finalization waits on its bounded slot.
const battleSettlementClaimBatchSize = 1

var errBattleSettlementPlaybackNotFinished = errors.New("playback not finished")

func (s *Server) startBattleSettlementLoop(workerID string) {
	workers := s.deps.Config.GlobalSettings.Battle.SettlementWorkers
	if workers <= 0 {
		workers = 3
	}
	for idx := 0; idx < workers; idx++ {
		settlementWorkerID := fmt.Sprintf("%s:settlement:%d", workerID, idx)
		s.launchBattleTask(func() { s.runBattleSettlementWorker(settlementWorkerID) })
	}
}

func (s *Server) runBattleSettlementWorker(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		jobs, err := s.deps.Store.ClaimBattleSettlementJobs(ctx, workerID, battleSettlementClaimBatchSize, battleSettlementJobLease, time.Now())
		cancel()
		if err != nil {
			log.Printf("battle settlement claim failed worker=%s err=%v", workerID, err)
			s.sleepBattleSettlementWorker(battleSettlementWorkerIdleSleep)
			continue
		}
		if len(jobs) == 0 {
			s.sleepBattleSettlementWorker(battleSettlementWorkerIdleSleep)
			continue
		}
		for _, job := range jobs {
			s.processBattleSettlementJob(workerID, job)
		}
	}
}

func (s *Server) sleepBattleSettlementWorker(duration time.Duration) {
	if duration <= 0 {
		duration = battleSettlementWorkerIdleSleep
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-s.battleStopCh:
	}
}

func (s *Server) processBattleSettlementJob(workerID string, job store.BattleSettlementJob) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), battleFinalizeOperationTimeout)
	defer cancel()
	payload, err := s.loadBattleSettlementRun(ctx, job.RunID)
	if err != nil {
		s.failBattleSettlementJob(ctx, workerID, job, err)
		return
	}
	run := payload.Run
	timeline := payload.Timeline
	finalResult := payload.FinalResult
	if run.RoomID != job.RoomID {
		s.quarantineBattleSettlementJob(ctx, workerID, job, fmt.Errorf("%w: job room %d does not match run room %d", store.ErrBattleSettlementRunDetached, job.RoomID, run.RoomID))
		return
	}
	switch run.Status {
	case store.BattleRunStatusEnded, store.BattleRunStatusFailed, store.BattleRunStatusInterrupted:
		if battleSettlementTerminalOutcomeConflict(run.Status, finalResult) {
			s.quarantineBattleSettlementJob(ctx, workerID, job, fmt.Errorf("terminal run status %q conflicts with final outcome %q", run.Status, battleOutcomeFromFinalResult(finalResult)))
			return
		}
		if err := battleSettlementCorePayloadError(finalResult, run.Status, nil); err != nil {
			s.quarantineBattleSettlementJob(ctx, workerID, job, err)
			return
		}
		if err := s.reconcileBattleSettlementPostFinalize(ctx, run, timeline, finalResult, time.Now()); err != nil {
			s.failBattleSettlementJob(ctx, workerID, job, err)
			return
		}
		s.markBattleSettlementJobDone(workerID, job)
		return
	}
	detail, err := s.loadBattleRoomDetailFresh(ctx, job.RoomID)
	if err != nil {
		s.failBattleSettlementJob(ctx, workerID, job, err)
		return
	}
	outcome, ok := battleSettlementExplicitOutcome(finalResult)
	if !ok {
		s.quarantineBattleSettlementJob(ctx, workerID, job, fmt.Errorf("settlement final outcome is missing or invalid: %v", finalResult["outcome"]))
		return
	}
	if err := battleSettlementCorePayloadError(finalResult, outcome, battleSettlementMemberUsers(detail.Members)); err != nil {
		s.quarantineBattleSettlementJob(ctx, workerID, job, err)
		return
	}
	if attachmentErr := battleSettlementRoomAttachmentError(detail, job); attachmentErr != nil {
		s.quarantineBattleSettlementJob(ctx, workerID, job, attachmentErr)
		return
	}
	if !battleRunReadyForSettlement(run) {
		s.battleMetrics.AddSettlementRejectedNotFinished()
		s.failBattleSettlementJob(ctx, workerID, job, errBattleSettlementPlaybackNotFinished)
		return
	}
	if err := s.settleBattleRunPlayback(ctx, detail, run, timeline, finalResult, time.Now()); err != nil {
		if errors.Is(err, store.ErrBattleRoomStateInvalid) || errors.Is(err, store.ErrBattleSettlementRunDetached) {
			alreadyDone, doneErr := s.battleRunFinalizeAlreadyApplied(ctx, job.RoomID, job.RunID, battleOutcomeFromFinalResult(finalResult))
			if doneErr != nil {
				s.failBattleSettlementJob(ctx, workerID, job, fmt.Errorf("verify finalized settlement: %w", doneErr))
				return
			}
			if alreadyDone {
				run.Status = battleOutcomeFromFinalResult(finalResult)
				if reconcileErr := s.reconcileBattleSettlementPostFinalize(ctx, run, timeline, finalResult, time.Now()); reconcileErr != nil {
					s.failBattleSettlementJob(ctx, workerID, job, reconcileErr)
					return
				}
				s.markBattleSettlementJobDone(workerID, job)
				return
			}
			if errors.Is(err, store.ErrBattleSettlementRunDetached) {
				s.quarantineBattleSettlementJob(ctx, workerID, job, err)
				return
			}
			freshDetail, detailErr := s.loadBattleRoomDetailFresh(ctx, job.RoomID)
			if detailErr != nil {
				s.failBattleSettlementJob(ctx, workerID, job, fmt.Errorf("verify settlement room state: %w", detailErr))
				return
			}
			if attachmentErr := battleSettlementRoomAttachmentError(freshDetail, job); attachmentErr != nil {
				s.quarantineBattleSettlementJob(ctx, workerID, job, attachmentErr)
				return
			}
		}
		s.failBattleSettlementJob(ctx, workerID, job, err)
		return
	}
	s.markBattleSettlementJobDone(workerID, job)
	elapsed := time.Since(startedAt)
	queueWait := startedAt.Sub(job.CreatedAt)
	if queueWait < 0 {
		queueWait = 0
	}
	if elapsed >= battleSettlementSlowLogThreshold || queueWait >= battleSettlementSlowLogThreshold {
		log.Printf("battle settlement job slow worker=%s job_id=%d room_id=%d run_id=%d attempts=%d queue_wait_ms=%d elapsed_ms=%d", workerID, job.ID, job.RoomID, job.RunID, job.Attempts, queueWait.Milliseconds(), elapsed.Milliseconds())
	}
}

func battleSettlementExplicitOutcome(finalResult map[string]any) (string, bool) {
	raw, ok := finalResult["outcome"].(string)
	if !ok {
		return "", false
	}
	outcome := strings.TrimSpace(strings.ToLower(raw))
	switch outcome {
	case store.BattleRunStatusEnded, store.BattleRunStatusFailed, store.BattleRunStatusInterrupted:
		return outcome, true
	default:
		return "", false
	}
}

type battleSettlementPayloadRoster struct {
	OwnedUsers     map[int64]struct{}
	PlayerUsers    map[int64]struct{}
	KittyOwnerByID map[int64]int64
	SkillIDsByUnit map[string]map[string]struct{}
}

type battleSettlementItemOwnerKey struct {
	UserID int64
	ItemID string
}

func battleSettlementMemberUsers(members []store.BattleRoomMember) map[int64]struct{} {
	users := make(map[int64]struct{}, len(members))
	for _, member := range members {
		if member.UserID > 0 {
			users[member.UserID] = struct{}{}
		}
	}
	return users
}

func battleSettlementCorePayloadError(finalResult map[string]any, outcome string, allowedUsers map[int64]struct{}) error {
	if outcome != store.BattleRunStatusEnded && outcome != store.BattleRunStatusFailed {
		return nil
	}
	rewardPlanRaw, ok := finalResult["rewardPlan"]
	if !ok || rewardPlanRaw == nil {
		return errors.New("settlement reward plan is missing")
	}
	rewardPlanJSON, err := json.Marshal(rewardPlanRaw)
	if err != nil {
		return fmt.Errorf("encode settlement reward plan: %w", err)
	}
	var rewardPlan []battleRewardGrant
	if err := json.Unmarshal(rewardPlanJSON, &rewardPlan); err != nil || rewardPlan == nil {
		if err != nil {
			return fmt.Errorf("decode settlement reward plan: %w", err)
		}
		return errors.New("settlement reward plan must be a non-null array")
	}
	seenUsers := make(map[int64]struct{}, len(rewardPlan))
	for _, reward := range rewardPlan {
		if reward.UserID <= 0 {
			return errors.New("settlement reward plan contains an invalid user")
		}
		if _, duplicate := seenUsers[reward.UserID]; duplicate {
			return fmt.Errorf("settlement reward plan contains duplicate user %d", reward.UserID)
		}
		if !battleSettlementRewardGrantValuesComplete(reward) {
			return fmt.Errorf("settlement reward plan contains invalid rewards for user %d", reward.UserID)
		}
		seenUsers[reward.UserID] = struct{}{}
	}
	roster, err := battleSettlementPayloadRosterFromFinalResult(finalResult)
	if err != nil {
		return err
	}
	for userID := range roster.OwnedUsers {
		if _, hasPlayer := roster.PlayerUsers[userID]; !hasPlayer {
			return fmt.Errorf("settlement owner %d has no player snapshot", userID)
		}
		if allowedUsers != nil {
			if _, allowed := allowedUsers[userID]; !allowed {
				return fmt.Errorf("settlement payload user %d is not a current room member", userID)
			}
		}
	}
	if allowedUsers != nil {
		for userID := range allowedUsers {
			if _, exists := roster.PlayerUsers[userID]; !exists {
				return fmt.Errorf("current room member %d is missing from settlement player snapshots", userID)
			}
		}
	}
	for _, reward := range rewardPlan {
		if _, exists := roster.PlayerUsers[reward.UserID]; !exists {
			return fmt.Errorf("settlement reward user %d is missing from player snapshots", reward.UserID)
		}
		for kittyID := range reward.KittyAttrExp {
			if ownerID := roster.KittyOwnerByID[kittyID]; ownerID != reward.UserID {
				return fmt.Errorf("settlement kitty %d does not belong to reward user %d", kittyID, reward.UserID)
			}
		}
	}
	return nil
}

func battleSettlementPayloadRosterFromFinalResult(finalResult map[string]any) (battleSettlementPayloadRoster, error) {
	stateKey, players, playerRows, err := battleSettlementPayloadPlayers(finalResult)
	if err != nil {
		return battleSettlementPayloadRoster{}, err
	}
	roster := battleSettlementPayloadRoster{
		OwnedUsers:     map[int64]struct{}{},
		PlayerUsers:    map[int64]struct{}{},
		KittyOwnerByID: map[int64]int64{},
		SkillIDsByUnit: map[string]map[string]struct{}{},
	}
	seenUnits := make(map[string]struct{}, len(players))
	seenItems := map[battleSettlementItemOwnerKey]struct{}{}
	for index, player := range players {
		unitID := strings.TrimSpace(player.UnitID)
		unitKey := strings.ToLower(unitID)
		if unitID == "" {
			return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s player %d unit id is missing", stateKey, index)
		}
		if _, duplicate := seenUnits[unitKey]; duplicate {
			return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s contains duplicate unit %q", stateKey, unitID)
		}
		seenUnits[unitKey] = struct{}{}
		if player.UserID < 0 {
			return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s unit %q has an invalid user", stateKey, unitID)
		}
		if player.UserID == 0 {
			if strings.HasPrefix(unitKey, "player:") || strings.HasPrefix(unitKey, "kitty:") {
				return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s formal unit %q has no owner", stateKey, unitID)
			}
			continue
		}
		unitType, formalID, identityErr := battleSettlementFormalUnitIdentity(player)
		if identityErr != nil {
			return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s unit %q identity is invalid: %w", stateKey, unitID, identityErr)
		}
		roster.OwnedUsers[player.UserID] = struct{}{}
		if unitType == "player" {
			if _, duplicate := roster.PlayerUsers[player.UserID]; duplicate {
				return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s contains duplicate player for user %d", stateKey, player.UserID)
			}
			roster.PlayerUsers[player.UserID] = struct{}{}
		} else {
			if existingOwner := roster.KittyOwnerByID[formalID]; existingOwner != 0 && existingOwner != player.UserID {
				return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s kitty %d has conflicting owners", stateKey, formalID)
			}
			roster.KittyOwnerByID[formalID] = player.UserID
		}
		fields := map[string]json.RawMessage{}
		if err := json.Unmarshal(playerRows[index], &fields); err != nil {
			return battleSettlementPayloadRoster{}, fmt.Errorf("settlement %s player %d is malformed", stateKey, index)
		}
		skillIDs, err := battleSettlementPlayerSkillsPayload(fields, stateKey, index)
		if err != nil {
			return battleSettlementPayloadRoster{}, err
		}
		roster.SkillIDsByUnit[unitKey] = skillIDs
		items, err := battleSettlementPlayerItemsPayload(fields, stateKey, index)
		if err != nil {
			return battleSettlementPayloadRoster{}, err
		}
		for _, item := range items {
			itemID := strings.TrimSpace(item.ItemID)
			key := battleSettlementItemOwnerKey{UserID: player.UserID, ItemID: itemID}
			if _, duplicate := seenItems[key]; duplicate {
				return battleSettlementPayloadRoster{}, fmt.Errorf("settlement user %d has duplicate battle item %q", player.UserID, itemID)
			}
			seenItems[key] = struct{}{}
		}
	}
	if len(roster.PlayerUsers) == 0 {
		return battleSettlementPayloadRoster{}, errors.New("settlement player carry state has no owned player")
	}
	if err := battleSettlementSkillCastsPayloadError(finalResult, roster.SkillIDsByUnit); err != nil {
		return battleSettlementPayloadRoster{}, err
	}
	return roster, nil
}

func battleSettlementPayloadPlayers(finalResult map[string]any) (string, []battleUnitState, []json.RawMessage, error) {
	for _, stateKey := range []string{"carryState", "finalState"} {
		state, ok := finalResult[stateKey].(map[string]any)
		if !ok || len(state) == 0 {
			continue
		}
		playersRaw, exists := state["players"]
		if !exists || playersRaw == nil {
			continue
		}
		playersJSON, err := json.Marshal(playersRaw)
		if err != nil {
			continue
		}
		var players []battleUnitState
		if err := json.Unmarshal(playersJSON, &players); err != nil || len(players) == 0 {
			continue
		}
		var playerRows []json.RawMessage
		if err := json.Unmarshal(playersJSON, &playerRows); err != nil || len(playerRows) != len(players) {
			return "", nil, nil, fmt.Errorf("settlement %s players are malformed", stateKey)
		}
		return stateKey, players, playerRows, nil
	}
	return "", nil, nil, errors.New("settlement player carry state is missing or invalid")
}

func battleSettlementFormalUnitIdentity(player battleUnitState) (string, int64, error) {
	unitID := strings.ToLower(strings.TrimSpace(player.UnitID))
	unitType := strings.ToLower(strings.TrimSpace(player.UnitType))
	prefix, rawID, ok := strings.Cut(unitID, ":")
	if !ok {
		return "", 0, errors.New("unit id is not canonical")
	}
	formalID, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
	if err != nil || formalID <= 0 {
		return "", 0, errors.New("unit id has an invalid numeric identity")
	}
	switch prefix {
	case "player":
		if formalID != player.UserID || (unitType != "" && unitType != "player") || player.KittyID != 0 {
			return "", 0, errors.New("player unit fields conflict")
		}
		return "player", formalID, nil
	case "kitty":
		if (unitType != "" && unitType != "kitty") || (player.KittyID > 0 && player.KittyID != formalID) {
			return "", 0, errors.New("kitty unit fields conflict")
		}
		return "kitty", formalID, nil
	default:
		return "", 0, errors.New("owned unit is neither a player nor a kitty")
	}
}

func battleSettlementPlayerSkillsPayload(fields map[string]json.RawMessage, stateKey string, index int) (map[string]struct{}, error) {
	raw, exists := fields["skills"]
	if !exists {
		return nil, fmt.Errorf("settlement %s player %d skills are missing", stateKey, index)
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return map[string]struct{}{}, nil
	}
	var skills []battleSkillState
	if err := json.Unmarshal(raw, &skills); err != nil || skills == nil {
		return nil, fmt.Errorf("settlement %s player %d skills must be an array or null", stateKey, index)
	}
	seen := make(map[string]struct{}, len(skills))
	for _, skill := range skills {
		skillID := strings.TrimSpace(skill.ID)
		skillKey := strings.ToLower(skillID)
		if skillID == "" || skill.Level <= 0 {
			return nil, fmt.Errorf("settlement %s player %d contains an invalid skill", stateKey, index)
		}
		if _, duplicate := seen[skillKey]; duplicate {
			return nil, fmt.Errorf("settlement %s player %d contains duplicate skill %q", stateKey, index, skillID)
		}
		seen[skillKey] = struct{}{}
	}
	return seen, nil
}

func battleSettlementSkillCastsPayloadError(finalResult map[string]any, skillIDsByUnit map[string]map[string]struct{}) error {
	maxSkillCasts := battleSkillExpMaxCastsPerUnit
	if rawBudget, exists := finalResult["compileBudgetSteps"]; exists {
		budget, ok := battleSettlementNonnegativeInt64(rawBudget)
		if !ok || budget <= 0 || budget > battleSkillExpMaxCastsPerUnit {
			return errors.New("settlement compile budget is invalid")
		}
		maxSkillCasts = budget
	}
	for _, rawStats := range []any{finalResult["battleStats"], finalResult["achievementStats"]} {
		stats := normalizeAnyMap(rawStats)
		castsByUnit := normalizeAnyMap(firstNonNil(stats["skillCastsByUnit"], stats["skill_casts_by_unit"]))
		if len(castsByUnit) == 0 {
			continue
		}
		for rawUnitID, rawPerSkill := range castsByUnit {
			unitID := strings.ToLower(strings.TrimSpace(rawUnitID))
			skillIDs, ownedUnit := skillIDsByUnit[unitID]
			for rawSkillID, rawCount := range normalizeAnyMap(rawPerSkill) {
				count, ok := battleSettlementNonnegativeInt64(rawCount)
				if !ok {
					return fmt.Errorf("settlement skill cast %q for unit %q has an invalid count", rawSkillID, rawUnitID)
				}
				if count == 0 {
					continue
				}
				if count > maxSkillCasts || count > math.MaxInt64/store.BattleSkillExpPerUse {
					return fmt.Errorf("settlement skill cast %q for unit %q exceeds the compile budget", rawSkillID, rawUnitID)
				}
				if !ownedUnit {
					if strings.HasPrefix(unitID, "player:") || strings.HasPrefix(unitID, "kitty:") {
						return fmt.Errorf("settlement skill casts reference missing owned unit %q", rawUnitID)
					}
					continue
				}
				if strings.EqualFold(strings.TrimSpace(rawSkillID), "basic_attack") {
					continue
				}
				skillID := strings.ToLower(strings.TrimSpace(rawSkillID))
				if _, exists := skillIDs[skillID]; !exists {
					return fmt.Errorf("settlement skill cast %q for unit %q is missing from its skill snapshot", rawSkillID, rawUnitID)
				}
			}
		}
		return nil
	}
	return nil
}

func battleSettlementPlayerItemsPayload(fields map[string]json.RawMessage, stateKey string, index int) ([]battleItemState, error) {
	raw, exists := fields["battleItems"]
	if !exists {
		return nil, nil
	}
	var items []battleItemState
	if strings.TrimSpace(string(raw)) == "null" || json.Unmarshal(raw, &items) != nil || items == nil {
		return nil, fmt.Errorf("settlement %s player %d battle items must be an array", stateKey, index)
	}
	for _, item := range items {
		if strings.TrimSpace(item.ItemID) == "" || item.Quantity < 0 {
			return nil, fmt.Errorf("settlement %s player %d contains an invalid battle item", stateKey, index)
		}
	}
	return items, nil
}

func battleSettlementRoomAttachmentError(detail store.BattleRoomDetail, job store.BattleSettlementJob) error {
	if detail.Room.CurrentRunID == nil || *detail.Room.CurrentRunID != job.RunID {
		return fmt.Errorf("%w: room current run %v does not match job run %d", store.ErrBattleSettlementRunDetached, detail.Room.CurrentRunID, job.RunID)
	}
	if detail.Room.Status != store.BattleRoomStatusPlaying {
		return fmt.Errorf("%w: room status %q is not playing for job run %d", store.ErrBattleSettlementRunDetached, detail.Room.Status, job.RunID)
	}
	return nil
}

func battleSettlementTerminalOutcomeConflict(runStatus string, finalResult map[string]any) bool {
	rawOutcome, exists := finalResult["outcome"]
	if !exists || rawOutcome == nil {
		return false
	}
	outcomeText, ok := rawOutcome.(string)
	if ok && strings.TrimSpace(outcomeText) == "" {
		return false
	}
	outcome, valid := battleSettlementExplicitOutcome(finalResult)
	if !valid {
		return true
	}
	return strings.TrimSpace(strings.ToLower(runStatus)) != outcome
}

func (s *Server) reconcileBattleSettlementPostFinalize(
	ctx context.Context,
	run store.BattleRun,
	timeline []wsbus.BattleTimelineEvent,
	finalResult map[string]any,
	now time.Time,
) error {
	if run.Status != store.BattleRunStatusEnded && run.Status != store.BattleRunStatusFailed {
		return nil
	}
	rewards := decodeBattleRewardPlan(finalResult)
	if err := s.enqueueBattleAttrExpJob(ctx, run.ID, rewards, now); err != nil {
		return err
	}
	if run.Status == store.BattleRunStatusEnded && len(rewards) > 0 {
		if err := s.applyBattleKillAchievementEvents(ctx, run.ID, rewards, finalResult, timeline); err != nil {
			return err
		}
	}
	_, skillExpRows, err := s.settleBattleSkillExpFromTimeline(ctx, timeline, finalResult)
	if err != nil {
		return err
	}
	if len(skillExpRows) > 0 {
		return s.enqueueBattleSkillExpJob(ctx, run.ID, skillExpRows, now)
	}
	return nil
}

func (s *Server) markBattleSettlementJobDone(workerID string, job store.BattleSettlementJob) {
	markCtx, markCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer markCancel()
	marked, err := s.deps.Store.MarkBattleSettlementJobDone(markCtx, job.ID, workerID, job.Attempts)
	if err != nil {
		log.Printf("battle settlement mark done failed worker=%s job_id=%d room_id=%d run_id=%d err=%v", workerID, job.ID, job.RoomID, job.RunID, err)
		return
	}
	if !marked {
		log.Printf("battle settlement mark done lease lost worker=%s job_id=%d room_id=%d run_id=%d attempts=%d", workerID, job.ID, job.RoomID, job.RunID, job.Attempts)
	}
}

func (s *Server) loadBattleSettlementRun(ctx context.Context, runID int64) (battleSettlementRunPayload, error) {
	minimal, err := s.deps.Store.GetBattleRunSettlementByID(ctx, runID)
	if err != nil {
		return battleSettlementRunPayload{}, err
	}
	return resolveBattleSettlementRun(minimal, func() (store.BattleRun, error) {
		return s.deps.Store.GetBattleRunByID(ctx, runID)
	})
}

func battleRunReadyForSettlement(run store.BattleRun) bool {
	state := run.State
	if len(state) == 0 {
		return false
	}
	stage := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", state["stage"])))
	if stage != "playback_finished" {
		return false
	}
	nextEventIdx := int(toInt64Loose(state["nextEventIdx"], 0))
	totalEvents := int(toInt64Loose(state["totalEvents"], 0))
	lastPushedSeq := toInt64Loose(state["lastPushedSeq"], 0)
	finalSeq := toInt64Loose(state["finalSeq"], 0)
	return nextEventIdx >= totalEvents && lastPushedSeq >= finalSeq
}

func (s *Server) failBattleSettlementJob(ctx context.Context, workerID string, job store.BattleSettlementJob, jobErr error) {
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
	marked, err := s.deps.Store.MarkBattleSettlementJobFailed(failCtx, job.ID, workerID, job.Attempts, jobErr.Error(), retryAt)
	if err != nil {
		log.Printf("battle settlement mark failed failed worker=%s job_id=%d room_id=%d run_id=%d err=%v original_err=%v", workerID, job.ID, job.RoomID, job.RunID, err, jobErr)
		return
	}
	if !marked {
		log.Printf("battle settlement mark failed lease lost worker=%s job_id=%d room_id=%d run_id=%d attempts=%d original_err=%v", workerID, job.ID, job.RoomID, job.RunID, job.Attempts, jobErr)
		return
	}
	log.Printf("battle settlement job failed worker=%s job_id=%d room_id=%d run_id=%d attempts=%d retry_at=%s err=%v", workerID, job.ID, job.RoomID, job.RunID, job.Attempts, retryAt.Format(time.RFC3339), jobErr)
}

func (s *Server) quarantineBattleSettlementJob(ctx context.Context, workerID string, job store.BattleSettlementJob, jobErr error) {
	quarantineCtx, quarantineCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer quarantineCancel()
	marked, err := s.deps.Store.MarkBattleSettlementJobQuarantined(quarantineCtx, job.ID, workerID, job.Attempts, jobErr.Error())
	if err != nil {
		log.Printf("battle settlement quarantine failed worker=%s job_id=%d room_id=%d run_id=%d err=%v original_err=%v", workerID, job.ID, job.RoomID, job.RunID, err, jobErr)
		return
	}
	if !marked {
		log.Printf("battle settlement quarantine lease lost worker=%s job_id=%d room_id=%d run_id=%d attempts=%d original_err=%v", workerID, job.ID, job.RoomID, job.RunID, job.Attempts, jobErr)
		return
	}
	log.Printf("battle settlement job quarantined worker=%s job_id=%d room_id=%d run_id=%d attempts=%d err=%v", workerID, job.ID, job.RoomID, job.RunID, job.Attempts, jobErr)
}
