package httpapi

import (
	"testing"
	"time"
)

func TestExecuteUnitTurnAppliesBuffTurnEventFilters(t *testing.T) {
	tests := []struct {
		name                      string
		turnKind                  string
		isPreparation             bool
		actionSuppressed          bool
		includeSuppressedTurns    bool
		wantResourceHookTriggered bool
	}{
		{
			name:                      "ordered turn triggers",
			turnKind:                  "ordered",
			includeSuppressedTurns:    true,
			wantResourceHookTriggered: true,
		},
		{
			name:                      "suppressed ordered turn triggers when included",
			turnKind:                  "ordered",
			actionSuppressed:          true,
			includeSuppressedTurns:    true,
			wantResourceHookTriggered: true,
		},
		{
			name:                   "suppressed ordered turn is filtered when excluded",
			turnKind:               "ordered",
			actionSuppressed:       true,
			includeSuppressedTurns: false,
		},
		{
			name:     "extra turn is filtered",
			turnKind: "extra",
		},
		{
			name:          "preparation turn is filtered",
			turnKind:      "ordered",
			isPreparation: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &Server{}
			s.deps.GameData.Buff = []map[string]any{
				{"id": "turn_counter", "durationTurns": -1, "permanent": true},
				{"id": "turn_suppression", "durationTurns": -1, "permanent": true},
			}
			s.deps.GameData.BuffEffects = []map[string]any{
				{
					"id":      "turn_counter_on_turn_end",
					"buffId":  "turn_counter",
					"trigger": "on_turn_end",
					"eventFilter": map[string]any{
						"turnKinds":              []any{"ordered"},
						"includePreparation":     false,
						"includeSuppressedTurns": test.includeSuppressedTurns,
					},
					"effects": []map[string]any{{"kind": "resource", "resource": "mp", "value": 7}},
				},
				{
					"id":      "turn_suppression_while_active",
					"buffId":  "turn_suppression",
					"trigger": "while_active",
					"effects": []map[string]any{{"kind": "turn_action_suppressed"}},
				},
			}

			actor := battleUnitState{
				UnitID:   "enemy:turn-filter",
				Side:     "enemy",
				BaseSide: "enemy",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MaxMP:    100,
				Stats:    map[string]float64{"hp": 100, "mp": 100, "speed": 10},
				Buffs:    []battleBuffState{{BuffID: "turn_counter", RemainTurns: -1}},
			}
			if test.actionSuppressed {
				actor.Buffs = append(actor.Buffs, battleBuffState{BuffID: "turn_suppression", RemainTurns: -1})
			}
			state := &battleRuntimeState{
				Seed:    1,
				Wave:    1,
				Enemies: []battleUnitState{actor},
			}
			if test.turnKind == "extra" {
				state.activeExtraTurnUnitIDs = map[string]int{actor.UnitID: 1}
			}

			s.executeUnitTurn(state, time.UnixMilli(1), &state.Enemies[0], "enemy", test.isPreparation)

			wantMP := 0.0
			if test.wantResourceHookTriggered {
				wantMP = 7
			}
			if got := roundTo4(state.Enemies[0].MP); got != wantMP {
				t.Fatalf("mp=%v, want %v", got, wantMP)
			}
		})
	}
}
