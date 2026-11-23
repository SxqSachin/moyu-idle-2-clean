package httpapi

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestBattleSkillExpWorkerUsesAtomicLeaseFencedApply(t *testing.T) {
	if battleSkillExpClaimBatchSize != 1 {
		t.Fatalf("skill exp claim batch=%d, want 1 to avoid locally expired claims", battleSkillExpClaimBatchSize)
	}
	raw, err := os.ReadFile("battle_skill_exp_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) processBattleSkillExpJob")
	if start < 0 {
		t.Fatal("processBattleSkillExpJob missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	if !strings.Contains(body, "ApplyBattleSkillExpJob(ctx, job.ID, workerID, job.Attempts") {
		t.Fatal("skill worker must apply rewards and mark done through the atomic job method")
	}
	decodeAt := strings.Index(body, "battleSkillExpJobRewards(job.Payload)")
	quarantineAt := strings.Index(body, "quarantineBattleSkillExpJob")
	applyAt := strings.Index(body, "ApplyBattleSkillExpJob")
	if decodeAt < 0 || quarantineAt <= decodeAt || applyAt <= quarantineAt {
		t.Fatal("invalid skill payload must be quarantined before the atomic reward transaction")
	}
	for _, forbidden := range []string{"ApplyBattleSkillExpRewardsV2", "MarkBattleSkillExpJobDone"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("skill worker retained non-atomic call %q", forbidden)
		}
	}
}

func TestBattleSkillExpJobRewardsFailClosedOnEmptyOrInvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "missing"},
		{name: "null", payload: []byte(`null`)},
		{name: "empty", payload: []byte(`[]`)},
		{name: "malformed", payload: []byte(`{`)},
		{name: "zero exp", payload: []byte(`[{"userId":1,"actorType":"player","skillId":"slash","exp":0}]`)},
		{name: "missing skill", payload: []byte(`[{"userId":1,"actorType":"player","exp":2}]`)},
		{name: "missing player", payload: []byte(`[{"actorType":"player","skillId":"slash","exp":2}]`)},
		{name: "missing kitty", payload: []byte(`[{"actorType":"kitty","skillId":"slash","exp":2}]`)},
		{name: "unknown actor", payload: []byte(`[{"userId":1,"actorType":"enemy","skillId":"slash","exp":2}]`)},
		{name: "single row exceeds battle budget", payload: []byte(`[{"userId":7,"skillId":"slash","exp":9223372036854775807}]`)},
		{name: "player aggregate overflow", payload: []byte(`[{"userId":7,"skillId":"slash","exp":9223372036854775807},{"userId":7,"skillId":"slash","exp":1}]`)},
		{name: "kitty aggregate overflow", payload: []byte(`[{"actorType":"kitty","actorId":9,"skillId":"pounce","exp":9223372036854775807},{"actorType":"kitty","actorId":9,"skillId":"pounce","exp":1}]`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := battleSkillExpJobRewards(tc.payload)
			if !errors.Is(err, errBattleSkillExpPayloadInvalid) {
				t.Fatalf("error=%v, want invalid skill payload", err)
			}
		})
	}
}

func TestBattleSkillExpJobRewardsPreservePlayerAndKittyGains(t *testing.T) {
	payload := []byte(`[
		{"userId":7,"actorType":"player","skillId":" slash ","exp":2},
		{"userId":7,"skillId":"slash","exp":3},
		{"actorType":"kitty","actorId":9,"skillId":"pounce","exp":4}
	]`)
	rows, rewards, kittyRewards, err := battleSkillExpJobRewards(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rewards[7]["slash"] != 5 || kittyRewards[9]["pounce"] != 4 {
		t.Fatalf("decoded rows/rewards changed: rows=%d players=%v kitties=%v", len(rows), rewards, kittyRewards)
	}
}

func TestBattleSkillExpPayloadPreservesCalculatedRewards(t *testing.T) {
	rows := []battleSkillExpRewardRow{
		{UserID: 7, ActorType: "player", SkillID: "slash", Exp: 6},
		{ActorType: "kitty", ActorID: 9, SkillID: "pounce", Exp: 4},
	}
	payload, err := battleSkillExpJobPayload(rows)
	if err != nil {
		t.Fatal(err)
	}
	decoded, rewards, kittyRewards, err := battleSkillExpJobRewards(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(rows) || rewards[7]["slash"] != 6 || kittyRewards[9]["pounce"] != 4 {
		t.Fatalf("serialized skill rewards changed: decoded=%v players=%v kitties=%v", decoded, rewards, kittyRewards)
	}
	emptyPayload, err := battleSkillExpJobPayload(nil)
	if err != nil || len(emptyPayload) != 0 {
		t.Fatalf("empty skill rewards payload=%q err=%v, want no job payload", emptyPayload, err)
	}
}

func TestBattleFinalizeFreezesSkillExpBeforeRunFencedTransaction(t *testing.T) {
	raw, err := os.ReadFile("battle_runtime_finalize.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) settleBattleRunPlayback")
	if start < 0 {
		t.Fatal("settleBattleRunPlayback missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	payloadAt := strings.Index(body, "skillExpJobPayload, err = battleSkillExpJobPayload(skillExpRows)")
	finalizeAt := strings.Index(body, "CompleteBattleRunAndMaybeContinue(ctx")
	if payloadAt < 0 || finalizeAt <= payloadAt {
		t.Fatal("skill experience payload must be calculated and serialized before the core finalization transaction")
	}
	for _, expected := range []string{
		"CompleteBattleRunAndMaybeContinue(ctx, detail.Room.ID, run.ID, rewards, itemBagUpdates, attrExpJobPayload, skillExpJobPayload",
		"HandleBattleRunFailure(ctx, detail.Room.ID, run.ID, rewards, itemBagUpdates, attrExpJobPayload, skillExpJobPayload",
		"FinishBattleRoom(ctx, detail.Room.ID, run.ID, outcome, rewards, itemBagUpdates, attrExpJobPayload, skillExpJobPayload",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("run-fenced skill experience settlement missing %q", expected)
		}
	}
	if strings.Contains(body, "s.enqueueBattleSkillExpJob(ctx") {
		t.Fatal("new finalizations must not enqueue skill experience after the core transaction")
	}
	if !strings.Contains(source, "reconcileBattleSettlementPostFinalize") {
		t.Fatal("terminal repair path must remain for older or already-finalized runs")
	}
}
