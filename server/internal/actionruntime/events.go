package actionruntime

import (
	"fmt"
	"strings"

	"mysecondwebgame/server/internal/store"
)

const SettledTopic = "action.runtime.settled"

type SettledEvent struct {
	UserID           int64                          `json:"userId"`
	Source           string                         `json:"source,omitempty"`
	PendingID        string                         `json:"pendingId,omitempty"`
	PendingAtMS      int64                          `json:"pendingAtMs,omitempty"`
	EquipmentChanged bool                           `json:"equipmentChanged,omitempty"`
	Summary          store.ActionFastForwardSummary `json:"summary"`
}

func SettledSubject(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return SettledTopic
	}
	return fmt.Sprintf("%s.%s", prefix, SettledTopic)
}

func EquipmentChanged(summary store.ActionFastForwardSummary) bool {
	if len(summary.EnhanceResults) > 0 ||
		len(summary.ActivatedWorkLoadouts) > 0 {
		return true
	}
	for _, actor := range summary.ActorSummaries {
		if actor.EquipmentChanged {
			return true
		}
	}
	return false
}
