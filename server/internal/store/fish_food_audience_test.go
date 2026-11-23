package store

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
)

func TestTimedFishFoodRejectsSharedTargetsIncludingSavedItems(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, recipe := range s.gameData.FishFoodRecipes {
		current, ok, err := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, nil)
		if err != nil || !ok {
			t.Fatal(err)
		}
		if current.Lifetime.Kind != operationeffect.LifetimeDuration {
			continue
		}
		t.Run(recipe.ID, func(t *testing.T) {
			selected := recipeTestFish(s, recipe, 1, 1.25, []string{"shimmering", "prismatic", "rainbow"})
			custom, err := newFishFoodExecutor(s, recipe).buildSpecificOutputSnapshot(selected)
			if err != nil {
				t.Fatal(err)
			}
			snapshots := []map[string]any{nil, custom}
			for _, id := range append(append([]string{}, recipe.LegacyEffectIDs...), recipe.ReplacedEffectIDs...) {
				old := s.operationEffectDefinitions[id]
				copy := map[string]any{}
				for key, value := range custom {
					copy[key] = value
				}
				copy["operationEffect"] = operationEffectItemSnapshot{SchemaVersion: 2, DefinitionID: old.ID, RulesetVersion: old.RulesetVersion, Params: old.Params, Lifetime: &old.Lifetime}
				snapshots = append(snapshots, copy)
			}
			for _, snapshot := range snapshots {
				raw, _ := json.Marshal(snapshot)
				def, ok, err := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, raw)
				if err != nil || !ok {
					t.Fatal("saved food resolution", err)
				}
				for _, kind := range []operationeffect.TargetKind{operationeffect.TargetAllKitties, operationeffect.TargetPlayerAndKitties} {
					if err := operationeffect.ValidateTarget(def, operationeffect.Target{Kind: kind}); err == nil {
						t.Fatal("shared target accepted", kind)
					}
				}
				for _, target := range []operationeffect.Target{{Kind: operationeffect.TargetPlayer}, {Kind: operationeffect.TargetKitty, ActorID: 7}} {
					if err := operationeffect.ValidateTarget(def, target); err != nil {
						t.Fatal("individual target rejected", err)
					}
				}
				if err := operationeffect.ValidateTarget(def, operationeffect.Target{Kind: operationeffect.TargetKitty}); err == nil {
					t.Fatal("missing kitty ID accepted")
				}
			}
		})
	}
}

func TestFishCuisinePostgresGrandfatheredSoupCannotBeExtended(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	current := s.operationEffectDefinitions["food_gentle_current_soup_v2"]
	current.Lifetime.DurationSeconds = 10
	old := current
	old.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}
	oldRaw, _ := json.Marshal(old)
	shared := operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}
	// Seed the exact persisted definition from before the recipient change.
	if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "gentle_current_soup", old, shared, oldRaw, at); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(current)
	for _, kind := range []operationeffect.TargetKind{operationeffect.TargetAllKitties, operationeffect.TargetPlayerAndKitties} {
		if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "gentle_current_soup", current, operationeffect.Target{Kind: kind}, raw, at.Add(5*time.Second)); !errors.Is(err, ErrOperationEffectTargetInvalid) {
			t.Fatal("new shared activation accepted", err)
		}
	}
	var until time.Time
	if err := tx.QueryRow(ctx, `SELECT valid_until FROM player_operation_effect_grants`).Scan(&until); err != nil || !until.Equal(at.Add(10*time.Second)) {
		t.Fatal("old expiry extended", until, err)
	}
	player := operationeffect.Target{Kind: operationeffect.TargetPlayer}
	if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "gentle_current_soup", current, player, raw, at.Add(5*time.Second)); !errors.Is(err, ErrOperationEffectStrengthConflict) {
		t.Fatal("overlapping soup stacked", err)
	}
	if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "gentle_current_soup", current, player, raw, until); err != nil {
		t.Fatal("expired legacy soup blocks a new individual serving", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue VALUES(1,41,'running',$1,$2)`, at.Add(8*time.Second), at.Add(9*time.Second)); err != nil {
		t.Fatal(err)
	}
	session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, at.Add(time.Hour))
	if err != nil || session == nil {
		t.Fatal("historical session", err)
	}
	for i, tc := range []struct {
		actor operationeffect.ActorKind
		id    int64
		start time.Time
		want  int
	}{
		{operationeffect.ActorKitty, 7, until.Add(-time.Second), 1},
		{operationeffect.ActorPlayer, 0, until.Add(-time.Second), 1},
		{operationeffect.ActorKitty, 7, until, 0},
		{operationeffect.ActorPlayer, 0, until, 1},
	} {
		op := operationeffect.Operation{Key: tc.start.String() + string(tc.actor), Domain: operationeffect.DomainAlchemy, ActionID: "alchemy.convert", ActorKind: tc.actor, ActorID: tc.id, StartedAt: tc.start}
		frozen, err := session.Commit(session.Prepare(op))
		if err != nil || len(frozen.Rules) != tc.want {
			t.Fatal("historical boundary", i, frozen, err)
		}
	}
}
