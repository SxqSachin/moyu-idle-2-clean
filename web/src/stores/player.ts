import { defineStore } from "pinia"
import { apiGet } from "@services/api"

type AttributeProgressNode = {
  value: number
  level: number
  exp: number
  nextExp: number
}

type AttributeSnapshotNode = {
  value: number
  baseValue?: number
  skillPointLevel?: number
  allowBattlePointLevelUp?: boolean
  unit: "number" | "percent"
  category: "work" | "battle" | "other"
  growthType: "levelable" | "static"
  order: number
  nameKey?: string
  descKey?: string
  iconId?: string
  emojiIcon?: string
  sources?: unknown
}

type FormulaSnapshot = {
  version?: string
  operators?: string[]
  derived?: Record<string, string>
}

type KittySnapshot = {
  id: number
  name: string
  status?: string
  level: number
  battleLevelCap?: number
  exp: number
  nextExp: number
  totalExp: number
  bondExp: number
  mood: number
  extra?: Record<string, unknown>
  createdAt: string
}

type CatVillageReputationState = {
  totalReputation: number
  level: number
  levelReputation: number
  nextLevelXp: number
}

type PlayerStatsSnapshot = {
  stats?: Record<string, number>
  attributes?: Record<string, Partial<AttributeSnapshotNode>>
  attributeProgress?: Record<string, Partial<AttributeProgressNode>>
  formula?: FormulaSnapshot
  basicAttack?: Record<string, unknown> | null
  [key: string]: unknown
}

function defaultStats() {
  return {} as Record<string, number>
}

function defaultAttributeProgress(): Record<string, AttributeProgressNode> {
  return {}
}

function defaultAttributes(): Record<string, AttributeSnapshotNode> {
  return {}
}

function defaultBuildings(): Record<string, number> {
  return {}
}

function defaultFormula() {
  return { version: "v2", operators: ["+", "-", "*", "/"], derived: {} as Record<string, string> }
}

function defaultKitties(): KittySnapshot[] {
  return []
}

function defaultCatVillageReputation(): CatVillageReputationState {
  return { totalReputation: 0, level: 0, levelReputation: 0, nextLevelXp: 0 }
}

function assignIfNumber(target: Record<string, number>, source: Record<string, unknown>, key: string) {
  const value = source[key]
  if (typeof value === "number" && Number.isFinite(value)) {
    target[key] = value
  }
}

export const usePlayerStore = defineStore("player", {
  state: () => ({
    initialized: false,
    attributesPatchVersion: 0,
    snapshot: {} as Record<string, unknown>,
    stats: defaultStats(),
    attributes: defaultAttributes(),
    buildings: defaultBuildings(),
    kitties: defaultKitties(),
    catVillageReputation: defaultCatVillageReputation(),
    attributeProgress: defaultAttributeProgress(),
    formula: defaultFormula(),
    basicAttack: null as Record<string, unknown> | null
  }),
  actions: {
    async initialize(token: string, force = false) {
      if (!token) {
        this.reset()
        return
      }
      if (this.initialized && !force) return
      const data = await apiGet<PlayerStatsSnapshot>("/api/player/stats", token)
      this.applyFullSnapshot(data || {})
    },
    applyFullSnapshot(payload: PlayerStatsSnapshot) {
      this.snapshot = { ...(payload || {}) }
      this.stats = defaultStats()
      this.attributes = defaultAttributes()
      this.buildings = defaultBuildings()
      this.kitties = defaultKitties()
      this.catVillageReputation = defaultCatVillageReputation()
      this.attributeProgress = defaultAttributeProgress()
      this.formula = defaultFormula()
      this.basicAttack = null
      this.applyPartialSnapshot(payload || {})
      this.initialized = true
    },
    patchFromWS(payload: Record<string, unknown> | null | undefined) {
      if (!payload || typeof payload !== "object") return
      this.snapshot = { ...this.snapshot, ...payload }
      this.applyPartialSnapshot(payload)
    },
    patchFromRequest(payload: Record<string, unknown> | null | undefined) {
      if (!payload || typeof payload !== "object") return
      this.snapshot = { ...this.snapshot, ...payload }
      this.applyPartialSnapshot(payload)
    },
    patchAttributesEvent(payload: Record<string, unknown> | null | undefined) {
      if (!payload || typeof payload !== "object") return
      const patchVersion = Number((payload.meta as { version?: unknown } | undefined)?.version || 0)
      if (!Number.isFinite(patchVersion) || patchVersion <= this.attributesPatchVersion) return
      const nextAttributes = { ...this.attributes }
      const upsert = payload.upsert
      if (upsert && typeof upsert === "object") {
        for (const [key, node] of Object.entries(upsert as Record<string, unknown>)) {
          if (!node || typeof node !== "object") continue
          const nextNode = node as Record<string, unknown>
          if (!nextAttributes[key]) {
            nextAttributes[key] = { value: 0, unit: "number", category: "other", growthType: "static", order: 0 }
          }
          assignIfNumber(nextAttributes[key] as Record<string, number>, nextNode, "value")
          assignIfNumber(nextAttributes[key] as Record<string, number>, nextNode, "baseValue")
          assignIfNumber(nextAttributes[key] as Record<string, number>, nextNode, "skillPointLevel")
          assignIfNumber(nextAttributes[key] as Record<string, number>, nextNode, "order")
          if (typeof nextNode.allowBattlePointLevelUp === "boolean") nextAttributes[key].allowBattlePointLevelUp = nextNode.allowBattlePointLevelUp
          const unit = nextNode.unit
          if (unit === "number" || unit === "percent") nextAttributes[key].unit = unit
          const category = nextNode.category
          if (category === "work" || category === "battle" || category === "other") nextAttributes[key].category = category
          const growthType = nextNode.growthType
          if (growthType === "levelable" || growthType === "static") nextAttributes[key].growthType = growthType
          if (typeof nextNode.nameKey === "string") nextAttributes[key].nameKey = nextNode.nameKey
          if (typeof nextNode.descKey === "string") nextAttributes[key].descKey = nextNode.descKey
          if (typeof nextNode.iconId === "string") nextAttributes[key].iconId = nextNode.iconId
          if (typeof nextNode.emojiIcon === "string") nextAttributes[key].emojiIcon = nextNode.emojiIcon
          if (nextNode.sources !== undefined) nextAttributes[key].sources = nextNode.sources
        }
      }
      const remove = payload.remove
      if (Array.isArray(remove)) {
        for (const key of remove) {
          if (typeof key !== "string") continue
          delete nextAttributes[key]
        }
      }

      const nextProgress = { ...this.attributeProgress }
      const progressUpsert = payload.attributeProgressUpsert
      if (progressUpsert && typeof progressUpsert === "object") {
        for (const [key, node] of Object.entries(progressUpsert as Record<string, unknown>)) {
          if (!nextProgress[key]) {
            nextProgress[key] = { value: 0, level: 1, exp: 0, nextExp: 100 }
          }
          if (!node || typeof node !== "object") continue
          const nextNode = node as Record<string, unknown>
          assignIfNumber(nextProgress[key], nextNode, "value")
          assignIfNumber(nextProgress[key], nextNode, "level")
          assignIfNumber(nextProgress[key], nextNode, "exp")
          assignIfNumber(nextProgress[key], nextNode, "nextExp")
        }
      }
      const progressRemove = payload.attributeProgressRemove
      if (Array.isArray(progressRemove)) {
        for (const key of progressRemove) {
          if (typeof key !== "string") continue
          delete nextProgress[key]
        }
      }

      this.attributes = nextAttributes
      this.attributeProgress = nextProgress
      this.attributesPatchVersion = patchVersion
      this.snapshot = {
        ...this.snapshot,
        attributes: this.attributes,
        attributeProgress: this.attributeProgress
      }
    },
    applyPartialSnapshot(payload: Record<string, unknown>) {
      const rawStats = payload.stats
      if (rawStats && typeof rawStats === "object") {
        for (const [key, value] of Object.entries(rawStats as Record<string, unknown>)) {
          if (typeof value === "number" && Number.isFinite(value)) {
            this.stats[key] = value
          }
        }
      }

      const rawAttributes = payload.attributes
      if (rawAttributes && typeof rawAttributes === "object") {
        for (const [key, node] of Object.entries(rawAttributes as Record<string, unknown>)) {
          if (!node || typeof node !== "object") continue
          const nextNode = node as Record<string, unknown>
          if (!this.attributes[key]) {
            this.attributes[key] = { value: 0, unit: "number", category: "other", growthType: "static", order: 0 }
          }
          assignIfNumber(this.attributes[key] as Record<string, number>, nextNode, "value")
          assignIfNumber(this.attributes[key] as Record<string, number>, nextNode, "baseValue")
          assignIfNumber(this.attributes[key] as Record<string, number>, nextNode, "skillPointLevel")
          assignIfNumber(this.attributes[key] as Record<string, number>, nextNode, "order")
          if (typeof nextNode.allowBattlePointLevelUp === "boolean") this.attributes[key].allowBattlePointLevelUp = nextNode.allowBattlePointLevelUp
          const unit = nextNode.unit
          if (unit === "number" || unit === "percent") this.attributes[key].unit = unit
          const category = nextNode.category
          if (category === "work" || category === "battle" || category === "other") this.attributes[key].category = category
          const growthType = nextNode.growthType
          if (growthType === "levelable" || growthType === "static") this.attributes[key].growthType = growthType
          if (typeof nextNode.nameKey === "string") this.attributes[key].nameKey = nextNode.nameKey
          if (typeof nextNode.descKey === "string") this.attributes[key].descKey = nextNode.descKey
          if (typeof nextNode.iconId === "string") this.attributes[key].iconId = nextNode.iconId
          if (typeof nextNode.emojiIcon === "string") this.attributes[key].emojiIcon = nextNode.emojiIcon
          if (nextNode.sources !== undefined) this.attributes[key].sources = nextNode.sources
        }
      }

      const rawProgress = payload.attributeProgress
      if (rawProgress && typeof rawProgress === "object") {
        for (const [key, node] of Object.entries(rawProgress as Record<string, unknown>)) {
          if (!this.attributeProgress[key]) {
            this.attributeProgress[key] = { value: 0, level: 1, exp: 0, nextExp: 100 }
          }
          if (!node || typeof node !== "object") continue
          const nextNode = node as Record<string, unknown>
          assignIfNumber(this.attributeProgress[key], nextNode, "value")
          assignIfNumber(this.attributeProgress[key], nextNode, "level")
          assignIfNumber(this.attributeProgress[key], nextNode, "exp")
          assignIfNumber(this.attributeProgress[key], nextNode, "nextExp")
        }
      }

      const rawFormula = payload.formula
      if (rawFormula && typeof rawFormula === "object") {
        const formula = rawFormula as Record<string, unknown>
        if (typeof formula.version === "string") this.formula.version = formula.version
        if (Array.isArray(formula.operators)) {
          this.formula.operators = formula.operators.filter((item): item is string => typeof item === "string")
        }
        if (formula.derived && typeof formula.derived === "object") {
          const next: Record<string, string> = {}
          for (const [k, v] of Object.entries(formula.derived as Record<string, unknown>)) {
            if (typeof v === "string") next[k] = v
          }
          this.formula.derived = next
        }
      }

      const rawBuildings = payload.buildings
      if (rawBuildings && typeof rawBuildings === "object") {
        const next: Record<string, number> = {}
        for (const [key, value] of Object.entries(rawBuildings as Record<string, unknown>)) {
          if (typeof value !== "number" || !Number.isFinite(value)) continue
          next[key] = Math.max(0, Math.floor(value))
        }
        this.buildings = next
      }

      const rawKitties = payload.kitties
      if (Array.isArray(rawKitties)) {
        const next: KittySnapshot[] = []
        for (const row of rawKitties) {
          if (!row || typeof row !== "object") continue
          const node = row as Record<string, unknown>
          const id = Number(node.id || 0)
          const name = typeof node.name === "string" ? node.name.trim() : ""
          const status = typeof node.status === "string" ? node.status.trim() : "idle"
          const createdAt = typeof node.createdAt === "string" ? node.createdAt : ""
          if (!Number.isFinite(id) || id <= 0 || !name) continue
          const level = Math.max(1, Math.floor(Number(node.level || 1)))
          const exp = Math.max(0, Math.floor(Number(node.exp || 0)))
          const nextExp = Math.max(0, Math.floor(Number(node.nextExp || 0)))
          const totalExp = Math.max(0, Math.floor(Number(node.totalExp || 0)))
          const bondExp = Math.max(0, Math.floor(Number(node.bondExp || 0)))
          const moodRaw = Number(node.mood || 0)
          const mood = Number.isFinite(moodRaw) ? Math.max(-1, Math.min(1, moodRaw)) : 0
          const extra = node.extra && typeof node.extra === "object" && !Array.isArray(node.extra)
            ? node.extra as Record<string, unknown>
            : undefined
          next.push({ id, name, status, level, exp, nextExp, totalExp, bondExp, mood, extra, createdAt })
        }
        this.kitties = next
      }

      const rawCatVillageReputation = payload.catVillageReputation
      if (rawCatVillageReputation && typeof rawCatVillageReputation === "object") {
        const node = rawCatVillageReputation as Record<string, unknown>
        const totalReputation = Math.max(0, Math.floor(Number(node.totalReputation || 0)))
        const level = Math.max(0, Math.floor(Number(node.level || 0)))
        const levelReputation = Math.max(0, Math.floor(Number(node.levelReputation || 0)))
        const nextLevelXp = Math.max(0, Math.floor(Number(node.nextLevelXp || 0)))
        this.catVillageReputation = { totalReputation, level, levelReputation, nextLevelXp }
      }

      const rawBasicAttack = payload.basicAttack
      if (rawBasicAttack && typeof rawBasicAttack === "object" && !Array.isArray(rawBasicAttack)) {
        this.basicAttack = rawBasicAttack as Record<string, unknown>
      }
    },
    reset() {
      this.initialized = false
      this.attributesPatchVersion = 0
      this.snapshot = {}
      this.stats = defaultStats()
      this.attributes = defaultAttributes()
      this.buildings = defaultBuildings()
      this.kitties = defaultKitties()
      this.catVillageReputation = defaultCatVillageReputation()
      this.attributeProgress = defaultAttributeProgress()
      this.formula = defaultFormula()
      this.basicAttack = null
    }
  }
})
