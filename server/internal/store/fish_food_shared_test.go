package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
)

func TestFishCuisineAllRecipesRespectAudienceAndExpiry(t *testing.T) {
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	for _, recipe := range s.gameData.FishFoodRecipes {
		t.Run(recipe.ID, func(t *testing.T) {
			def, ok, err := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, nil)
			if err != nil || !ok {
				t.Fatal(err)
			}
			if def.Domain == "enchantment" {
				if !reflect.DeepEqual(def.AllowedTargets, []operationeffect.TargetKind{operationeffect.TargetPlayer}) {
					t.Fatal(def.AllowedTargets)
				}
				return
			}
			allowed := []operationeffect.TargetKind{operationeffect.TargetPlayer, operationeffect.TargetKitty}
			if def.Lifetime.Kind == operationeffect.LifetimeCharges {
				allowed = append(allowed, operationeffect.TargetPlayerAndKitties)
			}
			if !reflect.DeepEqual(def.AllowedTargets, allowed) {
				t.Fatal(def.AllowedTargets)
			}
			actionID, mainType := "", ""
			if len(def.Selector.ActionIDs) > 0 {
				actionID = def.Selector.ActionIDs[0]
			} else if len(def.Selector.MainTypes) > 0 {
				actionID, mainType = "test-primary-work", def.Selector.MainTypes[0]
			}
			until := at.Add(time.Duration(def.Lifetime.DurationSeconds) * time.Second)
			grant := operationeffect.Grant{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, InitialCharges: 2, RemainingCharges: 2}
			if def.Lifetime.Kind == operationeffect.LifetimeDuration {
				grant.Target = operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: 1}
				grant.ValidUntil = &until
				grant.InitialCharges = 0
				grant.RemainingCharges = 0
			}
			session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{grant})
			if err != nil {
				t.Fatal(err)
			}
			// A second kitty is created after activation; the audience is not a frozen ID list.
			for i, actor := range []operationeffect.ActorKind{operationeffect.ActorKitty, operationeffect.ActorPlayer, operationeffect.ActorKitty} {
				op := operationeffect.Operation{Key: fmt.Sprintf("%s:%d", recipe.ID, i), Domain: def.Domain, ActionID: actionID, MainType: mainType, ActorKind: actor, ActorID: int64(i + 1), StartedAt: at.Add(time.Duration(i) * time.Second), Capabilities: map[string]bool{"main_output": true, "transmute.gold_eligible": true, alchemyeffect.CapabilityInfusion: true, alchemyeffect.CapabilityBasicMaterial: true, alchemyeffect.CapabilityRefinedMaterial: true, alchemyeffect.CapabilityBulkMaterial: true, alchemyeffect.CapabilityPreciousMaterial: true}}
				if actor == operationeffect.ActorPlayer {
					op.ActorID = 0
				}
				if i == 0 && (session.HasMatchingCrossActorCharge(op, at.Add(time.Hour)) != (def.Lifetime.Kind == operationeffect.LifetimeCharges)) {
					t.Fatal("cross-lane batching guard mismatch")
				}
				frozen, err := session.Commit(session.Prepare(op))
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if def.Lifetime.Kind == operationeffect.LifetimeDuration && i != 0 {
					want = 0
				}
				if i == 2 && def.Lifetime.Kind == operationeffect.LifetimeCharges {
					want = 0
				}
				if len(frozen.Rules) != want {
					t.Fatalf("actor=%s rules=%d want=%d", actor, len(frozen.Rules), want)
				}
				replay, err := session.Commit(session.Prepare(op))
				if err != nil || !reflect.DeepEqual(frozen, replay) {
					t.Fatal("retry altered snapshot or spent again")
				}
				if def.Lifetime.Kind == operationeffect.LifetimeDuration {
					op.Key += "expired"
					op.StartedAt = until
					after, err := session.Commit(session.Prepare(op))
					if err != nil || len(after.Rules) != 0 {
						t.Fatal("expiry did not apply to every actor")
					}
				}
			}
			expectedReceipts := 0
			if def.Lifetime.Kind == operationeffect.LifetimeCharges {
				expectedReceipts = 2
			}
			if len(session.ChargeReceipts()) != expectedReceipts {
				t.Fatal("shared uses duplicated")
			}
			// Loading a historical player-only snapshot does not broaden an existing grant.
			grant.Definition.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayer}
			grant.Target = operationeffect.Target{Kind: operationeffect.TargetPlayer}
			legacy, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{grant})
			if err != nil {
				t.Fatal(err)
			}
			kitty := operationeffect.Operation{Key: "legacy-kitty", Domain: def.Domain, ActionID: actionID, MainType: mainType, ActorKind: operationeffect.ActorKitty, ActorID: 1, StartedAt: at, Capabilities: map[string]bool{"main_output": true, alchemyeffect.CapabilityInfusion: true, alchemyeffect.CapabilityBasicMaterial: true, alchemyeffect.CapabilityRefinedMaterial: true, alchemyeffect.CapabilityBulkMaterial: true, alchemyeffect.CapabilityPreciousMaterial: true}}
			frozen, err := legacy.Commit(legacy.Prepare(kitty))
			if err != nil || len(frozen.Rules) != 0 {
				t.Fatal("historical audience changed")
			}
		})
	}
}

func TestFishCuisineLegacyItemsNowGrantSharedAudience(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, recipe := range s.gameData.FishFoodRecipes {
		for _, id := range recipe.LegacyEffectIDs {
			old := s.operationEffectDefinitions[id]
			raw, _ := json.Marshal(map[string]any{"operationEffect": operationEffectItemSnapshot{SchemaVersion: 1, DefinitionID: id, RulesetVersion: old.RulesetVersion, Params: old.Params}})
			current, ok, err := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, raw)
			if err != nil || !ok {
				t.Fatal(err)
			}
			allowed := []operationeffect.TargetKind{operationeffect.TargetPlayer, operationeffect.TargetKitty}
			if current.Lifetime.Kind == operationeffect.LifetimeCharges {
				allowed = append(allowed, operationeffect.TargetPlayerAndKitties)
			}
			if !reflect.DeepEqual(current.AllowedTargets, allowed) || !reflect.DeepEqual(current.Lifetime, old.Lifetime) {
				t.Fatalf("legacy audience/lifetime: %+v", current)
			}
		}
	}
}

func TestFishCuisinePostgresSharedChargeLedgerAndScopeConflict(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := s.operationEffectDefinitions["food_steadyfin_grill_v2"]
	def.Lifetime.Charges = 2
	activate := func(def operationeffect.Definition, target operationeffect.TargetKind) error {
		raw, _ := json.Marshal(def)
		_, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "steadyfin_grill", def, operationeffect.Target{Kind: target}, raw, at)
		return err
	}
	old := def
	old.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayer}
	if err := activate(old, operationeffect.TargetPlayer); err != nil {
		t.Fatal(err)
	}
	if err := activate(def, operationeffect.TargetPlayerAndKitties); !errors.Is(err, ErrOperationEffectStrengthConflict) {
		t.Fatalf("overlapping old scope was stacked: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM player_operation_effect_grants`); err != nil {
		t.Fatal(err)
	}
	if err := activate(def, operationeffect.TargetPlayerAndKitties); err != nil {
		t.Fatal(err)
	}
	for i, actor := range []string{ActionActorTypeKitty, ActionActorTypePlayer, ActionActorTypeKitty} {
		started := at.Add(time.Duration(i) * time.Second)
		session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, started)
		if err != nil {
			t.Fatal(err)
		}
		if session == nil {
			session, err = operationeffect.NewSession(s.operationEffectRegistry, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		item := QueueRuntimeItem{QueueID: int64(i + 1), ActionID: "equipment_enhance", ActorType: actor, ActorID: int64(i + 1)}
		if actor == ActionActorTypePlayer {
			item.ActorID = 0
		}
		op := queueOperationEffectOperation(operationeffect.DomainEnhancement, item, started, 1, nil)
		snapshot, err := session.Commit(session.Prepare(op))
		if err != nil {
			t.Fatal(err)
		}
		bonus, err := ratebonus.Resolve(snapshot, ratebonus.EnhanceSuccess)
		if err != nil {
			t.Fatal(err)
		}
		if (bonus > 0) != (i < 2) {
			t.Fatalf("actor=%s bonus=%v", actor, bonus)
		}
		for retry := 0; retry < 2; retry++ {
			if _, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, started); err != nil {
				t.Fatal(err)
			}
		}
	}
	var remaining, receipts int
	if err := tx.QueryRow(ctx, `SELECT remaining_charges,(SELECT COUNT(*) FROM player_operation_effect_charge_receipts) FROM player_operation_effect_grants`).Scan(&remaining, &receipts); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 || receipts != 2 {
		t.Fatalf("remaining=%d receipts=%d", remaining, receipts)
	}
	if other, err := s.loadOperationEffectSessionTx(ctx, tx, 42, at); err != nil || other != nil {
		t.Fatal("another account received the shared effect")
	}
}
