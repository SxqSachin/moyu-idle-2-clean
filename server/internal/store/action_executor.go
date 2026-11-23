package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/operationeffect"

	"github.com/jackc/pgx/v5"
)

type FastForwardBudget struct {
	MaxIterations int
	MaxExec       time.Duration
	StartedAt     time.Time
}

type QueueOfflineProjectionPolicy uint8

const (
	// Immediate is the safe default for executors with domain state that cannot
	// be reconstructed from the shared terminal action snapshot alone.
	QueueOfflineProjectionImmediate QueueOfflineProjectionPolicy = iota
	QueueOfflineProjectionTerminal
)

type QueueRuntimeItem struct {
	QueueID              int64
	ActorType            string
	ActorID              int64
	ActionID             string
	Spec                 ActionSpec
	TotalCount           int
	DoneCount            int
	PayloadRaw           []byte
	RunStartAt           *time.Time
	RunEndsAt            *time.Time
	OtherRunEnd          *time.Time
	AuditPhase           string
	WorkLoadoutRuntime   *WorkLoadoutRuntimeResolution
	ActivatedWorkLoadout *ActivatedWorkLoadout
	EquipmentChanged     bool
	EquipmentKittyIDs    []int64
	EffectSession        *operationeffect.Session
	OperationEffects     *operationeffect.Snapshot
	alchemyReads         *offlineAlchemyReads
	alchemyRandomSeed    func(ordinal int) int64
	// PreserveScheduledTimeline is true for offline/batched simulation, where
	// the next operation starts at the prior logical completion instant. Online
	// worker settlement starts the next operation at the actual settlement time.
	PreserveScheduledTimeline bool
}

type SettleResult struct {
	// A segment replacing single completions retains their floating-point
	// addition order in cumulative and per-actor summaries.
	exactSingleAttrExp map[string]float64
	// Progressed is reserved for state-machine progress that commits work without
	// completing a player-visible action, such as one or more fishing phases.
	Progressed        bool
	Completed         int
	BudgetIterations  int `json:"-"` // Internal work units; not player-visible completions.
	DoneAfter         int
	CompletedAt       time.Time
	NextRunStartAt    time.Time
	NextRunEndsAt     time.Time
	CancelAfterSettle bool
	CancelReason      string
	NextPayload       json.RawMessage
	ItemCosts         map[string]int64
	ItemRewards       map[string]int64
	AttrExp           map[string]float64
	OwnerAttrExp      map[string]float64
	FishingCatches    map[string]FishingCatchTally
	EnhanceResults    []EnhanceAttemptResult
}

type QueueActionExecutor interface {
	ActionID() string
	OfflineProjectionPolicy() QueueOfflineProjectionPolicy
	ValidateAndNormalizeArgs(raw json.RawMessage) (any, error)
	FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, spec ActionSpec, args any, now time.Time) (json.RawMessage, error)
	StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (json.RawMessage, error)
	CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error
	SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error)
	// SettleFastForwardTx must be semantically equivalent to repeated normal
	// settlement, honor the shared budget, and persist a resumable checkpoint.
	// A successful result must complete work, request cancel, or set Progressed
	// together with a positive BudgetIterations value.
	SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error)
	ClassifyStartError(err error) (cancelQueue bool, cancelReason string)
}

type queueDurationRefreshMode uint8

const (
	queueDurationRefreshPending queueDurationRefreshMode = 1 << iota
	queueDurationRefreshRunning
)

func (mode queueDurationRefreshMode) allows(status string) bool {
	switch strings.TrimSpace(status) {
	case "pending":
		return mode&queueDurationRefreshPending != 0
	case "running":
		return mode&queueDurationRefreshRunning != 0
	default:
		return false
	}
}

// queueDurationRefresher is an opt-in executor capability. Executors with
// state-machine runtimes simply do not implement it, so queue orchestration
// cannot accidentally replace their runtime with a generic duration payload.
type queueDurationRefresher interface {
	durationRefreshMode() queueDurationRefreshMode
	refreshQueueDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (json.RawMessage, bool, error)
}

func queueDurationRefresherForStatus(executor QueueActionExecutor, status string) (queueDurationRefresher, bool) {
	refresher, ok := executor.(queueDurationRefresher)
	if !ok || !refresher.durationRefreshMode().allows(status) {
		return nil, false
	}
	return refresher, true
}

type queueTotalCountProvider interface {
	QueueTotalCount() int
}

type queueSingleRunArgs interface {
	RequiresSingleQueueRun() bool
}

type queueRuntimeEnvelope struct {
	ActionID   string          `json:"actionId"`
	ExecutorID string          `json:"executorId"`
	Runtime    json.RawMessage `json:"runtime"`
	RunContext json.RawMessage `json:"runContext,omitempty"`
	// Orchestration metadata carried by every executor checkpoint; it is not
	// part of an individual action's gameplay runtime.
	DeferredWorkLoadoutID string `json:"deferredWorkLoadoutId,omitempty"`
	// OperationEffects freezes the exact rules selected at this operation's
	// start. A missing field is legacy-empty and must never be resolved later.
	OperationEffects   *operationeffect.Snapshot        `json:"operationEffects,omitempty"`
	PassiveEffects     *operationeffect.PassiveSnapshot `json:"passiveEffects,omitempty"`
	OrdinarySettlement *ordinarySettlementState         `json:"ordinarySettlement,omitempty"`
	TimingEffects      *operationeffect.PassiveSnapshot `json:"timingEffects,omitempty"`
}

type genericExecutorRuntime struct {
	BaseDurationMs      int64              `json:"baseDurationMs,omitempty"`
	DurationMs          int64              `json:"durationMs"`
	ExtraAttrExp        map[string]float64 `json:"extraAttrExp,omitempty"`
	UseInstanceBindings bool               `json:"useInstanceBindings,omitempty"`
}

type genericExecutor struct {
	store *Store
}

func newGenericExecutor(store *Store) *genericExecutor {
	return &genericExecutor{store: store}
}

func (e *genericExecutor) ActionID() string {
	return "generic"
}

func (e *genericExecutor) OfflineProjectionPolicy() QueueOfflineProjectionPolicy {
	return QueueOfflineProjectionTerminal
}

func (e *genericExecutor) durationRefreshMode() queueDurationRefreshMode {
	return queueDurationRefreshPending | queueDurationRefreshRunning
}

func (e *genericExecutor) refreshQueueDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (json.RawMessage, bool, error) {
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return nil, false, stateErr
	}
	if state != nil {
		item.Spec = state.Spec
	}
	runtime, ok := parseGenericExecutorRuntimeEnvelope(item.PayloadRaw)
	if !ok {
		return nil, false, errors.New("invalid generic runtime payload")
	}
	baseDuration := time.Duration(item.Spec.DurationS) * time.Second
	if runtime.BaseDurationMs > 0 {
		baseDuration = time.Duration(runtime.BaseDurationMs) * time.Millisecond
	}
	resolvedDuration, extraAttrExp, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, baseDuration, time.Time{})
	if err != nil {
		return nil, false, err
	}
	if snapshot, ok := queueOperationEffectSnapshot(item.PayloadRaw); ok {
		resolvedDuration, err = workMealDuration(resolvedDuration, snapshot)
		if err != nil {
			return nil, false, err
		}
	}
	nextPayload, err := setQueueRuntimePayload(item.PayloadRaw, genericExecutorRuntime{
		BaseDurationMs:      int64(baseDuration / time.Millisecond),
		DurationMs:          int64(resolvedDuration / time.Millisecond),
		ExtraAttrExp:        extraAttrExp,
		UseInstanceBindings: runtime.UseInstanceBindings,
	})
	if err != nil {
		return nil, false, err
	}
	return nextPayload, string(nextPayload) != string(item.PayloadRaw), nil
}

func (e *genericExecutor) ValidateAndNormalizeArgs(raw json.RawMessage) (any, error) {
	return parseGenericExecutorArgs(raw)
}

func (e *genericExecutor) FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, spec ActionSpec, rawArgs any, _ time.Time) (json.RawMessage, error) {
	args, _ := rawArgs.(genericExecutorArgs)
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	frozenDuration, frozenExtraExp, err := e.store.resolveDurationAndBonusTx(ctx, tx, userID, actorType, actorID, spec, time.Duration(spec.DurationS)*time.Second)
	if err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(genericExecutorRuntime{
		BaseDurationMs:      int64(time.Duration(spec.DurationS) * time.Second / time.Millisecond),
		DurationMs:          int64(frozenDuration / time.Millisecond),
		ExtraAttrExp:        frozenExtraExp,
		UseInstanceBindings: len(args.MaterialInstanceBindings) > 0,
	})
	if err != nil {
		return nil, err
	}
	runCtx := genericExecutorRunContext{
		UseInstanceBindings:      len(args.MaterialInstanceBindings) > 0,
		MaterialInstanceBindings: args.MaterialInstanceBindings,
		InheritFromInstanceID:    args.InheritFromInstanceID,
	}
	runCtxRaw, err := json.Marshal(runCtx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   spec.ID,
		ExecutorID: e.ActionID(),
		Runtime:    runtimeRaw,
		RunContext: runCtxRaw,
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (e *genericExecutor) StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, at time.Time) (json.RawMessage, error) {
	state, err := ordinaryState(item.PayloadRaw)
	if err != nil {
		return nil, err
	}
	if state != nil {
		item.Spec = state.Spec
	}
	runCtx, ok := parseGenericRunContext(item.PayloadRaw)
	if !ok {
		runCtx = genericExecutorRunContext{}
	}
	args := genericExecutorArgs{
		MaterialInstanceBindings: runCtx.MaterialInstanceBindings,
		InheritFromInstanceID:    runCtx.InheritFromInstanceID,
	}
	nextRunCtx, err := e.applyActionCostsWithBindingsTx(ctx, tx, userID, item.Spec, args)
	if err != nil {
		return nil, err
	}
	nextRunCtx.MaterialInstanceBindings = runCtx.MaterialInstanceBindings
	if nextRunCtx.InheritFromInstanceID == 0 {
		nextRunCtx.InheritFromInstanceID = runCtx.InheritFromInstanceID
	}
	nextPayload, err := setGenericRunContext(item.PayloadRaw, nextRunCtx)
	if err != nil {
		return nil, err
	}
	runtime, _ := parseGenericExecutorRuntimeEnvelope(nextPayload)
	baseDuration := time.Duration(item.Spec.DurationS) * time.Second
	if runtime.BaseDurationMs > 0 {
		baseDuration = time.Duration(runtime.BaseDurationMs) * time.Millisecond
	}
	resolvedDuration, extraAttrExp, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, baseDuration, at)
	if err != nil {
		return nil, err
	}
	if resolvedDuration <= 0 {
		resolvedDuration = minimumActionQueueRunDuration
	}
	if item.OperationEffects != nil {
		resolvedDuration, err = workMealDuration(resolvedDuration, *item.OperationEffects)
		if err != nil {
			return nil, err
		}
	}
	nextPayload, err = setWorkQueueRuntime(item, nextPayload, genericExecutorRuntime{
		BaseDurationMs:      int64(baseDuration / time.Millisecond),
		DurationMs:          int64(resolvedDuration / time.Millisecond),
		ExtraAttrExp:        extraAttrExp,
		UseInstanceBindings: runtime.UseInstanceBindings,
	}, at)
	if err != nil {
		return nil, err
	}
	return e.store.freezeOrdinarySettlement(item, nextPayload)
}

func (e *genericExecutor) CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return stateErr
	}
	if state != nil {
		item.Spec = state.Spec
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if runCtx, ok := parseGenericRunContext(item.PayloadRaw); ok && runCtx.UseInstanceBindings {
		for itemID, qty := range runCtx.RefundTemplateCosts {
			if qty <= 0 {
				continue
			}
			if err := e.store.LedgerGrantTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
				SourceSurface: "action_queue",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type": "action_refund_binding_template",
				},
			}); err != nil {
				return err
			}
		}
		for _, row := range runCtx.RefundEnhancedCosts {
			if row.Quantity <= 0 || row.EnhanceLevel <= 0 || strings.TrimSpace(row.ItemID) == "" {
				continue
			}
			for i := int64(0); i < row.Quantity; i++ {
				instanceID, err := e.store.AddInventoryEnhancedInstanceTx(ctx, tx, userID, row.ItemID, row.EnhanceLevel)
				if err != nil {
					return err
				}
				if err := e.store.LedgerLogOnlyTx(ctx, tx, userID, row.ItemID, 1, InventoryLedgerMeta{
					SourceSurface: "action_queue",
					TargetSurface: "inventory",
					InstanceID:    &instanceID,
					Details: map[string]any{
						"type": "action_refund_binding_enhanced",
					},
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return e.store.refundActionCostsTx(ctx, tx, userID, item.Spec.Costs)
}

func (e *genericExecutor) SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return SettleResult{}, err
	}
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return SettleResult{}, stateErr
	}
	if state != nil {
		item.Spec = state.Spec
		return e.settleOrdinaryTx(ctx, tx, userID, item, now, budget, state, nil)
	}
	// An existing prepaid operation remains legacy. Only its accepted successor
	// may acquire the new rules and random stream; never batch across migration.
	if e.store.config.OrdinarySettlementEnabled && ordinaryMainType(item.Spec.MainType) && item.TotalCount > item.DoneCount+1 {
		timing := resolveQueueRunTiming(item, now, parseQueueRuntimeDuration(item.PayloadRaw))
		return e.settleEffectfulGenericTx(ctx, tx, userID, item, now, timing)
	}
	runtime, _ := parseGenericExecutorRuntimeEnvelope(item.PayloadRaw)
	duration := runtime.Duration()
	extraAttrExp := runtime.ExtraAttrExp
	runCtx, _ := parseGenericRunContext(item.PayloadRaw)
	remaining := item.TotalCount - item.DoneCount
	timing := resolveQueueRunTiming(item, now, duration)
	limit, timingChanged, err := workEfficiencyBatchLimit(item, timing, now, budget.MaxIterations)
	if err != nil {
		return SettleResult{}, err
	}
	budget.MaxIterations = limit
	effectBatch := false
	if item.TotalCount > item.DoneCount+1 && (timingChanged || e.needsEffectfulGenericSettlement(item, timing, now)) {
		if !timingChanged {
			batchLimit, batchErr := e.primaryWorkEffectBatchLimitTx(ctx, tx, userID, item, timing, now, budget.MaxIterations)
			if batchErr != nil {
				return SettleResult{}, batchErr
			}
			if batchLimit > 1 {
				budget.MaxIterations = batchLimit
				effectBatch = true
			}
		}
		if !effectBatch {
			return e.settleEffectfulGenericTx(ctx, tx, userID, item, now, timing)
		}
	}
	completed, completedAt := completedRunsForQueueTiming(timing, now, remaining)
	if budget.MaxIterations > 0 && completed > budget.MaxIterations {
		completed = budget.MaxIterations
		completedAt = completedAtForQueueTiming(timing, completed)
	}
	if runCtx.UseInstanceBindings && completed > 1 {
		completed = 1
		completedAt = completedAtForQueueTiming(timing, completed)
	}
	if completed <= 0 {
		return SettleResult{}, nil
	}
	doneAfter := item.DoneCount + completed
	cancelAfterSettle := false
	startsToCharge := int64(0)
	requiredStarts := int64(computeAdditionalStarts(completed, doneAfter, item.TotalCount))
	if runCtx.UseInstanceBindings {
		requiredStarts = 0
	}
	if requiredStarts > 0 {
		affordableStarts, affordableErr := e.store.maxAffordableActionStartsTx(ctx, tx, userID, item.Spec.Costs, requiredStarts)
		if affordableErr != nil {
			return SettleResult{}, affordableErr
		}
		completed, cancelAfterSettle, startsToCharge = adjustCompletedForAffordableStarts(completed, item.DoneCount, item.TotalCount, affordableStarts)
		if completed <= 0 {
			return SettleResult{}, nil
		}
		doneAfter = item.DoneCount + completed
		completedAt = completedAtForQueueTiming(timing, completed)
	}
	if !cancelAfterSettle {
		startsToCharge = int64(computeAdditionalStarts(completed, doneAfter, item.TotalCount))
	}
	effectSnapshots, nextPayload, err := prepareGenericOperationEffectSnapshots(item, timing, now, completed, int(startsToCharge), doneAfter, cancelAfterSettle)
	if err != nil {
		return SettleResult{}, err
	}
	phase := item.AuditPhase
	if phase == "" {
		phase = "online_settle"
	}
	itemCosts := map[string]int64{}
	if startsToCharge > 0 {
		if err := e.store.applyActionCostsBatchTx(ctx, tx, userID, item.Spec.Costs, startsToCharge, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryActionQueue,
			Direction:     InventoryAuditDirectionConsume,
			SubjectType:   "queue",
			SubjectID:     strconv.FormatInt(item.QueueID, 10),
			SourceSurface: "inventory",
			TargetSurface: "action_queue",
			Details: map[string]any{
				"type":     "action_cost_batch",
				"queueId":  item.QueueID,
				"actionId": item.ActionID,
				"count":    startsToCharge,
				"phase":    phase,
			},
		}); err != nil {
			return SettleResult{}, err
		}
		for itemID, qty := range item.Spec.Costs {
			if qty <= 0 {
				continue
			}
			itemCosts[itemID] = qty * startsToCharge
		}
	}
	itemRewards := map[string]int64{}
	var rewardErr error
	if runCtx.UseInstanceBindings {
		itemRewards, rewardErr = e.store.applyActionRewardsBatchWithInheritanceTx(ctx, tx, userID, item.QueueID, item.DoneCount, completed, item.Spec.ItemReward, runCtx.InheritedEnhanceLevel, phase)
	} else {
		itemRewards, rewardErr = e.store.applyActionRewardsBatchTx(ctx, tx, userID, item.QueueID, item.DoneCount, completed, item.Spec.ItemReward, phase)
	}
	if rewardErr != nil {
		return SettleResult{}, rewardErr
	}
	effectRewards, err := e.applyLifeActionOperationEffectsTx(ctx, tx, userID, item, effectSnapshots, phase)
	if err != nil {
		return SettleResult{}, err
	}
	for itemID, quantity := range effectRewards {
		itemRewards[itemID] += quantity
	}
	attrGain := scaleAttrExp(mergeAttrExp(item.Spec.StatusExpReward, extraAttrExp), int64(completed))
	mealExp, err := workMealExperience(item.Spec, effectSnapshots)
	if err != nil {
		return SettleResult{}, err
	}
	attrGain = mergeAttrExp(attrGain, mealExp)
	ownerExp, err := shareActionExperience(item, mergeAttrExp(item.Spec.StatusExpReward, extraAttrExp), timing, now, completed)
	if err != nil {
		return SettleResult{}, err
	}
	// Food rules already use exact per-start settlement, so their additional
	// source experience is shared from the same frozen operation.
	if item.ActorType == ActionActorTypeKitty && len(mealExp) > 0 {
		snapshot, err := queuePassiveSnapshot(item.PayloadRaw)
		if err != nil {
			return SettleResult{}, err
		}
		extra, err := item.EffectSession.Passive().Share(snapshot, mealExp)
		if err != nil {
			return SettleResult{}, err
		}
		ownerExp = mergeAttrExp(ownerExp, extra)
	}
	if item.ActorType == ActionActorTypeKitty {
		attrGain = scaleAttrExpByMultiplier(attrGain, e.store.config.KittyStatusExpMultiplier)
	}
	persistedGain := mergeAttrExp(attrGain, ownerExp)
	if effectBatch {
		persistedGain, err = primaryWorkBatchRoundedExperience(item.Spec, effectSnapshots)
		if err != nil {
			return SettleResult{}, err
		}
	}
	if err := e.store.applyAttributedExperienceTx(ctx, tx, userID, persistedGain, ownerExp, item.EffectSession); err != nil {
		return SettleResult{}, err
	}
	if startsToCharge > 0 && doneAfter < item.TotalCount && !cancelAfterSettle {
		nextPayload, err = freezeQueuePassive(item, nextPayload, operationEffectSubsequentStartAt(item, timing, now, completed), operationeffect.DomainLifeAction)
		if err != nil {
			return SettleResult{}, err
		}
	}
	return SettleResult{
		OwnerAttrExp:      ownerExp,
		Completed:         completed,
		DoneAfter:         doneAfter,
		CompletedAt:       completedAt,
		NextRunStartAt:    completedAt,
		NextRunEndsAt:     completedAt.Add(timing.NextDuration),
		CancelAfterSettle: cancelAfterSettle,
		CancelReason:      ActionQueueCancelReasonCostInsufficient,
		NextPayload:       nextPayload,
		ItemCosts:         itemCosts,
		ItemRewards:       itemRewards,
		AttrExp:           attrGain,
	}, nil
}

func (e *genericExecutor) SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error) {
	return e.SettleFastForwardTx(ctx, tx, userID, item, now, FastForwardBudget{MaxIterations: 1})
}

func (e *genericExecutor) ClassifyStartError(err error) (bool, string) {
	if errors.Is(err, ErrInsufficientItem) {
		return true, ActionQueueCancelReasonCostInsufficient
	}
	if errors.Is(err, ErrStatusRequireNotMet) {
		return true, ActionQueueCancelReasonStatusRequirement
	}
	if errors.Is(err, ErrBuildingRequireNotMet) {
		return true, ActionQueueCancelReasonBuildingRequirement
	}
	if errors.Is(err, ErrActionMaterialBindingInvalid) || errors.Is(err, ErrActionMaterialAffixForbidden) {
		return true, ActionQueueCancelReasonTargetInvalid
	}
	return false, ""
}

func (s *Store) executorForAction(actionID string) QueueActionExecutor {
	if recipe := s.gameData.FishFoodRecipeByActionID(actionID); recipe != nil {
		return newFishFoodExecutor(s, *recipe)
	}
	switch strings.TrimSpace(actionID) {
	case "equipment_enhance":
		return newEnhanceExecutor(s)
	case "building_upgrade":
		return newBuildingExecutor(s)
	case "alchemy.convert", "alchemy.bench", "alchemy.transmute", "alchemy.smelt", "alchemy.inherit":
		return newAlchemyExecutor(s, strings.TrimSpace(actionID))
	case FishingTripActionID:
		return newFishingExecutor(s)
	}
	return newGenericExecutor(s)
}

type parsedGenericExecutorRuntime struct {
	BaseDurationMs      int64
	DurationMs          int64
	ExtraAttrExp        map[string]float64
	UseInstanceBindings bool
}

func (r parsedGenericExecutorRuntime) Duration() time.Duration {
	if r.DurationMs <= 0 {
		return 0
	}
	return normalizeActionQueueRunDuration(time.Duration(r.DurationMs) * time.Millisecond)
}

func parseGenericExecutorRuntimeEnvelope(payloadRaw []byte) (parsedGenericExecutorRuntime, bool) {
	if len(payloadRaw) == 0 {
		return parsedGenericExecutorRuntime{ExtraAttrExp: map[string]float64{}}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return parsedGenericExecutorRuntime{ExtraAttrExp: map[string]float64{}}, false
	}
	if envelope.ExecutorID != "generic" {
		return parsedGenericExecutorRuntime{ExtraAttrExp: map[string]float64{}}, false
	}
	var runtime genericExecutorRuntime
	if err := json.Unmarshal(envelope.Runtime, &runtime); err != nil {
		return parsedGenericExecutorRuntime{ExtraAttrExp: map[string]float64{}}, false
	}
	return parsedGenericExecutorRuntime{
		BaseDurationMs:      runtime.BaseDurationMs,
		DurationMs:          runtime.DurationMs,
		ExtraAttrExp:        runtime.ExtraAttrExp,
		UseInstanceBindings: runtime.UseInstanceBindings,
	}, true
}

func parseQueueRuntimeDuration(payloadRaw []byte) time.Duration {
	if len(payloadRaw) == 0 {
		return 0
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil || strings.TrimSpace(envelope.ExecutorID) == "" || len(envelope.Runtime) == 0 {
		return 0
	}
	var runtime struct {
		DurationMs int64 `json:"durationMs"`
	}
	if err := json.Unmarshal(envelope.Runtime, &runtime); err != nil || runtime.DurationMs <= 0 {
		return 0
	}
	return normalizeQueueRuntimeDuration(time.Duration(runtime.DurationMs)*time.Millisecond, payloadRaw)
}

func setQueueRuntimePayload(payloadRaw []byte, runtime any) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	envelope.Runtime = runtimeRaw
	nextPayload, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return nextPayload, nil
}

func queueRuntimeDeferredWorkLoadoutID(payloadRaw []byte) string {
	if len(payloadRaw) == 0 {
		return ""
	}
	var envelope queueRuntimeEnvelope
	if json.Unmarshal(payloadRaw, &envelope) != nil {
		return ""
	}
	return strings.TrimSpace(envelope.DeferredWorkLoadoutID)
}

func setQueueRuntimeDeferredWorkLoadoutID(payloadRaw []byte, loadoutID string) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	envelope.DeferredWorkLoadoutID = strings.TrimSpace(loadoutID)
	return json.Marshal(envelope)
}
