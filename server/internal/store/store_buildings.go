package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListPlayerBuildings(ctx context.Context, userID int64) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT building_id, level
		FROM player_buildings
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var buildingID string
		var level int64
		if err := rows.Scan(&buildingID, &level); err != nil {
			return nil, err
		}
		normalizedID := strings.TrimSpace(buildingID)
		if normalizedID == "" {
			continue
		}
		if level < 0 {
			level = 0
		}
		out[normalizedID] = level
	}
	return out, rows.Err()
}

func (s *Store) getPlayerBuildingLevelTx(ctx context.Context, tx pgx.Tx, userID int64, buildingID string) (int64, error) {
	var level int64
	err := tx.QueryRow(ctx, `
		SELECT level
		FROM player_buildings
		WHERE user_id = $1 AND building_id = $2
	`, userID, strings.TrimSpace(buildingID)).Scan(&level)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	if level < 0 {
		return 0, nil
	}
	return level, nil
}

func (s *Store) setPlayerBuildingLevelTx(ctx context.Context, tx pgx.Tx, userID int64, buildingID string, level int64) error {
	normalizedID := strings.TrimSpace(buildingID)
	if normalizedID == "" {
		return nil
	}
	if level < 0 {
		level = 0
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO player_buildings(user_id, building_id, level, updated_at)
		VALUES($1, $2, $3, now())
		ON CONFLICT (user_id, building_id)
		DO UPDATE SET level = EXCLUDED.level, updated_at = now()
	`, userID, normalizedID, level)
	return err
}
