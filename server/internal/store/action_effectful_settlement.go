package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
)

func (e *genericExecutor) needsEffectfulGenericSettlement(item QueueRuntimeItem, timing queueRunTiming, now time.Time) bool {
	if p := item.EffectSession.Passive(); p != nil && p.ProgressOrdering && item.ActorType == ActionActorTypePlayer {
		return true
	}
	if snapshot, ok := queueOperationEffectSnapshot(item.PayloadRaw); ok && len(snapshot.Rules) > 0 {
		return true
	}
	probe := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, timing.StartAt, item.DoneCount+1, item.PayloadRaw)
	return item.EffectSession != nil && item.EffectSession.HasMatching(probe, now)
}

// Settle the current operation before testing affordability of its successor.
// In particular, refunded materials must be available to that next start.
func (e *genericExecutor) settleEffectfulGenericTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, timing queueRunTiming) (SettleResult, error) {
	terminal := item
	terminal.TotalCount = item.DoneCount + 1
	result, err := e.SettleFastForwardTx(ctx, tx, userID, terminal, now, FastForwardBudget{MaxIterations: 1})
	if err != nil || result.Completed == 0 {
		return result, err
	}
	startedAt := operationEffectSubsequentStartAt(item, timing, now, 1)
	next := item
	next.DoneCount = result.DoneAfter
	op := queueOperationEffectOperation(operationeffect.DomainLifeAction, next, startedAt, next.DoneCount+1, next.PayloadRaw)
	snapshot := operationeffect.EmptySnapshot(op)
	var prepared *operationeffect.PreparedOperation
	if item.EffectSession != nil {
		p := item.EffectSession.Prepare(op)
		prepared = &p
		snapshot = p.Snapshot()
	}
	next.OperationEffects = &snapshot
	if _, err := tx.Exec(ctx, `SAVEPOINT effectful_action_next`); err != nil {
		return SettleResult{}, err
	}
	payload, startErr := e.StartRunTx(ctx, tx, userID, next, startedAt)
	if startErr != nil {
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT effectful_action_next`); err != nil {
			return SettleResult{}, errors.Join(startErr, err)
		}
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT effectful_action_next`); err != nil {
			return SettleResult{}, err
		}
		cancel, reason := e.ClassifyStartError(startErr)
		if !cancel {
			return SettleResult{}, startErr
		}
		result.CancelAfterSettle = true
		result.CancelReason = reason
		return result, nil
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT effectful_action_next`); err != nil {
		return SettleResult{}, err
	}
	if prepared != nil {
		snapshot, err = item.EffectSession.Commit(*prepared)
		if err != nil {
			return SettleResult{}, err
		}
	}
	result.NextPayload, err = setQueueOperationEffectSnapshot(payload, snapshot)
	if err != nil {
		return SettleResult{}, err
	}
	result.NextPayload, err = freezeQueuePassive(next, result.NextPayload, startedAt, operationeffect.DomainLifeAction)
	if err != nil {
		return SettleResult{}, err
	}
	result.NextRunStartAt = startedAt
	result.NextRunEndsAt = startedAt.Add(parseQueueRuntimeDuration(payload))
	result.ItemCosts = cloneCostMap(item.Spec.Costs)
	return result, nil
}
