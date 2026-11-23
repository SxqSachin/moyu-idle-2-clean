package httpapi

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestBattleRuntimeEmitRetainsRecentEventsWithoutReplacingBuffer(t *testing.T) {
	events := make([]battleEvent, battleRecentEventLimit, battleRecentEventLimit+1)
	for index := range events {
		events[index] = battleEvent{Seq: int64(index + 1), Type: fmt.Sprintf("event-%d", index+1)}
	}
	state := battleRuntimeState{Seq: battleRecentEventLimit, Events: events}
	bufferStart := &state.Events[0]

	for index := 0; index < 50; index++ {
		state.emit("new", time.UnixMilli(int64(index+1)), nil)
	}

	if len(state.Events) != battleRecentEventLimit {
		t.Fatalf("events len=%d, want %d", len(state.Events), battleRecentEventLimit)
	}
	if &state.Events[0] != bufferStart {
		t.Fatal("bounded event window replaced its backing buffer")
	}
	if got, want := state.Events[0].Seq, int64(51); got != want {
		t.Fatalf("first retained seq=%d, want %d", got, want)
	}
	if got, want := state.Events[len(state.Events)-1].Seq, int64(250); got != want {
		t.Fatalf("last retained seq=%d, want %d", got, want)
	}
}

func TestBattleRuntimeEmitCanRetainCompleteCompiledReplay(t *testing.T) {
	state := battleRuntimeState{RetainCompleteEventHistory: true}
	for index := 0; index < battleRecentEventLimit+25; index++ {
		state.emit("compiled", time.UnixMilli(int64(index+1)), nil)
	}

	if got, want := len(state.Events), battleRecentEventLimit+25; got != want {
		t.Fatalf("compiled replay events len=%d, want %d", got, want)
	}
	if state.Events[0].Seq != 1 || state.Events[len(state.Events)-1].Seq != int64(battleRecentEventLimit+25) {
		t.Fatalf("compiled replay lost timeline boundaries: first=%d last=%d", state.Events[0].Seq, state.Events[len(state.Events)-1].Seq)
	}
}

func TestBattleUnitCombinedRuntimeViewsPreserveSourceOrder(t *testing.T) {
	unit := battleUnitState{
		EquipmentHooks:                  []battleEquipmentHookRule{{ID: "equipment-1"}, {ID: "equipment-2"}},
		KittyPersonalityHooks:           []battleEquipmentHookRule{{ID: "personality-1"}},
		EffectMutations:                 []battleAffixEffectMutation{{Scope: "equipment-1"}, {Scope: "equipment-2"}},
		KittyPersonalityEffectMutations: []battleAffixEffectMutation{{Scope: "personality-1"}},
	}

	if got := unit.equipmentHookCount(); got != 3 {
		t.Fatalf("equipment hook count=%d, want 3", got)
	}
	for index, wantID := range []string{"equipment-1", "equipment-2", "personality-1"} {
		hook, ok := unit.equipmentHookAt(index)
		if !ok || hook.ID != wantID {
			t.Fatalf("equipment hook[%d]=%q,%t, want %q,true", index, hook.ID, ok, wantID)
		}
	}
	if _, ok := unit.equipmentHookAt(3); ok {
		t.Fatal("out-of-range equipment hook unexpectedly resolved")
	}

	if got := unit.effectMutationCount(); got != 3 {
		t.Fatalf("effect mutation count=%d, want 3", got)
	}
	for index, wantScope := range []string{"equipment-1", "equipment-2", "personality-1"} {
		mutation, ok := unit.effectMutationAt(index)
		if !ok || mutation.Scope != wantScope {
			t.Fatalf("effect mutation[%d]=%q,%t, want %q,true", index, mutation.Scope, ok, wantScope)
		}
	}
	if _, ok := unit.effectMutationAt(3); ok {
		t.Fatal("out-of-range effect mutation unexpectedly resolved")
	}
}

func BenchmarkBattleRuntimeEmitAtRecentEventLimit(b *testing.B) {
	state := battleRuntimeState{
		Events: make([]battleEvent, battleRecentEventLimit, battleRecentEventLimit+1),
		Seq:    battleRecentEventLimit,
	}
	now := time.UnixMilli(1_000)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		state.emit("battle.turn.action", now, nil)
	}
	runtime.KeepAlive(state)
}

func BenchmarkBattleUnitCombinedRuntimeViews(b *testing.B) {
	unit := battleUnitState{
		EquipmentHooks:                  make([]battleEquipmentHookRule, 8),
		KittyPersonalityHooks:           make([]battleEquipmentHookRule, 4),
		EffectMutations:                 make([]battleAffixEffectMutation, 8),
		KittyPersonalityEffectMutations: make([]battleAffixEffectMutation, 4),
	}
	visited := 0
	b.ReportAllocs()
	b.ResetTimer()
	for run := 0; run < b.N; run++ {
		for index := 0; index < unit.equipmentHookCount(); index++ {
			if _, ok := unit.equipmentHookAt(index); ok {
				visited++
			}
		}
		for index := 0; index < unit.effectMutationCount(); index++ {
			if _, ok := unit.effectMutationAt(index); ok {
				visited++
			}
		}
	}
	runtime.KeepAlive(visited)
}
