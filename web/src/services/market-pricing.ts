import { MARKET_PRICE_STEP_CONFIG, type MarketPriceStep } from "@/config/shared-global-config"
import { getItemDefinition } from "@services/item-definition"

type MarketPriceRow = {
  itemId: string
  price: number
  status?: string
}

function asSafePositiveInt(raw: unknown, fallback = 1): number {
  const numeric = typeof raw === "number" ? raw : Number(raw)
  if (!Number.isFinite(numeric)) return fallback
  return Math.max(1, Math.floor(numeric))
}

export function getItemRealValuePrice(itemID: string): number {
  const definition = getItemDefinition(itemID)
  return asSafePositiveInt(definition?.economyBaseValue, 1)
}

export function getMarketPriceStep(price: number): MarketPriceStep {
  const normalizedPrice = Math.max(0, Math.floor(price))
  const matched = MARKET_PRICE_STEP_CONFIG.find((row) => normalizedPrice >= row.min && normalizedPrice < row.max)
  return matched || MARKET_PRICE_STEP_CONFIG[MARKET_PRICE_STEP_CONFIG.length - 1] || { min: 0, max: Number.POSITIVE_INFINITY, step: 1 }
}

export function isPriceOnMarketStepGrid(price: number): boolean {
  const safePrice = asSafePositiveInt(price, 1)
  const stepCfg = getMarketPriceStep(safePrice)
  if (stepCfg.step <= 1) return true
  return (safePrice - stepCfg.min) % stepCfg.step === 0
}

export function isValidMarketPrice(price: number, minPrice: number): boolean {
  const safeMinPrice = asSafePositiveInt(minPrice, 1)
  const safePrice = asSafePositiveInt(price, 1)
  if (safePrice < safeMinPrice) return false
  if (safePrice === safeMinPrice) return true
  return isPriceOnMarketStepGrid(safePrice)
}

export function normalizeMarketPrice(price: number, minPrice: number, direction: "up" | "down"): number {
  const safeMinPrice = asSafePositiveInt(minPrice, 1)
  const safePrice = asSafePositiveInt(price, safeMinPrice)
  if (safePrice <= safeMinPrice) return safeMinPrice
  if (safePrice === safeMinPrice) return safeMinPrice
  if (isPriceOnMarketStepGrid(safePrice)) return safePrice

  const stepCfg = getMarketPriceStep(safePrice)
  if (stepCfg.step <= 1) return Math.max(safeMinPrice, safePrice)
  const offset = safePrice - stepCfg.min
  const remainder = ((offset % stepCfg.step) + stepCfg.step) % stepCfg.step
  if (remainder === 0) return Math.max(safeMinPrice, safePrice)
  if (direction === "down") {
    const next = safePrice - remainder
    if (next <= safeMinPrice) return safeMinPrice
    return next
  }
  const next = safePrice + (stepCfg.step - remainder)
  return Math.max(safeMinPrice, next)
}

export function nextMarketPrice(currentPrice: number, minPrice: number): number {
  const safeMinPrice = asSafePositiveInt(minPrice, 1)
  const safeCurrentPrice = Math.max(safeMinPrice, asSafePositiveInt(currentPrice, safeMinPrice))
  const stepCfg = getMarketPriceStep(safeCurrentPrice)
  const target = safeCurrentPrice + Math.max(1, stepCfg.step)
  return normalizeMarketPrice(target, safeMinPrice, "up")
}

export function previousMarketPrice(currentPrice: number, minPrice: number): number {
  const safeMinPrice = asSafePositiveInt(minPrice, 1)
  const safeCurrentPrice = Math.max(safeMinPrice, asSafePositiveInt(currentPrice, safeMinPrice))
  if (safeCurrentPrice <= safeMinPrice) return safeMinPrice
  const stepCfg = getMarketPriceStep(safeCurrentPrice)
  const target = safeCurrentPrice - Math.max(1, stepCfg.step)
  if (target <= safeMinPrice) return safeMinPrice
  return normalizeMarketPrice(target, safeMinPrice, "down")
}

function filterActiveRowsByItem(rows: MarketPriceRow[], itemID: string): number[] {
  const normalizedItemID = String(itemID || "").trim()
  if (!normalizedItemID) return []
  return rows
    .filter((row) => row.itemId === normalizedItemID)
    .filter((row) => !row.status || row.status === "active")
    .map((row) => asSafePositiveInt(row.price, 0))
    .filter((price) => price > 0)
}

export function suggestListingPrice(itemID: string, listings: MarketPriceRow[]): number {
  const minPrice = getItemRealValuePrice(itemID)
  const prices = filterActiveRowsByItem(listings, itemID)
  if (prices.length === 0) return minPrice
  const candidate = Math.min(...prices)
  if (candidate <= minPrice) return minPrice
  return isValidMarketPrice(candidate, minPrice) ? candidate : normalizeMarketPrice(candidate, minPrice, "up")
}

export function suggestBuyOrderPrice(itemID: string, buyOrders: MarketPriceRow[]): number {
  const minPrice = getItemRealValuePrice(itemID)
  const prices = filterActiveRowsByItem(buyOrders, itemID)
  if (prices.length === 0) return minPrice
  const candidate = Math.max(...prices)
  if (candidate <= minPrice) return minPrice
  return isValidMarketPrice(candidate, minPrice) ? candidate : normalizeMarketPrice(candidate, minPrice, "up")
}
