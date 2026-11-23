package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestCaughtFishUsesVariantsInsteadOfCommodityOrAuctionMarkets(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	st := Store{gameData: data}
	if !data.ItemByID("fish_catch").HasType("fish") {
		t.Fatal("caught fish must appear in the data-driven fish category")
	}
	if !data.ItemByID("fish_catch").UsesInstanceMarket() {
		t.Fatal("caught fish must opt into instance variants")
	}

	if st.TemplateItemMarketListable("fish_catch") {
		t.Fatal("caught fish must not be listable on the commodity market")
	}
	if st.auctionItemListable("fish_catch") {
		t.Fatal("caught fish must not be listable through the instance auction")
	}
	if _, err := st.CreateListing(context.Background(), 1, "fish_catch", 1, 1); !errors.Is(err, ErrMarketItemNotListable) {
		t.Fatalf("fixed-price listing error = %v, want %v", err, ErrMarketItemNotListable)
	}
}

func TestFishRecipeOutputsUseVariantsButRejectAuctions(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	st := Store{gameData: data}
	if len(data.FishFoodRecipes) == 0 {
		t.Fatal("expected configured fish food recipes")
	}
	for _, recipe := range data.FishFoodRecipes {
		t.Run(recipe.OutputItemID, func(t *testing.T) {
			item := data.ItemByID(recipe.OutputItemID)
			if item == nil || !item.UsesInstanceMarket() {
				t.Fatal("recipe output must opt into instance variants")
			}
			if st.auctionItemListable(recipe.OutputItemID) {
				t.Fatal("recipe output must not be listable through auctions")
			}
		})
	}
	if !st.auctionItemListable("reed_float") {
		t.Fatal("ordinary tradeable tackle must retain auction eligibility")
	}
}
