package store

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarshalBattleRunFinalResultReusesCarryWithoutChangingPayload(t *testing.T) {
	carry := map[string]any{
		"status":  BattleRunStatusEnded,
		"players": []any{map[string]any{"unitId": "player:7", "name": "A < B"}},
	}
	finalResult := map[string]any{
		"outcome":    BattleRunStatusEnded,
		"carryState": carry,
	}
	carryRaw, err := json.Marshal(carry)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, err := json.Marshal(finalResult)
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, err := marshalBattleRunFinalResult(finalResult, carry, carryRaw)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("marshaled bytes changed: got=%s want=%s", gotRaw, wantRaw)
	}
	var want, got map[string]any
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(gotRaw, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("marshaled result changed: got=%#v want=%#v", got, want)
	}
	if _, ok := finalResult["carryState"].(map[string]any); !ok {
		t.Fatalf("source final result was mutated: %#v", finalResult)
	}
}

func TestMarshalBattleRunFinalResultPreservesDistinctCarryPayload(t *testing.T) {
	t.Parallel()

	finalCarry := map[string]any{"status": BattleRunStatusEnded, "source": "final-result"}
	finalResult := map[string]any{
		"outcome":    BattleRunStatusEnded,
		"carryState": finalCarry,
	}
	wantRaw, err := json.Marshal(finalResult)
	if err != nil {
		t.Fatal(err)
	}
	for _, carryState := range []map[string]any{
		{"status": BattleRunStatusEnded, "source": "carry-column"},
		nil,
	} {
		carryRaw, marshalErr := json.Marshal(carryState)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		gotRaw, marshalErr := marshalBattleRunFinalResult(finalResult, carryState, carryRaw)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if string(gotRaw) != string(wantRaw) {
			t.Fatalf("distinct final carry was changed: got=%s want=%s", gotRaw, wantRaw)
		}
	}
}

func TestMarshalBattleRunFinalResultDoesNotReuseDeepEqualDistinctMap(t *testing.T) {
	t.Parallel()

	finalCarry := map[string]any{"value": math.Copysign(0, -1)}
	carryState := map[string]any{"value": float64(0)}
	if !reflect.DeepEqual(finalCarry, carryState) {
		t.Fatal("test requires values that are DeepEqual but encode differently")
	}
	finalResult := map[string]any{"carryState": finalCarry}
	wantRaw, err := json.Marshal(finalResult)
	if err != nil {
		t.Fatal(err)
	}
	carryRaw, err := json.Marshal(carryState)
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, err := marshalBattleRunFinalResult(finalResult, carryState, carryRaw)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("distinct final carry was changed: got=%s want=%s", gotRaw, wantRaw)
	}
}

func TestBattlePlaybackLeaseSQLSchedulesNextTickAtLeaseUntil(t *testing.T) {
	tests := []struct {
		name               string
		query              string
		nextTickAssignment string
	}{
		{name: "hold playback lease", query: holdBattleRoomPlaybackLeaseSQL, nextTickAssignment: "next_tick_at = $4"},
		{name: "handoff playback lease", query: handoffBattleRoomPlaybackLeaseSQL, nextTickAssignment: "next_tick_at = $5"},
		{name: "extend playback lease", query: extendBattleRoomPlaybackLeaseSQL, nextTickAssignment: "next_tick_at = $4"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			normalized := strings.ToLower(strings.Join(strings.Fields(tc.query), " "))
			if !strings.Contains(normalized, tc.nextTickAssignment) {
				t.Fatalf("query must schedule next_tick_at at leaseUntil: %s", normalized)
			}
			if strings.Contains(normalized, "next_tick_at = now()") {
				t.Fatalf("query must not keep playback rooms immediately due: %s", normalized)
			}
		})
	}
}

func TestBattlePlaybackHandoffFencesPlaybackOwnership(t *testing.T) {
	normalized := strings.ToLower(strings.Join(strings.Fields(handoffBattleRoomPlaybackLeaseSQL), " "))
	if !strings.Contains(normalized, "set next_tick_at = $5, owner_worker_id = $4") {
		t.Fatalf("handoff must install the unique playback owner token: %s", normalized)
	}
	if !strings.Contains(normalized, "owner_worker_id = $3") {
		t.Fatalf("handoff must verify the dispatcher lease owner: %s", normalized)
	}
	whereIndex := strings.Index(normalized, " where ")
	if whereIndex < 0 {
		t.Fatalf("handoff SQL missing WHERE clause: %s", normalized)
	}
	whereClause := normalized[whereIndex:]
	if strings.Contains(whereClause, "owner_worker_id = $4") {
		t.Fatalf("handoff must not accept the new playback owner token as prior ownership: %s", normalized)
	}
	for _, expected := range []string{"lease_until is null", "lease_until < $7"} {
		if !strings.Contains(whereClause, expected) {
			t.Fatalf("handoff fallback missing %q: %s", expected, normalized)
		}
	}
}

func TestBattlePlaybackLeaseExtensionRequiresExactOwnerToken(t *testing.T) {
	normalized := strings.ToLower(strings.Join(strings.Fields(extendBattleRoomPlaybackLeaseSQL), " "))
	whereIndex := strings.Index(normalized, " where ")
	if whereIndex < 0 {
		t.Fatalf("extend SQL missing WHERE clause: %s", normalized)
	}
	whereClause := normalized[whereIndex:]
	for _, expected := range []string{"current_run_id = $2", "owner_worker_id = $3", "lease_until < $4"} {
		if !strings.Contains(whereClause, expected) {
			t.Fatalf("extend lease fence missing %q: %s", expected, normalized)
		}
	}
	if strings.Contains(whereClause, "owner_worker_id = ''") {
		t.Fatalf("extend must not allow an ownerless or handed-off lease: %s", normalized)
	}
}

func TestBattleRoomClaimSQLKeepsPlayingRoomsWithoutCurrentRunClaimable(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(string(raw)), " "))
	if strings.Contains(normalized, "not (r.status = 'playing' and cr.status = 'settling')") {
		t.Fatal("claim SQL must not use nullable NOT predicate; it filters playing rooms with no current run")
	}
	const expected = "coalesce(cr.status, '') <> 'settling'"
	if count := strings.Count(normalized, expected); count < 3 {
		t.Fatalf("claim and next-tick SQL must explicitly allow NULL current-run status with %q; found %d", expected, count)
	}
}

func TestBattleRunPayloadRawPathDoesNotWriteLargeJSONB(t *testing.T) {
	raw, err := os.ReadFile("store_battle_run_payload.go")
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(string(raw)), " "))
	for _, expected := range []string{
		"compile_input_raw",
		"timeline_raw",
		"final_result_raw",
		"values($1, '{}'::jsonb, '[]'::jsonb, '{}'::jsonb, $2, $3, $4",
		"compile_input_jsonb = '{}'::jsonb",
		"timeline_jsonb = '[]'::jsonb",
		"final_result_jsonb = '{}'::jsonb",
	} {
		if !strings.Contains(normalized, expected) {
			t.Fatalf("raw payload SQL missing %q: %s", expected, normalized)
		}
	}
	if strings.Contains(normalized, "timeline_jsonb = excluded.timeline_jsonb") {
		t.Fatal("raw payload SQL must not keep writing large timeline JSONB")
	}
	if strings.Contains(normalized, "final_result_jsonb = excluded.final_result_jsonb") {
		t.Fatal("raw payload SQL must not keep writing large final result JSONB")
	}
	if strings.Contains(normalized, "string(compileinputraw)") {
		t.Fatal("raw payload SQL must not cast large compile input JSONB")
	}
}

func TestBattleCompileSaveLocksRoomBeforeRunAndPayloads(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) saveBattleRunCompiledIfStatus")
	if start < 0 {
		t.Fatal("saveBattleRunCompiledIfStatus missing")
	}
	body = body[start:]
	if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
		body = body[:next+1]
	}
	roomLockAt := strings.Index(body, "FOR UPDATE OF room")
	runCASAt := strings.Index(body, "UPDATE battle_runs")
	carryAt := strings.Index(body, "upsertBattleRunCarryStateRaw")
	coldAt := strings.Index(body, "upsertBattleRunColdPayloadRaw")
	phaseAt := strings.Index(body, "SET phase = 'playback'")
	if roomLockAt < 0 || runCASAt <= roomLockAt || carryAt <= runCASAt || coldAt <= carryAt || phaseAt <= coldAt {
		t.Fatalf("compile save must fence room -> run -> payloads -> phase in one transaction")
	}
}

func TestStartNextBattleRoomRunUsesBatchedDetailRead(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) StartNextBattleRoomRun")
	if start < 0 {
		t.Fatal("StartNextBattleRoomRun missing")
	}
	body = body[start:]
	if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
		body = body[:next+1]
	}
	if !strings.Contains(body, "getBattleRoomDetailBatched(ctx, roomID)") {
		t.Fatal("next-run dispatch must batch the room/member/unit detail read")
	}
}

func TestBatchedBattleRoomDetailPreservesPositionLocks(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) getBattleRoomDetailBatched")
	if start < 0 {
		t.Fatal("getBattleRoomDetailBatched missing")
	}
	body = body[start:]
	if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
		body = body[:next+1]
	}
	if !strings.Contains(body, "GetBattleRoomDetailHot(ctx, roomID)") ||
		!strings.Contains(body, "applyBattleRoomUnitPositionLocks(ctx, detail.Units)") {
		t.Fatal("batched detail must preserve the full detail position-lock contract")
	}
}

func TestBattleRewardUserLockOnlyCoversSynchronousWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		itemKinds    int
		attrKeys     int
		applyAttrExp bool
		want         bool
	}{
		{name: "no reward", want: false},
		{name: "deferred attr only", attrKeys: 2, want: false},
		{name: "item reward", itemKinds: 1, want: true},
		{name: "synchronous attr reward", attrKeys: 2, applyAttrExp: true, want: true},
		{name: "item plus deferred attr", itemKinds: 1, attrKeys: 2, want: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := battleRewardNeedsUserLock(tc.itemKinds, tc.attrKeys, tc.applyAttrExp); got != tc.want {
				t.Fatalf("battleRewardNeedsUserLock()=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestApplyBattleRoomRewardsDeferredAttrDoesNotTouchTransaction(t *testing.T) {
	t.Parallel()

	tx := &queuePerformanceExecTx{}
	metrics := battleFinalizeStoreMetrics{}
	rewards := map[int64]BattleWaveReward{
		7: {
			AttrExp:      map[string]float64{"strength": 1.25},
			KittyAttrExp: map[int64]map[string]float64{9: {"agility": 2.5}},
		},
	}
	if err := (&Store{}).applyBattleRoomRewardsTx(
		context.Background(),
		tx,
		11,
		22,
		rewards,
		"finish_reward",
		false,
		&metrics,
	); err != nil {
		t.Fatal(err)
	}
	if tx.execCount != 0 {
		t.Fatalf("deferred attribute rewards executed %d synchronous SQL statements", tx.execCount)
	}
	if metrics.rewardUsers != 0 || metrics.rewardAttrKeys != 0 {
		t.Fatalf("deferred attribute rewards polluted synchronous metrics: %#v", metrics)
	}
}

func TestShouldLogBattleRunCompiledSaveStages(t *testing.T) {
	if shouldLogBattleRunCompiledSaveStages(99*time.Millisecond, 99*time.Millisecond, 99*time.Millisecond, 1024*1024-1, 1024*1024-1, 1024*1024-1) {
		t.Fatal("expected sub-threshold save metrics to stay quiet")
	}
	if !shouldLogBattleRunCompiledSaveStages(0, 0, 100*time.Millisecond, 0, 0, 0) {
		t.Fatal("expected total save time at threshold to log")
	}
	if !shouldLogBattleRunCompiledSaveStages(0, 0, 0, 0, 1024*1024, 0) {
		t.Fatal("expected payload at large threshold to log")
	}
}

func TestBattleFailureRecoveryDurationBackoff(t *testing.T) {
	base := 10 * time.Second
	tests := []struct {
		name                string
		consecutiveFailures int
		want                time.Duration
	}{
		{name: "first failure uses base recovery", consecutiveFailures: 1, want: 10 * time.Second},
		{name: "second failure adds fifteen seconds", consecutiveFailures: 2, want: 25 * time.Second},
		{name: "third failure adds two increments", consecutiveFailures: 3, want: 40 * time.Second},
		{name: "max recovery caps at ten minutes", consecutiveFailures: 100, want: 600 * time.Second},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := battleFailureRecoveryDuration(base, tc.consecutiveFailures); got != tc.want {
				t.Fatalf("recovery=%s, want %s", got, tc.want)
			}
		})
	}
}

func TestBattleNextConsecutiveFailuresResetsAfterWaveThree(t *testing.T) {
	tests := []struct {
		name    string
		current int
		wave    int
		want    int
	}{
		{name: "below threshold increments", current: 2, wave: 2, want: 3},
		{name: "wave three still increments", current: 2, wave: 3, want: 3},
		{name: "wave four resets", current: 2, wave: 4, want: 0},
		{name: "later wave resets", current: 99, wave: 30, want: 0},
		{name: "negative current clamps before increment", current: -3, wave: 1, want: 1},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := battleNextConsecutiveFailures(tc.current, tc.wave); got != tc.want {
				t.Fatalf("battleNextConsecutiveFailures(%d, %d)=%d, want %d", tc.current, tc.wave, got, tc.want)
			}
		})
	}
}

func TestPlanBattleRoomUnitRepair(t *testing.T) {
	members := []BattleRoomMember{
		{RoomID: 10, UserID: 101, Position: "vanguard", IsReady: true},
		{RoomID: 10, UserID: 202, Position: "midguard", IsReady: false},
	}
	kittyID := int64(9001)
	units := []BattleRoomUnit{
		{RoomID: 10, OwnerUserID: 101, UnitType: "player", UnitID: 0, Position: "rearguard", IsReady: false},
		{RoomID: 10, OwnerUserID: 101, UnitType: "player", UnitID: 7, Position: "vanguard", IsReady: true},
		{RoomID: 10, OwnerUserID: 303, UnitType: "kitty", UnitID: kittyID, KittyID: &kittyID, Position: "midguard", IsReady: false},
	}

	plan := planBattleRoomUnitRepair(10, members, units)

	if !plan.hasChanges() {
		t.Fatal("expected repair plan to detect dirty battle room units")
	}
	if !reflect.DeepEqual(plan.MemberUserIDs, []int64{101, 202}) {
		t.Fatalf("member ids = %#v, want [101 202]", plan.MemberUserIDs)
	}
	if !reflect.DeepEqual(plan.UnitOwnerUserIDs, []int64{101, 303}) {
		t.Fatalf("unit owner ids = %#v, want [101 303]", plan.UnitOwnerUserIDs)
	}
	if !reflect.DeepEqual(plan.MissingPlayerUnits, []int64{202}) {
		t.Fatalf("missing player units = %#v, want [202]", plan.MissingPlayerUnits)
	}
	if len(plan.PlayerUnitMismatches) != 1 {
		t.Fatalf("player unit mismatches = %#v, want 1 row", plan.PlayerUnitMismatches)
	}
	mismatch := plan.PlayerUnitMismatches[0]
	if mismatch.UserID != 101 || mismatch.FromPos != "rearguard" || mismatch.ToPos != "vanguard" || mismatch.FromReady || !mismatch.ToReady {
		t.Fatalf("unexpected mismatch row: %#v", mismatch)
	}
	if len(plan.OrphanUnits) != 1 || plan.OrphanUnits[0].OwnerUserID != 303 {
		t.Fatalf("orphan units = %#v, want owner 303", plan.OrphanUnits)
	}
	if len(plan.UnexpectedPlayerUnits) != 1 || plan.UnexpectedPlayerUnits[0].UnitID != 7 {
		t.Fatalf("unexpected player units = %#v, want unit_id=7", plan.UnexpectedPlayerUnits)
	}
}

func TestFilterBattleRoomUnitsByMembers(t *testing.T) {
	members := []BattleRoomMember{
		{RoomID: 10, UserID: 101},
	}
	kittyID := int64(77)
	units := []BattleRoomUnit{
		{RoomID: 10, OwnerUserID: 101, UnitType: "player", UnitID: 0},
		{RoomID: 10, OwnerUserID: 202, UnitType: "kitty", UnitID: kittyID, KittyID: &kittyID},
	}

	filtered, orphaned := filterBattleRoomUnitsByMembers(10, members, units)

	if len(filtered) != 1 || filtered[0].OwnerUserID != 101 {
		t.Fatalf("filtered units = %#v, want only owner 101", filtered)
	}
	if len(orphaned) != 1 || orphaned[0].OwnerUserID != 202 {
		t.Fatalf("orphaned units = %#v, want only owner 202", orphaned)
	}
}
