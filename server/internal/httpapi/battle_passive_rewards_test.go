package httpapi

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/operationeffect"
	"reflect"
	"testing"
	"time"
)

func legacyPassiveRandom(state battleRuntimeState, owner, kitty int64, source, stage string) *rand.Rand {
	sum := sha256.Sum256([]byte(fmt.Sprintf("passive-battle-v1:%d:%d:%d:%d:%d:%s:%s", state.PassiveRunID, state.Seed, state.Wave, owner, kitty, source, stage)))
	return rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(sum[:8]))))
}

func TestPassiveBattlePooledRandomMatchesLegacySequence(t *testing.T) {
	rng := battlePassiveRNGPool.Get().(*rand.Rand)
	defer battlePassiveRNGPool.Put(rng)
	for i := int64(-100); i < 100; i++ {
		state := battleRuntimeState{Seed: i * 3571, Wave: int(i + 101), PassiveRunID: i + 101}
		for _, stage := range []string{"participation", "enemy:enemy:1:item:iron_ingot"} {
			legacy := legacyPassiveRandom(state, 41, i+101, "cabinet:v3", stage)
			rng.Seed(battlePassiveSeed(state, 41, i+101, "cabinet:v3", stage))
			if rng.Int63n(10000) != legacy.Int63n(10000) || rng.Float64() != legacy.Float64() || rng.Int63n(31) != legacy.Int63n(31) {
				t.Fatal("optimization changed an existing reward stream")
			}
		}
	}
}

func BenchmarkPassiveBattleRandom(b *testing.B) {
	state := battleRuntimeState{Seed: 57283, Wave: 71, PassiveRunID: 35297}
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = legacyPassiveRandom(state, 41, 7, "cabinet", "participation").Int63n(10000)
		}
	})
	b.Run("pooled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rng := battlePassiveRNGPool.Get().(*rand.Rand)
			rng.Seed(battlePassiveSeed(state, 41, 7, "cabinet", "participation"))
			_ = rng.Int63n(10000)
			battlePassiveRNGPool.Put(rng)
		}
	})
}

func TestPassiveBattleLootIsAdditionalFrozenAndDeterministic(t *testing.T) {
	s := &Server{}
	s.deps.Config = config.Config{GlobalSettings: config.DefaultGlobalSettings(), SharedGlobalSettings: config.DefaultSharedGlobalSettings()}
	s.deps.GameData.Enemies = []map[string]any{{"id": "enemy", "exp": 10, "drops": map[string]any{"wood": map[string]any{"range": map[string]any{"min": 2, "max": 2}, "percent": 1}}}}
	at := time.Unix(1000, 0)
	state := battleRuntimeState{Seed: 5, Wave: 1, PassiveRunID: 99, StartedAt: at.UnixMilli(), Players: []battleUnitState{
		{UnitID: "player:1", UserID: 1, IsPlayerUnit: true, RewardEligible: true, Stats: map[string]float64{}},
		{UnitID: "kitty:7", UserID: 1, UnitType: "kitty", KittyID: 7, Name: "Mochi", RewardEligible: true},
		{UnitID: "summon:8", UserID: 1, UnitType: "kitty", KittyID: 8, RewardEligible: true, SummonSourceUnitID: "kitty:7"},
		{UnitID: "kitty:9", UserID: 1, UnitType: "kitty", KittyID: 9, RewardEligible: false},
	}, Enemies: []battleUnitState{{UnitID: "enemy:1", EnemyID: "enemy", RewardEligible: true, Alive: false}}}
	baseline := s.resolveBattleWaveSettlement(state, battleWorldConfig{}, at.Add(time.Minute))
	source := operationeffect.PassiveSnapshot{Version: 1, Sources: []operationeffect.PassiveSource{{ID: "cabinet", Version: "v1", Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindExtraLoot, Domain: operationeffect.DomainBattleReward, RateBPS: 10000}}}}}
	state.PassiveSources = map[int64]operationeffect.PassiveSnapshot{1: source}
	got := s.resolveBattleWaveSettlement(state, battleWorldConfig{}, at.Add(time.Minute))
	if got.Err != nil {
		t.Fatal(got.Err)
	}
	if got.Rewards[1].ItemRewards["wood"] != baseline.Rewards[1].ItemRewards["wood"]+2 {
		t.Fatalf("expected one extra roll: %v baseline %v", got.Rewards, baseline.Rewards)
	}
	if !reflect.DeepEqual(got.Rewards[1].AttrExp, baseline.Rewards[1].AttrExp) || !reflect.DeepEqual(got.Rewards[1].KittyAttrExp, baseline.Rewards[1].KittyAttrExp) {
		t.Fatal("original experience changed")
	}
	a, _ := json.Marshal(got)
	again := s.resolveBattleWaveSettlement(state, battleWorldConfig{}, at.Add(time.Minute))
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("recompilation rerolled passive rewards")
	}
	rows := got.ByUser[1]["kittyExtraLoot"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatal("summon/ineligible unit received roll")
	}
	contribution := got.Rewards[1].Contributions["cabinet"]
	if contribution.Items["wood"] != 2 || contribution.Opportunities != 1 || contribution.Triggered != 1 || contribution.Empty != 0 {
		t.Fatalf("wrong attribution: %+v", contribution)
	}
	decoded := decodeBattleRewardPlan(map[string]any{"rewardPlan": encodeBattleRewardPlan(got.Rewards)})
	encodedContribution, _ := json.Marshal(got.Rewards[1].Contributions)
	decodedContribution, _ := json.Marshal(decoded[1].Contributions)
	if string(encodedContribution) != string(decodedContribution) {
		t.Fatal("checkpoint lost attribution")
	}
	if err := mergeBattleWaveRewards(decoded, got.Rewards); err != nil {
		t.Fatal(err)
	}
	if decoded[1].Contributions["cabinet"].Items["wood"] != 4 || decoded[1].Contributions["cabinet"].Opportunities != 2 {
		t.Fatal("wave merge lost attribution")
	}
	state.IsFogMode = true
	fog := s.resolveBattleWaveSettlement(state, battleWorldConfig{}, at.Add(time.Minute))
	if _, exists := fog.ByUser[1]["kittyExtraLoot"]; exists {
		t.Fatal("fog received passive roll")
	}
}

func TestPassiveBattleRandomStreamIsIndependent(t *testing.T) {
	state := battleRuntimeState{Seed: 101, Wave: 2, PassiveRunID: 17}
	a := battlePassiveRandom(state, 1, 7, "cabinet", "participation").Int63()
	state.randFloat64()
	state.randInt(100)
	if a != battlePassiveRandom(state, 1, 7, "cabinet", "participation").Int63() {
		t.Fatal("combat draws perturb furniture stream")
	}
	for _, kitty := range []int64{8, 9} {
		if a == battlePassiveRandom(state, 1, kitty, "cabinet", "participation").Int63() {
			t.Fatal("cat streams shared")
		}
	}
}
