<template>
  <div class="difficulty-selector" :class="{ 'difficulty-selector--compact': compact }">
    <el-select
      :model-value="modelValue"
      :placeholder="t('game.battleDifficultyPlaceholder')"
      :aria-label="t('game.battleDifficultyLevel')"
      class="difficulty-selector__select"
      :size="compact ? 'small' : 'default'"
      :popper-class="compact
        ? 'difficulty-selector-popper difficulty-selector-popper--compact'
        : 'difficulty-selector-popper'"
      @update:model-value="$emit('update:modelValue', $event)"
    >
      <el-option :label="t('game.battleDifficultyNone')" :value="0">
        <span class="app-muted">{{ t('game.battleDifficultyNone') }}</span>
      </el-option>
      <el-option v-for="dl in levels" :key="dl.level" :label="t(dl.labelKey)" :value="dl.level">
        <span class="difficulty-selector__option">
          <span>{{ t(dl.labelKey) }}</span>
          <span v-if="!compact" class="difficulty-selector__option-summary text-xs app-muted">
            {{ t("game.battleDifficultyEnemyMult") }}: {{ formatNum(dl.statMultiplier) }}x |
            {{ t("game.battleDifficultyExpBonus") }}: +{{ (dl.battleExpBonus * 100).toFixed(0) }}% |
            {{ t("game.battleDifficultyDropBonus") }}: +{{ (dl.battleDropRateBonus * 100).toFixed(0) }}%
          </span>
        </span>
      </el-option>
    </el-select>

    <div v-if="!compact && modelValue > 0 && detail" class="mt-2 text-xs app-muted border rounded p-2 space-y-1 bg-[var(--app-bg-light)]">
      <div>{{ t("game.battleDifficultyEnemyMult") }}: {{ formatNum(detail.statMultiplier) }}x</div>
      <div>{{ t("game.battleDifficultyExpBonus") }}: +{{ (detail.battleExpBonus * 100).toFixed(0) }}%</div>
      <div>{{ t("game.battleDifficultyDropBonus") }}: +{{ (detail.battleDropRateBonus * 100).toFixed(0) }}%</div>
      <div v-if="statBonuses.length > 0" class="mt-1 pt-1 border-t border-[var(--app-border)]">
        <div class="font-medium">{{ t("game.battleDifficultyAdditionalBonuses") }}:</div>
        <div v-for="sb in statBonuses" :key="sb.attrKey" class="ml-1">{{ toAttrLabel(sb.attrKey) }}: {{ formatBonus(sb.attrKey, sb.bonus) }}</div>
      </div>
      <div v-if="specialDrops.length > 0" class="mt-1 pt-1 border-t border-[var(--app-border)]">
        <div class="font-medium">{{ t("game.battleDifficultySpecialDrops") }}:</div>
        <div v-for="sd in specialDrops" :key="sd.itemId" class="ml-1">{{ sd.itemId }} x{{ sd.rangeMin }}-{{ sd.rangeMax }} ({{ Math.round(sd.percent * 100) }}%) Lv{{ sd.minLevel }}+</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from "vue"
import { useI18n } from "vue-i18n"
import { formatAttributeValueByKey } from "@services/attribute-display"
import {
  getCombatDifficultyLevel,
  listCombatDifficultyLevels,
  listWorldDifficultySpecialDrops,
  resolveWorldDifficultyStatBonuses,
  type WorldDifficultySettings
} from "@services/battle-difficulty"

const { t } = useI18n()

const props = withDefaults(defineProps<{
  modelValue: number
  worldDifficultySettings?: WorldDifficultySettings | null
  compact?: boolean
}>(), {
  worldDifficultySettings: null,
  compact: false
})

const emit = defineEmits<{
  (e: "update:modelValue", val: number): void
}>()

const levels = computed(() => listCombatDifficultyLevels(props.worldDifficultySettings))

const detail = computed(() => getCombatDifficultyLevel(props.modelValue, props.worldDifficultySettings))

const statBonuses = computed(() => {
  return resolveWorldDifficultyStatBonuses(props.worldDifficultySettings, props.modelValue)
    .filter((row) => row.bonus !== 0)
})

const specialDrops = computed(() => {
  return listWorldDifficultySpecialDrops(props.worldDifficultySettings, props.modelValue)
})

watch([() => props.modelValue, levels], ([modelValue, availableLevels]) => {
  if (modelValue <= 0 || availableLevels.some((row) => row.level === modelValue)) return
  emit("update:modelValue", 0)
}, { immediate: true })

function formatNum(v: number): string {
  return v.toFixed(2)
}

function toAttrLabel(key: string): string {
  const k = `attr.${key}.name`
  const label = t(k)
  return label === k ? key : label
}

function formatBonus(attrKey: string, value: number): string {
  return formatAttributeValueByKey(attrKey, value, true)
}
</script>

<style scoped>
.difficulty-selector {
  width: 100%;
}

.difficulty-selector--compact {
  width: 104px;
}

.difficulty-selector__select {
  width: 100%;
}

.difficulty-selector__option {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 8px;
}

.difficulty-selector__option-summary {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 640px) {
  .difficulty-selector__option-summary {
    display: none;
  }

  :global(.difficulty-selector-popper) {
    max-width: calc(100vw - 16px);
  }
}

:global(.difficulty-selector-popper--compact) {
  min-width: 104px !important;
}
</style>
