package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
)

// resolveCollectibleLevelMods computes cumulative player attribute bonuses for an upgradeable
// collectible at the given level by summing per-level increments from the LevelAttrCurve.
// Level 0 returns an empty map. Reuses resolveEnhanceAttrIncrement from the enhance package.
func resolveCollectibleLevelMods(def *catalog.Collectible, level int) map[string]float64 {
	out := make(map[string]float64)
	for l := 1; l <= level; l++ {
		inc, err := resolveEnhanceAttrIncrement(def.LevelAttrCurve, l, def.LevelMaxLevel)
		if err != nil {
			break
		}
		for k, v := range inc {
			out[k] += v
		}
	}
	return out
}

// AddCollectible handles both initial grant (level=1) and level-up for upgradeable collectibles.
// Returns (newLevel, nil) on success.
// - First use: inserts the collectible at level=1, consumes the item, recalculates stats.
// - Subsequent uses on upgradeable collectibles: increments level, recalculates stats.
// - Subsequent uses on non-upgradeable collectibles: returns ErrCollectibleAlreadyOwned.
// - Level already at cap: returns ErrCollectibleMaxLevel.
func (s *Store) AddCollectible(ctx context.Context, userID int64, instanceID int64, collectible catalog.Collectible) (newLevel int, err error) {
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.LedgerConsumeInstanceTx(ctx, tx, userID, instanceID, collectible.ItemID, 1, InventoryLedgerMeta{
			Category:      "collectible",
			SourceSurface: "inventory",
			TargetSurface: "collectible_collection",
			Details:       map[string]any{"collectibleId": collectible.ID},
		}); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, `
			INSERT INTO player_collectibles(user_id, collectible_id, level, obtained_at)
			VALUES($1, $2, 1, now())
			ON CONFLICT DO NOTHING
		`, userID, collectible.ID)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 1 {
			// First grant: level = 1
			newLevel = 1
			needsRecompute := len(collectible.PlayerMods) > 0 || collectible.Upgradeable
			if needsRecompute {
				if err := s.recomputeDerivedStatsTx(ctx, tx, userID); err != nil {
					return err
				}
			}
			if len(collectible.KittyMods) > 0 {
				kittyIDs, err := s.listPlayerKittyIDsTx(ctx, tx, userID)
				if err != nil {
					return err
				}
				seeds := s.attributeSeedsFromGameData()
				for _, kittyID := range kittyIDs {
					if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// Conflict: already owned
		if !collectible.Upgradeable {
			return ErrCollectibleAlreadyOwned
		}

		var currentLevel int
		if err := tx.QueryRow(ctx, `
			UPDATE player_collectibles SET level = level + 1
			WHERE user_id=$1 AND collectible_id=$2
			RETURNING level
		`, userID, collectible.ID).Scan(&currentLevel); err != nil {
			return err
		}
		newLevel = currentLevel

		if collectible.LevelMaxLevel > 0 && newLevel > collectible.LevelMaxLevel {
			return ErrCollectibleMaxLevel
		}

		if err := s.recomputeDerivedStatsTx(ctx, tx, userID); err != nil {
			return err
		}

		return nil
	})
	return newLevel, err
}

func (s *Store) listPlayerKittyIDsTx(ctx context.Context, tx pgx.Tx, userID int64) ([]int64, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM player_kitties WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) GetCollectibles(ctx context.Context, userID int64) ([]PlayerCollectible, error) {
	rows, err := s.db.Query(ctx, `
		SELECT collectible_id, level, obtained_at FROM player_collectibles WHERE user_id = $1 ORDER BY obtained_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PlayerCollectible, 0)
	for rows.Next() {
		var pc PlayerCollectible
		if err := rows.Scan(&pc.CollectibleID, &pc.Level, &pc.ObtainedAt); err != nil {
			return nil, err
		}
		out = append(out, pc)
	}
	return out, rows.Err()
}

// loadCollectiblePlayerModsTx reads player_collectibles, looks up DSL definitions,
// and returns per-collectible-per-attr contributions for source breakdown display.
// For upgradeable collectibles the bonus is level-based; for others it uses static PlayerMods.
func (s *Store) loadCollectiblePlayerModsTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string][]collectibleAttrContribution, map[string]float64, error) {
	rows, err := tx.Query(ctx, `
		SELECT collectible_id, level FROM player_collectibles WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byAttr := make(map[string][]collectibleAttrContribution)
	totals := make(map[string]float64)
	for rows.Next() {
		var id string
		var level int
		if err := rows.Scan(&id, &level); err != nil {
			return nil, nil, err
		}
		def := s.gameData.CollectibleByID(strings.TrimSpace(id))
		if def == nil {
			continue
		}

		var mods map[string]float64
		if def.Upgradeable {
			mods = resolveCollectibleLevelMods(def, level)
		} else {
			mods = def.PlayerMods
		}

		for attrKey, modValue := range mods {
			key := strings.TrimSpace(attrKey)
			if key == "" || modValue == 0 {
				continue
			}
			byAttr[key] = append(byAttr[key], collectibleAttrContribution{
				CollectibleID: def.ID,
				NameKey:       def.NameKey,
				Value:         modValue,
			})
			totals[key] += modValue
		}
	}
	return byAttr, totals, rows.Err()
}

// loadCollectibleKittyModsTx same as above but for kittyMods.
// Upgradeable collectibles with level-based kittyMods are handled here for future extensibility.
func (s *Store) loadCollectibleKittyModsTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string][]collectibleAttrContribution, map[string]float64, error) {
	rows, err := tx.Query(ctx, `
		SELECT collectible_id, level FROM player_collectibles WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byAttr := make(map[string][]collectibleAttrContribution)
	totals := make(map[string]float64)
	for rows.Next() {
		var id string
		var level int
		if err := rows.Scan(&id, &level); err != nil {
			return nil, nil, err
		}
		def := s.gameData.CollectibleByID(strings.TrimSpace(id))
		if def == nil {
			continue
		}
		for attrKey, modValue := range def.KittyMods {
			key := strings.TrimSpace(attrKey)
			if key == "" || modValue == 0 {
				continue
			}
			byAttr[key] = append(byAttr[key], collectibleAttrContribution{
				CollectibleID: def.ID,
				NameKey:       def.NameKey,
				Value:         modValue,
			})
			totals[key] += modValue
		}
	}
	return byAttr, totals, rows.Err()
}
