import { defineStore } from "pinia"
import { computed, ref } from "vue"
import { apiGet } from "@services/api"
import { getGameDataChunk } from "@services/game-data-runtime"

type CollectibleDefinition = {
  id: string
  itemId: string
  nameKey: string
  descKey: string
  playerMods: Record<string, number>
  kittyMods: Record<string, number>
  rarity: string
  categoryId: string
  disabled?: boolean
  upgradeable?: boolean
  levelMaxLevel?: number
  levelAttrCurve?: {
    base: Record<string, number>
    step: Record<string, number>
    overrides: Record<string, Record<string, number>>
  }
}

type PlayerCollectibleState = {
  collectibleId: string
  level: number
  obtainedAt: string
}

export type PlayerCollectibleDetail = {
  collectibleId: string
  itemId: string
  nameKey: string
  descKey: string
  playerMods: Record<string, number>
  kittyMods: Record<string, number>
  level: number
  upgradeable: boolean
  obtainedAt: string
}

type CollectiblesChunk = { collectibles?: CollectibleDefinition[] }

let definitionByIdCache: Map<string, CollectibleDefinition> | null = null

function getDefinitionById(): Map<string, CollectibleDefinition> {
  if (definitionByIdCache) return definitionByIdCache
  const collectiblesChunk = getGameDataChunk("collectibles")
  const localDefinitions = ((collectiblesChunk as CollectiblesChunk).collectibles ?? [])
    .filter((def) => !def.disabled)
  definitionByIdCache = new Map(localDefinitions.map((def) => [def.id, def] as const))
  return definitionByIdCache
}

// Replicates backend resolveEnhanceAttrIncrement logic.
function resolveAttrIncrement(
  curve: { base: Record<string, number>; step: Record<string, number>; overrides: Record<string, Record<string, number>> },
  level: number,
  maxLevel: number
): Record<string, number> {
  if (level <= 0) return {}
  if (maxLevel > 0 && level > maxLevel) return {}
  const override = curve.overrides[String(level)]
  if (override) return { ...override }
  const out: Record<string, number> = {}
  for (const [key, base] of Object.entries(curve.base)) {
    if (!key || !Number.isFinite(base)) continue
    out[key] = level === 1 ? base : (curve.step[key] ?? 0)
  }
  for (const [key, step] of Object.entries(curve.step)) {
    if (!key || !Number.isFinite(step)) continue
    if (key in out) continue
    out[key] = level === 1 ? 0 : step
  }
  return out
}

function resolveLevelMods(def: CollectibleDefinition, level: number): Record<string, number> {
  const curve = def.levelAttrCurve
  if (!curve || level <= 0) return {}
  const maxLevel = Number(def.levelMaxLevel ?? 0)
  const out: Record<string, number> = {}
  for (let l = 1; l <= level; l++) {
    const inc = resolveAttrIncrement(curve, l, maxLevel)
    for (const [k, v] of Object.entries(inc)) {
      out[k] = (out[k] ?? 0) + v
    }
  }
  return out
}

function buildDetail(def: CollectibleDefinition, state: PlayerCollectibleState): PlayerCollectibleDetail {
  const upgradeable = !!def.upgradeable
  const playerMods = upgradeable ? resolveLevelMods(def, state.level) : (def.playerMods ?? {})
  return {
    collectibleId: def.id,
    itemId: def.itemId,
    nameKey: def.nameKey,
    descKey: def.descKey,
    playerMods,
    kittyMods: def.kittyMods ?? {},
    level: state.level,
    upgradeable,
    obtainedAt: state.obtainedAt
  }
}

export function mergeCollectibleStates(states: PlayerCollectibleState[]): PlayerCollectibleDetail[] {
  const definitionById = getDefinitionById()
  const result: PlayerCollectibleDetail[] = []
  for (const state of states) {
    const def = definitionById.get(state.collectibleId)
    if (!def) continue
    result.push(buildDetail(def, state))
  }
  return result
}

export type { PlayerCollectibleState }

export const useCollectiblesStore = defineStore("collectibles", () => {
  const states = ref<PlayerCollectibleState[]>([])
  const loaded = ref(false)

  const collectibles = computed<PlayerCollectibleDetail[]>(() => mergeCollectibleStates(states.value))

  async function load(token: string) {
    const data = await apiGet<{ collectibles: PlayerCollectibleState[] }>("/api/collectibles", token)
    states.value = data.collectibles ?? []
    loaded.value = true
  }

  function reset() {
    states.value = []
    loaded.value = false
  }

  return { collectibles, loaded, load, reset }
})
