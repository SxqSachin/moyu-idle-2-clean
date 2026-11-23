package store

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestEnhanceValidateAndNormalizeArgs(t *testing.T) {
	executor := newEnhanceExecutor(&Store{})

	validRaw, _ := json.Marshal(enhanceExecutorArgs{
		EquipmentInstanceID: 1,
		MaxAttempts:         10,
		TargetEnhanceLevel:  5,
		UseProtect:          true,
		ProtectStartLevel:   3,
		ProtectItemID:       "protect_scroll",
	})
	if _, err := executor.ValidateAndNormalizeArgs(validRaw); err != nil {
		t.Fatalf("expected valid args, got err=%v", err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "invalid json", raw: "{bad"},
		{name: "negative instance", raw: `{"equipmentInstanceId":-1,"maxAttempts":1,"targetEnhanceLevel":1}`},
		{name: "zero attempts", raw: `{"equipmentInstanceId":1,"maxAttempts":0,"targetEnhanceLevel":1}`},
		{name: "decimal attempts", raw: `{"equipmentInstanceId":1,"maxAttempts":1.2,"targetEnhanceLevel":1}`},
		{name: "too many attempts", raw: `{"equipmentInstanceId":1,"maxAttempts":100001,"targetEnhanceLevel":1}`},
		{name: "invalid target level", raw: `{"equipmentInstanceId":1,"maxAttempts":1,"targetEnhanceLevel":0}`},
		{name: "negative protect level", raw: `{"equipmentInstanceId":1,"maxAttempts":1,"targetEnhanceLevel":1,"protectStartLevel":-1}`},
		{name: "protect missing level", raw: `{"equipmentInstanceId":1,"maxAttempts":1,"targetEnhanceLevel":1,"useProtect":true,"protectStartLevel":0}`},
		{name: "protect missing item id", raw: `{"equipmentInstanceId":1,"maxAttempts":1,"targetEnhanceLevel":1,"useProtect":true,"protectStartLevel":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(tc.raw)); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestEnhanceAttemptRollDeterministicAndBounded(t *testing.T) {
	if enhanceAttemptRoll(1, 2, 3, 0) {
		t.Fatalf("rate 0 should always fail")
	}
	if !enhanceAttemptRoll(1, 2, 3, 1) {
		t.Fatalf("rate 1 should always success")
	}

	for i := 0; i < 20; i++ {
		a := enhanceAttemptRoll(10, 20, i, 0.37)
		b := enhanceAttemptRoll(10, 20, i, 0.37)
		if a != b {
			t.Fatalf("determinism mismatch at i=%d", i)
		}
	}
}

func TestEnhanceAttemptRollBatchConsistency(t *testing.T) {
	const total = 15
	allAtOnce := make([]bool, 0, total)
	for i := 0; i < total; i++ {
		allAtOnce = append(allAtOnce, enhanceAttemptRoll(88, 99, i, 0.42))
	}
	split := make([]bool, 0, total)
	for i := 0; i < 5; i++ {
		split = append(split, enhanceAttemptRoll(88, 99, i, 0.42))
	}
	for i := 5; i < total; i++ {
		split = append(split, enhanceAttemptRoll(88, 99, i, 0.42))
	}

	for i := 0; i < total; i++ {
		if allAtOnce[i] != split[i] {
			t.Fatalf("batch consistency mismatch at i=%d", i)
		}
	}
}

func TestSplitEnhanceSuccessRates(t *testing.T) {
	base, bonus, total := splitEnhanceSuccessRates(0.4, 0.12)
	if base != 0.4 || bonus != 0.12 || total != 0.52 {
		t.Fatalf("unexpected split: base=%v bonus=%v total=%v", base, bonus, total)
	}

	base, bonus, total = splitEnhanceSuccessRates(-1, -0.2)
	if base != 0 || bonus != 0 || total != 0 {
		t.Fatalf("negative values should clamp to zero: base=%v bonus=%v total=%v", base, bonus, total)
	}

	base, bonus, total = splitEnhanceSuccessRates(0.95, 0.2)
	if base != 0.95 || bonus != 0.2 || total != 1 {
		t.Fatalf("total should clamp to one: base=%v bonus=%v total=%v", base, bonus, total)
	}
}

func TestNormalizeEnhanceRunContextBackwardCompatible(t *testing.T) {
	legacy := normalizeEnhanceRunContext(enhanceRunContext{SuccessRate: 0.37})
	if legacy.BaseSuccessRate != 0.37 || legacy.SuccessRateBonus != 0 || legacy.SuccessRate != 0.37 {
		t.Fatalf("legacy run context should use successRate as base: %+v", legacy)
	}
}

func TestParseEnhanceRuntimeAndDuration(t *testing.T) {
	runtimeRaw, err := json.Marshal(enhanceExecutorRuntime{
		TemplateID:          "wood_sword",
		EquipmentInstanceID: 1001,
		DurationMs:          2500,
		MaxLevel:            10,
		TargetEnhanceLevel:  8,
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
	parsed, ok := parseEnhanceExecutorRuntime(payloadRaw)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if parsed.TemplateID != "wood_sword" || parsed.EquipmentInstanceID != 1001 {
		t.Fatalf("unexpected runtime parsed: %+v", parsed)
	}
	if got := parseQueueRuntimeDuration(payloadRaw); got != 2500*time.Millisecond {
		t.Fatalf("parseQueueRuntimeDuration() = %v, want 2500ms", got)
	}

	if _, ok := parseEnhanceExecutorRuntime([]byte(`{"executorId":"generic"}`)); ok {
		t.Fatalf("expected parse failure for wrong executor")
	}
	if got := parseQueueRuntimeDuration([]byte(`{"executorId":"unknown"}`)); got != 0 {
		t.Fatalf("unknown executor should return 0 duration")
	}
}

func TestParseQueueRuntimeDurationEnforcesMinimumForEveryExecutor(t *testing.T) {
	tests := []struct {
		name       string
		executorID string
		runtime    any
	}{
		{name: "generic", executorID: "generic", runtime: genericExecutorRuntime{DurationMs: 1000}},
		{name: "enhance", executorID: "equipment_enhance", runtime: enhanceExecutorRuntime{DurationMs: 1000}},
		{name: "building", executorID: "building_upgrade", runtime: buildingExecutorRuntime{BuildingID: "forge", DurationMs: 1000}},
		{name: "alchemy", executorID: "alchemy.convert", runtime: alchemyRuntime{DurationMs: 1000}},
		{name: "future executor", executorID: "exploration_trip", runtime: map[string]any{"durationMs": 1000, "phase": "traveling"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtimeRaw, err := json.Marshal(test.runtime)
			if err != nil {
				t.Fatalf("marshal runtime: %v", err)
			}
			payloadRaw, err := json.Marshal(queueRuntimeEnvelope{ExecutorID: test.executorID, Runtime: runtimeRaw})
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			if got := parseQueueRuntimeDuration(payloadRaw); got != minimumActionQueueRunDuration {
				t.Fatalf("duration=%v, want %v", got, minimumActionQueueRunDuration)
			}
		})
	}
}

func TestClampEnhanceLevelAndAttemptCosts(t *testing.T) {
	if got := clampEnhanceLevel(-1, 5); got != 0 {
		t.Fatalf("clampEnhanceLevel(-1,5) = %d, want 0", got)
	}
	if got := clampEnhanceLevel(8, 5); got != 5 {
		t.Fatalf("clampEnhanceLevel(8,5) = %d, want 5", got)
	}
	if got := clampEnhanceLevel(3, 5); got != 3 {
		t.Fatalf("clampEnhanceLevel(3,5) = %d, want 3", got)
	}

	costs := attemptCosts(map[string]int64{"stone": 2}, "protect_scroll", true)
	if costs["stone"] != 2 || costs["protect_scroll"] != 1 {
		t.Fatalf("unexpected costs with protect: %+v", costs)
	}
	costs = attemptCosts(map[string]int64{"stone": 2}, "protect_scroll", false)
	if _, ok := costs["protect_scroll"]; ok {
		t.Fatalf("unexpected protect cost when protect disabled: %+v", costs)
	}
}

func TestReserveEnhanceLocalCostsTracksInventoryAndReasons(t *testing.T) {
	owned := map[string]int64{"stone": 5, "protect_scroll": 1}
	ok, reason, err := reserveEnhanceLocalCosts(owned, map[string]int64{"stone": 3, "protect_scroll": 1}, "protect_scroll", true)
	if err != nil || !ok || reason != "" {
		t.Fatalf("reserve first err=%v ok=%v reason=%q", err, ok, reason)
	}
	if owned["stone"] != 2 || owned["protect_scroll"] != 0 {
		t.Fatalf("owned after reserve=%+v", owned)
	}
	ok, reason, err = reserveEnhanceLocalCosts(owned, map[string]int64{"protect_scroll": 1}, "protect_scroll", true)
	if err != nil || ok || reason != EnhanceStopReasonProtectItemInsufficient {
		t.Fatalf("protect shortage err=%v ok=%v reason=%q", err, ok, reason)
	}
	ok, reason, err = reserveEnhanceLocalCosts(owned, map[string]int64{"stone": 3}, "protect_scroll", false)
	if err != nil || ok || reason != EnhanceStopReasonCostInsufficient {
		t.Fatalf("cost shortage err=%v ok=%v reason=%q", err, ok, reason)
	}
}

func TestMergeEnhanceCostsDetectsOverflow(t *testing.T) {
	costs := map[string]int64{"stone": math.MaxInt64}
	if err := addEnhanceCost(costs, "stone", 1); err != ErrInventoryQuantityOverflow {
		t.Fatalf("overflow err=%v, want %v", err, ErrInventoryQuantityOverflow)
	}
	merged := map[string]int64{}
	if err := mergeEnhanceCosts(merged, map[string]int64{"stone": 2}); err != nil {
		t.Fatalf("merge normal err=%v", err)
	}
	if merged["stone"] != 2 {
		t.Fatalf("merged stone=%d, want 2", merged["stone"])
	}
}

func TestEnhanceRunContextRoundTrip(t *testing.T) {
	runtimeRaw, err := json.Marshal(enhanceExecutorRuntime{
		TemplateID:          "wood_sword",
		EquipmentInstanceID: 1001,
		DurationMs:          1000,
		MaxLevel:            10,
		TargetEnhanceLevel:  6,
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
	nextPayload, err := setEnhanceRunContext(payloadRaw, enhanceRunContext{
		BeforeLevel: 3,
		SuccessRate: 0.4,
		UsedProtect: true,
		Costs:       map[string]int64{"stone": 2, "protect_scroll": 1},
		DeltaAttrs:  map[string]float64{"atk": 1.5},
	})
	if err != nil {
		t.Fatalf("setEnhanceRunContext err=%v", err)
	}
	runCtx, ok := parseEnhanceRunContext(nextPayload)
	if !ok {
		t.Fatalf("expected parseEnhanceRunContext success")
	}
	if runCtx.BeforeLevel != 3 || !runCtx.UsedProtect || runCtx.Costs["stone"] != 2 {
		t.Fatalf("unexpected runCtx: %+v", runCtx)
	}
}

func TestEnhanceCancelReasonByStopReason(t *testing.T) {
	cases := []struct {
		stopReason string
		want       string
	}{
		{stopReason: EnhanceStopReasonCostInsufficient, want: ActionQueueCancelReasonCostInsufficient},
		{stopReason: EnhanceStopReasonProtectItemInsufficient, want: ActionQueueCancelReasonProtectInsuff},
		{stopReason: EnhanceStopReasonTargetInvalid, want: ActionQueueCancelReasonTargetInvalid},
		{stopReason: EnhanceStopReasonMilestoneAffixUnavailable, want: ActionQueueCancelReasonTargetInvalid},
		{stopReason: "UNKNOWN", want: ActionQueueCancelReasonCostInsufficient},
	}
	for _, tc := range cases {
		if got := enhanceCancelReasonByStopReason(tc.stopReason); got != tc.want {
			t.Fatalf("enhanceCancelReasonByStopReason(%q)=%q, want %q", tc.stopReason, got, tc.want)
		}
	}
}

func TestFirstEmptyAffixSlotIndex(t *testing.T) {
	if got := firstEmptyAffixSlotIndex(3, map[int]struct{}{0: {}, 2: {}}); got != 1 {
		t.Fatalf("firstEmptyAffixSlotIndex() = %d, want 1", got)
	}
	if got := firstEmptyAffixSlotIndex(2, map[int]struct{}{0: {}, 1: {}}); got != -1 {
		t.Fatalf("firstEmptyAffixSlotIndex() = %d, want -1", got)
	}
}

func TestEnhanceClassifyStartErrorStatusRequirement(t *testing.T) {
	executor := newEnhanceExecutor(&Store{})
	cancelQueue, reason := executor.ClassifyStartError(ErrStatusRequireNotMet)
	if !cancelQueue {
		t.Fatalf("expected status requirement error to cancel queue")
	}
	if reason != ActionQueueCancelReasonStatusRequirement {
		t.Fatalf("reason=%q, want %q", reason, ActionQueueCancelReasonStatusRequirement)
	}
}

func TestEnhanceClassifyStartErrorBuildingRequirement(t *testing.T) {
	executor := newEnhanceExecutor(&Store{})
	cancelQueue, reason := executor.ClassifyStartError(ErrBuildingRequireNotMet)
	if !cancelQueue {
		t.Fatalf("expected building requirement error to cancel queue")
	}
	if reason != ActionQueueCancelReasonBuildingRequirement {
		t.Fatalf("reason=%q, want %q", reason, ActionQueueCancelReasonBuildingRequirement)
	}
}
