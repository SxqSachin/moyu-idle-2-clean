<script setup lang="ts">
import { computed } from 'vue';
import { asArray, asNumber, asString, isRecord } from '../model/catalog';

type FeedCondition = { field: string; op: string; value: string | number };
type FeedEffect = {
  kind: 'stat_delta';
  stat: string;
  mode: 'bonus_band' | 'flat';
  valuePerPortion: number;
  maxTargetMultiplier?: number;
  floor?: number;
  ceiling?: number;
};

const props = defineProps<{ modelValue?: Record<string, unknown> }>();
const emit = defineEmits<{ (event: 'update:modelValue', value: Record<string, unknown>): void }>();

const config = computed(() => isRecord(props.modelValue) ? props.modelValue : {});
const enabled = computed(() => Object.keys(config.value).length > 0);
const dslEnabled = computed(() => asString(config.value.rulesetVersion) === 'pond-feed-v2');
const targeted = computed(() => config.value.delivery === 'targeted');
const effectStats = ['health', 'satiety', 'lengthCm', 'weightKg', 'longevityDays', 'naturalProtectionDays'];
function lifeStat(stat: string) { return stat === 'longevityDays' || stat === 'naturalProtectionDays'; }
const conditions = computed(() => asArray(config.value.conditions).filter(isRecord).map((row) => ({
  field: asString(row.field, 'health'),
  op: asString(row.op, 'lt'),
  value: typeof row.value === 'string' ? row.value : asNumber(row.value, 0.98)
})) as FeedCondition[]);
const effects = computed(() => asArray(config.value.effects).filter(isRecord).map(normalizeEffect));

function normalizeEffect(row: Record<string, unknown>): FeedEffect {
  const stat = asString(row.stat, 'health');
  const growth = stat === 'lengthCm' || stat === 'weightKg';
  return {
    kind: 'stat_delta',
    stat,
    mode: growth ? 'bonus_band' : 'flat',
    valuePerPortion: asNumber(row.valuePerPortion, lifeStat(stat) ? 1095 : growth ? 0.04 : 0.0025),
    ...(growth ? { maxTargetMultiplier: asNumber(row.maxTargetMultiplier, 1.025) } : {}),
    ...(!growth && row.floor !== undefined ? { floor: asNumber(row.floor, 0) } : {}),
    ...(!growth && row.ceiling !== undefined ? { ceiling: asNumber(row.ceiling, stat === 'satiety' ? 100 : 1) } : {})
  };
}

function patch(next: Record<string, unknown>) {
  emit('update:modelValue', { ...config.value, ...next });
}

function toggleEnabled(value: boolean) {
  emit('update:modelValue', value ? { nutrition: 10, feedClass: 'herbal' } : {});
}

function toggleDsl(value: boolean) {
  if (value) {
    patch({ rulesetVersion: 'pond-feed-v2', role: 'staple', consumePriority: 0, conditions: [], effects: [] });
    return;
  }
  const { delivery: _delivery, role: _role, maxPortionsPerFishHour: _dose, rulesetVersion: _rulesetVersion, consumePriority: _consumePriority, conditions: _conditions, effects: _effects, ...basic } = config.value;
  emit('update:modelValue', { ...basic, nutrition: Math.max(1, asNumber(basic.nutrition, 10)) });
}

function changeDelivery(delivery: string) {
  const manual = delivery === 'targeted';
  patch({ delivery, nutrition: manual ? 0 : 10, role: effects.value.length || manual ? 'functional' : 'staple', consumePriority: manual ? undefined : 0, maxPortionsPerFishHour: !manual && effects.value.length ? 0.5 : undefined });
}

function emitConditions(next: FeedCondition[]) { patch({ conditions: next }); }
function addCondition() { emitConditions([...conditions.value, { field: 'health', op: 'lt', value: 0.98 }]); }
function removeCondition(index: number) { emitConditions(conditions.value.filter((_, rowIndex) => rowIndex !== index)); }
function updateCondition(index: number, next: Partial<FeedCondition>) {
  const rows = conditions.value.map((row, rowIndex) => rowIndex === index ? { ...row, ...next } : row);
  emitConditions(rows);
}
function changeConditionField(index: number, field: string) {
  if (field === 'dietClass') updateCondition(index, { field, op: 'eq', value: 'herbal' });
  else if (field === 'pondZone') updateCondition(index, { field, op: 'eq', value: 'main' });
  else updateCondition(index, { field, op: 'lt', value: field === 'satiety' ? 80 : field === 'condition' ? 0.9 : 0.98 });
}
function conditionOps(field: string) { return field === 'dietClass' || field === 'pondZone' ? ['eq', 'neq'] : ['eq', 'neq', 'lt', 'lte', 'gt', 'gte']; }
function conditionValueOptions(field: string) {
  if (field === 'dietClass') return ['herbal', 'insect', 'protein', 'arcane'];
  if (field === 'pondZone') return ['main', 'nursery'];
  return [];
}

function emitEffects(next: FeedEffect[]) { patch({ effects: next, role: next.length || targeted.value ? 'functional' : 'staple', maxPortionsPerFishHour: next.length && !targeted.value ? asNumber(config.value.maxPortionsPerFishHour, 0.5) : undefined }); }
function addEffect() {
  if (effects.value.length >= 4) return;
  const stat = effectStats.find(stat => (!lifeStat(stat) || targeted.value) && !effects.value.some(effect => effect.stat === stat));
  if (!stat) return;
  emitEffects([...effects.value, normalizeEffect({ stat })]);
}
function removeEffect(index: number) { emitEffects(effects.value.filter((_, rowIndex) => rowIndex !== index)); }
function updateEffect(index: number, next: Partial<FeedEffect>) {
  emitEffects(effects.value.map((row, rowIndex) => rowIndex === index ? { ...row, ...next } : row));
}
function changeEffectStat(index: number, stat: string) {
  if (lifeStat(stat)) { updateEffect(index, { stat, mode: 'flat', valuePerPortion: 1095, maxTargetMultiplier: undefined, floor: undefined, ceiling: undefined }); return; }
  const growth = stat === 'lengthCm' || stat === 'weightKg';
  updateEffect(index, growth
    ? { stat, mode: 'bonus_band', valuePerPortion: 0.04, maxTargetMultiplier: stat === 'lengthCm' ? 1.025 : 1.04, floor: undefined, ceiling: undefined }
    : { stat, mode: 'flat', valuePerPortion: stat === 'satiety' ? 3 : 0.0025, maxTargetMultiplier: undefined, floor: undefined, ceiling: stat === 'satiety' ? 100 : 1 });
}

function inputNumber(event: Event) { return Number((event.target as HTMLInputElement).value); }
function inputOptionalNumber(event: Event) {
  const value = (event.target as HTMLInputElement).value.trim();
  return value === '' ? undefined : Number(value);
}
function inputString(event: Event) { return (event.target as HTMLInputElement).value; }
</script>

<template>
  <section class="pond-feed-editor">
    <label class="toggle-row"><input type="checkbox" :checked="enabled" @change="toggleEnabled(($event.target as HTMLInputElement).checked)"><span>配置为池塘鱼粮</span></label>
    <template v-if="enabled">
      <div class="base-grid">
        <label v-if="!targeted">每份营养<input type="number" min="0.001" step="1" :value="asNumber(config.nutrition, 10)" @input="patch({ nutrition: inputNumber($event) })"></label>
        <label>食性类别<select :value="asString(config.feedClass, 'herbal')" @change="patch({ feedClass: inputString($event) })"><option v-for="value in ['herbal', 'insect', 'protein', 'arcane', 'recycled']" :key="value" :value="value">{{ value }}</option></select></label>
      </div>
      <label class="toggle-row"><input type="checkbox" :checked="dslEnabled" @change="toggleDsl(($event.target as HTMLInputElement).checked)"><span>启用 pond-feed-v2 组合规则</span></label>
      <template v-if="dslEnabled">
        <label class="priority">投喂方式<select :value="asString(config.delivery, 'feeder')" @change="changeDelivery(inputString($event))"><option value="feeder">共享料仓</option><option value="targeted">指定活鱼，整份投喂</option></select></label>
        <label v-if="!targeted" class="priority">消耗优先级<input type="number" min="-100" max="100" step="1" :value="asNumber(config.consumePriority, 0)" @input="patch({ consumePriority: inputNumber($event) })"></label>
        <label v-if="effects.length && !targeted" class="priority">每鱼每小时最多份数<input type="number" min="0.001" max="0.5" step="0.01" :value="asNumber(config.maxPortionsPerFishHour, 0.5)" @input="patch({ maxPortionsPerFishHour: inputNumber($event) })"></label>
        <div class="rule-block">
          <header><strong>取食条件</strong><button type="button" class="add" title="新增取食条件" @click="addCondition">+</button></header>
          <p v-if="conditions.length === 0">无条件，所有鱼都可取食。</p>
          <div v-for="(condition, index) in conditions" :key="index" class="rule-row condition-row">
            <select :value="condition.field" @change="changeConditionField(index, inputString($event))"><option v-for="value in ['dietClass', 'pondZone', 'health', 'satiety', 'condition']" :key="value" :value="value">{{ value }}</option></select>
            <select :value="condition.op" @change="updateCondition(index, { op: inputString($event) })"><option v-for="value in conditionOps(condition.field)" :key="value" :value="value">{{ value }}</option></select>
            <select v-if="conditionValueOptions(condition.field).length" :value="condition.value" @change="updateCondition(index, { value: inputString($event) })"><option v-for="value in conditionValueOptions(condition.field)" :key="value" :value="value">{{ value }}</option></select>
            <input v-else type="number" min="0" :max="condition.field === 'satiety' ? 100 : condition.field === 'condition' ? 1.12 : 1" step="0.01" :value="condition.value" @input="updateCondition(index, { value: inputNumber($event) })">
            <button type="button" class="remove" title="删除取食条件" @click="removeCondition(index)">×</button>
          </div>
        </div>
        <div class="rule-block">
          <header><strong>取食效果</strong><button type="button" class="add" title="新增取食效果" @click="addEffect">+</button></header>
          <p v-if="effects.length === 0">只提供基础营养，不附加属性效果。</p>
          <div v-for="(effect, index) in effects" :key="index" class="effect-row">
            <div class="rule-row">
              <select :value="effect.stat" @change="changeEffectStat(index, inputString($event))"><option v-for="value in effectStats" :key="value" :value="value" :disabled="lifeStat(value) && !targeted || effects.some((e, i) => i !== index && e.stat === value)">{{ value }}</option></select>
              <span>{{ effect.mode }}</span>
              <label>每份值<input type="number" step="0.0005" :value="effect.valuePerPortion" @input="updateEffect(index, { valuePerPortion: inputNumber($event) })"></label>
              <button type="button" class="remove" title="删除取食效果" @click="removeEffect(index)">×</button>
            </div>
            <div v-if="!lifeStat(effect.stat)" class="effect-options">
              <label v-if="effect.mode === 'bonus_band'">目标倍率<input type="number" min="1.001" max="1.15" step="0.005" :value="effect.maxTargetMultiplier" @input="updateEffect(index, { maxTargetMultiplier: inputNumber($event) })"></label>
              <template v-else><label>下限<input type="number" min="0" :max="effect.stat === 'satiety' ? 100 : 1" step="0.01" :value="effect.floor ?? ''" @input="updateEffect(index, { floor: inputOptionalNumber($event) })"></label><label>上限<input type="number" min="0" :max="effect.stat === 'satiety' ? 100 : 1" step="0.01" :value="effect.ceiling ?? ''" @input="updateEffect(index, { ceiling: inputOptionalNumber($event) })"></label></template>
            </div>
          </div>
        </div>
      </template>
    </template>
  </section>
</template>

<style scoped>
.pond-feed-editor { display: grid; gap: 10px; padding: 12px; border: 1px solid #dbe2f0; border-radius: 6px; background: #fbfcff; }
.toggle-row { display: flex; align-items: center; gap: 8px; color: #172033; font-weight: 750; }
.toggle-row input { width: 16px; height: 16px; }
.base-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.base-grid label, .priority, .effect-row label { display: grid; gap: 5px; color: #50607d; font-size: 12px; }
input, select { min-width: 0; padding: 7px 8px; border: 1px solid #dbe2f0; border-radius: 4px; background: #fff; color: #172033; }
.priority { max-width: 220px; }
.rule-block { display: grid; gap: 8px; padding: 10px; border: 1px solid #e5ebf6; border-radius: 6px; background: #fff; }
.rule-block header { display: flex; align-items: center; justify-content: space-between; }
.rule-block header strong { color: #172033; font-size: 13px; }
.rule-block p { margin: 0; color: #73809a; font-size: 12px; }
.rule-row { display: grid; grid-template-columns: minmax(100px, 1fr) minmax(76px, .7fr) minmax(100px, 1fr) 32px; gap: 7px; align-items: end; }
.rule-row > span { padding: 8px; color: #73809a; font-size: 12px; }
.effect-row { display: grid; gap: 7px; padding: 8px; border: 1px solid #e5ebf6; border-radius: 5px; background: #f8faff; }
.effect-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; padding-left: 2px; }
button { width: 30px; height: 30px; padding: 0; border: 1px solid #dbe2f0; border-radius: 4px; background: #fff; color: #172033; cursor: pointer; }
button.add { border-color: #c7d2fe; background: #eef2ff; color: #3157d4; font-size: 18px; }
button.remove { border-color: #fecaca; background: #fff0f0; color: #c93a3a; font-size: 17px; }
@media (max-width: 720px) { .base-grid, .effect-options { grid-template-columns: 1fr; }.rule-row { grid-template-columns: 1fr 1fr; }.rule-row .remove { justify-self: end; } }
</style>
