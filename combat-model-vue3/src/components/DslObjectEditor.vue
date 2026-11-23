<script lang="ts">
import type { DataRow } from '../model/catalog';

export type DslObjectSchema = Record<string, DslFieldSpec>;
export type DslObjectVariant = { label?: string; defaultValue: DataRow; schema: DslObjectSchema };
export type DslFieldSpec = {
  type: 'text' | 'number' | 'boolean' | 'select' | 'string_list' | 'object' | 'object_list';
  label?: string;
  defaultValue: unknown;
  required?: boolean;
  readonly?: boolean;
  options?: readonly string[];
  min?: number;
  max?: number;
  step?: number;
  schema?: DslObjectSchema;
  variants?: Record<string, DslObjectVariant>;
  variantKey?: string;
};
</script>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { clone, isRecord } from '../model/catalog';

const props = withDefaults(defineProps<{
  modelValue: DataRow;
  schema: DslObjectSchema;
  protectedKeys?: string[];
  hiddenKeys?: string[];
  depth?: number;
}>(), { protectedKeys: () => [], hiddenKeys: () => [], depth: 0 });

const emit = defineEmits<{ 'update:modelValue': [value: DataRow] }>();
const newKey = ref('');
const knownEntries = computed(() => Object.keys(props.schema).filter((key) => !props.hiddenKeys.includes(key) && Object.prototype.hasOwnProperty.call(props.modelValue, key)).map((key) => [key, props.modelValue[key]] as const));
const unknownEntries = computed(() => Object.keys(props.modelValue).filter((key) => !props.schema[key] && !props.hiddenKeys.includes(key)));
const availableKeys = computed(() => Object.keys(props.schema).filter((key) => !props.hiddenKeys.includes(key) && !Object.prototype.hasOwnProperty.call(props.modelValue, key)));

function updateKey(key: string, value: unknown) {
  emit('update:modelValue', { ...clone(props.modelValue), [key]: value });
}

function removeKey(key: string) {
  const next = clone(props.modelValue);
  delete next[key];
  emit('update:modelValue', next);
}

function addKey() {
  const key = newKey.value || availableKeys.value[0];
  const spec = props.schema[key];
  if (!key || !spec || Object.prototype.hasOwnProperty.call(props.modelValue, key)) return;
  updateKey(key, clone(spec.defaultValue));
  newKey.value = '';
}

function updatePrimitive(key: string, spec: DslFieldSpec, event: Event) {
  const target = event.target as HTMLInputElement | HTMLSelectElement;
  if (spec.type === 'number') updateKey(key, Number(target.value));
  else if (spec.type === 'boolean') updateKey(key, target.value === 'true');
  else updateKey(key, target.value);
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : [];
}

function updateStringList(key: string, spec: DslFieldSpec, event: Event) {
  const target = event.target as HTMLInputElement | HTMLSelectElement;
  const value = spec.options
    ? Array.from((target as HTMLSelectElement).selectedOptions).map((option) => option.value)
    : target.value.split(',').map((entry) => entry.trim()).filter(Boolean);
  updateKey(key, value);
}

function objectValue(value: unknown): DataRow {
  return isRecord(value) ? value : {};
}

function variantFor(spec: DslFieldSpec, value: unknown): DslObjectVariant | undefined {
  const record = objectValue(value);
  const variantKey = spec.variantKey ?? 'kind';
  const variantId = typeof record[variantKey] === 'string' ? record[variantKey] : '';
  return spec.variants?.[variantId];
}

function updateObjectVariant(key: string, spec: DslFieldSpec, event: Event) {
  const variantId = (event.target as HTMLSelectElement).value;
  const variant = spec.variants?.[variantId];
  if (variant) updateKey(key, clone(variant.defaultValue));
}

function updateArrayItem(key: string, index: number, value: DataRow) {
  const next = Array.isArray(props.modelValue[key]) ? clone(props.modelValue[key] as unknown[]) : [];
  next[index] = value;
  updateKey(key, next);
}

function removeArrayItem(key: string, index: number) {
  const next = Array.isArray(props.modelValue[key]) ? clone(props.modelValue[key] as unknown[]) : [];
  next.splice(index, 1);
  updateKey(key, next);
}

function addArrayItem(key: string, spec: DslFieldSpec) {
  const next = Array.isArray(props.modelValue[key]) ? clone(props.modelValue[key] as unknown[]) : [];
  const variant = spec.variants ? Object.values(spec.variants)[0] : undefined;
  const configuredDefault = Array.isArray(spec.defaultValue) && isRecord(spec.defaultValue[0]) ? spec.defaultValue[0] : {};
  const item = clone(variant?.defaultValue ?? configuredDefault);
  if (isRecord(item) && typeof item.id === 'string' && item.id.trim()) {
    const existing = new Set(next.filter(isRecord).map((entry) => typeof entry.id === 'string' ? entry.id : '').filter(Boolean));
    const base = item.id.trim();
    let candidate = base;
    let suffix = 2;
    while (existing.has(candidate)) candidate = `${base}_${suffix++}`;
    item.id = candidate;
  }
  next.push(item);
  updateKey(key, next);
}

function updateArrayVariant(key: string, index: number, spec: DslFieldSpec, event: Event) {
  const variant = spec.variants?.[(event.target as HTMLSelectElement).value];
  if (!variant) return;
  const previous = Array.isArray(props.modelValue[key]) && isRecord(props.modelValue[key][index])
    ? props.modelValue[key][index] as DataRow
    : {};
  const next = clone(variant.defaultValue);
  if (typeof previous.id === 'string' && previous.id.trim() && typeof next.id === 'string') next.id = previous.id;
  updateArrayItem(key, index, next);
}
</script>

<template>
  <div class="dsl-object" :class="{ nested: depth > 0 }">
    <div v-for="([key, value]) in knownEntries" :key="key" class="dsl-field">
      <header>
        <code>{{ schema[key].label || key }}</code>
        <button v-if="!schema[key].required && !protectedKeys.includes(key)" type="button" title="删除字段" @click="removeKey(key)">×</button>
      </header>

      <select v-if="schema[key].type === 'boolean'" :disabled="schema[key].readonly" :value="String(value)" @change="updatePrimitive(key, schema[key], $event)">
        <option value="true">是</option><option value="false">否</option>
      </select>
      <input
        v-else-if="schema[key].type === 'number'"
        type="number"
        :step="schema[key].step ?? 'any'"
        :min="schema[key].min"
        :max="schema[key].max"
        :readonly="schema[key].readonly"
        :value="value as number"
        @change="updatePrimitive(key, schema[key], $event)"
      />
      <select v-else-if="schema[key].type === 'select'" :disabled="schema[key].readonly" :value="value as string" @change="updatePrimitive(key, schema[key], $event)">
        <option v-for="option in schema[key].options" :key="option" :value="option">{{ option }}</option>
      </select>
      <template v-else-if="schema[key].type === 'string_list'">
        <select v-if="schema[key].options" class="multi-select" multiple :disabled="schema[key].readonly" :value="stringList(value)" @change="updateStringList(key, schema[key], $event)">
          <option v-for="option in schema[key].options" :key="option" :value="option">{{ option }}</option>
        </select>
        <input v-else :readonly="schema[key].readonly" :value="stringList(value).join(', ')" placeholder="逗号分隔" @change="updateStringList(key, schema[key], $event)" />
      </template>
      <input v-else-if="schema[key].type === 'text'" :readonly="schema[key].readonly" :value="value as string" @change="updatePrimitive(key, schema[key], $event)" />
      <div v-else-if="schema[key].type === 'object'" class="dsl-nested-object">
        <select v-if="schema[key].variants" :value="objectValue(value)[schema[key].variantKey ?? 'kind'] as string" @change="updateObjectVariant(key, schema[key], $event)">
          <option v-for="(variant, variantId) in schema[key].variants" :key="variantId" :value="variantId">{{ variant.label || variantId }}</option>
        </select>
        <DslObjectEditor
          :model-value="objectValue(value)"
          :schema="variantFor(schema[key], value)?.schema ?? schema[key].schema ?? {}"
          :protected-keys="schema[key].variants ? [schema[key].variantKey ?? 'kind'] : []"
          :hidden-keys="schema[key].variants ? [schema[key].variantKey ?? 'kind'] : []"
          :depth="depth + 1"
          @update:model-value="updateKey(key, $event)"
        />
      </div>
      <div v-else-if="schema[key].type === 'object_list'" class="dsl-array">
        <div v-for="(item, index) in (Array.isArray(value) ? value : [])" :key="index" class="dsl-array-item">
          <div>
            <select v-if="schema[key].variants" :value="objectValue(item)[schema[key].variantKey ?? 'kind'] as string" @change="updateArrayVariant(key, index, schema[key], $event)">
              <option v-for="(variant, variantId) in schema[key].variants" :key="variantId" :value="variantId">{{ variant.label || variantId }}</option>
            </select>
            <DslObjectEditor
              :model-value="objectValue(item)"
              :schema="variantFor(schema[key], item)?.schema ?? schema[key].schema ?? {}"
              :protected-keys="schema[key].variants ? [schema[key].variantKey ?? 'kind'] : []"
              :hidden-keys="schema[key].variants ? [schema[key].variantKey ?? 'kind'] : []"
              :depth="depth + 1"
              @update:model-value="updateArrayItem(key, index, $event)"
            />
          </div>
          <button type="button" title="删除数组项" @click="removeArrayItem(key, index)">×</button>
        </div>
        <footer><button type="button" @click="addArrayItem(key, schema[key])">+ 项目</button></footer>
      </div>
    </div>

    <div v-for="key in unknownEntries" :key="key" class="dsl-unknown">
      <span><code>{{ key }}</code> 不受当前 DSL 支持，保存前请删除</span>
      <button type="button" title="删除未知字段" @click="removeKey(key)">×</button>
    </div>

    <footer v-if="availableKeys.length" class="dsl-add">
      <select v-model="newKey" aria-label="可添加字段">
        <option value="">选择字段</option>
        <option v-for="key in availableKeys" :key="key" :value="key">{{ schema[key].label || key }}</option>
      </select>
      <button type="button" :disabled="!newKey" @click="addKey">+</button>
    </footer>
  </div>
</template>

<style scoped>
.dsl-object { display: grid; gap: 7px; min-width: 0; }
.dsl-object.nested { padding: 7px; border-left: 2px solid #dbe3ef; background: #f8faff; }
.dsl-field { display: grid; gap: 5px; min-width: 0; }
.dsl-field > header { display: flex; align-items: center; justify-content: space-between; gap: 6px; }
code { color: #4f5d75; font-size: 10px; overflow-wrap: anywhere; }
button { min-width: 25px; height: 25px; padding: 0 6px; border-radius: 5px; background: #edf1f7; color: #46546d; font-size: 10px; }
.dsl-field > header button, .dsl-array-item > button, .dsl-unknown button { color: #a52838; background: #fff0f1; }
input, select { min-width: 0; padding: 6px; font-size: 10px; }
.multi-select { min-height: 72px; }
.dsl-nested-object, .dsl-array { display: grid; gap: 5px; }
.dsl-array-item { display: grid; grid-template-columns: minmax(0, 1fr) 25px; gap: 5px; align-items: start; }
.dsl-array-item > div { display: grid; gap: 5px; min-width: 0; }
.dsl-array footer, .dsl-add { display: flex; gap: 5px; align-items: center; }
.dsl-add { padding-top: 6px; border-top: 1px dashed #dbe3ef; }
.dsl-add select { flex: 1 1 auto; }
.dsl-unknown { display: grid; grid-template-columns: minmax(0, 1fr) 26px; gap: 6px; align-items: center; padding: 7px; border: 1px solid #e3aab0; border-radius: 5px; color: #942735; background: #fff5f6; font-size: 10px; }
</style>
