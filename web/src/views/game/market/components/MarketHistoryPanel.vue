<template>
    <el-tabs v-model="activeTab">

      <el-tab-pane :label="t('game.marketHistoryListingsTab')" name="listings">
        <div v-if="props.myListings.length === 0" class="text-sm app-muted">{{ t("game.marketNoMyListings") }}</div>
        <template v-else>
          <div class="space-y-2">
            <div
              v-for="listing in paginatedListings"
              :key="`mine-${listing.id}`"
              class="flex flex-col gap-2 rounded border border-[var(--app-border)] p-3 text-sm sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="flex flex-col gap-1 min-w-0">
                <div class="flex items-center gap-2">
                  <QuickRef :item-id="listing.itemId" />
                  <el-tag :type="listing.status === 'active' ? 'success' : 'info'" size="small" class="shrink-0">
                    {{ props.listingStatusLabel(listing.status) }}
                  </el-tag>
                </div>
                <div class="app-muted text-xs pl-1">
                  {{ t("game.marketPrice") }}: {{ formatNumberWithThousands(listing.price) }} &nbsp;·&nbsp; {{ t("game.marketQuantity") }}: {{ listing.quantity }}
                </div>
              </div>
              <div class="flex items-center justify-start md:justify-end gap-1 shrink-0">
                <el-button v-if="listing.status === 'active'" size="small" type="warning" plain @click="$emit('cancel-listing', listing.id)">
                  {{ t("game.marketCancelListingAction") }}
                </el-button>
                <el-button size="small" @click="$emit('view-item', listing.itemId)">
                  {{ t("game.marketViewItem") }}
                </el-button>
              </div>
            </div>
          </div>
          <div v-if="props.myListings.length > ITEMS_PER_PAGE" class="mt-3 flex justify-center">
            <el-pagination
              v-model:current-page="listingsPage"
              :page-size="ITEMS_PER_PAGE"
              :total="props.myListings.length"
              layout="prev, pager, next"
              small
            />
          </div>
        </template>
      </el-tab-pane>

      <el-tab-pane :label="t('game.marketHistoryBuyOrdersTab')" name="buyOrders">
        <div v-if="props.myBuyOrders.length === 0" class="text-sm app-muted">{{ t("game.marketNoMyBuyOrders") }}</div>
        <template v-else>
          <div class="space-y-2">
            <div
              v-for="order in paginatedBuyOrders"
              :key="`my-buy-order-${order.id}`"
              class="flex flex-col gap-2 rounded border border-[var(--app-border)] p-3 text-sm sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="flex flex-col gap-1 min-w-0">
                <div class="flex items-center gap-2">
                  <QuickRef :item-id="order.itemId" />
                  <el-tag :type="order.status === 'active' ? 'success' : 'info'" size="small" class="shrink-0">
                    {{ props.buyOrderStatusLabel(order.status) }}
                  </el-tag>
                </div>
                <div class="app-muted text-xs pl-1">
                  {{ t("game.marketPrice") }}: {{ formatNumberWithThousands(order.price) }} &nbsp;·&nbsp; {{ t("game.marketQuantity") }}: {{ order.remainingQuantity }}/{{ order.quantity }}
                </div>
              </div>
              <div class="flex items-center justify-start md:justify-end gap-1 shrink-0">
                <el-button v-if="order.status === 'active'" size="small" type="warning" plain @click="$emit('cancel-buy-order', order.id)">
                  {{ t("game.marketCancelBuyOrder") }}
                </el-button>
                <el-button size="small" @click="$emit('view-item', order.itemId)">
                  {{ t("game.marketViewItem") }}
                </el-button>
              </div>
            </div>
          </div>
          <div v-if="props.myBuyOrders.length > ITEMS_PER_PAGE" class="mt-3 flex justify-center">
            <el-pagination
              v-model:current-page="buyOrdersPage"
              :page-size="ITEMS_PER_PAGE"
              :total="props.myBuyOrders.length"
              layout="prev, pager, next"
              small
            />
          </div>
        </template>
      </el-tab-pane>

      <slot name="extra-tabs" :active-tab="activeTab" />
    </el-tabs>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue"
import { useI18n } from "vue-i18n"
import QuickRef from "@components/game/item/QuickRef.vue"
import { formatNumberWithThousands } from "@utils/number-format"

type Listing = {
  id: number
  itemId: string
  price: number
  quantity: number
  status: string
}

type BuyOrder = {
  id: number
  itemId: string
  price: number
  quantity: number
  remainingQuantity: number
  status: string
}

const props = defineProps<{
  myListings: Listing[]
  myBuyOrders: BuyOrder[]
  listingStatusLabel: (status: string) => string
  buyOrderStatusLabel: (status: string) => string
}>()

defineEmits<{
  (e: "cancel-listing", listingID: number): void
  (e: "cancel-buy-order", orderID: number): void
  (e: "view-item", itemId: string): void
}>()

const { t } = useI18n()

const ITEMS_PER_PAGE = 15
const activeTab = ref("listings")
const listingsPage = ref(1)
const buyOrdersPage = ref(1)

watch(activeTab, () => {
  listingsPage.value = 1
  buyOrdersPage.value = 1
})

const paginatedListings = computed(() => {
  const start = (listingsPage.value - 1) * ITEMS_PER_PAGE
  return props.myListings.slice(start, start + ITEMS_PER_PAGE)
})

const paginatedBuyOrders = computed(() => {
  const start = (buyOrdersPage.value - 1) * ITEMS_PER_PAGE
  return props.myBuyOrders.slice(start, start + ITEMS_PER_PAGE)
})
</script>
