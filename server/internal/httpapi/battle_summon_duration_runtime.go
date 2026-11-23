package httpapi

import (
	"strings"
	"time"
)

// battleTickSourceBoundSkipTurnSummons advances summons that never receive an
// ordered turn. Acting summons keep the existing owner-independent duration.
func (s *Server) battleTickSourceBoundSkipTurnSummons(
	state *battleRuntimeState,
	now time.Time,
	source *battleUnitState,
	isPreparationTurn bool,
) []map[string]any {
	if s == nil || state == nil || source == nil || isPreparationTurn {
		return nil
	}
	sourceID := strings.TrimSpace(source.UnitID)
	if sourceID == "" {
		return nil
	}
	actions := make([]map[string]any, 0, 2)
	tick := func(unit *battleUnitState) {
		if unit == nil || !unit.Alive || unit.HP <= 0 || !unit.SkipTurnOrder {
			return
		}
		if strings.TrimSpace(unit.SummonTemplateUnitID) == "" || strings.TrimSpace(unit.SummonSourceUnitID) != sourceID {
			return
		}
		bindingIndex := -1
		for index := range unit.Buffs {
			if strings.EqualFold(strings.TrimSpace(unit.Buffs[index].BuffID), battleSummonBindingBuffID) {
				bindingIndex = index
				break
			}
		}
		if bindingIndex < 0 {
			return
		}
		remainTurns := unit.Buffs[bindingIndex].RemainTurns
		if remainTurns < 0 {
			return
		}
		if remainTurns > 0 {
			remainTurns--
		}
		unit.SummonRemainTurns = maxInt64(0, remainTurns)
		if remainTurns > 0 {
			unit.Buffs[bindingIndex].RemainTurns = remainTurns
			return
		}

		unit.Buffs = battleRemoveBuffByID(unit.Buffs, battleSummonBindingBuffID)
		expiredBuffIDs := []string{battleSummonBindingBuffID}
		actions = append(actions, buildBuffDetachActionLog(unit.UnitID, expiredBuffIDs, "duration_expired")...)
		actions = append(actions, s.battleTriggerUnitBuffHooks(state, now, unit, "on_detach", expiredBuffIDs...)...)
		_, expiryActions := s.battleApplyBuffExpiry(unit, expiredBuffIDs)
		actions = append(actions, expiryActions...)
	}
	for index := range state.Players {
		tick(&state.Players[index])
	}
	for index := range state.Enemies {
		tick(&state.Enemies[index])
	}
	if len(actions) == 0 {
		return nil
	}
	return actions
}
