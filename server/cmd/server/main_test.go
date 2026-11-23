package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mysecondwebgame/server/internal/gamedata"
)

func TestKittyPersonalityConfigsFromCatalog_PreservesBattleEffects(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	path := filepath.Join(root, "packages", "game-data", "generated", "chunks", "kitty_personalities.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kitty personalities: %v", err)
	}

	var payload struct {
		KittyPersonalities []map[string]any `json:"kitty_personalities"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal kitty personalities: %v", err)
	}

	rows := gamedata.KittyPersonalityConfigsFromCatalog(payload.KittyPersonalities)
	byID := map[string]bool{}
	for _, row := range rows {
		switch row.ID {
		case "taunting_tail":
			byID[row.ID] = true
			if len(row.Effects) != 1 {
				t.Fatalf("taunting_tail effects len=%d, want 1", len(row.Effects))
			}
			if len(row.Effects[0].BattleHooks) != 1 {
				t.Fatalf("taunting_tail battle hooks len=%d, want 1", len(row.Effects[0].BattleHooks))
			}
		case "sweeping_claws":
			byID[row.ID] = true
			if len(row.Effects) != 1 {
				t.Fatalf("sweeping_claws effects len=%d, want 1", len(row.Effects))
			}
			if len(row.Effects[0].EffectMutations) != 1 {
				t.Fatalf("sweeping_claws effect mutations len=%d, want 1", len(row.Effects[0].EffectMutations))
			}
		}
	}

	if !byID["taunting_tail"] {
		t.Fatal("taunting_tail missing from parsed rows")
	}
	if !byID["sweeping_claws"] {
		t.Fatal("sweeping_claws missing from parsed rows")
	}
}
