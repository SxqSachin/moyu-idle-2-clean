package kittyworker

import (
	"math/rand"
	"testing"
	"time"

	"mysecondwebgame/server/internal/config"
)

func TestDueScheduledAffectionSlots(t *testing.T) {
	worker := &Worker{kitty: config.DefaultKittySettings()}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	beforeMorning := worker.dueScheduledAffectionSlots(time.Date(2026, 6, 22, 8, 59, 0, 0, loc))
	if len(beforeMorning) != 0 {
		t.Fatalf("before morning slots=%+v, want none", beforeMorning)
	}

	afterMorning := worker.dueScheduledAffectionSlots(time.Date(2026, 6, 22, 9, 0, 0, 0, loc))
	if len(afterMorning) != 1 || afterMorning[0].DateKey != "2026-06-22" || afterMorning[0].SlotKey != "09:00" {
		t.Fatalf("after morning slots=%+v, want 09:00", afterMorning)
	}

	afterEvening := worker.dueScheduledAffectionSlots(time.Date(2026, 6, 22, 18, 1, 0, 0, loc))
	if len(afterEvening) != 2 || afterEvening[0].SlotKey != "09:00" || afterEvening[1].SlotKey != "18:00" {
		t.Fatalf("after evening slots=%+v, want 09:00 and 18:00", afterEvening)
	}
}

func TestRollCuteEquipmentDropDisabled(t *testing.T) {
	settings := config.DefaultKittySettings()
	settings.JournalRewards.CuteEquipmentDrop.Enabled = false
	worker := &Worker{kitty: settings}
	if itemID, ok := worker.rollCuteEquipmentDrop(); ok || itemID != "" {
		t.Fatalf("rollCuteEquipmentDrop disabled=(%q,%v), want empty false", itemID, ok)
	}
}

func TestRollRareItemDrops(t *testing.T) {
	settings := config.DefaultKittySettings()
	settings.JournalRewards.RareItemDrops = []config.KittyRareItemDropSettings{
		{
			Enabled:    true,
			Chance:     1,
			ItemID:     "harmonic_embryo",
			Quantity:   1,
			ContentKey: "game.catteryJournal.diary.harmonicEmbryoVariants.1",
		},
	}
	worker := &Worker{kitty: settings, rng: rand.New(rand.NewSource(1))}

	rewards, contentKey := worker.rollRareItemDrops()
	if len(rewards) != 1 || rewards[0].ItemID != "harmonic_embryo" || rewards[0].Quantity != 1 {
		t.Fatalf("rewards=%+v, want harmonic_embryo x1", rewards)
	}
	if contentKey != "game.catteryJournal.diary.harmonicEmbryoVariants.1" {
		t.Fatalf("contentKey=%q, want harmonic embryo variant", contentKey)
	}
}

func TestRollEncounterRareItemDropsAreSeparateFromDiaryDrops(t *testing.T) {
	settings := config.DefaultKittySettings()
	settings.JournalRewards.RareItemDrops = []config.KittyRareItemDropSettings{{
		Enabled:    true,
		Chance:     1,
		ItemID:     "harmonic_embryo",
		Quantity:   1,
		ContentKey: "game.catteryJournal.diary.harmonicEmbryoVariants.1",
	}}
	settings.JournalRewards.EncounterRareItemDrops = []config.KittyRareItemDropSettings{{
		Enabled:    true,
		Chance:     1,
		ItemID:     "cat_sword",
		Quantity:   1,
		ContentKey: "game.catteryJournal.encounter.catSwordVariants.1",
	}}
	worker := &Worker{kitty: settings, rng: rand.New(rand.NewSource(1))}

	diaryRewards, _ := worker.rollRareItemDrops()
	if len(diaryRewards) != 1 || diaryRewards[0].ItemID != "harmonic_embryo" {
		t.Fatalf("diary rewards=%+v, want harmonic_embryo only", diaryRewards)
	}
	encounterRewards, contentKey := worker.rollEncounterRareItemDrops()
	if len(encounterRewards) != 1 || encounterRewards[0].ItemID != "cat_sword" || encounterRewards[0].Quantity != 1 {
		t.Fatalf("encounter rewards=%+v, want cat_sword x1", encounterRewards)
	}
	if contentKey != "game.catteryJournal.encounter.catSwordVariants.1" {
		t.Fatalf("contentKey=%q, want cat sword variant", contentKey)
	}
}
