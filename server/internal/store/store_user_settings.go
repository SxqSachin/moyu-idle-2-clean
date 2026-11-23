package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	DefaultUserLanguage   = "zh-CN"
	DefaultUserTheme      = "light"
	DefaultUserMenuLayout = "icon-rail"
	DefaultUserSimpleMode = false
	LegacyUserSimpleMode  = true
)

type UserSettings struct {
	UserID               int64
	Username             string
	UsernameChangeID     int64
	Email                string
	EmailVerified        bool
	Language             string
	Theme                string
	MenuLayout           string
	SimpleMode           bool
	DisableIPLoginVerify bool
}

func NormalizeUserLanguage(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch trimmed {
	case "zh-CN", "zh-TW", "en-US", "ja", "ko", "ru":
		return trimmed
	}
	return DefaultUserLanguage
}

func NormalizeUserTheme(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "dark" {
		return "dark"
	}
	return DefaultUserTheme
}

func NormalizeUserMenuLayout(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	switch trimmed {
	case "drawer", "icon-rail":
		return trimmed
	}
	return DefaultUserMenuLayout
}

func (s *Store) GetUserSettings(ctx context.Context, userID int64) (UserSettings, error) {
	var settings UserSettings
	err := s.db.QueryRow(ctx, `
		SELECT
			u.id,
			u.username,
			COALESCE((
				SELECT MAX(changes.id)
				FROM player_username_changes changes
				WHERE changes.user_id = u.id
			), 0) AS username_change_id,
			u.email,
			u.email_verified,
			COALESCE(us.language, $2) AS language,
			COALESCE(us.theme, $3) AS theme,
			COALESCE(us.extra_jsonb->>'menuLayout', $4) AS menu_layout,
			COALESCE((us.extra_jsonb->>'simpleMode')::boolean, $5::boolean) AS simple_mode,
			COALESCE((us.extra_jsonb->>'disableIPLoginVerify')::boolean, false) AS disable_ip_login_verify
		FROM users u
		LEFT JOIN user_settings us ON us.user_id = u.id
		WHERE u.id = $1
	`, userID, DefaultUserLanguage, DefaultUserTheme, DefaultUserMenuLayout, LegacyUserSimpleMode).Scan(
		&settings.UserID,
		&settings.Username,
		&settings.UsernameChangeID,
		&settings.Email,
		&settings.EmailVerified,
		&settings.Language,
		&settings.Theme,
		&settings.MenuLayout,
		&settings.SimpleMode,
		&settings.DisableIPLoginVerify,
	)
	if err != nil {
		return UserSettings{}, err
	}
	settings.Username = strings.TrimSpace(settings.Username)
	settings.Email = strings.TrimSpace(settings.Email)
	settings.Language = NormalizeUserLanguage(settings.Language)
	settings.Theme = NormalizeUserTheme(settings.Theme)
	settings.MenuLayout = NormalizeUserMenuLayout(settings.MenuLayout)
	return settings, nil
}

func (s *Store) UpsertUserLanguage(ctx context.Context, userID int64, language string) error {
	normalized := NormalizeUserLanguage(language)
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, '{}'::jsonb, now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET language = EXCLUDED.language, updated_at = now()
	`, userID, normalized, DefaultUserTheme)
	return err
}

func (s *Store) UpsertUserTheme(ctx context.Context, userID int64, theme string) error {
	normalized := NormalizeUserTheme(theme)
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, '{}'::jsonb, now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET theme = EXCLUDED.theme, updated_at = now()
	`, userID, DefaultUserLanguage, normalized)
	return err
}

func (s *Store) UpsertUserMenuLayout(ctx context.Context, userID int64, menuLayout string) error {
	normalized := NormalizeUserMenuLayout(menuLayout)
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, jsonb_build_object('menuLayout', $4::text), now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			extra_jsonb = jsonb_set(
				COALESCE(user_settings.extra_jsonb, '{}'::jsonb),
				'{menuLayout}',
				to_jsonb(EXCLUDED.extra_jsonb->>'menuLayout'),
				true
			),
			updated_at = now()
	`, userID, DefaultUserLanguage, DefaultUserTheme, normalized)
	return err
}

func (s *Store) UpsertUserSimpleMode(ctx context.Context, userID int64, enabled bool) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, jsonb_build_object('simpleMode', $4::boolean), now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			extra_jsonb = jsonb_set(
				COALESCE(user_settings.extra_jsonb, '{}'::jsonb),
				'{simpleMode}',
				to_jsonb($4::boolean),
				true
			),
			updated_at = now()
	`, userID, DefaultUserLanguage, DefaultUserTheme, enabled)
	return err
}

const updateUnverifiedEmailSQL = `
	UPDATE users
	SET email = $2,
		email_verified = false,
		email_verified_at = NULL
	WHERE id = $1
`

func (s *Store) UpdateUnverifiedEmail(ctx context.Context, userID int64, email string) error {
	_, err := s.db.Exec(ctx, updateUnverifiedEmailSQL, userID, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
	}
	return err
}

func (s *Store) UpsertDisableIPLoginVerify(ctx context.Context, userID int64, disabled bool) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, jsonb_build_object('disableIPLoginVerify', $4::boolean), now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			extra_jsonb = jsonb_set(
				COALESCE(user_settings.extra_jsonb, '{}'::jsonb),
				'{disableIPLoginVerify}',
				to_jsonb($4::boolean),
				true
			),
			updated_at = now()
	`, userID, DefaultUserLanguage, DefaultUserTheme, disabled)
	return err
}

// tutorialIDPattern guards the dynamic jsonb object key against path injection.
// Tutorial ids are static frontend identifiers (e.g. "combat", "actions").
var tutorialIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// IsValidTutorialID reports whether id is a safe tutorial identifier.
func IsValidTutorialID(id string) bool {
	return tutorialIDPattern.MatchString(id)
}

// TutorialProgressEntry is one tutorial's completion record, stored as a jsonb
// value under user_settings.extra_jsonb->'tutorialProgress'->'<tutorialId>'.
type TutorialProgressEntry struct {
	Completed   bool  `json:"completed"`
	CompletedAt int64 `json:"completedAt"`
	Version     int   `json:"version"`
}

// GetTutorialProgress returns the full map of tutorial completion records for a
// user. A missing user_settings row or empty progress yields an empty map.
func (s *Store) GetTutorialProgress(ctx context.Context, userID int64) (map[string]TutorialProgressEntry, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(extra_jsonb->'tutorialProgress', '{}'::jsonb)
		FROM user_settings
		WHERE user_id = $1
	`, userID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]TutorialProgressEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query tutorial progress: %w", err)
	}
	result := map[string]TutorialProgressEntry{}
	if len(raw) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode tutorial progress: %w", err)
	}
	return result, nil
}

// UpsertTutorialProgressEntry writes a single tutorial's completion record,
// preserving other tutorials' entries. The dynamic object key is parameterized
// and validated to avoid jsonb path injection.
func (s *Store) UpsertTutorialProgressEntry(ctx context.Context, userID int64, tutorialID string, entry TutorialProgressEntry) error {
	if !IsValidTutorialID(tutorialID) {
		return fmt.Errorf("invalid tutorial id: %q", tutorialID)
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode tutorial progress entry: %w", err)
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, jsonb_build_object('tutorialProgress', jsonb_build_object($4::text, $5::jsonb)), now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			extra_jsonb = jsonb_set(
				jsonb_set(
					COALESCE(user_settings.extra_jsonb, '{}'::jsonb),
					'{tutorialProgress}',
					COALESCE(user_settings.extra_jsonb->'tutorialProgress', '{}'::jsonb),
					true
				),
				ARRAY['tutorialProgress', $4::text],
				$5::jsonb,
				true
			),
			updated_at = now()
	`, userID, DefaultUserLanguage, DefaultUserTheme, tutorialID, string(payload))
	if err != nil {
		return fmt.Errorf("upsert tutorial progress: %w", err)
	}
	return nil
}

// DeleteTutorialProgressEntry removes a single tutorial's completion record
// (used by the reset endpoint). Missing rows or keys are a no-op.
func (s *Store) DeleteTutorialProgressEntry(ctx context.Context, userID int64, tutorialID string) error {
	if !IsValidTutorialID(tutorialID) {
		return fmt.Errorf("invalid tutorial id: %q", tutorialID)
	}
	_, err := s.db.Exec(ctx, `
		UPDATE user_settings
		SET extra_jsonb = COALESCE(extra_jsonb, '{}'::jsonb) #- ARRAY['tutorialProgress', $2::text],
			updated_at = now()
		WHERE user_id = $1
	`, userID, tutorialID)
	if err != nil {
		return fmt.Errorf("delete tutorial progress: %w", err)
	}
	return nil
}

func (s *Store) GetPasswordHashByUserID(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := s.db.QueryRow(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1
	`, userID).Scan(&hash)
	return hash, err
}

func (s *Store) UpdatePasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2
		WHERE id = $1
	`, userID, passwordHash)
	return err
}
