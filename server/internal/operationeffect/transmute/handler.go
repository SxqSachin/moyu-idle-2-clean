// Package transmute owns point-gold modifiers; it never changes infusion rules.
package transmute

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"

	"mysecondwebgame/server/internal/operationeffect"
)

const Speed = "alchemy.transmute_duration"
const Gold = "alchemy.transmute_gold_bonus"
const GoldEligible = "transmute.gold_eligible"

type Params struct {
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	BonusRate       float64 `json:"bonusRate,omitempty"`
}
type handler struct{ kind string }

func Register(r *operationeffect.Registry) error {
	for _, kind := range []string{Speed, Gold} {
		if err := r.Register(kind, handler{kind}); err != nil {
			return err
		}
	}
	return nil
}
func (h handler) Domain() operationeffect.Domain { return operationeffect.DomainAlchemy }
func (h handler) StableAcrossStarts() bool       { return true }
func decode(kind string, raw json.RawMessage) (Params, error) {
	var p Params
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF || math.IsNaN(p.BonusRate) || math.IsInf(p.BonusRate, 0) {
		return p, errors.New("invalid transmute modifier")
	}
	if kind == Speed && p.DurationSeconds == 1 && p.BonusRate == 0 {
		return p, nil
	}
	if kind == Gold && p.DurationSeconds == 0 && p.BonusRate > 0 && p.BonusRate <= .10 {
		return p, nil
	}
	return p, errors.New("invalid transmute modifier")
}
func (h handler) ValidateParams(raw json.RawMessage) error { _, err := decode(h.kind, raw); return err }
func (h handler) Applies(op operationeffect.Operation, _ json.RawMessage) bool {
	return op.Domain == h.Domain() && op.ActionID == "alchemy.transmute" && (op.ActorKind == operationeffect.ActorPlayer || op.ActorKind == operationeffect.ActorKitty) && (h.kind != Gold || op.Capabilities[GoldEligible])
}
func (h handler) ValidateDefinitionConstraints(d operationeffect.Definition) error {
	if h.kind == Speed && d.Lifetime.Kind != operationeffect.LifetimeCharges {
		return errors.New("transmute speed must be count limited")
	}
	for _, id := range d.Selector.ActionIDs {
		if id != "alchemy.transmute" {
			return errors.New("transmute selector mismatch")
		}
	}
	return nil
}
func Resolve(s operationeffect.Snapshot) (Params, error) {
	var result Params
	for _, r := range s.Rules {
		if r.Domain != operationeffect.DomainAlchemy || (r.Kind != Speed && r.Kind != Gold) {
			continue
		}
		p, err := decode(r.Kind, r.Params)
		if err != nil {
			return result, err
		}
		result.DurationSeconds = math.Max(result.DurationSeconds, p.DurationSeconds)
		result.BonusRate = math.Min(.10, result.BonusRate+p.BonusRate)
	}
	return result, nil
}

// Integer basis points avoid float conversion at the int64 boundary. Round down
// per input, before any aggregation, and never multiply this bonus by luck.
func BonusGold(normal int64, rate float64) int64 {
	if normal <= 0 || rate <= 0 || rate > .10 || math.IsNaN(rate) {
		return 0
	}
	bps := int64(math.Round(rate * 10000))
	return normal/10000*bps + normal%10000*bps/10000
}
