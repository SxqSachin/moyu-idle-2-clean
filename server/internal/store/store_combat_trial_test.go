package store

import (
	"os"
	"strings"
	"testing"
)

func TestCombatTrialSchemaKeepsIndependentRunsMetricsAndRetention(t *testing.T) {
	assertForwardMigrationManifest(t, "combat_trial", []string{
		"20260831_combat_trial_v1.sql",
		"20260901_combat_trial_dsl_v2.sql",
		"20260901_combat_trial_review_hardening.sql",
		"20260901_combat_trial_v2_metric_outcomes.sql",
	})

	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260831_combat_trial_v1.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		if path == "../../schema/init.sql" {
			start := strings.Index(text, "-- === cat village combat trial ===")
			if start < 0 {
				t.Fatalf("%s missing combat trial schema section boundaries", path)
			}
			end := strings.Index(text[start:], "create table if not exists arena_shop_orders")
			if end < 0 {
				t.Fatalf("%s missing combat trial schema section boundaries", path)
			}
			text = text[start : start+end]
		}
		for _, required := range []string{
			"create table if not exists combat_trial_runs",
			"create table if not exists combat_trial_run_payloads",
			"create table if not exists combat_trial_personal_bests",
			"create table if not exists combat_trial_replay_shares",
			"battle_kind = 'combat_trial'",
			"format_definition_digest",
			"metrics_jsonb",
			"where status in ('queued', 'running')",
			"references combat_trial_runs(id) on delete cascade",
			"references combat_trial_runs(id) on delete set null",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing combat trial contract %q", path, required)
			}
		}
		if path == "../../schema/init.sql" && strings.Contains(text, "combat_trial_lineups") {
			t.Fatal("current schema must not persist a separate combat-trial lineup")
		}
		if path == "../../schema/init.sql" {
			for _, required := range []string{
				"value asc, achieved_at asc, user_id asc",
				"chk_combat_trial_run_end_reason",
				"chk_combat_trial_personal_best_end_reason",
				"end_reason = '' or end_reason ~ '^[a-z0-9_:-]{1,64}$'",
			} {
				if !strings.Contains(text, required) {
					t.Fatalf("%s missing current combat trial schema contract %q", path, required)
				}
			}
			for _, index := range []string{
				"uq_combat_trial_runs_user_active",
				"idx_combat_trial_runs_claim",
				"idx_combat_trial_runs_user_history",
				"idx_combat_trial_runs_current_format",
				"idx_combat_trial_personal_bests_leaderboard",
				"idx_combat_trial_personal_bests_leaderboard_asc",
				"idx_combat_trial_replay_shares_run",
				"uq_combat_trial_replay_shares_run_creator",
			} {
				if !strings.Contains(text, index) {
					t.Fatalf("%s missing current combat trial index %s", path, index)
				}
			}
		}
		for _, forbidden := range []string{"arena_rating", "attacker_rating_delta", "defender_rating_delta", "challenge_charges"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s couples combat trial persistence to Arena through %q", path, forbidden)
			}
		}
	}
}

func TestCombatTrialHardeningMigrationRemovesSavedLineupAndIndexesAscendingRanks(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260901_combat_trial_review_hardening.sql")
	if err != nil {
		t.Fatalf("read combat trial hardening migration: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, required := range []string{
		"drop table if exists combat_trial_lineups",
		"idx_combat_trial_personal_bests_leaderboard_asc",
		"value asc, achieved_at asc, user_id asc",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("combat trial hardening migration missing %q", required)
		}
	}
}

func TestCombatTrialPersonalBestTieKeepsFirstAchievementForBothRankDirections(t *testing.T) {
	raw, err := os.ReadFile("store_combat_trial.go")
	if err != nil {
		t.Fatalf("read combat trial store: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, required := range []string{
		"$10='desc' and excluded.value>combat_trial_personal_bests.value",
		"$10='asc' and excluded.value<combat_trial_personal_bests.value",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("personal-best upsert must replace only a strictly better value: missing %q", required)
		}
	}
	for _, required := range []string{
		"order by pb.value %s,pb.achieved_at asc,pb.user_id asc",
		"offset $2",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("combat trial store missing ordering/retention contract %q", required)
		}
	}
}

func TestCombatTrialMetricOutcomeMigrationPreservesPersonalBestSourceOutcome(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260901_combat_trial_v2_metric_outcomes.sql")
	if err != nil {
		t.Fatalf("read combat trial metric outcome migration: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, required := range []string{
		"add column if not exists end_reason",
		"from combat_trial_runs r",
		"pb.run_id = r.id",
		"chk_combat_trial_personal_best_end_reason",
		"end_reason = '' or end_reason ~ '^[a-z0-9_:-]{1,64}$'",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("combat trial metric outcome migration missing %q", required)
		}
	}
}

func TestCombatTrialClaimCandidateDoesNotExposeAmbiguousID(t *testing.T) {
	raw, err := os.ReadFile("store_combat_trial.go")
	if err != nil {
		t.Fatalf("read combat trial store: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, required := range []string{"select id as run_id from combat_trial_runs", "where r.id=candidate.run_id", "returning r.*", "from claimed"} {
		if !strings.Contains(text, required) {
			t.Fatalf("combat trial claim SQL missing unambiguous candidate contract %q", required)
		}
	}
	if strings.Contains(text, "where r.id=candidate.id") {
		t.Fatal("combat trial claim SQL still exposes an ambiguous id column")
	}
}

func TestCombatTrialDSLReasonMigrationKeepsEndReasonsDataDriven(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260901_combat_trial_dsl_v2.sql")
	if err != nil {
		t.Fatalf("read combat trial DSL migration: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, required := range []string{
		"drop constraint if exists combat_trial_runs_end_reason_check",
		"drop constraint if exists chk_combat_trial_run_end_reason",
		"end_reason ~ '^[a-z0-9_:-]{1,64}$'",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("combat trial DSL migration missing %q", required)
		}
	}
	if strings.Contains(text, "end_reason in ('', 'action_budget_reached', 'team_defeated')") {
		t.Fatal("combat trial DSL migration must not enumerate authored end reasons")
	}
}

func TestCombatTrialDownMigrationRemovesOnlyOwnedTables(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260831_combat_trial_v1_down.sql")
	if err != nil {
		t.Fatalf("read combat trial down migration: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, table := range []string{"combat_trial_replay_shares", "combat_trial_personal_bests", "combat_trial_run_payloads", "combat_trial_runs", "combat_trial_lineups"} {
		if !strings.Contains(text, "drop table if exists "+table) {
			t.Fatalf("down migration does not drop %s", table)
		}
	}
	if strings.Contains(text, "drop table if exists arena_") {
		t.Fatal("combat trial down migration must not remove Arena tables")
	}
}
