package store

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
)

func offlineRateSession(t *testing.T, domain operationeffect.Domain, kind string, start time.Time, charges bool) *operationeffect.Session {
	t.Helper()
	registry := operationeffect.NewRegistry()
	if err := ratebonus.Register(registry); err != nil {
		t.Fatal(err)
	}
	def := operationEffectTestDefinition(operationeffect.LifetimeDuration)
	def.Domain, def.Kind = domain, kind
	def.Params = json.RawMessage(`{"bonusRate":0.05}`)
	def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 120}
	def.Stacking.Mode = operationeffect.StackExtendDuration
	until := start.Add(2 * time.Minute)
	grant := operationeffect.Grant{ID: 31, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: start, ValidUntil: &until}
	if charges {
		grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
		grant.Definition.Stacking.Mode = operationeffect.StackAddCharges
		grant.ValidUntil, grant.InitialCharges, grant.RemainingCharges = nil, 1, 1
	}
	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestOfflineEnhancementEffectUsesLogicalStartAndFreezesRun(t *testing.T) {
	start := time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC)
	item := operationEffectQueueItem(t, start, offlineRateSession(t, operationeffect.DomainEnhancement, ratebonus.EnhanceSuccess, start, false))
	item.ActionID = "equipment_enhance"
	item.PreserveScheduledTimeline = true
	timing := queueRunTiming{StartAt: start, CurrentDuration: time.Minute, NextDuration: time.Minute, HasCurrentRun: true}
	for _, completed := range []int{1, 2, 3} {
		snapshot, err := nextEnhanceOperationEffects(item, timing, start.Add(time.Hour), completed)
		if err != nil {
			t.Fatal(err)
		}
		run := enhanceRunContext{BaseSuccessRate: 0.5, SuccessRateBonus: 0.1, SuccessRate: 0.6}
		if err := applyEnhanceOperationEffects(&run, snapshot); err != nil {
			t.Fatal(err)
		}
		want := 0.6
		if completed == 1 {
			want = 0.65
		}
		if math.Abs(run.SuccessRate-want) > 1e-12 {
			t.Fatalf("completed=%d rate=%v want=%v", completed, run.SuccessRate, want)
		}
		payload, err := setEnhanceRunContext(json.RawMessage(`{"actionId":"equipment_enhance","executorId":"equipment_enhance","runtime":{}}`), run)
		if err != nil {
			t.Fatal(err)
		}
		frozen, ok := parseEnhanceRunContext(payload)
		if !ok || frozen.SuccessRate != run.SuccessRate || frozen.SuccessRateBonus != run.SuccessRateBonus {
			t.Fatalf("run changed across chunk persistence: %+v", frozen)
		}
	}
	// Online continuation uses the actual new start rather than borrowing the
	// missed historical window of an expired duration grant.
	item.PreserveScheduledTimeline = false
	if snapshot, err := nextEnhanceOperationEffects(item, timing, start.Add(time.Hour), 1); err != nil || len(snapshot.Rules) != 0 {
		t.Fatalf("online continuation borrowed expired bonus: %+v %v", snapshot, err)
	}
}

func TestOfflineFishingCastEffectsFreezeWithoutLeakingOrDoubleCharging(t *testing.T) {
	start := time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC)
	e := newFishingExecutor(&Store{})
	for _, charges := range []bool{false, true} {
		session := offlineRateSession(t, operationeffect.DomainFishingCast, ratebonus.FishingBite, start, charges)
		runtime := fishingExecutorRuntime{
			TripID: "offline-effect-test", SpotID: "spot", WaterID: "water", ActorType: ActionActorTypePlayer,
			Loadout: FishingLoadoutSnapshot{Method: "lure"},
			Rules: fishingRulesSnapshot{Water: catalog.FishingWater{ID: "water"}, Spot: catalog.FishingSpot{ID: "spot"},
				Species: []catalog.FishingSpecies{{ID: "fish"}}},
			effectSession: session,
		}
		// A failed start must not spend charges or increment the cast ordinal.
		if ok, err := e.beginFishingCastTx(context.Background(), nil, 41, &runtime, start); ok || err != nil || len(session.ChargeReceipts()) != 0 || runtime.Summary.Casts != 0 {
			t.Fatalf("failed cast consumed effect: ok=%t err=%v", ok, err)
		}
		runtime.Supplies.Remaining = FishingSupplyCounts{MainLine: 1, Leader: 1, Lure: 1}
		at := start.Add(time.Minute)
		if ok, err := e.beginFishingCastTx(context.Background(), nil, 41, &runtime, at); !ok || err != nil || runtime.CastEffects == nil {
			t.Fatalf("valid historical cast missing effect: ok=%t err=%v", ok, err)
		}
		if err := freezeFishingCastEffects(&runtime, at); err != nil {
			t.Fatal(err)
		}
		wantReceipts := 0
		if charges {
			wantReceipts = 1
		}
		if len(session.ChargeReceipts()) != wantReceipts {
			t.Fatal("same cast double charged")
		}
		payload, err := marshalFishingEnvelope(FishingTripActionID, runtime)
		if err != nil {
			t.Fatal(err)
		}
		resumed, ok := parseFishingExecutorRuntime(payload)
		if !ok || resumed.CastEffects == nil {
			t.Fatal("frozen cast lost effect at chunk boundary")
		}
		if bonus, err := ratebonus.Resolve(*resumed.CastEffects, ratebonus.FishingBite); err != nil || bonus != 0.05 {
			t.Fatalf("frozen cast bonus=%v err=%v", bonus, err)
		}
		// A completed cast retains its snapshot even after expiry/exhaustion,
		// while the next cast must clear it, including when no grants are loaded.
		for _, nextSession := range []*operationeffect.Session{session, nil} {
			next := resumed
			next.effectSession = nextSession
			if ok, err := e.beginFishingCastTx(context.Background(), nil, 41, &next, start.Add(2*time.Minute)); !ok || err != nil || next.CastEffects != nil {
				t.Fatalf("next cast inherited expired/exhausted bonus: ok=%t err=%v", ok, err)
			}
		}
		if len(session.ChargeReceipts()) != wantReceipts {
			t.Fatal("exhausted/expired cast consumed an extra charge")
		}
	}
}
