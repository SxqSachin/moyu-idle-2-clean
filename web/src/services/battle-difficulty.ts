import sharedGlobalConfigData from "@shared-config/shared-global-config.json"

export type CombatDifficultyLevel = {
  level: number
  labelKey: string
  statMultiplier: number
  battleExpBonus: number
  battleDropRateBonus: number
}

export type WorldDifficultyStatBonus = {
  attrKey: string
  base: number
  perLevelMult: number
}

export type WorldDifficultySpecialDrop = {
  itemId: string
  percent: number
  rangeMin: number
  rangeMax: number
  minLevel: number
}

export type WorldDifficultySettings = {
  maxLevel: number
  statBonuses: WorldDifficultyStatBonus[]
  specialDrops: WorldDifficultySpecialDrop[]
}

export type ResolvedWorldDifficultyStatBonus = WorldDifficultyStatBonus & {
  bonus: number
}

type RawDifficultyLevel = {
  level?: unknown
  labelKey?: unknown
  statMultiplierCurve?: unknown
  battleExpBonus?: unknown
  battleDropRateBonus?: unknown
}

const ENEMY_DIFFICULTY_COMBAT_STATS = new Set([
  "hp",
  "mp",
  "hp_regen",
  "mp_regen",
  "strength",
  "dexterity",
  "intelligence",
  "stamina",
  "luck",
  "perception",
  "physics_damage",
  "magical_damage",
  "physics_defence",
  "magical_defence",
  "speed",
  "crit_chance",
  "crit_damage",
  "magic_crit_chance",
  "magic_crit_damage",
  "crit_damage_resist",
  "physics_damage_amplification",
  "magical_damage_amplification",
  "physics_damage_resist",
  "magical_damage_resist",
  "physics_flat_defense_penetration",
  "magicial_flat_defense_penetration",
  "physics_percent_defense_penetration",
  "magicial_percent_defense_penetration"
])

function finiteNumber(raw: unknown, fallback = 0): number {
  const value = Number(raw)
  return Number.isFinite(value) ? value : fallback
}

function objectNode(raw: unknown): Record<string, unknown> | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  return raw as Record<string, unknown>
}

export function resolveDifficultyCurveValue(rawCurve: unknown, level: number): number {
  const curve = objectNode(rawCurve)
  if (!curve) return 1
  const base = finiteNumber(curve.base, 1)
  const step = finiteNumber(curve.step, 0)
  const overrides = objectNode(curve.overrides)
  const override = overrides?.[String(level)]
  if (override !== undefined && override !== null) return finiteNumber(override, base)
  return base + step * (level - 1)
}

function parseCombatDifficultyLevels(): CombatDifficultyLevel[] {
  const rawConfig = sharedGlobalConfigData as Record<string, unknown>
  const combatDifficulty = objectNode(rawConfig.combatDifficulty)
  const rawLevels = Array.isArray(combatDifficulty?.levels) ? combatDifficulty.levels : []
  const byLevel = new Map<number, CombatDifficultyLevel>()

  for (const entry of rawLevels) {
    const row = objectNode(entry) as RawDifficultyLevel | null
    if (!row) continue
    const level = Math.trunc(finiteNumber(row.level, 0))
    if (level < 1) continue
    byLevel.set(level, {
      level,
      labelKey: String(row.labelKey || "").trim() || `game.difficulty.level${level}`,
      statMultiplier: resolveDifficultyCurveValue(row.statMultiplierCurve, level),
      battleExpBonus: finiteNumber(row.battleExpBonus, 0),
      battleDropRateBonus: finiteNumber(row.battleDropRateBonus, 0)
    })
  }

  return Array.from(byLevel.values()).sort((a, b) => a.level - b.level)
}

const COMBAT_DIFFICULTY_LEVELS = parseCombatDifficultyLevels()
const DEFAULT_MAX_LEVEL = Math.max(1, ...COMBAT_DIFFICULTY_LEVELS.map((row) => row.level))

function parseWorldStatBonuses(raw: unknown): WorldDifficultyStatBonus[] {
  if (!Array.isArray(raw)) return []
  const byAttr = new Map<string, WorldDifficultyStatBonus>()
  for (const entry of raw) {
    const row = objectNode(entry)
    if (!row) continue
    const attrKey = String(row.attrKey || "").trim()
    if (!attrKey) continue
    byAttr.set(attrKey, {
      attrKey,
      base: finiteNumber(row.base, 0),
      perLevelMult: finiteNumber(row.perLevelMult, 0)
    })
  }
  return Array.from(byAttr.values())
}

function parseWorldSpecialDrops(raw: unknown): WorldDifficultySpecialDrop[] {
  if (!Array.isArray(raw)) return []
  const out: WorldDifficultySpecialDrop[] = []
  for (const entry of raw) {
    const row = objectNode(entry)
    if (!row) continue
    const itemId = String(row.itemId || "").trim()
    if (!itemId) continue
    const rangeMin = Math.max(0, Math.trunc(finiteNumber(row.rangeMin, 1)))
    const rangeMax = Math.max(rangeMin, Math.trunc(finiteNumber(row.rangeMax, 1)))
    out.push({
      itemId,
      percent: Math.min(1, Math.max(0, finiteNumber(row.percent, 0))),
      rangeMin,
      rangeMax,
      minLevel: Math.max(1, Math.trunc(finiteNumber(row.minLevel, 1)))
    })
  }
  return out
}

export function normalizeWorldDifficultySettings(raw: unknown): WorldDifficultySettings | null {
  const row = objectNode(raw)
  if (!row) return null
  let maxLevel = Math.trunc(finiteNumber(row.maxLevel, DEFAULT_MAX_LEVEL))
  if (maxLevel < 1 || maxLevel > DEFAULT_MAX_LEVEL) maxLevel = DEFAULT_MAX_LEVEL
  return {
    maxLevel,
    statBonuses: parseWorldStatBonuses(row.statBonuses),
    specialDrops: parseWorldSpecialDrops(row.specialDrops)
  }
}

export function listCombatDifficultyLevels(worldSettings?: unknown): CombatDifficultyLevel[] {
  const maxLevel = normalizeWorldDifficultySettings(worldSettings)?.maxLevel ?? DEFAULT_MAX_LEVEL
  return COMBAT_DIFFICULTY_LEVELS.filter((row) => row.level <= maxLevel)
}

export function getCombatDifficultyLevel(level: number, worldSettings?: unknown): CombatDifficultyLevel | null {
  const normalizedLevel = Math.trunc(finiteNumber(level, 0))
  return listCombatDifficultyLevels(worldSettings).find((row) => row.level === normalizedLevel) || null
}

export function resolveWorldDifficultyStatBonuses(
  worldSettings: unknown,
  level: number
): ResolvedWorldDifficultyStatBonus[] {
  const settings = normalizeWorldDifficultySettings(worldSettings)
  const normalizedLevel = Math.trunc(finiteNumber(level, 0))
  if (!settings || normalizedLevel < 1 || normalizedLevel > settings.maxLevel) return []
  return settings.statBonuses.map((row) => ({
    ...row,
    bonus: row.base * (1 + row.perLevelMult * (normalizedLevel - 1))
  }))
}

export function listWorldDifficultySpecialDrops(
  worldSettings: unknown,
  level: number
): WorldDifficultySpecialDrop[] {
  const settings = normalizeWorldDifficultySettings(worldSettings)
  const normalizedLevel = Math.trunc(finiteNumber(level, 0))
  if (!settings || normalizedLevel < 1 || normalizedLevel > settings.maxLevel) return []
  return settings.specialDrops.filter((row) => row.minLevel <= normalizedLevel)
}

export function projectBattleEnemyDifficultyStats(
  baseStats: Record<string, number>,
  level: number,
  worldSettings?: unknown
): Record<string, number> {
  const out: Record<string, number> = {}
  for (const [rawKey, rawValue] of Object.entries(baseStats || {})) {
    const key = String(rawKey || "").trim()
    const value = finiteNumber(rawValue, Number.NaN)
    if (!key || !Number.isFinite(value)) continue
    out[key] = value
  }

  const difficulty = getCombatDifficultyLevel(level, worldSettings)
  if (!difficulty || difficulty.statMultiplier === 1 || difficulty.statMultiplier <= 0) return out
  const bonusByAttr = new Map(
    resolveWorldDifficultyStatBonuses(worldSettings, difficulty.level)
      .map((row) => [row.attrKey, row.bonus] as const)
  )
  for (const [key, value] of Object.entries(out)) {
    if (!ENEMY_DIFFICULTY_COMBAT_STATS.has(key)) continue
    out[key] = value * difficulty.statMultiplier + (bonusByAttr.get(key) || 0)
  }
  return out
}
