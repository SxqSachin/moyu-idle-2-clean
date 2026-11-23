package lifeaction

import (
	"encoding/json"
	"errors"
	"math"
	"mysecondwebgame/server/internal/operationeffect"
)

const (
	KindHarvestBonus   = "life_action.harvest_bonus"
	KindFocusedHarvest = "life_action.focused_harvest"
	KindPatientHarvest = "life_action.patient_harvest"
	KindWorkExperience = "life_action.work_experience"
)

type WorkMealParams struct {
	Chance                float64 `json:"chance,omitempty"`
	DurationPenaltyRate   float64 `json:"durationPenaltyRate,omitempty"`
	DurationReductionRate float64 `json:"durationReductionRate,omitempty"`
	ExperienceBonusRate   float64 `json:"experienceBonusRate,omitempty"`
}

func IsWorkMeal(kind string) bool {
	return kind == KindHarvestBonus || kind == KindFocusedHarvest || kind == KindPatientHarvest || kind == KindWorkExperience
}

func IsPrimaryWork(mainType string) bool {
	return mainType == "collecting" || mainType == "mining" || mainType == "farming_animal"
}

func decodeWorkMeal(kind string, raw json.RawMessage) (WorkMealParams, error) {
	var p WorkMealParams
	if err := decodeStrict(raw, &p); err != nil {
		return p, err
	}
	for _, value := range []float64{p.Chance, p.DurationPenaltyRate, p.DurationReductionRate, p.ExperienceBonusRate} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return p, errors.New("invalid work meal value")
		}
	}
	valid := false
	switch kind {
	case KindHarvestBonus:
		valid = p.Chance > 0 && p.Chance <= .05 && p.DurationReductionRate <= .05 && p.DurationPenaltyRate == 0 && p.ExperienceBonusRate == 0
	case KindFocusedHarvest:
		valid = p.Chance > 0 && p.Chance <= .03 && p.DurationReductionRate > 0 && p.DurationReductionRate <= .08 && p.DurationPenaltyRate == 0 && p.ExperienceBonusRate == 0
	case KindPatientHarvest:
		valid = p.Chance >= .08 && p.Chance <= .11 && p.DurationPenaltyRate == .04 && p.DurationReductionRate == 0 && p.ExperienceBonusRate == 0
	case KindWorkExperience:
		valid = p.ExperienceBonusRate > 0 && p.ExperienceBonusRate <= .15 && p.Chance == 0 && p.DurationPenaltyRate == 0 && p.DurationReductionRate == 0
	}
	if !valid {
		return p, errors.New("work meal parameters exceed mode bounds")
	}
	return p, nil
}

func (h handler) ValidateDefinitionConstraints(d operationeffect.Definition) error {
	if !IsWorkMeal(h.kind) {
		return nil
	}
	if len(d.Selector.MainTypes) == 0 {
		return errors.New("work meal requires explicit primary work scope")
	}
	for _, mainType := range d.Selector.MainTypes {
		if !IsPrimaryWork(mainType) {
			return errors.New("work meal cannot affect production actions")
		}
	}
	if !d.Stacking.Exclusive || d.Stacking.Key != "food.primary_work" {
		return errors.New("work meals must be mutually exclusive")
	}
	return nil
}

// Select one whole meal, never combine a benefit with another meal's penalty.
func WorkMealModifiers(snapshot operationeffect.Snapshot) (WorkMealParams, error) {
	return (*Resolver)(nil).WorkMealModifiers(snapshot)
}

func (r *Resolver) WorkMealModifiers(snapshot operationeffect.Snapshot) (WorkMealParams, error) {
	var result WorkMealParams
	found := false
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainLifeAction || !IsWorkMeal(rule.Kind) {
			continue
		}
		if found {
			return result, errors.New("multiple mutually exclusive work meals")
		}
		var err error
		result, err = r.meal(rule.Kind, rule.Params)
		if err != nil {
			return result, err
		}
		found = true
	}
	return result, nil
}
