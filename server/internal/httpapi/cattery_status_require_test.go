package httpapi

import (
	"reflect"
	"testing"
)

func TestUnmetStatusRequireFromFinalValuesUsesTargetKittyAttrs(t *testing.T) {
	require := map[string]int64{"strength": 10}

	playerAttrs := map[string]float64{"strength": 65}
	kittyAAttrs := map[string]float64{"strength": 1}
	kittyBAttrs := map[string]float64{"strength": 12}

	if got := unmetStatusRequireFromFinalValues(playerAttrs, require); len(got) != 0 {
		t.Fatalf("player attrs sanity check unmet=%v, want none", got)
	}
	if got := unmetStatusRequireFromFinalValues(kittyAAttrs, require); !reflect.DeepEqual(got, []string{"strength"}) {
		t.Fatalf("kitty A unmet=%v, want strength", got)
	}
	if got := unmetStatusRequireFromFinalValues(kittyBAttrs, require); len(got) != 0 {
		t.Fatalf("kitty B unmet=%v, want none", got)
	}
}

func TestUnmetStatusRequireFromFinalValuesNormalizesKeys(t *testing.T) {
	got := unmetStatusRequireFromFinalValues(
		map[string]float64{"strength": 9, "constitution": 20},
		map[string]int64{" strength ": 10, "": 10, "constitution": 0},
	)
	if !reflect.DeepEqual(got, []string{"strength"}) {
		t.Fatalf("unmet=%v, want strength only", got)
	}
}

func TestUnmetStatusRequireUsesKittyFishingLevel(t *testing.T) {
	require := map[string]int64{"fishing": 18}
	for _, test := range []struct {
		player, kitty float64
		allowed       bool
	}{
		{1, 18, true},
		{45, 17, false},
		{1, 32, true},
	} {
		player := map[string]float64{"fishing": test.player}
		kitty := map[string]float64{"fishing": test.kitty}
		if got := unmetStatusRequireFromFinalValues(kitty, require); (len(got) == 0) != test.allowed {
			t.Fatalf("player=%v kitty=%v unmet=%v", player, kitty, got)
		}
	}
}
