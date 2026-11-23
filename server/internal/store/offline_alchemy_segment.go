package store

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/transmute"
)

const offlineAlchemySegmentLimit = 1024

// Legacy single completions use independent wall-clock seeds, not the queue
// sampler. Keep that policy; the optional explicit entropy input makes old/new
// execution comparable without replacing random rewards with averages.
func alchemySingleRunRNG(item QueueRuntimeItem, ordinal int) *rand.Rand {
	return rand.New(rand.NewSource(alchemySingleRunSeed(item, ordinal)))
}

func alchemySingleRunSeed(item QueueRuntimeItem, ordinal int) int64 {
	if item.alchemyRandomSeed != nil {
		return item.alchemyRandomSeed(ordinal)
	}
	return time.Now().UnixNano()
}

type alchemySegmentRoll struct {
	failed bool
	itemID string
	qty    int64
	bonus  int64
}

type alchemySegmentProgram struct {
	input, groupID            string
	costs                     map[string]int64
	failChance, upgradeChance float64
	roll                      func(*rand.Rand) (alchemySegmentRoll, error)
}

// Compile only contracts whose rewards cannot feed their costs and whose
// grants merge into ordinary stacks. Instance creation and feedback recipes
// need an ordered inventory working set and deliberately stay on the oracle.
func (e *alchemyExecutor) compileAlchemySegmentTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (alchemySegmentProgram, bool, error) {
	p := alchemySegmentProgram{}
	derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return p, false, err
	}
	stackable := func(id string) bool {
		return e.store.gameData.ItemByID(id) != nil && e.store.itemStackable(id) && len(e.store.templateDefaultAffixSlots(id)) == 0
	}
	switch e.actionID {
	case "alchemy.convert":
		var args alchemyConvertArgs
		if parseAlchemyRunContext(item.PayloadRaw, &args) != nil {
			return p, false, nil
		}
		group := e.store.gameData.AlchemyGroupByID(args.GroupID)
		if group == nil {
			group = e.store.gameData.AlchemyGroupByItemID(args.SourceItemID)
		}
		if group == nil || !alchemyGroupContainsEntry(group, args.SourceItemID) {
			return p, false, nil
		}
		p.input, p.groupID = args.SourceItemID, group.ID
		if p.input == GoldCoinItemID {
			return p, false, nil
		}
		p.costs = map[string]int64{p.input: 1}
		if group.GoldCost > 0 {
			p.costs[GoldCoinItemID] += group.GoldCost
		}
		weights := e.store.adjustAlchemyConvertWeights(p.input, group.Entries, derived.LuckyChance, false)
		upgrades := e.store.adjustAlchemyConvertWeights("", group.UpgradeEntries, derived.LuckyChance, true)
		for _, candidates := range []map[string]float64{weights, upgrades} {
			for id := range candidates {
				if p.costs[id] > 0 || !stackable(id) {
					return p, false, nil
				}
			}
		}
		p.failChance = e.store.alchemyConvertFailChance(group, derived)
		p.failChance, err = alchemyeffect.ConversionFailureChance(p.failChance, alchemyOperationEffectSnapshot(item))
		if err != nil {
			return p, false, nil
		}
		p.upgradeChance = alchemyConvertUpgradeChance(group, upgrades)
		rollNormal, rollUpgrade := compileAlchemySegmentWeights(weights), compileAlchemySegmentWeights(upgrades)
		p.roll = func(rng *rand.Rand) (alchemySegmentRoll, error) {
			if rng.Float64() < p.failChance {
				return alchemySegmentRoll{failed: true}, nil
			}
			reward := ""
			if len(upgrades) > 0 && rng.Float64() < p.upgradeChance {
				reward = rollUpgrade(rng)
			}
			if reward == "" {
				reward = rollNormal(rng)
			}
			if reward == "" {
				return alchemySegmentRoll{}, ErrAlchemyGroupNotFound
			}
			return alchemySegmentRoll{itemID: reward, qty: 1}, nil
		}
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		if parseAlchemyRunContext(item.PayloadRaw, &args) != nil || args.InstanceID != 0 {
			return p, false, nil
		}
		rule := e.store.gameData.AlchemyTransmuteRuleForItemID(args.ItemID)
		def := e.store.gameData.ItemByID(args.ItemID)
		if rule == nil || !e.store.itemAllowedForAlchemyTransmute(def) || !stackable(GoldCoinItemID) {
			return p, false, nil
		}
		p.input, p.costs = args.ItemID, map[string]int64{args.ItemID: 1}
		baseFail := e.store.alchemyLevelAdjustedFailChance(rule.FailChanceBase, derived.Level, minAlchemyLevelForTransmute(rule, def), 0)
		p.failChance = applyAlchemyMinimumLossFailChance(applyAlchemyStabilityToFailChance(baseFail, derived.Stability), rule.MinFailChance)
		weights, tiers := alchemyTransmuteTierWeights(rule.LuckyTiers, derived.LuckyChance)
		rollTier := compileAlchemySegmentWeights(weights)
		mods, err := transmute.Resolve(alchemyOperationEffectSnapshot(item))
		if err != nil {
			return p, false, nil
		}
		p.roll = func(rng *rand.Rand) (alchemySegmentRoll, error) {
			if rng.Float64() < p.failChance {
				return alchemySegmentRoll{failed: true}, nil
			}
			ratio := sampleLuckSkewed(rule.ReturnRatio.MinRatio, rule.ReturnRatio.MaxRatio, derived.Stability, rng)
			gold := int64(math.Floor(def.EconomyBaseValue * ratio))
			if gold <= 0 {
				gold = 1
			}
			baseGold := gold
			if rng.Float64() < clampAlchemyFloat(rule.LuckyChanceBase+derived.LuckyChance, 0, 1) {
				tier := tiers[rollTier(rng)]
				if tier.Kind == "gold_multiplier" {
					multiplier := sampleLuckSkewed(1, math.Max(1, tier.Value), derived.LuckyChance*2, rng)
					if next := int64(math.Floor(float64(gold) * multiplier)); next > gold {
						gold = next
					}
				}
			}
			bonus := transmute.BonusGold(baseGold, mods.BonusRate)
			if gold > math.MaxInt64-bonus {
				return alchemySegmentRoll{}, ErrAlchemyArgsInvalid
			}
			return alchemySegmentRoll{itemID: GoldCoinItemID, qty: gold + bonus, bonus: bonus}, nil
		}
	default:
		return p, false, nil
	}
	return p, true, nil
}

func compileAlchemySegmentWeights(weights map[string]float64) func(*rand.Rand) string {
	keys := make([]string, 0, len(weights))
	for key, weight := range weights {
		if weight > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return func(rng *rand.Rand) string {
		// Retain rollByWeight's summation and draw semantics; only eliminate
		// per-draw key allocation/sorting, not floating-point operations.
		total := 0.0
		for _, weight := range weights {
			if weight > 0 {
				total += weight
			}
		}
		if total <= 0 {
			return ""
		}
		needle, sum := rng.Float64()*total, 0.0
		for _, key := range keys {
			sum += weights[key]
			if needle <= sum {
				return key
			}
		}
		return ""
	}
}

type alchemySegmentExperience struct {
	afterExp []float64
	versions []int64
	level    int64
}

// Preserve every original r4 addition, cap and version increment. The first
// level-changing completion is left to the original executor, which also
// recomputes attributes and refreshes other queue durations.
func (s *Store) planAlchemySegmentExperienceTx(ctx context.Context, tx pgx.Tx, userID int64, gain float64, count int) (alchemySegmentExperience, error) {
	p := alchemySegmentExperience{}
	var exp float64
	err := tx.QueryRow(ctx, `SELECT attr_exp FROM player_attributes WHERE user_id=$1 AND attr_key='alchemy_level' FOR UPDATE`, userID).Scan(&exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	progress, err := s.resolveAttributeProgress("alchemy_level", exp)
	if err != nil {
		return p, err
	}
	p.level = progress.Level
	p.afterExp, p.versions = make([]float64, 0, count), make([]int64, 0, count)
	gain = roundFloat(gain, 4)
	var versions int64
	for i := 0; i < count; i++ {
		after := exp
		if gain != 0 {
			after, err = s.attributeExpProfile.clampTotalExp("alchemy_level", roundFloat(exp+gain, 4))
			if err != nil {
				return p, err
			}
			after = roundFloat(after, 4)
		}
		progress, err = s.resolveAttributeProgress("alchemy_level", after)
		if err != nil {
			return p, err
		}
		if progress.Level != p.level {
			break
		}
		if math.Abs(after-exp) >= 1e-9 {
			versions++
		}
		exp = after
		p.afterExp, p.versions = append(p.afterExp, exp), append(p.versions, versions)
	}
	return p, nil
}

// trySettleExactSegmentTx never mutates on an unsupported contract. Once a
// write starts any error aborts the enclosing recovery transaction; callers
// must not fall back after an error. It only replaces the frozen-effect
// SINGLE-operation path, never the legacy aggregate sampler.
func (e *alchemyExecutor) trySettleExactSegmentTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, bool, error) {
	if item.ActorType != ActionActorTypePlayer || !item.PreserveScheduledTimeline || item.WorkLoadoutRuntime != nil ||
		(e.actionID != "alchemy.convert" && e.actionID != "alchemy.transmute") || budget.MaxIterations == 1 {
		return SettleResult{}, false, nil
	}
	snapshot, frozen := queueOperationEffectSnapshot(item.PayloadRaw)
	if !frozen || item.OperationEffects != nil {
		return SettleResult{}, false, nil
	}
	timing := resolveQueueRunTiming(item, now, parseQueueRuntimeDuration(item.PayloadRaw))
	count, _ := completedRunsForQueueTiming(timing, now, item.TotalCount-item.DoneCount)
	count = min(count, offlineAlchemySegmentLimit)
	if budget.MaxIterations > 0 {
		count = min(count, budget.MaxIterations)
	}
	probe := enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, completedAtForQueueTiming(timing, 1), item.DoneCount+2, item.PayloadRaw), item.PayloadRaw)
	if count < 2 || item.EffectSession.HasMatchingCrossActorCharge(probe, now) {
		return SettleResult{}, false, nil
	}
	perStart := len(snapshot.Rules) > 0
	var stableRules []operationeffect.RuleSnapshot
	if perStart {
		stableRules = item.EffectSession.Prepare(probe).Snapshot().Rules
		if !alchemySegmentRulesEqual(snapshot.Rules, stableRules) {
			return SettleResult{}, false, nil
		}
		count = min(count, 1+item.EffectSession.StableStartCount(probe, now, timing.NextDuration, count-1))
	} else if item.EffectSession.HasMatching(probe, now) {
		return SettleResult{}, false, nil
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return SettleResult{}, false, err
	}
	// Do not jump over another running lane, even at a tie. Within this
	// interval the original scheduler would select this same row repeatedly.
	var unknownBoundary bool
	var otherBoundary *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(bool_or(q.current_run_ends_at IS NULL),false),min(q.current_run_ends_at)
		FROM player_action_queue q
		WHERE q.user_id=$1 AND q.id<>$2 AND q.status='running'
		  AND NOT(q.actor_type='kitty' AND EXISTS(
		    SELECT 1 FROM player_kitties k WHERE k.user_id=q.user_id AND k.id=q.actor_id AND k.status=$3))
	`, userID, item.QueueID, KittyStatusInCombat).Scan(&unknownBoundary, &otherBoundary); err != nil {
		return SettleResult{}, false, err
	}
	if unknownBoundary {
		return SettleResult{}, false, nil
	}
	if otherBoundary != nil {
		count = sort.Search(count, func(i int) bool { return !completedAtForQueueTiming(timing, i+1).Before(*otherBoundary) })
	}
	if count < 2 {
		return SettleResult{}, false, nil
	}
	runtime := parseAlchemyRuntime(item.PayloadRaw)
	for key := range runtime.ExtraAttrExp {
		if key != "alchemy_level" {
			return SettleResult{}, false, nil
		}
	}
	nextPayload, duration, err := e.refreshNextRuntimePayloadTx(ctx, tx, userID, item, e.clearReservedPayload(item.PayloadRaw), completedAtForQueueTiming(timing, count))
	if err != nil {
		return SettleResult{}, false, err
	}
	effectiveDuration := duration
	if e.actionID == "alchemy.transmute" {
		mods, err := transmute.Resolve(snapshot)
		if err != nil {
			return SettleResult{}, false, nil
		}
		if mods.DurationSeconds > 0 {
			effectiveDuration = time.Second
		}
	}
	if effectiveDuration != timing.NextDuration || !reflect.DeepEqual(runtime.ExtraAttrExp, parseAlchemyRuntime(nextPayload).ExtraAttrExp) {
		return SettleResult{}, false, nil
	}
	program, supported, err := e.compileAlchemySegmentTx(ctx, tx, userID, item)
	if err != nil || !supported {
		return SettleResult{}, false, err
	}
	reserved := parseAlchemyReservedCosts(item.PayloadRaw)
	prepaid := 0
	if len(reserved) > 0 {
		if !reflect.DeepEqual(reserved, program.costs) {
			return SettleResult{}, false, nil
		}
		prepaid = 1
	}
	if perStart && prepaid != 1 {
		return SettleResult{}, false, nil
	}
	affordable, err := e.store.maxAffordableActionStartsTx(ctx, tx, userID, program.costs, int64(count-prepaid))
	if err != nil {
		return SettleResult{}, false, err
	}
	count = min(count, int(affordable)+prepaid)
	if count < 2 {
		return SettleResult{}, false, nil
	}
	experience, err := e.store.planAlchemySegmentExperienceTx(ctx, tx, userID, runtime.ExtraAttrExp["alchemy_level"], count)
	if err != nil {
		return SettleResult{}, false, err
	}
	count = len(experience.afterExp)
	if count < 2 {
		return SettleResult{}, false, nil
	}
	rolls := make([]alchemySegmentRoll, 0, count)
	rewards := map[string]int64{}
	var rewardOrder []string
	began := time.Now()
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < count; i++ {
		if shouldYieldAlchemyFastForwardBudget(began, budget, i) {
			break
		}
		// Reseeding retains independent legacy streams without allocating a
		// fresh multi-kilobyte RNG state for every completed operation.
		rng.Seed(alchemySingleRunSeed(item, item.DoneCount+i+1))
		roll, err := program.roll(rng)
		if err != nil {
			// No state has been written. Preserve the original executor's
			// cancellation/refund behavior for invalid-target outcomes.
			if _, stop := classifyAlchemyStopError(err); stop {
				return SettleResult{}, false, nil
			}
			return SettleResult{}, false, err
		}
		rolls = append(rolls, roll)
		if roll.qty > 0 {
			if rewards[roll.itemID] > math.MaxInt64-roll.qty {
				return SettleResult{}, false, ErrInventoryQuantityOverflow
			}
			if rewards[roll.itemID] == 0 {
				rewardOrder = append(rewardOrder, roll.itemID)
			}
			rewards[roll.itemID] += roll.qty
		}
	}
	count = len(rolls)
	if count < 2 {
		return SettleResult{}, false, nil
	}
	costs, err := checkedScaleInt64Map(program.costs, int64(count))
	if err != nil {
		return SettleResult{}, false, err
	}
	// Database work scales with touched templates, not completed operations.
	for id, qty := range program.costs {
		if err := e.store.ConsumeByTemplateTx(ctx, tx, userID, id, qty*int64(count-prepaid)); err != nil {
			return SettleResult{}, false, err
		}
	}
	for _, id := range rewardOrder {
		if err := e.store.GrantByTemplateTx(ctx, tx, userID, id, rewards[id]); err != nil {
			return SettleResult{}, false, err
		}
	}
	if experience.versions[count-1] > 0 {
		if _, err := tx.Exec(ctx, `UPDATE player_attributes SET attr_exp=$2,base_level=$3,lock_version=lock_version+$4,updated_at=now() WHERE user_id=$1 AND attr_key='alchemy_level'`, userID, experience.afterExp[count-1], experience.level, experience.versions[count-1]); err != nil {
			return SettleResult{}, false, err
		}
	}
	for i, roll := range rolls {
		run := item
		run.DoneCount += i
		if perStart && i > 0 {
			op := queueOperationEffectOperation(operationeffect.DomainAlchemy, run, completedAtForQueueTiming(timing, i), run.DoneCount+1, item.PayloadRaw)
			op.Capabilities = probe.Capabilities
			prepared := item.EffectSession.Prepare(op)
			if !reflect.DeepEqual(stableRules, prepared.Snapshot().Rules) {
				return SettleResult{}, false, operationeffect.ErrPreparedOperationStale
			}
			var err error
			snapshot, err = item.EffectSession.Commit(prepared)
			if err != nil {
				return SettleResult{}, false, err
			}
			for id, qty := range program.costs {
				if err := e.store.ledgerLogTx(ctx, tx, userID, id, qty, alchemyLedgerMeta(run, "inventory", "action_queue", InventoryAuditDirectionConsume, "alchemy_reserve_cost", map[string]any{"itemId": id})); err != nil {
					return SettleResult{}, false, err
				}
			}
		}
		if err := e.logAlchemySegmentRunTx(ctx, tx, userID, run, program, roll, perStart || i == 0 && prepaid == 1); err != nil {
			return SettleResult{}, false, err
		}
	}
	if e.actionID == "alchemy.convert" && len(rewardOrder) > 0 {
		if err := e.store.markAlchemyDiscoveredTx(ctx, tx, userID, "group", program.groupID); err != nil {
			return SettleResult{}, false, err
		}
		for _, id := range rewardOrder {
			if def := e.store.gameData.ItemTraitByItemID(id); def != nil {
				for _, trait := range def.Traits {
					if err := e.store.markAlchemyDiscoveredTx(ctx, tx, userID, "trait", trait.TraitID); err != nil {
						return SettleResult{}, false, err
					}
				}
			}
		}
	}
	doneAfter := item.DoneCount + count
	at := completedAtForQueueTiming(timing, count)
	cancelAfter, cancelReason := false, ""
	if doneAfter == item.TotalCount {
		nextPayload = e.clearReservedPayload(item.PayloadRaw)
	}
	if perStart {
		nextPayload, err = setQueueOperationEffectSnapshot(nextPayload, snapshot)
		if err != nil {
			return SettleResult{}, false, err
		}
		if doneAfter < item.TotalCount {
			refreshed := parseAlchemyRuntime(nextPayload)
			nextPayload, cancelAfter, cancelReason, err = e.prepareNextAlchemyEffectRunTx(ctx, tx, userID, item, nextPayload, at, doneAfter, &refreshed)
			if err != nil {
				return SettleResult{}, false, err
			}
			if !cancelAfter {
				duration = time.Duration(parseAlchemyRuntime(nextPayload).DurationMs) * time.Millisecond
			}
		} else {
			duration = timing.NextDuration
		}
	}
	singleExp := scaleAttrExp(runtime.ExtraAttrExp, 1)
	result := SettleResult{
		Completed: count, DoneAfter: doneAfter, CompletedAt: at,
		NextRunStartAt: at, NextRunEndsAt: at.Add(duration), NextPayload: nextPayload,
		ItemCosts: costs, ItemRewards: rewards, exactSingleAttrExp: singleExp,
		CancelAfterSettle: cancelAfter, CancelReason: cancelReason,
	}
	result.AttrExp = map[string]float64{}
	accumulateSettlementAttrExp(result.AttrExp, result, false)
	return result, true, nil
}

func alchemySegmentRulesEqual(left, right []operationeffect.RuleSnapshot) bool {
	if len(left) != len(right) {
		return false
	}
	for i, l := range left {
		r := right[i]
		if !operationEffectParamsEqual(l.Params, r.Params) {
			return false
		}
		l.Params, r.Params = nil, nil
		if !reflect.DeepEqual(l, r) {
			return false
		}
	}
	return true
}

func (e *alchemyExecutor) logAlchemySegmentRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, p alchemySegmentProgram, roll alchemySegmentRoll, prepaid bool) error {
	log := func(id string, qty int64, source, target string, direction InventoryAuditDirection, kind string, details map[string]any) error {
		return e.store.ledgerLogTx(ctx, tx, userID, id, qty, alchemyLedgerMeta(item, source, target, direction, kind, details))
	}
	if e.actionID == "alchemy.transmute" {
		if !prepaid {
			if err := log(p.input, 1, "inventory", "alchemy_transmute", InventoryAuditDirectionConsume, "alchemy_transmute", nil); err != nil {
				return err
			}
		}
		if roll.qty > 0 {
			return log(roll.itemID, roll.qty, "alchemy_transmute", "inventory", InventoryAuditDirectionGain, "alchemy_transmute_reward", map[string]any{"sourceItemId": p.input, "foodBonusGold": roll.bonus})
		}
		return nil
	}
	if !prepaid {
		if err := log(p.input, 1, "inventory", "alchemy_convert", InventoryAuditDirectionConsume, "alchemy_convert", map[string]any{"groupId": p.groupID, "failed": roll.failed, "failChance": p.failChance}); err != nil {
			return err
		}
		if qty := p.costs[GoldCoinItemID]; qty > 0 {
			if err := log(GoldCoinItemID, qty, "inventory", "alchemy_convert", InventoryAuditDirectionConsume, "alchemy_convert_gold", map[string]any{"groupId": p.groupID, "sourceItemId": p.input, "failed": roll.failed, "failChance": p.failChance}); err != nil {
				return err
			}
		}
	}
	if roll.qty > 0 {
		return log(roll.itemID, roll.qty, "alchemy_convert", "inventory", InventoryAuditDirectionGain, "alchemy_convert_reward", map[string]any{"groupId": p.groupID, "sourceItemId": p.input, "upgradeChance": p.upgradeChance, "failChance": p.failChance})
	}
	return nil
}

func accumulateSettlementAttrExp(dst map[string]float64, result SettleResult, skipZero bool) {
	gains, repeats := result.AttrExp, 1
	if result.exactSingleAttrExp != nil {
		gains, repeats = result.exactSingleAttrExp, result.Completed
	}
	for key, gain := range gains {
		if skipZero && gain == 0 {
			continue
		}
		for i := 0; i < repeats; i++ {
			dst[key] += gain
		}
	}
}
