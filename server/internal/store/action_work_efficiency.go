package store

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
)

func workTimingSnapshot(item QueueRuntimeItem, at time.Time) (operationeffect.PassiveSnapshot, error) {
	empty := operationeffect.PassiveSnapshot{Version: 1}
	if item.ActorType != ActionActorTypeKitty || !operationeffect.WorkEfficiencyMainType(item.Spec.MainType) {
		return empty, nil
	}
	if !at.IsZero() {
		return item.EffectSession.Passive().Snapshot(passiveOperation(item, operationeffect.DomainActionStart, at)), nil
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(item.PayloadRaw, &envelope); err != nil {
		return empty, err
	}
	if envelope.TimingEffects == nil {
		return empty, nil
	}
	return *envelope.TimingEffects, envelope.TimingEffects.Validate()
}

// A zero timestamp means a refresh of the current run: retain its frozen
// furniture contract even when equipment triggers a duration refresh.
func (s *Store) resolveWorkDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, base time.Duration, at time.Time) (time.Duration, map[string]float64, error) {
	snapshot, err := workTimingSnapshot(item, at)
	if err != nil {
		return 0, nil, err
	}
	if item.ActorType == ActionActorTypePlayer && item.WorkLoadoutRuntime != nil && item.WorkLoadoutRuntime.Available && len(item.WorkLoadoutRuntime.AttributeTotals) > 0 {
		return s.resolveDurationAndBonusWithTotals(base, item.Spec, item.WorkLoadoutRuntime.AttributeTotals)
	}
	return s.resolveDurationAndBonusTx(ctx, tx, userID, item.ActorType, item.ActorID, item.Spec, base, float64(snapshot.EfficiencyBPS())/10000)
}

func setWorkQueueRuntime(item QueueRuntimeItem, raw []byte, runtime any, at time.Time) (json.RawMessage, error) {
	payload, err := setQueueRuntimePayload(raw, runtime)
	if err != nil {
		return nil, err
	}
	if item.ActorType != ActionActorTypeKitty || !operationeffect.WorkEfficiencyMainType(item.Spec.MainType) {
		return payload, nil
	}
	snapshot, err := workTimingSnapshot(item, at)
	if err != nil {
		return nil, err
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}
	envelope.TimingEffects = nil
	if len(snapshot.Sources) > 0 {
		envelope.TimingEffects = &snapshot
	}
	return json.Marshal(envelope)
}

// Keep the bulk fast path for a constant contract. Stop just before the first
// changed start; only that boundary requires starting a new timed operation.
func workEfficiencyBatchLimit(item QueueRuntimeItem, timing queueRunTiming, now time.Time, limit int) (int, bool, error) {
	if item.ActorType != ActionActorTypeKitty || !operationeffect.WorkEfficiencyMainType(item.Spec.MainType) {
		return limit, false, nil
	}
	old, err := workTimingSnapshot(item, time.Time{})
	if err != nil {
		return 0, false, err
	}
	at := operationEffectSubsequentStartAt(item, timing, now, 1)
	next, err := workTimingSnapshot(item, at)
	if err != nil {
		return 0, false, err
	}
	if old.EfficiencyBPS() != next.EfficiencyBPS() {
		return 1, true, nil
	}
	boundary := item.EffectSession.Passive().NextEfficiencyBoundary(passiveOperation(item, operationeffect.DomainActionStart, at))
	if boundary.IsZero() || boundary.After(now) {
		return limit, false, nil
	}
	count, _ := completedRunsForQueueTiming(timing, now, item.TotalCount-item.DoneCount)
	if limit > 0 && count > limit {
		count = limit
	}
	index := sort.Search(count, func(i int) bool {
		return !operationEffectSubsequentStartAt(item, timing, now, i+1).Before(boundary)
	}) + 1
	if index <= count {
		return index - 1, false, nil
	}
	return limit, false, nil
}
