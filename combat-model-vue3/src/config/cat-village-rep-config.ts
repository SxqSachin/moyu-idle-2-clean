import { reactive } from 'vue'

export interface CatVillageRepConfig {
  maxLevel: number
  baseXp: number
  growthFactor: number
}

/** 生产环境声望等级阈值（来自 cat-village-config.json） */
export const PROD_REP_LEVELS: Array<{ level: number; xpRequired: number }> = [
  { level: 1, xpRequired: 50 },
  { level: 2, xpRequired: 53 },
  { level: 3, xpRequired: 55 },
  { level: 4, xpRequired: 58 },
  { level: 5, xpRequired: 61 },
  { level: 6, xpRequired: 64 },
  { level: 7, xpRequired: 67 },
  { level: 8, xpRequired: 70 },
  { level: 9, xpRequired: 74 },
  { level: 10, xpRequired: 78 },
  { level: 11, xpRequired: 81 },
  { level: 12, xpRequired: 86 },
  { level: 13, xpRequired: 90 },
  { level: 14, xpRequired: 94 },
  { level: 15, xpRequired: 99 },
  { level: 16, xpRequired: 104 },
  { level: 17, xpRequired: 109 },
  { level: 18, xpRequired: 115 },
  { level: 19, xpRequired: 120 },
  { level: 20, xpRequired: 126 },
]

export interface RepLevelResult {
  level: number
  levelReputation: number
  nextLevelXp: number
  totalReputation: number
}

/**
 * 根据累计声望值计算等级 —— 与后端 ComputeRepLevel 逻辑一致。
 * 依次扣除每级所需声望，直到不足以升级。
 */
export function computeRepLevel(totalRep: number): RepLevelResult {
  const sorted = [...PROD_REP_LEVELS].sort((a, b) => a.level - b.level)
  let remaining = totalRep
  let currentLevel = 0
  for (const lvl of sorted) {
    if (lvl.xpRequired <= 0 || remaining < lvl.xpRequired) break
    remaining -= lvl.xpRequired
    currentLevel = lvl.level
  }
  const nextLevelXp = sorted.find(l => l.level === currentLevel + 1)?.xpRequired ?? 0
  return {
    level: currentLevel,
    levelReputation: remaining,
    nextLevelXp,
    totalReputation: totalRep,
  }
}

const STORAGE_KEY = 'cat_village_rep_config'

function loadFromStorage(): CatVillageRepConfig {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return { ...defaults(), ...JSON.parse(raw) }
  } catch {}
  return defaults()
}

function defaults(): CatVillageRepConfig {
  return { maxLevel: 10, baseXp: 100, growthFactor: 1.5 }
}

export const repConfig = reactive<CatVillageRepConfig>(loadFromStorage())

export function saveRepConfig() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...repConfig }))
}

export function resetRepConfig() {
  const d = defaults()
  repConfig.maxLevel = d.maxLevel
  repConfig.baseXp = d.baseXp
  repConfig.growthFactor = d.growthFactor
  saveRepConfig()
}

export function computeLevels(cfg: CatVillageRepConfig): Array<{ level: number; xpRequired: number }> {
  return Array.from({ length: cfg.maxLevel }, (_, i) => ({
    level: i + 1,
    xpRequired: Math.round(cfg.baseXp * Math.pow(cfg.growthFactor, i))
  }))
}
