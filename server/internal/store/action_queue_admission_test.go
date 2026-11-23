package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
)

func TestSpecialQueueActionsRejectBareIDsBeforeDatabaseAccess(t *testing.T) {
	s := &Store{}
	specs := BuildActionSpecMap(catalog.Data{})
	for _, tc := range []struct {
		id       string
		executor string
		want     error
	}{
		{FishingTripActionID, "fishing_trip", ErrFishingArgsInvalid},
		{"equipment_enhance", "equipment_enhance", ErrEnhanceArgsInvalid},
		{"building_upgrade", "building_upgrade", ErrBuildingArgsInvalid},
		{"alchemy.convert", "alchemy.convert", ErrAlchemyArgsInvalid},
		{"alchemy.bench", "alchemy.bench", ErrAlchemyArgsInvalid},
		{"alchemy.transmute", "alchemy.transmute", ErrAlchemyArgsInvalid},
		{"alchemy.smelt", "alchemy.smelt", ErrAlchemyArgsInvalid},
		{"alchemy.inherit", "alchemy.inherit", ErrAlchemyArgsInvalid},
	} {
		t.Run(tc.id, func(t *testing.T) {
			specs[tc.id] = ActionSpec{ID: tc.id}
			if got := s.actionHandlerForType(tc.id).ActionID(); got != tc.executor {
				t.Fatalf("executor = %q, want %q", got, tc.executor)
			}
			for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`{"executorId":"generic","runtime":{"durationMs":1},"runContext":{}}`)} {
				assertQueueCommandRejectedWithoutDB(t, s, specs, ActionBatchStep{ActionID: tc.id, Count: 1, ArgsRaw: raw}, tc.want)
			}
		})
	}
}

func assertQueueCommandRejectedWithoutDB(t *testing.T, s *Store, specs map[string]ActionSpec, step ActionBatchStep, want error) {
	t.Helper()
	ctx := context.Background()
	// A nil pool proves rejected commands cannot touch inventory, actor state,
	// existing runs, or insert even the valid prefix of a mixed batch.
	specs["ordinary"] = ActionSpec{ID: "ordinary"}
	for _, actor := range []string{ActionActorTypePlayer, ActionActorTypeKitty} {
		for _, mode := range []string{"run", "queue"} {
			queue, _, _, _, err := s.RunAction(ctx, 7, specs, step.ActionID, mode, step.Count, step.ArgsRaw, actor, 42)
			if !errors.Is(err, want) || queue != nil {
				t.Fatalf("%s/%s: queue=%v err=%v, want %v", actor, mode, queue, err, want)
			}
		}
		for _, mode := range []string{ActionBatchInsertModeAppend, ActionBatchInsertModeRunNowLikePrepend} {
			steps := []ActionBatchStep{{ActionID: "ordinary", Count: 1}, step}
			queue, _, _, _, dropped, err := s.EnqueueActionBatch(ctx, 7, specs, steps, actor, 42, mode)
			if !errors.Is(err, want) || queue != nil || dropped != 0 {
				t.Fatalf("%s/%s: queue=%v dropped=%d err=%v, want %v", actor, mode, queue, dropped, err, want)
			}
		}
	}
}

func TestSpecialQueueActionsPreserveValidPlansAndCountContracts(t *testing.T) {
	s := &Store{}
	specs := BuildActionSpecMap(catalog.Data{})
	for _, tc := range []struct {
		id        string
		raw       string
		wantCount int
	}{
		{FishingTripActionID, `{"spotId":"spot","loadoutId":"loadout","durationSeconds":600,"supplies":{"mainLine":1,"leader":1,"hook":1,"floatOrSinker":1,"bait":10}}`, 1},
		{"building_upgrade", `{"buildingId":"pond"}`, 1},
		{"equipment_enhance", `{"equipmentInstanceId":42,"maxAttempts":7,"targetEnhanceLevel":3}`, 7},
	} {
		t.Run(tc.id, func(t *testing.T) {
			step := ActionBatchStep{ActionID: tc.id, Count: 1, ArgsRaw: json.RawMessage(tc.raw)}
			got, err := s.prepareActionQueueStep(specs, step)
			if err != nil || got.insertCount != tc.wantCount {
				t.Fatalf("prepared=%+v err=%v, want count=%d", got, err, tc.wantCount)
			}
			step.Count = 2
			assertQueueCommandRejectedWithoutDB(t, s, specs, step, ErrInvalidActionCount)
		})
	}
	for _, id := range []string{"ordinary", "alchemy.convert", "alchemy.transmute"} {
		specs[id] = ActionSpec{ID: id}
		raw := json.RawMessage(`{"sourceItemId":"wood","groupId":"group.life_primary_common"}`)
		if id == "alchemy.transmute" {
			raw = json.RawMessage(`{"itemId":"wood"}`)
		}
		got, err := s.prepareActionQueueStep(specs, ActionBatchStep{ActionID: id, Count: 20, ArgsRaw: raw})
		if err != nil || got.insertCount != 20 {
			t.Fatalf("%s: prepared=%+v err=%v, want 20 runs", id, got, err)
		}
	}
}

func TestFishFoodQueueAdmissionPreservesIngredientRules(t *testing.T) {
	recipe := fishFoodTestRecipe()
	s := &Store{gameData: catalog.Data{FishFoodRecipes: []catalog.FishFoodRecipe{recipe}}}
	specs := map[string]ActionSpec{recipe.ActionID: {ID: recipe.ActionID}}
	for _, count := range []int{1, 20} {
		got, err := s.prepareActionQueueStep(specs, ActionBatchStep{ActionID: recipe.ActionID, Count: count})
		if err != nil || got.insertCount != count || got.executor.ActionID() != fishFoodExecutorID {
			t.Fatalf("automatic ingredients: prepared=%+v err=%v", got, err)
		}
	}
	step := ActionBatchStep{ActionID: recipe.ActionID, Count: 2, ArgsRaw: json.RawMessage(`{"ingredientMode":"specific","fishIngredientInstances":{"crucian_carp":[23],"bluegill":[11]}}`)}
	assertQueueCommandRejectedWithoutDB(t, s, specs, step, ErrInvalidActionCount)
	for _, raw := range []string{`{"ingredientMode":"specific"}`, `{"ingredientMode":"specific","fishIngredientInstances":{"crucian_carp":[23],"bluegill":[23]}}`} {
		step.ArgsRaw = json.RawMessage(raw)
		step.Count = 1
		assertQueueCommandRejectedWithoutDB(t, s, specs, step, ErrFishFoodArgsInvalid)
	}
}

func TestSpecialQueuePostgresRejectsUnownedFishingPlan(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	specs := BuildActionSpecMap(s.gameData)
	raw := json.RawMessage(fmt.Sprintf(`{"spotId":%q,"loadoutId":"not-owned","durationSeconds":600,"supplies":{"mainLine":1,"leader":1}}`, s.gameData.FishingSpots[0].ID))
	for _, batch := range []bool{false, true} {
		var err error
		if batch {
			_, _, _, _, _, err = s.EnqueueActionBatch(ctx, 970071, specs, []ActionBatchStep{{ActionID: FishingTripActionID, Count: 1, ArgsRaw: raw}}, ActionActorTypePlayer, 0, ActionBatchInsertModeAppend)
		} else {
			_, _, _, _, err = s.RunAction(ctx, 970071, specs, FishingTripActionID, "run", 1, raw, ActionActorTypePlayer, 0)
		}
		if !errors.Is(err, ErrFishingLoadoutNotFound) {
			t.Fatalf("batch=%v: err=%v, want loadout not found", batch, err)
		}
		for _, table := range []string{"player_action_queue", "fishing_trips", "fishing_catches", "player_item_instances"} {
			var count int
			if err := s.db.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE user_id=970071`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("%s count=%d err=%v, rejected fishing must not write state", table, count, err)
			}
		}
	}
}

func TestSpecialQueuePostgresStartsValidatedFishingPlans(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	migration, err := os.ReadFile("../../schema/migrations/20260607_cat_village.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	specs := BuildActionSpecMap(s.gameData)
	spotID := ""
	for _, spot := range s.gameData.FishingSpots {
		water := s.gameData.FishingWaterByID(spot.WaterID)
		if water != nil && water.RequiredFishingLevel <= 1 && containsFishingMethod(spot.Methods, "float") {
			spotID = spot.ID
			break
		}
	}
	if spotID == "" {
		t.Fatal("starter float spot missing")
	}
	for idx, batch := range []bool{false, true} {
		userID := int64(970071 + idx)
		if err := s.withTx(ctx, func(tx pgx.Tx) error { return s.ensureAllPlayerAttributesTx(ctx, tx, userID) }); err != nil {
			t.Fatal(err)
		}
		starter, err := s.ClaimFishingStarter(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		raw := json.RawMessage(fmt.Sprintf(`{"spotId":%q,"loadoutId":%q,"durationSeconds":600,"supplies":{"mainLine":1,"leader":1,"hook":1,"floatOrSinker":1,"bait":10}}`, spotID, starter.LoadoutID))
		var queue []ActionQueueItem
		if batch {
			queue, _, _, _, _, err = s.EnqueueActionBatch(ctx, userID, specs, []ActionBatchStep{{ActionID: FishingTripActionID, Count: 1, ArgsRaw: raw}}, ActionActorTypePlayer, 0, ActionBatchInsertModeAppend)
		} else {
			queue, _, _, _, err = s.RunAction(ctx, userID, specs, FishingTripActionID, "run", 1, raw, ActionActorTypePlayer, 0)
		}
		if err != nil || len(queue) != 1 || queue[0].Status != "running" {
			t.Fatalf("batch=%v: queue=%+v err=%v", batch, queue, err)
		}
		var trips, mainLine int
		if err := s.db.QueryRow(ctx, `SELECT count(*) FROM fishing_trips WHERE user_id=$1 AND status='running' AND queue_id=$2`, userID, queue[0].ID).Scan(&trips); err != nil || trips != 1 {
			t.Fatalf("authoritative trip count=%d err=%v", trips, err)
		}
		if err := s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0) FROM player_item_instances WHERE user_id=$1 AND template_item_id='braided_main_line' AND state='inventory'`, userID).Scan(&mainLine); err != nil || mainLine != 7 {
			t.Fatalf("supplies were not reserved: mainLine=%d err=%v", mainLine, err)
		}
	}
}
