package wsbus

import (
	"encoding/json"
	"strings"
)

const (
	TargetTypeBroadcastAll = "broadcast_all"
	TargetTypeUsers        = "target_users"
	EventCategoryGlobal    = "global_broadcast"
	EventCategoryControl   = "must_deliver_control"
	EventCategoryUserState = "user_state"
)

type Event struct {
	EventType       string          `json:"eventType"`
	TargetType      string          `json:"targetType"`
	Category        string          `json:"category,omitempty"`
	UserIDs         []int64         `json:"userIds,omitempty"`
	RequestID       string          `json:"requestId,omitempty"`
	ExcludeClientID string          `json:"excludeClientId,omitempty"`
	TS              int64           `json:"ts"`
	Payload         json.RawMessage `json:"payload"`
	Type            string          `json:"type,omitempty"`
	UserID          int64           `json:"userId,omitempty"`
}

type BattleTimelineEvent struct {
	Seq              int64           `json:"seq"`
	OffsetMS         int64           `json:"offsetMs"`
	Lane             string          `json:"lane"`
	Phase            string          `json:"phase"`
	ParentSeq        *int64          `json:"parentSeq,omitempty"`
	GroupID          string          `json:"groupId,omitempty"`
	OrderKey         string          `json:"orderKey"`
	ActorID          string          `json:"actorId,omitempty"`
	ActorUserID      int64           `json:"actorUserId,omitempty"`
	HookID           string          `json:"hookId,omitempty"`
	ActionDurationMS int64           `json:"actionDurationMs"`
	Payload          json.RawMessage `json:"payload"`
	EventType        string          `json:"-"`
	Data             map[string]any  `json:"-"`
}

type BattleEventBatchPayload struct {
	RunID         int64                 `json:"runId"`
	FromSeq       int64                 `json:"fromSeq"`
	ToSeq         int64                 `json:"toSeq"`
	WindowStartMS int64                 `json:"windowStartMs"`
	WindowEndMS   int64                 `json:"windowEndMs"`
	Events        []BattleTimelineEvent `json:"events"`
	Snapshot      map[string]any        `json:"snapshot,omitempty"`
}

func BroadcastSubject(prefix string) string {
	base := "game"
	prefix = strings.TrimSpace(prefix)
	if prefix != "" {
		base = prefix
	}
	return base + ".ws.broadcast"
}

func NewBroadcastAll(eventType string, payload json.RawMessage, ts int64) Event {
	return Event{
		EventType:  strings.TrimSpace(eventType),
		TargetType: TargetTypeBroadcastAll,
		Category:   EventCategoryGlobal,
		TS:         ts,
		Payload:    payload,
	}
}

func NewTargetUsers(eventType string, userIDs []int64, payload json.RawMessage, ts int64, requestID string, excludeClientID string) Event {
	return Event{
		EventType:       strings.TrimSpace(eventType),
		TargetType:      TargetTypeUsers,
		Category:        EventCategoryUserState,
		UserIDs:         userIDs,
		RequestID:       strings.TrimSpace(requestID),
		ExcludeClientID: strings.TrimSpace(excludeClientID),
		TS:              ts,
		Payload:         payload,
	}
}
