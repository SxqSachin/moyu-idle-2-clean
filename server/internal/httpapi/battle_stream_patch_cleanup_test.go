package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func TestCollectPlaybackEventsAtCurrentOffset(t *testing.T) {
	timeline := []wsbus.BattleTimelineEvent{
		{Seq: 1, OffsetMS: 100},
		{Seq: 2, OffsetMS: 100},
		{Seq: 3, OffsetMS: 320},
		{Seq: 4, OffsetMS: 560},
		{Seq: 5, OffsetMS: 560},
	}

	events, nextIdx, hasRemainder := collectPlaybackEventsAtCurrentOffset(timeline, 0, 100)
	if nextIdx != 2 {
		t.Fatalf("expected nextIdx=2, got %d", nextIdx)
	}
	if hasRemainder {
		t.Fatalf("expected no offset remainder for idx0 full fetch")
	}
	if len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("unexpected grouped events at idx0: %#v", events)
	}

	events, nextIdx, hasRemainder = collectPlaybackEventsAtCurrentOffset(timeline, 3, 1)
	if nextIdx != 4 {
		t.Fatalf("expected nextIdx=4 with limited batch, got %d", nextIdx)
	}
	if !hasRemainder {
		t.Fatalf("expected offset remainder for limited batch at idx3")
	}
	if len(events) != 1 || events[0].Seq != 4 {
		t.Fatalf("unexpected grouped events at idx3 with limited batch: %#v", events)
	}

	events, nextIdx, hasRemainder = collectPlaybackEventsAtCurrentOffset(timeline, 4, 100)
	if nextIdx != 5 {
		t.Fatalf("expected nextIdx=5, got %d", nextIdx)
	}
	if hasRemainder {
		t.Fatalf("expected no offset remainder for idx4 full fetch")
	}
	if len(events) != 1 || events[0].Seq != 5 {
		t.Fatalf("unexpected grouped events at idx4: %#v", events)
	}

	events, nextIdx, hasRemainder = collectPlaybackEventsAtCurrentOffset(timeline, 5, 100)
	if nextIdx != 5 {
		t.Fatalf("expected unchanged nextIdx for out-of-range, got %d", nextIdx)
	}
	if hasRemainder {
		t.Fatalf("expected no offset remainder for out-of-range start")
	}
	if len(events) != 0 {
		t.Fatalf("expected no events for out-of-range start, got %#v", events)
	}
}

func TestPlaybackPublishFailureDoesNotAdvanceSnapshotCheckpoint(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	server := &Server{
		deps:                app.Dependencies{Config: config.Config{GlobalSettings: settings}},
		wsNATS:              &nats.Conn{},
		wsSubject:           "battle.test",
		battlePlaybackMem:   make(map[int64]battleRunPlaybackCache),
		battlePlaybackTouch: make(map[int64]time.Time),
		battleCheckpointMem: make(map[int64]battlePlaybackCheckpoint),
	}
	now := time.UnixMilli(30_000)
	runID := int64(301)
	checkpointMap := map[string]any{
		"playbackStartedAtMs": now.UnixMilli(),
		"wave":                1,
		"players":             []map[string]any{{"unitId": "player:1", "hp": float64(100)}},
		"enemies":             []map[string]any{{"unitId": "enemy:1", "hp": float64(50)}},
		"totalEvents":         1,
		"finalSeq":            int64(1),
	}
	server.setBattleRunPlaybackCache(runID, []wsbus.BattleTimelineEvent{{Seq: 1, OffsetMS: 500}})
	server.seedBattleRunPlaybackCheckpointLocal(runID, decodeBattlePlaybackCheckpoint(checkpointMap, now))

	_, _, err := server.playbackBattleRunTick(
		context.Background(),
		store.BattleRoomDetail{
			Room:    store.BattleRoom{ID: 17},
			Members: []store.BattleRoomMember{{UserID: 99}},
		},
		store.BattleRun{ID: runID, StartedAt: now, State: checkpointMap},
		now,
	)
	if !errors.Is(err, errBattlePlaybackPublishUnavailable) {
		t.Fatalf("playbackBattleRunTick error=%v, want publish unavailable", err)
	}
	persisted := server.battleCheckpointMem[runID]
	if persisted.SnapshotPushed || persisted.NextEventIdx != 0 || persisted.LastPushedSeq != 0 {
		t.Fatalf("failed snapshot advanced checkpoint: %#v", persisted)
	}
}

func TestPlaybackPublishFailureDoesNotMutateOrAdvanceEventCheckpoint(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	server := &Server{
		deps:                app.Dependencies{Config: config.Config{GlobalSettings: settings}},
		wsNATS:              &nats.Conn{},
		wsSubject:           "battle.test",
		battlePlaybackMem:   make(map[int64]battleRunPlaybackCache),
		battlePlaybackTouch: make(map[int64]time.Time),
		battleCheckpointMem: make(map[int64]battlePlaybackCheckpoint),
	}
	now := time.UnixMilli(40_000)
	runID := int64(401)
	checkpointMap := map[string]any{
		"playbackStartedAtMs": now.Add(-time.Second).UnixMilli(),
		"nextPushAtMs":        now.UnixMilli(),
		"snapshotPushed":      true,
		"wave":                1,
		"players":             []map[string]any{{"unitId": "player:1", "hp": float64(100)}},
		"enemies":             []map[string]any{{"unitId": "enemy:1", "hp": float64(50), "alive": true}},
		"totalEvents":         1,
		"finalSeq":            int64(1),
	}
	timeline := []wsbus.BattleTimelineEvent{
		timelineEventForTest(t, 1, 500, "battle.turn.action", map[string]any{
			"targets": []map[string]any{{"targetId": "enemy:1", "hp": float64(25), "alive": true}},
		}),
	}
	server.setBattleRunPlaybackCache(runID, timeline)
	server.seedBattleRunPlaybackCheckpointLocal(runID, decodeBattlePlaybackCheckpoint(checkpointMap, now))

	_, _, err := server.playbackBattleRunTick(
		context.Background(),
		store.BattleRoomDetail{
			Room:    store.BattleRoom{ID: 18},
			Members: []store.BattleRoomMember{{UserID: 100}},
		},
		store.BattleRun{ID: runID, StartedAt: now.Add(-time.Second), State: checkpointMap},
		now,
	)
	if !errors.Is(err, errBattlePlaybackPublishUnavailable) {
		t.Fatalf("playbackBattleRunTick error=%v, want publish unavailable", err)
	}
	persisted := server.battleCheckpointMem[runID]
	if persisted.NextEventIdx != 0 || persisted.LastPushedSeq != 0 {
		t.Fatalf("failed event publish advanced cursor: %#v", persisted)
	}
	enemy, ok := playbackFindUnitMap(persisted.Enemies, "enemy:1")
	if !ok || toFloat64Loose(enemy["hp"], 0) != 50 {
		t.Fatalf("failed event publish mutated checkpoint enemy: %#v", enemy)
	}
}

func TestCompilePlaybackHandoffPrimesSnapshotBeforeWorkerLaunch(t *testing.T) {
	body, err := os.ReadFile("battle_runtime_engine.go")
	if err != nil {
		t.Fatalf("read battle_runtime_engine.go: %v", err)
	}
	source := string(body)
	start := strings.Index(source, "func (s *Server) tryStartBattlePlaybackAfterCompile")
	if start < 0 {
		t.Fatal("tryStartBattlePlaybackAfterCompile missing")
	}
	end := strings.Index(source[start+1:], "\nfunc ")
	if end < 0 {
		t.Fatal("tryStartBattlePlaybackAfterCompile boundary missing")
	}
	functionBody := source[start : start+1+end]
	leaseAt := strings.Index(functionBody, "s.ensureBattlePlaybackLeaseForTick(")
	primeAt := strings.Index(functionBody, "s.playbackBattleRunTick(")
	launchAt := strings.Index(functionBody, "s.startReservedBattlePlaybackWorker(")
	if leaseAt < 0 || primeAt < 0 || launchAt < 0 || leaseAt >= primeAt || primeAt >= launchAt {
		t.Fatalf("initial playback lease must be checked before tick and worker launch: lease=%d prime=%d launch=%d", leaseAt, primeAt, launchAt)
	}
}

func TestPlaybackNextDelayMs(t *testing.T) {
	timeline := []wsbus.BattleTimelineEvent{
		{Seq: 1, OffsetMS: 100},
		{Seq: 2, OffsetMS: 350},
		{Seq: 3, OffsetMS: 900},
	}

	if delay := playbackNextDelayMs(200, 3000, timeline, 1, 100); delay != 200 {
		t.Fatalf("expected fixed delay 200, got %d", delay)
	}
	if delay := playbackNextDelayMs(300, 3000, timeline, 2, 350); delay != 300 {
		t.Fatalf("expected fixed delay 300, got %d", delay)
	}
	if delay := playbackNextDelayMs(300, 3000, timeline, 2, 700); delay != 300 {
		t.Fatalf("expected fixed delay 300, got %d", delay)
	}
	if delay := playbackNextDelayMs(300, 3000, timeline, 3, 900); delay != 300 {
		t.Fatalf("expected fixed delay 300 when no next event, got %d", delay)
	}
	if delay := playbackNextDelayMs(300, 1000, []wsbus.BattleTimelineEvent{{Seq: 1, OffsetMS: 100}, {Seq: 2, OffsetMS: 5000}}, 1, 100); delay != 300 {
		t.Fatalf("expected fixed delay 300, got %d", delay)
	}
	if delay := playbackNextDelayMs(2000, 1000, []wsbus.BattleTimelineEvent{{Seq: 1, OffsetMS: 100}, {Seq: 2, OffsetMS: 5000}}, 1, 100); delay != 2000 {
		t.Fatalf("expected fixed delay 2000, got %d", delay)
	}
}

func TestDerivePlaybackCheckpointByTimeUsesTimelineState(t *testing.T) {
	startedAt := time.UnixMilli(1000)
	initial := map[string]any{
		"wave":             1,
		"completedBattles": 0,
		"seq":              int64(0),
		"players": []map[string]any{
			{
				"unitId":         "player:1",
				"side":           "player",
				"hp":             float64(100),
				"mp":             float64(10),
				"maxHp":          float64(100),
				"maxMp":          float64(10),
				"alive":          true,
				"skillCooldowns": map[string]any{"power_strike": float64(0), "basic_attack": float64(0)},
				"skills": []map[string]any{
					{"id": "power_strike", "cooldownTurns": float64(3), "cooldownRemain": float64(0)},
					{"id": "basic_attack", "cooldownTurns": float64(0), "cooldownRemain": float64(0)},
				},
			},
		},
		"enemies": []map[string]any{
			{"unitId": "enemy:1", "side": "enemy", "hp": float64(100), "mp": float64(0), "maxHp": float64(100), "maxMp": float64(0), "alive": true},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		timelineEventForTest(t, 1, 500, "battle.timeline.node", map[string]any{
			"o": "player:1",
			"p": []map[string]any{
				{"k": "c", "s": "player:1", "i": "power_strike"},
				{"k": "d", "s": "player:1", "t": []map[string]any{{"u": "enemy:1", "h": float64(75), "l": true}}},
			},
			"u": []map[string]any{{"u": "enemy:1", "h": float64(75), "l": true}},
		}),
	}
	checkpoint := decodeBattlePlaybackCheckpoint(map[string]any{
		"playbackStartedAtMs": startedAt.UnixMilli(),
		"wave":                1,
		"completedBattles":    0,
		"seq":                 int64(0),
		"players":             initial["players"],
		"enemies":             initial["enemies"],
	}, startedAt)
	derived := derivePlaybackCheckpointByTime(checkpoint, timeline, map[string]any{}, startedAt.Add(time.Second))
	if derived.Seq != 1 || derived.NextEventIdx != 1 {
		t.Fatalf("derived cursor mismatch: seq=%d idx=%d", derived.Seq, derived.NextEventIdx)
	}
	enemy, ok := playbackFindUnitMap(derived.Enemies, "enemy:1")
	if !ok {
		t.Fatalf("enemy missing from derived checkpoint")
	}
	if hp := toFloat64Loose(enemy["hp"], 0); hp != 75 {
		t.Fatalf("enemy hp=%v, want 75", hp)
	}
	player, ok := playbackFindUnitMap(derived.Players, "player:1")
	if !ok {
		t.Fatalf("player missing from derived checkpoint")
	}
	cooldowns := normalizeAnyMap(player["skillCooldowns"])
	if got := toInt64Loose(cooldowns["power_strike"], -1); got != 2 {
		t.Fatalf("power_strike cooldown=%d, want 2", got)
	}
}

func TestBattlePlaybackCheckpointWithFallbackUsesRunState(t *testing.T) {
	startedAt := time.UnixMilli(1000)
	fallback := map[string]any{
		"wave":             1,
		"completedBattles": 0,
		"seq":              int64(0),
		"players": []map[string]any{
			{
				"unitId":         "player:1",
				"side":           "player",
				"hp":             float64(100),
				"mp":             float64(10),
				"maxHp":          float64(100),
				"maxMp":          float64(10),
				"alive":          true,
				"skillCooldowns": map[string]any{"power_strike": float64(0)},
			},
		},
		"enemies": []map[string]any{
			{"unitId": "enemy:1", "side": "enemy", "hp": float64(100), "mp": float64(0), "maxHp": float64(100), "maxMp": float64(0), "alive": true},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		timelineEventForTest(t, 1, 500, "battle.turn.action", map[string]any{
			"actorId": "player:1",
			"targets": []map[string]any{
				{"targetId": "enemy:1", "hp": float64(75), "alive": true},
			},
		}),
	}
	checkpoint := decodeBattlePlaybackCheckpoint(map[string]any{
		"playbackStartedAtMs": startedAt.UnixMilli(),
		"lastPushedSeq":       int64(1),
		"lastPushedOffsetMs":  int64(500),
		"nextEventIdx":        1,
		"snapshotPushed":      true,
	}, startedAt)
	derived := battlePlaybackCheckpointWithFallback(checkpoint, fallback, timeline, startedAt.Add(time.Second))
	if derived.Seq != 1 || derived.NextEventIdx != 1 {
		t.Fatalf("derived cursor mismatch: seq=%d idx=%d", derived.Seq, derived.NextEventIdx)
	}
	if len(derived.Players) == 0 || len(derived.Enemies) == 0 {
		t.Fatalf("expected fallback units to be preserved: %#v", derived)
	}
	enemy, ok := playbackFindUnitMap(derived.Enemies, "enemy:1")
	if !ok {
		t.Fatalf("enemy missing from fallback-derived checkpoint")
	}
	if hp := toFloat64Loose(enemy["hp"], 0); hp != 75 {
		t.Fatalf("enemy hp=%v, want 75", hp)
	}
	player, ok := playbackFindUnitMap(derived.Players, "player:1")
	if !ok {
		t.Fatalf("player missing from fallback-derived checkpoint")
	}
	cooldowns := normalizeAnyMap(player["skillCooldowns"])
	if got := toInt64Loose(cooldowns["power_strike"], -1); got != 0 {
		t.Fatalf("power_strike cooldown=%d, want 0", got)
	}
}

func TestRebuildBattlePlaybackCheckpointRepersistsRecoveredSnapshot(t *testing.T) {
	startedAt := time.UnixMilli(1000)
	server := &Server{
		battlePlaybackMem:   make(map[int64]battleRunPlaybackCache),
		battlePlaybackTouch: make(map[int64]time.Time),
		battleCheckpointMem: make(map[int64]battlePlaybackCheckpoint),
	}
	fallback := map[string]any{
		"wave":             1,
		"completedBattles": 0,
		"seq":              int64(0),
		"players": []map[string]any{
			{"unitId": "player:1", "side": "player", "hp": float64(100), "alive": true},
		},
		"enemies": []map[string]any{
			{"unitId": "enemy:1", "side": "enemy", "hp": float64(100), "alive": true},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		timelineEventForTest(t, 1, 500, "battle.turn.action", map[string]any{
			"actorId": "player:1",
			"targets": []map[string]any{
				{"targetId": "enemy:1", "hp": float64(75), "alive": true},
			},
		}),
	}
	cursorOnly := decodeBattlePlaybackCheckpoint(map[string]any{
		"playbackStartedAtMs": startedAt.UnixMilli(),
		"lastPushedSeq":       int64(1),
		"lastPushedOffsetMs":  int64(500),
		"nextEventIdx":        1,
		"snapshotPushed":      true,
	}, startedAt)
	derived := server.rebuildBattlePlaybackCheckpoint(42, cursorOnly, fallback, timeline, startedAt.Add(time.Second))
	if len(derived.Players) == 0 || len(derived.Enemies) == 0 {
		t.Fatalf("expected rebuilt checkpoint units, got %#v", derived)
	}
	persisted, ok := server.battleCheckpointMem[42]
	if !ok {
		t.Fatalf("expected recovered checkpoint to be cached")
	}
	if len(persisted.Players) == 0 || len(persisted.Enemies) == 0 {
		t.Fatalf("expected persisted checkpoint units, got %#v", persisted)
	}
}

func TestSeedBattleRunPlaybackCheckpointLocalUsesCompiledCursor(t *testing.T) {
	now := time.UnixMilli(10_000)
	server := &Server{}
	compiled := decodeBattlePlaybackCheckpoint(map[string]any{
		"playbackStartedAtMs": now.UnixMilli(),
		"compileStartedAtMs":  now.UnixMilli(),
		"compileFinishedAtMs": now.Add(125 * time.Millisecond).UnixMilli(),
		"wave":                17,
		"players":             []map[string]any{{"unitId": "player:1", "hp": float64(100)}},
		"enemies":             []map[string]any{{"unitId": "enemy:1", "hp": float64(50)}},
	}, now)

	server.seedBattleRunPlaybackCheckpointLocal(42, compiled)
	loaded := server.loadBattleRunPlaybackCheckpoint(
		42,
		map[string]any{"wave": 99},
		now.Add(time.Second),
	)

	if loaded.Wave != 17 {
		t.Fatalf("loaded wave=%d, want compiled wave 17", loaded.Wave)
	}
	if loaded.RedisPersistedAtMs != 0 {
		t.Fatalf("seeded checkpoint persisted_at_ms=%d, want 0", loaded.RedisPersistedAtMs)
	}
	if len(loaded.Players) != 1 || len(loaded.Enemies) != 1 {
		t.Fatalf("seeded units mismatch: players=%d enemies=%d", len(loaded.Players), len(loaded.Enemies))
	}
	if touchedAt := server.battlePlaybackTouch[42]; touchedAt.IsZero() {
		t.Fatal("expected seeded checkpoint to touch local playback cache")
	}

	progressed := compiled
	progressed.SnapshotPushed = true
	progressed.LastPushedSeq = 7
	progressed.NextEventIdx = 3
	server.seedBattleRunPlaybackCheckpointLocal(43, progressed)
	server.seedBattleRunPlaybackCheckpointLocal(43, compiled)
	if got := server.battleCheckpointMem[43]; got.LastPushedSeq != 7 || got.NextEventIdx != 3 {
		t.Fatalf("seed replaced progressed cursor: seq=%d idx=%d", got.LastPushedSeq, got.NextEventIdx)
	}
}

func TestSeededCompiledCheckpointSkipsRedisLoadAndPersistsFirstSnapshot(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rd.Close() })
	ctx := context.Background()
	if err := rd.Ping(ctx).Err(); err != nil {
		t.Fatalf("warm Redis connection: %v", err)
	}

	settings := config.DefaultGlobalSettings()
	server := &Server{
		deps: app.Dependencies{
			Redis:  rd,
			Config: config.Config{GlobalSettings: settings},
		},
		battleLoopEnabled:   true,
		battlePlaybackMem:   make(map[int64]battleRunPlaybackCache),
		battlePlaybackTouch: make(map[int64]time.Time),
		battleCheckpointMem: make(map[int64]battlePlaybackCheckpoint),
	}
	now := time.UnixMilli(20_000)
	runID := int64(42)
	checkpointMap := map[string]any{
		"playbackStartedAtMs": now.UnixMilli(),
		"compileStartedAtMs":  now.UnixMilli(),
		"compileFinishedAtMs": now.Add(100 * time.Millisecond).UnixMilli(),
		"wave":                3,
		"players":             []map[string]any{{"unitId": "player:1", "hp": float64(100)}},
		"enemies":             []map[string]any{{"unitId": "enemy:1", "hp": float64(50)}},
		"totalEvents":         1,
		"finalSeq":            int64(1),
	}
	timeline := []wsbus.BattleTimelineEvent{{Seq: 1, OffsetMS: 500, Payload: json.RawMessage(`{"eventType":"battle.turn.action","data":{}}`)}}
	server.setBattleRunPlaybackCache(runID, timeline)
	server.seedBattleRunPlaybackCheckpointLocal(runID, decodeBattlePlaybackCheckpoint(checkpointMap, now))

	commandsBeforeLoad := mr.CommandCount()
	loaded := server.loadBattleRunPlaybackCheckpoint(runID, map[string]any{"wave": 99}, now)
	if commandsAfterLoad := mr.CommandCount(); commandsAfterLoad != commandsBeforeLoad {
		t.Fatalf("local checkpoint load issued Redis command: before=%d after=%d", commandsBeforeLoad, commandsAfterLoad)
	}
	if loaded.Wave != 3 {
		t.Fatalf("loaded wave=%d, want compiled wave 3", loaded.Wave)
	}

	commandsBeforeTick := mr.CommandCount()
	nextTickAt, result, err := server.playbackBattleRunTick(
		ctx,
		store.BattleRoomDetail{Room: store.BattleRoom{ID: 7}},
		store.BattleRun{ID: runID, StartedAt: now, State: checkpointMap},
		now,
	)
	if err != nil {
		t.Fatalf("playbackBattleRunTick failed: %v", err)
	}
	if !nextTickAt.After(now) {
		t.Fatalf("next tick=%v, want after %v", nextTickAt, now)
	}
	if result.NextEventIdx != 0 || result.LastPushedSeq != 0 {
		t.Fatalf("first snapshot advanced timeline cursor: %#v", result)
	}
	if commandsAfterTick := mr.CommandCount(); commandsAfterTick-commandsBeforeTick != 1 {
		t.Fatalf("first snapshot Redis commands=%d, want exactly one SET", commandsAfterTick-commandsBeforeTick)
	}
	raw, err := mr.Get(battlePlaybackCheckpointRedisKey(runID))
	if err != nil {
		t.Fatalf("first snapshot checkpoint missing from Redis: %v", err)
	}
	persisted := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
		t.Fatalf("decode persisted checkpoint: %v", err)
	}
	if !toBoolLoose(persisted["snapshotPushed"], false) {
		t.Fatalf("persisted snapshotPushed=%v, want true", persisted["snapshotPushed"])
	}
	if got := toInt64Loose(persisted["nextPushAtMs"], 0); got <= now.UnixMilli() {
		t.Fatalf("persisted nextPushAtMs=%d, want after %d", got, now.UnixMilli())
	}
	if got := toInt64Loose(persisted["nextEventIdx"], -1); got != 0 {
		t.Fatalf("persisted nextEventIdx=%d, want 0", got)
	}
	if got := toInt64Loose(persisted["lastPushedSeq"], -1); got != 0 {
		t.Fatalf("persisted lastPushedSeq=%d, want 0", got)
	}
}

func timelineEventForTest(t *testing.T, seq int64, offset int64, eventType string, data map[string]any) wsbus.BattleTimelineEvent {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"eventType": eventType,
		"data":      data,
	})
	if err != nil {
		t.Fatalf("marshal timeline payload: %v", err)
	}
	return wsbus.BattleTimelineEvent{Seq: seq, OffsetMS: offset, Payload: raw}
}

func TestPlaybackCheckpointAtTimelineEnd(t *testing.T) {
	timeline := []wsbus.BattleTimelineEvent{
		{Seq: 1, OffsetMS: 100},
		{Seq: 2, OffsetMS: 200},
	}
	if playbackCheckpointAtTimelineEnd(battlePlaybackCheckpoint{NextEventIdx: 1}, timeline) {
		t.Fatalf("expected idx=1 with len=2 to be non-terminal")
	}
	if !playbackCheckpointAtTimelineEnd(battlePlaybackCheckpoint{NextEventIdx: 2}, timeline) {
		t.Fatalf("expected idx=len to be terminal")
	}
	if !playbackCheckpointAtTimelineEnd(battlePlaybackCheckpoint{NextEventIdx: 3}, timeline) {
		t.Fatalf("expected idx>len to be terminal")
	}
}

func TestBattleRoomMemberUserIDs(t *testing.T) {
	members := []store.BattleRoomMember{
		{UserID: 11},
		{UserID: 0},
		{UserID: 11},
		{UserID: -1},
		{UserID: 25},
	}
	userIDs := battleRoomMemberUserIDs(members)
	if len(userIDs) != 2 {
		t.Fatalf("expected 2 user ids, got %d (%#v)", len(userIDs), userIDs)
	}
	if userIDs[0] != 11 || userIDs[1] != 25 {
		t.Fatalf("unexpected user ids ordering/content: %#v", userIDs)
	}
}

func TestBattleEventRoomID(t *testing.T) {
	if roomID, ok := battleEventRoomID(map[string]any{"roomId": 123}); !ok || roomID != 123 {
		t.Fatalf("expected roomId field to be parsed, got roomID=%d ok=%t", roomID, ok)
	}
	if roomID, ok := battleEventRoomID(map[string]any{"m": float64(456)}); !ok || roomID != 456 {
		t.Fatalf("expected compact m field to be parsed, got roomID=%d ok=%t", roomID, ok)
	}
	if roomID, ok := battleEventRoomID(map[string]any{
		"room": map[string]any{"id": "789"},
	}); !ok || roomID != 789 {
		t.Fatalf("expected nested room.id field to be parsed, got roomID=%d ok=%t", roomID, ok)
	}
	if roomID, ok := battleEventRoomID(map[string]any{"x": 1}); ok || roomID != 0 {
		t.Fatalf("expected missing room id to fail parse, got roomID=%d ok=%t", roomID, ok)
	}
}

func TestIsBattleRunBatchWSEventType(t *testing.T) {
	if !isBattleRunBatchWSEventType("battle.run.batch") {
		t.Fatalf("expected battle.run.batch to be recognized")
	}
	if !isBattleRunBatchWSEventType(" Battle.Run.Batch ") {
		t.Fatalf("expected battle.run.batch to be case-insensitive")
	}
	if isBattleRunBatchWSEventType("battle.room.self") {
		t.Fatalf("battle.room.self should not be treated as battle.run.batch")
	}
}

func TestIsBattleRoomUpdatedWSEventType(t *testing.T) {
	if !isBattleRoomUpdatedWSEventType("battle.room.updated") {
		t.Fatalf("expected battle.room.updated to be recognized")
	}
	if !isBattleRoomUpdatedWSEventType(" Battle.Room.Updated ") {
		t.Fatalf("expected battle.room.updated to be case-insensitive")
	}
	if isBattleRoomUpdatedWSEventType("battle.room.self") {
		t.Fatalf("battle.room.self should not be treated as battle.room.updated")
	}
}

func TestIsStrictBattleRunBatchEnvelope(t *testing.T) {
	validPayload := map[string]any{"m": int64(321)}
	if !isStrictBattleRunBatchEnvelope("target_users", []int64{11, 11, 25}, validPayload) {
		t.Fatalf("expected strict battle batch envelope to pass for valid target users payload")
	}
	if isStrictBattleRunBatchEnvelope("broadcast_all", []int64{11}, validPayload) {
		t.Fatalf("battle.run.batch should reject non-target_users envelopes")
	}
	if isStrictBattleRunBatchEnvelope("target_users", nil, validPayload) {
		t.Fatalf("battle.run.batch should reject empty user ids")
	}
	if isStrictBattleRunBatchEnvelope("target_users", []int64{0, -1}, validPayload) {
		t.Fatalf("battle.run.batch should reject invalid user ids")
	}
	if isStrictBattleRunBatchEnvelope("target_users", []int64{11}, map[string]any{"x": 1}) {
		t.Fatalf("battle.run.batch should reject payload without room id")
	}
	if isStrictBattleRunBatchEnvelope("target_users", []int64{11}, map[string]any{"room": map[string]any{"id": "456"}}) == false {
		t.Fatalf("battle.run.batch should accept payload with nested room.id")
	}
}

func TestIsStrictBattleRoomUpdatedEnvelope(t *testing.T) {
	validPayload := map[string]any{"room": map[string]any{"id": int64(321)}}
	if !isStrictBattleRoomUpdatedEnvelope("target_users", []int64{11, 11, 25}, validPayload) {
		t.Fatalf("expected strict battle room updated envelope to pass for valid target users payload")
	}
	if isStrictBattleRoomUpdatedEnvelope("broadcast_all", []int64{11}, validPayload) {
		t.Fatalf("battle.room.updated should reject non-target_users envelopes")
	}
	if isStrictBattleRoomUpdatedEnvelope("target_users", nil, validPayload) {
		t.Fatalf("battle.room.updated should reject empty user ids")
	}
	if isStrictBattleRoomUpdatedEnvelope("target_users", []int64{0, -1}, validPayload) {
		t.Fatalf("battle.room.updated should reject invalid user ids")
	}
	if isStrictBattleRoomUpdatedEnvelope("target_users", []int64{11}, map[string]any{"x": 1}) {
		t.Fatalf("battle.room.updated should reject payload without room id")
	}
}

func TestClassifyWSEventCategory(t *testing.T) {
	if got := classifyWSEventCategory("chat.message", "broadcast_all", ""); got != wsbus.EventCategoryGlobal {
		t.Fatalf("expected broadcast_all to classify as global, got %s", got)
	}
	if got := classifyWSEventCategory("auth.kicked", "target_users", ""); got != wsbus.EventCategoryControl {
		t.Fatalf("expected auth.kicked to classify as control, got %s", got)
	}
	if got := classifyWSEventCategory("battle.run.batch", "target_users", ""); got != wsbus.EventCategoryUserState {
		t.Fatalf("expected battle.run.batch to classify as user_state, got %s", got)
	}
	if got := classifyWSEventCategory("battle.run.batch", "target_users", wsbus.EventCategoryControl); got != wsbus.EventCategoryControl {
		t.Fatalf("expected explicit category to win, got %s", got)
	}
}

func TestIsMustDeliverWSEventType(t *testing.T) {
	if !isMustDeliverWSEventType("auth.kicked") {
		t.Fatalf("auth.kicked should be must-deliver")
	}
	if !isMustDeliverWSEventType("auth.bound") {
		t.Fatalf("auth.bound should be must-deliver")
	}
	if !isMustDeliverWSEventType("error") {
		t.Fatalf("error should be must-deliver")
	}
	if isMustDeliverWSEventType("battle.run.batch") {
		t.Fatalf("battle.run.batch should not be must-deliver")
	}
}

func TestCompactBattleSnapshotUnitsIncludesSkillLevels(t *testing.T) {
	rows := []map[string]any{
		{
			"unitId":         "player:1",
			"hp":             100,
			"mp":             50,
			"maxHp":          100,
			"maxMp":          50,
			"skillIds":       []string{"fireball", "heal"},
			"skillCooldowns": map[string]any{"fireball": 2, "heal": 0},
			"skillLevels":    map[string]any{"fireball": 3, "heal": 0},
		},
	}

	compacted := compactBattleSnapshotUnits(rows)
	if len(compacted) != 1 {
		t.Fatalf("expected 1 compacted unit, got %d", len(compacted))
	}
	unit := compacted[0]
	levels, ok := unit["g"].(map[string]any)
	if !ok {
		t.Fatalf("expected skill level map in compacted unit, got %#v", unit["g"])
	}
	if got := toInt64Loose(levels["fireball"], 0); got != 3 {
		t.Fatalf("expected fireball level 3, got %d", got)
	}
	if _, exists := levels["heal"]; exists {
		t.Fatalf("expected non-positive heal level to be omitted, got %#v", levels["heal"])
	}
}

func TestNormalizePresenceStatus(t *testing.T) {
	if got := normalizePresenceStatus("offline"); got != "offline" {
		t.Fatalf("expected offline to remain offline, got %s", got)
	}
	if got := normalizePresenceStatus(" OFFLINE "); got != "offline" {
		t.Fatalf("expected normalized offline, got %s", got)
	}
	if got := normalizePresenceStatus("online"); got != "online" {
		t.Fatalf("expected online, got %s", got)
	}
	if got := normalizePresenceStatus("unknown"); got != "online" {
		t.Fatalf("unknown should fallback to online, got %s", got)
	}
}

func TestBattleStreamSubscriptionState(t *testing.T) {
	s := &Server{
		battleStreamSubs: make(map[string]struct{}),
	}
	if s.isBattleStreamSubscribed("client-1") {
		t.Fatalf("expected client-1 unsubscribed by default")
	}
	s.setBattleStreamSubscribed("client-1", true)
	if !s.isBattleStreamSubscribed("client-1") {
		t.Fatalf("expected client-1 subscribed after enabling")
	}
	s.setBattleStreamSubscribed("client-1", false)
	if s.isBattleStreamSubscribed("client-1") {
		t.Fatalf("expected client-1 unsubscribed after disabling")
	}
	s.setBattleStreamSubscribed("client-2", true)
	s.clearBattleStreamSubscribed("client-2")
	if s.isBattleStreamSubscribed("client-2") {
		t.Fatalf("expected clear to unsubscribe client-2")
	}
}
