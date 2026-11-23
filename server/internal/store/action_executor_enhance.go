package store

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type enhanceExecutor struct {
	store *Store
}

type enhanceExecutorArgs struct {
	EquipmentInstanceID int64  `json:"equipmentInstanceId"`
	MaxAttempts         int    `json:"maxAttempts"`
	TargetEnhanceLevel  int    `json:"targetEnhanceLevel"`
	UseProtect          bool   `json:"useProtect"`
	ProtectStartLevel   int    `json:"protectStartLevel,omitempty"`
	ProtectItemID       string `json:"protectItemId,omitempty"`
}

func (a enhanceExecutorArgs) QueueTotalCount() int {
	return a.MaxAttempts
}

func (enhanceExecutorArgs) RequiresSingleQueueRun() bool { return true }

type enhanceExecutorRuntime struct {
	TemplateID          string  `json:"templateId"`
	EquipmentInstanceID int64   `json:"equipmentInstanceId"`
	DurationMs          int64   `json:"durationMs"`
	MaxLevel            int     `json:"maxLevel"`
	EnhanceExp          float64 `json:"enhanceExp"`
	TargetEnhanceLevel  int     `json:"targetEnhanceLevel"`
	UseProtect          bool    `json:"useProtect"`
	ProtectStartLevel   int     `json:"protectStartLevel,omitempty"`
	ProtectItemID       string  `json:"protectItemId,omitempty"`
}

type enhanceRunContext struct {
	BeforeLevel      int                `json:"beforeLevel"`
	BaseSuccessRate  float64            `json:"baseSuccessRate"`
	SuccessRateBonus float64            `json:"successRateBonus"`
	SuccessRate      float64            `json:"successRate"`
	UsedProtect      bool               `json:"usedProtect"`
	Costs            map[string]int64   `json:"costs"`
	DeltaAttrs       map[string]float64 `json:"deltaAttrs"`
}

type enhanceFastForwardPlan struct {
	AttemptsDone       int
	FinalLevel         int
	ItemCosts          map[string]int64
	ChargeCosts        map[string]int64
	AttrExpGain        map[string]float64
	EnhanceResults     []EnhanceAttemptResult
	CancelAfterSettle  bool
	StopReason         string
	MilestoneAttemptNo int
}

const (
	EnhanceStopReasonTargetReached             = "TARGET_REACHED"
	EnhanceStopReasonMaxLevelReached           = "MAX_LEVEL_REACHED"
	EnhanceStopReasonCostInsufficient          = "COST_INSUFFICIENT"
	EnhanceStopReasonProtectItemInsufficient   = "PROTECT_ITEM_INSUFFICIENT"
	EnhanceStopReasonTargetInvalid             = "TARGET_INVALID"
	EnhanceStopReasonMilestoneAffixUnavailable = "MILESTONE_AFFIX_UNAVAILABLE"
	EnhanceStopReasonAttemptsExhausted         = "ATTEMPTS_EXHAUSTED"
	enhanceSuccessRateBonusAttrKey             = "enhancement_success_rate"
)

func newEnhanceExecutor(store *Store) *enhanceExecutor {
	return &enhanceExecutor{store: store}
}

func (e *enhanceExecutor) ActionID() string {
	return "equipment_enhance"
}

func (e *enhanceExecutor) OfflineProjectionPolicy() QueueOfflineProjectionPolicy {
	return QueueOfflineProjectionImmediate
}

func (e *enhanceExecutor) durationRefreshMode() queueDurationRefreshMode {
	return queueDurationRefreshPending
}

func (e *enhanceExecutor) refreshQueueDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (json.RawMessage, bool, error) {
	runtime, ok := parseEnhanceExecutorRuntime(item.PayloadRaw)
	if !ok {
		return nil, false, errors.New("invalid enhance runtime payload")
	}
	if e.store.enhanceCache == nil {
		return nil, false, errors.New("enhance cache unavailable")
	}
	template, ok := e.store.enhanceCache.Template(runtime.TemplateID)
	if !ok {
		return nil, false, ErrEnhanceTemplateInvalid
	}
	resolvedDuration, _, err := e.store.resolveDurationAndBonusTx(
		ctx, tx, userID, item.ActorType, item.ActorID, item.Spec, template.Duration,
	)
	if err != nil {
		return nil, false, err
	}
	runtime.DurationMs = int64(resolvedDuration / time.Millisecond)
	nextPayload, err := setQueueRuntimePayload(item.PayloadRaw, runtime)
	if err != nil {
		return nil, false, err
	}
	return nextPayload, string(nextPayload) != string(item.PayloadRaw), nil
}

func (e *enhanceExecutor) ValidateAndNormalizeArgs(raw json.RawMessage) (any, error) {
	var args enhanceExecutorArgs
	if len(raw) == 0 {
		return nil, ErrEnhanceArgsInvalid
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, ErrEnhanceArgsInvalid
	}
	if args.EquipmentInstanceID <= 0 || args.MaxAttempts <= 0 || args.TargetEnhanceLevel <= 0 {
		return nil, ErrEnhanceArgsInvalid
	}
	if args.ProtectStartLevel < 0 {
		return nil, ErrEnhanceArgsInvalid
	}
	if args.MaxAttempts > 100000 {
		return nil, ErrEnhanceArgsInvalid
	}
	if args.UseProtect && args.ProtectStartLevel <= 0 {
		return nil, ErrEnhanceArgsInvalid
	}
	if args.UseProtect && strings.TrimSpace(args.ProtectItemID) == "" {
		return nil, ErrEnhanceArgsInvalid
	}
	return args, nil
}

func (e *enhanceExecutor) FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, spec ActionSpec, rawArgs any, _ time.Time) (json.RawMessage, error) {
	args, ok := rawArgs.(enhanceExecutorArgs)
	if !ok {
		return nil, ErrEnhanceArgsInvalid
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	locked, err := e.store.LockInstanceTx(ctx, tx, args.EquipmentInstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnhanceTargetInvalid
		}
		return nil, err
	}
	if locked.UserID != userID {
		return nil, ErrEnhanceTargetInvalid
	}
	if strings.TrimSpace(locked.State) != "inventory" {
		return nil, ErrEnhanceNotInInventory
	}
	templateID := strings.TrimSpace(locked.TemplateItem)
	runtimeTemplate, ok := e.store.enhanceCache.Template(templateID)
	if !ok || runtimeTemplate.MaxLevel <= 0 {
		return nil, ErrEnhanceTemplateInvalid
	}
	if args.TargetEnhanceLevel > runtimeTemplate.MaxLevel {
		return nil, ErrEnhanceArgsInvalid
	}
	if locked.EnhanceLevel >= args.TargetEnhanceLevel {
		return nil, ErrEnhanceTargetInvalid
	}
	if args.UseProtect {
		if len(runtimeTemplate.ProtectItemIDs) == 0 {
			return nil, ErrEnhanceArgsInvalid
		}
		if args.ProtectStartLevel < 1 || args.ProtectStartLevel > runtimeTemplate.MaxLevel {
			return nil, ErrEnhanceArgsInvalid
		}
		if !containsString(runtimeTemplate.ProtectItemIDs, args.ProtectItemID) {
			return nil, ErrEnhanceArgsInvalid
		}
	}
	resolvedDuration, _, err := e.store.resolveDurationAndBonusTx(
		ctx,
		tx,
		userID,
		actorType,
		actorID,
		spec,
		runtimeTemplate.Duration,
	)
	if err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(enhanceExecutorRuntime{
		TemplateID:          templateID,
		EquipmentInstanceID: args.EquipmentInstanceID,
		DurationMs:          int64(resolvedDuration / time.Millisecond),
		MaxLevel:            runtimeTemplate.MaxLevel,
		EnhanceExp:          runtimeTemplate.EnhanceExp,
		TargetEnhanceLevel:  args.TargetEnhanceLevel,
		UseProtect:          args.UseProtect,
		ProtectStartLevel:   args.ProtectStartLevel,
		ProtectItemID:       strings.TrimSpace(args.ProtectItemID),
	})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   spec.ID,
		ExecutorID: e.ActionID(),
		Runtime:    runtimeRaw,
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (e *enhanceExecutor) StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, _ time.Time) (json.RawMessage, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	runtime, ok := parseEnhanceExecutorRuntime(item.PayloadRaw)
	if !ok {
		return nil, ErrEnhanceTargetInvalid
	}
	locked, err := e.store.LockInstanceTx(ctx, tx, runtime.EquipmentInstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnhanceTargetInvalid
		}
		return nil, err
	}
	if locked.UserID != userID {
		return nil, ErrEnhanceTargetInvalid
	}
	if strings.TrimSpace(locked.State) != "inventory" {
		return nil, ErrEnhanceNotInInventory
	}
	if strings.TrimSpace(locked.TemplateItem) != runtime.TemplateID {
		return nil, ErrEnhanceTargetInvalid
	}
	runCtx, stopReason, ok, err := e.prepareEnhanceRunContextTx(ctx, tx, userID, runtime, locked.EnhanceLevel)
	if err != nil {
		return nil, err
	}
	if !ok {
		switch stopReason {
		case EnhanceStopReasonTargetInvalid:
			return nil, ErrEnhanceTargetInvalid
		case EnhanceStopReasonProtectItemInsufficient, EnhanceStopReasonCostInsufficient:
			return nil, ErrInsufficientItem
		default:
			return nil, ErrEnhanceTargetInvalid
		}
	}
	if item.OperationEffects != nil {
		if err := applyEnhanceOperationEffects(&runCtx, *item.OperationEffects); err != nil {
			return nil, err
		}
	}
	return setEnhanceRunContext(item.PayloadRaw, runCtx)
}

func (e *enhanceExecutor) CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	runCtx, ok := parseEnhanceRunContext(item.PayloadRaw)
	if !ok {
		return nil
	}
	return e.refundEnhanceRunContextTx(ctx, tx, userID, runCtx)
}

func (e *enhanceExecutor) SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error) {
	return e.SettleFastForwardTx(ctx, tx, userID, item, now, FastForwardBudget{MaxIterations: 1})
}

func (e *enhanceExecutor) SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return SettleResult{}, err
	}
	runtime, ok := parseEnhanceExecutorRuntime(item.PayloadRaw)
	if !ok {
		return SettleResult{
			CancelAfterSettle: true,
			CancelReason:      ActionQueueCancelReasonTargetInvalid,
			NextPayload:       item.PayloadRaw,
			ItemCosts:         map[string]int64{},
			ItemRewards:       map[string]int64{},
			AttrExp:           map[string]float64{},
			EnhanceResults:    []EnhanceAttemptResult{},
		}, nil
	}
	duration := normalizeActionQueueRunDuration(time.Duration(runtime.DurationMs) * time.Millisecond)
	timing := resolveQueueRunTiming(item, now, duration)
	remaining := item.TotalCount - item.DoneCount
	completed, _ := completedRunsForQueueTiming(timing, now, remaining)
	if budget.MaxIterations > 0 && completed > budget.MaxIterations {
		completed = budget.MaxIterations
	}
	if completed <= 0 {
		return SettleResult{}, nil
	}

	locked, err := e.store.LockInstanceTx(ctx, tx, runtime.EquipmentInstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if runCtx, hasRunCtx := parseEnhanceRunContext(item.PayloadRaw); hasRunCtx {
				if refundErr := e.refundEnhanceRunContextTx(ctx, tx, userID, runCtx); refundErr != nil {
					return SettleResult{}, refundErr
				}
			}
			return SettleResult{
				CancelAfterSettle: true,
				CancelReason:      ActionQueueCancelReasonTargetInvalid,
				NextPayload:       item.PayloadRaw,
				ItemCosts:         map[string]int64{},
				ItemRewards:       map[string]int64{},
				AttrExp:           map[string]float64{},
				EnhanceResults:    []EnhanceAttemptResult{},
			}, nil
		}
		return SettleResult{}, err
	}
	if locked.UserID != userID || strings.TrimSpace(locked.State) != "inventory" || strings.TrimSpace(locked.TemplateItem) != runtime.TemplateID {
		if runCtx, hasRunCtx := parseEnhanceRunContext(item.PayloadRaw); hasRunCtx {
			if refundErr := e.refundEnhanceRunContextTx(ctx, tx, userID, runCtx); refundErr != nil {
				return SettleResult{}, refundErr
			}
		}
		return SettleResult{
			CancelAfterSettle: true,
			CancelReason:      ActionQueueCancelReasonTargetInvalid,
			NextPayload:       item.PayloadRaw,
			ItemCosts:         map[string]int64{},
			ItemRewards:       map[string]int64{},
			AttrExp:           map[string]float64{},
			EnhanceResults:    []EnhanceAttemptResult{},
		}, nil
	}

	firstRunCtx, firstRunCtxOK := parseEnhanceRunContext(item.PayloadRaw)
	budgetStartedAt := budget.StartedAt
	if budgetStartedAt.IsZero() {
		budgetStartedAt = time.Now()
	}
	plan, err := e.planEnhanceFastForwardTx(ctx, tx, userID, item, runtime, locked, completed, firstRunCtx, firstRunCtxOK, budgetStartedAt, budget)
	if err != nil {
		return SettleResult{}, err
	}

	if plan.AttemptsDone <= 0 {
		if plan.CancelAfterSettle {
			return SettleResult{
				CancelAfterSettle: true,
				CancelReason:      enhanceCancelReasonByStopReason(plan.StopReason),
				NextPayload:       item.PayloadRaw,
				ItemCosts:         map[string]int64{},
				ItemRewards:       map[string]int64{},
				AttrExp:           map[string]float64{},
				EnhanceResults:    []EnhanceAttemptResult{},
			}, nil
		}
		return SettleResult{}, nil
	}

	if err := e.consumeEnhanceAggregatedCostsTx(ctx, tx, userID, plan.ChargeCosts); err != nil {
		return SettleResult{}, err
	}
	locked.EnhanceLevel = clampEnhanceLevel(plan.FinalLevel, runtime.MaxLevel)
	if plan.MilestoneAttemptNo > 0 {
		milestoneStopReason, milestoneErr := e.applyEnhanceMilestoneAffixTx(ctx, tx, userID, item, runtime, &locked, plan.MilestoneAttemptNo)
		if milestoneErr != nil {
			return SettleResult{}, milestoneErr
		}
		if milestoneStopReason != "" {
			plan.CancelAfterSettle = true
			plan.StopReason = milestoneStopReason
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET enhance_level = $2, updated_at = now()
		WHERE instance_id = $1
	`, locked.InstanceID, locked.EnhanceLevel); err != nil {
		return SettleResult{}, err
	}
	locked.EnhanceLevel = clampEnhanceLevel(locked.EnhanceLevel, runtime.MaxLevel)
	if err := e.store.refreshLockedInstanceBaseSnapshotTx(ctx, tx, locked); err != nil {
		return SettleResult{}, err
	}
	if err := e.store.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
		return SettleResult{}, err
	}
	if err := e.store.applyAttrExpTx(ctx, tx, userID, plan.AttrExpGain); err != nil {
		return SettleResult{}, err
	}

	doneAfter := item.DoneCount + plan.AttemptsDone
	nextPayload := item.PayloadRaw
	if doneAfter >= item.TotalCount {
		plan.CancelAfterSettle = true
		plan.StopReason = EnhanceStopReasonAttemptsExhausted
	} else if !plan.CancelAfterSettle {
		nextRunCtx, reason, preparedOK, prepErr := e.prepareEnhanceRunContextTx(ctx, tx, userID, runtime, locked.EnhanceLevel)
		if prepErr != nil {
			return SettleResult{}, prepErr
		}
		if !preparedOK {
			plan.CancelAfterSettle = true
			plan.StopReason = reason
		} else {
			snapshot, effectErr := nextEnhanceOperationEffects(item, timing, now, plan.AttemptsDone)
			if effectErr != nil {
				return SettleResult{}, effectErr
			}
			if err := applyEnhanceOperationEffects(&nextRunCtx, snapshot); err != nil {
				return SettleResult{}, err
			}
			for itemID, qty := range nextRunCtx.Costs {
				if qty > 0 {
					if err := addEnhanceCost(plan.ItemCosts, itemID, qty); err != nil {
						return SettleResult{}, err
					}
				}
			}
			nextPayload, err = setEnhanceRunContext(item.PayloadRaw, nextRunCtx)
			if err != nil {
				return SettleResult{}, err
			}
			nextPayload, err = setQueueOperationEffectSnapshot(nextPayload, snapshot)
			if err != nil {
				return SettleResult{}, err
			}
		}
	}

	if len(plan.EnhanceResults) > 0 {
		if plan.StopReason == "" && !plan.CancelAfterSettle {
			plan.StopReason = EnhanceStopReasonAttemptsExhausted
		}
		plan.EnhanceResults[len(plan.EnhanceResults)-1].StopReason = plan.StopReason
	}

	completedAt := completedAtForQueueTiming(timing, plan.AttemptsDone)
	return SettleResult{
		Completed:         plan.AttemptsDone,
		DoneAfter:         doneAfter,
		CompletedAt:       completedAt,
		NextRunStartAt:    completedAt,
		NextRunEndsAt:     completedAt.Add(timing.NextDuration),
		CancelAfterSettle: plan.CancelAfterSettle,
		CancelReason:      enhanceCancelReasonByStopReason(plan.StopReason),
		NextPayload:       nextPayload,
		ItemCosts:         plan.ItemCosts,
		ItemRewards:       map[string]int64{},
		AttrExp:           plan.AttrExpGain,
		EnhanceResults:    plan.EnhanceResults,
	}, nil
}

func (e *enhanceExecutor) planEnhanceFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, runtime enhanceExecutorRuntime, locked LockedItemInstance, completed int, firstRunCtx enhanceRunContext, firstRunCtxOK bool, budgetStartedAt time.Time, budget FastForwardBudget) (enhanceFastForwardPlan, error) {
	plan := enhanceFastForwardPlan{
		FinalLevel:     locked.EnhanceLevel,
		ItemCosts:      map[string]int64{},
		ChargeCosts:    map[string]int64{},
		AttrExpGain:    map[string]float64{},
		EnhanceResults: make([]EnhanceAttemptResult, 0, completed),
	}
	if completed <= 0 {
		return plan, nil
	}
	successRateBonus, err := e.enhancementSuccessRateBonusTx(ctx, tx, userID)
	if err != nil {
		return enhanceFastForwardPlan{}, err
	}
	ownedByItem, err := e.lockEnhanceCostInventoryTx(ctx, tx, userID, runtime, firstRunCtx, firstRunCtxOK)
	if err != nil {
		return enhanceFastForwardPlan{}, err
	}
	currentLevel := locked.EnhanceLevel
	// Runtime and the base success bonus are fixed for this planner call. Keep
	// only immutable base contexts here; per-start effects modify a value copy.
	contextsByLevel := make(map[int]enhanceRunContext)
	for plan.AttemptsDone < completed {
		prepaid := plan.AttemptsDone == 0 && firstRunCtxOK
		var runCtx enhanceRunContext
		if prepaid {
			runCtx = normalizeEnhanceRunContext(firstRunCtx)
		} else {
			prepared, cached := contextsByLevel[currentLevel]
			if !cached {
				var reason string
				var preparedOK bool
				prepared, reason, preparedOK = e.buildEnhanceRunContext(runtime, currentLevel, successRateBonus)
				if !preparedOK {
					plan.CancelAfterSettle = true
					plan.StopReason = reason
					break
				}
				contextsByLevel[currentLevel] = prepared
			}
			if ok, reason, err := reserveEnhanceLocalCosts(ownedByItem, prepared.Costs, runtime.ProtectItemID, prepared.UsedProtect); err != nil {
				return enhanceFastForwardPlan{}, err
			} else if !ok {
				plan.CancelAfterSettle = true
				plan.StopReason = reason
				break
			}
			if err := mergeEnhanceCosts(plan.ChargeCosts, prepared.Costs); err != nil {
				return enhanceFastForwardPlan{}, err
			}
			runCtx = prepared
			// The first legacy run has no start snapshot. Only subsequent real
			// starts may acquire effects during fast-forward.
			if plan.AttemptsDone > 0 {
				timing := resolveQueueRunTiming(item, time.Time{}, time.Duration(runtime.DurationMs)*time.Millisecond)
				snapshot, err := nextEnhanceOperationEffects(item, timing, time.Time{}, plan.AttemptsDone)
				if err != nil {
					return enhanceFastForwardPlan{}, err
				}
				if err := applyEnhanceOperationEffects(&runCtx, snapshot); err != nil {
					return enhanceFastForwardPlan{}, err
				}
			}
		}
		if err := mergeEnhanceCosts(plan.ItemCosts, runCtx.Costs); err != nil {
			return enhanceFastForwardPlan{}, err
		}

		beforeLevel := currentLevel
		success := enhanceAttemptRoll(userID, item.QueueID, item.DoneCount+plan.AttemptsDone, runCtx.SuccessRate)
		if success {
			currentLevel = clampEnhanceLevel(beforeLevel+1, runtime.MaxLevel)
		} else if runCtx.UsedProtect {
			currentLevel = clampEnhanceLevel(beforeLevel-1, runtime.MaxLevel)
		} else {
			currentLevel = 0
		}
		attemptNo := item.DoneCount + plan.AttemptsDone + 1
		plan.EnhanceResults = append(plan.EnhanceResults, EnhanceAttemptResult{
			QueueID:             item.QueueID,
			ActionID:            item.ActionID,
			AttemptNo:           attemptNo,
			EquipmentInstanceID: runtime.EquipmentInstanceID,
			TemplateItemID:      locked.TemplateItem,
			BeforeLevel:         beforeLevel,
			AfterLevel:          currentLevel,
			Success:             success,
			UsedProtect:         runCtx.UsedProtect,
			BaseSuccessRate:     runCtx.BaseSuccessRate,
			SuccessRateBonus:    runCtx.SuccessRateBonus,
			SuccessRate:         runCtx.SuccessRate,
			Costs:               cloneCostMap(runCtx.Costs),
			DeltaAttrs:          cloneAttrMap(runCtx.DeltaAttrs),
		})
		if runtime.EnhanceExp > 0 {
			plan.AttrExpGain["enhance"] = roundFloat(plan.AttrExpGain["enhance"]+runtime.EnhanceExp, 4)
		}
		plan.AttemptsDone++
		plan.FinalLevel = currentLevel
		if success {
			if _, exists := e.store.enhanceCache.AffixMilestonePool(runtime.TemplateID, currentLevel); exists {
				plan.MilestoneAttemptNo = attemptNo
				break
			}
		}
		if currentLevel >= runtime.TargetEnhanceLevel || currentLevel >= runtime.MaxLevel {
			plan.CancelAfterSettle = true
			if currentLevel >= runtime.MaxLevel {
				plan.StopReason = EnhanceStopReasonMaxLevelReached
			} else {
				plan.StopReason = EnhanceStopReasonTargetReached
			}
			break
		}
		if budget.MaxExec > 0 && time.Since(budgetStartedAt) >= budget.MaxExec {
			break
		}
	}
	return plan, nil
}

func (e *enhanceExecutor) buildEnhanceRunContext(runtime enhanceExecutorRuntime, currentLevel int, successRateBonus float64) (enhanceRunContext, string, bool) {
	if currentLevel >= runtime.TargetEnhanceLevel {
		return enhanceRunContext{}, EnhanceStopReasonTargetReached, false
	}
	if currentLevel >= runtime.MaxLevel {
		return enhanceRunContext{}, EnhanceStopReasonMaxLevelReached, false
	}
	preview, exists := e.store.enhanceCache.NextPreview(runtime.TemplateID, currentLevel)
	if !exists || preview.NextLevel > runtime.MaxLevel {
		return enhanceRunContext{}, EnhanceStopReasonTargetInvalid, false
	}
	usedProtect := runtime.UseProtect && runtime.ProtectStartLevel > 0 && currentLevel >= runtime.ProtectStartLevel && strings.TrimSpace(runtime.ProtectItemID) != ""
	baseSuccessRate, successRateBonus, finalSuccessRate := splitEnhanceSuccessRates(preview.SuccessRate, successRateBonus)
	return enhanceRunContext{
		BeforeLevel:      currentLevel,
		BaseSuccessRate:  baseSuccessRate,
		SuccessRateBonus: successRateBonus,
		SuccessRate:      finalSuccessRate,
		UsedProtect:      usedProtect,
		Costs:            attemptCosts(preview.Costs, runtime.ProtectItemID, usedProtect),
		DeltaAttrs:       cloneAttrMap(preview.DeltaAttrs),
	}, "", true
}

func (e *enhanceExecutor) lockEnhanceCostInventoryTx(ctx context.Context, tx pgx.Tx, userID int64, runtime enhanceExecutorRuntime, firstRunCtx enhanceRunContext, firstRunCtxOK bool) (map[string]int64, error) {
	itemIDSet := map[string]struct{}{}
	if template, ok := e.store.enhanceCache.Template(runtime.TemplateID); ok {
		for level := 0; level <= template.MaxLevel; level++ {
			preview, exists := template.NextPreviewByLevel[level]
			if !exists {
				continue
			}
			for itemID, qty := range preview.Costs {
				if strings.TrimSpace(itemID) != "" && qty > 0 {
					itemIDSet[strings.TrimSpace(itemID)] = struct{}{}
				}
			}
		}
	}
	if firstRunCtxOK {
		for itemID, qty := range firstRunCtx.Costs {
			if strings.TrimSpace(itemID) != "" && qty > 0 {
				itemIDSet[strings.TrimSpace(itemID)] = struct{}{}
			}
		}
	}
	if protectID := strings.TrimSpace(runtime.ProtectItemID); runtime.UseProtect && protectID != "" {
		itemIDSet[protectID] = struct{}{}
	}
	itemIDs := make([]string, 0, len(itemIDSet))
	for itemID := range itemIDSet {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Strings(itemIDs)
	return e.store.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, itemIDs)
}

func reserveEnhanceLocalCosts(ownedByItem map[string]int64, costs map[string]int64, protectItemID string, usedProtect bool) (bool, string, error) {
	entries, err := buildActionCostEntries(costs, 1)
	if err != nil {
		return false, "", err
	}
	protectItemID = strings.TrimSpace(protectItemID)
	for _, entry := range entries {
		if ownedByItem[entry.ItemID] >= entry.Quantity {
			continue
		}
		if usedProtect && protectItemID != "" && strings.TrimSpace(entry.ItemID) == protectItemID {
			return false, EnhanceStopReasonProtectItemInsufficient, nil
		}
		return false, EnhanceStopReasonCostInsufficient, nil
	}
	for _, entry := range entries {
		ownedByItem[entry.ItemID] -= entry.Quantity
	}
	return true, "", nil
}

func (e *enhanceExecutor) consumeEnhanceAggregatedCostsTx(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64) error {
	entries, err := buildActionCostEntries(costs, 1)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	ownedByItem, err := e.store.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(entries))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if ownedByItem[entry.ItemID] < entry.Quantity {
			return ErrInsufficientItem
		}
	}
	for _, entry := range entries {
		if err := e.store.ConsumeByTemplateTx(ctx, tx, userID, entry.ItemID, entry.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func (e *enhanceExecutor) ClassifyStartError(err error) (bool, string) {
	switch {
	case errors.Is(err, ErrInsufficientItem):
		return true, ActionQueueCancelReasonCostInsufficient
	case errors.Is(err, ErrStatusRequireNotMet):
		return true, ActionQueueCancelReasonStatusRequirement
	case errors.Is(err, ErrBuildingRequireNotMet):
		return true, ActionQueueCancelReasonBuildingRequirement
	case errors.Is(err, ErrEnhanceTargetInvalid), errors.Is(err, ErrEnhanceNotInInventory), errors.Is(err, ErrEnhanceTemplateInvalid):
		return true, ActionQueueCancelReasonTargetInvalid
	default:
		return false, ""
	}
}

func (e *enhanceExecutor) prepareEnhanceRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, runtime enhanceExecutorRuntime, currentLevel int) (enhanceRunContext, string, bool, error) {
	if currentLevel >= runtime.TargetEnhanceLevel {
		return enhanceRunContext{}, EnhanceStopReasonTargetReached, false, nil
	}
	if currentLevel >= runtime.MaxLevel {
		return enhanceRunContext{}, EnhanceStopReasonMaxLevelReached, false, nil
	}
	preview, exists := e.store.enhanceCache.NextPreview(runtime.TemplateID, currentLevel)
	if !exists || preview.NextLevel > runtime.MaxLevel {
		return enhanceRunContext{}, EnhanceStopReasonTargetInvalid, false, nil
	}
	usedProtect := runtime.UseProtect && runtime.ProtectStartLevel > 0 && currentLevel >= runtime.ProtectStartLevel && strings.TrimSpace(runtime.ProtectItemID) != ""
	canConsume, insufficientProtect := e.canConsumeEnhanceAttempt(ctx, tx, userID, preview.Costs, runtime.ProtectItemID, usedProtect)
	if !canConsume {
		if insufficientProtect {
			return enhanceRunContext{}, EnhanceStopReasonProtectItemInsufficient, false, nil
		}
		return enhanceRunContext{}, EnhanceStopReasonCostInsufficient, false, nil
	}
	costs := attemptCosts(preview.Costs, runtime.ProtectItemID, usedProtect)
	for itemID, qty := range costs {
		if qty <= 0 {
			continue
		}
		if err := e.store.ConsumeByTemplateTx(ctx, tx, userID, itemID, qty); err != nil {
			return enhanceRunContext{}, "", false, err
		}
	}
	successRateBonus, err := e.enhancementSuccessRateBonusTx(ctx, tx, userID)
	if err != nil {
		return enhanceRunContext{}, "", false, err
	}
	baseSuccessRate, successRateBonus, finalSuccessRate := splitEnhanceSuccessRates(preview.SuccessRate, successRateBonus)
	return enhanceRunContext{
		BeforeLevel:      currentLevel,
		BaseSuccessRate:  baseSuccessRate,
		SuccessRateBonus: successRateBonus,
		SuccessRate:      finalSuccessRate,
		UsedProtect:      usedProtect,
		Costs:            costs,
		DeltaAttrs:       cloneAttrMap(preview.DeltaAttrs),
	}, "", true, nil
}

func (e *enhanceExecutor) applyEnhanceMilestoneAffixTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, runtime enhanceExecutorRuntime, locked *LockedItemInstance, attemptNo int) (string, error) {
	pool, exists := e.store.enhanceCache.AffixMilestonePool(runtime.TemplateID, locked.EnhanceLevel)
	if !exists {
		return "", nil
	}
	levelKey := strconv.Itoa(locked.EnhanceLevel)
	claims := decodeMilestoneAffixClaims(locked.MilestoneAffixesRaw)
	if claims[levelKey] {
		return "", nil
	}
	template := e.store.gameData.EquipmentTemplateByID(runtime.TemplateID)
	if template == nil {
		return EnhanceStopReasonTargetInvalid, nil
	}
	equippedAffixes := decodeAffixSlots(locked.AffixesRaw)
	occupiedSlots := collectOccupiedAffixSlotIndexes(equippedAffixes, template.AffixSlotCount)
	slotIndex := firstEmptyAffixSlotIndex(template.AffixSlotCount, occupiedSlots)
	if slotIndex < 0 {
		return EnhanceStopReasonMilestoneAffixUnavailable, nil
	}
	rng := rand.New(rand.NewSource(enhanceMilestoneSeed(userID, item.QueueID, attemptNo, locked.EnhanceLevel)))
	slot, err := e.store.rollEnhanceMilestoneAffix(pool, runtime.TemplateID, slotIndex, rng)
	if err != nil {
		if errors.Is(err, ErrAffixInvalidCarrier) || errors.Is(err, ErrAffixRarityNotAllowed) || errors.Is(err, ErrAffixNotAllowedByEquipment) || errors.Is(err, ErrAffixNoEmptySlot) || errors.Is(err, ErrAffixEquipmentMaxEffectiveExceeded) {
			return EnhanceStopReasonMilestoneAffixUnavailable, nil
		}
		return "", err
	}
	def := e.store.gameData.AffixDefinitionByID(strings.TrimSpace(slot.AffixID))
	if WouldExceedEquipmentAffixLimit(equippedAffixes, slot.AffixID, 1, def) {
		return EnhanceStopReasonMilestoneAffixUnavailable, nil
	}
	slot.SlotIndex = slotIndex
	equippedAffixes = append(equippedAffixes, slot)
	if err := e.store.SetAffixesTx(ctx, tx, locked.InstanceID, equippedAffixes); err != nil {
		return "", err
	}
	locked.AffixesRaw = encodeAffixSlots(equippedAffixes)
	claims[levelKey] = true
	claimsRaw := encodeMilestoneAffixClaims(claims)
	if err := e.store.SetMilestoneAffixClaimsRawTx(ctx, tx, locked.InstanceID, claimsRaw); err != nil {
		return "", err
	}
	locked.MilestoneAffixesRaw = claimsRaw
	return "", nil
}

func (e *enhanceExecutor) enhancementSuccessRateBonusTx(ctx context.Context, tx pgx.Tx, userID int64) (float64, error) {
	var statsRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT stats_jsonb
		FROM player_derived_stats_cache
		WHERE user_id = $1
	`, userID).Scan(&statsRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	stats := decodeNumericJSONMap(statsRaw)
	bonusRate := stats[enhanceSuccessRateBonusAttrKey]
	if bonusRate < 0 {
		return 0, nil
	}
	return bonusRate, nil
}

func splitEnhanceSuccessRates(baseSuccessRate float64, bonusSuccessRate float64) (float64, float64, float64) {
	base := roundEnhanceRate(math.Max(0, math.Min(1, baseSuccessRate)))
	bonus := bonusSuccessRate
	if !isFiniteNumber(bonus) || bonus < 0 {
		bonus = 0
	}
	bonus = roundEnhanceRate(bonus)
	final := roundEnhanceRate(math.Max(0, math.Min(1, base+bonus)))
	return base, bonus, final
}

func normalizeEnhanceRunContext(runCtx enhanceRunContext) enhanceRunContext {
	baseSuccessRate := runCtx.BaseSuccessRate
	successRateBonus := runCtx.SuccessRateBonus
	if baseSuccessRate == 0 && successRateBonus == 0 {
		baseSuccessRate = runCtx.SuccessRate
	}
	baseSuccessRate, successRateBonus, finalSuccessRate := splitEnhanceSuccessRates(baseSuccessRate, successRateBonus)
	runCtx.BaseSuccessRate = baseSuccessRate
	runCtx.SuccessRateBonus = successRateBonus
	runCtx.SuccessRate = finalSuccessRate
	return runCtx
}

func (e *enhanceExecutor) refundEnhanceRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, runCtx enhanceRunContext) error {
	for itemID, qty := range runCtx.Costs {
		if qty <= 0 {
			continue
		}
		if err := e.store.GrantByTemplateTx(ctx, tx, userID, itemID, qty); err != nil {
			return err
		}
	}
	return nil
}

func (e *enhanceExecutor) canConsumeEnhanceAttempt(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64, protectItemID string, usedProtect bool) (bool, bool) {
	allCosts := attemptCosts(costs, protectItemID, usedProtect)
	entries, err := buildActionCostEntries(allCosts, 1)
	if err != nil {
		return false, false
	}
	if len(entries) == 0 {
		return true, false
	}
	ownedByItem, err := e.store.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(entries))
	if err != nil {
		return false, false
	}
	normalizedProtectID := strings.TrimSpace(protectItemID)
	insufficientProtect := false
	for _, entry := range entries {
		if ownedByItem[entry.ItemID] >= entry.Quantity {
			continue
		}
		if usedProtect && normalizedProtectID != "" && strings.TrimSpace(entry.ItemID) == normalizedProtectID {
			insufficientProtect = true
		}
		return false, insufficientProtect
	}
	return true, false
}

func parseEnhanceExecutorRuntime(payloadRaw []byte) (enhanceExecutorRuntime, bool) {
	if len(payloadRaw) == 0 {
		return enhanceExecutorRuntime{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return enhanceExecutorRuntime{}, false
	}
	if envelope.ExecutorID != "equipment_enhance" {
		return enhanceExecutorRuntime{}, false
	}
	var runtime enhanceExecutorRuntime
	if err := json.Unmarshal(envelope.Runtime, &runtime); err != nil {
		return enhanceExecutorRuntime{}, false
	}
	if strings.TrimSpace(runtime.TemplateID) == "" || runtime.EquipmentInstanceID <= 0 {
		return enhanceExecutorRuntime{}, false
	}
	return runtime, true
}

func parseEnhanceRunContext(payloadRaw []byte) (enhanceRunContext, bool) {
	if len(payloadRaw) == 0 {
		return enhanceRunContext{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return enhanceRunContext{}, false
	}
	if envelope.ExecutorID != "equipment_enhance" || len(envelope.RunContext) == 0 {
		return enhanceRunContext{}, false
	}
	var runCtx enhanceRunContext
	if err := json.Unmarshal(envelope.RunContext, &runCtx); err != nil {
		return enhanceRunContext{}, false
	}
	return runCtx, true
}

func setEnhanceRunContext(payloadRaw []byte, runCtx enhanceRunContext) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	runCtxRaw, err := json.Marshal(runCtx)
	if err != nil {
		return nil, err
	}
	envelope.RunContext = runCtxRaw
	nextPayload, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return nextPayload, nil
}

func enhanceCancelReasonByStopReason(stopReason string) string {
	switch strings.TrimSpace(stopReason) {
	case EnhanceStopReasonTargetInvalid:
		return ActionQueueCancelReasonTargetInvalid
	case EnhanceStopReasonProtectItemInsufficient:
		return ActionQueueCancelReasonProtectInsuff
	case EnhanceStopReasonCostInsufficient:
		return ActionQueueCancelReasonCostInsufficient
	case EnhanceStopReasonMilestoneAffixUnavailable:
		return ActionQueueCancelReasonTargetInvalid
	default:
		return ActionQueueCancelReasonCostInsufficient
	}
}

func firstEmptyAffixSlotIndex(slotCount int64, occupied map[int]struct{}) int {
	if slotCount <= 0 {
		return -1
	}
	for idx := 0; idx < int(slotCount); idx++ {
		if _, exists := occupied[idx]; !exists {
			return idx
		}
	}
	return -1
}

func enhanceMilestoneSeed(userID int64, queueID int64, attemptNo int, level int) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("enhance-milestone-affix"))
	writeI64(h, userID)
	writeI64(h, queueID)
	writeI64(h, int64(attemptNo))
	writeI64(h, int64(level))
	return int64(h.Sum64())
}

func clampEnhanceLevel(level int, maxLevel int) int {
	if maxLevel < 0 {
		maxLevel = 0
	}
	if level < 0 {
		return 0
	}
	if level > maxLevel {
		return maxLevel
	}
	return level
}

func enhanceAttemptRoll(userID int64, queueID int64, doneCount int, successRate float64) bool {
	if successRate <= 0 {
		return false
	}
	if successRate >= 1 {
		return true
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte("enhance-attempt"))
	writeI64(h, userID)
	writeI64(h, queueID)
	writeI64(h, int64(doneCount))
	seed := h.Sum64()
	r := float64(seed%1_000_000) / 1_000_000
	return r < math.Max(0, math.Min(1, successRate))
}

func attemptCosts(costs map[string]int64, protectItemID string, usedProtect bool) map[string]int64 {
	out := make(map[string]int64, len(costs)+1)
	for itemID, qty := range costs {
		if qty <= 0 {
			continue
		}
		out[itemID] = qty
	}
	if usedProtect && strings.TrimSpace(protectItemID) != "" {
		out[protectItemID] += 1
	}
	return out
}

func mergeEnhanceCosts(dst map[string]int64, src map[string]int64) error {
	for itemID, qty := range src {
		if err := addEnhanceCost(dst, itemID, qty); err != nil {
			return err
		}
	}
	return nil
}

func addEnhanceCost(dst map[string]int64, itemID string, qty int64) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" || qty <= 0 {
		return nil
	}
	if dst[itemID] > math.MaxInt64-qty {
		return ErrInventoryQuantityOverflow
	}
	dst[itemID] += qty
	return nil
}

func containsString(values []string, target string) bool {
	normalizedTarget := strings.TrimSpace(target)
	if normalizedTarget == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == normalizedTarget {
			return true
		}
	}
	return false
}
