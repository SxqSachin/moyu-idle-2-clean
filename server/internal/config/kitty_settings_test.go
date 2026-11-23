package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultKittySettingsJournalPrivateRewards(t *testing.T) {
	settings := DefaultKittySettings()
	scheduled := settings.JournalRewards.ScheduledAffection
	if !scheduled.Enabled || scheduled.Timezone != "Asia/Shanghai" || scheduled.RewardItemID != "kitty_affection" || scheduled.Quantity != 2 {
		t.Fatalf("unexpected scheduled affection defaults: %+v", scheduled)
	}
	if len(scheduled.Times) != 2 || scheduled.Times[0] != "09:00" || scheduled.Times[1] != "18:00" {
		t.Fatalf("unexpected scheduled affection times: %+v", scheduled.Times)
	}
	drop := settings.JournalRewards.CuteEquipmentDrop
	if !drop.Enabled || drop.Chance != 0.005 || len(drop.ItemIDs) != 5 {
		t.Fatalf("unexpected cute equipment drop defaults: %+v", drop)
	}
	if len(settings.JournalRewards.RareItemDrops) != 0 {
		t.Fatalf("unexpected rare item drop defaults: %+v", settings.JournalRewards.RareItemDrops)
	}
	if len(settings.JournalRewards.EncounterRareItemDrops) != 0 {
		t.Fatalf("unexpected encounter rare item drop defaults: %+v", settings.JournalRewards.EncounterRareItemDrops)
	}
	if err := ValidateKittySettings(settings); err != nil {
		t.Fatalf("ValidateKittySettings failed: %v", err)
	}
}

func TestValidateKittySettingsPrivateRewardItems(t *testing.T) {
	settings := DefaultKittySettings()
	itemIDs := map[string]bool{
		"wood":            true,
		"gold_coin":       true,
		"sturdy_wood":     true,
		"silverleaf_wood": true,
		"kitty_affection": true,
		"cute_hat":        true,
		"cute_clothes":    true,
		"cute_gloves":     true,
		"cute_shoes":      true,
		"cute_cape":       true,
		"harmonic_embryo": true,
		"cat_sword":       true,
	}
	settings.JournalRewards.RareItemDrops = []KittyRareItemDropSettings{{
		Enabled:    true,
		Chance:     0.0005,
		ItemID:     "harmonic_embryo",
		Quantity:   1,
		ContentKey: "game.catteryJournal.diary.harmonicEmbryoVariants.1",
	}}
	settings.JournalRewards.EncounterRareItemDrops = []KittyRareItemDropSettings{{
		Enabled:    true,
		Chance:     0.0005,
		ItemID:     "cat_sword",
		Quantity:   1,
		ContentKey: "game.catteryJournal.encounter.catSwordVariants.1",
	}}
	if err := ValidateKittySettingsItemIDs(settings, itemIDs); err != nil {
		t.Fatalf("ValidateKittySettingsItemIDs failed: %v", err)
	}
	delete(itemIDs, "cute_cape")
	if err := ValidateKittySettingsItemIDs(settings, itemIDs); err == nil {
		t.Fatalf("expected missing cute_cape validation error")
	}
	itemIDs["cute_cape"] = true
	delete(itemIDs, "cat_sword")
	if err := ValidateKittySettingsItemIDs(settings, itemIDs); err == nil {
		t.Fatalf("expected missing cat_sword validation error")
	}
}

func TestValidateKittySettingsRejectsInvalidPrivateRewards(t *testing.T) {
	settings := DefaultKittySettings()
	settings.JournalRewards.ScheduledAffection.Times = []string{"24:00"}
	if err := ValidateKittySettings(settings); err == nil {
		t.Fatalf("expected invalid scheduled affection time")
	}

	settings = DefaultKittySettings()
	settings.JournalRewards.CuteEquipmentDrop.Chance = 1.2
	if err := ValidateKittySettings(settings); err == nil {
		t.Fatalf("expected invalid cute equipment chance")
	}

	settings = DefaultKittySettings()
	settings.JournalRewards.RareItemDrops = []KittyRareItemDropSettings{{
		Enabled:  true,
		Chance:   1.2,
		ItemID:   "harmonic_embryo",
		Quantity: 1,
	}}
	if err := ValidateKittySettings(settings); err == nil {
		t.Fatalf("expected invalid rare item drop chance")
	}

	settings = DefaultKittySettings()
	settings.JournalRewards.EncounterRareItemDrops = []KittyRareItemDropSettings{{
		Enabled:  true,
		Chance:   -0.1,
		ItemID:   "cat_sword",
		Quantity: 1,
	}}
	if err := ValidateKittySettings(settings); err == nil {
		t.Fatalf("expected invalid encounter rare item drop chance")
	}
}

func TestKittySourceConfigRoutesCatSwordToEncounters(t *testing.T) {
	settings, err := LoadKittySettings(filepath.Join("..", "..", "..", "packages", "game-config", "source", "kitty-config.json"))
	if err != nil {
		t.Fatalf("LoadKittySettings failed: %v", err)
	}
	hasHarmonicEmbryoDiaryDrop := false
	for _, drop := range settings.JournalRewards.RareItemDrops {
		if drop.Enabled && drop.ItemID == "cat_sword" {
			t.Fatalf("cat_sword must not be a diary rare drop")
		}
		if drop.Enabled && drop.ItemID == "harmonic_embryo" {
			hasHarmonicEmbryoDiaryDrop = true
		}
	}
	if !hasHarmonicEmbryoDiaryDrop {
		t.Fatalf("harmonic_embryo must remain a diary rare drop")
	}
	var catSwordDrop *KittyRareItemDropSettings
	for i := range settings.JournalRewards.EncounterRareItemDrops {
		drop := &settings.JournalRewards.EncounterRareItemDrops[i]
		if drop.ItemID == "cat_sword" {
			catSwordDrop = drop
			break
		}
	}
	if catSwordDrop == nil || !catSwordDrop.Enabled || catSwordDrop.Quantity != 1 || catSwordDrop.Chance != 0.0005 {
		t.Fatalf("encounter cat_sword drop=%+v, want enabled 0.05%% x1", catSwordDrop)
	}
}
