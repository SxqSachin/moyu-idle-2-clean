package alchemy

import (
	"encoding/json"
	"errors"
	"math"
	"mysecondwebgame/server/internal/operationeffect"
)

const KindConversionLossReduction = "alchemy.conversion_loss_reduction"

type conversionHandler struct{}
type ConversionParams struct {
	LossReductionRate float64 `json:"lossReductionRate"`
}

func (conversionHandler) Domain() operationeffect.Domain { return operationeffect.DomainAlchemy }
func (conversionHandler) StableAcrossStarts() bool       { return true }
func (conversionHandler) ValidateParams(raw json.RawMessage) error {
	var p ConversionParams
	if err := decodeStrict(raw, &p); err != nil {
		return err
	}
	if math.IsNaN(p.LossReductionRate) || math.IsInf(p.LossReductionRate, 0) || p.LossReductionRate <= 0 || p.LossReductionRate > .10 {
		return errors.New("invalid conversion loss reduction")
	}
	return nil
}
func (conversionHandler) Applies(op operationeffect.Operation, _ json.RawMessage) bool {
	return op.Domain == operationeffect.DomainAlchemy && op.ActionID == "alchemy.convert"
}
func (conversionHandler) ValidateDefinitionConstraints(d operationeffect.Definition) error {
	if len(d.Selector.ActionIDs) != 1 || d.Selector.ActionIDs[0] != "alchemy.convert" || !d.Stacking.Exclusive || d.Stacking.Key != "food.conversion" {
		return errors.New("conversion food requires exclusive conversion scope")
	}
	return nil
}

// Relative reduction preserves loss risk even at the ordinary stability floor.
func ConversionFailureChance(base float64, snapshot operationeffect.Snapshot) (float64, error) {
	rate := 0.0
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainAlchemy || rule.Kind != KindConversionLossReduction {
			continue
		}
		if err := (conversionHandler{}).ValidateParams(rule.Params); err != nil {
			return 0, err
		}
		var p ConversionParams
		_ = json.Unmarshal(rule.Params, &p)
		rate = math.Max(rate, p.LossReductionRate)
	}
	if rate == 0 {
		return base, nil
	}
	return math.Max(0, math.Min(1, base)) * (1 - rate), nil
}
