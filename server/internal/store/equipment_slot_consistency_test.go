package store

import (
	"strings"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestEquipmentTemplateAllowsEquipSlot(t *testing.T) {
	tests := []struct {
		name      string
		template  *catalog.EquipmentTemplate
		equipType string
		slot      string
		want      bool
	}{
		{
			name:      "single slot work head",
			template:  &catalog.EquipmentTemplate{EquipType: "work", Slot: "head"},
			equipType: "work",
			slot:      "head",
			want:      true,
		},
		{
			name:      "equip type mismatch",
			template:  &catalog.EquipmentTemplate{EquipType: "work", Slot: "head"},
			equipType: "battle",
			slot:      "head",
			want:      false,
		},
		{
			name:      "accessory physical slot",
			template:  &catalog.EquipmentTemplate{EquipType: "battle", Slot: "accessory"},
			equipType: "battle",
			slot:      "accessory_3",
			want:      true,
		},
		{
			name:      "raw accessory slot is not physical",
			template:  &catalog.EquipmentTemplate{EquipType: "battle", Slot: "accessory"},
			equipType: "battle",
			slot:      "accessory",
			want:      false,
		},
		{
			name:      "occupy slot",
			template:  &catalog.EquipmentTemplate{EquipType: "battle", Slot: "right_hand", OccupySlots: []string{"right_hand", "left_hand"}},
			equipType: "battle",
			slot:      "left_hand",
			want:      true,
		},
		{
			name:      "unknown slot",
			template:  &catalog.EquipmentTemplate{EquipType: "battle", Slot: "right_hand", OccupySlots: []string{"right_hand", "left_hand"}},
			equipType: "battle",
			slot:      "head",
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := equipmentTemplateAllowsEquipSlot(tc.template, tc.equipType, tc.slot); got != tc.want {
				t.Fatalf("equipmentTemplateAllowsEquipSlot()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestEquipmentTemplateAllowsKitty(t *testing.T) {
	tests := []struct {
		name     string
		template *catalog.EquipmentTemplate
		want     bool
	}{
		{name: "missing template", template: nil, want: false},
		{name: "shared target by default", template: &catalog.EquipmentTemplate{}, want: true},
		{name: "kitty target", template: &catalog.EquipmentTemplate{EquipTargets: []string{"kitty"}}, want: true},
		{name: "player target only", template: &catalog.EquipmentTemplate{EquipTargets: []string{"player"}}, want: false},
		{name: "player and kitty targets", template: &catalog.EquipmentTemplate{EquipTargets: []string{"player", "kitty"}}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := equipmentTemplateAllowsKitty(tc.template); got != tc.want {
				t.Fatalf("equipmentTemplateAllowsKitty()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidPlayerEquippedItemsQueryUsesSlotDrivenEquippedFilter(t *testing.T) {
	query := validPlayerEquippedItemsQuery()
	required := []string{
		"FROM player_equip_slots s",
		"JOIN player_item_instances i ON i.instance_id = s.instance_id AND i.user_id = s.user_id",
		"WHERE s.user_id = $1",
		"AND i.state = 'equipped'",
	}
	for _, needle := range required {
		if !strings.Contains(query, needle) {
			t.Fatalf("valid equipment query missing %q:\n%s", needle, query)
		}
	}
	if strings.Contains(query, "LEFT JOIN") {
		t.Fatalf("valid equipment query should not left join hot-path rows:\n%s", query)
	}
}
