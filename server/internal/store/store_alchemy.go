package store

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/transmute"

	"github.com/jackc/pgx/v5"
)

type alchemyRuntime struct {
	BaseDurationMs int64              `json:"baseDurationMs,omitempty"`
	DurationMs     int64              `json:"durationMs"`
	ExtraAttrExp   map[string]float64 `json:"extraAttrExp,omitempty"`
}

type alchemyRunContextEnvelope struct {
	Args                  json.RawMessage  `json:"args,omitempty"`
	ReservedTemplateCosts map[string]int64 `json:"reservedTemplateCosts,omitempty"`
}

type alchemyConvertArgs struct {
	SourceItemID string `json:"sourceItemId"`
	GroupID      string `json:"groupId"`
	SourceCount  int    `json:"sourceCount"`
}

type alchemyBenchArgs struct {
	CarrierInstanceID int64            `json:"carrierInstanceId"`
	CarrierItemID     string           `json:"carrierItemId,omitempty"`
	Materials         map[string]int64 `json:"materials"`
	ComboRecipeID     string           `json:"comboRecipeId,omitempty"`
	MediumItemID      string           `json:"mediumItemId,omitempty"`
	FishInstanceID    int64            `json:"fishInstanceId,omitempty"`
	FishInstanceIDs   []int64          `json:"fishInstanceIds,omitempty"`
}

type alchemyTransmuteArgs struct {
	ItemID     string `json:"itemId"`
	InstanceID int64  `json:"instanceId,omitempty"`
	TotalCount int    `json:"totalCount"`
}

const (
	// Bump when newly persisted infusion or inheritance results use a new
	// compatibility epoch. Existing affixes intentionally keep their stamp.
	AlchemyCurrentRulesetVersion       = "alchemy-v2"
	alchemyInfusionPositiveMaxRatio    = 0.85
	alchemyQuantityLinearStabilityKnee = 5.0
	alchemyQuantityLogScale            = 0.11231056256176605
	alchemyQuantityLogRate             = 4.4875174417026775
	alchemyQuantityBridgeStart         = 10.0
	alchemyQuantityPursuitStart        = 30.0
	alchemyQuantitySoftCapStart        = 100.0
	alchemyQuantitySoftCap             = 32.0
)

type AlchemyDiscoveries struct {
	Groups []string `json:"groups"`
	Traits []string `json:"traits"`
	Combos []string `json:"combos"`
}

type alchemyExecutor struct {
	store    *Store
	actionID string
}

func newAlchemyExecutor(store *Store, actionID string) *alchemyExecutor {
	return &alchemyExecutor{store: store, actionID: actionID}
}

func (e *alchemyExecutor) ActionID() string { return e.actionID }

func (e *alchemyExecutor) OfflineProjectionPolicy() QueueOfflineProjectionPolicy {
	return QueueOfflineProjectionTerminal
}

func (e *alchemyExecutor) durationRefreshMode() queueDurationRefreshMode {
	switch strings.TrimSpace(e.actionID) {
	case "alchemy.convert", "alchemy.bench", "alchemy.transmute", "alchemy.inherit":
		return queueDurationRefreshPending
	default:
		return 0
	}
}

func (e *alchemyExecutor) refreshQueueDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (json.RawMessage, bool, error) {
	nextPayload, _, err := e.refreshNextRuntimePayloadTx(ctx, tx, userID, item, item.PayloadRaw)
	if err != nil {
		return nil, false, err
	}
	return nextPayload, string(nextPayload) != string(item.PayloadRaw), nil
}

func isAlchemyActionID(actionID string) bool {
	switch strings.TrimSpace(actionID) {
	case "alchemy.convert", "alchemy.bench", "alchemy.transmute", "alchemy.smelt", "alchemy.inherit":
		return true
	default:
		return false
	}
}

func (e *alchemyExecutor) ValidateAndNormalizeArgs(raw json.RawMessage) (any, error) {
	switch e.actionID {
	case "alchemy.convert":
		var args alchemyConvertArgs
		if err := decodeAlchemyArgsStrict(raw, &args); err != nil {
			return nil, ErrAlchemyArgsInvalid
		}
		args.SourceItemID = strings.TrimSpace(args.SourceItemID)
		args.GroupID = strings.TrimSpace(args.GroupID)
		// Queue count owns repetition. Convert args describe one run only.
		args.SourceCount = 1
		if args.SourceItemID == "" {
			return nil, ErrAlchemyArgsInvalid
		}
		return args, nil
	case "alchemy.bench":
		var args alchemyBenchArgs
		if err := decodeAlchemyArgsStrict(raw, &args); err != nil {
			return nil, ErrAlchemyArgsInvalid
		}
		normalized := map[string]int64{}
		for itemID, qty := range args.Materials {
			itemID = strings.TrimSpace(itemID)
			if itemID == "" || qty <= 0 {
				continue
			}
			normalized[itemID] += qty
		}
		args.Materials = normalized
		args.ComboRecipeID = strings.TrimSpace(args.ComboRecipeID)
		args.MediumItemID = strings.TrimSpace(args.MediumItemID)
		args.CarrierItemID = strings.TrimSpace(args.CarrierItemID)
		if args.FishInstanceID > 0 || len(args.FishInstanceIDs) > 0 {
			return nil, ErrFishingInfusionUnavailable
		}
		if args.CarrierInstanceID <= 0 || len(args.Materials) == 0 {
			return nil, ErrAlchemyArgsInvalid
		}
		if e.store != nil {
			medium := e.store.gameData.AlchemyMediumByItemID(args.MediumItemID)
			if len(args.Materials) != catalog.EffectiveBenchSlots(medium) {
				return nil, ErrAlchemyTraitSlotOverflow
			}
			if args.ComboRecipeID != "" && e.store.matchAlchemyCombo(args.Materials, args.ComboRecipeID) == nil {
				return nil, ErrAlchemyArgsInvalid
			}
		}
		return args, nil
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		if err := decodeAlchemyArgsStrict(raw, &args); err != nil {
			return nil, ErrAlchemyArgsInvalid
		}
		args.ItemID = strings.TrimSpace(args.ItemID)
		// Queue count owns repetition. Transmute args describe one run only.
		args.TotalCount = 1
		if args.ItemID == "" {
			return nil, ErrAlchemyArgsInvalid
		}
		return args, nil
	case "alchemy.inherit":
		args, err := e.validateAndNormalizeInheritArgs(raw)
		if err != nil {
			return nil, err
		}
		return args, nil
	default:
		return nil, ErrAlchemyArgsInvalid
	}
}

func decodeAlchemyArgsStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("alchemy args contain trailing data")
	}
	return nil
}

func (e *alchemyExecutor) FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, spec ActionSpec, args any, _ time.Time) (json.RawMessage, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	baseDuration := e.store.resolveAlchemyBaseDuration(spec, args)
	frozenDuration, frozenExtraExp, err := e.store.resolveDurationAndBonusTx(ctx, tx, userID, actorType, actorID, spec, baseDuration)
	if err != nil {
		return nil, err
	}
	frozenExtraExp = mergeAttrExp(frozenExtraExp, e.store.resolveAlchemyExtraExp(spec, args))
	runtimeRaw, err := json.Marshal(alchemyRuntime{
		BaseDurationMs: int64(baseDuration / time.Millisecond),
		DurationMs:     int64(frozenDuration / time.Millisecond),
		ExtraAttrExp:   frozenExtraExp,
	})
	if err != nil {
		return nil, err
	}
	runCtxRaw, err := marshalAlchemyRunContext(args, nil)
	if err != nil {
		return nil, err
	}
	return json.Marshal(queueRuntimeEnvelope{
		ActionID:   spec.ID,
		ExecutorID: e.ActionID(),
		Runtime:    runtimeRaw,
		RunContext: runCtxRaw,
	})
}

func (e *alchemyExecutor) StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, at time.Time) (json.RawMessage, error) {
	return e.startRunWithTimingTx(ctx, tx, userID, item, at, nil)
}

// refreshed is scoped to this next start, after XP settlement. It reuses only
// the base timing already resolved by refreshNextRuntimePayloadTx, never food.
func (e *alchemyExecutor) startRunWithTimingTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, at time.Time, refreshed *parsedAlchemyRuntime) (json.RawMessage, error) {
	runtime := parseAlchemyRuntime(item.PayloadRaw)
	runCtxArgs, err := parseAlchemyRunContextArgs(e.actionID, item.PayloadRaw)
	if err != nil {
		return nil, err
	}
	if e.actionID == "alchemy.bench" {
		if err := e.validateAlchemyBenchOperationEffectsTx(ctx, tx, userID, item); err != nil {
			return nil, err
		}
	}
	baseDuration := time.Duration(0)
	if runtime.BaseDurationMs > 0 {
		baseDuration = time.Duration(runtime.BaseDurationMs) * time.Millisecond
	}
	if baseDuration <= 0 {
		baseDuration = e.store.resolveAlchemyBaseDuration(item.Spec, runCtxArgs)
	}
	var resolvedDuration time.Duration
	var extraAttrExp map[string]float64
	if refreshed != nil {
		baseDuration = time.Duration(refreshed.BaseDurationMs) * time.Millisecond
		resolvedDuration = time.Duration(refreshed.DurationMs) * time.Millisecond
		extraAttrExp = refreshed.ExtraAttrExp
	} else {
		resolvedDuration, extraAttrExp, err = e.resolveSettlementWorkDurationTx(ctx, tx, userID, item, baseDuration, at)
		if err != nil {
			return nil, err
		}
		extraAttrExp = mergeAttrExp(extraAttrExp, e.store.resolveAlchemyExtraExp(item.Spec, runCtxArgs))
	}
	if resolvedDuration <= 0 {
		resolvedDuration = minimumActionQueueRunDuration
	}
	if e.actionID == "alchemy.transmute" {
		mods, err := transmute.Resolve(alchemyOperationEffectSnapshot(item))
		if err != nil {
			return nil, err
		}
		if mods.DurationSeconds > 0 {
			resolvedDuration = time.Second
		}
	}
	reservedCosts, err := e.reserveAlchemyRunCostsTx(ctx, tx, userID, item)
	if err != nil {
		return nil, err
	}
	if e.actionID == "alchemy.bench" {
		if args, ok := runCtxArgs.(alchemyBenchArgs); ok && args.FishInstanceID > 0 {
			if err := e.store.reserveFishingInfusionInstancesTx(ctx, tx, userID, &args); err != nil {
				return nil, err
			}
			runCtxArgs = args
		}
	}
	if e.actionID == "alchemy.inherit" {
		var args alchemyInheritArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, err
		}
		_, _, _, medium, err := e.store.validateAlchemyInheritSetupTx(ctx, tx, userID, args, false)
		if err != nil {
			return nil, err
		}
		if err := e.store.validateAlchemyInheritMediumLevelForItemTx(ctx, tx, userID, item, medium); err != nil {
			return nil, err
		}
		if err := e.store.reserveAlchemyInheritInstancesTx(ctx, tx, userID, args); err != nil {
			return nil, err
		}
	}
	nextPayload, err := setWorkQueueRuntime(item, item.PayloadRaw, alchemyRuntime{
		BaseDurationMs: int64(baseDuration / time.Millisecond),
		DurationMs:     int64(resolvedDuration / time.Millisecond),
		ExtraAttrExp:   extraAttrExp,
	}, at)
	if err != nil {
		return nil, err
	}
	return setAlchemyReservedCosts(nextPayload, runCtxArgs, reservedCosts)
}

func (e *alchemyExecutor) CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	reserved := parseAlchemyReservedCosts(item.PayloadRaw)
	if e.actionID == "alchemy.bench" {
		var args alchemyBenchArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err == nil && args.FishInstanceID > 0 {
			if err := e.store.releaseFishingInfusionInstancesTx(ctx, tx, userID, args); err != nil {
				return err
			}
		}
	}
	if e.actionID == "alchemy.inherit" {
		var args alchemyInheritArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err == nil {
			if err := e.store.releaseAlchemyInheritReservationsTx(ctx, tx, userID, args); err != nil {
				return err
			}
		}
	}
	if len(reserved) > 0 {
		return e.store.refundAlchemyReservedCostsTx(ctx, tx, userID, item, reserved)
	}
	return nil
}

func (e *alchemyExecutor) alchemyRunCostsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (map[string]int64, error) {
	costs := map[string]int64{}
	switch e.actionID {
	case "alchemy.convert":
		var args alchemyConvertArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, err
		}
		group := e.store.gameData.AlchemyGroupByID(args.GroupID)
		if group == nil {
			group = e.store.gameData.AlchemyGroupByItemID(args.SourceItemID)
		}
		if group == nil {
			return nil, ErrAlchemyGroupNotFound
		}
		costs[args.SourceItemID] += 1
		if group.GoldCost > 0 {
			costs[GoldCoinItemID] += group.GoldCost
		}
	case "alchemy.bench":
		var args alchemyBenchArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, err
		}
		if args.FishInstanceID > 0 {
			preview, err := e.store.fishingInfusionPreviewTx(ctx, tx, userID, args.FishInstanceID, "inventory", alchemyDerived{})
			if err != nil {
				return nil, err
			}
			costs[fishingInfusionMediumItemID] = preview.MediumCount
			costs[GoldCoinItemID] = preview.GoldCost
			break
		}
		for itemID, qty := range args.Materials {
			if qty > 0 {
				costs[itemID] += qty
			}
		}
		if args.MediumItemID != "" {
			medium := e.store.gameData.AlchemyMediumByItemID(args.MediumItemID)
			if medium == nil {
				return nil, ErrAlchemyMediumUnknown
			}
			costs[medium.ItemID] += medium.ConsumePerCraft
		}
		if gold := e.store.alchemyBenchGoldCost(args.Materials, args.ComboRecipeID, e.store.gameData.AlchemyMediumByItemID(args.MediumItemID)); gold > 0 {
			costs[GoldCoinItemID] += gold
		}
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, err
		}
		costs[args.ItemID] += 1
	case "alchemy.inherit":
		var args alchemyInheritArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, err
		}
		var medium *catalog.AlchemyInheritMedium
		if args.MediumItemID != "" {
			medium = e.store.gameData.AlchemyInheritMediumByItemID(args.MediumItemID)
			if medium == nil {
				return nil, ErrAlchemyMediumUnknown
			}
		}
		costs = e.store.alchemyInheritPerRunCosts(args, medium)
	default:
		return nil, ErrAlchemyArgsInvalid
	}
	return costs, nil
}

func (e *alchemyExecutor) reserveAlchemyRunCostsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (map[string]int64, error) {
	costs, err := e.alchemyRunCostsTx(ctx, tx, userID, item)
	if err != nil {
		return nil, err
	}
	for itemID, qty := range costs {
		if qty <= 0 {
			delete(costs, itemID)
			continue
		}
		if err := e.store.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, qty, alchemyLedgerMeta(item, "inventory", "action_queue", InventoryAuditDirectionConsume, "alchemy_reserve_cost", map[string]any{"itemId": itemID})); err != nil {
			return nil, err
		}
	}
	return costs, nil
}

func (e *alchemyExecutor) SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error) {
	return e.SettleFastForwardTx(ctx, tx, userID, item, now, FastForwardBudget{MaxIterations: 1})
}

func (e *alchemyExecutor) SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return SettleResult{}, err
	}
	runtime := parseAlchemyRuntime(item.PayloadRaw)
	timing := resolveQueueRunTiming(item, now, parseQueueRuntimeDuration(item.PayloadRaw))
	limit, _, err := workEfficiencyBatchLimit(item, timing, now, budget.MaxIterations)
	if err != nil {
		return SettleResult{}, err
	}
	budget.MaxIterations = limit
	perStartEffects := e.operationEffectsNeedPerStart(item, timing, now)
	remaining := item.TotalCount - item.DoneCount
	completed, completedAt := completedRunsForQueueTiming(timing, now, remaining)
	if budget.MaxIterations > 0 && completed > budget.MaxIterations {
		completed = budget.MaxIterations
		completedAt = completedAtForQueueTiming(timing, completed)
	}
	if perStartEffects && completed > 1 {
		completed = 1
		completedAt = completedAtForQueueTiming(timing, completed)
	}
	if completed <= 0 {
		return SettleResult{}, nil
	}
	reservedCosts := parseAlchemyReservedCosts(item.PayloadRaw)
	actual, itemCosts, itemRewards, cancelAfter, cancelReason, err := e.settleFastForwardRunsTx(ctx, tx, userID, item, completed, reservedCosts, budget)
	if err != nil {
		if reason, stop := classifyAlchemyStopError(err); stop {
			if e.actionID == "alchemy.bench" {
				var args alchemyBenchArgs
				if parseErr := parseAlchemyRunContext(item.PayloadRaw, &args); parseErr == nil && args.FishInstanceID > 0 {
					if releaseErr := e.store.releaseFishingInfusionInstancesTx(ctx, tx, userID, args); releaseErr != nil {
						return SettleResult{}, releaseErr
					}
				}
			}
			if len(reservedCosts) > 0 {
				if refundErr := e.store.refundAlchemyReservedCostsTx(ctx, tx, userID, item, reservedCosts); refundErr != nil {
					return SettleResult{}, refundErr
				}
			}
			return SettleResult{
				Completed:         0,
				DoneAfter:         item.DoneCount,
				CompletedAt:       timing.StartAt,
				NextRunStartAt:    timing.StartAt,
				NextRunEndsAt:     timing.StartAt.Add(timing.CurrentDuration),
				CancelAfterSettle: true,
				CancelReason:      reason,
				NextPayload:       item.PayloadRaw,
			}, nil
		}
		return SettleResult{}, fmt.Errorf("%s: %w", e.actionID, err)
	}
	if actual <= 0 {
		if cancelAfter {
			return SettleResult{
				Completed:         0,
				DoneAfter:         item.DoneCount,
				CompletedAt:       timing.StartAt,
				NextRunStartAt:    timing.StartAt,
				NextRunEndsAt:     timing.StartAt.Add(timing.CurrentDuration),
				CancelAfterSettle: true,
				CancelReason:      cancelReason,
				NextPayload:       item.PayloadRaw,
			}, nil
		}
		return SettleResult{}, nil
	}
	completedAt = completedAtForQueueTiming(timing, actual)
	doneAfter := item.DoneCount + actual
	if !cancelAfter {
		cancelAfter = doneAfter < item.TotalCount && actual < completed
		if cancelAfter && cancelReason == "" {
			cancelReason = ActionQueueCancelReasonCostInsufficient
		}
	}
	attrGain := scaleAttrExp(mergeAttrExp(item.Spec.StatusExpReward, runtime.ExtraAttrExp), int64(actual))
	if isAlchemyActionID(item.ActionID) {
		attrGain = scaleAttrExp(runtime.ExtraAttrExp, int64(actual))
	} else if item.ActorType == ActionActorTypeKitty {
		attrGain = scaleAttrExpByMultiplier(attrGain, e.store.config.KittyStatusExpMultiplier)
	}
	expResult, err := e.store.applyAttrExpLockedResultTx(ctx, tx, userID, attrGain)
	if err != nil {
		return SettleResult{}, err
	}
	if expResult.LevelChanged {
		item.alchemyReads.invalidate()
	}
	if cancelAfter && cancelReason == "" {
		cancelReason = ActionQueueCancelReasonCostInsufficient
	}
	nextPayload := e.clearReservedPayload(item.PayloadRaw)
	nextDuration := timing.NextDuration
	if !cancelAfter && doneAfter < item.TotalCount {
		refreshedPayload, refreshedDuration, err := e.refreshNextRuntimePayloadTx(ctx, tx, userID, item, nextPayload, operationEffectSubsequentStartAt(item, timing, now, actual))
		if err != nil {
			return SettleResult{}, err
		}
		nextPayload = refreshedPayload
		if refreshedDuration > 0 {
			nextDuration = refreshedDuration
		}
		if perStartEffects && e.actionID == "alchemy.bench" {
			nextStartAt := completedAt
			if !item.PreserveScheduledTimeline {
				nextStartAt = now
			}
			nextPayload, cancelAfter, cancelReason, err = e.prepareNextAlchemyBenchEffectRunTx(
				ctx, tx, userID, item, nextPayload, nextStartAt, doneAfter,
			)
			if err != nil {
				return SettleResult{}, err
			}
		}
		if perStartEffects && (e.actionID == "alchemy.transmute" || e.actionID == "alchemy.convert") {
			startedAt := completedAt
			if !item.PreserveScheduledTimeline {
				startedAt = now
			}
			refreshed := parseAlchemyRuntime(nextPayload)
			nextPayload, cancelAfter, cancelReason, err = e.prepareNextAlchemyEffectRunTx(ctx, tx, userID, item, nextPayload, startedAt, doneAfter, &refreshed)
			if err != nil {
				return SettleResult{}, err
			}
			if !cancelAfter {
				nextDuration = time.Duration(parseAlchemyRuntime(nextPayload).DurationMs) * time.Millisecond
			}
		}
	}
	return SettleResult{
		Completed:         actual,
		DoneAfter:         doneAfter,
		CompletedAt:       completedAt,
		NextRunStartAt:    completedAt,
		NextRunEndsAt:     completedAt.Add(nextDuration),
		CancelAfterSettle: cancelAfter,
		CancelReason:      cancelReason,
		NextPayload:       nextPayload,
		ItemCosts:         itemCosts,
		ItemRewards:       itemRewards,
		AttrExp:           attrGain,
	}, nil
}

func (e *alchemyExecutor) settleFastForwardRunsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, completed int, reservedCosts map[string]int64, budget FastForwardBudget) (int, map[string]int64, map[string]int64, bool, string, error) {
	if completed <= 0 {
		return 0, map[string]int64{}, map[string]int64{}, false, "", nil
	}
	if completed == 1 {
		return e.settleFastForwardRunsIndividuallyTx(ctx, tx, userID, item, completed, reservedCosts, budget)
	}
	switch e.actionID {
	case "alchemy.convert":
		return e.settleAlchemyConvertBatchTx(ctx, tx, userID, item, completed, reservedCosts)
	case "alchemy.transmute":
		return e.settleAlchemyTransmuteBatchTx(ctx, tx, userID, item, completed, reservedCosts)
	case "alchemy.bench":
		return e.settleAlchemyBenchBatchTx(ctx, tx, userID, item, completed, reservedCosts)
	default:
		return e.settleFastForwardRunsIndividuallyTx(ctx, tx, userID, item, completed, reservedCosts, budget)
	}
}

func (e *alchemyExecutor) settleFastForwardRunsIndividuallyTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, completed int, reservedCosts map[string]int64, budget FastForwardBudget) (int, map[string]int64, map[string]int64, bool, string, error) {
	itemCosts := map[string]int64{}
	itemRewards := map[string]int64{}
	rng := alchemySingleRunRNG(item, item.DoneCount+1)
	loopStartedAt := time.Now()
	actual := 0
	cancelAfter := false
	cancelReason := ""
	for actual < completed {
		if shouldYieldAlchemyFastForwardBudget(loopStartedAt, budget, actual) {
			break
		}
		prepaid := actual == 0 && len(reservedCosts) > 0
		runItem := item
		runItem.DoneCount = item.DoneCount + actual
		costs, rewards, err := e.settleOnceTx(ctx, tx, userID, runItem, rng, prepaid)
		if err != nil {
			if reason, stop := classifyAlchemyStopError(err); stop {
				if prepaid {
					if refundErr := e.store.refundAlchemyReservedCostsTx(ctx, tx, userID, runItem, reservedCosts); refundErr != nil {
						return 0, nil, nil, false, "", refundErr
					}
				}
				cancelAfter = true
				cancelReason = reason
				break
			}
			return 0, nil, nil, false, "", err
		}
		mergeInt64Map(itemCosts, costs)
		mergeInt64Map(itemRewards, rewards)
		actual++
	}
	return actual, itemCosts, itemRewards, cancelAfter, cancelReason, nil
}

func (e *alchemyExecutor) settleAlchemyConvertBatchTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, completed int, reservedCosts map[string]int64) (int, map[string]int64, map[string]int64, bool, string, error) {
	var args alchemyConvertArgs
	if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
		return 0, nil, nil, false, "", err
	}
	group := e.store.gameData.AlchemyGroupByID(args.GroupID)
	if group == nil {
		group = e.store.gameData.AlchemyGroupByItemID(args.SourceItemID)
	}
	if group == nil {
		return 0, nil, nil, false, "", ErrAlchemyGroupNotFound
	}
	if !alchemyGroupContainsEntry(group, args.SourceItemID) {
		return 0, nil, nil, false, "", ErrAlchemyGroupNotFound
	}
	derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	perRunCosts := map[string]int64{args.SourceItemID: 1}
	if group.GoldCost > 0 {
		perRunCosts[GoldCoinItemID] += group.GoldCost
	}
	actual, cancelAfter, cancelReason, err := e.resolveAlchemyAffordableBatchRunsTx(ctx, tx, userID, perRunCosts, completed, reservedCosts, ActionQueueCancelReasonCostInsufficient)
	if err != nil || actual <= 0 {
		return actual, map[string]int64{}, map[string]int64{}, cancelAfter, cancelReason, err
	}
	totalCosts, err := checkedScaleInt64Map(perRunCosts, int64(actual))
	if err != nil {
		return 0, nil, nil, false, "", err
	}

	rng := alchemyQueueRNG(userID, item.QueueID, item.DoneCount, item.ActionID)
	failChance := e.store.alchemyConvertFailChance(group, derived)
	weights := e.store.adjustAlchemyConvertWeights(args.SourceItemID, group.Entries, derived.LuckyChance, false)
	upgradeWeights := e.store.adjustAlchemyConvertWeights("", group.UpgradeEntries, derived.LuckyChance, true)
	upgradeChance := alchemyConvertUpgradeChance(group, upgradeWeights)
	roll, err := rollAlchemyConvertBatchRewards(actual, failChance, upgradeChance, weights, upgradeWeights, rng)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	batchDetails := map[string]any{
		"groupId":       group.ID,
		"sourceItemId":  args.SourceItemID,
		"failedCount":   roll.FailedCount,
		"successCount":  actual - roll.FailedCount,
		"upgradeCount":  roll.UpgradeCount,
		"upgradeChance": upgradeChance,
		"failChance":    failChance,
	}
	if err := e.chargeAlchemyAdditionalBatchRunsTx(ctx, tx, userID, item, perRunCosts, actual, reservedCosts, batchDetails); err != nil {
		return 0, nil, nil, false, "", err
	}
	if err := e.grantAlchemyBatchRewardsTx(ctx, tx, userID, item, roll.Rewards, "alchemy_convert", "alchemy_convert_reward", actual, batchDetails); err != nil {
		return 0, nil, nil, false, "", err
	}
	_ = e.store.markAlchemyDiscoveredTx(ctx, tx, userID, "group", group.ID)
	for reward := range roll.Rewards {
		e.store.markAlchemyTraitsForItemTx(ctx, tx, userID, reward)
	}
	return actual, totalCosts, roll.Rewards, cancelAfter, cancelReason, nil
}

func (e *alchemyExecutor) settleAlchemyTransmuteBatchTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, completed int, reservedCosts map[string]int64) (int, map[string]int64, map[string]int64, bool, string, error) {
	var args alchemyTransmuteArgs
	if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
		return 0, nil, nil, false, "", err
	}
	rule := e.store.gameData.AlchemyTransmuteRuleForItemID(args.ItemID)
	if rule == nil {
		return 0, nil, nil, false, "", ErrAlchemyArgsInvalid
	}
	def := e.store.gameData.ItemByID(args.ItemID)
	if !e.store.itemAllowedForAlchemyTransmute(def) {
		return 0, nil, nil, false, "", ErrItemNotRecyclable
	}
	derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	perRunCosts := map[string]int64{args.ItemID: 1}
	actual, cancelAfter, cancelReason, err := e.resolveAlchemyAffordableBatchRunsTx(ctx, tx, userID, perRunCosts, completed, reservedCosts, ActionQueueCancelReasonCostInsufficient)
	if err != nil || actual <= 0 {
		return actual, map[string]int64{}, map[string]int64{}, cancelAfter, cancelReason, err
	}
	totalCosts, err := checkedScaleInt64Map(perRunCosts, int64(actual))
	if err != nil {
		return 0, nil, nil, false, "", err
	}

	rng := alchemyQueueRNG(userID, item.QueueID, item.DoneCount, item.ActionID)
	baseFail := e.store.alchemyLevelAdjustedFailChance(rule.FailChanceBase, derived.Level, minAlchemyLevelForTransmute(rule, def), 0)
	failChance := applyAlchemyMinimumLossFailChance(applyAlchemyStabilityToFailChance(baseFail, derived.Stability), rule.MinFailChance)
	roll, err := rollAlchemyTransmuteBatchRewards(actual, failChance, def.EconomyBaseValue, rule.ReturnRatio, derived.Stability, rule.LuckyChanceBase+derived.LuckyChance, derived.LuckyChance, rule.LuckyTiers, rng)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	batchDetails := map[string]any{
		"sourceItemId": args.ItemID,
		"failedCount":  roll.FailedCount,
		"successCount": actual - roll.FailedCount,
		"luckyCount":   roll.LuckyCount,
		"failChance":   failChance,
	}
	if err := e.chargeAlchemyAdditionalBatchRunsTx(ctx, tx, userID, item, perRunCosts, actual, reservedCosts, batchDetails); err != nil {
		return 0, nil, nil, false, "", err
	}
	if err := e.grantAlchemyBatchRewardsTx(ctx, tx, userID, item, roll.Rewards, "alchemy_transmute", "alchemy_transmute_reward", actual, batchDetails); err != nil {
		return 0, nil, nil, false, "", err
	}
	return actual, totalCosts, roll.Rewards, cancelAfter, cancelReason, nil
}

func (e *alchemyExecutor) settleAlchemyBenchBatchTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, completed int, reservedCosts map[string]int64) (int, map[string]int64, map[string]int64, bool, string, error) {
	var args alchemyBenchArgs
	if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
		return 0, nil, nil, false, "", err
	}
	if args.FishInstanceID > 0 {
		return e.settleFishingInfusionTx(ctx, tx, userID, item, args, completed, reservedCosts)
	}
	carrier, err := e.store.LockInstanceTx(ctx, tx, args.CarrierInstanceID)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	if carrier.UserID != userID {
		return 0, nil, nil, false, "", ErrInstanceNotOwned
	}
	if carrier.State != "inventory" {
		return 0, nil, nil, false, "", ErrInvalidInstanceState
	}
	rule := e.store.gameData.AffixCarrierRuleByItemID(carrier.TemplateItem)
	if rule == nil || !rule.Active || rule.MaxCustomAffixes <= 0 {
		return 0, nil, nil, false, "", ErrAffixInvalidCarrier
	}
	if len(decodeAffixSlots(carrier.AffixesRaw)) > 0 {
		return 0, nil, nil, false, "", ErrAlchemyCarrierOccupied
	}
	derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	var medium *catalog.AlchemyMedium
	if args.MediumItemID != "" {
		medium = e.store.gameData.AlchemyMediumByItemID(args.MediumItemID)
		if medium == nil {
			return 0, nil, nil, false, "", ErrAlchemyMediumUnknown
		}
	}
	if len(args.Materials) != catalog.EffectiveBenchSlots(medium) {
		return 0, nil, nil, false, "", ErrAlchemyTraitSlotOverflow
	}
	modifiers, err := alchemyOperationEffectModifiers(item)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	preflightCraft, err := e.store.resolveAlchemyCraftWithModifiers(args.Materials, args.ComboRecipeID, medium, derived, modifiers, rand.New(rand.NewSource(1)))
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	if rule.AllowedTraitKind != "" && rule.AllowedTraitKind != "both" && preflightCraft.Kind != rule.AllowedTraitKind {
		return 0, nil, nil, false, "", ErrAlchemyKindMismatch
	}
	if carrier.Quantity <= 0 {
		return 0, nil, nil, false, "", ErrInvalidInstanceState
	}
	maxByCarrier := completed
	if carrier.Quantity < int64(maxByCarrier) {
		maxByCarrier = int(carrier.Quantity)
	}
	perRunCosts := e.store.alchemyBenchPerRunCosts(args.Materials, args.ComboRecipeID, medium)
	actual, cancelAfter, cancelReason, err := e.resolveAlchemyAffordableBatchRunsTx(ctx, tx, userID, perRunCosts, maxByCarrier, reservedCosts, ActionQueueCancelReasonCostInsufficient)
	if err != nil || actual <= 0 {
		return actual, map[string]int64{}, map[string]int64{}, cancelAfter, cancelReason, err
	}
	totalCosts, err := checkedScaleInt64Map(perRunCosts, int64(actual))
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	if actual < completed && actual == maxByCarrier {
		cancelAfter = true
		cancelReason = ActionQueueCancelReasonTargetInvalid
	}

	rng := alchemyQueueRNG(userID, item.QueueID, item.DoneCount, item.ActionID)
	slots := make([]EquipmentAffixSlot, 0, actual)
	failedCount := 0
	for i := 0; i < actual; i++ {
		craft, failed, err := e.store.resolveAlchemyBenchCraftOutcomeWithModifiers(args.Materials, args.ComboRecipeID, medium, derived, modifiers, rng)
		if err != nil {
			return 0, nil, nil, false, "", err
		}
		if err := stampAlchemyCraftOperationEffects(&craft, alchemyOperationEffectSnapshot(item)); err != nil {
			return 0, nil, nil, false, "", err
		}
		if rule.AllowedTraitKind != "" && rule.AllowedTraitKind != "both" && craft.Kind != rule.AllowedTraitKind {
			return 0, nil, nil, false, "", ErrAlchemyKindMismatch
		}
		if failed {
			failedCount++
		}
		slots = append(slots, alchemyCraftSlot(craft))
		for _, traitID := range craft.TraitIDs {
			_ = e.store.markAlchemyDiscoveredTx(ctx, tx, userID, "trait", traitID)
		}
		if craft.ComboID != "" {
			_ = e.store.markAlchemyDiscoveredTx(ctx, tx, userID, "combo", craft.ComboID)
		}
	}
	batchDetails := map[string]any{
		"carrierInstanceId": args.CarrierInstanceID,
		"failedCount":       failedCount,
		"successCount":      actual - failedCount,
		"comboRecipeId":     args.ComboRecipeID,
		"mediumItemId":      args.MediumItemID,
	}
	if err := e.chargeAlchemyAdditionalBatchRunsTx(ctx, tx, userID, item, perRunCosts, actual, reservedCosts, batchDetails); err != nil {
		return 0, nil, nil, false, "", err
	}
	targetIDs, err := e.store.applyAlchemyBenchBatchSlotsTx(ctx, tx, userID, carrier, slots)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	resultMeta := alchemyBatchLedgerMeta(item, "alchemy_bench", "inventory", InventoryAuditDirectionGain, "alchemy_bench_result", actual, map[string]any{
		"carrierInstanceId":        args.CarrierInstanceID,
		"targetCarrierInstanceIds": targetIDs,
		"failedCount":              failedCount,
		"successCount":             actual - failedCount,
		"comboRecipeId":            args.ComboRecipeID,
		"mediumItemId":             args.MediumItemID,
	})
	e.store.recordInventoryAuditTx(tx, InventoryAuditEvent{
		UserID:        userID,
		Category:      resultMeta.Category,
		Direction:     resultMeta.Direction,
		SubjectType:   resultMeta.SubjectType,
		SubjectID:     resultMeta.SubjectID,
		Items:         map[string]int64{carrier.TemplateItem: int64(actual)},
		InstanceIDs:   targetIDs,
		SourceSurface: resultMeta.SourceSurface,
		TargetSurface: resultMeta.TargetSurface,
		Details:       resultMeta.Details,
	})
	return actual, totalCosts, map[string]int64{}, cancelAfter, cancelReason, nil
}

func (e *alchemyExecutor) resolveAlchemyAffordableBatchRunsTx(ctx context.Context, tx pgx.Tx, userID int64, perRunCosts map[string]int64, wanted int, reservedCosts map[string]int64, cancelReason string) (int, bool, string, error) {
	if wanted <= 0 {
		return 0, false, "", nil
	}
	prepaidRuns := alchemyReservedRunCount(reservedCosts, perRunCosts)
	additionalWanted := int64(wanted) - prepaidRuns
	if additionalWanted < 0 {
		additionalWanted = 0
	}
	affordable := additionalWanted
	if additionalWanted > 0 {
		var err error
		affordable, err = e.store.maxAffordableActionStartsTx(ctx, tx, userID, perRunCosts, additionalWanted)
		if err != nil {
			return 0, false, "", err
		}
	}
	actual64 := prepaidRuns + affordable
	if actual64 > int64(wanted) {
		actual64 = int64(wanted)
	}
	if actual64 <= 0 {
		return 0, true, cancelReason, nil
	}
	cancelAfter := actual64 < int64(wanted)
	reason := ""
	if cancelAfter {
		reason = cancelReason
	}
	return int(actual64), cancelAfter, reason, nil
}

func (e *alchemyExecutor) chargeAlchemyAdditionalBatchRunsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, perRunCosts map[string]int64, actual int, reservedCosts map[string]int64, extra map[string]any) error {
	if actual <= 0 {
		return nil
	}
	prepaidRuns := alchemyReservedRunCount(reservedCosts, perRunCosts)
	additionalToCharge := int64(actual) - prepaidRuns
	if additionalToCharge <= 0 {
		return nil
	}
	details := map[string]any{
		"chargedCount": additionalToCharge,
		"settledCount": actual,
	}
	for key, value := range extra {
		details[key] = value
	}
	return e.store.applyActionCostsBatchTx(ctx, tx, userID, perRunCosts, additionalToCharge, alchemyBatchLedgerMeta(item, "inventory", "alchemy_"+alchemyBatchActionName(item.ActionID), InventoryAuditDirectionConsume, "alchemy_"+alchemyBatchActionName(item.ActionID)+"_batch_cost", actual, details))
}

func (e *alchemyExecutor) grantAlchemyBatchRewardsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, rewards map[string]int64, source string, eventKind string, count int, extra map[string]any) error {
	if len(rewards) == 0 {
		return nil
	}
	meta := alchemyBatchLedgerMeta(item, source, "inventory", InventoryAuditDirectionGain, eventKind, count, extra)
	for rewardItemID, qty := range rewards {
		if qty <= 0 {
			continue
		}
		if err := e.store.LedgerGrantTemplateTx(ctx, tx, userID, rewardItemID, qty, meta); err != nil {
			return err
		}
	}
	return nil
}

func alchemyQueueRNG(userID int64, queueID int64, doneBefore int, actionID string) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte("alchemy-queue"))
	writeI64(h, userID)
	writeI64(h, queueID)
	writeI64(h, int64(doneBefore))
	_, _ = h.Write([]byte(strings.TrimSpace(actionID)))
	seed := int64(h.Sum64())
	if seed == 0 {
		seed = 1
	}
	return rand.New(rand.NewSource(seed))
}

func alchemyReservedRunCount(reservedCosts map[string]int64, perRunCosts map[string]int64) int64 {
	if len(reservedCosts) == 0 || len(perRunCosts) == 0 {
		return 0
	}
	for itemID, qty := range perRunCosts {
		if qty > 0 && reservedCosts[itemID] < qty {
			return 0
		}
	}
	return 1
}

func checkedScaleInt64Map(values map[string]int64, multiplier int64) (map[string]int64, error) {
	out := map[string]int64{}
	if multiplier <= 0 {
		return out, nil
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	for itemID, qty := range values {
		if itemID == "" || qty <= 0 {
			continue
		}
		if qty > maxInt64/multiplier {
			return nil, ErrInventoryQuantityOverflow
		}
		scaled := qty * multiplier
		if out[itemID] > maxInt64-scaled {
			return nil, ErrInventoryQuantityOverflow
		}
		out[itemID] += scaled
	}
	return out, nil
}

func alchemyBatchLedgerMeta(item QueueRuntimeItem, source, target string, direction InventoryAuditDirection, eventKind string, count int, extra map[string]any) InventoryLedgerMeta {
	if count <= 0 {
		count = 1
	}
	runStart := item.DoneCount + 1
	if runStart <= 0 {
		runStart = 1
	}
	runEnd := item.DoneCount + count
	if runEnd < runStart {
		runEnd = runStart
	}
	runID := fmt.Sprintf("%d:%d", item.QueueID, runStart)
	details := map[string]any{
		"type":       eventKind,
		"queueId":    item.QueueID,
		"actionId":   item.ActionID,
		"phase":      item.AuditPhase,
		"count":      count,
		"runNo":      runStart,
		"runNoStart": runStart,
		"runNoEnd":   runEnd,
		"runId":      runID,
	}
	for key, value := range extra {
		details[key] = value
	}
	return InventoryLedgerMeta{
		Category:      InventoryAuditCategoryAlchemy,
		Direction:     direction,
		SubjectType:   "alchemy",
		SubjectID:     strconv.FormatInt(item.QueueID, 10),
		SourceSurface: source,
		TargetSurface: target,
		Details:       details,
	}
}

func alchemyBatchActionName(actionID string) string {
	switch strings.TrimSpace(actionID) {
	case "alchemy.convert":
		return "convert"
	case "alchemy.bench":
		return "bench"
	case "alchemy.transmute":
		return "transmute"
	case "alchemy.inherit":
		return "inherit"
	default:
		return "unknown"
	}
}

func shouldYieldAlchemyFastForwardBudget(startedAt time.Time, budget FastForwardBudget, completed int) bool {
	if completed <= 0 || budget.MaxExec <= 0 || startedAt.IsZero() {
		return false
	}
	return time.Since(startedAt) >= budget.MaxExec
}

func (e *alchemyExecutor) refreshNextRuntimePayloadTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, payloadRaw []byte, starts ...time.Time) (json.RawMessage, time.Duration, error) {
	runCtxArgs, err := parseAlchemyRunContextArgs(e.actionID, payloadRaw)
	if err != nil {
		return nil, 0, err
	}
	baseDuration := e.store.resolveAlchemyBaseDuration(item.Spec, runCtxArgs)
	var at time.Time
	if len(starts) > 0 {
		at = starts[0]
	}
	resolvedDuration, extraAttrExp, err := e.resolveSettlementWorkDurationTx(ctx, tx, userID, item, baseDuration, at)
	if err != nil {
		return nil, 0, err
	}
	extraAttrExp = mergeAttrExp(extraAttrExp, e.store.resolveAlchemyExtraExp(item.Spec, runCtxArgs))
	if resolvedDuration <= 0 {
		resolvedDuration = minimumActionQueueRunDuration
	}
	nextPayload, err := setWorkQueueRuntime(item, payloadRaw, alchemyRuntime{
		BaseDurationMs: int64(baseDuration / time.Millisecond),
		DurationMs:     int64(resolvedDuration / time.Millisecond),
		ExtraAttrExp:   extraAttrExp,
	}, at)
	if err != nil {
		return nil, 0, err
	}
	return nextPayload, resolvedDuration, nil
}

func (e *alchemyExecutor) clearReservedPayload(payloadRaw []byte) json.RawMessage {
	var args any
	switch e.actionID {
	case "alchemy.convert":
		var a alchemyConvertArgs
		if err := parseAlchemyRunContext(payloadRaw, &a); err == nil {
			args = a
		}
	case "alchemy.bench":
		var a alchemyBenchArgs
		if err := parseAlchemyRunContext(payloadRaw, &a); err == nil {
			args = a
		}
	case "alchemy.transmute":
		var a alchemyTransmuteArgs
		if err := parseAlchemyRunContext(payloadRaw, &a); err == nil {
			args = a
		}
	case "alchemy.inherit":
		var a alchemyInheritArgs
		if err := parseAlchemyRunContext(payloadRaw, &a); err == nil {
			args = a
		}
	}
	if args == nil {
		return payloadRaw
	}
	next, err := clearAlchemyReservedCosts(payloadRaw, args)
	if err != nil {
		return payloadRaw
	}
	return next
}

func classifyAlchemyStopError(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrInsufficientItem):
		return ActionQueueCancelReasonCostInsufficient, true
	case errors.Is(err, ErrInvalidInstanceState), errors.Is(err, ErrInstanceNotOwned):
		return ActionQueueCancelReasonTargetInvalid, true
	case errors.Is(err, ErrAlchemyCarrierFull),
		errors.Is(err, ErrAlchemyCarrierOccupied),
		errors.Is(err, ErrAlchemyKindMismatch),
		errors.Is(err, ErrAlchemyMixedKind),
		errors.Is(err, ErrAlchemyMaterialOverflow),
		errors.Is(err, ErrAlchemyBasisInvalid),
		errors.Is(err, ErrAlchemyGroupNotFound),
		errors.Is(err, ErrAlchemyArgsInvalid),
		errors.Is(err, ErrAlchemyMediumUnknown),
		errors.Is(err, ErrAlchemyTraitSlotOverflow),
		errors.Is(err, ErrAffixInvalidCarrier),
		errors.Is(err, ErrAffixRarityNotAllowed),
		errors.Is(err, ErrAffixSlotNotFound),
		errors.Is(err, ErrItemNotRecyclable):
		return ActionQueueCancelReasonTargetInvalid, true
	case errors.Is(err, ErrAlchemyLevelInsufficient):
		return ActionQueueCancelReasonStatusRequirement, true
	}
	return "", false
}

func (e *alchemyExecutor) settleOnceTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, rng *rand.Rand, prepaid bool) (map[string]int64, map[string]int64, error) {
	switch e.actionID {
	case "alchemy.convert":
		var args alchemyConvertArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, nil, err
		}
		result, err := e.store.alchemyGroupConvertOnceTx(ctx, tx, userID, args.SourceItemID, args.GroupID, rng, item, prepaid)
		if err != nil {
			return nil, nil, err
		}
		return result.Costs, result.Rewards, nil
	case "alchemy.bench":
		var args alchemyBenchArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, nil, err
		}
		result, err := e.store.alchemyBenchCraftTx(ctx, tx, userID, args.CarrierInstanceID, args.Materials, args.ComboRecipeID, args.MediumItemID, rng, item, prepaid)
		return result.Costs, result.Rewards, err
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, nil, err
		}
		rule := e.store.gameData.AlchemyTransmuteRuleForItemID(args.ItemID)
		result, err := e.store.alchemyTransmuteOnceTx(ctx, tx, userID, args.ItemID, args.InstanceID, rule, rng, item, prepaid)
		return result.Costs, result.Rewards, err
	case "alchemy.inherit":
		var args alchemyInheritArgs
		if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
			return nil, nil, err
		}
		result, err := e.store.alchemyInheritSettleTx(ctx, tx, userID, args, rng, item, prepaid)
		return result.Costs, result.Rewards, err
	default:
		return nil, nil, ErrAlchemyArgsInvalid
	}
}

func (e *alchemyExecutor) ClassifyStartError(err error) (bool, string) {
	if errors.Is(err, ErrInsufficientItem) {
		return true, ActionQueueCancelReasonCostInsufficient
	}
	if errors.Is(err, ErrStatusRequireNotMet) || errors.Is(err, ErrAlchemyLevelInsufficient) {
		return true, ActionQueueCancelReasonStatusRequirement
	}
	if errors.Is(err, ErrAlchemyMixedKind) || errors.Is(err, ErrAlchemyKindMismatch) || errors.Is(err, ErrAlchemyCarrierFull) || errors.Is(err, ErrAlchemyCarrierOccupied) || errors.Is(err, ErrAlchemyMaterialOverflow) || errors.Is(err, ErrAlchemyGroupNotFound) || errors.Is(err, ErrAlchemyBasisInvalid) || errors.Is(err, ErrAlchemyArgsInvalid) || errors.Is(err, ErrAlchemyMediumUnknown) || errors.Is(err, ErrAlchemyTraitSlotOverflow) || errors.Is(err, ErrAffixInvalidCarrier) || errors.Is(err, ErrAffixRarityNotAllowed) || errors.Is(err, ErrAffixSlotNotFound) || errors.Is(err, ErrInvalidInstanceState) || errors.Is(err, ErrInstanceNotOwned) || errors.Is(err, pgx.ErrNoRows) {
		return true, ActionQueueCancelReasonTargetInvalid
	}
	return false, ""
}

type parsedAlchemyRuntime struct {
	BaseDurationMs int64
	DurationMs     int64
	ExtraAttrExp   map[string]float64
}

func (r parsedAlchemyRuntime) Duration() time.Duration {
	if r.DurationMs <= 0 {
		return 0
	}
	return normalizeActionQueueRunDuration(time.Duration(r.DurationMs) * time.Millisecond)
}

func parseAlchemyRuntime(raw []byte) parsedAlchemyRuntime {
	var env queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return parsedAlchemyRuntime{ExtraAttrExp: map[string]float64{}}
	}
	var runtime alchemyRuntime
	_ = json.Unmarshal(env.Runtime, &runtime)
	return parsedAlchemyRuntime{BaseDurationMs: runtime.BaseDurationMs, DurationMs: runtime.DurationMs, ExtraAttrExp: runtime.ExtraAttrExp}
}

func parseAlchemyRunContext(raw []byte, target any) error {
	var env queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	var wrapped alchemyRunContextEnvelope
	if err := json.Unmarshal(env.RunContext, &wrapped); err == nil && len(wrapped.Args) > 0 {
		return json.Unmarshal(wrapped.Args, target)
	}
	return json.Unmarshal(env.RunContext, target)
}

func parseAlchemyRunContextArgs(actionID string, payloadRaw []byte) (any, error) {
	switch strings.TrimSpace(actionID) {
	case "alchemy.convert":
		var args alchemyConvertArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err != nil {
			return nil, err
		}
		args.SourceItemID = strings.TrimSpace(args.SourceItemID)
		args.GroupID = strings.TrimSpace(args.GroupID)
		args.SourceCount = 1
		if args.SourceItemID == "" {
			return nil, ErrAlchemyArgsInvalid
		}
		return args, nil
	case "alchemy.bench":
		var args alchemyBenchArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err != nil {
			return nil, err
		}
		args.CarrierItemID = strings.TrimSpace(args.CarrierItemID)
		args.ComboRecipeID = strings.TrimSpace(args.ComboRecipeID)
		args.MediumItemID = strings.TrimSpace(args.MediumItemID)
		materials := map[string]int64{}
		for itemID, qty := range args.Materials {
			itemID = strings.TrimSpace(itemID)
			if itemID != "" && qty > 0 {
				materials[itemID] += qty
			}
		}
		args.Materials = materials
		if args.FishInstanceID > 0 {
			if args.CarrierInstanceID <= 0 || len(args.Materials) != 0 || args.ComboRecipeID != "" {
				return nil, ErrAlchemyArgsInvalid
			}
			args.CarrierItemID = fishingInfusionCarrierItemID
			args.MediumItemID = fishingInfusionMediumItemID
			return args, nil
		}
		if args.CarrierInstanceID <= 0 || len(args.Materials) == 0 {
			return nil, ErrAlchemyArgsInvalid
		}
		return args, nil
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err != nil {
			return nil, err
		}
		args.ItemID = strings.TrimSpace(args.ItemID)
		args.TotalCount = 1
		if args.ItemID == "" {
			return nil, ErrAlchemyArgsInvalid
		}
		return args, nil
	case "alchemy.inherit":
		var args alchemyInheritArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err != nil {
			return nil, err
		}
		args.MediumItemID = strings.TrimSpace(args.MediumItemID)
		choices, _, _, err := normalizeInheritGeneChoices(args.GeneChoices)
		if err != nil {
			return nil, err
		}
		args.GeneChoices = choices
		if args.FatherInstanceID <= 0 || args.MotherInstanceID <= 0 || args.CarrierInstanceID <= 0 {
			return nil, ErrAlchemyArgsInvalid
		}
		if args.FatherInstanceID == args.MotherInstanceID ||
			args.FatherInstanceID == args.CarrierInstanceID ||
			args.MotherInstanceID == args.CarrierInstanceID {
			return nil, ErrAlchemyArgsInvalid
		}
		return args, nil
	default:
		return nil, ErrAlchemyArgsInvalid
	}
}

func marshalAlchemyRunContext(args any, reserved map[string]int64) (json.RawMessage, error) {
	argsRaw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	clean := map[string]int64{}
	for itemID, qty := range reserved {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" && qty > 0 {
			clean[itemID] += qty
		}
	}
	return json.Marshal(alchemyRunContextEnvelope{Args: argsRaw, ReservedTemplateCosts: clean})
}

func parseAlchemyReservedCosts(raw []byte) map[string]int64 {
	var env queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	var wrapped alchemyRunContextEnvelope
	if err := json.Unmarshal(env.RunContext, &wrapped); err != nil || len(wrapped.ReservedTemplateCosts) == 0 {
		return nil
	}
	out := map[string]int64{}
	for itemID, qty := range wrapped.ReservedTemplateCosts {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" && qty > 0 {
			out[itemID] += qty
		}
	}
	return out
}

func setAlchemyReservedCosts(payloadRaw []byte, args any, reserved map[string]int64) (json.RawMessage, error) {
	var env queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &env); err != nil {
		return nil, err
	}
	runCtxRaw, err := marshalAlchemyRunContext(args, reserved)
	if err != nil {
		return nil, err
	}
	env.RunContext = runCtxRaw
	return json.Marshal(env)
}

func clearAlchemyReservedCosts(payloadRaw []byte, args any) (json.RawMessage, error) {
	return setAlchemyReservedCosts(payloadRaw, args, nil)
}

func (s *Store) refundAlchemyReservedCostsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, reserved map[string]int64) error {
	for itemID, qty := range reserved {
		if qty <= 0 {
			continue
		}
		if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, qty, alchemyLedgerMeta(item, "action_queue", "inventory", InventoryAuditDirectionGain, "alchemy_cancel_refund", map[string]any{"itemId": itemID})); err != nil {
			return err
		}
	}
	return nil
}

func alchemyLedgerMeta(item QueueRuntimeItem, source, target string, direction InventoryAuditDirection, eventKind string, extra map[string]any) InventoryLedgerMeta {
	runNo := item.DoneCount + 1
	if runNo <= 0 {
		runNo = 1
	}
	details := map[string]any{
		"type":     eventKind,
		"queueId":  item.QueueID,
		"actionId": item.ActionID,
		"phase":    item.AuditPhase,
		"runNo":    runNo,
		"runId":    fmt.Sprintf("%d:%d", item.QueueID, runNo),
	}
	for k, v := range extra {
		details[k] = v
	}
	return InventoryLedgerMeta{
		Category:      InventoryAuditCategoryAlchemy,
		Direction:     direction,
		SubjectType:   "alchemy",
		SubjectID:     strconv.FormatInt(item.QueueID, 10),
		SourceSurface: source,
		TargetSurface: target,
		Details:       details,
	}
}

func (s *Store) resolveAlchemyBaseDuration(spec ActionSpec, args any) time.Duration {
	seconds := s.config.Alchemy.DurationForAction(spec.ID)
	switch spec.ID {
	case "alchemy.convert":
		if a, ok := args.(alchemyConvertArgs); ok {
			if group := s.gameData.AlchemyGroupByID(a.GroupID); group != nil && group.DurationSec > 0 {
				return time.Duration(math.Ceil(group.DurationSec*1000)) * time.Millisecond
			}
			if group := s.gameData.AlchemyGroupByItemID(a.SourceItemID); group != nil && group.DurationSec > 0 {
				return time.Duration(math.Ceil(group.DurationSec*1000)) * time.Millisecond
			}
		}
	case "alchemy.bench":
		if a, ok := args.(alchemyBenchArgs); ok {
			total := float64(s.config.Alchemy.InfusionBaseDurationSec)
			if total <= 0 {
				total = 16
			}
			for itemID, qty := range a.Materials {
				if qty <= 0 {
					continue
				}
				if def := s.gameData.ItemTraitByItemID(itemID); def != nil {
					for _, trait := range def.Traits {
						total += trait.DurationSecPerUnit * float64(qty)
					}
				}
			}
			if match := s.matchAlchemyCombo(a.Materials, a.ComboRecipeID); match != nil && match.Recipe != nil && match.Recipe.ExtraDurationSec > 0 {
				total += match.Recipe.ExtraDurationSec * float64(maxInt64(1, match.Multiplier))
			}
			return time.Duration(math.Ceil(total*1000)) * time.Millisecond
		}
	case "alchemy.transmute":
		if a, ok := args.(alchemyTransmuteArgs); ok && strings.TrimSpace(a.ItemID) != "" {
			if item := s.gameData.ItemByID(a.ItemID); item != nil {
				if rule := s.gameData.AlchemyTransmuteRuleForItemID(a.ItemID); rule != nil {
					return time.Duration(math.Ceil(s.alchemyTransmuteDurationSeconds(rule, item, spec)*1000)) * time.Millisecond
				}
				if d := s.config.Alchemy.TransmuteDurationByRarity(int(s.rarityOrder(item.Rarity))); d > 0 {
					seconds = d
				}
			}
		}
	case "alchemy.inherit":
		total := float64(spec.DurationS)
		if total <= 0 {
			total = 60
		}
		return time.Duration(math.Ceil(total*1000)) * time.Millisecond
	}
	if seconds <= 0 {
		seconds = int(spec.DurationS)
	}
	if seconds <= 0 {
		seconds = 12
	}
	return time.Duration(seconds) * time.Second
}

func (s *Store) alchemyTransmuteDurationSeconds(rule *catalog.AlchemyTransmuteRule, item *catalog.Item, spec ActionSpec) float64 {
	if item == nil {
		return 0
	}
	base := rule.DurationSec
	if base <= 0 {
		if d := s.config.Alchemy.TransmuteDurationByRarity(int(s.rarityOrder(item.Rarity))); d > 0 {
			base = float64(d)
		}
	}
	if base <= 0 {
		base = float64(spec.DurationS)
	}
	if base <= 0 {
		base = 12
	}
	rarityAdd := 0.0
	rarityIdx := int(s.rarityOrder(item.Rarity))
	if len(rule.RarityDurationAddSecByRarityIndex) > 0 {
		if rarityIdx < 0 {
			rarityIdx = 0
		}
		if rarityIdx >= len(rule.RarityDurationAddSecByRarityIndex) {
			rarityIdx = len(rule.RarityDurationAddSecByRarityIndex) - 1
		}
		rarityAdd = rule.RarityDurationAddSecByRarityIndex[rarityIdx]
	}
	valueScale := 1 + math.Min(0.9, math.Sqrt(math.Max(0, item.EconomyBaseValue))/180)
	return math.Max(1, (base+rarityAdd)*valueScale)
}

func (s *Store) resolveAlchemyExtraExp(spec ActionSpec, args any) map[string]float64 {
	exp := 0.0
	switch spec.ID {
	case "alchemy.convert":
		if a, ok := args.(alchemyConvertArgs); ok {
			if group := s.gameData.AlchemyGroupByID(a.GroupID); group != nil {
				exp = group.AlchemyExp
			} else if group := s.gameData.AlchemyGroupByItemID(a.SourceItemID); group != nil {
				exp = group.AlchemyExp
			}
		}
	case "alchemy.bench":
		if a, ok := args.(alchemyBenchArgs); ok {
			exp = s.config.Alchemy.InfusionBaseExp
			if exp <= 0 {
				exp = 2
			}
			for itemID, qty := range a.Materials {
				if qty <= 0 {
					continue
				}
				if def := s.gameData.ItemTraitByItemID(itemID); def != nil {
					for _, trait := range def.Traits {
						exp += trait.AlchemyExpPerUnit * float64(qty)
					}
				}
			}
		}
	case "alchemy.transmute":
		if a, ok := args.(alchemyTransmuteArgs); ok {
			if rule := s.gameData.AlchemyTransmuteRuleForItemID(a.ItemID); rule != nil {
				exp = rule.AlchemyExp
			}
		}
	}
	if exp <= 0 || math.IsNaN(exp) || math.IsInf(exp, 0) {
		return nil
	}
	return map[string]float64{"alchemy_level": roundAlchemyValue(exp, 4)}
}

func sampleLuckSkewed(min, max, stability float64, rng *rand.Rand) float64 {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if max < min {
		min, max = max, min
	}
	if max == min {
		return min
	}
	if stability < 0 || math.IsNaN(stability) || math.IsInf(stability, 0) {
		stability = 0
	}
	u := rng.Float64()
	t := math.Pow(u, 1.0/(1.0+stability))
	return min + (max-min)*t
}

type alchemyDerived struct {
	Level       float64
	Precision   float64
	LuckyChance float64
	Stability   float64
}

func (s *Store) resolveAlchemyDerivedForItemTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (alchemyDerived, error) {
	totals := map[string]float64{}
	if item.WorkLoadoutRuntime != nil && item.WorkLoadoutRuntime.Available {
		// Read only the four derived inputs; the frozen loadout remains immutable.
		totals = item.WorkLoadoutRuntime.AttributeTotals
	} else {
		if item.alchemyReads != nil && item.alchemyReads.derived != nil {
			return *item.alchemyReads.derived, nil
		}
		resolved, _, err := s.computePlayerAttributeTotalsTx(ctx, tx, userID, nil)
		if err != nil {
			return alchemyDerived{}, err
		}
		totals = resolved
	}
	level := math.Max(1, totals["alchemy_level"])
	derived := alchemyDerived{
		Level:       level,
		Precision:   cleanAlchemyFloat(totals["alchemy_precision"]),
		LuckyChance: clampAlchemyFloat(totals["alchemy_lucky_chance"], 0, 0.20),
		Stability:   cleanAlchemyFloat(totals["alchemy_craft_success"]),
	}
	if item.alchemyReads != nil && (item.WorkLoadoutRuntime == nil || !item.WorkLoadoutRuntime.Available) {
		item.alchemyReads.derived = &derived
	}
	return derived, nil
}

func (s *Store) alchemyLevelDeficitFailDelta(playerLevel float64, minLevel int64) float64 {
	deficit := float64(minLevel) - playerLevel
	if deficit <= 0 {
		return 0
	}
	perLevel := s.config.Alchemy.LevelDeficitFailChancePerLevel
	if perLevel <= 0 || math.IsNaN(perLevel) || math.IsInf(perLevel, 0) {
		perLevel = 0.06
	}
	capValue := s.config.Alchemy.LevelDeficitFailChanceCap
	if capValue <= 0 || math.IsNaN(capValue) || math.IsInf(capValue, 0) {
		capValue = 0.85
	}
	return clampAlchemyFloat(deficit*perLevel, 0, capValue)
}

func (s *Store) alchemyLevelAdjustedFailChance(baseFail, playerLevel float64, minLevel int64, failChancePerLevelAbove float64) float64 {
	if playerLevel >= float64(minLevel) {
		return clampAlchemyFloat(baseFail-(playerLevel-float64(minLevel))*failChancePerLevelAbove, 0, 1)
	}
	return clampAlchemyFloat(baseFail+s.alchemyLevelDeficitFailDelta(playerLevel, minLevel), 0, 1)
}

func minAlchemyLevelForRarity(rarity string) int64 {
	switch strings.TrimSpace(rarity) {
	case "uncommon":
		return 8
	case "rare":
		return 15
	case "epic":
		return 20
	case "legendary":
		return 25
	case "mythic":
		return 35
	case "common":
		return 1
	default:
		return 1
	}
}

func minAlchemyLevelForTransmute(rule *catalog.AlchemyTransmuteRule, item *catalog.Item) int64 {
	required := int64(1)
	if rule != nil && rule.MinAlchemyLevel > required {
		required = rule.MinAlchemyLevel
	}
	if item != nil {
		if rarityRequired := minAlchemyLevelForRarity(item.Rarity); rarityRequired > required {
			required = rarityRequired
		}
	}
	return required
}

func cleanAlchemyFloat(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func clampAlchemyFloat(value, min, max float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

type alchemyConvertResult struct {
	Costs   map[string]int64
	Rewards map[string]int64
}

type alchemyConvertBatchRoll struct {
	Rewards      map[string]int64
	FailedCount  int
	UpgradeCount int
}

type alchemyTransmuteBatchRoll struct {
	Rewards     map[string]int64
	FailedCount int
	LuckyCount  int
}

const (
	exactBinomialSampleLimit       = 4096
	exactTransmuteBatchSampleLimit = 128
	alchemyTransmuteRewardBuckets  = 64
)

func rollAlchemyConvertBatchRewards(actual int, failChance float64, upgradeChance float64, weights map[string]float64, upgradeWeights map[string]float64, rng *rand.Rand) (alchemyConvertBatchRoll, error) {
	if actual <= 0 {
		return alchemyConvertBatchRoll{Rewards: map[string]int64{}}, nil
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	failed := sampleAlchemyBinomial(int64(actual), failChance, rng)
	if failed > int64(actual) {
		failed = int64(actual)
	}
	success := int64(actual) - failed
	upgradeCount := int64(0)
	rewards := map[string]int64{}
	if success > 0 && len(upgradeWeights) > 0 && upgradeChance > 0 {
		upgradeCount = sampleAlchemyBinomial(success, upgradeChance, rng)
		upgradeRewards, err := sampleAlchemyWeightedCounts(upgradeWeights, upgradeCount, rng)
		if err != nil {
			return alchemyConvertBatchRoll{}, err
		}
		mergeInt64Map(rewards, upgradeRewards)
	}
	normalCount := success - upgradeCount
	if normalCount > 0 {
		normalRewards, err := sampleAlchemyWeightedCounts(weights, normalCount, rng)
		if err != nil {
			return alchemyConvertBatchRoll{}, err
		}
		mergeInt64Map(rewards, normalRewards)
	}
	return alchemyConvertBatchRoll{
		Rewards:      rewards,
		FailedCount:  int(failed),
		UpgradeCount: int(upgradeCount),
	}, nil
}

func rollAlchemyTransmuteBatchRewards(actual int, failChance float64, economyBaseValue float64, returnRatio catalog.AlchemyReturnRatio, stability float64, luckyTriggerChance float64, luckyBonusChance float64, luckyTiers []catalog.AlchemyTransmuteTier, rng *rand.Rand) (alchemyTransmuteBatchRoll, error) {
	if actual <= 0 {
		return alchemyTransmuteBatchRoll{Rewards: map[string]int64{}}, nil
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if actual < exactTransmuteBatchSampleLimit {
		return rollAlchemyTransmuteBatchRewardsExact(actual, failChance, economyBaseValue, returnRatio, stability, luckyTriggerChance, luckyBonusChance, luckyTiers, rng)
	}

	failed := sampleAlchemyBinomial(int64(actual), failChance, rng)
	if failed > int64(actual) {
		failed = int64(actual)
	}
	success := int64(actual) - failed
	rewards := map[string]int64{}
	if success <= 0 {
		return alchemyTransmuteBatchRoll{Rewards: rewards, FailedCount: int(failed)}, nil
	}

	ratioBuckets := alchemyLuckSkewedBuckets(returnRatio.MinRatio, returnRatio.MaxRatio, stability, alchemyTransmuteRewardBuckets)
	ratioCounts := sampleAlchemyProbabilityCounts(success, ratioBuckets.Probabilities, rng)
	tierWeights, tiersByID := alchemyTransmuteTierWeights(luckyTiers, luckyBonusChance)
	luckyTriggerChance = clampAlchemyFloat(luckyTriggerChance, 0, 1)
	luckyCount := int64(0)
	for idx, count := range ratioCounts {
		if count <= 0 {
			continue
		}
		gold, err := alchemyGoldFromRatio(economyBaseValue, ratioBuckets.Values[idx])
		if err != nil {
			return alchemyTransmuteBatchRoll{}, err
		}
		luckyAttempts := int64(0)
		if luckyTriggerChance > 0 && len(tierWeights) > 0 {
			luckyAttempts = sampleAlchemyBinomial(count, luckyTriggerChance, rng)
		}
		normalCount := count - luckyAttempts
		if normalCount > 0 {
			total, err := checkedMulInt64(gold, normalCount)
			if err != nil {
				return alchemyTransmuteBatchRoll{}, err
			}
			if err := addAlchemyRewardQuantity(rewards, GoldCoinItemID, total); err != nil {
				return alchemyTransmuteBatchRoll{}, err
			}
		}
		if luckyAttempts <= 0 {
			continue
		}
		tierCounts, err := sampleAlchemyWeightedCounts(tierWeights, luckyAttempts, rng)
		if err != nil {
			return alchemyTransmuteBatchRoll{}, err
		}
		for tierID, tierCount := range tierCounts {
			if tierCount <= 0 {
				continue
			}
			tier := tiersByID[tierID]
			nextGold := gold
			if tier.Kind == "gold_multiplier" {
				multiplier := averageLuckSkewed(1, math.Max(1, tier.Value), luckyBonusChance*2)
				sampled, err := alchemyGoldAfterMultiplier(gold, multiplier)
				if err != nil {
					return alchemyTransmuteBatchRoll{}, err
				}
				if sampled > gold {
					nextGold = sampled
					luckyCount += tierCount
				}
			}
			total, err := checkedMulInt64(nextGold, tierCount)
			if err != nil {
				return alchemyTransmuteBatchRoll{}, err
			}
			if err := addAlchemyRewardQuantity(rewards, GoldCoinItemID, total); err != nil {
				return alchemyTransmuteBatchRoll{}, err
			}
		}
	}
	return alchemyTransmuteBatchRoll{
		Rewards:     rewards,
		FailedCount: int(failed),
		LuckyCount:  int(luckyCount),
	}, nil
}

func rollAlchemyTransmuteBatchRewardsExact(actual int, failChance float64, economyBaseValue float64, returnRatio catalog.AlchemyReturnRatio, stability float64, luckyTriggerChance float64, luckyBonusChance float64, luckyTiers []catalog.AlchemyTransmuteTier, rng *rand.Rand) (alchemyTransmuteBatchRoll, error) {
	rewards := map[string]int64{}
	failedCount := 0
	luckyCount := 0
	luckyTriggerChance = clampAlchemyFloat(luckyTriggerChance, 0, 1)
	for i := 0; i < actual; i++ {
		if rng.Float64() < failChance {
			failedCount++
			continue
		}
		ratio := sampleLuckSkewed(returnRatio.MinRatio, returnRatio.MaxRatio, stability, rng)
		gold, err := alchemyGoldFromRatio(economyBaseValue, ratio)
		if err != nil {
			return alchemyTransmuteBatchRoll{}, err
		}
		if rng.Float64() < luckyTriggerChance {
			tier := rollAlchemyTier(luckyTiers, luckyBonusChance, rng)
			if tier.Kind == "gold_multiplier" {
				multiplier := sampleLuckSkewed(1, math.Max(1, tier.Value), luckyBonusChance*2, rng)
				next, err := alchemyGoldAfterMultiplier(gold, multiplier)
				if err != nil {
					return alchemyTransmuteBatchRoll{}, err
				}
				if next > gold {
					gold = next
					luckyCount++
				}
			}
		}
		if err := addAlchemyRewardQuantity(rewards, GoldCoinItemID, gold); err != nil {
			return alchemyTransmuteBatchRoll{}, err
		}
	}
	return alchemyTransmuteBatchRoll{
		Rewards:     rewards,
		FailedCount: failedCount,
		LuckyCount:  luckyCount,
	}, nil
}

func sampleAlchemyBinomial(n int64, probability float64, rng *rand.Rand) int64 {
	if n <= 0 {
		return 0
	}
	p := clampAlchemyFloat(probability, 0, 1)
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return n
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if n <= exactBinomialSampleLimit {
		count := int64(0)
		for i := int64(0); i < n; i++ {
			if rng.Float64() < p {
				count++
			}
		}
		return count
	}
	mean := float64(n) * p
	if mean < 30 {
		return clampInt64(sampleAlchemyPoisson(mean, rng), 0, n)
	}
	upperMean := float64(n) * (1 - p)
	if upperMean < 30 {
		return n - clampInt64(sampleAlchemyPoisson(upperMean, rng), 0, n)
	}
	stddev := math.Sqrt(float64(n) * p * (1 - p))
	sampled := int64(math.Round(mean + rng.NormFloat64()*stddev))
	return clampInt64(sampled, 0, n)
}

func sampleAlchemyPoisson(lambda float64, rng *rand.Rand) int64 {
	if lambda <= 0 || math.IsNaN(lambda) || math.IsInf(lambda, 0) {
		return 0
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if lambda < 30 {
		limit := math.Exp(-lambda)
		product := 1.0
		count := int64(0)
		for product > limit {
			count++
			product *= rng.Float64()
		}
		return count - 1
	}
	sampled := int64(math.Round(lambda + rng.NormFloat64()*math.Sqrt(lambda)))
	if sampled < 0 {
		return 0
	}
	return sampled
}

func sampleAlchemyWeightedCounts(weights map[string]float64, count int64, rng *rand.Rand) (map[string]int64, error) {
	out := map[string]int64{}
	if count <= 0 {
		return out, nil
	}
	cleanWeights := map[string]float64{}
	total := 0.0
	for key, weight := range weights {
		key = strings.TrimSpace(key)
		if key == "" || weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			continue
		}
		cleanWeights[key] += weight
		total += weight
	}
	keys := make([]string, 0, len(cleanWeights))
	for key := range cleanWeights {
		keys = append(keys, key)
	}
	if len(keys) == 0 || total <= 0 {
		return nil, ErrAlchemyGroupNotFound
	}
	sort.Strings(keys)
	remainingCount := count
	remainingWeight := total
	for idx, key := range keys {
		if remainingCount <= 0 {
			break
		}
		if idx == len(keys)-1 {
			out[key] += remainingCount
			break
		}
		weight := cleanWeights[key]
		if weight <= 0 {
			continue
		}
		take := sampleAlchemyBinomial(remainingCount, weight/remainingWeight, rng)
		if take > 0 {
			out[key] += take
			remainingCount -= take
		}
		remainingWeight -= weight
		if remainingWeight <= 0 && remainingCount > 0 {
			out[key] += remainingCount
			break
		}
	}
	return out, nil
}

func sampleAlchemyProbabilityCounts(count int64, probabilities []float64, rng *rand.Rand) []int64 {
	out := make([]int64, len(probabilities))
	if count <= 0 || len(probabilities) == 0 {
		return out
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	total := 0.0
	clean := make([]float64, len(probabilities))
	for idx, probability := range probabilities {
		if probability <= 0 || math.IsNaN(probability) || math.IsInf(probability, 0) {
			continue
		}
		clean[idx] = probability
		total += probability
	}
	if total <= 0 {
		out[len(out)-1] = count
		return out
	}
	remainingCount := count
	remainingProbability := total
	for idx, probability := range clean {
		if remainingCount <= 0 {
			break
		}
		if probability <= 0 {
			continue
		}
		if idx == len(clean)-1 {
			out[idx] += remainingCount
			break
		}
		take := sampleAlchemyBinomial(remainingCount, probability/remainingProbability, rng)
		if take > 0 {
			out[idx] += take
			remainingCount -= take
		}
		remainingProbability -= probability
		if remainingProbability <= 0 && remainingCount > 0 {
			out[idx] += remainingCount
			break
		}
	}
	return out
}

type alchemyLuckSkewedBucketSet struct {
	Probabilities []float64
	Values        []float64
}

func alchemyLuckSkewedBuckets(minValue, maxValue, stability float64, bucketCount int) alchemyLuckSkewedBucketSet {
	if bucketCount <= 0 {
		bucketCount = 1
	}
	if maxValue < minValue {
		minValue, maxValue = maxValue, minValue
	}
	if maxValue == minValue {
		return alchemyLuckSkewedBucketSet{
			Probabilities: []float64{1},
			Values:        []float64{minValue},
		}
	}
	shape := alchemyLuckSkewedShape(stability)
	probabilities := make([]float64, 0, bucketCount)
	values := make([]float64, 0, bucketCount)
	for idx := 0; idx < bucketCount; idx++ {
		lower := float64(idx) / float64(bucketCount)
		upper := float64(idx+1) / float64(bucketCount)
		probability := math.Pow(upper, shape) - math.Pow(lower, shape)
		if probability <= 0 || math.IsNaN(probability) || math.IsInf(probability, 0) {
			continue
		}
		unitMean := averageLuckSkewedUnit(lower, upper, shape)
		probabilities = append(probabilities, probability)
		values = append(values, minValue+(maxValue-minValue)*unitMean)
	}
	if len(probabilities) == 0 {
		return alchemyLuckSkewedBucketSet{
			Probabilities: []float64{1},
			Values:        []float64{averageLuckSkewed(minValue, maxValue, stability)},
		}
	}
	return alchemyLuckSkewedBucketSet{Probabilities: probabilities, Values: values}
}

func alchemyLuckSkewedShape(stability float64) float64 {
	if stability < 0 || math.IsNaN(stability) || math.IsInf(stability, 0) {
		stability = 0
	}
	return 1 + stability
}

func averageLuckSkewed(minValue, maxValue, stability float64) float64 {
	if maxValue < minValue {
		minValue, maxValue = maxValue, minValue
	}
	if maxValue == minValue {
		return minValue
	}
	unitMean := averageLuckSkewedUnit(0, 1, alchemyLuckSkewedShape(stability))
	return minValue + (maxValue-minValue)*unitMean
}

func averageLuckSkewedUnit(lower, upper, shape float64) float64 {
	if upper < lower {
		lower, upper = upper, lower
	}
	lower = clampAlchemyFloat(lower, 0, 1)
	upper = clampAlchemyFloat(upper, 0, 1)
	if upper <= lower {
		return lower
	}
	if shape <= 0 || math.IsNaN(shape) || math.IsInf(shape, 0) {
		shape = 1
	}
	mass := math.Pow(upper, shape) - math.Pow(lower, shape)
	if mass <= 0 {
		return (lower + upper) / 2
	}
	mean := (shape / (shape + 1)) * (math.Pow(upper, shape+1) - math.Pow(lower, shape+1)) / mass
	return clampAlchemyFloat(mean, lower, upper)
}

func alchemyGoldFromRatio(economyBaseValue float64, ratio float64) (int64, error) {
	goldFloat := economyBaseValue * ratio
	if math.IsNaN(goldFloat) || math.IsInf(goldFloat, 0) || goldFloat > float64(math.MaxInt64) {
		return 0, ErrInventoryQuantityOverflow
	}
	gold := int64(math.Floor(goldFloat))
	if gold <= 0 {
		gold = 1
	}
	return gold, nil
}

func alchemyGoldAfterMultiplier(gold int64, multiplier float64) (int64, error) {
	goldFloat := float64(gold) * multiplier
	if math.IsNaN(goldFloat) || math.IsInf(goldFloat, 0) || goldFloat > float64(math.MaxInt64) {
		return 0, ErrInventoryQuantityOverflow
	}
	next := int64(math.Floor(goldFloat))
	if next <= 0 {
		next = 1
	}
	return next, nil
}

func addAlchemyRewardQuantity(rewards map[string]int64, itemID string, qty int64) error {
	if qty <= 0 {
		return nil
	}
	if rewards[itemID] > math.MaxInt64-qty {
		return ErrInventoryQuantityOverflow
	}
	rewards[itemID] += qty
	return nil
}

func clampInt64(value, minValue, maxValue int64) int64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (s *Store) alchemyGroupConvertOnceTx(ctx context.Context, tx pgx.Tx, userID int64, sourceItemID, groupID string, rng *rand.Rand, item QueueRuntimeItem, prepaid bool) (alchemyConvertResult, error) {
	group := s.gameData.AlchemyGroupByID(groupID)
	if group == nil {
		group = s.gameData.AlchemyGroupByItemID(sourceItemID)
	}
	if group == nil {
		return alchemyConvertResult{}, ErrAlchemyGroupNotFound
	}
	if !alchemyGroupContainsEntry(group, sourceItemID) {
		return alchemyConvertResult{}, ErrAlchemyGroupNotFound
	}
	derived, err := s.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return alchemyConvertResult{}, err
	}
	failChance := s.alchemyConvertFailChance(group, derived)
	failChance, err = alchemyeffect.ConversionFailureChance(failChance, alchemyOperationEffectSnapshot(item))
	if err != nil {
		return alchemyConvertResult{}, err
	}
	failed := rng.Float64() < failChance
	costs := map[string]int64{sourceItemID: 1}
	if !prepaid {
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, sourceItemID, 1, alchemyLedgerMeta(item, "inventory", "alchemy_convert", InventoryAuditDirectionConsume, "alchemy_convert", map[string]any{"groupId": group.ID, "failed": failed, "failChance": failChance})); err != nil {
			return alchemyConvertResult{}, err
		}
	}
	if group.GoldCost > 0 {
		costs[GoldCoinItemID] += group.GoldCost
	}
	if group.GoldCost > 0 && !prepaid {
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, GoldCoinItemID, group.GoldCost, alchemyLedgerMeta(item, "inventory", "alchemy_convert", InventoryAuditDirectionConsume, "alchemy_convert_gold", map[string]any{"groupId": group.ID, "sourceItemId": sourceItemID, "failed": failed, "failChance": failChance})); err != nil {
			return alchemyConvertResult{}, err
		}
	}
	if failed {
		return alchemyConvertResult{Costs: costs, Rewards: map[string]int64{}}, nil
	}
	weights := s.adjustAlchemyConvertWeights(sourceItemID, group.Entries, derived.LuckyChance, false)
	upgradeWeights := s.adjustAlchemyConvertWeights("", group.UpgradeEntries, derived.LuckyChance, true)
	reward := ""
	upgradeChance := alchemyConvertUpgradeChance(group, upgradeWeights)
	if len(upgradeWeights) > 0 && rng.Float64() < upgradeChance {
		reward = rollByWeight(upgradeWeights, rng)
	}
	if reward == "" {
		reward = rollByWeight(weights, rng)
	}
	if reward == "" {
		return alchemyConvertResult{}, ErrAlchemyGroupNotFound
	}
	if err := s.LedgerGrantTemplateTx(ctx, tx, userID, reward, 1, alchemyLedgerMeta(item, "alchemy_convert", "inventory", InventoryAuditDirectionGain, "alchemy_convert_reward", map[string]any{"groupId": group.ID, "sourceItemId": sourceItemID, "upgradeChance": upgradeChance, "failChance": failChance})); err != nil {
		return alchemyConvertResult{}, err
	}
	_ = s.markAlchemyDiscoveredTx(ctx, tx, userID, "group", group.ID)
	s.markAlchemyTraitsForItemTx(ctx, tx, userID, reward)
	return alchemyConvertResult{Costs: costs, Rewards: map[string]int64{reward: 1}}, nil
}

func alchemyGroupContainsEntry(group *catalog.AlchemyGroup, itemID string) bool {
	if group == nil {
		return false
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return false
	}
	for _, entry := range group.Entries {
		if strings.TrimSpace(entry.ItemID) == itemID {
			return true
		}
	}
	return false
}

func alchemyConvertFailChance(group *catalog.AlchemyGroup, derived alchemyDerived) float64 {
	if group == nil {
		return 0
	}
	return applyAlchemyMinimumLossFailChance(applyAlchemyStabilityToFailChance(group.FailChanceBase, derived.Stability), group.MinFailChance)
}

func (s *Store) alchemyConvertFailChance(group *catalog.AlchemyGroup, derived alchemyDerived) float64 {
	if group == nil {
		return 0
	}
	baseFail := s.alchemyLevelAdjustedFailChance(group.FailChanceBase, derived.Level, group.MinAlchemyLevel, 0)
	return applyAlchemyMinimumLossFailChance(applyAlchemyStabilityToFailChance(baseFail, derived.Stability), group.MinFailChance)
}

func alchemyConvertUpgradeChance(group *catalog.AlchemyGroup, upgradeWeights map[string]float64) float64 {
	if group == nil || len(upgradeWeights) == 0 {
		return 0
	}
	return clampAlchemyFloat(group.UpgradeChanceBase, 0, 1)
}

func (s *Store) adjustAlchemyConvertWeights(sourceItemID string, entries []catalog.AlchemyGroupEntry, luckyChance float64, allowSource bool) map[string]float64 {
	weights := map[string]float64{}
	for _, entry := range entries {
		itemID := strings.TrimSpace(entry.ItemID)
		if itemID == "" || entry.Weight <= 0 {
			continue
		}
		if !allowSource && itemID == sourceItemID && len(entries) > 1 {
			continue
		}
		weights[itemID] += math.Pow(entry.Weight, 1-clampAlchemyFloat(luckyChance*0.9, 0, 0.30))
	}
	if len(weights) == 0 {
		for _, entry := range entries {
			if strings.TrimSpace(entry.ItemID) != "" && entry.Weight > 0 {
				weights[entry.ItemID] += entry.Weight
			}
		}
	}
	return weights
}

type alchemyCraftResult struct {
	Costs   map[string]int64
	Rewards map[string]int64
}

func (s *Store) alchemyBenchCraftTx(ctx context.Context, tx pgx.Tx, userID int64, carrierInstanceID int64, materials map[string]int64, comboRecipeID, mediumItemID string, rng *rand.Rand, item QueueRuntimeItem, prepaid bool) (alchemyCraftResult, error) {
	logCtx := fmt.Sprintf("alchemy.bench userId=%d queueId=%d carrierInstanceId=%d materials=%v comboRecipeId=%q mediumItemId=%q", userID, item.QueueID, carrierInstanceID, materials, comboRecipeID, mediumItemID)
	carrier, err := s.LockInstanceTx(ctx, tx, carrierInstanceID)
	if err != nil {
		log.Printf("%s lock_carrier_err=%v", logCtx, err)
		return alchemyCraftResult{}, err
	}
	if carrier.UserID != userID {
		log.Printf("%s reject=NOT_OWNED carrier_user=%d", logCtx, carrier.UserID)
		return alchemyCraftResult{}, ErrInstanceNotOwned
	}
	if carrier.State != "inventory" {
		log.Printf("%s reject=INVALID_STATE state=%s", logCtx, carrier.State)
		return alchemyCraftResult{}, ErrInvalidInstanceState
	}
	rule := s.gameData.AffixCarrierRuleByItemID(carrier.TemplateItem)
	if rule == nil || !rule.Active || rule.MaxCustomAffixes <= 0 {
		log.Printf("%s reject=INVALID_CARRIER template=%q rule_nil=%v rule_active=%v max=%d", logCtx, carrier.TemplateItem, rule == nil, rule != nil && rule.Active, func() int64 {
			if rule != nil {
				return rule.MaxCustomAffixes
			}
			return 0
		}())
		return alchemyCraftResult{}, ErrAffixInvalidCarrier
	}
	derived, err := s.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		log.Printf("%s derived_err=%v", logCtx, err)
		return alchemyCraftResult{}, err
	}
	existingSlots := decodeAffixSlots(carrier.AffixesRaw)
	if len(existingSlots) > 0 {
		log.Printf("%s reject=CARRIER_OCCUPIED existing_slots=%d", logCtx, len(existingSlots))
		return alchemyCraftResult{}, ErrAlchemyCarrierOccupied
	}
	// Optional medium: resolves cross-domain (medium itemId may also be an enchant tool).
	// Each subsystem (medium/enchant) owns its own ruleset; this lookup does not
	// touch affix_enchant_rules.
	var medium *catalog.AlchemyMedium
	if mediumItemID != "" {
		medium = s.gameData.AlchemyMediumByItemID(mediumItemID)
		if medium == nil {
			log.Printf("%s reject=MEDIUM_UNKNOWN itemId=%s", logCtx, mediumItemID)
			return alchemyCraftResult{}, ErrAlchemyMediumUnknown
		}
	}
	// Enforce exact trait-material slot fill (no medium → DefaultBenchMaterialSlots).
	slotCap := catalog.EffectiveBenchSlots(medium)
	if len(materials) != slotCap {
		log.Printf("%s reject=SLOT_FILL_INVALID used=%d cap=%d", logCtx, len(materials), slotCap)
		return alchemyCraftResult{}, ErrAlchemyTraitSlotOverflow
	}
	modifiers, err := alchemyOperationEffectModifiers(item)
	if err != nil {
		return alchemyCraftResult{}, err
	}
	craft, failed, err := s.resolveAlchemyBenchCraftOutcomeWithModifiers(materials, comboRecipeID, medium, derived, modifiers, rng)
	if err != nil {
		log.Printf("%s resolve_outcome_err=%v", logCtx, err)
		return alchemyCraftResult{}, err
	}
	if err := stampAlchemyCraftOperationEffects(&craft, alchemyOperationEffectSnapshot(item)); err != nil {
		return alchemyCraftResult{}, err
	}
	if rule.AllowedTraitKind != "" && rule.AllowedTraitKind != "both" && craft.Kind != rule.AllowedTraitKind {
		log.Printf("%s reject=KIND_MISMATCH carrier_kind=%s craft_kind=%s", logCtx, rule.AllowedTraitKind, craft.Kind)
		return alchemyCraftResult{}, ErrAlchemyKindMismatch
	}
	goldCost := s.alchemyBenchGoldCost(materials, comboRecipeID, medium)
	targetCarrierID := carrier.InstanceID
	if carrier.Quantity > 1 {
		targetCarrierID, err = s.SplitOneFromStackTx(ctx, tx, carrier.InstanceID, userID)
		if err != nil {
			log.Printf("%s split_err=%v", logCtx, err)
			return alchemyCraftResult{}, err
		}
		log.Printf("%s split src=%d -> new=%d", logCtx, carrier.InstanceID, targetCarrierID)
	}
	nextSlot := 0
	slots := []EquipmentAffixSlot{}
	costs := map[string]int64{}
	if goldCost > 0 {
		costs[GoldCoinItemID] += goldCost
	}
	if goldCost > 0 && !prepaid {
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, GoldCoinItemID, goldCost, alchemyLedgerMeta(item, "inventory", "alchemy_bench", InventoryAuditDirectionConsume, "alchemy_bench_gold", map[string]any{"failed": false, "carrierInstanceId": carrierInstanceID})); err != nil {
			log.Printf("%s consume_gold_err qty=%d err=%v", logCtx, goldCost, err)
			return alchemyCraftResult{}, err
		}
	}
	log.Printf("%s craft_resolved kind=%s effect=%s fail_chance=%.3f failed=%v target_carrier=%d", logCtx, craft.Kind, craft.EffectType, craft.FailChance, failed, targetCarrierID)
	// Medium is consumed regardless of success (similar to "fuel"). On fail it
	// is consumed in full — matches game-feel of "potion used up".
	if medium != nil {
		costs[medium.ItemID] += medium.ConsumePerCraft
		if !prepaid {
			if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, medium.ItemID, medium.ConsumePerCraft, alchemyLedgerMeta(item, "inventory", "alchemy_bench", InventoryAuditDirectionConsume, "alchemy_bench_medium", map[string]any{"mediumId": medium.ID, "failed": failed, "carrierInstanceId": carrierInstanceID})); err != nil {
				log.Printf("%s consume_medium_err itemId=%s qty=%d err=%v", logCtx, medium.ItemID, medium.ConsumePerCraft, err)
				return alchemyCraftResult{}, err
			}
		}
	}
	for itemID, qty := range materials {
		// Full consumption regardless of success/failure (soft-floor mechanic).
		costs[itemID] += qty
		if !prepaid {
			if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, qty, alchemyLedgerMeta(item, "inventory", "alchemy_bench", InventoryAuditDirectionConsume, "alchemy_bench_material", map[string]any{"failed": failed, "carrierInstanceId": carrierInstanceID})); err != nil {
				log.Printf("%s consume_err item=%s qty=%d err=%v", logCtx, itemID, qty, err)
				return alchemyCraftResult{}, err
			}
		}
	}
	newSlot := alchemyCraftSlot(craft)
	newSlot.SlotIndex = nextSlot
	slots = append(slots, newSlot)
	if err := s.SetAffixesTx(ctx, tx, targetCarrierID, slots); err != nil {
		log.Printf("%s set_affixes_err target=%d err=%v", logCtx, targetCarrierID, err)
		return alchemyCraftResult{}, err
	}
	if err := s.refreshInstanceBaseSnapshotTx(ctx, tx, targetCarrierID, userID); err != nil {
		log.Printf("%s refresh_snapshot_err target=%d err=%v", logCtx, targetCarrierID, err)
		return alchemyCraftResult{}, err
	}
	targetCarrierIDCopy := targetCarrierID
	resultMeta := alchemyLedgerMeta(item, "alchemy_bench", "inventory", InventoryAuditDirectionGain, "alchemy_bench_result", map[string]any{
		"failed":                  failed,
		"carrierInstanceId":       carrierInstanceID,
		"targetCarrierInstanceId": targetCarrierID,
		"affixId":                 newSlot.AffixID,
	})
	s.recordInventoryAuditTx(tx, InventoryAuditEvent{
		UserID:        userID,
		Category:      resultMeta.Category,
		Direction:     resultMeta.Direction,
		SubjectType:   resultMeta.SubjectType,
		SubjectID:     resultMeta.SubjectID,
		EventType:     resultMeta.EventType,
		InstanceID:    &targetCarrierIDCopy,
		TemplateItem:  carrier.TemplateItem,
		Quantity:      1,
		SourceSurface: resultMeta.SourceSurface,
		TargetSurface: resultMeta.TargetSurface,
		Details:       resultMeta.Details,
	})
	for _, traitID := range craft.TraitIDs {
		_ = s.markAlchemyDiscoveredTx(ctx, tx, userID, "trait", traitID)
	}
	if craft.ComboID != "" {
		_ = s.markAlchemyDiscoveredTx(ctx, tx, userID, "combo", craft.ComboID)
	}
	log.Printf("%s result=SUCCESS target_carrier=%d affix_id=%s costs=%v", logCtx, targetCarrierID, newSlot.AffixID, costs)
	return alchemyCraftResult{Costs: costs, Rewards: map[string]int64{}}, nil
}

func (s *Store) alchemyBenchGoldCost(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium) int64 {
	total := s.config.Alchemy.InfusionBaseGoldCost
	if total <= 0 {
		total = 32
	}
	for itemID, qty := range materials {
		if qty <= 0 {
			continue
		}
		if def := s.gameData.ItemTraitByItemID(itemID); def != nil {
			for _, trait := range def.Traits {
				if trait.GoldCostPerUnit > 0 {
					total += trait.GoldCostPerUnit * qty
				}
			}
		}
	}
	if medium != nil && medium.ConsumePerCraft > 0 {
		if item := s.gameData.ItemByID(medium.ItemID); item != nil && item.EconomyBaseValue > 0 {
			total += int64(math.Ceil(math.Sqrt(item.EconomyBaseValue))) * medium.ConsumePerCraft
		}
	}
	if match := s.matchAlchemyCombo(materials, comboRecipeID); match != nil && match.Recipe != nil && match.Recipe.ExtraGoldCost > 0 {
		total += match.Recipe.ExtraGoldCost * maxInt64(1, match.Multiplier)
	}
	if total < 0 {
		return 0
	}
	return total
}

func (s *Store) alchemyBenchPerRunCosts(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium) map[string]int64 {
	costs := map[string]int64{}
	if gold := s.alchemyBenchGoldCost(materials, comboRecipeID, medium); gold > 0 {
		costs[GoldCoinItemID] += gold
	}
	if medium != nil && medium.ConsumePerCraft > 0 {
		costs[medium.ItemID] += medium.ConsumePerCraft
	}
	for itemID, qty := range materials {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" && qty > 0 {
			costs[itemID] += qty
		}
	}
	return costs
}

func (s *Store) resolveAlchemyBenchCraftOutcome(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, rng *rand.Rand) (resolvedAlchemyCraft, bool, error) {
	return s.resolveAlchemyBenchCraftOutcomeWithModifiers(materials, comboRecipeID, medium, derived, alchemyeffect.Modifiers{}, rng)
}

func (s *Store) resolveAlchemyBenchCraftOutcomeWithModifiers(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, modifiers alchemyeffect.Modifiers, rng *rand.Rand) (resolvedAlchemyCraft, bool, error) {
	craft, err := s.resolveAlchemyCraftWithModifiers(materials, comboRecipeID, medium, derived, modifiers, rng)
	if err != nil {
		return resolvedAlchemyCraft{}, false, err
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	failed := rng.Float64() < craft.FailChance
	// Soft-floor failure mechanic: on failure, the craft STILL produces an affix
	// but every effect value collapses to its per-unit minimum × qty.
	if failed {
		if floorCraft, matched, err := s.resolveAlchemyComboCraftWithModifiers(materials, comboRecipeID, medium, derived, modifiers, nil, true); matched && err == nil {
			craft.AttrMods = floorCraft.AttrMods
			craft.EffectParams = floorCraft.EffectParams
			craft.Components = floorAlchemyComponents(floorCraft.Components)
			setAlchemyComponentParams(craft.EffectParams, craft.Components)
			craft.Value = floorCraft.Value
		} else {
			floorAttrMods := s.computeFloorAttrModsWithModifiers(materials, comboRecipeID, medium, derived, modifiers)
			craft.AttrMods = floorAttrMods
			craft.EffectParams["attrMods"] = floorAttrMods
			craft.Components = floorAlchemyComponents(craft.Components)
			setAlchemyComponentParams(craft.EffectParams, craft.Components)
			craft.Value = stableValueSum(floorAttrMods)
		}
	}
	return craft, failed, nil
}

func alchemyCraftSlot(craft resolvedAlchemyCraft) EquipmentAffixSlot {
	return EquipmentAffixSlot{
		SlotIndex:             0,
		AffixID:               encodeCustomAffixID(craft.EffectType, craft.EffectParams, craft.Value, 3),
		RarityID:              craft.RarityID,
		SourceKind:            "craft",
		AlchemyRulesetVersion: AlchemyCurrentRulesetVersion,
		DisplayNameKey:        craft.DisplayNameKey,
		DisplayNameArgs:       craft.DisplayNameArgs,
		ValuePrecision:        3,
		AttrMods:              craft.AttrMods,
		EffectType:            craft.EffectType,
		EffectParams:          craft.EffectParams,
	}
}

func (s *Store) applyAlchemyBenchBatchSlotsTx(ctx context.Context, tx pgx.Tx, userID int64, carrier LockedItemInstance, slots []EquipmentAffixSlot) ([]int64, error) {
	if len(slots) == 0 {
		return nil, nil
	}
	if carrier.UserID != userID {
		return nil, ErrInstanceNotOwned
	}
	if carrier.State != "inventory" {
		return nil, ErrInvalidInstanceState
	}
	if carrier.Quantity < int64(len(slots)) {
		return nil, ErrInsufficientItem
	}
	targetIDs := make([]int64, 0, len(slots))
	nextSlotRaw := func(slot EquipmentAffixSlot) string {
		slot.SlotIndex = 0
		return string(encodeAffixSlots([]EquipmentAffixSlot{slot}))
	}
	insertOne := func(slot EquipmentAffixSlot) (int64, error) {
		var instanceID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO player_item_instances(
				user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
				milestone_affix_claims, durability, max_durability, quantity, state, created_at, updated_at
			)
			VALUES($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9, 1, 'inventory', now(), now())
			RETURNING instance_id
		`, carrier.UserID, carrier.TemplateItem, carrier.EnhanceLevel, carrier.RollSeed, string(carrier.BaseSnapshot), nextSlotRaw(slot), string(carrier.MilestoneAffixesRaw), carrier.Durability, carrier.MaxDurability).Scan(&instanceID); err != nil {
			return 0, err
		}
		if err := s.refreshInstanceBaseSnapshotTx(ctx, tx, instanceID, userID); err != nil {
			return 0, err
		}
		return instanceID, nil
	}

	startInsertAt := 0
	if carrier.Quantity == int64(len(slots)) {
		if _, err := tx.Exec(ctx, `
			UPDATE player_item_instances
			SET quantity = 1, affixes = $2::jsonb, updated_at = now()
			WHERE instance_id = $1 AND user_id = $3
		`, carrier.InstanceID, nextSlotRaw(slots[0]), userID); err != nil {
			return nil, err
		}
		if err := s.refreshInstanceBaseSnapshotTx(ctx, tx, carrier.InstanceID, userID); err != nil {
			return nil, err
		}
		targetIDs = append(targetIDs, carrier.InstanceID)
		startInsertAt = 1
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE player_item_instances
			SET quantity = quantity - $1, updated_at = now()
			WHERE instance_id = $2 AND user_id = $3 AND quantity >= $1
		`, int64(len(slots)), carrier.InstanceID, userID); err != nil {
			return nil, err
		}
	}
	for i := startInsertAt; i < len(slots); i++ {
		instanceID, err := insertOne(slots[i])
		if err != nil {
			return nil, err
		}
		targetIDs = append(targetIDs, instanceID)
	}
	return targetIDs, nil
}

type resolvedAlchemyCraft struct {
	Kind            string
	EffectType      string
	EffectParams    map[string]any
	AttrMods        map[string]float64
	Value           float64
	RarityID        string
	FailChance      float64
	DisplayNameKey  string
	DisplayNameArgs map[string]any
	TraitIDs        []string
	ComboID         string
	Components      []alchemyCraftComponent
}

type matchedAlchemyCombo struct {
	Recipe     *catalog.AlchemyComboRecipe
	Multiplier int64
}

type alchemyCraftComponent struct {
	SourceKind               string                       `json:"sourceKind"`
	ItemID                   string                       `json:"itemId,omitempty"`
	TraitID                  string                       `json:"traitId,omitempty"`
	ComboRecipeID            string                       `json:"comboRecipeId,omitempty"`
	Family                   string                       `json:"family,omitempty"`
	Attr                     string                       `json:"attr,omitempty"`
	Quantity                 int64                        `json:"quantity,omitempty"`
	ValueRange               catalog.AlchemyValueRange    `json:"valueRange"`
	CurrentValue             float64                      `json:"currentValue"`
	MaxEffectivePerEquipment int64                        `json:"maxEffectivePerEquipment"`
	MaxEffectivePerBody      int64                        `json:"maxEffectivePerBody"`
	OperationEffects         *alchemyOperationEffectStamp `json:"operationEffects,omitempty"`
}

// computeFloorAttrMods mirrors resolveAlchemyCraft's per-effect math but
// substitutes each per-unit roll with the deterministic minimum (the worst
// possible outcome). Used on failure to keep the bench from being a total loss
// — players still walk away with an affix, just stamped with floor values.
// Medium modifiers are applied here too so the floor matches the medium's
// reshaped curve.
func (s *Store) computeFloorAttrMods(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived) map[string]float64 {
	return s.computeFloorAttrModsWithModifiers(materials, comboRecipeID, medium, derived, alchemyeffect.Modifiers{})
}

func (s *Store) computeFloorAttrModsWithModifiers(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, modifiers alchemyeffect.Modifiers) map[string]float64 {
	attrMods := map[string]float64{}
	// Combo branch: floor = combo.OutputAffix.ValueRange.Min.
	if craft, matched, err := s.resolveAlchemyComboCraftWithModifiers(materials, comboRecipeID, medium, derived, modifiers, nil, true); matched && err == nil {
		return craft.AttrMods
	}
	// Trait branch: floor = effect.valuePerUnit.Min × qty, per attribute.
	for itemID, qty := range materials {
		def := s.gameData.ItemTraitByItemID(itemID)
		if def == nil {
			continue
		}
		for _, trait := range def.Traits {
			for _, effect := range trait.Effects {
				attr := strings.TrimSpace(effect.Attr)
				if attr == "" {
					continue
				}
				minValue, maxValue := applyAlchemyDerivedRange(effect.ValuePerUnit.Min*float64(qty), effect.ValuePerUnit.Max*float64(qty), derived.Precision, derived.Stability)
				minValue, _ = applyAlchemyInfusionOperationRange(minValue, maxValue, modifiers)
				raw := minValue
				value := s.roundAlchemyAttributeValue(attr, applyMediumToValue(raw, trait.Family, medium))
				attrMods[attr] += value
			}
		}
	}
	for key, value := range attrMods {
		attrMods[key] = s.roundAlchemyAttributeValue(key, value)
	}
	return attrMods
}

func floorAlchemyComponents(components []alchemyCraftComponent) []alchemyCraftComponent {
	if len(components) == 0 {
		return nil
	}
	out := make([]alchemyCraftComponent, 0, len(components))
	for _, component := range components {
		component.CurrentValue = component.ValueRange.Min
		out = append(out, component)
	}
	return out
}

func setAlchemyComponentParams(params map[string]any, components []alchemyCraftComponent) {
	if params == nil {
		return
	}
	if len(components) == 0 {
		delete(params, "alchemyComponents")
		return
	}
	params["alchemyComponents"] = components
}

func (s *Store) alchemyComponentRange(attr string, minRaw, maxRaw float64, family string, medium *catalog.AlchemyMedium) catalog.AlchemyValueRange {
	minValue := s.roundAlchemyAttributeValue(attr, applyMediumToValue(minRaw, family, medium))
	maxValue := s.roundAlchemyAttributeValue(attr, applyMediumToValue(maxRaw, family, medium))
	if minValue > maxValue {
		minValue, maxValue = maxValue, minValue
	}
	return catalog.AlchemyValueRange{Min: minValue, Max: maxValue}
}

func applyAlchemyDerivedRange(minRaw, maxRaw, precision, stability float64) (float64, float64) {
	if maxRaw < minRaw {
		minRaw, maxRaw = maxRaw, minRaw
	}
	if minRaw < 0 || maxRaw <= 0 {
		return minRaw, maxRaw
	}
	precisionRatio := alchemySaturatingRatio(precision)
	stabilityRatio := alchemySaturatingRatio(stability)
	baseMax := maxRaw
	if stabilityRatio >= 0 {
		minRaw = math.Max(minRaw, baseMax*stabilityRatio)
	} else {
		minRaw = baseMax * stabilityRatio
	}
	maxRaw = baseMax * math.Max(0.000001, 1+precisionRatio+stabilityRatio*0.25)
	if minRaw > maxRaw {
		minRaw = maxRaw
	}
	return minRaw, maxRaw
}

func capAlchemyInfusionPositiveRange(minRaw, maxRaw float64) (float64, float64) {
	return applyAlchemyInfusionOperationRange(minRaw, maxRaw, alchemyeffect.Modifiers{})
}

func applyAlchemyInfusionOperationRange(minRaw, maxRaw float64, modifiers alchemyeffect.Modifiers) (float64, float64) {
	if maxRaw < minRaw {
		minRaw, maxRaw = maxRaw, minRaw
	}
	if maxRaw <= 0 {
		return minRaw, maxRaw
	}
	if minRaw > 0 && modifiers.PositiveFloorReductionRate > 0 {
		minRaw *= 1 - clampAlchemyFloat(modifiers.PositiveFloorReductionRate, 0, alchemyeffect.MaxPositiveFloorReductionRate)
	}
	cappedMax := maxRaw * alchemyInfusionPositiveCeilingMultiplier(modifiers)
	return minRaw, math.Max(minRaw, cappedMax)
}

func alchemyInfusionPositiveCeilingMultiplier(modifiers alchemyeffect.Modifiers) float64 {
	bonus := clampAlchemyFloat(modifiers.PositiveCeilingBonusRate, 0, alchemyeffect.MaxPositiveCeilingBonusRate)
	return (1 + bonus) * alchemyInfusionPositiveMaxRatio
}

func alchemySaturatingRatio(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value / (1 + math.Abs(value))
}

// applyMediumToValue applies one medium's value-side modifiers to a raw rolled value.
//   - Positive values: multiplied by ValueMultiplier (+ familyAffinity[family]).
//   - Negative values: multiplied by NegativeMultiplier.
//
// Family affinity only boosts positive effects (so a "+offense" medium can't
// inadvertently amplify a -hp penalty on the same trait).
func applyMediumToValue(value float64, family string, m *catalog.AlchemyMedium) float64 {
	if m == nil {
		return value
	}
	if value >= 0 {
		mult := m.Modifiers.ValueMultiplier
		if mult == 0 {
			mult = 1
		}
		if family != "" {
			if bonus, ok := m.Modifiers.FamilyAffinity[family]; ok {
				mult *= 1 + bonus
			}
		}
		return value * mult
	}
	mult := m.Modifiers.NegativeMultiplier
	if mult == 0 {
		mult = 1
	}
	return value * mult
}

// applyMediumToFailChance adjusts the rolled fail chance with the medium
// delta and clamps to the medium's successFloor (i.e. failChance ≤ 1-floor).
func applyMediumToFailChance(failChance float64, m *catalog.AlchemyMedium) float64 {
	if m == nil {
		return clampAlchemyFloat(failChance, 0, 1)
	}
	adjusted := failChance + m.Modifiers.FailChanceDelta
	if m.Modifiers.SuccessFloor > 0 {
		maxFail := 1 - m.Modifiers.SuccessFloor
		if adjusted > maxFail {
			adjusted = maxFail
		}
	}
	return clampAlchemyFloat(adjusted, 0, 1)
}

func applyAlchemyStabilityToFailChance(failChance, stability float64) float64 {
	failChance = clampAlchemyFloat(failChance, 0, 1)
	if stability >= 0 {
		return clampAlchemyFloat(failChance/math.Max(0.01, 1+stability), 0, 1)
	}
	return clampAlchemyFloat(failChance*(1+math.Abs(stability)), 0, 1)
}

func applyAlchemyMinimumLossFailChance(failChance, minFailChance float64) float64 {
	return clampAlchemyFloat(math.Max(clampAlchemyFloat(minFailChance, 0, 1), failChance), 0, 1)
}

func alchemyTraitUnitBasis(trait catalog.AlchemyTraitEntry) int64 {
	if trait.UnitBasis <= 0 {
		return 1
	}
	return trait.UnitBasis
}

func alchemyStabilityQuantityMultiplier(stability float64) float64 {
	if math.IsNaN(stability) || math.IsInf(stability, 0) || stability <= 0 {
		return 1
	}
	if stability <= alchemyQuantityLinearStabilityKnee {
		return 1 + stability
	}
	if stability <= alchemyQuantityBridgeStart {
		// Preserve the 500% = 6x and 1000% = 8x anchors.
		return 6 + alchemyQuantityLogRate*math.Log1p(alchemyQuantityLogScale*(stability-alchemyQuantityLinearStabilityKnee))
	}
	if stability <= alchemyQuantityPursuitStart {
		// Smoothly bridge 1000% = 8x to 3000% = 15x. The endpoint
		// tangent is 1x per 1000%, matching the pursuit segment below.
		t := (stability - alchemyQuantityBridgeStart) / (alchemyQuantityPursuitStart - alchemyQuantityBridgeStart)
		return 8 + 6*t + 7*t*t - 6*t*t*t
	}
	if stability <= alchemyQuantitySoftCapStart {
		return 15 + (stability-alchemyQuantityPursuitStart)/10
	}
	// Continue the 1x-per-1000% tangent, then approach the 32x ceiling.
	x := (stability - alchemyQuantitySoftCapStart) / 10
	return alchemyQuantitySoftCap - 100/(10+x)
}

func alchemyTraitQuantityFailDelta(trait catalog.AlchemyTraitEntry, qty int64) float64 {
	basis := alchemyTraitUnitBasis(trait)
	extraBasis := qty/basis - 1
	if extraBasis <= 0 || trait.FailChancePerExtraBasis <= 0 {
		return 0
	}
	delta := float64(extraBasis) * trait.FailChancePerExtraBasis
	if trait.FailChanceQuantityCap > 0 && delta > trait.FailChanceQuantityCap {
		delta = trait.FailChanceQuantityCap
	}
	return clampAlchemyFloat(delta, 0, 1)
}

func alchemyTraitMaxQuantity(trait catalog.AlchemyTraitEntry, stability float64) int64 {
	basis := alchemyTraitUnitBasis(trait)
	baseMax := basis * trait.MaxBasisMultiplier
	if baseMax < basis {
		baseMax = basis
	}
	multiplier := alchemyStabilityQuantityMultiplier(stability)
	scaled := int64(math.Floor(float64(baseMax)*multiplier/float64(basis)+1e-9)) * basis
	if scaled < basis {
		return basis
	}
	return scaled
}

func (s *Store) validateAlchemyBenchLevelRequirements(materials map[string]int64, medium *catalog.AlchemyMedium, derived alchemyDerived) error {
	if medium != nil && derived.Level < float64(medium.MinAlchemyLevel) {
		return ErrAlchemyLevelInsufficient
	}
	for itemID := range materials {
		def := s.gameData.ItemTraitByItemID(itemID)
		if def == nil || len(def.Traits) == 0 {
			return ErrAlchemyArgsInvalid
		}
		for _, trait := range def.Traits {
			if derived.Level < float64(trait.MinAlchemyLevel) {
				return ErrAlchemyLevelInsufficient
			}
			break
		}
	}
	return nil
}

func (s *Store) resolveAlchemyCraft(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, rng *rand.Rand) (resolvedAlchemyCraft, error) {
	return s.resolveAlchemyCraftWithModifiers(materials, comboRecipeID, medium, derived, alchemyeffect.Modifiers{}, rng)
}

func (s *Store) resolveAlchemyCraftWithModifiers(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, modifiers alchemyeffect.Modifiers, rng *rand.Rand) (resolvedAlchemyCraft, error) {
	if err := s.validateAlchemyBenchLevelRequirements(materials, medium, derived); err != nil {
		return resolvedAlchemyCraft{}, err
	}
	if craft, matched, err := s.resolveAlchemyComboCraftWithModifiers(materials, comboRecipeID, medium, derived, modifiers, rng, false); matched || err != nil {
		return craft, err
	}
	if strings.TrimSpace(comboRecipeID) != "" {
		return resolvedAlchemyCraft{}, ErrAlchemyArgsInvalid
	}
	kind := ""
	failChance := 0.0
	attrMods := map[string]float64{}
	params := map[string]any{}
	traitIDs := []string{}
	components := []alchemyCraftComponent{}
	for itemID, qty := range materials {
		def := s.gameData.ItemTraitByItemID(itemID)
		if def == nil || len(def.Traits) == 0 {
			log.Printf("alchemy.resolve reject=NO_TRAIT_FOR_ITEM item=%s", itemID)
			return resolvedAlchemyCraft{}, ErrAlchemyArgsInvalid
		}
		for _, trait := range def.Traits {
			if kind == "" {
				kind = trait.Kind
			} else if kind != trait.Kind {
				log.Printf("alchemy.resolve reject=MIXED_KIND item=%s prev_kind=%s trait_kind=%s", itemID, kind, trait.Kind)
				return resolvedAlchemyCraft{}, ErrAlchemyMixedKind
			}
			basis := alchemyTraitUnitBasis(trait)
			if qty%basis != 0 {
				log.Printf("alchemy.resolve reject=BASIS_INVALID item=%s trait=%s qty=%d basis=%d", itemID, trait.TraitID, qty, basis)
				return resolvedAlchemyCraft{}, ErrAlchemyBasisInvalid
			}
			maxQty := s.alchemyMaterialMaxQuantity(itemID, trait, derived.Stability, modifiers)
			if qty > maxQty {
				log.Printf("alchemy.resolve reject=OVERFLOW item=%s trait=%s qty=%d max=%d stability=%.4f", itemID, trait.TraitID, qty, maxQty, derived.Stability)
				return resolvedAlchemyCraft{}, ErrAlchemyMaterialOverflow
			}
			// Sample each effect independently so multi-attribute traits roll a
			// fresh value per attr — gives visible diversity at high qty.
			for _, effect := range trait.Effects {
				attr := strings.TrimSpace(effect.Attr)
				if attr == "" {
					continue
				}
				rangeMin, rangeMax := applyAlchemyDerivedRange(effect.ValuePerUnit.Min*float64(qty), effect.ValuePerUnit.Max*float64(qty), derived.Precision, derived.Stability)
				rangeMin, rangeMax = applyAlchemyInfusionOperationRange(rangeMin, rangeMax, modifiers)
				rawValue := sampleLuckSkewed(rangeMin, rangeMax, 0, rng)
				value := s.roundAlchemyAttributeValue(attr, applyMediumToValue(rawValue, trait.Family, medium))
				attrMods[attr] += value
				components = append(components, alchemyCraftComponent{
					SourceKind:               "trait_effect",
					ItemID:                   itemID,
					TraitID:                  trait.TraitID,
					Family:                   trait.Family,
					Attr:                     attr,
					Quantity:                 qty,
					ValueRange:               s.alchemyComponentRange(attr, rangeMin, rangeMax, trait.Family, medium),
					CurrentValue:             value,
					MaxEffectivePerEquipment: trait.MaxEffectivePerEquipment,
					MaxEffectivePerBody:      trait.MaxEffectivePerBody,
				})
				if _, ok := params["attr"]; !ok {
					params["attr"] = attr
				}
			}
			baseFail := s.alchemyLevelAdjustedFailChance(trait.FailBaseChance, derived.Level, trait.MinAlchemyLevel, trait.FailChancePerLevelAbove)
			baseFail = clampAlchemyFloat(baseFail+alchemyTraitQuantityFailDelta(trait, qty), 0, 1)
			rawFail := applyAlchemyStabilityToFailChance(baseFail, derived.Stability)
			if rawFail > failChance {
				failChance = rawFail
			}
			traitIDs = append(traitIDs, trait.TraitID)
		}
	}
	// Medium's failChanceDelta & successFloor apply to the aggregate (max-of) failChance once.
	if medium != nil {
		failChance = clampAlchemyFloat(failChance+s.alchemyLevelDeficitFailDelta(derived.Level, medium.MinAlchemyLevel), 0, 1)
	}
	failChance = applyMediumToFailChance(failChance, medium)
	failChance = applyAlchemyFoodFailureChance(failChance, medium, modifiers)
	for key, value := range attrMods {
		attrMods[key] = s.roundAlchemyAttributeValue(key, value)
	}
	params["attrMods"] = attrMods
	params["alchemyKind"] = kind
	setAlchemyComponentParams(params, components)
	displayKey, displayArgs := s.computeAlchemyDisplayName(materials, "")
	return resolvedAlchemyCraft{Kind: kind, EffectType: "add_stat", EffectParams: params, AttrMods: attrMods, Value: stableValueSum(attrMods), RarityID: s.maxMaterialRarity(materials), FailChance: failChance, DisplayNameKey: displayKey, DisplayNameArgs: displayArgs, TraitIDs: traitIDs, Components: components}, nil
}

func (s *Store) resolveAlchemyComboCraft(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, rng *rand.Rand, forceFloor bool) (resolvedAlchemyCraft, bool, error) {
	return s.resolveAlchemyComboCraftWithModifiers(materials, comboRecipeID, medium, derived, alchemyeffect.Modifiers{}, rng, forceFloor)
}

func (s *Store) resolveAlchemyComboCraftWithModifiers(materials map[string]int64, comboRecipeID string, medium *catalog.AlchemyMedium, derived alchemyDerived, modifiers alchemyeffect.Modifiers, rng *rand.Rand, forceFloor bool) (resolvedAlchemyCraft, bool, error) {
	match := s.matchAlchemyCombo(materials, comboRecipeID)
	if match == nil {
		return resolvedAlchemyCraft{}, false, nil
	}
	combo := match.Recipe
	if combo == nil {
		return resolvedAlchemyCraft{}, true, ErrAlchemyArgsInvalid
	}
	if err := s.validateAlchemyComboMaterialsWithModifiers(materials, derived, modifiers); err != nil {
		return resolvedAlchemyCraft{}, true, err
	}
	scale := float64(maxInt64(1, match.Multiplier))
	baseMin := combo.OutputAffix.ValueRange.Min * scale
	baseMax := combo.OutputAffix.ValueRange.Max * scale
	comboMin, comboMax := applyAlchemyDerivedRange(baseMin, baseMax, derived.Precision, derived.Stability)
	comboMin, comboMax = applyAlchemyInfusionOperationRange(comboMin, comboMax, modifiers)
	rawValue := comboMin
	if !forceFloor {
		if rng == nil {
			rng = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		rawValue = sampleLuckSkewed(comboMin, comboMax, 0, rng)
	}
	value := roundAlchemyValue(applyMediumToValue(roundAlchemyValue(rawValue, 3), "", medium), 3)
	params := cloneMap(combo.OutputAffix.Params)
	if resolvedParams, ok := resolveAlchemyComboParamTemplates(params, value, match.Multiplier).(map[string]any); ok {
		params = resolvedParams
	}
	params["alchemyKind"] = combo.Kind
	params["comboPower"] = value
	params["comboMultiplier"] = match.Multiplier
	attrMods := extractAlchemyComboAttrMods(params)
	attr := ""
	if rawAttr, ok := params["attr"]; ok && rawAttr != nil {
		attr = cleanAlchemyComponentAttr(fmt.Sprint(rawAttr))
	}
	if attr != "" && len(attrMods) == 0 {
		attrMods[attr] = value
		params["attrMods"] = map[string]float64{attr: value}
	} else if len(attrMods) > 0 {
		params["attrMods"] = attrMods
	}
	components := make([]alchemyCraftComponent, 0, len(materials)+1)
	components = append(components, alchemyCraftComponent{
		SourceKind:    "combo_output",
		ComboRecipeID: combo.ID,
		Attr:          attr,
		ValueRange:    s.alchemyComponentRange("", comboMin, comboMax, "", medium),
		CurrentValue:  value,
	})
	for itemID, qty := range materials {
		if traitDef := s.gameData.ItemTraitByItemID(itemID); traitDef != nil {
			for _, trait := range traitDef.Traits {
				components = append(components, alchemyCraftComponent{
					SourceKind:               "combo_material",
					ItemID:                   itemID,
					TraitID:                  trait.TraitID,
					Quantity:                 qty,
					MaxEffectivePerEquipment: trait.MaxEffectivePerEquipment,
					MaxEffectivePerBody:      trait.MaxEffectivePerBody,
				})
				break
			}
		}
	}
	setAlchemyComponentParams(params, components)
	rarityID := s.maxMaterialRarity(materials)
	if combo.OutputRarity != nil && strings.TrimSpace(*combo.OutputRarity) != "" {
		rarityID = strings.TrimSpace(*combo.OutputRarity)
	}
	baseFail := s.alchemyLevelAdjustedFailChance(combo.FailBaseChance, derived.Level, combo.MinAlchemyLevel, combo.FailChancePerLevelAbove)
	baseFail = clampAlchemyFloat(baseFail+s.alchemyMaterialsLevelDeficitFailDelta(materials, derived), 0, 1)
	fail := applyAlchemyStabilityToFailChance(baseFail, derived.Stability)
	if medium != nil {
		fail = clampAlchemyFloat(fail+s.alchemyLevelDeficitFailDelta(derived.Level, medium.MinAlchemyLevel), 0, 1)
	}
	fail = applyMediumToFailChance(fail, medium)
	fail = applyAlchemyFoodFailureChance(fail, medium, modifiers)
	return resolvedAlchemyCraft{Kind: combo.Kind, EffectType: combo.OutputAffix.EffectType, EffectParams: params, AttrMods: attrMods, Value: value, RarityID: rarityID, FailChance: fail, DisplayNameKey: combo.NameKey, DisplayNameArgs: map[string]any{}, ComboID: combo.ID, Components: components}, true, nil
}

func (s *Store) validateAlchemyComboMaterials(materials map[string]int64, derived alchemyDerived) error {
	return s.validateAlchemyComboMaterialsWithModifiers(materials, derived, alchemyeffect.Modifiers{})
}

func (s *Store) validateAlchemyComboMaterialsWithModifiers(materials map[string]int64, derived alchemyDerived, modifiers alchemyeffect.Modifiers) error {
	for itemID, qty := range materials {
		def := s.gameData.ItemTraitByItemID(itemID)
		if def == nil || len(def.Traits) == 0 {
			log.Printf("alchemy.combo reject=NO_TRAIT_FOR_ITEM item=%s", itemID)
			return ErrAlchemyArgsInvalid
		}
		for _, trait := range def.Traits {
			basis := alchemyTraitUnitBasis(trait)
			if qty%basis != 0 {
				log.Printf("alchemy.combo reject=BASIS_INVALID item=%s trait=%s qty=%d basis=%d", itemID, trait.TraitID, qty, basis)
				return ErrAlchemyBasisInvalid
			}
			maxQty := s.alchemyMaterialMaxQuantity(itemID, trait, derived.Stability, modifiers)
			if qty > maxQty {
				log.Printf("alchemy.combo reject=OVERFLOW item=%s trait=%s qty=%d max=%d stability=%.4f", itemID, trait.TraitID, qty, maxQty, derived.Stability)
				return ErrAlchemyMaterialOverflow
			}
			break
		}
	}
	return nil
}

func (s *Store) alchemyMaterialsLevelDeficitFailDelta(materials map[string]int64, derived alchemyDerived) float64 {
	delta := 0.0
	for itemID := range materials {
		def := s.gameData.ItemTraitByItemID(itemID)
		if def == nil || len(def.Traits) == 0 {
			continue
		}
		for _, trait := range def.Traits {
			if next := s.alchemyLevelDeficitFailDelta(derived.Level, trait.MinAlchemyLevel); next > delta {
				delta = next
			}
			break
		}
	}
	return delta
}

func resolveAlchemyComboParamTemplates(value any, comboPower float64, comboMultiplier int64) any {
	switch node := value.(type) {
	case map[string]any:
		if coeffRaw, ok := node["alchemyValueCoeff"]; ok {
			base := numberFromTemplateNode(node["base"])
			return roundAlchemyValue(base+comboPower*numberFromTemplateNode(coeffRaw), 3)
		}
		if coeffRaw, ok := node["alchemyMultiplierCoeff"]; ok {
			base := numberFromTemplateNode(node["base"])
			return roundAlchemyValue(base+float64(comboMultiplier)*numberFromTemplateNode(coeffRaw), 3)
		}
		out := make(map[string]any, len(node))
		for key, child := range node {
			out[key] = resolveAlchemyComboParamTemplates(child, comboPower, comboMultiplier)
		}
		return out
	case []any:
		out := make([]any, 0, len(node))
		for _, child := range node {
			out = append(out, resolveAlchemyComboParamTemplates(child, comboPower, comboMultiplier))
		}
		return out
	default:
		return value
	}
}

func numberFromTemplateNode(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	case json.Number:
		n, err := v.Float64()
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return 0
		}
		return n
	default:
		n, ok := numericFromAny(value)
		if !ok {
			return 0
		}
		return n
	}
}

func extractAlchemyComboAttrMods(params map[string]any) map[string]float64 {
	out := map[string]float64{}
	raw, ok := params["attrMods"]
	if !ok || raw == nil {
		return out
	}
	switch nodes := raw.(type) {
	case map[string]float64:
		for key, value := range nodes {
			key = strings.TrimSpace(key)
			if key != "" {
				out[key] = roundAlchemyValue(value, 3)
			}
		}
	case map[string]any:
		for key, rawValue := range nodes {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if value, ok := numericFromAny(rawValue); ok {
				out[key] = roundAlchemyValue(value, 3)
			}
		}
	}
	return out
}

func (s *Store) matchAlchemyCombo(materials map[string]int64, comboRecipeID string) *matchedAlchemyCombo {
	matches := make([]matchedAlchemyCombo, 0)
	for idx := range s.gameData.AlchemyComboRecipes {
		combo := &s.gameData.AlchemyComboRecipes[idx]
		if combo.Disabled {
			continue
		}
		if comboRecipeID != "" && combo.ID != comboRecipeID {
			continue
		}
		if multiplier, ok := sameRatioInt64Map(materials, combo.Inputs); ok {
			matches = append(matches, matchedAlchemyCombo{Recipe: combo, Multiplier: multiplier})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Recipe.Priority != matches[j].Recipe.Priority {
			return matches[i].Recipe.Priority > matches[j].Recipe.Priority
		}
		return matches[i].Recipe.ID < matches[j].Recipe.ID
	})
	if len(matches) == 0 {
		return nil
	}
	return &matches[0]
}

type alchemyTransmuteResult struct {
	Costs   map[string]int64
	Rewards map[string]int64
}

func (s *Store) alchemyTransmuteOnceTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, instanceID int64, rule *catalog.AlchemyTransmuteRule, rng *rand.Rand, item QueueRuntimeItem, prepaid bool) (alchemyTransmuteResult, error) {
	if rule == nil {
		return alchemyTransmuteResult{}, ErrAlchemyArgsInvalid
	}
	def := s.gameData.ItemByID(itemID)
	if !s.itemAllowedForAlchemyTransmute(def) {
		return alchemyTransmuteResult{}, ErrItemNotRecyclable
	}
	derived, err := s.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return alchemyTransmuteResult{}, err
	}
	if !prepaid && instanceID > 0 {
		if err := s.LedgerConsumeInstanceTx(ctx, tx, userID, instanceID, itemID, 1, alchemyLedgerMeta(item, "inventory", "alchemy_transmute", InventoryAuditDirectionConsume, "alchemy_transmute", nil)); err != nil {
			return alchemyTransmuteResult{}, err
		}
	} else if !prepaid {
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, 1, alchemyLedgerMeta(item, "inventory", "alchemy_transmute", InventoryAuditDirectionConsume, "alchemy_transmute", nil)); err != nil {
			return alchemyTransmuteResult{}, err
		}
	}
	baseFail := s.alchemyLevelAdjustedFailChance(rule.FailChanceBase, derived.Level, minAlchemyLevelForTransmute(rule, def), 0)
	failChance := applyAlchemyMinimumLossFailChance(applyAlchemyStabilityToFailChance(baseFail, derived.Stability), rule.MinFailChance)
	if rng.Float64() < failChance {
		return alchemyTransmuteResult{Costs: map[string]int64{itemID: 1}, Rewards: map[string]int64{}}, nil
	}
	ratio := sampleLuckSkewed(rule.ReturnRatio.MinRatio, rule.ReturnRatio.MaxRatio, derived.Stability, rng)
	gold := int64(math.Floor(def.EconomyBaseValue * ratio))
	if gold <= 0 {
		gold = 1
	}
	rewards := map[string]int64{GoldCoinItemID: gold}
	if rng.Float64() < clampAlchemyFloat(rule.LuckyChanceBase+derived.LuckyChance, 0, 1) {
		tier := rollAlchemyTier(rule.LuckyTiers, derived.LuckyChance, rng)
		switch tier.Kind {
		case "gold_multiplier":
			multiplier := sampleLuckSkewed(1, math.Max(1, tier.Value), derived.LuckyChance*2, rng)
			next := int64(math.Floor(float64(gold) * multiplier))
			if next > gold {
				rewards[GoldCoinItemID] = next
			}
		}
	}
	mods, err := transmute.Resolve(alchemyOperationEffectSnapshot(item))
	if err != nil {
		return alchemyTransmuteResult{}, err
	}
	bonus := transmute.BonusGold(gold, mods.BonusRate)
	if rewards[GoldCoinItemID] > math.MaxInt64-bonus {
		return alchemyTransmuteResult{}, ErrAlchemyArgsInvalid
	}
	rewards[GoldCoinItemID] += bonus
	for rewardItemID, qty := range rewards {
		if err := s.LedgerGrantTemplateTx(ctx, tx, userID, rewardItemID, qty, alchemyLedgerMeta(item, "alchemy_transmute", "inventory", InventoryAuditDirectionGain, "alchemy_transmute_reward", map[string]any{"sourceItemId": itemID, "foodBonusGold": bonus})); err != nil {
			return alchemyTransmuteResult{}, err
		}
	}
	return alchemyTransmuteResult{Costs: map[string]int64{itemID: 1}, Rewards: rewards}, nil
}

func (s *Store) itemAllowedForAlchemyTransmute(def *catalog.Item) bool {
	if def == nil || def.Disabled || def.EconomyBaseValue <= 0 || def.ID == GoldCoinItemID || def.IsEquipment {
		return false
	}
	category := strings.TrimSpace(strings.ToLower(def.Category))
	switch category {
	case "equipment", "currency", "collectible", "quest", "map", "key", "alchemy_medium":
		return false
	}
	blockedTypes := map[string]bool{
		"currency": true, "collectible": true, "quest": true, "map": true, "key": true, "skillbook": true, "trigger_item": true,
	}
	for _, typ := range def.Type {
		if blockedTypes[strings.TrimSpace(strings.ToLower(typ))] {
			return false
		}
	}
	id := strings.TrimSpace(strings.ToLower(def.ID))
	if strings.HasPrefix(id, "map_") || strings.HasPrefix(id, "skillbook_") || strings.HasPrefix(id, "key_") || strings.HasPrefix(id, "world_boss") || strings.HasPrefix(id, "boss_") || strings.Contains(id, "rumor") {
		return false
	}
	return true
}

func rollAlchemyTier(tiers []catalog.AlchemyTransmuteTier, luckyChance float64, rng *rand.Rand) catalog.AlchemyTransmuteTier {
	weights, byID := alchemyTransmuteTierWeights(tiers, luckyChance)
	return byID[rollByWeight(weights, rng)]
}

func alchemyTransmuteTierWeights(tiers []catalog.AlchemyTransmuteTier, luckyChance float64) (map[string]float64, map[string]catalog.AlchemyTransmuteTier) {
	weights := map[string]float64{}
	byID := map[string]catalog.AlchemyTransmuteTier{}
	for _, tier := range tiers {
		if tier.Weight <= 0 || tier.ID == "" {
			continue
		}
		bonus := 1.0
		if tier.Kind == "gold_multiplier" && tier.Value > 1 {
			bonus += clampAlchemyFloat(luckyChance*1.8*(tier.Value-1), 0, 0.6)
		}
		weights[tier.ID] = tier.Weight * bonus
		byID[tier.ID] = tier
	}
	return weights, byID
}

func encodeCustomAffixID(effectType string, params map[string]any, value float64, precision int) string {
	payload := map[string]any{
		"effectType": strings.TrimSpace(effectType),
		"params":     sortMapDeep(params),
		"value":      roundAlchemyValue(value, precision),
	}
	raw, _ := json.Marshal(payload)
	sum := sha1.Sum(raw)
	return "custom:" + hex.EncodeToString(sum[:])[:12]
}

func (s *Store) computeAlchemyDisplayName(materials map[string]int64, comboNameKey string) (string, map[string]any) {
	if comboNameKey != "" {
		return comboNameKey, map[string]any{}
	}
	type materialName struct {
		ItemID    string
		RarityOrd int64
		PrefixKey string
		SuffixKey string
	}
	rows := make([]materialName, 0, len(materials))
	for itemID := range materials {
		trait := s.gameData.ItemTraitByItemID(itemID)
		item := s.gameData.ItemByID(itemID)
		nameAffix := s.gameData.ItemAlchemyNameAffix(itemID)
		if trait == nil || item == nil || nameAffix == nil {
			continue
		}
		rows = append(rows, materialName{ItemID: itemID, RarityOrd: s.rarityOrder(item.Rarity), PrefixKey: nameAffix.PrefixKey, SuffixKey: nameAffix.SuffixKey})
	}
	if len(rows) == 0 {
		return "alchemy.affix.template.single", map[string]any{}
	}
	if len(rows) == 1 {
		return "alchemy.affix.template.single", map[string]any{"prefixKey": rows[0].PrefixKey, "suffixKey": rows[0].SuffixKey}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RarityOrd != rows[j].RarityOrd {
			return rows[i].RarityOrd > rows[j].RarityOrd
		}
		return rows[i].ItemID < rows[j].ItemID
	})
	primary := rows[0]
	secondaries := append([]materialName{}, rows[1:]...)
	sort.Slice(secondaries, func(i, j int) bool {
		if secondaries[i].RarityOrd != secondaries[j].RarityOrd {
			return secondaries[i].RarityOrd < secondaries[j].RarityOrd
		}
		return secondaries[i].ItemID < secondaries[j].ItemID
	})
	prefixKeys := make([]string, 0, len(secondaries))
	for _, row := range secondaries {
		prefixKeys = append(prefixKeys, row.PrefixKey)
	}
	return "alchemy.affix.template.multi", map[string]any{"prefixKeys": prefixKeys, "suffixKey": primary.SuffixKey}
}

func (s *Store) maxMaterialRarity(materials map[string]int64) string {
	best := "common"
	bestOrder := int64(-1)
	for itemID := range materials {
		item := s.gameData.ItemByID(itemID)
		if item == nil {
			continue
		}
		order := s.rarityOrder(item.Rarity)
		if order > bestOrder {
			best = item.Rarity
			bestOrder = order
		}
	}
	return best
}

func (s *Store) rarityOrder(rarityID string) int64 {
	for _, row := range s.gameData.RarityDefinitions {
		if row.ID == rarityID {
			return row.Order
		}
	}
	return 0
}

func (s *Store) markAlchemyDiscoveredTx(ctx context.Context, tx pgx.Tx, userID int64, kind, id string) error {
	kind = strings.TrimSpace(kind)
	id = strings.TrimSpace(id)
	if kind == "" || id == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO alchemy_discoveries(user_id, kind, id, discovered_at)
		VALUES($1, $2, $3, now())
		ON CONFLICT (user_id, kind, id) DO NOTHING
	`, userID, kind, id)
	return err
}

func (s *Store) markAlchemyTraitsForItemTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string) {
	def := s.gameData.ItemTraitByItemID(itemID)
	if def == nil {
		return
	}
	for _, trait := range def.Traits {
		_ = s.markAlchemyDiscoveredTx(ctx, tx, userID, "trait", trait.TraitID)
	}
}

func (s *Store) GetAlchemyDiscoveries(ctx context.Context, userID int64) (AlchemyDiscoveries, error) {
	rows, err := s.db.Query(ctx, `
		SELECT kind, id
		FROM alchemy_discoveries
		WHERE user_id = $1
		ORDER BY kind ASC, id ASC
	`, userID)
	if err != nil {
		return AlchemyDiscoveries{}, err
	}
	defer rows.Close()
	out := AlchemyDiscoveries{Groups: []string{}, Traits: []string{}, Combos: []string{}}
	for rows.Next() {
		var kind, id string
		if err := rows.Scan(&kind, &id); err != nil {
			return AlchemyDiscoveries{}, err
		}
		switch strings.TrimSpace(kind) {
		case "group":
			out.Groups = append(out.Groups, id)
		case "trait":
			out.Traits = append(out.Traits, id)
		case "combo":
			out.Combos = append(out.Combos, id)
		}
	}
	return out, rows.Err()
}

func mergeInt64Map(dst, src map[string]int64) {
	for key, value := range src {
		if value != 0 {
			dst[key] += value
		}
	}
}

func cloneMap(src map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range src {
		out[key] = value
	}
	return out
}

func sortMapDeep(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(typed))
		for _, key := range keys {
			out[key] = sortMapDeep(typed[key])
		}
		return out
	case map[string]float64:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]float64, len(typed))
		for _, key := range keys {
			out[key] = typed[key]
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for idx := range typed {
			out[idx] = sortMapDeep(typed[idx])
		}
		return out
	default:
		return typed
	}
}

func stableValueSum(values map[string]float64) float64 {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	total := 0.0
	for _, key := range keys {
		total += values[key]
	}
	return total
}

func sameRatioInt64Map(a, b map[string]int64) (int64, bool) {
	if len(a) != len(b) {
		return 0, false
	}
	multiplier := int64(0)
	for key, av := range a {
		bv := b[key]
		if av <= 0 || bv <= 0 || av%bv != 0 {
			return 0, false
		}
		ratio := av / bv
		if ratio <= 0 {
			return 0, false
		}
		if multiplier == 0 {
			multiplier = ratio
		} else if multiplier != ratio {
			return 0, false
		}
	}
	if multiplier <= 0 {
		return 0, false
	}
	return multiplier, true
}

func roundAlchemyValue(value float64, precision int) float64 {
	if precision < 0 {
		precision = 0
	}
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

func (s *Store) roundAlchemyAttributeValue(attr string, value float64) float64 {
	precision := 3
	attr = strings.TrimSpace(attr)
	for _, definition := range s.gameData.Attributes {
		if definition.ID == attr && strings.EqualFold(strings.TrimSpace(definition.Unit), "percent") {
			precision = 6
			break
		}
	}
	return roundAlchemyValue(value, precision)
}
