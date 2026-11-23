<script setup lang="ts">
import { computed } from 'vue';
import { asNumber, isRecord } from '../model/catalog';
const props = defineProps<{ modelValue: Record<string, unknown>; kind: string }>();
const emit = defineEmits<{ (event: 'update:modelValue', value: Record<string, unknown>): void }>();
const field = computed(() => props.kind === 'grilled_fish' ? 'charges' : 'durationSeconds');
const names: Record<string, string> = { shimmering: '闪光', prismatic: '棱彩', rainbow: '彩虹' };
function value(id: string) {
  const row = props.modelValue[id];
  return isRecord(row) ? asNumber(row[field.value]) : 0;
}
function update(id: string, event: Event) {
  const amount = Math.max(0, Math.trunc(Number((event.target as HTMLInputElement).value) || 0));
  const next = { ...props.modelValue };
  if (amount) next[id] = { [field.value]: amount }; else delete next[id];
  emit('update:modelValue', next);
}
</script>

<template>
  <div class="lifetime-bonuses">
    <label v-for="(name, id) in names" :key="id">
      <span>{{ name }}{{ field === 'charges' ? '额外次数' : '额外秒数' }}</span>
      <input type="number" min="0" :max="field === 'charges' ? 1000 : 86400" step="1" :value="value(id)" @input="update(id, $event)" />
    </label>
  </div>
</template>

<style scoped>
.lifetime-bonuses { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 12px; }
label { display: grid; gap: 6px; font-size: 12px; }
input { min-width: 0; width: 100%; box-sizing: border-box; font-variant-numeric: tabular-nums; }
</style>
