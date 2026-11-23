package store

import (
	"math/rand"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestTemplateDefaultAffixSlotsDoesNotRollIncompleteDefaults(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{
					ID:             "legacy_robe",
					AffixSlotCount: 1,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.legacy"},
					},
				},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.legacy", AllowedRarities: []string{"rare"}, Active: true},
			},
		},
	}

	if got := s.templateDefaultAffixSlots("legacy_robe"); len(got) != 0 {
		t.Fatalf("templateDefaultAffixSlots rolled incomplete default affix: %#v", got)
	}
}

func TestRollTemplateDefaultAffixSlotsUsesExplicitPolicyWeights(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{
					ID:             "mist_charm",
					AffixSlotCount: 1,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.mist"},
					},
				},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{
					ID:              "affix.mist",
					AllowedRarities: []string{"rare"},
					Active:          true,
					ValuePrecision:  2,
					EffectType:      "battle_hook",
					EffectParams: map[string]any{
						"durationTurns": map[string]any{"rare": 2},
						"attrMods": map[string]any{
							"rare": map[string]any{
								"hp": map[string]any{"min": 10, "max": 20},
							},
						},
					},
				},
			},
		},
	}

	got, err := s.rollTemplateDefaultAffixSlots(
		"mist_charm",
		map[string]float64{"common": 100, "rare": 1},
		rand.New(rand.NewSource(1)),
	)
	if err != nil {
		t.Fatalf("rollTemplateDefaultAffixSlots returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got)=%d, want 1", len(got))
	}
	slot := got[0]
	if slot.AffixID != "affix.mist" || slot.RarityID != "rare" || slot.SlotIndex != 0 {
		t.Fatalf("slot=%#v, want affix.mist rare slot 0", slot)
	}
	if slot.RollValue < 0 || slot.RollValue > 1 {
		t.Fatalf("RollValue=%v, want [0,1]", slot.RollValue)
	}
	if slot.EffectType != "battle_hook" || slot.ValuePrecision != 2 {
		t.Fatalf("slot metadata=%#v, want effect type and precision copied", slot)
	}
	if len(slot.AttrMods) == 0 {
		t.Fatalf("AttrMods empty, want rolled affix attrs")
	}
}

func TestRollTemplateDefaultAffixSlotsRequiresCompatibleRarity(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{
					ID:             "mist_charm",
					AffixSlotCount: 1,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.mist"},
					},
				},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.mist", AllowedRarities: []string{"rare"}, Active: true},
			},
		},
	}

	if _, err := s.rollTemplateDefaultAffixSlots("mist_charm", map[string]float64{"common": 100}, rand.New(rand.NewSource(1))); err != ErrAffixEnchantNoRarityCandidate {
		t.Fatalf("error=%v, want ErrAffixEnchantNoRarityCandidate", err)
	}
}
