import type { FishInstanceData } from "@/types/equipment"

export type InventoryGridItem = {
  instanceId: number
  templateItemId: string
  enhanceLevel: number
  quantity?: number
  fishData?: FishInstanceData
}

export type LootCacheEntry = {
  id: number
  templateItemId: string
  quantity: number
  expiresAt: string
}
