package store

import (
	"os"
	"strings"
	"testing"
)

func TestBattleRoomDeletionWaitsForEveryRewardJob(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room_reward_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, required := range []string{
		"battle_settlement_jobs",
		"job.status <> 'done'",
		"battle_attr_exp_jobs",
		"job.applied_at IS NULL",
		"battle_skill_exp_jobs",
		"room.phase = 'reward_cleanup'",
		"FOR UPDATE OF room",
		"battle_room_members",
		"DELETE FROM battle_rooms",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("reward-aware empty room cleanup missing %q", required)
		}
	}
	pendingAt := strings.Index(source, "battleRoomHasPendingRewardWorkTx")
	deleteAt := strings.LastIndex(source, "DELETE FROM battle_rooms")
	if pendingAt < 0 || deleteAt <= pendingAt {
		t.Fatal("empty room deletion must remain after the pending reward check")
	}
	for _, path := range []string{
		"store_battle_settlement.go",
		"store_battle_attr_exp_jobs.go",
		"store_battle_skill_exp_jobs.go",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "deleteEmptyBattleRoomAfterRewardJobsTx(ctx, tx, runID)") {
			t.Fatalf("%s does not atomically clean a retained room when its job finishes", path)
		}
	}
}

func TestLastMemberLeaveRetainsRoomUntilRewardJobsFinish(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) LeaveBattleRoom")
	if start < 0 {
		t.Fatal("LeaveBattleRoom missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
		body = body[:next+1]
	}
	pendingAt := strings.Index(body, "battleRoomHasPendingRewardWorkTx")
	hideAt := strings.Index(body, "SET is_public = FALSE")
	deleteAt := strings.Index(body, "DELETE FROM battle_rooms")
	idleAt := strings.Index(body, "status = 'idle'")
	phaseAt := strings.Index(body, "phase = 'reward_cleanup'")
	if pendingAt < 0 || hideAt <= pendingAt || idleAt <= hideAt || phaseAt <= idleAt || deleteAt <= phaseAt {
		t.Fatal("last-member leave must retain and hide the room before any cascade delete")
	}
	if !strings.Contains(body, "retainedForRewardJobs") || !strings.Contains(body, "ErrBattleRoomNotFound") {
		t.Fatal("retained reward parent must still report successful last-member leave")
	}
}
