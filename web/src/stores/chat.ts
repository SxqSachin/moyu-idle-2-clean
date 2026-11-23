import { defineStore } from "pinia"
import { apiGet } from "@services/api"
import {
  chatAttachmentIdentity,
  normalizeChatAttachment,
  type ChatAttachment
} from "@services/chat-attachments"

export type {
  ActionReferenceChatAttachment,
  ArenaReplayChatAttachment,
  ChatAttachment,
  CombatTrialReplayChatAttachment,
  ItemInstanceChatAttachment,
  ItemReferenceChatAttachment
} from "@services/chat-attachments"

export type ChatChannelConfig = {
  id: string
  nameKey: string
  label?: string
  order: number
  playerSendable: boolean
}

export type ChatMessageDisplayMeta = {
  avatarId?: string
  avatarFrameId?: string
  [key: string]: unknown
}

export type ChatMessage = {
  id: number
  channel: string
  userId: number
  username: string
  titlePrefixId?: string
  titleSuffixId?: string
  displayMeta?: ChatMessageDisplayMeta
  attachment?: ChatAttachment
  content: string
  ts: number
  sentAt?: string
}

type ChatConfigPayload = {
  maxContentLength?: number
  defaultHistoryLimit?: number
  defaultChannel?: string
  channels?: ChatChannelConfig[]
}

type ChatMessagesPayload = {
  channel?: string
  items?: ChatMessage[]
  nextBeforeId?: number
  hasMore?: boolean
}

const FALLBACK_CHANNEL: ChatChannelConfig = {
  id: "world",
  nameKey: "chat.channel.world",
  label: "",
  order: 10,
  playerSendable: true
}
const BATTLE_ROOM_CHAT_CHANNEL_PREFIX = "battle_room:"
const BATTLE_ROOM_CHAT_CHANNEL_ORDER = 1000

function normalizeChannelID(raw: string | undefined | null): string {
  return String(raw || "").trim().toLowerCase()
}

function buildBattleRoomChannelID(roomID: number): string {
  const normalized = normalizeInteger(roomID, 0)
  if (normalized <= 0) return ""
  return `${BATTLE_ROOM_CHAT_CHANNEL_PREFIX}${normalized}`
}

function parseBattleRoomChannelID(channelID: string): number {
  const normalized = normalizeChannelID(channelID)
  if (!normalized.startsWith(BATTLE_ROOM_CHAT_CHANNEL_PREFIX)) return 0
  const roomRaw = normalized.slice(BATTLE_ROOM_CHAT_CHANNEL_PREFIX.length).trim()
  if (!roomRaw) return 0
  const roomID = normalizeInteger(roomRaw, 0)
  if (roomID <= 0) return 0
  return roomID
}

function isBattleRoomChannel(channelID: string): boolean {
  return parseBattleRoomChannelID(channelID) > 0
}

function normalizeInteger(raw: unknown, fallback: number): number {
  const value = typeof raw === "number" ? raw : Number(raw)
  if (!Number.isFinite(value)) return fallback
  return Math.trunc(value)
}

function normalizeChatMessage(raw: Partial<ChatMessage> | null | undefined, fallbackChannel: string): ChatMessage | null {
  if (!raw || typeof raw !== "object") return null
  const content = String(raw.content || "").trim()
  const attachment = normalizeChatAttachment(raw.attachment)
  if (!content && !attachment) return null
  const channel = normalizeChannelID(raw.channel) || fallbackChannel
  const userId = normalizeInteger(raw.userId, 0)
  const username = String(raw.username || "").trim() || (userId > 0 ? `#${userId}` : "-")
  const id = normalizeInteger(raw.id, 0)
  const ts = normalizeInteger(
    raw.ts,
    typeof raw.sentAt === "string" && raw.sentAt ? Date.parse(raw.sentAt) : Date.now()
  )
  return {
    id,
    channel,
    userId,
    username,
    titlePrefixId: typeof raw.titlePrefixId === "string" ? raw.titlePrefixId.trim() || undefined : undefined,
    titleSuffixId: typeof raw.titleSuffixId === "string" ? raw.titleSuffixId.trim() || undefined : undefined,
    displayMeta: typeof raw.displayMeta === "object" && raw.displayMeta ? (raw.displayMeta as ChatMessageDisplayMeta) : undefined,
    attachment,
    content,
    ts,
    sentAt: typeof raw.sentAt === "string" ? raw.sentAt : undefined
  }
}

function messageSortValue(message: ChatMessage): number {
  if (message.id > 0) return message.id
  return message.ts
}

function insertMessage(list: ChatMessage[], message: ChatMessage): boolean {
  if (message.id > 0 && list.some((item) => item.id === message.id)) return false
  if (
    message.id <= 0 &&
    list.some(
      (item) =>
        item.channel === message.channel &&
        item.userId === message.userId &&
        item.content === message.content &&
        chatAttachmentIdentity(item.attachment) === chatAttachmentIdentity(message.attachment) &&
        Math.abs(item.ts - message.ts) <= 5
    )
  ) {
    return false
  }
  list.push(message)
  list.sort((a, b) => messageSortValue(a) - messageSortValue(b))
  if (list.length > 200) {
    list.splice(0, list.length - 200)
  }
  return true
}

export const useChatStore = defineStore("chat", {
  state: () => ({
    initialized: false,
    panelOpen: false,
    maxContentLength: 300,
    defaultHistoryLimit: 30,
    defaultChannel: FALLBACK_CHANNEL.id,
    activeChannel: FALLBACK_CHANNEL.id,
    channels: [FALLBACK_CHANNEL] as ChatChannelConfig[],
    messagesByChannel: {} as Record<string, ChatMessage[]>,
    historyLoadedByChannel: {} as Record<string, boolean>,
    hasMoreByChannel: {} as Record<string, boolean>,
    nextBeforeByChannel: {} as Record<string, number>,
    unreadByChannel: {} as Record<string, number>,
    pendingAttachment: null as ChatAttachment | null
  }),
  getters: {
    totalUnread(state) {
      return Object.values(state.unreadByChannel).reduce((sum, value) => sum + Number(value || 0), 0)
    },
    activeMessages(state): ChatMessage[] {
      return state.messagesByChannel[state.activeChannel] || []
    }
  },
  actions: {
    reset() {
      this.initialized = false
      this.panelOpen = false
      this.maxContentLength = 300
      this.defaultHistoryLimit = 30
      this.defaultChannel = FALLBACK_CHANNEL.id
      this.activeChannel = FALLBACK_CHANNEL.id
      this.channels = [FALLBACK_CHANNEL]
      this.messagesByChannel = {}
      this.historyLoadedByChannel = {}
      this.hasMoreByChannel = {}
      this.nextBeforeByChannel = {}
      this.unreadByChannel = {}
      this.pendingAttachment = null
    },
    ensureChannelState(channelID: string) {
      if (!this.messagesByChannel[channelID]) this.messagesByChannel[channelID] = []
      if (this.historyLoadedByChannel[channelID] === undefined) this.historyLoadedByChannel[channelID] = false
      if (this.hasMoreByChannel[channelID] === undefined) this.hasMoreByChannel[channelID] = false
      if (this.nextBeforeByChannel[channelID] === undefined) this.nextBeforeByChannel[channelID] = 0
      if (this.unreadByChannel[channelID] === undefined) this.unreadByChannel[channelID] = 0
    },
    applyConfig(payload: ChatConfigPayload) {
      const channels = Array.isArray(payload.channels)
        ? payload.channels
            .map((item) => ({
              id: normalizeChannelID(item.id) || FALLBACK_CHANNEL.id,
              nameKey: String(item.nameKey || "").trim() || `chat.channel.${normalizeChannelID(item.id)}`,
              label: String(item.label || "").trim(),
              order: normalizeInteger(item.order, 0),
              playerSendable: Boolean(item.playerSendable)
            }))
            .filter((item) => item.id)
        : []
      const uniqueChannels: ChatChannelConfig[] = []
      const seen = new Set<string>()
      for (const item of channels.sort((a, b) => a.order - b.order || a.id.localeCompare(b.id))) {
        if (seen.has(item.id)) continue
        seen.add(item.id)
        uniqueChannels.push(item)
      }
      if (uniqueChannels.length === 0) {
        uniqueChannels.push(FALLBACK_CHANNEL)
      }
      this.channels = uniqueChannels
      this.maxContentLength = Math.max(1, Math.min(2000, normalizeInteger(payload.maxContentLength, 300)))
      this.defaultHistoryLimit = Math.max(1, Math.min(200, normalizeInteger(payload.defaultHistoryLimit, 30)))
      this.defaultChannel = normalizeChannelID(payload.defaultChannel) || uniqueChannels[0].id
      if (!uniqueChannels.some((item) => item.id === this.defaultChannel)) {
        this.defaultChannel = uniqueChannels[0].id
      }
      if (!uniqueChannels.some((item) => item.id === this.activeChannel)) {
        this.activeChannel = this.defaultChannel
      }
      for (const channel of uniqueChannels) {
        this.ensureChannelState(channel.id)
      }
    },
    async initialize(token: string, force = false) {
      if (!token) {
        this.reset()
        return
      }
      if (this.initialized && !force) return
      const config = await apiGet<ChatConfigPayload>("/api/chat/config", token)
      this.applyConfig(config || {})
      this.initialized = true
      await this.loadChannelHistory(token, this.activeChannel, true)
    },
    applyBootstrapSnapshot(configPayload: ChatConfigPayload, historyPayload: ChatMessagesPayload) {
      this.applyConfig(configPayload || {})
      const normalizedChannel = normalizeChannelID(historyPayload.channel) || this.defaultChannel
      this.ensureChannelState(normalizedChannel)
      const items = Array.isArray(historyPayload.items) ? historyPayload.items : []
      const nextList: ChatMessage[] = []
      for (const item of items) {
        const normalized = normalizeChatMessage(item, normalizedChannel)
        if (!normalized) continue
        insertMessage(nextList, normalized)
      }
      this.messagesByChannel[normalizedChannel] = nextList
      this.historyLoadedByChannel[normalizedChannel] = true
      this.nextBeforeByChannel[normalizedChannel] = normalizeInteger(historyPayload.nextBeforeId, 0)
      this.hasMoreByChannel[normalizedChannel] = Boolean(historyPayload.hasMore)
      this.activeChannel = normalizedChannel
      this.initialized = true
    },
    async loadChannelHistory(token: string, channelID: string, force = false) {
      const normalizedChannel = normalizeChannelID(channelID) || this.defaultChannel
      this.ensureChannelState(normalizedChannel)
      if (this.historyLoadedByChannel[normalizedChannel] && !force) return
      const data = await apiGet<ChatMessagesPayload>(
        `/api/chat/messages?channel=${encodeURIComponent(normalizedChannel)}&limit=${this.defaultHistoryLimit}`,
        token
      )
      const fallbackChannel = normalizeChannelID(data.channel) || normalizedChannel
      const items = Array.isArray(data.items) ? data.items : []
      const nextList: ChatMessage[] = []
      for (const item of items) {
        const normalized = normalizeChatMessage(item, fallbackChannel)
        if (!normalized) continue
        insertMessage(nextList, normalized)
      }
      this.messagesByChannel[fallbackChannel] = nextList
      this.historyLoadedByChannel[fallbackChannel] = true
      this.nextBeforeByChannel[fallbackChannel] = normalizeInteger(data.nextBeforeId, 0)
      this.hasMoreByChannel[fallbackChannel] = Boolean(data.hasMore)
      if (fallbackChannel !== normalizedChannel) {
        this.messagesByChannel[normalizedChannel] = nextList
        this.historyLoadedByChannel[normalizedChannel] = true
        this.nextBeforeByChannel[normalizedChannel] = this.nextBeforeByChannel[fallbackChannel]
        this.hasMoreByChannel[normalizedChannel] = this.hasMoreByChannel[fallbackChannel]
      }
    },
    async loadMoreHistory(token: string, channelID: string) {
      const normalizedChannel = normalizeChannelID(channelID) || this.defaultChannel
      const beforeID = this.nextBeforeByChannel[normalizedChannel] || 0
      if (beforeID <= 0) return
      const data = await apiGet<ChatMessagesPayload>(
        `/api/chat/messages?channel=${encodeURIComponent(normalizedChannel)}&limit=${this.defaultHistoryLimit}&beforeId=${beforeID}`,
        token
      )
      const items = Array.isArray(data.items) ? data.items : []
      const target = this.messagesByChannel[normalizedChannel] || []
      const normalizedList: ChatMessage[] = [...target]
      for (const item of items) {
        const normalized = normalizeChatMessage(item, normalizedChannel)
        if (!normalized) continue
        insertMessage(normalizedList, normalized)
      }
      this.messagesByChannel[normalizedChannel] = normalizedList
      this.nextBeforeByChannel[normalizedChannel] = normalizeInteger(data.nextBeforeId, 0)
      this.hasMoreByChannel[normalizedChannel] = Boolean(data.hasMore)
    },
    setPanelOpen(open: boolean) {
      this.panelOpen = open
      if (open) {
        this.clearUnread(this.activeChannel)
      }
    },
    togglePanel() {
      this.setPanelOpen(!this.panelOpen)
    },
    setActiveChannel(channelID: string) {
      const normalized = normalizeChannelID(channelID)
      if (!normalized || !this.channels.some((item) => item.id === normalized)) return
      this.activeChannel = normalized
      this.clearUnread(normalized)
    },
    clearUnread(channelID: string) {
      const normalized = normalizeChannelID(channelID)
      if (!normalized) return
      this.ensureChannelState(normalized)
      this.unreadByChannel[normalized] = 0
    },
    canSendToChannel(channelID: string): boolean {
      const normalized = normalizeChannelID(channelID)
      return this.channels.some((item) => item.id === normalized && item.playerSendable)
    },
    setPendingAttachment(attachment: ChatAttachment | null) {
      this.pendingAttachment = attachment ? { ...attachment } : null
    },
    clearPendingAttachment() {
      this.pendingAttachment = null
    },
    syncBattleRoomChannel(roomID: number, roomName: string) {
      const channelID = buildBattleRoomChannelID(roomID)
      if (!channelID) return
      const label = String(roomName || "").trim() || `#${roomID}`
      const nextChannels = this.channels.filter((channel) => {
        const existingRoomID = parseBattleRoomChannelID(channel.id)
        return existingRoomID <= 0 || existingRoomID === roomID
      })
      const idx = nextChannels.findIndex((channel) => channel.id === channelID)
      const roomChannel: ChatChannelConfig = {
        id: channelID,
        nameKey: "",
        label,
        order: BATTLE_ROOM_CHAT_CHANNEL_ORDER,
        playerSendable: true
      }
      if (idx >= 0) {
        nextChannels[idx] = roomChannel
      } else {
        nextChannels.push(roomChannel)
      }
      nextChannels.sort((a, b) => a.order - b.order || a.id.localeCompare(b.id))
      this.channels = nextChannels
      this.ensureChannelState(channelID)
    },
    clearBattleRoomChannels() {
      const removedChannelIDs = this.channels.filter((channel) => isBattleRoomChannel(channel.id)).map((channel) => channel.id)
      if (removedChannelIDs.length <= 0) return
      this.channels = this.channels.filter((channel) => !isBattleRoomChannel(channel.id))
      for (const channelID of removedChannelIDs) {
        delete this.messagesByChannel[channelID]
        delete this.historyLoadedByChannel[channelID]
        delete this.hasMoreByChannel[channelID]
        delete this.nextBeforeByChannel[channelID]
        delete this.unreadByChannel[channelID]
      }
      if (!this.channels.some((channel) => channel.id === this.activeChannel)) {
        this.activeChannel = this.defaultChannel
      }
    },
    appendIncomingMessage(payload: Partial<ChatMessage> | null | undefined) {
      const normalized = normalizeChatMessage(payload, this.defaultChannel)
      if (!normalized) return
      this.ensureChannelState(normalized.channel)
      const list = this.messagesByChannel[normalized.channel]
      const inserted = insertMessage(list, normalized)
      if (!inserted) return
      if (this.panelOpen && this.activeChannel === normalized.channel) {
        return
      }
      this.unreadByChannel[normalized.channel] = (this.unreadByChannel[normalized.channel] || 0) + 1
    }
  }
})
