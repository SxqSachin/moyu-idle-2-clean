package httpapi

import (
	"testing"
	"time"
)

func encoreEchoMaskTestHook(enhanceLevel int) battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID:                     "encore_echo_mask_reprise",
		SourceKind:             "equipment",
		SourceItemID:           "encore_echo_mask",
		SourceEnhanceLevel:     enhanceLevel,
		Trigger:                "on_other_unit_cast_skill",
		ActivationCost:         battleHookActivationCost{MaxHPRatio: 0.1, MaxMPRatio: 0.15},
		CooldownCompletedTurns: 2,
		Effects: []map[string]any{{
			"kind":                       "mirror_cast_skill",
			"triggerScope":               "ally",
			"targetingMode":              "reselect",
			"amountRatioBase":            0.4,
			"amountRatioPerEnhanceLevel": 0.0025,
			"mpCostMultiplier":           0,
			"hpCostMultiplier":           0,
			"includeBasicAttack":         false,
			"allowMirroredCastTrigger":   false,
			"applyCooldown":              false,
			"dispatchHooks":              false,
		}},
	}
}

func triggerEncoreEchoMaskForTest(s *Server, state *battleRuntimeState, parentSeq int64) {
	s.executeMirrorCastsForAction(
		state,
		time.UnixMilli(parentSeq),
		&state.Players[0],
		"player",
		map[string]any{"skillId": "arcane_bolt", "resultMeta": map[string]any{}},
		parentSeq,
	)
}

func TestEncoreEchoMaskPaysAtomicCostAndUsesCompletedTurnCooldown(t *testing.T) {
	s := newMirrorCastTestServer()
	state := newMirrorCastTestState()
	mask := &state.Players[1]
	state.CompletedTurnCounts = map[string]int{}
	mask.EquipmentHooks = []battleEquipmentHookRule{encoreEchoMaskTestHook(40)}
	mask.Skills[0].HPCost = 10

	triggerEncoreEchoMaskForTest(s, state, 1)
	if got := len(battleTurnActionEventsForTest(state)); got != 1 {
		t.Fatalf("first allied skill should produce one reprise, got %d events=%#v", got, state.Events)
	}
	if mask.HP != 90 || mask.MP != 85 {
		t.Fatalf("reprise should pay only 10%% max HP and 15%% max MP, hp=%v mp=%v", mask.HP, mask.MP)
	}
	meta := normalizeAnyMap(battleTurnActionEventsForTest(state)[0].Data["resultMeta"])
	if got := toFloat64(meta["amountRatio"], 0); got != 0.5 {
		t.Fatalf("+40 reprise ratio=%v, want 0.5", got)
	}
	if got := mask.Skills[0].CooldownRemain; got != 0 {
		t.Fatalf("reprise must not start the copied skill cooldown, got %d", got)
	}

	triggerEncoreEchoMaskForTest(s, state, 2)
	if got := len(battleTurnActionEventsForTest(state)); got != 1 {
		t.Fatalf("hard cooldown should block an immediate reprise, got %d", got)
	}
	state.CompletedTurnCounts[mask.UnitID] = 1
	triggerEncoreEchoMaskForTest(s, state, 3)
	if got := len(battleTurnActionEventsForTest(state)); got != 1 {
		t.Fatalf("one completed turn must not clear a two-turn cooldown, got %d", got)
	}
	state.CompletedTurnCounts[mask.UnitID] = 2
	triggerEncoreEchoMaskForTest(s, state, 4)
	if got := len(battleTurnActionEventsForTest(state)); got != 2 {
		t.Fatalf("two completed turns should unlock the next reprise, got %d", got)
	}
	if mask.HP != 80 || mask.MP != 70 {
		t.Fatalf("second reprise cost mismatch, hp=%v mp=%v", mask.HP, mask.MP)
	}
}

func TestEncoreEchoMaskInsufficientResourcesDoNotPartiallySettle(t *testing.T) {
	for _, tc := range []struct {
		name string
		hp   float64
		mp   float64
	}{
		{name: "hp", hp: 10, mp: 100},
		{name: "mp", hp: 100, mp: 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMirrorCastTestServer()
			state := newMirrorCastTestState()
			mask := &state.Players[1]
			mask.EquipmentHooks = []battleEquipmentHookRule{encoreEchoMaskTestHook(0)}
			mask.HP = tc.hp
			mask.MP = tc.mp

			triggerEncoreEchoMaskForTest(s, state, 1)
			if got := len(battleTurnActionEventsForTest(state)); got != 0 {
				t.Fatalf("insufficient %s should block reprise, got %d", tc.name, got)
			}
			if mask.HP != tc.hp || mask.MP != tc.mp {
				t.Fatalf("failed activation changed resources: hp=%v mp=%v", mask.HP, mask.MP)
			}
			if len(state.BattleHookCooldownUntilTurns) != 0 {
				t.Fatalf("failed activation started cooldown: %#v", state.BattleHookCooldownUntilTurns)
			}
		})
	}
}
