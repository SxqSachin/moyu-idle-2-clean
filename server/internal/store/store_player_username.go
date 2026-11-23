package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	defaultPlayerUsernameHistoryLimit = 50
	maxPlayerUsernameHistoryLimit     = 100
)

type PlayerUsernameHistoryEntry struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"userId"`
	OldUsername     string    `json:"oldUsername"`
	NewUsername     string    `json:"newUsername"`
	ChangedByUserID *int64    `json:"changedByUserId,omitempty"`
	ChangeSource    string    `json:"changeSource"`
	CostItemID      string    `json:"costItemId"`
	CostItemCount   int64     `json:"costItemCount"`
	ChangedAt       time.Time `json:"changedAt"`
}

type PlayerUsernameRenameResult struct {
	Username         string    `json:"username"`
	PreviousUsername string    `json:"previousUsername"`
	CostItemID       string    `json:"costItemId"`
	CostItemCount    int64     `json:"costItemCount"`
	HistoryID        int64     `json:"historyId"`
	ChangedAt        time.Time `json:"changedAt"`
}

type PlayerUsernameProfileSnapshot struct {
	PublicUsernameSnapshot
	History []PlayerUsernameHistoryEntry
}

func (s *Store) PlayerUsernameRenameCost() (string, int64) {
	return strings.TrimSpace(s.config.PlayerUsernameRenameCostItemID), s.config.PlayerUsernameRenameCostItemCount
}

func (s *Store) RenamePlayerUsername(
	ctx context.Context,
	userID int64,
	nextUsername string,
	expectedCurrentUsername string,
) (PlayerUsernameRenameResult, error) {
	nextUsername = strings.TrimSpace(nextUsername)
	if userID <= 0 {
		return PlayerUsernameRenameResult{}, ErrUserNotFound
	}
	if nextUsername == "" {
		return PlayerUsernameRenameResult{}, ErrPlayerUsernameInvalid
	}
	expectedCurrentUsername, err := normalizeExpectedPlayerUsername(expectedCurrentUsername)
	if err != nil {
		return PlayerUsernameRenameResult{}, err
	}
	costItemID, costItemCount := s.PlayerUsernameRenameCost()
	if costItemID == "" || costItemCount <= 0 {
		return PlayerUsernameRenameResult{}, ErrPlayerUsernameRenameCostInvalid
	}

	var result PlayerUsernameRenameResult
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("lock player username rename: %w", err)
		}

		var currentUsername string
		if err := tx.QueryRow(ctx, `
			SELECT username
			FROM users
			WHERE id = $1
			FOR UPDATE
		`, userID).Scan(&currentUsername); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return fmt.Errorf("read current player username: %w", err)
		}
		currentUsername = strings.TrimSpace(currentUsername)
		if !playerUsernameMatchesExpected(currentUsername, expectedCurrentUsername) {
			return ErrPlayerUsernameConflict
		}
		if currentUsername == nextUsername {
			return ErrPlayerUsernameUnchanged
		}

		tag, err := tx.Exec(ctx, `
			UPDATE users
			SET username = $2
			WHERE id = $1
		`, userID, nextUsername)
		if err != nil {
			return fmt.Errorf("update player username: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return ErrUserNotFound
		}

		var historyID int64
		var changedAt time.Time
		if err := tx.QueryRow(ctx, `
			INSERT INTO player_username_changes(
				user_id, old_username, new_username, changed_by_user_id, change_source,
				cost_item_id, cost_item_count, created_at
			)
			VALUES($1, $2, $3, $1, 'self_service', $4, $5, clock_timestamp())
			RETURNING id, created_at
		`, userID, currentUsername, nextUsername, costItemID, costItemCount).Scan(&historyID, &changedAt); err != nil {
			return fmt.Errorf("record player username history: %w", err)
		}

		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, costItemID, costItemCount, InventoryLedgerMeta{
			Category:      InventoryAuditCategorySystem,
			Direction:     InventoryAuditDirectionConsume,
			SubjectType:   "player_username_change",
			SubjectID:     strconv.FormatInt(historyID, 10),
			SourceSurface: "inventory",
			TargetSurface: "player_username_rename",
			Details: map[string]any{
				"type": "player_username_rename_cost",
			},
		}); err != nil {
			return fmt.Errorf("consume player username rename cost: %w", err)
		}

		result = PlayerUsernameRenameResult{
			Username:         nextUsername,
			PreviousUsername: currentUsername,
			CostItemID:       costItemID,
			CostItemCount:    costItemCount,
			HistoryID:        historyID,
			ChangedAt:        changedAt,
		}
		return nil
	})
	if err != nil {
		return PlayerUsernameRenameResult{}, mapPlayerUsernameRenameError(err)
	}
	return result, nil
}

func normalizeExpectedPlayerUsername(expectedCurrentUsername string) (string, error) {
	normalized := strings.TrimSpace(expectedCurrentUsername)
	if normalized == "" {
		return "", ErrPlayerUsernameInvalid
	}
	return normalized, nil
}

func playerUsernameMatchesExpected(currentUsername string, expectedCurrentUsername string) bool {
	return strings.TrimSpace(currentUsername) == expectedCurrentUsername
}

func (s *Store) GetPlayerUsernameProfileSnapshot(ctx context.Context, userID int64, limit int) (PlayerUsernameProfileSnapshot, error) {
	if userID <= 0 {
		return PlayerUsernameProfileSnapshot{}, ErrUserNotFound
	}
	limit = normalizePlayerUsernameHistoryLimit(limit)
	var snapshot PlayerUsernameProfileSnapshot
	var historyJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT
			u.username,
			COALESCE((
				SELECT MAX(changes.id)
				FROM player_username_changes changes
				WHERE changes.user_id = u.id
			), 0) AS username_change_id,
			COALESCE((
				SELECT jsonb_agg(
					jsonb_build_object(
						'id', recent.id,
						'userId', recent.user_id,
						'oldUsername', recent.old_username,
						'newUsername', recent.new_username,
						'changedByUserId', recent.changed_by_user_id,
						'changeSource', recent.change_source,
						'costItemId', recent.cost_item_id,
						'costItemCount', recent.cost_item_count,
						'changedAt', recent.created_at
					)
					ORDER BY recent.created_at DESC, recent.id DESC
				)
				FROM (
					SELECT
						id, user_id, old_username, new_username, changed_by_user_id,
						change_source, cost_item_id, cost_item_count, created_at
					FROM player_username_changes
					WHERE user_id = u.id
					ORDER BY created_at DESC, id DESC
					LIMIT $2
				) recent
			), '[]'::jsonb) AS username_history
		FROM users u
		WHERE u.id = $1
	`, userID, limit).Scan(&snapshot.Username, &snapshot.ChangeID, &historyJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerUsernameProfileSnapshot{}, ErrUserNotFound
	}
	if err != nil {
		return PlayerUsernameProfileSnapshot{}, fmt.Errorf("get player username profile snapshot: %w", err)
	}
	if err := json.Unmarshal(historyJSON, &snapshot.History); err != nil {
		return PlayerUsernameProfileSnapshot{}, fmt.Errorf("decode player username profile snapshot: %w", err)
	}
	snapshot.Username = strings.TrimSpace(snapshot.Username)
	if snapshot.History == nil {
		snapshot.History = []PlayerUsernameHistoryEntry{}
	}
	for index := range snapshot.History {
		snapshot.History[index].OldUsername = strings.TrimSpace(snapshot.History[index].OldUsername)
		snapshot.History[index].NewUsername = strings.TrimSpace(snapshot.History[index].NewUsername)
		snapshot.History[index].ChangeSource = strings.TrimSpace(snapshot.History[index].ChangeSource)
		snapshot.History[index].CostItemID = strings.TrimSpace(snapshot.History[index].CostItemID)
	}
	return snapshot, nil
}

func (s *Store) ListPlayerUsernameHistory(ctx context.Context, userID int64, limit int) ([]PlayerUsernameHistoryEntry, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	limit = normalizePlayerUsernameHistoryLimit(limit)
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, old_username, new_username, changed_by_user_id, change_source,
		       cost_item_id, cost_item_count, created_at
		FROM player_username_changes
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list player username history: %w", err)
	}
	defer rows.Close()

	out := make([]PlayerUsernameHistoryEntry, 0, limit)
	for rows.Next() {
		var entry PlayerUsernameHistoryEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.OldUsername,
			&entry.NewUsername,
			&entry.ChangedByUserID,
			&entry.ChangeSource,
			&entry.CostItemID,
			&entry.CostItemCount,
			&entry.ChangedAt,
		); err != nil {
			return nil, fmt.Errorf("scan player username history: %w", err)
		}
		entry.OldUsername = strings.TrimSpace(entry.OldUsername)
		entry.NewUsername = strings.TrimSpace(entry.NewUsername)
		entry.ChangeSource = strings.TrimSpace(entry.ChangeSource)
		entry.CostItemID = strings.TrimSpace(entry.CostItemID)
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate player username history: %w", err)
	}
	return out, nil
}

func normalizePlayerUsernameHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultPlayerUsernameHistoryLimit
	}
	if limit > maxPlayerUsernameHistoryLimit {
		return maxPlayerUsernameHistoryLimit
	}
	return limit
}

func mapPlayerUsernameRenameError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	constraintName := strings.TrimSpace(pgErr.ConstraintName)
	tableName := strings.TrimSpace(pgErr.TableName)
	if constraintName == "users_username_key" || (constraintName == "" && tableName == "users") {
		return ErrDuplicateUser
	}
	return err
}
