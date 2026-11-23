package operationeffect

import (
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
)

// DeterministicSeed owns a separate random namespace from every base business
// roll. Adding an operation effect therefore cannot perturb existing outcomes.
func DeterministicSeed(operationKey string, grantID int64, kind string, slot string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("operation-effect-v1"))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.TrimSpace(operationKey)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.FormatInt(grantID, 10)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.TrimSpace(kind)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.TrimSpace(slot)))
	seed := int64(h.Sum64())
	if seed == 0 {
		return 1
	}
	return seed
}

func Roll(operationKey string, grantID int64, kind string, slot string) float64 {
	return rand.New(rand.NewSource(DeterministicSeed(operationKey, grantID, kind, slot))).Float64()
}
