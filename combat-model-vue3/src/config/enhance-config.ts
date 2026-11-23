import { reactive } from 'vue'

export const enhanceConfig = reactive({
  // minSuccessRate = clamp(minRateBase - (maxLevel-15) * (minRateScale/35), minRateFloor, minRateBase)
  minRateBase: 0.50,
  minRateScale: 0.55,
  minRateFloor: 0.15,

  // curvePower = clamp(curvePowerBase - (maxLevel-15) * (curvePowerScale/35), curvePowerMin, curvePowerBase)
  curvePowerBase: 1.25,
  curvePowerScale: 0.50,
  curvePowerMin: 0.75,

  rarityFailMultiplier: {
    common: 1.00,
    uncommon: 1.06,
    rare: 1.12,
    epic: 1.20,
    legendary: 1.30,
    mythic: 1.45,
  } as Record<string, number>,
})

export const RARITIES = ['common', 'uncommon', 'rare', 'epic', 'legendary', 'mythic'] as const
export type RarityKey = typeof RARITIES[number]

export const RARITY_LABELS: Record<RarityKey, string> = {
  common: '普通',
  uncommon: '非凡',
  rare: '稀有',
  epic: '史诗',
  legendary: '传说',
  mythic: '神话',
}

export const RARITY_COLORS: Record<RarityKey, string> = {
  common: '#9ca3af',
  uncommon: '#4ade80',
  rare: '#60a5fa',
  epic: '#c084fc',
  legendary: '#fb923c',
  mythic: '#f43f5e',
}

export const MAX_LEVEL_COLORS: Record<number, string> = {
  10: '#94a3b8',
  15: '#6ee7b7',
  20: '#67e8f9',
  25: '#818cf8',
  30: '#f472b6',
  40: '#fb923c',
  50: '#ef4444',
}

export function computeSuccessRate(rarity: string, level: number, maxLevel: number): number {
  if (maxLevel <= 1) return 1
  const c = enhanceConfig
  const lvl = Math.max(1, Math.min(level, maxLevel))
  const minSuccessRate = Math.max(
    c.minRateFloor,
    Math.min(c.minRateBase, c.minRateBase - (maxLevel - 15) * (c.minRateScale / 35))
  )
  const curvePower = Math.max(
    c.curvePowerMin,
    Math.min(c.curvePowerBase, c.curvePowerBase - (maxLevel - 15) * (c.curvePowerScale / 35))
  )
  const progress = (lvl - 1) / (maxLevel - 1)
  const baseRate = minSuccessRate + (1 - minSuccessRate) * (1 - Math.pow(progress, curvePower))
  const failMult = c.rarityFailMultiplier[rarity] ?? 1
  return Math.round(Math.max(0.01, Math.min(1, 1 - (1 - baseRate) * failMult)) * 10000) / 10000
}

export function resetEnhanceConfig() {
  enhanceConfig.minRateBase = 0.50
  enhanceConfig.minRateScale = 0.55
  enhanceConfig.minRateFloor = 0.15
  enhanceConfig.curvePowerBase = 1.25
  enhanceConfig.curvePowerScale = 0.50
  enhanceConfig.curvePowerMin = 0.75
  enhanceConfig.rarityFailMultiplier.common = 1.00
  enhanceConfig.rarityFailMultiplier.uncommon = 1.06
  enhanceConfig.rarityFailMultiplier.rare = 1.12
  enhanceConfig.rarityFailMultiplier.epic = 1.20
  enhanceConfig.rarityFailMultiplier.legendary = 1.30
  enhanceConfig.rarityFailMultiplier.mythic = 1.45
}
