package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

func destructiveRecycleTestStore() *Store {
	s := &Store{
		config: RuntimeConfig{ItemRecycleGoldCoefficient: 0.75, ItemRecycleAffixMismatchExtraCoefficient: 0.6},
		gameData: catalog.Data{
			Items: []catalog.Item{
				{ID: "shield", EconomyBaseValue: 100},
				{ID: "plain", EconomyBaseValue: 100},
				{ID: GoldCoinItemID, Stackable: true, EconomyBaseValue: 1},
			},
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "shield", Slot: "offhand", AffixSlotCount: 3, BaseAttrs: map[string]float64{"armor": 3}, MaxDurability: 100,
					DefaultAffixSlots: []catalog.DefaultAffixSlot{{SlotIndex: 0, AffixID: "affix.native", RarityID: "common"}}},
				{ID: "plain", Slot: "body", AffixSlotCount: 3, BaseAttrs: map[string]float64{"armor": 3}, MaxDurability: 100},
			},
			AffixDefinitions: []catalog.AffixDefinition{
				{ID: "affix.added", Active: true, MaxEffectivePerEquipment: 1},
				{ID: "affix.native", Active: true, AllowedRarities: []string{"common"}},
			},
		},
	}
	s.enhanceCache = &EnhanceRuntimeCache{byTemplateID: map[string]EnhanceTemplateRuntime{}}
	for _, id := range []string{"shield", "plain"} {
		levels := map[int]map[string]float64{}
		for level := 0; level <= 20; level++ {
			levels[level] = map[string]float64{"armor": float64(level * 2)}
		}
		s.enhanceCache.byTemplateID[id] = EnhanceTemplateRuntime{MaxLevel: 20, CumulativeByLevel: levels}
	}
	return s
}

func destructiveRecycleTestRow() LockedItemInstance {
	return LockedItemInstance{InstanceID: 11, UserID: 42, TemplateItem: "shield", Quantity: 1, State: "inventory", Durability: 100, MaxDurability: 100,
		BaseSnapshot: []byte(`{"armor":8}`), AffixesRaw: []byte(`[{"slotIndex":0,"affixId":"affix.added","rarityId":"epic","attrMods":{"armor":5}}]`)}
}

func quoteDestructiveTestRow(s *Store, row LockedItemInstance) (DestructiveRecycleQuote, error) {
	_, quote, err := s.prepareDestructiveRecycle(42, []RecycleBatchItemInput{{InstanceID: row.InstanceID, Quantity: 1}}, nil, map[int64]LockedItemInstance{row.InstanceID: row}, false)
	return quote, err
}

func TestDestructiveRecycleBodyPriceAndNoMutation(t *testing.T) {
	s := destructiveRecycleTestStore()
	row := destructiveRecycleTestRow()
	beforeAffixes, beforeBase := bytes.Clone(row.AffixesRaw), bytes.Clone(row.BaseSnapshot)
	quote, err := quoteDestructiveTestRow(s, row)
	if err != nil {
		t.Fatal(err)
	}
	if quote.BodyGold != 35 || quote.DisposalFee != 4 || quote.TotalGold != 31 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	if !bytes.Equal(row.AffixesRaw, beforeAffixes) || !bytes.Equal(row.BaseSnapshot, beforeBase) {
		t.Fatal("pricing mutated the original equipment")
	}
	if s.lockedInstanceRecycleTemplateMatchKind(row) != recycleTemplateMatchNone {
		t.Fatal("ordinary recycling changed")
	}
	row.TemplateItem = "plain"
	plain, err := quoteDestructiveTestRow(s, row)
	if err != nil || plain.BodyGold != 75 || plain.DisposalFee != 8 || plain.TotalGold != 67 {
		t.Fatalf("plain body: %+v, %v", plain, err)
	}
	row.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.added","rarityId":"mythic","attrMods":{"armor":999999}}]`)
	row.BaseSnapshot = []byte(`{"armor":1000002}`)
	valuable, err := quoteDestructiveTestRow(s, row)
	if err != nil || valuable.TotalGold != plain.TotalGold {
		t.Fatalf("affix increased payout: %+v, %v", valuable, err)
	}
}

func TestDestructiveRecycleRejectsOtherRestrictions(t *testing.T) {
	tests := []struct {
		name   string
		change func(*LockedItemInstance)
	}{
		{"negative enhancement", func(r *LockedItemInstance) { r.EnhanceLevel = -1 }},
		{"enhancement beyond maximum", func(r *LockedItemInstance) { r.EnhanceLevel = 21 }},
		{"damaged", func(r *LockedItemInstance) { r.Durability = 99 }},
		{"max durability", func(r *LockedItemInstance) { r.Durability = 101; r.MaxDurability = 101 }},
		{"renamed", func(r *LockedItemInstance) { r.CustomData.Name = "Keep me" }},
		{"color", func(r *LockedItemInstance) { r.CustomData.LabelColor = "red" }},
		{"equipped", func(r *LockedItemInstance) { r.State = "equipped" }},
		{"auction", func(r *LockedItemInstance) { r.State = "auction" }},
		{"other owner", func(r *LockedItemInstance) { r.UserID = 7 }},
		{"stack", func(r *LockedItemInstance) { r.Quantity = 2 }},
		{"empty", func(r *LockedItemInstance) { r.Quantity = 0 }},
		{"modified base", func(r *LockedItemInstance) { r.BaseSnapshot = []byte(`{"armor":9}`) }},
		{"invalid json", func(r *LockedItemInstance) { r.AffixesRaw = []byte(`{`) }},
		{"empty affixes", func(r *LockedItemInstance) { r.AffixesRaw = []byte(`[]`) }},
		{"invalid slot", func(r *LockedItemInstance) {
			r.AffixesRaw = []byte(`[{"slotIndex":99,"affixId":"x","rarityId":"epic"}]`)
		}},
		{"duplicate slot", func(r *LockedItemInstance) {
			r.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"x","rarityId":"epic"},{"slotIndex":0,"affixId":"y","rarityId":"epic"}]`)
		}},
		{"missing identity", func(r *LockedItemInstance) { r.AffixesRaw = []byte(`[{"slotIndex":0}]`) }},
		{"missing rarity", func(r *LockedItemInstance) { r.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"x"}]`) }},
		{"unknown template", func(r *LockedItemInstance) { r.TemplateItem = "missing" }},
		{"currency", func(r *LockedItemInstance) { r.TemplateItem = GoldCoinItemID }},
		{"ordinary recyclable", func(r *LockedItemInstance) {
			r.BaseSnapshot = []byte(`{"armor":3}`)
			r.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.native","rarityId":"common"}]`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := destructiveRecycleTestRow()
			tt.change(&r)
			if _, err := quoteDestructiveTestRow(destructiveRecycleTestStore(), r); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestDestructiveRecycleEnhancementDoesNotIncreasePrice(t *testing.T) {
	s := destructiveRecycleTestStore()
	for _, templateID := range []string{"shield", "plain"} {
		for _, withAffixes := range []bool{false, true} {
			for _, level := range []int{1, 10, 20} {
				row := destructiveRecycleTestRow()
				row.TemplateItem, row.EnhanceLevel = templateID, level
				armor := 3 + level*2
				if withAffixes {
					armor += 5
				} else {
					row.AffixesRaw = []byte(`[]`)
				}
				row.BaseSnapshot, _ = json.Marshal(map[string]int{"armor": armor})
				original, _ := json.Marshal(row)
				quote, err := quoteDestructiveTestRow(s, row)
				if err != nil {
					t.Fatalf("%s +%d affixes=%v: %v", templateID, level, withAffixes, err)
				}
				wantGold := int64(31)
				if templateID == "plain" {
					wantGold = 67
				}
				if quote.TotalGold != wantGold || quote.Items[0].EnhanceLevel != level {
					t.Fatalf("enhancement affected price or was omitted: %+v", quote)
				}
				after, _ := json.Marshal(row)
				if !bytes.Equal(original, after) {
					t.Fatal("pricing mutated original enhancement")
				}
				if s.lockedInstanceRecycleTemplateMatchKind(row) != recycleTemplateMatchNone {
					t.Fatal("ordinary recycling broadened")
				}
				row.BaseSnapshot = []byte(`{"armor":9999}`)
				if _, err := quoteDestructiveTestRow(s, row); err == nil {
					t.Fatal("arbitrary enhanced base accepted")
				}
			}
		}
	}
	row := destructiveRecycleTestRow()
	row.EnhanceLevel = 1
	s.enhanceCache = nil
	if _, err := quoteDestructiveTestRow(s, row); err == nil {
		t.Fatal("enhanced equipment accepted without authoritative curve")
	}
}

func TestDestructiveRecycleLegacyAndEffectiveSnapshots(t *testing.T) {
	s := destructiveRecycleTestStore()
	row := destructiveRecycleTestRow()
	for _, base := range []string{`{}`, `{"armor":3}`, `{"armor":8}`} {
		row.BaseSnapshot = []byte(base)
		if _, err := quoteDestructiveTestRow(s, row); err != nil {
			t.Fatalf("base %s: %v", base, err)
		}
	}
	// Only the first of two limited affixes contributes to the stored base.
	row.AffixesRaw = []byte(`[{"slotIndex":0,"affixId":"affix.added","rarityId":"epic","attrMods":{"armor":5}},{"slotIndex":1,"affixId":"affix.added","rarityId":"rare","attrMods":{"armor":2}}]`)
	row.BaseSnapshot = []byte(`{"armor":8}`)
	now := time.Now()
	row.ArchivedAt = &now
	q, err := quoteDestructiveTestRow(s, row)
	if err != nil || len(q.Items[0].Affixes) != 2 {
		t.Fatalf("archived/effective snapshot: %+v %v", q, err)
	}
}

func TestDestructiveRecycleQuoteBindsOriginalState(t *testing.T) {
	s := destructiveRecycleTestStore()
	row := destructiveRecycleTestRow()
	q, err := quoteDestructiveTestRow(s, row)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []RecycleBatchItemInput{{InstanceID: row.InstanceID, Quantity: 1}}
	tokens := map[int64]string{row.InstanceID: q.Items[0].QuoteToken}
	if _, _, err := s.prepareDestructiveRecycle(42, inputs, tokens, map[int64]LockedItemInstance{row.InstanceID: row}, true); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*LockedItemInstance){
		func(r *LockedItemInstance) {
			r.AffixesRaw = bytes.ReplaceAll(r.AffixesRaw, []byte("epic"), []byte("legendary"))
		},
		func(r *LockedItemInstance) { r.RollSeed++ },
		func(r *LockedItemInstance) { r.EnhanceLevel = 1; r.BaseSnapshot = []byte(`{"armor":3}`) },
		func(r *LockedItemInstance) { r.MilestoneAffixesRaw = []byte(`[1]`) },
		func(r *LockedItemInstance) { now := time.Now(); r.ArchivedAt = &now },
	} {
		changed := row
		change(&changed)
		if _, _, err := s.prepareDestructiveRecycle(42, inputs, tokens, map[int64]LockedItemInstance{row.InstanceID: changed}, true); !errors.Is(err, ErrDestructiveRecycleChanged) {
			t.Fatalf("expected stale quote rejection, got %v", err)
		}
	}
	s.config.ItemRecycleGoldCoefficient = 0.8
	if _, _, err := s.prepareDestructiveRecycle(42, inputs, tokens, map[int64]LockedItemInstance{row.InstanceID: row}, true); !errors.Is(err, ErrDestructiveRecycleChanged) {
		t.Fatalf("price change accepted: %v", err)
	}
}

func TestDestructiveRecycleRoundingAndBounds(t *testing.T) {
	for body := int64(2); body < 10000; body++ {
		fee, net, err := destructiveRecycleNetGold(body)
		if err != nil || fee != (body+9)/10 || fee+net != body || net >= body {
			t.Fatalf("body=%d fee=%d net=%d err=%v", body, fee, net, err)
		}
	}
	for _, body := range []int64{-1, 0, 1, destructiveRecycleMaxGold + 1, math.MaxInt64} {
		if _, _, err := destructiveRecycleNetGold(body); err == nil {
			t.Fatalf("accepted %d", body)
		}
	}
	s := destructiveRecycleTestStore()
	s.gameData.Items[0].EconomyBaseValue = float64(destructiveRecycleMaxGold)
	s.config.ItemRecycleGoldCoefficient = 1
	s.config.ItemRecycleAffixMismatchExtraCoefficient = 1
	r1 := destructiveRecycleTestRow()
	r2 := r1
	r2.InstanceID++
	if _, _, err := s.prepareDestructiveRecycle(42, []RecycleBatchItemInput{{InstanceID: r1.InstanceID, Quantity: 1}, {InstanceID: r2.InstanceID, Quantity: 1}}, nil, map[int64]LockedItemInstance{r1.InstanceID: r1, r2.InstanceID: r2}, false); !errors.Is(err, ErrRecyclePriceInvalid) {
		t.Fatalf("batch overflow accepted: %v", err)
	}
}

func TestDestructiveRecycleInputRequiresExplicitFreshConsent(t *testing.T) {
	token := strings.Repeat("a", 64)
	for _, items := range [][]DestructiveRecycleInput{nil, {{InstanceID: 0}}, {{InstanceID: 11}}, {{InstanceID: 11, QuoteToken: "invalid"}}, {{InstanceID: 11, QuoteToken: token}, {InstanceID: 11, QuoteToken: token}}, make([]DestructiveRecycleInput, RecycleBatchItemLimit+1)} {
		if _, _, err := normalizeDestructiveRecycleItems(items, true); err == nil {
			t.Fatal("invalid confirmation accepted")
		}
	}
	inputs, _, err := normalizeDestructiveRecycleItems([]DestructiveRecycleInput{{InstanceID: 12, QuoteToken: token}, {InstanceID: 11, QuoteToken: token}}, true)
	if err != nil || inputs[0].InstanceID != 11 || inputs[0].Quantity != 1 {
		t.Fatalf("bad normalization: %+v %v", inputs, err)
	}
}
