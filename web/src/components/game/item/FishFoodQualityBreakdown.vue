<template>
  <dl class="fish-food-breakdown" :class="{ 'fish-food-breakdown--compact': compact }">
    <div class="fish-food-breakdown__scores">
      <span>{{ t('game.fishCuisine.lengthScore', { value: Math.round(quality.lengthScore * 100) }) }}</span>
      <span>{{ t('game.fishCuisine.weightScore', { value: Math.round(quality.weightScore * 100) }) }}</span>
      <span>{{ t('game.fishCuisine.traitScore', { value: quality.traitAverage.toFixed(2) }) }}</span>
    </div>
    <div v-for="(value, name) in quality.parameterMultipliers" :key="name" class="fish-food-breakdown__parameter">
      <dt>{{ t('game.fishCuisine.parameterNames.' + name) }}</dt>
      <dd>×{{ value.toFixed(4) }}</dd>
      <small v-if="scales?.[name]">{{ t('game.fishCuisine.weights', { length: Math.round(scales[name].length * 100), weight: Math.round(scales[name].weight * 100), traits: Math.round(scales[name].traits * 100) }) }}</small>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { FishFoodParameterScale } from '@services/fish-food'
defineProps<{
  quality: { lengthScore: number; weightScore: number; traitAverage: number; parameterMultipliers?: Record<string, number> }
  scales?: Record<string, FishFoodParameterScale>
  compact?: boolean
}>()
const { t } = useI18n()
</script>

<style scoped>
.fish-food-breakdown { margin: 12px 0 0; padding-top: 10px; border-top: 1px solid var(--app-border); font-size: 12px; line-height: 1.55; color: var(--app-text-secondary); }
.fish-food-breakdown--compact { margin: 0; padding: 0; border: 0; }
.fish-food-breakdown__scores { display: flex; flex-wrap: wrap; gap: 4px 12px; margin-bottom: 8px; }
.fish-food-breakdown__parameter { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0 8px; margin: 6px 0; }
.fish-food-breakdown__parameter dt { overflow-wrap: anywhere; }
.fish-food-breakdown__parameter dd { margin: 0; color: var(--app-primary); font-variant-numeric: tabular-nums; }
.fish-food-breakdown__parameter small { grid-column: 1 / -1; color: var(--app-muted); }
</style>
