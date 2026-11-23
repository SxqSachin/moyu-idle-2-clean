package httpapi

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func newPatchTestServer() *Server {
	return &Server{
		patchVersion:      make(map[string]int64),
		inventorySnap:     make(map[int64]map[int64]string),
		equipmentSnap:     make(map[int64]map[int64]string),
		attrSnap:          make(map[int64]attributePatchSnapshot),
		roleEquipmentSnap: make(map[string]map[int64]string),
		roleAttrSnap:      make(map[string]attributePatchSnapshot),
		patchUpdatedAt:    make(map[int64]time.Time),
	}
}

func TestRoleAttributePatchCacheIsIsolatedByKitty(t *testing.T) {
	s := newPatchTestServer()

	attrA := fiber.Map{"intelligence": fiber.Map{"value": 100}}
	attrB := fiber.Map{"intelligence": fiber.Map{"value": 5}}
	progress := fiber.Map{"intelligence": fiber.Map{"value": 100, "level": 1, "exp": 0, "nextExp": 100}}

	s.pushKittyAttributesPatch(1, 11, attrA, progress)
	s.pushKittyAttributesPatch(1, 12, attrB, progress)
	s.pushKittyAttributesPatch(1, 11, attrA, progress)

	if got := s.patchVersion["1:kitty.attributes.updated:kitty:11"]; got != 1 {
		t.Fatalf("kitty 11 version=%d, want 1", got)
	}
	if got := s.patchVersion["1:kitty.attributes.updated:kitty:12"]; got != 1 {
		t.Fatalf("kitty 12 version=%d, want 1", got)
	}
	if _, ok := s.roleAttrSnap["1:kitty:11"]; !ok {
		t.Fatalf("missing kitty 11 attr snapshot")
	}
	if _, ok := s.roleAttrSnap["1:kitty:12"]; !ok {
		t.Fatalf("missing kitty 12 attr snapshot")
	}
}

func TestRoleEquipmentPatchCacheIsIsolatedByRole(t *testing.T) {
	s := newPatchTestServer()

	playerItems := []store.EquippedItem{{InstanceID: 1, TemplateItemID: "wood_sword", EquipType: "battle", EquippedSlot: "right_hand"}}
	kittyItems := []store.EquippedItem{{InstanceID: 2, TemplateItemID: "wood_staff", EquipType: "battle", EquippedSlot: "right_hand"}}

	s.pushEquipmentInstancesPatch(1, playerItems)
	s.pushKittyEquipmentPatch(1, 11, kittyItems)
	s.pushKittyEquipmentPatch(1, 11, kittyItems)

	if got := s.patchVersion["1:equipment.instances.updated:player:self"]; got != 1 {
		t.Fatalf("player equipment version=%d, want 1", got)
	}
	if got := s.patchVersion["1:kitty.equipment.updated:kitty:11"]; got != 1 {
		t.Fatalf("kitty equipment version=%d, want 1", got)
	}
	if _, ok := s.roleEquipmentSnap["1:player:self"]; !ok {
		t.Fatalf("missing player equipment snapshot")
	}
	if _, ok := s.roleEquipmentSnap["1:kitty:11"]; !ok {
		t.Fatalf("missing kitty equipment snapshot")
	}
}

func TestPlayerEquipmentSnapshotRebasesPatchVersionAndRepresentsEmptyState(t *testing.T) {
	s := newPatchTestServer()
	s.patchVersion["1:equipment.instances.updated:player:self"] = 8
	s.resetRealtimePatchState(1)

	payload := s.pushEquipmentInstancesSnapshot(1, nil)
	items, ok := payload["items"].([]store.EquippedItem)
	if !ok {
		t.Fatalf("snapshot items type=%T, want []store.EquippedItem", payload["items"])
	}
	if items == nil {
		t.Fatal("snapshot items is nil, want an explicit empty list")
	}
	if len(items) != 0 {
		t.Fatalf("snapshot items=%v, want empty", items)
	}
	meta, ok := payload["meta"].(wsPatchMeta)
	if !ok {
		t.Fatalf("snapshot meta type=%T, want wsPatchMeta", payload["meta"])
	}
	if meta.Version != 1 {
		t.Fatalf("snapshot version=%d, want 1", meta.Version)
	}
	if snapshot, ok := s.roleEquipmentSnap["1:player:self"]; !ok || len(snapshot) != 0 {
		t.Fatalf("player equipment cache=%v, exists=%v, want exact empty snapshot", snapshot, ok)
	}

	item := store.EquippedItem{InstanceID: 9, TemplateItemID: "wood_sword", EquipType: "battle", EquippedSlot: "right_hand"}
	patch, changed := s.pushEquipmentInstancesPatch(1, []store.EquippedItem{item})
	if !changed {
		t.Fatal("equipment patch after snapshot was not emitted")
	}
	patchMeta, ok := patch["meta"].(wsPatchMeta)
	if !ok || patchMeta.Version != 2 {
		t.Fatalf("patch meta=%v, want version 2", patch["meta"])
	}
}

func TestKittyEquipmentSnapshotRepresentsColdCacheUnequip(t *testing.T) {
	s := newPatchTestServer()

	payload := s.pushKittyEquipmentSnapshot(1, 11, nil)
	items, ok := payload["items"].([]store.EquippedItem)
	if !ok {
		t.Fatalf("snapshot items type=%T, want []store.EquippedItem", payload["items"])
	}
	if items == nil {
		t.Fatal("snapshot items is nil, want an explicit empty list")
	}
	if len(items) != 0 {
		t.Fatalf("snapshot items=%v, want empty", items)
	}
	if payload["roleType"] != "kitty" || payload["roleId"] != int64(11) {
		t.Fatalf("snapshot role=(%v,%v), want (kitty,11)", payload["roleType"], payload["roleId"])
	}
	meta, ok := payload["meta"].(wsPatchMeta)
	if !ok || meta.Version != 1 {
		t.Fatalf("snapshot meta=%v, want version 1", payload["meta"])
	}
	if snapshot, ok := s.roleEquipmentSnap["1:kitty:11"]; !ok || len(snapshot) != 0 {
		t.Fatalf("kitty equipment cache=%v, exists=%v, want exact empty snapshot", snapshot, ok)
	}
}
