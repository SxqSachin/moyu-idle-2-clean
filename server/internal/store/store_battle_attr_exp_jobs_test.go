package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBattleAttrExpJobSchemaStaysAligned(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260828_battle_attr_exp_jobs.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(raw)
		for _, required := range []string{
			"battle_attr_exp_jobs",
			"UNIQUE(run_id)",
			"payload_jsonb JSONB",
			"applied_at TIMESTAMPTZ",
			"idx_battle_attr_exp_jobs_claim",
			"idx_battle_attr_exp_jobs_ready",
			"idx_battle_attr_exp_jobs_expired_lease",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
	}
}

func TestBattleAttrExpClaimUsesUnappliedPartialIndexPredicates(t *testing.T) {
	raw, err := os.ReadFile("store_battle_attr_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "func (s *Store) ClaimBattleAttrExpJobs")
	if start < 0 {
		t.Fatal("attribute reward claim method missing")
	}
	body := text[start:]
	if next := strings.Index(body, "\n// ApplyBattleAttrExpJob"); next >= 0 {
		body = body[:next]
	}
	for _, required := range []string{
		"applied_at IS NULL",
		"status IN ('pending', 'failed')",
		"status = 'running'",
		"FOR UPDATE SKIP LOCKED",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("attribute reward claim query missing %q", required)
		}
	}
}

func TestBattleAttrExpJobAppliesRewardAndDoneMarkerAtomically(t *testing.T) {
	raw, err := os.ReadFile("store_battle_attr_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "func (s *Store) ApplyBattleAttrExpJob")
	if start < 0 {
		t.Fatal("attribute reward job apply method missing")
	}
	body := text[start:]
	if next := strings.Index(body, "\nfunc (s *Store) MarkBattleAttrExpJobFailed"); next >= 0 {
		body = body[:next]
	}
	transactionAt := strings.Index(body, "s.withTx")
	lockAt := strings.Index(body, "FOR UPDATE")
	guardAt := strings.Index(body, "appliedAt != nil")
	leaseGuardAt := strings.Index(body, "lockedBy != workerID || attempts != claimAttempt")
	tryLockAt := strings.Index(body, "s.tryLockUserTx(ctx, tx, userID)")
	applyAt := strings.Index(body, "applyBattleWaveAttrRewardsWithMetricsTx")
	doneAt := strings.Index(body, "applied_at = now()")
	if transactionAt < 0 || lockAt <= transactionAt || guardAt <= lockAt || leaseGuardAt <= guardAt || tryLockAt <= leaseGuardAt || applyAt <= tryLockAt || doneAt <= applyAt {
		t.Fatalf("attribute rewards and the idempotent done marker are no longer ordered in one locked transaction")
	}
	if !strings.Contains(body, "return ErrBattleAttrExpUserBusy") {
		t.Fatal("background attribute reward job can block behind a player-facing user lock")
	}
	for _, fenced := range []string{"locked_by = $4", "attempts = $5"} {
		if !strings.Contains(body, fenced) {
			t.Fatalf("attribute reward done marker missing lease fence %q", fenced)
		}
	}
}

func TestBattleAttrExpFailureUpdateIsLeaseFenced(t *testing.T) {
	raw, err := os.ReadFile("store_battle_attr_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) MarkBattleAttrExpJobFailed")
	if start < 0 {
		t.Fatal("MarkBattleAttrExpJobFailed missing")
	}
	body = body[start:]
	for _, required := range []string{
		"status = $5",
		"locked_by = $6",
		"attempts = $7",
		"tag.RowsAffected() == 1",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("attribute reward failure update missing fence %q", required)
		}
	}
}

func TestBattleAttrExpQuarantineUpdateIsLeaseFenced(t *testing.T) {
	raw, err := os.ReadFile("store_battle_attr_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) MarkBattleAttrExpJobQuarantined")
	if start < 0 {
		t.Fatal("MarkBattleAttrExpJobQuarantined missing")
	}
	body = body[start:]
	for _, required := range []string{
		"BattleAttrExpJobStatusQuarantined",
		"applied_at IS NULL",
		"status = $4",
		"locked_by = $5",
		"attempts = $6",
		"tag.RowsAffected() == 1",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("attribute reward quarantine update missing fence %q", required)
		}
	}
}

func TestBattleAttrExpQuarantineSchemaStaysAligned(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_battle_settlement_quarantine.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(raw)
		start := strings.Index(text, "battle_attr_exp_jobs")
		if start < 0 {
			t.Fatalf("%s missing battle_attr_exp_jobs", path)
		}
		body := text[start:]
		if next := strings.Index(body, ";"); next >= 0 {
			body = body[:next]
		}
		if !strings.Contains(body, "quarantined") {
			t.Fatalf("%s attribute reward status constraint is missing quarantined", path)
		}
	}

	raw, err := os.ReadFile("../../schema/migrations/20260829_battle_settlement_quarantine.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	alterAt := strings.Index(text, "ALTER TABLE battle_attr_exp_jobs")
	if alterAt < 0 {
		t.Fatal("attribute reward status constraint replacement missing")
	}
	firstAlter := strings.SplitN(text[alterAt:], ";", 2)[0]
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS battle_attr_exp_jobs_status_check,",
		"ADD CONSTRAINT battle_attr_exp_jobs_status_check",
		"NOT VALID",
	} {
		if !strings.Contains(firstAlter, required) {
			t.Fatalf("attribute reward atomic constraint replacement missing %q", required)
		}
	}
	if !strings.Contains(text[alterAt:], "VALIDATE CONSTRAINT battle_attr_exp_jobs_status_check") {
		t.Fatal("attribute reward status constraint is not validated")
	}
}

func TestBattleAttrExpClaimDoesNotIncludeQuarantined(t *testing.T) {
	raw, err := os.ReadFile("store_battle_attr_exp_jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) ClaimBattleAttrExpJobs")
	if start < 0 {
		t.Fatal("ClaimBattleAttrExpJobs missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\n// ApplyBattleAttrExpJob"); next >= 0 {
		body = body[:next+1]
	}
	if strings.Contains(body, "BattleAttrExpJobStatusQuarantined") || strings.Contains(body, "'quarantined'") {
		t.Fatal("quarantined attribute experience jobs must not be claimable")
	}
}

func TestEnqueueBattleAttrExpJobTxUsesIdempotentRunOutbox(t *testing.T) {
	t.Parallel()

	tx := &queuePerformanceExecTx{}
	if err := (&Store{}).enqueueBattleAttrExpJobTx(context.Background(), tx, 17, []byte(`[{"userId":9,"attrExp":{"strength":1.25}}]`), time.Time{}); err != nil {
		t.Fatal(err)
	}
	if tx.execCount != 1 {
		t.Fatalf("outbox insert executions=%d, want 1", tx.execCount)
	}
	if !strings.Contains(tx.sql, "ON CONFLICT (run_id) DO NOTHING") {
		t.Fatalf("outbox insert is not idempotent by run: %s", tx.sql)
	}
}

func TestAttributeExpProgressOnlySkipsDependentRefreshes(t *testing.T) {
	checks := []struct {
		path          string
		function      string
		refreshCall   string
		missingEnsure string
	}{
		{
			path:        "action_queue.go",
			function:    "applyAttrExpLockedResultTx",
			refreshCall: "onPlayerAttributesChangedTx",
		},
		{
			path:          "store_kitty_battle_points.go",
			function:      "applyKittyAttrExpLockedResultTx",
			refreshCall:   "recomputeKittyDerivedStatsTx",
			missingEnsure: "ensureKittyAttributesTx",
		},
	}
	for _, check := range checks {
		raw, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		text := string(raw)
		start := strings.Index(text, "func (s *Store) "+check.function)
		if start < 0 {
			t.Fatalf("%s missing", check.function)
		}
		body := text[start:]
		if next := strings.Index(body, "\nfunc (s *Store)"); next >= 0 {
			body = body[:next]
		}
		noMutationAt := strings.Index(body, "levelDelta == 0 && math.Abs(afterExp-beforeExp) < 1e-9")
		updateAt := strings.Index(body, "UPDATE player_")
		refreshGuardAt := strings.Index(body, "if !result.LevelChanged")
		refreshAt := strings.LastIndex(body, check.refreshCall)
		if noMutationAt < 0 || updateAt <= noMutationAt || refreshGuardAt <= updateAt || refreshAt <= refreshGuardAt {
			t.Fatalf("%s must skip no-op writes and refresh dependents only after a level change", check.function)
		}
		if check.missingEnsure != "" {
			missingRowAt := strings.Index(body, "errors.Is(err, pgx.ErrNoRows)")
			ensureAt := strings.Index(body, check.missingEnsure)
			if missingRowAt < 0 || ensureAt <= missingRowAt {
				t.Fatalf("%s must initialize kitty attributes only after observing a missing row", check.function)
			}
		}
	}
}

func TestBattleRunCleanupRetainsUnappliedAttrExpJobs(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, functionName := range []string{"CompactBattleRunHistories", "PurgeBattleRunHistory"} {
		start := strings.Index(text, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s missing", functionName)
		}
		body := text[start:]
		if next := strings.Index(body, "\nfunc (s *Store)"); next >= 0 {
			body = body[:next]
		}
		for _, required := range []string{
			"battle_attr_exp_jobs",
			"applied_at IS NULL",
			"battle_settlement_jobs",
			"settlement_job.status <> 'done'",
			"battle_skill_exp_jobs",
			"skill_job.status <> 'done'",
		} {
			if !strings.Contains(body, required) {
				t.Fatalf("%s can delete a battle run with unfinished reward work; missing %q", functionName, required)
			}
		}
	}
}
