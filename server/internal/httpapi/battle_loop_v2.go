package httpapi

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"mysecondwebgame/server/internal/store"
)

// V2 dispatch loops — one per phase.
//
// Each loop claims rooms with schema_version=2 AND a specific phase.
// They NEVER overlap with compatibility v1 loops (which filter schema_version=1).
// They NEVER overlap with each other (each claims a distinct phase).
//
// v2 loops do NOT use the Redis due queue — DB claim SQL with
// schema_version+phase provides precise filtering without SCAN overhead.

const (
	battleV2StateAdvanceSleep      = 100 * time.Millisecond
	battleV2StateAdvanceBusySleep  = 10 * time.Millisecond
	battleV2ReadyDispatchSleep     = 100 * time.Millisecond
	battleV2ReadyDispatchBusySleep = 10 * time.Millisecond
	battleV2CompileDispatchSleep   = 100 * time.Millisecond
	battleV2CompileBusySleep       = 10 * time.Millisecond
	battleV2PlaybackDispatchSleep  = 100 * time.Millisecond
	battleV2PlaybackBusySleep      = 10 * time.Millisecond
	battleV2ClaimLimit             = 64
	battleV2ReadyDispatchWorkers   = 2
)

// startBattleV2Loops launches all four v2 dispatch loops.
func (s *Server) startBattleV2Loops(workerID string) {
	s.launchBattleTask(func() { s.runBattleStateAdvanceLoopV2(workerID) })
	s.launchBattleTask(func() { s.runBattleReadyDispatchLoopV2(workerID) })
	s.launchBattleTask(func() { s.runBattleCompileDispatchLoopV2(workerID) })
	s.launchBattleTask(func() { s.runBattlePlaybackDispatchLoopV2(workerID) })
	log.Printf("battle v2 dispatch loops started worker=%s", workerID)
}

// --- Loop 1: state advance (entering + recovering) ---

func (s *Server) runBattleStateAdvanceLoopV2(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		now := time.Now()
		sleepFor := s.tickBattleStateAdvanceV2(workerID, now)
		if sleepFor < battleV2StateAdvanceBusySleep {
			sleepFor = battleV2StateAdvanceBusySleep
		}
		timer := time.NewTimer(sleepFor)
		select {
		case <-timer.C:
		case <-s.battleStopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *Server) tickBattleStateAdvanceV2(workerID string, now time.Time) time.Duration {
	claimCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rooms, err := s.deps.Store.ClaimBattleStateAdvanceRoomsV2(
		claimCtx, workerID,
		battleV2ClaimLimit, battleRoomLeaseDuration, now,
	)
	if err != nil {
		log.Printf("battle v2 state-advance claim failed worker=%s err=%v", workerID, err)
		return battleV2StateAdvanceSleep
	}
	if len(rooms) == 0 {
		return battleV2StateAdvanceSleep
	}
	processed := 0
	for _, claim := range rooms {
		room := claim.AsBattleRoom()
		roomCtx, roomCancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		var nextTick time.Time
		var tickErr error
		switch room.Status {
		case store.BattleRoomStatusEntering:
			nextTick, tickErr = s.tryStartPreparingRoom(roomCtx, room, now)
		case store.BattleRoomStatusRecovering:
			nextTick, tickErr = s.tryAdvanceRecoveringRoom(roomCtx, room, now)
		}
		roomCancel()
		if tickErr != nil && !errors.Is(tickErr, store.ErrBattleRoomStateInvalid) {
			log.Printf("battle v2 state-advance tick failed room_id=%d status=%s worker=%s err=%v", room.ID, room.Status, workerID, tickErr)
		}
		if !nextTick.IsZero() {
			_ = s.deps.Store.ScheduleBattleRoomNextTick(context.Background(), room.ID, workerID, nextTick, now.Add(battleRoomLeaseDuration))
		}
		processed++
	}
	if processed > 0 {
		return battleV2StateAdvanceBusySleep
	}
	return battleV2StateAdvanceSleep
}

// --- Loop 2: ready dispatch (awaiting_run → create next run) ---

func (s *Server) runBattleReadyDispatchLoopV2(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		now := time.Now()
		sleepFor := s.tickBattleReadyDispatchV2(workerID, now)
		if sleepFor < battleV2ReadyDispatchBusySleep {
			sleepFor = battleV2ReadyDispatchBusySleep
		}
		timer := time.NewTimer(sleepFor)
		select {
		case <-timer.C:
		case <-s.battleStopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *Server) tickBattleReadyDispatchV2(workerID string, now time.Time) time.Duration {
	compilePressure := s.battleCompileQueuePressure()
	startBudget := compilePressure.readyDispatchStartBudget()
	if startBudget <= 0 {
		return battleV2ReadyDispatchSleep
	}
	claimCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	claimLimit := minInt(battleV2ClaimLimit, startBudget)
	rooms, err := s.deps.Store.ClaimBattleReadyNextRoomsV2(
		claimCtx, workerID, claimLimit, battleRoomLeaseDuration, now,
	)
	if err != nil {
		log.Printf("battle v2 ready-dispatch claim failed worker=%s err=%v", workerID, err)
		return battleV2ReadyDispatchSleep
	}
	if len(rooms) == 0 {
		return battleV2ReadyDispatchSleep
	}
	started := runBattleReadyDispatchWorkers(rooms, func(room store.BattleRoomV2Claim) bool {
		return s.startBattleReadyRoomV2(workerID, room, now)
	})
	return battleReadyDispatchNextSleep(compilePressure, started)
}

func battleReadyDispatchWorkerCount(roomCount int) int {
	if roomCount <= 0 {
		return 0
	}
	return minInt(roomCount, battleV2ReadyDispatchWorkers)
}

func runBattleReadyDispatchWorkers(rooms []store.BattleRoomV2Claim, start func(store.BattleRoomV2Claim) bool) int {
	workerCount := battleReadyDispatchWorkerCount(len(rooms))
	if workerCount == 0 || start == nil {
		return 0
	}
	shards := make([][]store.BattleRoomV2Claim, workerCount)
	for index, room := range rooms {
		shards[index%workerCount] = append(shards[index%workerCount], room)
	}
	// Keep the claim/admission budget unchanged; two lanes only remove
	// cross-room head-of-line blocking inside the already bounded batch.
	startedByShard := make([]int, workerCount)
	processShard := func(index int) {
		for _, room := range shards[index] {
			if start(room) {
				startedByShard[index]++
			}
		}
	}
	var wg sync.WaitGroup
	for shardIndex := 1; shardIndex < workerCount; shardIndex++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			processShard(index)
		}(shardIndex)
	}
	processShard(0)
	wg.Wait()
	started := 0
	for _, shardStarted := range startedByShard {
		started += shardStarted
	}
	return started
}

func (s *Server) startBattleReadyRoomV2(workerID string, room store.BattleRoomV2Claim, now time.Time) bool {
	roomCtx, roomCancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	detail, seed, startErr := s.deps.Store.StartNextBattleRoomRun(roomCtx, room.ID, now)
	roomCancel()
	if startErr != nil {
		if !errors.Is(startErr, store.ErrBattleRoomStateInvalid) {
			log.Printf("battle v2 ready-dispatch start next failed room_id=%d worker=%s err=%v", room.ID, workerID, startErr)
		}
		return false
	}
	s.pushBattleRoomUpdated(detail, context.Background())
	s.handleBattleRunStarted(detail, seed, now, battleCompileSourceReadyDispatch, battleCompileSourceReadyDispatch, true, context.Background())
	return true
}

func battleReadyDispatchNextSleep(compilePressure battleCompileQueuePressure, started int) time.Duration {
	if started > 0 && !compilePressure.isBacklogged() {
		return battleV2ReadyDispatchBusySleep
	}
	return battleV2ReadyDispatchSleep
}

// --- Loop 3: compile dispatch (compiling → kick off compile) ---

func (s *Server) runBattleCompileDispatchLoopV2(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		now := time.Now()
		sleepFor := s.tickBattleCompileDispatchV2(workerID, now)
		if sleepFor < battleV2CompileBusySleep {
			sleepFor = battleV2CompileBusySleep
		}
		timer := time.NewTimer(sleepFor)
		select {
		case <-timer.C:
		case <-s.battleStopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *Server) tickBattleCompileDispatchV2(workerID string, now time.Time) time.Duration {
	compilePressure := s.battleCompileQueuePressure()
	kickoffBudget := compilePressure.kickoffAdmissionBudget()
	if kickoffBudget <= 0 {
		return battleV2CompileDispatchSleep
	}
	claimCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	claimLimit := minInt(compilePressure.applyClaimLimit(battleV2ClaimLimit), kickoffBudget)
	rooms, err := s.deps.Store.ClaimBattleCompilingRoomsV2(
		claimCtx, workerID, claimLimit, battleRoomLeaseDuration, now,
	)
	if err != nil {
		log.Printf("battle v2 compile-dispatch claim failed worker=%s err=%v", workerID, err)
		return battleV2CompileDispatchSleep
	}
	if len(rooms) == 0 {
		return battleV2CompileDispatchSleep
	}
	kickedOff := 0
	for _, claim := range rooms {
		room := claim.AsBattleRoom()
		if room.CurrentRunID == nil || *room.CurrentRunID <= 0 {
			continue
		}
		startedAtMs := battleCompileDispatchOriginMs(room, now)
		// Compile dispatch only enqueues by room/run; the compile worker reloads full detail before building business results.
		detail := store.BattleRoomDetail{Room: room}
		if s.tryKickoffBattleCompile(detail, "", battleCompileSourceV2Dispatch, startedAtMs) {
			kickedOff++
		} else {
			// Compile queue full or dedup — reschedule.
			_ = s.deps.Store.ScheduleBattleRoomNextTick(context.Background(), room.ID, workerID, now.Add(200*time.Millisecond), now.Add(battleRoomLeaseDuration))
		}
	}
	if kickedOff > 0 {
		return battleV2CompileBusySleep
	}
	return battleV2CompileDispatchSleep
}

func battleCompileDispatchOriginMs(room store.BattleRoom, fallback time.Time) int64 {
	if !room.UpdatedAt.IsZero() {
		return room.UpdatedAt.UnixMilli()
	}
	return fallback.UnixMilli()
}

// --- Loop 4: playback dispatch (playback → start playback worker) ---

func (s *Server) runBattlePlaybackDispatchLoopV2(workerID string) {
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		now := time.Now()
		sleepFor := s.tickBattlePlaybackDispatchV2(workerID, now)
		if sleepFor < battleV2PlaybackBusySleep {
			sleepFor = battleV2PlaybackBusySleep
		}
		timer := time.NewTimer(sleepFor)
		select {
		case <-timer.C:
		case <-s.battleStopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *Server) tickBattlePlaybackDispatchV2(workerID string, now time.Time) time.Duration {
	claimCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rooms, err := s.deps.Store.ClaimBattlePlaybackRoomsV2(
		claimCtx, workerID, battleV2ClaimLimit, battleRoomLeaseDuration, now,
	)
	if err != nil {
		log.Printf("battle v2 playback-dispatch claim failed worker=%s err=%v", workerID, err)
		return battleV2PlaybackDispatchSleep
	}
	if len(rooms) == 0 {
		return battleV2PlaybackDispatchSleep
	}
	concurrency := battleTickShardWorkerCount(len(rooms), s.deps.Config.GlobalSettings.Battle.TickShardWorkers)
	shards := make([][]store.BattleRoomV2Claim, concurrency)
	for _, room := range rooms {
		shardIndex := int(room.ID % int64(concurrency))
		shards[shardIndex] = append(shards[shardIndex], room)
	}
	dispatchedByShard := make([]int, concurrency)
	var wg sync.WaitGroup
	for shardIndex := range shards {
		shardRooms := shards[shardIndex]
		if len(shardRooms) == 0 {
			continue
		}
		wg.Add(1)
		go func(index int, roomsInShard []store.BattleRoomV2Claim) {
			defer wg.Done()
			for _, room := range roomsInShard {
				if s.dispatchBattlePlaybackRoomV2(workerID, room) {
					dispatchedByShard[index]++
				}
			}
		}(shardIndex, shardRooms)
	}
	wg.Wait()
	dispatched := 0
	for _, count := range dispatchedByShard {
		dispatched += count
	}
	if dispatched > 0 {
		return battleV2PlaybackBusySleep
	}
	return battleV2PlaybackDispatchSleep
}

func (s *Server) dispatchBattlePlaybackRoomV2(workerID string, room store.BattleRoomV2Claim) bool {
	if room.CurrentRunID == nil || *room.CurrentRunID <= 0 {
		return false
	}
	roomCtx, roomCancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer roomCancel()
	memberIDs, memberErr := s.loadBattleRoomMemberIDsCached(roomCtx, room.ID)
	if memberErr != nil {
		log.Printf("battle v2 playback-dispatch load members failed room_id=%d worker=%s err=%v", room.ID, workerID, memberErr)
		return false
	}
	members := make([]store.BattleRoomMember, 0, len(memberIDs))
	for _, userID := range memberIDs {
		members = append(members, store.BattleRoomMember{UserID: userID})
	}
	run, runErr := s.deps.Store.GetBattleRunByRoomLite(roomCtx, room.ID)
	if runErr != nil || run.ID != *room.CurrentRunID {
		if runErr != nil {
			log.Printf("battle v2 playback-dispatch load run failed room_id=%d run_id=%d worker=%s err=%v", room.ID, *room.CurrentRunID, workerID, runErr)
		} else {
			log.Printf("battle v2 playback-dispatch run mismatch room_id=%d run_id=%d loaded_run_id=%d worker=%s", room.ID, *room.CurrentRunID, run.ID, workerID)
		}
		return false
	}
	detail := store.BattleRoomDetail{Room: room.AsBattleRoom(), Members: members}
	reservation, reserved := s.newBattlePlaybackWorkerReservation(room.ID, run.ID, workerID)
	if !reserved {
		return false
	}
	handoffAt := time.Now()
	leaseUntil := handoffAt.Add(battlePlaybackLeaseTTL)
	handoffOK, handoffErr := s.deps.Store.HandoffBattleRoomPlaybackLease(
		roomCtx,
		room.ID,
		run.ID,
		workerID,
		reservation.OwnerToken,
		leaseUntil,
		handoffAt,
	)
	if handoffErr != nil {
		log.Printf("battle v2 playback-dispatch handoff failed room_id=%d run_id=%d worker=%s err=%v", room.ID, run.ID, workerID, handoffErr)
		return false
	}
	if !handoffOK {
		return false
	}
	if !s.activateBattlePlaybackWorkerForRun(detail, reservation, handoffAt) {
		return false
	}
	leaseUntil, leaseErr := s.ensureBattlePlaybackLeaseBeforeStart(roomCtx, reservation, handoffAt, leaseUntil)
	if leaseErr != nil {
		s.releaseBattlePlaybackWorkerForRun(detail, reservation)
		log.Printf("battle v2 playback-dispatch lease lost before start room_id=%d run_id=%d worker=%s err=%v", room.ID, run.ID, workerID, leaseErr)
		return false
	}
	s.startReservedBattlePlaybackWorker(detail, run, reservation, handoffAt, leaseUntil)
	return true
}
