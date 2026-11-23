type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type FormulaTerm = {
  attrKey: string
  coefficient: number
  perSkillLevel: number
}

export type AffixMutationDisplayLine = {
  label: string
  value: string
  skillIds?: string[]
}

export type AffixMutationDisplayOptions = {
  unitAttributes?: Record<string, number>
}

export function buildAffixMutationDisplayLines(
  rawMutations: unknown,
  t: TranslateFn,
  options: AffixMutationDisplayOptions = {}
): AffixMutationDisplayLine[] {
  const mutations = Array.isArray(rawMutations) ? rawMutations : []
  const lines: AffixMutationDisplayLine[] = []
  const seenSkillGroups = new Set<string>()
  for (const node of mutations) {
    if (!node || typeof node !== "object" || Array.isArray(node)) continue
    const mutation = node as Record<string, unknown>
    const skillIds = normalizeStringArray(mutation.skillIds)
    const scope = normalizeToken(mutation.scope || "both")
    const skillGroupKey = skillIds.length > 0 ? skillIds.join("\u0000") : `*:${scope}`
    if (skillIds.length > 0 && !seenSkillGroups.has(skillGroupKey)) {
      seenSkillGroups.add(skillGroupKey)
      lines.push({
        label: t("game.itemDetailAffixMutationDisplayAffectedSkills"),
        value: "",
        skillIds
      })
    } else if (skillIds.length === 0 && !seenSkillGroups.has(skillGroupKey)) {
      seenSkillGroups.add(skillGroupKey)
      lines.push({
        label: t("game.itemDetailAffixMutationDisplayAffectedSkills"),
        value: localizeMutationScope(scope, t)
      })
    }

    const selectorText = localizeEffectSelector(mutation.effectSelector, t)
    if (selectorText) {
      lines.push({
        label: t("game.itemDetailAffixMutationDisplayAffectedObject"),
        value: selectorText
      })
    }

    const changeParts = buildMutationChangeParts(mutation, t, options)
    if (changeParts.length > 0) {
      lines.push({
        label: t("game.itemDetailAffixMutationDisplayChange"),
        value: changeParts.join(t("game.itemDetailAffixMutationDisplayPartSeparator"))
      })
    }

    const appendEffectParts = buildAppendEffectParts(mutation, t)
    if (appendEffectParts.length > 0) {
      lines.push({
        label: t("game.itemDetailAffixMutationDisplayAppendEffect"),
        value: appendEffectParts.join(t("game.itemDetailAffixMutationDisplayPartSeparator"))
      })
    }
  }
  return lines
}

export function hasAffixMutationDisplayLines(rawMutations: unknown): boolean {
  return Array.isArray(rawMutations) && rawMutations.some((node) => !!node && typeof node === "object" && !Array.isArray(node))
}

export function canonicalizeAffixMutations(rawMutations: unknown): string {
  return JSON.stringify(sortValueForCompare(Array.isArray(rawMutations) ? rawMutations : []))
}

function buildMutationChangeParts(
  mutation: Record<string, unknown>,
  t: TranslateFn,
  options: AffixMutationDisplayOptions
): string[] {
  const ops = mutation.ops && typeof mutation.ops === "object" && !Array.isArray(mutation.ops)
    ? mutation.ops as Record<string, unknown>
    : {}
  const constraints = mutation.constraints && typeof mutation.constraints === "object" && !Array.isArray(mutation.constraints)
    ? mutation.constraints as Record<string, unknown>
    : {}
  const parts: string[] = []
  const targetCountDelta = formatMutationFormulaDelta(ops.targetCountDelta, true, t)
  if (targetCountDelta) {
    parts.push(t("game.itemDetailAffixMutationDisplayTargetCountDelta", { value: targetCountDelta }))
  }
  const min = toFiniteNumber(constraints.targetCountMin)
  if (min !== null && Math.trunc(min) > 1) {
    parts.push(t("game.itemDetailAffixMutationDisplayTargetCountMin", { value: formatNumber(min, 0) }))
  }
  const max = toFiniteNumber(constraints.targetCountMax)
  if (max !== null) {
    parts.push(t("game.itemDetailAffixMutationDisplayTargetCountMax", { value: formatNumber(max, 0) }))
  }
  const amountBaseAddDelta = formatMutationFormulaDelta(ops.amountBaseAddDelta, true, t)
  if (amountBaseAddDelta) {
    parts.push(t("game.itemDetailAffixMutationDisplayAmountBaseDelta", { value: amountBaseAddDelta }))
  }
  const amountTermsAppend = normalizeTermArray(ops.amountTermsAppend)
  if (amountTermsAppend.length > 0) {
    const formula = amountTermsAppend.map((term) => formatFormulaTerm(term, t)).join(" + ")
    const projected = options.unitAttributes
      ? amountTermsAppend.reduce((total, term) => {
          const attributeValue = Number(options.unitAttributes?.[term.attrKey] || 0)
          return total + term.coefficient * attributeValue
        }, 0)
      : null
    parts.push(projected === null
      ? t("game.itemDetailAffixMutationDisplayAmountTermsAppend", { formula })
      : t("game.itemDetailAffixMutationDisplayAmountTermsAppendProjection", {
          formula,
          value: formatSignedNumber(projected, 2)
        }))
  }
  const statusAppend = normalizeStringArray(ops.statusIdsAppend)
  if (statusAppend.length > 0) {
    parts.push(t("game.itemDetailAffixMutationDisplayStatusAppend", { statuses: statusAppend.map((id) => localizeBuffName(id, t)).join(" / ") }))
  }
  const statusRemove = normalizeStringArray(ops.statusIdsRemove)
  if (statusRemove.length > 0) {
    parts.push(t("game.itemDetailAffixMutationDisplayStatusRemove", { statuses: statusRemove.map((id) => localizeBuffName(id, t)).join(" / ") }))
  }
  const hitCountDelta = toFiniteNumber(ops.hitCountDelta)
  if (hitCountDelta !== null && hitCountDelta !== 0) {
    parts.push(t("game.specialization.dsl.mutation.hitCountDelta", { value: formatSignedNumber(hitCountDelta, 0) }))
  }
  const amountMultiplier = toFiniteNumber(ops.amountMultiplier)
  if (amountMultiplier !== null && amountMultiplier !== 1) {
    parts.push(t("game.specialization.dsl.mutation.amountMultiplier", { value: formatPercent(amountMultiplier) }))
  }
  const cooldownDelta = toFiniteNumber(ops.cooldownDelta)
  if (cooldownDelta !== null && cooldownDelta !== 0) {
    parts.push(t("game.specialization.dsl.mutation.cooldownDelta", { value: formatSignedNumber(cooldownDelta, 0) }))
  }
  const cooldownFloor = toFiniteNumber(ops.cooldownFloor)
  if (cooldownFloor !== null && cooldownFloor >= 1) {
    parts.push(t("game.specialization.dsl.mutation.cooldownFloor", { value: formatNumber(cooldownFloor, 0) }))
  }
  const mpCostDelta = toFiniteNumber(ops.mpCostDelta)
  if (mpCostDelta !== null && mpCostDelta !== 0) {
    parts.push(t("game.specialization.dsl.mutation.mpCostDelta", { value: formatSignedNumber(mpCostDelta, 0) }))
  }
  const mpCostMultiplier = toFiniteNumber(ops.mpCostMultiplier)
  if (mpCostMultiplier !== null && mpCostMultiplier !== 1) {
    parts.push(t("game.specialization.dsl.mutation.mpCostMultiplier", { value: formatPercent(mpCostMultiplier) }))
  }
  const attackStatPolicy = normalizeToken(ops.attackStatPolicy)
  if (attackStatPolicy) {
    const key = `game.specialization.dsl.mutation.attackStatPolicy.${attackStatPolicy}`
    const localized = t(key)
    parts.push(localized === key ? attackStatPolicy : localized)
  }
  const damageTypePolicy = normalizeToken(ops.damageTypePolicy)
  if (damageTypePolicy) {
    const key = `game.specialization.dsl.mutation.damageTypePolicy.${damageTypePolicy}`
    const localized = t(key)
    parts.push(localized === key ? damageTypePolicy : localized)
  }
  const targetingPatch = ops.targetingPatch && typeof ops.targetingPatch === "object" && !Array.isArray(ops.targetingPatch)
    ? ops.targetingPatch as Record<string, unknown>
    : null
  if (targetingPatch) {
    const preferredOrder = [
      "target_scope",
      "target_positions",
      "select_rule",
      "select_attr_key",
      "secondary_seek",
      "fallback_self_if_empty",
      "target_count"
    ]
    const entries = Object.entries(targetingPatch).sort(([left], [right]) => {
      const leftIndex = preferredOrder.indexOf(normalizeI18nFieldToken(left))
      const rightIndex = preferredOrder.indexOf(normalizeI18nFieldToken(right))
      return (leftIndex < 0 ? preferredOrder.length : leftIndex) - (rightIndex < 0 ? preferredOrder.length : rightIndex)
    })
    const fields = entries.map(([field, value]) => {
      const fieldToken = normalizeI18nFieldToken(field)
      const key = `game.specialization.dsl.mutation.targetingField.${fieldToken}`
      const localized = t(key)
      const label = localized === key ? field : localized
      const valueText = localizeTargetingPatchFieldValue(fieldToken, value, t)
      return valueText
        ? t("game.specialization.dsl.mutation.targetingFieldValue", { field: label, value: valueText })
        : label
    })
    if (fields.length > 0) parts.push(t("game.specialization.dsl.mutation.targetingPatch", { fields: fields.join(" / ") }))
  }
  return parts
}

function buildAppendEffectParts(mutation: Record<string, unknown>, t: TranslateFn): string[] {
  const ops = mutation.ops && typeof mutation.ops === "object" && !Array.isArray(mutation.ops)
    ? mutation.ops as Record<string, unknown>
    : {}
  const appendEffects = Array.isArray(ops.appendEffects) ? ops.appendEffects : []
  return appendEffects
    .map((effect, index) => localizeAppendEffect(effect, index, t))
    .filter((text): text is string => !!text)
}

function localizeEffectSelector(rawSelector: unknown, t: TranslateFn): string {
  const selector = rawSelector && typeof rawSelector === "object" && !Array.isArray(rawSelector)
    ? rawSelector as Record<string, unknown>
    : {}
  const mode = normalizeToken(selector.mode || "all")
  if (mode === "all") return t("game.itemDetailAffixMutationDisplaySelectorAll")
  if (mode === "kind") {
    const kinds = normalizeStringArray(selector.kinds ?? (selector.kind ? [selector.kind] : []))
    if (kinds.length === 0) return t("game.itemDetailAffixMutationDisplaySelectorKindFallback", { kind: "effect" })
    return kinds.map((kind) => {
      const normalized = normalizeToken(kind)
      const key = `game.itemDetailAffixMutationDisplaySelectorKind.${normalized || "effect"}`
      const localized = t(key)
      return localized === key ? t("game.itemDetailAffixMutationDisplaySelectorKindFallback", { kind: normalized || "effect" }) : localized
    }).join(" / ")
  }
  if (mode === "effect_id") {
    const effectIds = normalizeStringArray(selector.effectIds ?? selector.effect_ids)
    return effectIds.length > 0
      ? t("game.specialization.dsl.mutation.selectorEffectIds", { ids: effectIds.join(" / ") })
      : t("game.specialization.dsl.mutation.selectorEffectIdsEmpty")
  }
  if (mode === "index") {
    const indexes = normalizeIntegerArray(selector.indexes)
    if (indexes.length === 0) return t("game.itemDetailAffixMutationDisplaySelectorIndex")
    return t("game.itemDetailAffixMutationDisplaySelectorIndexList", { indexes: indexes.join(", ") })
  }
  return t("game.itemDetailAffixMutationDisplaySelectorModeFallback", { mode: mode || "all" })
}

function localizeAppendEffect(effect: unknown, index: number, t: TranslateFn): string | null {
  if (!effect || typeof effect !== "object" || Array.isArray(effect)) return null
  const row = effect as Record<string, unknown>
  const kind = normalizeToken(row.kind || row.type)
  if (kind === "status") {
    const payload = row.statusPayload && typeof row.statusPayload === "object" && !Array.isArray(row.statusPayload)
      ? row.statusPayload as Record<string, unknown>
      : {}
    const statusIds = normalizeStringArray(payload.statusIds)
    if (statusIds.length === 0) return null
    const targetText = localizeTargeting(row.targeting, t)
    const durationText = localizeStatusDuration(payload, t)
    return t("game.itemDetailAffixMutationDisplayAppendStatus", {
      target: targetText,
      statuses: statusIds.map((id) => localizeBuffName(id, t)).join(" / "),
      duration: durationText
    })
  }
  if (kind === "consume_hp") {
    return t("game.specialization.dsl.mutation.appendConsumeHp", {
      value: formatPercent(toFiniteNumber(row.maxHpRatio ?? row.max_hp_ratio) ?? 0)
    })
  }
  if (kind === "runtime_damage_amp") {
    return t("game.specialization.dsl.mutation.appendDamageAmp", {
      value: formatPercent(toFiniteNumber(row.value) ?? 0)
    })
  }
  if (kind === "splash_damage") {
    return t("game.specialization.dsl.mutation.appendSplashDamage", {
      count: Math.max(1, Math.trunc(toFiniteNumber(row.targetCount ?? row.target_count) ?? 1)),
      value: formatPercent(toFiniteNumber(row.amountRatio ?? row.amount_ratio) ?? 0)
    })
  }
  if (kind === "secondary_damage") {
    const policy = normalizeToken(row.targetPolicy ?? row.target_policy)
    let target = t(`game.specialization.dsl.mutation.secondaryTarget.${policy || "all_rows"}`)
    if (policy === "fixed_row") {
      const position = normalizeToken(row.targetPosition ?? row.target_position)
      const positionKey = `skillForge.position.${position}`
      const localizedPosition = t(positionKey)
      target = t("game.specialization.dsl.mutation.secondaryTarget.fixed_row", {
        position: localizedPosition === positionKey ? position : localizedPosition
      })
    }
    return t("game.specialization.dsl.mutation.appendSecondaryDamage", {
      target,
      value: formatPercent(toFiniteNumber(row.amountRatio ?? row.amount_ratio) ?? 0)
    })
  }
  if (kind === "mark_target") {
    return t("game.specialization.dsl.mutation.appendMarkTarget", {
      mark: localizeSpecializationMechanicToken(row.markId, t),
      turns: Math.max(1, Math.trunc(toFiniteNumber(row.durationTurns ?? row.duration_turns) ?? 1))
    })
  }
  return t("game.itemDetailAffixMutationDisplayAppendUnknown", {
    index: index + 1,
    kind: kind || t("game.itemDetailAffixMutationDisplayUnknownEffect")
  })
}

function localizeTargeting(rawTargeting: unknown, t: TranslateFn): string {
  const targeting = rawTargeting && typeof rawTargeting === "object" && !Array.isArray(rawTargeting)
    ? rawTargeting as Record<string, unknown>
    : {}
  const scope = normalizeToken(targeting.targetScope || "enemy")
  const positions = normalizeStringArray(targeting.targetPositions)
  const selectRule = normalizeToken(targeting.selectRule || "first")
  const targetCount = targeting.targetCount && typeof targeting.targetCount === "object" && !Array.isArray(targeting.targetCount)
    ? targeting.targetCount as Record<string, unknown>
    : {}
  const count = Math.max(1, Math.trunc(Number(targetCount.base || 1)))
  const scopeText = localizeToken(`game.itemDetailAffixMutationDisplayTargetScope.${scope}`, scope, t)
  const positionText = (positions.length > 0 ? positions : ["vanguard"])
    .map((position) => localizeToken(`game.itemDetailAffixMutationDisplayTargetPosition.${normalizeToken(position)}`, position, t))
    .join("/")
  const selectText = localizeToken(`game.itemDetailAffixMutationDisplaySelectRule.${selectRule}`, selectRule, t)
  return t("game.itemDetailAffixMutationDisplayTargeting", {
    scope: scopeText,
    position: positionText,
    select: selectText,
    count: formatNumber(count, 0)
  })
}

function localizeTargetingPatchFieldValue(field: string, rawValue: unknown, t: TranslateFn): string {
  if (field === "target_scope") {
    const value = normalizeToken(rawValue)
    return localizeToken(`game.itemDetailAffixMutationDisplayTargetScope.${value}`, value, t)
  }
  if (field === "target_positions") {
    return normalizeStringArray(rawValue)
      .map((position) => localizeToken(`skillForge.position.${normalizeToken(position)}`, position, t))
      .join(" / ")
  }
  if (field === "select_rule") {
    const value = normalizeToken(rawValue)
    return localizeToken(`skillForge.selectRule.${value}`, value, t)
  }
  if (field === "select_attr_key") {
    const value = String(rawValue || "").trim()
    return value ? localizeAttr(value, t) : ""
  }
  if (field === "secondary_seek") {
    const value = normalizeToken(rawValue)
    return localizeToken(`skillForge.secondarySeek.${value}`, value, t)
  }
  if (field === "fallback_self_if_empty") {
    if (rawValue === true) return t("game.yes")
    if (rawValue === false) return t("game.no")
  }
  if (field === "target_count") {
    const direct = toFiniteNumber(rawValue)
    if (direct !== null) return formatNumber(direct, 0)
    if (rawValue && typeof rawValue === "object" && !Array.isArray(rawValue)) {
      const base = toFiniteNumber((rawValue as Record<string, unknown>).base)
      if (base !== null) return formatNumber(base, 0)
    }
  }
  if (typeof rawValue === "string" || typeof rawValue === "number") return String(rawValue)
  return ""
}

function localizeStatusDuration(payload: Record<string, unknown>, t: TranslateFn): string {
  const base = toFiniteNumber(payload.durationBaseTurns)
  const perLevel = toFiniteNumber(payload.durationPerSkillLevelTurns) ?? 0
  if (base === null || base <= 0) return t("game.itemDetailAffixMutationDisplayDurationInstant")
  if (Math.abs(perLevel) < 1e-9) {
    return t("game.itemDetailAffixMutationDisplayDurationTurns", { turns: formatNumber(base, 2) })
  }
  return t("game.itemDetailAffixMutationDisplayDurationTurnsScaling", {
    turns: formatNumber(base, 2),
    perLevel: formatSignedNumber(perLevel, 4)
  })
}

function localizeAttr(attrKey: string, t: TranslateFn): string {
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function localizeBuffName(buffId: string, t: TranslateFn): string {
  const key = `buff.${buffId}.name`
  const localized = t(key)
  return localized === key ? buffId : localized
}

function localizeToken(key: string, fallback: string, t: TranslateFn): string {
  const localized = t(key)
  return localized === key ? fallback : localized
}

function formatFormulaTerm(term: FormulaTerm, t: TranslateFn): string {
  const coefficient = formatSignedNumber(term.coefficient, 4)
  if (!term.perSkillLevel) return `${coefficient}x${localizeAttr(term.attrKey, t)}`
  return t("game.itemDetailAffixMutationDisplayScalingFormulaTerm", {
    coefficient,
    attr: localizeAttr(term.attrKey, t),
    perLevel: formatSignedNumber(term.perSkillLevel, 4)
  })
}

function formatMutationFormulaDelta(raw: unknown, hideZero: boolean, t: TranslateFn): string {
  const direct = toFiniteNumber(raw)
  if (direct !== null) {
    if (hideZero && Math.abs(direct) < 1e-9) return ""
    return formatSignedNumber(direct, 4)
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return ""
  const node = raw as Record<string, unknown>
  const baseAdd = toFiniteNumber(node.baseAdd) ?? 0
  const terms = normalizeTermArray(node.terms)
  if (hideZero && Math.abs(baseAdd) < 1e-9 && terms.length === 0) return ""
  const parts: string[] = []
  if (baseAdd !== 0) parts.push(formatSignedNumber(baseAdd, 4))
  if (terms.length > 0) {
    parts.push(terms.map((term) => formatFormulaTerm(term, t)).join(" + "))
  }
  return parts.join(" + ")
}

function normalizeToken(value: unknown): string {
  return String(value || "").trim().toLowerCase().replaceAll("-", "_")
}

function normalizeI18nFieldToken(value: unknown): string {
  return String(value || "")
    .trim()
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/[\s-]+/g, "_")
    .toLowerCase()
}

function normalizeStringArray(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return raw.map((item) => String(item || "").trim()).filter(Boolean)
}

function normalizeIntegerArray(raw: unknown): number[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => Math.trunc(Number(item)))
    .filter((value) => Number.isFinite(value) && value > 0)
}

function normalizeTermArray(raw: unknown): FormulaTerm[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((term) => {
      if (!term || typeof term !== "object" || Array.isArray(term)) return null
      const row = term as Record<string, unknown>
      const attrKey = String(row.attrKey || row.attributeKey || "").trim()
      const coefficient = Number(row.coefficient)
      const perSkillLevel = Number(row.perSkillLevel || 0)
      if (!attrKey || !Number.isFinite(coefficient) || coefficient === 0) return null
      return { attrKey, coefficient, perSkillLevel: Number.isFinite(perSkillLevel) ? perSkillLevel : 0 }
    })
    .filter((term): term is FormulaTerm => term !== null)
}

function toFiniteNumber(raw: unknown): number | null {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function formatNumber(value: number, precision: number): string {
  const pow = precision <= 0 ? 1 : 10 ** precision
  const rounded = Math.round(value * pow) / pow
  const fixed = precision > 0 ? rounded.toFixed(precision) : String(Math.round(rounded))
  return fixed.replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1")
}

function formatSignedNumber(value: number, precision: number): string {
  const text = formatNumber(value, precision)
  return value >= 0 ? `+${text}` : text
}

function formatPercent(value: number): string {
  const percent = Math.round(value * 10000) / 100
  return `${formatNumber(percent, 2)}%`
}

function localizeMutationScope(scope: string, t: TranslateFn): string {
  const key = `game.specialization.dsl.mutation.scope.${scope || "both"}`
  const localized = t(key)
  return localized === key ? t("game.itemDetailAffixMutationDisplayAllSkills") : localized
}

function localizeSpecializationMechanicToken(raw: unknown, t: TranslateFn): string {
  const token = normalizeToken(raw)
  if (!token) return "-"
  const key = `game.specialization.dsl.token.${token}`
  const localized = t(key)
  return localized === key ? token : localized
}

function sortValueForCompare(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortValueForCompare)
  if (!value || typeof value !== "object") return value
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, child]) => [key, sortValueForCompare(child)])
  )
}
