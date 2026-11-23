package store

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/operationeffect"
)

func TestTrainingFurniturePostgresPermanentLifecycle(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	d := &s.gameData.CatteryFurniture[0]
	d.Service, d.CapacitySeconds, d.RefillSeconds, d.RefillCosts = "permanent", 0, 0, nil
	d.Rules = trainingSource(1).Rules
	for level := 2; level <= 10; level++ {
		d.Levels = append(d.Levels, catteryfurniture.Level{Level: level, UpgradeCosts: map[string]int64{"wood": 20, "fiber": 10}, Rules: trainingSource(level).Rules})
	}
	migration, err := os.ReadFile("../../schema/migrations/20260906_cattery_furniture_permanent.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := s.db.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	install := FurnitureCommand{RequestID: "install-permanent", FurnitureID: d.ID, Version: d.Version, Action: "install"}
	p, err := s.CommandFurniture(ctx, uid, install)
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].EnabledUntil != nil || p.Items[0].RemainingMS != 0 || p.Items[0].RefillAllowed || !p.Items[0].Installed {
		t.Fatal("invalid permanent state")
	}
	if _, err := s.CommandFurniture(ctx, uid, install); err != nil {
		t.Fatal("install retry", err)
	}
	for _, action := range []string{"enable", "pause", "refill"} {
		_, err := s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "invalid-" + action, FurnitureID: d.ID, Version: d.Version, Revision: p.Revision, Action: action, Count: 1})
		if !errors.Is(err, ErrFurnitureInvalid) {
			t.Fatal("permanent maintenance accepted", action, err)
		}
	}
	var installedAt time.Time
	if err := s.db.QueryRow(ctx, `SELECT valid_from FROM player_cattery_furniture_periods`).Scan(&installedAt); err != nil {
		t.Fatal(err)
	}
	// A running old operation retains the history required by its frozen start.
	if _, err := s.db.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,status,actor_type,current_run_started_at,current_run_ends_at) VALUES(1,$1,'running','kitty',$2,$3)`, uid, installedAt, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for level := 2; level <= 10; level++ {
		q := p.Items[0].Upgrade
		if q == nil || len(q.SupplementCosts) != 0 || q.RemainingMS != 0 || q.TotalCosts["wood"] != 20 {
			t.Fatal("permanent quote added maintenance", q)
		}
		cmd := FurnitureCommand{RequestID: "upgrade-permanent-" + string(rune('a'+level)), FurnitureID: d.ID, Version: d.Version, Revision: p.Revision, Action: "upgrade", UpgradeQuote: &q.FurnitureQuoteIdentity}
		if level == 2 {
			if _, err := s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=0 WHERE template_item_id='wood'`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.CommandFurniture(ctx, uid, cmd); !errors.Is(err, ErrInsufficientItem) {
				t.Fatal("upgrade should rollback", err)
			}
			failed, err := s.GetFurniture(ctx, uid)
			if err != nil || failed.Revision != p.Revision || failed.Inventory["fiber"] != 990 {
				t.Fatal("partial debit or level committed", err)
			}
			if _, err := s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=990 WHERE template_item_id='wood'`); err != nil {
				t.Fatal(err)
			}
		}
		p, err = s.CommandFurniture(ctx, uid, cmd)
		if err != nil {
			t.Fatal(err)
		}
		retry, err := s.CommandFurniture(ctx, uid, cmd)
		if err != nil || retry.Revision != p.Revision {
			t.Fatal("upgrade retry", err)
		}
		if p.Items[0].Definition.CurrentLevel() != level || p.Items[0].RemainingMS != 0 {
			t.Fatal("lost permanent level")
		}
	}
	if p.Items[0].Upgrade != nil || p.Inventory["wood"] != 810 || p.Inventory["fiber"] != 900 {
		t.Fatal("wrong max level or payment", p)
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		passive, err := s.loadPassiveSessionTx(ctx, tx, uid, installedAt, time.Now().Add(100*365*24*time.Hour))
		if err != nil {
			return err
		}
		for _, window := range passive.Windows {
			op := operationeffect.Operation{ActorKind: operationeffect.ActorKitty, Domain: operationeffect.DomainActionStart, MainType: "collecting", StartedAt: window.From}
			if passive.Snapshot(op).EfficiencyBPS() != window.Source.Rules[0].RateBPS {
				t.Fatal("upgrade rewrote start boundary")
			}
		}
		if len(passive.Windows) != 10 || !passive.Windows[9].Until.IsZero() {
			t.Fatal("missing permanent history")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTrainingFurniturePostgresOnlineOfflineAndChunked(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	s.furniturePolicy = nil
	const uid int64 = 970041
	s.gameData.Attributes = []catalog.AttributeDefinition{{ID: "collecting_efficiency", RelatedActionMainType: "collecting", RelatedActionEffect: "actionEfficiency"}}
	if _, err := s.db.Exec(ctx, `CREATE TEMP TABLE player_kitty_attributes(kitty_id bigint,attr_key text,final_value float8);
 INSERT INTO player_kitties VALUES(7,970041,'idle');
 INSERT INTO player_kitty_attributes VALUES(7,'collecting_efficiency',-1);
 INSERT INTO player_cattery_furniture_state(user_id) VALUES(970041);`); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(1000, 0)
	for i, level := range []int{5, 10} {
		from := at.Add([]time.Duration{95 * time.Second, 500 * time.Second}[i])
		var until *time.Time
		if i == 0 {
			end := at.Add(500 * time.Second)
			until = &end
		}
		raw, _ := json.Marshal(trainingSource(level))
		if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_periods(user_id,furniture_id,valid_from,valid_until,source) VALUES($1,'training',$2,$3,$4)`, uid, from, until, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	expected := at
	for n := 0; n < 100; n++ {
		duration := 40 * time.Second
		if !expected.Before(at.Add(500 * time.Second)) {
			duration = 14285 * time.Millisecond
		} else if !expected.Before(at.Add(95 * time.Second)) {
			duration = 20 * time.Second
		}
		expected = expected.Add(duration)
	}
	for _, chunk := range []int{1, 7, 1000} {
		if _, err := s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=1000`); err != nil {
			t.Fatal(err)
		}
		item := QueueRuntimeItem{QueueID: 71, ActorType: ActionActorTypeKitty, ActorID: 7, ActionID: "training_gather", Spec: ActionSpec{ID: "training_gather", MainType: "collecting", DurationS: 20, Costs: map[string]int64{"wood": 1}}, TotalCount: 100, PreserveScheduledTimeline: true}
		executor := newGenericExecutor(s)
		var last time.Time
		batches := 0
		for item.DoneCount < item.TotalCount {
			err := s.withTx(ctx, func(tx pgx.Tx) error {
				p, err := s.loadPassiveSessionTx(ctx, tx, uid, at, at.Add(10000*time.Second))
				if err != nil {
					return err
				}
				effects, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
				effects.SetPassive(p)
				item.EffectSession = effects
				if item.RunStartAt == nil {
					item.PayloadRaw, err = executor.FreezeRunContextTx(ctx, tx, uid, item.ActorType, item.ActorID, item.Spec, genericExecutorArgs{}, at)
					if err != nil {
						return err
					}
					item.PayloadRaw, err = executor.StartRunTx(ctx, tx, uid, item, at)
					if err != nil {
						return err
					}
					start, end := at, at.Add(parseQueueRuntimeDuration(item.PayloadRaw))
					item.RunStartAt = &start
					item.RunEndsAt = &end
				}
				now := at.Add(10000 * time.Second)
				if chunk == 1 {
					now = *item.RunEndsAt
				}
				result, err := executor.SettleFastForwardTx(ctx, tx, uid, item, now, FastForwardBudget{MaxIterations: chunk})
				if err != nil {
					return err
				}
				if result.Completed <= 0 {
					t.Fatal("no progress")
				}
				item.DoneCount = result.DoneAfter
				item.PayloadRaw = result.NextPayload
				item.RunStartAt = &result.NextRunStartAt
				item.RunEndsAt = &result.NextRunEndsAt
				last = result.CompletedAt
				return nil
			})
			if err != nil {
				t.Fatalf("chunk %d: %v", chunk, err)
			}
			batches++
		}
		var stock int64
		if err := s.db.QueryRow(ctx, `SELECT quantity FROM player_item_instances WHERE template_item_id='wood'`).Scan(&stock); err != nil {
			t.Fatal(err)
		}
		if !last.Equal(expected) || stock != 900 {
			t.Fatalf("chunk %d: end %s want %s stock %d", chunk, last, expected, stock)
		}
		if chunk == 1000 && batches > 5 {
			t.Fatalf("lost batching: %d transactions", batches)
		}
	}
}
