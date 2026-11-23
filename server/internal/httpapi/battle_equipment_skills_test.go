package httpapi

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func equipmentComboFixture() (*Server, *battleRuntimeState) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack", "effects": []any{}}}
	child := func(id string) battleSkillState {
		return battleSkillState{ID: id, Level: 1, ComboOnly: true, MPCost: 5, CooldownTurns: 3,
			Effects: []battleSkillEffect{{Kind: "damage", DamageType: "physical", Targeting: battleSkillTargeting{TargetScope: "enemy", TargetPositions: []string{"vanguard"}, SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}, AmountFormula: battleSkillAmountFormula{BaseAdd: 10}}}}
	}
	state := &battleRuntimeState{Seed: 7, Wave: 1,
		Players: []battleUnitState{{UnitID: "player:1", UnitType: "player", UserID: 1, Side: "player", Alive: true, HP: 100, MaxHP: 100, MP: 50, MaxMP: 50, Position: "vanguard", Stats: map[string]float64{"hp": 100, "mp": 50}, Skills: []battleSkillState{
			{ID: store.EquipmentMartialComboSkill, CooldownTurns: 6, Combo: &battleComboPlan{Count: 2, Indexes: [4]int{1, 2}, SkillIDs: [4]string{"strike_a", "strike_b"}}}, child("strike_a"), child("strike_b"), {ID: "basic_attack"},
		}}},
		Enemies: []battleUnitState{{UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true, HP: 1000, MaxHP: 1000, Stats: map[string]float64{"hp": 1000}}},
	}
	return s, state
}

func TestEquipmentComboOneTurnSharedCooldownAndNoStore(t *testing.T) {
	s, state := equipmentComboFixture()
	actor := &state.Players[0]
	result := s.executeUnitTurn(state, time.Unix(0, 0), actor, "player", false)
	if result["skillId"] != store.EquipmentMartialComboSkill || actor.MP != 40 {
		t.Fatalf("cast/cost: %+v mp=%v", result, actor.MP)
	}
	for i, want := range []int{6, 3, 3, 0} {
		if actor.Skills[i].CooldownRemain != want {
			t.Fatalf("cooldown[%d]=%d want %d", i, actor.Skills[i].CooldownRemain, want)
		}
	}
	if got := state.AchievementStats.SkillCastsByUnit[actor.UnitID]; got["strike_a"] != 1 || got["strike_b"] != 1 || got[store.EquipmentMartialComboSkill] != 0 {
		t.Fatalf("skill rewards: %+v", got)
	}
	if state.nextBuffDurationTurnSerial != 1 {
		t.Fatalf("components advanced turns: %d", state.nextBuffDurationTurnSerial)
	}
	casts := []string{}
	for _, row := range battleAnyMapSlice(result["actionLog"]) {
		if row["kind"] == "cast" {
			casts = append(casts, row["skillId"].(string))
		}
	}
	if !reflect.DeepEqual(casts, []string{"strike_a", "strike_b"}) {
		t.Fatalf("order: %+v", casts)
	}
	node := buildBattleTimelineNodeData(battleEvent{}, result, nil, nil)
	steps := battleAnyMapSlice(node["p"])
	seen := map[string]bool{}
	for _, step := range steps {
		if step["k"] == "c" {
			seen[anyToString(step["i"])] = true
		}
	}
	if !seen["strike_a"] || !seen["strike_b"] {
		t.Fatalf("child cast names lost in timeline: %+v", steps)
	}
}

func TestEquipmentComboReadinessDoesNotSkipCooldownOrSpend(t *testing.T) {
	for _, reason := range []string{"cooldown", "mp", "hp", "invalid_index"} {
		t.Run(reason, func(t *testing.T) {
			s, state := equipmentComboFixture()
			actor := &state.Players[0]
			switch reason {
			case "cooldown":
				actor.Skills[2].CooldownRemain = 2
			case "mp":
				actor.MP = 9
			case "hp":
				actor.Skills[2].HPCost = 100
			case "invalid_index":
				actor.Skills[0].Combo.Indexes[1] = 99
			}
			hp, mp := actor.HP, actor.MP
			skill, _ := s.battleSelectSkillFiltered(state, *actor, "player", true, true)
			if skill.ID != "basic_attack" || actor.HP != hp || actor.MP != mp {
				t.Fatalf("must fall through without consuming: %s", skill.ID)
			}
		})
	}
}

func TestEquipmentDominationOnlyFirstAffordableEnemyPays(t *testing.T) {
	s, state := equipmentComboFixture()
	state.Players[0].DominationSkills = []string{"enemy_spell"}
	second := state.Players[0]
	second.UnitID = "player:2"
	second.Skills = nil
	state.Players = append(state.Players, second)
	battleRuntimeUnitIndex(state)
	result, ok := s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
	if !ok || result.Meta["interrupted"] != true || state.Players[0].HP != 85 || state.Players[0].MP != 43 || state.Players[1].HP != 100 {
		t.Fatalf("interception: %+v %+v", state.Players, result)
	}
	state.Players[0].HP = 15
	_, ok = s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
	if !ok || state.Players[0].HP != 15 || state.Players[1].HP != 85 {
		t.Fatal("affordability fallback failed")
	}
	before := state.Players[1].HP
	if _, ok = s.interceptBattleSkill(state, &state.Players[0], battleSkillState{ID: "enemy_spell"}); ok || state.Players[1].HP != before {
		t.Fatal("intercepted an ally")
	}
	if _, ok = s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell", Uninterruptible: true}); ok {
		t.Fatal("intercepted protected mechanic")
	}
}

func TestEquipmentComboInterceptedChildDoesNotCancelRemaining(t *testing.T) {
	s, state := equipmentComboFixture()
	state.Enemies[0].DominationSkills = []string{"strike_a"}
	state.Enemies[0].MP = 100
	state.Enemies[0].MaxMP = 100
	s.executeUnitTurn(state, time.Unix(0, 0), &state.Players[0], "player", false)
	if state.Players[0].MP != 40 || state.Players[0].Skills[1].CooldownRemain != 3 {
		t.Fatal("intercepted child must pay costs/cooldown")
	}
	if counts := state.AchievementStats.SkillCastsByUnit["player:1"]; counts["strike_a"] != 0 || counts["strike_b"] != 1 {
		t.Fatalf("incorrect child completion %+v", counts)
	}
}

func TestEquipmentSkillSnapshotRoundTrip(t *testing.T) {
	s, state := equipmentComboFixture()
	state.Players[0].DominationSkills = []string{"spell"}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBattleRuntimeState(m)
	if !reflect.DeepEqual(decoded.Players[0].Skills[0].Combo, state.Players[0].Skills[0].Combo) {
		t.Fatal("lost compiled combo")
	}
	carry := encodeBattleCarryUnitState(state.Players[0])
	raw, _ = json.Marshal(carry)
	var restored battleUnitState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.DominationSkills, []string{"spell"}) {
		t.Fatal("lost domination carry")
	}
	s.executeUnitTurn(&decoded, time.Unix(0, 0), &decoded.Players[0], "player", false)
	if decoded.Players[0].MP != 40 {
		t.Fatal("restored combo failed")
	}
	if state.Players[0].MP != 50 {
		t.Fatal("snapshot alias")
	}
}

func BenchmarkEquipmentSkillReadiness(b *testing.B) {
	b.Run("ordinary_selection", func(b *testing.B) {
		s, state := equipmentComboFixture()
		actor := state.Players[0]
		actor.Skills = actor.Skills[1:]
		actor.Skills[0].ComboOnly = false
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s.battleSelectSkillFiltered(state, actor, "player", true, true)
		}
	})
	b.Run("combo_selection", func(b *testing.B) {
		s, state := equipmentComboFixture()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s.battleSelectSkillFiltered(state, state.Players[0], "player", true, true)
		}
	})
	b.Run("domination_nonmatch", func(b *testing.B) {
		s, state := equipmentComboFixture()
		state.Players[0].DominationSkills = []string{"other"}
		state.unitIndexByID = nil
		battleRuntimeUnitIndex(state)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "unmatched"})
		}
	})
}

func TestEquipmentSkillSnapshotCapacityAndUnequip(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		enhance                       int
		equipped, standalone, enabled bool
		want                          bool
	}{
		{"valid", 10, true, false, true, true}, {"standalone", 10, true, true, true, true},
		{"downgraded", 0, true, false, true, false}, {"unequipped", 10, false, false, true, false}, {"disabled", 10, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, state := equipmentComboFixture()
			s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{{ID: "hundred_battle_badge", GrantedSkill: &catalog.EquipmentGrantedSkill{SkillID: store.EquipmentMartialComboSkill, BaseCapacity: 2, CapacityMilestones: []int{10, 20}}}}
			ids := []string{"strike_a", "strike_b", "strike_c"}
			unit := &state.Players[0]
			unit.Skills = unit.Skills[1:3]
			unit.Skills = append(unit.Skills, battleSkillState{ID: "strike_c"})
			for i, id := range ids {
				unit.Skills[i].ComboOnly = false
				s.deps.GameData.Skills = append(s.deps.GameData.Skills, map[string]any{"id": id, "effects": []any{map[string]any{"kind": "damage", "damageType": "physical"}}})
			}
			s.deps.GameData.Skills = append(s.deps.GameData.Skills, map[string]any{"id": store.EquipmentMartialComboSkill, "cooldown": 6, "effects": []any{}})
			cfg := store.EquipmentSkillConfig{Version: 1, Enabled: tc.enabled, AllowStandalone: tc.standalone, SkillIDs: ids, BeforeSkillID: "strike_b"}
			var gear []store.EquippedItem
			if tc.equipped {
				gear = []store.EquippedItem{{TemplateItemID: "hundred_battle_badge", EquipType: "battle", EnhanceLevel: tc.enhance}}
			}
			s.applyEquipmentSkillSnapshot(unit, gear, []store.PlayerSkill{{SkillID: store.EquipmentMartialComboSkill, EquipmentConfig: &cfg}}, nil)
			if len(unit.Skills) == 4 != tc.want {
				t.Fatalf("compiled unexpected config: %+v", unit.Skills)
			}
			if !tc.want {
				if unit.Skills[0].ComboOnly {
					t.Fatal("invalid combo disabled ordinary skills")
				}
				return
			}
			plan := unit.Skills[1].Combo
			for n, id := range ids {
				child, _, ok := battleComboChild(unit, plan, n)
				if !ok || child.ID != id || child.ComboOnly == tc.standalone {
					t.Fatal("compiled indices/standalone", n, child)
				}
			}
		})
	}
}
