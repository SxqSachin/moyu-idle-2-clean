package httpapi

import (
	"sort"
	"strings"
)

func battleSummonsOldestFirst(state *battleRuntimeState, templateUnitID string, sourceUnitID string) []*battleUnitState {
	if state == nil {
		return nil
	}
	units := make([]*battleUnitState, 0, 4)
	appendMatches := func(rows []battleUnitState) {
		for index := range rows {
			unit := &rows[index]
			if battleUnitIsSummonedBy(unit, templateUnitID, sourceUnitID) {
				units = append(units, unit)
			}
		}
	}
	appendMatches(state.Players)
	appendMatches(state.Enemies)
	sort.SliceStable(units, func(i, j int) bool {
		left := units[i]
		right := units[j]
		if left.SummonOrder != right.SummonOrder {
			if left.SummonOrder <= 0 {
				return true
			}
			if right.SummonOrder <= 0 {
				return false
			}
			return left.SummonOrder < right.SummonOrder
		}
		if left.SummonOrder > 0 {
			return false
		}
		return strings.TrimSpace(left.UnitID) < strings.TrimSpace(right.UnitID)
	})
	return units
}

func battlePrepareSummonCapacity(
	state *battleRuntimeState,
	templateUnitID string,
	sourceUnitID string,
	maxCountPerUnit int,
	onCapOverflow string,
) ([]map[string]any, bool) {
	templateUnitID = strings.TrimSpace(templateUnitID)
	sourceUnitID = strings.TrimSpace(sourceUnitID)
	if state == nil || templateUnitID == "" || sourceUnitID == "" {
		return nil, false
	}
	if maxCountPerUnit <= 0 {
		return nil, true
	}
	existing := battleSummonsOldestFirst(state, templateUnitID, sourceUnitID)
	if len(existing) < maxCountPerUnit {
		return nil, true
	}
	if normalizeBattleSummonOnCapOverflow(onCapOverflow) == battleSummonOnCapOverflowFail {
		return nil, false
	}

	replaceCount := len(existing) - maxCountPerUnit + 1
	actions := make([]map[string]any, 0, replaceCount)
	for _, oldest := range existing[:replaceCount] {
		despawned, beforeHP := battleDespawnSummonedUnit(oldest)
		if !despawned {
			return actions, false
		}
		actions = append(actions, map[string]any{
			"kind":       "summon_expire",
			"targetId":   oldest.UnitID,
			"sourceId":   sourceUnitID,
			"reason":     "summon_cap_replaced",
			"beforeHp":   roundTo4(beforeHP),
			"afterHp":    roundTo4(oldest.HP),
			"afterMp":    roundTo4(oldest.MP),
			"afterAlive": oldest.Alive,
			"delayMs":    int64(0),
		})
	}
	return actions, true
}
