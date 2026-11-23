import type { OperationEffectView } from "./operation-effects"

const sum = (effects: OperationEffectView[], kind: string, field: string, cap: number) => Math.min(cap, effects.filter(e => e.kind === kind).reduce((n, e) => {
  const v = Number(e.params[field]); return n + (Number.isFinite(v) && v > 0 ? v : 0)
}, 0))

export function transmuteFoodModifiers(effects: OperationEffectView[]) {
  return {
    bonusRate: sum(effects, "alchemy.transmute_gold_bonus", "bonusRate", .10),
    quickUses: effects.filter(e => e.kind === "alchemy.transmute_duration").reduce((n, e) => n + Math.max(0, e.remainingCharges || 0), 0)
  }
}

export function enchantFoodProbabilities(weights: Record<string, number>, effects: OperationEffectView[]) {
  const bonus = sum(effects, "enchantment.high_rarity_weight", "weightBonusRate", .25)
  const entries = Object.entries(weights).filter(([, v]) => Number.isFinite(v) && v > 0)
  const total = entries.reduce((n, [, v]) => n+v, 0)
  const adjusted = entries.map(([id, v]) => [id, v*(id === "epic" || id === "legendary" ? 1+bonus : 1)] as const)
  const afterTotal = adjusted.reduce((n, [, v]) => n+v, 0)
  return entries.map(([rarityId, v], i) => ({ rarityId, before: v/total, after: adjusted[i][1]/afterTotal }))
}

export function transmuteFoodBonus(normal: number, rate: number) {
  return Number.isSafeInteger(normal) && normal > 0 && Number.isFinite(rate) && rate > 0 && rate <= .10
    ? Math.floor(normal/10000)*Math.round(rate*10000)+Math.floor((normal%10000)*Math.round(rate*10000)/10000) : 0
}

export function transmuteFoodDuration(count: number, normalSeconds: number, quickUses: number) {
  const quick = Math.min(Math.max(0, Math.floor(quickUses)), count)
  return quick + Math.max(0, count-quick)*normalSeconds
}
