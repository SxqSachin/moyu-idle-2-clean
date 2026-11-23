import type { OperationEffectView } from "./operation-effects"

export function conversionFoodReduction(effects: readonly Pick<OperationEffectView, "kind" | "params">[]) {
  return effects.reduce((best, effect) => {
    const rate = Number(effect.params.lossReductionRate)
    return effect.kind === "alchemy.conversion_loss_reduction" && Number.isFinite(rate) && rate > 0 && rate <= .10 ? Math.max(best, rate) : best
  }, 0)
}

export function conversionFoodFailureChance(base: number, rate: number) {
  return Number.isFinite(rate) && rate > 0 && rate <= .10 ? Math.max(0, Math.min(1, base)) * (1 - rate) : base
}
