import { computed, ref, watch, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { useEquipmentStore } from "@stores/equipment"
import { useInventoryStore } from "@stores/inventory"
import { useRoleStateStore, type RoleKey } from "@stores/role-state"
import { apiGet, apiPost, createIdempotencyKey } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { useEquipmentSlots } from "@/composables/game/attributes/useEquipmentSlots"
import { useRoleAttributeModel } from "@/composables/game/roles/useRoleAttributeModel"
import { PLAYER_FISHING_SLOT_IDS } from "@services/specialization"
import type { EquipmentPanelTab, EquipmentSlotType } from "@/types/equipment"

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type KittyCombatProfile = {
  kitty?: Record<string, unknown>
  attributes?: Array<Record<string, unknown>>
  skills?: Array<Record<string, unknown>>
  equipment?: Array<Record<string, unknown>>
}

type KittyBattlePointState = {
  totalPoints: number
  spentPoints: number
  available: number
  resetRequirement: {
    itemId: string
    count: number
  }
}

type KittyMutationSyncPayload = {
  battlePoints?: KittyBattlePointState
  roleAttributePatch?: Record<string, unknown>
  roleEquipmentPatch?: Record<string, unknown>
}

export function useKittyCombatProfile(token: Ref<string>, kittyId: Ref<number>, t: TranslateFn) {
  const equipmentStore = useEquipmentStore()
  const inventoryStore = useInventoryStore()
  const roleStateStore = useRoleStateStore()
  const combatLoading = ref(false)
  const combatSaving = ref(false)
  const combatProfile = ref<KittyCombatProfile | null>(null)
  const kittyEquipmentTypeActiveTab = ref<EquipmentPanelTab>("battle")
  const kittyEffectiveEquipmentType = computed<EquipmentSlotType>(() =>
    kittyEquipmentTypeActiveTab.value === "fishing" ? "work" : "battle"
  )
  const equippingInstanceId = ref<number | null>(null)
  const equipSuccessVersion = ref(0)
  const allocateDialogVisible = ref(false)
  const allocateSubmitting = ref(false)
  const resetSubmitting = ref(false)
  const allocateBatchValues = ref<Record<string, number>>({})
  const battlePointState = ref<KittyBattlePointState>({
    totalPoints: 0,
    spentPoints: 0,
    available: 0,
    resetRequirement: {
      itemId: "",
      count: 1
    }
  })
  const roleKey = computed<RoleKey>(() => `kitty:${Math.max(0, Math.trunc(Number(kittyId.value || 0)))}` as RoleKey)
  const roleState = computed(() => roleStateStore.ensureRole(roleKey.value))
  const combatReady = computed(() => Boolean(combatProfile.value || roleState.value.initialized.profile || roleState.value.initialized.attributes))

  const kittyCombatConfigPath = computed(() =>
    kittyId.value > 0
      ? `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/combat-config`
      : ""
  )
  const kittyRuleConfigPath = computed(() =>
    kittyId.value > 0
      ? `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/rules/config`
      : ""
  )
  const combatFormula = computed(() => ({}))
  const { attrRows: combatAttrRows, unitAttributes: combatUnitAttributes } = useRoleAttributeModel(roleKey, combatFormula, t)
  const allocatableRows = computed(() =>
    combatAttrRows.value.filter((row) => row.category === "battle" && row.allowBattlePointLevelUp)
  )
  const allocateBatchTotal = computed(() =>
    Object.values(allocateBatchValues.value).reduce((sum, points) => sum + Math.max(0, Math.trunc(Number(points || 0))), 0)
  )
  const allocateBatchRemaining = computed(() => Math.max(0, battlePointState.value.available - allocateBatchTotal.value))
  const combatEquipment = computed(() =>
    roleState.value.equipment
  )
  const transferablePlayerFishingEquipment = computed(() =>
    equipmentStore.equippedItems.filter((instance) => {
      if (instance.equipType !== "work") return false
      const occupiedSlots = String(instance.equippedSlot || "")
        .split(",")
        .map((slot) => slot.trim())
      return occupiedSlots.some((slot) => PLAYER_FISHING_SLOT_IDS.includes(slot as typeof PLAYER_FISHING_SLOT_IDS[number]))
    })
  )
  const kittyVisibleEquipmentSlots = computed(() =>
    kittyEquipmentTypeActiveTab.value === "fishing"
      ? [...PLAYER_FISHING_SLOT_IDS]
      : roleState.value.capabilities.visibleSlots.filter((slot) => !PLAYER_FISHING_SLOT_IDS.includes(slot as typeof PLAYER_FISHING_SLOT_IDS[number]))
  )
  const { slotCards: kittySlotCards } = useEquipmentSlots(kittyEffectiveEquipmentType, {
    equippedItems: combatEquipment,
    transferableEquippedItems: transferablePlayerFishingEquipment,
    currentAttributes: combatUnitAttributes,
    visibleSlots: kittyVisibleEquipmentSlots
  })

  watch(
    () => [token.value, kittyId.value] as const,
    ([nextToken, nextKittyId], oldValue) => {
      const prevKittyId = oldValue?.[1]
      if (nextKittyId !== prevKittyId) combatProfile.value = null
      if (nextToken && nextKittyId > 0) void openCombatProfile()
    },
    { immediate: true }
  )

  async function openCombatProfile() {
    if (!token.value || kittyId.value <= 0) return
    await refreshOwnedEquipmentSources()
    if (roleState.value.initialized.profile && combatProfile.value) {
      await loadBattlePoints()
      return
    }
    await loadCombatProfile()
  }

  async function loadCombatProfile() {
    if (!token.value || kittyId.value <= 0) return
    combatLoading.value = !roleState.value.initialized.profile
    try {
      const profile = await apiGet<KittyCombatProfile>(
        `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/combat-profile`,
        token.value
      )
      combatProfile.value = profile
      roleStateStore.initializeKittyFromProfile(kittyId.value, profile)
      await loadBattlePoints()
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.catteryLoadFailed"))
    } finally {
      combatLoading.value = false
    }
  }

  async function loadBattlePoints() {
    if (!token.value || kittyId.value <= 0) return
    const payload = await apiGet<{
      totalPoints: number
      spentPoints: number
      available: number
      resetRequirement?: { itemId?: string; count?: number }
    }>(
      `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/battle-points`,
      token.value
    )
    applyBattlePointPayload(payload)
  }

  function applyBattlePointPayload(payload: {
    totalPoints?: number
    spentPoints?: number
    available?: number
    resetRequirement?: { itemId?: string; count?: number }
  } | null | undefined) {
    battlePointState.value = {
      totalPoints: Number(payload?.totalPoints || 0),
      spentPoints: Number(payload?.spentPoints || 0),
      available: Number(payload?.available || 0),
      resetRequirement: {
        itemId: String(payload?.resetRequirement?.itemId || "").trim(),
        count: Math.max(1, Number(payload?.resetRequirement?.count || 1))
      }
    }
    roleStateStore.setRoleBattlePoints(roleKey.value, battlePointState.value)
  }

  function applyMutationSyncPayload(payload: KittyMutationSyncPayload | null | undefined) {
    if (!payload || typeof payload !== "object") return
    roleStateStore.patchRoleMutationPayload(payload)
    if (payload.battlePoints) applyBattlePointPayload(payload.battlePoints)
  }

  async function equipKittyToSlot(payload: { slot: string; instanceId: number; equipType: "work" | "battle" }) {
    if (!token.value || kittyId.value <= 0 || !payload.instanceId) return
    equippingInstanceId.value = payload.instanceId
    combatSaving.value = true
    try {
      const result = await apiPost<KittyMutationSyncPayload>(
        `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/equip`,
        token.value,
        { instanceId: Number(payload.instanceId), slot: payload.slot, equipType: payload.equipType }
      )
      applyMutationSyncPayload(result)
      try {
        await refreshOwnedEquipmentSources(true)
      } catch (refreshError) {
        console.warn("[cattery] refresh equipment sources after equip failed", refreshError)
      }
      equipSuccessVersion.value += 1
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    } finally {
      equippingInstanceId.value = null
      combatSaving.value = false
    }
  }

  async function refreshCombatEquipment() {
    if (!roleState.value.initialized.profile) await loadCombatProfile()
    await refreshOwnedEquipmentSources(true)
  }

  async function refreshOwnedEquipmentSources(force = false) {
    if (!token.value) return
    await Promise.all([
      inventoryStore.initialize(token.value, force),
      equipmentStore.initialize(token.value, force)
    ])
    roleStateStore.applyEquipmentSnapshot("player:self", equipmentStore.equippedItems, equipmentStore.equippedVersion)
  }

  function allocationMaxFor(attrKey: string) {
    const current = Math.max(0, Math.trunc(Number(allocateBatchValues.value[attrKey] || 0)))
    const assignedOthers = Object.entries(allocateBatchValues.value)
      .filter(([key]) => key !== attrKey)
      .reduce((sum, [, points]) => sum + Math.max(0, Math.trunc(Number(points || 0))), 0)
    const remaining = Math.max(0, battlePointState.value.available - assignedOthers)
    return current + remaining
  }

  function setBatchAllocation(attrKey: string, rawValue: number) {
    const normalized = Math.max(0, Math.trunc(Number(rawValue || 0)))
    const maxAllowed = allocationMaxFor(attrKey)
    allocateBatchValues.value = {
      ...allocateBatchValues.value,
      [attrKey]: Math.min(maxAllowed, normalized)
    }
  }

  function handleBatchAllocationInput(attrKey: string, value: number | null | undefined) {
    setBatchAllocation(attrKey, Number(value ?? 0))
  }

  function openAllocateDialog() {
    const next: Record<string, number> = {}
    for (const row of allocatableRows.value) {
      next[row.key] = 0
    }
    allocateBatchValues.value = next
    allocateDialogVisible.value = true
  }

  function closeAllocateDialog() {
    allocateDialogVisible.value = false
  }

  async function submitAllocateBatch() {
    if (!token.value || kittyId.value <= 0 || allocateSubmitting.value) return
    const allocations = Object.entries(allocateBatchValues.value).reduce<Record<string, number>>((acc, [attrKey, points]) => {
      const normalized = Math.max(0, Math.trunc(Number(points || 0)))
      if (normalized > 0) {
        acc[attrKey] = normalized
      }
      return acc
    }, {})
    if (Object.keys(allocations).length <= 0) return
    allocateSubmitting.value = true
    try {
      const payload = await apiPost<KittyMutationSyncPayload>(
        `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/battle-points/allocate_batch`,
        token.value,
        { allocations },
        { "X-Idempotency-Key": createIdempotencyKey("kitty-battle-points-allocate-batch") }
      )
      applyMutationSyncPayload(payload)
      closeAllocateDialog()
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    } finally {
      allocateSubmitting.value = false
    }
  }

  async function resetBattlePoints() {
    if (!token.value || kittyId.value <= 0 || resetSubmitting.value) return
    resetSubmitting.value = true
    try {
      const payload = await apiPost<KittyMutationSyncPayload>(
        `/api/cattery/kitties/${encodeURIComponent(String(kittyId.value))}/battle-points/reset`,
        token.value,
        {},
        { "X-Idempotency-Key": createIdempotencyKey("kitty-battle-points-reset") }
      )
      applyMutationSyncPayload(payload)
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    } finally {
      resetSubmitting.value = false
    }
  }

  function kittyStatusLabel(status: string) {
    if (status === "in_combat") return "战斗中"
    if (status === "in_life_queue") return "生活队列中"
    return "空闲"
  }

  return {
    combatLoading,
    combatSaving,
    combatProfile,
    combatReady,
    combatRoleKey: roleKey,
    kittyEquipmentTypeActiveTab,
    equippingInstanceId,
    equipSuccessVersion,
    allocateDialogVisible,
    allocateSubmitting,
    resetSubmitting,
    battlePointState,
    kittyCombatConfigPath,
    kittyRuleConfigPath,
    combatUnitAttributes,
    allocatableRows,
    allocateBatchValues,
    allocateBatchTotal,
    allocateBatchRemaining,
    kittySlotCards,
    loadCombatProfile,
    equipKittyToSlot,
    refreshCombatEquipment,
    openAllocateDialog,
    closeAllocateDialog,
    allocationMaxFor,
    handleBatchAllocationInput,
    submitAllocateBatch,
    resetBattlePoints,
    kittyStatusLabel
  }
}
