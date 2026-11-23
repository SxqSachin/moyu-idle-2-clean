package httpapi

import (
	"math"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
)

func testCombatUnit(id, side string, hp float64, stats map[string]float64) battleUnitState {
	baseStats := make(map[string]float64, len(stats)+1)
	for key, value := range stats {
		baseStats[key] = value
	}
	baseStats["hp"] = hp
	unitStats := make(map[string]float64, len(baseStats))
	for key, value := range baseStats {
		unitStats[key] = value
	}
	return battleUnitState{
		UnitID:     id,
		Side:       side,
		OriginSide: side,
		BaseSide:   side,
		Position:   "midguard",
		Alive:      true,
		HP:         hp,
		MaxHP:      hp,
		Stats:      unitStats,
		BaseStats:  baseStats,
	}
}

func holyCatRevengeStarHook(enhanceLevel int) battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID:                 "holy_cat_revenge_star_after_heal",
		SourceItemID:       "holy_cat_revenge_star_staff",
		SourceEnhanceLevel: enhanceLevel,
		Trigger:            "on_after_deal_heal",
		Conditions: []map[string]any{{
			"type":  "incoming_heal_gte",
			"value": 1,
		}},
		Effects: []map[string]any{{
			"kind":       "damage",
			"targetMode": "targeting",
			"targeting": map[string]any{
				"targetPositions": []string{"vanguard", "midguard", "rearguard"},
				"secondarySeek":   "none",
				"targetScope":     "enemy",
				"selectRule":      "random",
				"targetCount":     map[string]any{"base": 1, "max": 1},
			},
			"damageType": "magical",
			"amountFormula": map[string]any{
				"baseAdd": 50,
				"terms": []map[string]any{
					{"attrKey": "intelligence", "coefficient": 0.55, "perEnhanceLevel": 0.0125},
					{"attrKey": "equipment_enhance_level", "coefficient": 4.0},
				},
			},
			"dispatchEquipmentHooks": false,
		}},
	}
}

func TestHolyCatRevengeStarHitsOneRandomLivingEnemyAfterHeal(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			testCombatUnit("player:healer", "player", 1000, map[string]float64{"intelligence": 100}),
			testCombatUnit("player:ally", "player", 1000, nil),
		},
		Enemies: []battleUnitState{
			testCombatUnit("enemy:last", "enemy", 1000, map[string]float64{"magical_defence": 0}),
			testCombatUnit("enemy:other", "enemy", 1000, map[string]float64{"magical_defence": 0}),
		},
	}
	healer := &state.Players[0]
	ally := &state.Players[1]
	ally.HP = 900
	firstEnemy := &state.Enemies[0]
	secondEnemy := &state.Enemies[1]
	healer.EquipmentHooks = []battleEquipmentHookRule{holyCatRevengeStarHook(20)}

	healRuntime := newBattleSkillActionRuntime(state, healer, time.UnixMilli(1), s, nil)
	healRuntime.ApplyHeal(battleskills.HealAction{
		SourceID: healer.UnitID,
		TargetID: ally.UnitID,
		Amount:   50,
		Reason:   "test_revenge_heal",
	})
	// 50 + (0.55 + 0.0125*20)*100 + 4*20 = 210 raw magical damage.
	if totalEnemyHP := firstEnemy.HP + secondEnemy.HP; totalEnemyHP != 1790 {
		t.Fatalf("total enemy HP=%.2f, want 1790", totalEnemyHP)
	}
	if (firstEnemy.HP == 790) == (secondEnemy.HP == 790) {
		t.Fatalf("exactly one random enemy should be hit, HPs=(%.2f, %.2f)", firstEnemy.HP, secondEnemy.HP)
	}
	if state.RNGIndex != 1 {
		t.Fatalf("random targeting should consume one authoritative RNG value, index=%d", state.RNGIndex)
	}
}

func prayerAshCleanseHook(enhanceLevel int, guaranteed bool) battleEquipmentHookRule {
	chance := 0.35
	maxChance := 0.75
	coefficient := 0.01
	if guaranteed {
		chance = 1
		maxChance = 1
		coefficient = 0
	}
	return battleEquipmentHookRule{
		ID:                 "prayer_ash_venom_cleanse",
		SourceItemID:       "prayer_ash_venom_censer",
		SourceEnhanceLevel: enhanceLevel,
		Trigger:            "on_after_deal_heal",
		Effects: []map[string]any{{
			"kind":            "remove_buff_by_kind",
			"buffKind":        "debuff",
			"targetMode":      "source",
			"dispellableOnly": true,
			"selectionMode":   "max_remaining",
			"maxBuffs":        1,
			"procChanceBase":  chance,
			"procChanceMax":   maxChance,
			"procChanceFormula": map[string]any{
				"terms": []map[string]any{{"attrKey": "equipment_enhance_level", "coefficient": coefficient}},
			},
		}},
	}
}

func TestPrayerAshVenomCleanseChanceAndLongestDispellableDebuff(t *testing.T) {
	effect := prayerAshCleanseHook(0, false).Effects[0]
	for _, tc := range []struct {
		level int
		want  float64
	}{{0, 0.35}, {20, 0.55}, {40, 0.75}, {80, 0.75}} {
		chance, configured := battleResolveEquipmentHookProcChance(&battleUnitState{}, prayerAshCleanseHook(tc.level, false), effect)
		if !configured || chance != tc.want {
			t.Fatalf("level %d chance=%.3f configured=%v, want %.3f", tc.level, chance, configured, tc.want)
		}
	}

	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "short_debuff", "kind": "debuff", "dispellable": true},
		{"id": "long_debuff", "kind": "debuff", "dispellable": true},
		{"id": "sealed_debuff", "kind": "debuff", "dispellable": false},
		{"id": "long_buff", "kind": "buff", "dispellable": true},
	}
	state := &battleRuntimeState{Players: []battleUnitState{
		testCombatUnit("player:healer", "player", 1000, nil),
		testCombatUnit("player:ally", "player", 1000, nil),
	}}
	healer := &state.Players[0]
	ally := &state.Players[1]
	healer.EquipmentHooks = []battleEquipmentHookRule{prayerAshCleanseHook(0, true)}
	ally.Buffs = []battleBuffState{
		{BuffID: "short_debuff", RemainTurns: 1},
		{BuffID: "long_debuff", RemainTurns: 4},
		{BuffID: "sealed_debuff", RemainTurns: 9},
		{BuffID: "long_buff", RemainTurns: 10},
	}
	actions := s.battleTriggerUnitEquipmentHooks(state, time.UnixMilli(0), healer, ally, "on_after_deal_heal", 50, "")
	if findBuffByID(ally, "long_debuff") != nil {
		t.Fatalf("longest dispellable debuff should be removed, buffs=%#v", ally.Buffs)
	}
	if findBuffByID(ally, "short_debuff") == nil || findBuffByID(ally, "sealed_debuff") == nil || findBuffByID(ally, "long_buff") == nil {
		t.Fatalf("cleanse removed the wrong state, buffs=%#v actions=%#v", ally.Buffs, actions)
	}
}

func forbiddenSealVerdictHook(enhanceLevel int) battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID:                 "cat_ancestor_forbidden_seal_verdict",
		SourceItemID:       "cat_ancestor_forbidden_seal_crown",
		SourceEnhanceLevel: enhanceLevel,
		Trigger:            battleEquipmentTriggerAfterApplyBuff,
		EventFilter: map[string]any{
			"buffKinds":             []string{"debuff"},
			"firstBuffInstanceOnly": true,
			"applicationKinds":      []string{"attach"},
			"excludeSelf":           true,
		},
		Conditions: []map[string]any{{
			"type":     "target_distinct_source_buff_count_gte",
			"buffKind": "debuff",
			"value":    3,
		}},
		Effects: []map[string]any{{
			"kind":       "damage",
			"targetMode": "source",
			"damageType": "magical",
			"amountFormula": map[string]any{
				"baseAdd": 220,
				"terms": []map[string]any{
					{"attrKey": "magical_damage", "coefficient": 1.4, "perEnhanceLevel": 0.035},
					{"attrKey": "equipment_enhance_level", "coefficient": 10.0},
				},
			},
			"dispatchEquipmentHooks": false,
		}},
	}
}

func TestForbiddenSealVerdictRequiresThreeDistinctWearerDebuffs(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "debuff_a", "kind": "debuff"},
		{"id": "debuff_b", "kind": "debuff"},
		{"id": "debuff_c", "kind": "debuff", "stackPolicy": "stack", "maxStacks": 3},
		{"id": "debuff_d", "kind": "debuff"},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{testCombatUnit("player:wearer", "player", 1000, map[string]float64{"magical_damage": 100})},
		Enemies: []battleUnitState{testCombatUnit("enemy:target", "enemy", 3000, map[string]float64{"magical_defence": 0})},
	}
	wearer := &state.Players[0]
	target := &state.Enemies[0]
	wearer.EquipmentHooks = []battleEquipmentHookRule{forbiddenSealVerdictHook(20)}

	applyTestBuff(t, s, state, wearer.UnitID, target.UnitID, "debuff_a")
	applyTestBuff(t, s, state, wearer.UnitID, target.UnitID, "debuff_b")
	if target.HP != 3000 {
		t.Fatalf("verdict must wait for three distinct debuffs, HP=%.2f", target.HP)
	}
	applyTestBuff(t, s, state, wearer.UnitID, target.UnitID, "debuff_c")
	// 220 + (1.4 + 0.035*20)*100 + 10*20 = 630 raw magical damage.
	if target.HP != 2370 {
		t.Fatalf("verdict HP=%.2f, want 2370", target.HP)
	}
	applyTestBuff(t, s, state, wearer.UnitID, target.UnitID, "debuff_c")
	applyTestBuff(t, s, state, wearer.UnitID, target.UnitID, "debuff_d")
	if math.Abs(target.HP-1740) > 1e-9 {
		t.Fatalf("a fourth distinct debuff should retrigger while a same-debuff stack should not, HP=%.2f", target.HP)
	}
}

func volatileSporeHooks(enhanceLevel int) []battleEquipmentHookRule {
	return []battleEquipmentHookRule{
		{
			ID:                 "volatile_spore_fuse_apply",
			SourceItemID:       "volatile_spore_fuse_ring",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            battleEquipmentTriggerAfterApplyBuff,
			Priority:           10,
			EventFilter: map[string]any{
				"buffKinds":        []string{"debuff"},
				"excludeBuffIds":   []string{"volatile_spore_charge"},
				"applicationKinds": []string{"attach", "refresh"},
				"excludeSelf":      true,
			},
			Effects: []map[string]any{{
				"kind":          "add_buff",
				"buffId":        "volatile_spore_charge",
				"durationTurns": 4,
				"targetMode":    "source",
			}},
		},
		{
			ID:                 "volatile_spore_fuse_detonate",
			SourceItemID:       "volatile_spore_fuse_ring",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            battleEquipmentTriggerAfterApplyBuff,
			EventFilter: map[string]any{
				"buffIds":          []string{"volatile_spore_charge"},
				"applicationKinds": []string{"attach"},
				"excludeSelf":      true,
			},
			Conditions: []map[string]any{{
				"type":   "target_buff_count_gte",
				"buffId": "volatile_spore_charge",
				"value":  3,
			}},
			Effects: []map[string]any{
				{
					"kind":       "damage",
					"targetMode": "source",
					"damageType": "magical",
					"amountFormula": map[string]any{
						"baseAdd": 80,
						"terms": []map[string]any{
							{"attrKey": "perception", "coefficient": 0.55, "perEnhanceLevel": 0.018},
							{"attrKey": "equipment_enhance_level", "coefficient": 5.0},
						},
					},
					"dispatchEquipmentHooks": false,
				},
				{"kind": "remove_buff", "buffId": "volatile_spore_charge", "targetMode": "source"},
			},
		},
	}
}

func TestVolatileSporeFuseStacksAndDetonatesOnlyOnMarkedTarget(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "debuff_a", "kind": "debuff"},
		{"id": "debuff_b", "kind": "debuff"},
		{"id": "debuff_c", "kind": "debuff"},
		{"id": "volatile_spore_charge", "kind": "debuff", "durationTurns": 4, "stackPolicy": "stack", "maxStacks": 3, "dispellable": false},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{testCombatUnit("player:wearer", "player", 1000, map[string]float64{"perception": 100})},
		Enemies: []battleUnitState{
			testCombatUnit("enemy:marked", "enemy", 2000, map[string]float64{"magical_defence": 0}),
			testCombatUnit("enemy:other", "enemy", 2000, map[string]float64{"magical_defence": 0}),
		},
	}
	wearer := &state.Players[0]
	marked := &state.Enemies[0]
	other := &state.Enemies[1]
	wearer.EquipmentHooks = volatileSporeHooks(20)

	applyTestBuff(t, s, state, wearer.UnitID, marked.UnitID, "debuff_a")
	applyTestBuff(t, s, state, wearer.UnitID, marked.UnitID, "debuff_b")
	if stacks := battleCountBuff(marked, "volatile_spore_charge"); stacks != 2 {
		t.Fatalf("spore stacks=%d after two debuffs, want 2", stacks)
	}
	if marked.HP != 2000 {
		t.Fatalf("spore must not explode before three stacks, HP=%.2f", marked.HP)
	}
	applyTestBuff(t, s, state, wearer.UnitID, marked.UnitID, "debuff_c")
	// 80 + (0.55 + 0.018*20)*100 + 5*20 = 271 raw magical damage.
	if marked.HP != 1729 {
		t.Fatalf("marked target HP=%.2f, want 1729", marked.HP)
	}
	if stacks := battleCountBuff(marked, "volatile_spore_charge"); stacks != 0 {
		t.Fatalf("detonation should clear all spore stacks, got %d", stacks)
	}
	if other.HP != 2000 {
		t.Fatalf("spore explosion must not splash, other HP=%.2f", other.HP)
	}
}
