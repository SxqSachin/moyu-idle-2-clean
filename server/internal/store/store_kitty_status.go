package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	KittyStatusIdle        = "idle"
	KittyStatusInLifeQueue = "in_life_queue"
	KittyStatusInCombat    = "in_combat"
)

func normalizeKittyStatus(status string) string {
	switch strings.TrimSpace(status) {
	case KittyStatusInLifeQueue:
		return KittyStatusInLifeQueue
	case KittyStatusInCombat:
		return KittyStatusInCombat
	default:
		return KittyStatusIdle
	}
}

func (s *Store) getOwnedKittyStatusTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, forUpdate bool) (string, error) {
	if kittyID <= 0 {
		return "", ErrKittyNotFound
	}
	query := `
		SELECT status
		FROM player_kitties
		WHERE user_id = $1 AND id = $2`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var status string
	if err := tx.QueryRow(ctx, query, userID, kittyID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrKittyNotFound
		}
		return "", err
	}
	return normalizeKittyStatus(status), nil
}

func (s *Store) setOwnedKittyStatusTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, status string) error {
	status = normalizeKittyStatus(status)
	tag, err := tx.Exec(ctx, `
		UPDATE player_kitties
		SET status = $3,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2
	`, userID, kittyID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() <= 0 {
		return ErrKittyNotFound
	}
	return nil
}

func (s *Store) setKittyStatusByIDTx(ctx context.Context, tx pgx.Tx, kittyID int64, status string) error {
	status = normalizeKittyStatus(status)
	_, err := tx.Exec(ctx, `
		UPDATE player_kitties
		SET status = $2,
		    updated_at = now()
		WHERE id = $1
	`, kittyID, status)
	return err
}

func (s *Store) cancelKittyLifeQueueTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "enter_combat_conflict"
	}
	_, err := tx.Exec(ctx, `
		UPDATE player_action_queue
		SET status = 'canceled',
		    updated_at = now(),
			    payload = jsonb_set(COALESCE(payload, '{}'::jsonb), '{cancelReason}', to_jsonb($3::text), true)
		WHERE user_id = $1
		  AND actor_type = 'kitty'
		  AND actor_id = $2
		  AND status IN ('pending', 'running')
	`, userID, kittyID, reason)
	return err
}
