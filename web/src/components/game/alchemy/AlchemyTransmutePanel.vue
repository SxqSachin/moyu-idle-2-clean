<template>
  <div class="alchemy-panel">
    <OperationEffectNotice :context="foodContext" />
    <div class="alchemy-panel__row" data-tutorial="alchemy-transmute-target">
      <div class="alchemy-panel__label">
        {{ t("alchemy.transmute.item") }}
      </div>
      <div class="alchemy-transmute-target">
        <div class="alchemy-transmute-target__item">
          <Card
            v-if="selectedEntry"
            :item-id="selectedEntry.itemId"
            :instance-id="selectedEntry.instanceId"
            :quantity="selectedEntry.quantity"
            @click="selectorVisible = true"
          />
          <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="selectorVisible = true" />
        </div>
        <div v-if="selectedEntry" class="alchemy-transmute-summary">
          <div class="alchemy-transmute-summary__item">
            <span>{{ t("alchemy.transmute.valueLabel") }}</span>
            <strong>{{ selectedEntry.unitValue }}</strong>
          </div>
          <div class="alchemy-transmute-summary__item">
            <span>{{ t("alchemy.transmute.levelRequirement") }}</span>
            <StatusRequireTag
              :attr-key="'alchemy_level'"
              :required-value="selectedEntry.minAlchemyLevel"
              :current-value="playerAlchemyLevel"
              placement="bottom-start"
              :popover-width="320"
            />
          </div>
          <div class="alchemy-transmute-summary__item alchemy-transmute-summary__item--gold">
            <span>{{ t("alchemy.transmute.estimatedGoldLabel") }}</span>
            <div class="alchemy-transmute-summary__gold">
              <QuickRef item-id="gold_coin" :count="estimatedMinTotal" normal-number-format />
              <span>~</span>
              <QuickRef item-id="gold_coin" :count="estimatedMaxTotal" normal-number-format />
<FoodValueHint v-if="foodModifiers.bonusRate > 0">{{ t("game.fishCuisine.foodGoldPreview", { min: transmuteFoodBonus(estimatedMin, foodModifiers.bonusRate), max: transmuteFoodBonus(Math.floor(estimatedMax), foodModifiers.bonusRate) }) }}</FoodValueHint>
            </div>
          </div>
          <div class="alchemy-transmute-summary__item">
            <span class="alchemy-transmute-summary__label">
              <span>{{ t("alchemy.transmute.luckyRateLabel") }}</span>
              <el-popover placement="top" :width="300" trigger="hover" :show-after="120">
                <template #reference>
                  <button type="button" class="alchemy-transmute-summary__help" :aria-label="t('alchemy.transmute.luckyRateLabel')" @click.stop>
                    <Icon icon="ep:question-filled" />
                  </button>
                </template>
                <div class="alchemy-popover-rows">
                  <div><span>{{ t("alchemy.transmute.baseLuckyChance") }}</span><strong>{{ formatPercent(selectedRule?.luckyChanceBase || 0) }}</strong></div>
                  <div><span>{{ t("alchemy.transmute.luckOffset") }}</span><strong>{{ formatSignedPercent(alchemyLuck) }}</strong></div>
                  <div><span>{{ t("alchemy.transmute.finalLuckyChance") }}</span><strong>{{ formatPercent(luckyTriggerChance) }}</strong></div>
                </div>
              </el-popover>
            </span>
            <strong>{{ formatPercent(luckyTriggerChance) }}</strong>
          </div>
        </div>
      </div>
    </div>

    <div v-if="selectedEntry" class="alchemy-panel__row" data-tutorial="alchemy-transmute-rewards">
      <div class="alchemy-panel__label">{{ t("alchemy.transmute.luckyTiers") }}</div>
      <div v-if="luckyTiers.length === 0" class="text-xs app-muted">{{ t("alchemy.transmute.noLuckyTiers") }}</div>
      <div v-else class="alchemy-outcome-grid">
        <div v-for="tier in luckyTiers" :key="tier.id" class="alchemy-outcome">
          <div class="alchemy-outcome__coin">G</div>
          <div class="alchemy-outcome__meta">
            <div class="alchemy-outcome__name">{{ tier.name }}</div>
            <div class="alchemy-outcome__prob">{{ formatPercent(tier.chance) }}</div>
            <div class="text-[11px] app-muted">{{ t("alchemy.transmute.multiplier", { value: formatNumber(tier.value, 2) }) }}</div>
          </div>
        </div>
      </div>
    </div>

    <AlchemyActionSubmitBar
      v-if="selectedEntry"
      data-tutorial="alchemy-transmute-submit"
      action-id="alchemy.transmute"
      :args="submitArgs"
      :count="totalCount"
      :max-count="selectedEntry.quantity"
      :per-run-duration-sec="transmuteDurationSec"
      :duration-total-sec="transmuteFoodDuration(totalCount, transmuteDurationSec, foodModifiers.quickUses)"
      :gold-cost="0"
      :exp-reward="(selectedRule?.alchemyExp || 0) * totalCount"
      :success-rate="successChance"
      :success-breakdown="transmuteSuccessBreakdown"
      :disabled="!canSubmit"
      :duration-breakdown="transmuteDurationBreakdown"
      :exp-breakdown="transmuteExpBreakdown"
      @update:count="totalCount = $event"
      @submitted="emit('submitted')"
    >
<template #duration-effect><FoodValueHint v-if="foodModifiers.quickUses > 0">{{ t('game.fishCuisine.foodDurationPreview', { count: Math.min(totalCount, foodModifiers.quickUses), before: formatSeconds(transmuteDurationSec) }) }}</FoodValueHint></template>
    </AlchemyActionSubmitBar>

    <div v-else class="text-sm app-muted py-2">{{ t("alchemy.transmute.empty") }}</div>

    <InventoryInstanceSelectorDialog
      v-model="selectorVisible"
      :title="t('alchemy.transmute.pickerTitle')"
      :instance-ids="transmutableInstanceIds"
      :selected-instance-id="selectedInstanceId"
      :allow-clear="false"
      :template-item-ids="transmutableTemplateIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.transmute.item') })"
      :empty-text="t('alchemy.transmute.empty')"
      @confirm="handleSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import FoodValueHint from "@components/game/operation-effect/FoodValueHint.vue"
import { useTransmuteFood } from "@/composables/game/useTransmuteFood"
import { transmuteFoodBonus, transmuteFoodDuration } from "@services/cuisine-operation-modifiers"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useInventoryStore } from "@stores/inventory"
import { usePlayerStore } from "@stores/player"
import { applyAlchemyMinimumLossFailChance, applyAlchemyStabilityToFailChance, computeAlchemyLevelDeficitFailDelta, getAlchemyTransmuteRuleForItem, isAlchemyTransmutableItem, minAlchemyLevelForRarity, resolveAlchemyMinFailChance, type AlchemyTransmuteRule } from "@services/alchemy-data"
import { getItemDefinition, getRarityOrder } from "@services/item-definition"
import Card from "@components/game/item/Card.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import StatusRequireTag from "@components/game/attribute/StatusRequireTag.vue"
import AlchemyActionSubmitBar from "./AlchemyActionSubmitBar.vue"
import { formatDurationI18n } from "@/utils/duration-format"

type TransmuteEntry = {
  instanceId: number
  itemId: string
  quantity: number
  unitValue: number
  minAlchemyLevel: number
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
const totalCount = ref(1)

const selectedRule = computed(() => getAlchemyTransmuteRuleForItem(selectedEntry.value?.itemId || "") || null)
const ratioMin = computed(() => selectedRule.value?.returnRatio.min || 0)
const ratioMax = computed(() => selectedRule.value?.returnRatio.max || 0)
const playerAlchemyLevel = computed(() => Number(playerStore.attributes?.alchemy_level?.value ?? 1))
const alchemyLuck = computed(() => Number(playerStore.attributes?.alchemy_lucky_chance?.value ?? 0))
const alchemyStability = computed(() => Number(playerStore.attributes?.alchemy_craft_success?.value ?? 0))
const baseFailChance = computed(() => Math.min(1, Math.max(0, selectedRule.value?.failChanceBase || 0)))
const levelFailDelta = computed(() => selectedEntry.value ? computeAlchemyLevelDeficitFailDelta(playerAlchemyLevel.value, selectedEntry.value.minAlchemyLevel) : 0)
const failChanceBeforeStability = computed(() => Math.min(1, Math.max(0, baseFailChance.value + levelFailDelta.value)))
const failChanceAfterStability = computed(() => applyAlchemyStabilityToFailChance(failChanceBeforeStability.value, alchemyStability.value))
const transmuteMinFailChance = computed(() => resolveAlchemyMinFailChance(selectedRule.value?.minFailChance, 0.10))
const failChance = computed(() => applyAlchemyMinimumLossFailChance(failChanceAfterStability.value, transmuteMinFailChance.value))
const stabilityFailDelta = computed(() => failChanceAfterStability.value - failChanceBeforeStability.value)
const minimumLossDelta = computed(() => failChance.value - failChanceAfterStability.value)
const successChance = computed(() => 1 - failChance.value)
const luckyTriggerChance = computed(() => Math.min(1, Math.max(0, (selectedRule.value?.luckyChanceBase || 0) + alchemyLuck.value)))
const transmuteSuccessBreakdown = computed(() => {
  const rows = [
    { label: t("alchemy.transmute.baseFailChance"), value: formatPercent(baseFailChance.value) },
  ]
  if (levelFailDelta.value > 0) {
    rows.push({ label: t("alchemy.transmute.levelFailDelta"), value: formatSignedPercent(levelFailDelta.value) })
  }
  rows.push({ label: t("alchemy.transmute.stabilityOffset"), value: formatSignedPercent(stabilityFailDelta.value) })
  if (minimumLossDelta.value > 0) {
    rows.push({ label: t("alchemy.transmute.minimumLossFloor"), value: formatPercent(transmuteMinFailChance.value) })
  }
  rows.push({ label: t("alchemy.transmute.finalSuccessChance"), value: formatPercent(successChance.value) })
  return rows
})

const transmutableEntries = computed<TransmuteEntry[]>(() => {
  const out: TransmuteEntry[] = []
  for (const item of inventoryStore.instances) {
    if (item.state !== "inventory" || item.quantity <= 0) continue
    if (item.templateItemId === "gold" || item.templateItemId === "gold_coin") continue
    const definition = getItemDefinition(item.templateItemId)
    if (!isAlchemyTransmutableItem(item.templateItemId)) continue
    const unitValue = Number(definition?.economyBaseValue || 0)
    if (!Number.isFinite(unitValue) || unitValue <= 0) continue
    const rule = getAlchemyTransmuteRuleForItem(item.templateItemId)
    if (!rule) continue
    out.push({
      instanceId: item.instanceId,
      itemId: item.templateItemId,
      quantity: item.quantity,
      unitValue,
      minAlchemyLevel: Math.max(rule.minAlchemyLevel || 1, minAlchemyLevelForRarity(definition?.rarity))
    })
  }
  return out
})

const transmutableInstanceIds = computed(() => transmutableEntries.value.map((row) => row.instanceId))
const transmutableTemplateIds = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const entry of transmutableEntries.value) {
    if (!seen.has(entry.itemId)) {
      seen.add(entry.itemId)
      out.push(entry.itemId)
    }
  }
  return out
})

const selectedEntry = computed<TransmuteEntry | null>(() =>
  transmutableEntries.value.find((row) => row.instanceId === selectedInstanceId.value) || null
)

const { context: foodContext, modifiers: foodModifiers } = useTransmuteFood(() => selectedEntry.value?.itemId || "")
const estimatedMin = computed(() => selectedEntry.value ? Math.floor(selectedEntry.value.unitValue * ratioMin.value) : 0)
const estimatedMax = computed(() => selectedEntry.value ? Math.ceil(selectedEntry.value.unitValue * ratioMax.value) : 0)
const estimatedMinTotal = computed(() => estimatedMin.value * Math.max(1, totalCount.value || 1))
const estimatedMaxTotal = computed(() => estimatedMax.value * Math.max(1, totalCount.value || 1))

const luckyTiers = computed(() => {
  if (!selectedRule.value) return []
  const weighted = selectedRule.value.luckyTiers
    .filter((tier) => tier.kind === "gold_multiplier" && tier.weight > 0)
    .map((tier) => ({
      ...tier,
      adjustedWeight: tier.weight * (1 + Math.min(0.6, Math.max(0, alchemyLuck.value * 1.8 * Math.max(0, tier.value - 1))))
    }))
  const total = weighted.reduce((sum, tier) => sum + tier.adjustedWeight, 0)
  return weighted.map((tier) => ({
    id: tier.id,
    value: tier.value,
    chance: total > 0 ? successChance.value * luckyTriggerChance.value * tier.adjustedWeight / total : 0,
    name: t(`alchemy.transmute.tier.${tier.id}`, tier.id)
  }))
})

const canSubmit = computed(() => !!selectedEntry.value && totalCount.value >= 1)
const transmuteDurationParts = computed(() => selectedEntry.value && selectedRule.value
  ? computeTransmuteDurationParts(selectedEntry.value.itemId, selectedEntry.value.unitValue, selectedRule.value)
  : { base: 0, rarityAdd: 0, valueMultiplier: 1, final: 0 }
)
const transmuteDurationSec = computed(() => transmuteDurationParts.value.final)
const submitArgs = computed(() => selectedEntry.value
  ? {
      itemId: selectedEntry.value.itemId,
      instanceId: selectedEntry.value.instanceId,
    }
  : {})
const transmuteDurationBreakdown = computed(() => selectedEntry.value
  ? [
      { label: t("alchemy.transmute.baseDuration"), value: formatSeconds(transmuteDurationParts.value.base) },
      { label: t("alchemy.transmute.rarityDurationAdd"), value: formatSeconds(transmuteDurationParts.value.rarityAdd) },
      { label: t("alchemy.transmute.valueDurationMultiplier"), value: `x${formatNumber(transmuteDurationParts.value.valueMultiplier, 2)}` },
      { label: t("alchemy.transmute.finalSingleDuration"), value: formatSeconds(transmuteDurationSec.value) },
      { label: t("alchemy.transmute.countLabel"), value: `x${totalCount.value}` },
    ]
  : [])
const transmuteExpBreakdown = computed(() => selectedRule.value
  ? [{ label: t("alchemy.transmute.singleExp"), value: formatNumber(selectedRule.value.alchemyExp, 2) }, { label: t("alchemy.transmute.countLabel"), value: `x${totalCount.value}` }]
  : [])

watch(selectedInstanceId, () => {
  totalCount.value = 1
})

const normalizedPreferredInstanceId = computed(() => {
  const value = Math.trunc(Number(props.preferredInstanceId || 0))
  return Number.isFinite(value) && value > 0 ? value : null
})

watch(
  normalizedPreferredInstanceId,
  (instanceId) => {
    if (!instanceId || !transmutableInstanceIds.value.includes(instanceId)) return
    selectedInstanceId.value = instanceId
  },
  { immediate: true }
)

watch(transmutableInstanceIds, (ids) => {
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

function formatSeconds(value: number): string {
  return formatDurationI18n(Math.max(0, Number(value) || 0), t)
}

function formatNumber(value: number, digits = 0): string {
  if (!Number.isFinite(value)) return "0"
  return value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1")
}

function computeTransmuteDurationParts(itemId: string, unitValue: number, rule: AlchemyTransmuteRule): { base: number; rarityAdd: number; valueMultiplier: number; final: number } {
  const def = getItemDefinition(itemId)
  const rarityIndex = Math.max(0, getRarityOrder(def?.rarity || ""))
  const rarityAdds = rule.rarityDurationAddSecByRarityIndex || []
  const rarityAdd = rarityAdds.length
    ? rarityAdds[Math.min(rarityIndex, rarityAdds.length - 1)] || 0
    : 0
  const base = rule.durationSec || 8
  const valueMultiplier = 1 + Math.min(0.9, Math.sqrt(Math.max(0, unitValue || 0)) / 180)
  return {
    base,
    rarityAdd,
    valueMultiplier,
    final: Math.max(1, (base + rarityAdd) * valueMultiplier)
  }
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

.alchemy-transmute-target {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 14px;
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--el-color-primary-light-9) 58%, transparent), transparent 64%),
    color-mix(in srgb, var(--app-surface) 96%, var(--app-surface-overlay));
}

.alchemy-transmute-target__item {
  min-width: 92px;
}

.alchemy-transmute-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(128px, 1fr));
  gap: 8px;
  min-width: 0;
}

.alchemy-transmute-summary__item {
  display: grid;
  align-content: center;
  gap: 4px;
  min-height: 72px;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface) 88%, white);
}

.alchemy-transmute-summary__item span {
  font-size: 12px;
  color: var(--app-muted);
}

.alchemy-transmute-summary__item strong {
  font-size: 20px;
  line-height: 1.1;
  color: var(--el-color-primary);
}

.alchemy-transmute-summary__item--gold {
  background: color-mix(in srgb, var(--el-color-primary-light-9) 48%, var(--app-surface));
}

.alchemy-transmute-summary__label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.alchemy-transmute-summary__help {
  display: inline-grid;
  place-items: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--app-muted);
  cursor: help;
}

.alchemy-transmute-summary__help:hover,
.alchemy-transmute-summary__help:focus-visible {
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary-light-9) 68%, transparent);
  outline: none;
}

.alchemy-transmute-summary__gold {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.alchemy-popover-rows {
  display: grid;
  gap: 6px;
  font-size: 12px;
}

.alchemy-popover-rows > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.alchemy-outcome-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 8px;
}

.alchemy-outcome {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
}

@media (max-width: 900px) {
  .alchemy-transmute-target {
    grid-template-columns: 1fr;
  }

  .alchemy-transmute-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .alchemy-transmute-summary {
    grid-template-columns: 1fr;
  }
}

.alchemy-outcome__meta {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.alchemy-outcome__coin {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--el-color-warning, #e6a23c) 45%, transparent);
  background: color-mix(in srgb, var(--el-color-warning, #e6a23c) 14%, transparent);
  color: var(--el-color-warning, #e6a23c);
  font-weight: 800;
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
  color: var(--el-color-primary);
}
</style>
