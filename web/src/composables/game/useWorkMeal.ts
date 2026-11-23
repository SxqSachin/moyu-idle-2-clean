import { computed } from "vue"
import { useOperationEffectContext } from "./useOperationEffectContext"
import type { OperationEffectContext } from "@services/operation-effect-context"
import { workMealModifiers, workMealPlanSeconds } from "@services/work-meal"

export function useWorkMeal(context: () => OperationEffectContext) {
  const resolved = useOperationEffectContext(context)
  const effects = computed(() => resolved.initialized.value && !resolved.stale.value ? resolved.applicable.value : [])
  return {
    modifiers: computed(() => workMealModifiers(effects.value)),
    planSeconds: (count: number, baseSeconds: number) => workMealPlanSeconds(count, baseSeconds, effects.value, resolved.nowMs.value)
  }
}
