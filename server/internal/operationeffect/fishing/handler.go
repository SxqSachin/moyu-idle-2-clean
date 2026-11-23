// Package fishing owns bounded adjustments to the fight within a frozen cast.
package fishing

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"

	"mysecondwebgame/server/internal/operationeffect"
)

const LandingBonus = "fishing_cast.landing_bonus"
const LineLoadReduction = "fishing_cast.line_load_reduction"
const MaxRate = 0.15

type Params struct {
	Rate float64 `json:"rate"`
}
type Modifiers struct{ LandingBonusRate, LineLoadReductionRate float64 }
type handler struct{}

func Register(r *operationeffect.Registry) error {
	for _, kind := range []string{LandingBonus, LineLoadReduction} {
		if err := r.Register(kind, handler{}); err != nil {
			return err
		}
	}
	return nil
}
func (handler) Domain() operationeffect.Domain           { return operationeffect.DomainFishingCast }
func (handler) ValidateParams(raw json.RawMessage) error { _, err := decode(raw); return err }
func (handler) Applies(op operationeffect.Operation, _ json.RawMessage) bool {
	return op.Domain == operationeffect.DomainFishingCast && op.ActionID == "fishing.trip" && (op.ActorKind == operationeffect.ActorPlayer || op.ActorKind == operationeffect.ActorKitty)
}
func (handler) ValidateDefinitionConstraints(d operationeffect.Definition) error {
	for _, id := range d.Selector.ActionIDs {
		if id != "fishing.trip" {
			return errors.New("fishing effect action mismatch")
		}
	}
	return nil
}
func decode(raw json.RawMessage) (Params, error) {
	var p Params
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF || math.IsNaN(p.Rate) || math.IsInf(p.Rate, 0) || p.Rate <= 0 || p.Rate > MaxRate {
		return p, errors.New("invalid fishing effect rate")
	}
	return p, nil
}
func Resolve(snapshot operationeffect.Snapshot) (Modifiers, error) {
	var out Modifiers
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainFishingCast || (rule.Kind != LandingBonus && rule.Kind != LineLoadReduction) {
			continue
		}
		p, err := decode(rule.Params)
		if err != nil {
			return out, err
		}
		if rule.Kind == LandingBonus {
			out.LandingBonusRate = math.Min(MaxRate, out.LandingBonusRate+p.Rate)
		} else {
			out.LineLoadReductionRate = math.Min(MaxRate, out.LineLoadReductionRate+p.Rate)
		}
	}
	return out, nil
}
