package httpapi

import (
	"context"
	"encoding/json"
	"strings"
)

const playerProfileChangedEventType = "player.profile.changed"

type playerProfileChangedPayload struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	ChangeID int64  `json:"changeId"`
}

func newPlayerProfileChangedPayload(userID int64, username string, changeID int64) (playerProfileChangedPayload, bool) {
	payload := playerProfileChangedPayload{
		UserID:   userID,
		Username: strings.TrimSpace(username),
		ChangeID: changeID,
	}
	return payload, payload.UserID > 0 && payload.Username != "" && payload.ChangeID > 0
}

func (s *Server) broadcastPlayerProfileChanged(ctx context.Context, userID int64, username string, changeID int64) {
	payload, ok := newPlayerProfileChangedPayload(userID, username, changeID)
	if !ok {
		return
	}
	s.broadcastWSCluster(playerProfileChangedEventType, payload, ctx)
}

// applyPlayerProfileChangedEvent runs on every gateway subscribed to the shared
// WS subject, including nodes without a local socket for the changed player.
func (s *Server) applyPlayerProfileChangedEvent(raw json.RawMessage) {
	var payload playerProfileChangedPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.UserID <= 0 {
		return
	}
	s.invalidatePlayerProfileCaches(payload.UserID)
}
