<template>
  <div class="market-catalog-item" :class="{ 'market-catalog-item--compact': compact }" :title="name">
    <Card :item-id="entry.itemId" :instance-preview="entry.preview" :preview-enabled="false" hide-label :width="compact ? 24 : 64" :height="compact ? 24 : 64" :icon-size="compact ? 14 : 32" :hide-border="compact" />
    <span>{{ name }}</span>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@components/game/item/Card.vue'
import { marketCatalogName, type MarketCatalogEntry } from '@services/market-catalog'
const props = defineProps<{ entry: MarketCatalogEntry; compact?: boolean }>()
const { locale } = useI18n()
const name = computed(() => marketCatalogName(props.entry, locale.value))
</script>
<style scoped>
.market-catalog-item { display:flex; flex-direction:column; align-items:center; gap:6px; width:64px; font-size:12px; cursor:pointer; }
.market-catalog-item > span { width:100%; overflow:hidden; white-space:nowrap; text-overflow:ellipsis; text-align:center; }
.market-catalog-item--compact { display:inline-flex; flex-direction:row; width:auto; max-width:100%; gap:4px; font-size:12px; vertical-align:middle; cursor:default; }
.market-catalog-item--compact > span { width:auto; }
.market-catalog-item--compact { line-height:24px; }
.market-catalog-item--compact :deep(.labeled-frame-card) { flex:none; }
.market-catalog-item--compact :deep(.item-card) { background:none; box-shadow:none; }
.market-catalog-item--compact :deep(.item-card-custom-fish-icon) { max-height:100%; }
</style>
