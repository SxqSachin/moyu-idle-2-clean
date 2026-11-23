import { apiGet, apiPost } from "@services/api"

export type OperationEffectDomain =
  | "life_action"
  | "enhancement"
  | "enchantment"
  | "alchemy"
  | "fishing_cast"
  | "building"

export type OperationEffectTargetKind =
  | "player"
  | "kitty"
  | "all_kitties"
  | "player_and_kitties"

export type OperationEffectLifetimeKind = "duration" | "charges"

export type OperationEffectLifetimeDefinition = {
  kind: OperationEffectLifetimeKind
  durationSeconds?: number
  charges?: number
}

export type OperationEffectSelector = {
  actionIds?: string[]
  mainTypes?: string[]
  subTypes?: string[]
}

export type OperationEffectDefinitionView = {
  id: string
  rulesetVersion: string
  domain: OperationEffectDomain
  kind: string
  nameKey: string
  descKey: string
  iconId?: string
  allowedTargets: OperationEffectTargetKind[]
  lifetime: OperationEffectLifetimeDefinition
  selector?: OperationEffectSelector
  params: Record<string, unknown>
  stacking?: OperationEffectStacking
}

export type OperationEffectStacking = {
  key: string
  mode: 'extend_duration' | 'add_charges' | 'independent'
  exclusive?: boolean
  maxDurationSeconds?: number
  maxCharges?: number
}

export type OperationEffectView = {
  grantId: number
  definitionId: string
  rulesetVersion: string
  domain: OperationEffectDomain
  kind: string
  nameKey: string
  descKey: string
  iconId?: string
  targetKind: OperationEffectTargetKind
  targetActorId?: number
  lifetimeKind: OperationEffectLifetimeKind
  validFrom: string
  validUntil?: string
  initialCharges?: number
  remainingCharges?: number
  sourceItemId?: string
  selector?: OperationEffectSelector
  params: Record<string, unknown>
  stacking?: OperationEffectStacking
}

export type OperationEffectProjection = {
  revision: number
  serverNow: string
  items: OperationEffectView[]
}

export type OperationEffectActivationTarget = {
  targetKind: OperationEffectTargetKind
  targetActorId?: number
}

export async function fetchOperationEffects(token: string) {
  return apiGet<OperationEffectProjection>("/api/operation-effects", token)
}

export async function useOperationEffectItem(
  token: string,
  instanceId: number,
  target: OperationEffectActivationTarget,
  requestId: string,
  quantity = 1
) {
  return apiPost<OperationEffectProjection>(
    "/api/items/use-operation-effect",
    token,
    {
      instanceId,
      targetKind: target.targetKind,
      targetActorId: target.targetActorId || 0,
      requestId,
      quantity
    },
    { "X-Idempotency-Key": requestId }
  )
}
