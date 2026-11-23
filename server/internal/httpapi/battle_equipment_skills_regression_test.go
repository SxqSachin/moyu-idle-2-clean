package httpapi

import (
	"fmt"
	"testing"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func TestEquipmentComboMissingPriorityAnchorFallsBackToFirst(t *testing.T) {
	s, state := equipmentComboFixture()
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{{ID: "hundred_battle_badge", GrantedSkill: &catalog.EquipmentGrantedSkill{SkillID: store.EquipmentMartialComboSkill, BaseCapacity: 2}}}
	unit := &state.Players[0]
	unit.Skills = unit.Skills[1:]
	for i, id := range []string{"strike_a", "strike_b"} {
		unit.Skills[i].ComboOnly = false
		s.deps.GameData.Skills = append(s.deps.GameData.Skills, map[string]any{"id": id, "effects": []any{map[string]any{"kind": "damage", "damageType": "physical"}}})
	}
	s.deps.GameData.Skills = append(s.deps.GameData.Skills, map[string]any{"id": store.EquipmentMartialComboSkill, "cooldown": 6, "effects": []any{}})
	cfg := store.EquipmentSkillConfig{Version: 1, Enabled: true, SkillIDs: []string{"strike_a", "strike_b"}, BeforeSkillID: "removed_skill"}
	s.applyEquipmentSkillSnapshot(unit, []store.EquippedItem{{TemplateItemID: "hundred_battle_badge", EquipType: "battle"}}, []store.PlayerSkill{{SkillID: store.EquipmentMartialComboSkill, EquipmentConfig: &cfg}}, nil)
	got, _ := s.battleSelectSkillFiltered(state, *unit, "player", true, true)
	if got.ID != store.EquipmentMartialComboSkill || unit.Skills[0].ID != got.ID {
		t.Fatalf("combo became unreachable after anchor removal: %s", got.ID)
	}
}

func TestEquipmentDominationChargesEveryInterceptionAndSerializesCosts(t *testing.T) {
	s, state := equipmentComboFixture()
	owner := &state.Players[0]
	owner.DominationSkills = []string{"enemy_spell"}
	battleRuntimeUnitIndex(state)
	for attempt := 0; attempt < 6; attempt++ {
		beforeHP, beforeMP := owner.HP, owner.MP
		result, ok := s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
		if !ok || owner.HP != beforeHP-15 || owner.MP != beforeMP-7 || len(result.Actions) != 3 {
			t.Fatalf("attempt %d: unpaid interruption: hp=%v mp=%v result=%+v", attempt, owner.HP, owner.MP, result)
		}
		for i, resource := range []string{"hp", "mp"} {
			step := battleTimelineStepFromActionRow(result.Actions[i])
			wantCost := []float64{-15, -7}[i]
			if step["resource"] != resource || step["v"] != wantCost || step["i"] != store.EquipmentDominationSkill {
				t.Fatalf("missing resource loss in compact replay: %+v", step)
			}
			target := battleAnyMapSlice(step["t"])[0]
			if i == 0 && (target["b"] != beforeHP || target["h"] != owner.HP) || i == 1 && (target["beforeMp"] != beforeMP || target["m"] != owner.MP) {
				t.Fatalf("incorrect before/after resource snapshot: %+v", target)
			}
		}
		step := battleTimelineStepFromActionRow(result.Actions[2])
		meta := normalizeAnyMap(step["me"])
		if meta["interruptedSkillId"] != "enemy_spell" || meta["interruptedActorId"] != state.Enemies[0].UnitID {
			t.Fatalf("interruption metadata lost: %+v", step)
		}
	}
	result, ok := s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
	if ok || owner.HP != 10 || owner.MP != 8 || len(result.Actions) != 0 {
		t.Fatal("insufficient HP must not charge or interrupt")
	}
}

func TestEquipmentDominationResourceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		hp, mp, maxHP, maxMP float64
		want                 bool
	}{
		{"cannot_spend_last_hp", 15, 50, 100, 50, false},
		{"insufficient_mp", 100, 6, 100, 50, false},
		{"zero_max_mp", 100, 0, 100, 0, false},
		{"minimum_cost", 2, 1, 6, 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, state := equipmentComboFixture()
			owner := &state.Players[0]
			owner.HP, owner.MP, owner.MaxHP, owner.MaxMP = tc.hp, tc.mp, tc.maxHP, tc.maxMP
			owner.DominationSkills = []string{"enemy_spell"}
			battleRuntimeUnitIndex(state)
			_, ok := s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
			if ok != tc.want || !ok && (owner.HP != tc.hp || owner.MP != tc.mp) || ok && (owner.HP != 1 || owner.MP != 0) {
				t.Fatalf("charge must be atomic and nonzero: ok=%v hp=%v mp=%v", ok, owner.HP, owner.MP)
			}
		})
	}
}

func equipmentDominationCamouflageFixture(count int) (*Server, *battleRuntimeState) {
	s, state := equipmentComboFixture()
	s.deps.GameData.BuffEffects = []map[string]any{
		{"id": "camouflage_scope", "buffId": "camouflage", "trigger": "while_active", "effects": []map[string]any{{"kind": "camouflage_targeting_remap"}}},
		{"id": "panic_scope", "buffId": "panic", "trigger": "while_active", "effects": []map[string]any{{"kind": "treat_all_as_enemy"}}},
	}
	state.Players[0].DominationSkills = []string{"enemy_spell"}
	state.Players = append(state.Players, battleUnitState{UnitID: "player:2", Side: "player", Alive: true, HP: 100})
	state.Enemies = make([]battleUnitState, count)
	for i := range state.Enemies {
		state.Enemies[i] = battleUnitState{UnitID: fmt.Sprintf("enemy:%d", i), Side: "enemy", Alive: true, HP: 100, Buffs: []battleBuffState{{BuffID: "camouflage", RemainTurns: 2}}}
	}
	state.Enemies[count-1].Buffs = nil
	battleRuntimeUnitIndex(state)
	return s, state
}

func TestEquipmentDominationIgnoresTargetingRemaps(t *testing.T) {
	s, state := equipmentDominationCamouflageFixture(16)
	if _, ok := s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"}); !ok {
		t.Fatal("camouflage must not bypass interception")
	}
	owner := &state.Players[0]
	owner.Buffs = []battleBuffState{{BuffID: "panic", RemainTurns: 2}}
	beforeHP, beforeMP := owner.HP, owner.MP
	if _, ok := s.interceptBattleSkill(state, &state.Players[1], battleSkillState{ID: "enemy_spell"}); ok || owner.HP != beforeHP || owner.MP != beforeMP {
		t.Fatal("panic must not intercept or charge for an allied cast")
	}
	state.Enemies[0].PerspectiveSide = "player"
	if _, ok := s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"}); ok {
		t.Fatal("current allegiance must be respected")
	}
}

func TestEquipmentDominationUnaffordableMatchDoesNotAllocate(t *testing.T) {
	s, state := equipmentDominationCamouflageFixture(256)
	state.Players[0].MP = 0
	allocations := testing.AllocsPerRun(100, func() {
		s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
	})
	if allocations != 0 {
		t.Fatalf("unaffordable interception must not allocate or run targeting scans: %v", allocations)
	}
}

func BenchmarkEquipmentDominationCamouflage(b *testing.B) {
	for _, count := range []int{16, 256} {
		for _, affordable := range []bool{false, true} {
			b.Run(fmt.Sprintf("enemies_%d/affordable_%t", count, affordable), func(b *testing.B) {
				s, state := equipmentDominationCamouflageFixture(count)
				owner := &state.Players[0]
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					owner.HP, owner.MP = 100, 50
					if !affordable {
						owner.MP = 0
					}
					s.interceptBattleSkill(state, &state.Enemies[0], battleSkillState{ID: "enemy_spell"})
				}
			})
		}
	}
}
