package store

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeBattleRunHistoryRoomIDs(t *testing.T) {
	roomIDs, err := normalizeBattleRunHistoryRoomIDs([]int64{3, 1, 3, 2})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(roomIDs, []int64{1, 2, 3}) {
		t.Fatalf("room ids=%v, want [1 2 3]", roomIDs)
	}
	for _, invalid := range [][]int64{{0}, {-1}, {1, 0}} {
		if _, err := normalizeBattleRunHistoryRoomIDs(invalid); !errors.Is(err, ErrBattleRoomConfigInvalid) {
			t.Fatalf("room ids=%v error=%v, want ErrBattleRoomConfigInvalid", invalid, err)
		}
	}
	tooMany := make([]int64, battleRunHistoryCompactMaxRooms+1)
	for index := range tooMany {
		tooMany[index] = int64(index + 1)
	}
	if _, err := normalizeBattleRunHistoryRoomIDs(tooMany); !errors.Is(err, ErrBattleRoomConfigInvalid) {
		t.Fatalf("oversized room batch error=%v, want ErrBattleRoomConfigInvalid", err)
	}
}

func TestCompactBattleRunHistoriesRejectsUnboundedInputBeforeDB(t *testing.T) {
	store := &Store{}
	deleted, err := store.CompactBattleRunHistories(context.Background(), nil, 10, 64)
	if err != nil || deleted != 0 {
		t.Fatalf("empty cleanup deleted=%d err=%v, want no-op", deleted, err)
	}
	_, err = store.CompactBattleRunHistories(
		context.Background(),
		[]int64{1},
		10,
		battleRunHistoryCompactMaxDeleteBatch+1,
	)
	if !errors.Is(err, ErrBattleRoomConfigInvalid) {
		t.Fatalf("oversized delete batch error=%v, want ErrBattleRoomConfigInvalid", err)
	}
}

func TestCompactBattleRunHistoryDelegatesToBoundedBatch(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) CompactBattleRunHistory")
	if start < 0 {
		t.Fatal("CompactBattleRunHistory missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	for _, required := range []string{
		"s.CompactBattleRunHistories",
		"battleRunHistoryCompactMaxDeleteBatch",
		"deleted < battleRunHistoryCompactMaxDeleteBatch",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("single-room cleanup must delegate and drain bounded batches; missing %q", required)
		}
	}
}

func TestCompactBattleRunHistoriesPartitionsRetentionAndStaysDungeonOnly(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) CompactBattleRunHistories")
	if start < 0 {
		t.Fatal("CompactBattleRunHistories missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
		body = body[:next+1]
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(body)), " ")
	for _, required := range []string{
		"from unnest($1::bigint[])",
		"row_number() over (partition by br.room_id order by br.id desc)",
		"join requested_rooms requested on requested.room_id = br.room_id",
		"left join battle_rooms room on room.current_run_id = ranked.id",
		"where ranked.row_no > $2",
		"order by ranked.id asc limit $3",
		"delete from battle_slow_run_archives",
		"delete from battle_runs",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("batched cleanup missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"arena_challenges",
		"arena_challenge_payloads",
		"combat_trial_runs",
		"combat_trial_run_payloads",
	} {
		if strings.Contains(normalized, forbidden) {
			t.Fatalf("batched dungeon cleanup references another battle domain: %q", forbidden)
		}
	}
}
