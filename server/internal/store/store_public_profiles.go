package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type PublicKittyProfile struct {
	UserID          int64     `json:"userId"`
	Username        string    `json:"username"`
	KittyID         int64     `json:"kittyId"`
	KittyName       string    `json:"kittyName"`
	KittyCreatedAt  time.Time `json:"kittyCreatedAt"`
	RunningActionID string    `json:"runningActionId,omitempty"`
}

type PublicUsernameSnapshot struct {
	Username string
	ChangeID int64
}

func (s *Store) GetPublicUsernameSnapshot(ctx context.Context, userID int64) (PublicUsernameSnapshot, error) {
	var snapshot PublicUsernameSnapshot
	err := s.db.QueryRow(ctx, `
		SELECT
			u.username,
			COALESCE((
				SELECT MAX(changes.id)
				FROM player_username_changes changes
				WHERE changes.user_id = u.id
			), 0) AS username_change_id
		FROM users u
		WHERE u.id = $1
	`, userID).Scan(&snapshot.Username, &snapshot.ChangeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicUsernameSnapshot{}, ErrUserNotFound
	}
	if err != nil {
		return PublicUsernameSnapshot{}, err
	}
	snapshot.Username = strings.TrimSpace(snapshot.Username)
	return snapshot, nil
}

func (s *Store) GetPublicUsernameByUserID(ctx context.Context, userID int64) (string, error) {
	snapshot, err := s.GetPublicUsernameSnapshot(ctx, userID)
	return snapshot.Username, err
}

func (s *Store) GetPublicKittyProfile(ctx context.Context, userID int64, kittyID int64) (PublicKittyProfile, error) {
	var row PublicKittyProfile
	err := s.db.QueryRow(ctx, `
		SELECT
			k.user_id,
			COALESCE(u.username, ''),
			k.id,
			k.name,
			k.created_at
		FROM player_kitties k
		JOIN users u ON u.id = k.user_id
		WHERE k.user_id = $1
		  AND k.id = $2
	`, userID, kittyID).Scan(
		&row.UserID,
		&row.Username,
		&row.KittyID,
		&row.KittyName,
		&row.KittyCreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicKittyProfile{}, ErrKittyNotFound
	}
	if err != nil {
		return PublicKittyProfile{}, err
	}
	row.Username = strings.TrimSpace(row.Username)
	row.KittyName = strings.TrimSpace(row.KittyName)
	actionID, actionErr := s.GetKittyRunningActionID(ctx, row.UserID, row.KittyID)
	if actionErr != nil {
		return PublicKittyProfile{}, actionErr
	}
	row.RunningActionID = strings.TrimSpace(actionID)
	return row, nil
}
