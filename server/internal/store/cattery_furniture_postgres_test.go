package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/operationeffect"
	"os"
	"testing"
	"time"
)

func furniturePostgresStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("OPERATION_EFFECT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OPERATION_EFFECT_TEST_DATABASE_URL for loopback temporary-table proof")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if h := cfg.ConnConfig.Host; h != "localhost" && h != "127.0.0.1" && h != "::1" {
		t.Fatal("loopback database required")
	}
	cfg.MaxConns = 1
	cfg.MinConns = 1
	testSchema := pgx.Identifier{fmt.Sprintf("furniture_cache_test_%d", time.Now().UnixNano())}.Sanitize()
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, `SET search_path=pg_temp;
   CREATE TEMP TABLE users(id bigint PRIMARY KEY,ban_reason text DEFAULT '',ban_until timestamptz,ban_is_permanent boolean DEFAULT false);
   CREATE TEMP TABLE player_buildings(user_id bigint,building_id text,level bigint);
   CREATE TEMP TABLE player_attributes(user_id bigint,attr_key text,attr_exp float8 DEFAULT 0,base_level bigint DEFAULT 1,final_value float8 DEFAULT 1,lock_version bigint DEFAULT 0,updated_at timestamptz,PRIMARY KEY(user_id,attr_key));
   CREATE TEMP TABLE player_action_queue(id bigint PRIMARY KEY,user_id bigint,status text,actor_type text,actor_id bigint DEFAULT 0,payload jsonb DEFAULT '{}',current_run_started_at timestamptz,current_run_ends_at timestamptz);
   CREATE TEMP TABLE player_kitties(id bigint,user_id bigint,status text);
   CREATE TEMP TABLE player_operation_effect_grants(id bigint,user_id bigint,target_kind text,target_actor_id bigint,source_item_id text,valid_from timestamptz,valid_until timestamptz,initial_charges bigint,remaining_charges bigint,created_at timestamptz,definition_snapshot_jsonb jsonb,lifetime_kind text,priority int);
   CREATE TEMP TABLE player_item_instances(instance_id bigint PRIMARY KEY,user_id bigint,template_item_id text,custom_data jsonb DEFAULT '{}',quantity bigint,enhance_level int DEFAULT 0,roll_seed bigint DEFAULT 0,base_snapshot jsonb DEFAULT '{}',affixes jsonb DEFAULT '[]',durability float8 DEFAULT 100,max_durability float8 DEFAULT 100,state text DEFAULT 'inventory',archived_at timestamptz,updated_at timestamptz);
   CREATE TEMP TABLE player_cattery_furniture_state(user_id bigint PRIMARY KEY,revision bigint DEFAULT 0,exp_remainders jsonb DEFAULT '{}',contribution_totals jsonb DEFAULT '{}',contribution_seen jsonb DEFAULT '{}',contribution_report jsonb DEFAULT '{}');
   CREATE TEMP TABLE player_cattery_furniture(user_id bigint,furniture_id text,definition jsonb,remaining_ms bigint CHECK(remaining_ms>=0),enabled_until timestamptz,updated_at timestamptz,PRIMARY KEY(user_id,furniture_id));
   CREATE TEMP TABLE player_cattery_furniture_periods(id bigserial PRIMARY KEY,user_id bigint,furniture_id text,valid_from timestamptz,valid_until timestamptz,source jsonb,CHECK(valid_until>=valid_from));
   CREATE TEMP TABLE player_cattery_furniture_commands(user_id bigint,request_id text,request jsonb,revision bigint,created_at timestamptz DEFAULT now(),PRIMARY KEY(user_id,request_id));
   CREATE TEMP TABLE battle_room_members(room_id bigint,user_id bigint);
   CREATE TEMP TABLE player_warehouse_change_logs(id bigserial PRIMARY KEY,user_id bigint,event_type text,item_instance_id bigint,template_item_id text,quantity bigint,source_surface text,target_surface text,details jsonb,category text,direction text,subject_type text,subject_id text,item_totals jsonb,instance_ids jsonb,audit_key text,created_at timestamptz);
   CREATE UNIQUE INDEX ON player_warehouse_change_logs(audit_key) WHERE audit_key<>'';
   INSERT INTO users(id) VALUES(970041),(970042);
   INSERT INTO player_buildings VALUES(970041,'cattery',1);
   INSERT INTO player_item_instances(instance_id,user_id,template_item_id,quantity) VALUES(1,970041,'wood',1000),(2,970041,'fiber',1000);
  `)
		if err != nil {
			return err
		}
		if _, err = c.Exec(ctx, `CREATE SCHEMA `+testSchema+`; SET search_path=`+testSchema+`,pg_temp`); err != nil {
			return err
		}
		migration, err := os.ReadFile("../../schema/migrations/20260907_cattery_furniture_cache.sql")
		if err != nil {
			return err
		}
		_, err = c.Exec(ctx, string(migration))
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, `DROP SCHEMA IF EXISTS `+testSchema+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	d := catteryfurniture.Definition{ID: "test_desk", Version: "v1", NameKey: "name", IconID: "item.study_desk", InstallCosts: map[string]int64{"wood": 10, "fiber": 10}, RefillCosts: map[string]int64{"wood": 2, "fiber": 3}, CapacitySeconds: 120000, RefillSeconds: 36000, Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1500}, {Kind: operationeffect.KindExtraLoot, Domain: operationeffect.DomainBattleReward, RateBPS: 2000}}}
	// Legacy economy fixtures settle immediately; cache/activation tests opt in
	// to the production policy explicitly and exercise the time boundary.
	return &Store{db: pool, gameData: catalog.Data{CatteryFurniture: []catteryfurniture.Definition{d}}, furniturePolicy: &furnitureRuntimePolicy{}}, ctx
}

func TestFurniturePostgresGenericOnlineOfflineAndChunked(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	s.furniturePolicy = nil
	const uid int64 = 970041
	s.attributeExpProfile = &attributeExpProfileResolver{profileByAttr: map[string]string{"gather": "test"}, curves: map[string]map[int64]float64{"test": {1: 1e9, 2: 0}}, capTotalExp: map[string]float64{"gather": 1e9}}
	at := time.Unix(1000, 0)
	source := s.gameData.CatteryFurniture[0].Source()
	raw, _ := json.Marshal(source)
	_, err := s.db.Exec(ctx, `INSERT INTO player_attributes(user_id,attr_key) VALUES($1,'gather')`, uid)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_state(user_id) VALUES($1)`, uid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_periods(user_id,furniture_id,valid_from,valid_until,source) VALUES($1,'test_desk',$2,$3,$4)`, uid, at, at.Add(37*time.Second), string(raw)); err != nil {
		t.Fatal(err)
	}
	var baselineXP float64
	for _, chunk := range []int{100, 1, 7, 13} {
		if _, err = s.db.Exec(ctx, `UPDATE player_attributes SET attr_exp=0`); err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.Exec(ctx, `UPDATE player_cattery_furniture_state SET exp_remainders='{}',contribution_totals='{}'`); err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=999 WHERE template_item_id='wood'`); err != nil {
			t.Fatal(err)
		}
		start, end := at, at.Add(2*time.Second)
		item := QueueRuntimeItem{QueueID: 71, ActorType: ActionActorTypeKitty, ActorID: 7, ActionID: "test_gather", Spec: ActionSpec{MainType: "gather", StatusExpReward: map[string]float64{"gather": 1.2345}, Costs: map[string]int64{"wood": 1}}, TotalCount: 100, RunStartAt: &start, RunEndsAt: &end, PayloadRaw: []byte(`{"executorId":"generic","runtime":{"durationMs":2000}}`), PreserveScheduledTimeline: true}
		for item.DoneCount < 100 {
			err = s.withTx(ctx, func(tx pgx.Tx) error {
				passive, e := s.loadPassiveSessionTx(ctx, tx, uid, *item.RunStartAt, at.Add(200*time.Second))
				if e != nil {
					return e
				}
				effects, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
				effects.SetPassive(passive)
				item.EffectSession = effects
				if item.DoneCount == 0 {
					item.PayloadRaw, e = freezeQueuePassive(item, item.PayloadRaw, at, operationeffect.DomainLifeAction)
					if e != nil {
						return e
					}
				}
				executor := &genericExecutor{store: s}
				var result SettleResult
				if chunk == 1 {
					result, e = executor.SettleNormalTx(ctx, tx, uid, item, *item.RunEndsAt)
				} else {
					result, e = executor.SettleFastForwardTx(ctx, tx, uid, item, at.Add(200*time.Second), FastForwardBudget{MaxIterations: chunk})
				}
				if e != nil {
					return e
				}
				if result.Completed <= 0 {
					t.Fatal("executor made no progress")
				}
				item.DoneCount = result.DoneAfter
				item.PayloadRaw = result.NextPayload
				start, end = result.NextRunStartAt, result.NextRunEndsAt
				_, e = s.flushOperationEffectSessionTx(ctx, tx, uid, effects, at.Add(200*time.Second))
				return e
			})
			if err != nil {
				t.Fatalf("chunk %d: %v", chunk, err)
			}
		}
		var xp float64
		var materials int64
		if err = s.db.QueryRow(ctx, `SELECT attr_exp FROM player_attributes WHERE user_id=$1 AND attr_key='gather'`, uid).Scan(&xp); err != nil {
			t.Fatal(err)
		}
		if err = s.db.QueryRow(ctx, `SELECT quantity FROM player_item_instances WHERE template_item_id='wood'`).Scan(&materials); err != nil {
			t.Fatal(err)
		}
		if chunk == 100 {
			baselineXP = xp
		}
		if math.Abs(xp-baselineXP) > 1e-9 || xp != 3.5183 || materials != 900 {
			t.Fatalf("chunk=%d xp=%v baseline=%v stock=%d", chunk, xp, baselineXP, materials)
		}
		report, err := s.FurnitureReport(ctx, uid, "")
		if err != nil {
			t.Fatal(err)
		}
		if report.Rows["test_desk"].Experience["gather"] != xp {
			t.Fatalf("chunk %d attribution differs from granted XP: %+v", chunk, report)
		}
		if _, err = s.db.Exec(ctx, `UPDATE player_cattery_furniture_state SET contribution_report='{}'`); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFurniturePostgresCommandsAtomicIdempotentAndHistory(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	cmd := FurnitureCommand{RequestID: "install-0001", FurnitureID: "test_desk", Version: "v1", Action: "install"}
	p, err := s.CommandFurniture(ctx, uid, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Revision != 1 || p.Inventory["wood"] != 990 || !p.Items[0].Installed || p.Items[0].EnabledUntil != nil {
		t.Fatalf("install: %+v", p)
	}
	again, err := s.CommandFurniture(ctx, uid, cmd)
	if err != nil || again.Revision != 1 || again.Inventory["wood"] != 990 {
		t.Fatalf("retry charged twice: %+v %v", again, err)
	}
	bad := cmd
	bad.Count = 1
	if _, err := s.CommandFurniture(ctx, uid, bad); !errors.Is(err, ErrFurnitureConflict) {
		t.Fatalf("key reuse: %v", err)
	}
	cmd = FurnitureCommand{RequestID: "enable-0001", FurnitureID: "test_desk", Version: "v1", Action: "enable", Revision: 1}
	p, err = s.CommandFurniture(ctx, uid, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].EnabledUntil == nil {
		t.Fatal("not enabled")
	}
	cmd.RequestID = "enable-0002"
	cmd.Revision = p.Revision
	if p, err = s.CommandFurniture(ctx, uid, cmd); err != nil {
		t.Fatal(err)
	}
	var periods int
	if err = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM player_cattery_furniture_periods`).Scan(&periods); err != nil || periods != 1 {
		t.Fatalf("no-op enable grew history: %d %v", periods, err)
	}
	start := time.Now().Add(-time.Hour)
	if _, err = s.db.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,status,actor_type,current_run_started_at,current_run_ends_at) VALUES(1,$1,'running','kitty',$2,$3)`, uid, start, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	cmd.RequestID = "pause-00001"
	cmd.Revision = p.Revision
	cmd.Action = "pause"
	p, err = s.CommandFurniture(ctx, uid, cmd)
	if err != nil {
		t.Fatalf("pause blocked by recovery: %v", err)
	}
	if p.Items[0].EnabledUntil != nil {
		t.Fatal("pause failed")
	}
	// Make capacity available while preserving real historical queue work.
	if _, err = s.db.Exec(ctx, `UPDATE player_cattery_furniture SET remaining_ms=0 WHERE user_id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	cmd.RequestID = "refill-0001"
	cmd.Revision = p.Revision
	cmd.Action = "refill"
	cmd.Count = 1
	if _, err = s.CommandFurniture(ctx, uid, cmd); !errors.Is(err, ErrOperationEffectRecoveryPending) {
		t.Fatalf("historical refill accepted: %v", err)
	}
	if _, err = s.db.Exec(ctx, `DELETE FROM player_action_queue`); err != nil {
		t.Fatal(err)
	}
	p, err = s.CommandFurniture(ctx, uid, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Inventory["wood"] != 988 || p.Inventory["fiber"] != 987 || p.Items[0].RemainingMS != 36000000 {
		t.Fatalf("refill: %+v", p)
	}
	stale := cmd
	stale.RequestID = "refill-stale"
	if _, err = s.CommandFurniture(ctx, uid, stale); !errors.Is(err, ErrFurnitureConflict) {
		t.Fatal("stale revision accepted")
	}
	// Failure of the second sorted material must roll back the first debit.
	if _, err = s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=0 WHERE template_item_id='wood'`); err != nil {
		t.Fatal(err)
	}
	cmd.RequestID = "refill-fail"
	cmd.Revision = p.Revision
	if _, err = s.CommandFurniture(ctx, uid, cmd); !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("expected insufficient material: %v", err)
	}
	after, err := s.GetFurniture(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if after.Inventory["fiber"] != 987 || after.Revision != p.Revision || after.Items[0].RemainingMS != p.Items[0].RemainingMS {
		t.Fatal("partial debit or state committed")
	}
}

func TestFurniturePostgresHistoricalSessionAndBattleFreeze(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	p, err := s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "install-0001", FurnitureID: "test_desk", Version: "v1", Action: "install"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "enable-00001", FurnitureID: "test_desk", Version: "v1", Action: "enable", Revision: p.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(ctx, `INSERT INTO battle_room_members VALUES(4,$1)`, uid); err != nil {
		t.Fatal(err)
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		raw, at, err := s.freezeBattlePassiveTx(ctx, tx, 4)
		if err != nil {
			return err
		}
		var frozen map[string]map[int64]operationeffect.PassiveSnapshot
		if err = json.Unmarshal([]byte(raw), &frozen); err != nil {
			return err
		}
		if len(frozen["passiveSources"][uid].Sources) != 1 || frozen["passiveSources"][uid].Sources[0].Rules[0].Kind != operationeffect.KindExtraLoot {
			t.Fatalf("bad battle freeze %s", raw)
		}
		past := at.Add(-time.Hour)
		if _, err = tx.Exec(ctx, `UPDATE player_cattery_furniture_periods SET valid_from=$1,valid_until=$2`, past, past.Add(time.Minute)); err != nil {
			return err
		}
		session, err := s.loadPassiveSessionTx(ctx, tx, uid, past, at)
		if err != nil {
			return err
		}
		if len(session.Windows) != 1 {
			t.Fatal("expired history dropped")
		}
		snap := session.Snapshot(operationeffect.Operation{Domain: operationeffect.DomainLifeAction, ActorKind: operationeffect.ActorKitty, StartedAt: past})
		g, err := session.Share(snap, map[string]float64{"fishing": 0.0001})
		if err != nil {
			return err
		}
		if g["fishing"] != 0 {
			t.Fatal("rounding carry lost")
		}
		effects, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
		effects.SetPassive(session)
		if _, err = s.flushOperationEffectSessionTx(ctx, tx, uid, effects, at); err != nil {
			return err
		}
		restored, err := s.loadPassiveSessionTx(ctx, tx, uid, past, at)
		if err != nil {
			return err
		}
		if restored.Remainders["fishing"] != 1500 {
			t.Fatal("carry not persisted")
		}
		payload, _ := json.Marshal(queueRuntimeEnvelope{PassiveEffects: &snap})
		if _, err = tx.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,status,actor_type,payload,current_run_started_at,current_run_ends_at) VALUES(77,$1,'running','kitty',$2,$3,$4)`, uid, string(payload), past, past.Add(2*time.Minute)); err != nil {
			return err
		}
		unified, err := s.loadOperationEffectSessionTx(ctx, tx, uid, at)
		if err != nil {
			return err
		}
		if unified == nil || !unified.Passive().ProgressOrdering {
			t.Fatal("frozen cross-actor XP did not request chronological starts")
		}
		if _, err = tx.Exec(ctx, `UPDATE player_action_queue SET status='pending' WHERE id=77`); err != nil {
			return err
		}
		pending, err := s.loadOperationEffectSessionTx(ctx, tx, uid, past.Add(30*time.Second))
		if err != nil {
			return err
		}
		if pending == nil || !pending.Passive().ProgressOrdering {
			t.Fatal("pending cat lane did not request chronological starts")
		}
		unowned, err := s.loadOperationEffectSessionTx(ctx, tx, 970042, at)
		if err != nil {
			return err
		}
		if unowned != nil {
			t.Fatal("unowned user left empty fast path")
		}
		other, err := s.loadPassiveSessionTx(ctx, tx, 970042, past, at)
		if err != nil {
			return err
		}
		if other != nil {
			t.Fatal("cross-account source leak")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
