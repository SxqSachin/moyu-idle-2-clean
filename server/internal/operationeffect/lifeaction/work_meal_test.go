package lifeaction

import (
	"fmt"
	"mysecondwebgame/server/internal/operationeffect"
	"reflect"
	"testing"
)

func TestWorkMealValidationAndEligibility(t *testing.T) {
	for _, tc := range []struct {
		kind, raw string
		valid     bool
	}{
		{KindHarvestBonus, `{"chance":0.05}`, true},
		{KindHarvestBonus, `{"chance":0.051}`, false},
		{KindPatientHarvest, `{"chance":0.10,"durationPenaltyRate":0.04}`, true},
		{KindPatientHarvest, `{"chance":0.08}`, false},
		{KindPatientHarvest, `{"chance":0.08,"durationPenaltyRate":0.02}`, false},
		{KindWorkExperience, `{"experienceBonusRate":0.15}`, true},
		{KindWorkExperience, `{"experienceBonusRate":0.151}`, false},
		{KindWorkExperience, `{"experienceBonusRate":0.08,"chance":0.01}`, false},
	} {
		h := handler{kind: tc.kind}
		if (h.ValidateParams([]byte(tc.raw)) == nil) != tc.valid {
			t.Fatal(tc)
		}
		for _, main := range []string{"collecting", "mining", "farming_animal", "cooking", "alchemy", "fishing"} {
			op := operationeffect.Operation{Domain: operationeffect.DomainLifeAction, MainType: main, Capabilities: map[string]bool{CapabilityMainOutput: true}}
			if h.Applies(op, nil) != IsPrimaryWork(main) {
				t.Fatal(main)
			}
			op.Capabilities = nil
			if h.Applies(op, nil) {
				t.Fatal("missing main output accepted")
			}
		}
		d := operationeffect.Definition{Selector: operationeffect.Selector{MainTypes: []string{"collecting"}}, Stacking: operationeffect.StackingDefinition{Key: "food.primary_work", Exclusive: true}}
		if err := h.ValidateDefinitionConstraints(d); err != nil {
			t.Fatal(err)
		}
		d.Selector.MainTypes = []string{"cooking"}
		if h.ValidateDefinitionConstraints(d) == nil {
			t.Fatal("crafting scope accepted")
		}
	}
}

func TestWorkMealBonusIsDeterministicSingleMainItem(t *testing.T) {
	s := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{GrantID: 1, Domain: operationeffect.DomainLifeAction, Kind: KindHarvestBonus, Params: []byte(`{"chance":0.05}`)}}}
	hits := 0
	for i := 0; i < 10000; i++ {
		s.OperationKey = fmt.Sprintf("work:%d", i)
		got, err := Resolve(s, map[string]int64{"wood": 20}, "ore")
		if err != nil {
			t.Fatal(err)
		}
		replayed, err := Resolve(s, map[string]int64{"wood": 20}, "ore")
		if err != nil || !reflect.DeepEqual(got, replayed) {
			t.Fatal("retry changed outcome")
		}
		if len(got.Refunds) != 0 || len(got.ExtraOutputs) > 1 {
			t.Fatal(got)
		}
		for _, out := range got.ExtraOutputs {
			if out.ItemID != "ore" || out.Quantity != 1 {
				t.Fatal(out)
			}
			hits++
		}
	}
	if hits < 400 || hits > 600 {
		t.Fatalf("unexpected deterministic distribution %d", hits)
	}
	s.Rules = append(s.Rules, s.Rules[0])
	if _, err := WorkMealModifiers(s); err == nil {
		t.Fatal("stacked work meals accepted")
	}
}
