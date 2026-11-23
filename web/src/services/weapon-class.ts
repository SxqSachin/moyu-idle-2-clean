import { getGameDataChunk } from "@services/game-data-runtime"
import type { Translate } from "@/types/item-label"

export type WeaponClassDefinition = {
  id: string
  nameKey: string
  descKey: string
  icon: string
  color: string
  order: number
}

function parseNumber(raw: unknown, fallback = 0): number {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function parseOptionalString(raw: unknown): string {
  return typeof raw === "string" && raw.trim() !== "" ? raw.trim() : ""
}

function getRawWeaponClasses(catalog: unknown): Record<string, unknown>[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { weapon_classes?: unknown }).weapon_classes
  if (!Array.isArray(rows)) return []
  return rows.filter((row): row is Record<string, unknown> => !!row && typeof row === "object")
}

function normalizeWeaponClass(raw: Record<string, unknown>, index: number): WeaponClassDefinition | null {
  const id = parseOptionalString(raw.id)
  if (!id) return null
  return {
    id,
    nameKey: parseOptionalString(raw.nameKey) || `weapon_class.${id}.name`,
    descKey: parseOptionalString(raw.descKey) || `weapon_class.${id}.desc`,
    icon: parseOptionalString(raw.icon),
    color: parseOptionalString(raw.color) || "#909399",
    order: Math.max(1, Math.round(parseNumber(raw.order, index + 1)))
  }
}

let weaponClassDefinitions: WeaponClassDefinition[] | null = null
let weaponClassDefinitionMap: Map<string, WeaponClassDefinition> | null = null

function ensureWeaponClassDefinitions() {
  if (weaponClassDefinitions && weaponClassDefinitionMap) return
  weaponClassDefinitions = getRawWeaponClasses(getGameDataChunk("weapon_classes"))
    .map((row, index) => normalizeWeaponClass(row, index))
    .filter((row): row is WeaponClassDefinition => !!row)
    .sort((a, b) => (a.order - b.order) || a.id.localeCompare(b.id))
  weaponClassDefinitionMap = new Map<string, WeaponClassDefinition>(weaponClassDefinitions.map((row) => [row.id, row]))
}

export function listWeaponClassDefinitions(): WeaponClassDefinition[] {
  ensureWeaponClassDefinitions()
  return weaponClassDefinitions!
}

export function getWeaponClassDefinition(weaponClass: string): WeaponClassDefinition | undefined {
  const normalized = String(weaponClass || "").trim()
  if (!normalized) return undefined
  ensureWeaponClassDefinitions()
  return weaponClassDefinitionMap!.get(normalized)
}

export function getLocalizedWeaponClassLabel(t: Translate, weaponClass: string, fallback = ""): string {
  const normalized = String(weaponClass || "").trim()
  if (!normalized) return fallback
  const definition = getWeaponClassDefinition(normalized)
  const keys = [definition?.nameKey || "", `weapon_class.${normalized}.name`].filter(Boolean)
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return fallback || normalized
}

export function describeWeaponClassList(t: Translate, weaponClasses: string[]): string {
  if (!Array.isArray(weaponClasses) || weaponClasses.length === 0) return ""
  return weaponClasses
    .map((weaponClass) => getLocalizedWeaponClassLabel(t, weaponClass, weaponClass))
    .filter(Boolean)
    .join(" / ")
}
