package store

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPlayerUsernameRenameCostUsesRuntimeConfig(t *testing.T) {
	s := &Store{config: RuntimeConfig{
		PlayerUsernameRenameCostItemID:    "  harmonic_embryo  ",
		PlayerUsernameRenameCostItemCount: 3,
	}}
	itemID, count := s.PlayerUsernameRenameCost()
	if itemID != "harmonic_embryo" || count != 3 {
		t.Fatalf("PlayerUsernameRenameCost()=(%q,%d), want (harmonic_embryo,3)", itemID, count)
	}
}

func TestRenamePlayerUsernameRejectsInvalidPreconditionsBeforeTransaction(t *testing.T) {
	configured := &Store{config: RuntimeConfig{
		PlayerUsernameRenameCostItemID:    "harmonic_embryo",
		PlayerUsernameRenameCostItemCount: 1,
	}}
	if _, err := configured.RenamePlayerUsername(t.Context(), 0, "next_name", "CurrentName"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("invalid user id error=%v, want ErrUserNotFound", err)
	}
	if _, err := configured.RenamePlayerUsername(t.Context(), 1, "  ", "CurrentName"); !errors.Is(err, ErrPlayerUsernameInvalid) {
		t.Fatalf("empty username error=%v, want ErrPlayerUsernameInvalid", err)
	}
	if _, err := configured.RenamePlayerUsername(t.Context(), 1, "next_name", "  "); !errors.Is(err, ErrPlayerUsernameInvalid) {
		t.Fatalf("empty expected username error=%v, want ErrPlayerUsernameInvalid", err)
	}

	unconfigured := &Store{}
	if _, err := unconfigured.RenamePlayerUsername(t.Context(), 1, "next_name", "CurrentName"); !errors.Is(err, ErrPlayerUsernameRenameCostInvalid) {
		t.Fatalf("invalid rename cost error=%v, want ErrPlayerUsernameRenameCostInvalid", err)
	}
}

func TestPlayerUsernameExpectedCurrentCAS(t *testing.T) {
	expected, err := normalizeExpectedPlayerUsername("  CurrentName  ")
	if err != nil || expected != "CurrentName" {
		t.Fatalf("normalized expectation=(%v,%v), want CurrentName", expected, err)
	}
	if !playerUsernameMatchesExpected("CurrentName", expected) {
		t.Fatal("matching current username was rejected")
	}
	if playerUsernameMatchesExpected("ChangedElsewhere", expected) {
		t.Fatal("stale expected current username was accepted")
	}
	if _, err := normalizeExpectedPlayerUsername("  "); !errors.Is(err, ErrPlayerUsernameInvalid) {
		t.Fatalf("empty expectation error=%v, want ErrPlayerUsernameInvalid", err)
	}
}

func TestNormalizePlayerUsernameHistoryLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "default zero", limit: 0, want: defaultPlayerUsernameHistoryLimit},
		{name: "default negative", limit: -1, want: defaultPlayerUsernameHistoryLimit},
		{name: "preserve requested", limit: 25, want: 25},
		{name: "preserve maximum", limit: maxPlayerUsernameHistoryLimit, want: maxPlayerUsernameHistoryLimit},
		{name: "clamp over maximum", limit: maxPlayerUsernameHistoryLimit + 1, want: maxPlayerUsernameHistoryLimit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizePlayerUsernameHistoryLimit(tc.limit); got != tc.want {
				t.Fatalf("normalizePlayerUsernameHistoryLimit(%d)=%d, want %d", tc.limit, got, tc.want)
			}
		})
	}
}

func TestMapPlayerUsernameRenameErrorMapsUniqueViolation(t *testing.T) {
	uniqueErr := &pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"}
	if got := mapPlayerUsernameRenameError(fmt.Errorf("update player username: %w", uniqueErr)); !errors.Is(got, ErrDuplicateUser) {
		t.Fatalf("unique violation mapped to %v, want ErrDuplicateUser", got)
	}
	usersFallbackErr := &pgconn.PgError{Code: "23505", TableName: "users"}
	if got := mapPlayerUsernameRenameError(usersFallbackErr); !errors.Is(got, ErrDuplicateUser) {
		t.Fatalf("users unique violation without constraint name mapped to %v, want ErrDuplicateUser", got)
	}

	historyUniqueErr := &pgconn.PgError{Code: "23505", ConstraintName: "player_username_changes_event_key", TableName: "player_username_changes"}
	if got := mapPlayerUsernameRenameError(historyUniqueErr); got != historyUniqueErr {
		t.Fatalf("unrelated unique violation mapped to %v, want original error", got)
	}

	otherErr := errors.New("write failed")
	if got := mapPlayerUsernameRenameError(otherErr); got != otherErr {
		t.Fatalf("non-unique error mapped to %v, want original error", got)
	}
}

func TestPlayerUsernameRenameKeepsStateAndHistoryInOneTransaction(t *testing.T) {
	raw, err := os.ReadFile("store_player_username.go")
	if err != nil {
		t.Fatalf("read store_player_username.go: %v", err)
	}
	source := string(raw)
	needles := []string{
		"s.withTx(ctx",
		"s.lockUserTx(ctx, tx, userID)",
		"FOR UPDATE",
		"!playerUsernameMatchesExpected(currentUsername, expectedCurrentUsername)",
		"return ErrPlayerUsernameConflict",
		"UPDATE users",
		"INSERT INTO player_username_changes",
		"clock_timestamp()",
		"s.LedgerConsumeTemplateTx",
	}
	last := -1
	for _, needle := range needles {
		index := strings.Index(source, needle)
		if index < 0 {
			t.Fatalf("username rename transaction is missing %q", needle)
		}
		if index <= last {
			t.Fatalf("username rename transaction order changed at %q", needle)
		}
		last = index
	}

	ledgerStart := strings.Index(source, "s.LedgerConsumeTemplateTx")
	if ledgerStart < 0 {
		t.Fatal("username rename ledger block not found")
	}
	ledgerEnd := strings.Index(source[ledgerStart:], "}); err != nil")
	if ledgerEnd < 0 {
		t.Fatal("username rename ledger block not found")
	}
	ledgerBlock := source[ledgerStart : ledgerStart+ledgerEnd]
	if !strings.Contains(ledgerBlock, `SubjectType:   "player_username_change"`) ||
		!strings.Contains(ledgerBlock, "strconv.FormatInt(historyID, 10)") {
		t.Fatalf("username rename audit is not linked to its history row: %s", ledgerBlock)
	}
	if strings.Contains(ledgerBlock, "currentUsername") || strings.Contains(ledgerBlock, "nextUsername") ||
		strings.Contains(ledgerBlock, "oldUsername") || strings.Contains(ledgerBlock, "newUsername") {
		t.Fatalf("username rename warehouse audit must not contain username plaintext: %s", ledgerBlock)
	}
}

func TestPlayerUsernameChangeSchemaHasForwardAndRollbackDefinitions(t *testing.T) {
	paths := []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260827_player_username_changes.sql",
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(raw)
		for _, needle := range []string{
			"CREATE TABLE IF NOT EXISTS player_username_changes",
			"old_username TEXT NOT NULL",
			"new_username TEXT NOT NULL",
			"cost_item_id TEXT NOT NULL",
			"cost_item_count BIGINT NOT NULL",
			"idx_player_username_changes_user_created",
		} {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s is missing %q", path, needle)
			}
		}
	}
	down, err := os.ReadFile("../../schema/migrations/20260827_player_username_changes_down.sql")
	if err != nil {
		t.Fatalf("read username history down migration: %v", err)
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS player_username_changes") {
		t.Fatalf("username history down migration does not drop the owned table: %s", down)
	}
}

func TestPlayerUsernameSnapshotsExposeLatestChangeID(t *testing.T) {
	checks := []struct {
		path    string
		needles []string
	}{
		{
			path: "store_public_profiles.go",
			needles: []string{
				"type PublicUsernameSnapshot struct",
				"func (s *Store) GetPublicUsernameSnapshot",
				"SELECT MAX(changes.id)",
				"WHERE changes.user_id = u.id",
			},
		},
		{
			path: "store_user_settings.go",
			needles: []string{
				"UsernameChangeID",
				"SELECT MAX(changes.id)",
				"WHERE changes.user_id = u.id",
			},
		},
	}
	for _, check := range checks {
		raw, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		for _, needle := range check.needles {
			if !strings.Contains(string(raw), needle) {
				t.Fatalf("%s is missing username revision contract %q", check.path, needle)
			}
		}
	}
}

func TestPlayerUsernameProfileSnapshotUsesOneBoundedSQLSnapshot(t *testing.T) {
	raw, err := os.ReadFile("store_player_username.go")
	if err != nil {
		t.Fatalf("read store_player_username.go: %v", err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) GetPlayerUsernameProfileSnapshot")
	end := strings.Index(source, "func (s *Store) ListPlayerUsernameHistory")
	if start < 0 || end <= start {
		t.Fatal("player username profile snapshot function block not found")
	}
	block := source[start:end]
	if count := strings.Count(block, "s.db.QueryRow"); count != 1 {
		t.Fatalf("username revision and history must use one SQL snapshot, found %d queries", count)
	}
	for _, needle := range []string{
		"SELECT MAX(changes.id)",
		"jsonb_agg(",
		"jsonb_build_object(",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT $2",
	} {
		if !strings.Contains(block, needle) {
			t.Fatalf("single username profile snapshot is missing %q", needle)
		}
	}
}
