package store

import (
	"errors"
	"math"
	"testing"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

func TestComputeRecycleGold(t *testing.T) {
	t.Run("integer price and quantity", func(t *testing.T) {
		unitPrice, totalGold, err := computeRecycleGold(16, 1, 3)
		if err != nil {
			t.Fatalf("computeRecycleGold returned error: %v", err)
		}
		if unitPrice != 16 || totalGold != 48 {
			t.Fatalf("computeRecycleGold(16,1,3)=(%d,%d), want (16,48)", unitPrice, totalGold)
		}
	})

	t.Run("decimal price floors total", func(t *testing.T) {
		unitPrice, totalGold, err := computeRecycleGold(2.4, 0.5, 3)
		if err != nil {
			t.Fatalf("computeRecycleGold returned error: %v", err)
		}
		if unitPrice != 1 || totalGold != 3 {
			t.Fatalf("computeRecycleGold(2.4,0.5,3)=(%d,%d), want (1,3)", unitPrice, totalGold)
		}
	})

	t.Run("reject non-positive price", func(t *testing.T) {
		_, _, err := computeRecycleGold(0, 1, 1)
		if !errors.Is(err, ErrRecyclePriceInvalid) {
			t.Fatalf("expected ErrRecyclePriceInvalid, got %v", err)
		}
	})

	t.Run("reject non-positive coefficient", func(t *testing.T) {
		_, _, err := computeRecycleGold(10, 0, 1)
		if !errors.Is(err, ErrRecyclePriceInvalid) {
			t.Fatalf("expected ErrRecyclePriceInvalid, got %v", err)
		}
	})

	t.Run("reject nan and inf", func(t *testing.T) {
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			_, _, err := computeRecycleGold(value, 1, 1)
			if !errors.Is(err, ErrRecyclePriceInvalid) {
				t.Fatalf("expected ErrRecyclePriceInvalid for %v, got %v", value, err)
			}
		}
	})

	t.Run("reject overflow and non-positive floored total", func(t *testing.T) {
		_, _, err := computeRecycleGold(float64(math.MaxInt64), 1, 2)
		if !errors.Is(err, ErrRecyclePriceInvalid) {
			t.Fatalf("expected ErrRecyclePriceInvalid for overflow, got %v", err)
		}
		_, _, err = computeRecycleGold(0.4, 1, 1)
		if !errors.Is(err, ErrRecyclePriceInvalid) {
			t.Fatalf("expected ErrRecyclePriceInvalid for floored total <= 0, got %v", err)
		}
	})
}

func TestResolveRecycleGoldCoefficient(t *testing.T) {
	t.Run("full match keeps base coefficient", func(t *testing.T) {
		coefficient, err := resolveRecycleGoldCoefficient(0.75, 0.6, recycleTemplateMatchFull)
		if err != nil {
			t.Fatalf("resolveRecycleGoldCoefficient returned error: %v", err)
		}
		if math.Abs(coefficient-0.75) > 0.000000001 {
			t.Fatalf("coefficient=%v, want 0.75", coefficient)
		}
	})

	t.Run("affix mismatch applies extra coefficient", func(t *testing.T) {
		coefficient, err := resolveRecycleGoldCoefficient(0.75, 0.6, recycleTemplateMatchAffixMismatchOnly)
		if err != nil {
			t.Fatalf("resolveRecycleGoldCoefficient returned error: %v", err)
		}
		if math.Abs(coefficient-0.35) > 0.000000001 {
			t.Fatalf("coefficient=%v, want 0.35", coefficient)
		}
	})

	t.Run("reject invalid match kind", func(t *testing.T) {
		if _, err := resolveRecycleGoldCoefficient(0.75, 0.6, recycleTemplateMatchNone); !errors.Is(err, ErrItemNotRecyclable) {
			t.Fatalf("expected ErrItemNotRecyclable, got %v", err)
		}
	})

	t.Run("reject non-positive coefficient after discount", func(t *testing.T) {
		if _, err := resolveRecycleGoldCoefficient(0.75, 0.25, recycleTemplateMatchAffixMismatchOnly); !errors.Is(err, ErrRecyclePriceInvalid) {
			t.Fatalf("expected ErrRecyclePriceInvalid, got %v", err)
		}
	})
}

func TestNormalizeRecycleBatchItemsDedupesAndSorts(t *testing.T) {
	got, err := normalizeRecycleBatchItems([]RecycleBatchItemInput{
		{InstanceID: 9, Quantity: 1},
		{InstanceID: 3, Quantity: 2},
		{InstanceID: 9, Quantity: 5},
		{InstanceID: 5, Quantity: 4},
	})
	if err != nil {
		t.Fatalf("normalizeRecycleBatchItems returned err: %v", err)
	}
	want := []RecycleBatchItemInput{
		{InstanceID: 3, Quantity: 2},
		{InstanceID: 5, Quantity: 4},
		{InstanceID: 9, Quantity: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d (%v)", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNormalizeRecycleBatchItemsRejectsInvalidBatches(t *testing.T) {
	if _, err := normalizeRecycleBatchItems(nil); err == nil {
		t.Fatalf("expected nil batch to be rejected")
	}
	if _, err := normalizeRecycleBatchItems([]RecycleBatchItemInput{{InstanceID: 1, Quantity: 0}}); err == nil {
		t.Fatalf("expected non-positive quantity to be rejected")
	}
	if _, err := normalizeRecycleBatchItems([]RecycleBatchItemInput{{InstanceID: 0, Quantity: 1}}); err == nil {
		t.Fatalf("expected non-positive instance id to be rejected")
	}
	tooMany := make([]RecycleBatchItemInput, RecycleBatchItemLimit+1)
	for idx := range tooMany {
		tooMany[idx] = RecycleBatchItemInput{InstanceID: int64(idx + 1), Quantity: 1}
	}
	if _, err := normalizeRecycleBatchItems(tooMany); err == nil {
		t.Fatalf("expected oversized batch to be rejected")
	}
}

func TestNormalizeRecycleBatchItemsAllowsBatchLimit(t *testing.T) {
	items := make([]RecycleBatchItemInput, RecycleBatchItemLimit)
	for idx := range items {
		items[idx] = RecycleBatchItemInput{InstanceID: int64(idx + 1), Quantity: 1}
	}
	got, err := normalizeRecycleBatchItems(items)
	if err != nil {
		t.Fatalf("normalizeRecycleBatchItems returned err: %v", err)
	}
	if len(got) != RecycleBatchItemLimit {
		t.Fatalf("len=%d, want %d", len(got), RecycleBatchItemLimit)
	}
}

func TestPrepareRecycleBatchItemsPreservesBatchSemantics(t *testing.T) {
	s := Store{
		config: RuntimeConfig{
			ItemRecycleGoldCoefficient:               1,
			ItemRecycleAffixMismatchExtraCoefficient: 1,
		},
		gameData: catalog.Data{
			Items: []catalog.Item{
				{ID: "wood", EconomyBaseValue: 7, Stackable: true},
				{ID: "stone", EconomyBaseValue: 3, Stackable: true},
			},
		},
	}
	items := []RecycleBatchItemInput{
		{InstanceID: 3, Quantity: 2},
		{InstanceID: 9, Quantity: 1},
	}
	locked := map[int64]LockedItemInstance{
		3: recycleTestLockedInstance(3, 42, "wood", 5),
		9: recycleTestLockedInstance(9, 42, "stone", 1),
	}

	prepared, out, err := s.prepareRecycleBatchItems(42, items, locked)
	if err != nil {
		t.Fatalf("prepareRecycleBatchItems returned err: %v", err)
	}
	if len(prepared) != 2 || len(out.Results) != 2 {
		t.Fatalf("prepared/results len=(%d,%d), want (2,2)", len(prepared), len(out.Results))
	}
	if out.TotalGold != 17 {
		t.Fatalf("totalGold=%d, want 17", out.TotalGold)
	}
	if out.Results[0].InstanceID != 3 || out.Results[0].TotalGold != 14 || out.Results[0].InstanceRemoved {
		t.Fatalf("unexpected first result: %+v", out.Results[0])
	}
	if out.Results[1].InstanceID != 9 || out.Results[1].TotalGold != 3 || !out.Results[1].InstanceRemoved {
		t.Fatalf("unexpected second result: %+v", out.Results[1])
	}
}

func TestPrepareRecycleBatchItemsRejectsInvalidRows(t *testing.T) {
	s := Store{
		config: RuntimeConfig{
			ItemRecycleGoldCoefficient:               1,
			ItemRecycleAffixMismatchExtraCoefficient: 1,
		},
		gameData: catalog.Data{
			Items: []catalog.Item{{ID: "wood", EconomyBaseValue: 7, Stackable: true}},
		},
	}

	_, _, err := s.prepareRecycleBatchItems(42, []RecycleBatchItemInput{{InstanceID: 1, Quantity: 1}}, map[int64]LockedItemInstance{})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for missing locked row, got %v", err)
	}

	locked := map[int64]LockedItemInstance{1: recycleTestLockedInstance(1, 7, "wood", 1)}
	_, _, err = s.prepareRecycleBatchItems(42, []RecycleBatchItemInput{{InstanceID: 1, Quantity: 1}}, locked)
	if !errors.Is(err, ErrInstanceNotOwned) {
		t.Fatalf("expected ErrInstanceNotOwned, got %v", err)
	}

	equipped := recycleTestLockedInstance(1, 42, "wood", 1)
	equipped.State = "equipped"
	locked[1] = equipped
	_, _, err = s.prepareRecycleBatchItems(42, []RecycleBatchItemInput{{InstanceID: 1, Quantity: 1}}, locked)
	if !errors.Is(err, ErrInvalidInstanceState) {
		t.Fatalf("expected ErrInvalidInstanceState, got %v", err)
	}

	locked[1] = recycleTestLockedInstance(1, 42, "wood", 1)
	_, _, err = s.prepareRecycleBatchItems(42, []RecycleBatchItemInput{{InstanceID: 1, Quantity: 2}}, locked)
	if !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("expected ErrInsufficientItem, got %v", err)
	}
}

func recycleTestLockedInstance(instanceID int64, userID int64, itemID string, quantity int64) LockedItemInstance {
	return LockedItemInstance{
		InstanceID:    instanceID,
		UserID:        userID,
		TemplateItem:  itemID,
		Quantity:      quantity,
		Durability:    100,
		MaxDurability: 100,
		State:         "inventory",
		AffixesRaw:    []byte(`[]`),
	}
}
