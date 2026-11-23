package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"

	"github.com/jackc/pgx/v5"
)

const (
	fishFoodExecutorID             = "fish_food"
	fishFoodIngredientModeAuto     = "automatic"
	fishFoodIngredientModeSpecific = "specific"
	fishFoodQualityRulesetV1       = "fish-food-quality-v1"
	fishFoodSnapshotSchemaVersion  = 1
	fishFoodCatchTemplateItemID    = "fish_catch"
	maxFishFoodSelectedInstances   = 16
)

var (
	ErrFishFoodArgsInvalid        = errors.New("fish food arguments are invalid")
	ErrFishFoodIngredientsInvalid = errors.New("fish food ingredients are invalid")
)

type fishFoodExecutorArgs struct {
	IngredientMode          string
	FishIngredientInstances map[string][]int64
}

func (args fishFoodExecutorArgs) RequiresSingleQueueRun() bool {
	return args.IngredientMode == fishFoodIngredientModeSpecific
}

type fishFoodExecutorRuntime struct {
	BaseDurationMs int64              `json:"baseDurationMs,omitempty"`
	DurationMs     int64              `json:"durationMs"`
	ExtraAttrExp   map[string]float64 `json:"extraAttrExp,omitempty"`
}

type fishFoodReservedIngredient struct {
	InstanceID int64  `json:"instanceId"`
	SpeciesID  string `json:"speciesId"`
}

type fishFoodExecutorRunContext struct {
	IngredientMode          string                       `json:"ingredientMode"`
	FishIngredientInstances map[string][]int64           `json:"fishIngredientInstances,omitempty"`
	RunReserved             bool                         `json:"runReserved,omitempty"`
	ReservedFish            []fishFoodReservedIngredient `json:"reservedFish,omitempty"`
	OutputBaseSnapshot      map[string]any               `json:"outputBaseSnapshot,omitempty"`
}

type fishFoodIngredientSnapshot struct {
	SpeciesID    string   `json:"speciesId"`
	LengthCM     float64  `json:"lengthCm"`
	WeightKG     float64  `json:"weightKg"`
	LengthScore  float64  `json:"lengthScore"`
	WeightScore  float64  `json:"weightScore"`
	TraitAverage float64  `json:"traitAverage"`
	Mutations    []string `json:"mutations,omitempty"`
	Multiplier   float64  `json:"multiplier"`
}

type fishFoodQualitySnapshot struct {
	Multiplier           float64            `json:"multiplier"`
	LengthScore          float64            `json:"lengthScore"`
	WeightScore          float64            `json:"weightScore"`
	TraitAverage         float64            `json:"traitAverage"`
	MutationCount        int                `json:"mutationCount"`
	SpecialMultiplier    float64            `json:"specialMultiplier,omitempty"`
	SpecialMutations     []string           `json:"specialMutations,omitempty"`
	ExtraCharges         int64              `json:"extraCharges,omitempty"`
	ExtraDurationSeconds int64              `json:"extraDurationSeconds,omitempty"`
	ParameterMultipliers map[string]float64 `json:"parameterMultipliers,omitempty"`
}

type fishFoodItemSnapshot struct {
	SchemaVersion         int                          `json:"schemaVersion"`
	RecipeID              string                       `json:"recipeId"`
	RecipeRulesetVersion  string                       `json:"recipeRulesetVersion"`
	QualityRulesetVersion string                       `json:"qualityRulesetVersion"`
	Quality               fishFoodQualitySnapshot      `json:"quality"`
	Ingredients           []fishFoodIngredientSnapshot `json:"ingredients"`
}

type selectedFishFoodIngredient struct {
	InstanceID int64
	OwnerID    int64
	State      string
	Quantity   int64
	ArchivedAt *time.Time
	Fish       FishInstanceData
}

type fishFoodExecutor struct {
	store  *Store
	recipe catalog.FishFoodRecipe
}

func newFishFoodExecutor(store *Store, recipe catalog.FishFoodRecipe) *fishFoodExecutor {
	return &fishFoodExecutor{store: store, recipe: recipe}
}

func (e *fishFoodExecutor) ActionID() string {
	return fishFoodExecutorID
}

func (e *fishFoodExecutor) OfflineProjectionPolicy() QueueOfflineProjectionPolicy {
	return QueueOfflineProjectionTerminal
}

func (e *fishFoodExecutor) OperationEffectDomain() operationeffect.Domain {
	return operationeffect.DomainLifeAction
}

func (e *fishFoodExecutor) OperationEffectsRequirePerStartSettlement() bool {
	return false // The versioned kernel bounds its own effect and input segments.
}

func (e *fishFoodExecutor) durationRefreshMode() queueDurationRefreshMode {
	return queueDurationRefreshPending | queueDurationRefreshRunning
}

func (e *fishFoodExecutor) refreshQueueDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (json.RawMessage, bool, error) {
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return nil, false, stateErr
	}
	if state != nil {
		item.Spec = state.Spec
	}
	runtime, ok := parseFishFoodExecutorRuntime(item.PayloadRaw)
	if !ok {
		return nil, false, ErrFishFoodArgsInvalid
	}
	baseDuration := time.Duration(item.Spec.DurationS) * time.Second
	if runtime.BaseDurationMs > 0 {
		baseDuration = time.Duration(runtime.BaseDurationMs) * time.Millisecond
	}
	resolvedDuration, extraAttrExp, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, baseDuration, time.Time{})
	if err != nil {
		return nil, false, err
	}
	nextPayload, err := setQueueRuntimePayload(item.PayloadRaw, fishFoodExecutorRuntime{
		BaseDurationMs: int64(baseDuration / time.Millisecond),
		DurationMs:     int64(resolvedDuration / time.Millisecond),
		ExtraAttrExp:   extraAttrExp,
	})
	if err != nil {
		return nil, false, err
	}
	return nextPayload, string(nextPayload) != string(item.PayloadRaw), nil
}

func (e *fishFoodExecutor) ValidateAndNormalizeArgs(raw json.RawMessage) (any, error) {
	var input struct {
		IngredientMode          string             `json:"ingredientMode"`
		FishIngredientInstances map[string][]int64 `json:"fishIngredientInstances"`
	}
	if len(raw) > 0 && string(raw) != "null" && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, ErrFishFoodArgsInvalid
		}
	}
	mode := strings.TrimSpace(input.IngredientMode)
	if mode == "" {
		mode = fishFoodIngredientModeAuto
	}
	if mode != fishFoodIngredientModeAuto && mode != fishFoodIngredientModeSpecific {
		return nil, ErrFishFoodArgsInvalid
	}
	if mode == fishFoodIngredientModeAuto {
		if len(input.FishIngredientInstances) > 0 {
			return nil, ErrFishFoodArgsInvalid
		}
		return fishFoodExecutorArgs{IngredientMode: mode, FishIngredientInstances: map[string][]int64{}}, nil
	}
	if !e.recipe.AllowSpecificIngredients || len(input.FishIngredientInstances) != len(e.recipe.FishIngredients) {
		return nil, ErrFishFoodArgsInvalid
	}
	expected := make(map[string]int, len(e.recipe.FishIngredients))
	for _, ingredient := range e.recipe.FishIngredients {
		expected[strings.TrimSpace(ingredient.SpeciesID)] = ingredient.Quantity
	}
	seenIDs := make(map[int64]struct{})
	total := 0
	normalized := make(map[string][]int64, len(input.FishIngredientInstances))
	for rawSpeciesID, ids := range input.FishIngredientInstances {
		speciesID := strings.TrimSpace(rawSpeciesID)
		quantity, ok := expected[speciesID]
		if !ok || len(ids) != quantity {
			return nil, ErrFishFoodArgsInvalid
		}
		copied := append([]int64(nil), ids...)
		for _, instanceID := range copied {
			if instanceID <= 0 {
				return nil, ErrFishFoodArgsInvalid
			}
			if _, exists := seenIDs[instanceID]; exists {
				return nil, ErrFishFoodArgsInvalid
			}
			seenIDs[instanceID] = struct{}{}
			total++
		}
		sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
		normalized[speciesID] = copied
	}
	if total <= 0 || total > maxFishFoodSelectedInstances {
		return nil, ErrFishFoodArgsInvalid
	}
	return fishFoodExecutorArgs{IngredientMode: mode, FishIngredientInstances: normalized}, nil
}

func (e *fishFoodExecutor) FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, spec ActionSpec, rawArgs any, _ time.Time) (json.RawMessage, error) {
	args, ok := rawArgs.(fishFoodExecutorArgs)
	if !ok {
		return nil, ErrFishFoodArgsInvalid
	}
	if err := e.validateRecipeContract(spec); err != nil {
		return nil, err
	}
	frozenDuration, frozenExtraExp, err := e.store.resolveDurationAndBonusTx(ctx, tx, userID, actorType, actorID, spec, time.Duration(spec.DurationS)*time.Second)
	if err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(fishFoodExecutorRuntime{
		BaseDurationMs: int64(time.Duration(spec.DurationS) * time.Second / time.Millisecond),
		DurationMs:     int64(frozenDuration / time.Millisecond),
		ExtraAttrExp:   frozenExtraExp,
	})
	if err != nil {
		return nil, err
	}
	runContextRaw, err := json.Marshal(fishFoodExecutorRunContext{
		IngredientMode:          args.IngredientMode,
		FishIngredientInstances: cloneFishFoodInstanceBindings(args.FishIngredientInstances),
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(queueRuntimeEnvelope{
		ActionID:   spec.ID,
		ExecutorID: fishFoodExecutorID,
		Runtime:    runtimeRaw,
		RunContext: runContextRaw,
	})
}

func (e *fishFoodExecutor) StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, at time.Time) (json.RawMessage, error) {
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return nil, stateErr
	}
	if state != nil {
		item.Spec = state.Spec
		if state.FishRecipe == nil {
			return nil, ErrFishFoodArgsInvalid
		}
		frozen := *e
		frozen.recipe = *state.FishRecipe
		e = &frozen
	}
	runContext, ok := parseFishFoodRunContext(item.PayloadRaw)
	if !ok {
		return nil, ErrFishFoodArgsInvalid
	}
	nextContext, err := e.reserveRunTx(ctx, tx, userID, item, runContext)
	if err != nil {
		return nil, err
	}
	nextPayload, err := setFishFoodRunContext(item.PayloadRaw, nextContext)
	if err != nil {
		return nil, err
	}
	runtime, ok := parseFishFoodExecutorRuntime(nextPayload)
	if !ok {
		return nil, ErrFishFoodArgsInvalid
	}
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
	nextPayload, err = setWorkQueueRuntime(item, nextPayload, fishFoodExecutorRuntime{
		BaseDurationMs: int64(baseDuration / time.Millisecond),
		DurationMs:     int64(resolvedDuration / time.Millisecond),
		ExtraAttrExp:   extraAttrExp,
	}, at)
	if err != nil {
		return nil, err
	}
	return e.store.freezeOrdinarySettlement(item, nextPayload)
}

func (e *fishFoodExecutor) CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return stateErr
	}
	if state != nil {
		item.Spec = state.Spec
	}
	runContext, ok := parseFishFoodRunContext(item.PayloadRaw)
	if !ok || !runContext.RunReserved {
		return nil
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	for _, reserved := range runContext.ReservedFish {
		if err := e.store.LedgerSetStateTx(ctx, tx, reserved.InstanceID, userID, "action_queue", "inventory", fishFoodCatchTemplateItemID, 1, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryActionQueue,
			Direction:     InventoryAuditDirectionRefund,
			SubjectType:   "queue",
			SubjectID:     strconv.FormatInt(item.QueueID, 10),
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":      "fish_food_fish_refund",
				"queueId":   item.QueueID,
				"actionId":  item.ActionID,
				"speciesId": reserved.SpeciesID,
			},
		}); err != nil {
			return err
		}
	}
	return e.store.refundActionCostsTx(ctx, tx, userID, item.Spec.Costs)
}

func (e *fishFoodExecutor) SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error) {
	return e.SettleFastForwardTx(ctx, tx, userID, item, now, FastForwardBudget{MaxIterations: 1})
}

func (e *fishFoodExecutor) SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return SettleResult{}, err
	}
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return SettleResult{}, stateErr
	}
	if state != nil {
		item.Spec = state.Spec
		if state.FishRecipe == nil {
			return SettleResult{}, ErrFishFoodArgsInvalid
		}
		frozen := *e
		frozen.recipe = *state.FishRecipe
		e = &frozen
		run, ok := parseFishFoodRunContext(item.PayloadRaw)
		if !ok {
			return SettleResult{}, ErrFishFoodArgsInvalid
		}
		if run.IngredientMode == fishFoodIngredientModeAuto {
			return newGenericExecutor(e.store).settleOrdinaryTx(ctx, tx, userID, item, now, budget, state, e)
		}
	}
	runtime, ok := parseFishFoodExecutorRuntime(item.PayloadRaw)
	if !ok {
		return SettleResult{}, ErrFishFoodArgsInvalid
	}
	runContext, ok := parseFishFoodRunContext(item.PayloadRaw)
	if !ok || !runContext.RunReserved || len(runContext.ReservedFish) == 0 {
		return SettleResult{}, ErrFishFoodIngredientsInvalid
	}
	timing := resolveQueueRunTiming(item, now, runtime.Duration())
	completed, completedAt := completedRunsForQueueTiming(timing, now, 1)
	if completed <= 0 {
		return SettleResult{}, nil
	}
	phase := item.AuditPhase
	if phase == "" {
		phase = "online_settle"
	}
	for _, reserved := range runContext.ReservedFish {
		locked, err := e.store.LockInstanceTx(ctx, tx, reserved.InstanceID)
		if err != nil {
			return SettleResult{}, err
		}
		if locked.UserID != userID || locked.State != "action_queue" || locked.TemplateItem != fishFoodCatchTemplateItemID || locked.Quantity != 1 {
			return SettleResult{}, ErrFishFoodIngredientsInvalid
		}
		if err := e.store.LedgerConsumeInstanceTx(ctx, tx, userID, reserved.InstanceID, fishFoodCatchTemplateItemID, 1, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryActionQueue,
			Direction:     InventoryAuditDirectionConsume,
			SubjectType:   "queue",
			SubjectID:     strconv.FormatInt(item.QueueID, 10),
			SourceSurface: "action_queue",
			TargetSurface: "fish_food",
			Details: map[string]any{
				"type":      "fish_food_fish_consume",
				"queueId":   item.QueueID,
				"actionId":  item.ActionID,
				"speciesId": reserved.SpeciesID,
				"phase":     phase,
			},
		}); err != nil {
			return SettleResult{}, err
		}
	}
	if err := e.grantOutputTx(ctx, tx, userID, item, runContext.OutputBaseSnapshot, 1, InventoryLedgerMeta{
		Category:      InventoryAuditCategoryActionQueue,
		Direction:     InventoryAuditDirectionGain,
		SubjectType:   "queue",
		SubjectID:     strconv.FormatInt(item.QueueID, 10),
		SourceSurface: "action_queue",
		TargetSurface: "inventory",
		Details: map[string]any{
			"type":     "fish_food_reward",
			"queueId":  item.QueueID,
			"actionId": item.ActionID,
			"phase":    phase,
		},
	}); err != nil {
		return SettleResult{}, err
	}

	currentSnapshot, hasSnapshot := queueOperationEffectSnapshot(item.PayloadRaw)
	if !hasSnapshot {
		currentSnapshot = operationeffect.EmptySnapshot(queueOperationEffectOperation(operationeffect.DomainLifeAction, item, timing.StartAt, item.DoneCount+1, item.PayloadRaw))
	}
	effectRewards, err := newGenericExecutor(e.store).applyLifeActionOperationEffectsWithMainOutputGrantTx(
		ctx,
		tx,
		userID,
		item,
		[]operationeffect.Snapshot{currentSnapshot},
		phase,
		func(snapshot operationeffect.Snapshot, extra lifeaction.ExtraOutput) error {
			return e.grantOutputTx(ctx, tx, userID, item, runContext.OutputBaseSnapshot, extra.Quantity, InventoryLedgerMeta{
				Category:      InventoryAuditCategoryOperationEffect,
				Direction:     InventoryAuditDirectionGain,
				SubjectType:   "operation_effect",
				SubjectID:     extra.DefinitionID,
				SourceSurface: "action_queue",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":         "operation_effect_extra_main_output",
					"queueId":      item.QueueID,
					"operationKey": snapshot.OperationKey,
					"grantId":      extra.GrantID,
					"phase":        phase,
				},
			})
		},
	)
	if err != nil {
		return SettleResult{}, err
	}
	itemRewards := map[string]int64{e.recipe.OutputItemID: 1}
	for itemID, quantity := range effectRewards {
		itemRewards[itemID] += quantity
	}
	attrGain := mergeAttrExp(item.Spec.StatusExpReward, runtime.ExtraAttrExp)
	ownerExp, err := shareActionExperience(item, attrGain, timing, now, 1)
	if err != nil {
		return SettleResult{}, err
	}
	if item.ActorType == ActionActorTypeKitty {
		attrGain = scaleAttrExpByMultiplier(attrGain, e.store.config.KittyStatusExpMultiplier)
	}
	if err := e.store.applyAttributedExperienceTx(ctx, tx, userID, mergeAttrExp(attrGain, ownerExp), ownerExp, item.EffectSession); err != nil {
		return SettleResult{}, err
	}

	doneAfter := item.DoneCount + 1
	nextPayload := json.RawMessage(item.PayloadRaw)
	nextStarted := false
	cancelAfterSettle := false
	cancelReason := ""
	itemCosts := map[string]int64{}
	if doneAfter < item.TotalCount {
		baseContext := runContext
		baseContext.RunReserved = false
		baseContext.ReservedFish = nil
		baseContext.OutputBaseSnapshot = nil
		nextContext, reserveErr := e.reserveRunWithRollbackTx(ctx, tx, userID, item, baseContext)
		if reserveErr != nil {
			if cancel, reason := e.ClassifyStartError(reserveErr); cancel {
				cancelAfterSettle = true
				cancelReason = reason
			} else {
				return SettleResult{}, reserveErr
			}
		} else {
			nextPayload, err = setFishFoodRunContext(item.PayloadRaw, nextContext)
			if err != nil {
				return SettleResult{}, err
			}
			nextStartAt := operationEffectSubsequentStartAt(item, timing, now, 1)
			_, changed, timingErr := workEfficiencyBatchLimit(item, timing, now, 1)
			if timingErr != nil {
				return SettleResult{}, timingErr
			}
			if changed {
				base := time.Duration(runtime.BaseDurationMs) * time.Millisecond
				if base <= 0 {
					base = time.Duration(item.Spec.DurationS) * time.Second
				}
				duration, extra, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, base, nextStartAt)
				if err != nil {
					return SettleResult{}, err
				}
				nextPayload, err = setWorkQueueRuntime(item, nextPayload, fishFoodExecutorRuntime{BaseDurationMs: base.Milliseconds(), DurationMs: duration.Milliseconds(), ExtraAttrExp: extra}, nextStartAt)
				if err != nil {
					return SettleResult{}, err
				}
				timing.NextDuration = time.Duration(duration.Milliseconds()) * time.Millisecond
			}
			operation := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, nextStartAt, doneAfter+1, nextPayload)
			nextSnapshot := operationeffect.EmptySnapshot(operation)
			if item.EffectSession != nil {
				prepared := item.EffectSession.Prepare(operation)
				nextSnapshot, err = item.EffectSession.Commit(prepared)
				if err != nil {
					return SettleResult{}, err
				}
			}
			nextPayload, err = setQueueOperationEffectSnapshot(nextPayload, nextSnapshot)
			if err != nil {
				return SettleResult{}, err
			}
			nextItem := item
			nextItem.DoneCount = doneAfter
			nextPayload, err = e.store.freezeOrdinarySettlement(nextItem, nextPayload)
			if err != nil {
				return SettleResult{}, err
			}
			nextStarted = true
			nextPayload, err = freezeQueuePassive(item, nextPayload, nextStartAt, operationeffect.DomainLifeAction)
			if err != nil {
				return SettleResult{}, err
			}
			for itemID, quantity := range item.Spec.Costs {
				if quantity > 0 {
					itemCosts[itemID] = quantity
				}
			}
		}
	}
	result := SettleResult{
		OwnerAttrExp:      ownerExp,
		Completed:         1,
		DoneAfter:         doneAfter,
		CompletedAt:       completedAt,
		NextRunStartAt:    completedAt,
		NextRunEndsAt:     completedAt.Add(timing.NextDuration),
		CancelAfterSettle: cancelAfterSettle,
		CancelReason:      cancelReason,
		NextPayload:       nextPayload,
		ItemCosts:         itemCosts,
		ItemRewards:       itemRewards,
		AttrExp:           attrGain,
	}
	if !nextStarted && !cancelAfterSettle && doneAfter < item.TotalCount {
		return SettleResult{}, errors.New("fish food continuation was not materialized")
	}
	return result, nil
}

func (e *fishFoodExecutor) ClassifyStartError(err error) (bool, string) {
	if errors.Is(err, ErrInsufficientItem) {
		return true, ActionQueueCancelReasonCostInsufficient
	}
	if errors.Is(err, ErrFishFoodIngredientsInvalid) || errors.Is(err, ErrFishFoodArgsInvalid) {
		return true, ActionQueueCancelReasonTargetInvalid
	}
	if errors.Is(err, ErrStatusRequireNotMet) {
		return true, ActionQueueCancelReasonStatusRequirement
	}
	if errors.Is(err, ErrBuildingRequireNotMet) {
		return true, ActionQueueCancelReasonBuildingRequirement
	}
	return false, ""
}

func (e *fishFoodExecutor) validateRecipeContract(spec ActionSpec) error {
	if strings.TrimSpace(e.recipe.ActionID) != strings.TrimSpace(spec.ID) ||
		strings.TrimSpace(e.recipe.OutputItemID) == "" ||
		strings.TrimSpace(spec.MainOutput) != strings.TrimSpace(e.recipe.OutputItemID) ||
		strings.TrimSpace(spec.MainType) != "cooking" ||
		(spec.SubType != "cooking_fish_food" && spec.SubType != "cooking_grilled_fish" && spec.SubType != "cooking_fish_soup") ||
		(e.recipe.QualityRulesetVersion != fishFoodQualityRulesetV1 && e.recipe.QualityRulesetVersion != fishFoodQualityRulesetV2) ||
		len(e.recipe.FishIngredients) < 1 {
		return ErrFishFoodArgsInvalid
	}
	return nil
}

func (e *fishFoodExecutor) reserveRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, runContext fishFoodExecutorRunContext) (fishFoodExecutorRunContext, error) {
	affordable, err := e.store.maxAffordableActionStartsTx(ctx, tx, userID, item.Spec.Costs, 1)
	if err != nil {
		return fishFoodExecutorRunContext{}, err
	}
	if affordable < 1 {
		return fishFoodExecutorRunContext{}, ErrInsufficientItem
	}
	var (
		selected []selectedFishFoodIngredient
	)
	if runContext.IngredientMode == fishFoodIngredientModeSpecific {
		selected, err = e.selectSpecificIngredientsTx(ctx, tx, userID, runContext.FishIngredientInstances)
	} else {
		selected, err = e.selectAutomaticIngredientsTx(ctx, tx, userID)
	}
	if err != nil {
		return fishFoodExecutorRunContext{}, err
	}
	outputSnapshot := map[string]any{}
	if runContext.IngredientMode == fishFoodIngredientModeSpecific {
		outputSnapshot, err = e.buildSpecificOutputSnapshot(selected)
		if err != nil {
			return fishFoodExecutorRunContext{}, err
		}
	}
	reserved := make([]fishFoodReservedIngredient, 0, len(selected))
	for _, ingredient := range selected {
		if err := e.store.LedgerSetStateTx(ctx, tx, ingredient.InstanceID, userID, "inventory", "action_queue", fishFoodCatchTemplateItemID, 1, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryActionQueue,
			Direction:     InventoryAuditDirectionConsume,
			SubjectType:   "queue",
			SubjectID:     strconv.FormatInt(item.QueueID, 10),
			SourceSurface: "inventory",
			TargetSurface: "action_queue",
			Details: map[string]any{
				"type":      "fish_food_fish_reserve",
				"queueId":   item.QueueID,
				"actionId":  item.ActionID,
				"speciesId": ingredient.Fish.SpeciesID,
			},
		}); err != nil {
			return fishFoodExecutorRunContext{}, err
		}
		reserved = append(reserved, fishFoodReservedIngredient{InstanceID: ingredient.InstanceID, SpeciesID: ingredient.Fish.SpeciesID})
	}
	if err := e.store.applyActionCostsBatchTx(ctx, tx, userID, item.Spec.Costs, 1, InventoryLedgerMeta{
		Category:      InventoryAuditCategoryActionQueue,
		Direction:     InventoryAuditDirectionConsume,
		SubjectType:   "queue",
		SubjectID:     strconv.FormatInt(item.QueueID, 10),
		SourceSurface: "inventory",
		TargetSurface: "action_queue",
		Details: map[string]any{
			"type":     "fish_food_cost",
			"queueId":  item.QueueID,
			"actionId": item.ActionID,
		},
	}); err != nil {
		return fishFoodExecutorRunContext{}, err
	}
	runContext.RunReserved = true
	runContext.ReservedFish = reserved
	runContext.OutputBaseSnapshot = outputSnapshot
	return runContext, nil
}

func (e *fishFoodExecutor) reserveRunWithRollbackTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, runContext fishFoodExecutorRunContext) (fishFoodExecutorRunContext, error) {
	if _, err := tx.Exec(ctx, `SAVEPOINT fish_food_continuation_start`); err != nil {
		return fishFoodExecutorRunContext{}, err
	}
	next, reserveErr := e.reserveRunTx(ctx, tx, userID, item, runContext)
	if reserveErr == nil {
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT fish_food_continuation_start`); err != nil {
			return fishFoodExecutorRunContext{}, err
		}
		return next, nil
	}
	if _, rollbackErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT fish_food_continuation_start`); rollbackErr != nil {
		return fishFoodExecutorRunContext{}, errors.Join(reserveErr, rollbackErr)
	}
	if _, releaseErr := tx.Exec(ctx, `RELEASE SAVEPOINT fish_food_continuation_start`); releaseErr != nil {
		return fishFoodExecutorRunContext{}, errors.Join(reserveErr, releaseErr)
	}
	return fishFoodExecutorRunContext{}, reserveErr
}

func (e *fishFoodExecutor) selectSpecificIngredientsTx(ctx context.Context, tx pgx.Tx, userID int64, bindings map[string][]int64) ([]selectedFishFoodIngredient, error) {
	selected := make([]selectedFishFoodIngredient, 0, maxFishFoodSelectedInstances)
	for _, requirement := range e.recipe.FishIngredients {
		speciesID := strings.TrimSpace(requirement.SpeciesID)
		instanceIDs := bindings[speciesID]
		if len(instanceIDs) != requirement.Quantity {
			return nil, ErrFishFoodIngredientsInvalid
		}
		for _, instanceID := range instanceIDs {
			ingredient, err := lockFishFoodIngredientTx(ctx, tx, instanceID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrFishFoodIngredientsInvalid) {
					return nil, ErrFishFoodIngredientsInvalid
				}
				return nil, err
			}
			if ingredient.InstanceID <= 0 ||
				ingredient.OwnerID != userID ||
				ingredient.State != "inventory" ||
				ingredient.Quantity != 1 ||
				ingredient.ArchivedAt != nil ||
				ingredient.Fish.SpeciesID != speciesID {
				return nil, ErrFishFoodIngredientsInvalid
			}
			selected = append(selected, ingredient)
		}
	}
	return selected, nil
}

func (e *fishFoodExecutor) selectAutomaticIngredientsTx(ctx context.Context, tx pgx.Tx, userID int64) ([]selectedFishFoodIngredient, error) {
	selected := make([]selectedFishFoodIngredient, 0, maxFishFoodSelectedInstances)
	for _, requirement := range e.recipe.FishIngredients {
		rows, err := tx.Query(ctx, `
			SELECT instance_id, fish_data
			FROM player_item_instances
			WHERE user_id=$1
			  AND template_item_id=$2
			  AND state='inventory'
			  AND archived_at IS NULL
			  AND quantity=1
			  AND fish_data->>'speciesId'=$3
			ORDER BY
			  CASE WHEN jsonb_typeof(fish_data->'genes'->'mutations')='array'
			       THEN jsonb_array_length(fish_data->'genes'->'mutations') ELSE 0 END,
			  COALESCE((fish_data->>'lengthCm')::double precision, 0),
			  COALESCE((fish_data->>'weightKg')::double precision, 0),
			  instance_id
			LIMIT $4
			FOR UPDATE
		`, userID, fishFoodCatchTemplateItemID, strings.TrimSpace(requirement.SpeciesID), requirement.Quantity)
		if err != nil {
			return nil, err
		}
		count := 0
		for rows.Next() {
			var ingredient selectedFishFoodIngredient
			var fishRaw []byte
			if err := rows.Scan(&ingredient.InstanceID, &fishRaw); err != nil {
				rows.Close()
				return nil, err
			}
			if json.Unmarshal(fishRaw, &ingredient.Fish) != nil || strings.TrimSpace(ingredient.Fish.SpeciesID) != strings.TrimSpace(requirement.SpeciesID) {
				rows.Close()
				return nil, ErrFishFoodIngredientsInvalid
			}
			selected = append(selected, ingredient)
			count++
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		if count != requirement.Quantity {
			return nil, ErrInsufficientItem
		}
	}
	return selected, nil
}

func lockFishFoodIngredientTx(ctx context.Context, tx pgx.Tx, instanceID int64) (selectedFishFoodIngredient, error) {
	var ingredient selectedFishFoodIngredient
	var templateItemID string
	var fishRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT instance_id, user_id, state, quantity, archived_at, template_item_id, fish_data
		FROM player_item_instances
		WHERE instance_id=$1
		FOR UPDATE
	`, instanceID).Scan(
		&ingredient.InstanceID,
		&ingredient.OwnerID,
		&ingredient.State,
		&ingredient.Quantity,
		&ingredient.ArchivedAt,
		&templateItemID,
		&fishRaw,
	)
	if err != nil {
		return selectedFishFoodIngredient{}, err
	}
	if templateItemID != fishFoodCatchTemplateItemID || json.Unmarshal(fishRaw, &ingredient.Fish) != nil {
		return selectedFishFoodIngredient{}, ErrFishFoodIngredientsInvalid
	}
	return ingredient, nil
}

func (e *fishFoodExecutor) buildSpecificOutputSnapshot(selected []selectedFishFoodIngredient) (map[string]any, error) {
	def, ok, err := e.store.resolveOperationEffectDefinitionForItem(e.recipe.OutputItemID, nil)
	if err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrOperationEffectNotFound
	}
	quality, ingredients, err := e.resolveQuality(selected)
	if err != nil {
		return nil, err
	}
	params, err := scaledFishFoodEffectParams(def, quality.Multiplier)
	if e.recipe.QualityRulesetVersion == fishFoodQualityRulesetV2 {
		params, err = scaleFishFoodParamsV2(def.Params, e.recipe.ParameterScaling, &quality)
	}
	if err != nil {
		return nil, err
	}
	customDefinition := def
	customDefinition.Params = params
	lifetime := def.Lifetime
	if lifetime.Kind == operationeffect.LifetimeCharges {
		lifetime.Charges = fishFoodScaledCharges(e.recipe, lifetime.Charges, &quality)
	} else {
		lifetime.DurationSeconds = fishFoodScaledDuration(e.recipe, lifetime.DurationSeconds, &quality)
	}
	customDefinition.Lifetime = lifetime
	if err := e.store.operationEffectRegistry.ValidateDefinition(customDefinition); err != nil {
		return nil, err
	}
	return map[string]any{
		"operationEffect": operationEffectItemSnapshot{
			SchemaVersion:  2,
			DefinitionID:   def.ID,
			RulesetVersion: def.RulesetVersion,
			Params:         params,
			Lifetime:       &lifetime,
		},
		"fishFood": fishFoodItemSnapshot{
			SchemaVersion:         fishFoodSnapshotSchemaVersion,
			RecipeID:              e.recipe.ID,
			RecipeRulesetVersion:  e.recipe.RulesetVersion,
			QualityRulesetVersion: e.recipe.QualityRulesetVersion,
			Quality:               quality,
			Ingredients:           ingredients,
		},
	}, nil
}

func (e *fishFoodExecutor) resolveQuality(selected []selectedFishFoodIngredient) (fishFoodQualitySnapshot, []fishFoodIngredientSnapshot, error) {
	if e.recipe.QualityRulesetVersion == fishFoodQualityRulesetV2 {
		return e.resolveQualityV2(selected)
	}
	if strings.TrimSpace(e.recipe.QualityRulesetVersion) != fishFoodQualityRulesetV1 || len(selected) == 0 {
		return fishFoodQualitySnapshot{}, nil, ErrFishFoodIngredientsInvalid
	}
	rows := make([]fishFoodIngredientSnapshot, 0, len(selected))
	var lengthTotal, weightTotal, traitTotal, multiplierTotal float64
	mutationIDs := make(map[string]struct{})
	for _, selectedFish := range selected {
		species := e.store.gameData.FishingSpeciesByID(selectedFish.Fish.SpeciesID)
		if species == nil {
			return fishFoodQualitySnapshot{}, nil, ErrFishFoodIngredientsInvalid
		}
		row := fishFoodIngredientQualityV1(*species, selectedFish.Fish)
		rows = append(rows, row)
		lengthTotal += row.LengthScore
		weightTotal += row.WeightScore
		traitTotal += row.TraitAverage
		multiplierTotal += row.Multiplier
		for _, mutationID := range row.Mutations {
			mutationIDs[mutationID] = struct{}{}
		}
	}
	count := float64(len(rows))
	return fishFoodQualitySnapshot{
		Multiplier:    roundFloat(clampFloat(multiplierTotal/count, 0.75, 1.35), 4),
		LengthScore:   roundFloat(lengthTotal/count, 4),
		WeightScore:   roundFloat(weightTotal/count, 4),
		TraitAverage:  roundFloat(traitTotal/count, 4),
		MutationCount: len(mutationIDs),
	}, rows, nil
}

func fishFoodIngredientQualityV1(species catalog.FishingSpecies, fish FishInstanceData) fishFoodIngredientSnapshot {
	lengthRange := math.Max(0.0001, species.MaxLengthCM-species.MinLengthCM)
	lengthScore := clampFloat((fish.LengthCM-species.MinLengthCM)/lengthRange, 0, 1)
	equivalentLength := species.MinLengthCM
	if species.ShapeFactor > 0 && fish.WeightKG > 0 {
		equivalentLength = math.Cbrt(fish.WeightKG * 100_000 / species.ShapeFactor)
	}
	weightScore := clampFloat((equivalentLength-species.MinLengthCM)/lengthRange, 0, 1)
	traits := []float64{
		fish.Genes.SizePotential,
		fish.Genes.GrowthRate,
		fish.Genes.FeedEfficiency,
		fish.Genes.Vitality,
		fish.Genes.Lifespan,
		fish.Genes.Fertility,
	}
	traitAverage := 0.0
	for _, value := range traits {
		traitAverage += clampFloat(value, -1, 1)
	}
	traitAverage /= float64(len(traits))
	mutationSet := make(map[string]struct{})
	for _, rawMutationID := range fish.Genes.Mutations {
		mutationID := strings.TrimSpace(rawMutationID)
		if mutationID != "" {
			mutationSet[mutationID] = struct{}{}
		}
	}
	mutations := make([]string, 0, len(mutationSet))
	for mutationID := range mutationSet {
		mutations = append(mutations, mutationID)
	}
	sort.Strings(mutations)
	mutationCount := len(mutations)
	if mutationCount > 3 {
		mutationCount = 3
	}
	multiplier := 1 + (lengthScore-0.5)*0.20 + (weightScore-0.5)*0.20 + traitAverage*0.10 + float64(mutationCount)*0.04
	return fishFoodIngredientSnapshot{
		SpeciesID:    strings.TrimSpace(fish.SpeciesID),
		LengthCM:     roundFloat(fish.LengthCM, 1),
		WeightKG:     roundFloat(fish.WeightKG, 3),
		LengthScore:  roundFloat(lengthScore, 4),
		WeightScore:  roundFloat(weightScore, 4),
		TraitAverage: roundFloat(traitAverage, 4),
		Mutations:    mutations,
		Multiplier:   roundFloat(clampFloat(multiplier, 0.75, 1.35), 4),
	}
}

func scaledFishFoodEffectParams(def operationeffect.Definition, multiplier float64) (json.RawMessage, error) {
	multiplier = clampFloat(multiplier, 0.75, 1.35)
	switch def.Kind {
	case alchemyeffect.KindInfusionCapacityFloorTradeoff:
		var base alchemyeffect.CapacityFloorTradeoffParams
		if err := json.Unmarshal(def.Params, &base); err != nil {
			return nil, err
		}
		params := alchemyeffect.CapacityFloorTradeoffParams{
			ExtraMaterialQuantity:      maxInt64Value(1, int64(math.Round(float64(base.ExtraMaterialQuantity)*multiplier))),
			PositiveFloorReductionRate: roundFloat(base.PositiveFloorReductionRate/multiplier, 4),
		}
		return json.Marshal(params)
	case alchemyeffect.KindInfusionPrecisionCeilingBonus:
		var base alchemyeffect.PrecisionCeilingBonusParams
		if err := json.Unmarshal(def.Params, &base); err != nil {
			return nil, err
		}
		return json.Marshal(alchemyeffect.PrecisionCeilingBonusParams{
			PositiveCeilingBonusRate: roundFloat(base.PositiveCeilingBonusRate*multiplier, 4),
		})
	default:
		return nil, ErrOperationEffectItemInvalid
	}
}

func maxInt64Value(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func (e *fishFoodExecutor) grantOutputTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, baseSnapshot map[string]any, quantity int64, meta InventoryLedgerMeta) error {
	if quantity <= 0 {
		return nil
	}
	for index := int64(0); index < quantity; index++ {
		instanceID, err := e.store.AddInventoryItemInstanceTx(ctx, tx, userID, e.recipe.OutputItemID, 1, baseSnapshot, nil)
		if err != nil {
			return err
		}
		entryMeta := meta
		entryMeta.Details = cloneAnyMap(meta.Details)
		entryMeta.InstanceID = &instanceID
		if entryMeta.Details == nil {
			entryMeta.Details = map[string]any{}
		}
		entryMeta.Details["outputItemId"] = e.recipe.OutputItemID
		if err := e.store.LedgerLogOnlyTx(ctx, tx, userID, e.recipe.OutputItemID, 1, entryMeta); err != nil {
			return err
		}
	}
	return nil
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func parseFishFoodExecutorRuntime(payloadRaw []byte) (fishFoodExecutorRuntime, bool) {
	var envelope queueRuntimeEnvelope
	if len(payloadRaw) == 0 || json.Unmarshal(payloadRaw, &envelope) != nil || envelope.ExecutorID != fishFoodExecutorID {
		return fishFoodExecutorRuntime{}, false
	}
	var runtime fishFoodExecutorRuntime
	if json.Unmarshal(envelope.Runtime, &runtime) != nil || runtime.DurationMs <= 0 {
		return fishFoodExecutorRuntime{}, false
	}
	if runtime.ExtraAttrExp == nil {
		runtime.ExtraAttrExp = map[string]float64{}
	}
	return runtime, true
}

func (runtime fishFoodExecutorRuntime) Duration() time.Duration {
	if runtime.DurationMs <= 0 {
		return 0
	}
	return normalizeActionQueueRunDuration(time.Duration(runtime.DurationMs) * time.Millisecond)
}

func parseFishFoodRunContext(payloadRaw []byte) (fishFoodExecutorRunContext, bool) {
	var envelope queueRuntimeEnvelope
	if len(payloadRaw) == 0 || json.Unmarshal(payloadRaw, &envelope) != nil || envelope.ExecutorID != fishFoodExecutorID {
		return fishFoodExecutorRunContext{}, false
	}
	var runContext fishFoodExecutorRunContext
	if json.Unmarshal(envelope.RunContext, &runContext) != nil {
		return fishFoodExecutorRunContext{}, false
	}
	if runContext.IngredientMode != fishFoodIngredientModeAuto && runContext.IngredientMode != fishFoodIngredientModeSpecific {
		return fishFoodExecutorRunContext{}, false
	}
	if runContext.FishIngredientInstances == nil {
		runContext.FishIngredientInstances = map[string][]int64{}
	}
	if runContext.OutputBaseSnapshot == nil {
		runContext.OutputBaseSnapshot = map[string]any{}
	}
	return runContext, true
}

func setFishFoodRunContext(payloadRaw []byte, runContext fishFoodExecutorRunContext) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil || envelope.ExecutorID != fishFoodExecutorID {
		return nil, ErrFishFoodArgsInvalid
	}
	raw, err := json.Marshal(runContext)
	if err != nil {
		return nil, err
	}
	envelope.RunContext = raw
	return json.Marshal(envelope)
}

func cloneFishFoodInstanceBindings(input map[string][]int64) map[string][]int64 {
	out := make(map[string][]int64, len(input))
	for speciesID, instanceIDs := range input {
		out[strings.TrimSpace(speciesID)] = append([]int64(nil), instanceIDs...)
	}
	return out
}
