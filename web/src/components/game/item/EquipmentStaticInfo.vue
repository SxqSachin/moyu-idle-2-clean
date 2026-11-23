<template>
  <template v-if="equipmentDefinition">
    <div v-if="statusRequireEntries.length" class="item-popover-row detail-surface">
      <div class="item-popover-label detail-section-title">{{ t("game.statusRequirements") }}</div>
      <div class="item-popover-value detail-row-value status-require-list">
        <StatusRequireTag
          v-for="entry in statusRequireEntries"
          :key="`equip-static-require-${equipmentDefinition.id}-${entry.attrKey}`"
          :attr-key="entry.attrKey"
          :required-value="entry.required"
          :current-value="requirementCurrentValue(entry.attrKey)"
        />
      </div>
    </div>

    <div v-if="baseAttrTags.length" class="item-popover-row detail-surface">
      <div class="item-popover-label detail-section-title">{{ t("game.itemDetailEquipmentEffect") }}</div>
      <div class="item-popover-value detail-row-value">
        <div v-if="baseAttrTags.length" class="status-require-list">
          <StatusTag
            v-for="entry in baseAttrTags"
            :key="`equip-static-attr-${equipmentDefinition.id}-${entry.attrKey}`"
            :attr-key="entry.attrKey"
            :value="entry.value"
            signed
          />
        </div>
      </div>
    </div>

    <div v-if="battleHookDescriptions.length || hasHiddenHooks" class="item-popover-row detail-surface">
      <div class="item-popover-label detail-section-title" style="display:flex;align-items:center;justify-content:space-between;">
        <span>{{ t("game.itemDetailBattleHooks") }}</span>
        <button v-if="hasHiddenHooks" class="text-xs opacity-60 hover:opacity-100 cursor-pointer bg-transparent border-0 p-0 ml-2" style="font-size:0.75rem;color:inherit;" @click="showAllHooks = !showAllHooks">{{ showAllHooks ? t("game.itemDetailBattleHooksShowLess") : t("game.itemDetailBattleHooksShowAll") }}</button>
      </div>
      <div class="item-popover-value detail-row-value">
        <div class="battle-hook-list">
          <div
            v-for="entry in battleHookDescriptions"
            :key="`equip-static-hook-${equipmentDefinition.id}-${entry.key}`"
            class="battle-hook-node"
          >
            <div class="battle-hook-node-title">
              {{ t("game.itemDetailBattleHookNode", { index: entry.index, trigger: entry.triggerText }) }}
            </div>
            <div v-if="entry.conditions.length > 0" class="battle-hook-node-conditions">
              <div class="battle-hook-line battle-hook-condition-label">{{ t("game.itemDetailBattleHookIf") }}</div>
              <div
                v-for="condition in entry.conditions"
                :key="`equip-static-hook-${equipmentDefinition.id}-${entry.key}-${condition.key}`"
                class="battle-hook-line battle-hook-content-line"
              >
                <span class="battle-hook-bullet">·</span>
                <span class="battle-hook-inline-text">{{ condition.text }}</span>
              </div>
            </div>
            <div class="battle-hook-node-effects">
              <div v-if="entry.conditions.length > 0" class="battle-hook-line battle-hook-effect-label">{{ t("game.itemDetailBattleHookThen") }}</div>
              <div
                v-for="effect in entry.effects"
                :key="`equip-static-hook-${equipmentDefinition.id}-${entry.key}-${effect.key}`"
                class="battle-hook-line battle-hook-content-line"
              >
                <span class="battle-hook-bullet">·</span>
                <template v-if="effect.kind === 'add_buff' || effect.kind === 'remove_buff'">
                  <span>{{ t(effect.kind === 'add_buff' ? "game.itemDetailBattleHookEffectAddBuffPrefix" : "game.itemDetailBattleHookEffectRemoveBuffPrefix", { target: effect.targetText }) }}</span>
                  <BuffQuickRef
                    :buff-id="effect.buffId"
                    :payload-formula="effect.kind === 'add_buff' ? effect.buffPayloadFormula : undefined"
                  />
                  <span v-if="effect.kind === 'add_buff' && effect.turns !== null">{{ t("game.itemDetailBattleHookEffectTurnsSuffix", { turns: effect.turns }) }}</span>
                  <span v-if="effect.kind === 'add_buff' && effect.procChanceText">{{ effect.procChanceText }}</span>
                  <span v-if="effect.kind === 'remove_buff' && effect.removeStack">{{ t("game.itemDetailBattleHookEffectRemoveBuffOneStack") }}</span>
                </template>
                <template v-else-if="effect.kind === 'heal'">
                  <span>{{ t("game.itemDetailBattleHookEffectHeal", { target: effect.targetText, formula: effect.formulaText }) }}</span>
                  <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
                </template>
                <template v-else-if="effect.kind === 'summon'">
                  <span>{{ t("game.itemDetailBattleHookEffectSummonPrefix", { target: effect.targetText }) }}</span>
                  <EnemyUnitQuickRef :enemy-id="effect.unitId" />
                  <span>{{ effect.durationText }}</span>
                  <span v-if="effect.countText">{{ effect.countText }}</span>
                  <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
                </template>
                <template v-else-if="effect.kind === 'consume_mp'">
                  <span>{{ t("game.itemDetailBattleHookEffectConsumeMp", { formula: effect.formulaText }) }}</span>
                </template>
                <template v-else-if="effect.kind === 'mirror_cast_skill'">
                  <span>{{ effect.descriptionText }}</span>
                  <div v-if="effect.filterText" class="battle-hook-subline">{{ effect.filterText }}</div>
                </template>
                <template v-else-if="effect.kind === 'basic_attack_marked_target'">
                  <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetPrefix") }}</span>
                  <BuffEffectQuickRef :effect-kind="effect.effectKind" />
                  <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetSuffix") }}</span>
                  <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
                </template>
                <template v-else>
                  <span class="battle-hook-inline-text">{{ effect.text }}</span>
                </template>
                <div v-if="effect.kind === 'summon' && effect.statBonusEntries.length" class="battle-hook-summon-bonuses">
                  <div class="ml-[-1em]">
                    {{ t("game.itemDetailBattleHookEffectSummonStatBonus") }}
                  </div>
                  <div
                    v-for="bonus in effect.statBonusEntries"
                    :key="`equip-static-hook-${equipmentDefinition.id}-${entry.key}-${effect.key}-${bonus.key}`"
                    class="battle-hook-summon-bonus-line"
                  >
                    <span class="battle-hook-summon-bonus-label">{{ bonus.label }}:</span>
                    <span>{{ bonus.value }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="attrConversionLines.length" class="item-popover-row detail-surface">
      <div class="item-popover-label detail-section-title">{{ t("game.itemDetailAttrConversion") }}</div>
      <div class="item-popover-value detail-row-value">
        <div v-for="(line, idx) in attrConversionLines" :key="idx" class="battle-hook-line battle-hook-content-line">
          {{ line.text }}
        </div>
      </div>
    </div>

    <BasicAttackDetailBlocks
      :groups="basicAttackEffectGroups"
      :details="basicAttackDetails"
      compact
      :max-groups="1"
      :max-rows-per-group="10"
    />

  </template>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { storeToRefs } from "pinia"
import { useI18n } from "vue-i18n"
import type { BasicAttackTargetingDefinition, EquipmentItemDefinition } from "@/types/equipment"
import { toStatusRequireEntries } from "@services/status-require"
import StatusRequireTag from "@components/game/attribute/StatusRequireTag.vue"
import StatusTag from "@components/game/attribute/StatusTag.vue"
import BasicAttackDetailBlocks from "@components/game/common/BasicAttackDetailBlocks.vue"
import BuffEffectQuickRef from "@components/game/buff/BuffEffectQuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import { usePlayerStore } from "@stores/player"
import { buildBattleHookDescriptions } from "@services/battle-hook-display"
import { localizeBasicAttackFormulaText, localizeTargetCountMaxText, localizeTargetCountText } from "@services/effect-formula-display"

const props = defineProps<{
  equipmentDefinition?: EquipmentItemDefinition
  equippedEffectAttrs?: Record<string, number>
  excludedAttrKeys?: string[]
  requirementAttributes?: Record<string, number> | null
  enhanceLevel?: number
}>()

const { t } = useI18n()
const playerStore = usePlayerStore()
const { attributes, stats } = storeToRefs(playerStore)

const statusRequireEntries = computed(() => {
  if (!props.equipmentDefinition) return []
  return toStatusRequireEntries(props.equipmentDefinition.statusRequirement)
})

function requirementCurrentValue(attrKey: string): number | undefined {
  if (!props.requirementAttributes) return undefined
  const value = Number(props.requirementAttributes[attrKey] || 0)
  return Number.isFinite(value) ? value : 0
}

const baseAttrTags = computed(() => {
  const out: Array<{ attrKey: string; value: number }> = []
  const excluded = new Set(props.excludedAttrKeys || [])
  const sourceAttrs =
    props.equippedEffectAttrs && Object.keys(props.equippedEffectAttrs).length > 0
      ? props.equippedEffectAttrs
      : props.equipmentDefinition?.baseAttrs || {}
  for (const [attrKey, value] of Object.entries(sourceAttrs)) {
    if (excluded.has(attrKey)) continue
    const numeric = Number(value)
    if (!Number.isFinite(numeric) || numeric === 0) continue
    out.push({ attrKey, value: numeric })
  }
  return out
})

const isWeapon = computed(() => {
  return !!props.equipmentDefinition
})

const basicAttackEffectGroups = computed(() => {
  const eq = props.equipmentDefinition

  if (!eq || !isWeapon.value) return []
  const effects = Array.isArray(eq.basicAttackEffects) ? eq.basicAttackEffects : []
  return effects.map((effect, index) => {
    const details: Array<{ key: string; label: string; value: string; buffIds?: string[]; buffPayloadFormula?: Record<string, unknown>; labelHint?: string }> = [
      {
        key: "kind",
        label: t("game.skillEffectKindLabel"),
        value: localizeEffectKind(effect.kind)
      },
      {
        key: "target-direction",
        label: t("game.itemDetailBasicAttackTargetDirection"),
        value: localizeTargetPositions(effect.targeting.targetPositions)
      },
      {
        key: "seek-strategy",
        label: t("game.itemDetailBasicAttackSeekStrategy"),
        value: localizeTargetSeekStrategy(effect.targeting.secondarySeek)
      },
      {
        key: "target-mode",
        label: t("game.skillTargetMode"),
        value: localizeTargetScope(effect.targeting.targetScope)
      },
      {
        key: "select-rule",
        label: t("game.skillSelectRuleLabel"),
        value: localizeSelectRule(effect.targeting.selectRule, effect.targeting.selectAttrKey)
      },
      {
        key: "target-count",
        label: t("game.skillTargetBaseCount"),
        value: localizeTargetCountText(effect.targeting.targetCount, t)
      }
    ]
    const targetCountMax = localizeTargetCountMaxText(effect.targeting.targetCount, t)
    if (targetCountMax !== null) {
      details.push({
        key: "target-count-max",
        label: t("game.skillTargetMaxCount"),
        value: targetCountMax,
        labelHint: t("game.skillTargetMaxCountHint")
      })
    }
    const targetPredicates = Array.isArray((effect.targeting as BasicAttackTargetingDefinition).targetPredicates)
      ? (effect.targeting as BasicAttackTargetingDefinition).targetPredicates || []
      : []
    if (targetPredicates.some((p) => p.kind === "target_hp_not_full")) {
      const labelKey = "game.targetPredicateHpNotFull"
      const localized = t(labelKey)
      details.push({
        key: "target-predicate-hp-not-full",
        label: t("game.skillTargetConditions"),
        value: localized === labelKey ? "生命值不满" : localized
      })
    }
    if (effect.kind === "damage" || effect.kind === "heal") {
      const terms = Array.isArray(effect.amountFormula?.terms) ? effect.amountFormula.terms : []
      const damageSpecFallback = effect.damageSpec && Object.keys(effect.damageSpec).length > 0
        ? effect.damageSpec
        : eq.basicAttackFormula?.damageSpec
      details.push(
        {
          key: "formula",
          label: t("game.itemDetailBasicAttackFormula"),
          value: localizeAmountFormula(effect.amountFormula?.baseAdd || 0, terms)
        },
        {
          key: "damage-type",
          label: t("game.itemDetailBasicAttackDamageType"),
          value: localizeDamageType(effect.damageType || "physical")
        }
      )
      details.push(...buildDamageSpecDetails(damageSpecFallback))
    }
    if (effect.kind === "heal") {
      const mpCost = (effect as { mpCost?: { baseAdd?: number; terms?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> } }).mpCost
      if (mpCost) {
        const mpCostBase = Number(mpCost.baseAdd || 0)
        const mpCostTerms = Array.isArray(mpCost.terms) ? mpCost.terms : []
        if (mpCostBase !== 0 || mpCostTerms.length > 0) {
          const labelKey = "game.itemDetailBasicAttackMpCost"
          const localized = t(labelKey)
          const hintKey = "game.itemDetailBasicAttackMpCostHint"
          const hint = t(hintKey)
          details.push({
            key: "mp-cost",
            label: localized === labelKey ? "魔法消耗" : localized,
            value: localizeAmountFormula(mpCostBase, mpCostTerms),
            labelHint: hint === hintKey ? "魔法不足时此效果被跳过，触发后续备选效果（如有）" : hint
          })
        }
      }
    }
    if (effect.kind === "status") {
      const statusIds = Array.isArray(effect.statusPayload?.statusIds) ? effect.statusPayload.statusIds : []
      if (statusIds.length > 0) {
        details.push({
          key: "status-ids",
          label: t("game.skillEffectStatusIds"),
          value: "-",
          buffIds: statusIds,
          buffPayloadFormula: effect.statusPayload?.payloadFormula
        })
      }
      details.push({
        key: "status-duration",
        label: t("game.skillEffectStatusDuration"),
        value: describeStatusDuration(effect.statusPayload)
      })
    }
    const onlyWhenRefs = Array.isArray(effect.onlyWhenNoTargetFromEffects) ? effect.onlyWhenNoTargetFromEffects : undefined
    const mpCostGatedRefs = onlyWhenRefs
      ? onlyWhenRefs.filter((ref) => {
          const refEffect = effects[ref - 1] as { mpCost?: { baseAdd?: number; terms?: unknown[] } } | undefined
          if (!refEffect?.mpCost) return false
          const base = Number(refEffect.mpCost.baseAdd || 0)
          const terms = Array.isArray(refEffect.mpCost.terms) ? refEffect.mpCost.terms : []
          return base !== 0 || terms.length > 0
        })
      : undefined
    return {
      key: `effect-${index + 1}`,
      title: `${t("game.skillEffectTitle")} ${index + 1}`,
      details,
      linkage: {
        onlyWhenNoTargetFromEffects: onlyWhenRefs,
        reuseTargetsFromEffect: Number.isFinite(Number(effect.reuseTargetsFromEffect)) ? Number(effect.reuseTargetsFromEffect) : undefined,
        mpCostGatedRefs: mpCostGatedRefs && mpCostGatedRefs.length > 0 ? mpCostGatedRefs : undefined
      }
    }
  })
})

const basicAttackDetails = computed(() => {
  const eq = props.equipmentDefinition

  if (!eq || !isWeapon.value || basicAttackEffectGroups.value.length > 0) return []
  const details: Array<{ key: string; label: string; value: string; labelHint?: string }> = []
  if (eq.basicAttackTargetDirection) {
    details.push({
      key: "target-direction",
      label: t("game.itemDetailBasicAttackTargetDirection"),
      value: localizeTargetDirection(eq.basicAttackTargetDirection)
    })
  }
  if (eq.targetSeekStrategy) {
    details.push({
      key: "seek-strategy",
      label: t("game.itemDetailBasicAttackSeekStrategy"),
      value: localizeTargetSeekStrategy(eq.targetSeekStrategy)
    })
  }
  if (eq.basicAttackFormula && Array.isArray(eq.basicAttackFormula.formula) && eq.basicAttackFormula.formula.length > 0) {
    const formulaPreview = calculateFormulaPreview(eq.basicAttackFormula.formula)
    details.push(
      {
        key: "formula",
        label: t("game.itemDetailBasicAttackFormula"),
        value: localizeBasicAttackFormulaText(eq.basicAttackFormula.formula, formulaPreview, t)
      },
      {
        key: "damage-type",
        label: t("game.itemDetailBasicAttackDamageType"),
        value: localizeDamageType(eq.basicAttackFormula.damageType)
      }
    )
  }
  return details
})

type AttrConversionLine = { text: string }
const attrConversionLines = computed((): AttrConversionLine[] => {
  const conversions = props.equipmentDefinition?.attrConversions
  if (!Array.isArray(conversions) || conversions.length === 0) return []
  return conversions.map((c) => {
    const fromName = localizeAttr(c.fromAttr ?? "")
    const toName = localizeAttr(c.toAttr ?? "")
    const cap = Math.round((c.threshold ?? 1) * 100)
    let baseRatio = 0
    const suffixes: string[] = []
    if (c.ratioFormula) {
      baseRatio = Math.round(c.ratioFormula.baseAdd * 100)
      for (const term of c.ratioFormula.terms ?? []) {
        const step = Math.round(term.coefficient * 100)
        if (step === 0) continue
        if (term.attrKey === "equipment_enhance_level") {
          suffixes.push(t("game.itemDetailAttrConversionPerEnhance", { step }))
        } else {
          const attrName = localizeAttr(term.attrKey)
          suffixes.push(t("game.itemDetailAttrConversionPerAttr", { attr: attrName, step }))
        }
      }
    } else if (typeof c.ratio === "number") {
      baseRatio = Math.round(c.ratio * 100)
    }
    const main = t("game.itemDetailAttrConversionLine", { from: fromName, cap, ratio: baseRatio, to: toName })
    return { text: suffixes.length > 0 ? `${main}${suffixes.join("")}` : main }
  })
})

const showAllHooks = ref(false)
const allBattleHookDescriptions = computed(() => {
  const eq = props.equipmentDefinition
  if (!eq || !Array.isArray(eq.battleHooks) || eq.battleHooks.length === 0) return []
  return buildBattleHookDescriptions(eq.battleHooks, t, { equipmentEnhanceLevel: Number(props.enhanceLevel || 0) })
})
const hasHiddenHooks = computed(() => allBattleHookDescriptions.value.some((n) => n.hidden))
const battleHookDescriptions = computed(() =>
  showAllHooks.value ? allBattleHookDescriptions.value : allBattleHookDescriptions.value.filter((n) => !n.hidden)
)

function localizeTargetDirection(direction: string): string {
  const key = `game.targetDirection.${direction}`
  const localized = t(key)
  return localized === key ? direction : localized
}

function localizeTargetPositions(positions: string[]): string {
  if (!Array.isArray(positions) || positions.length === 0) return "-"
  return positions.map((position) => localizeTargetDirection(position)).join(" / ")
}

function localizeTargetSeekStrategy(strategy: string): string {
  const key = `game.targetSeekStrategy.${strategy}`
  const localized = t(key)
  return localized === key ? strategy : localized
}

function localizeTargetScope(scope: string): string {
  const key = `game.skillTargetScope.${scope}`
  const localized = t(key)
  return localized === key ? scope : localized
}

function localizeSelectRule(rule: string, attrKey: string): string {
  const key = `game.skillSelectRule.${rule}`
  const localized = t(key)
  const ruleLabel = localized === key ? rule : localized
  if ((rule === "attr_max" || rule === "attr_min") && attrKey) {
    return `${ruleLabel}: ${localizeAttr(attrKey)}`
  }
  return ruleLabel
}

function localizeDamageType(type: string): string {
  const key = `game.damageType.${type}`
  const localized = t(key)
  return localized === key ? type : localized
}

function localizeEffectKind(kind: string): string {
  const key = `game.skillEffectKind.${kind || "damage"}`
  const localized = t(key)
  return localized === key ? (kind || "damage") : localized
}

function describeStatusDuration(
  payload: { durationBaseTurns?: number; durationPerSkillLevelTurns?: number } | undefined
): string {
  const baseTurns = Math.max(0, Math.trunc(Number(payload?.durationBaseTurns || 0)))
  const perLevel = Number(payload?.durationPerSkillLevelTurns || 0)
  if (!Number.isFinite(perLevel) || perLevel === 0) return String(baseTurns)
  return `${baseTurns} + ${trimTrailingZeros(perLevel)} / Lv`
}

function localizeAmountFormula(baseAdd: number, terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }>): string {
  const formulaText = terms
    .filter((term) => term.attrKey && Number.isFinite(term.coefficient))
    .map((term) => {
      const base = `${localizeAttr(term.attrKey)}×${trimTrailingZeros(term.coefficient)}`
      if (!Number.isFinite(term.perSkillLevel) || !term.perSkillLevel) return base
      return `${base} (+${trimTrailingZeros(term.perSkillLevel)} / Lv)`
    })
    .join(" + ")
  const computedPreview = calculateFormulaPreview(
    terms.map((term) => ({
      attrKey: term.attrKey,
      coefficient: Number(term.coefficient || 0)
    }))
  )
  const withBase = Number.isFinite(baseAdd) && baseAdd !== 0
    ? formulaText
      ? `${trimTrailingZeros(baseAdd)} + ${formulaText}`
      : trimTrailingZeros(baseAdd)
    : formulaText || "-"
  const localized = t("game.basicAttackFormula", { formula: withBase })
  const formulaDisplay = localized === "game.basicAttackFormula" ? withBase : localized
  const previewValue = computedPreview === null ? null : computedPreview + (Number.isFinite(baseAdd) ? baseAdd : 0)
  const previewText = localizeFormulaPreview(previewValue)
  if (previewText === "-") return formulaDisplay
  return `${formulaDisplay} (${previewText})`
}

function buildDamageSpecDetails(
  damageSpec: {
    hitCount?: { base: number; formula: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    flatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    percentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    physicalFlatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    physicalPercentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    magicalFlatDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
    magicalPercentDefensePenetrationFormula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  } | undefined
): Array<{ key: string; label: string; value: string }> {
  if (!damageSpec) return []
  const details: Array<{ key: string; label: string; value: string }> = []
  const hitCountBase = Math.max(1, Math.trunc(Number(damageSpec.hitCount?.base || 1)))
  const hitCountFormula = Array.isArray(damageSpec.hitCount?.formula) ? damageSpec.hitCount?.formula || [] : []
  if (hitCountBase > 1 || hitCountFormula.length > 0) {
    details.push({
      key: "hit-count",
      label: t("game.itemDetailBasicAttackHitCount"),
      value: localizeTargetCountText({ base: hitCountBase, formula: hitCountFormula }, t)
    })
  }
  const specs = [
    ["flatDefensePenetrationFormula", "itemDetailBasicAttackFlatDefensePenetration", false],
    ["percentDefensePenetrationFormula", "itemDetailBasicAttackPercentDefensePenetration", true],
    ["physicalFlatDefensePenetrationFormula", "itemDetailBasicAttackPhysicalFlatDefensePenetration", false],
    ["physicalPercentDefensePenetrationFormula", "itemDetailBasicAttackPhysicalPercentDefensePenetration", true],
    ["magicalFlatDefensePenetrationFormula", "itemDetailBasicAttackMagicalFlatDefensePenetration", false],
    ["magicalPercentDefensePenetrationFormula", "itemDetailBasicAttackMagicalPercentDefensePenetration", true]
  ] as const
  const penetrationDetails = specs.flatMap(([key, labelKey, percent]) => {
    const formula = damageSpec[key]
    if (!formula || (formula.baseAdd === 0 && formula.terms.length === 0)) return []
    return [{
      key,
      label: t(`game.${labelKey}`),
      value: localizePenetrationFormula(formula.baseAdd, formula.terms, percent)
    }]
  })
  return [...details, ...penetrationDetails]
}

function localizePenetrationFormula(
  baseAdd: number,
  terms: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }>,
  percent: boolean
): string {
  const renderedBase = Number.isFinite(baseAdd) && baseAdd !== 0 ? formatPenetrationValue(baseAdd, percent) : ""
  const formulaText = terms
    .filter((term) => term.attrKey && Number.isFinite(term.coefficient))
    .map((term) => {
      const base = `${localizeAttr(term.attrKey)}×${trimTrailingZeros(term.coefficient)}`
      if (!Number.isFinite(term.perSkillLevel) || !term.perSkillLevel) return base
      return `${base} (+${trimTrailingZeros(term.perSkillLevel)} / Lv)`
    })
    .join(" + ")
  const expression = renderedBase && formulaText ? `${renderedBase} + ${formulaText}` : renderedBase || formulaText || "-"
  const computedPreview = calculateFormulaPreview(
    terms.map((term) => ({
      attrKey: term.attrKey,
      coefficient: Number(term.coefficient || 0)
    }))
  )
  if (computedPreview === null) return expression
  return `${expression} (${formatPenetrationValue(computedPreview + (Number.isFinite(baseAdd) ? baseAdd : 0), percent)})`
}

function formatPenetrationValue(value: number, percent: boolean): string {
  if (!Number.isFinite(value)) return "0"
  if (!percent) return trimTrailingZeros(value)
  return `${trimTrailingZeros(value * 100)}%`
}

function localizeAttr(attrKey: string): string {
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function resolveCurrentAttrValue(attrKey: string): number | null {
  const attributeNode = attributes.value?.[attrKey]
  const attributeValue = Number(attributeNode?.value)
  if (Number.isFinite(attributeValue)) return attributeValue
  const statValue = Number(stats.value?.[attrKey])
  if (Number.isFinite(statValue)) return statValue
  return null
}

function calculateFormulaPreview(formula: Array<{ attrKey: string; coefficient: number }>): number | null {
  let total = 0
  let used = 0
  for (const term of formula) {
    const attrKey = String(term.attrKey || "").trim()
    if (!attrKey || !Number.isFinite(term.coefficient)) continue
    const currentValue = resolveCurrentAttrValue(attrKey)
    if (currentValue === null) continue
    total += currentValue * term.coefficient
    used += 1
  }
  if (used === 0) return null
  return total
}

function localizeFormulaPreview(value: number | null): string {
  if (value === null) return "-"
  const previewValue = Math.round(value * 10000) / 10000
  return t("game.itemDetailBasicAttackFormulaInlinePreview", { value: previewValue })
}

function trimTrailingZeros(value: number): string {
  if (!Number.isFinite(value)) return "0"
  const fixed = value.toFixed(6)
  return fixed.replace(/\.?0+$/, "")
}
</script>

<style scoped>
@reference "#app-style";

.item-popover-row {
  display: grid;
  gap: 2px;
}

.item-popover-label {
  font-size: 12px;
  color: var(--app-muted);
}

.battle-hook-list {
  @apply flex flex-col gap-1;
}

.battle-hook-node {
  @apply flex flex-col gap-1;
}

.battle-hook-node-title {
  @apply text-xs font-medium;
}

.battle-hook-node-effects {
  @apply flex flex-col gap-1;
}

.battle-hook-line {
  @apply flex flex-wrap items-center gap-1 text-xs leading-5;
}

.battle-hook-condition-label,
.battle-hook-effect-label {
  font-weight: 500;
  color: var(--app-muted);
}

.battle-hook-content-line {
  @apply pl-3;
}

.battle-hook-summon-bonuses {
  @apply basis-full pl-4 pt-1;
}

.battle-hook-subline {
  flex-basis: 100%;
  padding-left: 1rem;
  color: var(--el-text-color-regular);
}

.battle-hook-summon-bonus-line {
  @apply flex flex-wrap items-center gap-1 text-xs leading-5;
}

.battle-hook-summon-bonus-label {
  font-weight: 500;
  color: var(--app-muted);
}

.battle-hook-bullet {
  color: var(--app-muted);
  flex: 0 0 auto;
}

.battle-hook-inline-text {
  min-width: 0;
  flex: 1 1 12rem;
  white-space: normal;
  overflow-wrap: anywhere;
}

.item-popover-value {
  font-size: 12px;
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
  word-break: break-word;
}

.status-require-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.item-popover-detail-line {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px;
}

.item-popover-detail-label {
  color: var(--el-text-color-secondary);
}
</style>
