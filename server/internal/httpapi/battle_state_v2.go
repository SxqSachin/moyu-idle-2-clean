package httpapi

// Battle room phase constants (v2 state machine).
//
// Phase is a backend-only sub-state that tracks the processing stage
// within status="playing". It is NOT exposed to the API or frontend.
//
// Non-playing rooms always have phase="" (empty).
// Playing rooms cycle through: compiling → playback → settling.
// Playing rooms with no current run have phase=awaiting_run.
const (
	BattleRoomPhaseEmpty       = ""
	BattleRoomPhaseAwaitingRun = "awaiting_run" // waiting for next run creation
	BattleRoomPhaseCompiling   = "compiling"    // compile engine running
	BattleRoomPhasePlayback    = "playback"     // playback worker pushing events
	BattleRoomPhaseSettling    = "settling"     // settlement job in progress
)

// Battle room schema version constants.
//
// V2 is the only dispatched schema. V1 remains a persisted historical value;
// those rooms are not polled and may only move to V2 through an explicit
// user/admin transition.
const (
	BattleRoomSchemaV1 = 1
	BattleRoomSchemaV2 = 2
)

// isBattleRoomPhasePlaying returns true if the phase indicates an active
// battle sub-state (within status=playing).
func isBattleRoomPhasePlaying(phase string) bool {
	return phase == BattleRoomPhaseAwaitingRun ||
		phase == BattleRoomPhaseCompiling ||
		phase == BattleRoomPhasePlayback ||
		phase == BattleRoomPhaseSettling
}
