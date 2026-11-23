package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// These reads belong to one uninterrupted player conversion/transmutation lane
// inside the queue's user-locked transaction. They never survive a chunk,
// another executor, a new start/loadout, or a level-changing completion.
type offlineAlchemyReads struct {
	queueID      int64
	derived      *alchemyDerived
	baseDuration time.Duration
	duration     time.Duration
}

func (r *offlineAlchemyReads) invalidate() {
	if r != nil {
		r.derived = nil
		r.duration = 0
	}
}

func (e *alchemyExecutor) resolveSettlementWorkDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, base time.Duration, at time.Time) (time.Duration, map[string]float64, error) {
	reads := item.alchemyReads
	if item.WorkLoadoutRuntime != nil && item.WorkLoadoutRuntime.Available {
		reads = nil
	}
	if reads != nil && reads.duration > 0 && reads.baseDuration == base {
		return reads.duration, map[string]float64{}, nil
	}
	duration, extra, err := e.store.resolveWorkDurationTx(ctx, tx, userID, item, base, at)
	// Only base player timing is cached. Food is selected/applied per start,
	// and an unknown future source of bonus XP must retain the original path.
	if err == nil && reads != nil && len(extra) == 0 {
		reads.baseDuration, reads.duration = base, duration
	}
	return duration, extra, err
}

func (r *offlineAlchemyReads) forItem(item QueueRuntimeItem) *offlineAlchemyReads {
	eligible := item.ActorType == ActionActorTypePlayer && item.QueueID > 0
	switch item.ActionID {
	case "alchemy.convert":
	case "alchemy.transmute":
		var args alchemyTransmuteArgs
		// Instance consumption may detach equipment and change derived stats.
		eligible = eligible && parseAlchemyRunContext(item.PayloadRaw, &args) == nil && args.InstanceID == 0
	default:
		eligible = false
	}
	if !eligible || r.queueID != item.QueueID {
		*r = offlineAlchemyReads{}
	}
	if !eligible {
		return nil
	}
	r.queueID = item.QueueID
	return r
}
