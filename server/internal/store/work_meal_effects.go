package store

import (
	"math"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
	"time"
)

func workMealDuration(base time.Duration, snapshot operationeffect.Snapshot) (time.Duration, error) {
	mods, err := lifeaction.WorkMealModifiers(snapshot)
	if err != nil || (mods.DurationPenaltyRate == 0 && mods.DurationReductionRate == 0) {
		return base, err
	}
	base = normalizeActionQueueRunDuration(base)
	return normalizeActionQueueRunDuration(time.Duration(math.Ceil(float64(base/time.Millisecond)*(1+mods.DurationPenaltyRate-mods.DurationReductionRate))) * time.Millisecond), nil
}

func workMealExperience(spec ActionSpec, snapshots []operationeffect.Snapshot) (map[string]float64, error) {
	bonus := map[string]float64{}
	if !lifeaction.IsPrimaryWork(spec.MainType) {
		return bonus, nil
	}
	base := spec.StatusExpReward[spec.MainType]
	if base <= 0 {
		return bonus, nil
	}
	resolver := lifeaction.Resolver{}
	for _, snapshot := range snapshots {
		mods, err := resolver.WorkMealModifiers(snapshot)
		if err != nil {
			return nil, err
		}
		bonus[spec.MainType] += base * mods.ExperienceBonusRate
	}
	return bonus, nil
}
