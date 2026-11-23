package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/enchantment"
)

type AffixDetachResult struct {
	EquipmentInstanceID int64   `json:"equipmentInstanceId"`
	AffixItemInstanceID int64   `json:"affixItemInstanceId"`
	SlotIndex           int     `json:"slotIndex"`
	ToolItemID          string  `json:"toolItemId"`
	DestroyChance       float64 `json:"destroyChance"`
	Destroyed           bool    `json:"destroyed"`
}

type AffixAttachResult struct {
	EquipmentInstanceID int64 `json:"equipmentInstanceId"`
	AffixItemInstanceID int64 `json:"affixItemInstanceId"`
	SlotIndex           int   `json:"slotIndex"`
}

type AffixEnchantResult struct {
	EquipmentInstanceID int64               `json:"equipmentInstanceId"`
	ToolItemID          string              `json:"toolItemId"`
	SlotIndex           int                 `json:"slotIndex"`
	ConsumeCount        int64               `json:"consumeCount"`
	Failed              bool                `json:"failed"`
	Affix               *EquipmentAffixSlot `json:"affix,omitempty"`
}

func (s *Store) DetachEquipmentAffix(ctx context.Context, userID, equipmentInstanceID int64, slotIndex int, toolItemID string, carrierInstanceID int64) (AffixDetachResult, error) {
	rule := s.gameData.AffixDetachRuleByToolID(toolItemID)
	if rule == nil || !rule.Active {
		return AffixDetachResult{}, ErrAffixInvalidCarrier
	}
	if carrierInstanceID <= 0 {
		return AffixDetachResult{}, ErrAffixInvalidCarrier
	}

	result := AffixDetachResult{
		EquipmentInstanceID: equipmentInstanceID,
		SlotIndex:           slotIndex,
		ToolItemID:          toolItemID,
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		equipmentRow, err := s.LockInstanceTx(ctx, tx, equipmentInstanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAffixCarrierNotFound
			}
			return err
		}
		if equipmentRow.UserID != userID {
			return ErrInstanceNotOwned
		}
		if equipmentRow.State != "inventory" {
			return ErrInvalidInstanceState
		}
		template := s.gameData.EquipmentTemplateByID(equipmentRow.TemplateItem)
		if template == nil {
			return ErrAffixInvalidCarrier
		}
		if template.AffixSlotCount <= 0 {
			return ErrAffixSlotNotFound
		}
		if !isValidAffixSlotIndex(slotIndex, template.AffixSlotCount) {
			return ErrAffixSlotNotFound
		}

		affixes := decodeAffixSlots(equipmentRow.AffixesRaw)
		if countFilledAffixSlots(affixes, template.AffixSlotCount) <= 0 {
			return ErrAffixSlotNotFound
		}
		slotPos := -1
		for idx := range affixes {
			if !isValidAffixSlotIndex(affixes[idx].SlotIndex, template.AffixSlotCount) {
				continue
			}
			if strings.TrimSpace(affixes[idx].AffixID) == "" {
				continue
			}
			if affixes[idx].SlotIndex == slotIndex {
				slotPos = idx
				break
			}
		}
		if slotPos < 0 {
			return ErrAffixSlotNotFound
		}
		removed := affixes[slotPos]
		affixes = append(affixes[:slotPos], affixes[slotPos+1:]...)

		carrierRow, err := s.LockInstanceTx(ctx, tx, carrierInstanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAffixCarrierNotFound
			}
			return err
		}
		if carrierRow.UserID != userID {
			return ErrInstanceNotOwned
		}
		if carrierRow.State != "inventory" {
			return ErrInvalidInstanceState
		}
		carrierRule := s.gameData.AffixCarrierRuleByItemID(carrierRow.TemplateItem)
		if carrierRule == nil || !carrierRule.Active {
			return ErrAffixInvalidCarrier
		}
		if !carrierRuleAllowsRarity(carrierRule, removed.RarityID) {
			return ErrAffixRarityNotAllowed
		}
		if len(decodeAffixSlots(carrierRow.AffixesRaw)) > 0 {
			return ErrAffixSlotOccupied
		}

		targetCarrierID := carrierRow.InstanceID
		if carrierRow.Quantity > 1 {
			targetCarrierID, err = s.SplitOneFromStackTx(ctx, tx, carrierRow.InstanceID, userID)
			if err != nil {
				return err
			}
		}

		luck := 0.0
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(final_value, 0)
			FROM player_attributes
			WHERE user_id = $1 AND attr_key = 'luck'
		`, userID).Scan(&luck); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		destroyChance := rule.DestroyChanceBase + luck*rule.LuckImpactFactor
		if destroyChance < 0 {
			destroyChance = 0
		}
		if destroyChance > 1 {
			destroyChance = 1
		}
		destroyed := rand.New(rand.NewSource(time.Now().UnixNano())).Float64() < destroyChance

		if err = s.SetAffixesTx(ctx, tx, targetCarrierID, []EquipmentAffixSlot{removed}); err != nil {
			return err
		}
		if err = s.refreshInstanceBaseSnapshotTx(ctx, tx, targetCarrierID, userID); err != nil {
			return err
		}
		if destroyed {
			if err = s.LedgerConsumeInstanceTx(ctx, tx, userID, equipmentInstanceID, equipmentRow.TemplateItem, 1, InventoryLedgerMeta{
				SourceSurface: "inventory",
				TargetSurface: "affix_detach_destroy",
				Details: map[string]any{
					"slotIndex": slotIndex,
					"toolItem":  toolItemID,
				},
			}); err != nil {
				return err
			}
		} else if err = s.SetAffixesTx(ctx, tx, equipmentInstanceID, affixes); err != nil {
			return err
		} else if err = s.refreshInstanceBaseSnapshotTx(ctx, tx, equipmentInstanceID, userID); err != nil {
			return err
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, userID, toolItemID, 1, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "affix_detach",
			Details: map[string]any{
				"equipmentInstanceId": equipmentInstanceID,
				"slotIndex":           slotIndex,
			},
		}); err != nil {
			return err
		}
		if err = s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
			return err
		}

		result.AffixItemInstanceID = targetCarrierID
		result.DestroyChance = destroyChance
		result.Destroyed = destroyed
		return nil
	})
	if err != nil {
		return AffixDetachResult{}, err
	}
	return result, nil
}

func (s *Store) AttachEquipmentAffix(ctx context.Context, userID, equipmentInstanceID, affixInstanceID int64, slotIndex int) (AffixAttachResult, error) {
	result := AffixAttachResult{
		EquipmentInstanceID: equipmentInstanceID,
		AffixItemInstanceID: affixInstanceID,
		SlotIndex:           slotIndex,
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		equipmentRow, err := s.LockInstanceTx(ctx, tx, equipmentInstanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAffixCarrierNotFound
			}
			return err
		}
		if equipmentRow.UserID != userID {
			return ErrInstanceNotOwned
		}
		if equipmentRow.State != "inventory" {
			return ErrInvalidInstanceState
		}
		template := s.gameData.EquipmentTemplateByID(equipmentRow.TemplateItem)
		if template == nil {
			return ErrAffixInvalidCarrier
		}
		if template.AffixSlotCount <= 0 {
			return ErrAffixSlotNotFound
		}
		if !isValidAffixSlotIndex(slotIndex, template.AffixSlotCount) {
			return ErrAffixSlotNotFound
		}
		equippedAffixes := decodeAffixSlots(equipmentRow.AffixesRaw)
		occupiedSlots := collectOccupiedAffixSlotIndexes(equippedAffixes, template.AffixSlotCount)
		if len(occupiedSlots) >= int(template.AffixSlotCount) {
			return ErrAffixNoEmptySlot
		}
		if _, exists := occupiedSlots[slotIndex]; exists {
			return ErrAffixSlotOccupied
		}

		carrierRow, err := s.LockInstanceTx(ctx, tx, affixInstanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAffixCarrierNotFound
			}
			return err
		}
		if carrierRow.UserID != userID {
			return ErrInstanceNotOwned
		}
		if carrierRow.State != "inventory" {
			return ErrInvalidInstanceState
		}
		carrierRule := s.gameData.AffixCarrierRuleByItemID(carrierRow.TemplateItem)
		if carrierRule == nil || !carrierRule.Active {
			return ErrAffixInvalidCarrier
		}
		if carrierRow.Quantity != 1 {
			return ErrAffixInvalidCarrier
		}
		carrierAffixes := decodeAffixSlots(carrierRow.AffixesRaw)
		if len(carrierAffixes) != 1 {
			return ErrAffixInvalidCarrier
		}
		attached := carrierAffixes[0]
		if strings.TrimSpace(attached.AffixID) == "" || strings.TrimSpace(attached.RarityID) == "" {
			return ErrAffixInvalidCarrier
		}
		attachedDef := s.gameData.AffixDefinitionByID(attached.AffixID)
		if !customAlchemyAffixAllowsEquipment(template, attached) {
			return ErrAffixNotAllowedByEquipment
		}
		if !equipmentAllowsAffix(template, attachedDef, attached.AffixID) {
			return ErrAffixNotAllowedByEquipment
		}
		if !carrierRuleAllowsRarity(carrierRule, attached.RarityID) {
			return ErrAffixRarityNotAllowed
		}
		if WouldExceedEquipmentAffixLimit(equippedAffixes, attached.AffixID, 1, attachedDef) {
			return NewAffixEquipmentMaxEffectiveExceededError(attached.AffixID, AffixMaxEffectivePerEquipment(attachedDef))
		}
		if componentID, limit, exceeded := WouldExceedCustomAlchemyEquipmentLimitWithLookup(equippedAffixes, attached, s.gameData); exceeded {
			return NewAffixEquipmentMaxEffectiveExceededError(componentID, limit)
		}
		attached.SlotIndex = slotIndex
		equippedAffixes = append(equippedAffixes, attached)

		if err = s.LedgerConsumeInstanceTx(ctx, tx, userID, affixInstanceID, carrierRow.TemplateItem, 1, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "affix_attach",
			Details: map[string]any{
				"equipmentInstanceId": equipmentInstanceID,
				"slotIndex":           slotIndex,
			},
		}); err != nil {
			return err
		}
		if err = s.SetAffixesTx(ctx, tx, equipmentInstanceID, equippedAffixes); err != nil {
			return err
		}
		if err = s.refreshInstanceBaseSnapshotTx(ctx, tx, equipmentInstanceID, userID); err != nil {
			return err
		}
		return s.onPlayerAttributesChangedTx(ctx, tx, userID)
	})
	if err != nil {
		return AffixAttachResult{}, err
	}
	return result, nil
}

func (s *Store) EnchantEquipmentAffix(ctx context.Context, userID, equipmentInstanceID int64, slotIndex int, toolItemID string) (AffixEnchantResult, error) {
	rule := s.gameData.AffixEnchantRuleByToolID(toolItemID)
	if rule == nil || !rule.Active {
		return AffixEnchantResult{}, ErrAffixEnchantRuleNotFound
	}
	result := AffixEnchantResult{
		EquipmentInstanceID: equipmentInstanceID,
		ToolItemID:          toolItemID,
		SlotIndex:           slotIndex,
		ConsumeCount:        rule.ConsumeCount,
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		equipmentRow, err := s.LockInstanceTx(ctx, tx, equipmentInstanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAffixCarrierNotFound
			}
			return err
		}
		if equipmentRow.UserID != userID {
			return ErrInstanceNotOwned
		}
		if equipmentRow.State != "inventory" {
			return ErrInvalidInstanceState
		}
		template := s.gameData.EquipmentTemplateByID(equipmentRow.TemplateItem)
		if template == nil {
			return ErrAffixInvalidCarrier
		}
		if template.AffixSlotCount <= 0 {
			return ErrAffixSlotNotFound
		}
		if !isValidAffixSlotIndex(slotIndex, template.AffixSlotCount) {
			return ErrAffixSlotNotFound
		}

		equippedAffixes := decodeAffixSlots(equipmentRow.AffixesRaw)
		occupiedSlots := collectOccupiedAffixSlotIndexes(equippedAffixes, template.AffixSlotCount)
		if len(occupiedSlots) >= int(template.AffixSlotCount) {
			return ErrAffixNoEmptySlot
		}
		if _, exists := occupiedSlots[slotIndex]; exists {
			return ErrAffixSlotOccupied
		}

		_, weights, err := s.enchantAffixCandidates(rule, equipmentRow.TemplateItem)
		if err != nil {
			return err
		}
		startedAt := time.Now().UTC()
		session, err := s.loadOperationEffectSessionTx(ctx, tx, userID, startedAt)
		if err != nil {
			return err
		}
		op := operationeffect.Operation{Key: fmt.Sprintf("enchant:%d:%d:%d", equipmentInstanceID, slotIndex, startedAt.UnixNano()), Domain: enchantment.Domain, ActionID: "equipment.enchant", ActorKind: operationeffect.ActorPlayer, ActorID: userID, StartedAt: startedAt, Capabilities: map[string]bool{enchantment.Eligible: enchantment.EligibleWeights(weights)}}
		var prepared *operationeffect.PreparedOperation
		modifiedRule := *rule
		if session != nil {
			p := session.Prepare(op)
			prepared = &p
			bonus, err := enchantment.Resolve(p.Snapshot())
			if err != nil {
				return err
			}
			modifiedRule.RarityWeights = enchantment.AdjustWeights(weights, bonus)
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, userID, toolItemID, rule.ConsumeCount, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "affix_enchant",
			Details: map[string]any{
				"equipmentInstanceId": equipmentInstanceID,
				"slotIndex":           slotIndex,
			},
		}); err != nil {
			return err
		}
		if len(rule.Costs) > 0 {
			if err = s.applyAffixCostsTx(ctx, tx, userID, rule.Costs, equipmentInstanceID, slotIndex); err != nil {
				return err
			}
		}

		if prepared != nil {
			if _, err := session.Commit(*prepared); err != nil {
				return err
			}
			if _, err := s.flushOperationEffectSessionTx(ctx, tx, userID, session, startedAt); err != nil {
				return err
			}
		}
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		failed := rng.Float64() < rule.FailChance
		result.Failed = failed
		if failed {
			return s.ApplyAchievementEventsTx(ctx, tx, userID, []AchievementEvent{{
				Kind:       AchievementEventEnchantCount,
				Count:      1,
				Successful: false,
			}})
		}

		slot, err := s.rollEnchantAffix(&modifiedRule, equipmentRow.TemplateItem, slotIndex, rng)
		if err != nil {
			return err
		}
		def := s.gameData.AffixDefinitionByID(strings.TrimSpace(slot.AffixID))
		if WouldExceedEquipmentAffixLimit(equippedAffixes, slot.AffixID, 1, def) {
			return NewAffixEquipmentMaxEffectiveExceededError(slot.AffixID, AffixMaxEffectivePerEquipment(def))
		}
		equippedAffixes = append(equippedAffixes, slot)
		if err = s.SetAffixesTx(ctx, tx, equipmentInstanceID, equippedAffixes); err != nil {
			return err
		}
		if err = s.refreshInstanceBaseSnapshotTx(ctx, tx, equipmentInstanceID, userID); err != nil {
			return err
		}
		if err = s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
			return err
		}
		result.Affix = &slot
		return s.ApplyAchievementEventsTx(ctx, tx, userID, []AchievementEvent{{
			Kind:       AchievementEventEnchantCount,
			Count:      1,
			Successful: true,
		}})
	})
	if err != nil {
		return AffixEnchantResult{}, err
	}
	return result, nil
}

func (s *Store) applyAffixCostsTx(ctx context.Context, tx pgx.Tx, userID int64, costs map[string]int64, equipmentInstanceID int64, slotIndex int) error {
	for key, qty := range costs {
		if qty <= 0 {
			continue
		}
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, key, qty, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "affix_enchant",
			Details: map[string]any{
				"type":                "affix_cost",
				"equipmentInstanceId": equipmentInstanceID,
				"slotIndex":           slotIndex,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) enchantAffixCandidates(rule *catalog.AffixEnchantRule, templateID string) (map[string][]catalog.AffixDefinition, map[string]float64, error) {
	definitions := make([]catalog.AffixDefinition, 0, len(s.gameData.AffixDefinitions))
	allowedAffixIDs := s.resolveEnchantAllowedAffixIDs(rule, templateID)
	if rule != nil && len(rule.AllowedAffixScopes) > 0 && len(allowedAffixIDs) == 0 {
		return nil, nil, ErrAffixInvalidCarrier
	}
	allowedSet := make(map[string]struct{}, len(allowedAffixIDs))
	for _, id := range allowedAffixIDs {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		allowedSet[normalized] = struct{}{}
	}
	for _, def := range s.gameData.AffixDefinitions {
		if !def.Active || strings.TrimSpace(def.ID) == "" {
			continue
		}
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[def.ID]; !ok {
				continue
			}
		}
		definitions = append(definitions, def)
	}
	definitions = s.filterEnchantAffixCandidatesByEquipment(templateID, definitions)
	if len(definitions) == 0 {
		return nil, nil, ErrAffixInvalidCarrier
	}

	candidatesByRarity := make(map[string][]catalog.AffixDefinition, len(rule.RarityWeights))
	compatibleWeights := make(map[string]float64, len(rule.RarityWeights))
	for rarityID, weight := range rule.RarityWeights {
		normalizedRarity := strings.TrimSpace(rarityID)
		if normalizedRarity == "" || weight <= 0 {
			continue
		}
		candidates := make([]catalog.AffixDefinition, 0, len(definitions))
		for _, def := range definitions {
			if affixSupportsRarity(def, normalizedRarity) {
				candidates = append(candidates, def)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		candidatesByRarity[normalizedRarity] = candidates
		compatibleWeights[normalizedRarity] = weight
	}
	return candidatesByRarity, compatibleWeights, nil
}

func (s *Store) rollEnchantAffix(rule *catalog.AffixEnchantRule, templateID string, slotIndex int, rng *rand.Rand) (EquipmentAffixSlot, error) {
	candidatesByRarity, compatibleWeights, err := s.enchantAffixCandidates(rule, templateID)
	if err != nil {
		return EquipmentAffixSlot{}, err
	}
	rarityID := rollByWeight(compatibleWeights, rng)
	if rarityID == "" {
		return EquipmentAffixSlot{}, ErrAffixEnchantNoRarityCandidate
	}
	candidates := candidatesByRarity[rarityID]
	if len(candidates) == 0 {
		return EquipmentAffixSlot{}, ErrAffixEnchantNoRarityCandidate
	}
	selected := candidates[rng.Intn(len(candidates))]

	return buildEquipmentAffixSlot(selected, rarityID, rng.Float64(), slotIndex), nil
}

func buildEquipmentAffixSlot(def catalog.AffixDefinition, rarityID string, rollValue float64, slotIndex int) EquipmentAffixSlot {
	if math.IsNaN(rollValue) || math.IsInf(rollValue, 0) {
		rollValue = 0
	}
	if rollValue < 0 {
		rollValue = 0
	}
	if rollValue > 1 {
		rollValue = 1
	}
	slot := EquipmentAffixSlot{
		SlotIndex:      slotIndex,
		AffixID:        strings.TrimSpace(def.ID),
		RarityID:       strings.TrimSpace(rarityID),
		RollValue:      rollValue,
		ValuePrecision: def.ValuePrecision,
		EffectType:     def.EffectType,
		EffectParams:   map[string]any{},
	}
	if def.EffectParams != nil {
		slot.EffectParams = resolveAffixEffectParamsByRarity(def.EffectParams, slot.RarityID)
	}
	attrMods := rollAffixEffects(def, slot.RarityID, rollValue)
	if len(attrMods) > 0 {
		slot.AttrMods = attrMods
	}
	return slot
}

func (s *Store) rollEnhanceMilestoneAffix(pool []catalog.EnhanceAffixMilestonePoolEntry, templateID string, slotIndex int, rng *rand.Rand) (EquipmentAffixSlot, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if len(pool) == 0 {
		return EquipmentAffixSlot{}, ErrAffixInvalidCarrier
	}
	selected := pool[rng.Intn(len(pool))]
	affixID := strings.TrimSpace(selected.AffixID)
	rarityID := strings.TrimSpace(selected.RarityID)
	if affixID == "" || rarityID == "" {
		return EquipmentAffixSlot{}, ErrAffixInvalidCarrier
	}
	def := s.gameData.AffixDefinitionByID(affixID)
	if def == nil || !def.Active {
		return EquipmentAffixSlot{}, ErrAffixInvalidCarrier
	}
	if !equipmentAllowsAffix(s.gameData.EquipmentTemplateByID(templateID), def, affixID) {
		return EquipmentAffixSlot{}, ErrAffixNotAllowedByEquipment
	}
	if !affixSupportsRarity(*def, rarityID) {
		return EquipmentAffixSlot{}, ErrAffixRarityNotAllowed
	}
	rollValue := rng.Float64()
	if selected.RollValue != nil {
		rollValue = *selected.RollValue
	}
	return buildEquipmentAffixSlot(*def, rarityID, rollValue, slotIndex), nil
}

func (s *Store) filterEnchantAffixCandidatesByEquipment(templateID string, definitions []catalog.AffixDefinition) []catalog.AffixDefinition {
	template := s.gameData.EquipmentTemplateByID(templateID)
	if template == nil {
		return definitions
	}
	filtered := make([]catalog.AffixDefinition, 0, len(definitions))
	for idx := range definitions {
		if equipmentAllowsAffix(template, &definitions[idx], definitions[idx].ID) {
			filtered = append(filtered, definitions[idx])
		}
	}
	return filtered
}

func (s *Store) resolveEnchantAllowedAffixIDs(rule *catalog.AffixEnchantRule, templateID string) []string {
	if rule == nil {
		return []string{}
	}
	template := s.gameData.EquipmentTemplateByID(templateID)
	if template == nil {
		return rule.AllowedAffix
	}
	if len(rule.AllowedAffixScopes) > 0 {
		scopes := selectEnchantScopes(rule, template)
		if len(scopes) == 0 {
			return []string{}
		}
		allowed := make([]string, 0)
		seen := make(map[string]struct{})
		for _, scope := range scopes {
			for _, affixID := range scope.AllowedAffix {
				normalized := strings.TrimSpace(affixID)
				if normalized == "" {
					continue
				}
				if _, exists := seen[normalized]; exists {
					continue
				}
				seen[normalized] = struct{}{}
				allowed = append(allowed, normalized)
			}
		}
		return allowed
	}
	return rule.AllowedAffix
}

func selectEnchantScopes(rule *catalog.AffixEnchantRule, template *catalog.EquipmentTemplate) []*catalog.AffixEnchantScopeDef {
	if rule == nil || template == nil {
		return nil
	}
	templateEquipType := strings.TrimSpace(template.EquipType)
	templateSlot := strings.TrimSpace(template.Slot)
	if templateEquipType == "" {
		return nil
	}
	templateWeaponClasses := make(map[string]struct{}, len(template.WeaponClass))
	for _, weaponClass := range template.WeaponClass {
		normalized := strings.TrimSpace(weaponClass)
		if normalized == "" {
			continue
		}
		templateWeaponClasses[normalized] = struct{}{}
	}
	bestPriority := -1
	selected := make([]*catalog.AffixEnchantScopeDef, 0)
	for idx := range rule.AllowedAffixScopes {
		scope := &rule.AllowedAffixScopes[idx]
		if strings.TrimSpace(scope.EquipType) != templateEquipType {
			continue
		}
		slotMatched := false
		if len(scope.Slots) == 0 {
			slotMatched = true
		} else if templateSlot != "" {
			for _, slot := range scope.Slots {
				if strings.TrimSpace(slot) == templateSlot {
					slotMatched = true
					break
				}
			}
		}
		if !slotMatched {
			continue
		}
		weaponClassMatched := false
		if len(scope.WeaponClasses) == 0 {
			weaponClassMatched = true
		} else {
			for _, weaponClass := range scope.WeaponClasses {
				if _, exists := templateWeaponClasses[strings.TrimSpace(weaponClass)]; exists {
					weaponClassMatched = true
					break
				}
			}
		}
		if !weaponClassMatched {
			continue
		}
		priority := 0
		if len(scope.Slots) > 0 {
			priority += 2
		}
		if len(scope.WeaponClasses) > 0 {
			priority += 1
		}
		if priority > bestPriority {
			bestPriority = priority
			selected = selected[:0]
		}
		if priority == bestPriority {
			selected = append(selected, scope)
		}
	}
	return selected
}

func isValidAffixSlotIndex(slotIndex int, slotCount int64) bool {
	return slotIndex >= 0 && slotCount > 0 && int64(slotIndex) < slotCount
}

func collectOccupiedAffixSlotIndexes(affixes []EquipmentAffixSlot, slotCount int64) map[int]struct{} {
	occupied := make(map[int]struct{}, len(affixes))
	for _, slot := range affixes {
		if !isValidAffixSlotIndex(slot.SlotIndex, slotCount) {
			continue
		}
		if strings.TrimSpace(slot.AffixID) == "" {
			continue
		}
		occupied[slot.SlotIndex] = struct{}{}
	}
	return occupied
}

func countFilledAffixSlots(affixes []EquipmentAffixSlot, slotCount int64) int {
	return len(collectOccupiedAffixSlotIndexes(affixes, slotCount))
}

func equipmentAllowsAffixID(template *catalog.EquipmentTemplate, affixID string) bool {
	if template == nil {
		return false
	}
	normalizedAffixID := strings.TrimSpace(affixID)
	if normalizedAffixID == "" {
		return false
	}
	if len(template.AffixAllowedIDs) == 0 {
		return true
	}
	for _, allowedAffixID := range template.AffixAllowedIDs {
		if strings.TrimSpace(allowedAffixID) == normalizedAffixID {
			return true
		}
	}
	return false
}

func normalizeAffixEquipType(raw string) string {
	if strings.TrimSpace(strings.ToLower(raw)) == "work" {
		return "work"
	}
	return "battle"
}

func customAlchemyAffixAllowsEquipment(template *catalog.EquipmentTemplate, slot EquipmentAffixSlot) bool {
	if template == nil {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(slot.AffixID), "custom:") {
		return true
	}
	rawKind := ""
	if slot.EffectParams != nil {
		rawKind = strings.TrimSpace(strings.ToLower(fmt.Sprint(slot.EffectParams["alchemyKind"])))
	}
	if rawKind == "" {
		return true
	}
	expected := "battle"
	switch rawKind {
	case "life", "work":
		expected = "work"
	case "combat", "battle":
		expected = "battle"
	default:
		return false
	}
	return normalizeAffixEquipType(template.EquipType) == expected
}

func equipmentAllowsAffix(template *catalog.EquipmentTemplate, def *catalog.AffixDefinition, affixID string) bool {
	if template == nil {
		return false
	}
	if !equipmentAllowsAffixID(template, affixID) {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(affixID), "custom:") {
		return true
	}
	if def == nil {
		return false
	}
	return normalizeAffixEquipType(template.EquipType) == normalizeAffixEquipType(def.Type)
}

func rollByWeight(weights map[string]float64, rng *rand.Rand) string {
	keys := make([]string, 0, len(weights))
	total := 0.0
	for key, weight := range weights {
		if weight > 0 {
			total += weight
			keys = append(keys, key)
		}
	}
	if total <= 0 {
		return ""
	}
	sort.Strings(keys)
	needle := rng.Float64() * total
	sum := 0.0
	for _, key := range keys {
		weight := weights[key]
		sum += weight
		if needle <= sum {
			return key
		}
	}
	return ""
}

func affixSupportsRarity(def catalog.AffixDefinition, rarityID string) bool {
	if strings.TrimSpace(rarityID) == "" {
		return false
	}
	if len(def.AllowedRarities) == 0 {
		return true
	}
	for _, rarity := range def.AllowedRarities {
		if strings.TrimSpace(rarity) == strings.TrimSpace(rarityID) {
			return true
		}
	}
	return false
}

func carrierRuleAllowsRarity(rule *catalog.AffixCarrierRule, rarityID string) bool {
	if rule == nil {
		return false
	}
	normalizedRarity := strings.TrimSpace(rarityID)
	if normalizedRarity == "" {
		return false
	}
	if len(rule.AllowedRarityIDs) == 0 {
		return true
	}
	for _, allowed := range rule.AllowedRarityIDs {
		if strings.TrimSpace(allowed) == normalizedRarity {
			return true
		}
	}
	return false
}

func rollAffixEffects(def catalog.AffixDefinition, rarityID string, roll float64) map[string]float64 {
	attrMods := map[string]float64{}
	params := resolveAffixEffectParamsByRarity(def.EffectParams, rarityID)
	if params == nil {
		return attrMods
	}
	if rawAttrMods, ok := params["attrMods"]; ok {
		if nodes, ok := rawAttrMods.(map[string]any); ok {
			for key, raw := range nodes {
				value, ok := rollNumericParam(raw, roll)
				if !ok {
					continue
				}
				attrMods[key] = roundByPrecision(value, def.ValuePrecision)
			}
		}
	}
	return attrMods
}

func resolveAffixEffectParamsByRarity(effectParams map[string]any, rarityID string) map[string]any {
	if effectParams == nil {
		return map[string]any{}
	}
	resolved := map[string]any{}
	for key, raw := range effectParams {
		resolved[key] = selectRarityScopedParam(raw, rarityID)
	}
	return resolved
}

func selectRarityScopedParam(raw any, rarityID string) any {
	nodes, ok := raw.(map[string]any)
	if !ok {
		return raw
	}
	normalizedRarity := strings.TrimSpace(rarityID)
	if normalizedRarity != "" {
		if scoped, ok := nodes[normalizedRarity]; ok {
			return scoped
		}
	}
	if scoped, ok := nodes["default"]; ok {
		return scoped
	}
	if scoped, ok := nodes["*"]; ok {
		return scoped
	}
	return raw
}

func toAnySlice(raw any) ([]any, bool) {
	switch rows := raw.(type) {
	case []any:
		return rows, true
	case []map[string]any:
		out := make([]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, row)
		}
		return out, true
	default:
		return nil, false
	}
}

func rollNumericParam(raw any, roll float64) (float64, bool) {
	switch node := raw.(type) {
	case float64:
		return node, true
	case int:
		return float64(node), true
	case int64:
		return float64(node), true
	case map[string]any:
		min, minOK := numericFromAny(node["min"])
		max, maxOK := numericFromAny(node["max"])
		if !minOK || !maxOK {
			return 0, false
		}
		if max < min {
			min, max = max, min
		}
		return min + (max-min)*roll, true
	default:
		return 0, false
	}
}

func numericFromAny(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}

func roundByPrecision(value float64, precision int64) float64 {
	if precision <= 0 {
		return math.Round(value)
	}
	pow := math.Pow10(int(precision))
	return math.Round(value*pow) / pow
}

type AffixSmeltResult struct {
	ConsumedCarrierInstanceIDs []int64            `json:"consumedCarrierInstanceIds"`
	Rewards                    []SmeltRewardGrant `json:"rewards"`
}

type SmeltRewardGrant struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

func (s *Store) SmeltCarrierAffixes(ctx context.Context, userID int64, carrierInstanceIDs []int64) (AffixSmeltResult, error) {
	if len(carrierInstanceIDs) == 0 {
		return AffixSmeltResult{}, ErrAffixInvalidCarrier
	}

	// deduplicate to prevent double-consume of the same instance
	seen := make(map[int64]struct{}, len(carrierInstanceIDs))
	deduped := make([]int64, 0, len(carrierInstanceIDs))
	for _, id := range carrierInstanceIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		deduped = append(deduped, id)
	}
	carrierInstanceIDs = deduped

	if len(carrierInstanceIDs) > 20 {
		return AffixSmeltResult{}, ErrAffixInvalidCarrier
	}

	smeltRule := s.gameData.DefaultAffixSmeltRule()
	if smeltRule == nil {
		return AffixSmeltResult{}, ErrAffixInvalidCarrier
	}

	rewardAccum := map[string]int64{}
	result := AffixSmeltResult{
		ConsumedCarrierInstanceIDs: make([]int64, 0, len(carrierInstanceIDs)),
	}

	err := s.withTx(ctx, func(tx pgx.Tx) error {
		for _, carrierID := range carrierInstanceIDs {
			carrierRow, err := s.LockInstanceTx(ctx, tx, carrierID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrAffixCarrierNotFound
				}
				return err
			}
			if carrierRow.UserID != userID {
				return ErrInstanceNotOwned
			}
			if carrierRow.State != "inventory" {
				return ErrInvalidInstanceState
			}
			carrierRule := s.gameData.AffixCarrierRuleByItemID(carrierRow.TemplateItem)
			if carrierRule == nil || !carrierRule.Active {
				return ErrAffixInvalidCarrier
			}

			affixes := decodeAffixSlots(carrierRow.AffixesRaw)
			for _, slot := range affixes {
				if strings.TrimSpace(slot.AffixID) == "" {
					continue
				}
				rewards := smeltRewardsForSlot(s.gameData, smeltRule, slot)
				for _, r := range rewards {
					rewardAccum[r.ItemID] += r.Quantity
				}
			}

			if err := s.LedgerConsumeInstanceTx(ctx, tx, userID, carrierID, carrierRow.TemplateItem, 1, InventoryLedgerMeta{
				SourceSurface: "inventory",
				TargetSurface: "affix_smelt",
			}); err != nil {
				return err
			}
			result.ConsumedCarrierInstanceIDs = append(result.ConsumedCarrierInstanceIDs, carrierID)
		}

		for itemID, qty := range rewardAccum {
			if qty <= 0 {
				continue
			}
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
				SourceSurface: "affix_smelt",
				TargetSurface: "inventory",
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return AffixSmeltResult{}, err
	}

	for itemID, qty := range rewardAccum {
		if qty > 0 {
			result.Rewards = append(result.Rewards, SmeltRewardGrant{ItemID: itemID, Quantity: qty})
		}
	}
	sort.Slice(result.Rewards, func(i, j int) bool {
		return result.Rewards[i].ItemID < result.Rewards[j].ItemID
	})
	return result, nil
}

func smeltRewardsForSlot(gameData catalog.Data, smeltRule *catalog.AffixSmeltRule, slot EquipmentAffixSlot) []SmeltRewardGrant {
	rarityID := strings.TrimSpace(slot.RarityID)
	var entries []catalog.AffixSmeltRewardEntry

	affixDef := gameData.AffixDefinitionByID(slot.AffixID)
	if affixDef != nil && affixDef.SmeltOverride != nil {
		if override, ok := affixDef.SmeltOverride[rarityID]; ok {
			entries = override
		}
	}
	if entries == nil {
		entries = smeltRule.RarityRewards[rarityID]
	}
	if len(entries) == 0 {
		return nil
	}

	rollRatio := smeltRollRatio(affixDef, slot)

	out := make([]SmeltRewardGrant, 0, len(entries))
	for _, e := range entries {
		minC := e.MinCount
		maxC := e.MaxCount
		if maxC < minC {
			maxC = minC
		}
		count := minC + int64(math.Round(float64(maxC-minC)*rollRatio))
		if count <= 0 {
			continue
		}
		out = append(out, SmeltRewardGrant{ItemID: e.ItemID, Quantity: count})
	}
	return out
}

func smeltRollRatio(affixDef *catalog.AffixDefinition, slot EquipmentAffixSlot) float64 {
	if affixDef == nil || len(slot.AttrMods) == 0 {
		return 0
	}
	attrModsRaw, ok := affixDef.EffectParams["attrMods"]
	if !ok {
		return 0
	}
	byRarity, ok := attrModsRaw.(map[string]any)
	if !ok {
		return 0
	}
	rarityEntry, ok := byRarity[slot.RarityID]
	if !ok {
		return 0
	}
	rarityMap, ok := rarityEntry.(map[string]any)
	if !ok {
		return 0
	}

	var ratioSum float64
	var ratioCount int
	for attrKey, rolledValue := range slot.AttrMods {
		attrRange, ok := rarityMap[attrKey]
		if !ok {
			continue
		}
		rangeMap, ok := attrRange.(map[string]any)
		if !ok {
			continue
		}
		minVal, hasMin := numericFromAny(rangeMap["min"])
		maxVal, hasMax := numericFromAny(rangeMap["max"])
		if !hasMin || !hasMax || maxVal <= minVal {
			continue
		}
		ratio := (rolledValue - minVal) / (maxVal - minVal)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		ratioSum += ratio
		ratioCount++
	}
	if ratioCount == 0 {
		return 0
	}
	return ratioSum / float64(ratioCount)
}
