import type { EquipmentTemplateDefinition } from "@/types/equipment"

export type EnhancePreview = {
  nextLevel: number
  successRate: number
  costs: Record<string, number>
  deltaAttrs: Record<string, number>
}

function clampInt(raw: number): number {
  if (!Number.isFinite(raw)) return 0
  return Math.max(0, Math.round(raw))
}

function clampRate(raw: number): number {
  if (!Number.isFinite(raw)) return 0
  if (raw < 0) return 0
  if (raw > 1) return 1
  return raw
}

function roundRate(raw: number): number {
  if (!Number.isFinite(raw)) return 0
  return Math.round(raw * 10000) / 10000
}

function normalizeRarity(raw: string | undefined): string {
  const value = String(raw || "").trim().toLowerCase()
  switch (value) {
    case "common":
    case "uncommon":
    case "rare":
    case "epic":
    case "legendary":
    case "mythic":
      return value
    default:
      return "common"
  }
}

function rarityFailMultiplier(rarity: string): number {
  switch (rarity) {
    case "uncommon":
      return 1.06
    case "rare":
      return 1.12
    case "epic":
      return 1.1
    case "legendary":
      return 1.26
    case "mythic":
      return 1.4
    default:
      return 1
  }
}

function buildBaseSuccessRate(itemRarity: string | undefined, level: number, maxLevel: number): number {
  if (maxLevel <= 1) return 1
  const targetLevel = Math.max(1, Math.min(level, maxLevel))
  // const minSuccessRate = clampRate(Math.max(0.15, Math.min(0.5, 0.5 - (maxLevel - 15) * (0.55 / 35))))
  // const curvePower = Math.max(0.75, Math.min(1.25, 1.25 - (maxLevel - 15) * (0.5 / 35)))

  const minSuccessRate = clampRate(Math.max(0.15, Math.min(0.45, 0.45 - (maxLevel - 15) * (0.7 / 35))))
  const curvePower = Math.max(0.4, Math.min(1, 1 - (maxLevel - 15) * (0.4 / 35)))


  const progress = (targetLevel - 1) / (maxLevel - 1)
  const baseRate = minSuccessRate + (1 - minSuccessRate) * (1 - Math.pow(progress, curvePower))
  const successRate = 1 - (1 - baseRate) * rarityFailMultiplier(normalizeRarity(itemRarity))
  return roundRate(Math.max(0.01, Math.min(1, successRate)))
}

function resolveMapIncrement(
  base: Record<string, number>,
  step: Record<string, number>,
  overrides: Record<string, Record<string, number>>,
  level: number
): Record<string, number> {
  const override = overrides[String(level)]
  if (override && typeof override === "object") {
    const out: Record<string, number> = {}
    for (const [key, value] of Object.entries(override)) {
      if (!key || !Number.isFinite(value)) continue
      out[key] = value
    }
    return out
  }
  const keys = new Set<string>([...Object.keys(base), ...Object.keys(step)])
  const out: Record<string, number> = {}
  for (const key of keys) {
    const value = level <= 1 ? (base[key] || 0) : (step[key] || 0)
    if (!Number.isFinite(value)) continue
    out[key] = value
  }
  return out
}

function resolveMapLinear(
  base: Record<string, number>,
  step: Record<string, number>,
  overrides: Record<string, Record<string, number>>,
  level: number
): Record<string, number> {
  const override = overrides[String(level)]
  if (override && typeof override === "object") {
    const out: Record<string, number> = {}
    for (const [key, value] of Object.entries(override)) {
      if (!key || !Number.isFinite(value)) continue
      out[key] = value
    }
    return out
  }
  const keys = new Set<string>([...Object.keys(base), ...Object.keys(step)])
  const out: Record<string, number> = {}
  for (const key of keys) {
    const value = (base[key] || 0) + (step[key] || 0) * (level - 1)
    if (!Number.isFinite(value)) continue
    out[key] = value
  }
  return out
}

export function buildEnhanceCumulativeAttrs(template: EquipmentTemplateDefinition | undefined, level: number): Record<string, number> {
  if (!template || level <= 0 || template.enhanceMaxLevel <= 0) return {}
  const cappedLevel = Math.min(level, template.enhanceMaxLevel)
  const out: Record<string, number> = {}
  for (let next = 1; next <= cappedLevel; next += 1) {
    const increment = resolveMapIncrement(
      template.enhanceAttrCurve.base,
      template.enhanceAttrCurve.step,
      template.enhanceAttrCurve.overrides,
      next
    )
    for (const [attrKey, value] of Object.entries(increment)) {
      out[attrKey] = (out[attrKey] || 0) + value
    }
  }
  return out
}

export function buildEnhancePreview(template: EquipmentTemplateDefinition | undefined, currentLevel: number, itemRarity?: string): EnhancePreview | null {
  if (!template || template.enhanceMaxLevel <= 0) return null
  const level = Math.max(0, Math.min(currentLevel, template.enhanceMaxLevel))
  const nextLevel = level + 1
  if (nextLevel > template.enhanceMaxLevel) {
    return {
      nextLevel,
      successRate: 0,
      costs: {},
      deltaAttrs: {}
    }
  }
  const rawCosts = resolveMapLinear(
    template.enhanceCostCurve.base,
    template.enhanceCostCurve.step,
    template.enhanceCostCurve.overrides,
    nextLevel
  )
  const costs: Record<string, number> = {}
  for (const [itemID, value] of Object.entries(rawCosts)) {
    const qty = clampInt(value)
    if (qty <= 0) continue
    costs[itemID] = qty
  }
  const currentCumulative = buildEnhanceCumulativeAttrs(template, level)
  const nextCumulative = buildEnhanceCumulativeAttrs(template, nextLevel)
  const deltaAttrs: Record<string, number> = {}
  for (const key of new Set<string>([...Object.keys(currentCumulative), ...Object.keys(nextCumulative)])) {
    const value = Number(nextCumulative[key] || 0) - Number(currentCumulative[key] || 0)
    if (!Number.isFinite(value)) continue
    deltaAttrs[key] = value
  }
  return {
    nextLevel,
    successRate: clampRate(buildBaseSuccessRate(itemRarity, nextLevel, template.enhanceMaxLevel)),
    costs,
    deltaAttrs
  }
}
