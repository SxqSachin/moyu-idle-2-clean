package store

import (
	"encoding/json"
	"mysecondwebgame/server/internal/operationeffect"
	"time"
)

// Frozen per-start metadata construction before batch template reuse.
func prepareGenericOperationEffectSnapshotsReference(item QueueRuntimeItem, timing queueRunTiming, settledAt time.Time, completed, startsToCharge, doneAfter int, cancelAfterSettle bool) ([]operationeffect.Snapshot, json.RawMessage, error) {
	if completed <= 0 {
		return nil, item.PayloadRaw, nil
	}
	current, hasCurrentSnapshot := queueOperationEffectSnapshot(item.PayloadRaw)
	if !item.EffectSession.HasGrants() && (!hasCurrentSnapshot || len(current.Rules) == 0) {
		return nil, item.PayloadRaw, nil
	}
	snapshots := make([]operationeffect.Snapshot, completed)
	if hasCurrentSnapshot {
		snapshots[0] = current
	} else {
		// Missing means the operation predates this system. Freeze it as empty;
		// never resolve a newly eaten effect at settlement time.
		snapshots[0] = operationeffect.EmptySnapshot(queueOperationEffectOperation(operationeffect.DomainLifeAction, item, timing.StartAt, item.DoneCount+1, item.PayloadRaw))
	}
	nextPayload := json.RawMessage(item.PayloadRaw)
	for index := 0; index < startsToCharge; index++ {
		startedAt := operationEffectSubsequentStartAt(item, timing, settledAt, index+1)
		operation := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, startedAt, item.DoneCount+2+index, item.PayloadRaw)
		snapshot := operationeffect.EmptySnapshot(operation)
		if item.EffectSession != nil {
			var err error
			snapshot, err = item.EffectSession.Commit(item.EffectSession.Prepare(operation))
			if err != nil {
				return nil, nil, err
			}
		}
		if index+1 < completed {
			snapshots[index+1] = snapshot
		}
		if index == startsToCharge-1 && doneAfter < item.TotalCount && !cancelAfterSettle {
			var err error
			nextPayload, err = setQueueOperationEffectSnapshot(nextPayload, snapshot)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return snapshots, nextPayload, nil
}
