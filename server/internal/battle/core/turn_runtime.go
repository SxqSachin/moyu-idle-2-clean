package core

import (
	"sort"
	"strings"
)

type SkillState struct {
	ID                 string
	HPCost             float64
	MPCost             float64
	CooldownTurns      int
	CooldownRemain     int
	DamageCoeff        float64
	DamageFormula      []AttackTerm
	DamageType         string
	TargetDirection    string
	TargetSeekStrategy string
	TargetMode         string
	TargetBaseCount    int
	TargetFormula      []AttackTerm
	TargetCount        int
}

type AttackTerm struct {
	AttrKey       string
	Coefficient   float64
	PerSkillLevel float64
}

type UnitState struct {
	UnitID   string
	Position string
	Alive    bool
	HP       float64
	MP       float64
	MaxHP    float64
	MaxMP    float64
	Stats    map[string]float64
	Skills   []SkillState
}

func SideAlive(rows []UnitState) bool {
	for _, row := range rows {
		if row.Alive && row.HP > 0 {
			return true
		}
	}
	return false
}

func ApplyTurnRegen(unit *UnitState) {
	if unit == nil {
		return
	}
	if !unit.Alive || unit.HP <= 0 {
		unit.HP = 0
		unit.Alive = false
		return
	}
	hpRegen := maxFloat(0, unit.Stats["hp_regen"])
	mpRegen := maxFloat(0, unit.Stats["mp_regen"])
	unit.HP = clampFloat(unit.HP+hpRegen, 0, unit.MaxHP)
	unit.MP = clampFloat(unit.MP+mpRegen, 0, unit.MaxMP)
}

func DecrementSkillCooldowns(unit *UnitState) {
	if unit == nil {
		return
	}
	for skillIdx := range unit.Skills {
		if unit.Skills[skillIdx].CooldownRemain > 0 {
			unit.Skills[skillIdx].CooldownRemain--
		}
	}
}

func SelectSkill(unit UnitState, passiveChecker func(skillID string) bool) (SkillState, int) {
	for idx, skill := range unit.Skills {
		if !unit.Alive {
			break
		}
		if passiveChecker != nil && passiveChecker(skill.ID) {
			continue
		}
		if skill.CooldownRemain > 0 {
			continue
		}
		if unit.MP < skill.MPCost {
			continue
		}
		if unit.HP <= skill.HPCost {
			continue
		}
		return skill, idx
	}
	return SkillState{
		ID:                 "basic_attack",
		HPCost:             0,
		MPCost:             0,
		CooldownTurns:      1,
		CooldownRemain:     0,
		DamageCoeff:        1,
		DamageType:         "physical",
		TargetDirection:    "vanguard",
		TargetSeekStrategy: "front_to_rear",
		TargetMode:         "single",
		TargetBaseCount:    1,
		TargetCount:        1,
	}, -1
}

func SelectTargetIndexes(units []UnitState, skill SkillState) []int {
	if strings.ToLower(skill.TargetMode) == "all" || strings.ToLower(skill.TargetDirection) == "all" {
		indexes := make([]int, 0, len(units))
		for idx := range units {
			if !units[idx].Alive || units[idx].HP <= 0 {
				continue
			}
			indexes = append(indexes, idx)
		}
		return indexes
	}
	direction := strings.ToLower(strings.TrimSpace(skill.TargetDirection))
	if direction == "" {
		direction = "vanguard"
	}
	order := buildTargetRowOrder(direction, skill.TargetSeekStrategy)
	collected := make([]int, 0, len(units))
	targetCount := skill.TargetCount
	if targetCount <= 0 {
		targetCount = skill.TargetBaseCount
	}
	if targetCount <= 0 {
		targetCount = 1
	}
	chosen := make([]int, 0, len(units))
	for _, pos := range order {
		chosen = chosen[:0]
		for idx := range units {
			unit := units[idx]
			if !unit.Alive || unit.HP <= 0 {
				continue
			}
			if strings.ToLower(strings.TrimSpace(unit.Position)) != pos {
				continue
			}
			chosen = append(chosen, idx)
		}
		if len(chosen) == 0 {
			continue
		}
		if strings.ToLower(skill.TargetMode) == "multi" {
			return append([]int(nil), chosen...)
		}
		sort.SliceStable(chosen, func(i, j int) bool {
			left := units[chosen[i]].HP
			right := units[chosen[j]].HP
			if left == right {
				return units[chosen[i]].UnitID < units[chosen[j]].UnitID
			}
			return left < right
		})
		for _, idx := range chosen {
			collected = append(collected, idx)
			if len(collected) >= targetCount {
				return collected
			}
		}
	}
	return collected
}

func buildTargetRowOrder(direction string, seekStrategy string) []string {
	base := []string{"vanguard", "midguard", "rearguard"}
	if strings.EqualFold(strings.TrimSpace(seekStrategy), "rear_to_front") {
		base = []string{"rearguard", "midguard", "vanguard"}
	}
	idx := -1
	for i, row := range base {
		if row == direction {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return base
	}
	return append(append([]string{}, base[idx:]...), base[:idx]...)
}
