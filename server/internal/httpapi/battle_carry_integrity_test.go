package httpapi

import (
	"fmt"
	"testing"

	"mysecondwebgame/server/internal/store"
)

func carryIntegrityTestUnit(unitID string) battleUnitState {
	return battleUnitState{
		UnitID:      unitID,
		Side:        "player",
		Position:    "vanguard",
		Alive:       true,
		HP:          10,
		MaxHP:       10,
		Stats:       map[string]float64{"hp": 10},
		SkillRules:  map[string]battleSkillRuleState{},
		ItemRules:   map[string]battleSkillRuleState{},
		SkillSlots:  []string{},
		SkillIDs:    []string{},
		Skills:      []battleSkillState{},
		BattleItems: []battleItemState{},
	}
}

func TestBattleRepairPreviousWavePlayersDeduplicatesAndBoundsPersistentSummons(t *testing.T) {
	kittyID := int64(9)
	detail := store.BattleRoomDetail{
		Units: []store.BattleRoomUnit{
			{OwnerUserID: 1, UnitType: "player"},
			{OwnerUserID: 1, UnitType: "kitty", UnitID: kittyID, KittyID: &kittyID},
		},
	}
	player := carryIntegrityTestUnit("player:1")
	player.UserID = 1
	player.EquipmentHooks = []battleEquipmentHookRule{{
		ID:      "old_shadow_every_third_wave",
		Trigger: "on_wave_start",
		Effects: []map[string]any{{
			"kind":                 "player_clone",
			"maxCountPerUnit":      3,
			"persistAcrossBattles": true,
			"onCapOverflow":        "replace_oldest",
		}},
	}}
	kitty := carryIntegrityTestUnit("kitty:9")
	kitty.IsPlayerUnit = true
	kitty.UnitType = "kitty"
	kitty.UserID = 1
	kitty.KittyID = kittyID

	players := []battleUnitState{player, kitty, player}
	for order := 1; order <= 5; order++ {
		summon := carryIntegrityTestUnit(fmt.Sprintf("summon_old_shadow_%d", order))
		summon.SummonTemplateUnitID = battlePlayerCloneTemplateID
		summon.SummonSourceUnitID = "player:1"
		summon.SummonMaxCountPerUnit = 99
		summon.SummonPersistAcrossBattles = true
		summon.SummonOrder = int64(order)
		if order == 1 {
			summon.SummonTemplateUnitID = ""
			summon.SummonSourceUnitID = ""
			summon.SummonMaxCountPerUnit = 0
			summon.SummonPersistAcrossBattles = false
			summon.SummonOrder = 0
			summon.Buffs = []battleBuffState{{
				BuffID:      battleSummonBindingBuffID,
				RemainTurns: 3,
				SourceID:    "player:1",
				Payload: map[string]any{
					"templateUnitId":       battlePlayerCloneTemplateID,
					"sourceUnitId":         "player:1",
					"maxCountPerUnit":      99,
					"persistAcrossBattles": true,
					"summonOrder":          1,
				},
			}}
		}
		players = append(players, summon)
	}
	unknown := carryIntegrityTestUnit("player:999")
	invalidSummon := carryIntegrityTestUnit("summon_broken")
	nonPersistent := carryIntegrityTestUnit("summon_temporary")
	nonPersistent.SummonTemplateUnitID = "temporary"
	nonPersistent.SummonSourceUnitID = "player:1"
	players = append(players, unknown, invalidSummon, nonPersistent)

	repaired, report, err := battleRepairPreviousWavePlayers(detail, players, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired) != 5 {
		t.Fatalf("repaired unit count=%d, want 2 formal + 3 old shadows: %#v", len(repaired), repaired)
	}
	if report.DroppedDuplicateUnitIDs != 1 || report.DroppedUnknownFormalUnits != 1 ||
		report.DroppedInvalidSummons != 1 || report.DroppedNonPersistent != 1 ||
		report.DroppedExcessSummons != 2 || report.RestoredFormalIdentity != 1 ||
		report.RestoredSummonMetadata != 1 {
		t.Fatalf("unexpected carry repair report: %#v", report)
	}

	byID := map[string]battleUnitState{}
	for _, unit := range repaired {
		byID[unit.UnitID] = unit
	}
	for _, droppedID := range []string{"summon_old_shadow_1", "summon_old_shadow_2", "player:999", "summon_broken", "summon_temporary"} {
		if _, exists := byID[droppedID]; exists {
			t.Fatalf("corrupt or excess unit %s survived carry repair", droppedID)
		}
	}
	for _, keptID := range []string{"summon_old_shadow_3", "summon_old_shadow_4", "summon_old_shadow_5"} {
		unit, exists := byID[keptID]
		if !exists {
			t.Fatalf("newest bounded summon %s was dropped", keptID)
		}
		if unit.SummonMaxCountPerUnit != 3 || unit.IsPlayerUnit || unit.UnitType != "" || unit.RewardEligible {
			t.Fatalf("summon identity/cap was not normalized: %#v", unit)
		}
	}
	formal := byID["player:1"]
	if !formal.IsPlayerUnit || formal.UnitType != "player" || formal.UserID != 1 {
		t.Fatalf("formal player identity was not restored: %#v", formal)
	}
	order := battleFilterPreparationOrder([]string{"summon_old_shadow_5", "player:1", "kitty:9", "player:1"}, repaired)
	if len(order) != 2 || order[0] != "player:1" || order[1] != "kitty:9" {
		t.Fatalf("preparation order retained summons or duplicates: %#v", order)
	}
}

func TestBattleRepairPreviousWavePlayersEnforcesFormalRosterCap(t *testing.T) {
	detail := store.BattleRoomDetail{Units: []store.BattleRoomUnit{
		{OwnerUserID: 1, UnitType: "player"},
		{OwnerUserID: 2, UnitType: "player"},
		{OwnerUserID: 3, UnitType: "player"},
	}}
	players := []battleUnitState{
		carryIntegrityTestUnit("player:1"),
		carryIntegrityTestUnit("player:2"),
		carryIntegrityTestUnit("player:3"),
	}
	repaired, report, err := battleRepairPreviousWavePlayers(detail, players, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired) != 2 || report.DroppedFormalUnitsOverCap != 1 {
		t.Fatalf("formal roster cap was not enforced: units=%#v report=%#v", repaired, report)
	}
}

func TestBattleCarryUnitEncodingPreservesIdentityAndSummonCap(t *testing.T) {
	encoded := encodeBattleCarryUnitState(battleUnitState{
		UnitID:                     "player:1",
		IsPlayerUnit:               true,
		UnitType:                   "player",
		SummonMaxCountPerUnit:      3,
		SummonPersistAcrossBattles: true,
	})
	if !encoded.IsPlayerUnit || encoded.UnitType != "player" || encoded.SummonMaxCountPerUnit != 3 {
		t.Fatalf("carry encoding lost identity or summon cap: %#v", encoded)
	}
}
