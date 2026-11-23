package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
)

func TestOfflineEnhancePlannerPostgresEquivalence(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	previews := map[int]EnhanceAttemptPreview{}
	for level := 0; level < 100; level++ {
		previews[level] = EnhanceAttemptPreview{NextLevel: level + 1, SuccessRate: .4, Costs: map[string]int64{"wood": int64(level + 1)}, DeltaAttrs: map[string]float64{"attack": float64(level + 1)}}
	}
	s.enhanceCache = &EnhanceRuntimeCache{byTemplateID: map[string]EnhanceTemplateRuntime{"proof": {TemplateID: "proof", MaxLevel: 100, NextPreviewByLevel: previews}}}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_item_instances(user_id,template_item_id,quantity) VALUES(970071,'wood',10000000)`); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"overnight", "prepaid_effects", "insufficient", "milestone"} {
		t.Run(scenario, func(t *testing.T) {
			tx, err := s.db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			runtime := enhanceExecutorRuntime{TemplateID: "proof", MaxLevel: 100, TargetEnhanceLevel: 100, EquipmentInstanceID: 42, DurationMs: 2000, EnhanceExp: .12345}
			if scenario == "insufficient" {
				runtime.UseProtect, runtime.ProtectStartLevel, runtime.ProtectItemID = true, 1, "wood"
				if _, err := tx.Exec(ctx, `UPDATE player_item_instances SET quantity=5 WHERE user_id=970071`); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "milestone" {
				r := s.enhanceCache.byTemplateID["proof"]
				r.AffixMilestonePools = map[int][]catalog.EnhanceAffixMilestonePoolEntry{2: {{AffixID: "proof", RarityID: "common"}}}
				s.enhanceCache.byTemplateID["proof"] = r
			}
			at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
			end := at.Add(2 * time.Second)
			item := QueueRuntimeItem{QueueID: 77, ActorType: ActionActorTypePlayer, ActionID: "equipment_enhance", TotalCount: 20000, RunStartAt: &at, RunEndsAt: &end, PreserveScheduledTimeline: true}
			e := newEnhanceExecutor(s)
			first, _, _ := e.buildEnhanceRunContext(runtime, 0, 0)
			var want enhanceFastForwardPlan
			var receipts []operationeffect.ChargeReceipt
			for _, optimized := range []bool{false, true} {
				registry := operationeffect.NewRegistry()
				if err := ratebonus.Register(registry); err != nil {
					t.Fatal(err)
				}
				def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
				def.Domain, def.Kind, def.Params = operationeffect.DomainEnhancement, ratebonus.EnhanceSuccess, []byte(`{"bonusRate":0.1}`)
				def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 7}
				def.Selector = operationeffect.Selector{ActionIDs: []string{"equipment_enhance"}}
				grants := []operationeffect.Grant{}
				if scenario == "prepaid_effects" {
					grants = append(grants, operationeffect.Grant{ID: 9, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at.Add(8 * time.Second), InitialCharges: 7, RemainingCharges: 7})
				}
				item.EffectSession, err = operationeffect.NewSession(registry, grants)
				if err != nil {
					t.Fatal(err)
				}
				began := time.Now()
				var got enhanceFastForwardPlan
				if optimized {
					got, err = e.planEnhanceFastForwardTx(ctx, tx, 970071, item, runtime, LockedItemInstance{EnhanceLevel: 0, TemplateItem: "proof"}, 14400, first, scenario == "prepaid_effects", time.Now(), FastForwardBudget{})
				} else {
					got, err = e.planEnhanceFastForwardReferenceTx(ctx, tx, 970071, item, runtime, LockedItemInstance{EnhanceLevel: 0, TemplateItem: "proof"}, 14400, first, scenario == "prepaid_effects", time.Now(), FastForwardBudget{})
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("optimized=%v attempts=%d elapsed=%s", optimized, got.AttemptsDone, time.Since(began))
				if !optimized {
					want, receipts = got, item.EffectSession.ChargeReceipts()
				} else if !reflect.DeepEqual(want, got) || !reflect.DeepEqual(receipts, item.EffectSession.ChargeReceipts()) {
					t.Fatal("planner results or charge receipts changed")
				}
			}
		})
	}
}

func TestOfflineFishingInsertPostgresEquivalenceAndRollback(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO fishing_trips(id,user_id,water_id,spot_id,loadout_id,strategy,safety_load_percent,phase_started_at,phase_ends_at,seed,started_at)
 VALUES('proof',970071,'water','spot','loadout','balanced',80,now(),now(),42,now())`); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 1, 128} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var want []byte
			for _, optimized := range []bool{false, true} {
				tx, err := s.db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				pending := make([]*fishingPendingCatch, count)
				for i := range pending {
					at := time.Date(2026, 9, 7, 0, 0, i, 0, time.UTC)
					pending[i] = &fishingPendingCatch{Fish: FishInstanceData{SpeciesID: fmt.Sprintf("species-%d", i%3), LengthCM: float64(i + 10), WeightKG: float64(i+1) / 10, RulesetVersion: FishingRulesetVersion, CaughtAt: at}, Kept: i%3 != 0, CaughtAt: at}
				}
				began := time.Now()
				if optimized {
					err = insertFishingCatchBatchTx(ctx, tx, 970071, "proof", pending)
				} else {
					err = insertFishingCatchBatchReferenceTx(ctx, tx, 970071, "proof", pending)
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("optimized=%v catches=%d insert=%s", optimized, count, time.Since(began))
				for i, p := range pending {
					if p.CatchID <= 0 || p.Kept != (p.InventoryInstanceID != nil) || (i > 0 && p.CatchID <= pending[i-1].CatchID) {
						t.Fatal("catch identity/order lost", i, p)
					}
				}
				var actual []byte
				if err = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('fish',c.fish_data_jsonb,'species',c.species_id,'kept',c.kept,'caught',c.caught_at,'inventory',i.fish_data,'quantity',i.quantity,'template',i.template_item_id,'state',i.state) ORDER BY c.id),'[]'::jsonb)
 FROM fishing_catches c LEFT JOIN player_item_instances i ON i.instance_id=c.inventory_instance_id WHERE c.trip_id='proof'`).Scan(&actual); err != nil {
					t.Fatal(err)
				}
				if !optimized {
					want = actual
				} else if string(want) != string(actual) {
					t.Fatalf("batch data changed:\n%s\n%s", want, actual)
				}
				if err = tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				var left int
				if err = s.db.QueryRow(ctx, `SELECT (SELECT count(*) FROM fishing_catches)+(SELECT count(*) FROM player_item_instances)`).Scan(&left); err != nil || left != 0 {
					t.Fatal("rollback leaked inventory", left, err)
				}
			}
		})
	}
	// A failed catch insert must not leave the instances inserted by the same statement.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	p := []*fishingPendingCatch{{Fish: FishInstanceData{SpeciesID: "fault"}, Kept: true, CaughtAt: time.Now()}}
	if err = insertFishingCatchBatchTx(ctx, tx, 970071, "missing-trip", p); err == nil {
		t.Fatal("missing foreign key accepted")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	if err = s.db.QueryRow(ctx, `SELECT count(*) FROM player_item_instances`).Scan(&left); err != nil || left != 0 {
		t.Fatal("failed statement leaked instance", left, err)
	}
}
