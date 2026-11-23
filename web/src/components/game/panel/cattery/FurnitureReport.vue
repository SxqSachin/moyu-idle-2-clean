<template>
  <section v-if="loading || error || report?.token" class="furniture-report">
    <header><h4>{{ t('game.furniture.reportTitle') }}</h4><el-button v-if="error" :aria-label="t('game.furniture.refresh')" @click="load"><Icon icon="mdi:refresh" /></el-button></header>
    <p class="app-muted">{{ t('game.furniture.reportNote') }}</p>
    <el-skeleton v-if="loading" :rows="2" animated />
    <el-alert v-else-if="error" :title="t('game.furniture.errors.FURNITURE_UNAVAILABLE')" type="warning" :closable="false" />
    <template v-else-if="report">
      <div v-for="(row,id) in report.rows" :key="id" class="report-row"><strong>{{ t(report.names?.[id] || String(id)) }}</strong><FurnitureContributionItems :row="row" /></div>
      <el-button :disabled="seen" :loading="acking" @click="ack">{{ t(seen ? 'game.furniture.reportRead' : 'game.furniture.markRead') }}</el-button>
    </template>
  </section>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@iconify/vue'
import { useGameStore } from '@stores/game'
import { getFurnitureReport, acknowledgeFurnitureReport, type FurnitureReport } from '@services/cattery-furniture'
import FurnitureContributionItems from './FurnitureContributionItems.vue'
const props = defineProps<{ active: boolean }>()
const game = useGameStore(), { t } = useI18n()
const report = ref<FurnitureReport | null>(null), loading = ref(false), error = ref(false), seen = ref(false), acking = ref(false)
let epoch = 0
async function load() {
  const token = game.token, current = ++epoch
  report.value = null; error.value = false; seen.value = false
  if (!token || !props.active) { loading.value=false; return }
  loading.value = true
  try { const result = await getFurnitureReport(token); if (current === epoch) report.value=result }
  catch { if (current===epoch) error.value=true }
  finally { if (current===epoch) loading.value=false }
}
async function ack() {
  if (!report.value?.token || acking.value) return
  const current=epoch, token=game.token, id=report.value.token
  acking.value=true
  try { await acknowledgeFurnitureReport(token,id); if (current===epoch) seen.value=true }
  catch { if (current===epoch) error.value=true }
  finally { if (current===epoch) acking.value=false }
}
watch(() => [props.active,game.token], () => { acking.value=false; void load() }, { immediate:true })
onBeforeUnmount(() => { epoch++ })
</script>
<style scoped>
.furniture-report { border-top:1px solid var(--app-border); padding-top:16px; display:grid; gap:12px; }
header { display:flex; justify-content:space-between; align-items:center; } h4,p { margin:0; } p { font-size:12px; }
.report-row { display:grid; gap:8px; } .furniture-report > .el-button { justify-self:start; }
</style>
