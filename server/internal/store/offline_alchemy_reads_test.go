package store

import (
	"context"
	"testing"
	"time"
)

func TestOfflineAlchemyReadsScope(t *testing.T) {
	r := &offlineAlchemyReads{}
	item := QueueRuntimeItem{QueueID: 7, ActorType: ActionActorTypePlayer, ActionID: "alchemy.convert"}
	if r.forItem(item) != r {
		t.Fatal("eligible lane did not opt in")
	}
	r.derived, r.duration = &alchemyDerived{Level: 8}, time.Second
	if r.forItem(item).derived == nil {
		t.Fatal("unchanged lane lost reads")
	}
	for _, changed := range []QueueRuntimeItem{
		{QueueID: 8, ActorType: ActionActorTypePlayer, ActionID: "alchemy.convert"},
		{QueueID: 7, ActorType: ActionActorTypeKitty, ActionID: "alchemy.convert"},
		{QueueID: 7, ActorType: ActionActorTypePlayer, ActionID: "alchemy.bench"},
		{QueueID: 7, ActorType: ActionActorTypePlayer, ActionID: "alchemy.transmute", PayloadRaw: []byte(`{"runContext":{"args":{"itemId":"wood","instanceId":4}}}`)},
		{QueueID: 7, ActorType: ActionActorTypePlayer, ActionID: "alchemy.transmute", PayloadRaw: []byte(`invalid`)},
	} {
		r.forItem(item)
		r.derived, r.duration = &alchemyDerived{Level: 8}, time.Second
		r.forItem(changed)
		if r.derived != nil || r.duration != 0 {
			t.Fatalf("reads survived a lane/side-effect boundary: %+v", changed)
		}
	}
	r.derived, r.duration = &alchemyDerived{Level: 8}, time.Second
	r.invalidate()
	if r.derived != nil || r.duration != 0 {
		t.Fatal("level boundary retained reads")
	}
}

func TestOfflineAlchemyReadsFrozenLoadoutPrecedence(t *testing.T) {
	s := &Store{}
	item := QueueRuntimeItem{alchemyReads: &offlineAlchemyReads{derived: &alchemyDerived{Level: 8}},
		WorkLoadoutRuntime: &WorkLoadoutRuntimeResolution{Available: true, AttributeTotals: map[string]float64{"alchemy_level": 19}}}
	got, err := s.resolveAlchemyDerivedForItemTx(context.Background(), nil, 1, item)
	if err != nil || got.Level != 19 {
		t.Fatal("live reads replaced the frozen work loadout", got, err)
	}
	item.WorkLoadoutRuntime.AttributeTotals["alchemy_level"] = 21
	got, err = s.resolveAlchemyDerivedForItemTx(context.Background(), nil, 1, item)
	if err != nil || got.Level != 21 {
		t.Fatal("frozen loadout was cached across mutations", got, err)
	}
}
