export type CombatTrialPosition = "vanguard" | "midguard" | "rearguard"
export type CombatTrialMetricID = string
export type CombatTrialRunStatus = "queued" | "running" | "completed" | "failed"

export type CombatTrialKittySelection = {
  kittyId: number
  position: CombatTrialPosition
}

export type CombatTrialLineupSelection = {
  playerPosition: CombatTrialPosition
  kitties: CombatTrialKittySelection[]
  formatId: string
}

export type CombatTrialKitty = {
  id: number
  name: string
  level: number
  status: string
  positionLocked: string
  positionLockedByPersonalityId: string
  positionLockedByPersonalityKey: string
}

export type CombatTrialMetric = {
  id: string
  schemaVersion: number
  nameKey: string
  shortNameKey: string
  descKey: string
  eventType: string
  aggregation: "sum" | "max" | "final" | string
  valueSource?: string
  sourceScope: string
  targetScope: string
  ranked: boolean
  rankDirection: "asc" | "desc" | string
  eligibleEndReasons?: string[]
  valueFormat?: "number" | "integer" | string
  sortOrder: number
  enabled: boolean
}

export type CombatTrialLimit = {
  kind: "player_main_actions" | string
  value: number
}

export type CombatTrialLineupRules = {
  positionPolicy: "shared" | "distinct" | string
}

export type CombatTrialEnemyStatProfile = {
  difficultyLevel: number
  multiplier: number
  flatBonuses?: Record<string, number>
}

export type CombatTrialEncounterUnit = {
  id: string
  kind: "training_target" | "authored_enemy" | "scripted_ally" | string
  enemyId?: string
  nameKey?: string
  side?: "player" | "enemy" | string
  position: CombatTrialPosition
  behavior: "passive" | "active" | string
  healthMode: "immutable_full" | "normal" | string
  objectiveTarget: boolean
  initialHpRatio?: number
  skipTurnOrder?: boolean
  initialBuffs?: Array<{
    buffId: string
    payload?: Record<string, number>
  }>
  defenseSource?: string
  defense?: number
  maxHp?: number
  damageResist?: number
  criticalDamageResist?: number
  statOverrides?: Record<string, number>
  resolvedStats?: Record<string, number>
}

export type CombatTrialEncounterWave = {
  id: string
  nameKey?: string
  units: CombatTrialEncounterUnit[]
  enemyStatProfile?: CombatTrialEnemyStatProfile
}

export type CombatTrialEndCondition = {
  kind: string
  reason: string
  labelKey?: string
}

export type CombatTrialFormat = {
  id: string
  schemaVersion: number
  rulesetVersion: string
  balanceRevision: number
  nameKey: string
  shortDescKey: string
  descKey: string
  icon: string
  sortOrder: number
  enabled: boolean
  actionBudget: number
  defenseK?: number
  metricIds: string[]
  metrics: CombatTrialMetric[]
  lineupRules?: CombatTrialLineupRules
  limits?: CombatTrialLimit[]
  encounter?: {
    kind: string
    units?: CombatTrialEncounterUnit[]
    waves?: CombatTrialEncounterWave[]
    enemyStatProfile?: CombatTrialEnemyStatProfile
  }
  endConditions?: CombatTrialEndCondition[]
  triggers?: Array<Record<string, unknown>>
  dummy?: {
    count: number
    positions: CombatTrialPosition[]
    defenseSource: string
    defense: number
    hpMode: string
    maxHp: number
    passiveTurns: boolean
    damageResist: number
    criticalDamageResist: number
  }
  modifiers?: Array<Record<string, unknown>>
  definitionDigest: string
}

export type CombatTrialMetrics = Record<string, number>

export type CombatTrialPersonalBest = {
  userId: number
  formatId: string
  formatDefinitionDigest: string
  metricId: string
  value: number
  metrics: CombatTrialMetrics
  endReason: string
  kittyCount: number
  achievedAt: string
  runId?: number
}

export type CombatTrialRun = {
  id: number
  battleKind: "combat_trial"
  userId: number
  formatId: string
  formatDefinitionDigest: string
  seed: number
  status: CombatTrialRunStatus
  attempts: number
  metrics: CombatTrialMetrics
  actionCount: number
  actionBudget: number
  endReason: string
  kittyCount: number
  createdAt: string
  startedAt?: string
  completedAt?: string
  format?: {
    id: string
    nameKey: string
    shortDescKey: string
    icon: string
    definitionDigest: string
    actionBudget: number
    metrics: CombatTrialMetric[]
    endConditions: CombatTrialEndCondition[]
  }
}

export type CombatTrialOverview = {
  battleKind: "combat_trial"
  selection: CombatTrialLineupSelection
  kitties: CombatTrialKitty[]
  formats: CombatTrialFormat[]
  personalBests: CombatTrialPersonalBest[]
  activeRun: CombatTrialRun | null
  rules: {
    historyKeepCount: number
    ranked: boolean
    unlimitedRuns: boolean
    maxKitties: number
  }
}

export type CombatTrialRunsPayload = {
  battleKind: "combat_trial"
  runs: CombatTrialRun[]
  historyKeepCount: number
}

export type CombatTrialLeaderboardEntry = {
  rank: number
  username: string
  metricId: string
  value: number
  metrics: CombatTrialMetrics
  kittyCount: number
  achievedAt: string
}

export type CombatTrialLeaderboardPayload = {
  battleKind: "combat_trial"
  format: CombatTrialFormat
  metricId: string
  entries: CombatTrialLeaderboardEntry[]
  self: CombatTrialLeaderboardEntry | null
  total: number
  limit: number
}

export type CombatTrialTimelineEntry = {
  seq: number
  offsetMs: number
  lane: string
  phase: string
  parentSeq?: number
  groupId?: string
  orderKey: string
  actorId?: string
  actionDurationMs: number
  payload: {
    eventType: string
    ts: number
    data: Record<string, unknown>
  }
}

export type CombatTrialReplayPayload = {
  runId: number
  battleKind: "combat_trial"
  seed: number
  status: CombatTrialRunStatus
  endReason: string
  metrics: CombatTrialMetrics
  actionCount: number
  actionBudget: number
  initialState: Record<string, unknown>
  timeline: CombatTrialTimelineEntry[]
  finalResult: Record<string, unknown>
}

export type CombatTrialReplayShare = {
  token: string
  runId: number
  createdBy: number
  createdByName: string
  endReason: string
  metrics: CombatTrialMetrics
  createdAt: string
}

export type CombatTrialReplaySharePayload = {
  battleKind: "combat_trial"
  share: CombatTrialReplayShare
}

export type CombatTrialSharedReplayPayload = {
  battleKind: "combat_trial"
  share: CombatTrialReplayShare
  payload: CombatTrialReplayPayload
}

export function normalizeCombatTrialMetrics(raw: unknown): CombatTrialMetrics {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  const metrics: CombatTrialMetrics = {}
  for (const [metricId, rawValue] of Object.entries(raw as Record<string, unknown>)) {
    const id = String(metricId || "").trim()
    const value = Number(rawValue)
    if (!id || !Number.isFinite(value) || value < 0) continue
    metrics[id] = value
  }
  return metrics
}

export function combatTrialMetricValue(metrics: unknown, metricId: CombatTrialMetricID): number {
  return normalizeCombatTrialMetrics(metrics)[String(metricId || "").trim()] || 0
}

export function isCombatTrialMetricAvailableForEndReason(
  metric: Pick<CombatTrialMetric, "eligibleEndReasons">,
  endReason: unknown
): boolean {
  const reason = String(endReason || "").trim()
  const eligible = Array.isArray(metric.eligibleEndReasons)
    ? metric.eligibleEndReasons.map((entry) => String(entry || "").trim()).filter(Boolean)
    : []
  return eligible.length === 0 || (Boolean(reason) && eligible.includes(reason))
}

export function activeCombatTrialKittySelections(selection: CombatTrialLineupSelection): CombatTrialKittySelection[] {
  return (Array.isArray(selection.kitties) ? selection.kitties : [])
    .filter((kitty) => Number.isFinite(Number(kitty.kittyId)) && Number(kitty.kittyId) > 0)
    .slice(0, 2)
    .map((kitty) => ({ kittyId: Math.trunc(Number(kitty.kittyId)), position: kitty.position }))
}

export function isCombatTrialLineupSelectionValid(
  selection: CombatTrialLineupSelection,
  kitties: CombatTrialKitty[] = [],
  format?: Pick<CombatTrialFormat, "lineupRules"> | null
): boolean {
  const positions = new Set<CombatTrialPosition>(["vanguard", "midguard", "rearguard"])
  if (!positions.has(selection.playerPosition) || !String(selection.formatId || "").trim()) return false
  const selected = activeCombatTrialKittySelections(selection)
  if (selected.length !== new Set(selected.map((kitty) => kitty.kittyId)).size) return false
  const occupied = [selection.playerPosition, ...selected.map((kitty) => kitty.position)]
  if (occupied.some((position) => !positions.has(position))) return false
  if (format?.lineupRules?.positionPolicy === "distinct" && occupied.length !== new Set(occupied).size) return false
  const kittyByID = new Map(kitties.map((kitty) => [kitty.id, kitty]))
  return selected.every((entry) => {
    const kitty = kittyByID.get(entry.kittyId)
    if (!kitty) return false
    const locked = String(kitty.positionLocked || "").trim()
    return !locked || locked === entry.position
  })
}

export function isCombatTrialRunTerminal(status: unknown): boolean {
  return status === "completed" || status === "failed"
}
