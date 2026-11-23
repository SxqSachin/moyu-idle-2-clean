package store

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

func (batch *fishingFastForwardCatchBatch) flushState(ctx context.Context, tx pgx.Tx, userID int64) error {
	if batch == nil {
		return nil
	}
	knowledgeIDs := make([]string, 0, len(batch.Knowledge))
	for id, state := range batch.Knowledge {
		if state != nil && (state.EncounterDelta != 0 || state.CatchDelta != 0 || state.ReleaseDelta != 0) {
			knowledgeIDs = append(knowledgeIDs, id)
		}
	}
	sort.Strings(knowledgeIDs)
	encounters, catches, releases := []int64{}, []int64{}, []int64{}
	first, last := []*time.Time{}, []*time.Time{}
	for _, id := range knowledgeIDs {
		state := batch.Knowledge[id]
		encounters = append(encounters, state.EncounterDelta)
		catches = append(catches, state.CatchDelta)
		releases = append(releases, state.ReleaseDelta)
		first = append(first, state.FirstCaughtAt)
		last = append(last, state.LastCaughtAt)
	}
	stateBatch := &pgx.Batch{}
	if len(knowledgeIDs) > 0 {
		stateBatch.Queue(`
			INSERT INTO fishing_species_knowledge(
				user_id, species_id, encounters, catches, releases, first_caught_at, last_caught_at, updated_at
			)
			SELECT $1, species_id, encounters, catches, releases, first_caught_at, last_caught_at, now()
			FROM unnest($2::text[], $3::bigint[], $4::bigint[], $5::bigint[], $6::timestamptz[], $7::timestamptz[])
			WITH ORDINALITY AS v(species_id, encounters, catches, releases, first_caught_at, last_caught_at, ordinal)
			ORDER BY ordinal
			ON CONFLICT (user_id, species_id) DO UPDATE
			SET encounters = fishing_species_knowledge.encounters + EXCLUDED.encounters,
			    catches = fishing_species_knowledge.catches + EXCLUDED.catches,
			    releases = fishing_species_knowledge.releases + EXCLUDED.releases,
			    first_caught_at = COALESCE(fishing_species_knowledge.first_caught_at, EXCLUDED.first_caught_at),
			    last_caught_at = COALESCE(EXCLUDED.last_caught_at, fishing_species_knowledge.last_caught_at),
			    updated_at = now()
		`, userID, knowledgeIDs, encounters, catches, releases, first, last)
	}
	recordIDs := make([]string, 0, len(batch.Records))
	for id, state := range batch.Records {
		if state != nil && state.Dirty {
			recordIDs = append(recordIDs, id)
		}
	}
	sort.Strings(recordIDs)
	lengths, weights := []float64{}, []float64{}
	lengthIDs, weightIDs := []*int64{}, []*int64{}
	for _, id := range recordIDs {
		state := batch.Records[id]
		lengths = append(lengths, state.BestLengthCM)
		weights = append(weights, state.BestWeightKG)
		lengthIDs = append(lengthIDs, state.BestLengthCatchID)
		weightIDs = append(weightIDs, state.BestWeightCatchID)
	}
	if len(recordIDs) > 0 {
		stateBatch.Queue(`
			INSERT INTO fishing_records(
				user_id, species_id, ruleset_version,
				best_length_cm, best_length_catch_id, best_weight_kg, best_weight_catch_id, updated_at
			)
			SELECT $1, species_id, $3, best_length_cm, best_length_catch_id, best_weight_kg, best_weight_catch_id, now()
			FROM unnest($2::text[], $4::double precision[], $5::bigint[], $6::double precision[], $7::bigint[])
			WITH ORDINALITY AS v(species_id, best_length_cm, best_length_catch_id, best_weight_kg, best_weight_catch_id, ordinal)
			ORDER BY ordinal
			ON CONFLICT (user_id, species_id, ruleset_version) DO UPDATE
			SET best_length_cm = GREATEST(fishing_records.best_length_cm, EXCLUDED.best_length_cm),
			    best_length_catch_id = CASE WHEN EXCLUDED.best_length_cm > fishing_records.best_length_cm THEN EXCLUDED.best_length_catch_id ELSE fishing_records.best_length_catch_id END,
			    best_weight_kg = GREATEST(fishing_records.best_weight_kg, EXCLUDED.best_weight_kg),
			    best_weight_catch_id = CASE WHEN EXCLUDED.best_weight_kg > fishing_records.best_weight_kg THEN EXCLUDED.best_weight_catch_id ELSE fishing_records.best_weight_catch_id END,
			    updated_at = CASE WHEN EXCLUDED.best_length_cm > fishing_records.best_length_cm OR EXCLUDED.best_weight_kg > fishing_records.best_weight_kg THEN now() ELSE fishing_records.updated_at END
		`, userID, recordIDs, batch.RulesetVersion, lengths, lengthIDs, weights, weightIDs)
	}
	if stateBatch.Len() == 0 {
		return nil
	}
	results := tx.SendBatch(ctx, stateBatch)
	for range stateBatch.Len() {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return err
		}
	}
	return results.Close()
}

// Catch insertion already precedes all scaleglow grants in the offline path.
// A plain stack can therefore be merged once without moving any fish IDs or
// changing individual ledger/achievement events. Future affix-bearing variants
// retain the original grant path.
func (s *Store) grantFishingScaleglowBatchTx(ctx context.Context, tx pgx.Tx, userID int64, tripID string, pending []*fishingPendingCatch) error {
	var quantity int64
	for _, row := range pending {
		if row.Scaleglow {
			quantity++
		}
	}
	if quantity == 0 {
		return nil
	}
	batched := s.itemStackable("scaleglow_medium") && len(s.templateDefaultAffixSlots("scaleglow_medium")) == 0
	if batched {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.GrantByTemplateTx(ctx, tx, userID, "scaleglow_medium", quantity); err != nil {
			return err
		}
	}
	for _, row := range pending {
		if !row.Scaleglow {
			continue
		}
		meta := InventoryLedgerMeta{
			SourceSurface: "fishing", TargetSurface: "inventory",
			Details: map[string]any{"type": "fishing_scaleglow", "tripId": tripID, "catchId": row.CatchID},
		}
		var err error
		if batched {
			err = s.LedgerLogOnlyTx(ctx, tx, userID, "scaleglow_medium", 1, meta)
		} else {
			err = s.LedgerGrantTemplateTx(ctx, tx, userID, "scaleglow_medium", 1, meta)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
