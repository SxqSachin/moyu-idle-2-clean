import { getGameDataChunk } from "@services/game-data-runtime"
import type { Translate } from "@/types/item-label"
import { formatAttributeValue, formatAttributeValueByKey, getAttributeUnit } from "./attribute-display"
import { localizeAmountFormulaText, localizeEffectAttr } from "./effect-formula-display"

export type BuffDefinition = {
  id: string
  nameKey: string
  descKey: string
  icon: string
  durationTurns: number
  permanent: boolean
  kind: "buff" | "debuff"
  dispellable: boolean
  type: string
  showInBattleCard: boolean
  stackPolicy: string
  maxCharges: number
}

export type BuffTypeDefinition = {
  id: string
  nameKey: string
  descKey: string
  color: string
}

export type BuffEffectDescription = {
  key: string
  label: string
  buffIds?: string[]
  suffix?: string
}

type BuffEffectRule = {
  id: string
  buffId: string
  trigger: string
  eventFilter: Record<string, unknown>
  conditions: Array<Record<string, unknown>>
  maxTriggersPerTurn: number
  maxTriggersPerWave: number
  maxTriggersPerBattle: number
  effects: Array<Record<string, unknown>>
}

function parseNumber(raw: unknown, fallback = 0): number {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function normalizeBuffDefinition(raw: unknown): BuffDefinition | null {
  if (!raw || typeof raw !== "object") return null
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  if (!id) return null
  const kindRaw = String(row.kind || "buff").trim().toLowerCase()
  const kind: "buff" | "debuff" = kindRaw === "debuff" ? "debuff" : "buff"
  return {
    id,
    nameKey: typeof row.nameKey === "string" && row.nameKey.trim() ? row.nameKey.trim() : `buff.${id}.name`,
    descKey: typeof row.descKey === "string" && row.descKey.trim() ? row.descKey.trim() : `buff.${id}.desc`,
    icon: typeof row.icon === "string" ? row.icon.trim() : "",
    durationTurns: Math.max(0, Math.trunc(parseNumber(row.durationTurns, 0))),
    permanent: Boolean(row.permanent),
    kind,
    dispellable: Boolean(row.dispellable),
    type: typeof row.type === "string" ? row.type.trim() : "",
    showInBattleCard: Boolean(row.showInBattleCard),
    stackPolicy: typeof row.stackPolicy === "string" ? row.stackPolicy.trim() : "refresh_duration",
    maxCharges: Math.max(1, Math.trunc(parseNumber(row.maxCharges, 1)))
  }
}

function normalizeBuffTypeDefinition(raw: unknown): BuffTypeDefinition | null {
  if (!raw || typeof raw !== "object") return null
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  if (!id) return null
  return {
    id,
    nameKey: typeof row.nameKey === "string" && row.nameKey.trim() ? row.nameKey.trim() : `buff_type.${id}.name`,
    descKey: typeof row.descKey === "string" && row.descKey.trim() ? row.descKey.trim() : `buff_type.${id}.desc`,
    color: typeof row.color === "string" && row.color.trim() ? row.color.trim() : "#909399"
  }
}

function getRawBuffs(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { buff?: unknown }).buff
  return Array.isArray(rows) ? rows : []
}

function getRawBuffTypes(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { buff_type?: unknown }).buff_type
  return Array.isArray(rows) ? rows : []
}

function getRawBuffEffects(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { buff_effects?: unknown }).buff_effects
  return Array.isArray(rows) ? rows : []
}

let buffDefinitions: BuffDefinition[] | null = null
let buffDefinitionMap: Map<string, BuffDefinition> | null = null
let buffTypeDefinitions: BuffTypeDefinition[] | null = null
let buffTypeDefinitionMap: Map<string, BuffTypeDefinition> | null = null
let attributeNameKeyMap: Map<string, string> | null = null
let buffEffectRulesByBuffID: Map<string, BuffEffectRule[]> | null = null
let buffEffectRuleMap: Map<string, BuffEffectRule> | null = null

function ensureBuffDefinitions() {
  if (buffDefinitions && buffDefinitionMap) return
  buffDefinitions = getRawBuffs(getGameDataChunk("buff"))
    .map((row) => normalizeBuffDefinition(row))
    .filter((row): row is BuffDefinition => !!row)
  buffDefinitionMap = new Map<string, BuffDefinition>(buffDefinitions.map((row) => [row.id, row]))
}

function ensureBuffTypeDefinitions() {
  if (buffTypeDefinitions && buffTypeDefinitionMap) return
  buffTypeDefinitions = getRawBuffTypes(getGameDataChunk("buff_type"))
    .map((row) => normalizeBuffTypeDefinition(row))
    .filter((row): row is BuffTypeDefinition => !!row)
  buffTypeDefinitionMap = new Map<string, BuffTypeDefinition>(buffTypeDefinitions.map((row) => [row.id, row]))
}

function ensureAttributeNameKeyMap() {
  if (attributeNameKeyMap) return
  const attributesChunk = getGameDataChunk("attributes")
  attributeNameKeyMap = new Map<string, string>(
    ((attributesChunk as { attributes?: unknown }).attributes as Array<Record<string, unknown>> || [])
      .map((row) => [String(row.id || "").trim(), String(row.nameKey || "").trim()])
  )
}

function ensureBuffEffectRules() {
  if (buffEffectRulesByBuffID) return
  const buffEffectRules: BuffEffectRule[] = getRawBuffEffects(getGameDataChunk("buff_effects"))
    .map((row) => {
      if (!row || typeof row !== "object") return null
      const item = row as Record<string, unknown>
      const id = String(item.id || "").trim()
      const buffId = String(item.buffId || "").trim()
      const trigger = String(item.trigger || "").trim().toLowerCase()
      const effects = Array.isArray(item.effects)
        ? item.effects.map((effect) => (effect && typeof effect === "object" ? effect as Record<string, unknown> : {}))
        : []
      const eventFilter = item.eventFilter && typeof item.eventFilter === "object" && !Array.isArray(item.eventFilter)
        ? item.eventFilter as Record<string, unknown>
        : {}
      const conditions = Array.isArray(item.conditions)
        ? item.conditions.map((condition) => (condition && typeof condition === "object" && !Array.isArray(condition) ? condition as Record<string, unknown> : {}))
        : []
      if (!id || !buffId || !trigger || effects.length <= 0) return null
      return {
        id,
        buffId,
        trigger,
        eventFilter,
        conditions,
        maxTriggersPerTurn: Math.max(0, Math.trunc(parseNumber(item.maxTriggersPerTurn ?? item.max_triggers_per_turn, 0))),
        maxTriggersPerWave: Math.max(0, Math.trunc(parseNumber(item.maxTriggersPerWave ?? item.max_triggers_per_wave, 0))),
        maxTriggersPerBattle: Math.max(0, Math.trunc(parseNumber(item.maxTriggersPerBattle ?? item.max_triggers_per_battle, 0))),
        effects
      }
    })
    .filter((row): row is BuffEffectRule => Boolean(row))
  buffEffectRulesByBuffID = new Map<string, BuffEffectRule[]>()
  buffEffectRuleMap = new Map<string, BuffEffectRule>()
  for (const rule of buffEffectRules) {
    buffEffectRuleMap.set(rule.id, rule)
    const rows = buffEffectRulesByBuffID.get(rule.buffId)
    if (rows) rows.push(rule)
    else buffEffectRulesByBuffID.set(rule.buffId, [rule])
  }
}

export function getBuffDefinition(buffID: string): BuffDefinition | undefined {
  const normalized = String(buffID || "").trim()
  if (!normalized) return undefined
  ensureBuffDefinitions()
  return buffDefinitionMap!.get(normalized)
}

export function getBuffTypeDefinition(typeID: string): BuffTypeDefinition | undefined {
  const normalized = String(typeID || "").trim()
  if (!normalized) return undefined
  ensureBuffTypeDefinitions()
  return buffTypeDefinitionMap!.get(normalized)
}

export function listBuffDefinitions(): BuffDefinition[] {
  ensureBuffDefinitions()
  return buffDefinitions!
}

export function listBuffTypeDefinitions(): BuffTypeDefinition[] {
  ensureBuffTypeDefinitions()
  return buffTypeDefinitions!
}

export function getBuffEffectRule(ruleID: string): Pick<BuffEffectRule, "id" | "buffId" | "trigger"> | undefined {
  const normalized = String(ruleID || "").trim()
  if (!normalized) return undefined
  ensureBuffEffectRules()
  const rule = buffEffectRuleMap!.get(normalized)
  return rule ? { id: rule.id, buffId: rule.buffId, trigger: rule.trigger } : undefined
}

export function listBuffDefinitionsByEffectKind(effectKind: string): BuffDefinition[] {
  const normalized = normalizeEffectKind(effectKind)
  if (!normalized) return []
  ensureBuffDefinitions()
  ensureBuffEffectRules()
  const matched = new Set<string>()
  for (const [buffID, rules] of buffEffectRulesByBuffID!.entries()) {
    if (rules.some((rule) => rule.effects.some((effect) => normalizeEffectKind(effect.kind || effect.type) === normalized))) {
      matched.add(buffID)
    }
  }
  return buffDefinitions!.filter((definition) => matched.has(definition.id))
}

export function getLocalizedBuffName(t: Translate, buffID: string, fallback = ""): string {
  const normalized = String(buffID || "").trim()
  if (!normalized) return fallback
  const definition = getBuffDefinition(normalized)
  const keys = [
    definition?.nameKey || "",
    `buff.${normalized}.name`
  ].filter(Boolean)
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return fallback || normalized
}

function normalizeEffectKind(raw: unknown): string {
  return String(raw || "").trim().toLowerCase().replace(/[^a-z]/g, "")
}

function getLocalizedAttrName(t: Translate, attrKey: string): string {
  const normalized = String(attrKey || "").trim()
  if (!normalized) return "-"
  ensureAttributeNameKeyMap()
  const nameKey = attributeNameKeyMap!.get(normalized) || `attr.${normalized}.name`
  const localized = t(nameKey)
  if (localized !== nameKey && localized.trim() !== "") return localized
  return normalized
}

function formatSignedValue(value: number, asPercent = false): string {
  const normalized = asPercent ? value * 100 : value
  const sign = normalized >= 0 ? "+" : "-"
  const abs = Math.abs(normalized)
  const raw = Number.isInteger(abs) ? String(abs) : abs.toFixed(2).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
  return asPercent ? `${sign}${raw}%` : `${sign}${raw}`
}

function resolvePayloadValue(effect: Record<string, unknown>, payload: Record<string, unknown> | null | undefined): { configured: boolean; available: boolean; value: number } {
  const payloadKey = String(effect.payloadValueKey || "").trim()
  if (!payloadKey) return { configured: false, available: false, value: 0 }
  const coeff = parseNumber(effect.payloadValueCoeff, 1)
  const payloadValue = payload && typeof payload === "object" ? Number(payload[payloadKey]) : Number.NaN
  if (!Number.isFinite(payloadValue)) return { configured: true, available: false, value: 0 }
  return { configured: true, available: true, value: payloadValue * coeff }
}

function resolvePayloadFormula(effect: Record<string, unknown>, payloadFormula: Record<string, unknown> | null | undefined): { configured: boolean; formula?: Record<string, unknown>; coeff: number } {
  const payloadKey = String(effect.payloadValueKey || "").trim()
  if (!payloadKey) return { configured: false, coeff: 1 }
  const formula = payloadFormula && typeof payloadFormula === "object" && !Array.isArray(payloadFormula)
    ? payloadFormula[payloadKey]
    : undefined
  if (!formula || typeof formula !== "object" || Array.isArray(formula)) return { configured: true, coeff: parseNumber(effect.payloadValueCoeff, 1) }
  return { configured: true, formula: formula as Record<string, unknown>, coeff: parseNumber(effect.payloadValueCoeff, 1) }
}

function formatPayloadFormulaValue(
  rawFormula: Record<string, unknown>,
  coeff: number,
  t: Translate,
  options: { attrKey?: string; percent?: boolean; negative?: boolean } = {}
): string {
  const signMultiplier = options.negative ? -1 : 1
  const parts: string[] = []
  const baseAdd = parseNumber(rawFormula.baseAdd, 0) * coeff * signMultiplier
  if (baseAdd !== 0) {
    if (options.attrKey) parts.push(formatAttributeValueByKey(options.attrKey, baseAdd, true))
    else parts.push(formatSignedValue(baseAdd, Boolean(options.percent)))
  }
  const terms = Array.isArray(rawFormula.terms) ? rawFormula.terms : []
  for (const term of terms) {
    if (!term || typeof term !== "object" || Array.isArray(term)) continue
    const row = term as Record<string, unknown>
    const attrKey = String(row.attrKey || "").trim()
    const coefficient = parseNumber(row.coefficient, Number.NaN) * coeff * signMultiplier
    if (!attrKey || !Number.isFinite(coefficient) || coefficient === 0) continue
    const sign = coefficient >= 0 ? "+" : "-"
    const absCoefficient = Math.abs(coefficient)
    const coefficientText = options.attrKey
      ? formatAttributeValueByKey(options.attrKey, absCoefficient, false)
      : options.percent
        ? `${formatUnsignedPercent(absCoefficient)}`
        : formatUnsignedNumber(absCoefficient)
    parts.push(`${sign}${localizeEffectAttr(attrKey, t)}×${coefficientText}`)
  }
  if (parts.length === 0) return ""
  return parts.join(" ")
}

function formatUnsignedNumber(value: number): string {
  if (!Number.isFinite(value)) return "0"
  return value.toFixed(6).replace(/\.?0+$/, "")
}

function formatUnsignedPercent(value: number): string {
  if (!Number.isFinite(value)) return "0%"
  return `${formatUnsignedNumber(value * 100)}%`
}

function localizeBuffEffectAmount(effect: Record<string, unknown>, t: Translate): string {
  const rawValue = Number(effect.value ?? effect.amount ?? 0)
  const fixedValue = Number.isFinite(rawValue) ? rawValue : 0
  const formulaRaw = effect.amountFormula ?? effect.damageFormula
  if (formulaRaw && typeof formulaRaw === "object" && !Array.isArray(formulaRaw)) {
    const formula = formulaRaw as { baseAdd?: unknown; terms?: unknown }
    const baseAdd = Number(formula.baseAdd ?? 0)
    const mergedBaseAdd = (Number.isFinite(baseAdd) ? baseAdd : 0) + fixedValue
    return localizeAmountFormulaText(mergedBaseAdd, formula.terms, t)
  }
  return formatSignedValue(fixedValue, false)
}

export function getLocalizedBuffKindLabel(t: Translate, kind: "buff" | "debuff"): string {
  return t(kind === "debuff" ? "game.buffKindDebuff" : "game.buffKindBuff")
}

function localizeTriggerLabel(t: Translate, rule: BuffEffectRule, timingOverride = ""): string {
  const normalized = String(rule.trigger || "").trim().toLowerCase()
  const probabilityKinds = Array.isArray(rule.eventFilter.probabilityKinds)
    ? rule.eventFilter.probabilityKinds.map((kind) => String(kind || "").trim().toLowerCase()).filter(Boolean)
    : []
  let triggerLabel = String(timingOverride || "").trim()
  if (!triggerLabel && (normalized === "on_probability_success" || normalized === "on_probability_failure" || normalized === "on_before_probability_check")) {
    const scope = probabilityKinds.includes("equipment_proc") && probabilityKinds.includes("status_apply")
      ? "any"
      : probabilityKinds.includes("equipment_proc")
        ? "equipment"
        : probabilityKinds.includes("status_apply")
          ? "status"
          : "any"
    const phase = normalized === "on_probability_success" ? "success" : normalized === "on_probability_failure" ? "failure" : "before"
    triggerLabel = t(`game.buffProbabilityTrigger.${phase}.${scope}`)
  } else if (!triggerLabel) {
    const key = `game.buffTrigger_${normalized}`
    const localized = t(key)
    triggerLabel = localized !== key ? localized : t("game.buffTrigger_unknown")
  }
  const limits: string[] = []
  if (rule.maxTriggersPerTurn > 0) limits.push(translateWithParams(t, "game.buffTriggerLimitPerTurn", { count: rule.maxTriggersPerTurn }))
  if (rule.maxTriggersPerWave > 0) limits.push(translateWithParams(t, "game.buffTriggerLimitPerWave", { count: rule.maxTriggersPerWave }))
  if (rule.maxTriggersPerBattle > 0) limits.push(translateWithParams(t, "game.buffTriggerLimitPerBattle", { count: rule.maxTriggersPerBattle }))
  return limits.length > 0
    ? translateWithParams(t, "game.buffTriggerWithLimits", { trigger: triggerLabel, limits: limits.join(t("game.buffTriggerLimitSeparator")) })
    : triggerLabel
}

function conditionType(condition: Record<string, unknown>): string {
  return String(condition.type || condition.kind || "").trim().toLowerCase()
}

function conditionValue(rule: BuffEffectRule, type: string): number | null {
  const condition = rule.conditions.find((entry) => conditionType(entry) === type)
  if (!condition) return null
  const value = parseNumber(condition.value ?? condition.count, Number.NaN)
  return Number.isFinite(value) ? value : null
}

function hasRuleEffect(rule: BuffEffectRule, kind: string): boolean {
  const normalized = String(kind || "").trim().toLowerCase()
  return rule.effects.some((effect) => String(effect.kind || effect.type || "").trim().toLowerCase() === normalized)
}

function localizeBuffRuleTiming(t: Translate, rule: BuffEffectRule): string {
  const types = new Set(rule.conditions.map(conditionType).filter(Boolean))
  const hasAvailableUse = types.has("self_charges_remaining_gte") && (conditionValue(rule, "self_charges_remaining_gte") ?? 0) <= 1
  if (types.has("incoming_damage_would_be_lethal")) {
    return t(hasAvailableUse ? "game.buffConditionNextLethalDamage" : "game.buffConditionLethalDamage")
  }

  const requiredStacks = conditionValue(rule, "self_buff_count_gte")
  if (requiredStacks !== null && requiredStacks > 0) {
    return translateWithParams(t, "game.buffConditionAtStacks", { count: Math.trunc(requiredStacks) })
  }

  const chargesAtLeast = conditionValue(rule, "self_charges_remaining_gte")
  const chargesAtMost = conditionValue(rule, "self_charges_remaining_lte")
  const isCurtain = rule.buffId.includes("curtain")
  const exactCurtainAct = chargesAtLeast !== null && chargesAtMost !== null && Math.trunc(chargesAtLeast) === Math.trunc(chargesAtMost)
    ? Math.trunc(chargesAtLeast)
    : null
  if (isCurtain && rule.trigger === "on_before_take_damage" && types.has("incoming_damage_gte") && exactCurtainAct !== null) {
    return translateWithParams(t, "game.buffConditionCurtainAtActsBeforeDamage", { count: exactCurtainAct })
  }
  if (isCurtain && rule.trigger === "on_turn_end") {
    const exactAct = exactCurtainAct !== null
      ? exactCurtainAct
      : chargesAtMost !== null && chargesAtMost <= 1
        ? 1
        : null
    if (exactAct === 3) return t("game.buffConditionCurtainFirstActEnd")
    if (exactAct === 2) return t("game.buffConditionCurtainSecondActEnd")
    if (exactAct === 1) return t("game.buffConditionCurtainFinalActEnd")
  }

  if (hasAvailableUse) {
    if (rule.trigger === "on_before_probability_check") return t("game.buffConditionNextProbabilityCheck")
    if (rule.trigger === "on_probability_success") return t("game.buffConditionNextProbabilitySuccess")
    if (rule.trigger === "on_probability_failure") return t("game.buffConditionNextProbabilityFailure")
    if (rule.trigger === "on_before_take_damage" && types.has("incoming_damage_gte")) {
      return hasRuleEffect(rule, "consume_charge")
        ? t("game.buffConditionNextIncomingDamage")
        : t(isCurtain ? "game.buffConditionCurtainWhileRaisedBeforeDamage" : "game.buffConditionWhileUseRemainsBeforeDamage")
    }
    if (rule.trigger === "on_after_take_damage") {
      return hasRuleEffect(rule, "consume_charge")
        ? t("game.buffConditionFirstIncomingDamageAfter")
        : t(isCurtain ? "game.buffConditionCurtainWhileRaisedAfterDamage" : "game.buffConditionWhileUseRemainsAfterDamage")
    }
  }

  if (chargesAtLeast !== null && chargesAtMost !== null && Math.trunc(chargesAtLeast) === Math.trunc(chargesAtMost)) {
    return translateWithParams(t, "game.buffConditionUsesRemaining", { count: Math.trunc(chargesAtLeast) })
  }
  return ""
}

function localizeBuffTargetMode(t: Translate, raw: unknown): string {
  const mode = String(raw || "self").trim().toLowerCase()
  const key = `game.buffEffectTargetMode_${mode || "self"}`
  const localized = t(key)
  if (localized !== key && localized.trim() !== "") return localized
  return mode || t("game.buffEffectTargetMode_self")
}

function translateWithParams(t: Translate, key: string, params: Record<string, unknown>): string {
  return (t as (key: string, params?: Record<string, unknown>) => string)(key, params)
}

function localizeDurationSuffix(t: Translate, rawTurns: unknown): string {
  const turns = Math.trunc(parseNumber(rawTurns, 0))
  if (turns <= 0) return ""
  return translateWithParams(t, "game.buffEffectDurationTurns", { turns })
}

function parseBoolean(raw: unknown, fallback = false): boolean {
  if (typeof raw === "boolean") return raw
  if (typeof raw === "number") return raw !== 0
  if (typeof raw === "string") {
    const normalized = raw.trim().toLowerCase()
    if (["true", "1", "yes", "y"].includes(normalized)) return true
    if (["false", "0", "no", "n"].includes(normalized)) return false
  }
  return fallback
}

function buffLine(key: string, label: string, buffIds?: string[], suffix = ""): BuffEffectDescription {
  const normalizedBuffIds = (Array.isArray(buffIds) ? buffIds : [])
    .map((item) => String(item || "").trim())
    .filter(Boolean)
  return {
    key,
    label,
    ...(normalizedBuffIds.length > 0 ? { buffIds: normalizedBuffIds } : {}),
    ...(String(suffix || "").trim() ? { suffix: String(suffix || "").trim() } : {})
  }
}

function missingResourceRatio(effect: Record<string, unknown>, resource: "hp" | "mp"): number | null {
  const formulaRaw = effect.amountFormula ?? effect.damageFormula
  if (!formulaRaw || typeof formulaRaw !== "object" || Array.isArray(formulaRaw)) return null
  const formula = formulaRaw as Record<string, unknown>
  if (parseNumber(formula.baseAdd, 0) !== 0) return null
  const terms = Array.isArray(formula.terms) ? formula.terms : []
  const maxKey = resource === "hp" ? "max_hp" : "max_mp"
  const currentKey = resource === "hp" ? "current_hp" : "current_mp"
  let maxCoefficient = 0
  let currentCoefficient = 0
  for (const term of terms) {
    if (!term || typeof term !== "object" || Array.isArray(term)) return null
    const row = term as Record<string, unknown>
    const attrKey = String(row.attrKey || "").trim()
    const coefficient = parseNumber(row.coefficient, Number.NaN)
    if (!Number.isFinite(coefficient)) return null
    if (attrKey === maxKey) maxCoefficient += coefficient
    else if (attrKey === currentKey) currentCoefficient += coefficient
    else return null
  }
  if (maxCoefficient <= 0 || Math.abs(maxCoefficient + currentCoefficient) > 0.000001) return null
  return maxCoefficient
}

function localizeBuffProbabilityScope(t: Translate, raw: unknown): string {
  const kinds = Array.isArray(raw)
    ? raw.map((kind) => String(kind || "").trim().toLowerCase()).filter(Boolean)
    : []
  const scope = kinds.includes("equipment_proc") && kinds.includes("status_apply")
    ? "both"
    : kinds.includes("equipment_proc")
      ? "equipment"
      : kinds.includes("status_apply")
        ? "status"
        : "all"
  return t(`game.buffProbabilityScope.${scope}`)
}

function payloadDrivenEffectValue(
  effect: Record<string, unknown>,
  payload: Record<string, unknown> | null | undefined,
  payloadFormula: Record<string, unknown> | null | undefined,
  t: Translate,
  options: { attrKey?: string; percent?: boolean } = {}
): string | null {
  const baseValue = Number(effect.value ?? effect.percent ?? 0)
  const base = Number.isFinite(baseValue) ? baseValue : 0
  const payloadValue = resolvePayloadValue(effect, payload)
  const formulaValue = resolvePayloadFormula(effect, payloadFormula)
  if (payloadValue.configured && !payloadValue.available) {
    const formula = formulaValue.formula
      ? formatPayloadFormulaValue(formulaValue.formula, formulaValue.coeff, t, options)
      : ""
    const baseText = base !== 0
      ? options.attrKey
        ? formatAttributeValueByKey(options.attrKey, base, true)
        : formatSignedValue(base, Boolean(options.percent))
      : ""
    const combined = [baseText, formula].filter(Boolean).join(" ")
    return combined || t("game.buffEffectPayloadValue")
  }
  const resolved = base + payloadValue.value
  if (resolved === 0 && !payloadValue.configured) return null
  return options.attrKey
    ? formatAttributeValueByKey(options.attrKey, resolved, true)
    : formatSignedValue(resolved, Boolean(options.percent))
}

export function getBuffEffectDescriptions(t: Translate, buffID: string, payload?: Record<string, unknown> | null, payloadFormula?: Record<string, unknown> | null): BuffEffectDescription[] {
  const normalized = String(buffID || "").trim()
  if (!normalized) return []
  ensureBuffEffectRules()
  const rules = buffEffectRulesByBuffID!.get(normalized) || []
  const lines: BuffEffectDescription[] = []
  for (const rule of rules) {
    const triggerPrefix = localizeTriggerLabel(t, rule, localizeBuffRuleTiming(t, rule))
    const groupedEffectIndexes = new Set<number>()
    const missingHpIndex = rule.effects.findIndex((effect) => String(effect.kind || effect.type || "").trim().toLowerCase() === "heal" && missingResourceRatio(effect, "hp") !== null)
    const missingMpIndex = rule.effects.findIndex((effect) => {
      const kind = String(effect.kind || effect.type || "").trim().toLowerCase()
      return kind === "resource" && String(effect.resource || "mp").trim().toLowerCase() === "mp" && missingResourceRatio(effect, "mp") !== null
    })
    if (missingHpIndex >= 0 && missingMpIndex >= 0) {
      const hpRatio = missingResourceRatio(rule.effects[missingHpIndex]!, "hp")
      const mpRatio = missingResourceRatio(rule.effects[missingMpIndex]!, "mp")
      if (hpRatio !== null && mpRatio !== null && Math.abs(hpRatio - mpRatio) < 0.000001) {
        groupedEffectIndexes.add(missingHpIndex)
        groupedEffectIndexes.add(missingMpIndex)
        lines.push(buffLine(
          `${rule.id}-missing-hp-mp`,
          `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectRestoreMissingHpAndMp", { percent: formatUnsignedPercent(hpRatio) })}`
        ))
      }
    }
    rule.effects.forEach((effect, effectIndex) => {
      if (groupedEffectIndexes.has(effectIndex)) return
      const kind = String(effect.kind || effect.type || "").trim().toLowerCase()
      const lineKey = `${rule.id}-${effectIndex}`
      if (!kind) return
      if (kind === "add_stat" || kind === "addstat") {
        const attrKey = String(effect.attr || effect.stat || "").trim().toLowerCase() 
        const value = payloadDrivenEffectValue(effect, payload, payloadFormula, t, { attrKey })
        if (!value) return
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${getLocalizedAttrName(t, attrKey)} ${value}`))
        return
      }
      if (kind === "add_stat_percent" || kind === "addstatpercent") {
        const attrKey = String(effect.attr || effect.stat || "").trim().toLowerCase()
        const value = payloadDrivenEffectValue(effect, payload, payloadFormula, t, { percent: true })
        if (!value) return
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${getLocalizedAttrName(t, attrKey)} ${value}`))
        return
      }
      if (kind === "heal") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectHeal")} ${localizeBuffEffectAmount(effect, t)}`))
        return
      }
      if (kind === "damage") {
        const damageType = String(effect.damageType || effect.damage_type || "physical").trim().toLowerCase()
        const damageTypeKey = `game.damageType.${damageType}`
        const localizedDamageType = t(damageTypeKey)
        const damageTypeLabel = localizedDamageType === damageTypeKey ? damageType : localizedDamageType
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectDamage")}(${damageTypeLabel}) ${localizeBuffEffectAmount(effect, t)}`))
        return
      }
      if (kind === "resource") {
        const resource = String(effect.resource || "mp").trim().toLowerCase()
        const rawValue = Number(effect.value || 0)
        const isRecovery = rawValue >= 0
        const resourceLabel = resource === "hp"
          ? isRecovery ? t("game.skillRestoreHp") : t("game.skillCostHp")
          : isRecovery ? t("game.skillRestoreMp") : t("game.skillCostMp")
        const amountText = localizeBuffEffectAmount(effect, t)
        if (amountText === "+0" || amountText === "0") return
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${resourceLabel} ${amountText}`))
        return
      }
      if (kind === "extra_enemy_refresh" || kind === "extraenemyrefresh") {
        const count = Math.max(1, Math.trunc(parseNumber(effect.count, 1)))
        const enemyPool = String(effect.enemyPool || "").trim().toLowerCase()
        if (enemyPool !== "current_world") return
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectExtraEnemyRefresh", { count })}`))
        return
      }
      if (kind === "add_probability") {
        const value = Number(effect.value ?? effect.amount ?? 0)
        if (!Number.isFinite(value) || value === 0) return
        const scope = localizeBuffProbabilityScope(t, effect.probabilityKinds ?? rule.eventFilter.probabilityKinds)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectAddProbability", {
          scope,
          value: formatSignedValue(value, true)
        })}`))
        return
      }
      if (kind === "set_probability") {
        const value = Number(effect.value ?? effect.amount ?? 0)
        if (!Number.isFinite(value)) return
        const scope = localizeBuffProbabilityScope(t, effect.probabilityKinds ?? rule.eventFilter.probabilityKinds)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectSetProbability", {
          scope,
          value: formatUnsignedPercent(Math.max(0, Math.min(1, value)))
        })}`))
        return
      }
      if (kind === "add_skill_cooldown" || kind === "addskillcooldown" || kind === "add_skill_cooldown_turns" || kind === "addskillcooldownturns") {
        const turns = Math.max(0, Math.trunc(Number(effect.turns ?? effect.value ?? effect.amount ?? 0)))
        const includeBasic = parseBoolean(effect.includeBasic ?? effect.includeBasicAttack, false)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectAddSkillCooldown", { turns, target: includeBasic ? t("game.buffEffectSkillCooldownAllSkills") : t("game.buffEffectSkillCooldownNonBasic") })}`))
        return
      }
      if (kind === "reduce_damage_percent") {
        const baseValue = Number(effect.value || effect.percent || 0)
        const payloadValue = resolvePayloadValue(effect, payload)
        const formulaValue = resolvePayloadFormula(effect, payloadFormula)
        const valueText = payloadValue.configured && !payloadValue.available
          ? [
              formatSignedValue(-Math.abs(Number.isFinite(baseValue) ? baseValue : 0), true),
              formulaValue.formula ? formatPayloadFormulaValue(formulaValue.formula, formulaValue.coeff, t, { percent: true, negative: true }) : ""
            ].filter(Boolean).join(" ")
          : formatSignedValue(-Math.abs((Number.isFinite(baseValue) ? baseValue : 0) + payloadValue.value), true)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectReduceDamage")} ${valueText}`))
        return
      }
      if (kind === "set_damage_value") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectSetDamageValue")} ${Number(effect.value || 0)}`))
        return
      }
      if (kind === "set_heal_value") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectSetHealValue")} ${Number(effect.value || 0)}`))
        return
      }
      if (kind === "set_damage_multiplier") {
        const value = Number(effect.value ?? 1)
        if (!Number.isFinite(value)) return
        const key = Math.abs(value) < 0.000001
          ? "game.buffEffectNegateDamage"
          : "game.buffEffectSetDamageMultiplier"
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, key, { value: formatUnsignedPercent(Math.max(0, value)) })}`))
        return
      }
      if (kind === "consume_charge") {
        // A charge is an implementation detail. The natural timing above already
        // tells the player that this is the next/first use of the effect.
        return
      }
      if (kind === "add_damage_percent") {
        const baseValue = Number(effect.value || effect.percent || 0)
        const payloadValue = resolvePayloadValue(effect, payload)
        const formulaValue = resolvePayloadFormula(effect, payloadFormula)
        const valueText = payloadValue.configured && !payloadValue.available
          ? [
              formatSignedValue(Number.isFinite(baseValue) ? baseValue : 0, true),
              formulaValue.formula ? formatPayloadFormulaValue(formulaValue.formula, formulaValue.coeff, t, { percent: true }) : ""
            ].filter(Boolean).join(" ")
          : formatSignedValue((Number.isFinite(baseValue) ? baseValue : 0) + payloadValue.value, true)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectAddDamage")} ${valueText}`))
        return
      }
      if (kind === "add_buff") {
        const nextBuffID = String(effect.buffId || "").trim()
        const targetMode = localizeBuffTargetMode(t, effect.targetMode ?? effect.target)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectApplyBuffTo", { target: targetMode })}`, [nextBuffID], localizeDurationSuffix(t, effect.durationTurns)))
        return
      }
      if (kind === "remove_buff") {
        const nextBuffID = String(effect.buffId || "").trim()
        const targetMode = localizeBuffTargetMode(t, effect.targetMode ?? effect.target)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectRemoveBuffFrom", { target: targetMode })}`, [nextBuffID]))
        return
      }
      if (kind === "prefer_as_enemy_target") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectPreferAsEnemyTarget")}`))
        return
      }
      if (kind === "avoid_as_enemy_target") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectAvoidAsEnemyTarget")}`))
        return
      }
      if (kind === "prefer_as_ally_target") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectPreferAsAllyTarget")}`))
        return
      }
      if (kind === "avoid_as_ally_target") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectAvoidAsAllyTarget")}`))
        return
      }
      if (kind === "untargetable_by_ally") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectUntargetableByAlly")}`))
        return
      }
      if (kind === "untargetable_by_enemy") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectUntargetableByEnemy")}`))
        return
      }
      if (kind === "turn_action_suppressed") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectTurnActionSuppressed")}`))
        return
      }
      if (kind === "skill_action_suppressed") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectSkillActionSuppressed")}`))
        return
      }
      if (kind === "basic_attack_suppressed") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectBasicAttackSuppressed")}`))
        return
      }
      if (kind === "frontline_fixation") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectFrontlineFixation")}`))
        return
      }
      if (kind === "position_override") {
        const position = String(effect.position || effect.targetPosition || "").trim()
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectPositionOverride")} ${position ? t(`game.position_${position}`) : ""}`.trim()))
        return
      }
      if (kind === "position_map") {
        const mapping = effect.mapping && typeof effect.mapping === "object" && !Array.isArray(effect.mapping)
          ? effect.mapping as Record<string, unknown>
          : {}
        const entries = ["vanguard", "midguard", "rearguard"].flatMap((from) => {
          const to = String(mapping[from] || "").trim().toLowerCase()
          if (!to) return []
          return [translateWithParams(t, "game.buffEffectPositionMapEntry", {
            from: t(`game.position_${from}`),
            to: t(`game.position_${to}`)
          })]
        })
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectPositionMap")} ${entries.join(" / ")}`.trim()))
        return
      }
      if (kind === "advance_position") {
        const steps = Math.max(1, Math.trunc(parseNumber(effect.steps, 1)))
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${translateWithParams(t, "game.buffEffectAdvancePosition", { steps })}`))
        return
      }
      if (kind === "side_override") {
        const side = String(effect.side || "enemy").trim().toLowerCase()
        const sideKey = `game.buffEffectSideOverride.${side}`
        const sideText = t(sideKey)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${sideText === sideKey ? t("game.buffEffectSideOverride.other") : sideText}`))
        return
      }
      if (kind === "invert_relation_perspective") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectInvertRelationPerspective")}`))
        return
      }
      if (kind === "skip_turn_order") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectSkipTurnOrder")}`))
        return
      }
      if (kind === "turn_ineligible") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectTurnIneligible")}`))
        return
      }
      if (kind === "suppress_mirror_cast_trigger") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectSuppressMirrorCastTrigger")}`))
        return
      }
      if (kind === "trigger_synced_summon_turn") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectTriggerSyncedSummonTurn")}`))
        return
      }
      if (kind === "override_targeting") {
        const targeting = effect.targeting && typeof effect.targeting === "object" ? effect.targeting as Record<string, any> : effect
        const rawPositions = Array.isArray(targeting.targetPositions)
          ? targeting.targetPositions
          : targeting.targetPositions !== undefined
            ? [targeting.targetPositions]
            : []
        const positionLabels = rawPositions
          .map((item) => String(item || "").trim())
          .filter((item) => item.length > 0)
          .map((item) => t(`game.position_${item}`))
        const applyToBasic = effect.applyToBasicAttack !== undefined ? Boolean(effect.applyToBasicAttack) : true
        const applyToSkills = effect.applyToSkills !== undefined ? Boolean(effect.applyToSkills) : true
        const targetHint = positionLabels.length > 0 ? positionLabels.join(" / ") : "-"
        const scopeHint = applyToBasic && applyToSkills
          ? t("game.buffEffectOverrideTargetingScopeAll")
          : applyToBasic
            ? t("game.buffEffectOverrideTargetingScopeBasic")
            : applyToSkills
              ? t("game.buffEffectOverrideTargetingScopeSkills")
              : t("game.buffEffectOverrideTargetingScopeNone")
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectOverrideTargeting")} ${targetHint} (${scopeHint})`))
        return
      }
      if (kind === "damage_redirect_to_source") {
        const percent = parseNumber(effect.percent ?? effect.value, 1)
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectDamageRedirectToSource")} ${Math.round(percent * 100)}%`))
        return
      }
      if (kind === "treat_all_as_enemy") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectTreatAllAsEnemy")}`))
        return
      }
      if (kind === "camouflage_targeting_remap") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectCamouflageTargetingRemap")}`))
        return
      }
      if (kind === "marked_target") {
        lines.push(buffLine(lineKey, `${triggerPrefix}: ${t("game.buffEffectMarkedTarget")}`))
        return
      }
    })
  }
  return lines
}
