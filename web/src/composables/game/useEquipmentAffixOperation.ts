import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRouter } from "vue-router"
import { resolveErrorMessage } from "@services/error-messages"
import { getLocalizedItemName } from "@services/item-label"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useEquipmentStore } from "@stores/equipment"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import type { ItemDetailResponse } from "@/types/equipment"

export function useEquipmentAffixOperation() {
  const { t } = useI18n()
  const router = useRouter()
  const gameStore = useGameStore()
  const inventoryStore = useInventoryStore()
  const equipmentStore = useEquipmentStore()
  const instanceDetailStore = useItemInstanceDetailStore()

  const loading = ref(false)
  const detail = ref<ItemDetailResponse | null>(null)
  const target = computed(() => equipmentStore.operationTarget)

  onMounted(() => {
    gameStore.restoreToken()
    void loadDetail().catch(() => undefined)
  })

  async function loadDetail(refreshInventory = false) {
    if (!gameStore.token || !target.value) return
    loading.value = true
    try {
      detail.value = await instanceDetailStore.fetchByInstance(gameStore.token, target.value.instanceId, {
        itemId: target.value.templateItemId,
        force: true
      })
      if (refreshInventory) {
        await inventoryStore.initialize(gameStore.token, true)
      }
    } catch (err) {
      throw new Error(resolveErrorMessage(t, err, "game.itemDetailLoadFailed"))
    } finally {
      loading.value = false
    }
  }

  async function refreshAll() {
    await loadDetail(true)
  }

  async function clearDestroyedTarget(instanceId?: number) {
    const id = Number(instanceId || target.value?.instanceId || 0)
    if (id > 0) {
      instanceDetailStore.invalidate(id)
    }
    detail.value = null
    equipmentStore.clearOperationTarget()
    if (gameStore.token) {
      await inventoryStore.initialize(gameStore.token, true)
    }
  }

  function itemLabel(itemId: string) {
    return getLocalizedItemName(t, itemId, { fallbackText: itemId })
  }

  function goWarehouse() {
    void router.push("/game/warehouse")
  }

  return {
    t,
    gameStore,
    inventoryStore,
    loading,
    detail,
    target,
    itemLabel,
    goWarehouse,
    loadDetail,
    refreshAll,
    clearDestroyedTarget
  }
}
