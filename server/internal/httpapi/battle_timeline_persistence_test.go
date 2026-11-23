package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func TestMarshalBattleTimelineForPersistenceMatchesLegacyJSON(t *testing.T) {
	timeline, playback := battleTimelinePersistenceFixture(t, 8)
	if _, ok := battleTimelineRowsWithPlaybackPayloads(timeline, playback); !ok {
		t.Fatal("timeline generated with playback payloads did not use the persistence fast path")
	}
	want, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal legacy timeline: %v", err)
	}
	timelineJSON, err := marshalBattleTimelineForPersistence(timeline, playback)
	if err != nil {
		t.Fatalf("marshal timeline with playback payloads: %v", err)
	}
	got := timelineJSON.CloneBytes()
	if !bytes.Equal(got, want) {
		t.Fatalf("timeline JSON changed\ngot:  %s\nwant: %s", got, want)
	}
	if _, ok := timeline[0]["payload"].(map[string]any); !ok {
		t.Fatalf("in-memory timeline payload type changed: %T", timeline[0]["payload"])
	}
}

func TestEncodedBattleTimelineUsesPersistenceFastPath(t *testing.T) {
	events := []battleEvent{
		{
			Seq:  10,
			Type: "battle.wave.spawn",
			TS:   1_000,
			Data: map[string]any{
				"wave":          7,
				"players":       []map[string]any{{"unitId": "player:1", "hp": 100.0}},
				"enemies":       []map[string]any{{"unitId": "enemy:1", "hp": 80.0}},
				"escapedText":   "<battle>&result",
				"lineSeparator": "\u2028",
				"optional":      nil,
			},
		},
		{
			Seq:  11,
			Type: "battle.ended",
			TS:   3_000,
			Data: map[string]any{"reason": "wave_cleared"},
		},
	}
	timeline, playback := encodeBattleTimelineWithPlayback(events, 1_000, 2_000)
	if len(timeline) != len(events) || len(playback) != len(events) {
		t.Fatalf("encoded timeline lengths=(%d,%d), want %d", len(timeline), len(playback), len(events))
	}
	if _, ok := battleTimelineRowsWithPlaybackPayloads(timeline, playback); !ok {
		t.Fatal("encoded battle timeline did not satisfy persistence fast-path invariants")
	}
	want, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal legacy timeline: %v", err)
	}
	timelineJSON, err := marshalBattleTimelineForPersistence(timeline, playback)
	if err != nil {
		t.Fatalf("marshal timeline with playback payloads: %v", err)
	}
	got := timelineJSON.CloneBytes()
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded timeline JSON changed\ngot:  %s\nwant: %s", got, want)
	}
	var legacyDecoded, optimizedDecoded []map[string]any
	if err := json.Unmarshal(want, &legacyDecoded); err != nil {
		t.Fatalf("decode legacy timeline: %v", err)
	}
	if err := json.Unmarshal(got, &optimizedDecoded); err != nil {
		t.Fatalf("decode optimized timeline: %v", err)
	}
	if !reflect.DeepEqual(optimizedDecoded, legacyDecoded) {
		t.Fatalf("decoded timeline structure changed\ngot:  %#v\nwant: %#v", optimizedDecoded, legacyDecoded)
	}
	if !reflect.DeepEqual(decodeBattleTimeline(optimizedDecoded), decodeBattleTimeline(legacyDecoded)) {
		t.Fatal("playback timeline structure changed after persistence round trip")
	}
}

func TestMarshalBattleTimelineForPersistenceFallsBackSafely(t *testing.T) {
	timeline, playback := battleTimelinePersistenceFixture(t, 2)
	want, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal legacy timeline: %v", err)
	}

	tests := []struct {
		name     string
		playback []wsbus.BattleTimelineEvent
	}{
		{name: "missing row", playback: playback[:1]},
		{name: "invalid payload", playback: append([]wsbus.BattleTimelineEvent(nil), playback...)},
		{name: "malformed object payload", playback: append([]wsbus.BattleTimelineEvent(nil), playback...)},
		{name: "sequence mismatch", playback: append([]wsbus.BattleTimelineEvent(nil), playback...)},
		{name: "metadata mismatch", playback: append([]wsbus.BattleTimelineEvent(nil), playback...)},
		{name: "event type mismatch", playback: append([]wsbus.BattleTimelineEvent(nil), playback...)},
		{name: "data identity mismatch", playback: append([]wsbus.BattleTimelineEvent(nil), playback...)},
	}
	tests[1].playback[0].Payload = json.RawMessage(`not-json`)
	tests[2].playback[0].Payload = json.RawMessage(`{not-json}`)
	tests[3].playback[0].Seq++
	tests[4].playback[0].OffsetMS++
	tests[5].playback[0].EventType = "battle.timeline.other"
	tests[6].playback[0].Data = map[string]any{"different": true}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			timelineJSON, err := marshalBattleTimelineForPersistence(timeline, tc.playback)
			if err != nil {
				t.Fatalf("marshal timeline: %v", err)
			}
			got := timelineJSON.CloneBytes()
			if !bytes.Equal(got, want) {
				t.Fatalf("fallback timeline JSON changed\ngot:  %s\nwant: %s", got, want)
			}
		})
	}
}

func TestMarshalBattleTimelineForPersistencePreservesEmptyShapes(t *testing.T) {
	tests := []struct {
		name     string
		timeline []map[string]any
		want     string
	}{
		{name: "nil", timeline: nil, want: "null"},
		{name: "empty array", timeline: []map[string]any{}, want: "[]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			timelineJSON, err := marshalBattleTimelineForPersistence(tc.timeline, []wsbus.BattleTimelineEvent{})
			if err != nil {
				t.Fatalf("marshal timeline: %v", err)
			}
			if got := string(timelineJSON.CloneBytes()); got != tc.want {
				t.Fatalf("timeline JSON=%s, want %s", got, tc.want)
			}
		})
	}
}

func BenchmarkMarshalBattleTimelineForPersistence(b *testing.B) {
	timeline, playback := battleTimelinePersistenceFixture(b, 256)
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			raw, err := json.Marshal(timeline)
			if err != nil {
				b.Fatal(err)
			}
			battleTimelinePersistenceBenchmarkSink = raw
		}
	})
	b.Run("reuse_playback_payloads", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			timelineJSON, err := marshalBattleTimelineForPersistence(timeline, playback)
			if err != nil {
				b.Fatal(err)
			}
			battleTimelineJSONBenchmarkSink = timelineJSON
		}
	})
}

var battleTimelinePersistenceBenchmarkSink []byte
var battleTimelineJSONBenchmarkSink store.BattleRunTimelineJSON

func battleTimelinePersistenceFixture(tb testing.TB, count int) ([]map[string]any, []wsbus.BattleTimelineEvent) {
	tb.Helper()
	timeline := make([]map[string]any, 0, count)
	playback := make([]wsbus.BattleTimelineEvent, 0, count)
	for index := 0; index < count; index++ {
		seq := int64(index + 1)
		eventType := "battle.timeline.node"
		data := map[string]any{
			"actorId": fmt.Sprintf("player:%d", index%8+1),
			"steps": []map[string]any{
				{
					"k": "d",
					"s": fmt.Sprintf("player:%d", index%8+1),
					"t": []map[string]any{
						{"u": "enemy:1", "b": 1000.0, "h": 875.25, "m": 40.0, "a": true},
						{"u": "enemy:2", "b": 900.0, "h": 812.75, "m": 35.0, "a": true},
					},
					"meta": map[string]any{"skillId": "basic_attack", "crit": index%3 == 0},
				},
			},
			"players": []map[string]any{
				{"u": "player:1", "h": 1000.0, "m": 100.0, "a": true},
				{"u": "player:2", "h": 980.0, "m": 120.0, "a": true},
			},
			"order": []string{"player:1", "enemy:1", "player:2", "enemy:2"},
		}
		payload := map[string]any{
			"eventType": eventType,
			"ts":        int64(1_000 + index*2_000),
			"data":      data,
		}
		payloadRaw, err := json.Marshal(payload)
		if err != nil {
			tb.Fatalf("marshal fixture payload: %v", err)
		}
		timeline = append(timeline, map[string]any{
			"seq":              seq,
			"offsetMs":         int64(index * 2_000),
			"lane":             "main",
			"phase":            "action",
			"orderKey":         fmt.Sprintf("%020d:%020d", 1_000+index*2_000, seq),
			"actionDurationMs": int64(0),
			"payload":          payload,
		})
		playback = append(playback, wsbus.BattleTimelineEvent{
			Seq:              seq,
			OffsetMS:         int64(index * 2_000),
			Lane:             "main",
			Phase:            "action",
			OrderKey:         fmt.Sprintf("%020d:%020d", 1_000+index*2_000, seq),
			ActionDurationMS: 0,
			Payload:          payloadRaw,
			EventType:        eventType,
			Data:             data,
		})
	}
	return timeline, playback
}
