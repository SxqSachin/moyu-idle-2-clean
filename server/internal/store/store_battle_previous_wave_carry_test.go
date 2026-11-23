package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBattlePreviousWaveDedicatedCarryIsAuthoritative(t *testing.T) {
	raw, ok := resolveBattlePreviousWaveDedicatedCarryRaw(true, []byte(`{"status":"ended","players":[]}`))
	if !ok || string(raw) != `{"status":"ended","players":[]}` {
		t.Fatalf("dedicated carry was not authoritative: ok=%v raw=%s", ok, raw)
	}
	if _, ok := resolveBattlePreviousWaveDedicatedCarryRaw(false, []byte(`{"status":"ended"}`)); ok {
		t.Fatal("missing dedicated row must enter the legacy fallback path")
	}
	for _, malformed := range [][]byte{nil, []byte(`{"status":`)} {
		raw, ok := resolveBattlePreviousWaveDedicatedCarryRaw(true, malformed)
		if !ok {
			t.Fatalf("present dedicated row must not fall back: ok=%v raw=%s", ok, raw)
		}
		if len(malformed) == 0 && string(raw) != "null" {
			t.Fatalf("empty authoritative carry must stay authoritative as null: %s", raw)
		}
		if len(malformed) > 0 && string(raw) != string(malformed) {
			t.Fatalf("malformed authoritative carry was replaced by fallback: got=%s want=%s", raw, malformed)
		}
	}
}

func TestBattlePreviousWaveLegacyCarryPriority(t *testing.T) {
	tests := []struct {
		name    string
		sources battlePreviousWaveCarrySources
		want    string
	}{
		{
			name: "cold raw before cold jsonb",
			sources: battlePreviousWaveCarrySources{
				payloadExists:  true,
				coldFinalRaw:   []byte(`{"carryState":{"source":"raw"}}`),
				coldJSONBCarry: []byte(`{"source":"jsonb"}`),
				inlineCarry:    []byte(`{"source":"inline"}`),
			},
			want: `{"source":"raw"}`,
		},
		{
			name: "cold jsonb before inline",
			sources: battlePreviousWaveCarrySources{
				payloadExists:  true,
				coldJSONBCarry: []byte(`{"source":"jsonb"}`),
				inlineCarry:    []byte(`{"source":"inline"}`),
			},
			want: `{"source":"jsonb"}`,
		},
		{
			name: "inline only without payload row",
			sources: battlePreviousWaveCarrySources{
				inlineCarry: []byte(`{"source":"inline"}`),
			},
			want: `{"source":"inline"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := resolveBattlePreviousWaveCarryLegacyRaw(test.sources)
			if err != nil || string(raw) != test.want {
				t.Fatalf("raw=%s err=%v", raw, err)
			}
		})
	}
	if _, err := resolveBattlePreviousWaveCarryLegacyRaw(battlePreviousWaveCarrySources{
		payloadExists: true,
		coldFinalRaw:  []byte(`{"carryState":`),
	}); err == nil {
		t.Fatal("malformed cold raw payload must fail closed")
	}
}

func TestBattlePreviousWaveCarryQueriesFenceLineageAndKeepModernPathSmall(t *testing.T) {
	modern := strings.ToLower(strings.Join(strings.Fields(getBattlePreviousWaveCarryModernQuery), " "))
	for _, fragment := range []string{
		"current.id = room.current_run_id",
		"current.room_id = room.id",
		"room.id = $1",
		"current.id = $2",
		"current.wave = $3 + 1",
		"candidate.room_id = current.room_id",
		"candidate.id < current.id",
		"candidate.wave = $3",
		"order by candidate.id desc",
		"left join battle_run_carry_states",
		"carry.run_id = previous.id",
	} {
		if !strings.Contains(modern, fragment) {
			t.Fatalf("modern query missing lineage fence %q: %s", fragment, modern)
		}
	}
	for _, forbidden := range []string{"battle_run_payloads", "final_result", "timeline", "compile_input", "for update"} {
		if strings.Contains(modern, forbidden) {
			t.Fatalf("modern carry query includes cold payload %q: %s", forbidden, modern)
		}
	}

	legacy := strings.ToLower(strings.Join(strings.Fields(getBattlePreviousWaveCarryLegacyQuery), " "))
	for _, fragment := range []string{
		"battle_run_payloads",
		"payload.run_id = previous.id",
		"final_result_raw",
		"final_result_jsonb -> 'carrystate'",
		"payload.run_id is null",
		"previous.id = $1",
		"previous.room_id = $2",
		"previous.wave = $3",
	} {
		if !strings.Contains(legacy, fragment) {
			t.Fatalf("legacy query missing fallback rule %q: %s", fragment, legacy)
		}
	}
	if strings.Contains(legacy, "timeline") || strings.Contains(legacy, "for update") {
		t.Fatalf("legacy carry query loads timeline or locks rows: %s", legacy)
	}
}

func TestNormalizeBattleCarryRawNormalizesMissingToJSONNull(t *testing.T) {
	if !json.Valid(normalizeBattleCarryRaw(nil)) || string(normalizeBattleCarryRaw(nil)) != "null" {
		t.Fatalf("missing carry was not normalized to JSON null: %s", normalizeBattleCarryRaw(nil))
	}
}
