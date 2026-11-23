import { computed } from "vue"
import { useOperationEffectContext } from "./useOperationEffectContext"
import { conversionFoodReduction } from "@services/conversion-food"
import type { OperationEffectContext } from "@services/operation-effect-context"

export function useConversionFood(context: () => OperationEffectContext) {
  const cache = useOperationEffectContext(context)
  return computed(() => cache.initialized.value && !cache.stale.value ? conversionFoodReduction(cache.applicable.value) : 0)
}
