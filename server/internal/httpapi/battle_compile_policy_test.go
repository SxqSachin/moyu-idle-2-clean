package httpapi

import "testing"

func TestBattleCompileQueuePressureLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pressure battleCompileQueuePressure
		want     battleCompilePressureLevel
	}{
		{
			name:     "normal pressure",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 0, InFlight: 0},
			want:     battleCompilePressureNormal,
		},
		{
			name:     "pending reaches warning threshold",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2, InFlight: 0},
			want:     battleCompilePressurePressured,
		},
		{
			name:     "inflight reaches warning threshold",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 0, InFlight: 4},
			want:     battleCompilePressurePressured,
		},
		{
			name:     "pending reaches backlog threshold",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 4, InFlight: 0},
			want:     battleCompilePressureBacklog,
		},
		{
			name:     "first snapshot pending counts toward backlog",
			pressure: battleCompileQueuePressure{Slots: 4, FirstPending: 4, InFlight: 0},
			want:     battleCompilePressureBacklog,
		},
		{
			name:     "inflight reaches backlog threshold",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 0, InFlight: 8},
			want:     battleCompilePressureBacklog,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.pressure.level(); got != tc.want {
				t.Fatalf("pressure level=%s, want %s", got, tc.want)
			}
		})
	}
}

func TestBattleCompileQueuePressureKickoffAdmissionBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pressure battleCompileQueuePressure
		want     int
	}{
		{
			name:     "normal keeps dynamic budget",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 1, InFlight: 1},
			want:     3,
		},
		{
			name:     "pressured falls back to one start",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2, InFlight: 0},
			want:     1,
		},
		{
			name:     "backlogged pauses dispatch starts",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 4, InFlight: 1},
			want:     0,
		},
		{
			name:     "first snapshot backlog preserves one regular retry lane",
			pressure: battleCompileQueuePressure{Slots: 4, FirstPending: 4, InFlight: 4},
			want:     1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.pressure.kickoffAdmissionBudget(); got != tc.want {
				t.Fatalf("kickoff admission budget=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestBattleCompileQueuePressureReadyDispatchStartBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pressure battleCompileQueuePressure
		want     int
	}{
		{
			name:     "normal follows compile capacity",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 1, InFlight: 1},
			want:     3,
		},
		{
			name:     "pending pressure uses remaining queue headroom",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2},
			want:     2,
		},
		{
			name:     "fully utilized workers keep ready capacity",
			pressure: battleCompileQueuePressure{Slots: 4, InFlight: 4},
			want:     4,
		},
		{
			name:     "pending backlog still keeps one durable start",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 4},
			want:     1,
		},
		{
			name:     "outstanding backlog still keeps one durable start",
			pressure: battleCompileQueuePressure{Slots: 4, InFlight: 8},
			want:     1,
		},
		{
			name:     "full first snapshot queue stops new run creation",
			pressure: battleCompileQueuePressure{Slots: 4, FirstPending: 4, FirstCapacity: 4},
			want:     0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.pressure.readyDispatchStartBudget(); got != tc.want {
				t.Fatalf("ready dispatch start budget=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestBattleCompileQueuePressureAllowsKickoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pressure battleCompileQueuePressure
		source   string
		want     bool
	}{
		{
			name:     "normal accepts recovery",
			pressure: battleCompileQueuePressure{Slots: 4},
			source:   battleCompileSourceRecoveryAutoReenter,
			want:     true,
		},
		{
			name:     "pressured accepts ready dispatch",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2},
			source:   battleCompileSourceReadyDispatch,
			want:     true,
		},
		{
			name:     "pressured accepts bounded v2 dispatch",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2},
			source:   battleCompileSourceV2Dispatch,
			want:     true,
		},
		{
			name:     "pressured accepts recovery first snapshot",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2},
			source:   battleCompileSourceRecoveryAutoReenter,
			want:     true,
		},
		{
			name:     "pressured defers retry",
			pressure: battleCompileQueuePressure{Slots: 4, InFlight: 4},
			source:   battleCompileSourceTickRetry,
			want:     false,
		},
		{
			name:     "backlogged accepts first snapshot",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 4},
			source:   battleCompileSourcePrepare,
			want:     true,
		},
		{
			name:     "backlogged accepts v2 retry with regular headroom",
			pressure: battleCompileQueuePressure{Slots: 4, FirstPending: 4},
			source:   battleCompileSourceV2Dispatch,
			want:     true,
		},
		{
			name:     "backlogged defers v2 retry when regular queue is full",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 4},
			source:   battleCompileSourceV2Dispatch,
			want:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.pressure.allowsKickoff(tc.source); got != tc.want {
				t.Fatalf("allows kickoff=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestBattleCompileQueuePressureApplyClaimLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pressure  battleCompileQueuePressure
		claimBase int
		want      int
	}{
		{
			name:      "normal caps by compile slots x2",
			pressure:  battleCompileQueuePressure{Slots: 4, Pending: 0, InFlight: 0},
			claimBase: 32,
			want:      8,
		},
		{
			name:      "backlogged further caps to compile slots",
			pressure:  battleCompileQueuePressure{Slots: 4, Pending: 5, InFlight: 0},
			claimBase: 32,
			want:      4,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.pressure.applyClaimLimit(tc.claimBase); got != tc.want {
				t.Fatalf("claim limit=%d, want %d", got, tc.want)
			}
		})
	}
}
