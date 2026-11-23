import { describe, expect, it, vi } from "vitest"
vi.mock("@services/item-definition", () => ({ getItemDefinition: (id: string) => id === "arbitrary" ? { marketPolicy: { fixedPrice: { mode: "instance_variants", profileId: "x" }, buyOrders: false } } : { marketListable: true } }))
import { groupMarketQuotes, instanceMarketSelectable, normalizeMarketListings, usesInstanceMarket, type MarketListing, type MarketQuote } from "@services/instance-market"
import type { InventoryInstance } from "@/types/equipment"
describe("instance market capability and grouping", () => {
  const offer = { id: 1, status: 'active', quantity: 1, price: 600 } as MarketListing
  it("supports a single legacy offer without retaining seller identity", () => {
    const result = normalizeMarketListings([{ ...offer, isOwn: undefined, canBuy: undefined, sellerId: 99, sellerName: 'private' }], 42, 0)
    expect(result[0].canBuy).toBe(true)
    expect(result[0].isOwn).toBe(false)
    expect(result[0]).not.toHaveProperty('sellerName')
    expect(result[0]).not.toHaveProperty('sellerId')
  })
  it("does not enable own offers or skip earlier legacy offers", () => {
    const rows = [42, 99, 100].map((sellerId, index) => ({ ...offer, id: index + 1, sellerId }))
    expect(normalizeMarketListings(rows, 42, 0).map(r => [r.isOwn, r.canBuy])).toEqual([[true, false], [false, true], [false, false]])
    expect(normalizeMarketListings(rows, 42, 30).every(r => !r.canBuy)).toBe(true)
  })
  it("respects explicit server rejection and fails closed for unknown ownership", () => {
    expect(normalizeMarketListings([{ ...offer, isOwn: false, canBuy: false }], 42, 0)[0].canBuy).toBe(false)
    expect(normalizeMarketListings([offer], 42, 0)[0].canBuy).toBe(false)
    expect(normalizeMarketListings([{ ...offer, isOwn: true, canBuy: true }], 42, 0)[0].canBuy).toBe(false)
  })
  it("uses a policy rather than labels", () => { expect(usesInstanceMarket("arbitrary")).toBe(true); expect(usesInstanceMarket("fish_catch")).toBe(false) })
  it("does not combine item templates and trusts exact server keys", () => {
    const quotes = [{ itemId: "arbitrary", groupKey: "a" }, { itemId: "arbitrary", groupKey: "a" }, { itemId: "arbitrary", groupKey: "b" }, { itemId: "other", groupKey: "a" }] as MarketQuote[]
    expect(groupMarketQuotes(quotes).map(g => g.length)).toEqual([2, 1, 1])
  })
  it("rejects archived, escrowed and stacked instances", () => {
    const row = { templateItemId: "arbitrary", state: "inventory", quantity: 1 } as InventoryInstance
    expect(instanceMarketSelectable(row, "arbitrary")).toBe(true)
    expect(instanceMarketSelectable({ ...row, quantity: 2 }, "arbitrary")).toBe(false)
    expect(instanceMarketSelectable({ ...row, state: "market" }, "arbitrary")).toBe(false)
    expect(instanceMarketSelectable({ ...row, archivedAt: "2026-09-05" }, "arbitrary")).toBe(false)
  })
})
