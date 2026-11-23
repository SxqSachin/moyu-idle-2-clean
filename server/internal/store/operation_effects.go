package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/enchantment"
	fishingeffect "mysecondwebgame/server/internal/operationeffect/fishing"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
	"mysecondwebgame/server/internal/operationeffect/transmute"

	"github.com/jackc/pgx/v5"
)

var (
	ErrOperationEffectNotFound            = errors.New("operation effect definition not found")
	ErrOperationEffectTargetInvalid       = errors.New("operation effect target invalid")
	ErrOperationEffectTargetNotOwned      = errors.New("operation effect target not owned")
	ErrOperationEffectItemInvalid         = errors.New("operation effect item invalid")
	ErrOperationEffectAtCap               = errors.New("operation effect is already at its stacking cap")
	ErrOperationEffectStrengthConflict    = errors.New("a different strength of the operation effect is already active")
	ErrOperationEffectRequestInvalid      = errors.New("operation effect request invalid")
	ErrOperationEffectIdempotencyConflict = errors.New("operation effect idempotency conflict")
	ErrOperationEffectRecoveryPending     = errors.New("operation effect charge replenishment requires queue recovery")
)

const operationEffectItemSnapshotSchemaVersion = 1

type operationEffectItemSnapshot struct {
	SchemaVersion  int                                 `json:"schemaVersion"`
	DefinitionID   string                              `json:"definitionId"`
	RulesetVersion string                              `json:"rulesetVersion"`
	Params         json.RawMessage                     `json:"params"`
	Lifetime       *operationeffect.LifetimeDefinition `json:"lifetime,omitempty"`
}

type OperationEffectDefinitionView struct {
	ID             string                             `json:"id"`
	RulesetVersion string                             `json:"rulesetVersion"`
	Domain         operationeffect.Domain             `json:"domain"`
	Kind           string                             `json:"kind"`
	NameKey        string                             `json:"nameKey"`
	DescKey        string                             `json:"descKey"`
	IconID         string                             `json:"iconId,omitempty"`
	AllowedTargets []operationeffect.TargetKind       `json:"allowedTargets"`
	Lifetime       operationeffect.LifetimeDefinition `json:"lifetime"`
	Stacking       operationeffect.StackingDefinition `json:"stacking"`
	Selector       operationeffect.Selector           `json:"selector,omitempty"`
	Params         json.RawMessage                    `json:"params"`
}

type OperationEffectView struct {
	GrantID          int64                              `json:"grantId"`
	DefinitionID     string                             `json:"definitionId"`
	RulesetVersion   string                             `json:"rulesetVersion"`
	Domain           operationeffect.Domain             `json:"domain"`
	Kind             string                             `json:"kind"`
	NameKey          string                             `json:"nameKey"`
	DescKey          string                             `json:"descKey"`
	IconID           string                             `json:"iconId,omitempty"`
	TargetKind       operationeffect.TargetKind         `json:"targetKind"`
	TargetActorID    int64                              `json:"targetActorId,omitempty"`
	LifetimeKind     operationeffect.LifetimeKind       `json:"lifetimeKind"`
	ValidFrom        time.Time                          `json:"validFrom"`
	ValidUntil       *time.Time                         `json:"validUntil,omitempty"`
	InitialCharges   int64                              `json:"initialCharges,omitempty"`
	RemainingCharges int64                              `json:"remainingCharges,omitempty"`
	SourceItemID     string                             `json:"sourceItemId,omitempty"`
	Selector         operationeffect.Selector           `json:"selector,omitempty"`
	Params           json.RawMessage                    `json:"params"`
	Stacking         operationeffect.StackingDefinition `json:"stacking"`
}

type OperationEffectProjection struct {
	Revision  int64                 `json:"revision"`
	ServerNow time.Time             `json:"serverNow"`
	Items     []OperationEffectView `json:"items"`
}

func buildOperationEffectRuntime(gameData catalog.Data) (*operationeffect.Registry, map[string]operationeffect.Definition, error) {
	registry := operationeffect.NewRegistry()
	if err := transmute.Register(registry); err != nil {
		return nil, nil, err
	}
	if err := enchantment.Register(registry); err != nil {
		return nil, nil, err
	}
	if err := lifeaction.Register(registry); err != nil {
		return nil, nil, err
	}
	if err := alchemyeffect.Register(registry); err != nil {
		return nil, nil, err
	}
	if err := ratebonus.Register(registry); err != nil {
		return nil, nil, err
	}
	if err := fishingeffect.Register(registry); err != nil {
		return nil, nil, err
	}
	definitions := make(map[string]operationeffect.Definition, len(gameData.OperationEffects))
	for _, raw := range gameData.OperationEffects {
		def := operationEffectDefinitionFromCatalog(raw)
		if err := registry.ValidateDefinition(def); err != nil {
			return nil, nil, err
		}
		if _, exists := definitions[def.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate operation effect definition: %s", def.ID)
		}
		definitions[def.ID] = def
	}
	return registry, definitions, nil
}

func operationEffectDefinitionFromCatalog(raw catalog.OperationEffectDefinition) operationeffect.Definition {
	targets := make([]operationeffect.TargetKind, 0, len(raw.AllowedTargets))
	for _, target := range raw.AllowedTargets {
		targets = append(targets, operationeffect.TargetKind(strings.TrimSpace(target)))
	}
	return operationeffect.Definition{
		ID:             strings.TrimSpace(raw.ID),
		RulesetVersion: strings.TrimSpace(raw.RulesetVersion),
		Domain:         operationeffect.Domain(strings.TrimSpace(raw.Domain)),
		Kind:           strings.TrimSpace(raw.Kind),
		NameKey:        strings.TrimSpace(raw.NameKey),
		DescKey:        strings.TrimSpace(raw.DescKey),
		IconID:         strings.TrimSpace(raw.IconID),
		Priority:       raw.Priority,
		AllowedTargets: targets,
		Lifetime: operationeffect.LifetimeDefinition{
			Kind:            operationeffect.LifetimeKind(strings.TrimSpace(raw.Lifetime.Kind)),
			DurationSeconds: raw.Lifetime.DurationSeconds,
			Charges:         raw.Lifetime.Charges,
		},
		Stacking: operationeffect.StackingDefinition{
			Exclusive:          raw.Stacking.Exclusive,
			Key:                strings.TrimSpace(raw.Stacking.Key),
			Mode:               operationeffect.StackMode(strings.TrimSpace(raw.Stacking.Mode)),
			MaxDurationSeconds: raw.Stacking.MaxDurationSeconds,
			MaxCharges:         raw.Stacking.MaxCharges,
		},
		Selector: operationeffect.Selector{
			ActionIDs: append([]string(nil), raw.Selector.ActionIDs...),
			MainTypes: append([]string(nil), raw.Selector.MainTypes...),
			SubTypes:  append([]string(nil), raw.Selector.SubTypes...),
		},
		Params: append([]byte(nil), raw.Params...),
	}
}

func operationEffectDefinitionView(def operationeffect.Definition) OperationEffectDefinitionView {
	return OperationEffectDefinitionView{
		ID:             def.ID,
		RulesetVersion: def.RulesetVersion,
		Domain:         def.Domain,
		Kind:           def.Kind,
		NameKey:        def.NameKey,
		DescKey:        def.DescKey,
		IconID:         def.IconID,
		AllowedTargets: append([]operationeffect.TargetKind(nil), def.AllowedTargets...),
		Lifetime:       def.Lifetime,
		Stacking:       def.Stacking,
		Selector:       def.Selector,
		Params:         append(json.RawMessage(nil), def.Params...),
	}
}

func (s *Store) OperationEffectDefinition(id string) (OperationEffectDefinitionView, bool) {
	if s == nil {
		return OperationEffectDefinitionView{}, false
	}
	def, ok := s.operationEffectDefinitions[strings.TrimSpace(id)]
	if !ok {
		return OperationEffectDefinitionView{}, false
	}
	return operationEffectDefinitionView(def), true
}

func (s *Store) OperationEffectDefinitionForItem(itemID string, baseSnapshot map[string]any) (OperationEffectDefinitionView, bool, error) {
	baseRaw, err := json.Marshal(baseSnapshot)
	if err != nil {
		return OperationEffectDefinitionView{}, false, err
	}
	def, ok, err := s.resolveOperationEffectDefinitionForItem(itemID, baseRaw)
	if err != nil || !ok {
		return OperationEffectDefinitionView{}, ok, err
	}
	view := operationEffectDefinitionView(def)
	view.Stacking.Key, _, view.Stacking.Exclusive = s.operationEffectExclusiveFamily(def)
	return view, true, nil
}

func (s *Store) resolveOperationEffectDefinitionForItem(itemID string, baseSnapshotRaw []byte) (operationeffect.Definition, bool, error) {
	if s == nil {
		return operationeffect.Definition{}, false, nil
	}
	itemDef := s.gameData.ItemByID(strings.TrimSpace(itemID))
	if itemDef == nil || !itemDef.HasType("food") || strings.TrimSpace(itemDef.UseOperationEffectID) == "" {
		return operationeffect.Definition{}, false, nil
	}
	def, ok := s.operationEffectDefinitions[strings.TrimSpace(itemDef.UseOperationEffectID)]
	if !ok {
		return operationeffect.Definition{}, false, ErrOperationEffectNotFound
	}
	if len(baseSnapshotRaw) == 0 || string(baseSnapshotRaw) == "{}" || string(baseSnapshotRaw) == "null" {
		return def, true, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(baseSnapshotRaw, &envelope); err != nil {
		return operationeffect.Definition{}, false, errors.Join(ErrOperationEffectItemInvalid, err)
	}
	raw, hasOverride := envelope["operationEffect"]
	if !hasOverride {
		return def, true, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var snapshot operationEffectItemSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return operationeffect.Definition{}, false, errors.Join(ErrOperationEffectItemInvalid, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return operationeffect.Definition{}, false, ErrOperationEffectItemInvalid
	}
	current := def
	recipe := s.gameData.FishFoodRecipeByOutputItemID(itemID)
	replaced := false
	if snapshot.DefinitionID != def.ID {
		allowed := false
		for _, recipe := range s.gameData.FishFoodRecipes {
			if recipe.OutputItemID != itemID {
				continue
			}
			for _, id := range recipe.LegacyEffectIDs {
				if id == snapshot.DefinitionID {
					allowed = true
				}
			}
			for _, id := range recipe.ReplacedEffectIDs {
				if id == snapshot.DefinitionID {
					allowed, replaced = true, true
				}
			}
		}
		legacy, exists := s.operationEffectDefinitions[snapshot.DefinitionID]
		if !allowed || !exists {
			return operationeffect.Definition{}, false, ErrOperationEffectItemInvalid
		}
		def = legacy
	}
	if (snapshot.SchemaVersion != operationEffectItemSnapshotSchemaVersion && snapshot.SchemaVersion != 2) ||
		strings.TrimSpace(snapshot.DefinitionID) != def.ID ||
		strings.TrimSpace(snapshot.RulesetVersion) != def.RulesetVersion ||
		len(snapshot.Params) == 0 || string(snapshot.Params) == "null" {
		return operationeffect.Definition{}, false, ErrOperationEffectItemInvalid
	}
	def.Params = append(json.RawMessage(nil), snapshot.Params...)
	if snapshot.Lifetime != nil {
		if snapshot.SchemaVersion != 2 || snapshot.Lifetime.Kind != def.Lifetime.Kind {
			return operationeffect.Definition{}, false, ErrOperationEffectItemInvalid
		}
		lifetime := *snapshot.Lifetime
		maxCharges, maxSeconds := int64(1), int64(900)
		if recipe != nil && def.ID == current.ID {
			maxCharges, maxSeconds = fishFoodLifetimeBonus(*recipe, nil)
			if recipe.LifetimeScaling != nil && recipe.LifetimeScaling.Charges != nil {
				maxCharges = int64(recipe.LifetimeScaling.Charges.Max) - def.Lifetime.Charges
			}
			if recipe.LifetimeScaling != nil && recipe.LifetimeScaling.DurationSeconds != nil {
				maxSeconds = int64(recipe.LifetimeScaling.DurationSeconds.Max) - def.Lifetime.DurationSeconds
			}
		}
		if replaced {
			maxCharges = def.Stacking.MaxCharges - def.Lifetime.Charges
			maxSeconds = def.Stacking.MaxDurationSeconds - def.Lifetime.DurationSeconds
		}
		if lifetime.Kind == operationeffect.LifetimeCharges {
			if lifetime.Charges < def.Lifetime.Charges || lifetime.Charges > def.Lifetime.Charges+maxCharges || lifetime.DurationSeconds != 0 {
				return operationeffect.Definition{}, false, ErrOperationEffectItemInvalid
			}
		} else if lifetime.DurationSeconds < def.Lifetime.DurationSeconds || lifetime.DurationSeconds > def.Lifetime.DurationSeconds+maxSeconds || lifetime.Charges != 0 {
			return operationeffect.Definition{}, false, ErrOperationEffectItemInvalid
		}
		def.Lifetime = lifetime
	}
	if err := s.operationEffectRegistry.ValidateDefinition(def); err != nil {
		return operationeffect.Definition{}, false, errors.Join(ErrOperationEffectItemInvalid, err)
	}
	if replaced {
		converted, err := s.replaceFishFoodEffect(*recipe, current, envelope["fishFood"])
		return converted, err == nil, err
	}
	// Preserve the earliest dishes' rolled values and servings, but new uses
	// must follow the same material eligibility as current capacity food.
	if current.Kind == alchemyeffect.KindInfusionBulkCapacityFloorTradeoff && def.Kind == alchemyeffect.KindInfusionCapacityFloorTradeoff {
		def.Kind = current.Kind
		def.RulesetVersion += ":bulk-materials"
	}
	// Historical dishes retain their rolled values and lifetime, but eating
	// them now uses the current recipe's audience. Existing grants stay frozen.
	if recipe != nil {
		def.AllowedTargets = append([]operationeffect.TargetKind(nil), current.AllowedTargets...)
		// Storage capacity is a current policy, not part of a food's rolled strength.
		def.Stacking.MaxCharges = current.Stacking.MaxCharges
		def.Stacking.MaxDurationSeconds = current.Stacking.MaxDurationSeconds
	}
	return def, true, nil
}

func (s *Store) ActivateOperationEffect(ctx context.Context, userID, instanceID int64, target operationeffect.Target, requestID string, now time.Time) (OperationEffectProjection, error) {
	return s.ActivateOperationEffectQuantity(ctx, userID, instanceID, target, requestID, now, 1)
}

const maxOperationEffectUseQuantity int64 = 10000

func (s *Store) ActivateOperationEffectQuantity(ctx context.Context, userID, instanceID int64, target operationeffect.Target, requestID string, now time.Time, quantity int64) (OperationEffectProjection, error) {
	if userID <= 0 || instanceID <= 0 || quantity < 1 || quantity > maxOperationEffectUseQuantity {
		return OperationEffectProjection{}, ErrOperationEffectRequestInvalid
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 160 {
		return OperationEffectProjection{}, ErrOperationEffectRequestInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var existingInstanceID, existingQuantity int64
		var existingTargetKind operationeffect.TargetKind
		var existingTargetActorID *int64
		err := tx.QueryRow(ctx, `
			SELECT source_instance_id, target_kind, target_actor_id, quantity
			FROM player_operation_effect_activations
			WHERE user_id = $1 AND request_id = $2
		`, userID, requestID).Scan(&existingInstanceID, &existingTargetKind, &existingTargetActorID, &existingQuantity)
		if err == nil {
			if target.Kind == "" {
				target.Kind = existingTargetKind
				if existingTargetActorID != nil {
					target.ActorID = *existingTargetActorID
				}
			}
			targetActorMatches := (existingTargetActorID == nil && target.ActorID == 0) || (existingTargetActorID != nil && *existingTargetActorID == target.ActorID)
			if existingInstanceID != instanceID || existingTargetKind != target.Kind || !targetActorMatches || existingQuantity != quantity {
				return ErrOperationEffectIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		locked, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		sourceItemID := strings.TrimSpace(locked.TemplateItem)
		itemDef := s.gameData.ItemByID(sourceItemID)
		if locked.UserID != userID || strings.TrimSpace(locked.State) != "inventory" || itemDef == nil || !itemDef.HasType("food") || strings.TrimSpace(itemDef.UseOperationEffectID) == "" {
			return ErrOperationEffectItemInvalid
		}
		if locked.Quantity < quantity {
			return ErrInsufficientItem
		}
		def, ok, err := s.resolveOperationEffectDefinitionForItem(sourceItemID, locked.BaseSnapshot)
		if err != nil {
			return err
		}
		if !ok {
			return ErrOperationEffectNotFound
		}
		if target.Kind == "" && len(def.AllowedTargets) == 1 && def.AllowedTargets[0] != operationeffect.TargetKitty {
			target.Kind = def.AllowedTargets[0]
		}
		if err := operationeffect.ValidateTarget(def, target); err != nil {
			return errors.Join(ErrOperationEffectTargetInvalid, err)
		}
		if target.Kind == operationeffect.TargetKitty {
			var owned bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_kitties WHERE user_id=$1 AND id=$2)`, userID, target.ActorID).Scan(&owned); err != nil {
				return err
			}
			if !owned {
				return ErrOperationEffectTargetNotOwned
			}
		}
		definitionRaw, err := json.Marshal(def)
		if err != nil {
			return err
		}
		grantID, err := s.activateOperationEffectGrantQuantityTx(ctx, tx, userID, sourceItemID, def, target, definitionRaw, now, quantity)
		if err != nil {
			return err
		}
		if err := s.LedgerConsumeInstanceTx(ctx, tx, userID, instanceID, sourceItemID, quantity, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryOperationEffect,
			Direction:     InventoryAuditDirectionConsume,
			SubjectType:   "operation_effect",
			SubjectID:     def.ID,
			SourceSurface: "inventory",
			TargetSurface: "operation_effects",
			Details: map[string]any{
				"type":       "operation_effect_activation",
				"definition": def.ID,
				"grantId":    grantID,
				"targetKind": target.Kind,
				"quantity":   quantity,
			},
		}); err != nil {
			return err
		}
		var targetActorID *int64
		if target.Kind == operationeffect.TargetKitty {
			targetActorID = &target.ActorID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_operation_effect_activations(
				user_id, request_id, grant_id, source_instance_id, source_item_id,
				definition_id, target_kind, target_actor_id, created_at, quantity
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, userID, requestID, grantID, instanceID, sourceItemID, def.ID, target.Kind, targetActorID, now, quantity); err != nil {
			return err
		}
		return s.bumpOperationEffectRevisionTx(ctx, tx, userID, now)
	})
	if err != nil {
		return OperationEffectProjection{}, err
	}
	return s.ListOperationEffects(ctx, userID, now)
}

func (s *Store) activateOperationEffectGrantTx(ctx context.Context, tx pgx.Tx, userID int64, sourceItemID string, def operationeffect.Definition, target operationeffect.Target, definitionRaw []byte, now time.Time) (int64, error) {
	return s.activateOperationEffectGrantQuantityTx(ctx, tx, userID, sourceItemID, def, target, definitionRaw, now, 1)
}

func (s *Store) activateOperationEffectGrantQuantityTx(ctx context.Context, tx pgx.Tx, userID int64, sourceItemID string, def operationeffect.Definition, target operationeffect.Target, definitionRaw []byte, now time.Time, quantity int64) (int64, error) {
	amount, err := operationEffectServingAmount(def, quantity)
	if err != nil {
		return 0, err
	}
	if err := operationeffect.ValidateTarget(def, target); err != nil {
		return 0, errors.Join(ErrOperationEffectTargetInvalid, err)
	}
	{
		groupKey, relatedIDs, exclusive := s.operationEffectExclusiveFamily(def)
		rows, err := tx.Query(ctx, `
			SELECT definition_snapshot_jsonb FROM player_operation_effect_grants
			WHERE user_id=$1 AND (stacking_key=$2 OR definition_id=ANY($7::text[]))
			AND NOT (definition_id=$3 AND ruleset_version=$9 AND target_kind=$5 AND COALESCE(target_actor_id,0)=$6)
			AND ($8 OR definition_snapshot_jsonb->'stacking'->>'exclusive'='true')
			AND (remaining_charges>0 OR valid_until>$4)
			AND (target_kind=$5 AND COALESCE(target_actor_id,0)=$6
			 OR target_kind='player_and_kitties' OR $5='player_and_kitties'
			 OR target_kind='all_kitties' AND $5='kitty' OR target_kind='kitty' AND $5='all_kitties')
		`, userID, groupKey, def.ID, now, target.Kind, target.ActorID, relatedIDs, exclusive, def.RulesetVersion)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				rows.Close()
				return 0, err
			}
			var existing operationeffect.Definition
			if err := json.Unmarshal(raw, &existing); err != nil {
				rows.Close()
				return 0, err
			}
			if !s.isRetiredFishFoodEffect(existing) {
				rows.Close()
				return 0, ErrOperationEffectStrengthConflict
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return 0, err
		}
	}
	if def.Stacking.Mode != operationeffect.StackIndependent {
		var existingID, initialCharges, remainingCharges int64
		var validUntil *time.Time
		var existingDefinitionRaw []byte
		err := tx.QueryRow(ctx, `
			SELECT id, valid_until, initial_charges, remaining_charges, definition_snapshot_jsonb
			FROM player_operation_effect_grants
			WHERE user_id=$1 AND definition_id=$2 AND ruleset_version=$3 AND stacking_key=$4
			  AND target_kind=$5 AND target_actor_id IS NOT DISTINCT FROM $6
			  AND ((lifetime_kind='duration' AND valid_until > $7) OR (lifetime_kind='charges' AND remaining_charges > 0))
			ORDER BY id
			LIMIT 1
			FOR UPDATE
		`, userID, def.ID, def.RulesetVersion, def.Stacking.Key, target.Kind, nullableTargetActorID(target), now).Scan(&existingID, &validUntil, &initialCharges, &remainingCharges, &existingDefinitionRaw)
		if err == nil {
			var existingDef operationeffect.Definition
			if json.Unmarshal(existingDefinitionRaw, &existingDef) != nil || !operationEffectParamsEqual(existingDef.Params, def.Params) {
				return 0, ErrOperationEffectStrengthConflict
			}
			switch def.Lifetime.Kind {
			case operationeffect.LifetimeDuration:
				base := now
				if validUntil != nil && validUntil.After(base) {
					base = *validUntil
				}
				next := base.Add(time.Duration(amount) * time.Second)
				maxDurationSeconds := def.Stacking.MaxDurationSeconds
				if maxDurationSeconds == 0 {
					maxDurationSeconds = operationeffect.MaxDurationSeconds
				}
				capAt := now.Add(time.Duration(maxDurationSeconds) * time.Second)
				if next.After(capAt) {
					if def.Stacking.Exclusive || quantity > 1 {
						return 0, ErrOperationEffectAtCap
					}
					next = capAt
				}
				if validUntil != nil && !next.After(*validUntil) {
					return 0, ErrOperationEffectAtCap
				}
				_, err = tx.Exec(ctx, `UPDATE player_operation_effect_grants SET valid_until=$2, updated_at=$3 WHERE id=$1`, existingID, next, now)
			case operationeffect.LifetimeCharges:
				maxCharges := def.Stacking.MaxCharges
				if maxCharges == 0 {
					maxCharges = operationeffect.MaxChargeCount
				}
				added := amount
				if remainingCharges >= maxCharges {
					return 0, ErrOperationEffectAtCap
				}
				if added > maxCharges-remainingCharges {
					if def.Stacking.Exclusive || quantity > 1 {
						return 0, ErrOperationEffectAtCap
					}
					added = maxCharges - remainingCharges
				}
				nextRemaining := remainingCharges + added
				if added <= 0 {
					return 0, ErrOperationEffectAtCap
				}
				// New charges must not inherit the old grant's validFrom while
				// historical starts are still waiting to be settled.
				if err := s.ensureOperationEffectChargeReplenishmentCaughtUpTx(ctx, tx, userID, now); err != nil {
					return 0, err
				}
				nextInitial := initialCharges
				if nextRemaining > nextInitial {
					nextInitial = nextRemaining
				}
				_, err = tx.Exec(ctx, `
					UPDATE player_operation_effect_grants
					SET initial_charges=$2, remaining_charges=$3, updated_at=$4
					WHERE id=$1
				`, existingID, nextInitial, nextRemaining, now)
			}
			return existingID, err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
	}
	var validUntil *time.Time
	initialCharges, remainingCharges := int64(0), int64(0)
	if def.Lifetime.Kind == operationeffect.LifetimeDuration {
		until := now.Add(time.Duration(amount) * time.Second)
		validUntil = &until
	} else {
		initialCharges = amount
		remainingCharges = amount
	}
	var grantID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO player_operation_effect_grants(
			user_id, definition_id, ruleset_version, domain, effect_kind, stacking_key, priority,
			target_kind, target_actor_id, lifetime_kind, valid_from, valid_until,
			initial_charges, remaining_charges, definition_snapshot_jsonb, source_item_id, created_at, updated_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb,$16,$11,$11)
		RETURNING id
	`, userID, def.ID, def.RulesetVersion, def.Domain, def.Kind, def.Stacking.Key, def.Priority,
		target.Kind, nullableTargetActorID(target), def.Lifetime.Kind, now, validUntil,
		initialCharges, remainingCharges, string(definitionRaw), sourceItemID).Scan(&grantID)
	return grantID, err
}

func operationEffectParamsEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func nullableTargetActorID(target operationeffect.Target) *int64 {
	if target.Kind != operationeffect.TargetKitty || target.ActorID <= 0 {
		return nil
	}
	value := target.ActorID
	return &value
}

func (s *Store) ListOperationEffects(ctx context.Context, userID int64, now time.Time) (OperationEffectProjection, error) {
	if now.IsZero() {
		now = time.Now()
	}
	projection := OperationEffectProjection{ServerNow: now, Items: []OperationEffectView{}}
	err := s.withReadOnlyTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT revision FROM player_operation_effect_state WHERE user_id=$1), 0)`, userID).Scan(&projection.Revision); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT id, definition_id, ruleset_version, domain, effect_kind, target_kind,
			       target_actor_id, lifetime_kind, valid_from, valid_until,
			       initial_charges, remaining_charges, definition_snapshot_jsonb, source_item_id
			FROM player_operation_effect_grants
			WHERE user_id=$1
			  AND valid_from <= $2
			  AND ((lifetime_kind='duration' AND valid_until > $2) OR (lifetime_kind='charges' AND remaining_charges > 0))
			ORDER BY CASE WHEN lifetime_kind='duration' THEN 0 ELSE 1 END, valid_until ASC NULLS LAST, id ASC
		`, userID, now)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var view OperationEffectView
			var targetActorID *int64
			var definitionRaw []byte
			if err := rows.Scan(
				&view.GrantID, &view.DefinitionID, &view.RulesetVersion, &view.Domain, &view.Kind,
				&view.TargetKind, &targetActorID, &view.LifetimeKind, &view.ValidFrom, &view.ValidUntil,
				&view.InitialCharges, &view.RemainingCharges, &definitionRaw, &view.SourceItemID,
			); err != nil {
				return err
			}
			var def operationeffect.Definition
			if err := json.Unmarshal(definitionRaw, &def); err != nil {
				return err
			}
			view.NameKey, view.DescKey, view.IconID = def.NameKey, def.DescKey, def.IconID
			if s.isRetiredFishFoodEffect(def) {
				continue
			}
			view.Selector = def.Selector
			view.Stacking = def.Stacking
			view.Stacking.Key, _, view.Stacking.Exclusive = s.operationEffectExclusiveFamily(def)
			view.Params = append(json.RawMessage(nil), def.Params...)
			if targetActorID != nil {
				view.TargetActorID = *targetActorID
			}
			projection.Items = append(projection.Items, view)
		}
		return rows.Err()
	})
	return projection, err
}

func (s *Store) loadOperationEffectSessionTx(ctx context.Context, tx pgx.Tx, userID int64, through time.Time) (*operationeffect.Session, error) {
	if through.IsZero() {
		through = time.Now()
	}
	earliestStart := through
	var earliest *time.Time
	var pendingOwnerExperience bool
	var hasKittyLane bool
	query := `
		SELECT MIN(COALESCE(current_run_started_at, current_run_ends_at))
		FROM player_action_queue
		WHERE user_id=$1 AND status='running'
	`
	dest := []any{&earliest}
	args := []any{userID}
	if len(s.gameData.CatteryFurniture) > 0 {
		query = `SELECT MIN(COALESCE(current_run_started_at,current_run_ends_at)),
		 COALESCE(bool_or(actor_type='kitty' AND (
		 jsonb_path_exists(payload::jsonb,'$.passiveEffects.sources[*].rules[*] ? (@.kind == "owner_experience")') OR
		 jsonb_path_exists(payload::jsonb,'$.runtime.castPassive.sources[*].rules[*] ? (@.kind == "owner_experience")'))),false),
		 EXISTS(SELECT 1 FROM player_action_queue cat_queue WHERE cat_queue.user_id=$1
		 AND cat_queue.status IN ('running','pending') AND cat_queue.actor_type='kitty'
		 AND NOT EXISTS(SELECT 1 FROM player_kitties k
		 WHERE k.user_id=$1 AND k.id=cat_queue.actor_id AND k.status=$2))
		 FROM player_action_queue WHERE user_id=$1 AND status='running'`
		dest = append(dest, &pendingOwnerExperience, &hasKittyLane)
		args = append(args, KittyStatusInCombat)
	}
	if err := tx.QueryRow(ctx, query, args...).Scan(dest...); err != nil {
		return nil, err
	}
	if earliest != nil && earliest.Before(earliestStart) {
		earliestStart = *earliest
	}
	rows, err := tx.Query(ctx, `
		SELECT id, target_kind, target_actor_id, source_item_id, valid_from, valid_until,
		       initial_charges, remaining_charges, created_at, definition_snapshot_jsonb
		FROM player_operation_effect_grants
		WHERE user_id=$1 AND valid_from <= $2
		  AND ((lifetime_kind='duration' AND valid_until > $3) OR (lifetime_kind='charges' AND remaining_charges > 0))
		ORDER BY priority DESC, valid_until ASC NULLS LAST, id ASC
		FOR UPDATE
	`, userID, through, earliestStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]operationeffect.Grant, 0)
	for rows.Next() {
		var grant operationeffect.Grant
		var targetActorID *int64
		var definitionRaw []byte
		if err := rows.Scan(
			&grant.ID, &grant.Target.Kind, &targetActorID, &grant.SourceItemID,
			&grant.ValidFrom, &grant.ValidUntil, &grant.InitialCharges, &grant.RemainingCharges,
			&grant.CreatedAt, &definitionRaw,
		); err != nil {
			return nil, err
		}
		if targetActorID != nil {
			grant.Target.ActorID = *targetActorID
		}
		if err := json.Unmarshal(definitionRaw, &grant.Definition); err != nil {
			return nil, err
		}
		if !s.isRetiredFishFoodEffect(grant.Definition) {
			grants = append(grants, grant)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var passive *operationeffect.PassiveSession
	if len(s.gameData.CatteryFurniture) > 0 && (hasKittyLane || pendingOwnerExperience) {
		passive, err = s.loadPassiveSessionTx(ctx, tx, userID, earliestStart, through, pendingOwnerExperience)
		if err != nil {
			return nil, err
		}
	}
	if passive != nil {
		passive.PendingOwnerExperience = pendingOwnerExperience
		passive.ProgressOrdering = hasKittyLane && passive.AffectsOwnerProgress()
	}
	if len(grants) == 0 && passive == nil {
		return nil, nil
	}
	session, err := operationeffect.NewSession(s.operationEffectRegistry, grants)
	if err != nil {
		return nil, err
	}
	session.SetPassive(passive)
	return session, nil
}

func (s *Store) flushOperationEffectSessionTx(ctx context.Context, tx pgx.Tx, userID int64, session *operationeffect.Session, now time.Time) (bool, error) {
	if p := session.Passive(); p != nil && p.Dirty {
		if err := s.flushFurnitureAccountingTx(ctx, tx, userID, p.Contributions, p.Remainders); err != nil {
			return false, err
		}
		// Sessions belong to this transaction and are discarded on rollback.
		// Drain increments so a second flush cannot attribute them twice.
		p.Contributions = nil
		p.Dirty = false
	}
	receipts := session.ChargeReceipts()
	if len(receipts) == 0 {
		return false, nil
	}
	type receiptRow struct {
		GrantID      int64     `json:"grant_id"`
		OperationKey string    `json:"operation_key"`
		StartedAt    time.Time `json:"started_at"`
	}
	rows := make([]receiptRow, 0, len(receipts))
	for _, receipt := range receipts {
		rows = append(rows, receiptRow{GrantID: receipt.GrantID, OperationKey: receipt.OperationKey, StartedAt: receipt.StartedAt})
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return false, err
	}
	var insertedCount, deltaCount, updatedCount int64
	err = tx.QueryRow(ctx, `
		WITH input AS (
			SELECT grant_id, operation_key, started_at
			FROM jsonb_to_recordset($2::jsonb)
			  AS x(grant_id bigint, operation_key text, started_at timestamptz)
		), inserted AS (
			INSERT INTO player_operation_effect_charge_receipts(grant_id, user_id, operation_key, started_at, consumed_at)
			SELECT grant_id, $1, operation_key, started_at, $3 FROM input
			ON CONFLICT DO NOTHING
			RETURNING grant_id
		), deltas AS (
			SELECT grant_id, COUNT(*)::bigint AS uses FROM inserted GROUP BY grant_id
		), updated AS (
			UPDATE player_operation_effect_grants g
			SET remaining_charges = g.remaining_charges - d.uses, updated_at=$3
			FROM deltas d
			WHERE g.id=d.grant_id AND g.user_id=$1 AND g.lifetime_kind='charges' AND g.remaining_charges >= d.uses
			RETURNING g.id
		)
		SELECT (SELECT COUNT(*) FROM inserted), (SELECT COUNT(*) FROM deltas), (SELECT COUNT(*) FROM updated)
	`, userID, string(payload), now).Scan(&insertedCount, &deltaCount, &updatedCount)
	if err != nil {
		return false, err
	}
	if updatedCount != deltaCount {
		return false, errors.New("operation effect charge persistence mismatch")
	}
	if insertedCount == 0 {
		return false, nil
	}
	if err := s.bumpOperationEffectRevisionTx(ctx, tx, userID, now); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) bumpOperationEffectRevisionTx(ctx context.Context, tx pgx.Tx, userID int64, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO player_operation_effect_state(user_id, revision, updated_at)
		VALUES($1, 1, $2)
		ON CONFLICT(user_id) DO UPDATE
		SET revision=player_operation_effect_state.revision+1, updated_at=EXCLUDED.updated_at
	`, userID, now)
	return err
}
