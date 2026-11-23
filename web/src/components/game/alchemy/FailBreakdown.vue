<template>
  <div class="grid gap-1 text-xs">
    <div class="font-semibold">{{ t("alchemy.bench.failBreakdownTitle") }}</div>
    <div class="flex justify-between"><span>{{ t("alchemy.bench.failBase") }}</span><span>{{ formatPercent(breakdown.base) }}</span></div>
    <div v-if="breakdown.levelDelta !== 0" class="flex justify-between text-emerald-600 dark:text-emerald-400">
      <span>{{ t("alchemy.bench.failLevelDelta") }}</span><span>{{ formatSigned(breakdown.levelDelta) }}</span>
    </div>
    <div v-if="breakdown.quantityDelta !== 0" class="flex justify-between text-amber-600 dark:text-amber-400">
      <span>{{ t("alchemy.bench.failQuantityDelta") }}</span><span>{{ formatSigned(breakdown.quantityDelta) }}</span>
    </div>
    <div v-if="breakdown.craftSuccessDelta !== 0" class="flex justify-between text-emerald-600 dark:text-emerald-400">
      <span>{{ t("alchemy.bench.failCraftSuccessDelta") }}</span><span>{{ formatSigned(breakdown.craftSuccessDelta) }}</span>
    </div>
    <div
      v-if="breakdown.mediumDelta !== 0"
      class="flex justify-between"
      :class="breakdown.mediumDelta < 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'"
    >
      <span>{{ t("alchemy.bench.failMediumDelta") }}</span><span>{{ formatSigned(breakdown.mediumDelta) }}</span>
    </div>
    <div v-if="breakdown.successFloorClamp !== 0" class="flex justify-between text-emerald-600 dark:text-emerald-400">
      <span>{{ t("alchemy.bench.failFloorClamp") }}</span><span>{{ formatSigned(breakdown.successFloorClamp) }}</span>
    </div>
    <div v-if="breakdown.foodDelta" class="flex justify-between" :class="breakdown.foodDelta < 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
      <span>{{ t("game.fishCuisine.controls.failureAdjustment") }}</span><span>{{ formatSigned(breakdown.foodDelta) }}</span>
    </div>
    <div class="mt-1 pt-1 border-t border-[color:var(--app-border)] flex justify-between font-semibold">
      <span>{{ t("alchemy.bench.failFinal") }}</span><span>{{ formatPercent(breakdown.final) }}</span>
    </div>
    <div class="flex justify-between font-semibold" :class="tagClass(successPct)">
      <span>{{ t("alchemy.bench.successFinal") }}</span><span>{{ formatPercent(successPct) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import type { FailChanceBreakdown } from "@services/alchemy-data"

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

const props = defineProps<{
  breakdown: FailChanceBreakdown
  formatPercent: (value: number) => string
  formatSigned: (value: number) => string
  tagClass: (pct: number) => string
  t: TranslateFn
}>()

const t = props.t
const formatPercent = props.formatPercent
const formatSigned = props.formatSigned
const tagClass = props.tagClass

const successPct = computed(() => 1 - props.breakdown.final)
</script>
