package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func operationEffectPostgresTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	dsn := os.Getenv("OPERATION_EFFECT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OPERATION_EFFECT_TEST_DATABASE_URL for isolated PostgreSQL tests")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "localhost" && cfg.Host != "127.0.0.1" && cfg.Host != "::1" {
		t.Fatal("operation effect tests require a loopback PostgreSQL host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	// Never resolve a game table in public, even if the implementation changes.
	_, err = tx.Exec(ctx, `
		SET LOCAL search_path = pg_temp;
		CREATE TEMP TABLE player_action_queue(
		 id bigint PRIMARY KEY, user_id bigint, status text,
		 current_run_started_at timestamptz, current_run_ends_at timestamptz,
		 actor_type text DEFAULT 'player', actor_id bigint DEFAULT 0, payload jsonb DEFAULT '{}'
		);
		CREATE TEMP TABLE player_kitties(id bigint,user_id bigint,status text);
		CREATE TEMP TABLE player_cattery_furniture_state(user_id bigint);
		CREATE TEMP TABLE player_operation_effect_grants(
		 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, user_id bigint,
		 definition_id text, ruleset_version text, domain text, effect_kind text, stacking_key text, priority integer,
		 target_kind text, target_actor_id bigint, lifetime_kind text, valid_from timestamptz, valid_until timestamptz,
		 initial_charges bigint, remaining_charges bigint CHECK(remaining_charges >= 0),
		 definition_snapshot_jsonb jsonb, source_item_id text, created_at timestamptz, updated_at timestamptz
		);
		CREATE TEMP TABLE player_operation_effect_charge_receipts(
		 grant_id bigint REFERENCES player_operation_effect_grants(id), user_id bigint,
		 operation_key text, started_at timestamptz, consumed_at timestamptz, PRIMARY KEY(grant_id, operation_key)
		);
		CREATE TEMP TABLE player_operation_effect_state(user_id bigint PRIMARY KEY, revision bigint, updated_at timestamptz);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, tx
}

func TestOperationEffectPostgresRecoveryAndReceipts(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := &Store{operationEffectRegistry: offlineEffectRegistry(t)}
	start := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	now := start.Add(time.Hour)
	def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	activate := func(at time.Time) (int64, error) {
		return s.activateOperationEffectGrantTx(ctx, tx, 41, "food", def, operationeffect.Target{Kind: operationeffect.TargetPlayer}, raw, at)
	}
	id, err := activate(start)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	assertState := func(charges, receipts, revision int64) {
		t.Helper()
		var gotCharges, gotReceipts, gotRevision int64
		if err := tx.QueryRow(ctx, `SELECT remaining_charges,
		 (SELECT COUNT(*) FROM player_operation_effect_charge_receipts),
		 COALESCE((SELECT revision FROM player_operation_effect_state WHERE user_id=41),0)
		 FROM player_operation_effect_grants WHERE id=$1`, id).Scan(&gotCharges, &gotReceipts, &gotRevision); err != nil {
			t.Fatal(err)
		}
		if gotCharges != charges || gotReceipts != receipts || gotRevision != revision {
			t.Fatalf("state=(%d,%d,%d), want=(%d,%d,%d)", gotCharges, gotReceipts, gotRevision, charges, receipts, revision)
		}
	}
	// Including a null deadline and the exact boundary prevents paused or due
	// historical lanes from borrowing charges activated at the current time.
	for _, deadline := range []any{nil, start.Add(time.Minute), now} {
		exec(`INSERT INTO player_action_queue VALUES(1,41,'running',$1,$2)`, start, deadline)
		if _, err := activate(now); !errors.Is(err, ErrOperationEffectRecoveryPending) {
			t.Fatalf("deadline=%v: top-up err=%v", deadline, err)
		}
		assertState(2, 0, 0)
		exec(`DELETE FROM player_action_queue`)
	}
	exec(`INSERT INTO player_action_queue VALUES(1,42,'running',$1,$1),(2,41,'running',$1,$2)`, start, now.Add(time.Minute))
	if sameID, err := activate(now); err != nil || sameID != id {
		t.Fatalf("caught-up top-up: id=%d err=%v", sameID, err)
	}
	assertState(4, 0, 0)
	session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, now)
	if err != nil || session == nil {
		t.Fatalf("load: %v", err)
	}
	op := operationeffect.Operation{Key: "offline:1", Domain: operationeffect.DomainLifeAction,
		ActorKind: operationeffect.ActorPlayer, ActionID: "work_logging", StartedAt: now,
		Capabilities: map[string]bool{lifeaction.CapabilityMainOutput: true}}
	prepared := session.Prepare(op)
	if snapshot, err := session.Commit(prepared); err != nil || len(snapshot.Rules) != 1 {
		t.Fatalf("commit: snapshot=%+v err=%v", snapshot, err)
	}
	exec(`SAVEPOINT wrong_user`)
	if _, err := s.flushOperationEffectSessionTx(ctx, tx, 42, session, now); err == nil {
		t.Fatal("another user's grant accepted")
	}
	exec(`ROLLBACK TO SAVEPOINT wrong_user`)
	assertState(4, 0, 0)
	exec(`SAVEPOINT effect_flush`)
	if changed, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, now); err != nil || !changed {
		t.Fatalf("first flush: changed=%t err=%v", changed, err)
	}
	assertState(3, 1, 1)
	if changed, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, now); err != nil || changed {
		t.Fatalf("retry flush: changed=%t err=%v", changed, err)
	}
	assertState(3, 1, 1)
	exec(`ROLLBACK TO SAVEPOINT effect_flush`)
	assertState(4, 0, 0)
	// An inconsistent budget must fail closed; the queue transaction rolls back
	// the inserted receipt together with its rewards, not just the grant update.
	exec(`UPDATE player_operation_effect_grants SET remaining_charges=0 WHERE id=$1`, id)
	exec(`SAVEPOINT insufficient`)
	if _, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, now); err == nil {
		t.Fatal("insufficient charges accepted")
	}
	exec(`ROLLBACK TO SAVEPOINT insufficient`)
	assertState(0, 0, 0)
	if empty, err := s.loadOperationEffectSessionTx(ctx, tx, 41, now); err != nil || empty != nil {
		t.Fatalf("exhausted grant loaded: %v %v", empty, err)
	}
}

func TestOperationEffectPostgresLoadsExpiredHistoricalDuration(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := &Store{operationEffectRegistry: offlineEffectRegistry(t)}
	start := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	now := start.Add(time.Hour)
	def := operationEffectTestDefinition(operationeffect.LifetimeDuration)
	def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 120}
	def.Stacking.Mode = operationeffect.StackExtendDuration
	raw, _ := json.Marshal(def)
	if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "food", def, operationeffect.Target{Kind: operationeffect.TargetPlayer}, raw, start); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue VALUES(1,41,'running',$1,$2)`, start, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, now)
	if err != nil || session == nil {
		t.Fatalf("expired historical grant missing: %v", err)
	}
	for i, at := range []time.Time{start.Add(-time.Second), start, start.Add(time.Minute), start.Add(2 * time.Minute), now} {
		op := operationeffect.Operation{Key: at.String(), Domain: operationeffect.DomainLifeAction,
			ActorKind: operationeffect.ActorPlayer, StartedAt: at,
			Capabilities: map[string]bool{lifeaction.CapabilityMainOutput: true}}
		prepared := session.Prepare(op)
		snapshot, err := session.Commit(prepared)
		want := 0
		if i == 1 || i == 2 {
			want = 1
		}
		if err != nil || len(snapshot.Rules) != want {
			t.Fatalf("at=%v: rules=%d want=%d err=%v", at, len(snapshot.Rules), want, err)
		}
	}
	if changed, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, now); err != nil || changed {
		t.Fatalf("duration consumed charges: changed=%t err=%v", changed, err)
	}
}
