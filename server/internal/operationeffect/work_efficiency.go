package operationeffect

import (
	"sort"
	"time"
)

// Explicit capability boundary: no fishing, combat, enhancement or building.
func WorkEfficiencyMainType(main string) bool {
	switch main {
	case "collecting", "mining", "farming_animal", "crafting", "sewing", "forging", "alchemy", "cooking":
		return true
	}
	return false
}

// Matching rules within one source use the strongest value. Independent
// sources add, so overlapping selectors never double-count the same furniture.
func (s PassiveSnapshot) EfficiencyBPS() int64 {
	var total int64
	for _, source := range s.Sources {
		var best int64
		for _, rule := range source.Rules {
			if rule.Kind == KindActionEfficiency && rule.RateBPS > best {
				best = rule.RateBPS
			}
		}
		total += best
	}
	return total
}

func (s *PassiveSession) NextEfficiencyBoundary(op Operation) time.Time {
	var next time.Time
	if s == nil || op.ActorKind != ActorKitty || !WorkEfficiencyMainType(op.MainType) {
		return next
	}
	s.ensureWindowIndex()
	times := s.efficiencyBoundaries[op.MainType]
	index := sort.Search(len(times), func(i int) bool { return times[i].After(op.StartedAt) })
	if index < len(times) {
		next = times[index]
	}
	return next
}
