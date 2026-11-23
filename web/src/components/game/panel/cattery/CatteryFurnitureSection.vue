<template>
  <section class="furniture-section" :aria-busy="store.busy">
    <header class="furniture-header">
      <h3>{{ t('game.furniture.title') }}</h3>
      <el-button :disabled="store.busy || store.loading" :title="t('game.furniture.refresh')" :aria-label="t('game.furniture.refresh')" @click="refresh(true)"><Icon icon="mdi:refresh" width="20" /></el-button>
    </header>
    <el-alert v-if="store.errorCode" :title="errorText" type="warning" :closable="false" show-icon />
    <el-button v-if="store.pending" :loading="store.busy" @click="reconcile">{{ t('game.furniture.checkResult') }}</el-button>
    <el-skeleton v-if="!store.projection && !store.errorCode" :rows="4" animated />
    <p v-else-if="store.projection && !store.projection.available" class="app-muted">{{ t('game.furniture.errors.FURNITURE_CATTERY_REQUIRED') }}</p>
    <div v-if="store.projection?.items.length" class="furniture-grid" :aria-label="t('game.furniture.choose')">
        <button v-for="item in views" :key="item.definition.id" type="button" class="furniture-card" :data-furniture="item.definition.id" aria-haspopup="dialog" @click="showDetails(item)">
          <span class="card-heading">
            <img :src="art(item)" alt="" width="72" height="72" />
            <span class="card-name"><strong>{{ t(item.definition.nameKey) }}</strong><small>Lv.{{ item.installed ? item.definition.level || 1 : 0 }}<span> / {{ item.maxLevel || 1 }}</span></small></span>
            <Icon class="card-chevron" icon="mdi:chevron-right" width="18" />
          </span>
          <span class="card-effects">
            <span v-for="effect in cardEffects[item.definition.id]" :key="effect.key" class="card-effect">
              <strong>{{ effect.kind === 'action_efficiency_add' ? '+' : '' }}{{ effect.rateBps / 100 }}<small>%</small></strong>
              <span>{{ t(`game.furniture.${furnitureEffectLabel(effect.kind)}`) }}</span>
            </span>
            <small v-if="!item.installed" class="card-preview">{{ t('game.furniture.afterInstall') }}</small>
          </span>
          <span class="card-footer">
            <span class="card-status" :class="{ running: active(item), depleted: item.installed && !furnitureIsPermanent(item) && remaining(item) <= 0 }"><i />{{ status(item) }}</span>
            <span class="card-service"><Icon :icon="furnitureIsPermanent(item) ? 'mdi:infinity' : 'mdi:clock-outline'" width="14" />{{ serviceSummary(item) }}</span>
          </span>
        </button>
    </div>
    <GameDialog v-model="panelOpen" :title="dialog && selected ? `${t(selected.definition.nameKey)} · ${t(`game.furniture.${action}`)}` : current ? t(current.definition.nameKey) : ''" width="680px" modal-class="furniture-dialog-overlay" :close-on-click-modal="!store.busy" :close-on-press-escape="!store.busy" :show-close="!store.busy">
      <article v-if="current && !dialog" v-loading="store.loading" class="furniture-detail">
        <el-alert v-if="store.errorCode" :title="errorText" type="warning" :closable="false" show-icon />
        <header class="furniture-identity">
          <img :src="art(current)" :alt="t(current.definition.nameKey)" width="88" height="88" />
          <div class="identity-copy"><h4>{{ t(current.definition.nameKey) }} <small v-if="current.installed">Lv.{{ current.definition.level || 1 }}/{{ current.maxLevel || 1 }}</small></h4>
            <p v-for="effect in furnitureEffectGroups(current.definition.rules)" :key="effect.key" class="effect-stat"><strong>{{ effect.rateBps / 100 }}<small>%</small></strong><span>{{ t(`game.furniture.${furnitureEffectLabel(effect.kind)}`) }}</span></p>
          </div>
          <div class="furniture-switch"><span :class="{ running: active(current) }">{{ status(current) }}</span><el-switch v-if="current.installed && !furnitureIsPermanent(current)" :model-value="active(current)" :aria-label="t('game.furniture.enable')" :disabled="locked || !!current.pending || (!active(current) && remaining(current) <= 0)" @change="active(current) ? send(current, 'pause') : open(current, 'enable')" /></div>
        </header>
        <p v-if="current.pending" class="pending-change">{{ t('game.furniture.pendingChange', { seconds: pendingSeconds(current), level: current.pending.definition.level || 1 }) }}</p>
        <p v-if="store.projection?.activationDelaySeconds" class="app-muted transition-note">{{ t('game.furniture.changeDelay', { seconds: store.projection.activationDelaySeconds }) }}</p>
        <p v-if="current.definition.rules.some(r => r.kind === 'action_efficiency_add')" class="training-summary">{{ t('game.furniture.trainingNet', { rate: trainingNet(current) }) }}</p>
        <details class="effect-scope"><summary>{{ t('game.furniture.details') }}</summary><template v-for="effect in furnitureEffectGroups(current.definition.rules)" :key="effect.key"><p>{{ t(`game.furniture.${furnitureEffectDescription(effect.kind)}`, { rate: effect.rateBps / 100 }) }}</p><p class="app-muted">{{ effectScope(effect) }} · {{ t(`game.furniture.${furnitureEffectScope(effect.kind)}`) }}</p></template></details>
        <section v-if="!current.installed && !current.pending" class="detail-band">
          <div class="band-heading"><h5>{{ t('game.furniture.install') }}</h5><span>{{ t('game.furniture.legend') }}</span></div>
          <FurnitureCosts :costs="current.definition.installCosts" :inventory="stock" />
          <p class="app-muted">{{ t(furnitureIsPermanent(current) ? 'game.furniture.permanentInstall' : 'game.furniture.installNotice') }}</p>
          <el-button type="primary" :disabled="locked || !store.projection?.available || !affordable(current.definition.installCosts)" @click="open(current, 'install')"><Icon icon="mdi:hammer" width="18" />{{ t('game.furniture.install') }}</el-button>
        </section>
        <section v-if="!furnitureIsPermanent(current)" class="detail-band">
          <div class="band-heading"><h5>{{ t('game.furniture.maintenance') }}</h5><span>{{ t('game.furniture.perPack', { hours: current.definition.refillSeconds / 3600 }) }}</span></div>
          <div v-if="current.installed" class="service-meter"><div class="service-label"><strong>{{ t('game.furniture.duration', { hours: Math.floor(remaining(current) / 3600000), minutes: Math.floor(remaining(current) / 60000) % 60 }) }}</strong><small>{{ Math.floor(100 * remaining(current) / (current.definition.capacitySeconds * 1000)) }}%</small></div><el-progress :percentage="Math.min(100, remaining(current) / (current.definition.capacitySeconds * 10))" :show-text="false" :stroke-width="6" /></div>
          <FurnitureCosts :costs="current.definition.refillCosts" :inventory="stock" />
          <p class="app-muted">{{ t('game.furniture.chargeNote') }} <span>{{ t('game.furniture.legend') }}</span></p>
          <p class="app-muted">{{ t('game.furniture.upkeep') }}</p>
          <div v-if="current.installed" class="refill-controls"><el-button :disabled="locked || refillLimit(current) < 1" @click="open(current, 'refill')"><Icon icon="mdi:package-variant-plus" width="18" />{{ t('game.furniture.refill') }}</el-button><span v-if="refillLimit(current) < 1" class="app-muted">{{ refillHint(current) }}</span></div>
        </section>
        <section v-if="current.installed && !current.pending" class="detail-band">
          <template v-if="current.upgrade"><div class="band-heading"><h5>{{ t('game.furniture.nextLevel') }} · Lv.{{ current.upgrade.target.level }}</h5><el-button :disabled="locked" @click="open(current, 'upgrade')"><Icon icon="mdi:arrow-up-bold-outline" width="18" />{{ t('game.furniture.upgrade') }}</el-button></div><p v-for="effect in furnitureEffectGroups(current.upgrade.target.rules)" :key="effect.key" class="upgrade-comparison"><span>{{ t(`game.furniture.${furnitureEffectLabel(effect.kind)}`) }}</span><span>{{ furniturePreviousRate(current.definition.rules, effect) / 100 }}% <Icon icon="mdi:arrow-right" width="16" /> <strong>{{ effect.rateBps / 100 }}%</strong></span></p></template>
          <p v-else-if="(current.definition.level || 1) >= (current.maxLevel || 1)" class="app-muted">{{ t('game.furniture.maxLevel') }}</p>
        </section>
        <section v-if="current.installed && current.definition.rules.some(r => r.kind !== 'action_efficiency_add')" class="detail-band"><h5>{{ t('game.furniture.lifetime') }}</h5><FurnitureContributionItems :row="store.projection?.contributions?.[current.definition.id] || {}" /></section>
      </article>
      <div v-if="dialog && selected" class="confirmation-body">
        <el-alert v-if="store.errorCode" :title="errorText" type="warning" :closable="false" show-icon />
        <p v-if="store.projection?.activationDelaySeconds" class="app-muted">{{ t('game.furniture.changeDelay', { seconds: store.projection.activationDelaySeconds }) }}</p>
        <template v-if="action === 'upgrade' && selected.upgrade">
          <p class="upgrade-level">Lv.{{ selected.definition.level || 1 }} <Icon icon="mdi:arrow-right" width="20" /> Lv.{{ selected.upgrade.target.level }}</p>
          <p v-for="effect in furnitureEffectGroups(selected.upgrade.target.rules)" :key="effect.key" class="upgrade-comparison"><span>{{ t(`game.furniture.${furnitureEffectLabel(effect.kind)}`) }}</span><span>{{ furniturePreviousRate(selected.definition.rules, effect) / 100 }}% <Icon icon="mdi:arrow-right" width="16" /> <strong>{{ effect.rateBps / 100 }}%</strong></span></p>
          <p>{{ t(furnitureIsPermanent(selected) ? 'game.furniture.permanentUpgrade' : 'game.furniture.upgradeNote') }}</p>
          <template v-if="Object.keys(selected.upgrade.supplementCosts).length"><h5>{{ t('game.furniture.upgradeCost') }}</h5><FurnitureCosts :costs="selected.upgrade.upgradeCosts" :inventory="stock" /></template>
          <template v-if="Object.keys(selected.upgrade.supplementCosts).length"><h5>{{ t('game.furniture.supplement') }}</h5><FurnitureCosts :costs="selected.upgrade.supplementCosts" :inventory="stock" /></template>
          <div class="confirmation-total"><h5>{{ t('game.furniture.totalCost') }}</h5><FurnitureCosts :costs="costs" :inventory="stock" /></div>
          <template v-if="!furnitureIsPermanent(selected)"><h5>{{ t('game.furniture.newMaintenance') }} · {{ t('game.furniture.perPack', { hours: selected.upgrade.target.refillSeconds / 3600 }) }}</h5><FurnitureCosts :costs="selected.upgrade.target.refillCosts" :inventory="stock" /></template>
          <p class="app-muted">{{ t(furnitureIsPermanent(selected) ? 'game.furniture.permanentQuote' : 'game.furniture.quoteNote') }}</p>
          <p v-if="quoteExpired" class="quote-warning">{{ t('game.furniture.quoteExpired') }}</p>
        </template>
        <template v-else>
          <p v-if="confirmationStale" class="quote-warning">{{ t('game.furniture.quoteExpired') }}</p>
          <p>{{ t(`game.furniture.${action === 'enable' ? 'upkeep' : action === 'install' ? (furnitureIsPermanent(selected) ? 'permanentInstall' : 'installNotice') : 'refillNotice'}`) }}</p>
          <label v-if="action === 'refill'" class="furniture-quantity">{{ t('game.furniture.count') }}<el-input-number v-model="count" :min="1" :max="Math.max(1, refillLimit(selected))" :precision="0" :disabled="store.busy" /></label>
          <FurnitureCosts :costs="costs" :inventory="stock" />
          <p v-if="action !== 'enable' && !furnitureIsPermanent(selected)" class="app-muted">{{ t('game.furniture.addTime', { hours: Math.floor(addedSeconds / 3600), minutes: Math.floor(addedSeconds / 60) % 60 }) }}</p>
        </template>
      </div>
      <template #footer v-if="dialog || store.pending"><el-button v-if="dialog" :disabled="store.busy" @click="dialog = false"><Icon icon="mdi:arrow-left" width="18" />{{ t('game.furniture.backToDetails') }}</el-button><el-button v-if="store.pending" :loading="store.busy" type="primary" @click="reconcile">{{ t('game.furniture.checkResult') }}</el-button><template v-else-if="dialog"><el-button v-if="confirmationStale || (action === 'upgrade' && quoteExpired)" :disabled="locked" @click="refreshQuote"><Icon icon="mdi:refresh" width="18" />{{ t('game.furniture.refreshQuote') }}</el-button><el-button :disabled="!canConfirm" :loading="store.busy" type="primary" @click="confirm">{{ t(`game.furniture.${action}`) }}</el-button></template></template>
    </GameDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@iconify/vue'
import GameDialog from '@components/game/common/GameDialog.vue'
import FurnitureCosts from './FurnitureCosts.vue'
import FurnitureContributionItems from './FurnitureContributionItems.vue'
import { useGameStore } from '@stores/game'
import { useInventoryStore } from '@stores/inventory'
import { useCatteryFurnitureStore } from '@stores/cattery-furniture'
import { furnitureAt, furnitureRemaining, furnitureRefillLimit, furnitureEffectGroups, furniturePreviousRate, furnitureIsPermanent, furnitureIsActive, furnitureEffectLabel, furnitureEffectDescription, furnitureEffectScope, type FurnitureEffectGroup, type FurnitureView, type FurnitureCommand } from '@services/cattery-furniture'

const { t, te } = useI18n()
const props = withDefaults(defineProps<{ active?: boolean }>(), { active: true })
const game = useGameStore(), inventory = useInventoryStore(), store = useCatteryFurnitureStore()
const pulse = ref(0), online = ref(navigator.onLine), panelOpen = ref(false), dialog = ref(false), currentID = ref(''), count = ref(1)
const selected = ref<FurnitureView>(), quoteRevision = ref(0), needsReconfirmation = ref(false)
let viewEpoch = 0
const action = ref<'install' | 'refill' | 'enable' | 'upgrade'>('install')
const now = computed(() => { void pulse.value; return store.serverNow() })
const stock = computed(() => store.projection?.inventory || {})
const views = computed(() => (store.projection?.items || []).map(item => furnitureAt(item, now.value)))
const current = computed(() => views.value.find(x => x.definition.id === currentID.value) || views.value[0])
const cardEffects = computed(() => Object.fromEntries(views.value.map(item => [item.definition.id, furnitureEffectGroups(item.definition.rules)])))
const addedSeconds = computed(() => !selected.value ? 0 : action.value === 'install' ? selected.value.definition.capacitySeconds : selected.value.definition.refillSeconds * count.value)
const locked = computed(() => store.busy || store.loading || !!store.pending || !online.value || store.stale || !!store.projection?.summary)
const errorText = computed(() => t(store.errorCode === 'FURNITURE_PENDING' ? 'game.furniture.pendingError' : te(`game.furniture.errors.${store.errorCode}`) ? `game.furniture.errors.${store.errorCode}` : 'game.furniture.errors.FURNITURE_UNKNOWN'))
const remaining = (item: FurnitureView) => furnitureRemaining(item, now.value)
const active = (item: FurnitureView) => furnitureIsActive(item, now.value)
function serviceSummary(item: FurnitureView) {
  if (item.pending) return t('game.furniture.pendingSeconds', { seconds: pendingSeconds(item) })
  if (furnitureIsPermanent(item)) return t('game.furniture.noMaintenance')
  const seconds = item.installed ? remaining(item) / 1000 : item.definition.capacitySeconds
  return t('game.furniture.shortDuration', { hours: Math.floor(seconds / 3600), minutes: Math.floor(seconds / 60) % 60 })
}
const art = (item: FurnitureView) => `/assets/icons/${item.definition.iconId.replace('.', '/')}.png`
const affordable = (cost: Record<string,number>) => Object.entries(cost).every(([id,n]) => (stock.value[id] || 0) >= n)
const trainingNet = (item: FurnitureView) => (active(item) ? Math.max(0, ...item.definition.rules.filter(r => r.kind === 'action_efficiency_add').map(r => r.rateBps)) / 100 : 0) - 100
function effectScope(effect: FurnitureEffectGroup) {
  const parts = [effect.domains.filter(domain => domain !== 'action_start').map(domain => t(`game.furniture.domains.${domain}`)).join(' / ')].filter(Boolean)
  for (const [field, prefix] of [['actionIds', 'action'], ['mainTypes', 'action_main_type'], ['subTypes', 'action_sub_type']] as const) {
    const ids = effect.selector[field] || []
    if (ids.length) parts.push(ids.map(id => te(`${prefix}.${id}.name`) ? t(`${prefix}.${id}.name`) : id).join(' / '))
  }
  return parts.join('; ')
}
const pendingSeconds = (item: FurnitureView) => Math.max(0, Math.ceil((Date.parse(item.pending?.effectiveAt || '') - now.value) / 1000))
const status = (item: FurnitureView) => t(`game.furniture.${item.pending ? 'pendingStatus' : !item.installed ? 'notInstalled' : furnitureIsPermanent(item) ? 'permanentActive' : remaining(item) <= 0 ? 'empty' : active(item) ? 'active' : 'paused'}`)
const refillLimit = (item: FurnitureView) => furnitureRefillLimit(item, stock.value, now.value)
function refillHint(item: FurnitureView) {
  if (item.pending) return t('game.furniture.pendingStatus')
  if (item.refillAllowed === false) return t('game.furniture.versionNotice')
  const gap = item.definition.refillSeconds * 1000 - (item.definition.capacitySeconds * 1000 - remaining(item))
  return gap > 0 ? t('game.furniture.full', { hours: Math.ceil(gap / 360000) / 10 }) : t('game.furniture.errors.FURNITURE_MATERIALS')
}
const costs = computed(() => action.value === 'upgrade' ? selected.value?.upgrade?.totalCosts || {} : Object.fromEntries(Object.entries(!selected.value || action.value === 'enable' ? {} : action.value === 'install' ? selected.value.definition.installCosts : selected.value.definition.refillCosts).map(([id,n]) => [id, n * (action.value === 'refill' ? count.value : 1)])))
const confirmationStale = computed(() => needsReconfirmation.value || quoteRevision.value !== store.projection?.revision)
const quoteExpired = computed(() => !selected.value?.upgrade || Date.parse(selected.value.upgrade.expiresAt) <= now.value || confirmationStale.value)
const canConfirm = computed(() => !!selected.value && !locked.value && !confirmationStale.value && affordable(costs.value) && (action.value !== 'upgrade' || !quoteExpired.value) && (action.value !== 'refill' || (count.value > 0 && count.value <= refillLimit(selected.value!))))
async function refresh(force: boolean | Event = false) { online.value = navigator.onLine; if (props.active && online.value && game.token) await store.refresh(game.token, { force: force === true, details: panelOpen.value }) }
async function showDetails(item: FurnitureView) { viewEpoch++; currentID.value = item.definition.id; dialog.value = false; panelOpen.value = true; await refresh(true) }
async function open(item: FurnitureView, next: typeof action.value) {
  const token = game.token
  const epoch = viewEpoch
  await refresh(true)
  if (token !== game.token || epoch !== viewEpoch || !panelOpen.value || locked.value) return
  const fresh = store.projection?.items.find(x => x.definition.id === item.definition.id)
  if (!fresh || fresh.pending || (next === 'upgrade' && !fresh.upgrade)) return
  // Confirm the captured price and revision, never silently replace an open quote.
  selected.value = JSON.parse(JSON.stringify(fresh)); quoteRevision.value = store.projection!.revision
  needsReconfirmation.value = false; store.errorCode = ''
  action.value = next; count.value = 1; dialog.value = true; panelOpen.value = true
}
async function refreshQuote() { if (selected.value) await open(selected.value, action.value) }
async function afterSubmit(ok: boolean, token: string, action: FurnitureCommand['action']) {
  if (token !== game.token) return
  if (ok) { dialog.value = false; if (['install','refill','upgrade'].includes(action)) await inventory.initialize(token, true).catch(() => undefined) }
  else if (!store.pending) { needsReconfirmation.value = store.errorCode === 'FURNITURE_CONFLICT'; await refresh() }
}
async function send(item: FurnitureView, next: FurnitureCommand['action']) {
  if (locked.value || !store.projection || item.pending) return
  const token = game.token
  const quote = next === 'upgrade' ? item.upgrade : undefined
  const ok = await store.submit({ requestId: crypto.randomUUID(), furnitureId: item.definition.id, version: item.definition.version, revision: next === 'pause' ? store.projection.revision : quoteRevision.value, action: next, count: next === 'refill' ? count.value : 0, ...(quote ? { upgradeQuote: { token: quote.token, expiresAt: quote.expiresAt, remainingMs: quote.remainingMs } } : {}) })
  await afterSubmit(ok, token, next)
}
async function confirm() { if (canConfirm.value && selected.value) await send(selected.value, action.value) }
async function reconcile() { if (store.pending) { const token = game.token, command = store.pending; await afterSubmit(await store.submit(command), token, command.action) } }
function visible() { if (document.visibilityState === 'visible') void refresh() }
watch(panelOpen, open => { if (!open) viewEpoch++ }, { flush: 'sync' })
watch(() => (store.projection?.items || []).filter(item => item.pending && Date.parse(item.pending.effectiveAt) <= now.value).map(item => `${item.definition.id}:${item.pending!.effectiveAt}`).join(','), due => { if (due) void refresh(true) })
watch(() => game.token, token => { panelOpen.value = false; dialog.value = false; selected.value = undefined; currentID.value = ''; store.bind(token || ''); void refresh() }, { immediate: true })
watch(() => props.active, active => { if (active) void refresh() })
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { if (props.active && document.visibilityState === 'visible') pulse.value++ }, 1000); window.addEventListener('focus', refresh); window.addEventListener('online', refresh); window.addEventListener('offline', refresh); document.addEventListener('visibilitychange', visible) })
onBeforeUnmount(() => { clearInterval(timer); window.removeEventListener('focus', refresh); window.removeEventListener('online', refresh); window.removeEventListener('offline', refresh); document.removeEventListener('visibilitychange', visible) })
</script>

<style scoped>
.furniture-section { container-type:inline-size; display:grid; gap:16px; min-width:0; }
.furniture-header,.band-heading,.service-label,.furniture-quantity { display:flex; align-items:center; justify-content:space-between; gap:12px; flex-wrap:wrap; }
h3,h4,h5,p { margin:0; }
h3,h4 { font-size:16px; font-weight:650; }
h5 { font-size:14px; font-weight:600; }
.furniture-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:14px; }
.furniture-card { display:flex; flex-direction:column; gap:16px; min-width:0; padding:18px; text-align:start; color:var(--app-text); background:var(--el-bg-color); border:1px solid var(--app-border); border-radius:8px; cursor:pointer; font:inherit; transition:border-color .15s,box-shadow .15s; }
.furniture-card:hover { border-color:var(--el-color-primary-light-5); box-shadow:0 2px 8px var(--el-color-primary-light-9); }
.furniture-card:focus-visible { outline:2px solid var(--el-color-primary); outline-offset:2px; }
.card-heading { display:flex; align-items:center; gap:12px; width:100%; min-width:0; }
.card-heading img { flex-shrink:0; object-fit:contain; }
.card-name { display:grid; gap:6px; flex:1; min-width:0; }
.card-name>strong { font-size:15px; line-height:1.5; overflow-wrap:anywhere; }
.card-name>small { font-size:12px; font-variant-numeric:tabular-nums; }
.card-name>small>span,.card-chevron { color:var(--app-muted); }
.card-chevron { flex-shrink:0; }
.card-effects { display:grid; gap:4px; margin-block:auto; }
.card-effect { display:flex; align-items:baseline; gap:8px; flex-wrap:wrap; }
.card-effect>strong { font-size:24px; line-height:1.25; color:var(--el-color-primary); font-variant-numeric:tabular-nums; }
.card-effect>strong>small { font-size:14px; margin-left:2px; }
.card-effect>span { font-size:12px; line-height:1.5; overflow-wrap:anywhere; }
.card-preview { color:var(--app-muted); font-size:11px; }
.card-footer { display:flex; align-items:center; justify-content:space-between; gap:8px; flex-wrap:wrap; padding-top:12px; border-top:1px solid var(--app-border); color:var(--app-muted); font-size:12px; line-height:1.5; }
.card-status,.card-service { display:flex; align-items:center; gap:5px; min-width:0; overflow-wrap:anywhere; }
.card-status i { width:5px; height:5px; border-radius:50%; background:currentColor; flex-shrink:0; }
.card-service :deep(svg) { flex-shrink:0; }
.depleted { color:var(--el-color-warning); }
.furniture-detail { min-width:0; }
.furniture-detail>.el-alert { margin-bottom:12px; }
.furniture-identity { display:flex; align-items:center; gap:16px; min-width:0; }
.furniture-identity>img { flex-shrink:0; object-fit:contain; }
.identity-copy { flex:1; min-width:0; }
.identity-copy h4 { overflow-wrap:anywhere; }
.identity-copy h4 small { color:var(--app-muted); font-size:12px; white-space:nowrap; }
.effect-stat { display:flex; align-items:baseline; gap:10px; flex-wrap:wrap; margin-top:8px; }
.effect-stat>strong { font-size:28px; color:var(--el-color-primary); font-variant-numeric:tabular-nums; line-height:1.2; }
.effect-stat strong small { font-size:16px; margin-left:2px; }
.effect-stat>span { font-size:13px; }
.furniture-switch { display:flex; flex-direction:column; align-items:center; flex-shrink:0; font-size:12px; gap:2px; color:var(--app-muted); }
.running { color:var(--el-color-success); }
.training-summary { font-size:13px; line-height:1.6; margin-top:12px; color:var(--app-muted); }
.effect-scope { padding:12px 0 16px; font-size:12px; line-height:1.6; }
.effect-scope summary { cursor:pointer; color:var(--app-muted); width:fit-content; }
.effect-scope p { margin-top:8px; }
.detail-band { padding:16px 0; border-top:1px solid var(--app-border); display:grid; gap:12px; }
.detail-band>button { justify-self:start; }
.band-heading>span,.app-muted,.service-label small { font-size:12px; line-height:1.6; }
.band-heading>span { color:var(--app-muted); }
.service-meter { max-width:400px; display:grid; gap:8px; }
.service-label strong { font-size:13px; font-weight:500; font-variant-numeric:tabular-nums; }
.refill-controls { display:flex; align-items:center; gap:12px; flex-wrap:wrap; }
.upgrade-comparison { display:flex; justify-content:space-between; align-items:center; gap:12px; font-size:13px; flex-wrap:wrap; }
.upgrade-comparison>span:last-child { display:flex; align-items:center; gap:8px; white-space:nowrap; }
.upgrade-comparison strong { color:var(--el-color-success); }
.furniture-header :deep(.el-button) { width:36px; height:36px; padding:0; }
.furniture-detail :deep(.el-button) { min-height:36px; height:auto; white-space:normal; }
.furniture-detail :deep(.el-button>span) { gap:6px; }
.confirmation-body { display:grid; gap:14px; min-width:0; line-height:1.6; }
.confirmation-body h5 { margin:4px 0 0; }
.confirmation-body p { overflow-wrap:anywhere; }
.upgrade-level { display:flex; align-items:center; gap:12px; font-size:18px; font-weight:600; }
.confirmation-total { border-top:1px solid var(--app-border); border-bottom:1px solid var(--app-border); padding:16px 0; display:grid; gap:12px; }
.quote-warning { color:var(--el-color-warning); }
.pending-change { color:var(--el-color-warning); font-size:13px; line-height:1.6; margin-top:12px; }
.transition-note { margin-top:10px; }
@container (max-width:540px) {
  .furniture-grid { gap:10px; }
  .furniture-card { padding:12px; gap:12px; }
  .card-heading { flex-wrap:wrap; gap:6px; position:relative; }
  .card-heading img { width:60px; height:60px; }
  .card-name { flex-basis:100%; gap:3px; }
  .card-name>strong { font-size:14px; }
  .card-chevron { position:absolute; top:0; right:0; }
  .card-effect { flex-direction:column; gap:3px; }
  .card-footer { align-items:flex-start; flex-direction:column; gap:4px; padding-top:10px; font-size:11px; }
}
@media (max-width:600px) {
  .furniture-identity { gap:10px; flex-wrap:wrap; }
  .furniture-identity>img { width:60px; height:60px; }
  .furniture-switch { flex-direction:row; justify-content:space-between; width:100%; }
  .effect-stat { gap:6px; }
  .effect-stat>strong { font-size:24px; }
  .effect-stat>span { font-size:12px; }
  .furniture-detail :deep(.el-button) { min-height:44px; }
}
@media (prefers-reduced-motion:reduce) { .furniture-card { transition:none; } }
</style>

<style>
.furniture-dialog-overlay .el-dialog { max-width:calc(100vw - 24px); }
.furniture-dialog-overlay .el-dialog__footer { display:flex; gap:8px; flex-wrap:wrap; justify-content:flex-end; }
.furniture-dialog-overlay .el-dialog__footer .el-button { margin:0; min-height:36px; height:auto; white-space:normal; }
.furniture-dialog-overlay .el-dialog__footer .el-button>span { gap:6px; }
@media (max-width:600px) { .furniture-dialog-overlay .el-dialog__footer .el-button { min-height:44px; } }
</style>
