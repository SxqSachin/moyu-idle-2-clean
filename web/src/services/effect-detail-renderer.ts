import {
  evaluateAmountFormulaPreview,
  evaluateTargetCountPreview,
  formatEffectNumber,
  localizeTargetCountMaxText,
  localizeTargetCountText,
  localizeAmountFormulaText,
  localizeEffectAttr
} from "@services/effect-formula-display"
import {
  MAX_RUNTIME_DAMAGE_HITS,
  approximateHitDamageEquivalentCap,
  hitDamageEquivalentSum,
  hitDamageMultiplier,
  normalizeHitDamageScaling,
  type HitDamageScalingDefinition
} from "@services/hit-damage-scaling"

export type EffectDetailRow = {
  label: string
  value: string
  preview?: string
  buffIds?: string[]
  labelHint?: string
}

export type EffectDetailSection = {
  key: string
  title: string
  rows: EffectDetailRow[]
}

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type EffectTargeting = {
  targetPositions?: string[]
  secondarySeek?: string
  targetScope?: string
  selectRule?: string
  selectAttrKey?: string
  fallbackSelfIfEmpty?: boolean
  ignoreTargetPriority?: boolean
  targetCount?: {
    base?: number
    formula?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }>
    max?: number
  }
}

type EffectNode = {
  kind?: string
  resource?: string
  damageType?: string
  targeting?: EffectTargeting
  corpseTargeting?: EffectTargeting
  consumeCorpse?: boolean
  amountFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
  hitCount?: { base?: number; formula?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
  damageSpec?: {
    damageType?: string
    hitCount?: { base?: number; formula?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
    hitDamageScaling?: HitDamageScalingDefinition
    enemyOnlyDamageReductionPierce?: number
    enemyOnlyDamageReductionPierceTargetStatuses?: string[]
  }
  statusPayload?: {
    statusIds?: string[]
    durationBaseTurns?: number
    durationPerSkillLevelTurns?: number
    durationFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
    applyChanceBase?: number
    applyChancePerSkillLevel?: number
    applyChanceFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
    applyChanceMin?: number
    applyChanceMax?: number
  }
  summonPayload?: {
    unitId: string
    side?: string
    countFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
    durationFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
    durationTurns?: number
    maxCountPerUnit?: number
    maxCountFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
    persistAcrossBattles?: boolean
    onCasterDeath?: string
    onCapOverflow?: string
    onDispel?: string
    onDurationEnd?: string
    statsFormula?: Record<string, { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }>
  }
  choices?: Array<{ id?: string; weight?: number; statusId?: string; durationTurns?: number }>
  applyChanceBase?: number
  applyChancePerSkillLevel?: number
  applyChanceFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
  applyChanceMin?: number
  applyChanceMax?: number
  amount?: number
  includeBasic?: boolean
  skillIds?: string[]
  excludeSkillIds?: string[]
  buffIds?: string[]
  maxRemoved?: number
  reuseTargetsFromEffect?: number
  onlyWhenNoTargetFromEffects?: number[]
  requiredCasterStatuses?: string[]
  requiredTargetStatuses?: string[]
  consumeCasterStatusesOnHit?: string[]
}

export function buildEffectDetailSections(input: {
  effects: EffectNode[]
  t: TranslateFn
  skillLevel?: number
  unitAttributes?: Record<string, number> | null
  variant?: "skill" | "battle_item"
}): EffectDetailSection[] {
  const { effects, t } = input
  const skillLevel = Math.max(1, Math.trunc(Number(input.skillLevel || 1)))
  const unitAttributes = input.unitAttributes || null
  const isBattleItemVariant = input.variant === "battle_item"
  return effects.map((effect, index) => {
    const targeting = effect.targeting || {}
    const kind = String(effect.kind || "").trim().toLowerCase()
    const reusesPriorTargets = Number.isFinite(Number(effect.reuseTargetsFromEffect)) && Number(effect.reuseTargetsFromEffect) > 0
    const hideTargetSummaryRows = isBattleItemVariant && (isSelfPreferredWithoutSecondarySeek(targeting) || reusesPriorTargets)
    const rows: Array<EffectDetailRow | null> = [
      row(t("game.skillEffectKindLabel"), localizeEffectKind(effect, t)),
      hideTargetSummaryRows ? null : row(t("game.skillTargetInfo"), describeTargeting(targeting, t)),
      hideTargetSummaryRows ? null : ignoreTargetPriorityRow(targeting, t),
      hideTargetSummaryRows ? null : row(
        t("game.skillTargetBaseCount"),
        localizeTargetCountText(targeting.targetCount, t),
        previewValue(evaluateTargetCountPreview(targeting.targetCount, unitAttributes, skillLevel))
      ),
      hideTargetSummaryRows ? null : targetCountMaxRow(targeting.targetCount, t),
      hideTargetSummaryRows ? null : row(t("game.skillTargetMode"), localizeTargetScope(targeting.targetScope || "enemy", t)),
      hideTargetSummaryRows ? null : row(t("game.skillSelectRuleLabel"), localizeSelectRule(targeting.selectRule || "random", targeting.selectAttrKey || "", t)),
      hideTargetSummaryRows ? null : (targeting.fallbackSelfIfEmpty ? row(t("game.skillFallbackSelfIfEmpty"), t("game.yes")) : null),
      reuseTargetsRow(effect.reuseTargetsFromEffect, t),
      onlyWhenNoTargetRow(effect.onlyWhenNoTargetFromEffects, t),
      requiredStatusRow(t("game.skillEffectRequiredCasterStatuses"), effect.requiredCasterStatuses),
      requiredStatusRow(t("game.skillEffectRequiredTargetStatuses"), effect.requiredTargetStatuses),
      requiredStatusRow(t("game.skillEffectConsumeCasterStatusesOnHit"), effect.consumeCasterStatusesOnHit)
    ]

    if (kind === "damage" || kind === "heal" || kind === "resource") {
      const shouldShowDamageType = kind === "damage"
      const hitCount = effect.damageSpec?.hitCount || effect.hitCount
      const hitCountBase = Math.max(1, Math.trunc(Number(hitCount?.base || 1)))
      const displayFormula = kind === "resource" && resourceEffectDirection(effect) === "loss"
        ? negateAmountFormula(effect.amountFormula)
        : effect.amountFormula
      rows.push(
        kind === "resource" ? row(t("game.skillEffectResource"), localizeResource(effect.resource || "mp", t)) : null,
        shouldShowDamageType ? row(t("game.skillEffectDamageType"), localizeDamageType(effect.damageType || "physical", t)) : null,
        row(
          t("game.skillEffectAmountFormula"),
          describeAmountFormula(displayFormula, t),
          previewValue(evaluateAmountFormulaPreview(displayFormula, unitAttributes, skillLevel))
        ),
        hitCountBase > 1 || (hitCount?.formula?.length ?? 0) > 0
          ? row(t("game.skillEffectHitCount"), localizeTargetCountText({ base: hitCountBase, formula: hitCount?.formula ?? [] }, t))
          : null,
        kind === "damage" ? hitDamageScalingRow(effect.damageSpec, hitCount, unitAttributes, skillLevel, t) : null,
        kind === "damage" ? enemyOnlyDamageReductionPierceRow(effect.damageSpec, t) : null
      )
    } else if (kind === "corpse_explosion") {
      const corpseTargeting = effect.corpseTargeting || {}
      rows.push(
        row(t("game.skillCorpseTargetInfo"), describeTargeting(corpseTargeting, t)),
        row(t("game.skillCorpseTargetMode"), localizeTargetScope(corpseTargeting.targetScope || "all", t)),
        row(t("game.skillEffectDamageType"), localizeDamageType(effect.damageType || "magical", t)),
        row(
          t("game.skillEffectAmountFormula"),
          describeAmountFormula(effect.amountFormula, t),
          previewValue(evaluateAmountFormulaPreview(effect.amountFormula, unitAttributes, skillLevel))
        ),
        row(t("game.skillEffectConsumeCorpse"), effect.consumeCorpse === false ? t("game.no") : t("game.yes"))
      )
    } else if (kind === "status") {
      rows.push(statusPayloadRow(effect.statusPayload?.statusIds || [], t))
      rows.push(row(t("game.skillEffectStatusDuration"), describeStatusDuration(effect.statusPayload, t)))
      const applyChance = describeApplyChance(effect.statusPayload, t)
      if (applyChance) rows.push(row(t("game.skillEffectStatusApplyChance"), applyChance))
    } else if (kind === "summon") {
      const payload: NonNullable<EffectNode["summonPayload"]> = effect.summonPayload || { unitId: "" }
      rows.push(
        row(
          t("game.skillEffectSummonUnit"),
          localizeEnemyName(payload.unitId || "", t)
        ),
        row(t("game.skillEffectSummonSide"), localizeSummonSide(payload.side || "source", t)),
        row(t("game.skillEffectSummonCount"), describeAmountFormula(payload.countFormula, t)),
        row(t("game.skillEffectSummonDuration"), describeSummonDuration(payload, t)),
        describeSummonLimit(payload, t),
        describeSummonPolicies(payload, t)
      )
      const stats = Object.entries(payload.statsFormula || {}).map(([attr, formula]) => describeSummonStat(attr, formula, t))
      if (stats.length > 0) rows.push(row(t("game.skillEffectSummonStats"), stats.join(" / ")))
    } else if (kind === "weighted_random_per_target") {
      const applyChance = describeApplyChance(
        effect.statusPayload?.applyChanceBase !== undefined ? effect.statusPayload : effect,
        t
      )
      if (applyChance) rows.push(row(t("game.skillEffectStatusApplyChance"), applyChance))
      const validChoices = (Array.isArray(effect.choices) ? effect.choices : []).flatMap((choice) => {
        const statusId = String(choice?.statusId || "").trim()
        const weight = Number(choice?.weight)
        if (!statusId || !Number.isFinite(weight) || weight <= 0) return []
        const duration = Math.max(0, Math.trunc(Number(choice?.durationTurns || 0)))
        const durationText = duration > 0 ? t("game.skillDurationTurns", { turns: duration }) : t("game.skillFormulaUnknown")
        return [{ statusId, weight, durationText }]
      })
      const totalWeight = validChoices.reduce((sum, choice) => sum + choice.weight, 0)
      const equalWeights = validChoices.length > 0 && validChoices.every((choice) => Math.abs(choice.weight - validChoices[0]!.weight) < 0.000001)
      const choices = validChoices.map((choice) => {
        const status = localizeBuffName(choice.statusId, t)
        return equalWeights
          ? t("game.skillEffectWeightedChoiceEqual", { status, duration: choice.durationText })
          : t("game.skillEffectWeightedChoiceChance", {
            status,
            chance: formatPercent(totalWeight > 0 ? choice.weight / totalWeight : 0),
            duration: choice.durationText
          })
      })
      rows.push(row(
        t(equalWeights ? "game.skillEffectWeightedChoicesEqual" : "game.skillEffectWeightedChoices"),
        choices.join(" / ") || t("game.skillFormulaUnknown")
      ))
    } else if (kind === "adjust_skill_cooldown") {
      const skillIds = (Array.isArray(effect.skillIds) ? effect.skillIds : []).map((id) => String(id || "").trim()).filter(Boolean)
      const exclusions = (Array.isArray(effect.excludeSkillIds) ? effect.excludeSkillIds : []).map((id) => String(id || "").trim()).filter(Boolean)
      rows.push(
        row(t("game.skillEffectCooldownChange"), String(Math.trunc(Number(effect.amount || 0)))),
        row(t("game.skillEffectCooldownTargets"), skillIds.length > 0 ? skillIds.map((id) => localizeSkillName(id, t)).join(" / ") : t("game.skillEffectCooldownAllEligible")),
        exclusions.length > 0 ? row(t("game.skillEffectCooldownExclusions"), exclusions.map((id) => localizeSkillName(id, t)).join(" / ")) : null,
        effect.includeBasic ? row(t("game.skillEffectCooldownIncludeBasic"), t("game.yes")) : null
      )
    } else if (kind === "remove_buff") {
      const buffIds = (Array.isArray(effect.buffIds) ? effect.buffIds : []).map((id) => String(id || "").trim()).filter(Boolean)
      const maxRemoved = Math.max(0, Math.trunc(Number(effect.maxRemoved || 0)))
      rows.push(
        buffIds.length > 0 ? rowBuffs(t("game.skillEffectRemovedStatuses"), buffIds) : null,
        maxRemoved > 0 ? row(t("game.skillEffectMaxRemoved"), String(maxRemoved)) : null
      )
    }

    return {
      key: `effect-${index + 1}`,
      title: `${t("game.skillEffectTitle")} ${index + 1}`,
      rows: compactRows(rows)
    }
  })
}

function isSelfPreferredWithoutSecondarySeek(targeting: EffectTargeting): boolean {
  const secondarySeek = String(targeting.secondarySeek || "").trim().toLowerCase()
  const selectRule = String(targeting.selectRule || "").trim().toLowerCase()
  const targetScope = String(targeting.targetScope || "").trim().toLowerCase()
  const noSecondarySeek = secondarySeek === "" || secondarySeek === "none"
  const selfPreferred = selectRule === "self" || targetScope === "self"
  return noSecondarySeek && selfPreferred
}

function describeTargeting(targeting: EffectTargeting, t: TranslateFn): string {
  const positions = (Array.isArray(targeting.targetPositions) ? targeting.targetPositions : [])
    .map((value) => localizeTargetPosition(value, t))
    .filter(Boolean)
    .join(" / ")
  const seek = localizeSecondarySeek(targeting.secondarySeek || "front_to_rear", t)
  return [positions || t("game.skillFormulaUnknown"), seek].filter(Boolean).join(" · ")
}

function ignoreTargetPriorityRow(targeting: EffectTargeting, t: TranslateFn): EffectDetailRow | null {
  if (targeting.ignoreTargetPriority !== true) return null
  const positions = (Array.isArray(targeting.targetPositions) ? targeting.targetPositions : [])
    .map((value) => localizeTargetPosition(value, t))
    .filter(Boolean)
    .join(" / ") || t("game.skillFormulaUnknown")
  return row(
    t("game.skillTargetIgnorePriorityLabel"),
    t("game.skillTargetIgnorePriority", { positions })
  )
}

function describeAmountFormula(
  formula: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> } | undefined,
  t: TranslateFn
): string {
  if (!formula) return t("game.skillFormulaUnknown")
  return localizeAmountFormulaText(formula.baseAdd ?? 0, formula.terms ?? [], t)
}

function amountFormulaValues(formula: EffectNode["amountFormula"]): number[] {
  if (!formula) return []
  return [
    Number(formula.baseAdd || 0),
    ...(Array.isArray(formula.terms) ? formula.terms.flatMap((term) => [Number(term.coefficient || 0), Number(term.perSkillLevel || 0)]) : [])
  ].filter((value) => Number.isFinite(value) && value !== 0)
}

function resourceEffectDirection(effect: EffectNode): "gain" | "loss" | "mixed" {
  const values = amountFormulaValues(effect.amountFormula)
  if (values.length === 0) return "mixed"
  if (values.every((value) => value >= 0)) return "gain"
  if (values.every((value) => value <= 0)) return "loss"
  return "mixed"
}

function negateAmountFormula(formula: EffectNode["amountFormula"]): EffectNode["amountFormula"] {
  if (!formula) return formula
  return {
    ...formula,
    baseAdd: -Number(formula.baseAdd || 0),
    terms: (Array.isArray(formula.terms) ? formula.terms : []).map((term) => ({
      ...term,
      coefficient: -Number(term.coefficient || 0),
      perSkillLevel: -Number(term.perSkillLevel || 0)
    }))
  }
}

function describeStatusDuration(
  payload:
    | {
        durationBaseTurns?: number
        durationPerSkillLevelTurns?: number
        durationFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> }
      }
    | undefined,
  t: TranslateFn
): string {
  if (!payload) return t("game.skillFormulaUnknown")
  const base = Math.max(0, Math.trunc(Number(payload.durationBaseTurns || 0)))
  const perLevel = Number(payload.durationPerSkillLevelTurns || 0)
  const parts = [t("game.skillDurationTurns", { turns: base })]
  if (perLevel) parts.push(t("game.skillDurationPerLevel", { turns: trimTrailingZeros(perLevel) }))
  const durationFormulaText = localizeAmountFormulaText(payload.durationFormula?.baseAdd ?? 0, payload.durationFormula?.terms ?? [], t)
  if (durationFormulaText !== "0") {
    parts.push(t("game.skillFormulaFloor", { formula: durationFormulaText }))
  }
  return parts.join(" + ")
}

function describeApplyChance(payload: EffectNode["statusPayload"], t: TranslateFn): string | null {
  if (!payload || payload.applyChanceBase === undefined) return null
  const base = Number(payload.applyChanceBase)
  const perLevel = Number(payload.applyChancePerSkillLevel || 0)
  const formula = payload.applyChanceFormula
  const parts = [formatPercent(base)]
  if (Number.isFinite(perLevel) && perLevel !== 0) {
    parts.push(t("game.skillEffectChancePerLevel", { chance: formatPercent(perLevel) }))
  }
  if (formula && (Number(formula.baseAdd || 0) !== 0 || (formula.terms || []).length > 0)) {
    parts.push(t("game.skillEffectChanceFormula", { formula: describeAmountFormula(formula, t) }))
  }
  const min = Number(payload.applyChanceMin ?? 0)
  const max = Number(payload.applyChanceMax ?? 1)
  const fixedAtBase = Number.isFinite(base)
    && Math.abs(min - max) < 0.000001
    && Math.abs(base - min) < 0.000001
    && perLevel === 0
    && (!formula || (Number(formula.baseAdd || 0) === 0 && (formula.terms || []).length === 0))
  if (Number.isFinite(min) && Number.isFinite(max) && (min > 0 || max < 1) && !fixedAtBase) {
    parts.push(t("game.skillEffectChanceClamp", { min: formatPercent(min), max: formatPercent(max) }))
  }
  return parts.join(" · ")
}

function describeSummonDuration(payload: NonNullable<EffectNode["summonPayload"]>, t: TranslateFn): string {
  const formula = payload.durationFormula
  if (formula && (formula.terms || []).length > 0) {
    return describeAmountFormula(formula, t)
  }
  const formulaTurns = Math.max(0, Math.trunc(Number(formula?.baseAdd || 0)))
  if (formulaTurns > 0) return t("game.skillDurationTurns", { turns: formulaTurns })
  const turns = Math.max(0, Math.trunc(Number(payload.durationTurns || 0)))
  return turns > 0 ? t("game.skillDurationTurns", { turns }) : t("game.skillEffectSummonPermanent")
}

function describeSummonLimit(payload: NonNullable<EffectNode["summonPayload"]>, t: TranslateFn): EffectDetailRow | null {
  const formula = payload.maxCountFormula
  if (formula && (Number(formula.baseAdd || 0) !== 0 || (formula.terms || []).length > 0)) {
    return row(t("game.skillEffectSummonLimit"), describeAmountFormula(formula, t))
  }
  const count = Math.max(0, Math.trunc(Number(payload.maxCountPerUnit || 0)))
  return count > 0 ? row(t("game.skillEffectSummonLimit"), String(count)) : null
}

function describeSummonPolicies(payload: NonNullable<EffectNode["summonPayload"]>, t: TranslateFn): EffectDetailRow | null {
  const policies = [
    ["onCasterDeath", payload.onCasterDeath],
    ["onCapOverflow", payload.onCapOverflow],
    ["onDispel", payload.onDispel],
    ["onDurationEnd", payload.onDurationEnd]
  ].flatMap(([kind, value]) => {
    const normalized = String(value || "").trim()
    if (!normalized) return []
    const key = `game.itemDetailBattleHookSummonPolicyValue.${kind}.${normalized}`
    const localized = t(key)
    return [localized === key ? normalized : localized]
  })
  if (payload.persistAcrossBattles) policies.push(t("game.skillEffectSummonPersists"))
  const uniquePolicies = Array.from(new Set(policies))
  return uniquePolicies.length > 0 ? row(t("game.skillEffectSummonPolicies"), uniquePolicies.join(" / ")) : null
}

function describeSummonStat(
  attr: string,
  formula: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }> },
  t: TranslateFn
): string {
  const normalizedAttr = String(attr || "").trim().toLowerCase()
  const terms = Array.isArray(formula?.terms) ? formula.terms : []
  if (
    normalizedAttr === "hp"
    && Number(formula?.baseAdd || 0) === -1
    && terms.length === 1
    && String(terms[0]?.attrKey || "").trim().toLowerCase() === "max_hp"
    && Number(terms[0]?.coefficient || 0) > 0
    && Number(terms[0]?.perSkillLevel || 0) === 0
  ) {
    return `${localizeAttr(attr, t)} ${t("game.skillEffectSummonMaxHpRatio", { percent: formatPercent(Number(terms[0]?.coefficient || 0)) })}`
  }
  return `${localizeAttr(attr, t)} ${describeAmountFormula(formula, t)}`
}

function localizeEnemyName(enemyId: string, t: TranslateFn): string {
  const normalized = String(enemyId || "").trim()
  if (!normalized) return t("game.skillFormulaUnknown")
  const key = `enemy.${normalized}.name`
  const localized = t(key)
  return localized === key ? t("game.skillEffectSummonUnknownUnit") : localized
}

function localizeBuffName(buffId: string, t: TranslateFn): string {
  const normalized = String(buffId || "").trim()
  if (!normalized) return t("game.skillFormulaUnknown")
  const key = `buff.${normalized}.name`
  const localized = t(key)
  return localized === key ? t("game.skillEffectUnknownStatus") : localized
}

function localizeSkillName(skillId: string, t: TranslateFn): string {
  const normalized = String(skillId || "").trim()
  if (!normalized) return t("game.skillEffectUnknownSkill")
  const key = `skill.${normalized}.name`
  const localized = t(key)
  return localized === key ? t("game.skillEffectUnknownSkill") : localized
}

function localizeSummonSide(side: string, t: TranslateFn): string {
  const key = `game.skillEffectSummonSideValue.${String(side || "source").trim().toLowerCase()}`
  const localized = t(key)
  return localized === key ? t("game.skillEffectSummonSideValue.source") : localized
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "0%"
  return `${trimTrailingZeros(value * 100)}%`
}

function enemyOnlyDamageReductionPierceRow(
  spec: EffectNode["damageSpec"],
  t: TranslateFn
): EffectDetailRow | null {
  const pierce = Number(spec?.enemyOnlyDamageReductionPierce)
  if (!Number.isFinite(pierce) || pierce <= 0) return null
  const statuses = (Array.isArray(spec?.enemyOnlyDamageReductionPierceTargetStatuses)
    ? spec.enemyOnlyDamageReductionPierceTargetStatuses
    : [])
    .map((statusID) => String(statusID || "").trim())
    .filter(Boolean)
  const percent = formatPercent(Math.min(1, pierce))
  const value = statuses.length > 0
    ? t("game.skillEffectEnemyOnlyDamageReductionPierceWithStatuses", {
        percent,
        statuses: statuses.map((statusID) => localizeBuffName(statusID, t)).join(" / ")
      })
    : t("game.skillEffectEnemyOnlyDamageReductionPierce", { percent })
  return row(t("game.skillEffectEnemyOnlyDamageReductionPierceLabel"), value)
}

function hitDamageScalingRow(
  spec: EffectNode["damageSpec"],
  hitCount: EffectNode["hitCount"],
  unitAttributes: Record<string, number> | null,
  skillLevel: number,
  t: TranslateFn
): EffectDetailRow | null {
  const scaling = normalizeHitDamageScaling(spec?.hitDamageScaling)
  if (!scaling) return null
  const secondHit = formatPercent(scaling.secondHitMultiplier)
  const curvePower = formatEffectNumber(scaling.curvePower, 4)
  const anchorHit = 12
  const anchorMultiplier = formatPercent(hitDamageMultiplier(scaling, anchorHit))
  const cap = approximateHitDamageEquivalentCap(scaling)
  const capText = cap
    ? t(cap.converged ? "game.skillEffectHitDamageFalloffCap" : "game.skillEffectHitDamageFalloffCapLowerBound", {
        equivalent: formatEffectNumber(cap.equivalentHits, 4)
      })
    : ""
  const value = [
    t("game.skillEffectHitDamageFalloffRule", { secondHit, curvePower }),
    t("game.skillEffectHitDamageFalloffAnchor", { hit: anchorHit, multiplier: anchorMultiplier }),
    capText
  ].filter(Boolean).join(" ")

  const base = Math.max(1, Math.trunc(Number(hitCount?.base || 1)))
  const formula = Array.isArray(hitCount?.formula) ? hitCount.formula : []
  const evaluated = formula.length > 0
    ? evaluateTargetCountPreview({ base, formula }, unitAttributes, skillLevel)
    : base
  const count = evaluated === null
    ? null
    : Math.max(1, Math.min(MAX_RUNTIME_DAMAGE_HITS, Math.trunc(evaluated)))
  const preview = count === null
    ? undefined
    : t("game.skillEffectHitDamageFalloffCurrent", {
        hits: count,
        finalMultiplier: formatPercent(hitDamageMultiplier(scaling, count)),
        equivalent: formatEffectNumber(hitDamageEquivalentSum(scaling, count), 4)
      })
  return row(t("game.skillEffectHitDamageFalloffLabel"), value, preview)
}

function statusPayloadRow(statusIds: string[], t: TranslateFn): EffectDetailRow {
  const normalized = statusIds.map((statusID) => String(statusID || "").trim()).filter(Boolean)
  if (normalized.length <= 0) return row(t("game.skillEffectStatusIds"), t("game.skillFormulaUnknown"))
  return rowBuffs(t("game.skillEffectStatusIds"), normalized)
}

function requiredStatusRow(label: string, statusIds: string[] | undefined): EffectDetailRow | null {
  const normalized = (Array.isArray(statusIds) ? statusIds : []).map((statusID) => String(statusID || "").trim()).filter(Boolean)
  if (normalized.length <= 0) return null
  return rowBuffs(label, normalized)
}

function targetCountMaxRow(targetCount: EffectTargeting["targetCount"], t: TranslateFn): EffectDetailRow | null {
  const maxText = localizeTargetCountMaxText(targetCount, t)
  if (maxText === null) return null
  return row(t("game.skillTargetMaxCount"), maxText, undefined, t("game.skillTargetMaxCountHint"))
}

function localizeEffectKind(effect: EffectNode, t: TranslateFn): string {
  const kind = String(effect.kind || "").trim().toLowerCase()
  if (kind === "resource") {
    const direction = resourceEffectDirection(effect)
    if (direction === "loss") {
      const scope = String(effect.targeting?.targetScope || "").trim().toLowerCase()
      return t(scope === "enemy" ? "game.skillEffectKind.resource_drain" : "game.skillEffectKind.resource_cost")
    }
    if (direction === "mixed") return t("game.skillEffectKind.resource_change")
  }
  const key = `game.skillEffectKind.${kind}`
  const localized = t(key)
  return localized === key ? t("game.skillEffectKind.unknown") : localized
}

function localizeDamageType(type: string, t: TranslateFn): string {
  const key = `game.damageType.${type}`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookDamageTypeOther") : localized
}

function localizeResource(resource: string, t: TranslateFn): string {
  const normalized = String(resource || "").trim().toLowerCase()
  if (normalized === "mp") {
    const localized = t("game.family.mana")
    return localized === "game.family.mana" ? "MP" : localized
  }
  if (normalized === "hp") {
    const localized = t("attr.hp.name")
    return localized === "attr.hp.name" ? "HP" : localized
  }
  const key = `attr.${normalized}.name`
  const localized = t(key)
  return localized === key ? (normalized || resource) : localized
}

function localizeTargetPosition(position: string, t: TranslateFn): string {
  const key = `game.targetDirection.${position}`
  const localized = t(key)
  return localized === key ? t("game.skillTargetPositionUnknown") : localized
}

function localizeSecondarySeek(strategy: string, t: TranslateFn): string {
  const key = `game.targetSeekStrategy.${strategy}`
  const localized = t(key)
  return localized === key ? t("game.targetSeekStrategy.none") : localized
}

function localizeTargetScope(scope: string, t: TranslateFn): string {
  const key = `game.skillTargetScope.${scope}`
  const localized = t(key)
  return localized === key ? t("game.skillTargetScope.enemy") : localized
}

function localizeSelectRule(rule: string, attrKey: string, t: TranslateFn): string {
  const key = `game.skillSelectRule.${rule}`
  const localized = t(key)
  const label = localized === key ? t("game.skillSelectRule.random") : localized
  if ((rule === "attr_max" || rule === "attr_min") && attrKey) {
    return `${label}: ${localizeAttr(attrKey, t)}`
  }
  return label
}

function localizeAttr(attrKey: string, t: TranslateFn): string {
  return localizeEffectAttr(attrKey, t)
}

function onlyWhenNoTargetRow(refs: number[] | undefined, t: TranslateFn): EffectDetailRow | null {
  const normalized = (Array.isArray(refs) ? refs : [])
    .map((value) => Math.trunc(Number(value)))
    .filter((value) => Number.isFinite(value) && value > 0)
  if (normalized.length <= 0) return null
  return row(
    t("game.effectOnlyWhenNoTargetFromEffects"),
    normalized.map((index) => t("game.effectRefIndex", { index })).join(", ")
  )
}

function reuseTargetsRow(refIndex: number | undefined, t: TranslateFn): EffectDetailRow | null {
  const normalized = Math.trunc(Number(refIndex || 0))
  if (!Number.isFinite(normalized) || normalized <= 0) return null
  return row(t("game.effectReuseTargetsFromEffect"), t("game.effectRefIndex", { index: normalized }))
}

function previewValue(value: number | null): string | undefined {
  if (value === null || !Number.isFinite(value)) return undefined
  return `(${trimTrailingZeros(value)})`
}

function compactRows(rows: Array<EffectDetailRow | null | undefined>): EffectDetailRow[] {
  return rows.filter((row): row is EffectDetailRow => {
    if (!row) return false
    if (Array.isArray(row.buffIds) && row.buffIds.length > 0) return true
    return String(row.value || "").trim() !== ""
  })
}

function trimTrailingZeros(value: number): string {
  const normalized = Number(value)
  if (!Number.isFinite(normalized)) return "0"
  return normalized.toFixed(4).replace(/\.?0+$/, "")
}

function row(label: string, value: string, preview?: string, labelHint?: string): EffectDetailRow {
  return { label, value, preview, labelHint }
}

function rowBuffs(label: string, buffIds: string[]): EffectDetailRow {
  return { label, value: "", buffIds }
}
