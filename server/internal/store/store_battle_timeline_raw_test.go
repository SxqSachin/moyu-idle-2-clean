package store

import "testing"

func TestMarshalBattleRunTimelineJSONProducesOpaqueJSONArray(t *testing.T) {
	timelineJSON, err := MarshalBattleRunTimelineJSON([]map[string]any{{"seq": int64(1)}})
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	raw, err := timelineJSON.bytesForSave()
	if err != nil {
		t.Fatalf("read timeline for save: %v", err)
	}
	if got, want := string(raw), `[{"seq":1}]`; got != want {
		t.Fatalf("timeline JSON=%s, want %s", got, want)
	}
	clone := timelineJSON.CloneBytes()
	clone[0] = '{'
	if got := string(timelineJSON.CloneBytes()); got != `[{"seq":1}]` {
		t.Fatalf("mutating cloned bytes changed timeline JSON: %s", got)
	}
}

func TestBattleRunTimelineJSONPreservesNilAndRejectsZeroValue(t *testing.T) {
	timelineJSON, err := MarshalBattleRunTimelineJSON(nil)
	if err != nil {
		t.Fatalf("marshal nil timeline: %v", err)
	}
	if got := string(timelineJSON.CloneBytes()); got != "null" {
		t.Fatalf("nil timeline JSON=%s, want null", got)
	}
	var zero BattleRunTimelineJSON
	if _, err := zero.bytesForSave(); err == nil {
		t.Fatal("zero timeline JSON must not be persisted")
	}
}
