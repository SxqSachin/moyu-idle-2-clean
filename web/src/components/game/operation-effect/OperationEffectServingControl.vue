<template>
  <section class="food-serving">
    <div class="food-serving__heading">
      <label for="food-serving-quantity">{{ t('game.fishCuisine.serving.quantity') }}</label>
      <span>{{ t('game.fishCuisine.serving.stock', { count: stock }) }}</span>
    </div>
    <div class="food-serving__controls">
      <el-input-number id="food-serving-quantity" :model-value="modelValue" :min="1" :max="Math.max(1, preview.maxQuantity)" :precision="0" :step="1"
        :disabled="disabled || !ready || preview.maxQuantity < 1" @update:model-value="emit('update:modelValue', Number($event || 1))" />
      <el-button :disabled="disabled || !ready || preview.maxQuantity < 1" @click="emit('update:modelValue', preview.maxQuantity)">
        {{ t('game.fishCuisine.serving.fill') }}
      </el-button>
    </div>
    <p v-if="!ready || !preview.available" class="food-serving__notice">{{ t('game.fishCuisine.context.loading') }}</p>
    <p v-else-if="preview.conflict" class="food-serving__notice">{{ t('game.fishCuisine.serving.conflict') }}</p>
    <template v-else>
      <dl class="food-serving__totals">
        <div><dt>{{ t('game.fishCuisine.serving.remaining') }}</dt><dd>{{ format(preview.remaining) }}</dd></div>
        <div><dt>{{ t('game.fishCuisine.serving.added') }}</dt><dd>+{{ format(preview.added) }}</dd></div>
        <div class="food-serving__total"><dt>{{ t('game.fishCuisine.serving.total') }}</dt><dd>{{ format(preview.total) }}</dd></div>
      </dl>
      <div class="food-serving__limit">{{ t('game.fishCuisine.serving.limit', { value: format(preview.cap) }) }}</div>
      <p v-if="preview.maxQuantity < 1" class="food-serving__notice">{{ t('game.fishCuisine.serving.noRoom') }}</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { previewOperationEffectServings } from '@services/operation-effect-servings'
import { formatDurationI18n } from '@/utils/duration-format'

const props = defineProps<{
  modelValue: number
  preview: ReturnType<typeof previewOperationEffectServings>
  stock: number
  ready: boolean
  disabled?: boolean
}>()
const emit = defineEmits<{ (e: 'update:modelValue', value: number): void }>()
const { t } = useI18n()
function format(value: number) {
  return props.preview.duration ? formatDurationI18n(value, t) : t('game.fishCuisine.serving.uses', { count: value })
}
</script>

<style scoped>
.food-serving { border-top: 1px solid var(--el-border-color); padding-top: 16px; min-width: 0; }
.food-serving__heading, .food-serving__controls, .food-serving__totals > div { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.food-serving__heading { margin-bottom: 8px; }
.food-serving__heading span, .food-serving__limit { color: var(--el-text-color-secondary); font-size: 12px; }
.food-serving__controls :deep(.el-input-number) { flex: 1; min-width: 0; }
.food-serving__totals { margin: 16px 0 0; font-variant-numeric: tabular-nums; }
.food-serving__totals > div { padding: 4px 0; align-items: baseline; }
.food-serving__totals dt { color: var(--el-text-color-secondary); }
.food-serving__totals dd { margin: 0; text-align: right; overflow-wrap: anywhere; }
.food-serving__totals .food-serving__total { border-top: 1px solid var(--el-border-color-lighter); margin-top: 4px; padding-top: 8px; font-weight: 600; }
.food-serving__total dd { color: var(--el-color-primary); }
.food-serving__limit { text-align: right; margin-top: 4px; }
.food-serving__notice { color: var(--el-color-warning); font-size: 12px; margin: 8px 0 0; }
</style>
