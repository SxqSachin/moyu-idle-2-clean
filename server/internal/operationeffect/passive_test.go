package operationeffect

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func testPassiveSource(rate int64) PassiveSource {
	return PassiveSource{ID: "facility", Version: "v1", Rules: []PassiveRule{{Kind: KindOwnerExperience, Domain: DomainLifeAction, RateBPS: rate}}}
}

func TestPassiveWindowSelectorsAndFrozenSnapshot(t *testing.T) {
	at := time.Unix(100, 0)
	source := testPassiveSource(1500)
	source.Rules[0].Selector = Selector{MainTypes: []string{"gather"}}
	p := &PassiveSession{Windows: []PassiveWindow{{From: at, Until: at.Add(time.Minute), Source: source}}}
	op := Operation{StartedAt: at, ActorKind: ActorKitty, Domain: DomainLifeAction, MainType: "gather"}
	frozen := p.Snapshot(op)
	if len(frozen.Sources) != 1 {
		t.Fatal("active source missing")
	}
	for _, date := range []time.Time{at.Add(-time.Nanosecond), at.Add(time.Minute)} {
		op.StartedAt = date
		if len(p.Snapshot(op).Sources) != 0 {
			t.Fatal("half-open interval violated")
		}
	}
	op.StartedAt = at
	op.ActorKind = ActorPlayer
	if len(p.Snapshot(op).Sources) != 0 {
		t.Fatal("player was eligible")
	}
	op.ActorKind = ActorKitty
	op.MainType = "craft"
	if len(p.Snapshot(op).Sources) != 0 {
		t.Fatal("selector ignored")
	}
	p.Windows = nil
	gain, err := p.Share(frozen, map[string]float64{"woodcutting": 100})
	if err != nil || gain["woodcutting"] != 15 {
		t.Fatalf("frozen effect lost: %v %v", gain, err)
	}
}

func TestPassiveExperiencePartitionAndPersistence(t *testing.T) {
	snap := PassiveSnapshot{Version: 1, Sources: []PassiveSource{testPassiveSource(1500)}}
	one, batched := &PassiveSession{}, &PassiveSession{}
	total := 0.0
	for i := 0; i < 10001; i++ {
		gain, err := one.Share(snap, map[string]float64{"fishing": 0.0001})
		if err != nil {
			t.Fatal(err)
		}
		total += gain["fishing"]
		if i%37 == 0 {
			raw, _ := json.Marshal(one.Remainders)
			one = &PassiveSession{}
			if err := json.Unmarshal(raw, &one.Remainders); err != nil {
				t.Fatal(err)
			}
		}
	}
	gain, err := batched.ShareRepeated(snap, map[string]float64{"fishing": 0.0001}, 10001)
	if err != nil || math.Abs(total-gain["fishing"]) > 1e-10 || !reflect.DeepEqual(one.Remainders, batched.Remainders) {
		t.Fatalf("partition mismatch %v %v %v", total, gain, err)
	}
	if gain["fishing"] != 0.15 || batched.Remainders["fishing"] != 1500 {
		t.Fatalf("unexpected fixed point result: %v %v", gain, batched.Remainders)
	}
}

func TestPassiveInvalidAndMaxStack(t *testing.T) {
	source := testPassiveSource(1500)
	second := testPassiveSource(2000)
	second.ID = "other"
	snap := PassiveSnapshot{Version: 1, Sources: []PassiveSource{source, second}}
	if n, _ := snap.Best(KindOwnerExperience); n != 2000 {
		t.Fatal("must choose max rather than add")
	}
	for _, value := range []float64{-1, math.Inf(1), math.NaN(), 1e30} {
		if _, err := (&PassiveSession{}).Share(snap, map[string]float64{"x": value}); err == nil {
			t.Fatal("invalid XP accepted")
		}
	}
	if _, err := (&PassiveSession{}).ShareRepeated(snap, map[string]float64{"x": 1e8}, math.MaxInt64); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := (*PassiveSession)(nil).Share(snap, map[string]float64{"x": 1}); err == nil {
		t.Fatal("missing owner state accepted")
	}
	snap.Sources = append(snap.Sources, source)
	if snap.Validate() == nil {
		t.Fatal("duplicate source accepted")
	}
	if (PassiveRule{Kind: KindOwnerExperience, Domain: DomainBattleReward, RateBPS: 100}).Validate() == nil {
		t.Fatal("wrong domain accepted")
	}
}

func BenchmarkPassiveRepeated(b *testing.B) {
	snap := PassiveSnapshot{Version: 1, Sources: []PassiveSource{testPassiveSource(1500)}}
	exp := map[string]float64{"woodcutting": 3.1234}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := &PassiveSession{}
		if _, err := p.ShareRepeated(snap, exp, 100000); err != nil {
			b.Fatal(err)
		}
	}
}
