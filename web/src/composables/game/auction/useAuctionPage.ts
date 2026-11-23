import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useInventoryStore, type InventoryInstance } from "@stores/inventory"
import { apiGet, apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { isItemMarketListable } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { getItemRealValuePrice } from "@services/market-pricing"
import { itemCustomName } from "@services/item-customization"
import {
  AUCTION_ANTI_SNIPE_EXTEND_SECONDS,
  AUCTION_ANTI_SNIPE_TRIGGER_SECONDS,
  AUCTION_FEE_RATE_BPS,
  AUCTION_MIN_REP_LEVEL
} from "@/config/shared-global-config"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import type { EquipmentAffixSlot, FishInstanceData, ItemCustomization } from "@/types/equipment"

type AuctionListing = {
  id: number
  sellerId: number
  sellerUsername?: string
  listingKind: "stack" | "instance"
  itemId: string
  instanceId?: number | null
  instanceCustomData?: ItemCustomization
  fishData?: FishInstanceData | null
  instanceEnhanceLevel?: number | null
  instanceBaseSnapshot?: Record<string, unknown> | null
  instanceAffixes?: EquipmentAffixSlot[] | null
  instanceDurability?: number | null
  instanceMaxDurability?: number | null
  quantity: number
  startPrice: number
  currentPrice: number
  minBidIncrement: number
  minNextBid: number
  currentBidderId?: number | null
  currentBidderUsername?: string
  buyoutPrice?: number | null
  feeRateBps?: number
  feeAmount?: number
  sellerAmount?: number
  antiSnipeExtendCount?: number
  status: string
  endsAt: string
}

type AuctionBid = {
  id: number
  auctionId: number
  bidderId: number
  bidderUsername?: string
  bidAmount: number
  createdAt: string
}

type AuctionTab = "public" | "mine" | "detail"
type AuctionPublicStatusFilter = "participating" | "sold" | "expired" | "all"

const AUCTION_MIN_DURATION_HOURS = 4
const AUCTION_MAX_DURATION_HOURS = 72

function normalizeDurationHours(raw: number): number {
  if (!Number.isFinite(raw)) return 24
  const hours = Math.floor(raw)
  if (hours < AUCTION_MIN_DURATION_HOURS) return AUCTION_MIN_DURATION_HOURS
  if (hours > AUCTION_MAX_DURATION_HOURS) return AUCTION_MAX_DURATION_HOURS
  return hours
}

function minBuyoutPrice(startPrice: number): number {
  const safeStart = Math.max(1, Math.floor(startPrice || 1))
  return Math.floor((safeStart * 3) / 2) + 1
}

export function isAuctionInstanceSelectable(
  item: Pick<InventoryInstance, "state" | "quantity" | "templateItemId">
): boolean {
  return item.state === "inventory" && item.quantity === 1 && isItemMarketListable(item.templateItemId)
}

export function useAuctionPage() {
  const { t } = useI18n()
  const store = useGameStore()
  const inventoryStore = useInventoryStore()
  const { inventoryInstances, hasInventoryInstance } = useInventorySummary()

  const activeTab = ref<AuctionTab>("public")
  const publicAuctions = ref<AuctionListing[]>([])
  const myAuctions = ref<AuctionListing[]>([])
  const selectedAuctionDetail = ref<AuctionListing | null>(null)
  const createDialogVisible = ref(false)
  const instanceSelectorVisible = ref(false)
  const buyoutAutoFillEnabled = ref(true)
  const bidDraftByAuction = reactive<Record<number, number>>({})
  const bidHistoryVisible = ref(false)
  const bidHistory = ref<AuctionBid[]>([])
  const nowTs = ref(Date.now())
  const filterItemId = ref("")
  const publicStatusFilter = ref<AuctionPublicStatusFilter>("participating")
  const searchKeyword = ref("")
  const repLevel = ref(0)
  const requiredRepLevel = AUCTION_MIN_REP_LEVEL
  const canAuction = computed(() => repLevel.value >= requiredRepLevel)
  let ticker: number | null = null

  const createDraft = reactive({
    instanceId: 0,
    startPrice: 100,
    minBidIncrement: 10,
    buyoutPrice: 0,
    durationHours: 24
  })

  const instanceAuctionOptions = computed(() => {
    return inventoryInstances.value
      .filter(isAuctionInstanceSelectable)
      .map((item) => ({ instanceId: item.instanceId, itemId: item.templateItemId, archivedAt: item.archivedAt || "" }))
      .sort((a, b) =>
        Number(!!a.archivedAt) - Number(!!b.archivedAt) ||
        itemLabel(a.itemId).localeCompare(itemLabel(b.itemId)) ||
        a.instanceId - b.instanceId
      )
  })

  const instanceAuctionInstanceIDs = computed(() => instanceAuctionOptions.value.map((row) => row.instanceId))

  const selectedInstanceItemId = computed(() => {
    if (createDraft.instanceId <= 0) return ""
    return inventoryInstances.value.find((row) => row.instanceId === createDraft.instanceId)?.templateItemId || ""
  })

  const createMinStartPrice = computed(() => {
    if (createDraft.instanceId <= 0) return 1
    const instance = inventoryInstances.value.find((row) => row.instanceId === createDraft.instanceId)
    if (!instance) return 1
    const hasAffix = Array.isArray(instance.affixes) && instance.affixes.length > 0
    const multiplier = hasAffix ? 5 : 1
    return getItemRealValuePrice(instance.templateItemId) * multiplier
  })

  const createFeeEstimate = computed(() => Math.max(0, Math.floor(Math.max(1, createDraft.startPrice) * AUCTION_FEE_RATE_BPS / 10000)))
  const createSellerAmount = computed(() => Math.max(0, Math.max(1, createDraft.startPrice) - createFeeEstimate.value))
  const createMinBuyout = computed(() => minBuyoutPrice(createDraft.startPrice))
  const createMinBidIncrement = computed(() => Math.max(1, Math.ceil(Math.max(1, Math.floor(createDraft.startPrice || 1)) * 0.1)))
  const createMaxBidIncrement = computed(() => Math.max(createMinBidIncrement.value, Math.floor(Math.max(1, createDraft.startPrice || 1))))

  const canCreateAuction = computed(() => {
    if (createDraft.startPrice <= createMinStartPrice.value) return false
    if (createDraft.buyoutPrice > 0 && createDraft.buyoutPrice < createMinBuyout.value) return false
    if (createDraft.minBidIncrement < createMinBidIncrement.value || createDraft.minBidIncrement > createMaxBidIncrement.value) return false
    return (
      createDraft.instanceId > 0 &&
      hasInventoryInstance(createDraft.instanceId) &&
      instanceAuctionOptions.value.some((item) => item.instanceId === createDraft.instanceId)
    )
  })

  const auctionFilterOptions = computed(() => {
    const set = new Set<string>()
    for (const row of publicAuctions.value) set.add(row.itemId)
    for (const row of myAuctions.value) set.add(row.itemId)
    return Array.from(set).sort((a, b) => itemLabel(a).localeCompare(itemLabel(b)))
  })

  const filteredPublicAuctions = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    return publicAuctions.value.filter((row) => {
      if (publicStatusFilter.value === "participating") {
        if (row.status !== "active" || row.sellerId === store.userId) return false
      } else if (publicStatusFilter.value === "sold") {
        if (row.status !== "sold") return false
      } else if (publicStatusFilter.value === "expired") {
        if (row.status !== "expired") return false
      }
      if (filterItemId.value && row.itemId !== filterItemId.value) return false
      if (keyword && !auctionItemLabel(row).toLowerCase().includes(keyword)) return false
      return true
    })
  })

  const filteredMyAuctions = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    return myAuctions.value.filter((row) => {
      if (filterItemId.value && row.itemId !== filterItemId.value) return false
      if (keyword && !auctionItemLabel(row).toLowerCase().includes(keyword)) return false
      return true
    })
  })

  const activeAuctionCount = computed(() => publicAuctions.value.filter((row) => row.status === "active").length)
  const myActiveAuctionCount = computed(() => myAuctions.value.filter((row) => row.status === "active").length)
  const highestBidPrice = computed(() => {
    const prices = publicAuctions.value.map((row) => row.currentPrice).filter((price) => price > 0)
    if (prices.length === 0) return 0
    return Math.max(...prices)
  })
  const selectedCancelFee = computed(() => {
    const detail = selectedAuctionDetail.value
    if (!detail || detail.currentPrice <= 0 || !detail.currentBidderId) return 0
    const fee = Math.floor((detail.currentPrice * AUCTION_FEE_RATE_BPS) / 10_000)
    return Math.max(1, fee)
  })

  onMounted(() => {
    store.restoreToken()
    if (!store.token) return
    void inventoryStore.initialize(store.token)
    void loadAll()
    void loadReputation()
    ticker = window.setInterval(() => {
      nowTs.value = Date.now()
    }, 1000)
  })

  onUnmounted(() => {
    if (ticker !== null) {
      window.clearInterval(ticker)
      ticker = null
    }
  })

  watch(activeTab, (next, prev) => {
    if (prev === "detail" && next !== "detail") {
      selectedAuctionDetail.value = null
    }
  })

  function itemLabel(itemID: string) {
    return getLocalizedItemName(t, itemID, { fallbackText: t("game.unnamedItem") })
  }

  function auctionItemLabel(row: AuctionListing) {
    const original = itemLabel(row.itemId)
    const customName = itemCustomName(row.instanceCustomData)
    return customName ? `${customName}（${original}）` : original
  }

  function formatEndsAt(value: string) {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return "-"
    return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`
  }

  function formatRemaining(value: string) {
    const end = new Date(value).getTime()
    if (!Number.isFinite(end)) return "-"
    const remainSec = Math.max(0, Math.floor((end - nowTs.value) / 1000))
    const hours = Math.floor(remainSec / 3600)
    const minutes = Math.floor((remainSec % 3600) / 60)
    const seconds = remainSec % 60
    return `${hours}h ${minutes}m ${seconds}s`
  }

  function auctionStatusLabel(status: string) {
    if (status === "active") return t("game.marketStatusActive")
    if (status === "sold") return t("game.marketStatusSold")
    if (status === "cancelled") return t("game.marketStatusCancelled")
    if (status === "expired") return t("game.marketStatusOther")
    return t("game.marketStatusOther")
  }

  function normalizeDraftBid(row: AuctionListing): number {
    const minIncrement = Math.max(1, Math.floor(row.minBidIncrement || 1))
    return Math.max(1, row.minNextBid || row.startPrice || 1, row.currentPrice + minIncrement)
  }

  async function loadReputation() {
    if (!store.token) return
    try {
      const data = await apiGet<{ reputation: { level: number } }>("/api/cat-village/commissions", store.token)
      repLevel.value = data?.reputation?.level ?? 0
    } catch {
      repLevel.value = 0
    }
  }

  async function loadAll() {
    if (!store.token) return
    const [pub, mine] = await Promise.allSettled([
      apiGet<{ items: AuctionListing[] }>("/api/auction/listings?scope=public&includeEnded=true&limit=80", store.token),
      apiGet<{ items: AuctionListing[] }>("/api/auction/listings?scope=mine&includeEnded=true&limit=40", store.token)
    ])
    if (pub.status === "fulfilled") {
      publicAuctions.value = pub.value.items || []
      for (const row of publicAuctions.value) {
        const minBid = normalizeDraftBid(row)
        if (!bidDraftByAuction[row.id] || bidDraftByAuction[row.id] < minBid) {
          bidDraftByAuction[row.id] = minBid
        }
      }
    } else {
      publicAuctions.value = []
    }
    if (mine.status === "fulfilled") {
      myAuctions.value = mine.value.items || []
    } else {
      myAuctions.value = []
    }
    if (selectedAuctionDetail.value?.id) {
      const found = [...publicAuctions.value, ...myAuctions.value].find((row) => row.id === selectedAuctionDetail.value?.id)
      if (found) {
        selectedAuctionDetail.value = found
      } else {
        await openAuctionDetail(selectedAuctionDetail.value.id, false)
      }
    }
  }

  function resetCreateDraft() {
    createDraft.instanceId = instanceAuctionOptions.value[0]?.instanceId || 0
    createDraft.startPrice = Math.max(2, createMinStartPrice.value + 1)
    createDraft.minBidIncrement = createMinBidIncrement.value
    buyoutAutoFillEnabled.value = true
    createDraft.buyoutPrice = createMinBuyout.value
    createDraft.durationHours = 24
  }

  function openCreateDialog() {
    resetCreateDraft()
    createDialogVisible.value = true
  }

  function openInstanceSelector() {
    instanceSelectorVisible.value = true
  }

  function onConfirmSelectInstance(instanceId: number | null) {
    const selected = instanceAuctionOptions.value.find((item) => item.instanceId === Number(instanceId || 0))
    createDraft.instanceId = selected?.instanceId || 0
    createDraft.startPrice = Math.max(createDraft.startPrice, createMinStartPrice.value + 1)
    syncBuyoutFloor()
  }

  function syncBuyoutFloor(force = false) {
    if (!force && !buyoutAutoFillEnabled.value) return
    const floor = createMinBuyout.value
    if (createDraft.buyoutPrice <= 0 || createDraft.buyoutPrice < floor) {
      createDraft.buyoutPrice = floor
    }
  }

  function syncMinBidIncrementFloor() {
    const minValue = createMinBidIncrement.value
    const maxValue = createMaxBidIncrement.value
    if (createDraft.minBidIncrement < minValue) {
      createDraft.minBidIncrement = minValue
      return
    }
    if (createDraft.minBidIncrement > maxValue) {
      createDraft.minBidIncrement = maxValue
    }
  }

  watch(() => createDraft.buyoutPrice, (next, prev) => {
    if (next === prev) return
    if (next <= 0 && (prev ?? 0) > 0) {
      buyoutAutoFillEnabled.value = false
      return
    }
    if (next > 0) {
      buyoutAutoFillEnabled.value = true
      if (next < createMinBuyout.value) createDraft.buyoutPrice = createMinBuyout.value
    }
  })

  watch(createMinBuyout, () => {
    syncBuyoutFloor()
  })

  watch(createMinStartPrice, (floor) => {
    const nextMinStart = Math.max(2, floor + 1)
    if (createDraft.startPrice < nextMinStart) {
      createDraft.startPrice = nextMinStart
    }
  })

  watch(() => createDraft.startPrice, () => {
    syncMinBidIncrementFloor()
  })

  watch(() => createDraft.durationHours, (next) => {
    const normalized = normalizeDurationHours(next)
    if (normalized !== next) createDraft.durationHours = normalized
  })

  async function createAuction() {
    if (!canAuction.value) {
      ElMessage.warning(t("game.auctionReputationTooLow", { required: requiredRepLevel }))
      return
    }
    if (!store.token || !canCreateAuction.value) {
      ElMessage.warning(t("game.auctionCreateFailed"))
      return
    }
    try {
      await apiPost("/api/auction/list-instance", store.token, {
        instanceId: createDraft.instanceId,
        startPrice: createDraft.startPrice,
        buyoutPrice: createDraft.buyoutPrice > 0 ? createDraft.buyoutPrice : 0,
        minBidIncrement: createDraft.minBidIncrement,
        durationSeconds: normalizeDurationHours(createDraft.durationHours) * 3600
      })
      ElMessage.success(t("game.auctionCreateSuccess"))
      createDialogVisible.value = false
      await loadAll()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.auctionCreateFailed"))
    }
  }

  async function placeBid(auctionID: number) {
    if (!store.token) return
    if (!canAuction.value) {
      ElMessage.warning(t("game.auctionReputationTooLow", { required: requiredRepLevel }))
      return
    }
    const bidAmount = Math.max(1, Math.floor(bidDraftByAuction[auctionID] || 1))
    try {
      await apiPost("/api/auction/bid", store.token, { auctionId: auctionID, bidAmount })
      ElMessage.success(t("game.auctionBidSuccess"))
      await loadAll()
      if (selectedAuctionDetail.value?.id === auctionID) {
        await openAuctionDetail(auctionID, false)
      }
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.auctionBidFailed"))
    }
  }

  async function cancelAuction(auctionID: number) {
    if (!store.token) return
    try {
      await apiPost("/api/auction/cancel", store.token, { auctionId: auctionID })
      ElMessage.success(t("game.auctionCancelSuccess"))
      if (selectedAuctionDetail.value?.id === auctionID) {
        selectedAuctionDetail.value = null
      }
      await loadAll()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.auctionCancelFailed"))
    }
  }

  function adjustBid(auctionID: number, delta: number) {
    const current = Math.max(1, Math.floor(bidDraftByAuction[auctionID] || 1))
    bidDraftByAuction[auctionID] = Math.max(1, current + delta)
  }

  function adjustBidPercent(auctionID: number, percent: number) {
    const current = Math.max(1, Math.floor(bidDraftByAuction[auctionID] || 1))
    const delta = Math.max(1, Math.ceil(current * percent))
    bidDraftByAuction[auctionID] = Math.max(1, current + delta)
  }

  async function openBidHistory(auctionID: number) {
    if (!store.token) return
    try {
      const data = await apiGet<{ items: AuctionBid[] }>(`/api/auction/bids?auctionId=${auctionID}&limit=20`, store.token)
      bidHistory.value = data.items || []
      bidHistoryVisible.value = true
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.auctionBidFailed"))
    }
  }

  async function openAuctionDetail(auctionID: number, switchTab = true) {
    if (!store.token || auctionID <= 0) return
    try {
      const data = await apiGet<{ item: AuctionListing }>(`/api/auction/detail?auctionId=${auctionID}`, store.token)
      selectedAuctionDetail.value = data.item
      const minBid = normalizeDraftBid(data.item)
      if (!bidDraftByAuction[auctionID] || bidDraftByAuction[auctionID] < minBid) {
        bidDraftByAuction[auctionID] = minBid
      }
      if (switchTab) activeTab.value = "detail"
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.auctionBidFailed"))
    }
  }

  return {
    store,
    activeTab,
    auctionFilterOptions,
    filterItemId,
    publicStatusFilter,
    searchKeyword,
    activeAuctionCount,
    myActiveAuctionCount,
    highestBidPrice,
    publicAuctions,
    myAuctions,
    selectedAuctionDetail,
    createDraft,
    createDialogVisible,
    instanceSelectorVisible,
    instanceAuctionInstanceIDs,
    selectedInstanceItemId,
    createMinStartPrice,
    createMinBuyout,
    createMinBidIncrement,
    createMaxBidIncrement,
    createFeeEstimate,
    createSellerAmount,
    auctionFeeRateBps: AUCTION_FEE_RATE_BPS,
    antiSnipeTriggerSeconds: AUCTION_ANTI_SNIPE_TRIGGER_SECONDS,
    antiSnipeExtendSeconds: AUCTION_ANTI_SNIPE_EXTEND_SECONDS,
    selectedCancelFee,
    canCreateAuction,
    canAuction,
    repLevel,
    requiredRepLevel,
    filteredPublicAuctions,
    filteredMyAuctions,
    bidDraftByAuction,
    bidHistoryVisible,
    bidHistory,
    itemLabel,
    auctionItemLabel,
    formatEndsAt,
    formatRemaining,
    auctionStatusLabel,
    loadAll,
    openCreateDialog,
    openInstanceSelector,
    onConfirmSelectInstance,
    createAuction,
    placeBid,
    cancelAuction,
    adjustBid,
    adjustBidPercent,
    openBidHistory,
    openAuctionDetail
  }
}
