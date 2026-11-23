import { defineStore } from "pinia"
import { apiGet } from "@services/api"
import { mapAttributeStateMeta } from "@/services/attribute-state-meta"
import { KITTY_BATTLE_EQUIP_SLOTS } from "@/config/shared-global-config"
import { PLAYER_FISHING_SLOT_IDS } from "@services/specialization"

export type RoleType = "player" | "kitty"
export type RoleID = "self" | number
export type RoleKey = "player:self" | `kitty:${number}`
export type AttributeScene = "battle" | "work" | "other"
export type EquipmentType = "battle" | "work"

export type AttributeNode = {
  value: number
  baseValue?: number
  skillPointLevel?: number
  allowBattlePointLevelUp?: boolean
  unit: "number" | "percent"
  category: AttributeScene
  growthType: "levelable" | "static"
  order: number
  nameKey?: string
  descKey?: string
  iconId?: string
  emojiIcon?: string
  sources?: unknown
  [key: string]: unknown
}

export type AttributeProgressNode = {
  value: number
  level: number
  exp: number
  nextExp: number
  [key: string]: unknown
}

export type RoleCapabilities = {
  attributeScenes: AttributeScene[]
  equipmentTypes: EquipmentType[]
  visibleSlots: string[]
  battlePoints: boolean
  skills: boolean
  battleItemBag: boolean
  loadouts: boolean
  sets: boolean
}

export type BattlePointState = {
  totalPoints: number
  spentPoints: number
  available: number
  resetRequirement: {
    itemId: string
    count: number
  }
}

export type RoleEquipmentItem = {
  instanceId: number
  templateItemId: string
  equipType: EquipmentType
  enhanceLevel: number
  durability: number
  maxDurability: number
  state: string
  equippedSlot: string
  updatedAt: string
}

type RoleState = {
  roleKey: RoleKey
  roleType: RoleType
  roleId: RoleID
  attributes: Record<string, AttributeNode>
  attributeProgress: Record<string, AttributeProgressNode>
  equipment: RoleEquipmentItem[]
  battlePoints: BattlePointState | null
  capabilities: RoleCapabilities
  versions: {
    attributes: number
    equipment: number
  }
  initialized: {
    profile: boolean
    attributes: boolean
    equipment: boolean
  }
}

type RoleAttributePatchPayload = {
  roleType?: RoleType | string
  roleId?: RoleID | number | string
  kittyId?: number
  upsert?: Record<string, unknown>
  remove?: string[]
  attributeProgressUpsert?: Record<string, unknown>
  attributeProgressRemove?: string[]
  meta?: { version?: number }
}

type RoleEquipmentPatchPayload = {
  roleType?: RoleType | string
  roleId?: RoleID | number | string
  kittyId?: number
  items?: RoleEquipmentItem[]
  upsert?: RoleEquipmentItem[]
  remove?: number[]
  meta?: { version?: number }
}

type RoleMutationPatchPayload = {
  roleAttributePatch?: RoleAttributePatchPayload | Record<string, unknown>
  roleEquipmentPatch?: RoleEquipmentPatchPayload | Record<string, unknown>
}

type PlayerSnapshot = {
  attributes?: Record<string, unknown>
  attributeProgress?: Record<string, unknown>
}

type KittyCombatProfile = {
  attributes?: Array<Record<string, unknown>>
  equipment?: Array<Record<string, unknown>>
  capabilities?: Partial<RoleCapabilities>
}

function roleKeyFor(roleType: RoleType | string | undefined, roleId: RoleID | number | string | undefined, kittyId?: number): RoleKey {
  const type = String(roleType || (kittyId ? "kitty" : "player")).trim().toLowerCase()
  if (type === "kitty") {
    const id = Number(roleId === "self" || roleId === undefined ? kittyId : roleId)
    return `kitty:${Math.max(0, Math.trunc(Number.isFinite(id) ? id : 0))}` as RoleKey
  }
  return "player:self"
}

function parseRoleKey(roleKey: RoleKey): { roleType: RoleType; roleId: RoleID } {
  if (roleKey.startsWith("kitty:")) {
    return { roleType: "kitty", roleId: Math.max(0, Math.trunc(Number(roleKey.slice("kitty:".length) || 0))) }
  }
  return { roleType: "player", roleId: "self" }
}

function defaultCapabilities(roleType: RoleType): RoleCapabilities {
  if (roleType === "kitty") {
    return {
      attributeScenes: ["battle", "work"],
      equipmentTypes: ["battle", "work"],
      visibleSlots: [...KITTY_BATTLE_EQUIP_SLOTS, ...PLAYER_FISHING_SLOT_IDS],
      battlePoints: true,
      skills: true,
      battleItemBag: false,
      loadouts: false,
      sets: false
    }
  }
  return {
    attributeScenes: ["battle", "work", "other"],
    equipmentTypes: ["battle", "work"],
    visibleSlots: [],
    battlePoints: true,
    skills: true,
    battleItemBag: true,
    loadouts: true,
    sets: true
  }
}

function defaultBattlePoints(): BattlePointState {
  return {
    totalPoints: 0,
    spentPoints: 0,
    available: 0,
    resetRequirement: { itemId: "", count: 1 }
  }
}

function makeDefaultAttribute(): AttributeNode {
  return { value: 0, unit: "number", category: "other", growthType: "static", order: 0 }
}

function makeDefaultProgress(): AttributeProgressNode {
  return { value: 0, level: 1, exp: 0, nextExp: 100 }
}

function normalizeAttributeNode(raw: unknown, existing?: AttributeNode): AttributeNode | null {
  if (!raw || typeof raw !== "object") return null
  const node = raw as Record<string, unknown>
  const next: AttributeNode = { ...(existing || makeDefaultAttribute()) }
  for (const [key, value] of Object.entries(node)) {
    next[key] = value
  }
  for (const key of ["value", "baseValue", "skillPointLevel", "order"] as const) {
    const value = Number(node[key])
    if (Number.isFinite(value)) {
      ;(next as Record<string, number>)[key] = value
    }
  }
  if (node.unit === "number" || node.unit === "percent") next.unit = node.unit
  if (node.category === "battle" || node.category === "work" || node.category === "other") next.category = node.category
  if (node.growthType === "levelable" || node.growthType === "static") next.growthType = node.growthType
  if (typeof node.allowBattlePointLevelUp === "boolean") next.allowBattlePointLevelUp = node.allowBattlePointLevelUp
  return next
}

function normalizeProgressNode(raw: unknown, existing?: AttributeProgressNode): AttributeProgressNode | null {
  if (!raw || typeof raw !== "object") return null
  const node = raw as Record<string, unknown>
  const next: AttributeProgressNode = { ...(existing || makeDefaultProgress()) }
  for (const [key, value] of Object.entries(node)) {
    next[key] = value
  }
  for (const key of ["value", "level", "exp", "nextExp"] as const) {
    const value = Number(node[key])
    if (Number.isFinite(value)) {
      ;(next as Record<string, number>)[key] = value
    }
  }
  return next
}

function normalizeEquipmentItem(raw: unknown): RoleEquipmentItem | null {
  if (!raw || typeof raw !== "object") return null
  const node = raw as Record<string, unknown>
  const instanceId = Number(node.instanceId || 0)
  if (!Number.isFinite(instanceId) || instanceId <= 0) return null
  const equipType = node.equipType === "work" ? "work" : "battle"
  return {
    instanceId,
    templateItemId: String(node.templateItemId || ""),
    equipType,
    enhanceLevel: Number(node.enhanceLevel || 0),
    durability: Number(node.durability || 0),
    maxDurability: Number(node.maxDurability || 0),
    state: String(node.state || "equipped"),
    equippedSlot: String(node.equippedSlot || ""),
    updatedAt: String(node.updatedAt || "")
  }
}

function normalizeBattlePointState(raw: Partial<BattlePointState> | null | undefined): BattlePointState {
  return {
    totalPoints: Math.max(0, Math.trunc(Number(raw?.totalPoints || 0))),
    spentPoints: Math.max(0, Math.trunc(Number(raw?.spentPoints || 0))),
    available: Math.max(0, Math.trunc(Number(raw?.available || 0))),
    resetRequirement: {
      itemId: String(raw?.resetRequirement?.itemId || "").trim(),
      count: Math.max(1, Math.trunc(Number(raw?.resetRequirement?.count || 1)))
    }
  }
}

export const useRoleStateStore = defineStore("role-state", {
  state: () => ({
    roles: {} as Record<RoleKey, RoleState>
  }),
  actions: {
    ensureRole(roleKey: RoleKey): RoleState {
      const parsed = parseRoleKey(roleKey)
      if (!this.roles[roleKey]) {
        this.roles[roleKey] = {
          roleKey,
          roleType: parsed.roleType,
          roleId: parsed.roleId,
          attributes: {},
          attributeProgress: {},
          equipment: [],
          battlePoints: parsed.roleType === "kitty" || parsed.roleType === "player" ? defaultBattlePoints() : null,
          capabilities: defaultCapabilities(parsed.roleType),
          versions: { attributes: 0, equipment: 0 },
          initialized: { profile: false, attributes: false, equipment: false }
        }
      }
      return this.roles[roleKey]
    },
    initializePlayerFromSnapshot(payload: PlayerSnapshot | null | undefined) {
      const role = this.ensureRole("player:self")
      this.applyAttributeSnapshot(role.roleKey, payload?.attributes || {}, payload?.attributeProgress || {})
      role.initialized.profile = true
    },
    async ensureRoleInitialized(roleKey: RoleKey, token: string) {
      const role = this.ensureRole(roleKey)
      if (!token || role.initialized.profile) return
      if (role.roleType === "player") return
      const kittyID = Number(role.roleId || 0)
      if (!Number.isFinite(kittyID) || kittyID <= 0) return
      const profile = await apiGet<KittyCombatProfile>(`/api/cattery/kitties/${encodeURIComponent(String(kittyID))}/combat-profile`, token)
      this.initializeKittyFromProfile(kittyID, profile)
    },
    initializeKittyFromProfile(kittyID: number, profile: KittyCombatProfile | null | undefined) {
      if (!Number.isFinite(kittyID) || kittyID <= 0) return
      const roleKey = `kitty:${Math.trunc(kittyID)}` as RoleKey
      const role = this.ensureRole(roleKey)
      const attributes: Record<string, unknown> = {}
      const progress: Record<string, unknown> = {}
      for (const row of profile?.attributes || []) {
        const key = String(row.attrKey || "").trim()
        if (!key) continue
        const meta = mapAttributeStateMeta(row)
        attributes[key] = meta
        if (meta.growthType === "levelable") {
          progress[key] = {
            value: Number(row.finalValue || 0),
            level: Number(row.progressLevel || row.baseLevel || 1),
            exp: Number(row.progressExp || 0),
            nextExp: Number(row.progressNextExp || 0)
          }
        }
      }
      if (role.versions.attributes <= 0) {
        this.applyAttributeSnapshot(roleKey, attributes, progress)
      } else {
        role.initialized.attributes = true
      }
      if (role.versions.equipment <= 0) {
        this.applyEquipmentSnapshot(roleKey, profile?.equipment || [], 0)
      } else {
        role.initialized.equipment = true
      }
      role.capabilities = { ...role.capabilities, ...(profile?.capabilities || {}) }
      role.initialized.profile = true
    },
    applyAttributeSnapshot(roleKey: RoleKey, attributes: Record<string, unknown>, progress: Record<string, unknown>) {
      const role = this.ensureRole(roleKey)
      const nextAttributes: Record<string, AttributeNode> = {}
      for (const [key, raw] of Object.entries(attributes || {})) {
        const node = normalizeAttributeNode(raw)
        if (node) nextAttributes[key] = node
      }
      const nextProgress: Record<string, AttributeProgressNode> = {}
      for (const [key, raw] of Object.entries(progress || {})) {
        const node = normalizeProgressNode(raw)
        if (node) nextProgress[key] = node
      }
      role.attributes = nextAttributes
      role.attributeProgress = nextProgress
      role.initialized.attributes = true
    },
    applyEquipmentSnapshot(roleKey: RoleKey, items: unknown[] | null | undefined, version = 0) {
      const role = this.ensureRole(roleKey)
      role.equipment = (items || []).map(normalizeEquipmentItem).filter((item): item is RoleEquipmentItem => !!item)
      if (Number.isFinite(version) && version >= 0) {
        role.versions.equipment = Math.trunc(version)
      }
      role.initialized.equipment = true
    },
    beginEquipmentRealtimeSession(roleKey: RoleKey) {
      const role = this.ensureRole(roleKey)
      role.versions.equipment = 0
    },
    patchRoleAttributes(roleKey: RoleKey, payload: RoleAttributePatchPayload | null | undefined) {
      if (!payload || typeof payload !== "object") return
      const role = this.ensureRole(roleKey)
      const version = Number(payload.meta?.version || 0)
      if (version > 0 && version <= role.versions.attributes) return
      const nextAttributes = { ...role.attributes }
      for (const [key, raw] of Object.entries(payload.upsert || {})) {
        const node = normalizeAttributeNode(raw, nextAttributes[key])
        if (node) nextAttributes[key] = node
      }
      for (const key of payload.remove || []) {
        if (typeof key === "string") delete nextAttributes[key]
      }
      const nextProgress = { ...role.attributeProgress }
      for (const [key, raw] of Object.entries(payload.attributeProgressUpsert || {})) {
        const node = normalizeProgressNode(raw, nextProgress[key])
        if (node) nextProgress[key] = node
      }
      for (const key of payload.attributeProgressRemove || []) {
        if (typeof key === "string") delete nextProgress[key]
      }
      role.attributes = nextAttributes
      role.attributeProgress = nextProgress
      if (version > 0) role.versions.attributes = version
      role.initialized.attributes = true
    },
    patchRoleAttributesPayload(payload: RoleAttributePatchPayload | null | undefined) {
      if (!payload) return
      this.patchRoleAttributes(roleKeyFor(payload.roleType, payload.roleId, payload.kittyId), payload)
    },
    patchRoleEquipment(roleKey: RoleKey, payload: RoleEquipmentPatchPayload | null | undefined) {
      if (!payload || typeof payload !== "object") return
      const role = this.ensureRole(roleKey)
      if (Array.isArray(payload.items)) {
        const version = Number(payload.meta?.version || 0)
        this.applyEquipmentSnapshot(roleKey, payload.items, version)
        return
      }
      const version = Number(payload.meta?.version || 0)
      if (version > 0 && version <= role.versions.equipment) return
      const byID = new Map<number, RoleEquipmentItem>()
      for (const item of role.equipment) {
        byID.set(item.instanceId, item)
      }
      for (const raw of payload.upsert || []) {
        const item = normalizeEquipmentItem(raw)
        if (item) byID.set(item.instanceId, item)
      }
      for (const id of payload.remove || []) {
        if (typeof id === "number") byID.delete(id)
      }
      role.equipment = Array.from(byID.values())
      if (version > 0) role.versions.equipment = version
      role.initialized.equipment = true
    },
    patchRoleEquipmentPayload(payload: RoleEquipmentPatchPayload | null | undefined) {
      if (!payload) return
      this.patchRoleEquipment(roleKeyFor(payload.roleType, payload.roleId, payload.kittyId), payload)
    },
    patchRoleMutationPayload(payload: RoleMutationPatchPayload | null | undefined) {
      if (!payload || typeof payload !== "object") return
      if (payload.roleAttributePatch) {
        this.patchRoleAttributesPayload(payload.roleAttributePatch as RoleAttributePatchPayload)
      }
      if (payload.roleEquipmentPatch) {
        this.patchRoleEquipmentPayload(payload.roleEquipmentPatch as RoleEquipmentPatchPayload)
      }
    },
    setRoleBattlePoints(roleKey: RoleKey, payload: Partial<BattlePointState> | null | undefined) {
      const role = this.ensureRole(roleKey)
      role.battlePoints = normalizeBattlePointState(payload)
    },
    roleUnitAttributes(roleKey: RoleKey): Record<string, number> {
      const role = this.ensureRole(roleKey)
      const out: Record<string, number> = {}
      for (const [key, node] of Object.entries(role.attributes) as Array<[string, AttributeNode]>) {
        const value = Number(node.value || 0)
        if (Number.isFinite(value)) out[key] = value
      }
      return out
    },
    roleCapabilities(roleKey: RoleKey): RoleCapabilities {
      return this.ensureRole(roleKey).capabilities
    },
    reset() {
      this.roles = {}
    }
  }
})

export { roleKeyFor }
