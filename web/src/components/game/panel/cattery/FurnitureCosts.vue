<template>
  <div class="furniture-costs">
    <div v-for="[id, quantity] in Object.entries(costs)" :key="id" class="cost-entry">
      <QuickRefWithRequire :item-id="id" :required-count="quantity" :available-count="inventory[id] || 0" />
    </div>
  </div>
</template>
<script setup lang="ts">
import QuickRefWithRequire from '@components/game/item/QuickRefWithRequire.vue'
defineProps<{ costs: Record<string,number>; inventory: Record<string,number> }>()
</script>
<style scoped>
.furniture-costs { display:flex; align-items:flex-start; flex-wrap:wrap; gap:8px; min-width:0; }
.cost-entry { max-width:100%; min-width:0; display:flex; align-items:flex-start; }
.cost-entry :deep(.el-tag) { height:auto; min-height:24px; padding:0 6px; font-size:12px; line-height:22px; white-space:normal; max-width:100%; }
.cost-entry :deep(.el-tag__content) { display:flex; align-items:center; min-width:0; }
.cost-entry :deep(.item-quick-ref) { flex-wrap:wrap; gap:3px; }
.cost-entry :deep(.item-quick-ref-name) { overflow-wrap:anywhere; }
.cost-entry :deep(.item-quick-ref-count) { font-variant-numeric:tabular-nums; white-space:nowrap; }
</style>
