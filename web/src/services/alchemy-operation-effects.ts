export function isBulkInfusionMaterial(rarity: string): boolean {
  return ["common", "uncommon", "rare", "epic"].includes(rarity)
}

export function isPreciousInfusionMaterial(rarity: string): boolean {
  return rarity === "legendary" || rarity === "mythic"
}

export function infusionMaterialTier(rarity: string): 0 | 1 | 2 {
  return ["common", "uncommon"].includes(rarity) ? 1 : ["rare", "epic"].includes(rarity) ? 2 : 0
}

export function infusionTierRateLimit(tier: number): number {
  return tier === 1 ? .20 : tier === 2 ? .15 : MAX_QUANTITY_LIMIT_BONUS_RATE
}

export type InfusionTierCapacities = Partial<Record<1 | 2, { quantity: number; rate: number }>>

export const MAX_QUANTITY_LIMIT_BONUS_RATE = 0.17
export const MAX_SUCCESS_RATE_PENALTY = 0.20
export const MAX_SUCCESS_RATE_BONUS = 0.10

export function applyInfusionFoodFailureChance(failChance: number, penalty: number, successFloor = 0, bonus = 0): number {
  if (penalty <= 0 && bonus <= 0) return failChance
  const maximum = 1 - Math.min(1, Math.max(0, successFloor))
  const penaltyRate = Math.min(MAX_SUCCESS_RATE_PENALTY, Math.max(0, penalty))
  const bonusRate = Math.min(MAX_SUCCESS_RATE_BONUS, Math.max(0, bonus))
  return Math.max(0, Math.min(maximum, failChance + penaltyRate - bonusRate))
}

type CapacityModifiers = {
  tierCapacities?: InfusionTierCapacities
  extraMaterialQuantity: number
  bulkMaterialQuantity: number
  preciousMaterialQuantity?: number
  bulkQuantityLimitBonusRate?: number
  preciousQuantityLimitBonusRate?: number
}

export function infusionMaterialQuantityBonus(rarity: string, baseLimit: number, modifiers: CapacityModifiers): number {
  const scaled = (fixed: number, rate = 0, rateLimit = MAX_QUANTITY_LIMIT_BONUS_RATE) => {
    const base = Math.max(0, baseLimit)
    const proportional = Math.floor(base * Math.min(rateLimit, Math.max(0, rate)) + 1e-9)
    return Math.min(Math.max(fixed, proportional), base * 2)
  }
  const bulk = isBulkInfusionMaterial(rarity) ? scaled(modifiers.bulkMaterialQuantity, modifiers.bulkQuantityLimitBonusRate) : 0
  const precious = isPreciousInfusionMaterial(rarity) ? scaled(modifiers.preciousMaterialQuantity || 0, modifiers.preciousQuantityLimitBonusRate) : 0
  const tier = infusionMaterialTier(rarity)
  const capacity = tier ? modifiers.tierCapacities?.[tier] : undefined
  const scoped = capacity ? scaled(capacity.quantity, capacity.rate, infusionTierRateLimit(tier)) : 0
  return Math.min(1_000_000, modifiers.extraMaterialQuantity + Math.max(bulk, precious, scoped))
}

export function infusionQuantityLimits(rarity: string, baseLimit: number, available: number, unitBasis: number, modifiers: CapacityModifiers) {
  const basis = Math.max(1, Math.trunc(unitBasis) || 1)
  const align = (value: number) => Math.floor(Math.max(0, value) / basis) * basis
  const bonus = infusionMaterialQuantityBonus(rarity, baseLimit, modifiers)
  const baseCapacity = align(baseLimit)
  const capacity = align(baseLimit + bonus)
  const usableStock = align(available)
  return { baseCapacity, capacity, bonus, effectiveBonus: capacity - baseCapacity, usableStock, selectable: Math.min(capacity, usableStock), stockLimited: usableStock < capacity, basis }
}

// Keep the preview and its before/after explanation on the same range calculation.
export function applyInfusionOperationRange(minRaw: number, maxRaw: number, modifiers: { positiveFloorReductionRate: number; positiveCeilingBonusRate: number }): [number, number] {
  let min = Math.min(minRaw, maxRaw)
  const max = Math.max(minRaw, maxRaw)
  if (max <= 0) return [min, max]
  const adjustedMax = max * (1 + Math.min(0.25, Math.max(0, modifiers.positiveCeilingBonusRate)))
  if (min > 0) min *= 1 - Math.min(0.95, Math.max(0, modifiers.positiveFloorReductionRate))
  return [min, adjustedMax]
}
