package store

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
)

func (s *Store) preparePondFeedDemand(fish *FishInstanceData, instanceID, userID int64, at time.Time, stepHours float64, species *catalog.FishingSpecies, profile *catalog.FishingPondSpecies) (maintenance, targetWeight, desiredGain, needed float64) {
	s.initializeFishingPondFish(fish, userID, instanceID, at, profile)
	s.clampFishingPondFishSize(fish, species)
	bioDays := fishingPondBioDaysPerRealDay * stepHours / 24
	fish.BioAgeFraction += bioDays
	wholeDays := int(math.Floor(fish.BioAgeFraction))
	if wholeDays > 0 {
		fish.AgeDays += wholeDays
		fish.BioAgeFraction -= float64(wholeDays)
	}
	maintenance = math.Max(4, 2.4*math.Pow(math.Max(0.05, fish.WeightKG), 0.75)) * stepHours / 24
	targetLength := math.Max(fish.LengthCM, fishingPondTargetLength(*fish, species, profile))
	targetWeight = species.ShapeFactor * math.Pow(targetLength, 3) / 100_000 * math.Min(1.12, math.Max(0.82, fish.Condition))
	desiredGain = math.Max(0, targetWeight-fish.WeightKG) * clampFloat(0.32*stepHours, 0, 1)
	needed = maintenance + desiredGain*fishingPondNutritionPerKG
	return
}

func pondFeedRole(feed *catalog.PondFeedConfig) string {
	if feed.Role != "" {
		return feed.Role
	}
	if len(feed.Effects) > 0 {
		return "functional"
	}
	return "staple"
}

func pondFeedDoseLimit(feed *catalog.PondFeedConfig) float64 {
	if pondFeedRole(feed) != "functional" {
		return 0
	}
	if feed.MaxPortionsPerFishHour > 0 {
		return math.Min(fishingPondMaxEffectPortionsHour, feed.MaxPortionsPerFishHour)
	}
	return fishingPondMaxEffectPortionsHour
}

// A compound feed pays all costs and grants all benefits at the same dose.
func pondFeedPayableDose(fish FishInstanceData, feed *catalog.PondFeedConfig, dose float64) float64 {
	for _, effect := range feed.Effects {
		if effect.Mode != "flat" || effect.ValuePerPortion >= 0 {
			continue
		}
		current := fish.Health
		if effect.Stat == "satiety" {
			current = fish.Satiety
		}
		floor := 0.0
		if effect.Floor != nil {
			floor = *effect.Floor
		}
		dose = math.Min(dose, math.Max(0, current-floor)/-effect.ValuePerPortion)
	}
	return dose
}

type pondFeedCandidate struct {
	id         string
	feed       *catalog.PondFeedConfig
	efficiency float64
	preferred  bool
}

func (s *Store) pondFeedCandidates(fish FishInstanceData, profile *catalog.FishingPondSpecies, stock map[string]float64, paused map[string]bool, efficiencyGene float64) []pondFeedCandidate {
	var out []pondFeedCandidate
	if profile == nil {
		return out
	}
	for id, available := range stock {
		item := s.gameData.ItemByID(id)
		if available <= 0 || paused[id] || item == nil || item.Disabled || item.PondFeed == nil || item.PondFeed.Delivery == "targeted" || item.PondFeed.Nutrition <= 0 || !pondFeedConditionsMatch(fish, profile, item.PondFeed) {
			continue
		}
		feed := item.PondFeed
		if pondFeedRole(feed) == "functional" {
			if species := s.gameData.FishingSpeciesByID(fish.SpeciesID); species != nil {
				useful := false
				for _, effect := range feed.Effects {
					if effect.ValuePerPortion <= 0 {
						continue
					}
					after := fish
					s.applyFishingPondFeedEffect(&after, species, profile, effect, pondFeedDoseLimit(feed))
					useful = useful || after.LengthCM > fish.LengthCM || after.WeightKG > fish.WeightKG || after.Health > fish.Health || after.Satiety > fish.Satiety
				}
				if !useful {
					continue
				}
			}
		}
		efficiency := 0.35
		preferred := feed.FeedClass == profile.DietClass
		if preferred {
			efficiency = 1
		} else if pondContainsString(profile.SecondaryFeedClasses, feed.FeedClass) {
			efficiency = 0.7
		}
		efficiency *= 1 + 0.2*clampFloat(efficiencyGene, -1, 1)
		out = append(out, pondFeedCandidate{id, feed, efficiency, preferred})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if pondFeedRole(a.feed) != pondFeedRole(b.feed) {
			return pondFeedRole(a.feed) == "functional"
		}
		if a.feed.ConsumePriority != b.feed.ConsumePriority {
			return a.feed.ConsumePriority > b.feed.ConsumePriority
		}
		if a.efficiency != b.efficiency {
			return a.efficiency > b.efficiency
		}
		return a.id < b.id
	})
	return out
}

type pondFeedDemand struct {
	fish       FishInstanceData
	needed     float64
	candidates []pondFeedCandidate
}

// Each round distributes one candidate per fish proportionally before anyone
// advances to fallback feed. Inventory iteration order cannot select a winner.
func allocatePondFeed(demands []pondFeedDemand, stock map[string]float64, hours float64) []fishingPondNutritionResult {
	out := make([]fishingPondNutritionResult, len(demands))
	functionalUsed := make([]bool, len(demands))
	type request struct {
		index     int
		candidate pondFeedCandidate
		raw       float64
	}
	for _, role := range []string{"functional", "staple"} {
		positions := make([]int, len(demands))
		for {
			requests := map[string][]request{}
			for i, demand := range demands {
				remaining := math.Max(0, demand.needed-out[i].Effective)
				if remaining <= 1e-12 {
					continue
				}
				for positions[i] < len(demand.candidates) {
					candidate := demand.candidates[positions[i]]
					positions[i]++
					if pondFeedRole(candidate.feed) != role {
						continue
					}
					functional := pondFeedRole(candidate.feed) == "functional"
					if stock[candidate.id] <= 1e-12 || functional && functionalUsed[i] {
						continue
					}
					raw := remaining / candidate.efficiency
					if functional {
						dose := pondFeedPayableDose(demand.fish, candidate.feed, pondFeedDoseLimit(candidate.feed)*hours)
						raw = math.Min(raw, dose*candidate.feed.Nutrition)
					}
					if raw <= 1e-12 {
						continue
					}
					requests[candidate.id] = append(requests[candidate.id], request{i, candidate, raw})
					break
				}
			}
			if len(requests) == 0 {
				break
			}
			for id, group := range requests {
				total := 0.0
				for _, req := range group {
					total += req.raw
				}
				ratio := math.Min(1, stock[id]/total)
				used := 0.0
				for _, req := range group {
					raw := req.raw * ratio
					result := &out[req.index]
					result.Effective += raw * req.candidate.efficiency
					result.Preferred = result.Preferred || req.candidate.preferred
					result.Doses = append(result.Doses, fishingPondFeedDose{ItemID: id, Feed: req.candidate.feed, Portions: raw / req.candidate.feed.Nutrition})
					if pondFeedRole(req.candidate.feed) == "functional" {
						functionalUsed[req.index] = true
					}
					used += raw
				}
				stock[id] = math.Max(0, stock[id]-used)
			}
		}
	}
	return out
}

func (s *Store) settleFishingPondFeedingStep(rows []fishingPondFishRow, stock map[string]float64, paused map[string]bool, userID int64, at time.Time, hours float64) {
	if hours <= 0 {
		return
	}
	demands := make([]pondFeedDemand, len(rows))
	for i, row := range rows {
		fish := row.Fish
		profile, species := s.gameData.FishingPondSpeciesByID(fish.SpeciesID), s.gameData.FishingSpeciesByID(fish.SpeciesID)
		if normalizePondLifeState(fish.LifeState) != "alive" || profile == nil || species == nil {
			continue
		}
		_, _, _, needed := s.preparePondFeedDemand(&fish, row.InstanceID, userID, at, hours, species, profile)
		demands[i] = pondFeedDemand{fish, needed, s.pondFeedCandidates(fish, profile, stock, paused, fish.Genes.FeedEfficiency)}
	}
	results := allocatePondFeed(demands, stock, hours)
	for i := range rows {
		s.settleFishingPondFishWithFeed(&rows[i], userID, at, hours, results[i])
	}
}

type pondFeedStockEntry struct {
	Portions float64 `json:"portions"`
	Paused   bool    `json:"paused,omitempty"`
}
type pondFeedStockDocument struct {
	Version int                           `json:"version"`
	Feeds   map[string]pondFeedStockEntry `json:"feeds"`
}

func (s *Store) decodePondFeedStock(raw []byte) (map[string]float64, map[string]bool, error) {
	stock, paused := map[string]float64{}, map[string]bool{}
	if len(raw) == 0 {
		return stock, paused, nil
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) != nil || probe == nil {
		return nil, nil, ErrFishingPondFeedInvalid
	}
	if _, versioned := probe["version"]; versioned {
		var document pondFeedStockDocument
		if json.Unmarshal(raw, &document) != nil || document.Version != 2 || document.Feeds == nil {
			return nil, nil, ErrFishingPondFeedInvalid
		}
		for id, entry := range document.Feeds {
			item := s.gameData.ItemByID(id)
			if item == nil || item.PondFeed == nil || item.PondFeed.Nutrition <= 0 || entry.Portions < 0 || math.IsInf(entry.Portions, 0) || math.IsNaN(entry.Portions) {
				return nil, nil, ErrFishingPondFeedInvalid
			}
			stock[id], paused[id] = entry.Portions*item.PondFeed.Nutrition, entry.Paused
		}
	} else if json.Unmarshal(raw, &stock) != nil {
		return nil, nil, ErrFishingPondFeedInvalid
	}
	for _, value := range stock {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil, ErrFishingPondFeedInvalid
		}
	}
	return stock, paused, nil
}

func (s *Store) encodePondFeedStock(stock map[string]float64, paused map[string]bool) ([]byte, error) {
	document := pondFeedStockDocument{Version: 2, Feeds: map[string]pondFeedStockEntry{}}
	for id, nutrition := range stock {
		item := s.gameData.ItemByID(id)
		if item == nil || item.PondFeed == nil || item.PondFeed.Nutrition <= 0 {
			return nil, ErrFishingPondFeedInvalid
		}
		if nutrition < 0 || math.IsInf(nutrition, 0) || math.IsNaN(nutrition) {
			return nil, ErrFishingPondFeedInvalid
		}
		if nutrition > 0 || paused[id] {
			document.Feeds[id] = pondFeedStockEntry{nutrition / item.PondFeed.Nutrition, paused[id]}
		}
	}
	return json.Marshal(document)
}

func (s *Store) ManageFishingPondFeed(ctx context.Context, userID int64, itemID, operation string, quantity int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		state, err := s.lockFishingPondStateTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		item := s.gameData.ItemByID(itemID)
		if item == nil || item.PondFeed == nil || item.PondFeed.Nutrition <= 0 {
			return ErrFishingPondFeedInvalid
		}
		available := state.FeedStock[itemID] / item.PondFeed.Nutrition
		switch operation {
		case "pause", "resume":
			if available <= 0 {
				return ErrFishingPondFeedInvalid
			}
			state.Paused[itemID] = operation == "pause"
		case "withdraw":
			if quantity <= 0 || quantity > 10000 || float64(quantity) > available {
				return ErrFishingPondFeedInvalid
			}
			state.FeedStock[itemID] = math.Max(0, state.FeedStock[itemID]-float64(quantity)*item.PondFeed.Nutrition)
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, quantity, pondLedgerMeta("fishing_pond", "inventory", InventoryAuditDirectionGain, "pond_feed_withdraw")); err != nil {
				return err
			}
		case "discard_remainder":
			if available <= 0 || available >= 1 {
				return ErrFishingPondFeedInvalid
			}
			delete(state.FeedStock, itemID)
			delete(state.Paused, itemID)
		default:
			return ErrFishingPondFeedInvalid
		}
		if err := s.updateFishingPondFeedStockTx(ctx, tx, userID, state.FeedStock, now, state.Paused); err != nil {
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "feed_managed", nil, nil, map[string]any{"itemId": itemID, "operation": operation, "quantity": quantity})
	})
}
