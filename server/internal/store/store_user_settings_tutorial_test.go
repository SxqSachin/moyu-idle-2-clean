package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsValidTutorialID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"combat", true},
		{"actions", true},
		{"character-equipment", true},
		{"character_skills", true},
		{"CharacterLoadouts", true},
		{"", false},
		{"a b", false},
		{"tutorialProgress,combat", false}, // jsonb path injection attempt
		{"combat'}", false},
		{"a/b", false},
		{strings.Repeat("x", 65), false},
		{strings.Repeat("x", 64), true},
	}
	for _, tc := range cases {
		if got := IsValidTutorialID(tc.id); got != tc.want {
			t.Errorf("IsValidTutorialID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestTutorialProgressEntryJSONRoundTrip(t *testing.T) {
	entry := TutorialProgressEntry{Completed: true, CompletedAt: 1718200000000, Version: 3}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal err=%v", err)
	}
	// Wire keys must match the frontend TutorialProgress shape.
	if !strings.Contains(string(raw), `"completed":true`) ||
		!strings.Contains(string(raw), `"completedAt":1718200000000`) ||
		!strings.Contains(string(raw), `"version":3`) {
		t.Fatalf("unexpected json shape: %s", raw)
	}

	var decoded map[string]TutorialProgressEntry
	if err := json.Unmarshal([]byte(`{"combat":{"completed":true,"completedAt":1,"version":2}}`), &decoded); err != nil {
		t.Fatalf("unmarshal err=%v", err)
	}
	got := decoded["combat"]
	if !got.Completed || got.CompletedAt != 1 || got.Version != 2 {
		t.Fatalf("decoded mismatch: %+v", got)
	}
}
