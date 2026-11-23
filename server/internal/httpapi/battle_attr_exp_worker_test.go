package httpapi

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestBattleAttrExpBacklogLogIsRateLimited(t *testing.T) {
	s := &Server{}
	now := time.Now()
	if !s.shouldLogBattleAttrExpBacklog(now) {
		t.Fatal("first backlog sample must be logged")
	}
	if s.shouldLogBattleAttrExpBacklog(now.Add(battleAttrExpBacklogLogInterval - time.Millisecond)) {
		t.Fatal("backlog sample bypassed the log interval")
	}
	if !s.shouldLogBattleAttrExpBacklog(now.Add(battleAttrExpBacklogLogInterval)) {
		t.Fatal("backlog sample was not logged after the interval")
	}
}

func TestBattleAttrExpRewardRowsRoundTripWithoutItemRewards(t *testing.T) {
	rewards := map[int64]store.BattleWaveReward{
		22: {
			ItemRewards: map[string]int64{"gold_coin": 99},
			KittyAttrExp: map[int64]map[string]float64{
				7: {"agility": 1.25, "": 10},
			},
		},
		11: {
			AttrExp: map[string]float64{"strength": 2.5, "empty": 0},
		},
		0: {AttrExp: map[string]float64{"strength": 100}},
	}

	rows := battleAttrExpRewardRows(rewards)
	if len(rows) != 2 || rows[0].UserID != 11 || rows[1].UserID != 22 {
		t.Fatalf("attribute reward rows are not deterministic: %#v", rows)
	}
	rewards[11].AttrExp["strength"] = 999
	if rows[0].AttrExp["strength"] != 2.5 {
		t.Fatalf("queued payload aliases the settlement reward map: %#v", rows[0].AttrExp)
	}

	roundTrip, err := battleAttrExpRewardsFromRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]store.BattleWaveReward{
		11: {
			AttrExp:      map[string]float64{"strength": 2.5},
			KittyAttrExp: map[int64]map[string]float64{},
		},
		22: {
			AttrExp: map[string]float64{},
			KittyAttrExp: map[int64]map[string]float64{
				7: {"agility": 1.25},
			},
		},
	}
	if !reflect.DeepEqual(roundTrip, want) {
		t.Fatalf("attribute reward round trip=%#v, want %#v", roundTrip, want)
	}
}

func TestBattleAttrExpJobRewardsFailClosedOnEmptyOrInvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "missing"},
		{name: "null", payload: []byte(`null`)},
		{name: "empty", payload: []byte(`[]`)},
		{name: "malformed", payload: []byte(`{`)},
		{name: "object instead of rows", payload: []byte(`{}`)},
		{name: "trailing value", payload: []byte(`[] []`)},
		{name: "unknown field", payload: []byte(`[{
			"userId":1,"attrExp":{"strength":1},"unexpected":true
		}]`)},
		{name: "zero user", payload: []byte(`[{"userId":0,"attrExp":{"strength":1}}]`)},
		{name: "negative user", payload: []byte(`[{"userId":-1,"attrExp":{"strength":1}}]`)},
		{name: "no rewards", payload: []byte(`[{"userId":1}]`)},
		{name: "blank player key", payload: []byte(`[{"userId":1,"attrExp":{" ":1}}]`)},
		{name: "zero player exp", payload: []byte(`[{"userId":1,"attrExp":{"strength":0}}]`)},
		{name: "negative player exp", payload: []byte(`[{"userId":1,"attrExp":{"strength":-1}}]`)},
		{name: "non finite player exp", payload: []byte(`[{"userId":1,"attrExp":{"strength":1e309}}]`)},
		{name: "zero kitty", payload: []byte(`[{"userId":1,"kittyAttrExp":{"0":{"agility":1}}}]`)},
		{name: "negative kitty", payload: []byte(`[{"userId":1,"kittyAttrExp":{"-2":{"agility":1}}}]`)},
		{name: "empty kitty rewards", payload: []byte(`[{"userId":1,"kittyAttrExp":{"2":{}}}]`)},
		{name: "null kitty rewards", payload: []byte(`[{"userId":1,"kittyAttrExp":{"2":null}}]`)},
		{name: "blank kitty key", payload: []byte(`[{"userId":1,"kittyAttrExp":{"2":{" ":1}}}]`)},
		{name: "zero kitty exp", payload: []byte(`[{"userId":1,"kittyAttrExp":{"2":{"agility":0}}}]`)},
		{name: "negative kitty exp", payload: []byte(`[{"userId":1,"kittyAttrExp":{"2":{"agility":-1}}}]`)},
		{name: "duplicate user rows", payload: []byte(`[
			{"userId":1,"attrExp":{"strength":1}},
			{"userId":1,"attrExp":{"agility":2}}
		]`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := battleAttrExpJobRewards(tc.payload)
			if !errors.Is(err, errBattleAttrExpPayloadInvalid) {
				t.Fatalf("error=%v, want invalid attribute payload", err)
			}
		})
	}
}

func TestBattleAttrExpJobRewardsPreservePlayerAndKittyGains(t *testing.T) {
	payload := []byte(`[
		{"userId":11,"attrExp":{"strength":2.5}},
		{"userId":22,"kittyAttrExp":{"7":{"agility":1.25}}}
	]`)
	rows, rewards, err := battleAttrExpJobRewards(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]store.BattleWaveReward{
		11: {
			AttrExp:      map[string]float64{"strength": 2.5},
			KittyAttrExp: map[int64]map[string]float64{},
		},
		22: {
			AttrExp: map[string]float64{},
			KittyAttrExp: map[int64]map[string]float64{
				7: {"agility": 1.25},
			},
		},
	}
	if len(rows) != 2 || !reflect.DeepEqual(rewards, want) {
		t.Fatalf("decoded attribute rewards changed: rows=%d rewards=%#v, want %#v", len(rows), rewards, want)
	}
}

func TestBattleAttrExpWorkerQuarantinesInvalidPayloadBeforeApply(t *testing.T) {
	raw, err := os.ReadFile("battle_attr_exp_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) processBattleAttrExpJob")
	if start < 0 {
		t.Fatal("processBattleAttrExpJob missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	decodeAt := strings.Index(body, "battleAttrExpJobRewards(job.Payload)")
	quarantineAt := strings.Index(body, "quarantineBattleAttrExpJob")
	applyAt := strings.Index(body, "ApplyBattleAttrExpJob")
	if decodeAt < 0 || quarantineAt <= decodeAt || applyAt <= quarantineAt {
		t.Fatal("invalid attribute payload must be quarantined before its reward transaction")
	}
}

func TestBattleFinalizeUsesAtomicAttrExpOutboxWithRollingDeployFallback(t *testing.T) {
	finalizeRaw, err := os.ReadFile("battle_runtime_finalize.go")
	if err != nil {
		t.Fatal(err)
	}
	finalizeBody := string(finalizeRaw)
	payloadAt := strings.Index(finalizeBody, "attrExpJobPayload, err := battleAttrExpJobPayload(rewards)")
	finalizeAt := strings.Index(finalizeBody, "CompleteBattleRunAndMaybeContinue(ctx")
	if payloadAt < 0 || finalizeAt <= payloadAt {
		t.Fatal("attribute reward payload must be frozen before the finalization transaction")
	}
	concurrentAt := strings.Index(finalizeBody, "if !finalizedByThisCall {")
	reconcileAt := strings.Index(finalizeBody, "reconcileBattleSettlementPostFinalize")
	if concurrentAt < 0 || reconcileAt <= concurrentAt {
		t.Fatal("concurrent finalizer must repair every deferred reward before returning")
	}
	if strings.Contains(finalizeBody, "applyBattleAttrRewardsAsync") {
		t.Fatal("battle attribute rewards still have a finite in-process retry path")
	}

	storeRaw, err := os.ReadFile("../store/store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	storeBody := string(storeRaw)
	for _, functionName := range []string{
		"CompleteBattleRunAndMaybeContinue",
		"HandleBattleRunFailure",
		"FinishBattleRoom",
	} {
		start := strings.Index(storeBody, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s missing", functionName)
		}
		body := storeBody[start:]
		if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
			body = body[:next+1]
		}
		if !strings.Contains(body, "enqueueBattleAttrExpJobTx(ctx, tx, finalizedRunID, attrExpJobPayload, now)") {
			t.Fatalf("%s does not enqueue attribute experience in its finalization transaction", functionName)
		}
	}
}

func TestBattleAttrExpWorkerClaimsOnlyOneSerialJob(t *testing.T) {
	t.Parallel()
	if battleAttrExpClaimBatchSize != 1 {
		t.Fatalf("attribute worker claim batch=%d, want 1 so no queued job loses its lease before execution", battleAttrExpClaimBatchSize)
	}
}

func TestBattleAttrExpBusyRetryDelayIsBoundedStableAndJittered(t *testing.T) {
	t.Parallel()

	first := battleAttrExpBusyRetryDelay(store.BattleAttrExpJob{ID: 17, Attempts: 3})
	second := battleAttrExpBusyRetryDelay(store.BattleAttrExpJob{ID: 17, Attempts: 3})
	other := battleAttrExpBusyRetryDelay(store.BattleAttrExpJob{ID: 18, Attempts: 3})
	if first != second {
		t.Fatalf("busy retry jitter is not deterministic: %s vs %s", first, second)
	}
	if first < battleAttrExpBusyRetryMin || first >= battleAttrExpBusyRetryMin+battleAttrExpBusyRetrySpread {
		t.Fatalf(
			"busy retry delay=%s, want [%s, %s)",
			first,
			battleAttrExpBusyRetryMin,
			battleAttrExpBusyRetryMin+battleAttrExpBusyRetrySpread,
		)
	}
	if first == other {
		t.Fatalf("different jobs received identical busy retry delay %s", first)
	}
	for jobID := int64(1); jobID <= 100; jobID++ {
		for attempts := 1; attempts <= 20; attempts++ {
			delay := battleAttrExpBusyRetryDelay(store.BattleAttrExpJob{ID: jobID, Attempts: attempts})
			if delay < battleAttrExpBusyRetryMin || delay >= battleAttrExpBusyRetryMin+battleAttrExpBusyRetrySpread {
				t.Fatalf("job=%d attempts=%d delay=%s is outside the busy retry range", jobID, attempts, delay)
			}
		}
	}
}
