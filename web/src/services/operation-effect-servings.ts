import type { OperationEffectActivationTarget, OperationEffectDefinitionView, OperationEffectView } from './operation-effects'

function sameParams(a: Record<string, unknown>, b: Record<string, unknown>): boolean {
  const keys = Object.keys(a)
  return keys.length === Object.keys(b).length && keys.every(key => Object.hasOwn(b, key) && a[key] === b[key])
}

function targetsOverlap(a: OperationEffectActivationTarget, b: OperationEffectActivationTarget): boolean {
  if (a.targetKind === 'player_and_kitties' || b.targetKind === 'player_and_kitties') return true
  if (a.targetKind === 'player' || b.targetKind === 'player') return a.targetKind === b.targetKind
  return a.targetKind === 'all_kitties' || b.targetKind === 'all_kitties' || a.targetActorId === b.targetActorId
}

export function previewOperationEffectServings(
  definition: OperationEffectDefinitionView | null | undefined,
  target: OperationEffectActivationTarget | null,
  effects: readonly OperationEffectView[],
  nowMs: number,
  stock: number,
  quantity: number
) {
  const duration = definition?.lifetime.kind === 'duration'
  const unit = Number(duration ? definition?.lifetime.durationSeconds : definition?.lifetime.charges)
  const cap = Number(duration ? definition?.stacking?.maxDurationSeconds : definition?.stacking?.maxCharges)
  let remaining = 0
  let conflict = false
  const available = !!definition?.stacking && !!target && Number.isSafeInteger(unit) && unit > 0 && Number.isSafeInteger(cap) && cap >= unit
  if (available && definition && target) {
    for (const effect of effects) {
      if (Date.parse(effect.validFrom) > nowMs) continue
      const left = effect.lifetimeKind === 'duration' ? Math.max(0, Math.ceil((Date.parse(effect.validUntil || '') - nowMs) / 1000)) : Number(effect.remainingCharges || 0)
      if (!(left > 0) || !targetsOverlap(target, effect)) continue
      const sameTarget = target.targetKind === effect.targetKind && (target.targetActorId || 0) === (effect.targetActorId || 0)
      const sameDefinition = definition.id === effect.definitionId && definition.rulesetVersion === effect.rulesetVersion
      if (sameTarget && sameDefinition && definition.lifetime.kind === effect.lifetimeKind) {
        if (!sameParams(definition.params, effect.params)) conflict = true
        else remaining += left
      } else if (definition.stacking?.key === effect.stacking?.key && (definition.stacking.exclusive || effect.stacking?.exclusive)) conflict = true
    }
  }
  const maxQuantity = available && !conflict ? Math.max(0, Math.min(10000, Math.floor(stock), Math.floor((cap - remaining) / unit))) : 0
  const valid = Number.isSafeInteger(quantity) && quantity > 0 && quantity <= maxQuantity
  const added = Number.isSafeInteger(quantity) && quantity > 0 && Number.isFinite(unit) ? quantity * unit : 0
  return { duration, available, conflict, unit, cap, remaining, added, total: remaining + added, maxQuantity, valid }
}
