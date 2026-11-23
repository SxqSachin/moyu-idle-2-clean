package store

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestBattleRoomV2ClaimAsBattleRoom(t *testing.T) {
	runID := int64(42)
	enteringStartedAt := time.UnixMilli(1_000)
	updatedAt := time.UnixMilli(2_000)
	claim := BattleRoomV2Claim{
		ID:                7,
		DungeonID:         "dungeon-1",
		IsFogMode:         true,
		Status:            BattleRoomStatusEntering,
		CurrentRunID:      &runID,
		EnteringStartedAt: &enteringStartedAt,
		UpdatedAt:         updatedAt,
		SchemaVersion:     2,
	}

	room := claim.AsBattleRoom()
	if room.ID != claim.ID || room.DungeonID != claim.DungeonID || room.IsFogMode != claim.IsFogMode {
		t.Fatalf("claim identity fields not preserved: %#v", room)
	}
	if room.Status != claim.Status || room.CurrentRunID != claim.CurrentRunID {
		t.Fatalf("claim runtime fields not preserved: %#v", room)
	}
	if room.EnteringStartedAt != claim.EnteringStartedAt || !room.UpdatedAt.Equal(updatedAt) || room.SchemaVersion != 2 {
		t.Fatalf("claim scheduling fields not preserved: %#v", room)
	}
}

func TestBattleCompileRetrySQLFencesRunAndPersistsNotBefore(t *testing.T) {
	t.Parallel()

	normalized := normalizeBattleRoomV2SQL(scheduleBattleCompileRetrySQL)
	for _, required := range []string{
		"next_tick_at = $4",
		"lease_until = $4",
		"owner_worker_id = ''",
		"id = $1",
		"current_run_id = $2",
		"schema_version = 2",
		"status = 'playing'",
		"phase = 'compiling'",
		"owner_worker_id = $3",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("compile retry SQL missing %q: %s", required, normalized)
		}
	}
}

func TestBattleRoomV2ClaimQueriesMatchDueIndexes(t *testing.T) {
	queries := map[string]string{
		"state":        claimBattleStateAdvanceRoomsV2SQL,
		"awaiting_run": claimBattleReadyNextRoomsV2SQL,
		"compiling":    claimBattleCompilingRoomsV2SQL,
		"playback":     claimBattlePlaybackRoomsV2SQL,
	}
	for name, query := range queries {
		normalized := normalizeBattleRoomV2SQL(query)
		for _, required := range []string{
			"r.schema_version = 2",
			"r.next_tick_at <= $1",
			"owner_worker_id = $2",
			"order by r.next_tick_at asc, r.updated_at asc, r.id asc",
			"limit $3",
			"for update of r skip locked",
			"lease_until = $4",
			"returning r.id, r.dungeon_id, r.is_fog_mode, r.status, r.current_run_id, r.entering_started_at, r.updated_at, r.schema_version",
		} {
			if !strings.Contains(normalized, required) {
				t.Fatalf("%s claim missing %q: %s", name, required, normalized)
			}
		}
	}
	returning := normalizeBattleRoomV2SQL(battleRoomV2ClaimReturningSQL)
	for _, forbidden := range []string{"owner_id", "room_name", "auto_reenter_on_defeat", "completed_runs", "created_at"} {
		if strings.Contains(returning, forbidden) {
			t.Fatalf("claim returning shape includes unused field %q: %s", forbidden, returning)
		}
	}

	state := normalizeBattleRoomV2SQL(claimBattleStateAdvanceRoomsV2SQL)
	for _, required := range []string{"r.status in ('entering', 'recovering')", "r.phase = ''"} {
		if !strings.Contains(state, required) {
			t.Fatalf("state claim must match partial-index predicate %q: %s", required, state)
		}
	}

	for phase, query := range map[string]string{
		"awaiting_run": claimBattleReadyNextRoomsV2SQL,
		"compiling":    claimBattleCompilingRoomsV2SQL,
		"playback":     claimBattlePlaybackRoomsV2SQL,
	} {
		normalized := normalizeBattleRoomV2SQL(query)
		if !strings.Contains(normalized, "r.status = 'playing'") || !strings.Contains(normalized, "r.phase = '"+phase+"'") {
			t.Fatalf("%s claim must match playing-phase index predicate: %s", phase, normalized)
		}
	}

	compiling := normalizeBattleRoomV2SQL(claimBattleCompilingRoomsV2SQL)
	candidateEnd := strings.Index(compiling, "order by")
	if candidateEnd < 0 {
		t.Fatalf("compiling claim has no candidate ordering: %s", compiling)
	}
	if strings.Contains(compiling[:candidateEnd], "r.owner_worker_id = $2") {
		t.Fatalf("compiling claim must respect its lease even for the same worker: %s", compiling)
	}
}

func TestBattleRoomV2ClaimIndexesExist(t *testing.T) {
	files := []struct {
		path       string
		concurrent bool
	}{
		{path: "../../schema/init.sql"},
		{path: "../../schema/migrations/20260828_battle_room_claim_indexes.sql", concurrent: true},
	}
	for _, file := range files {
		raw, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatalf("read %s: %v", file.path, err)
		}
		normalized := normalizeBattleRoomV2SQL(string(raw))
		for _, required := range []string{
			"idx_battle_rooms_v1_status_due",
			"on battle_rooms(status, next_tick_at, updated_at, id) where schema_version = 1",
			"idx_battle_rooms_v2_playing_phase_due",
			"on battle_rooms(phase, next_tick_at, updated_at, id) where schema_version = 2 and status = 'playing'",
			"idx_battle_rooms_v2_state_due",
			"on battle_rooms(next_tick_at, updated_at, id) where schema_version = 2 and phase = '' and status in ('entering', 'recovering')",
		} {
			if !strings.Contains(normalized, required) {
				t.Fatalf("%s missing %q", file.path, required)
			}
		}
		if file.concurrent {
			if strings.Contains(normalized, "begin;") || strings.Contains(normalized, "commit;") {
				t.Fatalf("concurrent index migration must not use an explicit transaction: %s", file.path)
			}
			if strings.Count(normalized, "create index concurrently if not exists") != 3 {
				t.Fatalf("%s must create all claim indexes concurrently", file.path)
			}
		} else {
			for _, obsolete := range []string{"idx_battle_rooms_tick_claim", "idx_battle_rooms_version_phase_claim"} {
				if strings.Contains(normalized, obsolete) {
					t.Fatalf("fresh schema must not recreate lease-bearing index %s", obsolete)
				}
			}
		}
	}
}

func TestBattleRoomClaimIndexCleanupIsStagedAndConcurrent(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260828_battle_room_claim_indexes_cleanup.sql")
	if err != nil {
		t.Fatal(err)
	}
	normalized := normalizeBattleRoomV2SQL(string(raw))
	for _, required := range []string{
		"from pg_index",
		"and indisready",
		"and indisvalid",
		"drop index concurrently if exists idx_battle_rooms_tick_claim",
		"drop index concurrently if exists idx_battle_rooms_version_phase_claim",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("cleanup migration missing %q", required)
		}
	}
	if strings.Contains(normalized, "begin;") || strings.Contains(normalized, "commit;") {
		t.Fatal("concurrent index cleanup must not use an explicit transaction")
	}
}

func normalizeBattleRoomV2SQL(query string) string {
	return strings.ToLower(strings.Join(strings.Fields(query), " "))
}
