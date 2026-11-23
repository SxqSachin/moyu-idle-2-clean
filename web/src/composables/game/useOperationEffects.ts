import { computed, getCurrentScope, onMounted, onScopeDispose, ref, type ComputedRef } from "vue"
import { useOperationEffectsStore } from "@stores/operation-effects"
import { operationEffectMatchesQuery, operationEffectEligibility, type OperationEffectQuery } from "@services/operation-effect-context"
import { MAX_QUANTITY_LIMIT_BONUS_RATE, MAX_SUCCESS_RATE_PENALTY, MAX_SUCCESS_RATE_BONUS, infusionTierRateLimit, type InfusionTierCapacities } from "@services/alchemy-operation-effects"
import {
  fetchOperationEffects,
  useOperationEffectItem,
  type OperationEffectActivationTarget,
  type OperationEffectProjection,
  type OperationEffectTargetKind,
  type OperationEffectView
} from "@services/operation-effects"

const MAX_EXTRA_MATERIAL_QUANTITY = 1_000_000
const MAX_POSITIVE_FLOOR_REDUCTION_RATE = 0.95
const MAX_POSITIVE_CEILING_BONUS_RATE = 0.25

export type AlchemyInfusionModifiers = {
  tierCapacities?: InfusionTierCapacities
  extraMaterialQuantity: number
  bulkMaterialQuantity: number
  preciousMaterialQuantity: number
  bulkQuantityLimitBonusRate: number
  preciousQuantityLimitBonusRate: number
  positiveFloorReductionRate: number
  positiveCeilingBonusRate: number
  successRatePenalty: number
  successRateBonus: number
}

type UseOperationEffectsOptions = {
  token?: () => string
  autoRefresh?: boolean
}

let refreshRequest: { token: string; promise: Promise<OperationEffectProjection> } | null = null
let cacheEpoch = 0
let cacheToken = ""
const clockPulse = ref(0)
let clockTimer: number | null = null
let clockConsumers = 0

function retainClock() {
  if (typeof window === "undefined") return
  clockConsumers++
  if (clockTimer !== null) return
  clockTimer = window.setInterval(() => {
    clockPulse.value++
  }, 1_000)
}

function releaseClock() {
  if (typeof window === "undefined") return
  clockConsumers = Math.max(0, clockConsumers - 1)
  if (clockConsumers > 0 || clockTimer === null) return
  window.clearInterval(clockTimer)
  clockTimer = null
}

function finiteNumber(value: unknown): number {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : 0
}

function foldAlchemyInfusionModifiers(effects: OperationEffectView[]): AlchemyInfusionModifiers {
  const result: AlchemyInfusionModifiers = {
    extraMaterialQuantity: 0,
    bulkMaterialQuantity: 0,
    preciousMaterialQuantity: 0,
    bulkQuantityLimitBonusRate: 0,
    preciousQuantityLimitBonusRate: 0,
    positiveFloorReductionRate: 0,
    positiveCeilingBonusRate: 0,
    successRatePenalty: 0,
    successRateBonus: 0
  }
  for (const effect of effects) {
    const params = effect.params || {}
    if (["alchemy.infusion_bulk_capacity_floor_tradeoff", "alchemy.infusion_precious_capacity_bonus"].includes(effect.kind)) {
      result.successRatePenalty = Math.min(MAX_SUCCESS_RATE_PENALTY, result.successRatePenalty + Math.max(0, finiteNumber(params.successRatePenalty)))
    }
    if (["alchemy.infusion_capacity_floor_tradeoff", "alchemy.infusion_bulk_capacity_floor_tradeoff"].includes(effect.kind)) {
      const tier = Number(params.materialTier || 0)
      const field = effect.kind === "alchemy.infusion_bulk_capacity_floor_tradeoff" ? "bulkMaterialQuantity" : "extraMaterialQuantity"
      if (tier === 1 || tier === 2) {
        const capacities = result.tierCapacities ||= {}
        const capacity = capacities[tier] ||= { quantity: 0, rate: 0 }
        capacity.quantity = Math.min(MAX_EXTRA_MATERIAL_QUANTITY, capacity.quantity + Math.max(0, Math.trunc(finiteNumber(params.extraMaterialQuantity))))
        capacity.rate = Math.min(infusionTierRateLimit(tier), capacity.rate + Math.max(0, finiteNumber(params.quantityLimitBonusRate)))
      } else result[field] = Math.min(
        MAX_EXTRA_MATERIAL_QUANTITY,
        result[field] + Math.max(0, Math.trunc(finiteNumber(params.extraMaterialQuantity)))
      )
      result.positiveFloorReductionRate = Math.min(
        MAX_POSITIVE_FLOOR_REDUCTION_RATE,
        result.positiveFloorReductionRate + Math.max(0, finiteNumber(params.positiveFloorReductionRate))
      )
      if (effect.kind === "alchemy.infusion_bulk_capacity_floor_tradeoff" && !tier) {
        result.bulkQuantityLimitBonusRate = Math.min(
          MAX_QUANTITY_LIMIT_BONUS_RATE,
          result.bulkQuantityLimitBonusRate + Math.max(0, finiteNumber(params.quantityLimitBonusRate))
        )
      }
    } else if (effect.kind === "alchemy.infusion_precious_capacity_bonus") {
      result.preciousMaterialQuantity = Math.min(MAX_EXTRA_MATERIAL_QUANTITY, result.preciousMaterialQuantity + Math.max(0, Math.trunc(finiteNumber(params.extraMaterialQuantity))))
      result.preciousQuantityLimitBonusRate = Math.min(
        MAX_QUANTITY_LIMIT_BONUS_RATE,
        result.preciousQuantityLimitBonusRate + Math.max(0, finiteNumber(params.quantityLimitBonusRate))
      )
    } else if (effect.kind === "alchemy.infusion_success_bonus") {
      result.successRateBonus = Math.min(MAX_SUCCESS_RATE_BONUS, result.successRateBonus + Math.max(0, finiteNumber(params.bonusRate)))
    } else if (effect.kind === "alchemy.infusion_precision_ceiling_bonus") {
      result.positiveCeilingBonusRate = Math.min(
        MAX_POSITIVE_CEILING_BONUS_RATE,
        result.positiveCeilingBonusRate + Math.max(0, finiteNumber(params.positiveCeilingBonusRate))
      )
    }
  }
  return result
}

function enterAuthScope(token: string, resetCache: () => void): number {
  const normalizedToken = String(token || "").trim()
  if (cacheToken && normalizedToken && cacheToken !== normalizedToken) {
    cacheEpoch++
    refreshRequest = null
    resetCache()
  }
  if (normalizedToken) cacheToken = normalizedToken
  return cacheEpoch
}

export function useOperationEffects(options: UseOperationEffectsOptions = {}) {
  const store = useOperationEffectsStore()
  const serverNowMs = computed(() => {
    void clockPulse.value
    return store.estimatedServerNow()
  })
  const activeEffects = computed(() => store.activeItemsAt(serverNowMs.value))

  function effectsFor(query: OperationEffectQuery): OperationEffectView[] {
    return activeEffects.value.filter((effect) => operationEffectMatchesQuery(effect, query))
  }

  function enhancementSuccessBonusFor(actor: Pick<OperationEffectQuery, "actorKind" | "actorId"> = {}) {
    return Math.min(0.10, effectsFor({ domain: "enhancement", actionId: "equipment_enhance", ...actor })
    .filter((effect) => effect.kind === "enhancement.success_bonus")
    .reduce((sum, effect) => sum + Math.max(0, finiteNumber(effect.params.bonusRate)), 0))
  }
  const enhancementSuccessBonus = computed(() => enhancementSuccessBonusFor())
  const fishingBiteBonus = computed(() => Math.min(0.25, effectsFor({ domain: "fishing_cast", actionId: "fishing.trip", actorKind: "player" })
    .filter((effect) => effect.kind === "fishing_cast.bite_bonus")
    .reduce((sum, effect) => sum + Math.max(0, finiteNumber(effect.params.bonusRate)), 0)))
  const fishingFightModifiers = computed(() => {
    const effects = effectsFor({ domain: "fishing_cast", actionId: "fishing.trip", actorKind: "player" })
    const sum = (kind: string) => Math.min(0.15, effects.filter((effect) => effect.kind === kind)
      .reduce((total, effect) => total + Math.max(0, finiteNumber(effect.params.rate)), 0))
    return { lineLoadReductionRate: sum("fishing_cast.line_load_reduction"), landingBonusRate: sum("fishing_cast.landing_bonus") }
  })

  const alchemyInfusionModifiers: ComputedRef<AlchemyInfusionModifiers> = computed(() =>
    foldAlchemyInfusionModifiers(effectsFor({
      domain: "alchemy",
      actionId: "alchemy.bench",
      actorKind: "player"
    }))
  )

  function alchemyInfusionModifiersForMaterials(rarities: string[]): AlchemyInfusionModifiers {
    return foldAlchemyInfusionModifiers(effectsFor({ domain: "alchemy", actionId: "alchemy.bench", actorKind: "player" })
      .filter(effect => operationEffectEligibility(effect, { domain: "alchemy", infusionMode: "materials", materialRarities: rarities }).applicable))
  }

  function hydrate(payload: OperationEffectProjection | null | undefined): boolean {
    const scopedToken = options.token?.() || ""
    if (options.token && !String(scopedToken).trim()) return false
    enterAuthScope(scopedToken, store.reset)
    return store.applySnapshot(payload)
  }

  async function refresh(token: string, _force = false): Promise<OperationEffectProjection> {
    const normalizedToken = String(token || "").trim()
    if (!normalizedToken) throw new Error("operation effects token is required")
    const requestEpoch = enterAuthScope(normalizedToken, store.reset)
    if (refreshRequest?.token === normalizedToken) return refreshRequest.promise
    const promise = fetchOperationEffects(normalizedToken)
      .then((projection) => {
        if (requestEpoch === cacheEpoch && cacheToken === normalizedToken) {
          store.applySnapshot(projection)
        }
        return projection
      })
      .catch((error) => {
        if (requestEpoch === cacheEpoch && cacheToken === normalizedToken) store.markStale()
        throw error
      })
      .finally(() => {
        if (refreshRequest?.promise === promise) refreshRequest = null
      })
    refreshRequest = { token: normalizedToken, promise }
    return promise
  }

  async function activate(
    token: string,
    instanceId: number,
    target: OperationEffectActivationTarget,
    requestId: string,
    quantity = 1
  ): Promise<OperationEffectProjection> {
    const normalizedToken = String(token || "").trim()
    if (!normalizedToken) throw new Error("operation effects token is required")
    const requestEpoch = enterAuthScope(normalizedToken, store.reset)
    const projection = await useOperationEffectItem(normalizedToken, instanceId, target, requestId, quantity)
    if (requestEpoch === cacheEpoch && cacheToken === normalizedToken) {
      store.applySnapshot(projection)
    }
    return projection
  }

  function invalidate(token = "") {
    store.markStale()
    if (token) void refresh(token, true).catch(() => undefined)
  }

  function reset() {
    cacheEpoch++
    cacheToken = ""
    refreshRequest = null
    store.reset()
  }

  let removeAutoRefresh: (() => void) | null = null
  const installAutoRefresh = () => {
    if (!options.autoRefresh || typeof window === "undefined" || removeAutoRefresh) return
    const refreshIfResumed = () => {
      if (document.visibilityState !== "visible" || !navigator.onLine) return
      const token = options.token?.() || ""
      if (token && store.initialized) void refresh(token, true).catch(() => undefined)
    }
    window.addEventListener("online", refreshIfResumed)
    document.addEventListener("visibilitychange", refreshIfResumed)
    removeAutoRefresh = () => {
      window.removeEventListener("online", refreshIfResumed)
      document.removeEventListener("visibilitychange", refreshIfResumed)
    }
  }

  if (getCurrentScope()) {
    onMounted(() => {
      retainClock()
      installAutoRefresh()
    })
    onScopeDispose(() => {
      releaseClock()
      removeAutoRefresh?.()
      removeAutoRefresh = null
    })
  }

  return {
    initialized: computed(() => store.initialized),
    revision: computed(() => store.revision),
    stale: computed(() => store.stale),
    serverNowMs,
    activeEffects,
    alchemyInfusionModifiers,
    alchemyInfusionModifiersForMaterials,
    enhancementSuccessBonus,
    enhancementSuccessBonusFor,
    fishingBiteBonus,
    fishingFightModifiers,
    effectsFor,
    hydrate,
    refresh,
    activate,
    invalidate,
    reset
  }
}

export type { OperationEffectActivationTarget, OperationEffectTargetKind }
