package rules

import "testing"

func TestEvaluateEnemyCountDefaultsToLegacyEnemyCount(t *testing.T) {
	groups := []ConditionGroup{
		{
			Conditions: []Condition{
				{
					CompareType:         "value",
					DataType:            "enemy_count",
					ValueCompareAction:  ">=",
					ValueConditionValue: 2,
				},
			},
		},
	}
	if !Evaluate(groups, EvalContext{EnemyCount: 2}) {
		t.Fatalf("expected enemy_count>=2 to pass when EnemyCount=2")
	}
	if Evaluate(groups, EvalContext{EnemyCount: 1}) {
		t.Fatalf("expected enemy_count>=2 to fail when EnemyCount=1")
	}
}

func TestEvaluateEnemyCountUsesTargetAndPositionMetric(t *testing.T) {
	groups := []ConditionGroup{
		{
			Conditions: []Condition{
				{
					CompareType:         "value",
					DataType:            "enemy_count",
					Target:              "allies",
					EnemyCountPosition:  "rearguard",
					ValueCompareAction:  ">=",
					ValueConditionValue: 2,
				},
			},
		},
	}
	ctx := EvalContext{
		EnemyCount: 99,
		Metrics: map[string]float64{
			"enemy_count.allies.rearguard": 2,
		},
	}
	if !Evaluate(groups, ctx) {
		t.Fatalf("expected allies rearguard count condition to pass")
	}
}

func TestEvaluateAnyAllyHpPercent(t *testing.T) {
	groups := []ConditionGroup{{
		Conditions: []Condition{{
			CompareType:         "value",
			Target:              "allies",
			DataType:            "hp",
			ValueCompareAction:  "<=",
			ValueConditionValue: "70%",
		}},
	}}
	ctx := EvalContext{
		Self: &UnitContext{UnitID: "player:1", HP: 100, MaxHP: 100},
		Allies: []UnitContext{
			{UnitID: "player:1", HP: 100, MaxHP: 100},
			{UnitID: "player:2", HP: 60, MaxHP: 100},
		},
	}
	if !Evaluate(groups, ctx) {
		t.Fatalf("expected any ally hp<=70%% to pass when one ally is low")
	}
	ctx.Allies[1].HP = 80
	if Evaluate(groups, ctx) {
		t.Fatalf("expected any ally hp<=70%% to fail when every ally is above threshold")
	}
}

func TestEvaluateEachAllyHpPercent(t *testing.T) {
	groups := []ConditionGroup{{
		Conditions: []Condition{{
			CompareType:         "value",
			Target:              "eachAllies",
			DataType:            "hp",
			ValueCompareAction:  ">=",
			ValueConditionValue: "50%",
		}},
	}}
	ctx := EvalContext{
		Allies: []UnitContext{
			{UnitID: "player:1", HP: 50, MaxHP: 100},
			{UnitID: "player:2", HP: 80, MaxHP: 100},
		},
	}
	if !Evaluate(groups, ctx) {
		t.Fatalf("expected every ally hp>=50%% to pass")
	}
	ctx.Allies[0].HP = 49
	if Evaluate(groups, ctx) {
		t.Fatalf("expected every ally hp>=50%% to fail when one ally is below threshold")
	}
}

func TestEvaluateUnitValueTypes(t *testing.T) {
	ctx := EvalContext{
		PassedTurn: 3,
		Allies: []UnitContext{
			{
				UnitID: "player:1",
				HP:     70, MaxHP: 100,
				MP: 10, MaxMP: 40,
				StateStacks: map[string]float64{"regen": 1, "poison": 2},
				StateKinds:  map[string]string{"regen": "buff", "poison": "debuff"},
			},
			{UnitID: "player:2", HP: 40, MaxHP: 100, MP: 30, MaxMP: 40},
		},
	}
	cases := []Condition{
		{CompareType: "value", Target: "allies", DataType: "lostHp", ValueCompareAction: ">=", ValueConditionValue: "60%"},
		{CompareType: "value", Target: "allies", DataType: "lostMp", ValueCompareAction: ">=", ValueConditionValue: "75%"},
		{CompareType: "value", Target: "allies", DataType: "totalHp", ValueCompareAction: "<=", ValueConditionValue: "60%"},
		{CompareType: "value", Target: "allies", DataType: "count", ValueCompareAction: ">=", ValueConditionValue: 2},
		{CompareType: "value", Target: "allies", DataType: "buffCount", ValueCompareAction: ">=", ValueConditionValue: 1},
		{CompareType: "value", Target: "allies", DataType: "debuffCount", ValueCompareAction: ">=", ValueConditionValue: 1},
		{CompareType: "value", Target: "none", DataType: "passedTurn", ValueCompareAction: ">=", ValueConditionValue: 3},
	}
	for _, condition := range cases {
		if !Evaluate([]ConditionGroup{{Conditions: []Condition{condition}}}, ctx) {
			t.Fatalf("condition should pass: %#v", condition)
		}
	}
}

func TestEvaluateStateAndCooldownUseTargetUnits(t *testing.T) {
	ctx := EvalContext{
		Allies: []UnitContext{
			{
				UnitID:         "player:1",
				StateStacks:    map[string]float64{"regen": 2},
				SkillCooldowns: map[string]float64{"heal": 0},
				ItemCooldowns:  map[string]float64{"potion": 3},
			},
			{
				UnitID:         "player:2",
				StateStacks:    map[string]float64{},
				SkillCooldowns: map[string]float64{"heal": 2},
				ItemCooldowns:  map[string]float64{"potion": 0},
			},
		},
	}
	cases := []Condition{
		{CompareType: "state", Target: "allies", StateCompareAction: "someoneHasState", StateCompareStateID: "regen"},
		{CompareType: "stateStack", Target: "allies", StateCompareStateID: "regen", ValueCompareAction: ">=", ValueConditionValue: 2},
		{CompareType: "cooldown", Target: "allies", CooldownCompareAction: "someoneInCooldown", CooldownSkillIDs: []string{"heal"}},
		{CompareType: "itemCooldown", Target: "allies", CooldownCompareAction: "someoneInCooldown", CooldownItemIDs: []string{"potion"}},
	}
	for _, condition := range cases {
		if !Evaluate([]ConditionGroup{{Conditions: []Condition{condition}}}, ctx) {
			t.Fatalf("condition should pass: %#v", condition)
		}
	}
}

func TestEvaluateLegacySelfHPPercentStillWorks(t *testing.T) {
	groups := []ConditionGroup{{Conditions: []Condition{{
		CompareType:         "value",
		DataType:            "self_hp_percent",
		ValueCompareAction:  "<=",
		ValueConditionValue: "50%",
	}}}}
	if !Evaluate(groups, EvalContext{SelfHPPercent: 40}) {
		t.Fatalf("legacy self_hp_percent rule should still pass")
	}
	if Evaluate(groups, EvalContext{SelfHPPercent: 60}) {
		t.Fatalf("legacy self_hp_percent rule should fail above threshold")
	}
}

func TestEvaluateLegacyUnitExistWithoutTargetUsesAliveMap(t *testing.T) {
	groups := []ConditionGroup{{Conditions: []Condition{{
		CompareType:            "unitExistState",
		UnitExistCompareAction: "exist",
		UnitID:                 "enemy:1",
	}}}}
	ctx := EvalContext{
		Self:      &UnitContext{UnitID: "player:1"},
		UnitAlive: map[string]float64{"enemy:1": 1},
	}
	if !Evaluate(groups, ctx) {
		t.Fatalf("legacy unit existence rule without target should use UnitAlive")
	}
}

func TestEvaluateSpecialEnemyBossConditions(t *testing.T) {
	hasBoss := []ConditionGroup{{Conditions: []Condition{{
		CompareType:   "special",
		Target:        "enemies",
		SpecialAction: "enemyHasBoss",
	}}}}
	notHasBoss := []ConditionGroup{{Conditions: []Condition{{
		CompareType:   "special",
		Target:        "enemies",
		SpecialAction: "enemyNotHasBoss",
	}}}}

	ctx := EvalContext{Enemies: []UnitContext{{UnitID: "enemy:boss", IsBoss: true}}}
	if !Evaluate(hasBoss, ctx) {
		t.Fatalf("enemyHasBoss should pass when an enemy boss exists")
	}
	if Evaluate(notHasBoss, ctx) {
		t.Fatalf("enemyNotHasBoss should fail when an enemy boss exists")
	}

	ctx.Enemies = []UnitContext{{UnitID: "enemy:normal", IsBoss: false}}
	if Evaluate(hasBoss, ctx) {
		t.Fatalf("enemyHasBoss should fail when no enemy boss exists")
	}
	if !Evaluate(notHasBoss, ctx) {
		t.Fatalf("enemyNotHasBoss should pass when no enemy boss exists")
	}
}
