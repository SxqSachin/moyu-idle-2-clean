package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/store"
)

func TestBattleRuntimeCounterKeyIsJSONBSafeAndCollisionFree(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{"player:1", "shared:hook"},
		{"player", "1:shared:hook"},
		{"player\x00one", "hook\x00group"},
		{"", ""},
	}
	seen := make(map[string][2]string, len(pairs))
	for _, pair := range pairs {
		key := battleRuntimeCounterKey(pair[0], pair[1])
		if strings.ContainsRune(key, '\x00') {
			t.Fatalf("counter key contains NUL for %#v: %q", pair, key)
		}
		if previous, exists := seen[key]; exists && previous != pair {
			t.Fatalf("counter key collision: %#v and %#v both encoded as %q", previous, pair, key)
		}
		seen[key] = pair
	}
}

func TestBattleCarryCounterKeysRoundTripWithoutPostgresNUL(t *testing.T) {
	t.Parallel()

	battleKey := battleRuntimeCounterKey("player:carry", "battle:limit")
	cooldownKey := battleRuntimeCounterKey("player:carry", "cooldown:group")
	state := battleRuntimeState{
		Wave:                         1,
		Players:                      []battleUnitState{{UnitID: "player:carry", Alive: true, HP: 10, MaxHP: 10}},
		BattleHookBattleCounts:       map[string]int{battleKey: 2},
		BattleHookCooldownUntilTurns: map[string]int{cooldownKey: 7},
	}
	carry := battleCarryStatePayload(state, store.BattleRunStatusEnded)
	raw, err := json.Marshal(carry)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`\u0000`)) {
		t.Fatalf("carry payload contains PostgreSQL-incompatible NUL escape: %s", raw)
	}

	previous := store.BattlePreviousWaveCarry{Wave: 1, Status: store.BattleRunStatusEnded, CarryStateRaw: raw}
	decoded, err := decodeRequiredBattlePreviousWaveCarry(previous, 1)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.BattleHookBattleCounts[battleKey] != 2 {
		t.Fatalf("battle counter did not round trip: %#v", decoded.BattleHookBattleCounts)
	}
	if decoded.BattleHookCooldownUntilTurns[cooldownKey] != 7 {
		t.Fatalf("cooldown counter did not round trip: %#v", decoded.BattleHookCooldownUntilTurns)
	}
}

func probabilityTestUnit(id string) battleUnitState {
	return battleUnitState{
		UnitID:          id,
		Side:            "player",
		OriginSide:      "player",
		BaseSide:        "player",
		PerspectiveSide: "player",
		Position:        "midguard",
		BasePosition:    "midguard",
		Alive:           true,
		HP:              50,
		MaxHP:           100,
		MP:              10,
		MaxMP:           100,
		Stats:           map[string]float64{"hp": 100, "mp": 100},
		BaseStats:       map[string]float64{"hp": 100, "mp": 100},
	}
}

func TestBattleProbabilityComparatorsPreserveLegacyEqualityBoundaries(t *testing.T) {
	if !battleProbabilityRollSucceeds(battleProbabilityEquipmentProc, 0.5, 0.5) {
		t.Fatalf("equipment proc must succeed when roll equals chance")
	}
	if battleProbabilityRollSucceeds(battleProbabilityStatusApply, 0.5, 0.5) {
		t.Fatalf("status apply must fail when roll equals chance")
	}
}

func TestResolveBattleProbabilityPreservesShortCircuitsAndRNGConsumption(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.UserID = 1
	state := &battleRuntimeState{Seed: 41}

	zero := s.resolveBattleProbability(state, battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 0})
	if zero.Succeeded || state.RNGIndex != 0 {
		t.Fatalf("0%% result=%#v rngIndex=%d, want failed without RNG", zero, state.RNGIndex)
	}
	one := s.resolveBattleProbability(state, battleProbabilityCheck{Kind: battleProbabilityStatusApply, Actor: &actor, BaseChance: 1})
	if !one.Succeeded || state.RNGIndex != 0 {
		t.Fatalf("100%% result=%#v rngIndex=%d, want success without RNG", one, state.RNGIndex)
	}
	mid := s.resolveBattleProbability(state, battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 0.5})
	if !mid.Rolled || state.RNGIndex != 1 {
		t.Fatalf("mid chance result=%#v rngIndex=%d, want one RNG draw", mid, state.RNGIndex)
	}
}

func TestResolveBattleProbabilityMatchesLegacyUnmodifiedRolls(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	for _, tc := range []struct {
		name string
		kind string
		seed int64
	}{
		{name: "equipment", kind: battleProbabilityEquipmentProc, seed: 123},
		{name: "status", kind: battleProbabilityStatusApply, seed: 456},
	} {
		expectedState := &battleRuntimeState{Seed: tc.seed}
		expectedRoll := expectedState.randFloat64()
		actualState := &battleRuntimeState{Seed: tc.seed}
		result := s.resolveBattleProbability(actualState, battleProbabilityCheck{Kind: tc.kind, Actor: &actor, BaseChance: 0.37})
		if result.Roll != expectedRoll || result.Succeeded != battleProbabilityRollSucceeds(tc.kind, 0.37, expectedRoll) {
			t.Fatalf("%s result=%#v expectedRoll=%v", tc.name, result, expectedRoll)
		}
		if actualState.RNGIndex != expectedState.RNGIndex {
			t.Fatalf("%s rngIndex=%d, want legacy %d", tc.name, actualState.RNGIndex, expectedState.RNGIndex)
		}
	}
}

func TestResolveBattleProbabilityOrdersModifiersAndFiltersKinds(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:          "b_set_first",
			Trigger:     "on_before_probability_check",
			Priority:    20,
			EventFilter: map[string]any{"probabilityKinds": []string{"equipment_proc"}},
			Effects:     []map[string]any{{"kind": "set_probability", "value": 0.4}},
		},
		{
			ID:       "a_add_second",
			Trigger:  "on_before_probability_check",
			Priority: 10,
			Effects:  []map[string]any{{"kind": "add_probability", "value": 0.1}},
		},
	}
	state := &battleRuntimeState{Seed: 9}
	proc := s.resolveBattleProbability(state, battleProbabilityCheck{
		Kind:       battleProbabilityEquipmentProc,
		Actor:      &actor,
		BaseChance: 0.2,
		SourceIDs:  []string{"cat_sword"},
	})
	if proc.FinalChance != 0.5 {
		t.Fatalf("ordered final chance=%v, want 0.5", proc.FinalChance)
	}

	statusState := &battleRuntimeState{Seed: 9}
	status := s.resolveBattleProbability(statusState, battleProbabilityCheck{
		Kind:       battleProbabilityStatusApply,
		Actor:      &actor,
		BaseChance: 0.2,
		SourceIDs:  []string{"skill:test"},
	})
	if roundTo4(status.FinalChance) != 0.3 {
		t.Fatalf("kind-filtered final chance=%v, want 0.3", status.FinalChance)
	}
}

func TestResolveBattleProbabilityUsesHookIDAsPriorityTieBreaker(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{ID: "z_set", Trigger: "on_before_probability_check", Priority: 5, Effects: []map[string]any{{"kind": "set_probability", "value": 0.4}}},
		{ID: "a_add", Trigger: "on_before_probability_check", Priority: 5, Effects: []map[string]any{{"kind": "add_probability", "value": 0.2}}},
	}
	result := s.resolveBattleProbability(&battleRuntimeState{Seed: 5}, battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 0.1})
	// a_add executes first, then z_set wins as the last modifier at equal priority.
	if result.FinalChance != 0.4 {
		t.Fatalf("tie-broken chance=%v, want 0.4", result.FinalChance)
	}
}

func TestResolveBattleProbabilityForceFailureIsTerminalAndDoesNotRoll(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:       "force_failure",
			Trigger:  "on_before_probability_check",
			Priority: 100,
			Effects:  []map[string]any{{"kind": "force_probability_failure"}},
		},
		{
			ID:       "late_set",
			Trigger:  "on_before_probability_check",
			Priority: 0,
			Effects:  []map[string]any{{"kind": "set_probability", "value": 1}},
		},
	}
	state := &battleRuntimeState{Seed: 77}
	result := s.resolveBattleProbability(state, battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 1})
	if result.Succeeded || !result.ForcedFailure || state.RNGIndex != 0 {
		t.Fatalf("forced result=%#v rngIndex=%d", result, state.RNGIndex)
	}
}

func TestResolveBattleProbabilityIgnoreHooksPreventsModifierAndOutcomeDispatch(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{ID: "forced", Trigger: "on_before_probability_check", Effects: []map[string]any{{"kind": "force_probability_failure"}}},
		{ID: "reward", Trigger: "on_probability_success", Effects: []map[string]any{{"kind": "heal", "value": 25}}},
	}
	state := &battleRuntimeState{}
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 1, IgnoreHooks: true}
	result := s.resolveBattleProbability(state, check)
	if !result.Succeeded || result.ForcedFailure {
		t.Fatalf("ignored hooks changed result: %#v", result)
	}
	if actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result); len(actions) != 0 || actor.HP != 50 {
		t.Fatalf("ignored hooks dispatched outcome actions=%#v hp=%v", actions, actor.HP)
	}
}

func TestResolveBattleProbabilitySupportsActiveBuffModifiers(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":          "lucky_probability_bonus",
			"buffId":      "lucky_blessing",
			"trigger":     "on_before_probability_check",
			"eventFilter": map[string]any{"sourceIds": []string{"cat_sword"}},
			"effects":     []map[string]any{{"kind": "add_probability", "value": 0.03}},
		},
	}
	actor := probabilityTestUnit("player:1")
	actor.Buffs = []battleBuffState{{BuffID: "lucky_blessing", RemainTurns: 2}}
	result := s.resolveBattleProbability(&battleRuntimeState{Seed: 3}, battleProbabilityCheck{
		Kind:       battleProbabilityEquipmentProc,
		Actor:      &actor,
		BaseChance: 0.2,
		SourceIDs:  []string{"cat_sword"},
	})
	if result.FinalChance != 0.23 {
		t.Fatalf("buff-modified chance=%v, want 0.23", result.FinalChance)
	}

	unmatched := s.resolveBattleProbability(&battleRuntimeState{Seed: 3}, battleProbabilityCheck{
		Kind:       battleProbabilityEquipmentProc,
		Actor:      &actor,
		BaseChance: 0.2,
		SourceIDs:  []string{"another_item"},
	})
	if unmatched.FinalChance != 0.2 {
		t.Fatalf("source-filtered chance=%v, want 0.2", unmatched.FinalChance)
	}
}

func TestResolveBattleProbabilityGloballyOrdersEquipmentAndBuffModifiers(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "buff_add_first",
			"buffId":   "lucky_blessing",
			"trigger":  "on_before_probability_check",
			"priority": 20,
			"effects":  []map[string]any{{"kind": "add_probability", "value": 0.25}},
		},
	}
	actor := probabilityTestUnit("player:1")
	actor.Buffs = []battleBuffState{{BuffID: "lucky_blessing", RemainTurns: 2}}
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:       "equipment_set_second",
			Trigger:  "on_before_probability_check",
			Priority: 10,
			Effects:  []map[string]any{{"kind": "set_probability", "value": 0.4}},
		},
	}
	result := s.resolveBattleProbability(&battleRuntimeState{Seed: 12}, battleProbabilityCheck{
		Kind:       battleProbabilityEquipmentProc,
		Actor:      &actor,
		BaseChance: 0.1,
	})
	if result.FinalChance != 0.4 {
		t.Fatalf("cross-source modifier order produced chance=%v, want 0.4", result.FinalChance)
	}
}

func TestProbabilityLifecycleProcCannotRecursivelyDispatchLifecycleHooks(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:      "success_heal_with_forbidden_nested_proc",
			Trigger: "on_probability_success",
			Effects: []map[string]any{{
				"kind":           "heal",
				"value":          1,
				"targetMode":     "self",
				"procChanceBase": 1,
			}},
		},
	}
	state := &battleRuntimeState{Wave: 1}
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 1}
	result := s.resolveBattleProbability(state, check)
	actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if actor.HP != 51 || len(actions) != 1 {
		t.Fatalf("nested lifecycle proc recursively dispatched: hp=%v actions=%#v", actor.HP, actions)
	}
	if state.RNGIndex != 0 || state.probabilityHookDepth != 0 {
		t.Fatalf("nested 100%% proc changed rng/depth: rng=%d depth=%d", state.RNGIndex, state.probabilityHookDepth)
	}
}

func TestProbabilityFailureExtraTurnRestoresLifecycleAndIsolatesTurnLimits(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "extra_turn_probability_reward", "durationTurns": 2, "stackPolicy": "refresh_duration"},
	}
	exitHook := battleEquipmentHookRule{
		ID:                   "first_failure_extra_turn",
		Trigger:              "on_probability_failure",
		MaxTriggersPerBattle: 1,
		Effects:              []map[string]any{{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"}},
	}
	probeHook := battleEquipmentHookRule{
		ID:                   "extra_turn_probability_probe",
		SourceItemID:         "probability_probe",
		Trigger:              "on_turn_start",
		MaxTriggersPerBattle: 2,
		Effects: []map[string]any{{
			"kind":                "heal",
			"value":               1,
			"targetMode":          "self",
			"procChanceBase":      1,
			"probabilityGroup":    "extra_turn_probability_probe",
			"maxSuccessesPerTurn": 1,
		}},
	}
	rewardHook := battleEquipmentHookRule{
		ID:                   "extra_turn_probability_reward",
		Trigger:              "on_probability_success",
		MaxTriggersPerTurn:   1,
		MaxTriggersPerBattle: 2,
		Effects: []map[string]any{{
			"kind":          "add_buff",
			"buffId":        "extra_turn_probability_reward",
			"durationTurns": 2,
			"targetMode":    "self",
		}},
	}
	actor := probabilityTestUnit("player:1")
	actor.UserID = 1
	actor.EquipmentHooks = []battleEquipmentHookRule{exitHook, probeHook, rewardHook}
	actor.Skills = []battleSkillState{testDamageSkill(
		"extra_turn_probe_attack",
		1,
		testSingleEnemyTargeting("midguard"),
		battleDamageSpec{
			DamageType:    "physical",
			AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
			HitCount:      battleSkillTargetCount{Base: 1},
		},
	)}
	actor.SkillIDs = []string{"extra_turn_probe_attack"}
	enemy := probabilityTestUnit("enemy:1")
	enemy.Side = "enemy"
	enemy.OriginSide = "enemy"
	enemy.BaseSide = "enemy"
	enemy.PerspectiveSide = "enemy"
	state := &battleRuntimeState{
		Wave:    1,
		Seed:    17,
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{enemy},
		Order:   []string{actor.UnitID, enemy.UnitID},
	}
	actorPtr := &state.Players[0]
	rewardTurnKey, _ := battleHookCounterKeys(state, actorPtr, battleEquipmentHookCounterGroup(rewardHook))
	state.BattleHookTurnCounts = map[string]int{rewardTurnKey: 1}
	probeSuccessKey := battleProbabilitySuccessCounterKey(actorPtr, "extra_turn_probability_probe")
	state.ProbabilitySuccessCounts = map[string]int{probeSuccessKey: 1}

	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: actorPtr, BaseChance: 0, SourceIDs: []string{"outer_failure"}}
	result := s.resolveBattleProbability(state, check)
	actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if got := countActionKindRecursive(actions, "extra_turn"); got != 1 {
		t.Fatalf("failure hook must queue exactly one formal extra turn, got=%d actions=%#v", got, actions)
	}
	if len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("failure hook did not persist one formal extra turn: %#v", state.ExtraTurnQueue)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
		t.Fatalf("process formal extra turn: %v", err)
	}
	actorPtr = battleUnitByID(state, "player:1")
	if actorPtr == nil || !battleHasBuff(actorPtr, "extra_turn_probability_reward") {
		t.Fatalf("probability outcome add-buff hook was suppressed in extra turn: actor=%#v", actorPtr)
	}
	if actorPtr.HP != 51 {
		t.Fatalf("extra-turn proc did not settle exactly once, hp=%v", actorPtr.HP)
	}
	if state.ProbabilitySuccessCounts[probeSuccessKey] != 1 || len(state.ProbabilitySuccessCounts) != 1 {
		t.Fatalf("extra-turn success caps leaked into outer turn: %#v", state.ProbabilitySuccessCounts)
	}
	if state.BattleHookTurnCounts[rewardTurnKey] != 1 {
		t.Fatalf("extra-turn hook limits overwrote outer turn count: %#v", state.BattleHookTurnCounts)
	}
	for _, hook := range []battleEquipmentHookRule{exitHook, probeHook, rewardHook} {
		_, battleKey := battleHookCounterKeys(state, actorPtr, battleEquipmentHookCounterGroup(hook))
		if state.BattleHookBattleCounts[battleKey] != 1 {
			t.Fatalf("per-battle count did not survive extra turn for %s: %#v", hook.ID, state.BattleHookBattleCounts)
		}
	}
	if state.probabilityHookDepth != 0 {
		t.Fatalf("probability hook guard leaked after extra turn: %d", state.probabilityHookDepth)
	}
}

func TestProbabilitySuccessCapSkipsCheckWithoutFailureOrRNG(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID:      "failure_heal",
		Trigger: "on_probability_failure",
		Effects: []map[string]any{{"kind": "heal", "value": 10}},
	}}
	state := &battleRuntimeState{Seed: 4}
	check := battleProbabilityCheck{
		Kind:                battleProbabilityEquipmentProc,
		Actor:               &actor,
		BaseChance:          1,
		ProbabilityGroup:    "cat_sword_proc",
		MaxSuccessesPerTurn: 1,
	}
	first := s.resolveBattleProbability(state, check)
	second := s.resolveBattleProbability(state, check)
	if !first.Succeeded || second.Established || state.RNGIndex != 0 {
		t.Fatalf("first=%#v second=%#v rngIndex=%d", first, second, state.RNGIndex)
	}
	actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, second)
	if len(actions) != 0 || actor.HP != 50 {
		t.Fatalf("capped check must not dispatch failure, actions=%#v hp=%v", actions, actor.HP)
	}
}

func TestProbabilityOutcomeHooksSettleAfterMainEffectAndRespectSharedLimit(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:                 "success_heal_a",
			Trigger:            "on_probability_success",
			Priority:           10,
			MaxTriggersPerTurn: 1,
			SharedTriggerGroup: "success_reward",
			Effects:            []map[string]any{{"kind": "heal", "value": 10}},
		},
		{
			ID:                 "success_heal_b",
			Trigger:            "on_probability_success",
			Priority:           0,
			MaxTriggersPerTurn: 1,
			SharedTriggerGroup: "success_reward",
			Effects:            []map[string]any{{"kind": "heal", "value": 20}},
		},
	}
	state := &battleRuntimeState{Wave: 1}
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 1, SourceIDs: []string{"cat_sword"}}
	result := s.resolveBattleProbability(state, check)
	if actor.HP != 50 {
		t.Fatalf("outcome hook ran before main effect/finalization, hp=%v", actor.HP)
	}
	actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if actor.HP != 60 || len(actions) != 1 {
		t.Fatalf("shared group should allow only first hook, hp=%v actions=%#v", actor.HP, actions)
	}
	if meta := normalizeAnyMap(actions[0]["meta"]); meta["probabilityKind"] != battleProbabilityEquipmentProc {
		t.Fatalf("probability metadata missing: %#v", meta)
	}

	state.beginBattleHookTurn()
	result = s.resolveBattleProbability(state, check)
	actions = s.battleFinalizeProbabilityResult(state, time.UnixMilli(2), check, result)
	if actor.HP != 70 || len(actions) != 1 {
		t.Fatalf("per-turn limit did not reset, hp=%v actions=%#v", actor.HP, actions)
	}
}

func TestProbabilityOutcomeMaxPerBattlePersistsAcrossWaves(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID:                   "once_per_battle_reward",
		Trigger:              "on_probability_success",
		MaxTriggersPerBattle: 1,
		Effects:              []map[string]any{{"kind": "heal", "value": 5}},
	}}
	state := &battleRuntimeState{Wave: 7}
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 1}
	for i := 0; i < 2; i++ {
		state.beginBattleHookTurn()
		result := s.resolveBattleProbability(state, check)
		s.battleFinalizeProbabilityResult(state, time.UnixMilli(int64(i+1)), check, result)
	}
	if roundTo4(actor.HP) != 55 {
		t.Fatalf("battle limit allowed repeated reward, hp=%v", actor.HP)
	}
	state.Wave = 8
	state.beginBattleHookTurn()
	result := s.resolveBattleProbability(state, check)
	s.battleFinalizeProbabilityResult(state, time.UnixMilli(3), check, result)
	if roundTo4(actor.HP) != 55 {
		t.Fatalf("battle-scoped limit reset at a wave boundary, hp=%v", actor.HP)
	}
	newBattle := &battleRuntimeState{Wave: 1}
	newBattle.beginBattleHookTurn()
	result = s.resolveBattleProbability(newBattle, check)
	s.battleFinalizeProbabilityResult(newBattle, time.UnixMilli(4), check, result)
	if roundTo4(actor.HP) != 60 {
		t.Fatalf("fresh battle state did not reset battle-scoped limit, hp=%v", actor.HP)
	}
}

func TestProbabilityBuffOutcomeFiltersAndLimits(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":                 "lucky_success_resource",
			"buffId":             "lucky_blessing",
			"trigger":            "on_probability_success",
			"maxTriggersPerTurn": 2,
			"eventFilter":        map[string]any{"probabilityKinds": []string{"equipment_proc"}},
			"effects": []map[string]any{{
				"kind":     "resource",
				"resource": "mp",
				"amountFormula": map[string]any{
					"terms": []map[string]any{
						{"attrKey": "max_mp", "coefficient": 0.05},
						{"attrKey": "current_mp", "coefficient": -0.05},
					},
				},
			}},
		},
	}
	actor := probabilityTestUnit("player:1")
	actor.Buffs = []battleBuffState{{BuffID: "lucky_blessing", RemainTurns: 2}}
	state := &battleRuntimeState{Wave: 1}
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &actor, BaseChance: 1}
	for i := 0; i < 3; i++ {
		result := s.resolveBattleProbability(state, check)
		s.battleFinalizeProbabilityResult(state, time.UnixMilli(int64(i+1)), check, result)
	}
	// 10 -> 14.5 -> 18.775. The third success is over the hook limit.
	if roundTo4(actor.MP) != 18.775 {
		t.Fatalf("limited missing-MP restores produced mp=%v, want 18.775", actor.MP)
	}
}

func TestProbabilityOutcomeDoesNotRetroactivelyTriggerNewBuff(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id":      "lucky_success_heal",
		"buffId":  "lucky_blessing",
		"trigger": "on_probability_success",
		"effects": []map[string]any{{"kind": "heal", "value": 10}},
	}}
	actor := probabilityTestUnit("player:1")
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID:                   "grant_lucky_after_success",
		Trigger:              "on_probability_success",
		MaxTriggersPerBattle: 1,
		Effects: []map[string]any{{
			"kind": "add_buff", "buffId": "lucky_blessing", "durationTurns": 2, "targetMode": "self",
		}},
	}}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
	actor = state.Players[0]
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: &state.Players[0], BaseChance: 1}

	result := s.resolveBattleProbability(state, check)
	s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if findBuffByID(&state.Players[0], "lucky_blessing") == nil {
		t.Fatal("success outcome did not grant lucky blessing")
	}
	if state.Players[0].HP != actor.HP {
		t.Fatalf("new lucky blessing responded retroactively, hp=%v want=%v", state.Players[0].HP, actor.HP)
	}

	result = s.resolveBattleProbability(state, check)
	s.battleFinalizeProbabilityResult(state, time.UnixMilli(2), check, result)
	if state.Players[0].HP != actor.HP+10 {
		t.Fatalf("active lucky blessing did not respond to the next success, hp=%v want=%v", state.Players[0].HP, actor.HP+10)
	}
}

func TestRuntimeDamageAmpUsesAuthoritativeEquipmentProbability(t *testing.T) {
	s := &Server{}
	actor := probabilityTestUnit("player:1")
	target := probabilityTestUnit("enemy:1")
	target.Side = "enemy"
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:                 "cat_sword_proc",
			SourceItemID:       "cat_sword",
			SourceEnhanceLevel: 40,
			Trigger:            "on_before_deal_damage",
			Effects: []map[string]any{{
				"kind":                "runtime_damage_amp",
				"value":               3.0,
				"valueFormula":        map[string]any{"baseAdd": 0, "terms": []map[string]any{{"attrKey": "equipment_enhance_level", "coefficient": 0.05}}},
				"valueMin":            3.0,
				"valueMax":            5.5,
				"procChanceBase":      1.0,
				"probabilityGroup":    "cat_sword_proc",
				"maxSuccessesPerTurn": 1,
			}},
		},
	}
	state := &battleRuntimeState{Wave: 1, Seed: 8}
	amp, actions, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooks(state, time.UnixMilli(1), &actor, &target, "on_before_deal_damage", 100, "physical")
	if amp != 5 || len(actions) != 1 || state.RNGIndex != 0 {
		t.Fatalf("first proc amp=%v actions=%#v rngIndex=%d", amp, actions, state.RNGIndex)
	}
	amp, actions, _ = s.battleCollectRuntimeDamageAmpFromEquipmentHooks(state, time.UnixMilli(2), &actor, &target, "on_before_deal_damage", 100, "physical")
	if amp != 0 || len(actions) != 0 || state.RNGIndex != 0 {
		t.Fatalf("capped proc amp=%v actions=%#v rngIndex=%d", amp, actions, state.RNGIndex)
	}
}

func TestRuntimeDamageAmpOutcomesSettleAfterMainDamage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		chance     float64
		outcome    string
		expectedHP float64
	}{
		{name: "success", chance: 1, outcome: "on_probability_success", expectedHP: 83},
		{name: "failure", chance: 0, outcome: "on_probability_failure", expectedHP: 85},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			target := probabilityTestUnit("player:target")
			target.HP = 95
			target.EquipmentHooks = []battleEquipmentHookRule{
				{
					ID:      "probabilistic_incoming_amp",
					Trigger: "on_before_take_damage",
					Effects: []map[string]any{{
						"kind":           "runtime_damage_amp",
						"value":          0.1,
						"procChanceBase": tc.chance,
					}},
				},
				{
					ID:      "heal_after_probability_result",
					Trigger: tc.outcome,
					Effects: []map[string]any{{
						"kind":       "heal",
						"value":      10,
						"targetMode": "self",
					}},
				},
			}
			source := probabilityTestUnit("enemy:source")
			source.Side = "enemy"
			source.OriginSide = "enemy"
			source.BaseSide = "enemy"
			source.PerspectiveSide = "enemy"
			state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{target}, Enemies: []battleUnitState{source}}
			runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)
			runtime.ApplyDamage(battleskills.DamageAction{
				SourceID:   source.UnitID,
				TargetID:   target.UnitID,
				Amount:     20,
				DamageType: "physical",
				Reason:     "probability_timing_test",
			})
			if state.Players[0].HP != tc.expectedHP {
				t.Fatalf("result heal settled before main damage: hp=%v, want %v", state.Players[0].HP, tc.expectedHP)
			}
			if state.RNGIndex != 0 {
				t.Fatalf("0%%/100%% proc consumed RNG: %d", state.RNGIndex)
			}
			damageIndex := -1
			healIndex := -1
			for index, action := range runtime.ActionLog() {
				switch strings.TrimSpace(fmt.Sprintf("%v", action["kind"])) {
				case "damage":
					if strings.TrimSpace(fmt.Sprintf("%v", action["reason"])) == "probability_timing_test" {
						damageIndex = index
					}
				case "heal":
					healIndex = index
				}
			}
			if damageIndex < 0 || healIndex <= damageIndex {
				t.Fatalf("probability outcome timeline must follow main damage: %#v", runtime.ActionLog())
			}
		})
	}
}

func TestRuntimeDamageAmpWithoutOutcomeHookKeepsLegacyTimeline(t *testing.T) {
	s := &Server{}
	target := probabilityTestUnit("player:target")
	target.HP = 95
	target.EquipmentHooks = []battleEquipmentHookRule{{
		ID:      "probabilistic_incoming_amp",
		Trigger: "on_before_take_damage",
		Effects: []map[string]any{{"kind": "runtime_damage_amp", "value": 0.1, "procChanceBase": 1}},
	}}
	source := probabilityTestUnit("enemy:source")
	source.Side = "enemy"
	source.OriginSide = "enemy"
	source.BaseSide = "enemy"
	source.PerspectiveSide = "enemy"
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{target}, Enemies: []battleUnitState{source}}
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: source.UnitID, TargetID: target.UnitID, Amount: 20, DamageType: "physical", Reason: "legacy_timeline_test",
	})
	actions := runtime.ActionLog()
	if state.Players[0].HP != 73 || state.RNGIndex != 0 {
		t.Fatalf("unmodified 100%% proc settlement drifted: hp=%v rng=%d", state.Players[0].HP, state.RNGIndex)
	}
	if len(actions) != 2 || strings.TrimSpace(fmt.Sprintf("%v", actions[0]["kind"])) != "trigger" || strings.TrimSpace(fmt.Sprintf("%v", actions[1]["kind"])) != "damage" {
		t.Fatalf("legacy trigger/damage timeline changed without result hooks: %#v", actions)
	}
}

func TestDarkCatSwordFailureHookHitsEveryoneElseAtMostFiveTimes(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Wave: 1,
		Players: []battleUnitState{
			probabilityTestUnit("player:dark-cat"),
			probabilityTestUnit("player:ally"),
		},
		Enemies: []battleUnitState{probabilityTestUnit("enemy:1")},
	}
	for index := range state.Players {
		state.Players[index].HP = 1000
		state.Players[index].MaxHP = 1000
		state.Players[index].BaseStats["hp"] = 1000
		state.Players[index].Stats["hp"] = 1000
	}
	state.Enemies[0].Side = "enemy"
	state.Enemies[0].OriginSide = "enemy"
	state.Enemies[0].BaseSide = "enemy"
	state.Enemies[0].PerspectiveSide = "enemy"
	state.Enemies[0].HP = 1000
	state.Enemies[0].MaxHP = 1000
	state.Enemies[0].BaseStats["hp"] = 1000
	state.Enemies[0].Stats["hp"] = 1000
	actor := &state.Players[0]
	actor.Stats["physics_damage"] = 100
	actor.BaseStats["physics_damage"] = 100
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:       "hollow_force_for_test",
			Trigger:  "on_before_probability_check",
			Priority: 1000,
			Effects:  []map[string]any{{"kind": "force_probability_failure"}},
		},
		{
			ID:                 "dark_cat_sword_failure_carnival",
			SourceItemID:       "dark_cat_sword",
			SourceEnhanceLevel: 10,
			Trigger:            "on_probability_failure",
			MaxTriggersPerTurn: 5,
			Effects: []map[string]any{{
				"kind":       "damage",
				"targetMode": "targeting",
				"targeting": map[string]any{
					"targetPositions": []string{"vanguard", "midguard", "rearguard"},
					"secondarySeek":   "none",
					"targetScope":     "all",
					"selectRule":      "first",
					"excludeSelf":     true,
					"targetCount":     map[string]any{"base": 999, "max": 999},
				},
				"damageType": "magical",
				"amountFormula": map[string]any{
					"terms": []map[string]any{{"attrKey": "physics_damage", "coefficient": 1.0, "perEnhanceLevel": 0.01}},
				},
				"dispatchEquipmentHooks": false,
				"ignoreProbabilityHooks": true,
				"simultaneous":           true,
			}},
		},
	}
	state.beginBattleHookTurn()
	check := battleProbabilityCheck{Kind: battleProbabilityEquipmentProc, Actor: actor, BaseChance: 1, SourceIDs: []string{"some_proc"}}
	for attempt := 0; attempt < 6; attempt++ {
		result := s.resolveBattleProbability(state, check)
		if !result.ForcedFailure {
			t.Fatalf("attempt %d was not force-failed: %#v", attempt, result)
		}
		s.battleFinalizeProbabilityResult(state, time.UnixMilli(int64(attempt+1)), check, result)
	}
	if actor.HP != 1000 {
		t.Fatalf("dark sword damaged its bearer, hp=%v", actor.HP)
	}
	if state.Players[1].HP != 450 || state.Enemies[0].HP != 450 {
		t.Fatalf("five 110-damage failures expected, ally=%v enemy=%v", state.Players[1].HP, state.Enemies[0].HP)
	}
	if state.RNGIndex != 0 {
		t.Fatalf("forced failures or deterministic targeting consumed RNG: %d", state.RNGIndex)
	}
}

func TestHollowHeartFailureStacksDarkHeartAtTwenty(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{{
		"id": "dark_heart", "durationTurns": 3, "stackPolicy": "stack", "maxStacks": 20,
	}}
	actor := probabilityTestUnit("player:hollow")
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:       "hollow_heart_deny_chance",
			Trigger:  "on_before_probability_check",
			Priority: 1000,
			Effects:  []map[string]any{{"kind": "force_probability_failure"}},
		},
		{
			ID:                 "hollow_heart_gather_darkness",
			SourceItemID:       "hollow_heart",
			SourceEnhanceLevel: 40,
			Trigger:            "on_probability_failure",
			Effects: []map[string]any{{
				"kind":          "add_buff",
				"buffId":        "dark_heart",
				"durationTurns": 3,
				"targetMode":    "self",
				"payloadFormula": map[string]any{
					"mainStatBonus": map[string]any{
						"baseAdd": 1,
						"terms":   []map[string]any{{"attrKey": "equipment_enhance_level", "coefficient": 0.025}},
					},
					"damageResistBonus": map[string]any{
						"baseAdd": 0.005,
						"terms":   []map[string]any{{"attrKey": "equipment_enhance_level", "coefficient": 0.0000625}},
					},
				},
			}},
		},
	}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
	actorPtr := &state.Players[0]
	check := battleProbabilityCheck{Kind: battleProbabilityStatusApply, Actor: actorPtr, BaseChance: 1, SourceIDs: []string{"status:test"}}
	for attempt := 0; attempt < 21; attempt++ {
		result := s.resolveBattleProbability(state, check)
		s.battleFinalizeProbabilityResult(state, time.UnixMilli(int64(attempt+1)), check, result)
	}
	if stacks := battleCountBuff(actorPtr, "dark_heart"); stacks != 20 {
		t.Fatalf("dark heart stacks=%d, want 20", stacks)
	}
	first := findBuffByID(actorPtr, "dark_heart")
	if first == nil || first.RemainTurns != 3 || first.Payload["mainStatBonus"] != 2.0 || first.Payload["damageResistBonus"] != 0.0075 {
		t.Fatalf("dark heart payload/duration=%#v", first)
	}
	if state.RNGIndex != 0 {
		t.Fatalf("forced status failures consumed RNG: %d", state.RNGIndex)
	}
}
