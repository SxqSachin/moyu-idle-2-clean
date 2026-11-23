package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidArenaKittyIDsAllowsOptionalSlots(t *testing.T) {
	if ArenaMaxKittyCount != 2 {
		t.Fatalf("current arena kitty limit = %d, want 2", ArenaMaxKittyCount)
	}
	for _, kittyIDs := range [][]int64{nil, {}, {10}, {10, 11}} {
		if !validArenaKittyIDs(kittyIDs) {
			t.Fatalf("kitty ids %v should be valid", kittyIDs)
		}
	}
	for _, kittyIDs := range [][]int64{{0}, {-1}, {10, 10}, {10, 11, 12}} {
		if validArenaKittyIDs(kittyIDs) {
			t.Fatalf("kitty ids %v should be invalid", kittyIDs)
		}
	}
}

func TestArenaDefenseKittyConstraintRepairsLegacyWithoutPersistingCountLimit(t *testing.T) {
	paths := []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_arena.sql",
		"../../schema/migrations/20260830_arena_optional_kitties.sql",
		"../../schema/migrations/20260830_arena_optional_kitties_repair.sql",
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, required := range []string{
			"arena_kitty_ids_are_valid",
			"arena_defenses_valid_kitty_ids",
			"0::bigint < all($1)",
			"count(distinct kitty_id)",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing flexible kitty constraint contract %q", path, required)
			}
		}
		if strings.Contains(text, "cardinality(kitty_ids) between 0 and 2") {
			t.Fatalf("%s must not persist the current kitty count limit in the database", path)
		}
	}

	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260830_arena_optional_kitties_repair.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		if !strings.Contains(text, "drop constraint") {
			t.Fatalf("%s does not drop legacy arena constraints", path)
		}
		for _, legacy := range []string{
			"arena_defenses_two_distinct_kitties",
			"arena_defenses_up_to_two_distinct_kitties",
		} {
			if !strings.Contains(text, legacy) {
				t.Fatalf("%s does not repair legacy constraint %q", path, legacy)
			}
		}
	}
}

func TestArenaReplayShareSchemaUsesRetentionAndCarriesChatAttachment(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260830_arena_replay_shares.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, required := range []string{
			"arena_replay_shares",
			"references arena_challenges(id) on delete cascade",
			"uq_arena_replay_shares_challenge_creator",
			"expires_at",
			"revoked_at",
			"attachment_jsonb",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
		if strings.Contains(text, "expires_at     timestamptz not null") || strings.Contains(text, "arena_replay_shares_expiry check") {
			t.Fatalf("%s must leave replay shares without a default time expiry", path)
		}
	}
	repair, err := os.ReadFile("../../schema/migrations/20260830_arena_replay_retention_only.sql")
	if err != nil {
		t.Fatalf("read retention-only migration: %v", err)
	}
	repairText := strings.ToLower(string(repair))
	for _, required := range []string{"drop constraint if exists arena_replay_shares_expiry", "alter column expires_at drop not null", "set expires_at = null", "drop index if exists idx_arena_replay_shares_expiry"} {
		if !strings.Contains(repairText, required) {
			t.Fatalf("retention-only migration missing %q", required)
		}
	}
}

func TestListArenaTargetsQueryTypesEveryBoundParameter(t *testing.T) {
	for parameter := 1; parameter <= 8; parameter++ {
		placeholder := fmt.Sprintf("$%d", parameter)
		if !strings.Contains(listArenaTargetsQuery, placeholder) {
			t.Fatalf("target query does not reference bound parameter %s", placeholder)
		}
	}
	if !strings.Contains(listArenaTargetsQuery, "$6::boolean") {
		t.Fatal("nearby parameter must have an explicit PostgreSQL type")
	}
	if !strings.Contains(listArenaTargetsQuery, "$2::bigint") {
		t.Fatal("battle level parameter must have an explicit PostgreSQL type")
	}
	if !strings.Contains(listArenaTargetsQuery, "$7::text") {
		t.Fatal("format filter parameter must have an explicit PostgreSQL type")
	}
	if !strings.Contains(listArenaTargetsQuery, "$8::text") {
		t.Fatal("eligibility policy parameter must have an explicit PostgreSQL type")
	}
	query := strings.ToLower(listArenaTargetsQuery)
	if !strings.Contains(query, "d.selection_jsonb->>'formatid'=$7::text") {
		t.Fatal("format filter must use the authored selection format id")
	}
	if !strings.Contains(query, "d.selection_jsonb->>'eligibilitypolicyversion'=$8::text") {
		t.Fatal("target query must exclude defenses published under older eligibility policies")
	}
	if strings.Contains(query, "specializationenabled") || strings.Contains(query, "coalesce(nullif(d.selection_jsonb->>'formatid'") {
		t.Fatal("target query must not preserve retired selection fields or format fallbacks")
	}
}

func TestListArenaMoreTargetsQueryUsesStableBoundedSampleBeforeFormatFilter(t *testing.T) {
	for parameter := 1; parameter <= 7; parameter++ {
		placeholder := fmt.Sprintf("$%d", parameter)
		if !strings.Contains(listArenaMoreTargetsQuery, placeholder) {
			t.Fatalf("more-target query does not reference bound parameter %s", placeholder)
		}
	}
	query := strings.ToLower(listArenaMoreTargetsQuery)
	for _, required := range []string{
		"with forward as",
		"wrapped as",
		"sampled_ids as",
		"union all",
		"md5(",
		"$1::bigint",
		"$2::bigint",
		"limit $3::int",
		"$4::text",
		"coalesce(ps.rating,$5::bigint)",
		"$6::text='all'",
		"d.selection_jsonb->>'formatid'=$6::text",
		"d.selection_jsonb->>'eligibilitypolicyversion'=$7::text",
		"from sampled_ids sample",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("more-target query missing %q", required)
		}
	}
	if strings.Contains(query, "random()") || strings.Contains(query, " offset ") {
		t.Fatal("more-target query must be stable and non-pageable")
	}
	sampledJoin := strings.Index(query, "from sampled_ids sample")
	formatFilter := strings.Index(query, "d.selection_jsonb->>'formatid'=$6::text")
	if sampledJoin < 0 || formatFilter < sampledJoin {
		t.Fatal("format filtering must happen after the daily sample is bounded")
	}
	maxQuery := strings.ToLower(maxArenaActiveDefenseUserIDQuery)
	if !strings.Contains(maxQuery, "max(user_id)") || !strings.Contains(maxQuery, "active=true") {
		t.Fatal("more-target pivot must use the indexed active-player upper bound")
	}
}

func TestArenaMoreTargetPivotIsStableAndBounded(t *testing.T) {
	const maxUserID = int64(10_000)
	first := arenaMoreTargetPivot(42, "2026-08-30", maxUserID)
	second := arenaMoreTargetPivot(42, "2026-08-30", maxUserID)
	if first != second {
		t.Fatalf("daily pivot changed: %d then %d", first, second)
	}
	if first < 1 || first > maxUserID {
		t.Fatalf("daily pivot=%d outside 1..%d", first, maxUserID)
	}
	for _, invalid := range []struct {
		userID    int64
		seed      string
		maxUserID int64
	}{
		{userID: 0, seed: "2026-08-30", maxUserID: maxUserID},
		{userID: 42, seed: "", maxUserID: maxUserID},
		{userID: 42, seed: "2026-08-30", maxUserID: 0},
	} {
		if got := arenaMoreTargetPivot(invalid.userID, invalid.seed, invalid.maxUserID); got != 0 {
			t.Fatalf("invalid pivot input returned %d", got)
		}
	}
}

func TestArenaMoreTargetsSchemaHasActiveUserIndex(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260830_arena_more_targets.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, required := range []string{
			"idx_arena_defenses_active_user",
			"on arena_defenses(user_id)",
			"where active = true",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
	}
	down, err := os.ReadFile("../../schema/migrations/20260830_arena_more_targets_down.sql")
	if err != nil {
		t.Fatalf("read more-target down migration: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(down)), "drop index if exists idx_arena_defenses_active_user") {
		t.Fatal("more-target down migration does not remove its index")
	}
}

func TestArenaFormatSchemaFreezesChallengeSnapshotWithoutFormatColumns(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_arena.sql",
		"../../schema/migrations/20260830_arena_formats.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, required := range []string{
			"format_snapshot_jsonb",
			"idx_arena_defenses_active_format",
			"selection_jsonb->>'formatid'",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing arena format persistence contract %q", path, required)
			}
		}
		for _, forbidden := range []string{"format_id text", "format_kind text", "format_effect_id"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s hard-codes an arena format column %q; new DSL formats must remain data-only", path, forbidden)
			}
		}
		if strings.Contains(text, "coalesce(nullif(selection_jsonb->>'formatid'") {
			t.Fatalf("%s must index the required format id directly", path)
		}
	}
}

func TestArenaProductionPathsDoNotSeedOrFabricateTargets(t *testing.T) {
	migrations, err := filepath.Glob("../../schema/migrations/*arena*.sql")
	if err != nil {
		t.Fatalf("list arena migrations: %v", err)
	}
	paths := append([]string{"../../schema/init.sql"}, migrations...)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, forbidden := range []string{
			"insert into arena_player_states",
			"insert into arena_defenses",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must not seed production arena data with %q", path, forbidden)
			}
		}
	}

	normalizedTargets := strings.ToLower(listArenaTargetsQuery)
	for _, required := range []string{"from arena_defenses", "join users", "d.active=true"} {
		if !strings.Contains(normalizedTargets, required) {
			t.Fatalf("target query must read real active players; missing %q", required)
		}
	}
	for _, forbidden := range []string{" values ", " union all "} {
		if strings.Contains(normalizedTargets, forbidden) {
			t.Fatalf("target query must not fabricate fallback rows with %q", forbidden)
		}
	}
}

func TestArenaLeaderboardQueryIsBoundedTypedAndRealPlayerOnly(t *testing.T) {
	normalized := strings.ToLower(listArenaLeaderboardQuery)
	for _, required := range []string{
		"from arena_player_states",
		"join users",
		"left join arena_defenses",
		"coalesce(d.active,false)",
		"ps.attack_wins+ps.attack_losses+ps.defense_wins+ps.defense_losses>0",
		"row_number() over",
		"limit $1::int offset $2::int",
		"user_id=$3::bigint",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("leaderboard query missing production contract %q", required)
		}
	}
}

func TestArenaLeaderboardSchemaHasMatchingIndex(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_arena.sql",
		"../../schema/migrations/20260830_arena_leaderboard.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(strings.ToLower(string(raw)), "idx_arena_player_states_leaderboard") {
			t.Fatalf("%s missing arena leaderboard index", path)
		}
	}
}

func TestAdminResetArenaLeaderboardQueryResetsOnlyLeaderboardState(t *testing.T) {
	query := strings.ToLower(adminResetArenaLeaderboardQuery)
	for _, required := range []string{
		"from arena_player_states",
		"order by user_id asc",
		"for update",
		"update arena_player_states",
		"rating=$1::bigint",
		"attack_wins=0",
		"attack_losses=0",
		"defense_wins=0",
		"defense_losses=0",
		"updated_at=now()",
		"count(*)::bigint",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("arena leaderboard reset query missing %q", required)
		}
	}
	for _, preserved := range []string{
		"tokens=",
		"challenge_charges=",
		"challenge_charge_refreshed_at=",
		"arena_defenses",
		"arena_challenges",
	} {
		if strings.Contains(query, preserved) {
			t.Fatalf("arena leaderboard reset query must preserve %q state", preserved)
		}
	}
}

func TestAdminResetArenaLeaderboardRejectsNegativeDefaultRating(t *testing.T) {
	_, err := (&Store{}).AdminResetArenaLeaderboard(context.Background(), -1)
	if !errors.Is(err, ErrBadQuantity) {
		t.Fatalf("error=%v, want %v", err, ErrBadQuantity)
	}
}

func TestArenaDefenseInitializationCandidatesOnlyIncludeNeverPublishedRecentUsers(t *testing.T) {
	query := strings.ToLower(listArenaDefenseInitializationCandidatesQuery)
	for _, required := range []string{
		"from player_presence",
		"last_seen_at >= $1",
		"p.user_id > $2::bigint",
		"not exists",
		"from arena_defenses",
		"d.user_id = p.user_id",
		"order by p.user_id asc",
		"limit $3::int",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("arena initialization candidate query missing %q", required)
		}
	}
	if strings.Contains(query, "d.active") {
		t.Fatal("inactive defenses must also exclude a player from initialization")
	}
}

func TestCreateArenaDefenseIfAbsentNeverUpdatesExistingDefense(t *testing.T) {
	query := strings.ToLower(createArenaDefenseIfAbsentQuery)
	for _, required := range []string{
		"insert into arena_defenses",
		"on conflict(user_id) do nothing",
		"returning user_id",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("create-if-absent query missing %q", required)
		}
	}
	if strings.Contains(query, "do update") {
		t.Fatal("arena initialization must never overwrite an existing defense")
	}
}

func TestArenaOpponentCooldownQueryUsesBoundedIndexedHistory(t *testing.T) {
	opponentQuery := strings.ToLower(arenaChallengeOpponentCooldownsQuery)
	for _, required := range []string{
		"attacker_id=$1::bigint",
		"defender_id=any($2::bigint[])",
		"created_at>clock.server_time-($3::bigint*interval '1 second')",
		"group by clock.server_time,c.defender_id",
	} {
		if !strings.Contains(opponentQuery, required) {
			t.Fatalf("opponent cooldown query missing %q", required)
		}
	}

	for _, path := range []string{"../../schema/init.sql", "../../schema/migrations/20260829_arena.sql"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, indexName := range []string{"idx_arena_challenges_attacker_history", "idx_arena_challenges_pair_cooldown"} {
			if !strings.Contains(text, indexName) {
				t.Fatalf("%s missing cooldown index %q", path, indexName)
			}
		}
		for _, columnName := range []string{"challenge_charges", "challenge_charge_refreshed_at"} {
			if !strings.Contains(text, columnName) {
				t.Fatalf("%s missing challenge charge column %q", path, columnName)
			}
		}
	}
}

func TestArenaChallengeChargeLazyRecoveryPreservesPartialProgress(t *testing.T) {
	policy := ArenaChallengeChargePolicy{Capacity: 10, Recovery: 10 * time.Minute}
	serverTime := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	refreshedAt := serverTime.Add(-26 * time.Minute)
	charges, nextAnchor, err := normalizeArenaChallengeCharges(2, refreshedAt, serverTime, policy)
	if err != nil {
		t.Fatal(err)
	}
	if charges != 4 {
		t.Fatalf("charges=%d, want 4", charges)
	}
	if want := refreshedAt.Add(20 * time.Minute); !nextAnchor.Equal(want) {
		t.Fatalf("refreshedAt=%s, want %s", nextAnchor, want)
	}
	state := arenaChallengeChargeState(charges, nextAnchor, serverTime, policy)
	if state.NextRecoveryAt == nil || !state.NextRecoveryAt.Equal(serverTime.Add(4*time.Minute)) {
		t.Fatalf("next recovery=%v, want %s", state.NextRecoveryAt, serverTime.Add(4*time.Minute))
	}
}

func TestArenaChallengeChargeLazyRecoveryCapsWithoutBanking(t *testing.T) {
	policy := ArenaChallengeChargePolicy{Capacity: 10, Recovery: 10 * time.Minute}
	serverTime := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	charges, refreshedAt, err := normalizeArenaChallengeCharges(9, serverTime.Add(-25*time.Minute), serverTime, policy)
	if err != nil {
		t.Fatal(err)
	}
	if charges != 10 || !refreshedAt.Equal(serverTime) {
		t.Fatalf("normalized charges=%d refreshedAt=%s, want 10 at %s", charges, refreshedAt, serverTime)
	}
	if state := arenaChallengeChargeState(charges, refreshedAt, serverTime, policy); state.NextRecoveryAt != nil {
		t.Fatalf("full charge state should not have next recovery: %v", state.NextRecoveryAt)
	}
}

func TestArenaChallengeChargeConsumeStartsRecoveryAtFullCapacity(t *testing.T) {
	policy := ArenaChallengeChargePolicy{Capacity: 10, Recovery: 10 * time.Minute}
	serverTime := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	charges, refreshedAt, state, err := consumeArenaChallengeCharge(10, serverTime.Add(-time.Hour), serverTime, policy)
	if err != nil {
		t.Fatal(err)
	}
	if charges != 9 || !refreshedAt.Equal(serverTime) {
		t.Fatalf("consumed charges=%d refreshedAt=%s, want 9 at %s", charges, refreshedAt, serverTime)
	}
	if state.NextRecoveryAt == nil || !state.NextRecoveryAt.Equal(serverTime.Add(10*time.Minute)) {
		t.Fatalf("next recovery=%v, want %s", state.NextRecoveryAt, serverTime.Add(10*time.Minute))
	}
	if _, _, _, err := consumeArenaChallengeCharge(0, serverTime, serverTime, policy); !errors.Is(err, ErrArenaChallengeChargesExhausted) {
		t.Fatalf("empty charge consume error=%v, want %v", err, ErrArenaChallengeChargesExhausted)
	}
}

func TestArenaChallengeChargeRefundPreservesPartialRecovery(t *testing.T) {
	policy := ArenaChallengeChargePolicy{Capacity: 10, Recovery: 10 * time.Minute}
	serverTime := time.Date(2026, time.August, 30, 12, 5, 0, 0, time.UTC)
	anchor := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	charges, refreshedAt, err := refundArenaChallengeCharge(4, anchor, serverTime, policy)
	if err != nil {
		t.Fatal(err)
	}
	if charges != 5 || !refreshedAt.Equal(anchor) {
		t.Fatalf("refunded charges=%d refreshedAt=%s, want 5 with anchor %s", charges, refreshedAt, anchor)
	}
}

func TestArenaChallengeCooldownInputNormalization(t *testing.T) {
	if got := arenaChallengeCooldownSeconds(1500 * time.Millisecond); got != 2 {
		t.Fatalf("cooldown seconds=%d, want 2", got)
	}
	if got := arenaChallengeCooldownSeconds(0); got != 0 {
		t.Fatalf("zero cooldown seconds=%d, want 0", got)
	}
	opponents, err := normalizeArenaCooldownOpponentIDs([]int64{12, 8, 12})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(opponents) != "[12 8]" {
		t.Fatalf("normalized opponents=%v, want [12 8]", opponents)
	}
	if _, err := normalizeArenaCooldownOpponentIDs([]int64{0}); err == nil {
		t.Fatal("expected invalid opponent id error")
	}
}

func TestArenaSettlementRatingDeltasRequireRecommendedEligibility(t *testing.T) {
	policy := ArenaSettlementPolicy{
		AttackerWinRating: 20, AttackerLossRatingPenalty: 5,
		DefenderWinRating: 10, DefenderLossRatingPenalty: 10,
	}
	for _, outcome := range []string{ArenaChallengeAttackerWin, ArenaChallengeDefenderWin} {
		attacker, defender := arenaSettlementRatingDeltas(outcome, false, policy)
		if attacker != 0 || defender != 0 {
			t.Fatalf("unrated %s deltas=%d/%d, want 0/0", outcome, attacker, defender)
		}
	}
	attacker, defender := arenaSettlementRatingDeltas(ArenaChallengeAttackerWin, true, policy)
	if attacker != 20 || defender != -10 {
		t.Fatalf("rated attacker win deltas=%d/%d, want 20/-10", attacker, defender)
	}
	attacker, defender = arenaSettlementRatingDeltas(ArenaChallengeDefenderWin, true, policy)
	if attacker != -5 || defender != 10 {
		t.Fatalf("rated defender win deltas=%d/%d, want -5/10", attacker, defender)
	}
}

func TestArenaRatedRecommendationSchemaFreezesEligibility(t *testing.T) {
	for _, path := range []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260829_arena.sql",
		"../../schema/migrations/20260830_arena_rated_recommendations.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(strings.ToLower(string(raw)), "rating_eligible") {
			t.Fatalf("%s must persist arena rating eligibility", path)
		}
	}
}
