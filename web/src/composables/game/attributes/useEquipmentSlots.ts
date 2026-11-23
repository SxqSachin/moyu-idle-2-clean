import { computed, type Ref } from "vue"
import { useInventoryStore } from "@stores/inventory"
import { useEquipmentStore } from "@stores/equipment"
import { useRoleStateStore } from "@stores/role-state"
import { getEquipmentAllowedSlots, getEquipmentEquipType, getEquipmentItemDefinition, getEquipmentStatusRequirement } from "@services/item-definition"
import { PLAYER_BASE_BATTLE_SLOT_IDS, PLAYER_FISHING_SLOT_IDS, PLAYER_SPECIALIZATION_BATTLE_SLOT_IDS, PLAYER_WORK_SLOT_IDS } from "@services/specialization"
import { evaluateStatusRequire } from "@services/status-require"
import type { EquipmentItemDefinition, EquipmentItemInstance, EquipmentSlotType } from "@/types/equipment"

const KNOWN_SLOT_ORDER: string[] = [...PLAYER_SPECIALIZATION_BATTLE_SLOT_IDS, ...PLAYER_FISHING_SLOT_IDS]

const knownSlotSet = new Set<string>(KNOWN_SLOT_ORDER)

function splitSlots(raw?: string): string[] {
  if (!raw) return []
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

type EquippedLike = Pick<
  EquipmentItemInstance,
  "instanceId" | "templateItemId" | "enhanceLevel" | "durability" | "maxDurability" | "state" | "equippedSlot" | "equipType"
>

type CandidateLike = Pick<
  EquipmentItemInstance,
  "instanceId" | "templateItemId" | "enhanceLevel" | "state" | "archivedAt"
>

export type SlotCandidate = {
  instanceId: number
  templateItemId: string
  enhanceLevel: number
  archivedAt?: string | null
  item: EquipmentItemDefinition
  canEquip: boolean
  reasonKey?: string
  requirementAttributes: Record<string, number>
  unmetStatus: Array<{ attrKey: string; required: number; current: number }>
}

export type EquipmentSlotCard = {
  slot: string
  instance: EquippedLike | null
  occupiedSlots: string[]
  requirementAttributes: Record<string, number>
  candidates: SlotCandidate[]
}

type UseEquipmentSlotsOptions = {
  equippedItems?: Ref<EquippedLike[]>
  inventoryItems?: Ref<EquipmentItemInstance[]>
  transferableEquippedItems?: Ref<EquippedLike[]>
  currentAttributes?: Ref<Record<string, number>>
  visibleSlots?: Ref<string[]> | string[]
}

export function useEquipmentSlots(activeEquipType: Ref<EquipmentSlotType>, options: UseEquipmentSlotsOptions = {}) {
  const equipmentStore = useEquipmentStore()
  const inventoryStore = useInventoryStore()
  const roleStateStore = useRoleStateStore()
  const playerRole = computed(() => roleStateStore.ensureRole("player:self"))

  const currentAttributes = computed<Record<string, number>>(() => {
    if (options.currentAttributes) return options.currentAttributes.value
    return roleStateStore.roleUnitAttributes("player:self")
  })

  const sourceEquippedItems = computed<EquippedLike[]>(() => {
    if (options.equippedItems) return options.equippedItems.value
    if (equipmentStore.initialized) return equipmentStore.equippedItems
    if (playerRole.value.initialized.equipment) return playerRole.value.equipment
    return []
  })

  const sourceInventoryItems = computed<EquipmentItemInstance[]>(() => {
    if (options.inventoryItems) return options.inventoryItems.value
    return inventoryStore.instances
  })
  const sourceCandidateItems = computed<CandidateLike[]>(() => {
    const byID = new Map<number, CandidateLike>()
    for (const instance of sourceInventoryItems.value) {
      if (instance.state !== "inventory") continue
      byID.set(instance.instanceId, instance)
    }
    if (!options.transferableEquippedItems) return Array.from(byID.values())

    const equippedOnTarget = new Set(sourceEquippedItems.value.map((instance) => instance.instanceId))
    for (const instance of options.transferableEquippedItems.value) {
      if (instance.state !== "equipped" || equippedOnTarget.has(instance.instanceId)) continue
      byID.set(instance.instanceId, {
        instanceId: instance.instanceId,
        templateItemId: instance.templateItemId,
        enhanceLevel: instance.enhanceLevel,
        state: instance.state
      })
    }
    return Array.from(byID.values())
  })
  const visibleSlots = computed<string[]>(() => {
    const raw = Array.isArray(options.visibleSlots)
      ? options.visibleSlots
      : options.visibleSlots?.value
    if (!raw || raw.length === 0) {
      return activeEquipType.value === "work" ? [...PLAYER_WORK_SLOT_IDS] : [...PLAYER_BASE_BATTLE_SLOT_IDS]
    }
    const seen = new Set<string>()
    const normalized = raw
      .map((slot) => String(slot || "").trim())
      .filter((slot) => slot && knownSlotSet.has(slot) && !seen.has(slot) && (seen.add(slot), true))
    normalized.sort((left, right) => KNOWN_SLOT_ORDER.indexOf(left) - KNOWN_SLOT_ORDER.indexOf(right))
    return normalized.length > 0
      ? normalized
      : activeEquipType.value === "work"
        ? [...PLAYER_WORK_SLOT_IDS]
        : [...PLAYER_BASE_BATTLE_SLOT_IDS]
  })
  const visibleSlotSet = computed(() => new Set(visibleSlots.value))

  const equippedBySlot = computed(() => {
    const map = new Map<string, EquippedLike>()
    const targetType = activeEquipType.value
    for (const item of sourceEquippedItems.value) {
      const equipType = item.equipType || "battle"
      if (equipType !== targetType) continue
      for (const slot of splitSlots(item.equippedSlot)) {
        if (!map.has(slot)) {
          map.set(slot, item)
        }
      }
    }
    return map
  })

  const equippedAccessoryTemplateIds = computed(() => {
    const ids = new Set<string>()
    const targetType = activeEquipType.value
    for (const item of sourceEquippedItems.value) {
      const equipType = item.equipType || "battle"
      if (equipType !== targetType) continue
      const itemDef = getEquipmentItemDefinition(item.templateItemId)
      if (!itemDef || itemDef.slot !== "accessory") continue
      ids.add(item.templateItemId)
    }
    return ids
  })

  const slotCandidates = computed(() => {
    const slotMap = new Map<string, SlotCandidate[]>(visibleSlots.value.map((slot) => [slot, []]))
    const targetType = activeEquipType.value
    const requirementAttributes = { ...currentAttributes.value }
    for (const instance of sourceCandidateItems.value) {
      const item = getEquipmentItemDefinition(instance.templateItemId)
      if (!item) continue
      if (getEquipmentEquipType(instance.templateItemId) !== targetType) continue
      const allowedSlots = getEquipmentAllowedSlots(instance.templateItemId)
      if (allowedSlots.length === 0) continue
      const itemDef = getEquipmentItemDefinition(instance.templateItemId)
      const candidate: SlotCandidate = {
        instanceId: instance.instanceId,
        templateItemId: instance.templateItemId,
        enhanceLevel: instance.enhanceLevel || 0,
        archivedAt: instance.archivedAt || null,
        item,
        requirementAttributes,
        unmetStatus: evaluateStatusRequire(getEquipmentStatusRequirement(instance.templateItemId), requirementAttributes),
        canEquip: true
      }
      candidate.canEquip = candidate.unmetStatus.length === 0
      if (itemDef?.slot === "accessory" && equippedAccessoryTemplateIds.value.has(instance.templateItemId)) {
        candidate.canEquip = false
        candidate.reasonKey = "game.error.equipAccessoryConflict"
      } else if (!candidate.canEquip) {
        candidate.reasonKey = "game.error.statusRequirementNotMet"
      }
      for (const slot of allowedSlots) {
        if (!visibleSlotSet.value.has(slot)) continue
        const list = slotMap.get(slot)
        if (!list) continue
        list.push(candidate)
      }
    }
    for (const [, list] of slotMap) {
      list.sort((a, b) => (b.enhanceLevel - a.enhanceLevel) || (a.instanceId - b.instanceId))
    }
    return slotMap
  })

  const slotCards = computed<EquipmentSlotCard[]>(() =>
    visibleSlots.value.map((slot) => {
      const instance = equippedBySlot.value.get(slot) || null
      return {
        slot,
        instance,
        occupiedSlots: instance ? splitSlots(instance.equippedSlot) : [],
        requirementAttributes: { ...currentAttributes.value },
        candidates: slotCandidates.value.get(slot) || []
      }
    })
  )

  return { slotCards }
}
