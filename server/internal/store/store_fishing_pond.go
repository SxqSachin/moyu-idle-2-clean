package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	FishingPondBuildingID             = "fish_pond"
	FishingPondRulesetVersion         = "fishing-pond-v5"
	fishingPondPreviousRulesetVersion = "fishing-pond-v4"
	fishingPondState                  = "fishing_pond"
	fishingPondZoneMain               = "main"
	fishingPondZoneNursery            = "nursery"
	fishingPondMaxOfflineHours        = 30 * 24
	fishingPondBioDaysPerRealDay      = 365.0
	fishingPondNutritionPerKG         = 180.0
	fishingPondBreedFeedHours         = 1.0
	fishingPondBreedCooldown          = time.Hour
	fishingPondMaxEffectPortionsHour  = 0.5
	pondFeedRulesetVersion            = "pond-feed-v2"
	pondScatterMutationInheritance    = 0.5
	pondBroodstockMashItemID          = "pond_broodstock_mash"
	pondLineageReagentItemID          = "pond_lineage_reagent"
	pondFishmealItemID                = "pond_fishmeal"
)

type FishingPondLimits struct {
	FishCapacity    int64   `json:"fishCapacity"`
	NurseryCapacity int64   `json:"nurseryCapacity"`
	BiomassKG       float64 `json:"biomassKg"`
	BreedingPairs   int64   `json:"breedingPairs"`
	FeederNutrition float64 `json:"feederNutrition"`
}

type FishingPondFeedView struct {
	Delivery               string                      `json:"delivery,omitempty"`
	Role                   string                      `json:"role"`
	MaxPortionsPerFishHour float64                     `json:"maxPortionsPerFishHour,omitempty"`
	StoredPortions         float64                     `json:"storedPortions"`
	Paused                 bool                        `json:"paused"`
	ItemID                 string                      `json:"itemId"`
	FeedClass              string                      `json:"feedClass"`
	Nutrition              float64                     `json:"nutrition"`
	RulesetVersion         string                      `json:"rulesetVersion,omitempty"`
	ConsumePriority        int                         `json:"consumePriority,omitempty"`
	Conditions             []catalog.PondFeedCondition `json:"conditions,omitempty"`
	Effects                []catalog.PondFeedEffect    `json:"effects,omitempty"`
	InventoryCount         int64                       `json:"inventoryCount"`
	StoredNutrition        float64                     `json:"storedNutrition"`
}

type FishingPondFishView struct {
	NaturalProtectionHours float64                       `json:"naturalProtectionHours,omitempty"`
	CareRevision           int64                         `json:"careRevision"`
	InstanceID             int64                         `json:"instanceId"`
	CustomData             ItemCustomization             `json:"customData,omitempty"`
	Fish                   FishInstanceData              `json:"fish"`
	DietClass              string                        `json:"dietClass"`
	SecondaryFeedClass     []string                      `json:"secondaryFeedClasses,omitempty"`
	MaturityAgeDays        int                           `json:"maturityAgeDays"`
	ExpectedLifespanDay    int                           `json:"expectedLifespanDays"`
	Mature                 bool                          `json:"mature"`
	MaturityProgress       float64                       `json:"maturityProgress"`
	MaturityRealHours      float64                       `json:"maturityRealHoursRemaining"`
	Phenotypes             []string                      `json:"phenotypes,omitempty"`
	PhenotypeEffects       []FishingPondPhenotypeEffect  `json:"phenotypeEffects,omitempty"`
	Predation              *catalog.FishingPondPredation `json:"predation,omitempty"`
}

type FishingPondPhenotypeEffect struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

type FishingPondPairView struct {
	SlotIndex        int64 `json:"slotIndex"`
	FemaleInstanceID int64 `json:"femaleInstanceId"`
	MaleInstanceID   int64 `json:"maleInstanceId"`
	MashReady        bool  `json:"mashReady"`
}

type FishingPondEventView struct {
	ID                int64          `json:"id"`
	Kind              string         `json:"kind"`
	FishInstanceID    *int64         `json:"fishInstanceId,omitempty"`
	RelatedInstanceID *int64         `json:"relatedInstanceId,omitempty"`
	Details           map[string]any `json:"details,omitempty"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type FishingPondOverview struct {
	Built                   bool                             `json:"built"`
	Level                   int64                            `json:"level"`
	RulesetVersion          string                           `json:"rulesetVersion"`
	BioDaysPerRealDay       float64                          `json:"bioDaysPerRealDay"`
	MaxOfflineHours         int                              `json:"maxOfflineHours"`
	Limits                  FishingPondLimits                `json:"limits"`
	Fish                    []FishingPondFishView            `json:"fish"`
	AvailableFish           []FishingPondFishView            `json:"availableFish"`
	AvailableFishNextCursor int64                            `json:"availableFishNextCursor,omitempty"`
	Pairs                   []FishingPondPairView            `json:"pairs"`
	Feeds                   []FishingPondFeedView            `json:"feeds"`
	FeedTotalNutrition      float64                          `json:"feedTotalNutrition"`
	BiomassKG               float64                          `json:"biomassKg"`
	Events                  []FishingPondEventView           `json:"events"`
	LineageReagentCount     int64                            `json:"lineageReagentCount"`
	BroodstockMashCount     int64                            `json:"broodstockMashCount"`
	LastSettledAt           time.Time                        `json:"lastSettledAt"`
	Mutations               []config.FishingMutationSettings `json:"mutations"`
}

type fishingPondFishRow struct {
	InstanceID int64
	CustomData ItemCustomization
	Fish       FishInstanceData
}

type fishingPondPairRow struct {
	SlotIndex        int64
	FemaleInstanceID int64
	MaleInstanceID   int64
	MashReady        bool
}

type fishingPondStateRow struct {
	Paused         map[string]bool
	FeedStock      map[string]float64
	LastSettledAt  time.Time
	RulesetVersion string
}

type fishingPondFeedDose struct {
	ItemID   string
	Feed     *catalog.PondFeedConfig
	Portions float64
}

type fishingPondNutritionResult struct {
	Effective float64
	Preferred bool
	Doses     []fishingPondFeedDose
}

type fishingPondBreedingPolicy struct {
	Capacity         int64
	ParentZone       string
	NewbornZone      string
	NewbornRuleset   string
	EnforceBiomass   bool
	CountAllPondFish bool
}

type fishingPondPredationEvent struct {
	PredatorInstanceID int64
	PreyInstanceID     int64
	PredatorSpeciesID  string
	PreySpeciesID      string
	PredatorName       string
	PreyName           string
	PredatorLengthCM   float64
	PreyLengthCM       float64
	PreyWeightKG       float64
}

func (s *Store) GetFishingPond(ctx context.Context, userID int64, now time.Time) (FishingPondOverview, error) {
	var out FishingPondOverview
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, FishingPondBuildingID)
		if err != nil {
			return err
		}
		out = FishingPondOverview{Built: level > 0, Level: level, RulesetVersion: FishingPondRulesetVersion, BioDaysPerRealDay: fishingPondBioDaysPerRealDay, MaxOfflineHours: fishingPondMaxOfflineHours, Mutations: s.FishingMutationCatalog()}
		if level <= 0 {
			return nil
		}
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := s.settleFishingPondTx(ctx, tx, userID, level, now); err != nil {
			return err
		}
		out, err = s.fishingPondOverviewTx(ctx, tx, userID, level)
		return err
	})
	return out, err
}

func (s *Store) AddFishingPondFish(ctx context.Context, userID, instanceID int64, now time.Time) (FishingPondOverview, error) {
	return s.AddFishingPondFishBatch(ctx, userID, []int64{instanceID}, now)
}

func (s *Store) AddFishingPondFishBatch(ctx context.Context, userID int64, instanceIDs []int64, now time.Time) (FishingPondOverview, error) {
	normalizedIDs, err := normalizeFishingPondInstanceIDs(instanceIDs)
	if err != nil {
		return FishingPondOverview{}, err
	}
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, level int64, limits FishingPondLimits) error {
		fishRows, err := s.listFishingPondFishTx(ctx, tx, userID, true)
		if err != nil {
			return err
		}
		if fishingPondZoneOccupiedCount(fishRows, fishingPondZoneMain)+int64(len(normalizedIDs)) > limits.FishCapacity {
			return ErrFishingPondFull
		}
		rows, err := tx.Query(ctx, `
			SELECT instance_id, COALESCE(fish_data, '{}'::jsonb)
			FROM player_item_instances
			WHERE user_id=$1 AND instance_id = ANY($2) AND state='inventory'
			  AND template_item_id='fish_catch' AND quantity=1
			ORDER BY instance_id
			FOR UPDATE
		`, userID, normalizedIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		selected := make([]fishingPondFishRow, 0, len(normalizedIDs))
		for rows.Next() {
			var row fishingPondFishRow
			var raw []byte
			if err := rows.Scan(&row.InstanceID, &raw); err != nil {
				return err
			}
			if json.Unmarshal(raw, &row.Fish) != nil || strings.TrimSpace(row.Fish.SpeciesID) == "" {
				return ErrFishingPondFishInvalid
			}
			selected = append(selected, row)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(selected) != len(normalizedIDs) {
			return ErrFishingPondFishInvalid
		}
		biomass := fishingPondMainBiomass(fishRows)
		for _, row := range selected {
			species := s.gameData.FishingSpeciesByID(row.Fish.SpeciesID)
			if normalizePondLifeState(row.Fish.LifeState) != "alive" || s.gameData.FishingPondSpeciesByID(row.Fish.SpeciesID) == nil || !fishingPondFishSizeValid(row.Fish, species) {
				return ErrFishingPondFishInvalid
			}
			biomass += row.Fish.WeightKG
		}
		if biomass > limits.BiomassKG+0.0001 {
			return ErrFishingPondBiomassFull
		}
		for _, row := range selected {
			fish := row.Fish
			s.initializeFishingPondFish(&fish, userID, row.InstanceID, now, s.gameData.FishingPondSpeciesByID(fish.SpeciesID))
			fish.Origin = "cultured"
			fish.PondZone = fishingPondZoneMain
			s.clampFishingPondFishSize(&fish, s.gameData.FishingSpeciesByID(fish.SpeciesID))
			raw, err := json.Marshal(fish)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE player_item_instances SET fish_data=$3::jsonb, updated_at=$4 WHERE user_id=$1 AND instance_id=$2`, userID, row.InstanceID, string(raw), now); err != nil {
				return err
			}
			if err = s.LedgerSetStateTx(ctx, tx, row.InstanceID, userID, "inventory", fishingPondState, "fish_catch", 1, pondLedgerMeta("inventory", "fishing_pond", InventoryAuditDirectionConsume, "pond_stock")); err != nil {
				return err
			}
			fishID := row.InstanceID
			if err = s.addFishingPondEventTx(ctx, tx, userID, "stocked", &fishID, nil, map[string]any{"speciesId": fish.SpeciesID}); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeFishingPondInstanceIDs(instanceIDs []int64) ([]int64, error) {
	if len(instanceIDs) == 0 || len(instanceIDs) > 200 {
		return nil, ErrFishingPondFishInvalid
	}
	seen := make(map[int64]struct{}, len(instanceIDs))
	out := append([]int64(nil), instanceIDs...)
	for _, instanceID := range out {
		if instanceID <= 0 {
			return nil, ErrFishingPondFishInvalid
		}
		if _, exists := seen[instanceID]; exists {
			return nil, ErrFishingPondFishInvalid
		}
		seen[instanceID] = struct{}{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (s *Store) RemoveFishingPondFish(ctx context.Context, userID, instanceID int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		fish, err := s.lockFishingPondFishTx(ctx, tx, userID, instanceID)
		if err != nil {
			return err
		}
		if normalizePondLifeState(fish.LifeState) != "alive" {
			return ErrFishingPondFishInvalid
		}
		if _, err := tx.Exec(ctx, `DELETE FROM fishing_pond_pairs WHERE user_id=$1 AND (female_instance_id=$2 OR male_instance_id=$2)`, userID, instanceID); err != nil {
			return err
		}
		if err := s.LedgerSetStateTx(ctx, tx, instanceID, userID, fishingPondState, "inventory", "fish_catch", 1, pondLedgerMeta("fishing_pond", "inventory", InventoryAuditDirectionGain, "pond_remove")); err != nil {
			return err
		}
		s.clampFishingPondFishSize(&fish, s.gameData.FishingSpeciesByID(fish.SpeciesID))
		fish.PondZone = ""
		raw, err := json.Marshal(fish)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE player_item_instances SET fish_data=$3::jsonb, updated_at=$4 WHERE user_id=$1 AND instance_id=$2 AND state='inventory'`, userID, instanceID, string(raw), now); err != nil {
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "removed", &instanceID, nil, map[string]any{"speciesId": fish.SpeciesID})
	})
}

func (s *Store) MoveFishingPondFishToMain(ctx context.Context, userID, instanceID int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, limits FishingPondLimits) error {
		rows, err := s.listFishingPondFishTx(ctx, tx, userID, true)
		if err != nil {
			return err
		}
		var target *fishingPondFishRow
		for index := range rows {
			if rows[index].InstanceID == instanceID {
				target = &rows[index]
				break
			}
		}
		if target == nil || normalizePondLifeState(target.Fish.LifeState) != "alive" || normalizeFishingPondZone(target.Fish.PondZone) != fishingPondZoneNursery {
			return ErrFishingPondFishInvalid
		}
		if fishingPondZoneOccupiedCount(rows, fishingPondZoneMain)+1 > limits.FishCapacity {
			return ErrFishingPondFull
		}
		if fishingPondMainBiomass(rows)+target.Fish.WeightKG > limits.BiomassKG+0.0001 {
			return ErrFishingPondBiomassFull
		}
		target.Fish.PondZone = fishingPondZoneMain
		if err := s.updateFishingPondFishTx(ctx, tx, userID, instanceID, target.Fish, now); err != nil {
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "moved_to_main", &instanceID, nil, map[string]any{"speciesId": target.Fish.SpeciesID})
	})
}

func (s *Store) AddFishingPondFeed(ctx context.Context, userID int64, itemID string, quantity int64, now time.Time) (FishingPondOverview, error) {
	itemID = strings.TrimSpace(itemID)
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, limits FishingPondLimits) error {
		item := s.gameData.ItemByID(itemID)
		if item == nil || item.Disabled || item.PondFeed == nil || item.PondFeed.Delivery == "targeted" || item.PondFeed.Nutrition <= 0 || quantity <= 0 || quantity > 10000 {
			return ErrFishingPondFeedInvalid
		}
		state, err := s.lockFishingPondStateTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		feedStock := state.FeedStock
		added := item.PondFeed.Nutrition * float64(quantity)
		if pondFeedTotal(feedStock)+added > limits.FeederNutrition+0.0001 {
			return ErrFishingPondFeederFull
		}
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, quantity, pondLedgerMeta("inventory", "fishing_pond", InventoryAuditDirectionConsume, "pond_feed")); err != nil {
			return err
		}
		feedStock[itemID] = math.Max(0, feedStock[itemID]) + added
		if err := s.updateFishingPondFeedStockTx(ctx, tx, userID, feedStock, now, state.Paused); err != nil {
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "fed", nil, nil, map[string]any{"itemId": itemID, "quantity": quantity, "nutrition": added})
	})
}

func (s *Store) SetFishingPondPair(ctx context.Context, userID, femaleInstanceID, maleInstanceID int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, limits FishingPondLimits) error {
		female, err := s.lockFishingPondFishTx(ctx, tx, userID, femaleInstanceID)
		if err != nil {
			return err
		}
		male, err := s.lockFishingPondFishTx(ctx, tx, userID, maleInstanceID)
		if err != nil {
			return err
		}
		if femaleInstanceID == maleInstanceID || female.Sex != "female" || male.Sex != "male" || female.SpeciesID != male.SpeciesID || normalizePondLifeState(female.LifeState) != "alive" || normalizePondLifeState(male.LifeState) != "alive" || normalizeFishingPondZone(female.PondZone) != fishingPondZoneMain || normalizeFishingPondZone(male.PondZone) != fishingPondZoneMain {
			return ErrFishingPondPairInvalid
		}
		profile := s.gameData.FishingPondSpeciesByID(female.SpeciesID)
		species := s.gameData.FishingSpeciesByID(female.SpeciesID)
		if profile == nil {
			return ErrFishingPondPairInvalid
		}
		maturityAgeDays := profile.MaturityAgeDays
		if species != nil && species.MaturityAgeDays > 0 {
			maturityAgeDays = species.MaturityAgeDays
		}
		if maturityAgeDays <= 0 || female.AgeDays < maturityAgeDays || male.AgeDays < maturityAgeDays {
			return ErrFishingPondPairInvalid
		}
		var count int64
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM fishing_pond_pairs WHERE user_id=$1`, userID).Scan(&count); err != nil {
			return err
		}
		if count >= limits.BreedingPairs {
			return ErrFishingPondPairFull
		}
		var slot int64
		for slot = 0; slot < limits.BreedingPairs; slot++ {
			var occupied bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fishing_pond_pairs WHERE user_id=$1 AND slot_index=$2)`, userID, slot).Scan(&occupied); err != nil {
				return err
			}
			if !occupied {
				break
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO fishing_pond_pairs(user_id, slot_index, female_instance_id, male_instance_id, mash_ready, updated_at)
			VALUES($1,$2,$3,$4,FALSE,$5)
		`, userID, slot, femaleInstanceID, maleInstanceID, now)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23514" || pgErr.Code == "23503") {
				return ErrFishingPondPairInvalid
			}
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "pair_set", &femaleInstanceID, &maleInstanceID, map[string]any{"slotIndex": slot})
	})
}

func (s *Store) DeleteFishingPondPair(ctx context.Context, userID, slotIndex int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		result, err := tx.Exec(ctx, `DELETE FROM fishing_pond_pairs WHERE user_id=$1 AND slot_index=$2`, userID, slotIndex)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrFishingPondPairInvalid
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "pair_removed", nil, nil, map[string]any{"slotIndex": slotIndex})
	})
}

func (s *Store) ApplyFishingPondMash(ctx context.Context, userID, slotIndex int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, pondBroodstockMashItemID, 1, pondLedgerMeta("inventory", "fishing_pond", InventoryAuditDirectionConsume, "pond_broodstock_mash")); err != nil {
			if errors.Is(err, ErrInsufficientItem) {
				return ErrFishingPondBroodstockMashMissing
			}
			return err
		}
		result, err := tx.Exec(ctx, `UPDATE fishing_pond_pairs SET mash_ready=TRUE, updated_at=$3 WHERE user_id=$1 AND slot_index=$2 AND mash_ready=FALSE`, userID, slotIndex, now)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrFishingPondPairInvalid
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "mash_applied", nil, nil, map[string]any{"slotIndex": slotIndex})
	})
}

func (s *Store) RevealFishingPondGenome(ctx context.Context, userID, instanceID int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		fish, err := s.lockFishingPondFishTx(ctx, tx, userID, instanceID)
		if err != nil {
			return err
		}
		if fish.GenomeRevealed {
			return nil
		}
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, pondLineageReagentItemID, 1, pondLedgerMeta("inventory", "fishing_pond", InventoryAuditDirectionConsume, "pond_genome_reveal")); err != nil {
			if errors.Is(err, ErrInsufficientItem) {
				return ErrFishingPondLineageReagentMissing
			}
			return err
		}
		fish.GenomeRevealed = true
		if err := s.updateFishingPondFishTx(ctx, tx, userID, instanceID, fish, now); err != nil {
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "genome_revealed", &instanceID, nil, map[string]any{"speciesId": fish.SpeciesID})
	})
}

func (s *Store) ProcessFishingPondFish(ctx context.Context, userID, instanceID int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		fish, err := s.lockFishingPondFishTx(ctx, tx, userID, instanceID)
		if err != nil {
			return err
		}
		if normalizePondLifeState(fish.LifeState) != "dead" {
			return ErrFishingPondFishInvalid
		}
		yield := int64(math.Floor(math.Max(0.5, fish.WeightKG) * 2))
		if yield < 1 {
			yield = 1
		}
		if yield > 200 {
			yield = 200
		}
		if _, err := tx.Exec(ctx, `DELETE FROM fishing_pond_pairs WHERE user_id=$1 AND (female_instance_id=$2 OR male_instance_id=$2)`, userID, instanceID); err != nil {
			return err
		}
		if err := s.LedgerConsumeInstanceTx(ctx, tx, userID, instanceID, "fish_catch", 1, pondLedgerMeta("fishing_pond", "processing", InventoryAuditDirectionConsume, "pond_corpse_process")); err != nil {
			return err
		}
		if err := s.LedgerGrantTemplateTx(ctx, tx, userID, pondFishmealItemID, yield, pondLedgerMeta("processing", "inventory", InventoryAuditDirectionGain, "pond_fishmeal")); err != nil {
			return err
		}
		return s.addFishingPondEventTx(ctx, tx, userID, "processed", &instanceID, nil, map[string]any{"speciesId": fish.SpeciesID, "fishmeal": yield})
	})
}

func (s *Store) mutateFishingPond(ctx context.Context, userID int64, now time.Time, mutation func(pgx.Tx, int64, FishingPondLimits) error) (FishingPondOverview, error) {
	var out FishingPondOverview
	if now.IsZero() {
		now = time.Now()
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, FishingPondBuildingID)
		if err != nil {
			return err
		}
		if level <= 0 {
			return ErrFishingPondNotBuilt
		}
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		limits := s.fishingPondLimits(level)
		if _, err := s.settleFishingPondTx(ctx, tx, userID, level, now); err != nil {
			return err
		}
		if err := mutation(tx, level, limits); err != nil {
			return err
		}
		out, err = s.fishingPondOverviewTx(ctx, tx, userID, level)
		return err
	})
	return out, err
}

func (s *Store) fishingPondLimits(level int64) FishingPondLimits {
	bonuses := buildingLevelBonuses(s.gameData.BuildingDefinitionByID(FishingPondBuildingID), level)
	return FishingPondLimits{
		FishCapacity: bonuses["fishCapacity"], NurseryCapacity: bonuses["nurseryCapacity"], BiomassKG: float64(bonuses["biomassKg"]),
		BreedingPairs: bonuses["breedingPairs"], FeederNutrition: float64(bonuses["feederNutrition"]),
	}
}

func (s *Store) fishingPondOverviewTx(ctx context.Context, tx pgx.Tx, userID, level int64) (FishingPondOverview, error) {
	limits := s.fishingPondLimits(level)
	state, err := s.getFishingPondStateTx(ctx, tx, userID)
	if err != nil {
		return FishingPondOverview{}, err
	}
	rows, err := s.listFishingPondFishTx(ctx, tx, userID, false)
	if err != nil {
		return FishingPondOverview{}, err
	}
	out := FishingPondOverview{
		Built: true, Level: level, RulesetVersion: state.RulesetVersion, BioDaysPerRealDay: fishingPondBioDaysPerRealDay, MaxOfflineHours: fishingPondMaxOfflineHours, Limits: limits,
		Fish: make([]FishingPondFishView, 0, len(rows)), AvailableFish: []FishingPondFishView{},
		Pairs: []FishingPondPairView{}, Feeds: []FishingPondFeedView{}, Events: []FishingPondEventView{},
		LastSettledAt: state.LastSettledAt, Mutations: s.FishingMutationCatalog(),
	}
	for _, row := range rows {
		view := s.fishingPondFishView(row)
		out.Fish = append(out.Fish, view)
		if view.Fish.PondZone == fishingPondZoneMain && normalizePondLifeState(view.Fish.LifeState) == "alive" {
			out.BiomassKG += view.Fish.WeightKG
		}
	}
	out.BiomassKG = roundFloat(out.BiomassKG, 3)
	available, err := s.listAvailablePondFishPage(ctx, tx, userID, 0)
	if err != nil {
		return FishingPondOverview{}, err
	}
	out.AvailableFish = available.Items
	out.AvailableFishNextCursor = available.NextCursor
	pairs, err := s.listFishingPondPairsTx(ctx, tx, userID)
	if err != nil {
		return FishingPondOverview{}, err
	}
	for _, pair := range pairs {
		out.Pairs = append(out.Pairs, FishingPondPairView(pair))
	}
	counts, err := s.pondFeedInventoryCountsTx(ctx, tx, userID)
	if err != nil {
		return FishingPondOverview{}, err
	}
	out.LineageReagentCount = counts[pondLineageReagentItemID]
	out.BroodstockMashCount = counts[pondBroodstockMashItemID]
	for _, item := range s.gameData.Items {
		if item.PondFeed == nil || item.PondFeed.Nutrition <= 0 && item.PondFeed.Delivery != "targeted" {
			continue
		}
		stored := roundFloat(state.FeedStock[item.ID], 3)
		storedPortions := 0.0
		if item.PondFeed.Nutrition > 0 {
			storedPortions = state.FeedStock[item.ID] / item.PondFeed.Nutrition
		}
		out.Feeds = append(out.Feeds, FishingPondFeedView{
			Delivery: item.PondFeed.Delivery,
			Role:     pondFeedRole(item.PondFeed), MaxPortionsPerFishHour: pondFeedDoseLimit(item.PondFeed),
			StoredPortions: storedPortions, Paused: state.Paused[item.ID],
			ItemID: item.ID, FeedClass: item.PondFeed.FeedClass, Nutrition: item.PondFeed.Nutrition,
			RulesetVersion: item.PondFeed.RulesetVersion, ConsumePriority: item.PondFeed.ConsumePriority,
			Conditions:     append([]catalog.PondFeedCondition(nil), item.PondFeed.Conditions...),
			Effects:        append([]catalog.PondFeedEffect(nil), item.PondFeed.Effects...),
			InventoryCount: counts[item.ID], StoredNutrition: stored,
		})
		out.FeedTotalNutrition += stored
	}
	feedOrder := map[string]int{"herbal": 0, "insect": 1, "protein": 2, "arcane": 3, "recycled": 4}
	sort.Slice(out.Feeds, func(i, j int) bool {
		left, leftKnown := feedOrder[out.Feeds[i].FeedClass]
		right, rightKnown := feedOrder[out.Feeds[j].FeedClass]
		if leftKnown && rightKnown && left != right {
			return left < right
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return out.Feeds[i].ItemID < out.Feeds[j].ItemID
	})
	out.FeedTotalNutrition = roundFloat(out.FeedTotalNutrition, 3)
	events, err := s.listFishingPondEventsTx(ctx, tx, userID, 30)
	if err != nil {
		return FishingPondOverview{}, err
	}
	out.Events = events
	return out, nil
}

func (s *Store) fishingPondFishView(row fishingPondFishRow) FishingPondFishView {
	profile := s.gameData.FishingPondSpeciesByID(row.Fish.SpeciesID)
	species := s.gameData.FishingSpeciesByID(row.Fish.SpeciesID)
	viewFish := row.Fish
	s.clampFishingPondFishSize(&viewFish, species)
	viewFish.Sex = normalizedPondSex(viewFish.Sex, row.InstanceID, viewFish.SpeciesID)
	viewFish.PondZone = normalizeFishingPondZone(viewFish.PondZone)
	viewFish.LongevityLimitDays = 0
	viewFish.NaturalProtectionUntilAgeDays = 0
	viewFish.PondCareRevision = 0
	if !viewFish.GenomeRevealed {
		viewFish.Genes = FishGenes{Mutations: append([]string(nil), row.Fish.Genes.Mutations...)}
	}
	view := FishingPondFishView{
		NaturalProtectionHours: math.Max(0, row.Fish.NaturalProtectionUntilAgeDays-float64(row.Fish.AgeDays)-row.Fish.BioAgeFraction) * 24 / fishingPondBioDaysPerRealDay,
		CareRevision:           row.Fish.PondCareRevision,
		InstanceID:             row.InstanceID, CustomData: row.CustomData, Fish: viewFish,
		Phenotypes: pondPhenotypes(row.Fish.Genes), PhenotypeEffects: pondPhenotypeEffects(row.Fish.Genes),
	}
	if profile != nil {
		view.DietClass = profile.DietClass
		view.SecondaryFeedClass = append([]string(nil), profile.SecondaryFeedClasses...)
		view.MaturityAgeDays = profile.MaturityAgeDays
		view.ExpectedLifespanDay = profile.LifespanDays
		if viewFish.PondZone == fishingPondZoneMain && profile.Predation != nil {
			predation := *profile.Predation
			view.Predation = &predation
		}
	}
	if species != nil {
		if species.MaturityAgeDays > 0 {
			view.MaturityAgeDays = species.MaturityAgeDays
		}
		if species.LifespanDays > 0 {
			view.ExpectedLifespanDay = int(math.Round(float64(species.LifespanDays) * s.fishingMutationLifespanMultiplier(row.Fish.Genes.Mutations)))
		}
	}
	if view.MaturityAgeDays > 0 {
		view.Mature = row.Fish.AgeDays >= view.MaturityAgeDays
		view.MaturityProgress = roundFloat(clampFloat((float64(row.Fish.AgeDays)+row.Fish.BioAgeFraction)/float64(view.MaturityAgeDays), 0, 1), 4)
		remainingBioDays := math.Max(0, float64(view.MaturityAgeDays-row.Fish.AgeDays)-row.Fish.BioAgeFraction)
		view.MaturityRealHours = roundFloat(remainingBioDays/fishingPondBioDaysPerRealDay*24, 1)
	}
	return view
}

func (s *Store) settleFishingPondTx(ctx context.Context, tx pgx.Tx, userID, level int64, now time.Time) (time.Time, error) {
	if now.IsZero() {
		now = time.Now()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fishing_ponds(user_id, feed_stock_jsonb, ruleset_version, last_settled_at, created_at, updated_at)
		VALUES($1, '{}'::jsonb, $3, $2, $2, $2)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, now, FishingPondRulesetVersion); err != nil {
		return time.Time{}, err
	}
	state, err := s.lockFishingPondStateTx(ctx, tx, userID)
	if err != nil {
		return time.Time{}, err
	}
	if state.LastSettledAt.IsZero() || !now.After(state.LastSettledAt) {
		if err := s.promoteFishingPondRulesetTx(ctx, tx, userID, state.RulesetVersion, now); err != nil {
			return time.Time{}, err
		}
		return state.LastSettledAt, nil
	}
	cursor, settledAt, elapsedHours := fishingPondSettlementWindow(state.LastSettledAt, now)
	remainingDuration := settledAt.Sub(cursor) - time.Duration(elapsedHours)*time.Hour
	if elapsedHours <= 0 && remainingDuration <= 0 {
		if err := s.promoteFishingPondRulesetTx(ctx, tx, userID, state.RulesetVersion, now); err != nil {
			return time.Time{}, err
		}
		return state.LastSettledAt, nil
	}
	rows, err := s.listFishingPondFishTx(ctx, tx, userID, true)
	if err != nil {
		return time.Time{}, err
	}
	pairs, err := s.listFishingPondPairsTx(ctx, tx, userID)
	if err != nil {
		return time.Time{}, err
	}
	limits := s.fishingPondLimits(level)
	aliveBefore := make(map[int64]bool, len(rows))
	for _, row := range rows {
		aliveBefore[row.InstanceID] = normalizePondLifeState(row.Fish.LifeState) == "alive"
	}
	predationEvents := make([]fishingPondPredationEvent, 0)
	for range elapsedHours {
		cursor = cursor.Add(time.Hour)
		s.settleFishingPondFeedingStep(rows, state.FeedStock, state.Paused, userID, cursor, 1)
		predationEvents = append(predationEvents, s.settleFishingPondPredationStep(rows, userID, cursor, 1)...)
		if err := s.settleFishingPondBreedingStepTx(ctx, tx, userID, cursor, 1, state.RulesetVersion, limits, &rows, &pairs); err != nil {
			return time.Time{}, err
		}
	}
	if remainingDuration > 0 {
		cursor = cursor.Add(remainingDuration)
		stepHours := remainingDuration.Hours()
		s.settleFishingPondFeedingStep(rows, state.FeedStock, state.Paused, userID, cursor, stepHours)
		predationEvents = append(predationEvents, s.settleFishingPondPredationStep(rows, userID, cursor, stepHours)...)
		if err := s.settleFishingPondBreedingStepTx(ctx, tx, userID, cursor, stepHours, state.RulesetVersion, limits, &rows, &pairs); err != nil {
			return time.Time{}, err
		}
	}
	predatedIDs := make(map[int64]struct{}, len(predationEvents))
	for _, event := range predationEvents {
		predatedIDs[event.PreyInstanceID] = struct{}{}
	}
	for _, row := range rows {
		if _, predated := predatedIDs[row.InstanceID]; predated {
			continue
		}
		if err := s.updateFishingPondFishTx(ctx, tx, userID, row.InstanceID, row.Fish, now); err != nil {
			return time.Time{}, err
		}
		wasAlive, existedBefore := aliveBefore[row.InstanceID]
		if normalizePondLifeState(row.Fish.LifeState) == "dead" && (wasAlive || !existedBefore) {
			fishID := row.InstanceID
			if err := s.addFishingPondEventTx(ctx, tx, userID, "died", &fishID, nil, map[string]any{"speciesId": row.Fish.SpeciesID, "cause": row.Fish.DeathCause}); err != nil {
				return time.Time{}, err
			}
		}
	}
	for _, event := range predationEvents {
		meta := pondLedgerMetaForRuleset("fishing_pond", "pond_ecology", InventoryAuditDirectionConsume, "pond_predation", state.RulesetVersion)
		meta.Details["predatorInstanceId"] = event.PredatorInstanceID
		if err := s.LedgerConsumeInstanceTx(ctx, tx, userID, event.PreyInstanceID, "fish_catch", 1, meta); err != nil {
			return time.Time{}, err
		}
		predatorID := event.PredatorInstanceID
		preyID := event.PreyInstanceID
		details := map[string]any{
			"speciesId": event.PredatorSpeciesID, "predatorSpeciesId": event.PredatorSpeciesID, "preySpeciesId": event.PreySpeciesID,
			"predatorLengthCm": event.PredatorLengthCM, "preyLengthCm": event.PreyLengthCM, "preyWeightKg": event.PreyWeightKG,
		}
		if event.PredatorName != "" {
			details["predatorName"] = event.PredatorName
		}
		if event.PreyName != "" {
			details["preyName"] = event.PreyName
		}
		if err := s.addFishingPondEventTx(ctx, tx, userID, "predated", &predatorID, &preyID, details); err != nil {
			return time.Time{}, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM fishing_pond_pairs p USING player_item_instances i WHERE p.user_id=$1 AND (i.instance_id=p.female_instance_id OR i.instance_id=p.male_instance_id) AND (i.state<>$2 OR COALESCE(i.fish_data->>'lifeState','alive')='dead')`, userID, fishingPondState); err != nil {
		return time.Time{}, err
	}
	if err := s.updateFishingPondFeedStockTx(ctx, tx, userID, state.FeedStock, settledAt, state.Paused); err != nil {
		return time.Time{}, err
	}
	if err := s.promoteFishingPondRulesetTx(ctx, tx, userID, state.RulesetVersion, settledAt); err != nil {
		return time.Time{}, err
	}
	return settledAt, nil
}

func (s *Store) settleFishingPondFishStep(row *fishingPondFishRow, feedStock map[string]float64, userID int64, at time.Time, stepHours float64) {
	if row == nil || stepHours <= 0 {
		return
	}
	rows := []fishingPondFishRow{*row}
	s.settleFishingPondFeedingStep(rows, feedStock, nil, userID, at, stepHours)
	*row = rows[0]
}

func (s *Store) settleFishingPondFishWithFeed(row *fishingPondFishRow, userID int64, at time.Time, stepHours float64, feedResult fishingPondNutritionResult) {
	if row == nil || normalizePondLifeState(row.Fish.LifeState) != "alive" {
		return
	}
	fish := &row.Fish
	profile := s.gameData.FishingPondSpeciesByID(fish.SpeciesID)
	species := s.gameData.FishingSpeciesByID(fish.SpeciesID)
	if profile == nil || species == nil {
		return
	}
	maintenance, targetWeight, desiredGain, needed := s.preparePondFeedDemand(fish, row.InstanceID, userID, at, stepHours, species, profile)
	effective, preferred := feedResult.Effective, feedResult.Preferred
	fedRatio := 0.0
	if needed > 0 {
		fedRatio = clampFloat(effective/needed, 0, 1)
	}
	if fedRatio >= 0.98 {
		fish.Satiety = clampFloat(fish.Satiety+8*stepHours, 0, 100)
		fish.StarvationHours = 0
		fish.Condition = clampFloat(fish.Condition+0.003*stepHours/24, 0.5, 1.12)
		fish.Health = clampFloat(fish.Health+0.004*stepHours/24, 0, 1)
	} else {
		fish.Satiety = clampFloat(fish.Satiety-(1-fedRatio)*(100.0/24)*stepHours, 0, 100)
		if fish.Satiety <= 0.0001 {
			fish.StarvationHours += stepHours
			if fish.StarvationHours <= 48 {
				fish.Condition = clampFloat(fish.Condition-0.16*stepHours/48, 0.4, 1.12)
			} else {
				fish.Health = clampFloat(fish.Health-0.08*stepHours/24, 0, 1)
			}
		}
	}
	if preferred && fedRatio >= 0.5 {
		fish.PreferredFeedHours = math.Min(240, fish.PreferredFeedHours+stepHours)
	} else {
		fish.PreferredFeedHours = math.Max(0, fish.PreferredFeedHours-stepHours*2)
	}
	growthNutrition := math.Max(0, effective-maintenance)
	gain := math.Min(desiredGain, growthNutrition/fishingPondNutritionPerKG)
	if gain > 0 {
		fish.WeightKG = math.Max(fish.WeightKG, roundFloat(math.Min(targetWeight, fish.WeightKG+gain), 3))
		denominator := species.ShapeFactor * math.Max(0.5, fish.Condition)
		if denominator > 0 {
			fish.LengthCM = math.Max(fish.LengthCM, roundFloat(math.Min(species.MaxLengthCM, math.Cbrt(fish.WeightKG*100_000/denominator)), 1))
		}
	}
	s.applyFishingPondFeedEffects(fish, species, profile, feedResult.Doses, stepHours)
	s.clampFishingPondFishSize(fish, species)
	deathChance := fishingPondDeathChance(*fish, profile, stepHours)
	if fish.Health <= 0 || pondUnit(fmt.Sprintf("%d:%d:%s:death", userID, row.InstanceID, fishingPondSettlementStepSeed(at, stepHours))) < deathChance {
		fish.LifeState = "dead"
		fish.DeathAt = timePointer(at)
		fish.DeathCause = "starvation"
		if fish.Health > 0 {
			fish.DeathCause = "old_age"
		}
	}
}

func fishingPondTargetLength(fish FishInstanceData, species *catalog.FishingSpecies, profile *catalog.FishingPondSpecies) float64 {
	if species == nil || profile == nil || species.MaxLengthCM <= 0 {
		return math.Max(0, fish.LengthCM)
	}
	fryLength := math.Max(2, species.MinLengthCM*0.18)
	maturityLength := species.MaturityLengthCM
	if maturityLength <= fryLength || maturityLength >= species.MaxLengthCM {
		maturityLength = species.MinLengthCM + (species.MaxLengthCM-species.MinLengthCM)*0.35
	}
	maturityAge := math.Max(1, float64(profile.MaturityAgeDays))
	lifespan := math.Max(maturityAge+1, float64(profile.LifespanDays))
	effectiveAge := math.Max(0, float64(fish.AgeDays)+fish.BioAgeFraction) * (1 + 0.25*clampFloat(fish.Genes.GrowthRate, -1, 1))
	if normalizeFishingPondZone(fish.PondZone) == fishingPondZoneNursery {
		effectiveAge = math.Min(effectiveAge, maturityAge)
	}
	targetLength := fryLength
	if effectiveAge <= maturityAge {
		progress := math.Pow(clampFloat(effectiveAge/maturityAge, 0, 1), 0.62)
		targetLength += (maturityLength - fryLength) * progress
	} else {
		adultTarget := math.Max(maturityLength, species.MaxLengthCM*0.88)
		progress := math.Pow(clampFloat((effectiveAge-maturityAge)/(lifespan-maturityAge), 0, 1), 0.72)
		targetLength = maturityLength + (adultTarget-maturityLength)*progress
	}
	targetLength *= 1 + 0.10*clampFloat(fish.Genes.SizePotential, -1, 1)
	return clampFloat(targetLength, fryLength, species.MaxLengthCM)
}

func (s *Store) settleFishingPondPredationStep(rows []fishingPondFishRow, userID int64, at time.Time, stepHours float64) []fishingPondPredationEvent {
	if len(rows) < 2 || stepHours <= 0 {
		return nil
	}
	predatorIndexes := make([]int, 0, len(rows))
	for index := range rows {
		profile := s.gameData.FishingPondSpeciesByID(rows[index].Fish.SpeciesID)
		if profile != nil && profile.Predation != nil && normalizeFishingPondZone(rows[index].Fish.PondZone) == fishingPondZoneMain && normalizePondLifeState(rows[index].Fish.LifeState) == "alive" {
			predatorIndexes = append(predatorIndexes, index)
		}
	}
	sort.Slice(predatorIndexes, func(i, j int) bool {
		left := rows[predatorIndexes[i]]
		right := rows[predatorIndexes[j]]
		if math.Abs(left.Fish.LengthCM-right.Fish.LengthCM) > 0.0001 {
			return left.Fish.LengthCM > right.Fish.LengthCM
		}
		return left.InstanceID < right.InstanceID
	})

	events := make([]fishingPondPredationEvent, 0)
	for _, predatorIndex := range predatorIndexes {
		predator := &rows[predatorIndex]
		if normalizePondLifeState(predator.Fish.LifeState) != "alive" {
			continue
		}
		profile := s.gameData.FishingPondSpeciesByID(predator.Fish.SpeciesID)
		if profile == nil || profile.Predation == nil {
			continue
		}
		config := profile.Predation
		if predator.Fish.LengthCM <= 0 || predator.Fish.Satiety >= config.HuntBelowSatiety {
			continue
		}

		preyIndexes := make([]int, 0, len(rows)-1)
		maximumPreyLength := predator.Fish.LengthCM * config.MaxPreyLengthRatio
		for preyIndex := range rows {
			prey := &rows[preyIndex]
			if preyIndex == predatorIndex || normalizeFishingPondZone(prey.Fish.PondZone) != fishingPondZoneMain || normalizePondLifeState(prey.Fish.LifeState) != "alive" || prey.Fish.LengthCM <= 0 || prey.Fish.LengthCM > maximumPreyLength+0.0001 {
				continue
			}
			preyIndexes = append(preyIndexes, preyIndex)
		}
		if len(preyIndexes) == 0 {
			continue
		}
		sort.Slice(preyIndexes, func(i, j int) bool {
			left := rows[preyIndexes[i]]
			right := rows[preyIndexes[j]]
			if math.Abs(left.Fish.LengthCM-right.Fish.LengthCM) > 0.0001 {
				return left.Fish.LengthCM < right.Fish.LengthCM
			}
			if math.Abs(left.Fish.WeightKG-right.Fish.WeightKG) > 0.000001 {
				return left.Fish.WeightKG < right.Fish.WeightKG
			}
			return left.InstanceID < right.InstanceID
		})
		prey := &rows[preyIndexes[0]]
		hunger := clampFloat((config.HuntBelowSatiety-predator.Fish.Satiety)/math.Max(1, config.HuntBelowSatiety), 0, 1)
		chance := fishingPondPredationChance(config.DailyHuntChance, hunger, stepHours)
		seed := fmt.Sprintf("%d:%d:%d:%s:predation", userID, predator.InstanceID, prey.InstanceID, fishingPondSettlementStepSeed(at, stepHours))
		if pondUnit(seed) >= chance {
			continue
		}

		prey.Fish.LifeState = "dead"
		prey.Fish.DeathAt = timePointer(at)
		prey.Fish.DeathCause = "predation"
		preyWeight := math.Max(0.001, prey.Fish.WeightKG)
		predatorLength := predator.Fish.LengthCM
		predatorWeight := math.Max(0.01, predator.Fish.WeightKG)
		mealRatio := preyWeight / predatorWeight
		predator.Fish.Satiety = clampFloat(predator.Fish.Satiety+clampFloat(10+mealRatio*120, 10, 70), 0, 100)
		predator.Fish.StarvationHours = 0
		predator.Fish.Condition = clampFloat(predator.Fish.Condition+math.Min(0.018, mealRatio*0.025), 0.5, 1.12)
		predator.Fish.Health = clampFloat(predator.Fish.Health+math.Min(0.01, mealRatio*0.015), 0, 1)
		s.applyFishingPondPredationGrowth(&predator.Fish, preyWeight)
		events = append(events, fishingPondPredationEvent{
			PredatorInstanceID: predator.InstanceID, PreyInstanceID: prey.InstanceID,
			PredatorSpeciesID: predator.Fish.SpeciesID, PreySpeciesID: prey.Fish.SpeciesID,
			PredatorName: strings.TrimSpace(predator.CustomData.Name), PreyName: strings.TrimSpace(prey.CustomData.Name),
			PredatorLengthCM: predatorLength, PreyLengthCM: prey.Fish.LengthCM, PreyWeightKG: preyWeight,
		})
	}
	return events
}

func fishingPondPredationChance(dailyChance, hunger, stepHours float64) float64 {
	if dailyChance <= 0 || stepHours <= 0 {
		return 0
	}
	effectiveDailyChance := clampFloat(dailyChance, 0, 1) * (0.35 + 0.65*clampFloat(hunger, 0, 1))
	return clampFloat(1-math.Pow(1-effectiveDailyChance, stepHours/24), 0, 1)
}

func (s *Store) applyFishingPondPredationGrowth(predator *FishInstanceData, preyWeightKG float64) {
	if predator == nil || preyWeightKG <= 0 {
		return
	}
	species := s.gameData.FishingSpeciesByID(predator.SpeciesID)
	if species == nil || species.ShapeFactor <= 0 || species.MaxLengthCM <= 0 {
		return
	}
	maximumWeight := fishingPondMaximumWeight(species)
	previousWeight := predator.WeightKG
	predator.WeightKG = math.Max(predator.WeightKG, roundFloat(math.Min(maximumWeight, predator.WeightKG+preyWeightKG*0.12), 3))
	if predator.WeightKG <= previousWeight {
		return
	}
	denominator := species.ShapeFactor * math.Max(0.5, predator.Condition)
	predator.LengthCM = math.Max(predator.LengthCM, roundFloat(math.Min(species.MaxLengthCM, math.Cbrt(predator.WeightKG*100_000/denominator)), 1))
	s.clampFishingPondFishSize(predator, species)
}

func (s *Store) consumeFishingPondNutrition(stock map[string]float64, profile *catalog.FishingPondSpecies, feedEfficiency, needed float64) (float64, bool) {
	result := s.consumeFishingPondNutritionForFish(stock, FishInstanceData{}, profile, feedEfficiency, needed)
	return result.Effective, result.Preferred
}

func (s *Store) consumeFishingPondNutritionForFish(stock map[string]float64, fish FishInstanceData, profile *catalog.FishingPondSpecies, feedEfficiency, needed float64) fishingPondNutritionResult {
	demand := pondFeedDemand{fish, needed, s.pondFeedCandidates(fish, profile, stock, nil, feedEfficiency)}
	return allocatePondFeed([]pondFeedDemand{demand}, stock, 1)[0]
}

func pondFeedConditionsMatch(fish FishInstanceData, profile *catalog.FishingPondSpecies, feed *catalog.PondFeedConfig) bool {
	if feed == nil || (feed.RulesetVersion != "" && feed.RulesetVersion != pondFeedRulesetVersion) {
		return false
	}
	for _, condition := range feed.Conditions {
		switch condition.Field {
		case "dietClass":
			if profile == nil || !pondFeedStringComparison(profile.DietClass, condition.Op, condition.Value) {
				return false
			}
		case "pondZone":
			if !pondFeedStringComparison(normalizeFishingPondZone(fish.PondZone), condition.Op, condition.Value) {
				return false
			}
		case "health":
			if !pondFeedNumberComparison(fish.Health, condition.Op, condition.Value) {
				return false
			}
		case "satiety":
			if !pondFeedNumberComparison(fish.Satiety, condition.Op, condition.Value) {
				return false
			}
		case "condition":
			if !pondFeedNumberComparison(fish.Condition, condition.Op, condition.Value) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func pondFeedStringComparison(actual, op string, expected any) bool {
	wanted, ok := expected.(string)
	if !ok {
		return false
	}
	switch op {
	case "eq":
		return actual == wanted
	case "neq":
		return actual != wanted
	default:
		return false
	}
}

func pondFeedNumberComparison(actual float64, op string, expected any) bool {
	wanted, ok := pondFeedNumber(expected)
	if !ok || math.IsNaN(actual) || math.IsInf(actual, 0) {
		return false
	}
	switch op {
	case "eq":
		return math.Abs(actual-wanted) <= 0.000001
	case "neq":
		return math.Abs(actual-wanted) > 0.000001
	case "lt":
		return actual < wanted
	case "lte":
		return actual <= wanted
	case "gt":
		return actual > wanted
	case "gte":
		return actual >= wanted
	default:
		return false
	}
}

func pondFeedNumber(value any) (float64, bool) {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}

func (s *Store) applyFishingPondFeedEffects(fish *FishInstanceData, species *catalog.FishingSpecies, profile *catalog.FishingPondSpecies, doses []fishingPondFeedDose, stepHours float64) {
	if fish == nil || species == nil || profile == nil || stepHours <= 0 {
		return
	}
	remainingDose := fishingPondMaxEffectPortionsHour * stepHours
	for _, dose := range doses {
		if remainingDose <= 0 {
			break
		}
		if dose.Feed == nil || dose.Feed.RulesetVersion != pondFeedRulesetVersion || len(dose.Feed.Effects) == 0 || dose.Portions <= 0 {
			continue
		}
		appliedDose := math.Min(remainingDose, dose.Portions)
		appliedDose = pondFeedPayableDose(*fish, dose.Feed, appliedDose)
		s.applyFishingPondFeedCompound(fish, species, profile, dose.Feed, appliedDose)
		remainingDose -= appliedDose
	}
}

func (s *Store) applyFishingPondFeedCompound(fish *FishInstanceData, species *catalog.FishingSpecies, profile *catalog.FishingPondSpecies, feed *catalog.PondFeedConfig, appliedDose float64) {
	before := *fish
	after := before
	for _, effect := range feed.Effects {
		value := before
		s.applyFishingPondFeedEffect(&value, species, profile, effect, appliedDose)
		switch effect.Stat {
		case "lengthCm":
			after.LengthCM = value.LengthCM
		case "weightKg":
			after.WeightKG = value.WeightKG
		case "health":
			after.Health += value.Health - before.Health
		case "satiety":
			after.Satiety += value.Satiety - before.Satiety
		case "longevityDays":
			after.LongevityLimitDays = value.LongevityLimitDays
		case "naturalProtectionDays":
			after.NaturalProtectionUntilAgeDays = value.NaturalProtectionUntilAgeDays
		}
	}
	after.Health = clampFloat(after.Health, 0, 1)
	after.Satiety = clampFloat(after.Satiety, 0, 100)
	*fish = after
}

func (s *Store) applyFishingPondFeedEffect(fish *FishInstanceData, species *catalog.FishingSpecies, profile *catalog.FishingPondSpecies, effect catalog.PondFeedEffect, dose float64) {
	if fish == nil || species == nil || profile == nil || effect.Kind != "stat_delta" || dose <= 0 {
		return
	}
	if effect.Mode == "bonus_band" && effect.ValuePerPortion > 0 && effect.ValuePerPortion < 1 && effect.MaxTargetMultiplier > 1 {
		progress := 1 - math.Pow(1-effect.ValuePerPortion, dose)
		switch effect.Stat {
		case "lengthCm":
			naturalTarget := fishingPondTargetLength(*fish, species, profile)
			ceiling := math.Min(species.MaxLengthCM, naturalTarget*effect.MaxTargetMultiplier)
			if fish.LengthCM < ceiling {
				fish.LengthCM += (ceiling - fish.LengthCM) * progress
			}
		case "weightKg":
			naturalLength := math.Max(fish.LengthCM, fishingPondTargetLength(*fish, species, profile))
			naturalTarget := species.ShapeFactor * math.Pow(naturalLength, 3) / 100_000 * clampFloat(fish.Condition, 0.82, 1.12)
			ceiling := math.Min(fishingPondMaximumWeight(species), naturalTarget*effect.MaxTargetMultiplier)
			if fish.WeightKG < ceiling {
				fish.WeightKG += (ceiling - fish.WeightKG) * progress
			}
		}
		return
	}
	if effect.Mode != "flat" {
		return
	}
	if effect.ValuePerPortion > 0 {
		switch effect.Stat {
		case "longevityDays":
			fish.LongevityLimitDays += effect.ValuePerPortion * dose
			return
		case "naturalProtectionDays":
			age := float64(fish.AgeDays) + fish.BioAgeFraction
			fish.NaturalProtectionUntilAgeDays = math.Max(age, fish.NaturalProtectionUntilAgeDays) + effect.ValuePerPortion*dose
			return
		}
	}
	minimum, maximum := 0.0, 1.0
	if effect.Stat == "satiety" {
		maximum = 100
	}
	if effect.Floor != nil {
		minimum = *effect.Floor
	}
	if effect.Ceiling != nil {
		maximum = *effect.Ceiling
	}
	switch effect.Stat {
	case "health":
		if effect.ValuePerPortion > 0 {
			fish.Health += math.Min(effect.ValuePerPortion*dose, math.Max(0, maximum-fish.Health))
		} else {
			fish.Health -= math.Min(-effect.ValuePerPortion*dose, math.Max(0, fish.Health-minimum))
		}
	case "satiety":
		if effect.ValuePerPortion > 0 {
			fish.Satiety += math.Min(effect.ValuePerPortion*dose, math.Max(0, maximum-fish.Satiety))
		} else {
			fish.Satiety -= math.Min(-effect.ValuePerPortion*dose, math.Max(0, fish.Satiety-minimum))
		}
	}
}

func fishingPondMaximumWeight(species *catalog.FishingSpecies) float64 {
	if species == nil || species.ShapeFactor <= 0 || species.MaxLengthCM <= 0 {
		return 0
	}
	return roundFloat(species.ShapeFactor*math.Pow(species.MaxLengthCM, 3)/100_000*1.12, 3)
}

func fishingPondFishSizeValid(fish FishInstanceData, species *catalog.FishingSpecies) bool {
	maximumWeight := fishingPondMaximumWeight(species)
	return species != nil && fish.LengthCM > 0 && fish.WeightKG > 0 &&
		!math.IsNaN(fish.LengthCM) && !math.IsInf(fish.LengthCM, 0) &&
		!math.IsNaN(fish.WeightKG) && !math.IsInf(fish.WeightKG, 0) &&
		fish.LengthCM <= species.MaxLengthCM+0.000001 && fish.WeightKG <= maximumWeight+0.000001
}

func (s *Store) clampFishingPondFishSize(fish *FishInstanceData, species *catalog.FishingSpecies) {
	if fish == nil || species == nil {
		return
	}
	if species.MaxLengthCM > 0 && fish.LengthCM > species.MaxLengthCM {
		fish.LengthCM = species.MaxLengthCM
	}
	maximumWeight := fishingPondMaximumWeight(species)
	if maximumWeight > 0 && fish.WeightKG > maximumWeight {
		fish.WeightKG = maximumWeight
	}
}

func (s *Store) settleFishingPondBreedingStepTx(ctx context.Context, tx pgx.Tx, userID int64, at time.Time, stepHours float64, rulesetVersion string, limits FishingPondLimits, rows *[]fishingPondFishRow, pairs *[]fishingPondPairRow) error {
	if rows == nil || pairs == nil {
		return nil
	}
	policy, err := fishingPondBreedingPolicyForRuleset(rulesetVersion, limits)
	if err != nil {
		return err
	}
	occupied := fishingPondZoneOccupiedCount(*rows, policy.NewbornZone)
	if policy.CountAllPondFish {
		occupied = fishingPondBreedingOccupiedCount(*rows)
	}
	if occupied >= policy.Capacity {
		return nil
	}
	newborns := make([]fishingPondFishRow, 0, 1)
	rowByID := make(map[int64]*fishingPondFishRow, len(*rows))
	for index := range *rows {
		rowByID[(*rows)[index].InstanceID] = &(*rows)[index]
	}
	designated := make(map[int64]*fishingPondPairRow, len(*pairs))
	for index := range *pairs {
		designated[(*pairs)[index].FemaleInstanceID] = &(*pairs)[index]
	}
	females := make([]*fishingPondFishRow, 0)
	males := make([]*fishingPondFishRow, 0)
	biomass := 0.0
	for index := range *rows {
		row := &(*rows)[index]
		if normalizePondLifeState(row.Fish.LifeState) != "alive" {
			continue
		}
		biomass += row.Fish.WeightKG
		if policy.ParentZone != "" && normalizeFishingPondZone(row.Fish.PondZone) != policy.ParentZone {
			continue
		}
		if row.Fish.Sex == "female" {
			females = append(females, row)
		} else if row.Fish.Sex == "male" {
			males = append(males, row)
		}
	}
	for _, female := range females {
		if occupied+int64(len(newborns)) >= policy.Capacity || policy.EnforceBiomass && biomass >= limits.BiomassKG {
			break
		}
		profile := s.gameData.FishingPondSpeciesByID(female.Fish.SpeciesID)
		if profile == nil || !pondFishCanBreed(female.Fish, profile, at) {
			continue
		}
		species := s.gameData.FishingSpeciesByID(female.Fish.SpeciesID)
		if species == nil || policy.EnforceBiomass && biomass+fishingPondMaximumFryWeight(species) > limits.BiomassKG+0.0001 {
			continue
		}
		var male *fishingPondFishRow
		pair := designated[female.InstanceID]
		if pair != nil {
			male = rowByID[pair.MaleInstanceID]
			if male != nil && policy.ParentZone != "" && normalizeFishingPondZone(male.Fish.PondZone) != policy.ParentZone {
				male = nil
			}
		}
		if male == nil {
			for _, candidate := range males {
				if candidate.Fish.SpeciesID == female.Fish.SpeciesID && pondFishCanBreed(candidate.Fish, profile, at) {
					male = candidate
					break
				}
			}
		}
		if male == nil || !pondFishCanBreed(male.Fish, profile, at) {
			continue
		}
		averageFertility := (female.Fish.Genes.Fertility + male.Fish.Genes.Fertility) / 2
		chance := fishingPondBreedChance(species, pair != nil, averageFertility, pair != nil && pair.MashReady, stepHours)
		seed := fmt.Sprintf("%d:%d:%d:%s:breed", userID, female.InstanceID, male.InstanceID, fishingPondSettlementStepSeed(at, stepHours))
		if pondUnit(seed) >= chance {
			continue
		}
		mutationInheritanceMultiplier := 1.0
		if pair == nil {
			mutationInheritanceMultiplier = pondScatterMutationInheritance
		}
		newborn, err := s.createFishingPondFryTx(ctx, tx, userID, *female, *male, profile, mutationInheritanceMultiplier, policy.NewbornRuleset, policy.NewbornZone, at)
		if err != nil {
			return err
		}
		female.Fish.LastBredAt = timePointer(at)
		male.Fish.LastBredAt = timePointer(at)
		newborns = append(newborns, newborn)
		biomass += newborn.Fish.WeightKG
		if pair != nil {
			pair.MashReady = false
			if _, err := tx.Exec(ctx, `UPDATE fishing_pond_pairs SET mash_ready=FALSE, updated_at=$3 WHERE user_id=$1 AND slot_index=$2`, userID, pair.SlotIndex, at); err != nil {
				return err
			}
		}
	}
	*rows = append(*rows, newborns...)
	return nil
}

func fishingPondBreedingPolicyForRuleset(rulesetVersion string, limits FishingPondLimits) (fishingPondBreedingPolicy, error) {
	switch strings.TrimSpace(rulesetVersion) {
	case fishingPondPreviousRulesetVersion:
		return fishingPondBreedingPolicy{
			Capacity: limits.FishCapacity, NewbornZone: fishingPondZoneMain, NewbornRuleset: fishingPondPreviousRulesetVersion,
			EnforceBiomass: true, CountAllPondFish: true,
		}, nil
	case FishingPondRulesetVersion:
		return fishingPondBreedingPolicy{
			Capacity: limits.NurseryCapacity, ParentZone: fishingPondZoneMain, NewbornZone: fishingPondZoneNursery, NewbornRuleset: FishingPondRulesetVersion,
		}, nil
	default:
		return fishingPondBreedingPolicy{}, fmt.Errorf("%w: %q", ErrFishingPondRulesetUnsupported, rulesetVersion)
	}
}

func fishingPondBreedingOccupiedCount(rows []fishingPondFishRow) int64 {
	occupied := int64(0)
	for _, row := range rows {
		if normalizePondLifeState(row.Fish.LifeState) == "dead" && row.Fish.DeathCause == "predation" {
			continue
		}
		occupied++
	}
	return occupied
}

func fishingPondZoneOccupiedCount(rows []fishingPondFishRow, zone string) int64 {
	zone = normalizeFishingPondZone(zone)
	occupied := int64(0)
	for _, row := range rows {
		if normalizeFishingPondZone(row.Fish.PondZone) != zone {
			continue
		}
		if normalizePondLifeState(row.Fish.LifeState) == "dead" && row.Fish.DeathCause == "predation" {
			continue
		}
		occupied++
	}
	return occupied
}

func fishingPondMainBiomass(rows []fishingPondFishRow) float64 {
	biomass := 0.0
	for _, row := range rows {
		if normalizeFishingPondZone(row.Fish.PondZone) == fishingPondZoneMain && normalizePondLifeState(row.Fish.LifeState) == "alive" {
			biomass += row.Fish.WeightKG
		}
	}
	return biomass
}

func normalizeFishingPondZone(zone string) string {
	if strings.EqualFold(strings.TrimSpace(zone), fishingPondZoneNursery) {
		return fishingPondZoneNursery
	}
	return fishingPondZoneMain
}

func (s *Store) createFishingPondFryTx(ctx context.Context, tx pgx.Tx, userID int64, female, male fishingPondFishRow, profile *catalog.FishingPondSpecies, mutationInheritanceMultiplier float64, rulesetVersion, pondZone string, at time.Time) (fishingPondFishRow, error) {
	species := s.gameData.FishingSpeciesByID(female.Fish.SpeciesID)
	if species == nil || profile == nil {
		return fishingPondFishRow{}, ErrFishingPondPairInvalid
	}
	seed := fmt.Sprintf("%d:%d:%d:%d:fry", userID, female.InstanceID, male.InstanceID, at.UnixNano())
	genes := inheritPondGenes(female.Fish, male.Fish, seed)
	genes.Mutations = s.inheritFishingMutations(female.Fish, male.Fish, seed, mutationInheritanceMultiplier)
	sex := "female"
	if pondUnit(seed+":sex") < 0.5 {
		sex = "male"
	}
	length := math.Max(2, species.MinLengthCM*0.18)
	condition := 0.94 + pondUnit(seed+":condition")*0.08
	weight := math.Max(0.005, species.ShapeFactor*math.Pow(length, 3)/100_000*condition)
	fish := FishInstanceData{
		SpeciesID: species.ID, AgeDays: 0, Sex: sex, LengthCM: roundFloat(length, 1), WeightKG: roundFloat(weight, 3),
		Condition: roundFloat(condition, 3), Health: 1, WaterID: species.WaterID, SpotID: "fish_pond", CaughtAt: at,
		RulesetVersion: rulesetVersion, Origin: "cultured", PondZone: normalizeFishingPondZone(pondZone), LifeState: "alive",
		Generation: pondMaxInt(female.Fish.Generation, male.Fish.Generation) + 1, ParentInstanceIDs: []int64{female.InstanceID, male.InstanceID},
		Genes: genes, Satiety: 100, BornAt: timePointer(at), PondEnteredAt: timePointer(at),
	}
	s.initializeFishingPondFish(&fish, userID, int64(stableFishingHash(seed)), at, profile)
	s.clampFishingPondFishSize(&fish, species)
	instanceID, err := s.addFishInstanceTx(ctx, tx, userID, fish)
	if err != nil {
		return fishingPondFishRow{}, err
	}
	if err := s.LedgerSetStateTx(ctx, tx, instanceID, userID, "inventory", fishingPondState, "fish_catch", 1, pondLedgerMetaForRuleset("fishing_pond", "fishing_pond", InventoryAuditDirectionGain, "pond_birth", rulesetVersion)); err != nil {
		return fishingPondFishRow{}, err
	}
	if err := s.addFishingPondEventTx(ctx, tx, userID, "born", &instanceID, &female.InstanceID, map[string]any{"speciesId": fish.SpeciesID, "maleInstanceId": male.InstanceID, "generation": fish.Generation}); err != nil {
		return fishingPondFishRow{}, err
	}
	return fishingPondFishRow{InstanceID: instanceID, Fish: fish}, nil
}

func fishingPondMaximumFryWeight(species *catalog.FishingSpecies) float64 {
	if species == nil {
		return 0
	}
	length := math.Max(2, species.MinLengthCM*0.18)
	return math.Max(0.005, species.ShapeFactor*math.Pow(length, 3)/100_000*1.02)
}

func (s *Store) listFishingPondFishTx(ctx context.Context, tx pgx.Tx, userID int64, lock bool) ([]fishingPondFishRow, error) {
	query := `SELECT instance_id, COALESCE(custom_data, '{}'::jsonb), COALESCE(fish_data, '{}'::jsonb) FROM player_item_instances WHERE user_id=$1 AND state=$2 AND template_item_id='fish_catch' ORDER BY instance_id`
	if lock {
		query += ` FOR UPDATE`
	}
	rows, err := tx.Query(ctx, query, userID, fishingPondState)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]fishingPondFishRow, 0)
	for rows.Next() {
		var row fishingPondFishRow
		var customRaw, fishRaw []byte
		if err := rows.Scan(&row.InstanceID, &customRaw, &fishRaw); err != nil {
			return nil, err
		}
		if json.Unmarshal(customRaw, &row.CustomData) != nil || json.Unmarshal(fishRaw, &row.Fish) != nil || strings.TrimSpace(row.Fish.SpeciesID) == "" {
			continue
		}
		s.clampFishingPondFishSize(&row.Fish, s.gameData.FishingSpeciesByID(row.Fish.SpeciesID))
		out = append(out, row)
	}
	return out, rows.Err()
}

type FishingPondFishPage struct {
	Items      []FishingPondFishView `json:"items"`
	NextCursor int64                 `json:"nextCursor,omitempty"`
}

func (s *Store) ListAvailableFishingPondFish(ctx context.Context, userID, cursor int64) (FishingPondFishPage, error) {
	return s.listAvailablePondFishPage(ctx, s.db, userID, cursor)
}

func (s *Store) listAvailablePondFishPage(ctx context.Context, queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, userID, cursor int64) (FishingPondFishPage, error) {
	const pageSize = 200
	if cursor <= 0 {
		cursor = math.MaxInt64
	}
	rows, err := queryer.Query(ctx, `SELECT instance_id, COALESCE(custom_data, '{}'::jsonb), COALESCE(fish_data, '{}'::jsonb)
		FROM player_item_instances
		WHERE user_id=$1 AND instance_id<$2 AND state='inventory' AND archived_at IS NULL
		  AND quantity=1 AND template_item_id='fish_catch' AND fish_data<>'{}'::jsonb
		ORDER BY instance_id DESC LIMIT $3`, userID, cursor, pageSize+1)
	if err != nil {
		return FishingPondFishPage{}, err
	}
	defer rows.Close()
	out := FishingPondFishPage{Items: make([]FishingPondFishView, 0)}
	scanned := 0
	var lastInstanceID int64
	for rows.Next() {
		var row fishingPondFishRow
		var customRaw, fishRaw []byte
		if err := rows.Scan(&row.InstanceID, &customRaw, &fishRaw); err != nil {
			return FishingPondFishPage{}, err
		}
		// Advance by scanned inventory rows, even when an entire page is ineligible.
		if scanned == pageSize {
			out.NextCursor = lastInstanceID
			break
		}
		scanned++
		lastInstanceID = row.InstanceID
		if json.Unmarshal(customRaw, &row.CustomData) != nil || json.Unmarshal(fishRaw, &row.Fish) != nil || normalizePondLifeState(row.Fish.LifeState) != "alive" || s.gameData.FishingPondSpeciesByID(row.Fish.SpeciesID) == nil {
			continue
		}
		out.Items = append(out.Items, s.fishingPondFishView(row))
	}
	return out, rows.Err()
}

func (s *Store) lockFishingPondFishTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64) (FishInstanceData, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT COALESCE(fish_data, '{}'::jsonb) FROM player_item_instances WHERE user_id=$1 AND instance_id=$2 AND state=$3 AND template_item_id='fish_catch' FOR UPDATE`, userID, instanceID, fishingPondState).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return FishInstanceData{}, ErrFishingPondFishInvalid
	}
	if err != nil {
		return FishInstanceData{}, err
	}
	var fish FishInstanceData
	if json.Unmarshal(raw, &fish) != nil || strings.TrimSpace(fish.SpeciesID) == "" {
		return FishInstanceData{}, ErrFishingPondFishInvalid
	}
	s.clampFishingPondFishSize(&fish, s.gameData.FishingSpeciesByID(fish.SpeciesID))
	return fish, nil
}

func (s *Store) updateFishingPondFishTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64, fish FishInstanceData, now time.Time) error {
	s.clampFishingPondFishSize(&fish, s.gameData.FishingSpeciesByID(fish.SpeciesID))
	raw, err := json.Marshal(fish)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE player_item_instances SET fish_data=$4::jsonb, updated_at=$5 WHERE user_id=$1 AND instance_id=$2 AND state=$3`, userID, instanceID, fishingPondState, string(raw), now)
	return err
}

func (s *Store) listFishingPondPairsTx(ctx context.Context, tx pgx.Tx, userID int64) ([]fishingPondPairRow, error) {
	rows, err := tx.Query(ctx, `SELECT slot_index, female_instance_id, male_instance_id, mash_ready FROM fishing_pond_pairs WHERE user_id=$1 ORDER BY slot_index`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]fishingPondPairRow, 0)
	for rows.Next() {
		var row fishingPondPairRow
		if err := rows.Scan(&row.SlotIndex, &row.FemaleInstanceID, &row.MaleInstanceID, &row.MashReady); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) lockFishingPondStateTx(ctx context.Context, tx pgx.Tx, userID int64) (fishingPondStateRow, error) {
	var raw []byte
	var state fishingPondStateRow
	err := tx.QueryRow(ctx, `SELECT feed_stock_jsonb, last_settled_at, ruleset_version FROM fishing_ponds WHERE user_id=$1 FOR UPDATE`, userID).Scan(&raw, &state.LastSettledAt, &state.RulesetVersion)
	if err != nil {
		return fishingPondStateRow{}, err
	}
	state.FeedStock, state.Paused, err = s.decodePondFeedStock(raw)
	if err != nil {
		return fishingPondStateRow{}, ErrFishingPondFeedInvalid
	}
	if !isSupportedFishingPondRuleset(state.RulesetVersion) {
		return fishingPondStateRow{}, fmt.Errorf("%w: %q", ErrFishingPondRulesetUnsupported, state.RulesetVersion)
	}
	return state, nil
}

func (s *Store) getFishingPondStateTx(ctx context.Context, tx pgx.Tx, userID int64) (fishingPondStateRow, error) {
	var raw []byte
	var state fishingPondStateRow
	err := tx.QueryRow(ctx, `SELECT feed_stock_jsonb, last_settled_at, ruleset_version FROM fishing_ponds WHERE user_id=$1`, userID).Scan(&raw, &state.LastSettledAt, &state.RulesetVersion)
	if err != nil {
		return fishingPondStateRow{}, err
	}
	state.FeedStock, state.Paused, err = s.decodePondFeedStock(raw)
	if err != nil {
		return fishingPondStateRow{}, ErrFishingPondFeedInvalid
	}
	if !isSupportedFishingPondRuleset(state.RulesetVersion) {
		return fishingPondStateRow{}, fmt.Errorf("%w: %q", ErrFishingPondRulesetUnsupported, state.RulesetVersion)
	}
	return state, nil
}

func isSupportedFishingPondRuleset(rulesetVersion string) bool {
	rulesetVersion = strings.TrimSpace(rulesetVersion)
	return rulesetVersion == fishingPondPreviousRulesetVersion || rulesetVersion == FishingPondRulesetVersion
}

func (s *Store) promoteFishingPondRulesetTx(ctx context.Context, tx pgx.Tx, userID int64, rulesetVersion string, now time.Time) error {
	rulesetVersion = strings.TrimSpace(rulesetVersion)
	if rulesetVersion == FishingPondRulesetVersion {
		return nil
	}
	if rulesetVersion != fishingPondPreviousRulesetVersion {
		return fmt.Errorf("%w: %q", ErrFishingPondRulesetUnsupported, rulesetVersion)
	}
	result, err := tx.Exec(ctx, `UPDATE fishing_ponds SET ruleset_version=$2, updated_at=GREATEST(updated_at,$3) WHERE user_id=$1 AND ruleset_version=$4`, userID, FishingPondRulesetVersion, now, fishingPondPreviousRulesetVersion)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("%w: concurrent cutover for user %d", ErrFishingPondRulesetUnsupported, userID)
	}
	return nil
}

func (s *Store) updateFishingPondFeedStockTx(ctx context.Context, tx pgx.Tx, userID int64, stock map[string]float64, now time.Time, paused map[string]bool) error {
	raw, err := s.encodePondFeedStock(stock, paused)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE fishing_ponds SET feed_stock_jsonb=$2::jsonb, last_settled_at=$3, updated_at=$3 WHERE user_id=$1`, userID, string(raw), now)
	return err
}

func (s *Store) pondFeedInventoryCountsTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string]int64, error) {
	rows, err := tx.Query(ctx, `SELECT template_item_id, SUM(quantity) FROM player_item_instances WHERE user_id=$1 AND state='inventory' GROUP BY template_item_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var qty int64
		if err := rows.Scan(&id, &qty); err != nil {
			return nil, err
		}
		out[id] = qty
	}
	return out, rows.Err()
}

func (s *Store) addFishingPondEventTx(ctx context.Context, tx pgx.Tx, userID int64, kind string, fishID, relatedID *int64, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO fishing_pond_events(user_id,event_kind,fish_instance_id,related_instance_id,details_jsonb,created_at) VALUES($1,$2,$3,$4,$5::jsonb,now())`, userID, kind, fishID, relatedID, string(raw))
	return err
}

func (s *Store) listFishingPondEventsTx(ctx context.Context, tx pgx.Tx, userID int64, limit int) ([]FishingPondEventView, error) {
	rows, err := tx.Query(ctx, `SELECT id,event_kind,fish_instance_id,related_instance_id,details_jsonb,created_at FROM fishing_pond_events WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingPondEventView, 0)
	for rows.Next() {
		var row FishingPondEventView
		var raw []byte
		if err := rows.Scan(&row.ID, &row.Kind, &row.FishInstanceID, &row.RelatedInstanceID, &raw, &row.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &row.Details)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) initializeFishingPondFish(fish *FishInstanceData, userID, instanceID int64, now time.Time, profile *catalog.FishingPondSpecies) {
	if fish == nil || profile == nil {
		return
	}
	seed := fmt.Sprintf("%d:%d:%s:pond-genes", userID, instanceID, fish.SpeciesID)
	if fish.Origin == "" {
		fish.Origin = "cultured"
	}
	if fish.LifeState == "" {
		fish.LifeState = "alive"
	}
	fish.PondZone = normalizeFishingPondZone(fish.PondZone)
	fish.Sex = normalizedPondSex(fish.Sex, instanceID, fish.SpeciesID)
	if fish.PondEnteredAt == nil {
		fish.PondEnteredAt = timePointer(now)
	}
	if fish.Satiety <= 0 && fish.StarvationHours <= 0 {
		fish.Satiety = 100
	}
	if pondGenesAreZero(fish.Genes) {
		mutations := append([]string(nil), fish.Genes.Mutations...)
		fish.Genes = FishGenes{
			SizePotential: pondGene(seed + ":size"), GrowthRate: pondGene(seed + ":growth"), FeedEfficiency: pondGene(seed + ":feed"),
			Vitality: pondGene(seed + ":vitality"), Lifespan: pondGene(seed + ":life"), Fertility: pondGene(seed + ":fertility"),
			Mutations: mutations,
		}
	}
	if fish.LongevityLimitDays <= 0 {
		fish.LongevityLimitDays = s.fishingPondLongevityLimitDays(*fish, seed, profile)
	}
}

func (s *Store) fishingPondLongevityLimitDays(fish FishInstanceData, seed string, profile *catalog.FishingPondSpecies) float64 {
	if profile == nil {
		return 0
	}
	roll := pondUnit(seed + ":longevity")
	multiplier := 0.85 + pondUnit(seed+":longevity-range")*0.35
	if roll < 0.05 {
		multiplier = 1.2 + pondUnit(seed+":elder")*0.4
	}
	multiplier *= 1 + 0.12*clampFloat(fish.Genes.Lifespan, -1, 1)
	longevityLimitDays := float64(profile.LifespanDays) * multiplier * s.fishingMutationLifespanMultiplier(fish.Genes.Mutations)
	// A wild fish can already be older than its rolled pond lifespan. Give every
	// newly stocked fish a deterministic grace period instead of killing it on entry.
	minimumRemainingDays := math.Max(365, float64(profile.MaturityAgeDays)*0.5)
	return math.Max(longevityLimitDays, float64(fish.AgeDays)+minimumRemainingDays)
}

func fishingPondSettlementWindow(lastSettledAt, now time.Time) (time.Time, time.Time, int) {
	elapsed := now.Sub(lastSettledAt)
	if elapsed <= 0 {
		return lastSettledAt, lastSettledAt, 0
	}
	maximum := fishingPondMaxOfflineHours * time.Hour
	start := lastSettledAt
	if elapsed > maximum {
		start = now.Add(-maximum)
		elapsed = maximum
	}
	return start, now, int(elapsed / time.Hour)
}

func fishingPondSettlementStepSeed(at time.Time, stepHours float64) string {
	if math.Abs(stepHours-1) <= 0.0000001 {
		return fmt.Sprintf("hour:%d", at.Unix()/3600)
	}
	return fmt.Sprintf("partial:%d:%d", at.UnixNano(), int64(math.Round(stepHours*float64(time.Hour))))
}

func fishingPondDeathChance(fish FishInstanceData, profile *catalog.FishingPondSpecies, stepHours float64) float64 {
	if profile == nil || fish.LongevityLimitDays <= 0 {
		return 0
	}
	// Protection follows settled biological age, including partial/offline steps.
	if fish.NaturalProtectionUntilAgeDays > 0 {
		unprotectedDays := math.Max(0, float64(fish.AgeDays)+fish.BioAgeFraction-fish.NaturalProtectionUntilAgeDays)
		stepHours = math.Min(stepHours, unprotectedDays*24/fishingPondBioDaysPerRealDay)
		if stepHours <= 0 {
			return 0
		}
	}
	ratio := float64(fish.AgeDays) / fish.LongevityLimitDays
	daily := 0.0001
	if ratio > 0.6 && ratio <= 1 {
		daily = 0.0001 + 0.0299*math.Pow((ratio-0.6)/0.4, 3)
	}
	if ratio > 1 {
		daily = math.Min(0.5, 0.03+0.47*math.Pow(math.Min(1, (ratio-1)/0.5), 2))
	}
	daily *= 1 - 0.18*clampFloat(fish.Genes.Vitality, -1, 1)
	daily += math.Max(0, 1-fish.Health) * 0.08
	return clampFloat(daily*stepHours/24, 0, 0.95)
}

func pondFishCanBreed(fish FishInstanceData, profile *catalog.FishingPondSpecies, now time.Time) bool {
	if profile == nil || normalizePondLifeState(fish.LifeState) != "alive" || fish.AgeDays < profile.MaturityAgeDays || fish.Satiety < 80 || fish.Health < 0.85 || fish.Condition < 0.9 || fish.PreferredFeedHours < fishingPondBreedFeedHours {
		return false
	}
	if fish.LastBredAt == nil {
		return true
	}
	return now.Sub(*fish.LastBredAt) >= fishingPondBreedCooldown
}

// Target waits are real-time means for one continuously eligible designated pair.
func fishingPondBreedExpectedHours(species *catalog.FishingSpecies, designatedPair bool) float64 {
	if species == nil {
		return 0
	}
	hours := 6.0
	switch strings.ToLower(species.Rarity) {
	case "uncommon":
		hours = 8
	case "rare":
		hours = 10
	case "epic":
		hours = 13
	case "legendary", "mythic":
		hours = 16
	}
	if !designatedPair {
		hours *= 2
	}
	return hours
}

func fishingPondBreedChance(species *catalog.FishingSpecies, designatedPair bool, averageFertility float64, mashReady bool, stepHours float64) float64 {
	expectedHours := fishingPondBreedExpectedHours(species, designatedPair)
	if expectedHours <= 0 || stepHours <= 0 {
		return 0
	}
	hourlyRate := 1 / expectedHours
	hourlyRate += 0.025 * clampFloat(averageFertility, -1, 1) / 24
	if mashReady {
		hourlyRate *= 1.5
	}
	hourlyChance := clampFloat(hourlyRate, 0, 1)
	return clampFloat(1-math.Pow(1-hourlyChance, stepHours), 0, 1)
}

func inheritPondGenes(female, male FishInstanceData, seed string) FishGenes {
	inbreeding := sharesPondLineage(female, male)
	inherit := func(a, b float64, salt string) float64 {
		value := (a+b)/2 + (pondUnit(seed+salt)-0.5)*0.16
		if inbreeding {
			value -= 0.08 + pondUnit(seed+salt+":inbreed")*0.08
		}
		return roundFloat(clampFloat(value, -1, 1), 4)
	}
	return FishGenes{SizePotential: inherit(female.Genes.SizePotential, male.Genes.SizePotential, ":size"), GrowthRate: inherit(female.Genes.GrowthRate, male.Genes.GrowthRate, ":growth"), FeedEfficiency: inherit(female.Genes.FeedEfficiency, male.Genes.FeedEfficiency, ":feed"), Vitality: inherit(female.Genes.Vitality, male.Genes.Vitality, ":vitality"), Lifespan: inherit(female.Genes.Lifespan, male.Genes.Lifespan, ":life"), Fertility: inherit(female.Genes.Fertility, male.Genes.Fertility, ":fertility")}
}

func sharesPondLineage(a, b FishInstanceData) bool {
	for _, x := range a.ParentInstanceIDs {
		for _, y := range b.ParentInstanceIDs {
			if x > 0 && x == y {
				return true
			}
		}
	}
	return false
}
func pondGenesAreZero(g FishGenes) bool {
	return g.SizePotential == 0 && g.GrowthRate == 0 && g.FeedEfficiency == 0 && g.Vitality == 0 && g.Lifespan == 0 && g.Fertility == 0
}
func pondGene(seed string) float64 { return roundFloat((pondUnit(seed)*2-1)*0.65, 4) }
func pondUnit(seed string) float64 { return float64(stableFishingHash(seed)%1_000_000) / 1_000_000 }
func normalizePondLifeState(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "dead") {
		return "dead"
	}
	return "alive"
}
func normalizedPondSex(value string, instanceID int64, speciesID string) string {
	if value == "female" || value == "male" {
		return value
	}
	if pondUnit(fmt.Sprintf("%d:%s:pond-sex", instanceID, speciesID)) < 0.5 {
		return "male"
	}
	return "female"
}
func pondFeedTotal(stock map[string]float64) float64 {
	total := 0.0
	for _, v := range stock {
		if v > 0 {
			total += v
		}
	}
	return total
}
func pondContainsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
func timePointer(value time.Time) *time.Time { copyValue := value; return &copyValue }
func pondMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pondPhenotypes(genes FishGenes) []string {
	traits := []struct {
		name  string
		value float64
	}{{"large_frame", genes.SizePotential}, {"fast_growth", genes.GrowthRate}, {"efficient_feeder", genes.FeedEfficiency}, {"hardy", genes.Vitality}, {"long_lived", genes.Lifespan}, {"fertile", genes.Fertility}}
	out := make([]string, 0, 2)
	sort.Slice(traits, func(i, j int) bool { return traits[i].value > traits[j].value })
	for _, trait := range traits {
		if trait.value >= 0.28 {
			out = append(out, trait.name)
			if len(out) == 2 {
				break
			}
		}
	}
	return out
}

func pondPhenotypeEffects(genes FishGenes) []FishingPondPhenotypeEffect {
	values := map[string]float64{
		"large_frame":      0.10 * clampFloat(genes.SizePotential, -1, 1),
		"fast_growth":      0.25 * clampFloat(genes.GrowthRate, -1, 1),
		"efficient_feeder": 0.20 * clampFloat(genes.FeedEfficiency, -1, 1),
		"hardy":            0.18 * clampFloat(genes.Vitality, -1, 1),
		"long_lived":       0.12 * clampFloat(genes.Lifespan, -1, 1),
		"fertile":          0.025 * clampFloat(genes.Fertility, -1, 1),
	}
	phenotypes := pondPhenotypes(genes)
	out := make([]FishingPondPhenotypeEffect, 0, len(phenotypes))
	for _, phenotype := range phenotypes {
		out = append(out, FishingPondPhenotypeEffect{Key: phenotype, Value: roundFloat(values[phenotype], 4)})
	}
	return out
}

func pondLedgerMeta(source, target string, direction InventoryAuditDirection, eventKind string) InventoryLedgerMeta {
	return pondLedgerMetaForRuleset(source, target, direction, eventKind, FishingPondRulesetVersion)
}

func pondLedgerMetaForRuleset(source, target string, direction InventoryAuditDirection, eventKind, rulesetVersion string) InventoryLedgerMeta {
	return InventoryLedgerMeta{SourceSurface: source, TargetSurface: target, Direction: direction, Details: map[string]any{"type": eventKind, "rulesetVersion": rulesetVersion}}
}
