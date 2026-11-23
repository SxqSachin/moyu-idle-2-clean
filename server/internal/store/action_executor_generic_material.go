package store

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type actionMaterialBindingArgs interface {
	HasMaterialInstanceBindings() bool
}

type genericExecutorArgs struct {
	MaterialInstanceBindings map[string][]int64 `json:"materialInstanceBindings"`
	InheritFromInstanceID    int64              `json:"inheritFromInstanceId,omitempty"`
}

func (a genericExecutorArgs) HasMaterialInstanceBindings() bool {
	return len(a.MaterialInstanceBindings) > 0
}

type genericExecutorRunContext struct {
	UseInstanceBindings      bool                `json:"useInstanceBindings"`
	MaterialInstanceBindings map[string][]int64  `json:"materialInstanceBindings,omitempty"`
	InheritFromInstanceID    int64               `json:"inheritFromInstanceId,omitempty"`
	InheritedEnhanceLevel    int                 `json:"inheritedEnhanceLevel,omitempty"`
	RefundTemplateCosts      map[string]int64    `json:"refundTemplateCosts,omitempty"`
	RefundEnhancedCosts      []genericRefundCost `json:"refundEnhancedCosts,omitempty"`
}

type genericRefundCost struct {
	ItemID       string `json:"itemId"`
	EnhanceLevel int    `json:"enhanceLevel"`
	Quantity     int64  `json:"quantity"`
}

func parseGenericExecutorArgs(raw json.RawMessage) (genericExecutorArgs, error) {
	if len(raw) == 0 {
		return genericExecutorArgs{MaterialInstanceBindings: map[string][]int64{}}, nil
	}
	var args genericExecutorArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return genericExecutorArgs{}, ErrActionMaterialBindingInvalid
	}
	normalized := make(map[string][]int64, len(args.MaterialInstanceBindings))
	allBoundInstanceIDs := map[int64]struct{}{}
	for itemID, rawIDs := range args.MaterialInstanceBindings {
		normalizedItemID := strings.TrimSpace(itemID)
		if normalizedItemID == "" {
			return genericExecutorArgs{}, ErrActionMaterialBindingInvalid
		}
		seen := map[int64]struct{}{}
		ids := make([]int64, 0, len(rawIDs))
		for _, id := range rawIDs {
			if id <= 0 {
				return genericExecutorArgs{}, ErrActionMaterialBindingInvalid
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			allBoundInstanceIDs[id] = struct{}{}
		}
		if len(ids) == 0 {
			continue
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		normalized[normalizedItemID] = ids
	}
	if args.InheritFromInstanceID < 0 {
		return genericExecutorArgs{}, ErrActionMaterialBindingInvalid
	}
	if args.InheritFromInstanceID > 0 {
		if len(normalized) == 0 {
			return genericExecutorArgs{}, ErrActionMaterialBindingInvalid
		}
		if _, ok := allBoundInstanceIDs[args.InheritFromInstanceID]; !ok {
			return genericExecutorArgs{}, ErrActionMaterialBindingInvalid
		}
	}
	args.MaterialInstanceBindings = normalized
	return args, nil
}

func parseGenericRunContext(payloadRaw []byte) (genericExecutorRunContext, bool) {
	if len(payloadRaw) == 0 {
		return genericExecutorRunContext{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return genericExecutorRunContext{}, false
	}
	if envelope.ExecutorID != "generic" || len(envelope.RunContext) == 0 {
		return genericExecutorRunContext{}, false
	}
	var runCtx genericExecutorRunContext
	if err := json.Unmarshal(envelope.RunContext, &runCtx); err != nil {
		return genericExecutorRunContext{}, false
	}
	return runCtx, true
}

func setGenericRunContext(payloadRaw []byte, runCtx genericExecutorRunContext) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	runCtxRaw, err := json.Marshal(runCtx)
	if err != nil {
		return nil, err
	}
	envelope.RunContext = runCtxRaw
	nextPayload, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return nextPayload, nil
}

type genericBoundConsumePlan struct {
	InstanceID     int64
	ItemID         string
	Quantity       int64
	CountsTemplate bool
}

func (e *genericExecutor) applyActionCostsWithBindingsTx(ctx context.Context, tx pgx.Tx, userID int64, spec ActionSpec, args genericExecutorArgs) (genericExecutorRunContext, error) {
	costs := spec.Costs
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return genericExecutorRunContext{}, err
	}
	runCtx := genericExecutorRunContext{
		UseInstanceBindings:   len(args.MaterialInstanceBindings) > 0,
		InheritFromInstanceID: args.InheritFromInstanceID,
		RefundTemplateCosts:   map[string]int64{},
		RefundEnhancedCosts:   []genericRefundCost{},
	}
	if len(args.MaterialInstanceBindings) == 0 {
		entries, err := buildActionCostEntries(costs, 1)
		if err != nil {
			return genericExecutorRunContext{}, err
		}
		if err := e.store.applyActionCostsTx(ctx, tx, userID, costs); err != nil {
			return genericExecutorRunContext{}, err
		}
		for _, entry := range entries {
			runCtx.RefundTemplateCosts[entry.ItemID] += entry.Quantity
		}
		return runCtx, nil
	}
	entries, err := buildActionCostEntries(costs, 1)
	if err != nil {
		return genericExecutorRunContext{}, err
	}
	plannedConsumes := make([]genericBoundConsumePlan, 0, len(entries))
	fallbackCosts := make([]actionCostEntry, 0, len(entries))
	boundConsumableByItem := map[string]int64{}
	sourceFound := args.InheritFromInstanceID <= 0
	sourceEnhanceLevel := 0
	for _, entry := range entries {
		remain := entry.Quantity
		boundInstanceIDs := append([]int64(nil), args.MaterialInstanceBindings[entry.ItemID]...)
		sort.Slice(boundInstanceIDs, func(i, j int) bool { return boundInstanceIDs[i] < boundInstanceIDs[j] })
		for _, instanceID := range boundInstanceIDs {
			if remain <= 0 {
				break
			}
			locked, err := e.store.LockInstanceTx(ctx, tx, instanceID)
			if err != nil {
				return genericExecutorRunContext{}, ErrActionMaterialBindingInvalid
			}
			if locked.UserID != userID || strings.TrimSpace(locked.State) != "inventory" || strings.TrimSpace(locked.TemplateItem) != entry.ItemID {
				return genericExecutorRunContext{}, ErrActionMaterialBindingInvalid
			}
			if requirement, required := spec.MaterialInstanceRequirements[entry.ItemID]; required && !e.store.instanceSatisfiesActionMaterialRequirement(locked, requirement) {
				return genericExecutorRunContext{}, ErrActionMaterialBindingInvalid
			}
			if !e.store.instanceAffixesMatchTemplateDefault(locked.TemplateItem, locked.AffixesRaw) {
				return genericExecutorRunContext{}, ErrActionMaterialAffixForbidden
			}
			if instanceID == args.InheritFromInstanceID {
				sourceFound = true
				sourceEnhanceLevel = maxInt(sourceEnhanceLevel, locked.EnhanceLevel)
			}
			consumeQty := remain
			if locked.Quantity < consumeQty {
				consumeQty = locked.Quantity
			}
			if consumeQty <= 0 {
				continue
			}
			countsTemplate := e.store.lockedInstanceIsTemplateCommodity(locked)
			plannedConsumes = append(plannedConsumes, genericBoundConsumePlan{
				InstanceID:     instanceID,
				ItemID:         entry.ItemID,
				Quantity:       consumeQty,
				CountsTemplate: countsTemplate,
			})
			if countsTemplate {
				boundConsumableByItem[entry.ItemID] += consumeQty
			}
			if locked.EnhanceLevel > 0 {
				runCtx.RefundEnhancedCosts = append(runCtx.RefundEnhancedCosts, genericRefundCost{
					ItemID:       entry.ItemID,
					EnhanceLevel: locked.EnhanceLevel,
					Quantity:     consumeQty,
				})
			} else {
				runCtx.RefundTemplateCosts[entry.ItemID] += consumeQty
			}
			remain -= consumeQty
		}
		if remain > 0 {
			fallbackCosts = append(fallbackCosts, actionCostEntry{ItemID: entry.ItemID, Quantity: remain})
			runCtx.RefundTemplateCosts[entry.ItemID] += remain
		}
	}
	if !sourceFound {
		return genericExecutorRunContext{}, ErrActionMaterialBindingInvalid
	}
	if len(fallbackCosts) > 0 {
		ownedByItem, err := e.store.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(fallbackCosts))
		if err != nil {
			return genericExecutorRunContext{}, err
		}
		if !fallbackCostsAffordable(fallbackCosts, ownedByItem, boundConsumableByItem) {
			return genericExecutorRunContext{}, ErrInsufficientItem
		}
	}
	for _, planned := range plannedConsumes {
		if err := e.store.LedgerConsumeInstanceTx(ctx, tx, userID, planned.InstanceID, planned.ItemID, planned.Quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "action_queue",
			Details: map[string]any{
				"type": "action_cost_instance",
			},
		}); err != nil {
			return genericExecutorRunContext{}, err
		}
	}
	for _, fallback := range fallbackCosts {
		if err := e.store.LedgerConsumeTemplateTx(ctx, tx, userID, fallback.ItemID, fallback.Quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "action_queue",
			Details: map[string]any{
				"type": "action_cost_fallback",
			},
		}); err != nil {
			return genericExecutorRunContext{}, err
		}
	}
	if sourceEnhanceLevel > 0 {
		ratio := e.store.config.ActionEnhanceInheritanceRatio
		if ratio < 0 || ratio > 1 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			ratio = 0.5
		}
		inheritedLevel := int(math.Floor(float64(sourceEnhanceLevel) * ratio))
		if inheritedLevel > 0 {
			runCtx.InheritedEnhanceLevel = inheritedLevel
		}
	}
	return runCtx, nil
}

func (s *Store) instanceSatisfiesActionMaterialRequirement(row LockedItemInstance, requirement ActionMaterialInstanceRequirement) bool {
	if row.EnhanceLevel != int(requirement.EnhanceLevel) {
		return false
	}
	if requirement.RequireTemplateAffixes && !s.instanceAffixesMatchTemplateDefault(row.TemplateItem, row.AffixesRaw) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(requirement.Durability), "full") && (row.Durability <= 0 || row.Durability != row.MaxDurability) {
		return false
	}
	// These recipes consume the authored item, never a renamed or otherwise customized derivative.
	return itemCustomizationEmpty(row.CustomData)
}

func baseSnapshotJSONEmptyRaw(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return false
	}
	switch typed := node.(type) {
	case nil:
		return true
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func fallbackCostsAffordable(fallbackCosts []actionCostEntry, ownedByItem map[string]int64, boundConsumableByItem map[string]int64) bool {
	for _, fallback := range fallbackCosts {
		owned := ownedByItem[fallback.ItemID] - boundConsumableByItem[fallback.ItemID]
		if owned < fallback.Quantity {
			return false
		}
	}
	return true
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
