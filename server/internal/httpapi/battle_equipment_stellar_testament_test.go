package httpapi

import (
	"fmt"
	"testing"
	"time"
)

const (
	stellarTestamentStarTemplateID = "stellar_testament_star"
	stellarTestamentStarSkillID    = "stellar_testament_ray"
)

func newStellarTestamentTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "stellar_testament_command", "durationTurns": 999, "permanent": true},
		{"id": "stellar_testament_star_orbit", "durationTurns": 999, "permanent": true},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "stellar_testament_command_wa",
			"buffId":  "stellar_testament_command",
			"trigger": "while_active",
			"effects": []map[string]any{{
				"kind":                     "trigger_synced_summon_turn",
				"includeSummonTemplateIds": []string{stellarTestamentStarTemplateID},
			}},
		},
		{
			"id":      "stellar_testament_star_orbit_wa",
			"buffId":  "stellar_testament_star_orbit",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": "skip_turn_order"},
				{"kind": "untargetable_by_enemy"},
				{"kind": "suppress_mirror_cast_trigger"},
			},
		},
	}
	s.deps.GameData.Skills = []map[string]any{
		{
			"id":       stellarTestamentStarSkillID,
			"category": "arcane",
			"mpCost":   0,
			"cooldown": 0,
			"effects": []map[string]any{{
				"kind": "damage",
				"targeting": map[string]any{
					"targetPositions": []any{"vanguard", "midguard", "rearguard"},
					"secondarySeek":   "random_alive",
					"targetScope":     "enemy",
					"selectRule":      "random",
					"targetCount":     map[string]any{"base": 1, "max": 1},
				},
				"damageType": "magical",
				"amountFormula": map[string]any{
					"baseAdd": 59,
					"terms": []any{
						map[string]any{"attrKey": "magical_damage", "coefficient": 1, "perSkillLevel": 0},
					},
				},
			}},
		},
	}
	s.deps.GameData.Enemies = []map[string]any{{
		"id":             stellarTestamentStarTemplateID,
		"name":           "Testament Star",
		"position":       "rearguard",
		"type":           "normal",
		"level":          1,
		"skillIds":       []any{stellarTestamentStarSkillID},
		"hp":             1,
		"mp":             0,
		"magical_damage": 1,
		"perception":     1,
		"speed":          1,
		"crit_chance":    0,
	}}
	return s
}

func stellarTestamentHooks(enhanceLevel int) []battleEquipmentHookRule {
	return []battleEquipmentHookRule{
		{
			ID:                 "stellar_testament_unit_death",
			SourceItemID:       "stellar_testament_pendant",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            "on_unit_death",
			SubjectFilter: map[string]any{
				"excludeSummonTemplateIds": []string{stellarTestamentStarTemplateID},
			},
			Conditions: []map[string]any{},
			Effects: []map[string]any{{
				"kind":       "summon",
				"targetMode": "self",
				"summonPayload": map[string]any{
					"unitId":          stellarTestamentStarTemplateID,
					"side":            "source",
					"countFormula":    map[string]any{"baseAdd": 1, "terms": []any{}},
					"durationTurns":   999,
					"maxCountPerUnit": 6,
					"maxCountFormula": map[string]any{
						"baseAdd": 6,
						"terms": []any{
							map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 0.1, "perSkillLevel": 0},
						},
					},
					"persistAcrossBattles": false,
					"onCasterDeath":        "despawn_immediately",
					"onCapOverflow":        "replace_oldest",
					"statsFormula": map[string]any{
						"magical_damage": map[string]any{
							"baseAdd": 0,
							"terms": []any{
								map[string]any{"attrKey": "perception", "coefficient": 0.2, "perSkillLevel": 0},
								map[string]any{"attrKey": "magical_damage", "coefficient": 0.1, "perSkillLevel": 0},
								map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 5, "perSkillLevel": 0},
							},
						},
					},
				},
			}},
		},
		{
			ID:                 "stellar_testament_bind_star",
			SourceItemID:       "stellar_testament_pendant",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            "on_after_summon",
			SubjectFilter: map[string]any{
				"includeUnitTypes":         []string{"summon"},
				"includeSummonTemplateIds": []string{stellarTestamentStarTemplateID},
			},
			Conditions: []map[string]any{},
			Effects: []map[string]any{{
				"kind":          "add_buff",
				"buffId":        "stellar_testament_star_orbit",
				"durationTurns": 999,
				"targetMode":    "source",
			}},
		},
	}
}

func TestStellarTestamentMaxCountScalesEveryTenEnhancementLevels(t *testing.T) {
	payload := parseBattleSkillSummonPayload(map[string]any{
		"maxCountPerUnit": 6,
		"maxCountFormula": map[string]any{
			"baseAdd": 6,
			"terms": []any{
				map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 0.1, "perSkillLevel": 0},
			},
		},
	})
	owner := stellarTestamentOwner(0)
	for _, tc := range []struct {
		enhanceLevel int
		want         int
	}{
		{enhanceLevel: 0, want: 6},
		{enhanceLevel: 9, want: 6},
		{enhanceLevel: 10, want: 7},
		{enhanceLevel: 19, want: 7},
		{enhanceLevel: 20, want: 8},
		{enhanceLevel: 30, want: 9},
		{enhanceLevel: 40, want: 10},
	} {
		if got := battleResolveSummonMaxCount(&owner, 1, payload, tc.enhanceLevel); got != tc.want {
			t.Fatalf("enhancement +%d max stars = %d, want %d", tc.enhanceLevel, got, tc.want)
		}
	}
	legacyPayload := parseBattleSkillSummonPayload(map[string]any{"maxCountPerUnit": 6})
	if got := battleResolveSummonMaxCount(&owner, 1, legacyPayload, 40); got != 6 {
		t.Fatalf("legacy fixed summon cap = %d, want 6", got)
	}
}

func TestStellarTestamentEquipmentHookUsesEnhancementScaledCap(t *testing.T) {
	s := newStellarTestamentTestServer()
	state := &battleRuntimeState{
		Players: []battleUnitState{stellarTestamentOwner(10)},
	}
	for index := 0; index < 8; index++ {
		state.Enemies = append(state.Enemies, battleUnitState{
			UnitID:   fmt.Sprintf("enemy:fallen:%d", index),
			Side:     "enemy",
			Alive:    false,
			HP:       0,
			MaxHP:    100,
			Position: "vanguard",
			Stats:    map[string]float64{"hp": 100},
			BaseStats: map[string]float64{
				"hp": 100,
			},
		})
		fallen := &state.Enemies[len(state.Enemies)-1]
		battleQueueUnitDeath(state, fallen, &state.Players[0])
		s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(int64(index), 0))
	}
	if got := battleCountSummonsByTemplateAndSource(state, stellarTestamentStarTemplateID, "player:owner"); got != 7 {
		t.Fatalf("+10 pendant should retain seven stars after cap replacement, got %d", got)
	}
}

func stellarTestamentOwner(enhanceLevel int) battleUnitState {
	return battleUnitState{
		UnitID:          "player:owner",
		Side:            "player",
		OriginSide:      "player",
		BaseSide:        "player",
		PerspectiveSide: "player",
		UnitType:        "player",
		Position:        "rearguard",
		BasePosition:    "rearguard",
		Alive:           true,
		HP:              1000,
		MaxHP:           1000,
		MP:              100,
		MaxMP:           100,
		Stats: map[string]float64{
			"hp": 1000, "mp": 100, "perception": 100, "magical_damage": 200, "speed": 10,
		},
		BaseStats: map[string]float64{
			"hp": 1000, "mp": 100, "perception": 100, "magical_damage": 200, "speed": 10,
		},
		EquipmentHooks: stellarTestamentHooks(enhanceLevel),
	}
}

func TestStellarTestamentDeathCreatesBoundStarWithEnhanceScaling(t *testing.T) {
	s := newStellarTestamentTestServer()
	state := &battleRuntimeState{
		Players: []battleUnitState{stellarTestamentOwner(20)},
		Enemies: []battleUnitState{{
			UnitID: "enemy:fallen", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy",
			Alive: false, HP: 0, MaxHP: 500, Position: "vanguard", BasePosition: "vanguard",
			Stats: map[string]float64{"hp": 500}, BaseStats: map[string]float64{"hp": 500},
		}},
	}

	battleQueueUnitDeath(state, &state.Enemies[0], &state.Players[0])
	actions := s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(0, 0))
	if len(actions) == 0 || len(state.Players) != 2 {
		t.Fatalf("death should create one star: actions=%#v players=%d", actions, len(state.Players))
	}
	star := &state.Players[1]
	if star.SummonTemplateUnitID != stellarTestamentStarTemplateID || star.SummonSourceUnitID != "player:owner" {
		t.Fatalf("summon binding mismatch: %#v", star)
	}
	if !star.SkipTurnOrder || !battleHasBuff(star, "stellar_testament_star_orbit") {
		t.Fatalf("star must be bound outside normal order: %#v", star)
	}
	// 1 base + 20% of 100 Perception + 10% of 200 Magical Damage + 5*20 enhancement.
	if got, want := star.Stats["magical_damage"], 141.0; got != want {
		t.Fatalf("star inherited damage = %.2f, want %.2f", got, want)
	}
	state.reseedOrder()
	for _, unitID := range state.Order {
		if unitID == star.UnitID {
			t.Fatalf("star must not enter ordered turns: %#v", state.Order)
		}
	}
}

func TestStellarTestamentStarDeathIsExcludedAndVolleyDefersNewStar(t *testing.T) {
	s := newStellarTestamentTestServer()
	owner := stellarTestamentOwner(20)
	owner.Buffs = []battleBuffState{{BuffID: "stellar_testament_command", RemainTurns: 999}}
	state := &battleRuntimeState{
		Players: []battleUnitState{owner},
		Enemies: []battleUnitState{
			{
				UnitID: "enemy:target:1", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy",
				Alive: true, HP: 100, MaxHP: 100, Position: "vanguard", BasePosition: "vanguard",
				Stats: map[string]float64{"hp": 100, "magical_defence": 0}, BaseStats: map[string]float64{"hp": 100, "magical_defence": 0},
			},
			{
				UnitID: "enemy:target:2", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy",
				Alive: true, HP: 100, MaxHP: 100, Position: "vanguard", BasePosition: "vanguard",
				Stats: map[string]float64{"hp": 100, "magical_defence": 0}, BaseStats: map[string]float64{"hp": 100, "magical_defence": 0},
			},
		},
	}
	s.battleRecomputeUnitStats(&state.Players[0])
	state.Enemies[0].Alive = false
	state.Enemies[0].HP = 0
	battleQueueUnitDeath(state, &state.Enemies[0], &state.Players[0])
	s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(0, 0))
	state.Enemies[0].Alive = true
	state.Enemies[0].HP = 100
	state.Players = append(state.Players, battleUnitState{
		UnitID: "player:other-summon", Side: "player", OriginSide: "player", BaseSide: "player", PerspectiveSide: "player",
		Alive: true, HP: 100, MaxHP: 100, MP: 20, MaxMP: 20, SkipTurnOrder: true,
		SummonSourceUnitID: "player:owner", SummonTemplateUnitID: "other_summon",
		Stats: map[string]float64{"hp": 100, "mp": 20, "speed": 99}, BaseStats: map[string]float64{"hp": 100, "mp": 20, "speed": 99},
		Skills: []battleSkillState{{ID: "basic_attack", MPCost: 10}}, SkillIDs: []string{"basic_attack"}, SkillSlots: []string{"basic_attack"},
	})

	s.executeSyncedSummonTurns(state, time.Unix(1, 0), &state.Players[0], "player", 1)
	aliveEnemies := 0
	for index := range state.Enemies {
		if state.Enemies[index].Alive && state.Enemies[index].HP > 0 {
			aliveEnemies++
		}
	}
	if aliveEnemies != 1 {
		t.Fatalf("one starting star must fire once; a star created by that kill waits for the next owner action: %#v", state.Enemies)
	}
	if got := battleCountSummonsByTemplateAndSource(state, stellarTestamentStarTemplateID, "player:owner"); got != 2 {
		t.Fatalf("star kill should create one deferred star, got %d", got)
	}
	if got := battleUnitByID(state, "player:other-summon").MP; got != 20 {
		t.Fatalf("pendant sync filter must not grant unrelated summons an action, mp=%v", got)
	}
	newest := battleFindOldestSummonByTemplateAndSource(state, stellarTestamentStarTemplateID, "player:owner")
	if newest == nil {
		t.Fatal("expected stars to remain addressable")
	}
	newest.Alive = false
	newest.HP = 0
	battleQueueUnitDeath(state, newest, &state.Enemies[0])
	s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(2, 0))
	if got := battleCountSummonsByTemplateAndSource(state, stellarTestamentStarTemplateID, "player:owner"); got != 1 {
		t.Fatalf("a defeated Testament Star must not recursively replace itself, alive stars=%d", got)
	}
}
