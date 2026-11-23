package httpapi

import "strings"

type battleCompileQueuePressure struct {
	Slots         int
	Pending       int
	FirstPending  int
	FirstCapacity int
	InFlight      int
}

type battleCompilePressureLevel string

const (
	battleCompilePressureNormal    battleCompilePressureLevel = "normal"
	battleCompilePressurePressured battleCompilePressureLevel = "pressured"
	battleCompilePressureBacklog   battleCompilePressureLevel = "backlogged"
)

const (
	battleCompileSourceReadyDispatch       = "ready_dispatch"
	battleCompileSourceEnsureRun           = "ensure_run"
	battleCompileSourcePrepare             = "prepare"
	battleCompileSourceRecoveryAutoReenter = "recovery_auto_reenter"
	battleCompileSourceTickRetry           = "tick_retry"
	battleCompileSourceV2Dispatch          = "v2_compile_dispatch"
)

func normalizeBattleCompileSource(source string) string {
	return strings.TrimSpace(source)
}

func isBattleCompileFirstSnapshotSource(source string) bool {
	switch normalizeBattleCompileSource(source) {
	case battleCompileSourceReadyDispatch,
		battleCompileSourceEnsureRun,
		battleCompileSourcePrepare,
		battleCompileSourceRecoveryAutoReenter:
		return true
	default:
		return false
	}
}

func (pressure battleCompileQueuePressure) normalizedSlots() int {
	if pressure.Slots <= 0 {
		return 1
	}
	return pressure.Slots
}

func (pressure battleCompileQueuePressure) warnPendingThreshold() int {
	warnPending := pressure.normalizedSlots() / 2
	if warnPending < 1 {
		warnPending = 1
	}
	return warnPending
}

func (pressure battleCompileQueuePressure) totalPending() int {
	return maxInt(0, pressure.Pending) + maxInt(0, pressure.FirstPending)
}

func (pressure battleCompileQueuePressure) level() battleCompilePressureLevel {
	slots := pressure.normalizedSlots()
	if pressure.totalPending() >= slots || pressure.InFlight >= slots*2 {
		return battleCompilePressureBacklog
	}
	if pressure.totalPending() >= pressure.warnPendingThreshold() || pressure.InFlight >= slots {
		return battleCompilePressurePressured
	}
	return battleCompilePressureNormal
}

func (pressure battleCompileQueuePressure) isBacklogged() bool {
	return pressure.level() == battleCompilePressureBacklog
}

func (pressure battleCompileQueuePressure) isPressured() bool {
	return pressure.level() != battleCompilePressureNormal
}

func (pressure battleCompileQueuePressure) allowsKickoff(source string) bool {
	source = normalizeBattleCompileSource(source)
	if isBattleCompileFirstSnapshotSource(source) {
		return true
	}
	if pressure.isBacklogged() {
		return source == battleCompileSourceV2Dispatch && pressure.Pending < pressure.normalizedSlots()
	}
	if !pressure.isPressured() {
		return true
	}
	switch source {
	case battleCompileSourceTickRetry:
		return false
	default:
		return true
	}
}

func (pressure battleCompileQueuePressure) kickoffAdmissionBudget() int {
	remaining := pressure.normalizedSlots() - maxInt(0, pressure.Pending)
	if remaining <= 0 {
		return 0
	}
	if pressure.isPressured() {
		return 1
	}
	if remaining > 4 {
		remaining = 4
	}
	return remaining
}

func (pressure battleCompileQueuePressure) readyDispatchStartBudget() int {
	firstCapacity := pressure.FirstCapacity
	if firstCapacity <= 0 {
		firstCapacity = pressure.normalizedSlots()
	}
	firstHeadroom := firstCapacity - maxInt(0, pressure.FirstPending)
	if firstHeadroom <= 0 {
		return 0
	}
	if pressure.isBacklogged() {
		// Creating a V2 run only moves it into the durable compiling phase.
		// Keep this path alive so the inter-battle timer cannot be starved by
		// an unrelated in-memory compile backlog, but do not grow it quickly.
		return 1
	}
	remaining := pressure.normalizedSlots() - pressure.totalPending()
	if remaining < 1 {
		remaining = 1
	}
	if remaining > firstHeadroom {
		remaining = firstHeadroom
	}
	if remaining > 4 {
		remaining = 4
	}
	return remaining
}

func (pressure battleCompileQueuePressure) applyClaimLimit(claimLimit int) int {
	slots := pressure.normalizedSlots()
	compileDrivenCap := maxInt(4, slots*2)
	if claimLimit > compileDrivenCap {
		claimLimit = compileDrivenCap
	}
	if pressure.isBacklogged() {
		claimLimit = maxInt(1, minInt(claimLimit, slots))
	}
	return claimLimit
}

func (s *Server) battleCompileQueuePressure() battleCompileQueuePressure {
	slots := cap(s.battleCompileSlots)
	if slots <= 0 {
		slots = 1
	}
	pending := len(s.battleCompileQueue)
	firstPending := len(s.battleCompileFirstQ)
	s.battleCompileRunMu.Lock()
	inFlight := len(s.battleCompileRuns)
	s.battleCompileRunMu.Unlock()
	return battleCompileQueuePressure{
		Slots:         slots,
		Pending:       pending,
		FirstPending:  firstPending,
		FirstCapacity: cap(s.battleCompileFirstQ),
		InFlight:      inFlight,
	}
}

func (s *Server) battleCompileQueuePressured() bool {
	return s.battleCompileQueuePressure().isPressured()
}
