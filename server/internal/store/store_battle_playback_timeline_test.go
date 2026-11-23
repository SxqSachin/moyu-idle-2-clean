package store

import (
	"strings"
	"testing"
)

func TestBattleRunPlaybackTimelineQueriesExcludeUnneededArtifacts(t *testing.T) {
	coldSQL := strings.ToLower(battleRunPlaybackTimelineColdSQL)
	for _, expected := range []string{"timeline_raw", "timeline_jsonb", "battle_run_payloads"} {
		if !strings.Contains(coldSQL, expected) {
			t.Fatalf("cold timeline query missing %q: %s", expected, coldSQL)
		}
	}
	for _, forbidden := range []string{"compile_input", "final_result", "carry_state"} {
		if strings.Contains(coldSQL, forbidden) {
			t.Fatalf("cold timeline query loads %q: %s", forbidden, coldSQL)
		}
	}
	legacySQL := strings.ToLower(battleRunPlaybackTimelineLegacySQL)
	if !strings.Contains(legacySQL, "timeline_jsonb") || !strings.Contains(legacySQL, "battle_runs") {
		t.Fatalf("legacy timeline query lost fallback: %s", legacySQL)
	}
}

func TestDecodeBattleRunTimelinePayloadPrefersRawBytes(t *testing.T) {
	raw := []byte(`[{"seq":2,"source":"raw"}]`)
	jsonb := []byte(`[{"seq":1,"source":"jsonb"}]`)
	timeline := decodeBattleRunTimelinePayload(raw, jsonb)
	if len(timeline) != 1 || timeline[0]["source"] != "raw" {
		t.Fatalf("decoded timeline=%#v, want raw payload", timeline)
	}
}

func TestDecodeBattleRunTimelinePayloadFallsBackAndFailsClosed(t *testing.T) {
	timeline := decodeBattleRunTimelinePayload(nil, []byte(`[{"seq":1}]`))
	if len(timeline) != 1 {
		t.Fatalf("decoded fallback rows=%d, want 1", len(timeline))
	}
	for _, raw := range [][]byte{nil, []byte(`not-json`)} {
		got := decodeBattleRunTimelinePayload(raw, nil)
		if got == nil {
			t.Fatalf("decoded %q to nil slice", raw)
		}
		if len(got) != 0 {
			t.Fatalf("decoded %q to %d rows, want 0", raw, len(got))
		}
	}
}
