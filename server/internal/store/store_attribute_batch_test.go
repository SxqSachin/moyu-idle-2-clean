package store

import (
	"encoding/json"
	"testing"
)

func TestPlayerAttributeDefaultRowsFromSeeds(t *testing.T) {
	rows, err := playerAttributeDefaultRowsFromSeeds([]PlayerAttributeSeed{
		{AttrKey: " strength ", BaseDefault: 1.23456},
		{AttrKey: "", BaseDefault: 99},
		{AttrKey: "strength", BaseDefault: 9},
		{AttrKey: "hp", BaseDefault: 100},
	})
	if err != nil {
		t.Fatalf("playerAttributeDefaultRowsFromSeeds() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}
	if rows[0].AttrKey != "strength" || rows[0].FinalValue != 1.23456 {
		t.Fatalf("first row = %+v, want strength/1.23456", rows[0])
	}
	if rows[1].AttrKey != "hp" || rows[1].FinalValue != 100 {
		t.Fatalf("second row = %+v, want hp/100", rows[1])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rows[0].SourcePayload), &payload); err != nil {
		t.Fatalf("source payload json error = %v", err)
	}
	if payload["version"] != "v2" {
		t.Fatalf("payload version = %v, want v2", payload["version"])
	}
	base, ok := payload["base"].(map[string]any)
	if !ok {
		t.Fatalf("payload base = %#v, want object", payload["base"])
	}
	if base["value"] != 1.2346 || base["baseDefault"] != 1.2346 {
		t.Fatalf("payload base = %#v, want rounded defaults", base)
	}
}
