// Package ratebonus owns bounded probability additions for atomic operations.
package ratebonus

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"

	"mysecondwebgame/server/internal/operationeffect"
)

const EnhanceSuccess = "enhancement.success_bonus"
const FishingBite = "fishing_cast.bite_bonus"

type Params struct {
	BonusRate float64 `json:"bonusRate"`
}
type handler struct {
	domain   operationeffect.Domain
	actionID string
	cap      float64
}

func Register(r *operationeffect.Registry) error {
	if err := r.Register(EnhanceSuccess, handler{operationeffect.DomainEnhancement, "equipment_enhance", 0.10}); err != nil {
		return err
	}
	return r.Register(FishingBite, handler{operationeffect.DomainFishingCast, "fishing.trip", 0.25})
}
func (h handler) Domain() operationeffect.Domain { return h.domain }
func (h handler) ValidateParams(raw json.RawMessage) error {
	_, err := decode(raw, h.cap)
	return err
}
func (h handler) Applies(op operationeffect.Operation, _ json.RawMessage) bool {
	return op.Domain == h.domain && op.ActionID == h.actionID && (op.ActorKind == operationeffect.ActorPlayer || op.ActorKind == operationeffect.ActorKitty)
}
func (h handler) ValidateDefinitionConstraints(d operationeffect.Definition) error {
	for _, id := range d.Selector.ActionIDs {
		if id != h.actionID {
			return errors.New("rate bonus action selector mismatch")
		}
	}
	return nil
}
func decode(raw json.RawMessage, cap float64) (Params, error) {
	var p Params
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF || math.IsNaN(p.BonusRate) || math.IsInf(p.BonusRate, 0) || p.BonusRate <= 0 || p.BonusRate > cap {
		return p, errors.New("invalid rate bonus")
	}
	return p, nil
}
func Resolve(snapshot operationeffect.Snapshot, kind string) (float64, error) {
	cap, domain := 0.10, operationeffect.DomainEnhancement
	if kind == FishingBite {
		cap, domain = 0.25, operationeffect.DomainFishingCast
	} else if kind != EnhanceSuccess {
		return 0, errors.New("unknown rate bonus kind")
	}
	total := 0.0
	for _, rule := range snapshot.Rules {
		if rule.Kind != kind || rule.Domain != domain {
			continue
		}
		p, err := decode(rule.Params, cap)
		if err != nil {
			return 0, err
		}
		total += p.BonusRate
	}
	return math.Min(cap, total), nil
}
