package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"strings"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

const (
	FishingInfusionRulesetVersion = "fish-infusion-v3"
	fishingInfusionMediumItemID   = "scaleglow_medium"
	fishingInfusionCarrierItemID  = "engrave_stone"
	fishingInfusionReservedState  = "alchemy_fish_reserved"
)

type FishingInfusionPreview struct {
	FishInstanceID          int64                          `json:"fishInstanceId"`
	SpeciesID               string                         `json:"speciesId"`
	TraitKind               string                         `json:"traitKind"`
	Attribute               string                         `json:"attribute"`
	LengthCM                float64                        `json:"lengthCm"`
	WeightKG                float64                        `json:"weightKg"`
	Power                   float64                        `json:"power"`
	Mutations               []string                       `json:"mutations,omitempty"`
	MutationPowerMultiplier float64                        `json:"mutationPowerMultiplier"`
	MutationBonusAttributes []FishingInfusionMutationBonus `json:"mutationBonusAttributes,omitempty"`
	BaseRange               catalog.AlchemyValueRange      `json:"baseRange"`
	EffectiveRange          catalog.AlchemyValueRange      `json:"effectiveRange"`
	FailureFloor            float64                        `json:"failureFloor"`
	FailChance              float64                        `json:"failChance"`
	GoldCost                int64                          `json:"goldCost"`
	MediumItemID            string                         `json:"mediumItemId"`
	MediumCount             int64                          `json:"mediumCount"`
	RulesetVersion          string                         `json:"rulesetVersion"`
}

type FishingInfusionMutationBonus struct {
	Attribute      string                    `json:"attribute"`
	MutationIDs    []string                  `json:"mutationIds"`
	BaseRange      catalog.AlchemyValueRange `json:"baseRange"`
	EffectiveRange catalog.AlchemyValueRange `json:"effectiveRange"`
	FailureFloor   float64                   `json:"failureFloor"`
}

func fishingInfusionRarityParameters(rarity string) (float64, float64) {
	switch strings.ToLower(strings.TrimSpace(rarity)) {
	case "uncommon":
		return 75, 200
	case "rare":
		return 120, 320
	case "epic":
		return 180, 480
	case "legendary", "mythic":
		return 260, 720
	default:
		return 40, 110
	}
}

func fishingInfusionPower(species catalog.FishingSpecies, lengthCM, weightKG float64) float64 {
	base, capValue := fishingInfusionRarityParameters(species.Rarity)
	lengthCM = math.Max(0, lengthCM)
	weightKG = math.Max(0, weightKG)
	if species.ShapeFactor > 0 && lengthCM > 0 {
		referenceWeight := species.ShapeFactor * math.Pow(lengthCM, 3) / 100_000
		weightKG = math.Min(weightKG, referenceWeight*1.2)
	}
	// Length remains the stable biological anchor. Weight still rewards condition,
	// but no longer explodes when a species' real-world body mass is recalibrated.
	return math.Min(capValue, base+0.30*lengthCM+3*weightKG)
}

func (s *Store) fishingInfusionGoldCost() int64 {
	value := s.config.Alchemy.InfusionBaseGoldCost
	if value <= 0 {
		return 32
	}
	return value
}

func (s *Store) FishingInfusionPreview(ctx context.Context, userID, fishInstanceID int64) (FishingInfusionPreview, error) {
	var preview FishingInfusionPreview
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		derived, err := s.resolveAlchemyDerivedForItemTx(ctx, tx, userID, QueueRuntimeItem{})
		if err != nil {
			return err
		}
		preview, err = s.fishingInfusionPreviewTx(ctx, tx, userID, fishInstanceID, "inventory", derived)
		return err
	})
	return preview, err
}

func (s *Store) fishingInfusionPreviewTx(ctx context.Context, tx pgx.Tx, userID, fishInstanceID int64, requiredState string, derived alchemyDerived) (FishingInfusionPreview, error) {
	if fishInstanceID <= 0 {
		return FishingInfusionPreview{}, ErrAlchemyArgsInvalid
	}
	var templateItemID string
	var fishRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT template_item_id, COALESCE(fish_data, '{}'::jsonb)
		FROM player_item_instances
		WHERE instance_id = $1 AND user_id = $2 AND state = $3 AND quantity = 1
		FOR UPDATE
	`, fishInstanceID, userID, requiredState).Scan(&templateItemID, &fishRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return FishingInfusionPreview{}, ErrInvalidInstanceState
	}
	if err != nil {
		return FishingInfusionPreview{}, err
	}
	if templateItemID != "fish_catch" {
		return FishingInfusionPreview{}, ErrAlchemyArgsInvalid
	}
	var fish FishInstanceData
	if err := json.Unmarshal(fishRaw, &fish); err != nil || strings.TrimSpace(fish.SpeciesID) == "" {
		return FishingInfusionPreview{}, ErrAlchemyArgsInvalid
	}
	if normalizePondLifeState(fish.LifeState) != "alive" {
		return FishingInfusionPreview{}, ErrAlchemyArgsInvalid
	}
	species := s.gameData.FishingSpeciesByID(fish.SpeciesID)
	profile := s.gameData.FishingInfusionProfileBySpeciesID(fish.SpeciesID)
	if species == nil || profile == nil || profile.UnitScale <= 0 || strings.TrimSpace(profile.AttrKey) == "" {
		return FishingInfusionPreview{}, ErrAlchemyArgsInvalid
	}
	lengthCM := fish.LengthCM
	weightKG := fish.WeightKG
	cultured := strings.EqualFold(strings.TrimSpace(fish.Origin), "cultured")
	if cultured {
		lengthCM = math.Min(lengthCM, species.MaxLengthCM)
		wildWeightCap := fishingPondMaximumWeight(species)
		weightKG = math.Min(weightKG, wildWeightCap)
	}
	mutations := s.configuredFishMutationIDs(fish.Genes.Mutations)
	mutationPowerMultiplier := s.fishingMutationPrimaryValueMultiplier(mutations)
	power := fishingInfusionPower(*species, lengthCM, weightKG) * mutationPowerMultiplier
	baseMin := power * 0.65 * profile.UnitScale
	baseMax := power * profile.UnitScale
	effectiveMin, effectiveMax := applyAlchemyDerivedRange(baseMin, baseMax, derived.Precision, derived.Stability)
	baseRange := catalog.AlchemyValueRange{
		Min: s.roundAlchemyAttributeValue(profile.AttrKey, baseMin),
		Max: s.roundAlchemyAttributeValue(profile.AttrKey, baseMax),
	}
	effectiveRange := catalog.AlchemyValueRange{
		Min: s.roundAlchemyAttributeValue(profile.AttrKey, effectiveMin),
		Max: s.roundAlchemyAttributeValue(profile.AttrKey, effectiveMax),
	}
	if effectiveRange.Min > effectiveRange.Max {
		effectiveRange.Min, effectiveRange.Max = effectiveRange.Max, effectiveRange.Min
	}
	mutationBonusAttributes := make([]FishingInfusionMutationBonus, 0)
	for _, bonus := range s.fishingMutationAlchemyBonuses(mutations) {
		effectiveBonusMin, effectiveBonusMax := applyAlchemyDerivedRange(bonus.Range.Min, bonus.Range.Max, derived.Precision, derived.Stability)
		baseBonusRange := catalog.AlchemyValueRange{
			Min: s.roundAlchemyAttributeValue(bonus.Attribute, bonus.Range.Min),
			Max: s.roundAlchemyAttributeValue(bonus.Attribute, bonus.Range.Max),
		}
		effectiveBonusRange := catalog.AlchemyValueRange{
			Min: s.roundAlchemyAttributeValue(bonus.Attribute, effectiveBonusMin),
			Max: s.roundAlchemyAttributeValue(bonus.Attribute, effectiveBonusMax),
		}
		if effectiveBonusRange.Min > effectiveBonusRange.Max {
			effectiveBonusRange.Min, effectiveBonusRange.Max = effectiveBonusRange.Max, effectiveBonusRange.Min
		}
		mutationBonusAttributes = append(mutationBonusAttributes, FishingInfusionMutationBonus{
			Attribute: bonus.Attribute, MutationIDs: append([]string(nil), bonus.MutationIDs...), BaseRange: baseBonusRange,
			EffectiveRange: effectiveBonusRange, FailureFloor: effectiveBonusRange.Min,
		})
	}
	failChance := applyAlchemyStabilityToFailChance(0.15, derived.Stability)
	goldCost := s.fishingInfusionGoldCost()
	mediumCount := int64(1)
	if cultured {
		goldCost *= 2
		mediumCount = 2
	}
	return FishingInfusionPreview{
		FishInstanceID:          fishInstanceID,
		SpeciesID:               fish.SpeciesID,
		TraitKind:               profile.TraitKind,
		Attribute:               profile.AttrKey,
		LengthCM:                lengthCM,
		WeightKG:                weightKG,
		Power:                   roundAlchemyValue(power, 3),
		Mutations:               mutations,
		MutationPowerMultiplier: roundAlchemyValue(mutationPowerMultiplier, 4),
		MutationBonusAttributes: mutationBonusAttributes,
		BaseRange:               baseRange,
		EffectiveRange:          effectiveRange,
		FailureFloor:            effectiveRange.Min,
		FailChance:              roundAlchemyValue(failChance, 6),
		GoldCost:                goldCost,
		MediumItemID:            fishingInfusionMediumItemID,
		MediumCount:             mediumCount,
		RulesetVersion:          FishingInfusionRulesetVersion,
	}, nil
}

func (s *Store) reserveFishingInfusionInstancesTx(ctx context.Context, tx pgx.Tx, userID int64, args *alchemyBenchArgs) error {
	if args == nil || args.FishInstanceID <= 0 || args.CarrierInstanceID <= 0 {
		return ErrAlchemyArgsInvalid
	}
	if _, err := s.fishingInfusionPreviewTx(ctx, tx, userID, args.FishInstanceID, "inventory", alchemyDerived{}); err != nil {
		return err
	}
	carrier, err := s.LockInstanceTx(ctx, tx, args.CarrierInstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAffixCarrierNotFound
		}
		return err
	}
	if carrier.UserID != userID {
		return ErrInstanceNotOwned
	}
	if carrier.State != "inventory" || carrier.TemplateItem != fishingInfusionCarrierItemID || len(decodeAffixSlots(carrier.AffixesRaw)) > 0 {
		return ErrAffixInvalidCarrier
	}
	reservedCarrierID, err := s.SplitOneFromStackTx(ctx, tx, carrier.InstanceID, userID)
	if err != nil {
		return err
	}
	args.CarrierInstanceID = reservedCarrierID
	if err := s.SetStateTx(ctx, tx, args.FishInstanceID, userID, "inventory", fishingInfusionReservedState); err != nil {
		return err
	}
	return s.SetStateTx(ctx, tx, reservedCarrierID, userID, "inventory", fishingInfusionReservedState)
}

func (s *Store) releaseFishingInfusionInstancesTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyBenchArgs) error {
	for _, instanceID := range []int64{args.FishInstanceID, args.CarrierInstanceID} {
		if instanceID <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_item_instances
			SET state = 'inventory', updated_at = now()
			WHERE instance_id = $1 AND user_id = $2 AND state = $3
		`, instanceID, userID, fishingInfusionReservedState); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) resolveFishingInfusionSlotTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyBenchArgs, derived alchemyDerived, rng *rand.Rand) (EquipmentAffixSlot, FishingInfusionPreview, bool, error) {
	preview, err := s.fishingInfusionPreviewTx(ctx, tx, userID, args.FishInstanceID, fishingInfusionReservedState, derived)
	if err != nil {
		return EquipmentAffixSlot{}, FishingInfusionPreview{}, false, err
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(args.FishInstanceID ^ args.CarrierInstanceID))
	}
	failed := rng.Float64() < preview.FailChance
	value := preview.EffectiveRange.Min
	if !failed {
		value = sampleLuckSkewed(preview.EffectiveRange.Min, preview.EffectiveRange.Max, derived.LuckyChance*4, rng)
	}
	value = s.roundAlchemyAttributeValue(preview.Attribute, value)
	profile := s.gameData.FishingInfusionProfileBySpeciesID(preview.SpeciesID)
	species := s.gameData.FishingSpeciesByID(preview.SpeciesID)
	if profile == nil || species == nil {
		return EquipmentAffixSlot{}, FishingInfusionPreview{}, false, ErrAlchemyArgsInvalid
	}
	bodyLimit := int64(2)
	if preview.Attribute == "fishing_efficiency" {
		bodyLimit = 1
	}
	components := []alchemyCraftComponent{{
		SourceKind:               "fish",
		TraitID:                  "fish:" + preview.SpeciesID,
		Family:                   "fish",
		Attr:                     preview.Attribute,
		Quantity:                 1,
		ValueRange:               preview.EffectiveRange,
		CurrentValue:             value,
		MaxEffectivePerEquipment: 1,
		MaxEffectivePerBody:      bodyLimit,
	}}
	attrMods := map[string]float64{preview.Attribute: value}
	mutationBonusValues := make([]map[string]any, 0, len(preview.MutationBonusAttributes))
	for _, bonus := range preview.MutationBonusAttributes {
		bonusValue := bonus.EffectiveRange.Min
		if !failed {
			bonusValue = sampleLuckSkewed(bonus.EffectiveRange.Min, bonus.EffectiveRange.Max, derived.LuckyChance*4, rng)
		}
		bonusValue = s.roundAlchemyAttributeValue(bonus.Attribute, bonusValue)
		attrMods[bonus.Attribute] = s.roundAlchemyAttributeValue(bonus.Attribute, attrMods[bonus.Attribute]+bonusValue)
		components = append(components, alchemyCraftComponent{
			SourceKind:               "fish_mutation",
			TraitID:                  "fish-mutation:" + strings.Join(bonus.MutationIDs, "+"),
			Family:                   "fish",
			Attr:                     bonus.Attribute,
			Quantity:                 1,
			ValueRange:               bonus.EffectiveRange,
			CurrentValue:             bonusValue,
			MaxEffectivePerEquipment: 1,
			MaxEffectivePerBody:      1,
		})
		mutationBonusValues = append(mutationBonusValues, map[string]any{
			"attribute": bonus.Attribute, "mutationIds": bonus.MutationIDs, "valueRange": bonus.EffectiveRange, "value": bonusValue,
		})
	}
	params := map[string]any{
		"attr":              preview.Attribute,
		"attrMods":          attrMods,
		"alchemyKind":       preview.TraitKind,
		"alchemyComponents": components,
		"fishInfusion": map[string]any{
			"version":                 FishingInfusionRulesetVersion,
			"speciesId":               preview.SpeciesID,
			"lengthCm":                preview.LengthCM,
			"weightKg":                preview.WeightKG,
			"power":                   preview.Power,
			"valueRange":              preview.EffectiveRange,
			"mutations":               preview.Mutations,
			"mutationPowerMultiplier": preview.MutationPowerMultiplier,
			"mutationBonusAttributes": mutationBonusValues,
			"failed":                  failed,
		},
	}
	return EquipmentAffixSlot{
		SlotIndex:             0,
		AffixID:               encodeCustomAffixID("add_stat", params, stableValueSum(attrMods), 4),
		RarityID:              species.Rarity,
		SourceKind:            "fish",
		AlchemyRulesetVersion: FishingInfusionRulesetVersion,
		DisplayNameKey:        "game.fishing.fishInfusionAffix",
		DisplayNameArgs:       map[string]any{},
		ValuePrecision:        4,
		AttrMods:              attrMods,
		EffectType:            "add_stat",
		EffectParams:          params,
	}, preview, failed, nil
}

func (e *alchemyExecutor) settleFishingInfusionTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, args alchemyBenchArgs, completed int, reservedCosts map[string]int64) (int, map[string]int64, map[string]int64, bool, string, error) {
	if completed <= 0 {
		return 0, map[string]int64{}, map[string]int64{}, false, "", nil
	}
	costPreview, err := e.store.fishingInfusionPreviewTx(ctx, tx, userID, args.FishInstanceID, fishingInfusionReservedState, alchemyDerived{})
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	expectedCosts := map[string]int64{
		fishingInfusionMediumItemID: costPreview.MediumCount,
		GoldCoinItemID:              costPreview.GoldCost,
	}
	for itemID, quantity := range expectedCosts {
		if reservedCosts[itemID] < quantity {
			return 0, nil, nil, false, "", ErrInsufficientItem
		}
	}
	carrier, err := e.store.LockInstanceTx(ctx, tx, args.CarrierInstanceID)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	if carrier.UserID != userID {
		return 0, nil, nil, false, "", ErrInstanceNotOwned
	}
	if carrier.State != fishingInfusionReservedState || carrier.TemplateItem != fishingInfusionCarrierItemID || carrier.Quantity != 1 || len(decodeAffixSlots(carrier.AffixesRaw)) > 0 {
		return 0, nil, nil, false, "", ErrAffixInvalidCarrier
	}
	derived, err := e.store.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	rng := alchemyQueueRNG(userID, item.QueueID, item.DoneCount, item.ActionID+":fish")
	slot, preview, failed, err := e.store.resolveFishingInfusionSlotTx(ctx, tx, userID, args, derived, rng)
	if err != nil {
		return 0, nil, nil, false, "", err
	}
	if err := e.store.SetAffixesTx(ctx, tx, carrier.InstanceID, []EquipmentAffixSlot{slot}); err != nil {
		return 0, nil, nil, false, "", err
	}
	if err := e.store.SetStateTx(ctx, tx, carrier.InstanceID, userID, fishingInfusionReservedState, "inventory"); err != nil {
		return 0, nil, nil, false, "", err
	}
	if err := e.store.refreshInstanceBaseSnapshotTx(ctx, tx, carrier.InstanceID, userID); err != nil {
		return 0, nil, nil, false, "", err
	}
	if err := e.store.LedgerConsumeInstanceTx(ctx, tx, userID, args.FishInstanceID, "fish_catch", 1, alchemyLedgerMeta(item, "alchemy_fish_reserved", "alchemy_bench", InventoryAuditDirectionConsume, "alchemy_fish_consumed", map[string]any{
		"speciesId": preview.SpeciesID,
		"lengthCm":  preview.LengthCM,
		"weightKg":  preview.WeightKG,
		"mutations": preview.Mutations,
	})); err != nil {
		return 0, nil, nil, false, "", err
	}
	carrierID := carrier.InstanceID
	meta := alchemyLedgerMeta(item, "alchemy_bench", "inventory", InventoryAuditDirectionGain, "alchemy_fish_infusion_result", map[string]any{
		"carrierInstanceId": carrier.InstanceID,
		"fishInstanceId":    args.FishInstanceID,
		"speciesId":         preview.SpeciesID,
		"attribute":         preview.Attribute,
		"mutations":         preview.Mutations,
		"failed":            failed,
		"rulesetVersion":    FishingInfusionRulesetVersion,
	})
	e.store.recordInventoryAuditTx(tx, InventoryAuditEvent{
		UserID:        userID,
		Category:      meta.Category,
		Direction:     meta.Direction,
		SubjectType:   meta.SubjectType,
		SubjectID:     meta.SubjectID,
		EventType:     meta.EventType,
		InstanceID:    &carrierID,
		TemplateItem:  carrier.TemplateItem,
		Quantity:      1,
		SourceSurface: meta.SourceSurface,
		TargetSurface: meta.TargetSurface,
		Details:       meta.Details,
	})
	cancelAfter := item.TotalCount > 1 || completed > 1
	cancelReason := ""
	if cancelAfter {
		cancelReason = ActionQueueCancelReasonTargetInvalid
	}
	return 1, expectedCosts, map[string]int64{}, cancelAfter, cancelReason, nil
}
