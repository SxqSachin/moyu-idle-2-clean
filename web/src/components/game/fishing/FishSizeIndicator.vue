<template>
  <el-tooltip v-if="comparison" :content="tooltip" placement="top">
    <span
      class="fish-size-indicator"
      :class="`is-tier-${comparison.arrowCount}`"
      :aria-label="tooltip"
      role="img"
      :data-fish-size-metric="metric"
      :data-arrow-count="comparison.arrowCount"
    >
      <Icon v-for="index in comparison.arrowCount" :key="index" icon="lucide:arrow-up" aria-hidden="true" />
    </span>
  </el-tooltip>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import {
  compareFishSize,
  type FishSizeMetric,
  type FishSizeReferenceSpecies
} from "@services/fish-size-comparison"

const props = defineProps<{
  metric: FishSizeMetric
  value: number
  species?: FishSizeReferenceSpecies | null
}>()

const { t } = useI18n()
const comparison = computed(() => compareFishSize(props.metric, props.value, props.species))
const metricLabel = computed(() => t(`game.fishing.${props.metric}`))
const standardLabel = computed(() => {
  const value = comparison.value?.standardValue || 0
  if (props.metric === "length") return `${value.toFixed(1)} cm`
  return `${value.toFixed(value < 1 ? 3 : value < 10 ? 2 : 1)} kg`
})
const tooltip = computed(() => comparison.value
  ? t("game.fishing.sizeAboveStandard", {
      metric: metricLabel.value,
      percent: comparison.value.excessPercent,
      standard: standardLabel.value
    })
  : "")
</script>

<style scoped>
.fish-size-indicator {
  display: inline-flex;
  align-items: center;
  margin-inline-start: 2px;
  color: #2876c8;
  line-height: 1;
  vertical-align: .08em;
}
.fish-size-indicator svg {
  width: 10px;
  height: 10px;
  stroke-width: 2.8;
}
.fish-size-indicator svg + svg { margin-inline-start: -2px; }
</style>
