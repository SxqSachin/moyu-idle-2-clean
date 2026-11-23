package enchantment

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"mysecondwebgame/server/internal/operationeffect"
	"strings"
)

const Domain operationeffect.Domain = "enchantment"
const Kind = "enchantment.high_rarity_weight"
const Eligible = "enchantment.mixed_rarities"

type Params struct {
	WeightBonusRate float64 `json:"weightBonusRate"`
}
type handler struct{}

func Register(r *operationeffect.Registry) error { return r.Register(Kind, handler{}) }
func (handler) Domain() operationeffect.Domain   { return Domain }
func decode(raw json.RawMessage) (float64, error) {
	var p Params
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return 0, err
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF || math.IsNaN(p.WeightBonusRate) || p.WeightBonusRate <= 0 || p.WeightBonusRate > .25 {
		return 0, errors.New("invalid enchantment weight bonus")
	}
	return p.WeightBonusRate, nil
}
func (handler) ValidateParams(raw json.RawMessage) error { _, err := decode(raw); return err }
func (handler) Applies(op operationeffect.Operation, _ json.RawMessage) bool {
	return op.Domain == Domain && op.ActionID == "equipment.enchant" && op.ActorKind == operationeffect.ActorPlayer && op.Capabilities[Eligible]
}
func (handler) ValidateDefinitionConstraints(d operationeffect.Definition) error {
	if d.Lifetime.Kind != operationeffect.LifetimeCharges || d.Lifetime.Charges != 1 {
		return errors.New("enchantment serving must have one charge")
	}
	for _, id := range d.Selector.ActionIDs {
		if id != "equipment.enchant" {
			return errors.New("enchantment selector mismatch")
		}
	}
	return nil
}
func Resolve(s operationeffect.Snapshot) (float64, error) {
	total := 0.0
	for _, r := range s.Rules {
		if r.Domain == Domain && r.Kind == Kind {
			v, err := decode(r.Params)
			if err != nil {
				return 0, err
			}
			total += v
		}
	}
	return math.Min(.25, total), nil
}
func EligibleWeights(weights map[string]float64) bool {
	var high, low bool
	for rarity, weight := range weights {
		if weight > 0 {
			if rarity == "epic" || rarity == "legendary" {
				high = true
			} else {
				low = true
			}
		}
	}
	return high && low
}

// Input must already contain only equipment/tool-compatible candidates.
func AdjustWeights(weights map[string]float64, bonus float64) map[string]float64 {
	result := make(map[string]float64, len(weights))
	if !EligibleWeights(weights) || math.IsNaN(bonus) {
		bonus = 0
	}
	bonus = math.Max(0, math.Min(.25, bonus))
	for rarity, weight := range weights {
		if rarity == "epic" || rarity == "legendary" {
			weight *= 1 + bonus
		}
		result[rarity] = weight
	}
	return result
}
