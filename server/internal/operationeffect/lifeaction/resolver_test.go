package lifeaction

import (
	"fmt"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
)

func resolverTestRules() []operationeffect.RuleSnapshot {
	return []operationeffect.RuleSnapshot{
		{GrantID: 11, DefinitionID: "refund", Domain: operationeffect.DomainLifeAction, Kind: KindMaterialRefundChance, Params: []byte(`{"chance":0.37,"refundRate":0.5}`)},
		{GrantID: 12, DefinitionID: "extra", Domain: operationeffect.DomainLifeAction, Kind: KindExtraMainOutputChance, Params: []byte(`{"chance":0.73,"quantity":2}`)},
		{GrantID: 13, DefinitionID: "meal", Domain: operationeffect.DomainLifeAction, Kind: KindFocusedHarvest, Params: []byte(`{"chance":0.0272,"durationReductionRate":0.04}`)},
	}
}

func TestBatchResolverMatchesUncached(t *testing.T) {
	resolver := Resolver{}
	for i := 0; i < 5000; i++ {
		rules := resolverTestRules()
		switch i % 9 {
		case 0:
			rules = nil
		case 1:
			rules[0].Params = []byte(`{"chance":1,"refundRate":0.2}`)
		case 2:
			rules[2].Kind = KindWorkExperience
			rules[2].Params = []byte(`{"experienceBonusRate":0.123456}`)
		case 3:
			rules[2].Params = []byte(`{"chance":0.04,"durationReductionRate":0.04}`)
		case 4:
			rules = append(rules, rules[2])
		case 5:
			rules[0].Params = []byte(`{"chance":1,"refundRate":0.2,"unknown":1}`)
		case 6:
			rules[1].Domain = operationeffect.DomainAlchemy
		case 7:
			rules[1].Params = []byte(`{"chance":0,"quantity":7}`)
		case 8:
			rules[1].GrantID = int64(i)
			rules[1].DefinitionID = "changed"
		}
		snapshot := operationeffect.Snapshot{OperationKey: fmt.Sprintf("queue:%d", i), Rules: rules}
		costs := map[string]int64{"wood": int64(i % 31), "stone": 11, "negative": -1}
		output := "plank"
		if i%13 == 0 {
			output = ""
		}
		want, wantErr := Resolve(snapshot, costs, output)
		got, gotErr := resolver.Resolve(snapshot, costs, output)
		if !reflect.DeepEqual(got, want) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
			t.Fatalf("case %d: got=%+v/%v want=%+v/%v", i, got, gotErr, want, wantErr)
		}
		wantMeal, wantErr := WorkMealModifiers(snapshot)
		gotMeal, gotErr := resolver.WorkMealModifiers(snapshot)
		if gotMeal != wantMeal || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
			t.Fatalf("meal case %d differs", i)
		}
	}
}

func BenchmarkBatchLifeActionResolver(b *testing.B) {
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprintf("cached=%t", cached), func(b *testing.B) {
			snapshots := make([]operationeffect.Snapshot, 512)
			for i := range snapshots {
				snapshots[i] = operationeffect.Snapshot{OperationKey: fmt.Sprintf("queue:%d", i), Rules: resolverTestRules()}
			}
			costs := map[string]int64{"wood": 5}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				var resolver *Resolver
				if cached {
					resolver = &Resolver{}
				}
				for _, snapshot := range snapshots {
					if _, err := resolver.Resolve(snapshot, costs, "plank"); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
