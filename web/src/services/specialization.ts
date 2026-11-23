import { APIClientError, apiGet, apiPost, createIdempotencyKey } from "@services/api"

export const PLAYER_COMBAT_SPECIALIZATION_TREE_ID = "player_combat_specialization"

const SPECIALIZATION_STATE_CONFLICT_CODES = new Set([
  "SPECIALIZATION_REVISION_CONFLICT",
  "SPECIALIZATION_RULESET_CONFLICT",
  "SPECIALIZATION_IMPACT_CONFLICT"
])

export function isSpecializationStateConflictError(error: unknown): error is APIClientError {
  return error instanceof APIClientError
    && Boolean(error.code)
    && SPECIALIZATION_STATE_CONFLICT_CODES.has(error.code || "")
}

export const PLAYER_BASE_BATTLE_SLOT_IDS = [
  "head",
  "body",
  "cloak",
  "left_hand",
  "right_hand",
  "arm",
  "legs",
  "feet",
  "accessory_1",
  "accessory_2",
  "accessory_3",
  "accessory_4"
] as const

export const PLAYER_WORK_SLOT_IDS = [
  ...PLAYER_BASE_BATTLE_SLOT_IDS
] as const

export const PLAYER_FISHING_SLOT_IDS = [
  "fishing_rod",
  "fishing_reel"
] as const

export const PLAYER_SPECIALIZATION_BATTLE_SLOT_IDS = [
  "cloak",
  "head",
  "head_2",
  "left_hand",
  "body",
  "body_2",
  "right_hand",
  "right_hand_2",
  "arm",
  "arm_2",
  "legs",
  "feet",
  "feet_2",
  "accessory_1",
  "accessory_2",
  "accessory_3",
  "accessory_4",
  "accessory_5",
  "accessory_6",
  "accessory_7",
  "accessory_8"
] as const

export function filterPlayerBattleEquipSlots(
  allowedSlots: readonly string[],
  effectiveSlots?: readonly string[] | null
): string[] {
  const enabled = new Set(effectiveSlots ?? PLAYER_BASE_BATTLE_SLOT_IDS)
  return allowedSlots.filter((slot) => enabled.has(slot))
}

export type SpecializationSchoolId = "strength" | "dexterity" | "intelligence" | "perception" | "stamina"

export type SpecializationRequirement = {
  kind: "node_rank" | "attribute_gte" | "pure_school_points_gte" | "school_points_gte"
  nodeId?: string
  minRank?: number
  attributeKey?: string
  value?: number
  schoolId?: SpecializationSchoolId
  points?: number
}

export type SpecializationGrant = {
  id: string
  kind: "stat_modifier" | "battle_hook" | "effect_mutation" | "equipment_slot_capacity"
  scope?: "character" | "battle"
  modifiers?: Array<{
    attributeKey: string
    operation: "add" | "multiply"
    value: number
  }>
  equipType?: "battle"
  deltas?: Record<string, number>
  battleHooks?: Array<Record<string, unknown>>
  effectMutations?: Array<Record<string, unknown>>
}

export type SpecializationLevel = {
  level: number
  cost: number
  grants: SpecializationGrant[]
  /** Optional explicit glossary terms used by this rank's player copy. */
  keywordIds?: string[]
}

export type SpecializationEffectSummaryToken = {
  text?: string
  keywordId?: string
}

export type SpecializationEffectSummaryLine = string | {
  text?: string
  tokens?: Array<string | SpecializationEffectSummaryToken>
}

export type SpecializationKeywordDefinition = {
  id: string
  mechanicKind?: string
  runtimeIds?: string[]
  nameKey?: string
  summaryKey?: string
  mechanicDetailsKey?: string
  sourceHintKey?: string
  descriptionKey?: string
  triggerKey?: string
  sourceKey?: string
  sourceNodeIds?: string[]
  aliases?: string[]
}

export type SpecializationLocalizedKeyword = {
  name?: string
  label?: string
  playerSummary?: string
  mechanicDetails?: string
  sourceHint?: string
  description?: string
  effect?: string
  trigger?: string
  source?: string
  acquisition?: string
  aliases?: string[]
}

export type SpecializationKeywordCollection =
  | SpecializationKeywordDefinition[]
  | Record<string, Omit<SpecializationKeywordDefinition, "id"> & { id?: string }>

export type SpecializationNode = {
  id: string
  kind: "root" | "technique" | "terminal" | "confluence"
  schoolIds: SpecializationSchoolId[]
  nameKey: string
  descKey: string
  /** Stable glossary ids. Player UI must prefer these over copy matching. */
  keywordIds?: string[]
  exclusiveGroupId?: string
  requirements?: {
    all?: SpecializationRequirement[]
    anyOf?: SpecializationRequirement[][]
  }
  levels: SpecializationLevel[]
  ui: {
    column?: number
    row?: number
    branch?: string
    accent?: string
    x?: number
    y?: number
  }
}

export type SpecializationSchool = {
  id: SpecializationSchoolId
  nameKey: string
  descKey: string
  attributeKey: string
  ui?: {
    column?: number
    color?: string
  }
}

export type SpecializationTree = {
  id: string
  schemaVersion: number
  rulesetVersion: string
  balanceRevision: number
  ownerType?: "player"
  nameKey: string
  descKey: string
  i18n?: Record<string, {
    name: string
    desc: string
    schools: Record<string, { name: string; desc: string }>
    nodes: Record<string, {
      name: string
      desc: string
      effectSummary?: SpecializationEffectSummaryLine[]
      levelEffectSummary?: SpecializationEffectSummaryLine[][]
    }>
    keywords?: Record<string, SpecializationLocalizedKeyword>
  }>
  /** Canonical stable keyword definitions for the current specialization ruleset. */
  keywords?: SpecializationKeywordCollection
  initialPoints?: number
  maxPoints?: number
  maxActiveRootNodes: number
  unlock?: { battleLevel: number }
  pointBands?: Array<{
    id: string
    fromLevel: number
    toLevel: number
    everyLevels: number
    pointsPerGrant: number
  }>
  schools: SpecializationSchool[]
  nodes: SpecializationNode[]
  ui?: {
    layout?: string
    schoolOrder?: SpecializationSchoolId[]
  }
}

export type SpecializationAllocation = {
  nodeId: string
  rank: number
  paidCost: number
}

export type SpecializationNodeState = {
  nodeId: string
  rank: number
  maxRank: number
  nextCost: number
  canAllocate: boolean
  unmetReasonCodes: string[]
}

export type SpecializationPointSource = {
  sourceId: string
  kind: "unlock_grant" | "level_milestone" | "reward"
  labelKey:
    | "game.specialization.pointSource.unlockGrant"
    | "game.specialization.pointSource.levelMilestone"
    | "game.specialization.pointSource.reward"
  bandId?: string
  requiredBattleLevel: number | null
  points: number
  countedPoints: number
  earned: boolean
}

export type SpecializationPointMilestoneBand = {
  bandId: string
  labelKey: "game.specialization.pointSource.levelMilestone"
  fromBattleLevel: number
  toBattleLevel: number
  everyLevels: number
  pointsPerGrant: number
  earnedGrantCount: number
  earnedPoints: number
  countedPoints: number
  firstEarnedBattleLevel: number | null
  lastEarnedBattleLevel: number | null
  nextMilestoneBattleLevel: number | null
}

export type SpecializationPointBreakdown = {
  unlockGrantPoints: number
  levelMilestonePoints: number
  rewardPoints: number
  countedRewardPoints: number
  /** Earned source points excluded from totalPoints by maxPoints. */
  cappedPoints: number
  maxPoints: number
  atCap: boolean
  nextPointBattleLevel: number | null
  nextPointPoints: number
  milestoneBands: SpecializationPointMilestoneBand[]
  sources: SpecializationPointSource[]
}

export type SpecializationStatus = {
  treeId: string
  rulesetVersion: string
  balanceRevision: number
  revision: number
  battleLevel: number
  formulaPoints: number
  bonusPoints: number
  totalPoints: number
  spentPoints: number
  availablePoints: number
  pointBreakdown: SpecializationPointBreakdown
  allocations: SpecializationAllocation[]
  nodeStates?: SpecializationNodeState[]
  resetCost: {
    itemId: string
    count: number
  }
}

export type EffectiveEquipmentSlot = {
  slot: string
  family: string
  ordinal: number
  enabled: boolean
  accepts: string[]
}

export type SpecializationSummary = {
  tree: SpecializationTree
  status: SpecializationStatus
  effectiveSlots: EffectiveEquipmentSlot[]
  slotCapacities: Record<string, number>
  characterAttributes?: Record<string, number>
  rulesetReset?: SpecializationRulesetResetNotice
}

export type SpecializationImpactItem = {
  instanceId: number
  templateItemId: string
  fromSlots: string[]
  reason: string
}

export type SpecializationImpact = {
  digest: string
  disabledSlots: string[]
  enabledSlots: string[]
  autoUnequipped: SpecializationImpactItem[]
  disabledSkills?: Array<{
    skillId: string
    reason: string
  }>
}

export type SpecializationRulesetResetNotice = {
  reason: "ruleset_changed" | "allocation_invalid"
  previousRulesetVersion: string
  currentRulesetVersion: string
  previousRevision: number
  currentRevision: number
  removedAllocations: SpecializationAllocation[]
  impact: SpecializationImpact
}

export type SpecializationMutationResult = {
  committed: boolean
  requiresConfirmation: boolean
  status: SpecializationStatus
  effectiveSlots: EffectiveEquipmentSlot[]
  slotCapacities: Record<string, number>
  characterAttributes?: Record<string, number>
  impact: SpecializationImpact
}

export type SpecializationDisabledSkillImpact = {
  skillId: string
  reasons: string[]
}

export function normalizeDisabledSkillImpacts(impact: SpecializationImpact | null | undefined): SpecializationDisabledSkillImpact[] {
  const bySkill = new Map<string, Set<string>>()
  for (const entry of impact?.disabledSkills || []) {
    const skillId = String(entry.skillId || "").trim()
    const reason = String(entry.reason || "").trim()
    if (!skillId) continue
    const reasons = bySkill.get(skillId) || new Set<string>()
    if (reason) reasons.add(reason)
    bySkill.set(skillId, reasons)
  }
  return Array.from(bySkill, ([skillId, reasons]) => ({ skillId, reasons: Array.from(reasons) }))
}

export type SpecializationWriteVersion = {
  expectedRevision: number
  expectedRulesetVersion: string
  impactDigest?: string
}

export function fetchPlayerSpecialization(token: string, treeId = PLAYER_COMBAT_SPECIALIZATION_TREE_ID) {
  return apiGet<SpecializationSummary>(`/api/player/specializations/${encodeURIComponent(treeId)}`, token)
}

export function allocatePlayerSpecialization(
  token: string,
  treeId: string,
  nodeId: string,
  version: SpecializationWriteVersion,
  idempotencyKey = createIdempotencyKey("specialization-allocate")
): Promise<SpecializationMutationResult> {
  return postSpecializationMutation(
    `/api/player/specializations/${encodeURIComponent(treeId)}/allocate`,
    token,
    {
      nodeId,
      expectedRevision: version.expectedRevision,
      expectedRulesetVersion: version.expectedRulesetVersion,
      ...(version.impactDigest ? { impactDigest: version.impactDigest } : {})
    },
    { "X-Idempotency-Key": idempotencyKey }
  )
}

export function resetPlayerSpecialization(
  token: string,
  treeId: string,
  version: SpecializationWriteVersion,
  idempotencyKey = createIdempotencyKey("specialization-reset")
): Promise<SpecializationMutationResult> {
  return postSpecializationMutation(
    `/api/player/specializations/${encodeURIComponent(treeId)}/reset`,
    token,
    {
      expectedRevision: version.expectedRevision,
      expectedRulesetVersion: version.expectedRulesetVersion,
      ...(version.impactDigest ? { impactDigest: version.impactDigest } : {})
    },
    { "X-Idempotency-Key": idempotencyKey }
  )
}

async function postSpecializationMutation(
  path: string,
  token: string,
  body: Record<string, unknown>,
  headers: Record<string, string>
): Promise<SpecializationMutationResult> {
  try {
    return await apiPost<SpecializationMutationResult>(path, token, body, headers)
  } catch (error: unknown) {
    if (
      error instanceof APIClientError
      && error.code === "SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED"
      && isSpecializationMutationResult(error.data)
    ) {
      return error.data
    }
    throw error
  }
}

function isSpecializationMutationResult(value: unknown): value is SpecializationMutationResult {
  if (!value || typeof value !== "object") return false
  const candidate = value as Partial<SpecializationMutationResult>
  return candidate.committed === false
    && candidate.requiresConfirmation === true
    && Boolean(candidate.status)
    && Boolean(candidate.impact?.digest)
}
