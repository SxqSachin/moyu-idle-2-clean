<script setup lang="ts">
import { computed } from 'vue';
import { asNumber } from '../model/catalog';
const props = defineProps<{ modelValue: Record<string, unknown>; kind: string }>();
const emit = defineEmits<{ (event: 'update:modelValue', value: Record<string, unknown>): void }>();
const bulk = computed(() => props.kind === 'alchemy.infusion_bulk_capacity_floor_tradeoff');
const tier = computed(() => asNumber(props.modelValue.materialTier));
const fields = computed(() => [
  { key: 'extraMaterialQuantity', label: '固定容量', min: 1, max: 1000000, step: 1 },
  { key: 'quantityLimitBonusRate', label: '容量比例', min: 0, max: tier.value === 1 ? .20 : tier.value === 2 ? .15 : .17, step: .01 },
  { key: 'successRatePenalty', label: '成功率代价', min: 0, max: .20, step: .01 },
  ...(bulk.value ? [{ key: 'positiveFloorReductionRate', label: '正向下限代价', min: .0001, max: .95, step: .01 }] : [])
]);
function set(key: string, value: number) {
  const next = { ...props.modelValue, [key]: value };
  if (key === 'materialTier' && value === 0) delete next.materialTier;
  emit('update:modelValue', next);
}
</script>

<template>
  <div class="infusion-capacity-fields">
    <label v-if="bulk">素材范围
      <select :value="tier" @change="set('materialTier', Number(($event.target as HTMLSelectElement).value))">
        <option :value="1">普通、罕见</option>
        <option :value="2">稀有、史诗</option>
        <option :value="0">历史：普通至史诗</option>
      </select>
    </label>
    <label v-for="field in fields" :key="field.key">{{ field.label }}
      <input type="number" :min="field.min" :max="field.max" :step="field.step" :value="asNumber(modelValue[field.key])" @input="set(field.key, Number(($event.target as HTMLInputElement).value))" />
    </label>
  </div>
</template>

<style scoped>
.infusion-capacity-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 8px; }
label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; }
input, select { width: 100%; min-width: 0; }
</style>
