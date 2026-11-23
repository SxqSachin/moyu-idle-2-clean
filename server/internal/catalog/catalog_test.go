package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadItemBaseAttrsAcceptsDecimal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	content := `{
		"version": 1,
		"items": [
			{
				"id": "test_item",
				"type": ["equipment"],
				"rarity": "common",
				"isEquipment": false,
				"baseAttrs": {
					"physics_defence": 10,
					"physics_damage_resist": 0.0216
				},
				"stackable": true,
				"carryableInCombat": false,
				"marketListable": true
			}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write catalog json: %v", err)
	}

	data, err := Load(path)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(data.Items))
	}
	got := data.Items[0].BaseAttrs["physics_damage_resist"]
	if got != 0.0216 {
		t.Fatalf("unexpected decimal attr value: got %v want %v", got, 0.0216)
	}
}

func TestLoadRejectsMythicAlchemyUpgradeOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	content := `{
		"version": 1,
		"items": [
			{"id": "legendary_source", "type": ["material"], "rarity": "legendary"},
			{"id": "mythic_reward", "type": ["material"], "rarity": "mythic"}
		],
		"alchemy_groups": [{
			"id": "group.test",
			"entries": [{"itemId": "legendary_source", "weight": 1}],
			"upgradeEntries": [{"itemId": "mythic_reward", "weight": 1}]
		}]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write catalog json: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "cannot produce mythic items") {
		t.Fatalf("Load() error=%v, want mythic conversion upgrade rejection", err)
	}
}

func TestLoadRejectsFishingPondWithoutNurseryCapacity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	content := `{
		"version": 1,
		"buildings": [{
			"id": "fish_pond",
			"maxLevel": 1,
			"levels": [{"level": 1, "bonuses": {"fishCapacity": 12}}]
		}]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write catalog json: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "positive nurseryCapacity") {
		t.Fatalf("Load() error=%v, want missing nursery capacity rejection", err)
	}
}

func TestLoadAcceptsFishingPondWithNurseryCapacity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	content := `{
		"version": 1,
		"buildings": [{
			"id": "fish_pond",
			"maxLevel": 1,
			"levels": [{"level": 1, "bonuses": {"fishCapacity": 12, "nurseryCapacity": 12}}]
		}]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write catalog json: %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() rejected valid pond capacities: %v", err)
	}
}

func TestAlchemyTransmuteRuleForItemIDUsesPriorityAndDefault(t *testing.T) {
	t.Parallel()

	data := Data{AlchemyTransmuteRules: []AlchemyTransmuteRule{
		{ID: "transmute.default", Default: true, Priority: 0},
		{ID: "transmute.lower", ItemIDs: []string{"boss_drop"}, Priority: 10},
		{ID: "transmute.higher", ItemIDs: []string{"boss_drop"}, Priority: 20},
	}}
	if got := data.AlchemyTransmuteRuleForItemID("boss_drop"); got == nil || got.ID != "transmute.higher" {
		t.Fatalf("specific rule=%v, want transmute.higher", got)
	}
	if got := data.AlchemyTransmuteRuleForItemID("ordinary_drop"); got == nil || got.ID != "transmute.default" {
		t.Fatalf("fallback rule=%v, want transmute.default", got)
	}
}

func TestDefaultAlchemyTransmuteRulePreservesLegacyFirstActiveRule(t *testing.T) {
	t.Parallel()

	data := Data{AlchemyTransmuteRules: []AlchemyTransmuteRule{
		{ID: "disabled", Disabled: true},
		{ID: "legacy.first"},
		{ID: "legacy.second"},
	}}
	if got := data.DefaultAlchemyTransmuteRule(); got == nil || got.ID != "legacy.first" {
		t.Fatalf("legacy default=%v, want legacy.first", got)
	}
}

func TestItemAlchemyNameAffixIgnoresWeightOnlyMetadata(t *testing.T) {
	t.Parallel()

	data := Data{Items: []Item{{
		ID: "weighted_skillbook",
		Alchemy: &ItemAlchemyMeta{
			ConvertWeight: 17,
		},
	}}}
	if got := data.ItemAlchemyNameAffix("weighted_skillbook"); got != nil {
		t.Fatalf("weight-only item name affix=%+v, want nil", got)
	}
}
