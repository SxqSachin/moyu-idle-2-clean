import { getAttributeDefinition } from "@services/attribute-display"
import { buildAffixMutationDisplayLines } from "@services/affix-mutation-display"
import { buildBattleHookDescriptions, type BattleHookDisplayEffect } from "@services/battle-hook-display"
import { getLocalizedBuffName } from "@services/buff-definition"
import type { SpecializationGrant } from "@services/specialization"

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

export type SpecializationGrantDisplayOptions = {
  t: TranslateFn
  localized: (key: string, fallback: string) => string
  skillName: (skillId: string) => string
  slotFamilyLabel: (family: string) => string
  characterAttributes?: Record<string, number>
  includeHookMetadata?: boolean
}

export function buildSpecializationGrantDisplayLines(
  grant: SpecializationGrant,
  options: SpecializationGrantDisplayOptions
): string[] {
  const { t } = options
  if (grant.kind === "stat_modifier") {
    return (grant.modifiers || []).map((modifier) => {
      const definition = getAttributeDefinition(modifier.attributeKey)
      const attr = definition?.nameKey
        ? options.localized(definition.nameKey, modifier.attributeKey)
        : modifier.attributeKey
      const value = modifier.operation === "multiply"
        ? `${modifier.value >= 0 ? "+" : ""}${(modifier.value * 100).toLocaleString()}%`
        : `${modifier.value >= 0 ? "+" : ""}${modifier.value.toLocaleString()}`
      return t("game.specialization.effect.stat", { attr, value })
    })
  }
  if (grant.kind === "equipment_slot_capacity") {
    return Object.entries(grant.deltas || {}).map(([family, delta]) => t("game.specialization.effect.slot", {
      slot: options.slotFamilyLabel(family),
      value: `${delta >= 0 ? "+" : ""}${delta}`
    }))
  }
  if (grant.kind === "battle_hook") {
    const descriptions = buildBattleHookDescriptions(grant.battleHooks || [], t, {
      sourceKind: "specialization",
      unitAttributes: options.characterAttributes || {}
    }).filter((hook) => !hook.hidden)
    const lines = descriptions.flatMap((hook) => {
      const effects = hook.effects.map((effect) => t("game.specialization.effect.hookEffect", {
        effect: specializationHookEffectText(effect, t)
      }))
      if (!options.includeHookMetadata) return effects
      return [
        t("game.specialization.effect.hookTrigger", { trigger: hook.triggerText }),
        ...hook.conditions.map((condition) => t("game.specialization.effect.hookCondition", { condition: condition.text })),
        ...effects
      ]
    })
    return lines.length > 0
      ? Array.from(new Set(lines))
      : [t("game.specialization.effect.hookEffectFallback")]
  }

  const affectedObjectLabel = t("game.itemDetailAffixMutationDisplayAffectedObject")
  const selectorAll = t("game.itemDetailAffixMutationDisplaySelectorAll")
  const lines = buildAffixMutationDisplayLines(grant.effectMutations || [], t, {
    unitAttributes: options.characterAttributes || {}
  }).filter((line) => !(line.label === affectedObjectLabel && line.value === selectorAll))
  const projected = lines.map((line) => {
    const value = line.skillIds?.length
      ? line.skillIds.map(options.skillName).join(" / ")
      : line.value
    const label = line.label === affectedObjectLabel
      ? t("game.specialization.effect.mutationPart")
      : line.label
    return t("game.specialization.effect.mutationLine", { label, value })
  })
  return projected.length > 0
    ? Array.from(new Set(projected))
    : [t("game.specialization.effect.mutations", { count: grant.effectMutations?.length || 0 })]
}

function specializationHookEffectText(effect: BattleHookDisplayEffect, t: TranslateFn): string {
  if (effect.kind === "text") return effect.text
  if (effect.kind === "mirror_cast_skill") return effect.descriptionText
  if (effect.kind === "consume_mp") return effect.formulaText
  if (effect.kind === "heal") {
    return t("game.specialization.effect.hookFormula", { target: effect.targetText, formula: effect.formulaText })
  }
  if (effect.kind === "summon") {
    return t("game.specialization.effect.hookSummon", { target: effect.targetText, unit: effect.unitId })
  }
  if (effect.kind === "remove_buff") {
    return t("game.specialization.effect.hookRemoveBuff", {
      target: effect.targetText,
      buff: getLocalizedBuffName(t, effect.buffId, effect.buffId)
    })
  }
  if (effect.kind === "basic_attack_marked_target") {
    return t("game.specialization.effect.hookMarkedAttack", { kind: effect.effectKind })
  }
  return t("game.specialization.effect.hookAddBuff", {
    target: effect.targetText,
    buff: getLocalizedBuffName(t, effect.buffId, effect.buffId)
  })
}
