package httpapi

import (
	"testing"
	"time"
)

func TestResolveBattlePersonalityHooks_ApplyBattleStartBuff(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "taunt", "durationTurns": 2},
	}

	profiles := []battlePersonalityProfile{
		{
			ID: "taunting_tail",
			BattleHooks: []map[string]any{
				{
					"trigger": "on_battle_start",
					"effects": []map[string]any{
						{
							"kind":          "add_buff",
							"buffId":        "taunt",
							"durationTurns": 2,
							"targetMode":    "self",
						},
					},
				},
			},
		},
	}

	hooks := s.resolveBattlePersonalityHooks(profiles)
	if len(hooks) != 1 {
		t.Fatalf("hooks len=%d, want 1", len(hooks))
	}
	if hooks[0].SourceItemID != "personality:taunting_tail" {
		t.Fatalf("unexpected hook source item id: %q", hooks[0].SourceItemID)
	}
	if hooks[0].ID != "personality:taunting_tail:battle_hook:1" {
		t.Fatalf("unexpected hook id: %q", hooks[0].ID)
	}

	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:         "player:1",
				Side:           "player",
				Position:       "vanguard",
				Alive:          true,
				HP:             100,
				MaxHP:          100,
				Stats:          map[string]float64{"hp": 100},
				EquipmentHooks: hooks,
			},
		},
	}

	actions := s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	if len(actions) == 0 {
		t.Fatalf("expected battle start actions")
	}
	if len(state.Players[0].Buffs) != 1 || state.Players[0].Buffs[0].BuffID != "taunt" {
		t.Fatalf("battle start personality hook should apply taunt, buffs=%#v", state.Players[0].Buffs)
	}
}

func TestResolveBattlePersonalityEffectMutations_SweepingClawsAddsTargets(t *testing.T) {
	s := &Server{}

	profiles := []battlePersonalityProfile{
		{
			ID: "sweeping_claws",
			EffectMutations: []map[string]any{
				{
					"scope": "basic_attack",
					"effectSelector": map[string]any{
						"mode":    "index",
						"indexes": []any{1},
					},
					"ops": map[string]any{
						"targetCountDelta": 2,
					},
					"constraints": map[string]any{
						"targetCountMin": 1,
					},
				},
			},
		},
	}

	mutations := s.resolveBattlePersonalityEffectMutations(profiles)
	if len(mutations) != 1 {
		t.Fatalf("mutations len=%d, want 1", len(mutations))
	}

	actor := &battleUnitState{
		UnitID:          "player:1",
		EffectMutations: mutations,
	}
	effects := []battleSkillEffect{
		{
			Kind: "damage",
			Targeting: battleSkillTargeting{
				TargetPositions: []string{"vanguard"},
				SecondarySeek:   "front_to_rear",
				TargetScope:     "enemy",
				SelectRule:      "first",
				TargetCount: battleSkillTargetCount{
					Base: 1,
				},
			},
			AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
			DamageSpec: battleDamageSpec{
				DamageType:    "physical",
				AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
				HitCount:      battleSkillTargetCount{Base: 1},
			},
		},
	}

	out := s.applyBattleAffixEffectMutations(actor, "basic_attack", "basic_attack", 1, effects[0].Targeting, effects)
	if len(out) != 1 {
		t.Fatalf("effects len=%d, want 1", len(out))
	}
	if out[0].Targeting.TargetCount.Base != 3 {
		t.Fatalf("basic attack target count=%d, want 3", out[0].Targeting.TargetCount.Base)
	}
}

func TestKittyTauntingTailPrioritizesEnemySingleTargetAttack(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "taunt", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "taunt_prefer_target",
			"buffId":  "taunt",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "prefer_as_enemy_target"}},
		},
	}

	profiles := []battlePersonalityProfile{
		{
			ID: "taunting_tail",
			BattleHooks: []map[string]any{
				{
					"trigger": "on_battle_start",
					"effects": []map[string]any{
						{
							"kind":          "add_buff",
							"buffId":        "taunt",
							"durationTurns": 2,
							"targetMode":    "self",
						},
					},
				},
			},
		},
	}

	basicSkill := s.battleSkillByID(s.battleDefaultBasicSkillID())
	state := &battleRuntimeState{
		Seed:  2026,
		Wave:  1,
		Order: []string{"enemy:basic", "player:plain", "kitty:77"},
		Players: []battleUnitState{
			{
				UnitID:   "player:plain",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":    100,
					"mp":    0,
					"speed": 1,
				},
			},
			{
				UnitID:                "kitty:77",
				UnitType:              "kitty",
				Side:                  "player",
				Position:              "rearguard",
				Alive:                 true,
				HP:                    100,
				MaxHP:                 100,
				Stats:                 map[string]float64{"hp": 100, "mp": 0, "speed": 1},
				KittyPersonalityHooks: s.resolveBattlePersonalityHooks(profiles),
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:basic",
				Side:     "enemy",
				EnemyID:  "enemy_basic",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":             100,
					"mp":             0,
					"speed":          10,
					"physics_damage": 40,
				},
				SkillIDs: []string{basicSkill.ID},
				Skills:   []battleSkillState{basicSkill},
			},
		},
	}

	s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	actor := &state.Enemies[0]
	s.executeBattleSkill(state, time.Unix(0, 0), actor, actor.Skills[0], "active", nil)

	if len(state.Players[1].Buffs) == 0 || state.Players[1].Buffs[0].BuffID != "taunt" {
		t.Fatalf("kitty should gain taunt from taunting_tail, buffs=%#v", state.Players[1].Buffs)
	}
	if state.Players[1].HP >= 100 {
		t.Fatalf("kitty taunt target should be prioritized, kittyHP=%v playerHP=%v", state.Players[1].HP, state.Players[0].HP)
	}
	if state.Players[0].HP < 100 {
		t.Fatalf("plain player should not be hit while kitty taunt exists, hp=%v", state.Players[0].HP)
	}
}
