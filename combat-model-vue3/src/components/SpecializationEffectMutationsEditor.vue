<script setup lang="ts">
import { computed } from 'vue';
import { asArray, clone, isRecord, type DataRow } from '../model/catalog';
import DslObjectEditor from './DslObjectEditor.vue';
import { createMutationOpsSchema, type SpecializationDslRefs } from './specializationDslSchemas';

const props = withDefaults(defineProps<{
  modelValue: unknown[];
  attributeIds?: string[];
  skillIds?: string[];
  skillCategories?: string[];
  weaponClasses?: string[];
  buffIds?: string[];
  effectIds?: string[];
  skillEffectKinds?: string[];
  lockIds?: boolean;
}>(), { attributeIds: () => [], skillIds: () => [], skillCategories: () => [], weaponClasses: () => [], buffIds: () => [], effectIds: () => [], skillEffectKinds: () => [], lockIds: false });
const emit = defineEmits<{ 'update:modelValue': [value: DataRow[]] }>();

const refs = computed<SpecializationDslRefs>(() => ({
  attributeIds: props.attributeIds,
  skillIds: props.skillIds,
  skillCategories: props.skillCategories,
  weaponClasses: props.weaponClasses,
  buffIds: props.buffIds,
  buffKinds: [],
  effectIds: props.effectIds,
  skillEffectKinds: props.skillEffectKinds
}));
const opsSchema = computed(() => createMutationOpsSchema(refs.value, { lockStableIds: props.lockIds }));

function rows(): DataRow[] { return asArray(props.modelValue).filter(isRecord).map((entry) => clone(entry)); }
function update(index: number, key: string, value: unknown) { const next = rows(); next[index][key] = value; emit('update:modelValue', next); }
function removeOptional(index: number, key: string) { const next = rows(); delete next[index][key]; emit('update:modelValue', next); }
function remove(index: number) { const next = rows(); next.splice(index, 1); emit('update:modelValue', next); }
function add() { emit('update:modelValue', [...rows(), { id: `mutation_${Date.now()}`, priority: 0, scope: 'skill', effectSelector: { mode: 'all', includeGenerated: false }, ops: { amountBaseAddDelta: 1 } }]); }
function selector(row: DataRow): DataRow { return isRecord(row.effectSelector) ? row.effectSelector : { mode: 'all' }; }
function updateSelector(index: number, key: string, value: unknown) { update(index, 'effectSelector', { ...selector(rows()[index]), [key]: value }); }
function updateSelectorMode(index: number, mode: string) {
  const includeGenerated = selector(rows()[index]).includeGenerated === true;
  if (mode === 'kind') update(index, 'effectSelector', { mode, kinds: [props.skillEffectKinds[0] ?? 'damage'], includeGenerated });
  else if (mode === 'effect_id') update(index, 'effectSelector', { mode, effectIds: props.effectIds[0] ? [props.effectIds[0]] : [], includeGenerated });
  else update(index, 'effectSelector', { mode: 'all', includeGenerated });
}
function updateList(index: number, key: string, event: Event, target: 'mutation' | 'selector') {
  const values = Array.from((event.target as HTMLSelectElement).selectedOptions).map((option) => option.value);
  if (target === 'selector') updateSelector(index, key, values);
  else if (values.length) update(index, key, values);
  else removeOptional(index, key);
}
function stringList(value: unknown): string[] { return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : []; }
</script>

<template>
  <div class="mutation-editor">
    <div class="editor-head"><strong>效果变异</strong><button type="button" @click="add">+ 变异</button></div>
    <section v-for="(mutation, index) in rows()" :key="`${mutation.id}:${index}`" class="mutation-row">
      <header><strong>{{ mutation.id || `Mutation ${index + 1}` }}</strong><button type="button" title="删除变异" @click="remove(index)">×</button></header>
      <label>稳定 ID<input :readonly="lockIds" :value="mutation.id" @change="update(index, 'id', ($event.target as HTMLInputElement).value)" /></label>
      <label>优先级<input type="number" step="1" :value="mutation.priority ?? 0" @change="update(index, 'priority', Number(($event.target as HTMLInputElement).value))" /></label>
      <div class="pair">
        <label>作用域<select :value="mutation.scope" @change="update(index, 'scope', ($event.target as HTMLSelectElement).value)"><option value="skill">技能</option><option value="basic_attack">普攻</option><option value="both">技能与普攻</option></select></label>
        <label>效果选择<select :value="selector(mutation).mode" @change="updateSelectorMode(index, ($event.target as HTMLSelectElement).value)"><option value="all">全部稳定效果</option><option value="kind">效果类别</option><option value="effect_id">稳定效果 ID</option></select></label>
      </div>
      <label v-if="selector(mutation).mode === 'kind'">效果类别<select class="multi-select" multiple :value="stringList(selector(mutation).kinds)" @change="updateList(index, 'kinds', $event, 'selector')"><option v-for="kind in skillEffectKinds" :key="kind" :value="kind">{{ kind }}</option></select></label>
      <label v-if="selector(mutation).mode === 'effect_id'">效果 ID<select class="multi-select" multiple :value="stringList(selector(mutation).effectIds)" @change="updateList(index, 'effectIds', $event, 'selector')"><option v-for="id in effectIds" :key="id" :value="id">{{ id }}</option></select></label>
      <label class="check-row"><input type="checkbox" :checked="selector(mutation).includeGenerated === true" @change="updateSelector(index, 'includeGenerated', ($event.target as HTMLInputElement).checked)" />包含其他变异生成的效果</label>
      <label>限定技能<select class="multi-select" multiple :value="stringList(mutation.skillIds)" @change="updateList(index, 'skillIds', $event, 'mutation')"><option v-for="id in skillIds" :key="id" :value="id">{{ id }}</option></select><small>不选择时作用于作用域内全部技能。</small></label>
      <label>限定技能类别<select class="multi-select" multiple :value="stringList(mutation.skillCategories)" @change="updateList(index, 'skillCategories', $event, 'mutation')"><option v-for="id in skillCategories" :key="id" :value="id">{{ id }}</option></select></label>
      <label>限定武器类别<select class="multi-select" multiple :value="stringList(mutation.weaponClasses)" @change="updateList(index, 'weaponClasses', $event, 'mutation')"><option v-for="id in weaponClasses" :key="id" :value="id">{{ id }}</option></select></label>
      <label>限定伤害类型<select class="multi-select" multiple :value="stringList(mutation.damageTypes)" @change="updateList(index, 'damageTypes', $event, 'mutation')"><option value="physical">physical</option><option value="magical">magical</option><option value="true">true</option></select></label>
      <div class="ops"><strong>变异操作</strong><DslObjectEditor :model-value="isRecord(mutation.ops) ? mutation.ops : {}" :schema="opsSchema" @update:model-value="update(index, 'ops', $event)" /></div>
    </section>
  </div>
</template>

<style scoped>
.mutation-editor, .mutation-row, .ops { display: grid; gap: 8px; }
.editor-head, .mutation-row > header { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.mutation-row { padding: 9px 0; border-top: 1px solid #e2e7f0; }
.mutation-row > header button { background: #fff0f1; color: #a52838; }
.pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.check-row { display: flex; align-items: center; gap: 6px; }
.check-row input { width: auto; }
label { display: grid; gap: 4px; color: #526079; font-size: 10px; font-weight: 750; }
label small { color: #7c8799; font-size: 9px; font-weight: 500; }
input, select { min-width: 0; padding: 6px; font-size: 10px; }
.multi-select { min-height: 72px; }
button { min-height: 27px; padding: 0 8px; border-radius: 5px; font-size: 10px; }
.ops { padding: 7px; border: 1px solid #e1e7f0; border-radius: 6px; background: #f8faff; }
</style>
