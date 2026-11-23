import { computed, ref } from "vue"
import { defineStore } from "pinia"
import { useGameStore } from "@stores/game"
import { APIClientError } from "@services/api"
import {
  PLAYER_COMBAT_SPECIALIZATION_TREE_ID,
  allocatePlayerSpecialization,
  fetchPlayerSpecialization,
  isSpecializationStateConflictError,
  resetPlayerSpecialization,
  type SpecializationMutationResult,
  type SpecializationSummary
} from "@services/specialization"

export const useSpecializationStore = defineStore("specialization", () => {
  const gameStore = useGameStore()
  const summary = ref<SpecializationSummary | null>(null)
  const loading = ref(false)
  const mutating = ref(false)
  const loadError = ref("")
  let activeLoad: { key: string; generation: number; promise: Promise<SpecializationSummary | null> } | null = null
  let activeMutation: { id: number; key: string } | null = null
  let nextMutationId = 0
  let loadedPrincipalKey = ""
  let generation = 0

  function principalKey() {
    return gameStore.token && gameStore.userId > 0
      ? `${gameStore.userId}:${gameStore.token}`
      : ""
  }

  const enabledBattleSlotIds = computed(() =>
    (summary.value?.effectiveSlots || [])
      .filter((slot) => slot.enabled)
      .map((slot) => slot.slot)
  )

  async function load(force = false): Promise<SpecializationSummary | null> {
    const key = principalKey()
    const token = gameStore.token
    if (!key || !token) return null
    if (loadedPrincipalKey !== key) {
      generation += 1
      activeLoad = null
      summary.value = null
      loadError.value = ""
      loading.value = false
      mutating.value = false
      activeMutation = null
      loadedPrincipalKey = key
    }
    if (activeLoad?.key === key) return activeLoad.promise
    if (!force && summary.value) return summary.value
    const requestGeneration = ++generation
    loading.value = true
    loadError.value = ""
    const promise = fetchPlayerSpecialization(token, PLAYER_COMBAT_SPECIALIZATION_TREE_ID)
      .then((next) => {
        if (generation !== requestGeneration || principalKey() !== key) return null
        summary.value = next
        return next
      })
      .catch((error: unknown) => {
        if (generation === requestGeneration && principalKey() === key) {
          loadError.value = error instanceof Error ? error.message : "specialization load failed"
        }
        throw error
      })
      .finally(() => {
        if (activeLoad?.generation === requestGeneration) activeLoad = null
        if (generation === requestGeneration && principalKey() === key) loading.value = false
      })
    activeLoad = { key, generation: requestGeneration, promise }
    return promise
  }

  function staleMutationError() {
    return Object.assign(new Error("specialization principal changed during mutation"), {
      code: "SPECIALIZATION_PRINCIPAL_CHANGED"
    })
  }

  function assertMutationPrincipal(key: string, mutationId: number) {
    if (principalKey() !== key || activeMutation?.id !== mutationId) throw staleMutationError()
  }

  function invalidateLoads() {
    generation += 1
    activeLoad = null
    loading.value = false
  }

  async function refreshAfterMutationConflict(error: unknown, key: string, mutationId: number) {
    if (!isSpecializationStateConflictError(error) || principalKey() !== key || activeMutation?.id !== mutationId) return
    invalidateLoads()
    try {
      await load(true)
    } catch {
      // Preserve the original mutation error; load() keeps the refresh failure in loadError.
    }
  }

  async function mutate(
    send: (token: string, current: SpecializationSummary) => Promise<SpecializationMutationResult>
  ): Promise<SpecializationMutationResult> {
    const key = principalKey()
    const token = gameStore.token
    if (!key || !token) throw new Error("specialization state unavailable")
    const assertNotMutating = () => {
      if (activeMutation?.key === key) {
        throw new APIClientError("duplicate request", { code: "REQUEST_DUPLICATE" })
      }
    }
    assertNotMutating()
    const current = await load()
    if (!current || loadedPrincipalKey !== key || principalKey() !== key) throw staleMutationError()
    assertNotMutating()
    const mutationId = ++nextMutationId
    activeMutation = { id: mutationId, key }
    mutating.value = true
    try {
      const result = await send(token, current)
      assertMutationPrincipal(key, mutationId)
      if (result.committed) {
        // Reads started before the commit cannot supply its follow-up snapshot.
        invalidateLoads()
        summary.value = {
          ...current,
          status: result.status,
          effectiveSlots: result.effectiveSlots,
          slotCapacities: result.slotCapacities,
          characterAttributes: result.characterAttributes ?? current.characterAttributes,
          rulesetReset: undefined
        }
        try {
          await load(true)
        } catch {
          // The write succeeded. Keep its receipt and expose the GET failure via loadError.
        }
        assertMutationPrincipal(key, mutationId)
      }
      return result
    } catch (error: unknown) {
      await refreshAfterMutationConflict(error, key, mutationId)
      throw error
    } finally {
      if (activeMutation?.id === mutationId) {
        activeMutation = null
        mutating.value = false
      }
    }
  }

  function allocate(nodeId: string, impactDigest = "", idempotencyKey = ""): Promise<SpecializationMutationResult> {
    return mutate((token, current) => allocatePlayerSpecialization(
      token,
      current.tree.id,
      nodeId,
      {
        expectedRevision: current.status.revision,
        expectedRulesetVersion: current.tree.rulesetVersion,
        ...(impactDigest ? { impactDigest } : {})
      },
      idempotencyKey || undefined
    ))
  }

  function reset(impactDigest = "", idempotencyKey = ""): Promise<SpecializationMutationResult> {
    return mutate((token, current) => resetPlayerSpecialization(
      token,
      current.tree.id,
      {
        expectedRevision: current.status.revision,
        expectedRulesetVersion: current.tree.rulesetVersion,
        ...(impactDigest ? { impactDigest } : {})
      },
      idempotencyKey || undefined
    ))
  }

  function clear() {
    generation += 1
    activeLoad = null
    activeMutation = null
    summary.value = null
    loadError.value = ""
    loading.value = false
    mutating.value = false
    loadedPrincipalKey = ""
  }

  return {
    summary,
    loading,
    mutating,
    loadError,
    enabledBattleSlotIds,
    load,
    allocate,
    reset,
    clear
  }
})
