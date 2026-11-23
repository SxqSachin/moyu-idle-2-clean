package store

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

type affixTemplateSignature struct {
	slotIndex int
	affixID   string
}

type affixFixedTemplateSignature struct {
	slotIndex int
	affixID   string
	rarityID  string
	rollValue float64
}

type fixedTemplateAffixMatchKind int

const (
	fixedTemplateAffixMatchInvalid fixedTemplateAffixMatchKind = iota
	fixedTemplateAffixMatchExact
	fixedTemplateAffixMatchMismatch
)

type recycleTemplateMatchKind int

const (
	recycleTemplateMatchNone recycleTemplateMatchKind = iota
	recycleTemplateMatchFull
	recycleTemplateMatchAffixMismatchOnly
)

func normalizeAffixTemplateSignatures(slots []EquipmentAffixSlot) []affixTemplateSignature {
	out := make([]affixTemplateSignature, 0, len(slots))
	for _, slot := range slots {
		affixID := strings.TrimSpace(slot.AffixID)
		if slot.SlotIndex < 0 || affixID == "" {
			continue
		}
		out = append(out, affixTemplateSignature{
			slotIndex: slot.SlotIndex,
			affixID:   affixID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].slotIndex != out[j].slotIndex {
			return out[i].slotIndex < out[j].slotIndex
		}
		return out[i].affixID < out[j].affixID
	})
	return out
}

func affixTemplateSignaturesEqual(actual []affixTemplateSignature, expected []affixTemplateSignature) bool {
	if len(actual) != len(expected) {
		return false
	}
	for idx := range actual {
		if actual[idx] != expected[idx] {
			return false
		}
	}
	return true
}

func normalizeFixedAffixTemplateSignatures(slots []EquipmentAffixSlot) []affixFixedTemplateSignature {
	out := make([]affixFixedTemplateSignature, 0, len(slots))
	for _, slot := range slots {
		affixID := strings.TrimSpace(slot.AffixID)
		rarityID := strings.TrimSpace(slot.RarityID)
		if slot.SlotIndex < 0 || affixID == "" || rarityID == "" {
			continue
		}
		out = append(out, affixFixedTemplateSignature{
			slotIndex: slot.SlotIndex,
			affixID:   affixID,
			rarityID:  rarityID,
			rollValue: slot.RollValue,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].slotIndex != out[j].slotIndex {
			return out[i].slotIndex < out[j].slotIndex
		}
		if out[i].affixID != out[j].affixID {
			return out[i].affixID < out[j].affixID
		}
		return out[i].rarityID < out[j].rarityID
	})
	return out
}

func decodeAffixSlotsForTemplateConsistency(affixRaw []byte) ([]EquipmentAffixSlot, bool) {
	if affixesJSONEmptyRaw(affixRaw) {
		return []EquipmentAffixSlot{}, true
	}
	var out []EquipmentAffixSlot
	if err := json.Unmarshal(affixRaw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func (s *Store) instanceAffixesMatchTemplateDefault(templateItemID string, affixRaw []byte) bool {
	actualSlots, ok := decodeAffixSlotsForTemplateConsistency(affixRaw)
	if !ok {
		return false
	}
	actual := normalizeAffixTemplateSignatures(actualSlots)
	expected := normalizeAffixTemplateSignatures(s.templateDefaultAffixSlots(templateItemID))
	return affixTemplateSignaturesEqual(actual, expected)
}

func (s *Store) instanceAffixesMatchFixedTemplateDefault(templateItemID string, affixRaw []byte) bool {
	return s.instanceFixedTemplateAffixMatchKind(templateItemID, affixRaw) == fixedTemplateAffixMatchExact
}

func (s *Store) instanceFixedTemplateAffixMatchKind(templateItemID string, affixRaw []byte) fixedTemplateAffixMatchKind {
	actualSlots, ok := decodeAffixSlotsForTemplateConsistency(affixRaw)
	if !ok {
		return fixedTemplateAffixMatchInvalid
	}
	actual := normalizeFixedAffixTemplateSignatures(actualSlots)
	expected := normalizeFixedAffixTemplateSignatures(s.templateDefaultAffixSlots(templateItemID))
	if len(actual) != len(expected) {
		return fixedTemplateAffixMatchMismatch
	}
	for idx := range actual {
		if actual[idx].slotIndex != expected[idx].slotIndex ||
			actual[idx].affixID != expected[idx].affixID ||
			actual[idx].rarityID != expected[idx].rarityID ||
			math.Abs(actual[idx].rollValue-expected[idx].rollValue) > 0.000000001 {
			return fixedTemplateAffixMatchMismatch
		}
	}
	return fixedTemplateAffixMatchExact
}

func (s *Store) itemHasRandomDefaultAffixGrant(itemID string) bool {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return false
	}
	for _, action := range s.gameData.Actions {
		reward, ok := action.ItemReward[itemID]
		if !ok {
			continue
		}
		if strings.TrimSpace(reward.GrantPolicy) == InventoryGrantPolicyRollDefaultAffixes {
			return true
		}
	}
	return false
}

func itemHasNonMarketType(itemType []string) bool {
	for _, entry := range itemType {
		if strings.TrimSpace(entry) == "currency" {
			return true
		}
	}
	return false
}

// TemplateItemMarketListable reports whether an item template can participate
// in the fixed-price template commodity market.
func (s *Store) TemplateItemMarketListable(itemID string) bool {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return false
	}
	item := s.gameData.ItemByID(itemID)
	if item == nil || !item.MarketListable || itemHasNonMarketType(item.Type) {
		return false
	}
	return !s.itemHasRandomDefaultAffixGrant(itemID)
}

func (s *Store) marketTemplateItemIDs() []string {
	out := make([]string, 0, len(s.gameData.Items))
	for _, item := range s.gameData.Items {
		if s.TemplateItemMarketListable(item.ID) {
			out = append(out, strings.TrimSpace(item.ID))
		}
	}
	sort.Strings(out)
	return out
}

func normalizeTemplateBaseAttrs(raw map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(raw))
	for key, value := range raw {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = value
	}
	return out
}

func baseSnapshotMatchesTemplateDefault(templateBase map[string]float64, raw []byte) bool {
	if baseSnapshotJSONEmptyRaw(raw) {
		return true
	}
	var actual map[string]float64
	if err := json.Unmarshal(raw, &actual); err != nil {
		return false
	}
	expected := normalizeTemplateBaseAttrs(templateBase)
	normalizedActual := normalizeTemplateBaseAttrs(actual)
	if len(normalizedActual) != len(expected) {
		return false
	}
	for key, expectedValue := range expected {
		actualValue, ok := normalizedActual[key]
		if !ok {
			return false
		}
		if math.Abs(actualValue-expectedValue) > 0.000000001 {
			return false
		}
	}
	return true
}

func (s *Store) lockedInstanceMatchesTemplateBase(row LockedItemInstance, rejectRandomDefaultAffixGrant bool) bool {
	if strings.TrimSpace(row.State) != "inventory" {
		return false
	}
	if strings.TrimSpace(row.TemplateItem) == "" || row.Quantity <= 0 {
		return false
	}
	if !itemCustomizationEmpty(row.CustomData) {
		return false
	}
	if row.EnhanceLevel != 0 {
		return false
	}
	if row.Durability <= 0 || row.MaxDurability <= 0 || row.Durability != row.MaxDurability {
		return false
	}
	if int64(row.MaxDurability) != s.defaultMaxDurability(row.TemplateItem) {
		return false
	}
	if rejectRandomDefaultAffixGrant && s.itemHasRandomDefaultAffixGrant(row.TemplateItem) {
		return false
	}
	template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(row.TemplateItem))
	templateBase := map[string]float64{}
	if template != nil {
		templateBase = template.BaseAttrs
	}
	if !baseSnapshotMatchesTemplateDefault(templateBase, row.BaseSnapshot) {
		return false
	}
	return true
}

func (s *Store) lockedInstanceMatchesTemplateCommodityBase(row LockedItemInstance) bool {
	return s.lockedInstanceMatchesTemplateBase(row, true)
}

func (s *Store) lockedInstanceMatchesTemplateRecycleBase(row LockedItemInstance) bool {
	return s.lockedInstanceMatchesTemplateBase(row, false)
}

func (s *Store) lockedInstanceIsTemplateCommodity(row LockedItemInstance) bool {
	return s.lockedInstanceMatchesTemplateCommodityBase(row) &&
		s.instanceAffixesMatchFixedTemplateDefault(row.TemplateItem, row.AffixesRaw)
}

func (s *Store) lockedInstanceRecycleTemplateMatchKind(row LockedItemInstance) recycleTemplateMatchKind {
	if !s.lockedInstanceMatchesTemplateRecycleBase(row) {
		return recycleTemplateMatchNone
	}
	actualSlots, ok := decodeAffixSlotsForTemplateConsistency(row.AffixesRaw)
	if !ok {
		return recycleTemplateMatchNone
	}
	actualAffixIDs := normalizeAffixTemplateSignatures(actualSlots)
	expectedAffixIDs := normalizeAffixTemplateSignatures(s.templateDefaultAffixSlots(row.TemplateItem))
	templateHasAffix := len(expectedAffixIDs) > 0
	instanceHasAnyAffix := len(actualAffixIDs) > 0
	if s.itemHasRandomDefaultAffixGrant(row.TemplateItem) {
		if templateHasAffix && affixTemplateSignaturesEqual(actualAffixIDs, expectedAffixIDs) {
			return recycleTemplateMatchFull
		}
		if templateHasAffix && !instanceHasAnyAffix {
			return recycleTemplateMatchAffixMismatchOnly
		}
		return recycleTemplateMatchNone
	}
	fixedMatch := s.instanceFixedTemplateAffixMatchKind(row.TemplateItem, row.AffixesRaw)
	switch fixedMatch {
	case fixedTemplateAffixMatchExact:
		return recycleTemplateMatchFull
	case fixedTemplateAffixMatchInvalid:
		return recycleTemplateMatchNone
	}
	if templateHasAffix && !instanceHasAnyAffix {
		return recycleTemplateMatchAffixMismatchOnly
	}
	return recycleTemplateMatchNone
}

func (s *Store) instanceMatchesTemplateForMarketRecycle(templateItemID string, enhanceLevel int, affixRaw []byte) bool {
	if enhanceLevel != 0 {
		return false
	}
	if s.itemHasRandomDefaultAffixGrant(templateItemID) {
		return false
	}
	return s.instanceAffixesMatchFixedTemplateDefault(templateItemID, affixRaw)
}
