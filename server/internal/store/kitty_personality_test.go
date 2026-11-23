package store

import "testing"

func TestResolveKittyPositionLockUsesSnapshotPersonalityOrder(t *testing.T) {
	st := &Store{config: RuntimeConfig{KittyPersonalityProfiles: map[string]KittyPersonalityProfile{
		"front": {
			ID: "front", NameKey: "kitty.front", Effects: []KittyPersonalityEffect{{Kind: "battle_position_lock", Position: "vanguard"}},
		},
		"rear": {
			ID: "rear", NameKey: "kitty.rear", Effects: []KittyPersonalityEffect{{Kind: "battle_position_lock", Position: "rearguard"}},
		},
	}}}
	position, personalityID, personalityKey := st.ResolveKittyPositionLock(map[string]any{
		kittyPersonalityIDsExtraKey: []string{"rear", "front"},
	})
	if position != "rearguard" || personalityID != "rear" || personalityKey != "kitty.rear" {
		t.Fatalf("resolved lock=(%q,%q,%q)", position, personalityID, personalityKey)
	}
}

func TestResolveKittyPositionLockIgnoresUnknownAndUnrelatedEffects(t *testing.T) {
	st := &Store{config: RuntimeConfig{KittyPersonalityProfiles: map[string]KittyPersonalityProfile{
		"worker": {
			ID: "worker", Effects: []KittyPersonalityEffect{{Kind: "attr_mod", Position: "vanguard"}},
		},
	}}}
	position, personalityID, personalityKey := st.ResolveKittyPositionLock(map[string]any{
		kittyPersonalityIDsExtraKey: []string{"unknown", "worker"},
	})
	if position != "" || personalityID != "" || personalityKey != "" {
		t.Fatalf("unexpected lock=(%q,%q,%q)", position, personalityID, personalityKey)
	}
}
