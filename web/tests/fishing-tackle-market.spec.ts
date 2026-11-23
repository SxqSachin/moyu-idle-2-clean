import { describe, expect, it } from "vitest"
import { getGameDataTable } from "../src/services/game-data-runtime"
import { isAuctionInstanceSelectable } from "../src/composables/game/auction/useAuctionPage"
import { getItemDefinition, getItemTypes, isItemFixedMarketListable, isItemMarketListable } from "../src/services/item-definition"
import { usesInstanceMarket } from "../src/services/instance-market"

type FishingTackleMarketDefinition = {
  itemId?: string
  tournamentReward?: boolean
}

describe("fishing tackle market eligibility", () => {
  it("categorizes caught fish for the variant market, not commodity or auction trading", () => {
    expect(getItemTypes("fish_catch")).toEqual(["fish"])
    expect(usesInstanceMarket("fish_catch")).toBe(true)
    expect(getItemDefinition("fish_catch")?.marketListable).toBe(false)
    expect(isItemMarketListable("fish_catch")).toBe(false)
    expect(isItemFixedMarketListable("fish_catch")).toBe(false)
    expect(isAuctionInstanceSelectable({
      state: "inventory",
      quantity: 1,
      templateItemId: "fish_catch"
    })).toBe(false)
    expect(isAuctionInstanceSelectable({
      state: "inventory",
      quantity: 1,
      templateItemId: "reed_float"
    })).toBe(true)
  })

  it("keeps every configured fish recipe output out of auctions while allowing variant listings", () => {
    const recipes = getGameDataTable<{ outputItemId: string }>("fish_food_recipes")
    expect(recipes.length).toBeGreaterThan(0)
    for (const { outputItemId } of recipes) {
      expect(usesInstanceMarket(outputItemId), outputItemId).toBe(true)
      expect(getItemDefinition(outputItemId)?.marketPolicy?.auction, outputItemId).toBe(false)
      expect(isItemMarketListable(outputItemId), outputItemId).toBe(false)
      expect(isAuctionInstanceSelectable({
        state: "inventory",
        quantity: 1,
        templateItemId: outputItemId
      }), outputItemId).toBe(false)
    }
  })

  it("allows every craftable tackle item while keeping tournament rewards bound", () => {
    const definitions = getGameDataTable<FishingTackleMarketDefinition>("fishing_tackle")
    expect(definitions.length).toBeGreaterThan(0)

    for (const definition of definitions) {
      const itemId = String(definition.itemId || "").trim()
      expect(itemId).not.toBe("")
      if (definition.tournamentReward) {
        expect(getItemDefinition(itemId)?.marketListable, itemId).toBe(false)
        expect(isItemFixedMarketListable(itemId), itemId).toBe(false)
        continue
      }
      expect(getItemDefinition(itemId)?.marketListable, itemId).toBe(true)
      expect(isItemFixedMarketListable(itemId), itemId).toBe(true)
    }
  })
})
