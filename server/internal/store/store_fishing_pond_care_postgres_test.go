package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mysecondwebgame/server/internal/catalog"
)

func TestPondCarePostgresAtomicConsumption(t *testing.T) {
	dsn := os.Getenv("POND_CARE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set POND_CARE_TEST_DATABASE_URL for isolated loopback PostgreSQL tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "localhost" && cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "::1" {
		t.Fatal("loopback database required")
	}
	// Only this connection's temporary tables are addressable, never public data.
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["search_path"] = "pg_temp"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
 CREATE TEMP TABLE player_item_instances(instance_id bigint PRIMARY KEY,user_id bigint,template_item_id text,fish_data jsonb,custom_data jsonb DEFAULT '{}',quantity bigint DEFAULT 1,enhance_level int DEFAULT 0,roll_seed bigint DEFAULT 0,base_snapshot jsonb DEFAULT '{}',affixes jsonb DEFAULT '[]',durability int DEFAULT 100,max_durability int DEFAULT 100,state text DEFAULT 'inventory',archived_at timestamptz,updated_at timestamptz DEFAULT now());
 CREATE TEMP TABLE fishing_pond_events(user_id bigint,event_kind text,fish_instance_id bigint,related_instance_id bigint,details_jsonb jsonb,created_at timestamptz);
 CREATE TEMP TABLE player_warehouse_change_logs(user_id bigint,event_type text,item_instance_id bigint,template_item_id text,quantity bigint,source_surface text,target_surface text,details jsonb,category text,direction text,subject_type text,subject_id text,item_totals jsonb,instance_ids jsonb,audit_key text,created_at timestamptz);
 CREATE UNIQUE INDEX ON player_warehouse_change_logs(audit_key) WHERE audit_key <> '';
 `)
	if err != nil {
		t.Fatal(err)
	}
	s := pondFeedTestStore()
	s.db = pool
	s.gameData.Items = append(s.gameData.Items, catalog.Item{ID: "care", PondFeed: pondCareTestFeed()})
	raw, _ := json.Marshal(pondCareTestFish())
	_, err = pool.Exec(ctx, `INSERT INTO player_item_instances(instance_id,user_id,template_item_id,fish_data,state,quantity) VALUES(1,900707,'fish_catch',$1,$2,1),(2,900707,'care',NULL,'inventory',2)`, raw, fishingPondState)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(user, rev int64, inject bool) error {
		return s.withTx(ctx, func(tx pgx.Tx) error {
			if err := s.useFishingPondTargetedFeedTx(ctx, tx, user, 1, "care", rev, time.Now()); err != nil {
				return err
			}
			if inject {
				return errors.New("injected failure after inventory, fish and event writes")
			}
			return nil
		})
	}
	check := func(quantity, revision, events int64) {
		t.Helper()
		var q, r, e int64
		err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT quantity FROM player_item_instances WHERE instance_id=2),0),(SELECT COALESCE((fish_data->>'pondCareRevision')::bigint,0) FROM player_item_instances WHERE instance_id=1),(SELECT count(*) FROM fishing_pond_events)`).Scan(&q, &r, &e)
		if err != nil || q != quantity || r != revision || e != events {
			t.Fatalf("inventory/revision/events=%d/%d/%d err=%v", q, r, e, err)
		}
	}
	if apply(900708, 0, false) == nil {
		t.Fatal("another owner could feed fish")
	}
	check(2, 0, 0)
	if apply(900707, 0, true) == nil {
		t.Fatal("failure injection succeeded")
	}
	check(2, 0, 0)
	if err := apply(900707, 0, false); err != nil {
		t.Fatal(err)
	}
	check(1, 1, 1)
	if apply(900707, 0, false) == nil {
		t.Fatal("stale retry consumed twice")
	}
	check(1, 1, 1)
	if err := apply(900707, 1, false); err != nil {
		t.Fatal(err)
	}
	check(0, 2, 2)
	if apply(900707, 2, false) == nil {
		t.Fatal("empty inventory extended life")
	}
	check(0, 2, 2)
}
