package store

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func TestOrdinaryPostgresSharedActorsAndQueueHandoff(t *testing.T) {
	type proof struct {
		steps               int64
		inventory, receipts string
	}
	run := func(t *testing.T, budget int) proof {
		fixture, ctx, _ := recoveryPostgresStore(t)
		s := New(fixture.db, fixture.config, fixture.gameData)
		s.config.OrdinarySettlementEnabled = true
		at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		until := at.Add(time.Hour)
		tx, err := s.db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		if err := s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO player_kitties(id,user_id,name,status) VALUES(55,970071,'ordinary-proof','in_life_queue')`); err != nil {
			t.Fatal(err)
		}
		if err := s.AddInventoryItemTx(ctx, tx, 970071, "ordinary_input", 31); err != nil {
			t.Fatal(err)
		}
		def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
		def.ID, def.Kind, def.Params = "ordinary-shared", lifeaction.KindExtraMainOutputChance, json.RawMessage(`{"chance":1,"quantity":1}`)
		def.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}
		def.Selector = operationeffect.Selector{}
		def.Stacking = operationeffect.StackingDefinition{Key: def.ID, Mode: operationeffect.StackAddCharges}
		def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 17}
		raw, _ := json.Marshal(def)
		if _, err := s.activateOperationEffectGrantTx(ctx, tx, 970071, "proof-food", def, operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, raw, at); err != nil {
			t.Fatal(err)
		}
		effects, err := s.loadOperationEffectSessionTx(ctx, tx, 970071, until)
		if err != nil {
			t.Fatal(err)
		}
		specs := map[string]ActionSpec{}
		one := int64(1)
		for i, actor := range []string{ActionActorTypePlayer, ActionActorTypeKitty, ActionActorTypePlayer} {
			id := int64(i + 1)
			actionID := "logging"
			actorID := int64(0)
			if actor == ActionActorTypeKitty {
				actionID, actorID = "ordinary_kitty", 55
			}
			spec := ActionSpec{ID: actionID, MainType: "collecting", DurationS: 2, Costs: map[string]int64{"ordinary_input": 1}, ItemReward: map[string]ActionItemRewardSpec{actionID + "_output": {Count: &one}}, MainOutput: actionID + "_output"}
			specs[actionID] = spec
			total := 30
			if i == 0 {
				total = 3
			}
			item := QueueRuntimeItem{QueueID: id, ActorType: actor, ActorID: actorID, ActionID: actionID, Spec: spec, TotalCount: total, EffectSession: effects, PreserveScheduledTimeline: true}
			executor := newGenericExecutor(s)
			item.PayloadRaw, err = executor.FreezeRunContextTx(ctx, tx, 970071, actor, actorID, spec, genericExecutorArgs{}, at)
			if err != nil {
				t.Fatal(err)
			}
			status := "pending"
			var start, end *time.Time
			if i < 2 {
				status = "running"
				op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, at, 1, item.PayloadRaw)
				snapshot, err := effects.Commit(effects.Prepare(op))
				if err != nil {
					t.Fatal(err)
				}
				item.OperationEffects = &snapshot
				item.PayloadRaw, err = executor.StartRunTx(ctx, tx, 970071, item, at)
				if err != nil {
					t.Fatal(err)
				}
				item.PayloadRaw, err = setQueueOperationEffectSnapshot(item.PayloadRaw, snapshot)
				if err != nil {
					t.Fatal(err)
				}
				var envelope queueRuntimeEnvelope
				if err := json.Unmarshal(item.PayloadRaw, &envelope); err != nil {
					t.Fatal(err)
				}
				envelope.OrdinarySettlement.Seed = strings.Repeat("49", 32)
				item.PayloadRaw, err = json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
				a, b := at, at.Add(parseQueueRuntimeDuration(item.PayloadRaw))
				start, end = &a, &b
			}
			if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,actor_type,actor_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload) VALUES($1,970071,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`, id, actor, actorID, actionID, total, status, start, end, string(item.PayloadRaw)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.flushOperationEffectSessionTx(ctx, tx, 970071, effects, at); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var out proof
		for i := 0; i < 100; i++ {
			summary, err := s.AdvanceActionQueueOfflineChunk(ctx, 970071, specs, until, FastForwardBudget{MaxIterations: budget, MaxExec: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			out.steps += summary.CompletedSteps
			if !summary.HasMoreDue {
				break
			}
		}
		if err := s.db.QueryRow(ctx, `SELECT COALESCE(jsonb_object_agg(template_item_id,n),'{}'::jsonb)::text FROM (SELECT template_item_id,sum(quantity) n FROM player_item_instances WHERE user_id=970071 GROUP BY template_item_id) q`).Scan(&out.inventory); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(operation_key ORDER BY operation_key),'[]'::jsonb)::text FROM player_operation_effect_charge_receipts`).Scan(&out.receipts); err != nil {
			t.Fatal(err)
		}
		var charges, receipts int
		if err := s.db.QueryRow(ctx, `SELECT remaining_charges,(SELECT count(*) FROM player_operation_effect_charge_receipts) FROM player_operation_effect_grants`).Scan(&charges, &receipts); err != nil || charges != 0 || receipts != 17 {
			t.Fatalf("shared charges=%d receipts=%d err=%v", charges, receipts, err)
		}
		if out.steps != 31 {
			t.Fatalf("prefix affordability lost: %+v", out)
		}
		return out
	}
	var single, batch proof
	t.Run("single", func(t *testing.T) { single = run(t, 1) })
	t.Run("batch", func(t *testing.T) { batch = run(t, 1000) })
	if !reflect.DeepEqual(single, batch) {
		t.Fatalf("actor allocation or queue handoff changed\nsingle=%+v\nbatch=%+v", single, batch)
	}
}
