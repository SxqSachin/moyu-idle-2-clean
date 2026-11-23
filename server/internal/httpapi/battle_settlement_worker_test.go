package httpapi

import (
	"errors"
	"os"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestBattleRunReadyForSettlement(t *testing.T) {
	run := store.BattleRun{
		State: map[string]any{
			"stage":         "playback_finished",
			"nextEventIdx":  12,
			"totalEvents":   12,
			"lastPushedSeq": int64(99),
			"finalSeq":      int64(99),
		},
	}
	if !battleRunReadyForSettlement(run) {
		t.Fatalf("expected playback-finished run to be settlement-ready")
	}
	run.State["stage"] = "playback_streaming"
	if battleRunReadyForSettlement(run) {
		t.Fatalf("expected non-finished playback stage to be rejected")
	}
}

func TestBattleSettlementClaimsOneJobPerSerialWorker(t *testing.T) {
	if battleSettlementClaimBatchSize != 1 {
		t.Fatalf("settlement claim batch=%d, want 1 to keep every claim inside its lease", battleSettlementClaimBatchSize)
	}
}

func TestBattleSettlementTerminalOutcomeConflict(t *testing.T) {
	tests := []struct {
		name        string
		runStatus   string
		finalResult map[string]any
		want        bool
	}{
		{name: "matching ended", runStatus: store.BattleRunStatusEnded, finalResult: map[string]any{"outcome": store.BattleRunStatusEnded}},
		{name: "matching failed", runStatus: store.BattleRunStatusFailed, finalResult: map[string]any{"outcome": store.BattleRunStatusFailed}},
		{name: "missing legacy outcome", runStatus: store.BattleRunStatusFailed, finalResult: map[string]any{}, want: false},
		{name: "blank legacy outcome", runStatus: store.BattleRunStatusFailed, finalResult: map[string]any{"outcome": "  "}, want: false},
		{name: "invalid explicit outcome", runStatus: store.BattleRunStatusEnded, finalResult: map[string]any{"outcome": "victory"}, want: true},
		{name: "non-string explicit outcome", runStatus: store.BattleRunStatusEnded, finalResult: map[string]any{"outcome": 1}, want: true},
		{name: "conflicting terminal outcome", runStatus: store.BattleRunStatusFailed, finalResult: map[string]any{"outcome": store.BattleRunStatusEnded}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := battleSettlementTerminalOutcomeConflict(tc.runStatus, tc.finalResult); got != tc.want {
				t.Fatalf("conflict=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestBattleSettlementCorePayloadFailsClosedWithoutRewardOrCarryData(t *testing.T) {
	validPlayers := map[string]any{"players": []map[string]any{{
		"unitId": "player:7", "userId": int64(7), "unitType": "player", "skills": []any{}, "battleItems": []any{},
	}}}
	member7 := map[int64]struct{}{7: {}}
	tests := []struct {
		name        string
		outcome     string
		finalResult map[string]any
		allowed     map[int64]struct{}
		wantErr     bool
	}{
		{
			name:        "ended complete empty rewards",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": validPlayers},
			allowed:     member7,
		},
		{
			name:        "failed accepts legacy omitted reward maps",
			outcome:     store.BattleRunStatusFailed,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{"userId": int64(7)}}, "finalState": validPlayers},
			allowed:     member7,
		},
		{
			name:        "explicit null skills remain a compatible empty set",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7, "skills": nil}}}},
			allowed:     member7,
		},
		{
			name:        "interrupted does not require reward payload",
			outcome:     store.BattleRunStatusInterrupted,
			finalResult: map[string]any{},
		},
		{
			name:        "missing reward plan",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "null reward plan",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": nil, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "reward plan is not an array",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": map[string]any{}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "invalid reward user",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{}}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "duplicate reward user",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{"userId": 7}, map[string]any{"userId": 7}}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "missing player carry state",
			outcome:     store.BattleRunStatusFailed,
			finalResult: map[string]any{"rewardPlan": []any{}},
			wantErr:     true,
		},
		{
			name:        "carry state has no owned player",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "summon:1"}}}},
			wantErr:     true,
		},
		{
			name:        "owned player skills missing",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7}}}},
			wantErr:     true,
		},
		{
			name:        "owned player skill malformed",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7, "skills": []any{map[string]any{"id": "slash", "level": 0}}}}}},
			wantErr:     true,
		},
		{
			name:    "skill cast missing from explicit empty snapshot",
			outcome: store.BattleRunStatusEnded,
			finalResult: map[string]any{
				"rewardPlan": []any{},
				"battleStats": map[string]any{"skillCastsByUnit": map[string]any{
					"player:7": map[string]any{"slash": 1},
				}},
				"carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7, "skills": nil}}},
			},
			wantErr: true,
		},
		{
			name:        "battle item negative quantity",
			outcome:     store.BattleRunStatusFailed,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7, "skills": nil, "battleItems": []any{map[string]any{"itemId": "potion", "quantity": -1}}}}}},
			wantErr:     true,
		},
		{
			name:        "duplicate battle item for owner",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7, "skills": nil, "battleItems": []any{map[string]any{"itemId": "potion", "quantity": 1}, map[string]any{"itemId": "potion", "quantity": 1}}}}}},
			wantErr:     true,
		},
		{
			name:        "reward user missing from snapshots",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{"userId": 8}}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "negative attribute experience",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{"userId": 7, "attrExp": map[string]any{"battle": -1}}}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "zero item reward",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{"userId": 7, "itemRewards": map[string]any{"slime": 0}}}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "kitty attribute reward owner mismatch",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{map[string]any{"userId": 7, "kittyAttrExp": map[string]any{"9": map[string]any{"battle": 1}}}}, "carryState": validPlayers},
			wantErr:     true,
		},
		{
			name:        "player identity mismatch",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": map[string]any{"players": []map[string]any{{"unitId": "player:8", "userId": 7, "skills": nil}}}},
			wantErr:     true,
		},
		{
			name:        "payload user is not a current member",
			outcome:     store.BattleRunStatusEnded,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": validPlayers},
			allowed:     map[int64]struct{}{8: {}},
			wantErr:     true,
		},
		{
			name:        "current member is missing from payload",
			outcome:     store.BattleRunStatusFailed,
			finalResult: map[string]any{"rewardPlan": []any{}, "carryState": validPlayers},
			allowed:     map[int64]struct{}{7: {}, 8: {}},
			wantErr:     true,
		},
		{
			name:    "formal kitty casts require kitty snapshot",
			outcome: store.BattleRunStatusEnded,
			finalResult: map[string]any{
				"rewardPlan": []any{},
				"battleStats": map[string]any{"skillCastsByUnit": map[string]any{
					"kitty:9": map[string]any{"scratch": 1},
				}},
				"carryState": validPlayers,
			},
			allowed: member7,
			wantErr: true,
		},
		{
			name:    "skill casts cannot exceed compile budget",
			outcome: store.BattleRunStatusEnded,
			finalResult: map[string]any{
				"rewardPlan":         []any{},
				"compileBudgetSteps": int64(10),
				"battleStats": map[string]any{"skillCastsByUnit": map[string]any{
					"player:7": map[string]any{"slash": 11},
				}},
				"carryState": map[string]any{"players": []map[string]any{{"unitId": "player:7", "userId": 7, "skills": []any{map[string]any{"id": "slash", "level": 1}}}}},
			},
			allowed: member7,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := battleSettlementCorePayloadError(tc.finalResult, tc.outcome, tc.allowed)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, wantErr=%t", err, tc.wantErr)
			}
		})
	}
}

func TestBattleSettlementExplicitOutcomeFailsClosed(t *testing.T) {
	tests := []struct {
		name        string
		finalResult map[string]any
		want        string
		ok          bool
	}{
		{name: "nil result", finalResult: nil},
		{name: "empty result", finalResult: map[string]any{}},
		{name: "non-string outcome", finalResult: map[string]any{"outcome": 1}},
		{name: "invalid outcome", finalResult: map[string]any{"outcome": "victory"}},
		{name: "ended", finalResult: map[string]any{"outcome": " ended "}, want: store.BattleRunStatusEnded, ok: true},
		{name: "failed", finalResult: map[string]any{"outcome": store.BattleRunStatusFailed}, want: store.BattleRunStatusFailed, ok: true},
		{name: "interrupted", finalResult: map[string]any{"outcome": store.BattleRunStatusInterrupted}, want: store.BattleRunStatusInterrupted, ok: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := battleSettlementExplicitOutcome(tc.finalResult)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("outcome=(%q,%t), want (%q,%t)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestBattleSettlementWorkerQuarantinesDetachedRunBeforeFinalization(t *testing.T) {
	raw, err := os.ReadFile("battle_settlement_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) processBattleSettlementJob")
	if start < 0 {
		t.Fatal("processBattleSettlementJob missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	runRoomFenceAt := strings.Index(body, "run.RoomID != job.RoomID")
	outcomeFenceAt := strings.Index(body, "battleSettlementExplicitOutcome(finalResult)")
	payloadFenceAt := strings.Index(body, "battleSettlementCorePayloadError(finalResult, outcome,")
	currentRunFenceAt := strings.Index(body, "battleSettlementRoomAttachmentError(detail, job)")
	finalizeAt := strings.Index(body, "settleBattleRunPlayback")
	if runRoomFenceAt < 0 || outcomeFenceAt <= runRoomFenceAt || payloadFenceAt <= outcomeFenceAt || currentRunFenceAt <= payloadFenceAt || finalizeAt <= currentRunFenceAt {
		t.Fatal("run identity, explicit outcome, reward payload, and room attachment must be fenced before finalization")
	}
	if strings.Count(body[:finalizeAt], "quarantineBattleSettlementJob") < 3 {
		t.Fatal("run identity, payload, and room attachment mismatches must enter quarantine")
	}
}

func TestTerminalSettlementValidatesDeferredRewardPayloadBeforeRepair(t *testing.T) {
	raw, err := os.ReadFile("battle_settlement_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "case store.BattleRunStatusEnded, store.BattleRunStatusFailed, store.BattleRunStatusInterrupted:")
	if start < 0 {
		t.Fatal("terminal settlement branch missing")
	}
	body := source[start:]
	validateAt := strings.Index(body, "battleSettlementCorePayloadError(finalResult, run.Status, nil)")
	reconcileAt := strings.Index(body, "reconcileBattleSettlementPostFinalize")
	if validateAt < 0 || reconcileAt <= validateAt {
		t.Fatal("terminal settlement must validate deferred reward payload before repair")
	}
}

func TestTerminalSettlementRepairDoesNotDependOnRoomStillExisting(t *testing.T) {
	raw, err := os.ReadFile("battle_settlement_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) processBattleSettlementJob")
	if start < 0 {
		t.Fatal("processBattleSettlementJob missing")
	}
	body := source[start:]
	loadRunAt := strings.Index(body, "loadBattleSettlementRun")
	terminalAt := strings.Index(body, "case store.BattleRunStatusEnded, store.BattleRunStatusFailed, store.BattleRunStatusInterrupted:")
	loadRoomAt := strings.Index(body, "loadBattleRoomDetailFresh")
	if loadRunAt < 0 || terminalAt <= loadRunAt || loadRoomAt <= terminalAt {
		t.Fatal("terminal settlement must repair from the run payload before loading the mutable room")
	}
}

func TestBattleSettlementRoomAttachmentRequiresPlayingCurrentRun(t *testing.T) {
	runID := int64(42)
	job := store.BattleSettlementJob{RoomID: 7, RunID: runID}
	detail := store.BattleRoomDetail{Room: store.BattleRoom{
		ID:           job.RoomID,
		Status:       store.BattleRoomStatusPlaying,
		CurrentRunID: &runID,
	}}
	if err := battleSettlementRoomAttachmentError(detail, job); err != nil {
		t.Fatalf("valid settlement attachment rejected: %v", err)
	}
	detail.Room.Status = store.BattleRoomStatusRecovering
	if err := battleSettlementRoomAttachmentError(detail, job); !errors.Is(err, store.ErrBattleSettlementRunDetached) {
		t.Fatalf("non-playing room error=%v, want detached settlement", err)
	}
	otherRunID := int64(43)
	detail.Room.Status = store.BattleRoomStatusPlaying
	detail.Room.CurrentRunID = &otherRunID
	if err := battleSettlementRoomAttachmentError(detail, job); !errors.Is(err, store.ErrBattleSettlementRunDetached) {
		t.Fatalf("different current run error=%v, want detached settlement", err)
	}
}

func TestTerminalSettlementRepairCoversEveryDeferredReward(t *testing.T) {
	raw, err := os.ReadFile("battle_settlement_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) reconcileBattleSettlementPostFinalize")
	if start < 0 {
		t.Fatal("reconcileBattleSettlementPostFinalize missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	for _, required := range []string{
		"enqueueBattleAttrExpJob",
		"applyBattleKillAchievementEvents",
		"settleBattleSkillExpFromTimeline",
		"enqueueBattleSkillExpJob",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("terminal settlement repair missing %s", required)
		}
	}
}

func TestConcurrentFinalizePathRepairsDeferredRewardsBeforeReturning(t *testing.T) {
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
	concurrentAt := strings.Index(body, "if !finalizedByThisCall {")
	reconcileAt := strings.Index(body, "reconcileBattleSettlementPostFinalize")
	if concurrentAt < 0 || reconcileAt <= concurrentAt {
		t.Fatal("concurrent finalization repair path missing")
	}
	earlyReturnAt := strings.Index(body[reconcileAt:], "return nil")
	if earlyReturnAt < 0 {
		t.Fatal("concurrent finalization must reconcile every deferred reward before returning success")
	}
}
