package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrOfflineRecoveryPending = errors.New("offline recovery pending")
var ErrOfflineRecoveryBusy = errors.New("offline recovery busy")

type OfflineRecovery struct {
	ID            int64
	UserID        int64
	StartedAt     time.Time
	TargetAt      time.Time
	ResumedAt     time.Time
	Status        string
	Revision      int64
	Summary       ActionFastForwardSummary
	PendingRunIDs []int64
	Attempts      int
	ErrorCode     string
	Background    bool
	AvailableAt   time.Time
}

const offlineRecoveryColumns = `id,user_id,started_at,target_at,resumed_at,status,revision,summary,pending_run_ids,attempts,error_code,background,available_at`

func scanOfflineRecovery(row pgx.Row) (OfflineRecovery, error) {
	var r OfflineRecovery
	var raw []byte
	err := row.Scan(&r.ID, &r.UserID, &r.StartedAt, &r.TargetAt, &r.ResumedAt, &r.Status, &r.Revision, &raw, &r.PendingRunIDs, &r.Attempts, &r.ErrorCode, &r.Background, &r.AvailableAt)
	if err == nil {
		err = json.Unmarshal(raw, &r.Summary)
	}
	return r, err
}

// Presence must not advance until the original recovery window is durable.
func (s *Store) EnsureOfflineRecovery(ctx context.Context, userID int64, start, target, resumed time.Time) (OfflineRecovery, error) {
	var result OfflineRecovery
	if userID <= 0 || target.Before(start) || resumed.Before(target) {
		return result, errors.New("invalid recovery window")
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		previous, err := scanOfflineRecovery(tx.QueryRow(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries WHERE user_id=$1 ORDER BY id DESC LIMIT 1`, userID))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && previous.Background {
			// Seal the original absence under the same lock as settlement. A stale
			// background worker cannot extend or commit this window afterwards.
			target = previous.StartedAt.Add(target.Sub(start))
			if cap := s.config.OfflineProgressMaxDuration; cap > 0 {
				target = minRecoveryTime(resumed, previous.StartedAt.Add(cap))
			}
			if target.Before(previous.TargetAt) {
				target = previous.TargetAt
			}
			result, err = scanOfflineRecovery(tx.QueryRow(ctx, `UPDATE player_offline_recoveries SET
 background=false,target_at=$2,resumed_at=$3,status='running',revision=revision+1,attempts=0,error_code='',available_at=now(),completed_at=NULL,
 pending_run_ids=ARRAY(SELECT r.id FROM battle_rooms b JOIN battle_room_members m ON m.room_id=b.id
 JOIN battle_runs r ON r.id=b.current_run_id WHERE m.user_id=$4 AND r.started_at<=$3 AND r.status=ANY($5::text[]))
 WHERE id=$1 RETURNING `+offlineRecoveryColumns, previous.ID, target, resumed, userID, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling}))
			return err
		}
		// A repeated bind for the same absence must not create a new time window.
		if err == nil && !start.After(previous.ResumedAt) {
			result = previous
			return nil
		}
		result, err = scanOfflineRecovery(tx.QueryRow(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,pending_run_ids)
   VALUES($1,$2,$3,$4,ARRAY(SELECT r.id FROM battle_rooms b JOIN battle_room_members m ON m.room_id=b.id
    JOIN battle_runs r ON r.id=b.current_run_id WHERE m.user_id=$1 AND r.started_at<=$4 AND r.status=ANY($5::text[])))
   RETURNING `+offlineRecoveryColumns, userID, start, target, resumed, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling}))
		return err
	})
	return result, err
}

func (s *Store) GetOfflineRecovery(ctx context.Context, userID int64) (OfflineRecovery, error) {
	return scanOfflineRecovery(s.db.QueryRow(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries WHERE user_id=$1 AND NOT background AND acknowledged_at IS NULL ORDER BY id LIMIT 1`, userID))
}

func (s *Store) HasOfflineRecovery(ctx context.Context, userID int64) (bool, error) {
	var pending bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_offline_recoveries WHERE user_id=$1 AND (background OR status IN ('running','stalled')))`, userID).Scan(&pending)
	return pending, err
}

func (s *Store) assertNoOfflineRecoveryTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_offline_recoveries WHERE user_id=$1 AND (background OR status IN ('running','stalled')))`, userID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrOfflineRecoveryPending
	}
	return nil
}

func (s *Store) ListDueOfflineRecoveries(ctx context.Context, limit int) ([]OfflineRecovery, error) {
	if limit < 1 || limit > 16 {
		limit = 4
	}
	rows, err := s.db.Query(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries j WHERE NOT background AND status='running' AND available_at<=now()
  AND NOT EXISTS(SELECT 1 FROM player_offline_recoveries p WHERE p.user_id=j.user_id AND p.id<j.id AND p.status IN ('running','stalled'))
  ORDER BY available_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OfflineRecovery
	for rows.Next() {
		r, err := scanOfflineRecovery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AdvanceOfflineRecovery(ctx context.Context, r OfflineRecovery, specs map[string]ActionSpec) error {
	steps := s.config.ActionQueueFastForwardMaxSteps
	if steps <= 0 {
		steps = defaultActionQueueFastForwardMaxSteps
	}
	steps = max(1, steps>>max(0, min(r.Attempts, 10)))
	_, _, _, _, err := s.advanceActionQueueInternal(ctx, r.UserID, specs, actionQueueAdvanceOptions{
		AllowBatch: true, OfflineLog: true, SkipReadModels: true, SkipQueueRead: true, SettleAt: r.TargetAt, Recovery: &r,
		Budget: FastForwardBudget{MaxIterations: steps},
	})
	return err
}

// The version predicate also fences failure handling after an ambiguous COMMIT.
func (s *Store) RetryOfflineRecoveryLater(ctx context.Context, r OfflineRecovery, cause error) error {
	if errors.Is(cause, ErrOfflineRecoveryBusy) {
		_, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries SET available_at=now()+interval '200 milliseconds' WHERE id=$1 AND revision=$2 AND status='running'`, r.ID, r.Revision)
		return err
	}
	delay := time.Duration(min(r.Attempts+1, 30)) * time.Second
	_, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries SET attempts=LEAST(attempts+1,30),error_code='OFFLINE_RECOVERY_RETRY',
  status=CASE WHEN NOT $4 AND attempts>=9 THEN 'stalled' ELSE 'running' END,available_at=now()+$3::interval
  WHERE id=$1 AND revision=$2 AND status='running'`, r.ID, r.Revision, delay.String(), transientRecoveryError(cause))
	return err
}

func transientRecoveryError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || pgconn.SafeToRetry(err) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) {
		return true
	}
	var postgres *pgconn.PgError
	if errors.As(err, &postgres) {
		return strings.HasPrefix(postgres.Code, "08") || strings.HasPrefix(postgres.Code, "53") || postgres.Code == "40001" || postgres.Code == "40P01" || postgres.Code == "55P03" || postgres.Code == "57014" || postgres.Code == "57P01" || postgres.Code == "57P02" || postgres.Code == "57P03"
	}
	return false
}

func (s *Store) RetryOfflineRecovery(ctx context.Context, userID, id, revision int64) error {
	// A retry is a new attempt generation; late workers must not restore an old failure.
	_, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries SET status='running',revision=revision+1,attempts=0,error_code='',available_at=now()
  WHERE user_id=$1 AND id=$2 AND revision=$3 AND status='stalled'`, userID, id, revision)
	return err
}

func (s *Store) AcknowledgeOfflineRecovery(ctx context.Context, userID, id, revision int64) error {
	_, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries j SET acknowledged_at=COALESCE(acknowledged_at,now())
  WHERE user_id=$1 AND id=$2 AND revision=$3 AND status='complete' AND NOT background
  AND NOT EXISTS (
   SELECT 1 FROM unnest(j.pending_run_ids) AS pending(run_id)
   JOIN battle_runs b ON b.id=pending.run_id AND b.status=ANY($4::text[])
   WHERE NOT EXISTS(SELECT 1 FROM battle_reward_receipts r WHERE r.user_id=j.user_id AND r.run_id=pending.run_id)
   AND NOT EXISTS(SELECT 1 FROM player_offline_recoveries prior WHERE prior.user_id=j.user_id
    AND prior.started_at<j.started_at AND pending.run_id=ANY(prior.pending_run_ids))
  )`, userID, id, revision, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling})
	return err
}

func (s *Store) loadRecoveryForAdvanceTx(ctx context.Context, tx pgx.Tx, expected OfflineRecovery) (OfflineRecovery, bool, error) {
	r, err := scanOfflineRecovery(tx.QueryRow(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries WHERE id=$1 AND user_id=$2 FOR UPDATE`, expected.ID, expected.UserID))
	if err != nil {
		return r, false, err
	}
	if r.Status != "running" || r.Revision != expected.Revision {
		return r, false, nil
	}
	var older bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_offline_recoveries WHERE user_id=$1 AND id<$2 AND status IN ('running','stalled'))`, r.UserID, r.ID).Scan(&older)
	return r, !older, err
}

func (s *Store) commitRecoveryProgressTx(ctx context.Context, tx pgx.Tx, r OfflineRecovery, chunk ActionFastForwardSummary) error {
	summary := mergeOfflineRecoverySummary(r.Summary, chunk)
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	status := "running"
	if chunk.YieldReason != "" {
		status = "stalled"
	} else if !chunk.HasMoreDue {
		status = "complete"
	}
	tag, err := tx.Exec(ctx, `UPDATE player_offline_recoveries SET summary=$3,status=$4,revision=revision+1,attempts=0,error_code=$5,
  available_at=now()+interval '25 milliseconds',completed_at=CASE WHEN $4='complete' THEN now() ELSE NULL END,schedule_pending=($4='complete' AND NOT background)
  WHERE id=$1 AND revision=$2 AND status='running'`, r.ID, r.Revision, string(raw), status, chunk.YieldReason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("recovery revision conflict")
	}
	return nil
}

func mergeOfflineRecoverySummary(base, next ActionFastForwardSummary) ActionFastForwardSummary {
	out := next
	out.CompletedSteps += base.CompletedSteps
	out.ItemRewards = mergeRecoveryCounts(base.ItemRewards, next.ItemRewards)
	out.ItemCosts = mergeRecoveryCounts(base.ItemCosts, next.ItemCosts)
	out.AttrExp = mergeAttrExp(base.AttrExp, next.AttrExp)
	actors := map[string]*ActionActorSummary{}
	for _, row := range append(base.ActorSummaries, next.ActorSummaries...) {
		key := actorLaneKey(row.ActorType, row.ActorID)
		v := actors[key]
		if v == nil {
			copy := row
			actors[key] = &copy
			continue
		}
		v.CompletedSteps += row.CompletedSteps
		v.ItemRewards = mergeRecoveryCounts(v.ItemRewards, row.ItemRewards)
		v.ItemCosts = mergeRecoveryCounts(v.ItemCosts, row.ItemCosts)
		v.AttrExp = mergeAttrExp(v.AttrExp, row.AttrExp)
		v.OwnerAttrExp = mergeAttrExp(v.OwnerAttrExp, row.OwnerAttrExp)
		v.EquipmentChanged = v.EquipmentChanged || row.EquipmentChanged
	}
	out.ActorSummaries = buildActionActorSummaries(actors)
	out.FishingCatches = base.FishingCatches
	if out.FishingCatches == nil {
		out.FishingCatches = map[string]FishingCatchTally{}
	}
	mergeFishingCatchTallies(out.FishingCatches, next.FishingCatches)
	out.FishingChanged = base.FishingChanged || next.FishingChanged
	out.OperationEffectsChanged = base.OperationEffectsChanged || next.OperationEffectsChanged
	out.RequiresImmediateProjection = base.RequiresImmediateProjection || next.RequiresImmediateProjection
	out.QueueCompleted = append(base.QueueCompleted, next.QueueCompleted...)
	out.QueueCanceled = append(base.QueueCanceled, next.QueueCanceled...)
	out.EnhanceResults = append(base.EnhanceResults, next.EnhanceResults...)
	if len(out.EnhanceResults) > 100 {
		out.EnhanceResults = out.EnhanceResults[len(out.EnhanceResults)-100:]
	}
	out.WorkLoadoutUsages = append(base.WorkLoadoutUsages, next.WorkLoadoutUsages...)
	if len(out.WorkLoadoutUsages) > 100 {
		out.WorkLoadoutUsages = out.WorkLoadoutUsages[len(out.WorkLoadoutUsages)-100:]
	}
	if out.FinalWorkLoadoutID == "" {
		out.FinalWorkLoadoutID = base.FinalWorkLoadoutID
	}
	return out
}

func mergeRecoveryCounts(a, b map[string]int64) map[string]int64 {
	out := copyInt64Map(a)
	for k, v := range b {
		out[k] += v
	}
	return out
}

func (s *Store) OfflineRecoverySchedules(ctx context.Context) (map[int64]int64, error) {
	rows, err := s.db.Query(ctx, `SELECT id,user_id FROM player_offline_recoveries WHERE schedule_pending ORDER BY id LIMIT 16`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, user int64
		if err = rows.Scan(&id, &user); err != nil {
			return nil, err
		}
		out[id] = user
	}
	return out, rows.Err()
}

func (s *Store) MarkOfflineRecoveryScheduled(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries SET schedule_pending=false WHERE id=$1`, id)
	return err
}
