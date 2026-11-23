package store

import "testing"

func TestActionQueueLaneLimitExceeded(t *testing.T) {
	tests := []struct {
		name         string
		queuedCount  int
		additional   int
		laneLimit    int
		wantExceeded bool
	}{
		{name: "below limit", queuedCount: 0, additional: 1, laneLimit: 20, wantExceeded: false},
		{name: "at limit", queuedCount: 19, additional: 1, laneLimit: 20, wantExceeded: false},
		{name: "over limit", queuedCount: 19, additional: 2, laneLimit: 20, wantExceeded: true},
		{name: "batch over limit", queuedCount: 18, additional: 3, laneLimit: 20, wantExceeded: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := actionQueueLaneLimitExceeded(tc.queuedCount, tc.additional, tc.laneLimit)
			if got != tc.wantExceeded {
				t.Fatalf("actionQueueLaneLimitExceeded(%d, %d, %d) = %v, want %v", tc.queuedCount, tc.additional, tc.laneLimit, got, tc.wantExceeded)
			}
		})
	}
}

func TestActionQueueLaneLimitUsesActorSpecificConfig(t *testing.T) {
	s := &Store{config: RuntimeConfig{ActionQueueLaneLimit: 30, KittyActionQueueLaneLimit: 12}}
	if got := s.actionQueueLaneLimit(ActionActorTypePlayer); got != 30 {
		t.Fatalf("player lane limit=%d, want 30", got)
	}
	if got := s.actionQueueLaneLimit(ActionActorTypeKitty); got != 12 {
		t.Fatalf("kitty lane limit=%d, want 12", got)
	}
}

func TestNormalizeActionBatchInsertMode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default append", input: "", want: ActionBatchInsertModeAppend},
		{name: "trim spaces", input: "  append  ", want: ActionBatchInsertModeAppend},
		{name: "run now like prepend", input: ActionBatchInsertModeRunNowLikePrepend, want: ActionBatchInsertModeRunNowLikePrepend},
		{name: "unknown fallback append", input: "unknown", want: ActionBatchInsertModeAppend},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeActionBatchInsertMode(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeActionBatchInsertMode(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
