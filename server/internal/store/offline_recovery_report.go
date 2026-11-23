package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type OfflineRecoveryReceipt struct {
	TaskID   string `json:"taskId"`
	Revision int64  `json:"revision"`
}

// A report is a read projection, not a new settlement window. Its members retain
// their original checkpoints, reward ownership and acknowledgement revisions.
type OfflineRecoveryReport struct {
	Recovery        OfflineRecovery
	Tasks           []OfflineRecoveryReceipt
	Key             string
	DurationSeconds int64
}

func (s *Store) GetOfflineRecoveryReport(ctx context.Context, userID int64) (OfflineRecoveryReport, error) {
	rows, err := s.db.Query(ctx, `SELECT `+offlineRecoveryColumns+` FROM player_offline_recoveries
 WHERE user_id=$1 AND NOT background AND acknowledged_at IS NULL ORDER BY id`, userID)
	if err != nil {
		return OfflineRecoveryReport{}, err
	}
	defer rows.Close()
	var recoveries []OfflineRecovery
	for rows.Next() {
		r, err := scanOfflineRecovery(rows)
		if err != nil {
			return OfflineRecoveryReport{}, err
		}
		recoveries = append(recoveries, r)
	}
	if err := rows.Err(); err != nil {
		return OfflineRecoveryReport{}, err
	}
	if len(recoveries) == 0 {
		return OfflineRecoveryReport{}, pgx.ErrNoRows
	}
	return buildOfflineRecoveryReport(recoveries), nil
}

func buildOfflineRecoveryReport(rows []OfflineRecovery) OfflineRecoveryReport {
	report := OfflineRecoveryReport{Recovery: rows[len(rows)-1]}
	var summary ActionFastForwardSummary
	var activated []ActivatedWorkLoadout
	blocked := false
	for _, r := range rows {
		report.Tasks = append(report.Tasks, OfflineRecoveryReceipt{TaskID: strconv.FormatInt(r.ID, 10), Revision: r.Revision})
		report.DurationSeconds += int64(r.TargetAt.Sub(r.StartedAt) / time.Second)
		summary = mergeOfflineRecoverySummary(summary, r.Summary)
		activated = append(activated, r.Summary.ActivatedWorkLoadouts...)
		if !blocked && r.Status != "complete" {
			// Retry must address the first unfinished member, not the latest row.
			report.Recovery = r
			blocked = true
		}
	}
	summary.ActivatedWorkLoadouts = activated
	summary.StalledQueueID = report.Recovery.Summary.StalledQueueID
	summary.SettledUntilMS = report.Recovery.Summary.SettledUntilMS
	report.Recovery.Summary = summary
	report.Recovery.StartedAt = rows[0].StartedAt
	report.Key = offlineRecoveryReportKey(report.Tasks)
	return report
}

func offlineRecoveryReportKey(tasks []OfflineRecoveryReceipt) string {
	raw, _ := json.Marshal(tasks)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func offlineRecoveryReceiptIDs(tasks []OfflineRecoveryReceipt) ([]int64, []int64, error) {
	ids, revisions := make([]int64, 0, len(tasks)), make([]int64, 0, len(tasks))
	seen := make(map[int64]bool, len(tasks))
	for _, task := range tasks {
		id, err := strconv.ParseInt(task.TaskID, 10, 64)
		if err != nil || id <= 0 || task.Revision < 0 || seen[id] {
			return nil, nil, errors.New("invalid offline recovery receipt")
		}
		seen[id] = true
		ids, revisions = append(ids, id), append(revisions, task.Revision)
	}
	return ids, revisions, nil
}

func (s *Store) AcknowledgeOfflineRecoveryReport(ctx context.Context, userID int64, tasks []OfflineRecoveryReceipt) error {
	ids, revisions, err := offlineRecoveryReceiptIDs(tasks)
	if err != nil || len(ids) == 0 {
		return err
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		return acknowledgeOfflineRecoveryReportTx(ctx, tx, userID, ids, revisions)
	})
}

// Reconstruct membership under the user lock so large historical backlogs do not
// require an ACK larger than the websocket's incoming frame limit. Newer windows
// are excluded; any changed member/revision makes the digest fail closed.
func (s *Store) AcknowledgeOfflineRecoveryReportKey(ctx context.Context, userID, throughID int64, key string) error {
	if throughID <= 0 || len(key) != sha256.Size*2 {
		return errors.New("invalid offline recovery report key")
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id,revision FROM player_offline_recoveries
 WHERE user_id=$1 AND id<=$2 AND NOT background AND acknowledged_at IS NULL ORDER BY id FOR UPDATE`, userID, throughID)
		if err != nil {
			return err
		}
		defer rows.Close()
		var tasks []OfflineRecoveryReceipt
		for rows.Next() {
			var id, revision int64
			if err := rows.Scan(&id, &revision); err != nil {
				return err
			}
			tasks = append(tasks, OfflineRecoveryReceipt{TaskID: strconv.FormatInt(id, 10), Revision: revision})
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(tasks) == 0 || offlineRecoveryReportKey(tasks) != key {
			return nil
		}
		ids, revisions, err := offlineRecoveryReceiptIDs(tasks)
		if err != nil {
			return err
		}
		return acknowledgeOfflineRecoveryReportTx(ctx, tx, userID, ids, revisions)
	})
}

func acknowledgeOfflineRecoveryReportTx(ctx context.Context, tx pgx.Tx, userID int64, ids, revisions []int64) error {
	// All-or-nothing even for stale/mixed-owner requests. Already acknowledged
	// members are eligible so a lost reply can be replayed without side effects.
	_, err := tx.Exec(ctx, `WITH requested AS (
 SELECT * FROM unnest($2::bigint[],$3::bigint[]) AS t(id,revision)
), eligible AS MATERIALIZED (
 SELECT j.id FROM player_offline_recoveries j JOIN requested t ON j.id=t.id AND j.revision=t.revision
 WHERE j.user_id=$1 AND j.status='complete' AND NOT j.background
 AND NOT EXISTS (
  SELECT 1 FROM unnest(j.pending_run_ids) AS pending(run_id)
  JOIN battle_runs b ON b.id=pending.run_id AND b.status=ANY($4::text[])
  WHERE NOT EXISTS(SELECT 1 FROM battle_reward_receipts r WHERE r.user_id=j.user_id AND r.run_id=pending.run_id)
  AND NOT EXISTS(SELECT 1 FROM player_offline_recoveries prior WHERE prior.user_id=j.user_id
   AND prior.started_at<j.started_at AND pending.run_id=ANY(prior.pending_run_ids))
 ) FOR UPDATE OF j
)
UPDATE player_offline_recoveries SET acknowledged_at=COALESCE(acknowledged_at,now())
WHERE id IN (SELECT id FROM eligible) AND (SELECT count(*) FROM eligible)=cardinality($2::bigint[])`,
		userID, ids, revisions, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling})
	return err
}

// EXISTS takes the union of the actual absence intervals, not the span between
// them. Each battle receipt/log is counted once, including captured late runs.
func (s *Store) GetOfflineRecoveryReportCombat(ctx context.Context, userID int64, tasks []OfflineRecoveryReceipt) (OfflineCombatRewards, error) {
	out := OfflineCombatRewards{Items: map[string]int64{}}
	ids, _, err := offlineRecoveryReceiptIDs(tasks)
	if err != nil {
		return out, err
	}
	var raw []byte
	err = s.db.QueryRow(ctx, `WITH windows AS MATERIALIZED (
 SELECT started_at,resumed_at,pending_run_ids FROM player_offline_recoveries WHERE user_id=$1 AND id=ANY($2::bigint[])
), owned_runs AS MATERIALIZED (
 SELECT DISTINCT p.run_id FROM windows w CROSS JOIN LATERAL unnest(w.pending_run_ids) p(run_id)
 WHERE NOT EXISTS(SELECT 1 FROM player_offline_recoveries prior WHERE prior.user_id=$1
  AND prior.started_at<w.started_at AND p.run_id=ANY(prior.pending_run_ids))
), receipts AS (
 SELECT r.item_rewards FROM battle_reward_receipts r WHERE r.user_id=$1 AND EXISTS(
  SELECT 1 FROM windows w WHERE ((r.settled_at>=w.started_at AND r.settled_at<w.resumed_at) OR r.run_id=ANY(w.pending_run_ids))
  AND NOT EXISTS(SELECT 1 FROM player_offline_recoveries prior WHERE prior.user_id=$1
   AND prior.started_at<w.started_at AND r.run_id=ANY(prior.pending_run_ids))
 )
), legacy AS (
 SELECT l.item_totals FROM player_warehouse_change_logs l WHERE l.user_id=$1 AND l.category='battle' AND l.direction='gain'
 AND EXISTS(SELECT 1 FROM windows w WHERE l.created_at>=w.started_at AND l.created_at<w.resumed_at)
 AND NOT EXISTS(SELECT 1 FROM battle_reward_receipts r WHERE r.user_id=$1 AND r.run_id=
  CASE WHEN l.details->>'runId' ~ '^[0-9]{1,19}$' THEN
   CASE WHEN (l.details->>'runId')::numeric<=9223372036854775807 THEN (l.details->>'runId')::bigint END END)
), entries AS (
 SELECT key,value::bigint AS quantity,false AS legacy FROM receipts CROSS JOIN LATERAL jsonb_each_text(item_rewards)
 UNION ALL SELECT key,value::bigint,true FROM legacy CROSS JOIN LATERAL jsonb_each_text(item_totals)
), totals AS (SELECT key,sum(quantity)::bigint AS quantity,bool_or(legacy) AS legacy FROM entries GROUP BY key)
SELECT COALESCE((SELECT jsonb_object_agg(key,quantity) FROM totals),'{}'::jsonb),
 COALESCE((SELECT bool_or(legacy) FROM totals),false),
 (SELECT count(*) FROM owned_runs p JOIN battle_runs b ON b.id=p.run_id AND b.status=ANY($3::text[])
  WHERE NOT EXISTS(SELECT 1 FROM battle_reward_receipts r WHERE r.user_id=$1 AND r.run_id=p.run_id))`,
		userID, ids, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling}).Scan(&raw, &out.LegacyLogs, &out.PendingRuns)
	if err == nil {
		err = json.Unmarshal(raw, &out.Items)
	}
	return out, err
}
