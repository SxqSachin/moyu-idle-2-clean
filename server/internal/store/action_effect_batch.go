package store

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

// Only independent, cost-free player work opts in. Refund-funded starts,
// inherited/rolled instances, other executors and cross-actor progress retain
// their exact sequential path. New effect kinds must be explicitly reviewed.
func primaryWorkEffectBatchEligible(item QueueRuntimeItem) bool {
	if item.ActorType != ActionActorTypePlayer || !item.PreserveScheduledTimeline ||
		!lifeaction.IsPrimaryWork(item.Spec.MainType) || len(item.Spec.Costs) != 0 || len(item.Spec.MaterialInstanceRequirements) != 0 {
		return false
	}
	if p := item.EffectSession.Passive(); p != nil && p.ProgressOrdering {
		return false
	}
	runtime, ok := parseGenericExecutorRuntimeEnvelope(item.PayloadRaw)
	if !ok || runtime.UseInstanceBindings || len(runtime.ExtraAttrExp) != 0 {
		return false
	}
	if run, _ := parseGenericRunContext(item.PayloadRaw); run.UseInstanceBindings ||
		len(run.MaterialInstanceBindings) != 0 || run.InheritFromInstanceID != 0 || run.InheritedEnhanceLevel != 0 ||
		len(run.RefundTemplateCosts) != 0 || len(run.RefundEnhancedCosts) != 0 {
		return false
	}
	for _, reward := range item.Spec.ItemReward {
		if reward.Count == nil || *reward.Count < 0 || reward.Range != nil || reward.Percent != nil || reward.GrantPolicy != "" || len(reward.DefaultAffixRarityWeights) != 0 {
			return false
		}
	}
	for key, gain := range item.Spec.StatusExpReward {
		if key != item.Spec.MainType || math.IsNaN(gain) || math.IsInf(gain, 0) || gain < 0 {
			return false
		}
	}
	return true
}

func primaryWorkBatchSnapshotSupported(snapshot operationeffect.Snapshot) bool {
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainLifeAction {
			return false
		}
		switch rule.Kind {
		case lifeaction.KindExtraMainOutputChance, lifeaction.KindHarvestBonus, lifeaction.KindFocusedHarvest, lifeaction.KindPatientHarvest, lifeaction.KindWorkExperience:
		default:
			return false
		}
	}
	return true
}

func (e *genericExecutor) primaryWorkEffectBatchLimitTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, timing queueRunTiming, through time.Time, limit int) (int, error) {
	if limit <= 1 || !timing.HasCurrentRun || !primaryWorkEffectBatchEligible(item) {
		return 1, nil
	}
	count, _ := completedRunsForQueueTiming(timing, through, item.TotalCount-item.DoneCount)
	limit = min(limit, count, 512)
	if limit <= 1 {
		return 1, nil
	}
	for id := range item.Spec.ItemReward {
		definition := e.store.gameData.ItemByID(id)
		if definition == nil || !definition.Stackable || definition.IsEquipment {
			return 1, nil
		}
	}
	if item.Spec.MainOutput != "" {
		definition := e.store.gameData.ItemByID(item.Spec.MainOutput)
		if definition == nil || !definition.Stackable || definition.IsEquipment {
			return 1, nil
		}
	}
	current, _ := queueOperationEffectSnapshot(item.PayloadRaw)
	nextStart := operationEffectSubsequentStartAt(item, timing, through, 1)
	op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, nextStart, item.DoneCount+2, item.PayloadRaw)
	next := item.EffectSession.Prepare(op).Snapshot()
	if !primaryWorkBatchSnapshotSupported(current) || !primaryWorkBatchSnapshotSupported(next) {
		return 1, nil
	}
	limit = item.EffectSession.StableStartCount(op, through, timing.NextDuration, limit)
	if limit <= 1 {
		return 1, nil
	}
	runtime, _ := parseGenericExecutorRuntimeEnvelope(item.PayloadRaw)
	base := time.Duration(item.Spec.DurationS) * time.Second
	if runtime.BaseDurationMs > 0 {
		base = time.Duration(runtime.BaseDurationMs) * time.Millisecond
	}
	duration, extraExp, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, base, nextStart)
	if err != nil {
		return 0, err
	}
	if len(extraExp) != 0 {
		return 1, nil
	}
	duration, err = workMealDuration(duration, next)
	if err != nil {
		return 0, err
	}
	if time.Duration(duration.Milliseconds())*time.Millisecond != timing.NextDuration {
		return 1, nil
	}
	firstGain, err := primaryWorkRoundedGain(item.Spec, current)
	if err != nil {
		return 0, err
	}
	nextGain, err := primaryWorkRoundedGain(item.Spec, next)
	if err != nil {
		return 0, err
	}
	if firstGain == 0 && nextGain == 0 {
		return limit, nil
	}
	var exp float64
	err = tx.QueryRow(ctx, `SELECT attr_exp FROM player_attributes WHERE user_id=$1 AND attr_key=$2 FOR UPDATE`, userID, item.Spec.MainType).Scan(&exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	progress, err := e.store.resolveAttributeProgress(item.Spec.MainType, exp)
	if err != nil {
		return 0, err
	}
	return primaryWorkBeforeLevelLimit(exp, progress, firstGain, nextGain, limit), nil
}

// Leave the level-changing completion to the original path, which recomputes
// derived stats and starts the successor using the newly authoritative speed.
func primaryWorkBeforeLevelLimit(exp float64, progress attributeProgressNode, firstGain, nextGain float64, limit int) int {
	if progress.NextExp <= 0 {
		return limit
	}
	needed := progress.NextExp - progress.Exp
	virtual := exp
	for i := 0; i < limit; i++ {
		gain := nextGain
		if i == 0 {
			gain = firstGain
		}
		virtual = roundFloat(virtual+gain, 4)
		if virtual-exp+attributeExpEpsilon >= needed {
			return max(1, i)
		}
	}
	return limit
}

func primaryWorkRoundedGain(spec ActionSpec, snapshot operationeffect.Snapshot) (float64, error) {
	mods, err := lifeaction.WorkMealModifiers(snapshot)
	if err != nil {
		return 0, err
	}
	base := spec.StatusExpReward[spec.MainType]
	return roundFloat(base+base*mods.ExperienceBonusRate, 4), nil
}

func primaryWorkBatchRoundedExperience(spec ActionSpec, snapshots []operationeffect.Snapshot) (map[string]float64, error) {
	out := map[string]float64{}
	resolver := lifeaction.Resolver{}
	for _, snapshot := range snapshots {
		mods, err := resolver.WorkMealModifiers(snapshot)
		if err != nil {
			return nil, err
		}
		base := spec.StatusExpReward[spec.MainType]
		gain := roundFloat(base+base*mods.ExperienceBonusRate, 4)
		out[spec.MainType] = roundFloat(out[spec.MainType]+gain, 4)
	}
	return out, nil
}
