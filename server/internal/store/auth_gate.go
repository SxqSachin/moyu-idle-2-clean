package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthGateStore struct {
	db *pgxpool.Pool
}

type AuthGateState struct {
	UserID         int64
	SessionVersion int64
	BanReason      string
	BanUntil       *time.Time
	BanIsPermanent bool
}

func NewAuthGateStore(db *pgxpool.Pool) *AuthGateStore {
	return &AuthGateStore{db: db}
}

func (s *AuthGateStore) Health(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("auth gate store unavailable")
	}
	return s.db.Ping(ctx)
}

func (s *AuthGateStore) PoolStats() (acquired, idle, maxConns int32) {
	if s == nil || s.db == nil {
		return 0, 0, 0
	}
	stat := s.db.Stat()
	return stat.AcquiredConns(), stat.IdleConns(), stat.MaxConns()
}

func (s *AuthGateStore) GetAuthGateState(ctx context.Context, userID int64) (AuthGateState, error) {
	if s == nil || s.db == nil {
		return AuthGateState{}, errors.New("auth gate store unavailable")
	}
	if userID <= 0 {
		return AuthGateState{}, ErrUserNotFound
	}
	var state AuthGateState
	err := s.db.QueryRow(ctx, `
		SELECT u.id,
		       COALESCE(p.session_version, 0),
		       u.ban_reason,
		       u.ban_until,
		       u.ban_is_permanent
		FROM users u
		LEFT JOIN player_presence p ON p.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&state.UserID,
		&state.SessionVersion,
		&state.BanReason,
		&state.BanUntil,
		&state.BanIsPermanent,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthGateState{}, ErrUserNotFound
	}
	if err != nil {
		return AuthGateState{}, err
	}
	state.BanReason = strings.TrimSpace(state.BanReason)
	return state, nil
}

func (s AuthGateState) SessionVersionValid(tokenSessionVersion int64) bool {
	return tokenSessionVersion > 0 && s.SessionVersion == tokenSessionVersion
}

func (s AuthGateState) IsBanned(now time.Time) bool {
	return isUserBanActive(s.BanReason, s.BanUntil, s.BanIsPermanent, now.UTC())
}
