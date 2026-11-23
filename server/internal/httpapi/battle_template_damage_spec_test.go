package httpapi

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func testSingleEnemyTargeting(position string) battleSkillTargeting {
	if position == "" {
		position = "vanguard"
	}
	return battleSkillTargeting{
		TargetPositions: []string{position},
		SecondarySeek:   "none",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount: battleSkillTargetCount{
			Base: 1,
		},
	}
}

func testDamageSkill(skillID string, level int, targeting battleSkillTargeting, spec battleDamageSpec) battleSkillState {
	spec = normalizeBattleDamageSpecForRuntime(spec, spec.DamageType, spec.AmountFormula)
	effect := battleSkillEffect{
		Kind:          "damage",
		DamageType:    spec.DamageType,
		Targeting:     targeting,
		AmountFormula: spec.AmountFormula,
		DamageSpec:    spec,
	}
	return battleSkillState{
		ID:        skillID,
		Name:      skillID,
		Level:     maxInt(1, level),
		Targeting: targeting,
		Effects:   []battleSkillEffect{effect},
	}
}

func runDamageSkill(t *testing.T, s *Server, actor battleUnitState, target battleUnitState, skill battleSkillState) (battleSkillExecution, *battleRuntimeState) {
	t.Helper()
	state := &battleRuntimeState{
		Seed:  11,
		Wave:  1,
		Seq:   0,
		Order: []string{},
		Players: []battleUnitState{
			actor,
		},
		Enemies: []battleUnitState{
			target,
		},
	}
	execution := s.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
	return execution, state
}

func runSkillExecution(t *testing.T, s *Server, state *battleRuntimeState, skill battleSkillState) battleSkillExecution {
	t.Helper()
	return s.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
}

func TestTemplateDamageUsesDefenseAndCrit(t *testing.T) {
	s := &Server{}
	skill := testDamageSkill(
		"skill_physical",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MP:       10,
		MaxHP:    100,
		MaxMP:    10,
		Stats: map[string]float64{
			"physics_damage": 50,
			"crit_chance":    1,
			"crit_damage":    0.5,
			"hp":             100,
			"mp":             10,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"physics_defence": 20,
			"hp":              100,
			"mp":              0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Effects) != 1 {
		t.Fatalf("effects=%d, want 1", len(execution.Effects))
	}
	effect := execution.Effects[0]
	if !effect["crit"].(bool) {
		t.Fatalf("crit should be true")
	}
	if effect["damageType"] != "physical" {
		t.Fatalf("damageType=%v, want physical", effect["damageType"])
	}
	if math.Abs(state.Enemies[0].HP-37.5) > 1e-6 {
		t.Fatalf("enemy hp=%v, want 37.5", state.Enemies[0].HP)
	}
}

func TestTemplateDamageUsesMagicCritAndCritDamageResist(t *testing.T) {
	s := &Server{}
	skill := testDamageSkill(
		"skill_magical",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "magical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "magical_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MP:       10,
		MaxHP:    100,
		MaxMP:    10,
		Stats: map[string]float64{
			"magical_damage":    50,
			"crit_chance":       0,
			"crit_damage":       0.5,
			"magic_crit_chance": 1,
			"magic_crit_damage": 0.5,
			"hp":                100,
			"mp":                10,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"magical_defence":    20,
			"crit_damage_resist": 0.5,
			"hp":                 100,
			"mp":                 0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Effects) != 1 {
		t.Fatalf("effects=%d, want 1", len(execution.Effects))
	}
	effect := execution.Effects[0]
	if !effect["crit"].(bool) {
		t.Fatalf("crit should be true")
	}
	if effect["damageType"] != "magical" {
		t.Fatalf("damageType=%v, want magical", effect["damageType"])
	}
	if math.Abs(state.Enemies[0].HP-47.9167) > 1e-6 {
		t.Fatalf("enemy hp=%v, want 47.9167", state.Enemies[0].HP)
	}
}

func TestBasicAttackProfileOverridesDamageTypeAndFormula(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	targeting := testSingleEnemyTargeting("vanguard")
	skill := testDamageSkill(
		"basic_attack",
		1,
		targeting,
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MP:       10,
		MaxHP:    100,
		MaxMP:    10,
		Stats: map[string]float64{
			"physics_damage": 50,
			"magical_damage": 50,
			"hp":             100,
			"mp":             10,
		},
		BasicAttack: &battleBasicAttackProfile{
			TargetDirection:    "midguard",
			TargetSeekStrategy: "front_to_rear",
			DamageType:         "magical",
			DamageSpec: battleDamageSpec{
				DamageType: "magical",
				AmountFormula: battleSkillAmountFormula{
					Terms: []battleAttackTerm{{AttrKey: "magical_damage", Coefficient: 1}},
				},
				HitCount: battleSkillTargetCount{Base: 1},
			},
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "midguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"physics_defence": 0,
			"magical_defence": 40,
			"hp":              100,
			"mp":              0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Effects) != 1 {
		t.Fatalf("effects=%d, want 1", len(execution.Effects))
	}
	if execution.Effects[0]["damageType"] != "magical" {
		t.Fatalf("damageType=%v, want magical", execution.Effects[0]["damageType"])
	}
	if math.Abs(state.Enemies[0].HP-64.2857) > 1e-6 {
		t.Fatalf("enemy hp=%v, want 64.2857", state.Enemies[0].HP)
	}
}

func TestTemplateDamageSupportsPenetrationAndAlias(t *testing.T) {
	s := &Server{}
	skill := testDamageSkill(
		"skill_arcane",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "magical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "magical_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
			FlatDefensePenetrationFormula: battleSkillAmountFormula{
				BaseAdd: 3,
			},
			PercentDefensePenetrationFormula: battleSkillAmountFormula{
				BaseAdd: 0.1,
			},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"magical_damage":                       50,
			"magicial_flat_defense_penetration":    5,
			"magical_flat_defense_penetration":     7,
			"magicial_percent_defense_penetration": 0.1,
			"magical_percent_defense_penetration":  0.05,
			"magical_damage_amplification":         0,
			"magical_damage_resist":                0,
			"hp":                                   100,
			"mp":                                   0,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"magical_defence": 40,
			"hp":              100,
			"mp":              0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Actions) == 0 {
		t.Fatalf("missing action rows")
	}
	// New formula: flat pen first, then percent pen.
	// Total flat=15, percent=0.25; effectiveDefense=(40-15)*(1-0.25)=18.75
	// damage=50*100/(100+18.75)=42.1053, hp=100-42.1053=57.8947
	if math.Abs(state.Enemies[0].HP-57.8947) > 1e-4 {
		t.Fatalf("enemy hp=%v, want 57.8947", state.Enemies[0].HP)
	}
	meta, _ := execution.Actions[0]["meta"].(map[string]any)
	if meta == nil {
		t.Fatalf("missing damage action meta")
	}
	if got := meta["effectiveDefense"]; got != 18.75 {
		t.Fatalf("effectiveDefense=%v, want 18.75", got)
	}
	if got := meta["flatPenetrationSource"]; got != "magicial_flat_defense_penetration+magical_flat_defense_penetration" {
		t.Fatalf("flatPenetrationSource=%v", got)
	}
	if got := meta["percentPenetrationSource"]; got != "magicial_percent_defense_penetration+magical_percent_defense_penetration" {
		t.Fatalf("percentPenetrationSource=%v", got)
	}
}

// When flat pen exceeds defense (with no percent pen), effectiveDefense goes negative,
// which amplifies damage via K / (K + effectiveDefense) instead of adding to attack.
// At effectiveDefense = -50, K=100: multiplier = 100/50 = 2x.
func TestTemplateDamageOverflowPenetrationAmplifiesViaMultiplier(t *testing.T) {
	s := &Server{}
	skill := testDamageSkill(
		"skill_overflow",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			// flatPen=70 > defense=20 → rawEffDef = (20-70)*1.0 = -50 (exactly at 2x cap)
			"physics_damage":                   10,
			"physics_flat_defense_penetration": 70,
			"physics_damage_amplification":     0.5,
			"crit_chance":                      1,
			"crit_damage":                      1,
			"hp":                               100,
			"mp":                               0,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       200,
		MaxHP:    200,
		Stats: map[string]float64{
			"physics_defence":       20,
			"physics_damage_resist": 0.2,
			"hp":                    200,
			"mp":                    0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Actions) == 0 {
		t.Fatalf("missing action rows")
	}
	// effectiveDefense = (20-70)*1.0 = -50 → multiplier = 100/(100-50) = 2.0x
	// damage = 10 * 2.0 = 20; crit(x2): 20*2=40; amp(x1.5): 40*1.5=60; resist(x0.8): 60*0.8=48
	if math.Abs(state.Enemies[0].HP-152) > 1e-4 {
		t.Fatalf("enemy hp=%v, want 152", state.Enemies[0].HP)
	}
	meta, _ := execution.Actions[0]["meta"].(map[string]any)
	if meta == nil {
		t.Fatalf("missing damage action meta")
	}
	// Raw effectiveDefense logged is -50 (before internal cap in ComputeDamage)
	if got := meta["effectiveDefense"]; got != -50.0 {
		t.Fatalf("effectiveDefense=%v, want -50", got)
	}
}

func TestTemplateDamageExtraAmplificationMergedIntoSourcePool(t *testing.T) {
	s := &Server{}
	skill := testDamageSkill(
		"skill_amp",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
			ExtraDamageAmplificationFormula: battleSkillAmountFormula{
				BaseAdd: 0.2,
			},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"physics_damage":               50,
			"physics_damage_amplification": 0.1,
			"hp":                           100,
			"mp":                           0,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"physics_defence": 20,
			"hp":              100,
			"mp":              0,
		},
	}
	_, state := runDamageSkill(t, s, actor, target, skill)
	if math.Abs(state.Enemies[0].HP-45.8333) > 1e-4 {
		t.Fatalf("enemy hp=%v, want 45.8333", state.Enemies[0].HP)
	}
}

func TestTemplateDamageHitCountUsesPerHitCrit(t *testing.T) {
	s := &Server{}
	skill := testDamageSkill(
		"skill_multihit",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 10,
			},
			HitCount: battleSkillTargetCount{Base: 2},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"crit_chance": 1,
			"crit_damage": 1,
			"hp":          100,
			"mp":          0,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"hp": 100,
			"mp": 0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Effects) != 2 {
		t.Fatalf("effects=%d, want 2", len(execution.Effects))
	}
	for idx, row := range execution.Effects {
		crit, _ := row["crit"].(bool)
		if !crit {
			t.Fatalf("effect[%d] should crit", idx)
		}
	}
	if math.Abs(state.Enemies[0].HP-60) > 1e-6 {
		t.Fatalf("enemy hp=%v, want 60", state.Enemies[0].HP)
	}
}

func TestLegacyDamageEffectWithoutDamageSpecStillWorks(t *testing.T) {
	s := &Server{}
	targeting := testSingleEnemyTargeting("vanguard")
	skill := battleSkillState{
		ID:        "skill_legacy_damage",
		Name:      "skill_legacy_damage",
		Level:     1,
		Targeting: targeting,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "magical",
				Targeting:  targeting,
				AmountFormula: battleSkillAmountFormula{
					Terms: []battleAttackTerm{{AttrKey: "magical_damage", Coefficient: 1}},
				},
			},
		},
	}
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"magical_damage": 45,
			"hp":             100,
			"mp":             0,
		},
	}
	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"magical_defence": 25,
			"hp":              100,
			"mp":              0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Effects) != 1 {
		t.Fatalf("effects=%d, want 1", len(execution.Effects))
	}
	if execution.Effects[0]["damageType"] != "magical" {
		t.Fatalf("damageType=%v, want magical", execution.Effects[0]["damageType"])
	}
	if math.Abs(state.Enemies[0].HP-64) > 1e-6 {
		t.Fatalf("enemy hp=%v, want 64", state.Enemies[0].HP)
	}
}

func TestSkillExecutionUsesEffectTargetingOverLegacyTargeting(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 11,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 50,
					"crit_chance":    0,
					"crit_damage":    0,
					"hp":             100,
					"mp":             0,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:v",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
			{
				UnitID:   "enemy:m",
				Side:     "enemy",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
		},
	}
	skill := battleSkillState{
		ID:        "skill_effect_targeting",
		Name:      "skill_effect_targeting",
		Level:     1,
		Targeting: testSingleEnemyTargeting("midguard"),
		Effects: []battleSkillEffect{
			{
				Kind:          "damage",
				DamageType:    "physical",
				Targeting:     testSingleEnemyTargeting("vanguard"),
				AmountFormula: battleSkillAmountFormula{BaseAdd: 50},
				DamageSpec: battleDamageSpec{
					DamageType: "physical",
					AmountFormula: battleSkillAmountFormula{
						BaseAdd: 50,
					},
					HitCount: battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("vanguard target should be damaged")
	}
	if state.Enemies[1].HP != 100 {
		t.Fatalf("midguard target should stay untouched, hp=%v", state.Enemies[1].HP)
	}
}

func TestCorpseExplosionDamagesFourEnemiesAndConsumesCorpse(t *testing.T) {
	s := &Server{}
	corpseTargeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard", "midguard", "rearguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "all",
		SelectRule:      "attr_max",
		SelectAttrKey:   "hp",
		TargetCount:     battleSkillTargetCount{Base: 1, Max: 1, MaxSet: true},
	}
	targeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard", "midguard", "rearguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount:     battleSkillTargetCount{Base: 4, Max: 4, MaxSet: true},
	}
	skill := battleSkillState{
		ID:        "perception_corpse_explosion",
		Name:      "perception_corpse_explosion",
		Level:     5,
		Targeting: targeting,
		Effects: []battleSkillEffect{{
			Kind:            "corpse_explosion",
			DamageType:      "magical",
			Targeting:       targeting,
			CorpseTargeting: corpseTargeting,
			ConsumeCorpse:   true,
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{
					{AttrKey: "corpse_max_hp", Coefficient: 0.31, PerSkillLevel: 0.01},
					{AttrKey: "perception", Coefficient: 0.31, PerSkillLevel: 0.01},
				},
			},
		}},
	}
	state := &battleRuntimeState{
		Seed: 11,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100,
				Stats: map[string]float64{"hp": 100, "mp": 0, "perception": 80},
			},
			{
				UnitID: "player:ally", Side: "player", Position: "midguard", Alive: true, HP: 100, MaxHP: 100,
				Stats: map[string]float64{"hp": 100, "mp": 0},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID: "enemy:corpse", Side: "enemy", Position: "vanguard", Alive: false, HP: 0, MaxHP: 200,
				Stats: map[string]float64{"hp": 200, "mp": 0},
			},
			{UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:2", Side: "enemy", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:3", Side: "enemy", Position: "midguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:4", Side: "enemy", Position: "rearguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:5", Side: "enemy", Position: "rearguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
		},
	}

	execution := runSkillExecution(t, s, state, skill)
	if got := len(execution.Effects); got != 4 {
		t.Fatalf("damage effects=%d, want 4, effects=%#v actions=%#v", got, execution.Effects, execution.Actions)
	}
	for idx := 1; idx <= 4; idx++ {
		if got := state.Enemies[idx].HP; math.Abs(got-2) > 1e-6 {
			t.Fatalf("enemy %d hp=%v, want 2", idx, got)
		}
	}
	if got := state.Enemies[5].HP; math.Abs(got-100) > 1e-6 {
		t.Fatalf("fifth enemy hp=%v, want 100", got)
	}
	if got := state.Players[0].HP; math.Abs(got-100) > 1e-6 {
		t.Fatalf("caster hp=%v, want 100", got)
	}
	if got := state.Players[1].HP; math.Abs(got-100) > 1e-6 {
		t.Fatalf("ally hp=%v, want 100", got)
	}
	if !state.Enemies[0].CorpseConsumed {
		t.Fatalf("corpse should be consumed")
	}
	if got := len(execution.Actions); got != 6 {
		t.Fatalf("actions=%d, want focus + 4 damage + consume, actions=%#v", got, execution.Actions)
	}
	if execution.Actions[0]["kind"] != "corpse_focus" || execution.Actions[0]["targetId"] != "enemy:corpse" || execution.Actions[0]["sourceId"] != "player:1" {
		t.Fatalf("expected caster to focus corpse first, actions=%#v", execution.Actions)
	}
	for idx := 1; idx <= 4; idx++ {
		if execution.Actions[idx]["kind"] != "damage" {
			t.Fatalf("action %d kind=%v, want damage, actions=%#v", idx, execution.Actions[idx]["kind"], execution.Actions)
		}
		if execution.Actions[idx]["sourceId"] != "player:1" {
			t.Fatalf("damage action %d sourceId=%v, want player:1", idx, execution.Actions[idx]["sourceId"])
		}
		if execution.Actions[idx]["visualSourceId"] != "enemy:corpse" {
			t.Fatalf("damage action %d visualSourceId=%v, want enemy:corpse", idx, execution.Actions[idx]["visualSourceId"])
		}
	}
	if execution.Actions[len(execution.Actions)-1]["kind"] != "corpse_consume" {
		t.Fatalf("expected corpse_consume action, actions=%#v", execution.Actions)
	}

	second := runSkillExecution(t, s, state, skill)
	if got := len(second.Effects); got != 0 {
		t.Fatalf("second cast effects=%d, want 0 after corpse consumed, effects=%#v", got, second.Effects)
	}
}

func TestTemplateDamageBossFirstSelectsBossTarget(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 11,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 50,
					"crit_chance":    0,
					"crit_damage":    0,
					"hp":             100,
					"mp":             0,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:    "enemy:boss",
				Side:      "enemy",
				EnemyType: "boss",
				Position:  "vanguard",
				Alive:     true,
				HP:        100,
				MaxHP:     100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
			{
				UnitID:   "enemy:normal",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
		},
	}
	skill := battleSkillState{
		ID:    "skill_boss_first",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "boss_first",
					TargetCount: battleSkillTargetCount{
						Base: 1,
					},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 50},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 50},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("boss target should be damaged")
	}
	if state.Enemies[1].HP != 100 {
		t.Fatalf("normal target should stay untouched, hp=%v", state.Enemies[1].HP)
	}
}

func TestTemplateDamageBonusAgainstBossAppliesOnlyToBoss(t *testing.T) {
	s := &Server{}
	baseSkill := func(enemyType string) float64 {
		state := &battleRuntimeState{
			Seed: 11,
			Wave: 1,
			Players: []battleUnitState{
				{
					UnitID:   "player:1",
					Side:     "player",
					Position: "vanguard",
					Alive:    true,
					HP:       100,
					MaxHP:    100,
					Stats: map[string]float64{
						"physics_damage": 50,
						"crit_chance":    0,
						"crit_damage":    0,
						"hp":             100,
						"mp":             0,
					},
				},
			},
			Enemies: []battleUnitState{
				{
					UnitID:    "enemy:1",
					Side:      "enemy",
					EnemyType: enemyType,
					Position:  "vanguard",
					Alive:     true,
					HP:        100,
					MaxHP:     100,
					Stats: map[string]float64{
						"physics_defence": 0,
						"hp":              100,
						"mp":              0,
					},
				},
			},
		}
		skill := battleSkillState{
			ID:    "skill_bonus",
			Level: 1,
			Effects: []battleSkillEffect{
				{
					Kind:          "damage",
					DamageType:    "physical",
					Targeting:     testSingleEnemyTargeting("vanguard"),
					AmountFormula: battleSkillAmountFormula{BaseAdd: 50},
					DamageSpec: battleDamageSpec{
						DamageType:    "physical",
						AmountFormula: battleSkillAmountFormula{BaseAdd: 50},
						HitCount:      battleSkillTargetCount{Base: 1},
						BonusAgainstTargetTypes: map[string]float64{
							"boss": 0.2,
						},
					},
				},
			},
		}
		_ = runSkillExecution(t, s, state, skill)
		return state.Enemies[0].HP
	}
	bossHP := baseSkill("boss")
	normalHP := baseSkill("normal")
	eliteHP := baseSkill("elite")
	if bossHP >= normalHP {
		t.Fatalf("boss hp=%v, normal hp=%v; boss should take more damage", bossHP, normalHP)
	}
	if eliteHP != normalHP {
		t.Fatalf("elite hp=%v, normal hp=%v; elite should not receive boss bonus", eliteHP, normalHP)
	}
}

func TestBasicAttackProfileSupportsMultipleEffects(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	state := &battleRuntimeState{
		Seed: 11,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 40,
					"crit_chance":    0,
					"crit_damage":    0,
					"hp":             100,
					"mp":             0,
				},
				BasicAttack: &battleBasicAttackProfile{
					Effects: []battleSkillEffect{
						{
							Kind:       "damage",
							DamageType: "physical",
							Targeting: battleSkillTargeting{
								TargetPositions: []string{"vanguard"},
								SecondarySeek:   "front_to_rear",
								TargetScope:     "enemy",
								SelectRule:      "first",
								TargetCount:     battleSkillTargetCount{Base: 1},
							},
							AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
							DamageSpec: battleDamageSpec{
								DamageType:    "physical",
								AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
								HitCount:      battleSkillTargetCount{Base: 1},
							},
						},
						{
							Kind:       "damage",
							DamageType: "physical",
							Targeting: battleSkillTargeting{
								TargetPositions: []string{"midguard"},
								SecondarySeek:   "front_to_rear",
								TargetScope:     "enemy",
								SelectRule:      "first",
								TargetCount:     battleSkillTargetCount{Base: 1},
							},
							AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
							DamageSpec: battleDamageSpec{
								DamageType:    "physical",
								AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
								HitCount:      battleSkillTargetCount{Base: 1},
							},
						},
					},
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:v",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
			{
				UnitID:   "enemy:m",
				Side:     "enemy",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
		},
	}
	_ = runSkillExecution(t, s, state, battleSkillState{ID: "basic_attack", Level: 1})
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("vanguard target should be damaged")
	}
	if state.Enemies[1].HP >= 100 {
		t.Fatalf("midguard target should be damaged")
	}
}

func TestBasicAttackStatusOnlyEffectUsesEnhancePayload(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	state := &battleRuntimeState{
		Seed: 11,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100, "mp": 0},
				BasicAttack: &battleBasicAttackProfile{
					SourceEnhanceLevel: 20,
					Effects: []battleSkillEffect{
						{
							Kind: "status",
							Targeting: battleSkillTargeting{
								TargetPositions:     []string{"vanguard", "midguard", "rearguard"},
								SecondarySeek:       "none",
								TargetScope:         "ally",
								SelectRule:          "first",
								FallbackSelfIfEmpty: true,
								TargetCount:         battleSkillTargetCount{Base: 1},
							},
							StatusPayload: battleSkillStatusPayload{
								StatusIDs:         []string{"formation_commander_order"},
								DurationBaseTurns: 3,
								PayloadFormula: map[string]battleSkillAmountFormula{
									"attackBonus": {
										BaseAdd: 10,
										Terms:   []battleAttackTerm{{AttrKey: "equipment_enhance_level", Coefficient: 0.5}},
									},
								},
							},
						},
					},
				},
			},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
		},
	}

	execution := runSkillExecution(t, s, state, battleSkillState{ID: "basic_attack", Level: 1})
	if state.Enemies[0].HP != 100 {
		t.Fatalf("status-only basic attack should not damage enemies, hp=%v", state.Enemies[0].HP)
	}
	if len(state.Players[0].Buffs) != 1 {
		t.Fatalf("status-only basic attack should apply one buff, buffs=%#v", state.Players[0].Buffs)
	}
	buff := state.Players[0].Buffs[0]
	if buff.BuffID != "formation_commander_order" || buff.RemainTurns != 3 {
		t.Fatalf("unexpected buff applied: %#v", buff)
	}
	if got := toFloat64(buff.Payload["attackBonus"], 0); got != 20 {
		t.Fatalf("attackBonus payload=%v, want 20", got)
	}
	if len(execution.Effects) != 1 || strings.TrimSpace(fmt.Sprintf("%v", execution.Effects[0]["kind"])) != "buff" {
		t.Fatalf("execution effects=%#v, want one buff effect", execution.Effects)
	}
}

func TestTemplateSkillEffectOnlyWhenNoTargetFromEffects(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 21,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 40,
					"hp":             100,
					"mp":             0,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:r",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
		},
	}
	skill := battleSkillState{
		ID:    "skill_effect_link_fallback",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "none",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
			{
				Kind:                        "damage",
				DamageType:                  "physical",
				OnlyWhenNoTargetFromEffects: []int{1},
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"rearguard"},
					SecondarySeek:   "none",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 35},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 35},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("fallback effect should trigger when referenced effect has no targets")
	}
}

func TestTemplateSkillEffectOnlyWhenNoTargetFromEffectsBlocked(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 22,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 40,
					"hp":             100,
					"mp":             0,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:v",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
			{
				UnitID:   "enemy:r",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
		},
	}
	skill := battleSkillState{
		ID:    "skill_effect_link_block",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "none",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 40},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
			{
				Kind:                        "damage",
				DamageType:                  "physical",
				OnlyWhenNoTargetFromEffects: []int{1},
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"rearguard"},
					SecondarySeek:   "none",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 35},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 35},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("first effect should hit vanguard target")
	}
	if state.Enemies[1].HP < 100 {
		t.Fatalf("linked fallback effect should not trigger when referenced effect has targets")
	}
}

func TestTemplateSkillEffectReuseTargetsFromEffect(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 23,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 40,
					"hp":             100,
					"mp":             0,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:v",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
			{
				UnitID:   "enemy:r",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_defence": 0,
					"hp":              100,
					"mp":              0,
				},
			},
		},
	}
	skill := battleSkillState{
		ID:    "skill_effect_link_reuse",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "none",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 30},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 30},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
			{
				Kind:                   "damage",
				DamageType:             "physical",
				ReuseTargetsFromEffect: 1,
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"rearguard"},
					SecondarySeek:   "none",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				AmountFormula: battleSkillAmountFormula{BaseAdd: 25},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 25},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("vanguard target should be damaged by first effect")
	}
	if state.Enemies[1].HP < 100 {
		t.Fatalf("second effect should reuse first effect target instead of selecting rearguard")
	}
}

func TestLegacyBasicAttackProfileWithoutDamageSpecStillWorks(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	targeting := testSingleEnemyTargeting("vanguard")
	skill := testDamageSkill(
		"basic_attack",
		1,
		targeting,
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MP:       10,
		MaxHP:    100,
		MaxMP:    10,
		Stats: map[string]float64{
			"physics_damage": 50,
			"magical_damage": 50,
			"hp":             100,
			"mp":             10,
		},
		BasicAttack: &battleBasicAttackProfile{
			TargetDirection:    "midguard",
			TargetSeekStrategy: "front_to_rear",
			DamageType:         "magical",
			Formula: []battleAttackTerm{
				{AttrKey: "magical_damage", Coefficient: 1},
			},
		},
	}

	target := battleUnitState{
		UnitID:   "enemy:1",
		Side:     "enemy",
		Position: "midguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"physics_defence": 0,
			"magical_defence": 40,
			"hp":              100,
			"mp":              0,
		},
	}
	execution, state := runDamageSkill(t, s, actor, target, skill)
	if len(execution.Effects) != 1 {
		t.Fatalf("effects=%d, want 1", len(execution.Effects))
	}
	if execution.Effects[0]["damageType"] != "magical" {
		t.Fatalf("damageType=%v, want magical", execution.Effects[0]["damageType"])
	}
	if math.Abs(state.Enemies[0].HP-64.2857) > 1e-6 {
		t.Fatalf("enemy hp=%v, want 64.2857", state.Enemies[0].HP)
	}
}

func TestResolveBattleBasicAttackProfileDoesNotDependOnItemTypeWeapon(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Items = []catalog.Item{
		{
			ID:   "wood_staff",
			Type: []string{"equipment"},
		},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID:                 "wood_staff",
			BasicAttackTarget:  "midguard",
			TargetSeekStrategy: "front_to_rear",
			BasicAttackFormula: catalog.BasicAttackFormula{
				DamageType: "magical",
				Formula: []catalog.BasicAttackFormulaTerm{
					{AttrKey: "magical_damage", Coefficient: 0.8},
				},
			},
		},
	}

	profile := s.resolveBattleBasicAttackProfile([]store.EquippedItem{
		{
			TemplateItemID: "wood_staff",
			EquipType:      "battle",
			EquippedSlot:   "right_hand",
		},
	})
	if profile == nil {
		t.Fatalf("profile should not be nil")
	}
	if profile.DamageType != "magical" {
		t.Fatalf("damageType=%s, want magical", profile.DamageType)
	}
	if profile.TargetDirection != "midguard" {
		t.Fatalf("targetDirection=%s, want midguard", profile.TargetDirection)
	}
	if profile.TargetSeekStrategy != "front_to_rear" {
		t.Fatalf("targetSeekStrategy=%s, want front_to_rear", profile.TargetSeekStrategy)
	}
	if len(profile.Formula) != 1 || profile.Formula[0].AttrKey != "magical_damage" {
		t.Fatalf("formula=%+v, want magical_damage formula", profile.Formula)
	}
}

func TestResolveBattleBasicAttackProfileRequiresCompleteTemplateFields(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Items = []catalog.Item{
		{
			ID:   "broken_staff",
			Type: []string{"equipment"},
		},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID:                "broken_staff",
			BasicAttackTarget: "midguard",
			BasicAttackFormula: catalog.BasicAttackFormula{
				DamageType: "magical",
				Formula: []catalog.BasicAttackFormulaTerm{
					{AttrKey: "magical_damage", Coefficient: 1},
				},
			},
		},
	}

	profile := s.resolveBattleBasicAttackProfile([]store.EquippedItem{
		{
			TemplateItemID: "broken_staff",
			EquipType:      "battle",
			EquippedSlot:   "right_hand",
		},
	})
	if profile == nil {
		t.Fatalf("profile should not be nil when legacy fields can be recovered")
	}
	if profile.TargetSeekStrategy != "front_to_rear" {
		t.Fatalf("targetSeekStrategy=%s, want front_to_rear", profile.TargetSeekStrategy)
	}
	if profile.DamageType != "magical" {
		t.Fatalf("damageType=%s, want magical", profile.DamageType)
	}
}

func TestResolveBattleBasicAttackProfileSupportsLegacyWeaponSlot(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Items = []catalog.Item{
		{
			ID:   "wood_staff",
			Type: []string{"equipment"},
		},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID:                 "wood_staff",
			BasicAttackTarget:  "midguard",
			TargetSeekStrategy: "front_to_rear",
			BasicAttackFormula: catalog.BasicAttackFormula{
				DamageType: "magical",
				Formula: []catalog.BasicAttackFormulaTerm{
					{AttrKey: "magical_damage", Coefficient: 0.8},
				},
			},
		},
	}

	profile := s.resolveBattleBasicAttackProfile([]store.EquippedItem{
		{
			TemplateItemID: "wood_staff",
			EquipType:      "battle",
			EquippedSlot:   "weapon",
		},
	})
	if profile == nil {
		t.Fatalf("profile should not be nil for legacy weapon slot")
	}
	if profile.DamageType != "magical" {
		t.Fatalf("damageType=%s, want magical", profile.DamageType)
	}
}

func TestResolveBattleBasicAttackProfileFallsBackToSecondRightHand(t *testing.T) {
	s := &Server{}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID:                 "primary_blade",
			BasicAttackTarget:  "vanguard",
			TargetSeekStrategy: "front_to_rear",
			BasicAttackFormula: catalog.BasicAttackFormula{
				DamageType: "physical",
				Formula: []catalog.BasicAttackFormulaTerm{
					{AttrKey: "strength", Coefficient: 1},
				},
			},
		},
		{
			ID:                 "second_blade",
			BasicAttackTarget:  "rearguard",
			TargetSeekStrategy: "rear_to_front",
			BasicAttackFormula: catalog.BasicAttackFormula{
				DamageType: "physical",
				Formula: []catalog.BasicAttackFormulaTerm{
					{AttrKey: "dexterity", Coefficient: 1},
				},
			},
		},
	}

	t.Run("uses second hand when primary is empty", func(t *testing.T) {
		profile := s.resolveBattleBasicAttackProfile([]store.EquippedItem{
			{TemplateItemID: "second_blade", EquipType: "battle", EquippedSlot: "right_hand_2"},
		})
		if profile == nil || len(profile.Formula) != 1 || profile.Formula[0].AttrKey != "dexterity" {
			t.Fatalf("profile=%+v, want second-hand basic attack", profile)
		}
	})

	t.Run("primary hand remains authoritative", func(t *testing.T) {
		profile := s.resolveBattleBasicAttackProfile([]store.EquippedItem{
			{TemplateItemID: "second_blade", EquipType: "battle", EquippedSlot: "right_hand_2"},
			{TemplateItemID: "primary_blade", EquipType: "battle", EquippedSlot: "right_hand"},
		})
		if profile == nil || len(profile.Formula) != 1 || profile.Formula[0].AttrKey != "strength" {
			t.Fatalf("profile=%+v, want primary-hand basic attack", profile)
		}
	})
}

func TestTemplateTargetingUntargetableByEnemy(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "stealth_hide",
			"buffId":  "stealth",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "untargetable_by_enemy"}},
		},
	}
	skill := testDamageSkill(
		"skill_target_filter",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 20,
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	state := &battleRuntimeState{
		Seed:  7,
		Wave:  1,
		Seq:   0,
		Order: []string{},
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage": 20,
					"hp":             100,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:stealth",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "stealth", RemainTurns: 2}},
			},
			{
				UnitID:   "enemy:plain",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP != 100 {
		t.Fatalf("stealth target should not be selected, hp=%v", state.Enemies[0].HP)
	}
	if state.Enemies[1].HP >= 100 {
		t.Fatalf("plain target should be selected")
	}
}

func TestTemplateTargetingIgnoreUntargetableBypassesFilter(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "stealth_hide",
			"buffId":  "stealth",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "untargetable_by_enemy"}},
		},
	}
	skill := testDamageSkill(
		"skill_target_filter_bypass",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 20,
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	skill.Effects[0].IgnoreUntargetable = true
	state := &battleRuntimeState{
		Seed:  7,
		Wave:  1,
		Seq:   0,
		Order: []string{},
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:stealth",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "stealth", RemainTurns: 2}},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("stealth target should be hit when ignoreUntargetable=true")
	}
}

func TestPanicTreatAllAsEnemyAffectsTemplateAndQuerySelection(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "panic_scope",
			"buffId":  "panic",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "treat_all_as_enemy"}},
		},
		{
			"id":      "stealth_hide",
			"buffId":  "stealth",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "untargetable_by_enemy"}},
		},
	}
	skill := testDamageSkill(
		"skill_panic_enemy_scope",
		1,
		battleSkillTargeting{
			TargetPositions: []string{"vanguard"},
			SecondarySeek:   "none",
			TargetScope:     "enemy",
			SelectRule:      "first",
			TargetCount:     battleSkillTargetCount{Base: 1},
		},
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 20,
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	state := &battleRuntimeState{
		Seed:  7,
		Wave:  1,
		Seq:   0,
		Order: []string{},
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "panic", RemainTurns: 2}},
			},
			{
				UnitID:   "player:ally",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "stealth", RemainTurns: 2}},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Players[1].HP != 100 {
		t.Fatalf("ally with stealth should be filtered from panic enemy-scope targeting")
	}
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("enemy should be selected after stealth ally is filtered")
	}

	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), s, nil)
	targets := runtime.SelectTargets(battleskills.TargetQuery{Side: "enemy", AliveOnly: true})
	if len(targets) != 2 {
		t.Fatalf("query enemy targets mismatch: %#v", targets)
	}
	if targets[0].UnitID != "enemy:1" || targets[1].UnitID != "player:1" {
		t.Fatalf("query enemy targets order mismatch: %#v", targets)
	}

	allTargets := runtime.SelectTargets(battleskills.TargetQuery{
		Side:               "enemy",
		AliveOnly:          true,
		IgnoreUntargetable: true,
	})
	if len(allTargets) != 3 {
		t.Fatalf("query with IgnoreUntargetable should include actor+ally+enemy, got %d", len(allTargets))
	}
}

func TestTemplateTargetingSelfScopeSelectsActor(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:actor",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	targets := s.battleSelectTemplateTargets(state, &state.Players[0], battleSkillState{Level: 1}, battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "none",
		TargetScope:     "self",
		SelectRule:      "random",
		TargetCount:     battleSkillTargetCount{Base: 1},
	}, false, nil)
	if len(targets) != 1 || targets[0] == nil || targets[0].UnitID != "player:actor" {
		t.Fatalf("self scope should select actor, got %#v", targets)
	}
}

func TestNormalizeBattleSelectRuleTreatsSelfFirstAsSelf(t *testing.T) {
	if got := normalizeBattleSelectRule("self_first"); got != "self" {
		t.Fatalf("normalizeBattleSelectRule(self_first)=%q, want self", got)
	}
}

func TestTemplateTargetingSelfSelectRuleSelectsActor(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:actor",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	targets := s.battleSelectTemplateTargets(state, &state.Players[0], battleSkillState{Level: 1}, battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "none",
		TargetScope:     "enemy",
		SelectRule:      "self",
		TargetCount:     battleSkillTargetCount{Base: 1},
	}, false, nil)
	if len(targets) != 1 || targets[0] == nil || targets[0].UnitID != "player:actor" {
		t.Fatalf("self rule should select actor, got %#v", targets)
	}
}

func TestCamouflageRemapsTargetingOnlyWithRealAlly(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "camouflage_scope",
			"buffId":  "camouflage",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "camouflage_targeting_remap"}},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:actor",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
			{
				UnitID:   "player:ally",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:camouflage",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs:    []battleBuffState{{BuffID: "camouflage", RemainTurns: 1}},
			},
			{
				UnitID:   "enemy:plain",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	actor := &state.Players[0]
	enemyTargets := s.battleTemplateTargetsByScope(state, actor, "enemy")
	if len(enemyTargets) != 1 || enemyTargets[0].UnitID != "enemy:plain" {
		t.Fatalf("enemy targets should exclude camouflage enemy when real ally exists: %#v", enemyTargets)
	}
	allyTargets := s.battleTemplateTargetsByScope(state, actor, "ally")
	if len(allyTargets) != 3 {
		t.Fatalf("ally targets should include self+real ally+camouflage enemy, got %#v", allyTargets)
	}
	seen := map[string]bool{}
	for _, unit := range allyTargets {
		seen[unit.UnitID] = true
	}
	if !seen["player:actor"] || !seen["player:ally"] || !seen["enemy:camouflage"] {
		t.Fatalf("ally targets mismatch: %#v", allyTargets)
	}
}

func TestCamouflageIgnoredWhenNoRealAllyOrAllEnemiesCamouflage(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "camouflage_scope",
			"buffId":  "camouflage",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "camouflage_targeting_remap"}},
		},
	}
	stateNoAlly := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:solo",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:camouflage",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs:    []battleBuffState{{BuffID: "camouflage", RemainTurns: 1}},
			},
			{
				UnitID:   "enemy:plain",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	enemyTargets := s.battleTemplateTargetsByScope(stateNoAlly, &stateNoAlly.Players[0], "enemy")
	if len(enemyTargets) != 2 {
		t.Fatalf("camouflage should be ignored without real ally: %#v", enemyTargets)
	}

	stateAllCamouflage := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:actor",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
			{
				UnitID:   "player:ally",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:camouflage:a",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs:    []battleBuffState{{BuffID: "camouflage", RemainTurns: 1}},
			},
			{
				UnitID:   "enemy:camouflage:b",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs:    []battleBuffState{{BuffID: "camouflage", RemainTurns: 1}},
			},
		},
	}
	enemyTargets = s.battleTemplateTargetsByScope(stateAllCamouflage, &stateAllCamouflage.Players[0], "enemy")
	if len(enemyTargets) != 2 {
		t.Fatalf("camouflage should fallback when all enemies are camouflage: %#v", enemyTargets)
	}
}

func TestEquipmentHookAddsCamouflageAfterTakeDamage(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "camouflage", "name": "camouflage", "duration": 1},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "base2_sporeveil_robe",
			BattleHooks: []map[string]any{
				{
					"id":       "robe_camouflage",
					"trigger":  "on_after_take_damage",
					"priority": 0,
					"effects": []map[string]any{
						{"kind": "add_buff", "buffId": "camouflage", "durationTurns": 1},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":       100,
					"strength": 50,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
		},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "base2_sporeveil_robe", EquipType: "battle", EquippedSlot: "body"},
	})
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "test",
	})
	if len(state.Players[0].Buffs) == 0 || state.Players[0].Buffs[0].BuffID != "camouflage" {
		t.Fatalf("camouflage buff should be added by equipment hook, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Buffs[0].RemainTurns != 1 {
		t.Fatalf("camouflage remainTurns=%d, want 1", state.Players[0].Buffs[0].RemainTurns)
	}
}

func TestCrystalFangKillStacksTriggerBloodRage(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "crystal_fang_soul_hunger", "stackPolicy": "stack_count", "maxStacks": 5, "durationTurns": -1},
		{"id": "crystal_fang_blood_rage", "stackPolicy": "refresh_duration", "durationTurns": -1, "permanent": true},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "crystal_fang_blood_rage_while_active",
			"buffId":  "crystal_fang_blood_rage",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": "add_stat", "attr": "physics_damage", "value": 150},
				{"kind": "add_stat", "attr": "strength", "value": 50},
			},
		},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "crystal_fang",
			BattleHooks: []map[string]any{
				{
					"id":       "crystal_fang_soul_stack",
					"trigger":  "on_kill",
					"priority": 5,
					"conditions": []map[string]any{
						{"type": "self_buff_count_lte", "buffId": "crystal_fang_soul_hunger", "value": 4},
					},
					"effects": []map[string]any{
						{"kind": "add_buff", "buffId": "crystal_fang_soul_hunger", "durationTurns": 0, "targetMode": "self"},
					},
				},
				{
					"id":       "crystal_fang_awakening",
					"trigger":  "on_kill",
					"priority": 4,
					"conditions": []map[string]any{
						{"type": "self_buff_count_eq", "buffId": "crystal_fang_soul_hunger", "value": 5},
					},
					"effects": []map[string]any{
						{"kind": "remove_buff", "buffId": "crystal_fang_soul_hunger", "targetMode": "self"},
						{"kind": "add_buff", "buffId": "crystal_fang_blood_rage", "durationTurns": 0, "targetMode": "self"},
					},
				},
			},
		},
	}

	unit := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"hp":             100,
			"physics_damage": 80,
			"strength":       150,
		},
		BaseStats: map[string]float64{
			"hp":             100,
			"physics_damage": 80,
			"strength":       150,
		},
	}
	unit.EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "crystal_fang", EquipType: "battle", EquippedSlot: "right_hand"},
	})
	if len(unit.EquipmentHooks) != 2 {
		t.Fatalf("equipment hooks=%#v, want 2 hooks", unit.EquipmentHooks)
	}

	for kill := 1; kill <= 5; kill++ {
		actions := s.battleTriggerUnitEquipmentHooks(nil, time.Unix(0, 0), &unit, nil, "on_kill", 0, "")
		if len(actions) == 0 {
			t.Fatalf("kill %d should produce equipment hook actions", kill)
		}
	}

	if got := battleCountBuff(&unit, "crystal_fang_soul_hunger"); got != 0 {
		t.Fatalf("soul hunger stacks=%d, want 0 after awakening, buffs=%#v", got, unit.Buffs)
	}
	if !battleHasBuff(&unit, "crystal_fang_blood_rage") {
		t.Fatalf("blood rage should be present after 5 kills, buffs=%#v", unit.Buffs)
	}
	if got := unit.Stats["physics_damage"]; got != 230 {
		t.Fatalf("physics_damage=%v, want 230", got)
	}
	if got := unit.Stats["strength"]; got != 200 {
		t.Fatalf("strength=%v, want 200", got)
	}
}

func TestAffixBattleHookAddsCamouflageAfterTakeDamage(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "camouflage", "name": "camouflage", "duration": 1},
	}
	s.deps.GameData.AffixDefinitions = []catalog.AffixDefinition{
		{
			ID:         "battle_sporeveil_on_hit",
			EffectType: "battle_trigger",
			EffectParams: map[string]any{
				"battleHooks": []map[string]any{
					{
						"id":       "affix_camouflage_on_hit",
						"trigger":  "on_after_take_damage",
						"priority": 0,
						"effects": []map[string]any{
							{"kind": "add_buff", "buffId": "camouflage", "durationTurns": 1, "targetMode": "self"},
						},
					},
				},
			},
			Active: true,
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":       100,
					"strength": 50,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
		},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{
			TemplateItemID: "base2_sporeveil_robe",
			EquipType:      "battle",
			EquippedSlot:   "body",
			Affixes: []store.EquipmentAffixSlot{
				{
					SlotIndex:  0,
					AffixID:    "battle_sporeveil_on_hit",
					RarityID:   "rare",
					EffectType: "battle_trigger",
					EffectParams: map[string]any{
						"battleHooks": []map[string]any{
							{
								"id":       "affix_camouflage_on_hit",
								"trigger":  "on_after_take_damage",
								"priority": 0,
								"effects": []map[string]any{
									{"kind": "add_buff", "buffId": "camouflage", "durationTurns": 1, "targetMode": "self"},
								},
							},
						},
					},
				},
			},
		},
	})
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "test",
	})
	if len(state.Players[0].Buffs) == 0 || state.Players[0].Buffs[0].BuffID != "camouflage" {
		t.Fatalf("camouflage buff should be added by affix battle hook, buffs=%#v", state.Players[0].Buffs)
	}
}

func TestEquipmentHookSupportsSourceTargetMode(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "panic", "name": "panic", "duration": 1},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "base2_sporeveil_robe",
			BattleHooks: []map[string]any{
				{
					"id":       "robe_panic_source_on_hit",
					"trigger":  "on_after_take_damage",
					"priority": 0,
					"effects": []map[string]any{
						{"kind": "add_buff", "buffId": "panic", "durationTurns": 1, "targetMode": "source"},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
		},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "base2_sporeveil_robe", EquipType: "battle", EquippedSlot: "body"},
	})
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "test",
	})
	if len(state.Enemies[0].Buffs) == 0 || state.Enemies[0].Buffs[0].BuffID != "panic" {
		t.Fatalf("panic buff should be added to damage source, buffs=%#v", state.Enemies[0].Buffs)
	}
}

func TestEquipmentHookAllyAfterTakeDamageExtraTurn(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "triage_focus_orb_cooldown", "durationTurns": 3, "stackPolicy": "refresh_duration", "dispellable": false},
	}
	state := newTriageFocusOrbTestState()
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:ally",
		Amount:     75,
		DamageType: "physical",
		Reason:     "test",
	})
	if state.Players[1].HP != 25 {
		t.Fatalf("ally HP=%v, want 25", state.Players[1].HP)
	}
	if state.Players[0].MP != 65 {
		t.Fatalf("mage MP=%v, want 65 after 35 MP trigger cost", state.Players[0].MP)
	}
	if !battleHasBuff(&state.Players[0], "triage_focus_orb_cooldown") {
		t.Fatalf("mage should gain cooldown buff, buffs=%#v", state.Players[0].Buffs)
	}
	if state.TurnCursor != 1 {
		t.Fatalf("extra turn should not advance original turn cursor, got %d", state.TurnCursor)
	}
	if state.Enemies[0].HP != 100 || len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("extra turn must queue without casting inline: enemy=%#v queue=%#v", state.Enemies[0], state.ExtraTurnQueue)
	}
	if countActionKind(runtime.ActionLog(), "extra_turn") != 1 {
		t.Fatalf("expected one extra_turn action, actions=%#v", runtime.ActionLog())
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(1), nil); err != nil {
		t.Fatalf("process triage formal extra turn: %v", err)
	}
	if state.Enemies[0].HP >= 100 || state.TurnCursor != 1 || len(state.ExtraTurnQueue) != 0 {
		t.Fatalf("formal extra turn did not cast independently while preserving cursor: enemy=%#v cursor=%d queue=%#v", state.Enemies[0], state.TurnCursor, state.ExtraTurnQueue)
	}
}

func TestEquipmentHookAllyAfterTakeDamageExtraTurnSkipsInvalidCases(t *testing.T) {
	cases := []struct {
		name   string
		damage float64
		mutate func(*battleRuntimeState)
	}{
		{
			name:   "self_damage",
			damage: 75,
			mutate: func(state *battleRuntimeState) {
				state.Players[1].UnitID = "player:other"
			},
		},
		{
			name:   "summon_target",
			damage: 75,
			mutate: func(state *battleRuntimeState) {
				state.Players[1].SummonTemplateUnitID = "test_summon"
				state.Players[1].SummonSourceUnitID = "player:mage"
			},
		},
		{
			name:   "target_dead",
			damage: 100,
		},
		{
			name:   "target_above_threshold",
			damage: 60,
		},
		{
			name:   "mp_insufficient",
			damage: 75,
			mutate: func(state *battleRuntimeState) {
				state.Players[0].MP = 34
			},
		},
		{
			name:   "cooldown_present",
			damage: 75,
			mutate: func(state *battleRuntimeState) {
				state.Players[0].Buffs = []battleBuffState{{BuffID: "triage_focus_orb_cooldown", RemainTurns: 2}}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			s.deps.GameData.Buff = []map[string]any{
				{"id": "triage_focus_orb_cooldown", "durationTurns": 3, "stackPolicy": "refresh_duration", "dispellable": false},
			}
			state := newTriageFocusOrbTestState()
			targetID := "player:ally"
			if tc.name == "self_damage" {
				targetID = "player:mage"
			}
			if tc.mutate != nil {
				tc.mutate(state)
			}
			beforeMP := state.Players[0].MP
			runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
			runtime.ApplyDamage(battleskills.DamageAction{
				SourceID:   "enemy:1",
				TargetID:   targetID,
				Amount:     tc.damage,
				DamageType: "physical",
				Reason:     "test",
			})
			if countActionKind(runtime.ActionLog(), "extra_turn") != 0 {
				t.Fatalf("extra turn should not trigger, actions=%#v", runtime.ActionLog())
			}
			if state.Players[0].MP != beforeMP {
				t.Fatalf("mage MP changed without trigger: got %v want %v", state.Players[0].MP, beforeMP)
			}
		})
	}
}

func TestEquipmentHookExtraTurnDoesNotReenterSameHook(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "triage_focus_orb_cooldown", "durationTurns": 3, "stackPolicy": "refresh_duration", "dispellable": false},
	}
	state := newTriageFocusOrbTestState()
	state.Players[0].MP = 1000
	state.Players[0].MaxMP = 1000
	state.Players[0].Stats["mp"] = 1000
	state.Players[0].BaseStats = cloneFloatMap(state.Players[0].Stats)
	state.Players[0].Skills = []battleSkillState{testDamageSkill(
		"ally_probe",
		1,
		battleSkillTargeting{
			TargetPositions: []string{"vanguard"},
			SecondarySeek:   "none",
			TargetScope:     "ally",
			SelectRule:      "first",
			TargetCount:     battleSkillTargetCount{Base: 1},
		},
		battleDamageSpec{
			DamageType:    "magical",
			AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
			HitCount:      battleSkillTargetCount{Base: 1},
		},
	)}
	state.Players[0].SkillIDs = []string{"ally_probe"}
	state.Players[0].EquipmentHooks = append(state.Players[0].EquipmentHooks, battleEquipmentHookRule{
		ID:      "different_hook_nested_extra_turn",
		Trigger: "on_after_deal_damage",
		Conditions: []map[string]any{
			{"type": "incoming_damage_gte", "value": 1},
		},
		Effects: []map[string]any{
			{"kind": "consume_mp", "amountFormula": map[string]any{"baseAdd": 35, "terms": []any{}}, "requireFullAmount": true, "abortHookOnFail": true},
			{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
		},
	})

	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:ally",
		Amount:     75,
		DamageType: "physical",
		Reason:     "test",
	})
	if got := countActionKindRecursive(runtime.ActionLog(), "extra_turn"); got != 1 {
		t.Fatalf("extra_turn hooks should not grant the same unit another extra turn while running, got %d actions=%#v", got, runtime.ActionLog())
	}
	if state.Players[0].MP != 965 {
		t.Fatalf("nested extra_turn hook should be skipped before consuming MP, got MP=%v want 965", state.Players[0].MP)
	}
}

func newTriageFocusOrbTestState() *battleRuntimeState {
	triageHook := battleEquipmentHookRule{
		ID:            "triage_focus_orb_emergency_turn",
		Trigger:       "on_ally_after_take_damage",
		SubjectFilter: map[string]any{"requireAlive": true, "excludeUnitTypes": []any{"summon"}},
		Conditions: []map[string]any{
			{"type": "target_hp_percent_lte", "value": 30},
			{"type": "incoming_damage_gte", "value": 1},
			{"type": "self_mp_gte", "value": 35},
			{"type": "self_buff_count_eq", "buffId": "triage_focus_orb_cooldown", "value": 0},
		},
		Effects: []map[string]any{
			{"kind": "consume_mp", "amountFormula": map[string]any{"baseAdd": 35, "terms": []any{}}, "requireFullAmount": true, "abortHookOnFail": true},
			{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
			{"kind": "add_buff", "buffId": "triage_focus_orb_cooldown", "durationTurns": 3, "targetMode": "self"},
		},
	}
	mageSkill := testDamageSkill(
		"triage_bolt",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType:    "magical",
			AmountFormula: battleSkillAmountFormula{BaseAdd: 10},
			HitCount:      battleSkillTargetCount{Base: 1},
		},
	)
	return &battleRuntimeState{
		Seed:       7,
		Wave:       1,
		TurnCursor: 1,
		Order:      []string{"player:ally", "enemy:1", "player:mage"},
		Players: []battleUnitState{
			{
				UnitID:         "player:mage",
				UserID:         1,
				Side:           "player",
				Position:       "rearguard",
				Alive:          true,
				HP:             100,
				MaxHP:          100,
				MP:             100,
				MaxMP:          100,
				Stats:          map[string]float64{"hp": 100, "mp": 100, "magical_damage": 20, "speed": 5},
				Skills:         []battleSkillState{mageSkill},
				SkillIDs:       []string{"triage_bolt"},
				EquipmentHooks: []battleEquipmentHookRule{triageHook},
			},
			{
				UnitID:   "player:ally",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       0,
				MaxMP:    0,
				Stats:    map[string]float64{"hp": 100, "speed": 3},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100, "speed": 1},
			},
		},
	}
}

func countActionKind(actions []map[string]any, kind string) int {
	count := 0
	for _, action := range actions {
		if strings.TrimSpace(fmt.Sprintf("%v", action["kind"])) == kind {
			count++
		}
	}
	return count
}

func countActionKindRecursive(actions []map[string]any, kind string) int {
	count := 0
	for _, action := range actions {
		if strings.TrimSpace(fmt.Sprintf("%v", action["kind"])) == kind {
			count++
		}
		if turn := normalizeAnyMap(action["turn"]); len(turn) > 0 {
			count += countActionKindRecursive(battleAnyMapSlice(turn["actionLog"]), kind)
		}
	}
	return count
}

func TestEquipmentHookCanFreezeDamageTargetAndSource(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "frozen_action_suppressed", "name": "frozen", "duration": 1},
	}
	s.deps.GameData.Enemies = []map[string]any{
		{
			"id":         "ice_slime_001",
			"name":       "Ice Slime",
			"position":   "vanguard",
			"hp":         30,
			"mp":         0,
			"speed":      1,
			"skillIds":   []string{"basic_attack"},
			"nameKey":    "enemy.ice_slime_001.name",
			"descKey":    "enemy.ice_slime_001.desc",
			"level":      1,
			"type":       "normal",
			"disabled":   false,
			"strength":   0,
			"stamina":    1,
			"dexterity":  0,
			"perception": 0,
		},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "frostspore_scale_shield",
			BattleHooks: []map[string]any{
				{
					"id":      "freeze_after_deal",
					"trigger": "on_after_deal_damage",
					"effects": []map[string]any{
						{"kind": "add_buff", "buffId": "frozen_action_suppressed", "durationTurns": 1, "targetMode": "source", "procChanceBase": 1},
					},
				},
				{
					"id":      "freeze_after_take",
					"trigger": "on_after_take_damage",
					"effects": []map[string]any{
						{"kind": "add_buff", "buffId": "frozen_action_suppressed", "durationTurns": 1, "targetMode": "source", "procChanceBase": 1},
					},
				},
				{
					"id":      "summon_after_take",
					"trigger": "on_after_take_damage",
					"effects": []map[string]any{
						{
							"kind":           "summon",
							"targetMode":     "self",
							"procChanceBase": 1,
							"summonPayload": map[string]any{
								"unitId":          "ice_slime_001",
								"side":            "player",
								"countFormula":    map[string]any{"baseAdd": 1, "terms": []map[string]any{}},
								"durationTurns":   2,
								"maxCountPerUnit": 1,
							},
						},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "frostspore_scale_shield", EquipType: "battle", EquippedSlot: "right_hand"},
	})
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "player:1",
		TargetID:   "enemy:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "deal",
	})
	if len(state.Enemies[0].Buffs) == 0 || state.Enemies[0].Buffs[0].BuffID != "frozen_action_suppressed" {
		t.Fatalf("damage target should be frozen after deal damage, buffs=%#v", state.Enemies[0].Buffs)
	}

	state.Enemies[0].Buffs = nil
	runtime = newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "take",
	})
	if len(state.Enemies[0].Buffs) == 0 || state.Enemies[0].Buffs[0].BuffID != "frozen_action_suppressed" {
		t.Fatalf("damage source should be frozen after take damage, buffs=%#v", state.Enemies[0].Buffs)
	}
	if len(state.Players) != 2 {
		t.Fatalf("expected shield to summon one player-side slime, players=%#v", state.Players)
	}
	if state.Players[1].SummonRemainTurns != 2 {
		t.Fatalf("summon remain turns=%d, want 2", state.Players[1].SummonRemainTurns)
	}
	if !battleHasBuff(&state.Players[1], battleSummonBindingBuffID) {
		t.Fatalf("summoned unit should carry summon binding buff, buffs=%#v", state.Players[1].Buffs)
	}
	if !battleRemoveBuff(&state.Players[1], battleSummonBindingBuffID) {
		t.Fatalf("expected summon binding buff to be removable")
	}
	if state.Players[1].Alive || state.Players[1].HP > 0 {
		t.Fatalf("summoned unit should despawn when summon binding buff is removed, unit=%#v", state.Players[1])
	}
}

func TestSummonCapOverflowReplacesOldestPerCaster(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Enemies = []map[string]any{
		{
			"id":         "ice_slime_001",
			"name":       "Ice Slime",
			"position":   "vanguard",
			"hp":         30,
			"mp":         0,
			"speed":      1,
			"skillIds":   []string{"basic_attack"},
			"nameKey":    "enemy.ice_slime_001.name",
			"descKey":    "enemy.ice_slime_001.desc",
			"level":      1,
			"type":       "normal",
			"disabled":   false,
			"strength":   0,
			"stamina":    1,
			"dexterity":  0,
			"perception": 0,
		},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "summon_cap_test_staff",
			BattleHooks: []map[string]any{
				{
					"id":      "summon_on_battle_start",
					"trigger": "on_battle_start",
					"effects": []map[string]any{
						{
							"kind":       "summon",
							"targetMode": "self",
							"summonPayload": map[string]any{
								"unitId":          "ice_slime_001",
								"side":            "player",
								"countFormula":    map[string]any{"baseAdd": 2, "terms": []map[string]any{}},
								"durationTurns":   2,
								"maxCountPerUnit": 1,
								"onCapOverflow":   "replace_oldest",
							},
						},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 11,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "summon_cap_test_staff", EquipType: "battle", EquippedSlot: "right_hand"},
	})
	actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[0], nil, "on_battle_start", 0, "")

	if alive := battleCountSummonsByTemplateAndSource(state, "ice_slime_001", "player:1"); alive != 1 {
		t.Fatalf("alive summon count=%d, want 1; players=%#v", alive, state.Players)
	}
	replaced := false
	for _, action := range actions {
		if action == nil {
			continue
		}
		if kind, _ := action["kind"].(string); kind == "summon_expire" {
			if reason, _ := action["reason"].(string); reason == "summon_cap_replaced" {
				replaced = true
				break
			}
		}
	}
	if !replaced {
		t.Fatalf("expected summon_cap_replaced action, got %#v", actions)
	}
}

func TestSummonCasterDeathPolicyDespawnsSummon(t *testing.T) {
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    false,
				HP:       0,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
			{
				UnitID:                     "summon_player_1_ice_slime_001_1",
				Side:                       "player",
				Position:                   "vanguard",
				Alive:                      true,
				HP:                         30,
				MaxHP:                      30,
				Stats:                      map[string]float64{"hp": 30},
				SummonTemplateUnitID:       "ice_slime_001",
				SummonSourceUnitID:         "player:1",
				SummonOnCasterDeath:        "despawn_immediately",
				SummonOnDispel:             "despawn_immediately",
				SummonOnDurationEnd:        "despawn_immediately",
				SummonPersistAcrossBattles: false,
				Buffs: []battleBuffState{
					{BuffID: battleSummonBindingBuffID, RemainTurns: 2},
				},
			},
		},
	}
	actions := battleApplySummonCasterDeathPolicy(state)
	if len(actions) == 0 {
		t.Fatalf("expected summon despawn action when caster is dead")
	}
	summon := &state.Players[1]
	if summon.Alive || summon.HP > 0 {
		t.Fatalf("summon should despawn after caster death policy, unit=%#v", summon)
	}
}

func TestEquipmentHookAddBuffSupportsProcChanceFormula(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "panic", "name": "panic", "duration": 1},
	}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "base2_sporeveil_robe",
			BattleHooks: []map[string]any{
				{
					"id":      "robe_panic_source_on_hit",
					"trigger": "on_after_take_damage",
					"effects": []map[string]any{
						{
							"kind":           "add_buff",
							"buffId":         "panic",
							"durationTurns":  1,
							"targetMode":     "source",
							"procChanceBase": 0,
							"procChanceFormula": map[string]any{
								"baseAdd": 0,
								"terms": []map[string]any{
									{"attrKey": "strength", "coefficient": 0.02},
								},
							},
							"procChanceMax": 1,
						},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 3,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":       100,
					"strength": 50,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
		},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "base2_sporeveil_robe", EquipType: "battle", EquippedSlot: "body"},
	})
	if len(state.Players[0].EquipmentHooks) == 0 || len(state.Players[0].EquipmentHooks[0].Effects) == 0 {
		t.Fatalf("expected parsed equipment hook effects, got=%#v", state.Players[0].EquipmentHooks)
	}
	chance, configured := battleResolveEquipmentHookProcChance(&state.Players[0], state.Players[0].EquipmentHooks[0], state.Players[0].EquipmentHooks[0].Effects[0])
	formula := parseBattleSkillAmountFormula(state.Players[0].EquipmentHooks[0].Effects[0]["procChanceFormula"], nil)
	if !configured || chance < 0.99 {
		t.Fatalf("proc chance should parse from formula, configured=%v chance=%v formula=%#v effect=%#v", configured, chance, formula, state.Players[0].EquipmentHooks[0].Effects[0])
	}
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "test",
	})
	if len(state.Enemies[0].Buffs) == 0 || state.Enemies[0].Buffs[0].BuffID != "panic" {
		t.Fatalf("panic buff should be added when formula chance reaches cap, buffs=%#v", state.Enemies[0].Buffs)
	}

	state.Enemies[0].Buffs = nil
	state.Players[0].Stats["strength"] = 0
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "enemy:1",
		TargetID:   "player:1",
		Amount:     10,
		DamageType: "physical",
		Reason:     "test-zero",
	})
	if len(state.Enemies[0].Buffs) != 0 {
		t.Fatalf("panic buff should not be added when chance is zero, buffs=%#v", state.Enemies[0].Buffs)
	}
}

func TestEquipmentHookProcChanceFormulaUsesSourceEnhanceLevel(t *testing.T) {
	actor := &battleUnitState{
		Stats: map[string]float64{
			"perception": 50,
		},
	}
	hook := battleEquipmentHookRule{SourceEnhanceLevel: 10}
	effect := map[string]any{
		"procChanceBase": 0.33,
		"procChanceMin":  0,
		"procChanceMax":  0.7,
		"procChanceFormula": map[string]any{
			"baseAdd": 0,
			"terms": []map[string]any{
				{"attrKey": "perception", "coefficient": 0.0015},
				{"attrKey": "equipment_enhance_level", "coefficient": 0.004},
			},
		},
	}
	chance, configured := battleResolveEquipmentHookProcChance(actor, hook, effect)
	if !configured {
		t.Fatalf("expected proc chance to be configured")
	}
	if math.Abs(chance-0.445) > 0.000001 {
		t.Fatalf("chance=%v, want 0.445", chance)
	}

	hook.SourceEnhanceLevel = 100
	chance, configured = battleResolveEquipmentHookProcChance(actor, hook, effect)
	if !configured || math.Abs(chance-0.7) > 0.000001 {
		t.Fatalf("chance should clamp to max, configured=%v chance=%v", configured, chance)
	}
}

func TestEquipmentHookSourceEnhanceLevelResolvedFromEquippedItem(t *testing.T) {
	s := &Server{}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "enhanced_shield",
			BattleHooks: []map[string]any{
				{
					"id":      "template_hook",
					"trigger": "on_after_take_damage",
					"effects": []map[string]any{
						{"kind": "add_buff", "buffId": "panic", "durationTurns": 1, "targetMode": "source"},
					},
				},
			},
		},
	}
	hooks := s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{TemplateItemID: "enhanced_shield", EquipType: "battle", EquippedSlot: "right_hand", EnhanceLevel: 12},
	})
	if len(hooks) != 1 {
		t.Fatalf("expected one hook, got=%#v", hooks)
	}
	if hooks[0].SourceEnhanceLevel != 12 {
		t.Fatalf("SourceEnhanceLevel=%d, want 12", hooks[0].SourceEnhanceLevel)
	}
}

func TestRuntimeDamageAmpAggregatesAcrossTemplateAndAffixHooks(t *testing.T) {
	s := &Server{}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID: "breaker_charm",
			BattleHooks: []map[string]any{
				{
					"id":      "breaker_charm_focus",
					"trigger": "on_before_deal_damage",
					"conditions": []map[string]any{
						{"type": "target_hp_percent_gte", "value": 80},
						{"damageTypes": []string{"physical"}},
					},
					"effects": []map[string]any{
						{"kind": "runtime_damage_amp", "value": 0.3, "damageTypes": []string{"physical"}},
					},
				},
			},
		},
	}
	s.deps.GameData.AffixDefinitions = []catalog.AffixDefinition{
		{
			ID:         "affix_breaker_bonus",
			EffectType: "battle_trigger",
			EffectParams: map[string]any{
				"battleHooks": []map[string]any{
					{
						"id":      "affix_breaker_bonus_hook",
						"trigger": "on_before_deal_damage",
						"conditions": []map[string]any{
							{"type": "target_hp_percent_gte", "value": 80},
							{"damageTypes": []string{"physical"}},
						},
						"effects": []map[string]any{
							{"kind": "runtime_damage_amp", "value": 0.3},
						},
					},
				},
			},
			Active: true,
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"physics_damage_amplification": 0,
					"magical_damage_amplification": 0,
					"physics_damage":               20,
					"hp":                           100,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       200,
				MaxHP:    200,
				Stats: map[string]float64{
					"physics_damage_resist": 0,
					"magical_damage_resist": 0,
					"hp":                    200,
				},
			},
		},
	}
	state.Players[0].EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{
			TemplateItemID: "breaker_charm",
			EquipType:      "battle",
			EquippedSlot:   "accessory1",
			Affixes: []store.EquipmentAffixSlot{
				{
					SlotIndex:  0,
					AffixID:    "affix_breaker_bonus",
					RarityID:   "epic",
					EffectType: "battle_trigger",
					EffectParams: map[string]any{
						"battleHooks": []map[string]any{
							{
								"id":      "affix_breaker_bonus_hook",
								"trigger": "on_before_deal_damage",
								"conditions": []map[string]any{
									{"type": "target_hp_percent_gte", "value": 80},
									{"damageTypes": []string{"physical"}},
								},
								"effects": []map[string]any{
									{"kind": "runtime_damage_amp", "value": 0.3},
								},
							},
						},
					},
				},
			},
		},
	})

	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "player:1",
		TargetID:   "enemy:1",
		Amount:     50,
		DamageType: "physical",
		Reason:     "test",
	})
	if math.Abs(state.Enemies[0].HP-120) > 1e-6 {
		t.Fatalf("physical damage should aggregate runtime amp to 1.6x, hp=%v want=120", state.Enemies[0].HP)
	}

	state.Enemies[0].HP = 200
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   "player:1",
		TargetID:   "enemy:1",
		Amount:     50,
		DamageType: "magical",
		Reason:     "test",
	})
	if math.Abs(state.Enemies[0].HP-150) > 1e-6 {
		t.Fatalf("magical damage should not match physical damageTypes filter, hp=%v want=150", state.Enemies[0].HP)
	}
}

func TestTemplateTargetingPreferAsEnemyTarget(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "taunt_prefer_target",
			"buffId":  "taunt",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "prefer_as_enemy_target"}},
		},
	}
	skill := testDamageSkill(
		"skill_arcane_like",
		1,
		battleSkillTargeting{
			TargetPositions: []string{"vanguard", "midguard", "rearguard"},
			SecondarySeek:   "random_alive",
			TargetScope:     "enemy",
			SelectRule:      "random",
			TargetCount:     battleSkillTargetCount{Base: 3},
		},
		battleDamageSpec{
			DamageType: "magical",
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 10,
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	state := &battleRuntimeState{
		Seed:  99,
		Wave:  1,
		Seq:   0,
		Order: []string{},
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"magical_damage": 20,
					"hp":             100,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:taunt:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"magical_defence": 0, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "taunt", RemainTurns: 2}},
			},
			{
				UnitID:   "enemy:taunt:2",
				Side:     "enemy",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"magical_defence": 0, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "taunt", RemainTurns: 2}},
			},
			{
				UnitID:   "enemy:plain:1",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"magical_defence": 0, "hp": 100},
			},
			{
				UnitID:   "enemy:plain:2",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"magical_defence": 0, "hp": 100},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("taunt target #1 should be selected")
	}
	if state.Enemies[1].HP >= 100 {
		t.Fatalf("taunt target #2 should be selected")
	}
}

func TestTemplateTargetingPreferAsEnemyTargetOverridesRowFilter(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "taunt_prefer_target",
			"buffId":  "taunt",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "prefer_as_enemy_target"}},
		},
	}
	skill := testDamageSkill(
		"skill_row_restrict",
		1,
		battleSkillTargeting{
			TargetPositions: []string{"vanguard"},
			SecondarySeek:   "none",
			TargetScope:     "enemy",
			SelectRule:      "first",
			TargetCount:     battleSkillTargetCount{Base: 1},
		},
		battleDamageSpec{
			DamageType: "physical",
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 10,
			},
			HitCount: battleSkillTargetCount{Base: 1},
		},
	)
	state := &battleRuntimeState{
		Seed:  17,
		Wave:  1,
		Seq:   0,
		Order: []string{},
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:front:plain",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
			},
			{
				UnitID:   "enemy:rear:taunt",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_defence": 0, "hp": 100},
				Buffs:    []battleBuffState{{BuffID: "taunt", RemainTurns: 2}},
			},
		},
	}
	_ = runSkillExecution(t, s, state, skill)
	if state.Enemies[1].HP >= 100 {
		t.Fatalf("rear taunt target should override row filter and be selected")
	}
	if state.Enemies[0].HP < 100 {
		t.Fatalf("front non-taunt target should not be selected when taunt exists")
	}
}

func TestSelectTargetsPreferAsEnemyTargetForScriptLimit(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "taunt_prefer_target",
			"buffId":  "taunt",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "prefer_as_enemy_target"}},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:actor",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:taunt:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs:    []battleBuffState{{BuffID: "taunt", RemainTurns: 2}},
			},
			{
				UnitID:   "enemy:taunt:2",
				Side:     "enemy",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs:    []battleBuffState{{BuffID: "taunt", RemainTurns: 2}},
			},
			{
				UnitID:   "enemy:plain:1",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), s, nil)
	targets := runtime.SelectTargets(battleskills.TargetQuery{
		Side:      "enemy",
		AliveOnly: true,
		Limit:     2,
	})
	if len(targets) != 2 {
		t.Fatalf("targets len=%d, want 2", len(targets))
	}
	seen := map[string]bool{
		targets[0].UnitID: true,
		targets[1].UnitID: true,
	}
	if !seen["enemy:taunt:1"] || !seen["enemy:taunt:2"] {
		t.Fatalf("script target limit should prioritize taunt targets, got %#v", targets)
	}
}

func TestBattleStartEquipmentHooksApplyToAllUnits(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "taunt", "name": "taunt", "duration": 2},
	}
	playerHook := battleEquipmentHookRule{
		ID:       "p1_hook",
		Trigger:  "on_battle_start",
		Priority: 0,
		Effects: []map[string]any{
			{"kind": "add_buff", "buffId": "taunt", "durationTurns": 2, "targetMode": "self"},
		},
	}
	enemyHook := battleEquipmentHookRule{
		ID:       "e1_hook",
		Trigger:  "on_battle_start",
		Priority: 0,
		Effects: []map[string]any{
			{"kind": "add_buff", "buffId": "taunt", "durationTurns": 2, "targetMode": "self"},
		},
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
				EquipmentHooks: []battleEquipmentHookRule{playerHook},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:         "enemy:1",
				Side:           "enemy",
				Position:       "vanguard",
				Alive:          true,
				HP:             100,
				MaxHP:          100,
				Stats:          map[string]float64{"hp": 100},
				EquipmentHooks: []battleEquipmentHookRule{enemyHook},
			},
		},
	}
	actions := s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	if len(actions) != 2 {
		t.Fatalf("battle start hooks should trigger for all units, actions=%#v", actions)
	}
	if len(state.Players[0].Buffs) == 0 || state.Players[0].Buffs[0].BuffID != "taunt" {
		t.Fatalf("player should gain taunt on battle start, buffs=%#v", state.Players[0].Buffs)
	}
	if len(state.Enemies[0].Buffs) == 0 || state.Enemies[0].Buffs[0].BuffID != "taunt" {
		t.Fatalf("enemy should gain taunt on battle start, buffs=%#v", state.Enemies[0].Buffs)
	}
}

func TestBattleStartEquipmentHookSupportsFormationConditionsAndPayloadFormula(t *testing.T) {
	s := &Server{}
	hook := battleEquipmentHookRule{
		ID:                 "formation_commander_opening_orders",
		Trigger:            "on_battle_start",
		Priority:           0,
		SourceEnhanceLevel: 20,
		Conditions: []map[string]any{
			{"type": "self_position", "positions": []any{"midguard"}},
			{"type": "ally_side_has_positions", "positions": []any{"vanguard", "midguard", "rearguard"}},
		},
		Effects: []map[string]any{
			{
				"kind":       "add_buff",
				"buffId":     "formation_commander_rearline_order",
				"targetMode": "targeting",
				"targeting": map[string]any{
					"targetPositions": []any{"rearguard"},
					"targetScope":     "ally",
					"selectRule":      "first",
					"targetCount":     map[string]any{"base": 99, "max": 99},
				},
				"payloadFormula": map[string]any{
					"damageReduction": map[string]any{
						"baseAdd": float64(0.1),
						"terms": []any{
							map[string]any{"attrKey": "equipment_enhance_level", "coefficient": float64(0.0025)},
						},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:front", Side: "player", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "player:mid", Side: "player", Position: "midguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}, EquipmentHooks: []battleEquipmentHookRule{hook}},
			{UnitID: "player:rear", Side: "player", Position: "rearguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
		},
	}

	actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[1], nil, "on_battle_start", 0, "")
	if len(actions) != 1 {
		t.Fatalf("formation hook actions=%#v, want one action", actions)
	}
	if len(state.Players[2].Buffs) != 1 {
		t.Fatalf("rear unit should receive one order buff, buffs=%#v", state.Players[2].Buffs)
	}
	buff := state.Players[2].Buffs[0]
	if buff.BuffID != "formation_commander_rearline_order" {
		t.Fatalf("rear buff id=%s", buff.BuffID)
	}
	if got := toFloat64(buff.Payload["damageReduction"], 0); got != 0.15 {
		t.Fatalf("damageReduction payload=%v, want 0.15", got)
	}

	state.Players[2].Alive = false
	state.Players[2].HP = 0
	state.Players[2].Buffs = nil
	actions = s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[1], nil, "on_battle_start", 0, "")
	if len(actions) != 0 || len(state.Players[2].Buffs) != 0 {
		t.Fatalf("formation hook should not trigger without living rear line, actions=%#v buffs=%#v", actions, state.Players[2].Buffs)
	}
}

func TestApplyBuffRefreshesExistingBuffDuration(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
			},
		},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), s, nil)
	runtime.ApplyBuff(battleskills.BuffAction{
		SourceID:      "player:1",
		TargetID:      "player:1",
		BuffID:        "taunt",
		DurationTurns: 1,
		Reason:        "test",
	})
	runtime.ApplyBuff(battleskills.BuffAction{
		SourceID:      "player:1",
		TargetID:      "player:1",
		BuffID:        "taunt",
		DurationTurns: 3,
		Reason:        "test_refresh",
	})
	if len(state.Players[0].Buffs) != 1 {
		t.Fatalf("duplicate buff should refresh duration instead of stacking, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Buffs[0].RemainTurns != 3 {
		t.Fatalf("buff duration should refresh to latest value, got=%d", state.Players[0].Buffs[0].RemainTurns)
	}
}

func TestTemplateStatusRespectsPredicatesAndDurationFormula(t *testing.T) {
	s := &Server{}
	skill := battleSkillState{
		ID:    "perception_mind_control",
		Name:  "perception_mind_control",
		Level: 1,
		Targeting: battleSkillTargeting{
			TargetPositions: []string{"rearguard"},
			SecondarySeek:   "rear_to_front",
			TargetScope:     "enemy",
			SelectRule:      "first",
			TargetCount: battleSkillTargetCount{
				Base: 2,
				Max:  2,
			},
		},
		Effects: []battleSkillEffect{
			{
				Kind: "status",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"rearguard"},
					SecondarySeek:   "rear_to_front",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount: battleSkillTargetCount{
						Base: 2,
						Max:  2,
					},
					TargetPredicates: []battleSkillTargetPredicate{
						{Kind: "compare_attr", ActorAttr: "perception", TargetAttr: "perception", Operator: ">"},
						{Kind: "exclude_enemy_type", EnemyType: "boss"},
					},
				},
				StatusPayload: battleSkillStatusPayload{
					StatusIDs:                  []string{"basic_left_dagger_betrayal"},
					DurationBaseTurns:          2,
					DurationPerSkillLevelTurns: 0,
					DurationFormula: battleSkillAmountFormula{
						BaseAdd: 0,
						Terms: []battleAttackTerm{
							{AttrKey: "perception", Coefficient: 0.01},
						},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 3,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       100,
				MaxMP:    100,
				Stats: map[string]float64{
					"hp":         100,
					"mp":         100,
					"perception": 250,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:    "enemy:boss",
				Side:      "enemy",
				Position:  "rearguard",
				EnemyType: "boss",
				Alive:     true,
				HP:        100,
				MaxHP:     100,
				Stats: map[string]float64{
					"hp":         100,
					"perception": 50,
				},
			},
			{
				UnitID:    "enemy:1",
				Side:      "enemy",
				Position:  "rearguard",
				EnemyType: "normal",
				Alive:     true,
				HP:        100,
				MaxHP:     100,
				Stats: map[string]float64{
					"hp":         100,
					"perception": 80,
				},
			},
		},
	}
	execution := s.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
	if len(execution.Effects) == 0 {
		t.Fatalf("expected status effect to execute")
	}
	if len(state.Enemies[0].Buffs) != 0 {
		t.Fatalf("boss target should be filtered, buffs=%#v", state.Enemies[0].Buffs)
	}
	if len(state.Enemies[1].Buffs) != 1 {
		t.Fatalf("valid target should receive one buff, buffs=%#v", state.Enemies[1].Buffs)
	}
	if state.Enemies[1].Buffs[0].RemainTurns != 4 {
		t.Fatalf("duration should be 2 + floor(250*0.01)=4, got=%d", state.Enemies[1].Buffs[0].RemainTurns)
	}
}

func TestExcludeEnemyTypeBossDoesNotExcludeElite(t *testing.T) {
	s := &Server{}
	skill := battleSkillState{
		ID:    "test_skill",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind: "status",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 3, Max: 3},
					TargetPredicates: []battleSkillTargetPredicate{
						{Kind: "exclude_enemy_type", EnemyType: "boss"},
					},
				},
				StatusPayload: battleSkillStatusPayload{
					StatusIDs:         []string{"taunt"},
					DurationBaseTurns: 2,
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 1,
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:boss", Side: "enemy", Position: "vanguard", EnemyType: "boss", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:elite", Side: "enemy", Position: "vanguard", EnemyType: "elite", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:normal", Side: "enemy", Position: "vanguard", EnemyType: "normal", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
		},
	}
	_ = s.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
	if len(state.Enemies[0].Buffs) != 0 {
		t.Fatalf("boss should be excluded by exclude_enemy_type:boss, buffs=%#v", state.Enemies[0].Buffs)
	}
	if len(state.Enemies[1].Buffs) != 1 {
		t.Fatalf("elite should NOT be excluded by exclude_enemy_type:boss, buffs=%#v", state.Enemies[1].Buffs)
	}
	if len(state.Enemies[2].Buffs) != 1 {
		t.Fatalf("normal should not be excluded, buffs=%#v", state.Enemies[2].Buffs)
	}
}

func TestTemplateDamageConsumesStatusesOnHit(t *testing.T) {
	s := &Server{}
	skill := battleSkillState{
		ID:    "perception_shockburst",
		Name:  "perception_shockburst",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "magical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard", "midguard", "rearguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "all",
					SelectRule:      "random",
					TargetCount: battleSkillTargetCount{
						Base: 99,
						Max:  99,
					},
					TargetPredicates: []battleSkillTargetPredicate{
						{Kind: "require_target_has_status", StatusID: "panic"},
					},
				},
				DamageSpec: battleDamageSpec{
					DamageType: "magical",
					AmountFormula: battleSkillAmountFormula{
						BaseAdd: 10,
					},
					HitCount: battleSkillTargetCount{Base: 1},
				},
				ConsumeStatusesOnHit: []string{"panic"},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 5,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       100,
				MaxMP:    100,
				Stats: map[string]float64{
					"hp": 100,
					"mp": 100,
				},
			},
			{
				UnitID:   "player:2",
				Side:     "player",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp": 100,
				},
				Buffs: []battleBuffState{{BuffID: "panic", RemainTurns: 3}},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp": 100,
				},
				Buffs: []battleBuffState{{BuffID: "panic", RemainTurns: 2}},
			},
			{
				UnitID:   "enemy:2",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp": 100,
				},
			},
		},
	}
	execution := s.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
	if len(execution.Actions) == 0 {
		t.Fatalf("expected runtime actions")
	}
	if battleHasBuff(&state.Players[1], "panic") {
		t.Fatalf("panic should be consumed on allied panic target")
	}
	if battleHasBuff(&state.Enemies[0], "panic") {
		t.Fatalf("panic should be consumed on enemy panic target")
	}
	if battleHasBuff(&state.Enemies[1], "panic") {
		t.Fatalf("enemy without panic should remain unchanged")
	}
}

func TestTemplateEffectRequiredCasterStatusesGatesBreakStealthHit(t *testing.T) {
	s := &Server{}
	skill := battleSkillState{
		ID:    "veilstrike_test",
		Name:  "veilstrike_test",
		Level: 1,
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 10},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount:     battleSkillTargetCount{Base: 1},
				},
				ReuseTargetsFromEffect:     1,
				RequiredCasterStatuses:     []string{"shadow_paw_stealth"},
				ConsumeCasterStatusesOnHit: []string{"shadow_paw_stealth"},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 10},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	newState := func(withStealth bool) *battleRuntimeState {
		buffs := []battleBuffState(nil)
		if withStealth {
			buffs = []battleBuffState{{BuffID: "shadow_paw_stealth", RemainTurns: 1}}
		}
		return &battleRuntimeState{
			Seed: 11,
			Players: []battleUnitState{
				{
					UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
					HP: 100, MaxHP: 100,
					Stats: map[string]float64{"hp": 100, "strength": 10, "dexterity": 10, "physics_damage": 10},
					Buffs: buffs,
				},
			},
			Enemies: []battleUnitState{
				{
					UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
					HP: 100, MaxHP: 100,
					Stats: map[string]float64{"hp": 100, "physics_defence": 0},
				},
			},
		}
	}
	countDamage := func(actions []map[string]any) int {
		count := 0
		for _, action := range actions {
			if action["kind"] == "damage" {
				count++
			}
		}
		return count
	}

	plain := newState(false)
	plainRuntime := newBattleSkillActionRuntime(plain, &plain.Players[0], time.Unix(0, 0), s, nil)
	_ = s.executeBattleTemplateSkillEffects(plain, time.Unix(0, 0), &plain.Players[0], skill, plainRuntime)
	if got := countDamage(plainRuntime.ActionLog()); got != 1 {
		t.Fatalf("without stealth should only deal base hit, got %d actions=%#v", got, plainRuntime.ActionLog())
	}

	stealthed := newState(true)
	stealthRuntime := newBattleSkillActionRuntime(stealthed, &stealthed.Players[0], time.Unix(0, 0), s, nil)
	_ = s.executeBattleTemplateSkillEffects(stealthed, time.Unix(0, 0), &stealthed.Players[0], skill, stealthRuntime)
	if got := countDamage(stealthRuntime.ActionLog()); got != 2 {
		t.Fatalf("with stealth should deal base and break-stealth hit, got %d actions=%#v", got, stealthRuntime.ActionLog())
	}
	if battleHasBuff(&stealthed.Players[0], "shadow_paw_stealth") {
		t.Fatalf("break-stealth hit should consume stealth, buffs=%#v", stealthed.Players[0].Buffs)
	}
}
