package httpapi

import (
	"math"
	"testing"

	battlecore "mysecondwebgame/server/internal/battle/core"
)

func TestApplyBattleDamageStatModifiersPhysical(t *testing.T) {
	source := &battleUnitState{Stats: map[string]float64{"physics_damage_amplification": 0.1}}
	target := &battleUnitState{Stats: map[string]float64{"physics_damage_resist": 0.5}}
	damage := applyBattleDamageStatModifiers(source, target, 50, "physical")
	if math.Abs(damage-27.5) > 0.0000001 {
		t.Fatalf("damage=%v, want 27.5", damage)
	}
}

func TestApplyBattleDamageStatModifiersMagical(t *testing.T) {
	source := &battleUnitState{Stats: map[string]float64{"magical_damage_amplification": 0.2}}
	target := &battleUnitState{Stats: map[string]float64{"magical_damage_resist": 0.2}}
	damage := applyBattleDamageStatModifiers(source, target, 20, "magical")
	if math.Abs(damage-19.2) > 0.0000001 {
		t.Fatalf("damage=%v, want 19.2", damage)
	}
}

func TestApplyBattleDamageStatModifiersCapsResistAt99Point9Percent(t *testing.T) {
	tests := []struct {
		name       string
		damageType string
		statKey    string
		resist     float64
	}{
		{name: "physical at 100 percent", damageType: "physical", statKey: "physics_damage_resist", resist: 1},
		{name: "magical above 100 percent", damageType: "magical", statKey: "magical_damage_resist", resist: 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &battleUnitState{Stats: map[string]float64{tt.statKey: tt.resist}}
			damage := applyBattleDamageStatModifiers(nil, target, 10000, tt.damageType)
			if math.Abs(damage-10) > 0.0000001 {
				t.Fatalf("damage=%v, want 10", damage)
			}
		})
	}
}

func TestApplyBattleDamageStatModifiersPreservesNegativeResist(t *testing.T) {
	target := &battleUnitState{Stats: map[string]float64{"physics_damage_resist": -0.2}}
	damage := applyBattleDamageStatModifiers(nil, target, 100, "physical")
	if math.Abs(damage-120) > 0.0000001 {
		t.Fatalf("damage=%v, want 120", damage)
	}
}

func TestDamagePipelineDefenseBeforeResist(t *testing.T) {
	baseDamage, _ := battlecore.ComputeDamage(battlecore.DamageInput{
		Attack:     50,
		Defense:    20,
		DefenseK:   100,
		Coeff:      1,
		CritChance: 0,
		Rand:       0.99,
	})
	source := &battleUnitState{Stats: map[string]float64{"physics_damage_amplification": 0}}
	target := &battleUnitState{Stats: map[string]float64{"physics_damage_resist": 0.1}}
	finalDamage := applyBattleDamageStatModifiers(source, target, baseDamage, "physical")
	if math.Abs(finalDamage-37.5) > 0.0001 {
		t.Fatalf("finalDamage=%v, want 37.5", finalDamage)
	}
}

func TestApplyBattleDamageStatModifiersResistMonotonicBySameType(t *testing.T) {
	source := &battleUnitState{Stats: map[string]float64{"physics_damage_amplification": 0}}
	targetLowResist := &battleUnitState{Stats: map[string]float64{"physics_damage_resist": 0.1}}
	targetHighResist := &battleUnitState{Stats: map[string]float64{"physics_damage_resist": 0.4}}
	lowResistDamage := applyBattleDamageStatModifiers(source, targetLowResist, 100, "physical")
	highResistDamage := applyBattleDamageStatModifiers(source, targetHighResist, 100, "physical")
	if highResistDamage > lowResistDamage {
		t.Fatalf("higher physical resist should not increase physical damage: low=%v high=%v", lowResistDamage, highResistDamage)
	}
}

func TestApplyBattleDamageStatModifiersOnlyUsesMatchingResistType(t *testing.T) {
	source := &battleUnitState{Stats: map[string]float64{}}
	targetPhysicalResist := &battleUnitState{Stats: map[string]float64{"physics_damage_resist": 0.5}}
	damageWithPhysicalResist := applyBattleDamageStatModifiers(source, targetPhysicalResist, 100, "magical")
	if damageWithPhysicalResist != 100 {
		t.Fatalf("physical resist should not reduce magical damage: got=%v want=100", damageWithPhysicalResist)
	}
	targetMagicalResist := &battleUnitState{Stats: map[string]float64{"magical_damage_resist": 0.5}}
	damageWithMagicalResist := applyBattleDamageStatModifiers(source, targetMagicalResist, 100, "magical")
	if damageWithMagicalResist != 50 {
		t.Fatalf("magical resist should reduce magical damage: got=%v want=50", damageWithMagicalResist)
	}
}
