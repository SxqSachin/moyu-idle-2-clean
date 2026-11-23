package store

import (
	"context"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestAlchemyMediumAndOperationEffectsPreserveCraftAndFailureRanges(t *testing.T) {
	for _, comboID := range []string{"", "test_combo"} {
		t.Run("recipe="+comboID, func(t *testing.T) {
			s := arenaAlchemyGateTestStore()
			trait := &s.gameData.ItemTraits[0].Traits[0]
			trait.Family = "alchemy"
			trait.FailBaseChance = 1
			trait.Effects[0].ValuePerUnit = catalog.AlchemyValueRange{Min: 10, Max: 100}
			if comboID == "" {
				s.gameData.AlchemyComboRecipes = nil
			} else {
				combo := &s.gameData.AlchemyComboRecipes[0]
				combo.FailBaseChance = 1
				combo.OutputAffix = catalog.AlchemyComboOutputAffix{EffectType: "add_stat", Params: map[string]any{"attr": "speed"}, ValueRange: catalog.AlchemyValueRange{Min: 10, Max: 100}}
			}
			medium := &catalog.AlchemyMedium{Modifiers: catalog.AlchemyMediumModifiers{ValueMultiplier: 1.2, NegativeMultiplier: .7, FamilyAffinity: map[string]float64{"alchemy": .1}}}
			derived, err := s.resolveAlchemyDerivedForItemTx(context.Background(), nil, 1, QueueRuntimeItem{
				WorkLoadoutRuntime: &WorkLoadoutRuntimeResolution{Available: true, AttributeTotals: map[string]float64{"alchemy_level": 80, "alchemy_precision": 1, "alchemy_craft_success": 1}},
			})
			if err != nil {
				t.Fatal(err)
			}
			materials := map[string]int64{"test_material": 1}
			modifiers := alchemyeffect.Modifiers{PositiveFloorReductionRate: .2, PositiveCeilingBonusRate: .1}
			multiplier := 1.2
			if comboID == "" {
				multiplier *= 1.1
			}
			wantMin := s.roundAlchemyAttributeValue("speed", 50*.8*multiplier)
			wantMax := s.roundAlchemyAttributeValue("speed", 162.5*1.1*.85*multiplier)
			failures, successes := 0, 0
			for seed := int64(0); seed < 64; seed++ {
				craft, failed, err := s.resolveAlchemyBenchCraftOutcomeWithModifiers(materials, comboID, medium, derived, modifiers, rand.New(rand.NewSource(seed)))
				if err != nil {
					t.Fatal(err)
				}
				component := craft.Components[0]
				if !nearlyEqual(component.ValueRange.Min, wantMin) || !nearlyEqual(component.ValueRange.Max, wantMax) {
					t.Fatalf("unexpected component range: %+v", component.ValueRange)
				}
				value := craft.AttrMods["speed"]
				if value < wantMin || value > wantMax || !nearlyEqual(component.CurrentValue, value) {
					t.Fatalf("result and component differ: value=%v component=%+v", value, component)
				}
				if failed {
					failures++
					if !nearlyEqual(value, wantMin) {
						t.Fatal("failure lost adjusted floor")
					}
				} else {
					successes++
				}
			}
			if failures == 0 || successes == 0 {
				t.Fatal("both outcome branches must be exercised")
			}
			floor := s.computeFloorAttrModsWithModifiers(materials, comboID, medium, derived, modifiers)
			if !nearlyEqual(floor["speed"], wantMin) {
				t.Fatal("floor resolver disagrees with outcome")
			}
		})
	}
}

func TestAlchemyRangeBuffSnapshotSurvivesExpiryAndChargeExhaustion(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	at := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	until := at.Add(time.Minute)
	for _, id := range []string{"food_starlight_silverfish_grill_v2", "food_starlight_silverfish_soup_v2"} {
		t.Run(id, func(t *testing.T) {
			def := s.operationEffectDefinitions[id]
			grant := operationeffect.Grant{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at}
			if def.Lifetime.Kind == operationeffect.LifetimeCharges {
				grant.InitialCharges = 1
				grant.RemainingCharges = 1
			} else {
				grant.ValidUntil = &until
			}
			session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{grant})
			if err != nil {
				t.Fatal(err)
			}
			operation := func(seq int, start time.Time) operationeffect.Operation {
				run, _ := json.Marshal(alchemyBenchArgs{Materials: map[string]int64{"coal": 1}})
				raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: run})
				item := QueueRuntimeItem{QueueID: 17, ActionID: "alchemy.bench", ActorType: ActionActorTypePlayer}
				return enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, start, seq, raw), raw)
			}
			first := operation(1, until.Add(-time.Second))
			frozen, err := session.Commit(session.Prepare(first))
			if err != nil || len(frozen.Rules) != 1 {
				t.Fatal("first run did not receive effect", err)
			}
			retry, err := session.Commit(session.Prepare(first))
			if err != nil || !reflect.DeepEqual(frozen, retry) {
				t.Fatal("retry changed the snapshot", err)
			}
			after, err := session.Commit(session.Prepare(operation(2, until)))
			if err != nil || len(after.Rules) != 0 {
				t.Fatal("inactive effect applied to a new run", err)
			}
			payload, _ := json.Marshal(queueRuntimeEnvelope{OperationEffects: &frozen})
			mods, err := alchemyOperationEffectModifiers(QueueRuntimeItem{PayloadRaw: payload})
			if err != nil || mods.PositiveCeilingBonusRate <= 0 {
				t.Fatal("persisted run lost its range buff", err)
			}
			baseMin, baseMax := applyAlchemyInfusionOperationRange(10, 100, alchemyeffect.Modifiers{})
			min, max := applyAlchemyInfusionOperationRange(10, 100, mods)
			if min != baseMin || !nearlyEqual(max/baseMax, 1+mods.PositiveCeilingBonusRate) {
				t.Fatal("range buff magnitude changed")
			}
			craft := resolvedAlchemyCraft{EffectParams: map[string]any{}, Components: []alchemyCraftComponent{{SourceKind: "trait_effect"}}}
			if err := stampAlchemyCraftOperationEffects(&craft, frozen); err != nil {
				t.Fatal(err)
			}
			restored, err := resolveAlchemyOperationEffectStamp(craft.Components[0].OperationEffects)
			if err != nil || restored.PositiveCeilingBonusRate != mods.PositiveCeilingBonusRate {
				t.Fatal("craft provenance lost its buff", err)
			}
		})
	}
}

func TestAlchemyInfusionAndArenaUseTheSameBuffMultiplier(t *testing.T) {
	for _, bonus := range []float64{0, .03, .1, .25, 1} {
		mods := alchemyeffect.Modifiers{PositiveCeilingBonusRate: bonus}
		_, max := applyAlchemyInfusionOperationRange(0, 2.25, mods)
		if !nearlyEqual(max, arenaAlchemyDerivedPositiveMaxMultiplierWithModifiers(mods)) {
			t.Fatal("competitive validation differs from infusion")
		}
	}
}
