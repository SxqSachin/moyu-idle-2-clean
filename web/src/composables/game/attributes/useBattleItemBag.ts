import { computed, ref } from "vue"
import { apiGet, apiPost } from "@services/api"

type BattleItemBagEntry = {
  slotNo: number
  itemId: string
  quantity: number
}

type BattleItemBagPayload = {
  items?: BattleItemBagEntry[]
  limits?: {
    maxKinds?: number
    maxTotalQuantity?: number
  }
}

type InventoryInstance = {
  templateItemId?: string
  quantity?: number
  state?: string
}

export function useBattleItemBag(token: () => string) {
  const loading = ref(false)
  const mutating = ref(false)
  const battleBagEntries = ref<BattleItemBagEntry[]>([])
  const limits = ref({
    maxKinds: 4,
    maxTotalQuantity: 1000
  })
  const inventoryCountByItem = ref<Record<string, number>>({})

  const bagKinds = computed(() => battleBagEntries.value.filter((row) => row.quantity > 0).length)
  const bagTotalQuantity = computed(() => battleBagEntries.value.reduce((sum, row) => sum + Math.max(0, row.quantity), 0))

  async function loadBag() {
    const currentToken = token()
    if (!currentToken) return
    const payload = await apiGet<BattleItemBagPayload>("/api/battle/items/bag", currentToken)
    const list = Array.isArray(payload.items) ? payload.items : []
    battleBagEntries.value = list
      .map((row) => ({
        slotNo: Math.max(1, Math.trunc(Number(row.slotNo || 1))),
        itemId: String(row.itemId || "").trim(),
        quantity: Math.max(0, Math.trunc(Number(row.quantity || 0)))
      }))
      .filter((row) => row.itemId)
      .sort((a, b) => a.slotNo - b.slotNo)
    limits.value = {
      maxKinds: Math.max(1, Math.trunc(Number(payload.limits?.maxKinds || 4))),
      maxTotalQuantity: Math.max(1, Math.trunc(Number(payload.limits?.maxTotalQuantity || 1000)))
    }
  }

  async function loadInventory() {
    const currentToken = token()
    if (!currentToken) return
    const payload = await apiGet<{ items?: InventoryInstance[] }>("/api/inventory/instances", currentToken)
    const list = Array.isArray(payload.items) ? payload.items : []
    const next: Record<string, number> = {}
    for (const row of list) {
      const state = String(row.state || "").trim().toLowerCase()
      if (state && state !== "inventory") continue
      const itemID = String(row.templateItemId || "").trim()
      if (!itemID) continue
      next[itemID] = (next[itemID] || 0) + Math.max(0, Math.trunc(Number(row.quantity || 0)))
    }
    inventoryCountByItem.value = next
  }

  async function loadAll() {
    const currentToken = token()
    if (!currentToken) return
    loading.value = true
    try {
      await Promise.all([loadBag(), loadInventory()])
    } finally {
      loading.value = false
    }
  }

  async function addQuantity(slotNo: number, itemID: string, quantity: number) {
    const currentToken = token()
    if (!currentToken || mutating.value) return
    const normalizedSlotNo = Math.max(1, Math.trunc(Number(slotNo || 1)))
    const qty = Math.max(1, Math.trunc(Number(quantity || 0)))
    mutating.value = true
    try {
      await apiPost("/api/battle/items/bag/add", currentToken, { slotNo: normalizedSlotNo, itemId: itemID, quantity: qty })
      applyAddLocalDelta(normalizedSlotNo, itemID, qty)
      await loadBag()
      void loadInventory()
    } finally {
      mutating.value = false
    }
  }

  async function takeBackSlot(slotNo: number) {
    const currentToken = token()
    if (!currentToken || mutating.value) return
    const normalizedSlotNo = Math.max(1, Math.trunc(Number(slotNo || 1)))
    mutating.value = true
    try {
      const before = battleBagEntries.value.find((row) => row.slotNo === normalizedSlotNo)
      await apiPost("/api/battle/items/bag/takeback", currentToken, { slotNo: normalizedSlotNo })
      applyTakeBackLocalDelta(normalizedSlotNo, before)
      await loadBag()
      void loadInventory()
    } finally {
      mutating.value = false
    }
  }

  function applyAddLocalDelta(slotNo: number, itemID: string, qty: number) {
    const normalizedItemID = String(itemID || "").trim()
    if (!normalizedItemID || qty <= 0) return
    const rows = battleBagEntries.value.slice()
    const idx = rows.findIndex((row) => row.slotNo === slotNo)
    if (idx >= 0) {
      rows[idx] = {
        ...rows[idx],
        itemId: normalizedItemID,
        quantity: Math.max(0, Math.trunc(Number(rows[idx].quantity || 0) + qty))
      }
    } else {
      rows.push({
        slotNo,
        itemId: normalizedItemID,
        quantity: Math.max(0, Math.trunc(Number(qty)))
      })
      rows.sort((a, b) => a.slotNo - b.slotNo)
    }
    battleBagEntries.value = rows
    inventoryCountByItem.value = {
      ...inventoryCountByItem.value,
      [normalizedItemID]: Math.max(0, Math.trunc(Number(inventoryCountByItem.value[normalizedItemID] || 0) - qty))
    }
  }

  function applyTakeBackLocalDelta(slotNo: number, before: BattleItemBagEntry | undefined) {
    const itemID = String(before?.itemId || "").trim()
    const qty = Math.max(0, Math.trunc(Number(before?.quantity || 0)))
    battleBagEntries.value = battleBagEntries.value.filter((row) => row.slotNo !== slotNo)
    if (!itemID || qty <= 0) return
    inventoryCountByItem.value = {
      ...inventoryCountByItem.value,
      [itemID]: Math.max(0, Math.trunc(Number(inventoryCountByItem.value[itemID] || 0) + qty))
    }
  }

  return {
    loading,
    mutating,
    limits,
    bagKinds,
    bagTotalQuantity,
    battleBagEntries,
    inventoryCountByItem,
    loadAll,
    addQuantity,
    takeBackSlot
  }
}
