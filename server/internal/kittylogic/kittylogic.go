package kittylogic

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

type RewardResult struct {
	PoolID    string
	Items     []store.KittyRewardGrantSpec
	KittyExp  int64
	BondExp   int64
	MoodDelta float64
}

func RollRewardPool(poolID string, pool config.KittyRewardPool, kittyLevel int, rng *rand.Rand) RewardResult {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if pool.MinKittyLevel > 0 && kittyLevel < pool.MinKittyLevel {
		return RewardResult{PoolID: poolID}
	}
	result := RewardResult{
		PoolID:    poolID,
		MoodDelta: pool.MoodDelta,
	}
	if pool.KittyExp != nil {
		result.KittyExp = rollRange(*pool.KittyExp, rng)
	}
	if pool.BondExp != nil {
		result.BondExp = rollRange(*pool.BondExp, rng)
	}
	itemTotals := map[string]int64{}
	for _, roll := range pool.Rolls {
		if rng.Float64() > roll.Chance {
			continue
		}
		switch roll.Mode {
		case "weightedOne":
			if entry, ok := pickWeightedEntry(roll.Entries, rng); ok {
				qty := rollEntryQuantity(entry, rng)
				if qty > 0 {
					itemTotals[strings.TrimSpace(entry.ItemID)] += qty
				}
			}
		case "independent":
			for _, entry := range roll.Entries {
				if rng.Float64() <= entry.Chance {
					qty := rollEntryQuantity(entry, rng)
					if qty > 0 {
						itemTotals[strings.TrimSpace(entry.ItemID)] += qty
					}
				}
			}
		}
	}
	keys := make([]string, 0, len(itemTotals))
	for itemID := range itemTotals {
		keys = append(keys, itemID)
	}
	sort.Strings(keys)
	for _, itemID := range keys {
		if itemID == "" || itemTotals[itemID] <= 0 {
			continue
		}
		result.Items = append(result.Items, store.KittyRewardGrantSpec{ItemID: itemID, Quantity: itemTotals[itemID]})
	}
	return result
}

func ResolveJournalPool(settings config.KittySettings, contentKey string, category string) (string, config.KittyRewardPool, bool) {
	if !settings.JournalRewards.Enabled {
		return "", config.KittyRewardPool{}, false
	}
	if poolID := strings.TrimSpace(settings.JournalRewards.ContentPoolMap[strings.TrimSpace(contentKey)]); poolID != "" {
		if pool, ok := settings.JournalRewards.Pools[poolID]; ok {
			return poolID, pool, true
		}
	}
	if poolID := strings.TrimSpace(settings.JournalRewards.CategoryPoolMap[strings.TrimSpace(category)]); poolID != "" {
		if pool, ok := settings.JournalRewards.Pools[poolID]; ok {
			return poolID, pool, true
		}
	}
	return "", config.KittyRewardPool{}, false
}

func RewardPayload(result RewardResult) map[string]any {
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, map[string]any{"itemId": item.ItemID, "quantity": item.Quantity})
	}
	return map[string]any{
		"items":     items,
		"kittyExp":  result.KittyExp,
		"bondExp":   result.BondExp,
		"moodDelta": result.MoodDelta,
	}
}

func SanitizeRewardPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return payload
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		switch strings.TrimSpace(key) {
		case "poolId", "pool", "rolls", "entries", "chance", "probability":
			continue
		default:
			out[key] = value
		}
	}
	return out
}

func VisibleCommissions(settings config.KittySettings, userID int64, kitty store.PlayerKitty, now time.Time) []config.KittyCommissionItem {
	if !settings.Commissions.Enabled || settings.Commissions.VisibleCount <= 0 {
		return nil
	}
	itemsByID := map[string]config.KittyCommissionItem{}
	for _, item := range settings.Commissions.Items {
		itemsByID[strings.TrimSpace(item.ID)] = item
	}
	seed := StableSeed(settings.Version, userID, kitty.ID, PeriodKey(now, settings.Commissions.RefreshMode))
	rng := rand.New(rand.NewSource(int64(seed)))
	selected := map[string]bool{}
	out := make([]config.KittyCommissionItem, 0, settings.Commissions.VisibleCount)
	pools := append([]config.KittyCommissionPool(nil), settings.Commissions.Pools...)
	sort.SliceStable(pools, func(i, j int) bool { return pools[i].ID < pools[j].ID })
	for len(out) < settings.Commissions.VisibleCount {
		before := len(out)
		for _, pool := range pools {
			if len(out) >= settings.Commissions.VisibleCount {
				break
			}
			if pool.MinKittyLevel > 0 && kitty.Level < pool.MinKittyLevel {
				continue
			}
			entry, ok := pickCommissionEntry(pool.Entries, selected, rng)
			if !ok {
				continue
			}
			item, ok := itemsByID[strings.TrimSpace(entry.CommissionID)]
			if !ok || (item.MinKittyLevel > 0 && kitty.Level < item.MinKittyLevel) {
				selected[strings.TrimSpace(entry.CommissionID)] = true
				continue
			}
			selected[item.ID] = true
			out = append(out, item)
		}
		if len(out) == before {
			break
		}
	}
	return out
}

func PeriodKey(now time.Time, refreshMode string) string {
	if refreshMode == "" || refreshMode == "daily" {
		return now.Format("2006-01-02")
	}
	return now.Format("2006-01-02")
}

func StableSeed(version string, userID int64, kittyID int64, period string) uint64 {
	h := sha256.Sum256([]byte(strings.TrimSpace(version) + "|" + int64String(userID) + "|" + int64String(kittyID) + "|" + strings.TrimSpace(period)))
	return binary.LittleEndian.Uint64(h[:8])
}

func rollRange(r config.KittyIntRange, rng *rand.Rand) int64 {
	if r.Max <= r.Min {
		return r.Min
	}
	return r.Min + rng.Int63n(r.Max-r.Min+1)
}

func rollEntryQuantity(entry config.KittyRewardEntry, rng *rand.Rand) int64 {
	min := entry.Min
	max := entry.Max
	if max <= min {
		return min
	}
	return min + rng.Int63n(max-min+1)
}

func pickWeightedEntry(entries []config.KittyRewardEntry, rng *rand.Rand) (config.KittyRewardEntry, bool) {
	total := 0.0
	for _, entry := range entries {
		if entry.Weight > 0 {
			total += entry.Weight
		}
	}
	if total <= 0 {
		return config.KittyRewardEntry{}, false
	}
	roll := rng.Float64() * total
	for _, entry := range entries {
		if entry.Weight <= 0 {
			continue
		}
		roll -= entry.Weight
		if roll <= 0 {
			return entry, true
		}
	}
	return entries[len(entries)-1], true
}

func pickCommissionEntry(entries []config.KittyCommissionPoolEntry, selected map[string]bool, rng *rand.Rand) (config.KittyCommissionPoolEntry, bool) {
	total := 0.0
	for _, entry := range entries {
		if selected[strings.TrimSpace(entry.CommissionID)] || entry.Weight <= 0 {
			continue
		}
		total += entry.Weight
	}
	if total <= 0 {
		return config.KittyCommissionPoolEntry{}, false
	}
	roll := rng.Float64() * total
	for _, entry := range entries {
		if selected[strings.TrimSpace(entry.CommissionID)] || entry.Weight <= 0 {
			continue
		}
		roll -= entry.Weight
		if roll <= 0 {
			return entry, true
		}
	}
	return config.KittyCommissionPoolEntry{}, false
}

func int64String(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
