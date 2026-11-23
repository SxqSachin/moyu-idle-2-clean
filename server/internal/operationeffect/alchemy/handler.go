package alchemy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"mysecondwebgame/server/internal/operationeffect"
)

const (
	KindInfusionSuccessBonus              = "alchemy.infusion_success_bonus"
	KindInfusionCapacityFloorTradeoff     = "alchemy.infusion_capacity_floor_tradeoff"
	KindInfusionBulkCapacityFloorTradeoff = "alchemy.infusion_bulk_capacity_floor_tradeoff"
	KindInfusionPrecisionCeilingBonus     = "alchemy.infusion_precision_ceiling_bonus"
	KindInfusionPreciousCapacityBonus     = "alchemy.infusion_precious_capacity_bonus"

	CapabilityInfusion         = "infusion"
	CapabilityBulkMaterial     = "infusion_bulk_material"
	CapabilityPreciousMaterial = "infusion_precious_material"
	CapabilityBasicMaterial    = "infusion_basic_material"
	CapabilityRefinedMaterial  = "infusion_refined_material"

	MaxExtraMaterialQuantity      int64 = 1_000_000
	MaxPositiveFloorReductionRate       = 0.95
	MaxPositiveCeilingBonusRate         = 0.25
	MaxQuantityLimitBonusRate           = 0.17
	MaxSuccessRatePenalty               = 0.20
	MaxSuccessRateBonus                 = 0.10
)

type SuccessBonusParams struct {
	BonusRate float64 `json:"bonusRate"`
}

type CapacityFloorTradeoffParams struct {
	// Zero retains the historical common-through-epic scope in frozen items.
	MaterialTier               int     `json:"materialTier,omitempty"`
	ExtraMaterialQuantity      int64   `json:"extraMaterialQuantity"`
	PositiveFloorReductionRate float64 `json:"positiveFloorReductionRate"`
	QuantityLimitBonusRate     float64 `json:"quantityLimitBonusRate,omitempty"`
	SuccessRatePenalty         float64 `json:"successRatePenalty,omitempty"`
}

type PrecisionCeilingBonusParams struct {
	PositiveCeilingBonusRate float64 `json:"positiveCeilingBonusRate"`
}

type PreciousCapacityParams struct {
	ExtraMaterialQuantity  int64   `json:"extraMaterialQuantity"`
	QuantityLimitBonusRate float64 `json:"quantityLimitBonusRate,omitempty"`
	SuccessRatePenalty     float64 `json:"successRatePenalty,omitempty"`
}

type Modifiers struct {
	TierCapacities                 [2]CapacityBonus
	ExtraMaterialQuantity          int64
	BulkMaterialQuantity           int64
	PreciousMaterialQuantity       int64
	BulkQuantityLimitBonusRate     float64
	PreciousQuantityLimitBonusRate float64
	PositiveFloorReductionRate     float64
	PositiveCeilingBonusRate       float64
	SuccessRatePenalty             float64
	SuccessRateBonus               float64
}

type CapacityBonus struct {
	Quantity int64
	Rate     float64
}

func MaterialTier(rarity string) int {
	switch rarity {
	case "common", "uncommon":
		return 1
	case "rare", "epic":
		return 2
	}
	return 0
}

func TierRateLimit(tier int) float64 {
	switch tier {
	case 1:
		return .20
	case 2:
		return .15
	}
	return MaxQuantityLimitBonusRate
}

type handler struct {
	kind string
}

func Register(registry *operationeffect.Registry) error {
	if registry == nil {
		return errors.New("operation effect registry is required")
	}
	for _, kind := range []string{KindInfusionCapacityFloorTradeoff, KindInfusionBulkCapacityFloorTradeoff, KindInfusionPrecisionCeilingBonus, KindInfusionPreciousCapacityBonus, KindInfusionSuccessBonus} {
		if err := registry.Register(kind, handler{kind: kind}); err != nil {
			return err
		}
	}
	return registry.Register(KindConversionLossReduction, conversionHandler{})
}

func (h handler) Domain() operationeffect.Domain {
	return operationeffect.DomainAlchemy
}

func (h handler) ValidateDefinitionConstraints(def operationeffect.Definition) error {
	for _, actionID := range def.Selector.ActionIDs {
		if actionID != "alchemy.bench" {
			return errors.New("infusion effect action selector must contain only alchemy.bench")
		}
	}
	return nil
}

func (h handler) ValidateParams(raw json.RawMessage) error {
	switch h.kind {
	case KindInfusionSuccessBonus:
		var params SuccessBonusParams
		if err := decodeStrict(raw, &params); err != nil {
			return err
		}
		return validateRate("bonusRate", params.BonusRate, MaxSuccessRateBonus)
	case KindInfusionPreciousCapacityBonus:
		var params PreciousCapacityParams
		if err := decodeStrict(raw, &params); err != nil {
			return err
		}
		if params.ExtraMaterialQuantity <= 0 || params.ExtraMaterialQuantity > MaxExtraMaterialQuantity {
			return fmt.Errorf("extraMaterialQuantity must be in [1, %d]", MaxExtraMaterialQuantity)
		}
		if err := validateQuantityLimitRate(params.QuantityLimitBonusRate); err != nil {
			return err
		}
		if err := validateSuccessRatePenalty(params.SuccessRatePenalty); err != nil {
			return err
		}
	case KindInfusionCapacityFloorTradeoff, KindInfusionBulkCapacityFloorTradeoff:
		var params CapacityFloorTradeoffParams
		if err := decodeStrict(raw, &params); err != nil {
			return err
		}
		if params.ExtraMaterialQuantity <= 0 || params.ExtraMaterialQuantity > MaxExtraMaterialQuantity {
			return fmt.Errorf("extraMaterialQuantity must be in [1, %d]", MaxExtraMaterialQuantity)
		}
		if err := validateRate("positiveFloorReductionRate", params.PositiveFloorReductionRate, MaxPositiveFloorReductionRate); err != nil {
			return err
		}
		if params.MaterialTier < 0 || params.MaterialTier > 2 || (h.kind == KindInfusionCapacityFloorTradeoff && params.MaterialTier != 0) {
			return errors.New("materialTier must be 0 (legacy), 1 (basic), or 2 (refined) for scoped capacity")
		}
		if rate := params.QuantityLimitBonusRate; math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > TierRateLimit(params.MaterialTier) {
			return errors.New("quantityLimitBonusRate exceeds its material tier")
		}
		if err := validateSuccessRatePenalty(params.SuccessRatePenalty); err != nil {
			return err
		}
		if h.kind == KindInfusionCapacityFloorTradeoff && (params.QuantityLimitBonusRate != 0 || params.SuccessRatePenalty != 0) {
			return errors.New("legacy capacity does not support quantityLimitBonusRate or successRatePenalty")
		}
	case KindInfusionPrecisionCeilingBonus:
		var params PrecisionCeilingBonusParams
		if err := decodeStrict(raw, &params); err != nil {
			return err
		}
		if err := validateRate("positiveCeilingBonusRate", params.PositiveCeilingBonusRate, MaxPositiveCeilingBonusRate); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported alchemy operation effect kind: %s", h.kind)
	}
	return nil
}

func (h handler) Applies(operation operationeffect.Operation, raw json.RawMessage) bool {
	if h.kind == KindInfusionBulkCapacityFloorTradeoff {
		var params CapacityFloorTradeoffParams
		if decodeStrict(raw, &params) != nil {
			return false
		}
		if params.MaterialTier == 1 && !operation.Capabilities[CapabilityBasicMaterial] {
			return false
		}
		if params.MaterialTier == 2 && !operation.Capabilities[CapabilityRefinedMaterial] {
			return false
		}
	}
	return operation.Domain == operationeffect.DomainAlchemy &&
		(h.kind != KindInfusionBulkCapacityFloorTradeoff || operation.Capabilities[CapabilityBulkMaterial]) &&
		(h.kind != KindInfusionPreciousCapacityBonus || operation.Capabilities[CapabilityPreciousMaterial]) &&
		(operation.ActorKind == operationeffect.ActorPlayer || operation.ActorKind == operationeffect.ActorKitty) &&
		operation.ActionID == "alchemy.bench" &&
		operation.Capabilities[CapabilityInfusion]
}

// Resolve folds the frozen rules for one infusion. Tradeoffs are paid once per
// matching rule; QuantityBonus resolves overlapping historical capacity scopes.
func Resolve(snapshot operationeffect.Snapshot) (Modifiers, error) {
	var modifiers Modifiers
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainAlchemy {
			continue
		}
		switch rule.Kind {
		case KindInfusionSuccessBonus:
			if err := (handler{kind: rule.Kind}).ValidateParams(rule.Params); err != nil {
				return Modifiers{}, err
			}
			var params SuccessBonusParams
			if err := decodeStrict(rule.Params, &params); err != nil {
				return Modifiers{}, err
			}
			modifiers.SuccessRateBonus = math.Min(MaxSuccessRateBonus, modifiers.SuccessRateBonus+params.BonusRate)
		case KindInfusionPreciousCapacityBonus:
			if err := (handler{kind: rule.Kind}).ValidateParams(rule.Params); err != nil {
				return Modifiers{}, err
			}
			var params PreciousCapacityParams
			if err := decodeStrict(rule.Params, &params); err != nil {
				return Modifiers{}, err
			}
			modifiers.PreciousQuantityLimitBonusRate = math.Min(MaxQuantityLimitBonusRate, modifiers.PreciousQuantityLimitBonusRate+params.QuantityLimitBonusRate)
			modifiers.SuccessRatePenalty = math.Min(MaxSuccessRatePenalty, modifiers.SuccessRatePenalty+params.SuccessRatePenalty)
			if params.ExtraMaterialQuantity > MaxExtraMaterialQuantity-modifiers.PreciousMaterialQuantity {
				modifiers.PreciousMaterialQuantity = MaxExtraMaterialQuantity
			} else {
				modifiers.PreciousMaterialQuantity += params.ExtraMaterialQuantity
			}
		case KindInfusionCapacityFloorTradeoff, KindInfusionBulkCapacityFloorTradeoff:
			if err := (handler{kind: rule.Kind}).ValidateParams(rule.Params); err != nil {
				return Modifiers{}, err
			}
			var params CapacityFloorTradeoffParams
			if err := decodeStrict(rule.Params, &params); err != nil {
				return Modifiers{}, err
			}
			quantity := &modifiers.ExtraMaterialQuantity
			modifiers.SuccessRatePenalty = math.Min(MaxSuccessRatePenalty, modifiers.SuccessRatePenalty+params.SuccessRatePenalty)
			if params.MaterialTier > 0 {
				tier := &modifiers.TierCapacities[params.MaterialTier-1]
				quantity = &tier.Quantity
				tier.Rate = math.Min(TierRateLimit(params.MaterialTier), tier.Rate+params.QuantityLimitBonusRate)
			} else if rule.Kind == KindInfusionBulkCapacityFloorTradeoff {
				quantity = &modifiers.BulkMaterialQuantity
				modifiers.BulkQuantityLimitBonusRate = math.Min(MaxQuantityLimitBonusRate, modifiers.BulkQuantityLimitBonusRate+params.QuantityLimitBonusRate)
			}
			if params.ExtraMaterialQuantity > MaxExtraMaterialQuantity-*quantity {
				*quantity = MaxExtraMaterialQuantity
			} else {
				*quantity += params.ExtraMaterialQuantity
			}
			modifiers.PositiveFloorReductionRate = math.Min(
				MaxPositiveFloorReductionRate,
				modifiers.PositiveFloorReductionRate+params.PositiveFloorReductionRate,
			)
		case KindInfusionPrecisionCeilingBonus:
			if err := (handler{kind: rule.Kind}).ValidateParams(rule.Params); err != nil {
				return Modifiers{}, err
			}
			var params PrecisionCeilingBonusParams
			if err := decodeStrict(rule.Params, &params); err != nil {
				return Modifiers{}, err
			}
			modifiers.PositiveCeilingBonusRate = math.Min(
				MaxPositiveCeilingBonusRate,
				modifiers.PositiveCeilingBonusRate+params.PositiveCeilingBonusRate,
			)
		}
	}
	return modifiers, nil
}

func BulkMaterialEligible(rarity string) bool {
	switch rarity {
	case "common", "uncommon", "rare", "epic":
		return true
	}
	return false
}

func PreciousMaterialEligible(rarity string) bool {
	return rarity == "legendary" || rarity == "mythic"
}

// Legacy stamped crafts retain their unrestricted quantity. Rarity-scoped effects
// respect both material rarity and a relative cap for unusually small limits.
func (m Modifiers) QuantityBonus(rarity string, baseLimit int64) int64 {
	bonus := m.ExtraMaterialQuantity
	quantity := int64(0)
	rate := float64(0)
	if BulkMaterialEligible(rarity) {
		quantity = m.BulkMaterialQuantity
		rate = m.BulkQuantityLimitBonusRate
	}
	if PreciousMaterialEligible(rarity) {
		quantity = m.PreciousMaterialQuantity
		rate = m.PreciousQuantityLimitBonusRate
	}
	if baseLimit > 0 && rate > 0 {
		proportional := int64(math.Floor(math.Min(float64(MaxExtraMaterialQuantity), float64(baseLimit)*math.Min(rate, MaxQuantityLimitBonusRate)) + 1e-9))
		if proportional > quantity {
			quantity = proportional
		}
	}
	// Current tiers never overlap. If an older broad-scope grant coexists, take
	// the better complete bonus rather than compounding two capacity foods.
	if tier := MaterialTier(rarity); tier > 0 && baseLimit > 0 {
		capacity := m.TierCapacities[tier-1]
		proportional := int64(math.Floor(math.Min(float64(MaxExtraMaterialQuantity), float64(baseLimit)*math.Min(capacity.Rate, TierRateLimit(tier))) + 1e-9))
		quantity = max(quantity, capacity.Quantity, proportional)
	}
	if quantity > 0 && baseLimit > 0 {
		limit := MaxExtraMaterialQuantity
		if baseLimit < limit/2 {
			limit = baseLimit * 2
		}
		extra := quantity
		if extra > limit {
			extra = limit
		}
		if extra > MaxExtraMaterialQuantity-bonus {
			return MaxExtraMaterialQuantity
		}
		bonus += extra
	}
	return bonus
}

func validateRate(name string, value, maximum float64) error {
	if value <= 0 || value > maximum || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be in (0, %.2f]", name, maximum)
	}
	return nil
}

// Apply the frozen food cost after stability and medium adjustments, while
// preserving the medium's guaranteed minimum success chance.
func (m Modifiers) ApplyFailureChance(failChance, successFloor float64) float64 {
	if m.SuccessRatePenalty <= 0 && m.SuccessRateBonus <= 0 {
		return failChance
	}
	maximum := 1 - math.Min(1, math.Max(0, successFloor))
	penalty := math.Min(MaxSuccessRatePenalty, math.Max(0, m.SuccessRatePenalty))
	bonus := math.Min(MaxSuccessRateBonus, math.Max(0, m.SuccessRateBonus))
	// Combine offsets before clamping so a bonus can offset a capacity tradeoff.
	return math.Max(0, math.Min(maximum, failChance+penalty-bonus))
}

func validateSuccessRatePenalty(rate float64) error {
	if rate < 0 || rate > MaxSuccessRatePenalty || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return errors.New("successRatePenalty must be in [0, 0.20]")
	}
	return nil
}

func validateQuantityLimitRate(rate float64) error {
	if rate < 0 || rate > MaxQuantityLimitBonusRate || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return errors.New("quantityLimitBonusRate must be in [0, 0.17]")
	}
	return nil
}

func decodeStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("params contain trailing data")
	}
	return nil
}
