package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Allocate IDs in input order, including released catches, so record ties and
// catch/instance associations do not depend on INSERT RETURNING row order.
const fishingCatchBatchInsertSQL = `WITH input AS MATERIALIZED (
 SELECT ord, fish::jsonb AS fish, kept, caught_at,
        nextval(pg_get_serial_sequence('fishing_catches','id')) AS catch_id,
        CASE WHEN kept THEN nextval(pg_get_serial_sequence('player_item_instances','instance_id')) END AS instance_id
 FROM unnest($3::text[], $4::boolean[], $5::timestamptz[]) WITH ORDINALITY AS v(fish,kept,caught_at,ord)
 ORDER BY ord
), instances AS (
 INSERT INTO player_item_instances(instance_id,user_id,template_item_id,enhance_level,roll_seed,base_snapshot,affixes,custom_data,fish_data,
                                   durability,max_durability,quantity,state,created_at,updated_at)
 SELECT instance_id,$1,'fish_catch',0,0,'{}'::jsonb,'[]'::jsonb,'{}'::jsonb,fish,100,100,1,'inventory',now(),now()
 FROM input WHERE kept ORDER BY ord RETURNING instance_id
), catches AS (
 INSERT INTO fishing_catches(id,trip_id,user_id,species_id,fish_data_jsonb,kept,inventory_instance_id,caught_at,created_at)
 SELECT i.catch_id,$2,$1,i.fish->>'speciesId',i.fish,i.kept,n.instance_id,i.caught_at,now()
 FROM input i LEFT JOIN instances n ON n.instance_id=i.instance_id ORDER BY i.ord
 RETURNING id,inventory_instance_id
)
SELECT c.id,c.inventory_instance_id FROM input i JOIN catches c ON c.id=i.catch_id ORDER BY i.ord`

func insertFishingCatchBatchTx(ctx context.Context, tx pgx.Tx, userID int64, tripID string, pending []*fishingPendingCatch) error {
	if len(pending) == 0 {
		return nil
	}
	if len(pending) > fishingMaxFastForwardCatches {
		return errors.New("fishing catch insert exceeds batch limit")
	}
	fish := make([]string, len(pending))
	kept := make([]bool, len(pending))
	caughtAt := make([]time.Time, len(pending))
	for i, p := range pending {
		raw, err := json.Marshal(p.Fish)
		if err != nil {
			return err
		}
		fish[i], kept[i], caughtAt[i] = string(raw), p.Kept, p.CaughtAt
	}
	rows, err := tx.Query(ctx, fishingCatchBatchInsertSQL, userID, tripID, fish, kept, caughtAt)
	if err != nil {
		return err
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		if i >= len(pending) {
			return errors.New("fishing catch insert returned too many rows")
		}
		if err := rows.Scan(&pending[i].CatchID, &pending[i].InventoryInstanceID); err != nil {
			return err
		}
		i++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if i != len(pending) {
		return errors.New("fishing catch insert returned incomplete rows")
	}
	return nil
}
