import { adminGet, adminPost } from "./client"
import { getAdminBridge } from "./bridge"

export type TitleEntry = {
  id: string
  nameKey: string
  rarity: string
  order: number
  disabled: boolean
}

export async function loadTitlePrefixes(): Promise<TitleEntry[]> {
  return getAdminBridge().readGameData("title_prefixes") as Promise<TitleEntry[]>
}

export async function loadTitleSuffixes(): Promise<TitleEntry[]> {
  return getAdminBridge().readGameData("title_suffixes") as Promise<TitleEntry[]>
}

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

export type WardrobeListResponse = {
  userId: number
  owned: WardrobeOwnedItem[]
  equipped: WardrobeEquipEntry[]
}

export function listWardrobe(userId: number) {
  return adminGet<WardrobeListResponse>("wardrobe", "/list", { userId })
}

export function grantCosmetic(payload: { userId: number; category: string; itemId: string; source: string }) {
  return adminPost<{ ok: boolean; alreadyOwned: boolean }>("wardrobe", "/grant", payload)
}

export function batchGrantCosmetics(payload: { userId: number; items: Array<{ category: string; itemId: string }>; source: string }) {
  return adminPost<{ ok: boolean; grantedCount: number; skippedCount: number; granted: WardrobeOwnedItem[] }>(
    "wardrobe", "/batch-grant", payload
  )
}

export function revokeCosmetic(payload: { userId: number; category: string; itemId: string }) {
  return adminPost<{ ok: boolean }>("wardrobe", "/revoke", payload)
}
