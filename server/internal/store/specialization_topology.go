package store

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var ErrSpecializationEquipmentTopologyInvalid = errors.New("specialization equipment topology invalid")

type EquipmentSlotDefinition struct {
	Slot    string   `json:"slot"`
	Family  string   `json:"family"`
	Ordinal int      `json:"ordinal"`
	Enabled bool     `json:"enabled"`
	Accepts []string `json:"accepts"`
}

type EquipmentSlotProfile struct {
	Capacities map[string]int            `json:"slotCapacities"`
	Slots      []EquipmentSlotDefinition `json:"effectiveSlots"`
}

type equipmentSlotFamilyRule struct {
	Family       string
	BaseCapacity int
	MinCapacity  int
	MaxCapacity  int
	Accepts      []string
	SlotKey      func(int) string
}

var playerBattleSlotFamilyRules = []equipmentSlotFamilyRule{
	{Family: "cloak", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 1, Accepts: []string{"cloak"}, SlotKey: singletonSlotKey("cloak")},
	{Family: "head", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 2, Accepts: []string{"head"}, SlotKey: baseIndexedSlotKey("head")},
	{Family: "left_hand", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 1, Accepts: []string{"left_hand", "weapon"}, SlotKey: singletonSlotKey("left_hand")},
	{Family: "body", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 2, Accepts: []string{"body"}, SlotKey: baseIndexedSlotKey("body")},
	{Family: "right_hand", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 2, Accepts: []string{"right_hand", "weapon"}, SlotKey: rightHandSlotKey},
	{Family: "arm", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 2, Accepts: []string{"arm"}, SlotKey: baseIndexedSlotKey("arm")},
	{Family: "legs", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 1, Accepts: []string{"legs"}, SlotKey: singletonSlotKey("legs")},
	{Family: "feet", BaseCapacity: 1, MinCapacity: 0, MaxCapacity: 2, Accepts: []string{"feet"}, SlotKey: baseIndexedSlotKey("feet")},
	{Family: "accessory", BaseCapacity: 4, MinCapacity: 0, MaxCapacity: 8, Accepts: []string{"accessory"}, SlotKey: indexedSlotKey("accessory")},
}

func singletonSlotKey(slot string) func(int) string {
	return func(ordinal int) string {
		if ordinal != 1 {
			return ""
		}
		return slot
	}
}

func indexedSlotKey(family string) func(int) string {
	return func(ordinal int) string {
		if ordinal <= 0 {
			return ""
		}
		return family + "_" + strconv.Itoa(ordinal)
	}
}

func baseIndexedSlotKey(family string) func(int) string {
	return func(ordinal int) string {
		if ordinal == 1 {
			return family
		}
		if ordinal > 1 {
			return family + "_" + strconv.Itoa(ordinal)
		}
		return ""
	}
}

func rightHandSlotKey(ordinal int) string {
	if ordinal == 1 {
		return "right_hand"
	}
	if ordinal > 1 {
		return "right_hand_" + strconv.Itoa(ordinal)
	}
	return ""
}

func ResolvePlayerBattleSlotProfile(capacityDeltas map[string]int) (EquipmentSlotProfile, error) {
	profile := EquipmentSlotProfile{
		Capacities: make(map[string]int, len(playerBattleSlotFamilyRules)),
		Slots:      make([]EquipmentSlotDefinition, 0, 16),
	}
	knownFamilies := make(map[string]struct{}, len(playerBattleSlotFamilyRules))
	for _, rule := range playerBattleSlotFamilyRules {
		knownFamilies[rule.Family] = struct{}{}
		capacity := rule.BaseCapacity + capacityDeltas[rule.Family]
		if capacity < rule.MinCapacity || capacity > rule.MaxCapacity {
			return EquipmentSlotProfile{}, fmt.Errorf(
				"%w: family %s capacity %d outside [%d,%d]",
				ErrSpecializationEquipmentTopologyInvalid,
				rule.Family,
				capacity,
				rule.MinCapacity,
				rule.MaxCapacity,
			)
		}
		profile.Capacities[rule.Family] = capacity
		for ordinal := 1; ordinal <= rule.MaxCapacity; ordinal++ {
			slot := rule.SlotKey(ordinal)
			if slot == "" {
				continue
			}
			profile.Slots = append(profile.Slots, EquipmentSlotDefinition{
				Slot:    slot,
				Family:  rule.Family,
				Ordinal: ordinal,
				Enabled: ordinal <= capacity,
				Accepts: append([]string(nil), rule.Accepts...),
			})
		}
	}
	for rawFamily, delta := range capacityDeltas {
		family := strings.TrimSpace(rawFamily)
		if family == "" || delta == 0 {
			continue
		}
		if _, ok := knownFamilies[family]; !ok {
			return EquipmentSlotProfile{}, fmt.Errorf("%w: unknown family %s", ErrSpecializationEquipmentTopologyInvalid, family)
		}
	}
	return profile, nil
}

func BasePlayerBattleSlotProfile() EquipmentSlotProfile {
	profile, err := ResolvePlayerBattleSlotProfile(nil)
	if err != nil {
		panic(err)
	}
	return profile
}

func (p EquipmentSlotProfile) Enabled(slot string) bool {
	slot = strings.TrimSpace(slot)
	for _, row := range p.Slots {
		if row.Slot == slot {
			return row.Enabled
		}
	}
	return false
}

func (p EquipmentSlotProfile) Definition(slot string) (EquipmentSlotDefinition, bool) {
	slot = strings.TrimSpace(slot)
	for _, row := range p.Slots {
		if row.Slot == slot {
			return row, true
		}
	}
	return EquipmentSlotDefinition{}, false
}

func (p EquipmentSlotProfile) EnabledSlotKeys() []string {
	out := make([]string, 0, len(p.Slots))
	for _, row := range p.Slots {
		if row.Enabled {
			out = append(out, row.Slot)
		}
	}
	return out
}

func equipmentSlotCapacityDelta(before, after EquipmentSlotProfile) (disabled []string, enabled []string) {
	beforeEnabled := make(map[string]struct{}, len(before.Slots))
	afterEnabled := make(map[string]struct{}, len(after.Slots))
	for _, row := range before.Slots {
		if row.Enabled {
			beforeEnabled[row.Slot] = struct{}{}
		}
	}
	for _, row := range after.Slots {
		if row.Enabled {
			afterEnabled[row.Slot] = struct{}{}
		}
	}
	for slot := range beforeEnabled {
		if _, ok := afterEnabled[slot]; !ok {
			disabled = append(disabled, slot)
		}
	}
	for slot := range afterEnabled {
		if _, ok := beforeEnabled[slot]; !ok {
			enabled = append(enabled, slot)
		}
	}
	sort.Strings(disabled)
	sort.Strings(enabled)
	return disabled, enabled
}
