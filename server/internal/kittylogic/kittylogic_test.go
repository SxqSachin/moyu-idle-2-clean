package kittylogic

import (
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestRewardPayloadDoesNotExposePool(t *testing.T) {
	payload := RewardPayload(RewardResult{
		PoolID: "secret_pool",
		Items:  []store.KittyRewardGrantSpec{{ItemID: "kitty_affection", Quantity: 2}},
	})
	if _, exists := payload["poolId"]; exists {
		t.Fatalf("RewardPayload exposed poolId: %#v", payload)
	}
	if _, exists := payload["chance"]; exists {
		t.Fatalf("RewardPayload exposed chance: %#v", payload)
	}
}

func TestSanitizeRewardPayloadRemovesPrivateFields(t *testing.T) {
	payload := SanitizeRewardPayload(map[string]any{
		"poolId":      "old_pool",
		"chance":      0.005,
		"probability": 0.005,
		"rolls":       []any{"hidden"},
		"entries":     []any{"hidden"},
		"items":       []any{map[string]any{"itemId": "cute_hat", "quantity": 1}},
	})
	for _, key := range []string{"poolId", "chance", "probability", "rolls", "entries"} {
		if _, exists := payload[key]; exists {
			t.Fatalf("SanitizeRewardPayload kept private key %q in %#v", key, payload)
		}
	}
	if _, exists := payload["items"]; !exists {
		t.Fatalf("SanitizeRewardPayload removed visible reward result: %#v", payload)
	}
}
