package operationeffect

import (
	"math"
	"testing"
)

func TestContributionMergeDifferenceAndBounds(t *testing.T) {
	first := Contributions{"desk": {Experience: map[string]float64{"gather": 1.2345}}, "cabinet": {Items: map[string]int64{"trophy": 2}, Opportunities: 4, Triggered: 2, Empty: 1}}
	second := Contributions{"desk": {Experience: map[string]float64{"gather": 0.0001}}, "cabinet": {Items: map[string]int64{"trophy": 1}, Opportunities: 3, Triggered: 1}}
	total, err := MergeContributions(first, second)
	if err != nil {
		t.Fatal(err)
	}
	diff := ContributionDifference(total, first)
	if diff["desk"].Experience["gather"] != 0.0001 || diff["cabinet"].Items["trophy"] != 1 || diff["cabinet"].Opportunities != 3 || diff["cabinet"].Triggered != 1 {
		t.Fatalf("bad difference: %+v", diff)
	}
	total["desk"].Experience["gather"] = 10
	if first["desk"].Experience["gather"] != 1.2345 {
		t.Fatal("merge mutated baseline")
	}
	for _, bad := range []Contribution{{Opportunities: math.MaxInt64}, {Experience: map[string]float64{"g": math.NaN()}}, {Triggered: 1}, {Opportunities: 1, Triggered: 1, Empty: 2}, {Items: map[string]int64{"x": -1}}} {
		if (Contributions{"x": bad}).Validate() == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	if _, err := MergeContributions(Contributions{"x": {Opportunities: 1e12}}, Contributions{"x": {Opportunities: 1}}); err == nil {
		t.Fatal("counter overflow accepted")
	}
}

func TestExperienceContributionUsesAcceptedQuantity(t *testing.T) {
	for _, tc := range []struct{ applied, want float64 }{{115, 15}, {110, 10}, {99, 0}, {0, 0}} {
		p := &PassiveSession{PendingExperience: map[string]map[string]float64{"desk": {"gather": 15}}}
		owner := map[string]float64{"gather": 15}
		if err := p.CommitExperience(map[string]float64{"gather": 115}, map[string]float64{"gather": tc.applied}, owner); err != nil {
			t.Fatal(err)
		}
		if p.Contributions["desk"].Experience["gather"] != tc.want || owner["gather"] != tc.want || len(p.PendingExperience) != 0 {
			t.Fatalf("cap %v: %+v owner %+v", tc.applied, p.Contributions, owner)
		}
	}
}
