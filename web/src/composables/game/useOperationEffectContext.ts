import { computed } from "vue"
import { useOperationEffects } from "./useOperationEffects"
import { operationEffectEligibility, type OperationEffectContext } from "@services/operation-effect-context"

export function useOperationEffectContext(context: () => OperationEffectContext) {
  const cache = useOperationEffects()
  const rows = computed(() => {
    const current = context()
    return cache.effectsFor(current).map(effect => ({ effect, eligibility: operationEffectEligibility(effect, current) }))
  })
  return {
    initialized: cache.initialized,
    stale: cache.stale,
    nowMs: cache.serverNowMs,
    applicable: computed(() => rows.value.filter(row => row.eligibility.applicable).map(row => row.effect)),
    unavailable: computed(() => rows.value.flatMap(row => "reason" in row.eligibility ? [{ effect: row.effect, reason: row.eligibility.reason }] : []))
  }
}
