package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mysecondwebgame/server/internal/operationeffect"
)

func batchFoodPostgresStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("OPERATION_EFFECT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OPERATION_EFFECT_TEST_DATABASE_URL for isolated food batch proof")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if h := cfg.ConnConfig.Host; h != "localhost" && h != "127.0.0.1" && h != "::1" {
		t.Fatal("loopback database required")
	}
	base, err := os.ReadFile("../../schema/migrations/20260904_operation_effects.sql")
	if err != nil {
		t.Fatal(err)
	}
	batch, err := os.ReadFile("../../schema/migrations/20260906_operation_effect_batch_quantity.sql")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, `SET search_path=pg_temp;
CREATE TEMP TABLE users(id bigint PRIMARY KEY);
INSERT INTO users VALUES(970051),(970052);
CREATE TEMP TABLE player_kitties(id bigint,user_id bigint);
INSERT INTO player_kitties VALUES(7,970051),(8,970051),(9,970052);
CREATE TEMP TABLE player_action_queue(id bigint PRIMARY KEY,user_id bigint,status text,current_run_started_at timestamptz,current_run_ends_at timestamptz);
CREATE TEMP TABLE player_item_instances(instance_id bigint PRIMARY KEY,user_id bigint,template_item_id text,custom_data jsonb DEFAULT '{}',quantity bigint CHECK(quantity>0),enhance_level int DEFAULT 0,roll_seed bigint DEFAULT 0,base_snapshot jsonb DEFAULT '{}',affixes jsonb DEFAULT '[]',milestone_affix_claims jsonb DEFAULT '[]',durability int DEFAULT 100,max_durability int DEFAULT 100,state text DEFAULT 'inventory',archived_at timestamptz,updated_at timestamptz);
CREATE TEMP TABLE player_equip_slots(instance_id bigint,user_id bigint);
CREATE TEMP TABLE player_kitty_equip_slots(instance_id bigint,kitty_id bigint);
CREATE TEMP TABLE fishing_trips(user_id bigint,status text,loadout_snapshot_jsonb jsonb);
CREATE TEMP TABLE player_warehouse_change_logs(user_id bigint,event_type text,item_instance_id bigint,template_item_id text,quantity bigint,source_surface text,target_surface text,details jsonb,category text,direction text,subject_type text,subject_id text,item_totals jsonb,instance_ids jsonb,audit_key text,created_at timestamptz);
CREATE UNIQUE INDEX ON player_warehouse_change_logs(audit_key) WHERE audit_key<>'';
`+strings.ReplaceAll(string(base), "CREATE TABLE", "CREATE TEMP TABLE")+string(batch)+string(batch))
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := fishCuisineTestStore(t)
	s.db = pool
	s.gameData.CatteryFurniture = nil // This fixture exercises food, not furniture tables.
	return s, ctx
}

func TestOperationEffectBatchPostgresAtomicReserveAndReceipts(t *testing.T) {
	s, ctx := batchFoodPostgresStore(t)
	const uid int64 = 970051
	at := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := s.db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO player_item_instances(instance_id,user_id,template_item_id,quantity) VALUES(1,$1,'field_honey_grill',150),(2,$1,'field_honey_soup',200),(3,$1,'creamfin_dual_fish_grill',20)`, uid)
	player := operationeffect.Target{Kind: operationeffect.TargetPlayer}
	shared := operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}
	use := func(id, q int64, target operationeffect.Target, key string, now time.Time) error {
		_, err := s.ActivateOperationEffectQuantity(ctx, uid, id, target, key, now, q)
		return err
	}
	// Simultaneous duplicate submissions must consume just one batch.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- use(1, 100, shared, "batch-one", at) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var stock, remaining, receipts, quantity int64
	if err := s.db.QueryRow(ctx, `SELECT (SELECT quantity FROM player_item_instances WHERE instance_id=1),remaining_charges,(SELECT count(*) FROM player_operation_effect_activations),(SELECT quantity FROM player_operation_effect_activations LIMIT 1) FROM player_operation_effect_grants`).Scan(&stock, &remaining, &receipts, &quantity); err != nil {
		t.Fatal(err)
	}
	if stock != 50 || remaining != 22000 || receipts != 1 || quantity != 100 {
		t.Fatal(stock, remaining, receipts, quantity)
	}
	if err := use(1, 99, shared, "batch-one", at); !errors.Is(err, ErrOperationEffectIdempotencyConflict) {
		t.Fatal(err)
	}
	if err := use(1, 1, shared, "over-cap", at); !errors.Is(err, ErrOperationEffectAtCap) {
		t.Fatal(err)
	}
	if err := use(2, 1, player, "conflict", at); !errors.Is(err, ErrOperationEffectStrengthConflict) {
		t.Fatal(err)
	}
	// A late receipt failure rolls back grant creation AND inventory consumption.
	exec(`ALTER TABLE player_operation_effect_activations ADD CONSTRAINT reject_five CHECK(quantity<>5)`)
	if err := use(3, 5, player, "rollback", at); err == nil {
		t.Fatal("expected receipt constraint failure")
	}
	if err := s.db.QueryRow(ctx, `SELECT (SELECT quantity FROM player_item_instances WHERE instance_id=3),(SELECT count(*) FROM player_operation_effect_grants)`).Scan(&stock, &receipts); err != nil || stock != 20 || receipts != 1 {
		t.Fatal(stock, receipts, err)
	}
	exec(`ALTER TABLE player_operation_effect_activations DROP CONSTRAINT reject_five`)
	// Exhaust the old charge pool, then time food can be purchased per actor.
	exec(`UPDATE player_operation_effect_grants SET remaining_charges=0 WHERE lifetime_kind='charges'`)
	for _, id := range []int64{7, 8} {
		if err := use(2, 96, operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: id}, "soup-"+string(rune('0'+id)), at); err != nil {
			t.Fatal(err)
		}
	}
	if err := use(2, 9, player, "stock", at); !errors.Is(err, ErrInsufficientItem) {
		t.Fatal(err)
	}
	if err := use(2, 1, operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: 9}, "foreign", at); !errors.Is(err, ErrOperationEffectTargetNotOwned) {
		t.Fatal(err)
	}
	if err := use(2, 1, shared, "shared-soup", at); !errors.Is(err, ErrOperationEffectTargetInvalid) {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `SELECT min(EXTRACT(epoch FROM valid_until-valid_from))::bigint,count(*) FROM player_operation_effect_grants WHERE lifetime_kind='duration'`).Scan(&remaining, &receipts); err != nil || remaining != 86400 || receipts != 2 {
		t.Fatal(remaining, receipts, err)
	}
}

func TestOperationEffectBatchPostgresTopUpOldCapAndOfflineBoundary(t *testing.T) {
	s, ctx := batchFoodPostgresStore(t)
	const uid int64 = 970051
	at := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	player := operationeffect.Target{Kind: operationeffect.TargetPlayer}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_item_instances(instance_id,user_id,template_item_id,quantity) VALUES(1,$1,'field_honey_grill',150)`, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActivateOperationEffect(ctx, uid, 1, player, "old", at); err != nil {
		t.Fatal(err)
	}
	// Historical grant has the old storage cap, but retains its own strength.
	if _, err := s.db.Exec(ctx, `UPDATE player_operation_effect_grants SET definition_snapshot_jsonb=jsonb_set(definition_snapshot_jsonb,'{stacking,maxCharges}','500')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_action_queue VALUES(1,$1,'running',$2,$3)`, uid, at, at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActivateOperationEffectQuantity(ctx, uid, 1, player, "blocked", at.Add(time.Minute), 99); !errors.Is(err, ErrOperationEffectRecoveryPending) {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `DELETE FROM player_action_queue`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActivateOperationEffectQuantity(ctx, uid, 1, player, "refill", at.Add(time.Minute), 99); err != nil {
		t.Fatal(err)
	}
	var remaining, stock int64
	if err := s.db.QueryRow(ctx, `SELECT remaining_charges,(SELECT quantity FROM player_item_instances WHERE instance_id=1) FROM player_operation_effect_grants`).Scan(&remaining, &stock); err != nil || remaining != 22000 || stock != 50 {
		t.Fatal(remaining, stock, err)
	}
}
