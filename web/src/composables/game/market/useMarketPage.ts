import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { apiGet, apiPost, createIdempotencyKey } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getItemTypes, isItemFixedMarketListable, listItemDefinitions } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import {
  getItemRealValuePrice,
  isValidMarketPrice,
  nextMarketPrice,
  normalizeMarketPrice,
  previousMarketPrice,
  suggestBuyOrderPrice,
  suggestListingPrice
} from "@services/market-pricing"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import { usesInstanceMarket } from "@services/instance-market"
import { marketCatalogEntries, marketCatalogName, type MarketCatalogEntry } from '@services/market-catalog'

type Listing = {
  id: number
  sellerId: number
  itemId: string
  listingKind: string
  price: number
  quantity: number
  status: string
}

type BuyOrder = {
  id: number
  buyerId: number
  itemId: string
  price: number
  quantity: number
  remainingQuantity: number
  status: string
}

type MarketSnapshot = {
  publicListings?: Listing[]
  publicBuyOrders?: BuyOrder[]
  myListings?: Listing[]
  myBuyOrders?: BuyOrder[]
}

type CatalogItem = {
  key: string
  itemId: string
  itemTypes: string[]
  entry?: MarketCatalogEntry
}

type MarketTab = "catalog" | "trade" | "history"
const goldCoinItemID = "gold_coin"

type ListableOption = {
  key: string
  itemId: string
  label: string
  mode: "stack"
  maxQuantity: number
}

export function useMarketPage() {
  const { t, locale } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const store = useGameStore()
  const inventoryStore = useInventoryStore()
  const { getTemplateCommodityQuantity } = useInventorySummary()

  const marketTab = ref<MarketTab>("catalog")
  const listDialogVisible = ref(false)
  const buyOrderDialogVisible = ref(false)
  const buyListingDialogVisible = ref(false)
  const fillBuyOrderDialogVisible = ref(false)
  const publicListings = ref<Listing[]>([])
  const myListings = ref<Listing[]>([])
  const publicBuyOrders = ref<BuyOrder[]>([])
  const myBuyOrders = ref<BuyOrder[]>([])
  const listingFilterItemId = ref("")
  const listingCatalogValues = ref("")
  const searchKeyword = ref("")
  const selectedListingId = ref(0)
  const selectedBuyOrderId = ref(0)
  const buyListingQuantity = ref(1)
  const fillBuyOrderQuantity = ref(1)

  const stackDraft = reactive({
    itemId: "",
    price: 1,
    quantity: 1
  })

  const buyOrderDraft = reactive({
    itemId: "",
    price: 1,
    quantity: 1
  })

  const listOrderDraft = reactive({
    selectedKey: "",
    price: 1,
    quantity: 1
  })

  const marketFeeRate = 0.05
  let syncingFromRoute = false
  let suppressNextFilterLoad = false
  let publicMarketRefreshing = false
  const marketRefreshCoolingDown = ref(false)
  let marketRefreshCooldownTimer: ReturnType<typeof setTimeout> | null = null

  const stackItemOptions = computed(() => {
    const map = new Map<string, number>()
    for (const item of listItemDefinitions()) {
      if (!isItemFixedMarketListable(item.id)) continue
      const quantity = getTemplateCommodityQuantity(item.id)
      if (quantity <= 0) continue
      map.set(item.id, quantity)
    }
    return Array.from(map.entries())
      .map(([itemId, quantity]) => ({ itemId, quantity }))
      .sort((a, b) => itemLabel(a.itemId).localeCompare(itemLabel(b.itemId)))
  })

  const listableOptions = computed<ListableOption[]>(() => {
    const options: ListableOption[] = []
    for (const item of stackItemOptions.value) {
      options.push({
        key: `stack:${item.itemId}`,
        itemId: item.itemId,
        label: `${itemLabel(item.itemId)} (${t("game.marketInventoryQuantity")}: ${item.quantity})`,
        mode: "stack",
        maxQuantity: item.quantity
      })
    }
    return options
  })

  const marketCatalogItems = computed<CatalogItem[]>(() => {
    return listItemDefinitions()
      .filter((item) => isItemFixedMarketListable(item.id) || usesInstanceMarket(item.id))
      .flatMap((item): CatalogItem[] => {
        const base = { key: item.id, itemId: item.id, itemTypes: getItemTypes(item.id) }
        const entries = marketCatalogEntries(item.id)
        return entries.length ? entries.map(entry => ({ ...base, key: entry.key, entry })) : [base]
      })
      .sort((a, b) => (a.entry ? marketCatalogName(a.entry, locale.value) : itemLabel(a.itemId)).localeCompare(b.entry ? marketCatalogName(b.entry, locale.value) : itemLabel(b.itemId)))
  })
  const selectedCatalogEntry = computed(() => marketCatalogItems.value.find(item => item.itemId === listingFilterItemId.value && item.entry?.catalogValues === listingCatalogValues.value)?.entry)

  const filterItemOptions = computed(() => marketCatalogItems.value.filter((item) => !usesInstanceMarket(item.itemId)).map((item) => item.itemId))

  const buyOrderItemOptions = computed(() => {
    const set = new Set<string>()
    for (const item of stackItemOptions.value) set.add(item.itemId)
    for (const itemId of filterItemOptions.value) set.add(itemId)
    return Array.from(set).sort((a, b) => itemLabel(a).localeCompare(itemLabel(b)))
  })

  const filteredPublicListings = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    return publicListings.value.filter((listing) => {
      if (listingFilterItemId.value && listing.itemId !== listingFilterItemId.value) return false
      if (keyword && !itemLabel(listing.itemId).toLowerCase().includes(keyword)) return false
      return true
    })
  })

  const filteredPublicBuyOrders = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    return publicBuyOrders.value.filter((order) => {
      if (listingFilterItemId.value && order.itemId !== listingFilterItemId.value) return false
      if (keyword && !itemLabel(order.itemId).toLowerCase().includes(keyword)) return false
      return true
    })
  })

  const selectedListableOption = computed(() => listableOptions.value.find((option) => option.key === listOrderDraft.selectedKey) || null)
  const listMinPrice = computed(() => {
    const itemID = selectedListableOption.value?.itemId || ""
    return getItemRealValuePrice(itemID)
  })
  const buyOrderMinPrice = computed(() => getItemRealValuePrice(buyOrderDraft.itemId))
  const listUnitPrice = computed(() => Math.max(1, listOrderDraft.price))
  const listQuantity = computed(() => {
    const selected = selectedListableOption.value
    if (!selected) return 0
    return Math.max(1, Math.min(selected.maxQuantity, listOrderDraft.quantity))
  })
  const listGrossIncome = computed(() => Math.max(0, listUnitPrice.value * listQuantity.value))
  const listFeeEstimate = computed(() => Math.floor(listGrossIncome.value * marketFeeRate))
  const listNetIncome = computed(() => Math.max(0, listGrossIncome.value - listFeeEstimate.value))
  const buyOrderTotalPayment = computed(() => Math.max(0, buyOrderDraft.price * buyOrderDraft.quantity))
  const buyOrderFeeEstimate = computed(() => Math.floor(buyOrderTotalPayment.value * marketFeeRate))
  const buyOrderFrozenTotal = computed(() => buyOrderTotalPayment.value)
  const selectedListing = computed(() => publicListings.value.find((item) => item.id === selectedListingId.value) || null)
  const selectedBuyOrder = computed(() => publicBuyOrders.value.find((item) => item.id === selectedBuyOrderId.value) || null)
  const buyListingTotalPrice = computed(() => {
    if (!selectedListing.value) return 0
    return Math.max(0, selectedListing.value.price * buyListingQuantity.value)
  })
  const goldInventoryQuantity = computed(() => Math.max(0, getTemplateCommodityQuantity(goldCoinItemID)))
  const buyListingRequiredGold = computed(() => buyListingTotalPrice.value)
  const buyListingInventoryItemQuantity = computed(() => {
    if (!selectedListing.value) return 0
    return Math.max(0, getTemplateCommodityQuantity(selectedListing.value.itemId))
  })
  const fillBuyOrderTotalPrice = computed(() => {
    if (!selectedBuyOrder.value) return 0
    return Math.max(0, selectedBuyOrder.value.price * fillBuyOrderQuantity.value)
  })
  const fillBuyOrderReceivableGold = computed(() => fillBuyOrderTotalPrice.value)
  const fillBuyOrderTotalReceivableGold = computed(() => fillBuyOrderTotalPrice.value)
  const fillBuyOrderInventoryItemQuantity = computed(() => {
    if (!selectedBuyOrder.value) return 0
    return Math.max(0, getTemplateCommodityQuantity(selectedBuyOrder.value.itemId))
  })

  function readQueryValue(raw: unknown): string {
    if (Array.isArray(raw)) return String(raw[0] || "").trim()
    return String(raw || "").trim()
  }

  function normalizeMarketTab(raw: unknown): MarketTab {
    const value = readQueryValue(raw)
    if (value === "trade" || value === "history" || value === "catalog") return value
    return "catalog"
  }

  function syncStateFromRoute() {
    syncingFromRoute = true
    const queryTab = normalizeMarketTab(route.query.tab)
    const queryItemID = readQueryValue(route.query.itemId)
    listingCatalogValues.value = readQueryValue(route.query.catalogValues)
    suppressNextFilterLoad = listingFilterItemId.value !== queryItemID
    if (queryItemID) {
      listingFilterItemId.value = queryItemID
      stackDraft.itemId = queryItemID
      buyOrderDraft.itemId = queryItemID
      marketTab.value = queryTab === "history" ? "history" : "trade"
      syncingFromRoute = false
      return
    }
    listingFilterItemId.value = ""
    listingCatalogValues.value = ""
    marketTab.value = queryTab
    syncingFromRoute = false
  }

  function syncRouteFromState() {
    if (syncingFromRoute) return
    const currentTab = normalizeMarketTab(route.query.tab)
    const currentItemID = readQueryValue(route.query.itemId)
    const nextTab = marketTab.value
    const nextItemID = listingFilterItemId.value.trim()
    const nextCatalogValues = nextTab === 'catalog' ? '' : listingCatalogValues.value
    if (currentTab === nextTab && currentItemID === nextItemID && readQueryValue(route.query.catalogValues) === nextCatalogValues) return
    void router.replace({
      query: {
        ...route.query,
        tab: nextTab,
        itemId: nextTab === "catalog" ? undefined : (nextItemID || undefined),
        catalogValues: nextCatalogValues || undefined
      }
    })
  }

  function openMarketDetail(itemID: string, catalogValues = '') {
    listingFilterItemId.value = itemID
    listingCatalogValues.value = catalogValues
    stackDraft.itemId = itemID
    buyOrderDraft.itemId = itemID
    const preferred = listableOptions.value.find((option) => option.itemId === itemID)
    if (preferred) {
      listOrderDraft.selectedKey = preferred.key
      listOrderDraft.quantity = Math.max(1, Math.min(preferred.maxQuantity, listOrderDraft.quantity))
    }
    marketTab.value = "trade"
  }

  function backToCatalog() {
    listingFilterItemId.value = ""
    listingCatalogValues.value = ""
    marketTab.value = "catalog"
  }

  function handleTabChange(tab: MarketTab) {
    if (tab === "catalog") {
      listingFilterItemId.value = ""
      listingCatalogValues.value = ""
      stackDraft.itemId = ""
      buyOrderDraft.itemId = ""
    }
  }

  function openListDialog() {
    const preferred = listingFilterItemId.value
      ? listableOptions.value.find((option) => option.itemId === listingFilterItemId.value)
      : listableOptions.value[0]
    listOrderDraft.selectedKey = preferred?.key || ""
    listOrderDraft.quantity = Math.max(1, Math.min(preferred?.maxQuantity || 1, listOrderDraft.quantity))
    const itemID = preferred?.itemId || ""
    if (itemID) {
      listOrderDraft.price = suggestListingPrice(itemID, publicListings.value)
      listOrderDraft.price = normalizeMarketPrice(listOrderDraft.price, getItemRealValuePrice(itemID), "up")
      void loadPublicMarketData(itemID).then(() => {
        if (!listDialogVisible.value) return
        const currentSelected = selectedListableOption.value
        if (!currentSelected || currentSelected.itemId !== itemID) return
        const minPrice = getItemRealValuePrice(itemID)
        const suggested = suggestListingPrice(itemID, publicListings.value)
        listOrderDraft.price = normalizeMarketPrice(suggested, minPrice, "up")
      })
    } else {
      listOrderDraft.price = 1
    }
    listDialogVisible.value = true
  }

  function openBuyOrderDialog() {
    if (listingFilterItemId.value) {
      buyOrderDraft.itemId = listingFilterItemId.value
    }
    const itemID = buyOrderDraft.itemId
    if (itemID) {
      buyOrderDraft.price = suggestBuyOrderPrice(itemID, publicBuyOrders.value)
      buyOrderDraft.price = normalizeMarketPrice(buyOrderDraft.price, getItemRealValuePrice(itemID), "up")
      void loadPublicMarketData(itemID).then(() => {
        if (!buyOrderDialogVisible.value || buyOrderDraft.itemId !== itemID) return
        const minPrice = getItemRealValuePrice(itemID)
        const suggested = suggestBuyOrderPrice(itemID, publicBuyOrders.value)
        buyOrderDraft.price = normalizeMarketPrice(suggested, minPrice, "up")
      })
    } else {
      buyOrderDraft.price = 1
    }
    buyOrderDialogVisible.value = true
  }

  function adjustPriceValue(value: number, minPrice: number, op: "half" | "minus" | "plus" | "double"): number {
    const safeMin = Math.max(1, Math.floor(minPrice || 1))
    const safeValue = Math.max(safeMin, Math.floor(value || safeMin))
    if (op === "minus") return previousMarketPrice(safeValue, safeMin)
    if (op === "plus") return nextMarketPrice(safeValue, safeMin)
    if (op === "half") return normalizeMarketPrice(Math.floor(safeValue / 2), safeMin, "down")
    return normalizeMarketPrice(safeValue * 2, safeMin, "up")
  }

  function adjustListPrice(op: "half" | "minus" | "plus" | "double") {
    listOrderDraft.price = adjustPriceValue(listOrderDraft.price, listMinPrice.value, op)
  }

  function adjustBuyOrderPrice(op: "half" | "minus" | "plus" | "double") {
    buyOrderDraft.price = adjustPriceValue(buyOrderDraft.price, buyOrderMinPrice.value, op)
  }

  function quickSetStackQuantity(next: number) {
    const max = Math.max(1, selectedListableOption.value?.maxQuantity || 1)
    const normalized = Math.max(1, Math.min(max, Math.floor(next)))
    listOrderDraft.quantity = normalized
  }

  function itemLabel(itemID: string) {
    return getLocalizedItemName(t, itemID, { fallbackText: t("game.unnamedItem") })
  }

  function inventorySellableForBuyOrder(itemID: string): number {
    return Math.max(0, getTemplateCommodityQuantity(itemID))
  }

  function listingStatusLabel(status: string) {
    if (status === "active") return t("game.marketStatusActive")
    if (status === "sold") return t("game.marketStatusSold")
    return t("game.marketStatusOther")
  }

  function buyOrderStatusLabel(status: string) {
    if (status === "active") return t("game.marketStatusActive")
    if (status === "filled") return t("game.marketStatusFilled")
    if (status === "cancelled") return t("game.marketStatusCancelled")
    return t("game.marketStatusOther")
  }

  async function loadPublicMarketData(itemID: string) {
    if (!store.token) return
    const normalizedItemID = itemID.trim()
    if (!normalizedItemID) {
      publicListings.value = []
      publicBuyOrders.value = []
      return
    }
    const [publicResult, publicBuyOrderResult] = await Promise.allSettled([
      apiGet<{ items: Listing[] }>(
        `/api/market/listings?scope=public&itemId=${encodeURIComponent(normalizedItemID)}&limit=80`,
        store.token
      ),
      apiGet<{ items: BuyOrder[] }>(
        `/api/market/buy-orders?scope=public&itemId=${encodeURIComponent(normalizedItemID)}&limit=80`,
        store.token
      )
    ])

    if (publicResult.status === "fulfilled") {
      publicListings.value = publicResult.value.items || []
    } else {
      publicListings.value = []
      ElMessage.error(resolveErrorMessage(t, publicResult.reason, "game.marketLoadFailed"))
    }

    if (publicBuyOrderResult.status === "fulfilled") {
      publicBuyOrders.value = publicBuyOrderResult.value.items || []
    } else {
      publicBuyOrders.value = []
      ElMessage.error(resolveErrorMessage(t, publicBuyOrderResult.reason, "game.marketBuyOrdersLoadFailed"))
    }
  }

  async function loadMarketSnapshot() {
    if (!store.token) return
    const normalizedItemID = listingFilterItemId.value.trim()
    try {
      const query = normalizedItemID ? `?itemId=${encodeURIComponent(normalizedItemID)}` : ""
      const snapshot = await apiGet<MarketSnapshot>(`/api/market/snapshot${query}`, store.token)
      myListings.value = snapshot.myListings || []
      myBuyOrders.value = snapshot.myBuyOrders || []
      publicListings.value = snapshot.publicListings || []
      publicBuyOrders.value = snapshot.publicBuyOrders || []
    } catch (err) {
      myListings.value = []
      myBuyOrders.value = []
      publicListings.value = []
      publicBuyOrders.value = []
      ElMessage.error(resolveErrorMessage(t, err, "game.marketLoadFailed"))
    }
  }

  async function loadPlayerMarketData() {
    await loadMarketSnapshot()
  }

  function startMarketRefreshCooldown() {
    marketRefreshCoolingDown.value = true
    if (marketRefreshCooldownTimer) {
      clearTimeout(marketRefreshCooldownTimer)
    }
    marketRefreshCooldownTimer = setTimeout(() => {
      marketRefreshCooldownTimer = null
      marketRefreshCoolingDown.value = false
    }, 5_000)
  }

  async function refreshCurrentMarketPage() {
    if (marketRefreshCoolingDown.value || publicMarketRefreshing) return
    startMarketRefreshCooldown()
    await loadPlayerMarketData()
  }

  async function refreshPublicMarketData(force = false) {
    const itemID = listingFilterItemId.value.trim()
    if (!store.token || !itemID) return
    if (!force && marketTab.value !== "trade") return
    if (publicMarketRefreshing) return
    publicMarketRefreshing = true
    try {
      await loadPublicMarketData(itemID)
    } finally {
      publicMarketRefreshing = false
    }
  }

  async function submitStackListing() {
    if (!store.token || !stackDraft.itemId) {
      ElMessage.warning(t("game.marketSelectItem"))
      return
    }
    if (stackDraft.quantity > getTemplateCommodityQuantity(stackDraft.itemId)) {
      ElMessage.warning(t("game.marketInsufficientInventory"))
      return
    }
    try {
      await apiPost("/api/market/list", store.token, {
        itemId: stackDraft.itemId,
        price: stackDraft.price,
        quantity: stackDraft.quantity
      })
      ElMessage.success(t("game.marketCreateListingSuccess"))
      listDialogVisible.value = false
      await loadPlayerMarketData()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.marketCreateListingFailed"))
    }
  }

  async function submitUnifiedListing() {
    const selected = selectedListableOption.value
    if (!store.token || !selected) {
      ElMessage.warning(t("game.marketSelectItem"))
      return
    }
    const minPrice = getItemRealValuePrice(selected.itemId)
    const normalizedPrice = normalizeMarketPrice(listOrderDraft.price, minPrice, "up")
    listOrderDraft.price = normalizedPrice
    if (!isValidMarketPrice(normalizedPrice, minPrice)) {
      ElMessage.warning(t("game.marketCreateListingFailed"))
      return
    }
    stackDraft.itemId = selected.itemId
    stackDraft.price = normalizedPrice
    stackDraft.quantity = listQuantity.value
    await submitStackListing()
  }

  async function openBuyListingDialog(listingID: number) {
    await refreshPublicMarketData(true)
    const listing = publicListings.value.find((item) => item.id === listingID)
    if (!listing || listing.status !== "active") return
    selectedListingId.value = listingID
    buyListingQuantity.value = Math.max(1, Math.min(listing.quantity, Math.floor(buyListingQuantity.value || 1)))
    buyListingDialogVisible.value = true
  }

  async function buyListing(listingID: number, quantity: number) {
    if (!store.token) return
    const safeQuantity = Math.max(1, Math.floor(quantity || 1))
    try {
      await apiPost(
        "/api/market/buy",
        store.token,
        { listingId: listingID, quantity: safeQuantity },
        { "X-Idempotency-Key": createIdempotencyKey(`market-buy-${listingID}`) }
      )
      ElMessage.success(t("game.marketBuySuccess"))
      buyListingDialogVisible.value = false
      await loadPlayerMarketData()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.marketBuyFailed"))
    }
  }

  async function confirmBuyListingDialog() {
    const listing = selectedListing.value
    if (!listing) return
    const quantity = Math.max(1, Math.min(listing.quantity, Math.floor(buyListingQuantity.value || 1)))
    await buyListing(listing.id, quantity)
  }

  async function submitBuyOrder() {
    if (!store.token || !buyOrderDraft.itemId) {
      ElMessage.warning(t("game.marketSelectItem"))
      return
    }
    const minPrice = getItemRealValuePrice(buyOrderDraft.itemId)
    const normalizedPrice = normalizeMarketPrice(buyOrderDraft.price, minPrice, "up")
    buyOrderDraft.price = normalizedPrice
    if (!isValidMarketPrice(normalizedPrice, minPrice)) {
      ElMessage.warning(t("game.marketCreateBuyOrderFailed"))
      return
    }
    try {
      await apiPost("/api/market/buy-orders", store.token, {
        itemId: buyOrderDraft.itemId,
        price: normalizedPrice,
        quantity: buyOrderDraft.quantity
      })
      ElMessage.success(t("game.marketCreateBuyOrderSuccess"))
      buyOrderDialogVisible.value = false
      await loadPlayerMarketData()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.marketCreateBuyOrderFailed"))
    }
  }

  async function confirmListDialog() {
    await submitUnifiedListing()
  }

  async function confirmBuyOrderDialog() {
    await submitBuyOrder()
  }

  async function openFillBuyOrderDialog(orderID: number) {
    await refreshPublicMarketData(true)
    const order = publicBuyOrders.value.find((item) => item.id === orderID)
    if (!order || order.status !== "active") return
    selectedBuyOrderId.value = orderID
    const maxSellable = Math.min(order.remainingQuantity, getTemplateCommodityQuantity(order.itemId))
    fillBuyOrderQuantity.value = Math.max(1, Math.min(maxSellable, Math.floor(fillBuyOrderQuantity.value || 1)))
    fillBuyOrderDialogVisible.value = true
  }

  async function fillBuyOrder(orderID: number, quantity: number) {
    if (!store.token) return
    const order = publicBuyOrders.value.find((item) => item.id === orderID)
    if (!order) {
      ElMessage.warning(t("game.marketFillBuyOrderFailed"))
      return
    }
    const maxSellable = Math.min(order.remainingQuantity, getTemplateCommodityQuantity(order.itemId))
    if (maxSellable <= 0) {
      ElMessage.warning(t("game.marketInsufficientInventory"))
      return
    }
    const safeQuantity = Math.max(1, Math.min(maxSellable, Math.floor(quantity || 1)))
    try {
      await apiPost("/api/market/buy-orders/fill", store.token, { orderId: orderID, quantity: safeQuantity })
      ElMessage.success(t("game.marketFillBuyOrderSuccess"))
      fillBuyOrderDialogVisible.value = false
      await loadPlayerMarketData()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.marketFillBuyOrderFailed"))
    }
  }

  async function confirmFillBuyOrderDialog() {
    const order = selectedBuyOrder.value
    if (!order) return
    const maxSellable = Math.min(order.remainingQuantity, getTemplateCommodityQuantity(order.itemId))
    if (maxSellable <= 0) {
      ElMessage.warning(t("game.marketInsufficientInventory"))
      return
    }
    const quantity = Math.max(1, Math.min(maxSellable, Math.floor(fillBuyOrderQuantity.value || 1)))
    await fillBuyOrder(order.id, quantity)
  }

  async function cancelBuyOrder(orderID: number) {
    if (!store.token) return
    try {
      await apiPost("/api/market/buy-orders/cancel", store.token, { orderId: orderID })
      ElMessage.success(t("game.marketCancelBuyOrderSuccess"))
      await loadPlayerMarketData()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.marketCancelBuyOrderFailed"))
    }
  }

  async function cancelListing(listingID: number) {
    if (!store.token) return
    try {
      await apiPost("/api/market/listings/cancel", store.token, { listingId: listingID })
      ElMessage.success(t("game.marketCancelListingSuccess"))
      await loadPlayerMarketData()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.marketCancelListingFailed"))
    }
  }

  onMounted(() => {
    store.restoreToken()
    if (!store.token) return
    syncStateFromRoute()
    void inventoryStore.initialize(store.token)
    void loadPlayerMarketData()
  })

  onUnmounted(() => {
    if (!marketRefreshCooldownTimer) return
    clearTimeout(marketRefreshCooldownTimer)
    marketRefreshCooldownTimer = null
  })

  watch(
    () => route.query,
    () => {
      syncStateFromRoute()
    },
    { immediate: true }
  )

  watch(
    () => listingFilterItemId.value,
    (itemID, previousItemID) => {
      if (!itemID) {
        if (previousItemID) {
          publicListings.value = []
          publicBuyOrders.value = []
        }
        suppressNextFilterLoad = false
        return
      }
      stackDraft.itemId = itemID
      buyOrderDraft.itemId = itemID
      if (suppressNextFilterLoad) {
        suppressNextFilterLoad = false
        return
      }
      void loadPublicMarketData(itemID)
    }
  )

  watch(
    () => listOrderDraft.selectedKey,
    () => {
      const selected = selectedListableOption.value
      if (!selected) return
      listOrderDraft.quantity = Math.max(1, Math.min(selected.maxQuantity, listOrderDraft.quantity))
      const minPrice = getItemRealValuePrice(selected.itemId)
      const suggested = suggestListingPrice(selected.itemId, publicListings.value)
      listOrderDraft.price = normalizeMarketPrice(suggested, minPrice, "up")
    }
  )

  watch(
    () => buyOrderDraft.itemId,
    (itemID) => {
      if (!itemID) {
        buyOrderDraft.price = 1
        return
      }
      const minPrice = getItemRealValuePrice(itemID)
      const suggested = suggestBuyOrderPrice(itemID, publicBuyOrders.value)
      buyOrderDraft.price = normalizeMarketPrice(suggested, minPrice, "up")
    }
  )

  watch(
    () => [marketTab.value, listingFilterItemId.value, listingCatalogValues.value] as const,
    () => {
      syncRouteFromState()
    }
  )

  return {
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
    buyListing,
    openFillBuyOrderDialog,
    confirmFillBuyOrderDialog,
    confirmListDialog,
    confirmBuyOrderDialog,
    fillBuyOrder,
    cancelBuyOrder,
    cancelListing
  }
}
