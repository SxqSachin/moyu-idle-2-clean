package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

const battleSkillExpMaxCastsPerUnit int64 = 1_000_000

type battleSkillExpSettlementStats struct {
	Users  int
	Skills int
	Exp    int64
}

func (s *Server) settleBattleSkillExpFromTimeline(ctx context.Context, timeline []wsbus.BattleTimelineEvent, finalResult map[string]any) (battleSkillExpSettlementStats, []battleSkillExpRewardRow, error) {
	_ = ctx
	rewards := battleSkillExpRewardsFromTimeline(timeline, finalResult, s.battleSkillMaxLevelByID)
	if len(rewards) == 0 {
		return battleSkillExpSettlementStats{}, nil, nil
	}
	stats := battleSkillExpSettlementStats{}
	rows := make([]battleSkillExpRewardRow, 0, len(rewards)*2)
	for _, perSkill := range rewards {
		if len(perSkill) == 0 {
			continue
		}
		stats.Users++
		for _, gain := range perSkill {
			if gain <= 0 {
				continue
			}
			stats.Skills++
			stats.Exp += gain
		}
	}
	for rewardKey, perSkill := range rewards {
		for skillID, gain := range perSkill {
			if gain <= 0 {
				continue
			}
			userID := rewardKey
			actorType := "player"
			actorID := int64(0)
			if rewardKey < 0 {
				userID = 0
				actorType = "kitty"
				actorID = -rewardKey
				gain = int64(math.Round(float64(gain) * s.deps.Config.SharedGlobalSettings.Kitty.BattleSkillExpMultiplier))
			}
			if gain <= 0 {
				continue
			}
			rows = append(rows, battleSkillExpRewardRow{
				UserID:    userID,
				ActorType: actorType,
				ActorID:   actorID,
				SkillID:   skillID,
				Exp:       gain,
			})
		}
	}
	return stats, rows, nil
}

func battleSkillExpRewardsFromTimeline(
	timeline []wsbus.BattleTimelineEvent,
	finalResult map[string]any,
	maxLevelLookup func(string) int,
) map[int64]map[string]int64 {
	if rewards, ok := battleSkillExpRewardsFromStats(finalResult, maxLevelLookup); ok {
		return rewards
	}
	players := decodeBattlePlayersFromFinalResult(finalResult)
	if len(players) == 0 || len(timeline) == 0 {
		return nil
	}
	unitToActor, skillLevelByUnitID := battleSkillExpActorIndexes(players)

	rewards := map[int64]map[string]int64{}
	for _, event := range timeline {
		eventType, data := battleTimelineEventPayload(event)
		if len(data) == 0 {
			continue
		}
		actorID, skillID, ok := battleSkillCastFromTimelineEvent(eventType, data)
		if !ok {
			continue
		}
		battleAccumulateSkillExpReward(rewards, unitToActor, skillLevelByUnitID, actorID, skillID, maxLevelLookup, store.BattleSkillExpPerUse)
	}
	return rewards
}

func battleSkillExpRewardsFromStats(
	finalResult map[string]any,
	maxLevelLookup func(string) int,
) (map[int64]map[string]int64, bool) {
	for _, rawStats := range []any{finalResult["battleStats"], finalResult["achievementStats"]} {
		stats := normalizeAnyMap(rawStats)
		castsByUnit := normalizeAnyMap(firstNonNil(stats["skillCastsByUnit"], stats["skill_casts_by_unit"]))
		if len(castsByUnit) == 0 {
			continue
		}
		return battleSkillExpRewardsFromCastsByUnit(finalResult, castsByUnit, maxLevelLookup), true
	}
	return nil, false
}

func battleSkillExpRewardsFromCastsByUnit(
	finalResult map[string]any,
	castsByUnit map[string]any,
	maxLevelLookup func(string) int,
) map[int64]map[string]int64 {
	players := decodeBattlePlayersFromFinalResult(finalResult)
	if len(players) == 0 {
		return nil
	}
	unitToActor, skillLevelByUnitID := battleSkillExpActorIndexes(players)
	rewards := map[int64]map[string]int64{}
	for unitID, rawPerSkill := range castsByUnit {
		perSkill := normalizeAnyMap(rawPerSkill)
		if len(perSkill) == 0 {
			continue
		}
		for skillID, rawCount := range perSkill {
			count := toInt64Loose(rawCount, 0)
			if count <= 0 {
				continue
			}
			battleAccumulateSkillExpReward(rewards, unitToActor, skillLevelByUnitID, unitID, skillID, maxLevelLookup, count*store.BattleSkillExpPerUse)
		}
	}
	return rewards
}

type battleSkillExpActorRef struct {
	UserID    int64
	ActorType string
	ActorID   int64
}

func battleSkillExpActorIndexes(players []battleUnitState) (map[string]battleSkillExpActorRef, map[string]map[string]int) {
	unitToActor := make(map[string]battleSkillExpActorRef, len(players))
	skillLevelByUnitID := make(map[string]map[string]int, len(players))
	for _, player := range players {
		if player.UserID <= 0 {
			continue
		}
		unitID := strings.TrimSpace(player.UnitID)
		if unitID == "" {
			continue
		}
		ref := battleSkillExpActorRef{UserID: player.UserID, ActorType: "player", ActorID: 0}
		if player.UnitType == "kitty" && player.KittyID > 0 {
			ref.ActorType = "kitty"
			ref.ActorID = player.KittyID
		}
		unitToActor[unitID] = ref
		skillLevelMap := make(map[string]int, len(player.Skills))
		for _, skill := range player.Skills {
			skillID := strings.TrimSpace(skill.ID)
			if skillID == "" {
				continue
			}
			skillLevelMap[strings.ToLower(skillID)] = maxInt(1, skill.Level)
		}
		skillLevelByUnitID[unitID] = skillLevelMap
	}
	return unitToActor, skillLevelByUnitID
}

func battleAccumulateSkillExpReward(
	rewards map[int64]map[string]int64,
	unitToActor map[string]battleSkillExpActorRef,
	skillLevelByUnitID map[string]map[string]int,
	actorID string,
	skillID string,
	maxLevelLookup func(string) int,
	gain int64,
) {
	actorID = strings.TrimSpace(actorID)
	skillID = strings.TrimSpace(skillID)
	if actorID == "" || skillID == "" || gain <= 0 {
		return
	}
	if strings.EqualFold(skillID, "basic_attack") {
		return
	}
	actor, ok := unitToActor[actorID]
	if !ok || actor.UserID <= 0 {
		return
	}
	levelMap := skillLevelByUnitID[actorID]
	level := levelMap[strings.ToLower(skillID)]
	if level <= 0 {
		return
	}
	maxLevel := 200
	if maxLevelLookup != nil {
		maxLevel = maxInt(1, maxLevelLookup(skillID))
	}
	if level >= maxLevel {
		return
	}
	rewardKey := actor.UserID
	if actor.ActorType == "kitty" && actor.ActorID > 0 {
		rewardKey = -actor.ActorID
	}
	if _, ok := rewards[rewardKey]; !ok {
		rewards[rewardKey] = map[string]int64{}
	}
	rewards[rewardKey][skillID] += gain
}

func battleSkillCastFromTimelineEvent(eventType string, data map[string]any) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "battle.turn.action":
		actorID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(data["actorId"], data["actorID"], data["u"])))
		skillID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(data["skillId"], data["i"])))
		if actorID == "" || skillID == "" {
			return "", "", false
		}
		return actorID, skillID, true
	case "battle.timeline.node":
		return battleSkillCastFromTimelineNodeData(data)
	default:
		return "", "", false
	}
}

func battleSkillCastFromTimelineNodeData(data map[string]any) (string, string, bool) {
	nodeActorID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(data["o"], data["actorId"], data["actorID"], data["u"])))
	steps := battleAnyMapSlice(firstNonNil(data["p"], data["steps"]))
	if len(steps) == 0 {
		return "", "", false
	}
	for _, step := range steps {
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["k"], step["kind"]))))
		if kind != "c" && kind != "cast" {
			continue
		}
		phase := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["ph"], step["phase"]))))
		if phase == "i" || phase == "item" || phase == "r" || phase == "regen" {
			continue
		}
		actorID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["s"], step["sourceId"], step["actorId"], nodeActorID)))
		skillID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["i"], step["skillId"])))
		if actorID == "" || skillID == "" {
			continue
		}
		// battle.timeline.node may carry hook casts; keep only the node actor's primary cast.
		if nodeActorID != "" && !strings.EqualFold(actorID, nodeActorID) {
			continue
		}
		return actorID, skillID, true
	}
	return "", "", false
}

func (s *Server) battleSkillMaxLevelByID(skillID string) int {
	raw, ok := s.battleSkillRawByID(skillID)
	if !ok {
		return 200
	}
	return maxInt(1, int(toInt64Loose(firstNonNil(raw["maxLevel"], raw["max_level"]), 200)))
}

func battleTimelineEventPayload(event wsbus.BattleTimelineEvent) (string, map[string]any) {
	if len(event.Payload) == 0 {
		return "", nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return "", nil
	}
	eventType, _ := payload["eventType"].(string)
	data, _ := payload["data"].(map[string]any)
	return strings.TrimSpace(eventType), data
}
