import { computed, onBeforeUnmount, ref, watch, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { APIClientError, createIdempotencyKey } from "@services/api"
import { groupMarketQuotes, instanceMarketSelectable, marketRead, marketWrite, type MarketDetail, type MarketGroup, type MarketListing, type MarketOrder, type MarketQuote } from "@services/instance-market"
import { getItemRealValuePrice, isValidMarketPrice } from "@services/market-pricing"
import { normalizeMarketListings } from "@services/instance-market"
import { matchesMarketCatalog } from '@services/market-catalog'

export function useInstanceMarket(itemId: Ref<string>, history: Ref<boolean>, historyView: Ref<'mine' | 'orders'> = ref('mine'), catalogValues: Ref<string> = ref('')) {
  const game = useGameStore()
  const inventory = useInventoryStore()
  const { t } = useI18n()
  const view = ref<"groups" | "listings" | "mine" | "orders">("groups")
  const selectedGroup = ref<MarketGroup | null>(null)
  const rows = ref<Array<MarketGroup | MarketListing | MarketOrder>>([])
  const offset = ref(0)
  const hasMore = ref(false)
  const loading = ref(false)
  const error = ref("")
  const listingOpen = ref(false)
  const selectedIds = ref<number[]>([])
  const inventoryOffset = ref(0)
  const quoteGroups = ref<MarketQuote[][]>([])
  const quoteGroup = ref(0)
  const quantity = ref(1)
  const price = ref(1)
  const buyTarget = ref<MarketListing | null>(null)
  const buyQuantity = ref(1)
  const details = ref<MarketDetail[]>([])
  const detailOpen = ref(false)
  const detailItemId = ref("")
  const busy = ref(false)
  const pending = ref<{ command: string; body: unknown; key: string } | null>(null)
  let sequence = 0
  let detailSequence = 0
  let alive = true
  const principal = () => `${game.userId}:${game.token}`
  const context = () => `${principal()}:${itemId.value}:${history.value}:${catalogValues.value}`
  const eligible = computed(() => inventory.instances.filter(row => instanceMarketSelectable(row, itemId.value) && matchesMarketCatalog(itemId.value, row, catalogValues.value)))
  const visibleInventory = computed(() => eligible.value.slice(inventoryOffset.value, inventoryOffset.value + 40))
  const chosenQuotes = computed(() => quoteGroups.value[quoteGroup.value] || [])
  const bestBuyListing = computed(() => view.value === 'listings' ? (rows.value as MarketListing[]).find(row => row.status === 'active' && !row.isOwn && row.canBuy) : undefined)
  const canList = computed(() => chosenQuotes.value.length > 0 && Number.isSafeInteger(price.value) && price.value > 0 && price.value <= 1000000000000 && Number.isSafeInteger(quantity.value) && quantity.value > 0 && quantity.value <= chosenQuotes.value.length && isValidMarketPrice(price.value, getItemRealValuePrice(itemId.value)))
  const canBuy = computed(() => buyTarget.value && Number.isSafeInteger(buyQuantity.value) && buyQuantity.value > 0 && buyQuantity.value <= buyTarget.value.quantity)

  function message(err: unknown) {
    const code = err instanceof APIClientError ? err.code : ""
    if (code === "INSTANCE_MARKET_CHANGED" || code === "INSTANCE_MARKET_SOLD") return t("game.instanceMarket.changed")
    if (code === "INSTANCE_MARKET_UNAVAILABLE") return t("game.instanceMarket.unavailable")
    return t("game.instanceMarket.error")
  }
  async function refresh() {
    const token = game.token
    if (!token || (!history.value && !itemId.value)) return
    const seq = ++sequence
    const owner = context()
    loading.value = true
    error.value = ""
    try {
      const result = await marketRead<MarketGroup | MarketListing | MarketOrder>(token, view.value, {
        itemId: history.value ? "" : itemId.value, groupKey: selectedGroup.value?.groupKey || "", offset: offset.value,
        ...(view.value === 'groups' && catalogValues.value ? { catalogValues: catalogValues.value } : {})
      })
      if (!alive || seq !== sequence || owner !== context()) return
      rows.value = view.value === 'listings' || view.value === 'mine'
        ? normalizeMarketListings((result.items || []) as MarketListing[], game.userId, offset.value)
        : view.value === 'groups' && catalogValues.value
          ? (result.items || []).filter(row => matchesMarketCatalog(itemId.value, (row as MarketGroup).snapshot, catalogValues.value))
          : result.items || []
      hasMore.value = result.hasMore
    } catch (err) {
      if (!alive || seq !== sequence || owner !== context()) return
      error.value = message(err)
      rows.value = []
      hasMore.value = false
    } finally { if (alive && seq === sequence && owner === context()) loading.value = false }
  }
  function openGroup(group: MarketGroup) { selectedGroup.value = group; offset.value = 0; view.value = "listings" }
  function back() { selectedGroup.value = null; offset.value = 0; view.value = "groups" }
  function toggle(id: number, value: boolean) {
    if (value && selectedIds.value.length < 100) selectedIds.value = [...selectedIds.value, id]
    else if (!value) selectedIds.value = selectedIds.value.filter(entry => entry !== id)
  }
  async function openList() {
    if (busy.value || pending.value) return
    selectedIds.value = []; quoteGroups.value = []; quoteGroup.value = 0; quantity.value = 1; inventoryOffset.value = 0
    price.value = getItemRealValuePrice(itemId.value)
    listingOpen.value = true
    const owner = principal()
    try { await inventory.initialize(game.token, true) } catch { if (alive && owner === principal()) ElMessage.error(t("game.instanceMarket.error")) }
  }
  async function preview() {
    if (busy.value || pending.value || selectedIds.value.length === 0) return
    const owner = context()
    busy.value = true
    try {
      const result = await marketWrite<{ items: MarketQuote[] }>(game.token, "quote", { items: selectedIds.value.map(instanceId => ({ instanceId })) }, createIdempotencyKey("market-preview"))
      if (!alive || owner !== context()) return
      quoteGroups.value = groupMarketQuotes(result.items)
      quoteGroup.value = 0
      quantity.value = quoteGroups.value[0]?.length || 1
    } catch (err) { if (alive && owner === context()) ElMessage.error(message(err)) }
    finally { if (alive && owner === context()) busy.value = false }
  }
  // Keep an uncertain request's exact payload and idempotency key. A successful
  // write is never reclassified as failure merely because a later GET failed.
  async function commit(command: string, body: unknown, success: string) {
    if (busy.value || (pending.value && pending.value.command !== command)) return
    const owner = context()
    const token = game.token
    if (!pending.value) pending.value = { command, body, key: createIdempotencyKey(`instance-market-${command}`) }
    const request = pending.value
    busy.value = true
    try {
      await marketWrite(token, request.command, request.body, request.key)
      if (!alive || owner !== context()) return
      pending.value = null
      listingOpen.value = false
      buyTarget.value = null
      ElMessage.success(t(success))
      await refresh()
      if (!alive || owner !== context()) return
      if (error.value) ElMessage.warning(t("game.instanceMarket.refreshFailed"))
      // WS synchronization owns the authoritative inventory refresh; this also
      // covers clients temporarily disconnected from the socket.
      try { await inventory.initialize(token, true) } catch { if (alive && owner === context()) ElMessage.warning(t("game.instanceMarket.refreshFailed")) }
    } catch (err) {
      if (!alive || owner !== context()) return
      if (err instanceof APIClientError && err.status && err.status < 500 && err.code !== "REQUEST_DUPLICATE") {
        pending.value = null
        ElMessage.error(message(err))
      } else ElMessage.warning(t("game.instanceMarket.pending"))
    } finally { if (alive && owner === context()) busy.value = false }
  }
  async function confirmList() {
    if (!pending.value && !canList.value) return
    await commit("list", { itemId: itemId.value, price: price.value, items: chosenQuotes.value.slice(0, quantity.value).map(q => ({ instanceId: q.instanceId, quoteToken: q.quoteToken })) }, "game.instanceMarket.listed")
  }
  function openBuy(row: MarketListing) {
    if (busy.value || pending.value || row.id !== bestBuyListing.value?.id) return
    buyTarget.value = row; buyQuantity.value = 1
  }
  function buyFirst() {
    if (bestBuyListing.value) openBuy(bestBuyListing.value)
    else if (offset.value > 0) offset.value = 0
    else void refresh()
  }
  async function confirmBuy() {
    if (!pending.value && !canBuy.value) return
    const row = buyTarget.value
    await commit("buy", { listingId: row?.id, quantity: buyQuantity.value, price: row?.price, groupKey: row?.groupKey }, "game.instanceMarket.success")
  }
  async function cancelListing(row: MarketListing) {
    if (pending.value && (pending.value.command !== "cancel" || (pending.value.body as { listingId: number }).listingId !== row.id)) return
    await commit("cancel", { listingId: row.id }, "game.instanceMarket.cancelled")
  }
  async function retryPending() {
    if (!pending.value) return
    const request = pending.value
    const success = request.command === "list" ? "listed" : request.command === "cancel" ? "cancelled" : "success"
    await commit(request.command, request.body, `game.instanceMarket.${success}`)
  }
  async function showDetails(listingId: number, id: string) {
    const owner = principal(); const seq = ++detailSequence
    details.value = []; detailItemId.value = id; detailOpen.value = true
    try {
      const result = await marketRead<MarketDetail>(game.token, "details", { listingId })
      if (alive && owner === principal() && seq === detailSequence) details.value = result.items
    } catch (err) { if (alive && owner === principal() && seq === detailSequence) ElMessage.error(message(err)) }
  }
  watch([itemId, history, historyView, catalogValues, () => game.token, () => game.userId], () => {
    sequence++; detailSequence++; rows.value = []; selectedGroup.value = null; offset.value = 0
    view.value = history.value ? historyView.value : "groups"
    listingOpen.value = false; buyTarget.value = null; detailOpen.value = false; details.value = []; pending.value = null; busy.value = false
    selectedIds.value = []; quoteGroups.value = []
    void refresh()
  }, { immediate: true })
  watch([view, offset], () => { void refresh() })
  watch(quoteGroup, () => { quantity.value = chosenQuotes.value.length || 1 })
  onBeforeUnmount(() => { alive = false; sequence++; detailSequence++ })
  return { game, inventory, view, selectedGroup, rows, offset, hasMore, loading, error, listingOpen, selectedIds, inventoryOffset, quoteGroups, quoteGroup, quantity, price, buyTarget, buyQuantity, details, detailOpen, detailItemId, busy, pending, eligible, visibleInventory, chosenQuotes, canList, canBuy, bestBuyListing, buyFirst, refresh, openGroup, back, toggle, openList, preview, confirmList, openBuy, confirmBuy, cancelListing, retryPending, showDetails }
}
