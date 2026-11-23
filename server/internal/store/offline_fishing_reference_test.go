package store

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

// Frozen pipelined INSERT path used before bulk persistence.
func insertFishingCatchBatchReferenceTx(ctx context.Context, tx pgx.Tx, userID int64, tripID string, pending []*fishingPendingCatch) error {
	batch := &fishingFastForwardCatchBatch{Pending: pending}
	if len(batch.Pending) > 0 {
		insertBatch := &pgx.Batch{}
		for _, pending := range batch.Pending {
			fishRaw, err := json.Marshal(pending.Fish)
			if err != nil {
				return err
			}
			insertBatch.Queue(`
				WITH inserted_instance AS (
					INSERT INTO player_item_instances(
						user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes, custom_data, fish_data,
						durability, max_durability, quantity, state, created_at, updated_at
					)
					SELECT $1, 'fish_catch', 0, 0, '{}'::jsonb, '[]'::jsonb, '{}'::jsonb, $2::jsonb,
					       100, 100, 1, 'inventory', now(), now()
					WHERE $3
					RETURNING instance_id
				), inserted_catch AS (
					INSERT INTO fishing_catches(trip_id, user_id, species_id, fish_data_jsonb, kept, inventory_instance_id, caught_at, created_at)
					VALUES($4, $1, $5, $2::jsonb, $3, (SELECT instance_id FROM inserted_instance), $6, now())
					RETURNING id, inventory_instance_id
				)
				SELECT id, inventory_instance_id FROM inserted_catch
			`, userID, string(fishRaw), pending.Kept, tripID, pending.Fish.SpeciesID, pending.CaughtAt)
		}
		results := tx.SendBatch(ctx, insertBatch)
		for _, pending := range batch.Pending {
			if err := results.QueryRow().Scan(&pending.CatchID, &pending.InventoryInstanceID); err != nil {
				_ = results.Close()
				return err
			}
		}
		if err := results.Close(); err != nil {
			return err
		}
	}

	return nil
}
