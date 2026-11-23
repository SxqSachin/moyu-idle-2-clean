import type { OperationEffectActivationTarget, OperationEffectDefinitionView } from './operation-effects'

export type FoodTargetOption = OperationEffectActivationTarget & { key: string; label: string }
type Translate = (key: string, params?: Record<string, string | number>) => string

export function operationEffectFoodTargets(
  definition: Pick<OperationEffectDefinitionView, 'allowedTargets' | 'lifetime'> | null | undefined,
  kitties: readonly { id: number; name?: string }[],
  t: Translate
): FoodTargetOption[] {
  const options: FoodTargetOption[] = []
  for (const kind of definition?.allowedTargets || []) {
    // Also exclude shared recipients from stale pre-update item-detail responses.
    if (definition?.lifetime.kind === 'duration' && kind !== 'player' && kind !== 'kitty') continue
    if (kind === 'kitty') {
      for (const kitty of kitties) {
        const id = Number(kitty.id)
        if (!Number.isSafeInteger(id) || id <= 0) continue
        options.push({ key: `kitty:${id}`, label: t('game.operationEffects.targetKitty', { name: String(kitty.name || '').trim() || `#${id}` }), targetKind: kind, targetActorId: id })
      }
    } else {
      const label = { player: 'targetPlayer', all_kitties: 'targetAllKitties', player_and_kitties: 'targetPlayerAndKitties' }[kind]
      if (label) options.push({ key: kind, label: t(`game.operationEffects.${label}`), targetKind: kind })
    }
  }
  return options
}
