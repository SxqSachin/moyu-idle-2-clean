package store

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestCursedSilkSachetUsesAuthoritativeEnhancementCurve(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	gameDataPath := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "packages", "game-data", "generated", "game-data.json")
	gameData, err := catalog.Load(gameDataPath)
	if err != nil {
		t.Fatalf("catalog.Load(%q): %v", gameDataPath, err)
	}
	cache, err := buildEnhanceRuntimeCache(gameData)
	if err != nil {
		t.Fatalf("buildEnhanceRuntimeCache(): %v", err)
	}

	template := gameData.EquipmentTemplateByID("cursed_silk_sachet")
	if template == nil {
		t.Fatal("cursed_silk_sachet equipment template not found")
	}
	for _, tc := range []struct {
		level int
		want  float64
	}{{0, 0.08}, {10, 0.10}, {20, 0.12}, {40, 0.16}} {
		total := template.BaseAttrs["battle_drop_rate_bonus"] + cache.CumulativeAttrs(template.ID, tc.level)["battle_drop_rate_bonus"]
		if math.Abs(total-tc.want) > 1e-9 {
			t.Fatalf("+%d battle_drop_rate_bonus=%v, want %v", tc.level, total, tc.want)
		}
	}

	level20, ok := cache.NextPreview(template.ID, 19)
	if !ok || level20.Costs["base4_abyssal_core"] != 1 || level20.Costs["base4_ancestor_shadow_jade"] != 1 {
		t.Fatalf("+20 Boss-material milestone mismatch: %#v", level20)
	}
	level40, ok := cache.NextPreview(template.ID, 39)
	if !ok || level40.Costs["final_volume_key_sigil"] != 1 || level40.Costs["shadow_paw_master_sigil"] != 1 {
		t.Fatalf("+40 Boss-material milestone mismatch: %#v", level40)
	}
}
