package store

import (
	"context"
	"strings"
	"time"
)

type AdminUserDetail struct {
	UserID         int64      `json:"userId"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	EmailVerified  bool       `json:"emailVerified"`
	BanReason      string     `json:"banReason"`
	BanUntil       *time.Time `json:"banUntil,omitempty"`
	BanIsPermanent bool       `json:"banIsPermanent"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type AdminBannedUserFilter struct {
	Query string
	Limit int
}

func (s *Store) GetAdminUserDetail(ctx context.Context, userID int64) (AdminUserDetail, error) {
	var item AdminUserDetail
	err := s.db.QueryRow(ctx, `
		SELECT id, username, email, email_verified, ban_reason, ban_until, ban_is_permanent, created_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&item.UserID,
		&item.Username,
		&item.Email,
		&item.EmailVerified,
		&item.BanReason,
		&item.BanUntil,
		&item.BanIsPermanent,
		&item.CreatedAt,
	)
	if err != nil {
		return AdminUserDetail{}, err
	}
	item.Username = strings.TrimSpace(item.Username)
	item.Email = strings.TrimSpace(item.Email)
	item.BanReason = strings.TrimSpace(item.BanReason)
	return item, nil
}

func (s *Store) ListAdminBannedUsers(ctx context.Context, filter AdminBannedUserFilter) ([]AdminUserDetail, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := strings.TrimSpace(filter.Query)
	rows, err := s.db.Query(ctx, `
		SELECT id, username, email, email_verified, ban_reason, ban_until, ban_is_permanent, created_at
		FROM users
		WHERE btrim(ban_reason) <> ''
		  AND (
		    $1 = ''
		    OR CAST(id AS TEXT) = $1
		    OR lower(username) LIKE '%' || lower($1) || '%'
		    OR lower(email) LIKE '%' || lower($1) || '%'
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminUserDetail, 0, limit)
	for rows.Next() {
		var item AdminUserDetail
		if err := rows.Scan(
			&item.UserID,
			&item.Username,
			&item.Email,
			&item.EmailVerified,
			&item.BanReason,
			&item.BanUntil,
			&item.BanIsPermanent,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Username = strings.TrimSpace(item.Username)
		item.Email = strings.TrimSpace(item.Email)
		item.BanReason = strings.TrimSpace(item.BanReason)
		items = append(items, item)
	}
	return items, rows.Err()
}
