import { APIClientError } from "@services/api"

export type ArenaPosition = "vanguard" | "midguard" | "rearguard"
export type ArenaChallengeRole = "all" | "attack" | "defense"
export type ArenaChallengeStatus = "queued" | "running" | "attacker_win" | "defender_win" | "failed"

export type ArenaKittySelection = {
  kittyId: number
  position: ArenaPosition
}

export type ArenaLineupSelection = {
  playerPosition: ArenaPosition
  kitties: ArenaKittySelection[]
  formatId: string
}

export type ArenaFormat = {
  id: string
  baseFormatId?: string
  nameKey: string
  shortDescKey: string
  descKey: string
  icon: string
  standard: boolean
  schemaVersion?: number
  rulesetVersion?: string
  balanceRevision?: number
  definitionDigest?: string
  rules?: ArenaFormatRule[]
}

export type ArenaFormatRule = {
  event: "lineup_snapshot" | "battle_start" | "after_summon" | string
  effectKind: "warmup_buff" | "matchup_hp_buff" | "specialization_policy" | "turn_barrier" | "damage" | string
  targetScope: "player_units" | "eligible_side_units" | "formal_units_by_side" | "all_units" | "all_alive_units" | string
  includeSummons: boolean
  buffId?: string
  payloadKey?: string
  warmup?: {
    levelSource: "player_battle_level" | string
    higherSideGapExceeds: number
    alwaysAboveLevel: number
    normalActions: number
    startMultiplier: number
    recoveryPerNormalAction: number
    maximumMultiplier: number
    outputs: string[]
  }
  matchupHp?: {
    levelSource: "player_battle_level" | string
    bothAtLeastLevel: number
    bothSidesFlatBonus: number
    lowerSideBonusPerLevel: number
  }
  specializationEnabled?: boolean
  charges?: number
  damageType?: "physical" | "magical" | string
  skipFinalDamageModifiers: boolean
  dispatchHooks: boolean
  simultaneous: boolean
  amount?: {
    basis: "target_max_hp" | string
    multiplier: number
    flat: number
    minimum: number
    maximum?: number
  }
}

export type ArenaLineupUnit = {
  unitType: "player" | "kitty"
  kittyId?: number
  name: string
  position: ArenaPosition
  battleLevel: number
  maxHp: number
  skillIds: string[]
  equipmentDigest: string[]
}

export type ArenaDefense = {
  userId: number
  revision: number
  active: boolean
  battleLevel: number
  selection: ArenaLineupSelection
  format: ArenaFormat
  lineup: ArenaLineupUnit[]
  lineupDigest: string
  bountyTokens: number
  successfulDefenses: number
  challengeCount: number
  publishedAt: string
  updatedAt: string
  eligibilityPolicyVersion?: string
  eligibilityPolicyCurrent?: boolean
}

export type ArenaAlchemyGateViolation = {
  unitType: "player" | "kitty" | string
  unitId: number
  unitName: string
  equipmentInstanceId: number
  templateItemId: string
  equipmentName?: string
  equippedSlot?: string
  affixSlotIndex: number
  alchemyRulesetVersion?: string
  attribute: string
  actualValue: number
  allowedValue: number
  reason: string
}

export type ArenaAlchemyGatePayload = {
  policyVersion: string
  requiredAlchemyRulesetVersion: string
  violations: ArenaAlchemyGateViolation[]
}

export function arenaAlchemyGatePayloadFromError(error: unknown): ArenaAlchemyGatePayload | null {
  if (!(error instanceof APIClientError) || error.code !== "ARENA_ALCHEMY_EQUIPMENT_RESTRICTED") return null
  if (!error.data || typeof error.data !== "object") return null
  const raw = error.data as Record<string, unknown>
  if (!Array.isArray(raw.violations)) return null
  const violations = raw.violations.flatMap((entry): ArenaAlchemyGateViolation[] => {
    if (!entry || typeof entry !== "object") return []
    const row = entry as Record<string, unknown>
    const actualValue = Number(row.actualValue)
    const allowedValue = Number(row.allowedValue)
    const equipmentInstanceId = Number(row.equipmentInstanceId)
    if (!Number.isFinite(actualValue) || !Number.isFinite(allowedValue) || !Number.isFinite(equipmentInstanceId)) return []
    return [{
      unitType: String(row.unitType || ""),
      unitId: Number(row.unitId || 0),
      unitName: String(row.unitName || ""),
      equipmentInstanceId,
      templateItemId: String(row.templateItemId || ""),
      equipmentName: String(row.equipmentName || "") || undefined,
      equippedSlot: String(row.equippedSlot || "") || undefined,
      affixSlotIndex: Number(row.affixSlotIndex || 0),
      alchemyRulesetVersion: String(row.alchemyRulesetVersion || "") || undefined,
      attribute: String(row.attribute || ""),
      actualValue,
      allowedValue,
      reason: String(row.reason || "")
    }]
  })
  if (violations.length === 0) return null
  return {
    policyVersion: String(raw.policyVersion || ""),
    requiredAlchemyRulesetVersion: String(raw.requiredAlchemyRulesetVersion || ""),
    violations
  }
}

export type ArenaPlayerState = {
  userId: number
  rating: number
  tokens: number
  attackWins: number
  attackLosses: number
  defenseWins: number
  defenseLosses: number
  updatedAt: string
}

export type ArenaKitty = {
  id: number
  name: string
  level: number
  status: string
  positionLocked: string
  positionLockedByPersonalityId: string
  positionLockedByPersonalityKey: string
}

export type ArenaRules = {
  historyKeepCount: number
  attackerWinTokens: number
  attackerLossTokens: number
  defenderWinTokens: number
  bountyPerDefenseWin: number
  bountyCap: number
}

export type ArenaChallengeEntries = {
  serverTime: string
  remaining: number
  capacity: number
  recoverySeconds: number
  nextRecoveryAt?: string | null
  opponentCooldownSeconds: number
}

export type ArenaOpponentCooldown = {
  serverTime: string
  seconds: number
}

export type ArenaRecommendationRefresh = {
  serverTime: string
  availableAt?: string | null
  cooldownSeconds: number
}

export type ArenaOverview = {
  battleKind: "arena"
  state: ArenaPlayerState
  defense: ArenaDefense | null
  kitties: ArenaKitty[]
  formats?: ArenaFormat[]
  challengeEntries: ArenaChallengeEntries
  rules: ArenaRules
}

export type ArenaTarget = ArenaDefense & {
  username: string
  rating: number
  estimatedRewardTokens: number
  ratingEligible: boolean
  recommendationTier?: "nearby" | "relaxed" | "random" | ""
  recommendationToken?: string
  opponentCooldownUntil?: string | null
}

export type ArenaTargetsPayload = {
  battleKind: "arena"
  targets: ArenaTarget[]
  opponentCooldown: ArenaOpponentCooldown
  recommendationRefresh?: ArenaRecommendationRefresh
  offset?: number
  limit?: number
  nextOffset?: number
  hasMore?: boolean
}

export type ArenaRecommendationRefreshPayload = {
  battleKind: "arena"
  recommendationRefresh: ArenaRecommendationRefresh
}

export type ArenaLeaderboardEntry = {
  rank: number
  userId: number
  username: string
  rating: number
  attackWins: number
  attackLosses: number
  defenseWins: number
  defenseLosses: number
  activeDefense: boolean
  battleLevel: number
}

export type ArenaLeaderboardPayload = {
  battleKind: "arena"
  entries: ArenaLeaderboardEntry[]
  self: ArenaLeaderboardEntry | null
  offset: number
  limit: number
  total: number
  hasMore: boolean
}

export type ArenaChallenge = {
  id: number
  battleKind: "arena"
  attackerId: number
  defenderId: number
  opponentId?: number
  opponentName?: string
  role?: "attack" | "defense"
  defenseRevision: number
  seed: number
  status: ArenaChallengeStatus
  outcome: string
  attempts: number
  ratingEligible: boolean
  attackerRewardTokens: number
  defenderRewardTokens: number
  bountyRewardTokens: number
  attackerRatingDelta: number
  defenderRatingDelta: number
  createdAt: string
  startedAt?: string
  completedAt?: string
  format: ArenaFormat
}

export type ArenaChallengeCreatePayload = {
  battleKind: "arena"
  challenge: ArenaChallenge
  challengeEntries: ArenaChallengeEntries
  opponentCooldownUntil?: string | null
}

export type ArenaHistoryPayload = {
  battleKind: "arena"
  challenges: ArenaChallenge[]
  historyKeepCount: number
}

export type ArenaShopOffer = {
  id: string
  itemId: string
  quantity: number
  cost: number
}

export type ArenaShopPayload = {
  battleKind: "arena"
  tokens: number
  offers: ArenaShopOffer[]
}

export type ArenaTimelineEntry = {
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

export type ArenaReplayPayload = {
  challengeId: number
  battleKind: "arena"
  seed: number
  status: ArenaChallengeStatus
  outcome: string
  initialState: Record<string, unknown> & {
    players?: Array<Record<string, unknown>>
    enemies?: Array<Record<string, unknown>>
  }
  timeline: ArenaTimelineEntry[]
  finalResult: Record<string, unknown>
}

export type ArenaReplayShare = {
  token: string
  challengeId: number
  createdBy: number
  createdByName: string
  outcome: string
  createdAt: string
}

export type ArenaReplaySharePayload = {
  battleKind: "arena"
  share: ArenaReplayShare
}

export type ArenaSharedReplayPayload = {
  battleKind: "arena"
  share: ArenaReplayShare
  payload: ArenaReplayPayload
}

export function arenaPositionOrder(position: unknown): number {
  switch (String(position || "").trim()) {
    case "vanguard":
      return 0
    case "midguard":
      return 1
    case "rearguard":
      return 2
    default:
      return 3
  }
}

export function isArenaChallengeTerminal(status: unknown): boolean {
  return status === "attacker_win" || status === "defender_win" || status === "failed"
}

export function arenaCooldownSecondsRemaining(until: unknown, nowMs = Date.now()): number {
  const untilMs = Date.parse(String(until || ""))
  if (!Number.isFinite(untilMs) || !Number.isFinite(nowMs)) return 0
  return Math.max(0, Math.ceil((untilMs - nowMs) / 1000))
}

export type ArenaChallengeEntryProjection = {
  remaining: number
  capacity: number
  secondsToNext: number
  nextRecoveryAtMs: number | null
}

export function projectArenaChallengeEntries(entries: ArenaChallengeEntries | null | undefined, nowMs = Date.now()): ArenaChallengeEntryProjection {
  const currentMs = Number.isFinite(nowMs) ? nowMs : 0
  const capacity = Math.max(0, Math.floor(Number(entries?.capacity) || 0))
  let remaining = Math.min(capacity, Math.max(0, Math.floor(Number(entries?.remaining) || 0)))
  const recoveryMs = Math.max(0, Math.floor(Number(entries?.recoverySeconds) || 0)) * 1000
  let nextRecoveryAtMs = Date.parse(String(entries?.nextRecoveryAt || ""))
  if (!Number.isFinite(nextRecoveryAtMs) || recoveryMs <= 0 || remaining >= capacity) {
    nextRecoveryAtMs = Number.NaN
  } else if (currentMs >= nextRecoveryAtMs) {
    const recovered = 1 + Math.floor((currentMs - nextRecoveryAtMs) / recoveryMs)
    remaining = Math.min(capacity, remaining + recovered)
    nextRecoveryAtMs = remaining >= capacity ? Number.NaN : nextRecoveryAtMs + recovered * recoveryMs
  }
  const nextAt = Number.isFinite(nextRecoveryAtMs) ? nextRecoveryAtMs : null
  return {
    remaining,
    capacity,
    secondsToNext: nextAt === null ? 0 : Math.max(0, Math.ceil((nextAt - currentMs) / 1000)),
    nextRecoveryAtMs: nextAt
  }
}

export function formatArenaCooldown(totalSeconds: unknown): string {
  const seconds = Math.max(0, Math.floor(Number(totalSeconds) || 0))
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainder = seconds % 60
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`
  }
  return `${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`
}

export function activeArenaKittySelections(selection: ArenaLineupSelection): ArenaKittySelection[] {
  return selection.kitties
    .filter((kitty) => Number.isInteger(kitty.kittyId) && kitty.kittyId > 0)
    .map((kitty) => ({ ...kitty }))
}

export function isArenaLineupSelectionValid(selection: ArenaLineupSelection): boolean {
  if (!selection || !/^[a-z0-9][a-z0-9_:-]{0,63}$/.test(String(selection.formatId || "").trim().toLowerCase())) return false
  if (arenaPositionOrder(selection.playerPosition) > 2 || selection.kitties.length > 2) return false
  const kitties = activeArenaKittySelections(selection)
  if (kitties.length !== selection.kitties.filter((kitty) => kitty.kittyId !== 0).length) return false
  if (new Set(kitties.map((kitty) => kitty.kittyId)).size !== kitties.length) return false
  if (kitties.some((kitty) => arenaPositionOrder(kitty.position) > 2)) return false
  return true
}
