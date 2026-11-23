export type QueueCardDisplayEnhance = {
  equipmentInstanceId: number
  templateItemId: string
  targetEnhanceLevel: number
  useProtect: boolean
  protectStartLevel?: number
  protectItemId?: string
}

export type QueueCardDisplayBuilding = {
  buildingId: string
  fromLevel: number
  toLevel: number
  durationS: number
}

export type QueueCardDisplayAlchemy = {
  subKind: "convert" | "bench" | "transmute" | "smelt" | string
  carrierInstanceId?: number
  carrierItemId?: string
  materials?: Record<string, number>
  comboRecipeId?: string
  mediumItemId?: string
  sourceItemId?: string
  sourceCount?: number
  groupId?: string
  itemId?: string
  instanceId?: number
  totalCount?: number
  fatherInstanceId?: number
  fatherAffixIndex?: number
  motherInstanceId?: number
  motherAffixIndex?: number
  geneChoiceCount?: number
  inheritGeneCount?: number
  suppressGeneCount?: number
}

export type QueueCardFishingSupplyCounts = {
  mainLine: number
  leader: number
  hook?: number
  floatOrSinker?: number
  lure?: number
  bait?: number
}

export type QueueCardDisplayFishing = {
  tripId: string
  waterId: string
  spotId: string
  loadoutName: string
  method: string
  strategy: string
  phase: string
  lineState?: string
  plannedDurationSeconds: number
  startedAtMs?: number
  plannedEndAtMs?: number
  phaseEndsAtMs?: number
  landed: number
  released: number
  totalWeightKg: number
  initialSupplies: QueueCardFishingSupplyCounts
  remainingSupplies: QueueCardFishingSupplyCounts
}

export type QueueCardDisplay = {
  kind: string
  effectiveDurationMs?: number
  enhance?: QueueCardDisplayEnhance
  building?: QueueCardDisplayBuilding
  alchemy?: QueueCardDisplayAlchemy
  fishing?: QueueCardDisplayFishing
}

export type QueueCard = {
  id: number
  actorType: "player" | "kitty"
  actorId: number
  actionId: string
  label: string
  iconType: string
  iconId: string
  iconFallback: string
  status: "pending" | "running" | "done" | "failed"
  statusLabelKey?: string
  doneCount: number
  totalCount: number
  remainingMs: number
  startAt: number
  progressPct: number
  runStartAtMs: number
  runEndsAtMs: number
  progressToken: string
  etaUnknown?: boolean
  display?: QueueCardDisplay
}
