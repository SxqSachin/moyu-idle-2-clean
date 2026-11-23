package actionruntime

import (
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestEquipmentChangedIncludesFishingActorGearSwitch(t *testing.T) {
	summary := store.ActionFastForwardSummary{
		ActorSummaries: []store.ActionActorSummary{
			{
				ActorType:        store.ActionActorTypeKitty,
				ActorID:          73,
				EquipmentChanged: true,
			},
		},
	}
	if !EquipmentChanged(summary) {
		t.Fatal("fishing actor equipment change must trigger equipment synchronization")
	}
}

func TestEquipmentChangedIgnoresFishingProgressOnly(t *testing.T) {
	summary := store.ActionFastForwardSummary{
		FishingChanged: true,
		ActorSummaries: []store.ActionActorSummary{
			{
				ActorType: store.ActionActorTypeKitty,
				ActorID:   73,
				AttrExp:   map[string]float64{"fishing": 0.5},
			},
		},
	}
	if EquipmentChanged(summary) {
		t.Fatal("ordinary fishing progress must not trigger equipment synchronization")
	}
}

func TestEquipmentChangedIgnoresDeferredWorkLoadoutIntent(t *testing.T) {
	summary := store.ActionFastForwardSummary{FinalWorkLoadoutID: "loadout-mining"}
	if EquipmentChanged(summary) {
		t.Fatal("a deferred loadout ID alone does not mean physical equipment changed")
	}
}
