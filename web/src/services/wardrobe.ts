import { apiGet, apiPost } from "@services/api"

export type WardrobeOwnedItem = {
  category: string
  itemId: string
  source: string
  obtainedAt: string
}

export type WardrobeEquipEntry = {
  ownerType: string
  ownerId: number
  slotId: string
  itemId: string
}

export type WardrobeState = {
  owned: WardrobeOwnedItem[]
  equipped: WardrobeEquipEntry[]
}

export async function fetchWardrobe(token: string): Promise<WardrobeState> {
  return apiGet<WardrobeState>("/api/wardrobe", token)
}

export async function equipCosmetic(token: string, slotId: string, itemId: string) {
  return apiPost<{ ok: boolean }>("/api/wardrobe/equip", token, { slotId, itemId })
}

export async function unequipCosmetic(token: string, slotId: string) {
  return apiPost<{ ok: boolean }>("/api/wardrobe/unequip", token, { slotId })
}

export async function equipKittyCosmetic(token: string, kittyId: number, slotId: string, itemId: string) {
  return apiPost<{ ok: boolean; kittyId: number }>(
    `/api/cattery/kitties/${kittyId}/wardrobe/equip`,
    token,
    { slotId, itemId }
  )
}

export async function unequipKittyCosmetic(token: string, kittyId: number, slotId: string) {
  return apiPost<{ ok: boolean; kittyId: number }>(
    `/api/cattery/kitties/${kittyId}/wardrobe/unequip`,
    token,
    { slotId }
  )
}
