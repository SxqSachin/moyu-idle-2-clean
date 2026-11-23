<template>
  <div class="page-shell">
    <GamePageFrame
      title-key="game.menu.market"
      tutorial-id="market"
      :visual-src="marketVisual.src"
      :visual-tone="marketVisual.tone"
      visual-key="market"
      data-tutorial="market-page"
    >
      <el-tabs v-model="marketTab" data-tutorial="market-tabs" @tab-change="handleTabChange">
        <el-tab-pane :label="t('game.marketCatalogTab')" name="catalog" lazy>
          <div
            class="space-y-3"
            data-tutorial="market-catalog"
            style="min-height: 240px"
            v-loading="!catalogReady"
          >
            <ItemBrowserPanel
              v-if="catalogReady"
              :items="catalogBrowserItems"
              :empty-text="t('game.marketCatalogEmpty')"
              :search-placeholder="t('game.marketCatalogSearchPlaceholder')"
              search-width-class="md:w-32!"
              :all-label="t('game.marketCatalogCategoryAll')"
              :grid-min-width="64"
              @item-click="handleCatalogItemClick"
            >
              <template #item="{ item, onClick }">
                <MarketCatalogItem v-if="catalogEntryByKey.get(item.key)" :entry="catalogEntryByKey.get(item.key)!" @click="onClick" />
                <Card v-else :item-id="item.itemId" @click="onClick" />
              </template>
            </ItemBrowserPanel>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('game.marketTradeTab')" name="trade" lazy>
          <div class="space-y-3" data-tutorial="market-trade">
            <div class="flex items-center justify-between gap-2">
              <div @click="backToCatalog" class="text-blue-600 text-sm flex items-center cursor-pointer gap-1 shrink-0 whitespace-nowrap">
                <el-icon class="cursor-pointer"><Icon icon="ep:arrow-left"></Icon></el-icon>
                {{ t("game.marketBackToCatalog") }}
              </div>
              <div v-if="listingFilterItemId" class="text-sm app-muted min-w-0 flex items-center gap-1">
                <span class="text-xs">{{ t("game.marketSelectedItem") }}: </span>
                <MarketCatalogItem v-if="selectedCatalogEntry" :entry="selectedCatalogEntry" compact />
                <QuickRef v-else :item-id="listingFilterItemId"></QuickRef>
              </div>
              <div class="shrink-0 flex justify-end">
                <el-button
                  v-if="listingFilterItemId"
                  size="small"
                  :disabled="marketRefreshCoolingDown"
                  @click="refreshMarketView"
                >
                  {{ t("game.marketRefresh") }}
                </el-button>
              </div>
            </div>
            <!-- <div class="flex flex-wrap items-center gap-2">
              <el-select v-model="listingFilterItemId" clearable :placeholder="t('game.marketFilterByItem')" style="max-width: 260px">
                <el-option v-for="itemId in filterItemOptions" :key="itemId" :label="itemLabel(itemId)" :value="itemId" />
              </el-select>
              <el-input v-model="searchKeyword" :placeholder="t('game.marketSearchPlaceholder')" style="max-width: 220px" clearable />
              <el-button @click="loadPlayerMarketData">{{ t("game.marketRefresh") }}</el-button>
            </div> -->

            <ItemBrowserPanel v-if="instanceCatalogPending" :items="tradeCatalogItems" :empty-text="t('game.marketCatalogEmpty')" :grid-min-width="64" @item-click="handleCatalogItemClick">
              <template #item="{ item, onClick }"><MarketCatalogItem :entry="catalogEntryByKey.get(item.key)!" @click="onClick" /></template>
            </ItemBrowserPanel>
            <InstanceMarketPanel v-else-if="instanceMode" :key="listingFilterItemId" :item-id="listingFilterItemId" :catalog-values="listingCatalogValues" :refresh-version="instanceRefresh" />
            <details v-if="instanceMode && (filteredPublicListings.length || filteredPublicBuyOrders.length)">
              <summary>{{ t('game.instanceMarket.legacy') }}</summary>
              <MarketPublicListingsTable :listings="filteredPublicListings" :user-id="store.userId" @buy="openBuyListingDialog" @cancel="cancelListing" />
              <MarketPublicBuyOrdersTable :orders="filteredPublicBuyOrders" :user-id="store.userId" :inventory-sellable-for-buy-order="inventorySellableForBuyOrder" @fill="openFillBuyOrderDialog" @cancel="cancelBuyOrder" />
            </details>
            <div v-if="!instanceMode" class="hidden md:block">
              <div class="flex gap-2">
                <el-card class="app-card flex-1" shadow="never">
                  <template #header>
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium">{{ t("game.marketPublicListings") }}</span>
                      <el-button size="small" type="primary" @click="openListDialog">{{ t("game.marketCreateListing") }}</el-button>
                    </div>
                  </template>
                  <div v-if="!listingFilterItemId" class="text-sm app-muted">{{ t("game.marketSelectItem") }}</div>
                  <div v-else-if="filteredPublicListings.length === 0" class="text-sm app-muted">{{ t("game.marketNoListings") }}</div>
                  <MarketPublicListingsTable
                    v-else
                    :listings="filteredPublicListings"
                    :user-id="store.userId"
                    @buy="openBuyListingDialog"
                    @cancel="cancelListing"
                  />
                </el-card>

                <el-card class="app-card flex-1" shadow="never">
                  <template #header>
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium">{{ t("game.marketPublicBuyOrders") }}</span>
                      <el-button size="small" type="primary" @click="openBuyOrderDialog">{{ t("game.marketCreateBuyOrder") }}</el-button>
                    </div>
                  </template>
                  <div v-if="!listingFilterItemId" class="text-sm app-muted">{{ t("game.marketSelectItem") }}</div>
                  <div v-else-if="filteredPublicBuyOrders.length === 0" class="text-sm app-muted">{{ t("game.marketNoBuyOrders") }}</div>
                  <MarketPublicBuyOrdersTable
                    v-else
                    :orders="filteredPublicBuyOrders"
                    :user-id="store.userId"
                    :inventory-sellable-for-buy-order="inventorySellableForBuyOrder"
                    @fill="openFillBuyOrderDialog"
                    @cancel="cancelBuyOrder"
                  />
                </el-card>
              </div>
            </div>

            <div v-if="!instanceMode" class="space-y-3 md:hidden">
              <el-card class="app-card" shadow="never">
                <template #header>
                  <div class="flex items-center justify-between gap-2">
                    <span class="font-medium">{{ t("game.marketPublicListings") }}</span>
                    <el-button size="small" type="primary" @click="openListDialog">{{ t("game.marketCreateListing") }}</el-button>
                  </div>
                </template>
                <MarketPublicListingsTable
                  :listings="filteredPublicListings"
                  :user-id="store.userId"
                  @buy="openBuyListingDialog"
                  @cancel="cancelListing"
                />
              </el-card>

              <el-card class="app-card" shadow="never">
                <template #header>
                  <div class="flex items-center justify-between gap-2">
                    <span class="font-medium">{{ t("game.marketPublicBuyOrders") }}</span>
                    <el-button size="small" type="primary" @click="openBuyOrderDialog">{{ t("game.marketCreateBuyOrder") }}</el-button>
                  </div>
                </template>
                <MarketPublicBuyOrdersTable
                  :orders="filteredPublicBuyOrders"
                  :user-id="store.userId"
                  :inventory-sellable-for-buy-order="inventorySellableForBuyOrder"
                  @fill="openFillBuyOrderDialog"
                  @cancel="cancelBuyOrder"
                />
              </el-card>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('game.marketSubtabHistory')" name="history" lazy>
          <div data-tutorial="market-history">
          <MarketHistoryPanel
            :my-listings="myListings"
            :my-buy-orders="myBuyOrders"
            :listing-status-label="listingStatusLabel"
            :buy-order-status-label="buyOrderStatusLabel"
            @cancel-listing="cancelListing"
            @cancel-buy-order="cancelBuyOrder"
            @view-item="openMarketDetail"
          >
            <template #extra-tabs="{ activeTab }">
              <el-tab-pane :label="t('game.instanceMarket.listingRecords')" name="instanceListings" lazy>
                <InstanceMarketPanel v-if="marketTab === 'history' && activeTab === 'instanceListings'" history @view-item="openMarketDetail" />
              </el-tab-pane>
              <el-tab-pane :label="t('game.instanceMarket.purchaseRecords')" name="instanceOrders" lazy>
                <InstanceMarketPanel v-if="marketTab === 'history' && activeTab === 'instanceOrders'" history history-view="orders" @view-item="openMarketDetail" />
              </el-tab-pane>
            </template>
          </MarketHistoryPanel>
          </div>
        </el-tab-pane>
      </el-tabs>
    </GamePageFrame>

    <MarketListDialog v-if="listDialogVisible" :visible="listDialogVisible" :draft="listOrderDraft" :options="listableOptions"
      :selected-option="selectedListableOption" :show-item-selector="!listingFilterItemId"
      :min-price="listMinPrice"
      :gross-income="listGrossIncome" :fee-estimate="listFeeEstimate" :net-income="listNetIncome"
      :confirm-disabled="listableOptions.length === 0"
      @update:visible="listDialogVisible = $event" @adjust-price="adjustListPrice"
      @quick-quantity="quickSetStackQuantity" @confirm="confirmListDialog" />

    <MarketBuyOrderDialog v-if="buyOrderDialogVisible" :visible="buyOrderDialogVisible" :draft="buyOrderDraft" :item-options="buyOrderItemOptions"
      :show-item-selector="!listingFilterItemId" :disable-item-selection="!!listingFilterItemId"
      :min-price="buyOrderMinPrice"
      :total-payment="buyOrderTotalPayment" :fee-estimate="buyOrderFeeEstimate" :frozen-total="buyOrderFrozenTotal"
      :item-label="itemLabel" @update:visible="buyOrderDialogVisible = $event" @adjust-price="adjustBuyOrderPrice"
      @confirm="confirmBuyOrderDialog" />

    <GameDialog
      :model-value="buyListingDialogVisible"
      :title="t('game.marketBuyAction')"
      width="420px"
      @update:model-value="buyListingDialogVisible = $event"
    >
      <div v-if="selectedListing" class="space-y-3" data-tutorial="market-buy-dialog">
        <div class="text-sm">{{ itemLabel(selectedListing.itemId) }}</div>
        <div class="text-xs app-muted">{{ t("game.marketPrice") }}: {{ formatNumberWithThousands(selectedListing.price) }}</div>
        <div class="text-xs app-muted">{{ t("game.marketQuantity") }}: {{ selectedListing.quantity }}</div>
        <div class="flex items-center gap-2">
          <span class="text-sm w-24">{{ t("game.marketQuantity") }}</span>
          <el-input-number
            v-model="buyListingQuantity"
            :min="1"
            :max="Math.max(1, selectedListing.quantity)"
            class="w-full"
          />
        </div>
        <div class="rounded border border-[var(--app-border)] p-2 text-xs flex flex-col gap-2 app-muted">
          <!-- <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogInventoryGold") }}:</span>
            <QuickRef item-id="gold_coin" />
            <span>x {{ goldInventoryQuantity }}</span>
          </div> -->
          <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogRequiredGold") }}:</span>
            <QuickRefWithRequire item-id="gold_coin" :required-count="buyListingRequiredGold" />
          </div>
          <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogReceivableItem") }}:</span>
            <QuickRef :item-id="selectedListing.itemId" :count="buyListingQuantity" />
          </div>
          <!-- <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogTotalPayment") }}:</span>
            <QuickRef item-id="gold_coin" />
            <span>x {{ buyListingTotalPrice }}</span>
          </div> -->
        </div>
      </div>
      <template #footer>
        <el-button @click="buyListingDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" @click="confirmBuyListingDialog">{{ t("game.confirm") }}</el-button>
      </template>
    </GameDialog>

    <GameDialog
      v-model="tutorialBuyPreviewVisible"
      :title="t('game.marketBuyAction')"
      width="420px"
    >
      <div class="space-y-3" data-tutorial="market-buy-dialog">
        <div class="text-sm">{{ itemLabel(tutorialBuyPreviewItemId) }}</div>
        <div class="text-xs app-muted">{{ t("game.marketPrice") }}: {{ formatNumberWithThousands(tutorialBuyPreviewPrice) }}</div>
        <div class="text-xs app-muted">{{ t("game.marketQuantity") }}: {{ tutorialBuyPreviewQuantity }}</div>
        <div class="flex items-center gap-2">
          <span class="text-sm w-24">{{ t("game.marketQuantity") }}</span>
          <el-input-number
            v-model="tutorialBuyPreviewBuyQuantity"
            :min="1"
            :max="tutorialBuyPreviewQuantity"
            class="w-full"
          />
        </div>
        <div class="rounded border border-[var(--app-border)] p-2 text-xs flex flex-col gap-2 app-muted">
          <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogRequiredGold") }}:</span>
            <QuickRefWithRequire item-id="gold_coin" :required-count="tutorialBuyPreviewTotalPrice" />
          </div>
          <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogReceivableItem") }}:</span>
            <QuickRef :item-id="tutorialBuyPreviewItemId" :count="tutorialBuyPreviewBuyQuantity" />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="tutorialBuyPreviewVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" disabled>{{ t("game.confirm") }}</el-button>
      </template>
    </GameDialog>

    <GameDialog
      :model-value="fillBuyOrderDialogVisible"
      :title="t('game.marketSellToBuyOrder')"
      width="420px"
      @update:model-value="fillBuyOrderDialogVisible = $event"
    >
      <div v-if="selectedBuyOrder" class="space-y-3">
        <div class="text-sm">{{ itemLabel(selectedBuyOrder.itemId) }}</div>
        <div class="text-xs app-muted">{{ t("game.marketPrice") }}: {{ formatNumberWithThousands(selectedBuyOrder.price) }}</div>
        <div class="text-xs app-muted">
          {{ t("game.marketQuantity") }}: {{ selectedBuyOrder.remainingQuantity }}/{{ selectedBuyOrder.quantity }}
        </div>
        <div class="flex items-center gap-2">
          <span class="text-sm w-24">{{ t("game.marketQuantity") }}</span>
          <el-input-number
            v-model="fillBuyOrderQuantity"
            :min="1"
            :max="Math.max(1, Math.min(selectedBuyOrder.remainingQuantity, inventorySellableForBuyOrder(selectedBuyOrder.itemId)))"
            class="w-full"
          />
        </div>
        <div class="rounded border border-[var(--app-border)] p-2 text-xs app-muted flex flex-col gap-2 ">
          <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogInventoryItemQuantity") }}:</span>
            <QuickRefWithRequire :item-id="selectedBuyOrder.itemId" :required-count="fillBuyOrderQuantity" />
          </div>
          <div class="flex items-center gap-2">
            <span>{{ t("game.marketDialogTotalReceivableGold") }}:</span>
            <QuickRef item-id="gold_coin" :count="fillBuyOrderTotalReceivableGold" />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="fillBuyOrderDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" @click="confirmFillBuyOrderDialog">{{ t("game.confirm") }}</el-button>
      </template>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import MarketListDialog from "@views/game/market/components/MarketListDialog.vue"
import MarketBuyOrderDialog from "@views/game/market/components/MarketBuyOrderDialog.vue"
import MarketHistoryPanel from "@views/game/market/components/MarketHistoryPanel.vue"
import MarketPublicListingsTable from "@views/game/market/components/MarketPublicListingsTable.vue"
import MarketPublicBuyOrdersTable from "@views/game/market/components/MarketPublicBuyOrdersTable.vue"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import Card from "@/components/game/item/Card.vue"
import { useMarketPage } from "@/composables/game/market/useMarketPage"
import { formatNumberWithThousands } from "@/utils/number-format"
import QuickRef from "@/components/game/item/QuickRef.vue"
import { Icon } from "@iconify/vue"
import QuickRefWithRequire from "@/components/game/item/QuickRefWithRequire.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"
import InstanceMarketPanel from "./components/InstanceMarketPanel.vue"
import MarketCatalogItem from './components/MarketCatalogItem.vue'
import { usesInstanceMarket } from "@services/instance-market"

const MARKET_TUTORIAL_OPEN_LIST_DIALOG_EVENT = "market:tutorial-open-list-dialog"
const MARKET_TUTORIAL_OPEN_BUY_DIALOG_EVENT = "market:tutorial-open-buy-dialog"
const MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT = "market:tutorial-close-dialogs"

const { t } = useI18n()
const marketVisual = gamePageVisuals.market
const {
  store,
  marketTab,
  listDialogVisible,
  buyOrderDialogVisible,
  buyListingDialogVisible,
  fillBuyOrderDialogVisible,
  myListings,
  myBuyOrders,
  listingFilterItemId,
  listingCatalogValues,
  selectedCatalogEntry,
  searchKeyword,
  selectedListing,
  selectedBuyOrder,
  buyListingQuantity,
  fillBuyOrderQuantity,
  goldInventoryQuantity,
  buyListingRequiredGold,
  buyListingInventoryItemQuantity,
  buyListingTotalPrice,
  fillBuyOrderReceivableGold,
  fillBuyOrderTotalReceivableGold,
  fillBuyOrderInventoryItemQuantity,
  fillBuyOrderTotalPrice,
  buyOrderDraft,
  listOrderDraft,
  listableOptions,
  selectedListableOption,
  listMinPrice,
  buyOrderMinPrice,
  filterItemOptions,
  buyOrderItemOptions,
  marketCatalogItems,
  filteredPublicListings,
  filteredPublicBuyOrders,
  listGrossIncome,
  listFeeEstimate,
  listNetIncome,
  buyOrderTotalPayment,
  buyOrderFeeEstimate,
  buyOrderFrozenTotal,
  openMarketDetail,
  backToCatalog,
  handleTabChange,
  openListDialog,
  openBuyOrderDialog,
  adjustListPrice,
  adjustBuyOrderPrice,
  quickSetStackQuantity,
  itemLabel,
  inventorySellableForBuyOrder,
  listingStatusLabel,
  buyOrderStatusLabel,
  loadPlayerMarketData,
  refreshCurrentMarketPage,
  marketRefreshCoolingDown,
  openBuyListingDialog,
  confirmBuyListingDialog,
  openFillBuyOrderDialog,
  confirmFillBuyOrderDialog,
  confirmListDialog,
  confirmBuyOrderDialog,
  cancelBuyOrder,
  cancelListing
} = useMarketPage()

const instanceMode = computed(() => usesInstanceMarket(listingFilterItemId.value))
const instanceRefresh = ref(0)
function refreshMarketView() {
  if (instanceMode.value) instanceRefresh.value++
  void refreshCurrentMarketPage()
}

const catalogBrowserItems = computed<ItemBrowserItem[]>(() =>
  marketCatalogItems.value.map((item) => ({
    key: item.key,
    itemId: item.itemId,
    itemTypes: item.itemTypes,
    searchText: item.entry ? Object.values(item.entry.names).join(' ') : undefined
  }))
)
const catalogEntryByKey = computed(() => new Map<string | number, NonNullable<typeof selectedCatalogEntry.value>>(marketCatalogItems.value.filter(item => item.entry).map(item => [item.key, item.entry!])))
const tradeCatalogItems = computed(() => catalogBrowserItems.value.filter(item => item.itemId === listingFilterItemId.value && catalogEntryByKey.value.has(item.key)))
const instanceCatalogPending = computed(() => instanceMode.value && tradeCatalogItems.value.length > 0 && !selectedCatalogEntry.value)
const tutorialBuyPreviewVisible = ref(false)
const tutorialBuyPreviewItemId = ref("gold_coin")
const tutorialBuyPreviewPrice = ref(120)
const tutorialBuyPreviewQuantity = ref(8)
const tutorialBuyPreviewBuyQuantity = ref(1)
const tutorialBuyPreviewTotalPrice = computed(() => tutorialBuyPreviewPrice.value * tutorialBuyPreviewBuyQuantity.value)

function handleCatalogItemClick(item: ItemBrowserItem | null) {
  if (!item) return
  openMarketDetail(item.itemId, catalogEntryByKey.value.get(item.key)?.catalogValues || '')
}

async function handleMarketTutorialOpenListDialog() {
  closeMarketTutorialDialogs()
  marketTab.value = "trade"
  searchKeyword.value = ""
  if (!listingFilterItemId.value && listableOptions.value[0]?.itemId) {
    listingFilterItemId.value = listableOptions.value[0].itemId
  }
  await loadPlayerMarketData()
  openListDialog()
}

async function handleMarketTutorialOpenBuyDialog() {
  closeMarketTutorialDialogs()
  marketTab.value = "trade"
  searchKeyword.value = ""
  await loadPlayerMarketData()

  let targetListing = filteredPublicListings.value.find((listing) => listing.status === "active" && listing.sellerId !== store.userId)
    || filteredPublicListings.value.find((listing) => listing.status === "active")

  if (!targetListing && marketCatalogItems.value[0]?.itemId) {
    listingFilterItemId.value = marketCatalogItems.value[0].itemId
    await loadPlayerMarketData()
    targetListing = filteredPublicListings.value.find((listing) => listing.status === "active" && listing.sellerId !== store.userId)
      || filteredPublicListings.value.find((listing) => listing.status === "active")
  }

  if (!targetListing) {
    tutorialBuyPreviewItemId.value = listingFilterItemId.value || marketCatalogItems.value[0]?.itemId || "gold_coin"
    tutorialBuyPreviewPrice.value = 120
    tutorialBuyPreviewQuantity.value = 8
    tutorialBuyPreviewBuyQuantity.value = 1
    tutorialBuyPreviewVisible.value = true
    return
  }
  listingFilterItemId.value = targetListing.itemId
  await openBuyListingDialog(targetListing.id)
}

function closeMarketTutorialDialogs() {
  listDialogVisible.value = false
  buyOrderDialogVisible.value = false
  buyListingDialogVisible.value = false
  fillBuyOrderDialogVisible.value = false
  tutorialBuyPreviewVisible.value = false
}

// 商品目录卡片数量大（数百个），同步挂载会让点击市场菜单时卡顿。
// 延迟一帧 + 50ms 再挂载，让路由切换/页面骨架先渲染，卡顿转为进入后的短暂加载。
const catalogReady = ref(false)
onMounted(() => {
  window.addEventListener(MARKET_TUTORIAL_OPEN_LIST_DIALOG_EVENT, handleMarketTutorialOpenListDialog)
  window.addEventListener(MARKET_TUTORIAL_OPEN_BUY_DIALOG_EVENT, handleMarketTutorialOpenBuyDialog)
  window.addEventListener(MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT, closeMarketTutorialDialogs)
  window.setTimeout(() => {
    catalogReady.value = true
  }, 10)
})

onBeforeUnmount(() => {
  window.removeEventListener(MARKET_TUTORIAL_OPEN_LIST_DIALOG_EVENT, handleMarketTutorialOpenListDialog)
  window.removeEventListener(MARKET_TUTORIAL_OPEN_BUY_DIALOG_EVENT, handleMarketTutorialOpenBuyDialog)
  window.removeEventListener(MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT, closeMarketTutorialDialogs)
})
</script>
