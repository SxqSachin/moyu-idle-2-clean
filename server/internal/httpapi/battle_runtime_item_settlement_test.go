package httpapi

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/store"
)

type retryingBattleItemBagWriter struct {
	failNext bool
	calls    int
	values   map[string]int64
}

func (w *retryingBattleItemBagWriter) SetBattleItemBagQuantities(_ context.Context, updates []store.BattleItemBagQuantityUpdate) error {
	w.calls++
	if w.failNext {
		w.failNext = false
		return errors.New("injected atomic batch failure")
	}
	if w.values == nil {
		w.values = map[string]int64{}
	}
	for _, update := range updates {
		key := strings.Join([]string{strconv.FormatInt(update.UserID, 10), update.ItemID}, ":")
		w.values[key] = update.Quantity
	}
	return nil
}

func TestBattleItemBagSettlementRetryUsesAbsoluteAtomicSnapshot(t *testing.T) {
	state := battleRuntimeState{Players: []battleUnitState{
		{UserID: 101, BattleItems: []battleItemState{{ItemID: "mirror_house_reflection_candy", Quantity: 0}, {ItemID: "expired_candy_salute", Quantity: 2}}},
		{UserID: 202, BattleItems: []battleItemState{{ItemID: "plush_stunt_double_box", Quantity: 1}}},
	}}
	finalResult := map[string]any{"carryState": battleCarryStatePayload(state, "ended")}
	writer := &retryingBattleItemBagWriter{failNext: true}

	firstStats, err := settleBattleItemBagFromFinalResultWithWriter(context.Background(), writer, finalResult)
	if err == nil {
		t.Fatal("expected the injected first batch failure")
	}
	if firstStats.RowsUpdated != 0 || len(writer.values) != 0 {
		t.Fatalf("failed atomic batch must not report or retain partial rows: stats=%+v values=%v", firstStats, writer.values)
	}

	secondStats, err := settleBattleItemBagFromFinalResultWithWriter(context.Background(), writer, finalResult)
	if err != nil {
		t.Fatalf("retry settlement: %v", err)
	}
	if secondStats.RowsUpdated != 3 || writer.calls != 2 {
		t.Fatalf("unexpected retry stats/calls: stats=%+v calls=%d", secondStats, writer.calls)
	}
	want := map[string]int64{
		"101:mirror_house_reflection_candy": 0,
		"101:expired_candy_salute":          2,
		"202:plush_stunt_double_box":        1,
	}
	for key, quantity := range want {
		if writer.values[key] != quantity {
			t.Fatalf("settled quantity %s=%d, want %d", key, writer.values[key], quantity)
		}
	}

	if _, err := settleBattleItemBagFromFinalResultWithWriter(context.Background(), writer, finalResult); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	for key, quantity := range want {
		if writer.values[key] != quantity {
			t.Fatalf("replay changed absolute quantity %s=%d, want %d", key, writer.values[key], quantity)
		}
	}
}

func TestBattleItemBagSettlementIsPassedIntoRunFencedFinalization(t *testing.T) {
	raw, err := os.ReadFile("battle_runtime_finalize.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Contains(source, "settleBattleItemBagFromFinalResult(ctx, finalResult)") {
		t.Fatal("battle item snapshot must not commit before the room/run fence")
	}
	for _, expected := range []string{
		"battleItemBagUpdatesFromFinalResult(finalResult)",
		"CompleteBattleRunAndMaybeContinue(ctx, detail.Room.ID, run.ID, rewards, itemBagUpdates",
		"HandleBattleRunFailure(ctx, detail.Room.ID, run.ID, rewards, itemBagUpdates",
		"FinishBattleRoom(ctx, detail.Room.ID, run.ID, outcome, rewards, itemBagUpdates",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("run-fenced item settlement missing %q", expected)
		}
	}
}
