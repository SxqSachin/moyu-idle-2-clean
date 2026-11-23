package actionsettlement

import (
	"encoding/binary"
	"math"
	"testing"
)

func testSeed(n uint64) [32]byte {
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:], n)
	return seed
}

func TestRandomPartitionsAndSingleOperations(t *testing.T) {
	for _, p := range []float64{0, 1e-9, 0.01, 0.37, 0.5, 0.99, 1} {
		for _, first := range []int64{0, 1, 63, 255, 256, 511, 50001} {
			r := NewRandom(testSeed(9281))
			whole := r.Count("food", first, 5000, p)
			split, single := 0, 0
			for i := 0; i < 5000; {
				n := min(5000-i, 1+(i*17+23)%431)
				// Recreating the object models transaction/worker restart.
				split += NewRandom(testSeed(9281)).Count("food", first+int64(i), n, p)
				i += n
			}
			for i := 0; i < 5000; i++ {
				if r.Hit("food", first+int64(i), p) {
					single++
				}
			}
			if whole != split || split != single {
				t.Fatalf("p=%g first=%d whole=%d split=%d single=%d", p, first, whole, split, single)
			}
		}
	}
}

func TestRandomBinomialDistributionAndIndependence(t *testing.T) {
	const samples = 20000
	for _, p := range []float64{0.001, 0.04, 0.5, 0.98} {
		var sum, squares, cross, sumA, sumB float64
		for i := 0; i < samples; i++ {
			r := NewRandom(testSeed(uint64(i + 1)))
			n := float64(r.Count("base", 0, BlockSize, p))
			sum += n
			squares += n * n
			a, b := 0.0, 0.0
			if r.Hit("base", 3, p) {
				a = 1
			}
			if r.Hit("base", 181, p) {
				b = 1
			}
			cross += a * b
			sumA += a
			sumB += b
		}
		mean := sum / samples
		variance := squares/samples - mean*mean
		wantMean := BlockSize * p
		wantVariance := BlockSize * p * (1 - p)
		if math.Abs(mean-wantMean) > 7*math.Sqrt(wantVariance/samples) {
			t.Fatalf("p=%g mean=%g want=%g", p, mean, wantMean)
		}
		if math.Abs(variance-wantVariance) > max(0.035, 0.065*wantVariance) {
			t.Fatalf("p=%g variance=%g want=%g", p, variance, wantVariance)
		}
		if math.Abs(cross/samples-(sumA/samples)*(sumB/samples)) > 0.007 {
			t.Fatalf("p=%g correlated positions", p)
		}
	}
}

func TestRandomStreamsAndEdges(t *testing.T) {
	r := NewRandom(testSeed(22))
	if r.Hit("a", -1, 1) || r.Hit("a", 0, math.NaN()) || r.Count("a", 0, 0, 0.5) != 0 {
		t.Fatal("invalid bounds")
	}
	if r.Count("a", 71, 50000, 0) != 0 || r.Count("a", 71, 50000, 1) != 50000 {
		t.Fatal("endpoint probability")
	}
	if r.block("a", 0, 0.5) == r.block("b", 0, 0.5) || r.block("a", 0, 0.5) == r.block("a", 1, 0.5) {
		t.Fatal("streams reused")
	}
}

func BenchmarkStatisticalCount50000(b *testing.B) {
	for b.Loop() {
		NewRandom(testSeed(123)).Count("food", 0, 50000, 0.04)
	}
}
