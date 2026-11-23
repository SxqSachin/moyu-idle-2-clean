package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) GetUserByEmail(ctx context.Context, email string) (int64, string, bool, error) {
	var (
		userID        int64
		username      string
		emailVerified bool
	)
	err := s.db.QueryRow(ctx, `
		SELECT id, username, email_verified
		FROM users
		WHERE lower(email) = lower($1) AND btrim(email) <> ''
		LIMIT 1
	`, strings.TrimSpace(email)).Scan(&userID, &username, &emailVerified)
	if err != nil {
		return 0, "", false, err
	}
	return userID, strings.TrimSpace(username), emailVerified, nil
}

func (s *Store) MarkUserEmailVerified(ctx context.Context, userID int64, email string, verifiedAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE users
		SET email = $2,
			email_verified = true,
			email_verified_at = $3
		WHERE id = $1
	`, userID, strings.TrimSpace(strings.ToLower(email)), verifiedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
	}
	return err
}

func (s *Store) CreatePasswordResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO password_reset_tokens(user_id, token_hash, expires_at)
		VALUES($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (s *Store) ConsumePasswordResetToken(ctx context.Context, userID int64, tokenHash string, now time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = $3
		WHERE user_id = $1
		  AND token_hash = $2
		  AND used_at IS NULL
		  AND expires_at > $3
	`, userID, tokenHash, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) ConsumePasswordResetTokenAndUpdatePassword(ctx context.Context, tokenHash string, passwordHash string, now time.Time) (int64, bool, error) {
	var userID int64
	applied := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT user_id
			FROM password_reset_tokens
			WHERE token_hash = $1
			  AND used_at IS NULL
			  AND expires_at > $2
			FOR UPDATE
		`, tokenHash, now).Scan(&userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				applied = false
				return nil
			}
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE password_reset_tokens
			SET used_at = $3
			WHERE user_id = $1
			  AND token_hash = $2
			  AND used_at IS NULL
			  AND expires_at > $3
		`, userID, tokenHash, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			applied = false
			return nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET password_hash = $2
			WHERE id = $1
		`, userID, passwordHash); err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return userID, applied, nil
}
