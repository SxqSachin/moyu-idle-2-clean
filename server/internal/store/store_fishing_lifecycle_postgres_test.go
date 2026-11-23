package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

// This opt-in test uses only transaction-local temporary tables. Excluding
// public from search_path makes an unexpected domain write fail, not hit game data.
func fishingLifecyclePostgresTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	dsn := os.Getenv("FISHING_LIFECYCLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set FISHING_LIFECYCLE_TEST_DATABASE_URL for isolated PostgreSQL tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `
		SET LOCAL search_path = pg_temp;
		CREATE TEMP TABLE player_action_queue(id bigint PRIMARY KEY);
		CREATE TEMP TABLE fishing_trips(
		 id text PRIMARY KEY, user_id bigint NOT NULL, queue_id bigint REFERENCES player_action_queue(id) ON DELETE SET NULL,
		 actor_type text DEFAULT 'kitty', actor_id bigint DEFAULT 2333,
		 status text DEFAULT 'running', phase text DEFAULT 'fighting',
		 loadout_snapshot_jsonb jsonb, supplies_jsonb jsonb, summary_jsonb jsonb,
		 current_fish_jsonb jsonb DEFAULT '{}', stop_reason text DEFAULT '', event_seq bigint DEFAULT 4,
		 started_at timestamptz DEFAULT now(), ended_at timestamptz, updated_at timestamptz DEFAULT now(),
		 phase_started_at timestamptz DEFAULT now(), phase_ends_at timestamptz DEFAULT now()
		);
		CREATE TEMP TABLE fishing_trip_events(trip_id text REFERENCES fishing_trips(id), sequence bigint, event_jsonb jsonb, PRIMARY KEY(trip_id, sequence));
		CREATE TEMP TABLE fishing_tackle_wear(user_id bigint, item_id text, wear double precision, updated_at timestamptz, PRIMARY KEY(user_id, item_id));
		CREATE TEMP TABLE fishing_spot_pressure(user_id bigint, spot_id text, pressure double precision, last_fished_at timestamptz, updated_at timestamptz, PRIMARY KEY(user_id, spot_id));
		INSERT INTO player_action_queue VALUES(411304);
		INSERT INTO fishing_trips(id, user_id, queue_id, loadout_snapshot_jsonb, supplies_jsonb, summary_jsonb)
		VALUES('trip-test', 9003034, 411304,
		 '{"method":"float","components":{"mainLineItemId":"main-line","leaderItemId":"leader"}}',
		 '{"remaining":{"leader":1,"hook":1,"floatOrSinker":1},"wear":{"mainLine":0.3,"leader":0.2}}',
		 '{"casts":19,"landed":10,"fishingExp":9.7,"recentEvents":[{"kind":"cast","sequence":63,"at":"2026-09-04T10:23:57Z"}]}');
		INSERT INTO fishing_trip_events VALUES('trip-test',63,'{"kind":"cast","sequence":63}');
	`)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, tx
}

func TestFishingLifecyclePostgres(t *testing.T) {
	t.Run("healthy normal and offline phases preserve identical checkpoints", func(t *testing.T) {
		var normalPayload []byte
		for _, offline := range []bool{false, true} {
			ctx, tx := fishingLifecyclePostgresTx(t)
			at := time.Date(2026, 9, 4, 10, 24, 0, 0, time.UTC)
			runtime := fishingExecutorRuntime{
				TripID: "trip-test", SpotID: "spot", WaterID: "water", DurationMS: 4_000,
				ActorType: ActionActorTypeKitty, ActorID: 2333, Phase: fishingPhaseResting,
				PhaseStartedAtMS: at.Add(-4 * time.Second).UnixMilli(), PhaseEndsAtMS: at.UnixMilli(),
				Loadout:  FishingLoadoutSnapshot{Method: "lure"},
				Supplies: FishingTripSupplies{Remaining: FishingSupplyCounts{MainLine: 1, Leader: 1, Lure: 1}},
				Rules:    fishingRulesSnapshot{Water: catalog.FishingWater{ID: "water"}, Spot: catalog.FishingSpot{ID: "spot"}, Species: []catalog.FishingSpecies{{ID: "test-fish"}}},
				Summary:  FishingTripSummary{Casts: 19, Landed: 10}, TimelineSeq: 63, Seed: 42,
			}
			payload, err := marshalFishingEnvelope(FishingTripActionID, runtime)
			if err != nil {
				t.Fatal(err)
			}
			item := QueueRuntimeItem{QueueID: 411304, ActorType: ActionActorTypeKitty, ActorID: 2333, PayloadRaw: payload, TotalCount: 1}
			e := newFishingExecutor(&Store{})
			var result SettleResult
			if offline {
				result, err = e.SettleFastForwardTx(ctx, tx, 9003034, item, at.Add(time.Hour), FastForwardBudget{MaxIterations: 1})
			} else {
				result, err = e.SettleNormalTx(ctx, tx, 9003034, item, at.Add(time.Hour))
			}
			if err != nil || !result.Progressed || result.CancelAfterSettle {
				t.Fatalf("offline=%t result=%+v err=%v", offline, result, err)
			}
			parsed, ok := parseFishingExecutorRuntime(result.NextPayload)
			if !ok || parsed.Phase != fishingPhaseWaiting || parsed.Summary.Casts != 20 || parsed.Summary.Landed != 10 {
				t.Fatalf("checkpoint=%+v", parsed)
			}
			start, end := nextQueueRunWindow(at.Add(time.Hour), result, result.NextPayload, offline, offline)
			var storedStart, storedEnd time.Time
			if err := tx.QueryRow(ctx, `SELECT phase_started_at,phase_ends_at FROM fishing_trips`).Scan(&storedStart, &storedEnd); err != nil {
				t.Fatal(err)
			}
			if !start.Equal(storedStart) || !end.Equal(storedEnd) {
				t.Fatalf("queue and trip clock diverged: %v..%v != %v..%v", start, end, storedStart, storedEnd)
			}
			if offline && string(normalPayload) != string(result.NextPayload) {
				t.Fatal("offline checkpoint differs from normal settlement")
			}
			normalPayload = result.NextPayload
			// Release the test's user advisory lock before opening the next connection.
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("invalid payload closes before deletion and cannot return twice", func(t *testing.T) {
		ctx, tx := fishingLifecyclePostgresTx(t)
		s := &Store{}
		e := newFishingExecutor(s)
		item := QueueRuntimeItem{QueueID: 411304, PayloadRaw: []byte(`{"executorId":"fishing_trip","runtime":{"durationMs":2300}}`)}
		result, err := e.SettleFastForwardTx(ctx, tx, 9003034, item, time.Now(), FastForwardBudget{})
		if err != nil || !result.CancelAfterSettle {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		var status, reason string
		var suppliesRaw, summaryRaw []byte
		if err := tx.QueryRow(ctx, `SELECT status,stop_reason,supplies_jsonb,summary_jsonb FROM fishing_trips`).Scan(&status, &reason, &suppliesRaw, &summaryRaw); err != nil {
			t.Fatal(err)
		}
		var supplies FishingTripSupplies
		var summary FishingTripSummary
		if err := json.Unmarshal(suppliesRaw, &supplies); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(summaryRaw, &summary); err != nil {
			t.Fatal(err)
		}
		if status != "stopped" || reason != fishingTripRuntimeInvalidStopReason || !supplies.Returned || supplies.Consumed.Leader != 1 || supplies.Consumed.Hook != 1 || supplies.Consumed.FloatOrSinker != 1 {
			t.Fatalf("status=%s reason=%s supplies=%+v", status, reason, supplies)
		}
		if summary.Casts != 19 || summary.Landed != 10 || summary.FishingExp != 9.7 || summary.RecentEvents[len(summary.RecentEvents)-1].Sequence != 64 {
			t.Fatalf("lost summary/history: %+v", summary)
		}
		// An explicit retry must not create another finished event or consume tackle again.
		if err := e.CancelRunTx(ctx, tx, 9003034, item); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM fishing_trip_events`).Scan(&count); err != nil || count != 2 {
			t.Fatalf("events=%d err=%v", count, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM player_action_queue WHERE id=411304`); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT status FROM fishing_trips WHERE queue_id IS NULL`).Scan(&status); err != nil || status != "stopped" {
			t.Fatalf("orphan state=%s err=%v", status, err)
		}
	})
	t.Run("queue deletion failure rolls back trip and supplies", func(t *testing.T) {
		ctx, tx := fishingLifecyclePostgresTx(t)
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE queue_blocker(queue_id bigint REFERENCES player_action_queue(id)); INSERT INTO queue_blocker VALUES(411304)`); err != nil {
			t.Fatal(err)
		}
		attempt, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = newFishingExecutor(&Store{}).SettleNormalTx(ctx, attempt, 9003034, QueueRuntimeItem{QueueID: 411304, PayloadRaw: []byte(`{}`)}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := attempt.Exec(ctx, `DELETE FROM player_action_queue WHERE id=411304`); err == nil {
			t.Fatal("expected queue deletion failure")
		}
		if err := attempt.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		var status string
		var returned bool
		var count int
		if err := tx.QueryRow(ctx, `SELECT status,COALESCE((supplies_jsonb->>'returned')::boolean,false) FROM fishing_trips`).Scan(&status, &returned); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM fishing_trip_events`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if status != "running" || returned || count != 1 {
			t.Fatalf("partial cleanup committed: %s returned=%t events=%d", status, returned, count)
		}
	})
	t.Run("manual stop ignores corrupted payload and closed trip cannot be revived", func(t *testing.T) {
		ctx, tx := fishingLifecyclePostgresTx(t)
		e := newFishingExecutor(&Store{})
		if err := e.CancelRunTx(ctx, tx, 9003034, QueueRuntimeItem{QueueID: 411304, PayloadRaw: []byte(`{"tripId":"wrong"}`)}); err != nil {
			t.Fatal(err)
		}
		runtime := fishingExecutorRuntime{TripID: "trip-test", ActorType: ActionActorTypeKitty, ActorID: 2333, Phase: fishingPhaseWaiting}
		if err := e.saveFishingTripRuntimeTx(ctx, tx, 9003034, 411304, runtime, false, time.Now()); err == nil {
			t.Fatal("revived stopped trip")
		}
		var reason string
		if err := tx.QueryRow(ctx, `SELECT stop_reason FROM fishing_trips`).Scan(&reason); err != nil || reason != "manual_stop" {
			t.Fatalf("reason=%s err=%v", reason, err)
		}
	})
}
