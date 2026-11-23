package store

import (
	"testing"
)

func TestSlotToCategory(t *testing.T) {
	cases := []struct {
		slotID      string
		wantCat     string
		wantOK      bool
	}{
		{EquipSlotTitlePrefix, CosmeticCategoryTitlePrefix, true},
		{EquipSlotTitleSuffix, CosmeticCategoryTitleSuffix, true},
		{EquipSlotKittyTitlePrefix, CosmeticCategoryTitlePrefix, true},
		{EquipSlotKittyTitleSuffix, CosmeticCategoryTitleSuffix, true},
		{EquipSlotKittyAvatar, CosmeticCategoryAvatar, true},
		{EquipSlotKittyAvatarFrame, CosmeticCategoryAvatarFrame, true},
		{"nonexistent", "", false},
		{"", "", false},
		{"title_prefix_extra", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.slotID, func(t *testing.T) {
			cat, ok := slotToCategory(tc.slotID)
			if ok != tc.wantOK || cat != tc.wantCat {
				t.Fatalf("slotToCategory(%q) = (%q, %v), want (%q, %v)", tc.slotID, cat, ok, tc.wantCat, tc.wantOK)
			}
		})
	}
}

func TestValidEquipSlot(t *testing.T) {
	playerSlots := []string{EquipSlotTitlePrefix, EquipSlotTitleSuffix}
	for _, slot := range playerSlots {
		if !validEquipSlot(slot) {
			t.Errorf("validEquipSlot(%q) = false, want true", slot)
		}
	}
	kittySlots := []string{EquipSlotKittyTitlePrefix, EquipSlotKittyTitleSuffix, EquipSlotKittyAvatar, EquipSlotKittyAvatarFrame}
	for _, slot := range kittySlots {
		if validEquipSlot(slot) {
			t.Errorf("validEquipSlot(%q) = true, want false (kitty slot)", slot)
		}
	}
	if validEquipSlot("bogus") {
		t.Error("validEquipSlot(\"bogus\") = true, want false")
	}
}

func TestValidKittySlot(t *testing.T) {
	kittySlots := []string{EquipSlotKittyTitlePrefix, EquipSlotKittyTitleSuffix, EquipSlotKittyAvatar, EquipSlotKittyAvatarFrame}
	for _, slot := range kittySlots {
		if !validKittySlot(slot) {
			t.Errorf("validKittySlot(%q) = false, want true", slot)
		}
	}
	playerSlots := []string{EquipSlotTitlePrefix, EquipSlotTitleSuffix}
	for _, slot := range playerSlots {
		if validKittySlot(slot) {
			t.Errorf("validKittySlot(%q) = true, want false (player slot)", slot)
		}
	}
}

func TestValidCosmeticCategory(t *testing.T) {
	valid := []string{CosmeticCategoryTitlePrefix, CosmeticCategoryTitleSuffix, CosmeticCategoryAvatar, CosmeticCategoryAvatarFrame}
	for _, cat := range valid {
		if !validCosmeticCategory(cat) {
			t.Errorf("validCosmeticCategory(%q) = false, want true", cat)
		}
	}
	invalid := []string{"", "weapon", "armor", "title"}
	for _, cat := range invalid {
		if validCosmeticCategory(cat) {
			t.Errorf("validCosmeticCategory(%q) = true, want false", cat)
		}
	}
}

func TestCosmeticConstants(t *testing.T) {
	// Verify slot constants are consistent with category mapping.
	for slotID, expectedCat := range playerEquipSlots {
		cat, ok := slotToCategory(slotID)
		if !ok {
			t.Errorf("player slot %q not found in slotToCategory", slotID)
			continue
		}
		if cat != expectedCat {
			t.Errorf("player slot %q → category %q, want %q", slotID, cat, expectedCat)
		}
	}
	for slotID, expectedCat := range kittyEquipSlots {
		cat, ok := slotToCategory(slotID)
		if !ok {
			t.Errorf("kitty slot %q not found in slotToCategory", slotID)
			continue
		}
		if cat != expectedCat {
			t.Errorf("kitty slot %q → category %q, want %q", slotID, cat, expectedCat)
		}
	}
}
