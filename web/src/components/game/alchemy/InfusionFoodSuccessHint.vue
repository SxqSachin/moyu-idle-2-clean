<template>
  <span v-if="failureDelta" class="text-xs" :class="failureDelta < 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
    {{ t(failureDelta < 0 ? 'game.fishCuisine.controls.successBonus' : 'game.fishCuisine.controls.successPenalty', { percent: `${(Math.abs(failureDelta) * 100).toFixed(2)}%` }) }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
defineProps<{ failureDelta?: number }>()
const { t } = useI18n()
</script>
