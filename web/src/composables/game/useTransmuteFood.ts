import { computed, ref, watch } from "vue"
import { useGameStore } from "@stores/game"
import { apiGet } from "@services/api"
import { useOperationEffects } from "./useOperationEffects"
import { useOperationEffectContext } from "./useOperationEffectContext"
import { transmuteFoodModifiers } from "@services/cuisine-operation-modifiers"

export function useTransmuteFood(itemId: () => string) {
  const game = useGameStore()
  const effects = useOperationEffects()
  const eligible = ref(false)
  watch([itemId, () => game.token, effects.revision], async ([id, token], _, onCleanup) => {
    eligible.value = false
    let current = true
    onCleanup(() => { current = false })
    if (!id || !token) return
    try {
      const result = await apiGet<{ eligible: boolean }>(`/api/operation-effects/transmute-eligibility/${encodeURIComponent(id)}`, token)
      if (current) eligible.value = result.eligible === true
    } catch { /* Eligibility remains false until a successful authoritative read. */ }
  }, { immediate: true })
  const context = computed(() => ({ domain: "alchemy" as const, actionId: "alchemy.transmute", transmuteGoldEligible: eligible.value }))
  const resolved = useOperationEffectContext(() => context.value)
  const modifiers = computed(() => transmuteFoodModifiers(resolved.initialized.value && !resolved.stale.value ? resolved.applicable.value : []))
  return { context, modifiers }
}
