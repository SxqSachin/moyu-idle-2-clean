package lifeaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"mysecondwebgame/server/internal/operationeffect"
)

const (
	KindMaterialRefundChance  = "life_action.material_refund_chance"
	KindExtraMainOutputChance = "life_action.extra_main_output_chance"

	CapabilityTemplateCosts = "template_costs"
	CapabilityMainOutput    = "main_output"
)

type chanceParams struct {
	Chance float64 `json:"chance"`
}

type materialRefundParams struct {
	Chance     float64 `json:"chance"`
	RefundRate float64 `json:"refundRate"`
}

type extraMainOutputParams struct {
	Chance   float64 `json:"chance"`
	Quantity int64   `json:"quantity"`
}

type handler struct {
	kind string
}

func Register(registry *operationeffect.Registry) error {
	if registry == nil {
		return errors.New("operation effect registry is required")
	}
	for _, kind := range []string{KindMaterialRefundChance, KindExtraMainOutputChance, KindHarvestBonus, KindFocusedHarvest, KindPatientHarvest, KindWorkExperience} {
		if err := registry.Register(kind, handler{kind: kind}); err != nil {
			return err
		}
	}
	return nil
}

func (h handler) Domain() operationeffect.Domain {
	return operationeffect.DomainLifeAction
}

func (h handler) StableAcrossStarts() bool {
	return true
}

func (h handler) ValidateParams(raw json.RawMessage) error {
	if IsWorkMeal(h.kind) {
		_, err := decodeWorkMeal(h.kind, raw)
		return err
	}
	switch h.kind {
	case KindMaterialRefundChance:
		var params materialRefundParams
		if err := decodeStrict(raw, &params); err != nil {
			return err
		}
		if err := validateChance(params.Chance); err != nil {
			return err
		}
		if params.RefundRate <= 0 || params.RefundRate > 1 || math.IsNaN(params.RefundRate) || math.IsInf(params.RefundRate, 0) {
			return errors.New("refundRate must be in (0, 1]")
		}
	case KindExtraMainOutputChance:
		var params extraMainOutputParams
		if err := decodeStrict(raw, &params); err != nil {
			return err
		}
		if err := validateChance(params.Chance); err != nil {
			return err
		}
		if params.Quantity <= 0 || params.Quantity > 1000 {
			return errors.New("quantity must be in [1, 1000]")
		}
	default:
		return fmt.Errorf("unsupported life action effect kind: %s", h.kind)
	}
	return nil
}

func (h handler) Applies(operation operationeffect.Operation, _ json.RawMessage) bool {
	if operation.Domain != operationeffect.DomainLifeAction {
		return false
	}
	if IsWorkMeal(h.kind) {
		return IsPrimaryWork(operation.MainType) && operation.Capabilities[CapabilityMainOutput]
	}
	switch h.kind {
	case KindMaterialRefundChance:
		return operation.Capabilities[CapabilityTemplateCosts]
	case KindExtraMainOutputChance:
		return operation.Capabilities[CapabilityMainOutput]
	default:
		return false
	}
}

type Refund struct {
	GrantID      int64
	DefinitionID string
	Items        map[string]int64
}

type ExtraOutput struct {
	GrantID      int64
	DefinitionID string
	ItemID       string
	Quantity     int64
}

type Outcome struct {
	Refunds      []Refund
	ExtraOutputs []ExtraOutput
}

func Resolve(snapshot operationeffect.Snapshot, costs map[string]int64, mainOutput string) (Outcome, error) {
	return (*Resolver)(nil).Resolve(snapshot, costs, mainOutput)
}

// Resolver reuses decoded parameters within one bounded settlement. It never
// caches rolls, operation keys, costs or selected grants. Do not share it across
// goroutines or retain it beyond the batch.
type Resolver struct {
	// Sample, when present, supplies a versioned Bernoulli decision. Rule
	// semantics (including correlated multi-material refunds) remain here.
	Sample  func(operationeffect.RuleSnapshot, float64) bool
	meals   map[mealParamKey]WorkMealParams
	refunds map[string]materialRefundParams
	extras  map[string]extraMainOutputParams
	rng     *rand.Rand
}

func (r *Resolver) hit(snapshot operationeffect.Snapshot, rule operationeffect.RuleSnapshot, chance float64) bool {
	if r != nil && r.Sample != nil {
		return r.Sample(rule, chance)
	}
	return r.roll(snapshot.OperationKey, rule.GrantID, rule.Kind) < chance
}

type mealParamKey struct{ kind, raw string }

func (r *Resolver) roll(operationKey string, grantID int64, kind string) float64 {
	if r == nil {
		return operationeffect.Roll(operationKey, grantID, kind, "proc")
	}
	seed := operationeffect.DeterministicSeed(operationKey, grantID, kind, "proc")
	if r.rng == nil {
		r.rng = rand.New(rand.NewSource(seed))
	} else {
		r.rng.Seed(seed)
	}
	return r.rng.Float64()
}

func (r *Resolver) meal(kind string, raw json.RawMessage) (WorkMealParams, error) {
	if r == nil {
		return decodeWorkMeal(kind, raw)
	}
	key := mealParamKey{kind, string(raw)}
	if value, ok := r.meals[key]; ok {
		return value, nil
	}
	value, err := decodeWorkMeal(kind, raw)
	if err == nil {
		if r.meals == nil {
			r.meals = make(map[mealParamKey]WorkMealParams)
		}
		r.meals[key] = value
	}
	return value, err
}

func (r *Resolver) refund(raw json.RawMessage) (materialRefundParams, error) {
	var value materialRefundParams
	if r == nil {
		err := decodeStrict(raw, &value)
		return value, err
	}
	key := string(raw)
	if value, ok := r.refunds[key]; ok {
		return value, nil
	}
	err := decodeStrict(raw, &value)
	if err == nil {
		if r.refunds == nil {
			r.refunds = make(map[string]materialRefundParams)
		}
		r.refunds[key] = value
	}
	return value, err
}

func (r *Resolver) extra(raw json.RawMessage) (extraMainOutputParams, error) {
	var value extraMainOutputParams
	if r == nil {
		err := decodeStrict(raw, &value)
		return value, err
	}
	key := string(raw)
	if value, ok := r.extras[key]; ok {
		return value, nil
	}
	err := decodeStrict(raw, &value)
	if err == nil {
		if r.extras == nil {
			r.extras = make(map[string]extraMainOutputParams)
		}
		r.extras[key] = value
	}
	return value, err
}

func (r *Resolver) Resolve(snapshot operationeffect.Snapshot, costs map[string]int64, mainOutput string) (Outcome, error) {
	outcome := Outcome{}
	if _, err := r.WorkMealModifiers(snapshot); err != nil {
		return outcome, err
	}
	mainOutput = strings.TrimSpace(mainOutput)
	for _, rule := range snapshot.Rules {
		if rule.Domain != operationeffect.DomainLifeAction {
			continue
		}
		switch rule.Kind {
		case KindHarvestBonus, KindFocusedHarvest, KindPatientHarvest:
			params, err := r.meal(rule.Kind, rule.Params)
			if err != nil {
				return Outcome{}, err
			}
			if mainOutput != "" && r.hit(snapshot, rule, params.Chance) {
				outcome.ExtraOutputs = append(outcome.ExtraOutputs, ExtraOutput{GrantID: rule.GrantID, DefinitionID: rule.DefinitionID, ItemID: mainOutput, Quantity: 1})
			}
		case KindMaterialRefundChance:
			params, err := r.refund(rule.Params)
			if err != nil {
				return Outcome{}, err
			}
			if !r.hit(snapshot, rule, params.Chance) {
				continue
			}
			items := make(map[string]int64)
			for itemID, quantity := range costs {
				if quantity <= 0 {
					continue
				}
				refunded := int64(math.Floor(float64(quantity) * params.RefundRate))
				if refunded > 0 {
					items[itemID] += refunded
				}
			}
			if len(items) > 0 {
				outcome.Refunds = append(outcome.Refunds, Refund{GrantID: rule.GrantID, DefinitionID: rule.DefinitionID, Items: items})
			}
		case KindExtraMainOutputChance:
			if mainOutput == "" {
				continue
			}
			params, err := r.extra(rule.Params)
			if err != nil {
				return Outcome{}, err
			}
			if !r.hit(snapshot, rule, params.Chance) {
				continue
			}
			outcome.ExtraOutputs = append(outcome.ExtraOutputs, ExtraOutput{
				GrantID: rule.GrantID, DefinitionID: rule.DefinitionID, ItemID: mainOutput, Quantity: params.Quantity,
			})
		}
	}
	return outcome, nil
}

func validateChance(chance float64) error {
	if chance < 0 || chance > 1 || math.IsNaN(chance) || math.IsInf(chance, 0) {
		return errors.New("chance must be in [0, 1]")
	}
	return nil
}

func decodeStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}
