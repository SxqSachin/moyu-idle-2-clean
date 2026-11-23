package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/transmute"
	"time"
)

// Only a server-frozen speed effect may bypass the legacy two-second floor.
// Expiry is deliberately not rechecked after this operation has started.
func normalizeQueueRuntimeDuration(duration time.Duration, payload []byte) time.Duration {
	if duration == time.Second {
		var envelope queueRuntimeEnvelope
		if json.Unmarshal(payload, &envelope) == nil && envelope.ExecutorID == "alchemy.transmute" {
			if snapshot, ok := queueOperationEffectSnapshot(payload); ok {
				if mods, err := transmute.Resolve(snapshot); err == nil && mods.DurationSeconds == 1 {
					return time.Second
				}
			}
		}
	}
	return normalizeActionQueueRunDuration(duration)
}

func (e *alchemyExecutor) enrichTransmuteOperation(op operationeffect.Operation, payload []byte) operationeffect.Operation {
	var args alchemyTransmuteArgs
	if parseAlchemyRunContext(payload, &args) != nil {
		return op
	}
	if op.Capabilities == nil {
		op.Capabilities = map[string]bool{}
	}
	op.Capabilities[transmute.GoldEligible] = e.store.transmuteFoodEligible(args.ItemID)
	return op
}

func (s *Store) transmuteFoodEligible(itemID string) bool {
	return s.gameData.AlchemyTransmuteRuleForItemID(itemID) != nil &&
		s.itemAllowedForAlchemyTransmute(s.gameData.ItemByID(itemID))
}

func (s *Store) TransmuteFoodEligible(_ context.Context, itemID string) (bool, error) {
	return s.transmuteFoodEligible(itemID), nil
}

func (e *alchemyExecutor) prepareNextAlchemyEffectRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, payload json.RawMessage, startedAt time.Time, doneAfter int, refreshed *parsedAlchemyRuntime) (json.RawMessage, bool, string, error) {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, false, "", err
	}
	probe := item
	probe.PayloadRaw = payload
	costs, err := e.alchemyRunCostsTx(ctx, tx, userID, probe)
	if err != nil {
		if reason, stop := classifyAlchemyStopError(err); stop {
			return payload, true, reason, nil
		}
		return nil, false, "", err
	}
	// Check the whole cost before any deduction or audit event is produced.
	affordable, err := e.store.maxAffordableActionStartsTx(ctx, tx, userID, costs, 1)
	if err != nil {
		return nil, false, "", err
	}
	if affordable < 1 {
		return payload, true, ActionQueueCancelReasonCostInsufficient, nil
	}
	op := e.EnrichOperationEffectContext(queueOperationEffectOperation(operationeffect.DomainAlchemy, item, startedAt, doneAfter+1, payload), payload)
	snapshot := operationeffect.EmptySnapshot(op)
	var prepared *operationeffect.PreparedOperation
	if item.EffectSession != nil {
		p := item.EffectSession.Prepare(op)
		prepared = &p
		snapshot = p.Snapshot()
	}
	next := item
	next.DoneCount = doneAfter
	next.PayloadRaw = payload
	next.RunStartAt = &startedAt
	next.OperationEffects = &snapshot
	// A failed next start must not retain a partially reserved multi-item cost.
	if _, err := tx.Exec(ctx, `SAVEPOINT alchemy_effect_next`); err != nil {
		return nil, false, "", err
	}
	result, err := e.startRunWithTimingTx(ctx, tx, userID, next, startedAt, refreshed)
	if err != nil {
		if _, rollbackErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT alchemy_effect_next`); rollbackErr != nil {
			return nil, false, "", errors.Join(err, rollbackErr)
		}
		if _, releaseErr := tx.Exec(ctx, `RELEASE SAVEPOINT alchemy_effect_next`); releaseErr != nil {
			return nil, false, "", errors.Join(err, releaseErr)
		}
		if reason, stop := classifyAlchemyStopError(err); stop {
			return payload, true, reason, nil
		}
		return nil, false, "", err
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT alchemy_effect_next`); err != nil {
		return nil, false, "", err
	}
	result, err = setQueueOperationEffectSnapshot(result, snapshot)
	if err != nil {
		return nil, false, "", err
	}
	if prepared != nil {
		if _, err = item.EffectSession.Commit(*prepared); err != nil {
			return nil, false, "", err
		}
	}
	return result, false, "", nil
}
