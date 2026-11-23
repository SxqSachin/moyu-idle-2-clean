package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) recordBattleRewardReceiptTx(ctx context.Context, tx pgx.Tx, runID, userID int64, items map[string]int64) (bool, error) {
	if items == nil {
		items = map[string]int64{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO battle_reward_receipts(run_id,user_id,item_rewards) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, runID, userID, string(raw))
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}
	var equal bool
	if err = tx.QueryRow(ctx, `SELECT item_rewards=$3::jsonb FROM battle_reward_receipts WHERE run_id=$1 AND user_id=$2`, runID, userID, string(raw)).Scan(&equal); err != nil {
		return false, err
	}
	if !equal {
		return false, errors.New("battle reward receipt conflicts with reward plan")
	}
	return false, nil
}

type OfflineCombatRewards struct {
	Items       map[string]int64
	PendingRuns int
	LegacyLogs  bool
}

const offlineCombatRewardsSQL = `WITH previous_runs AS MATERIALIZED (
  SELECT unnest(pending_run_ids) AS run_id FROM player_offline_recoveries WHERE user_id=$1 AND started_at<$2
 ), receipts AS (
  SELECT item_rewards FROM battle_reward_receipts r WHERE user_id=$1 AND ((settled_at>=$2 AND settled_at<$3) OR run_id=ANY($4::bigint[]))
   AND NOT EXISTS(SELECT 1 FROM previous_runs p WHERE p.run_id=r.run_id)
 ), legacy AS (
  SELECT l.item_totals FROM player_warehouse_change_logs l WHERE l.user_id=$1 AND l.category='battle' AND l.direction='gain'
   AND l.created_at>=$2 AND l.created_at<$3
   AND NOT EXISTS(SELECT 1 FROM battle_reward_receipts r WHERE r.user_id=$1 AND r.run_id=
    CASE WHEN l.details->>'runId' ~ '^[0-9]{1,19}$' THEN
     CASE WHEN (l.details->>'runId')::numeric<=9223372036854775807 THEN (l.details->>'runId')::bigint END END)
 ), entries AS (
  SELECT key,value::bigint AS quantity,false AS legacy FROM receipts CROSS JOIN LATERAL jsonb_each_text(item_rewards)
  UNION ALL SELECT key,value::bigint,true FROM legacy CROSS JOIN LATERAL jsonb_each_text(item_totals)
 ), totals AS (SELECT key,sum(quantity)::bigint AS quantity,bool_or(legacy) AS legacy FROM entries GROUP BY key)
 SELECT COALESCE((SELECT jsonb_object_agg(key,quantity) FROM totals),'{}'::jsonb),
 COALESCE((SELECT bool_or(legacy) FROM totals),false),
 (SELECT count(*) FROM unnest($4::bigint[]) AS p(run_id)
  WHERE NOT EXISTS(SELECT 1 FROM battle_reward_receipts r WHERE r.user_id=$1 AND r.run_id=p.run_id)
  AND NOT EXISTS(SELECT 1 FROM previous_runs prior WHERE prior.run_id=p.run_id)
  AND EXISTS(SELECT 1 FROM battle_runs b WHERE b.id=p.run_id AND b.status=ANY($5::text[])))`

// Pending runs are captured at reconnect, so a late settlement is still visible
// after the fixed offline cutoff. Legacy logs are explicitly non-authoritative.
func (s *Store) GetOfflineCombatRewards(ctx context.Context, userID int64, since, until time.Time, pendingRunIDs []int64) (OfflineCombatRewards, error) {
	out := OfflineCombatRewards{Items: map[string]int64{}}
	if pendingRunIDs == nil {
		pendingRunIDs = []int64{}
	}
	var raw []byte
	err := s.db.QueryRow(ctx, offlineCombatRewardsSQL, userID, since, until, pendingRunIDs, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling}).Scan(&raw, &out.LegacyLogs, &out.PendingRuns)
	if err == nil {
		err = json.Unmarshal(raw, &out.Items)
	}
	return out, err
}
