package store

import (
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestAffixesJSONEmptyRaw(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want bool
	}{
		{name: "nil", raw: nil, want: true},
		{name: "empty array", raw: []byte("[]"), want: true},
		{name: "empty object", raw: []byte("{}"), want: true},
		{name: "non-empty array", raw: []byte(`[ {"affixId":"affix.alpha"} ]`), want: false},
		{name: "non-empty object", raw: []byte(`{"affixId":"affix.alpha"}`), want: false},
		{name: "invalid json", raw: []byte("not-json"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := affixesJSONEmptyRaw(tc.raw)
			if got != tc.want {
				t.Fatalf("affixesJSONEmptyRaw(%q) = %v, want %v", string(tc.raw), got, tc.want)
			}
		})
	}
}

func TestHasInstanceSpecialState(t *testing.T) {
	tests := []struct {
		name         string
		enhanceLevel int
		affixRaw     []byte
		want         bool
	}{
		{name: "enhance level only", enhanceLevel: 1, affixRaw: []byte("[]"), want: true},
		{name: "empty array", enhanceLevel: 0, affixRaw: []byte("[]"), want: false},
		{name: "empty object", enhanceLevel: 0, affixRaw: []byte("{}"), want: false},
		{name: "non-empty array", enhanceLevel: 0, affixRaw: []byte(`[ {"affixId":"affix.alpha"} ]`), want: true},
		{name: "non-empty object", enhanceLevel: 0, affixRaw: []byte(`{"k":1}`), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasInstanceSpecialState(tc.enhanceLevel, tc.affixRaw)
			if got != tc.want {
				t.Fatalf("hasInstanceSpecialState(%d, %q) = %v, want %v", tc.enhanceLevel, string(tc.affixRaw), got, tc.want)
			}
		})
	}
}

func TestMilestoneAffixClaimsCodec(t *testing.T) {
	claims := map[string]bool{"50": true, "10": false}
	raw := encodeMilestoneAffixClaims(claims)
	decoded := decodeMilestoneAffixClaims(raw)
	if !decoded["50"] {
		t.Fatalf("expected level 50 to be claimed")
	}
	if decoded["10"] {
		t.Fatalf("expected level 10 to be unclaimed")
	}
	if got := decodeMilestoneAffixClaims([]byte(`{"5":1,"6":0}`)); !got["5"] || got["6"] {
		t.Fatalf("numeric claims should be decoded as bool flags, got=%#v", got)
	}
}

func TestInstanceAffixesMatchTemplateDefault(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{
					ID:             "shield",
					AffixSlotCount: 2,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.taunt", RarityID: "common"},
						{SlotIndex: 1, AffixID: "affix.block", RarityID: "common"},
					},
				},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.taunt", AllowedRarities: []string{"common"}, Active: true},
				{ID: "affix.block", AllowedRarities: []string{"common"}, Active: true},
			},
		},
	}

	tests := []struct {
		name     string
		itemID   string
		affixRaw []byte
		want     bool
	}{
		{
			name:     "matches template defaults",
			itemID:   "shield",
			affixRaw: []byte(`[{"slotIndex":0,"affixId":"affix.taunt"},{"slotIndex":1,"affixId":"affix.block","rarityId":"epic"}]`),
			want:     true,
		},
		{
			name:     "order does not matter",
			itemID:   "shield",
			affixRaw: []byte(`[{"slotIndex":1,"affixId":"affix.block"},{"slotIndex":0,"affixId":"affix.taunt"}]`),
			want:     true,
		},
		{
			name:     "missing one default affix",
			itemID:   "shield",
			affixRaw: []byte(`[{"slotIndex":0,"affixId":"affix.taunt"}]`),
			want:     false,
		},
		{
			name:     "different slot index",
			itemID:   "shield",
			affixRaw: []byte(`[{"slotIndex":0,"affixId":"affix.taunt"},{"slotIndex":0,"affixId":"affix.block"}]`),
			want:     false,
		},
		{
			name:     "invalid json is mismatch",
			itemID:   "shield",
			affixRaw: []byte(`not-json`),
			want:     false,
		},
		{
			name:     "no default affix template accepts empty object",
			itemID:   "plain_sword",
			affixRaw: []byte(`{}`),
			want:     true,
		},
		{
			name:     "no default affix template rejects non-empty",
			itemID:   "plain_sword",
			affixRaw: []byte(`[{"slotIndex":0,"affixId":"affix.taunt"}]`),
			want:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.instanceAffixesMatchTemplateDefault(tc.itemID, tc.affixRaw)
			if got != tc.want {
				t.Fatalf("instanceAffixesMatchTemplateDefault(%q, %q) = %v, want %v", tc.itemID, string(tc.affixRaw), got, tc.want)
			}
		})
	}
}

func TestInstanceMatchesTemplateForMarketRecycle(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{
					ID:             "shield",
					AffixSlotCount: 1,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.taunt", RarityID: "common"},
					},
				},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.taunt", AllowedRarities: []string{"common"}, Active: true},
			},
		},
	}
	if !s.instanceMatchesTemplateForMarketRecycle("shield", 0, []byte(`[{"slotIndex":0,"affixId":"affix.taunt","rarityId":"common"}]`)) {
		t.Fatalf("expected baseline instance to match market/recycle rule")
	}
	if s.instanceMatchesTemplateForMarketRecycle("shield", 1, []byte(`[{"slotIndex":0,"affixId":"affix.taunt","rarityId":"common"}]`)) {
		t.Fatalf("expected enhanced instance to fail market/recycle rule")
	}
}

func TestLockedInstanceIsTemplateCommodity(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{
					ID:             "shield",
					AffixSlotCount: 1,
					BaseAttrs:      map[string]float64{"armor": 3},
					MaxDurability:  240,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.taunt", RarityID: "common"},
					},
				},
				{
					ID:             "random_bracer",
					AffixSlotCount: 1,
					MaxDurability:  100,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{
						{SlotIndex: 0, AffixID: "affix.guard", RarityID: "rare", RollValue: 1},
					},
				},
				{
					ID:             "plain_cloth",
					AffixSlotCount: 1,
					MaxDurability:  100,
				},
				{
					ID:             "random_plain",
					AffixSlotCount: 1,
					MaxDurability:  100,
				},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.taunt", AllowedRarities: []string{"common"}, Active: true},
				{ID: "affix.guard", AllowedRarities: []string{"rare"}, Active: true},
			},
			Actions: []catalog.ActionDefinition{
				{
					ID: "craft_random_bracer",
					ItemReward: map[string]catalog.ActionItemReward{
						"random_bracer": {GrantPolicy: InventoryGrantPolicyRollDefaultAffixes},
						"random_plain":  {GrantPolicy: InventoryGrantPolicyRollDefaultAffixes},
					},
				},
			},
		},
	}
	base := LockedItemInstance{
		TemplateItem:  "shield",
		Quantity:      1,
		EnhanceLevel:  0,
		BaseSnapshot:  []byte(`{"armor":3}`),
		AffixesRaw:    []byte(`[{"slotIndex":0,"affixId":"affix.taunt","rarityId":"common"}]`),
		Durability:    240,
		MaxDurability: 240,
		State:         "inventory",
	}
	if !s.lockedInstanceIsTemplateCommodity(base) {
		t.Fatalf("expected fixed default affix instance to be commodity")
	}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(base); kind != recycleTemplateMatchFull {
		t.Fatalf("expected full recycle match kind, got %v", kind)
	}
	enhanced := base
	enhanced.EnhanceLevel = 1
	if s.lockedInstanceIsTemplateCommodity(enhanced) {
		t.Fatalf("expected enhanced instance to be custom")
	}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(enhanced); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for enhanced instance, got %v", kind)
	}
	damaged := base
	damaged.Durability = 239
	if s.lockedInstanceIsTemplateCommodity(damaged) {
		t.Fatalf("expected damaged instance to be custom")
	}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(damaged); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for damaged instance, got %v", kind)
	}
	affixChanged := base
	affixChanged.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.taunt","rarityId":"common","rollValue":2}]`)
	if s.lockedInstanceIsTemplateCommodity(affixChanged) {
		t.Fatalf("expected affix-mismatch instance to be non-commodity")
	}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(affixChanged); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for affix roll mismatch instance, got %v", kind)
	}
	affixRemoved := base
	affixRemoved.AffixesRaw = []byte(`[]`)
	if s.lockedInstanceIsTemplateCommodity(affixRemoved) {
		t.Fatalf("expected affix-removed instance to be non-commodity")
	}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(affixRemoved); kind != recycleTemplateMatchAffixMismatchOnly {
		t.Fatalf("expected affix-mismatch-only recycle match kind for template-affix but instance-no-affix, got %v", kind)
	}
	random := base
	random.TemplateItem = "random_bracer"
	random.BaseSnapshot = []byte(`{}`)
	random.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.guard","rarityId":"epic","rollValue":0.42}]`)
	random.Durability = 100
	random.MaxDurability = 100
	if s.lockedInstanceIsTemplateCommodity(random) {
		t.Fatalf("expected rollDefaultAffixes reward item to be custom")
	}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(random); kind != recycleTemplateMatchFull {
		t.Fatalf("expected full recycle match kind for random default-affix grant item with matching affix identity, got %v", kind)
	}
	randomAffixChanged := random
	randomAffixChanged.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.other","rarityId":"rare","rollValue":1}]`)
	if kind := s.lockedInstanceRecycleTemplateMatchKind(randomAffixChanged); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for random default-affix item with mismatched affix identity, got %v", kind)
	}
	randomEnhanced := random
	randomEnhanced.EnhanceLevel = 1
	if kind := s.lockedInstanceRecycleTemplateMatchKind(randomEnhanced); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for enhanced random default-affix item, got %v", kind)
	}
	randomDamaged := random
	randomDamaged.Durability = 99
	if kind := s.lockedInstanceRecycleTemplateMatchKind(randomDamaged); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for damaged random default-affix item, got %v", kind)
	}
	randomCustomized := random
	randomCustomized.CustomData = ItemCustomization{Name: "Kept"}
	if kind := s.lockedInstanceRecycleTemplateMatchKind(randomCustomized); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for customized random default-affix item, got %v", kind)
	}
	randomEquipped := random
	randomEquipped.State = "equipped"
	if kind := s.lockedInstanceRecycleTemplateMatchKind(randomEquipped); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for non-inventory random default-affix item, got %v", kind)
	}
	randomPlain := random
	randomPlain.TemplateItem = "random_plain"
	randomPlain.AffixesRaw = []byte(`[]`)
	if kind := s.lockedInstanceRecycleTemplateMatchKind(randomPlain); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for random default-affix item without template affix, got %v", kind)
	}
	plainWithAffix := base
	plainWithAffix.TemplateItem = "plain_cloth"
	plainWithAffix.BaseSnapshot = []byte(`{}`)
	plainWithAffix.Durability = 100
	plainWithAffix.MaxDurability = 100
	plainWithAffix.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.taunt","rarityId":"common"}]`)
	if kind := s.lockedInstanceRecycleTemplateMatchKind(plainWithAffix); kind != recycleTemplateMatchNone {
		t.Fatalf("expected non-recyclable match kind for template-without-affix but instance-with-affix, got %v", kind)
	}
}

func TestTemplateItemMarketListable(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			Items: []catalog.Item{
				{ID: "wood", Type: []string{"material"}, MarketListable: true},
				{ID: "gold_coin", Type: []string{"currency"}, MarketListable: false},
				{ID: "bound_scroll", Type: []string{"special"}, MarketListable: false},
				{ID: "random_blades", Type: []string{"equipment"}, MarketListable: true},
			},
			Actions: []catalog.ActionDefinition{
				{
					ID: "craft_random_blades",
					ItemReward: map[string]catalog.ActionItemReward{
						"random_blades": {GrantPolicy: InventoryGrantPolicyRollDefaultAffixes},
					},
				},
			},
		},
	}
	if !s.TemplateItemMarketListable("wood") {
		t.Fatalf("expected normal marketListable material to be fixed-market listable")
	}
	for _, itemID := range []string{"gold_coin", "bound_scroll", "random_blades", "missing"} {
		if s.TemplateItemMarketListable(itemID) {
			t.Fatalf("expected %s to be excluded from fixed-price market", itemID)
		}
	}
}
