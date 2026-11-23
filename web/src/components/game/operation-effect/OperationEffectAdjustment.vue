<template>
  <el-popover placement="top" :width="280" trigger="click">
    <template #reference>
      <button type="button" class="operation-effect-adjustment" :class="{ 'is-cost': cost }" :aria-label="label">
        <Icon icon="lucide:sparkles" aria-hidden="true" />
        <span>{{ label }}</span>
        <Icon icon="lucide:circle-help" aria-hidden="true" />
      </button>
    </template>
    <div class="operation-effect-adjustment__detail">{{ detail }}</div>
  </el-popover>
</template>

<script setup lang="ts">
import { Icon } from "@iconify/vue"
defineProps<{ label: string; detail: string; cost?: boolean }>()
</script>

<style scoped>
.operation-effect-adjustment { display: inline-flex; align-items: center; gap: 4px; max-width: 100%; padding: 2px 0; border: 0; background: none; color: var(--app-primary); font: inherit; font-size: 12px; line-height: 1.5; text-align: start; cursor: pointer; }
.operation-effect-adjustment.is-cost { color: var(--el-color-warning); }
.operation-effect-adjustment svg { flex: 0 0 13px; width: 13px; height: 13px; }
.operation-effect-adjustment span { min-width: 0; overflow-wrap: anywhere; }
.operation-effect-adjustment:focus-visible { outline: 2px solid var(--app-primary); outline-offset: 3px; }
.operation-effect-adjustment__detail { font-size: 12px; line-height: 1.6; white-space: pre-line; overflow-wrap: anywhere; }
</style>
