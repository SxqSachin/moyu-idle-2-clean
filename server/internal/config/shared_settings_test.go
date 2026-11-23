package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSharedSettingsFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shared-global-config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture failed: %v", err)
	}
	return path
}

func TestLoadSharedGlobalSettingsKittyDefaults(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  }
}`)
	settings, err := LoadSharedGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadSharedGlobalSettings failed: %v", err)
	}
	if settings.Kitty.ActionDurationMultiplier != 1.5 {
		t.Fatalf("kitty.actionDurationMultiplier=%v, want 1.5", settings.Kitty.ActionDurationMultiplier)
	}
	if settings.Kitty.StatusExpMultiplier != 0 {
		t.Fatalf("kitty.statusExpMultiplier=%v, want 0", settings.Kitty.StatusExpMultiplier)
	}
	if settings.Kitty.BattleAttrExpMultiplier != 1 {
		t.Fatalf("kitty.battleAttrExpMultiplier=%v, want 1", settings.Kitty.BattleAttrExpMultiplier)
	}
	if settings.Kitty.BattleSkillExpMultiplier != 1 {
		t.Fatalf("kitty.battleSkillExpMultiplier=%v, want 1", settings.Kitty.BattleSkillExpMultiplier)
	}
	if settings.Kitty.BattleLevelCapDefault != 100 {
		t.Fatalf("kitty.battleLevelCapDefault=%v, want 100", settings.Kitty.BattleLevelCapDefault)
	}
	if len(settings.Kitty.BattleEquipSlots) == 0 {
		t.Fatalf("kitty.battleEquipSlots should not be empty")
	}
	if settings.Kitty.DefaultPersonalityID != "balanced" {
		t.Fatalf("kitty.defaultPersonalityId=%v, want balanced", settings.Kitty.DefaultPersonalityID)
	}
	if settings.Presence.VisibilityOfflineDelaySeconds != 120 {
		t.Fatalf("presence.visibilityOfflineDelaySeconds=%v, want 120", settings.Presence.VisibilityOfflineDelaySeconds)
	}
	if settings.Action.EnhanceInheritanceRatio != 0.5 {
		t.Fatalf("action.enhanceInheritanceRatio=%v, want 0.5", settings.Action.EnhanceInheritanceRatio)
	}
	if settings.Action.QueueLaneLimit != 30 {
		t.Fatalf("action.queueLaneLimit=%v, want 30", settings.Action.QueueLaneLimit)
	}
	if settings.Action.KittyQueueLaneLimit != 20 {
		t.Fatalf("action.kittyQueueLaneLimit=%v, want 20", settings.Action.KittyQueueLaneLimit)
	}
	if settings.Auction.FeeRateBps != 500 {
		t.Fatalf("auction.feeRateBps=%v, want 500", settings.Auction.FeeRateBps)
	}
	if settings.Auction.MinBidIncrement != 1 {
		t.Fatalf("auction.minBidIncrement=%v, want 1", settings.Auction.MinBidIncrement)
	}
	if settings.Auction.AntiSnipeTriggerSeconds != 1800 {
		t.Fatalf("auction.antiSnipeTriggerSeconds=%v, want 1800", settings.Auction.AntiSnipeTriggerSeconds)
	}
	if settings.Auction.AntiSnipeExtendSeconds != 600 {
		t.Fatalf("auction.antiSnipeExtendSeconds=%v, want 600", settings.Auction.AntiSnipeExtendSeconds)
	}
	if settings.ItemRename.MaxNameRunes != 16 {
		t.Fatalf("itemRename.maxNameRunes=%v, want 16", settings.ItemRename.MaxNameRunes)
	}
	if settings.ItemRename.Cost["harmonic_embryo"] != 1 {
		t.Fatalf("itemRename.cost.harmonic_embryo=%v, want 1", settings.ItemRename.Cost["harmonic_embryo"])
	}
	if settings.PlayerUsernameRename.CostItemID != "harmonic_embryo" {
		t.Fatalf("playerUsernameRename.costItemId=%v, want harmonic_embryo", settings.PlayerUsernameRename.CostItemID)
	}
	if settings.PlayerUsernameRename.CostItemCount != 1 {
		t.Fatalf("playerUsernameRename.costItemCount=%v, want 1", settings.PlayerUsernameRename.CostItemCount)
	}
	if settings.BattleItemBagMaxKinds != 4 {
		t.Fatalf("battleItemBagMaxKinds=%v, want 4", settings.BattleItemBagMaxKinds)
	}
	if settings.BattleItemBagMaxTotalQuantity != 1000 {
		t.Fatalf("battleItemBagMaxTotalQuantity=%v, want 1000", settings.BattleItemBagMaxTotalQuantity)
	}
	if settings.IntervalBetweenItemAndAction != 100 {
		t.Fatalf("intervalBetweenItemAndAction=%v, want 100", settings.IntervalBetweenItemAndAction)
	}
	if settings.CombatBalanceModel.MaxBattleLevel != 500 {
		t.Fatalf("combatBalanceModel.maxBattleLevel=%v, want 500", settings.CombatBalanceModel.MaxBattleLevel)
	}
	if settings.CombatBalanceModel.PrimaryPointCap != 500 {
		t.Fatalf("combatBalanceModel.primaryPointCap=%v, want 500", settings.CombatBalanceModel.PrimaryPointCap)
	}
	if settings.CombatBalanceModel.DefenseK != 100 {
		t.Fatalf("combatBalanceModel.defenseKBase=%v, want 100", settings.CombatBalanceModel.DefenseK)
	}
	if len(settings.CombatBalanceModel.LevelBands) == 0 || settings.CombatBalanceModel.LevelBands[len(settings.CombatBalanceModel.LevelBands)-1].Level != 500 {
		t.Fatalf("combatBalanceModel.levelBands should include level 500")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidPlayerUsernameRenameCost(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name: "empty item id",
			config: `{
  "market": {"priceSteps": [{"min": 0, "max": null, "step": 1}]},
  "playerUsernameRename": {"costItemId": "  ", "costItemCount": 1}
}`,
		},
		{
			name: "non-positive item count",
			config: `{
  "market": {"priceSteps": [{"min": 0, "max": null, "step": 1}]},
  "playerUsernameRename": {"costItemId": "harmonic_embryo", "costItemCount": 0}
}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeSharedSettingsFixture(t, tc.config)
			if _, err := LoadSharedGlobalSettings(path); err == nil {
				t.Fatalf("expected playerUsernameRename cost validation error")
			}
		})
	}
}

func TestLoadSharedGlobalSettingsNormalizesPlayerUsernameRenameCost(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {"priceSteps": [{"min": 0, "max": null, "step": 1}]},
  "playerUsernameRename": {"costItemId": "  harmonic_embryo  ", "costItemCount": 3}
}`)
	settings, err := LoadSharedGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadSharedGlobalSettings failed: %v", err)
	}
	if settings.PlayerUsernameRename.CostItemID != "harmonic_embryo" {
		t.Fatalf("playerUsernameRename.costItemId=%q, want harmonic_embryo", settings.PlayerUsernameRename.CostItemID)
	}
	if settings.PlayerUsernameRename.CostItemCount != 3 {
		t.Fatalf("playerUsernameRename.costItemCount=%d, want 3", settings.PlayerUsernameRename.CostItemCount)
	}
}

func TestValidatePlayerUsernameRenameItemID(t *testing.T) {
	settings := SharedPlayerUsernameRenameSettings{CostItemID: "  harmonic_embryo  ", CostItemCount: 1}
	if err := ValidatePlayerUsernameRenameItemID(settings, map[string]bool{"harmonic_embryo": true}); err != nil {
		t.Fatalf("ValidatePlayerUsernameRenameItemID failed: %v", err)
	}
	if err := ValidatePlayerUsernameRenameItemID(settings, map[string]bool{"gold_coin": true}); err == nil {
		t.Fatal("expected missing player username rename item validation error")
	}
	if err := ValidatePlayerUsernameRenameItemID(settings, nil); err != nil {
		t.Fatalf("empty catalog should skip item validation: %v", err)
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidKittyBattleExpMultiplier(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "kitty": {
    "actionDurationMultiplier": 1.2,
    "statusExpMultiplier": 0,
    "battleAttrExpMultiplier": 2
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected kitty.battleAttrExpMultiplier validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidKittyBattleLevelCapDefault(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "kitty": {
    "battleLevelCapDefault": 0
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected kitty.battleLevelCapDefault validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidKittyPersonalityDefaultID(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "kitty": {
    "defaultPersonalityId": ""
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected kitty.defaultPersonalityId validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidKittyDurationMultiplier(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "kitty": {
    "actionDurationMultiplier": 0,
    "statusExpMultiplier": 0
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected kitty.actionDurationMultiplier validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidKittyExpMultiplier(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "kitty": {
    "actionDurationMultiplier": 1.2,
    "statusExpMultiplier": -0.1
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected kitty.statusExpMultiplier validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidPresenceDelay(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "presence": {
    "visibilityOfflineDelaySeconds": -1
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected presence.visibilityOfflineDelaySeconds validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidAuctionMinBidIncrement(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "auction": {
    "feeRateBps": 500,
    "minBidIncrement": 0,
    "antiSnipeTriggerSeconds": 1800,
    "antiSnipeExtendSeconds": 600
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected auction.minBidIncrement validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidBattleItemBagLimits(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "battleItemBagMaxKinds": 0
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected battleItemBagMaxKinds validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidActionQueueLimit(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "action": {
    "enhanceInheritanceRatio": 0.5,
    "queueLaneLimit": 0,
    "kittyQueueLaneLimit": 20
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected action.queueLaneLimit validation error")
	}
}

func TestLoadSharedGlobalSettingsRejectInvalidKittyActionQueueLimit(t *testing.T) {
	path := writeSharedSettingsFixture(t, `{
  "market": {
    "priceSteps": [
      { "min": 0, "max": 100, "step": 1 },
      { "min": 100, "max": null, "step": 10 }
    ]
  },
  "action": {
    "enhanceInheritanceRatio": 0.5,
    "queueLaneLimit": 30,
    "kittyQueueLaneLimit": 0
  }
}`)
	if _, err := LoadSharedGlobalSettings(path); err == nil {
		t.Fatalf("expected action.kittyQueueLaneLimit validation error")
	}
}
