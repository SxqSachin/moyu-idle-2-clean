package store

import (
	"strconv"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
)

func (e *enhanceExecutor) OperationEffectDomain() operationeffect.Domain {
	return operationeffect.DomainEnhancement
}
func (e *fishingExecutor) OperationEffectDomain() operationeffect.Domain {
	return operationeffect.DomainFishingCast
}

func applyEnhanceOperationEffects(run *enhanceRunContext, snapshot operationeffect.Snapshot) error {
	bonus, err := ratebonus.Resolve(snapshot, ratebonus.EnhanceSuccess)
	if err != nil {
		return err
	}
	if bonus > 0 {
		run.BaseSuccessRate, run.SuccessRateBonus, run.SuccessRate = splitEnhanceSuccessRates(run.BaseSuccessRate, run.SuccessRateBonus+bonus)
	}
	return nil
}

func nextEnhanceOperationEffects(item QueueRuntimeItem, timing queueRunTiming, now time.Time, completed int) (operationeffect.Snapshot, error) {
	op := queueOperationEffectOperation(operationeffect.DomainEnhancement, item, operationEffectSubsequentStartAt(item, timing, now, completed), item.DoneCount+completed+1, item.PayloadRaw)
	if item.EffectSession == nil {
		return operationeffect.EmptySnapshot(op), nil
	}
	return item.EffectSession.Commit(item.EffectSession.Prepare(op))
}

func freezeFishingCastEffects(runtime *fishingExecutorRuntime, at time.Time) error {
	runtime.CastEffects = nil
	runtime.CastPassive = nil
	if runtime.effectSession == nil {
		return nil
	}
	op := operationeffect.Operation{
		Key:    "fishing_cast:" + runtime.TripID + ":" + strconv.FormatInt(runtime.Summary.Casts, 10) + ":" + strconv.FormatInt(at.UnixMilli(), 10),
		Domain: operationeffect.DomainFishingCast, ActionID: FishingTripActionID,
		ActorKind: operationeffect.ActorPlayer, StartedAt: at,
	}
	if normalizeActorType(runtime.ActorType) == ActionActorTypeKitty {
		op.ActorKind, op.ActorID = operationeffect.ActorKitty, runtime.ActorID
	}
	passive := runtime.effectSession.Passive().Snapshot(op)
	if len(passive.Sources) > 0 {
		runtime.CastPassive = &passive
	}
	snapshot, err := runtime.effectSession.Commit(runtime.effectSession.Prepare(op))
	if err != nil {
		return err
	}
	if len(snapshot.Rules) > 0 {
		runtime.CastEffects = &snapshot
	}
	return nil
}
