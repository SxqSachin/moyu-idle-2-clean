package store

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

type ordinaryLedgerKey struct {
	kind, item, definition string
	grant                  int64
}
type ordinaryLedgerEntry struct {
	quantity    int64
	hits        int
	first, last string
}

func addOrdinaryQuantity(out map[string]int64, id string, quantity int64) error {
	if quantity < 0 || out[id] > math.MaxInt64-quantity {
		return ErrInventoryQuantityOverflow
	}
	if quantity > 0 {
		out[id] += quantity
	}
	return nil
}

func ordinarySnapshotSupported(snapshot operationeffect.Snapshot) bool {
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainLifeAction {
			return false
		}
		switch rule.Kind {
		case lifeaction.KindMaterialRefundChance, lifeaction.KindExtraMainOutputChance, lifeaction.KindHarvestBonus, lifeaction.KindFocusedHarvest, lifeaction.KindPatientHarvest, lifeaction.KindWorkExperience:
		default:
			return false
		}
	}
	return true
}

func ordinarySameRules(a, b []operationeffect.RuleSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		left, right := a[i], b[i]
		var lp, rp any
		if json.Unmarshal(left.Params, &lp) != nil || json.Unmarshal(right.Params, &rp) != nil || !reflect.DeepEqual(lp, rp) {
			return false
		}
		left.Params, right.Params = nil, nil
		if !reflect.DeepEqual(left, right) {
			return false
		}
	}
	return true
}

func (e *genericExecutor) ordinaryLimitTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget, timing queueRunTiming, snapshot operationeffect.Snapshot, state *ordinarySettlementState) (int, error) {
	count, _ := completedRunsForQueueTiming(timing, now, item.TotalCount-item.DoneCount)
	limit := min(count, ordinarySegmentLimit)
	if budget.MaxIterations > 0 {
		limit = min(limit, budget.MaxIterations)
	}
	if limit <= 1 {
		return limit, nil
	}
	if !item.PreserveScheduledTimeline || !ordinarySnapshotSupported(snapshot) {
		return 1, nil
	}
	// Inventory eligibility remains authoritative at each real start. A catalog
	// edit must not make frozen grants reusable only inside a long batch.
	for id, rule := range state.Grants {
		if rule.Commodity != !e.store.itemHasRandomDefaultAffixGrant(id) ||
			rule.Durability != e.store.defaultMaxDurability(id) ||
			!sameAffixSlots(rule.DefaultAffixes, e.store.templateDefaultAffixSlots(id)) {
			return 1, nil
		}
	}
	if p := item.EffectSession.Passive(); p != nil && (p.ProgressOrdering || p.AffectsOwnerProgress()) {
		return 1, nil
	}
	run, _ := parseGenericRunContext(item.PayloadRaw)
	if run.UseInstanceBindings {
		return 1, nil
	}
	var worstInstances int64
	for id, reward := range item.Spec.ItemReward {
		if state.Grants[id].Stackable {
			continue
		}
		n := int64(0)
		if reward.Count != nil {
			n = *reward.Count
		} else if reward.Range != nil {
			n = reward.Range.Max
		}
		if n > ordinarySegmentLimit {
			return 1, nil
		}
		worstInstances += n
	}
	// The bound includes all food outputs, even if their probability is tiny.
	if item.Spec.MainOutput != "" && !state.Grants[item.Spec.MainOutput].Stackable {
		all := lifeaction.Resolver{Sample: func(operationeffect.RuleSnapshot, float64) bool { return true }}
		outcome, err := all.Resolve(snapshot, item.Spec.Costs, item.Spec.MainOutput)
		if err != nil {
			return 0, err
		}
		for _, extra := range outcome.ExtraOutputs {
			worstInstances += extra.Quantity
		}
	}
	if worstInstances > 0 {
		limit = min(limit, max(1, int(ordinarySegmentLimit/worstInstances)))
	}
	if item.OtherRunEnd != nil && !item.OtherRunEnd.After(now) {
		if !timing.StartAt.Add(timing.CurrentDuration).Before(*item.OtherRunEnd) {
			return 1, nil
		}
		n, _ := completedRunsForQueueTiming(timing, item.OtherRunEnd.Add(-time.Nanosecond), limit)
		limit = min(limit, max(1, n))
	}
	limit, changed, err := workEfficiencyBatchLimit(item, timing, now, limit)
	if err != nil {
		return 0, err
	}
	if changed || limit <= 1 {
		return 1, nil
	}
	start := operationEffectSubsequentStartAt(item, timing, now, 1)
	op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, start, item.DoneCount+2, item.PayloadRaw)
	next := item.EffectSession.Prepare(op).Snapshot()
	if !ordinarySameRules(snapshot.Rules, next.Rules) {
		return 1, nil
	}
	limit = min(limit, 1+item.EffectSession.StableOrderedStartCount(op, now, timing.NextDuration, limit-1))
	runtime := parseOrdinaryRuntime(item.PayloadRaw)
	base := time.Duration(item.Spec.DurationS) * time.Second
	if runtime.BaseDurationMs > 0 {
		base = time.Duration(runtime.BaseDurationMs) * time.Millisecond
	}
	duration, extra, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, base, start)
	if err != nil {
		return 0, err
	}
	duration, err = workMealDuration(duration, next)
	if err != nil {
		return 0, err
	}
	if time.Duration(duration.Milliseconds())*time.Millisecond != timing.NextDuration || !maps.Equal(extra, runtime.ExtraAttrExp) {
		return 1, nil
	}
	return limit, nil
}

func (e *genericExecutor) settleOrdinaryTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget, state *ordinarySettlementState, fish *fishFoodExecutor) (SettleResult, error) {
	if item.DoneCount < state.FirstOrdinal {
		return SettleResult{}, errors.New("ordinary settlement ordinal precedes checkpoint")
	}
	runtime := parseOrdinaryRuntime(item.PayloadRaw)
	timing := resolveQueueRunTiming(item, now, runtime.Duration())
	snapshot, frozen := queueOperationEffectSnapshot(item.PayloadRaw)
	if !frozen {
		snapshot = operationeffect.EmptySnapshot(queueOperationEffectOperation(operationeffect.DomainLifeAction, item, timing.StartAt, item.DoneCount+1, item.PayloadRaw))
	}
	run, _ := parseGenericRunContext(item.PayloadRaw)
	if run.UseInstanceBindings && item.TotalCount != item.DoneCount+1 {
		return SettleResult{}, ErrActionMaterialBindingInvalid
	}
	if !ordinarySnapshotSupported(snapshot) {
		return SettleResult{}, errors.New("unsupported ordinary effect checkpoint")
	}
	limit, err := e.ordinaryLimitTx(ctx, tx, userID, item, now, budget, timing, snapshot, state)
	if err != nil || limit <= 0 {
		return SettleResult{}, err
	}
	meal, err := workMealExperience(item.Spec, []operationeffect.Snapshot{snapshot})
	if err != nil {
		return SettleResult{}, err
	}
	sourceGain := mergeAttrExp(mergeAttrExp(item.Spec.StatusExpReward, runtime.ExtraAttrExp), meal)
	gain := sourceGain
	if item.ActorType == ActionActorTypeKitty {
		gain = scaleAttrExpByMultiplier(gain, e.store.config.KittyStatusExpMultiplier)
	}
	xp, err := e.store.planOrdinaryExperienceTx(ctx, tx, userID, gain, limit)
	if err != nil {
		return SettleResult{}, err
	}
	limit = xp.limit
	var fishInputs *ordinaryFishInputs
	if fish != nil {
		fishInputs, err = fish.loadOrdinaryFishInputsTx(ctx, tx, userID, item, limit)
		if err != nil {
			return SettleResult{}, err
		}
		limit = min(limit, 1+len(fishInputs.subsequent))
	}
	random := state.random()
	inv, err := e.store.loadOrdinaryInventoryTx(ctx, tx, userID, state, random)
	if err != nil {
		return SettleResult{}, err
	}
	result := SettleResult{ItemCosts: map[string]int64{}, ItemRewards: map[string]int64{}, AttrExp: map[string]float64{}}
	ledger := map[ordinaryLedgerKey]ordinaryLedgerEntry{}
	log := func(key ordinaryLedgerKey, qty int64, operation string) error {
		if qty <= 0 {
			return nil
		}
		entry := ledger[key]
		if entry.quantity > math.MaxInt64-qty {
			return ErrInventoryQuantityOverflow
		}
		entry.quantity += qty
		entry.hits++
		if entry.first == "" {
			entry.first = operation
		}
		entry.last = operation
		ledger[key] = entry
		return nil
	}
	ids := make([]string, 0, len(item.Spec.ItemReward))
	for id := range item.Spec.ItemReward {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	ordinal := item.DoneCount
	resolver := lifeaction.Resolver{Sample: func(rule operationeffect.RuleSnapshot, p float64) bool {
		return random.Hit(ordinaryEffectStream(rule.GrantID, rule.Kind), int64(ordinal), p)
	}}
	algebraic := len(snapshot.Rules) == 0 && !run.UseInstanceBindings
	for id, reward := range item.Spec.ItemReward {
		if !state.Grants[id].Stackable || reward.Count == nil || item.Spec.Costs[id] > 0 {
			algebraic = false
			break
		}
	}
	if algebraic {
		result.Completed = min(limit, 1+inv.affordableStarts(item.Spec.Costs, limit-1))
		result.CancelAfterSettle = result.Completed < limit
		entries, err := buildActionCostEntries(item.Spec.Costs, int64(result.Completed-1))
		if err != nil {
			return SettleResult{}, err
		}
		costs := map[string]int64{}
		for _, entry := range entries {
			costs[entry.ItemID] = entry.Quantity
		}
		if err := inv.consume(costs); err != nil {
			return SettleResult{}, err
		}
		for id, quantity := range costs {
			result.ItemCosts[id] = quantity
			if err := log(ordinaryLedgerKey{kind: "action_cost_batch", item: id}, quantity, snapshot.OperationKey); err != nil {
				return SettleResult{}, err
			}
		}
		for _, id := range ids {
			reward := item.Spec.ItemReward[id]
			p := 1.0
			if reward.Percent != nil {
				p = *reward.Percent
			}
			hits := random.Count("reward:"+id, int64(item.DoneCount), result.Completed, p)
			if hits > 0 && *reward.Count > math.MaxInt64/int64(hits) {
				return SettleResult{}, ErrInventoryQuantityOverflow
			}
			quantity := *reward.Count * int64(hits)
			if err := inv.grant(id, quantity, item.DoneCount, "base", false, false); err != nil {
				return SettleResult{}, err
			}
			if err := addOrdinaryQuantity(result.ItemRewards, id, quantity); err != nil {
				return SettleResult{}, err
			}
			if err := log(ordinaryLedgerKey{kind: "action_reward_batch", item: id}, quantity, snapshot.OperationKey); err != nil {
				return SettleResult{}, err
			}
		}
		for i := 0; i < result.Completed; i++ {
			for key, value := range gain {
				result.AttrExp[key] += value
			}
		}
		for key, entry := range ledger {
			entry.hits = result.Completed
			first := item.DoneCount + 1
			if key.kind == "action_cost_batch" {
				entry.hits--
				first++
			}
			firstStart := timing.StartAt
			if first > item.DoneCount+1 {
				firstStart = operationEffectSubsequentStartAt(item, timing, now, first-item.DoneCount-1)
			}
			lastStart := timing.StartAt
			if result.Completed > 1 {
				lastStart = operationEffectSubsequentStartAt(item, timing, now, result.Completed-1)
			}
			entry.first = queueOperationEffectOperation(operationeffect.DomainLifeAction, item, firstStart, first, item.PayloadRaw).Key
			entry.last = queueOperationEffectOperation(operationeffect.DomainLifeAction, item, lastStart, item.DoneCount+result.Completed, item.PayloadRaw).Key
			ledger[key] = entry
		}
	}
	for i := 0; !algebraic && i < limit; i++ {
		if err := ctx.Err(); err != nil {
			return SettleResult{}, err
		}
		ordinal = item.DoneCount + i
		if i > 0 {
			if !inv.affordable(item.Spec.Costs) {
				result.CancelAfterSettle = true
				break
			}
			if err := inv.consume(item.Spec.Costs); err != nil {
				return SettleResult{}, err
			}
			start := operationEffectSubsequentStartAt(item, timing, now, i)
			op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, start, ordinal+1, item.PayloadRaw)
			snapshot, err = item.EffectSession.Commit(item.EffectSession.Prepare(op))
			if err != nil {
				return SettleResult{}, err
			}
			for id, qty := range item.Spec.Costs {
				if err := addOrdinaryQuantity(result.ItemCosts, id, qty); err != nil {
					return SettleResult{}, err
				}
				if err := log(ordinaryLedgerKey{kind: "action_cost_batch", item: id}, qty, snapshot.OperationKey); err != nil {
					return SettleResult{}, err
				}
			}
		}
		for _, id := range ids {
			reward := item.Spec.ItemReward[id]
			qty := ordinaryRewardQuantity(random, id, ordinal, reward)
			if err := inv.grant(id, qty, ordinal, "base", reward.GrantPolicy == InventoryGrantPolicyRollDefaultAffixes, run.UseInstanceBindings); err != nil {
				return SettleResult{}, err
			}
			if err := addOrdinaryQuantity(result.ItemRewards, id, qty); err != nil {
				return SettleResult{}, err
			}
			if !run.UseInstanceBindings || state.Grants[id].EnhanceLevel <= 0 {
				if err := log(ordinaryLedgerKey{kind: "action_reward_batch", item: id}, qty, snapshot.OperationKey); err != nil {
					return SettleResult{}, err
				}
			}
		}
		outcome, err := resolver.Resolve(snapshot, item.Spec.Costs, item.Spec.MainOutput)
		if err != nil {
			return SettleResult{}, err
		}
		for _, refund := range outcome.Refunds {
			for id, qty := range refund.Items {
				if err := inv.grant(id, qty, ordinal, "refund", false, false); err != nil {
					return SettleResult{}, err
				}
				if err := addOrdinaryQuantity(result.ItemRewards, id, qty); err != nil {
					return SettleResult{}, err
				}
				if err := log(ordinaryLedgerKey{kind: "operation_effect_material_refund", item: id, definition: refund.DefinitionID, grant: refund.GrantID}, qty, snapshot.OperationKey); err != nil {
					return SettleResult{}, err
				}
			}
		}
		for _, extra := range outcome.ExtraOutputs {
			rolled := item.Spec.ItemReward[extra.ItemID].GrantPolicy == InventoryGrantPolicyRollDefaultAffixes
			if err := inv.grant(extra.ItemID, extra.Quantity, ordinal, "extra:"+strconv.FormatInt(extra.GrantID, 10), rolled, false); err != nil {
				return SettleResult{}, err
			}
			if err := addOrdinaryQuantity(result.ItemRewards, extra.ItemID, extra.Quantity); err != nil {
				return SettleResult{}, err
			}
			if err := log(ordinaryLedgerKey{kind: "operation_effect_extra_main_output", item: extra.ItemID, definition: extra.DefinitionID, grant: extra.GrantID}, extra.Quantity, snapshot.OperationKey); err != nil {
				return SettleResult{}, err
			}
		}
		result.Completed++
		for key, value := range gain {
			result.AttrExp[key] += value
		}
		if budget.MaxExec > 0 && !budget.StartedAt.IsZero() && time.Since(budget.StartedAt) >= budget.MaxExec {
			break
		}
	}
	if fishInputs != nil {
		if err := fishInputs.flushTx(ctx, tx, e.store, userID, item, result.Completed); err != nil {
			return SettleResult{}, err
		}
	}
	if err := inv.flushTx(ctx, tx, userID); err != nil {
		return SettleResult{}, err
	}
	for _, row := range inv.created {
		if row.enhanceLevel <= 0 {
			continue
		}
		if err := e.store.LedgerLogOnlyTx(ctx, tx, userID, row.item, 1, InventoryLedgerMeta{Category: InventoryAuditCategoryActionQueue, Direction: InventoryAuditDirectionGain, SubjectType: "queue", SubjectID: strconv.FormatInt(item.QueueID, 10), SourceSurface: "action_queue", TargetSurface: "inventory", InstanceID: &row.id, Details: map[string]any{"type": "action_reward_batch_inherited", "queueId": item.QueueID, "count": 1, "phase": item.AuditPhase, "inheritedEnhanceLvl": row.enhanceLevel, "settlementVersion": ordinarySettlementVersion}}); err != nil {
			return SettleResult{}, err
		}
	}
	if err := e.flushOrdinaryLedgerTx(ctx, tx, userID, item, ledger); err != nil {
		return SettleResult{}, err
	}
	if result.Completed == 1 {
		owner, err := shareActionExperience(item, sourceGain, timing, now, 1)
		if err != nil {
			return SettleResult{}, err
		}
		result.OwnerAttrExp = owner
		if err := e.store.applyAttributedExperienceTx(ctx, tx, userID, mergeAttrExp(gain, owner), owner, item.EffectSession); err != nil {
			return SettleResult{}, err
		}
	} else {
		if err := xp.flushTx(ctx, tx, userID, result.Completed); err != nil {
			return SettleResult{}, err
		}
		result.exactSingleAttrExp = gain
	}
	result.DoneAfter = item.DoneCount + result.Completed
	result.CompletedAt = completedAtForQueueTiming(timing, result.Completed)
	result.NextPayload = item.PayloadRaw
	result.CancelReason = ActionQueueCancelReasonCostInsufficient
	if result.DoneAfter < item.TotalCount && !result.CancelAfterSettle {
		var starter QueueActionExecutor = e
		if fish != nil {
			starter = fish
		}
		if err := e.startOrdinarySuccessorTx(ctx, tx, userID, item, now, timing, &result, starter); err != nil {
			return SettleResult{}, err
		}
	}
	return result, nil
}

func (e *genericExecutor) startOrdinarySuccessorTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, timing queueRunTiming, result *SettleResult, starter QueueActionExecutor) error {
	start := operationEffectSubsequentStartAt(item, timing, now, result.Completed)
	next := item
	next.DoneCount = result.DoneAfter
	op := queueOperationEffectOperation(operationeffect.DomainLifeAction, next, start, next.DoneCount+1, item.PayloadRaw)
	prepared := item.EffectSession.Prepare(op)
	snapshot := prepared.Snapshot()
	next.OperationEffects = &snapshot
	if _, err := tx.Exec(ctx, `SAVEPOINT ordinary_successor`); err != nil {
		return err
	}
	payload, startErr := starter.StartRunTx(ctx, tx, userID, next, start)
	if startErr != nil {
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT ordinary_successor`); err != nil {
			return errors.Join(startErr, err)
		}
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT ordinary_successor`); err != nil {
			return err
		}
		cancel, reason := starter.ClassifyStartError(startErr)
		if !cancel {
			return startErr
		}
		result.CancelAfterSettle, result.CancelReason = cancel, reason
		return nil
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT ordinary_successor`); err != nil {
		return err
	}
	snapshot, err := item.EffectSession.Commit(prepared)
	if err != nil {
		return err
	}
	result.NextPayload, err = setQueueOperationEffectSnapshot(payload, snapshot)
	if err != nil {
		return err
	}
	result.NextPayload, err = freezeQueuePassive(next, result.NextPayload, start, operationeffect.DomainLifeAction)
	if err != nil {
		return err
	}
	result.NextRunStartAt, result.NextRunEndsAt = start, start.Add(parseQueueRuntimeDuration(payload))
	for id, qty := range item.Spec.Costs {
		if err := addOrdinaryQuantity(result.ItemCosts, id, qty); err != nil {
			return err
		}
	}
	return nil
}

func (e *genericExecutor) flushOrdinaryLedgerTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, ledger map[ordinaryLedgerKey]ordinaryLedgerEntry) error {
	keys := make([]ordinaryLedgerKey, 0, len(ledger))
	for key := range ledger {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.item != b.item {
			return a.item < b.item
		}
		if a.grant != b.grant {
			return a.grant < b.grant
		}
		return a.definition < b.definition
	})
	for _, key := range keys {
		entry := ledger[key]
		meta := InventoryLedgerMeta{Category: InventoryAuditCategoryActionQueue, Direction: InventoryAuditDirectionGain, SubjectType: "queue", SubjectID: strconv.FormatInt(item.QueueID, 10), SourceSurface: "action_queue", TargetSurface: "inventory", Details: map[string]any{
			"type": key.kind, "queueId": item.QueueID, "actionId": item.ActionID, "phase": item.AuditPhase, "settlementVersion": ordinarySettlementVersion,
			"count": entry.hits, "firstOperationKey": entry.first, "lastOperationKey": entry.last,
		}}
		if key.kind == "action_cost_batch" {
			meta.Direction = InventoryAuditDirectionConsume
			meta.SourceSurface = "inventory"
			meta.TargetSurface = "action_queue"
		}
		if key.grant != 0 {
			meta.Category = InventoryAuditCategoryOperationEffect
			meta.SubjectType = "operation_effect"
			meta.SubjectID = key.definition
			meta.Details["grantId"] = key.grant
			meta.Details["triggeredOperations"] = entry.hits
			if key.kind == "operation_effect_material_refund" {
				meta.Direction = InventoryAuditDirectionRefund
			}
		}
		if err := e.store.LedgerLogOnlyTx(ctx, tx, userID, key.item, entry.quantity, meta); err != nil {
			return err
		}
	}
	return nil
}
