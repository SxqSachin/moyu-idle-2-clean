package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mysecondwebgame/server/internal/catalog"
)

type equipmentSkillQueryCounter struct{ count atomic.Int64 }

func (q *equipmentSkillQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	return ctx
}
func (*equipmentSkillQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestEquipmentSkillPostgresMigrationCASAndSingleRead(t *testing.T) {
	dsn := os.Getenv("EQUIPMENT_SKILL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set EQUIPMENT_SKILL_TEST_DATABASE_URL for isolated temporary-table proof")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if host := config.ConnConfig.Host; host != "localhost" && host != "127.0.0.1" && host != "::1" {
		t.Fatal("loopback database required")
	}
	config.MaxConns = 1
	config.ConnConfig.RuntimeParams["search_path"] = "pg_temp"
	counter := &equipmentSkillQueryCounter{}
	config.ConnConfig.Tracer = counter
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TEMP TABLE player_skills(user_id bigint,skill_id text,enabled boolean DEFAULT false,cast_order int DEFAULT 0,rule_enabled boolean DEFAULT false,rule_groups_jsonb jsonb DEFAULT '[]',level int DEFAULT 1,exp bigint DEFAULT 0,updated_at timestamptz DEFAULT now(),PRIMARY KEY(user_id,skill_id));
CREATE TEMP TABLE player_equip_slots(user_id bigint,instance_id bigint,equip_type text);
CREATE TEMP TABLE player_item_instances(user_id bigint,instance_id bigint,template_item_id text,enhance_level int,state text);
CREATE TEMP TABLE player_custom_skills(id text,user_id bigint,archived_at timestamptz,current_revision_id bigint);
CREATE TEMP TABLE player_custom_skill_revisions(id bigint,compiled_skill_jsonb jsonb,ruleset_version int,result_status text);
INSERT INTO player_equip_slots VALUES(970971,1,'battle');
INSERT INTO player_item_instances VALUES(970971,1,'badge',0,'equipped');
INSERT INTO player_skills(user_id,skill_id,enabled) VALUES(970971,'a',true),(970971,'b',true);`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../schema/migrations/20260907_equipment_skill_config.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	st := &Store{db: pool}
	st.gameData.EquipmentTemplates = []catalog.EquipmentTemplate{{ID: "badge", GrantedSkill: &catalog.EquipmentGrantedSkill{SkillID: EquipmentMartialComboSkill, BaseCapacity: 2, CapacityMilestones: []int{10, 20}}}}
	for _, id := range []string{"a", "b", "c"} {
		st.gameData.Skills = append(st.gameData.Skills, map[string]any{"id": id, "effects": []any{map[string]any{"kind": "damage", "damageType": "physical"}}})
	}
	cfg := EquipmentSkillConfig{Version: 1, Enabled: true, SkillIDs: []string{"a", "b"}}
	saved, err := st.SaveEquipmentSkillConfig(ctx, 970971, EquipmentMartialComboSkill, cfg)
	if err != nil || saved.Revision != 1 {
		t.Fatal("initial save", saved, err)
	}
	if _, err := st.SaveEquipmentSkillConfig(ctx, 970971, EquipmentMartialComboSkill, cfg); !errors.Is(err, ErrEquipmentSkillRevision) {
		t.Fatal("stale revision accepted", err)
	}
	if _, err := st.SaveEquipmentSkillConfig(ctx, 970972, EquipmentMartialComboSkill, cfg); !errors.Is(err, ErrEquipmentSkillConfig) {
		t.Fatal("another owner accepted", err)
	}
	tooMany := saved
	tooMany.SkillIDs = []string{"a", "b", "c"}
	if _, err := st.SaveEquipmentSkillConfig(ctx, 970971, EquipmentMartialComboSkill, tooMany); !errors.Is(err, ErrEquipmentSkillConfig) {
		t.Fatal("over capacity accepted", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := st.SaveEquipmentSkillConfig(ctx, 970971, EquipmentMartialComboSkill, saved)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrEquipmentSkillRevision) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("CAS winners=%d conflicts=%d", ok, conflict)
	}
	counter.count.Store(0)
	rows, err := st.ListPlayerSkills(ctx, 970971)
	if err != nil || len(rows) != 3 || counter.count.Load() != 1 {
		t.Fatalf("single read: rows=%d queries=%d err=%v", len(rows), counter.count.Load(), err)
	}
	combat, err := st.GetCombatConfig(ctx, 970971)
	if err != nil || len(combat.SkillSlots) != 2 || combat.EquipmentSkills[EquipmentMartialComboSkill].Revision != 2 {
		t.Fatal("combat config round trip", combat, err)
	}
	owned, err := st.ListOwnedSkills(ctx, 970971)
	if err != nil || len(owned) != 2 {
		t.Fatal("equipment grant leaked into learned skills", owned, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE player_skills SET equipment_config_jsonb='{}' WHERE skill_id='a'`); err == nil {
		t.Fatal("ordinary skill accepted equipment config")
	}
	if _, err := pool.Exec(ctx, `UPDATE player_skills SET enabled=false WHERE skill_id='b'`); err != nil {
		t.Fatal(err)
	}
	saved.Revision = 2
	if _, err := st.SaveEquipmentSkillConfig(ctx, 970971, EquipmentMartialComboSkill, saved); !errors.Is(err, ErrEquipmentSkillConfig) {
		t.Fatal("uncarried child accepted", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO player_custom_skill_revisions VALUES(1,'{"id":"custom:own","effects":[{"kind":"damage","damageType":"physical"}]}',1,'accepted');
INSERT INTO player_custom_skills VALUES('custom:own',970971,NULL,1),('custom:kitty',970971,NULL,1),('custom:foreign',970972,NULL,1);
INSERT INTO player_skills(user_id,skill_id) VALUES(970971,'custom:own'),(970972,'custom:foreign');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.withTx(ctx, func(tx pgx.Tx) error {
		counter.count.Store(0)
		defs, err := st.loadoutCustomSkillDefinitionsTx(ctx, tx, 970971, []string{"custom:own", "custom:kitty", "custom:foreign"})
		if err != nil || len(defs) != 1 || defs["custom:own"] == nil || counter.count.Load() != 1 {
			t.Fatalf("private loadout ownership/bulk read: %+v %v", defs, err)
		}
		counter.count.Store(0)
		_, err = st.loadoutCustomSkillDefinitionsTx(ctx, tx, 970971, []string{"a", "b"})
		if err != nil || counter.count.Load() != 0 {
			t.Fatal("ordinary loadout added custom reads", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
