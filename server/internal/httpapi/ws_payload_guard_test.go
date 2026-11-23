package httpapi

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/wsbus"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPrepareWSBroadcastEnvelopeCompactsOversizedInventoryPatch(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"upsert": []map[string]any{{"instanceId": 1, "name": strings.Repeat("x", 1024)}},
		"remove": []int64{},
		"meta":   map[string]any{"version": 17, "epoch": "gateway-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	evt := wsbus.NewTargetUsers(inventoryInstancesUpdatedEventType, []int64{42}, payload, 123, "", "")
	prepared, body, compacted, err := prepareWSBroadcastEnvelopeForPublish(evt, 320, false)
	if err != nil {
		t.Fatalf("prepare event: %v", err)
	}
	if !compacted || len(body) > 320 {
		t.Fatalf("compacted=%t bytes=%d", compacted, len(body))
	}
	var refresh inventoryRefreshRequiredPayload
	if err := json.Unmarshal(prepared.Payload, &refresh); err != nil {
		t.Fatal(err)
	}
	if !refresh.RefreshRequired || refresh.Meta.Version != 17 || refresh.Meta.Epoch != "gateway-a" {
		t.Fatalf("refresh payload=%#v", refresh)
	}
	if len(prepared.UserIDs) != 1 || prepared.UserIDs[0] != 42 {
		t.Fatalf("target users changed: %#v", prepared.UserIDs)
	}
}

func TestPrepareWSBroadcastEnvelopeRejectsUnrecoverableOversizedEvent(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"value": strings.Repeat("x", 1024)})
	evt := wsbus.NewTargetUsers("player.attributes.updated", []int64{42}, payload, 123, "", "")
	if _, _, _, err := prepareWSBroadcastEnvelopeForPublish(evt, 320, false); err == nil {
		t.Fatal("oversized non-inventory event was accepted")
	}
}

func TestPrepareWSBroadcastEnvelopeLeavesOrdinaryJSONUnchanged(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"roomId": 9, "events": []any{}})
	evt := wsbus.NewTargetUsers("battle.run.batch", []int64{42}, payload, 123, "request-1", "client-2")
	want, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	prepared, body, compacted, err := prepareWSBroadcastEnvelopeForPublish(evt, int64(len(want)), false)
	if err != nil {
		t.Fatalf("prepare event: %v", err)
	}
	if compacted {
		t.Fatal("ordinary event was marked compacted")
	}
	if !reflect.DeepEqual(prepared, evt) {
		t.Fatalf("prepared event changed: got=%#v want=%#v", prepared, evt)
	}
	if !bytes.Equal(body, want) {
		t.Fatal("ordinary event JSON changed")
	}
}

func TestPrepareWSBroadcastEnvelopeCompressesOversizedBattleBatch(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"m": 9,
		"r": 27,
		"n": map[string]any{"players": []map[string]any{{"id": "player-1", "state": strings.Repeat("x", 64<<10)}}},
	})
	evt := wsbus.NewTargetUsers("battle.run.batch", []int64{42}, payload, 123, "request-1", "client-2")

	prepared, body, compacted, err := prepareWSBroadcastEnvelopeForPublish(evt, 2048, true)
	if err != nil {
		t.Fatalf("prepare event: %v", err)
	}
	if compacted {
		t.Fatal("battle batch was marked as an inventory compaction")
	}
	if !isWSBroadcastCompressedFrame(body) {
		t.Fatal("oversized battle batch was not framed")
	}
	if len(body) > 2048 {
		t.Fatalf("compressed bytes=%d max=2048", len(body))
	}
	if !reflect.DeepEqual(prepared, evt) {
		t.Fatalf("prepared event changed: got=%#v want=%#v", prepared, evt)
	}

	decoded, compressed, err := decodeWSBroadcastEnvelope(body)
	if err != nil {
		t.Fatalf("decode compressed event: %v", err)
	}
	if !compressed {
		t.Fatal("compressed event was reported as plain JSON")
	}
	if !reflect.DeepEqual(decoded, evt) {
		t.Fatalf("decoded event differs: got=%#v want=%#v", decoded, evt)
	}
}

func TestPrepareWSBroadcastEnvelopeDoesNotCompressBeforeRolloutIsEnabled(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"m": 9,
		"r": 27,
		"n": map[string]any{"state": strings.Repeat("x", 64<<10)},
	})
	evt := wsbus.NewTargetUsers("battle.run.batch", []int64{42}, payload, 123, "", "")

	_, body, compacted, err := prepareWSBroadcastEnvelopeForPublish(evt, 2048, false)
	if err == nil || !strings.Contains(err.Error(), "compression is disabled") {
		t.Fatalf("expected rollout-gate rejection, got compacted=%t bytes=%d err=%v", compacted, len(body), err)
	}
}

func TestPrepareWSBroadcastEnvelopeRejectsBattleBatchThatStillExceedsLimitAfterCompression(t *testing.T) {
	noise := make([]byte, 64<<10)
	state := uint32(0x12345678)
	for i := range noise {
		state = state*1664525 + 1013904223
		noise[i] = byte(state >> 24)
	}
	payload, _ := json.Marshal(map[string]any{"m": 9, "r": 27, "f": 10, "t": 20, "data": noise})
	evt := wsbus.NewTargetUsers("battle.run.batch", []int64{42}, payload, 123, "", "")

	_, _, compacted, err := prepareWSBroadcastEnvelopeForPublish(evt, 512, true)
	if err == nil || !strings.Contains(err.Error(), "compressed battle batch exceeds nats maximum") {
		t.Fatalf("expected compressed-size rejection, got compacted=%t err=%v", compacted, err)
	}
}

func TestPrepareWSBroadcastEnvelopeRejectsBattleBatchAboveTinyMaximum(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"m": 9, "r": 27, "data": strings.Repeat("x", 4096)})
	evt := wsbus.NewTargetUsers("battle.run.batch", []int64{42}, payload, 123, "", "")

	_, _, _, err := prepareWSBroadcastEnvelopeForPublish(evt, 64, true)
	if err == nil || !strings.Contains(err.Error(), "compressed battle batch exceeds nats maximum") {
		t.Fatalf("expected compressed-size rejection, got %v", err)
	}
}

func TestDecodeWSBroadcastEnvelopeAcceptsLegacyJSON(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"version": 17})
	evt := wsbus.NewTargetUsers("player.attributes.updated", []int64{42}, payload, 123, "", "")
	body, _ := json.Marshal(evt)

	decoded, compressed, err := decodeWSBroadcastEnvelope(body)
	if err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if compressed {
		t.Fatal("legacy JSON was reported as compressed")
	}
	if !reflect.DeepEqual(decoded, evt) {
		t.Fatalf("decoded event differs: got=%#v want=%#v", decoded, evt)
	}
}

func TestDecodeWSBroadcastEnvelopeRejectsCompressedNonBattleEvent(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"version": 17})
	evt := wsbus.NewTargetUsers("player.attributes.updated", []int64{42}, payload, 123, "", "")
	raw, _ := json.Marshal(evt)
	frame, err := encodeWSBroadcastCompressedFrame(raw)
	if err != nil {
		t.Fatal(err)
	}

	if _, compressed, err := decodeWSBroadcastEnvelope(frame); err == nil || !compressed {
		t.Fatalf("expected compressed non-battle rejection, compressed=%t err=%v", compressed, err)
	}
}

func TestDecodeWSBroadcastFrameRejectsMalformedFrames(t *testing.T) {
	t.Run("truncated header", func(t *testing.T) {
		frame := []byte(wsBroadcastFrameMagic)
		if _, compressed, err := decodeWSBroadcastFrame(frame); err == nil || !compressed {
			t.Fatalf("compressed=%t err=%v", compressed, err)
		}
	})

	t.Run("unsupported version", func(t *testing.T) {
		frame := make([]byte, wsBroadcastFrameHeaderBytes)
		copy(frame, wsBroadcastFrameMagic)
		frame[len(wsBroadcastFrameMagic)] = wsBroadcastFrameVersion + 1
		if _, compressed, err := decodeWSBroadcastFrame(frame); err == nil || !compressed {
			t.Fatalf("compressed=%t err=%v", compressed, err)
		}
	})

	t.Run("declared size above hard limit", func(t *testing.T) {
		frame := make([]byte, wsBroadcastFrameHeaderBytes)
		copy(frame, wsBroadcastFrameMagic)
		frame[len(wsBroadcastFrameMagic)] = wsBroadcastFrameVersion
		sizeOffset := len(wsBroadcastFrameMagic) + 1
		binary.BigEndian.PutUint32(frame[sizeOffset:sizeOffset+4], wsBroadcastFrameDecodedMaxBytes+1)
		if _, compressed, err := decodeWSBroadcastFrame(frame); err == nil || !compressed {
			t.Fatalf("compressed=%t err=%v", compressed, err)
		}
	})

	t.Run("invalid zlib", func(t *testing.T) {
		frame := make([]byte, wsBroadcastFrameHeaderBytes)
		copy(frame, wsBroadcastFrameMagic)
		frame[len(wsBroadcastFrameMagic)] = wsBroadcastFrameVersion
		frame = append(frame, []byte("not-zlib")...)
		if _, compressed, err := decodeWSBroadcastFrame(frame); err == nil || !compressed {
			t.Fatalf("compressed=%t err=%v", compressed, err)
		}
	})

	t.Run("declared size mismatch", func(t *testing.T) {
		frame, err := encodeWSBroadcastCompressedFrame([]byte("payload"))
		if err != nil {
			t.Fatal(err)
		}
		sizeOffset := len(wsBroadcastFrameMagic) + 1
		binary.BigEndian.PutUint32(frame[sizeOffset:sizeOffset+4], 1)
		if _, compressed, err := decodeWSBroadcastFrame(frame); err == nil || !compressed {
			t.Fatalf("compressed=%t err=%v", compressed, err)
		}
	})
}

func TestDecodeWSBroadcastFrameEnforcesDecodedHardLimit(t *testing.T) {
	tooLarge := bytes.Repeat([]byte{'x'}, wsBroadcastFrameDecodedMaxBytes+1)
	if _, err := encodeWSBroadcastCompressedFrame(tooLarge); err == nil {
		t.Fatal("encoder accepted payload above the decoded hard limit")
	}

	var frame bytes.Buffer
	frame.WriteString(wsBroadcastFrameMagic)
	frame.WriteByte(wsBroadcastFrameVersion)
	var declaredSize [4]byte
	binary.BigEndian.PutUint32(declaredSize[:], wsBroadcastFrameDecodedMaxBytes)
	frame.Write(declaredSize[:])
	zw, err := zlib.NewWriterLevel(&frame, zlib.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(tooLarge); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	if _, compressed, err := decodeWSBroadcastFrame(frame.Bytes()); err == nil || !compressed {
		t.Fatalf("expected zip bomb rejection, compressed=%t err=%v", compressed, err)
	}
}

func TestPublishWSBroadcastEnvelopeReportsLocalDelivery(t *testing.T) {
	s := &Server{}
	if !s.publishWSBroadcastEnvelope("game.ws.broadcast", wsBroadcastEvent{}) {
		t.Fatal("local delivery was reported as failed")
	}
	if s.publishWSBroadcastEnvelope(" ", wsBroadcastEvent{}) {
		t.Fatal("blank subject was reported as delivered")
	}
}

func TestPublishWSBattleBatchFlushFailureReturnsFalse(t *testing.T) {
	flushErr := errors.New("flush unavailable")
	nc := &wsNATSTestClient{
		connected:  true,
		maxPayload: 1 << 20,
		flushErr:   flushErr,
	}
	s := &Server{wsNATS: nc}
	evt := testWSBattleBatchEvent(t)

	if s.publishWSBroadcastEnvelope("game.ws.broadcast", evt) {
		t.Fatal("battle batch flush failure was reported as published")
	}
	if nc.publishCalls != 1 || nc.flushCalls != 1 {
		t.Fatalf("publish calls=%d flush calls=%d, want 1 each", nc.publishCalls, nc.flushCalls)
	}
	if nc.flushTimeout != wsBattleBatchFlushTimeout {
		t.Fatalf("flush timeout=%s, want %s", nc.flushTimeout, wsBattleBatchFlushTimeout)
	}
}

func TestPublishWSBattleBatchFlushSuccessReturnsTrue(t *testing.T) {
	nc := &wsNATSTestClient{connected: true, maxPayload: 1 << 20}
	s := &Server{wsNATS: nc}

	if !s.publishWSBroadcastEnvelope("game.ws.broadcast", testWSBattleBatchEvent(t)) {
		t.Fatal("flushed battle batch was reported as failed")
	}
	if nc.publishCalls != 1 || nc.flushCalls != 1 {
		t.Fatalf("publish calls=%d flush calls=%d, want 1 each", nc.publishCalls, nc.flushCalls)
	}
}

func TestPublishWSOrdinaryEventDoesNotFlush(t *testing.T) {
	nc := &wsNATSTestClient{connected: true, maxPayload: 1 << 20}
	s := &Server{wsNATS: nc}
	payload, err := json.Marshal(map[string]any{"version": 17})
	if err != nil {
		t.Fatal(err)
	}
	evt := wsbus.NewTargetUsers("player.attributes.updated", []int64{42}, payload, 123, "", "")

	if !s.publishWSBroadcastEnvelope("game.ws.broadcast", evt) {
		t.Fatal("ordinary event was reported as failed")
	}
	if nc.publishCalls != 1 || nc.flushCalls != 0 {
		t.Fatalf("publish calls=%d flush calls=%d, want publish=1 flush=0", nc.publishCalls, nc.flushCalls)
	}
}

func TestPublishWSTargetedBattleBatchFlushesOnceForMultipleRoutes(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	nc := &wsNATSTestClient{connected: true, maxPayload: 1 << 20}
	s := &Server{
		deps:              app.Dependencies{Redis: redisClient},
		wsNATS:            nc,
		wsSubject:         "game.ws.broadcast",
		wsNodeSubject:     "game.ws.broadcast.node",
		wsTargetedRouting: true,
		wsNodeAdvertise:   "publisher:8080",
	}
	ctx := context.Background()
	const userID int64 = 42
	nodes := []string{"10.0.0.1:8080", "10.0.0.2:8080"}
	if err := redisClient.SAdd(ctx, s.wsRouteUserDirKey(userID), nodes[0], nodes[1]).Err(); err != nil {
		t.Fatal(err)
	}
	for _, nodeAddr := range nodes {
		if err := redisClient.Set(ctx, s.wsRouteNodeKey(userID, nodeAddr), "1", time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
	}

	payload := map[string]any{"m": 9, "r": 27, "v": []any{}}
	if !s.publishWSTargetUsers("battle.run.batch", []int64{userID}, payload, "", "") {
		t.Fatal("targeted battle batch was reported as failed")
	}
	if nc.publishCalls != len(nodes) || nc.flushCalls != 1 {
		t.Fatalf("publish calls=%d flush calls=%d, want publish=%d flush=1", nc.publishCalls, nc.flushCalls, len(nodes))
	}
}

func TestBattleRunIDFromWirePayloadReadsCompactHeaderWithoutDecodingTrailingBatch(t *testing.T) {
	payload := json.RawMessage(`{"m":9,"r":27,"v":[{"d":"` + strings.Repeat("x", 1<<20) + `"}]}`)
	if got := battleRunIDFromWirePayload(payload); got != 27 {
		t.Fatalf("run id=%d, want 27", got)
	}
	if got := battleRunIDFromWirePayload(json.RawMessage(`{"roomId":9,"runId":31,"events":[]}`)); got != 31 {
		t.Fatalf("legacy run id=%d, want 31", got)
	}
	for _, malformed := range []json.RawMessage{nil, json.RawMessage(`[]`), json.RawMessage(`{"r":"bad"}`)} {
		if got := battleRunIDFromWirePayload(malformed); got != 0 {
			t.Fatalf("malformed payload %q returned run id=%d", malformed, got)
		}
	}
}

func testWSBattleBatchEvent(t *testing.T) wsBroadcastEvent {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"m": 9, "r": 27, "v": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	return wsbus.NewTargetUsers("battle.run.batch", []int64{42}, payload, 123, "", "")
}

type wsNATSTestClient struct {
	connected    bool
	maxPayload   int64
	publishErr   error
	flushErr     error
	publishCalls int
	flushCalls   int
	flushTimeout time.Duration
}

func (c *wsNATSTestClient) IsConnected() bool {
	return c.connected
}

func (c *wsNATSTestClient) MaxPayload() int64 {
	return c.maxPayload
}

func (c *wsNATSTestClient) Publish(_ string, _ []byte) error {
	c.publishCalls++
	return c.publishErr
}

func (c *wsNATSTestClient) FlushTimeout(timeout time.Duration) error {
	c.flushCalls++
	c.flushTimeout = timeout
	return c.flushErr
}
