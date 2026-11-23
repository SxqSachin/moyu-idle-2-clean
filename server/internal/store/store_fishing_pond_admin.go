package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

type AdminFishingPondGenesInput struct {
	SizePotential  *float64 `json:"sizePotential,omitempty"`
	GrowthRate     *float64 `json:"growthRate,omitempty"`
	FeedEfficiency *float64 `json:"feedEfficiency,omitempty"`
	Vitality       *float64 `json:"vitality,omitempty"`
	Lifespan       *float64 `json:"lifespan,omitempty"`
	Fertility      *float64 `json:"fertility,omitempty"`
}

type AdminFishingPondFishInput struct {
	UserID             int64                       `json:"userId"`
	SpeciesID          string                      `json:"speciesId,omitempty"`
	Mutations          []string                    `json:"mutations,omitempty"`
	AgeDays            *int                        `json:"ageDays,omitempty"`
	BioAgeFraction     *float64                    `json:"bioAgeFraction,omitempty"`
	Sex                string                      `json:"sex,omitempty"`
	LengthCM           *float64                    `json:"lengthCm,omitempty"`
	WeightKG           *float64                    `json:"weightKg,omitempty"`
	Condition          *float64                    `json:"condition,omitempty"`
	Health             *float64                    `json:"health,omitempty"`
	Generation         *int                        `json:"generation,omitempty"`
	Genes              *AdminFishingPondGenesInput `json:"genes,omitempty"`
	LongevityLimitDays *float64                    `json:"longevityLimitDays,omitempty"`
	GenomeRevealed     *bool                       `json:"genomeRevealed,omitempty"`
	Satiety            *float64                    `json:"satiety,omitempty"`
	PreferredFeedHours *float64                    `json:"preferredFeedHours,omitempty"`
	Name               string                      `json:"name,omitempty"`
	LabelColor         string                      `json:"labelColor,omitempty"`
}

type AdminFishingPondFishResult struct {
	InstanceID int64             `json:"instanceId"`
	CustomData ItemCustomization `json:"customData,omitempty"`
	Fish       FishInstanceData  `json:"fish"`
}

func (s *Store) AdminAddFishingPondFish(ctx context.Context, input AdminFishingPondFishInput) (AdminFishingPondFishResult, error) {
	if s == nil || input.UserID <= 0 {
		return AdminFishingPondFishResult{}, fmt.Errorf("%w: userId", ErrAdminFishingPondFishInvalid)
	}
	species, profile, mutations, customData, err := s.validateAdminFishingPondFishInput(input)
	if err != nil {
		return AdminFishingPondFishResult{}, err
	}
	now := time.Now().UTC()
	var result AdminFishingPondFishResult
	_, err = s.mutateFishingPond(ctx, input.UserID, now, func(tx pgx.Tx, _ int64, limits FishingPondLimits) error {
		rows, err := s.listFishingPondFishTx(ctx, tx, input.UserID, true)
		if err != nil {
			return err
		}
		if fishingPondZoneOccupiedCount(rows, fishingPondZoneMain)+1 > limits.FishCapacity {
			return ErrFishingPondFull
		}

		fish := buildAdminFishingPondFish(input, species, mutations, now)
		s.clampFishingPondFishSize(&fish, species)
		biomass := fish.WeightKG + fishingPondMainBiomass(rows)
		if biomass > limits.BiomassKG+0.0001 {
			return ErrFishingPondBiomassFull
		}

		instanceID, err := s.addFishInstanceTx(ctx, tx, input.UserID, fish)
		if err != nil {
			return err
		}
		s.initializeFishingPondFish(&fish, input.UserID, instanceID, now, profile)
		if input.Satiety != nil {
			fish.Satiety = roundFloat(*input.Satiety, 3)
		}
		applyAdminFishingGeneOverrides(&fish.Genes, input.Genes)
		seed := fmt.Sprintf("%d:%d:%s:pond-genes", input.UserID, instanceID, fish.SpeciesID)
		if input.LongevityLimitDays != nil {
			fish.LongevityLimitDays = *input.LongevityLimitDays
		} else if input.Genes != nil {
			fish.LongevityLimitDays = s.fishingPondLongevityLimitDays(fish, seed, profile)
		}
		fishRaw, err := json.Marshal(fish)
		if err != nil {
			return err
		}
		customRaw, err := encodeItemCustomization(customData)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_item_instances
			SET fish_data=$3::jsonb, custom_data=$4::jsonb, updated_at=$5
			WHERE user_id=$1 AND instance_id=$2
		`, input.UserID, instanceID, string(fishRaw), string(customRaw), now); err != nil {
			return err
		}
		ledgerMeta := pondLedgerMeta("admin_fishing_pond", "fishing_pond", InventoryAuditDirectionGain, "admin_pond_stock")
		ledgerMeta.Details["speciesId"] = fish.SpeciesID
		ledgerMeta.Details["mutations"] = mutations
		if err := s.LedgerSetStateTx(ctx, tx, instanceID, input.UserID, "inventory", fishingPondState, "fish_catch", 1, ledgerMeta); err != nil {
			return err
		}
		if err := s.addFishingPondEventTx(ctx, tx, input.UserID, "admin_stocked", &instanceID, nil, map[string]any{"speciesId": fish.SpeciesID, "mutations": mutations}); err != nil {
			return err
		}
		result = AdminFishingPondFishResult{InstanceID: instanceID, CustomData: customData, Fish: fish}
		return nil
	})
	if err != nil {
		return AdminFishingPondFishResult{}, err
	}
	return result, nil
}

func (s *Store) validateAdminFishingPondFishInput(input AdminFishingPondFishInput) (*catalog.FishingSpecies, *catalog.FishingPondSpecies, []string, ItemCustomization, error) {
	speciesID := strings.ToLower(strings.TrimSpace(input.SpeciesID))
	if speciesID == "" {
		speciesID = "crucian_carp"
	}
	species := s.gameData.FishingSpeciesByID(speciesID)
	profile := s.gameData.FishingPondSpeciesByID(speciesID)
	if species == nil || profile == nil {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: speciesId", ErrAdminFishingPondFishInvalid)
	}
	if input.AgeDays != nil && (*input.AgeDays < 0 || *input.AgeDays > 1_000_000) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: ageDays", ErrAdminFishingPondFishInvalid)
	}
	if input.BioAgeFraction != nil && (!finiteAdminFishingNumber(*input.BioAgeFraction) || *input.BioAgeFraction < 0 || *input.BioAgeFraction >= 1) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: bioAgeFraction", ErrAdminFishingPondFishInvalid)
	}
	sex := strings.ToLower(strings.TrimSpace(input.Sex))
	if sex != "" && sex != "female" && sex != "male" {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: sex", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.LengthCM, 0.1, species.MaxLengthCM) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: lengthCm", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.WeightKG, 0.001, fishingPondMaximumWeight(species)) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: weightKg", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.Condition, 0.1, 3) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: condition", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.Health, 0.001, 1) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: health", ErrAdminFishingPondFishInvalid)
	}
	if input.Generation != nil && (*input.Generation < 0 || *input.Generation > 1_000_000) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: generation", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.LongevityLimitDays, 1, float64(profile.LifespanDays)*20) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: longevityLimitDays", ErrAdminFishingPondFishInvalid)
	}
	resolvedAgeDays := species.MaturityAgeDays
	if input.AgeDays != nil {
		resolvedAgeDays = *input.AgeDays
	}
	resolvedBioAgeFraction := 0.0
	if input.BioAgeFraction != nil {
		resolvedBioAgeFraction = *input.BioAgeFraction
	}
	if input.LongevityLimitDays != nil && *input.LongevityLimitDays <= float64(resolvedAgeDays)+resolvedBioAgeFraction {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: longevityLimitDays must exceed biological age", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.Satiety, 0, 100) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: satiety", ErrAdminFishingPondFishInvalid)
	}
	if !validOptionalAdminFishingNumber(input.PreferredFeedHours, 0, 1_000_000) {
		return nil, nil, nil, ItemCustomization{}, fmt.Errorf("%w: preferredFeedHours", ErrAdminFishingPondFishInvalid)
	}
	if err := validateAdminFishingGeneOverrides(input.Genes); err != nil {
		return nil, nil, nil, ItemCustomization{}, err
	}
	mutations, err := s.normalizeAdminFishingMutations(input.Mutations)
	if err != nil {
		return nil, nil, nil, ItemCustomization{}, err
	}
	customData, err := s.normalizeAdminFishingCustomization(input.Name, input.LabelColor)
	if err != nil {
		return nil, nil, nil, ItemCustomization{}, err
	}
	return species, profile, mutations, customData, nil
}

func (s *Store) normalizeAdminFishingMutations(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	wanted := make(map[string]struct{}, len(values))
	for _, value := range values {
		id := strings.ToLower(strings.TrimSpace(value))
		if id == "" {
			return nil, fmt.Errorf("%w: mutations", ErrAdminFishingPondFishInvalid)
		}
		wanted[id] = struct{}{}
	}
	out := make([]string, 0, len(wanted))
	for _, mutation := range s.config.Fishing.Mutations {
		if _, ok := wanted[mutation.ID]; ok {
			out = append(out, mutation.ID)
			delete(wanted, mutation.ID)
		}
	}
	if len(wanted) != 0 {
		return nil, fmt.Errorf("%w: mutations", ErrAdminFishingPondFishInvalid)
	}
	return out, nil
}

func (s *Store) normalizeAdminFishingCustomization(name, labelColor string) (ItemCustomization, error) {
	name = strings.TrimSpace(name)
	labelColor = strings.ToLower(strings.TrimSpace(labelColor))
	if name == "" {
		if labelColor != "" {
			return ItemCustomization{}, fmt.Errorf("%w: labelColor requires name", ErrAdminFishingPondFishInvalid)
		}
		return ItemCustomization{}, nil
	}
	if utf8.RuneCountInString(name) > s.ItemRenameMaxNameRunes() || !IsValidItemCustomName(name) {
		return ItemCustomization{}, fmt.Errorf("%w: name", ErrAdminFishingPondFishInvalid)
	}
	if labelColor == "" {
		labelColor = s.DefaultItemRenameLabelColor()
	}
	if !s.IsValidItemRenameLabelColor(labelColor) {
		return ItemCustomization{}, fmt.Errorf("%w: labelColor", ErrAdminFishingPondFishInvalid)
	}
	return ItemCustomization{Name: name, LabelColor: labelColor}, nil
}

func buildAdminFishingPondFish(input AdminFishingPondFishInput, species *catalog.FishingSpecies, mutations []string, now time.Time) FishInstanceData {
	ageDays := species.MaturityAgeDays
	if input.AgeDays != nil {
		ageDays = *input.AgeDays
	}
	length := species.MaturityLengthCM
	if input.LengthCM != nil {
		length = *input.LengthCM
	}
	condition := 1.0
	if input.Condition != nil {
		condition = *input.Condition
	}
	weight := species.ShapeFactor * math.Pow(length, 3) / 100_000 * condition
	if input.WeightKG != nil {
		weight = *input.WeightKG
	}
	health := 1.0
	if input.Health != nil {
		health = *input.Health
	}
	generation := 0
	if input.Generation != nil {
		generation = *input.Generation
	}
	bioAgeFraction := 0.0
	if input.BioAgeFraction != nil {
		bioAgeFraction = *input.BioAgeFraction
	}
	satiety := 100.0
	if input.Satiety != nil {
		satiety = *input.Satiety
	}
	preferredFeedHours := 0.0
	if input.PreferredFeedHours != nil {
		preferredFeedHours = *input.PreferredFeedHours
	}
	genomeRevealed := false
	if input.GenomeRevealed != nil {
		genomeRevealed = *input.GenomeRevealed
	}
	return FishInstanceData{
		SpeciesID: species.ID, AgeDays: ageDays, BioAgeFraction: bioAgeFraction, Sex: strings.ToLower(strings.TrimSpace(input.Sex)),
		LengthCM: roundFloat(length, 1), WeightKG: roundFloat(weight, 3), Condition: roundFloat(condition, 3), Health: roundFloat(health, 3),
		WaterID: species.WaterID, SpotID: "fish_pond", CaughtAt: now, RulesetVersion: FishingPondRulesetVersion,
		Origin: "cultured", PondZone: fishingPondZoneMain, LifeState: "alive", Generation: generation, Genes: FishGenes{Mutations: append([]string(nil), mutations...)},
		GenomeRevealed: genomeRevealed, Satiety: roundFloat(satiety, 3), PreferredFeedHours: roundFloat(preferredFeedHours, 3), PondEnteredAt: timePointer(now),
	}
}

func validateAdminFishingGeneOverrides(genes *AdminFishingPondGenesInput) error {
	if genes == nil {
		return nil
	}
	for name, value := range map[string]*float64{
		"sizePotential": genes.SizePotential, "growthRate": genes.GrowthRate, "feedEfficiency": genes.FeedEfficiency,
		"vitality": genes.Vitality, "lifespan": genes.Lifespan, "fertility": genes.Fertility,
	} {
		if !validOptionalAdminFishingNumber(value, -1, 1) {
			return fmt.Errorf("%w: genes.%s", ErrAdminFishingPondFishInvalid, name)
		}
	}
	return nil
}

func applyAdminFishingGeneOverrides(genes *FishGenes, overrides *AdminFishingPondGenesInput) {
	if genes == nil || overrides == nil {
		return
	}
	if overrides.SizePotential != nil {
		genes.SizePotential = roundFloat(*overrides.SizePotential, 4)
	}
	if overrides.GrowthRate != nil {
		genes.GrowthRate = roundFloat(*overrides.GrowthRate, 4)
	}
	if overrides.FeedEfficiency != nil {
		genes.FeedEfficiency = roundFloat(*overrides.FeedEfficiency, 4)
	}
	if overrides.Vitality != nil {
		genes.Vitality = roundFloat(*overrides.Vitality, 4)
	}
	if overrides.Lifespan != nil {
		genes.Lifespan = roundFloat(*overrides.Lifespan, 4)
	}
	if overrides.Fertility != nil {
		genes.Fertility = roundFloat(*overrides.Fertility, 4)
	}
}

func validOptionalAdminFishingNumber(value *float64, minimum, maximum float64) bool {
	return value == nil || finiteAdminFishingNumber(*value) && *value >= minimum && *value <= maximum
}

func finiteAdminFishingNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
