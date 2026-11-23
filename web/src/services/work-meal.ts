import type { OperationEffectView } from "./operation-effects"

export const primaryWorkTypes = ["collecting", "mining", "farming_animal"] as const
export const workMealKinds = ["life_action.harvest_bonus", "life_action.focused_harvest", "life_action.patient_harvest", "life_action.work_experience"] as const
export const isPrimaryWork = (mainType?: string) => primaryWorkTypes.some(value => value === mainType)

export function workMealModifiers(effects: OperationEffectView[]) {
  const meals = effects.filter(effect => workMealKinds.some(kind => kind === effect.kind))
  const empty = { chance: 0, durationPenaltyRate: 0, durationReductionRate: 0, experienceBonusRate: 0 }
  if (meals.length !== 1) return empty
  const effect = meals[0]
  const value = (key: string) => Number(effect.params[key] || 0)
  const chance = value("chance"), durationPenaltyRate = value("durationPenaltyRate"), experienceBonusRate = value("experienceBonusRate")
  const durationReductionRate = value("durationReductionRate")
  if ([chance, durationPenaltyRate, durationReductionRate, experienceBonusRate].some(v => !Number.isFinite(v) || v < 0)) return empty
  const valid = effect.kind === "life_action.harvest_bonus" ? chance > 0 && chance <= .05 && durationReductionRate <= .05 && !durationPenaltyRate && !experienceBonusRate
    : effect.kind === "life_action.focused_harvest" ? chance > 0 && chance <= .03 && durationReductionRate > 0 && durationReductionRate <= .08 && !durationPenaltyRate && !experienceBonusRate
    : effect.kind === "life_action.patient_harvest" ? chance >= .08 && chance <= .11 && durationPenaltyRate === .04 && !experienceBonusRate && !durationReductionRate
    : experienceBonusRate > 0 && experienceBonusRate <= .15 && !chance && !durationPenaltyRate && !durationReductionRate
  return valid ? { chance, durationPenaltyRate, durationReductionRate, experienceBonusRate } : empty
}

export function workMealDuration(baseSeconds: number, rate: number, reduction = 0) {
  return rate > 0 || reduction > 0 ? Math.max(2, Math.ceil(Math.max(2, baseSeconds) * 1000 * (1 + rate - reduction)) / 1000) : baseSeconds
}

// Estimate only this actor's immediately started work; shared uses are not reserved.
export function workMealPlanSeconds(count: number, baseSeconds: number, effects: OperationEffectView[], nowMs: number) {
  const {durationPenaltyRate: rate,durationReductionRate: reduction} = workMealModifiers(effects)
  if (!rate && !reduction) return count * baseSeconds
  const meal = effects.find(effect => workMealKinds.some(kind => kind === effect.kind))!
  const slow = workMealDuration(baseSeconds, rate, reduction)
  const covered = meal.lifetimeKind === "charges" ? Math.max(0, meal.remainingCharges || 0)
    : Math.max(0, Math.ceil((Date.parse(meal.validUntil || "") - nowMs) / (slow * 1000)))
  const uses = Math.min(count, Number.isFinite(covered) ? covered : 0)
  return uses * slow + (count - uses) * baseSeconds
}
