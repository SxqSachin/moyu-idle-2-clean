<template>
  <section class="instance-market" data-testid="instance-market">
    <header v-if="view !== 'listings'" class="variant-toolbar">
      <div v-if="!history" class="variant-heading">
        <h3>{{ t('game.instanceMarket.variants') }}</h3>
      </div>
      <div class="variant-tools">
        <el-tooltip v-if="history" :content="t('game.instanceMarket.refresh')"><el-button circle :disabled="loading || busy" :aria-label="t('game.instanceMarket.refresh')" @click="refresh"><Icon icon="lucide:refresh-cw" /></el-button></el-tooltip>
        <el-button v-if="!history" type="primary" :disabled="busy || !!pending" @click="openList"><Icon icon="lucide:plus" />{{ t('game.marketCreateListing') }}</el-button>
      </div>
    </header>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-alert v-if="pending" :title="t('game.instanceMarket.pending')" type="warning" :closable="false" />
    <el-button v-if="pending && !listingOpen && !buyTarget" :loading="busy" @click="retryPending"><Icon icon="lucide:rotate-cw" />{{ t('game.instanceMarket.retry') }}</el-button>
    <div v-loading="loading" class="variant-results">
      <div v-if="view !== 'listings' && !loading && !error && !rows.length" class="variant-empty">{{ t('game.instanceMarket.empty') }}</div>
      <div v-if="view === 'groups'" class="variant-grid">
        <article v-for="group in groups" :key="group.groupKey" class="variant-card" @click="openGroup(group)">
          <div class="variant-item"><Card :item-id="group.itemId" :instance-preview="group.snapshot" hide-label hide-border :preview-enabled="false" :width="40" :height="40" /><span>{{ itemName(group.itemId, group.snapshot) }}</span></div>
          <InstanceMarketFields :fields="group.fields" />
          <div class="variant-stock"><span>{{ t('game.instanceMarket.quantity', { count: group.quantity }) }}</span><span>{{ t('game.instanceMarket.sellers', { count: group.sellerCount }) }}</span></div>
          <div class="variant-price"><span>{{ t('game.instanceMarket.lowest') }}</span><QuickRef item-id="gold_coin" :count="group.lowestPrice" normal-number-format /><el-button text :aria-label="t('game.instanceMarket.choose')" @click.stop="openGroup(group)"><Icon icon="lucide:chevron-right" /></el-button></div>
        </article>
      </div>
      <el-card v-else-if="view === 'listings'" class="app-card variant-listing-panel" shadow="never">
        <template #header>
          <div class="listing-panel-heading">
            <div class="variant-heading">
              <el-tooltip :content="t('game.instanceMarket.back')"><el-button text type="primary" :aria-label="t('game.instanceMarket.back')" @click="back"><Icon icon="lucide:arrow-left" /></el-button></el-tooltip>
              <span class="font-medium">{{ t('game.marketPublicListings') }}</span>
            </div>
            <el-button size="small" type="primary" :disabled="busy || !!pending" @click="openList">{{ t('game.marketCreateListing') }}</el-button>
          </div>
        </template>
        <div v-if="selectedGroup" class="selected-variant">
          <strong class="effect-heading">{{ t('game.itemDetailEffect') }}</strong>
          <InstanceMarketFields :fields="selectedGroup.fields" />
        </div>
      <el-table :data="listings" size="small" border>
        <el-table-column :label="t('game.marketQuantity')" prop="quantity" :width="isMobile ? 48 : 90" />
        <el-table-column :label="t('game.marketPrice')" :width="isMobile ? 112 : 160"><template #default="{ row }"><QuickRef item-id="gold_coin" :count="row.price" normal-number-format /></template></el-table-column>
        <el-table-column :label="t('game.marketAction')" :min-width="isMobile ? 88 : 120">
          <template #default="{ row }">
            <div class="offer-actions">
              <el-button v-if="row.isOwn" size="small" type="warning" :disabled="busy || !!pending" @click="cancelListing(row)">{{ t('game.marketCancelListingAction') }}</el-button>
              <el-button v-else-if="row.id === bestBuyListing?.id" size="small" type="success" :disabled="busy || !!pending" @click="openBuy(row)">{{ t('game.marketBuyAction') }}</el-button>
              <el-tooltip v-else-if="bestBuyListing || offset > 0" :content="t('game.instanceMarket.buyFirst')"><el-button text type="primary" :disabled="busy || !!pending" :aria-label="t('game.instanceMarket.buyFirst')" @click="buyFirst"><Icon icon="lucide:arrow-up" /></el-button></el-tooltip>
              <el-button v-else size="small" disabled>{{ t('game.marketBuyAction') }}</el-button>
              <el-tooltip :content="t('game.instanceMarket.details')"><el-button text :aria-label="t('game.instanceMarket.details')" @click="showDetails(row.id, row.itemId)"><Icon icon="lucide:scan-eye" /></el-button></el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>
      </el-card>
      <div v-else-if="view === 'mine'" class="variant-offers">
        <article v-for="row in listings" :key="row.id" class="variant-offer">
          <div class="offer-owner">
            <div class="record-heading"><QuickRef :item-id="row.itemId" /><el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">{{ t(`game.marketStatus${row.status === 'active' ? 'Active' : row.status === 'sold' ? 'Sold' : 'Cancelled'}`) }}</el-tag></div>
            <div class="record-meta"><span>{{ t('game.instanceMarket.unitPrice') }}</span><QuickRef item-id="gold_coin" :count="row.price" normal-number-format /><span>{{ t('game.instanceMarket.amount') }} {{ row.quantity }} / {{ row.initialQuantity }}</span></div>
          </div>
          <div class="offer-actions">
            <el-tooltip :content="t('game.instanceMarket.details')"><el-button circle :aria-label="t('game.instanceMarket.details')" @click="showDetails(row.id, row.itemId)"><Icon icon="lucide:scan-eye" /></el-button></el-tooltip>
            <el-button v-if="row.status === 'active'" size="small" type="warning" plain :disabled="busy || !!pending" @click="cancelListing(row)">{{ t('game.marketCancelListingAction') }}</el-button>
            <el-button size="small" @click="$emit('view-item', row.itemId)">{{ t('game.marketViewItem') }}</el-button>
          </div>
        </article>
      </div>
      <div v-else class="variant-offers">
        <article v-for="order in orders" :key="order.id" class="variant-offer">
          <div class="offer-owner"><QuickRef :item-id="order.itemId" :count="order.quantity" /><small>{{ new Date(order.createdAt).toLocaleString() }}</small></div>
          <div class="record-meta"><span>{{ t('game.instanceMarket.total') }}</span><QuickRef item-id="gold_coin" :count="order.total" normal-number-format /></div>
          <el-button size="small" @click="showDetails(order.listingId, order.itemId)">{{ t('game.instanceMarket.details') }}</el-button>
        </article>
      </div>
    </div>
    <footer v-if="offset > 0 || hasMore" class="variant-pager">
      <el-button :disabled="loading || offset === 0" @click="offset -= 30"><Icon icon="lucide:chevron-left" />{{ t('game.instanceMarket.previous') }}</el-button>
      <span>{{ t('game.instanceMarket.page', { page: offset / 30 + 1 }) }}</span>
      <el-button :disabled="loading || !hasMore" @click="offset += 30">{{ t('game.instanceMarket.more') }}<Icon icon="lucide:chevron-right" /></el-button>
    </footer>

    <GameDialog v-model="listingOpen" :title="t('game.marketCreateListing')" width="760px" :close-on-press-escape="!busy && !pending" :close-on-click-modal="!busy && !pending" :show-close="!busy && !pending">
      <template v-if="!quoteGroups.length">
        <div class="variant-toolbar"><strong>{{ t('game.instanceMarket.select') }}</strong><span>{{ t('game.instanceMarket.selection', { count: selectedIds.length }) }}</span></div>
        <div v-if="!eligible.length" class="variant-empty">{{ t('game.instanceMarket.noInventory') }}</div>
        <div class="inventory-selection inventory-selection--picker">
          <div
            v-for="item in visibleInventory" :key="item.instanceId"
            class="batch-card-wrapper" role="checkbox" tabindex="0"
            :class="{ 'batch-card-wrapper--selected': selectedIds.includes(item.instanceId), 'batch-card-wrapper--unselectable': selectionDisabled(item.instanceId) }"
            :aria-checked="selectedIds.includes(item.instanceId)"
            :aria-disabled="selectionDisabled(item.instanceId)"
            :aria-label="`${t('game.instanceMarket.choose')} ${itemName(item.templateItemId, item)}`"
            @click="selectInstance(item.instanceId)"
            @keydown.space.prevent.stop="selectInstance(item.instanceId)"
            @keydown.enter.prevent.stop="selectInstance(item.instanceId)"
          >
            <span v-if="selectedIds.includes(item.instanceId)" class="batch-card-badge" aria-hidden="true"><Icon icon="lucide:check" /></span>
            <Card :item-id="item.templateItemId" :instance-preview="item" />
          </div>
        </div>
        <div v-if="eligible.length > 40" class="variant-pager">
          <el-button :disabled="inventoryOffset === 0" @click="inventoryOffset -= 40"><Icon icon="lucide:chevron-left" /></el-button>
          <span>{{ t('game.instanceMarket.page', { page: inventoryOffset / 40 + 1 }) }}</span>
          <el-button :disabled="inventoryOffset + 40 >= eligible.length" @click="inventoryOffset += 40"><Icon icon="lucide:chevron-right" /></el-button>
        </div>
      </template>
      <template v-else>
        <el-select v-if="quoteGroups.length > 1" v-model="quoteGroup" :disabled="busy || !!pending" class="group-select" :aria-label="t('game.instanceMarket.chooseGroup')">
          <el-option v-for="(group, index) in quoteGroups" :key="group[0].groupKey" :value="index" :label="`${index + 1}. ${t('game.instanceMarket.selectedGroup', { count: group.length })}`" />
        </el-select>
        <div class="dialog-item"><Card :item-id="itemId" :instance-preview="chosenQuotes[0]?.snapshot" hide-label :width="48" :height="48" /><strong>{{ itemName(itemId, chosenQuotes[0]?.snapshot) }}</strong></div>
        <div class="dialog-effects"><strong class="effect-heading">{{ t('game.itemDetailEffect') }}</strong><InstanceMarketFields :fields="chosenQuotes[0]?.fields || []" /></div>
        <div class="listing-inputs">
          <label>{{ t('game.instanceMarket.unitPrice') }}<el-input-number v-model="price" :min="minimumPrice" :max="1000000000000" :step="priceStep" :precision="0" :disabled="busy || !!pending" @change="normalizePrice" /></label>
          <label>{{ t('game.instanceMarket.amount') }}<el-input-number v-model="quantity" :min="1" :max="chosenQuotes.length" :precision="0" :disabled="busy || !!pending" /></label>
        </div>
        <div class="variant-total">{{ t('game.instanceMarket.total') }} <QuickRef item-id="gold_coin" :count="price * quantity" normal-number-format /></div>
        <small>{{ t('game.instanceMarket.fee') }}</small>
      </template>
      <template #footer>
        <el-button :disabled="busy || !!pending" @click="listingOpen = false">{{ t('game.instanceMarket.cancel') }}</el-button>
        <el-button v-if="!quoteGroups.length" type="primary" :loading="busy" :disabled="!selectedIds.length" @click="preview">{{ t('game.instanceMarket.preview') }}</el-button>
        <el-button v-else type="primary" :loading="busy" :disabled="!pending && !canList" @click="confirmList">{{ t('game.instanceMarket.confirm') }}</el-button>
      </template>
    </GameDialog>
    <GameDialog :model-value="!!buyTarget" :title="t('game.marketBuyAction')" width="460px" :close-on-press-escape="!busy && !pending" :show-close="!busy && !pending" :close-on-click-modal="!busy && !pending" @update:model-value="!$event && !busy && !pending && (buyTarget = null)">
      <template v-if="buyTarget">
        <div class="dialog-item"><Card :item-id="buyTarget.itemId" :instance-preview="selectedGroup?.snapshot" hide-label :width="48" :height="48" /><strong>{{ itemName(buyTarget.itemId, selectedGroup?.snapshot) }}</strong></div>
        <div class="dialog-effects"><strong class="effect-heading">{{ t('game.itemDetailEffect') }}</strong><InstanceMarketFields :fields="buyTarget.fields" /></div>
        <div class="variant-total"><span>{{ t('game.instanceMarket.unitPrice') }}</span><QuickRef item-id="gold_coin" :count="buyTarget.price" normal-number-format /></div>
        <div class="listing-inputs"><label>{{ t('game.instanceMarket.amount') }}<el-input-number v-model="buyQuantity" :min="1" :max="buyTarget.quantity" :precision="0" :disabled="busy || !!pending" /></label></div>
        <div class="variant-total">{{ t('game.instanceMarket.total') }} <QuickRef item-id="gold_coin" :count="buyQuantity * buyTarget.price" normal-number-format /></div>
      </template>
      <template #footer><el-button :disabled="busy || !!pending" @click="buyTarget = null">{{ t('game.instanceMarket.cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!pending && !canBuy" @click="confirmBuy">{{ t('game.instanceMarket.buy') }}</el-button></template>
    </GameDialog>
    <GameDialog v-model="detailOpen" :title="t('game.instanceMarket.details')" width="680px">
      <div class="inventory-selection"><div v-for="detail in details" :key="detail.instanceId" class="inventory-option"><Card :item-id="detailItemId" :instance-preview="detail.snapshot" /></div></div>
    </GameDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, toRef, watch } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import Card from "@components/game/item/Card.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import InstanceMarketFields from "./InstanceMarketFields.vue"
import { useInstanceMarket } from "@/composables/game/market/useInstanceMarket"
import { useDevice } from "@composables/game/useDevice"
import { getLocalizedItemName } from "@services/item-label"
import { getItemRealValuePrice, getMarketPriceStep, normalizeMarketPrice } from "@services/market-pricing"
import type { MarketGroup, MarketListing, MarketOrder } from "@services/instance-market"
import { marketCatalogForInstance, marketCatalogName } from '@services/market-catalog'
const props = withDefaults(defineProps<{ itemId?: string; history?: boolean; historyView?: 'mine' | 'orders'; refreshVersion?: number; catalogValues?: string }>(), { itemId: "", history: false, historyView: 'mine', refreshVersion: 0, catalogValues: '' })
defineEmits<{ (e: 'view-item', itemId: string): void }>()
const { t, locale } = useI18n()
const { isMobile } = useDevice()
const { view, selectedGroup, rows, offset, hasMore, loading, error, listingOpen, selectedIds, inventoryOffset, quoteGroups, quoteGroup, quantity, price, buyTarget, buyQuantity, details, detailOpen, detailItemId, busy, pending, eligible, visibleInventory, chosenQuotes, canList, canBuy, bestBuyListing, buyFirst, refresh, openGroup, back, toggle, openList, preview, confirmList, openBuy, confirmBuy, cancelListing, retryPending, showDetails } = useInstanceMarket(toRef(props, "itemId"), toRef(props, "history"), toRef(props, "historyView"), toRef(props, 'catalogValues'))
const groups = computed(() => rows.value as MarketGroup[])
const listings = computed(() => rows.value as MarketListing[])
const orders = computed(() => rows.value as MarketOrder[])
const minimumPrice = computed(() => getItemRealValuePrice(props.itemId))
const priceStep = computed(() => getMarketPriceStep(price.value).step)
function normalizePrice(value: number | undefined, old: number | undefined) {
  price.value = normalizeMarketPrice(value || minimumPrice.value, minimumPrice.value, Number(value) < Number(old) ? "down" : "up")
}
const itemName = (id: string, snapshot?: unknown) => {
  const entry = marketCatalogForInstance(id, snapshot)
  return entry ? marketCatalogName(entry, locale.value) : getLocalizedItemName(t, id)
}
const selectionDisabled = (id: number) => busy.value || !!pending.value || (selectedIds.value.length >= 100 && !selectedIds.value.includes(id))
function selectInstance(id: number) {
  if (!selectionDisabled(id)) toggle(id, !selectedIds.value.includes(id))
}
watch(() => props.refreshVersion, () => { void refresh() })
</script>

<style scoped>
.instance-market { min-width: 0; color: var(--el-text-color-primary); }
.variant-toolbar,.variant-tools,.variant-heading,.variant-item,.variant-stock,.variant-price,.offer-actions,.variant-pager { display:flex; align-items:center; gap:10px; }
.variant-toolbar { justify-content:space-between; margin-bottom:16px; flex-wrap:wrap; }
h3 { font-size:16px; margin:0; }
.variant-results { min-height:180px; }
.variant-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(min(100%,270px),1fr)); gap:12px; }
.variant-card { display:flex; flex-direction:column; gap:14px; padding:16px; border:1px solid var(--el-border-color); background:var(--el-bg-color); border-radius:8px; min-width:0; cursor:pointer; }
.variant-card:hover { border-color:var(--el-color-primary); }
.variant-item { font-size:14px; font-weight:600; }
.variant-stock { margin-top:auto; justify-content:space-between; color:var(--el-text-color-secondary); font-size:12px; flex-wrap:wrap; }
.variant-price { display:grid; grid-template-columns:minmax(0,1fr) auto; width:100%; border-top:1px solid var(--el-border-color-lighter); padding:12px 0 0; color:var(--el-text-color-regular); font-size:12px; text-align:left; }
.variant-price > :deep(.el-tag) { justify-self:start; }
.variant-price > .el-button { grid-column:2; grid-row:1 / span 2; }
.variant-offers { display:grid; gap:8px; }
.variant-offer { display:flex; align-items:center; gap:12px; padding:12px; border:1px solid var(--app-border); border-radius:4px; flex-wrap:wrap; min-width:0; }
.offer-owner { flex:1; min-width:120px; }
.offer-numbers { display:grid; gap:5px; font-size:13px; font-variant-numeric:tabular-nums; }
.offer-actions { margin-left:auto; }
small { display:block; color:var(--el-text-color-secondary); font-size:12px; margin-top:5px; }
.variant-empty { padding:48px 8px; text-align:center; color:var(--el-text-color-secondary); }
.variant-pager { justify-content:center; margin-top:20px; font-size:12px; flex-wrap:wrap; }
.selected-variant { max-width:620px; padding:0 0 16px; }
.listing-panel-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; }
.variant-listing-panel .offer-actions { justify-content:flex-start; flex-wrap:wrap; margin:0; gap:4px; }
.variant-listing-panel .offer-actions :deep(.el-button + .el-button) { margin-left:0; }
@media(max-width:480px) { .variant-listing-panel :deep(.el-card__body), .variant-listing-panel :deep(.el-card__header) { padding:12px; } }
.inventory-selection { display:grid; grid-template-columns:repeat(auto-fill,minmax(100px,1fr)); gap:10px; max-height:420px; overflow:auto; }
.inventory-option { position:relative; display:flex; gap:6px; align-items:center; justify-content:center; padding:12px 6px; min-width:0; font-size:12px; }
.inventory-selection--picker { padding:8px; gap:18px 10px; }
.batch-card-wrapper:focus-visible { outline:2px solid var(--el-color-primary); outline-offset:4px; }
.dialog-item,.record-heading,.record-meta { display:flex; align-items:center; gap:8px; flex-wrap:wrap; min-width:0; }
.dialog-item { margin-bottom:16px; font-size:14px; }
.dialog-effects { padding:12px 0; margin-bottom:16px; border-block:1px solid var(--app-border); }
.effect-heading { display:block; font-size:12px; margin-bottom:8px; }
.record-meta { margin-top:6px; font-size:12px; color:var(--el-text-color-secondary); }
.variant-tools { margin-left:auto; }
.variant-total { align-items:center; flex-wrap:wrap; }
.listing-inputs { display:flex; gap:16px; flex-wrap:wrap; margin:22px 0; }
.listing-inputs label { display:grid; gap:8px; font-size:13px; max-width:100%; }
.variant-total { display:flex; gap:12px; justify-content:space-between; font-size:15px; }
.group-select { width:100%; margin-bottom:18px; }
@media(max-width:480px) { .variant-card { padding:12px; } .variant-offer { gap:12px; } .offer-actions { width:100%; justify-content:flex-end; flex-wrap:wrap; } .listing-inputs label { flex:1; } .listing-inputs :deep(.el-input-number) { width:100%; min-width:130px; } }
</style>
<style scoped src="../../../../components/game/item/batch-card-selection.css"></style>
