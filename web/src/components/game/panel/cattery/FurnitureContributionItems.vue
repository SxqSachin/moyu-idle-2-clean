<template>
  <div class="contribution-items">
    <p v-if="!hasRewards && !row.opportunities" class="app-muted">{{ t('game.furniture.noContribution') }}</p>
    <p v-if="row.opportunities" class="app-muted">{{ t('game.furniture.participation', { hits: row.triggered || 0, total: row.opportunities, empty: row.empty || 0 }) }}</p>
    <template v-if="Object.keys(row.experience || {}).length"><p class="app-muted">{{ t('game.fastForwardRewardsAttrExp') }}</p><div class="contribution-values"><StatusTag v-for="(n,key) in row.experience" :key="key" :attr-key="String(key)" :value="n" /></div></template>
    <div v-if="Object.keys(row.items || {}).length" class="contribution-values"><QuickRef v-for="(n,id) in row.items" :key="id" :item-id="String(id)" :count="n" normal-number-format /></div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import QuickRef from '@components/game/item/QuickRef.vue'
import StatusTag from '@components/game/attribute/StatusTag.vue'
import type { FurnitureContribution } from '@services/cattery-furniture'
const props = defineProps<{ row: FurnitureContribution }>()
const { t } = useI18n()
const hasRewards = computed(() => Object.keys(props.row.items || {}).length + Object.keys(props.row.experience || {}).length > 0)
</script>
<style scoped>
.contribution-items { display:grid; gap:8px; min-width:0; }
.contribution-items p { margin:0; font-size:12px; }
.contribution-values { display:flex; gap:8px; flex-wrap:wrap; min-width:0; }
</style>
