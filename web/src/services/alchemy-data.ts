import { getGameDataChunk } from "@services/game-data-runtime"
import { apiGet, apiPost } from "@services/api"
import { getItemDefinition } from "@services/item-definition"
import { ALCHEMY_CONFIG } from "@/config/shared-global-config"

export type AlchemyGroupEntry = {
  itemId: string
  weight: number
  units: number
}

export type AlchemyGroup = {
  id: string
  nameKey: string
  unitBasis: string
  minAlchemyLevel: number
  entries: AlchemyGroupEntry[]
  goldCost: number
  durationSec: number
  alchemyExp: number
  failChanceBase: number
  minFailChance: number
  upgradeChanceBase: number
  upgradeEntries: AlchemyGroupEntry[]
  active: boolean
}

export type AlchemyTraitEffect = {
  attr: string
  valuePerUnit: { min: number; max: number }
}

// One trait packaged per item. Effects[] supports multi-attribute outputs,
// including negative entries (side-effect / penalty).
export type AlchemyTrait = {
  id: string
  itemId: string
  nameKey: string
  kind: "life" | "combat" | string
  family: string
  effectType: string
  effects: AlchemyTraitEffect[]
  unitBasis: number
  maxBasisMultiplier: number
  minAlchemyLevel: number
  failBaseChance: number
  failChancePerLevelAbove: number
  failChancePerExtraBasis: number
  failChanceQuantityCap: number
  maxEffectivePerEquipment: number
  maxEffectivePerBody: number
  goldCostPerUnit: number
  durationSecPerUnit: number
  alchemyExpPerUnit: number
  active: boolean
}

export type AlchemyNameAffix = {
  prefixKey: string
  suffixKey: string
}

export type AlchemyComboRecipe = {
  id: string
  nameKey: string
  descKey: string
  summaryKey?: string
  kind: "life" | "combat" | string
  minAlchemyLevel: number
  inputs: Record<string, number>
  outputAffix?: {
    traitKind?: string
    effectType?: string
    params?: Record<string, unknown>
    attrMods?: Record<string, number>
    valueRange?: { min: number; max: number }
    rarityId?: string
    displayNameKey?: string
    displayNameArgs?: Record<string, unknown>
    battleHooks?: unknown[]
  }
  extraGoldCost: number
  extraDurationSec: number
  failBaseChance: number
  failChancePerLevelAbove: number
  priority: number
  active: boolean
}

export type AlchemyMediumModifiers = {
  valueMultiplier: number
  negativeMultiplier: number
  failChanceDelta: number
  successFloor: number
  maxTraitSlots: number
  familyAffinity: Record<string, number>
}

export type AlchemyMedium = {
  id: string
  itemId: string
  nameKey: string
  descKey: string
  consumePerCraft: number
  minAlchemyLevel: number
  modifiers: AlchemyMediumModifiers
  active: boolean
}

export type AlchemyInheritModifiers = {
  inheritSlots: number
  suppressSlots: number
  ignoreKeepChance: number
  positiveIgnoreKeepChanceBase: number
  positiveIgnoreKeepChanceBonus: number
  negativeIgnoreKeepChanceBase: number
  negativeIgnoreSuppressChanceBonus: number
  inheritCost: AlchemyInheritCost
  suppressCost: AlchemyInheritCost
  hybridMergeDivisor: number
  inbreedMutationChance: number
  inbreedGoodChance: number
  inbreedGoodMultiplierRange: AlchemyInheritMultiplierRange
  inbreedBadMultiplierRange: AlchemyInheritMultiplierRange
  mutationChance: number
  precisionScaling: AlchemyInheritPrecisionScaling
  luckyScaling: AlchemyInheritLuckyScaling
  stabilityScaling: AlchemyInheritStabilityScaling
}

export type AlchemyInheritCost = {
  gold: number
  items: Record<string, number>
}

export type AlchemyInheritMultiplierRange = {
  min: number
  max: number
}

export type AlchemyInheritPrecisionScaling = {
  hybridDivisorReductionPerPoint: number
  hybridDivisorReductionCap: number
}

export type AlchemyInheritLuckyScaling = {
  ignoreKeepChancePerPoint: number
  ignoreKeepChanceCap: number
  inbreedGoodChancePerPoint: number
  inbreedGoodChanceCap: number
}

export type AlchemyInheritStabilityScaling = {
  inbreedMutationChanceReductionPerPoint: number
  inbreedMutationChanceReductionCap: number
  badMultiplierFloorBonusPerPoint: number
  badMultiplierFloorBonusCap: number
}

export type AlchemyInheritMedium = {
  id: string
  itemId: string
  nameKey: string
  descKey: string
  consumePerCraft: number
  minAlchemyLevel: number
  modifiers: AlchemyInheritModifiers
  active: boolean
}

export type AlchemyTransmuteRule = {
  id: string
  itemIds: string[]
  priority: number
  default: boolean
  returnRatio: { min: number; max: number }
  failChanceBase: number
  minFailChance: number
  durationSec: number
  rarityDurationAddSecByRarityIndex: number[]
  alchemyExp: number
  luckyChanceBase: number
  luckyTiers: Array<{ id: string; kind: string; weight: number; value: number; itemId: string; minCount: number; maxCount: number }>
  minAlchemyLevel: number
  active: boolean
}

export type AlchemyDiscoveries = {
  groups: string[]
  traits: string[]
  combos: string[]
}

export type FishingInfusionPreview = {
  fishInstanceId: number
  speciesId: string
  traitKind: "combat" | "life"
  attribute: string
  lengthCm: number
  weightKg: number
  power: number
  mutations?: string[]
  mutationPowerMultiplier: number
  mutationBonusAttributes?: Array<{
    attribute: string
    mutationIds: string[]
    baseRange: { min: number; max: number }
    effectiveRange: { min: number; max: number }
    failureFloor: number
  }>
  baseRange: { min: number; max: number }
  effectiveRange: { min: number; max: number }
  failureFloor: number
  failChance: number
  goldCost: number
  mediumItemId: string
  mediumCount: number
  rulesetVersion: string
}

let alchemyGroups: AlchemyGroup[] | null = null
let itemTraits: AlchemyTrait[] | null = null
let comboRecipes: AlchemyComboRecipe[] | null = null
let transmuteRules: AlchemyTransmuteRule[] | null = null
let alchemyMediums: AlchemyMedium[] | null = null
let alchemyInheritMediums: AlchemyInheritMedium[] | null = null
let groupById: Map<string, AlchemyGroup> | null = null
let traitByItemId: Map<string, AlchemyTrait> | null = null

function ensureAlchemyData() {
  if (
    alchemyGroups &&
    itemTraits &&
    comboRecipes &&
    transmuteRules &&
    alchemyMediums &&
    alchemyInheritMediums &&
    groupById &&
    traitByItemId
  ) return
  alchemyGroups = parseArray(getGameDataChunk("alchemy_groups"), "alchemy_groups").map(normalizeGroup).filter((row): row is AlchemyGroup => !!row)
  itemTraits = parseArray(getGameDataChunk("item_traits"), "item_traits").flatMap(normalizeTraitsForItem)
  comboRecipes = parseArray(getGameDataChunk("alchemy_combo_recipes"), "alchemy_combo_recipes").map(normalizeCombo).filter((row): row is AlchemyComboRecipe => !!row)
  transmuteRules = parseArray(getGameDataChunk("alchemy_transmute_rules"), "alchemy_transmute_rules").map(normalizeTransmuteRule).filter((row): row is AlchemyTransmuteRule => !!row)
  alchemyMediums = parseArray(getGameDataChunk("alchemy_mediums"), "alchemy_mediums").map(normalizeMedium).filter((row): row is AlchemyMedium => !!row)
  alchemyInheritMediums = parseArray(getGameDataChunk("alchemy_inherit_mediums"), "alchemy_inherit_mediums").map(normalizeInheritMedium).filter((row): row is AlchemyInheritMedium => !!row)
  groupById = new Map(alchemyGroups.map((row) => [row.id, row]))
  traitByItemId = new Map(itemTraits.filter((row) => row.active).map((row) => [row.itemId, row]))
}

export function minAlchemyLevelForRarity(rarityId: string | undefined | null): number {
  switch (String(rarityId || "common").trim()) {
    case "uncommon":
      return 8
    case "rare":
      return 15
    case "epic":
      return 20
    case "legendary":
      return 25
    case "mythic":
      return 35
    case "common":
    default:
      return 1
  }
}

export function computeAlchemyLevelDeficitFailDelta(playerLevel: number, requiredLevel: number): number {
  const deficit = Math.max(0, Number(requiredLevel || 0) - Number(playerLevel || 0))
  if (deficit <= 0) return 0
  return Math.max(0, Math.min(ALCHEMY_CONFIG.levelDeficitFailChanceCap, deficit * ALCHEMY_CONFIG.levelDeficitFailChancePerLevel))
}

export function computeAlchemyLevelFailDelta(playerLevel: number, requiredLevel: number, failChancePerLevelAbove = 0): number {
  if (Number(playerLevel || 0) >= Number(requiredLevel || 0)) {
    return -Math.max(0, Number(playerLevel || 0) - Number(requiredLevel || 0)) * Math.max(0, failChancePerLevelAbove)
  }
  return computeAlchemyLevelDeficitFailDelta(playerLevel, requiredLevel)
}

export function listAlchemyGroups(): AlchemyGroup[] {
  ensureAlchemyData()
  return alchemyGroups!.filter((row) => row.active)
}

export function getAlchemyGroup(id: string): AlchemyGroup | undefined {
  ensureAlchemyData()
  return groupById!.get(String(id || "").trim())
}

export function getAlchemyGroupForItem(itemId: string): AlchemyGroup | undefined {
  const normalized = String(itemId || "").trim()
  return listAlchemyGroups().find((group) => group.entries.some((entry) => entry.itemId === normalized))
}

export function isAlchemyTransmutableItem(itemId: string): boolean {
  const definition = getItemDefinition(String(itemId || "").trim())
  if (!definition) return false
  const id = String(itemId || "").toLowerCase()
  if (id.startsWith("map_") || id.startsWith("skillbook_") || id.startsWith("key_") || id.startsWith("world_boss") || id.startsWith("boss_") || id.includes("rumor")) return false
  const category = String(definition.category || "").toLowerCase()
  if (["equipment", "currency", "collectible", "quest", "map", "key", "alchemy_medium"].includes(category)) return false
  const types = Array.isArray(definition.type) ? definition.type.map((row) => String(row).toLowerCase()) : []
  if (types.some((type) => ["currency", "collectible", "quest", "map", "key", "skillbook", "trigger_item"].includes(type))) return false
  return true
}

export function listAlchemyTraits(): AlchemyTrait[] {
  ensureAlchemyData()
  return itemTraits!.filter((row) => row.active)
}

export function getAlchemyTraitForItem(itemId: string): AlchemyTrait | undefined {
  ensureAlchemyData()
  return traitByItemId!.get(String(itemId || "").trim())
}

// Source of truth: items.json `alchemy.nameAffix`.
// Returns null when the item has no alchemy metadata (i.e., not a trait-bearing material).
export function getItemAlchemyNameAffix(itemId: string): AlchemyNameAffix | null {
  const def = getItemDefinition(String(itemId || "").trim())
  const affix = def?.alchemy?.nameAffix
  if (!affix?.prefixKey || !affix?.suffixKey) return null
  return { prefixKey: affix.prefixKey, suffixKey: affix.suffixKey }
}

export function listAlchemyComboRecipes(): AlchemyComboRecipe[] {
  ensureAlchemyData()
  return comboRecipes!.filter((row) => row.active)
}

export function listAlchemyTransmuteRules(): AlchemyTransmuteRule[] {
  ensureAlchemyData()
  return transmuteRules!.filter((row) => row.active)
}

export function selectAlchemyTransmuteRule(rules: AlchemyTransmuteRule[], itemId: string): AlchemyTransmuteRule | undefined {
  const normalized = String(itemId || "").trim()
  const activeRules = rules.filter((rule) => rule.active)
  let selected: AlchemyTransmuteRule | undefined
  for (const rule of activeRules) {
    if (rule.default || !rule.itemIds.includes(normalized)) continue
    if (!selected || rule.priority > selected.priority || (rule.priority === selected.priority && rule.id < selected.id)) {
      selected = rule
    }
  }
  if (selected) return selected
  for (const rule of activeRules) {
    if (!rule.default) continue
    if (!selected || rule.priority > selected.priority || (rule.priority === selected.priority && rule.id < selected.id)) {
      selected = rule
    }
  }
  if (selected) return selected
  // Old catalogs had one active rule with neither default nor itemIds.
  return activeRules.find((rule) => rule.itemIds.length === 0) || activeRules[0]
}

export function getAlchemyTransmuteRuleForItem(itemId: string): AlchemyTransmuteRule | undefined {
  return selectAlchemyTransmuteRule(listAlchemyTransmuteRules(), itemId)
}

export function listAlchemyMediums(): AlchemyMedium[] {
  ensureAlchemyData()
  return alchemyMediums!.filter((row) => row.active)
}

export function getAlchemyMediumByItemId(itemId: string): AlchemyMedium | undefined {
  const id = String(itemId || "").trim()
  ensureAlchemyData()
  return alchemyMediums!.find((row) => row.active && row.itemId === id)
}

export function listAlchemyInheritMediums(): AlchemyInheritMedium[] {
  ensureAlchemyData()
  return alchemyInheritMediums!.filter((row) => row.active)
}

export function getAlchemyInheritMediumByItemId(itemId: string): AlchemyInheritMedium | undefined {
  const id = String(itemId || "").trim()
  ensureAlchemyData()
  return alchemyInheritMediums!.find((row) => row.active && row.itemId === id)
}

// DefaultBenchMaterialSlots mirrors the backend constant: when no medium is
// equipped, the bench accepts this many trait-materials per craft.
export const DEFAULT_BENCH_MATERIAL_SLOTS = 3

export function effectiveBenchSlots(medium: AlchemyMedium | null | undefined): number {
  const v = medium?.modifiers?.maxTraitSlots ?? 0
  return v > 0 ? v : DEFAULT_BENCH_MATERIAL_SLOTS
}

export async function fetchAlchemyDiscoveries(token: string): Promise<AlchemyDiscoveries> {
  const payload = await apiGet<Partial<AlchemyDiscoveries>>("/api/alchemy/discoveries", token)
  return {
    groups: normalizeStringArray(payload.groups),
    traits: normalizeStringArray(payload.traits),
    combos: normalizeStringArray(payload.combos)
  }
}

export async function fetchFishingInfusionPreview(token: string, fishInstanceId: number): Promise<FishingInfusionPreview> {
  return apiPost<FishingInfusionPreview>("/api/alchemy/bench/fish-preview", token, { fishInstanceId })
}

function parseArray(chunk: unknown, key: string): Record<string, unknown>[] {
  const rows = chunk && typeof chunk === "object" ? (chunk as Record<string, unknown>)[key] : null
  return Array.isArray(rows) ? rows.filter((row): row is Record<string, unknown> => !!row && typeof row === "object" && !Array.isArray(row)) : []
}

function normalizeStringArray(raw: unknown): string[] {
  return Array.isArray(raw) ? raw.map((value) => String(value || "").trim()).filter(Boolean) : []
}

function numberValue(raw: unknown, fallback = 0): number {
  const value = Number(raw)
  return Number.isFinite(value) ? value : fallback
}

function numberMap(raw: unknown): Record<string, number> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const normalizedKey = String(key || "").trim()
    const numeric = numberValue(value, Number.NaN)
    if (normalizedKey && Number.isFinite(numeric)) out[normalizedKey] = numeric
  }
  return out
}

function normalizeGroup(raw: Record<string, unknown>): AlchemyGroup | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  const entries = Array.isArray(raw.entries) ? raw.entries : []
  const upgradeEntries = Array.isArray(raw.upgradeEntries) ? raw.upgradeEntries : []
  const normalizeEntries = (rows: unknown[]) =>
    rows
      .map((entry) => {
        if (!entry || typeof entry !== "object" || Array.isArray(entry)) return null
        const row = entry as Record<string, unknown>
        const itemId = String(row.itemId || "").trim()
        if (!itemId) return null
        return { itemId, weight: numberValue(row.weight, 1), units: numberValue(row.units, 1) }
      })
      .filter((entry): entry is AlchemyGroupEntry => !!entry)
  return {
    id,
    nameKey: String(raw.nameKey || `alchemy.group.${id}.name`).trim(),
    unitBasis: String(raw.unitBasis || "").trim(),
    minAlchemyLevel: Math.max(1, Math.round(numberValue(raw.minAlchemyLevel, 1))),
    active: raw.active !== false && raw.disabled !== true,
    goldCost: Math.max(0, Math.round(numberValue(raw.goldCost, 0))),
    durationSec: Math.max(0, numberValue(raw.durationSec, 0)),
    alchemyExp: Math.max(0, numberValue(raw.alchemyExp, 0)),
    failChanceBase: Math.max(0, Math.min(1, numberValue(raw.failChanceBase, 0))),
    minFailChance: Math.max(0, Math.min(1, numberValue(raw.minFailChance, 0))),
    upgradeChanceBase: Math.max(0, numberValue(raw.upgradeChanceBase, 0)),
    entries: normalizeEntries(entries),
    upgradeEntries: normalizeEntries(upgradeEntries)
  }
}

function normalizeTraitsForItem(raw: Record<string, unknown>): AlchemyTrait[] {
  const itemId = String(raw.itemId || "").trim()
  if (!itemId || !Array.isArray(raw.traits)) return []
  return raw.traits
    .map((entry) => {
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) return null
      const row = entry as Record<string, unknown>
      const id = String(row.traitId || row.id || "").trim()
      if (!id) return null
      const effects: AlchemyTraitEffect[] = Array.isArray(row.effects)
        ? row.effects
            .map((eff) => {
              if (!eff || typeof eff !== "object" || Array.isArray(eff)) return null
              const e = eff as Record<string, unknown>
              const attr = String(e.attr || "").trim()
              if (!attr) return null
              const range = e.valuePerUnit && typeof e.valuePerUnit === "object" && !Array.isArray(e.valuePerUnit)
                ? e.valuePerUnit as Record<string, unknown>
                : {}
              return { attr, valuePerUnit: { min: numberValue(range.min, 0), max: numberValue(range.max, 0) } }
            })
            .filter((eff): eff is AlchemyTraitEffect => !!eff)
        : []
      return {
        id,
        itemId,
        nameKey: String(row.nameKey || `alchemy.trait.${id}.name`).trim(),
        kind: String(row.kind || "combat").trim(),
        family: String(row.family || "").trim(),
        effectType: String(row.effectType || "add_stat").trim(),
        effects,
        unitBasis: Math.max(1, Math.round(numberValue(row.unitBasis, 1))),
        maxBasisMultiplier: Math.max(1, Math.round(numberValue(row.maxBasisMultiplier, 1))),
        minAlchemyLevel: Math.max(1, Math.round(numberValue(row.minAlchemyLevel, 1))),
        failBaseChance: numberValue(row.failBaseChance, 0),
        failChancePerLevelAbove: numberValue(row.failChancePerLevelAbove, 0),
        failChancePerExtraBasis: Math.max(0, numberValue(row.failChancePerExtraBasis, 0)),
        failChanceQuantityCap: Math.max(0, Math.min(1, numberValue(row.failChanceQuantityCap, 0))),
        maxEffectivePerEquipment: Math.max(1, Math.round(numberValue(row.maxEffectivePerEquipment, 2))),
        maxEffectivePerBody: Math.max(1, Math.round(numberValue(row.maxEffectivePerBody, 5))),
        goldCostPerUnit: Math.max(0, Math.round(numberValue(row.goldCostPerUnit, 0))),
        durationSecPerUnit: Math.max(0, numberValue(row.durationSecPerUnit, 0)),
        alchemyExpPerUnit: Math.max(0, numberValue(row.alchemyExpPerUnit, 0)),
        active: row.active !== false && row.disabled !== true
      }
    })
    .filter((row): row is NonNullable<typeof row> => !!row)
}

// Compute the per-trait failure chance for a player at given alchemy level &
// crafting bonus. Mirrors the backend formula so UI display matches actual
// roll. Returns a probability in [0, 1].
export function computeTraitFailChance(trait: AlchemyTrait, playerLevel: number, craftSuccessBonus: number, quantity = trait.unitBasis): number {
  const levelDelta = computeAlchemyLevelFailDelta(playerLevel, trait.minAlchemyLevel, trait.failChancePerLevelAbove)
  const quantityDelta = computeTraitQuantityFailDelta(trait, quantity)
  const leveledFail = Math.max(0, Math.min(1, trait.failBaseChance + levelDelta))
  const raw = applyAlchemyStabilityToFailChance(leveledFail + quantityDelta, craftSuccessBonus)
  if (!Number.isFinite(raw)) return 0
  return Math.max(0, Math.min(1, raw))
}

export function computeTraitSuccessChance(trait: AlchemyTrait, playerLevel: number, craftSuccessBonus: number, quantity = trait.unitBasis): number {
  return 1 - computeTraitFailChance(trait, playerLevel, craftSuccessBonus, quantity)
}

// FailChanceBreakdown — each component returned separately so UI can show
// how the final number was computed.
export type FailChanceBreakdown = {
  base: number
  levelDelta: number           // -X% from levels above min
  quantityDelta: number        // +Y% from investing beyond the first basis
  craftSuccessDelta: number    // failure delta after alchemy_craft_success divisor
  mediumDelta: number          // ±Z% from medium failChanceDelta
  successFloorClamp: number    // negative if floor pushed fail down
  foodDelta?: number          // signed food adjustment to failure chance, after clamping
  final: number                // clamped [0, 1]
}

export function computeFailChanceBreakdown(
  trait: AlchemyTrait,
  playerLevel: number,
  craftSuccessBonus: number,
  medium: AlchemyMedium | null,
  quantity = trait.unitBasis
): FailChanceBreakdown {
  const base = trait.failBaseChance
  const levelDelta = computeAlchemyLevelFailDelta(playerLevel, trait.minAlchemyLevel, trait.failChancePerLevelAbove)
  const quantityDelta = computeTraitQuantityFailDelta(trait, quantity)
  const beforeQuantity = Math.max(0, Math.min(1, base + levelDelta))
  const beforeStability = Math.max(0, Math.min(1, beforeQuantity + quantityDelta))
  const afterStability = applyAlchemyStabilityToFailChance(beforeStability, craftSuccessBonus)
  const craftSuccessDelta = afterStability - beforeStability
  const mediumDelta = medium
    ? medium.modifiers.failChanceDelta + computeAlchemyLevelDeficitFailDelta(playerLevel, medium.minAlchemyLevel)
    : 0
  const beforeFloor = Math.max(0, Math.min(1, afterStability + mediumDelta))
  let successFloorClamp = 0
  let final = beforeFloor
  if (medium && medium.modifiers.successFloor > 0) {
    const maxFail = 1 - medium.modifiers.successFloor
    if (final > maxFail) {
      successFloorClamp = maxFail - final
      final = maxFail
    }
  }
  return {
    base,
    levelDelta,
    quantityDelta,
    craftSuccessDelta,
    mediumDelta,
    successFloorClamp,
    final: Math.max(0, Math.min(1, final))
  }
}

export function computeComboFailChanceBreakdown(
  combo: AlchemyComboRecipe,
  playerLevel: number,
  craftSuccessBonus: number,
  medium: AlchemyMedium | null = null,
  extraLevelDeficitDelta = 0
): FailChanceBreakdown {
  const base = combo.failBaseChance
  const levelDelta = computeAlchemyLevelFailDelta(playerLevel, combo.minAlchemyLevel, combo.failChancePerLevelAbove)
  const beforeStability = Math.max(0, Math.min(1, base + levelDelta + Math.max(0, extraLevelDeficitDelta)))
  const final = applyAlchemyStabilityToFailChance(beforeStability, craftSuccessBonus)
  const craftSuccessDelta = final - beforeStability
  const mediumDelta = medium
    ? medium.modifiers.failChanceDelta + computeAlchemyLevelDeficitFailDelta(playerLevel, medium.minAlchemyLevel)
    : 0
  const beforeFloor = Math.max(0, Math.min(1, final + mediumDelta))
  let successFloorClamp = 0
  let adjustedFinal = beforeFloor
  if (medium && medium.modifiers.successFloor > 0) {
    const maxFail = 1 - medium.modifiers.successFloor
    if (adjustedFinal > maxFail) {
      successFloorClamp = maxFail - adjustedFinal
      adjustedFinal = maxFail
    }
  }
  return {
    base,
    levelDelta,
    quantityDelta: 0,
    craftSuccessDelta,
    mediumDelta,
    successFloorClamp,
    final: Math.max(0, Math.min(1, adjustedFinal))
  }
}

export function computeAlchemyInheritIgnoreKeepChance(medium: AlchemyInheritMedium | null | undefined, luckyChance = 0, sourceValue: number | null = null, stability = 0): number {
  const mods = medium?.modifiers
  const add = mods
    ? Math.min(Math.max(0, mods.luckyScaling.ignoreKeepChanceCap), Math.max(0, luckyChance) * Math.max(0, mods.luckyScaling.ignoreKeepChancePerPoint))
    : 0
  if (sourceValue !== null && Number.isFinite(sourceValue) && sourceValue < 0) {
    const base = ALCHEMY_CONFIG.inherit.negativeIgnoreKeepChanceBase
    return Math.max(0, Math.min(1, base * alchemyPositiveRatio(stability) - Math.max(0, mods?.negativeIgnoreSuppressChanceBonus ?? 0)))
  }
  if (sourceValue !== null && Number.isFinite(sourceValue) && sourceValue >= 0) {
    const base = ALCHEMY_CONFIG.inherit.positiveIgnoreKeepChanceBase
    return Math.max(0, Math.min(1, 1 - (1 - base) * alchemyPositiveRatio(stability) + Math.max(0, mods?.positiveIgnoreKeepChanceBonus ?? 0) + add))
  }
  return Math.max(0, Math.min(1, ALCHEMY_CONFIG.inherit.positiveIgnoreKeepChanceBase + Math.max(0, mods?.ignoreKeepChance ?? 0) + add))
}

export function computeAlchemyInheritEffectiveInheritSlots(baseSlots: number, precision = 0, mediumBonusSlots = 0): number {
  const base = Math.max(0, Math.floor(Number(baseSlots) || 0))
  const bonus = Math.max(0, Math.floor(Number(mediumBonusSlots) || 0))
  const scale = Math.max(0, ALCHEMY_CONFIG.inherit.inheritSlotPrecisionScalePerPoint)
  return Math.floor(base * (1 + Math.max(0, precision) * scale) + 1e-9) + bonus
}

export function computeAlchemyInheritEffectiveSuppressSlots(baseSlots: number, stability = 0, mediumBonusSlots = 0): number {
  const base = Math.max(0, Math.floor(Number(baseSlots) || 0))
  const bonus = Math.max(0, Math.floor(Number(mediumBonusSlots) || 0))
  const scale = Math.max(0, ALCHEMY_CONFIG.inherit.suppressSlotStabilityBonusPerPoint)
  return base + Math.floor(Math.max(0, stability) * scale + 1e-9) + bonus
}

function alchemyPositiveRatio(value: number): number {
  const normalized = Number.isFinite(value) && value > 0 ? value : 0
  return 1 / Math.max(0.01, 1 + normalized)
}

export function computeTraitQuantityFailDelta(trait: AlchemyTrait, quantity: number): number {
  const basis = Math.max(1, Math.round(Number(trait.unitBasis) || 1))
  const qty = Math.max(0, Math.trunc(Number(quantity) || 0))
  const extraBasis = Math.max(0, Math.floor(qty / basis) - 1)
  const perBasis = Math.max(0, Number(trait.failChancePerExtraBasis) || 0)
  const cap = Math.max(0, Math.min(1, Number(trait.failChanceQuantityCap) || 0))
  return Math.min(cap, extraBasis * perBasis)
}

const ALCHEMY_QUANTITY_LINEAR_STABILITY_KNEE = 5
const ALCHEMY_QUANTITY_LOG_SCALE = 0.11231056256176605
const ALCHEMY_QUANTITY_LOG_RATE = 4.4875174417026775
const ALCHEMY_QUANTITY_BRIDGE_START = 10
const ALCHEMY_QUANTITY_PURSUIT_START = 30
const ALCHEMY_QUANTITY_SOFT_CAP_START = 100
const ALCHEMY_QUANTITY_SOFT_CAP = 32

export function alchemyStabilityQuantityMultiplier(stability: number): number {
  const normalized = Number(stability)
  if (!Number.isFinite(normalized) || normalized <= 0) return 1
  if (normalized <= ALCHEMY_QUANTITY_LINEAR_STABILITY_KNEE) return 1 + normalized
  if (normalized <= ALCHEMY_QUANTITY_BRIDGE_START) {
    // Preserve the 500% = 6x and 1000% = 8x anchors.
    return 6 + ALCHEMY_QUANTITY_LOG_RATE * Math.log1p(ALCHEMY_QUANTITY_LOG_SCALE * (normalized - ALCHEMY_QUANTITY_LINEAR_STABILITY_KNEE))
  }
  if (normalized <= ALCHEMY_QUANTITY_PURSUIT_START) {
    // Smoothly bridge 1000% = 8x to 3000% = 15x. The endpoint
    // tangent is 1x per 1000%, matching the pursuit segment below.
    const t = (normalized - ALCHEMY_QUANTITY_BRIDGE_START) / (ALCHEMY_QUANTITY_PURSUIT_START - ALCHEMY_QUANTITY_BRIDGE_START)
    return 8 + 6 * t + 7 * t * t - 6 * t * t * t
  }
  if (normalized <= ALCHEMY_QUANTITY_SOFT_CAP_START) {
    return 15 + (normalized - ALCHEMY_QUANTITY_PURSUIT_START) / 10
  }
  // Continue the 1x-per-1000% tangent, then approach the 32x ceiling.
  const x = (normalized - ALCHEMY_QUANTITY_SOFT_CAP_START) / 10
  return ALCHEMY_QUANTITY_SOFT_CAP - 100 / (10 + x)
}

export function computeTraitMaxQuantity(trait: AlchemyTrait, stability: number): number {
  const basis = Math.max(1, Math.round(Number(trait.unitBasis) || 1))
  const baseMax = Math.max(basis, basis * Math.max(1, Math.round(Number(trait.maxBasisMultiplier) || 1)))
  const multiplier = alchemyStabilityQuantityMultiplier(stability)
  return Math.max(basis, Math.floor((baseMax * multiplier) / basis + 1e-9) * basis)
}

export function computeTraitQuantityCeiling(trait: AlchemyTrait): number {
  const basis = Math.max(1, Math.round(Number(trait.unitBasis) || 1))
  return basis * Math.max(1, Math.round(Number(trait.maxBasisMultiplier) || 1)) * ALCHEMY_QUANTITY_SOFT_CAP
}

export function applyAlchemyStabilityToFailChance(failChance: number, stability: number): number {
  const base = Math.max(0, Math.min(1, Number(failChance) || 0))
  const normalized = Number(stability) || 0
  if (normalized >= 0) {
    return Math.max(0, Math.min(1, base / Math.max(0.01, 1 + normalized)))
  }
  return Math.max(0, Math.min(1, base * (1 + Math.abs(normalized))))
}

export function applyAlchemyMinimumLossFailChance(failChance: number, minFailChance = 0.10): number {
  return Math.max(Math.max(0, Math.min(1, Number(minFailChance) || 0)), Math.min(1, Number(failChance) || 0))
}

export function resolveAlchemyMinFailChance(value: number | undefined | null, fallback: number): number {
  const n = Number(value)
  if (Number.isFinite(n)) {
    return Math.max(0, Math.min(1, n))
  }
  return Math.max(0, Math.min(1, Number(fallback) || 0))
}

function normalizeMedium(raw: Record<string, unknown>): AlchemyMedium | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  const itemId = String(raw.itemId || "").trim()
  if (!itemId) return null
  const modsRaw = raw.modifiers && typeof raw.modifiers === "object" && !Array.isArray(raw.modifiers)
    ? raw.modifiers as Record<string, unknown>
    : {}
  const familyAffinityRaw = modsRaw.familyAffinity && typeof modsRaw.familyAffinity === "object" && !Array.isArray(modsRaw.familyAffinity)
    ? modsRaw.familyAffinity as Record<string, unknown>
    : {}
  const familyAffinity: Record<string, number> = {}
  for (const [k, v] of Object.entries(familyAffinityRaw)) {
    const num = Number(v)
    if (Number.isFinite(num)) familyAffinity[k] = num
  }
  return {
    id,
    itemId,
    nameKey: String(raw.nameKey || `alchemy.medium.${id}.name`).trim(),
    descKey: String(raw.descKey || `alchemy.medium.${id}.desc`).trim(),
    consumePerCraft: Math.max(1, Math.round(numberValue(raw.consumePerCraft, 1))),
    minAlchemyLevel: Math.max(1, Math.round(numberValue(raw.minAlchemyLevel, 1))),
    modifiers: {
      valueMultiplier: numberValue(modsRaw.valueMultiplier, 1),
      negativeMultiplier: numberValue(modsRaw.negativeMultiplier, 1),
      failChanceDelta: numberValue(modsRaw.failChanceDelta, 0),
      successFloor: Math.max(0, Math.min(1, numberValue(modsRaw.successFloor, 0))),
      maxTraitSlots: Math.max(0, Math.round(numberValue(modsRaw.maxTraitSlots, 0))),
      familyAffinity
    },
    active: raw.active !== false && raw.disabled !== true
  }
}

function normalizeInheritMedium(raw: Record<string, unknown>): AlchemyInheritMedium | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  const itemId = String(raw.itemId || "").trim()
  if (!itemId) return null
  const modsRaw = raw.modifiers && typeof raw.modifiers === "object" && !Array.isArray(raw.modifiers)
    ? raw.modifiers as Record<string, unknown>
    : {}
  return {
    id,
    itemId,
    nameKey: String(raw.nameKey || `alchemy.inherit.medium.${id}.name`).trim(),
    descKey: String(raw.descKey || `alchemy.inherit.medium.${id}.desc`).trim(),
    consumePerCraft: Math.max(1, Math.round(numberValue(raw.consumePerCraft, 1))),
    minAlchemyLevel: Math.max(1, Math.round(numberValue(raw.minAlchemyLevel, 1))),
    modifiers: {
      inheritSlots: Math.max(0, Math.round(numberValue(modsRaw.inheritSlots, 0))),
      suppressSlots: Math.max(0, Math.round(numberValue(modsRaw.suppressSlots, 0))),
      ignoreKeepChance: Math.max(0, Math.min(1, numberValue(modsRaw.ignoreKeepChance, 0))),
      positiveIgnoreKeepChanceBase: Math.max(0, Math.min(1, numberValue(modsRaw.positiveIgnoreKeepChanceBase, 0.5))),
      positiveIgnoreKeepChanceBonus: Math.max(0, Math.min(1, numberValue(modsRaw.positiveIgnoreKeepChanceBonus, 0))),
      negativeIgnoreKeepChanceBase: Math.max(0, Math.min(1, numberValue(modsRaw.negativeIgnoreKeepChanceBase, 0.9))),
      negativeIgnoreSuppressChanceBonus: Math.max(0, Math.min(1, numberValue(modsRaw.negativeIgnoreSuppressChanceBonus, 0))),
      inheritCost: normalizeInheritCost(modsRaw.inheritCost),
      suppressCost: normalizeInheritCost(modsRaw.suppressCost),
      hybridMergeDivisor: Math.max(1, numberValue(modsRaw.hybridMergeDivisor, 1)),
      inbreedMutationChance: Math.max(0, Math.min(1, numberValue(modsRaw.inbreedMutationChance, 0))),
      inbreedGoodChance: Math.max(0, Math.min(1, numberValue(modsRaw.inbreedGoodChance, 0))),
      inbreedGoodMultiplierRange: normalizeMultiplierRange(modsRaw.inbreedGoodMultiplierRange, 1, 1),
      inbreedBadMultiplierRange: normalizeMultiplierRange(modsRaw.inbreedBadMultiplierRange, 1, 1),
      mutationChance: Math.max(0, Math.min(1, numberValue(modsRaw.mutationChance, 0))),
      precisionScaling: normalizePrecisionScaling(modsRaw.precisionScaling),
      luckyScaling: normalizeLuckyScaling(modsRaw.luckyScaling),
      stabilityScaling: normalizeStabilityScaling(modsRaw.stabilityScaling)
    },
    active: raw.active !== false && raw.disabled !== true
  }
}

function normalizeInheritCost(raw: unknown): AlchemyInheritCost {
  const value = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    gold: Math.max(0, Math.round(numberValue(value.gold, 0))),
    items: numberMap(value.items)
  }
}

function normalizeMultiplierRange(raw: unknown, defaultMin: number, defaultMax: number): AlchemyInheritMultiplierRange {
  const value = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  const min = Math.max(0, numberValue(value.min, defaultMin))
  const max = Math.max(min, numberValue(value.max, defaultMax))
  return { min, max }
}

function normalizePrecisionScaling(raw: unknown): AlchemyInheritPrecisionScaling {
  const value = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    hybridDivisorReductionPerPoint: Math.max(0, numberValue(value.hybridDivisorReductionPerPoint, 0)),
    hybridDivisorReductionCap: Math.max(0, numberValue(value.hybridDivisorReductionCap, 0))
  }
}

function normalizeLuckyScaling(raw: unknown): AlchemyInheritLuckyScaling {
  const value = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    ignoreKeepChancePerPoint: Math.max(0, numberValue(value.ignoreKeepChancePerPoint, 0)),
    ignoreKeepChanceCap: Math.max(0, numberValue(value.ignoreKeepChanceCap, 0)),
    inbreedGoodChancePerPoint: Math.max(0, numberValue(value.inbreedGoodChancePerPoint, 0)),
    inbreedGoodChanceCap: Math.max(0, numberValue(value.inbreedGoodChanceCap, 0))
  }
}

function normalizeStabilityScaling(raw: unknown): AlchemyInheritStabilityScaling {
  const value = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    inbreedMutationChanceReductionPerPoint: Math.max(0, numberValue(value.inbreedMutationChanceReductionPerPoint, 0)),
    inbreedMutationChanceReductionCap: Math.max(0, numberValue(value.inbreedMutationChanceReductionCap, 0)),
    badMultiplierFloorBonusPerPoint: Math.max(0, numberValue(value.badMultiplierFloorBonusPerPoint, 0)),
    badMultiplierFloorBonusCap: Math.max(0, numberValue(value.badMultiplierFloorBonusCap, 0))
  }
}

function normalizeCombo(raw: Record<string, unknown>): AlchemyComboRecipe | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  const output = raw.outputAffix && typeof raw.outputAffix === "object" && !Array.isArray(raw.outputAffix)
    ? raw.outputAffix as Record<string, unknown>
    : {}
  const valueRange = output.valueRange && typeof output.valueRange === "object" && !Array.isArray(output.valueRange)
    ? output.valueRange as Record<string, unknown>
    : {}
  return {
    id,
    nameKey: String(raw.nameKey || `alchemy.combo.${id}.name`).trim(),
    descKey: String(raw.descKey || `alchemy.combo.${id}.desc`).trim(),
    summaryKey: String(raw.summaryKey || "").trim() || undefined,
    kind: String(raw.kind || "").trim(),
    minAlchemyLevel: Math.max(1, Math.round(numberValue(raw.minAlchemyLevel, 1))),
    inputs: numberMap(raw.inputs),
    outputAffix: {
      traitKind: String(output.traitKind || "").trim() || undefined,
      effectType: String(output.effectType || "").trim() || undefined,
      params: output.params && typeof output.params === "object" && !Array.isArray(output.params)
        ? output.params as Record<string, unknown>
        : undefined,
      attrMods: numberMap(output.attrMods || output.params),
      valueRange: { min: numberValue(valueRange.min, 0), max: numberValue(valueRange.max, 0) },
      rarityId: String(output.rarityId || raw.outputRarity || "").trim() || undefined,
      displayNameKey: String(output.displayNameKey || "").trim() || undefined,
      displayNameArgs: output.displayNameArgs && typeof output.displayNameArgs === "object" && !Array.isArray(output.displayNameArgs)
        ? output.displayNameArgs as Record<string, unknown>
        : undefined,
      battleHooks: Array.isArray((output.params as Record<string, unknown> | undefined)?.battleHooks)
        ? (output.params as Record<string, unknown>).battleHooks as unknown[]
        : undefined
    },
    extraGoldCost: Math.max(0, Math.round(numberValue(raw.extraGoldCost, 0))),
    extraDurationSec: Math.max(0, numberValue(raw.extraDurationSec, 0)),
    failBaseChance: numberValue(raw.failBaseChance, 0),
    failChancePerLevelAbove: numberValue(raw.failChancePerLevelAbove, 0),
    priority: Math.round(numberValue(raw.priority, 0)),
    active: raw.active !== false && raw.disabled !== true
  }
}

function normalizeTransmuteRule(raw: Record<string, unknown>): AlchemyTransmuteRule | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  const ratio = raw.returnRatio && typeof raw.returnRatio === "object" && !Array.isArray(raw.returnRatio)
    ? raw.returnRatio as Record<string, unknown>
    : {}
  const tiers = Array.isArray(raw.luckyTiers) ? raw.luckyTiers : []
  return {
    id,
    itemIds: normalizeStringArray(raw.itemIds),
    priority: Math.round(numberValue(raw.priority, 0)),
    default: raw.default === true,
    returnRatio: { min: numberValue(ratio.min ?? ratio.minRatio, 0), max: numberValue(ratio.max ?? ratio.maxRatio, 0) },
    failChanceBase: Math.max(0, Math.min(1, numberValue(raw.failChanceBase, 0))),
    minFailChance: Math.max(0, Math.min(1, numberValue(raw.minFailChance, 0))),
    durationSec: Math.max(0, numberValue(raw.durationSec, 0)),
    rarityDurationAddSecByRarityIndex: Array.isArray(raw.rarityDurationAddSecByRarityIndex)
      ? raw.rarityDurationAddSecByRarityIndex.map((value) => Math.max(0, numberValue(value, 0)))
      : [],
    alchemyExp: Math.max(0, numberValue(raw.alchemyExp, 0)),
    luckyChanceBase: Math.max(0, numberValue(raw.luckyChanceBase, 0)),
    luckyTiers: tiers
      .map((tier) => {
        if (!tier || typeof tier !== "object" || Array.isArray(tier)) return null
        const row = tier as Record<string, unknown>
        const id = String(row.id || "").trim()
        if (!id) return null
        return {
          id,
          kind: String(row.kind || "").trim(),
          weight: numberValue(row.weight ?? row.chance, 0),
          value: numberValue(row.value, 0),
          itemId: String(row.itemId || "").trim(),
          minCount: numberValue(row.minCount ?? row.min, 0),
          maxCount: numberValue(row.maxCount ?? row.max, 0)
        }
      })
      .filter((tier): tier is AlchemyTransmuteRule["luckyTiers"][number] => !!tier),
    minAlchemyLevel: Math.max(1, Math.round(numberValue(raw.minAlchemyLevel, 1))),
    active: raw.active !== false && raw.disabled !== true
  }
}
