package store

import (
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

func TestBuildEnhanceTemplateRuntime(t *testing.T) {
	template := catalog.EquipmentTemplate{
		ID:                 "wood_sword",
		EnhanceMaxLevel:    3,
		EnhanceDurationS:   8,
		EnhanceExp:         1.25,
		EnhanceProtectItem: []string{"protect_scroll"},
		EnhanceAffixMilestonePools: map[string][]catalog.EnhanceAffixMilestonePoolEntry{
			"2": []catalog.EnhanceAffixMilestonePoolEntry{
				{
					AffixID:  "affix.bonus_hp",
					RarityID: "rare",
				},
			},
		},
		EnhanceCostCurve: catalog.EnhanceCostCurve{
			Base: map[string]int64{"stone": 2},
			Step: map[string]int64{"stone": 1},
			Overrides: map[string]map[string]int64{
				"2": {"stone": 7},
			},
		},
		EnhanceAttrCurve: catalog.EnhanceAttrCurve{
			Base: map[string]float64{"physics_damage": 1},
			Step: map[string]float64{"physics_damage": 1},
		},
	}
	runtime, err := buildEnhanceTemplateRuntime(template, "common")
	if err != nil {
		t.Fatalf("buildEnhanceTemplateRuntime() err = %v", err)
	}
	if runtime.RarityID != "common" {
		t.Fatalf("RarityID = %q, want common", runtime.RarityID)
	}
	if runtime.MaxLevel != 3 {
		t.Fatalf("MaxLevel = %d, want 3", runtime.MaxLevel)
	}
	if runtime.Duration != 8*time.Second {
		t.Fatalf("Duration = %v, want 8s", runtime.Duration)
	}
	if len(runtime.ProtectItemIDs) != 1 || runtime.ProtectItemIDs[0] != "protect_scroll" {
		t.Fatalf("ProtectItemIDs = %#v, want [protect_scroll]", runtime.ProtectItemIDs)
	}
	if runtime.EnhanceExp != 1.25 {
		t.Fatalf("EnhanceExp = %v, want 1.25", runtime.EnhanceExp)
	}
	if got := runtime.CumulativeByLevel[1]["physics_damage"]; got != 1 {
		t.Fatalf("CumulativeByLevel[1] physics_damage = %v, want 1", got)
	}
	if got := runtime.CumulativeByLevel[2]["physics_damage"]; got != 2 {
		t.Fatalf("CumulativeByLevel[2] physics_damage = %v, want 2", got)
	}
	if got := runtime.CumulativeByLevel[3]["physics_damage"]; got != 3 {
		t.Fatalf("CumulativeByLevel[3] physics_damage = %v, want 3", got)
	}
	pool, ok := runtime.AffixMilestonePools[2]
	if !ok || len(pool) != 1 || pool[0].AffixID != "affix.bonus_hp" || pool[0].RarityID != "rare" {
		t.Fatalf("AffixMilestonePools[2] = %#v, want one rare affix.bonus_hp", pool)
	}
	previewAt1 := runtime.NextPreviewByLevel[1]
	if previewAt1.NextLevel != 2 {
		t.Fatalf("previewAt1.NextLevel = %d, want 2", previewAt1.NextLevel)
	}
	if got := previewAt1.DeltaAttrs["physics_damage"]; got != 1 {
		t.Fatalf("previewAt1 physics_damage delta = %v, want 1", got)
	}
	if got := previewAt1.Costs["stone"]; got != 7 {
		t.Fatalf("previewAt1 stone cost = %d, want 7", got)
	}
	if want := resolveEnhanceSuccessRateByFormula("common", 2, 3); math.Abs(previewAt1.SuccessRate-want) > 1e-9 {
		t.Fatalf("previewAt1 successRate = %v, want %v", previewAt1.SuccessRate, want)
	}
	previewAt2 := runtime.NextPreviewByLevel[2]
	if want := resolveEnhanceSuccessRateByFormula("common", 3, 3); math.Abs(previewAt2.SuccessRate-want) > 1e-9 {
		t.Fatalf("previewAt2 successRate = %v, want %v", previewAt2.SuccessRate, want)
	}
}

func TestBuildEnhanceTemplateRuntimeRejectsInvalidOverrideKey(t *testing.T) {
	template := catalog.EquipmentTemplate{
		ID:               "wood_sword",
		EnhanceMaxLevel:  2,
		EnhanceDurationS: 5,
		EnhanceCostCurve: catalog.EnhanceCostCurve{
			Base: map[string]int64{"stone": 1},
			Overrides: map[string]map[string]int64{
				"x": {"stone": 2},
			},
		},
		EnhanceAttrCurve: catalog.EnhanceAttrCurve{
			Base: map[string]float64{"physics_damage": 1},
		},
	}
	_, err := buildEnhanceTemplateRuntime(template, "common")
	if err == nil {
		t.Fatalf("expected error for invalid override key")
	}
}

func TestBuildEnhanceTemplateRuntimeOverrideReplacesSingleLevelIncrement(t *testing.T) {
	template := catalog.EquipmentTemplate{
		ID:               "wood_sword",
		EnhanceMaxLevel:  10,
		EnhanceDurationS: 5,
		EnhanceAttrCurve: catalog.EnhanceAttrCurve{
			Base: map[string]float64{"hp": 1},
			Step: map[string]float64{"hp": 1},
			Overrides: map[string]map[string]float64{
				"5": {"hp": 10},
			},
		},
	}
	runtime, err := buildEnhanceTemplateRuntime(template, "common")
	if err != nil {
		t.Fatalf("buildEnhanceTemplateRuntime() err = %v", err)
	}
	if got := runtime.CumulativeByLevel[4]["hp"]; got != 4 {
		t.Fatalf("CumulativeByLevel[4] hp = %v, want 4", got)
	}
	if got := runtime.CumulativeByLevel[5]["hp"]; got != 14 {
		t.Fatalf("CumulativeByLevel[5] hp = %v, want 14", got)
	}
	if got := runtime.CumulativeByLevel[10]["hp"]; got != 19 {
		t.Fatalf("CumulativeByLevel[10] hp = %v, want 19", got)
	}
	if got := runtime.NextPreviewByLevel[4].DeltaAttrs["hp"]; got != 10 {
		t.Fatalf("NextPreviewByLevel[4] hp delta = %v, want 10", got)
	}
	if got := runtime.NextPreviewByLevel[5].DeltaAttrs["hp"]; got != 1 {
		t.Fatalf("NextPreviewByLevel[5] hp delta = %v, want 1", got)
	}
}

func TestBuildEnhanceTemplateRuntimeUsesSuccessRateOverride(t *testing.T) {
	successRate := 0.4
	template := catalog.EquipmentTemplate{
		ID:                         "cute_hat",
		EnhanceMaxLevel:            3,
		EnhanceDurationS:           5,
		EnhanceSuccessRateOverride: &successRate,
		EnhanceCostCurve: catalog.EnhanceCostCurve{
			Base: map[string]int64{"kitty_affection": 4},
		},
		EnhanceAttrCurve: catalog.EnhanceAttrCurve{
			Base: map[string]float64{"luck": 0.1},
		},
	}
	runtime, err := buildEnhanceTemplateRuntime(template, "mythic")
	if err != nil {
		t.Fatalf("buildEnhanceTemplateRuntime() err = %v", err)
	}
	for level := 0; level < runtime.MaxLevel; level++ {
		preview := runtime.NextPreviewByLevel[level]
		if math.Abs(preview.SuccessRate-0.4) > 1e-9 {
			t.Fatalf("NextPreviewByLevel[%d].SuccessRate = %v, want 0.4", level, preview.SuccessRate)
		}
	}
}
