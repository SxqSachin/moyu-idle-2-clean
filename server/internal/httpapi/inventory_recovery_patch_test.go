package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func TestInventoryRecoveryPatchCompactAndVersioned(t *testing.T) {
	items := make([]store.ItemInstance, 50000)
	for i := range items {
		items[i] = store.ItemInstance{InstanceID: int64(i + 1), TemplateItemID: "iron_bracers", Quantity: 1}
	}
	meta := wsPatchMeta{Version: 7, Epoch: "gateway"}
	payload := inventoryPatchPayload(items, nil, meta, inventoryInlinePatchMaxBytes+1, false)
	compact, ok := payload.(inventoryRefreshRequiredPayload)
	if !ok || !compact.RefreshRequired || compact.Meta != meta {
		t.Fatalf("large inventory did not compact: %T", payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > 256 {
		t.Fatalf("compact message size=%d err=%v", len(raw), err)
	}
	if _, ok := inventoryPatchPayload(nil, nil, meta, 128, true).(inventoryRefreshRequiredPayload); !ok {
		t.Fatal("empty baseline must invalidate a possibly stale client")
	}
	if _, ok := inventoryPatchPayload(items[:1], nil, meta, 1024, false).(fiber.Map); !ok {
		t.Fatal("small delta should remain inline")
	}
	s := newPatchTestServer()
	s.patchEpoch = "gateway"
	first := s.pushInventoryInstancesPatch(1, items)
	again := s.pushInventoryInstancesPatch(1, items)
	if first.Version != 1 || first != again {
		t.Fatalf("unchanged terminal projection changed its version: %v %v", first, again)
	}
	empty := s.pushInventoryInstancesPatch(2, nil)
	if empty.Version != 1 || empty.Epoch == "" {
		t.Fatalf("empty terminal projection has no receipt: %+v", empty)
	}
	s.resetRealtimePatchState(1)
	rebound := s.pushInventoryInstancesPatch(1, items[:1])
	if rebound.Version != 1 || rebound.Epoch == first.Epoch || rebound.Epoch == "" {
		t.Fatal("rebound inventory reused a stale client's version namespace")
	}
}
