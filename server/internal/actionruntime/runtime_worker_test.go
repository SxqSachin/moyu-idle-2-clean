package actionruntime

import (
	"os"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func TestRuntimeKeepsWorkerSpecsIndependentFromCommandCopies(t *testing.T) {
	runtime := NewWorker(nil, catalog.Data{})
	if _, ok := runtime.workerSpecs["equipment_enhance"]; !ok {
		t.Fatal("worker specs missing built-in equipment action")
	}
	if spec, ok := runtime.workerSpecs[store.FishingTripActionID]; !ok {
		t.Fatal("worker specs missing built-in fishing action")
	} else if spec.MainType != "fishing" || spec.SubType != "fishing_trip" || spec.DurationS != 30 {
		t.Fatalf("worker fishing spec = %+v, want stable fishing fallback", spec)
	}

	commandSpecs := runtime.ActionSpecMap()
	delete(commandSpecs, "equipment_enhance")
	delete(commandSpecs, store.FishingTripActionID)
	if _, ok := runtime.workerSpecs["equipment_enhance"]; !ok {
		t.Fatal("command spec mutation changed cached worker specs")
	}
	if _, ok := runtime.workerSpecs[store.FishingTripActionID]; !ok {
		t.Fatal("command spec mutation changed cached fishing worker spec")
	}
}

func TestMapActionStoreErrorFishingTripActive(t *testing.T) {
	got := mapActionStoreError(store.ErrFishingTripActive, "fallback")
	if got == nil || got.Code != "WS_FISHING_TRIP_ACTIVE" {
		t.Fatalf("mapActionStoreError() = %+v, want WS_FISHING_TRIP_ACTIVE", got)
	}
}

func TestMapActionStoreErrorFishingSupplyIncludesItem(t *testing.T) {
	got := mapActionStoreError(&store.FishingSupplyShortageError{
		ItemID:    "reed_float",
		Required:  4,
		Available: 3,
	}, "fallback")
	if got == nil || got.Code != "WS_FISHING_SUPPLY_INSUFFICIENT" || got.InstanceState != "reed_float" {
		t.Fatalf("mapActionStoreError() = %+v, want exact fishing supply context", got)
	}
}

func TestMapActionStoreErrorFishingTackleNotEquippedIncludesItem(t *testing.T) {
	got := mapActionStoreError(&store.FishingTackleNotEquippedError{ItemID: "basic_fishing_rod"}, "fallback")
	if got == nil || got.Code != "WS_FISHING_TACKLE_NOT_EQUIPPED" || got.InstanceState != "basic_fishing_rod" {
		t.Fatalf("mapActionStoreError() = %+v, want exact fishing tackle context", got)
	}
}

func TestMapActionStoreErrorFishingTackleInUse(t *testing.T) {
	got := mapActionStoreError(store.ErrFishingTackleInUse, "fallback")
	if got == nil || got.Code != "WS_FISHING_TACKLE_IN_USE" {
		t.Fatalf("mapActionStoreError() = %+v, want WS_FISHING_TACKLE_IN_USE", got)
	}
}

func TestMapActionStoreErrorFishFoodArguments(t *testing.T) {
	if got := mapActionStoreError(store.ErrFishFoodArgsInvalid, "fallback"); got == nil || got.Code != "WS_ACTION_INVALID_PAYLOAD" {
		t.Fatalf("fish food args mapping = %+v, want WS_ACTION_INVALID_PAYLOAD", got)
	}
	if got := mapActionStoreError(store.ErrFishFoodIngredientsInvalid, "fallback"); got == nil || got.Code != "WS_FISH_FOOD_INGREDIENT_INVALID" {
		t.Fatalf("fish food ingredient mapping = %+v, want WS_FISH_FOOD_INGREDIENT_INVALID", got)
	}
}

func TestGatewayRuntimeDoesNotBuildWorkerSpecCache(t *testing.T) {
	runtime := New(nil, catalog.Data{})
	if runtime.workerSpecs != nil {
		t.Fatal("gateway runtime unexpectedly built worker specs")
	}
}

func TestOfflineFastForwardKeepsCommitSeparateFromProjectionReads(t *testing.T) {
	raw, err := os.ReadFile("runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (rt *Runtime) FastForwardOffline")
	if start < 0 {
		t.Fatal("FastForwardOffline not found")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	if !strings.Contains(body, "AdvanceActionQueueOfflineChunk") {
		t.Fatal("FastForwardOffline must use the mutation-only chunk boundary")
	}
	for _, forbidden := range []string{"ListActionQueue", "GetPlayerState", "GetAttributeExp"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("FastForwardOffline must not perform post-commit projection read %q", forbidden)
		}
	}
}

func TestOfflineSnapshotDoesNotRepeatAttributeProjection(t *testing.T) {
	raw, err := os.ReadFile("runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (rt *Runtime) LoadSnapshot")
	if start < 0 {
		t.Fatal("LoadSnapshot not found")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	if !strings.Contains(body, "ListActionQueue") || !strings.Contains(body, "GetPlayerState") {
		t.Fatal("LoadSnapshot must load the queue and player-state projections")
	}
	if strings.Contains(body, "GetAttributeExp") {
		t.Fatal("LoadSnapshot must not repeat the richer attribute broadcast query")
	}
}

func TestComputeNextDueScheduleMatchesQueueSemantics(t *testing.T) {
	now := time.Now()
	earlier := now.Add(time.Second)
	later := now.Add(2 * time.Second)
	blocked := map[int64]struct{}{9: {}}
	schedule := []store.ActionQueueScheduleItem{
		{ActorType: store.ActionActorTypeKitty, ActorID: 9, Status: "running", RunEndsAt: earlier},
		{ActorType: store.ActionActorTypePlayer, Status: "running", RunEndsAt: later},
		{ActorType: store.ActionActorTypeKitty, ActorID: 10, Status: "pending"},
	}
	queue := []store.ActionQueueItem{
		{ActorType: store.ActionActorTypeKitty, ActorID: 9, Status: "running", RunEndsAt: earlier},
		{ActorType: store.ActionActorTypePlayer, Status: "running", RunEndsAt: later},
		{ActorType: store.ActionActorTypeKitty, ActorID: 10, Status: "pending"},
	}

	want, wantOK := ComputeNextDue(queue, blocked, now)
	got, gotOK := ComputeNextDueSchedule(schedule, blocked, now)
	if gotOK != wantOK || !got.Equal(want) {
		t.Fatalf("schedule due=(%v,%t), full queue due=(%v,%t)", got, gotOK, want, wantOK)
	}
}
