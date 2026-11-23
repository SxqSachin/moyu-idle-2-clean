package store

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"sort"
	"time"
)

// Reuses the existing attribute locks and reads. Capture accepted quantities
// only for effect-bearing settlements, without extra database queries.
func (s *Store) applyAttributedExperienceTx(ctx context.Context, tx pgx.Tx, userID int64, total, owner map[string]float64, effects *operationeffect.Session) error {
	p := effects.Passive()
	if p == nil || len(p.PendingExperience) == 0 {
		return s.applyAttrExpTx(ctx, tx, userID, total)
	}
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	accepted := map[string]float64{}
	if _, err := s.applyAttrExpLockedResultTx(ctx, tx, userID, total, accepted); err != nil {
		return err
	}
	return p.CommitExperience(total, accepted, owner)
}

// Do not batch across an owner's next completion, including a tie with the
// request's cutoff. The scheduler retains its normal queue-order tie breaker.
func passiveProgressSettlementLimit(actor string, runEnd, ownerEnd *time.Time, through time.Time, limit int) (time.Time, int) {
	if actor == ActionActorTypePlayer {
		return through, 1
	}
	if ownerEnd != nil && !ownerEnd.After(through) {
		if runEnd != nil && runEnd.Before(*ownerEnd) {
			return ownerEnd.Add(-time.Nanosecond), limit
		}
		return through, 1
	}
	return through, limit
}

func passiveOperation(item QueueRuntimeItem, domain operationeffect.Domain, at time.Time) operationeffect.Operation {
	kind := operationeffect.ActorPlayer
	if item.ActorType == ActionActorTypeKitty {
		kind = operationeffect.ActorKitty
	}
	return operationeffect.Operation{Domain: domain, ActorKind: kind, ActorID: item.ActorID, StartedAt: at, ActionID: item.ActionID, MainType: item.Spec.MainType, SubType: item.Spec.SubType}
}

func setQueuePassiveSnapshot(raw []byte, snapshot operationeffect.PassiveSnapshot) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	envelope.PassiveEffects = nil
	if len(snapshot.Sources) > 0 {
		envelope.PassiveEffects = &snapshot
	}
	return json.Marshal(envelope)
}

func queuePassiveSnapshot(raw []byte) (operationeffect.PassiveSnapshot, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return operationeffect.PassiveSnapshot{}, err
	}
	if envelope.PassiveEffects == nil {
		return operationeffect.PassiveSnapshot{Version: 1}, nil
	}
	return *envelope.PassiveEffects, envelope.PassiveEffects.Validate()
}

func freezeQueuePassive(item QueueRuntimeItem, raw []byte, at time.Time, domain operationeffect.Domain) (json.RawMessage, error) {
	// Fishing owns a per-cast snapshot, not a trip-long envelope snapshot.
	if domain == operationeffect.DomainFishingCast || domain == operationeffect.DomainNone || item.ActorType != ActionActorTypeKitty || item.EffectSession.Passive() == nil {
		return json.RawMessage(raw), nil
	}
	return setQueuePassiveSnapshot(raw, item.EffectSession.Passive().Snapshot(passiveOperation(item, domain, at)))
}

func shareActionExperience(item QueueRuntimeItem, source map[string]float64, timing queueRunTiming, now time.Time, completed int) (map[string]float64, error) {
	out := map[string]float64{}
	if item.ActorType != ActionActorTypeKitty || completed <= 0 {
		return out, nil
	}
	first, err := queuePassiveSnapshot(item.PayloadRaw)
	if err != nil {
		return nil, err
	}
	p := item.EffectSession.Passive()
	if p == nil {
		_, err = p.Share(first, source)
		return out, err
	}
	// Only service boundaries can change an otherwise identical work series.
	// Binary search their operation indices rather than iterating every action.
	cuts := []int{0, 1, completed}
	for _, w := range p.Windows {
		for _, boundary := range []time.Time{w.From, w.Until} {
			index := sort.Search(completed-1, func(i int) bool {
				return !operationEffectSubsequentStartAt(item, timing, now, i+1).Before(boundary)
			}) + 1
			if index < completed {
				cuts = append(cuts, index)
			}
		}
	}
	sort.Ints(cuts)
	for j := 0; j+1 < len(cuts); j++ {
		index, end := cuts[j], cuts[j+1]
		if index == end {
			continue
		}
		snapshot := first
		if index > 0 {
			snapshot = p.Snapshot(passiveOperation(item, operationeffect.DomainLifeAction, operationEffectSubsequentStartAt(item, timing, now, index)))
		}
		gain, err := p.ShareRepeated(snapshot, source, int64(end-index))
		if err != nil {
			return nil, err
		}
		out = mergeAttrExp(out, gain)
	}
	return out, nil
}
