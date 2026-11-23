// Package actionsettlement contains deterministic, database-free settlement primitives.
package actionsettlement

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/bits"
	"math/rand/v2"

	"gonum.org/v1/gonum/stat/distuv"
)

// Changing the block size, sampler or stream derivation requires a new version.
const RandomVersion = "binomial-subset-v1"
const BlockSize = 256

type blockKey struct {
	stream      string
	index       int64
	probability uint64
}

// Random is transaction-local. A block first samples its binomial hit count,
// then a uniform subset of that size (Floyd's algorithm). This is an iid
// Bernoulli sequence, while counts AND positions survive arbitrary chunking.
// Sampling the smaller complement bounds the work even near probability one.
type Random struct {
	seed   [32]byte
	blocks map[blockKey][BlockSize / 64]uint64
}

func NewRandom(seed [32]byte) *Random {
	return &Random{seed: seed, blocks: make(map[blockKey][BlockSize / 64]uint64)}
}

func (r *Random) Source(stream string, ordinal int64) *rand.ChaCha8 {
	h := sha256.New()
	h.Write(r.seed[:])
	h.Write([]byte(RandomVersion))
	var number [8]byte
	binary.LittleEndian.PutUint64(number[:], uint64(len(stream)))
	h.Write(number[:])
	h.Write([]byte(stream))
	binary.LittleEndian.PutUint64(number[:], uint64(ordinal))
	h.Write(number[:])
	var seed [32]byte
	copy(seed[:], h.Sum(nil))
	return rand.NewChaCha8(seed)
}

func (r *Random) block(stream string, index int64, p float64) [BlockSize / 64]uint64 {
	key := blockKey{stream, index, math.Float64bits(p)}
	if block, ok := r.blocks[key]; ok {
		return block
	}
	// Probabilities are part of the stream, not merely part of the cache key.
	var probability [8]byte
	binary.LittleEndian.PutUint64(probability[:], key.probability)
	source := r.Source("proc:"+string(probability[:])+stream, index)
	rng := rand.New(source)
	count := int(distuv.Binomial{N: BlockSize, P: p, Src: source}.Rand())
	complement := count > BlockSize/2
	if complement {
		count = BlockSize - count
	}
	var block [BlockSize / 64]uint64
	for j := BlockSize - count; j < BlockSize; j++ {
		position := rng.IntN(j + 1)
		if block[position/64]&(uint64(1)<<(position%64)) != 0 {
			position = j
		}
		block[position/64] |= uint64(1) << (position % 64)
	}
	if complement {
		for i := range block {
			block[i] = ^block[i]
		}
	}
	r.blocks[key] = block
	return block
}

// Hit uses zero-based action ordinals. Invalid probabilities are rejected by
// the rule compiler; this guard also prevents an invalid sampler from hanging.
func (r *Random) Hit(stream string, ordinal int64, p float64) bool {
	if ordinal < 0 || math.IsNaN(p) || p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	block := r.block(stream, ordinal/BlockSize, p)
	position := ordinal % BlockSize
	return block[position/64]&(uint64(1)<<(position%64)) != 0
}

func (r *Random) Count(stream string, first int64, count int, p float64) int {
	if first < 0 || count <= 0 || math.IsNaN(p) || p <= 0 {
		return 0
	}
	if p >= 1 {
		return count
	}
	hits := 0
	for count > 0 {
		block := r.block(stream, first/BlockSize, p)
		position := int(first % BlockSize)
		n := min(count, BlockSize-position)
		for offset, left := position, n; left > 0; {
			width := min(left, 64-offset%64)
			mask := ^uint64(0) >> (64 - width)
			hits += bits.OnesCount64((block[offset/64] >> (offset % 64)) & mask)
			offset += width
			left -= width
		}
		first += int64(n)
		count -= n
	}
	return hits
}
