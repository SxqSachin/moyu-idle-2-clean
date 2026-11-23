package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/emailpolicy"
)

func writeGlobalSettingsFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "global-config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture failed: %v", err)
	}
	return path
}

func TestLoadGlobalSettingsChatSortAndNormalize(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{
  "chat": {
    "maxContentLength": 512,
    "defaultHistoryLimit": 40,
    "channels": [
      {"id": " Battle ", "nameKey": "chat.channel.battle", "order": 40, "playerSendable": true},
      {"id": "world", "nameKey": "chat.channel.world", "order": 10, "playerSendable": true},
      {"id": "system", "nameKey": "chat.channel.system", "order": 20, "playerSendable": false}
    ]
  }
}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if got := settings.Chat.MaxContentLength; got != 512 {
		t.Fatalf("chat.maxContentLength=%d, want 512", got)
	}
	if len(settings.Chat.Channels) != 3 {
		t.Fatalf("chat.channels len=%d, want 3", len(settings.Chat.Channels))
	}
	if settings.Chat.Channels[0].ID != "world" {
		t.Fatalf("channels[0].id=%q, want world", settings.Chat.Channels[0].ID)
	}
	if settings.Chat.Channels[1].ID != "system" {
		t.Fatalf("channels[1].id=%q, want system", settings.Chat.Channels[1].ID)
	}
	if settings.Chat.Channels[2].ID != "battle" {
		t.Fatalf("channels[2].id=%q, want battle", settings.Chat.Channels[2].ID)
	}
}

func TestLoadGlobalSettingsChatSendRateLimitDefaultsAndValidation(t *testing.T) {
	settings, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, `{}`))
	if err != nil {
		t.Fatalf("LoadGlobalSettings defaults failed: %v", err)
	}
	if settings.RateLimit.ChatSend.Max != 10 || settings.RateLimit.ChatSend.WindowSeconds != 10 {
		t.Fatalf("chat send rate limit=%#v", settings.RateLimit.ChatSend)
	}

	for _, raw := range []string{
		`{"rateLimit":{"chatSend":{"max":0,"windowSeconds":10}}}`,
		`{"rateLimit":{"chatSend":{"max":10,"windowSeconds":0}}}`,
		`{"rateLimit":{"chatSend":{"max":101,"windowSeconds":10}}}`,
		`{"rateLimit":{"chatSend":{"max":10,"windowSeconds":301}}}`,
	} {
		if _, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, raw)); err == nil {
			t.Fatalf("invalid chat send rate limit accepted: %s", raw)
		}
	}
}

func TestLoadGlobalSettingsRejectDuplicateChatChannelID(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{
  "chat": {
    "channels": [
      {"id": "world", "nameKey": "chat.channel.world", "order": 10, "playerSendable": true},
      {"id": "world", "nameKey": "chat.channel.world_dup", "order": 20, "playerSendable": true}
    ]
  }
}`)
	if _, err := LoadGlobalSettings(path); err == nil {
		t.Fatalf("expected duplicate channel id error")
	}
}

func TestLoadGlobalSettingsRejectNoSendableChannel(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{
  "chat": {
    "channels": [
      {"id": "system", "nameKey": "chat.channel.system", "order": 10, "playerSendable": false}
    ]
  }
}`)
	if _, err := LoadGlobalSettings(path); err == nil {
		t.Fatalf("expected no playerSendable channel error")
	}
}

func TestLoadGlobalSettingsDerivedStatsNormalizeAndParse(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{
  "derivedStats": {
    "version": "custom",
    "operators": [" + ", " * "],
    "formulas": [
      {
        "targetAttr": " physics_damage ",
        "terms": [
          {"attrKey": " strength ", "coefficient": 0.5}
        ]
      }
    ]
  }
}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if got := settings.DerivedStats.Version; got != "custom" {
		t.Fatalf("derivedStats.version=%q, want custom", got)
	}
	if got := settings.DerivedStats.Operators[0]; got != "+" {
		t.Fatalf("operators[0]=%q, want +", got)
	}
	formula := settings.DerivedStats.Formulas[0]
	if formula.TargetAttr != "physics_damage" {
		t.Fatalf("targetAttr=%q, want physics_damage", formula.TargetAttr)
	}
	if formula.Terms[0].AttrKey != "strength" {
		t.Fatalf("attrKey=%q, want strength", formula.Terms[0].AttrKey)
	}
	if got := BuildDerivedFormulaExpression(formula.Terms); got != "strength*0.5" {
		t.Fatalf("expression=%q, want strength*0.5", got)
	}
}

func TestLoadGlobalSettingsRejectInvalidDerivedStatsFormula(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{
  "derivedStats": {
    "formulas": [
      {
        "targetAttr": "physics_damage",
        "terms": [
          {"attrKey": "strength", "coefficient": 0}
        ]
      }
    ]
  }
}`)
	if _, err := LoadGlobalSettings(path); err == nil {
		t.Fatalf("expected invalid derived formula coefficient error")
	}
}

func TestLoadGlobalSettingsActionQueueFastForwardDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}

	if settings.ActionQueue.FastForward.MaxSteps != 10_000 {
		t.Fatalf("actionQueue.fastForward.maxSteps=%d, want 10000", settings.ActionQueue.FastForward.MaxSteps)
	}
	if settings.ActionQueue.FastForward.MaxExecMs != 1_000 {
		t.Fatalf("actionQueue.fastForward.maxExecMs=%d, want 1000", settings.ActionQueue.FastForward.MaxExecMs)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "actionQueue": {
    "fastForward": {
      "maxSteps": 0,
      "maxExecMs": 500
    }
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid actionQueue.fastForward.maxSteps error")
	}
}

func TestLoadGlobalSettingsSkillForgeBookCountDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	forge := settings.SkillForge
	if forge.CompilerVersion != 10 || forge.RulesetVersion != 1 || forge.BaseSuccessChance != 0.30 {
		t.Fatalf("unexpected production skill-forge chance baseline: %+v", forge)
	}
	if forge.SourceSkillbookMinCount != 200 || forge.SourceSkillbookMaxCount != 1000 || forge.SourceSkillbookCountStep != 100 {
		t.Fatalf("unexpected production source-skillbook counts: %+v", forge)
	}
	if forge.SourceSkillbookBonusStep != 0.10 || forge.SourceSkillbookMaxBonus != 0.80 {
		t.Fatalf("unexpected production source-skillbook bonuses: %+v", forge)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{"skillForge":{"sourceSkillbookMinCount":200,"sourceSkillbookMaxCount":950,"sourceSkillbookCountStep":100}}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatal("expected invalid skillForge source-skillbook range error")
	}
}

func TestLoadGlobalSettingsBattlePlaybackGapDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.Battle.CompileMaxTurns != 300 {
		t.Fatalf("battle.compileMaxTurns=%d, want 300", settings.Battle.CompileMaxTurns)
	}
	if settings.Battle.CompileTurnsPerPlayerUnit != 100 {
		t.Fatalf("battle.compileTurnsPerPlayerUnit=%d, want 100", settings.Battle.CompileTurnsPerPlayerUnit)
	}
	if settings.Battle.CompileKickoffSlots != 4 {
		t.Fatalf("battle.compileKickoffSlots=%d, want 4", settings.Battle.CompileKickoffSlots)
	}
	if settings.Battle.AttrExpWorkers != 4 {
		t.Fatalf("battle.attrExpWorkers=%d, want 4", settings.Battle.AttrExpWorkers)
	}
	if !settings.Battle.StateMachineV2 {
		t.Fatal("battle.stateMachineV2=false, want v2-only default")
	}
	if settings.Battle.PlaybackMaxGapMs != 3000 {
		t.Fatalf("battle.playbackMaxGapMs=%d, want 3000", settings.Battle.PlaybackMaxGapMs)
	}
	if settings.Battle.PlaybackFixedIntervalMs != settings.Battle.TimelinePushIntervalMs {
		t.Fatalf("battle.playbackFixedIntervalMs=%d, want equal timelinePushIntervalMs=%d", settings.Battle.PlaybackFixedIntervalMs, settings.Battle.TimelinePushIntervalMs)
	}
	if settings.Battle.PlaybackIntervalJitterTolerancePct != 10 {
		t.Fatalf("battle.playbackIntervalJitterTolerancePct=%d, want 10", settings.Battle.PlaybackIntervalJitterTolerancePct)
	}
	if !settings.Battle.SettlementRequirePlaybackFinished {
		t.Fatalf("battle.settlementRequirePlaybackFinished=false, want true")
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "playbackMaxGapMs": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid battle.playbackMaxGapMs error")
	}

	invalidTurnsPath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "compileMaxTurns": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidTurnsPath); err == nil {
		t.Fatalf("expected invalid battle.compileMaxTurns error")
	}

	invalidTurnsPerUnitPath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "compileTurnsPerPlayerUnit": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidTurnsPerUnitPath); err == nil {
		t.Fatalf("expected invalid battle.compileTurnsPerPlayerUnit error")
	}

	invalidKickoffSlotsPath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "compileKickoffSlots": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidKickoffSlotsPath); err == nil {
		t.Fatalf("expected invalid battle.compileKickoffSlots error")
	}

	invalidAttrExpWorkersPath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "attrExpWorkers": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidAttrExpWorkersPath); err == nil {
		t.Fatalf("expected invalid battle.attrExpWorkers error")
	}

	legacyStateMachinePath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "stateMachineV2": false
  }
}`)
	if _, err := LoadGlobalSettings(legacyStateMachinePath); err == nil {
		t.Fatalf("expected retired battle v1 state machine error")
	}

	invalidSettlePath := writeGlobalSettingsFixture(t, `{
  "battle": {
    "settlementRequirePlaybackFinished": false
  }
}`)
	if _, err := LoadGlobalSettings(invalidSettlePath); err == nil {
		t.Fatalf("expected invalid battle.settlementRequirePlaybackFinished error")
	}
}

func TestProductionGlobalSettingsUseFullBattleFinalizeCapacity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if settings.Battle.SettlementWorkers != 4 {
		t.Fatalf("battle.settlementWorkers=%d, want 4", settings.Battle.SettlementWorkers)
	}
	if settings.Battle.FinalizeSlots != 4 {
		t.Fatalf("battle.finalizeSlots=%d, want 4", settings.Battle.FinalizeSlots)
	}
}

func TestProductionGlobalSettingsBoundBattleAttrExpConcurrency(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if settings.Battle.AttrExpWorkers != 4 {
		t.Fatalf("battle.attrExpWorkers=%d, want 4", settings.Battle.AttrExpWorkers)
	}
}

func TestProductionGlobalSettingsUseExpandedActionQueueFastForwardBudget(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if settings.ActionQueue.FastForward.MaxSteps != 10_000 {
		t.Fatalf("actionQueue.fastForward.maxSteps=%d, want 10000", settings.ActionQueue.FastForward.MaxSteps)
	}
	if settings.ActionQueue.FastForward.MaxExecMs != 1_000 {
		t.Fatalf("actionQueue.fastForward.maxExecMs=%d, want 1000", settings.ActionQueue.FastForward.MaxExecMs)
	}
}

func TestProductionArenaShopStartsEmpty(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if len(settings.Arena.ShopOffers) != 0 {
		t.Fatalf("arena.shopOffers=%d, want 0", len(settings.Arena.ShopOffers))
	}
}

func TestArenaChallengeChargeDefaultsAndValidation(t *testing.T) {
	settings, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, `{}`))
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.Arena.ChallengeChargeCapacity != 10 {
		t.Fatalf("arena.challengeChargeCapacity=%d, want 10", settings.Arena.ChallengeChargeCapacity)
	}
	if settings.Arena.ChallengeChargeRecoverySeconds != 600 {
		t.Fatalf("arena.challengeChargeRecoverySeconds=%d, want 600", settings.Arena.ChallengeChargeRecoverySeconds)
	}
	if settings.Arena.OpponentChallengeCooldownSeconds != 900 {
		t.Fatalf("arena.opponentChallengeCooldownSeconds=%d, want 900", settings.Arena.OpponentChallengeCooldownSeconds)
	}
	if settings.Arena.RecommendationRefreshSeconds != 300 {
		t.Fatalf("arena.recommendationRefreshCooldownSeconds=%d, want 300", settings.Arena.RecommendationRefreshSeconds)
	}
	if settings.Arena.TargetCount != 12 || settings.Arena.NearbyTargetCount != 7 || settings.Arena.RelaxedTargetCount != 4 {
		t.Fatalf("arena target distribution=%d/%d/%d, want 12/7/4", settings.Arena.TargetCount, settings.Arena.NearbyTargetCount, settings.Arena.RelaxedTargetCount)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{"arena":{"challengeChargeCapacity":101}}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatal("expected oversized arena challenge charge capacity error")
	}
	invalidPath = writeGlobalSettingsFixture(t, `{"arena":{"targetCount":12,"nearbyTargetCount":9,"relaxedTargetCount":4}}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatal("expected oversized arena recommendation quotas error")
	}
	invalidPath = writeGlobalSettingsFixture(t, `{"arena":{"recommendationRefreshCooldownSeconds":0}}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatal("expected zero arena recommendation refresh cooldown error")
	}
}

func TestProductionArenaChallengeChargesBoundTargetRotationSpam(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if settings.Arena.ChallengeChargeCapacity != 10 || settings.Arena.ChallengeChargeRecoverySeconds != 600 || settings.Arena.OpponentChallengeCooldownSeconds != 900 {
		t.Fatalf("unexpected arena challenge policy: capacity=%d recovery=%d opponent=%d", settings.Arena.ChallengeChargeCapacity, settings.Arena.ChallengeChargeRecoverySeconds, settings.Arena.OpponentChallengeCooldownSeconds)
	}
	if settings.Arena.TargetCount != 12 || settings.Arena.NearbyTargetCount != 7 || settings.Arena.RelaxedTargetCount != 4 {
		t.Fatalf("unexpected arena recommendation policy: target=%d nearby=%d relaxed=%d", settings.Arena.TargetCount, settings.Arena.NearbyTargetCount, settings.Arena.RelaxedTargetCount)
	}
	if settings.Arena.RecommendationRefreshSeconds != 300 {
		t.Fatalf("unexpected arena recommendation refresh cooldown: %d", settings.Arena.RecommendationRefreshSeconds)
	}
}

func TestLoadGlobalSettingsKittyJournalDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.KittyJournal.IntervalSeconds != 1800 {
		t.Fatalf("kittyJournal.intervalSeconds=%d, want 1800", settings.KittyJournal.IntervalSeconds)
	}
	if settings.KittyJournal.JitterSeconds != 600 {
		t.Fatalf("kittyJournal.jitterSeconds=%d, want 600", settings.KittyJournal.JitterSeconds)
	}
	if settings.KittyJournal.DiaryProbability != 0.1 {
		t.Fatalf("kittyJournal.diaryProbability=%v, want 0.1", settings.KittyJournal.DiaryProbability)
	}
	if settings.KittyJournal.EncounterProbability != 0.1 {
		t.Fatalf("kittyJournal.encounterProbability=%v, want 0.1", settings.KittyJournal.EncounterProbability)
	}
	if settings.KittyJournal.MaxEntriesPerKitty != 200 {
		t.Fatalf("kittyJournal.maxEntriesPerKitty=%d, want 200", settings.KittyJournal.MaxEntriesPerKitty)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "kittyJournal": {
    "intervalSeconds": 1800,
    "jitterSeconds": 600,
    "diaryProbability": 1.2,
    "encounterProbability": 0.1,
    "maxEntriesPerKitty": 200
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid kittyJournal.diaryProbability error")
	}
}

func TestLoadGlobalSettingsReleaseDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.Release.BuildID != "dev-local" {
		t.Fatalf("release.buildId=%q, want dev-local", settings.Release.BuildID)
	}
	if settings.Release.DefaultReloadDelaySeconds != 120 {
		t.Fatalf("release.defaultReloadDelaySeconds=%d, want 120", settings.Release.DefaultReloadDelaySeconds)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "release": {
    "buildId": "",
    "defaultReloadDelaySeconds": 120
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid release.buildId error")
	}
}

func TestLoadGlobalSettingsRuleDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.Rule.MaxGroups != 6 {
		t.Fatalf("rule.maxGroups=%d, want 6", settings.Rule.MaxGroups)
	}
	if settings.Rule.MaxConditionsPerGroup != 6 {
		t.Fatalf("rule.maxConditionsPerGroup=%d, want 6", settings.Rule.MaxConditionsPerGroup)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "rule": {
    "maxGroups": 0,
    "maxConditionsPerGroup": 6
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid rule.maxGroups error")
	}
}

func TestLoadGlobalSettingsSkillDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.Skill.LevelUpBaseExp != 100 {
		t.Fatalf("skill.levelUpBaseExp=%d, want 100", settings.Skill.LevelUpBaseExp)
	}
	if settings.Skill.LevelUpPerLevelExtraExp != 100 {
		t.Fatalf("skill.levelUpPerLevelExtraExp=%d, want 100", settings.Skill.LevelUpPerLevelExtraExp)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "skill": {
    "levelUpBaseExp": 0,
    "levelUpPerLevelExtraExp": 100
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid skill.levelUpBaseExp error")
	}
}

func TestLoadGlobalSettingsSkillForgeDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.SkillForge.CompilerVersion != 10 || settings.SkillForge.RulesetVersion != 1 || settings.SkillForge.BaseSuccessChance != 0.30 || settings.SkillForge.SourceSkillbookMinCount != 200 || settings.SkillForge.SourceSkillbookMaxCount != 1000 || settings.SkillForge.SourceSkillbookCountStep != 100 || settings.SkillForge.SourceSkillbookMaxBonus != 0.80 || settings.SkillForge.MaxEffects != 3 || settings.SkillForge.MaxCustomSkills != 12 || settings.SkillForge.RecentHistoryLimit != 50 || settings.SkillForge.IdempotencyRetentionDays != 30 || settings.SkillForge.MinCooldown != 1 || settings.SkillForge.MaxGraphBytes != 128*1024 || settings.SkillForge.MaxTradeoffRefundRatio != 0.35 || settings.SkillForge.StatusBaseApplyChance != 0.25 || settings.SkillForge.StatusApplyChancePerStep != 0.10 || settings.SkillForge.CustomizedNodeSealItemID != "spell_seal_lock_plate" || settings.SkillForge.CustomizedNodeSealCount != 10 || settings.SkillForge.ForgeCatalystItemID != "final_volume_key_sigil" || settings.SkillForge.ForgeCatalystCount != 5 || settings.SkillForge.GoldCostItemID != "gold_coin" || settings.SkillForge.GoldCostBase != 10000 || settings.SkillForge.GoldCostPerMaterial != 20 || settings.SkillForge.GoldCostPerBudget != 5000 {
		t.Fatalf("unexpected skill forge defaults: %#v", settings.SkillForge)
	}
	invalidPath := writeGlobalSettingsFixture(t, `{"skillForge":{"maxNodes":0}}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatal("expected invalid skill forge limits to fail")
	}
	invalidChancePath := writeGlobalSettingsFixture(t, `{"skillForge":{"baseSuccessChance":1}}`)
	if _, err := LoadGlobalSettings(invalidChancePath); err == nil {
		t.Fatal("expected invalid skill forge base success chance to fail")
	}
	invalidRetentionPath := writeGlobalSettingsFixture(t, `{"skillForge":{"recentHistoryLimit":0}}`)
	if _, err := LoadGlobalSettings(invalidRetentionPath); err == nil {
		t.Fatal("expected invalid skill forge history retention to fail")
	}
	disabledRulesetPath := writeGlobalSettingsFixture(t, `{"skillForge":{"rulesetVersion":2,"disabledRulesetVersions":[1]}}`)
	disabledSettings, err := LoadGlobalSettings(disabledRulesetPath)
	if err != nil {
		t.Fatalf("load disabled ruleset settings: %v", err)
	}
	if disabledSettings.SkillForge.RulesetEnabled(1) || !disabledSettings.SkillForge.RulesetEnabled(2) {
		t.Fatalf("unexpected ruleset policy: %#v", disabledSettings.SkillForge)
	}
	invalidRulesetPath := writeGlobalSettingsFixture(t, `{"skillForge":{"disabledRulesetVersions":[1,1]}}`)
	if _, err := LoadGlobalSettings(invalidRulesetPath); err == nil {
		t.Fatal("expected duplicate disabled ruleset version to fail")
	}
}

func TestLoadGlobalSettingsEconomyDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if settings.Economy.ItemRecycleGoldCoefficient != 0.5 {
		t.Fatalf("economy.itemRecycleGoldCoefficient=%v, want 0.5", settings.Economy.ItemRecycleGoldCoefficient)
	}
	if settings.Economy.ItemRecycleAffixMismatchExtraCoefficient != 0.6 {
		t.Fatalf("economy.itemRecycleAffixMismatchExtraCoefficient=%v, want 0.6", settings.Economy.ItemRecycleAffixMismatchExtraCoefficient)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "economy": {
    "itemRecycleGoldCoefficient": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid economy.itemRecycleGoldCoefficient error")
	}
	invalidExtraPath := writeGlobalSettingsFixture(t, `{
  "economy": {
    "itemRecycleGoldCoefficient": 0.75,
    "itemRecycleAffixMismatchExtraCoefficient": 0
  }
}`)
	if _, err := LoadGlobalSettings(invalidExtraPath); err == nil {
		t.Fatalf("expected invalid economy.itemRecycleAffixMismatchExtraCoefficient error")
	}
	invalidExtraGtOnePath := writeGlobalSettingsFixture(t, `{
  "economy": {
    "itemRecycleGoldCoefficient": 0.75,
    "itemRecycleAffixMismatchExtraCoefficient": 1.01
  }
}`)
	if _, err := LoadGlobalSettings(invalidExtraGtOnePath); err == nil {
		t.Fatalf("expected invalid economy.itemRecycleAffixMismatchExtraCoefficient > 1 error")
	}
	invalidCombinedCoefficientPath := writeGlobalSettingsFixture(t, `{
  "economy": {
    "itemRecycleGoldCoefficient": 0.75,
    "itemRecycleAffixMismatchExtraCoefficient": 0.2
  }
}`)
	if _, err := LoadGlobalSettings(invalidCombinedCoefficientPath); err == nil {
		t.Fatalf("expected invalid economy.itemRecycleGoldCoefficient + itemRecycleAffixMismatchExtraCoefficient <= 1 error")
	}
}

func TestLoadGlobalSettingsAdminBanReasonsDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if len(settings.Admin.UserBanReasons) != 2 {
		t.Fatalf("admin.userBanReasons len=%d, want 2", len(settings.Admin.UserBanReasons))
	}
	if settings.Admin.UserBanReasons[0].Key != "abnormal_trade" {
		t.Fatalf("admin.userBanReasons[0].key=%q, want abnormal_trade", settings.Admin.UserBanReasons[0].Key)
	}
	if settings.Admin.UserBanReasons[1].Key != "multi_account" {
		t.Fatalf("admin.userBanReasons[1].key=%q, want multi_account", settings.Admin.UserBanReasons[1].Key)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "admin": {
    "userBanReasons": []
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid admin.userBanReasons error")
	}
}

func TestLoadGlobalSettingsContentModerationValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{
  "contentModeration": {
    "enabled": true,
    "vocabularyJsonPath": "/tmp/lexicon.json",
    "blockMessageKey": "error.contentModeration.blocked"
  }
}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if !settings.ContentModeration.Enabled {
		t.Fatalf("expected content moderation enabled")
	}
	if settings.ContentModeration.VocabularyJSONPath != "/tmp/lexicon.json" {
		t.Fatalf("vocabularyJsonPath=%q", settings.ContentModeration.VocabularyJSONPath)
	}
	if !settings.ContentModeration.Targets.ItemCustomName {
		t.Fatalf("expected item custom name moderation target enabled by default")
	}

	invalidPath := writeGlobalSettingsFixture(t, `{
  "contentModeration": {
    "enabled": true,
    "vocabularyJsonPath": "",
    "blockMessageKey": "error.contentModeration.blocked"
  }
}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatalf("expected invalid contentModeration.vocabularyJsonPath error")
	}
}

func TestLoadGlobalSettingsCombatTrialDefaultsAndValidation(t *testing.T) {
	path := writeGlobalSettingsFixture(t, `{}`)
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if !settings.CombatTrial.Enabled || settings.CombatTrial.HistoryKeepCount != 50 || settings.CombatTrial.WorkerCount != 2 || settings.CombatTrial.MaxAttempts != 3 {
		t.Fatalf("unexpected combat trial defaults: %#v", settings.CombatTrial)
	}

	invalidPath := writeGlobalSettingsFixture(t, `{"combatTrial":{"historyKeepCount":51}}`)
	if _, err := LoadGlobalSettings(invalidPath); err == nil {
		t.Fatal("expected combatTrial.historyKeepCount validation error")
	}
	unsafeLeasePath := writeGlobalSettingsFixture(t, `{"combatTrial":{"workerLeaseSeconds":29}}`)
	if _, err := LoadGlobalSettings(unsafeLeasePath); err == nil {
		t.Fatal("expected combatTrial.workerLeaseSeconds safety validation error")
	}
}

func TestLoadGlobalSettingsFishingMutationDSL(t *testing.T) {
	settings, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, `{}`))
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if len(settings.Fishing.Mutations) != 3 {
		t.Fatalf("default fishing mutations=%d, want 3", len(settings.Fishing.Mutations))
	}
	if got := settings.Fishing.Mutations[0]; got.ID != "prismatic" || got.BadgeIcon != "lucide:gem" || got.AppearanceChance != 0.0025 || got.InheritanceChance != 0.06 || got.LifespanMultiplier != 1.8 || got.SaleValueMultiplier != 2 {
		t.Fatalf("unexpected prismatic defaults: %+v", got)
	}

	custom, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, `{"fishing":{"mutations":[{"id":" Aurora ","nameKey":"fish.aurora.name","descriptionKey":"fish.aurora.desc","visualEffect":" Aurora ","badgeIcon":" LUCIDE:STAR ","appearanceChance":0.02,"inheritanceChance":0.1,"lifespanMultiplier":2,"saleValueMultiplier":3.5,"alchemy":{"primaryValueMultiplier":1.4,"bonusAttributes":[{"attrKey":" Luck ","min":1,"max":2}]}}]}}`))
	if err != nil {
		t.Fatalf("LoadGlobalSettings custom fishing mutation failed: %v", err)
	}
	if got := custom.Fishing.Mutations[0]; got.ID != "aurora" || got.VisualEffect != "aurora" || got.BadgeIcon != "lucide:star" || got.SaleValueMultiplier != 3.5 || got.Alchemy.BonusAttributes[0].AttrKey != "luck" {
		t.Fatalf("custom mutation was not normalized: %+v", got)
	}
	invalidFixtures := []string{
		`{"fishing":{"mutations":[{"id":"same","nameKey":"a","descriptionKey":"b","visualEffect":"same","appearanceChance":0,"inheritanceChance":0,"lifespanMultiplier":1,"alchemy":{"primaryValueMultiplier":1}},{"id":"same","nameKey":"c","descriptionKey":"d","visualEffect":"same","appearanceChance":0,"inheritanceChance":0,"lifespanMultiplier":1,"alchemy":{"primaryValueMultiplier":1}}]}}`,
		`{"fishing":{"mutations":[{"id":"bad","nameKey":"a","descriptionKey":"b","visualEffect":"bad","appearanceChance":1.01,"inheritanceChance":0,"lifespanMultiplier":1,"alchemy":{"primaryValueMultiplier":1}}]}}`,
		`{"fishing":{"mutations":[{"id":"bad","nameKey":"a","descriptionKey":"b","visualEffect":"bad","appearanceChance":0,"inheritanceChance":0,"lifespanMultiplier":0.9,"alchemy":{"primaryValueMultiplier":1}}]}}`,
		`{"fishing":{"mutations":[{"id":"bad","nameKey":"a","descriptionKey":"b","visualEffect":"bad","badgeIcon":"https://bad.example/icon","appearanceChance":0,"inheritanceChance":0,"lifespanMultiplier":1,"alchemy":{"primaryValueMultiplier":1}}]}}`,
		`{"fishing":{"mutations":[{"id":"bad","nameKey":"a","descriptionKey":"b","visualEffect":"bad","appearanceChance":0,"inheritanceChance":0,"lifespanMultiplier":1,"saleValueMultiplier":0.5,"alchemy":{"primaryValueMultiplier":1}}]}}`,
		`{"fishing":{"mutations":[{"id":"bad","nameKey":"a","descriptionKey":"b","visualEffect":"bad","appearanceChance":0,"inheritanceChance":0,"lifespanMultiplier":1,"alchemy":{"primaryValueMultiplier":1,"bonusAttributes":[{"attrKey":"luck","min":2,"max":1}]}}]}}`,
	}
	for index, fixture := range invalidFixtures {
		if _, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, fixture)); err == nil {
			t.Fatalf("invalid fishing mutation fixture %d was accepted", index)
		}
	}
}

func TestProductionGlobalSettingsFishingMutations(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if len(settings.Fishing.Mutations) != 3 {
		t.Fatalf("production fishing mutations=%d, want 3", len(settings.Fishing.Mutations))
	}
	for _, mutation := range settings.Fishing.Mutations {
		if mutation.AppearanceChance <= 0 || mutation.InheritanceChance <= 0 || mutation.LifespanMultiplier <= 1 || mutation.SaleValueMultiplier <= 1 || mutation.BadgeIcon == "" || mutation.Alchemy.PrimaryValueMultiplier <= 1 {
			t.Fatalf("production mutation is missing gameplay effects: %+v", mutation)
		}
	}
}

func TestResolvePathOverrideUsesConfigDir(t *testing.T) {
	got := resolvePathOverride("../../Sensitive-lexicon-1.2/compiled/sensitive-words.json", "/repo/packages/game-config/source/global-config.json")
	want := "/repo/packages/Sensitive-lexicon-1.2/compiled/sensitive-words.json"
	if got != want {
		t.Fatalf("resolvePathOverride=%q, want %q", got, want)
	}
}

func TestLoadGlobalSettingsEmailDomainPolicy(t *testing.T) {
	settings, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, `{
  "emailDomainPolicy": {
    "enabled": true,
    "blockedDomains": [" PUGB.VIP ", "pugb.vip", ""],
    "allowedDomains": [" Trusted.PUGB.VIP "]
  }
}`))
	if err != nil {
		t.Fatalf("LoadGlobalSettings failed: %v", err)
	}
	if len(settings.EmailDomainPolicy.BlockedDomains) != 1 || settings.EmailDomainPolicy.BlockedDomains[0] != "pugb.vip" {
		t.Fatalf("blocked domains were not normalized: %#v", settings.EmailDomainPolicy.BlockedDomains)
	}
	if len(settings.EmailDomainPolicy.AllowedDomains) != 1 || settings.EmailDomainPolicy.AllowedDomains[0] != "trusted.pugb.vip" {
		t.Fatalf("allowed domains were not normalized: %#v", settings.EmailDomainPolicy.AllowedDomains)
	}

	if _, err := LoadGlobalSettings(writeGlobalSettingsFixture(t, `{"emailDomainPolicy":{"enabled":true}}`)); err == nil {
		t.Fatal("enabled email policy without blocklist was accepted")
	}
}

func TestProductionEmailDomainPolicyBlocksObservedProvider(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packages", "game-config", "source", "global-config.json")
	settings, err := LoadGlobalSettings(path)
	if err != nil {
		t.Fatalf("LoadGlobalSettings(%q) failed: %v", path, err)
	}
	if !settings.EmailDomainPolicy.Enabled {
		t.Fatal("production email domain policy is disabled")
	}
	blocklistPath := resolvePathOverride(settings.EmailDomainPolicy.BlocklistPath, path)
	raw, err := os.ReadFile(blocklistPath)
	if err != nil {
		t.Fatalf("read configured email blocklist %q: %v", blocklistPath, err)
	}
	if !strings.Contains(string(raw), "pugb.vip") {
		t.Fatalf("configured email blocklist %q does not contain pugb.vip", blocklistPath)
	}
	policy, err := emailpolicy.New(emailpolicy.Config{
		Enabled:        settings.EmailDomainPolicy.Enabled,
		BlocklistPath:  blocklistPath,
		BlockedDomains: settings.EmailDomainPolicy.BlockedDomains,
		AllowedDomains: settings.EmailDomainPolicy.AllowedDomains,
	})
	if err != nil {
		t.Fatalf("initialize configured email policy: %v", err)
	}
	if !policy.IsBlocked("player@pugb.vip") {
		t.Fatal("production policy did not block pugb.vip")
	}
}
