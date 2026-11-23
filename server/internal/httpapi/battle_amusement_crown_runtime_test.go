package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func amusementCrownHooks(enhanceLevel int) []battleEquipmentHookRule {
	return []battleEquipmentHookRule{
		{
			ID:                 "king_of_amusement_park_opening_parade",
			SourceItemID:       "king_of_amusement_park",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            "on_battle_start",
			Effects: []map[string]any{{
				"kind":       "summon",
				"targetMode": "self",
				"summonPayload": map[string]any{
					"unitId": "candy_clown_001",
					"side":   "player",
					"countFormula": map[string]any{
						"baseAdd": 1,
						"terms":   []any{map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 0.05}},
					},
					"durationFormula": map[string]any{
						"baseAdd": 2,
						"terms":   []any{map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 0.05}},
					},
					"maxCountPerUnit": 1,
					"maxCountFormula": map[string]any{
						"baseAdd": 1,
						"terms":   []any{map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 0.05}},
					},
					"persistAcrossBattles": false,
					"onCasterDeath":        "keep_until_duration_end",
					"onCapOverflow":        "replace_oldest",
					"onDispel":             "despawn_immediately",
					"onDurationEnd":        "despawn_immediately",
				},
			}},
		},
		{
			ID:                     "king_of_amusement_park_curtain_call",
			SourceItemID:           "king_of_amusement_park",
			SourceEnhanceLevel:     enhanceLevel,
			Trigger:                "on_unit_death",
			MaxTriggersPerBattle:   2,
			ActivationCost:         battleHookActivationCost{MaxHPRatio: 0.1, MaxMPRatio: 0.15},
			CooldownCompletedTurns: 2,
			SubjectFilter: map[string]any{
				"includeUnitTypes":         []any{"summon"},
				"includeSummonTemplateIds": []any{"candy_clown_001"},
			},
			Conditions: []map[string]any{{"type": "equipment_enhance_level_gte", "value": 40}},
			Effects:    []map[string]any{{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"}},
		},
	}
}

func TestAmusementCrownOpeningParadeRunsEveryWave(t *testing.T) {
	s := newAmusementCrownTestServer()
	owner := amusementCrownOwner(40)
	owner.UserID = 1
	owner.Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{owner}}

	if actions := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1)); len(actions) == 0 {
		t.Fatal("wave one opening parade did not trigger")
	}
	if got := battleCountSummonsByTemplateAndSource(state, "candy_clown_001", owner.UnitID); got != 3 {
		t.Fatalf("wave one clown count = %d, want 3", got)
	}

	carriedPlayers := make([]battleUnitState, 0, len(state.Players))
	for index := range state.Players {
		if !battleUnitIsSummon(&state.Players[index]) {
			carriedPlayers = append(carriedPlayers, state.Players[index])
		}
	}
	state.Players = carriedPlayers
	state.Wave = 2

	if actions := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(2)); len(actions) == 0 {
		t.Fatal("wave two opening parade did not trigger")
	}
	if got := battleCountSummonsByTemplateAndSource(state, "candy_clown_001", owner.UnitID); got != 3 {
		t.Fatalf("wave two clown count = %d, want 3", got)
	}
}

func newAmusementCrownTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{
		"id": "basic_attack", "category": "basic", "mpCost": 0, "cooldown": 0,
		"effects": []map[string]any{{
			"id": "basic_attack_damage", "kind": "damage", "damageType": "physical",
			"targeting": map[string]any{
				"targetPositions": []any{"vanguard", "midguard", "rearguard"},
				"secondarySeek":   "random_alive", "targetScope": "enemy", "selectRule": "first",
				"targetCount": map[string]any{"base": 1, "max": 1},
			},
			"amountFormula": map[string]any{"baseAdd": 10, "terms": []any{}},
		}},
	}}
	s.deps.GameData.Enemies = []map[string]any{{
		"id": "candy_clown_001", "name": "Candy Clown", "position": "midguard", "type": "normal", "level": 1,
		"skillIds": []any{"basic_attack"}, "hp": 100, "mp": 0, "physics_damage": 10, "speed": 10,
	}}
	return s
}

func amusementCrownOwner(enhanceLevel int) battleUnitState {
	return battleUnitState{
		UnitID: "player:crown", Side: "player", OriginSide: "player", BaseSide: "player", PerspectiveSide: "player",
		UnitType: "player", Position: "midguard", BasePosition: "midguard", Alive: true,
		HP: 500, MaxHP: 500, MP: 100, MaxMP: 100,
		Stats:          map[string]float64{"hp": 500, "mp": 100, "physics_damage": 50, "speed": 20},
		BaseStats:      map[string]float64{"hp": 500, "mp": 100, "physics_damage": 50, "speed": 20},
		SkillIDs:       []string{"basic_attack"},
		Skills:         []battleSkillState{{ID: "basic_attack", Category: "basic"}},
		EquipmentHooks: amusementCrownHooks(enhanceLevel),
	}
}

func TestAmusementCrownOpeningParadeAndCurtainCallSettle(t *testing.T) {
	s := newAmusementCrownTestServer()
	state := &battleRuntimeState{
		Wave:    1,
		Players: []battleUnitState{amusementCrownOwner(40)},
		Enemies: []battleUnitState{{
			UnitID: "enemy:target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1000, MaxHP: 1000,
			Stats: map[string]float64{"hp": 1000}, BaseStats: map[string]float64{"hp": 1000},
		}},
	}
	state.Players[0].UserID = 1
	state.Players[0].Skills = []battleSkillState{s.battleSkillByID("basic_attack")}

	startActions := s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	if got := battleCountSummonsByTemplateAndSource(state, "candy_clown_001", "player:crown"); got != 3 {
		t.Fatalf("+40 crown should open with three clowns, got %d actions=%#v", got, startActions)
	}
	for index := range state.Players {
		unit := &state.Players[index]
		if unit.SummonTemplateUnitID == "candy_clown_001" && unit.SummonRemainTurns != 4 {
			t.Fatalf("+40 parade duration=%d, want 4", unit.SummonRemainTurns)
		}
	}
	if state.RNGIndex != 0 {
		t.Fatalf("deterministic opening parade consumed RNG index=%d", state.RNGIndex)
	}

	var fallen *battleUnitState
	for index := range state.Players {
		if state.Players[index].SummonTemplateUnitID == "candy_clown_001" {
			fallen = &state.Players[index]
			break
		}
	}
	if fallen == nil {
		t.Fatal("expected an addressable parade clown")
	}
	fallen.Alive = false
	fallen.HP = 0
	battleQueueUnitDeath(state, fallen, &state.Enemies[0])
	deathActions := s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(1, 0))
	if got := countActionKindRecursive(deathActions, "extra_turn"); got != 1 {
		t.Fatalf("defeated parade clown should grant one curtain-call turn, got %d actions=%#v", got, deathActions)
	}
	if state.Enemies[0].HP != 1000 || len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("curtain-call grant executed inline instead of queueing: enemyHp=%v queue=%#v", state.Enemies[0].HP, state.ExtraTurnQueue)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(2, 0), nil); err != nil {
		t.Fatalf("process curtain-call formal extra turn: %v", err)
	}
	if state.Enemies[0].HP >= 1000 {
		t.Fatalf("curtain-call formal turn did not execute an attack, enemy hp=%v", state.Enemies[0].HP)
	}
	if state.RNGIndex != 1 {
		t.Fatalf("curtain-call turn RNG index=%d, want deterministic index 1", state.RNGIndex)
	}
	if got := state.Players[0].HP; got != 450 {
		t.Fatalf("curtain-call hp=%v, want 450 after paying 10%% max HP", got)
	}
	if got := state.Players[0].MP; got != 85 {
		t.Fatalf("curtain-call mp=%v, want 85 after paying 15%% max MP", got)
	}
}

func TestAmusementCrownSummonCountUsesEnhanceMilestones(t *testing.T) {
	tests := []struct {
		level    int
		count    int
		duration int
	}{
		{level: 0, count: 1, duration: 2},
		{level: 19, count: 1, duration: 2},
		{level: 20, count: 2, duration: 3},
		{level: 39, count: 2, duration: 3},
		{level: 40, count: 3, duration: 4},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("enhance_%d", tt.level), func(t *testing.T) {
			s := newAmusementCrownTestServer()
			state := &battleRuntimeState{Players: []battleUnitState{amusementCrownOwner(tt.level)}}
			actions := s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
			if got := battleCountSummonsByTemplateAndSource(state, "candy_clown_001", "player:crown"); got != tt.count {
				t.Fatalf("+%d summon count=%d, want %d actions=%#v", tt.level, got, tt.count, actions)
			}
			for index := range state.Players {
				unit := &state.Players[index]
				if unit.SummonTemplateUnitID == "candy_clown_001" && unit.SummonRemainTurns != int64(tt.duration) {
					t.Fatalf("+%d summon duration=%d, want %d", tt.level, unit.SummonRemainTurns, tt.duration)
				}
			}
		})
	}
}

func TestAmusementCrownCurtainCallRequiresMaxEnhanceAndResources(t *testing.T) {
	newState := func(level int, hp, mp float64) (*Server, *battleRuntimeState) {
		s := newAmusementCrownTestServer()
		owner := amusementCrownOwner(level)
		owner.HP = hp
		owner.MP = mp
		owner.Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
		state := &battleRuntimeState{
			Players: []battleUnitState{owner},
			Enemies: []battleUnitState{{
				UnitID: "enemy:target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
				Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1000, MaxHP: 1000,
				Stats: map[string]float64{"hp": 1000}, BaseStats: map[string]float64{"hp": 1000},
			}},
		}
		return s, state
	}
	queueClownDeath := func(state *battleRuntimeState, id string) {
		state.Players = append(state.Players, battleUnitState{
			UnitID: id, Side: "player", OriginSide: "player", BaseSide: "player", PerspectiveSide: "player",
			UnitType: "summon", SummonTemplateUnitID: "candy_clown_001", SummonSourceUnitID: "player:crown",
			Position: "midguard", BasePosition: "midguard", Alive: false, HP: 0, MaxHP: 100,
		})
		battleQueueUnitDeath(state, &state.Players[len(state.Players)-1], &state.Enemies[0])
	}

	t.Run("below max enhancement", func(t *testing.T) {
		s, state := newState(39, 500, 100)
		queueClownDeath(state, "summon:not-max")
		actions := s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(0, 0))
		if got := countActionKindRecursive(actions, "extra_turn"); got != 0 {
			t.Fatalf("+39 crown granted %d curtain-call turns", got)
		}
		if state.Players[0].HP != 500 || state.Players[0].MP != 100 {
			t.Fatalf("blocked +39 curtain call paid resources: hp=%v mp=%v", state.Players[0].HP, state.Players[0].MP)
		}
	})

	for _, tt := range []struct {
		name string
		hp   float64
		mp   float64
	}{
		{name: "hp must remain positive", hp: 50, mp: 100},
		{name: "mp must be sufficient", hp: 500, mp: 14},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, state := newState(40, tt.hp, tt.mp)
			queueClownDeath(state, "summon:insufficient")
			actions := s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(0, 0))
			if got := countActionKindRecursive(actions, "extra_turn"); got != 0 {
				t.Fatalf("insufficient resources granted %d curtain-call turns", got)
			}
			if state.Players[0].HP != tt.hp || state.Players[0].MP != tt.mp {
				t.Fatalf("blocked curtain call paid resources: hp=%v mp=%v", state.Players[0].HP, state.Players[0].MP)
			}
			state.Players[0].HP = 500
			state.Players[0].MP = 100
			queueClownDeath(state, "summon:funded")
			actions = s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(1, 0))
			if got := countActionKindRecursive(actions, "extra_turn"); got != 1 {
				t.Fatalf("funded retry should still trigger once, got %d actions=%#v", got, actions)
			}
		})
	}
}

func TestAmusementCrownCurtainCallUsesCompletedTurnCooldown(t *testing.T) {
	s := newAmusementCrownTestServer()
	state := &battleRuntimeState{
		Players: []battleUnitState{amusementCrownOwner(40)},
		Enemies: []battleUnitState{{
			UnitID: "enemy:target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1000, MaxHP: 1000,
			Stats: map[string]float64{"hp": 1000}, BaseStats: map[string]float64{"hp": 1000},
		}},
	}
	state.Players[0].Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
	s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	clowns := make([]*battleUnitState, 0, 3)
	for index := range state.Players {
		if state.Players[index].SummonTemplateUnitID == "candy_clown_001" {
			clowns = append(clowns, &state.Players[index])
		}
	}
	if len(clowns) != 3 {
		t.Fatalf("expected three +40 curtain-call actors, got %d", len(clowns))
	}

	killAndFlush := func(index int, now int64) []map[string]any {
		clowns[index].Alive = false
		clowns[index].HP = 0
		battleQueueUnitDeath(state, clowns[index], &state.Enemies[0])
		return s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(now, 0))
	}
	if got := countActionKindRecursive(killAndFlush(0, 1), "extra_turn"); got != 1 {
		t.Fatalf("first defeated clown granted %d turns, want 1", got)
	}
	if got := countActionKindRecursive(killAndFlush(1, 2), "extra_turn"); got != 0 {
		t.Fatalf("cooldown allowed immediate second curtain call: %d", got)
	}
	if state.Players[0].HP != 450 || state.Players[0].MP != 85 {
		t.Fatalf("blocked second curtain call changed resources: hp=%v mp=%v", state.Players[0].HP, state.Players[0].MP)
	}
	if state.CompletedTurnCounts == nil {
		state.CompletedTurnCounts = map[string]int{}
	}
	state.CompletedTurnCounts[state.Players[0].UnitID] = 2
	if got := countActionKindRecursive(killAndFlush(2, 3), "extra_turn"); got != 1 {
		t.Fatalf("curtain call did not reopen after two completed turns: %d", got)
	}
	if state.Players[0].HP != 400 || state.Players[0].MP != 70 {
		t.Fatalf("second funded curtain call resource totals: hp=%v mp=%v", state.Players[0].HP, state.Players[0].MP)
	}
}

func TestAmusementCrownCurtainCallIgnoresUnrelatedDeaths(t *testing.T) {
	s := newAmusementCrownTestServer()
	state := &battleRuntimeState{
		Players: []battleUnitState{amusementCrownOwner(40)},
		Enemies: []battleUnitState{{
			UnitID: "enemy:ordinary", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", Alive: false, HP: 0, MaxHP: 100,
			Position: "vanguard", BasePosition: "vanguard", Stats: map[string]float64{"hp": 100}, BaseStats: map[string]float64{"hp": 100},
		}},
	}
	battleQueueUnitDeath(state, &state.Enemies[0], &state.Players[0])
	actions := s.battleFlushPendingUnitDeathEquipmentHooks(state, time.Unix(0, 0))
	if got := countActionKindRecursive(actions, "extra_turn"); got != 0 {
		t.Fatalf("ordinary death must not trigger crown encore, got %d actions=%#v", got, actions)
	}
}
