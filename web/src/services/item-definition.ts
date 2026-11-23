import { getGameDataChunk } from "@services/game-data-runtime"
import { listActionDefinitions } from "@services/action-definition"
import { normalizeHitDamageScaling } from "@services/hit-damage-scaling"
import type { ItemDefinition } from "@/types/game-data"
import type {
  AffixDefinition,
  AffixSmeltRule,
  BasicAttackEffectDefinition,
  BasicAttackSummonPayloadDefinition,
  BasicAttackTargetingDefinition,
  BasicAttackTargetPredicateDefinition,
  BattleUsableItemDefinition,
  EquipmentBattleHookDefinition,
  EquipmentItemDefinition,
  EquipmentItemType,
  EquipmentSlotType,
  ItemTypeDefinition,
  EquipmentTemplateDefinition,
  RarityDefinition,
  RuntimeItemDefinition
} from "@/types/equipment"

type TargetPredicateKind = "compare_attr" | "exclude_enemy_type" | "require_target_has_status" | "exclude_target_has_status" | "target_hp_not_full"
const KNOWN_TARGET_PREDICATE_KINDS = new Set<string>(["compare_attr", "exclude_enemy_type", "require_target_has_status", "exclude_target_has_status", "target_hp_not_full"])

const nonMarketTypes = new Set<string>(["currency"])
const equipmentItemTypes = new Set<EquipmentItemType>(["weapon", "armor", "cloak", "helmet", "pants", "boots", "accessory"])

let gameDataCatalog: Record<string, unknown> | null = null
let randomDefaultAffixItemIds: Set<string> | null = null

function getGameDataCatalog(): Record<string, unknown> {
  if (!gameDataCatalog) {
    gameDataCatalog = {
      ...getGameDataChunk<Record<string, unknown>>("core"),
      ...getGameDataChunk<Record<string, unknown>>("equipment_templates"),
      ...getGameDataChunk<Record<string, unknown>>("rarity_definitions"),
      ...getGameDataChunk<Record<string, unknown>>("item_types"),
      ...getGameDataChunk<Record<string, unknown>>("affix_definitions"),
      ...getGameDataChunk<Record<string, unknown>>("battle_usable_items"),
      ...getGameDataChunk<Record<string, unknown>>("affix_smelt_rules"),
      ...getGameDataChunk<Record<string, unknown>>("affix_carrier_rules")
    }
  }
  return gameDataCatalog
}

function getRandomDefaultAffixItemIds(): Set<string> {
  if (!randomDefaultAffixItemIds) {
    randomDefaultAffixItemIds = new Set<string>()
    for (const action of listActionDefinitions()) {
      const rewards = (action.itemReward || {}) as Record<string, unknown>
      for (const [itemId, rawReward] of Object.entries(rewards)) {
        if (!rawReward || typeof rawReward !== "object") continue
        const grantPolicy = String((rawReward as Record<string, unknown>).grantPolicy || "").trim()
        if (grantPolicy === "rollDefaultAffixes") randomDefaultAffixItemIds.add(itemId)
      }
    }
  }
  return randomDefaultAffixItemIds
}

function isItemDefinition(value: unknown): value is ItemDefinition {
  if (!value || typeof value !== "object") return false
  const item = value as Record<string, unknown>
  const itemTypes = Array.isArray(item.type) ? item.type : []
  return (
    typeof item.id === "string" &&
    itemTypes.length > 0 &&
    itemTypes.every((entry) => typeof entry === "string") &&
    typeof item.rarity === "string" &&
    typeof item.isEquipment === "boolean" &&
    typeof item.stackable === "boolean" &&
    typeof item.carryableInCombat === "boolean" &&
    typeof item.marketListable === "boolean" &&
    !!item.baseAttrs &&
    typeof item.baseAttrs === "object"
  )
}

function getRawItems(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const items = (catalog as { items?: unknown }).items
  return Array.isArray(items) ? items : []
}

function getRawRarityDefinitions(catalog: unknown): Record<string, unknown>[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { rarity_definitions?: unknown }).rarity_definitions
  if (!Array.isArray(rows)) return []
  return rows.filter((row): row is Record<string, unknown> => !!row && typeof row === "object")
}

function getRawItemTypeDefinitions(catalog: unknown): Record<string, unknown>[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { item_types?: unknown }).item_types
  if (!Array.isArray(rows)) return []
  return rows.filter((row): row is Record<string, unknown> => !!row && typeof row === "object")
}

function getRawAffixDefinitions(catalog: unknown): Record<string, unknown>[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { affix_definitions?: unknown }).affix_definitions
  if (!Array.isArray(rows)) return []
  return rows.filter((row): row is Record<string, unknown> => !!row && typeof row === "object")
}

function getRawBattleUsableItems(catalog: unknown): Record<string, unknown>[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { battle_usable_items?: unknown }).battle_usable_items
  if (!Array.isArray(rows)) return []
  return rows.filter((row): row is Record<string, unknown> => !!row && typeof row === "object")
}

function parseStringArray(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return raw.map((value) => String(value).trim()).filter(Boolean)
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    return raw
      .split("|")
      .map((value) => value.trim())
      .filter(Boolean)
  }
  return []
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

function parseNumberMap(raw: unknown): Record<string, number> {
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const out: Record<string, number> = {}
    for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
      const normalizedKey = String(key || "").trim()
      const parsedValue = parseNumber(value, Number.NaN)
      if (!normalizedKey || !Number.isFinite(parsedValue)) continue
      out[normalizedKey] = parsedValue
    }
    return out
  }
  return {}
}

function parseNestedNumberMap(raw: unknown): Record<string, Record<string, number>> {
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const out: Record<string, Record<string, number>> = {}
    for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
      const normalizedKey = String(key || "").trim()
      if (!normalizedKey) continue
      out[normalizedKey] = parseNumberMap(value)
    }
    return out
  }
  return {}
}

type MilestoneAffixPoolEntry = {
  affixId: string
  rarityId: string
  rollValue?: number
}

function parseNestedAffixMilestonePools(raw: unknown): Record<string, MilestoneAffixPoolEntry[]> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  const out: Record<string, MilestoneAffixPoolEntry[]> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const level = String(key || "").trim()
    if (!level) continue
    const rows = Array.isArray(value) ? value : value && typeof value === "object" ? [value] : []
    const pool: MilestoneAffixPoolEntry[] = []
    for (const entry of rows) {
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
      const rawEntry = entry as Record<string, unknown>
      const affixId = String(rawEntry.affixId || "").trim()
      const rarityId = String(rawEntry.rarityId || "").trim()
      if (!affixId || !rarityId) continue
      const rollValueRaw = rawEntry.rollValue
      if (rollValueRaw === undefined || rollValueRaw === null || rollValueRaw === "") {
        pool.push({ affixId, rarityId })
        continue
      }
      const rollValue = parseNumber(rollValueRaw, Number.NaN)
      if (!Number.isFinite(rollValue)) continue
      pool.push({ affixId, rarityId, rollValue })
    }
    if (pool.length > 0) out[level] = pool
  }
  return out
}

function parseBaseAttrs(raw: unknown): Record<string, number> {
  return parseNumberMap(raw)
}

function parseBasicAttackFormulaTerms(raw: unknown): Array<{ attrKey: string; coefficient: number }> {
  if (!Array.isArray(raw)) return []
  return raw
    .map((entry) => {
      if (!entry || typeof entry !== "object") return null
      const term = entry as Record<string, unknown>
      const attrKey = String(term.attrKey || "").trim()
      const coefficient = parseNumber(term.coefficient, Number.NaN)
      if (!attrKey || !Number.isFinite(coefficient)) return null
      return { attrKey, coefficient }
    })
    .filter((entry): entry is { attrKey: string; coefficient: number } => entry !== null)
}

function normalizeEquipType(raw: unknown): EquipmentSlotType {
  return String(raw || "").trim().toLowerCase() === "work" ? "work" : "battle"
}

function normalizeTargetDirection(raw: unknown): "vanguard" | "midguard" | "rearguard" | undefined {
  const value = String(raw || "").trim().toLowerCase()
  if (!value) return undefined
  if (value === "midguard") return "midguard"
  if (value === "rearguard") return "rearguard"
  return value === "vanguard" ? "vanguard" : undefined
}

function normalizeTargetSeekStrategy(raw: unknown): "rear_to_front" | "front_to_rear" | "random_alive" | "none" | undefined {
  const value = String(raw || "").trim().toLowerCase()
  if (!value) return undefined
  if (value === "rear_to_front" || value === "random_alive" || value === "none") return value
  return value === "front_to_rear" ? "front_to_rear" : undefined
}

function normalizeTargetScope(raw: unknown): "ally" | "enemy" | "all" | "self" {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "ally" || value === "enemy" || value === "all" || value === "self") return value
  return "enemy"
}

function normalizeSelectRule(raw: unknown): "hp_max" | "hp_min" | "hp_percent_max" | "hp_percent_min" | "first" | "attr_max" | "attr_min" | "random" | "boss_first" | "self" {
  const value = String(raw || "").trim().toLowerCase()
  if (
    value === "hp_max" ||
    value === "hp_min" ||
    value === "hp_percent_max" ||
    value === "hp_percent_min" ||
    value === "first" ||
    value === "attr_max" ||
    value === "attr_min" ||
    value === "random" ||
    value === "boss_first" ||
    value === "self"
  ) return value
  if (value === "self_first") return "self"
  return "random"
}

function parseCombatFormulaTerms(raw: unknown): Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> {
  if (!Array.isArray(raw)) return []
  return raw
    .map((entry) => {
      if (!entry || typeof entry !== "object") return null
      const term = entry as Record<string, unknown>
      const attrKey = String(term.attrKey || "").trim()
      const coefficient = parseNumber(term.coefficient, Number.NaN)
      if (!attrKey || !Number.isFinite(coefficient)) return null
      const perSkillLevel = parseNumber(term.perSkillLevel, Number.NaN)
      return {
        attrKey,
        coefficient,
        ...(Number.isFinite(perSkillLevel) ? { perSkillLevel } : {})
      }
    })
    .filter((entry): entry is { attrKey: string; coefficient: number; perSkillLevel?: number } => entry !== null)
}

function normalizeBasicAttackTargeting(
  raw: unknown,
  fallback: {
    targetDirection: "vanguard" | "midguard" | "rearguard"
    targetSeekStrategy: "rear_to_front" | "front_to_rear" | "random_alive" | "none"
  }
): BasicAttackTargetingDefinition {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? (raw as Record<string, unknown>) : {}
  const targetPositionsRaw = Array.isArray(row.targetPositions) ? row.targetPositions : []
  const targetPositions = targetPositionsRaw
    .map((value) => normalizeTargetDirection(value))
    .filter((value): value is "vanguard" | "midguard" | "rearguard" => !!value)
  const targetCountRaw = row.targetCount && typeof row.targetCount === "object" && !Array.isArray(row.targetCount)
    ? row.targetCount as Record<string, unknown>
    : {}
  const targetCountMax = Math.round(parseNumber(targetCountRaw.max, Number.NaN))
  const selectRule = normalizeSelectRule(row.selectRule)
  const selectAttrKey = String(row.selectAttrKey || "").trim()
  const targetPredicatesRaw = Array.isArray(row.targetPredicates) ? row.targetPredicates : []
  const targetPredicates = targetPredicatesRaw
    .filter((p): p is Record<string, unknown> => !!p && typeof p === "object" && !Array.isArray(p))
    .flatMap((p): Array<BasicAttackTargetPredicateDefinition> => {
      const kind = String(p.kind || "").trim().toLowerCase()
      if (!KNOWN_TARGET_PREDICATE_KINDS.has(kind)) return []
      const entry: BasicAttackTargetPredicateDefinition = { kind: kind as TargetPredicateKind }
      if (p.actorAttr !== undefined) entry.actorAttr = String(p.actorAttr)
      if (p.targetAttr !== undefined) entry.targetAttr = String(p.targetAttr)
      if (p.operator !== undefined) entry.operator = String(p.operator)
      if (p.enemyType !== undefined) entry.enemyType = String(p.enemyType)
      if (p.statusId !== undefined) entry.statusId = String(p.statusId)
      return [entry]
    })
  return {
    targetPositions: targetPositions.length > 0 ? targetPositions : [fallback.targetDirection],
    secondarySeek: normalizeTargetSeekStrategy(row.secondarySeek) || fallback.targetSeekStrategy,
    targetScope: normalizeTargetScope(row.targetScope),
    selectRule,
    selectAttrKey: selectRule === "attr_max" || selectRule === "attr_min" ? selectAttrKey : "",
    fallbackSelfIfEmpty: parseBoolean(row.fallbackSelfIfEmpty, false),
    targetCount: {
      base: Math.max(1, Math.round(parseNumber(targetCountRaw.base, 1))),
      formula: parseCombatFormulaTerms(targetCountRaw.formula),
      ...(Number.isFinite(targetCountMax) && targetCountMax > 0 ? { max: targetCountMax } : {})
    },
    ...(targetPredicates.length > 0 ? { targetPredicates } : {})
  }
}

function normalizeBasicAttackAmountFormula(raw: unknown): { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> } {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    baseAdd: parseNumber(row.baseAdd, 0),
    terms: parseCombatFormulaTerms(row.terms)
  }
}

function normalizeBasicAttackDamageSpec(raw: unknown): {
  damageType?: "physical" | "magical"
  amountFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  hitCount?: { base: number; formula: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  hitDamageScaling?: { kind: "powered_exponential_decay"; secondHitMultiplier: number; curvePower: number }
  flatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  percentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  physicalFlatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  physicalPercentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  magicalFlatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  magicalPercentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  extraDamageAmplificationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  bonusAgainstTargetTypes?: Record<string, number>
} | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const row = raw as Record<string, unknown>
  const out: {
    damageType?: "physical" | "magical"
    amountFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    hitCount?: { base: number; formula: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    hitDamageScaling?: { kind: "powered_exponential_decay"; secondHitMultiplier: number; curvePower: number }
    flatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    percentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    physicalFlatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    physicalPercentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    magicalFlatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    magicalPercentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    extraDamageAmplificationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    bonusAgainstTargetTypes?: Record<string, number>
  } = {}
  const rawDamageType = String(row.damageType || "").trim().toLowerCase()
  if (rawDamageType === "physical" || rawDamageType === "magical") {
    out.damageType = rawDamageType
  }
  const amountFormula = normalizeBasicAttackAmountFormula(row.amountFormula)
  if (amountFormula.baseAdd !== 0 || amountFormula.terms.length > 0) {
    out.amountFormula = amountFormula
  }
  if (row.hitCount && typeof row.hitCount === "object" && !Array.isArray(row.hitCount)) {
    const hitCountNode = row.hitCount as Record<string, unknown>
    const base = Math.max(1, Math.round(parseNumber(hitCountNode.base, 1)))
    const formula = parseCombatFormulaTerms(hitCountNode.formula)
    if (base !== 1 || formula.length > 0) {
      out.hitCount = { base, formula }
    }
  }
  const hitDamageScaling = normalizeHitDamageScaling(row.hitDamageScaling)
  if (hitDamageScaling) out.hitDamageScaling = hitDamageScaling
  const formulaKeys = [
    "flatDefensePenetrationFormula",
    "percentDefensePenetrationFormula",
    "physicalFlatDefensePenetrationFormula",
    "physicalPercentDefensePenetrationFormula",
    "magicalFlatDefensePenetrationFormula",
    "magicalPercentDefensePenetrationFormula",
    "extraDamageAmplificationFormula"
  ] as const
  for (const key of formulaKeys) {
    const formula = normalizeBasicAttackAmountFormula(row[key])
    if (formula.baseAdd !== 0 || formula.terms.length > 0) {
      out[key] = formula
    }
  }
  if (row.bonusAgainstTargetTypes && typeof row.bonusAgainstTargetTypes === "object" && !Array.isArray(row.bonusAgainstTargetTypes)) {
    const bonusNode = row.bonusAgainstTargetTypes as Record<string, unknown>
    const bonuses: Record<string, number> = {}
    for (const [enemyType, rawValue] of Object.entries(bonusNode)) {
      const key = String(enemyType || "").trim().toLowerCase()
      if (!key) continue
      const value = parseNumber(rawValue, Number.NaN)
      if (!Number.isFinite(value)) continue
      bonuses[key] = value
    }
    if (Object.keys(bonuses).length > 0) out.bonusAgainstTargetTypes = bonuses
  }
  return Object.keys(out).length > 0 ? out : undefined
}

function parseEffectReferenceIndexes(raw: unknown): number[] {
  const refs: number[] = []
  const append = (value: unknown) => {
    const numeric = Math.round(parseNumber(value, Number.NaN))
    if (!Number.isFinite(numeric) || numeric <= 0) return
    if (refs.includes(numeric)) return
    refs.push(numeric)
  }
  if (Array.isArray(raw)) {
    raw.forEach((value) => append(value))
  } else if (typeof raw === "string" && raw.trim() !== "") {
    raw.split(",").forEach((value) => append(value))
  } else if (raw !== undefined && raw !== null) {
    append(raw)
  }
  return refs
}

function parseEffectReferenceIndex(raw: unknown): number | undefined {
  const refs = parseEffectReferenceIndexes(raw)
  return refs.length > 0 ? refs[0] : undefined
}

function normalizeBasicAttackStatusPayload(raw: unknown): NonNullable<BasicAttackEffectDefinition["statusPayload"]> {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  const statusIds = Array.isArray(row.statusIds)
    ? row.statusIds.map((value) => String(value || "").trim()).filter(Boolean)
    : typeof row.statusIds === "string"
      ? row.statusIds.split(",").map((part) => part.trim()).filter(Boolean)
      : []
  const payload = row.payload && typeof row.payload === "object" && !Array.isArray(row.payload)
    ? row.payload as Record<string, unknown>
    : undefined
  const payloadFormulaRaw = row.payloadFormula && typeof row.payloadFormula === "object" && !Array.isArray(row.payloadFormula)
    ? row.payloadFormula as Record<string, unknown>
    : undefined
  const payloadFormula = payloadFormulaRaw
    ? Object.fromEntries(Object.entries(payloadFormulaRaw).map(([key, value]) => [key, normalizeBasicAttackAmountFormula(value)]))
    : undefined
  const durationFormula = row.durationFormula && typeof row.durationFormula === "object" && !Array.isArray(row.durationFormula)
    ? normalizeBasicAttackAmountFormula(row.durationFormula)
    : undefined
  const hasApplyChance = row.applyChanceBase !== undefined || row.applyChance !== undefined || row.applyChanceFormula !== undefined
  const applyChanceFormula = row.applyChanceFormula && typeof row.applyChanceFormula === "object" && !Array.isArray(row.applyChanceFormula)
    ? normalizeBasicAttackAmountFormula(row.applyChanceFormula)
    : undefined
  return {
    statusIds: Array.from(new Set(statusIds)),
    durationBaseTurns: Math.max(0, Math.round(parseNumber(row.durationBaseTurns, 0))),
    durationPerSkillLevelTurns: parseNumber(row.durationPerSkillLevelTurns, 0),
    ...(durationFormula ? { durationFormula } : {}),
    ...(hasApplyChance ? {
      applyChanceBase: parseNumber(row.applyChanceBase ?? row.applyChance, 0),
      applyChancePerSkillLevel: parseNumber(row.applyChancePerSkillLevel, 0),
      ...(applyChanceFormula ? { applyChanceFormula } : {}),
      applyChanceMin: Math.max(0, Math.min(1, parseNumber(row.applyChanceMin, 0))),
      applyChanceMax: Math.max(0, Math.min(1, parseNumber(row.applyChanceMax, 1)))
    } : {}),
    ...(payload ? { payload } : {}),
    ...(payloadFormula ? { payloadFormula } : {})
  }
}

function normalizeBasicAttackSummonPayload(raw: unknown): BasicAttackSummonPayloadDefinition | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const row = raw as Record<string, unknown>
  const unitId = String(row.unitId || "").trim()
  if (!unitId) return null
  const sideRaw = String(row.side || "source").trim().toLowerCase()
  const side: "source" | "player" | "enemy" = sideRaw === "player" || sideRaw === "enemy" ? sideRaw : "source"
  const countFormula = normalizeBasicAttackAmountFormula(row.countFormula)
  if (countFormula.baseAdd === 0 && countFormula.terms.length === 0) countFormula.baseAdd = 1
  const durationFormula = row.durationFormula && typeof row.durationFormula === "object" && !Array.isArray(row.durationFormula)
    ? normalizeBasicAttackAmountFormula(row.durationFormula)
    : undefined
  const maxCountFormula = row.maxCountFormula && typeof row.maxCountFormula === "object" && !Array.isArray(row.maxCountFormula)
    ? normalizeBasicAttackAmountFormula(row.maxCountFormula)
    : undefined
  const statsFormula = normalizePayloadFormulaMap(row.statsFormula)
  const onCasterDeathRaw = String(row.onCasterDeath || "").trim().toLowerCase()
  const onCapOverflowRaw = String(row.onCapOverflow || "").trim().toLowerCase()
  const onDispelRaw = String(row.onDispel || "").trim().toLowerCase()
  return {
    unitId,
    side,
    countFormula,
    ...(durationFormula ? { durationFormula } : {}),
    ...(row.durationTurns !== undefined ? { durationTurns: Math.max(0, Math.round(parseNumber(row.durationTurns, 0))) } : {}),
    ...(row.maxCountPerUnit !== undefined ? { maxCountPerUnit: Math.max(0, Math.round(parseNumber(row.maxCountPerUnit, 0))) } : {}),
    ...(maxCountFormula ? { maxCountFormula } : {}),
    ...(row.persistAcrossBattles !== undefined ? { persistAcrossBattles: parseBoolean(row.persistAcrossBattles, false) } : {}),
    ...(onCasterDeathRaw === "keep_until_duration_end" || onCasterDeathRaw === "despawn_immediately" ? { onCasterDeath: onCasterDeathRaw } : {}),
    ...(onCapOverflowRaw === "replace_oldest" || onCapOverflowRaw === "fail_new_summon" ? { onCapOverflow: onCapOverflowRaw } : {}),
    ...(onDispelRaw === "despawn_immediately" || onDispelRaw === "ignore_dispel" ? { onDispel: onDispelRaw } : {}),
    ...(String(row.onDurationEnd || "").trim().toLowerCase() === "despawn_immediately" ? { onDurationEnd: "despawn_immediately" as const } : {}),
    ...(statsFormula ? { statsFormula } : {})
  }
}

function normalizeWeightedStatusChoices(raw: unknown): NonNullable<BasicAttackEffectDefinition["choices"]> {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry, index) => {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
    const row = entry as Record<string, unknown>
    const statusId = String(row.statusId || "").trim()
    const weight = parseNumber(row.weight, Number.NaN)
    if (!statusId || !Number.isFinite(weight) || weight <= 0) return []
    const id = String(row.id || `choice_${index + 1}`).trim() || `choice_${index + 1}`
    const payload = normalizePlainPayload(row.payload)
    return [{
      id,
      weight,
      statusId,
      durationTurns: Math.max(0, Math.round(parseNumber(row.durationTurns, 0))),
      ...(payload ? { payload } : {})
    }]
  })
}

function normalizeEffectApplyChance(raw: Record<string, unknown>): Partial<Pick<
  BasicAttackEffectDefinition,
  "applyChanceBase" | "applyChancePerSkillLevel" | "applyChanceFormula" | "applyChanceMin" | "applyChanceMax"
>> {
  const hasApplyChance = raw.applyChanceBase !== undefined
    || raw.applyChance !== undefined
    || raw.applyChancePerSkillLevel !== undefined
    || raw.applyChanceFormula !== undefined
    || raw.applyChanceMin !== undefined
    || raw.applyChanceMax !== undefined
  if (!hasApplyChance) return {}
  const applyChanceFormula = raw.applyChanceFormula && typeof raw.applyChanceFormula === "object" && !Array.isArray(raw.applyChanceFormula)
    ? normalizeBasicAttackAmountFormula(raw.applyChanceFormula)
    : undefined
  return {
    applyChanceBase: parseNumber(raw.applyChanceBase ?? raw.applyChance, 0),
    applyChancePerSkillLevel: parseNumber(raw.applyChancePerSkillLevel, 0),
    ...(applyChanceFormula ? { applyChanceFormula } : {}),
    applyChanceMin: Math.max(0, Math.min(1, parseNumber(raw.applyChanceMin, 0))),
    applyChanceMax: Math.max(0, Math.min(1, parseNumber(raw.applyChanceMax, 1)))
  }
}

function normalizePayloadFormulaMap(raw: unknown): Record<string, { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const out = Object.fromEntries(
    Object.entries(raw as Record<string, unknown>)
      .map(([key, value]) => [String(key || "").trim(), normalizeBasicAttackAmountFormula(value)])
      .filter(([key]) => key !== "")
  )
  return Object.keys(out).length > 0 ? out : undefined
}

function normalizePlainPayload(raw: unknown): Record<string, unknown> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const out = Object.fromEntries(
    Object.entries(raw as Record<string, unknown>)
      .map(([key, value]) => [String(key || "").trim(), value])
      .filter(([key]) => key !== "")
  )
  return Object.keys(out).length > 0 ? out : undefined
}

function normalizeBasicAttackEffects(
  raw: unknown,
  fallback: {
    targetDirection?: "vanguard" | "midguard" | "rearguard"
    targetSeekStrategy?: "rear_to_front" | "front_to_rear" | "random_alive" | "none"
    formula?: Array<{ attrKey: string; coefficient: number }>
    damageType?: "physical" | "magical"
  }
): BasicAttackEffectDefinition[] {
  if (!Array.isArray(raw)) return []
  const out: BasicAttackEffectDefinition[] = []
  for (const node of raw) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const row = node as Record<string, unknown>
    const kind = String(row.kind || "").trim().toLowerCase()
    if (
      kind !== "damage" &&
      kind !== "heal" &&
      kind !== "resource" &&
      kind !== "status" &&
      kind !== "summon" &&
      kind !== "adjust_skill_cooldown" &&
      kind !== "weighted_random_per_target"
    ) continue
    const targeting = normalizeBasicAttackTargeting(row.targeting, {
      targetDirection: fallback.targetDirection || "vanguard",
      targetSeekStrategy: fallback.targetSeekStrategy || "front_to_rear"
    })
    const onlyWhenNoTargetFromEffects = parseEffectReferenceIndexes(row.onlyWhenNoTargetFromEffects)
    const reuseTargetsFromEffect = parseEffectReferenceIndex(row.reuseTargetsFromEffect)
    const common = {
      ...(onlyWhenNoTargetFromEffects.length > 0 ? { onlyWhenNoTargetFromEffects } : {}),
      ...(typeof reuseTargetsFromEffect === "number" ? { reuseTargetsFromEffect } : {}),
      ...(row.ignoreUntargetable !== undefined ? { ignoreUntargetable: parseBoolean(row.ignoreUntargetable, false) } : {}),
      ...normalizeEffectStatusLists(row),
      ...normalizeEffectApplyChance(row)
    }
    if (kind === "resource") {
      const resource = String(row.resource || "mp").trim().toLowerCase() || "mp"
      const amountFormula = normalizeBasicAttackAmountFormula(row.amountFormula)
      out.push({
        kind,
        resource,
        ...common,
        targeting,
        amountFormula: { baseAdd: amountFormula.baseAdd, terms: amountFormula.terms }
      })
      continue
    }
    if (kind === "damage" || kind === "heal") {
      const damageTypeRaw = String(row.damageType || "").trim().toLowerCase()
      const damageType = damageTypeRaw === "magical" ? "magical" : "physical"
      const amountFormula = normalizeBasicAttackAmountFormula(row.amountFormula)
      const terms = amountFormula.terms.length > 0
        ? amountFormula.terms
        : (fallback.formula || []).map((term) => ({ attrKey: term.attrKey, coefficient: term.coefficient }))
      const mpCostRaw = row.mpCost && typeof row.mpCost === "object" && !Array.isArray(row.mpCost)
        ? normalizeBasicAttackAmountFormula(row.mpCost)
        : null
      const mpCost = mpCostRaw && (mpCostRaw.baseAdd !== 0 || mpCostRaw.terms.length > 0) ? mpCostRaw : null
      out.push({
        kind,
        damageType: damageTypeRaw === "physical" || damageTypeRaw === "magical" ? damageType : (fallback.damageType || "physical"),
        ...common,
        targeting,
        amountFormula: { baseAdd: amountFormula.baseAdd, terms },
        damageSpec: normalizeBasicAttackDamageSpec(row.damageSpec),
        ...(mpCost ? { mpCost } : {})
      })
      continue
    }
    if (kind === "summon") {
      const summonPayload = normalizeBasicAttackSummonPayload(row.summonPayload)
      if (!summonPayload) continue
      out.push({ kind, ...common, targeting, summonPayload })
      continue
    }
    if (kind === "adjust_skill_cooldown") {
      out.push({
        kind,
        ...common,
        targeting,
        amount: Math.round(parseNumber(row.amount ?? row.turns ?? row.value, 0)),
        includeBasic: parseBoolean(row.includeBasic ?? row.include_basic, false),
        skillIds: parseStringArray(row.skillIds),
        excludeSkillIds: parseStringArray(row.excludeSkillIds)
      })
      continue
    }
    if (kind === "weighted_random_per_target") {
      const choices = normalizeWeightedStatusChoices(row.choices)
      if (choices.length === 0) continue
      out.push({ kind, ...common, targeting, choices })
      continue
    }
    out.push({
      kind,
      ...common,
      targeting,
      statusPayload: normalizeBasicAttackStatusPayload(row.statusPayload)
    })
  }
  return out
}

function normalizeEffectStatusLists(raw: Record<string, unknown>): Partial<Pick<
  BasicAttackEffectDefinition,
  "requiredCasterStatuses" | "requiredTargetStatuses" | "forbiddenCasterStatuses" | "forbiddenTargetStatuses" | "consumeStatusesOnHit" | "consumeCasterStatusesOnHit"
>> {
  const out: Record<string, string[]> = {}
  for (const key of [
    "requiredCasterStatuses",
    "requiredTargetStatuses",
    "forbiddenCasterStatuses",
    "forbiddenTargetStatuses",
    "consumeStatusesOnHit",
    "consumeCasterStatusesOnHit"
  ]) {
    const values = Array.from(new Set(parseStringArray(raw[key])))
    if (values.length > 0) out[key] = values
  }
  return out
}

function normalizeBattleUsableItem(raw: Record<string, unknown>): BattleUsableItemDefinition | null {
  const id = String(raw.id || "").trim()
  const itemId = String(raw.itemId || "").trim()
  if (!id || !itemId) return null
  const isTrigger = raw.isTrigger === true
  const targeting = normalizeBasicAttackTargeting(raw.targeting, {
    targetDirection: "vanguard",
    targetSeekStrategy: "front_to_rear"
  })
  const effects = normalizeBasicAttackEffects(raw.effects, {
    targetDirection: targeting.targetPositions[0] || "vanguard",
    targetSeekStrategy: targeting.secondarySeek,
    damageType: "physical"
  })
  if (effects.length <= 0 && !isTrigger) return null
  const normalizedCooldown = Math.max(
    0,
    Math.round(parseNumber(raw.cooldownTurns, parseNumber(raw.cooldown, 0)))
  )
  const name = String(raw.name || "").trim()
  const icon = String(raw.icon || "").trim()
  const dungeonIds = Array.isArray(raw.dungeonIds) ? raw.dungeonIds.map((d) => String(d)) : undefined
  const spawnEnemyIds = Array.isArray(raw.spawnEnemyIds) ? raw.spawnEnemyIds.map((e) => String(e)) : undefined
  const minWave = typeof raw.minWave === "number" ? raw.minWave : undefined
  const perWaveChance = typeof raw.perWaveChance === "number" ? raw.perWaveChance : undefined
  const intervalWaves = typeof raw.intervalWaves === "number" && raw.intervalWaves > 0 ? raw.intervalWaves : undefined
  const partyTriggerGroup = typeof raw.partyTriggerGroup === "string" ? raw.partyTriggerGroup.trim() : ""
  return {
    id,
    itemId,
    ...(name ? { name } : {}),
    ...(icon ? { icon } : {}),
    cooldownTurns: normalizedCooldown,
    targeting,
    effects,
    ...(isTrigger ? { isTrigger } : {}),
    ...(dungeonIds ? { dungeonIds } : {}),
    ...(spawnEnemyIds ? { spawnEnemyIds } : {}),
    ...(minWave !== undefined ? { minWave } : {}),
    ...(perWaveChance !== undefined ? { perWaveChance } : {}),
    ...(intervalWaves !== undefined ? { intervalWaves } : {}),
    ...(partyTriggerGroup ? { partyTriggerGroup } : {})
  }
}

function normalizeBasicAttackFormula(
  raw: unknown
): { damageType: "physical" | "magical"; formula: Array<{ attrKey: string; coefficient: number }> } | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const row = raw as Record<string, unknown>
  const damageTypeRaw = String(row.damageType || "").trim().toLowerCase()
  if (damageTypeRaw !== "physical" && damageTypeRaw !== "magical") return undefined
  const formula = parseBasicAttackFormulaTerms(row.formula)
  if (formula.length === 0) return undefined
  return {
    damageType: damageTypeRaw,
    formula
  }
}

function normalizeEquipmentBattleHooks(raw: unknown): EquipmentBattleHookDefinition[] {
  if (!Array.isArray(raw)) return []
  const out: EquipmentBattleHookDefinition[] = []
  for (const node of raw) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const row = node as Record<string, unknown>
    const trigger = String(row.trigger || "").trim().toLowerCase()
    if (!trigger) continue
    const conditions = Array.isArray(row.conditions)
      ? row.conditions.filter((entry): entry is Record<string, unknown> => !!entry && typeof entry === "object" && !Array.isArray(entry))
      : []
    const subjectFilterRaw = row.subjectFilter && typeof row.subjectFilter === "object" && !Array.isArray(row.subjectFilter)
      ? row.subjectFilter as Record<string, unknown>
      : null
    const subjectFilter = subjectFilterRaw
      ? {
          ...(typeof subjectFilterRaw.requireAlive === "boolean" ? { requireAlive: subjectFilterRaw.requireAlive } : {}),
          includeUnitTypes: parseStringArray(subjectFilterRaw.includeUnitTypes),
          excludeUnitTypes: parseStringArray(subjectFilterRaw.excludeUnitTypes),
          includeSummonTemplateIds: parseStringArray(subjectFilterRaw.includeSummonTemplateIds),
          excludeSummonTemplateIds: parseStringArray(subjectFilterRaw.excludeSummonTemplateIds)
        }
      : null
    const eventFilterRaw = row.eventFilter && typeof row.eventFilter === "object" && !Array.isArray(row.eventFilter)
      ? row.eventFilter as Record<string, unknown>
      : null
    const eventFilter = eventFilterRaw
      ? {
          ...((eventFilterRaw.buffIds !== undefined || eventFilterRaw.buff_ids !== undefined)
            ? { buffIds: parseStringArray(eventFilterRaw.buffIds ?? eventFilterRaw.buff_ids) }
            : {}),
          ...((eventFilterRaw.excludeBuffIds !== undefined || eventFilterRaw.exclude_buff_ids !== undefined)
            ? { excludeBuffIds: parseStringArray(eventFilterRaw.excludeBuffIds ?? eventFilterRaw.exclude_buff_ids) }
            : {}),
          ...((eventFilterRaw.buffKinds !== undefined || eventFilterRaw.buff_kinds !== undefined)
            ? {
                buffKinds: parseStringArray(eventFilterRaw.buffKinds ?? eventFilterRaw.buff_kinds)
                  .filter((kind): kind is "buff" | "debuff" => kind === "buff" || kind === "debuff")
              }
            : {}),
          ...((typeof eventFilterRaw.firstBuffInstanceOnly === "boolean" || typeof eventFilterRaw.first_buff_instance_only === "boolean")
            ? { firstBuffInstanceOnly: eventFilterRaw.firstBuffInstanceOnly === true || eventFilterRaw.first_buff_instance_only === true }
            : {}),
          ...((eventFilterRaw.applicationKinds !== undefined || eventFilterRaw.application_kinds !== undefined)
            ? {
                applicationKinds: parseStringArray(eventFilterRaw.applicationKinds ?? eventFilterRaw.application_kinds)
                  .filter((kind): kind is "attach" | "refresh" => kind === "attach" || kind === "refresh")
              }
            : {}),
          excludeSelf: eventFilterRaw.excludeSelf === true || eventFilterRaw.exclude_self === true,
          ...((eventFilterRaw.turnKinds !== undefined || eventFilterRaw.turn_kinds !== undefined)
            ? {
                turnKinds: parseStringArray(eventFilterRaw.turnKinds ?? eventFilterRaw.turn_kinds)
                  .filter((kind): kind is "ordered" | "extra" => kind === "ordered" || kind === "extra")
              }
            : {}),
          ...((typeof eventFilterRaw.includePreparation === "boolean" || typeof eventFilterRaw.include_preparation === "boolean")
            ? { includePreparation: eventFilterRaw.includePreparation === true || eventFilterRaw.include_preparation === true }
            : {}),
          ...((typeof eventFilterRaw.includeSuppressedTurns === "boolean" || typeof eventFilterRaw.include_suppressed_turns === "boolean")
            ? { includeSuppressedTurns: eventFilterRaw.includeSuppressedTurns === true || eventFilterRaw.include_suppressed_turns === true }
            : {}),
          ...((eventFilterRaw.probabilityKinds !== undefined || eventFilterRaw.probability_kinds !== undefined)
            ? {
                probabilityKinds: parseStringArray(eventFilterRaw.probabilityKinds ?? eventFilterRaw.probability_kinds)
                  .filter((kind): kind is "equipment_proc" | "status_apply" => kind === "equipment_proc" || kind === "status_apply")
              }
            : {}),
          ...((eventFilterRaw.sourceIds !== undefined || eventFilterRaw.source_ids !== undefined || eventFilterRaw.probabilitySourceIds !== undefined || eventFilterRaw.probability_source_ids !== undefined)
            ? { sourceIds: parseStringArray(eventFilterRaw.sourceIds ?? eventFilterRaw.source_ids ?? eventFilterRaw.probabilitySourceIds ?? eventFilterRaw.probability_source_ids) }
            : {}),
          ...((eventFilterRaw.excludeSourceIds !== undefined || eventFilterRaw.exclude_source_ids !== undefined || eventFilterRaw.excludeProbabilitySourceIds !== undefined || eventFilterRaw.exclude_probability_source_ids !== undefined)
            ? { excludeSourceIds: parseStringArray(eventFilterRaw.excludeSourceIds ?? eventFilterRaw.exclude_source_ids ?? eventFilterRaw.excludeProbabilitySourceIds ?? eventFilterRaw.exclude_probability_source_ids) }
            : {})
        }
      : null
    const effectsRaw = Array.isArray(row.effects) ? row.effects : []
    const effects: Array<{ kind: string; buffId?: string; durationTurns?: number; [key: string]: unknown }> = []
    for (const effectNode of effectsRaw) {
      if (!effectNode || typeof effectNode !== "object" || Array.isArray(effectNode)) continue
      const effect = effectNode as Record<string, unknown>
      const kindRaw = String(effect.kind || effect.type || "").trim().toLowerCase()
      if (!kindRaw) continue
      const probabilityKinds = parseStringArray(effect.probabilityKinds ?? effect.probability_kinds)
        .filter((kind): kind is "equipment_proc" | "status_apply" => kind === "equipment_proc" || kind === "status_apply")
      const sourceIds = parseStringArray(effect.sourceIds ?? effect.source_ids ?? effect.probabilitySourceIds ?? effect.probability_source_ids)
      const excludeSourceIds = parseStringArray(effect.excludeSourceIds ?? effect.exclude_source_ids ?? effect.excludeProbabilitySourceIds ?? effect.exclude_probability_source_ids)
      const probabilityFilters = {
        ...((effect.probabilityKinds !== undefined || effect.probability_kinds !== undefined) ? { probabilityKinds } : {}),
        ...((effect.sourceIds !== undefined || effect.source_ids !== undefined || effect.probabilitySourceIds !== undefined || effect.probability_source_ids !== undefined) ? { sourceIds } : {}),
        ...((effect.excludeSourceIds !== undefined || effect.exclude_source_ids !== undefined || effect.excludeProbabilitySourceIds !== undefined || effect.exclude_probability_source_ids !== undefined) ? { excludeSourceIds } : {})
      }
      if (kindRaw === "add_buff" || kindRaw === "addbuff") {
        const buffId = String(effect.buffId || "").trim()
        if (!buffId) continue
        const durationTurns = Math.max(0, Math.round(parseNumber(effect.durationTurns, 0)))
        const targetModeRaw = String(effect.targetMode || effect.target || "").trim().toLowerCase()
        const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
        const targeting = normalizeBasicAttackTargeting(effect.targeting, {
          targetDirection: "vanguard",
          targetSeekStrategy: "front_to_rear"
        })
        const procChanceBase = parseFiniteNumber(effect.procChanceBase ?? effect.proc_chance_base ?? effect.procChance ?? effect.proc_chance)
        const procChanceMin = parseFiniteNumber(effect.procChanceMin ?? effect.proc_chance_min)
        const procChanceMax = parseFiniteNumber(effect.procChanceMax ?? effect.proc_chance_max)
        const procChanceFormula = normalizeChanceFormula(effect.procChanceFormula ?? effect.proc_chance_formula)
        const payload = normalizePlainPayload(effect.payload)
        const payloadFormula = normalizePayloadFormulaMap(effect.payloadFormula ?? effect.payloadFormulas ?? effect.payload_formula ?? effect.payload_formulas)
        const probabilityGroup = String(effect.probabilityGroup ?? effect.probability_group ?? "").trim()
        const maxSuccessesPerTurn = Math.max(0, Math.round(parseNumber(effect.maxSuccessesPerTurn ?? effect.max_successes_per_turn, 0)))
        effects.push({
          kind: "add_buff",
          buffId,
          ...(durationTurns > 0 ? { durationTurns } : {}),
          targetMode,
          ...(targetMode === "targeting" ? { targeting } : {}),
          ...(procChanceBase !== null ? { procChanceBase } : {}),
          ...(procChanceMin !== null ? { procChanceMin } : {}),
          ...(procChanceMax !== null ? { procChanceMax } : {}),
          ...(procChanceFormula ? { procChanceFormula } : {}),
          ...(payload ? { payload } : {}),
          ...(payloadFormula ? { payloadFormula } : {}),
          ...(probabilityGroup ? { probabilityGroup } : {}),
          ...(maxSuccessesPerTurn > 0 ? { maxSuccessesPerTurn } : {}),
          ...probabilityFilters,
          ...((effect.ignoreProbabilityHooks !== undefined || effect.ignore_probability_hooks !== undefined)
            ? { ignoreProbabilityHooks: parseBoolean(effect.ignoreProbabilityHooks ?? effect.ignore_probability_hooks, false) }
            : {})
        })
        continue
      }
      if (kindRaw === "add_probability" || kindRaw === "set_probability" || kindRaw === "force_probability_failure") {
        const value = parseFiniteNumber(effect.value ?? effect.amount)
        const valueFormula = normalizeChanceFormula(effect.valueFormula ?? effect.value_formula ?? effect.amountFormula ?? effect.amount_formula)
        effects.push({
          kind: kindRaw,
          ...(kindRaw !== "force_probability_failure" && value !== null ? { value } : {}),
          ...(kindRaw !== "force_probability_failure" && valueFormula ? { valueFormula } : {}),
          ...probabilityFilters
        })
        continue
      }
      effects.push({
        ...effect,
        kind: kindRaw,
        ...probabilityFilters,
        ...((effect.probabilityGroup !== undefined || effect.probability_group !== undefined)
          ? { probabilityGroup: String(effect.probabilityGroup ?? effect.probability_group ?? "").trim() }
          : {}),
        ...((effect.maxSuccessesPerTurn !== undefined || effect.max_successes_per_turn !== undefined)
          ? { maxSuccessesPerTurn: Math.max(0, Math.round(parseNumber(effect.maxSuccessesPerTurn ?? effect.max_successes_per_turn, 0))) }
          : {}),
        ...((effect.ignoreProbabilityHooks !== undefined || effect.ignore_probability_hooks !== undefined)
          ? { ignoreProbabilityHooks: parseBoolean(effect.ignoreProbabilityHooks ?? effect.ignore_probability_hooks, false) }
          : {})
      })
    }
    const hideDescription = row.hideDescription === true
    const maxTriggersPerTurn = Math.max(0, Math.round(parseNumber(row.maxTriggersPerTurn ?? row.max_triggers_per_turn, 0)))
    const maxTriggersPerWave = Math.max(0, Math.round(parseNumber(row.maxTriggersPerWave ?? row.max_triggers_per_wave, 0)))
    const maxTriggersPerBattle = Math.max(0, Math.round(parseNumber(row.maxTriggersPerBattle ?? row.max_triggers_per_battle, 0)))
    const sharedTriggerGroup = String(row.sharedTriggerGroup ?? row.shared_trigger_group ?? "").trim()
    const activationCostRaw = row.activationCost ?? row.activation_cost
    const activationCostRow = activationCostRaw && typeof activationCostRaw === "object" && !Array.isArray(activationCostRaw)
      ? activationCostRaw as Record<string, unknown>
      : null
    const maxHpRatio = activationCostRow
      ? Math.max(0, parseNumber(activationCostRow.maxHpRatio ?? activationCostRow.max_hp_ratio, 0))
      : 0
    const maxMpRatio = activationCostRow
      ? Math.max(0, parseNumber(activationCostRow.maxMpRatio ?? activationCostRow.max_mp_ratio, 0))
      : 0
    const activationCost = maxHpRatio > 0 || maxMpRatio > 0
      ? {
          ...(maxHpRatio > 0 ? { maxHpRatio } : {}),
          ...(maxMpRatio > 0 ? { maxMpRatio } : {})
        }
      : null
    const cooldownCompletedTurns = Math.max(0, Math.round(parseNumber(row.cooldownCompletedTurns ?? row.cooldown_completed_turns, 0)))
    if (effects.length === 0 && !hideDescription) continue
    const id = String(row.id || "").trim()
    out.push({
      ...(id ? { id } : {}),
      trigger,
      priority: Math.round(parseNumber(row.priority, 0)),
      ...(activationCost ? { activationCost } : {}),
      ...(cooldownCompletedTurns > 0 ? { cooldownCompletedTurns } : {}),
      ...(subjectFilter ? { subjectFilter } : {}),
      ...(eventFilter ? { eventFilter } : {}),
      conditions,
      effects,
      ...(maxTriggersPerTurn > 0 ? { maxTriggersPerTurn } : {}),
      ...(maxTriggersPerWave > 0 ? { maxTriggersPerWave } : {}),
      ...(maxTriggersPerBattle > 0 ? { maxTriggersPerBattle } : {}),
      ...(sharedTriggerGroup ? { sharedTriggerGroup } : {}),
      ...(hideDescription ? { hideDescription: true } : {})
    })
  }
  return out
}

function parseFiniteNumber(value: unknown): number | null {
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function normalizeChanceFormula(raw: unknown): { baseAdd?: number; terms?: Array<{ attrKey: string; coefficient: number; perEnhanceLevel?: number }> } | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const row = raw as Record<string, unknown>
  const baseAdd = parseFiniteNumber(row.baseAdd)
  const termsRaw = Array.isArray(row.terms) ? row.terms : []
  const terms: Array<{ attrKey: string; coefficient: number; perEnhanceLevel?: number }> = []
  for (const node of termsRaw) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const term = node as Record<string, unknown>
    const attrKey = String(term.attrKey || "").trim()
    const coefficient = parseFiniteNumber(term.coefficient)
    if (!attrKey || coefficient === null || coefficient === 0) continue
    const perEnhanceLevel = parseFiniteNumber(term.perEnhanceLevel ?? term.per_enhance_level)
    terms.push({ attrKey, coefficient, ...(perEnhanceLevel !== null && perEnhanceLevel !== 0 ? { perEnhanceLevel } : {}) })
  }
  if (baseAdd === null && terms.length === 0) return null
  return {
    ...(baseAdd !== null ? { baseAdd } : {}),
    ...(terms.length > 0 ? { terms } : {})
  }
}

function normalizeEquipmentAffixSlots(raw: unknown): Array<{
  slotIndex: number
  affixId: string
  rarityId: string
  sourceKind?: string
  rollValue?: number
  valuePrecision?: number
  attrMods?: Record<string, number>
  effectType?: string
  effectParams?: Record<string, unknown>
  displayNameKey?: string
  displayNameArgs?: Record<string, unknown>
}> {
  if (!Array.isArray(raw)) return []
  const out: Array<{
    slotIndex: number
    affixId: string
    rarityId: string
    sourceKind?: string
    rollValue?: number
    valuePrecision?: number
    attrMods?: Record<string, number>
    effectType?: string
    effectParams?: Record<string, unknown>
    displayNameKey?: string
    displayNameArgs?: Record<string, unknown>
  }> = []
  for (const node of raw) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const row = node as Record<string, unknown>
    const affixId = String(row.affixId || "").trim()
    if (!affixId) continue
    const rarityId = String(row.rarityId || "").trim()
    out.push({
      slotIndex: Math.max(0, Math.round(parseNumber(row.slotIndex, 0))),
      affixId,
      rarityId,
      sourceKind: typeof row.sourceKind === "string" ? row.sourceKind.trim() : undefined,
      rollValue: Number.isFinite(Number(row.rollValue)) ? Number(row.rollValue) : undefined,
      valuePrecision: Number.isFinite(Number(row.valuePrecision)) ? Math.max(0, Math.round(Number(row.valuePrecision))) : undefined,
      attrMods: parseNumberMap(row.attrMods),
      effectType: typeof row.effectType === "string" ? row.effectType.trim() : undefined,
      effectParams:
        row.effectParams && typeof row.effectParams === "object" && !Array.isArray(row.effectParams)
          ? (row.effectParams as Record<string, unknown>)
          : undefined,
      displayNameKey: typeof row.displayNameKey === "string" ? row.displayNameKey.trim() : undefined,
      displayNameArgs:
        row.displayNameArgs && typeof row.displayNameArgs === "object" && !Array.isArray(row.displayNameArgs)
          ? (row.displayNameArgs as Record<string, unknown>)
          : undefined
    })
  }
  return out
}

function isEquipmentTemplateLike(value: unknown): value is Record<string, unknown> {
  if (!value || typeof value !== "object") return false
  const row = value as Record<string, unknown>
  return typeof row.id === "string" && typeof row.slot === "string"
}

function getRawEquipmentTemplates(catalog: unknown): Record<string, unknown>[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { equipment_templates?: unknown }).equipment_templates
  if (!Array.isArray(rows)) return []
  return rows.filter((row): row is Record<string, unknown> => isEquipmentTemplateLike(row))
}

function normalizeEquipmentTemplate(raw: Record<string, unknown>): EquipmentTemplateDefinition | null {
  const id = String(raw.id || "").trim()
  const slot = String(raw.slot || "").trim()
  if (!id || !slot) return null
  const basicAttackTargetDirection = normalizeTargetDirection(raw.basicAttackTargetDirection)
  const targetSeekStrategy = normalizeTargetSeekStrategy(raw.targetSeekStrategy)
  const basicAttackFormula = normalizeBasicAttackFormula(raw.basicAttackFormula)
  return {
    id,
    equipType: normalizeEquipType(raw.equipType),
    equipTargets: parseStringArray(raw.equipTargets),
    slot,
    occupySlots: parseStringArray(raw.occupySlots),
    weaponClass: parseStringArray(raw.weaponClass),
    equipmentSecondaryClass: typeof raw.equipmentSecondaryClass === "string" ? raw.equipmentSecondaryClass.trim() : "",
    statusRequirement: parseNumberMap(raw.statusRequirement),
    accessoryGroup: typeof raw.accessoryGroup === "string" ? raw.accessoryGroup.trim() : "",
	grantedSkill: raw.grantedSkill as EquipmentTemplateDefinition["grantedSkill"],
    affixSlotCount: Math.max(0, Math.round(parseNumber(raw.affixSlotCount, 0))),
    affixAllowedIds: parseStringArray(raw.affixAllowedIds),
    baseAttrs: parseBaseAttrs(raw.baseAttrs),
    maxDurability: Math.max(1, Math.round(parseNumber(raw.maxDurability, 100))),
    decayRate: Math.max(0, parseNumber(raw.decayRate, 0)),
    enhanceMaxLevel: Math.max(0, Math.round(parseNumber(raw.enhanceMaxLevel, 0))),
    enhanceDurationS: Math.max(1, Math.round(parseNumber(raw.enhanceDurationS, 1))),
    enhanceExp: Math.max(0, parseNumber(raw.enhanceExp, 0)),
    enhanceProtectItemId: Array.isArray(raw.enhanceProtectItemId)
      ? raw.enhanceProtectItemId
          .map((item) => String(item || "").trim())
          .filter((item) => item !== "")
      : [],
    enhanceCostCurve: {
      base: parseNumberMap((raw.enhanceCostCurve as Record<string, unknown> | undefined)?.base),
      step: parseNumberMap((raw.enhanceCostCurve as Record<string, unknown> | undefined)?.step),
      overrides: parseNestedNumberMap((raw.enhanceCostCurve as Record<string, unknown> | undefined)?.overrides)
    },
    enhanceAttrCurve: {
      base: parseNumberMap((raw.enhanceAttrCurve as Record<string, unknown> | undefined)?.base),
      step: parseNumberMap((raw.enhanceAttrCurve as Record<string, unknown> | undefined)?.step),
      overrides: parseNestedNumberMap((raw.enhanceAttrCurve as Record<string, unknown> | undefined)?.overrides)
    },
    enhanceAffixMilestonePools: parseNestedAffixMilestonePools(raw.enhanceAffixMilestonePools),
    basicAttackTargetDirection,
    targetSeekStrategy,
    basicAttackFormula,
    basicAttackEffects: normalizeBasicAttackEffects(raw.basicAttackEffects, {
      targetDirection: basicAttackTargetDirection,
      targetSeekStrategy,
      formula: basicAttackFormula?.formula,
      damageType: basicAttackFormula?.damageType
    }),
    battleHooks: normalizeEquipmentBattleHooks(raw.battleHooks),
    attrConversions: normalizeEquipmentAttrConversions(raw.attrConversions),
    defaultAffixSlots: normalizeEquipmentAffixSlots(raw.defaultAffixSlots)
  }
}

function normalizeEquipmentAttrConversions(raw: unknown): EquipmentTemplateDefinition["attrConversions"] {
  if (!Array.isArray(raw) || raw.length === 0) return undefined
  const out: NonNullable<EquipmentTemplateDefinition["attrConversions"]> = []
  for (const node of raw) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const c = node as Record<string, unknown>
    const fromAttr = String(c.fromAttr ?? "").trim()
    const toAttr = String(c.toAttr ?? "").trim()
    if (!fromAttr || !toAttr) continue
    const threshold = typeof c.threshold === "number" ? c.threshold : 1.0
    const ratio = typeof c.ratio === "number" ? c.ratio : undefined
    const rawFormula = c.ratioFormula
    let ratioFormula: NonNullable<EquipmentTemplateDefinition["attrConversions"]>[number]["ratioFormula"]
    if (rawFormula && typeof rawFormula === "object" && !Array.isArray(rawFormula)) {
      const f = rawFormula as Record<string, unknown>
      const baseAdd = typeof f.baseAdd === "number" ? f.baseAdd : 0
      const terms = Array.isArray(f.terms)
        ? f.terms.flatMap((t) => {
            if (!t || typeof t !== "object" || Array.isArray(t)) return []
            const term = t as Record<string, unknown>
            const attrKey = String(term.attrKey ?? "").trim()
            const coefficient = typeof term.coefficient === "number" ? term.coefficient : 0
            const perSkillLevel = typeof term.perSkillLevel === "number" ? term.perSkillLevel : undefined
            return attrKey ? [{ attrKey, coefficient, perSkillLevel }] : []
          })
        : []
      ratioFormula = { baseAdd, terms }
    }
    out.push({ fromAttr, toAttr, threshold, ratio, ratioFormula })
  }
  return out.length > 0 ? out : undefined
}

function normalizeRarityDefinition(raw: Record<string, unknown>, index: number): RarityDefinition | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  return {
    id,
    nameKey: typeof raw.nameKey === "string" && raw.nameKey.trim() ? raw.nameKey.trim() : `rarity.${id}`,
    order: Math.max(1, Math.round(parseNumber(raw.order, index + 1))),
    color: typeof raw.color === "string" ? raw.color.trim() : ""
  }
}

function normalizeItemTypeDefinition(raw: Record<string, unknown>, index: number): ItemTypeDefinition | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  return {
    id,
    nameKey: typeof raw.nameKey === "string" && raw.nameKey.trim() ? raw.nameKey.trim() : `item_type.${id}.name`,
    descKey: typeof raw.descKey === "string" && raw.descKey.trim() ? raw.descKey.trim() : `item_type.${id}.desc`,
    order: Math.max(1, Math.round(parseNumber(raw.order, index + 1)))
  }
}

function normalizeAffixDefinition(raw: Record<string, unknown>): AffixDefinition | null {
  const id = String(raw.id || "").trim()
  if (!id) return null
  const defaultNameKey = id.startsWith("affix.") ? `${id}.name` : `affix.${id}.name`
  const defaultDescKey = id.startsWith("affix.") ? `${id}.desc` : `affix.${id}.desc`
  const effectParams =
    raw.effectParams && typeof raw.effectParams === "object" && !Array.isArray(raw.effectParams)
      ? (raw.effectParams as Record<string, unknown>)
      : {}
  return {
    id,
    // Affix i18n keys are generated from id; source values are ignored.
    nameKey: defaultNameKey,
    descKey: defaultDescKey,
    effectType: typeof raw.effectType === "string" ? raw.effectType.trim() : "",
    effectParams,
    type: normalizeEquipType(raw.type),
    valuePrecision: Math.max(0, Math.round(parseNumber(raw.valuePrecision, 0))),
    maxEffectivePerEquipment: Math.max(0, Math.round(parseNumber(raw.maxEffectivePerEquipment, 0))),
    maxEffectivePerBody: Math.max(0, Math.round(parseNumber(raw.maxEffectivePerBody, 0))),
    allowedRarities: parseStringArray(raw.allowedRarities),
    active: !(raw.active === false || raw.active === 0 || raw.active === "false")
  }
}

let equipmentTemplateDefinitions: EquipmentTemplateDefinition[] | null = null
let equipmentTemplateMap: Map<string, EquipmentTemplateDefinition> | null = null
let rarityDefinitions: RarityDefinition[] | null = null
let rarityDefinitionMap: Map<string, RarityDefinition> | null = null
let itemTypeDefinitions: ItemTypeDefinition[] | null = null
let itemTypeDefinitionMap: Map<string, ItemTypeDefinition> | null = null
let affixDefinitions: AffixDefinition[] | null = null
let affixDefinitionMap: Map<string, AffixDefinition> | null = null
let itemDefinitions: RuntimeItemDefinition[] | null = null
let itemDefinitionMap: Map<string, RuntimeItemDefinition> | null = null
let battleUsableItemDefinitions: BattleUsableItemDefinition[] | null = null
let battleUsableItemByUsableID: Map<string, BattleUsableItemDefinition> | null = null
let battleUsableItemByItemID: Map<string, BattleUsableItemDefinition> | null = null
let carrierItemIdSet: Set<string> | null = null

function toRuntimeItemDefinition(item: ItemDefinition): RuntimeItemDefinition {
  const template = equipmentTemplateMap!.get(item.id)
  const catalogFlag = parseBoolean((item as Partial<RuntimeItemDefinition>).isEquipment, false)
  const isEquipment = catalogFlag || !!template
  return {
    ...item,
    baseAttrs: template?.baseAttrs || item.baseAttrs || {},
    isEquipment
  }
}

function ensureItemDefinitions() {
  if (
    equipmentTemplateDefinitions &&
    equipmentTemplateMap &&
    rarityDefinitions &&
    rarityDefinitionMap &&
    itemTypeDefinitions &&
    itemTypeDefinitionMap &&
    affixDefinitions &&
    affixDefinitionMap &&
    itemDefinitions &&
    itemDefinitionMap &&
    battleUsableItemDefinitions &&
    battleUsableItemByUsableID &&
    battleUsableItemByItemID
  ) return
  const catalog = getGameDataCatalog()
  equipmentTemplateDefinitions = getRawEquipmentTemplates(catalog)
    .map((row) => normalizeEquipmentTemplate(row))
    .filter((row): row is EquipmentTemplateDefinition => !!row)
  equipmentTemplateMap = new Map<string, EquipmentTemplateDefinition>(equipmentTemplateDefinitions.map((row) => [row.id, row]))
  rarityDefinitions = getRawRarityDefinitions(catalog)
    .map((row, index) => normalizeRarityDefinition(row, index))
    .filter((row): row is RarityDefinition => !!row)
    .sort((a, b) => (a.order - b.order) || a.id.localeCompare(b.id))
  rarityDefinitionMap = new Map<string, RarityDefinition>(rarityDefinitions.map((row) => [row.id, row]))
  itemTypeDefinitions = getRawItemTypeDefinitions(catalog)
    .map((row, index) => normalizeItemTypeDefinition(row, index))
    .filter((row): row is ItemTypeDefinition => !!row)
    .sort((a, b) => (a.order - b.order) || a.id.localeCompare(b.id))
  itemTypeDefinitionMap = new Map<string, ItemTypeDefinition>(itemTypeDefinitions.map((row) => [row.id, row]))
  affixDefinitions = getRawAffixDefinitions(catalog)
    .map((row) => normalizeAffixDefinition(row))
    .filter((row): row is AffixDefinition => !!row)
  affixDefinitionMap = new Map<string, AffixDefinition>(affixDefinitions.map((row) => [row.id, row]))
  itemDefinitions = getRawItems(catalog)
    .filter((item): item is ItemDefinition => isItemDefinition(item) && item.id.length > 0)
    .map((item) => toRuntimeItemDefinition(item))
    .filter((item) => !parseBoolean((item as Partial<RuntimeItemDefinition> & { disabled?: unknown }).disabled, false))
  itemDefinitionMap = new Map<string, RuntimeItemDefinition>(itemDefinitions.map((item) => [item.id, item]))
  battleUsableItemDefinitions = getRawBattleUsableItems(catalog)
    .map((row) => normalizeBattleUsableItem(row))
    .filter((row): row is BattleUsableItemDefinition => !!row)
    .filter((row) => itemDefinitionMap!.has(row.itemId))
  battleUsableItemByUsableID = new Map<string, BattleUsableItemDefinition>(
    battleUsableItemDefinitions.map((item) => [item.id, item])
  )
  battleUsableItemByItemID = new Map<string, BattleUsableItemDefinition>(
    battleUsableItemDefinitions.map((item) => [item.itemId, item])
  )
}

export function listItemDefinitions(): RuntimeItemDefinition[] {
  ensureItemDefinitions()
  return itemDefinitions!
}

export function getItemDefinition(itemId: string): RuntimeItemDefinition | undefined {
  ensureItemDefinitions()
  return itemDefinitionMap!.get(itemId)
}

export function listBattleUsableItemDefinitions(): BattleUsableItemDefinition[] {
  ensureItemDefinitions()
  return battleUsableItemDefinitions!
}

export function getBattleUsableItemDefinitionByUsableID(usableId: string): BattleUsableItemDefinition | undefined {
  const normalized = String(usableId || "").trim()
  if (!normalized) return undefined
  ensureItemDefinitions()
  return battleUsableItemByUsableID!.get(normalized)
}

export function getBattleUsableItemDefinitionByItemID(itemId: string): BattleUsableItemDefinition | undefined {
  const normalized = String(itemId || "").trim()
  if (!normalized) return undefined
  ensureItemDefinitions()
  return battleUsableItemByItemID!.get(normalized)
}

export function listEquipmentTemplateDefinitions(): EquipmentTemplateDefinition[] {
  ensureItemDefinitions()
  return equipmentTemplateDefinitions!
}

export function getEquipmentTemplateDefinition(itemId: string): EquipmentTemplateDefinition | undefined {
  ensureItemDefinitions()
  return equipmentTemplateMap!.get(itemId)
}

export function getItemType(itemId: string): string {
  const types = getItemDefinition(itemId)?.type || []
  return types[0] || "unknown"
}

export function getItemTypes(itemId: string): string[] {
  return getItemDefinition(itemId)?.type || []
}

export function listItemTypeDefinitions(): ItemTypeDefinition[] {
  ensureItemDefinitions()
  return itemTypeDefinitions!
}

export function getItemTypeDefinition(itemTypeId: string): ItemTypeDefinition | undefined {
  const normalized = String(itemTypeId || "").trim()
  if (!normalized) return undefined
  ensureItemDefinitions()
  return itemTypeDefinitionMap!.get(normalized)
}

export function orderItemTypes(itemTypes: Iterable<string>): string[] {
  const normalizedIDs = new Set<string>()
  for (const itemType of itemTypes) {
    const normalized = String(itemType || "").trim()
    if (!normalized || normalized === "unknown") continue
    normalizedIDs.add(normalized)
  }
  const known: string[] = []
  const unknown: string[] = []
  ensureItemDefinitions()
  for (const id of normalizedIDs) {
    if (itemTypeDefinitionMap!.has(id)) {
      known.push(id)
    } else {
      unknown.push(id)
    }
  }
  known.sort((a, b) => {
    const orderA = getItemTypeDefinition(a)?.order || Number.MAX_SAFE_INTEGER
    const orderB = getItemTypeDefinition(b)?.order || Number.MAX_SAFE_INTEGER
    if (orderA !== orderB) return orderA - orderB
    return a.localeCompare(b)
  })
  unknown.sort((a, b) => a.localeCompare(b))
  return [...known, ...unknown]
}

export function isEquipmentItemType(itemType: string): itemType is EquipmentItemType {
  return equipmentItemTypes.has(itemType as EquipmentItemType)
}

export function resolveAllowedEquipSlots(template: EquipmentTemplateDefinition | undefined): string[] {
  if (!template) return []
  if (template.slot === "weapon") return ["left_hand", "right_hand", "right_hand_2"]
  if (template.slot === "right_hand" && (template.occupySlots?.length || 0) === 0) {
    return ["right_hand", "right_hand_2"]
  }
  if (template.slot === "accessory") {
    return ["accessory_1", "accessory_2", "accessory_3", "accessory_4", "accessory_5", "accessory_6", "accessory_7", "accessory_8"]
  }
  if (["head", "body", "arm", "feet"].includes(template.slot)) return [template.slot, `${template.slot}_2`]
  return [template.slot]
}

export function getEquipmentAllowedSlots(itemId: string): string[] {
  return resolveAllowedEquipSlots(getEquipmentTemplateDefinition(itemId))
}

export function getEquipmentEquipType(itemId: string): EquipmentSlotType {
  return getEquipmentTemplateDefinition(itemId)?.equipType || "battle"
}

export function getEquipmentStatusRequirement(itemId: string): Record<string, number> {
  return getEquipmentTemplateDefinition(itemId)?.statusRequirement || {}
}

export function listRarityDefinitions(): RarityDefinition[] {
  ensureItemDefinitions()
  return rarityDefinitions!
}

export function getRarityDefinition(rarityId: string): RarityDefinition | undefined {
  const normalized = String(rarityId || "").trim()
  if (!normalized) return undefined
  ensureItemDefinitions()
  return rarityDefinitionMap!.get(normalized)
}

export function getRarityOrder(rarityId: string): number {
  const normalized = String(rarityId || "").trim()
  if (!normalized) return Number.MAX_SAFE_INTEGER
  return getRarityDefinition(normalized)?.order || Number.MAX_SAFE_INTEGER
}

export function compareRarityIDs(a: string, b: string, direction: "asc" | "desc" = "desc"): number {
  const orderA = getRarityOrder(a)
  const orderB = getRarityOrder(b)
  if (orderA !== orderB) return direction === "asc" ? orderA - orderB : orderB - orderA
  return String(a || "").localeCompare(String(b || ""))
}

export function compareItemIDsByRarity(itemIDA: string, itemIDB: string, direction: "asc" | "desc" = "desc"): number {
  const rarityA = getItemDefinition(itemIDA)?.rarity || ""
  const rarityB = getItemDefinition(itemIDB)?.rarity || ""
  return compareRarityIDs(rarityA, rarityB, direction)
}

export function getRarityNameKey(rarityId: string): string {
  const normalized = String(rarityId || "").trim()
  if (!normalized) return ""
  const baseKey = getRarityDefinition(normalized)?.nameKey || `rarity.${normalized}`
  return baseKey.endsWith(".name") ? baseKey : `${baseKey}.name`
}

export function listAffixDefinitions(): AffixDefinition[] {
  ensureItemDefinitions()
  return affixDefinitions!
}

export function getAffixDefinition(affixId: string): AffixDefinition | undefined {
  const normalized = String(affixId || "").trim()
  if (!normalized) return undefined
  ensureItemDefinitions()
  return affixDefinitionMap!.get(normalized)
}

export function asEquipmentItemDefinition(item: RuntimeItemDefinition | undefined): EquipmentItemDefinition | undefined {
  if (!item) return undefined
  const template = getEquipmentTemplateDefinition(item.id)
  if (!template) return undefined

  return {
    ...template,
    ...item,
    isEquipment: true,
    type: Array.isArray(item.type) ? item.type.slice() : []
  }
}

export function getEquipmentItemDefinition(itemId: string): EquipmentItemDefinition | undefined {
  return asEquipmentItemDefinition(getItemDefinition(itemId))
}

export function isEquipmentItem(itemId: string): boolean {
  return !!getEquipmentTemplateDefinition(itemId)
}

export function isItemMarketListable(itemId: string): boolean {
  const definition = getItemDefinition(itemId)
  if (!definition) return false
  if (definition.marketPolicy) return definition.marketPolicy.auction === true && definition.marketPolicy.fixedPrice?.mode === "instance_variants"
  if (definition.marketListable === false) return false
  return !definition.type.some((entry) => nonMarketTypes.has(entry))
}

export function itemHasRandomDefaultAffixGrant(itemId: string): boolean {
  return getRandomDefaultAffixItemIds().has(String(itemId || "").trim())
}

export function isItemFixedMarketListable(itemId: string): boolean {
  if (getItemDefinition(itemId)?.marketPolicy) return false
  if (!isItemMarketListable(itemId)) return false
  return !itemHasRandomDefaultAffixGrant(itemId)
}

function getCarrierItemIdSet(): Set<string> {
  if (carrierItemIdSet) return carrierItemIdSet
  const catalog = getGameDataCatalog()
  const rules = Array.isArray(catalog.affix_carrier_rules) ? catalog.affix_carrier_rules : []
  const ids = new Set<string>()
  for (const row of rules) {
    if (row && typeof row === "object") {
      const itemId = String((row as Record<string, unknown>).carrierItemId || "").trim()
      const active = (row as Record<string, unknown>).active !== false
      if (itemId && active) ids.add(itemId)
    }
  }
  carrierItemIdSet = ids
  return carrierItemIdSet
}

export function isCarrierItemId(itemId: string): boolean {
  return getCarrierItemIdSet().has(itemId)
}

function isAffixSmeltRule(value: unknown): value is AffixSmeltRule {
  if (!value || typeof value !== "object") return false
  const row = value as Record<string, unknown>
  return typeof row.id === "string" && typeof row.active === "boolean" && !!row.rarityRewards && typeof row.rarityRewards === "object"
}

export function getDefaultAffixSmeltRule(): AffixSmeltRule | null {
  const catalog = getGameDataCatalog()
  const rules = Array.isArray(catalog.affix_smelt_rules) ? catalog.affix_smelt_rules : []
  for (const row of rules) {
    if (isAffixSmeltRule(row) && row.active && row.id === "default") return row
  }
  return null
}
