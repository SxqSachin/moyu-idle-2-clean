package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"

	"github.com/jackc/pgx/v5"
)

const (
	alchemyOperationEffectStampVersion  = "alchemy-operation-effects-v1"
	alchemyOperationEffectStampMaxRules = 32
)

type alchemyOperationEffectStamp struct {
	Version string                            `json:"version"`
	Rules   []alchemyOperationEffectStampRule `json:"rules"`
}

type alchemyOperationEffectStampRule struct {
	DefinitionID   string          `json:"definitionId"`
	RulesetVersion string          `json:"rulesetVersion"`
	Kind           string          `json:"kind"`
	Params         json.RawMessage `json:"params"`
}

func (e *alchemyExecutor) OperationEffectDomain() operationeffect.Domain {
	if e != nil && (e.actionID == "alchemy.bench" || e.actionID == "alchemy.transmute" || e.actionID == "alchemy.convert") {
		return operationeffect.DomainAlchemy
	}
	return operationeffect.DomainNone
}

func (e *alchemyExecutor) OperationEffectsRequirePerStartSettlement() bool {
	return e != nil && (e.actionID == "alchemy.bench" || e.actionID == "alchemy.transmute" || e.actionID == "alchemy.convert")
}

func (e *alchemyExecutor) EnrichOperationEffectContext(op operationeffect.Operation, payload []byte) operationeffect.Operation {
	if e.actionID == "alchemy.transmute" {
		return e.enrichTransmuteOperation(op, payload)
	}
	if !op.Capabilities[alchemyeffect.CapabilityInfusion] {
		return op
	}
	var args alchemyBenchArgs
	if parseAlchemyRunContext(payload, &args) != nil {
		return op
	}
	for itemID, quantity := range args.Materials {
		item := e.store.gameData.ItemByID(itemID)
		if quantity > 0 && item != nil && alchemyeffect.BulkMaterialEligible(item.Rarity) {
			op.Capabilities[alchemyeffect.CapabilityBulkMaterial] = true
			switch alchemyeffect.MaterialTier(item.Rarity) {
			case 1:
				op.Capabilities[alchemyeffect.CapabilityBasicMaterial] = true
			case 2:
				op.Capabilities[alchemyeffect.CapabilityRefinedMaterial] = true
			}
		}
		if quantity > 0 && item != nil && alchemyeffect.PreciousMaterialEligible(item.Rarity) {
			op.Capabilities[alchemyeffect.CapabilityPreciousMaterial] = true
		}
	}
	return op
}

func alchemyOperationEffectSnapshot(item QueueRuntimeItem) operationeffect.Snapshot {
	if item.OperationEffects != nil {
		return *item.OperationEffects
	}
	if snapshot, ok := queueOperationEffectSnapshot(item.PayloadRaw); ok {
		return snapshot
	}
	return operationeffect.Snapshot{}
}

func alchemyOperationEffectModifiers(item QueueRuntimeItem) (alchemyeffect.Modifiers, error) {
	return alchemyeffect.Resolve(alchemyOperationEffectSnapshot(item))
}

func alchemyInfusionOperationEffectKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case alchemyeffect.KindInfusionCapacityFloorTradeoff, alchemyeffect.KindInfusionBulkCapacityFloorTradeoff, alchemyeffect.KindInfusionPrecisionCeilingBonus, alchemyeffect.KindInfusionPreciousCapacityBonus, alchemyeffect.KindInfusionSuccessBonus:
		return true
	default:
		return false
	}
}

func stampAlchemyCraftOperationEffects(craft *resolvedAlchemyCraft, snapshot operationeffect.Snapshot) error {
	if craft == nil {
		return nil
	}
	stamp := &alchemyOperationEffectStamp{
		Version: alchemyOperationEffectStampVersion,
		Rules:   make([]alchemyOperationEffectStampRule, 0, len(snapshot.Rules)),
	}
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainAlchemy || !alchemyInfusionOperationEffectKind(rule.Kind) {
			continue
		}
		stamp.Rules = append(stamp.Rules, alchemyOperationEffectStampRule{
			DefinitionID:   strings.TrimSpace(rule.DefinitionID),
			RulesetVersion: strings.TrimSpace(rule.RulesetVersion),
			Kind:           strings.TrimSpace(rule.Kind),
			Params:         append(json.RawMessage(nil), rule.Params...),
		})
	}
	if len(stamp.Rules) == 0 {
		return nil
	}
	if _, err := resolveAlchemyOperationEffectStamp(stamp); err != nil {
		return err
	}
	for index := range craft.Components {
		componentStamp := *stamp
		componentStamp.Rules = append([]alchemyOperationEffectStampRule(nil), stamp.Rules...)
		craft.Components[index].OperationEffects = &componentStamp
	}
	if craft.EffectParams == nil {
		craft.EffectParams = map[string]any{}
	}
	setAlchemyComponentParams(craft.EffectParams, craft.Components)
	return nil
}

func resolveAlchemyOperationEffectStamp(stamp *alchemyOperationEffectStamp) (alchemyeffect.Modifiers, error) {
	if stamp == nil || strings.TrimSpace(stamp.Version) != alchemyOperationEffectStampVersion {
		return alchemyeffect.Modifiers{}, errors.New("alchemy operation effect stamp version is invalid")
	}
	if len(stamp.Rules) == 0 || len(stamp.Rules) > alchemyOperationEffectStampMaxRules {
		return alchemyeffect.Modifiers{}, errors.New("alchemy operation effect stamp rule count is invalid")
	}
	snapshot := operationeffect.Snapshot{
		SchemaVersion: operationeffect.SnapshotSchemaVersion,
		Rules:         make([]operationeffect.RuleSnapshot, 0, len(stamp.Rules)),
	}
	for _, rule := range stamp.Rules {
		if strings.TrimSpace(rule.DefinitionID) == "" || strings.TrimSpace(rule.RulesetVersion) == "" || !alchemyInfusionOperationEffectKind(rule.Kind) || len(rule.Params) == 0 {
			return alchemyeffect.Modifiers{}, errors.New("alchemy operation effect stamp rule is invalid")
		}
		snapshot.Rules = append(snapshot.Rules, operationeffect.RuleSnapshot{
			DefinitionID:   strings.TrimSpace(rule.DefinitionID),
			RulesetVersion: strings.TrimSpace(rule.RulesetVersion),
			Domain:         operationeffect.DomainAlchemy,
			Kind:           strings.TrimSpace(rule.Kind),
			Params:         append(json.RawMessage(nil), rule.Params...),
		})
	}
	modifiers, err := alchemyeffect.Resolve(snapshot)
	if err != nil {
		return alchemyeffect.Modifiers{}, err
	}
	if modifiers.ExtraMaterialQuantity <= 0 && modifiers.BulkMaterialQuantity <= 0 && modifiers.PreciousMaterialQuantity <= 0 && modifiers.PositiveFloorReductionRate <= 0 && modifiers.PositiveCeilingBonusRate <= 0 && modifiers.SuccessRateBonus <= 0 {
		return alchemyeffect.Modifiers{}, errors.New("alchemy operation effect stamp has no supported modifier")
	}
	return modifiers, nil
}

func decodeAlchemyOperationEffectStamp(value any) (*alchemyOperationEffectStamp, error) {
	if value == nil {
		return nil, errors.New("alchemy operation effect stamp is missing")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var stamp alchemyOperationEffectStamp
	if err := decoder.Decode(&stamp); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("alchemy operation effect stamp contains trailing data")
	}
	return &stamp, nil
}

func alchemyOperationEffectModifiersFromComponent(component alchemyInheritComponent) (alchemyeffect.Modifiers, bool) {
	if component.OperationEffects == nil {
		return alchemyeffect.Modifiers{}, false
	}
	modifiers, err := resolveAlchemyOperationEffectStamp(component.OperationEffects)
	return modifiers, err == nil
}

func (e *alchemyExecutor) operationEffectsNeedPerStart(item QueueRuntimeItem, timing queueRunTiming, settledAt time.Time) bool {
	if e == nil || e.OperationEffectDomain() == operationeffect.DomainNone {
		return false
	}
	if snapshot, ok := queueOperationEffectSnapshot(item.PayloadRaw); ok && len(snapshot.Rules) > 0 {
		return true
	}
	if item.OperationEffects != nil && len(item.OperationEffects.Rules) > 0 {
		return true
	}
	if item.EffectSession == nil {
		return false
	}
	startAt := operationEffectSubsequentStartAt(item, timing, settledAt, 1)
	probe := queueOperationEffectOperation(operationeffect.DomainAlchemy, item, startAt, item.DoneCount+2, item.PayloadRaw)
	probe = e.EnrichOperationEffectContext(probe, item.PayloadRaw)
	return item.EffectSession.HasMatching(probe, settledAt)
}

func (e *alchemyExecutor) validateAlchemyBenchOperationEffectsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	snapshot := alchemyOperationEffectSnapshot(item)
	if len(snapshot.Rules) == 0 {
		return nil
	}
	modifiers, err := alchemyeffect.Resolve(snapshot)
	if err != nil {
		return err
	}
	var args alchemyBenchArgs
	if err := parseAlchemyRunContext(item.PayloadRaw, &args); err != nil {
		return err
	}
	if args.FishInstanceID > 0 || len(args.Materials) == 0 {
		return nil
	}
	derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return err
	}
	return e.store.validateAlchemyMaterialQuantities(args.Materials, derived, modifiers)
}

func (e *alchemyExecutor) prepareNextAlchemyBenchEffectRunTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	item QueueRuntimeItem,
	payloadRaw json.RawMessage,
	startedAt time.Time,
	doneAfter int,
) (json.RawMessage, bool, string, error) {
	if e == nil || e.actionID != "alchemy.bench" {
		return payloadRaw, false, "", nil
	}
	operation := queueOperationEffectOperation(operationeffect.DomainAlchemy, item, startedAt, doneAfter+1, payloadRaw)
	operation = e.EnrichOperationEffectContext(operation, payloadRaw)
	snapshot := operationeffect.EmptySnapshot(operation)
	var prepared *operationeffect.PreparedOperation
	if item.EffectSession != nil {
		candidate := item.EffectSession.Prepare(operation)
		prepared = &candidate
		snapshot = candidate.Snapshot()
	}
	nextItem := item
	nextItem.DoneCount = doneAfter
	nextItem.PayloadRaw = payloadRaw
	nextItem.RunStartAt = &startedAt
	nextItem.OperationEffects = &snapshot

	var args alchemyBenchArgs
	if err := parseAlchemyRunContext(payloadRaw, &args); err != nil {
		return nil, false, "", err
	}
	if args.FishInstanceID <= 0 {
		modifiers, err := alchemyeffect.Resolve(snapshot)
		if err != nil {
			return nil, false, "", err
		}
		derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, nextItem)
		if err != nil {
			return nil, false, "", err
		}
		if err := e.store.validateAlchemyMaterialQuantities(args.Materials, derived, modifiers); err != nil {
			if reason, stop := classifyAlchemyStopError(err); stop {
				return payloadRaw, true, reason, nil
			}
			return nil, false, "", err
		}
	}

	if err := e.store.validateAlchemyBenchContinuationTargetTx(ctx, tx, userID, args); err != nil {
		if reason, stop := classifyAlchemyStopError(err); stop {
			return payloadRaw, true, reason, nil
		}
		return nil, false, "", err
	}
	var medium *catalog.AlchemyMedium
	if strings.TrimSpace(args.MediumItemID) != "" {
		medium = e.store.gameData.AlchemyMediumByItemID(args.MediumItemID)
		if medium == nil {
			return payloadRaw, true, ActionQueueCancelReasonTargetInvalid, nil
		}
	}
	perRunCosts := e.store.alchemyBenchPerRunCosts(args.Materials, args.ComboRecipeID, medium)
	affordable, err := e.store.maxAffordableActionStartsTx(ctx, tx, userID, perRunCosts, 1)
	if err != nil {
		return nil, false, "", err
	}
	if affordable < 1 {
		return payloadRaw, true, ActionQueueCancelReasonCostInsufficient, nil
	}
	reserved, err := e.reserveAlchemyRunCostsTx(ctx, tx, userID, nextItem)
	if err != nil {
		return nil, false, "", err
	}
	nextPayload, err := setAlchemyReservedCosts(payloadRaw, args, reserved)
	if err != nil {
		return nil, false, "", err
	}
	if prepared != nil {
		if _, err := item.EffectSession.Commit(*prepared); err != nil {
			return nil, false, "", err
		}
	}
	nextPayload, err = setQueueOperationEffectSnapshot(nextPayload, snapshot)
	if err != nil {
		return nil, false, "", err
	}
	return nextPayload, false, "", nil
}

func (s *Store) validateAlchemyBenchContinuationTargetTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyBenchArgs) error {
	carrier, err := s.LockInstanceTx(ctx, tx, args.CarrierInstanceID)
	if err != nil {
		return err
	}
	if carrier.UserID != userID {
		return ErrInstanceNotOwned
	}
	if carrier.State != "inventory" || carrier.Quantity <= 0 || len(decodeAffixSlots(carrier.AffixesRaw)) > 0 {
		return ErrInvalidInstanceState
	}
	rule := s.gameData.AffixCarrierRuleByItemID(carrier.TemplateItem)
	if rule == nil || !rule.Active || rule.MaxCustomAffixes <= 0 {
		return ErrAffixInvalidCarrier
	}
	return nil
}

func (s *Store) validateAlchemyMaterialQuantities(materials map[string]int64, derived alchemyDerived, modifiers alchemyeffect.Modifiers) error {
	for itemID, qty := range materials {
		def := s.gameData.ItemTraitByItemID(itemID)
		if def == nil || len(def.Traits) == 0 {
			return ErrAlchemyArgsInvalid
		}
		for _, trait := range def.Traits {
			basis := alchemyTraitUnitBasis(trait)
			if qty <= 0 || qty%basis != 0 {
				return ErrAlchemyBasisInvalid
			}
			if qty > s.alchemyMaterialMaxQuantity(itemID, trait, derived.Stability, modifiers) {
				return ErrAlchemyMaterialOverflow
			}
		}
	}
	return nil
}

func alchemyTraitMaxQuantityWithModifiers(trait catalog.AlchemyTraitEntry, stability float64, modifiers alchemyeffect.Modifiers) int64 {
	return addAlchemyQuantityBonus(alchemyTraitMaxQuantity(trait, stability), modifiers.ExtraMaterialQuantity)
}

func (s *Store) alchemyMaterialQuantityBonus(itemID string, base int64, modifiers alchemyeffect.Modifiers) int64 {
	rarity := ""
	if item := s.gameData.ItemByID(itemID); item != nil {
		rarity = item.Rarity
	}
	return modifiers.QuantityBonus(rarity, base)
}

func (s *Store) alchemyMaterialMaxQuantity(itemID string, trait catalog.AlchemyTraitEntry, stability float64, modifiers alchemyeffect.Modifiers) int64 {
	base := alchemyTraitMaxQuantity(trait, stability)
	return addAlchemyQuantityBonus(base, s.alchemyMaterialQuantityBonus(itemID, base, modifiers))
}

func applyAlchemyFoodFailureChance(failChance float64, medium *catalog.AlchemyMedium, modifiers alchemyeffect.Modifiers) float64 {
	successFloor := 0.0
	if medium != nil {
		successFloor = medium.Modifiers.SuccessFloor
	}
	return modifiers.ApplyFailureChance(failChance, successFloor)
}

func addAlchemyQuantityBonus(base, bonus int64) int64 {
	if bonus <= 0 {
		return base
	}
	if base > math.MaxInt64-bonus {
		return math.MaxInt64
	}
	return base + bonus
}
