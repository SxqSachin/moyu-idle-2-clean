package store

import (
	"fmt"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"
)

type AffixDefinitionLookup interface {
	AffixDefinitionByID(id string) *catalog.AffixDefinition
	ItemTraitByItemID(itemID string) *catalog.ItemTraitDefinition
}

func AffixMaxEffectivePerEquipment(def *catalog.AffixDefinition) int {
	if def == nil || def.MaxEffectivePerEquipment <= 0 {
		return 0
	}
	return int(def.MaxEffectivePerEquipment)
}

func AffixMaxEffectivePerBody(def *catalog.AffixDefinition) int {
	if def == nil || def.MaxEffectivePerBody <= 0 {
		return 0
	}
	return int(def.MaxEffectivePerBody)
}

func CountAffixIDInSlots(slots []EquipmentAffixSlot, affixID string) int {
	normalizedAffixID := strings.TrimSpace(affixID)
	if normalizedAffixID == "" {
		return 0
	}
	count := 0
	for _, slot := range slots {
		if strings.TrimSpace(slot.AffixID) == normalizedAffixID {
			count++
		}
	}
	return count
}

func WouldExceedEquipmentAffixLimit(slots []EquipmentAffixSlot, affixID string, additionalCount int, def *catalog.AffixDefinition) bool {
	if additionalCount <= 0 {
		return false
	}
	limit := AffixMaxEffectivePerEquipment(def)
	if limit <= 0 {
		return false
	}
	return CountAffixIDInSlots(slots, affixID)+additionalCount > limit
}

type alchemyLimitComponent struct {
	key                      string
	displayID                string
	maxEffectivePerEquipment int
	maxEffectivePerBody      int
}

func numberFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case jsonNumber:
		out, _ := typed.Float64()
		return out
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

type jsonNumber interface {
	Float64() (float64, error)
}

func alchemyLimitComponents(slot EquipmentAffixSlot, lookup AffixDefinitionLookup) []alchemyLimitComponent {
	if slot.EffectParams == nil {
		return nil
	}
	raw, ok := slot.EffectParams["alchemyComponents"]
	if !ok || raw == nil {
		return nil
	}
	rows, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]alchemyCraftComponent); ok {
			out := make([]any, 0, len(typed))
			for _, row := range typed {
				out = append(out, row)
			}
			rows = out
		} else if typed, ok := raw.([]alchemyInheritComponent); ok {
			out := make([]any, 0, len(typed))
			for _, row := range typed {
				out = append(out, row)
			}
			rows = out
		} else {
			return nil
		}
	}
	seen := map[string]bool{}
	out := make([]alchemyLimitComponent, 0, len(rows))
	appendMaterialLimit := func(itemID, traitID string, equipmentLimit, bodyLimit int) {
		itemID = strings.TrimSpace(itemID)
		traitID = strings.TrimSpace(traitID)
		keyID := itemID
		if keyID == "" {
			keyID = traitID
		}
		if keyID == "" {
			return
		}
		if lookup != nil && itemID != "" {
			if equipment, body := latestAlchemyTraitLimits(lookup, itemID, traitID); equipment > 0 || body > 0 {
				if equipment > 0 {
					equipmentLimit = equipment
				}
				if body > 0 {
					bodyLimit = body
				}
			}
		}
		key := "alchemy_material:" + keyID
		if seen[key] {
			return
		}
		seen[key] = true
		if equipmentLimit <= 0 {
			equipmentLimit = 2
		}
		if bodyLimit <= 0 {
			bodyLimit = 5
		}
		out = append(out, alchemyLimitComponent{
			key:                      key,
			displayID:                keyID,
			maxEffectivePerEquipment: equipmentLimit,
			maxEffectivePerBody:      bodyLimit,
		})
	}
	appendOrdinaryAffixLimit := func(affixID string, equipmentLimit, bodyLimit int) {
		affixID = strings.TrimSpace(affixID)
		if affixID == "" || strings.HasPrefix(affixID, "custom:") {
			return
		}
		if lookup != nil {
			def := lookup.AffixDefinitionByID(affixID)
			if equipment := AffixMaxEffectivePerEquipment(def); equipment > 0 {
				equipmentLimit = equipment
			}
			if body := AffixMaxEffectivePerBody(def); body > 0 {
				bodyLimit = body
			}
		}
		if equipmentLimit <= 0 && bodyLimit <= 0 {
			return
		}
		key := affixID
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, alchemyLimitComponent{
			key:                      key,
			displayID:                affixID,
			maxEffectivePerEquipment: equipmentLimit,
			maxEffectivePerBody:      bodyLimit,
		})
	}
	for _, row := range rows {
		switch typed := row.(type) {
		case alchemyCraftComponent:
			appendMaterialLimit(typed.ItemID, typed.TraitID, int(typed.MaxEffectivePerEquipment), int(typed.MaxEffectivePerBody))
		case alchemyInheritComponent:
			appendMaterialLimit(typed.ItemID, typed.TraitID, int(typed.MaxEffectivePerEquipment), int(typed.MaxEffectivePerBody))
			appendOrdinaryAffixLimit(typed.AffixID, int(typed.MaxEffectivePerEquipment), int(typed.MaxEffectivePerBody))
			for _, ancestry := range typed.Ancestry {
				appendOrdinaryAffixLimit(ancestry.AffixID, 0, 0)
			}
		case map[string]any:
			appendMaterialLimit(
				stringFromAny(typed["itemId"]),
				stringFromAny(typed["traitId"]),
				int(numberFromAny(typed["maxEffectivePerEquipment"])),
				int(numberFromAny(typed["maxEffectivePerBody"])),
			)
			appendOrdinaryAffixLimit(
				stringFromAny(typed["affixId"]),
				int(numberFromAny(typed["maxEffectivePerEquipment"])),
				int(numberFromAny(typed["maxEffectivePerBody"])),
			)
			for _, affixID := range ordinaryAffixIDsFromAlchemyAncestry(typed["ancestry"]) {
				appendOrdinaryAffixLimit(affixID, 0, 0)
			}
		default:
			continue
		}
	}
	return out
}

func ordinaryAffixIDsFromAlchemyAncestry(raw any) []string {
	rows, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, row := range rows {
		node, ok := row.(map[string]any)
		if !ok {
			continue
		}
		affixID := stringFromAny(node["affixId"])
		if affixID == "" || strings.HasPrefix(affixID, "custom:") || seen[affixID] {
			continue
		}
		seen[affixID] = true
		out = append(out, affixID)
	}
	return out
}

func latestAlchemyTraitLimits(lookup AffixDefinitionLookup, itemID, traitID string) (int, int) {
	if lookup == nil {
		return 0, 0
	}
	def := lookup.ItemTraitByItemID(strings.TrimSpace(itemID))
	if def == nil {
		return 0, 0
	}
	normalizedTraitID := strings.TrimSpace(traitID)
	for _, trait := range def.Traits {
		if normalizedTraitID != "" && strings.TrimSpace(trait.TraitID) != normalizedTraitID {
			continue
		}
		return int(trait.MaxEffectivePerEquipment), int(trait.MaxEffectivePerBody)
	}
	if normalizedTraitID == "" && len(def.Traits) > 0 {
		trait := def.Traits[0]
		return int(trait.MaxEffectivePerEquipment), int(trait.MaxEffectivePerBody)
	}
	return 0, 0
}

func CountAlchemyComponentInSlots(slots []EquipmentAffixSlot, componentKey string) int {
	return countAlchemyComponentInSlots(slots, componentKey, nil)
}

func countAlchemyComponentInSlots(slots []EquipmentAffixSlot, componentKey string, lookup AffixDefinitionLookup) int {
	componentKey = strings.TrimSpace(componentKey)
	if componentKey == "" {
		return 0
	}
	count := 0
	for _, slot := range slots {
		if strings.TrimSpace(slot.AffixID) == componentKey {
			count++
			continue
		}
		for _, component := range alchemyLimitComponents(slot, lookup) {
			if component.key == componentKey {
				count++
				break
			}
		}
	}
	return count
}

func WouldExceedCustomAlchemyEquipmentLimit(slots []EquipmentAffixSlot, candidate EquipmentAffixSlot) (string, int, bool) {
	return WouldExceedCustomAlchemyEquipmentLimitWithLookup(slots, candidate, nil)
}

func WouldExceedCustomAlchemyEquipmentLimitWithLookup(slots []EquipmentAffixSlot, candidate EquipmentAffixSlot, lookup AffixDefinitionLookup) (string, int, bool) {
	for _, component := range alchemyLimitComponents(candidate, lookup) {
		limit := component.maxEffectivePerEquipment
		if limit <= 0 {
			continue
		}
		if countAlchemyComponentInSlots(slots, component.key, lookup)+1 > limit {
			return component.displayID, limit, true
		}
	}
	return "", 0, false
}

func WouldExceedCustomAlchemyBodyLimit(bodySeen map[string]int, candidate EquipmentAffixSlot) (string, int, bool) {
	return WouldExceedCustomAlchemyBodyLimitWithLookup(bodySeen, candidate, nil)
}

func WouldExceedCustomAlchemyBodyLimitWithLookup(bodySeen map[string]int, candidate EquipmentAffixSlot, lookup AffixDefinitionLookup) (string, int, bool) {
	if bodySeen == nil {
		return "", 0, false
	}
	for _, component := range alchemyLimitComponents(candidate, lookup) {
		limit := component.maxEffectivePerBody
		if limit <= 0 {
			continue
		}
		if bodySeen[component.key]+1 > limit {
			return component.displayID, limit, true
		}
	}
	return "", 0, false
}

type indexedAffixSlot struct {
	index int
	slot  EquipmentAffixSlot
}

func sortAffixSlotsBySlotIndex(slots []EquipmentAffixSlot) []EquipmentAffixSlot {
	indexed := make([]indexedAffixSlot, 0, len(slots))
	for idx, slot := range slots {
		indexed = append(indexed, indexedAffixSlot{index: idx, slot: slot})
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		if indexed[i].slot.SlotIndex != indexed[j].slot.SlotIndex {
			return indexed[i].slot.SlotIndex < indexed[j].slot.SlotIndex
		}
		return indexed[i].index < indexed[j].index
	})
	sorted := make([]EquipmentAffixSlot, 0, len(indexed))
	for _, entry := range indexed {
		sorted = append(sorted, entry.slot)
	}
	return sorted
}

// FilterEffectiveAffixSlotsByLimit applies equipment/body max-effective constraints and
// returns only effective slots in deterministic order: slotIndex asc, then original order.
// Pass bodySeen=nil to apply equipment-only constraints.
func FilterEffectiveAffixSlotsByLimit(
	slots []EquipmentAffixSlot,
	lookup AffixDefinitionLookup,
	bodySeen map[string]int,
) []EquipmentAffixSlot {
	sorted := sortAffixSlotsBySlotIndex(slots)
	out := make([]EquipmentAffixSlot, 0, len(sorted))
	equipmentSeen := map[string]int{}
	for _, slot := range sorted {
		affixID := strings.TrimSpace(slot.AffixID)
		if affixID == "" {
			continue
		}
		var def *catalog.AffixDefinition
		if lookup != nil {
			def = lookup.AffixDefinitionByID(affixID)
		}
		equipmentLimit := AffixMaxEffectivePerEquipment(def)
		nextEquipmentCount := equipmentSeen[affixID] + 1
		if equipmentLimit > 0 && nextEquipmentCount > equipmentLimit {
			continue
		}
		components := alchemyLimitComponents(slot, lookup)
		skipCustom := false
		for _, component := range components {
			if component.maxEffectivePerEquipment > 0 && equipmentSeen[component.key]+1 > component.maxEffectivePerEquipment {
				skipCustom = true
				break
			}
			if bodySeen != nil && component.maxEffectivePerBody > 0 && bodySeen[component.key]+1 > component.maxEffectivePerBody {
				skipCustom = true
				break
			}
		}
		if skipCustom {
			continue
		}
		nextBodyCount := 0
		if bodySeen != nil {
			nextBodyCount = bodySeen[affixID] + 1
			bodyLimit := AffixMaxEffectivePerBody(def)
			if bodyLimit > 0 && nextBodyCount > bodyLimit {
				continue
			}
		}
		equipmentSeen[affixID] = nextEquipmentCount
		if bodySeen != nil {
			bodySeen[affixID] = nextBodyCount
		}
		for _, component := range components {
			equipmentSeen[component.key]++
			if bodySeen != nil {
				bodySeen[component.key]++
			}
		}
		out = append(out, slot)
	}
	return out
}
