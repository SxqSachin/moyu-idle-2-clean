package store

import (
	"math"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

const (
	arenaAlchemyDerivedPositiveMaxMultiplier = 2.25 * alchemyInfusionPositiveMaxRatio
	arenaAlchemyAbsoluteQuantityMultiplier   = int64(alchemyQuantitySoftCap)
	arenaAlchemyComboPowerAttribute          = "alchemy_combo_power"
)

var arenaAlchemyIgnoredBattleAttributes = map[string]struct{}{
	"battle":                 {},
	"battle_drop_rate_bonus": {},
	"battle_exp_bonus":       {},
}

type ArenaAlchemyEquipmentViolation struct {
	EquipmentInstanceID   int64   `json:"equipmentInstanceId"`
	TemplateItemID        string  `json:"templateItemId"`
	EquipmentName         string  `json:"equipmentName,omitempty"`
	EquippedSlot          string  `json:"equippedSlot,omitempty"`
	AffixSlotIndex        int     `json:"affixSlotIndex"`
	AlchemyRulesetVersion string  `json:"alchemyRulesetVersion,omitempty"`
	Attribute             string  `json:"attribute"`
	ActualValue           float64 `json:"actualValue"`
	AllowedValue          float64 `json:"allowedValue"`
	Reason                string  `json:"reason"`
}

func (s *Store) ArenaAlchemyEquipmentViolations(items []EquippedItem) []ArenaAlchemyEquipmentViolation {
	if s == nil || len(items) == 0 {
		return []ArenaAlchemyEquipmentViolation{}
	}
	battleAttributes := s.arenaAlchemyBattleAttributeSet()
	type globalLimitKey struct {
		attribute string
		inherited bool
	}
	globalLimits := make(map[globalLimitKey]float64)
	seenEquipment := make(map[int64]struct{})
	violations := make([]ArenaAlchemyEquipmentViolation, 0)
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.EquipType), "battle") {
			continue
		}
		if item.InstanceID > 0 {
			if _, exists := seenEquipment[item.InstanceID]; exists {
				continue
			}
			seenEquipment[item.InstanceID] = struct{}{}
		}
		for _, slot := range item.Affixes {
			if !isCustomAlchemyAffix(slot) {
				continue
			}
			allowed, complete := s.arenaAlchemyAffixAllowedValues(slot)
			inherited := isInheritedAlchemyAffix(slot)
			fishInfusion := strings.EqualFold(strings.TrimSpace(slot.SourceKind), "fish")
			attrs := make([]string, 0, len(slot.AttrMods))
			for rawAttr := range slot.AttrMods {
				attr := strings.TrimSpace(rawAttr)
				if _, ok := battleAttributes[attr]; ok {
					attrs = append(attrs, attr)
				}
			}
			sort.Strings(attrs)
			for _, attr := range attrs {
				actual := slot.AttrMods[attr]
				if math.IsNaN(actual) || math.IsInf(actual, 0) {
					violations = append(violations, arenaAlchemyViolationFor(item, slot, attr, 0, 0, "invalid_numeric_value"))
					continue
				}
				if actual <= 0 {
					continue
				}
				limit := allowed[attr]
				if fishInfusion && (limit <= 0 || !complete[attr]) {
					violations = append(violations, arenaAlchemyViolationFor(item, slot, attr, actual, 0, arenaAlchemyExcessReason(slot)))
					continue
				}
				if limit <= 0 || !complete[attr] {
					key := globalLimitKey{attribute: attr, inherited: inherited}
					fallback, exists := globalLimits[key]
					if !exists {
						fallback = s.arenaAlchemyGlobalAttributeLimit(attr, inherited)
						globalLimits[key] = fallback
					}
					if fallback > limit {
						limit = fallback
					}
				}
				if limit <= 0 {
					violations = append(violations, arenaAlchemyViolationFor(item, slot, attr, actual, 0, arenaAlchemyExcessReason(slot)))
					continue
				}
				tolerance := math.Max(0.001, math.Abs(limit)*0.000001)
				if actual <= limit+tolerance {
					continue
				}
				violations = append(violations, arenaAlchemyViolationFor(item, slot, attr, actual, limit, arenaAlchemyExcessReason(slot)))
			}
			violations = append(violations, s.arenaAlchemyComboPowerViolations(item, slot)...)
		}
	}
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].EquipmentInstanceID != violations[j].EquipmentInstanceID {
			return violations[i].EquipmentInstanceID < violations[j].EquipmentInstanceID
		}
		if violations[i].AffixSlotIndex != violations[j].AffixSlotIndex {
			return violations[i].AffixSlotIndex < violations[j].AffixSlotIndex
		}
		return violations[i].Attribute < violations[j].Attribute
	})
	return violations
}

func arenaAlchemyExcessReason(slot EquipmentAffixSlot) string {
	if strings.EqualFold(strings.TrimSpace(slot.SourceKind), "fish") {
		return "value_exceeds_fish_infusion_profile"
	}
	version := strings.TrimSpace(slot.AlchemyRulesetVersion)
	if version == "" {
		return "legacy_unversioned_value_exceeds_current_ruleset"
	}
	if version != AlchemyCurrentRulesetVersion {
		return "legacy_version_value_exceeds_current_ruleset"
	}
	return "value_exceeds_current_ruleset"
}

func arenaAlchemyViolationFor(item EquippedItem, slot EquipmentAffixSlot, attr string, actual, allowed float64, reason string) ArenaAlchemyEquipmentViolation {
	return ArenaAlchemyEquipmentViolation{
		EquipmentInstanceID:   item.InstanceID,
		TemplateItemID:        strings.TrimSpace(item.TemplateItemID),
		EquipmentName:         strings.TrimSpace(item.CustomData.Name),
		EquippedSlot:          strings.TrimSpace(item.EquippedSlot),
		AffixSlotIndex:        slot.SlotIndex,
		AlchemyRulesetVersion: strings.TrimSpace(slot.AlchemyRulesetVersion),
		Attribute:             attr,
		ActualValue:           actual,
		AllowedValue:          allowed,
		Reason:                reason,
	}
}

func (s *Store) arenaAlchemyBattleAttributeSet() map[string]struct{} {
	out := make(map[string]struct{})
	for _, definition := range s.gameData.Attributes {
		id := strings.TrimSpace(definition.ID)
		if id == "" || !strings.EqualFold(strings.TrimSpace(definition.Category), "battle") {
			continue
		}
		if _, ignored := arenaAlchemyIgnoredBattleAttributes[id]; ignored {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func isCustomAlchemyAffix(slot EquipmentAffixSlot) bool {
	switch strings.ToLower(strings.TrimSpace(slot.SourceKind)) {
	case "craft", "fish", "inherit", "inherit_orphan":
		return true
	}
	if slot.EffectParams != nil {
		if _, ok := slot.EffectParams["alchemyComponents"]; ok {
			return true
		}
		if _, ok := slot.EffectParams["alchemyKind"]; ok {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimSpace(slot.AffixID), "custom:")
}

func isInheritedAlchemyAffix(slot EquipmentAffixSlot) bool {
	sourceKind := strings.ToLower(strings.TrimSpace(slot.SourceKind))
	if sourceKind == "inherit" || sourceKind == "inherit_orphan" {
		return true
	}
	if slot.EffectParams == nil {
		return false
	}
	inheritSourceKind := strings.ToLower(strings.TrimSpace(stringFromAny(slot.EffectParams["inheritSourceKind"])))
	return inheritSourceKind == "inherit" || inheritSourceKind == "inherit_orphan" || boolFromAny(slot.EffectParams["inheritOrphan"])
}

func markArenaAlchemyResolution(complete map[string]bool, attr string, resolved bool) {
	attr = strings.TrimSpace(attr)
	if attr == "" {
		return
	}
	if current, exists := complete[attr]; exists {
		complete[attr] = current && resolved
		return
	}
	complete[attr] = resolved
}

func (s *Store) arenaAlchemyAffixAllowedValues(slot EquipmentAffixSlot) (map[string]float64, map[string]bool) {
	allowed := map[string]float64{}
	complete := map[string]bool{}
	if slot.EffectParams == nil {
		return allowed, complete
	}
	if strings.EqualFold(strings.TrimSpace(slot.SourceKind), "fish") {
		attr := strings.TrimSpace(stringFromAny(slot.EffectParams["attr"]))
		stamp, ok := slot.EffectParams["fishInfusion"].(map[string]any)
		if !ok || attr == "" || strings.TrimSpace(stringFromAny(stamp["version"])) != FishingInfusionRulesetVersion {
			if attr != "" {
				complete[attr] = false
			}
			return allowed, complete
		}
		rangeMap, ok := stamp["valueRange"].(map[string]any)
		if !ok {
			complete[attr] = false
			return allowed, complete
		}
		maximum := numberFromAny(rangeMap["max"])
		if maximum <= 0 || !isFiniteAlchemyNumber(maximum) {
			complete[attr] = false
			return allowed, complete
		}
		allowed[attr] = maximum
		complete[attr] = true
		return allowed, complete
	}
	rows := anySliceFromAlchemyComponents(slot.EffectParams["alchemyComponents"])
	if len(rows) == 0 {
		return allowed, complete
	}
	inherited := isInheritedAlchemyAffix(slot)
	allowOperationEffects := strings.TrimSpace(slot.AlchemyRulesetVersion) == AlchemyCurrentRulesetVersion
	comboMaterials := map[string]int64{}
	for _, row := range rows {
		component, ok := inheritComponentFromAny(row)
		if !ok || strings.TrimSpace(component.SourceKind) != "combo_material" || component.Quantity <= 0 {
			continue
		}
		itemID := strings.TrimSpace(component.ItemID)
		if itemID != "" {
			comboMaterials[itemID] += component.Quantity
		}
	}
	for _, row := range rows {
		component, ok := inheritComponentFromAny(row)
		if !ok {
			continue
		}
		modifiers := alchemyeffect.Modifiers{}
		if allowOperationEffects {
			modifiers, _ = alchemyOperationEffectModifiersFromComponent(component)
		}
		attr := strings.TrimSpace(component.Attr)
		sourceKind := strings.ToLower(strings.TrimSpace(component.SourceKind))
		switch {
		case strings.TrimSpace(component.ComboRecipeID) != "" && sourceKind != "combo_material":
			limits, resolved := s.arenaAlchemyComboLimitsWithModifiers(component.ComboRecipeID, comboMaterials, inherited, modifiers)
			for key, value := range limits {
				if value > 0 {
					allowed[key] += value
					markArenaAlchemyResolution(complete, key, resolved)
				}
			}
			if attr != "" && !resolved {
				markArenaAlchemyResolution(complete, attr, false)
			}
		case sourceKind == "ordinary_affix" || strings.EqualFold(strings.TrimSpace(component.SourceType), "affix"):
			if attr == "" {
				continue
			}
			limit := math.Max(component.ValueRange.Min, component.ValueRange.Max)
			if inherited && limit > 0 {
				limit *= s.alchemyInheritNumericCapMultiplier()
			}
			if limit > 0 && isFiniteAlchemyNumber(limit) {
				allowed[attr] += limit
				markArenaAlchemyResolution(complete, attr, true)
			} else {
				markArenaAlchemyResolution(complete, attr, false)
			}
		case attr != "":
			limit, resolved := s.arenaAlchemyTraitComponentLimitWithModifiers(component, inherited, modifiers)
			if resolved && limit > 0 {
				allowed[attr] += limit
				markArenaAlchemyResolution(complete, attr, true)
			} else {
				markArenaAlchemyResolution(complete, attr, false)
			}
		}
	}
	return allowed, complete
}

func (s *Store) arenaAlchemyTraitComponentLimit(component alchemyInheritComponent, inherited bool) (float64, bool) {
	return s.arenaAlchemyTraitComponentLimitWithModifiers(component, inherited, alchemyeffect.Modifiers{})
}

func (s *Store) arenaAlchemyTraitComponentLimitWithModifiers(component alchemyInheritComponent, inherited bool, modifiers alchemyeffect.Modifiers) (float64, bool) {
	attr := strings.TrimSpace(component.Attr)
	itemID := strings.TrimSpace(component.ItemID)
	traitID := strings.TrimSpace(component.TraitID)
	if attr == "" || itemID == "" || component.Quantity <= 0 {
		return 0, false
	}
	definition := s.gameData.ItemTraitByItemID(itemID)
	if definition == nil {
		return 0, false
	}
	for _, trait := range definition.Traits {
		if traitID != "" && strings.TrimSpace(trait.TraitID) != traitID {
			continue
		}
		for _, effect := range trait.Effects {
			if strings.TrimSpace(effect.Attr) != attr {
				continue
			}
			quantity := component.Quantity
			if absoluteMax := addAlchemyQuantityBonus(arenaAlchemyTraitAbsoluteMaxQuantity(trait), s.alchemyMaterialQuantityBonus(itemID, arenaAlchemyTraitAbsoluteMaxQuantity(trait), modifiers)); absoluteMax > 0 && quantity > absoluteMax {
				quantity = absoluteMax
			}
			perUnitMax := math.Max(effect.ValuePerUnit.Min, effect.ValuePerUnit.Max)
			if perUnitMax <= 0 || !isFiniteAlchemyNumber(perUnitMax) {
				return 0, true
			}
			limit := perUnitMax * float64(quantity) * arenaAlchemyDerivedPositiveMaxMultiplierWithModifiers(modifiers)
			limit *= s.arenaAlchemyMaxPositiveMediumMultiplier(trait.Family)
			if inherited {
				limit *= s.alchemyInheritNumericCapMultiplier()
			}
			return limit, isFiniteAlchemyNumber(limit)
		}
		if traitID != "" {
			return 0, false
		}
	}
	return 0, false
}

func arenaAlchemyDerivedPositiveMaxMultiplierWithModifiers(modifiers alchemyeffect.Modifiers) float64 {
	return 2.25 * alchemyInfusionPositiveCeilingMultiplier(modifiers)
}

func arenaAlchemyTraitAbsoluteMaxQuantity(trait catalog.AlchemyTraitEntry) int64 {
	basis := alchemyTraitUnitBasis(trait)
	baseUnits := trait.MaxBasisMultiplier
	if baseUnits <= 0 {
		baseUnits = 1
	}
	units := baseUnits * arenaAlchemyAbsoluteQuantityMultiplier
	if units < 1 {
		units = 1
	}
	return basis * units
}

func (s *Store) arenaAlchemyMaxPositiveMediumMultiplier(family string) float64 {
	maximum := 1.0
	for index := range s.gameData.AlchemyMediums {
		medium := &s.gameData.AlchemyMediums[index]
		if medium.Disabled || !medium.Active {
			continue
		}
		multiplier := medium.Modifiers.ValueMultiplier
		if multiplier == 0 {
			multiplier = 1
		}
		if family = strings.TrimSpace(family); family != "" {
			if affinity, ok := medium.Modifiers.FamilyAffinity[family]; ok {
				multiplier *= 1 + affinity
			}
		}
		if isFiniteAlchemyNumber(multiplier) && multiplier > maximum {
			maximum = multiplier
		}
	}
	return maximum
}

func (s *Store) arenaAlchemyComboLimits(comboRecipeID string, materials map[string]int64, inherited bool) (map[string]float64, bool) {
	return s.arenaAlchemyComboLimitsWithModifiers(comboRecipeID, materials, inherited, alchemyeffect.Modifiers{})
}

func (s *Store) arenaAlchemyComboLimitsWithModifiers(comboRecipeID string, materials map[string]int64, inherited bool, modifiers alchemyeffect.Modifiers) (map[string]float64, bool) {
	combo := s.arenaAlchemyComboByID(comboRecipeID)
	if combo == nil {
		return nil, false
	}
	maximumMultiplier, ok := s.arenaAlchemyComboAbsoluteMaxMultiplierWithModifiers(combo, modifiers)
	if !ok || maximumMultiplier <= 0 {
		return nil, false
	}
	multiplier := maximumMultiplier
	if match := s.matchAlchemyCombo(materials, combo.ID); match != nil && match.Multiplier > 0 && match.Multiplier < multiplier {
		multiplier = match.Multiplier
	}
	baseMax := math.Max(combo.OutputAffix.ValueRange.Min, combo.OutputAffix.ValueRange.Max) * float64(multiplier)
	if baseMax <= 0 || !isFiniteAlchemyNumber(baseMax) {
		return map[string]float64{}, true
	}
	value := baseMax * arenaAlchemyDerivedPositiveMaxMultiplierWithModifiers(modifiers) * s.arenaAlchemyMaxPositiveMediumMultiplier("")
	limits := map[string]float64{}
	for _, powerCorner := range []float64{0, value} {
		for _, multiplierCorner := range []int64{1, multiplier} {
			params := cloneMap(combo.OutputAffix.Params)
			if resolved, resolvedOK := resolveAlchemyComboParamTemplates(params, powerCorner, multiplierCorner).(map[string]any); resolvedOK {
				params = resolved
			}
			cornerLimits := extractAlchemyComboAttrMods(params)
			if attrRaw, exists := params["attr"]; exists && len(cornerLimits) == 0 {
				attr := cleanAlchemyComponentAttr(stringFromAny(attrRaw))
				if attr != "" {
					cornerLimits[attr] = powerCorner
				}
			}
			for attr, limit := range cornerLimits {
				if limit > limits[attr] {
					limits[attr] = limit
				}
			}
		}
	}
	capMultiplier := 1.0
	if inherited {
		capMultiplier = s.alchemyInheritNumericCapMultiplier()
	}
	for attr, limit := range limits {
		if limit <= 0 || !isFiniteAlchemyNumber(limit) {
			delete(limits, attr)
			continue
		}
		limits[attr] = limit * capMultiplier
	}
	return limits, true
}

func (s *Store) arenaAlchemyComboPowerViolations(item EquippedItem, slot EquipmentAffixSlot) []ArenaAlchemyEquipmentViolation {
	if slot.EffectParams == nil {
		return nil
	}
	rows := anySliceFromAlchemyComponents(slot.EffectParams["alchemyComponents"])
	if len(rows) == 0 {
		return nil
	}
	materials := map[string]int64{}
	components := make([]alchemyInheritComponent, 0, 1)
	for _, row := range rows {
		component, ok := inheritComponentFromAny(row)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(component.SourceKind), "combo_material") {
			if itemID := strings.TrimSpace(component.ItemID); itemID != "" && component.Quantity > 0 {
				materials[itemID] += component.Quantity
			}
			continue
		}
		if strings.TrimSpace(component.ComboRecipeID) != "" {
			components = append(components, component)
		}
	}
	if len(components) == 0 {
		return nil
	}
	inherited := isInheritedAlchemyAffix(slot)
	allowOperationEffects := strings.TrimSpace(slot.AlchemyRulesetVersion) == AlchemyCurrentRulesetVersion
	topLevelPower, hasTopLevelPower := numericFromAny(slot.EffectParams["comboPower"])
	out := make([]ArenaAlchemyEquipmentViolation, 0, len(components))
	for _, component := range components {
		modifiers := alchemyeffect.Modifiers{}
		if allowOperationEffects {
			modifiers, _ = alchemyOperationEffectModifiersFromComponent(component)
		}
		actual := component.CurrentValue
		if componentPower, ok := numericFromAny(component.EffectParams["comboPower"]); ok && componentPower > actual {
			actual = componentPower
		}
		if len(components) == 1 && hasTopLevelPower && topLevelPower > actual {
			actual = topLevelPower
		}
		if math.IsNaN(actual) || math.IsInf(actual, 0) {
			out = append(out, arenaAlchemyViolationFor(item, slot, arenaAlchemyComboPowerAttribute, 0, 0, "invalid_numeric_value"))
			continue
		}
		if actual <= 0 {
			continue
		}
		limit, resolved := s.arenaAlchemyComboPowerLimitWithModifiers(component.ComboRecipeID, materials, inherited, modifiers)
		if !resolved || limit <= 0 {
			limit = s.arenaAlchemyGlobalComboPowerLimit(inherited)
		}
		if limit <= 0 {
			out = append(out, arenaAlchemyViolationFor(item, slot, arenaAlchemyComboPowerAttribute, actual, 0, arenaAlchemyExcessReason(slot)))
			continue
		}
		tolerance := math.Max(0.001, math.Abs(limit)*0.000001)
		if actual > limit+tolerance {
			out = append(out, arenaAlchemyViolationFor(item, slot, arenaAlchemyComboPowerAttribute, actual, limit, arenaAlchemyExcessReason(slot)))
		}
	}
	return out
}

func (s *Store) arenaAlchemyComboPowerLimit(comboRecipeID string, materials map[string]int64, inherited bool) (float64, bool) {
	return s.arenaAlchemyComboPowerLimitWithModifiers(comboRecipeID, materials, inherited, alchemyeffect.Modifiers{})
}

func (s *Store) arenaAlchemyComboPowerLimitWithModifiers(comboRecipeID string, materials map[string]int64, inherited bool, modifiers alchemyeffect.Modifiers) (float64, bool) {
	combo := s.arenaAlchemyComboByID(comboRecipeID)
	if combo == nil || !strings.EqualFold(strings.TrimSpace(combo.Kind), "combat") {
		return 0, false
	}
	maximumMultiplier, ok := s.arenaAlchemyComboAbsoluteMaxMultiplierWithModifiers(combo, modifiers)
	if !ok || maximumMultiplier <= 0 {
		return 0, false
	}
	multiplier := maximumMultiplier
	if match := s.matchAlchemyCombo(materials, combo.ID); match != nil && match.Multiplier > 0 && match.Multiplier < multiplier {
		multiplier = match.Multiplier
	}
	limit := math.Max(combo.OutputAffix.ValueRange.Min, combo.OutputAffix.ValueRange.Max) * float64(multiplier)
	limit *= arenaAlchemyDerivedPositiveMaxMultiplierWithModifiers(modifiers) * s.arenaAlchemyMaxPositiveMediumMultiplier("")
	if inherited {
		limit *= s.alchemyInheritNumericCapMultiplier()
	}
	return limit, limit > 0 && isFiniteAlchemyNumber(limit)
}

func (s *Store) arenaAlchemyGlobalComboPowerLimit(inherited bool) float64 {
	maximum := 0.0
	for index := range s.gameData.AlchemyComboRecipes {
		combo := &s.gameData.AlchemyComboRecipes[index]
		limit, ok := s.arenaAlchemyComboPowerLimit(combo.ID, nil, inherited)
		if ok && limit > maximum {
			maximum = limit
		}
	}
	if inherited {
		maximum *= 2
	}
	return maximum
}

func (s *Store) arenaAlchemyComboByID(comboRecipeID string) *catalog.AlchemyComboRecipe {
	comboRecipeID = strings.TrimSpace(comboRecipeID)
	for index := range s.gameData.AlchemyComboRecipes {
		combo := &s.gameData.AlchemyComboRecipes[index]
		if !combo.Disabled && strings.TrimSpace(combo.ID) == comboRecipeID {
			return combo
		}
	}
	return nil
}

func (s *Store) arenaAlchemyComboAbsoluteMaxMultiplier(combo *catalog.AlchemyComboRecipe) (int64, bool) {
	return s.arenaAlchemyComboAbsoluteMaxMultiplierWithModifiers(combo, alchemyeffect.Modifiers{})
}

func (s *Store) arenaAlchemyComboAbsoluteMaxMultiplierWithModifiers(combo *catalog.AlchemyComboRecipe, modifiers alchemyeffect.Modifiers) (int64, bool) {
	if combo == nil || len(combo.Inputs) == 0 {
		return 0, false
	}
	maximum := int64(0)
	for itemID, inputQuantity := range combo.Inputs {
		if inputQuantity <= 0 {
			return 0, false
		}
		definition := s.gameData.ItemTraitByItemID(itemID)
		if definition == nil || len(definition.Traits) == 0 {
			return 0, false
		}
		quantity := addAlchemyQuantityBonus(arenaAlchemyTraitAbsoluteMaxQuantity(definition.Traits[0]), s.alchemyMaterialQuantityBonus(itemID, arenaAlchemyTraitAbsoluteMaxQuantity(definition.Traits[0]), modifiers))
		candidate := quantity / inputQuantity
		if candidate <= 0 {
			return 0, false
		}
		if maximum == 0 || candidate < maximum {
			maximum = candidate
		}
	}
	return maximum, maximum > 0
}

func (s *Store) arenaAlchemyGlobalAttributeLimit(attr string, inherited bool) float64 {
	byItem := make(map[string]float64)
	for _, definition := range s.gameData.ItemTraits {
		itemID := strings.TrimSpace(definition.ItemID)
		for _, trait := range definition.Traits {
			quantity := arenaAlchemyTraitAbsoluteMaxQuantity(trait)
			for _, effect := range trait.Effects {
				if strings.TrimSpace(effect.Attr) != attr {
					continue
				}
				perUnitMax := math.Max(effect.ValuePerUnit.Min, effect.ValuePerUnit.Max)
				if perUnitMax <= 0 || !isFiniteAlchemyNumber(perUnitMax) {
					continue
				}
				byItem[itemID] += perUnitMax * float64(quantity) * arenaAlchemyDerivedPositiveMaxMultiplier * s.arenaAlchemyMaxPositiveMediumMultiplier(trait.Family)
			}
		}
	}
	candidates := make([]float64, 0, len(byItem))
	for _, value := range byItem {
		if value > 0 && isFiniteAlchemyNumber(value) {
			candidates = append(candidates, value)
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(candidates)))
	maxSlots := catalog.DefaultBenchMaterialSlots
	for index := range s.gameData.AlchemyMediums {
		medium := &s.gameData.AlchemyMediums[index]
		if medium.Active && !medium.Disabled && catalog.EffectiveBenchSlots(medium) > maxSlots {
			maxSlots = catalog.EffectiveBenchSlots(medium)
		}
	}
	traitLimit := 0.0
	for index := 0; index < len(candidates) && index < maxSlots; index++ {
		traitLimit += candidates[index]
	}
	comboLimit := 0.0
	for index := range s.gameData.AlchemyComboRecipes {
		combo := &s.gameData.AlchemyComboRecipes[index]
		limits, ok := s.arenaAlchemyComboLimits(combo.ID, nil, false)
		if ok && limits[attr] > comboLimit {
			comboLimit = limits[attr]
		}
	}
	limit := math.Max(traitLimit, comboLimit)
	if inherited && limit > 0 {
		limit *= s.alchemyInheritNumericCapMultiplier() * 2
	}
	return limit
}

func isFiniteAlchemyNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
