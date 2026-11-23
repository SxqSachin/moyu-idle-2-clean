package store

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ordinaryFishInputs struct {
	current    []fishFoodReservedIngredient
	subsequent [][]fishFoodReservedIngredient
}

// Automatic cooking uses authored, ordinary food output. Specific ingredients
// retain their single-run quality snapshot and never enter this adapter.
func (e *fishFoodExecutor) loadOrdinaryFishInputsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, limit int) (*ordinaryFishInputs, error) {
	run, ok := parseFishFoodRunContext(item.PayloadRaw)
	if !ok || !run.RunReserved || len(run.ReservedFish) == 0 || run.IngredientMode != fishFoodIngredientModeAuto || len(run.OutputBaseSnapshot) > 0 {
		return nil, ErrFishFoodIngredientsInvalid
	}
	inputs := &ordinaryFishInputs{current: run.ReservedFish}
	// Keep even unusually large recipes bounded. Duplicate species require the
	// original successive reservation owner, rather than overlapping SQL slices.
	perStart := 0
	species := map[string]bool{}
	for _, requirement := range e.recipe.FishIngredients {
		id := strings.TrimSpace(requirement.SpeciesID)
		if id == "" || requirement.Quantity <= 0 || requirement.Quantity > maxFishFoodSelectedInstances {
			return nil, ErrFishFoodIngredientsInvalid
		}
		if species[id] {
			limit = 1
		}
		species[id] = true
		perStart += requirement.Quantity
		if perStart > ordinarySegmentLimit {
			limit = 1
		}
	}
	if perStart > 0 {
		limit = min(limit, max(1, ordinarySegmentLimit/perStart))
	}
	ids := make([]int64, 0, len(run.ReservedFish))
	seen := map[int64]bool{}
	for _, fish := range run.ReservedFish {
		if seen[fish.InstanceID] {
			return nil, ErrFishFoodIngredientsInvalid
		}
		seen[fish.InstanceID] = true
		ids = append(ids, fish.InstanceID)
	}
	rows, err := tx.Query(ctx, `SELECT instance_id,user_id,template_item_id,state,quantity FROM player_item_instances WHERE instance_id=ANY($1::bigint[]) ORDER BY instance_id FOR UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	count := 0
	for rows.Next() {
		var id, owner, quantity int64
		var template, state string
		if err := rows.Scan(&id, &owner, &template, &state, &quantity); err != nil {
			rows.Close()
			return nil, err
		}
		if owner != userID || template != fishFoodCatchTemplateItemID || state != "action_queue" || quantity != 1 {
			rows.Close()
			return nil, ErrFishFoodIngredientsInvalid
		}
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count != len(ids) {
		return nil, ErrFishFoodIngredientsInvalid
	}
	if limit <= 1 {
		return inputs, nil
	}
	starts := limit - 1
	selected := make([][]fishFoodReservedIngredient, 0, len(e.recipe.FishIngredients))
	for _, requirement := range e.recipe.FishIngredients {
		if requirement.Quantity <= 0 || requirement.Quantity > maxFishFoodSelectedInstances {
			return nil, ErrFishFoodIngredientsInvalid
		}
		rows, err := tx.Query(ctx, `SELECT instance_id,fish_data FROM player_item_instances
		 WHERE user_id=$1 AND template_item_id=$2 AND state='inventory' AND archived_at IS NULL AND quantity=1 AND fish_data->>'speciesId'=$3
		 ORDER BY CASE WHEN jsonb_typeof(fish_data->'genes'->'mutations')='array' THEN jsonb_array_length(fish_data->'genes'->'mutations') ELSE 0 END,
		 COALESCE((fish_data->>'lengthCm')::double precision,0),COALESCE((fish_data->>'weightKg')::double precision,0),instance_id LIMIT $4 FOR UPDATE`, userID, fishFoodCatchTemplateItemID, strings.TrimSpace(requirement.SpeciesID), requirement.Quantity*(limit-1))
		if err != nil {
			return nil, err
		}
		var candidates []fishFoodReservedIngredient
		for rows.Next() {
			var id int64
			var raw []byte
			var fish FishInstanceData
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return nil, err
			}
			if json.Unmarshal(raw, &fish) != nil || strings.TrimSpace(fish.SpeciesID) != strings.TrimSpace(requirement.SpeciesID) {
				rows.Close()
				return nil, ErrFishFoodIngredientsInvalid
			}
			candidates = append(candidates, fishFoodReservedIngredient{InstanceID: id, SpeciesID: fish.SpeciesID})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		starts = min(starts, len(candidates)/requirement.Quantity)
		selected = append(selected, candidates)
	}
	inputs.subsequent = make([][]fishFoodReservedIngredient, starts)
	for index, requirement := range e.recipe.FishIngredients {
		for i := 0; i < starts; i++ {
			inputs.subsequent[i] = append(inputs.subsequent[i], selected[index][i*requirement.Quantity:(i+1)*requirement.Quantity]...)
		}
	}
	return inputs, nil
}

func (inputs *ordinaryFishInputs) flushTx(ctx context.Context, tx pgx.Tx, s *Store, userID int64, item QueueRuntimeItem, completed int) error {
	if completed <= 0 || completed > len(inputs.subsequent)+1 {
		return ErrFishFoodIngredientsInvalid
	}
	all := append([]fishFoodReservedIngredient(nil), inputs.current...)
	for i := 0; i < completed-1; i++ {
		all = append(all, inputs.subsequent[i]...)
	}
	ids := make([]int64, 0, len(all))
	for _, fish := range all {
		ids = append(ids, fish.InstanceID)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM player_item_instances WHERE user_id=$1 AND instance_id=ANY($2::bigint[]) AND template_item_id=$3 AND state IN ('inventory','action_queue')`, userID, ids, fishFoodCatchTemplateItemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != int64(len(ids)) {
		return ErrFishFoodIngredientsInvalid
	}
	for index, fish := range all {
		meta := InventoryLedgerMeta{Category: InventoryAuditCategoryActionQueue, Direction: InventoryAuditDirectionConsume, SubjectType: "queue", SubjectID: strconv.FormatInt(item.QueueID, 10), SourceSurface: "inventory", TargetSurface: "action_queue", InstanceID: &fish.InstanceID, Details: map[string]any{"type": "fish_food_fish_reserve", "queueId": item.QueueID, "actionId": item.ActionID, "speciesId": fish.SpeciesID, "phase": item.AuditPhase, "settlementVersion": ordinarySettlementVersion}}
		if index >= len(inputs.current) {
			if err := s.LedgerLogOnlyTx(ctx, tx, userID, fishFoodCatchTemplateItemID, 1, meta); err != nil {
				return err
			}
		}
		meta.SourceSurface = "action_queue"
		meta.TargetSurface = "fish_food"
		meta.Details = map[string]any{"type": "fish_food_fish_consume", "queueId": item.QueueID, "actionId": item.ActionID, "speciesId": fish.SpeciesID, "phase": item.AuditPhase, "settlementVersion": ordinarySettlementVersion}
		if err := s.LedgerLogOnlyTx(ctx, tx, userID, fishFoodCatchTemplateItemID, 1, meta); err != nil {
			return err
		}
	}
	return nil
}
