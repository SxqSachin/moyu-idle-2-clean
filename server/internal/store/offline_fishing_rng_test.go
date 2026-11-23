package store

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestOfflineFishingRNGExactAndResumable(t *testing.T) {
	for _, seed := range []int64{0, 1, -1, 42, math.MaxInt64, math.MinInt64} {
		old := fishingExecutorRuntime{Seed: seed, EventSeq: math.MaxInt64 - 300}
		batch := old
		batch.batchRNG = rand.New(rand.NewSource(1))
		for phase := 0; phase < 1000; phase++ {
			a, b := fishingRuntimeRNG(&old), fishingRuntimeRNG(&batch)
			for draw := 0; draw < phase%23; draw++ {
				if a.Float64() != b.Float64() || a.Int63() != b.Int63() || a.Intn(177) != b.Intn(177) {
					t.Fatalf("seed=%d phase=%d draw=%d changed", seed, phase, draw)
				}
			}
			aa, bb := make([]byte, phase%11), make([]byte, phase%11)
			_, _ = a.Read(aa)
			_, _ = b.Read(bb)
			if !reflect.DeepEqual(aa, bb) || old.EventSeq != batch.EventSeq {
				t.Fatal("cached read state or phase sequence changed")
			}
			if phase%37 == 0 {
				raw, err := json.Marshal(batch)
				if err != nil {
					t.Fatal(err)
				}
				var resumed fishingExecutorRuntime
				if err = json.Unmarshal(raw, &resumed); err != nil || resumed.batchRNG != nil {
					t.Fatal("scratch generator leaked into checkpoint", err)
				}
				batch = resumed
				batch.batchRNG = rand.New(rand.NewSource(1))
			}
		}
	}
}

func BenchmarkOfflineFishingRNG(b *testing.B) {
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprintf("cached=%t", cached), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				runtime := fishingExecutorRuntime{Seed: 42}
				if cached {
					runtime.batchRNG = rand.New(rand.NewSource(1))
				}
				for range 2000 {
					_ = fishingRuntimeRNG(&runtime).Float64()
				}
			}
		})
	}
}
