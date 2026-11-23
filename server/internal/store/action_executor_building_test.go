package store

import (
	"encoding/json"
	"testing"
)

func TestBuildingValidateAndNormalizeArgs(t *testing.T) {
	executor := newBuildingExecutor(&Store{})
	validRaw, _ := json.Marshal(buildingExecutorArgs{BuildingID: "workshop"})
	if _, err := executor.ValidateAndNormalizeArgs(validRaw); err != nil {
		t.Fatalf("expected valid args, got err=%v", err)
	}
	cases := []string{
		"",
		"{bad",
		`{"buildingId":""}`,
	}
	for _, raw := range cases {
		if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(raw)); err == nil {
			t.Fatalf("expected validation error for raw=%q", raw)
		}
	}
}

func TestBuildingCurveHelpers(t *testing.T) {
	if got := buildingCurveDuration(10, 2, 3); got != 16 {
		t.Fatalf("buildingCurveDuration() = %d, want 16", got)
	}
	if got := buildingCurveDuration(0, 0, 0); got != 1 {
		t.Fatalf("buildingCurveDuration() min clamp = %d, want 1", got)
	}
	costs := buildingCurveMap(
		map[string]int64{"wood": 10, "stone": 0},
		map[string]int64{"wood": 2, "stone": 1},
		3,
	)
	if costs["wood"] != 16 {
		t.Fatalf("wood cost = %d, want 16", costs["wood"])
	}
	if costs["stone"] != 3 {
		t.Fatalf("stone cost = %d, want 3", costs["stone"])
	}
}

func TestBuildingClassifyStartError(t *testing.T) {
	executor := newBuildingExecutor(&Store{})
	tests := []struct {
		err  error
		want string
	}{
		{err: ErrInsufficientItem, want: ActionQueueCancelReasonCostInsufficient},
		{err: ErrStatusRequireNotMet, want: ActionQueueCancelReasonStatusRequirement},
		{err: ErrBuildingRequireNotMet, want: ActionQueueCancelReasonBuildingRequirement},
		{err: ErrBuildingMaxLevel, want: ActionQueueCancelReasonTargetInvalid},
	}
	for _, tc := range tests {
		cancel, reason := executor.ClassifyStartError(tc.err)
		if !cancel {
			t.Fatalf("expected cancel for err=%v", tc.err)
		}
		if reason != tc.want {
			t.Fatalf("reason=%q, want %q", reason, tc.want)
		}
	}
}
