package store

import (
	"encoding/json"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestEquipmentSkillCapacityAndValidation(t *testing.T) {
	for _, base := range []int{1, 2} {
		grant := catalog.EquipmentGrantedSkill{BaseCapacity: base, CapacityMilestones: []int{10, 20}}
		for _, tc := range []struct{ level, bonus int }{{0, 0}, {9, 0}, {10, 1}, {19, 1}, {20, 2}, {25, 2}} {
			if got := grant.Capacity(tc.level); got != base+tc.bonus {
				t.Fatalf("capacity +%d: %d", tc.level, got)
			}
		}
	}
	valid := EquipmentSkillConfig{Version: 1, Enabled: true, SkillIDs: []string{"a", "b"}}
	if err := ValidateEquipmentSkillConfig(EquipmentMartialComboSkill, valid); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]string{{"a"}, {"a", "a"}, {"a", "basic_attack"}, {"a", EquipmentMartialComboSkill}, {"a", " b"}, {"a", "b", "c", "d", "e"}} {
		cfg := valid
		cfg.SkillIDs = ids
		if ValidateEquipmentSkillConfig(EquipmentMartialComboSkill, cfg) == nil {
			t.Fatalf("accepted invalid selection: %v", ids)
		}
	}
	valid.AllowStandalone = true
	if ValidateEquipmentSkillConfig(EquipmentDominationSkill, valid) == nil {
		t.Fatal("domination accepted combo settings")
	}
	if ValidateEquipmentSkillConfig("forged_skill", valid) == nil {
		t.Fatal("arbitrary skill accepted")
	}
}

func TestEquipmentSkillEligibility(t *testing.T) {
	for _, tc := range []struct {
		raw               string
		combo, domination bool
	}{
		{`{"id":"strike","effects":[{"kind":"damage","damageSpec":{"damageType":"physical"}}]}`, true, true},
		{`{"id":"hybrid","effects":[{"kind":"damage","damageType":"physical"},{"kind":"damage","damageType":"fire"}]}`, false, true},
		{`{"id":"heal","effects":[{"kind":"heal"}]}`, false, true},
		{`{"id":"boss","enemyOnly":true,"effects":[{"kind":"damage","damageType":"physical"}]}`, false, true},
		{`{"id":"protected","uninterruptible":true}`, false, false},
		{`{"id":"custom:private","effects":[{"kind":"damage","damageType":"physical"}]}`, true, false},
		{`{"id":"equipment_domination","equipmentOnly":true}`, false, false},
		{`{"id":"basic_attack","effects":[{"kind":"damage","damageType":"physical"}]}`, false, false},
	} {
		var raw map[string]any
		if err := json.Unmarshal([]byte(tc.raw), &raw); err != nil {
			t.Fatal(err)
		}
		if EquipmentComboEligible(raw) != tc.combo || EquipmentDominationEligible(raw) != tc.domination {
			t.Fatalf("eligibility: %s", tc.raw)
		}
	}
}

func TestEquipmentSkillLoadoutPreservesExplicitEmptyAndLegacy(t *testing.T) {
	for _, configs := range []map[string]EquipmentSkillConfig{nil, {}, {EquipmentMartialComboSkill: {Version: 1, Enabled: true, SkillIDs: []string{"a", "b"}}}} {
		raw, err := marshalLoadoutPayload("battle", BattleLoadout{EquipmentSkills: configs}, WorkLoadout{})
		if err != nil {
			t.Fatal(err)
		}
		got := decodeBattleLoadout("test", "test", raw)
		if (got.EquipmentSkills == nil) != (configs == nil) || len(got.EquipmentSkills) != len(configs) {
			t.Fatalf("loadout lost empty/legacy distinction: %s", raw)
		}
	}
}
