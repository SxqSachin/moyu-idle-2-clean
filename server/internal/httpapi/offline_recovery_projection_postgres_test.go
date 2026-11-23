package httpapi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func TestOfflineRecoveryPostgresPlayerProjectionIsReadOnly(t *testing.T) {
	dsn := os.Getenv("OFFLINE_RECOVERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OFFLINE_RECOVERY_TEST_DATABASE_URL for isolated projection proof")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if host := cfg.ConnConfig.Host; host != "localhost" && host != "127.0.0.1" && host != "::1" {
		t.Fatal("loopback database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	database := fmt.Sprintf("offline_projection_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), `DROP DATABASE `+pgx.Identifier{database}.Sanitize()); err != nil {
			t.Error(err)
		}
	}()
	cfg.ConnConfig.Database, cfg.MaxConns = database, 1
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw, err := os.ReadFile("../../schema/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(raw), "-- === Identity")
	if start < 0 {
		t.Fatal("missing baseline identity schema")
	}
	if _, err = db.Exec(ctx, string(raw[start:])); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO users(id,username,password_hash) VALUES(1,'projection-test','test');
 INSERT INTO player_attributes(user_id,attr_key,final_value) VALUES(1,'strength',42);
 INSERT INTO player_derived_stats_cache(user_id,stats_jsonb) VALUES(1,'{"power":42}');`); err != nil {
		t.Fatal(err)
	}
	s := newPatchTestServer()
	s.deps.GameData = catalog.Data{Attributes: []catalog.AttributeDefinition{{ID: "strength", GrowthType: "static"}}}
	s.deps.Store = store.New(db, store.RuntimeConfig{}, s.deps.GameData)
	s.roleEquipmentSnap["1:player:self"] = map[int64]string{9: "old equipped item"}
	// A report read must not write skill settings or recompute historical gains.
	if _, err = db.Exec(ctx, `SET default_transaction_read_only=on`); err != nil {
		t.Fatal(err)
	}
	summary := store.ActionFastForwardSummary{EnhanceResults: []store.EnhanceAttemptResult{{EquipmentInstanceID: 9, AfterLevel: 2}}}
	started := time.Now()
	if _, err = s.syncOfflineRecoveryDomains(ctx, 1, summary); err != nil {
		t.Fatal(err)
	}
	if current, ok := s.roleEquipmentSnap["1:player:self"]; !ok || len(current) != 0 {
		t.Fatal("terminal loadout retained old equipped items", current)
	}
	if value := s.attrSnap[1].Attributes["strength"]; !strings.Contains(value, "42") {
		t.Fatal("terminal player attributes were not refreshed", value)
	}
	t.Logf("terminal inventory/equipment/attribute projection: %s", time.Since(started))
	if _, err = db.Exec(ctx, `SET default_transaction_read_only=off`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `DROP TABLE player_attributes`); err != nil {
		t.Fatal(err)
	}
	if err = s.syncOfflineRecoveryPlayer(ctx, 1, summary); err == nil {
		t.Fatal("failed player projection was reported as synchronized")
	}
}
