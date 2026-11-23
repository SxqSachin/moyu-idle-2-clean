package httpapi

import (
	"encoding/json"
	"testing"
)

func TestBattleTimelineStepFromActionRowPreservesCritAndTargetState(t *testing.T) {
	step := battleTimelineStepFromActionRow(map[string]any{
		"kind":       "damage",
		"sourceId":   "player:1",
		"targetId":   "enemy:1",
		"amount":     123.45678,
		"crit":       true,
		"damageType": "magical",
		"beforeHp":   300.12,
		"afterHp":    176.66321,
		"afterMp":    42.5,
		"afterAlive": true,
		"maxHpAfter": 500.0,
		"maxMpAfter": 80.0,
	})

	if got := step["k"]; got != "d" {
		t.Fatalf("kind code mismatch: got=%v want=d", got)
	}
	if got, ok := step["c"].(bool); !ok || !got {
		t.Fatalf("step crit should be true, got=%#v", step["c"])
	}
	if got := step["dt"]; got != "magical" {
		t.Fatalf("step damageType mismatch: got=%v want=magical", got)
	}
	if got := step["v"]; got != roundTo4(123.45678) {
		t.Fatalf("step amount mismatch: got=%v want=%v", got, roundTo4(123.45678))
	}

	targets, ok := step["t"].([]map[string]any)
	if !ok || len(targets) != 1 {
		t.Fatalf("step targets mismatch: %#v", step["t"])
	}
	target := targets[0]
	if got := target["u"]; got != "enemy:1" {
		t.Fatalf("target id mismatch: got=%v want=enemy:1", got)
	}
	if got, ok := target["c"].(bool); !ok || !got {
		t.Fatalf("target crit should be true, got=%#v", target["c"])
	}
	if got := target["b"]; got != roundTo4(300.12) {
		t.Fatalf("target beforeHp mismatch: got=%v want=%v", got, roundTo4(300.12))
	}
	if got := target["h"]; got != roundTo4(176.66321) {
		t.Fatalf("target afterHp mismatch: got=%v want=%v", got, roundTo4(176.66321))
	}
	if got := target["m"]; got != roundTo4(42.5) {
		t.Fatalf("target afterMp mismatch: got=%v want=%v", got, roundTo4(42.5))
	}
	if got, ok := target["l"].(bool); !ok || !got {
		t.Fatalf("target alive mismatch: got=%#v", target["l"])
	}
	if got := target["x"]; got != roundTo4(500.0) {
		t.Fatalf("target maxHp mismatch: got=%v want=%v", got, roundTo4(500.0))
	}
	if got := target["y"]; got != roundTo4(80.0) {
		t.Fatalf("target maxMp mismatch: got=%v want=%v", got, roundTo4(80.0))
	}
}

func TestBattleTimelineStepFromActionRowParsesLooseCritValue(t *testing.T) {
	step := battleTimelineStepFromActionRow(map[string]any{
		"kind":     "damage",
		"sourceId": "player:1",
		"targetId": "enemy:1",
		"amount":   10,
		"crit":     "1",
	})
	if got, ok := step["c"].(bool); !ok || !got {
		t.Fatalf("step crit should parse from string, got=%#v", step["c"])
	}
	targets, _ := step["t"].([]map[string]any)
	if len(targets) != 1 {
		t.Fatalf("target list mismatch: %#v", step["t"])
	}
	if got, ok := targets[0]["c"].(bool); !ok || !got {
		t.Fatalf("target crit should parse from string, got=%#v", targets[0]["c"])
	}
}

func TestBattleTimelineStepFromActionRowPreservesChargedBuffProgress(t *testing.T) {
	step := battleTimelineStepFromActionRow(map[string]any{
		"kind":              "buff_charge",
		"sourceId":          "player:1",
		"targetId":          "enemy:1",
		"buffId":            "parade_three_act_curtain",
		"buffDurationTurns": 99,
		"buffTriggered":     1,
	})

	if got := step["k"]; got != "bc" {
		t.Fatalf("kind code mismatch: got=%v want=bc", got)
	}
	if got := step["bi"]; got != "parade_three_act_curtain" {
		t.Fatalf("buff id mismatch: got=%v", got)
	}
	if got := step["bt"]; got != int64(99) {
		t.Fatalf("buff duration mismatch: got=%v want=99", got)
	}
	if got := step["bg"]; got != int64(1) {
		t.Fatalf("buff progress mismatch: got=%v want=1", got)
	}
	if got := battleTimelineKindFromCode("bc"); got != "buff_charge" {
		t.Fatalf("kind expansion mismatch: got=%v want=buff_charge", got)
	}
}

func TestCompactBattleSnapshotBuffsPreservesChargedBuffProgress(t *testing.T) {
	rows := compactBattleSnapshotBuffs([]map[string]any{
		{
			"buffId":      "parade_three_act_curtain",
			"remainTurns": 99,
			"triggered":   2,
		},
	})

	if len(rows) != 1 {
		t.Fatalf("snapshot buffs mismatch: %#v", rows)
	}
	if got := rows[0]["g"]; got != int64(2) {
		t.Fatalf("snapshot buff progress mismatch: got=%v want=2", got)
	}
}

func TestEncodeBattleTimelineKeepsSyncedSummonChildrenInParentGroup(t *testing.T) {
	events := []battleEvent{
		{
			Seq:  10,
			Type: "battle.turn.action",
			TS:   1000,
			Data: map[string]any{
				"wave":       1,
				"actorId":    "player:owner",
				"skillId":    "basic_attack",
				"actorHp":    100,
				"actorMp":    10,
				"actorAlive": true,
				"actionLog":  []map[string]any{},
				"hooks":      []map[string]any{},
				"order":      []string{"player:owner", "enemy:1"},
			},
		},
		{
			Seq:  11,
			Type: "battle.turn.action",
			TS:   1000,
			Data: map[string]any{
				"wave":       1,
				"actorId":    "player:summon",
				"skillId":    "basic_attack",
				"actorHp":    50,
				"actorMp":    5,
				"actorAlive": true,
				"actionLog":  []map[string]any{},
				"hooks":      []map[string]any{},
				"parentSeq":  int64(10),
				"order":      []string{"player:owner", "enemy:1"},
			},
		},
	}

	raw, playback := encodeBattleTimelineWithPlayback(events, 0, 1000)
	if len(raw) != 2 {
		t.Fatalf("timeline length=%d, want 2: %#v", len(raw), raw)
	}
	if len(playback) != 2 {
		t.Fatalf("playback timeline length=%d, want 2: %#v", len(playback), playback)
	}
	parentSeq := toInt64Loose(raw[0]["seq"], 0)
	childParentSeq := toInt64Loose(raw[1]["parentSeq"], 0)
	if childParentSeq != parentSeq {
		t.Fatalf("child parentSeq=%d, want parent timeline seq=%d raw=%#v", childParentSeq, parentSeq, raw[1])
	}
	if playback[1].ParentSeq == nil || *playback[1].ParentSeq != parentSeq {
		t.Fatalf("playback child parentSeq=%v, want %d", playback[1].ParentSeq, parentSeq)
	}
	if got, want := toInt64Loose(raw[1]["offsetMs"], -1), toInt64Loose(raw[0]["offsetMs"], -2); got != want {
		t.Fatalf("child offset=%d, want parent offset=%d", got, want)
	}
	if got, want := raw[1]["groupId"], "sync:1"; got != want {
		t.Fatalf("child groupId=%v, want %s", got, want)
	}
	if got, want := playback[1].GroupID, "sync:1"; got != want {
		t.Fatalf("playback child groupId=%q, want %s", got, want)
	}
	if playback[0].EventType != "battle.timeline.node" || playback[0].Data == nil {
		t.Fatalf("playback payload cache missing: %#v", playback[0])
	}
	encodedPlayback, err := json.Marshal(playback[0])
	if err != nil {
		t.Fatalf("marshal playback event: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encodedPlayback, &wire); err != nil {
		t.Fatalf("unmarshal playback event wire: %v", err)
	}
	if _, exists := wire["EventType"]; exists {
		t.Fatalf("cached EventType leaked into wire payload: %s", string(encodedPlayback))
	}
	if _, exists := wire["Data"]; exists {
		t.Fatalf("cached Data leaked into wire payload: %s", string(encodedPlayback))
	}

	decoded := decodeBattleTimeline(raw)
	if len(decoded) != 2 || decoded[1].ParentSeq == nil || *decoded[1].ParentSeq != decoded[0].Seq {
		t.Fatalf("decoded parent link mismatch: %#v", decoded)
	}
	if decoded[1].GroupID != "sync:1" {
		t.Fatalf("decoded groupId=%q, want sync:1", decoded[1].GroupID)
	}
	if decoded[0].EventType != "battle.timeline.node" || decoded[0].Data == nil {
		t.Fatalf("decoded payload cache missing: %#v", decoded[0])
	}
}
