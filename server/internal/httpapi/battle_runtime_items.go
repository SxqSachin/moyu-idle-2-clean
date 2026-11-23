package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mysecondwebgame/server/internal/rules"
	"mysecondwebgame/server/internal/store"
)

type battleItemBagSettlementStats struct {
	UsersTouched int
	EntriesSeen  int
	RowsUpdated  int
}

type battleItemBagQuantityWriter interface {
	SetBattleItemBagQuantities(context.Context, []store.BattleItemBagQuantityUpdate) error
}

func (s *Server) battleItemByItemID(itemID string) battleItemState {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return battleItemState{}
	}
	item := s.deps.GameData.ItemByID(itemID)
	if item == nil {
		return battleItemState{}
	}
	battleUsableItemID := strings.TrimSpace(item.BattleUsableItemID)
	if battleUsableItemID == "" {
		return battleItemState{}
	}
	out := battleItemState{
		ID:     battleUsableItemID,
		ItemID: itemID,
		Name:   itemID,
		Icon:   strings.TrimSpace(item.IconID),
		Targeting: battleSkillTargeting{
			TargetPositions: []string{"vanguard"},
			SecondarySeek:   "front_to_rear",
			TargetScope:     "self",
			SelectRule:      "self_first",
			TargetCount:     battleSkillTargetCount{Base: 1},
		},
		Effects: []battleSkillEffect{},
	}
	for _, raw := range s.deps.GameData.BattleUsableItems {
		id, _ := stringFromAny(raw["id"])
		if strings.TrimSpace(id) != battleUsableItemID {
			continue
		}
		if name, _ := stringFromAny(raw["name"]); strings.TrimSpace(name) != "" {
			out.Name = strings.TrimSpace(name)
		}
		if icon, _ := stringFromAny(raw["icon"]); strings.TrimSpace(icon) != "" {
			out.Icon = strings.TrimSpace(icon)
		}
		rawCD := toInt64Loose(firstNonNil(raw["cooldownTurns"], raw["cooldown"]), 0)
		if rawCD > 0 {
			out.CooldownTurns = int(rawCD)
		}
		// Parse trigger-item fields
		if isTrig, _ := raw["isTrigger"].(bool); isTrig {
			out.IsTrigger = true
			out.PerWaveChance = clampFloat(toFloat64(raw["perWaveChance"], 0), 0, 1)
			out.MinWave = int(toInt64Loose(raw["minWave"], 1))
			if out.MinWave < 1 {
				out.MinWave = 1
			}
			out.IntervalWaves = int(toInt64Loose(raw["intervalWaves"], 0))
			if out.IntervalWaves < 0 {
				out.IntervalWaves = 0
			}
			out.DungeonIDs = parseStringSliceFromAny(raw["dungeonIds"])
			out.SpawnEnemyIDs = parseStringSliceFromAny(raw["spawnEnemyIds"])
			out.PartyTriggerGroup = strings.TrimSpace(fmt.Sprintf("%v", raw["partyTriggerGroup"]))
			if out.PartyTriggerGroup == "<nil>" {
				out.PartyTriggerGroup = ""
			}
		} else {
			out.Targeting = parseBattleSkillTargeting(raw["targeting"], "vanguard", "front_to_rear", 1, nil, 0, false, nil)
			out.Effects = parseBattleSkillEffects(raw["effects"], out.Targeting, "physical")
			if len(out.Effects) > 0 {
				out.Targeting = out.Effects[0].Targeting
			}
		}
		break
	}
	return out
}

func parseStringSliceFromAny(v any) []string {
	rawSlice, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rawSlice))
	for _, item := range rawSlice {
		s := strings.TrimSpace(fmt.Sprintf("%v", item))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (s *Server) buildBattleItemStatesFromBag(entries []store.BattleItemBagEntry) []battleItemState {
	if len(entries) == 0 {
		return []battleItemState{}
	}
	out := make([]battleItemState, 0, len(entries))
	for _, row := range entries {
		itemID := strings.TrimSpace(row.ItemID)
		if itemID == "" {
			continue
		}
		battleItem := s.battleItemByItemID(itemID)
		if strings.TrimSpace(battleItem.ItemID) == "" {
			continue
		}
		battleItem.Quantity = maxInt64(0, row.Quantity)
		out = append(out, battleItem)
	}
	return out
}

func settleBattleItemBagFromFinalResultWithWriter(
	ctx context.Context,
	writer battleItemBagQuantityWriter,
	finalResult map[string]any,
) (battleItemBagSettlementStats, error) {
	updates, stats := battleItemBagUpdatesFromFinalResult(finalResult)
	if len(updates) == 0 {
		return stats, nil
	}
	if err := writer.SetBattleItemBagQuantities(ctx, updates); err != nil {
		return stats, err
	}
	stats.RowsUpdated = len(updates)
	return stats, nil
}

func battleItemBagUpdatesFromFinalResult(finalResult map[string]any) ([]store.BattleItemBagQuantityUpdate, battleItemBagSettlementStats) {
	stats := battleItemBagSettlementStats{}
	players := decodeBattlePlayersFromFinalResult(finalResult)
	if len(players) == 0 {
		return nil, stats
	}
	updates := make([]store.BattleItemBagQuantityUpdate, 0, 8)
	for _, player := range players {
		if player.UserID <= 0 || len(player.BattleItems) == 0 {
			continue
		}
		stats.UsersTouched++
		for _, item := range player.BattleItems {
			stats.EntriesSeen++
			itemID := strings.TrimSpace(item.ItemID)
			if itemID == "" {
				continue
			}
			updates = append(updates, store.BattleItemBagQuantityUpdate{
				UserID:   player.UserID,
				ItemID:   itemID,
				Quantity: maxInt64(0, item.Quantity),
			})
		}
	}
	return updates, stats
}

func decrementBattleItemCooldowns(unit *battleUnitState) {
	if unit == nil {
		return
	}
	for idx := range unit.BattleItems {
		if unit.BattleItems[idx].CooldownRemain > 0 {
			unit.BattleItems[idx].CooldownRemain--
		}
		if unit.BattleItems[idx].CooldownRemain < 0 {
			unit.BattleItems[idx].CooldownRemain = 0
		}
	}
}

func (s *Server) tryExecuteBattleTurnItem(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	actorSide string,
	actionSuppressed bool,
) {
	if state == nil || actor == nil || actionSuppressed {
		return
	}
	item, itemIdx := s.battleSelectItem(state, *actor, actorSide)
	if itemIdx < 0 || strings.TrimSpace(item.ItemID) == "" || itemIdx >= len(actor.BattleItems) {
		return
	}
	itemSkill := battleSkillState{
		ID:             item.ItemID,
		Name:           item.Name,
		Icon:           item.Icon,
		Level:          1,
		CooldownTurns:  item.CooldownTurns,
		CooldownRemain: item.CooldownRemain,
		Targeting:      item.Targeting,
		Effects:        append([]battleSkillEffect(nil), item.Effects...),
	}
	selected := &actor.BattleItems[itemIdx]
	selected.Resolving = true
	if selected.Quantity > 0 {
		selected.Quantity--
	}
	if selected.Quantity < 0 {
		selected.Quantity = 0
	}
	execution := s.executeBattleSkill(state, now, actor, itemSkill, "item_use", nil)
	selected.Resolving = false
	if selected.CooldownTurns > 0 {
		selected.CooldownRemain = selected.CooldownTurns + 1
	}
	itemPayload := buildBattleTurnItemPayload(state.Wave, actor, *selected, execution)
	populateTargetSlots(itemPayload["targets"].([]map[string]any), state, actorSide)
	state.emit("battle.turn.item_use", now, itemPayload)
}

func buildBattleTurnItemPayload(
	wave int,
	actor *battleUnitState,
	item battleItemState,
	execution battleSkillExecution,
) map[string]any {
	if actor == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"wave":         wave,
		"actorId":      actor.UnitID,
		"actorSide":    actor.Side,
		"actorUserId":  actor.UserID,
		"actorEnemyId": actor.EnemyID,
		"itemId":       item.ItemID,
		"itemName":     item.Name,
		"itemQuantity": maxInt64(0, item.Quantity),
		"actorHp":      roundTo4(actor.HP),
		"actorMp":      roundTo4(actor.MP),
		"actorAlive":   actor.Alive,
		"targets":      execution.Targets,
		"effects":      execution.Effects,
		"actionLog":    execution.Actions,
	}
	if len(execution.Meta) > 0 {
		payload["resultMeta"] = execution.Meta
	}
	return payload
}

func (s *Server) battleSelectItem(state *battleRuntimeState, unit battleUnitState, actorSide string) (battleItemState, int) {
	for idx, item := range unit.BattleItems {
		if !unit.Alive {
			break
		}
		if item.IsTrigger {
			continue // trigger items are passive; never actively used
		}
		if item.Resolving {
			continue
		}
		if item.Quantity <= 0 {
			continue
		}
		if item.CooldownRemain > 0 {
			continue
		}
		if !s.battleItemRulePassed(state, unit, actorSide, item) {
			continue
		}
		return item, idx
	}
	return battleItemState{}, -1
}

func (s *Server) battleItemRulePassed(state *battleRuntimeState, actor battleUnitState, actorSide string, item battleItemState) bool {
	key := strings.ToLower(strings.TrimSpace(item.ItemID))
	if key == "" {
		return true
	}
	ruleState, ok := actor.ItemRules[key]
	if !ok || len(ruleState.Groups) == 0 {
		return true
	}
	ctx := s.buildBattleRuleEvalContext(state, actor, actorSide, battleSkillState{
		ID:             item.ItemID,
		CooldownRemain: item.CooldownRemain,
	}, battleRuleNeedsUnitContext(ruleState.Groups))
	return rules.Evaluate(ruleState.Groups, ctx)
}

func (s *Server) loadBattleItemRules(ctx context.Context, userID int64) map[string]battleSkillRuleState {
	sets, err := s.deps.Store.ListRuleSets(ctx, userID, "item")
	if err != nil {
		return map[string]battleSkillRuleState{}
	}
	out := make(map[string]battleSkillRuleState, len(sets))
	for _, set := range sets {
		slotKey := strings.ToLower(strings.TrimSpace(set.SlotKey))
		if slotKey == "" {
			continue
		}
		buf, err := json.Marshal(set.Groups)
		if err != nil {
			continue
		}
		groups := []rules.ConditionGroup{}
		if err := json.Unmarshal(buf, &groups); err != nil {
			continue
		}
		out[slotKey] = battleSkillRuleState{
			Enabled: true,
			Groups:  groups,
		}
	}
	return out
}
