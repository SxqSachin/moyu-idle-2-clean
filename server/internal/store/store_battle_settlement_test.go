package store

import (
	"os"
	"strings"
	"testing"
)

func TestBattleSettlementQuarantineSchemaStaysAligned(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_battle_settlement_quarantine.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(raw)
		for _, required := range []string{"battle_settlement_jobs", "quarantined"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
	}
}

func TestBattleSettlementQuarantineMigrationNeverLeavesConstraintDropped(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260829_battle_settlement_quarantine.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "SET lock_timeout = '5s'") {
		t.Fatal("settlement constraint migration must fail fast instead of waiting behind production traffic")
	}
	for _, table := range []string{"battle_settlement_jobs", "battle_skill_exp_jobs"} {
		alterAt := strings.Index(text, "ALTER TABLE "+table)
		if alterAt < 0 {
			t.Fatalf("%s constraint replacement missing", table)
		}
		firstAlter := strings.SplitN(text[alterAt:], ";", 2)[0]
		for _, required := range []string{
			"DROP CONSTRAINT IF EXISTS " + table + "_status_check,",
			"ADD CONSTRAINT " + table + "_status_check",
			"NOT VALID",
		} {
			if !strings.Contains(firstAlter, required) {
				t.Fatalf("%s atomic replacement statement missing %q", table, required)
			}
		}
	}
}

func TestBattleSettlementStatusUpdatesAreLeaseFenced(t *testing.T) {
	raw, err := os.ReadFile("store_battle_settlement.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, functionName := range []string{
		"MarkBattleSettlementJobFailed",
		"MarkBattleSettlementJobQuarantined",
	} {
		start := strings.Index(source, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s missing", functionName)
		}
		body := source[start:]
		if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
			body = body[:next+1]
		}
		for _, required := range []string{
			"BattleSettlementJobStatusRunning",
			"locked_by =",
			"attempts =",
			"RowsAffected() == 1",
		} {
			if !strings.Contains(body, required) {
				t.Fatalf("%s missing lease fence %q", functionName, required)
			}
		}
	}
	start := strings.Index(source, "func (s *Store) MarkBattleSettlementJobDone")
	if start < 0 {
		t.Fatal("MarkBattleSettlementJobDone missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	for _, required := range []string{
		"BattleSettlementJobStatusRunning",
		"locked_by = $4",
		"attempts = $5",
		"RETURNING run_id",
		"errors.Is(err, pgx.ErrNoRows)",
		"deleteEmptyBattleRoomAfterRewardJobsTx(ctx, tx, runID)",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("MarkBattleSettlementJobDone missing atomic fence %q", required)
		}
	}
}

func TestBattleSettlementClaimDoesNotIncludeQuarantined(t *testing.T) {
	raw, err := os.ReadFile("store_battle_settlement.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) ClaimBattleSettlementJobs")
	if start < 0 {
		t.Fatal("ClaimBattleSettlementJobs missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	if strings.Contains(body, "BattleSettlementJobStatusQuarantined") {
		t.Fatal("quarantined settlement jobs must not be claimable")
	}
}
