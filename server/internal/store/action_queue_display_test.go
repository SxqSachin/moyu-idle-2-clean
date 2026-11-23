package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildActionQueueDisplayEnhance(t *testing.T) {
	runtimeRaw, err := json.Marshal(enhanceExecutorRuntime{
		TemplateID:          "wood_sword",
		EquipmentInstanceID: 42,
		DurationMs:          9000,
		TargetEnhanceLevel:  8,
		UseProtect:          true,
		ProtectStartLevel:   5,
	})
	if err != nil {
		t.Fatalf("marshal runtime: %v", err)
	}
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "equipment_enhance",
		ExecutorID: "equipment_enhance",
		Runtime:    runtimeRaw,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	display := buildActionQueueDisplay("equipment_enhance", payloadRaw)
	if display == nil || display.Kind != "equipment_enhance" || display.Enhance == nil {
		t.Fatalf("expected enhance display, got %+v", display)
	}
	if display.Enhance.TemplateItemID != "wood_sword" || display.Enhance.TargetEnhanceLevel != 8 {
		t.Fatalf("unexpected enhance display: %+v", display.Enhance)
	}
	if display.EffectiveDurationMs != 9000 {
		t.Fatalf("unexpected effective duration: %+v", display)
	}
}

func TestBuildActionQueueDisplayGeneric(t *testing.T) {
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "work_mining",
		ExecutorID: "generic",
		Runtime: func() json.RawMessage {
			raw, _ := json.Marshal(genericExecutorRuntime{DurationMs: 12000})
			return raw
		}(),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	display := buildActionQueueDisplay("work_mining", payloadRaw)
	if display == nil || display.Kind != "work_mining" {
		t.Fatalf("expected generic display, got %+v", display)
	}
	if display.Enhance != nil {
		t.Fatalf("generic display should not include enhance payload: %+v", display.Enhance)
	}
	if display.EffectiveDurationMs != 12000 {
		t.Fatalf("unexpected effective duration: %+v", display)
	}
}

func TestBuildActionQueueDisplayEnhanceLooseRuntime(t *testing.T) {
	payloadRaw, err := json.Marshal(map[string]any{
		"actionId": "equipment_enhance",
		"runtime": map[string]any{
			"templateId":          "wood_sword",
			"equipmentInstanceId": 7,
			"targetEnhanceLevel":  5,
			"useProtect":          false,
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	display := buildActionQueueDisplay("equipment_enhance", payloadRaw)
	if display == nil || display.Enhance == nil {
		t.Fatalf("expected enhance display from loose runtime, got %+v", display)
	}
	if display.Enhance.EquipmentInstanceID != 7 || display.Enhance.TemplateItemID != "wood_sword" {
		t.Fatalf("unexpected enhance display: %+v", display.Enhance)
	}
}

func TestBuildActionQueueDisplayBuilding(t *testing.T) {
	runtimeRaw, err := json.Marshal(buildingExecutorRuntime{
		BuildingID: "workshop",
		FromLevel:  1,
		ToLevel:    2,
		DurationMs: 90000,
	})
	if err != nil {
		t.Fatalf("marshal runtime: %v", err)
	}
	runCtxRaw, err := json.Marshal(buildingRunContext{
		BuildingID: "workshop",
		FromLevel:  1,
		ToLevel:    2,
		Costs:      map[string]int64{"wood": 15},
	})
	if err != nil {
		t.Fatalf("marshal runCtx: %v", err)
	}
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "building_upgrade",
		ExecutorID: "building_upgrade",
		Runtime:    runtimeRaw,
		RunContext: runCtxRaw,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	display := buildActionQueueDisplay("building_upgrade", payloadRaw)
	if display == nil || display.Building == nil {
		t.Fatalf("expected building display, got %+v", display)
	}
	if display.Building.BuildingID != "workshop" || display.Building.FromLevel != 1 || display.Building.ToLevel != 2 {
		t.Fatalf("unexpected building display: %+v", display.Building)
	}
	if display.EffectiveDurationMs != 90000 {
		t.Fatalf("unexpected effective duration: %+v", display)
	}
}

func TestBuildActionQueueDisplayUsesMinimumDuration(t *testing.T) {
	runtimeRaw, err := json.Marshal(buildingExecutorRuntime{
		BuildingID: "workshop",
		DurationMs: 1000,
	})
	if err != nil {
		t.Fatalf("marshal runtime: %v", err)
	}
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "building_upgrade",
		ExecutorID: "building_upgrade",
		Runtime:    runtimeRaw,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	display := buildActionQueueDisplay("building_upgrade", payloadRaw)
	if display == nil || display.Building == nil {
		t.Fatalf("expected building display, got %+v", display)
	}
	if display.EffectiveDurationMs != int64(minimumActionQueueRunDuration/time.Millisecond) {
		t.Fatalf("effective duration=%d, want %d", display.EffectiveDurationMs, minimumActionQueueRunDuration/time.Millisecond)
	}
	if display.Building.DurationS != int64(minimumActionQueueRunDuration/time.Second) {
		t.Fatalf("building duration=%d, want %d", display.Building.DurationS, minimumActionQueueRunDuration/time.Second)
	}
}

func TestBuildActionQueueDisplayFishingUsesWholeTripSummary(t *testing.T) {
	runtime := fishingExecutorRuntime{
		DurationMS:             33_900,
		TripID:                 "trip-queue-display",
		WaterID:                "sunmere_shallows",
		SpotID:                 "sunmere_lily_cove",
		Strategy:               "balanced",
		SafetyLoadPercent:      82,
		PlannedDurationSeconds: 7_200,
		StartedAtMS:            1_000,
		PlannedEndAtMS:         7_201_000,
		Phase:                  fishingPhaseFighting,
		PhaseEndsAtMS:          40_000,
		CurrentFish:            &FishInstanceData{SpeciesID: "common_carp"},
		CurrentLineLoad:        3.5,
		Loadout: FishingLoadoutSnapshot{
			Name:      "Starter float tackle",
			Method:    "float",
			MaxLoadKG: 5,
		},
		Summary: FishingTripSummary{Landed: 3, Released: 1, TotalWeight: 4.25},
		Supplies: FishingTripSupplies{
			Initial:   FishingSupplyCounts{Hook: 12, Bait: 120},
			Remaining: FishingSupplyCounts{Hook: 11, Bait: 96},
		},
	}
	payloadRaw, err := marshalFishingEnvelope(FishingTripActionID, runtime)
	if err != nil {
		t.Fatalf("marshal fishing runtime: %v", err)
	}

	display := buildActionQueueDisplay(FishingTripActionID, payloadRaw)
	if display == nil || display.Fishing == nil {
		t.Fatalf("expected fishing display, got %+v", display)
	}
	if display.Fishing.PlannedEndAtMS != runtime.PlannedEndAtMS || display.Fishing.PhaseEndsAtMS != runtime.PhaseEndsAtMS {
		t.Fatalf("trip and phase times were not preserved: %+v", display.Fishing)
	}
	if display.Fishing.Landed != 3 || display.Fishing.TotalWeightKG != 4.25 {
		t.Fatalf("trip summary was not preserved: %+v", display.Fishing)
	}
	if display.Fishing.RemainingSupplies.Bait != 96 || display.Fishing.LineState != "tight" {
		t.Fatalf("live fishing state was not preserved: %+v", display.Fishing)
	}
}

func TestBuildActionQueueDisplayFishingPendingUsesPackedPlan(t *testing.T) {
	runtime := fishingExecutorRuntime{
		DurationMS:             1_000,
		TripID:                 "trip-pending",
		WaterID:                "sunmere_shallows",
		SpotID:                 "sunmere_reed_edge",
		Strategy:               "conservative",
		PlannedDurationSeconds: 1_800,
		Phase:                  "pending",
		Loadout:                FishingLoadoutSnapshot{Name: "Reed set", Method: "float"},
		Supplies:               FishingTripSupplies{Initial: FishingSupplyCounts{Hook: 8, Bait: 60}},
	}
	payloadRaw, err := marshalFishingEnvelope(FishingTripActionID, runtime)
	if err != nil {
		t.Fatalf("marshal fishing runtime: %v", err)
	}

	display := buildActionQueueDisplay(FishingTripActionID, payloadRaw)
	if display == nil || display.Fishing == nil {
		t.Fatalf("expected fishing display, got %+v", display)
	}
	if display.Fishing.RemainingSupplies.Bait != 60 || display.Fishing.PlannedDurationSeconds != 1_800 {
		t.Fatalf("pending fishing plan was not preserved: %+v", display.Fishing)
	}
}
