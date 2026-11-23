import { apiGet, apiPost } from "@services/api"
import { getItemDefinition } from "@services/item-definition"
import type { InventoryInstance } from "@/types/equipment"

export type MarketField = { labelKey: string; format: "text" | "number" | "percent" | "seconds" | "operation_effect"; value: unknown }
export type MarketGroup = { itemId: string; groupKey: string; catalogKey: string; quantity: number; sellerCount: number; lowestPrice: number; fields: MarketField[]; snapshot?: MarketPreview }
export type MarketListing = { id: number; isOwn: boolean; canBuy: boolean; itemId: string; groupKey: string; quantity: number; initialQuantity: number; price: number; status: string; fields: MarketField[]; createdAt: string }
export type MarketOrder = { id: number; listingId: number; itemId: string; quantity: number; total: number; fields: MarketField[]; createdAt: string }
export type MarketPreview = Pick<InventoryInstance, "customData" | "enhanceLevel" | "baseSnapshot" | "affixes" | "durability" | "maxDurability" | "fishData">
export type MarketQuote = { instanceId: number; itemId: string; groupKey: string; catalogKey: string; revision: string; fields: MarketField[]; quoteToken: string; snapshot: MarketPreview }
export type MarketDetail = { instanceId: number; state: string; snapshot: MarketPreview }
export type MarketPage<T> = { items: T[]; hasMore: boolean }

type MarketListingResponse = Omit<MarketListing, 'isOwn' | 'canBuy'> & {
  isOwn?: boolean; canBuy?: boolean; sellerId?: number; sellerName?: string
}

export function normalizeMarketListings(rows: MarketListingResponse[], viewer: number, offset: number): MarketListing[] {
  const ownership = (row: MarketListingResponse) => typeof row.isOwn === 'boolean'
    ? row.isOwn
    : Number.isSafeInteger(row.sellerId) && Number(row.sellerId) > 0 && viewer > 0 ? row.sellerId === viewer : undefined
  // During a rolling update, older responses lack canBuy. Only the first page
  // with known ownership can establish the first eligible offer; explicit false wins.
  const knownOwners = rows.every(row => ownership(row) !== undefined)
  const first = offset === 0 && knownOwners ? rows.find(row => row.status === 'active' && row.quantity > 0 && !ownership(row)) : undefined
  return rows.map(({ sellerId, sellerName, ...row }) => ({
    ...row,
    isOwn: ownership({ ...row, sellerId }) === true,
    canBuy: row.status === 'active' && row.quantity > 0 && ownership({ ...row, sellerId }) === false &&
      (typeof row.canBuy === 'boolean' ? row.canBuy : row.id === first?.id)
  }))
}

export function usesInstanceMarket(itemId: string): boolean {
  const p = getItemDefinition(itemId)?.marketPolicy
  return p?.fixedPrice?.mode === "instance_variants" && Boolean(p.fixedPrice.profileId) && p.buyOrders === false
}
export function instanceMarketSelectable(row: InventoryInstance, itemId: string): boolean {
  return row.templateItemId === itemId && row.state === "inventory" && row.quantity === 1 && !row.archivedAt && usesInstanceMarket(itemId)
}
export function groupMarketQuotes(quotes: MarketQuote[]): MarketQuote[][] {
  const groups = new Map<string, MarketQuote[]>()
  for (const quote of quotes) {
    const key = `${quote.itemId}:${quote.groupKey}`
    const group = groups.get(key) || []
    group.push(quote)
    groups.set(key, group)
  }
  return [...groups.values()]
}
export function marketRead<T>(token: string, view: string, query: Record<string, string | number> = {}): Promise<MarketPage<T>> {
  const params = new URLSearchParams(Object.entries(query).map(([k, v]) => [k, String(v)]))
  return apiGet(`/api/market/instances/${view}?${params}`, token)
}
export function marketWrite<T>(token: string, command: string, body: unknown, key: string): Promise<T> {
  return apiPost(`/api/market/instances/${command}`, token, body, { "X-Idempotency-Key": key })
}
