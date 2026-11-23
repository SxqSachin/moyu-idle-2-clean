package store

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func fishingCatchListSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("store_fishing.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) ListFishingCatches")
	end := strings.Index(body, "func (s *Store) pruneFishingCatchHistoryTx")
	if start < 0 || end <= start {
		t.Fatal("fishing catch list function boundaries not found")
	}
	return body[start:end]
}

func TestFishingCatchListRoutesKeptCollectionToCurrentInventory(t *testing.T) {
	query := fishingCatchListSource(t)
	for _, required := range []string{
		"if keptOnly {",
		"return s.listCurrentFishingInventory(ctx, userID, cursor, limit, speciesID)",
		"func (s *Store) listCurrentFishingInventory",
		"FROM player_item_instances i",
		"LEFT JOIN LATERAL",
		"recent.inventory_instance_id = i.instance_id",
		"CASE WHEN c.id IS NULL THEN -i.instance_id ELSE c.id END",
		"i.template_item_id = 'fish_catch'",
		"i.state = 'inventory'",
		"i.archived_at IS NULL",
		"row.InventoryInstanceID = &inventoryInstanceID",
		"out.NextCursor = inventoryInstanceID + 1",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("kept fishing collection must be derived from every current fish inventory instance; missing %q", required)
		}
	}
}

func TestFishingCatchHistoryUsesCurrentInventoryState(t *testing.T) {
	query := fishingCatchListSource(t)
	for _, required := range []string{
		"LEFT JOIN player_item_instances i",
		"WHEN i.state = 'inventory' AND i.archived_at IS NULL THEN c.inventory_instance_id",
		"COALESCE(i.fish_data, '{}'::jsonb)",
		"row.Fish = fishingCatchCurrentFish(row.SpeciesID, row.Fish, inventoryFish)",
		"AND i.state IS DISTINCT FROM $8",
		"AND (NOT $4 OR (i.state = 'inventory' AND i.archived_at IS NULL))",
		"fishingSaleIneligibility(species, profile, inventoryFish)",
		"FishingCatchHistoryDays, fishingPondState",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("fishing catch list must derive visibility from current inventory state; missing %q", required)
		}
	}
	if strings.Contains(query, "AND (NOT $4 OR c.inventory_instance_id IS NOT NULL)") {
		t.Fatal("kept-only filtering must not treat a pond or consumed instance as sellable inventory")
	}
}

func TestFishingCatchCurrentFishUsesAuthoritativeInventoryData(t *testing.T) {
	snapshot := FishInstanceData{SpeciesID: "common_carp", AgeDays: 10}
	inventory := FishInstanceData{SpeciesID: "common_carp", AgeDays: 20, Genes: FishGenes{Mutations: []string{"prismatic"}}}
	if got := fishingCatchCurrentFish("common_carp", snapshot, inventory); !reflect.DeepEqual(got, inventory) {
		t.Fatalf("fishingCatchCurrentFish()=%+v, want inventory data %+v", got, inventory)
	}
	if got := fishingCatchCurrentFish("common_carp", snapshot, FishInstanceData{SpeciesID: "other"}); !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("mismatched inventory data=%+v, want snapshot %+v", got, snapshot)
	}
}

func TestFishingCatchListDefaultsMissingRecordFlags(t *testing.T) {
	query := fishingCatchListSource(t)
	for _, required := range []string{
		"COALESCE(r.best_length_catch_id = c.id, FALSE)",
		"COALESCE(r.best_weight_catch_id = c.id, FALSE)",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("fishing catch record flags must be non-null booleans; missing %q", required)
		}
	}
}
