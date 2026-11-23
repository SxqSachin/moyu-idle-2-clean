package httpapi

import (
	"testing"

	"mysecondwebgame/server/internal/rules"
)

func TestBuildBattleRuleEvalContextEnemyCountMetrics(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true, HP: 100},
			{UnitID: "player:2", Side: "player", Position: "rearguard", Alive: true, HP: 80},
			{UnitID: "player:3", Side: "player", Position: "midguard", Alive: false, HP: 0},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true, HP: 90},
			{UnitID: "enemy:2", Side: "enemy", Position: "midguard", Alive: true, HP: 60},
			{UnitID: "enemy:3", Side: "enemy", Position: "rearguard", Alive: false, HP: 0},
		},
	}
	ctx := s.buildBattleRuleEvalContext(
		state,
		battleUnitState{UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100},
		"player",
		battleSkillState{},
		false,
	)
	if ctx.EnemyCount != 2 {
		t.Fatalf("EnemyCount=%v, want 2", ctx.EnemyCount)
	}
	assertMetric := func(key string, want float64) {
		got := ctx.Metrics[key]
		if got != want {
			t.Fatalf("metric %s=%v, want %v", key, got, want)
		}
	}
	assertMetric("enemy_count.enemies.all", 2)
	assertMetric("enemy_count.enemies.vanguard", 1)
	assertMetric("enemy_count.enemies.midguard", 1)
	assertMetric("enemy_count.enemies.rearguard", 0)
	assertMetric("enemy_count.allies.all", 2)
	assertMetric("enemy_count.allies.vanguard", 1)
	assertMetric("enemy_count.all.all", 4)
	assertMetric("enemy_count.none.all", 0)
}

func TestBuildBattleRuleEvalContextTargetUnits(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "regen", "kind": "buff"},
		{"id": "poison", "kind": "debuff"},
	}
	state := &battleRuntimeState{
		Wave:           2,
		ActedUnits:     []string{"player:0", "enemy:0"},
		RuleTurnCounts: map[string]int{"player:1": 1},
		Players: []battleUnitState{
			{
				UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100, MP: 20, MaxMP: 40,
				Skills:      []battleSkillState{{ID: "heal", CooldownRemain: 0}},
				BattleItems: []battleItemState{{ItemID: "potion", CooldownRemain: 3}},
			},
			{
				UnitID: "player:2", Side: "player", Alive: true, HP: 60, MaxHP: 100, MP: 30, MaxMP: 40,
				Buffs: []battleBuffState{{BuffID: "poison"}},
			},
			{
				UnitID: "player:dead", Side: "player", Alive: false, HP: 0, MaxHP: 100,
			},
			{
				UnitID: "summon:1", Side: "player", Alive: true, HP: 30, MaxHP: 50,
				SummonTemplateUnitID: "wolf_summon", SummonSourceUnitID: "player:1",
			},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 80, MaxHP: 100, EnemyID: "forest_boss", EnemyType: "boss"},
		},
	}
	ctx := s.buildBattleRuleEvalContext(state, state.Players[0], "player", battleSkillState{ID: "heal"}, true)

	if ctx.Self == nil || ctx.Self.UnitID != "player:1" {
		t.Fatalf("Self=%#v, want player:1", ctx.Self)
	}
	if len(ctx.Allies) != 3 {
		t.Fatalf("Allies=%#v, want self, living ally, and summon", ctx.Allies)
	}
	if len(ctx.Enemies) != 1 || !ctx.Enemies[0].IsBoss {
		t.Fatalf("Enemies=%#v, want one boss enemy", ctx.Enemies)
	}
	if len(ctx.SelfSummonedUnits) != 1 || ctx.SelfSummonedUnits[0].TemplateUnitID != "wolf_summon" {
		t.Fatalf("SelfSummonedUnits=%#v, want wolf_summon", ctx.SelfSummonedUnits)
	}
	if ctx.PassedTurn != 2 {
		t.Fatalf("PassedTurn=%v, want 2", ctx.PassedTurn)
	}
	if ctx.Allies[1].StateKinds["poison"] != "debuff" {
		t.Fatalf("poison kind=%q, want debuff", ctx.Allies[1].StateKinds["poison"])
	}
}

func TestBuildBattleRuleEvalContextItemCooldownWithoutBuffs(t *testing.T) {
	s := &Server{}
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100,
		BattleItems: []battleItemState{{ItemID: "potion", CooldownRemain: 2}},
	}
	state := &battleRuntimeState{Players: []battleUnitState{actor}}
	ctx := s.buildBattleRuleEvalContext(state, actor, "player", battleSkillState{}, true)

	if got := ctx.ItemCooldowns["potion"]; got != 2 {
		t.Fatalf("ctx.ItemCooldowns[potion]=%v, want 2", got)
	}
	if !rules.Evaluate([]rules.ConditionGroup{{Conditions: []rules.Condition{{
		CompareType:           "itemCooldown",
		Target:                "self",
		CooldownCompareAction: "inCooldown",
		CooldownItemIDs:       []string{"potion"},
	}}}}, ctx) {
		t.Fatalf("item cooldown rule should read cooldown even when actor has no buffs")
	}
}

func TestBuildBattleRuleEvalContextCanSkipUnitContextForLegacyRules(t *testing.T) {
	s := &Server{}
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Alive: true, HP: 80, MaxHP: 100,
		BattleItems: []battleItemState{{ItemID: "potion", CooldownRemain: 1}},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, EnemyID: "slime"},
		},
	}
	ctx := s.buildBattleRuleEvalContext(state, actor, "player", battleSkillState{}, false)

	if ctx.Self != nil || len(ctx.Allies) != 0 || len(ctx.Enemies) != 0 || len(ctx.AllUnits) != 0 {
		t.Fatalf("legacy context should skip expanded unit slices: %#v", ctx)
	}
	if got := ctx.ItemCooldowns["potion"]; got != 1 {
		t.Fatalf("ctx.ItemCooldowns[potion]=%v, want 1", got)
	}
	if got := ctx.UnitAlive["slime"]; got != 1 {
		t.Fatalf("ctx.UnitAlive[slime]=%v, want 1", got)
	}
}

func TestBattleRuleNeedsUnitContextKeepsLegacyEnemyCountLight(t *testing.T) {
	legacyEnemyCount := []rules.ConditionGroup{{Conditions: []rules.Condition{{
		CompareType:         "value",
		Target:              "enemies",
		DataType:            "enemy_count",
		ValueCompareAction:  ">=",
		ValueConditionValue: 1,
	}}}}
	if battleRuleNeedsUnitContext(legacyEnemyCount) {
		t.Fatalf("legacy enemy_count rules should use metrics without expanded unit context")
	}

	allyHP := []rules.ConditionGroup{{Conditions: []rules.Condition{{
		CompareType:         "value",
		Target:              "allies",
		DataType:            "hp",
		ValueCompareAction:  "<=",
		ValueConditionValue: "70%",
	}}}}
	if !battleRuleNeedsUnitContext(allyHP) {
		t.Fatalf("ally hp rules should request expanded unit context")
	}
}

func TestBattleSkillRulePassedEnemyBossSpecialConditions(t *testing.T) {
	s := &Server{}
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100,
		SkillRules: map[string]battleSkillRuleState{
			"boss_skill": {
				Enabled: true,
				Groups: []rules.ConditionGroup{{Conditions: []rules.Condition{{
					CompareType:   "special",
					Target:        "enemies",
					SpecialAction: "enemyHasBoss",
				}}}},
			},
			"normal_skill": {
				Enabled: true,
				Groups: []rules.ConditionGroup{{Conditions: []rules.Condition{{
					CompareType:   "special",
					Target:        "enemies",
					SpecialAction: "enemyNotHasBoss",
				}}}},
			},
		},
	}
	bossSkill := battleSkillState{ID: "boss_skill"}
	normalSkill := battleSkillState{ID: "normal_skill"}
	state := &battleRuntimeState{
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{
			{UnitID: "enemy:boss", Side: "enemy", Alive: true, HP: 100, MaxHP: 100, EnemyID: "forest_boss", EnemyType: "boss"},
		},
	}

	if !s.battleSkillRulePassed(state, actor, "player", bossSkill) {
		t.Fatalf("enemyHasBoss skill rule should pass when a living enemy boss exists")
	}
	if s.battleSkillRulePassed(state, actor, "player", normalSkill) {
		t.Fatalf("enemyNotHasBoss skill rule should fail when a living enemy boss exists")
	}

	state.Enemies = []battleUnitState{
		{UnitID: "enemy:normal", Side: "enemy", Alive: true, HP: 100, MaxHP: 100, EnemyID: "slime", EnemyType: "normal"},
	}
	if s.battleSkillRulePassed(state, actor, "player", bossSkill) {
		t.Fatalf("enemyHasBoss skill rule should fail when no living enemy boss exists")
	}
	if !s.battleSkillRulePassed(state, actor, "player", normalSkill) {
		t.Fatalf("enemyNotHasBoss skill rule should pass when no living enemy boss exists")
	}
}
