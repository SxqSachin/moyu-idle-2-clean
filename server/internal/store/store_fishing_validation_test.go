package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func assertFishingValidation(t *testing.T, err error, sentinel error, reason, field string) {
	t.Helper()
	var validation *FishingValidationError
	if !errors.Is(err, sentinel) || !errors.As(err, &validation) {
		t.Fatalf("error = %v, want typed validation wrapping %v", err, sentinel)
	}
	if validation.Reason != reason || validation.Field != field || fishingValidationMessages[reason] == "" {
		t.Fatalf("validation = %+v, want reason=%s field=%s with a public message", validation, reason, field)
	}
	if cancel, reason := newFishingExecutor(&Store{}).ClassifyStartError(fmt.Errorf("wrapped: %w", err)); !cancel || reason != ActionQueueCancelReasonTargetInvalid {
		t.Fatal("typed error lost queue eligibility classification")
	}
}

func TestFishingValidationArgumentReasons(t *testing.T) {
	executor := newFishingExecutor(&Store{})
	tests := []struct {
		field  string
		value  any
		reason string
	}{
		{"spotId", "", "SPOT_REQUIRED"}, {"loadoutId", "", "LOADOUT_REQUIRED"},
		{"tournamentDivision", "bad", "DIVISION_INVALID"},
		{"durationSeconds", 599, "DURATION_INVALID"}, {"durationSeconds", 86401, "DURATION_INVALID"},
		{"strategy", "bad", "STRATEGY_INVALID"}, {"safetyLoadPercent", 49, "SAFETY_INVALID"},
		{"safetyLoadPercent", 101, "SAFETY_INVALID"}, {"targetLayer", "bad", "LAYER_INVALID"},
		{"pace", "bad", "PACE_INVALID"}, {"durationSeconds", "600", "PAYLOAD_INVALID"},
	}
	for _, test := range tests {
		t.Run(test.field+fmt.Sprint(test.value), func(t *testing.T) {
			args := map[string]any{"spotId": "spot", "loadoutId": "set", "supplies": FishingSupplyCounts{MainLine: 1, Leader: 1}}
			args[test.field] = test.value
			raw, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			_, err = executor.ValidateAndNormalizeArgs(raw)
			assertFishingValidation(t, err, ErrFishingArgsInvalid, test.reason, "")
		})
	}
	for _, raw := range []json.RawMessage{nil, []byte(`{`)} {
		_, err := executor.ValidateAndNormalizeArgs(raw)
		assertFishingValidation(t, err, ErrFishingArgsInvalid, "PAYLOAD_INVALID", "")
	}
	for _, duration := range []int64{0, 600, 86400} {
		raw, _ := json.Marshal(fishingExecutorArgs{SpotID: "spot", LoadoutID: "set", DurationSeconds: duration, Supplies: FishingSupplyCounts{MainLine: 1, Leader: 1}})
		if _, err := executor.ValidateAndNormalizeArgs(raw); err != nil {
			t.Fatalf("duration %d: %v", duration, err)
		}
	}
}

func TestFishingValidationSupplySlotsAndRules(t *testing.T) {
	for _, method := range []string{"float", "bottom", "lure"} {
		base := FishingSupplyCounts{MainLine: 1, Leader: 1, Hook: 1, FloatOrSinker: 1, Bait: 1}
		if method == "lure" {
			base = FishingSupplyCounts{MainLine: 1, Leader: 1, Lure: 1}
		}
		if err := validateFishingSuppliesForLoadout(base, method); err != nil {
			t.Fatal(err)
		}
		for index, entry := range fishingSupplyCountEntries(base) {
			for _, value := range []int64{-1, 0, 1, 100000, 100001} {
				counts := base
				fields := []*int64{&counts.MainLine, &counts.Leader, &counts.Hook, &counts.FloatOrSinker, &counts.Lure, &counts.Bait}
				*fields[index] = value
				err := validateFishingSuppliesForLoadout(counts, method)
				reason := ""
				switch {
				case value < 0 || value > 100000:
					reason = "SUPPLY_RANGE"
				case entry.count > 0 && value == 0:
					reason = "SUPPLY_REQUIRED"
				case entry.count == 0 && value > 0:
					reason = "SUPPLY_UNEXPECTED"
				}
				if reason == "" {
					if err != nil {
						t.Fatalf("%s %s=%d: %v", method, entry.slot, value, err)
					}
				} else {
					assertFishingValidation(t, err, ErrFishingArgsInvalid, reason, entry.slot)
				}
			}
		}
	}
}

func TestFishingValidationLoadoutSelectionBeforeQueries(t *testing.T) {
	s := &Store{}
	for _, test := range []struct {
		row           FishingLoadout
		reason, field string
	}{
		{FishingLoadout{}, "METHOD_INVALID", ""},
		{FishingLoadout{Method: "float"}, "COMPONENT_REQUIRED", "rod"},
		{FishingLoadout{Method: "float", Components: FishingLoadoutComponents{RodInstanceID: 1}}, "COMPONENT_REQUIRED", "reel"},
	} {
		_, err := s.freezeFishingLoadoutTx(context.Background(), nil, 1, test.row)
		assertFishingValidation(t, err, ErrFishingLoadoutInvalid, test.reason, test.field)
	}
}

func TestFishingValidationCodeSurvivesJoinedErrors(t *testing.T) {
	err := errors.Join(fishingLoadoutError("COMPONENT_INCOMPATIBLE", "float_or_sinker"), errors.New("private database diagnostic"))
	assertFishingValidation(t, err, ErrFishingLoadoutInvalid, "COMPONENT_INCOMPATIBLE", "float_or_sinker")
	var validation *FishingValidationError
	errors.As(err, &validation)
	if validation.Code() != "FISHING_SETTINGS_COMPONENT_INCOMPATIBLE_FLOAT_OR_SINKER" {
		t.Fatalf("code = %s", validation.Code())
	}
}
