package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

const (
	FishingTripActionID              = "fishing.trip"
	FishingRulesetVersion            = "fishing-v2"
	fishingLegacyRulesetVersion      = "fishing-v1"
	FishingCatchHistoryDays          = 90
	FishingSaleBatchMax              = 100
	FishingDefaultPageLimit          = 24
	FishingMaximumPageLimit          = 100
	FishingTripHistoryLimit          = 20
	FishingMaximumLoadouts           = 16
	fishingCatchMutationGroupLimit   = 24
	FishingStarterGrantKey           = "fishing_starter_v1"
	FishingStarterEquipmentRepairKey = "fishing_starter_equipment_v2"
	fishingSaleReferenceLengthRatio  = 0.30
	fishingSaleReferenceAgeDays      = 930.0
	fishingSaleAgeExponent           = 0.06
	fishingSaleLengthExponent        = 0.45
	fishingSaleWeightExponent        = 0.15
	fishingSaleSpecimenMinimum       = 0.55
	fishingSaleSpecimenMaximum       = 1.75
	fishingSaleMarketMinimum         = 0.90
	fishingSaleMarketMaximum         = 1.10
	fishingCulturedSaleFactor        = 0.50
	fishingCulturedValuePerKGCap     = 300.0
)

type FishingSupplyCounts struct {
	MainLine      int64 `json:"mainLine"`
	Leader        int64 `json:"leader"`
	Hook          int64 `json:"hook,omitempty"`
	FloatOrSinker int64 `json:"floatOrSinker,omitempty"`
	Lure          int64 `json:"lure,omitempty"`
	Bait          int64 `json:"bait,omitempty"`
}

type FishingTripSupplies struct {
	Initial   FishingSupplyCounts `json:"initial"`
	Remaining FishingSupplyCounts `json:"remaining"`
	Consumed  FishingSupplyCounts `json:"consumed"`
	Wear      FishingSupplyWear   `json:"wear,omitempty"`
	Returned  bool                `json:"returned,omitempty"`
}

type FishingSupplyWear struct {
	MainLine float64 `json:"mainLine,omitempty"`
	Leader   float64 `json:"leader,omitempty"`
}

type FishingLoadoutComponents struct {
	RodInstanceID       int64  `json:"rodInstanceId"`
	ReelInstanceID      int64  `json:"reelInstanceId"`
	MainLineItemID      string `json:"mainLineItemId"`
	LeaderItemID        string `json:"leaderItemId"`
	HookItemID          string `json:"hookItemId,omitempty"`
	FloatOrSinkerItemID string `json:"floatOrSinkerItemId,omitempty"`
	LureItemID          string `json:"lureItemId,omitempty"`
	BaitItemID          string `json:"baitItemId,omitempty"`
}

type FishingLoadout struct {
	LoadoutID         string                   `json:"loadoutId"`
	Name              string                   `json:"name"`
	Method            string                   `json:"method"`
	Strategy          string                   `json:"strategy"`
	SafetyLoadPercent int                      `json:"safetyLoadPercent"`
	Components        FishingLoadoutComponents `json:"components"`
	UpdatedAt         time.Time                `json:"updatedAt"`
}

type FishingLoadoutInput struct {
	LoadoutID         string                   `json:"loadoutId"`
	Name              string                   `json:"name"`
	Method            string                   `json:"method"`
	Strategy          string                   `json:"strategy"`
	SafetyLoadPercent int                      `json:"safetyLoadPercent"`
	Components        FishingLoadoutComponents `json:"components"`
}

type FishingFrozenComponent struct {
	Slot                     string  `json:"slot"`
	ItemID                   string  `json:"itemId"`
	InstanceID               int64   `json:"instanceId,omitempty"`
	Control                  float64 `json:"control,omitempty"`
	Sensitivity              float64 `json:"sensitivity,omitempty"`
	Casting                  float64 `json:"casting,omitempty"`
	Hooking                  float64 `json:"hooking,omitempty"`
	Efficiency               float64 `json:"efficiency,omitempty"`
	MaxLoadKG                float64 `json:"maxLoadKg,omitempty"`
	WearPerFight             float64 `json:"wearPerFight,omitempty"`
	MutationChanceMultiplier float64 `json:"mutationChanceMultiplier,omitempty"`
}

type FishingLoadoutSnapshot struct {
	LoadoutID                string                   `json:"loadoutId"`
	Name                     string                   `json:"name"`
	Method                   string                   `json:"method"`
	Strategy                 string                   `json:"strategy"`
	SafetyLoadPercent        int                      `json:"safetyLoadPercent"`
	Components               FishingLoadoutComponents `json:"components"`
	FrozenComponents         []FishingFrozenComponent `json:"frozenComponents"`
	Control                  float64                  `json:"control"`
	Sensitivity              float64                  `json:"sensitivity"`
	Efficiency               float64                  `json:"efficiency"`
	Power                    float64                  `json:"power"`
	Casting                  float64                  `json:"casting"`
	Hooking                  float64                  `json:"hooking"`
	MaxLoadKG                float64                  `json:"maxLoadKg"`
	BiteMultiplier           float64                  `json:"biteMultiplier"`
	MutationChanceMultiplier float64                  `json:"mutationChanceMultiplier,omitempty"`
	WaterBiteModifiers       map[string]float64       `json:"waterBiteModifiers,omitempty"`
	SpeciesWeightModifiers   map[string]float64       `json:"speciesWeightModifiers,omitempty"`
	FishingLevel             int64                    `json:"fishingLevel"`
}

type FishingKeepRule struct {
	Mode        string  `json:"mode"`
	MinWeightKG float64 `json:"minWeightKg,omitempty"`
}

type FishingTripEvent struct {
	Sequence    int64     `json:"sequence"`
	Kind        string    `json:"kind"`
	Reason      string    `json:"reason,omitempty"`
	At          time.Time `json:"at"`
	CastNumber  int64     `json:"castNumber,omitempty"`
	SizeClass   string    `json:"sizeClass,omitempty"`
	SpeciesID   string    `json:"speciesId,omitempty"`
	LengthCM    float64   `json:"lengthCm,omitempty"`
	WeightKG    float64   `json:"weightKg,omitempty"`
	Mutations   []string  `json:"mutations,omitempty"`
	Disposition string    `json:"disposition,omitempty"`
}

type FishingTripSummary struct {
	Casts          int64               `json:"casts"`
	Bites          int64               `json:"bites"`
	Landed         int64               `json:"landed"`
	Kept           int64               `json:"kept"`
	Released       int64               `json:"released"`
	Escaped        int64               `json:"escaped"`
	LineBreaks     int64               `json:"lineBreaks"`
	BaitUsed       int64               `json:"baitUsed"`
	ScaleglowFound int64               `json:"scaleglowFound"`
	TotalWeight    float64             `json:"totalWeightKg"`
	FishingExp     float64             `json:"fishingExp"`
	NewSpecies     []string            `json:"newSpecies,omitempty"`
	Records        []FishingRecordFlag `json:"records,omitempty"`
	LastCatch      *FishingCatchView   `json:"lastCatch,omitempty"`
	RecentEvents   []FishingTripEvent  `json:"recentEvents,omitempty"`
	StopReason     string              `json:"stopReason,omitempty"`
}

type FishingRecordFlag struct {
	SpeciesID string `json:"speciesId"`
	Length    bool   `json:"length"`
	Weight    bool   `json:"weight"`
}

type FishingCatchTally struct {
	Landed          int64                       `json:"landed"`
	Kept            int64                       `json:"kept"`
	Released        int64                       `json:"released"`
	TotalWeightKG   float64                     `json:"totalWeightKg"`
	MutatedLanded   int64                       `json:"mutatedLanded,omitempty"`
	MutatedKept     int64                       `json:"mutatedKept,omitempty"`
	MutatedReleased int64                       `json:"mutatedReleased,omitempty"`
	MutationGroups  []FishingCatchMutationTally `json:"mutationGroups,omitempty"`
}

type FishingCatchMutationTally struct {
	Mutations []string `json:"mutations"`
	Landed    int64    `json:"landed"`
	Kept      int64    `json:"kept"`
	Released  int64    `json:"released"`
}

func addFishingCatchTally(tallies map[string]FishingCatchTally, speciesID string, kept bool, weightKG float64, mutations []string) {
	speciesID = strings.TrimSpace(speciesID)
	if tallies == nil || speciesID == "" {
		return
	}
	tally := tallies[speciesID]
	tally.Landed++
	if kept {
		tally.Kept++
	} else {
		tally.Released++
	}
	tally.TotalWeightKG = roundFloat(tally.TotalWeightKG+math.Max(0, weightKG), 3)
	mutationIDs := normalizedFishingCatchMutationIDs(mutations)
	if len(mutationIDs) > 0 {
		tally.MutatedLanded++
		if kept {
			tally.MutatedKept++
		} else {
			tally.MutatedReleased++
		}
		mergeFishingCatchMutationGroup(&tally, FishingCatchMutationTally{
			Mutations: mutationIDs,
			Landed:    1,
			Kept:      boolCount(kept),
			Released:  boolCount(!kept),
		})
	}
	tallies[speciesID] = tally
}

func mergeFishingCatchTallies(target, source map[string]FishingCatchTally) {
	for speciesID, incoming := range source {
		speciesID = strings.TrimSpace(speciesID)
		if target == nil || speciesID == "" || incoming.Landed <= 0 {
			continue
		}
		current := target[speciesID]
		current.Landed += incoming.Landed
		current.Kept += incoming.Kept
		current.Released += incoming.Released
		current.TotalWeightKG = roundFloat(current.TotalWeightKG+incoming.TotalWeightKG, 3)
		current.MutatedLanded += incoming.MutatedLanded
		current.MutatedKept += incoming.MutatedKept
		current.MutatedReleased += incoming.MutatedReleased
		for _, group := range incoming.MutationGroups {
			mergeFishingCatchMutationGroup(&current, group)
		}
		target[speciesID] = current
	}
}

func normalizedFishingCatchMutationIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.ToLower(strings.TrimSpace(value))
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func mergeFishingCatchMutationGroup(tally *FishingCatchTally, incoming FishingCatchMutationTally) {
	if tally == nil || incoming.Landed <= 0 {
		return
	}
	mutationIDs := normalizedFishingCatchMutationIDs(incoming.Mutations)
	if len(mutationIDs) == 0 {
		return
	}
	key := strings.Join(mutationIDs, "\x00")
	for index := range tally.MutationGroups {
		current := &tally.MutationGroups[index]
		if strings.Join(normalizedFishingCatchMutationIDs(current.Mutations), "\x00") != key {
			continue
		}
		current.Mutations = mutationIDs
		current.Landed += incoming.Landed
		current.Kept += incoming.Kept
		current.Released += incoming.Released
		return
	}
	if len(tally.MutationGroups) >= fishingCatchMutationGroupLimit {
		return
	}
	tally.MutationGroups = append(tally.MutationGroups, FishingCatchMutationTally{
		Mutations: append([]string(nil), mutationIDs...),
		Landed:    incoming.Landed,
		Kept:      incoming.Kept,
		Released:  incoming.Released,
	})
}

func boolCount(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

type FishingTripState struct {
	TripID            string              `json:"tripId"`
	QueueID           int64               `json:"queueId"`
	ActorType         string              `json:"actorType"`
	ActorID           int64               `json:"actorId"`
	WaterID           string              `json:"waterId"`
	SpotID            string              `json:"spotId"`
	LoadoutID         string              `json:"loadoutId"`
	Strategy          string              `json:"strategy"`
	SafetyLoadPercent int                 `json:"safetyLoadPercent"`
	Status            string              `json:"status"`
	Phase             string              `json:"phase"`
	PhaseStartedAt    time.Time           `json:"phaseStartedAt"`
	PhaseEndsAt       time.Time           `json:"phaseEndsAt"`
	PlannedEndAt      *time.Time          `json:"plannedEndAt,omitempty"`
	CurrentFish       map[string]any      `json:"currentFish,omitempty"`
	Summary           FishingTripSummary  `json:"summary"`
	Supplies          FishingTripSupplies `json:"supplies"`
	StopReason        string              `json:"stopReason,omitempty"`
	StartedAt         time.Time           `json:"startedAt"`
	EndedAt           *time.Time          `json:"endedAt,omitempty"`
}

type FishingAnglerSummary struct {
	ActorType    string `json:"actorType"`
	ActorID      int64  `json:"actorId"`
	Name         string `json:"name,omitempty"`
	Status       string `json:"status"`
	FishingLevel int64  `json:"fishingLevel"`
}

type FishingStarterGrantResult struct {
	Granted          bool             `json:"granted"`
	EquipmentChanged bool             `json:"equipmentChanged,omitempty"`
	GrantKey         string           `json:"grantKey"`
	LoadoutID        string           `json:"loadoutId"`
	Items            map[string]int64 `json:"items"`
}

type FishingCatchView struct {
	CatchID              int64             `json:"catchId"`
	TripID               string            `json:"tripId"`
	SpeciesID            string            `json:"speciesId"`
	Fish                 FishInstanceData  `json:"fish"`
	Kept                 bool              `json:"kept"`
	InventoryInstanceID  *int64            `json:"inventoryInstanceId,omitempty"`
	SaleEligible         bool              `json:"saleEligible"`
	SaleIneligibleReason string            `json:"saleIneligibleReason,omitempty"`
	SaleCurrentAgeDays   int               `json:"saleCurrentAgeDays,omitempty"`
	SaleRequiredAgeDays  int               `json:"saleRequiredAgeDays,omitempty"`
	CaughtAt             time.Time         `json:"caughtAt"`
	Record               FishingRecordFlag `json:"record"`
}

type FishingCatchPage struct {
	Items      []FishingCatchView `json:"items"`
	NextCursor int64              `json:"nextCursor,omitempty"`
}

type FishingKnowledgeEntry struct {
	SpeciesID     string     `json:"speciesId"`
	Encounters    int64      `json:"encounters"`
	Catches       int64      `json:"catches"`
	Releases      int64      `json:"releases"`
	FirstCaughtAt *time.Time `json:"firstCaughtAt,omitempty"`
	LastCaughtAt  *time.Time `json:"lastCaughtAt,omitempty"`
	BestLengthCM  float64    `json:"bestLengthCm"`
	BestWeightKG  float64    `json:"bestWeightKg"`
}

type FishingLeaderboardRow struct {
	Rank       int64   `json:"rank"`
	UserID     int64   `json:"userId"`
	PlayerName string  `json:"playerName"`
	SpeciesID  string  `json:"speciesId"`
	Value      float64 `json:"value"`
}

type FishingOwnedTackle struct {
	ItemID        string `json:"itemId"`
	Slot          string `json:"slot"`
	Quantity      int64  `json:"quantity"`
	InstanceID    int64  `json:"instanceId,omitempty"`
	EnhanceLevel  int    `json:"enhanceLevel,omitempty"`
	Durability    int    `json:"durability,omitempty"`
	MaxDurability int    `json:"maxDurability,omitempty"`
}

type FishingSpotForecast struct {
	SpotID   string              `json:"spotId"`
	Pressure float64             `json:"pressure"`
	Now      FishingConditions   `json:"now"`
	Next     []FishingConditions `json:"next"`
}

type FishingPriceLine struct {
	InstanceID         int64    `json:"instanceId"`
	SpeciesID          string   `json:"speciesId"`
	AgeDays            int      `json:"ageDays"`
	LengthCM           float64  `json:"lengthCm"`
	WeightKG           float64  `json:"weightKg"`
	Mutations          []string `json:"mutations,omitempty"`
	BaseValue          float64  `json:"baseValue"`
	AgeFactor          float64  `json:"ageFactor"`
	LengthFactor       float64  `json:"lengthFactor"`
	WeightFactor       float64  `json:"weightFactor"`
	QualityFactor      float64  `json:"qualityFactor"`
	SpecimenMultiplier float64  `json:"specimenMultiplier"`
	MutationMultiplier float64  `json:"mutationMultiplier"`
	DailyDemand        float64  `json:"dailyDemand"`
	SupplyFactor       float64  `json:"supplyFactor"`
	CultureFactor      float64  `json:"cultureFactor"`
	MarketMultiplier   float64  `json:"marketMultiplier"`
	FinalGold          int64    `json:"finalGold"`
}

type FishingSaleQuote struct {
	QuoteAt    time.Time          `json:"quoteAt"`
	MarketDate string             `json:"marketDate"`
	Items      []FishingPriceLine `json:"items"`
	TotalGold  int64              `json:"totalGold"`
}

func normalizeFishingMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "float", "bottom", "lure":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeFishingStrategy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "conservative", "balanced", "aggressive":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "balanced"
	}
}

func normalizeFishingKeepRule(rule FishingKeepRule) FishingKeepRule {
	switch strings.ToLower(strings.TrimSpace(rule.Mode)) {
	case "keep_all", "release_all", "record_only", "min_weight":
		rule.Mode = strings.ToLower(strings.TrimSpace(rule.Mode))
	default:
		rule.Mode = "keep_all"
	}
	if rule.MinWeightKG < 0 {
		rule.MinWeightKG = 0
	}
	if rule.MinWeightKG > 500 {
		rule.MinWeightKG = 500
	}
	return rule
}

func fishingShouldKeep(rule FishingKeepRule, fish FishInstanceData, isRecord bool) bool {
	rule = normalizeFishingKeepRule(rule)
	switch rule.Mode {
	case "release_all":
		return false
	case "record_only":
		return isRecord
	case "min_weight":
		return fish.WeightKG >= rule.MinWeightKG
	default:
		return true
	}
}

func (s *Store) FishingCatalog() ([]catalog.FishingWater, []catalog.FishingSpot, []catalog.FishingSpecies, []catalog.FishingPondSpecies, []catalog.FishingTackleDefinition) {
	waters := append([]catalog.FishingWater(nil), s.gameData.FishingWaters...)
	spots := append([]catalog.FishingSpot(nil), s.gameData.FishingSpots...)
	species := append([]catalog.FishingSpecies(nil), s.gameData.FishingSpecies...)
	pondSpecies := append([]catalog.FishingPondSpecies(nil), s.gameData.FishingPondSpecies...)
	tackle := append([]catalog.FishingTackleDefinition(nil), s.gameData.FishingTackle...)
	return waters, spots, species, pondSpecies, tackle
}

func (s *Store) GetFishingLevel(ctx context.Context, userID int64) (int64, error) {
	level := int64(1)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		level, err = s.fishingLevelTx(ctx, tx, userID)
		return err
	})
	return level, err
}

func (s *Store) ListFishingAnglers(ctx context.Context, userID int64) ([]FishingAnglerSummary, error) {
	anglers := make([]FishingAnglerSummary, 0, 5)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		playerLevel, err := s.fishingLevelTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		anglers = append(anglers, FishingAnglerSummary{
			ActorType: ActionActorTypePlayer, Status: "idle", FishingLevel: playerLevel,
		})

		rows, err := tx.Query(ctx, `
			SELECT k.id, k.name, k.status,
			       GREATEST(COALESCE(a.base_level + a.skill_point_level, 1), 1)
			FROM player_kitties k
			LEFT JOIN player_kitty_attributes a
			  ON a.kitty_id = k.id
			 AND a.attr_key = 'fishing'
			WHERE k.user_id = $1
			ORDER BY k.created_at ASC, k.id ASC
		`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			row := FishingAnglerSummary{ActorType: ActionActorTypeKitty}
			if err := rows.Scan(&row.ActorID, &row.Name, &row.Status, &row.FishingLevel); err != nil {
				return err
			}
			row.Name = strings.TrimSpace(row.Name)
			row.Status = normalizeKittyStatus(row.Status)
			anglers = append(anglers, row)
		}
		return rows.Err()
	})
	return anglers, err
}

func (s *Store) FishingSpotCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT spot_id, COUNT(*)
		FROM fishing_trips
		WHERE status = 'running'
		GROUP BY spot_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64, len(s.gameData.FishingSpots))
	for rows.Next() {
		var spotID string
		var count int64
		if err := rows.Scan(&spotID, &count); err != nil {
			return nil, err
		}
		out[spotID] = count
	}
	return out, rows.Err()
}

func (s *Store) ClaimFishingStarter(ctx context.Context, userID int64) (FishingStarterGrantResult, error) {
	items := map[string]int64{
		"basic_fishing_rod": 1, "basic_spinning_reel": 1,
		"braided_main_line": 8, "clear_leader": 8, "fine_fishing_hook": 12,
		"reed_float": 4, "earthworm_bait": 120,
	}
	result := FishingStarterGrantResult{GrantKey: FishingStarterGrantKey, LoadoutID: "starter-float-v1", Items: items}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		grantRaw, err := json.Marshal(map[string]any{"items": items, "loadoutId": result.LoadoutID})
		if err != nil {
			return err
		}
		var inserted int
		err = tx.QueryRow(ctx, `
			INSERT INTO player_one_time_grants(user_id, grant_key, grant_jsonb, granted_at)
			VALUES($1, $2, $3::jsonb, now())
			ON CONFLICT (user_id, grant_key) DO NOTHING
			RETURNING 1
		`, userID, FishingStarterGrantKey, string(grantRaw)).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			result.Granted = false
			var repairInserted int
			repairRaw, marshalErr := json.Marshal(map[string]any{
				"sourceGrant": FishingStarterGrantKey,
				"purpose":     "equip_fishing_rod_and_reel",
			})
			if marshalErr != nil {
				return marshalErr
			}
			repairErr := tx.QueryRow(ctx, `
				INSERT INTO player_one_time_grants(user_id, grant_key, grant_jsonb, granted_at)
				VALUES($1, $2, $3::jsonb, now())
				ON CONFLICT (user_id, grant_key) DO NOTHING
				RETURNING 1
			`, userID, FishingStarterEquipmentRepairKey, string(repairRaw)).Scan(&repairInserted)
			if errors.Is(repairErr, pgx.ErrNoRows) {
				return nil
			}
			if repairErr != nil {
				return repairErr
			}
			changed, equipErr := s.ensurePlayerFishingStarterEquipmentTx(ctx, tx, userID)
			result.EquipmentChanged = changed
			return equipErr
		}
		if err != nil {
			return err
		}
		for _, itemID := range []string{"basic_fishing_rod", "basic_spinning_reel", "braided_main_line", "clear_leader", "fine_fishing_hook", "reed_float", "earthworm_bait"} {
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, items[itemID], InventoryLedgerMeta{
				SourceSurface: "system", TargetSurface: "inventory",
				Details: map[string]any{"type": "one_time_grant", "grantKey": FishingStarterGrantKey},
			}); err != nil {
				return err
			}
		}
		var rodID, reelID int64
		if err := tx.QueryRow(ctx, `SELECT instance_id FROM player_item_instances WHERE user_id=$1 AND template_item_id='basic_fishing_rod' AND state='inventory' ORDER BY instance_id DESC LIMIT 1`, userID).Scan(&rodID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT instance_id FROM player_item_instances WHERE user_id=$1 AND template_item_id='basic_spinning_reel' AND state='inventory' ORDER BY instance_id DESC LIMIT 1`, userID).Scan(&reelID); err != nil {
			return err
		}
		componentsRaw, err := json.Marshal(FishingLoadoutComponents{
			RodInstanceID: rodID, ReelInstanceID: reelID,
			MainLineItemID: "braided_main_line", LeaderItemID: "clear_leader",
			HookItemID: "fine_fishing_hook", FloatOrSinkerItemID: "reed_float", BaitItemID: "earthworm_bait",
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fishing_loadouts(user_id, loadout_id, name, method, strategy, safety_load_percent, components_jsonb, created_at, updated_at)
			VALUES($1, $2, '入门浮漂钓组', 'float', 'balanced', 82, $3::jsonb, now(), now())
			ON CONFLICT (user_id, loadout_id) DO NOTHING
		`, userID, result.LoadoutID, string(componentsRaw)); err != nil {
			return err
		}
		changed, err := s.ensurePlayerFishingStarterEquipmentTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		result.EquipmentChanged = changed
		repairRaw, err := json.Marshal(map[string]any{
			"sourceGrant": FishingStarterGrantKey,
			"purpose":     "equip_fishing_rod_and_reel",
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_one_time_grants(user_id, grant_key, grant_jsonb, granted_at)
			VALUES($1, $2, $3::jsonb, now())
			ON CONFLICT (user_id, grant_key) DO NOTHING
		`, userID, FishingStarterEquipmentRepairKey, string(repairRaw)); err != nil {
			return err
		}
		result.Granted = true
		return nil
	})
	return result, err
}

func (s *Store) ensurePlayerFishingStarterEquipmentTx(ctx context.Context, tx pgx.Tx, userID int64) (bool, error) {
	targets := []struct {
		slot   string
		itemID string
	}{
		{slot: "fishing_rod", itemID: "basic_fishing_rod"},
		{slot: "fishing_reel", itemID: "basic_spinning_reel"},
	}
	changed := false
	for _, target := range targets {
		var occupied bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM player_equip_slots
				WHERE user_id = $1 AND equip_type = 'work' AND slot = $2
			)
		`, userID, target.slot).Scan(&occupied); err != nil {
			return false, err
		}
		if occupied {
			continue
		}
		var instanceID int64
		err := tx.QueryRow(ctx, `
			SELECT instance_id
			FROM player_item_instances
			WHERE user_id = $1 AND template_item_id = $2 AND state = 'inventory'
			ORDER BY instance_id ASC
			LIMIT 1
			FOR UPDATE
		`, userID, target.itemID).Scan(&instanceID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		if err := s.EquipTx(ctx, tx, userID, instanceID, "work", []string{target.slot}, nil); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func (s *Store) ListOwnedFishingTackle(ctx context.Context, userID int64) ([]FishingOwnedTackle, error) {
	known := make(map[string]string, len(s.gameData.FishingTackle))
	ids := make([]string, 0, len(s.gameData.FishingTackle))
	for _, definition := range s.gameData.FishingTackle {
		known[definition.ItemID] = definition.Slot
		ids = append(ids, definition.ItemID)
	}
	rows, err := s.db.Query(ctx, `
		SELECT instance_id, user_id, template_item_id, COALESCE(custom_data, '{}'::jsonb), quantity, enhance_level, roll_seed,
		       base_snapshot, affixes, durability, max_durability, state, archived_at
		FROM player_item_instances
		WHERE user_id = $1 AND template_item_id = ANY($2) AND state IN ('inventory', 'equipped')
		ORDER BY template_item_id, instance_id
	`, userID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingOwnedTackle, 0, len(ids)+4)
	stackQuantities := make(map[string]int64, len(ids))
	for rows.Next() {
		var instance LockedItemInstance
		var customDataRaw []byte
		if err := rows.Scan(
			&instance.InstanceID,
			&instance.UserID,
			&instance.TemplateItem,
			&customDataRaw,
			&instance.Quantity,
			&instance.EnhanceLevel,
			&instance.RollSeed,
			&instance.BaseSnapshot,
			&instance.AffixesRaw,
			&instance.Durability,
			&instance.MaxDurability,
			&instance.State,
			&instance.ArchivedAt,
		); err != nil {
			return nil, err
		}
		instance.CustomData = decodeItemCustomization(customDataRaw)
		if s.itemStackable(instance.TemplateItem) {
			if !s.lockedInstanceIsTemplateCommodity(instance) {
				continue
			}
			quantity := stackQuantities[instance.TemplateItem]
			if instance.Quantity > math.MaxInt64-quantity {
				return nil, ErrInventoryQuantityOverflow
			}
			stackQuantities[instance.TemplateItem] = quantity + instance.Quantity
			continue
		}
		out = append(out, FishingOwnedTackle{
			ItemID: instance.TemplateItem, Slot: known[instance.TemplateItem], Quantity: instance.Quantity,
			InstanceID: instance.InstanceID, EnhanceLevel: instance.EnhanceLevel,
			Durability: instance.Durability, MaxDurability: instance.MaxDurability,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	stackIDs := make([]string, 0, len(stackQuantities))
	for itemID := range stackQuantities {
		stackIDs = append(stackIDs, itemID)
	}
	sort.Strings(stackIDs)
	for _, itemID := range stackIDs {
		out = append(out, FishingOwnedTackle{ItemID: itemID, Slot: known[itemID], Quantity: stackQuantities[itemID]})
	}
	return out, nil
}

func (s *Store) GetFishingForecast(ctx context.Context, userID int64, now time.Time) ([]FishingSpotForecast, error) {
	if now.IsZero() {
		now = time.Now()
	}
	out := make([]FishingSpotForecast, 0, len(s.gameData.FishingSpots))
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		for _, spot := range s.gameData.FishingSpots {
			water := s.gameData.FishingWaterByID(spot.WaterID)
			if water == nil {
				continue
			}
			pressure, err := s.fishingPressureTx(ctx, tx, userID, spot.ID, now)
			if err != nil {
				return err
			}
			row := FishingSpotForecast{
				SpotID:   spot.ID,
				Pressure: roundFloat(pressure, 3),
				Now:      fishingForecastConditions(fishingConditionsAt(*water, spot, now), pressure),
			}
			for step := 1; step <= 6; step++ {
				ahead := time.Duration(step) * 10 * time.Minute
				futurePressure := decayFishingPressure(pressure, ahead)
				row.Next = append(row.Next, fishingForecastConditions(fishingConditionsAt(*water, spot, now.Add(ahead)), futurePressure))
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func fishingForecastConditions(conditions FishingConditions, pressure float64) FishingConditions {
	conditions.Activity = roundFloat(conditions.Activity*(1-clampFloat(pressure, 0, 1)*0.55), 3)
	conditions.ActivityBand = "steady"
	if conditions.Activity >= 1.12 {
		conditions.ActivityBand = "busy"
	} else if conditions.Activity < 0.83 {
		conditions.ActivityBand = "quiet"
	}
	return conditions
}

func (s *Store) ListFishingLoadouts(ctx context.Context, userID int64) ([]FishingLoadout, error) {
	rows, err := s.db.Query(ctx, `
		SELECT loadout_id, name, method, strategy, safety_load_percent, components_jsonb, updated_at
		FROM fishing_loadouts
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT $2
	`, userID, FishingMaximumLoadouts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingLoadout, 0, 8)
	for rows.Next() {
		var row FishingLoadout
		var componentsRaw []byte
		if err := rows.Scan(&row.LoadoutID, &row.Name, &row.Method, &row.Strategy, &row.SafetyLoadPercent, &componentsRaw, &row.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(componentsRaw, &row.Components); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) SaveFishingLoadout(ctx context.Context, userID int64, input FishingLoadoutInput) (FishingLoadout, error) {
	input.LoadoutID = strings.TrimSpace(input.LoadoutID)
	if input.LoadoutID == "" {
		input.LoadoutID = newFishingID("loadout")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "My tackle"
	}
	if len([]rune(input.Name)) > 40 || len(input.LoadoutID) > 80 {
		return FishingLoadout{}, fishingLoadoutError("LOADOUT_NAME_INVALID", "")
	}
	input.Method = normalizeFishingMethod(input.Method)
	if input.Method == "" {
		return FishingLoadout{}, fishingLoadoutError("METHOD_INVALID", "")
	}
	input.Strategy = normalizeFishingStrategy(input.Strategy)
	if input.SafetyLoadPercent == 0 {
		input.SafetyLoadPercent = 82
	}
	if input.SafetyLoadPercent < 50 || input.SafetyLoadPercent > 100 {
		return FishingLoadout{}, fishingLoadoutError("SAFETY_INVALID", "")
	}
	componentsRaw, err := json.Marshal(input.Components)
	if err != nil {
		return FishingLoadout{}, err
	}
	var result FishingLoadout
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var canSave bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM fishing_loadouts WHERE user_id = $1 AND loadout_id = $2
			) OR (
				SELECT COUNT(*) < $3 FROM fishing_loadouts WHERE user_id = $1
			)
		`, userID, input.LoadoutID, FishingMaximumLoadouts).Scan(&canSave); err != nil {
			return err
		}
		if !canSave {
			return fishingLoadoutError("LOADOUT_LIMIT", "")
		}
		candidate := FishingLoadout{
			LoadoutID: input.LoadoutID, Name: input.Name, Method: input.Method,
			Strategy: input.Strategy, SafetyLoadPercent: input.SafetyLoadPercent, Components: input.Components,
		}
		if _, err := s.freezeFishingLoadoutTx(ctx, tx, userID, candidate); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			INSERT INTO fishing_loadouts(user_id, loadout_id, name, method, strategy, safety_load_percent, components_jsonb, created_at, updated_at)
			VALUES($1, $2, $3, $4, $5, $6, $7::jsonb, now(), now())
			ON CONFLICT (user_id, loadout_id) DO UPDATE
			SET name = EXCLUDED.name,
			    method = EXCLUDED.method,
			    strategy = EXCLUDED.strategy,
			    safety_load_percent = EXCLUDED.safety_load_percent,
			    components_jsonb = EXCLUDED.components_jsonb,
			    updated_at = now()
			RETURNING loadout_id, name, method, strategy, safety_load_percent, components_jsonb, updated_at
		`, userID, input.LoadoutID, input.Name, input.Method, input.Strategy, input.SafetyLoadPercent, string(componentsRaw)).Scan(
			&result.LoadoutID, &result.Name, &result.Method, &result.Strategy, &result.SafetyLoadPercent, &componentsRaw, &result.UpdatedAt,
		)
	})
	if err != nil {
		return FishingLoadout{}, err
	}
	if err := json.Unmarshal(componentsRaw, &result.Components); err != nil {
		return FishingLoadout{}, err
	}
	return result, nil
}

func (s *Store) DeleteFishingLoadout(ctx context.Context, userID int64, loadoutID string) error {
	loadoutID = strings.TrimSpace(loadoutID)
	if loadoutID == "" {
		return ErrFishingLoadoutNotFound
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM fishing_loadouts WHERE user_id = $1 AND loadout_id = $2`, userID, loadoutID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFishingLoadoutNotFound
	}
	return nil
}

func (s *Store) getFishingLoadoutTx(ctx context.Context, tx pgx.Tx, userID int64, loadoutID string) (FishingLoadout, error) {
	var row FishingLoadout
	var componentsRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT loadout_id, name, method, strategy, safety_load_percent, components_jsonb, updated_at
		FROM fishing_loadouts
		WHERE user_id = $1 AND loadout_id = $2
	`, userID, strings.TrimSpace(loadoutID)).Scan(
		&row.LoadoutID, &row.Name, &row.Method, &row.Strategy, &row.SafetyLoadPercent, &componentsRaw, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return FishingLoadout{}, ErrFishingLoadoutNotFound
	}
	if err != nil {
		return FishingLoadout{}, err
	}
	if err := json.Unmarshal(componentsRaw, &row.Components); err != nil {
		return FishingLoadout{}, fishingLoadoutError("LOADOUT_CORRUPT", "")
	}
	return row, nil
}

func (s *Store) freezeFishingLoadoutTx(ctx context.Context, tx pgx.Tx, userID int64, row FishingLoadout) (FishingLoadoutSnapshot, error) {
	// Saved loadouts are shared by all anglers. Validate components only;
	// preview/start applies the loadout and checks the actual actor's requirements.
	return s.freezeFishingLoadoutForActorTx(ctx, tx, userID, ActionActorTypePlayer, 0, row, false)
}

type fishingEquipmentTarget struct {
	instanceID    int64
	tackleSlot    string
	equipmentSlot string
	itemID        string
	occupySlots   []string
}

func (s *Store) fishingEquipmentKittyIDsTx(
	ctx context.Context,
	tx pgx.Tx,
	actorType string,
	actorID int64,
	row FishingLoadout,
) ([]int64, error) {
	seen := map[int64]struct{}{}
	if normalizeActorType(actorType) == ActionActorTypeKitty && actorID > 0 {
		seen[actorID] = struct{}{}
	}
	instanceIDs := []int64{row.Components.RodInstanceID, row.Components.ReelInstanceID}
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT kitty_id
		FROM player_kitty_equip_slots
		WHERE instance_id = ANY($1::bigint[])
	`, instanceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kittyID int64
		if err := rows.Scan(&kittyID); err != nil {
			return nil, err
		}
		if kittyID > 0 {
			seen[kittyID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	kittyIDs := make([]int64, 0, len(seen))
	for kittyID := range seen {
		kittyIDs = append(kittyIDs, kittyID)
	}
	sort.Slice(kittyIDs, func(i, j int) bool { return kittyIDs[i] < kittyIDs[j] })
	return kittyIDs, nil
}

func (s *Store) previewFishingLoadoutForActorTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	actorType string,
	actorID int64,
	row FishingLoadout,
) (FishingLoadoutSnapshot, error) {
	previewTx, err := tx.Begin(ctx)
	if err != nil {
		return FishingLoadoutSnapshot{}, err
	}
	if err = s.applyFishingLoadoutEquipmentForActorTx(ctx, previewTx, userID, actorType, actorID, row); err != nil {
		rollbackErr := previewTx.Rollback(ctx)
		return FishingLoadoutSnapshot{}, errors.Join(err, rollbackErr)
	}
	snapshot, previewErr := s.freezeFishingLoadoutForActorTx(ctx, previewTx, userID, actorType, actorID, row, true)
	rollbackErr := previewTx.Rollback(ctx)
	if previewErr != nil {
		return FishingLoadoutSnapshot{}, errors.Join(previewErr, rollbackErr)
	}
	if rollbackErr != nil {
		return FishingLoadoutSnapshot{}, rollbackErr
	}
	return snapshot, nil
}

func (s *Store) applyFishingLoadoutEquipmentForActorTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	actorType string,
	actorID int64,
	row FishingLoadout,
) error {
	actorType = normalizeActorType(actorType)
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if actorType == ActionActorTypePlayer {
		actorID = 0
	} else if actorType == ActionActorTypeKitty {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, actorID, true); err != nil {
			return err
		}
	} else {
		return ErrActionActorInvalid
	}
	method := normalizeFishingMethod(row.Method)
	if method == "" {
		return fishingLoadoutError("METHOD_INVALID", "")
	}
	if row.Components.RodInstanceID <= 0 {
		return fishingLoadoutError("COMPONENT_REQUIRED", "rod")
	}
	if row.Components.ReelInstanceID <= 0 {
		return fishingLoadoutError("COMPONENT_REQUIRED", "reel")
	}
	if row.Components.RodInstanceID == row.Components.ReelInstanceID {
		return fishingLoadoutError("COMPONENT_INCOMPATIBLE", "reel")
	}
	targets := []fishingEquipmentTarget{
		{instanceID: row.Components.RodInstanceID, tackleSlot: "rod", equipmentSlot: "fishing_rod"},
		{instanceID: row.Components.ReelInstanceID, tackleSlot: "reel", equipmentSlot: "fishing_reel"},
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].instanceID < targets[j].instanceID })
	for index := range targets {
		target := &targets[index]
		locked, err := s.LockInstanceTx(ctx, tx, target.instanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFishingTackleMissing
			}
			return err
		}
		if locked.UserID != userID {
			return ErrInstanceNotOwned
		}
		if locked.State != "inventory" && locked.State != "equipped" {
			return &InvalidInstanceStateError{State: locked.State}
		}
		target.itemID = strings.TrimSpace(locked.TemplateItem)
		definition := s.gameData.FishingTackleByItemID(target.itemID)
		if definition == nil || definition.Slot != target.tackleSlot || !containsFishingMethod(definition.Methods, method) {
			return fishingLoadoutError("COMPONENT_INCOMPATIBLE", target.tackleSlot)
		}
		template := s.gameData.EquipmentTemplateByID(target.itemID)
		if template == nil || normalizeEquipType(template.EquipType) != "work" || strings.TrimSpace(template.Slot) != target.equipmentSlot {
			return fishingLoadoutError("EQUIPMENT_INVALID", target.tackleSlot)
		}
		if actorType == ActionActorTypePlayer {
			if !equipmentTemplateAllowsPlayer(template) {
				return ErrEquipTargetNotAllowed
			}
			if err := s.ensureEquipmentStatusRequirementsTx(ctx, tx, userID, template); err != nil {
				return err
			}
		} else {
			if !equipmentTemplateAllowsKitty(template) {
				return ErrEquipTargetNotAllowed
			}
			if err := s.ensureKittyEquipmentStatusRequirementsTx(ctx, tx, userID, actorID, template); err != nil {
				return err
			}
		}
		target.occupySlots = []string{target.equipmentSlot}
		if len(template.OccupySlots) > 0 {
			target.occupySlots = uniqueTrimmedStrings(template.OccupySlots)
			if len(target.occupySlots) != 1 || target.occupySlots[0] != target.equipmentSlot {
				return fishingLoadoutError("EQUIPMENT_INVALID", target.tackleSlot)
			}
		}
	}
	for _, target := range targets {
		if actorType == ActionActorTypePlayer {
			if err := s.EquipTx(ctx, tx, userID, target.instanceID, "work", target.occupySlots, nil); err != nil {
				return err
			}
			continue
		}
		if err := s.equipKittyItemTx(ctx, tx, userID, actorID, target.instanceID, "work", target.occupySlots); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureKittyEquipmentStatusRequirementsTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	kittyID int64,
	template *catalog.EquipmentTemplate,
) error {
	if template == nil || len(template.StatusRequirement) == 0 {
		return nil
	}
	if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData()); err != nil {
		return err
	}
	for rawKey, required := range template.StatusRequirement {
		key := strings.TrimSpace(rawKey)
		if key == "" || required <= 0 {
			continue
		}
		var value float64
		if err := tx.QueryRow(ctx, `
			SELECT final_value
			FROM player_kitty_attributes
			WHERE kitty_id = $1 AND attr_key = $2
			FOR UPDATE
		`, kittyID, key).Scan(&value); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrStatusRequireNotMet
			}
			return err
		}
		if value < float64(required) {
			return ErrStatusRequireNotMet
		}
	}
	return nil
}

func (s *Store) ensureEquipmentInstancesNotInActiveFishingTripTx(ctx context.Context, tx pgx.Tx, userID int64, instanceIDs ...int64) error {
	unique := make([]int64, 0, len(instanceIDs))
	seen := make(map[int64]struct{}, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		if instanceID <= 0 {
			continue
		}
		if _, ok := seen[instanceID]; ok {
			continue
		}
		seen[instanceID] = struct{}{}
		unique = append(unique, instanceID)
	}
	if len(unique) == 0 {
		return nil
	}
	var inUse bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM fishing_trips t
			CROSS JOIN unnest($2::bigint[]) AS requested(instance_id)
			WHERE t.user_id = $1
			  AND t.status = 'running'
			  AND (
				COALESCE(t.loadout_snapshot_jsonb -> 'components' ->> 'rodInstanceId', '') = requested.instance_id::text
				OR COALESCE(t.loadout_snapshot_jsonb -> 'components' ->> 'reelInstanceId', '') = requested.instance_id::text
			  )
		)
	`, userID, unique).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrFishingTackleInUse
	}
	return nil
}

func (s *Store) freezeFishingLoadoutForActorTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	actorType string,
	actorID int64,
	row FishingLoadout,
	requireEquipped bool,
) (FishingLoadoutSnapshot, error) {
	actorType = normalizeActorType(actorType)
	if actorType == ActionActorTypePlayer {
		actorID = 0
	} else if actorType == ActionActorTypeKitty {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, actorID, false); err != nil {
			return FishingLoadoutSnapshot{}, err
		}
	} else {
		return FishingLoadoutSnapshot{}, ErrActionActorInvalid
	}
	method := normalizeFishingMethod(row.Method)
	if method == "" {
		return FishingLoadoutSnapshot{}, fishingLoadoutError("METHOD_INVALID", "")
	}
	if row.Components.RodInstanceID <= 0 {
		return FishingLoadoutSnapshot{}, fishingLoadoutError("COMPONENT_REQUIRED", "rod")
	}
	if row.Components.ReelInstanceID <= 0 {
		return FishingLoadoutSnapshot{}, fishingLoadoutError("COMPONENT_REQUIRED", "reel")
	}
	componentIDs := map[string]string{
		"main_line": row.Components.MainLineItemID,
		"leader":    row.Components.LeaderItemID,
	}
	if method == "lure" {
		componentIDs["lure"] = row.Components.LureItemID
	} else {
		componentIDs["hook"] = row.Components.HookItemID
		componentIDs["float_or_sinker"] = row.Components.FloatOrSinkerItemID
		componentIDs["bait"] = row.Components.BaitItemID
	}

	frozen := make([]FishingFrozenComponent, 0, len(componentIDs)+2)
	control := 0.0
	sensitivity := 0.0
	casting := 0.0
	hooking := 0.0
	efficiencyBonus := 0.0
	maxLoad := math.MaxFloat64
	biteMultiplier := 1.0
	mutationChanceMultiplier := 1.0
	waterBiteModifiers := map[string]float64{}
	speciesWeightModifiers := map[string]float64{}
	mergeModifiers := func(target map[string]float64, source map[string]float64) {
		for key, value := range source {
			key = strings.TrimSpace(key)
			if key != "" && math.IsNaN(value) == false && math.IsInf(value, 0) == false {
				target[key] += value
			}
		}
	}
	for _, instance := range []struct {
		slot string
		id   int64
	}{{"rod", row.Components.RodInstanceID}, {"reel", row.Components.ReelInstanceID}} {
		var itemID string
		err := tx.QueryRow(ctx, `
			SELECT template_item_id
			FROM player_item_instances
			WHERE user_id = $1 AND instance_id = $2 AND state IN ('inventory', 'equipped')
			FOR SHARE
		`, userID, instance.id).Scan(&itemID)
		if errors.Is(err, pgx.ErrNoRows) {
			return FishingLoadoutSnapshot{}, ErrFishingTackleMissing
		}
		if err != nil {
			return FishingLoadoutSnapshot{}, err
		}
		definition := s.gameData.FishingTackleByItemID(itemID)
		if definition == nil || definition.Slot != instance.slot || !containsFishingMethod(definition.Methods, method) {
			return FishingLoadoutSnapshot{}, fishingLoadoutError("COMPONENT_INCOMPATIBLE", instance.slot)
		}
		template := s.gameData.EquipmentTemplateByID(itemID)
		if template == nil || normalizeEquipType(template.EquipType) != "work" {
			return FishingLoadoutSnapshot{}, fishingLoadoutError("EQUIPMENT_INVALID", instance.slot)
		}
		if requireEquipped {
			if err := s.ensureFishingEquipmentOwnedByActorTx(ctx, tx, userID, actorType, actorID, instance.id, template.Slot); err != nil {
				if errors.Is(err, ErrFishingTackleMissing) {
					return FishingLoadoutSnapshot{}, &FishingTackleNotEquippedError{ItemID: itemID}
				}
				return FishingLoadoutSnapshot{}, err
			}
		}
		if requireEquipped && actorType == ActionActorTypePlayer {
			if err := s.ensureEquipmentStatusRequirementsTx(ctx, tx, userID, template); err != nil {
				return FishingLoadoutSnapshot{}, err
			}
		}
		entry := FishingFrozenComponent{
			Slot: definition.Slot, ItemID: itemID, InstanceID: instance.id,
			Control: definition.Control, Sensitivity: definition.Sensitivity,
			Casting: definition.Casting, Hooking: definition.Hooking, Efficiency: definition.Efficiency,
			MaxLoadKG: definition.MaxLoadKG, MutationChanceMultiplier: definition.MutationChanceMultiplier,
		}
		frozen = append(frozen, entry)
		if definition.BiteMultiplier > 0 {
			biteMultiplier *= definition.BiteMultiplier
		}
		if definition.MutationChanceMultiplier > 0 {
			mutationChanceMultiplier *= definition.MutationChanceMultiplier
		}
		mergeModifiers(waterBiteModifiers, definition.WaterBiteModifiers)
		mergeModifiers(speciesWeightModifiers, definition.SpeciesWeightModifiers)
	}
	componentSlots := make([]string, 0, len(componentIDs))
	for slot := range componentIDs {
		componentSlots = append(componentSlots, slot)
	}
	sort.Strings(componentSlots)
	for _, slot := range componentSlots {
		itemID := componentIDs[slot]
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			return FishingLoadoutSnapshot{}, fishingLoadoutError("COMPONENT_REQUIRED", slot)
		}
		definition := s.gameData.FishingTackleByItemID(itemID)
		if definition == nil || definition.Slot != slot || !containsFishingMethod(definition.Methods, method) {
			return FishingLoadoutSnapshot{}, fishingLoadoutError("COMPONENT_INCOMPATIBLE", slot)
		}
		quantity, err := s.ConsumableQuantityByTemplateTx(ctx, tx, userID, itemID)
		if err != nil {
			return FishingLoadoutSnapshot{}, err
		}
		if quantity <= 0 {
			return FishingLoadoutSnapshot{}, &FishingSupplyShortageError{ItemID: itemID, Required: 1, Available: quantity}
		}
		entry := FishingFrozenComponent{Slot: definition.Slot, ItemID: itemID, Control: definition.Control, Sensitivity: definition.Sensitivity, Casting: definition.Casting, Hooking: definition.Hooking, Efficiency: definition.Efficiency, MaxLoadKG: definition.MaxLoadKG, WearPerFight: definition.WearPerFight, MutationChanceMultiplier: definition.MutationChanceMultiplier}
		frozen = append(frozen, entry)
		control += definition.Control
		sensitivity += definition.Sensitivity
		casting += definition.Casting
		hooking += definition.Hooking
		efficiencyBonus += definition.Efficiency
		if definition.MaxLoadKG > 0 && definition.MaxLoadKG < maxLoad {
			maxLoad = definition.MaxLoadKG
		}
		if definition.BiteMultiplier > 0 {
			biteMultiplier *= definition.BiteMultiplier
		}
		if definition.MutationChanceMultiplier > 0 {
			mutationChanceMultiplier *= definition.MutationChanceMultiplier
		}
		mergeModifiers(waterBiteModifiers, definition.WaterBiteModifiers)
		mergeModifiers(speciesWeightModifiers, definition.SpeciesWeightModifiers)
	}
	if maxLoad == math.MaxFloat64 || maxLoad <= 0 {
		return FishingLoadoutSnapshot{}, fishingLoadoutError("LOAD_CAPACITY_INVALID", "")
	}
	var (
		level      int64 = 1
		attrTotals map[string]float64
		err        error
	)
	if !requireEquipped {
		// No angler is selected while saving a shared loadout.
		attrTotals = map[string]float64{}
	} else if actorType == ActionActorTypeKitty {
		level, attrTotals, err = s.fishingKittyAttributeTotalsTx(ctx, tx, userID, actorID)
	} else {
		level, err = s.fishingLevelTx(ctx, tx, userID)
		if err == nil {
			attrTotals, _, err = s.computePlayerAttributeTotalsTx(ctx, tx, userID, nil)
		}
	}
	if err != nil {
		return FishingLoadoutSnapshot{}, err
	}
	control += attrTotals["fishing_control"]
	sensitivity += attrTotals["fishing_sensitivity"]
	casting += attrTotals["fishing_casting"]
	hooking += attrTotals["fishing_hooking"]
	power := clampFloat(attrTotals["fishing_power"], 0, 0.4)
	maxLoad *= 1 + power
	efficiency := float64(max(level-1, 0))*0.01 + attrTotals["fishing_efficiency"] + efficiencyBonus
	return FishingLoadoutSnapshot{
		LoadoutID: row.LoadoutID, Name: row.Name, Method: method,
		Strategy: normalizeFishingStrategy(row.Strategy), SafetyLoadPercent: row.SafetyLoadPercent,
		Components: row.Components, FrozenComponents: frozen,
		Control: roundFloat(control, 4), Sensitivity: roundFloat(sensitivity, 4), Efficiency: roundFloat(clampFloat(efficiency, -0.5, 1.5), 4), Power: roundFloat(power, 4),
		Casting: roundFloat(casting, 4), Hooking: roundFloat(hooking, 4),
		MaxLoadKG: roundFloat(maxLoad, 3), BiteMultiplier: roundFloat(biteMultiplier, 4), MutationChanceMultiplier: roundFloat(mutationChanceMultiplier, 4), WaterBiteModifiers: waterBiteModifiers, SpeciesWeightModifiers: speciesWeightModifiers, FishingLevel: level,
	}, nil
}

func (s *Store) ensureFishingEquipmentOwnedByActorTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	actorType string,
	actorID int64,
	instanceID int64,
	slot string,
) error {
	var exists int
	if actorType == ActionActorTypeKitty {
		err := tx.QueryRow(ctx, `
			SELECT 1
			FROM player_kitty_equip_slots s
			JOIN player_kitties k ON k.id = s.kitty_id
			WHERE k.user_id = $1 AND s.kitty_id = $2 AND s.instance_id = $3
			  AND s.equip_type = 'work' AND s.slot = $4
			LIMIT 1
		`, userID, actorID, instanceID, strings.TrimSpace(slot)).Scan(&exists)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrFishingTackleMissing
		}
		return err
	}
	err := tx.QueryRow(ctx, `
		SELECT 1
		FROM player_equip_slots
		WHERE user_id = $1 AND instance_id = $2 AND equip_type = 'work' AND slot = $3
		LIMIT 1
	`, userID, instanceID, strings.TrimSpace(slot)).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrFishingTackleMissing
	}
	return err
}

func (s *Store) fishingKittyAttributeTotalsTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64) (int64, map[string]float64, error) {
	seeds := s.attributeSeedsFromGameData()
	if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds); err != nil {
		return 0, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT attr_key, final_value, base_level, skill_point_level
		FROM player_kitty_attributes
		WHERE kitty_id = $1
	`, kittyID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	level := int64(1)
	totals := make(map[string]float64, len(seeds))
	for rows.Next() {
		var key string
		var value float64
		var baseLevel, skillPointLevel int64
		if err := rows.Scan(&key, &value, &baseLevel, &skillPointLevel); err != nil {
			return 0, nil, err
		}
		key = normalizeLower(key)
		if key == "" {
			continue
		}
		totals[key] = value
		if key == "fishing" {
			level = maxInt64(1, baseLevel+skillPointLevel)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	return level, totals, nil
}

func containsFishingMethod(methods []string, method string) bool {
	for _, candidate := range methods {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(method)) {
			return true
		}
	}
	return false
}

func (s *Store) fishingLevelTx(ctx context.Context, tx pgx.Tx, userID int64) (int64, error) {
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, userID); err != nil {
		return 0, err
	}
	var level int64
	err := tx.QueryRow(ctx, `
		SELECT GREATEST(base_level + skill_point_level, 1)
		FROM player_attributes
		WHERE user_id = $1 AND attr_key = 'fishing'
	`, userID).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1, nil
	}
	return level, err
}

func (s *Store) fishingPressureTx(ctx context.Context, tx pgx.Tx, userID int64, spotID string, now time.Time) (float64, error) {
	var pressure float64
	var updatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT pressure, updated_at
		FROM fishing_spot_pressure
		WHERE user_id = $1 AND spot_id = $2
		FOR UPDATE
	`, userID, spotID).Scan(&pressure, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return decayFishingPressure(pressure, now.Sub(updatedAt)), nil
}

func decayFishingPressure(pressure float64, elapsed time.Duration) float64 {
	if pressure <= 0 {
		return 0
	}
	if elapsed <= 0 {
		return clampFloat(pressure, 0, 1)
	}
	// Pressure has a six-hour half-life. It remains personal and never depletes a shared fish pool.
	return clampFloat(pressure*math.Pow(0.5, elapsed.Hours()/6), 0, 1)
}

func (s *Store) saveFishingPressureTx(ctx context.Context, tx pgx.Tx, userID int64, spotID string, pressure float64, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fishing_spot_pressure(user_id, spot_id, pressure, last_fished_at, updated_at)
		VALUES($1, $2, $3, $4, $4)
		ON CONFLICT (user_id, spot_id) DO UPDATE
		SET pressure = EXCLUDED.pressure,
		    last_fished_at = EXCLUDED.last_fished_at,
		    updated_at = EXCLUDED.updated_at
	`, userID, spotID, clampFloat(pressure, 0, 1), now)
	return err
}

func (s *Store) addFishInstanceTx(ctx context.Context, tx pgx.Tx, userID int64, fish FishInstanceData) (int64, error) {
	raw, err := json.Marshal(fish)
	if err != nil {
		return 0, err
	}
	var instanceID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO player_item_instances(
			user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes, custom_data, fish_data,
			durability, max_durability, quantity, state, created_at, updated_at
		)
		VALUES($1, 'fish_catch', 0, 0, '{}'::jsonb, '[]'::jsonb, '{}'::jsonb, $2::jsonb,
		       100, 100, 1, 'inventory', now(), now())
		RETURNING instance_id
	`, userID, string(raw)).Scan(&instanceID)
	return instanceID, err
}

func (s *Store) GetCurrentFishingTrip(ctx context.Context, userID int64) (*FishingTripState, error) {
	return s.getFishingTrip(ctx, userID, true)
}

func (s *Store) GetLatestFishingTrip(ctx context.Context, userID int64) (*FishingTripState, error) {
	return s.getFishingTrip(ctx, userID, false)
}

func (s *Store) ListRecentFishingTrips(ctx context.Context, userID int64) ([]FishingTripState, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, COALESCE(queue_id, 0), actor_type, actor_id, water_id, spot_id, loadout_id, strategy, safety_load_percent,
		       status, phase, phase_started_at, phase_ends_at, planned_end_at, current_fish_jsonb,
		       summary_jsonb, supplies_jsonb, stop_reason, started_at, ended_at
		FROM fishing_trips
		WHERE user_id = $1 AND status IN ('completed', 'stopped')
		ORDER BY started_at DESC, id DESC
		LIMIT $2
	`, userID, FishingTripHistoryLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]FishingTripState, 0, FishingTripHistoryLimit)
	for rows.Next() {
		row, err := scanFishingTripState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const fishingTripQueueMissingStopReason = "queue_missing"

const orphanedFishingTripExistsSQL = `
	SELECT EXISTS (
		SELECT 1
		FROM fishing_trips ft
		WHERE ft.user_id = $1
		  AND ft.status = 'running'
		  AND NOT EXISTS (
			SELECT 1
			FROM player_action_queue q
			WHERE q.id = ft.queue_id
			  AND q.user_id = ft.user_id
			  AND q.action_id = $2
			  AND q.actor_type = ft.actor_type
			  AND q.actor_id = ft.actor_id
			  AND q.status = 'running'
		  )
	)
`

const fishingTripStopSelectSQL = `
	SELECT ft.id, ft.queue_id, ft.phase,
	       ft.loadout_snapshot_jsonb, ft.supplies_jsonb, ft.summary_jsonb,
	       EXISTS (
	         SELECT 1
	         FROM player_action_queue q
	         WHERE q.id = ft.queue_id
	           AND q.user_id = ft.user_id
	           AND q.action_id = $2
	           AND q.actor_type = ft.actor_type
	           AND q.actor_id = ft.actor_id
	           AND q.status = 'running'
	       )
	FROM fishing_trips ft
	WHERE ft.user_id = $1 AND ft.status = 'running'
	ORDER BY ft.started_at DESC
	LIMIT 1
	FOR UPDATE OF ft
`

// StopStaleFishingTrip force-closes a running trip that could not be cancelled
// through the ordinary queue path. It remains the final fallback for the
// explicit stop endpoint.
func (s *Store) StopStaleFishingTrip(ctx context.Context, userID int64) (bool, error) {
	return s.stopFishingTrip(ctx, userID, false, "manual_stop")
}

// RecoverOrphanedFishingTrip closes a running trip only when its matching
// running queue row is gone. Reserved supplies are returned before the stale
// state is exposed to the client again.
func (s *Store) RecoverOrphanedFishingTrip(ctx context.Context, userID int64) (bool, error) {
	var orphaned bool
	if err := s.db.QueryRow(ctx, orphanedFishingTripExistsSQL, userID, FishingTripActionID).Scan(&orphaned); err != nil {
		return false, err
	}
	if !orphaned {
		return false, nil
	}
	return s.stopFishingTrip(ctx, userID, true, fishingTripQueueMissingStopReason)
}

func (s *Store) stopFishingTrip(ctx context.Context, userID int64, orphanedOnly bool, stopReason string) (bool, error) {
	stopped := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var (
			tripID                  string
			queueID                 *int64
			phase                   string
			loadoutRaw, suppliesRaw []byte
			summaryRaw              []byte
			hasRunningQueue         bool
		)
		err := tx.QueryRow(ctx, fishingTripStopSelectSQL, userID, FishingTripActionID).Scan(
			&tripID, &queueID, &phase,
			&loadoutRaw, &suppliesRaw, &summaryRaw, &hasRunningQueue,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if !shouldStopFishingTrip(orphanedOnly, hasRunningQueue) {
			return nil
		}
		if err := s.finishStoppedFishingTripTx(ctx, tx, userID, fishingTripStopState{
			ID: tripID, Phase: phase, LoadoutRaw: loadoutRaw, SuppliesRaw: suppliesRaw, SummaryRaw: summaryRaw,
		}, stopReason, time.Now()); err != nil {
			return err
		}
		if queueID != nil && *queueID > 0 {
			if _, err := tx.Exec(ctx, `
				DELETE FROM player_action_queue
				WHERE id = $1 AND user_id = $2 AND action_id = $3
			`, *queueID, userID, FishingTripActionID); err != nil {
				return err
			}
		}
		if err := s.normalizeQueueOrderTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.reconcileKittyLaneStatusesTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.pruneFishingTripEventsTx(ctx, tx, userID); err != nil {
			return err
		}
		stopped = true
		return nil
	})
	return stopped, err
}

func shouldStopFishingTrip(orphanedOnly bool, hasRunningQueue bool) bool {
	return !orphanedOnly || !hasRunningQueue
}

func (s *Store) getFishingTrip(ctx context.Context, userID int64, runningOnly bool) (*FishingTripState, error) {
	query := `
		SELECT id, COALESCE(queue_id, 0), actor_type, actor_id, water_id, spot_id, loadout_id, strategy, safety_load_percent,
		       status, phase, phase_started_at, phase_ends_at, planned_end_at, current_fish_jsonb,
		       summary_jsonb, supplies_jsonb, stop_reason, started_at, ended_at
		FROM fishing_trips
		WHERE user_id = $1`
	if runningOnly {
		query += ` AND status = 'running'`
	}
	query += ` ORDER BY started_at DESC LIMIT 1`
	row, err := scanFishingTripState(s.db.QueryRow(ctx, query, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

type fishingTripScanner interface {
	Scan(dest ...any) error
}

func scanFishingTripState(scanner fishingTripScanner) (FishingTripState, error) {
	var row FishingTripState
	var currentRaw, summaryRaw, suppliesRaw []byte
	err := scanner.Scan(
		&row.TripID, &row.QueueID, &row.ActorType, &row.ActorID, &row.WaterID, &row.SpotID, &row.LoadoutID, &row.Strategy, &row.SafetyLoadPercent,
		&row.Status, &row.Phase, &row.PhaseStartedAt, &row.PhaseEndsAt, &row.PlannedEndAt, &currentRaw,
		&summaryRaw, &suppliesRaw, &row.StopReason, &row.StartedAt, &row.EndedAt,
	)
	if err != nil {
		return row, err
	}
	if len(currentRaw) > 0 && string(currentRaw) != "{}" {
		if err := json.Unmarshal(currentRaw, &row.CurrentFish); err != nil {
			return row, err
		}
	}
	if err := json.Unmarshal(summaryRaw, &row.Summary); err != nil {
		return row, err
	}
	if len(suppliesRaw) > 0 && string(suppliesRaw) != "{}" {
		if err := json.Unmarshal(suppliesRaw, &row.Supplies); err != nil {
			return row, err
		}
	}
	return row, nil
}

func (s *Store) ListFishingCatches(ctx context.Context, userID int64, cursor int64, limit int, speciesID string, keptOnly bool) (FishingCatchPage, error) {
	if limit <= 0 {
		limit = FishingDefaultPageLimit
	}
	if limit > FishingMaximumPageLimit {
		limit = FishingMaximumPageLimit
	}
	if cursor <= 0 {
		cursor = math.MaxInt64
	}
	if keptOnly {
		return s.listCurrentFishingInventory(ctx, userID, cursor, limit, speciesID)
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.trip_id, c.species_id, c.fish_data_jsonb, c.kept,
		       CASE
		         WHEN i.state = 'inventory' AND i.archived_at IS NULL THEN c.inventory_instance_id
		         ELSE NULL
		       END,
		       COALESCE(i.fish_data, '{}'::jsonb),
		       c.caught_at,
		       COALESCE(r.best_length_catch_id = c.id, FALSE),
		       COALESCE(r.best_weight_catch_id = c.id, FALSE)
		FROM fishing_catches c
		LEFT JOIN player_item_instances i
		  ON i.instance_id = c.inventory_instance_id AND i.user_id = c.user_id
		LEFT JOIN fishing_records r
		  ON r.user_id = c.user_id AND r.species_id = c.species_id AND r.ruleset_version = $6
		WHERE c.user_id = $1
		  AND c.id < $2
		  AND ($3 = '' OR c.species_id = $3)
		  AND i.state IS DISTINCT FROM $8
		  AND (NOT $4 OR (i.state = 'inventory' AND i.archived_at IS NULL))
		  AND (c.caught_at >= now() - ($7 * interval '1 day') OR r.best_length_catch_id = c.id OR r.best_weight_catch_id = c.id OR (i.state = 'inventory' AND i.archived_at IS NULL))
		ORDER BY c.id DESC
		LIMIT $5
	`, userID, cursor, strings.TrimSpace(speciesID), keptOnly, limit+1, FishingRulesetVersion, FishingCatchHistoryDays, fishingPondState)
	if err != nil {
		return FishingCatchPage{}, err
	}
	defer rows.Close()
	out := FishingCatchPage{Items: make([]FishingCatchView, 0, limit)}
	for rows.Next() {
		var row FishingCatchView
		var fishRaw, inventoryFishRaw []byte
		if err := rows.Scan(&row.CatchID, &row.TripID, &row.SpeciesID, &fishRaw, &row.Kept, &row.InventoryInstanceID, &inventoryFishRaw, &row.CaughtAt, &row.Record.Length, &row.Record.Weight); err != nil {
			return FishingCatchPage{}, err
		}
		if err := json.Unmarshal(fishRaw, &row.Fish); err != nil {
			return FishingCatchPage{}, err
		}
		if row.InventoryInstanceID != nil {
			var inventoryFish FishInstanceData
			if json.Unmarshal(inventoryFishRaw, &inventoryFish) == nil {
				// The inventory instance is authoritative for a kept fish. Its pond growth,
				// inherited mutations, or administrative adjustments may be newer than the
				// immutable catch-history snapshot.
				row.Fish = fishingCatchCurrentFish(row.SpeciesID, row.Fish, inventoryFish)
				species := s.gameData.FishingSpeciesByID(inventoryFish.SpeciesID)
				profile := s.gameData.FishingPondSpeciesByID(inventoryFish.SpeciesID)
				row.SaleIneligibleReason, row.SaleRequiredAgeDays = fishingSaleIneligibility(species, profile, inventoryFish)
				row.SaleEligible = row.SaleIneligibleReason == ""
				row.SaleCurrentAgeDays = inventoryFish.AgeDays
			}
		}
		row.Record.SpeciesID = row.SpeciesID
		if len(out.Items) == limit {
			out.NextCursor = row.CatchID + 1
			break
		}
		out.Items = append(out.Items, row)
	}
	return out, rows.Err()
}

func (s *Store) listCurrentFishingInventory(ctx context.Context, userID int64, cursor int64, limit int, speciesID string) (FishingCatchPage, error) {
	rows, err := s.db.Query(ctx, `
		SELECT CASE WHEN c.id IS NULL THEN -i.instance_id ELSE c.id END,
		       COALESCE(c.trip_id, ''),
		       i.fish_data->>'speciesId',
		       i.fish_data,
		       i.instance_id,
		       COALESCE(c.caught_at, i.created_at),
		       c.id IS NOT NULL,
		       COALESCE(r.best_length_catch_id = c.id, FALSE),
		       COALESCE(r.best_weight_catch_id = c.id, FALSE)
		FROM player_item_instances i
		LEFT JOIN LATERAL (
			SELECT recent.id, recent.trip_id, recent.species_id, recent.caught_at
			FROM fishing_catches recent
			WHERE recent.user_id = i.user_id
			  AND recent.inventory_instance_id = i.instance_id
			ORDER BY recent.id DESC
			LIMIT 1
		) c ON TRUE
		LEFT JOIN fishing_records r
		  ON r.user_id = i.user_id
		 AND r.species_id = i.fish_data->>'speciesId'
		 AND r.ruleset_version = $5
		WHERE i.user_id = $1
		  AND i.instance_id < $2
		  AND i.template_item_id = 'fish_catch'
		  AND i.state = 'inventory'
		  AND i.archived_at IS NULL
		  AND COALESCE(NULLIF(BTRIM(i.fish_data->>'speciesId'), ''), '') <> ''
		  AND ($3 = '' OR i.fish_data->>'speciesId' = $3)
		ORDER BY i.instance_id DESC
		LIMIT $4
	`, userID, cursor, strings.TrimSpace(speciesID), limit+1, FishingRulesetVersion)
	if err != nil {
		return FishingCatchPage{}, err
	}
	defer rows.Close()

	out := FishingCatchPage{Items: make([]FishingCatchView, 0, limit)}
	for rows.Next() {
		var row FishingCatchView
		var fishRaw []byte
		var inventoryInstanceID int64
		var hasCatchHistory bool
		if err := rows.Scan(
			&row.CatchID,
			&row.TripID,
			&row.SpeciesID,
			&fishRaw,
			&inventoryInstanceID,
			&row.CaughtAt,
			&hasCatchHistory,
			&row.Record.Length,
			&row.Record.Weight,
		); err != nil {
			return FishingCatchPage{}, err
		}
		if err := json.Unmarshal(fishRaw, &row.Fish); err != nil {
			return FishingCatchPage{}, err
		}
		if !hasCatchHistory && !row.Fish.CaughtAt.IsZero() {
			row.CaughtAt = row.Fish.CaughtAt
		}
		row.Kept = true
		row.InventoryInstanceID = &inventoryInstanceID
		row.Record.SpeciesID = row.SpeciesID
		species := s.gameData.FishingSpeciesByID(row.Fish.SpeciesID)
		profile := s.gameData.FishingPondSpeciesByID(row.Fish.SpeciesID)
		row.SaleIneligibleReason, row.SaleRequiredAgeDays = fishingSaleIneligibility(species, profile, row.Fish)
		row.SaleEligible = row.SaleIneligibleReason == ""
		row.SaleCurrentAgeDays = row.Fish.AgeDays
		if len(out.Items) == limit {
			out.NextCursor = inventoryInstanceID + 1
			break
		}
		out.Items = append(out.Items, row)
	}
	return out, rows.Err()
}

func fishingCatchCurrentFish(speciesID string, snapshot, inventory FishInstanceData) FishInstanceData {
	if inventory.SpeciesID == speciesID {
		return inventory
	}
	return snapshot
}

func (s *Store) pruneFishingCatchHistoryTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		WITH expired AS (
			SELECT c.id
			FROM fishing_catches c
			WHERE c.user_id = $1
			  AND c.caught_at < now() - ($2 * interval '1 day')
			  AND c.inventory_instance_id IS NULL
			  AND NOT EXISTS (
				SELECT 1 FROM fishing_records r
				WHERE r.user_id = c.user_id
				  AND (r.best_length_catch_id = c.id OR r.best_weight_catch_id = c.id)
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM fishing_tournament_entries e WHERE e.catch_id = c.id
			  )
			ORDER BY c.id
			LIMIT 200
		)
		DELETE FROM fishing_catches c
		USING expired
		WHERE c.id = expired.id
	`, userID, FishingCatchHistoryDays)
	return err
}

func (s *Store) FishingJournal(ctx context.Context, userID int64) ([]FishingKnowledgeEntry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT k.species_id, k.encounters, k.catches, k.releases, k.first_caught_at, k.last_caught_at,
		       COALESCE(r.best_length_cm, 0), COALESCE(r.best_weight_kg, 0)
		FROM fishing_species_knowledge k
		LEFT JOIN fishing_records r
		  ON r.user_id = k.user_id AND r.species_id = k.species_id AND r.ruleset_version = $2
		WHERE k.user_id = $1
		ORDER BY k.first_caught_at NULLS LAST, k.species_id
	`, userID, FishingRulesetVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingKnowledgeEntry, 0, len(s.gameData.FishingSpecies))
	for rows.Next() {
		var row FishingKnowledgeEntry
		if err := rows.Scan(&row.SpeciesID, &row.Encounters, &row.Catches, &row.Releases, &row.FirstCaughtAt, &row.LastCaughtAt, &row.BestLengthCM, &row.BestWeightKG); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) FishingLeaderboard(ctx context.Context, speciesID, metric string, limit int) ([]FishingLeaderboardRow, error) {
	if s.gameData.FishingSpeciesByID(speciesID) == nil {
		return nil, ErrFishingCatchNotFound
	}
	metric = strings.ToLower(strings.TrimSpace(metric))
	if metric != "length" && metric != "weight" {
		return nil, ErrFishingArgsInvalid
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	column := "best_weight_kg"
	if metric == "length" {
		column = "best_length_cm"
	}
	query := fmt.Sprintf(`
		SELECT ROW_NUMBER() OVER (ORDER BY r.%s DESC, r.updated_at ASC, r.user_id ASC),
		       r.user_id, COALESCE(u.username, ''), r.species_id, r.%s
		FROM fishing_records r
		JOIN users u ON u.id = r.user_id
		WHERE r.species_id = $1 AND r.ruleset_version = $2 AND r.%s > 0
		ORDER BY r.%s DESC, r.updated_at ASC, r.user_id ASC
		LIMIT $3
	`, column, column, column, column)
	rows, err := s.db.Query(ctx, query, strings.TrimSpace(speciesID), FishingRulesetVersion, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingLeaderboardRow, 0, limit)
	for rows.Next() {
		var row FishingLeaderboardRow
		if err := rows.Scan(&row.Rank, &row.UserID, &row.PlayerName, &row.SpeciesID, &row.Value); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func fishingSaleIneligibility(species *catalog.FishingSpecies, profile *catalog.FishingPondSpecies, fish FishInstanceData) (string, int) {
	if species == nil || species.BaseValue <= 0 || species.MinLengthCM <= 0 || species.MaxLengthCM < species.MinLengthCM || species.ShapeFactor <= 0 || fish.AgeDays < 0 || fish.LengthCM <= 0 || fish.WeightKG <= 0 {
		return "invalid_data", 0
	}
	if normalizePondLifeState(fish.LifeState) != "alive" {
		return "dead", 0
	}
	if strings.EqualFold(strings.TrimSpace(fish.Origin), "cultured") {
		if profile == nil {
			return "invalid_data", 0
		}
		if fish.AgeDays < profile.MaturityAgeDays {
			return "cultured_immature", profile.MaturityAgeDays
		}
	}
	return "", 0
}

func (s *Store) QuoteFishingSale(ctx context.Context, userID int64, instanceIDs []int64, now time.Time) (FishingSaleQuote, error) {
	var quote FishingSaleQuote
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		quote, err = s.quoteFishingSaleTx(ctx, tx, userID, instanceIDs, now, false)
		return err
	})
	return quote, err
}

func (s *Store) SellFishingCatches(ctx context.Context, userID int64, instanceIDs []int64, expectedTotalGold int64, idempotencyKey string, now time.Time) (FishingSaleQuote, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 160 {
		return FishingSaleQuote{}, ErrFishingSaleInvalid
	}
	var quote FishingSaleQuote
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT quote_jsonb FROM fishing_npc_sales WHERE user_id=$1 AND idempotency_key=$2`, userID, key).Scan(&raw)
		if err == nil {
			if json.Unmarshal(raw, &quote) != nil {
				return ErrFishingSaleInvalid
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if expectedTotalGold <= 0 {
			return ErrFishingSaleInvalid
		}
		quote, err = s.quoteFishingSaleTx(ctx, tx, userID, instanceIDs, now, true)
		if err != nil {
			return err
		}
		if quote.TotalGold != expectedTotalGold {
			return ErrFishingSaleQuoteChanged
		}
		for _, line := range quote.Items {
			if err := s.LedgerSetStateTx(ctx, tx, line.InstanceID, userID, "inventory", "consumed", "fish_catch", 1, InventoryLedgerMeta{
				SourceSurface: "inventory", TargetSurface: "fishing_npc", Direction: InventoryAuditDirectionConsume,
				Details: map[string]any{"type": "fishing_npc_sale", "speciesId": line.SpeciesID, "idempotencyKey": key},
			}); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE player_item_instances SET archived_at=$3, updated_at=$3 WHERE user_id=$1 AND instance_id=$2`, userID, line.InstanceID, quote.QuoteAt); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE fishing_npc_supply
				SET sold_weight_kg=sold_weight_kg+$3, sold_count=sold_count+1, updated_at=$4
				WHERE market_date=$1 AND species_id=$2
			`, quote.MarketDate, line.SpeciesID, line.WeightKG, quote.QuoteAt); err != nil {
				return err
			}
		}
		if quote.TotalGold > 0 {
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, GoldCoinItemID, quote.TotalGold, InventoryLedgerMeta{
				SourceSurface: "fishing_npc", TargetSurface: "inventory", Direction: InventoryAuditDirectionGain,
				Details: map[string]any{"type": "fishing_npc_sale_gold", "idempotencyKey": key},
			}); err != nil {
				return err
			}
		}
		raw, err = json.Marshal(quote)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO fishing_npc_sales(user_id,idempotency_key,instance_ids,total_gold,quote_jsonb,created_at)
			VALUES($1,$2,$3,$4,$5::jsonb,$6)
		`, userID, key, uniquePositiveInt64(instanceIDs), quote.TotalGold, string(raw), quote.QuoteAt)
		return err
	})
	return quote, err
}

func (s *Store) quoteFishingSaleTx(ctx context.Context, tx pgx.Tx, userID int64, instanceIDs []int64, now time.Time, lock bool) (FishingSaleQuote, error) {
	ids := uniquePositiveInt64(instanceIDs)
	if len(ids) == 0 || len(ids) > FishingSaleBatchMax || len(ids) != len(instanceIDs) {
		return FishingSaleQuote{}, ErrFishingSaleInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	marketDate := now.UTC().Format("2006-01-02")
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	rows, err := tx.Query(ctx, `
		SELECT instance_id, fish_data
		FROM player_item_instances
		WHERE user_id = $1 AND instance_id = ANY($2) AND state = 'inventory' AND fish_data <> '{}'::jsonb
		ORDER BY instance_id
	`+lockClause, userID, ids)
	if err != nil {
		return FishingSaleQuote{}, err
	}
	type selectedFish struct {
		instanceID int64
		fish       FishInstanceData
		species    *catalog.FishingSpecies
	}
	selected := make([]selectedFish, 0, len(ids))
	for rows.Next() {
		var instanceID int64
		var raw []byte
		if err := rows.Scan(&instanceID, &raw); err != nil {
			rows.Close()
			return FishingSaleQuote{}, err
		}
		var fish FishInstanceData
		if err := json.Unmarshal(raw, &fish); err != nil {
			rows.Close()
			return FishingSaleQuote{}, ErrFishingSaleInvalid
		}
		species := s.gameData.FishingSpeciesByID(fish.SpeciesID)
		profile := s.gameData.FishingPondSpeciesByID(fish.SpeciesID)
		if reason, _ := fishingSaleIneligibility(species, profile, fish); reason != "" {
			rows.Close()
			return FishingSaleQuote{}, ErrFishingSaleInvalid
		}
		selected = append(selected, selectedFish{instanceID: instanceID, fish: fish, species: species})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return FishingSaleQuote{}, err
	}
	if len(selected) != len(ids) {
		return FishingSaleQuote{}, ErrFishingCatchNotFound
	}

	speciesIDs := make([]string, 0, len(selected))
	seenSpecies := make(map[string]struct{}, len(selected))
	for _, row := range selected {
		if _, exists := seenSpecies[row.fish.SpeciesID]; exists {
			continue
		}
		seenSpecies[row.fish.SpeciesID] = struct{}{}
		speciesIDs = append(speciesIDs, row.fish.SpeciesID)
	}
	sort.Strings(speciesIDs)
	soldWeightBySpecies := make(map[string]float64, len(speciesIDs))
	for _, speciesID := range speciesIDs {
		if lock {
			if _, err := tx.Exec(ctx, `
				INSERT INTO fishing_npc_supply(market_date, species_id, sold_weight_kg, sold_count, updated_at)
				VALUES($1, $2, 0, 0, now())
				ON CONFLICT (market_date, species_id) DO NOTHING
			`, marketDate, speciesID); err != nil {
				return FishingSaleQuote{}, err
			}
		}
		query := `
			SELECT COALESCE(sold_weight_kg, 0)
			FROM fishing_npc_supply
			WHERE market_date = $1 AND species_id = $2`
		if lock {
			query += ` FOR UPDATE`
		}
		var soldWeight float64
		if err := tx.QueryRow(ctx, query, marketDate, speciesID).Scan(&soldWeight); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return FishingSaleQuote{}, err
			}
			soldWeight = 0
		}
		soldWeightBySpecies[speciesID] = soldWeight
	}

	quote := FishingSaleQuote{QuoteAt: now, MarketDate: marketDate, Items: make([]FishingPriceLine, 0, len(ids))}
	batchWeightBySpecies := make(map[string]float64, len(speciesIDs))
	for _, row := range selected {
		fish := row.fish
		species := row.species
		soldWeight := soldWeightBySpecies[fish.SpeciesID] + batchWeightBySpecies[fish.SpeciesID]
		demand := fishingDailyDemand(fish.SpeciesID, marketDate)
		supply := clampFloat(1.06-soldWeight/1500, 0.93, 1.06)
		factors := fishingSaleSpecimenFactors(*species, fish)
		marketMultiplier := fishingSaleMarketMultiplier(demand, supply)
		culture := fishingSaleCultureFactor(*species, fish)
		mutationMultiplier := s.fishingMutationSaleValueMultiplier(fish.Genes.Mutations)
		gold := fishingSaleFinalGold(species.BaseValue, factors.SpecimenMultiplier, mutationMultiplier, marketMultiplier, culture)
		line := FishingPriceLine{
			InstanceID: row.instanceID, SpeciesID: fish.SpeciesID, AgeDays: fish.AgeDays,
			LengthCM: fish.LengthCM, WeightKG: fish.WeightKG, Mutations: append([]string(nil), fish.Genes.Mutations...), BaseValue: species.BaseValue,
			AgeFactor: roundFloat(factors.AgeFactor, 3), LengthFactor: roundFloat(factors.LengthFactor, 3),
			WeightFactor: roundFloat(factors.WeightFactor, 3), QualityFactor: roundFloat(factors.QualityFactor, 3),
			SpecimenMultiplier: roundFloat(factors.SpecimenMultiplier, 3),
			MutationMultiplier: roundFloat(mutationMultiplier, 3),
			DailyDemand:        roundFloat(demand, 3), SupplyFactor: roundFloat(supply, 3), CultureFactor: culture,
			MarketMultiplier: roundFloat(marketMultiplier, 3), FinalGold: gold,
		}
		quote.Items = append(quote.Items, line)
		quote.TotalGold += gold
		batchWeightBySpecies[fish.SpeciesID] += fish.WeightKG
	}
	return quote, nil
}

func fishingSaleFinalGold(baseValue, specimenMultiplier, mutationMultiplier, marketMultiplier, cultureFactor float64) int64 {
	gold := int64(math.Round(baseValue * specimenMultiplier * mutationMultiplier * marketMultiplier * cultureFactor))
	if gold < 1 {
		return 1
	}
	return gold
}

func fishingDailyDemand(speciesID, marketDate string) float64 {
	h := stableFishingHash(speciesID + ":" + marketDate + ":demand")
	return 0.96 + float64(h%81)/1000
}

type fishingSaleFactorBreakdown struct {
	AgeFactor          float64
	LengthFactor       float64
	WeightFactor       float64
	QualityFactor      float64
	SpecimenMultiplier float64
}

func fishingSaleReferenceMeasurements(species catalog.FishingSpecies) (float64, float64) {
	referenceLength := species.MinLengthCM + (species.MaxLengthCM-species.MinLengthCM)*fishingSaleReferenceLengthRatio
	referenceLength = math.Max(0.1, referenceLength)
	referenceWeight := species.ShapeFactor * math.Pow(referenceLength, 3) / 100_000
	return referenceLength, math.Max(0.001, referenceWeight)
}

func fishingSaleSpecimenFactors(species catalog.FishingSpecies, fish FishInstanceData) fishingSaleFactorBreakdown {
	referenceLength, referenceWeight := fishingSaleReferenceMeasurements(species)
	ageFactor := clampFloat(math.Pow(math.Max(1, float64(fish.AgeDays))/fishingSaleReferenceAgeDays, fishingSaleAgeExponent), 0.90, 1.10)
	lengthFactor := clampFloat(math.Pow(math.Max(0.1, fish.LengthCM)/referenceLength, fishingSaleLengthExponent), 0.75, 1.30)
	weightFactor := clampFloat(math.Pow(math.Max(0.001, fish.WeightKG)/referenceWeight, fishingSaleWeightExponent), 0.80, 1.25)
	qualityFactor := clampFloat(0.78+fish.Condition*0.15+fish.Health*0.07, 0.85, 1.03)
	return fishingSaleFactorBreakdown{
		AgeFactor: ageFactor, LengthFactor: lengthFactor, WeightFactor: weightFactor, QualityFactor: qualityFactor,
		SpecimenMultiplier: clampFloat(ageFactor*lengthFactor*weightFactor*qualityFactor, fishingSaleSpecimenMinimum, fishingSaleSpecimenMaximum),
	}
}

func fishingSaleMarketMultiplier(demand, supply float64) float64 {
	return clampFloat(demand*supply, fishingSaleMarketMinimum, fishingSaleMarketMaximum)
}

func fishingSaleCultureFactor(species catalog.FishingSpecies, fish FishInstanceData) float64 {
	if !strings.EqualFold(strings.TrimSpace(fish.Origin), "cultured") {
		return 1
	}
	weightLimitedFactor := fishingCulturedValuePerKGCap * math.Max(0, fish.WeightKG) / math.Max(1, species.BaseValue)
	return roundFloat(clampFloat(weightLimitedFactor, 0, fishingCulturedSaleFactor), 4)
}

func uniquePositiveInt64(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func newFishingID(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}

func stableFishingHash(value string) uint64 {
	const offset = uint64(1469598103934665603)
	const prime = uint64(1099511628211)
	h := offset
	for i := 0; i < len(value); i++ {
		h ^= uint64(value[i])
		h *= prime
	}
	return h
}
