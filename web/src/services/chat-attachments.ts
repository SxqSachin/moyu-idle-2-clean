import type {
  EquipmentAffixSlot,
  FishInstanceData,
  InventoryInstance,
  ItemCustomization
} from "@/types/equipment"

export type ArenaReplayChatAttachment = {
  type: "arena_replay"
  battleKind: "arena"
  token: string
  challengeId: number
  runId?: never
  createdBy: number
  createdByName: string
  outcome: string
}

export type CombatTrialReplayChatAttachment = {
  type: "combat_trial_replay"
  battleKind: "combat_trial"
  token: string
  runId: number
  challengeId?: never
  createdBy: number
  createdByName: string
  outcome: string
  outcomeLabelKey?: string
  metrics: Record<string, number>
}

export type ItemReferenceChatAttachment = {
  type: "item"
  itemId: string
}

export type ItemInstanceChatAttachmentPreview = {
  customData?: ItemCustomization
  fishData?: FishInstanceData
  enhanceLevel?: number
  baseSnapshot?: Record<string, unknown>
  affixes?: EquipmentAffixSlot[]
}

export type ItemInstanceChatAttachment = {
  type: "item_instance"
  itemId: string
  // Present only while the owner is composing. Public chat payloads omit database IDs.
  instanceId?: number
  preview: ItemInstanceChatAttachmentPreview
}

export type ActionReferenceChatAttachment = {
  type: "action"
  actionId: string
}

export type ChatAttachment =
  | ArenaReplayChatAttachment
  | CombatTrialReplayChatAttachment
  | ItemReferenceChatAttachment
  | ItemInstanceChatAttachment
  | ActionReferenceChatAttachment

export type ChatAttachmentRequest =
  | { type: "arena_replay" | "combat_trial_replay"; token: string }
  | { type: "item"; itemId: string }
  | { type: "item_instance"; instanceId: number }
  | { type: "action"; actionId: string }

function normalizeInteger(raw: unknown, fallback = 0): number {
  const value = typeof raw === "number" ? raw : Number(raw)
  if (!Number.isFinite(value)) return fallback
  return Math.trunc(value)
}

function normalizeFiniteNumber(raw: unknown, fallback = 0): number {
  const value = typeof raw === "number" ? raw : Number(raw)
  return Number.isFinite(value) ? value : fallback
}

function normalizeObject(raw: unknown): Record<string, unknown> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  return raw as Record<string, unknown>
}

function normalizeItemCustomization(raw: unknown): ItemCustomization | undefined {
  const data = normalizeObject(raw)
  if (!data) return undefined
  const name = String(data.name || "").trim()
  const labelColor = String(data.labelColor || "").trim()
  if (!name && !labelColor) return undefined
  return {
    ...(name ? { name } : {}),
    ...(labelColor ? { labelColor } : {})
  }
}

function normalizeItemInstancePreview(raw: unknown): ItemInstanceChatAttachmentPreview {
  const data = normalizeObject(raw) || {}
  const customData = normalizeItemCustomization(data.customData)
  const fishData = normalizeObject(data.fishData) as FishInstanceData | undefined
  const baseSnapshot = normalizeObject(data.baseSnapshot)
  const affixes = Array.isArray(data.affixes)
    ? data.affixes.filter((entry): entry is EquipmentAffixSlot => !!entry && typeof entry === "object" && !Array.isArray(entry))
    : undefined
  return {
    ...(customData ? { customData } : {}),
    ...(fishData ? { fishData } : {}),
    enhanceLevel: Math.max(0, normalizeInteger(data.enhanceLevel)),
    ...(baseSnapshot ? { baseSnapshot } : {}),
    ...(affixes ? { affixes } : {})
  }
}

export function normalizeChatAttachment(raw: unknown): ChatAttachment | undefined {
  const data = normalizeObject(raw)
  if (!data) return undefined
  const type = String(data.type || "").trim().toLowerCase()
  if (type === "item") {
    const itemId = String(data.itemId || "").trim()
    return itemId ? { type: "item", itemId } : undefined
  }
  if (type === "item_instance") {
    const itemId = String(data.itemId || "").trim()
    const instanceId = normalizeInteger(data.instanceId)
    if (!itemId) return undefined
    return {
      type: "item_instance",
      itemId,
      ...(instanceId > 0 ? { instanceId } : {}),
      preview: normalizeItemInstancePreview(data.preview)
    }
  }
  if (type === "action") {
    const actionId = String(data.actionId || "").trim()
    return actionId ? { type: "action", actionId } : undefined
  }

  const token = String(data.token || "").trim()
  if (!token) return undefined
  if (type === "arena_replay") {
    const challengeId = normalizeInteger(data.challengeId)
    if (challengeId <= 0) return undefined
    return {
      type: "arena_replay",
      battleKind: "arena",
      token,
      challengeId,
      createdBy: normalizeInteger(data.createdBy),
      createdByName: String(data.createdByName || "").trim(),
      outcome: String(data.outcome || "").trim()
    }
  }
  if (type !== "combat_trial_replay") return undefined
  const runId = normalizeInteger(data.runId)
  if (runId <= 0) return undefined
  const metrics: Record<string, number> = {}
  const rawMetrics = normalizeObject(data.metrics)
  if (rawMetrics) {
    for (const [metricId, rawValue] of Object.entries(rawMetrics)) {
      const value = normalizeFiniteNumber(rawValue, -1)
      if (metricId.trim() && value >= 0) metrics[metricId] = value
    }
  }
  return {
    type: "combat_trial_replay",
    battleKind: "combat_trial",
    token,
    runId,
    createdBy: normalizeInteger(data.createdBy),
    createdByName: String(data.createdByName || "").trim(),
    outcome: String(data.outcome || "").trim(),
    outcomeLabelKey: String(data.outcomeLabelKey || "").trim() || undefined,
    metrics
  }
}

export function createItemInstanceChatAttachment(instance: InventoryInstance): ItemInstanceChatAttachment {
  return {
    type: "item_instance",
    itemId: String(instance.templateItemId || "").trim(),
    instanceId: Math.max(0, Math.trunc(Number(instance.instanceId || 0))),
    preview: {
      ...(instance.customData ? { customData: { ...instance.customData } } : {}),
      ...(instance.fishData ? { fishData: { ...instance.fishData } } : {}),
      enhanceLevel: Math.max(0, Math.trunc(Number(instance.enhanceLevel || 0))),
      ...(instance.baseSnapshot ? { baseSnapshot: { ...instance.baseSnapshot } } : {}),
      ...(Array.isArray(instance.affixes) ? { affixes: instance.affixes.map((entry) => ({ ...entry })) } : {})
    }
  }
}

export function toChatAttachmentRequest(attachment: ChatAttachment): ChatAttachmentRequest {
  switch (attachment.type) {
    case "arena_replay":
    case "combat_trial_replay":
      return { type: attachment.type, token: attachment.token }
    case "item":
      return { type: "item", itemId: attachment.itemId }
    case "item_instance": {
      const instanceId = Math.trunc(Number(attachment.instanceId || 0))
      if (instanceId <= 0) throw new Error("item instance attachment is missing its private lookup id")
      return { type: "item_instance", instanceId }
    }
    case "action":
      return { type: "action", actionId: attachment.actionId }
  }
}

export function chatAttachmentIdentity(attachment: ChatAttachment | undefined): string {
  if (!attachment) return ""
  switch (attachment.type) {
    case "arena_replay":
    case "combat_trial_replay":
      return `${attachment.type}:${attachment.token}`
    case "item":
      return `${attachment.type}:${attachment.itemId}`
    case "item_instance":
      return `${attachment.type}:${attachment.instanceId || attachment.itemId}`
    case "action":
      return `${attachment.type}:${attachment.actionId}`
  }
}
