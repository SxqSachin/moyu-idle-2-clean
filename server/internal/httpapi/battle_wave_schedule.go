package httpapi

import "sort"

func matchingBattleWaveSchedule(wave int, schedules []battleWaveSchedule) (battleWaveSchedule, bool) {
	if wave <= 0 || len(schedules) == 0 {
		return battleWaveSchedule{}, false
	}
	matched := make([]battleWaveSchedule, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule.Every > 0 && wave%schedule.Every == 0 && len(schedule.EnemyChoices) > 0 {
			matched = append(matched, schedule)
		}
	}
	if len(matched) == 0 {
		return battleWaveSchedule{}, false
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].Priority != matched[j].Priority {
			return matched[i].Priority > matched[j].Priority
		}
		return matched[i].ID < matched[j].ID
	})
	return matched[0], true
}

func (s *Server) resolveWaveScheduleEnemyIDs(state *battleRuntimeState, worldCfg battleWorldConfig) []string {
	if state == nil {
		return nil
	}
	schedule, ok := matchingBattleWaveSchedule(state.Wave, worldCfg.WaveSchedules)
	if !ok {
		return nil
	}
	minCount := maxInt(1, schedule.MinCount)
	maxCount := maxInt(minCount, schedule.MaxCount)
	count := minCount
	if maxCount > minCount {
		count += state.randInt(maxCount - minCount + 1)
	}
	ids := make([]string, 0, count)
	for index := 0; index < count; index++ {
		choice, selected := chooseBattleWaveScheduleEnemy(state, schedule.EnemyChoices)
		if selected {
			ids = append(ids, choice.EnemyID)
		}
	}
	return ids
}

func chooseBattleWaveScheduleEnemy(state *battleRuntimeState, choices []battleWaveScheduleEnemyChoice) (battleWaveScheduleEnemyChoice, bool) {
	if state == nil || len(choices) == 0 {
		return battleWaveScheduleEnemyChoice{}, false
	}
	if len(choices) == 1 {
		return choices[0], true
	}
	total := 0.0
	for _, choice := range choices {
		if choice.Weight > 0 {
			total += choice.Weight
		}
	}
	if total <= 0 {
		return battleWaveScheduleEnemyChoice{}, false
	}
	roll := state.randFloat64() * total
	cumulative := 0.0
	for _, choice := range choices {
		if choice.Weight <= 0 {
			continue
		}
		cumulative += choice.Weight
		if roll < cumulative {
			return choice, true
		}
	}
	return choices[len(choices)-1], true
}
