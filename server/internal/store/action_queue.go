package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"

	"github.com/jackc/pgx/v5"
)

type ActionFastForwardSummary struct {
	CompletedSteps              int64                        `json:"completedSteps"`
	ItemRewards                 map[string]int64             `json:"itemRewards,omitempty"`
	ItemCosts                   map[string]int64             `json:"itemCosts,omitempty"`
	AttrExp                     map[string]float64           `json:"attrExp,omitempty"`
	ActorSummaries              []ActionActorSummary         `json:"actorSummaries,omitempty"`
	QueueCompleted              []int64                      `json:"queueCompleted,omitempty"`
	QueueCanceled               []ActionQueueCancellation    `json:"queueCanceled,omitempty"`
	EnhanceResults              []EnhanceAttemptResult       `json:"enhanceResults,omitempty"`
	WorkLoadoutUsages           []WorkLoadoutUsage           `json:"workLoadoutUsages,omitempty"`
	ActivatedWorkLoadouts       []ActivatedWorkLoadout       `json:"activatedWorkLoadouts,omitempty"`
	FinalWorkLoadoutID          string                       `json:"finalWorkLoadoutId,omitempty"`
	BudgetExhausted             bool                         `json:"budgetExhausted,omitempty"`
	HasMoreDue                  bool                         `json:"hasMoreDue,omitempty"`
	YieldReason                 string                       `json:"yieldReason,omitempty"`
	StalledQueueID              int64                        `json:"stalledQueueId,omitempty"`
	SettledUntilMS              int64                        `json:"settledUntilMS,omitempty"`
	TargetAtMS                  int64                        `json:"targetAtMS,omitempty"`
	FishingChanged              bool                         `json:"fishingChanged,omitempty"`
	FishingCatches              map[string]FishingCatchTally `json:"fishingCatches,omitempty"`
	RequiresImmediateProjection bool                         `json:"requiresImmediateProjection,omitempty"`
	OperationEffectsChanged     bool                         `json:"operationEffectsChanged,omitempty"`
}

const (
	maxActionQueueTotalCount              = 2_147_483_647
	minimumActionQueueRunDuration         = 2 * time.Second
	defaultActionQueueFastForwardMaxSteps = 10_000
	defaultActionQueueFastForwardMaxExec  = time.Second
	ActionQueueYieldReasonNoProgress      = "no_progress"
)

type actionQueueAdvanceOptions struct {
	AllowBatch     bool
	OfflineLog     bool
	SkipReadModels bool
	SkipQueueRead  bool
	SettleAt       time.Time
	Budget         FastForwardBudget
	Recovery       *OfflineRecovery
}

type workLoadoutActivationMode uint8

const (
	workLoadoutActivationImmediate workLoadoutActivationMode = iota
	workLoadoutActivationDeferred
)

type queueRunStartOptions struct {
	WorkLoadoutActivation workLoadoutActivationMode
	DeferredWorkLoadoutID string
}

type ActionActorSummary struct {
	ActorType        string             `json:"actorType"`
	ActorID          int64              `json:"actorId"`
	CompletedSteps   int64              `json:"completedSteps,omitempty"`
	ItemRewards      map[string]int64   `json:"itemRewards,omitempty"`
	ItemCosts        map[string]int64   `json:"itemCosts,omitempty"`
	AttrExp          map[string]float64 `json:"attrExp,omitempty"`
	OwnerAttrExp     map[string]float64 `json:"ownerAttrExp,omitempty"`
	EquipmentChanged bool               `json:"equipmentChanged,omitempty"`
}

type EnhanceAttemptResult struct {
	QueueID             int64              `json:"queueId"`
	ActionID            string             `json:"actionId"`
	AttemptNo           int                `json:"attemptNo"`
	EquipmentInstanceID int64              `json:"equipmentInstanceId"`
	TemplateItemID      string             `json:"templateItemId,omitempty"`
	BeforeLevel         int                `json:"beforeLevel"`
	AfterLevel          int                `json:"afterLevel"`
	Success             bool               `json:"success"`
	UsedProtect         bool               `json:"usedProtect"`
	BaseSuccessRate     float64            `json:"baseSuccessRate"`
	SuccessRateBonus    float64            `json:"successRateBonus"`
	SuccessRate         float64            `json:"successRate"`
	Costs               map[string]int64   `json:"costs"`
	DeltaAttrs          map[string]float64 `json:"deltaAttrs"`
	StopReason          string             `json:"stopReason,omitempty"`
}

type ActionQueueCancellation struct {
	QueueID  int64  `json:"queueId"`
	ActionID string `json:"actionId"`
	Reason   string `json:"reason"`
}

type ActionBatchStep struct {
	ActionID string
	Count    int
	ArgsRaw  json.RawMessage
}

const (
	ActionBatchInsertModeAppend            = "append"
	ActionBatchInsertModeRunNowLikePrepend = "run_now_like_prepend"
)

const (
	ActionQueueCancelReasonCostInsufficient    = "COST_INSUFFICIENT"
	ActionQueueCancelReasonStatusRequirement   = "STATUS_REQUIREMENT_NOT_MET"
	ActionQueueCancelReasonBuildingRequirement = "BUILDING_REQUIREMENT_NOT_MET"
	ActionQueueCancelReasonTargetInvalid       = "TARGET_INVALID"
	ActionQueueCancelReasonProtectInsuff       = "PROTECT_ITEM_INSUFFICIENT"
)

func (s *Store) GetAttributeExp(ctx context.Context, userID int64) (AttributeExp, error) {
	rows, err := s.db.Query(ctx, `
		SELECT attr_key, attr_exp, updated_at
		FROM player_attributes
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return AttributeExp{}, err
	}
	defer rows.Close()
	out := AttributeExp{Values: map[string]float64{}}
	for rows.Next() {
		var key string
		var exp float64
		var updatedAt time.Time
		if err := rows.Scan(&key, &exp, &updatedAt); err != nil {
			return AttributeExp{}, err
		}
		out.Values[key] = exp
		if updatedAt.After(out.UpdatedAt) {
			out.UpdatedAt = updatedAt
		}
	}
	return out, rows.Err()
}

func (s *Store) ListActionQueue(ctx context.Context, userID int64, limit int) ([]ActionQueueItem, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, actor_type, actor_id, action_id, mode, total_count, done_count, status, created_at, updated_at, current_run_started_at, current_run_ends_at, payload
		FROM player_action_queue
		WHERE user_id = $1 AND status IN ('pending', 'running')
		ORDER BY
		  CASE WHEN actor_type = 'player' THEN 0 ELSE 1 END,
		  actor_id ASC,
		  queue_order ASC,
		  id ASC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ActionQueueItem, 0, limit)
	for rows.Next() {
		var item ActionQueueItem
		var runStartAt *time.Time
		var runEndsAt *time.Time
		var payloadRaw []byte
		if err := rows.Scan(&item.ID, &item.ActorType, &item.ActorID, &item.ActionID, &item.Mode, &item.TotalCount, &item.DoneCount, &item.Status, &item.CreatedAt, &item.UpdatedAt, &runStartAt, &runEndsAt, &payloadRaw); err != nil {
			return nil, err
		}
		item.ActorType = normalizeActorType(item.ActorType)
		if item.ActorType == ActionActorTypePlayer {
			item.ActorID = 0
		}
		if runStartAt != nil {
			item.RunStartAt = *runStartAt
		}
		if runEndsAt != nil {
			item.RunEndsAt = *runEndsAt
		}
		item.Display = buildActionQueueDisplay(item.ActionID, payloadRaw)
		out = append(out, item)
	}
	return out, rows.Err()
}

const actionQueueScheduleSelectSQL = `
	SELECT actor_type, actor_id, status, current_run_ends_at
	FROM player_action_queue
	WHERE user_id = $1 AND status IN ('pending', 'running')
	ORDER BY
	  CASE WHEN actor_type = 'player' THEN 0 ELSE 1 END,
	  actor_id ASC,
	  queue_order ASC,
	  id ASC
	LIMIT $2
`

func (s *Store) ListActionQueueSchedule(ctx context.Context, userID int64, limit int) ([]ActionQueueScheduleItem, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, actionQueueScheduleSelectSQL, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ActionQueueScheduleItem, 0, limit)
	for rows.Next() {
		var item ActionQueueScheduleItem
		var runEndsAt *time.Time
		if err := rows.Scan(&item.ActorType, &item.ActorID, &item.Status, &runEndsAt); err != nil {
			return nil, err
		}
		item.ActorType = normalizeActorType(item.ActorType)
		if item.ActorType == ActionActorTypePlayer {
			item.ActorID = 0
		}
		if runEndsAt != nil {
			item.RunEndsAt = *runEndsAt
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) checkActionQueueLaneLimitTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, additional int) error {
	if additional <= 0 {
		return nil
	}
	normalizedActorType := normalizeActorType(actorType)
	normalizedActorID := actorID
	if normalizedActorType == ActionActorTypePlayer {
		normalizedActorID = 0
	}
	laneLimit := s.actionQueueLaneLimit(normalizedActorType)
	var queuedCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM player_action_queue
		WHERE user_id = $1
		  AND actor_type = $2
		  AND actor_id = $3
		  AND status IN ('pending', 'running')
	`, userID, normalizedActorType, normalizedActorID).Scan(&queuedCount); err != nil {
		return err
	}
	if actionQueueLaneLimitExceeded(queuedCount, additional, laneLimit) {
		return ErrActionQueueLaneLimitReached
	}
	return nil
}

func (s *Store) actionQueueLaneLimit(actorType string) int {
	if normalizeActorType(actorType) == ActionActorTypeKitty {
		if s.config.KittyActionQueueLaneLimit > 0 {
			return s.config.KittyActionQueueLaneLimit
		}
		return 20
	}
	if s.config.ActionQueueLaneLimit > 0 {
		return s.config.ActionQueueLaneLimit
	}
	return 30
}

func actionQueueLaneLimitExceeded(queuedCount int, additional int, laneLimit int) bool {
	if laneLimit <= 0 {
		return false
	}
	return queuedCount+additional > laneLimit
}

func normalizeActionBatchInsertMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case ActionBatchInsertModeRunNowLikePrepend:
		return ActionBatchInsertModeRunNowLikePrepend
	default:
		return ActionBatchInsertModeAppend
	}
}

func (s *Store) refreshQueueDurationsTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	specs := BuildActionSpecMap(s.gameData)
	type queueDurationRefreshRow struct {
		queueID             int64
		actorType           string
		actorID             int64
		actionID            string
		status              string
		currentRunStartedAt *time.Time
		payloadRaw          []byte
	}
	rows, err := tx.Query(ctx, `
		SELECT id, actor_type, actor_id, action_id, status, current_run_started_at, payload
		FROM player_action_queue
		WHERE user_id = $1
		  AND status IN ('pending', 'running')
		ORDER BY queue_order ASC, id ASC
		FOR UPDATE
	`, userID)
	if err != nil {
		return err
	}
	refreshRows := make([]queueDurationRefreshRow, 0, 32)
	for rows.Next() {
		var row queueDurationRefreshRow
		if err := rows.Scan(&row.queueID, &row.actorType, &row.actorID, &row.actionID, &row.status, &row.currentRunStartedAt, &row.payloadRaw); err != nil {
			rows.Close()
			return err
		}
		refreshRows = append(refreshRows, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, row := range refreshRows {
		spec, ok := specs[row.actionID]
		if !ok {
			continue
		}
		refresher, ok := queueDurationRefresherForStatus(s.actionHandlerForType(row.actionID), row.status)
		if !ok {
			continue
		}
		nextPayload, changed, err := refresher.refreshQueueDurationTx(ctx, tx, userID, QueueRuntimeItem{
			QueueID:    row.queueID,
			ActorType:  row.actorType,
			ActorID:    row.actorID,
			ActionID:   row.actionID,
			Spec:       spec,
			PayloadRaw: row.payloadRaw,
		})
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if strings.TrimSpace(row.status) == "running" && row.currentRunStartedAt != nil {
			nextDuration := parseQueueRuntimeDuration(nextPayload)
			if nextDuration <= 0 {
				nextDuration = minimumActionQueueRunDuration
			}
			nextRunEndsAt := row.currentRunStartedAt.Add(nextDuration)
			if _, err := tx.Exec(ctx, `
				UPDATE player_action_queue
				SET payload = $2::jsonb,
				    current_run_ends_at = $3,
				    updated_at = now()
				WHERE id = $1
			`, row.queueID, string(nextPayload), nextRunEndsAt); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_action_queue
			SET payload = $2::jsonb,
			    updated_at = now()
			WHERE id = $1
		`, row.queueID, string(nextPayload)); err != nil {
			return err
		}
	}
	return nil
}

func resolveQueueInsertCount(actionID string, requestedCount int, normalizedArgs any) (int, error) {
	if requestedCount <= 0 {
		return 0, ErrInvalidActionCount
	}
	insertCount := requestedCount
	if canArgsOverrideQueueInsertCount(actionID) {
		provider, ok := normalizedArgs.(queueTotalCountProvider)
		if !ok {
			return 0, ErrInvalidActionCount
		}
		insertCount = provider.QueueTotalCount()
	}
	if insertCount <= 0 {
		return 0, ErrInvalidActionCount
	}
	if insertCount > maxActionQueueTotalCount {
		return 0, ErrInvalidActionCount
	}
	if strings.TrimSpace(actionID) == "alchemy.inherit" && insertCount != 1 {
		return 0, ErrInvalidActionCount
	}
	if args, ok := normalizedArgs.(alchemyBenchArgs); ok && args.FishInstanceID > 0 && insertCount != 1 {
		return 0, ErrInvalidActionCount
	}
	return insertCount, nil
}

func canArgsOverrideQueueInsertCount(actionID string) bool {
	switch strings.TrimSpace(actionID) {
	case "equipment_enhance":
		return true
	default:
		return false
	}
}

func buildActionQueueDisplay(actionID string, payloadRaw []byte) *ActionQueueDisplay {
	kind := strings.TrimSpace(actionID)
	if kind == "" {
		return nil
	}
	display := &ActionQueueDisplay{
		Kind:                kind,
		EffectiveDurationMs: int64(parseQueueRuntimeDuration(payloadRaw) / time.Millisecond),
	}
	switch kind {
	case FishingTripActionID:
		runtime, ok := parseFishingExecutorRuntime(payloadRaw)
		if !ok {
			return display
		}
		lineState := ""
		if runtime.CurrentFish != nil && runtime.CurrentLineLoad > 0 {
			lineState = fishingLineState(runtime.CurrentLineLoad, runtime.Loadout.MaxLoadKG, runtime.SafetyLoadPercent)
		}
		remainingSupplies := runtime.Supplies.Remaining
		if runtime.Phase == "pending" {
			remainingSupplies = runtime.Supplies.Initial
		}
		display.Fishing = &ActionQueueFishingDisplay{
			TripID:                 strings.TrimSpace(runtime.TripID),
			WaterID:                strings.TrimSpace(runtime.WaterID),
			SpotID:                 strings.TrimSpace(runtime.SpotID),
			LoadoutName:            strings.TrimSpace(runtime.Loadout.Name),
			Method:                 strings.TrimSpace(runtime.Loadout.Method),
			Strategy:               strings.TrimSpace(runtime.Strategy),
			Phase:                  strings.TrimSpace(runtime.Phase),
			LineState:              lineState,
			PlannedDurationSeconds: runtime.PlannedDurationSeconds,
			StartedAtMS:            runtime.StartedAtMS,
			PlannedEndAtMS:         runtime.PlannedEndAtMS,
			PhaseEndsAtMS:          runtime.PhaseEndsAtMS,
			Landed:                 runtime.Summary.Landed,
			Released:               runtime.Summary.Released,
			TotalWeightKG:          runtime.Summary.TotalWeight,
			InitialSupplies:        runtime.Supplies.Initial,
			RemainingSupplies:      remainingSupplies,
		}
	case "equipment_enhance":
		runtime, ok := parseEnhanceExecutorRuntime(payloadRaw)
		if !ok {
			if fallbackRuntime, fallbackOK := parseEnhanceRuntimeLoose(payloadRaw); fallbackOK {
				runtime = fallbackRuntime
				ok = true
			}
		}
		if !ok {
			return display
		}
		display.Enhance = &ActionQueueEnhanceDisplay{
			EquipmentInstanceID: runtime.EquipmentInstanceID,
			TemplateItemID:      strings.TrimSpace(runtime.TemplateID),
			TargetEnhanceLevel:  runtime.TargetEnhanceLevel,
			UseProtect:          runtime.UseProtect,
			ProtectStartLevel:   runtime.ProtectStartLevel,
			ProtectItemID:       strings.TrimSpace(runtime.ProtectItemID),
		}
	case "building_upgrade":
		runtime, ok := parseBuildingExecutorRuntime(payloadRaw)
		if !ok {
			return display
		}
		fromLevel := runtime.FromLevel
		toLevel := runtime.ToLevel
		if runCtx, runCtxOK := parseBuildingRunContext(payloadRaw); runCtxOK {
			fromLevel = runCtx.FromLevel
			toLevel = runCtx.ToLevel
		}
		durationSeconds := display.EffectiveDurationMs / int64(time.Second/time.Millisecond)
		display.Building = &ActionQueueBuildingDisplay{
			BuildingID: strings.TrimSpace(runtime.BuildingID),
			FromLevel:  fromLevel,
			ToLevel:    toLevel,
			DurationS:  durationSeconds,
		}
	case "alchemy.convert":
		var args alchemyConvertArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err == nil {
			display.Alchemy = &ActionQueueAlchemyDisplay{
				SubKind:      "convert",
				SourceItemID: args.SourceItemID,
				GroupID:      args.GroupID,
				SourceCount:  1,
			}
		}
	case "alchemy.bench":
		var args alchemyBenchArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err == nil {
			display.Alchemy = &ActionQueueAlchemyDisplay{
				SubKind:           "bench",
				CarrierInstanceID: args.CarrierInstanceID,
				CarrierItemID:     args.CarrierItemID,
				Materials:         args.Materials,
				ComboRecipeID:     args.ComboRecipeID,
				MediumItemID:      args.MediumItemID,
				FishInstanceID:    args.FishInstanceID,
			}
		}
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err == nil {
			display.Alchemy = &ActionQueueAlchemyDisplay{
				SubKind:    "transmute",
				ItemID:     args.ItemID,
				InstanceID: args.InstanceID,
				TotalCount: 1,
			}
		}
	case "alchemy.inherit":
		var args alchemyInheritArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err == nil {
			inheritCount := 0
			suppressCount := 0
			for _, choice := range args.GeneChoices {
				if choice.Decision == inheritDecisionInherit {
					inheritCount++
				}
				if choice.Decision == inheritDecisionSuppress {
					suppressCount++
				}
			}
			display.Alchemy = &ActionQueueAlchemyDisplay{
				SubKind:           "inherit",
				CarrierInstanceID: args.CarrierInstanceID,
				MediumItemID:      args.MediumItemID,
				FatherInstanceID:  args.FatherInstanceID,
				FatherAffixIndex:  args.FatherAffixIndex,
				MotherInstanceID:  args.MotherInstanceID,
				MotherAffixIndex:  args.MotherAffixIndex,
				GeneChoiceCount:   len(args.GeneChoices),
				InheritGeneCount:  inheritCount,
				SuppressGeneCount: suppressCount,
			}
		}
	}
	return display
}

func parseEnhanceRuntimeLoose(payloadRaw []byte) (enhanceExecutorRuntime, bool) {
	if len(payloadRaw) == 0 {
		return enhanceExecutorRuntime{}, false
	}
	var envelope map[string]any
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return enhanceExecutorRuntime{}, false
	}
	rawRuntime, ok := envelope["runtime"]
	if !ok {
		return enhanceExecutorRuntime{}, false
	}
	runtimeMap, ok := rawRuntime.(map[string]any)
	if !ok {
		return enhanceExecutorRuntime{}, false
	}
	out := enhanceExecutorRuntime{
		TemplateID:          toString(runtimeMap["templateId"]),
		EquipmentInstanceID: toInt64(runtimeMap["equipmentInstanceId"]),
		DurationMs:          toInt64(runtimeMap["durationMs"]),
		MaxLevel:            int(toInt64(runtimeMap["maxLevel"])),
		TargetEnhanceLevel:  int(toInt64(runtimeMap["targetEnhanceLevel"])),
		UseProtect:          toBool(runtimeMap["useProtect"]),
		ProtectStartLevel:   int(toInt64(runtimeMap["protectStartLevel"])),
		ProtectItemID:       toString(runtimeMap["protectItemId"]),
	}
	if strings.TrimSpace(out.TemplateID) == "" || out.EquipmentInstanceID <= 0 {
		return enhanceExecutorRuntime{}, false
	}
	return out, true
}

func toString(raw any) string {
	value, _ := raw.(string)
	return strings.TrimSpace(value)
}

func toBool(raw any) bool {
	value, _ := raw.(bool)
	return value
}

func toInt64(raw any) int64 {
	switch v := raw.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func (s *Store) RunAction(ctx context.Context, userID int64, specs map[string]ActionSpec, actionID string, mode string, count int, argsRaw json.RawMessage, actorType string, actorID int64) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	prepared, err := s.prepareActionQueueStep(specs, ActionBatchStep{ActionID: actionID, Count: count, ArgsRaw: argsRaw})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	if mode != "run" && mode != "queue" {
		mode = "queue"
	}
	var queueID int64
	resolvedActorType := ActionActorTypePlayer
	resolvedActorID := int64(0)
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertUserNotBannedTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_action_queue
			WHERE user_id = $1 AND status IN ('done', 'failed')
		`, userID); err != nil {
			return err
		}
		normalizedType, normalizedID, err := s.resolveActionActorTx(ctx, tx, userID, actorType, actorID)
		if err != nil {
			return err
		}
		resolvedActorType = normalizedType
		resolvedActorID = normalizedID
		if isAlchemyActionID(actionID) && resolvedActorType != ActionActorTypePlayer {
			return ErrAlchemyArgsInvalid
		}
		if resolvedActorType == ActionActorTypeKitty {
			kittyStatus, err := s.getOwnedKittyStatusTx(ctx, tx, userID, resolvedActorID, true)
			if err != nil {
				return err
			}
			if kittyStatus != KittyStatusInLifeQueue && kittyStatus != KittyStatusInCombat {
				if err := s.setOwnedKittyStatusTx(ctx, tx, userID, resolvedActorID, KittyStatusInLifeQueue); err != nil {
					return err
				}
			}
		}
		if err := s.checkActionQueueLaneLimitTx(ctx, tx, userID, resolvedActorType, resolvedActorID, 1); err != nil {
			return err
		}
		payload, err := prepared.executor.FreezeRunContextTx(ctx, tx, userID, resolvedActorType, resolvedActorID, prepared.spec, prepared.args, time.Now())
		if err != nil {
			return err
		}

		if err = tx.QueryRow(ctx, `
			INSERT INTO player_action_queue(user_id, actor_type, actor_id, action_id, action_type, mode, total_count, done_count, status, payload, created_at, updated_at)
			VALUES($1, $2, $3, $4, $5, $6, $7, 0, 'pending', $8::jsonb, now(), now())
			RETURNING id
		`, userID, resolvedActorType, resolvedActorID, actionID, actionID, mode, prepared.insertCount, string(payload)).Scan(&queueID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE player_action_queue
			SET queue_order = $2
			WHERE id = $1
		`, queueID, queueID); err != nil {
			return err
		}
		if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return err
		}
		return s.reconcileKittyLaneStatusesTx(ctx, tx, userID)
	})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	if mode == "run" {
		return s.RunQueueItemNowWithSummary(ctx, userID, specs, queueID)
	}
	return s.AdvanceActionQueueWithSummary(ctx, userID, specs)
}

func (s *Store) EnqueueActionBatch(ctx context.Context, userID int64, specs map[string]ActionSpec, steps []ActionBatchStep, actorType string, actorID int64, insertMode string) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, int, error) {
	if len(steps) == 0 {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, 0, ErrInvalidActionCount
	}
	preparedSteps := make([]preparedActionQueueStep, len(steps))
	for idx, step := range steps {
		prepared, err := s.prepareActionQueueStep(specs, step)
		if err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, 0, err
		}
		preparedSteps[idx] = prepared
	}
	normalizedInsertMode := normalizeActionBatchInsertMode(insertMode)
	resolvedActorType := ActionActorTypePlayer
	resolvedActorID := int64(0)
	droppedCount := int64(0)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertUserNotBannedTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_action_queue
			WHERE user_id = $1 AND status IN ('done', 'failed')
		`, userID); err != nil {
			return err
		}
		normalizedType, normalizedID, err := s.resolveActionActorTx(ctx, tx, userID, actorType, actorID)
		if err != nil {
			return err
		}
		resolvedActorType = normalizedType
		resolvedActorID = normalizedID
		if resolvedActorType == ActionActorTypeKitty {
			kittyStatus, err := s.getOwnedKittyStatusTx(ctx, tx, userID, resolvedActorID, true)
			if err != nil {
				return err
			}
			if kittyStatus != KittyStatusInLifeQueue && kittyStatus != KittyStatusInCombat {
				if err := s.setOwnedKittyStatusTx(ctx, tx, userID, resolvedActorID, KittyStatusInLifeQueue); err != nil {
					return err
				}
			}
		}
		if normalizedInsertMode == ActionBatchInsertModeAppend {
			if err := s.checkActionQueueLaneLimitTx(ctx, tx, userID, resolvedActorType, resolvedActorID, len(steps)); err != nil {
				return err
			}
		}
		now := time.Now()
		minOrder := int64(0)
		var runningID int64
		var runningActionID string
		var runningTotalCount int
		var runningDoneCount int
		var runningPayloadRaw []byte
		hasRunning := false
		if normalizedInsertMode == ActionBatchInsertModeRunNowLikePrepend {
			if err := tx.QueryRow(ctx, `
				SELECT COALESCE(MIN(queue_order), 0)
				FROM player_action_queue
				WHERE user_id = $1
				  AND actor_type = $2
				  AND actor_id = $3
				  AND status IN ('pending', 'running')
			`, userID, resolvedActorType, resolvedActorID).Scan(&minOrder); err != nil {
				return err
			}
			err := tx.QueryRow(ctx, `
				SELECT id, action_id, total_count, done_count, payload
				FROM player_action_queue
				WHERE user_id = $1
				  AND actor_type = $2
				  AND actor_id = $3
				  AND status = 'running'
				ORDER BY queue_order ASC, id ASC
				LIMIT 1
				FOR UPDATE
			`, userID, resolvedActorType, resolvedActorID).Scan(&runningID, &runningActionID, &runningTotalCount, &runningDoneCount, &runningPayloadRaw)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				hasRunning = true
			}
		}
		nextHeadOrder := minOrder - int64(len(steps)+2)
		for idx, step := range steps {
			prepared := preparedSteps[idx]
			if isAlchemyActionID(step.ActionID) && resolvedActorType != ActionActorTypePlayer {
				return ErrAlchemyArgsInvalid
			}
			payload, err := prepared.executor.FreezeRunContextTx(ctx, tx, userID, resolvedActorType, resolvedActorID, prepared.spec, prepared.args, now)
			if err != nil {
				return err
			}
			var queueID int64
			if err = tx.QueryRow(ctx, `
				INSERT INTO player_action_queue(user_id, actor_type, actor_id, action_id, action_type, mode, total_count, done_count, status, payload, created_at, updated_at)
				VALUES($1, $2, $3, $4, $5, 'queue', $6, 0, 'pending', $7::jsonb, now(), now())
				RETURNING id
			`, userID, resolvedActorType, resolvedActorID, step.ActionID, step.ActionID, prepared.insertCount, string(payload)).Scan(&queueID); err != nil {
				return err
			}
			queueOrder := queueID
			if normalizedInsertMode == ActionBatchInsertModeRunNowLikePrepend {
				queueOrder = nextHeadOrder + int64(idx)
			}
			if _, err = tx.Exec(ctx, `
				UPDATE player_action_queue
				SET queue_order = $2
				WHERE id = $1
			`, queueID, queueOrder); err != nil {
				return err
			}
		}
		if normalizedInsertMode == ActionBatchInsertModeRunNowLikePrepend && hasRunning {
			if runningSpec, ok := specs[runningActionID]; ok {
				runningRuntime := QueueRuntimeItem{
					QueueID:    runningID,
					ActorType:  resolvedActorType,
					ActorID:    resolvedActorID,
					ActionID:   runningActionID,
					Spec:       runningSpec,
					TotalCount: runningTotalCount,
					DoneCount:  runningDoneCount,
					PayloadRaw: runningPayloadRaw,
				}
				if cancelErr := s.actionHandlerForType(runningActionID).CancelRunTx(ctx, tx, userID, runningRuntime); cancelErr != nil {
					return cancelErr
				}
			}
			if _, err = tx.Exec(ctx, `
				UPDATE player_action_queue
				SET status = 'pending',
				    queue_order = $2,
				    current_run_started_at = NULL,
				    current_run_ends_at = NULL,
				    updated_at = now()
				WHERE id = $1
			`, runningID, minOrder-1); err != nil {
				return err
			}
		}
		if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return err
		}
		if normalizedInsertMode == ActionBatchInsertModeRunNowLikePrepend {
			tag, err := tx.Exec(ctx, `
				WITH ordered AS (
					SELECT id,
					       ROW_NUMBER() OVER (ORDER BY queue_order ASC, id ASC) AS row_no
					FROM player_action_queue
					WHERE user_id = $1
					  AND actor_type = $2
					  AND actor_id = $3
					  AND status IN ('pending', 'running')
				),
				to_delete AS (
					SELECT id
					FROM ordered
					WHERE row_no > $4
				)
				DELETE FROM player_action_queue q
				USING to_delete d
				WHERE q.id = d.id
			`, userID, resolvedActorType, resolvedActorID, s.actionQueueLaneLimit(resolvedActorType))
			if err != nil {
				return err
			}
			droppedCount = tag.RowsAffected()
			if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
				return err
			}
			return s.reconcileKittyLaneStatusesTx(ctx, tx, userID)
		}
		return s.reconcileKittyLaneStatusesTx(ctx, tx, userID)
	})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, 0, err
	}
	queue, state, attr, summary, advanceErr := s.AdvanceActionQueueWithSummary(ctx, userID, specs)
	if advanceErr != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, 0, advanceErr
	}
	return queue, state, attr, summary, int(droppedCount), nil
}

func (s *Store) AdvanceActionQueue(ctx context.Context, userID int64, specs map[string]ActionSpec) ([]ActionQueueItem, PlayerState, AttributeExp, error) {
	queue, state, attr, _, err := s.advanceActionQueueInternal(ctx, userID, specs, actionQueueAdvanceOptions{})
	return queue, state, attr, err
}

func (s *Store) AdvanceActionQueueWithSummary(ctx context.Context, userID int64, specs map[string]ActionSpec) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	return s.advanceActionQueueInternal(ctx, userID, specs, actionQueueAdvanceOptions{})
}

// AdvanceActionQueueForWorker omits all gateway read models and returns only
// the fields needed to rebuild Redis's derived due index.
func (s *Store) AdvanceActionQueueForWorker(ctx context.Context, userID int64, specs map[string]ActionSpec) ([]ActionQueueScheduleItem, ActionFastForwardSummary, error) {
	_, _, _, summary, err := s.advanceActionQueueInternal(ctx, userID, specs, actionQueueAdvanceOptions{
		SkipReadModels: true,
		SkipQueueRead:  true,
	})
	if err != nil {
		return nil, ActionFastForwardSummary{}, err
	}
	schedule, err := s.ListActionQueueSchedule(ctx, userID, 50)
	return schedule, summary, err
}

func (s *Store) AdvanceActionQueueWithSummaryMode(ctx context.Context, userID int64, specs map[string]ActionSpec, allowBatch bool) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	return s.advanceActionQueueInternal(ctx, userID, specs, actionQueueAdvanceOptions{AllowBatch: allowBatch})
}

func (s *Store) AdvanceActionQueueOfflineWithSummary(ctx context.Context, userID int64, specs map[string]ActionSpec) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	return s.AdvanceActionQueueOfflineUntil(ctx, userID, specs, time.Now(), FastForwardBudget{})
}

func (s *Store) AdvanceActionQueueOfflineUntil(ctx context.Context, userID int64, specs map[string]ActionSpec, targetAt time.Time, budget FastForwardBudget) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	if targetAt.IsZero() {
		targetAt = time.Now()
	}
	return s.advanceActionQueueInternal(ctx, userID, specs, actionQueueAdvanceOptions{
		AllowBatch: true,
		OfflineLog: true,
		SettleAt:   targetAt,
		Budget:     budget,
	})
}

// AdvanceActionQueueOfflineChunk commits one bounded offline settlement chunk
// without coupling that commit to gateway read-model refreshes. Callers must
// load projections separately after this method succeeds.
func (s *Store) AdvanceActionQueueOfflineChunk(ctx context.Context, userID int64, specs map[string]ActionSpec, targetAt time.Time, budget FastForwardBudget) (ActionFastForwardSummary, error) {
	if targetAt.IsZero() {
		targetAt = time.Now()
	}
	_, _, _, summary, err := s.advanceActionQueueInternal(ctx, userID, specs, actionQueueAdvanceOptions{
		AllowBatch:     true,
		OfflineLog:     true,
		SkipReadModels: true,
		SkipQueueRead:  true,
		SettleAt:       targetAt,
		Budget:         budget,
	})
	return summary, err
}

func (s *Store) advanceActionQueueInternal(ctx context.Context, userID int64, specs map[string]ActionSpec, opts actionQueueAdvanceOptions) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	s.registerInventoryAuditBuffer(tx)
	defer s.discardInventoryAuditBuffer(tx)
	defer tx.Rollback(ctx)
	if opts.Recovery != nil {
		locked, lockErr := s.tryLockUserTx(ctx, tx, userID)
		if lockErr != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, lockErr
		}
		if !locked {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, ErrOfflineRecoveryBusy
		}
		recovery, current, loadErr := s.loadRecoveryForAdvanceTx(ctx, tx, *opts.Recovery)
		if loadErr != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, loadErr
		}
		if !current {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, nil
		}
		opts.Recovery = &recovery
		opts.SettleAt = recovery.TargetAt
	} else {
		if err = s.lockUserTx(ctx, tx, userID); err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
		if err = s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
	}
	if err := s.assertUserNotBannedTx(ctx, tx, userID); err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	var achievementBatch *offlineAchievementBatch
	if opts.Recovery != nil {
		achievementBatch = &offlineAchievementBatch{store: s, tx: tx, userID: userID}
		ctx = context.WithValue(ctx, offlineAchievementBatchKey{}, achievementBatch)
	}
	summary := ActionFastForwardSummary{
		ItemRewards:    make(map[string]int64),
		ItemCosts:      make(map[string]int64),
		AttrExp:        make(map[string]float64),
		FishingCatches: make(map[string]FishingCatchTally),
	}
	actorSummaryByLane := map[string]*ActionActorSummary{}
	var pendingAchievementEvents []AchievementEvent

	allowBatch := opts.AllowBatch
	offlineLog := opts.OfflineLog
	realNow := time.Now()
	settleAt := opts.SettleAt
	if settleAt.IsZero() {
		settleAt = realNow
	}
	if offlineLog {
		summary.TargetAtMS = settleAt.UnixMilli()
	}
	effectSession, err := s.loadOperationEffectSessionTx(ctx, tx, userID, settleAt)
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	loopStartedAt := realNow
	maxSteps := opts.Budget.MaxIterations
	if maxSteps <= 0 {
		maxSteps = s.config.ActionQueueFastForwardMaxSteps
		if maxSteps <= 0 {
			maxSteps = defaultActionQueueFastForwardMaxSteps
		}
	}
	maxExec := opts.Budget.MaxExec
	if maxExec <= 0 {
		maxExec = s.config.ActionQueueFastForwardMaxExec
		if maxExec <= 0 {
			maxExec = defaultActionQueueFastForwardMaxExec
		}
	}
	budgetStartedAt := opts.Budget.StartedAt
	if budgetStartedAt.IsZero() {
		budgetStartedAt = loopStartedAt
	}
	carryStartAtByLane := map[string]time.Time{}
	lastProgressAt := time.Time{}
	stalledPlayerLane := false
	budgetExhausted := false
	queueRowRemoved := false
	maxLoopIterations := maxSteps + 128
	if maxLoopIterations < 256 {
		maxLoopIterations = 256
	}
	completedBudgetRemaining := maxSteps
	loopHitGuard := true
	alchemyReads := offlineAlchemyReads{}
	alchemySegments, alchemySegmentSteps := 0, 0
	ordinaryOrdering := false
	for i := 0; i < maxLoopIterations; i++ {
		hasCarryLane := len(carryStartAtByLane) > 0
		if offlineLog && completedBudgetRemaining <= 0 && !hasCarryLane {
			budgetExhausted = true
			break
		}
		handoffOnly := offlineLog &&
			hasCarryLane &&
			(completedBudgetRemaining <= 0 || actionQueueAdvanceTimeBudgetExpired(loopStartedAt, maxExec))
		if handoffOnly {
			// Materialize each completed lane's next running row before yielding.
			// This persists both its timeline carry and deferred orchestration data
			// without settling another player-visible step beyond the budget.
			budgetExhausted = true
		} else if shouldYieldActionQueueAdvanceBudget(loopStartedAt, maxExec, carryStartAtByLane) {
			budgetExhausted = true
			break
		}
		materializeCarry := handoffOnly || (hasCarryLane && (ordinaryOrdering || effectSession.Passive() != nil && effectSession.Passive().ProgressOrdering))
		var runningID int64
		var runningActorType string
		var runningActorID int64
		var runningActionID string
		var totalCount int
		var doneCount int
		var payloadRaw []byte
		var runStartAt *time.Time
		var runEndsAt *time.Time
		var otherProgressBoundary *time.Time
		var otherRunEnd *time.Time
		err = pgx.ErrNoRows
		if !materializeCarry {
			progressColumn := ""
			if p := effectSession.Passive(); p != nil && p.ProgressOrdering {
				progressColumn = `,
				 (SELECT MIN(COALESCE(q.current_run_ends_at,q.current_run_started_at,'1970-01-01'::timestamptz)) FROM player_action_queue q
				 WHERE q.user_id=$1 AND q.status='running' AND q.actor_type='player' AND q.id<>player_action_queue.id)`
			}
			query := `
				SELECT id, actor_type, actor_id, action_id, total_count, done_count, payload, current_run_started_at, current_run_ends_at,
				 (SELECT MIN(COALESCE(q.current_run_ends_at,q.current_run_started_at)) FROM player_action_queue q
				  WHERE q.user_id=$1 AND q.status='running' AND q.id<>player_action_queue.id
				    AND NOT (q.actor_type='kitty' AND EXISTS (SELECT 1 FROM player_kitties k WHERE k.user_id=q.user_id AND k.id=q.actor_id AND k.status=$3)))` + progressColumn + `
				FROM player_action_queue
				WHERE user_id = $1
				  AND status = 'running'
				  AND (current_run_ends_at IS NULL OR current_run_ends_at <= $2)
				  AND NOT (
				    actor_type = 'kitty'
				    AND EXISTS (
				      SELECT 1
				      FROM player_kitties k
				      WHERE k.user_id = player_action_queue.user_id
				        AND k.id = player_action_queue.actor_id
				        AND k.status = $3
				    )
				  )
				ORDER BY current_run_ends_at ASC NULLS FIRST, queue_order ASC, id ASC
				LIMIT 1
				FOR UPDATE
			`
			dest := []any{&runningID, &runningActorType, &runningActorID, &runningActionID, &totalCount, &doneCount, &payloadRaw, &runStartAt, &runEndsAt, &otherRunEnd}
			if p := effectSession.Passive(); p != nil && p.ProgressOrdering {
				// The existing queue read also finds the next cross-lane boundary;
				// no per-operation furniture query or extra database round trip.
				dest = append(dest, &otherProgressBoundary)
			}
			err = tx.QueryRow(ctx, query, userID, settleAt, KittyStatusInCombat).Scan(dest...)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
		if err == nil {
			runningActorType = normalizeActorType(runningActorType)
			if runningActorType == ActionActorTypePlayer {
				runningActorID = 0
			}
			runningLaneKey := actorLaneKey(runningActorType, runningActorID)
			spec, ok := specs[runningActionID]
			ordinary, ordinaryErr := ordinaryState(payloadRaw)
			if ordinaryErr != nil {
				return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, ordinaryErr
			}
			if ordinary != nil {
				spec, ok = ordinary.Spec, true
				// Persist the next queue head before advancing another actor, even
				// when the physical worker budget did not force a handoff.
				ordinaryOrdering = true
			}
			if !ok {
				if _, err = tx.Exec(ctx, `
					DELETE FROM player_action_queue
					WHERE id = $1
				`, runningID); err != nil {
					return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
				}
				queueRowRemoved = true
				delete(carryStartAtByLane, runningLaneKey)
				continue
			}
			executor := s.actionHandlerForType(runningActionID)
			if ordinary != nil && ordinary.FishRecipe != nil {
				executor = newFishFoodExecutor(s, *ordinary.FishRecipe)
			}
			runtimeItem := QueueRuntimeItem{
				QueueID:                   runningID,
				ActorType:                 runningActorType,
				ActorID:                   runningActorID,
				ActionID:                  runningActionID,
				Spec:                      spec,
				TotalCount:                totalCount,
				DoneCount:                 doneCount,
				PayloadRaw:                payloadRaw,
				RunStartAt:                runStartAt,
				RunEndsAt:                 runEndsAt,
				OtherRunEnd:               otherRunEnd,
				EffectSession:             effectSession,
				PreserveScheduledTimeline: allowBatch || offlineLog,
			}
			if offlineLog && allowBatch {
				runtimeItem.alchemyReads = alchemyReads.forItem(runtimeItem)
			}
			if offlineLog {
				runtimeItem.AuditPhase = "offline_fastforward"
				if runningActorType == ActionActorTypePlayer {
					if loadoutID := queueRuntimeDeferredWorkLoadoutID(payloadRaw); loadoutID != "" {
						summary.FinalWorkLoadoutID = loadoutID
					}
				}
			} else {
				runtimeItem.AuditPhase = "online_settle"
			}
			if offlineLog && completedBudgetRemaining <= 0 {
				budgetExhausted = true
				break
			}
			var settleResult SettleResult
			settleBudget := FastForwardBudget{
				MaxIterations: completedBudgetRemaining,
				MaxExec:       maxExec,
				StartedAt:     budgetStartedAt,
			}
			executorSettleAt := settleAt
			if p := effectSession.Passive(); p != nil && p.ProgressOrdering {
				executorSettleAt, settleBudget.MaxIterations = passiveProgressSettlementLimit(runningActorType, runEndsAt, otherProgressBoundary, settleAt, settleBudget.MaxIterations)
			}
			effectDomain := executorOperationEffectDomain(executor)
			exactSegment := false
			if opts.Recovery != nil && offlineLog && allowBatch {
				if alchemy, ok := executor.(*alchemyExecutor); ok {
					settleResult, exactSegment, err = alchemy.trySettleExactSegmentTx(ctx, tx, userID, runtimeItem, executorSettleAt, settleBudget)
					if err != nil {
						return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
					}
					if exactSegment {
						alchemySegments++
						alchemySegmentSteps += settleResult.Completed
					}
				}
			}
			if !exactSegment && allowBatch && effectDomain != operationeffect.DomainNone {
				probeAt := settleAt
				if runEndsAt != nil && runEndsAt.Before(probeAt) {
					probeAt = *runEndsAt
				}
				probe := queueOperationEffectOperation(effectDomain, runtimeItem, probeAt, doneCount+2, payloadRaw)
				probe = enrichOperationEffectContext(executor, probe, payloadRaw)
				_, hasFrozenEffect := queueOperationEffectSnapshot(payloadRaw)
				perStart, requiresPerStart := executor.(operationEffectPerStartExecutor)
				if (requiresPerStart && perStart.OperationEffectsRequirePerStartSettlement() &&
					(hasFrozenEffect || effectSession.HasMatching(probe, settleAt))) ||
					(ordinary == nil && effectSession.HasMatchingCrossActorCharge(probe, settleAt)) {
					settleBudget.MaxIterations = 1
				}
			}
			if !exactSegment && allowBatch {
				settleResult, err = executor.SettleFastForwardTx(ctx, tx, userID, runtimeItem, executorSettleAt, FastForwardBudget{
					MaxIterations: settleBudget.MaxIterations,
					MaxExec:       settleBudget.MaxExec,
					StartedAt:     settleBudget.StartedAt,
				})
			} else if !exactSegment {
				settleResult, err = executor.SettleNormalTx(ctx, tx, userID, runtimeItem, settleAt)
			}
			if err != nil {
				return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
			}
			settleBudgetIterations := fastForwardBudgetIterations(settleResult)
			if !actionQueueSettleMadeProgress(settleResult, allowBatch) {
				budgetExhausted = true
				if offlineLog && !actionQueueAdvanceTimeBudgetExpired(budgetStartedAt, maxExec) {
					summary.YieldReason = ActionQueueYieldReasonNoProgress
					summary.StalledQueueID = runningID
					stalledPlayerLane = runningActorType == ActionActorTypePlayer
					log.Printf("offline action queue executor made no progress user_id=%d queue_id=%d action_id=%s", userID, runningID, runningActionID)
				}
				break
			}
			if runningActionID == FishingTripActionID {
				summary.FishingChanged = true
				mergeFishingCatchTallies(summary.FishingCatches, settleResult.FishingCatches)
				if event, ok := fishingCatchAchievementReconcileEvent(runningID, settleResult); ok {
					pendingAchievementEvents = append(pendingAchievementEvents, event)
				}
			}
			if offlineLog && executor.OfflineProjectionPolicy() == QueueOfflineProjectionImmediate {
				summary.RequiresImmediateProjection = true
			}
			if settleResult.Completed <= 0 {
				if settleResult.Progressed {
					// A fishing trip advances through several visible phases without
					// completing another life action. Keep its costs and experience in
					// the offline summary while leaving CompletedSteps unchanged.
					if runningActorType == ActionActorTypePlayer {
						for itemID, delta := range settleResult.ItemCosts {
							if delta > 0 {
								summary.ItemCosts[itemID] += delta
							}
						}
						for itemID, qty := range settleResult.ItemRewards {
							if qty > 0 {
								summary.ItemRewards[itemID] += qty
							}
						}
						for key, value := range settleResult.AttrExp {
							if value != 0 {
								summary.AttrExp[key] += value
							}
						}
					}
					for key, value := range settleResult.OwnerAttrExp {
						summary.AttrExp[key] += value
					}
					accumulateActionActorSummary(actorSummaryByLane, runningActorType, runningActorID, settleResult)
					nextPayload := settleResult.NextPayload
					if len(nextPayload) == 0 {
						nextPayload = payloadRaw
					}
					nextRunStartAt, nextRunEndsAt := nextQueueRunWindow(settleAt, settleResult, nextPayload, allowBatch, offlineLog)
					if _, err = tx.Exec(ctx, `
						UPDATE player_action_queue
						SET status = 'running',
						    current_run_started_at = $2,
						    current_run_ends_at = $3,
						    payload = $4::jsonb,
						    updated_at = now()
						WHERE id = $1
					`, runningID, nextRunStartAt, nextRunEndsAt, string(nextPayload)); err != nil {
						return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
					}
					if offlineLog && !settleResult.CompletedAt.IsZero() && (lastProgressAt.IsZero() || settleResult.CompletedAt.After(lastProgressAt)) {
						lastProgressAt = settleResult.CompletedAt
					}
					if offlineLog {
						completedBudgetRemaining -= settleBudgetIterations
					}
					delete(carryStartAtByLane, runningLaneKey)
					if offlineLog && completedBudgetRemaining <= 0 {
						budgetExhausted = true
						break
					}
					continue
				}
				if settleResult.CancelAfterSettle {
					cancelReason := strings.TrimSpace(settleResult.CancelReason)
					if cancelReason == "" {
						cancelReason = ActionQueueCancelReasonCostInsufficient
					}
					if _, err = tx.Exec(ctx, `
						DELETE FROM player_action_queue
						WHERE id = $1
					`, runningID); err != nil {
						return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
					}
					queueRowRemoved = true
					summary.QueueCompleted = append(summary.QueueCompleted, runningID)
					summary.QueueCanceled = append(summary.QueueCanceled, ActionQueueCancellation{
						QueueID:  runningID,
						ActionID: runningActionID,
						Reason:   cancelReason,
					})
					laneCarryStartAt := settleAt
					if runStartAt != nil && !runStartAt.IsZero() {
						laneCarryStartAt = *runStartAt
					}
					carryStartAtByLane[runningLaneKey] = laneCarryStartAt
					continue
				}
			}
			// Only accumulate player-actor rewards into the top-level summary.
			// Kitty rewards are shown separately in the per-actor breakdown
			// (actorSummaries); including them here causes double-display in
			// the offline rewards popup.
			if runningActorType == ActionActorTypePlayer {
				for itemID, delta := range settleResult.ItemCosts {
					if delta <= 0 {
						continue
					}
					summary.ItemCosts[itemID] += delta
				}
				for itemID, qty := range settleResult.ItemRewards {
					if qty <= 0 {
						continue
					}
					summary.ItemRewards[itemID] += qty
				}
				accumulateSettlementAttrExp(summary.AttrExp, settleResult, false)
			}
			for key, value := range settleResult.OwnerAttrExp {
				summary.AttrExp[key] += value
			}
			accumulateActionActorSummary(actorSummaryByLane, runningActorType, runningActorID, settleResult)
			summary.CompletedSteps += int64(settleResult.Completed)
			if offlineLog {
				completedBudgetRemaining -= settleBudgetIterations
				if !settleResult.CompletedAt.IsZero() && (lastProgressAt.IsZero() || settleResult.CompletedAt.After(lastProgressAt)) {
					lastProgressAt = settleResult.CompletedAt
				}
			}
			if len(settleResult.EnhanceResults) > 0 {
				summary.EnhanceResults = append(summary.EnhanceResults, settleResult.EnhanceResults...)
			}
			achievementEvents := []AchievementEvent{{
				Kind:      AchievementEventCompleteAction,
				ActionID:  runningActionID,
				ActorType: runningActorType,
				SubjectID: runningActionID,
				Count:     int64(settleResult.Completed),
				SourceKey: "action_queue:" + strconv.FormatInt(runningID, 10) + ":" + strconv.Itoa(settleResult.DoneAfter),
			}}
			for _, enhanceResult := range settleResult.EnhanceResults {
				achievementEvents = append(achievementEvents, AchievementEvent{
					Kind:         AchievementEventEnhanceCount,
					Count:        1,
					SourceKey:    "enhance:" + strconv.FormatInt(enhanceResult.QueueID, 10) + ":" + strconv.Itoa(enhanceResult.AttemptNo),
					ItemID:       strings.TrimSpace(enhanceResult.TemplateItemID),
					SubjectID:    strings.TrimSpace(enhanceResult.TemplateItemID),
					Successful:   enhanceResult.Success,
					EnhanceLevel: enhanceResult.AfterLevel,
				})
			}
			pendingAchievementEvents = append(pendingAchievementEvents, achievementEvents...)
			if settleResult.DoneAfter >= totalCount || settleResult.CancelAfterSettle {
				if _, err = tx.Exec(ctx, `
					DELETE FROM player_action_queue
					WHERE id = $1
				`, runningID); err != nil {
					return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
				}
				queueRowRemoved = true
				summary.QueueCompleted = append(summary.QueueCompleted, runningID)
				if settleResult.CancelAfterSettle && settleResult.DoneAfter < totalCount {
					cancelReason := strings.TrimSpace(settleResult.CancelReason)
					if cancelReason == "" {
						cancelReason = ActionQueueCancelReasonCostInsufficient
					}
					summary.QueueCanceled = append(summary.QueueCanceled, ActionQueueCancellation{
						QueueID:  runningID,
						ActionID: runningActionID,
						Reason:   cancelReason,
					})
				}
				carryStartAtByLane[runningLaneKey] = nextQueueCarryStartAt(settleAt, settleResult.CompletedAt, allowBatch, offlineLog)
				continue
			}
			nextPayload := settleResult.NextPayload
			if len(nextPayload) == 0 {
				nextPayload = payloadRaw
			}
			nextRunStartAt, nextRunEndsAt := nextQueueRunWindow(settleAt, settleResult, nextPayload, allowBatch, offlineLog)
			if _, err = tx.Exec(ctx, `
				UPDATE player_action_queue
				SET done_count = $2,
				    status = 'running',
				    current_run_started_at = $3,
				    current_run_ends_at = $4,
				    payload = $5::jsonb,
				    updated_at = now()
				WHERE id = $1
			`, runningID, settleResult.DoneAfter, nextRunStartAt, nextRunEndsAt, string(nextPayload)); err != nil {
				return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
			}
			delete(carryStartAtByLane, runningLaneKey)
			continue
		}

		// Starting a queued action can apply a loadout or other attribute changes.
		alchemyReads = offlineAlchemyReads{}
		var pendingID int64
		var pendingActorType string
		var pendingActorID int64
		var pendingActionID string
		var pendingPayload []byte
		var pendingTotalCount int
		var pendingDoneCount int
		handoffLaneKeys := []string{}
		if materializeCarry {
			handoffLaneKeys = actionQueueCarryLaneKeys(carryStartAtByLane)
		}
		err = tx.QueryRow(ctx, `
			SELECT p.id, p.actor_type, p.actor_id, p.action_id, p.total_count, p.done_count, p.payload
			FROM player_action_queue p
			WHERE p.user_id = $1
			  AND p.status = 'pending'
			  AND (
			    NOT $3::boolean
			    OR (p.actor_type || ':' || p.actor_id::text) = ANY($4::text[])
			  )
			  AND NOT (
			    p.actor_type = 'kitty'
			    AND EXISTS (
			      SELECT 1
			      FROM player_kitties k
			      WHERE k.user_id = p.user_id
			        AND k.id = p.actor_id
			        AND k.status = $2
			    )
			  )
			  AND NOT EXISTS (
			    SELECT 1
			    FROM player_action_queue r
			    WHERE r.user_id = p.user_id
			      AND r.status = 'running'
			      AND r.actor_type = p.actor_type
			      AND r.actor_id = p.actor_id
			  )
			ORDER BY p.queue_order ASC, p.id ASC
			LIMIT 1
			FOR UPDATE
		`, userID, KittyStatusInCombat, materializeCarry, handoffLaneKeys).Scan(&pendingID, &pendingActorType, &pendingActorID, &pendingActionID, &pendingTotalCount, &pendingDoneCount, &pendingPayload)
		if errors.Is(err, pgx.ErrNoRows) {
			if materializeCarry && !handoffOnly {
				clear(carryStartAtByLane)
				continue
			}
			loopHitGuard = false
			break
		}
		if err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
		pendingActorType = normalizeActorType(pendingActorType)
		if pendingActorType == ActionActorTypePlayer {
			pendingActorID = 0
		}
		pendingLaneKey := actorLaneKey(pendingActorType, pendingActorID)
		startAt := settleAt
		if carryStartAt, ok := carryStartAtByLane[pendingLaneKey]; ok {
			startAt = carryStartAt
		}
		spec, ok := specs[pendingActionID]
		if !ok {
			if _, err = tx.Exec(ctx, `
				DELETE FROM player_action_queue
				WHERE id = $1
			`, pendingID); err != nil {
				return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
			}
			queueRowRemoved = true
			carryStartAtByLane[pendingLaneKey] = startAt
			continue
		}
		pendingExecutor := s.actionHandlerForType(pendingActionID)
		runtimeItem := QueueRuntimeItem{
			QueueID:       pendingID,
			ActorType:     pendingActorType,
			ActorID:       pendingActorID,
			ActionID:      pendingActionID,
			Spec:          spec,
			TotalCount:    pendingTotalCount,
			DoneCount:     pendingDoneCount,
			PayloadRaw:    pendingPayload,
			EffectSession: effectSession,
		}
		if _, err = tx.Exec(ctx, `
			UPDATE player_action_queue
			SET status = 'running', updated_at = now()
			WHERE id = $1
		`, pendingID); err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
		startOptions := queueRunStartOptions{}
		if offlineLog {
			startOptions.WorkLoadoutActivation = workLoadoutActivationDeferred
			startOptions.DeferredWorkLoadoutID = summary.FinalWorkLoadoutID
		}
		if err = s.startQueueRunWithRollbackTx(ctx, tx, userID, &runtimeItem, startAt, startOptions); err != nil {
			_, _ = tx.Exec(ctx, `
				DELETE FROM player_action_queue
				WHERE id = $1
			`, pendingID)
			queueRowRemoved = true
			cancelQueue, reason := pendingExecutor.ClassifyStartError(err)
			if cancelQueue {
				if strings.TrimSpace(reason) == "" {
					reason = ActionQueueCancelReasonCostInsufficient
				}
				summary.QueueCompleted = append(summary.QueueCompleted, pendingID)
				summary.QueueCanceled = append(summary.QueueCanceled, ActionQueueCancellation{
					QueueID:  pendingID,
					ActionID: pendingActionID,
					Reason:   reason,
				})
				carryStartAtByLane[pendingLaneKey] = startAt
				continue
			}
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
		if offlineLog && pendingExecutor.OfflineProjectionPolicy() == QueueOfflineProjectionImmediate {
			summary.RequiresImmediateProjection = true
		}
		if pendingActionID == FishingTripActionID {
			summary.FishingChanged = true
		}
		if runtimeItem.EquipmentChanged {
			markActionActorEquipmentChanged(actorSummaryByLane, pendingActorType, pendingActorID)
			for _, kittyID := range runtimeItem.EquipmentKittyIDs {
				markActionActorEquipmentChanged(actorSummaryByLane, ActionActorTypeKitty, kittyID)
			}
		}
		if runtimeItem.WorkLoadoutRuntime != nil {
			usage := WorkLoadoutUsage{
				ActionID:       pendingActionID,
				LoadoutID:      runtimeItem.WorkLoadoutRuntime.LoadoutID,
				Applied:        runtimeItem.WorkLoadoutRuntime.Available,
				FallbackReason: runtimeItem.WorkLoadoutRuntime.FallbackReason,
				Offline:        offlineLog,
			}
			summary.WorkLoadoutUsages = append(summary.WorkLoadoutUsages, usage)
		}
		if offlineLog && pendingActorType == ActionActorTypePlayer {
			if loadoutID := queueRuntimeDeferredWorkLoadoutID(runtimeItem.PayloadRaw); loadoutID != "" {
				summary.FinalWorkLoadoutID = loadoutID
			}
		}
		if runtimeItem.ActivatedWorkLoadout != nil && !offlineLog {
			summary.ActivatedWorkLoadouts = append(summary.ActivatedWorkLoadouts, *runtimeItem.ActivatedWorkLoadout)
		}
		delete(carryStartAtByLane, pendingLaneKey)
	}
	if loopHitGuard {
		budgetExhausted = true
	}
	if offlineLog {
		hasMoreDue, dueErr := s.hasActionQueueDueUntilTx(ctx, tx, userID, settleAt)
		if dueErr != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, dueErr
		}
		summary.BudgetExhausted = budgetExhausted
		summary.HasMoreDue = hasMoreDue
		if summary.HasMoreDue {
			if !lastProgressAt.IsZero() {
				summary.SettledUntilMS = lastProgressAt.UnixMilli()
			}
		} else {
			summary.SettledUntilMS = settleAt.UnixMilli()
			shiftUntil := realNow
			if opts.Recovery != nil {
				shiftUntil = opts.Recovery.ResumedAt
				if opts.Recovery.Background {
					shiftUntil = settleAt
				}
			}
			if shift := offlineCapWindowShift(settleAt, shiftUntil); shift > 0 {
				if err = s.shiftOfflineCappedRunningQueueWindowsTx(ctx, tx, userID, shift, realNow); err != nil {
					return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
				}
			}
		}
		if summary.FinalWorkLoadoutID == "" && opts.Recovery != nil {
			summary.FinalWorkLoadoutID = opts.Recovery.Summary.FinalWorkLoadoutID
		}
		if strings.TrimSpace(summary.FinalWorkLoadoutID) != "" && (opts.Recovery == nil || !opts.Recovery.Background) {
			hasMorePlayerDue := false
			if summary.HasMoreDue && !stalledPlayerLane {
				var playerDueErr error
				hasMorePlayerDue, playerDueErr = s.hasPlayerActionQueueDueUntilTx(ctx, tx, userID, settleAt)
				if playerDueErr != nil {
					return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, playerDueErr
				}
			}
			if shouldApplyOfflineFinalWorkLoadout(hasMorePlayerDue, stalledPlayerLane) {
				changed, applyErr := s.applyWorkLoadoutTx(ctx, tx, userID, summary.FinalWorkLoadoutID)
				if applyErr != nil {
					return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, applyErr
				}
				if clearErr := s.clearOfflineWorkLoadoutIntentTx(ctx, tx, userID, summary.FinalWorkLoadoutID); clearErr != nil {
					return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, clearErr
				}
				if changed {
					markActionActorEquipmentChanged(actorSummaryByLane, ActionActorTypePlayer, 0)
				}
			}
		}
	}

	// Settling an in-progress row does not change lane order. Membership changes
	// can leave gaps, so normalize only after a row was removed.
	if queueRowRemoved {
		if err = s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
	}
	effectsChanged, err := s.flushOperationEffectSessionTx(ctx, tx, userID, effectSession, realNow)
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	summary.OperationEffectsChanged = effectsChanged
	if err = s.reconcileKittyLaneStatusesTx(ctx, tx, userID); err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	summary.ActorSummaries = buildActionActorSummaries(actorSummaryByLane)
	if opts.Recovery != nil {
		if len(pendingAchievementEvents) > 0 {
			token := fmt.Sprintf("recovery:%d:%d", opts.Recovery.ID, opts.Recovery.Revision+1)
			if err = s.EnqueueAchievementEventsTx(ctx, tx, userID, aggregateAchievementEventsForOfflineBatch(token, pendingAchievementEvents)); err != nil {
				return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
			}
			pendingAchievementEvents = nil
		}
		if err = achievementBatch.flush(ctx); err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
		if err = s.commitRecoveryProgressTx(ctx, tx, *opts.Recovery, summary); err != nil {
			return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		s.discardInventoryAuditBuffer(tx)
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	if achievementBatch != nil && achievementBatch.inserted && s.achievementNotify != nil {
		s.achievementNotify(ctx)
	}
	settleElapsed := time.Since(realNow)
	auditStartedAt := time.Now()
	s.flushInventoryAuditBuffer(ctx, tx)
	if opts.Recovery != nil {
		log.Printf("offline recovery committed user_id=%d recovery_id=%d revision=%d completed_steps=%d has_more_due=%v yield_reason=%s settle_ms=%d audit_ms=%d alchemy_segments=%d alchemy_segment_steps=%d elapsed_ms=%d", userID, opts.Recovery.ID, opts.Recovery.Revision+1, summary.CompletedSteps, summary.HasMoreDue, summary.YieldReason, settleElapsed.Milliseconds(), time.Since(auditStartedAt).Milliseconds(), alchemySegments, alchemySegmentSteps, time.Since(realNow).Milliseconds())
	}
	if len(pendingAchievementEvents) > 0 {
		if offlineLog {
			rawEventCount := len(pendingAchievementEvents)
			batchToken := strconv.FormatInt(time.Now().UnixNano(), 10)
			pendingAchievementEvents = aggregateAchievementEventsForOfflineBatch(batchToken, pendingAchievementEvents)
			log.Printf("offline fastforward achievement batch prepared user_id=%d batch=%s raw_events=%d aggregated_events=%d", userID, batchToken, rawEventCount, len(pendingAchievementEvents))
		}
		if aErr := s.ApplyAchievementEventsNoReconcile(context.Background(), userID, pendingAchievementEvents); aErr != nil {
			log.Printf("achievement events enqueue failed after commit user_id=%d events=%d err=%v", userID, len(pendingAchievementEvents), aErr)
		}
	}
	summary.ActorSummaries = buildActionActorSummaries(actorSummaryByLane)
	if opts.SkipQueueRead {
		return nil, PlayerState{}, AttributeExp{}, summary, nil
	}
	queue, err := s.ListActionQueue(ctx, userID, 50)
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	if opts.SkipReadModels {
		return queue, PlayerState{}, AttributeExp{}, summary, nil
	}
	state, err := s.GetPlayerState(ctx, userID)
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	attr, err := s.GetAttributeExp(ctx, userID)
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	return queue, state, attr, summary, nil
}

func shouldYieldActionQueueAdvanceBudget(startedAt time.Time, maxExec time.Duration, carryStartAtByLane map[string]time.Time) bool {
	if len(carryStartAtByLane) > 0 {
		return false
	}
	return actionQueueAdvanceTimeBudgetExpired(startedAt, maxExec)
}

func actionQueueAdvanceTimeBudgetExpired(startedAt time.Time, maxExec time.Duration) bool {
	return !startedAt.IsZero() && maxExec > 0 && time.Since(startedAt) >= maxExec
}

func actionQueueCarryLaneKeys(carryStartAtByLane map[string]time.Time) []string {
	keys := make([]string, 0, len(carryStartAtByLane))
	for laneKey := range carryStartAtByLane {
		keys = append(keys, laneKey)
	}
	sort.Strings(keys)
	return keys
}

func fastForwardBudgetIterations(result SettleResult) int {
	iterations := result.Completed
	if result.BudgetIterations > iterations {
		iterations = result.BudgetIterations
	}
	if iterations < 0 {
		return 0
	}
	return iterations
}

func actionQueueSettleMadeProgress(result SettleResult, requireBudgetIterations bool) bool {
	return result.Completed > 0 ||
		(result.Progressed && (!requireBudgetIterations || result.BudgetIterations > 0)) ||
		result.CancelAfterSettle
}

func shouldApplyOfflineFinalWorkLoadout(hasMorePlayerDue, stalledPlayerLane bool) bool {
	return !hasMorePlayerDue || stalledPlayerLane
}

// Consume orchestration intent even when equipment already matches. Leaving it
// on a long-running row would replay the old loadout on a later offline return.
func (s *Store) clearOfflineWorkLoadoutIntentTx(ctx context.Context, tx pgx.Tx, userID int64, loadoutID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE player_action_queue
		SET payload = payload - 'deferredWorkLoadoutId', updated_at = now()
		WHERE user_id = $1
		  AND actor_type = 'player'
		  AND actor_id = 0
		  AND status = 'running'
		  AND payload->>'deferredWorkLoadoutId' = $2
	`, userID, loadoutID)
	return err
}

func fishingCatchAchievementReconcileEvent(queueID int64, result SettleResult) (AchievementEvent, bool) {
	landed := false
	for _, tally := range result.FishingCatches {
		if tally.Landed > 0 {
			landed = true
			break
		}
	}
	if !landed || result.CompletedAt.IsZero() {
		return AchievementEvent{}, false
	}
	return AchievementEvent{
		Kind:      AchievementEventStateReconcile,
		StateKind: "catch_fish",
		SourceKey: "fishing_catch:" + strconv.FormatInt(queueID, 10) + ":" + strconv.FormatInt(result.CompletedAt.UnixMilli(), 10),
	}, true
}

func (s *Store) CancelActionGroup(ctx context.Context, userID int64, specs map[string]ActionSpec, actionID string, actorType string, actorID int64) ([]ActionQueueItem, PlayerState, AttributeExp, error) {
	normalizedActorType := normalizeActorType(actorType)
	if normalizedActorType == ActionActorTypePlayer {
		actorID = 0
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return err
		}
		var queueID int64
		var status string
		var totalCount int
		var doneCount int
		var payloadRaw []byte
		err := tx.QueryRow(ctx, `
			SELECT id, status, total_count, done_count, payload
			FROM player_action_queue
			WHERE user_id = $1
			  AND action_id = $2
			  AND actor_type = $3
			  AND actor_id = $4
			  AND status IN ('running', 'pending')
			ORDER BY queue_order ASC, id ASC
			LIMIT 1
			FOR UPDATE
		`, userID, actionID, normalizedActorType, actorID).Scan(&queueID, &status, &totalCount, &doneCount, &payloadRaw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if status == "running" {
			if spec, ok := specs[actionID]; ok {
				runtimeItem := QueueRuntimeItem{
					QueueID:    queueID,
					ActorType:  normalizedActorType,
					ActorID:    actorID,
					ActionID:   actionID,
					Spec:       spec,
					TotalCount: totalCount,
					DoneCount:  doneCount,
					PayloadRaw: payloadRaw,
				}
				if err = s.actionHandlerForType(actionID).CancelRunTx(ctx, tx, userID, runtimeItem); err != nil {
					return err
				}
			}
		}
		if _, err = tx.Exec(ctx, `
			DELETE FROM player_action_queue
			WHERE id = $1
		`, queueID); err != nil {
			return err
		}
		if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return err
		}
		return s.reconcileKittyLaneStatusesTx(ctx, tx, userID)
	})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, err
	}
	return s.AdvanceActionQueue(ctx, userID, specs)
}

func aggregateAchievementEventsForOfflineBatch(batchToken string, events []AchievementEvent) []AchievementEvent {
	normalized := normalizeAchievementEvents(events)
	if len(normalized) == 0 {
		return nil
	}
	byKey := make(map[string]AchievementEvent, len(normalized))
	order := make([]string, 0, len(normalized))
	for _, event := range normalized {
		counterKey := achievementEventPrimaryCounterKey(event)
		if counterKey == "" {
			continue
		}
		aggregationKey := offlineAchievementAggregationKey(event, counterKey)
		if aggregationKey == "" {
			continue
		}
		existing, ok := byKey[aggregationKey]
		if !ok {
			event.SourceKey = "offline_batch:" + strings.TrimSpace(batchToken) + ":" + aggregationKey
			byKey[aggregationKey] = event
			order = append(order, aggregationKey)
			continue
		}
		if event.Kind != AchievementEventStateReconcile {
			existing.Count += event.Count
		}
		byKey[aggregationKey] = existing
	}
	out := make([]AchievementEvent, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}

func offlineAchievementAggregationKey(event AchievementEvent, counterKey string) string {
	kind := strings.TrimSpace(event.Kind)
	switch kind {
	case AchievementEventStateReconcile:
		return "state_reconcile:" + counterKey
	case AchievementEventCompleteAction:
		actionID := firstNonEmptyAchievementString(event.ActionID, event.SubjectID)
		return strings.Join([]string{kind, counterKey, normalizeActorType(event.ActorType), actionID}, "|")
	case AchievementEventEnhanceCount, AchievementEventEnchantCount:
		itemID := firstNonEmptyAchievementString(event.ItemID, event.SubjectID)
		success := "0"
		if event.Successful {
			success = "1"
		}
		return strings.Join([]string{kind, counterKey, itemID, strconv.Itoa(event.EnhanceLevel), success}, "|")
	case AchievementEventObtainItem:
		itemID := firstNonEmptyAchievementString(event.ItemID, event.SubjectID)
		return strings.Join([]string{kind, counterKey, itemID, strconv.Itoa(event.EnhanceLevel)}, "|")
	case AchievementEventKillEnemy:
		enemyID := firstNonEmptyAchievementString(event.EnemyID, event.SubjectID)
		return strings.Join([]string{kind, counterKey, enemyID}, "|")
	case AchievementEventCompleteSpecificCommission:
		commissionID := firstNonEmptyAchievementString(event.CommissionID, event.SubjectID)
		return strings.Join([]string{kind, counterKey, commissionID}, "|")
	default:
		return strings.Join([]string{kind, counterKey}, "|")
	}
}

func (s *Store) CancelQueueItem(ctx context.Context, userID int64, specs map[string]ActionSpec, queueID int64) ([]ActionQueueItem, PlayerState, AttributeExp, error) {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return err
		}
		var actorType string
		var actorID int64
		var actionID string
		var status string
		var totalCount int
		var doneCount int
		var payloadRaw []byte
		err := tx.QueryRow(ctx, `
			SELECT actor_type, actor_id, action_id, status, total_count, done_count, payload
			FROM player_action_queue
			WHERE id = $1 AND user_id = $2 AND status IN ('running', 'pending')
			FOR UPDATE
		`, queueID, userID).Scan(&actorType, &actorID, &actionID, &status, &totalCount, &doneCount, &payloadRaw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		actorType = normalizeActorType(actorType)
		if actorType == ActionActorTypePlayer {
			actorID = 0
		}
		if status == "running" {
			if spec, ok := specs[actionID]; ok {
				runtimeItem := QueueRuntimeItem{
					QueueID:    queueID,
					ActorType:  actorType,
					ActorID:    actorID,
					ActionID:   actionID,
					Spec:       spec,
					TotalCount: totalCount,
					DoneCount:  doneCount,
					PayloadRaw: payloadRaw,
				}
				if err = s.actionHandlerForType(actionID).CancelRunTx(ctx, tx, userID, runtimeItem); err != nil {
					return err
				}
			}
		}
		if _, err = tx.Exec(ctx, `
			DELETE FROM player_action_queue
			WHERE id = $1
		`, queueID); err != nil {
			return err
		}
		if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return err
		}
		return s.reconcileKittyLaneStatusesTx(ctx, tx, userID)
	})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, err
	}
	return s.AdvanceActionQueue(ctx, userID, specs)
}

func (s *Store) normalizeQueueOrderTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		WITH ordered AS (
			SELECT id,
			       ROW_NUMBER() OVER (
			           PARTITION BY actor_type, actor_id
			           ORDER BY
			               CASE WHEN status = 'running' THEN 0 ELSE 1 END,
			               queue_order ASC,
			               id ASC
			       ) AS next_order
			FROM player_action_queue
			WHERE user_id = $1 AND status IN ('pending', 'running')
		)
		UPDATE player_action_queue q
		SET queue_order = ordered.next_order,
		    updated_at = now()
		FROM ordered
		WHERE q.id = ordered.id
		  AND q.queue_order IS DISTINCT FROM ordered.next_order
	`, userID)
	return err
}

func (s *Store) MoveQueueItem(ctx context.Context, userID int64, specs map[string]ActionSpec, queueID int64, direction string) ([]ActionQueueItem, PlayerState, AttributeExp, error) {
	queue, state, attr, _, err := s.MoveQueueItemWithSummary(ctx, userID, specs, queueID, direction)
	return queue, state, attr, err
}

func (s *Store) MoveQueueItemWithSummary(ctx context.Context, userID int64, specs map[string]ActionSpec, queueID int64, direction string) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	switch direction {
	case "up", "down", "top", "bottom":
	default:
		direction = "up"
	}
	runNow := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return err
		}
		var targetOrder int64
		var actorType string
		var actorID int64
		var status string
		err := tx.QueryRow(ctx, `
			SELECT queue_order, actor_type, actor_id, status
			FROM player_action_queue
			WHERE id = $1 AND user_id = $2 AND status IN ('pending', 'running')
			FOR UPDATE
		`, queueID, userID).Scan(&targetOrder, &actorType, &actorID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		actorType = normalizeActorType(actorType)
		if actorType == ActionActorTypePlayer {
			actorID = 0
		}
		if status != "pending" {
			return nil
		}

		if direction == "top" || direction == "bottom" {
			var extremeOrder *int64
			if direction == "top" {
				err = tx.QueryRow(ctx, `
					SELECT MIN(queue_order)
					FROM player_action_queue
					WHERE user_id = $1 AND actor_type = $2 AND actor_id = $3 AND status = 'pending'
				`, userID, actorType, actorID).Scan(&extremeOrder)
			} else {
				err = tx.QueryRow(ctx, `
					SELECT MAX(queue_order)
					FROM player_action_queue
					WHERE user_id = $1 AND actor_type = $2 AND actor_id = $3 AND status = 'pending'
				`, userID, actorType, actorID).Scan(&extremeOrder)
			}
			if err != nil {
				return err
			}
			if extremeOrder == nil || targetOrder == *extremeOrder {
				return nil
			}
			newOrder := *extremeOrder - 1
			if direction == "bottom" {
				newOrder = *extremeOrder + 1
			}
			_, err = tx.Exec(ctx, `
				UPDATE player_action_queue SET queue_order = $2, updated_at = now() WHERE id = $1
			`, queueID, newOrder)
			return err
		}

		var neighborID int64
		var neighborOrder int64
		if direction == "up" {
			err = tx.QueryRow(ctx, `
				SELECT id, queue_order
				FROM player_action_queue
				WHERE user_id = $1
				  AND actor_type = $2
				  AND actor_id = $3
				  AND status = 'pending'
				  AND queue_order < $4
				ORDER BY queue_order DESC, id DESC
				LIMIT 1
				FOR UPDATE
			`, userID, actorType, actorID, targetOrder).Scan(&neighborID, &neighborOrder)
		} else {
			err = tx.QueryRow(ctx, `
				SELECT id, queue_order
				FROM player_action_queue
				WHERE user_id = $1
				  AND actor_type = $2
				  AND actor_id = $3
				  AND status = 'pending'
				  AND queue_order > $4
				ORDER BY queue_order ASC, id ASC
				LIMIT 1
				FOR UPDATE
			`, userID, actorType, actorID, targetOrder).Scan(&neighborID, &neighborOrder)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			runNow = direction == "up"
			return nil
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE player_action_queue
			SET queue_order = $2, updated_at = now()
			WHERE id = $1
		`, queueID, neighborOrder); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE player_action_queue
			SET queue_order = $2, updated_at = now()
			WHERE id = $1
		`, neighborID, targetOrder); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	if runNow {
		return s.RunQueueItemNowWithSummary(ctx, userID, specs, queueID)
	}
	return s.AdvanceActionQueueWithSummary(ctx, userID, specs)
}

func (s *Store) RunQueueItemNow(ctx context.Context, userID int64, specs map[string]ActionSpec, queueID int64) ([]ActionQueueItem, PlayerState, AttributeExp, error) {
	queue, state, attr, _, err := s.RunQueueItemNowWithSummary(ctx, userID, specs, queueID)
	return queue, state, attr, err
}

func (s *Store) RunQueueItemNowWithSummary(ctx context.Context, userID int64, specs map[string]ActionSpec, queueID int64) ([]ActionQueueItem, PlayerState, AttributeExp, ActionFastForwardSummary, error) {
	var activated *ActivatedWorkLoadout
	var equipmentChangedActorType string
	var equipmentChangedActorID int64
	var equipmentChangedKittyIDs []int64
	operationEffectsChanged := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
			return err
		}
		var targetActorType string
		var targetActorID int64
		var status string
		var actionID string
		var payloadRaw []byte
		var totalCount int
		var doneCount int
		err := tx.QueryRow(ctx, `
			SELECT actor_type, actor_id, status, action_id, total_count, done_count, payload
			FROM player_action_queue
			WHERE id = $1 AND user_id = $2 AND status IN ('pending', 'running')
			FOR UPDATE
		`, queueID, userID).Scan(&targetActorType, &targetActorID, &status, &actionID, &totalCount, &doneCount, &payloadRaw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		targetActorType = normalizeActorType(targetActorType)
		if targetActorType == ActionActorTypePlayer {
			targetActorID = 0
		}
		if status != "pending" {
			return nil
		}
		if targetActorType == ActionActorTypeKitty {
			kittyStatus, err := s.getOwnedKittyStatusTx(ctx, tx, userID, targetActorID, true)
			if err != nil {
				return err
			}
			if kittyStatus == KittyStatusInCombat {
				return ErrKittyModeConflictLifeQueue
			}
		}

		var minOrder int64
		if err = tx.QueryRow(ctx, `
			SELECT COALESCE(MIN(queue_order), 0)
			FROM player_action_queue
			WHERE user_id = $1
			  AND actor_type = $2
			  AND actor_id = $3
			  AND status IN ('pending', 'running')
		`, userID, targetActorType, targetActorID).Scan(&minOrder); err != nil {
			return err
		}

		var runningID int64
		var runningActorType string
		var runningActorID int64
		var runningActionID string
		var runningTotalCount int
		var runningDoneCount int
		var runningPayloadRaw []byte
		hasRunning := true
		err = tx.QueryRow(ctx, `
			SELECT id, actor_type, actor_id, action_id, total_count, done_count, payload
			FROM player_action_queue
			WHERE user_id = $1
			  AND actor_type = $2
			  AND actor_id = $3
			  AND status = 'running'
			ORDER BY queue_order ASC, id ASC
			LIMIT 1
			FOR UPDATE
		`, userID, targetActorType, targetActorID).Scan(&runningID, &runningActorType, &runningActorID, &runningActionID, &runningTotalCount, &runningDoneCount, &runningPayloadRaw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			hasRunning = false
		} else {
			runningActorType = normalizeActorType(runningActorType)
			if runningActorType == ActionActorTypePlayer {
				runningActorID = 0
			}
		}

		promotedOrder := minOrder - 2
		if _, err = tx.Exec(ctx, `
			UPDATE player_action_queue
			SET queue_order = $2, updated_at = now()
			WHERE id = $1
		`, queueID, promotedOrder); err != nil {
			return err
		}

		spec, ok := specs[actionID]
		if !ok {
			return errors.New("action spec not found")
		}
		targetRuntime := QueueRuntimeItem{
			QueueID:    queueID,
			ActorType:  targetActorType,
			ActorID:    targetActorID,
			ActionID:   actionID,
			Spec:       spec,
			TotalCount: totalCount,
			DoneCount:  doneCount,
			PayloadRaw: payloadRaw,
		}
		startAt := time.Now()
		var effectSession *operationeffect.Session
		if executorOperationEffectDomain(s.actionHandlerForType(actionID)) != operationeffect.DomainNone {
			effectSession, err = s.loadOperationEffectSessionTx(ctx, tx, userID, startAt)
			if err != nil {
				return err
			}
			targetRuntime.EffectSession = effectSession
		}
		if err = s.startQueueRunWithRollbackTx(ctx, tx, userID, &targetRuntime, startAt, queueRunStartOptions{}); err != nil {
			cancelQueue, _ := s.actionHandlerForType(actionID).ClassifyStartError(err)
			if cancelQueue {
				if _, delErr := tx.Exec(ctx, `
					DELETE FROM player_action_queue
					WHERE id = $1
				`, queueID); delErr != nil {
					return delErr
				}
				return s.normalizeQueueOrderTx(ctx, tx, userID)
			}
			return err
		}
		if targetRuntime.ActivatedWorkLoadout != nil {
			copyValue := *targetRuntime.ActivatedWorkLoadout
			activated = &copyValue
		}
		if targetRuntime.EquipmentChanged {
			equipmentChangedActorType = targetRuntime.ActorType
			equipmentChangedActorID = targetRuntime.ActorID
			equipmentChangedKittyIDs = append([]int64(nil), targetRuntime.EquipmentKittyIDs...)
		}
		if hasRunning {
			if runningSpec, ok := specs[runningActionID]; ok {
				runningRuntime := QueueRuntimeItem{
					QueueID:    runningID,
					ActorType:  runningActorType,
					ActorID:    runningActorID,
					ActionID:   runningActionID,
					Spec:       runningSpec,
					TotalCount: runningTotalCount,
					DoneCount:  runningDoneCount,
					PayloadRaw: runningPayloadRaw,
				}
				if cancelErr := s.actionHandlerForType(runningActionID).CancelRunTx(ctx, tx, userID, runningRuntime); cancelErr != nil {
					return cancelErr
				}
			}
			if _, err = tx.Exec(ctx, `
				UPDATE player_action_queue
				SET status = 'pending',
				    queue_order = $2,
				    current_run_started_at = NULL,
				    current_run_ends_at = NULL,
				    updated_at = now()
				WHERE id = $1
			`, runningID, minOrder-1); err != nil {
				return err
			}
		}
		if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return err
		}
		if effectSession != nil {
			changed, err := s.flushOperationEffectSessionTx(ctx, tx, userID, effectSession, time.Now())
			if err != nil {
				return err
			}
			operationEffectsChanged = changed
		}
		return nil
	})
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	queue, state, attr, summary, err := s.AdvanceActionQueueWithSummary(ctx, userID, specs)
	if err != nil {
		return nil, PlayerState{}, AttributeExp{}, ActionFastForwardSummary{}, err
	}
	summary.OperationEffectsChanged = summary.OperationEffectsChanged || operationEffectsChanged
	if activated != nil {
		alreadyIncluded := false
		for _, row := range summary.ActivatedWorkLoadouts {
			if strings.EqualFold(strings.TrimSpace(row.ActionID), strings.TrimSpace(activated.ActionID)) &&
				strings.EqualFold(strings.TrimSpace(row.LoadoutID), strings.TrimSpace(activated.LoadoutID)) {
				alreadyIncluded = true
				break
			}
		}
		if !alreadyIncluded {
			summary.ActivatedWorkLoadouts = append([]ActivatedWorkLoadout{*activated}, summary.ActivatedWorkLoadouts...)
		}
	}
	if strings.TrimSpace(equipmentChangedActorType) != "" {
		summaryByLane := actionActorSummaryMap(summary.ActorSummaries)
		markActionActorEquipmentChanged(summaryByLane, equipmentChangedActorType, equipmentChangedActorID)
		for _, kittyID := range equipmentChangedKittyIDs {
			markActionActorEquipmentChanged(summaryByLane, ActionActorTypeKitty, kittyID)
		}
		summary.ActorSummaries = buildActionActorSummaries(summaryByLane)
		summary.FishingChanged = true
	}
	return queue, state, attr, summary, nil
}

func (s *Store) startQueueRunTx(ctx context.Context, tx pgx.Tx, userID int64, item *QueueRuntimeItem, startAt time.Time, opts queueRunStartOptions) error {
	if item == nil {
		return ErrInvalidActionCount
	}
	item.ActorType = normalizeActorType(item.ActorType)
	if item.ActorType == ActionActorTypePlayer {
		item.ActorID = 0
	}
	deferredWorkLoadoutID := strings.TrimSpace(opts.DeferredWorkLoadoutID)
	if item.ActorType == ActionActorTypePlayer {
		resolution, err := s.ResolveWorkLoadoutRuntime(ctx, tx, userID, item.ActionID, item.Spec.MainType)
		if err != nil {
			return err
		}
		if resolution.Available {
			item.WorkLoadoutRuntime = &resolution
			if loadoutID := strings.TrimSpace(resolution.LoadoutID); opts.WorkLoadoutActivation == workLoadoutActivationDeferred && loadoutID != "" {
				deferredWorkLoadoutID = loadoutID
			}
			if opts.WorkLoadoutActivation == workLoadoutActivationImmediate && strings.TrimSpace(resolution.LoadoutID) != "" {
				changed, err := s.applyWorkLoadoutTx(ctx, tx, userID, resolution.LoadoutID)
				if err != nil {
					return err
				}
				if changed {
					item.ActivatedWorkLoadout = &ActivatedWorkLoadout{
						ActionID:  strings.TrimSpace(item.ActionID),
						LoadoutID: strings.TrimSpace(resolution.LoadoutID),
					}
				}
			}
		} else if strings.TrimSpace(resolution.LoadoutID) != "" {
			item.WorkLoadoutRuntime = &resolution
		}
	}
	if item.WorkLoadoutRuntime != nil && item.WorkLoadoutRuntime.Available && len(item.WorkLoadoutRuntime.AttributeTotals) > 0 {
		if !meetStatusRequireWithTotals(item.WorkLoadoutRuntime.AttributeTotals, item.Spec.StatusRequirement) {
			return ErrStatusRequireNotMet
		}
	} else if ok, err := s.meetStatusRequireTx(ctx, tx, userID, item.Spec.StatusRequirement); err != nil {
		return err
	} else if !ok {
		return ErrStatusRequireNotMet
	}
	if ok, err := s.meetBuildingRequireTx(ctx, tx, userID, item.Spec.BuildingRequirement); err != nil {
		return err
	} else if !ok {
		return ErrBuildingRequireNotMet
	}
	executor := s.actionHandlerForType(item.ActionID)
	var preparedEffects *operationeffect.PreparedOperation
	if domain := executorOperationEffectDomain(executor); domain != operationeffect.DomainNone && item.EffectSession != nil {
		operation := queueOperationEffectOperation(domain, *item, startAt, item.DoneCount+1, item.PayloadRaw)
		operation = enrichOperationEffectContext(executor, operation, item.PayloadRaw)
		if enricher, ok := executor.(operationEffectContextTxEnricher); ok {
			var err error
			operation, err = enricher.EnrichOperationEffectContextTx(ctx, tx, operation, item.PayloadRaw)
			if err != nil {
				return err
			}
		}
		prepared := item.EffectSession.Prepare(operation)
		preview := prepared.Snapshot()
		preparedEffects = &prepared
		item.OperationEffects = &preview
	}
	payloadRaw, err := executor.StartRunTx(ctx, tx, userID, *item, startAt)
	if err != nil {
		return err
	}
	if len(payloadRaw) == 0 {
		payloadRaw = item.PayloadRaw
	}
	payloadRaw, err = freezeQueuePassive(*item, payloadRaw, startAt, executorOperationEffectDomain(executor))
	if err != nil {
		return err
	}
	if item.OperationEffects != nil {
		payloadRaw, err = setQueueOperationEffectSnapshot(payloadRaw, *item.OperationEffects)
		if err != nil {
			return err
		}
	}
	if item.ActorType == ActionActorTypePlayer && opts.WorkLoadoutActivation == workLoadoutActivationDeferred {
		payloadRaw, err = setQueueRuntimeDeferredWorkLoadoutID(payloadRaw, deferredWorkLoadoutID)
		if err != nil {
			return err
		}
	}
	item.PayloadRaw = payloadRaw
	if item.ActionID == FishingTripActionID {
		if runtime, ok := parseFishingExecutorRuntime(payloadRaw); ok {
			item.EquipmentChanged = true
			item.EquipmentKittyIDs = append([]int64(nil), runtime.EquipmentKittyIDs...)
		}
	}
	duration := parseQueueRuntimeDuration(payloadRaw)
	if duration <= 0 {
		duration = minimumActionQueueRunDuration
	}
	endsAt := startAt.Add(duration)
	_, err = tx.Exec(ctx, `
		UPDATE player_action_queue
		SET current_run_started_at = $2,
		    current_run_ends_at = $3,
		    payload = $4::jsonb,
		    status = 'running',
		    updated_at = now()
		WHERE id = $1
	`, item.QueueID, startAt, endsAt, string(payloadRaw))
	if err != nil {
		return err
	}
	if preparedEffects != nil {
		if _, err := item.EffectSession.Commit(*preparedEffects); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) startQueueRunWithRollbackTx(ctx context.Context, tx pgx.Tx, userID int64, item *QueueRuntimeItem, startAt time.Time, opts queueRunStartOptions) error {
	if _, err := tx.Exec(ctx, `SAVEPOINT queue_start_attempt`); err != nil {
		return err
	}
	startErr := s.startQueueRunTx(ctx, tx, userID, item, startAt, opts)
	if startErr == nil {
		_, err := tx.Exec(ctx, `RELEASE SAVEPOINT queue_start_attempt`)
		return err
	}
	if _, rollbackErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT queue_start_attempt`); rollbackErr != nil {
		return errors.Join(startErr, rollbackErr)
	}
	if _, releaseErr := tx.Exec(ctx, `RELEASE SAVEPOINT queue_start_attempt`); releaseErr != nil {
		return errors.Join(startErr, releaseErr)
	}
	return startErr
}

func (s *Store) meetStatusRequireTx(ctx context.Context, tx pgx.Tx, userID int64, require map[string]int64) (bool, error) {
	if len(require) == 0 {
		return true, nil
	}
	for key, minRequired := range require {
		var finalValue float64
		err := tx.QueryRow(ctx, `
			SELECT final_value
			FROM player_attributes
			WHERE user_id = $1 AND attr_key = $2
		`, userID, key).Scan(&finalValue)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if finalValue < float64(minRequired) {
			return false, nil
		}
	}
	return true, nil
}

func meetStatusRequireWithTotals(attrTotals map[string]float64, require map[string]int64) bool {
	if len(require) == 0 {
		return true
	}
	for key, minRequired := range require {
		if roundStatValue(attrTotals[strings.TrimSpace(key)]) < float64(minRequired) {
			return false
		}
	}
	return true
}

func (s *Store) meetBuildingRequireTx(ctx context.Context, tx pgx.Tx, userID int64, require map[string]int64) (bool, error) {
	if len(require) == 0 {
		return true, nil
	}
	for key, minRequired := range require {
		required := minRequired
		if required <= 0 {
			continue
		}
		var level int64
		err := tx.QueryRow(ctx, `
			SELECT level
			FROM player_buildings
			WHERE user_id = $1 AND building_id = $2
		`, userID, key).Scan(&level)
		if errors.Is(err, pgx.ErrNoRows) {
			level = 0
		} else if err != nil {
			return false, err
		}
		if level < required {
			return false, nil
		}
	}
	return true, nil
}

func (s *Store) resolveDurationAndBonusTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	actorType string,
	actorID int64,
	spec ActionSpec,
	baseDuration time.Duration,
	additiveEfficiency ...float64,
) (time.Duration, map[string]float64, error) {
	if baseDuration <= 0 {
		baseDuration = time.Second
	}
	efficiency, err := s.actionEfficiencyBonusTx(ctx, tx, userID, normalizeActorType(actorType), actorID, spec.MainType)
	if err != nil {
		return 0, nil, err
	}
	for _, add := range additiveEfficiency {
		efficiency += add
	}
	minMultiplier := 0.05
	if normalizeActorType(actorType) == ActionActorTypeKitty {
		minMultiplier = 0.5
	}
	resolved := computeEffectiveActionDurationWithFloor(baseDuration, efficiency, minMultiplier)
	return resolved, map[string]float64{}, nil
}

func (s *Store) resolveDurationAndBonusWithTotals(
	baseDuration time.Duration,
	spec ActionSpec,
	attrTotals map[string]float64,
) (time.Duration, map[string]float64, error) {
	if baseDuration <= 0 {
		baseDuration = time.Second
	}
	efficiency := s.actionEfficiencyBonusWithTotals(attrTotals, spec.MainType)
	resolved := computeEffectiveActionDurationWithFloor(baseDuration, efficiency, 0.05)
	return resolved, map[string]float64{}, nil
}

func (s *Store) actionEfficiencyBonusTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	actorType string,
	actorID int64,
	actionMainType string,
) (float64, error) {
	keys := collectActionEfficiencyAttributeKeys(s.gameData.Attributes, actionMainType)
	if len(keys) == 0 {
		return 0, nil
	}
	var rows pgx.Rows
	var err error
	switch actorType {
	case ActionActorTypePlayer:
		rows, err = tx.Query(ctx, `
			SELECT attr_key, final_value
			FROM player_attributes
			WHERE user_id = $1
			  AND attr_key = ANY($2)
		`, userID, keys)
	case ActionActorTypeKitty:
		if actorID <= 0 {
			return 0, nil
		}
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, actorID, false); err != nil {
			if errors.Is(err, ErrKittyNotFound) {
				return 0, nil
			}
			return 0, err
		}
		rows, err = tx.Query(ctx, `
			SELECT attr_key, final_value
			FROM player_kitty_attributes
			WHERE kitty_id = $1
			  AND attr_key = ANY($2)
		`, actorID, keys)
	default:
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	bonus := 0.0
	for rows.Next() {
		var attrKey string
		var value float64
		if err := rows.Scan(&attrKey, &value); err != nil {
			return 0, err
		}
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			bonus += value
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return bonus, nil
}

func (s *Store) actionEfficiencyBonusWithTotals(attrTotals map[string]float64, actionMainType string) float64 {
	keys := collectActionEfficiencyAttributeKeys(s.gameData.Attributes, actionMainType)
	if len(keys) == 0 {
		return 0
	}
	bonus := 0.0
	for _, key := range keys {
		bonus += roundStatValue(attrTotals[strings.TrimSpace(key)])
	}
	return roundStatValue(bonus)
}

func collectActionEfficiencyAttributeKeys(attributes []catalog.AttributeDefinition, actionMainType string) []string {
	normalizedMainType := strings.ToLower(strings.TrimSpace(actionMainType))
	if normalizedMainType == "" {
		return nil
	}
	keys := make([]string, 0)
	seen := map[string]struct{}{}
	for _, row := range attributes {
		attrKey := strings.TrimSpace(row.ID)
		if attrKey == "" {
			continue
		}
		relatedMainType := strings.ToLower(strings.TrimSpace(row.RelatedActionMainType))
		if relatedMainType == "" || relatedMainType != normalizedMainType {
			continue
		}
		relatedEffect := strings.ToLower(strings.TrimSpace(row.RelatedActionEffect))
		if relatedEffect == "" {
			relatedEffect = "actionefficiency"
		}
		if relatedEffect != "actionefficiency" {
			continue
		}
		if _, ok := seen[attrKey]; ok {
			continue
		}
		seen[attrKey] = struct{}{}
		keys = append(keys, attrKey)
	}
	return keys
}

func computeEffectiveActionDuration(baseDuration time.Duration, efficiency float64) time.Duration {
	return computeEffectiveActionDurationWithFloor(baseDuration, efficiency, 0.05)
}

func computeEffectiveActionDurationWithFloor(baseDuration time.Duration, efficiency float64, minMultiplier float64) time.Duration {
	if baseDuration <= 0 {
		baseDuration = time.Second
	}
	if math.IsNaN(minMultiplier) || math.IsInf(minMultiplier, 0) || minMultiplier <= 0 {
		minMultiplier = 0.05
	}
	resolved := baseDuration
	if math.IsNaN(efficiency) || math.IsInf(efficiency, 0) {
		efficiency = 0
	}
	if efficiency >= 0 {
		denominator := math.Max(0.01, 1+efficiency)
		resolved = time.Duration(float64(baseDuration) / denominator)
		minDuration := time.Duration(float64(baseDuration) * minMultiplier)
		if resolved < minDuration {
			resolved = minDuration
		}
	} else {
		resolved = time.Duration(float64(baseDuration) * (1 + math.Abs(efficiency)))
	}
	return normalizeActionQueueRunDuration(resolved)
}

func normalizeActionQueueRunDuration(duration time.Duration) time.Duration {
	if duration < minimumActionQueueRunDuration {
		return minimumActionQueueRunDuration
	}
	return duration
}

func mergeAttrExp(base map[string]float64, extra map[string]float64) map[string]float64 {
	out := make(map[string]float64)
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] += v
	}
	return out
}

func scaleAttrExp(attr map[string]float64, multiplier int64) map[string]float64 {
	if multiplier <= 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(attr))
	for k, v := range attr {
		out[k] = v * float64(multiplier)
	}
	return out
}

func computeCompletedRuns(elapsed time.Duration, duration time.Duration, remaining int) int {
	if remaining <= 0 {
		return 0
	}
	if duration <= 0 {
		duration = minimumActionQueueRunDuration
	}
	if elapsed < 0 {
		elapsed = 0
	}
	completed := int(elapsed / duration)
	if completed <= 0 {
		completed = 1
	}
	if completed > remaining {
		completed = remaining
	}
	return completed
}

type queueRunTiming struct {
	StartAt         time.Time
	CurrentDuration time.Duration
	NextDuration    time.Duration
	HasCurrentRun   bool
}

func resolveQueueRunTiming(item QueueRuntimeItem, now time.Time, nextDuration time.Duration) queueRunTiming {
	windowDuration := time.Duration(0)
	if item.RunStartAt != nil && item.RunEndsAt != nil && item.RunEndsAt.After(*item.RunStartAt) {
		windowDuration = item.RunEndsAt.Sub(*item.RunStartAt)
	}
	if nextDuration <= 0 && windowDuration > 0 {
		nextDuration = windowDuration
	}
	nextDuration = normalizeQueueRuntimeDuration(nextDuration, item.PayloadRaw)
	startAt := now.Add(-nextDuration)
	if item.RunEndsAt != nil {
		startAt = item.RunEndsAt.Add(-nextDuration)
	}
	if item.RunStartAt != nil {
		startAt = *item.RunStartAt
	}
	currentDuration := nextDuration
	hasCurrentRun := false
	if windowDuration > 0 {
		currentDuration = windowDuration
		hasCurrentRun = true
	}
	return queueRunTiming{
		StartAt:         startAt,
		CurrentDuration: currentDuration,
		NextDuration:    nextDuration,
		HasCurrentRun:   hasCurrentRun,
	}
}

func completedRunsForQueueTiming(timing queueRunTiming, now time.Time, remaining int) (int, time.Time) {
	if remaining <= 0 {
		return 0, timing.StartAt
	}
	elapsed := now.Sub(timing.StartAt)
	if elapsed < 0 {
		elapsed = 0
	}
	if !timing.HasCurrentRun {
		completed := computeCompletedRuns(elapsed, timing.NextDuration, remaining)
		return completed, completedAtForQueueTiming(timing, completed)
	}
	if elapsed < timing.CurrentDuration {
		return 0, timing.StartAt
	}
	completed := 1
	elapsed -= timing.CurrentDuration
	remaining--
	if remaining > 0 {
		extra := int(elapsed / timing.NextDuration)
		if extra > remaining {
			extra = remaining
		}
		completed += extra
	}
	return completed, completedAtForQueueTiming(timing, completed)
}

func completedAtForQueueTiming(timing queueRunTiming, completed int) time.Time {
	if completed <= 0 {
		return timing.StartAt
	}
	if timing.HasCurrentRun {
		completedAt := timing.StartAt.Add(timing.CurrentDuration)
		if completed > 1 {
			completedAt = completedAt.Add(time.Duration(completed-1) * timing.NextDuration)
		}
		return completedAt
	}
	return timing.StartAt.Add(time.Duration(completed) * timing.NextDuration)
}

func nextQueueCarryStartAt(now, completedAt time.Time, allowBatch bool, offlineLog bool) time.Time {
	if allowBatch || offlineLog {
		if !completedAt.IsZero() {
			return completedAt
		}
	}
	return now
}

func nextQueueRunWindow(now time.Time, result SettleResult, nextPayload json.RawMessage, allowBatch bool, offlineLog bool) (time.Time, time.Time) {
	// A phase checkpoint belongs to an existing action, not a fresh action run.
	// Rebasing it (or clamping short phases) desynchronizes the executor clock.
	if result.Progressed && !result.NextRunStartAt.IsZero() && result.NextRunEndsAt.After(result.NextRunStartAt) {
		return result.NextRunStartAt, result.NextRunEndsAt
	}
	duration := result.NextRunEndsAt.Sub(result.NextRunStartAt)
	if duration <= 0 {
		duration = parseQueueRuntimeDuration(nextPayload)
	}
	duration = normalizeQueueRuntimeDuration(duration, nextPayload)
	if allowBatch || offlineLog {
		return result.NextRunStartAt, result.NextRunStartAt.Add(duration)
	}
	return now, now.Add(duration)
}

func (s *Store) hasActionQueueDueUntilTx(ctx context.Context, tx pgx.Tx, userID int64, targetAt time.Time) (bool, error) {
	var hasDue bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM player_action_queue q
			WHERE q.user_id = $1
			  AND (
			    (
			      q.status = 'running'
			      AND (q.current_run_ends_at IS NULL OR q.current_run_ends_at <= $2)
			    )
			    OR (
			      q.status = 'pending'
			      AND NOT EXISTS (
			        SELECT 1
			        FROM player_action_queue r
			        WHERE r.user_id = q.user_id
			          AND r.status = 'running'
			          AND r.actor_type = q.actor_type
			          AND r.actor_id = q.actor_id
			      )
			    )
			  )
			  AND NOT (
			    q.actor_type = 'kitty'
			    AND EXISTS (
			      SELECT 1
			      FROM player_kitties k
			      WHERE k.user_id = q.user_id
			        AND k.id = q.actor_id
			        AND k.status = $3
			    )
			  )
		)
	`, userID, targetAt, KittyStatusInCombat).Scan(&hasDue)
	return hasDue, err
}

func (s *Store) hasPlayerActionQueueDueUntilTx(ctx context.Context, tx pgx.Tx, userID int64, targetAt time.Time) (bool, error) {
	var hasDue bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM player_action_queue q
			WHERE q.user_id = $1
			  AND q.actor_type = 'player'
			  AND q.actor_id = 0
			  AND (
			    (
			      q.status = 'running'
			      AND (q.current_run_ends_at IS NULL OR q.current_run_ends_at <= $2)
			    )
			    OR (
			      q.status = 'pending'
			      AND NOT EXISTS (
			        SELECT 1
			        FROM player_action_queue r
			        WHERE r.user_id = q.user_id
			          AND r.status = 'running'
			          AND r.actor_type = q.actor_type
			          AND r.actor_id = q.actor_id
			      )
			    )
			  )
		)
	`, userID, targetAt).Scan(&hasDue)
	return hasDue, err
}

func offlineCapWindowShift(settleAt, realNow time.Time) time.Duration {
	if settleAt.IsZero() || realNow.IsZero() || !realNow.After(settleAt) {
		return 0
	}
	return realNow.Sub(settleAt)
}

func (s *Store) shiftOfflineCappedRunningQueueWindowsTx(ctx context.Context, tx pgx.Tx, userID int64, shift time.Duration, now time.Time) error {
	if userID <= 0 || shift <= 0 {
		return nil
	}
	shiftMS := shift.Milliseconds()
	if shiftMS <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE player_action_queue
		SET current_run_started_at = current_run_started_at + ($2::bigint * INTERVAL '1 millisecond'),
		    current_run_ends_at = current_run_ends_at + ($2::bigint * INTERVAL '1 millisecond'),
		    updated_at = $3
		WHERE user_id = $1
		  AND status = 'running'
		  AND current_run_started_at IS NOT NULL
		  AND current_run_ends_at IS NOT NULL
	`, userID, shiftMS, now)
	return err
}

func computeAdditionalStarts(completed int, doneAfter int, totalCount int) int {
	if completed <= 0 {
		return 0
	}
	additionalStarts := completed - 1
	if doneAfter < totalCount {
		additionalStarts = completed
	}
	if additionalStarts < 0 {
		return 0
	}
	return additionalStarts
}

func adjustCompletedForAffordableStarts(initialCompleted int, doneBefore int, totalCount int, affordableStarts int64) (int, bool, int64) {
	if initialCompleted <= 0 {
		return 0, false, 0
	}
	remaining := totalCount - doneBefore
	if remaining <= 0 {
		return 0, false, 0
	}
	if initialCompleted > remaining {
		initialCompleted = remaining
	}
	doneAfter := doneBefore + initialCompleted
	requiredStarts := int64(computeAdditionalStarts(initialCompleted, doneAfter, totalCount))
	if requiredStarts <= affordableStarts {
		return initialCompleted, false, requiredStarts
	}

	maxCompletedWithCancel := int(affordableStarts + 1)
	if maxCompletedWithCancel < 1 {
		maxCompletedWithCancel = 1
	}
	if maxCompletedWithCancel > remaining {
		maxCompletedWithCancel = remaining
	}
	doneAfter = doneBefore + maxCompletedWithCancel
	if doneAfter >= totalCount {
		startsToCharge := int64(computeAdditionalStarts(maxCompletedWithCancel, doneAfter, totalCount))
		return maxCompletedWithCancel, false, startsToCharge
	}
	startsToCharge := int64(maxCompletedWithCancel - 1)
	if startsToCharge < 0 {
		startsToCharge = 0
	}
	return maxCompletedWithCancel, true, startsToCharge
}

type actionCostEntry struct {
	ItemID   string
	Quantity int64
}

func buildActionCostEntries(costs map[string]int64, multiplier int64) ([]actionCostEntry, error) {
	if multiplier <= 0 || len(costs) == 0 {
		return []actionCostEntry{}, nil
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	entries := make([]actionCostEntry, 0, len(costs))
	for itemID, qty := range costs {
		if qty <= 0 {
			continue
		}
		if qty > maxInt64/multiplier {
			return nil, ErrInventoryQuantityOverflow
		}
		entries = append(entries, actionCostEntry{
			ItemID:   itemID,
			Quantity: qty * multiplier,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ItemID < entries[j].ItemID
	})
	return entries, nil
}

func actionCostItemIDs(entries []actionCostEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.ItemID)
	}
	return out
}

func (s *Store) lockConsumableQuantitiesByTemplateIDsTx(ctx context.Context, tx pgx.Tx, userID int64, itemIDs []string) (map[string]int64, error) {
	if len(itemIDs) == 0 {
		return map[string]int64{}, nil
	}
	rows, err := s.lockTemplateCommodityRowsByTemplateIDsTx(ctx, tx, userID, itemIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(itemIDs))
	for _, itemID := range itemIDs {
		out[itemID] = 0
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	for _, row := range rows {
		owned := out[row.TemplateItem]
		if row.Quantity > maxInt64-owned {
			return nil, ErrInventoryQuantityOverflow
		}
		out[row.TemplateItem] = owned + row.Quantity
	}
	return out, nil
}

func (s *Store) applyActionCostsTx(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64) error {
	entries, err := buildActionCostEntries(costs, 1)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	ownedByItem, err := s.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(entries))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if ownedByItem[entry.ItemID] < entry.Quantity {
			return ErrInsufficientItem
		}
	}
	for _, entry := range entries {
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, entry.ItemID, entry.Quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "action_queue",
			Details: map[string]any{
				"type": "action_cost",
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) maxAffordableActionStartsTx(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64, maxStarts int64) (int64, error) {
	if maxStarts <= 0 {
		return 0, nil
	}
	entries, err := buildActionCostEntries(costs, 1)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return maxStarts, nil
	}
	ownedByItem, err := s.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(entries))
	if err != nil {
		return 0, err
	}
	return maxAffordableStartsByOwned(entries, ownedByItem, maxStarts), nil
}

func maxAffordableStartsByOwned(entries []actionCostEntry, ownedByItem map[string]int64, maxStarts int64) int64 {
	if maxStarts <= 0 {
		return 0
	}
	affordable := maxStarts
	for _, entry := range entries {
		if entry.Quantity <= 0 {
			continue
		}
		canStart := ownedByItem[entry.ItemID] / entry.Quantity
		if canStart < affordable {
			affordable = canStart
		}
		if affordable <= 0 {
			return 0
		}
	}
	if affordable < 0 {
		return 0
	}
	return affordable
}

func (s *Store) inventoryQuantityByTemplateTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string) (int64, error) {
	return s.ConsumableQuantityByTemplateTx(ctx, tx, userID, itemID)
}

func (s *Store) applyActionCostsBatchTx(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64, count int64, meta InventoryLedgerMeta) error {
	if count <= 0 {
		return nil
	}
	entries, err := buildActionCostEntries(costs, count)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	ownedByItem, err := s.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(entries))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if ownedByItem[entry.ItemID] < entry.Quantity {
			return ErrInsufficientItem
		}
	}
	for _, entry := range entries {
		entryMeta := meta
		if entryMeta.SourceSurface == "" {
			entryMeta.SourceSurface = "inventory"
		}
		if entryMeta.TargetSurface == "" {
			entryMeta.TargetSurface = "action_queue"
		}
		if entryMeta.Details == nil {
			entryMeta.Details = map[string]any{
				"type":  "action_cost_batch",
				"count": count,
			}
		}
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, entry.ItemID, entry.Quantity, entryMeta); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) refundActionCostsTx(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64) error {
	for key, qty := range costs {
		if qty <= 0 {
			continue
		}
		if err := s.LedgerGrantTemplateTx(ctx, tx, userID, key, qty, InventoryLedgerMeta{
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type": "action_refund",
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyActionRewardsTx(ctx context.Context, tx pgx.Tx, userID int64, actionID string, menu string, rewards map[string]ActionItemRewardSpec) error {
	_ = menu
	_ = actionID
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for key, reward := range rewards {
		qty := resolveActionRewardQuantity(reward, rng)
		if qty <= 0 {
			continue
		}
		grantOptions := inventoryGrantOptionsFromActionReward(reward, rng)
		if err := s.LedgerGrantTemplateWithOptionsTx(ctx, tx, userID, key, qty, InventoryLedgerMeta{
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":     "action_reward",
				"actionId": actionID,
				"menu":     menu,
			},
		}, grantOptions); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyActionRewardsBatchTx(ctx context.Context, tx pgx.Tx, userID int64, queueID int64, doneBefore int, count int, rewards map[string]ActionItemRewardSpec, phase string) (map[string]int64, error) {
	if count <= 0 {
		return map[string]int64{}, nil
	}
	rng := queueRewardRNG(userID, queueID, doneBefore)
	out := make(map[string]int64)
	for key, reward := range rewards {
		qty := resolveActionRewardBatchQuantity(reward, count, rng)
		if qty <= 0 {
			continue
		}
		grantOptions := inventoryGrantOptionsFromActionReward(reward, rng)
		if err := s.LedgerGrantTemplateWithOptionsTx(ctx, tx, userID, key, qty, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryActionQueue,
			Direction:     InventoryAuditDirectionGain,
			SubjectType:   "queue",
			SubjectID:     strconv.FormatInt(queueID, 10),
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":    "action_reward_batch",
				"queueId": queueID,
				"count":   count,
				"phase":   phase,
			},
		}, grantOptions); err != nil {
			return nil, err
		}
		out[key] += qty
	}
	return out, nil
}

func (s *Store) applyActionRewardsBatchWithInheritanceTx(ctx context.Context, tx pgx.Tx, userID int64, queueID int64, doneBefore int, count int, rewards map[string]ActionItemRewardSpec, inheritedEnhanceLevel int, phase string) (map[string]int64, error) {
	if count <= 0 {
		return map[string]int64{}, nil
	}
	rng := queueRewardRNG(userID, queueID, doneBefore)
	out := make(map[string]int64)
	for key, reward := range rewards {
		qty := resolveActionRewardBatchQuantity(reward, count, rng)
		if qty <= 0 {
			continue
		}
		template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(key))
		targetLevel := inheritedEnhanceLevel
		if template != nil {
			maxLevel := int(template.EnhanceMaxLevel)
			if targetLevel > maxLevel {
				targetLevel = maxLevel
			}
		} else {
			targetLevel = 0
		}
		if targetLevel > 0 {
			grantOptions := inventoryGrantOptionsFromActionReward(reward, rng)
			for i := int64(0); i < qty; i++ {
				instanceID, err := s.AddInventoryEnhancedInstanceWithOptionsTx(ctx, tx, userID, key, targetLevel, grantOptions)
				if err != nil {
					return nil, err
				}
				if err := s.LedgerLogOnlyTx(ctx, tx, userID, key, 1, InventoryLedgerMeta{
					Category:      InventoryAuditCategoryActionQueue,
					Direction:     InventoryAuditDirectionGain,
					SubjectType:   "queue",
					SubjectID:     strconv.FormatInt(queueID, 10),
					SourceSurface: "action_queue",
					TargetSurface: "inventory",
					InstanceID:    &instanceID,
					Details: map[string]any{
						"type":                "action_reward_batch_inherited",
						"queueId":             queueID,
						"count":               count,
						"phase":               phase,
						"inheritedEnhanceLvl": targetLevel,
					},
				}); err != nil {
					return nil, err
				}
			}
			out[key] += qty
			continue
		}
		grantOptions := inventoryGrantOptionsFromActionReward(reward, rng)
		if err := s.LedgerGrantTemplateWithOptionsTx(ctx, tx, userID, key, qty, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryActionQueue,
			Direction:     InventoryAuditDirectionGain,
			SubjectType:   "queue",
			SubjectID:     strconv.FormatInt(queueID, 10),
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":    "action_reward_batch",
				"queueId": queueID,
				"count":   count,
				"phase":   phase,
			},
		}, grantOptions); err != nil {
			return nil, err
		}
		out[key] += qty
	}
	return out, nil
}

func inventoryGrantOptionsFromActionReward(reward ActionItemRewardSpec, rng *rand.Rand) InventoryGrantOptions {
	if strings.TrimSpace(reward.GrantPolicy) != InventoryGrantPolicyRollDefaultAffixes {
		return InventoryGrantOptions{}
	}
	return InventoryGrantOptions{
		Policy:                    InventoryGrantPolicyRollDefaultAffixes,
		DefaultAffixRarityWeights: reward.DefaultAffixRarityWeights,
		RNG:                       rng,
	}
}

func queueRewardRNG(userID int64, queueID int64, doneBefore int) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte("queue-reward"))
	writeI64(h, userID)
	writeI64(h, queueID)
	writeI64(h, int64(doneBefore))
	seed := int64(h.Sum64())
	if seed == 0 {
		seed = 1
	}
	return rand.New(rand.NewSource(seed))
}

func writeI64(h hash.Hash64, v int64) {
	b := [8]byte{
		byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32),
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v),
	}
	_, _ = h.Write(b[:])
}

func resolveActionRewardBatchQuantity(reward ActionItemRewardSpec, count int, rng *rand.Rand) int64 {
	if count <= 0 {
		return 0
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	percent := 1.0
	if reward.Percent != nil {
		percent = *reward.Percent
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	hits := count
	if percent < 1 {
		hits = 0
		for i := 0; i < count; i++ {
			if rng.Float64() < percent {
				hits++
			}
		}
	}
	if hits <= 0 {
		return 0
	}
	if reward.Count != nil {
		if *reward.Count <= 0 {
			return 0
		}
		return *reward.Count * int64(hits)
	}
	if reward.Range != nil {
		minVal := reward.Range.Min
		maxVal := reward.Range.Max
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		if maxVal <= 0 {
			return 0
		}
		if minVal < 0 {
			minVal = 0
		}
		total := int64(0)
		if minVal == maxVal {
			return minVal * int64(hits)
		}
		for i := 0; i < hits; i++ {
			total += minVal + rng.Int63n(maxVal-minVal+1)
		}
		return total
	}
	return 0
}

func resolveActionRewardQuantity(reward ActionItemRewardSpec, rng *rand.Rand) int64 {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	percent := 1.0
	if reward.Percent != nil {
		percent = *reward.Percent
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	if percent < 1 && rng.Float64() >= percent {
		return 0
	}
	if reward.Count != nil {
		if *reward.Count <= 0 {
			return 0
		}
		return *reward.Count
	}
	if reward.Range != nil {
		minVal := reward.Range.Min
		maxVal := reward.Range.Max
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		if maxVal <= 0 {
			return 0
		}
		if minVal < 0 {
			minVal = 0
		}
		if minVal == maxVal {
			return minVal
		}
		return minVal + rng.Int63n(maxVal-minVal+1)
	}
	return 0
}

func (s *Store) applyAttrExpTx(ctx context.Context, tx pgx.Tx, userID int64, attrExp map[string]float64) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	return s.applyAttrExpLockedTx(ctx, tx, userID, attrExp)
}

func (s *Store) applyAttrExpLockedTx(ctx context.Context, tx pgx.Tx, userID int64, attrExp map[string]float64) error {
	_, err := s.applyAttrExpLockedResultTx(ctx, tx, userID, attrExp)
	return err
}

type attributeExpApplyResult struct {
	Changed      bool
	LevelChanged bool
}

func (s *Store) applyAttrExpLockedResultTx(ctx context.Context, tx pgx.Tx, userID int64, attrExp map[string]float64, accepted ...map[string]float64) (attributeExpApplyResult, error) {
	result := attributeExpApplyResult{}
	if len(attrExp) == 0 {
		return result, nil
	}
	attrKeys := make([]string, 0, len(attrExp))
	for key := range attrExp {
		if strings.TrimSpace(key) != "" {
			attrKeys = append(attrKeys, key)
		}
	}
	sort.Strings(attrKeys)
	for _, key := range attrKeys {
		gain := attrExp[key]
		if strings.TrimSpace(key) == "" || gain == 0 {
			continue
		}
		gain = roundFloat(gain, 4)
		if gain == 0 {
			continue
		}
		var beforeExp float64
		loadErr := tx.QueryRow(ctx, `
			SELECT attr_exp
			FROM player_attributes
			WHERE user_id = $1 AND attr_key = $2
			FOR UPDATE
		`, userID, key).Scan(&beforeExp)
		if errors.Is(loadErr, pgx.ErrNoRows) {
			if err := s.ensureAllPlayerAttributesTx(ctx, tx, userID); err != nil {
				return result, err
			}
			loadErr = tx.QueryRow(ctx, `
				SELECT attr_exp
				FROM player_attributes
				WHERE user_id = $1 AND attr_key = $2
				FOR UPDATE
			`, userID, key).Scan(&beforeExp)
			if errors.Is(loadErr, pgx.ErrNoRows) {
				continue
			}
		}
		if loadErr != nil {
			return result, loadErr
		}
		beforeProgress, err := s.resolveAttributeProgress(key, beforeExp)
		if err != nil {
			return result, err
		}
		afterExp, err := s.attributeExpProfile.clampTotalExp(key, roundFloat(beforeExp+gain, 4))
		if err != nil {
			return result, err
		}
		afterExp = roundFloat(afterExp, 4)
		if len(accepted) > 0 && accepted[0] != nil {
			accepted[0][key] = roundFloat(afterExp-beforeExp, 4)
		}
		afterProgress, err := s.resolveAttributeProgress(key, afterExp)
		if err != nil {
			return result, err
		}
		afterLevel := afterProgress.Level
		levelDelta := afterLevel - beforeProgress.Level
		if levelDelta == 0 && math.Abs(afterExp-beforeExp) < 1e-9 {
			continue
		}
		if _, err = tx.Exec(ctx, `
			UPDATE player_attributes
			SET attr_exp = $3,
			    base_level = $4,
			    final_value = final_value + $5,
			    lock_version = lock_version + CASE WHEN $5 = 0 THEN 1 ELSE 0 END,
			    updated_at = now()
			WHERE user_id = $1 AND attr_key = $2
		`, userID, key, afterExp, afterLevel, float64(levelDelta)); err != nil {
			return result, err
		}
		result.Changed = true
		result.LevelChanged = result.LevelChanged || levelDelta != 0
		if key == "battle" && levelDelta > 0 {
			if _, err = tx.Exec(ctx, `
				INSERT INTO player_battle_point_state(user_id, total_points, spent_points, updated_at)
				VALUES($1, $2, 0, now())
				ON CONFLICT (user_id)
				DO UPDATE SET total_points = player_battle_point_state.total_points + EXCLUDED.total_points,
				              updated_at = now()
			`, userID, levelDelta); err != nil {
				return result, err
			}
		}
	}
	if !result.LevelChanged {
		return result, nil
	}
	return result, s.onPlayerAttributesChangedTx(ctx, tx, userID)
}

func roundFloat(value float64, precision int) float64 {
	if precision <= 0 {
		return math.Round(value)
	}
	pow := math.Pow10(precision)
	return math.Round(value*pow) / pow
}

func normalizeActorType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ActionActorTypeKitty:
		return ActionActorTypeKitty
	default:
		return ActionActorTypePlayer
	}
}

func actorLaneKey(actorType string, actorID int64) string {
	return normalizeActorType(actorType) + ":" + strconv.FormatInt(actorID, 10)
}

func scaleDuration(duration time.Duration, multiplier float64) time.Duration {
	if duration <= 0 {
		return duration
	}
	if multiplier <= 0 {
		return duration
	}
	scaled := time.Duration(float64(duration) * multiplier)
	return normalizeActionQueueRunDuration(scaled)
}

func scaleAttrExpByMultiplier(attr map[string]float64, multiplier float64) map[string]float64 {
	if len(attr) == 0 {
		return map[string]float64{}
	}
	if multiplier < 0 {
		multiplier = 0
	}
	if multiplier == 1 {
		out := make(map[string]float64, len(attr))
		for key, value := range attr {
			out[key] = value
		}
		return out
	}
	out := make(map[string]float64, len(attr))
	for key, value := range attr {
		scaled := value * multiplier
		if scaled == 0 {
			continue
		}
		out[key] = scaled
	}
	return out
}

func accumulateActionActorSummary(summaryByLane map[string]*ActionActorSummary, actorType string, actorID int64, settled SettleResult) {
	normalizedActorType := normalizeActorType(actorType)
	normalizedActorID := actorID
	if normalizedActorType == ActionActorTypePlayer {
		normalizedActorID = 0
	}
	laneKey := actorLaneKey(normalizedActorType, normalizedActorID)
	row, exists := summaryByLane[laneKey]
	if !exists {
		row = &ActionActorSummary{
			ActorType:   normalizedActorType,
			ActorID:     normalizedActorID,
			ItemRewards: map[string]int64{},
			ItemCosts:   map[string]int64{},
			AttrExp:     map[string]float64{},
		}
		summaryByLane[laneKey] = row
	}
	row.CompletedSteps += int64(settled.Completed)
	if len(settled.OwnerAttrExp) > 0 {
		if row.OwnerAttrExp == nil {
			row.OwnerAttrExp = map[string]float64{}
		}
		for key, value := range settled.OwnerAttrExp {
			row.OwnerAttrExp[key] += value
		}
	}
	for itemID, qty := range settled.ItemRewards {
		if qty <= 0 {
			continue
		}
		row.ItemRewards[itemID] += qty
	}
	for itemID, qty := range settled.ItemCosts {
		if qty <= 0 {
			continue
		}
		row.ItemCosts[itemID] += qty
	}
	accumulateSettlementAttrExp(row.AttrExp, settled, true)
}

func actionActorSummaryMap(rows []ActionActorSummary) map[string]*ActionActorSummary {
	summaryByLane := make(map[string]*ActionActorSummary, len(rows))
	for index := range rows {
		row := rows[index]
		row.ActorType = normalizeActorType(row.ActorType)
		if row.ActorType == ActionActorTypePlayer {
			row.ActorID = 0
		}
		copyRow := row
		summaryByLane[actorLaneKey(copyRow.ActorType, copyRow.ActorID)] = &copyRow
	}
	return summaryByLane
}

func markActionActorEquipmentChanged(summaryByLane map[string]*ActionActorSummary, actorType string, actorID int64) {
	normalizedActorType := normalizeActorType(actorType)
	normalizedActorID := actorID
	if normalizedActorType == ActionActorTypePlayer {
		normalizedActorID = 0
	}
	laneKey := actorLaneKey(normalizedActorType, normalizedActorID)
	row, exists := summaryByLane[laneKey]
	if !exists {
		row = &ActionActorSummary{
			ActorType:   normalizedActorType,
			ActorID:     normalizedActorID,
			ItemRewards: map[string]int64{},
			ItemCosts:   map[string]int64{},
			AttrExp:     map[string]float64{},
		}
		summaryByLane[laneKey] = row
	}
	row.EquipmentChanged = true
}

func buildActionActorSummaries(summaryByLane map[string]*ActionActorSummary) []ActionActorSummary {
	if len(summaryByLane) == 0 {
		return nil
	}
	out := make([]ActionActorSummary, 0, len(summaryByLane))
	for _, row := range summaryByLane {
		if row == nil {
			continue
		}
		clone := ActionActorSummary{
			ActorType:        normalizeActorType(row.ActorType),
			ActorID:          row.ActorID,
			CompletedSteps:   row.CompletedSteps,
			ItemRewards:      row.ItemRewards,
			ItemCosts:        row.ItemCosts,
			AttrExp:          row.AttrExp,
			OwnerAttrExp:     row.OwnerAttrExp,
			EquipmentChanged: row.EquipmentChanged,
		}
		if clone.ActorType == ActionActorTypePlayer {
			clone.ActorID = 0
		}
		out = append(out, clone)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		if left.ActorType != right.ActorType {
			if left.ActorType == ActionActorTypePlayer {
				return true
			}
			if right.ActorType == ActionActorTypePlayer {
				return false
			}
			return left.ActorType < right.ActorType
		}
		return left.ActorID < right.ActorID
	})
	return out
}

func (s *Store) resolveActionActorTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64) (string, int64, error) {
	switch normalizeActorType(actorType) {
	case ActionActorTypePlayer:
		return ActionActorTypePlayer, 0, nil
	case ActionActorTypeKitty:
		if actorID <= 0 {
			return "", 0, ErrActionActorInvalid
		}
		ok, err := s.isOwnedKittyTx(ctx, tx, userID, actorID)
		if err != nil {
			return "", 0, err
		}
		if !ok {
			return "", 0, ErrActionActorForbidden
		}
		return ActionActorTypeKitty, actorID, nil
	default:
		return "", 0, ErrActionActorInvalid
	}
}

func (s *Store) reconcileKittyLaneStatusesTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		WITH desired AS MATERIALIZED (
			SELECT k.id,
			       CASE WHEN active.actor_id IS NULL THEN $3::text ELSE $2::text END AS status
			FROM player_kitties k
			LEFT JOIN (
				SELECT DISTINCT actor_id
				FROM player_action_queue
				WHERE user_id = $1
				  AND actor_type = 'kitty'
				  AND status IN ('pending', 'running')
			) active ON active.actor_id = k.id
			WHERE k.user_id = $1
			  AND k.status IN ($2, $3)
		)
		UPDATE player_kitties k
		SET status = desired.status,
		    updated_at = now()
		FROM desired
		WHERE k.user_id = $1
		  AND k.id = desired.id
		  AND k.status IS DISTINCT FROM desired.status
	`, userID, KittyStatusInLifeQueue, KittyStatusIdle)
	return err
}
