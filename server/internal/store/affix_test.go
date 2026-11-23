package store

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestListActiveEnchantRulesIncludesAllowedAffixIDsAndConsumeCount(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			AffixEnchantRules: []catalog.AffixEnchantRule{
				{
					ToolItemID:    "craftsmans_hammer",
					ConsumeCount:  3,
					Costs:         map[string]int64{"gold_coin": 1000, "enhance_stone": 2},
					FailChance:    0.2,
					RarityWeights: map[string]float64{"rare": 80, "epic": 20},
					AllowedAffix:  []string{" affix.a ", "", "affix.b"},
					Active:        true,
				},
				{
					ToolItemID:    "inactive_tool",
					ConsumeCount:  99,
					RarityWeights: map[string]float64{"legendary": 100},
					AllowedAffix:  []string{"affix.c"},
					Active:        false,
				},
			},
		},
	}

	rules := s.listActiveEnchantRules("")
	if len(rules) != 1 {
		t.Fatalf("len(rules)=%d, want 1", len(rules))
	}
	got := rules[0]
	if got.ToolItemID != "craftsmans_hammer" {
		t.Fatalf("ToolItemID=%q, want craftsmans_hammer", got.ToolItemID)
	}
	if got.ConsumeCount != 3 {
		t.Fatalf("ConsumeCount=%d, want 3", got.ConsumeCount)
	}
	if !reflect.DeepEqual(got.Costs, map[string]int64{"gold_coin": 1000, "enhance_stone": 2}) {
		t.Fatalf("Costs=%v, want map[enhance_stone:2 gold_coin:1000]", got.Costs)
	}
	if !reflect.DeepEqual(got.AllowedAffixIDs, []string{"affix.a", "affix.b"}) {
		t.Fatalf("AllowedAffixIDs=%v, want [affix.a affix.b]", got.AllowedAffixIDs)
	}
}

func TestRollEnchantAffixReturnsErrorWhenNoRarityCandidate(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			AffixDefinitions: []catalog.AffixDefinition{
				{
					ID:              "affix.common_only",
					AllowedRarities: []string{"common"},
					Active:          true,
				},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix:  []string{"affix.common_only"},
		RarityWeights: map[string]float64{"legendary": 100},
		Active:        true,
	}

	slot, err := s.rollEnchantAffix(rule, "wood_sword", 0, rand.New(rand.NewSource(1)))
	if !errors.Is(err, ErrAffixEnchantNoRarityCandidate) {
		t.Fatalf("rollEnchantAffix error=%v, want ErrAffixEnchantNoRarityCandidate", err)
	}
	if slot.AffixID != "" {
		t.Fatalf("AffixID=%q, want empty", slot.AffixID)
	}
}

func TestRollEnchantAffixIgnoresUnavailableRarityWeights(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			AffixDefinitions: []catalog.AffixDefinition{
				{
					ID:              "affix.rare_only",
					AllowedRarities: []string{"rare"},
					Active:          true,
				},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix:  []string{"affix.rare_only"},
		RarityWeights: map[string]float64{"common": 100, "rare": 1},
		Active:        true,
	}

	slot, err := s.rollEnchantAffix(rule, "wood_sword", 1, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatalf("rollEnchantAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.rare_only" {
		t.Fatalf("AffixID=%q, want affix.rare_only", slot.AffixID)
	}
	if slot.RarityID != "rare" {
		t.Fatalf("RarityID=%q, want rare", slot.RarityID)
	}
}

func TestRollEnchantAffixUsesAllActiveAffixesWhenAllowedAffixEmpty(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			AffixDefinitions: []catalog.AffixDefinition{
				{
					ID:              "affix.alpha",
					AllowedRarities: []string{"rare"},
					Active:          true,
				},
				{
					ID:              "affix.beta",
					AllowedRarities: []string{"rare"},
					Active:          true,
				},
				{
					ID:              "affix.inactive",
					AllowedRarities: []string{"rare"},
					Active:          false,
				},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix:  []string{},
		RarityWeights: map[string]float64{"rare": 100},
		Active:        true,
	}

	slot, err := s.rollEnchantAffix(rule, "wood_sword", 1, rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatalf("rollEnchantAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.alpha" && slot.AffixID != "affix.beta" {
		t.Fatalf("AffixID=%q, want one of active affixes", slot.AffixID)
	}
	if slot.RarityID != "rare" {
		t.Fatalf("RarityID=%q, want rare", slot.RarityID)
	}
}

func TestResolveEnchantAllowedAffixIDsPrefersSlotScopeThenEquipTypeFallback(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "battle_sword", EquipType: "battle", Slot: "right_hand"},
				{ID: "battle_head", EquipType: "battle", Slot: "head"},
				{ID: "work_tool", EquipType: "work", Slot: "right_hand"},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix: []string{"default_affix"},
		AllowedAffixScopes: []catalog.AffixEnchantScopeDef{
			{EquipType: "battle", Slots: []string{}, AllowedAffix: []string{"battle_default_affix"}},
			{EquipType: "battle", Slots: []string{"right_hand"}, AllowedAffix: []string{"battle_right_hand_affix"}},
			{EquipType: "work", Slots: []string{}, AllowedAffix: []string{"work_default_affix"}},
		},
	}

	if got := s.resolveEnchantAllowedAffixIDs(rule, "battle_sword"); !reflect.DeepEqual(got, []string{"battle_right_hand_affix"}) {
		t.Fatalf("battle_sword allowed=%v, want [battle_right_hand_affix]", got)
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "battle_head"); !reflect.DeepEqual(got, []string{"battle_default_affix"}) {
		t.Fatalf("battle_head allowed=%v, want [battle_default_affix]", got)
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "work_tool"); !reflect.DeepEqual(got, []string{"work_default_affix"}) {
		t.Fatalf("work_tool allowed=%v, want [work_default_affix]", got)
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "unknown_template"); !reflect.DeepEqual(got, []string{"default_affix"}) {
		t.Fatalf("unknown_template allowed=%v, want [default_affix]", got)
	}
}

func TestResolveEnchantAllowedAffixIDsSupportsWeaponClassPriorityAndUnion(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "battle_dagger", EquipType: "battle", Slot: "right_hand", WeaponClass: []string{"dagger"}},
				{ID: "battle_mixed", EquipType: "battle", Slot: "right_hand", WeaponClass: []string{"shield", "dagger"}},
				{ID: "battle_plain", EquipType: "battle", Slot: "right_hand"},
				{ID: "battle_head_shield", EquipType: "battle", Slot: "head", WeaponClass: []string{"shield"}},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix: []string{"default_affix"},
		AllowedAffixScopes: []catalog.AffixEnchantScopeDef{
			{EquipType: "battle", Slots: []string{}, AllowedAffix: []string{"battle_default_affix"}},
			{EquipType: "battle", Slots: []string{}, WeaponClasses: []string{"shield"}, AllowedAffix: []string{"battle_shield_affix"}},
			{EquipType: "battle", Slots: []string{"right_hand"}, AllowedAffix: []string{"battle_right_hand_affix"}},
			{EquipType: "battle", Slots: []string{"right_hand"}, WeaponClasses: []string{"dagger"}, AllowedAffix: []string{"dagger_right_hand_affix"}},
			{EquipType: "battle", Slots: []string{"right_hand"}, WeaponClasses: []string{"shield"}, AllowedAffix: []string{"shield_right_hand_affix"}},
		},
	}

	if got := s.resolveEnchantAllowedAffixIDs(rule, "battle_dagger"); !reflect.DeepEqual(got, []string{"dagger_right_hand_affix"}) {
		t.Fatalf("battle_dagger allowed=%v, want [dagger_right_hand_affix]", got)
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "battle_mixed"); !reflect.DeepEqual(got, []string{"dagger_right_hand_affix", "shield_right_hand_affix"}) {
		t.Fatalf("battle_mixed allowed=%v, want [dagger_right_hand_affix shield_right_hand_affix]", got)
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "battle_plain"); !reflect.DeepEqual(got, []string{"battle_right_hand_affix"}) {
		t.Fatalf("battle_plain allowed=%v, want [battle_right_hand_affix]", got)
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "battle_head_shield"); !reflect.DeepEqual(got, []string{"battle_shield_affix"}) {
		t.Fatalf("battle_head_shield allowed=%v, want [battle_shield_affix]", got)
	}
}

func TestResolveEnchantAllowedAffixIDsReturnsEmptyWhenScopeNotMatched(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "work_tool", EquipType: "work", Slot: "right_hand"},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix: []string{"default_affix"},
		AllowedAffixScopes: []catalog.AffixEnchantScopeDef{
			{EquipType: "battle", Slots: []string{"right_hand"}, AllowedAffix: []string{"battle_affix"}},
		},
	}
	if got := s.resolveEnchantAllowedAffixIDs(rule, "work_tool"); len(got) != 0 {
		t.Fatalf("work_tool allowed=%v, want []", got)
	}
}

func TestListActiveEnchantRulesUsesTemplateScopedAllowedAffixIDs(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "battle_sword", EquipType: "battle", Slot: "right_hand"},
			},
			AffixEnchantRules: []catalog.AffixEnchantRule{
				{
					ToolItemID:   "craftsmans_hammer",
					ConsumeCount: 2,
					FailChance:   0.1,
					RarityWeights: map[string]float64{
						"rare": 100,
					},
					AllowedAffix: []string{"default_affix"},
					AllowedAffixScopes: []catalog.AffixEnchantScopeDef{
						{EquipType: "battle", Slots: []string{}, AllowedAffix: []string{" battle_default ", "", "battle_default_2"}},
						{EquipType: "battle", Slots: []string{"right_hand"}, AllowedAffix: []string{" battle_right_hand ", "", "battle_right_hand_2"}},
					},
					Active: true,
				},
			},
		},
	}

	rules := s.listActiveEnchantRules("battle_sword")
	if len(rules) != 1 {
		t.Fatalf("len(rules)=%d, want 1", len(rules))
	}
	if !reflect.DeepEqual(rules[0].AllowedAffixIDs, []string{"battle_right_hand", "battle_right_hand_2"}) {
		t.Fatalf("AllowedAffixIDs=%v, want [battle_right_hand battle_right_hand_2]", rules[0].AllowedAffixIDs)
	}
}

func TestListActiveEnchantRulesSkipsRuleWhenNoScopeCandidate(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "work_tool", EquipType: "work", Slot: "right_hand"},
			},
			AffixEnchantRules: []catalog.AffixEnchantRule{
				{
					ToolItemID:   "battle_only_tool",
					ConsumeCount: 1,
					RarityWeights: map[string]float64{
						"legendary": 100,
					},
					AllowedAffixScopes: []catalog.AffixEnchantScopeDef{
						{EquipType: "battle", Slots: []string{"right_hand"}, AllowedAffix: []string{"affix.battle_only"}},
					},
					Active: true,
				},
			},
		},
	}
	rules := s.listActiveEnchantRules("work_tool")
	if len(rules) != 0 {
		t.Fatalf("len(rules)=%d, want 0", len(rules))
	}
}

func TestRollEnchantAffixHonorsAllowedAffixAndRarityCompatibility(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			AffixDefinitions: []catalog.AffixDefinition{
				{
					ID:              "affix.common_only",
					AllowedRarities: []string{"common"},
					Active:          true,
				},
				{
					ID:              "affix.rare_only",
					AllowedRarities: []string{"rare"},
					Active:          true,
				},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix:  []string{"affix.rare_only"},
		RarityWeights: map[string]float64{"common": 100, "rare": 1},
		Active:        true,
	}

	slot, err := s.rollEnchantAffix(rule, "wood_sword", 2, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatalf("rollEnchantAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.rare_only" {
		t.Fatalf("AffixID=%q, want affix.rare_only", slot.AffixID)
	}
	if slot.RarityID != "rare" {
		t.Fatalf("RarityID=%q, want rare", slot.RarityID)
	}
}

func TestCarrierRuleAllowsRarity(t *testing.T) {
	allRarityRule := &catalog.AffixCarrierRule{
		CarrierItemID:    "engrave_stone",
		AllowedRarityIDs: []string{},
		Active:           true,
	}
	if !carrierRuleAllowsRarity(allRarityRule, "legendary") {
		t.Fatalf("expected empty rarity list to allow all rarities")
	}

	targetRule := &catalog.AffixCarrierRule{
		CarrierItemID:    "engrave_stone",
		AllowedRarityIDs: []string{"rare", "epic"},
		Active:           true,
	}
	if !carrierRuleAllowsRarity(targetRule, "epic") {
		t.Fatalf("expected epic to be allowed")
	}
	if carrierRuleAllowsRarity(targetRule, "legendary") {
		t.Fatalf("expected legendary to be disallowed")
	}
}

func TestListActiveCarrierRules(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			AffixCarrierRules: []catalog.AffixCarrierRule{
				{
					CarrierItemID:    "engrave_stone",
					AllowedRarityIDs: []string{" rare ", "", "epic"},
					Active:           true,
				},
				{
					CarrierItemID:    "inactive_carrier",
					AllowedRarityIDs: []string{"legendary"},
					Active:           false,
				},
			},
		},
	}

	rules := s.listActiveCarrierRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules)=%d, want 1", len(rules))
	}
	if rules[0].CarrierItemID != "engrave_stone" {
		t.Fatalf("CarrierItemID=%q, want engrave_stone", rules[0].CarrierItemID)
	}
	if !reflect.DeepEqual(rules[0].AllowedRarityIDs, []string{"rare", "epic"}) {
		t.Fatalf("AllowedRarityIDs=%v, want [rare epic]", rules[0].AllowedRarityIDs)
	}
}

func TestRollEnchantAffixRespectsEquipmentAffixAllowedIDs(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "wood_sword", AffixAllowedIDs: []string{"affix.dexterity_minor"}},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.strength_minor", AllowedRarities: []string{"rare"}, Active: true},
				{ID: "affix.dexterity_minor", AllowedRarities: []string{"rare"}, Active: true},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix:  []string{},
		RarityWeights: map[string]float64{"rare": 100},
		Active:        true,
	}

	slot, err := s.rollEnchantAffix(rule, "wood_sword", 0, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatalf("rollEnchantAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.dexterity_minor" {
		t.Fatalf("AffixID=%q, want affix.dexterity_minor", slot.AffixID)
	}
}

func TestEquipmentAllowsAffixID(t *testing.T) {
	if equipmentAllowsAffixID(nil, "affix.alpha") {
		t.Fatalf("nil template should not allow affix")
	}

	templateAllowAll := &catalog.EquipmentTemplate{ID: "wood_sword", AffixAllowedIDs: []string{}}
	if !equipmentAllowsAffixID(templateAllowAll, "affix.alpha") {
		t.Fatalf("empty affix allowed list should allow all")
	}

	templateRestricted := &catalog.EquipmentTemplate{
		ID:              "wood_sword",
		AffixAllowedIDs: []string{" affix.beta ", "", "affix.gamma"},
	}
	if equipmentAllowsAffixID(templateRestricted, "affix.alpha") {
		t.Fatalf("affix.alpha should be disallowed")
	}
	if !equipmentAllowsAffixID(templateRestricted, "affix.beta") {
		t.Fatalf("affix.beta should be allowed")
	}
}

func TestEquipmentAllowsAffixIDCrystalBracersAllowsExtractedTaunt(t *testing.T) {
	template := &catalog.EquipmentTemplate{
		ID:              "crystal_bracers",
		AffixAllowedIDs: []string{},
	}
	if !equipmentAllowsAffixID(template, "battle_taunt_start") {
		t.Fatalf("crystal_bracers should allow extracted battle_taunt_start when no affix whitelist is configured")
	}
}

func TestEquipmentAllowsAffixRequiresMatchingEquipType(t *testing.T) {
	templateWork := &catalog.EquipmentTemplate{
		ID:        "work_pickaxe",
		EquipType: "work",
	}
	affixWork := &catalog.AffixDefinition{ID: "affix.mining_speed", Type: "work", Active: true}
	affixBattle := &catalog.AffixDefinition{ID: "affix.crit_rate", Type: "battle", Active: true}
	if !equipmentAllowsAffix(templateWork, affixWork, affixWork.ID) {
		t.Fatalf("work affix should be allowed for work equipment")
	}
	if equipmentAllowsAffix(templateWork, affixBattle, affixBattle.ID) {
		t.Fatalf("battle affix should be disallowed for work equipment")
	}
}

func TestRollEnchantAffixRespectsAffixTypeByEquipment(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "work_pickaxe", EquipType: "work"},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.work_only", Type: "work", AllowedRarities: []string{"rare"}, Active: true},
				{ID: "affix.battle_only", Type: "battle", AllowedRarities: []string{"rare"}, Active: true},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix:  []string{},
		RarityWeights: map[string]float64{"rare": 100},
		Active:        true,
	}
	slot, err := s.rollEnchantAffix(rule, "work_pickaxe", 0, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatalf("rollEnchantAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.work_only" {
		t.Fatalf("AffixID=%q, want affix.work_only", slot.AffixID)
	}
}

func TestRollEnchantAffixReturnsInvalidCarrierWhenScopeNotMatched(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "work_pickaxe", EquipType: "work", Slot: "right_hand"},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.work_only", Type: "work", AllowedRarities: []string{"legendary"}, Active: true},
			},
		},
	}
	rule := &catalog.AffixEnchantRule{
		AllowedAffix: []string{},
		AllowedAffixScopes: []catalog.AffixEnchantScopeDef{
			{EquipType: "battle", Slots: []string{"right_hand"}, AllowedAffix: []string{"affix.battle_only"}},
		},
		RarityWeights: map[string]float64{"legendary": 100},
		Active:        true,
	}
	_, err := s.rollEnchantAffix(rule, "work_pickaxe", 0, rand.New(rand.NewSource(1)))
	if !errors.Is(err, ErrAffixInvalidCarrier) {
		t.Fatalf("error=%v, want ErrAffixInvalidCarrier", err)
	}
}

func TestCollectOccupiedAffixSlotIndexesIgnoresInvalidSlots(t *testing.T) {
	occupied := collectOccupiedAffixSlotIndexes([]EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "affix.alpha"},
		{SlotIndex: 0, AffixID: "affix.beta"},
		{SlotIndex: 1, AffixID: ""},
		{SlotIndex: -1, AffixID: "affix.invalid"},
		{SlotIndex: 9, AffixID: "affix.invalid"},
	}, 3)
	if len(occupied) != 1 {
		t.Fatalf("len(occupied)=%d, want 1", len(occupied))
	}
	if _, ok := occupied[0]; !ok {
		t.Fatalf("slot 0 should be occupied")
	}
}

func TestWouldExceedEquipmentAffixLimit(t *testing.T) {
	def := &catalog.AffixDefinition{ID: "affix.alpha", MaxEffectivePerEquipment: 2}
	slots := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "affix.alpha"},
		{SlotIndex: 1, AffixID: "affix.alpha"},
	}
	if !WouldExceedEquipmentAffixLimit(slots, "affix.alpha", 1, def) {
		t.Fatalf("expected adding one more affix.alpha to exceed equipment limit")
	}
	if WouldExceedEquipmentAffixLimit(slots, "affix.beta", 1, def) {
		t.Fatalf("different affix id should not use alpha limit")
	}
}

func TestFilterEffectiveAffixSlotsByLimit_UsesSlotOrderAndBodyCap(t *testing.T) {
	gameData := catalog.Data{
		AffixDefinitions: []catalog.AffixDefinition{
			{ID: "affix.alpha", MaxEffectivePerEquipment: 2, MaxEffectivePerBody: 3, Active: true},
			{ID: "affix.beta", MaxEffectivePerEquipment: 0, MaxEffectivePerBody: 1, Active: true},
		},
	}
	bodySeen := map[string]int{}
	firstEquipment := []EquipmentAffixSlot{
		{SlotIndex: 2, AffixID: "affix.alpha"},
		{SlotIndex: 0, AffixID: "affix.alpha"},
		{SlotIndex: 1, AffixID: "affix.alpha"},
		{SlotIndex: 3, AffixID: "affix.beta"},
	}
	gotFirst := FilterEffectiveAffixSlotsByLimit(firstEquipment, gameData, bodySeen)
	if len(gotFirst) != 3 {
		t.Fatalf("first equipment effective len=%d, want 3", len(gotFirst))
	}
	if gotFirst[0].SlotIndex != 0 || gotFirst[1].SlotIndex != 1 {
		t.Fatalf("first equipment should keep slotIndex asc order, got=%v,%v", gotFirst[0].SlotIndex, gotFirst[1].SlotIndex)
	}
	if bodySeen["affix.alpha"] != 2 || bodySeen["affix.beta"] != 1 {
		t.Fatalf("body seen unexpected: %#v", bodySeen)
	}

	secondEquipment := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "affix.alpha"},
		{SlotIndex: 1, AffixID: "affix.alpha"},
		{SlotIndex: 2, AffixID: "affix.beta"},
	}
	gotSecond := FilterEffectiveAffixSlotsByLimit(secondEquipment, gameData, bodySeen)
	if len(gotSecond) != 1 {
		t.Fatalf("second equipment effective len=%d, want 1", len(gotSecond))
	}
	if gotSecond[0].AffixID != "affix.alpha" {
		t.Fatalf("second equipment should only keep one alpha, got %#v", gotSecond[0])
	}
}

func TestWouldExceedCustomAlchemyEquipmentLimit_UsesMaterialComponent(t *testing.T) {
	slot := EquipmentAffixSlot{
		SlotIndex: 0,
		AffixID:   "custom.one",
		EffectParams: map[string]any{
			"alchemyComponents": []any{
				map[string]any{"itemId": "holy_cat_royal_jewel", "traitId": "holy_cat_royal_jewel", "maxEffectivePerEquipment": 1.0, "maxEffectivePerBody": 2.0},
				map[string]any{"itemId": "holy_cat_royal_jewel", "traitId": "holy_cat_royal_jewel", "attr": "luck", "maxEffectivePerEquipment": 1.0, "maxEffectivePerBody": 2.0},
			},
		},
	}
	candidate := EquipmentAffixSlot{
		SlotIndex: 1,
		AffixID:   "custom.two",
		EffectParams: map[string]any{
			"alchemyComponents": []any{
				map[string]any{"itemId": "holy_cat_royal_jewel", "traitId": "holy_cat_royal_jewel", "maxEffectivePerEquipment": 1.0, "maxEffectivePerBody": 2.0},
			},
		},
	}
	componentID, limit, exceeded := WouldExceedCustomAlchemyEquipmentLimit([]EquipmentAffixSlot{slot}, candidate)
	if !exceeded {
		t.Fatalf("expected repeated material component to exceed equipment limit")
	}
	if componentID != "holy_cat_royal_jewel" || limit != 1 {
		t.Fatalf("unexpected exceeded component: id=%q limit=%d", componentID, limit)
	}
}

func TestWouldExceedCustomAlchemyEquipmentLimit_UsesLatestTraitLimit(t *testing.T) {
	gameData := catalog.Data{
		ItemTraits: []catalog.ItemTraitDefinition{{
			ItemID: "moon_dew",
			Traits: []catalog.AlchemyTraitEntry{{
				TraitID:                  "moon_dew",
				MaxEffectivePerEquipment: 1,
				MaxEffectivePerBody:      2,
			}},
		}},
	}
	slot := EquipmentAffixSlot{
		SlotIndex: 0,
		AffixID:   "custom.one",
		EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"itemId": "moon_dew", "traitId": "moon_dew", "maxEffectivePerEquipment": 2.0, "maxEffectivePerBody": 8.0},
		}},
	}
	candidate := EquipmentAffixSlot{
		SlotIndex: 1,
		AffixID:   "custom.two",
		EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"itemId": "moon_dew", "traitId": "moon_dew", "maxEffectivePerEquipment": 2.0, "maxEffectivePerBody": 8.0},
		}},
	}
	componentID, limit, exceeded := WouldExceedCustomAlchemyEquipmentLimitWithLookup([]EquipmentAffixSlot{slot}, candidate, gameData)
	if !exceeded {
		t.Fatalf("expected latest catalog equipment limit to override stale component limit")
	}
	if componentID != "moon_dew" || limit != 1 {
		t.Fatalf("unexpected exceeded component: id=%q limit=%d", componentID, limit)
	}
}

func TestWouldExceedCustomAlchemyEquipmentLimit_UsesOrdinaryAffixTemplateLimit(t *testing.T) {
	gameData := catalog.Data{
		AffixDefinitions: []catalog.AffixDefinition{
			{ID: "affix.alpha", MaxEffectivePerEquipment: 1, MaxEffectivePerBody: 2, Active: true},
		},
	}
	slot := EquipmentAffixSlot{SlotIndex: 0, AffixID: "affix.alpha"}
	candidate := EquipmentAffixSlot{
		SlotIndex: 1,
		AffixID:   "custom.inherited",
		EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"sourceKind": "ordinary_affix", "affixId": "affix.alpha", "attr": "atk"},
		}},
	}
	componentID, limit, exceeded := WouldExceedCustomAlchemyEquipmentLimitWithLookup([]EquipmentAffixSlot{slot}, candidate, gameData)
	if !exceeded {
		t.Fatalf("expected inherited ordinary affix source to share the original affix equipment limit")
	}
	if componentID != "affix.alpha" || limit != 1 {
		t.Fatalf("unexpected exceeded component: id=%q limit=%d", componentID, limit)
	}
}

func TestWouldExceedCustomAlchemyEquipmentLimit_UsesAllOrdinaryAffixAncestryLimits(t *testing.T) {
	gameData := catalog.Data{
		AffixDefinitions: []catalog.AffixDefinition{
			{ID: "affix.alpha", MaxEffectivePerEquipment: 2, MaxEffectivePerBody: 2, Active: true},
			{ID: "affix.beta", MaxEffectivePerEquipment: 1, MaxEffectivePerBody: 2, Active: true},
		},
	}
	slot := EquipmentAffixSlot{SlotIndex: 0, AffixID: "affix.beta"}
	candidate := EquipmentAffixSlot{
		SlotIndex: 1,
		AffixID:   "custom.inherited",
		EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{
				"sourceKind": "ordinary_affix",
				"affixId":    "affix.alpha",
				"attr":       "atk",
				"ancestry": []any{
					map[string]any{"affixId": "affix.alpha", "sourceType": "affix"},
					map[string]any{"affixId": "affix.beta", "sourceType": "affix"},
				},
			},
		}},
	}
	componentID, limit, exceeded := WouldExceedCustomAlchemyEquipmentLimitWithLookup([]EquipmentAffixSlot{slot}, candidate, gameData)
	if !exceeded {
		t.Fatalf("expected inherited ordinary affix to honor every ordinary affix template in ancestry")
	}
	if componentID != "affix.beta" || limit != 1 {
		t.Fatalf("unexpected exceeded component: id=%q limit=%d", componentID, limit)
	}
}

func TestFilterEffectiveAffixSlotsByLimit_UsesOrdinaryAffixTemplateLimitForInheritedSources(t *testing.T) {
	gameData := catalog.Data{
		AffixDefinitions: []catalog.AffixDefinition{
			{ID: "affix.alpha", MaxEffectivePerEquipment: 2, MaxEffectivePerBody: 2, Active: true},
			{ID: "affix.beta", MaxEffectivePerEquipment: 1, MaxEffectivePerBody: 2, Active: true},
		},
	}
	slots := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "affix.beta"},
		{
			SlotIndex: 1,
			AffixID:   "custom.inherited",
			EffectParams: map[string]any{"alchemyComponents": []any{
				map[string]any{
					"sourceKind": "ordinary_affix",
					"affixId":    "affix.alpha",
					"attr":       "atk",
					"ancestry": []any{
						map[string]any{"affixId": "affix.alpha", "sourceType": "affix"},
						map[string]any{"affixId": "affix.beta", "sourceType": "affix"},
					},
				},
			}},
		},
	}
	got := FilterEffectiveAffixSlotsByLimit(slots, gameData, map[string]int{})
	if len(got) != 1 {
		t.Fatalf("effective len=%d, want 1", len(got))
	}
	if got[0].AffixID != "affix.beta" {
		t.Fatalf("expected inherited custom affix to be filtered by beta template limit, got %#v", got)
	}
}

func TestFilterEffectiveAffixSlotsByLimit_UsesCustomAlchemyBodyCap(t *testing.T) {
	gameData := catalog.Data{}
	bodySeen := map[string]int{}
	firstEquipment := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "custom.one", EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"itemId": "holy_cat_royal_jewel", "traitId": "holy_cat_royal_jewel", "maxEffectivePerEquipment": 2.0, "maxEffectivePerBody": 1.0},
		}}},
	}
	secondEquipment := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "custom.two", EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"itemId": "holy_cat_royal_jewel", "traitId": "holy_cat_royal_jewel", "maxEffectivePerEquipment": 2.0, "maxEffectivePerBody": 1.0},
		}}},
		{SlotIndex: 1, AffixID: "custom.three", EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"sourceKind": "combo_output", "comboRecipeId": "alchemy.combo.test", "attr": "atk", "maxEffectivePerEquipment": 1.0, "maxEffectivePerBody": 1.0},
		}}},
	}
	gotFirst := FilterEffectiveAffixSlotsByLimit(firstEquipment, gameData, bodySeen)
	if len(gotFirst) != 1 {
		t.Fatalf("first equipment effective len=%d, want 1", len(gotFirst))
	}
	gotSecond := FilterEffectiveAffixSlotsByLimit(secondEquipment, gameData, bodySeen)
	if len(gotSecond) != 1 {
		t.Fatalf("second equipment effective len=%d, want 1", len(gotSecond))
	}
	if gotSecond[0].AffixID != "custom.three" {
		t.Fatalf("combo output without material id should not be capped as a material, got %#v", gotSecond[0])
	}
}

func TestFilterEffectiveAffixSlotsByLimit_UsesLatestTraitBodyLimit(t *testing.T) {
	gameData := catalog.Data{
		ItemTraits: []catalog.ItemTraitDefinition{{
			ItemID: "moon_dew",
			Traits: []catalog.AlchemyTraitEntry{{
				TraitID:                  "moon_dew",
				MaxEffectivePerEquipment: 2,
				MaxEffectivePerBody:      1,
			}},
		}},
	}
	bodySeen := map[string]int{}
	firstEquipment := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "custom.one", EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"itemId": "moon_dew", "traitId": "moon_dew", "maxEffectivePerEquipment": 2.0, "maxEffectivePerBody": 8.0},
		}}},
	}
	secondEquipment := []EquipmentAffixSlot{
		{SlotIndex: 0, AffixID: "custom.two", EffectParams: map[string]any{"alchemyComponents": []any{
			map[string]any{"itemId": "moon_dew", "traitId": "moon_dew", "maxEffectivePerEquipment": 2.0, "maxEffectivePerBody": 8.0},
		}}},
	}
	gotFirst := FilterEffectiveAffixSlotsByLimit(firstEquipment, gameData, bodySeen)
	if len(gotFirst) != 1 {
		t.Fatalf("first equipment effective len=%d, want 1", len(gotFirst))
	}
	gotSecond := FilterEffectiveAffixSlotsByLimit(secondEquipment, gameData, bodySeen)
	if len(gotSecond) != 0 {
		t.Fatalf("second equipment effective len=%d, want 0", len(gotSecond))
	}
}

func TestRollEnhanceMilestoneAffixUsesConfiguredPool(t *testing.T) {
	rollValue := 0.42
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "wood_sword", EquipType: "battle", Slot: "right_hand"},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.fixed", Type: "battle", AllowedRarities: []string{"rare"}, Active: true},
				{ID: "affix.random", Type: "battle", AllowedRarities: []string{"rare"}, Active: true},
			},
		},
	}

	slot, err := s.rollEnhanceMilestoneAffix([]catalog.EnhanceAffixMilestonePoolEntry{
		{AffixID: "affix.fixed", RarityID: "rare", RollValue: &rollValue},
	}, "wood_sword", 1, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatalf("rollEnhanceMilestoneAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.fixed" || slot.RarityID != "rare" || slot.SlotIndex != 1 {
		t.Fatalf("unexpected milestone slot: %#v", slot)
	}
	if slot.RollValue != rollValue {
		t.Fatalf("RollValue=%v, want %v", slot.RollValue, rollValue)
	}

	slot, err = s.rollEnhanceMilestoneAffix([]catalog.EnhanceAffixMilestonePoolEntry{
		{AffixID: "affix.fixed", RarityID: "rare"},
		{AffixID: "affix.random", RarityID: "rare"},
	}, "wood_sword", 2, rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatalf("rollEnhanceMilestoneAffix returned error: %v", err)
	}
	if slot.AffixID != "affix.fixed" && slot.AffixID != "affix.random" {
		t.Fatalf("unexpected chosen affix: %#v", slot)
	}
	if slot.SlotIndex != 2 {
		t.Fatalf("SlotIndex=%d, want 2", slot.SlotIndex)
	}
}
