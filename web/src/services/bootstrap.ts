import { apiGet } from "@services/api"
import type { ChatAttachment } from "@services/chat-attachments"
import type { PlayerSettingsPayload } from "@services/player-settings"
import type { TutorialProgressMap } from "@services/player-tutorial"
import type { EquippedItem } from "@stores/equipment"
import type { InventoryInstance } from "@/types/equipment"
import type { OperationEffectProjection } from "@services/operation-effects"

type ChatChannelConfig = {
  id: string
  nameKey: string
  order: number
  playerSendable: boolean
}

type ChatMessage = {
  id: number
  channel: string
  userId: number
  username: string
  titlePrefixId?: string
  titleSuffixId?: string
  displayMeta?: Record<string, unknown>
  attachment?: ChatAttachment
  content: string
  ts: number
  sentAt?: string
}

export type BootstrapInitPayload = {
  playerStats: Record<string, unknown>
  inventory: {
    items?: InventoryInstance[]
  }
  equipment?: {
    items?: EquippedItem[]
  }
  chatConfig: {
    maxContentLength?: number
    defaultHistoryLimit?: number
    defaultChannel?: string
    channels?: ChatChannelConfig[]
  }
  chatHistory: {
    channel?: string
    items?: ChatMessage[]
    nextBeforeId?: number
    hasMore?: boolean
  }
  playerSettings: PlayerSettingsPayload
  tutorialProgress?: TutorialProgressMap
  operationEffects: OperationEffectProjection
  playerPresence: {
    status?: "online" | "offline"
  }
  systemVersion: {
    releaseId?: string
    kind?: "soft" | "force" | string
    messageI18nKey?: string
    recommendedDelaySec?: number
    forceAtMs?: number
    minAcceptedBuild?: string
    announcedAtMs?: number
  }
  recycleConfig?: {
    goldCoefficient?: number
    affixMismatchExtraCoefficient?: number
  }
}

export async function fetchBootstrapInit(token: string) {
  return apiGet<BootstrapInitPayload>("/api/bootstrap/init", token)
}
