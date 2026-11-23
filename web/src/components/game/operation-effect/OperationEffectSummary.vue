<template>
  <ul class="effect-summary">
    <li v-for="line in lines" :key="line"><span class="effect-summary__dot" aria-hidden="true" /><span>{{ line }}</span></li>
  </ul>
</template>

<script setup lang="ts">
defineProps<{ lines: string[] }>()
</script>

<style scoped>
.effect-summary { display: grid; gap: 4px; padding: 0; margin: 0; list-style: none; font-size: 12px; line-height: 1.6; }
.effect-summary li { display: flex; align-items: flex-start; gap: 6px; min-width: 0; overflow-wrap: anywhere; }
.effect-summary__dot { width: 6px; height: 6px; flex: 0 0 6px; margin-top: 6px; border-radius: 50%; background: var(--el-color-success); }
</style>
