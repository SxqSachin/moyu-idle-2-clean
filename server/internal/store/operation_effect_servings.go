package store

import "mysecondwebgame/server/internal/operationeffect"

// Scale the serving, never the definition or its per-operation parameters.
func operationEffectServingAmount(def operationeffect.Definition, quantity int64) (int64, error) {
	if quantity < 1 || quantity > maxOperationEffectUseQuantity || (quantity > 1 && def.Stacking.Mode == operationeffect.StackIndependent) {
		return 0, ErrOperationEffectRequestInvalid
	}
	unit, limit := def.Lifetime.Charges, def.Stacking.MaxCharges
	if def.Lifetime.Kind == operationeffect.LifetimeDuration {
		unit, limit = def.Lifetime.DurationSeconds, def.Stacking.MaxDurationSeconds
		if limit == 0 || limit > operationeffect.MaxDurationSeconds {
			limit = operationeffect.MaxDurationSeconds
		}
	} else if def.Lifetime.Kind == operationeffect.LifetimeCharges {
		if limit == 0 || limit > operationeffect.MaxChargeCount {
			limit = operationeffect.MaxChargeCount
		}
	} else {
		return 0, ErrOperationEffectRequestInvalid
	}
	if unit <= 0 {
		return 0, ErrOperationEffectRequestInvalid
	}
	if quantity > limit/unit {
		return 0, ErrOperationEffectAtCap
	}
	return unit * quantity, nil
}
