export type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type FormulaTerm = {
  attrKey?: string
  attributeKey?: string
  coefficient?: number
  perSkillLevel?: number
  perEnhanceLevel?: number
}

type TargetCountNode = {
  base?: number
  formula?: FormulaTerm[]
  max?: number
}

type AmountFormulaNode = {
  baseAdd?: number
  terms?: FormulaTerm[]
}

export type FormulaEvaluationOptions = {
  missingAttributeValue?: number
  includeBaseOnly?: boolean
}

export function localizeEffectAttr(attrKey: string, t: TranslateFn): string {
  if (attrKey === "equipment_enhance_level") {
    const key = "game.itemDetailBattleHookAttrEquipmentEnhanceLevel"
    const localized = t(key)
    return localized === key ? attrKey : localized
  }
  if (attrKey === "corpse_max_hp") {
    const key = "game.skillFormulaAttrCorpseMaxHp"
    const localized = t(key)
    return localized === key ? attrKey : localized
  }
  if (attrKey === "max_hp" || attrKey === "max_mp") {
    const key = attrKey === "max_hp" ? "game.skillFormulaAttrMaxHp" : "game.skillFormulaAttrMaxMp"
    const localized = t(key)
    return localized === key ? attrKey : localized
  }
  if (attrKey === "current_hp" || attrKey === "current_mp") {
    const key = attrKey === "current_hp" ? "game.skillFormulaAttrCurrentHp" : "game.skillFormulaAttrCurrentMp"
    const localized = t(key)
    return localized === key ? (attrKey === "current_hp" ? "HP" : "MP") : localized
  }
  // The attribute catalog retains the legacy spelling; alias only the display lookup.
  const localeAttrKey = attrKey === "magical_flat_defense_penetration" ? "magicial_flat_defense_penetration" : attrKey
  const key = `attr.${localeAttrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

export function formatEffectNumber(value: number, precision = 6): string {
  if (!Number.isFinite(value)) return "0"
  const fixed = value.toFixed(precision)
  return precision > 0 ? fixed.replace(/\.?0+$/, "") : fixed
}

export function formatEffectPercent(value: number, precision = 2): string {
  return `${formatEffectNumber(value * 100, precision)}%`
}

export function localizeTargetCountText(targetCount: TargetCountNode | null | undefined, t: TranslateFn): string {
  const base = Math.max(1, Math.trunc(Number(targetCount?.base || 1)))
  const terms = normalizeFormulaTerms(targetCount?.formula, true)
  const formula = terms
    .map((term) => {
      const baseText = `${localizeEffectAttr(term.attrKey, t)}×${formatEffectNumber(term.coefficient)}`
      if (!term.perSkillLevel) return baseText
      return t("game.skillFormulaPerLevel", {
        formula: baseText,
        value: formatEffectNumber(term.perSkillLevel)
      })
    })
    .join(" + ")
  if (!formula) return localizeTargetCountValue(base, t)
  return `${localizeTargetCountValue(base, t)} + ${t("game.skillFormulaFloor", { formula })}`
}

export function resolveTargetCountMax(targetCount: TargetCountNode | null | undefined): number | null {
  const maxRaw = Math.trunc(Number(targetCount?.max ?? Number.NaN))
  if (!Number.isFinite(maxRaw) || maxRaw <= 0) return null
  return maxRaw
}

export function localizeTargetCountValue(value: number | string | null | undefined, t: TranslateFn): string {
  const numeric = Math.trunc(Number(value))
  if (Number.isFinite(numeric) && numeric >= 999) {
    const localized = t("game.skillTargetAll")
    return localized === "game.skillTargetAll" ? "全部目标" : localized
  }
  if (Number.isFinite(numeric)) return String(numeric)
  return String(value ?? "")
}

export function localizeTargetCountMaxText(targetCount: TargetCountNode | null | undefined, t: TranslateFn): string | null {
  const max = resolveTargetCountMax(targetCount)
  if (max === null) return null
  return localizeTargetCountValue(max, t)
}

export function localizeBasicAttackFormulaText(
  formula: FormulaTerm[] | null | undefined,
  previewValue: number | null,
  t: TranslateFn
): string {
  const terms = normalizeFormulaTerms(formula, false)
  const formulaText = terms
    .map((term) => `${localizeEffectAttr(term.attrKey, t)}×${formatEffectNumber(term.coefficient)}`)
    .join(" + ")
  const localized = t("game.basicAttackFormula", { formula: formulaText || "-" })
  const formulaDisplay = localized === "game.basicAttackFormula" ? (formulaText || "-") : localized
  if (previewValue === null || !Number.isFinite(previewValue)) return formulaDisplay
  return `${formulaDisplay} (${t("game.itemDetailBasicAttackFormulaInlinePreview", { value: Math.round(previewValue * 10000) / 10000 })})`
}

export function localizeAmountFormulaText(baseAddRaw: unknown, rawTerms: unknown, t: TranslateFn): string {
  const baseAdd = Number(baseAddRaw)
  const terms = normalizeFormulaTerms(rawTerms, true)
  const base = Number.isFinite(baseAdd) ? baseAdd : 0
  const parts: string[] = []
  if (base !== 0 || terms.length === 0) parts.push(formatEffectNumber(base, 4))
  for (const term of terms) {
    const coefficient = term.coefficient
    const levelDelta = term.perSkillLevel
    const enhanceDelta = term.perEnhanceLevel
    const enhanceLevelText = localizeEffectAttr("equipment_enhance_level", t)
    const coefficientText = enhanceDelta
      ? `(${formatEffectNumber(coefficient, 4)} ${enhanceDelta < 0 ? "-" : "+"} ${formatEffectNumber(Math.abs(enhanceDelta), 4)}×${enhanceLevelText})`
      : formatEffectNumber(Math.abs(coefficient), 4)
    let termText = `${coefficientText}×${localizeEffectAttr(term.attrKey, t)}`
    if (levelDelta) {
      termText = t("game.skillFormulaPerLevelSigned", {
        formula: termText,
        value: `${levelDelta < 0 ? "-" : "+"}${formatEffectNumber(Math.abs(levelDelta), 4)}`
      })
    }
    if (parts.length === 0) {
      parts.push(!enhanceDelta && coefficient < 0 ? `-${termText}` : termText)
    } else {
      parts.push(`${!enhanceDelta && coefficient < 0 ? "-" : "+"} ${termText}`)
    }
  }
  return parts.join(" ")
}

export function evaluateEffectFormulaTerms(
  raw: unknown,
  unitAttributes: Record<string, number> | null | undefined,
  skillLevel = 1,
  options: FormulaEvaluationOptions = {}
): number | null {
  const terms = normalizeFormulaTerms(raw, true)
  if (terms.length === 0) return null
  const source = unitAttributes && typeof unitAttributes === "object" ? unitAttributes : null
  const missingAttributeValue = Number(options.missingAttributeValue)
  const canSubstituteMissing = Number.isFinite(missingAttributeValue)
  if (!source && !canSubstituteMissing) return null
  const levelOffset = Math.max(0, Math.trunc(Number(skillLevel)) - 1)
  let total = 0
  for (const term of terms) {
    const rawAttrValue = source?.[term.attrKey]
    const attrValue = Number(rawAttrValue)
    const resolvedAttrValue = Number.isFinite(attrValue)
      ? attrValue
      : canSubstituteMissing
        ? missingAttributeValue
        : Number.NaN
    if (!Number.isFinite(resolvedAttrValue)) return null
    total += resolvedAttrValue * (term.coefficient + term.perSkillLevel * levelOffset)
  }
  return total
}

export function evaluateTargetCountPreview(
  targetCount: TargetCountNode | null | undefined,
  unitAttributes: Record<string, number> | null | undefined,
  skillLevel = 1
): number | null {
  if (!targetCount) return null
  const base = Math.max(1, Math.trunc(Number(targetCount.base || 1)))
  const maxValue = resolveTargetCountMax(targetCount)
  const terms = normalizeFormulaTerms(targetCount.formula, true)
  if (terms.length === 0) return null
  const evaluated = evaluateEffectFormulaTerms(terms, unitAttributes, skillLevel)
  if (evaluated === null) return null
  const total = base + Math.floor(evaluated)
  if (maxValue === null) return total
  return Math.min(total, maxValue)
}

export function evaluateAmountFormulaPreview(
  formula: AmountFormulaNode | null | undefined,
  unitAttributes: Record<string, number> | null | undefined,
  skillLevel = 1,
  options: FormulaEvaluationOptions = {}
): number | null {
  if (!formula) return null
  const baseAdd = Number(formula.baseAdd || 0)
  const terms = normalizeFormulaTerms(formula.terms, true)
  if (terms.length === 0) return options.includeBaseOnly === true ? (Number.isFinite(baseAdd) ? baseAdd : 0) : null
  const evaluated = evaluateEffectFormulaTerms(terms, unitAttributes, skillLevel, options)
  if (evaluated === null) return null
  return (Number.isFinite(baseAdd) ? baseAdd : 0) + evaluated
}

function normalizeFormulaTerms(raw: unknown, includePerSkillLevel: boolean): Array<{ attrKey: string; coefficient: number; perSkillLevel: number; perEnhanceLevel: number }> {
  const rows = Array.isArray(raw) ? raw : []
  return rows
    .map((term) => {
      const attrKey = String(term?.attrKey || (term as { attributeKey?: unknown } | null | undefined)?.attributeKey || "").trim()
      const coefficient = Number(term?.coefficient)
      const perSkillLevel = Number(term?.perSkillLevel ?? 0)
      const perEnhanceLevel = Number(term?.perEnhanceLevel ?? 0)
      if (!attrKey || !Number.isFinite(coefficient) || !Number.isFinite(perSkillLevel) || !Number.isFinite(perEnhanceLevel)) return null
      return { attrKey, coefficient, perSkillLevel: includePerSkillLevel ? perSkillLevel : 0, perEnhanceLevel }
    })
    .filter((term): term is { attrKey: string; coefficient: number; perSkillLevel: number; perEnhanceLevel: number } => !!term)
}
