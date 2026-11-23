package store

import (
	"strings"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestKittyAttributeBaseDefaultForActionEfficiency(t *testing.T) {
	store := &Store{
		gameData: catalog.Data{
			Attributes: []catalog.AttributeDefinition{
				{ID: "collecting_speed", RelatedActionMainType: "collecting", RelatedActionEffect: "actionEfficiency"},
				{ID: "strength"},
			},
		},
	}

	if got := store.kittyAttributeBaseDefault("collecting_speed", 0); got != -1 {
		t.Fatalf("kittyAttributeBaseDefault()=%v, want -1", got)
	}
	if got := store.kittyAttributeBaseDefault("fishing_efficiency", 0); got != 0 {
		t.Fatalf("kittyAttributeBaseDefault(fishing_efficiency)=%v, want 0", got)
	}
	if got := store.kittyAttributeBaseDefault("strength", 3); got != 3 {
		t.Fatalf("kittyAttributeBaseDefault()=%v, want 3", got)
	}
}

func TestListEquippedItemsQueryOnlyReadsPlayerEquipSlots(t *testing.T) {
	query := listEquippedItemsQuery()

	if !strings.Contains(query, "FROM player_equip_slots s") {
		t.Fatalf("query should be driven by player_equip_slots, got:\n%s", query)
	}
	if !strings.Contains(query, "JOIN player_item_instances i ON i.instance_id = s.instance_id AND i.user_id = s.user_id") {
		t.Fatalf("query should join instances through player equip slots, got:\n%s", query)
	}
	if strings.Contains(query, "LEFT JOIN player_equip_slots") {
		t.Fatalf("query must not use left join against player_equip_slots, got:\n%s", query)
	}
}
