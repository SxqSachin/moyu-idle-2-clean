package store

import (
	"encoding/json"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"testing"
	"time"
)

func TestFishCuisineBulkGrowthAndMutationBounds(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, tc := range []struct {
		id                     string
		base, maximum, mutated float64
	}{{"mossfire_dual_fish_grill", 50, 200, 363}, {"mossfire_dual_fish_soup", 45, 180, 327}} {
		recipe := *s.gameData.FishFoodRecipeByActionID("cook_" + tc.id)
		def, _, err := s.resolveOperationEffectDefinitionForItem(tc.id, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, sample := range []struct{ l, w, trait, mutation, want float64 }{{0, 0, 0, 1, tc.base}, {1, 1, 0, 1, tc.maximum}, {1, 1, 1, 1.815, tc.mutated}} {
			q := fishFoodQualitySnapshot{LengthScore: sample.l, WeightScore: sample.w, TraitAverage: sample.trait, SpecialMultiplier: sample.mutation}
			raw, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, &q)
			if err != nil {
				t.Fatal(err)
			}
			var params map[string]float64
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatal(err)
			}
			if params["extraMaterialQuantity"] != sample.want {
				t.Fatalf("%s %+v: %v", tc.id, sample, params)
			}
		}
	}
}

func TestFishCuisineBulkEnrichmentUsesActualMaterials(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	for _, materials := range []map[string]int64{{"coal": 1}, {"base4_abyssal_core": 1}, {"unknown": 1}} {
		args, _ := json.Marshal(alchemyBenchArgs{Materials: materials})
		raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: args})
		op := operationeffect.Operation{Capabilities: map[string]bool{alchemyeffect.CapabilityInfusion: true}}
		got := e.EnrichOperationEffectContext(op, raw)
		want := false
		for id := range materials {
			if item := s.gameData.ItemByID(id); item != nil {
				want = alchemyeffect.BulkMaterialEligible(item.Rarity)
			}
		}
		if got.Capabilities[alchemyeffect.CapabilityBulkMaterial] != want {
			t.Fatalf("%v: %v", materials, got)
		}
	}
}

func TestFishCuisineBulkChargesAndHistoricalStart(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := s.operationEffectDefinitions["food_mossfire_dual_fish_grill_v4"]
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1}})
	if err != nil {
		t.Fatal(err)
	}
	var high string
	for _, item := range s.gameData.Items {
		if item.Rarity == "mythic" && s.gameData.ItemTraitByItemID(item.ID) != nil {
			high = item.ID
			break
		}
	}
	if high == "" {
		t.Fatal("missing high rarity fixture")
	}
	op := func(sequence int, materials map[string]int64, start time.Time) operationeffect.Operation {
		args, _ := json.Marshal(alchemyBenchArgs{Materials: materials})
		raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: args})
		item := QueueRuntimeItem{QueueID: 7, ActionID: "alchemy.bench", ActorType: ActionActorTypePlayer}
		return enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, start, sequence, raw), raw)
	}
	for i, materials := range []map[string]int64{{high: 1}, {high: 1, "coal": 1}, {"coal": 1}} {
		snapshot, err := session.Commit(session.Prepare(op(i+1, materials, at.Add(time.Duration(i)*time.Second))))
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if i == 1 {
			want = 1
		}
		if len(snapshot.Rules) != want {
			t.Fatalf("step %d rules=%d want=%d", i, len(snapshot.Rules), want)
		}
	}
	def = s.operationEffectDefinitions["food_mossfire_dual_fish_soup_v4"]
	until := at.Add(10 * time.Minute)
	session, err = operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 2, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, ValidUntil: &until}})
	if err != nil {
		t.Fatal(err)
	}
	for i, start := range []time.Time{at.Add(-time.Second), until.Add(-time.Millisecond), until, until.Add(time.Hour)} {
		snapshot, err := session.Commit(session.Prepare(op(i+1, map[string]int64{"coal": 1}, start)))
		if err != nil {
			t.Fatal(err)
		}
		m, err := alchemyeffect.Resolve(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		want := int64(0)
		if i == 1 {
			want = 45
		}
		if m.BulkMaterialQuantity != want {
			t.Fatalf("historical start %v bonus=%d want=%d", start, m.BulkMaterialQuantity, want)
		}
	}
}

func TestFishCuisinePostgresRetiredCapacityDoesNotBlockNewFood(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"food_mossfire_dual_fish_grill", "food_mossfire_dual_fish_soup_v4"} {
		def := s.operationEffectDefinitions[id]
		raw, _ := json.Marshal(def)
		if _, err := s.activateOperationEffectGrantTx(ctx, tx, 11, "food", def, operationeffect.Target{Kind: def.AllowedTargets[0]}, raw, at); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
}
