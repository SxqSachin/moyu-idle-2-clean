<template>
  <div class="affix-quick-ref">
    <el-popover
      v-model:visible="popoverActive"
      placement="top"
      :width="360"
      :trigger="supportsHover && enablePopover ? 'hover' : 'manual'"
      :persistent="false"
      :show-after="supportsHover && enablePopover ? 200 : 0"
      :disabled="!enablePopover"
      @show="onPopoverShow"
      @hide="onPopoverHide"
    >
      <template #reference>
        <el-tag
          class="affix-quick-ref-tag w-fit"
          type="primary"
          effect="plain"
          :disable-transitions="true"
          :style="tagStyle"
          @click="onTagClick"
          @mouseenter="primePopover"
          @focusin="primePopover"
        >
          <span class="affix-quick-ref-content">
            <!-- <TypeIcon type="affix" :id="normalizedAffixID" :alt="affixName" :size="20" fallback="✨" /> -->
            <span v-if="prefixLabel">{{ prefixLabel }}</span>
            <span>{{ affixName }}</span>
            <!-- <span v-if="affixScore" class="affix-score-mini">{{ affixScore.score }}</span> -->
            <!-- <span v-if="showRarity && rarityID">(<RarityText :rarity-id="rarityID" />)</span> -->
          </span>
        </el-tag>
      </template>
      <div v-if="popoverLoaded && enablePopover" class="affix-quick-ref-popover-stack">
        <AffixTemplatePopoverContent
          v-if="hasTemplatePopoverContent"
          :affix-id="normalizedAffixID"
          :rarity-weights="templateRarityWeights"
        />
        <ul v-if="showPopoverInstanceDetails" class="affix-quick-ref-lines affix-quick-ref-lines--popover">
          <template v-for="section in instanceEffectSections" :key="`affix-instance-pop-section-${section.key}`">
            <li class="affix-effect-section-line" :class="`is-${section.key}`">{{ section.label }}</li>
            <li
              v-for="(line, idx) in section.lines"
              :key="`affix-instance-pop-line-${section.key}-${idx}`"
              class="affix-effect-value-line"
              :class="`is-${line.tone}`"
            >
              <template v-if="line.skillIds?.length">
                <span class="affix-effect-line-label">{{ line.label }}:</span>
                <span class="affix-effect-skill-list">
                  <SkillQuickRef
                    v-for="skillId in line.skillIds"
                    :key="`affix-instance-pop-skill-${section.key}-${idx}-${skillId}`"
                    :skill-id="skillId"
                  />
                </span>
              </template>
              <template v-else>{{ line.text }}</template>
            </li>
          </template>
          <template v-if="instanceLimitLines.length > 0">
            <li class="affix-limit-section-line">{{ t("game.affixLimitSection") }}</li>
            <li
              v-for="line in instanceLimitLines"
              :key="`affix-instance-pop-limit-${line.itemId}`"
              class="affix-limit-line"
            >
              <QuickRef :item-id="line.itemId" />
              <span>{{ t("game.affixLimitCompact", { equipment: line.equipmentText, body: line.bodyText }) }}</span>
            </li>
          </template>
          <li v-if="hasHiddenInstanceHooks" class="affix-hook-trigger-line" style="display:flex;justify-content:flex-end;">
            <button style="font-size:0.7rem;opacity:0.6;background:transparent;border:0;cursor:pointer;padding:0;" @click="showAllInstanceHooks = !showAllInstanceHooks">{{ showAllInstanceHooks ? t("game.itemDetailBattleHooksShowLess") : t("game.itemDetailBattleHooksShowAll") }}</button>
          </li>
          <template v-for="hook in instanceHookDescriptions" :key="`affix-pop-hook-${hook.key}`">
            <li class="affix-hook-trigger-line">{{ hook.triggerText }}</li>
            <li v-if="hook.conditions.length > 0" class="affix-hook-label-line">{{ t("game.itemDetailBattleHookIf") }}</li>
            <li
              v-for="condition in hook.conditions"
              :key="`affix-pop-hook-condition-${hook.key}-${condition.key}`"
              class="affix-hook-content-line"
            >
              <span class="affix-hook-bullet">·</span>{{ condition.text }}
            </li>
            <li v-if="hook.conditions.length > 0" class="affix-hook-label-line">{{ t("game.itemDetailBattleHookThen") }}</li>
            <li v-for="effect in hook.effects" :key="`affix-pop-hook-${hook.key}-${effect.key}`" class="affix-hook-content-line">
              <span class="affix-hook-bullet">·</span>
              <div v-if="effect.kind === 'add_buff' || effect.kind === 'remove_buff'" class="inline-flex flex-row items-center justify-start">
                <span>{{ t(effect.kind === 'add_buff' ? "game.itemDetailBattleHookEffectAddBuffPrefix" : "game.itemDetailBattleHookEffectRemoveBuffPrefix", { target: effect.targetText }) }}</span>
                <span class="m-2">
                  <BuffQuickRef :buff-id="effect.buffId" />
                </span>
                <span v-if="effect.kind === 'add_buff' && effect.turns !== null">{{ t("game.itemDetailBattleHookEffectTurnsSuffix", { turns: effect.turns }) }}</span>
                <span v-if="effect.kind === 'add_buff' && effect.procChanceText">{{ effect.procChanceText }}</span>
                <span v-if="effect.kind === 'remove_buff' && effect.removeStack">{{ t("game.itemDetailBattleHookEffectRemoveBuffOneStack") }}</span>
              </div>
              <div v-else-if="effect.kind === 'heal'" class="inline-flex flex-row items-center justify-start">
                <span>{{ t("game.itemDetailBattleHookEffectHeal", { target: effect.targetText, formula: effect.formulaText }) }}</span>
                <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
              </div>
              <div v-else-if="effect.kind === 'summon'" class="inline-flex flex-row flex-wrap items-center justify-start gap-1">
                <span>{{ t("game.itemDetailBattleHookEffectSummonPrefix", { target: effect.targetText }) }}</span>
                <EnemyUnitQuickRef :enemy-id="effect.unitId" />
                <span>{{ effect.durationText }}</span>
                <span v-if="effect.countText">{{ effect.countText }}</span>
                <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
                <div v-if="effect.statBonusEntries.length" class="basis-full pl-4 pt-1">
                  <div v-for="bonus in effect.statBonusEntries" :key="`affix-pop-hook-${hook.key}-${effect.key}-${bonus.key}`">
                    <span class="font-medium">{{ bonus.label }}:</span>
                    <span>{{ bonus.value }}</span>
                  </div>
                </div>
              </div>
              <div v-else-if="effect.kind === 'mirror_cast_skill'" class="inline-flex flex-col items-start justify-start">
                <span>{{ effect.descriptionText }}</span>
                <span v-if="effect.filterText" class="affix-hook-subline">{{ effect.filterText }}</span>
              </div>
              <div v-else-if="effect.kind === 'basic_attack_marked_target'" class="inline-flex flex-row flex-wrap items-center justify-start gap-1">
                <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetPrefix") }}</span>
                <BuffEffectQuickRef :effect-kind="effect.effectKind" />
                <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetSuffix") }}</span>
                <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
              </div>
              <template v-else>
                {{ formatHookEffectFallback(effect) }}
              </template>
            </li>
          </template>
        </ul>
        <div v-if="sourceBadges.length" class="affix-quick-ref-source-badges">
          <span
            v-for="badge in sourceBadges"
            :key="badge.kind"
            class="affix-quick-ref-source-badge"
            :class="`is-${badge.kind}`"
          >
            <Icon :icon="badge.icon" />
            <span>{{ badge.title }}</span>
          </span>
        </div>
      </div>
    </el-popover>
    <GameDialog v-model="mobilePopoverVisible" class="affix-quick-ref-dialog dialog-mobile-windowed" :width="dialogWidth">
      <div class="affix-quick-ref-popover-stack">
        <AffixTemplatePopoverContent
          v-if="hasTemplatePopoverContent"
          :affix-id="normalizedAffixID"
          :rarity-weights="templateRarityWeights"
        />
        <ul v-if="showPopoverInstanceDetails" class="affix-quick-ref-lines affix-quick-ref-lines--popover">
          <template v-for="section in instanceEffectSections" :key="`affix-instance-dialog-section-${section.key}`">
            <li class="affix-effect-section-line" :class="`is-${section.key}`">{{ section.label }}</li>
            <li
              v-for="(line, idx) in section.lines"
              :key="`affix-instance-dialog-line-${section.key}-${idx}`"
              class="affix-effect-value-line"
              :class="`is-${line.tone}`"
            >
              <template v-if="line.skillIds?.length">
                <span class="affix-effect-line-label">{{ line.label }}:</span>
                <span class="affix-effect-skill-list">
                  <SkillQuickRef
                    v-for="skillId in line.skillIds"
                    :key="`affix-instance-dialog-skill-${section.key}-${idx}-${skillId}`"
                    :skill-id="skillId"
                  />
                </span>
              </template>
              <template v-else>{{ line.text }}</template>
            </li>
          </template>
          <template v-if="instanceLimitLines.length > 0">
            <li class="affix-limit-section-line">{{ t("game.affixLimitSection") }}</li>
            <li
              v-for="line in instanceLimitLines"
              :key="`affix-instance-dialog-limit-${line.itemId}`"
              class="affix-limit-line"
            >
              <QuickRef :item-id="line.itemId" />
              <span>{{ t("game.affixLimitCompact", { equipment: line.equipmentText, body: line.bodyText }) }}</span>
            </li>
          </template>
        </ul>
        <div v-if="sourceBadges.length" class="affix-quick-ref-source-badges">
          <span
            v-for="badge in sourceBadges"
            :key="badge.kind"
            class="affix-quick-ref-source-badge"
            :class="`is-${badge.kind}`"
          >
            <Icon :icon="badge.icon" />
            <span>{{ badge.title }}</span>
          </span>
        </div>
      </div>
    </GameDialog>

    <ul v-if="showInstanceDetails && hasInstanceDetails" class="affix-quick-ref-lines">
      <template v-for="section in instanceEffectSections" :key="`affix-instance-section-${section.key}`">
        <li class="affix-effect-section-line" :class="`is-${section.key}`">{{ section.label }}</li>
        <li
          v-for="(line, idx) in section.lines"
          :key="`affix-instance-line-${section.key}-${idx}`"
          class="affix-effect-value-line"
          :class="`is-${line.tone}`"
        >
          <template v-if="line.skillIds?.length">
            <span class="affix-effect-line-label">{{ line.label }}:</span>
            <span class="affix-effect-skill-list">
              <SkillQuickRef
                v-for="skillId in line.skillIds"
                :key="`affix-instance-skill-${section.key}-${idx}-${skillId}`"
                :skill-id="skillId"
              />
            </span>
          </template>
          <template v-else>{{ line.text }}</template>
        </li>
      </template>
      <template v-if="instanceLimitLines.length > 0">
        <li class="affix-limit-section-line">{{ t("game.affixLimitSection") }}</li>
        <li
          v-for="line in instanceLimitLines"
          :key="`affix-instance-limit-${line.itemId}`"
          class="affix-limit-line"
        >
          <QuickRef :item-id="line.itemId" />
          <span>{{ t("game.affixLimitCompact", { equipment: line.equipmentText, body: line.bodyText }) }}</span>
        </li>
      </template>
      <li v-if="hasHiddenInstanceHooks" class="affix-hook-trigger-line" style="display:flex;justify-content:flex-end;">
        <button style="font-size:0.7rem;opacity:0.6;background:transparent;border:0;cursor:pointer;padding:0;" @click="showAllInstanceHooks = !showAllInstanceHooks">{{ showAllInstanceHooks ? t("game.itemDetailBattleHooksShowLess") : t("game.itemDetailBattleHooksShowAll") }}</button>
      </li>
      <template v-for="hook in instanceHookDescriptions" :key="`affix-hook-${hook.key}`">
        <li class="affix-hook-trigger-line">{{ hook.triggerText }}</li>
        <li v-if="hook.conditions.length > 0" class="affix-hook-label-line">{{ t("game.itemDetailBattleHookIf") }}</li>
        <li
          v-for="condition in hook.conditions"
          :key="`affix-hook-condition-${hook.key}-${condition.key}`"
          class="affix-hook-content-line"
        >
          <span class="affix-hook-bullet">·</span>{{ condition.text }}
        </li>
        <li v-if="hook.conditions.length > 0" class="affix-hook-label-line">{{ t("game.itemDetailBattleHookThen") }}</li>
        <li v-for="effect in hook.effects" :key="`affix-hook-${hook.key}-${effect.key}`" class="affix-hook-content-line">
          <span class="affix-hook-bullet">·</span>
          <div v-if="effect.kind === 'add_buff' || effect.kind === 'remove_buff'" class="inline-flex flex-row items-center justify-start">
            <span>{{ t(effect.kind === 'add_buff' ? "game.itemDetailBattleHookEffectAddBuffPrefix" : "game.itemDetailBattleHookEffectRemoveBuffPrefix", { target: effect.targetText }) }}</span>
            <span class="m-2">
              <BuffQuickRef :buff-id="effect.buffId" />
            </span>
            <span v-if="effect.kind === 'add_buff' && effect.turns !== null">{{ t("game.itemDetailBattleHookEffectTurnsSuffix", { turns: effect.turns }) }}</span>
            <span v-if="effect.kind === 'add_buff' && effect.procChanceText">{{ effect.procChanceText }}</span>
            <span v-if="effect.kind === 'remove_buff' && effect.removeStack">{{ t("game.itemDetailBattleHookEffectRemoveBuffOneStack") }}</span>
          </div>
          <div v-else-if="effect.kind === 'heal'" class="inline-flex flex-row items-center justify-start">
            <span>{{ t("game.itemDetailBattleHookEffectHeal", { target: effect.targetText, formula: effect.formulaText }) }}</span>
            <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
          </div>
          <div v-else-if="effect.kind === 'summon'" class="inline-flex flex-row flex-wrap items-center justify-start gap-1">
            <span>{{ t("game.itemDetailBattleHookEffectSummonPrefix", { target: effect.targetText }) }}</span>
            <EnemyUnitQuickRef :enemy-id="effect.unitId" />
            <span>{{ effect.durationText }}</span>
            <span v-if="effect.countText">{{ effect.countText }}</span>
            <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
            <div v-if="effect.statBonusEntries.length" class="basis-full pl-4 pt-1">
              <div v-for="bonus in effect.statBonusEntries" :key="`affix-hook-${hook.key}-${effect.key}-${bonus.key}`">
                <span class="font-medium">{{ bonus.label }}:</span>
                <span>{{ bonus.value }}</span>
              </div>
            </div>
          </div>
          <div v-else-if="effect.kind === 'mirror_cast_skill'" class="inline-flex flex-col items-start justify-start">
            <span>{{ effect.descriptionText }}</span>
            <span v-if="effect.filterText" class="affix-hook-subline">{{ effect.filterText }}</span>
          </div>
          <div v-else-if="effect.kind === 'basic_attack_marked_target'" class="inline-flex flex-row flex-wrap items-center justify-start gap-1">
            <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetPrefix") }}</span>
            <BuffEffectQuickRef :effect-kind="effect.effectKind" />
            <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetSuffix") }}</span>
            <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
          </div>
          <template v-else>
            {{ formatHookEffectFallback(effect) }}
          </template>
        </li>
      </template>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import AffixTemplatePopoverContent from "@components/game/affix/AffixTemplatePopoverContent.vue"
import BuffEffectQuickRef from "@components/game/buff/BuffEffectQuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import { formatAttributeValueByKey } from "@services/attribute-display"
import { listAlchemyTraits } from "@services/alchemy-data"
import { buildBattleHookDescriptions } from "@services/battle-hook-display"
import {
  buildAffixMutationDisplayLines,
  canonicalizeAffixMutations,
  hasAffixMutationDisplayLines
} from "@services/affix-mutation-display"
import { getAffixSlotSourceBadgeDefinitions } from "@services/affix-source-badges"
import { getAffixDefinition, getRarityDefinition } from "@services/item-definition"
import type { EquipmentAffixSlot } from "@/types/equipment"
import { scoreAffixDefinition } from "@services/combat-scoring"
import GameDialog from "@components/game/common/GameDialog.vue"

const alchemyTraitLimitByItemId = new Map(
  listAlchemyTraits().map((trait) => [
    trait.itemId,
    {
      equipment: trait.maxEffectivePerEquipment,
      body: trait.maxEffectivePerBody
    }
  ])
)

type InstanceEffectTone = "positive" | "negative" | "neutral"
type InstanceEffectLine = {
  text: string
  tone: InstanceEffectTone
  label?: string
  skillIds?: string[]
}
type AlchemyLimitLine = {
  itemId: string
  equipmentText: string
  bodyText: string
}

const props = withDefaults(
  defineProps<{
    affixId: string
    slot?: EquipmentAffixSlot | null
    showInstanceDetails?: boolean
    showRarity?: boolean
    prefixLabel?: string
    templateRarityWeights?: Record<string, number> | null
  }>(),
  {
    slot: null,
    showInstanceDetails: false,
    showRarity: true,
    prefixLabel: "",
    templateRarityWeights: null
  }
)

const { t } = useI18n()
const { supportsHover, isTouchLike, isMobile } = useDevice()
const popoverActive = ref(false)
const popoverLoaded = ref(false)
const mobilePopoverVisible = ref(false)

const normalizedAffixID = computed(() => normalizeAffixID(props.affixId || props.slot?.affixId || ""))
const affixName = computed(() => localizeCustomAffixName(props.slot) || localizeAffixName(normalizedAffixID.value))
const affixScore = computed(() => scoreAffixDefinition(normalizedAffixID.value))
const rarityID = computed(() => String(props.slot?.rarityId || "").trim())
const hasTemplateRarityWeights = computed(() => {
  const weights = props.templateRarityWeights
  if (!weights || typeof weights !== "object" || Array.isArray(weights)) return false
  return Object.values(weights).some((value) => Number.isFinite(Number(value)) && Number(value) > 0)
})
const hasTemplatePopoverContent = computed(() => !!getAffixDefinition(normalizedAffixID.value) || (!props.slot && hasTemplateRarityWeights.value))
const enablePopover = computed(() => normalizedAffixID.value !== "" && (!!getAffixDefinition(normalizedAffixID.value) || !!props.slot || (!props.slot && hasTemplateRarityWeights.value)))
const resolvedInstanceEffectParams = computed(() => props.slot ? resolveAffixEffectParamsForSlot(props.slot) : {})
const resolvedTemplateEffectParams = computed(() => {
  const def = getAffixDefinition(normalizedAffixID.value)
  if (!def?.effectParams) return {}
  return resolveAffixEffectParamsByRarity(def.effectParams, rarityID.value)
})
const instanceMutationsDuplicateTemplate = computed(() => {
  if (!props.slot || !getAffixDefinition(normalizedAffixID.value)) return false
  const instanceMutations = resolvedInstanceEffectParams.value.effectMutations
  const templateMutations = resolvedTemplateEffectParams.value.effectMutations
  if (!hasAffixMutationDisplayLines(instanceMutations) || !hasAffixMutationDisplayLines(templateMutations)) return false
  return canonicalizeAffixMutations(instanceMutations) === canonicalizeAffixMutations(templateMutations)
})
const instanceEffectLines = computed(() => {
  if (!props.slot) return []
  return buildInstanceEffectLines(props.slot, resolvedInstanceEffectParams.value, {
    includeMutations: !instanceMutationsDuplicateTemplate.value
  })
})
const instanceEffectSections = computed(() => {
  const buckets: Record<InstanceEffectTone, InstanceEffectLine[]> = {
    positive: [],
    negative: [],
    neutral: []
  }
  for (const line of instanceEffectLines.value) buckets[line.tone].push(line)
  return (["positive", "negative", "neutral"] as InstanceEffectTone[])
    .filter((tone) => buckets[tone].length > 0)
    .map((tone) => ({
      key: tone,
      label: t(`game.affixEffectSection.${tone}`),
      lines: buckets[tone]
    }))
})
const instanceLimitLines = computed(() => props.slot ? buildAlchemyComponentLimitLines(props.slot) : [])
const sourceBadges = computed(() => getAffixSlotSourceBadgeDefinitions(props.slot).map((badge) => ({ ...badge, title: t(badge.titleKey) })))
const showAllInstanceHooks = ref(false)
const allInstanceHookDescriptions = computed(() => {
  if (!props.slot) return []
  return buildBattleHookDescriptions(resolvedInstanceEffectParams.value.battleHooks, t)
})
const hasHiddenInstanceHooks = computed(() => allInstanceHookDescriptions.value.some((n) => n.hidden))
const instanceHookDescriptions = computed(() =>
  showAllInstanceHooks.value ? allInstanceHookDescriptions.value : allInstanceHookDescriptions.value.filter((n) => !n.hidden)
)
const hasInstanceDetails = computed(() => instanceEffectLines.value.length > 0 || instanceLimitLines.value.length > 0 || instanceHookDescriptions.value.length > 0 || hasHiddenInstanceHooks.value)
const showPopoverInstanceDetails = computed(() => !!props.slot && hasInstanceDetails.value)
const tagStyle = computed(() => {
  const color = rarityID.value ? getRarityDefinition(rarityID.value)?.color || "" : ""
  if (!color) return {}
  return {
    borderColor: color,
    color
  }
})
const dialogWidth = computed(() => {
  if (isMobile.value) return "92%"
  return "520px"
})

function normalizeAffixID(affixID: string): string {
  const normalized = String(affixID || "").trim()
  if (normalized.startsWith("affix.")) return normalized.slice("affix.".length)
  return normalized
}

function localizeAffixName(affixID: string): string {
  const keys = [`affix.${affixID}.name`, `${affixID}.name`].filter(Boolean)
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return affixID
}

function localizeCustomAffixName(slot: EquipmentAffixSlot | null | undefined): string {
  if (!slot?.displayNameKey) return ""
  const key = String(slot.displayNameKey || "").trim()
  if (!key) return ""
  const args = buildDisplayNameArgs(slot.displayNameArgs)
  const localized = t(key, args)
  if (localized !== key && localized.trim() !== "") return localized
  return Object.values(args).filter((value) => String(value || "").trim() !== "").join("")
}

function buildDisplayNameArgs(raw: Record<string, unknown> | undefined): Record<string, string> {
  const out: Record<string, string> = {}
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return out
  for (const [key, value] of Object.entries(raw)) {
    if (key.endsWith("Key") && typeof value === "string") {
      out[key.slice(0, -"Key".length)] = localizeLooseKey(value)
      continue
    }
    if (key.endsWith("Keys") && Array.isArray(value)) {
      out[key.slice(0, -"Keys".length)] = value.map((entry) => localizeLooseKey(String(entry || ""))).filter(Boolean).join(t("alchemy.affix.prefixSeparator"))
      continue
    }
    out[key] = String(value ?? "")
  }
  return out
}

function localizeLooseKey(key: string): string {
  const normalized = String(key || "").trim()
  if (!normalized) return ""
  const localized = t(normalized)
  return localized === normalized ? normalized : localized
}

function localizeAttr(attrKey: string): string {
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function buildInstanceEffectLines(slot: EquipmentAffixSlot, effectParams: Record<string, unknown>, options: { includeMutations: boolean }): InstanceEffectLine[] {
  const lines: InstanceEffectLine[] = []
  const attrMods = Object.keys(slot.attrMods || {}).length > 0
    ? slot.attrMods || {}
    : resolveAttrModsFromEffectParams(effectParams, slot.rollValue ?? 1)
  for (const [attrKey, value] of Object.entries(attrMods)) {
    if (!Number.isFinite(value)) continue
    const numeric = Number(value)
    lines.push({
      text: `${localizeAttr(attrKey)} ${formatAttributeValueByKey(attrKey, numeric, true)}`,
      tone: numeric > 0 ? "positive" : numeric < 0 ? "negative" : "neutral"
    })
  }
  if (options.includeMutations) {
    lines.push(...buildInstanceMutationLines(effectParams))
  }
  return lines
}

function buildInstanceMutationLines(effectParams: Record<string, unknown>): InstanceEffectLine[] {
  return buildAffixMutationDisplayLines(effectParams.effectMutations, t).map((line) => ({
    text: line.skillIds?.length ? line.label : `${line.label}: ${line.value}`,
    tone: "neutral",
    label: line.label,
    skillIds: line.skillIds
  }))
}

function buildAlchemyComponentLimitLines(slot: EquipmentAffixSlot): AlchemyLimitLine[] {
  const params = slot.effectParams && typeof slot.effectParams === "object" && !Array.isArray(slot.effectParams)
    ? slot.effectParams as Record<string, unknown>
    : {}
  const raw = params.alchemyComponents
  if (!Array.isArray(raw)) return []
  const seen = new Set<string>()
  const lines: AlchemyLimitLine[] = []
  for (const entry of raw) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
    const row = entry as Record<string, unknown>
    const itemId = String(row.itemId || row.traitId || "").trim()
    if (!itemId || seen.has(itemId)) continue
    const latestLimit = alchemyTraitLimitByItemId.get(itemId)
    const equipment = Math.max(0, Math.round(Number(latestLimit?.equipment ?? row.maxEffectivePerEquipment ?? 0)))
    const body = Math.max(0, Math.round(Number(latestLimit?.body ?? row.maxEffectivePerBody ?? 0)))
    if (equipment <= 0 && body <= 0) continue
    seen.add(itemId)
    lines.push({
      itemId,
      equipmentText: String(equipment || "-"),
      bodyText: String(body || "-")
    })
  }
  return lines
}

function resolveAffixEffectParamsForSlot(slot: EquipmentAffixSlot): Record<string, unknown> {
  const slotParams = slot.effectParams && typeof slot.effectParams === "object" && !Array.isArray(slot.effectParams)
    ? slot.effectParams as Record<string, unknown>
    : {}
  if (Object.keys(slotParams).length > 0) {
    return resolveAffixEffectParamsByRarity(slotParams, slot.rarityId || "")
  }
  const def = getAffixDefinition(normalizeAffixID(slot.affixId || ""))
  if (!def?.effectParams) return {}
  return resolveAffixEffectParamsByRarity(def.effectParams, slot.rarityId || "")
}

function resolveAffixEffectParamsByRarity(effectParams: Record<string, unknown>, rarityID: string): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, raw] of Object.entries(effectParams || {})) {
    out[key] = selectRarityScopedParam(raw, rarityID)
  }
  return out
}

function selectRarityScopedParam(raw: unknown, rarityID: string): unknown {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return raw
  const node = raw as Record<string, unknown>
  const normalizedRarity = String(rarityID || "").trim()
  if (normalizedRarity && node[normalizedRarity] !== undefined) return node[normalizedRarity]
  if (node.default !== undefined) return node.default
  if (node["*"] !== undefined) return node["*"]
  return raw
}

function resolveAttrModsFromEffectParams(effectParams: Record<string, unknown>, rollValue: number): Record<string, number> {
  const rawAttrMods = effectParams.attrMods
  if (!rawAttrMods || typeof rawAttrMods !== "object" || Array.isArray(rawAttrMods)) return {}
  const out: Record<string, number> = {}
  for (const [attrKey, value] of Object.entries(rawAttrMods as Record<string, unknown>)) {
    const numeric = resolveRolledNumericValue(value, rollValue)
    if (!Number.isFinite(numeric)) continue
    out[attrKey] = numeric
  }
  return out
}

function resolveRolledNumericValue(raw: unknown, rollValue: number): number {
  const direct = Number(raw)
  if (Number.isFinite(direct)) return direct
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return Number.NaN
  const node = raw as Record<string, unknown>
  const minRaw = Number(node.min)
  const maxRaw = Number(node.max)
  if (!Number.isFinite(minRaw) && !Number.isFinite(maxRaw)) return Number.NaN
  const min = Number.isFinite(minRaw) ? minRaw : maxRaw
  const max = Number.isFinite(maxRaw) ? maxRaw : minRaw
  const clampedRoll = Math.max(0, Math.min(1, Number.isFinite(Number(rollValue)) ? Number(rollValue) : 1))
  return min + (max - min) * clampedRoll
}

function formatHookEffectFallback(effect: { text?: string; formulaText?: string }): string {
  return String(effect.text || effect.formulaText || "")
}

function onPopoverShow() {
  if (!supportsHover.value || !enablePopover.value) return
  popoverLoaded.value = true
  popoverActive.value = true
}

function onPopoverHide() {
  popoverActive.value = false
}

function primePopover() {
  if (!supportsHover.value || !enablePopover.value) return
  popoverLoaded.value = true
}

function openMobilePopover() {
  if (!enablePopover.value) return
  mobilePopoverVisible.value = true
}

function onTagClick() {
  if (!isTouchLike.value) return
  openMobilePopover()
}
</script>

<style scoped>
.affix-quick-ref {
  display: grid;
  gap: 4px;
}

.affix-quick-ref-content {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.affix-score-mini {
  font-size: 11px;
  opacity: 0.72;
}

.affix-quick-ref-popover-stack {
  display: grid;
  gap: 8px;
}

.affix-quick-ref-source-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding-top: 6px;
  border-top: 1px solid var(--app-border, #dde4ea);
}

.affix-quick-ref-source-badge {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 7px 2px 5px;
  border-radius: 999px;
  color: #fff;
  font-size: 11px;
  line-height: 1.35;
}

.affix-quick-ref-source-badge :deep(svg) {
  width: 13px;
  height: 13px;
  flex: 0 0 auto;
}

.affix-quick-ref-source-badge.is-craft {
  background: #1f9d61;
}

.affix-quick-ref-source-badge.is-inherit {
  background: #2f6fdb;
}

.affix-quick-ref-source-badge.is-orphan {
  background: #d64550;
}

.affix-quick-ref-source-badge.is-ordinary {
  background: #667085;
}

.affix-quick-ref-lines {
  font-size: 12px;
  color: var(--el-text-color-regular);
  display: grid;
  gap: 2px;
}

.affix-effect-section-line {
  margin-top: 6px;
  color: var(--app-muted, #667085);
  font-size: 11px;
  font-weight: 700;
  line-height: 1.35;
}

.affix-effect-section-line:first-child {
  margin-top: 0;
}

.affix-effect-section-line.is-positive {
  color: #047857;
}

.affix-effect-section-line.is-negative {
  color: #e11d48;
}

.affix-effect-value-line {
  padding: 1px 0 1px 8px;
  border-left: 2px solid transparent;
  line-height: 1.45;
}

.affix-effect-line-label {
  color: var(--el-text-color-secondary);
}

.affix-effect-skill-list {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  margin-left: 4px;
  vertical-align: middle;
}

.affix-effect-value-line.is-positive {
  border-left-color: color-mix(in srgb, #10b981 45%, transparent);
  color: #047857;
}

.affix-effect-value-line.is-negative {
  border-left-color: color-mix(in srgb, #f43f5e 45%, transparent);
  color: #e11d48;
}

.affix-effect-value-line.is-neutral {
  color: var(--el-text-color-regular);
}

.affix-hook-trigger-line {
  font-weight: 600;
}

.affix-hook-label-line {
  color: var(--el-text-color-secondary);
  padding-left: 8px;
}

.affix-hook-content-line {
  padding-left: 16px;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.affix-limit-section-line {
  margin-top: 7px;
  padding-top: 8px;
  border-top: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 82%, transparent);
  color: var(--el-text-color-secondary);
  font-size: 11px;
  font-weight: 800;
  line-height: 1.35;
}

.affix-limit-line {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--el-text-color-secondary);
  font-size: 11px;
  line-height: 1.4;
}

.affix-limit-line :deep(.item-quick-ref) {
  min-width: 0;
}

.affix-limit-line :deep(.item-quick-ref-name) {
  max-width: 118px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.affix-limit-line > span {
  flex: 0 0 auto;
  color: var(--app-muted, #667085);
}

.affix-hook-bullet {
  color: var(--el-text-color-secondary);
}

.affix-hook-subline {
  color: var(--el-text-color-secondary);
}

.affix-quick-ref-dialog :deep(.el-dialog__body) {
  padding-top: 12px;
}
</style>
