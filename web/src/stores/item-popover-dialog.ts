import { defineStore } from "pinia"
import type { FishInstanceData, ItemCustomization } from "@/types/equipment"

export type ItemPopoverDialogPreviewAffix = {
  slotIndex: number
  affixId: string
  rarityId: string
  rollValue?: number
  valuePrecision?: number
  attrMods?: Record<string, number>
  effectType?: string
  effectParams?: Record<string, unknown>
}

export type ItemPopoverDialogPublicInstancePreview = {
  customData?: ItemCustomization
  fishData?: FishInstanceData
  saleEligible?: boolean
  saleIneligibleReason?: "cultured_immature" | "dead" | "invalid_data"
  saleCurrentAgeDays?: number
  saleRequiredAgeDays?: number
  enhanceLevel?: number
  durability?: number
  maxDurability?: number
  baseSnapshot?: Record<string, unknown>
  affixes?: ItemPopoverDialogPreviewAffix[]
}

type ItemPopoverDialogPayload = {
  itemId: string
  instanceId?: number | null
  publicInstancePreview?: ItemPopoverDialogPublicInstancePreview | null
  requirementAttributes?: Record<string, number> | null
}

export const useItemPopoverDialogStore = defineStore("itemPopoverDialog", {
  state: () => ({
    visible: false,
    itemId: "",
    instanceId: null as number | null,
    publicInstancePreview: null as ItemPopoverDialogPublicInstancePreview | null,
    requirementAttributes: null as Record<string, number> | null
  }),
  actions: {
    open(payload: ItemPopoverDialogPayload) {
      const itemId = String(payload.itemId || "").trim()
      if (!itemId) return
      this.itemId = itemId
      this.instanceId = payload.instanceId ?? null
      this.publicInstancePreview = payload.publicInstancePreview ?? null
      this.requirementAttributes = payload.requirementAttributes
        ? { ...payload.requirementAttributes }
        : null
      this.visible = true
    },
    close() {
      this.visible = false
      this.itemId = ""
      this.instanceId = null
      this.publicInstancePreview = null
      this.requirementAttributes = null
    }
  }
})
