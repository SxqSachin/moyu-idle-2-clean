package httpapi

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func TestCustomSkillCompilesIntoIndependentBattleSnapshot(t *testing.T) {
	raw := map[string]any{
		"id":       "custom:runtime-test",
		"name":     "Runtime Test",
		"cooldown": 3,
		"customForge": map[string]any{
			"rulesetVersion": 3,
		},
		"effects": []any{map[string]any{
			"kind": "damage",
			"targeting": map[string]any{
				"targetScope":     "enemy",
				"targetPositions": []any{"vanguard"},
				"targetCount":     map[string]any{"base": 1},
			},
			"amountFormula": map[string]any{
				"baseAdd": 0,
				"terms": []any{map[string]any{
					"attrKey":     "strength",
					"coefficient": 2.5,
				}},
			},
		}},
	}
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{Skills: []map[string]any{{"id": "basic_attack"}}}}}
	custom := map[string]map[string]any{"custom:runtime-test": raw}

	resolved, ok := server.battleSkillRawByIDWithCustom("custom:runtime-test", custom)
	if !ok {
		t.Fatal("custom skill was not merged into the player battle catalog")
	}
	snapshot := server.battleSkillByRawWithProgress("custom:runtime-test", 7, 123, resolved)
	if snapshot.ID != "custom:runtime-test" || snapshot.Level != 7 || snapshot.Exp != 123 {
		t.Fatalf("custom progress was not preserved: %#v", snapshot)
	}
	if snapshot.RulesetVersion != 3 {
		t.Fatalf("custom ruleset version = %d, want 3", snapshot.RulesetVersion)
	}
	if len(snapshot.Effects) != 1 || len(snapshot.Effects[0].AmountFormula.Terms) != 1 || snapshot.Effects[0].AmountFormula.Terms[0].Coefficient != 2.5 {
		t.Fatalf("compiled custom DSL missing from battle snapshot: %#v", snapshot.Effects)
	}

	term := raw["effects"].([]any)[0].(map[string]any)["amountFormula"].(map[string]any)["terms"].([]any)[0].(map[string]any)
	term["coefficient"] = 99.0
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal battle snapshot: %v", err)
	}
	if snapshot.Effects[0].AmountFormula.Terms[0].Coefficient != 2.5 || len(encoded) == 0 {
		t.Fatal("an already-built battle snapshot changed with the accepted revision source")
	}
}

func TestDisabledCustomSkillRulesetIsBlockedAtBattleRuntime(t *testing.T) {
	server := &Server{deps: app.Dependencies{Config: config.Config{GlobalSettings: config.GlobalSettings{
		SkillForge: config.SkillForgeSettings{DisabledRulesetVersions: []int{1}},
	}}}}
	for _, skill := range []battleSkillState{
		{ID: "custom:explicit-v1", RulesetVersion: 1},
		{ID: "custom:legacy-snapshot"},
	} {
		if server.battleSkillRulesetEnabled(skill) {
			t.Fatalf("disabled custom skill remained enabled: %#v", skill)
		}
		state := &battleRuntimeState{}
		actor := &battleUnitState{UnitID: "player:1", Alive: true, HP: 100, MaxHP: 100}
		execution := server.executeBattleSkill(state, time.Unix(0, 0), actor, skill, "active", nil)
		if execution.Meta["blocked"] != true || execution.Meta["reason"] != "skill_forge_ruleset_disabled" {
			t.Fatalf("disabled ruleset execution was not safely blocked: %#v", execution.Meta)
		}
	}
	if !server.battleSkillRulesetEnabled(battleSkillState{ID: "basic_attack"}) {
		t.Fatal("official skills must not be affected by custom ruleset switches")
	}
}

func TestAcceptedCustomSkillCastUsesSnapshotWithoutStoreAccess(t *testing.T) {
	server := &Server{}
	state := &battleRuntimeState{
		Seed: 11,
		Players: []battleUnitState{{
			UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
			HP: 100, MaxHP: 100, MP: 20, MaxMP: 20, Stats: map[string]float64{"strength": 10},
		}},
		Enemies: []battleUnitState{{
			UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
			HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
		}},
	}
	skill := battleSkillState{
		ID: "custom:snapshot-only", Level: 1, RulesetVersion: 1,
		Effects: []battleSkillEffect{{
			Kind: "damage",
			Targeting: battleSkillTargeting{
				TargetScope: "enemy", TargetPositions: []string{"vanguard"}, SelectRule: "first",
				TargetCount: battleSkillTargetCount{Base: 1},
			},
			AmountFormula: battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "strength", Coefficient: 1}}},
		}},
	}

	execution := server.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
	if execution.Meta["blocked"] == true || len(execution.Effects) == 0 {
		t.Fatalf("snapshot custom skill did not execute: meta=%#v effects=%#v", execution.Meta, execution.Effects)
	}
	if state.Enemies[0].HP >= 100 {
		t.Fatalf("snapshot custom skill dealt no damage: hp=%v", state.Enemies[0].HP)
	}
}

func TestCustomSkillIsNotInStaticKittySkillCatalog(t *testing.T) {
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{Skills: []map[string]any{{"id": "basic_attack"}}}}}
	custom := map[string]map[string]any{"custom:runtime-test": {
		"id": "custom:runtime-test", "effects": []any{map[string]any{"kind": "damage"}},
	}}
	if _, ok := server.battleSkillRawByIDWithCustom("custom:runtime-test", custom); !ok {
		t.Fatal("player custom skill should resolve through the player overlay")
	}
	if server.battleSkillExists("custom:runtime-test") {
		t.Fatal("custom skill must not enter the static catalog used by kitty skill loading")
	}
}

func TestCustomSkillBattleBoundaryAcceptsTypedEffectSlices(t *testing.T) {
	server := &Server{}
	raw := map[string]any{
		"id":      "custom:typed-effects",
		"effects": []map[string]any{{"kind": "heal"}},
	}
	if !server.validAcceptedCustomBattleSkillRaw("custom:typed-effects", raw) {
		t.Fatal("typed effect slice should pass the same accepted-skill boundary as decoded JSON")
	}
}

func TestCustomSkillBattleBoundaryRejectsOrBoundsMalformedDefinitions(t *testing.T) {
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{Skills: []map[string]any{{"id": "basic_attack"}}}}}
	terms := make([]any, 0, 20)
	for idx := 0; idx < 20; idx++ {
		terms = append(terms, map[string]any{"attrKey": "strength", "coefficient": 1.0})
	}
	effect := map[string]any{
		"kind": "damage",
		"targeting": map[string]any{
			"targetPositions": []any{"vanguard", "midguard", "rearguard", "vanguard"},
			"targetCount":     map[string]any{"base": 99, "formula": terms},
		},
		"amountFormula": map[string]any{"terms": terms},
		"damageSpec":    map[string]any{"hitCount": map[string]any{"base": 99, "formula": terms}},
		"requiredTargetStatuses": []any{
			"status:1", "status:2", "status:3", "status:4", "status:5", "status:6", "status:7", "status:8", "status:9",
		},
	}
	raw := map[string]any{"id": "custom:bounded", "cooldown": 999, "effects": []any{effect, effect, effect}}
	resolved, ok := server.battleSkillRawByIDWithCustom("custom:bounded", map[string]map[string]any{"custom:bounded": raw})
	if !ok {
		t.Fatal("bounded custom definition should resolve")
	}
	snapshot := server.battleSkillByRawWithProgress("custom:bounded", 1, 0, resolved)
	if len(snapshot.Effects) != 3 || snapshot.CooldownTurns != 12 {
		t.Fatalf("custom skill runtime bounds missing: effects=%d cooldown=%d", len(snapshot.Effects), snapshot.CooldownTurns)
	}
	first := snapshot.Effects[0]
	if first.Targeting.TargetCount.Base != 8 || !first.Targeting.TargetCount.MaxSet || first.Targeting.TargetCount.Max != 8 {
		t.Fatalf("target count was not bounded: %#v", first.Targeting.TargetCount)
	}
	if len(first.AmountFormula.Terms) != 4 || first.DamageSpec.HitCount.Base != 8 || first.DamageSpec.HitCount.Max != 8 {
		t.Fatalf("formula or hit count was not bounded: %#v", first)
	}
	if len(first.Targeting.TargetPositions) != 3 || len(first.RequiredTargetStatuses) != 8 {
		t.Fatalf("targeting or status conditions were not bounded: %#v", first)
	}
	if got := battleResolveDamageHitCount(&battleUnitState{Stats: map[string]float64{"strength": 999}}, 1, first.DamageSpec); got != 8 {
		t.Fatalf("bounded hit count = %d, want 8", got)
	}

	invalidDefinitions := []map[string]any{
		{"id": "custom:other", "effects": []any{effect}},
		{"id": "custom:bounded", "effects": []any{map[string]any{"kind": "summon"}}},
		{"id": "custom:bounded", "effects": []any{effect, effect, effect, effect}},
	}
	for _, invalid := range invalidDefinitions {
		if _, ok := server.battleSkillRawByIDWithCustom("custom:bounded", map[string]map[string]any{"custom:bounded": invalid}); ok {
			t.Fatalf("invalid custom skill reached battle runtime: %#v", invalid)
		}
	}
}

func TestCustomSkillStatusApplyChanceSettlesInBattle(t *testing.T) {
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{Buff: []map[string]any{{"id": "armor_break", "durationTurns": 2}}}}}
	run := func(chance float64) bool {
		state := &battleRuntimeState{
			Seed: 7,
			Players: []battleUnitState{{
				UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
				HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
			}},
			Enemies: []battleUnitState{{
				UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
				HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
			}},
		}
		skill := battleSkillState{ID: "custom:status", Level: 1, Effects: []battleSkillEffect{{
			Kind: "status",
			Targeting: battleSkillTargeting{
				TargetPositions: []string{"vanguard"}, TargetScope: "enemy", SelectRule: "first",
				TargetCount: battleSkillTargetCount{Base: 1},
			},
			StatusPayload: battleSkillStatusPayload{
				StatusIDs: []string{"armor_break"}, DurationBaseTurns: 2, ApplyChanceBase: &chance,
			},
		}}}
		runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), server, nil)
		_ = server.executeBattleTemplateSkillEffects(state, time.Unix(0, 0), &state.Players[0], skill, runtime)
		return battleHasBuff(&state.Enemies[0], "armor_break")
	}

	if run(0) {
		t.Fatal("zero apply chance must not apply the status")
	}
	if !run(1) {
		t.Fatal("full apply chance must apply the status")
	}
}

func TestCustomSkillStatusChanceDSLIsWrittenIntoBattleSnapshot(t *testing.T) {
	effects := parseBattleSkillEffects([]any{map[string]any{
		"kind": "status",
		"statusPayload": map[string]any{
			"statusIds": []any{"armor_break"}, "durationBaseTurns": 2,
			"applyChanceBase": 0.35, "applyChancePerSkillLevel": 0.01,
			"applyChanceMin": 0.1, "applyChanceMax": 0.8,
		},
	}}, battleSkillTargeting{}, "physical")
	if len(effects) != 1 || effects[0].StatusPayload.ApplyChanceBase == nil {
		t.Fatalf("status chance DSL missing from snapshot: %#v", effects)
	}
	payload := effects[0].StatusPayload
	if *payload.ApplyChanceBase != 0.35 || payload.ApplyChancePerSkillLevel != 0.01 || payload.ApplyChanceMin == nil || *payload.ApplyChanceMin != 0.1 || payload.ApplyChanceMax == nil || *payload.ApplyChanceMax != 0.8 {
		t.Fatalf("status chance DSL mismatch: %#v", payload)
	}
}

func TestCustomSkillTargetGroupCanIncludeOrExcludeSelf(t *testing.T) {
	server := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:self", Side: "player", Position: "vanguard", Alive: true, HP: 100},
			{UnitID: "player:ally", Side: "player", Position: "midguard", Alive: true, HP: 100},
		},
		Enemies: []battleUnitState{{UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true, HP: 100}},
	}
	targeting := battleSkillTargeting{
		TargetScope: "ally", TargetPositions: []string{"vanguard", "midguard", "rearguard"},
		SelectRule: "random", TargetCount: battleSkillTargetCount{Base: 8},
	}
	withSelf := server.battleSelectTemplateTargets(state, &state.Players[0], battleSkillState{Level: 1}, targeting, false, func() float64 { return 0.5 })
	if len(withSelf) != 2 {
		t.Fatalf("all allies with self=%d want 2", len(withSelf))
	}
	targeting.ExcludeSelf = true
	withoutSelf := server.battleSelectTemplateTargets(state, &state.Players[0], battleSkillState{Level: 1}, targeting, false, func() float64 { return 0.5 })
	if len(withoutSelf) != 1 || withoutSelf[0].UnitID != "player:ally" {
		t.Fatalf("all allies without self=%#v", withoutSelf)
	}
}

func TestCustomSkillIfElseStatusBranchesFilterTargets(t *testing.T) {
	withArmorBreak := &battleUnitState{UnitID: "enemy:marked", Buffs: []battleBuffState{{BuffID: "armor_break", RemainTurns: 1}}}
	withoutArmorBreak := &battleUnitState{UnitID: "enemy:plain"}
	units := []*battleUnitState{withArmorBreak, withoutArmorBreak}
	trueBranch := battleFilterEffectStatusConditions(units, []string{"armor_break"}, nil)
	falseBranch := battleFilterEffectStatusConditions(units, nil, []string{"armor_break"})
	if len(trueBranch) != 1 || trueBranch[0].UnitID != "enemy:marked" {
		t.Fatalf("true branch targets=%#v", trueBranch)
	}
	if len(falseBranch) != 1 || falseBranch[0].UnitID != "enemy:plain" {
		t.Fatalf("false branch targets=%#v", falseBranch)
	}
	server := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{{UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true, HP: 100}},
		Enemies: []battleUnitState{
			{UnitID: "enemy:marked", Side: "enemy", Position: "vanguard", Alive: true, HP: 100, Buffs: withArmorBreak.Buffs},
			{UnitID: "enemy:plain", Side: "enemy", Position: "vanguard", Alive: true, HP: 100},
		},
	}
	targeting := battleSkillTargeting{
		TargetScope: "enemy", TargetPositions: []string{"vanguard"}, SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1},
		TargetPredicates: []battleSkillTargetPredicate{{Kind: "exclude_target_has_status", StatusID: "armor_break"}},
	}
	selected := server.battleSelectTemplateTargets(state, &state.Players[0], battleSkillState{Level: 1}, targeting, false, func() float64 { return 0.5 })
	if len(selected) != 1 || selected[0].UnitID != "enemy:plain" {
		t.Fatalf("target selector must filter conditions before limiting target count: %#v", selected)
	}

	effects := parseBattleSkillEffects([]any{map[string]any{
		"kind": "damage", "forbiddenTargetStatuses": []any{"armor_break"},
	}}, battleSkillTargeting{}, "physical")
	if len(effects) != 1 || len(effects[0].ForbiddenTargetStatuses) != 1 || effects[0].ForbiddenTargetStatuses[0] != "armor_break" {
		t.Fatalf("forbidden status DSL missing from battle snapshot: %#v", effects)
	}
}

func TestCustomSkillVirtualFormulaAttributesAndSelfCurrentHPLoss(t *testing.T) {
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
		HP: 80, MaxHP: 100, MP: 30, MaxMP: 40,
		Stats: map[string]float64{"intelligence": 20, "perception": 12},
	}
	formula := battleSkillAmountFormula{Terms: []battleAttackTerm{
		{AttrKey: "skill_level", Coefficient: 0.1},
		{AttrKey: "intelligence", Coefficient: 0.01},
	}}
	if got := battleEvaluateAmountFormula(&actor, 5, formula); got != 0.7 {
		t.Fatalf("skill level + intelligence formula=%v, want 0.7", got)
	}

	state := &battleRuntimeState{Players: []battleUnitState{actor}}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.Unix(0, 0), &Server{}, nil)
	lossFormula := battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "current_hp", Coefficient: -0.3}}}
	loss := math.Floor(battleEvaluateAmountFormula(&state.Players[0], 5, lossFormula))
	runtime.ApplyResource(battleskills.ResourceAction{
		SourceID: state.Players[0].UnitID,
		TargetID: state.Players[0].UnitID,
		Resource: "hp",
		Amount:   loss,
		Reason:   "skill_forge_self_current_hp_loss",
	})
	if state.Players[0].HP != 56 {
		t.Fatalf("self current HP loss left hp=%v, want 56", state.Players[0].HP)
	}
}
