import { getGameDataChunk } from "@services/game-data-runtime"
import type { Translate } from "@/types/item-label"
import {
  normalizeHitDamageScaling,
  type HitDamageScalingDefinition
} from "@services/hit-damage-scaling"

export type SkillDefinition = {
  id: string
  name: string
  icon: string
  nameKey: string
  descKey: string
  category: string
  hpCost: number
  mpCost: number
  cooldown: number
  maxLevel: number
  enemyOnly: boolean
  targeting: SkillTargetingDefinition
  effects: SkillEffectDefinition[]
  castGuards: SkillCastGuardDefinition[]
  statusRequirement: Record<string, number>
  weaponRequirement: string[]
  nameParams?: {
    sourceSkillId?: string
    materialKey?: string
    effectKey?: string
    traitKey?: string
  }
  customForge?: {
    sourceKind: string
    sourceSkillId: string
    budget: { used: number; limit: number; refund: number; failChance: number }
  }
}

export type SkillDamageTerm = {
  attrKey: string
  coefficient: number
  perSkillLevel: number
}

export type SkillTargetTerm = {
  attrKey: string
  coefficient: number
  perSkillLevel: number
}

export type SkillTargetPredicateDefinition = {
  kind: "compare_attr" | "exclude_enemy_type" | "require_target_has_status" | "target_hp_not_full"
  actorAttr?: string
  targetAttr?: string
  operator?: string
  enemyType?: string
  statusId?: string
}

export type SkillTargetingDefinition = {
  targetPositions: Array<"vanguard" | "midguard" | "rearguard">
  secondarySeek: "rear_to_front" | "front_to_rear" | "random_alive" | "none"
  targetScope: "ally" | "enemy" | "all" | "self"
  selectRule: "hp_max" | "hp_min" | "hp_percent_max" | "hp_percent_min" | "first" | "attr_max" | "attr_min" | "random" | "boss_first" | "self"
  selectAttrKey: string
  fallbackSelfIfEmpty: boolean
  ignoreTargetPriority: boolean
  targetCount: {
    base: number
    formula: SkillTargetTerm[]
    max?: number
  }
  targetPredicates: SkillTargetPredicateDefinition[]
}

export type SkillAmountFormulaDefinition = {
  baseAdd: number
  terms: SkillDamageTerm[]
}

export type SkillDamageSpecDefinition = {
  hitCount: { base: number; formula: SkillTargetTerm[] }
  hitDamageScaling?: HitDamageScalingDefinition
  enemyOnlyDamageReductionPierce?: number
  enemyOnlyDamageReductionPierceTargetStatuses?: string[]
}

export type SkillStatusPayloadDefinition = {
  statusIds: string[]
  durationBaseTurns: number
  durationPerSkillLevelTurns: number
  durationFormula?: SkillAmountFormulaDefinition
  applyChanceBase?: number
  applyChancePerSkillLevel?: number
  applyChanceFormula?: SkillAmountFormulaDefinition
  applyChanceMin?: number
  applyChanceMax?: number
}

export type SkillCastGuardDefinition =
  | { kind: "enemy_side_not_has_effect"; effectKind: string }
  | { kind: "ally_side_has_non_full_hp" }

export type SkillEffectDefinition = {
  kind: "damage" | "heal" | "resource" | "status" | "corpse_explosion" | "adjust_skill_cooldown"
  damageType?: "physical" | "magical"
  resource?: "hp" | "mp"
  targeting: SkillTargetingDefinition
  corpseTargeting?: SkillTargetingDefinition
  consumeCorpse?: boolean
  onlyWhenNoTargetFromEffects?: number[]
  reuseTargetsFromEffect?: number
  requiredCasterStatuses?: string[]
  requiredTargetStatuses?: string[]
  consumeCasterStatusesOnHit?: string[]
  amountFormula?: SkillAmountFormulaDefinition
  hitCount?: { base: number; formula: SkillTargetTerm[] }
  damageSpec?: SkillDamageSpecDefinition
  statusPayload?: SkillStatusPayloadDefinition
  amount?: number
  includeBasic?: boolean
  skillIds?: string[]
  excludeSkillIds?: string[]
}

export type SkillTypeDefinition = {
  id: string
  nameKey: string
  descKey: string
  color: string
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
  if (typeof raw === "string") {
    const value = raw.trim().toLowerCase()
    if (value === "true" || value === "1" || value === "yes") return true
    if (value === "false" || value === "0" || value === "no") return false
  }
  if (typeof raw === "number") {
    if (raw === 1) return true
    if (raw === 0) return false
  }
  return fallback
}

function parseSkillFormulaTerms(raw: unknown): SkillDamageTerm[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((term) => {
      if (!term || typeof term !== "object") return null
      const rawTerm = term as Record<string, unknown>
      const attrKey = String(rawTerm.attrKey || "").trim()
      const coefficient = parseNumber(rawTerm.coefficient, Number.NaN)
      const perSkillLevel = parseNumber(rawTerm.perSkillLevel, 0)
      if (!attrKey || !Number.isFinite(coefficient) || !Number.isFinite(perSkillLevel)) return null
      return { attrKey, coefficient, perSkillLevel }
    })
    .filter((term): term is SkillDamageTerm => !!term)
}

function normalizeTargetPosition(raw: unknown): "vanguard" | "midguard" | "rearguard" | null {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "vanguard" || value === "midguard" || value === "rearguard") return value
  return null
}

function normalizeSecondarySeek(raw: unknown): "rear_to_front" | "front_to_rear" | "random_alive" | "none" {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "rear_to_front" || value === "front_to_rear" || value === "random_alive" || value === "none") return value
  return "front_to_rear"
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

function normalizeTargeting(raw: unknown, fallback: { targetDirection: string; targetSeekStrategy: string; targetBaseCount: number; targetFormula: SkillTargetTerm[] }): SkillTargetingDefinition {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? (raw as Record<string, unknown>) : {}
  const targetPositionsRaw = Array.isArray(row.targetPositions) ? row.targetPositions : []
  const targetPositions = targetPositionsRaw
    .map((value) => normalizeTargetPosition(value))
    .filter((value): value is "vanguard" | "midguard" | "rearguard" => !!value)
  const fallbackPosition = normalizeTargetPosition(fallback.targetDirection) || "vanguard"
  const selectRule = normalizeSelectRule(row.selectRule)
  const selectAttrKey = String(row.selectAttrKey || "").trim()
  const targetCountRaw = row.targetCount && typeof row.targetCount === "object" && !Array.isArray(row.targetCount)
    ? row.targetCount as Record<string, unknown>
    : {}
  const targetCountMax = Math.round(parseNumber(targetCountRaw.max, Number.NaN))
  return {
    targetPositions: targetPositions.length > 0 ? targetPositions : [fallbackPosition],
    secondarySeek: normalizeSecondarySeek(row.secondarySeek ?? fallback.targetSeekStrategy),
    targetScope: normalizeTargetScope(row.targetScope),
    selectRule,
    selectAttrKey: selectRule === "attr_max" || selectRule === "attr_min" ? selectAttrKey : "",
    fallbackSelfIfEmpty: parseBoolean(row.fallbackSelfIfEmpty, false),
    ignoreTargetPriority: parseBoolean(row.ignoreTargetPriority, false),
    targetCount: {
      base: Math.max(1, Math.round(parseNumber(targetCountRaw.base, fallback.targetBaseCount))),
      formula: parseSkillFormulaTerms(targetCountRaw.formula ?? fallback.targetFormula),
      ...(Number.isFinite(targetCountMax) && targetCountMax > 0
        ? { max: targetCountMax }
        : {})
    },
    targetPredicates: normalizeTargetPredicates(row.targetPredicates)
  }
}

function normalizeTargetPredicates(raw: unknown): SkillTargetPredicateDefinition[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => {
      if (!item || typeof item !== "object" || Array.isArray(item)) return null
      const row = item as Record<string, unknown>
      const kind = String(row.kind || "").trim().toLowerCase()
      if (kind === "compare_attr") {
        const actorAttr = String(row.actorAttr || "").trim()
        const targetAttr = String(row.targetAttr || "").trim()
        if (!actorAttr || !targetAttr) return null
        return { kind, actorAttr, targetAttr, operator: String(row.operator || ">").trim() } as SkillTargetPredicateDefinition
      }
      if (kind === "exclude_enemy_type") {
        const enemyType = String(row.enemyType || "").trim().toLowerCase()
        if (!enemyType) return null
        return { kind, enemyType } as SkillTargetPredicateDefinition
      }
      if (kind === "require_target_has_status") {
        const statusId = String(row.statusId || row.buffId || "").trim()
        if (!statusId) return null
        return { kind, statusId } as SkillTargetPredicateDefinition
      }
      return null
    })
    .filter((item): item is SkillTargetPredicateDefinition => !!item)
}

function normalizeAmountFormula(raw: unknown, fallbackTerms: SkillDamageTerm[]): SkillAmountFormulaDefinition {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    baseAdd: parseNumber(row.baseAdd, 0),
    terms: parseSkillFormulaTerms(row.terms ?? fallbackTerms)
  }
}

function normalizeStatusPayload(raw: unknown): SkillStatusPayloadDefinition {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  const statusIds = Array.isArray(row.statusIds)
    ? row.statusIds.map((value) => String(value || "").trim()).filter(Boolean)
    : typeof row.statusIds === "string"
      ? row.statusIds.split(",").map((part) => part.trim()).filter(Boolean)
      : []
  return {
    statusIds: Array.from(new Set(statusIds)),
    durationBaseTurns: Math.max(0, Math.round(parseNumber(row.durationBaseTurns, 0))),
    durationPerSkillLevelTurns: parseNumber(row.durationPerSkillLevelTurns, 0),
    ...(row.applyChanceBase !== undefined || row.applyChance !== undefined || row.applyChanceFormula !== undefined ? {
      applyChanceBase: parseNumber(row.applyChanceBase ?? row.applyChance, 0),
      applyChancePerSkillLevel: parseNumber(row.applyChancePerSkillLevel, 0),
      ...(row.applyChanceFormula && typeof row.applyChanceFormula === "object" && !Array.isArray(row.applyChanceFormula)
        ? { applyChanceFormula: normalizeAmountFormula(row.applyChanceFormula, []) }
        : {}),
      applyChanceMin: Math.max(0, Math.min(1, parseNumber(row.applyChanceMin, 0))),
      applyChanceMax: Math.max(0, Math.min(1, parseNumber(row.applyChanceMax, 1)))
    } : {}),
    ...(row.durationFormula && typeof row.durationFormula === "object" && !Array.isArray(row.durationFormula)
      ? { durationFormula: normalizeAmountFormula(row.durationFormula, []) }
      : {})
  }
}

function parseEffectReferenceIndexes(raw: unknown): number[] {
  const refs: number[] = []
  const append = (value: unknown) => {
    const parsed = Math.round(parseNumber(value, Number.NaN))
    if (!Number.isFinite(parsed) || parsed <= 0) return
    if (refs.includes(parsed)) return
    refs.push(parsed)
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

function normalizeEffects(raw: unknown, fallback: { damageType: "physical" | "magical"; targeting: SkillTargetingDefinition }): SkillEffectDefinition[] {
  if (!Array.isArray(raw)) return []
  const out: SkillEffectDefinition[] = []
  for (const node of raw) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const row = node as Record<string, unknown>
    const kind = String(row.kind || "").trim().toLowerCase()
    if (kind !== "damage" && kind !== "heal" && kind !== "resource" && kind !== "status" && kind !== "corpse_explosion" && kind !== "adjust_skill_cooldown") continue
    const effect: SkillEffectDefinition = {
      kind,
      targeting: normalizeTargeting(row.targeting, {
        targetDirection: fallback.targeting.targetPositions[0] || "vanguard",
        targetSeekStrategy: fallback.targeting.secondarySeek,
        targetBaseCount: fallback.targeting.targetCount.base,
        targetFormula: fallback.targeting.targetCount.formula
      })
    }
    if (kind === "corpse_explosion") {
      effect.corpseTargeting = normalizeTargeting(row.corpseTargeting, {
        targetDirection: effect.targeting.targetPositions[0] || "vanguard",
        targetSeekStrategy: effect.targeting.secondarySeek,
        targetBaseCount: 1,
        targetFormula: []
      })
      effect.consumeCorpse = parseBoolean(row.consumeCorpse, true)
    }

    const onlyWhenNoTargetFromEffects = parseEffectReferenceIndexes(row.onlyWhenNoTargetFromEffects)
    if (onlyWhenNoTargetFromEffects.length > 0) {
      effect.onlyWhenNoTargetFromEffects = onlyWhenNoTargetFromEffects
    }
    const reuseTargetsFromEffect = parseEffectReferenceIndex(row.reuseTargetsFromEffect)
    if (typeof reuseTargetsFromEffect === "number") {
      effect.reuseTargetsFromEffect = reuseTargetsFromEffect
    }
    const requiredCasterStatuses = Array.isArray(row.requiredCasterStatuses)
      ? row.requiredCasterStatuses.map((value) => String(value || "").trim()).filter(Boolean)
      : []
    if (requiredCasterStatuses.length > 0) {
      effect.requiredCasterStatuses = Array.from(new Set(requiredCasterStatuses))
    }
    const requiredTargetStatuses = Array.isArray(row.requiredTargetStatuses)
      ? row.requiredTargetStatuses.map((value) => String(value || "").trim()).filter(Boolean)
      : []
    if (requiredTargetStatuses.length > 0) {
      effect.requiredTargetStatuses = Array.from(new Set(requiredTargetStatuses))
    }
    const consumeCasterStatusesOnHit = Array.isArray(row.consumeCasterStatusesOnHit)
      ? row.consumeCasterStatusesOnHit.map((value) => String(value || "").trim()).filter(Boolean)
      : []
    if (consumeCasterStatusesOnHit.length > 0) {
      effect.consumeCasterStatusesOnHit = Array.from(new Set(consumeCasterStatusesOnHit))
    }
    if (kind === "damage" || kind === "heal" || kind === "resource" || kind === "corpse_explosion") {
      effect.damageType = String(row.damageType || "").trim().toLowerCase() === "magical" ? "magical" : "physical"
      effect.amountFormula = normalizeAmountFormula(row.amountFormula, [])
      if (kind === "resource") {
        effect.resource = String(row.resource || "mp").trim().toLowerCase() === "hp" ? "hp" : "mp"
        delete effect.damageType
      }
      const damageSpec = row.damageSpec && typeof row.damageSpec === "object" && !Array.isArray(row.damageSpec)
        ? row.damageSpec as Record<string, unknown>
        : null
      const hitCountRaw = damageSpec?.hitCount && typeof damageSpec.hitCount === "object" && !Array.isArray(damageSpec.hitCount)
        ? damageSpec.hitCount as Record<string, unknown>
        : null
      if (kind === "damage" && damageSpec) {
        const hitCount = {
          base: Math.max(1, Math.round(parseNumber(hitCountRaw?.base, 1))),
          formula: parseSkillFormulaTerms(hitCountRaw?.formula)
        }
        const hitDamageScaling = normalizeHitDamageScaling(damageSpec.hitDamageScaling)
        const pierce = Math.max(0, Math.min(1, parseNumber(damageSpec.enemyOnlyDamageReductionPierce, 0)))
        const targetStatuses = Array.isArray(damageSpec.enemyOnlyDamageReductionPierceTargetStatuses)
          ? Array.from(new Set(damageSpec.enemyOnlyDamageReductionPierceTargetStatuses
            .map((value) => String(value || "").trim())
            .filter(Boolean)))
          : []
        effect.damageSpec = {
          hitCount,
          ...(hitDamageScaling ? { hitDamageScaling } : {}),
          ...(pierce > 0 ? { enemyOnlyDamageReductionPierce: pierce } : {}),
          ...(targetStatuses.length > 0 ? { enemyOnlyDamageReductionPierceTargetStatuses: targetStatuses } : {})
        }
      }
      if (hitCountRaw) {
        const base = Math.max(1, Math.round(parseNumber(hitCountRaw.base, 1)))
        if (base > 1) {
          effect.hitCount = { base, formula: parseSkillFormulaTerms(hitCountRaw.formula) }
        }
      }
    } else if (kind === "status") {
      effect.statusPayload = normalizeStatusPayload(row.statusPayload)
    } else {
      effect.amount = Math.trunc(parseNumber(row.amount ?? row.turns ?? row.value, 0))
      effect.includeBasic = parseBoolean(row.includeBasic ?? row.include_basic, false)
      effect.skillIds = Array.isArray(row.skillIds) ? row.skillIds.map((value) => String(value || "").trim()).filter(Boolean) : []
      effect.excludeSkillIds = Array.isArray(row.excludeSkillIds) ? row.excludeSkillIds.map((value) => String(value || "").trim()).filter(Boolean) : []
    }
    out.push(effect)
  }
  return out
}

function normalizeCastGuards(raw: unknown): SkillCastGuardDefinition[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => {
      if (!item || typeof item !== "object" || Array.isArray(item)) return null
      const row = item as Record<string, unknown>
      const kind = String(row.kind || "").trim().toLowerCase()
      if (kind === "ally_side_has_non_full_hp") return { kind }
      if (kind !== "enemy_side_not_has_effect") return null
      const effectKind = String(row.effectKind || row.effect || "").trim()
      if (!effectKind) return null
      return { kind, effectKind }
    })
    .filter((item): item is SkillCastGuardDefinition => !!item)
}

function normalizeSkillDefinition(raw: unknown): SkillDefinition | null {
  if (!raw || typeof raw !== "object") return null
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  if (!id) return null
  const nameKey = typeof row.nameKey === "string" && row.nameKey.trim() ? row.nameKey.trim() : `skill.${id}.name`
  const descKey = typeof row.descKey === "string" && row.descKey.trim() ? row.descKey.trim() : `skill.${id}.desc`
  const targeting = normalizeTargeting(row.targeting, {
    targetDirection: "vanguard",
    targetSeekStrategy: "front_to_rear",
    targetBaseCount: 1,
    targetFormula: []
  })
  const effects = normalizeEffects(row.effects, {
    damageType: "physical",
    targeting
  })
  const primaryTargeting = effects[0]?.targeting || targeting
  const rawNameParams = row.nameParams && typeof row.nameParams === "object" && !Array.isArray(row.nameParams)
    ? row.nameParams as Record<string, unknown>
    : null
  const rawCustomForge = row.customForge && typeof row.customForge === "object" && !Array.isArray(row.customForge)
    ? row.customForge as Record<string, unknown>
    : null
  const rawBudget = rawCustomForge?.budget && typeof rawCustomForge.budget === "object" && !Array.isArray(rawCustomForge.budget)
    ? rawCustomForge.budget as Record<string, unknown>
    : {}
  return {
    id,
    name: typeof row.name === "string" ? row.name.trim() : "",
    icon: typeof row.icon === "string" ? row.icon.trim() : "",
    nameKey,
    descKey,
    category: typeof row.category === "string" ? row.category.trim() : "",
    hpCost: Math.max(0, parseNumber(row.hpCost, 0)),
    mpCost: Math.max(0, parseNumber(row.mpCost, 0)),
    cooldown: Math.max(0, Math.round(parseNumber(row.cooldown, 0))),
    maxLevel: Math.max(1, Math.round(parseNumber(row.maxLevel, 200))),
    enemyOnly: parseBoolean(row.enemyOnly, false),
    targeting: primaryTargeting,
    effects,
    castGuards: normalizeCastGuards(row.castGuards),
    statusRequirement:
      row.statusRequirement && typeof row.statusRequirement === "object" && !Array.isArray(row.statusRequirement)
        ? Object.fromEntries(
            Object.entries(row.statusRequirement as Record<string, unknown>)
              .map(([key, value]) => [String(key).trim(), parseNumber(value, Number.NaN)])
              .filter(([key, value]) => key && Number.isFinite(value))
          )
        : {},
    weaponRequirement: Array.isArray(row.weaponRequirement)
      ? row.weaponRequirement.map((value) => String(value || "").trim()).filter(Boolean)
      : [],
    ...(rawNameParams
      ? {
          nameParams: {
            sourceSkillId: String(rawNameParams.sourceSkillId || "").trim(),
            materialKey: String(rawNameParams.materialKey || "").trim(),
            effectKey: String(rawNameParams.effectKey || "").trim(),
            traitKey: String(rawNameParams.traitKey || "").trim()
          }
        }
      : {}),
    ...(rawCustomForge
      ? {
          customForge: {
            sourceKind: String(rawCustomForge.sourceKind || "").trim(),
            sourceSkillId: String(rawCustomForge.sourceSkillId || "").trim(),
            budget: {
              used: parseNumber(rawBudget.used, 0),
              limit: parseNumber(rawBudget.limit, 0),
              refund: parseNumber(rawBudget.refund, 0),
              failChance: parseNumber(rawBudget.failChance, 0)
            }
          }
        }
      : {})
  }
}

function normalizeSkillTypeDefinition(raw: unknown): SkillTypeDefinition | null {
  if (!raw || typeof raw !== "object") return null
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  if (!id) return null
  return {
    id,
    nameKey: typeof row.nameKey === "string" && row.nameKey.trim() ? row.nameKey.trim() : `skill_type.${id}.name`,
    descKey: typeof row.descKey === "string" && row.descKey.trim() ? row.descKey.trim() : `skill_type.${id}.desc`,
    color: typeof row.color === "string" && row.color.trim() ? row.color.trim() : "#909399"
  }
}

function getRawSkills(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { skills?: unknown }).skills
  return Array.isArray(rows) ? rows : []
}

function getRawSkillTypes(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { skill_type?: unknown }).skill_type
  return Array.isArray(rows) ? rows : []
}

let skillDefinitions: SkillDefinition[] | null = null
let skillDefinitionMap: Map<string, SkillDefinition> | null = null
const runtimeSkillDefinitionMap = new Map<string, SkillDefinition>()
let skillTypeDefinitions: SkillTypeDefinition[] | null = null
let skillTypeDefinitionMap: Map<string, SkillTypeDefinition> | null = null

function ensureSkillDefinitions() {
  if (skillDefinitions && skillDefinitionMap) return
  skillDefinitions = getRawSkills(getGameDataChunk("skills"))
    .map((row) => normalizeSkillDefinition(row))
    .filter((row): row is SkillDefinition => !!row)
  skillDefinitionMap = new Map<string, SkillDefinition>(skillDefinitions.map((row) => [row.id, row]))
}

function ensureSkillTypeDefinitions() {
  if (skillTypeDefinitions && skillTypeDefinitionMap) return
  skillTypeDefinitions = getRawSkillTypes(getGameDataChunk("skill_type"))
    .map((row) => normalizeSkillTypeDefinition(row))
    .filter((row): row is SkillTypeDefinition => !!row)
    .sort((a, b) => a.id.localeCompare(b.id))
  skillTypeDefinitionMap = new Map<string, SkillTypeDefinition>(skillTypeDefinitions.map((row) => [row.id, row]))
}

export function listSkillDefinitions(): SkillDefinition[] {
  ensureSkillDefinitions()
  const merged = new Map(skillDefinitions!.map((row) => [row.id, row]))
  for (const [id, row] of runtimeSkillDefinitionMap) merged.set(id, row)
  return Array.from(merged.values())
}

export function getSkillDefinition(skillId: string): SkillDefinition | undefined {
  ensureSkillDefinitions()
  const normalized = String(skillId || "").trim()
  return runtimeSkillDefinitionMap.get(normalized) || skillDefinitionMap!.get(normalized)
}

export function installRuntimeSkillDefinitions(rows: unknown[]): void {
  runtimeSkillDefinitionMap.clear()
  for (const raw of rows) {
    const definition = normalizeSkillDefinition(raw)
    if (definition) runtimeSkillDefinitionMap.set(definition.id, definition)
  }
}

export function mergeRuntimeSkillDefinitions(rows: unknown[]): void {
  for (const raw of rows) {
    const definition = normalizeSkillDefinition(raw)
    if (definition) runtimeSkillDefinitionMap.set(definition.id, definition)
  }
}

export function listSkillTypeDefinitions(): SkillTypeDefinition[] {
  ensureSkillTypeDefinitions()
  return skillTypeDefinitions!
}

export function getSkillTypeDefinition(skillTypeId: string): SkillTypeDefinition | undefined {
  const normalized = String(skillTypeId || "").trim()
  if (!normalized) return undefined
  ensureSkillTypeDefinitions()
  return skillTypeDefinitionMap!.get(normalized)
}

export function getLocalizedSkillName(t: Translate, skillId: string, fallback = ""): string {
  const normalized = String(skillId || "").trim()
  if (!normalized) return fallback
  const definition = getSkillDefinition(normalized)
  return localizeSkillDefinitionName(t, normalized, definition, fallback)
}

export function getLocalizedRuntimeSkillName(
  t: Translate,
  skillId: string,
  runtimeSkill: Record<string, unknown> | null | undefined,
  fallback = ""
): string {
  const normalized = String(skillId || runtimeSkill?.id || "").trim()
  if (!runtimeSkill || typeof runtimeSkill !== "object" || Array.isArray(runtimeSkill)) {
    return getLocalizedSkillName(t, normalized, fallback)
  }
  const rawNameParams = runtimeSkill.nameParams && typeof runtimeSkill.nameParams === "object" && !Array.isArray(runtimeSkill.nameParams)
    ? runtimeSkill.nameParams as Record<string, unknown>
    : null
  const definition: Pick<SkillDefinition, "name" | "nameKey" | "nameParams"> = {
    name: typeof runtimeSkill.name === "string" ? runtimeSkill.name.trim() : "",
    nameKey: typeof runtimeSkill.nameKey === "string" ? runtimeSkill.nameKey.trim() : "",
    ...(rawNameParams
      ? {
          nameParams: {
            sourceSkillId: String(rawNameParams.sourceSkillId || "").trim(),
            materialKey: String(rawNameParams.materialKey || "").trim(),
            effectKey: String(rawNameParams.effectKey || "").trim(),
            traitKey: String(rawNameParams.traitKey || "").trim()
          }
        }
      : {})
  }
  return localizeSkillDefinitionName(t, normalized, definition, fallback)
}

function localizeSkillDefinitionName(
  t: Translate,
  normalized: string,
  definition: Pick<SkillDefinition, "name" | "nameKey" | "nameParams"> | undefined,
  fallback: string
): string {
  const nameParams = definition?.nameParams
  if (nameParams?.materialKey && nameParams.effectKey) {
    const material = translatedOrFallback(t, nameParams.materialKey, nameParams.materialKey)
    const effect = translatedOrFallback(t, nameParams.effectKey, "")
    const trait = nameParams.traitKey && nameParams.traitKey !== nameParams.effectKey
      ? translatedOrFallback(t, nameParams.traitKey, "")
      : ""
    const sourceID = String(nameParams.sourceSkillId || "").trim()
    const source = sourceID && sourceID !== normalized ? getLocalizedSkillName(t, sourceID, sourceID) : ""
    return [source, `${material}${trait || effect}`].filter(Boolean).join(" · ")
  }
  const keys = [
    definition?.nameKey || "",
    `skill.${normalized}.name`
  ].filter(Boolean)
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  if (definition?.name) return definition.name
  return fallback || normalized
}

function translatedOrFallback(t: Translate, key: string, fallback: string): string {
  const translated = t(key)
  return translated !== key && translated.trim() !== "" ? translated : fallback
}
