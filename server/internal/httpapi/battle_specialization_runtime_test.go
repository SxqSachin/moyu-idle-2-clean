package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func TestApplyBattleSpecializationStatModifiers(t *testing.T) {
	stats := map[string]float64{"physics_damage_amplification": 0.1}
	err := applyBattleSpecializationStatModifiers(stats, []store.SpecializationStatModifier{
		{AttributeKey: "physics_damage_amplification", Operation: "add", Value: 0.08},
		{AttributeKey: "physics_damage_amplification", Operation: "multiply", Value: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats["physics_damage_amplification"] != 0.36 {
		t.Fatalf("battle stat=%v, want 0.36", stats["physics_damage_amplification"])
	}
}

func TestBattleSpecializationRuntimeGrantIDAcceptsCurrentAndSnapshottedRulesets(t *testing.T) {
	grant := store.SpecializationRuntimeGrant{
		ID:           "tree:node:grant:payload",
		SourceNodeID: "node",
		Payload:      map[string]any{"id": "payload"},
	}
	for _, rulesetVersion := range []string{"specialization-v5", "specialization-v6"} {
		if _, err := battleSpecializationRuntimeGrantID("tree", rulesetVersion, grant); err != nil {
			t.Fatalf("ruleset %s rejected: %v", rulesetVersion, err)
		}
	}
	if _, err := battleSpecializationRuntimeGrantID("tree", "specialization-v4", grant); err == nil {
		t.Fatal("unsupported specialization ruleset was accepted")
	}
}

func TestResolveBattleSpecializationHooksAndMutations(t *testing.T) {
	s := &Server{}
	contribution := store.SpecializationBattleContribution{
		TreeID: "tree", RulesetVersion: "specialization-v5",
		Hooks: []store.SpecializationRuntimeGrant{
			{
				ID:            "tree:node:grant_hook:retaliate",
				SourceNodeID:  "node",
				SourceNameKey: "specialization.node.name",
				Payload: map[string]any{
					"id":            "retaliate",
					"trigger":       "onAfterTakeDamage",
					"dispatchHooks": false,
					"effects":       []any{map[string]any{"kind": "damage", "targetMode": "source", "damageType": "physical", "dispatchHooks": false, "amountFormula": map[string]any{"baseAdd": 5, "terms": []any{map[string]any{"attributeKey": "strength", "coefficient": .1}}}}},
				},
			},
		},
		Mutations: []store.SpecializationRuntimeGrant{
			{
				ID:            "tree:node:grant_mutation:mind_over_matter",
				SourceNodeID:  "node",
				SourceNameKey: "specialization.node.name",
				Payload: map[string]any{
					"id":             "mind_over_matter",
					"scope":          "both",
					"effectSelector": map[string]any{"mode": "all"},
					"ops": map[string]any{
						"amountTermsAppend": []any{map[string]any{"attributeKey": "strength", "coefficient": 0.12}},
					},
				},
			},
		},
	}
	hooks, err := s.resolveBattleSpecializationHooks(contribution)
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 1 || hooks[0].SourceKind != "specialization" || hooks[0].SourceID != "node" || hooks[0].Trigger != "onaftertakedamage" {
		t.Fatalf("unexpected hooks: %#v", hooks)
	}
	mutations, err := s.resolveBattleSpecializationEffectMutations(contribution)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 1 || mutations[0].SourceKind != "specialization" || len(mutations[0].AmountTermsAppend) != 1 || mutations[0].AmountTermsAppend[0].AttrKey != "strength" {
		t.Fatalf("unexpected mutations: %#v", mutations)
	}
	for _, mutate := range []func(map[string]any){
		func(payload map[string]any) { payload["trigger"] = "onFutureEvent" },
		func(payload map[string]any) { payload["effects"] = []any{map[string]any{"kind": "future_effect"}} },
	} {
		bad := contribution
		bad.Hooks = append([]store.SpecializationRuntimeGrant(nil), contribution.Hooks...)
		bad.Hooks[0].Payload = cloneAnyMap(contribution.Hooks[0].Payload)
		mutate(bad.Hooks[0].Payload)
		if _, err := s.resolveBattleSpecializationHooks(bad); err == nil {
			t.Fatalf("unsupported specialization hook definition was silently accepted: %#v", bad.Hooks[0].Payload)
		}
	}
}

func TestResolveBattleSpecializationSecondaryDamageBounds(t *testing.T) {
	parseOps := func(ops map[string]any) error {
		_, err := parseBattleSpecializationEffectMutation(map[string]any{
			"id":             "bounded_secondary",
			"scope":          "skill",
			"weaponClasses":  []any{"sword"},
			"damageTypes":    []any{"physical"},
			"effectSelector": map[string]any{"mode": "kind", "kinds": []any{"damage"}},
			"ops":            ops,
		})
		return err
	}
	parse := func(effect map[string]any) error {
		return parseOps(map[string]any{"appendEffects": []any{effect}})
	}
	for _, valid := range []map[string]any{
		{"id": "same", "kind": "secondary_damage", "targetPolicy": "same_row", "amountRatio": .5},
		{"id": "fixed", "kind": "secondary_damage", "targetPolicy": "fixed_row", "targetPosition": "rearguard", "amountRatio": .4},
		{"id": "all", "kind": "secondary_damage", "targetPolicy": "all_rows", "amountRatio": .3},
	} {
		if err := parse(valid); err != nil {
			t.Fatalf("valid secondary damage rejected: effect=%#v err=%v", valid, err)
		}
	}
	for _, invalid := range []map[string]any{
		{"id": "oversized", "kind": "secondary_damage", "targetPolicy": "same_row", "amountRatio": .5001},
		{"id": "unknown", "kind": "secondary_damage", "targetPolicy": "nearest", "amountRatio": .3},
		{"id": "missing_row", "kind": "secondary_damage", "targetPolicy": "fixed_row", "amountRatio": .3},
		{"id": "wrong_row_field", "kind": "secondary_damage", "targetPolicy": "same_row", "targetPosition": "rearguard", "amountRatio": .3},
	} {
		if err := parse(invalid); err == nil {
			t.Fatalf("invalid secondary damage accepted: %#v", invalid)
		}
	}
	secondary := map[string]any{"id": "secondary", "kind": "secondary_damage", "targetPolicy": "same_row", "amountRatio": .4}
	conflicts := []map[string]any{
		{"appendEffects": []any{secondary, map[string]any{"id": "secondary_2", "kind": "secondary_damage", "targetPolicy": "all_rows", "amountRatio": .3}}},
		{"appendEffects": []any{secondary}, "targetCountDelta": 1},
		{"appendEffects": []any{secondary}, "hitCountDelta": 1},
		{"appendEffects": []any{secondary, map[string]any{"id": "splash", "kind": "splash_damage", "targetCount": 1, "amountRatio": .2}}},
	}
	for _, ops := range conflicts {
		if err := parseOps(ops); err == nil {
			t.Fatalf("secondary damage expansion conflict accepted: %#v", ops)
		}
	}
}

func TestResolveBattleSpecializationV5RequiresOuterCapForEveryDirectExtraAction(t *testing.T) {
	s := &Server{}
	for _, trigger := range []string{
		"onAfterDealCrit",
		"onAfterDealDamage",
		"onAfterSkillCast",
		"onAfterTakeDamage",
		"onAllyBeforeTakeDamage",
		"onBattleStart",
		"onBeforeDealDamage",
		"onBeforeSkillCast",
		"onBeforeTakeDamage",
		"onKill",
		"onTurnEnd",
		"onTurnStart",
		"onWaveStart",
	} {
		t.Run(trigger, func(t *testing.T) {
			contribution := store.SpecializationBattleContribution{
				TreeID: "tree", RulesetVersion: "specialization-v5",
				Hooks: []store.SpecializationRuntimeGrant{{
					ID: "tree:node:grant_extra:unsafe_extra", SourceNodeID: "node", SourceNameKey: "specialization.node.name",
					Payload: map[string]any{
						"id": "unsafe_extra", "trigger": trigger, "dispatchHooks": false,
						"limits":     map[string]any{"perTurn": 1},
						"conditions": []any{map[string]any{"kind": "self_weapon_class", "weaponClasses": []any{"dagger"}}},
						"effects": []any{map[string]any{
							"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false,
							"procChanceBase": 0.2, "procChanceMax": 0.4, "abortHookOnFail": true,
						}},
					},
				}},
			}
			if _, err := s.resolveBattleSpecializationHooks(contribution); err == nil {
				t.Fatal("v5 per-turn-only direct extra action was accepted")
			}
			contribution.Hooks[0].Payload["limits"] = map[string]any{"perWave": 1}
			if hooks, err := s.resolveBattleSpecializationHooks(contribution); err != nil || len(hooks) != 1 || hooks[0].MaxTriggersPerWave != 1 {
				t.Fatalf("v5 per-wave extra action was rejected: hooks=%#v err=%v", hooks, err)
			}
			unsafeChance := cloneAnyMap(contribution.Hooks[0].Payload)
			unsafeChance["effects"] = []any{map[string]any{
				"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false,
				"procChanceBase": 0.41, "procChanceMax": 0.41, "abortHookOnFail": true,
			}}
			contribution.Hooks[0].Payload = unsafeChance
			if _, err := s.resolveBattleSpecializationHooks(contribution); err == nil {
				t.Fatal("v5 extra action above the 40% cap was accepted")
			}
		})
	}
}

func TestValidateBattleSpecializationProbabilisticExtraTurn(t *testing.T) {
	valid := map[string]any{
		"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false,
		"procChanceBase": 0.2, "procChanceMax": 0.4, "abortHookOnFail": true,
	}
	if err := validateBattleSpecializationHookEffect(valid); err != nil {
		t.Fatalf("valid capped extra-action chance was rejected: %v", err)
	}
	invalidCap := cloneAnyMap(valid)
	invalidCap["procChanceMax"] = 0.19
	if err := validateBattleSpecializationHookEffect(invalidCap); err == nil {
		t.Fatal("extra-action chance cap below its base chance was accepted")
	}
	missingChance := cloneAnyMap(valid)
	delete(missingChance, "procChanceBase")
	delete(missingChance, "procChanceMax")
	if err := validateBattleSpecializationHookEffect(missingChance); err == nil {
		t.Fatal("abortHookOnFail without an extra-action probability was accepted")
	}
}

func TestBattleSpecializationStablePayloadIdentitySurvivesReordering(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	contribution := specializationTestSourceContribution(t, "strength_root_break_stance")
	if len(contribution.Hooks) != 2 {
		t.Fatalf("strength root source hooks=%d, want 2", len(contribution.Hooks))
	}
	original, err := s.resolveBattleSpecializationHooks(contribution)
	if err != nil {
		t.Fatal(err)
	}
	reorderedContribution := contribution
	reorderedContribution.Hooks = append([]store.SpecializationRuntimeGrant(nil), contribution.Hooks...)
	reorderedContribution.Hooks[0], reorderedContribution.Hooks[1] = reorderedContribution.Hooks[1], reorderedContribution.Hooks[0]
	reordered, err := s.resolveBattleSpecializationHooks(reorderedContribution)
	if err != nil {
		t.Fatal(err)
	}

	type hookIdentity struct {
		ID           string
		CounterGroup string
		SourceID     string
		SourceName   string
		MetaNodeID   string
		MetaHookID   string
		PerTurn      int
		PerWave      int
		PerBattle    int
	}
	snapshot := func(hooks []battleEquipmentHookRule) map[string]hookIdentity {
		out := make(map[string]hookIdentity, len(hooks))
		for _, hook := range hooks {
			meta := battleSpecializationSourceMeta(hook, nil)
			out[hook.ID] = hookIdentity{
				ID: hook.ID, CounterGroup: battleEquipmentHookCounterGroup(hook),
				SourceID: hook.SourceID, SourceName: hook.SourceNameKey,
				MetaNodeID: strings.TrimSpace(meta["nodeId"].(string)), MetaHookID: strings.TrimSpace(meta["hookId"].(string)),
				PerTurn: hook.MaxTriggersPerTurn, PerWave: hook.MaxTriggersPerWave, PerBattle: hook.MaxTriggersPerBattle,
			}
		}
		return out
	}
	if got, want := snapshot(reordered), snapshot(original); !reflect.DeepEqual(got, want) {
		t.Fatalf("hook identity/limits/provenance changed after payload reorder: got=%#v want=%#v", got, want)
	}
	reorderedByID := make(map[string]battleEquipmentHookRule, len(reordered))
	for _, hook := range reordered {
		reorderedByID[hook.ID] = hook
	}
	ancestryState := &battleRuntimeState{}
	leave, blockedBy := battleEnterSpecializationContribution(ancestryState, original[0])
	if leave == nil || blockedBy != "" {
		t.Fatalf("first stable contribution was blocked: %q", blockedBy)
	}
	if nestedLeave, nestedBlockedBy := battleEnterSpecializationContribution(ancestryState, reorderedByID[original[0].ID]); nestedLeave != nil || nestedBlockedBy != "ancestor_contribution" {
		t.Fatalf("reordered copy did not share recursion identity: leave=%v blockedBy=%q", nestedLeave != nil, nestedBlockedBy)
	}
	leave()
	for _, payloadID := range []string{"strength_break_momentum_cycle", "strength_break_momentum_release"} {
		wantID := strings.Join([]string{contribution.TreeID, "strength_root_break_stance", "grant_strength_root_break_cycle", payloadID}, ":")
		if _, exists := snapshot(original)[wantID]; !exists {
			t.Fatalf("stable source hook id %q is missing: %#v", wantID, snapshot(original))
		}
	}

	payload := cloneAnyMap(contribution.Hooks[0].Payload)
	payloadID := strings.TrimSpace(payload["id"].(string))
	crossNode := store.SpecializationBattleContribution{
		TreeID: contribution.TreeID, RulesetVersion: contribution.RulesetVersion,
		Hooks: []store.SpecializationRuntimeGrant{
			{ID: strings.Join([]string{contribution.TreeID, "node_a", "shared_grant", payloadID}, ":"), SourceNodeID: "node_a", SourceNameKey: "node_a.name", Payload: payload},
			{ID: strings.Join([]string{contribution.TreeID, "node_b", "shared_grant", payloadID}, ":"), SourceNodeID: "node_b", SourceNameKey: "node_b.name", Payload: cloneAnyMap(payload)},
		},
	}
	crossNodeHooks, err := s.resolveBattleSpecializationHooks(crossNode)
	if err != nil {
		t.Fatal(err)
	}
	if len(crossNodeHooks) != 2 || crossNodeHooks[0].ID == crossNodeHooks[1].ID ||
		battleEquipmentHookCounterGroup(crossNodeHooks[0]) == battleEquipmentHookCounterGroup(crossNodeHooks[1]) {
		t.Fatalf("same grant/payload collided across nodes: %#v", crossNodeHooks)
	}
	crossNodeAncestry := &battleRuntimeState{}
	leaveA, blockedA := battleEnterSpecializationContribution(crossNodeAncestry, crossNodeHooks[0])
	leaveB, blockedB := battleEnterSpecializationContribution(crossNodeAncestry, crossNodeHooks[1])
	if leaveA == nil || leaveB == nil || blockedA != "" || blockedB != "" {
		t.Fatalf("distinct-node contributions collided in recursion ancestry: blockedA=%q blockedB=%q", blockedA, blockedB)
	}
	leaveB()
	leaveA()
	missingGrantID := crossNode
	missingGrantID.Hooks = append([]store.SpecializationRuntimeGrant(nil), crossNode.Hooks[:1]...)
	missingGrantID.Hooks[0].ID = strings.Join([]string{contribution.TreeID, "node_a", payloadID}, ":")
	if _, err := s.resolveBattleSpecializationHooks(missingGrantID); err == nil {
		t.Fatal("specialization-v5 contribution without an author grant id was accepted")
	}
	delimitedGrantID := crossNode
	delimitedGrantID.Hooks = append([]store.SpecializationRuntimeGrant(nil), crossNode.Hooks[:1]...)
	delimitedGrantID.Hooks[0].ID = strings.Join([]string{contribution.TreeID, "node_a", "shared:grant", payloadID}, ":")
	if _, err := s.resolveBattleSpecializationHooks(delimitedGrantID); err == nil {
		t.Fatal("specialization-v5 contribution with a delimiter-bearing author grant id was accepted")
	}

	mutationContribution := specializationTestSourceContribution(t, "strength_tech_rupture", "dexterity_tech_chain_steps")
	mutations, err := s.resolveBattleSpecializationEffectMutations(mutationContribution)
	if err != nil {
		t.Fatal(err)
	}
	reorderedMutationContribution := mutationContribution
	reorderedMutationContribution.Mutations = append([]store.SpecializationRuntimeGrant(nil), mutationContribution.Mutations...)
	for left, right := 0, len(reorderedMutationContribution.Mutations)-1; left < right; left, right = left+1, right-1 {
		reorderedMutationContribution.Mutations[left], reorderedMutationContribution.Mutations[right] = reorderedMutationContribution.Mutations[right], reorderedMutationContribution.Mutations[left]
	}
	reorderedMutations, err := s.resolveBattleSpecializationEffectMutations(reorderedMutationContribution)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mutations, reorderedMutations) {
		t.Fatalf("mutation identity or ordering changed after contribution reorder: got=%#v want=%#v", reorderedMutations, mutations)
	}
}

func TestResolveBattleSpecializationMutationsRejectsCrossGrantAppendIDCollision(t *testing.T) {
	s := &Server{}
	appendMutation := func(nodeID, grantID, payloadID, effectID string) store.SpecializationRuntimeGrant {
		return store.SpecializationRuntimeGrant{
			ID: "tree:" + nodeID + ":" + grantID + ":" + payloadID, SourceNodeID: nodeID, SourceNameKey: "specialization.node.name",
			Payload: map[string]any{
				"id": payloadID, "scope": "skill",
				"effectSelector": map[string]any{"mode": "all"},
				"ops": map[string]any{"appendEffects": []any{map[string]any{
					"id": effectID, "kind": "consume_hp", "maxHpRatio": .03, "minimumHp": 1,
				}}},
			},
		}
	}
	contribution := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Mutations: []store.SpecializationRuntimeGrant{
		appendMutation("node_first", "grant_first", "mutation_first", "specialization:shared-effect"),
		appendMutation("node_second", "grant_second", "mutation_second", "SPECIALIZATION:SHARED-EFFECT"),
	}}
	if _, err := s.resolveBattleSpecializationEffectMutations(contribution); err == nil {
		t.Fatal("cross-grant appended effect ID collision was accepted")
	}
}

func TestResolveBattleSpecializationGeneratedEffectDependencies(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.Skills = []map[string]any{{
		"id": "skill:test", "effects": []any{
			map[string]any{"id": "base:damage", "kind": "damage"},
			map[string]any{"id": "base:status", "kind": "status"},
		},
	}}
	producer := func(priority int, appended map[string]any) store.SpecializationRuntimeGrant {
		return store.SpecializationRuntimeGrant{
			ID: "tree:producer:grant_producer:producer", SourceNodeID: "producer", SourceNameKey: "specialization.node.producer.name",
			Payload: map[string]any{
				"id": "producer", "priority": priority, "scope": "skill",
				"effectSelector": map[string]any{"mode": "effect_id", "effectIds": []any{"base:damage"}},
				"ops":            map[string]any{"appendEffects": []any{appended}},
			},
		}
	}
	consumer := func(priority int, includeGenerated bool, effectID string, ops map[string]any) store.SpecializationRuntimeGrant {
		return store.SpecializationRuntimeGrant{
			ID: "tree:consumer:grant_consumer:consumer", SourceNodeID: "consumer", SourceNameKey: "specialization.node.consumer.name",
			Payload: map[string]any{
				"id": "consumer", "priority": priority, "scope": "skill",
				"effectSelector": map[string]any{"mode": "effect_id", "effectIds": []any{effectID}, "includeGenerated": includeGenerated},
				"ops":            ops,
			},
		}
	}
	splash := map[string]any{"id": "generated:splash", "kind": "splash_damage", "targetCount": 1, "amountRatio": .5}
	valid := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Mutations: []store.SpecializationRuntimeGrant{
		consumer(0, true, "generated:splash", map[string]any{"amountMultiplier": 2.0}),
		producer(10, splash),
	}}
	mutations, err := s.resolveBattleSpecializationEffectMutations(valid)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 2 || mutations[0].ID != "tree:producer:grant_producer:producer" || mutations[1].ID != "tree:consumer:grant_consumer:consumer" {
		t.Fatalf("generated dependency did not preserve priority order: %#v", mutations)
	}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	actor.Skills = []battleSkillState{{ID: "skill:test", Effects: []battleSkillEffect{{ID: "base:damage", Kind: "damage"}}}}
	actor.SpecializationEffectMutations = mutations
	targeting := battleSkillTargeting{TargetScope: "enemy", TargetCount: battleSkillTargetCount{Base: 1}}
	effects := []battleSkillEffect{{
		ID: "base:damage", Kind: "damage", DamageType: "physical", Targeting: targeting,
		AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, DamageSpec: battleDamageSpec{DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: 20}, HitCount: battleSkillTargetCount{Base: 1}},
	}}
	mutated := s.applyBattleAffixEffectMutations(&actor, "skill", "skill:test", 1, targeting, effects)
	if len(mutated) != 2 || mutated[1].ID != "generated:splash" || mutated[1].AmountFormula.BaseAdd != 20 {
		t.Fatalf("generated effect consumer did not settle after producer: %#v", mutated)
	}

	t.Run("includeGenerated is required", func(t *testing.T) {
		bad := valid
		bad.Mutations = []store.SpecializationRuntimeGrant{producer(10, splash), consumer(0, false, "generated:splash", map[string]any{"amountMultiplier": 2.0})}
		if _, err := s.resolveBattleSpecializationEffectMutations(bad); err == nil {
			t.Fatal("generated effect selector without includeGenerated was accepted")
		}
	})
	t.Run("producer must execute first", func(t *testing.T) {
		bad := valid
		bad.Mutations = []store.SpecializationRuntimeGrant{producer(0, splash), consumer(10, true, "generated:splash", map[string]any{"amountMultiplier": 2.0})}
		if _, err := s.resolveBattleSpecializationEffectMutations(bad); err == nil {
			t.Fatal("generated effect consumer before producer was accepted")
		}
	})
	t.Run("self-produced selector is rejected", func(t *testing.T) {
		self := producer(0, splash)
		self.Payload["effectSelector"] = map[string]any{"mode": "effect_id", "effectIds": []any{"generated:splash"}, "includeGenerated": true}
		if _, err := s.resolveBattleSpecializationEffectMutations(store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Mutations: []store.SpecializationRuntimeGrant{self}}); err == nil {
			t.Fatal("mutation selecting its own generated effect was accepted")
		}
	})
	t.Run("non-materialized generated effect is rejected", func(t *testing.T) {
		amp := map[string]any{"id": "generated:amp", "kind": "runtime_damage_amp", "value": .2}
		bad := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Mutations: []store.SpecializationRuntimeGrant{
			producer(10, amp), consumer(0, true, "generated:amp", map[string]any{"amountMultiplier": 2.0}),
		}}
		if _, err := s.resolveBattleSpecializationEffectMutations(bad); err == nil {
			t.Fatal("selector for non-materialized runtime damage amp was accepted")
		}
	})
	t.Run("exact effect operation compatibility is enforced", func(t *testing.T) {
		bad := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Mutations: []store.SpecializationRuntimeGrant{
			consumer(0, false, "base:status", map[string]any{"hitCountDelta": 1}),
		}}
		if _, err := s.resolveBattleSpecializationEffectMutations(bad); err == nil {
			t.Fatal("damage-only operation targeting a status effect was accepted")
		}
	})
	t.Run("generated effect cannot select skill scalar operations", func(t *testing.T) {
		bad := store.SpecializationBattleContribution{TreeID: "tree", RulesetVersion: "specialization-v5", Mutations: []store.SpecializationRuntimeGrant{
			producer(10, splash), consumer(0, true, "generated:splash", map[string]any{"cooldownDelta": -1, "cooldownFloor": 1, "mpCostMultiplier": 3}),
		}}
		if _, err := s.resolveBattleSpecializationEffectMutations(bad); err == nil {
			t.Fatal("generated effect selector silently accepted a skill-level cooldown operation")
		}
	})
}

func TestBattleCarryStatePreservesSpecializationSnapshot(t *testing.T) {
	unit := battleUnitState{
		UnitID:                        "player:1",
		SpecializationTreeID:          "player_combat_specialization",
		SpecializationRulesetVersion:  "specialization-v5",
		SpecializationBalanceRevision: 5,
		SpecializationRevision:        7,
		SpecializationDigest:          "abc123",
		SpecializationHooks: []battleEquipmentHookRule{
			{ID: "specialization-hook", SourceKind: "specialization", Trigger: "on_battle_start", Effects: []map[string]any{{"kind": "heal"}}},
		},
	}
	carried := encodeBattleCarryUnitState(unit)
	if carried.SpecializationTreeID != unit.SpecializationTreeID || carried.SpecializationRulesetVersion != unit.SpecializationRulesetVersion || carried.SpecializationBalanceRevision != 5 || carried.SpecializationRevision != 7 || carried.SpecializationDigest != "abc123" || len(carried.SpecializationHooks) != 1 {
		t.Fatalf("specialization snapshot not preserved: %#v", carried)
	}
}

func TestSpecializationImpactConfirmationUses409ErrorEnvelope(t *testing.T) {
	s := &Server{}
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		key, err := specializationIdempotencyKey(c)
		if err != nil || key != "idem-1" {
			t.Fatalf("idempotency key=%q err=%v", key, err)
		}
		return s.writeSpecializationImpactConfirmation(c, store.SpecializationMutationResult{
			Committed:       false,
			RequiresConfirm: true,
			Impact: store.SpecializationEquipmentImpact{
				Digest: "impact-digest",
			},
		})
	})
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Idempotency-Key", "idem-1")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("status=%d, want 409", resp.StatusCode)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		Data store.SpecializationMutationResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED" || !envelope.Data.RequiresConfirm || envelope.Data.Impact.Digest != "impact-digest" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestSpecializationDurableIdempotencyRoutesBypassReplayGuard(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/player/specializations/player_five_spokes/allocate", want: true},
		{path: "/api/player/specializations/player_five_spokes/reset", want: true},
		{path: "/api/player/specializations/player_five_spokes", want: false},
		{path: "/api/player/specializations//allocate", want: false},
		{path: "/api/player/specializations/player_five_spokes/preview", want: false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := hasDurableHTTPIdempotency(test.path); got != test.want {
				t.Fatalf("hasDurableHTTPIdempotency(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestSpecializationMutationInvalidatesProfileCachesBeforeSync(t *testing.T) {
	const userID int64 = 42
	queue := make(chan mutationSyncScope, 1)
	s := &Server{
		mutationSyncQueue: queue,
		playerProfileMem:  map[int64]playerProfileCacheEntry{userID: {}},
		playerPublicMem:   map[int64]playerProfileCacheEntry{userID: {}},
	}
	s.syncAfterSpecializationMutation(userID)
	if _, exists := s.playerProfileMem[userID]; exists {
		t.Fatal("specialization mutation left the private player profile cache stale")
	}
	if _, exists := s.playerPublicMem[userID]; exists {
		t.Fatal("specialization mutation left the public player profile cache stale")
	}
	select {
	case scope := <-queue:
		if len(scope.UserIDs) != 1 || scope.UserIDs[0] != userID || !scope.PlayerState || !scope.Inventory || !scope.Equipment || !scope.Attributes || !scope.Queue || !scope.Achievements {
			t.Fatalf("specialization mutation sync scope=%#v", scope)
		}
	default:
		t.Fatal("specialization mutation did not enqueue its domain sync")
	}
}
