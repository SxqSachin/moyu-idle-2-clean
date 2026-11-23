package store

import (
	"context"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestItemStackablePrefersCatalog(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			Items: []catalog.Item{
				{ID: "wood", Stackable: true},
				{ID: "engrave_stone", Stackable: false},
			},
		},
	}

	if !s.itemStackable("wood") {
		t.Fatalf("expected wood to be stackable from catalog")
	}
	if s.itemStackable("engrave_stone") {
		t.Fatalf("expected engrave_stone to be non-stackable from catalog")
	}
}

func TestItemStackableFallback(t *testing.T) {
	s := Store{}
	if s.itemStackable("wood_sword") {
		t.Fatalf("expected fallback wood_sword to be non-stackable")
	}
	if !s.itemStackable("wood") {
		t.Fatalf("expected fallback wood to be stackable")
	}
}

func TestDefaultMaxDurability(t *testing.T) {
	s := Store{
		gameData: catalog.Data{
			EquipmentTemplates: []catalog.EquipmentTemplate{
				{ID: "wood_sword", MaxDurability: 240},
			},
		},
	}

	if got := s.defaultMaxDurability("wood_sword"); got != 240 {
		t.Fatalf("defaultMaxDurability(wood_sword)=%d, want 240", got)
	}
	if got := s.defaultMaxDurability("wood"); got != 100 {
		t.Fatalf("defaultMaxDurability(wood)=%d, want 100", got)
	}
}

func TestInventoryGrantLayout(t *testing.T) {
	rows, qtyPerRow, err := inventoryGrantLayout(true, 9)
	if err != nil {
		t.Fatalf("stackable layout returned err: %v", err)
	}
	if rows != 1 || qtyPerRow != 9 {
		t.Fatalf("stackable layout=(%d,%d), want (1,9)", rows, qtyPerRow)
	}

	rows, qtyPerRow, err = inventoryGrantLayout(false, 9)
	if err != nil {
		t.Fatalf("non-stackable layout returned err: %v", err)
	}
	if rows != 9 || qtyPerRow != 1 {
		t.Fatalf("non-stackable layout=(%d,%d), want (9,1)", rows, qtyPerRow)
	}
}

func TestInventoryGrantLayoutRejectsBadQuantity(t *testing.T) {
	if _, _, err := inventoryGrantLayout(true, 0); err == nil {
		t.Fatalf("expected error for qty=0")
	}
	if _, _, err := inventoryGrantLayout(false, -3); err == nil {
		t.Fatalf("expected error for qty<0")
	}
}

func TestNormalizeArchiveInstanceIDsDedupesAndSorts(t *testing.T) {
	got, err := normalizeArchiveInstanceIDs([]int64{9, 3, 9, 5})
	if err != nil {
		t.Fatalf("normalizeArchiveInstanceIDs returned err: %v", err)
	}
	want := []int64{3, 5, 9}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d (%v)", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNormalizeArchiveInstanceIDsRejectsInvalidBatches(t *testing.T) {
	if _, err := normalizeArchiveInstanceIDs(nil); err == nil {
		t.Fatalf("expected nil batch to be rejected")
	}
	if _, err := normalizeArchiveInstanceIDs([]int64{1, 0}); err == nil {
		t.Fatalf("expected non-positive instance id to be rejected")
	}
	tooMany := make([]int64, InventoryArchiveBatchLimit+1)
	for idx := range tooMany {
		tooMany[idx] = int64(idx + 1)
	}
	if _, err := normalizeArchiveInstanceIDs(tooMany); err == nil {
		t.Fatalf("expected oversized batch to be rejected")
	}
}

func TestNormalizeArchiveInstanceIDsAllowsBatchLimit(t *testing.T) {
	ids := make([]int64, InventoryArchiveBatchLimit)
	for idx := range ids {
		ids[idx] = int64(idx + 1)
	}
	got, err := normalizeArchiveInstanceIDs(ids)
	if err != nil {
		t.Fatalf("normalizeArchiveInstanceIDs returned err: %v", err)
	}
	if len(got) != InventoryArchiveBatchLimit {
		t.Fatalf("len=%d, want %d", len(got), InventoryArchiveBatchLimit)
	}
}

func TestShouldMergeInventoryStack(t *testing.T) {
	if !shouldMergeInventoryStack(true) {
		t.Fatalf("stackable items should merge into existing stack")
	}
	if shouldMergeInventoryStack(false) {
		t.Fatalf("non-stackable items should not merge into stack")
	}
}

func TestAddInventoryItemInstanceRejectsBatchQuantity(t *testing.T) {
	s := Store{}
	if _, err := s.AddInventoryItemInstanceTx(context.Background(), nil, 1, "wood_staff", 2, nil, nil); err == nil {
		t.Fatalf("expected batch instance insert to be rejected")
	}
}
