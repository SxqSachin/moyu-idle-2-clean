package store

import (
	"errors"
	"math"
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
)

func TestFoodReserveCapsDoNotChangeSingleServings(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, recipe := range s.gameData.FishFoodRecipes {
		def, ok, err := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, nil)
		if err != nil || !ok {
			t.Fatal(err)
		}
		if def.Lifetime.Kind == operationeffect.LifetimeDuration {
			if def.Stacking.MaxDurationSeconds != 86400 {
				t.Fatal(recipe.ID, def.Stacking)
			}
		} else if def.Stacking.MaxCharges != def.Lifetime.Charges*100 {
			t.Fatal(recipe.ID, def.Stacking)
		}
		if def.Kind == "enchantment.high_rarity_weight" && def.Lifetime.Charges != 1 {
			t.Fatal("enchantment strength increased")
		}
		for _, q := range []int64{0, -1, 10001, math.MaxInt64} {
			if _, err := operationEffectServingAmount(def, q); !errors.Is(err, ErrOperationEffectRequestInvalid) {
				t.Fatal(q, err)
			}
		}
		if _, err := operationEffectServingAmount(def, 1); err != nil {
			t.Fatal(recipe.ID, err)
		}
	}
}

func TestBatchServingAmountRejectsOverflowAndWholeServingOverCap(t *testing.T) {
	def := operationeffect.Definition{Lifetime: operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 900}, Stacking: operationeffect.StackingDefinition{Mode: operationeffect.StackExtendDuration, MaxDurationSeconds: 86400}}
	if got, err := operationEffectServingAmount(def, 96); err != nil || got != 86400 {
		t.Fatal(got, err)
	}
	if _, err := operationEffectServingAmount(def, 97); !errors.Is(err, ErrOperationEffectAtCap) {
		t.Fatal(err)
	}
	def.Lifetime.DurationSeconds = math.MaxInt64
	if _, err := operationEffectServingAmount(def, 2); !errors.Is(err, ErrOperationEffectAtCap) {
		t.Fatal(err)
	}
}
