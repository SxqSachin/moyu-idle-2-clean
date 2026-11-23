import {
  evaluateAmountFormulaPreview,
  formatEffectNumber,
  localizeAmountFormulaText,
  localizeEffectAttr,
  localizeTargetCountText
} from "@services/effect-formula-display"
import { getLocalizedBuffName } from "@services/buff-definition"

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

function localizedToken(t: TranslateFn, key: string, fallback: string): string {
  const value = t(key)
  return value === key ? fallback : value
}

function inlineSeparator(t: TranslateFn): string {
  return localizedToken(t, "game.itemDetailBattleHookInlineSeparator", "，")
}

function clauseSeparator(t: TranslateFn): string {
  return localizedToken(t, "game.itemDetailBattleHookClauseSeparator", "；")
}

function listSeparator(t: TranslateFn): string {
  return localizedToken(t, "game.itemDetailBattleHookListSeparator", "、")
}

export type BattleHookDisplayEffect =
  | { key: string; kind: "add_buff"; targetText: string; buffId: string; turns: number | null; procChanceText: string | null; buffPayload?: Record<string, unknown>; buffPayloadFormula?: Record<string, unknown> }
  | { key: string; kind: "remove_buff"; targetText: string; buffId: string; removeStack: boolean }
  | { key: string; kind: "heal"; targetText: string; formulaText: string; procChanceText: string | null }
  | { key: string; kind: "summon"; targetText: string; unitId: string; durationText: string; countText: string | null; procChanceText: string | null; statBonusEntries: Array<{ key: string; label: string; value: string }> }
  | { key: string; kind: "consume_mp"; formulaText: string }
  | { key: string; kind: "mirror_cast_skill"; descriptionText: string; filterText: string | null }
  | { key: string; kind: "basic_attack_marked_target"; effectKind: string; procChanceText: string | null }
  | { key: string; kind: "text"; text: string }

export type BattleHookDisplayNode = {
  key: string
  index: number
  triggerText: string
  conditions: Array<{ key: string; text: string }>
  effects: BattleHookDisplayEffect[]
  hidden: boolean
}

export type BattleHookDisplayOptions = {
  equipmentEnhanceLevel?: number
  sourceKind?: "equipment" | "specialization"
  unitAttributes?: Record<string, number>
}

export function buildBattleHookDescriptions(rawHooks: unknown, t: TranslateFn, options: BattleHookDisplayOptions = {}): BattleHookDisplayNode[] {
  const hooks = Array.isArray(rawHooks) ? rawHooks : []
  const out: BattleHookDisplayNode[] = []
  for (let index = 0; index < hooks.length; index += 1) {
    const hook = hooks[index]
    if (!hook || typeof hook !== "object" || Array.isArray(hook)) continue
    const row = hook as Record<string, unknown>
    const hidden = row.hideDescription === true
    const trigger = normalizeDslToken(row.trigger)
    const summonDefeatedTriggerText = trigger === "on_unit_death"
      ? localizeSummonDefeatedTrigger(row.subjectFilter ?? row.subject_filter, t)
      : null
    const triggerText = summonDefeatedTriggerText || localizeBattleHookTrigger(trigger, t)
    const conditionsRaw = Array.isArray(row.conditions) ? row.conditions : []
    const hasCompletedTurnCycle = conditionsRaw.some(isCompletedTurnCycleCondition)
    const conditions = conditionsRaw
      .map((condition, conditionIndex) => {
        const text = isCompletedTurnCycleCondition(condition)
          ? localizeCompletedTurnCycleCondition(condition, row.eventFilter, t)
          : localizeBattleHookCondition(condition, t)
        if (!text) return null
        return { key: `condition-${conditionIndex + 1}`, text }
      })
      .filter((entry): entry is { key: string; text: string } => !!entry)
    conditions.push(...localizeBattleHookEventFilter(row.eventFilter, t, {
      omitTurnScope: hasCompletedTurnCycle
    }).map((text, eventFilterIndex) => ({
      key: `event-filter-${eventFilterIndex + 1}`,
      text
    })))
    if (!summonDefeatedTriggerText) {
      conditions.push(...localizeBattleHookSubjectFilter(row.subjectFilter ?? row.subject_filter, t).map((text, subjectFilterIndex) => ({
        key: `subject-filter-${subjectFilterIndex + 1}`,
        text
      })))
    }
    conditions.push(...localizeBattleHookLimits(row, t).map((text, limitIndex) => ({
      key: `trigger-limit-${limitIndex + 1}`,
      text
    })))
    conditions.push(...localizeBattleHookActivationPolicy(row, t).map((text, policyIndex) => ({
      key: `activation-policy-${policyIndex + 1}`,
      text
    })))
    const effectsRaw = Array.isArray(row.effects) ? row.effects : []
    const effects = effectsRaw
      .map((effect, effectIndex) => {
        const display = localizeBattleHookEffect(effect, t, options)
        if (!display) return null
        if (display.kind === "add_buff" || display.kind === "remove_buff" || display.kind === "heal" || display.kind === "summon" || display.kind === "consume_mp" || display.kind === "mirror_cast_skill" || display.kind === "basic_attack_marked_target") {
          const base = { key: `effect-${effectIndex + 1}`, kind: display.kind }
          if (display.kind === "summon") {
            return { ...base, targetText: display.targetText, unitId: display.unitId, durationText: display.durationText, countText: display.countText, procChanceText: display.procChanceText, statBonusEntries: display.statBonusEntries }
          }
          if (display.kind === "heal") {
            return { ...base, targetText: display.targetText, formulaText: display.formulaText, procChanceText: display.procChanceText }
          }
          if (display.kind === "remove_buff") {
            return { ...base, targetText: display.targetText, buffId: display.buffId, removeStack: display.removeStack }
          }
          if (display.kind === "consume_mp") {
            return { ...base, formulaText: display.formulaText }
          }
          if (display.kind === "mirror_cast_skill") {
            return { ...base, descriptionText: display.descriptionText, filterText: display.filterText }
          }
          if (display.kind === "basic_attack_marked_target") {
            return { ...base, effectKind: display.effectKind, procChanceText: display.procChanceText }
          }
          return { ...base, targetText: display.targetText, buffId: display.buffId, turns: display.turns, procChanceText: display.procChanceText, ...(display.buffPayload ? { buffPayload: display.buffPayload } : {}), ...(display.buffPayloadFormula ? { buffPayloadFormula: display.buffPayloadFormula } : {}) }
        }
        return {
          key: `effect-${effectIndex + 1}`,
          kind: "text" as const,
          text: display.text
        }
      })
      .filter((entry): entry is BattleHookDisplayEffect => !!entry)
    if (effects.length === 0 && !hidden) continue
    out.push({
      key: String((row as { id?: string }).id || `hook-${index + 1}`),
      index: index + 1,
      triggerText,
      conditions,
      effects,
      hidden
    })
  }
  return out
}

function localizeBattleHookSubjectFilter(raw: unknown, t: TranslateFn): string[] {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
  const filter = raw as Record<string, unknown>
  const summonTemplateIDs = normalizeStringList(filter.includeSummonTemplateIds ?? filter.include_summon_template_ids)
  if (summonTemplateIDs.length > 0) {
    const units = Array.from(new Set(summonTemplateIDs.map((id) => localizeEnemyName(id, t)))).join(" / ")
    return [t("game.itemDetailBattleHookSubjectSummons", { units })]
  }
  const unitTypes = normalizeStringList(filter.includeUnitTypes ?? filter.include_unit_types)
  if (unitTypes.includes("summon")) return [t("game.itemDetailBattleHookSubjectAnySummon")]
  return []
}

function localizeSummonDefeatedTrigger(raw: unknown, t: TranslateFn): string | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const filter = raw as Record<string, unknown>
  const unitTypes = normalizeStringList(filter.includeUnitTypes ?? filter.include_unit_types)
  if (!unitTypes.includes("summon")) return null
  const summonTemplateIDs = normalizeStringList(filter.includeSummonTemplateIds ?? filter.include_summon_template_ids)
  if (summonTemplateIDs.length === 0) return t("game.itemDetailBattleHookTriggerAnySummonDefeated")
  const units = Array.from(new Set(summonTemplateIDs.map((id) => localizeEnemyName(id, t)))).join(" / ")
  return t("game.itemDetailBattleHookTriggerSummonDefeated", { units })
}

function localizeBattleHookTrigger(trigger: string, t: TranslateFn): string {
  const normalized = normalizeDslToken(trigger)
  const key = `game.itemDetailBattleHookTrigger.${normalized}`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookTriggerUnknown") : localized
}

function localizeBattleHookEventFilter(
  raw: unknown,
  t: TranslateFn,
  options: { omitTurnScope?: boolean } = {}
): string[] {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
  const filter = raw as Record<string, unknown>
  const out: string[] = []
  const probabilityKinds = normalizeStringList(filter.probabilityKinds ?? filter.probability_kinds)
  if (probabilityKinds.length > 0) {
    out.push(t("game.itemDetailBattleHookEventProbabilityKinds", {
      kinds: probabilityKinds.map((kind) => localizeProbabilityKind(kind, t)).join(" / ")
    }))
  }
  const sourceIDs = normalizeStringList(filter.sourceIds ?? filter.source_ids ?? filter.probabilitySourceIds ?? filter.probability_source_ids)
  if (sourceIDs.length > 0) {
    out.push(t("game.itemDetailBattleHookEventProbabilitySources", {
      sources: localizeProbabilitySourceNames(sourceIDs, t)
    }))
  }
  const excludedSourceIDs = normalizeStringList(filter.excludeSourceIds ?? filter.exclude_source_ids ?? filter.excludeProbabilitySourceIds ?? filter.exclude_probability_source_ids)
  if (excludedSourceIDs.length > 0) {
    out.push(t("game.itemDetailBattleHookEventExcludeProbabilitySources", {
      sources: localizeProbabilitySourceNames(excludedSourceIDs, t)
    }))
  }
  const buffIDs = normalizeStringList(filter.buffIds ?? filter.buff_ids)
  if (buffIDs.length > 0) {
    out.push(t("game.itemDetailBattleHookEventBuffIds", {
      buffs: buffIDs.map((buffID) => localizeBuffName(buffID, t)).join(" / ")
    }))
  }
  const excludedBuffIDs = normalizeStringList(filter.excludeBuffIds ?? filter.exclude_buff_ids)
  if (excludedBuffIDs.length > 0) {
    out.push(t("game.itemDetailBattleHookEventExcludeBuffIds", {
      buffs: excludedBuffIDs.map((buffID) => localizeBuffName(buffID, t)).join(" / ")
    }))
  }
  const buffKinds = normalizeStringList(filter.buffKinds ?? filter.buff_kinds)
  if (buffKinds.length > 0) {
    out.push(t("game.itemDetailBattleHookEventBuffKinds", {
      kinds: buffKinds.map((kind) => t(`game.itemDetailBattleHookBuffKind.${kind}`)).join(" / ")
    }))
  }
  if (filter.firstBuffInstanceOnly === true || filter.first_buff_instance_only === true) {
    out.push(t("game.itemDetailBattleHookEventFirstBuffInstanceOnly"))
  }
  const applicationKinds = normalizeStringList(filter.applicationKinds ?? filter.application_kinds)
  if (applicationKinds.length > 0) {
    out.push(t("game.itemDetailBattleHookEventApplicationKinds", {
      kinds: applicationKinds.map((kind) => {
        const key = `game.itemDetailBattleHookApplicationKind.${kind}`
        const localized = t(key)
        return localized === key ? kind : localized
      }).join(" / ")
    }))
  }
  if (filter.excludeSelf === true || filter.exclude_self === true) {
    out.push(t("game.itemDetailBattleHookEventExcludeSelf"))
  }
  if (!options.omitTurnScope) {
    const turnScope = localizeBattleHookTurnScope(filter, t)
    if (turnScope) out.push(turnScope)
  }
  return out
}

function localizeBattleHookTurnScope(filter: Record<string, unknown>, t: TranslateFn): string | null {
  const turnKinds = normalizeStringList(filter.turnKinds ?? filter.turn_kinds)
  const hasPreparationRule = typeof filter.includePreparation === "boolean" || typeof filter.include_preparation === "boolean"
  const hasSuppressedRule = typeof filter.includeSuppressedTurns === "boolean" || typeof filter.include_suppressed_turns === "boolean"
  if (turnKinds.length === 0 && !hasPreparationRule && !hasSuppressedRule) return null

  const kindSet = new Set(turnKinds)
  const scopeKey = kindSet.has("ordered") && kindSet.has("extra")
    ? "orderedAndExtra"
    : kindSet.has("extra")
      ? "extra"
      : kindSet.has("ordered")
        ? "ordered"
        : "all"
  const scope = t(`game.itemDetailBattleHookTurnScope.${scopeKey}`)
  const notes: string[] = []
  if (filter.includePreparation === false || filter.include_preparation === false) {
    notes.push(t("game.itemDetailBattleHookTurnScopeNote.excludePreparation"))
  } else if (filter.includePreparation === true || filter.include_preparation === true) {
    notes.push(t("game.itemDetailBattleHookTurnScopeNote.includePreparation"))
  }
  if (filter.includeSuppressedTurns === true || filter.include_suppressed_turns === true) {
    notes.push(t("game.itemDetailBattleHookTurnScopeNote.includeSuppressed"))
  } else if (filter.includeSuppressedTurns === false || filter.include_suppressed_turns === false) {
    notes.push(t("game.itemDetailBattleHookTurnScopeNote.excludeSuppressed"))
  }
  return notes.length > 0
    ? t("game.itemDetailBattleHookTurnScopeWithNotes", { scope, notes: notes.join(t("game.itemDetailBattleHookTurnScopeNoteSeparator")) })
    : scope
}

function localizeBattleHookLimits(row: Record<string, unknown>, t: TranslateFn): string[] {
  const out: string[] = []
  const limits = row.limits && typeof row.limits === "object" && !Array.isArray(row.limits)
    ? row.limits as Record<string, unknown>
    : {}
  const perTurn = Math.max(0, Math.trunc(toFiniteNumber(
    limits.perTurn ?? limits.per_turn ?? row.maxTriggersPerTurn ?? row.max_triggers_per_turn
  ) ?? 0))
  const perWave = Math.max(0, Math.trunc(toFiniteNumber(
    limits.perWave ?? limits.per_wave ?? row.maxTriggersPerWave ?? row.max_triggers_per_wave
  ) ?? 0))
  const perBattle = Math.max(0, Math.trunc(toFiniteNumber(
    limits.perBattle ?? limits.per_battle ?? row.maxTriggersPerBattle ?? row.max_triggers_per_battle
  ) ?? 0))
  if (perTurn > 0) out.push(t("game.itemDetailBattleHookMaxTriggersPerTurn", { count: perTurn }))
  if (perWave > 0) out.push(t("game.itemDetailBattleHookMaxTriggersPerWave", { count: perWave }))
  if (perBattle > 0) out.push(t("game.itemDetailBattleHookMaxTriggersPerBattle", { count: perBattle }))
  return out
}

function localizeBattleHookActivationPolicy(row: Record<string, unknown>, t: TranslateFn): string[] {
  const out: string[] = []
  const rawCost = row.activationCost ?? row.activation_cost
  const cost = rawCost && typeof rawCost === "object" && !Array.isArray(rawCost)
    ? rawCost as Record<string, unknown>
    : {}
  const hpRatio = Math.max(0, toFiniteNumber(cost.maxHpRatio ?? cost.max_hp_ratio) ?? 0)
  const mpRatio = Math.max(0, toFiniteNumber(cost.maxMpRatio ?? cost.max_mp_ratio) ?? 0)
  if (hpRatio > 0 && mpRatio > 0) {
    out.push(t("game.itemDetailBattleHookActivationCostBoth", {
      hp: formatUnsignedPercent(hpRatio),
      mp: formatUnsignedPercent(mpRatio)
    }))
  } else if (hpRatio > 0) {
    out.push(t("game.itemDetailBattleHookActivationCostHp", { hp: formatUnsignedPercent(hpRatio) }))
  } else if (mpRatio > 0) {
    out.push(t("game.itemDetailBattleHookActivationCostMp", { mp: formatUnsignedPercent(mpRatio) }))
  }
  const cooldown = Math.max(0, Math.trunc(toFiniteNumber(row.cooldownCompletedTurns ?? row.cooldown_completed_turns) ?? 0))
  if (cooldown > 0) out.push(t("game.itemDetailBattleHookCompletedTurnCooldown", { count: cooldown }))
  return out
}

function localizeBattleHookEffect(effect: unknown, t: TranslateFn, options: BattleHookDisplayOptions = {}):
  | { kind: "add_buff"; targetText: string; buffId: string; turns: number | null; procChanceText: string | null; buffPayload?: Record<string, unknown>; buffPayloadFormula?: Record<string, unknown> }
  | { kind: "remove_buff"; targetText: string; buffId: string; removeStack: boolean }
  | { kind: "heal"; targetText: string; formulaText: string; procChanceText: string | null }
  | { kind: "summon"; targetText: string; unitId: string; durationText: string; countText: string | null; procChanceText: string | null; statBonusEntries: Array<{ key: string; label: string; value: string }> }
  | { kind: "consume_mp"; formulaText: string }
  | { kind: "mirror_cast_skill"; descriptionText: string; filterText: string | null }
  | { kind: "basic_attack_marked_target"; effectKind: string; procChanceText: string | null }
  | { kind: "text"; text: string }
  | null {
  if (!effect || typeof effect !== "object" || Array.isArray(effect)) return null
  const row = effect as Record<string, unknown>
  const kindRaw = normalizeDslToken(row.kind || row.type)
  const kindToken = kindRaw.replace(/[^a-z]/g, "")
  if (kindRaw === "force_probability_failure" || kindToken === "forceprobabilityfailure") {
    return {
      kind: "text",
      text: appendProbabilityFilter(t("game.itemDetailBattleHookEffectForceProbabilityFailure"), row, t)
    }
  }
  if (kindRaw === "add_probability" || kindToken === "addprobability" || kindRaw === "set_probability" || kindToken === "setprobability") {
    const value = toFiniteNumber(row.value ?? row.amount) ?? 0
    const formulaText = localizeBattleHookPercentFormula(row.valueFormula ?? row.amountFormula ?? row.value_formula ?? row.amount_formula, t)
    const valueText = kindRaw === "set_probability" || kindToken === "setprobability"
      ? formatUnsignedPercent(value)
      : formatSignedPercent(value)
    const key = kindRaw === "set_probability" || kindToken === "setprobability"
      ? "game.itemDetailBattleHookEffectSetProbability"
      : "game.itemDetailBattleHookEffectAddProbability"
    return {
      kind: "text",
      text: appendProbabilityFilter(t(key, { value: valueText, formula: formulaText || "" }), row, t)
    }
  }
  if (kindRaw === "add_buff" || kindRaw === "addbuff") {
    const buffId = String(row.buffId || "").trim()
    if (!buffId) return null
    const turns = Number(row.durationTurns)
    const targetModeRaw = String(row.targetMode || row.target || "").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" || targetModeRaw === "enemy"
      ? targetModeRaw
      : "self"
    const selectionMode = String(row.selectionMode || row.selection_mode || "").trim().toLowerCase()
    let targetText = targetMode === "enemy"
      ? localizeSpecializationSelection(selectionMode || "enemy", t)
      : localizeBattleHookEffectTargetMode(targetMode, t, options)
    if (targetMode === "targeting") {
      const targeting = row.targeting && typeof row.targeting === "object"
        ? row.targeting as {
            targetPositions?: string[]
            secondarySeek?: string
            targetScope?: string
            selectRule?: string
            selectAttrKey?: string
            targetCount?: {
              base?: number
              formula?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }>
            }
          }
        : {}
      const positions = Array.isArray(targeting.targetPositions) ? targeting.targetPositions : []
      const positionText = localizeTargetPositions(positions.length > 0 ? positions : ["vanguard"], t)
      const scopeText = localizeTargetScope(String(targeting.targetScope || "enemy"), t)
      const seekText = localizeTargetSeekStrategy(String(targeting.secondarySeek || "front_to_rear"), t)
      const selectText = localizeSelectRule(String(targeting.selectRule || "first"), String(targeting.selectAttrKey || ""), t)
      const targetCount = targeting.targetCount && typeof targeting.targetCount === "object"
        ? targeting.targetCount as { base?: number; formula?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
        : { base: 1, formula: [] }
      const countText = localizeTargetCountText({
        base: Number(targetCount.base || 1),
        formula: Array.isArray(targetCount.formula) ? targetCount.formula : []
      }, t)
      targetText = localizeBattleHookTargetingText({
        scopeText,
        positionText,
        seekText,
        selectText,
        countText,
        targetCountBase: Number(targetCount.base || 1)
      }, t)
    }
    return {
      kind: "add_buff",
      targetText,
      buffId,
      turns: Number.isFinite(turns) && turns > 0 ? Math.round(turns) : null,
      procChanceText: localizeBattleHookProcChance(row, t),
      ...(() => {
        const buffPayload = resolveBattleHookPreviewPayload(row, Number(options.equipmentEnhanceLevel || 0))
        const buffPayloadFormula = resolveBattleHookPayloadFormula(row)
        return {
          ...(buffPayload ? { buffPayload } : {}),
          ...(buffPayloadFormula ? { buffPayloadFormula } : {})
        }
      })()
    }
  }
  if (kindRaw === "remove_buff" || kindRaw === "removebuff") {
    const buffId = String(row.buffId || row.stateId || "").trim()
    if (!buffId) return null
    const targetModeRaw = String(row.targetMode || row.target || "").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" || targetModeRaw === "target"
      ? targetModeRaw
      : "self"
    return {
      kind: "remove_buff",
      targetText: localizeBattleHookEffectTargetMode(targetMode, t, options),
      buffId,
      removeStack: row.removeStack === true
    }
  }
  if (kindRaw === "remove_buff_by_kind" || kindToken === "removebuffbykind") {
    const buffKind = String(row.buffKind || row.buff_kind || "debuff").trim().toLowerCase()
    const targetModeRaw = String(row.targetMode || row.target || "source").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
    const maxBuffs = Math.max(1, Math.trunc(toFiniteNumber(row.maxBuffs ?? row.max_buffs) ?? 1))
    const selectionMode = String(row.selectionMode || row.selection_mode || "max_remaining").trim().toLowerCase()
    const text = t("game.itemDetailBattleHookEffectRemoveBuffByKind", {
      target: localizeBattleHookEffectTargetMode(targetMode, t, options),
      kind: t(`game.itemDetailBattleHookBuffKind.${buffKind}`),
      count: String(maxBuffs),
      selection: t(`game.itemDetailBattleHookBuffSelection.${selectionMode}`),
      dispellable: row.dispellableOnly === true || row.dispellable_only === true
        ? t("game.itemDetailBattleHookEffectDispellableOnly")
        : ""
    })
    const procChanceText = localizeBattleHookProcChance(row, t)
    return { kind: "text", text: procChanceText ? `${text}${inlineSeparator(t)}${procChanceText}` : text }
  }
  if (kindRaw === "heal") {
    const targetModeRaw = String(row.targetMode || row.target || "").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" || targetModeRaw === "self" || targetModeRaw === "lowest_hp_ally"
      ? targetModeRaw
      : "targeting"
    let targetText = localizeBattleHookEffectTargetMode(targetMode, t, options)
    if (targetMode === "targeting") {
      const targeting = row.targeting && typeof row.targeting === "object"
        ? row.targeting as { targetPositions?: string[]; secondarySeek?: string; targetScope?: string; selectRule?: string; selectAttrKey?: string; targetCount?: { base?: number; formula?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> } }
        : {}
      const positions = Array.isArray(targeting.targetPositions) ? targeting.targetPositions : []
      const positionText = localizeTargetPositions(positions.length > 0 ? positions : ["vanguard"], t)
      const scopeText = localizeTargetScope(String(targeting.targetScope || "enemy"), t)
      const seekText = localizeTargetSeekStrategy(String(targeting.secondarySeek || "front_to_rear"), t)
      const selectText = localizeSelectRule(String(targeting.selectRule || "first"), String(targeting.selectAttrKey || ""), t)
      const targetCount = targeting.targetCount && typeof targeting.targetCount === "object"
        ? targeting.targetCount as { base?: number; formula?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
        : { base: 1, formula: [] }
      const countText = localizeTargetCountText({
        base: Number(targetCount.base || 1),
        formula: Array.isArray(targetCount.formula) ? targetCount.formula : []
      }, t)
      targetText = localizeBattleHookTargetingText({
        scopeText,
        positionText,
        seekText,
        selectText,
        countText,
        targetCountBase: Number(targetCount.base || 1)
      }, t)
    }
    const missingHpRatio = toFiniteNumber(row.missingHpRatio ?? row.missing_hp_ratio)
    const maxHpRatio = toFiniteNumber(row.maxHpRatio ?? row.max_hp_ratio)
    const formulaRaw = row.amountFormula && typeof row.amountFormula === "object" && !Array.isArray(row.amountFormula)
      ? row.amountFormula as { baseAdd?: unknown; terms?: unknown[] }
      : { baseAdd: row.value ?? 0, terms: [] }
    const fallbackRatio = missingHpRatio ?? maxHpRatio ?? toFiniteNumber(row.value) ?? 0
    const projection = specializationValueFormulaProjection(row, fallbackRatio, t, options)
    const formulaText = missingHpRatio !== null
      ? appendFormulaProjection(t("game.specialization.dsl.effect.healMissingHp", { value: formatUnsignedPercent(projection.value) }), projection)
      : maxHpRatio !== null
        ? appendFormulaProjection(t("game.specialization.dsl.effect.healMaxHp", { value: formatUnsignedPercent(projection.value) }), projection)
        : localizeAmountFormulaText(formulaRaw.baseAdd, formulaRaw.terms, t)
    return {
      kind: "heal",
      targetText,
      formulaText,
      procChanceText: localizeBattleHookProcChance(row, t)
    }
  }
  if (kindRaw === "damage" || kindToken === "damage") {
    const targetModeRaw = String(row.targetMode || row.target || "").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" ? targetModeRaw : targetModeRaw === "self" ? "self" : "targeting"
    let targetText = localizeBattleHookEffectTargetMode(targetMode, t, options)
    if (targetMode === "targeting") {
      const targeting = row.targeting && typeof row.targeting === "object"
        ? row.targeting as { targetPositions?: string[]; secondarySeek?: string; targetScope?: string; selectRule?: string; selectAttrKey?: string; targetCount?: { base?: number; formula?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> } }
        : {}
      const positions = Array.isArray(targeting.targetPositions) ? targeting.targetPositions : []
      const positionText = localizeTargetPositions(positions.length > 0 ? positions : ["vanguard"], t)
      const scopeText = localizeTargetScope(String(targeting.targetScope || "enemy"), t)
      const seekText = localizeTargetSeekStrategy(String(targeting.secondarySeek || "front_to_rear"), t)
      const selectText = localizeSelectRule(String(targeting.selectRule || "first"), String(targeting.selectAttrKey || ""), t)
      const targetCount = targeting.targetCount && typeof targeting.targetCount === "object"
        ? targeting.targetCount as { base?: number; formula?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
        : { base: 1, formula: [] }
      const countText = localizeTargetCountText({
        base: Number(targetCount.base || 1),
        formula: Array.isArray(targetCount.formula) ? targetCount.formula : []
      }, t)
      targetText = localizeBattleHookTargetingText({
        scopeText,
        positionText,
        seekText,
        selectText,
        countText,
        targetCountBase: Number(targetCount.base || 1)
      }, t)
    }
    const damageSpec = row.damageSpec && typeof row.damageSpec === "object" && !Array.isArray(row.damageSpec)
      ? row.damageSpec as Record<string, unknown>
      : {}
    const formulaRaw = row.amountFormula && typeof row.amountFormula === "object" && !Array.isArray(row.amountFormula)
      ? row.amountFormula as { baseAdd?: unknown; terms?: unknown[] }
      : damageSpec.amountFormula && typeof damageSpec.amountFormula === "object" && !Array.isArray(damageSpec.amountFormula)
        ? damageSpec.amountFormula as { baseAdd?: unknown; terms?: unknown[] }
        : { baseAdd: row.value ?? row.amount ?? 0, terms: [] }
    const damageType = String(row.damageType || damageSpec.damageType || "physical").trim().toLowerCase()
    const damageTypeText = localizeBattleHookDamageTypes([damageType], t)
    const relationFormulas = row.amountFormulaByRelation && typeof row.amountFormulaByRelation === "object" && !Array.isArray(row.amountFormulaByRelation)
      ? row.amountFormulaByRelation as Record<string, unknown>
      : null
    const allyFormula = relationFormulas?.ally && typeof relationFormulas.ally === "object" && !Array.isArray(relationFormulas.ally)
      ? relationFormulas.ally as { baseAdd?: unknown; terms?: unknown[] }
      : null
    const enemyFormula = relationFormulas?.enemy && typeof relationFormulas.enemy === "object" && !Array.isArray(relationFormulas.enemy)
      ? relationFormulas.enemy as { baseAdd?: unknown; terms?: unknown[] }
      : null
    let text = allyFormula && enemyFormula
      ? t("game.itemDetailBattleHookEffectDamageByRelation", {
          target: targetText,
          damageType: damageTypeText,
          allyFormula: localizeAmountFormulaText(allyFormula.baseAdd, allyFormula.terms, t),
          enemyFormula: localizeAmountFormulaText(enemyFormula.baseAdd, enemyFormula.terms, t)
        })
      : t("game.itemDetailBattleHookEffectDamage", {
          target: targetText,
          damageType: damageTypeText,
          formula: localizeAmountFormulaText(formulaRaw.baseAdd, formulaRaw.terms, t)
        })
    if (row.simultaneous === true) text = `${text}${clauseSeparator(t)}${t("game.itemDetailBattleHookEffectDamageSimultaneous")}`
    const procChanceText = localizeBattleHookProcChance(row, t)
    return {
      kind: "text",
      text: procChanceText ? `${text}${inlineSeparator(t)}${procChanceText}` : text
    }
  }
  if (kindRaw === "consume_mp" || kindToken === "consumemp") {
    const formulaRaw = row.amountFormula && typeof row.amountFormula === "object" && !Array.isArray(row.amountFormula)
      ? row.amountFormula as { baseAdd?: unknown; terms?: unknown[] }
      : { baseAdd: row.value ?? 0, terms: [] }
    const formulaText = localizeAmountFormulaText(formulaRaw.baseAdd, formulaRaw.terms, t)
    return {
      kind: "consume_mp",
      formulaText
    }
  }
  if (kindRaw === "adjust_skill_cooldown" || kindToken === "adjustskillcooldown") {
    const amount = Math.trunc(toFiniteNumber(row.amount ?? row.turns ?? row.value) ?? 0)
    if (amount === 0) return null
    const targetModeRaw = String(row.targetMode || row.target || "self").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
    const categories = normalizeStringList(row.skillCategories ?? row.skill_categories)
    const categoryText = categories.length > 0
      ? categories.map((id) => localizeSkillCategoryName(id, t)).join(" / ")
      : t("game.itemDetailBattleHookSkillCategoryAny")
    const selectionMode = String(row.selectionMode || row.selection_mode || "all").trim().toLowerCase()
    const selectionText = t(`game.itemDetailBattleHookCooldownSelection.${selectionMode}`)
    const maxSkills = Math.max(0, Math.trunc(toFiniteNumber(row.maxSkills ?? row.max_skills) ?? 0))
    return {
      kind: "text",
      text: t("game.itemDetailBattleHookEffectAdjustSkillCooldown", {
        target: localizeBattleHookEffectTargetMode(targetMode, t, options),
        categories: categoryText,
        selection: selectionText,
        count: maxSkills > 0 ? String(maxSkills) : t("game.itemDetailBattleHookCooldownCountAll"),
        change: amount > 0 ? `+${amount}` : String(amount)
      })
    }
  }
  if (kindRaw === "transfer_skill_cooldown" || kindToken === "transferskillcooldown") {
    const amount = Math.max(1, Math.trunc(toFiniteNumber(row.amount ?? row.turns ?? row.value) ?? 1))
    const from = row.from && typeof row.from === "object" && !Array.isArray(row.from) ? row.from as Record<string, unknown> : {}
    const to = row.to && typeof row.to === "object" && !Array.isArray(row.to) ? row.to as Record<string, unknown> : {}
    const categories = Array.from(new Set([
      ...normalizeStringList(from.skillCategories ?? from.skill_categories),
      ...normalizeStringList(to.skillCategories ?? to.skill_categories)
    ]))
    const categoryText = categories.length > 0
      ? categories.map((id) => localizeSkillCategoryName(id, t)).join(" / ")
      : t("game.itemDetailBattleHookSkillCategoryAny")
    const waiverBuffId = String(row.waiveIncreaseBuffId || row.waive_increase_buff_id || "").trim()
    return {
      kind: "text",
      text: t("game.itemDetailBattleHookEffectTransferSkillCooldown", {
        categories: categoryText,
        amount,
        waiver: waiverBuffId
          ? t("game.itemDetailBattleHookEffectTransferSkillCooldownWaiver", { buff: localizeBuffName(waiverBuffId, t) })
          : ""
      })
    }
  }
  if (kindRaw === "extra_turn" || kindToken === "extraturn" || kindRaw === "immediate_action" || kindToken === "immediateaction") {
    const text = t("game.itemDetailBattleHookEffectExtraTurn")
    const procChanceText = localizeBattleHookProcChance(row, t)
    return {
      kind: "text",
      text: procChanceText ? `${text}${inlineSeparator(t)}${procChanceText}` : text
    }
  }
  if (kindRaw === "basic_attack_marked_target" || kindToken === "basicattackmarkedtarget") {
    return {
      kind: "basic_attack_marked_target",
      effectKind: "marked_target",
      procChanceText: localizeBattleHookProcChance(row, t)
    }
  }
  if (kindRaw === "mirror_cast_skill" || kindToken === "mirrorcastskill") {
    const targetingMode = String(row.targetingMode || row.targeting_mode || "reselect").trim().toLowerCase()
    const mpCostMultiplier = toFiniteNumber(row.mpCostMultiplier ?? row.mp_cost_multiplier)
    const mpReduction = mpCostMultiplier !== null ? Math.max(0, 1 - mpCostMultiplier) : 0
    const applyCooldown = row.applyCooldown !== false && row.apply_cooldown !== false && row.setCooldown !== false && row.set_cooldown !== false
    const skillIds = normalizeStringList(row.skillIds ?? row.skill_ids)
    const skillCategories = normalizeStringList(row.skillCategories ?? row.skill_categories ?? row.skillTypes ?? row.skill_types)
    const filterParts: string[] = []
    if (skillIds.length > 0) {
      filterParts.push(t("game.itemDetailBattleHookMirrorSkillFilterIds", {
        skills: skillIds.map((id) => localizeSkillName(id, t)).join(" / ")
      }))
    }
    if (skillCategories.length > 0) {
      filterParts.push(t("game.itemDetailBattleHookMirrorSkillFilterCategories", {
        categories: skillCategories.map((id) => localizeSkillCategoryName(id, t)).join(" / ")
      }))
    }
    const targetNote = targetingMode === "reselect"
      ? null
      : t("game.itemDetailBattleHookMirrorTargetingNote", {
        targeting: localizeMirrorTargetingMode(targetingMode, t)
      })
    const amountRatioBase = toFiniteNumber(row.amountRatioBase ?? row.amount_ratio_base ?? row.amountRatio ?? row.amount_ratio)
    const amountRatioPerEnhance = Math.max(0, toFiniteNumber(row.amountRatioPerEnhanceLevel ?? row.amount_ratio_per_enhance_level) ?? 0)
    const amountRatio = amountRatioBase === null
      ? null
      : amountRatioBase + amountRatioPerEnhance * Math.max(0, Math.trunc(options.equipmentEnhanceLevel ?? 0))
    const projection = specializationValueFormulaProjection(row, amountRatio ?? 1, t, options)
    return {
      kind: "mirror_cast_skill",
      descriptionText: amountRatio !== null
        ? appendFormulaProjection(t("game.specialization.dsl.effect.mirrorCastSkill", {
          value: formatUnsignedPercent(projection.value),
          cooldown: t(applyCooldown ? "game.itemDetailBattleHookMirrorCooldownApplied" : "game.itemDetailBattleHookMirrorCooldownSkipped")
        }), projection)
        : t("game.itemDetailBattleHookEffectMirrorCastSkill", {
          mpReduction: formatUnsignedPercent(mpReduction),
          cooldownText: t(applyCooldown ? "game.itemDetailBattleHookMirrorCooldownApplied" : "game.itemDetailBattleHookMirrorCooldownSkipped"),
          targetNote: targetNote ? ` ${targetNote}` : ""
        }),
      filterText: filterParts.length > 0 ? filterParts.join(clauseSeparator(t)) : null
    }
  }
  if (kindRaw === "execute") {
    const threshold = Number((row as Record<string, unknown>).hpPercentThreshold ?? 0.1)
    const perEnhance = Number((row as Record<string, unknown>).hpPercentPerEnhanceLevel ?? 0.005)
    return {
      kind: "text",
      text: t("game.itemDetailBattleHookEffectExecute", {
        threshold: formatUnsignedPercent(threshold),
        perEnhance: formatUnsignedPercent(perEnhance)
      })
    }
  }
  if (kindRaw === "runtime_damage_amp" || kindRaw === "add_runtime_damage_amp" || kindRaw === "add_damage_amp" || kindToken === "runtimedamageamp" || kindToken === "addruntimedamageamp" || kindToken === "adddamageamp") {
    const value = Number(row.value)
    if (!Number.isFinite(value)) return null
    const damageTypes = Array.isArray(row.damageTypes)
      ? row.damageTypes.map((entry) => String(entry || "").trim().toLowerCase()).filter(Boolean)
      : []
    const specializationProjection = specializationValueFormulaProjection(row, value, t, options, formatSignedPercent)
    if (specializationProjection.suffix) {
      const projectedText = appendFormulaProjection(t("game.itemDetailBattleHookEffectRuntimeDamageAmp", {
        value: formatSignedPercent(specializationProjection.value),
        damageType: localizeBattleHookDamageTypes(damageTypes, t)
      }), specializationProjection)
      const procChanceText = localizeBattleHookProcChance(row, t)
      return { kind: "text", text: procChanceText ? `${projectedText}${inlineSeparator(t)}${procChanceText}` : projectedText }
    }
    const formulaText = localizeBattleHookPercentFormula(row.valueFormula, t)
    const valueMax = toFiniteNumber(row.valueMax ?? row.value_max)
    if (formulaText) {
      const formulaWithMax = valueMax !== null
        ? t("game.itemDetailBattleHookEffectRuntimeDamageAmpFormulaWithMax", {
          value: formatSignedPercent(value),
          formula: formulaText,
          max: formatSignedPercent(valueMax),
          damageType: localizeBattleHookDamageTypes(damageTypes, t)
        })
        : t("game.itemDetailBattleHookEffectRuntimeDamageAmpFormula", {
          value: formatSignedPercent(value),
          formula: formulaText,
          damageType: localizeBattleHookDamageTypes(damageTypes, t)
        })
      const procChanceText = localizeBattleHookProcChance(row, t)
      return { kind: "text", text: procChanceText ? `${formulaWithMax}${inlineSeparator(t)}${procChanceText}` : formulaWithMax }
    }
    const text = t("game.itemDetailBattleHookEffectRuntimeDamageAmp", {
        value: formatSignedPercent(value),
        damageType: localizeBattleHookDamageTypes(damageTypes, t)
      })
    const procChanceText = localizeBattleHookProcChance(row, t)
    return {
      kind: "text",
      text: procChanceText ? `${text}${inlineSeparator(t)}${procChanceText}` : text
    }
  }
  if (kindToken === "playerclone") {
    const durationTurns = Number(row.durationTurns)
    const durationFormula = row.durationFormula && typeof row.durationFormula === "object" && !Array.isArray(row.durationFormula)
      ? row.durationFormula as { baseAdd?: unknown; terms?: unknown[] }
      : null
    const hasDurationFormula = !!durationFormula && Array.isArray(durationFormula.terms) && durationFormula.terms.length > 0
    const durationText = Number.isFinite(durationTurns) && durationTurns > 0
      ? t("game.itemDetailBattleHookEffectSummonDuration", { turns: String(Math.trunc(durationTurns)) })
      : t("game.itemDetailBattleHookEffectSummonDurationPermanent")
    const durationFormulaText = hasDurationFormula && durationFormula
      ? t("game.itemDetailBattleHookEffectSummonDurationFormula", {
        formula: localizeAmountFormulaText(durationFormula.baseAdd, durationFormula.terms, t)
      })
      : null
    const statBonusEntries: Array<{ key: string; label: string; value: string }> = []
    if (row.statModifiers && typeof row.statModifiers === "object" && !Array.isArray(row.statModifiers)) {
      for (const [attrKey, val] of Object.entries(row.statModifiers as Record<string, unknown>)) {
        const num = Number(val)
        if (!Number.isFinite(num) || num === 0) continue
        const sign = num > 0 ? "+" : ""
        statBonusEntries.push({ key: `mod_${attrKey}`, label: localizeAttr(attrKey, t), value: `${sign}${num}` })
      }
    }
    const formulaEntries = localizeSummonStatBonusEntries(row.statsFormula, t)
    // Build inherit summary text
    let inheritCountText: string | null = null
    const inh = row.inherit && typeof row.inherit === "object" && !Array.isArray(row.inherit)
      ? row.inherit as Record<string, unknown>
      : null
    if (inh) {
      const parts: string[] = []
      if (inh.basicAttack !== false) parts.push(t("game.playerCloneInheritBasicAttack"))
      if (inh.skills !== false) parts.push(t("game.playerCloneInheritSkills"))
      if (inh.effectMutations !== false) parts.push(t("game.playerCloneInheritEffectMutations"))
      const statsVal = inh.stats
      if (Array.isArray(statsVal) && statsVal.length > 0) {
        const attrLabels = (statsVal as unknown[]).map((k) => localizeAttr(String(k), t)).join(listSeparator(t))
        parts.push(`${t("game.playerCloneInheritStats")}(${attrLabels})`)
      } else if (statsVal === "all" || statsVal == null) {
        parts.push(t("game.playerCloneInheritStatsAll"))
      }
      if (parts.length > 0) {
        const prefix = t("game.playerCloneInheritPrefix")
        const label = prefix === "game.playerCloneInheritPrefix" ? "继承" : prefix
        const detail = parts.join(" · ")
        const localized = t("game.itemDetailBattleHookLabeledText", { label, text: detail })
        inheritCountText = localized === "game.itemDetailBattleHookLabeledText" ? `${label}：${detail}` : localized
      }
    }
    return {
      kind: "summon",
      targetText: t("game.itemDetailBattleHookTargetMode.self"),
      unitId: "player_echo",
      durationText: durationFormulaText ? `${durationText}${inlineSeparator(t)}${durationFormulaText}` : durationText,
      countText: inheritCountText,
      procChanceText: localizeBattleHookProcChance(row, t),
      statBonusEntries: [...statBonusEntries, ...formulaEntries]
    }
  }
  if (kindRaw === "summon" || kindToken === "summon") {
    const payload = row.summonPayload && typeof row.summonPayload === "object" && !Array.isArray(row.summonPayload)
      ? row.summonPayload as Record<string, unknown>
      : row
    const unitId = String(payload.unitId || payload.enemyId || "").trim()
    if (!unitId) return null
    const targetModeRaw = String(row.targetMode || row.target || "").trim().toLowerCase()
    const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
    const durationTurns = Number(payload.durationTurns)
    const countFormula = payload.countFormula && typeof payload.countFormula === "object" && !Array.isArray(payload.countFormula)
      ? payload.countFormula as { baseAdd?: unknown; terms?: unknown[] }
      : { baseAdd: 1, terms: [] }
    const durationFormula = payload.durationFormula && typeof payload.durationFormula === "object" && !Array.isArray(payload.durationFormula)
      ? payload.durationFormula as { baseAdd?: unknown; terms?: unknown[] }
      : null
    const hasDurationFormula = !!durationFormula && Array.isArray(durationFormula.terms) && durationFormula.terms.length > 0
    const durationProgressionText = localizeEquipmentEnhanceSummonProgression(durationFormula, "duration", t)
    const fixedDurationText = Number.isFinite(durationTurns) && durationTurns > 0
      ? t("game.itemDetailBattleHookEffectSummonDuration", { turns: String(Math.trunc(durationTurns)) })
      : t("game.itemDetailBattleHookEffectSummonDurationPermanent")
    const countFormulaText = localizeAmountFormulaText(countFormula.baseAdd, countFormula.terms, t)
    const countParts: string[] = []
    if (!isDefaultSummonCount(countFormula)) {
      countParts.push(localizeEquipmentEnhanceSummonProgression(countFormula, "count", t)
        || t("game.itemDetailBattleHookEffectSummonCount", { count: countFormulaText }))
    }
    const maxCountFormula = payload.maxCountFormula && typeof payload.maxCountFormula === "object" && !Array.isArray(payload.maxCountFormula)
      ? payload.maxCountFormula as { baseAdd?: unknown; terms?: unknown[] }
      : null
    if (maxCountFormula) {
      if (!sameSimpleFormula(maxCountFormula, countFormula)) {
        countParts.push(localizeEquipmentEnhanceSummonProgression(maxCountFormula, "limit", t)
          || t("game.itemDetailBattleHookEffectSummonLimitFormula", {
            count: localizeAmountFormulaText(maxCountFormula.baseAdd, maxCountFormula.terms, t)
          }))
      }
    } else {
      const fixedMaxCount = Number(payload.maxCountPerUnit)
      if (Number.isFinite(fixedMaxCount) && fixedMaxCount > 0) {
        countParts.push(t("game.itemDetailBattleHookEffectSummonLimit", { count: String(Math.trunc(fixedMaxCount)) }))
      }
    }
    const countText = countParts.length > 0 ? countParts.join(t("game.itemDetailBattleHookEffectSummonPartSeparator")) : null
    const durationFormulaText = !durationProgressionText && hasDurationFormula && durationFormula
      ? t("game.itemDetailBattleHookEffectSummonDurationFormula", {
        formula: localizeAmountFormulaText(durationFormula.baseAdd, durationFormula.terms, t)
      })
      : null
    const durationText = durationProgressionText
      || (durationFormulaText ? `${fixedDurationText}${t("game.itemDetailBattleHookEffectSummonPartSeparator")}${durationFormulaText}` : fixedDurationText)
    return {
      kind: "summon",
      targetText: localizeBattleHookEffectTargetMode(targetMode, t, options),
      unitId,
      durationText,
      countText,
      procChanceText: localizeBattleHookProcChance(row, t),
      statBonusEntries: localizeSummonStatBonusEntries(payload.statsFormula, t)
    }
  }
  if (kindRaw === "extra_enemy_refresh" || kindToken === "extraenemyrefresh") {
    const enemyPool = normalizeDslToken(row.enemyPool)
    const settlementMode = normalizeDslToken(row.settlementMode)
    if (enemyPool !== "current_wave_boss" || settlementMode !== "normal") {
      return { kind: "text", text: t("game.itemDetailBattleHookEffectUnknown") }
    }
    const countFormula = row.countFormula && typeof row.countFormula === "object" && !Array.isArray(row.countFormula)
      ? row.countFormula as { baseAdd?: unknown; terms?: unknown[] }
      : { baseAdd: 1, terms: [] }
    const countText = localizeEquipmentEnhanceSummonProgression(countFormula, "count", t)
      || t("game.itemDetailBattleHookEffectSummonCount", {
        count: localizeAmountFormulaText(countFormula.baseAdd, countFormula.terms, t)
      })
    const maxCount = Math.max(1, Math.trunc(toFiniteNumber(row.maxCount) ?? 1))
    return {
      kind: "text",
      text: t("game.itemDetailBattleHookEffectExtraBossRefresh", { count: countText, maxCount })
    }
  }
  const specializationText = localizeSpecializationBattleHookEffect(row, kindRaw, t, options)
  if (specializationText) return { kind: "text", text: specializationText }
  return { kind: "text", text: t("game.itemDetailBattleHookEffectUnknown") }
}

function isDefaultSummonCount(formula: { baseAdd?: unknown; terms?: unknown[] }): boolean {
  const base = Number(formula.baseAdd)
  const terms = Array.isArray(formula.terms) ? formula.terms : []
  return (!Number.isFinite(base) || base === 1) && terms.length === 0
}

type SimpleSummonFormula = { baseAdd?: unknown; terms?: unknown[] } | null

function sameSimpleFormula(left: SimpleSummonFormula, right: SimpleSummonFormula): boolean {
  if (!left || !right) return false
  const normalize = (formula: Exclude<SimpleSummonFormula, null>) => ({
    baseAdd: Number(formula.baseAdd || 0),
    terms: (Array.isArray(formula.terms) ? formula.terms : []).map((term) => {
      const row = term && typeof term === "object" && !Array.isArray(term) ? term as Record<string, unknown> : {}
      return {
        attrKey: String(row.attrKey || row.attributeKey || "").trim(),
        coefficient: Number(row.coefficient || 0),
        perSkillLevel: Number(row.perSkillLevel || 0)
      }
    })
  })
  return JSON.stringify(normalize(left)) === JSON.stringify(normalize(right))
}

function equipmentEnhanceMilestones(formula: SimpleSummonFormula): { base: number; changes: Array<{ level: number; value: number }> } | null {
  if (!formula || !Array.isArray(formula.terms) || formula.terms.length !== 1) return null
  const termRaw = formula.terms[0]
  if (!termRaw || typeof termRaw !== "object" || Array.isArray(termRaw)) return null
  const term = termRaw as Record<string, unknown>
  if (String(term.attrKey || term.attributeKey || "").trim() !== "equipment_enhance_level") return null
  const coefficient = Number(term.coefficient)
  const perSkillLevel = Number(term.perSkillLevel || 0)
  const baseRaw = Number(formula.baseAdd || 0)
  if (!Number.isFinite(baseRaw) || !Number.isFinite(coefficient) || coefficient <= 0 || perSkillLevel !== 0) return null

  const base = Math.max(0, Math.floor(baseRaw))
  const changes: Array<{ level: number; value: number }> = []
  let previous = base
  for (let level = 1; level <= 50; level += 1) {
    const value = Math.max(0, Math.floor(baseRaw + coefficient * level))
    if (value === previous) continue
    changes.push({ level, value })
    previous = value
    if (changes.length > 2) return null
  }
  return changes.length > 0 ? { base, changes } : null
}

function localizeEquipmentEnhanceSummonProgression(
  formula: SimpleSummonFormula,
  kind: "count" | "limit" | "duration",
  t: TranslateFn
): string | null {
  const progression = equipmentEnhanceMilestones(formula)
  if (!progression) return null
  const milestones = progression.changes.map(({ level, value }) => t(
    kind === "duration"
      ? "game.itemDetailBattleHookEffectSummonDurationMilestone"
      : "game.itemDetailBattleHookEffectSummonCountMilestone",
    { level, value }
  )).join(t("game.itemDetailBattleHookEffectSummonMilestoneSeparator"))
  return t(
    kind === "duration"
      ? "game.itemDetailBattleHookEffectSummonDurationProgression"
      : kind === "limit"
        ? "game.itemDetailBattleHookEffectSummonLimitProgression"
        : "game.itemDetailBattleHookEffectSummonCountProgression",
    { base: progression.base, milestones }
  )
}

function localizeSummonStatBonusEntries(raw: unknown, t: TranslateFn): Array<{ key: string; label: string; value: string }> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
  const out: Array<{ key: string; label: string; value: string }> = []
  for (const [attrKey, formulaRaw] of Object.entries(raw as Record<string, unknown>)) {
    if (!formulaRaw || typeof formulaRaw !== "object" || Array.isArray(formulaRaw)) continue
    const formula = formulaRaw as { baseAdd?: unknown; terms?: unknown[] }
    const text = localizeAmountFormulaText(formula.baseAdd, formula.terms, t)
    if (!text || text === "0") continue
    out.push({ key: attrKey, label: localizeAttr(attrKey, t), value: `+${text}` })
  }
  return out
}

function localizeBattleHookProcChance(row: Record<string, unknown>, t: TranslateFn): string | null {
  const base = toFiniteNumber(row.procChanceBase ?? row.proc_chance_base ?? row.procChance ?? row.proc_chance)
  const min = toFiniteNumber(row.procChanceMin ?? row.proc_chance_min)
  const max = toFiniteNumber(row.procChanceMax ?? row.proc_chance_max)
  const formulaRaw = row.procChanceFormula ?? row.proc_chance_formula
  const formula = formulaRaw && typeof formulaRaw === "object" && !Array.isArray(formulaRaw)
    ? formulaRaw as { baseAdd?: unknown; terms?: unknown[] }
    : null
  const hasFormula = !!formula
  const policyText = localizeBattleHookProbabilityPolicy(row, t)
  if (base === null && min === null && max === null && !hasFormula) return policyText

  const parts: string[] = []
  const pushSigned = (positive: boolean, text: string) => {
    if (!text) return
    if (parts.length === 0) {
      parts.push(positive ? text : `-${text}`)
      return
    }
    parts.push(`${positive ? "+" : "-"} ${text}`)
  }

  if (base !== null && base !== 0) pushSigned(base >= 0, formatUnsignedPercent(Math.abs(base)))
  if (formula) {
    const baseAdd = toFiniteNumber(formula.baseAdd)
    if (baseAdd !== null && baseAdd !== 0) pushSigned(baseAdd >= 0, formatUnsignedPercent(Math.abs(baseAdd)))
    const terms = Array.isArray(formula.terms) ? formula.terms : []
    for (const term of terms) {
      if (!term || typeof term !== "object" || Array.isArray(term)) continue
      const termRow = term as Record<string, unknown>
      const attrKey = String(termRow.attrKey || termRow.attributeKey || "").trim()
      const coefficient = toFiniteNumber(termRow.coefficient)
      if (!attrKey || coefficient === null || coefficient === 0) continue
      pushSigned(coefficient >= 0, `${localizeAttr(attrKey, t)}×${formatUnsignedPercent(Math.abs(coefficient))}`)
    }
  }
  const expr = parts.length > 0 ? parts.join(" ") : "0%"
  const hasCustomMax = max !== null && Math.abs(max - 1) > 0.000001
  const chanceText = hasCustomMax
    ? t("game.itemDetailBattleHookEffectProcChanceWithMax", {
      chance: expr,
      max: formatUnsignedPercent(max as number)
    })
    : expr
  const chanceLabel = t("game.itemDetailBattleHookEffectProcChance", { chance: chanceText })
  return policyText ? `${chanceLabel}${clauseSeparator(t)}${policyText}` : chanceLabel
}

function localizeBattleHookProbabilityPolicy(row: Record<string, unknown>, t: TranslateFn): string | null {
  const parts: string[] = []
  const group = String(row.probabilityGroup ?? row.probability_group ?? "").trim()
  const maxSuccesses = Math.max(0, Math.trunc(toFiniteNumber(row.maxSuccessesPerTurn ?? row.max_successes_per_turn) ?? 0))
  if (group && maxSuccesses > 0) {
    parts.push(t("game.itemDetailBattleHookProbabilitySuccessCap", { count: maxSuccesses }))
  }
  if (row.ignoreProbabilityHooks === true || row.ignore_probability_hooks === true) {
    parts.push(t("game.itemDetailBattleHookIgnoreProbabilityHooks"))
  }
  return parts.length > 0 ? parts.join(clauseSeparator(t)) : null
}

function appendProbabilityFilter(text: string, row: Record<string, unknown>, t: TranslateFn): string {
  const filters = localizeBattleHookEventFilter(row, t)
  if (filters.length === 0) return text
  const filterText = filters.join(clauseSeparator(t))
  const localized = t("game.itemDetailBattleHookTextWithFilters", { text, filters: filterText })
  return localized === "game.itemDetailBattleHookTextWithFilters" ? `${text}（${filterText}）` : localized
}

function localizeProbabilityKind(kind: string, t: TranslateFn): string {
  const normalized = String(kind || "").trim().toLowerCase()
  const key = `game.itemDetailBattleHookProbabilityKind.${normalized}`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookProbabilityKind.other") : localized
}

function localizeProbabilitySourceNames(sourceIDs: string[], t: TranslateFn): string {
  const names = sourceIDs.map((sourceID) => localizeProbabilitySourceName(sourceID, t))
  return Array.from(new Set(names)).join(" / ")
}

function localizeProbabilitySourceName(sourceID: string, t: TranslateFn): string {
  const normalized = String(sourceID || "").trim()
  if (!normalized) return t("game.itemDetailBattleHookProbabilitySourceFallback")
  const segments = normalized.split("_").filter(Boolean)
  const candidates = [normalized]
  for (let length = segments.length - 1; length >= 2; length -= 1) {
    candidates.push(segments.slice(0, length).join("_"))
  }
  for (const candidate of candidates) {
    for (const domain of ["item", "skill", "buff", "enemy"]) {
      const key = `${domain}.${candidate}.name`
      const localized = t(key)
      if (localized !== key && localized.trim() !== "") return localized
    }
  }
  return t("game.itemDetailBattleHookProbabilitySourceFallback")
}

function toFiniteNumber(value: unknown): number | null {
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function normalizeStringList(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return raw.map((entry) => String(entry || "").trim()).filter(Boolean)
}

function normalizeDslToken(raw: unknown): string {
  return String(raw || "")
    .trim()
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/[\s-]+/g, "_")
    .toLowerCase()
}

function isCompletedTurnCycleCondition(condition: unknown): boolean {
  if (!condition || typeof condition !== "object" || Array.isArray(condition)) return false
  const row = condition as Record<string, unknown>
  return normalizeDslToken(row.kind || row.type).replace(/[^a-z]/g, "") === "selfcompletedturncountmodeq"
}

function localizeCompletedTurnCycleCondition(condition: unknown, eventFilter: unknown, t: TranslateFn): string | null {
  if (!condition || typeof condition !== "object" || Array.isArray(condition)) return null
  const row = condition as Record<string, unknown>
  const divisor = Math.trunc(toFiniteNumber(row.divisor) ?? 0)
  const remainder = Math.trunc(toFiniteNumber(row.remainder) ?? 0)
  if (divisor <= 0 || remainder < 0 || remainder >= divisor) return null

  const base = remainder === 0
    ? t("game.itemDetailBattleHookConditionCompletedActionCycle", { count: divisor })
    : t("game.itemDetailBattleHookConditionCompletedActionRemainder", { count: divisor, remainder })
  const filter = eventFilter && typeof eventFilter === "object" && !Array.isArray(eventFilter)
    ? eventFilter as Record<string, unknown>
    : {}
  const kinds = new Set(normalizeStringList(filter.turnKinds ?? filter.turn_kinds))
  const notes: string[] = []
  if (kinds.size === 0 || kinds.has("extra")) {
    notes.push(t("game.itemDetailBattleHookCompletedActionNote.includeExtra"))
  } else if (kinds.has("ordered")) {
    notes.push(t("game.itemDetailBattleHookCompletedActionNote.orderedOnly"))
  }
  if (filter.includeSuppressedTurns === false || filter.include_suppressed_turns === false) {
    notes.push(t("game.itemDetailBattleHookCompletedActionNote.excludeSuppressed"))
  }
  return notes.length > 0
    ? t("game.itemDetailBattleHookConditionWithNotes", {
      condition: base,
      notes: notes.join(t("game.itemDetailBattleHookTurnScopeNoteSeparator"))
    })
    : base
}

function localizeBattleHookCondition(condition: unknown, t: TranslateFn): string | null {
  if (!condition || typeof condition !== "object" || Array.isArray(condition)) return null
  const row = condition as Record<string, unknown>
  const typeRaw = normalizeDslToken(row.kind || row.type)
  const typeToken = typeRaw.replace(/[^a-z]/g, "")
  const value = Number(row.value)
  const damageTypes = Array.isArray(row.damageTypes)
    ? row.damageTypes.map((entry) => String(entry || "").trim().toLowerCase()).filter(Boolean)
    : []
  if (damageTypes.length > 0) {
    return t("game.itemDetailBattleHookConditionDamageType", { damageType: localizeBattleHookDamageTypes(damageTypes, t) })
  }
  if (typeToken === "targethppercentgte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionTargetHpPercentGte", { value: formatUnsignedPercent(value / 100) })
  }
  if (typeToken === "targethppercentlte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionTargetHpPercentLte", { value: formatUnsignedPercent(value / 100) })
  }
  if (typeToken === "sourcehppercentgte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionSourceHpPercentGte", { value: formatUnsignedPercent(value / 100) })
  }
  if (typeToken === "sourcehppercentlte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionSourceHpPercentLte", { value: formatUnsignedPercent(value / 100) })
  }
  if (typeToken === "incomingdamagegte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionIncomingDamageGte", { value: String(Math.trunc(value)) })
  }
  if (typeToken === "incomingdamagelte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionIncomingDamageLte", { value: String(Math.trunc(value)) })
  }
  if (typeToken === "incominghealgte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionIncomingHealGte", { value: String(Math.trunc(value)) })
  }
  if (typeToken === "incomingheallte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionIncomingHealLte", { value: String(Math.trunc(value)) })
  }
  if (typeToken === "hasbuff" || typeToken === "sourcehasbuff" || typeToken === "targethasbuff") {
    const buffID = String(row.buffId || row.value || "").trim()
    if (!buffID) return null
    const buffName = localizeBuffName(buffID, t)
    if (typeToken === "hasbuff") return t("game.itemDetailBattleHookConditionHasBuff", { buff: buffName })
    if (typeToken === "targethasbuff") return t("game.itemDetailBattleHookConditionTargetHasBuff", { buff: buffName })
    return t("game.itemDetailBattleHookConditionSourceHasBuff", { buff: buffName })
  }
  if (typeToken === "selfweaponclass" || typeToken === "sourceweaponclass") {
    const weaponClasses = normalizeStringList(row.weaponClasses || row.weaponClass || row.value)
    if (weaponClasses.length === 0) return null
    const weaponClassText = weaponClasses.map((weaponClass) => {
      const key = `weapon_class.${weaponClass}.name`
      const localized = t(key)
      return localized === key ? weaponClass : localized
    }).join(" / ")
    const key = typeToken === "sourceweaponclass"
      ? "game.itemDetailBattleHookConditionSourceWeaponClass"
      : "game.itemDetailBattleHookConditionSelfWeaponClass"
    return t(key, { weaponClass: weaponClassText })
  }
  if (typeToken === "selfposition" || typeToken === "allysidehaspositions") {
    const positions = normalizeStringList(row.positions || row.position || row.value)
    if (positions.length === 0) return null
    const positionText = localizeTargetPositions(positions, t)
    const key = typeToken === "allysidehaspositions"
      ? "game.itemDetailBattleHookConditionAllySideHasPositions"
      : "game.itemDetailBattleHookConditionSelfPosition"
    return t(key, { positions: positionText })
  }
  if (typeToken === "selfbuffcountgte" || typeToken === "selfbuffcountgt" || typeToken === "selfbuffcountlte" || typeToken === "selfbuffcountlt" || typeToken === "selfbuffcounteq") {
    const buffID = String(row.buffId || "").trim()
    if (!buffID) return null
    const buffName = localizeBuffName(buffID, t)
    const op = typeToken === "selfbuffcountgte" ? "≥" : typeToken === "selfbuffcountgt" ? ">" : typeToken === "selfbuffcountlte" ? "≤" : typeToken === "selfbuffcountlt" ? "<" : "="
    return t("game.itemDetailBattleHookConditionSelfBuffCount", { buff: buffName, op, value: String(Math.trunc(Number.isFinite(value) ? value : 1)) })
  }
  if (typeToken === "targetbuffcountgte") {
    const buffID = String(row.buffId || "").trim()
    if (!buffID || !Number.isFinite(value)) return null
    return t("game.itemDetailBattleHookConditionTargetBuffCountGte", {
      buff: localizeBuffName(buffID, t),
      value: String(Math.max(1, Math.trunc(value)))
    })
  }
  if (typeToken === "targetdistinctsourcebuffcountgte") {
    const buffKind = String(row.buffKind || "debuff").trim().toLowerCase()
    if (!Number.isFinite(value)) return null
    return t("game.itemDetailBattleHookConditionTargetDistinctSourceBuffCountGte", {
      kind: t(`game.itemDetailBattleHookBuffKind.${buffKind}`),
      value: String(Math.max(1, Math.trunc(value)))
    })
  }
  if (typeToken === "selfmpgte" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionSelfMpGte", { value: String(Math.trunc(value)) })
  }
  if (typeToken === "selfmplt" && Number.isFinite(value)) {
    return t("game.itemDetailBattleHookConditionSelfMpLt", { value: String(Math.trunc(value)) })
  }
  if (typeToken === "selfskillcooldowncountgte" && Number.isFinite(value)) {
	const categories = normalizeStringList(row.skillCategories ?? row.skill_categories)
	const categoryText = categories.length > 0
	  ? categories.map((id) => localizeSkillCategoryName(id, t)).join(" / ")
	  : t("game.itemDetailBattleHookSkillCategoryAny")
	return t("game.itemDetailBattleHookConditionSelfSkillCooldownCountGte", {
	  value: String(Math.max(1, Math.trunc(value))),
	  categories: categoryText
	})
  }
  if (typeToken === "equipmentenhancelevelgte" && Number.isFinite(value)) {
	return t("game.itemDetailBattleHookConditionEquipmentEnhanceLevelGte", { value: String(Math.max(0, Math.trunc(value))) })
  }
  if (typeToken === "playerpartyequippedunitcounteq" && Number.isFinite(value)) {
    const itemID = String(row.itemId ?? row.item_id ?? "").trim()
    if (!itemID) return null
    const itemKey = `item.${itemID}.name`
    const localizedItem = t(itemKey)
    return t("game.itemDetailBattleHookConditionPlayerPartyEquippedUnitCountEq", {
      count: String(Math.max(0, Math.trunc(value))),
      item: localizedItem === itemKey ? itemID : localizedItem
    })
  }
  if (typeToken === "waveintervaleq") {
    const divisor = Number(row.divisor)
    if (Number.isFinite(divisor) && divisor > 0) {
      const key = "game.itemDetailBattleHookConditionWaveIntervalEq"
      const localized = t(key, { divisor: String(Math.trunc(divisor)) })
      return localized === key ? `每 ${Math.trunc(divisor)} 波次` : localized
    }
  }
  const specializationText = localizeSpecializationBattleHookCondition(row, typeRaw, t)
  if (specializationText) return specializationText
  if (typeRaw) return t("game.itemDetailBattleHookConditionUnknown")
  return null
}

function localizeSpecializationBattleHookCondition(row: Record<string, unknown>, kind: string, t: TranslateFn): string | null {
  const value = toFiniteNumber(row.value)
  if (kind === "context_damage_type") {
    return t("game.specialization.dsl.condition.contextDamageType", {
      damageType: localizeBattleHookDamageTypes([normalizeDslToken(row.value) || "physical"], t)
    })
  }
  if (kind === "context_hit_count_gte") {
    return t("game.specialization.dsl.condition.contextHitCountGte", { value: Math.max(1, Math.trunc(value ?? 1)) })
  }
  if (kind === "context_mp_insufficient") return t("game.specialization.dsl.condition.contextMpInsufficient")
  if (kind === "context_skill_non_basic") return t("game.specialization.dsl.condition.contextSkillNonBasic")
  if (kind === "counter_gte") {
    return t("game.specialization.dsl.condition.counterGte", {
      counter: localizeSpecializationMechanicToken(row.counterId, t),
      value: Math.max(1, Math.trunc(value ?? 1))
    })
  }
  if (kind === "incoming_damage_would_be_lethal") return t("game.specialization.dsl.condition.incomingLethal")
  if (kind === "self_hp_ratio_lte") {
    return t("game.specialization.dsl.condition.selfHpRatioLte", { value: formatUnsignedPercent(value ?? 0) })
  }
  if (kind === "self_mp_ratio_gte") {
    return t("game.specialization.dsl.condition.selfMpRatioGte", { value: formatUnsignedPercent(value ?? 0) })
  }
  if (kind === "source_is_enemy") return t("game.specialization.dsl.condition.sourceIsEnemy")
  if (kind === "target_has_any_buff") {
    const buffIds = normalizeStringList(row.buffIds ?? row.buff_ids)
    return t("game.specialization.dsl.condition.targetHasAnyBuff", {
      buffs: buffIds.map((buffId) => getLocalizedBuffName(t, buffId, buffId)).join(listSeparator(t))
    })
  }
  if (kind === "target_has_buff") {
    const buffId = String(row.buffId ?? row.buff_id ?? "").trim()
    return t("game.specialization.dsl.condition.targetHasBuff", {
      buff: getLocalizedBuffName(t, buffId, buffId || "-")
    })
  }
  if (kind === "target_has_any_specialization_mark") return t("game.specialization.dsl.condition.targetHasAnyMark")
  if (kind === "target_has_mark") {
    return t("game.specialization.dsl.condition.targetHasMark", {
      mark: localizeSpecializationMechanicToken(row.markId, t)
    })
  }
  if (kind === "target_is_lowest_hp_ally") return t("game.specialization.dsl.condition.targetLowestHpAlly")
  if (kind === "adjacent_ally_exists") return t("game.specialization.dsl.condition.adjacentAlly")
  if (kind === "trigger_limit") {
    const parts: string[] = []
    const perTurn = Math.max(0, Math.trunc(toFiniteNumber(row.perTurn ?? row.per_turn) ?? 0))
    const perWave = Math.max(0, Math.trunc(toFiniteNumber(row.perWave ?? row.per_wave) ?? 0))
    const perBattle = Math.max(0, Math.trunc(toFiniteNumber(row.perBattle ?? row.per_battle) ?? 0))
    if (perTurn > 0) parts.push(t("game.specialization.dsl.condition.limitPerTurn", { value: perTurn }))
    if (perWave > 0) parts.push(t("game.specialization.dsl.condition.limitPerWave", { value: perWave }))
    if (perBattle > 0) parts.push(t("game.specialization.dsl.condition.limitPerBattle", { value: perBattle }))
    return parts.join(" / ") || null
  }
  return null
}

function localizeSpecializationBattleHookEffect(row: Record<string, unknown>, kind: string, t: TranslateFn, options: BattleHookDisplayOptions): string | null {
  const value = toFiniteNumber(row.value) ?? 0
  const counter = localizeSpecializationMechanicToken(row.counterId, t)
  const mark = localizeSpecializationMechanicToken(row.markId, t)
  if (kind === "cycle_counter") {
    const hasAtMax = row.atMax && typeof row.atMax === "object" && !Array.isArray(row.atMax)
    const atMax = hasAtMax
      ? localizeSpecializationCycleOutcome(row.atMax as Record<string, unknown>, t, options)
      : ""
    return t(hasAtMax ? "game.specialization.dsl.effect.cycleCounter" : "game.specialization.dsl.effect.cycleCounterStack", {
      counter,
      gain: Math.max(1, Math.trunc(toFiniteNumber(row.gain) ?? 1)),
      max: Math.max(1, Math.trunc(toFiniteNumber(row.maxStacks ?? row.max_stacks) ?? 1)),
      outcome: atMax
    })
  }
  if (kind === "consume_counter_for_damage_amp") {
    const projection = specializationValueFormulaProjection(row, value, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.consumeCounterForDamageAmp", {
      counter,
      consume: Math.max(1, Math.trunc(toFiniteNumber(row.consume) ?? 1)),
      value: formatUnsignedPercent(projection.value),
      penetration: formatUnsignedPercent(toFiniteNumber(row.percentDefensePenetration ?? row.percent_defense_penetration) ?? 0)
    }), projection)
  }
  if (kind === "action_meter_gain") {
    const projection = specializationValueFormulaProjection(row, value, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.actionMeterGain", { value: formatUnsignedPercent(projection.value) }), projection)
  }
  if (kind === "restore_mp") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.maxMpRatio ?? row.max_mp_ratio) ?? value, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.restoreMp", {
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "resource_debt_cast") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.hpCostRatio ?? row.hp_cost_ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.resourceDebtCast", {
      hp: formatUnsignedPercent(projection.value),
      cooldown: Math.trunc(toFiniteNumber(row.cooldownDelta ?? row.cooldown_delta) ?? 0)
    }), projection)
  }
  if (kind === "mark_cycle") {
    const markedHit = row.markedHit && typeof row.markedHit === "object" && !Array.isArray(row.markedHit)
      ? row.markedHit as Record<string, unknown>
      : {}
    const projection = specializationValueFormulaProjection(markedHit, toFiniteNumber(markedHit.value) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.markCycle", {
      mark,
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "buff_cycle") {
    const buffId = String(row.buffId ?? row.buff_id ?? "").trim()
    const buff = getLocalizedBuffName(t, buffId, buffId || "-")
    const buffedHit = row.buffedHit && typeof row.buffedHit === "object" && !Array.isArray(row.buffedHit)
      ? row.buffedHit as Record<string, unknown>
      : {}
    const projection = specializationValueFormulaProjection(buffedHit, toFiniteNumber(buffedHit.value) ?? 0, t, options)
    const key = buffedHit.consumeBuff === true || buffedHit.consume_buff === true
      ? "game.specialization.dsl.effect.buffCycleConsume"
      : "game.specialization.dsl.effect.buffCycleKeep"
    return appendFormulaProjection(t(key, {
      buff,
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "mark_target") {
    return t("game.specialization.dsl.effect.markTarget", {
      target: localizeSpecializationSelection(row.selectionMode ?? row.targetMode, t),
      mark,
      turns: formatOptionalTurns(row.durationTurns ?? row.duration_turns, t)
    })
  }
  if (kind === "designate_priority_target") {
    return t("game.specialization.dsl.effect.designatePriorityTarget", {
      target: localizeSpecializationSelection(row.selectionMode, t),
      mark
    })
  }
  if (kind === "consume_mark") return t("game.specialization.dsl.effect.consumeMark", { mark })
  if (kind === "redirect_damage") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.redirectDamage", {
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "prevent_lethal") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.remainingHpRatio ?? row.remaining_hp_ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.preventLethal", {
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "flat_defense_penetration") {
    const projection = specializationValueFormulaProjection(row, 0, t, options, (value) => formatEffectNumber(value, 2))
    return appendFormulaProjection(t("game.specialization.dsl.effect.flatDefensePenetration", {
      formula: formatEffectNumber(projection.value, 2)
    }), projection)
  }
  if (kind === "retaliate") {
    return t("game.specialization.dsl.effect.retaliate", {
      strength: formatUnsignedPercent(toFiniteNumber(row.strengthRatio ?? row.strength_ratio) ?? 0),
      stamina: formatUnsignedPercent(toFiniteNumber(row.staminaRatio ?? row.stamina_ratio) ?? 0)
    })
  }
  if (kind === "repeat_context_effect") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.amountRatio ?? row.amount_ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.repeatContextEffect", {
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "shift_position") {
    return t("game.specialization.dsl.effect.shiftPosition", {
      policy: localizeSpecializationSelection(row.policy, t),
      steps: Math.max(1, Math.trunc(toFiniteNumber(row.maxSteps ?? row.max_steps) ?? 1))
    })
  }
  if (kind === "shared_damage_reduction") {
    const projection = specializationValueFormulaProjection(row, value, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.sharedDamageReduction", {
      value: formatUnsignedPercent(projection.value),
      turns: Math.max(1, Math.trunc(toFiniteNumber(row.durationTurns ?? row.duration_turns) ?? 1))
    }), projection)
  }
  if (kind === "resource_shield") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.absorbRatio ?? row.absorb_ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.resourceShield", {
      resource: String(row.resource || "mp").toUpperCase(),
      value: formatUnsignedPercent(projection.value),
      cost: String(toFiniteNumber(row.resourcePerDamage ?? row.resource_per_damage) ?? 0)
    }), projection)
  }
  return null
}

function localizeSpecializationCycleOutcome(row: Record<string, unknown>, t: TranslateFn, options: BattleHookDisplayOptions): string {
  const kind = normalizeDslToken(row.kind || row.type)
  if (kind === "empower_next_hit") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.damageAmp ?? row.damage_amp) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.cycleEmpowerHit", {
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "action_meter_gain") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.value) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.actionMeterGain", { value: formatUnsignedPercent(projection.value) }), projection)
  }
  if (kind === "restore_mp_and_tick_cooldown") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.mpRatio ?? row.mp_ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.cycleRestoreMpCooldown", {
      mp: formatUnsignedPercent(projection.value),
      cooldown: Math.abs(Math.trunc(toFiniteNumber(row.cooldownAmount ?? row.cooldown_amount) ?? 0))
    }), projection)
  }
  if (kind === "restore_mp") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.mpRatio ?? row.mp_ratio) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.restoreMp", {
      value: formatUnsignedPercent(projection.value)
    }), projection)
  }
  if (kind === "guard_lowest_hp_ally") {
    const projection = specializationValueFormulaProjection(row, toFiniteNumber(row.damageReduction ?? row.damage_reduction) ?? 0, t, options)
    return appendFormulaProjection(t("game.specialization.dsl.effect.cycleGuardAlly", {
      value: formatUnsignedPercent(projection.value),
      turns: Math.max(1, Math.trunc(toFiniteNumber(row.durationTurns ?? row.duration_turns) ?? 1))
    }), projection)
  }
  return kind
}

function localizeSpecializationMechanicToken(raw: unknown, t: TranslateFn): string {
  const token = normalizeDslToken(raw)
  if (!token) return "-"
  const key = `game.specialization.dsl.token.${token}`
  const localized = t(key)
  return localized === key ? token : localized
}

function localizeSpecializationSelection(raw: unknown, t: TranslateFn): string {
  const token = normalizeDslToken(raw) || "original"
  const key = `game.specialization.dsl.selection.${token}`
  const localized = t(key)
  return localized === key ? token : localized
}

type SpecializationFormulaProjection = {
  value: number
  suffix: string
}

function specializationValueFormulaProjection(
  row: Record<string, unknown>,
  fallbackValue: number,
  t: TranslateFn,
  options: BattleHookDisplayOptions,
  formatProjectedValue: (value: number) => string = formatUnsignedPercent
): SpecializationFormulaProjection {
  if (options.sourceKind !== "specialization") return { value: fallbackValue, suffix: "" }
  const rawFormula = row.valueFormula ?? row.value_formula
  if (!rawFormula || typeof rawFormula !== "object" || Array.isArray(rawFormula)) return { value: fallbackValue, suffix: "" }
  const formula = rawFormula as { baseAdd?: number; terms?: Array<{ attributeKey?: string; attrKey?: string; coefficient?: number }> }
  const formulaText = localizeAmountFormulaText(formula.baseAdd, formula.terms, t)
  const projected = options.unitAttributes !== undefined
    ? evaluateAmountFormulaPreview(formula, options.unitAttributes, 1, { missingAttributeValue: 0, includeBaseOnly: true })
    : null
  const value = projected ?? fallbackValue
  const key = projected === null
    ? "game.specialization.dsl.effect.formula"
    : "game.specialization.dsl.effect.formulaProjection"
  const suffix = t(key, {
    formula: formulaText || "0",
    value: formatProjectedValue(value)
  })
  return { value, suffix }
}

function appendFormulaProjection(text: string, projection: SpecializationFormulaProjection): string {
  return projection.suffix ? `${text} ${projection.suffix}` : text
}

function formatOptionalTurns(raw: unknown, t: TranslateFn): string {
  const turns = Math.max(0, Math.trunc(toFiniteNumber(raw) ?? 0))
  return turns > 0 ? t("game.specialization.dsl.effect.duration", { turns }) : t("game.specialization.dsl.effect.durationCurrentBattle")
}

function formatSignedPercent(value: number): string {
  const percent = value * 100
  const abs = Math.abs(percent)
  const text = Number.isInteger(abs) ? String(abs) : String(Number(abs.toFixed(2)))
  const sign = percent > 0 ? "+" : percent < 0 ? "-" : ""
  return `${sign}${text}%`
}

function resolveBattleHookPreviewPayload(effect: Record<string, unknown>, equipmentEnhanceLevel: number): Record<string, unknown> | undefined {
  const payload = effect.payload && typeof effect.payload === "object" && !Array.isArray(effect.payload)
    ? { ...(effect.payload as Record<string, unknown>) }
    : {}
  const formulaSource = effect.payloadFormula && typeof effect.payloadFormula === "object" && !Array.isArray(effect.payloadFormula)
    ? effect.payloadFormula as Record<string, unknown>
    : effect.payloadFormulas && typeof effect.payloadFormulas === "object" && !Array.isArray(effect.payloadFormulas)
      ? effect.payloadFormulas as Record<string, unknown>
      : {}
  for (const [key, rawFormula] of Object.entries(formulaSource)) {
    const normalizedKey = String(key || "").trim()
    if (!normalizedKey) continue
    const value = evaluateBattleHookPreviewFormula(rawFormula, equipmentEnhanceLevel)
    if (value !== null) payload[normalizedKey] = roundPreviewValue(value)
  }
  return Object.keys(payload).length > 0 ? payload : undefined
}

function resolveBattleHookPayloadFormula(effect: Record<string, unknown>): Record<string, unknown> | undefined {
  const formulaSource = effect.payloadFormula && typeof effect.payloadFormula === "object" && !Array.isArray(effect.payloadFormula)
    ? effect.payloadFormula as Record<string, unknown>
    : effect.payloadFormulas && typeof effect.payloadFormulas === "object" && !Array.isArray(effect.payloadFormulas)
      ? effect.payloadFormulas as Record<string, unknown>
      : {}
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(formulaSource)) {
    const normalizedKey = String(key || "").trim()
    if (!normalizedKey || !value || typeof value !== "object" || Array.isArray(value)) continue
    out[normalizedKey] = value
  }
  return Object.keys(out).length > 0 ? out : undefined
}

function evaluateBattleHookPreviewFormula(rawFormula: unknown, equipmentEnhanceLevel: number): number | null {
  if (!rawFormula || typeof rawFormula !== "object" || Array.isArray(rawFormula)) return null
  const formula = rawFormula as { baseAdd?: unknown; terms?: unknown[] }
  let total = toFiniteNumber(formula.baseAdd) ?? 0
  const terms = Array.isArray(formula.terms) ? formula.terms : []
  for (const term of terms) {
    if (!term || typeof term !== "object" || Array.isArray(term)) continue
    const row = term as Record<string, unknown>
    const attrKey = String(row.attrKey || "").trim()
    const coefficient = toFiniteNumber(row.coefficient)
    if (!attrKey || coefficient === null) continue
    const attrValue = attrKey === "equipment_enhance_level" ? Math.max(0, Math.trunc(equipmentEnhanceLevel)) : 0
    total += attrValue * coefficient
  }
  return total
}

function roundPreviewValue(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.round(value * 10000) / 10000
}

function localizeBattleHookPercentFormula(raw: unknown, t: TranslateFn): string | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const row = raw as { baseAdd?: unknown; terms?: unknown[] }
  const parts: string[] = []
  const baseAdd = toFiniteNumber(row.baseAdd)
  if (baseAdd !== null && baseAdd !== 0) parts.push(formatSignedPercent(baseAdd))
  const terms = Array.isArray(row.terms) ? row.terms : []
  for (const term of terms) {
    if (!term || typeof term !== "object" || Array.isArray(term)) continue
    const termRow = term as Record<string, unknown>
    const attrKey = String(termRow.attrKey || termRow.attributeKey || "").trim()
    const coefficient = toFiniteNumber(termRow.coefficient)
    if (!attrKey || coefficient === null || coefficient === 0) continue
    const sign = coefficient > 0 ? "+" : "-"
    parts.push(`${sign}${localizeEffectAttr(attrKey, t)}×${formatUnsignedPercent(Math.abs(coefficient))}`)
  }
  if (parts.length === 0) return null
  return parts.join("")
}

function formatUnsignedPercent(value: number): string {
  const percent = value * 100
  const abs = Math.abs(percent)
  const text = Number.isInteger(abs) ? String(abs) : String(Number(abs.toFixed(2)))
  return `${text}%`
}

function localizeBattleHookDamageTypes(types: string[], t: TranslateFn): string {
  if (!types.length) return t("game.itemDetailBattleHookDamageTypeAny")
  const localized = types.map((type) => {
    const key = `game.damageType.${type}`
    const text = t(key)
    return text === key ? t("game.itemDetailBattleHookDamageTypeOther") : text
  })
  return localized.join(" / ")
}

function localizeBuffName(buffID: string, t: TranslateFn): string {
  const key = `buff.${buffID}.name`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookBuffFallback") : localized
}

function localizeSkillName(skillID: string, t: TranslateFn): string {
  const key = `skill.${skillID}.name`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookSkillFallback") : localized
}

function localizeEnemyName(enemyID: string, t: TranslateFn): string {
  const key = `enemy.${enemyID}.name`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookSummonFallback") : localized
}

function localizeSkillCategoryName(categoryID: string, t: TranslateFn): string {
  const key = `skill_type.${categoryID}.name`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookSkillCategoryAny") : localized
}

function localizeMirrorTargetingMode(mode: string, t: TranslateFn): string {
  const key = `game.itemDetailBattleHookMirrorTargeting.${mode || "reselect"}`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookMirrorTargeting.reselect") : localized
}

function localizeBattleHookTargetMode(mode: string, t: TranslateFn): string {
  const key = `game.itemDetailBattleHookTargetMode.${mode}`
  const localized = t(key)
  return localized === key ? t("game.itemDetailBattleHookTargetMode.self") : localized
}

function localizeBattleHookEffectTargetMode(
  mode: string,
  t: TranslateFn,
  options: BattleHookDisplayOptions
): string {
  if (options.sourceKind !== "specialization") return localizeBattleHookTargetMode(mode, t)
  const normalized = String(mode || "self").trim().toLowerCase()
  const selectionMode = normalized === "target" ? "targeting" : normalized
  if (["self", "source", "enemy", "marked_enemy", "targeting", "lowest_hp_ally"].includes(selectionMode)) {
    return localizeSpecializationSelection(selectionMode, t)
  }
  return localizeBattleHookTargetMode(normalized, t)
}

function localizeBattleHookTargetingText(input: {
  scopeText: string
  positionText: string
  seekText: string
  selectText: string
  countText: string
  targetCountBase: number
}, t: TranslateFn): string {
  const selectNormalized = String(input.selectText || "").trim()
  const seekNormalized = String(input.seekText || "").trim()
  const allTargets = Number(input.targetCountBase) >= 999 || String(input.countText || "").includes("全部") || String(input.countText || "").toLowerCase().includes("all")
  const firstZh = selectNormalized === "首位"
  const firstEn = selectNormalized.toLowerCase() === "first"
  const noSeekZh = seekNormalized === "无次要寻敌"
  const noSeekEn = seekNormalized.toLowerCase() === "no secondary seek"
  if (allTargets && (firstZh || firstEn) && (noSeekZh || noSeekEn)) {
    const allScopes = new Set(["全部", "all"])
    const key = allScopes.has(String(input.scopeText || "").trim().toLowerCase())
      ? "game.itemDetailBattleHookTargetingAllUnitsInPositions"
      : "game.itemDetailBattleHookTargetingAllInPositions"
    const localized = t(key, { positions: input.positionText, scope: input.scopeText })
    if (localized !== key) return localized
  }
  const fallbackKey = "game.itemDetailBattleHookTargetingRule"
  const fallback = t(fallbackKey, {
    scope: input.scopeText,
    positions: input.positionText,
    seek: input.seekText,
    select: input.selectText,
    count: input.countText
  })
  return fallback === fallbackKey
    ? `${input.scopeText} / ${input.positionText} / ${input.seekText} / ${input.selectText} / ${input.countText}`
    : fallback
}

function localizeTargetDirection(direction: string, t: TranslateFn): string {
  const key = `game.targetDirection.${direction}`
  const localized = t(key)
  return localized === key ? direction : localized
}

function localizeTargetPositions(positions: string[], t: TranslateFn): string {
  if (!Array.isArray(positions) || positions.length === 0) return "-"
  return positions.map((position) => localizeTargetDirection(position, t)).join(" / ")
}

function localizeTargetSeekStrategy(strategy: string, t: TranslateFn): string {
  const key = `game.targetSeekStrategy.${strategy}`
  const localized = t(key)
  return localized === key ? strategy : localized
}

function localizeTargetScope(scope: string, t: TranslateFn): string {
  const key = `game.skillTargetScope.${scope}`
  const localized = t(key)
  return localized === key ? scope : localized
}

function localizeAttr(attrKey: string, t: TranslateFn): string {
  if (attrKey === "equipment_enhance_level") {
    const key = "game.itemDetailBattleHookAttrEquipmentEnhanceLevel"
    const localized = t(key)
    return localized === key ? attrKey : localized
  }
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function localizeSelectRule(rule: string, attrKey: string, t: TranslateFn): string {
  const key = `game.skillSelectRule.${rule}`
  const localized = t(key)
  const ruleLabel = localized === key ? rule : localized
  if ((rule === "attr_max" || rule === "attr_min") && attrKey) {
    return `${ruleLabel}: ${localizeAttr(attrKey, t)}`
  }
  return ruleLabel
}
