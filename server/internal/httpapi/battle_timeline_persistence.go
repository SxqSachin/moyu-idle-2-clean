package httpapi

import (
	"bytes"
	"encoding/json"
	"reflect"
	"time"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func (s *Server) waitBattlePlaybackDelay(delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-s.battleStopCh:
		return false
	}
}

// marshalBattleTimelineForPersistence reuses payload JSON produced for
// playback while leaving the in-memory timeline maps unchanged.
func marshalBattleTimelineForPersistence(timeline []map[string]any, playback []wsbus.BattleTimelineEvent) (store.BattleRunTimelineJSON, error) {
	if timeline == nil {
		return store.MarshalBattleRunTimelineJSON(timeline)
	}
	rows, ok := battleTimelineRowsWithPlaybackPayloads(timeline, playback)
	if ok {
		if timelineJSON, err := store.MarshalBattleRunTimelineJSON(rows); err == nil {
			return timelineJSON, nil
		}
	}
	return store.MarshalBattleRunTimelineJSON(timeline)
}

func battleTimelineRowsWithPlaybackPayloads(timeline []map[string]any, playback []wsbus.BattleTimelineEvent) ([]map[string]any, bool) {
	if len(timeline) != len(playback) {
		return nil, false
	}
	rows := make([]map[string]any, len(timeline))
	for index, item := range timeline {
		if item == nil {
			return nil, false
		}
		payload, ok := item["payload"].(map[string]any)
		if !ok {
			return nil, false
		}
		event := playback[index]
		if !battleTimelineRowMatchesPlaybackEvent(item, payload, event) {
			return nil, false
		}
		payloadRaw := bytes.TrimSpace(event.Payload)
		if len(payloadRaw) < 2 || payloadRaw[0] != '{' || payloadRaw[len(payloadRaw)-1] != '}' {
			return nil, false
		}
		row := make(map[string]any, len(item))
		for key, value := range item {
			row[key] = value
		}
		row["payload"] = json.RawMessage(event.Payload)
		rows[index] = row
	}
	return rows, true
}

func battleTimelineRowMatchesPlaybackEvent(item map[string]any, payload map[string]any, event wsbus.BattleTimelineEvent) bool {
	seq, seqExists := item["seq"]
	offsetMS, offsetExists := item["offsetMs"]
	lane, laneOK := item["lane"].(string)
	phase, phaseOK := item["phase"].(string)
	orderKey, orderKeyOK := item["orderKey"].(string)
	actionDurationMS, durationExists := item["actionDurationMs"]
	eventType, eventTypeOK := payload["eventType"].(string)
	data, dataOK := payload["data"].(map[string]any)
	if !seqExists || toInt64Loose(seq, -1) != event.Seq ||
		!offsetExists || toInt64Loose(offsetMS, -1) != event.OffsetMS ||
		!laneOK || lane != event.Lane ||
		!phaseOK || phase != event.Phase ||
		!orderKeyOK || orderKey != event.OrderKey ||
		!durationExists || toInt64Loose(actionDurationMS, -1) != event.ActionDurationMS ||
		!eventTypeOK || eventType != event.EventType ||
		!dataOK || reflect.ValueOf(data).Pointer() != reflect.ValueOf(event.Data).Pointer() {
		return false
	}
	parentSeq, parentExists := item["parentSeq"]
	if event.ParentSeq == nil {
		if parentExists && parentSeq != nil {
			return false
		}
	} else if !parentExists || toInt64Loose(parentSeq, -1) != *event.ParentSeq {
		return false
	}
	groupID, groupExists := item["groupId"]
	if event.GroupID == "" {
		return !groupExists || groupID == nil || groupID == ""
	}
	groupIDString, ok := groupID.(string)
	return groupExists && ok && groupIDString == event.GroupID
}
