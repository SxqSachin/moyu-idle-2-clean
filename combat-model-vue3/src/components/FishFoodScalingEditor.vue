<script setup lang="ts">
import { computed, ref } from 'vue';
import { asNumber, isRecord } from '../model/catalog';
const props = defineProps<{ modelValue: Record<string, unknown>; lifetime?: boolean }>();
const emit = defineEmits<{ (event: 'update:modelValue', value: Record<string, unknown>): void }>();
const names: Record<string, string> = { charges: '使用次数', weightBonusRate: '高稀有度权重', extraMaterialQuantity: '灌注容量', quantityLimitBonusRate: '容量突破比例', successRatePenalty: '成功率代价', positiveFloorReductionRate: '下限代价', positiveCeilingBonusRate: '精度上限', rate: '钓鱼效果强度', bonusRate: '概率加成' };
const selected = ref('rate');
const rows = computed(() => Object.entries(props.modelValue).filter(([, value]) => isRecord(value)).map(([key, value]) => ({ key, scale: value as Record<string, unknown> })));
function set(key: string, field: string, value: number | boolean | string) {
  const scale = { ...(isRecord(props.modelValue[key]) ? props.modelValue[key] : {}), [field]: value };
  if (field === 'curve') {
    if (value === 'size_growth' || value === 'bounded_growth') Object.assign(scale, { length: 0.3, weight: 0.7, traits: 0.1, integer: key === 'extraMaterialQuantity', inverse: false, maxBeforeMutation: asNumber(scale.max) });
    else { delete scale.curve; delete scale.maxBeforeMutation; }
  }
  emit('update:modelValue', { ...props.modelValue, [key]: scale });
}
function add() {
  const key = props.lifetime ? 'charges' : selected.value;
  if (props.lifetime) { emit('update:modelValue', { charges: { curve: 'bounded_growth', length: .3, weight: .7, traits: .1, min: 20, maxBeforeMutation: 40, max: 80 } }); return; }
  if (props.modelValue[key]) return;
  emit('update:modelValue', { ...props.modelValue, [key]: { length: 0.4, weight: 0.1, traits: 0.2, min: key === 'extraMaterialQuantity' ? 1 : 0.01, max: key === 'extraMaterialQuantity' ? 80 : 0.1, integer: key === 'extraMaterialQuantity', inverse: ['positiveFloorReductionRate', 'successRatePenalty'].includes(key) } });
}
function remove(key: string) { const next = { ...props.modelValue }; delete next[key]; emit('update:modelValue', next); }
</script>

<template>
  <div class="fish-scaling">
    <div class="fish-scaling__add"><select v-if="!lifetime" v-model="selected"><option v-for="(label, key) in names" :key="key" :value="key">{{ label }}</option></select><button type="button" :disabled="lifetime && rows.length > 0" @click="add">{{ lifetime ? '添加次数成长' : '添加参数' }}</button></div>
    <section v-for="row in rows" :key="row.key">
      <header><strong>{{ names[row.key] || row.key }}</strong><button type="button" :aria-label="'删除' + row.key" title="删除参数" @click="remove(row.key)">×</button></header>
      <div class="fish-scaling__fields">
        <label v-if="!lifetime">成长曲线<select :value="row.scale.curve || ''" @change="set(row.key, 'curve', ($event.target as HTMLSelectElement).value)"><option value="">线性权重</option><option value="size_growth">体型平方成长</option><option value="bounded_growth">尺寸特质平方成长</option></select></label>
        <label v-if="['size_growth','bounded_growth'].includes(String(row.scale.curve))">变异前上限<input type="number" :min="row.key === 'extraMaterialQuantity' ? 1 : 0.0001" :step="row.key === 'extraMaterialQuantity' ? 1 : 0.01" :value="asNumber(row.scale.maxBeforeMutation)" @input="set(row.key, 'maxBeforeMutation', Number(($event.target as HTMLInputElement).value))" /></label>
        <label v-for="field in ['length','weight','traits','min','max']" :key="field">{{ ({length:'长度权重',weight:'体重权重',traits:'特质权重',min:'下限',max:'上限'} as Record<string,string>)[field] }}<input type="number" min="0" :step="field === 'min' || field === 'max' ? 0.01 : 0.05" :value="asNumber(row.scale[field])" @input="set(row.key, field, Number(($event.target as HTMLInputElement).value))" /></label>
        <label v-if="!lifetime"><input type="checkbox" :checked="Boolean(row.scale.inverse)" @change="set(row.key,'inverse',($event.target as HTMLInputElement).checked)" />反比减轻代价</label>
        <label v-if="!lifetime"><input type="checkbox" :checked="Boolean(row.scale.integer)" @change="set(row.key,'integer',($event.target as HTMLInputElement).checked)" />整数数量</label>
      </div>
    </section>
  </div>
</template>

<style scoped>
.fish-scaling section { border-top: 1px solid var(--border, #d5d8dc); padding: 10px 0; }
.fish-scaling header, .fish-scaling__add { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
.fish-scaling__fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 8px; }
.fish-scaling label { display: flex; flex-wrap: wrap; gap: 4px; font-size: 12px; }
.fish-scaling input[type=number] { min-width: 0; width: 100%; }
</style>
