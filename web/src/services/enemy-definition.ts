import { getGameDataChunk } from "@services/game-data-runtime"
import type { Translate } from "@/types/item-label"
import type { EquipmentAffixSlot, EquipmentSlotType } from "@/types/equipment"

export type EnemyDropRule = {
  itemId: string
  min: number
  max: number
  percent: number
}

export type EnemyDefinition = {
  id: string
  name: string
  nameKey: string
  descKey: string
  position: "vanguard" | "midguard" | "rearguard"
  level: number
  exp: number
  stats: Record<string, number>
  drops: EnemyDropRule[]
  skillIds: string[]
  initialBuffIds: string[]
  equipment: EnemyEquipmentDefinition[]
}

export type EnemyEquipmentDefinition = {
  templateItemId: string
  equippedSlot: string
  equipType: EquipmentSlotType
  enhanceLevel: number
  baseSnapshot?: Record<string, number>
  affixes: EquipmentAffixSlot[]
}

function parseNumber(raw: unknown, fallback = 0): number {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function parseBoolean(raw: unknown, fallback = false): boolean {
  if (typeof raw === "boolean") return raw
  if (typeof raw === "number") return raw !== 0
  if (typeof raw === "string") {
    const normalized = raw.trim().toLowerCase()
    if (normalized === "true" || normalized === "1") return true
    if (normalized === "false" || normalized === "0") return false
  }
  return fallback
}

function toPosition(raw: unknown): "vanguard" | "midguard" | "rearguard" {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "midguard" || value === "rearguard") return value
  return "vanguard"
}

function parseSkillIDs(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return Array.from(new Set(raw.map((item) => String(item || "").trim()).filter(Boolean)))
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    return Array.from(new Set(raw.split(",").map((item) => item.trim()).filter(Boolean)))
  }
  return []
}

function parseDrops(raw: unknown): EnemyDropRule[] {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
  const rows: EnemyDropRule[] = []
  for (const [itemId, value] of Object.entries(raw as Record<string, unknown>)) {
    const normalizedItemID = String(itemId || "").trim()
    if (!normalizedItemID) continue
    const node = value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {}
    const range = node.range && typeof node.range === "object" && !Array.isArray(node.range) ? node.range as Record<string, unknown> : {}
    const min = Math.max(0, Math.trunc(parseNumber(range.min, 0)))
    const max = Math.max(min, Math.trunc(parseNumber(range.max, min)))
    const percent = Math.max(0, Math.min(1, parseNumber(node.percent, 0)))
    rows.push({
      itemId: normalizedItemID,
      min,
      max,
      percent
    })
  }
  rows.sort((a, b) => a.itemId.localeCompare(b.itemId))
  return rows
}

function parseNumberMap(raw: unknown): Record<string, number> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const normalizedKey = String(key || "").trim()
    const numeric = parseNumber(value, Number.NaN)
    if (!normalizedKey || !Number.isFinite(numeric) || numeric === 0) continue
    out[normalizedKey] = numeric
  }
  return Object.keys(out).length > 0 ? out : undefined
}

function parseEquipment(raw: unknown): EnemyEquipmentDefinition[] {
  if (!Array.isArray(raw)) return []
  const rows: EnemyEquipmentDefinition[] = []
  for (const item of raw) {
    if (!item || typeof item !== "object" || Array.isArray(item)) continue
    const node = item as Record<string, unknown>
    const templateItemId = String(node.templateItemId || node.itemId || node.id || "").trim()
    if (!templateItemId) continue
    const affixes = Array.isArray(node.affixes)
      ? node.affixes
          .filter((slot): slot is Record<string, unknown> => Boolean(slot) && typeof slot === "object" && !Array.isArray(slot))
          .map((slot) => ({
            slotIndex: Math.max(0, Math.trunc(parseNumber(slot.slotIndex, 0))),
            affixId: String(slot.affixId || "").trim(),
            rarityId: String(slot.rarityId || "").trim(),
            rollValue: parseNumber(slot.rollValue, 0),
            valuePrecision: Math.trunc(parseNumber(slot.valuePrecision, 0)),
            attrMods: parseNumberMap(slot.attrMods),
            effectType: String(slot.effectType || "").trim(),
            effectParams: slot.effectParams && typeof slot.effectParams === "object" && !Array.isArray(slot.effectParams)
              ? slot.effectParams as Record<string, unknown>
              : undefined
          }))
          .filter((slot) => slot.affixId)
      : []
    rows.push({
      templateItemId,
      equippedSlot: String(node.equippedSlot || "").trim(),
      equipType: String(node.equipType || "battle").trim() === "work" ? "work" : "battle",
      enhanceLevel: Math.max(0, Math.trunc(parseNumber(node.enhanceLevel, 0))),
      baseSnapshot: parseNumberMap(node.baseSnapshot),
      affixes
    })
  }
  return rows
}

function normalizeEnemy(raw: unknown): EnemyDefinition | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  if (!id) return null
  const stats: Record<string, number> = {}
  for (const [key, value] of Object.entries(row)) {
    if (key === "id" || key === "name" || key === "nameKey" || key === "descKey" || key === "position" || key === "level" || key === "exp" || key === "drops" || key === "skillIds" || key === "initialBuffIds" || key === "equipment") {
      continue
    }
    const parsed = parseNumber(value, Number.NaN)
    if (!Number.isFinite(parsed)) continue
    const normalizedKey = String(key || "").trim()
    if (!normalizedKey || normalizedKey.startsWith("_")) continue
    stats[normalizedKey] = parsed
  }
  return {
    id,
    name: String(row.name || "").trim(),
    nameKey: String(row.nameKey || "").trim() || `enemy.${id}.name`,
    descKey: String(row.descKey || "").trim() || `enemy.${id}.desc`,
    position: toPosition(row.position),
    level: Math.max(1, Math.trunc(parseNumber(row.level, 1))),
    exp: Math.max(0, parseNumber(row.exp, 0)),
    stats,
    drops: parseDrops(row.drops),
    skillIds: parseSkillIDs(row.skillIds),
    initialBuffIds: parseSkillIDs(row.initialBuffIds),
    equipment: parseEquipment(row.equipment)
  }
}

let enemyDefinitions: EnemyDefinition[] | null = null
let enemyDefinitionMap: Map<string, EnemyDefinition> | null = null

function ensureEnemyDefinitions() {
  if (enemyDefinitions && enemyDefinitionMap) return
  const enemiesChunk = getGameDataChunk("enemies")
  enemyDefinitions = Array.isArray((enemiesChunk as { enemies?: unknown }).enemies)
    ? ((enemiesChunk as { enemies: unknown[] }).enemies || [])
        .map((row) => ({
          definition: normalizeEnemy(row),
          disabled: parseBoolean((row as Record<string, unknown> | null)?.disabled, false)
        }))
        .filter((row): row is { definition: EnemyDefinition; disabled: boolean } => Boolean(row.definition))
        .filter((row) => !row.disabled)
        .map((row) => row.definition)
    : []
  enemyDefinitionMap = new Map<string, EnemyDefinition>(enemyDefinitions.map((row) => [row.id, row]))
}

export function listEnemyDefinitions(): EnemyDefinition[] {
  ensureEnemyDefinitions()
  return enemyDefinitions!
}

export function getEnemyDefinition(enemyId: string): EnemyDefinition | undefined {
  const normalized = String(enemyId || "").trim()
  if (!normalized) return undefined
  ensureEnemyDefinitions()
  return enemyDefinitionMap!.get(normalized)
}

export function getLocalizedEnemyName(t: Translate, enemyId: string, fallback = ""): string {
  const normalized = String(enemyId || "").trim()
  if (!normalized) return fallback
  const definition = getEnemyDefinition(normalized)
  const key = definition?.nameKey || `enemy.${normalized}.name`
  const localized = t(key)
  if (localized !== key && localized.trim() !== "") return localized
  if (definition?.name) return definition.name
  return fallback || normalized
}

export function getLocalizedEnemyDesc(t: Translate, enemyId: string, fallback = ""): string {
  const normalized = String(enemyId || "").trim()
  if (!normalized) return fallback
  const definition = getEnemyDefinition(normalized)
  const key = definition?.descKey || `enemy.${normalized}.desc`
  const localized = t(key)
  if (localized !== key && localized.trim() !== "") return localized
  return fallback
}
