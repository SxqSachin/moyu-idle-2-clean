package httpapi

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestDecodeFiberMapSlicePreservesWireSemanticsForSupportedShapes(t *testing.T) {
	tests := []struct {
		name string
		wrap func(fiber.Map) any
	}{
		{
			name: "map slice",
			wrap: func(row fiber.Map) any { return []map[string]any{map[string]any(row)} },
		},
		{
			name: "fiber map slice",
			wrap: func(row fiber.Map) any { return []fiber.Map{row} },
		},
		{
			name: "any slice",
			wrap: func(row fiber.Map) any { return []any{row} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.wrap(playbackCloneFixture())
			got := decodeFiberMapSlice(raw)
			if len(got) != 1 {
				t.Fatalf("decoded rows=%d, want 1", len(got))
			}

			legacyRaw, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("marshal legacy fixture: %v", err)
			}
			legacy := []fiber.Map{}
			if err := json.Unmarshal(legacyRaw, &legacy); err != nil {
				t.Fatalf("decode legacy fixture: %v", err)
			}
			gotRaw, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal cloned fixture: %v", err)
			}
			if string(gotRaw) != string(legacyRaw) {
				t.Fatalf("wire payload changed:\n got: %s\nwant: %s", gotRaw, legacyRaw)
			}
		})
	}
}

func TestDecodeFiberMapSliceDeepClonesMutableValues(t *testing.T) {
	source := playbackCloneFixture()
	got := decodeFiberMapSlice([]fiber.Map{source})
	if len(got) != 1 {
		t.Fatalf("decoded rows=%d, want 1", len(got))
	}

	got[0]["nested"].(map[string]any)["label"] = "changed"
	got[0]["skills"].([]fiber.Map)[0]["id"] = "changed"
	got[0]["stats"].(map[string]float64)["hp"] = 1
	gotEffect := got[0]["effects"].([]battleSkillEffect)
	gotEffect[0].WeightedStatusChoices[0].Payload["stacks"] = int64(9)

	if source["nested"].(map[string]any)["label"] != "original" {
		t.Fatal("nested map mutation leaked into source")
	}
	if source["skills"].([]fiber.Map)[0]["id"] != "skill:1" {
		t.Fatal("nested fiber map mutation leaked into source")
	}
	if source["stats"].(map[string]float64)["hp"] != 100 {
		t.Fatal("typed map mutation leaked into source")
	}
	sourceEffect := source["effects"].([]battleSkillEffect)
	if sourceEffect[0].WeightedStatusChoices[0].Payload["stacks"] != int64(2) {
		t.Fatal("nested struct payload mutation leaked into source")
	}
}

func TestCloneBattlePlaybackFiberMapFastPathMatchesLegacyReflection(t *testing.T) {
	source := playbackCloneFixture()
	source["stringSlice"] = []string{"a", "b"}
	source["intSlice"] = []int{1, 2}
	source["int64Slice"] = []int64{3, 4}
	source["intMap"] = map[string]int{"a": 1}
	source["int64Map"] = map[string]int64{"b": 2}
	source["stringMap"] = map[string]string{"c": "value"}

	got, gotOK := cloneBattlePlaybackFiberMap(source, 0)
	want, wantOK := cloneBattlePlaybackFiberMapReflect(source, 0)
	if gotOK != wantOK || !gotOK {
		t.Fatalf("clone success fast=%v legacy=%v", gotOK, wantOK)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fast clone differs from legacy reflection:\n got: %#v\nwant: %#v", got, want)
	}
	gotRaw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal fast clone: %v", err)
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal legacy clone: %v", err)
	}
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("fast clone wire payload differs:\n got: %s\nwant: %s", gotRaw, wantRaw)
	}

	got["stringSlice"].([]string)[0] = "changed"
	got["intMap"].(map[string]int)["a"] = 9
	if source["stringSlice"].([]string)[0] != "a" || source["intMap"].(map[string]int)["a"] != 1 {
		t.Fatal("fast scalar clone mutation leaked into source")
	}
}

func TestDecodeFiberMapSlicePreservesConcreteNilAndEmptyShapes(t *testing.T) {
	source := fiber.Map{
		"nilMap":           map[string]any(nil),
		"emptyMap":         map[string]any{},
		"nilFiberMap":      fiber.Map(nil),
		"emptyFiberMap":    fiber.Map{},
		"nilAnySlice":      []any(nil),
		"emptyAnySlice":    []any{},
		"nilMapSlice":      []map[string]any(nil),
		"emptyMapSlice":    []map[string]any{},
		"nilStringSlice":   []string(nil),
		"emptyStringSlice": []string{},
		"nilIntMap":        map[string]int(nil),
		"emptyIntMap":      map[string]int{},
	}

	got := decodeFiberMapSlice([]fiber.Map{source})
	if len(got) != 1 {
		t.Fatalf("decoded rows=%d, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0], source) {
		t.Fatalf("concrete nil/empty shapes changed:\n got: %#v\nwant: %#v", got[0], source)
	}
	if got[0]["nilMap"].(map[string]any) != nil || got[0]["emptyMap"].(map[string]any) == nil {
		t.Fatal("map nil/empty distinction changed")
	}
	if got[0]["nilFiberMap"].(fiber.Map) != nil || got[0]["emptyFiberMap"].(fiber.Map) == nil {
		t.Fatal("fiber.Map nil/empty distinction changed")
	}
	if got[0]["nilAnySlice"].([]any) != nil || got[0]["emptyAnySlice"].([]any) == nil {
		t.Fatal("[]any nil/empty distinction changed")
	}
	if got[0]["nilMapSlice"].([]map[string]any) != nil || got[0]["emptyMapSlice"].([]map[string]any) == nil {
		t.Fatal("[]map[string]any nil/empty distinction changed")
	}
	if got[0]["nilStringSlice"].([]string) != nil || got[0]["emptyStringSlice"].([]string) == nil {
		t.Fatal("[]string nil/empty distinction changed")
	}
	if got[0]["nilIntMap"].(map[string]int) != nil || got[0]["emptyIntMap"].(map[string]int) == nil {
		t.Fatal("map[string]int nil/empty distinction changed")
	}
}

func TestDecodeFiberMapSliceKeepsLegacyDepthAndNumberValidation(t *testing.T) {
	buildNested := func(mapCount int) fiber.Map {
		var current any = "leaf"
		for idx := 0; idx < mapCount; idx++ {
			current = map[string]any{"next": current}
		}
		return fiber.Map(current.(map[string]any))
	}

	if got := decodeFiberMapSlice([]fiber.Map{buildNested(32)}); len(got) != 1 {
		t.Fatalf("maximum supported depth decoded %d rows, want 1", len(got))
	}
	if got := decodeFiberMapSlice([]fiber.Map{buildNested(33)}); len(got) != 0 {
		t.Fatalf("over-depth payload decoded %d rows, want fail-closed empty slice", len(got))
	}
	for _, invalid := range []any{math.NaN(), math.Inf(1), float32(math.Inf(-1))} {
		if got := decodeFiberMapSlice([]fiber.Map{{"invalid": invalid}}); len(got) != 0 {
			t.Fatalf("invalid number %#v decoded %d rows, want fail-closed empty slice", invalid, len(got))
		}
	}
}

func TestDecodeFiberMapSliceFailsClosedWithNonNilEmptySlice(t *testing.T) {
	for _, raw := range []any{
		nil,
		"invalid",
		[]string{"invalid"},
		[]any{"invalid"},
		[]any{map[int]any{1: "invalid"}},
		[]any{fiber.Map{"opaque": playbackCloneUnsupported{hidden: map[string]any{"value": 1}}}},
	} {
		got := decodeFiberMapSlice(raw)
		if got == nil {
			t.Fatalf("decoded %#v to nil slice", raw)
		}
		if len(got) != 0 {
			t.Fatalf("decoded %#v to %d rows, want 0", raw, len(got))
		}
	}
}

func BenchmarkDecodeFiberMapSlice(b *testing.B) {
	rows := make([]fiber.Map, 24)
	for idx := range rows {
		rows[idx] = playbackCloneFixture()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		_ = decodeFiberMapSlice(rows)
	}
}

func BenchmarkDecodeFiberMapSliceLegacyJSONRoundTrip(b *testing.B) {
	rows := make([]fiber.Map, 24)
	for idx := range rows {
		rows[idx] = playbackCloneFixture()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		raw, err := json.Marshal(rows)
		if err != nil {
			b.Fatal(err)
		}
		out := []fiber.Map{}
		if err := json.Unmarshal(raw, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCloneBattlePlaybackFiberMapFast(b *testing.B) {
	source := playbackCloneFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		if _, ok := cloneBattlePlaybackFiberMap(source, 0); !ok {
			b.Fatal("fast clone failed")
		}
	}
}

func BenchmarkCloneBattlePlaybackFiberMapLegacyReflection(b *testing.B) {
	source := playbackCloneFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		if _, ok := cloneBattlePlaybackFiberMapReflect(source, 0); !ok {
			b.Fatal("legacy reflection clone failed")
		}
	}
}

type playbackCloneUnsupported struct {
	hidden map[string]any
}

func playbackCloneFixture() fiber.Map {
	return fiber.Map{
		"unitId": "player:1",
		"alive":  true,
		"level":  int64(7),
		"ratio":  json.Number("1.25"),
		"nested": map[string]any{
			"label": "original",
		},
		"stats": map[string]float64{
			"hp": 100,
		},
		"skills": []fiber.Map{
			{"id": "skill:1", "cooldownRemain": 2},
		},
		"effects": []battleSkillEffect{
			{
				Kind: "buff",
				WeightedStatusChoices: []battleWeightedStatusChoice{
					{
						ID:       "choice:1",
						Weight:   1,
						StatusID: "status:1",
						Payload:  map[string]any{"stacks": int64(2)},
					},
				},
			},
		},
	}
}
