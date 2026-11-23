import { ATTRIBUTE_LAYOUT_CONFIG, type AttributeLayoutDomain, type AttributeLayoutSceneId } from "@/config/shared-global-config"
import { getAttributeDomain } from "@services/attribute-display"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"

export type AttributeLayoutCell<T> = {
  key: string
  value: T | null
  placeholder: boolean
}

export type AttributeLayoutSection = {
  key: string
  titleKey: string
  kind: "primary" | "derived"
  columnTitleKeys?: string[]
  rows: AttributeRow[]
  columns: Array<Array<AttributeLayoutCell<AttributeRow>>>
}

type CharacterAttributeGroup = {
  key: string
  titleKey: string
  sections: AttributeLayoutSection[]
}

type AttributeSectionConfig = {
  key: string
  titleKey: string
  match: (row: AttributeRow) => boolean
  sortRows?: (rows: AttributeRow[]) => AttributeRow[]
  buildColumns?: (rows: AttributeRow[]) => Array<Array<AttributeLayoutCell<AttributeRow>>>
  columnTitleKeys?: string[]
}

const BATTLE_MAIN_DETAIL_ORDER = new Map([
  ["hp", 0],
  ["mp", 1],
  ["hp_regen", 2],
  ["mp_regen", 3],
  ["speed", 4]
])

const BATTLE_DERIVED_SECTION_CONFIGS: AttributeSectionConfig[] = [
  {
    key: "main",
    titleKey: "game.attrBattleMainGroup",
    sortRows: sortBattleMainDetailRows,
    match: (row) => {
      const key = normalizeAttrKey(row.key)
      return key === "hp" ||
        key === "hp_regen" ||
        key === "mp" ||
        key === "mp_regen" ||
        key === "speed"
    }
  },
  {
    key: "attack",
    titleKey: "game.attrBattleAttackGroup",
    buildColumns: buildBattleTypedColumns,
    columnTitleKeys: ["game.attrBattlePhysicalGroup", "game.attrBattleMagicalGroup"],
    match: (row) => {
      const key = normalizeAttrKey(row.key)
      if (key.includes("penetration")) return true
      if (key.includes("defence") ||
        key.includes("defense") ||
        key.includes("resist") ||
        key.includes("reduction") ||
        key.includes("mitigation")) {
        return false
      }
      return key === "physics_damage" ||
        key === "magical_damage" ||
        key.includes("amplification") ||
        key.includes("crit") ||
        key.includes("attack")
    }
  },
  {
    key: "defense",
    titleKey: "game.attrBattleDefenseGroup",
    buildColumns: buildBattleTypedColumns,
    columnTitleKeys: ["game.attrBattlePhysicalGroup", "game.attrBattleMagicalGroup", "game.attrBattleCommonGroup"],
    match: (row) => {
      const key = normalizeAttrKey(row.key)
      return key.includes("defence") ||
        key.includes("defense") ||
        key.includes("resist") ||
        key.includes("reduction") ||
        key.includes("mitigation")
    }
  },
  {
    key: "other",
    titleKey: "game.attrBattleOtherGroup",
    match: () => true
  }
]

const WORK_DERIVED_SECTION_CONFIGS: AttributeSectionConfig[] = [
  {
    key: "action",
    titleKey: "game.attrWorkActionGroup",
    match: (row) => {
      const key = normalizeAttrKey(row.key)
      if (key.startsWith("alchemy") || key.startsWith("fishing_")) return false
      return key.endsWith("_speed") || key.includes("_efficiency")
    }
  },
  {
    key: "enhance",
    titleKey: "game.attrWorkEnhanceGroup",
    match: (row) => {
      const key = normalizeAttrKey(row.key)
      return key.includes("enhance") || key.includes("enhancement")
    }
  },
  {
    key: "alchemy",
    titleKey: "game.attrWorkAlchemyGroup",
    match: (row) => normalizeAttrKey(row.key).startsWith("alchemy")
  },
  {
    key: "fishing",
    titleKey: "game.attrWorkFishingGroup",
    match: (row) => normalizeAttrKey(row.key).startsWith("fishing_")
  },
  {
    key: "other",
    titleKey: "game.attrWorkOtherGroup",
    match: () => true
  }
]

function getSceneConfig(scene: AttributeLayoutSceneId) {
  return ATTRIBUTE_LAYOUT_CONFIG.scenes[scene]
}

function normalizeAttrKey(key: string): string {
  return String(key || "").trim().toLowerCase()
}

export function getConfiguredColumnsForScene(
  scene: AttributeLayoutSceneId,
  preferredDomain?: AttributeLayoutDomain,
  options?: { includeBattlePrimary?: boolean }
): string[][] {
  const source = getSceneConfig(scene)
  const includeBattlePrimary = options?.includeBattlePrimary ?? true
  if (preferredDomain === "battlePrimary") {
    return source.battlePrimary.map((column) => [...column])
  }
  if (preferredDomain === "battle") {
    const primary = includeBattlePrimary ? source.battlePrimary.flat() : []
    if (source.battle.length > 1) {
      const [firstColumn = [], ...rest] = source.battle
      return [[...primary, ...firstColumn], ...rest]
    }
    return [[...primary, ...source.battle.flat()]]
  }
  if (preferredDomain === "work") {
    return source.work.map((column) => [...column])
  }
  return [
    [...source.battlePrimary.flat(), ...source.battle.flat()],
    ...source.work.map((column) => [...column])
  ]
}

export function buildLayoutColumns<T extends { key: string }>(
  scene: AttributeLayoutSceneId,
  domain: AttributeLayoutDomain,
  rows: T[],
  options?: { includeBattlePrimary?: boolean }
): Array<Array<AttributeLayoutCell<T>>> {
  const columns = getConfiguredColumnsForScene(scene, domain, options)
  const rowMap = new Map(rows.map((row) => [row.key.toLowerCase(), row]))
  const resolved = columns.map((column, columnIndex) =>
    column.map((token, rowIndex) => {
      const normalized = String(token || "").trim().toLowerCase()
      if (!normalized) {
        return {
          key: `placeholder-${columnIndex}-${rowIndex}`,
          value: null,
          placeholder: true
        }
      }
      const row = rowMap.get(normalized) || null
      return {
        key: normalized || `missing-${columnIndex}-${rowIndex}`,
        value: row,
        placeholder: !row
      }
    })
  )
  return resolved
}

export function sortAttributeKeysForScene(
  scene: AttributeLayoutSceneId,
  keys: string[],
  preferredDomain?: AttributeLayoutDomain
): string[] {
  const source = getConfiguredColumnsForScene(scene, preferredDomain)
  const normalized = keys
    .map((key) => String(key || "").trim())
    .filter(Boolean)
  const uniqueKeys: string[] = []
  const seen = new Set<string>()
  for (const key of normalized) {
    const lowered = key.toLowerCase()
    if (seen.has(lowered)) continue
    seen.add(lowered)
    uniqueKeys.push(key)
  }
  const byDomain = uniqueKeys.filter((key) => {
    if (!preferredDomain) return true
    if (preferredDomain === "battlePrimary") return getAttributeDomain(key) === "battle"
    return getAttributeDomain(key) === preferredDomain
  })
  const configuredOrder = source
    .flatMap((column) => column)
    .map((key) => String(key || "").trim())
    .filter(Boolean)
  const pool = new Map(byDomain.map((key) => [key.toLowerCase(), key]))
  const ordered: string[] = []
  for (const key of configuredOrder) {
    const hit = pool.get(key.toLowerCase())
    if (!hit) continue
    ordered.push(hit)
    pool.delete(key.toLowerCase())
  }
  if (preferredDomain === "battlePrimary") return ordered
  return [...ordered, ...Array.from(pool.values())]
}

export function getScenePreviewLimit(scene: AttributeLayoutSceneId, fallback: number): number {
  const raw = getSceneConfig(scene).previewLimit
  if (!Number.isFinite(raw) || raw === null) return fallback
  return Math.max(1, Math.trunc(raw))
}

function orderRowsByConfiguredKeys(rows: AttributeRow[], configuredKeys: string[]): AttributeRow[] {
  const rowMap = new Map(rows.map((row) => [normalizeAttrKey(row.key), row]))
  const used = new Set<string>()
  const ordered: AttributeRow[] = []
  for (const key of configuredKeys) {
    const normalized = normalizeAttrKey(key)
    if (!normalized || used.has(normalized)) continue
    const row = rowMap.get(normalized)
    if (!row) continue
    ordered.push(row)
    used.add(normalized)
  }
  const remaining = rows
    .filter((row) => !used.has(normalizeAttrKey(row.key)))
    .sort((a, b) => {
      if (a.order !== b.order) return a.order - b.order
      return a.key.localeCompare(b.key)
    })
  return [...ordered, ...remaining]
}

function buildBalancedRowColumns(rows: AttributeRow[], columnCount: number): Array<Array<AttributeLayoutCell<AttributeRow>>> {
  const normalizedColumnCount = Math.max(1, Math.trunc(columnCount || 1))
  const safeRows = rows.filter(Boolean)
  if (normalizedColumnCount <= 1 || safeRows.length <= 1) {
    return [safeRows.map((row, index) => ({ key: `${row.key}-${index}`, value: row, placeholder: false }))]
  }
  const columns: Array<Array<AttributeLayoutCell<AttributeRow>>> = Array.from({ length: normalizedColumnCount }, () => [])
  const rowsPerColumn = Math.ceil(safeRows.length / normalizedColumnCount)
  safeRows.forEach((row, index) => {
    const columnIndex = Math.min(normalizedColumnCount - 1, Math.floor(index / rowsPerColumn))
    columns[columnIndex].push({
      key: `${columnIndex}-${row.key}-${index}`,
      value: row,
      placeholder: false
    })
  })
  return columns
}

function toLayoutCells(rows: AttributeRow[], prefix: string): Array<AttributeLayoutCell<AttributeRow>> {
  return rows.map((row, index) => ({ key: `${prefix}-${row.key}-${index}`, value: row, placeholder: false }))
}

function getBattleTypeBucket(key: string): "physical" | "magical" | "common" {
  const normalized = normalizeAttrKey(key)
  if (normalized.startsWith("magical") || normalized.startsWith("magicial") || normalized.startsWith("magic_")) {
    return "magical"
  }
  if (
    normalized.startsWith("physics") ||
    normalized.startsWith("physical") ||
    normalized === "crit_chance" ||
    normalized === "crit_damage"
  ) {
    return "physical"
  }
  return "common"
}

function buildBattleTypedColumns(rows: AttributeRow[]): Array<Array<AttributeLayoutCell<AttributeRow>>> {
  const physical: AttributeRow[] = []
  const magical: AttributeRow[] = []
  const common: AttributeRow[] = []
  for (const row of rows) {
    const bucket = getBattleTypeBucket(row.key)
    if (bucket === "physical") {
      physical.push(row)
    } else if (bucket === "magical") {
      magical.push(row)
    } else {
      common.push(row)
    }
  }
  const columns = [
    toLayoutCells(physical, "physical"),
    toLayoutCells(magical, "magical")
  ]
  if (common.length > 0) {
    columns.push(toLayoutCells(common, "common"))
  }
  return columns
}

function sortBattleMainDetailRows(rows: AttributeRow[]): AttributeRow[] {
  return [...rows].sort((a, b) => {
    const weightA = BATTLE_MAIN_DETAIL_ORDER.get(normalizeAttrKey(a.key)) ?? 99
    const weightB = BATTLE_MAIN_DETAIL_ORDER.get(normalizeAttrKey(b.key)) ?? 99
    if (weightA !== weightB) return weightA - weightB
    if (a.order !== b.order) return a.order - b.order
    return a.key.localeCompare(b.key)
  })
}

function buildSectionRows(
  rows: AttributeRow[],
  configs: AttributeSectionConfig[]
): AttributeLayoutSection[] {
  const consumed = new Set<string>()
  const sections: CharacterAttributeGroup["sections"] = []
  for (const config of configs) {
    const matchedRows = rows.filter((row) => {
      const normalized = normalizeAttrKey(row.key)
      if (consumed.has(normalized)) return false
      return config.match(row)
    })
    const sectionRows = config.sortRows ? config.sortRows(matchedRows) : matchedRows
    for (const row of sectionRows) {
      consumed.add(normalizeAttrKey(row.key))
    }
    sections.push({
      key: config.key,
      titleKey: config.titleKey,
      kind: "derived",
      columnTitleKeys: config.columnTitleKeys,
      rows: sectionRows,
      columns: config.buildColumns ? config.buildColumns(sectionRows) : buildBalancedRowColumns(sectionRows, 1)
    })
  }
  return sections
}

function isPrimaryAttribute(row: AttributeRow): boolean {
  return row.growthType === "levelable"
}

function buildPrimarySection(rows: AttributeRow[]): AttributeLayoutSection {
  return {
    key: "primary",
    titleKey: "game.attrGrowthLevelable",
    kind: "primary",
    rows,
    columns: buildBalancedRowColumns(rows, 2)
  }
}

function buildDomainSections(
  rows: AttributeRow[],
  derivedConfigs: AttributeSectionConfig[],
  isPrimary: (row: AttributeRow) => boolean = isPrimaryAttribute
): CharacterAttributeGroup["sections"] {
  const primaryRows = rows.filter(isPrimary)
  const derivedRows = rows.filter((row) => !isPrimary(row))
  return [
    buildPrimarySection(primaryRows),
    ...buildSectionRows(derivedRows, derivedConfigs)
  ]
}

export function buildBattleDetailAttributeSections(rows: AttributeRow[]): AttributeLayoutSection[] {
  return buildSectionRows(rows, BATTLE_DERIVED_SECTION_CONFIGS)
}

export function buildCharacterAttributeGroups(rows: AttributeRow[]): CharacterAttributeGroup[] {
  const battleRows = rows.filter((row) => row.category === "battle")
  const workRows = rows.filter((row) => row.category === "work")
  const battlePrimaryKeys = new Set(ATTRIBUTE_LAYOUT_CONFIG.scenes.character_stats.battlePrimary.flat().map(normalizeAttrKey).filter(Boolean))
  const configuredBattleKeys = [
    ...ATTRIBUTE_LAYOUT_CONFIG.scenes.character_stats.battlePrimary.flat(),
    ...ATTRIBUTE_LAYOUT_CONFIG.scenes.character_stats.battle.flat()
  ]
  const configuredWorkKeys = ATTRIBUTE_LAYOUT_CONFIG.scenes.character_stats.work.flat()
  const orderedBattleRows = orderRowsByConfiguredKeys(battleRows, configuredBattleKeys)
  const orderedWorkRows = orderRowsByConfiguredKeys(workRows, configuredWorkKeys)

  return [
    {
      key: "battle",
      titleKey: "game.equipmentTypeBattle",
      sections: buildDomainSections(
        orderedBattleRows,
        BATTLE_DERIVED_SECTION_CONFIGS,
        (row) => battlePrimaryKeys.has(normalizeAttrKey(row.key))
      )
    },
    {
      key: "work",
      titleKey: "game.equipmentTypeWork",
      sections: buildDomainSections(orderedWorkRows, WORK_DERIVED_SECTION_CONFIGS)
    }
  ]
}
