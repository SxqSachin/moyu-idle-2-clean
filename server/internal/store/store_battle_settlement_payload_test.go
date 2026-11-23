package store

import (
	"strings"
	"testing"
)

func TestBattleRunSettlementQueryReadsOnlyHotStateAndCompactSummary(t *testing.T) {
	normalized := strings.ToLower(strings.Join(strings.Fields(getBattleRunSettlementByIDSQL), " "))
	for _, required := range []string{
		"playback_state_jsonb",
		"result_summary_jsonb",
		"from battle_runs",
		"where id = $1",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("settlement query missing %q: %s", required, normalized)
		}
	}
	for _, forbidden := range []string{
		"compile_input",
		"timeline",
		"final_result",
		"battle_run_payloads",
		"battle_run_carry_states",
	} {
		if strings.Contains(normalized, forbidden) {
			t.Fatalf("settlement query must not read cold field %q: %s", forbidden, normalized)
		}
	}
}
