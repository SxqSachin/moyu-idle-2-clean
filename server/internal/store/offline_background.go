package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const OfflineBackgroundInterval = 30 * time.Second

func minRecoveryTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Fail at startup rather than serving a permanently broken recovery protocol.
func CheckOfflineRecoverySchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `SELECT `+offlineRecoveryColumns+`,completed_at,acknowledged_at,schedule_pending FROM player_offline_recoveries LIMIT 0;
 SELECT run_id,user_id,item_rewards FROM battle_reward_receipts LIMIT 0;
 SELECT equipment_config_jsonb FROM player_skills LIMIT 0`)
	if err != nil {
		return fmt.Errorf("offline recovery dependencies unavailable; apply 20260907_offline_recovery.sql, 20260908_background_offline_recovery.sql and 20260907_equipment_skill_config.sql before starting services: %w", err)
	}
	return nil
}

// The supplied presence snapshot is only a hint. Recheck it under the user
// lock before opening/extending an absence, so reconnect wins against old work.
func (s *Store) PrepareBackgroundOfflineRecovery(ctx context.Context, userID int64, expected PresenceState, start, now time.Time) (OfflineRecovery, bool, error) {
	var result OfflineRecovery
	ready := false
	if start.IsZero() || !now.After(start) {
		return result, false, nil
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var presence PresenceState
		if err := tx.QueryRow(ctx, `SELECT status,last_seen_at,session_version FROM player_presence WHERE user_id=$1 FOR UPDATE`, userID).Scan(&presence.Status, &presence.LastSeenAt, &presence.SessionVersion); err != nil {
			return err
		}
		if presence.Status != expected.Status || !presence.LastSeenAt.Equal(expected.LastSeenAt) || presence.SessionVersion != expected.SessionVersion {
			return nil
		}
		previous, err := scanOfflineRecovery(tx.QueryRow(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries WHERE user_id=$1 ORDER BY id DESC LIMIT 1`, userID))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && !previous.Background && !start.After(previous.ResumedAt) {
			return nil
		}
		if err == nil && previous.Background {
			start = previous.StartedAt
			if previous.Status != "complete" {
				result, ready = previous, previous.Status == "running" && !previous.AvailableAt.After(now)
				return nil
			}
		}
		target := now
		if cap := s.config.OfflineProgressMaxDuration; cap > 0 {
			target = minRecoveryTime(target, start.Add(cap))
		}
		if previous.Background {
			if !target.After(previous.TargetAt) {
				result = previous
				return nil
			}
			result, err = scanOfflineRecovery(tx.QueryRow(ctx, `UPDATE player_offline_recoveries SET target_at=$2,resumed_at=$2,status='running',revision=revision+1,completed_at=NULL,available_at=now() WHERE id=$1 RETURNING `+offlineRecoveryColumns, previous.ID, target))
		} else {
			// Do not start newer work ahead of an unfinished historical window.
			if err := s.assertNoOfflineRecoveryTx(ctx, tx, userID); err != nil {
				return err
			}
			result, err = scanOfflineRecovery(tx.QueryRow(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,background) VALUES($1,$2,$3,$3,true) RETURNING `+offlineRecoveryColumns, userID, start, target))
		}
		ready = err == nil
		return err
	})
	return result, ready, err
}

// Redis is only a wake-up index. Keyset pagination also recovers users removed
// by the old offline-skip worker and rebuilds scheduling after Redis loss.
type BackgroundActionPage struct {
	UserIDs   []int64
	NextAfter int64
}

func (s *Store) ListBackgroundActionUsers(ctx context.Context, after int64, limit int) (BackgroundActionPage, error) {
	var page BackgroundActionPage
	if limit <= 0 || limit > 256 {
		limit = 256
	}
	rows, err := s.db.Query(ctx, `WITH candidates AS MATERIALIZED (
 SELECT DISTINCT user_id FROM player_action_queue WHERE user_id>$1 AND status IN ('running','pending') ORDER BY user_id LIMIT $2)
 SELECT q.user_id,NOT EXISTS(SELECT 1 FROM player_offline_recoveries r WHERE r.user_id=q.user_id AND r.background AND r.status='complete' AND r.target_at>=r.started_at+$3::interval)
 FROM candidates q ORDER BY q.user_id`, after, limit, s.config.OfflineProgressMaxDuration.String())
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var eligible bool
		if err := rows.Scan(&id, &eligible); err != nil {
			return page, err
		}
		page.NextAfter = id
		if eligible {
			page.UserIDs = append(page.UserIDs, id)
		}
	}
	return page, rows.Err()
}

func (s *Store) AdvanceBackgroundActions(ctx context.Context, userID int64, presence PresenceState, start, now time.Time, specs map[string]ActionSpec) (time.Time, error) {
	r, ready, err := s.PrepareBackgroundOfflineRecovery(ctx, userID, presence, start, now)
	if err != nil {
		return time.Time{}, err
	}
	if !ready {
		if r.Background && r.Status == "stalled" {
			return time.Time{}, fmt.Errorf("background recovery %d requires repair: %s", r.ID, r.ErrorCode)
		}
		if r.Background && r.Status == "running" {
			return r.AvailableAt, nil
		}
		if r.Background && s.config.OfflineProgressMaxDuration > 0 && !r.TargetAt.Before(r.StartedAt.Add(s.config.OfflineProgressMaxDuration)) {
			return time.Time{}, nil
		}
		return now.Add(OfflineBackgroundInterval), nil
	}
	if err := s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if retryErr := s.RetryOfflineRecoveryLater(retryCtx, r, err); retryErr != nil {
			return time.Time{}, errors.Join(err, retryErr)
		}
		return time.Time{}, err
	}
	current, err := scanOfflineRecovery(s.db.QueryRow(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries WHERE id=$1`, r.ID))
	if err != nil {
		return time.Time{}, err
	}
	if !current.Background {
		return now, nil
	}
	if current.Status == "stalled" {
		return time.Time{}, fmt.Errorf("background recovery %d requires repair: %s", current.ID, current.ErrorCode)
	}
	if current.Status == "running" {
		return now.Add(time.Second), nil
	}
	if cap := s.config.OfflineProgressMaxDuration; cap > 0 && !current.TargetAt.Before(current.StartedAt.Add(cap)) {
		return time.Time{}, nil
	}
	var active bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_action_queue WHERE user_id=$1 AND status IN ('running','pending'))`, userID).Scan(&active); err != nil {
		return time.Time{}, err
	}
	if !active {
		return time.Time{}, nil
	}
	return now.Add(OfflineBackgroundInterval + time.Duration(userID%5000)*time.Millisecond), nil
}
