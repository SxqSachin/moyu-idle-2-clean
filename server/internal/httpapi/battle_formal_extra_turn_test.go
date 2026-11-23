package httpapi

import (
	"context"
	"testing"
	"time"
)

func TestFormalExtraTurnRunsAfterOwnerWithoutConsumingNormalOrder(t *testing.T) {
	s := &Server{}
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}

	targeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount:     battleSkillTargetCount{Base: 1},
	}
	damageEffect := func(id string, amount float64) battleSkillEffect {
		return battleSkillEffect{
			ID: id, Kind: "damage", DamageType: "physical", Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: amount},
			DamageSpec: battleDamageSpec{
				DamageType: "physical", AmountFormula: battleSkillAmountFormula{BaseAdd: amount},
				HitCount: battleSkillTargetCount{Base: 1},
			},
		}
	}
	burst := battleSkillState{
		ID: "skill:burst", Name: "Burst", Category: "attack", CooldownTurns: 3,
		Targeting: targeting, Effects: []battleSkillEffect{damageEffect("burst:damage", 10)},
	}
	basic := battleSkillState{
		ID: "basic_attack", Name: "Basic", Category: "basic", Targeting: targeting,
		Effects: []battleSkillEffect{damageEffect("basic:damage", 5)},
	}
	extraHook := battleEquipmentHookRule{
		ID: "formal_extra_turn_test", Trigger: "on_after_skill_cast", MaxTriggersPerBattle: 1,
		Effects: []map[string]any{{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"}},
	}
	actor := battleUnitState{
		UnitID: "player:actor", UserID: 1, UnitType: "player",
		Side: "player", OriginSide: "player", BaseSide: "player", PerspectiveSide: "player",
		Position: "vanguard", BasePosition: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, MP: 100, MaxMP: 100,
		Stats: map[string]float64{"hp": 100, "mp": 100, "speed": 20}, BaseStats: map[string]float64{"hp": 100, "mp": 100, "speed": 20},
		Skills: []battleSkillState{burst, basic}, SkillIDs: []string{burst.ID, basic.ID}, SkillSlots: []string{burst.ID, basic.ID},
		Buffs: []battleBuffState{{BuffID: "formal_turn_duration", RemainTurns: 3, SourceID: "test"}},
		SpecializationEffectMutations: []battleAffixEffectMutation{{
			ID: "specialization_cooldown_minus_one", SourceKind: "specialization", SourceID: "test_specialization_node",
			Scope: "skill", SelectAll: true, CooldownDelta: -1, CooldownFloor: 1,
		}},
		EquipmentHooks: []battleEquipmentHookRule{extraHook},
	}
	enemy := battleUnitState{
		UnitID: "enemy:next", UnitType: "enemy",
		Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
		Position: "vanguard", BasePosition: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, MP: 0, MaxMP: 0,
		Stats: map[string]float64{"hp": 100, "speed": 10}, BaseStats: map[string]float64{"hp": 100, "speed": 10},
		Skills: []battleSkillState{basic}, SkillIDs: []string{basic.ID}, SkillSlots: []string{basic.ID},
	}
	state := &battleRuntimeState{
		Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{enemy},
		Order: []string{actor.UnitID, enemy.UnitID}, FullOrder: []string{actor.UnitID, enemy.UnitID},
	}

	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(1), nil); err != nil {
		t.Fatalf("process normal turn: %v", err)
	}
	first := lastBattleTurnActionData(t, state)
	if first["actorId"] != actor.UnitID || first["turnKind"] != "ordered" {
		t.Fatalf("first action should be the owner's normal turn: %#v", first)
	}
	if state.TurnCursor != 1 || len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("normal cursor should already point at the untouched next unit: cursor=%d queue=%#v", state.TurnCursor, state.ExtraTurnQueue)
	}
	if got := state.Players[0].Skills[0].CooldownRemain; got != 2 {
		t.Fatalf("specialization -CD was not used for the authoritative cast cooldown: got=%d want=2", got)
	}
	if got := findBuffByID(&state.Players[0], "formal_turn_duration").RemainTurns; got != 2 {
		t.Fatalf("normal turn did not tick buff duration: got=%d want=2", got)
	}
	markerFound := false
	for _, row := range battleAnyMapSlice(first["actionLog"]) {
		if row["kind"] != "extra_turn" {
			continue
		}
		markerFound = true
		if !parseBooleanLoose(row["queued"], false) {
			t.Fatalf("extra turn grant was not marked as queued: %#v", row)
		}
		if _, nested := row["turn"]; nested {
			t.Fatalf("extra turn grant must not contain a nested child turn: %#v", row)
		}
	}
	if !markerFound {
		t.Fatalf("normal turn did not expose the extra-turn grant: %#v", first)
	}

	publicAfterNormal := battlePublicUnitSlice(state.Players)
	publicSkills := battleAnyMapSlice(publicAfterNormal[0]["skills"])
	if len(publicSkills) == 0 || toInt64Loose(publicSkills[0]["cooldownTurns"], 0) != 2 {
		t.Fatalf("public battle state did not expose effective -CD total: %#v", publicSkills)
	}
	compactPlayers := compactBattleSnapshotUnits(publicAfterNormal)
	if len(compactPlayers) != 1 {
		t.Fatalf("compact snapshot omitted the player: %#v", compactPlayers)
	}
	compactCooldownTurns := normalizeAnyMap(compactPlayers[0]["st"])
	if toInt64Loose(compactCooldownTurns[burst.ID], -1) != 2 {
		t.Fatalf("compact snapshot omitted the effective -CD total: %#v", compactCooldownTurns)
	}
	if basicTotal, ok := compactCooldownTurns[basic.ID]; !ok || toInt64Loose(basicTotal, -1) != 0 {
		t.Fatalf("compact snapshot must preserve authoritative zero cooldown totals: %#v", compactCooldownTurns)
	}
	checkpoint := &battlePlaybackCheckpoint{
		Players: publicAfterNormal,
		Enemies: battlePublicUnitSlice(state.Enemies),
		Order:   append([]string(nil), state.Order...),
	}

	restored := decodeBattleRuntimeState(state.toMap())
	state = &restored
	if len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("formal extra turn did not survive runtime persistence: %#v", state.ExtraTurnQueue)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
		t.Fatalf("process formal extra turn: %v", err)
	}
	extra := lastBattleTurnActionData(t, state)
	if extra["actorId"] != actor.UnitID || extra["turnKind"] != "extra" || !parseBooleanLoose(extra["extraTurn"], false) {
		t.Fatalf("queued action was not emitted as an independent formal extra turn: %#v", extra)
	}
	if _, nested := extra["parentSeq"]; nested {
		t.Fatalf("formal extra turn must not be attached to the previous action: %#v", extra)
	}
	if state.TurnCursor != 1 || len(state.ExtraTurnQueue) != 0 {
		t.Fatalf("extra turn consumed the normal-order cursor: cursor=%d queue=%#v", state.TurnCursor, state.ExtraTurnQueue)
	}
	if got := state.Players[0].Skills[0].CooldownRemain; got != 1 {
		t.Fatalf("formal extra turn did not tick skill cooldown: got=%d want=1", got)
	}
	if got := findBuffByID(&state.Players[0], "formal_turn_duration").RemainTurns; got != 1 {
		t.Fatalf("formal extra turn did not tick buff duration: got=%d want=1", got)
	}
	if got := state.CompletedTurnCounts[actor.UnitID]; got != 2 {
		t.Fatalf("formal extra turn did not count as a completed turn: got=%d want=2", got)
	}

	node := buildBattleTimelineNodeData(battleEvent{}, extra, nil, nil)
	if !parseBooleanLoose(node["et"], false) || toInt64Loose(normalizeAnyMap(node["sc"])[burst.ID], -1) != 1 || toInt64Loose(normalizeAnyMap(node["st"])[burst.ID], -1) != 2 {
		t.Fatalf("timeline omitted authoritative extra-turn cooldown state: %#v", node)
	}
	applyPlaybackTimelineNode(checkpoint, node)
	playbackCooldowns := normalizeAnyMap(checkpoint.Players[0]["skillCooldowns"])
	playbackCooldownTurns := normalizeAnyMap(checkpoint.Players[0]["skillCooldownTurns"])
	playbackSkills := battleAnyMapSlice(checkpoint.Players[0]["skills"])
	if toInt64Loose(playbackCooldowns[burst.ID], -1) != 1 || toInt64Loose(playbackCooldownTurns[burst.ID], -1) != 2 || toInt64Loose(playbackSkills[0]["cooldownTurns"], -1) != 2 {
		t.Fatalf("playback did not use authoritative cooldown state: cooldowns=%#v totals=%#v skills=%#v", playbackCooldowns, playbackCooldownTurns, playbackSkills)
	}

	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(3), nil); err != nil {
		t.Fatalf("process original next unit: %v", err)
	}
	next := lastBattleTurnActionData(t, state)
	if next["actorId"] != enemy.UnitID || next["turnKind"] != "ordered" {
		t.Fatalf("original next unit did not retain its normal turn: %#v", next)
	}
}

func TestExtraTurnGrantedDuringExtraTurnPrecedesOlderQueueEntries(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:current", Side: "player", Alive: true, HP: 100, MaxHP: 100},
			{UnitID: "player:child", Side: "player", Alive: true, HP: 100, MaxHP: 100},
		},
		ExtraTurnQueue: []battleQueuedExtraTurn{{ActorID: "player:older", TurnMode: "full", ChainDepth: 1}},
		activeExtraTurnRequest: &battleQueuedExtraTurn{
			ActorID: "player:current", TurnMode: "full", ChainDepth: 4,
			ChainHookCounts: map[string]int{"existing": 1},
		},
		activeExtraTurnUnitIDs: map[string]int{"player:current": 1},
	}

	for index, hookID := range []string{"child:first", "child:second"} {
		if _, ok := s.executeEquipmentExtraTurn(
			state,
			time.UnixMilli(int64(index+1)),
			&state.Players[1],
			"on_other_unit_turn_end",
			battleEquipmentHookRule{ID: hookID},
			nil,
			"full",
			true,
		); !ok {
			t.Fatalf("queue child extra turn %d", index+1)
		}
	}

	if len(state.ExtraTurnQueue) != 3 {
		t.Fatalf("child turns were not inserted ahead of the older queue: %#v", state.ExtraTurnQueue)
	}
	if state.ExtraTurnQueue[0].HookID != "child:first" || state.ExtraTurnQueue[1].HookID != "child:second" || state.ExtraTurnQueue[2].ActorID != "player:older" {
		t.Fatalf("child turns did not preserve immediate grant order: %#v", state.ExtraTurnQueue)
	}
	for _, request := range state.ExtraTurnQueue[:2] {
		if request.ChainDepth != 5 || request.ChainHookCounts["existing"] != 1 {
			t.Fatalf("child turn lost its causal chain boundary: %#v", request)
		}
	}

	state.activeExtraTurnRequest.ChainDepth = battleMaxQueuedExtraTurnsPerChain
	before := len(state.ExtraTurnQueue)
	if _, ok := s.executeEquipmentExtraTurn(
		state,
		time.UnixMilli(3),
		&state.Players[1],
		"on_other_unit_turn_end",
		battleEquipmentHookRule{ID: "child:overflow"},
		nil,
		"full",
		true,
	); ok || len(state.ExtraTurnQueue) != before {
		t.Fatalf("chain-depth cap did not reject another child: queue=%#v", state.ExtraTurnQueue)
	}
}
