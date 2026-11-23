package store

import (
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestGroupLoadoutEquipmentDedupAndSort(t *testing.T) {
	got := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "right_hand", InstanceID: 11},
		{Slot: "left_hand", InstanceID: 11},
		{Slot: "left_hand", InstanceID: 11},
		{Slot: "head", InstanceID: 7},
	})
	if len(got) != 2 {
		t.Fatalf("len(grouped)=%d, want 2", len(got))
	}
	if got[0].InstanceID != 7 || len(got[0].Slots) != 1 || got[0].Slots[0] != "head" {
		t.Fatalf("grouped[0]=%+v, want instance 7/head", got[0])
	}
	if got[1].InstanceID != 11 || len(got[1].Slots) != 2 || got[1].Slots[0] != "left_hand" || got[1].Slots[1] != "right_hand" {
		t.Fatalf("grouped[1]=%+v, want instance 11/left+right", got[1])
	}
}

func TestGroupedLoadoutEquipmentEqualIgnoresSlotOrderAndDuplicates(t *testing.T) {
	current := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "right_hand", InstanceID: 11},
		{Slot: "head", InstanceID: 7},
		{Slot: "left_hand", InstanceID: 11},
	})
	target := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "left_hand", InstanceID: 11},
		{Slot: "left_hand", InstanceID: 11},
		{Slot: "right_hand", InstanceID: 11},
		{Slot: "head", InstanceID: 7},
	})
	if !groupedLoadoutEquipmentEqual(current, target) {
		t.Fatalf("expected grouped loadout equipment to match")
	}
}

func TestGroupedLoadoutEquipmentEqualDetectsSlotMismatch(t *testing.T) {
	current := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "right_hand", InstanceID: 11},
		{Slot: "left_hand", InstanceID: 11},
	})
	target := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "right_hand", InstanceID: 11},
	})
	if groupedLoadoutEquipmentEqual(current, target) {
		t.Fatalf("expected grouped loadout equipment to differ when a slot is missing")
	}
}

func TestGroupedLoadoutEquipmentEqualDetectsInstanceMismatch(t *testing.T) {
	current := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "head", InstanceID: 7},
	})
	target := groupLoadoutEquipment([]LoadoutEquipmentSlot{
		{Slot: "head", InstanceID: 8},
	})
	if groupedLoadoutEquipmentEqual(current, target) {
		t.Fatalf("expected grouped loadout equipment to differ when instance changes")
	}
}

func TestMeetStatusRequireWithTotals(t *testing.T) {
	if !meetStatusRequireWithTotals(map[string]float64{"mining": 15, "strength": 10}, map[string]int64{"mining": 15}) {
		t.Fatalf("expected totals to meet requirement")
	}
	if meetStatusRequireWithTotals(map[string]float64{"mining": 14.9999}, map[string]int64{"mining": 15}) {
		t.Fatalf("expected totals to fail requirement")
	}
}

func TestActionEfficiencyBonusWithTotals(t *testing.T) {
	store := &Store{
		gameData: catalog.Data{
			Attributes: []catalog.AttributeDefinition{
				{ID: "mining_speed", RelatedActionMainType: "mining", RelatedActionEffect: "actionEfficiency"},
				{ID: "mining_bonus_legacy", RelatedActionMainType: "mining", RelatedActionEffect: ""},
				{ID: "craft_speed", RelatedActionMainType: "crafting", RelatedActionEffect: "actionEfficiency"},
			},
		},
	}
	got := store.actionEfficiencyBonusWithTotals(map[string]float64{
		"mining_speed":        0.5,
		"mining_bonus_legacy": 0.25,
		"craft_speed":         9,
	}, "mining")
	if got != 0.75 {
		t.Fatalf("actionEfficiencyBonusWithTotals()=%v, want 0.75", got)
	}
}
