<template>
  <span class="food-value-hint" data-testid="food-value-hint"><Icon icon="lucide:sparkles" /><span><slot /></span></span>
</template>
<script setup lang="ts">
import { Icon } from "@iconify/vue"
</script>
<style scoped>
.food-value-hint { display: inline-flex; align-items: baseline; gap: 5px; color: var(--app-primary); font-size: 12px; line-height: 1.5; min-width: 0; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
.food-value-hint > svg { flex: 0 0 13px; align-self: center; }
</style>
