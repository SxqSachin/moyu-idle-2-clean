package store

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestAmusementCatSwordsUseAuthoritativeEnhanceRuntime(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	gameDataPath := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "packages", "game-data", "generated", "game-data.json")
	gameData, err := catalog.Load(gameDataPath)
	if err != nil {
		t.Fatalf("catalog.Load(%q): %v", gameDataPath, err)
	}
	enhanceCache, err := buildEnhanceRuntimeCache(gameData)
	if err != nil {
		t.Fatalf("buildEnhanceRuntimeCache(): %v", err)
	}

	tests := []struct {
		id        string
		rarity    string
		materials []string
		want40    float64
		want50    float64
	}{
		{
			id:        "cat_sword",
			rarity:    "legendary",
			materials: []string{"candy_bomb", "plush_fur", "bottled_starlight", "condensed_stardust"},
			want40:    157.5,
			want50:    223.3,
		},
		{
			id:        "dark_cat_sword",
			rarity:    "mythic",
			materials: []string{"ghost_essence", "condensed_stardust", "base4_shadow_orb", "base4_shadow_ink"},
			want40:    237,
			want50:    336.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			template := gameData.EquipmentTemplateByID(tt.id)
			if template == nil {
				t.Fatalf("equipment template %q not found", tt.id)
			}
			enhanceRuntime, exists := enhanceCache.Template(tt.id)
			if !exists {
				t.Fatalf("enhance runtime %q not found", tt.id)
			}
			if enhanceRuntime.RarityID != tt.rarity {
				t.Fatalf("RarityID = %q, want %q", enhanceRuntime.RarityID, tt.rarity)
			}
			if enhanceRuntime.MaxLevel != 50 {
				t.Fatalf("MaxLevel = %d, want 50", enhanceRuntime.MaxLevel)
			}

			baseDamage := template.BaseAttrs["physics_damage"]
			assertEnhanceFloatClose(t, baseDamage+enhanceRuntime.CumulativeByLevel[40]["physics_damage"], tt.want40, "+40 physics_damage")
			assertEnhanceFloatClose(t, baseDamage+enhanceRuntime.CumulativeByLevel[50]["physics_damage"], tt.want50, "+50 physics_damage")

			assertEnhanceAttemptCosts(t, enhanceRuntime.NextPreviewByLevel[39], 40, tt.materials, 40)
			assertEnhanceAttemptCosts(t, enhanceRuntime.NextPreviewByLevel[49], 50, tt.materials, 50)

			maxed := enhanceRuntime.NextPreviewByLevel[50]
			if maxed.NextLevel != 51 || maxed.SuccessRate != 0 || len(maxed.Costs) != 0 || len(maxed.DeltaAttrs) != 0 {
				t.Fatalf("max-level preview = %#v, want empty +51 preview", maxed)
			}
		})
	}
}

func assertEnhanceAttemptCosts(t *testing.T, preview EnhanceAttemptPreview, wantLevel int, materials []string, wantCost int64) {
	t.Helper()
	if preview.NextLevel != wantLevel {
		t.Fatalf("NextLevel = %d, want %d", preview.NextLevel, wantLevel)
	}
	if len(preview.Costs) != len(materials) {
		t.Fatalf("cost material count = %d, want %d: %#v", len(preview.Costs), len(materials), preview.Costs)
	}
	for _, materialID := range materials {
		if got := preview.Costs[materialID]; got != wantCost {
			t.Fatalf("cost[%s] = %d, want %d", materialID, got, wantCost)
		}
	}
}

func assertEnhanceFloatClose(t *testing.T, got float64, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12g, want %.12g", label, got, want)
	}
}
