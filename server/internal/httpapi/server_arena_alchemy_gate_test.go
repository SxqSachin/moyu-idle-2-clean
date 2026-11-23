package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func TestArenaDefenseEligibilityPolicyRequiresRepublishForLegacySnapshots(t *testing.T) {
	currentRaw, err := json.Marshal(arenaLineupSelection{
		PlayerPosition:           "vanguard",
		FormatID:                 "standard",
		EligibilityPolicyVersion: arenaAlchemyEligibilityPolicyVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	legacyRaw, err := json.Marshal(arenaLineupSelection{PlayerPosition: "vanguard", FormatID: "standard"})
	if err != nil {
		t.Fatal(err)
	}
	rows := []store.ArenaTarget{
		{ArenaDefense: store.ArenaDefense{UserID: 1, Selection: currentRaw}},
		{ArenaDefense: store.ArenaDefense{UserID: 2, Selection: legacyRaw}},
	}

	filtered := filterArenaTargetsByEligibilityPolicy(rows)
	if len(filtered) != 1 || filtered[0].UserID != 1 {
		t.Fatalf("filtered targets=%#v want only current policy", filtered)
	}
	if arenaDefenseEligibilityPolicyCurrent(rows[1].ArenaDefense) {
		t.Fatal("legacy defense unexpectedly passed current policy")
	}
}

func TestWriteArenaAlchemyEligibilityErrorIncludesActionableViolations(t *testing.T) {
	s := &Server{
		App:  fiber.New(),
		deps: app.Dependencies{Config: config.Config{AppEnv: "production"}},
	}
	s.Get("/test", func(c *fiber.Ctx) error {
		return s.writeArenaError(c, "ARENA_TEST_FAILED", &competitiveAlchemyEligibilityError{Violations: []competitiveAlchemyEligibilityViolation{{
			UnitType:            "kitty",
			UnitID:              19,
			UnitName:            "Mimi",
			EquipmentInstanceID: 71,
			TemplateItemID:      "test_armor",
			Attribute:           "speed",
			ActualValue:         12,
			AllowedValue:        3,
		}}})
	})

	resp, err := s.Test(httptest.NewRequest("GET", "/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("status=%d want %d", resp.StatusCode, fiber.StatusConflict)
	}
	var payload struct {
		Error apiErrorBody `json:"error"`
		Data  struct {
			PolicyVersion                 string                                   `json:"policyVersion"`
			RequiredAlchemyRulesetVersion string                                   `json:"requiredAlchemyRulesetVersion"`
			Violations                    []competitiveAlchemyEligibilityViolation `json:"violations"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "ARENA_ALCHEMY_EQUIPMENT_RESTRICTED" || payload.Data.PolicyVersion != arenaAlchemyEligibilityPolicyVersion {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.Data.RequiredAlchemyRulesetVersion != store.AlchemyCurrentRulesetVersion || len(payload.Data.Violations) != 1 {
		t.Fatalf("unexpected eligibility data: %#v", payload.Data)
	}
	violation := payload.Data.Violations[0]
	if violation.UnitName != "Mimi" || violation.EquipmentInstanceID != 71 || violation.Attribute != "speed" {
		t.Fatalf("unexpected violation: %#v", violation)
	}
}

func TestArenaEquipmentSnapshotsMatchIsOrderStableAndAffixSensitive(t *testing.T) {
	first := store.EquippedItem{
		InstanceID: 2,
		EquipType:  "battle",
		Affixes: []store.EquipmentAffixSlot{{
			AffixID:  "custom:test",
			AttrMods: map[string]float64{"speed": 0.5},
		}},
	}
	second := store.EquippedItem{InstanceID: 1, EquipType: "battle"}
	if !competitiveEquipmentSnapshotsMatch([]store.EquippedItem{first, second}, []store.EquippedItem{second, first}) {
		t.Fatal("equivalent equipment snapshots must not depend on query order")
	}
	changed := first
	changed.Affixes = []store.EquipmentAffixSlot{{AffixID: "custom:test", AttrMods: map[string]float64{"speed": 5}}}
	if competitiveEquipmentSnapshotsMatch([]store.EquippedItem{first}, []store.EquippedItem{changed}) {
		t.Fatal("affix changes must invalidate the arena equipment fence")
	}
	workItem := store.EquippedItem{InstanceID: 3, EquipType: "work"}
	if !competitiveEquipmentSnapshotsMatch([]store.EquippedItem{first}, []store.EquippedItem{first, workItem}) {
		t.Fatal("work equipment must not invalidate a battle-lineup fence")
	}
}

func TestArenaLineupDigestOmitsTransientEquipmentSnapshot(t *testing.T) {
	_, raw, err := arenaLineupDigest([]battleUnitState{{
		UnitID:                 "player:1",
		UnitType:               "player",
		equipmentSnapshotItems: []store.EquippedItem{{InstanceID: 91, TemplateItemID: "secret_test_item", EquipType: "battle"}},
	}})
	if err != nil {
		t.Fatalf("arenaLineupDigest: %v", err)
	}
	if strings.Contains(string(raw), "secret_test_item") || strings.Contains(string(raw), "equipmentSnapshotItems") {
		t.Fatalf("transient equipment snapshot leaked into persisted lineup: %s", raw)
	}
}
