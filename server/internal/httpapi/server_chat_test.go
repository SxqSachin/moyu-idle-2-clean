package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func buildChatTestServer(channels []config.ChatChannel) *Server {
	return &Server{
		deps: app.Dependencies{
			Config: config.Config{
				GlobalSettings: config.GlobalSettings{
					Chat: config.ChatSettings{
						DefaultHistoryLimit: 30,
						Channels:            channels,
					},
				},
			},
		},
	}
}

func TestResolveChatChannelDefault(t *testing.T) {
	s := buildChatTestServer([]config.ChatChannel{
		{ID: "world", NameKey: "chat.channel.world", Order: 10, PlayerSendable: true},
		{ID: "system", NameKey: "chat.channel.system", Order: 20, PlayerSendable: false},
	})
	ch, ok := s.resolveChatChannel("")
	if !ok {
		t.Fatalf("resolveChatChannel should return default channel")
	}
	if ch.ID != "world" {
		t.Fatalf("default channel=%q, want world", ch.ID)
	}
	ch, ok = s.resolveChatChannel(" SYSTEM ")
	if !ok {
		t.Fatalf("resolveChatChannel should resolve system")
	}
	if ch.ID != "system" {
		t.Fatalf("resolved channel=%q, want system", ch.ID)
	}
	if ch.PlayerSendable {
		t.Fatalf("system channel should be readonly")
	}
	if _, ok = s.resolveChatChannel("unknown"); ok {
		t.Fatalf("resolveChatChannel should reject unknown channel")
	}
}

func TestParseChatHistoryLimit(t *testing.T) {
	limit, ok := parseChatHistoryLimit("", 30)
	if !ok || limit != 30 {
		t.Fatalf("empty limit => %d,%v, want 30,true", limit, ok)
	}
	limit, ok = parseChatHistoryLimit("205", 30)
	if !ok || limit != 200 {
		t.Fatalf("clamped limit => %d,%v, want 200,true", limit, ok)
	}
	if _, ok = parseChatHistoryLimit("oops", 30); ok {
		t.Fatalf("invalid limit should fail")
	}
}

func TestParseChatBeforeID(t *testing.T) {
	beforeID, ok := parseChatBeforeID("")
	if !ok || beforeID != 0 {
		t.Fatalf("empty beforeId => %d,%v, want 0,true", beforeID, ok)
	}
	beforeID, ok = parseChatBeforeID("123")
	if !ok || beforeID != 123 {
		t.Fatalf("beforeId => %d,%v, want 123,true", beforeID, ok)
	}
	if _, ok = parseChatBeforeID("-1"); ok {
		t.Fatalf("negative beforeId should fail")
	}
}

func TestBattleRoomChatChannelID(t *testing.T) {
	if got := buildBattleRoomChatChannelID(12); got != "battle_room:12" {
		t.Fatalf("buildBattleRoomChatChannelID=%q, want battle_room:12", got)
	}
	if got := buildBattleRoomChatChannelID(0); got != "" {
		t.Fatalf("buildBattleRoomChatChannelID with invalid room id should be empty, got=%q", got)
	}
	roomID, ok := parseBattleRoomChatChannelID(" battle_room:12 ")
	if !ok || roomID != 12 {
		t.Fatalf("parseBattleRoomChatChannelID => %d,%v, want 12,true", roomID, ok)
	}
	if _, ok = parseBattleRoomChatChannelID("battle_room:0"); ok {
		t.Fatalf("battle_room:0 should be rejected")
	}
	if _, ok = parseBattleRoomChatChannelID("world"); ok {
		t.Fatalf("world should not be parsed as battle room channel")
	}
}

func TestCombatTrialReplayOutcomeLabelComesFromFrozenFormat(t *testing.T) {
	format := combatTrialTestFormat()
	format.SchemaVersion = 2
	format.RulesetVersion = combatTrialFormatRulesetVersion
	format.DefenseK = format.Dummy.Defense
	format.Limits = []combatTrialLimitDefinition{{Kind: "player_main_actions", Value: 20}}
	format.Encounter = legacyCombatTrialEncounter(format.Dummy)
	format.EndConditions = []combatTrialEndConditionDefinition{
		{Kind: "player_team_defeated", Reason: "team_defeated", LabelKey: "game.combatTrial.endReason.teamDefeated"},
		{Kind: "player_main_actions_reached", Reason: "boss_timeout", LabelKey: "game.combatTrial.endReason.bossTimeout"},
	}
	format.MetricIDs = []string{"damage_total", "damage_max_segment"}
	for index := range format.Metrics {
		format.Metrics[index].SchemaVersion = 2
		format.Metrics[index].Ranked = true
		format.Metrics[index].RankDirection = "desc"
		format.Metrics[index].EligibleEndReasons = []string{"team_defeated", "boss_timeout"}
	}
	digest, err := combatTrialDefinitionDigest(*format)
	if err != nil {
		t.Fatalf("digest format: %v", err)
	}
	format.DefinitionDigest = digest
	raw, err := json.Marshal(format)
	if err != nil {
		t.Fatalf("marshal format: %v", err)
	}
	if got := combatTrialReplayOutcomeLabelKey(raw, "boss_timeout"); got != "game.combatTrial.endReason.bossTimeout" {
		t.Fatalf("frozen outcome label = %q", got)
	}
	if got := combatTrialReplayOutcomeLabelKey(raw, "unknown"); got != "" {
		t.Fatalf("unknown outcome label = %q", got)
	}
}

func TestResolveChatCatalogAttachments(t *testing.T) {
	s := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Items: []catalog.Item{
			{ID: "iron_ore", Type: []string{"material"}},
			{ID: "disabled_item", Type: []string{"material"}, Disabled: true},
		},
		Actions: []catalog.ActionDefinition{
			{ID: "mining.iron", MainType: "mining"},
			{ID: "alchemy.convert", MainType: "alchemy"},
		},
	}}}

	itemAttachment, err := s.resolveChatAttachment(context.Background(), 7, map[string]any{
		"type":    "item",
		"itemId":  " iron_ore ",
		"preview": map[string]any{"clientControlled": true},
	})
	if err != nil {
		t.Fatalf("resolve item attachment: %v", err)
	}
	if itemAttachment["type"] != chatAttachmentItem || itemAttachment["itemId"] != "iron_ore" || len(itemAttachment) != 2 {
		t.Fatalf("item attachment=%#v", itemAttachment)
	}

	actionAttachment, err := s.resolveChatAttachment(context.Background(), 7, map[string]any{
		"type":     "action",
		"actionId": "mining.iron",
	})
	if err != nil {
		t.Fatalf("resolve action attachment: %v", err)
	}
	if actionAttachment["type"] != chatAttachmentAction || actionAttachment["actionId"] != "mining.iron" || len(actionAttachment) != 2 {
		t.Fatalf("action attachment=%#v", actionAttachment)
	}

	for _, raw := range []map[string]any{
		{"type": "item", "itemId": "missing"},
		{"type": "item", "itemId": "disabled_item"},
		{"type": "item", "itemId": map[string]any{"id": "iron_ore"}},
		{"type": "item", "itemId": strings.Repeat("x", chatAttachmentIdentifierMaxBytes+1)},
		{"type": "action", "actionId": "missing"},
		{"type": "action", "actionId": "alchemy.convert"},
	} {
		if _, err := s.resolveChatAttachment(context.Background(), 7, raw); !errors.Is(err, errChatAttachmentInvalid) {
			t.Fatalf("attachment %#v error=%v, want errChatAttachmentInvalid", raw, err)
		}
	}
}

func TestBuildChatItemInstanceAttachmentUsesServerSnapshot(t *testing.T) {
	instance := store.ItemInstance{
		InstanceID:     42,
		UserID:         7,
		TemplateItemID: "iron_sword",
		CustomData:     store.ItemCustomization{Name: "Northwind", LabelColor: "sky"},
		Quantity:       1,
		EnhanceLevel:   6,
		BaseSnapshot:   map[string]any{"attack": float64(18)},
		Affixes: []store.EquipmentAffixSlot{{
			SlotIndex: 0, AffixID: "sharp", RarityID: "rare", SourceKind: "craft",
			AlchemyRulesetVersion: "private-ruleset", AttrMods: map[string]float64{"physics_attack": 4},
			EffectParams: map[string]any{"privateAncestry": []any{1, 2, 3}},
		}},
		Durability:    81,
		MaxDurability: 100,
		State:         "inventory",
	}
	attachment, err := (&Server{}).buildChatItemInstanceAttachment(instance)
	if err != nil {
		t.Fatalf("build instance attachment: %v", err)
	}
	if attachment["type"] != chatAttachmentItemInstance || attachment["itemId"] != "iron_sword" {
		t.Fatalf("instance attachment=%#v", attachment)
	}
	preview, ok := attachment["preview"].(map[string]any)
	if !ok {
		t.Fatalf("preview=%T, want map", attachment["preview"])
	}
	if preview["enhanceLevel"] != 6 {
		t.Fatalf("preview=%#v", preview)
	}
	customData, ok := preview["customData"].(map[string]any)
	if !ok || customData["name"] != "Northwind" || customData["labelColor"] != "sky" {
		t.Fatalf("customData=%#v", preview["customData"])
	}
	encoded, err := json.Marshal(attachment)
	if err != nil {
		t.Fatalf("marshal attachment: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal attachment: %v", err)
	}
	for _, forbidden := range []string{"userId", "instanceId", "quantity", "durability", "maxDurability", "alchemyRulesetVersion", "effectParams", "privateAncestry"} {
		if strings.Contains(string(encoded), `"`+forbidden+`"`) {
			t.Fatalf("attachment leaked private field %q: %s", forbidden, encoded)
		}
	}
}

func TestPublicChatAttachmentRedactsLegacyInstanceFields(t *testing.T) {
	attachment := publicChatAttachment(map[string]any{
		"type":       chatAttachmentItemInstance,
		"itemId":     "iron_sword",
		"instanceId": float64(42),
		"quantity":   float64(9),
		"preview": map[string]any{
			"enhanceLevel":  float64(6),
			"durability":    float64(81),
			"maxDurability": float64(100),
			"customData":    map[string]any{"name": "Northwind", "labelColor": "sky"},
		},
	})
	encoded, err := json.Marshal(attachment)
	if err != nil {
		t.Fatalf("marshal public attachment: %v", err)
	}
	for _, forbidden := range []string{"instanceId", "quantity", "durability", "maxDurability"} {
		if strings.Contains(string(encoded), `"`+forbidden+`"`) {
			t.Fatalf("legacy attachment leaked %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"labelColor":"sky"`) {
		t.Fatalf("public attachment lost customization: %s", encoded)
	}
}

func TestBuildChatFishInstancePreviewOmitsPrivateState(t *testing.T) {
	s := &Server{deps: app.Dependencies{Config: config.Config{GlobalSettings: config.GlobalSettings{
		Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{{ID: "prismatic"}}},
	}}}}
	privateTime := time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)
	preview := s.buildChatFishInstancePreview(store.FishInstanceData{
		SpeciesID: "common_carp", AgeDays: 60, Sex: "female", LengthCM: 41.2, WeightKG: 1.8,
		Condition: 0.9, Health: 0.8, WaterID: "sunmere_shallows", SpotID: "old_willow", CaughtAt: privateTime,
		RulesetVersion: "private-ruleset", ParentInstanceIDs: []int64{12, 13}, PreferredFeedHours: 42,
		Genes: store.FishGenes{GrowthRate: 1.25, Mutations: []string{"prismatic", "unconfigured"}},
	})
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal fish preview: %v", err)
	}
	payload := string(encoded)
	for _, forbidden := range []string{"rulesetVersion", "parentInstanceIds", "preferredFeedHours", "growthRate", "unconfigured"} {
		if strings.Contains(payload, `"`+forbidden+`"`) {
			t.Fatalf("attachment leaked private fish field %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(payload, `"mutations":["prismatic"]`) {
		t.Fatalf("fish preview lost visible mutation: %s", encoded)
	}
}

func TestBuildChatItemInstanceAttachmentRejectsUnavailableInventory(t *testing.T) {
	now := time.Now()
	for _, instance := range []store.ItemInstance{
		{InstanceID: 1, TemplateItemID: "iron_sword", Quantity: 1, State: "market"},
		{InstanceID: 2, TemplateItemID: "iron_sword", Quantity: 0, State: "inventory"},
		{InstanceID: 3, TemplateItemID: "iron_sword", Quantity: 1, State: "inventory", ArchivedAt: &now},
	} {
		if _, err := (&Server{}).buildChatItemInstanceAttachment(instance); !errors.Is(err, errChatItemInstanceUnavailable) {
			t.Fatalf("instance %#v error=%v, want unavailable", instance, err)
		}
	}
}

func TestParsePositiveChatAttachmentID(t *testing.T) {
	for _, raw := range []any{1, int64(2), float64(3), json.Number("4"), "5"} {
		if id, ok := parsePositiveChatAttachmentID(raw); !ok || id <= 0 {
			t.Fatalf("parse %v => %d,%v", raw, id, ok)
		}
	}
	for _, raw := range []any{nil, 0, -1, 1.5, "1.5", ""} {
		if id, ok := parsePositiveChatAttachmentID(raw); ok || id != 0 {
			t.Fatalf("invalid parse %v => %d,%v", raw, id, ok)
		}
	}
}

func TestParseChatAttachmentTextIsStrictAndBounded(t *testing.T) {
	if value, ok := parseChatAttachmentText("  iron_ore  ", 32); !ok || value != "iron_ore" {
		t.Fatalf("valid attachment text => %q,%v", value, ok)
	}
	for _, raw := range []any{nil, 7, true, []string{"iron_ore"}, map[string]any{"id": "iron_ore"}, "", strings.Repeat("x", 33)} {
		if value, ok := parseChatAttachmentText(raw, 32); ok || value != "" {
			t.Fatalf("invalid attachment text %#v => %q,%v", raw, value, ok)
		}
	}
}
