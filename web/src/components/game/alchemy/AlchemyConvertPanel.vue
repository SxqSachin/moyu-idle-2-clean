<template>
  <div class="alchemy-panel">
    <OperationEffectNotice :context="conversionContext" />
    <div class="alchemy-panel__row" data-tutorial="alchemy-convert-source">
      <div class="alchemy-panel__label">{{ t("alchemy.convert.material") }}</div>
      <div class="flex flex-wrap items-center gap-3">
        <Card
          v-if="selectedEntry"
          :item-id="selectedEntry.itemId"
          :instance-id="selectedEntry.instanceId"
          :quantity="selectedEntry.quantity"
          @click="selectorVisible = true"
        />
        <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="selectorVisible = true" />
        <div v-if="selectedEntry" class="text-xs app-muted">
          {{ t("alchemy.convert.group") }} · {{ t(selectedEntry.group.nameKey) }}
        </div>
        <StatusRequireTag
          v-if="selectedEntry"
          :attr-key="'alchemy_level'"
          :required-value="selectedEntry.group.minAlchemyLevel"
          :current-value="playerAlchemyLevel"
          placement="bottom-start"
          :popover-width="320"
        />
      </div>
    </div>

    <div v-if="selectedEntry" class="alchemy-panel__row" data-tutorial="alchemy-convert-outcomes">
      <div class="alchemy-panel__label">{{ t("alchemy.convert.outcomes") }}</div>
      <div class="alchemy-prob-summary">
        <div
          v-for="section in outcomeSections"
          :key="`summary-${section.key}`"
          class="alchemy-prob-summary__item"
          :class="sectionClass(section.key)"
        >
          <div class="alchemy-prob-summary__top">
            <span>{{ section.title }}</span>
            <strong>{{ formatPercent(section.totalProbability) }}</strong>
          </div>
          <div class="alchemy-prob-summary__bar">
            <span :style="{ width: percentBarWidth(section.totalProbability) }" />
          </div>
        </div>
      </div>
      <div class="alchemy-outcome-sections">
        <div
          v-for="section in outcomeSections"
          :key="section.key"
          class="alchemy-outcome-section"
          :class="sectionClass(section.key)"
        >
          <div class="alchemy-outcome-section__head">
            <span>{{ section.title }}</span>
            <span>{{ formatPercent(section.totalProbability) }}</span>
          </div>
          <div class="alchemy-outcome-list" :class="{ 'alchemy-outcome-list--single': section.key === 'fail' }">
            <div
              v-for="outcome in section.rows"
              :key="outcome.key"
              class="alchemy-outcome"
              :class="{ 'alchemy-outcome--source': outcome.itemId === selectedEntry.itemId, 'alchemy-outcome--fail': outcome.section === 'fail' }"
            >
              <div class="alchemy-outcome__icon">
                <div v-if="outcome.section === 'fail'" class="alchemy-outcome__fail-icon">!</div>
                <Card v-else :item-id="outcome.itemId" hide-label :width="42" :height="42" />
              </div>
              <div class="alchemy-outcome__body">
                <div class="alchemy-outcome__top">
                  <span class="alchemy-outcome__name">{{ outcome.name }}</span>
                  <span class="alchemy-outcome__prob">{{ formatPercent(outcome.probability) }}</span>
                </div>
                <div class="alchemy-outcome__meta">
                  {{ outcome.meta }}
                </div>
                <FoodValueHint v-if="outcome.section === 'fail' && conversionReduction > 0">{{ t('game.fishCuisine.conversionLoss', { percent: Number((conversionReduction * 100).toFixed(2)) }) }}</FoodValueHint>
                <div class="alchemy-outcome__bar">
                  <span :style="{ width: `${Math.max(4, outcome.probability * 100)}%` }" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <AlchemyActionSubmitBar
      v-if="selectedEntry"
      data-tutorial="alchemy-convert-submit"
      action-id="alchemy.convert"
      :args="submitArgs"
      :count="sourceCount"
      :max-count="selectedEntry.quantity"
      :per-run-duration-sec="selectedEntry.group.durationSec"
      :gold-cost="selectedEntry.group.goldCost * sourceCount"
      :exp-reward="selectedEntry.group.alchemyExp * sourceCount"
      :success-rate="convertSuccessChance"
      :disabled="!canSubmit"
      :gold-breakdown="convertGoldBreakdown"
      :duration-breakdown="convertDurationBreakdown"
      :exp-breakdown="convertExpBreakdown"
      @update:count="sourceCount = $event"
      @submitted="emit('submitted')"
    />

    <div v-else class="text-sm app-muted py-2">{{ t("alchemy.convert.empty") }}</div>

    <InventoryInstanceSelectorDialog
      v-model="selectorVisible"
      :title="t('alchemy.convert.pickerTitle')"
      :instance-ids="convertibleInstanceIds"
      :selected-instance-id="selectedInstanceId"
      :allow-clear="false"
      :template-item-ids="convertibleTemplateIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.convert.material') })"
      :empty-text="t('alchemy.convert.empty')"
      @confirm="handleSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import FoodValueHint from "@components/game/operation-effect/FoodValueHint.vue"
import { useConversionFood } from "@/composables/game/useConversionFood"
import { conversionFoodFailureChance } from "@services/conversion-food"
import { useI18n } from "vue-i18n"
import { useInventoryStore } from "@stores/inventory"
import { usePlayerStore } from "@stores/player"
import {
  applyAlchemyMinimumLossFailChance,
  applyAlchemyStabilityToFailChance,
  computeAlchemyLevelDeficitFailDelta,
  getAlchemyGroupForItem,
  resolveAlchemyMinFailChance,
  type AlchemyGroupEntry,
  type AlchemyGroup
} from "@services/alchemy-data"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import StatusRequireTag from "@components/game/attribute/StatusRequireTag.vue"
import AlchemyActionSubmitBar from "./AlchemyActionSubmitBar.vue"
import { formatDurationI18n } from "@/utils/duration-format"

type ConvertEntry = {
  instanceId: number
  itemId: string
  quantity: number
  group: AlchemyGroup
}

type OutcomeRow = {
  key: string
  itemId: string
  name: string
  section: "fail" | "base" | "upgrade"
  baseProbability: number
  delta: number
  probability: number
  meta: string
}

type OutcomeSection = {
  key: OutcomeRow["section"]
  title: string
  totalProbability: number
  rows: OutcomeRow[]
}

const emit = defineEmits<{
  (e: "submitted"): void
}>()

const props = withDefaults(defineProps<{
  preferredInstanceId?: number | null
}>(), {
  preferredInstanceId: null
})

const { t } = useI18n()
const inventoryStore = useInventoryStore()
const playerStore = usePlayerStore()

const selectorVisible = ref(false)
const selectedInstanceId = ref<number | null>(null)
const sourceCount = ref(1)

const convertibleInstances = computed<ConvertEntry[]>(() =>
  inventoryStore.instances
    .filter((row) => row.state === "inventory" && row.quantity > 0)
    .map((row) => {
      const group = getAlchemyGroupForItem(row.templateItemId)
      return group ? { instanceId: row.instanceId, itemId: row.templateItemId, quantity: row.quantity, group } : null
    })
    .filter((entry): entry is ConvertEntry => !!entry)
)
const convertibleInstanceIds = computed(() => convertibleInstances.value.map((row) => row.instanceId))
const convertibleTemplateIds = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const entry of convertibleInstances.value) {
    if (!seen.has(entry.itemId)) {
      seen.add(entry.itemId)
      out.push(entry.itemId)
    }
  }
  return out
})

const selectedEntry = computed<ConvertEntry | null>(() =>
  convertibleInstances.value.find((row) => row.instanceId === selectedInstanceId.value) || null
)
const conversionContext = computed(() => ({domain:'alchemy' as const,actionId:'alchemy.convert',actorKind:'player' as const,conversionInput:!!selectedEntry.value}))
const conversionReduction = useConversionFood(() => conversionContext.value)
const playerAlchemyLevel = computed(() => Number(playerStore.attributes?.alchemy_level?.value ?? 1))
const playerAlchemyLuck = computed(() => Number(playerStore.attributes?.alchemy_lucky_chance?.value ?? 0))
const playerAlchemyStability = computed(() => Number(playerStore.attributes?.alchemy_craft_success?.value ?? 0))
const convertSuccessChance = computed(() => {
  if (!selectedEntry.value) return null
  return 1 - conversionFoodFailureChance(convertFailChance(selectedEntry.value.group, playerAlchemyStability.value, playerAlchemyLevel.value), conversionReduction.value)
})

const outcomes = computed<OutcomeRow[]>(() => {
  if (!selectedEntry.value) return []
  const group = selectedEntry.value.group
  const sourceItemId = selectedEntry.value.itemId
  const baseProbabilities = buildConvertProbabilityMap(group, sourceItemId, 0, 0, group.minAlchemyLevel)
  const finalProbabilities = buildConvertProbabilityMap(group, sourceItemId, playerAlchemyLuck.value, playerAlchemyStability.value, playerAlchemyLevel.value, conversionReduction.value)
  const rows: OutcomeRow[] = []
  for (const [itemId, row] of finalProbabilities) {
    const baseProbability = baseProbabilities.get(itemId)?.probability || 0
    const delta = row.probability - baseProbability
    const isFail = row.section === "fail"
    rows.push({
      key: itemId,
      itemId,
      name: isFail ? t("alchemy.convert.failOutcomeName") : t(`item.${itemId}.name`, itemId),
      section: row.section,
      baseProbability,
      delta,
      probability: row.probability,
      meta: isFail
        ? t("alchemy.convert.failHint", { base: formatPercent(baseProbability), delta: formatSignedPercent(convertFailChance(group, playerAlchemyStability.value, playerAlchemyLevel.value) - baseProbability) })
        : t("alchemy.convert.probHint", { base: formatPercent(baseProbability), delta: formatSignedPercent(delta) })
    })
  }
  return rows
    .sort((a, b) => sectionOrder(a.section) - sectionOrder(b.section) || b.probability - a.probability)
})

const outcomeSections = computed<OutcomeSection[]>(() => {
  const sections: OutcomeSection[] = []
  for (const key of ["fail", "base", "upgrade"] as const) {
    const rows = outcomes.value.filter((row) => row.section === key)
    if (rows.length === 0) continue
    sections.push({
      key,
      title: outcomeSectionTitle(key),
      totalProbability: rows.reduce((sum, row) => sum + row.probability, 0),
      rows
    })
  }
  return sections
})

const canSubmit = computed(() => !!selectedEntry.value && sourceCount.value >= 1)
const submitArgs = computed(() => selectedEntry.value
  ? {
      sourceItemId: selectedEntry.value.itemId,
      groupId: selectedEntry.value.group.id,
    }
  : {})
const convertGoldBreakdown = computed(() => selectedEntry.value
  ? [{ label: t("alchemy.convert.singleCost"), value: t("alchemy.cost.gold", { value: selectedEntry.value.group.goldCost }) }, { label: t("alchemy.convert.countLabel"), value: `x${sourceCount.value}` }]
  : [])
const convertDurationBreakdown = computed(() => selectedEntry.value
  ? [{ label: t("alchemy.convert.singleDuration"), value: formatSeconds(selectedEntry.value.group.durationSec) }, { label: t("alchemy.convert.countLabel"), value: `x${sourceCount.value}` }]
  : [])
const convertExpBreakdown = computed(() => selectedEntry.value
  ? [{ label: t("alchemy.convert.singleExp"), value: formatNumber(selectedEntry.value.group.alchemyExp, 2) }, { label: t("alchemy.convert.countLabel"), value: `x${sourceCount.value}` }]
  : [])

watch(selectedInstanceId, () => {
  sourceCount.value = 1
})

const normalizedPreferredInstanceId = computed(() => {
  const value = Math.trunc(Number(props.preferredInstanceId || 0))
  return Number.isFinite(value) && value > 0 ? value : null
})

watch(
  normalizedPreferredInstanceId,
  (instanceId) => {
    if (!instanceId || !convertibleInstanceIds.value.includes(instanceId)) return
    selectedInstanceId.value = instanceId
  },
  { immediate: true }
)

watch(convertibleInstanceIds, (ids) => {
  const preferred = normalizedPreferredInstanceId.value
  if (preferred && ids.includes(preferred)) {
    selectedInstanceId.value = preferred
    return
  }
  if (selectedInstanceId.value !== null && !ids.includes(selectedInstanceId.value)) {
    selectedInstanceId.value = ids[0] || null
  } else if (selectedInstanceId.value === null && ids.length > 0) {
    selectedInstanceId.value = ids[0]
  }
}, { immediate: true })

function handleSelect(instanceId: number | null) {
  selectedInstanceId.value = instanceId
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "0%"
  return `${(value * 100).toFixed(2)}%`
}

function formatSignedPercent(value: number): string {
  const sign = value >= 0 ? "+" : ""
  return `${sign}${formatPercent(value)}`
}

function buildConvertProbabilityMap(group: AlchemyGroup, sourceItemId: string, luck: number, stability: number, alchemyLevel: number, foodReduction = 0): Map<string, { section: OutcomeRow["section"]; probability: number }> {
  const baseWeights = adjustConvertWeightsWithLuck(group.entries, sourceItemId, false, luck)
  const upgradeWeights = adjustConvertWeightsWithLuck(group.upgradeEntries, "", true, luck)
  const upgradeChance = convertUpgradeChance(group, upgradeWeights.length)
  const failChance = conversionFoodFailureChance(convertFailChance(group, stability, alchemyLevel), foodReduction)
  const successChance = 1 - failChance
  const baseTotal = baseWeights.reduce((sum, row) => sum + row.finalWeight, 0)
  const upgradeTotal = upgradeWeights.reduce((sum, row) => sum + row.finalWeight, 0)
  const out = new Map<string, { section: OutcomeRow["section"]; probability: number }>()
  if (failChance > 0) {
    out.set("__alchemy_convert_fail__", { section: "fail", probability: failChance })
  }
  if (baseTotal > 0) {
    for (const entry of baseWeights) {
      out.set(entry.itemId, { section: "base", probability: successChance * (1 - (upgradeTotal > 0 ? upgradeChance : 0)) * entry.finalWeight / baseTotal })
    }
  }
  if (upgradeTotal > 0) {
    for (const entry of upgradeWeights) {
      out.set(entry.itemId, { section: "upgrade", probability: successChance * upgradeChance * entry.finalWeight / upgradeTotal })
    }
  }
  return out
}

function convertFailChance(group: AlchemyGroup, stability: number, alchemyLevel: number): number {
  return applyAlchemyMinimumLossFailChance(
    applyAlchemyStabilityToFailChance(
      Math.min(1, Math.max(0, group.failChanceBase + computeAlchemyLevelDeficitFailDelta(alchemyLevel, group.minAlchemyLevel))),
      stability
    ),
    resolveAlchemyMinFailChance(group.minFailChance, 0.15)
  )
}

function convertUpgradeChance(group: AlchemyGroup, upgradeEntryCount: number): number {
  if (upgradeEntryCount <= 0) return 0
  return Math.min(1, Math.max(0, group.upgradeChanceBase))
}

function adjustConvertWeightsWithLuck(entries: AlchemyGroupEntry[], sourceItemId: string, allowSource: boolean, luck: number): Array<{ itemId: string; baseWeight: number; finalWeight: number }> {
  const filtered = entries.filter((entry) => entry.itemId && entry.weight > 0 && (allowSource || entry.itemId !== sourceItemId || entries.length <= 1))
  const source = filtered.length > 0 ? filtered : entries.filter((entry) => entry.itemId && entry.weight > 0)
  const exponent = 1 - Math.min(0.30, Math.max(0, luck * 0.9))
  return source.map((entry) => ({
    itemId: entry.itemId,
    baseWeight: entry.weight,
    finalWeight: Math.pow(entry.weight, exponent)
  }))
}

function sectionOrder(section: OutcomeRow["section"]): number {
  if (section === "fail") return 0
  if (section === "base") return 1
  return 2
}

function outcomeSectionTitle(section: OutcomeRow["section"]): string {
  if (section === "fail") return t("alchemy.convert.failedOutcome")
  if (section === "base") return t("alchemy.convert.baseOutcome")
  return t("alchemy.convert.upgradeOutcome")
}

function sectionClass(section: OutcomeRow["section"]): string {
  return `alchemy-outcome-section--${section}`
}

function percentBarWidth(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0%"
  return `${Math.min(100, Math.max(2, value * 100))}%`
}

function formatSeconds(value: number): string {
  return formatDurationI18n(Math.max(0, Number(value) || 0), t)
}

function formatNumber(value: number, digits = 0): string {
  if (!Number.isFinite(value)) return "0"
  return value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1")
}
</script>

<style scoped>
.alchemy-panel {
  display: grid;
  gap: 16px;
}

.alchemy-panel__row {
  display: grid;
  gap: 8px;
}

.alchemy-panel__row--actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}

.alchemy-panel__label {
  font-size: 13px;
  font-weight: 700;
  color: var(--app-muted);
}

.alchemy-prob-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 8px;
}

.alchemy-prob-summary__item {
  --alchemy-outcome-accent: var(--el-color-primary);
  display: grid;
  gap: 6px;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--alchemy-outcome-accent) 24%, var(--app-border));
  border-radius: 8px;
  background: color-mix(in srgb, var(--alchemy-outcome-accent) 5%, var(--app-surface, transparent));
}

.alchemy-prob-summary__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  color: var(--app-muted);
}

.alchemy-prob-summary__top strong {
  font-size: 14px;
  color: var(--alchemy-outcome-accent);
  white-space: nowrap;
}

.alchemy-prob-summary__bar {
  height: 5px;
  border-radius: 99px;
  overflow: hidden;
  background: color-mix(in srgb, var(--alchemy-outcome-accent) 12%, transparent);
}

.alchemy-prob-summary__bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--alchemy-outcome-accent);
}

.alchemy-outcome-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 8px;
}

.alchemy-outcome-list--single {
  grid-template-columns: minmax(0, 1fr);
}

.alchemy-outcome-sections {
  display: grid;
  gap: 12px;
}

.alchemy-outcome-section {
  --alchemy-outcome-accent: var(--el-color-primary);
  display: grid;
  gap: 9px;
  padding: 10px;
  border: 1px solid color-mix(in srgb, var(--alchemy-outcome-accent) 16%, var(--app-border));
  border-radius: 8px;
  background: color-mix(in srgb, var(--alchemy-outcome-accent) 3%, transparent);
}

.alchemy-outcome-section--fail {
  --alchemy-outcome-accent: var(--el-color-warning);
}

.alchemy-outcome-section--base {
  --alchemy-outcome-accent: var(--el-color-primary);
}

.alchemy-outcome-section--upgrade {
  --alchemy-outcome-accent: var(--el-color-success);
}

.alchemy-outcome-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  font-weight: 700;
  color: var(--app-muted);
}

.alchemy-outcome {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 9px;
  border: 1px solid color-mix(in srgb, var(--app-border) 80%, transparent);
  border-radius: 8px;
  background: var(--app-surface, transparent);
  min-width: 0;
}

.alchemy-outcome--source {
  opacity: 0.5;
}

.alchemy-outcome--fail {
  border-color: color-mix(in srgb, var(--alchemy-outcome-accent) 45%, var(--app-border));
}

.alchemy-outcome__icon {
  width: 42px;
  height: 42px;
  overflow: hidden;
  display: grid;
  place-items: center;
}

.alchemy-outcome__fail-icon {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-weight: 800;
  color: var(--el-color-warning);
  background: color-mix(in srgb, var(--el-color-warning) 14%, transparent);
  border: 1px solid color-mix(in srgb, var(--el-color-warning) 35%, transparent);
}

.alchemy-outcome__body {
  display: grid;
  gap: 5px;
  min-width: 0;
  flex: 1;
}

.alchemy-outcome__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.alchemy-outcome__name {
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.alchemy-outcome__prob {
  font-size: 13px;
  font-weight: 700;
  color: var(--alchemy-outcome-accent);
  white-space: nowrap;
}

.alchemy-outcome__bar {
  height: 4px;
  border-radius: 99px;
  background: color-mix(in srgb, var(--alchemy-outcome-accent) 12%, transparent);
  overflow: hidden;
}

.alchemy-outcome__meta {
  color: var(--app-muted);
  font-size: 11px;
}

.alchemy-outcome__bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--alchemy-outcome-accent);
}
</style>
