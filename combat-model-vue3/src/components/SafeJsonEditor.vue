<script setup lang="ts">
import { ref, watch } from 'vue';

const props = withDefaults(defineProps<{
  modelValue: unknown;
  label?: string;
  expected?: 'array' | 'object' | 'either';
}>(), {
  label: 'JSON',
  expected: 'either'
});

const emit = defineEmits<{
  'update:modelValue': [value: unknown];
  focus: [];
}>();

const raw = ref(JSON.stringify(props.modelValue, null, 2));
const error = ref('');
const focused = ref(false);

watch(
  () => props.modelValue,
  (value) => {
    if (!focused.value || !error.value) raw.value = JSON.stringify(value, null, 2);
  },
  { deep: true }
);

function handleInput(event: Event) {
  raw.value = (event.target as HTMLTextAreaElement).value;
  try {
    const parsed = JSON.parse(raw.value) as unknown;
    if (props.expected === 'array' && !Array.isArray(parsed)) throw new Error('必须是 JSON 数组');
    if (props.expected === 'object' && (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))) throw new Error('必须是 JSON 对象');
    if (props.expected === 'either' && (!parsed || typeof parsed !== 'object')) throw new Error('必须是 JSON 对象或数组');
    error.value = '';
    emit('update:modelValue', parsed);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause);
  }
}
</script>

<template>
  <label class="safe-json">
    <span>{{ label }}</span>
    <textarea
      :value="raw"
      :class="{ invalid: error }"
      rows="9"
      spellcheck="false"
      @focus="focused = true; emit('focus')"
      @blur="focused = false"
      @input="handleInput"
    />
    <small v-if="error">{{ error }}</small>
    <small v-else class="valid">有效 JSON，已同步</small>
  </label>
</template>

<style scoped>
.safe-json { display: grid; gap: 6px; min-width: 0; color: #4b5870; font-size: 12px; font-weight: 750; }
textarea { min-height: 170px; resize: vertical; font: 12px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
textarea.invalid { border-color: #c24141; background: #fff8f8; }
small { color: #b42332; font-weight: 700; }
small.valid { color: #277052; }
</style>
