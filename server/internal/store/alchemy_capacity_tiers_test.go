package store

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestCapacityFoodTiersScaleWithoutChangingScope(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, tc := range []struct {
		series          string
		tier            int
		before, maximum float64
	}{
		{"mossfire", 1, .12, .20}, {"lotusflame", 2, .10, .15}, {"frostmoon", 0, .09, .12},
	} {
		for _, form := range []string{"grill", "soup"} {
			id := tc.series + "_dual_fish_" + form
			def, ok, err := s.resolveOperationEffectDefinitionForItem(id, nil)
			if err != nil || !ok {
				t.Fatal(id, err)
			}
			recipe := s.gameData.FishFoodRecipeByOutputItemID(id)
			for _, sample := range []struct{ score, multiplier, want float64 }{{0, 1, .07}, {1, 1, tc.before}, {1, 1.815, tc.maximum}} {
				q := fishFoodQualitySnapshot{LengthScore: sample.score, WeightScore: sample.score, SpecialMultiplier: sample.multiplier}
				raw, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, &q)
				if err != nil {
					t.Fatal(id, err)
				}
				var p map[string]float64
				if err := json.Unmarshal(raw, &p); err != nil {
					t.Fatal(err)
				}
				if p["quantityLimitBonusRate"] != sample.want || int(p["materialTier"]) != tc.tier {
					t.Fatal(id, sample, p)
				}
				mods, err := alchemyeffect.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: def.Domain, Kind: def.Kind, Params: raw}}})
				if err != nil {
					t.Fatal(err)
				}
				for _, rarity := range []string{"common", "uncommon", "rare", "epic", "legendary", "mythic"} {
					eligible := (tc.tier > 0 && alchemyeffect.MaterialTier(rarity) == tc.tier) || (tc.tier == 0 && alchemyeffect.PreciousMaterialEligible(rarity))
					want := int64(0)
					if eligible {
						want = int64(math.Round(10000 * sample.want))
					}
					if got := mods.QuantityBonus(rarity, 10000); got != want {
						t.Fatal(id, rarity, got, want)
					}
				}
			}
		}
	}
}

func TestTierContextAndThreePreciousMaterialsSpendOneUse(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	materials := map[string]int64{}
	for _, item := range s.gameData.Items {
		if item.Rarity == "legendary" && s.gameData.ItemTraitByItemID(item.ID) != nil {
			materials[item.ID] = 1
		}
		if len(materials) == 3 {
			break
		}
	}
	if len(materials) != 3 {
		t.Fatal("missing legendary fixtures")
	}
	args, _ := json.Marshal(alchemyBenchArgs{Materials: materials})
	raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: args})
	for _, actor := range []string{ActionActorTypePlayer, ActionActorTypeKitty} {
		item := QueueRuntimeItem{QueueID: 1, ActionID: "alchemy.bench", ActorType: actor, ActorID: 2}
		op := enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, at, 1, raw), raw)
		grants := []operationeffect.Grant{}
		for i, id := range []string{"food_mossfire_dual_fish_grill_v7", "food_lotusflame_dual_fish_grill_v1", "food_frostmoon_dual_fish_grill_v4"} {
			def := s.operationEffectDefinitions[id]
			grants = append(grants, operationeffect.Grant{ID: int64(i + 1), Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1})
		}
		session, err := operationeffect.NewSession(s.operationEffectRegistry, grants)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := session.Commit(session.Prepare(op))
		if err != nil || len(snapshot.Rules) != 1 || len(session.ChargeReceipts()) != 1 {
			t.Fatal(actor, snapshot, err)
		}
		mods, err := alchemyeffect.Resolve(snapshot)
		if err != nil || mods.SuccessRatePenalty != .06 {
			t.Fatal(actor, mods, err)
		}
		for id := range materials {
			if s.alchemyMaterialQuantityBonus(id, 10000, mods) != 700 {
				t.Fatal("a legendary slot missed its bonus", id)
			}
		}
		if _, err := session.Commit(session.Prepare(op)); err != nil || len(session.ChargeReceipts()) != 1 {
			t.Fatal("retry spent another use", err)
		}
	}
}

func TestCurrentCapacityFoodMatchesOnlyActualMaterialTier(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	for _, rarity := range []string{"common", "uncommon", "rare", "epic", "legendary", "mythic"} {
		itemID := ""
		for _, item := range s.gameData.Items {
			if item.Rarity == rarity && s.gameData.ItemTraitByItemID(item.ID) != nil {
				itemID = item.ID
				break
			}
		}
		if itemID == "" {
			t.Fatal("missing rarity fixture", rarity)
		}
		for i, series := range []string{"mossfire", "lotusflame", "frostmoon"} {
			def, _, err := s.resolveOperationEffectDefinitionForItem(series+"_dual_fish_grill", nil)
			if err != nil {
				t.Fatal(err)
			}
			session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1}})
			if err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(alchemyBenchArgs{Materials: map[string]int64{itemID: 1}})
			raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: args})
			op := enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, QueueRuntimeItem{QueueID: 1, ActionID: "alchemy.bench", ActorType: ActionActorTypeKitty, ActorID: 9}, at, 1, raw), raw)
			snapshot, err := session.Commit(session.Prepare(op))
			want := alchemyeffect.MaterialTier(rarity) == i+1 || (i == 2 && alchemyeffect.PreciousMaterialEligible(rarity))
			if err != nil || (len(snapshot.Rules) == 1) != want {
				t.Fatal(series, rarity, snapshot, err)
			}
		}
	}
}

func TestCompetitiveGateAcceptsTierCeilingsOnlyWithMatchingProvenance(t *testing.T) {
	for _, tc := range []struct {
		rarity, other string
		tier          int
		rate          float64
	}{
		{"common", "rare", 1, .20}, {"uncommon", "epic", 1, .20}, {"rare", "common", 2, .15}, {"epic", "uncommon", 2, .15},
	} {
		s := arenaAlchemyGateTestStore()
		s.gameData.Items = []catalog.Item{{ID: "test_material", Rarity: tc.rarity}}
		s.gameData.ItemTraits[0].Traits[0].MaxBasisMultiplier = 1000
		quantity := int64(math.Round(32000 * (1 + tc.rate)))
		value := float64(quantity) * .01 * 2.25 * .75 * .999
		craft := resolvedAlchemyCraft{EffectType: "add_stat", EffectParams: map[string]any{"alchemyKind": "combat"}, AttrMods: map[string]float64{"speed": value}, Components: []alchemyCraftComponent{{SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait", Attr: "speed", Quantity: quantity, CurrentValue: value}}}
		params, _ := json.Marshal(alchemyeffect.CapacityFloorTradeoffParams{MaterialTier: tc.tier, ExtraMaterialQuantity: 1, QuantityLimitBonusRate: tc.rate, PositiveFloorReductionRate: .1})
		snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{DefinitionID: "tier-food", RulesetVersion: "food-capacity-tiers-v1", Domain: operationeffect.DomainAlchemy, Kind: alchemyeffect.KindInfusionBulkCapacityFloorTradeoff, Params: params}}}
		if err := stampAlchemyCraftOperationEffects(&craft, snapshot); err != nil {
			t.Fatal(err)
		}
		persisted, _ := json.Marshal(craft.EffectParams)
		craft.EffectParams = nil
		if err := json.Unmarshal(persisted, &craft.EffectParams); err != nil {
			t.Fatal(err)
		}
		item := EquippedItem{InstanceID: 1, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{alchemyCraftSlot(craft)}}
		if got := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(got) != 0 {
			t.Fatal(tc.rarity, got)
		}
		s.gameData.Items[0].Rarity = tc.other
		if got := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(got) == 0 {
			t.Fatal("tier scope was bypassed", tc.rarity)
		}
		s.gameData.Items[0].Rarity = tc.rarity
		item.Affixes[0].AlchemyRulesetVersion = ""
		if got := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(got) == 0 {
			t.Fatal("unstamped item received blanket inflation", tc.rarity)
		}
	}
}
