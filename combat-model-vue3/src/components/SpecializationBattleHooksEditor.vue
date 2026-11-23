<script setup lang="ts">
import { computed, inject, type ComputedRef } from 'vue';
import { asArray, asString, clone, isRecord, type DataRow } from '../model/catalog';
import {
  SPECIALIZATION_BATTLE_TRIGGERS,
  specializationConditionKindsForTrigger,
  specializationEventFilterKeysForTrigger,
  specializationHookEffectKindsForTrigger
} from '../model/specialization';
import DslObjectEditor from './DslObjectEditor.vue';
import {
  SPECIALIZATION_LIMITS_SCHEMA,
  createConditionVariants,
  createEventFilterSchema,
  createHookEffectVariants,
  type SpecializationDslRefs
} from './specializationDslSchemas';

const props = withDefaults(defineProps<{
  modelValue: unknown[];
  attributeIds?: string[];
  skillIds?: string[];
  skillCategories?: string[];
  weaponClasses?: string[];
  buffIds?: string[];
  buffKinds?: string[];
  lockIds?: boolean;
}>(), { attributeIds: () => [], skillIds: () => [], skillCategories: () => [], weaponClasses: () => [], buffIds: () => [], buffKinds: () => [], lockIds: false });
const emit = defineEmits<{ 'update:modelValue': [value: DataRow[]] }>();
const inheritedWeaponClasses = inject<ComputedRef<string[]>>('specializationWeaponClasses');

const refs = computed<SpecializationDslRefs>(() => ({
  attributeIds: props.attributeIds,
  skillIds: props.skillIds,
  skillCategories: props.skillCategories,
  weaponClasses: props.weaponClasses.length > 0 ? props.weaponClasses : inheritedWeaponClasses?.value ?? [],
  buffIds: props.buffIds,
  buffKinds: props.buffKinds,
  effectIds: [],
  skillEffectKinds: []
}));
function hooks(): DataRow[] {
  return asArray(props.modelValue).filter(isRecord).map((hook) => clone(hook));
}
function updateHook(index: number, key: string, value: unknown) {
  const next = hooks();
  next[index][key] = value;
  emit('update:modelValue', next);
}
function removeHookField(index: number, key: string) {
  const next = hooks();
  delete next[index][key];
  emit('update:modelValue', next);
}
function addHook() {
  const effect = createHookEffectVariants(refs.value, 'onBattleStart').heal;
  emit('update:modelValue', [...hooks(), { id: `hook_${Date.now()}`, trigger: 'onBattleStart', priority: 0, conditions: [], effects: [clone(effect.defaultValue)], dispatchHooks: false }]);
}
function removeHook(index: number) {
  const next = hooks();
  next.splice(index, 1);
  emit('update:modelValue', next);
}
function objectList(hook: DataRow, key: 'conditions' | 'effects'): DataRow[] {
  return asArray(hook[key]).filter(isRecord);
}
function updateObject(index: number, key: 'conditions' | 'effects', objectIndex: number, value: DataRow) {
  const next = objectList(hooks()[index], key).map((entry) => clone(entry));
  next[objectIndex] = value;
  updateHook(index, key, next);
}
function replaceObjectKind(index: number, key: 'conditions' | 'effects', objectIndex: number, kind: string) {
  const trigger = asString(hooks()[index].trigger);
  const variants = key === 'conditions' ? createConditionVariants(trigger, refs.value) : createHookEffectVariants(refs.value, trigger);
  const next = variants[kind]?.defaultValue;
  if (next) updateObject(index, key, objectIndex, clone(next));
}
function addObject(index: number, key: 'conditions' | 'effects') {
  const trigger = asString(hooks()[index].trigger);
  const variants = key === 'conditions' ? createConditionVariants(trigger, refs.value) : createHookEffectVariants(refs.value, trigger);
  const fallback = key === 'conditions' ? variants.adjacent_ally_exists : variants.heal;
  updateHook(index, key, [...objectList(hooks()[index], key), clone(fallback.defaultValue)]);
}
function removeObject(index: number, key: 'conditions' | 'effects', objectIndex: number) {
  const next = objectList(hooks()[index], key).map((entry) => clone(entry));
  next.splice(objectIndex, 1);
  updateHook(index, key, next);
}
function schemaFor(hook: DataRow, key: 'conditions' | 'effects', row: DataRow) {
  const trigger = asString(hook.trigger);
  const variants = key === 'conditions' ? createConditionVariants(trigger, refs.value) : createHookEffectVariants(refs.value, trigger);
  return variants[asString(row.kind)]?.schema ?? {};
}
function kindsFor(hook: DataRow, key: 'conditions' | 'effects'): string[] {
  const trigger = asString(hook.trigger);
  return key === 'conditions' ? specializationConditionKindsForTrigger(trigger) : specializationHookEffectKindsForTrigger(trigger);
}
function eventFilterSchema(hook: DataRow) { return createEventFilterSchema(refs.value, asString(hook.trigger)); }
function canAddEventFilter(hook: DataRow): boolean { return specializationEventFilterKeysForTrigger(asString(hook.trigger)).length > 0; }
function defaultEventFilter(hook: DataRow): DataRow {
  const trigger = asString(hook.trigger);
  const keys = new Set(specializationEventFilterKeysForTrigger(trigger));
  if (keys.has('includeBasicAttack')) return { includeBasicAttack: true };
  if (keys.has('turnKinds')) return { turnKinds: ['ordered'] };
  return {};
}
</script>

<template>
  <div class="hook-editor">
    <div class="editor-head"><strong>战斗触发</strong><button type="button" @click="addHook">+ 钩子</button></div>
    <section v-for="(hook, index) in hooks()" :key="`${hook.id}:${index}`" class="hook-row">
      <header><strong>{{ hook.id || `Hook ${index + 1}` }}</strong><button type="button" title="删除钩子" @click="removeHook(index)">×</button></header>
      <label>稳定 ID<input :readonly="lockIds" :value="hook.id" @change="updateHook(index, 'id', ($event.target as HTMLInputElement).value)" /></label>
      <div class="pair">
        <label>触发时机<select :value="hook.trigger" @change="updateHook(index, 'trigger', ($event.target as HTMLSelectElement).value)"><option v-for="trigger in SPECIALIZATION_BATTLE_TRIGGERS" :key="trigger" :value="trigger">{{ trigger }}</option></select></label>
        <label>优先级<input type="number" step="1" :value="hook.priority" @change="updateHook(index, 'priority', Number(($event.target as HTMLInputElement).value))" /></label>
      </div>
      <div class="optional-grid">
        <div class="optional-field">
          <header><strong>事件过滤</strong><button v-if="isRecord(hook.eventFilter)" type="button" title="移除过滤" @click="removeHookField(index, 'eventFilter')">×</button><button v-else-if="canAddEventFilter(hook)" type="button" title="添加过滤" @click="updateHook(index, 'eventFilter', defaultEventFilter(hook))">+</button></header>
          <DslObjectEditor v-if="isRecord(hook.eventFilter)" :model-value="hook.eventFilter" :schema="eventFilterSchema(hook)" @update:model-value="updateHook(index, 'eventFilter', $event)" />
        </div>
        <div class="optional-field">
          <header><strong>触发上限</strong><button v-if="isRecord(hook.limits)" type="button" title="移除上限" @click="removeHookField(index, 'limits')">×</button><button v-else type="button" title="添加上限" @click="updateHook(index, 'limits', { perTurn: 1 })">+</button></header>
          <DslObjectEditor v-if="isRecord(hook.limits)" :model-value="hook.limits" :schema="SPECIALIZATION_LIMITS_SCHEMA" @update:model-value="updateHook(index, 'limits', $event)" />
        </div>
      </div>
      <label class="check"><input type="checkbox" :checked="hook.dispatchHooks === true" @change="updateHook(index, 'dispatchHooks', ($event.target as HTMLInputElement).checked)" />派生事件继续派发 Hook</label>

      <div class="object-list">
        <header><strong>条件</strong><button type="button" @click="addObject(index, 'conditions')">+</button></header>
        <div v-for="(condition, conditionIndex) in objectList(hook, 'conditions')" :key="conditionIndex" class="object-row">
          <div class="structured-object">
            <select :value="condition.kind" @change="replaceObjectKind(index, 'conditions', conditionIndex, ($event.target as HTMLSelectElement).value)"><option v-for="kind in kindsFor(hook, 'conditions')" :key="kind" :value="kind">{{ kind }}</option></select>
            <DslObjectEditor :model-value="condition" :schema="schemaFor(hook, 'conditions', condition)" :hidden-keys="['kind']" @update:model-value="updateObject(index, 'conditions', conditionIndex, $event)" />
          </div>
          <button type="button" title="删除条件" @click="removeObject(index, 'conditions', conditionIndex)">×</button>
        </div>
      </div>

      <div class="object-list">
        <header><strong>效果</strong><button type="button" @click="addObject(index, 'effects')">+</button></header>
        <div v-for="(effect, effectIndex) in objectList(hook, 'effects')" :key="effectIndex" class="object-row">
          <div class="structured-object">
            <select :value="effect.kind" @change="replaceObjectKind(index, 'effects', effectIndex, ($event.target as HTMLSelectElement).value)"><option v-for="kind in kindsFor(hook, 'effects')" :key="kind" :value="kind">{{ kind }}</option></select>
            <DslObjectEditor :model-value="effect" :schema="schemaFor(hook, 'effects', effect)" :hidden-keys="['kind']" @update:model-value="updateObject(index, 'effects', effectIndex, $event)" />
          </div>
          <button type="button" title="删除效果" @click="removeObject(index, 'effects', effectIndex)">×</button>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.hook-editor, .hook-row, .object-list, .optional-field, .structured-object { display: grid; gap: 8px; }
.editor-head, .hook-row > header, .object-list > header { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.hook-row { padding: 9px 0; border-top: 1px solid #e2e7f0; }
.hook-row > header button, .object-row > button { background: #fff0f1; color: #a52838; }
.pair { display: grid; grid-template-columns: minmax(0, 1fr) 76px; gap: 7px; }
.optional-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.optional-field { align-content: start; padding: 7px; border: 1px solid #e1e7f0; border-radius: 6px; background: #f8faff; }
.optional-field > header { display: flex; align-items: center; justify-content: space-between; gap: 6px; }
.optional-field > header button { width: 26px; height: 26px; padding: 0; }
label { display: grid; gap: 4px; color: #526079; font-size: 10px; font-weight: 750; }
.check { display: flex; align-items: center; gap: 6px; }
.check input { width: auto; }
input, select { min-width: 0; padding: 6px; font-size: 10px; }
button { min-height: 27px; padding: 0 8px; border-radius: 5px; font-size: 10px; }
.object-list { padding: 7px; background: #f8faff; border: 1px solid #e1e7f0; border-radius: 6px; }
.object-row { display: grid; grid-template-columns: minmax(0, 1fr) 26px; gap: 5px; align-items: start; padding-top: 6px; border-top: 1px dashed #dbe3ef; }
.object-row > button { width: 26px; height: 26px; padding: 0; }
@media (max-width: 1320px) { .optional-grid { grid-template-columns: minmax(0, 1fr); } }
</style>
