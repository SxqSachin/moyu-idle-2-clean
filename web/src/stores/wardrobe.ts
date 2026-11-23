import { defineStore } from "pinia"
import { ref, computed } from "vue"
import {
  fetchWardrobe,
  equipCosmetic,
  unequipCosmetic,
  equipKittyCosmetic,
  unequipKittyCosmetic,
  type WardrobeOwnedItem,
  type WardrobeEquipEntry
} from "@services/wardrobe"

export type { WardrobeOwnedItem, WardrobeEquipEntry }

export const useWardrobeStore = defineStore("wardrobe", () => {
  const owned = ref<WardrobeOwnedItem[]>([])
  const equipped = ref<WardrobeEquipEntry[]>([])
  const loaded = ref(false)

  const ownedByCategory = computed(() => {
    const map: Record<string, WardrobeOwnedItem[]> = {}
    for (const item of owned.value) {
      if (!map[item.category]) map[item.category] = []
      map[item.category].push(item)
    }
    return map
  })

  const equippedMap = computed(() => {
    const map: Record<string, WardrobeEquipEntry> = {}
    for (const entry of equipped.value) {
      map[entry.slotId] = entry
    }
    return map
  })

  const playerTitlePrefix = computed(() => equippedMap.value["title_prefix"]?.itemId ?? "")
  const playerTitleSuffix = computed(() => equippedMap.value["title_suffix"]?.itemId ?? "")

  async function load(token: string) {
    const data = await fetchWardrobe(token)
    owned.value = data.owned ?? []
    equipped.value = data.equipped ?? []
    loaded.value = true
  }

  async function equip(token: string, slotId: string, itemId: string) {
    await equipCosmetic(token, slotId, itemId)
    await load(token)
  }

  async function unequip(token: string, slotId: string) {
    await unequipCosmetic(token, slotId)
    await load(token)
  }

  async function equipKitty(token: string, kittyId: number, slotId: string, itemId: string) {
    await equipKittyCosmetic(token, kittyId, slotId, itemId)
    await load(token)
  }

  async function unequipKitty(token: string, kittyId: number, slotId: string) {
    await unequipKittyCosmetic(token, kittyId, slotId)
    await load(token)
  }

  function applyPatch(state: { owned?: WardrobeOwnedItem[]; equipped?: WardrobeEquipEntry[] }) {
    if (Array.isArray(state.owned)) owned.value = state.owned
    if (Array.isArray(state.equipped)) equipped.value = state.equipped
    loaded.value = true
  }

  function reset() {
    owned.value = []
    equipped.value = []
    loaded.value = false
  }

  return {
    owned,
    equipped,
    loaded,
    ownedByCategory,
    equippedMap,
    playerTitlePrefix,
    playerTitleSuffix,
    load,
    equip,
    unequip,
    equipKitty,
    unequipKitty,
    applyPatch,
    reset
  }
})
