package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fishingLifecycleRow func(...any) error

func (row fishingLifecycleRow) Scan(dest ...any) error { return row(dest...) }

type fishingLifecycleTx struct {
	pgx.Tx
	queryRow func(string, []any) pgx.Row
	exec     func(string, []any) (pgconn.CommandTag, error)
}

func (tx fishingLifecycleTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	return tx.queryRow(query, args)
}

func (tx fishingLifecycleTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return tx.exec(query, args)
}

func TestFishingInvalidRuntimeCannotAuthorizeDeletionAfterCleanupFailure(t *testing.T) {
	wantErr := errors.New("trip read failed")
	for _, offline := range []bool{false, true} {
		tx := fishingLifecycleTx{queryRow: func(query string, args []any) pgx.Row {
			if !strings.Contains(query, "queue_id = $2") || !strings.Contains(query, "user_id = $1") ||
				!strings.Contains(query, "status = 'running'") || !strings.Contains(query, "FOR UPDATE") {
				t.Fatalf("unsafe trip cleanup query: %s", query)
			}
			if args[0] != int64(3034) || args[1] != int64(411304) {
				t.Fatalf("cleanup owner = %v", args)
			}
			return fishingLifecycleRow(func(...any) error { return wantErr })
		}}
		item := QueueRuntimeItem{QueueID: 411304, PayloadRaw: []byte(`{"executorId":"fishing_trip","runtime":{"durationMs":2300}}`)}
		e := newFishingExecutor(&Store{})
		var result SettleResult
		var err error
		if offline {
			result, err = e.SettleFastForwardTx(context.Background(), tx, 3034, item, time.Now(), FastForwardBudget{})
		} else {
			result, err = e.SettleNormalTx(context.Background(), tx, 3034, item, time.Now())
		}
		if !errors.Is(err, wantErr) || result.CancelAfterSettle || result.Progressed || result.Completed != 0 {
			t.Fatalf("offline=%t: result=%+v err=%v", offline, result, err)
		}
	}
}

func TestFishingCancelDoesNotTrustPayloadTripID(t *testing.T) {
	for _, raw := range []string{`{`, `{}`, `{"executorId":"fishing_trip","runtime":{"tripId":"another-trip","spotId":"spot","durationMs":1000,"phase":"pending"}}`} {
		read := false
		tx := fishingLifecycleTx{queryRow: func(query string, args []any) pgx.Row {
			read = true
			if args[0] != int64(3034) || args[1] != int64(411304) {
				t.Fatalf("cleanup used payload instead of queue owner: %v", args)
			}
			return fishingLifecycleRow(func(...any) error { return pgx.ErrNoRows })
		}}
		err := newFishingExecutor(&Store{}).CancelRunTx(context.Background(), tx, 3034, QueueRuntimeItem{QueueID: 411304, PayloadRaw: []byte(raw)})
		if err != nil || !read {
			t.Fatalf("cancel payload %s: read=%t err=%v", raw, read, err)
		}
	}
}

func TestFishingRuntimeCheckpointRejectsMissingOrFinishedTrip(t *testing.T) {
	for _, finished := range []bool{false, true} {
		tx := fishingLifecycleTx{exec: func(query string, args []any) (pgconn.CommandTag, error) {
			for _, fragment := range []string{"status = 'running'", "queue_id = $13", "actor_type = $14", "actor_id = $15"} {
				if !strings.Contains(query, fragment) {
					t.Fatalf("checkpoint missing %q: %s", fragment, query)
				}
			}
			if args[12] != int64(411304) || args[13] != ActionActorTypeKitty || args[14] != int64(2333) {
				t.Fatalf("checkpoint owner: %v", args[12:])
			}
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}}
		err := newFishingExecutor(&Store{}).saveFishingTripRuntimeTx(context.Background(), tx, 3034, 411304,
			fishingExecutorRuntime{TripID: "trip", ActorType: ActionActorTypeKitty, ActorID: 2333}, finished, time.Now())
		if err == nil || !strings.Contains(err.Error(), "checkpoint rejected") {
			t.Fatalf("finished=%t: err=%v", finished, err)
		}
	}
}

func TestFishingRuntimeSettlementValidatesActorPhaseAndClock(t *testing.T) {
	base := fishingExecutorRuntime{ActorType: ActionActorTypeKitty, ActorID: 2333,
		Phase: fishingPhaseWaiting, PhaseStartedAtMS: 1_000, PhaseEndsAtMS: 2_000}
	item := QueueRuntimeItem{ActorType: ActionActorTypeKitty, ActorID: 2333}
	for _, phase := range []string{fishingPhaseWaiting, fishingPhaseBite, fishingPhaseFighting, fishingPhaseLanded, fishingPhaseResting} {
		runtime := base
		runtime.Phase = phase
		if !fishingRuntimeCanSettle(runtime, item) {
			t.Fatalf("rejected valid phase %s", phase)
		}
	}
	for name, mutate := range map[string]func(*fishingExecutorRuntime){
		"pending":           func(r *fishingExecutorRuntime) { r.Phase = "pending" },
		"finished":          func(r *fishingExecutorRuntime) { r.Phase = fishingPhaseFinished },
		"unknown phase":     func(r *fishingExecutorRuntime) { r.Phase = "unknown" },
		"wrong actor":       func(r *fishingExecutorRuntime) { r.ActorID++ },
		"wrong lane":        func(r *fishingExecutorRuntime) { r.ActorType = ActionActorTypePlayer },
		"returned supplies": func(r *fishingExecutorRuntime) { r.Supplies.Returned = true },
		"missing start":     func(r *fishingExecutorRuntime) { r.PhaseStartedAtMS = 0 },
		"reversed clock":    func(r *fishingExecutorRuntime) { r.PhaseEndsAtMS = r.PhaseStartedAtMS - 1 },
		"zero duration":     func(r *fishingExecutorRuntime) { r.PhaseEndsAtMS = r.PhaseStartedAtMS },
	} {
		t.Run(name, func(t *testing.T) {
			runtime := base
			mutate(&runtime)
			if fishingRuntimeCanSettle(runtime, item) {
				t.Fatal("accepted invalid runtime")
			}
		})
	}
}

func TestFishingPhaseQueueWindowPreservesExecutorClock(t *testing.T) {
	now := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC)
	for _, duration := range []time.Duration{time.Second, 3 * time.Second, 90 * time.Second} {
		result := SettleResult{Progressed: true, NextRunStartAt: now.Add(-time.Hour), NextRunEndsAt: now.Add(-time.Hour).Add(duration)}
		for _, mode := range [][2]bool{{false, false}, {true, false}, {true, true}} {
			start, end := nextQueueRunWindow(now, result, nil, mode[0], mode[1])
			if !start.Equal(result.NextRunStartAt) || !end.Equal(result.NextRunEndsAt) {
				t.Fatalf("mode=%v duration=%v: phase window shifted to %v..%v", mode, duration, start, end)
			}
		}
	}
}
