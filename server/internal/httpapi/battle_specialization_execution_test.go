package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func newSpecializationExecutionTestServer() *Server {
	s := &Server{}
	s.deps.Config.SharedGlobalSettings = config.DefaultSharedGlobalSettings()
	s.deps.GameData.Buff = []map[string]any{
		{"id": "test_positive_buff", "kind": "buff", "dispellable": true, "durationTurns": 3},
		{"id": "specialization_critical_exposure", "kind": "debuff", "dispellable": true, "durationTurns": 6, "stackPolicy": "stack", "maxStacks": 6},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "specialization_critical_exposure_while_active",
			"buffId":  "specialization_critical_exposure",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "add_stat", "attr": "crit_damage_resist", "value": -0.05}},
		},
	}
	return s
}

func specializationTestUnit(id, side string, hp, mp float64) battleUnitState {
	unit := battleUnitState{
		UnitID: id, Side: side, OriginSide: side, BaseSide: side, PerspectiveSide: side,
		Position: "vanguard", BasePosition: "vanguard", Alive: hp > 0,
		HP: hp, MaxHP: 100, MP: mp, MaxMP: 100,
		Stats: map[string]float64{
			"hp": 100, "mp": 100, "strength": 20, "dexterity": 20, "intelligence": 20, "stamina": 20, "perception": 20,
			"attack": 20, "magic_attack": 20, "physical_defence": 100, "magical_defence": 100,
		},
	}
	normalizeBattleUnitState(&unit)
	return unit
}

func specializationTestHook(id, trigger, sourceID string, effects ...map[string]any) battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID: id, SourceKind: "specialization", SourceID: sourceID,
		SourceNameKey: "specialization.node." + sourceID + ".name", Trigger: trigger, Effects: effects,
	}
}

func specializationTestValueFormula(base float64, terms map[string]float64) map[string]any {
	rows := make([]any, 0, len(terms))
	for attributeKey, coefficient := range terms {
		rows = append(rows, map[string]any{"attributeKey": attributeKey, "coefficient": coefficient})
	}
	return map[string]any{"baseAdd": base, "terms": rows}
}

func specializationTestSourceContribution(t *testing.T, nodeIDs ...string) store.SpecializationBattleContribution {
	t.Helper()
	raw, err := os.ReadFile("../../../packages/game-data/source/json/specialization_trees.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Trees []catalog.SpecializationTree `json:"specialization_trees"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	wanted := make(map[string]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		wanted[nodeID] = struct{}{}
	}
	var contribution store.SpecializationBattleContribution
	for _, tree := range source.Trees {
		contribution.TreeID = tree.ID
		contribution.RulesetVersion = tree.RulesetVersion
		for _, node := range tree.Nodes {
			if len(wanted) > 0 {
				if _, ok := wanted[node.ID]; !ok {
					continue
				}
			}
			if len(node.Levels) == 0 {
				continue
			}
			// levels[] are complete rank snapshots. Exercise each node at max
			// rank without duplicating the same stable Hook/Mutation IDs.
			level := node.Levels[len(node.Levels)-1]
			for _, grant := range level.Grants {
				for _, hook := range grant.BattleHooks {
					payloadID := strings.TrimSpace(fmt.Sprintf("%v", hook["id"]))
					contribution.Hooks = append(contribution.Hooks, store.SpecializationRuntimeGrant{
						ID: strings.Join([]string{tree.ID, node.ID, grant.ID, payloadID}, ":"), SourceNodeID: node.ID, SourceNameKey: node.NameKey, Payload: hook,
					})
				}
				for _, mutation := range grant.EffectMutations {
					payloadID := strings.TrimSpace(fmt.Sprintf("%v", mutation["id"]))
					contribution.Mutations = append(contribution.Mutations, store.SpecializationRuntimeGrant{
						ID: strings.Join([]string{tree.ID, node.ID, grant.ID, payloadID}, ":"), SourceNodeID: node.ID, SourceNameKey: node.NameKey, Payload: mutation,
					})
				}
			}
		}
	}
	return contribution
}

func specializationTestSourceHooks(t *testing.T, nodeIDs ...string) []battleEquipmentHookRule {
	t.Helper()
	s := newSpecializationExecutionTestServer()
	hooks, err := s.resolveBattleSpecializationHooks(specializationTestSourceContribution(t, nodeIDs...))
	if err != nil {
		t.Fatal(err)
	}
	sort.SliceStable(hooks, func(i, j int) bool {
		if hooks[i].Priority != hooks[j].Priority {
			return hooks[i].Priority > hooks[j].Priority
		}
		return hooks[i].ID < hooks[j].ID
	})
	return hooks
}

func specializationTestSourceHook(t *testing.T, definitionID string) battleEquipmentHookRule {
	t.Helper()
	for _, hook := range specializationTestSourceHooks(t) {
		if strings.HasSuffix(hook.ID, ":"+definitionID) {
			return hook
		}
	}
	t.Fatalf("source specialization hook %q not found", definitionID)
	return battleEquipmentHookRule{}
}

func TestBattleSpecializationConditionsExecuteFailClosed(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	state := &battleRuntimeState{
		Wave: 1,
		Players: []battleUnitState{
			specializationTestUnit("player:self", "player", 50, 60),
			specializationTestUnit("player:low", "player", 20, 10),
		},
		Enemies: []battleUnitState{specializationTestUnit("enemy:source", "enemy", 80, 20)},
	}
	self, low, enemy := &state.Players[0], &state.Players[1], &state.Enemies[0]
	self.Position = "midguard"
	self.WeaponClasses = []string{"bow"}
	low.Position = "vanguard"
	battleSetSpecializationCounter(self, "tempo", 2)
	battleApplyOrRefreshBuff(enemy, battleSpecializationMarkBuffID("exposed"), 2, nil, self.UnitID, "refresh_duration")
	context := &battleEquipmentHookContext{SkillID: "skill:non_basic", HitCount: 2, MPInsufficient: true}

	cases := []struct {
		name      string
		condition map[string]any
		source    *battleUnitState
		target    *battleUnitState
		damage    float64
		damageTyp string
	}{
		{"adjacent ally", map[string]any{"kind": "adjacent_ally_exists"}, enemy, low, 0, ""},
		{"damage type", map[string]any{"kind": "context_damage_type", "value": "physical"}, enemy, enemy, 0, "physical"},
		{"hit count", map[string]any{"kind": "context_hit_count_gte", "value": 2}, enemy, enemy, 0, ""},
		{"mp insufficient", map[string]any{"kind": "context_mp_insufficient"}, enemy, enemy, 0, ""},
		{"non basic", map[string]any{"kind": "context_skill_non_basic"}, enemy, enemy, 0, ""},
		{"counter", map[string]any{"kind": "counter_gte", "counterId": "tempo", "value": 2}, enemy, enemy, 0, ""},
		{"lethal", map[string]any{"kind": "incoming_damage_would_be_lethal"}, enemy, self, 50, "physical"},
		{"hp ratio", map[string]any{"kind": "self_hp_ratio_lte", "value": .5}, enemy, self, 0, ""},
		{"mp ratio", map[string]any{"kind": "self_mp_ratio_gte", "value": .6}, enemy, self, 0, ""},
		{"weapon class", map[string]any{"kind": "self_weapon_class", "weaponClasses": []any{"bow", "crossbow"}}, enemy, self, 0, ""},
		{"enemy source", map[string]any{"kind": "source_is_enemy"}, enemy, self, 0, ""},
		{"exact mark", map[string]any{"kind": "target_has_mark", "markId": "exposed"}, self, enemy, 0, ""},
		{"any mark", map[string]any{"kind": "target_has_any_specialization_mark"}, self, enemy, 0, ""},
		{"lowest ally", map[string]any{"kind": "target_is_lowest_hp_ally"}, enemy, low, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !s.battleEvaluateEquipmentHookCondition(state, tc.condition, self, tc.source, tc.target, tc.damage, tc.damageTyp, 0, context) {
				t.Fatalf("condition did not pass: %#v", tc.condition)
			}
		})
	}
	if battleSpecializationConditionSupported("future_unknown_condition") {
		t.Fatal("unknown specialization condition was accepted by strict resolver")
	}
	self.SpecializationHooks = []battleEquipmentHookRule{
		{ID: "invalid", SourceKind: "specialization", SourceID: "invalid_node", Trigger: "onTurnEnd", Conditions: []map[string]any{{"kind": "future_unknown_condition"}}, Effects: []map[string]any{{"kind": "heal", "maxHpRatio": .5}}},
	}
	before := self.HP
	rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), self, enemy, self, "onTurnEnd", 0, "", context)
	if self.HP != before || len(rows) != 1 || rows[0]["kind"] != "specialization_definition_invalid" {
		t.Fatalf("unknown condition did not fail closed: hp=%v rows=%#v", self.HP, rows)
	}
}

func TestBattleSpecializationPreDamagePrimitivesSettle(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	target := specializationTestUnit("player:target", "player", 100, 50)
	protector := specializationTestUnit("player:protector", "player", 100, 0)
	source := specializationTestUnit("enemy:source", "enemy", 100, 0)
	redirectHook := specializationTestHook("sentinel", "onAllyBeforeTakeDamage", "sentinel_node",
		map[string]any{"kind": "redirect_damage", "targetMode": "self", "ratio": .2},
		map[string]any{"kind": "mark_target", "targetMode": "source", "markId": "sentinel"})
	redirectHook.DispatchHooks = true
	protector.SpecializationHooks = []battleEquipmentHookRule{
		redirectHook,
		specializationTestHook("guard_cycle", "onAfterTakeDamage", "guard_node", map[string]any{"kind": "cycle_counter", "counterId": "guard", "gain": 1, "maxStacks": 3}),
	}
	target.SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("ward", "onBeforeTakeDamage", "ward_node", map[string]any{"kind": "resource_shield", "resource": "mp", "absorbRatio": .3, "resourcePerDamage": .5}),
		specializationTestHook("last_line", "onBeforeTakeDamage", "last_line_node",
			map[string]any{"kind": "prevent_lethal", "remainingHpRatio": .15},
			map[string]any{"kind": "heal", "targetMode": "self", "maxHpRatio": .1}),
	}
	battleApplyOrRefreshBuff(&target, "specialization:damage_reduction:guard", 1, map[string]any{
		"value": .1, "sourceKind": "specialization", "sourceId": "guard_node", "nodeId": "guard_node", "sourceNameKey": "specialization.node.guard.name",
	}, protector.UnitID, "refresh_duration")
	state := &battleRuntimeState{Players: []battleUnitState{target, protector}, Enemies: []battleUnitState{source}}
	result := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], 200, "physical", &battleEquipmentHookContext{})
	if math.Abs(result.Amount-85) > .001 || math.Abs(result.DeferredHeal-10) > .001 {
		t.Fatalf("adjusted amount=%v deferredHeal=%v, want 85 and 10", result.Amount, result.DeferredHeal)
	}
	if state.Players[1].HP != 60 || math.Abs(state.Players[0].MP-28.4) > .001 {
		t.Fatalf("redirect/shield did not settle: protectorHP=%v targetMP=%v", state.Players[1].HP, state.Players[0].MP)
	}
	if battleSpecializationCounterStacks(&state.Players[1], "guard") != 1 {
		t.Fatalf("redirected damage did not dispatch protector take-damage hooks: %#v", state.Players[1].Buffs)
	}
	if !battleHasBuff(&state.Enemies[0], battleSpecializationMarkBuffID("sentinel")) {
		t.Fatal("ally interception mark did not settle on the damage source")
	}
	for _, row := range result.Actions {
		meta := normalizeAnyMap(row["meta"])
		if parseBooleanLoose(meta["specialization"], false) && row["kind"] != "damage_reduction" && stringsTrim(meta["sourceNameKey"]) == "" {
			t.Fatalf("specialization action lost source visibility: %#v", row)
		}
	}
}

func TestBattleSpecializationDeferredHealKeepsNodeSource(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	target := specializationTestUnit("player:target", "player", 100, 0)
	source := specializationTestUnit("enemy:source", "enemy", 100, 0)
	hook := specializationTestHook("last_line", "onBeforeTakeDamage", "last_line_node",
		map[string]any{"kind": "prevent_lethal", "remainingHpRatio": .15},
		map[string]any{"kind": "heal", "targetMode": "self", "maxHpRatio": .1})
	hook.Conditions = []map[string]any{{"kind": "incoming_damage_would_be_lethal"}}
	target.SpecializationHooks = []battleEquipmentHookRule{hook}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{target}, Enemies: []battleUnitState{source}}
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{SourceID: source.UnitID, TargetID: target.UnitID, Amount: 200, DamageType: "physical"})
	found := false
	for _, row := range runtime.ActionLog() {
		if row["reason"] != "specialization_prevent_lethal_recovery" {
			continue
		}
		meta := normalizeAnyMap(row["meta"])
		if meta["sourceKind"] == "specialization" && meta["nodeId"] == "last_line_node" && meta["sourceNameKey"] != "" && meta["hookId"] == "last_line" {
			found = true
		}
	}
	if !found {
		t.Fatalf("deferred heal lost its specialization node source: %#v", runtime.ActionLog())
	}
}

func stringsTrim(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(value.(string))
}

func TestBattleSpecializationRuntimeAmpPenetrationAndSource(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	battleSetSpecializationCounter(&actor, "momentum", 2)
	actor.SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("pressure", "onBeforeDealDamage", "pressure_node",
			map[string]any{"kind": "runtime_damage_amp", "value": .9, "valueFormula": specializationTestValueFormula(.1, map[string]float64{"strength": .002, "perception": .002})},
			map[string]any{"kind": "flat_defense_penetration", "valueFormula": specializationTestValueFormula(0, map[string]float64{"perception": .35})}),
		specializationTestHook("spend", "onBeforeDealDamage", "spend_node",
			map[string]any{"kind": "consume_counter_for_damage_amp", "counterId": "momentum", "consume": 2, "value": .9, "percentDefensePenetration": .3, "valueFormula": specializationTestValueFormula(.06, map[string]float64{"strength": .003})}),
	}
	state := &battleRuntimeState{Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	context := &battleEquipmentHookContext{RawDefense: 100, FlatPenetration: 0, PercentPenetration: .8}
	expectedFlatAmp := s.battleEquivalentRuntimePenetrationAmp(state, &state.Players[0], &state.Enemies[0], "physical", 7, 0, context)
	amp, actions, sources := s.battleCollectRuntimeDamageAmpFromEquipmentHooksWithContextForTest(state, &state.Players[0], &state.Enemies[0], context)
	if math.Abs(amp-(.30+expectedFlatAmp)) > .001 {
		t.Fatalf("amp=%v, want %v (runtime + flat pen + counter; capped percent adds zero)", amp, .30+expectedFlatAmp)
	}
	if battleSpecializationCounterStacks(&state.Players[0], "momentum") != 0 {
		t.Fatal("counter was not consumed")
	}
	if len(actions) < 3 || len(sources) != 1 || sources[0]["kind"] != "specialization" || sources[0]["nodeId"] != "pressure_node" {
		t.Fatalf("source/action projection mismatch actions=%#v sources=%#v", actions, sources)
	}
	if extra := s.battleEquivalentRuntimePenetrationAmp(state, &state.Players[0], &state.Enemies[0], "physical", 0, .3, context); extra != 0 {
		t.Fatalf("penetration above the existing cap must add no damage, got %v", extra)
	}
}

func (s *Server) battleCollectRuntimeDamageAmpFromEquipmentHooksWithContextForTest(state *battleRuntimeState, self, target *battleUnitState, context *battleEquipmentHookContext) (float64, []map[string]any, []map[string]any) {
	total, actions, sources, pending := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), self, target, "on_before_deal_damage", 100, "physical", context)
	actions = append(actions, s.battleFinalizePendingProbabilityOutcomes(state, time.UnixMilli(1), pending)...)
	return total, actions, sources
}

func TestBattleSpecializationSourceCatalogStrictlyResolves(t *testing.T) {
	contribution := specializationTestSourceContribution(t)
	s := newSpecializationExecutionTestServer()
	hooks, err := s.resolveBattleSpecializationHooks(contribution)
	if err != nil {
		t.Fatal(err)
	}
	mutations, err := s.resolveBattleSpecializationEffectMutations(contribution)
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != len(contribution.Hooks) || len(mutations) != len(contribution.Mutations) {
		t.Fatalf("resolved hooks=%d/%d mutations=%d/%d", len(hooks), len(contribution.Hooks), len(mutations), len(contribution.Mutations))
	}
}

func TestBattleSpecializationEverySourceMutationSettlesObservably(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	contribution := specializationTestSourceContribution(t)
	mutations, err := s.resolveBattleSpecializationEffectMutations(contribution)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != len(contribution.Mutations) {
		t.Fatalf("resolved source mutations=%d, want source-derived %d", len(mutations), len(contribution.Mutations))
	}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	baseEffect := battleSkillEffect{
		ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{BaseAdd: 20},
		DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}},
	}
	settled := map[string]bool{}
	payloadIDsByGrant := make(map[string]string, len(contribution.Mutations))
	for _, grant := range contribution.Mutations {
		payloadIDsByGrant[grant.ID] = strings.TrimSpace(fmt.Sprintf("%v", grant.Payload["id"]))
	}
	for _, mutation := range mutations {
		payloadID := payloadIDsByGrant[mutation.ID]
		actor := specializationTestUnit("player:actor", "player", 100, 100)
		actor.SpecializationEffectMutations = []battleAffixEffectMutation{mutation}
		actor.WeaponClasses = append([]string(nil), mutation.WeaponClassList...)
		category := "attack"
		if payloadID == "quick_formula_weapon_cooldown" {
			category = "arcane"
		} else if payloadID == "longsword_divided_arc" {
			category = "blade"
		}
		actor.Skills = []battleSkillState{{ID: "skill:test", Category: category, CooldownTurns: 3, MPCost: 4, Effects: []battleSkillEffect{baseEffect}}}
		origin := "skill"
		if mutation.Scope == "basic_attack" {
			origin = "basic_attack"
		}
		skill := actor.applyBattleSkillScalarMutations(origin, actor.Skills[0])
		effects := s.applyBattleAffixEffectMutations(&actor, origin, skill.ID, 1, targeting, []battleSkillEffect{baseEffect})
		switch payloadID {
		case "strength_blood_forge_price":
			settled[payloadID] = len(effects) == 2 && effects[0].AmountFormula.BaseAdd > 20 && effects[1].ID == "strength_blood_forge_hp_cost" && effects[1].GeneratedBy == mutation.ID
		case "dexterity_first_effect_extra_target":
			settled[payloadID] = effects[0].DamageSpec.HitCount.Base == 2 && len(effects[0].AmountFormula.Terms) == 1
		case "dexterity_reaping_sweep":
			settled[payloadID] = len(effects) == 2 && effects[1].ID == "dexterity_reaping_sweep_effect" && effects[1].GeneratedBy == mutation.ID && effects[1].TargetRowFromEffect == 1 && reflect.DeepEqual(effects[1].ExcludeTargetsFromEffects, []int{1})
		case "perception_spear_rear_thrust":
			settled[payloadID] = len(effects) == 2 && effects[1].ID == "perception_spear_rear_thrust_effect" && effects[1].GeneratedBy == mutation.ID && reflect.DeepEqual(effects[1].Targeting.TargetPositions, []string{"rearguard"}) && effects[1].Targeting.SecondarySeek == "none" && reflect.DeepEqual(effects[1].ExcludeTargetsFromEffects, []int{1})
		case "longsword_divided_arc":
			settled[payloadID] = len(effects) == 2 && effects[1].ID == "longsword_divided_arc_effect" && effects[1].GeneratedBy == mutation.ID && reflect.DeepEqual(effects[1].Targeting.TargetPositions, []string{"vanguard", "midguard", "rearguard"}) && effects[1].Targeting.SecondarySeek == "front_to_rear" && reflect.DeepEqual(effects[1].ExcludeTargetsFromEffects, []int{1})
		case "intelligence_staff_cooldown_ledger":
			settled[payloadID] = skill.CooldownTurns == 2 && skill.MPCost == 12 && len(effects[0].MutationSources) == 1
		case "perception_reveal_on_damage":
			settled[payloadID] = len(effects) == 2 && effects[1].ID == "perception_reveal_mark_effect" && effects[1].GeneratedBy == mutation.ID && effects[1].Kind == "status"
		case "mana_edge_dual_scaling":
			settled[payloadID] = mutation.AttackStatPolicy == "higher" && effects[0].DamageTypePolicy == "" && effects[0].DamageType == "physical" &&
				math.Abs(effects[0].AmountFormula.BaseAdd-21.2) < 1e-9 &&
				len(effects[0].MutationSources) == 1
		case "quick_formula_weapon_cooldown":
			unmatched := actor.Skills[0]
			unmatched.Category = "blade"
			unmatched = actor.applyBattleSkillScalarMutations(origin, unmatched)
			settled[payloadID] = skill.CooldownTurns == 2 && skill.MPCost == 12 && len(effects[0].MutationSources) == 1 &&
				unmatched.CooldownTurns == 3 && unmatched.MPCost == 4
		case "dexterity_rapid_basic_double_hit":
			settled[payloadID] = effects[0].DamageSpec.HitCount.Base == 2 && len(effects[0].AmountFormula.Terms) == 1 && effects[0].AmountFormula.Terms[0].AttrKey == "dexterity"
		case "dexterity_afterimage_basic_splash":
			settled[payloadID] = len(effects) == 2 && effects[1].ID == "dexterity_afterimage_splash_effect" && effects[1].GeneratedBy == mutation.ID
		case "intelligence_archmage_extra_target":
			settled[payloadID] = effects[0].Targeting.TargetCount.Base == 2 && len(effects[0].AmountFormula.Terms) == 1 && effects[0].AmountFormula.Terms[0].AttrKey == "intelligence"
		case "mana_weapon_double_cast_hit":
			settled[payloadID] = skill.MPCost == 5 && effects[0].DamageSpec.HitCount.Base == 2 && len(effects[0].AmountFormula.Terms) == 2
		case "mind_lock_low_hp_targeting":
			settled[payloadID] = effects[0].Targeting.SelectRule == "hp_percent_min" &&
				effects[0].Targeting.SecondarySeek == "front_to_rear" &&
				math.Abs(effects[0].AmountFormula.BaseAdd-21.2) < 1e-9 &&
				len(effects[0].MutationSources) == 1
		default:
			t.Fatalf("source mutation %q is missing an execution scenario", payloadID)
		}
		if !settled[payloadID] {
			t.Fatalf("source mutation %q produced no authoritative observable change: skill=%#v effects=%#v", payloadID, skill, effects)
		}
	}
	if len(settled) != len(mutations) {
		t.Fatalf("settled source mutations=%d, want %d", len(settled), len(mutations))
	}
}

func TestBattleSpecializationSourceStrengthThirdHitUsesExecutioner(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.SpecializationHooks = specializationTestSourceHooks(t, "strength_root_break_stance", "strength_terminal_executioner")
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	target.HP, target.MaxHP = 10_000, 10_000
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}

	for hit := 1; hit <= 3; hit++ {
		runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(int64(hit)), s, nil)
		runtime.skillID = "skill:physical"
		runtime.ApplyDamage(battleskills.DamageAction{
			SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 100, DamageType: "physical",
		})
		wantStacks := hit
		if hit == 3 {
			wantStacks = 1
			seenExecutioner := false
			seenRootRelease := false
			for _, row := range runtime.ActionLog() {
				meta := normalizeAnyMap(row["meta"])
				switch meta["nodeId"] {
				case "strength_terminal_executioner":
					seenExecutioner = true
				case "strength_root_break_stance":
					if strings.Contains(fmt.Sprintf("%v", meta["hookId"]), "strength_break_momentum_release") {
						seenRootRelease = true
					}
				}
			}
			if !seenExecutioner || seenRootRelease {
				t.Fatalf("third hit did not let the higher-priority executioner replace root release: %#v", runtime.ActionLog())
			}
		}
		if got := battleSpecializationCounterStacks(&state.Players[0], "strength_break_momentum"); got != wantStacks {
			t.Fatalf("hit %d momentum=%d, want %d; actions=%#v", hit, got, wantStacks, runtime.ActionLog())
		}
	}
}

func TestBattleSpecializationSourceRedirectsDispatchRealGuardCycle(t *testing.T) {
	for _, tc := range []struct {
		name          string
		interceptNode string
		wantMark      bool
	}{
		{name: "shared guard", interceptNode: "stamina_tech_shared_guard"},
		{name: "sentinel link", interceptNode: "confluence_perception_stamina_sentinel_link", wantMark: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSpecializationExecutionTestServer()
			protector := specializationTestUnit("player:protector", "player", 100, 0)
			protector.SpecializationHooks = specializationTestSourceHooks(t, "stamina_root_hold_line", tc.interceptNode)
			ally := specializationTestUnit("player:ally", "player", 50, 0)
			enemy := specializationTestUnit("enemy:source", "enemy", 100, 0)
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{ally, protector}, Enemies: []battleUnitState{enemy}}
			runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)
			runtime.skillID = "skill:physical"
			runtime.ApplyDamage(battleskills.DamageAction{
				SourceID: enemy.UnitID, TargetID: ally.UnitID, Amount: 20, DamageType: "physical",
			})
			if state.Players[1].HP >= 100 || battleSpecializationCounterStacks(&state.Players[1], "stamina_guard") != 1 {
				t.Fatalf("real redirect did not flow through protector onAfterTakeDamage: protector=%#v actions=%#v", state.Players[1], runtime.ActionLog())
			}
			marked := battleHasBuff(&state.Enemies[0], "specialization_sentinel_target")
			if marked != tc.wantMark {
				t.Fatalf("sentinel source mark=%v, want %v; enemy=%#v", marked, tc.wantMark, state.Enemies[0])
			}
		})
	}
}

func TestBattleSpecializationEverySourceHookSettlesObservably(t *testing.T) {
	contribution := specializationTestSourceContribution(t)
	hooks := specializationTestSourceHooks(t)
	if len(hooks) != len(contribution.Hooks) {
		t.Fatalf("source hook count=%d, want source-derived %d", len(hooks), len(contribution.Hooks))
	}
	settledIDs := make(map[string]struct{}, len(hooks))
	for _, sourceHook := range hooks {
		hook := sourceHook
		separator := strings.LastIndex(hook.ID, ":")
		definitionID := hook.ID
		if separator >= 0 {
			definitionID = hook.ID[separator+1:]
		}
		t.Run(definitionID, func(t *testing.T) {
			s := newSpecializationExecutionTestServer()
			s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
			s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}, {"id": "skill:test"}}
			targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
			skill := battleSkillState{
				ID: "skill:test", Name: "test", Category: "attack", Level: 1, CooldownTurns: 3, CooldownRemain: 2, Targeting: targeting,
				Effects: []battleSkillEffect{{ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting, AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, HitCount: battleSkillTargetCount{Base: 1}}}},
			}
			self := specializationTestUnit("player:self", "player", 50, 50)
			self.Position = "midguard"
			self.Skills = []battleSkillState{skill}
			self.SkillIDs = []string{skill.ID}
			self.SkillSlots = []string{skill.ID}
			self.SpecializationHooks = []battleEquipmentHookRule{hook}
			ally := specializationTestUnit("player:ally", "player", 20, 0)
			ally.Position = "vanguard"
			enemy := specializationTestUnit("enemy:source", "enemy", 100, 0)
			enemy.HP, enemy.MaxHP = 1_000, 1_000
			enemy.Skills = []battleSkillState{skill}
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self, ally}, Enemies: []battleUnitState{enemy}}
			selfPtr, allyPtr, enemyPtr := &state.Players[0], &state.Players[1], &state.Enemies[0]
			context := &battleEquipmentHookContext{SkillID: skill.ID, SkillCategory: skill.Category, CastKind: "skill", DamageType: "physical", HitCount: 2}
			selfPtr.MP = selfPtr.MaxMP
			for _, condition := range hook.Conditions {
				switch battleNormalizeTrigger(fmt.Sprintf("%v", condition["kind"])) {
				case "selfweaponclass":
					classes := compactStringSlice(condition["weaponClasses"])
					if len(classes) > 0 {
						selfPtr.WeaponClasses = []string{classes[0]}
					}
				case "contextdamagetype":
					context.DamageType = strings.TrimSpace(fmt.Sprintf("%v", condition["value"]))
				case "targethasbuff":
					buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
					if buffID != "" {
						battleApplyOrRefreshBuff(enemyPtr, buffID, 2, nil, selfPtr.UnitID, "refresh_duration")
					}
				case "targethasanybuff":
					buffIDs := compactStringSlice(condition["buffIds"])
					if len(buffIDs) > 0 {
						battleApplyOrRefreshBuff(enemyPtr, buffIDs[0], 2, nil, selfPtr.UnitID, "refresh_duration")
					}
				}
			}
			// This catalog-wide test proves the effect settles when its roll wins;
			// dedicated probability tests below cover the authored 0-40% chance.
			for _, effect := range hook.Effects {
				if battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"])) == "extraturn" {
					effect["procChanceBase"] = 1.0
					effect["procChanceMax"] = 1.0
				}
			}
			settled := false

			switch definitionID {
			case "strength_break_momentum_cycle":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && battleSpecializationCounterStacks(selfPtr, "strength_break_momentum") == 1
			case "strength_break_momentum_release", "strength_heavy_momentum_spend", "strength_mountain_breaker":
				battleSetSpecializationCounter(selfPtr, "strength_break_momentum", 2)
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "physical", &battleEquipmentHookContext{DamageType: "physical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0 && battleSpecializationCounterStacks(selfPtr, "strength_break_momentum") == 0
			case "dexterity_tempo_cycle":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, enemyPtr, "onAfterSkillCast", 0, "", context)
				settled = len(rows) > 0 && battleSpecializationCounterStacks(selfPtr, "dexterity_tempo") == 1
			case "dexterity_stolen_beat_turn":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onKill", 10, "physical", context)
				for _, row := range rows {
					settled = settled || row["kind"] == "extra_turn"
				}
			case "intelligence_pattern_cycle":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, enemyPtr, "onAfterSkillCast", 0, "", context)
				settled = len(rows) > 0 && battleSpecializationCounterStacks(selfPtr, "intelligence_pattern_weaving") == 1
			case "intelligence_mana_lattice_flow":
				selfPtr.MP = 10
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && selfPtr.MP > 10
			case "intelligence_first_echo_cast":
				selfPtr.Skills[0].CooldownRemain = 0
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, enemyPtr, "onAfterSkillCast", 0, "", context)
				settled = len(rows) > 0 && enemyPtr.HP < before && selfPtr.MP == selfPtr.MaxMP
			case "intelligence_resource_debt_cast":
				selfPtr.MP = 0
				debtContext := &battleEquipmentHookContext{SkillID: skill.ID, SkillCategory: skill.Category, CastKind: "skill", MPCost: 10, MPInsufficient: true}
				before := selfPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, enemyPtr, "onBeforeSkillCast", 0, "", debtContext)
				settled = len(rows) > 0 && debtContext.AllowDebtCast && selfPtr.HP < before
			case "perception_observation_cycle":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "perception_observed_gap")
			case "perception_mind_orb_critical_exposure":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && battleCountBuff(enemyPtr, "specialization_critical_exposure") == 1 && math.Abs(enemyPtr.Stats["crit_damage_resist"]+0.05) < 0.0001
			case "perception_designate_priority_target":
				rows := s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(1))
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "specialization_hunter_mark")
			case "perception_consume_exposed":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				battleApplyOrRefreshBuff(enemyPtr, "test_positive_buff", 3, nil, enemyPtr.UnitID, "refresh_duration")
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && !battleHasBuff(enemyPtr, "specialization_exposed") && !battleHasBuff(enemyPtr, "test_positive_buff")
			case "stamina_guard_cycle":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && battleSpecializationCounterStacks(selfPtr, "stamina_guard") == 1
			case "stamina_shared_guard_intercept":
				before := selfPtr.HP
				result := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), allyPtr, enemyPtr, 20, "physical", context)
				settled = len(result.Actions) > 0 && selfPtr.HP < before && result.Amount < 20
			case "stamina_recovery_cadence_loop":
				before := selfPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnEnd", 0, "", context)
				settled = len(rows) > 0 && selfPtr.HP > before
			case "stamina_retaliatory_wall":
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "stamina_prevent_first_lethal":
				result := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), selfPtr, enemyPtr, 100, "physical", context)
				settled = len(result.Actions) > 0 && result.Amount < 100 && result.DeferredHeal > 0
			case "impact_dash_spear_followup":
				beforeEnemyHP := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < beforeEnemyHP
			case "pressure_point_break_armor":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "physical", &battleEquipmentHookContext{DamageType: "physical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0
			case "counterframe_retaliation":
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "scythe_hunt_repeat":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealCrit", 20, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "roving_formation_step":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnStart", 0, "", context)
				settled = len(rows) > 0 && selfPtr.Position == "vanguard" && findBuffByID(selfPtr, "specialization:damage_reduction:"+hook.ID) != nil
			case "mana_ward_absorb":
				beforeMP := selfPtr.MP
				result := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), selfPtr, enemyPtr, 20, "magical", context)
				settled = len(result.Actions) > 0 && result.Amount < 20 && selfPtr.MP < beforeMP
			case "sentinel_link_intercept":
				before := selfPtr.HP
				result := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), allyPtr, enemyPtr, 20, "physical", context)
				settled = len(result.Actions) > 0 && selfPtr.HP < before && battleHasBuff(enemyPtr, "specialization_sentinel_target")
			case "strength_sacrificial_assault_low_hp", "strength_desperate_stand_damage", "strength_greatblade_desperate_strike":
				selfPtr.HP = 30
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "physical", &battleEquipmentHookContext{DamageType: "physical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0
			case "strength_bloodied_counterattack":
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "strength_sword_finisher_composure":
				beforeHP := selfPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onKill", 10, "physical", context)
				settled = len(rows) > 0 && selfPtr.HP > beforeHP && battleHasBuff(selfPtr, "specialization_sword_composure")
			case "scythe_deadly_repeat":
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealCrit", 20, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "dexterity_spear_brace_counter":
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "dexterity_bow_rhythm_crit":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealCrit", 20, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(selfPtr, "specialization_bow_rhythm")
			case "scythe_reaper_wound":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 20, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "specialization_reaper_wound") && battleCountBuff(enemyPtr, "specialization_critical_exposure") == 1 && math.Abs(enemyPtr.Stats["crit_damage_resist"]+0.05) < 0.0001
			case "dexterity_shadow_dance_extra_turn":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealCrit", 20, "physical", context)
				for _, row := range rows {
					settled = settled || row["kind"] == "extra_turn"
				}
			case "dexterity_shadowstep_momentum":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealCrit", 20, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(selfPtr, "specialization_shadow_dance")
			case "intelligence_magic_amplification_high_mp":
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "magical", &battleEquipmentHookContext{DamageType: "magical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0
			case "intelligence_mind_focus_turn":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnStart", 0, "", context)
				settled = len(rows) > 0 && selfPtr.Skills[0].CooldownRemain == 1
			case "intelligence_mind_focus_damage":
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "magical", &battleEquipmentHookContext{DamageType: "magical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0
			case "staff_arcane_overflow_repeat":
				magical := *context
				magical.DamageType = "magical"
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "magical", &magical)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "intelligence_mana_flood_cycle":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, enemyPtr, "onAfterSkillCast", 0, "", context)
				settled = len(rows) > 0 && battleSpecializationCounterStacks(selfPtr, "intelligence_mana_flood") == 1
			case "perception_eagle_eye_mark":
				rows := s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(1))
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "specialization_eagle_eye")
			case "perception_eagle_eye_bow_opening", "perception_preemptive_spear_opening":
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "physical", &battleEquipmentHookContext{DamageType: "physical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0
			case "perception_precision_mark_penetration":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(state, time.UnixMilli(1), selfPtr, enemyPtr, "on_before_deal_damage", 10, "physical", &battleEquipmentHookContext{DamageType: "physical", RawDefense: 100})
				settled = total > 0 && len(rows) > 0
			case "perception_bow_retarget":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onKill", 10, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "specialization_eagle_eye")
			case "perception_quick_nock_reward":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onKill", 10, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(selfPtr, "specialization_quick_nock")
			case "bow_vital_insight_repeat":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealCrit", 20, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "perception_preemptive_designation":
				rows := s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(1))
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "specialization_preemptive_target")
			case "stamina_teamwork_guard", "stamina_absolute_defense_turn":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnStart", 0, "", context)
				settled = len(rows) > 0 && findBuffByID(selfPtr, "specialization:damage_reduction:"+hook.ID) != nil
			case "stamina_vitality_ally_heal", "watchful_aid_turn_end":
				before := allyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnEnd", 0, "", context)
				settled = len(rows) > 0 && allyPtr.HP > before
			case "stamina_field_aid_after_hit":
				before := allyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && allyPtr.HP > before
			case "stamina_selfless_guard_lethal_ally":
				before := selfPtr.HP
				result := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), allyPtr, enemyPtr, 100, "physical", context)
				settled = len(result.Actions) > 0 && selfPtr.HP < before && result.Amount < 100
			case "spear_weakpoint_repeat":
				battleApplyOrRefreshBuff(enemyPtr, "specialization_exposed", 2, nil, selfPtr.UnitID, "refresh_duration")
				before := enemyPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && enemyPtr.HP < before
			case "battle_fervor_after_hit":
				beforeHP := selfPtr.HP
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && selfPtr.HP > beforeHP
			case "instant_arcana_extra_turn":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, enemyPtr, "onAfterSkillCast", 0, "", context)
				for _, row := range rows {
					settled = settled || row["kind"] == "extra_turn"
				}
			case "eagle_volley_multihit_mark":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 10, "physical", context)
				settled = len(rows) > 0 && battleHasBuff(enemyPtr, "specialization_eagle_volley")
			case "guard_dash_turn_start":
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnStart", 0, "", context)
				settled = len(rows) > 0 && selfPtr.Position == "vanguard"
			case "mana_return_after_hit":
				selfPtr.MP = 10
				rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", context)
				settled = len(rows) > 0 && selfPtr.MP > 10
			default:
				t.Fatalf("source hook %q is missing an execution scenario", definitionID)
			}
			if !settled {
				t.Fatalf("source hook %q produced no authoritative observable settlement: self=%#v ally=%#v enemy=%#v", definitionID, *selfPtr, *allyPtr, *enemyPtr)
			}
			settledIDs[definitionID] = struct{}{}
		})
	}
	if len(settledIDs) != len(hooks) {
		t.Fatalf("settled source hooks=%d, want %d", len(settledIDs), len(hooks))
	}
}

func TestBattleSpecializationCriticalExposureRequiresMindOrbAndSharesScytheCap(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	rules := s.battleFindBuffRules("specialization_critical_exposure", "while_active")
	if len(rules) != 1 || len(rules[0].EffectKinds) != 1 || rules[0].EffectKinds[0] != "addstat" {
		t.Fatalf("critical-exposure fixture did not load its stat rule: %#v", rules)
	}
	mindOrbHook := specializationTestSourceHook(t, "perception_mind_orb_critical_exposure")
	scytheHook := specializationTestSourceHook(t, "scythe_reaper_wound")
	self := specializationTestUnit("player:self", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	target.Stats["crit_damage_resist"] = 0.5
	target.BaseStats["crit_damage_resist"] = 0.5
	self.SpecializationHooks = []battleEquipmentHookRule{mindOrbHook}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}, Enemies: []battleUnitState{target}}
	selfPtr, targetPtr := &state.Players[0], &state.Enemies[0]
	context := &battleEquipmentHookContext{SkillID: "skill:test", SkillCategory: "attack", CastKind: "skill", DamageType: "physical", HitCount: 1}

	rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), selfPtr, targetPtr, targetPtr, "onAfterDealDamage", 10, "physical", context)
	if len(rows) != 0 || battleHasBuff(targetPtr, "specialization_critical_exposure") {
		t.Fatalf("mind-orb exposure bypassed its equipment gate: rows=%#v buffs=%#v", rows, targetPtr.Buffs)
	}
	selfPtr.WeaponClasses = []string{"mind_orb"}
	rows = s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(2), selfPtr, targetPtr, targetPtr, "onAfterDealDamage", 10, "physical", context)
	if len(rows) == 0 || battleCountBuff(targetPtr, "specialization_critical_exposure") != 1 || math.Abs(targetPtr.Stats["crit_damage_resist"]-0.45) >= 0.0001 {
		t.Fatalf("mind-orb exposure did not settle: rows=%#v buffs=%#v stats=%#v", rows, targetPtr.Buffs, targetPtr.Stats)
	}

	selfPtr.SpecializationHooks = []battleEquipmentHookRule{mindOrbHook, scytheHook}
	selfPtr.WeaponClasses = []string{"mind_orb", "scythe"}
	for turn := 0; turn < 5; turn++ {
		state.BattleHookTurnCounts = map[string]int{}
		rows = s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(int64(turn+3)), selfPtr, targetPtr, targetPtr, "onAfterDealDamage", 10, "physical", context)
		if len(rows) == 0 {
			t.Fatalf("shared exposure hooks produced no settlement on turn %d", turn+1)
		}
	}
	if got := battleCountBuff(targetPtr, "specialization_critical_exposure"); got != 6 {
		t.Fatalf("mind-orb and scythe exposure stacks=%d, want shared cap 6", got)
	}
	if got := targetPtr.Stats["crit_damage_resist"]; math.Abs(got-0.20) >= 0.0001 {
		t.Fatalf("critical damage resistance=%v, want 0.20 after a 0.30 reduction", got)
	}
}

func TestBattleSpecializationRecursiveDispatchGuards(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	hookA := specializationTestHook("hook:a", "onAfterDealDamage", "node:a", map[string]any{"kind": "repeat_context_effect", "amountRatio": .1, "targetMode": "targeting", "dispatchHooks": true})
	hookB := specializationTestHook("hook:b", "onAfterDealDamage", "node:b", map[string]any{"kind": "repeat_context_effect", "amountRatio": .1, "targetMode": "targeting", "dispatchHooks": true})
	actor.SpecializationHooks = []battleEquipmentHookRule{hookA, hookB}
	state := &battleRuntimeState{Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)
	runtime.skillID = "skill:test"
	runtime.ApplyDamage(battleskills.DamageAction{SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 10, DamageType: "physical"})
	blocked := 0
	damageRows := 0
	for _, row := range runtime.ActionLog() {
		if row["kind"] == "specialization_dispatch_blocked" {
			blocked++
		}
		if row["kind"] == "damage" {
			damageRows++
		}
	}
	if blocked == 0 || damageRows == 0 || damageRows > battleMaxSpecializationDispatchEvents+1 {
		t.Fatalf("A->B->A guard did not bound recursion: blocked=%d damage=%d actions=%#v", blocked, damageRows, runtime.ActionLog())
	}

	scopeDone := battleBeginSpecializationDispatchScope(state)
	leave, reason := battleEnterSpecializationContribution(state, hookA)
	if reason != "" {
		t.Fatalf("first sibling entry blocked: %s", reason)
	}
	leave()
	leave, reason = battleEnterSpecializationContribution(state, hookA)
	if reason != "" {
		t.Fatalf("sibling entry was mistaken for recursion: %s", reason)
	}
	leave()
	for index := state.specializationDispatchEvents; index < battleMaxSpecializationDispatchEvents; index++ {
		hook := hookA
		hook.ID = fmt.Sprintf("budget:%d", index)
		leave, reason = battleEnterSpecializationContribution(state, hook)
		if reason != "" {
			t.Fatalf("budget blocked early at %d: %s", index, reason)
		}
		leave()
	}
	if _, reason = battleEnterSpecializationContribution(state, specializationTestHook("budget:overflow", "onAfterDealDamage", "overflow", map[string]any{"kind": "heal"})); reason != "event_budget" {
		t.Fatalf("budget overflow reason=%q, want event_budget", reason)
	}
	scopeDone()
}

func TestBattleSpecializationRecursiveEffectKindsStayBounded(t *testing.T) {
	containsBlocked := func(rows []map[string]any) bool {
		payload, err := json.Marshal(rows)
		return err == nil && strings.Contains(string(payload), `"kind":"specialization_dispatch_blocked"`)
	}

	t.Run("damage", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		target := specializationTestUnit("enemy:target", "enemy", 100, 0)
		target.HP, target.MaxHP = 1_000, 1_000
		actor.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("damage_loop", "onAfterDealDamage", "damage_node", map[string]any{
			"kind": "damage", "targetMode": "targeting", "damageType": "physical", "dispatchHooks": true,
			"amountFormula": map[string]any{"baseAdd": 5},
		})}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
		runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)
		runtime.ApplyDamage(battleskills.DamageAction{SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 10, DamageType: "physical"})
		if !containsBlocked(runtime.ActionLog()) {
			t.Fatalf("recursive damage contribution escaped ancestry guard: %#v", runtime.ActionLog())
		}
	})

	t.Run("retaliate A to B to A", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		enemy := specializationTestUnit("enemy:actor", "enemy", 100, 0)
		actor.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("retaliate_a", "onAfterTakeDamage", "retaliate_a_node", map[string]any{
			"kind": "retaliate", "targetMode": "source", "strengthRatio": .1, "staminaRatio": .1, "dispatchHooks": true,
		})}
		enemy.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("retaliate_b", "onAfterTakeDamage", "retaliate_b_node", map[string]any{
			"kind": "retaliate", "targetMode": "source", "strengthRatio": .1, "staminaRatio": .1, "dispatchHooks": true,
		})}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{enemy}}
		runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)
		runtime.ApplyDamage(battleskills.DamageAction{SourceID: enemy.UnitID, TargetID: actor.UnitID, Amount: 10, DamageType: "physical"})
		if !containsBlocked(runtime.ActionLog()) {
			t.Fatalf("retaliate A->B->A escaped ancestry guard: %#v", runtime.ActionLog())
		}
	})

	t.Run("mirror derived damage", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
		skill := battleSkillState{ID: "skill:mirror", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{
			ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, HitCount: battleSkillTargetCount{Base: 1}},
		}}}
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.Skills = []battleSkillState{skill}
		actor.SpecializationHooks = []battleEquipmentHookRule{
			specializationTestHook("mirror", "onAfterSkillCast", "mirror_node", map[string]any{"kind": "mirror_cast_skill", "amountRatio": .5, "applyCooldown": false, "dispatchHooks": true}),
			specializationTestHook("mirror_damage_loop", "onAfterDealDamage", "mirror_damage_node", map[string]any{
				"kind": "repeat_context_effect", "amountRatio": .1, "targetMode": "targeting", "dispatchHooks": true,
			}),
		}
		target := specializationTestUnit("enemy:target", "enemy", 100, 0)
		target.HP, target.MaxHP = 1_000, 1_000
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
		rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", &battleEquipmentHookContext{SkillID: skill.ID, CastKind: "skill"})
		if !containsBlocked(rows) {
			t.Fatalf("mirror-derived damage escaped the active dispatch scope: %#v", rows)
		}
	})

	t.Run("add buff derived hook", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("add_a", "onAfterDealDamage", "add_a_node", map[string]any{
			"kind": "add_buff", "buffId": "test_positive_buff", "durationTurns": 1, "dispatchHooks": true,
		})}
		actor.EquipmentHooks = []battleEquipmentHookRule{{
			ID: "equipment:buff_damage", SourceKind: "equipment", SourceItemID: "test:item", Trigger: "onAfterApplyBuff",
			Effects: []map[string]any{{
				"kind": "damage", "targetMode": "enemy", "damageType": "physical",
				"amountFormula": map[string]any{"baseAdd": 5}, "dispatchHooks": true,
			}},
		}}
		target := specializationTestUnit("enemy:target", "enemy", 1_000, 0)
		target.MaxHP = 1_000
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
		runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)
		runtime.ApplyDamage(battleskills.DamageAction{SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 10, DamageType: "physical"})
		if !battleHasBuff(&state.Players[0], "test_positive_buff") || len(runtime.ActionLog()) > battleMaxSpecializationDispatchEvents+4 {
			t.Fatalf("recursive add_buff chain was not bounded: %#v", runtime.ActionLog())
		}
	})

	t.Run("cast skill derived damage", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
		skill := battleSkillState{ID: "skill:cast_loop", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{
			ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, HitCount: battleSkillTargetCount{Base: 1}},
		}}}
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.Skills = []battleSkillState{skill}
		actor.SpecializationHooks = []battleEquipmentHookRule{
			specializationTestHook("cast_a", "onTurnStart", "cast_a_node", map[string]any{
				"kind": "cast_skill", "skillId": skill.ID, "amountRatio": 1.0, "dispatchHooks": true,
			}),
			specializationTestHook("cast_b", "onAfterDealDamage", "cast_b_node", map[string]any{
				"kind": "cast_skill", "skillId": skill.ID, "amountRatio": 1.0, "dispatchHooks": true,
			}),
		}
		target := specializationTestUnit("enemy:target", "enemy", 1_000, 0)
		target.MaxHP = 1_000
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
		rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, &state.Enemies[0], "onTurnStart", 0, "", nil)
		if !containsBlocked(rows) {
			t.Fatalf("recursive cast_skill contribution escaped ancestry guard: %#v", rows)
		}
	})

	t.Run("extra turn cannot reenter itself", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
		targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
		skill := battleSkillState{ID: "skill:turn", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting, AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, HitCount: battleSkillTargetCount{Base: 1}}}}}
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.UserID = 1
		actor.Skills = []battleSkillState{skill}
		actor.SkillIDs = []string{skill.ID}
		actor.SkillSlots = []string{skill.ID}
		extraHook := specializationTestHook("extra_loop", "onTurnStart", "extra_node", map[string]any{"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": true})
		extraHook.MaxTriggersPerBattle = 1
		actor.SpecializationHooks = []battleEquipmentHookRule{extraHook}
		target := specializationTestUnit("enemy:target", "enemy", 100, 0)
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}, Order: []string{actor.UnitID, target.UnitID}}
		rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
		payload, err := json.Marshal(rows)
		if err != nil || strings.Count(string(payload), `"kind":"extra_turn"`) != 1 {
			t.Fatalf("extra-turn grant was not queued exactly once: %#v", rows)
		}
		if len(state.ExtraTurnQueue) != 1 || normalizeAnyMap(rows[0]["turn"]) != nil {
			t.Fatalf("extra turn was nested instead of entering the formal queue: rows=%#v queue=%#v", rows, state.ExtraTurnQueue)
		}
		if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
			t.Fatalf("process formal extra turn: %v", err)
		}
		if len(state.ExtraTurnQueue) != 0 || !parseBooleanLoose(normalizeAnyMap(lastBattleTurnActionData(t, state)["resultMeta"])["extraTurn"], false) {
			t.Fatalf("one-shot specialization extra turn did not settle independently: queue=%#v event=%#v", state.ExtraTurnQueue, lastBattleTurnActionData(t, state))
		}
	})
}

func TestBattleSpecializationDerivedHookKeepsInnermostSource(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 1_000, 0)
	target.MaxHP = 1_000
	actor.SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("outer_damage", "onAfterDealDamage", "outer_node", map[string]any{
			"kind": "repeat_context_effect", "amountRatio": .1, "targetMode": "target", "dispatchHooks": true,
		}),
		specializationTestHook("inner_restore", "onAfterDealDamage", "inner_node", map[string]any{
			"kind": "restore_mp", "maxMpRatio": .1,
		}),
	}
	for _, hook := range actor.SpecializationHooks {
		if err := validateBattleSpecializationHookRule(hook); err != nil {
			t.Fatalf("test hook %s is invalid: %v", hook.ID, err)
		}
	}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 10, DamageType: "physical"})
	rows := runtime.ActionLog()
	foundOuter, foundInner := false, false
	for _, row := range rows {
		meta := normalizeAnyMap(row["meta"])
		switch strings.TrimSpace(fmt.Sprintf("%v", row["kind"])) {
		case "damage":
			foundOuter = foundOuter || meta["nodeId"] == "outer_node" && meta["sourceNameKey"] == "specialization.node.outer_node.name"
		case "resource":
			foundInner = foundInner || meta["nodeId"] == "inner_node" && meta["sourceNameKey"] == "specialization.node.inner_node.name"
		}
	}
	if !foundOuter || !foundInner {
		t.Fatalf("derived specialization source was overwritten by its parent: %#v", rows)
	}
}

func TestBattleSpecializationAuthoringParityEffectsExecute(t *testing.T) {
	t.Run("add buff dispatches only when enabled", func(t *testing.T) {
		for _, dispatch := range []bool{false, true} {
			s := newSpecializationExecutionTestServer()
			actor := specializationTestUnit("player:actor", "player", 100, 0)
			actor.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("add", "onTurnStart", "add_node", map[string]any{
				"kind": "add_buff", "buffId": "test_positive_buff", "durationTurns": 2, "dispatchHooks": dispatch,
			})}
			actor.EquipmentHooks = []battleEquipmentHookRule{{
				ID: "equipment:after_buff", SourceKind: "equipment", SourceItemID: "test:item", Trigger: "onAfterApplyBuff",
				Effects: []map[string]any{{"kind": "add_buff", "buffId": "test_nested_buff", "durationTurns": 1, "targetMode": "self"}},
			}}
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
			rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
			if !battleHasBuff(&state.Players[0], "test_positive_buff") || len(rows) == 0 {
				t.Fatalf("dispatch=%v add_buff did not settle: %#v", dispatch, rows)
			}
			if gotNested := battleHasBuff(&state.Players[0], "test_nested_buff"); gotNested != dispatch {
				t.Fatalf("dispatch=%v nested buff applied=%v", dispatch, gotNested)
			}
		}
	})

	t.Run("remove buff dispatch controls expiry consequence", func(t *testing.T) {
		for _, dispatch := range []bool{false, true} {
			s := newSpecializationExecutionTestServer()
			s.deps.GameData.Buff = append(s.deps.GameData.Buff, map[string]any{
				"id": "test_remove_unit_buff", "kind": "buff", "durationTurns": 2, "onExpire": "remove_unit",
			})
			actor := specializationTestUnit("player:actor", "player", 100, 0)
			battleApplyOrRefreshBuff(&actor, "test_remove_unit_buff", 2, nil, actor.UnitID, "refresh_duration")
			actor.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("remove", "onTurnStart", "remove_node", map[string]any{
				"kind": "remove_buff", "buffId": "test_remove_unit_buff", "dispatchHooks": dispatch,
			})}
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
			rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
			if battleHasBuff(&state.Players[0], "test_remove_unit_buff") || len(rows) == 0 {
				t.Fatalf("dispatch=%v remove_buff did not settle: %#v", dispatch, rows)
			}
			if gotAlive := state.Players[0].Alive; gotAlive == dispatch {
				t.Fatalf("dispatch=%v alive=%v, expiry consequence mismatch", dispatch, gotAlive)
			}
		}
	})

	t.Run("cast skill settles and respects dispatch", func(t *testing.T) {
		for _, dispatch := range []bool{false, true} {
			s := newSpecializationExecutionTestServer()
			targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
			skill := battleSkillState{ID: "skill:derived", Name: "derived", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{
				ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
				AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}},
			}}}
			actor := specializationTestUnit("player:actor", "player", 100, 0)
			actor.Skills = []battleSkillState{skill}
			actor.SpecializationHooks = []battleEquipmentHookRule{
				specializationTestHook("cast", "onTurnStart", "cast_node", map[string]any{
					"kind": "cast_skill", "skillId": skill.ID, "amountRatio": .5, "dispatchHooks": dispatch,
				}),
				specializationTestHook("damage_seen", "onAfterDealDamage", "damage_seen_node", map[string]any{
					"kind": "cycle_counter", "counterId": "derived_damage", "gain": 1, "maxStacks": 9,
				}),
			}
			target := specializationTestUnit("enemy:target", "enemy", 100, 0)
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
			rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, &state.Enemies[0], "onTurnStart", 0, "", nil)
			if state.Enemies[0].HP >= 100 || len(rows) == 0 {
				t.Fatalf("dispatch=%v cast_skill produced no damage: hp=%v rows=%#v", dispatch, state.Enemies[0].HP, rows)
			}
			wantCounter := 0
			if dispatch {
				wantCounter = 1
			}
			if got := battleSpecializationCounterStacks(&state.Players[0], "derived_damage"); got != wantCounter {
				t.Fatalf("dispatch=%v derived damage counter=%d, want %d", dispatch, got, wantCounter)
			}
		}
	})

	valid := []map[string]any{
		{"kind": "add_buff", "buffId": "test_positive_buff", "dispatchHooks": false},
		{"kind": "remove_buff", "buffId": "test_positive_buff", "dispatchHooks": false},
		{"kind": "cast_skill", "skillId": "skill:test", "amountRatio": 1.0, "dispatchHooks": false},
	}
	for _, effect := range valid {
		if err := validateBattleSpecializationHookEffect(effect); err != nil {
			t.Fatalf("valid authoring effect rejected: %#v: %v", effect, err)
		}
	}
	invalid := []map[string]any{
		{"kind": "add_buff", "buffId": "test_positive_buff"},
		{"kind": "add_buff", "buffId": "test_positive_buff", "dispatchHooks": false, "futureField": true},
		{"kind": "remove_buff", "buffId": "", "dispatchHooks": false},
		{"kind": "cast_skill", "skillId": "", "dispatchHooks": false},
		{"kind": "cast_skill", "skillId": "skill:test", "amountRatio": 0.0, "dispatchHooks": false},
		{"kind": "mirror_cast_skill", "skillIds": "skill:test", "dispatchHooks": false},
	}
	for _, effect := range invalid {
		if err := validateBattleSpecializationHookEffect(effect); err == nil {
			t.Fatalf("malformed authoring effect accepted: %#v", effect)
		}
	}
}

func TestBattleSpecializationMirrorFiltersExecute(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	skill := battleSkillState{ID: "skill:allowed", Category: "magic", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{
		ID: "damage:1", Kind: "damage", DamageType: "magical", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, DamageSpec: battleDamageSpec{DamageType: "magical", AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, HitCount: battleSkillTargetCount{Base: 1}},
	}}}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.Skills = []battleSkillState{skill}
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	hook := specializationTestHook("mirror", "onAfterSkillCast", "mirror_node")
	effect := map[string]any{
		"kind": "mirror_cast_skill", "amountRatio": .5, "applyCooldown": false,
		"allowMirroredCastTrigger": false, "skillIds": []any{skill.ID}, "dispatchHooks": false,
	}
	rows, handled := s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", hook, effect, &battleEquipmentHookContext{SkillID: "skill:other", SkillCategory: "magic", CastKind: "skill"})
	if !handled || len(rows) != 0 || state.Enemies[0].HP != 100 {
		t.Fatalf("skillIds mismatch did not suppress mirror: rows=%#v hp=%v", rows, state.Enemies[0].HP)
	}
	rows, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(2), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", hook, effect, &battleEquipmentHookContext{SkillID: skill.ID, SkillCategory: "magic", CastKind: "mirror_cast"})
	if len(rows) != 0 || state.Enemies[0].HP != 100 {
		t.Fatalf("allowMirroredCastTrigger=false did not suppress mirrored source: rows=%#v hp=%v", rows, state.Enemies[0].HP)
	}
	effect["allowMirroredCastTrigger"] = true
	rows, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(3), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", hook, effect, &battleEquipmentHookContext{SkillID: skill.ID, SkillCategory: "magic", CastKind: "mirror_cast"})
	if len(rows) == 0 || state.Enemies[0].HP >= 100 {
		t.Fatalf("allowMirroredCastTrigger=true did not execute mirror: rows=%#v hp=%v", rows, state.Enemies[0].HP)
	}
	state.Enemies[0].HP = 100
	effect["amountRatio"] = 2.0
	rows, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(4), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", hook, effect, &battleEquipmentHookContext{SkillID: skill.ID, SkillCategory: "magic", CastKind: "skill"})
	if len(rows) == 0 || state.Enemies[0].HP != 90 {
		t.Fatalf("authored amountRatio above one was clamped or ignored: rows=%#v hp=%v", rows, state.Enemies[0].HP)
	}
}

func TestBattleSpecializationDamageTypePoliciesExecute(t *testing.T) {
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	for _, tc := range []struct {
		policy string
		want   string
	}{
		{"preserve", "magical"},
		{"physical", "physical"},
		{"magical", "magical"},
		{"higher_final_damage", "physical"},
	} {
		t.Run(tc.policy, func(t *testing.T) {
			s := newSpecializationExecutionTestServer()
			mutation, err := parseBattleSpecializationEffectMutation(map[string]any{
				"id": "policy:" + tc.policy, "scope": "skill",
				"effectSelector": map[string]any{"mode": "all"},
				"ops":            map[string]any{"damageTypePolicy": tc.policy},
			})
			if err != nil {
				t.Fatal(err)
			}
			skill := battleSkillState{ID: "skill:policy", Category: "magic", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{
				ID: "damage:1", Kind: "damage", DamageType: "magical", Targeting: targeting,
				AmountFormula: battleSkillAmountFormula{BaseAdd: 100}, DamageSpec: battleDamageSpec{DamageType: "magical", AmountFormula: battleSkillAmountFormula{BaseAdd: 100}, HitCount: battleSkillTargetCount{Base: 1}},
			}}}
			actor := specializationTestUnit("player:actor", "player", 100, 0)
			actor.SpecializationEffectMutations = []battleAffixEffectMutation{mutation}
			target := specializationTestUnit("enemy:target", "enemy", 100, 0)
			target.Stats["physical_defence"] = 0
			target.Stats["magical_defence"] = 1000
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
			execution := s.executeBattleSkill(state, time.UnixMilli(1), &state.Players[0], skill, "active", nil)
			if len(execution.Effects) == 0 || execution.Effects[0]["damageType"] != tc.want {
				t.Fatalf("policy %s resolved effect=%#v, want %s", tc.policy, execution.Effects, tc.want)
			}
		})
	}
	if _, err := parseBattleSpecializationEffectMutation(map[string]any{
		"id": "policy:bad", "scope": "skill", "effectSelector": map[string]any{"mode": "all"},
		"ops": map[string]any{"damageTypePolicy": "future_policy"},
	}); err == nil {
		t.Fatal("unknown damageTypePolicy was accepted")
	}
}

func TestBattleSpecializationAttackStatPolicyUsesHigherAttackWithoutChangingDamageChannel(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	contribution := specializationTestSourceContribution(t, "confluence_strength_intelligence_mana_edge")
	mutations, err := s.resolveBattleSpecializationEffectMutations(contribution)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 1 || mutations[0].AttackStatPolicy != "higher" || mutations[0].DamageTypePolicy != "" {
		t.Fatalf("mana edge mutation=%#v", mutations)
	}

	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	damageFormula := battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}}
	damageEffect := battleSkillEffect{
		ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
		AmountFormula: damageFormula,
		DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}}, HitCount: battleSkillTargetCount{Base: 1}},
	}
	healEffect := battleSkillEffect{
		ID: "heal:1", Kind: "heal", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}},
	}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.Stats["physics_damage"] = 40
	actor.Stats["magical_damage"] = 100
	actor.SpecializationEffectMutations = mutations
	actor.Skills = []battleSkillState{{ID: "skill:strong_strike", Category: "blade", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{damageEffect, healEffect}}}

	out := s.applyBattleAffixEffectMutations(&actor, "skill", actor.Skills[0].ID, 1, targeting, actor.Skills[0].Effects)
	if len(out) != 2 {
		t.Fatalf("mutated effects=%#v", out)
	}
	if out[0].DamageType != "physical" || out[0].DamageSpec.DamageType != "physical" || out[0].DamageTypePolicy != "" {
		t.Fatalf("damage channel changed: %#v", out[0])
	}
	if len(out[0].AmountFormula.Terms) != 1 || out[0].AmountFormula.Terms[0].AttrKey != "magical_damage" || math.Abs(out[0].AmountFormula.Terms[0].Coefficient-1.06) > 1e-9 {
		t.Fatalf("outer amount formula did not use the higher attack stat: %#v", out[0].AmountFormula)
	}
	if len(out[0].DamageSpec.AmountFormula.Terms) != 1 || out[0].DamageSpec.AmountFormula.Terms[0].AttrKey != "magical_damage" || math.Abs(out[0].DamageSpec.AmountFormula.Terms[0].Coefficient-1.06) > 1e-9 {
		t.Fatalf("damage spec formula did not use the higher attack stat: %#v", out[0].DamageSpec.AmountFormula)
	}
	if len(out[1].AmountFormula.Terms) != 1 || out[1].AmountFormula.Terms[0].AttrKey != "physics_damage" || out[1].AmountFormula.Terms[0].Coefficient != 1 {
		t.Fatalf("damage-only selector changed a heal formula: %#v", out[1])
	}
	if actor.Skills[0].Effects[0].AmountFormula.Terms[0].AttrKey != "physics_damage" || actor.Skills[0].Effects[0].AmountFormula.Terms[0].Coefficient != 1 {
		t.Fatalf("source skill formula was mutated in place: %#v", actor.Skills[0].Effects[0].AmountFormula)
	}
	basicOut := s.applyBattleAffixEffectMutations(&actor, "basic_attack", "basic_attack", 1, targeting, []battleSkillEffect{damageEffect})
	if len(basicOut) != 1 || basicOut[0].DamageType != "physical" || basicOut[0].AmountFormula.Terms[0].AttrKey != "magical_damage" || math.Abs(basicOut[0].AmountFormula.Terms[0].Coefficient-1.06) > 1e-9 {
		t.Fatalf("basic attack did not use the same higher-attack rule: %#v", basicOut)
	}

	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	target.HP, target.MaxHP = 1000, 1000
	target.Stats["physical_defence"] = 0
	target.Stats["magical_defence"] = 10000
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	execution := s.executeBattleSkill(state, time.UnixMilli(1), &state.Players[0], actor.Skills[0], "active", nil)
	if len(execution.Effects) == 0 || execution.Effects[0]["damageType"] != "physical" {
		t.Fatalf("authoritative execution changed the damage channel: %#v", execution.Effects)
	}
	meta := normalizeAnyMap(execution.Effects[0]["meta"])
	if got := toFloat64Loose(meta["baseAttack"], -1); got != 106 {
		t.Fatalf("authoritative execution baseAttack=%v, want 106 from magical attack while retaining physical channel; meta=%#v", got, meta)
	}
	if got := toFloat64Loose(meta["rawDefense"], -1); got != 0 {
		t.Fatalf("authoritative execution used non-physical defense=%v; meta=%#v", got, meta)
	}
}

func TestBattleAttackStatPolicyPreservesTiesFixedAndMixedFormulas(t *testing.T) {
	formula := battleSkillAmountFormula{BaseAdd: 3, Terms: []battleAttackTerm{{AttrKey: "magical_damage", Coefficient: 2}}}
	reverseActor := &battleUnitState{Stats: map[string]float64{"physics_damage": 80, "magical_damage": 20}}
	reversed := applyBattleAttackStatPolicy(reverseActor, "higher", formula)
	if reversed.Terms[0].AttrKey != "physics_damage" || reversed.BaseAdd != 3 {
		t.Fatalf("magical damage formula did not adopt the higher physical attack: %#v", reversed)
	}

	tieActor := &battleUnitState{Stats: map[string]float64{"physics_damage": 50, "magical_damage": 50}}
	tied := applyBattleAttackStatPolicy(tieActor, "higher", formula)
	if tied.Terms[0].AttrKey != "magical_damage" {
		t.Fatalf("tie should preserve the authored attack stat: %#v", tied)
	}

	fixed := applyBattleAttackStatPolicy(reverseActor, "higher", battleSkillAmountFormula{BaseAdd: 25})
	if fixed.BaseAdd != 25 || len(fixed.Terms) != 0 {
		t.Fatalf("fixed formula changed: %#v", fixed)
	}
	mixedFormula := battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "magical_damage", Coefficient: 1}, {AttrKey: "physics_damage", Coefficient: .5}}}
	mixed := applyBattleAttackStatPolicy(reverseActor, "higher", mixedFormula)
	if !reflect.DeepEqual(mixed, mixedFormula) {
		t.Fatalf("mixed formula was collapsed: got=%#v want=%#v", mixed, mixedFormula)
	}
}

func TestBattleAttackStatPolicyDoesNotCompoundOrMutateSourceFormula(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	mutation, err := parseBattleSpecializationEffectMutation(map[string]any{
		"id": "attack_stat:repeat", "scope": "skill",
		"effectSelector": map[string]any{"mode": "kind", "kinds": []any{"damage"}},
		"ops":            map[string]any{"attackStatPolicy": "higher", "amountMultiplier": 1.06},
	})
	if err != nil {
		t.Fatal(err)
	}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	source := []battleSkillEffect{{
		ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}},
		DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}}, HitCount: battleSkillTargetCount{Base: 1}},
	}}
	actor := &battleUnitState{
		Stats:                         map[string]float64{"physics_damage": 40, "magical_damage": 100},
		Skills:                        []battleSkillState{{ID: "skill:test", Category: "blade", Effects: source}},
		SpecializationEffectMutations: []battleAffixEffectMutation{mutation},
	}
	first := s.applyBattleAffixEffectMutations(actor, "skill", "skill:test", 1, targeting, source)
	second := s.applyBattleAffixEffectMutations(actor, "skill", "skill:test", 1, targeting, source)
	if first[0].AmountFormula.Terms[0].Coefficient != 1.06 || second[0].AmountFormula.Terms[0].Coefficient != 1.06 {
		t.Fatalf("repeat resolution compounded the source formula: first=%#v second=%#v", first[0].AmountFormula, second[0].AmountFormula)
	}
	if source[0].AmountFormula.Terms[0].Coefficient != 1 || source[0].AmountFormula.Terms[0].AttrKey != "physics_damage" {
		t.Fatalf("source formula mutated in place: %#v", source[0].AmountFormula)
	}
}

func TestBattleAttackStatPolicyValidationAndPlayerPreviews(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	mutationPayload := map[string]any{
		"id": "attack_stat:preview", "scope": "both",
		"effectSelector": map[string]any{"mode": "kind", "kinds": []any{"damage"}},
		"ops":            map[string]any{"attackStatPolicy": "higher", "amountMultiplier": 1.06},
	}
	mutation, err := parseBattleSpecializationEffectMutation(mutationPayload)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"unknown policy": func(payload map[string]any) { normalizeAnyMap(payload["ops"])["attackStatPolicy"] = "future_policy" },
		"all selector":   func(payload map[string]any) { payload["effectSelector"] = map[string]any{"mode": "all"} },
		"heal selector": func(payload map[string]any) {
			payload["effectSelector"] = map[string]any{"mode": "kind", "kinds": []any{"heal"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			payload := cloneAnyMap(mutationPayload)
			payload["ops"] = cloneAnyMap(normalizeAnyMap(mutationPayload["ops"]))
			mutate(payload)
			if _, err := parseBattleSpecializationEffectMutation(payload); err == nil {
				t.Fatalf("invalid attackStatPolicy payload was accepted: %#v", payload)
			}
		})
	}

	rawSkill := map[string]any{
		"id": "skill:preview", "name": "Preview", "category": "blade", "cooldown": 3,
		"targeting": map[string]any{"targetPositions": []any{"vanguard"}, "targetScope": "enemy", "selectRule": "first", "targetCount": map[string]any{"base": 1}},
		"effects": []any{map[string]any{
			"id": "damage:preview", "kind": "damage", "damageType": "physical",
			"targeting":     map[string]any{"targetPositions": []any{"vanguard"}, "targetScope": "enemy", "selectRule": "first", "targetCount": map[string]any{"base": 1}},
			"amountFormula": map[string]any{"terms": []any{map[string]any{"attrKey": "physics_damage", "coefficient": 1.8, "perSkillLevel": 0.05}}},
			"damageSpec":    map[string]any{"damageType": "physical", "amountFormula": map[string]any{"terms": []any{map[string]any{"attrKey": "physics_damage", "coefficient": 1.8, "perSkillLevel": 0.05}}}, "hitCount": map[string]any{"base": 1}},
		}},
	}
	s.deps.GameData.Skills = []map[string]any{rawSkill}
	actor := battleUnitState{
		Stats:                         map[string]float64{"physics_damage": 40, "magical_damage": 100},
		SpecializationEffectMutations: []battleAffixEffectMutation{mutation},
	}
	catalog := s.battleSkillCatalogRows(nil)
	s.applyBattleSkillCatalogAttackStatPreview(catalog, actor, []store.OwnedSkill{{SkillID: "skill:preview", Level: 30}}, nil, "basic_attack")
	catalogEffects, ok := catalog[0]["effects"].([]battleSkillEffect)
	if !ok || len(catalogEffects) != 1 || catalogEffects[0].DamageType != "physical" || catalogEffects[0].AmountFormula.Terms[0].AttrKey != "magical_damage" || math.Abs(catalogEffects[0].AmountFormula.Terms[0].Coefficient-1.908) > 1e-9 || math.Abs(catalogEffects[0].AmountFormula.Terms[0].PerSkillLevel-0.053) > 1e-9 {
		t.Fatalf("skill catalog preview did not expose the resolved formula: %#v", catalog[0]["effects"])
	}

	actor.Skills = []battleSkillState{s.battleSkillByRawWithProgress("skill:preview", 30, 0, rawSkill)}
	publicUnits := battlePublicUnitSlice([]battleUnitState{actor})
	publicSkills := publicUnits[0]["skills"].([]fiber.Map)
	publicEffects := publicSkills[0]["effects"].([]battleSkillEffect)
	if len(publicEffects) != 1 || publicEffects[0].DamageType != "physical" || publicEffects[0].AmountFormula.Terms[0].AttrKey != "magical_damage" || math.Abs(publicEffects[0].AmountFormula.Terms[0].Coefficient-1.908) > 1e-9 || math.Abs(publicEffects[0].AmountFormula.Terms[0].PerSkillLevel-0.053) > 1e-9 {
		t.Fatalf("battle public preview did not expose the resolved formula: %#v", publicEffects)
	}
}

func TestPlayerBasicAttackPreviewAppliesSpecializationMutationLayer(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	mutation, err := parseBattleSpecializationEffectMutation(map[string]any{
		"id": "preview:scale", "scope": "both",
		"effectSelector": map[string]any{"mode": "all"},
		"ops":            map[string]any{"amountMultiplier": 2.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	profile := &battleBasicAttackProfile{
		DamageType: "physical", TargetDirection: "vanguard", TargetSeekStrategy: "front_to_rear",
		Effects: []battleSkillEffect{{
			ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}},
		}},
	}
	actor := specializationTestUnit("player:preview", "player", 100, 0)
	actor.SpecializationEffectMutations = []battleAffixEffectMutation{mutation}
	s.applyPlayerBasicAttackPreviewMutations(profile, &actor)
	if len(profile.Effects) != 1 || profile.Effects[0].AmountFormula.BaseAdd != 40 || profile.DamageSpec.AmountFormula.BaseAdd != 40 {
		t.Fatalf("basic attack preview omitted specialization mutation: %#v", profile)
	}
}

func TestBattleSpecializationMarkedHitRunsAfterEquipmentLayer(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	markID := "ordered_mark"
	buffID := battleSpecializationMarkBuffID(markID)
	battleApplyOrRefreshBuff(&target, buffID, 2, nil, actor.UnitID, "refresh_duration")
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID: "equipment:amp", SourceKind: "equipment", SourceItemID: "test:item", Trigger: "onBeforeDealDamage",
		Effects: []map[string]any{{"kind": "runtime_damage_amp", "value": .1}},
	}}
	actor.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("mark_cycle", "onAfterDealDamage", "mark_node", map[string]any{
		"kind": "mark_cycle", "markId": markID, "firstHit": "apply_mark",
		"markedHit": map[string]any{"kind": "runtime_damage_amp", "value": .5, "consumeMark": true},
	})}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	total, rows, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(
		state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], "onBeforeDealDamage", 10, "physical", &battleEquipmentHookContext{},
	)
	if math.Abs(total-.6) > .001 || battleHasBuff(&state.Enemies[0], buffID) || len(rows) < 2 {
		t.Fatalf("layered amps did not settle: total=%v rows=%#v buffs=%#v", total, rows, state.Enemies[0].Buffs)
	}
	firstMeta := normalizeAnyMap(rows[0]["meta"])
	secondMeta := normalizeAnyMap(rows[1]["meta"])
	if firstMeta["sourceKind"] == "specialization" || secondMeta["sourceKind"] != "specialization" {
		t.Fatalf("marked-hit specialization did not run after equipment layer: rows=%#v", rows)
	}
}

func TestBattlePreviousWaveCarryFailsClosed(t *testing.T) {
	player := specializationTestUnit("player:carry", "player", 80, 40)
	state := battleRuntimeState{
		Wave: 1, Players: []battleUnitState{player},
		BattleHookBattleCounts: map[string]int{"player:carry:hook": 2},
		CompletedTurnCounts:    map[string]int{"player:carry": 3},
	}
	previousRaw, err := json.Marshal(battleCarryStatePayload(state, store.BattleRunStatusEnded))
	if err != nil {
		t.Fatal(err)
	}
	previous := store.BattlePreviousWaveCarry{Wave: 1, Status: store.BattleRunStatusEnded, CarryStateRaw: previousRaw}
	carried, err := loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) { return previous, nil }, 1)
	if err != nil || len(carried.Players) != 1 || carried.BattleHookBattleCounts["player:carry:hook"] != 2 || carried.CompletedTurnCounts["player:carry"] != 3 {
		t.Fatalf("valid carry rejected or lost counters: carry=%#v err=%v", carried, err)
	}

	emptyCounterState := battleRuntimeState{Wave: 1, Players: []battleUnitState{player}}
	emptyCounterCarry := battleCarryStatePayload(emptyCounterState, store.BattleRunStatusEnded)
	encodedCarry, err := json.Marshal(emptyCounterCarry)
	if err != nil {
		t.Fatal(err)
	}
	persistedCarry := map[string]any{}
	if err := json.Unmarshal(encodedCarry, &persistedCarry); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"battleHookBattleCounts", "battleHookCooldownUntilTurns", "completedTurnCounts"} {
		if _, ok := persistedCarry[key].(map[string]any); !ok {
			t.Fatalf("empty %s must persist as an object, carry=%s", key, encodedCarry)
		}
	}

	for _, legacyShape := range []string{"missing", "null"} {
		legacyCarry := battleCarryStatePayload(emptyCounterState, store.BattleRunStatusEnded)
		for _, key := range []string{"battleHookBattleCounts", "battleHookCooldownUntilTurns", "completedTurnCounts"} {
			if legacyShape == "missing" {
				delete(legacyCarry, key)
			} else {
				legacyCarry[key] = nil
			}
		}
		legacyRaw, marshalErr := json.Marshal(legacyCarry)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		legacy := store.BattlePreviousWaveCarry{Wave: 1, Status: store.BattleRunStatusEnded, CarryStateRaw: legacyRaw}
		carried, err := loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) { return legacy, nil }, 1)
		if err != nil || len(carried.BattleHookBattleCounts) != 0 || len(carried.BattleHookCooldownUntilTurns) != 0 || len(carried.CompletedTurnCounts) != 0 {
			t.Fatalf("legacy %s counters were not decoded as empty: carry=%#v err=%v", legacyShape, carried, err)
		}
	}

	queryErr := errors.New("previous run unavailable")
	if _, err := loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) { return store.BattlePreviousWaveCarry{}, queryErr }, 1); !errors.Is(err, queryErr) {
		t.Fatalf("previous-run query failure did not fail closed: %v", err)
	}
	malformed := previous
	malformed.CarryStateRaw = json.RawMessage(`"corrupt"`)
	if _, err := loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) { return malformed, nil }, 1); !errors.Is(err, errBattlePreviousWaveCarryInvalid) {
		t.Fatalf("malformed carry did not fail closed: %v", err)
	}
	malformedCounter := previous
	malformedCounterCarry := battleCarryStatePayload(state, store.BattleRunStatusEnded)
	malformedCounterCarry["battleHookBattleCounts"] = []any{}
	malformedCounter.CarryStateRaw, err = json.Marshal(malformedCounterCarry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) { return malformedCounter, nil }, 1); !errors.Is(err, errBattlePreviousWaveCarryInvalid) {
		t.Fatalf("malformed counter object did not fail closed: %v", err)
	}
	empty := previous
	emptyCarry := battleCarryStatePayload(state, store.BattleRunStatusEnded)
	emptyCarry["players"] = []any{}
	empty.CarryStateRaw, err = json.Marshal(emptyCarry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) { return empty, nil }, 1); !errors.Is(err, errBattlePreviousWaveCarryInvalid) {
		t.Fatalf("empty carried players did not fail closed: %v", err)
	}
}

func TestBattlePlayerSnapshotSpecializationFenceDetectsDrift(t *testing.T) {
	base := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", BalanceRevision: 7, Revision: 3, Digest: "digest-3"}
	unit := battleUnitState{SpecializationTreeID: "tree", SpecializationRulesetVersion: "specialization-v5", SpecializationBalanceRevision: 7, SpecializationRevision: 3, SpecializationDigest: "digest-3"}
	if !battlePlayerSnapshotSpecializationStable(unit, base, base) {
		t.Fatal("stable before/unit/after snapshot was rejected")
	}
	after := base
	after.Revision = 4
	after.Digest = "digest-4"
	if battlePlayerSnapshotSpecializationStable(unit, base, after) {
		t.Fatal("revision drift during snapshot assembly was accepted")
	}
	after = base
	after.BalanceRevision++
	if battlePlayerSnapshotSpecializationStable(unit, base, after) {
		t.Fatal("balance revision drift during snapshot assembly was accepted")
	}
	unit.SpecializationDigest = "mixed"
	if battlePlayerSnapshotSpecializationStable(unit, base, base) {
		t.Fatal("mixed unit contribution was accepted")
	}
	if battlePlayerSnapshotMaxAttempts != 3 || errBattlePlayerSnapshotUnstable == nil {
		t.Fatal("snapshot retry boundary is not explicit")
	}

	t.Run("retries a controllable mid-assembly revision commit", func(t *testing.T) {
		revision3 := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", BalanceRevision: 7, Revision: 3, Digest: "digest-3"}
		revision4 := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", BalanceRevision: 7, Revision: 4, Digest: "digest-4"}
		reads := []store.SpecializationBattleContribution{revision3, revision4, revision4, revision4}
		readIndex := 0
		buildCount := 0
		unit, err := buildBattlePlayerSnapshotWithFence(
			func() (store.SpecializationBattleContribution, error) {
				row := reads[readIndex]
				readIndex++
				return row, nil
			},
			func() (battleUnitState, error) {
				buildCount++
				row := revision3
				if buildCount > 1 {
					row = revision4
				}
				return battleUnitState{
					SpecializationTreeID: row.TreeID, SpecializationRulesetVersion: row.RulesetVersion,
					SpecializationBalanceRevision: row.BalanceRevision,
					SpecializationRevision:        row.Revision, SpecializationDigest: row.Digest,
				}, nil
			},
		)
		if err != nil || buildCount != 2 || unit.SpecializationRevision != 4 {
			t.Fatalf("snapshot fence did not retry onto the stable revision: unit=%#v builds=%d err=%v", unit, buildCount, err)
		}
	})

	t.Run("fails closed after bounded repeated commits", func(t *testing.T) {
		readCount := 0
		buildCount := 0
		_, err := buildBattlePlayerSnapshotWithFence(
			func() (store.SpecializationBattleContribution, error) {
				readCount++
				revision := int64(1)
				if readCount%2 == 0 {
					revision = 2
				}
				return store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", BalanceRevision: 7, Revision: revision, Digest: fmt.Sprintf("digest-%d", revision)}, nil
			},
			func() (battleUnitState, error) {
				buildCount++
				return battleUnitState{SpecializationTreeID: "tree", SpecializationRulesetVersion: "specialization-v5", SpecializationBalanceRevision: 7, SpecializationRevision: 1, SpecializationDigest: "digest-1"}, nil
			},
		)
		if !errors.Is(err, errBattlePlayerSnapshotUnstable) || buildCount != battlePlayerSnapshotMaxAttempts {
			t.Fatalf("unstable snapshot was not bounded: builds=%d err=%v", buildCount, err)
		}
	})
}

func TestBattleStartAndWaveStartRunEveryWave(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	attackTarget := specializationTestUnit("enemy:attack", "enemy", 100, 0)
	threatTarget := specializationTestUnit("enemy:threat", "enemy", 100, 0)
	attackTarget.Stats["attack"] = 80
	attackTarget.Stats["magic_attack"] = 0
	threatTarget.Stats["attack"] = 30
	threatTarget.Stats["magic_attack"] = 70
	threatTarget.MaxHP = 1000
	actor.SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("hunter", "onBattleStart", "hunter_node", map[string]any{"kind": "mark_target", "markId": "hunter", "targetMode": "enemy", "selectionMode": "highest_attack"}),
		specializationTestHook("foresight", "onWaveStart", "foresight_node", map[string]any{
			"kind": "designate_priority_target", "markId": "foresight", "selectionMode": "highest_threat",
			"rewriteTargeting": map[string]any{"preferMarked": true, "fallback": "original"},
		}),
	}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{attackTarget, threatTarget}}
	if rows := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1)); len(rows) == 0 || !battleHasBuff(&state.Enemies[0], battleSpecializationMarkBuffID("hunter")) {
		t.Fatalf("battle-start hunter selection did not choose highest attack: %#v", rows)
	}
	battleRemoveBuff(&state.Enemies[0], battleSpecializationMarkBuffID("hunter"))
	state.Wave = 2
	if rows := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(2)); len(rows) == 0 || (!battleHasBuff(&state.Enemies[0], battleSpecializationMarkBuffID("hunter")) && !battleHasBuff(&state.Enemies[1], battleSpecializationMarkBuffID("hunter"))) {
		t.Fatalf("battle-start hooks did not run on wave two: %#v", rows)
	}
	state.Enemies[0].Stats["attack"] = 80
	state.Enemies[0].Stats["magic_attack"] = 0
	state.Enemies[1].Stats["attack"] = 30
	state.Enemies[1].Stats["magic_attack"] = 70
	if rows := s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(2)); len(rows) == 0 || !battleHasBuff(&state.Enemies[1], battleSpecializationMarkBuffID("foresight")) {
		t.Fatalf("wave-start foresight did not choose highest threat: %#v", rows)
	}
	targets := s.battleSelectTemplateTargets(state, &state.Players[0], battleSkillState{Level: 1}, battleSkillTargeting{
		TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "random", TargetCount: battleSkillTargetCount{Base: 1},
	}, false, func() float64 { return 0 })
	if len(targets) != 1 || targets[0].UnitID != "enemy:threat" {
		t.Fatalf("rewriteTargeting preferMarked did not affect real selection: %#v", targets)
	}
}

func TestBattleSpecializationMarkTargetSelectionModes(t *testing.T) {
	tests := []struct {
		name          string
		selectionMode string
		wantTargetID  string
	}{
		{name: "highest attack", selectionMode: "highest_attack", wantTargetID: "enemy:attack"},
		{name: "highest threat", selectionMode: "highest_threat", wantTargetID: "enemy:threat"},
		{name: "lowest hp", selectionMode: "lowest_hp", wantTargetID: "enemy:wounded"},
		{name: "seeded random", selectionMode: "random", wantTargetID: "enemy:threat"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newSpecializationExecutionTestServer()
			actor := specializationTestUnit("player:actor", "player", 100, 0)
			attackTarget := specializationTestUnit("enemy:attack", "enemy", 90, 0)
			attackTarget.Stats["attack"] = 100
			attackTarget.Stats["magic_attack"] = 0
			threatTarget := specializationTestUnit("enemy:threat", "enemy", 80, 0)
			threatTarget.Stats["attack"] = 20
			threatTarget.Stats["magic_attack"] = 100
			threatTarget.MaxHP = 1000
			woundedTarget := specializationTestUnit("enemy:wounded", "enemy", 10, 0)
			woundedTarget.Stats["attack"] = 10
			woundedTarget.Stats["magic_attack"] = 0
			actor.SpecializationHooks = []battleEquipmentHookRule{
				specializationTestHook("selection", "onBattleStart", "selection_node", map[string]any{
					"kind": "mark_target", "markId": "selection", "selectionMode": tc.selectionMode,
				}),
			}
			state := &battleRuntimeState{
				Seed: 1, Wave: 1, Players: []battleUnitState{actor},
				Enemies: []battleUnitState{woundedTarget, attackTarget, threatTarget},
			}
			rows := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1))
			if len(rows) == 0 {
				t.Fatalf("selectionMode=%s produced no action", tc.selectionMode)
			}
			markedTargetID := ""
			for index := range state.Enemies {
				if battleHasBuff(&state.Enemies[index], battleSpecializationMarkBuffID("selection")) {
					if markedTargetID != "" {
						t.Fatalf("selectionMode=%s marked multiple targets", tc.selectionMode)
					}
					markedTargetID = state.Enemies[index].UnitID
				}
			}
			if markedTargetID != tc.wantTargetID {
				t.Fatalf("selectionMode=%s marked %q, want %q", tc.selectionMode, markedTargetID, tc.wantTargetID)
			}
			wantRNGIndex := int64(0)
			if tc.selectionMode == "random" {
				wantRNGIndex = 1
			}
			if state.RNGIndex != wantRNGIndex {
				t.Fatalf("selectionMode=%s consumed %d RNG draws, want %d", tc.selectionMode, state.RNGIndex, wantRNGIndex)
			}
		})
	}

	valid := []map[string]any{
		{"kind": "mark_target", "markId": "selection", "selectionMode": "random"},
		{"kind": "designate_priority_target", "markId": "selection", "selectionMode": "lowest_hp", "rewriteTargeting": map[string]any{"preferMarked": true, "fallback": "original"}},
	}
	for _, effect := range valid {
		if err := validateBattleSpecializationHookEffect(effect); err != nil {
			t.Fatalf("valid targeting effect rejected: %#v: %v", effect, err)
		}
	}
	invalid := []map[string]any{
		{"kind": "mark_target", "markId": "selection", "selectionMode": "nearest"},
		{"kind": "mark_target", "markId": "selection"},
		{"kind": "designate_priority_target", "markId": "selection", "selectionMode": "nearest", "rewriteTargeting": map[string]any{"preferMarked": true, "fallback": "original"}},
	}
	for _, effect := range invalid {
		if err := validateBattleSpecializationHookEffect(effect); err == nil {
			t.Fatalf("invalid targeting effect accepted: %#v", effect)
		}
	}
}

func TestBattleSpecializationContextCooldownAndPositionSteps(t *testing.T) {
	t.Run("context skill cooldown touches only the triggering skill", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.Skills = []battleSkillState{
			{ID: "skill:context", CooldownRemain: 3},
			{ID: "skill:other", CooldownRemain: 5},
		}
		actor.SpecializationHooks = []battleEquipmentHookRule{
			specializationTestHook("context_cooldown", "onAfterSkillCast", "context_node", map[string]any{
				"kind": "adjust_skill_cooldown", "targetMode": "self", "amount": -1, "selectionMode": "context_skill", "maxSkills": 1,
			}),
		}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
		context := &battleEquipmentHookContext{SkillID: "skill:context", SkillCategory: "attack"}
		rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onAfterSkillCast", 0, "", context)
		if len(rows) == 0 || state.Players[0].Skills[0].CooldownRemain != 2 || state.Players[0].Skills[1].CooldownRemain != 5 {
			t.Fatalf("context skill cooldown selected the wrong skills: skills=%#v rows=%#v", state.Players[0].Skills, rows)
		}
	})

	t.Run("shift position honors maxSteps", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.Position = "vanguard"
		ally := specializationTestUnit("player:ally", "player", 10, 0)
		ally.Position = "rearguard"
		actor.SpecializationHooks = []battleEquipmentHookRule{
			specializationTestHook("shift", "onTurnStart", "shift_node", map[string]any{
				"kind": "shift_position", "policy": "toward_lowest_hp_ally", "maxSteps": 2,
			}),
		}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor, ally}}
		rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
		if len(rows) == 0 || state.Players[0].Position != "rearguard" {
			t.Fatalf("maxSteps=2 did not cross two formation steps: position=%s rows=%#v", state.Players[0].Position, rows)
		}
	})
}

func TestBattleSpecializationHealTargetModes(t *testing.T) {
	tests := []struct {
		mode         string
		wantTargetID string
	}{
		{mode: "self", wantTargetID: "player:actor"},
		{mode: "source", wantTargetID: "enemy:source"},
		{mode: "enemy", wantTargetID: "enemy:target"},
		{mode: "marked_enemy", wantTargetID: "enemy:marked"},
		{mode: "targeting", wantTargetID: "enemy:target"},
		{mode: "lowest_hp_ally", wantTargetID: "player:ally"},
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			s := newSpecializationExecutionTestServer()
			actor := specializationTestUnit("player:actor", "player", 60, 0)
			ally := specializationTestUnit("player:ally", "player", 20, 0)
			source := specializationTestUnit("enemy:source", "enemy", 40, 0)
			target := specializationTestUnit("enemy:target", "enemy", 50, 0)
			target.Stats["attack"] = 100
			marked := specializationTestUnit("enemy:marked", "enemy", 30, 0)
			battleApplyOrRefreshBuff(&marked, battleSpecializationMarkBuffID("existing"), 2, nil, actor.UnitID, "refresh_duration")
			state := &battleRuntimeState{
				Wave: 1, Players: []battleUnitState{actor, ally},
				Enemies: []battleUnitState{source, target, marked},
			}
			before := map[string]float64{}
			for index := range state.Players {
				before[state.Players[index].UnitID] = state.Players[index].HP
			}
			for index := range state.Enemies {
				before[state.Enemies[index].UnitID] = state.Enemies[index].HP
			}
			hook := specializationTestHook("heal", "onTurnEnd", "heal_node")
			rows, handled := s.executeBattleSpecializationHookEffect(
				state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], &state.Enemies[1],
				"onTurnEnd", 0, "", hook,
				map[string]any{"kind": "heal", "targetMode": tc.mode, "maxHpRatio": .1}, nil,
			)
			if !handled || len(rows) == 0 {
				t.Fatalf("targetMode=%s produced no heal", tc.mode)
			}
			assertHP := func(unit *battleUnitState) {
				want := before[unit.UnitID]
				if unit.UnitID == tc.wantTargetID {
					want += 10
				}
				if unit.HP != want {
					t.Fatalf("targetMode=%s unit %s hp=%v, want %v", tc.mode, unit.UnitID, unit.HP, want)
				}
			}
			for index := range state.Players {
				assertHP(&state.Players[index])
			}
			for index := range state.Enemies {
				assertHP(&state.Enemies[index])
			}
		})
	}
}

func TestBattleSpecializationBeforeAfterSkillCastAndDebtExecute(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}, {"id": "skill:test"}}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	actor.Skills = []battleSkillState{{
		ID: "skill:test", Name: "test", Level: 1, MPCost: 10, CooldownTurns: 2, Targeting: targeting,
		Effects: []battleSkillEffect{{ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting, AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}}}},
	}}
	actor.SkillIDs = []string{"skill:test"}
	actor.SkillSlots = []string{"skill:test"}
	debt := specializationTestHook("debt", "onBeforeSkillCast", "debt_node", map[string]any{"kind": "resource_debt_cast", "hpCostRatio": .12, "cooldownDelta": 1, "minimumHp": 1})
	debt.Conditions = []map[string]any{{"kind": "context_mp_insufficient"}}
	pattern := specializationTestHook("pattern", "onAfterSkillCast", "pattern_node", map[string]any{"kind": "cycle_counter", "counterId": "pattern", "gain": 1, "maxStacks": 3})
	pattern.Conditions = []map[string]any{{"kind": "context_skill_non_basic"}}
	actor.SpecializationHooks = []battleEquipmentHookRule{debt, pattern}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	payload := s.executeUnitTurn(state, time.UnixMilli(1), &state.Players[0], "player", false)
	if state.Players[0].HP != 88 || battleSpecializationCounterStacks(&state.Players[0], "pattern") != 1 || state.Enemies[0].HP >= 100 {
		t.Fatalf("cast hooks did not settle: actor=%#v enemyHP=%v payload=%#v", state.Players[0], state.Enemies[0].HP, payload)
	}
	visibleDebt := false
	for _, row := range battleAnyMapSlice(payload["actionLog"]) {
		meta := normalizeAnyMap(row["meta"])
		if row["kind"] == "resource" && meta["nodeId"] == "debt_node" && meta["sourceNameKey"] != "" {
			visibleDebt = true
		}
	}
	if !visibleDebt {
		t.Fatalf("debt cast action lacked specialization source metadata: %#v", payload["actionLog"])
	}
}

func TestBattleSpecializationRepeatContextRatioAboveOneIsHonored(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	target.Stats["physical_defence"] = 0
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	hook := specializationTestHook("repeat", "onAfterDealDamage", "repeat_node")
	rows, handled := s.executeBattleSpecializationHookEffect(
		state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], &state.Enemies[0],
		"onAfterDealDamage", 10, "physical", hook,
		map[string]any{"kind": "repeat_context_effect", "amountRatio": 1.5, "targetMode": "targeting", "dispatchHooks": false}, nil,
	)
	if !handled || len(rows) == 0 || math.Abs(state.Enemies[0].HP-85) > .001 {
		t.Fatalf("repeat ratio above one was clamped or skipped: hp=%v rows=%#v", state.Enemies[0].HP, rows)
	}
}

func TestBattleSpecializationMutationSelectorsAndOperationsExecute(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	payload := map[string]any{
		"id": "mutation:test", "scope": "skill",
		"skillIds": []any{"skill:test"}, "skillCategories": []any{"attack"}, "weaponClasses": []any{"sword"}, "damageTypes": []any{"physical"},
		"effectSelector": map[string]any{"mode": "effect_id", "ids": []any{"damage:1"}},
		"ops": map[string]any{
			"amountMultiplier": .5, "amountTermsAppend": []any{map[string]any{"attributeKey": "strength", "coefficient": .2}},
			"hitCountDelta": 1, "cooldownDelta": -1, "cooldownFloor": 1,
			"mpCostDelta": 2, "mpCostMultiplier": 3, "damageTypePolicy": "higher_final_damage",
			"targetingPatch": map[string]any{"targetCount": 2, "selectRule": "hp_min"},
			"appendEffects": []any{
				map[string]any{"id": "generated:consume", "kind": "consume_hp", "maxHpRatio": .03, "minimumHp": 1},
				map[string]any{"id": "generated:splash", "kind": "splash_damage", "targetCount": 2, "amountRatio": .4},
				map[string]any{"id": "generated:mark", "kind": "mark_target", "markId": "exposed", "durationTurns": 2},
			},
		},
	}
	mutation, err := parseBattleSpecializationEffectMutation(payload)
	if err != nil {
		t.Fatal(err)
	}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.WeaponClasses = []string{"sword"}
	scalarEffects := []battleSkillEffect{{ID: "damage:1", Kind: "damage", DamageType: "physical", DamageSpec: battleDamageSpec{DamageType: "physical"}}}
	actor.Skills = []battleSkillState{{ID: "skill:test", Category: "attack", Effects: scalarEffects}}
	actor.SpecializationEffectMutations = []battleAffixEffectMutation{mutation}
	skill := actor.applyBattleSkillScalarMutations("skill", battleSkillState{ID: "skill:test", Category: "attack", CooldownTurns: 3, MPCost: 4, Effects: scalarEffects})
	if skill.CooldownTurns != 2 || skill.MPCost != 18 {
		t.Fatalf("scalar mutation failed: %#v", skill)
	}
	lowCooldown := actor.applyBattleSkillScalarMutations("skill", battleSkillState{ID: "skill:test", Category: "attack", CooldownTurns: 1, MPCost: 4, Effects: scalarEffects})
	if lowCooldown.CooldownTurns != 1 || lowCooldown.MPCost != 18 {
		t.Fatalf("cooldown floor or MP multiplier failed: %#v", lowCooldown)
	}
	zeroCooldown := actor.applyBattleSkillScalarMutations("skill", battleSkillState{ID: "skill:test", Category: "attack", CooldownTurns: 0, MPCost: 4, Effects: scalarEffects})
	if zeroCooldown.CooldownTurns != 0 || zeroCooldown.MPCost != 18 {
		t.Fatalf("zero-cooldown skill was converted into a cooldown skill: %#v", zeroCooldown)
	}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1, Max: 1, MaxSet: true}}
	effects := []battleSkillEffect{{
		ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{BaseAdd: 20},
		DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}},
	}}
	out := s.applyBattleAffixEffectMutations(&actor, "skill", "skill:test", 1, targeting, effects)
	if len(out) != 4 {
		t.Fatalf("mutated effects=%#v, want selected plus three generated effects", out)
	}
	if out[0].AmountFormula.BaseAdd != 10 || len(out[0].AmountFormula.Terms) != 1 || out[0].DamageSpec.HitCount.Base != 2 || out[0].DamageTypePolicy != "higher_final_damage" || out[0].Targeting.TargetCount.Base != 2 || !out[0].Targeting.TargetCount.MaxSet || out[0].Targeting.TargetCount.Max != 2 || out[0].Targeting.SelectRule != "hp_min" {
		t.Fatalf("effect mutation operations did not settle: %#v", out[0])
	}
	if out[1].Kind != "consume_hp" || out[2].Kind != "damage" || out[2].GeneratedBy != "mutation:test" || out[3].Kind != "status" || out[3].StatusPayload.StatusIDs[0] != battleSpecializationMarkBuffID("exposed") {
		t.Fatalf("generated effects did not settle: %#v", out[1:])
	}
	bad := cloneAnyMap(payload)
	bad["effectSelector"] = map[string]any{"mode": "index", "indexes": []any{1}}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("specialization index selector was accepted")
	}
	bad = cloneAnyMap(payload)
	bad["ops"] = map[string]any{"futureOp": 1}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("unknown specialization mutation op was accepted")
	}
	bad = cloneAnyMap(payload)
	bad["ops"] = map[string]any{"cooldownDelta": -1, "mpCostMultiplier": 3}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("cooldown reduction without a one-turn floor was accepted")
	}
	bad = cloneAnyMap(payload)
	bad["ops"] = map[string]any{"cooldownDelta": -1, "cooldownFloor": 1, "mpCostMultiplier": 2.99}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("cooldown reduction below the 300% MP multiplier was accepted")
	}
	bad = cloneAnyMap(payload)
	bad["ops"] = map[string]any{"cooldownDelta": -1, "cooldownFloor": 1, "mpCostDelta": -1, "mpCostMultiplier": 3}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("cooldown reduction with a negative MP cost delta was accepted")
	}
	bad = cloneAnyMap(payload)
	delete(bad, "weaponClasses")
	bad["ops"] = map[string]any{"hitCountDelta": 1}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("extra-hit specialization without a weapon restriction was accepted")
	}
	for _, tc := range []struct {
		name  string
		field string
		value []any
	}{
		{"skill category", "skillCategories", []any{"magic"}},
		{"weapon class", "weaponClasses", []any{"bow"}},
		{"damage type", "damageTypes", []any{"magical"}},
	} {
		t.Run(tc.name+" exclusion", func(t *testing.T) {
			filteredPayload := cloneAnyMap(payload)
			filteredPayload[tc.field] = tc.value
			filteredMutation, err := parseBattleSpecializationEffectMutation(filteredPayload)
			if err != nil {
				t.Fatal(err)
			}
			actor.SpecializationEffectMutations = []battleAffixEffectMutation{filteredMutation}
			filtered := s.applyBattleAffixEffectMutations(&actor, "skill", "skill:test", 1, targeting, effects)
			if len(filtered) != 1 || filtered[0].AmountFormula.BaseAdd != 20 {
				t.Fatalf("mismatched %s selector still mutated/appended effects: %#v", tc.name, filtered)
			}
		})
	}
	physicalOnly := cloneAnyMap(payload)
	physicalOnly["damageTypes"] = []any{"magical"}
	physicalOnly["ops"] = map[string]any{"cooldownDelta": -2, "cooldownFloor": 1, "mpCostDelta": 5, "mpCostMultiplier": 3}
	physicalOnlyMutation, err := parseBattleSpecializationEffectMutation(physicalOnly)
	if err != nil {
		t.Fatal(err)
	}
	actor.SpecializationEffectMutations = []battleAffixEffectMutation{physicalOnlyMutation}
	unmatchedScalar := actor.applyBattleSkillScalarMutations("skill", battleSkillState{ID: "skill:test", Category: "attack", CooldownTurns: 3, MPCost: 4, Effects: scalarEffects})
	if unmatchedScalar.CooldownTurns != 3 || unmatchedScalar.MPCost != 4 {
		t.Fatalf("damageTypes selector mutated scalar fields on a skill without a matching effect: %#v", unmatchedScalar)
	}
	bad = cloneAnyMap(payload)
	bad["skillCategories"] = "attack"
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("non-array top-level selector was accepted")
	}
	bad = cloneAnyMap(payload)
	bad["futureSelector"] = []any{"attack"}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("unknown top-level mutation selector was accepted")
	}
	bad = cloneAnyMap(payload)
	bad["ops"] = map[string]any{"appendEffects": []any{map[string]any{"kind": "consume_hp", "maxHpRatio": .03, "minimumHp": 1}}}
	if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
		t.Fatal("appended specialization effect without a stable id was accepted")
	}
	for _, targetingPatch := range []map[string]any{
		{"selectRule": "attr_max"},
		{"selectRule": "hp_min", "selectAttrKey": "strength"},
		{"targetCount": map[string]any{"base": 2, "max": 2}},
	} {
		bad = cloneAnyMap(payload)
		bad["ops"] = map[string]any{"targetingPatch": targetingPatch}
		if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
			t.Fatalf("invalid targetingPatch was accepted: %#v", targetingPatch)
		}
	}
	for _, tc := range []struct {
		kinds []any
		ops   map[string]any
	}{
		{kinds: []any{"status"}, ops: map[string]any{"amountMultiplier": 2.0}},
		{kinds: []any{"heal"}, ops: map[string]any{"hitCountDelta": 1}},
		{kinds: []any{"damage"}, ops: map[string]any{"statusIdsAppend": []any{"test_positive_buff"}}},
	} {
		bad = cloneAnyMap(payload)
		bad["effectSelector"] = map[string]any{"mode": "kind", "kinds": tc.kinds}
		bad["ops"] = tc.ops
		if _, err := parseBattleSpecializationEffectMutation(bad); err == nil {
			t.Fatalf("incompatible mutation kind/operation was accepted: kinds=%#v ops=%#v", tc.kinds, tc.ops)
		}
	}

	appendMutation, err := parseBattleSpecializationEffectMutation(map[string]any{
		"id": "mutation:append", "scope": "skill",
		"effectSelector": map[string]any{"mode": "effect_id", "ids": []any{"damage:1"}},
		"ops": map[string]any{"appendEffects": []any{map[string]any{
			"id": "generated:splash", "kind": "splash_damage", "targetCount": 1, "amountRatio": .5,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	buildMultiplier := func(includeGenerated bool) battleAffixEffectMutation {
		mutation, err := parseBattleSpecializationEffectMutation(map[string]any{
			"id": "mutation:scale", "scope": "skill",
			"effectSelector": map[string]any{"mode": "kind", "kinds": []any{"damage"}, "includeGenerated": includeGenerated},
			"ops":            map[string]any{"amountMultiplier": 2.0},
		})
		if err != nil {
			t.Fatal(err)
		}
		return mutation
	}
	for _, tc := range []struct {
		includeGenerated bool
		wantGenerated    float64
	}{{false, 10}, {true, 20}} {
		actor.SpecializationEffectMutations = []battleAffixEffectMutation{appendMutation, buildMultiplier(tc.includeGenerated)}
		generated := s.applyBattleAffixEffectMutations(&actor, "skill", "skill:test", 1, targeting, effects)
		if len(generated) != 2 || generated[0].AmountFormula.BaseAdd != 40 || generated[1].ID != "generated:splash" || generated[1].AmountFormula.BaseAdd != tc.wantGenerated {
			t.Fatalf("includeGenerated=%v selection mismatch: %#v", tc.includeGenerated, generated)
		}
	}
}

func TestBattleSpecializationMutationProvenanceReachesActionsAndTimeline(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	baseSkill := battleSkillState{
		ID: "skill:test", Category: "attack", Level: 1, Targeting: targeting,
		Effects: []battleSkillEffect{{
			ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 20},
			DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}},
		}},
	}
	parse := func(payload map[string]any, nodeID, nameKey string) battleAffixEffectMutation {
		mutation, err := parseBattleSpecializationEffectMutation(payload)
		if err != nil {
			t.Fatal(err)
		}
		mutation.SourceKind = "specialization"
		mutation.SourceID = nodeID
		mutation.SourceNameKey = nameKey
		return mutation
	}
	first := parse(map[string]any{
		"id": "mutation:first", "scope": "skill",
		"effectSelector": map[string]any{"mode": "effect_id", "ids": []any{"damage:1"}},
		"ops":            map[string]any{"amountMultiplier": 1.1},
	}, "node:first", "specialization.node.first.name")
	second := parse(map[string]any{
		"id": "mutation:second", "scope": "skill",
		"effectSelector": map[string]any{"mode": "effect_id", "ids": []any{"damage:1"}},
		"ops": map[string]any{"appendEffects": []any{map[string]any{
			"id": "mark:generated", "kind": "mark_target", "markId": "exposed", "durationTurns": 2,
		}}},
	}, "node:second", "specialization.node.second.name")
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.Skills = []battleSkillState{baseSkill}
	actor.SpecializationEffectMutations = []battleAffixEffectMutation{first, second}
	target := specializationTestUnit("enemy:target", "enemy", 1_000, 0)
	target.MaxHP = 1_000
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	execution := s.executeBattleSkill(state, time.UnixMilli(1), &state.Players[0], baseSkill, "test", nil)

	var damageMeta, markMeta map[string]any
	for _, row := range execution.Actions {
		meta := normalizeAnyMap(row["meta"])
		switch strings.TrimSpace(fmt.Sprintf("%v", row["kind"])) {
		case "damage":
			damageMeta = meta
		case "buff":
			markMeta = meta
		}
	}
	if damageMeta["nodeId"] != "node:second" || damageMeta["sourceNameKey"] != "specialization.node.second.name" {
		t.Fatalf("mutated damage did not promote a specialization source: %#v", damageMeta)
	}
	if sources := battleAnyMapSlice(damageMeta["mutationSources"]); len(sources) != 2 || sources[0]["sourceId"] != "node:first" || sources[1]["sourceId"] != "node:second" {
		t.Fatalf("mutated damage did not retain all specialization sources: %#v", damageMeta)
	}
	if markMeta["nodeId"] != "node:second" || markMeta["sourceNameKey"] != "specialization.node.second.name" {
		t.Fatalf("appended effect did not retain its specialization source: %#v", markMeta)
	}
	compact := compactBattleTimelineStepMeta(damageMeta)
	if compact["sourceKind"] != "specialization" || compact["sourceId"] != "node:second" || compact["sourceNameKey"] != "specialization.node.second.name" || compact["mutationId"] != "mutation:second" {
		t.Fatalf("timeline compaction dropped mutation provenance: %#v", compact)
	}
	if sources := battleAnyMapSlice(compact["ms"]); len(sources) != 2 || sources[0]["sourceId"] != "node:first" || sources[1]["sourceId"] != "node:second" {
		t.Fatalf("timeline compaction dropped multi-source mutation provenance: %#v", compact)
	}
}

func TestBattleSpecializationSecondaryDamageTargetsExactlyOneBoundedEnemy(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	targeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard"}, SecondarySeek: "none", TargetScope: "enemy", SelectRule: "first",
		TargetCount: battleSkillTargetCount{Base: 1},
	}
	baseSkill := battleSkillState{
		ID: "skill:secondary_target_probe", Category: "blade", Level: 1, Targeting: targeting,
		Effects: []battleSkillEffect{{
			ID: "damage:primary", Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 25},
			DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 25}, HitCount: battleSkillTargetCount{Base: 1}},
		}},
	}
	makeEnemy := func(id, position string) battleUnitState {
		enemy := specializationTestUnit(id, "enemy", 1_000, 0)
		enemy.HP = 1_000
		enemy.MaxHP = 1_000
		enemy.Position = position
		enemy.BasePosition = position
		return enemy
	}
	run := func(t *testing.T, policy, position string, enemies []battleUnitState) []string {
		t.Helper()
		appended := map[string]any{
			"id": "damage:secondary", "kind": "secondary_damage", "targetPolicy": policy, "amountRatio": .4,
		}
		if position != "" {
			appended["targetPosition"] = position
		}
		mutation, err := parseBattleSpecializationEffectMutation(map[string]any{
			"id": "mutation:secondary_target_probe", "scope": "skill",
			"weaponClasses":  []any{"scythe"},
			"damageTypes":    []any{"physical"},
			"effectSelector": map[string]any{"mode": "kind", "kinds": []any{"damage"}},
			"ops":            map[string]any{"appendEffects": []any{appended}},
		})
		if err != nil {
			t.Fatal(err)
		}
		mutation.SourceKind = "specialization"
		mutation.SourceID = "node:secondary_target_probe"
		mutation.SourceNameKey = "specialization.node.secondary_target_probe.name"
		actor := specializationTestUnit("player:actor", "player", 100, 0)
		actor.WeaponClasses = []string{"scythe"}
		actor.Skills = []battleSkillState{baseSkill}
		actor.SpecializationEffectMutations = []battleAffixEffectMutation{mutation}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: enemies}
		execution := s.executeBattleSkill(state, time.UnixMilli(1), &state.Players[0], baseSkill, "test", nil)
		targetIDs := make([]string, 0, 2)
		for _, row := range execution.Actions {
			if strings.TrimSpace(fmt.Sprintf("%v", row["kind"])) == "damage" {
				targetIDs = append(targetIDs, strings.TrimSpace(fmt.Sprintf("%v", row["targetId"])))
			}
		}
		return targetIDs
	}

	t.Run("same row excludes the primary target", func(t *testing.T) {
		got := run(t, "same_row", "", []battleUnitState{
			makeEnemy("enemy:front-primary", "vanguard"),
			makeEnemy("enemy:front-secondary", "vanguard"),
			makeEnemy("enemy:middle", "midguard"),
		})
		if !reflect.DeepEqual(got, []string{"enemy:front-primary", "enemy:front-secondary"}) {
			t.Fatalf("same-row targets=%v", got)
		}
	})

	t.Run("fixed rear row does not fall back", func(t *testing.T) {
		got := run(t, "fixed_row", "rearguard", []battleUnitState{
			makeEnemy("enemy:front", "vanguard"),
			makeEnemy("enemy:middle", "midguard"),
			makeEnemy("enemy:rear", "rearguard"),
		})
		if !reflect.DeepEqual(got, []string{"enemy:front", "enemy:rear"}) {
			t.Fatalf("fixed-row targets=%v", got)
		}
		withoutRear := run(t, "fixed_row", "rearguard", []battleUnitState{
			makeEnemy("enemy:front", "vanguard"),
			makeEnemy("enemy:middle", "midguard"),
		})
		if !reflect.DeepEqual(withoutRear, []string{"enemy:front"}) {
			t.Fatalf("fixed-row fallback escaped its row: %v", withoutRear)
		}
	})

	t.Run("all rows still adds only one distinct target", func(t *testing.T) {
		got := run(t, "all_rows", "", []battleUnitState{
			makeEnemy("enemy:front", "vanguard"),
			makeEnemy("enemy:middle", "midguard"),
			makeEnemy("enemy:rear", "rearguard"),
		})
		if len(got) != 2 || got[0] != "enemy:front" || got[1] == got[0] {
			t.Fatalf("all-row targets=%v", got)
		}
	})
}

func TestBattleSpecializationEffectIDSelectorMatchesRealEquipmentBasicAttack(t *testing.T) {
	data, err := catalog.Load("../../../packages/game-data/source/json/equipment_templates.json")
	if err != nil {
		t.Fatal(err)
	}
	s := newSpecializationExecutionTestServer()
	s.deps.GameData = data
	profile := s.resolveBattleBasicAttackProfile([]store.EquippedItem{{
		InstanceID: 1, TemplateItemID: "wood_sword", EquipType: "battle", EquippedSlot: "right_hand",
	}})
	if profile == nil || len(profile.Effects) == 0 || profile.Effects[0].ID != "equipment_wood_sword_damage" {
		t.Fatalf("real equipment basic attack did not preserve its catalog effect id: %#v", profile)
	}
	mutation, err := parseBattleSpecializationEffectMutation(map[string]any{
		"id": "mutation:wood_sword", "scope": "basic_attack",
		"weaponClasses":  []any{"sword"},
		"effectSelector": map[string]any{"mode": "effect_id", "ids": []any{"equipment_wood_sword_damage"}},
		"ops":            map[string]any{"hitCountDelta": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	mutation.SourceKind = "specialization"
	mutation.SourceID = "node:wood_sword"
	mutation.SourceNameKey = "specialization.node.wood_sword.name"
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.WeaponClasses = []string{"sword"}
	actor.BasicAttack = profile
	actor.SpecializationEffectMutations = []battleAffixEffectMutation{mutation}
	mutated := s.applyBattleAffixEffectMutations(&actor, "basic_attack", "basic_attack", 1, profile.Effects[0].Targeting, profile.Effects)
	if len(mutated) == 0 || mutated[0].ID != "equipment_wood_sword_damage" || mutated[0].DamageSpec.HitCount.Base != 2 {
		t.Fatalf("real equipment effect_id selector did not mutate the intended basic attack effect: %#v", mutated)
	}
	if len(mutated[0].MutationSources) != 1 || mutated[0].MutationSources[0].SourceID != "node:wood_sword" {
		t.Fatalf("real equipment effect_id mutation lost specialization provenance: %#v", mutated[0].MutationSources)
	}
}

func TestBattleStartSpecializationActionsBecomeTimelineNode(t *testing.T) {
	events := []battleEvent{{
		Seq: 1, Type: "battle.wave.spawn", TS: 1_000,
		Data: map[string]any{
			"wave": 1,
			"battleStartActions": []map[string]any{{
				"kind": "state_attach", "sourceId": "player:1", "targetId": "enemy:1", "buffId": "specialization:mark:hunted",
				"meta": map[string]any{"sourceKind": "specialization", "sourceId": "perception_root_hunter_mark", "nodeId": "perception_root_hunter_mark", "sourceNameKey": "specialization.node.perception_root_hunter_mark.name"},
			}},
		},
	}}
	timeline, playback := encodeBattleTimelineWithPlayback(events, 1_000, 1_000)
	if len(timeline) != 2 || len(playback) != 2 || playback[0].EventType != "battle.wave.spawn" || playback[1].EventType != "battle.timeline.node" {
		t.Fatalf("battle start actions did not produce a compatible spawn plus timeline node: timeline=%#v playback=%#v", timeline, playback)
	}
	node := normalizeAnyMap(normalizeAnyMap(timeline[1]["payload"])["data"])
	steps := battleAnyMapSlice(node["p"])
	if len(steps) != 1 {
		t.Fatalf("battle start timeline node steps=%#v, want one", steps)
	}
	meta := normalizeAnyMap(steps[0]["me"])
	if meta["sourceKind"] != "specialization" || meta["nodeId"] != "perception_root_hunter_mark" || meta["sourceNameKey"] != "specialization.node.perception_root_hunter_mark.name" {
		t.Fatalf("battle start timeline node dropped specialization source: %#v", steps[0])
	}
}

func TestBattleSpecializationMirrorAndExtraTurnRespectDispatchHooks(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}, {"id": "skill:echo"}}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	skill := battleSkillState{
		ID: "skill:echo", Name: "echo", Level: 1, MPCost: 10, Targeting: targeting,
		Effects: []battleSkillEffect{{ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting, AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, HitCount: battleSkillTargetCount{Base: 1}}}},
	}
	cycle := specializationTestHook("cycle", "onAfterDealDamage", "cycle_node", map[string]any{"kind": "cycle_counter", "counterId": "dispatch", "gain": 1, "maxStacks": 10})
	actor := specializationTestUnit("player:actor", "player", 100, 100)
	actor.UserID = 1
	actor.MP = 0 // The original cast spent the last MP before onAfterSkillCast.
	actor.Skills = []battleSkillState{skill}
	actor.SkillIDs = []string{skill.ID}
	actor.SkillSlots = []string{skill.ID}
	actor.SpecializationHooks = []battleEquipmentHookRule{cycle}
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}, Order: []string{actor.UnitID, target.UnitID}}
	mirrorHook := specializationTestHook("mirror", "onAfterSkillCast", "mirror_node", map[string]any{"kind": "mirror_cast_skill"})
	hookContext := &battleEquipmentHookContext{SkillID: skill.ID, CastKind: "skill"}
	rows, handled := s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", mirrorHook, map[string]any{
		"kind": "mirror_cast_skill", "amountRatio": .9, "applyCooldown": false, "dispatchHooks": false,
		"valueFormula": specializationTestValueFormula(.35, map[string]float64{"intelligence": .01}),
	}, hookContext)
	if !handled || len(rows) == 0 || battleSpecializationCounterStacks(&state.Players[0], "dispatch") != 0 {
		t.Fatalf("mirror dispatchHooks=false leaked hooks: rows=%#v buffs=%#v", rows, state.Players[0].Buffs)
	}
	turn := normalizeAnyMap(rows[0]["turn"])
	resultMeta := normalizeAnyMap(turn["resultMeta"])
	if ratio := toFloat64(resultMeta["amountRatio"], 0); math.Abs(ratio-.55) > .001 {
		t.Fatalf("mirror valueFormula ratio=%v, want .55 (fallback amountRatio must be ignored)", ratio)
	}
	if state.Players[0].MP != 0 || toFloat64(resultMeta["mpCostMultiplier"], -1) != 0 {
		t.Fatalf("specialization mirror charged MP after the original cast exhausted it: mp=%v meta=%#v", state.Players[0].MP, resultMeta)
	}
	_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(2), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", mirrorHook, map[string]any{
		"kind": "mirror_cast_skill", "amountRatio": .5, "applyCooldown": false, "dispatchHooks": true,
	}, hookContext)
	if battleSpecializationCounterStacks(&state.Players[0], "dispatch") != 1 {
		t.Fatalf("mirror dispatchHooks=true did not dispatch damage hooks: %#v", state.Players[0].Buffs)
	}

	battleSetSpecializationCounter(&state.Players[0], "dispatch", 0)
	state.Players[0].MP = 100
	extraHook := specializationTestHook("extra", "onKill", "extra_node", map[string]any{"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false})
	if _, ok := s.executeEquipmentExtraTurn(state, time.UnixMilli(3), &state.Players[0], "onKill", extraHook, &state.Enemies[0], "full", false); !ok {
		t.Fatal("extra turn with dispatchHooks=false was not queued")
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(4), nil); err != nil {
		t.Fatalf("process dispatchHooks=false extra turn: %v", err)
	}
	if battleSpecializationCounterStacks(&state.Players[0], "dispatch") != 0 {
		t.Fatalf("extra turn dispatchHooks=false leaked hooks: %#v", state.Players[0].Buffs)
	}
	if _, ok := s.executeEquipmentExtraTurn(state, time.UnixMilli(5), &state.Players[0], "onKill", extraHook, &state.Enemies[0], "full", true); !ok {
		t.Fatal("extra turn with dispatchHooks=true was not queued")
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(6), nil); err != nil {
		t.Fatalf("process dispatchHooks=true extra turn: %v", err)
	}
	if battleSpecializationCounterStacks(&state.Players[0], "dispatch") != 1 {
		t.Fatalf("extra turn dispatchHooks=true did not dispatch hooks: %#v", state.Players[0].Buffs)
	}
}

func TestBattleSpecializationThreeStepCounterGrantsOneFullExtraAction(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	skill := battleSkillState{ID: "skill:turn", Level: 1, Targeting: targeting, Effects: []battleSkillEffect{{
		ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, HitCount: battleSkillTargetCount{Base: 1}},
	}}}
	actor := specializationTestUnit("player:actor", "player", 100, 100)
	actor.Skills = []battleSkillState{skill}
	actor.SkillIDs = []string{skill.ID}
	actor.SkillSlots = []string{skill.ID}
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	hook := specializationTestHook("tempo_cycle", "onAfterSkillCast", "tempo_node")
	effect := map[string]any{
		"kind": "cycle_counter", "counterId": "tempo", "gain": 1, "maxStacks": 3,
		"atMax": map[string]any{"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false, "consumeStacks": 3},
	}
	if err := validateBattleSpecializationHookEffect(effect); err != nil {
		t.Fatalf("valid three-step extra action cycle was rejected: %v", err)
	}
	for step := 1; step <= 3; step++ {
		rows, handled := s.executeBattleSpecializationHookEffect(state, time.UnixMilli(int64(step)), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", hook, effect, &battleEquipmentHookContext{SkillID: skill.ID})
		if !handled {
			t.Fatalf("counter step %d was not handled", step)
		}
		payload, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		extraCount := strings.Count(string(payload), `"kind":"extra_turn"`)
		if step < 3 && extraCount != 0 {
			t.Fatalf("counter granted an early extra action at step %d: %#v", step, rows)
		}
		if step == 3 && extraCount != 1 {
			t.Fatalf("counter did not grant exactly one extra action at step 3: %#v", rows)
		}
	}
	if got := battleSpecializationCounterStacks(&state.Players[0], "tempo"); got != 0 {
		t.Fatalf("counter stacks=%d after consuming the extra action, want 0", got)
	}
}

func TestBattleSpecializationSourceFirstEchoFailureDoesNotConsumeWaveLimit(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}, {"id": "skill:echo"}}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	echoSkill := battleSkillState{
		ID: "skill:echo", Name: "echo", Level: 1, MPCost: 10, Targeting: targeting,
		Effects: []battleSkillEffect{{ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting, AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 10}, HitCount: battleSkillTargetCount{Base: 1}}}},
	}
	actor := specializationTestUnit("player:actor", "player", 100, 75)
	actor.WeaponClasses = []string{"mind_orb"}
	actor.SpecializationHooks = specializationTestSourceHooks(t, "intelligence_terminal_first_echo")
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	context := &battleEquipmentHookContext{SkillID: echoSkill.ID, SkillCategory: "magic", CastKind: "skill"}

	failedRows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", context)
	if len(failedRows) != 0 {
		t.Fatalf("missing echo skill unexpectedly settled: %#v", failedRows)
	}
	state.Players[0].Skills = []battleSkillState{echoSkill}
	state.Players[0].SkillIDs = []string{echoSkill.ID}
	state.Players[0].SkillSlots = []string{echoSkill.ID}
	state.Players[0].MP = 75
	successRows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(2), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", context)
	if len(successRows) == 0 || state.Enemies[0].HP >= 100 || state.Players[0].MP != 75 {
		t.Fatalf("failed preview consumed first_echo or free echo did not settle: rows=%#v actor=%#v enemy=%#v", successRows, state.Players[0], state.Enemies[0])
	}
	thirdRows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(3), &state.Players[0], nil, &state.Enemies[0], "onAfterSkillCast", 0, "", context)
	if len(thirdRows) != 0 {
		t.Fatalf("successful first_echo did not consume its per-wave limit: %#v", thirdRows)
	}
}

func TestBattleSpecializationProbabilisticExtraTurnGatesBenefitsAndConsumesLimitOnlyOnSuccess(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	targeting := battleSkillTargeting{TargetPositions: []string{"vanguard"}, SecondarySeek: "front_to_rear", TargetScope: "enemy", SelectRule: "first", TargetCount: battleSkillTargetCount{Base: 1}}
	basic := battleSkillState{
		ID: "basic_attack", Name: "basic", Level: 1, Targeting: targeting,
		Effects: []battleSkillEffect{{
			ID: "damage:1", Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 5},
			DamageSpec:    battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 5}, HitCount: battleSkillTargetCount{Base: 1}},
		}},
	}
	cooling := battleSkillState{ID: "skill:cooling", Name: "cooling", Level: 1, CooldownRemain: 3}
	extraTurn := map[string]any{
		"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false,
		"procChanceBase": 0.0, "procChanceMax": 1.0, "abortHookOnFail": true,
	}
	hook := specializationTestHook(
		"chance_extra", "onKill", "chance_extra_node",
		extraTurn,
		map[string]any{"kind": "adjust_skill_cooldown", "targetMode": "self", "amount": -1, "selectionMode": "max_remaining", "maxSkills": 1},
	)
	hook.MaxTriggersPerWave = 1
	actor := specializationTestUnit("player:actor", "player", 100, 100)
	actor.UserID = 1
	actor.Skills = []battleSkillState{basic, cooling}
	actor.SkillIDs = []string{basic.ID, cooling.ID}
	actor.SkillSlots = []string{basic.ID, cooling.ID}
	actor.SpecializationHooks = []battleEquipmentHookRule{hook}
	deadTarget := specializationTestUnit("enemy:defeated", "enemy", 0, 0)
	aliveTarget := specializationTestUnit("enemy:next", "enemy", 100, 0)
	state := &battleRuntimeState{Seed: 7, Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{deadTarget, aliveTarget}, Order: []string{actor.UnitID, aliveTarget.UnitID}}

	failedRows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], &state.Enemies[0], "onKill", 0, "", nil)
	if len(failedRows) != 0 || state.Players[0].Skills[1].CooldownRemain != 3 || len(state.SpecializationHookWaveCounts) != 0 {
		t.Fatalf("failed extra-action roll applied benefits or consumed its wave limit: rows=%#v actor=%#v counts=%#v", failedRows, state.Players[0], state.SpecializationHookWaveCounts)
	}

	extraTurn["procChanceBase"] = 1.0
	extraTurn["procChanceMax"] = 1.0
	successRows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(2), &state.Players[0], &state.Enemies[0], &state.Enemies[0], "onKill", 0, "", nil)
	payload, err := json.Marshal(successRows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"kind":"extra_turn"`) || !strings.Contains(string(payload), `"kind":"skill_cooldown"`) || state.Players[0].Skills[1].CooldownRemain != 2 || len(state.ExtraTurnQueue) != 1 || len(state.SpecializationHookWaveCounts) != 1 {
		t.Fatalf("successful extra-action roll did not queue its formal turn and immediate benefit exactly once: rows=%#v actor=%#v queue=%#v counts=%#v", successRows, state.Players[0], state.ExtraTurnQueue, state.SpecializationHookWaveCounts)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(3), nil); err != nil {
		t.Fatalf("process probabilistic formal extra turn: %v", err)
	}
	if state.Players[0].Skills[1].CooldownRemain != 1 || len(state.ExtraTurnQueue) != 0 || !parseBooleanLoose(normalizeAnyMap(lastBattleTurnActionData(t, state)["resultMeta"])["extraTurn"], false) {
		t.Fatalf("formal extra turn did not tick cooldown as an independent turn: actor=%#v queue=%#v event=%#v", state.Players[0], state.ExtraTurnQueue, lastBattleTurnActionData(t, state))
	}
	thirdRows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(4), &state.Players[0], &state.Enemies[0], &state.Enemies[0], "onKill", 0, "", nil)
	if len(thirdRows) != 0 || state.Players[0].Skills[1].CooldownRemain != 1 {
		t.Fatalf("successful roll did not lock the specialization for the rest of the wave: rows=%#v actor=%#v", thirdRows, state.Players[0])
	}
}

func TestBattleSpecializationRemainingHookPrimitivesSettle(t *testing.T) {
	t.Run("direct helper effects", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		self := specializationTestUnit("player:self", "player", 50, 20)
		ally := specializationTestUnit("player:ally", "player", 25, 0)
		ally.Position = "rearguard"
		enemy := specializationTestUnit("enemy:source", "enemy", 100, 0)
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self, ally}, Enemies: []battleUnitState{enemy}}
		selfPtr, allyPtr, enemyPtr := &state.Players[0], &state.Players[1], &state.Enemies[0]
		hook := specializationTestHook("direct", "onTurnStart", "direct_node", map[string]any{"kind": "heal"})

		rows, handled := s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), selfPtr, enemyPtr, enemyPtr, "onTurnStart", 20, "physical", hook, map[string]any{"kind": "action_meter_gain", "value": .25}, nil)
		if !handled || len(rows) == 0 || toFloat64(findBuffByID(selfPtr, "specialization:action_meter").Payload["value"], 0) != .25 {
			t.Fatalf("action meter did not settle: %#v %#v", rows, selfPtr.Buffs)
		}
		battleApplyOrRefreshBuff(enemyPtr, battleSpecializationMarkBuffID("exposed"), 2, nil, selfPtr.UnitID, "refresh_duration")
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(2), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 20, "physical", hook, map[string]any{"kind": "consume_mark", "markId": "exposed"}, nil)
		if battleHasBuff(enemyPtr, battleSpecializationMarkBuffID("exposed")) {
			t.Fatal("consume_mark did not remove the event target mark")
		}
		selfPtr.MP = 20
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(3), selfPtr, enemyPtr, enemyPtr, "onAfterDealDamage", 0, "", hook, map[string]any{"kind": "restore_mp", "maxMpRatio": .1}, nil)
		if selfPtr.MP != 30 {
			t.Fatalf("restore_mp=%v, want 30", selfPtr.MP)
		}
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(4), selfPtr, enemyPtr, selfPtr, "onTurnEnd", 0, "", hook, map[string]any{"kind": "heal", "targetMode": "self", "maxHpRatio": .1}, nil)
		if selfPtr.HP != 60 {
			t.Fatalf("heal=%v, want 60", selfPtr.HP)
		}
		beforeEnemyHP := enemyPtr.HP
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(5), selfPtr, enemyPtr, selfPtr, "onAfterTakeDamage", 10, "physical", hook, map[string]any{"kind": "retaliate", "targetMode": "source", "strengthRatio": .35, "staminaRatio": .35, "dispatchHooks": false}, nil)
		if enemyPtr.HP >= beforeEnemyHP {
			t.Fatal("retaliate did not deal authoritative damage")
		}
		selfPtr.Position = "vanguard"
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(6), selfPtr, nil, allyPtr, "onTurnStart", 0, "", hook, map[string]any{"kind": "shift_position", "maxSteps": 1}, nil)
		if selfPtr.Position != "midguard" {
			t.Fatalf("shift_position=%q, want midguard", selfPtr.Position)
		}
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(7), selfPtr, nil, allyPtr, "onTurnStart", 0, "", hook, map[string]any{"kind": "shared_damage_reduction", "value": .08, "durationTurns": 1}, nil)
		if findBuffByID(selfPtr, "specialization:damage_reduction:"+hook.ID) == nil || findBuffByID(allyPtr, "specialization:damage_reduction:"+hook.ID) == nil {
			t.Fatalf("shared reduction did not attach to both units: self=%#v ally=%#v", selfPtr.Buffs, allyPtr.Buffs)
		}
	})

	t.Run("generic hook effects and judgement target", func(t *testing.T) {
		s := newSpecializationExecutionTestServer()
		self := specializationTestUnit("player:self", "player", 100, 0)
		enemy := specializationTestUnit("enemy:target", "enemy", 100, 0)
		self.Skills = []battleSkillState{{ID: "skill:a", CooldownRemain: 3}, {ID: "skill:b", CooldownRemain: 1}}
		battleApplyOrRefreshBuff(&enemy, "test_positive_buff", 3, nil, enemy.UnitID, "refresh_duration")
		self.SpecializationHooks = []battleEquipmentHookRule{
			specializationTestHook("cooldown", "onAfterDealDamage", "cooldown_node", map[string]any{"kind": "adjust_skill_cooldown", "amount": -1, "selectionMode": "max_remaining", "maxSkills": 1}),
			specializationTestHook("damage", "onAfterDealDamage", "damage_node", map[string]any{"kind": "damage", "targetMode": "targeting", "damageType": "physical", "dispatchHooks": false, "amountFormula": map[string]any{"baseAdd": 20}}),
			specializationTestHook("judgement", "onAfterDealDamage", "judgement_node", map[string]any{"kind": "remove_buff_by_kind", "buffKind": "buff", "selectionMode": "max_remaining", "maxBuffs": 1}),
		}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}, Enemies: []battleUnitState{enemy}}
		rows := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], &state.Enemies[0], "onAfterDealDamage", 10, "physical", &battleEquipmentHookContext{SkillID: "skill:a"})
		if state.Players[0].Skills[0].CooldownRemain != 2 || state.Enemies[0].HP >= 100 || battleHasBuff(&state.Enemies[0], "test_positive_buff") {
			t.Fatalf("generic hook primitives did not settle: self=%#v enemy=%#v rows=%#v", state.Players[0], state.Enemies[0], rows)
		}
		if battleHasBuff(&state.Players[0], "test_positive_buff") {
			t.Fatal("remove_buff_by_kind incorrectly targeted the attacker")
		}
	})
}

func TestBattleSpecializationMarkCycleAlternatesRealHits(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	actor.SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("observation", "onAfterDealDamage", "observation_node", map[string]any{
			"kind": "mark_cycle", "markId": "gap", "firstHit": "apply_mark",
			"markedHit": map[string]any{"kind": "runtime_damage_amp", "value": .9, "consumeMark": true, "valueFormula": specializationTestValueFormula(.06, map[string]float64{"perception": .002})},
		}),
	}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	first := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)
	first.skillID = "skill:test"
	first.ApplyDamage(battleskills.DamageAction{SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 10, DamageType: "physical"})
	if !battleHasBuff(&state.Enemies[0], battleSpecializationMarkBuffID("gap")) {
		t.Fatal("first hit did not apply observation mark")
	}
	second := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(2), s, nil)
	second.skillID = "skill:test"
	effect := second.ApplyDamage(battleskills.DamageAction{SourceID: actor.UnitID, TargetID: target.UnitID, Amount: 10, DamageType: "physical"})
	if battleHasBuff(&state.Enemies[0], battleSpecializationMarkBuffID("gap")) || toFloat64(effect.Meta["runtimeDamageAmp"], 0) < .099 {
		t.Fatalf("marked hit did not consume mark and amplify: buffs=%#v effect=%#v", state.Enemies[0].Buffs, effect)
	}
}

func TestBattleSpecializationValueFormulaDirectAndNestedPayoffsExecute(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	hook := specializationTestHook("formula", "onTurnStart", "formula_node")

	t.Run("direct action meter", func(t *testing.T) {
		self := specializationTestUnit("player:self", "player", 100, 0)
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}}
		rows, handled := s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", hook, map[string]any{
			"kind": "action_meter_gain", "value": .9,
			"valueFormula": specializationTestValueFormula(.03, map[string]float64{"dexterity": .001}),
		}, nil)
		buff := findBuffByID(&state.Players[0], "specialization:action_meter")
		if !handled || len(rows) == 0 || buff == nil || math.Abs(toFloat64(buff.Payload["value"], 0)-.05) > .001 {
			t.Fatalf("action meter formula did not override fallback: rows=%#v buffs=%#v", rows, state.Players[0].Buffs)
		}
	})

	t.Run("empower next hit", func(t *testing.T) {
		self := specializationTestUnit("player:self", "player", 100, 0)
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}}
		effect := map[string]any{
			"kind": "cycle_counter", "counterId": "strength", "gain": 1, "maxStacks": 1,
			"atMax": map[string]any{"kind": "empower_next_hit", "damageAmp": .9, "consumeStacks": 1, "valueFormula": specializationTestValueFormula(.09, map[string]float64{"strength": .003})},
		}
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", hook, effect, nil)
		buff := findBuffByID(&state.Players[0], "specialization:empower:strength")
		if buff == nil || math.Abs(toFloat64(buff.Payload["damageAmp"], 0)-.15) > .001 {
			t.Fatalf("empower formula did not settle: %#v", state.Players[0].Buffs)
		}
	})

	t.Run("action meter terminal", func(t *testing.T) {
		self := specializationTestUnit("player:self", "player", 100, 0)
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}}
		effect := map[string]any{
			"kind": "cycle_counter", "counterId": "tempo", "gain": 1, "maxStacks": 1,
			"atMax": map[string]any{"kind": "action_meter_gain", "value": .9, "consumeStacks": 1, "valueFormula": specializationTestValueFormula(.2, map[string]float64{"dexterity": .005})},
		}
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", hook, effect, nil)
		buff := findBuffByID(&state.Players[0], "specialization:action_meter")
		if buff == nil || math.Abs(toFloat64(buff.Payload["value"], 0)-.3) > .001 {
			t.Fatalf("action meter terminal formula did not settle: %#v", state.Players[0].Buffs)
		}
	})

	t.Run("restore and cooldown terminal", func(t *testing.T) {
		self := specializationTestUnit("player:self", "player", 100, 0)
		self.Skills = []battleSkillState{{ID: "skill:test", CooldownRemain: 2}}
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}}
		effect := map[string]any{
			"kind": "cycle_counter", "counterId": "pattern", "gain": 1, "maxStacks": 1,
			"atMax": map[string]any{"kind": "restore_mp_and_tick_cooldown", "mpRatio": .9, "cooldownAmount": -1, "consumeStacks": 1, "valueFormula": specializationTestValueFormula(.04, map[string]float64{"intelligence": .002})},
		}
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", hook, effect, nil)
		if math.Abs(state.Players[0].MP-8) > .001 || state.Players[0].Skills[0].CooldownRemain != 1 {
			t.Fatalf("restore/cooldown terminal did not settle: mp=%v skills=%#v", state.Players[0].MP, state.Players[0].Skills)
		}
	})

	t.Run("guard lowest ally terminal", func(t *testing.T) {
		self := specializationTestUnit("player:self", "player", 100, 0)
		ally := specializationTestUnit("player:ally", "player", 25, 0)
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self, ally}}
		effect := map[string]any{
			"kind": "cycle_counter", "counterId": "guard", "gain": 1, "maxStacks": 1,
			"atMax": map[string]any{"kind": "guard_lowest_hp_ally", "damageReduction": .9, "durationTurns": 1, "consumeStacks": 1, "valueFormula": specializationTestValueFormula(.08, map[string]float64{"stamina": .002})},
		}
		_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", hook, effect, nil)
		buff := findBuffByID(&state.Players[1], "specialization:damage_reduction:"+hook.ID)
		if buff == nil || math.Abs(toFloat64(buff.Payload["value"], 0)-.12) > .001 {
			t.Fatalf("guard terminal formula did not settle: %#v", state.Players[1].Buffs)
		}
	})
}

func TestBattleSpecializationValueFormulaRatiosClampAndSettle(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	hook := specializationTestHook("formula", "onTurnStart", "formula_node")

	self := specializationTestUnit("player:self", "player", 50, 0)
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{self}}
	selfPtr := &state.Players[0]
	_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(1), selfPtr, nil, selfPtr, "onTurnStart", 0, "", hook, map[string]any{
		"kind": "restore_mp", "maxMpRatio": .9,
		"valueFormula": specializationTestValueFormula(.01, map[string]float64{"intelligence": .001}),
	}, nil)
	if math.Abs(selfPtr.MP-3) > .001 {
		t.Fatalf("restore_mp formula=%v, want 3", selfPtr.MP)
	}
	_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(2), selfPtr, nil, selfPtr, "onTurnEnd", 0, "", hook, map[string]any{
		"kind": "heal", "targetMode": "self", "missingHpRatio": .9,
		"valueFormula": specializationTestValueFormula(.02, map[string]float64{"stamina": .001}),
	}, nil)
	if math.Abs(selfPtr.HP-52) > .001 {
		t.Fatalf("missing-health heal formula=%v, want 52", selfPtr.HP)
	}

	context := &battleEquipmentHookContext{SkillID: "skill:debt", MPCost: 10, MPInsufficient: true}
	beforeHP := selfPtr.HP
	_, _ = s.executeBattleSpecializationHookEffect(state, time.UnixMilli(3), selfPtr, nil, nil, "onBeforeSkillCast", 0, "", hook, map[string]any{
		"kind": "resource_debt_cast", "hpCostRatio": .9, "minimumHp": 1,
		"valueFormula": specializationTestValueFormula(.13, map[string]float64{"intelligence": -.0005}),
	}, context)
	if !context.AllowDebtCast || math.Abs((beforeHP-selfPtr.HP)-12) > .001 {
		t.Fatalf("resource debt formula did not settle: hp=%v context=%#v", selfPtr.HP, context)
	}
	debtEffect := map[string]any{
		"kind": "resource_debt_cast", "hpCostRatio": .9, "minimumHp": 1,
		"valueFormula": specializationTestValueFormula(.13, map[string]float64{"intelligence": -.0005}),
	}
	debtHook := specializationTestHook("debt", "onBeforeSkillCast", "debt_node", debtEffect)
	debtHook.Conditions = []map[string]any{{"kind": "context_mp_insufficient"}}
	debtUnit := specializationTestUnit("player:debt", "player", 13, 0)
	debtUnit.SpecializationHooks = []battleEquipmentHookRule{debtHook}
	debtState := &battleRuntimeState{Wave: 1, Players: []battleUnitState{debtUnit}}
	debtSkill := battleSkillState{ID: "skill:debt", MPCost: 10}
	if !s.battleCanPaySpecializationResourceDebt(debtState, &debtState.Players[0], debtSkill) {
		t.Fatal("resource-debt preflight ignored the high-intelligence valueFormula discount")
	}
	debtState.Players[0].Stats["intelligence"] = 0
	if s.battleCanPaySpecializationResourceDebt(debtState, &debtState.Players[0], debtSkill) {
		t.Fatal("resource-debt preflight selected a skill whose formula cost cannot preserve minimum HP")
	}

	target := specializationTestUnit("player:target", "player", 100, 100)
	protector := specializationTestUnit("player:protector", "player", 100, 0)
	source := specializationTestUnit("enemy:source", "enemy", 100, 0)
	protector.SpecializationHooks = []battleEquipmentHookRule{specializationTestHook("redirect", "onAllyBeforeTakeDamage", "redirect_node", map[string]any{
		"kind": "redirect_damage", "targetMode": "self", "ratio": .9,
		"valueFormula": specializationTestValueFormula(.08, map[string]float64{"stamina": .002}),
	})}
	target.SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("shield", "onBeforeTakeDamage", "shield_node", map[string]any{
			"kind": "resource_shield", "resource": "mp", "absorbRatio": .9, "resourcePerDamage": 1,
			"valueFormula": specializationTestValueFormula(.2, map[string]float64{"intelligence": .0025, "stamina": .0025}),
		}),
		specializationTestHook("last", "onBeforeTakeDamage", "last_node", map[string]any{
			"kind": "prevent_lethal", "remainingHpRatio": .9,
			"valueFormula": specializationTestValueFormula(.1, map[string]float64{"stamina": .0025}),
		}),
	}
	damageState := &battleRuntimeState{Wave: 1, Players: []battleUnitState{target, protector}, Enemies: []battleUnitState{source}}
	result := s.battleApplySpecializationPreDamageAdjustments(damageState, time.UnixMilli(4), &damageState.Players[0], &damageState.Enemies[0], 200, "physical", &battleEquipmentHookContext{})
	if math.Abs(damageState.Players[1].HP-76) > .001 || math.Abs(damageState.Players[0].MP-47.2) > .001 || math.Abs(result.Amount-85) > .001 {
		t.Fatalf("ratio formulas did not settle in order: protectorHP=%v targetMP=%v amount=%v", damageState.Players[1].HP, damageState.Players[0].MP, result.Amount)
	}

	clamped := specializationTestUnit("player:clamped", "player", 100, 0)
	clampState := &battleRuntimeState{Wave: 1, Players: []battleUnitState{clamped}}
	_, _ = s.executeBattleSpecializationHookEffect(clampState, time.UnixMilli(5), &clampState.Players[0], nil, nil, "onTurnStart", 0, "", hook, map[string]any{
		"kind": "restore_mp", "maxMpRatio": .1, "valueFormula": specializationTestValueFormula(2, map[string]float64{"strength": .001}),
	}, nil)
	if clampState.Players[0].MP != clampState.Players[0].MaxMP {
		t.Fatalf("ratio above one was not clamped: mp=%v", clampState.Players[0].MP)
	}
	clampState.Players[0].Stats["intelligence"] = 400
	negativeContext := &battleEquipmentHookContext{SkillID: "skill:debt", MPInsufficient: true}
	rows, _ := s.executeBattleSpecializationHookEffect(clampState, time.UnixMilli(6), &clampState.Players[0], nil, nil, "onBeforeSkillCast", 0, "", hook, map[string]any{
		"kind": "resource_debt_cast", "hpCostRatio": .9, "minimumHp": 1,
		"valueFormula": specializationTestValueFormula(.13, map[string]float64{"intelligence": -.0005}),
	}, negativeContext)
	if negativeContext.AllowDebtCast || len(rows) != 0 {
		t.Fatalf("negative ratio must clamp to zero rather than pay or heal: rows=%#v context=%#v", rows, negativeContext)
	}
}

func TestBattleSpecializationValueFormulaValidationFailsClosed(t *testing.T) {
	badFormula := map[string]any{"baseAdd": 0, "terms": []any{map[string]any{"attributeKey": "strength"}}}
	if err := validateBattleSpecializationHookEffect(map[string]any{"kind": "runtime_damage_amp", "valueFormula": badFormula}); err == nil {
		t.Fatal("malformed valueFormula coefficient was accepted")
	}
	if err := validateBattleSpecializationHookEffect(map[string]any{
		"kind": "cycle_counter", "valueFormula": specializationTestValueFormula(1, nil),
		"counterId": "integer_counter", "gain": 1, "maxStacks": 3,
	}); err == nil {
		t.Fatal("valueFormula was allowed to target integer cycle-counter fields")
	}
	for _, tc := range []struct {
		name    string
		formula map[string]any
	}{
		{name: "empty terms", formula: map[string]any{"baseAdd": 0, "terms": []any{}}},
		{name: "zero coefficient", formula: map[string]any{"baseAdd": 0, "terms": []any{map[string]any{"attributeKey": "strength", "coefficient": 0}}}},
		{name: "unknown attribute", formula: map[string]any{"baseAdd": 0, "terms": []any{map[string]any{"attributeKey": "strenght", "coefficient": .1}}}},
		{name: "legacy term key", formula: map[string]any{"baseAdd": 0, "terms": []any{map[string]any{"attrKey": "strength", "coefficient": .1}}}},
		{name: "unknown formula field", formula: map[string]any{"baseAdd": 0, "terms": []any{map[string]any{"attributeKey": "strength", "coefficient": .1}}, "clamp": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateBattleSpecializationHookEffect(map[string]any{"kind": "runtime_damage_amp", "valueFormula": tc.formula}); err == nil {
				t.Fatalf("malformed valueFormula was accepted: %#v", tc.formula)
			}
		})
	}
}

func TestBattleSpecializationDispatchHooksShapeFailsClosed(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	base := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Hooks: []store.SpecializationRuntimeGrant{{
		ID: "tree:node:grant_hook:hook", SourceNodeID: "node", SourceNameKey: "specialization.node.name",
		Payload: map[string]any{
			"id": "hook", "trigger": "onAfterDealDamage", "dispatchHooks": false,
			"effects": []any{map[string]any{"kind": "damage", "targetMode": "targeting", "damageType": "physical", "dispatchHooks": false, "amountFormula": map[string]any{"baseAdd": 1}}},
		},
	}}}
	missingHook := base
	missingHook.Hooks = append([]store.SpecializationRuntimeGrant(nil), base.Hooks...)
	missingHook.Hooks[0].Payload = cloneAnyMap(base.Hooks[0].Payload)
	delete(missingHook.Hooks[0].Payload, "dispatchHooks")
	if _, err := s.resolveBattleSpecializationHooks(missingHook); err == nil {
		t.Fatal("hook-level dispatchHooks omission was accepted")
	}
	nonBooleanHook := base
	nonBooleanHook.Hooks = append([]store.SpecializationRuntimeGrant(nil), base.Hooks...)
	nonBooleanHook.Hooks[0].Payload = cloneAnyMap(base.Hooks[0].Payload)
	nonBooleanHook.Hooks[0].Payload["dispatchHooks"] = "false"
	if _, err := s.resolveBattleSpecializationHooks(nonBooleanHook); err == nil {
		t.Fatal("non-boolean hook-level dispatchHooks was accepted")
	}
	missingEffect := base
	missingEffect.Hooks = append([]store.SpecializationRuntimeGrant(nil), base.Hooks...)
	missingEffect.Hooks[0].Payload = cloneAnyMap(base.Hooks[0].Payload)
	missingEffect.Hooks[0].Payload["effects"] = []any{map[string]any{"kind": "damage", "targetMode": "targeting", "damageType": "physical", "amountFormula": map[string]any{"baseAdd": 1}}}
	if _, err := s.resolveBattleSpecializationHooks(missingEffect); err == nil {
		t.Fatal("recursive effect dispatchHooks omission was accepted")
	}
}

func TestBattleSpecializationUnknownDSLFieldsFailClosed(t *testing.T) {
	for _, effect := range []map[string]any{
		{"kind": "heal", "targetMode": "self", "maxHpRatio": .1, "futureField": true},
		{"kind": "damage", "targetMode": "targeting", "damageType": "physical", "amountFormula": map[string]any{"baseAdd": 1, "terms": []any{map[string]any{"attributeKey": "strength", "coefficient": .1}}}, "dispatchHooks": false, "amountForula": map[string]any{}},
	} {
		if err := validateBattleSpecializationHookEffect(effect); err == nil {
			t.Fatalf("specialization effect with unknown field was accepted: %#v", effect)
		}
	}
	s := newSpecializationExecutionTestServer()
	base := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Hooks: []store.SpecializationRuntimeGrant{{
		ID: "tree:strict_node:strict_grant:strict_hook", SourceNodeID: "strict_node", SourceNameKey: "specialization.node.strict.name",
		Payload: map[string]any{
			"id": "strict_hook", "trigger": "onTurnStart", "priority": 0, "dispatchHooks": false,
			"conditions": []any{map[string]any{"kind": "self_hp_ratio_lte", "value": .5}},
			"effects":    []any{map[string]any{"kind": "action_meter_gain", "value": .1}},
		},
	}}}
	for _, mutate := range []func(map[string]any){
		func(hook map[string]any) { hook["futureHookField"] = true },
		func(hook map[string]any) {
			hook["conditions"] = []any{map[string]any{"kind": "self_hp_ratio_lte", "value": .5, "ration": .4}}
		},
		func(hook map[string]any) {
			hook["subjectFilter"] = map[string]any{"requireAlive": true, "futureFilter": true}
		},
	} {
		bad := base
		bad.Hooks = append([]store.SpecializationRuntimeGrant(nil), base.Hooks...)
		bad.Hooks[0].Payload = cloneAnyMap(base.Hooks[0].Payload)
		mutate(bad.Hooks[0].Payload)
		if _, err := s.resolveBattleSpecializationHooks(bad); err == nil {
			t.Fatalf("specialization hook with unknown DSL field was accepted: %#v", bad.Hooks[0].Payload)
		}
	}
}

func TestBattleSpecializationTopLevelLimitsCompileAndExecute(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	contribution := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Hooks: []store.SpecializationRuntimeGrant{{
		ID: "tree:limited_node:limited_grant:limited", SourceNodeID: "limited_node", SourceNameKey: "specialization.node.limited.name",
		Payload: map[string]any{
			"id": "limited", "trigger": "onTurnStart", "dispatchHooks": false,
			"limits":     map[string]any{"perTurn": 1, "perWave": 2, "perBattle": 3},
			"conditions": []map[string]any{{"kind": "trigger_limit", "perTurn": 2}},
			"effects":    []map[string]any{{"kind": "action_meter_gain", "value": .1}},
		},
	}}}
	hooks, err := s.resolveBattleSpecializationHooks(contribution)
	if err != nil || len(hooks) != 1 {
		t.Fatalf("top-level limits did not compile: hooks=%#v err=%v", hooks, err)
	}
	if hooks[0].MaxTriggersPerTurn != 1 || hooks[0].MaxTriggersPerWave != 2 || hooks[0].MaxTriggersPerBattle != 3 {
		t.Fatalf("limits did not merge to the strictest values: %#v", hooks[0])
	}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.SpecializationHooks = hooks
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
	first := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(1), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
	second := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(2), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
	if len(first) == 0 || len(second) != 0 {
		t.Fatalf("compiled per-turn limit did not execute authoritatively: first=%#v second=%#v", first, second)
	}
	state.beginBattleHookTurn()
	waveSecond := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(3), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
	state.beginBattleHookTurn()
	waveOverflow := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(4), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
	if len(waveSecond) == 0 || len(waveOverflow) != 0 {
		t.Fatalf("compiled per-wave limit did not execute authoritatively: second=%#v overflow=%#v", waveSecond, waveOverflow)
	}
	state.Wave = 2
	state.SpecializationHookWaveCounts = map[string]int{}
	state.beginBattleHookTurn()
	battleThird := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(5), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
	state.beginBattleHookTurn()
	battleOverflow := s.battleTriggerUnitEquipmentHooksWithContext(state, time.UnixMilli(6), &state.Players[0], nil, nil, "onTurnStart", 0, "", nil)
	if len(battleThird) == 0 || len(battleOverflow) != 0 {
		t.Fatalf("compiled per-battle limit did not execute authoritatively: third=%#v overflow=%#v", battleThird, battleOverflow)
	}

	bad := contribution
	bad.Hooks = append([]store.SpecializationRuntimeGrant(nil), contribution.Hooks...)
	bad.Hooks[0].Payload = cloneAnyMap(contribution.Hooks[0].Payload)
	bad.Hooks[0].Payload["limits"] = map[string]any{"perTarget": 1}
	if _, err := s.resolveBattleSpecializationHooks(bad); err == nil {
		t.Fatal("unsupported perTarget limit was silently accepted")
	}
}

func TestBattleSpecializationEventFiltersExecuteAndFailClosed(t *testing.T) {
	self := specializationTestUnit("player:self", "player", 100, 0)
	subject := specializationTestUnit("enemy:subject", "enemy", 100, 0)
	context := &battleEquipmentHookContext{
		SkillID: "skill:fire", SkillCategory: "magic", CastKind: "skill",
		DamageType: "magical", IsBasicAttack: false,
	}
	cases := []struct {
		name   string
		filter map[string]any
		want   bool
	}{
		{"included skill", map[string]any{"skillIds": []any{"skill:fire"}}, true},
		{"other skill", map[string]any{"skillIds": []any{"skill:ice"}}, false},
		{"excluded skill", map[string]any{"excludeSkillIds": []any{"skill:fire"}}, false},
		{"category", map[string]any{"skillCategories": []any{"magic"}}, true},
		{"other category", map[string]any{"skillCategories": []any{"physical"}}, false},
		{"damage type", map[string]any{"damageTypes": []any{"magical"}}, true},
		{"other damage type", map[string]any{"damageTypes": []any{"physical"}}, false},
		{"enemy subject", map[string]any{"unitTypes": []any{"enemy"}}, true},
		{"player subject", map[string]any{"unitTypes": []any{"player"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hook := specializationTestHook("filter", "onBeforeDealDamage", "filter_node")
			hook.EventFilter = tc.filter
			if got := battleEquipmentHookEventAllowed(hook, &self, &subject, context); got != tc.want {
				t.Fatalf("event filter result=%v, want %v for %#v", got, tc.want, tc.filter)
			}
		})
	}
	basicContext := *context
	basicContext.IsBasicAttack = true
	basicContext.CastKind = "basic_attack"
	for _, tc := range []struct {
		value bool
		want  bool
	}{{false, false}, {true, true}} {
		hook := specializationTestHook("basic", "onBeforeDealDamage", "basic_node")
		hook.EventFilter = map[string]any{"includeBasicAttack": tc.value}
		if got := battleEquipmentHookEventAllowed(hook, &self, &subject, &basicContext); got != tc.want {
			t.Fatalf("includeBasicAttack=%v result=%v, want %v", tc.value, got, tc.want)
		}
	}
	if err := validateBattleSpecializationEventFilter(map[string]any{"futureFilter": true}); err == nil {
		t.Fatal("unknown eventFilter field was silently accepted")
	}
	for _, tc := range []struct {
		trigger string
		filter  map[string]any
		valid   bool
	}{
		{"onBeforeDealDamage", map[string]any{"skillIds": []any{"skill:fire"}, "damageTypes": []any{"magical"}, "unitTypes": []any{"enemy"}}, true},
		{"onTurnStart", map[string]any{"turnKinds": []any{"ordered"}, "includePreparation": true}, true},
		{"onBattleStart", map[string]any{"skillIds": []any{"skill:fire"}}, false},
		{"onAfterSkillCast", map[string]any{"damageTypes": []any{"magical"}}, false},
		{"onBeforeDealDamage", map[string]any{"buffIds": []any{"test_positive_buff"}}, false},
		{"onBeforeDealDamage", map[string]any{"probabilityKinds": []any{"status_apply"}}, false},
		{"onAfterSkillCast", map[string]any{"unitTypes": []any{"enemy"}}, false},
	} {
		err := validateBattleSpecializationEventFilter(tc.filter)
		if err == nil {
			err = validateBattleSpecializationEventFilterForTrigger(tc.trigger, tc.filter)
		}
		if (err == nil) != tc.valid {
			t.Fatalf("trigger=%s filter=%#v valid=%v err=%v", tc.trigger, tc.filter, tc.valid, err)
		}
	}
}

func TestBattleSpecializationTriggerBoundDSLAndPreDamageFilters(t *testing.T) {
	for _, tc := range []struct {
		trigger   string
		effect    map[string]any
		condition map[string]any
		valid     bool
	}{
		{trigger: "onAfterSkillCast", effect: map[string]any{"kind": "adjust_skill_cooldown", "amount": -1, "selectionMode": "context_skill"}, valid: true},
		{trigger: "onTurnStart", effect: map[string]any{"kind": "adjust_skill_cooldown", "amount": -1, "selectionMode": "context_skill"}, valid: false},
		{trigger: "onTurnStart", effect: map[string]any{"kind": "heal", "targetMode": "source", "maxHpRatio": .1}, valid: false},
		{trigger: "onAfterDealDamage", effect: map[string]any{"kind": "consume_mark", "markId": "gap"}, valid: true},
		{trigger: "onBattleStart", effect: map[string]any{"kind": "consume_mark", "markId": "gap"}, valid: false},
		{trigger: "onBeforeTakeDamage", condition: map[string]any{"kind": "incoming_damage_would_be_lethal"}, valid: true},
		{trigger: "onTurnStart", condition: map[string]any{"kind": "incoming_damage_would_be_lethal"}, valid: false},
	} {
		var err error
		if tc.effect != nil {
			err = validateBattleSpecializationHookEffect(tc.effect)
			if err == nil {
				err = validateBattleSpecializationEffectTrigger(tc.trigger, tc.effect)
			}
		} else {
			err = validateBattleSpecializationCondition(tc.condition)
			if err == nil {
				err = validateBattleSpecializationConditionTrigger(tc.trigger, tc.condition)
			}
		}
		if (err == nil) != tc.valid {
			t.Fatalf("trigger=%s effect=%#v condition=%#v valid=%v err=%v", tc.trigger, tc.effect, tc.condition, tc.valid, err)
		}
	}

	s := newSpecializationExecutionTestServer()
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 100)
	shield := specializationTestHook("shield", "onBeforeTakeDamage", "shield_node", map[string]any{
		"kind": "resource_shield", "resource": "mp", "absorbRatio": .5, "resourcePerDamage": 1,
	})
	shield.EventFilter = map[string]any{"skillIds": []any{"skill:allowed"}}
	target.SpecializationHooks = []battleEquipmentHookRule{shield}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	mismatch := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(1), &state.Enemies[0], &state.Players[0], 100, "physical", &battleEquipmentHookContext{SkillID: "skill:other", DamageType: "physical"})
	if mismatch.Amount != 100 || state.Enemies[0].MP != 100 {
		t.Fatalf("pre-damage specialization ignored eventFilter mismatch: result=%#v mp=%v", mismatch, state.Enemies[0].MP)
	}
	matched := s.battleApplySpecializationPreDamageAdjustments(state, time.UnixMilli(2), &state.Enemies[0], &state.Players[0], 100, "physical", &battleEquipmentHookContext{SkillID: "skill:allowed", DamageType: "physical"})
	if matched.Amount != 50 || state.Enemies[0].MP != 50 {
		t.Fatalf("pre-damage specialization did not settle matching eventFilter: result=%#v mp=%v", matched, state.Enemies[0].MP)
	}
}
