package core

import "testing"

func TestComputeDamageDefenseUsesDiminishingReduction(t *testing.T) {
	damage, crit := ComputeDamage(DamageInput{
		Attack:     50,
		Defense:    20,
		DefenseK:   100,
		Coeff:      1,
		CritChance: 0,
		Rand:       0.99,
	})
	if crit {
		t.Fatalf("crit=%v, want false", crit)
	}
	if damage != 41.6667 {
		t.Fatalf("damage=%v, want 41.6667", damage)
	}
}

func TestComputeDamageDefenseReductionIsMonotonic(t *testing.T) {
	lowDefense, _ := ComputeDamage(DamageInput{Attack: 50, Defense: 20, DefenseK: 100, CritChance: 0, Rand: 0.99})
	highDefense, _ := ComputeDamage(DamageInput{Attack: 50, Defense: 80, DefenseK: 100, CritChance: 0, Rand: 0.99})
	if highDefense >= lowDefense {
		t.Fatalf("highDefense=%v should be lower than lowDefense=%v", highDefense, lowDefense)
	}
	if highDefense <= 0 {
		t.Fatalf("highDefense=%v should stay positive", highDefense)
	}
}

func TestComputeDamageCritAppliesAfterDefenseReduction(t *testing.T) {
	damage, crit := ComputeDamage(DamageInput{
		Attack:     50,
		Defense:    20,
		DefenseK:   100,
		Coeff:      1,
		CritChance: 1,
		CritDamage: 0.5,
		Rand:       0,
	})
	if !crit {
		t.Fatalf("crit=%v, want true", crit)
	}
	if damage != 62.5 {
		t.Fatalf("damage=%v, want 62.5", damage)
	}
}

func TestComputeDamageCritDamageResistOnlyReducesBonusCritDamage(t *testing.T) {
	damage, crit := ComputeDamage(DamageInput{
		Attack:           50,
		Defense:          20,
		DefenseK:         100,
		Coeff:            1,
		CritChance:       1,
		CritDamage:       0.5,
		CritDamageResist: 0.5,
		Rand:             0,
	})
	if !crit {
		t.Fatalf("crit=%v, want true", crit)
	}
	if damage != 52.0833 {
		t.Fatalf("damage=%v, want 52.0833", damage)
	}
}

func TestComputeDamageHighDefenseStillKeepsMinimumOneDamage(t *testing.T) {
	damage, _ := ComputeDamage(DamageInput{
		Attack:     10,
		Defense:    100000,
		DefenseK:   100,
		Coeff:      1,
		CritChance: 0,
		Rand:       0.99,
	})
	if damage != 1 {
		t.Fatalf("damage=%v, want 1", damage)
	}
}

// Negative effective defense (flat pen > defense with no percent pen) → amplification.
// At effectiveDefense = -50, K = 100: multiplier = 100/(100-50) = 2x.
func TestComputeDamageNegativeDefenseAmplifies(t *testing.T) {
	// effectiveDefense passed in as -50 (caller already resolved penetration)
	damage, _ := ComputeDamage(DamageInput{
		Attack:     50,
		Defense:    -50,
		DefenseK:   100,
		Coeff:      1,
		CritChance: 0,
		Rand:       0.99,
	})
	// 50 * 100 / (100 - 50) = 50 * 2 = 100
	if damage != 100 {
		t.Fatalf("damage=%v, want 100", damage)
	}
}

// Effective defense below -K/2 is clamped to -K/2 (max 2x amplification).
func TestComputeDamageNegativeDefenseIsCappedAtHalfK(t *testing.T) {
	beyondCap, _ := ComputeDamage(DamageInput{
		Attack: 50, Defense: -200, DefenseK: 100, Coeff: 1, CritChance: 0, Rand: 0.99,
	})
	atCap, _ := ComputeDamage(DamageInput{
		Attack: 50, Defense: -50, DefenseK: 100, Coeff: 1, CritChance: 0, Rand: 0.99,
	})
	if beyondCap != atCap {
		t.Fatalf("beyondCap=%v should equal atCap=%v (both capped at 2x)", beyondCap, atCap)
	}
	if beyondCap != 100 {
		t.Fatalf("damage=%v, want 100 (2x amplification cap)", beyondCap)
	}
}
