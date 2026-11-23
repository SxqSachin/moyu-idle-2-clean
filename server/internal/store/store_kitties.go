package store

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	ActionActorTypePlayer = "player"
	ActionActorTypeKitty  = "kitty"
)

func (s *Store) ListPlayerKitties(ctx context.Context, userID int64) ([]PlayerKitty, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, status, level, battle_level_cap, exp, total_exp, bond_exp, mood, extra_jsonb, created_at
		FROM player_kitties
		WHERE user_id = $1
		ORDER BY created_at ASC, id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlayerKitty, 0, 4)
	kittyIDs := make([]int64, 0, 4)
	for rows.Next() {
		var row PlayerKitty
		var extraRaw []byte
		if err := rows.Scan(&row.ID, &row.Name, &row.Status, &row.Level, &row.BattleLevelCap, &row.Exp, &row.TotalExp, &row.BondExp, &row.Mood, &extraRaw, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.Name = strings.TrimSpace(row.Name)
		row.Status = normalizeKittyStatus(row.Status)
		row.BattleLevelCap = s.effectiveBattleLevelCap(row.BattleLevelCap, row.Level)
		row.Extra = decodeJSONMap(extraRaw)
		row.NextExp = s.KittyNextExp(row.Level)
		out = append(out, row)
		kittyIDs = append(kittyIDs, row.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	personalityByKitty, err := s.listKittyPersonalityIDs(ctx, userID, kittyIDs)
	if err != nil {
		return nil, err
	}
	for idx := range out {
		out[idx].Extra = setKittyPersonalityIDsExtra(out[idx].Extra, personalityByKitty[out[idx].ID])
	}
	return out, nil
}

func (s *Store) AdoptKitty(ctx context.Context, userID int64, name string) (PlayerKitty, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return PlayerKitty{}, ErrKittyNameInvalid
	}
	var out PlayerKitty
	var outExtraRaw []byte
	personalityIDs := []string(nil)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
			return err
		}
		level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, "cattery")
		if err != nil {
			return err
		}
		if level <= 0 {
			return ErrCatteryUnavailable
		}
		capacity := level
		if capacity < 1 {
			capacity = 1
		}
		var currentCount int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM player_kitties
			WHERE user_id = $1
		`, userID).Scan(&currentCount); err != nil {
			return err
		}
		if currentCount >= capacity {
			return ErrKittyAdoptLimitReached
		}
		personalityIDs = s.resolveNewKittyPersonalityIDs(currentCount + 1)
		if err := tx.QueryRow(ctx, `
			INSERT INTO player_kitties(user_id, name, status, level, battle_level_cap, exp, total_exp, bond_exp, mood, extra_jsonb, created_at, updated_at)
			VALUES($1, $2, 'idle', 1, $3, 0, 0, 0, 0, '{}'::jsonb, now(), now())
			RETURNING id, name, status, level, battle_level_cap, exp, total_exp, bond_exp, mood, extra_jsonb, created_at
		`, userID, normalized, s.kittyBattleLevelCapDefault()).Scan(&out.ID, &out.Name, &out.Status, &out.Level, &out.BattleLevelCap, &out.Exp, &out.TotalExp, &out.BondExp, &out.Mood, &outExtraRaw, &out.CreatedAt); err != nil {
			return err
		}
		if err := s.replaceKittyPersonalityRowsTx(ctx, tx, userID, out.ID, personalityIDs); err != nil {
			return err
		}
		return s.EnqueueAchievementEventsTx(ctx, tx, userID, []AchievementEvent{{
			Kind:      AchievementEventStateReconcile,
			StateKind: "own_kitty",
			SourceKey: "kitty_adopt:" + strconv.FormatInt(out.ID, 10),
		}})
	})
	if err != nil {
		return PlayerKitty{}, err
	}
	out.Name = strings.TrimSpace(out.Name)
	out.Status = normalizeKittyStatus(out.Status)
	out.BattleLevelCap = s.effectiveBattleLevelCap(out.BattleLevelCap, out.Level)
	out.Extra = decodeJSONMap(outExtraRaw)
	out.Extra = setKittyPersonalityIDsExtra(out.Extra, personalityIDs)
	out.NextExp = s.KittyNextExp(out.Level)
	return out, nil
}

func (s *Store) KittyNextExp(level int) int64 {
	if level < 1 {
		level = 1
	}
	maxLevel := s.config.KittyMaxLevel
	if maxLevel <= 0 {
		maxLevel = 30
	}
	if level >= maxLevel {
		return 0
	}
	if s.config.KittyLevelExp != nil {
		if next := s.config.KittyLevelExp[level]; next > 0 {
			return next
		}
	}
	base := s.config.KittyLevelExpBase
	if base <= 0 {
		base = 100
	}
	perLevel := s.config.KittyLevelExpPerLevel
	power := s.config.KittyLevelExpPower
	if power <= 0 || math.IsNaN(power) || math.IsInf(power, 0) {
		power = 1.25
	}
	next := float64(base) + float64(perLevel)*math.Pow(float64(level), power)
	if next < 1 || math.IsNaN(next) || math.IsInf(next, 0) {
		return base
	}
	return int64(math.Round(next))
}

func (s *Store) kittyBattleLevelCapDefault() int64 {
	value := s.config.KittyBattleLevelCapDefault
	if value <= 0 {
		return 100
	}
	return value
}

func (s *Store) kittyBattleLevelCapPerKittyLevel() int64 {
	if s.config.KittyBattleLevelCapPerKittyLevel <= 0 {
		return 25
	}
	return s.config.KittyBattleLevelCapPerKittyLevel
}

func (s *Store) effectiveBattleLevelCap(storedCap int64, kittyLevel int) int64 {
	base := storedCap
	if base <= 0 {
		base = s.kittyBattleLevelCapDefault()
	}
	return base + int64(kittyLevel)*s.kittyBattleLevelCapPerKittyLevel()
}

func (s *Store) isOwnedKittyTx(ctx context.Context, tx pgx.Tx, userID int64, kittyID int64) (bool, error) {
	if kittyID <= 0 {
		return false, nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM player_kitties
			WHERE user_id = $1 AND id = $2
		)
	`, userID, kittyID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
