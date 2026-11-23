package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSetBattleItemBagQuantitiesValidatesWholeBatchBeforeTransaction(t *testing.T) {
	s := &Store{}
	tests := []struct {
		name    string
		updates []BattleItemBagQuantityUpdate
		wantErr error
	}{
		{name: "empty item", updates: []BattleItemBagQuantityUpdate{{UserID: 1, ItemID: "  ", Quantity: 1}}, wantErr: ErrInvalidItemID},
		{name: "invalid user", updates: []BattleItemBagQuantityUpdate{{UserID: 0, ItemID: "candy_bomb", Quantity: 1}}, wantErr: ErrInvalidItemID},
		{name: "negative quantity", updates: []BattleItemBagQuantityUpdate{{UserID: 1, ItemID: "candy_bomb", Quantity: -1}}, wantErr: ErrBadQuantity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.SetBattleItemBagQuantities(context.Background(), tc.updates); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestBattleFinalizationFencesRunBeforeItemSnapshot(t *testing.T) {
	raw, err := os.ReadFile("store_battle_room.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, functionName := range []string{
		"CompleteBattleRunAndMaybeContinue",
		"HandleBattleRunFailure",
		"FinishBattleRoom",
	} {
		start := strings.Index(source, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s missing", functionName)
		}
		body := source[start:]
		if next := strings.Index(body[1:], "\nfunc (s *Store)"); next >= 0 {
			body = body[:next+1]
		}
		roomLockAt := strings.Index(body, "FOR UPDATE")
		runFenceAt := strings.Index(body, "*currentRunID != expectedRunID")
		itemSnapshotAt := strings.Index(body, "setBattleItemBagQuantitiesTx")
		if roomLockAt < 0 || runFenceAt <= roomLockAt || itemSnapshotAt <= runFenceAt {
			t.Fatalf("%s must lock room, fence expected run, then write the item snapshot", functionName)
		}
		for _, required := range []string{"AND room_id = $4", "AND status IN ($5, $6)"} {
			if !strings.Contains(body, required) {
				t.Fatalf("%s run update missing %q", functionName, required)
			}
		}
	}
}
