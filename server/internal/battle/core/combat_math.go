package core

import (
	"math"
)

type DamageInput struct {
	Physical         bool
	Attack           float64
	Defense          float64
	DefenseK         float64
	Strength         float64
	Dexterity        float64
	Coeff            float64
	CritChance       float64
	CritDamage       float64
	CritDamageResist float64
	Rand             float64
}

func ComputeDamage(in DamageInput) (float64, bool) {
	attack := in.Attack
	if attack <= 0 {
		attack = maxFloat(1, in.Strength*0.8+in.Dexterity*0.2)
	}
	coeff := in.Coeff
	if coeff <= 0 {
		coeff = 1
	}
	damage := attack * coeff
	if in.Defense != 0 {
		defenseK := in.DefenseK
		if defenseK <= 0 || math.IsNaN(defenseK) || math.IsInf(defenseK, 0) {
			defenseK = 100
		}
		// Defense > 0: mitigation. Defense < 0: amplification (overflow penetration).
		// Cap at -defenseK/2 so max amplification = 2x (symmetric: K defense = 0.5x).
		defense := maxFloat(in.Defense, -defenseK/2)
		damage = damage * defenseK / (defenseK + defense)
	}
	if damage < 1 {
		damage = 1
	}
	critChance := in.CritChance
	// 暴击率有可能大于1，很正常
	// if critChance > 1 {
	// 	critChance = critChance / 100
	// }
	critChance = clampFloat(critChance, 0, 1)
	crit := in.Rand < critChance
	if crit {
		critMul := in.CritDamage
		if critMul <= 0 {
			critMul = 0.5
		}
		critResist := clampFloat(in.CritDamageResist, 0, 1)
		damage = damage * (1 + critMul*(1-critResist))
	}
	return roundTo4(damage), crit
}

func roundTo4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
