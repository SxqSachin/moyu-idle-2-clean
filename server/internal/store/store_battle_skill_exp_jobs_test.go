package store

import (
	"os"
	"strings"
	"testing"
)

func TestBattleSkillExpJobAppliesRewardAndDoneMarkerAtomically(t *testing.T) {
	raw, err := os.ReadFile("store_battle_skill_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) ApplyBattleSkillExpJob")
	if start < 0 {
		t.Fatal("ApplyBattleSkillExpJob missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	transactionAt := strings.Index(body, "s.withTx")
	lockAt := strings.Index(body, "FOR UPDATE")
	leaseGuardAt := strings.Index(body, "lockedBy != workerID || attempts != claimAttempt")
	applyAt := strings.Index(body, "applyBattleSkillExpRewardsTx")
	doneAt := strings.Index(body, "tag, err := tx.Exec")
	if transactionAt < 0 || lockAt <= transactionAt || leaseGuardAt <= lockAt || applyAt <= leaseGuardAt || doneAt <= applyAt {
		t.Fatal("skill rewards and the done marker must stay ordered in one lease-fenced transaction")
	}
	for _, required := range []string{
		"status = $3",
		"locked_by = $4",
		"attempts = $5",
		"tag.RowsAffected() != 1",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("skill reward done marker missing fence %q", required)
		}
	}
}

func TestBattleSkillExpFailureUpdateIsLeaseFenced(t *testing.T) {
	raw, err := os.ReadFile("store_battle_skill_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) MarkBattleSkillExpJobFailed")
	if start < 0 {
		t.Fatal("MarkBattleSkillExpJobFailed missing")
	}
	body = body[start:]
	for _, required := range []string{
		"status = $5",
		"locked_by = $6",
		"attempts = $7",
		"tag.RowsAffected() == 1",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("skill reward failure update missing fence %q", required)
		}
	}
}

func TestBattleSkillExpQuarantineUpdateIsLeaseFenced(t *testing.T) {
	raw, err := os.ReadFile("store_battle_skill_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) MarkBattleSkillExpJobQuarantined")
	if start < 0 {
		t.Fatal("MarkBattleSkillExpJobQuarantined missing")
	}
	body = body[start:]
	for _, required := range []string{
		"BattleSkillExpJobStatusQuarantined",
		"status = $4",
		"locked_by = $5",
		"attempts = $6",
		"tag.RowsAffected() == 1",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("skill reward quarantine update missing fence %q", required)
		}
	}
}

func TestBattleSkillExpQuarantineSchemaStaysAligned(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_battle_settlement_quarantine.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(raw)
		for _, required := range []string{"battle_skill_exp_jobs", "quarantined"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
	}
}

func TestBattleSkillExpClaimDoesNotIncludeQuarantined(t *testing.T) {
	raw, err := os.ReadFile("store_battle_skill_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) ClaimBattleSkillExpJobs")
	if start < 0 {
		t.Fatal("ClaimBattleSkillExpJobs missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	if strings.Contains(body, "BattleSkillExpJobStatusQuarantined") {
		t.Fatal("quarantined skill experience jobs must not be claimable")
	}
}

func TestBattleFinalizationEnqueuesSkillExpInCoreTransaction(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, functionName := range []string{
		"CompleteBattleRunAndMaybeContinue",
		"HandleBattleRunFailure",
		"FinishBattleRoom",
	} {
		start := strings.Index(source, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s missing", functionName)
		}
		body := source[start:]
		if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
			body = body[:next+1]
		}
		runFenceAt := strings.Index(body, "*currentRunID != expectedRunID")
		enqueueAt := strings.Index(body, "enqueueBattleSkillExpJobTx(ctx, tx, finalizedRunID, skillExpJobPayload, now)")
		roomUpdateAt := strings.Index(body, "UPDATE battle_rooms")
		if runFenceAt < 0 || enqueueAt <= runFenceAt || roomUpdateAt <= enqueueAt {
			t.Fatalf("%s must fence the run, enqueue skill experience, then release the room to its next state", functionName)
		}
	}

	jobRaw, err := os.ReadFile("store_battle_skill_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	jobSource := string(jobRaw)
	start := strings.Index(jobSource, "func (s *Store) enqueueBattleSkillExpJobTx")
	if start < 0 {
		t.Fatal("transaction-aware skill experience enqueue helper missing")
	}
	body := jobSource[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	for _, required := range []string{"db.Exec", "ON CONFLICT (run_id) DO NOTHING", "validatePostgresJSONBRaw"} {
		if !strings.Contains(body, required) {
			t.Fatalf("transaction-aware skill experience enqueue missing %q", required)
		}
	}
}

func TestBattleFinalizationEnqueuesKillAchievementsInCoreTransaction(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, functionName := range []string{
		"CompleteBattleRunAndMaybeContinue",
		"HandleBattleRunFailure",
		"FinishBattleRoom",
	} {
		start := strings.Index(source, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s missing", functionName)
		}
		body := source[start:]
		if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
			body = body[:next+1]
		}
		runFenceAt := strings.Index(body, "*currentRunID != expectedRunID")
		enqueueAt := strings.Index(body, "enqueueBattleAchievementEventBatchesTx(ctx, tx, killAchievementBatches)")
		roomUpdateAt := strings.Index(body, "UPDATE battle_rooms")
		if runFenceAt < 0 || enqueueAt <= runFenceAt || roomUpdateAt <= enqueueAt {
			t.Fatalf("%s must fence the run, enqueue kill achievements, then release the room", functionName)
		}
	}

	start := strings.Index(source, "func (s *Store) enqueueBattleAchievementEventBatchesTx")
	if start < 0 {
		t.Fatal("transaction-aware battle achievement enqueue helper missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
		body = body[:next+1]
	}
	if !strings.Contains(body, "s.EnqueueAchievementEventsTx(ctx, tx, batch.UserID, batch.Events)") {
		t.Fatal("battle achievement helper must enqueue every batch through the caller's transaction")
	}
}
