package httpapi

import (
	"testing"
	"time"
)

func TestBattleItemIsReservedBeforeProbabilityFailureExtraTurn(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{{
		"id": "amusement_stun", "durationTurns": 1, "stackPolicy": "refresh_duration",
	}}
	zeroChance := 0.0
	targeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard", "midguard", "rearguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount:     battleSkillTargetCount{Base: 1, Max: 1, MaxSet: true},
	}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID:                 "haunted_house_exit_ticket_first_failure",
		SourceItemID:       "haunted_house_exit_ticket",
		Trigger:            "on_probability_failure",
		MaxTriggersPerWave: 1,
		Effects: []map[string]any{{
			"kind": "extra_turn", "targetMode": "self", "turnMode": "full",
		}},
	}}
	actor.BattleItems = []battleItemState{{
		ID: "battle_item_mirror_house_reflection_candy", ItemID: "mirror_house_reflection_candy",
		Name: "Mirror Candy", Quantity: 2, CooldownTurns: 16, Targeting: targeting,
		Effects: []battleSkillEffect{{
			ID: "mirror_house_reflection", Kind: "status", Targeting: targeting,
			StatusPayload: battleSkillStatusPayload{
				StatusIDs: []string{"amusement_stun"}, DurationBaseTurns: 1,
				ApplyChanceBase: &zeroChance, ApplyChanceMin: &zeroChance, ApplyChanceMax: &zeroChance,
			},
		}},
	}}
	target := probabilityTestUnit("enemy:1")
	target.Side = "enemy"
	target.OriginSide = "enemy"
	target.BaseSide = "enemy"
	target.PerspectiveSide = "enemy"
	state := &battleRuntimeState{Seed: 7, Wave: 17, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}

	s.executeUnitTurn(state, time.UnixMilli(1), &state.Players[0], "player", false)

	item := state.Players[0].BattleItems[0]
	if item.Quantity != 1 || item.CooldownRemain != 16 || item.Resolving {
		t.Fatalf("outer item was not reserved and settled once: %#v", item)
	}
	itemUseEvents := 0
	for _, event := range state.Events {
		if event.Type == "battle.turn.item_use" && event.Data["itemId"] == "mirror_house_reflection_candy" {
			itemUseEvents++
		}
	}
	if itemUseEvents != 1 {
		t.Fatalf("probability extra turn reused the resolving item, events=%#v", state.Events)
	}
	if state.RNGIndex != 0 {
		t.Fatalf("zero-chance status or item reservation consumed RNG: %d", state.RNGIndex)
	}
}
