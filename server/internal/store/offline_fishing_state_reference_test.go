package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"sort"
)

// Frozen pre-bulk state persistence for full database A/B comparisons.
func (batch *fishingFastForwardCatchBatch) flushStateReference(ctx context.Context, tx pgx.Tx, userID int64) error {
	if batch == nil {
		return nil
	}
	stateBatch := &pgx.Batch{}
	statementCount := 0
	knowledgeIDs := make([]string, 0, len(batch.Knowledge))
	for speciesID := range batch.Knowledge {
		knowledgeIDs = append(knowledgeIDs, speciesID)
	}
	sort.Strings(knowledgeIDs)
	for _, speciesID := range knowledgeIDs {
		state := batch.Knowledge[speciesID]
		if state == nil || (state.EncounterDelta == 0 && state.CatchDelta == 0 && state.ReleaseDelta == 0) {
			continue
		}
		stateBatch.Queue(`
			INSERT INTO fishing_species_knowledge(
				user_id, species_id, encounters, catches, releases, first_caught_at, last_caught_at, updated_at
			)
			VALUES($1, $2, $3, $4, $5, $6, $7, now())
			ON CONFLICT (user_id, species_id) DO UPDATE
			SET encounters = fishing_species_knowledge.encounters + EXCLUDED.encounters,
			    catches = fishing_species_knowledge.catches + EXCLUDED.catches,
			    releases = fishing_species_knowledge.releases + EXCLUDED.releases,
			    first_caught_at = COALESCE(fishing_species_knowledge.first_caught_at, EXCLUDED.first_caught_at),
			    last_caught_at = COALESCE(EXCLUDED.last_caught_at, fishing_species_knowledge.last_caught_at),
			    updated_at = now()
		`, userID, speciesID, state.EncounterDelta, state.CatchDelta, state.ReleaseDelta, state.FirstCaughtAt, state.LastCaughtAt)
		statementCount++
	}
	recordIDs := make([]string, 0, len(batch.Records))
	for speciesID := range batch.Records {
		recordIDs = append(recordIDs, speciesID)
	}
	sort.Strings(recordIDs)
	for _, speciesID := range recordIDs {
		state := batch.Records[speciesID]
		if state == nil || !state.Dirty {
			continue
		}
		stateBatch.Queue(`
			INSERT INTO fishing_records(
				user_id, species_id, ruleset_version,
				best_length_cm, best_length_catch_id, best_weight_kg, best_weight_catch_id, updated_at
			)
			VALUES($1, $2, $3, $4, $5, $6, $7, now())
			ON CONFLICT (user_id, species_id, ruleset_version) DO UPDATE
			SET best_length_cm = GREATEST(fishing_records.best_length_cm, EXCLUDED.best_length_cm),
			    best_length_catch_id = CASE WHEN EXCLUDED.best_length_cm > fishing_records.best_length_cm THEN EXCLUDED.best_length_catch_id ELSE fishing_records.best_length_catch_id END,
			    best_weight_kg = GREATEST(fishing_records.best_weight_kg, EXCLUDED.best_weight_kg),
			    best_weight_catch_id = CASE WHEN EXCLUDED.best_weight_kg > fishing_records.best_weight_kg THEN EXCLUDED.best_weight_catch_id ELSE fishing_records.best_weight_catch_id END,
			    updated_at = CASE WHEN EXCLUDED.best_length_cm > fishing_records.best_length_cm OR EXCLUDED.best_weight_kg > fishing_records.best_weight_kg THEN now() ELSE fishing_records.updated_at END
		`, userID, speciesID, batch.RulesetVersion, state.BestLengthCM, state.BestLengthCatchID, state.BestWeightKG, state.BestWeightCatchID)
		statementCount++
	}
	if statementCount == 0 {
		return nil
	}
	results := tx.SendBatch(ctx, stateBatch)
	for index := 0; index < statementCount; index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return err
		}
	}
	return results.Close()
}
