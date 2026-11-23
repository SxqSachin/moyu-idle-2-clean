# Battle Streaming Architecture (Current)

This document captures the current battle push path after the recent performance and routing fixes.

## 1) Broadcast Path

1. Battle worker produces room-scoped battle events.
2. `pushBattleRoomEvent` resolves room members and calls `pushBattleRoomEventToUsers`.
3. `pushBattleRoomEventToUsers` sends one targeted WS cluster event (`publishWSTargetUsers`) with all user IDs.
4. NATS carries **plain JSON envelope** (`wsbus.Event`).
5. On each node, `deliverWSBroadcastEvent` fans out to local clients and runs `canDeliverWSClientEvent`.

## 2) Delivery Guardrails

For all WS events, client authentication is required (`UserID > 0` and `SessionVersion > 0`).

`battle.run.batch` now uses strict gateway envelope checks:

- `targetType` must be `target_users`.
- `userIds` must contain at least one valid user id.
- Event payload must include room id (`roomId` / `m` / `room.id`).

Invalid `battle.run.batch` envelopes are dropped by the gateway (no compatibility fallback).

## 3) Compression Boundary

Compression is applied **after NATS consumption** and immediately before socket enqueue:

- `writeWSEvent -> enqueueWSMessage -> tryCompressWSMessage`
- If message size >= configured threshold, payload is zlib-compressed and sent as binary WS frame.

NATS payload is not compressed in this path.

## 4) Playback Tick Pacing

`playbackBattleRunTick` uses checkpoint-driven pacing:

- Snapshot batch is sent once per run to room-targeted recipients.
- Checkpoint stores `nextPushAtMs`, `nextEventIdx`, `lastPushedSeq`, `lastPushedOffsetMs`.
- Each due tick pushes one offset bucket (events sharing the same `offsetMs`).
- Next delay uses `max(timelinePushIntervalMs, nextOffset - lastOffset)`.
- Client replay queue uses the server event spacing as an upper bound so it does not drift slower than the streamed timeline.

This implements per-turn paced replay without catch-up bursts.

## 5) Key Files

- `server/internal/httpapi/server_ws.go`
- `server/internal/httpapi/server.go`
- `server/internal/httpapi/server_battle.go`
- `server/internal/httpapi/battle_runtime_engine.go`
- `web/src/composables/game/combat/useCombatPage.ts`
