<template>
  <el-popover
    v-if="hasTooltipContent"
    placement="top"
    :trigger="popoverTrigger"
    :width="320"
    :show-after="380"
    popper-class="attribute-stat-popover"
  >
    <template #reference>
      <div :class="cardClasses" role="group" :data-tutorial="tutorialHook" tabindex="0">
        <div class="attribute-stat-card__main">
          <span class="attribute-stat-card__icon">
            <AttributeIcon :attr-key="row.key" :alt="row.name" :size="emphasis === 'primary' ? 20 : 18" />
          </span>
          <span class="attribute-stat-card__label">
            <span class="attribute-stat-card__name">{{ row.name }}</span>
            <span v-if="isLevelable && !displayLevelAsValue" class="attribute-stat-card__level">Lv.{{ row.level }}</span>
          </span>
          <span class="attribute-stat-card__value">
            {{ displayValueText }}<span v-if="displayValueSuffix" class="attribute-stat-card__suffix">{{ displayValueSuffix }}</span>
          </span>
        </div>
        <div v-if="shouldShowProgress" class="attribute-stat-card__progress-row">
          <span class="attribute-stat-card__track">
            <span v-if="isLevelable" class="attribute-stat-card__bar" :style="{ width: progressWidth }" />
          </span>
          <span class="attribute-stat-card__progress">{{ isLevelable ? progressSummary : "" }}</span>
        </div>
      </div>
    </template>
    <div class="attribute-tooltip-content">
      <div v-if="row.desc">
        <div class="attribute-tooltip-title">{{ t("game.attrTooltipDescTitle") }}</div>
        <div class="attribute-tooltip-muted">{{ row.desc }}</div>
      </div>
      <div v-if="row.compositionLines.length > 0">
        <div class="attribute-tooltip-title">{{ t("game.attrTooltipCompositionTitle") }}</div>
        <div>{{ row.compositionLines.join("\n") }}</div>
      </div>
      <div v-if="isLevelable" class="attribute-tooltip-progress">
        <span>{{ t("game.attrCurrentExpLabel", { exp: currentExpText }) }}</span>
        <span>{{ t("game.attrNextExpLabel", { exp: nextExpText }) }}</span>
      </div>
      <div v-if="row.impactLines.length > 0" class="flex flex-col gap-1">
        <div class="attribute-tooltip-title">{{ t("game.attrTooltipImpactTitle") }}</div>
        <div class="">
          <div>{{ row.impactIntro || t("game.attrPerPointImpactTitle") }}</div>
          <div v-for="str in row.impactLines" :key="str">
            - {{ str }}
          </div>
        </div>
      </div>
    </div>
  </el-popover>

  <div v-else :class="cardClasses" role="group" :data-tutorial="tutorialHook">
    <div class="attribute-stat-card__main">
      <span class="attribute-stat-card__icon">
        <AttributeIcon :attr-key="row.key" :alt="row.name" :size="emphasis === 'primary' ? 20 : 18" />
      </span>
      <span class="attribute-stat-card__label">
        <span class="attribute-stat-card__name">{{ row.name }}</span>
        <span v-if="isLevelable && !displayLevelAsValue" class="attribute-stat-card__level">Lv.{{ row.level }}</span>
      </span>
      <span class="attribute-stat-card__value">
        {{ displayValueText }}<span v-if="displayValueSuffix" class="attribute-stat-card__suffix">{{ displayValueSuffix }}</span>
      </span>
    </div>
    <div v-if="shouldShowProgress" class="attribute-stat-card__progress-row">
      <span class="attribute-stat-card__track">
        <span v-if="isLevelable" class="attribute-stat-card__bar" :style="{ width: progressWidth }" />
      </span>
      <span class="attribute-stat-card__progress">{{ isLevelable ? progressSummary : "" }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import { formatDecimal } from "@utils/number-format"
import { useDevice } from "@/composables/game/useDevice"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"

const props = defineProps<{
  row: AttributeRow
  variant?: "default" | "split" | "hero" | "metric"
  emphasis?: "normal" | "primary"
  reserveProgressSpace?: boolean
  displayLevelAsValue?: boolean
  showProgress?: boolean
}>()

const { t } = useI18n()
const { supportsHover } = useDevice()

const isLevelable = computed(() => props.row.growthType === "levelable")
const reserveProgressSpace = computed(() => props.reserveProgressSpace === true)
const shouldShowProgress = computed(() => props.showProgress !== false && (isLevelable.value || reserveProgressSpace.value))
const displayLevelAsValue = computed(() => props.displayLevelAsValue === true && isLevelable.value)
const displayValueText = computed(() => displayLevelAsValue.value ? `Lv.${props.row.level}` : props.row.value)
const displayValueSuffix = computed(() => displayLevelAsValue.value ? "" : props.row.valueSuffix)
const isSplitVariant = computed(() => props.variant === "split")
const progressSummary = computed(() => `${props.row.progress}%`)
const progressWidth = computed(() => `${Math.max(0, Math.min(100, Number(props.row.progress || 0)))}%`)
const currentExpText = computed(() => formatDecimal(Number(props.row.exp || 0), 2))
const nextExpText = computed(() => formatDecimal(Number(props.row.nextExp || 0), 2))
const hasTooltipContent = computed(
  () => Boolean(props.row.desc) || props.row.compositionLines.length > 0 || props.row.impactLines.length > 0
)
const popoverTrigger = computed(() => supportsHover.value ? "hover" : "click")
const tutorialHook = computed(() => props.emphasis === "primary" ? "attribute-primary-card" : "attribute-stat-card")
const cardClasses = computed(() => [
  "attribute-stat-card",
  `attribute-stat-card--${props.variant || "default"}`,
  {
    "attribute-stat-card--split": isSplitVariant.value,
    "attribute-stat-card--primary": props.emphasis === "primary",
    "attribute-stat-card--levelable": isLevelable.value,
    "attribute-stat-card--static": !isLevelable.value,
    "cursor-help": hasTooltipContent.value
  }
])
</script>

<style scoped>
.attribute-stat-card {
  min-height: 56px;
  display: grid;
  align-content: center;
  gap: 6px;
  padding: 9px 10px;
  border: 1px solid color-mix(in srgb, var(--app-border) 82%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface-elevated) 72%, var(--app-surface));
  transition: border-color 0.14s ease, background 0.14s ease;
}

.attribute-stat-card--hero {
  min-height: 76px;
  padding: 12px;
  background: color-mix(in srgb, var(--app-surface-elevated) 80%, var(--app-surface));
}

.attribute-stat-card--metric {
  min-height: 38px;
  padding: 7px 0;
  border-width: 0 0 1px;
  border-color: color-mix(in srgb, var(--app-border) 62%, transparent);
  border-radius: 0;
  background: transparent;
}

.attribute-stat-card:hover {
  border-color: color-mix(in srgb, var(--app-primary) 24%, var(--app-border));
  background: color-mix(in srgb, var(--app-surface-elevated) 86%, var(--app-surface));
}

.attribute-stat-card--metric:hover {
  border-color: color-mix(in srgb, var(--app-border) 62%, transparent);
  background: transparent;
}

.attribute-stat-card--primary {
  min-height: 64px;
  padding: 11px 12px;
}

.attribute-stat-card--hero.attribute-stat-card--primary {
  min-height: 76px;
  padding: 12px;
}

.attribute-stat-card--static {
  min-height: 50px;
}

.attribute-stat-card--metric.attribute-stat-card--static {
  min-height: 38px;
}

.attribute-stat-card__main {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) minmax(64px, auto);
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.attribute-stat-card--metric .attribute-stat-card__main {
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 7px;
}

.attribute-stat-card__icon {
  display: inline-flex;
  width: 28px;
  height: 28px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: var(--app-muted);
  background: color-mix(in srgb, var(--app-border) 42%, transparent);
}

.attribute-stat-card--metric .attribute-stat-card__icon {
  width: 22px;
  height: 22px;
  border-radius: 5px;
}

.attribute-stat-card__label {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.attribute-stat-card__name {
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  font-weight: 600;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attribute-stat-card--hero .attribute-stat-card__name,
.attribute-stat-card--metric .attribute-stat-card__name {
  white-space: normal;
}

.attribute-stat-card--metric .attribute-stat-card__name {
  font-size: 12px;
  line-height: 16px;
}

.attribute-stat-card__level {
  flex: none;
  border-radius: 999px;
  padding: 1px 6px;
  color: var(--app-muted);
  background: color-mix(in srgb, var(--app-border) 48%, transparent);
  font-size: 10px;
  font-weight: 700;
  line-height: 14px;
}

.attribute-stat-card--metric .attribute-stat-card__level {
  padding: 0 5px;
  font-size: 9px;
  line-height: 13px;
}

.attribute-stat-card__value {
  min-width: 0;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 700;
  line-height: 18px;
  text-align: right;
  overflow-wrap: anywhere;
}

.attribute-stat-card--metric .attribute-stat-card__value {
  font-size: 13px;
  line-height: 16px;
}

.attribute-stat-card--primary .attribute-stat-card__value {
  font-size: 15px;
}

.attribute-stat-card__suffix {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 500;
}

.attribute-stat-card__progress-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
}

.attribute-stat-card--metric .attribute-stat-card__progress-row {
  display: none;
}

.attribute-stat-card__track {
  height: 4px;
  overflow: hidden;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-border) 70%, transparent);
}

.attribute-stat-card__bar {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--app-primary);
}

.attribute-stat-card__progress {
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 600;
  line-height: 14px;
}

.attribute-tooltip-content {
  display: grid;
  max-height: min(420px, calc(100vh - 120px));
  gap: 10px;
  overflow: auto;
  padding-right: 2px;
  color: var(--el-text-color-regular);
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.attribute-tooltip-title {
  color: var(--app-text);
  font-weight: 600;
}

.attribute-tooltip-progress {
  display: grid;
  gap: 2px;
  color: var(--app-muted);
  font-size: 11px;
}

.attribute-tooltip-muted {
  color: var(--app-muted);
}

:global(.attribute-stat-popover) {
  max-width: calc(100vw - 32px);
}
</style>
