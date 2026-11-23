package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

type fishingLoadoutRequirementTx struct {
	pgx.Tx
	t              *testing.T
	missingRod     bool
	supplyQuantity int64
	playerLevel    float64
	playerChecks   int
}

func (tx *fishingLoadoutRequirementTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	tx.t.Helper()
	return fishingLifecycleRow(func(dest ...any) error {
		if args[0] != int64(7) {
			tx.t.Fatalf("wrong owner: %v", args)
		}
		switch {
		case strings.Contains(query, "FROM player_item_instances"):
			if !strings.Contains(query, "state IN ('inventory', 'equipped')") {
				tx.t.Fatal("missing equipment state check")
			}
			if tx.missingRod && args[1] == int64(1) {
				return pgx.ErrNoRows
			}
			id := "rod"
			if args[1] == int64(2) {
				id = "reel"
			}
			*dest[0].(*string) = id
		case strings.Contains(query, "FROM player_equip_slots"):
			*dest[0].(*int) = 1
		case strings.Contains(query, "FROM player_attributes"):
			tx.playerChecks++
			*dest[0].(*float64) = tx.playerLevel
		default:
			tx.t.Fatalf("unexpected query: %s", query)
		}
		return nil
	})
}

func (tx *fishingLoadoutRequirementTx) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	tx.t.Helper()
	if !strings.Contains(query, "FROM player_item_instances") || args[0] != int64(7) {
		tx.t.Fatalf("unexpected query: %s, %v", query, args)
	}
	return &fishingLoadoutSupplyRows{quantity: tx.supplyQuantity, itemID: args[1].([]string)[0]}, nil
}

type fishingLoadoutSupplyRows struct {
	pgx.Rows
	quantity int64
	itemID   string
	done     bool
}

func (r *fishingLoadoutSupplyRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *fishingLoadoutSupplyRows) Close()     {}
func (r *fishingLoadoutSupplyRows) Err() error { return nil }
func (r *fishingLoadoutSupplyRows) Scan(dest ...any) error {
	*dest[2].(*string) = r.itemID
	*dest[4].(*int64) = r.quantity
	*dest[7].(*[]byte) = []byte(`{}`)
	*dest[8].(*[]byte) = []byte(`[]`)
	*dest[9].(*int) = 100
	*dest[10].(*int) = 100
	*dest[11].(*string) = "inventory"
	return nil
}

func fishingRequirementFixture() (*Store, FishingLoadout) {
	data := catalog.Data{}
	for _, slot := range []string{"rod", "reel", "main_line", "leader", "lure"} {
		data.FishingTackle = append(data.FishingTackle, catalog.FishingTackleDefinition{ItemID: slot, Slot: slot, Methods: []string{"lure"}, MaxLoadKG: 10})
	}
	for _, slot := range []string{"rod", "reel"} {
		data.EquipmentTemplates = append(data.EquipmentTemplates, catalog.EquipmentTemplate{ID: slot, EquipType: "work", Slot: "fishing_" + slot, StatusRequirement: map[string]int64{"fishing": 18}})
	}
	return &Store{gameData: data}, FishingLoadout{Method: "lure", Components: FishingLoadoutComponents{RodInstanceID: 1, ReelInstanceID: 2, MainLineItemID: "main_line", LeaderItemID: "leader", LureItemID: "lure"}}
}

func TestFishingSharedLoadoutDoesNotRequirePlayerLevel(t *testing.T) {
	s, loadout := fishingRequirementFixture()
	tx := &fishingLoadoutRequirementTx{t: t, supplyQuantity: 1, playerLevel: 1}
	snapshot, err := s.freezeFishingLoadoutTx(context.Background(), tx, 7, loadout)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.FrozenComponents) != 5 || tx.playerChecks != 0 {
		t.Fatalf("shared loadout must validate all components without player requirements: %+v, checks=%d", snapshot, tx.playerChecks)
	}
	_, err = s.freezeFishingLoadoutForActorTx(context.Background(), tx, 7, ActionActorTypePlayer, 0, loadout, true)
	if !errors.Is(err, ErrStatusRequireNotMet) || tx.playerChecks != 1 {
		t.Fatalf("using the loadout must still reject the underlevelled player: %v, checks=%d", err, tx.playerChecks)
	}
}

func TestFishingSharedLoadoutPreservesComponentValidation(t *testing.T) {
	for _, name := range []string{"missing equipment", "incompatible method", "missing supplies"} {
		t.Run(name, func(t *testing.T) {
			s, loadout := fishingRequirementFixture()
			tx := &fishingLoadoutRequirementTx{t: t, supplyQuantity: 1}
			want := ErrFishingTackleMissing
			switch name {
			case "missing equipment":
				tx.missingRod = true
			case "incompatible method":
				loadout.Method = "float"
				want = ErrFishingLoadoutInvalid
			case "missing supplies":
				tx.supplyQuantity = 0
				want = ErrInsufficientItem
			}
			_, err := s.freezeFishingLoadoutTx(context.Background(), tx, 7, loadout)
			if !errors.Is(err, want) {
				t.Fatalf("err=%v, want %v", err, want)
			}
		})
	}
}
