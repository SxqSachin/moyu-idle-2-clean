package store

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"time"
)

// Freeze once at run creation. Hot reads use immutable leased windows; cold
// reads acquire only source locks, after any battle entry inventory locks.
func (s *Store) freezeBattlePassiveTx(ctx context.Context, tx pgx.Tx, roomID int64, memberIDs ...int64) (string, time.Time, error) {
	now := time.Now().UTC()
	if len(s.gameData.CatteryFurniture) == 0 {
		return "{}", now, nil
	}
	// Runtime callers already loaded the roster while locking the room. The
	// fallback is retained for explicit snapshot readers, not the battle hot path.
	if len(memberIDs) == 0 {
		rows, err := tx.Query(ctx, `SELECT user_id FROM battle_room_members WHERE room_id=$1`, roomID)
		if err != nil {
			return "", now, err
		}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return "", now, err
			}
			memberIDs = append(memberIDs, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", now, err
		}
	}
	windows, err := s.furnitureWindowsTx(ctx, tx, memberIDs, now, now)
	if err != nil {
		return "", now, err
	}
	snapshots := map[int64]operationeffect.PassiveSnapshot{}
	for id, list := range windows {
		for _, window := range list {
			if now.Before(window.From) || (!window.Until.IsZero() && !now.Before(window.Until)) {
				continue
			}
			source := window.Source
			filtered := source
			filtered.Rules = nil
			for _, r := range source.Rules {
				if r.Domain == operationeffect.DomainBattleReward {
					filtered.Rules = append(filtered.Rules, r)
				}
			}
			if len(filtered.Rules) == 0 {
				continue
			}
			snap := snapshots[id]
			snap.Version = 1
			snap.Sources = append(snap.Sources, filtered)
			if err = snap.Validate(); err != nil {
				return "", now, err
			}
			snapshots[id] = snap
		}
	}
	if len(snapshots) == 0 {
		return "{}", now, nil
	}
	raw, err := json.Marshal(map[string]any{"passiveSources": snapshots})
	return string(raw), now, err
}
