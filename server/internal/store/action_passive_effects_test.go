package store

import (
	"encoding/json"
	"math"
	"mysecondwebgame/server/internal/operationeffect"
	"reflect"
	"testing"
	"time"
)

func TestFishingPassiveCastFreezesAndExpires(t *testing.T) {
	at := time.Unix(1000, 0)
	src := operationeffect.PassiveSource{ID: "desk", Version: "v1", Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainFishingCast, RateBPS: 1500}}}
	session := passiveTestSession(t, []operationeffect.PassiveWindow{{From: at, Until: at.Add(time.Minute), Source: src}})
	runtime := fishingExecutorRuntime{ActorType: ActionActorTypeKitty, ActorID: 7, effectSession: session}
	if err := freezeFishingCastEffects(&runtime, at); err != nil {
		t.Fatal(err)
	}
	if runtime.CastPassive == nil {
		t.Fatal("first cat cast missed source")
	}
	raw, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	var restored fishingExecutorRuntime
	if err = json.Unmarshal(raw, &restored); err != nil || restored.CastPassive == nil {
		t.Fatalf("cast checkpoint lost source: %v", err)
	}
	if err = freezeFishingCastEffects(&runtime, at.Add(time.Minute)); err != nil || runtime.CastPassive != nil {
		t.Fatalf("next cast borrowed expired service: %v", err)
	}
	gain, err := session.Passive().Share(*restored.CastPassive, map[string]float64{"fishing": 10})
	if err != nil || gain["fishing"] != 1.5 {
		t.Fatalf("frozen cast changed after expiration: %v %v", gain, err)
	}
	runtime.ActorType = ActionActorTypePlayer
	if err = freezeFishingCastEffects(&runtime, at); err != nil || runtime.CastPassive != nil {
		t.Fatalf("player cast received cat effect: %v", err)
	}
}

func passiveTestSession(t testing.TB, windows []operationeffect.PassiveWindow) *operationeffect.Session {
	t.Helper()
	s, err := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	s.SetPassive(&operationeffect.PassiveSession{Windows: windows})
	return s
}
func TestPassiveActionBatchMatchesEachOperation(t *testing.T) {
	at := time.Unix(1000, 0)
	source := operationeffect.PassiveSource{ID: "desk", Version: "v1", Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1500}}}
	windows := []operationeffect.PassiveWindow{{From: at, Until: at.Add(73 * time.Second), Source: source}, {From: at.Add(90 * time.Second), Until: at.Add(100 * time.Second), Source: source}}
	for _, count := range []int{1, 2, 36, 37, 38, 45, 46, 1000} {
		s := passiveTestSession(t, windows)
		item := QueueRuntimeItem{ActorType: ActionActorTypeKitty, ActorID: 7, EffectSession: s, PayloadRaw: []byte(`{"runtime":{}}`), PreserveScheduledTimeline: true}
		var err error
		item.PayloadRaw, err = freezeQueuePassive(item, item.PayloadRaw, at, operationeffect.DomainLifeAction)
		if err != nil {
			t.Fatal(err)
		}
		timing := queueRunTiming{StartAt: at, CurrentDuration: 3 * time.Second, NextDuration: 2 * time.Second, HasCurrentRun: true}
		exp := map[string]float64{"woodcutting": 0.0123}
		gain, err := shareActionExperience(item, exp, timing, at.Add(time.Hour), count)
		if err != nil {
			t.Fatal(err)
		}
		sequential := passiveTestSession(t, windows).Passive()
		total := 0.0
		for i := 0; i < count; i++ {
			start := at
			if i > 0 {
				start = operationEffectSubsequentStartAt(item, timing, at.Add(time.Hour), i)
			}
			g, e := sequential.Share(sequential.Snapshot(passiveOperation(item, operationeffect.DomainLifeAction, start)), exp)
			if e != nil {
				t.Fatal(e)
			}
			total += g["woodcutting"]
		}
		if math.Abs(gain["woodcutting"]-total) > 1e-10 || !reflect.DeepEqual(sequential.Remainders, s.Passive().Remainders) {
			t.Fatalf("count=%d, batch=%v sequential=%v", count, gain, total)
		}
	}
}

func TestPassiveQueueSnapshotSurvivesEnvelopeUpdates(t *testing.T) {
	at := time.Now()
	source := operationeffect.PassiveSource{ID: "desk", Version: "v1", Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1500}}}
	item := QueueRuntimeItem{ActorType: ActionActorTypeKitty, EffectSession: passiveTestSession(t, []operationeffect.PassiveWindow{{From: at, Until: at.Add(time.Hour), Source: source}})}
	raw, err := freezeQueuePassive(item, []byte(`{"runtime":{}}`), at, operationeffect.DomainLifeAction)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = setQueueRuntimePayload(raw, genericExecutorRuntime{DurationMs: 2000})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := queuePassiveSnapshot(raw)
	if err != nil || len(frozen.Sources) != 1 {
		t.Fatalf("snapshot lost: %s %v", raw, err)
	}
}

func TestPassiveOnlySessionKeepsFoodBatchFastPath(t *testing.T) {
	item := QueueRuntimeItem{EffectSession: passiveTestSession(t, nil), PayloadRaw: []byte(`{"executorId":"generic","runtime":{}}`)}
	rows, _, err := prepareGenericOperationEffectSnapshots(item, queueRunTiming{}, time.Now(), 100000, 99999, 100000, false)
	if err != nil || len(rows) != 0 {
		t.Fatalf("passive-only session allocated per-operation food snapshots: %d %v", len(rows), err)
	}
}

func TestPassiveProgressSettlementBoundaries(t *testing.T) {
	at := time.Unix(1000, 0)
	early, boundary, later := at.Add(time.Second), at.Add(2*time.Second), at.Add(3*time.Second)
	for _, tc := range []struct {
		name, actor   string
		end, owner    *time.Time
		through, want time.Time
		limit         int
	}{
		{"player restarts", ActionActorTypePlayer, &early, nil, later, later, 1},
		{"cat before owner", ActionActorTypeKitty, &early, &boundary, later, boundary.Add(-time.Nanosecond), 100},
		{"owner equals cutoff", ActionActorTypeKitty, &early, &boundary, boundary, boundary.Add(-time.Nanosecond), 100},
		{"tied completion", ActionActorTypeKitty, &boundary, &boundary, later, later, 1},
		{"unknown phase", ActionActorTypeKitty, nil, &boundary, later, later, 1},
		{"future owner", ActionActorTypeKitty, &early, &later, boundary, boundary, 100},
		{"cat only", ActionActorTypeKitty, &early, nil, later, later, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			through, limit := passiveProgressSettlementLimit(tc.actor, tc.end, tc.owner, tc.through, 100)
			if !through.Equal(tc.want) || limit != tc.limit {
				t.Fatalf("got %v/%d want %v/%d", through, limit, tc.want, tc.limit)
			}
		})
	}
}
