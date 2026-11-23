import { computed, onMounted, onUnmounted, ref, watch } from "vue"
import { ElMessage, ElNotification } from "element-plus"
import { useI18n } from "vue-i18n"
import { getGameDataChunk } from "@services/game-data-runtime"
import { apiGet, apiPost } from "@services/api"
import { getSkillDefinition } from "@services/skill-definition"
import { bind } from "@services/ws"
import { resolveErrorMessage } from "@services/error-messages"
import { normalizeWorldDifficultySettings, type WorldDifficultySettings } from "@services/battle-difficulty"
import { useGameStore } from "@stores/game"
import { useChatStore } from "@stores/chat"
import { useRealtimeStore } from "@stores/realtime"
import {
  battleTransitionCanComplete,
  markBattleRunPreparing,
  shouldPreserveBattleRunSnapshot,
  type BattleIntermissionWait
} from "@/services/battle-run-transition"

type BattleRoomStatus = "idle" | "preparing" | "entering" | "playing" | "recovering"
type BattleRunStatus = "compiled" | "playing" | "settling" | "ended" | "failed" | "interrupted"

type BattleRoom = {
  id: number
  ownerId: number
  dungeonId: string
  roomName: string
  autoReenterOnDefeat: boolean
  disableEntryRound: boolean
  isPublic: boolean
  isFogMode: boolean
  status: BattleRoomStatus
  difficultyLevel: number
  completedRuns: number
  consecutiveFailures: number
  memberCount?: number
  currentRunId?: number | null
  seed: number
  enteringStartedAt?: string | null
  recoveringStartedAt?: string | null
  recoveryEndsAt?: string | null
}

type BattleRoomMember = {
  roomId: number
  userId: number
  username?: string
  position: string
  isReady: boolean
}

type BattleRoomDetail = {
  room: BattleRoom
  members: BattleRoomMember[]
  units?: BattleRoomUnit[]
}

type BattleRoomUnit = {
  roomId: number
  ownerUserId: number
  username?: string
  unitType: "player" | "kitty"
  unitId: number
  kittyId?: number | null
  kittyName?: string
  position: string
  positionLockedByPersonalityId?: string
  positionLockedByPersonalityKey?: string
  isReady: boolean
}

type BattleRoomListPayload = {
  items: BattleRoom[]
  mine: BattleRoomDetail | null
}

type BattleSkill = {
  id?: string
  basePosition?: string
  level?: number
  exp?: number
  cooldown?: number
  cooldownTurns?: number
  cooldownRemain?: number
  [key: string]: unknown
}

export type BattleUnit = {
  unitId?: string | number
  unitType?: string
  side?: string
  baseSide?: string
  perspectiveSide?: string
  originSide?: string
  name?: string
  nameKey?: string
  position?: string
  hp?: number
  maxHp?: number
  mp?: number
  maxMp?: number
  alive?: boolean
  corpseConsumed?: boolean
  rewardEligible?: boolean
  stats?: Record<string, number>
  attributes?: Record<string, number>
  baseStats?: Record<string, number>
  statModifiers?: Record<string, number>
  statSources?: Record<string, Record<string, number>>
  skillIds?: string[]
  skillCooldowns?: Record<string, number>
  skillCooldownTurns?: Record<string, number>
  itemCooldowns?: Record<string, number>
  itemQuantities?: Record<string, number>
  skills?: BattleSkill[]
  battleItems?: Array<{
    itemId?: string
    quantity?: number
    cooldownTurns?: number
    cooldownRemain?: number
  }>
  basicAttack?: {
    damageType?: string
    targetDirection?: string
    targetSeekStrategy?: string
    formula?: Array<{ attrKey?: string; coefficient?: number }>
  }
  equipmentInstanceIds?: number[]
  buffs?: Array<{ buffId?: string; remainTurns?: number; payload?: Record<string, unknown>; triggered?: number }>
  summonTemplateUnitId?: string
  [key: string]: unknown
}

type BattleRunEventData = {
  actorId?: string | number
  skillName?: string
  skillId?: string
  order?: string[]
  parentSeq?: number
  batchGroup?: string
  targets?: Array<{ targetId?: string | number; hp?: number; mp?: number; alive?: boolean } & Record<string, unknown>>
  turnOwner?: { unitId?: string | number; side?: string; userId?: number; enemyId?: string }
  nodeId?: string
  steps?: Array<{
    stepId?: string
    kind?: string
    phase?: string
    buffId?: string
    buffDurationTurns?: number
    buffPayload?: Record<string, unknown>
    buffTriggered?: number
    amount?: number
    resource?: string
    crit?: boolean
    damageType?: string
    source?: { unitId?: string | number; side?: string }
    targets?: Array<{
      unitId?: string | number
      targetId?: string | number
      side?: string
      crit?: boolean
      corpseConsumed?: boolean
      hpBefore?: number
      hpAfter?: number
      mpBefore?: number
      mpAfter?: number
      aliveAfter?: boolean
      maxHpAfter?: number
      maxMpAfter?: number
      fromPosition?: string
      toPosition?: string
      position?: string
      basePosition?: string
      baseSide?: string
      perspectiveSide?: string
      hp?: number
      mp?: number
      alive?: boolean
    } & Record<string, unknown>>
    summon?: Record<string, unknown>
    skill?: {
      skillId?: string
      skillName?: string
    }
    delayMs?: number
    reason?: string
    display?: Record<string, unknown>
    meta?: Record<string, unknown>
  }>
  players?: BattleUnit[]
  enemies?: BattleUnit[]
  wave?: number
  completedBattles?: number
  preparationTurn?: boolean
  preparationOrder?: string[]
  extraTurn?: boolean
  turnKind?: string
  skillCooldowns?: Record<string, number>
  skillCooldownTurns?: Record<string, number>
  itemCooldowns?: Record<string, number>
  summary?: { actor?: Record<string, unknown> }
  combatTrial?: BattleCombatTrialState
  unitUpdates?: Array<{
    unitId?: string | number
    side?: string
    hpAfter?: number
    mpAfter?: number
    aliveAfter?: boolean
    corpseConsumed?: boolean
    maxHpAfter?: number
    maxMpAfter?: number
    position?: string
    basePosition?: string
    baseSide?: string
    perspectiveSide?: string
    stats?: Record<string, number>
    attributes?: Record<string, number>
    baseStats?: Record<string, number>
    statModifiers?: Record<string, number>
    statSources?: Record<string, Record<string, number>>
    buffs?: BattleUnit["buffs"]
    skillCooldowns?: Record<string, number>
    skillCooldownTurns?: Record<string, number>
    itemCooldowns?: Record<string, number>
  }>
  [key: string]: unknown
}

export type BattleRunEventPayload = {
  seq: number
  type?: string
  ts?: number
  data?: BattleRunEventData
}

export function filterBatchEventsAfterSnapshot<T extends { seq?: number }>(events: T[], snapshotSeq: number): T[] {
  const normalizedSnapshotSeq = Number.isFinite(snapshotSeq) ? Math.max(0, Math.trunc(snapshotSeq)) : 0
  if (!Array.isArray(events) || events.length === 0) return []
  if (normalizedSnapshotSeq <= 0) return events.slice()
  return events.filter((event) => {
    const seq = Math.trunc(Number(event?.seq || 0))
    return Number.isFinite(seq) && seq > normalizedSnapshotSeq
  })
}

export function mergeReplayEvents<T extends { seq?: number }>(current: T[], incoming: T[], limit = 120): T[] {
  const currentRows = Array.isArray(current) ? current : []
  const incomingRows = Array.isArray(incoming) ? incoming : []
  if (incomingRows.length === 0) return currentRows.slice(-limit)
  const maxCurrentSeq = currentRows.reduce((maxSeq, event) => {
    const seq = Math.trunc(Number(event?.seq || 0))
    return Number.isFinite(seq) ? Math.max(maxSeq, seq) : maxSeq
  }, 0)
  const nextRows = incomingRows.filter((event) => {
    const seq = Math.trunc(Number(event?.seq || 0))
    return Number.isFinite(seq) && seq > maxCurrentSeq
  })
  if (nextRows.length === 0) return currentRows.slice(-limit)
  return [...currentRows, ...nextRows].slice(-limit)
}

export type BattleRunStatePayload = {
  roomId: number
  wave: number
  completedBattles: number
  seq: number
  order?: string[]
  preparationOrder?: string[]
  players: BattleUnit[]
  enemies: BattleUnit[]
  battlefieldEffects?: BattlefieldEffectState[]
  arenaFormat?: BattleArenaFormatState
  combatTrialFormat?: BattleCombatTrialFormatState
  combatTrial?: BattleCombatTrialState
}

export type BattleArenaFormatState = {
  id: string
  nameKey: string
  shortDescKey: string
  descKey: string
  icon: string
  standard: boolean
  schemaVersion?: number
  rulesetVersion?: string
  balanceRevision?: number
  definitionDigest?: string
}

export type BattleCombatTrialMetricState = {
  id: string
  nameKey?: string
  shortNameKey?: string
  descKey?: string
  aggregation?: string
  eligibleEndReasons?: string[]
}

export type BattleCombatTrialFormatState = {
  id: string
  nameKey: string
  shortDescKey?: string
  descKey?: string
  icon: string
  actionBudget: number
  definitionDigest?: string
  metrics: BattleCombatTrialMetricState[]
  endConditions?: Array<{
    kind: string
    reason: string
    labelKey?: string
  }>
  encounter?: {
    kind: string
    units?: Array<{
      id: string
      kind: string
      healthMode: string
    }>
    waves?: Array<{
      id: string
      nameKey?: string
      units: Array<{
        id: string
        kind: string
        healthMode: string
      }>
    }>
  }
}

export type BattleCombatTrialState = {
  progressVersion?: number
  progressSeq?: number
  actionCount: number
  actionBudget: number
  endReason?: string
  metrics: Record<string, number>
  encounterKind?: string
  currentWave?: number
  waveCount?: number
  wavesCleared?: number
  objectiveUnitsDefeated?: number
}

export type BattlefieldEffectState = {
  id: string
  remaining: number
  progress: number
  goal: number
  windows: Record<string, number>
}

export type BattleRunBatchPayload = {
  k?: string
  m: number
  r: number
  f: number
  t: number
  s: number
  e: number
  v: Array<{
    q: number
    o: number
    e?: string
    d?: Record<string, unknown>
  }>
  n?: {
    w?: number
    c?: number
    q?: number
    ro?: string[]
    bo?: string[]
    p?: Array<Record<string, unknown>>
    e?: Array<Record<string, unknown>>
    bf?: Array<Record<string, unknown>>
    ctf?: Record<string, unknown>
    ct?: Record<string, unknown>
  }
}

type BattleRunRuntimeState = BattleRunStatePayload & {
  events?: BattleRunEventPayload[]
}

const SHADOW_CROSS_BUFF_ID = "basic_left_dagger_shadow_cross"

type BattleRunRecord = {
  id: number
  roomId: number
  wave: number
  status: BattleRunStatus
  state: BattleRunRuntimeState
  timeline?: Array<Record<string, unknown>>
}

type BattleRunPayload = {
  run: BattleRunRecord | null
}

function parsePreparationOrder(raw: unknown): string[] {
  return Array.isArray(raw)
    ? raw.map((value) => String(value || "").trim()).filter(Boolean)
    : []
}

function parseBattleOrder(raw: unknown): string[] {
  return Array.isArray(raw)
    ? raw.map((value) => String(value || "").trim()).filter(Boolean)
    : []
}

export function parseBattlefieldEffectStates(raw: unknown): BattlefieldEffectState[] {
  if (!Array.isArray(raw)) return []
  const out: BattlefieldEffectState[] = []
  for (const entry of raw) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
    const node = entry as Record<string, unknown>
    const id = String(node.id ?? node.i ?? "").trim()
    if (!id) continue
    const windows = toNumberMap(node.windows ?? node.w)
    out.push({
      id,
      remaining: Math.max(0, Math.trunc(Number(node.remaining ?? node.r) || 0)),
      progress: Math.max(0, Math.trunc(Number(node.progress ?? node.p) || 0)),
      goal: Math.max(0, Math.trunc(Number(node.goal ?? node.g) || 0)),
      windows
    })
  }
  return out
}

export function applyBattlefieldTargetResult(unit: BattleUnit, raw: unknown) {
  if (!unit || !raw || typeof raw !== "object" || Array.isArray(raw)) return unit
  const target = raw as Record<string, unknown>
  const hp = Number(target.afterHp ?? target.hp)
  if (Number.isFinite(hp)) unit.hp = hp
  if (typeof target.alive === "boolean") unit.alive = target.alive
  if (Array.isArray(target.buffs)) {
    unit.buffs = target.buffs.flatMap((item) => {
      if (!item || typeof item !== "object" || Array.isArray(item)) return []
      const row = item as Record<string, unknown>
      const buffId = String(row.buffId || "").trim()
      if (!buffId) return []
      const remainTurns = Number(row.remainTurns)
      const triggered = Number(row.triggered)
      return [{
        buffId,
        remainTurns: Number.isFinite(remainTurns) ? Math.trunc(remainTurns) : undefined,
        triggered: Number.isFinite(triggered) && triggered >= 0 ? Math.trunc(triggered) : undefined,
        payload: row.payload && typeof row.payload === "object" && !Array.isArray(row.payload)
          ? row.payload as Record<string, unknown>
          : {}
      }]
    })
  }
  if (target.skillCooldowns && typeof target.skillCooldowns === "object" && !Array.isArray(target.skillCooldowns)) {
    const cooldowns: Record<string, number> = {}
    for (const [rawSkillID, rawRemain] of Object.entries(target.skillCooldowns as Record<string, unknown>)) {
      const skillID = String(rawSkillID || "").trim()
      const remain = Number(rawRemain)
      if (!skillID || !Number.isFinite(remain)) continue
      cooldowns[skillID] = Math.max(0, Math.trunc(remain))
    }
    unit.skillCooldowns = cooldowns
    if (Array.isArray(unit.skills)) {
      unit.skills = unit.skills.map((skill) => {
        const skillID = String(skill.id || "").trim()
        if (!skillID || !Object.prototype.hasOwnProperty.call(cooldowns, skillID)) return { ...skill }
        return { ...skill, cooldownRemain: cooldowns[skillID] }
      })
    }
  }
  return unit
}

export function parseRunStateLoose(roomID: number, raw: Record<string, unknown> | null | undefined): BattleRunStatePayload {
  const state = raw || {}
  const players = Array.isArray(state.players)
    ? state.players as BattleUnit[]
    : expandSnapshotUnits(state.p, "player")
  const enemies = Array.isArray(state.enemies)
    ? state.enemies as BattleUnit[]
    : expandSnapshotUnits(state.e, "enemy")
  return {
    roomId: roomID,
    wave: Number(state.wave ?? state.w ?? 1),
    completedBattles: Number(state.completedBattles ?? state.c ?? 0),
    seq: Number(state.seq ?? state.q ?? 0),
    order: parseBattleOrder(state.order ?? state.ro),
    preparationOrder: parsePreparationOrder(state.preparationOrder ?? state.bo),
    players,
    enemies,
    battlefieldEffects: parseBattlefieldEffectStates(state.battlefieldEffects ?? state.bf),
    arenaFormat: parseBattleArenaFormatState(state.arenaFormat ?? state.af),
    combatTrialFormat: parseBattleCombatTrialFormatState(state.combatTrialFormat ?? state.ctf),
    combatTrial: parseBattleCombatTrialState(state.combatTrial ?? state.ct)
  }
}

function parseBattleCombatTrialFormatState(raw: unknown): BattleCombatTrialFormatState | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  const nameKey = String(row.nameKey || "").trim()
  if (!id || !nameKey) return undefined
  const metrics = Array.isArray(row.metrics)
    ? row.metrics.flatMap((entry): BattleCombatTrialMetricState[] => {
        if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
        const metric = entry as Record<string, unknown>
        const metricID = String(metric.id || "").trim()
        if (!metricID) return []
        return [{
          id: metricID,
          nameKey: String(metric.nameKey || "").trim() || undefined,
          shortNameKey: String(metric.shortNameKey || "").trim() || undefined,
          descKey: String(metric.descKey || "").trim() || undefined,
          aggregation: String(metric.aggregation || "").trim() || undefined,
          eligibleEndReasons: Array.isArray(metric.eligibleEndReasons)
            ? metric.eligibleEndReasons.map((reason) => String(reason || "").trim()).filter(Boolean)
            : undefined
        }]
      })
    : []
  const encounterRow = row.encounter && typeof row.encounter === "object" && !Array.isArray(row.encounter)
    ? row.encounter as Record<string, unknown>
    : null
  const parseEncounterUnits = (rawUnits: unknown): NonNullable<NonNullable<BattleCombatTrialFormatState["encounter"]>["units"]> => Array.isArray(rawUnits)
    ? rawUnits.flatMap((entry): NonNullable<NonNullable<BattleCombatTrialFormatState["encounter"]>["units"]> => {
        if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
        const unit = entry as Record<string, unknown>
        const unitID = String(unit.id || "").trim()
        if (!unitID) return []
        return [{
          id: unitID,
          kind: String(unit.kind || "").trim(),
          healthMode: String(unit.healthMode || "").trim()
        }]
      })
    : []
  const encounterUnits = parseEncounterUnits(encounterRow?.units)
  const encounterWaves = Array.isArray(encounterRow?.waves)
    ? encounterRow.waves.flatMap((entry): NonNullable<NonNullable<BattleCombatTrialFormatState["encounter"]>["waves"]> => {
        if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
        const wave = entry as Record<string, unknown>
        const waveID = String(wave.id || "").trim()
        if (!waveID) return []
        return [{
          id: waveID,
          nameKey: String(wave.nameKey || "").trim() || undefined,
          units: parseEncounterUnits(wave.units)
        }]
      })
    : []
  const endConditions = Array.isArray(row.endConditions)
    ? row.endConditions.flatMap((entry): NonNullable<BattleCombatTrialFormatState["endConditions"]> => {
        if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
        const condition = entry as Record<string, unknown>
        const kind = String(condition.kind || "").trim()
        const reason = String(condition.reason || "").trim()
        if (!kind || !reason) return []
        return [{
          kind,
          reason,
          labelKey: String(condition.labelKey || "").trim() || undefined
        }]
      })
    : []
  return {
    id,
    nameKey,
    shortDescKey: String(row.shortDescKey || "").trim() || undefined,
    descKey: String(row.descKey || "").trim() || undefined,
    icon: String(row.icon || "lucide:target").trim(),
    actionBudget: Math.max(0, Math.trunc(Number(row.actionBudget) || 0)),
    definitionDigest: String(row.definitionDigest || "").trim() || undefined,
    metrics,
    endConditions,
    encounter: encounterRow ? {
      kind: String(encounterRow.kind || "").trim(),
      units: encounterUnits.length > 0 ? encounterUnits : undefined,
      waves: encounterWaves.length > 0 ? encounterWaves : undefined
    } : undefined
  }
}

function parseBattleCombatTrialState(raw: unknown): BattleCombatTrialState | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const row = raw as Record<string, unknown>
  const metrics: Record<string, number> = {}
  if (row.metrics && typeof row.metrics === "object" && !Array.isArray(row.metrics)) {
    for (const [metricID, rawValue] of Object.entries(row.metrics as Record<string, unknown>)) {
      const value = Number(rawValue)
      if (metricID.trim() && Number.isFinite(value)) metrics[metricID] = Math.max(0, value)
    }
  }
  const state: BattleCombatTrialState = {
    progressVersion: Math.max(0, Math.trunc(Number(row.progressVersion) || 0)) || undefined,
    actionCount: Math.max(0, Math.trunc(Number(row.actionCount) || 0)),
    actionBudget: Math.max(0, Math.trunc(Number(row.actionBudget) || 0)),
    endReason: String(row.endReason || "").trim() || undefined,
    metrics
  }
  if (row.encounterKind !== undefined) state.encounterKind = String(row.encounterKind || "").trim() || undefined
  if (row.currentWave !== undefined) state.currentWave = Math.max(0, Math.trunc(Number(row.currentWave) || 0)) || undefined
  if (row.waveCount !== undefined) state.waveCount = Math.max(0, Math.trunc(Number(row.waveCount) || 0)) || undefined
  if (row.wavesCleared !== undefined) state.wavesCleared = Math.max(0, Math.trunc(Number(row.wavesCleared) || 0))
  if (row.objectiveUnitsDefeated !== undefined) state.objectiveUnitsDefeated = Math.max(0, Math.trunc(Number(row.objectiveUnitsDefeated) || 0))
  return state
}

function parseBattleArenaFormatState(raw: unknown): BattleArenaFormatState | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const row = raw as Record<string, unknown>
  const id = String(row.id || "").trim()
  const nameKey = String(row.nameKey || "").trim()
  if (!id || !nameKey) return undefined
  const numberOrUndefined = (value: unknown): number | undefined => {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? Math.trunc(parsed) : undefined
  }
  return {
    id,
    nameKey,
    shortDescKey: String(row.shortDescKey || "").trim(),
    descKey: String(row.descKey || "").trim(),
    icon: String(row.icon || "lucide:circle-off").trim(),
    standard: row.standard === true,
    schemaVersion: numberOrUndefined(row.schemaVersion),
    rulesetVersion: String(row.rulesetVersion || "").trim() || undefined,
    balanceRevision: numberOrUndefined(row.balanceRevision),
    definitionDigest: String(row.definitionDigest || "").trim() || undefined
  }
}

function mergeSnapshotIntoRunState(current: BattleRunStatePayload, snapshot: BattleRunStatePayload): BattleRunStatePayload {
  if (Number(snapshot.seq || 0) < Number(current.seq || 0)) return current
  return {
    roomId: current.roomId,
    wave: Number.isFinite(Number(snapshot.wave)) && Number(snapshot.wave) > 0 ? Number(snapshot.wave) : current.wave,
    completedBattles: Number.isFinite(Number(snapshot.completedBattles)) ? Number(snapshot.completedBattles) : current.completedBattles,
    seq: Number.isFinite(Number(snapshot.seq)) ? Number(snapshot.seq) : current.seq,
    order: Array.isArray(snapshot.order)
      ? snapshot.order.slice()
      : (Array.isArray(current.order) ? current.order.slice() : []),
    preparationOrder: Array.isArray(snapshot.preparationOrder)
      ? snapshot.preparationOrder.slice()
      : (Array.isArray(current.preparationOrder) ? current.preparationOrder.slice() : []),
    players: Array.isArray(snapshot.players) && snapshot.players.length > 0
      ? snapshot.players
      : current.players.map((unit) => ({ ...unit })),
    enemies: Array.isArray(snapshot.enemies) && snapshot.enemies.length > 0
      ? snapshot.enemies
      : current.enemies.map((unit) => ({ ...unit })),
    battlefieldEffects: Array.isArray(snapshot.battlefieldEffects)
      ? snapshot.battlefieldEffects.map((effect) => ({ ...effect, windows: { ...effect.windows } }))
      : (current.battlefieldEffects || []).map((effect) => ({ ...effect, windows: { ...effect.windows } })),
    arenaFormat: snapshot.arenaFormat ? { ...snapshot.arenaFormat } : (current.arenaFormat ? { ...current.arenaFormat } : undefined),
    combatTrialFormat: snapshot.combatTrialFormat
      ? {
          ...snapshot.combatTrialFormat,
          metrics: snapshot.combatTrialFormat.metrics.map((metric) => ({ ...metric })),
          endConditions: snapshot.combatTrialFormat.endConditions?.map((condition) => ({ ...condition })),
          encounter: snapshot.combatTrialFormat.encounter
            ? { ...snapshot.combatTrialFormat.encounter, units: snapshot.combatTrialFormat.encounter.units.map((unit) => ({ ...unit })) }
            : undefined
        }
      : (current.combatTrialFormat
        ? {
            ...current.combatTrialFormat,
            metrics: current.combatTrialFormat.metrics.map((metric) => ({ ...metric })),
            endConditions: current.combatTrialFormat.endConditions?.map((condition) => ({ ...condition })),
            encounter: current.combatTrialFormat.encounter
              ? { ...current.combatTrialFormat.encounter, units: current.combatTrialFormat.encounter.units.map((unit) => ({ ...unit })) }
              : undefined
          }
        : undefined),
    combatTrial: snapshot.combatTrial
      ? { ...snapshot.combatTrial, metrics: { ...snapshot.combatTrial.metrics } }
      : (current.combatTrial ? { ...current.combatTrial, metrics: { ...current.combatTrial.metrics } } : undefined)
  }
}

function timelineKindFromCompact(raw: unknown): string {
  const code = String(raw || "").trim().toLowerCase()
  switch (code) {
    case "c":
      return "cast"
    case "d":
      return "damage"
    case "h":
      return "heal"
    case "r":
      return "resource"
    case "b":
      return "buff"
    case "bc":
      return "buff_charge"
    case "s":
      return "summon"
    case "se":
      return "summon_expire"
    case "cf":
      return "corpse_focus"
    case "cc":
      return "corpse_consume"
    case "t":
      return "trigger"
    case "pm":
      return "position_move"
    default:
      return code
  }
}

export function normalizeTimelineNodeData(raw: unknown): BattleRunEventData {
  const data = (raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}) as Record<string, unknown>
  const compactSteps = Array.isArray(data.p) ? data.p : []
  const legacySteps = Array.isArray(data.steps) ? data.steps : []
  const sourceSteps = compactSteps.length > 0 ? compactSteps : legacySteps
  const steps = sourceSteps
    .map((entry) => {
      const step = (entry && typeof entry === "object" && !Array.isArray(entry) ? entry : {}) as Record<string, unknown>
      const kind = timelineKindFromCompact(step.k ?? step.kind)
      const sourceUnitId = String(step.s ?? ((step.source as Record<string, unknown> | undefined)?.unitId) ?? "").trim()
      const skillId = String(step.i ?? ((step.skill as Record<string, unknown> | undefined)?.skillId) ?? "").trim()
      const buffId = String(step.bi ?? step.buffId ?? "").trim()
      const targetsRaw = Array.isArray(step.t) ? step.t : (Array.isArray(step.targets) ? step.targets : [])
      const targets = targetsRaw
        .map((targetEntry) => {
          const target = (targetEntry && typeof targetEntry === "object" && !Array.isArray(targetEntry) ? targetEntry : {}) as Record<string, unknown>
          const unitId = String(target.u ?? target.unitId ?? target.targetId ?? "").trim()
          if (!unitId) return null
          const normalizedTarget: Record<string, unknown> = { unitId }
          if (target.c !== undefined || target.crit !== undefined) {
            normalizedTarget.crit = target.c ?? target.crit
          }
          if (target.b !== undefined || target.beforeHp !== undefined || target.hpBefore !== undefined) {
            normalizedTarget.hpBefore = target.b ?? target.beforeHp ?? target.hpBefore
          }
          if (target.h !== undefined || target.afterHp !== undefined || target.hpAfter !== undefined || target.hp !== undefined) {
            normalizedTarget.hpAfter = target.h ?? target.afterHp ?? target.hpAfter ?? target.hp
          }
          if (target.m !== undefined || target.afterMp !== undefined || target.mpAfter !== undefined || target.mp !== undefined) {
            normalizedTarget.mpAfter = target.m ?? target.afterMp ?? target.mpAfter ?? target.mp
          }
          if (target.beforeMp !== undefined || target.mpBefore !== undefined) {
            normalizedTarget.mpBefore = target.beforeMp ?? target.mpBefore
          }
          if (target.l !== undefined || target.afterAlive !== undefined || target.aliveAfter !== undefined || target.alive !== undefined) {
            normalizedTarget.aliveAfter = target.l ?? target.afterAlive ?? target.aliveAfter ?? target.alive
          }
          if (target.cc !== undefined || target.corpseConsumed !== undefined) {
            normalizedTarget.corpseConsumed = target.cc ?? target.corpseConsumed
          }
          if (target.x !== undefined || target.maxHpAfter !== undefined || target.maxHp !== undefined) {
            normalizedTarget.maxHpAfter = target.x ?? target.maxHpAfter ?? target.maxHp
          }
          if (target.y !== undefined || target.maxMpAfter !== undefined || target.maxMp !== undefined) {
            normalizedTarget.maxMpAfter = target.y ?? target.maxMpAfter ?? target.maxMp
          }
          const fromPosition = expandBattlePositionCode(target.f ?? target.fromPosition)
          if (fromPosition) normalizedTarget.fromPosition = fromPosition
          const toPosition = expandBattlePositionCode(target.t ?? target.toPosition ?? target.p ?? target.position)
          if (toPosition) normalizedTarget.toPosition = toPosition
          const position = expandBattlePositionCode(target.p ?? target.position)
          if (position) normalizedTarget.position = position
          const basePosition = expandBattlePositionCode(target.bp ?? target.basePosition)
          if (basePosition) normalizedTarget.basePosition = basePosition
          const side = expandBattleSideCode(target.s ?? target.side)
          if (side) normalizedTarget.side = side
          const baseSide = expandBattleSideCode(target.bs ?? target.baseSide)
          if (baseSide) normalizedTarget.baseSide = baseSide
          const perspectiveSide = expandBattleSideCode(target.ps ?? target.perspectiveSide)
          if (perspectiveSide) normalizedTarget.perspectiveSide = perspectiveSide
          if (target.stats !== undefined) normalizedTarget.stats = target.stats
          if (target.baseStats !== undefined) normalizedTarget.baseStats = target.baseStats
          if (target.statModifiers !== undefined || target.deltaStats !== undefined) {
            normalizedTarget.statModifiers = target.statModifiers ?? target.deltaStats
          }
          if (target.statSources !== undefined) normalizedTarget.statSources = target.statSources
          if (target.buffs !== undefined) normalizedTarget.buffs = target.buffs
          if (target.meta !== undefined) normalizedTarget.meta = target.meta
          return normalizedTarget
        })
        .filter((item): item is Record<string, unknown> => Boolean(item))
      const delayMs = Number(step.d ?? step.delayMs ?? 0)
      const phaseCode = String(step.ph ?? step.phase ?? "").trim().toLowerCase()
      const phase = phaseCode === "r" ? "regen" : phaseCode === "i" ? "item" : phaseCode === "a" ? "action" : ""
      const normalized: Record<string, unknown> = {
        kind,
        delayMs: Number.isFinite(delayMs) ? delayMs : 0,
        source: { unitId: sourceUnitId },
        targets
      }
      if (phase) normalized.phase = phase
      if (skillId) normalized.skill = { skillId }
      if (buffId) normalized.buffId = buffId
      const buffDurationTurns = Number(step.bt ?? step.buffDurationTurns ?? 0)
      if (Number.isFinite(buffDurationTurns) && Math.trunc(buffDurationTurns) !== 0) {
        normalized.buffDurationTurns = Math.trunc(buffDurationTurns)
      }
      const buffTriggered = Number(step.bg ?? step.buffTriggered)
      if (Number.isFinite(buffTriggered) && buffTriggered >= 0) {
        normalized.buffTriggered = Math.trunc(buffTriggered)
      }
      const buffPayload = step.bp ?? step.buffPayload
      if (buffPayload && typeof buffPayload === "object" && !Array.isArray(buffPayload)) {
        normalized.buffPayload = { ...(buffPayload as Record<string, unknown>) }
      }
      if (step.v !== undefined || step.amount !== undefined) normalized.amount = step.v ?? step.amount
      if (step.resource !== undefined) normalized.resource = step.resource
      if (step.reason !== undefined) normalized.reason = step.reason
      if (step.c !== undefined || step.crit !== undefined) normalized.crit = step.c ?? step.crit
      if (step.dt !== undefined || step.damageType !== undefined) normalized.damageType = step.dt ?? step.damageType
      const compactMeta = (step.me ?? step.meta)
      if (compactMeta && typeof compactMeta === "object" && !Array.isArray(compactMeta)) {
        const metaRow = compactMeta as Record<string, unknown>
        const normalizedMeta: Record<string, unknown> = {}
        if (metaRow.ra !== undefined || metaRow.runtimeDamageAmp !== undefined) {
          normalizedMeta.runtimeDamageAmp = metaRow.ra ?? metaRow.runtimeDamageAmp
        }
        if (metaRow.rs !== undefined || metaRow.runtimeDamageAmpSources !== undefined) {
          normalizedMeta.runtimeDamageAmpSources = normalizeRuntimeDamageAmpSources(metaRow.rs ?? metaRow.runtimeDamageAmpSources)
        }
        if (metaRow.ms !== undefined || metaRow.mutationSources !== undefined) {
          normalizedMeta.mutationSources = normalizeRuntimeDamageAmpSources(metaRow.ms ?? metaRow.mutationSources)
        }
        if (metaRow.cm !== undefined || metaRow.criticalMultiplier !== undefined) {
          normalizedMeta.criticalMultiplier = metaRow.cm ?? metaRow.criticalMultiplier
        }
        if (metaRow.hi !== undefined || metaRow.hitIndex !== undefined) {
          normalizedMeta.hitIndex = metaRow.hi ?? metaRow.hitIndex
        }
        if (metaRow.hc !== undefined || metaRow.hitCount !== undefined) {
          normalizedMeta.hitCount = metaRow.hc ?? metaRow.hitCount
        }
        if (metaRow.hm !== undefined || metaRow.hitDamageMultiplier !== undefined) {
          normalizedMeta.hitDamageMultiplier = metaRow.hm ?? metaRow.hitDamageMultiplier
        }
        if (metaRow.ci !== undefined || metaRow.chainId !== undefined) {
          normalizedMeta.chainId = metaRow.ci ?? metaRow.chainId
        }
        if (metaRow.ce !== undefined || metaRow.corpseExplosion !== undefined) {
          normalizedMeta.corpseExplosion = metaRow.ce ?? metaRow.corpseExplosion
        }
        for (const key of ["sourceKind", "sourceId", "nodeId", "sourceNameKey", "hookId", "hookDefinitionId", "mutationId", "interruptedSkillId", "interruptedActorId"] as const) {
          if (metaRow[key] !== undefined) normalizedMeta[key] = metaRow[key]
        }
        if (metaRow.sourceInstanceId !== undefined) normalizedMeta.sourceInstanceId = metaRow.sourceInstanceId
        if (Object.keys(normalizedMeta).length > 0) {
          normalized.meta = normalizedMeta
        }
      }
      const rawSummon = (step.q ?? step.summon)
      if (rawSummon && typeof rawSummon === "object" && !Array.isArray(rawSummon)) {
        const summon = rawSummon as Record<string, unknown>
        normalized.summon = {
          unitId: summon.u ?? summon.unitId,
          side: expandBattleSideCode(summon.s ?? summon.side) || (summon.s ?? summon.side),
          name: summon.n ?? summon.name,
          nameKey: summon.k ?? summon.nameKey,
          position: expandBattlePositionCode(summon.p ?? summon.position) || (summon.p ?? summon.position),
          hp: summon.h ?? summon.hp,
          mp: summon.m ?? summon.mp,
          maxHp: summon.x ?? summon.maxHp,
          maxMp: summon.y ?? summon.maxMp,
          enemyId: summon.enemyId,
          summonTemplateUnitId: summon.summonTemplateUnitId
        }
      }
      return normalized
    })
    .filter((item): item is Record<string, unknown> => Boolean(item))

  const ownerId = String(data.o ?? ((data.turnOwner as Record<string, unknown> | undefined)?.unitId) ?? "").trim()
  const actor = (data.a && typeof data.a === "object" && !Array.isArray(data.a) ? data.a : (data.summary as Record<string, unknown> | undefined)?.actor || {}) as Record<string, unknown>
  const unitUpdatesRaw = Array.isArray(data.u) ? data.u : (Array.isArray((data as Record<string, unknown>).unitUpdates) ? (data as Record<string, unknown>).unitUpdates as unknown[] : [])
  const unitUpdates = unitUpdatesRaw
    .map((entry) => {
      const row = (entry && typeof entry === "object" && !Array.isArray(entry) ? entry : {}) as Record<string, unknown>
      const unitId = String(row.u ?? row.unitId ?? "").trim()
      if (!unitId) return null
      const out: Record<string, unknown> = { unitId }
      if (row.h !== undefined || row.hpAfter !== undefined) out.hpAfter = row.h ?? row.hpAfter
      if (row.m !== undefined || row.mpAfter !== undefined) out.mpAfter = row.m ?? row.mpAfter
      if (row.l !== undefined || row.aliveAfter !== undefined) out.aliveAfter = row.l ?? row.aliveAfter
      if (row.cc !== undefined || row.corpseConsumed !== undefined) out.corpseConsumed = row.cc ?? row.corpseConsumed
      if (row.x !== undefined || row.maxHpAfter !== undefined || row.maxHp !== undefined) out.maxHpAfter = row.x ?? row.maxHpAfter ?? row.maxHp
      if (row.y !== undefined || row.maxMpAfter !== undefined || row.maxMp !== undefined) out.maxMpAfter = row.y ?? row.maxMpAfter ?? row.maxMp
      const position = expandBattlePositionCode(row.p ?? row.position ?? row.toPosition)
      if (position) out.position = position
      const basePosition = expandBattlePositionCode(row.bp ?? row.basePosition)
      if (basePosition) out.basePosition = basePosition
      const side = expandBattleSideCode(row.s ?? row.side)
      if (side) out.side = side
      const baseSide = expandBattleSideCode(row.bs ?? row.baseSide)
      if (baseSide) out.baseSide = baseSide
      const perspectiveSide = expandBattleSideCode(row.ps ?? row.perspectiveSide)
      if (perspectiveSide) out.perspectiveSide = perspectiveSide
      if (row.stats !== undefined) out.stats = row.stats
      if (row.baseStats !== undefined) out.baseStats = row.baseStats
      if (row.statModifiers !== undefined || row.deltaStats !== undefined) out.statModifiers = row.statModifiers ?? row.deltaStats
      if (row.statSources !== undefined) out.statSources = row.statSources
      if (row.buffs !== undefined) out.buffs = row.buffs
      return out
    })
    .filter((item): item is Record<string, unknown> => Boolean(item))

  return {
    wave: Number(data.w ?? data.wave ?? 0),
    preparationTurn: Boolean(data.pr ?? data.preparationTurn),
    extraTurn: Boolean(data.et ?? data.extraTurn),
    turnKind: String(data.tk ?? data.turnKind ?? "").trim() || undefined,
    skillCooldowns: toAuthoritativeCooldownRecordLoose(data.sc ?? data.skillCooldowns),
    skillCooldownTurns: toAuthoritativeCooldownRecordLoose(data.st ?? data.skillCooldownTurns),
    itemCooldowns: toAuthoritativeCooldownRecordLoose(data.ic ?? data.itemCooldowns),
    order: parseBattleOrder(data.ro ?? data.order),
    preparationOrder: Array.isArray(data.po)
      ? data.po.map((value) => String(value || "").trim()).filter(Boolean)
      : (Array.isArray(data.preparationOrder)
        ? data.preparationOrder.map((value) => String(value || "").trim()).filter(Boolean)
        : []),
    turnOwner: { unitId: ownerId },
    parentSeq: Number(data.parentSeq || 0) || undefined,
    batchGroup: String(data.batchGroup || "").trim() || undefined,
    combatTrial: parseBattleCombatTrialState(data.combatTrial ?? data.ct),
    steps,
    unitUpdates,
    summary: {
      actor: {
        hp: actor.h ?? actor.hp,
        mp: actor.m ?? actor.mp,
        alive: actor.l ?? actor.alive
      }
    }
  } as BattleRunEventData
}

export function normalizeBattleEventData(eventType: string, raw: unknown): BattleRunEventData {
  if (eventType === "battle.timeline.node") {
    return normalizeTimelineNodeData(raw)
  }
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    return raw as BattleRunEventData
  }
  return {} as BattleRunEventData
}

export function normalizeRuntimeDamageAmpSources(raw: unknown) {
  if (!Array.isArray(raw)) return []
  return raw
    .map((entry) => {
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) return null
      const source = entry as Record<string, unknown>
      const normalized: Record<string, unknown> = {}
      for (const key of [
        "kind",
        "id",
        "sourceKind",
        "sourceId",
        "sourceItemId",
        "sourceAffixId",
        "sourceAffixSlot",
        "sourceInstanceId",
        "sourceEnhanceLevel",
        "nodeId",
        "sourceNameKey",
        "hookId",
        "mutationId",
        "ruleId",
        "effect",
        "runtimeDamageAmp"
      ] as const) {
        if (source[key] !== undefined) {
          normalized[key] = source[key]
        }
      }
      return Object.keys(normalized).length > 0 ? normalized : null
    })
    .filter((entry): entry is Record<string, unknown> => entry !== null)
}

export function expandBattleEventType(code: unknown): string {
  const normalized = String(code || "").trim()
  switch (normalized) {
    case "n":
      return "battle.timeline.node"
    case "s":
      return "battle.wave.spawn"
    case "w":
      return "battle.wave.win"
    case "a":
      return "battle.turn.action"
    case "r":
      return "battle.turn.regen"
    case "i":
      return "battle.turn.item_use"
    case "ft":
      return "battle.field.tick"
    case "fg":
      return "battle.field.trigger"
    case "fw":
      return "battle.field.window"
    default:
      return normalized
  }
}

export function battlePayloadStreamKind(payload: unknown): "room" | "arena" | "combat_trial" {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) return "room"
  const data = payload as { k?: unknown; battleKind?: unknown }
  const kind = String(data.k ?? data.battleKind ?? "").trim().toLowerCase()
  if (kind === "arena" || kind === "combat_trial") return kind
  return "room"
}

function expandBattleSideCode(raw: unknown): string | undefined {
  if (raw === 1 || String(raw || "").trim() === "1") return "enemy"
  if (raw === 0 || String(raw || "").trim() === "0") return "player"
  const value = String(raw || "").trim().toLowerCase()
  if (!value) return undefined
  if (value === "enemy" || value === "player") return value
  return undefined
}

function expandBattlePositionCode(raw: unknown): string | undefined {
  if (raw === 0 || String(raw || "").trim() === "0") return "vanguard"
  if (raw === 1 || String(raw || "").trim() === "1") return "midguard"
  if (raw === 2 || String(raw || "").trim() === "2") return "rearguard"
  const value = String(raw || "").trim().toLowerCase()
  if (!value) return undefined
  if (value === "vanguard" || value === "midguard" || value === "rearguard") return value
  return undefined
}

function normalizeBattleSideLoose(raw: unknown): "player" | "enemy" | "" {
  const side = String(raw || "").trim().toLowerCase()
  if (side === "player" || side === "enemy") return side
  return ""
}

function toCooldownRecordLoose(raw: unknown): Record<string, number> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const input = raw as Record<string, unknown>
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(input)) {
    const normalizedKey = String(key || "").trim()
    if (!normalizedKey) continue
    const parsed = Math.trunc(Number(value))
    if (!Number.isFinite(parsed) || parsed <= 0) continue
    out[normalizedKey] = parsed
  }
  return Object.keys(out).length > 0 ? out : undefined
}

function toAuthoritativeCooldownRecordLoose(raw: unknown): Record<string, number> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const input = raw as Record<string, unknown>
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(input)) {
    const normalizedKey = String(key || "").trim()
    const parsed = Math.trunc(Number(value))
    if (!normalizedKey || !Number.isFinite(parsed)) continue
    out[normalizedKey] = Math.max(0, parsed)
  }
  return out
}

function toSkillLevelRecordLoose(raw: unknown): Record<string, number> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined
  const input = raw as Record<string, unknown>
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(input)) {
    const normalizedKey = String(key || "").trim()
    if (!normalizedKey) continue
    const parsed = Math.trunc(Number(value))
    if (!Number.isFinite(parsed) || parsed <= 0) continue
    out[normalizedKey] = parsed
  }
  return Object.keys(out).length > 0 ? out : undefined
}

function toNumberRecordLoose(raw: unknown): Record<string, number> | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const input = raw as Record<string, unknown>
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(input)) {
    const parsed = Number(value)
    if (!Number.isFinite(parsed)) continue
    const normalized = String(key || "").trim()
    if (!normalized) continue
    out[normalized] = parsed
  }
  return out
}

function toNestedNumberRecordLoose(raw: unknown): Record<string, Record<string, number>> | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const input = raw as Record<string, unknown>
  const out: Record<string, Record<string, number>> = {}
  for (const [key, value] of Object.entries(input)) {
    const normalized = String(key || "").trim()
    if (!normalized) continue
    const row = toNumberRecordLoose(value)
    if (!row) continue
    out[normalized] = row
  }
  return out
}

function expandSnapshotBuffs(raw: unknown): BattleUnit["buffs"] {
  if (!Array.isArray(raw)) return []
  const out = raw
    .map((entry) => {
      const row = (entry && typeof entry === "object" && !Array.isArray(entry) ? entry : {}) as Record<string, unknown>
      const buffId = String(row.i || "").trim()
      if (!buffId) return null
      const remainTurns = Number(row.t)
      const triggered = Number(row.g)
      const payload = row.p && typeof row.p === "object" && !Array.isArray(row.p)
        ? (row.p as Record<string, unknown>)
        : {}
      return {
        buffId,
        remainTurns: Number.isFinite(remainTurns) ? Math.trunc(remainTurns) : undefined,
        payload,
        triggered: Number.isFinite(triggered) && triggered >= 0 ? Math.trunc(triggered) : undefined
      }
    })
    .filter((item): item is NonNullable<typeof item> => Boolean(item))
  return out
}

export function expandSnapshotUnits(raw: unknown, defaultSide: "player" | "enemy"): BattleUnit[] {
  if (!Array.isArray(raw)) return []
  const out: BattleUnit[] = []
  for (const entry of raw) {
    const row = (entry && typeof entry === "object" && !Array.isArray(entry) ? entry : {}) as Record<string, unknown>
    const unitId = String(row.u || "").trim()
    if (!unitId) continue
    const hp = Number(row.h)
    const mp = Number(row.m)
    const maxHp = Number(row.x)
    const maxMp = Number(row.y)
    let alive = typeof row.a === "boolean" ? row.a : undefined
    if (alive === false && Number.isFinite(hp) && hp > 0) {
      alive = true
    }
    const cooldowns = toCooldownRecordLoose(row.skillCooldowns || row.d)
    const cooldownTurns = toAuthoritativeCooldownRecordLoose(row.skillCooldownTurns ?? row.st)
    const itemCooldowns = toCooldownRecordLoose(row.itemCooldowns || row.ic)
    const itemQuantities = toSkillLevelRecordLoose(row.itemQuantities || row.iq)
    const levels = toSkillLevelRecordLoose(row.skillLevels || row.g)
    const stats = toNumberRecordLoose(row.z)
    const skillIds = Array.isArray(row.i)
      ? row.i.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    const skills = skillIds.map((skillID) => {
      const skill: BattleSkill = {
        id: skillID,
        level: Math.max(1, Math.trunc(Number(levels?.[skillID] ?? 1))),
        cooldownRemain: Math.max(0, Math.trunc(Number(cooldowns?.[skillID] ?? 0)))
      }
      if (cooldownTurns && Object.prototype.hasOwnProperty.call(cooldownTurns, skillID)) {
        const total = Math.max(0, Math.trunc(Number(cooldownTurns[skillID])))
        skill.cooldown = total
        skill.cooldownTurns = total
      }
      return skill
    })
    const rawBattleItems = Array.isArray(row.battleItems)
      ? row.battleItems
      : Array.isArray(row.bi)
        ? row.bi
        : []
    const battleItems = Array.isArray(rawBattleItems)
      ? rawBattleItems
        .map((item) => (item && typeof item === "object" && !Array.isArray(item) ? (item as Record<string, unknown>) : null))
        .filter((item): item is Record<string, unknown> => Boolean(item))
        .map((item) => ({
          itemId: String(item.itemId || item.i || "").trim(),
          quantity: Math.max(0, Math.trunc(Number(item.quantity || item.q || 0))),
          cooldownTurns: Math.max(0, Math.trunc(Number(item.cooldownTurns || item.t || 0))),
          cooldownRemain: Math.max(0, Math.trunc(Number(item.cooldownRemain || item.r || 0)))
        }))
        .filter((item) => item.itemId)
      : []
    out.push({
      unitId,
      unitType: String(row.t || row.unitType || "").trim() || undefined,
      side: expandBattleSideCode(row.s) || defaultSide,
      name: String(row.n || "").trim() || undefined,
      position: expandBattlePositionCode(row.p) || "vanguard",
      hp: Number.isFinite(hp) ? hp : undefined,
      mp: Number.isFinite(mp) ? mp : undefined,
      maxHp: Number.isFinite(maxHp) ? maxHp : undefined,
      maxMp: Number.isFinite(maxMp) ? maxMp : undefined,
      alive,
      skillIds,
      skillCooldowns: cooldowns,
      skillCooldownTurns: cooldownTurns,
      itemCooldowns,
      itemQuantities,
      skills,
      battleItems,
      buffs: expandSnapshotBuffs(row.f),
      stats: stats || undefined,
      attributes: stats ? { ...stats } : undefined
    })
  }
  return out
}

type BattleWorldOption = {
  id: string
  label: string
  description: string
  enemyIds: string[]
  specialEnemyIds: string[]
  specialWaves: number[]
  specialWaveEvery: number
  specialWaveMinCount: number
  specialWaveMaxCount: number
  waveSchedules: Array<{
    id: string
    label: string
    description: string
    every: number
    priority: number
    minCount: number
    maxCount: number
    enemyChoices: Array<{ enemyId: string; weight: number }>
  }>
  triggerEncounters: Array<{
    id: string
    itemId: string
    enemyIds: string[]
    minWave?: number
    intervalWaves?: number
    perWaveChance?: number
    partyTriggerGroup?: string
  }>
  entryDurationSeconds: number
  maxTeamSize: number
  entryCosts: Record<string, number>
  fogMapItemId: string
  normalEnemyCount: number
  normalEnemyCountRange: { min: number; max: number }
  extraEnemyPerPlayer: { min: number; max: number }
  customLineupConf: Array<{
    id: string
    label: string
    percent: number
    isSpecialWaveLineup: boolean
    lineup: string[]
    drops: {
      items: Record<string, { min: number; max: number; percent: number }>
    }
  }>
  waveDrops: Record<string, { min: number; max: number; percent: number }>
  difficultySettings: WorldDifficultySettings | null
  battlefieldEffects: Array<{
    id: string
    nameKey: string
    descKey: string
    icon: string
    initial: number
    displayMode: "countdown" | "progress"
  }>
}

function toStringArray(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean)
  }
  if (typeof value === "string" && value.trim()) {
    try {
      const parsed = JSON.parse(value)
      if (Array.isArray(parsed)) {
        return parsed.map((item) => String(item).trim()).filter(Boolean)
      }
    } catch {
      return value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean)
    }
  }
  return []
}

function toNumberArray(value: unknown): number[] {
  if (Array.isArray(value)) {
    return value
      .map((item) => Number(item))
      .filter((item) => Number.isFinite(item))
      .map((item) => Math.trunc(item))
  }
  if (typeof value === "string" && value.trim()) {
    try {
      const parsed = JSON.parse(value)
      if (Array.isArray(parsed)) {
        return parsed
          .map((item) => Number(item))
          .filter((item) => Number.isFinite(item))
          .map((item) => Math.trunc(item))
      }
    } catch {
      return value
        .split(",")
        .map((item) => Number(item.trim()))
        .filter((item) => Number.isFinite(item))
        .map((item) => Math.trunc(item))
    }
  }
  return []
}

function toNumberMap(value: unknown): Record<string, number> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {}
  const out: Record<string, number> = {}
  for (const [key, raw] of Object.entries(value)) {
    const normalizedKey = String(key || "").trim()
    if (!normalizedKey) continue
    const parsed = Number(raw)
    if (!Number.isFinite(parsed)) continue
    out[normalizedKey] = parsed
  }
  return out
}

function parseWorldCountRange(raw: unknown): { min: number; max: number } {
  const parseIntSafe = (value: unknown, fallback = 0) => {
    const parsed = Math.trunc(Number(value))
    if (!Number.isFinite(parsed)) return fallback
    return Math.max(0, parsed)
  }
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const node = raw as Record<string, unknown>
    const min = parseIntSafe(node.min, 0)
    const max = Math.max(min, parseIntSafe(node.max, min))
    return { min, max }
  }
  const fixed = parseIntSafe(raw, 0)
  return { min: fixed, max: fixed }
}

function parseWorldSpecialWaves(raw: unknown): Pick<BattleWorldOption, "specialWaves" | "specialWaveEvery" | "specialWaveMinCount" | "specialWaveMaxCount"> {
  const fallback = {
    specialWaves: [] as number[],
    specialWaveEvery: 0,
    specialWaveMinCount: 0,
    specialWaveMaxCount: 0
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return {
      ...fallback,
      specialWaves: toNumberArray(raw).sort((a, b) => a - b)
    }
  }
  const node = raw as Record<string, unknown>
  const every = Math.max(0, Math.trunc(Number(node.every) || 0))
  if (every <= 0) {
    return fallback
  }
  const minCount = Math.max(1, Math.trunc(Number(node.minCount) || 1))
  const maxCount = Math.max(minCount, Math.trunc(Number(node.maxCount) || minCount))
  return {
    specialWaves: [],
    specialWaveEvery: every,
    specialWaveMinCount: minCount,
    specialWaveMaxCount: maxCount
  }
}

function parseWorldCustomLineupConf(raw: unknown, t: (key: string) => string): BattleWorldOption["customLineupConf"] {
  if (!Array.isArray(raw)) return []
  const out: BattleWorldOption["customLineupConf"] = []
  for (const entry of raw) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
    const node = entry as Record<string, unknown>
    const id = typeof node.id === "string" ? node.id.trim() : ""
    if (!id) continue
    const nameKey = typeof node.nameKey === "string" ? node.nameKey.trim() : ""
    const translated = nameKey ? t(nameKey) : ""
    const dropsNode = node.drops && typeof node.drops === "object" && !Array.isArray(node.drops)
      ? (node.drops as Record<string, unknown>)
      : {}
    const itemsNode = dropsNode.items && typeof dropsNode.items === "object" && !Array.isArray(dropsNode.items)
      ? (dropsNode.items as Record<string, unknown>)
      : {}
    const items: Record<string, { min: number; max: number; percent: number }> = {}
    for (const [itemID, payload] of Object.entries(itemsNode)) {
      const normalizedItemID = String(itemID || "").trim()
      if (!normalizedItemID || !payload || typeof payload !== "object" || Array.isArray(payload)) continue
      const rule = payload as Record<string, unknown>
      const min = Math.max(0, Math.trunc(Number(rule.min) || 0))
      const max = Math.max(min, Math.trunc(Number(rule.max) || min))
      const percent = Math.min(1, Math.max(0, Number(rule.percent) || 0))
      items[normalizedItemID] = { min, max, percent }
    }
    out.push({
      id,
      label: translated && translated !== nameKey ? translated : id,
      percent: Math.min(1, Math.max(0, Number(node.percent) || 0)),
      isSpecialWaveLineup: Boolean(node.isSpecialWaveLineup),
      lineup: toStringArray(node.lineup),
      drops: { items }
    })
  }
  return out
}

export function parseWorldWaveSchedules(raw: unknown, t: (key: string) => string): BattleWorldOption["waveSchedules"] {
  if (!Array.isArray(raw)) return []
  const seen = new Set<string>()
  const out: BattleWorldOption["waveSchedules"] = []
  for (const entry of raw) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
    const row = entry as Record<string, unknown>
    const id = String(row.id || "").trim()
    const every = Math.max(0, Math.trunc(Number(row.every) || 0))
    if (!id || every <= 0 || seen.has(id)) continue
    const enemyChoices = (Array.isArray(row.enemyChoices) ? row.enemyChoices : []).flatMap((choice) => {
      if (!choice || typeof choice !== "object" || Array.isArray(choice)) return []
      const node = choice as Record<string, unknown>
      const enemyId = String(node.enemyId || "").trim()
      const weight = Number(node.weight)
      return enemyId && Number.isFinite(weight) && weight > 0 ? [{ enemyId, weight }] : []
    })
    if (enemyChoices.length === 0) continue
    const minCount = Math.max(1, Math.trunc(Number(row.minCount) || 1))
    const maxCount = Math.max(minCount, Math.trunc(Number(row.maxCount) || minCount))
    const nameKey = String(row.nameKey || "").trim()
    const descKey = String(row.descKey || "").trim()
    const translatedName = nameKey ? t(nameKey) : ""
    const translatedDesc = descKey ? t(descKey) : ""
    seen.add(id)
    out.push({
      id,
      label: translatedName && translatedName !== nameKey ? translatedName : id,
      description: translatedDesc && translatedDesc !== descKey ? translatedDesc : "",
      every,
      priority: Math.trunc(Number(row.priority) || 0),
      minCount,
      maxCount,
      enemyChoices
    })
  }
  return out.sort((a, b) => b.priority - a.priority || a.id.localeCompare(b.id))
}

type BattleWorldTriggerEncounter = BattleWorldOption["triggerEncounters"][number]

function buildWorldTriggerEncounterIndex(raw: unknown): Map<string, BattleWorldTriggerEncounter[]> {
  const rows = Array.isArray(raw)
    ? raw
    : raw && typeof raw === "object" && !Array.isArray(raw)
      ? (raw as { battle_usable_items?: unknown }).battle_usable_items
      : []
  if (!Array.isArray(rows)) return new Map()

  const byWorldID = new Map<string, BattleWorldTriggerEncounter[]>()
  for (const entry of rows) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
    const row = entry as Record<string, unknown>
    if (!parseDisabledFlag(row.isTrigger) || parseDisabledFlag(row.disabled)) continue
    const id = String(row.id || "").trim()
    const itemId = String(row.itemId || "").trim()
    const dungeonIds = Array.from(new Set(toStringArray(row.dungeonIds)))
    const enemyIds = Array.from(new Set(toStringArray(row.spawnEnemyIds)))
    if (!id || !itemId || dungeonIds.length === 0 || enemyIds.length === 0) continue

    const minWaveValue = Number(row.minWave)
    const intervalWavesValue = Number(row.intervalWaves)
    const perWaveChanceValue = Number(row.perWaveChance)
    const partyTriggerGroup = String(row.partyTriggerGroup || "").trim()
    const encounter: BattleWorldTriggerEncounter = {
      id,
      itemId,
      enemyIds,
      ...(Number.isFinite(minWaveValue) ? { minWave: Math.max(1, Math.trunc(minWaveValue)) } : {}),
      ...(Number.isFinite(intervalWavesValue) && intervalWavesValue > 0
        ? { intervalWaves: Math.trunc(intervalWavesValue) }
        : {}),
      ...(Number.isFinite(perWaveChanceValue)
        ? { perWaveChance: Math.min(1, Math.max(0, perWaveChanceValue)) }
        : {}),
      ...(partyTriggerGroup ? { partyTriggerGroup } : {})
    }
    for (const dungeonId of dungeonIds) {
      const encounters = byWorldID.get(dungeonId) || []
      encounters.push(encounter)
      byWorldID.set(dungeonId, encounters)
    }
  }

  for (const encounters of byWorldID.values()) {
    encounters.sort((a, b) => (a.minWave ?? 0) - (b.minWave ?? 0) || a.id.localeCompare(b.id))
  }
  return byWorldID
}

export function parseWorldTriggerEncounters(raw: unknown, worldID: string): BattleWorldOption["triggerEncounters"] {
  const normalizedWorldID = String(worldID || "").trim()
  if (!normalizedWorldID) return []
  return buildWorldTriggerEncounterIndex(raw).get(normalizedWorldID) || []
}

function resolveWorldLabel(row: Record<string, unknown>, t: (key: string) => string, id: string): string {
  const nameKey = typeof row.nameKey === "string" ? row.nameKey.trim() : ""
  if (nameKey) {
    const translated = t(nameKey)
    if (translated && translated !== nameKey) return translated
  }
  return id
}

function resolveWorldDescription(row: Record<string, unknown>, t: (key: string) => string): string {
  const descKey = typeof row.descKey === "string" ? row.descKey.trim() : ""
  if (!descKey) return ""
  const translated = t(descKey)
  return translated && translated !== descKey ? translated : ""
}

function parseDisabledFlag(raw: unknown): boolean {
  if (typeof raw === "boolean") return raw
  if (typeof raw === "number") return raw !== 0
  if (typeof raw === "string") {
    const normalized = raw.trim().toLowerCase()
    if (normalized === "true" || normalized === "1") return true
    if (normalized === "false" || normalized === "0") return false
  }
  return false
}

function parseWorldDropRules(raw: unknown): Record<string, { min: number; max: number; percent: number }> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  const out: Record<string, { min: number; max: number; percent: number }> = {}
  for (const [itemId, payload] of Object.entries(raw as Record<string, unknown>)) {
    if (!itemId.trim() || !payload || typeof payload !== "object") continue
    const p = payload as Record<string, unknown>
    let min = 0, max = 0
    if (p.range && typeof p.range === "object") {
      const r = p.range as Record<string, unknown>
      min = Math.max(0, Number(r.min) || 0)
      max = Math.max(min, Number(r.max) || min)
    } else {
      min = Math.max(0, Number(p.min) || 0)
      max = Math.max(min, Number(p.max) || min)
    }
    const percent = Math.min(1, Math.max(0, Number(p.percent) || 0))
    out[itemId.trim()] = { min, max, percent }
  }
  return out
}

function parseWorldBattlefieldEffects(raw: unknown): BattleWorldOption["battlefieldEffects"] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
    const node = entry as Record<string, unknown>
    const counter = node.counter && typeof node.counter === "object" && !Array.isArray(node.counter)
      ? node.counter as Record<string, unknown>
      : {}
    const id = String(node.id || "").trim()
    if (!id) return []
    return [{
      id,
      nameKey: String(node.nameKey || "").trim(),
      descKey: String(node.descKey || "").trim(),
      icon: String(node.icon || "").trim(),
      initial: Math.max(1, Math.trunc(Number(counter.initial) || 1)),
      displayMode: String(node.displayMode || "").trim().toLowerCase() === "progress" ? "progress" : "countdown"
    }]
  })
}

export function parseWorldRows(t: (key: string) => string): BattleWorldOption[] {
  const catalog = getGameDataChunk("world") as Record<string, unknown>
  const triggerEncounterIndex = buildWorldTriggerEncounterIndex(getGameDataChunk("battle_usable_items"))
  const rawRows = Array.isArray(catalog.world) ? catalog.world : []
  const worlds: BattleWorldOption[] = []
  for (const row of rawRows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const source = row as Record<string, unknown>
    if (parseDisabledFlag(source.disabled)) continue
    const id = typeof source.id === "string" ? source.id.trim() : ""
    if (!id) continue
    const specialWaveConfig = parseWorldSpecialWaves(source.specialWaves)
    const normalEnemyCountRange = parseWorldCountRange(source.normalEnemyCountRange ?? source.normalEnemyCount)
    const extraEnemyPerPlayer = parseWorldCountRange(source.extraEnemyPerPlayer)
    const customLineupConf = parseWorldCustomLineupConf(source.customLineupConf, t)
    const waveSchedules = parseWorldWaveSchedules(source.waveSchedules, t)
    worlds.push({
      id,
      label: resolveWorldLabel(source, t, id),
      description: resolveWorldDescription(source, t),
      enemyIds: toStringArray(source.enemyIds),
      specialEnemyIds: toStringArray(source.specialEnemyIds),
      specialWaves: specialWaveConfig.specialWaves,
      specialWaveEvery: specialWaveConfig.specialWaveEvery,
      specialWaveMinCount: specialWaveConfig.specialWaveMinCount,
      specialWaveMaxCount: specialWaveConfig.specialWaveMaxCount,
      waveSchedules,
      triggerEncounters: triggerEncounterIndex.get(id) || [],
      entryDurationSeconds: Math.max(0, Math.trunc(Number(source.entryDurationSeconds) || 0)),
      maxTeamSize: Math.max(1, Math.trunc(Number(source.maxTeamSize) || 3)),
      entryCosts: toNumberMap(source.entryCosts),
      fogMapItemId: String(source.fogMapItemId || "").trim(),
      normalEnemyCount: normalEnemyCountRange.min,
      normalEnemyCountRange,
      extraEnemyPerPlayer,
      customLineupConf,
      waveDrops: parseWorldDropRules(source.waveDrops),
      difficultySettings: normalizeWorldDifficultySettings(source.difficultySettings),
      battlefieldEffects: parseWorldBattlefieldEffects(source.battlefieldEffects)
    })
  }
  return worlds
}

export function useCombatPage() {
  const { t } = useI18n()
  const store = useGameStore()
  const chatStore = useChatStore()
  const realtimeStore = useRealtimeStore()
  const wsUnbinders: Array<() => void> = []

  const selectedWorldID = ref("")
  const createRoomName = ref("")
  const createAutoReenterOnDefeat = ref(true)
  const createIsPublic = ref(false)
  const createIsFogMode = ref(false)
  const createPosition = ref("vanguard")
  const createDifficultyLevel = ref(0)

  const publicRooms = ref<BattleRoom[]>([])
  const myRoom = ref<BattleRoomDetail | null>(null)
  const selectedRoomID = ref<number | null>(null)
  const selectedRoom = ref<BattleRoomDetail | null>(null)
  const selectedRun = ref<BattleRunRecord | null>(null)
  const runState = ref<BattleRunStatePayload | null>(null)
  const runEvents = ref<BattleRunEventPayload[]>([])
  const roomsLoading = ref(false)
  const runLoading = ref(false)
  const kickedRoomID = ref<number | null>(null)
  const interBattleWait = ref<BattleIntermissionWait | null>(null)
  const enteringWarnedRoomId = ref<number | null>(null)
  let enteringWarnTimer: ReturnType<typeof setTimeout> | null = null
  let runBatchWatchTimer: ReturnType<typeof setInterval> | null = null
  let runBootstrapLoading = false
  let runBootstrapRoomID: number | null = null
  let runBootstrapRunID: number | null = null
  let runLoadFlight: { roomID: number; runID: number; promise: Promise<void> } | null = null
  let loadRoomsRequestSeq = 0
  let loadRunRequestSeq = 0
  let lastRunCompiledAtMs = 0
  let lastRunBatchAtMs = 0
  let runResyncCooldownUntilMs = 0
  const runBatchStallThresholdMs = 12_000
  const runBatchResyncCooldownMs = 4_000
  const runCompiledBatchWatchdogMs = 3_000

  const worlds = computed(() => parseWorldRows(t))
  const selectedWorld = computed(() => worlds.value.find((world) => world.id === selectedWorldID.value) || null)
  const selectedWorldCosts = computed(() => Object.entries(selectedWorld.value?.entryCosts || {}).sort(([a], [b]) => a.localeCompare(b)))

  const isRoomOwner = computed(() => {
    if (!selectedRoom.value) return false
    return selectedRoom.value.room.ownerId === store.userId
  })

  const isMyRoomOwner = computed(() => {
    if (!myRoom.value) return false
    return myRoom.value.room.ownerId === store.userId
  })

  const isInSelectedRoom = computed(() => {
    if (!selectedRoom.value) return false
    return selectedRoom.value.members.some((member) => member.userId === store.userId)
  })

  const myMember = computed(() => {
    if (!selectedRoom.value) return null
    return selectedRoom.value.members.find((member) => member.userId === store.userId) || null
  })

  const myRoomDetail = computed(() => {
    if (!myRoom.value) return null
    if (selectedRoom.value?.room.id === myRoom.value.room.id) {
      return selectedRoom.value
    }
    return myRoom.value
  })

  const myRoomMember = computed(() => {
    if (!myRoomDetail.value) return null
    return myRoomDetail.value.members.find((member) => member.userId === store.userId) || null
  })

  function roomStatusLabel(status: string) {
    switch (status) {
      case "idle":
        return t("game.battleRoomStatusIdle")
      case "preparing":
        return t("game.battleRoomStatusPreparing")
      case "entering":
        return t("game.battleRoomStatusEntering")
      case "playing":
        return t("game.battleRoomStatusPlaying")
      case "recovering":
        return t("game.battleRoomStatusRecovering")
      default:
        return status
    }
  }

  function runStatusLabel(status: string) {
    switch (status) {
      case "compiled":
      case "settling":
        return t("game.battleRoomStatusPreparing")
      case "playing":
        return t("game.battleRunStatusRunning")
      case "ended":
        return t("game.battleRunStatusEnded")
      case "failed":
        return t("game.battleRunStatusFailed")
      case "interrupted":
        return t("game.battleRunStatusInterrupted")
      default:
        return status
    }
  }

  function positionLabel(position: string) {
    switch (position) {
      case "vanguard":
        return t("game.battlePositionVanguard")
      case "midguard":
        return t("game.battlePositionMidguard")
      case "rearguard":
        return t("game.battlePositionRearguard")
      default:
        return position
    }
  }

  function normalizeBattlePosition(position: unknown) {
    const raw = String(position || "").trim().toLowerCase()
    if (raw === "vanguard" || raw === "midguard" || raw === "rearguard") return raw
    return ""
  }

  function personalityLabel(personalityID: string, nameKey?: string) {
    const normalizedID = String(personalityID || "").trim().toLowerCase()
    const normalizedKey = String(nameKey || "").trim()
    const key = normalizedKey || `kitty.personality.${normalizedID}.name`
    const localized = t(key)
    if (localized !== key && localized.trim()) return localized
    return normalizedID || "-"
  }

  function clearBattleState() {
    loadRunRequestSeq++
    runLoadFlight = null
    runLoading.value = false
    selectedRun.value = null
    runState.value = null
    runEvents.value = []
    interBattleWait.value = null
    lastRunCompiledAtMs = 0
    lastRunBatchAtMs = 0
    runResyncCooldownUntilMs = 0
    runBootstrapLoading = false
    runBootstrapRoomID = null
    runBootstrapRunID = null
  }

  function hasBatchSnapshot(payload: BattleRunBatchPayload | null | undefined): boolean {
    if (!payload?.n) return false
    return payload.n.w !== undefined
      || payload.n.c !== undefined
      || payload.n.q !== undefined
      || Array.isArray(payload.n.ro)
      || Array.isArray(payload.n.bo)
      || Array.isArray(payload.n.p)
      || Array.isArray(payload.n.e)
      || Array.isArray(payload.n.bf)
  }

  function hasRunMainState(roomID: number, runID: number): boolean {
    if (!runState.value || !selectedRun.value) return false
    if (runState.value.roomId !== roomID) return false
    if (selectedRun.value.id !== runID) return false
    return true
  }

  function ensureRunMainState(roomID: number, runID: number) {
    if (runBootstrapLoading && runBootstrapRoomID === roomID && runBootstrapRunID === runID) return
    runBootstrapLoading = true
    runBootstrapRoomID = roomID
    runBootstrapRunID = runID
    void loadRun(roomID, runID).finally(() => {
      if (runBootstrapRoomID === roomID && runBootstrapRunID === runID) {
        runBootstrapLoading = false
        runBootstrapRoomID = null
        runBootstrapRunID = null
      }
    })
  }

  function requestRunStateResync(roomID: number, runID: number) {
    const now = Date.now()
    if (now < runResyncCooldownUntilMs) return
    runResyncCooldownUntilMs = now + runBatchResyncCooldownMs
    ensureRunMainState(roomID, runID)
  }

  function completeBattleTransition(runID: number) {
    if (battleTransitionCanComplete(interBattleWait.value, runID)) {
      interBattleWait.value = null
    }
  }

  function clearRunBootstrapState(roomID?: number) {
    if (roomID && runBootstrapRoomID !== roomID) return
    runBootstrapLoading = false
    runBootstrapRoomID = null
    runBootstrapRunID = null
  }

  function monitorRunBatchStall() {
    const roomID = Number(myRoom.value?.room.id || selectedRoomID.value || 0)
    if (!roomID) return
    const roomDetail = getTrackedRoomDetail(roomID)
    const roomStatus = String(roomDetail?.room.status || "")
    if (roomStatus !== "playing") return
    const runID = Number(selectedRun.value?.id || roomDetail?.room.currentRunId || 0)
    if (!runID) return
    const now = Date.now()
    if (runBootstrapLoading) return
    if (lastRunCompiledAtMs > 0 && lastRunBatchAtMs < lastRunCompiledAtMs && now-lastRunCompiledAtMs >= runCompiledBatchWatchdogMs) {
      requestRunStateResync(roomID, runID)
      return
    }
    if (lastRunBatchAtMs <= 0) {
      requestRunStateResync(roomID, runID)
      return
    }
    if (now-lastRunBatchAtMs >= runBatchStallThresholdMs) {
      requestRunStateResync(roomID, runID)
    }
  }

  function createRunStateFromBatchSnapshot(roomID: number, snapshot?: BattleRunBatchPayload["n"]): BattleRunStatePayload {
    return parseRunStateLoose(roomID, (snapshot as Record<string, unknown>) || {})
  }

  function applyEventDeltas(prev: BattleRunStatePayload | null, events: BattleRunEventPayload[]): BattleRunStatePayload | null {
    if (!prev) return prev
    if (!Array.isArray(events) || events.length <= 0) return prev
    const next: BattleRunStatePayload = {
      roomId: prev.roomId,
      wave: prev.wave,
      completedBattles: prev.completedBattles,
      seq: prev.seq,
      order: Array.isArray(prev.order) ? prev.order.slice() : [],
      preparationOrder: Array.isArray(prev.preparationOrder) ? prev.preparationOrder.slice() : [],
      players: prev.players.map((unit) => ({ ...unit })),
      enemies: prev.enemies.map((unit) => ({ ...unit })),
      battlefieldEffects: (prev.battlefieldEffects || []).map((effect) => ({ ...effect, windows: { ...effect.windows } }))
    }
    const unitIndex = new Map<string, BattleUnit>()
    for (const unit of next.players) {
      const key = String(unit.unitId || "").trim()
      if (key) unitIndex.set(key, unit)
    }
    for (const unit of next.enemies) {
      const key = String(unit.unitId || "").trim()
      if (key) unitIndex.set(key, unit)
    }
    const removeUnitById = (list: BattleUnit[], unitId: string): BattleUnit[] =>
      list.filter((item) => String(item.unitId || "").trim() !== unitId)
    const relocateUnitBySide = (unit: BattleUnit) => {
      const unitId = String(unit.unitId || "").trim()
      if (!unitId) return
      const side = String(unit.side || "").trim().toLowerCase()
      if (side !== "player" && side !== "enemy") return
      const inPlayers = next.players.some((item) => String(item.unitId || "").trim() === unitId)
      const inEnemies = next.enemies.some((item) => String(item.unitId || "").trim() === unitId)
      if (side === "enemy") {
        if (inEnemies && !inPlayers) return
        if (inPlayers) {
          next.players = removeUnitById(next.players, unitId)
        }
        if (!inEnemies) {
          next.enemies = [...next.enemies, unit]
        }
        return
      }
      if (inPlayers && !inEnemies) return
      if (inEnemies) {
        next.enemies = removeUnitById(next.enemies, unitId)
      }
      if (!inPlayers) {
        next.players = [...next.players, unit]
      }
    }
    const rebalanceUnitsBySide = () => {
      const allUnits = [...next.players, ...next.enemies]
      next.players = []
      next.enemies = []
      for (const unit of allUnits) {
        const side = String(unit.side || "").trim().toLowerCase()
        if (side === "enemy") {
          next.enemies = [...next.enemies, unit]
        } else {
          next.players = [...next.players, unit]
        }
      }
    }
    const applyBattlefieldEffectState = (raw: unknown) => {
      const parsed = parseBattlefieldEffectStates(raw ? [raw] : [])
      if (parsed.length === 0) return
      const incoming = parsed[0]
      const current = next.battlefieldEffects || []
      const index = current.findIndex((effect) => effect.id === incoming.id)
      if (index >= 0) {
        current[index] = incoming
      } else {
        current.push(incoming)
      }
      next.battlefieldEffects = current
    }
    for (const event of events) {
      const type = String(event.type || "").trim()
      const data = normalizeBattleEventData(type, event.data || {})
      next.seq = Math.max(next.seq, Number(event.seq || 0))
      if (type === "battle.wave.spawn") {
        if (Array.isArray(data.players)) next.players = data.players.map((unit) => ({ ...unit }))
        if (Array.isArray(data.enemies)) next.enemies = data.enemies.map((unit) => ({ ...unit }))
        if (Array.isArray(data.battlefieldEffects)) {
          next.battlefieldEffects = parseBattlefieldEffectStates(data.battlefieldEffects)
        }
        rebalanceUnitsBySide()
        if (Array.isArray(data.order)) {
          next.order = data.order
            .map((value) => String(value || "").trim())
            .filter((value) => value.length > 0)
        }
        const wave = Number(data.wave || next.wave)
        if (Number.isFinite(wave) && wave > 0) next.wave = Math.trunc(wave)
        unitIndex.clear()
        for (const unit of next.players) {
          const key = String(unit.unitId || "").trim()
          if (key) unitIndex.set(key, unit)
        }
        for (const unit of next.enemies) {
          const key = String(unit.unitId || "").trim()
          if (key) unitIndex.set(key, unit)
        }
        continue
      }
      if (type === "battle.wave.win") {
        const completed = Number(data.completedBattles || next.completedBattles)
        if (Number.isFinite(completed) && completed >= 0) next.completedBattles = Math.trunc(completed)
        continue
      }
      if (type === "battle.timeline.node") {
        applyTimelineNodeDelta(next, unitIndex, data)
        continue
      }
      if (type === "battle.field.tick" || type === "battle.field.window" || type === "battle.field.trigger") {
        applyBattlefieldEffectState(data.state)
        if (type === "battle.field.trigger") {
          const targets = Array.isArray(data.targets) ? data.targets : []
          for (const target of targets) {
            const key = String(target.unitId || target.targetId || "").trim()
            const unit = key ? unitIndex.get(key) : undefined
            if (!unit) continue
            applyBattlefieldTargetResult(unit, target)
          }
        }
        continue
      }
      if (type !== "battle.turn.action" && type !== "battle.turn.item_use") continue
      if (Array.isArray(data.order)) {
        next.order = data.order
          .map((value) => String(value || "").trim())
          .filter(Boolean)
      }
      const actorKey = String(data.actorId || "").trim()
      if (actorKey) {
        const actor = unitIndex.get(actorKey)
        if (actor) {
          const actorHP = Number(data.actorHp)
          if (Number.isFinite(actorHP)) actor.hp = actorHP
          const actorMP = Number(data.actorMp)
          if (Number.isFinite(actorMP)) actor.mp = actorMP
          if (typeof data.actorAlive === "boolean") actor.alive = data.actorAlive
        }
      }
      const targets = Array.isArray(data.targets) ? data.targets : []
      for (const target of targets) {
        const key = String(target.targetId || "").trim()
        if (!key) continue
        const unit = unitIndex.get(key)
        if (!unit) continue
        const hp = Number(target.hp)
        if (Number.isFinite(hp)) unit.hp = hp
        const mp = Number(target.mp)
        if (Number.isFinite(mp)) unit.mp = mp
        if (typeof target.alive === "boolean") unit.alive = target.alive
      }
    }
    return next
  }

  function applyTimelineNodeDelta(next: BattleRunStatePayload, unitIndex: Map<string, BattleUnit>, data: BattleRunEventData) {
    const wave = Number(data.wave || next.wave)
    if (Number.isFinite(wave) && wave > 0) next.wave = Math.trunc(wave)
    if (Array.isArray(data.preparationOrder)) {
      next.preparationOrder = data.preparationOrder
        .map((value) => String(value || "").trim())
        .filter(Boolean)
    } else if (data.preparationTurn) {
      const ownerID = String((data.turnOwner || {}).unitId || "").trim()
      if (ownerID) {
        next.preparationOrder = (next.preparationOrder || []).filter((unitID) => unitID !== ownerID)
      }
    }
    if (Array.isArray(data.order)) {
      next.order = data.order
        .map((value) => String(value || "").trim())
        .filter(Boolean)
    }
    const owner = data.turnOwner || {}
    const ownerID = String(owner.unitId || "").trim()
    const ownerUnit = ownerID ? unitIndex.get(ownerID) : null
    const relocateUnitBySide = (unit: BattleUnit) => {
      const unitID = String(unit?.unitId || "").trim()
      if (!unitID) return
      const targetSide = normalizeBattleSideLoose(unit.side) || "player"
      const inPlayers = next.players.some((row) => String(row.unitId || "").trim() === unitID)
      const inEnemies = next.enemies.some((row) => String(row.unitId || "").trim() === unitID)
      if (targetSide === "enemy") {
        if (inEnemies && !inPlayers) return
        if (inPlayers) {
          next.players = next.players.filter((row) => String(row.unitId || "").trim() !== unitID)
        }
        if (!inEnemies) {
          next.enemies = [...next.enemies, unit]
        }
      } else {
        if (inPlayers && !inEnemies) return
        if (inEnemies) {
          next.enemies = next.enemies.filter((row) => String(row.unitId || "").trim() !== unitID)
        }
        if (!inPlayers) {
          next.players = [...next.players, unit]
        }
      }
      unitIndex.set(unitID, unit)
    }
    const applyUnitSidePayload = (unit: BattleUnit, payload: Record<string, unknown>, relocate = false) => {
      const side = normalizeBattleSideLoose(payload.side)
      if (side) {
        unit.side = side
        if (relocate) relocateUnitBySide(unit)
      }
      const baseSide = normalizeBattleSideLoose(payload.baseSide)
      if (baseSide) unit.baseSide = baseSide
      const perspectiveSide = normalizeBattleSideLoose(payload.perspectiveSide)
      if (perspectiveSide) unit.perspectiveSide = perspectiveSide
    }
    const steps = Array.isArray(data.steps) ? data.steps : []
    for (const step of steps) {
      const kind = String(step.kind || "").trim().toLowerCase()
      const stepBuffID = String(step.buffId || "").trim()
      if (kind === "summon") {
        applyTimelineSummonStep(next, unitIndex, step)
        continue
      }
      if (kind === "summon_expire") {
        const expireTargets = Array.isArray(step.targets) ? step.targets : []
        for (const target of expireTargets) {
          const unitId = String((target as Record<string, unknown>).unitId || "").trim()
          if (!unitId) continue
          next.players = next.players.filter((u) => String(u.unitId || "").trim() !== unitId)
          next.enemies = next.enemies.filter((u) => String(u.unitId || "").trim() !== unitId)
          unitIndex.delete(unitId)
        }
        continue
      }
      const targets = Array.isArray(step.targets) ? step.targets : []
      if ((kind === "buff" || kind === "buff_charge" || kind === "state_detach") && stepBuffID) {
        const buffPayload = step.buffPayload && typeof step.buffPayload === "object" && !Array.isArray(step.buffPayload)
          ? (step.buffPayload as Record<string, unknown>)
          : undefined
        const buffTurns = Number(step.buffDurationTurns)
        const nextRemainTurns = Number.isFinite(buffTurns) ? Math.trunc(buffTurns) : undefined
        const buffTriggered = Number(step.buffTriggered)
        const nextTriggered = Number.isFinite(buffTriggered) && buffTriggered >= 0 ? Math.trunc(buffTriggered) : undefined
        for (const target of targets) {
          const targetID = String(target.unitId || target.targetId || "").trim()
          if (!targetID) continue
          const unit = unitIndex.get(targetID)
          if (!unit) continue
          const current = Array.isArray(unit.buffs) ? unit.buffs.map((row) => ({ ...row })) : []
          const foundIdx = current.findIndex((row) => String(row.buffId || "").trim() === stepBuffID)
          if (kind === "state_detach") {
            unit.buffs = foundIdx >= 0 ? current.filter((_, idx) => idx !== foundIdx) : current
            if (stepBuffID === SHADOW_CROSS_BUFF_ID) {
              applyShadowCrossSideMigration(unit, false)
            }
            continue
          }
          const nextRow = {
            buffId: stepBuffID,
            remainTurns: nextRemainTurns ?? (foundIdx >= 0 ? current[foundIdx].remainTurns : undefined),
            payload: buffPayload ? { ...buffPayload } : (foundIdx >= 0 ? (current[foundIdx].payload || {}) : {}),
            triggered: nextTriggered ?? (foundIdx >= 0 ? current[foundIdx].triggered : undefined)
          }
          if (foundIdx >= 0) {
            current[foundIdx] = nextRow
          } else {
            current.push(nextRow)
          }
          unit.buffs = current
          if (kind === "buff" && stepBuffID === SHADOW_CROSS_BUFF_ID) {
            applyShadowCrossSideMigration(unit, true)
          }
        }
      }
      for (const target of targets) {
        const targetID = String(target.unitId || target.targetId || "").trim()
        if (!targetID) continue
        const unit = unitIndex.get(targetID)
        if (!unit) continue
        applyUnitSidePayload(unit, target as Record<string, unknown>, false)
        const hp = Number(target.hpAfter ?? target.hp)
        if (Number.isFinite(hp)) unit.hp = hp
        const mp = Number(target.mpAfter ?? target.mp)
        if (Number.isFinite(mp)) unit.mp = mp
        const maxHp = Number(target.maxHpAfter ?? target.maxHp)
        if (Number.isFinite(maxHp) && maxHp > 0) unit.maxHp = maxHp
        const maxMp = Number(target.maxMpAfter ?? target.maxMp)
        if (Number.isFinite(maxMp) && maxMp >= 0) unit.maxMp = maxMp
        if (typeof target.aliveAfter === "boolean") {
          unit.alive = target.aliveAfter
        } else if (typeof target.alive === "boolean") {
          unit.alive = target.alive
        }
        if (typeof target.corpseConsumed === "boolean") {
          unit.corpseConsumed = target.corpseConsumed
        }
        const position = String(target.toPosition || target.position || "").trim()
        if (position) unit.position = position
        const basePosition = String(target.basePosition || "").trim()
        if (basePosition) unit.basePosition = basePosition
        applyUnitSidePayload(unit, target as Record<string, unknown>, true)
        const stats = toNumberRecordLoose(target.stats)
        if (stats) {
          unit.stats = stats
          unit.attributes = { ...stats }
        }
        const baseStats = toNumberRecordLoose(target.baseStats)
        if (baseStats) unit.baseStats = baseStats
        const statModifiers = toNumberRecordLoose(target.statModifiers)
        if (statModifiers) unit.statModifiers = statModifiers
        const statSources = toNestedNumberRecordLoose(target.statSources)
        if (statSources) unit.statSources = statSources
        const buffs = toBattleBuffs(target.buffs)
        if (buffs) unit.buffs = buffs
      }
    }
    if (ownerUnit) {
      applyTimelineCooldownDelta(
        ownerUnit,
        steps,
        data.skillCooldowns,
        data.skillCooldownTurns,
        data.itemCooldowns
      )
    }
    const unitUpdates = Array.isArray(data.unitUpdates) ? data.unitUpdates : []
    for (const update of unitUpdates) {
      const targetID = String(update.unitId || "").trim()
      if (!targetID) continue
      const unit = unitIndex.get(targetID)
      if (!unit) continue
      applyUnitSidePayload(unit, update as Record<string, unknown>, false)
      const hp = Number(update.hpAfter)
      if (Number.isFinite(hp)) unit.hp = hp
      const mp = Number(update.mpAfter)
      if (Number.isFinite(mp)) unit.mp = mp
      const maxHp = Number(update.maxHpAfter)
      if (Number.isFinite(maxHp) && maxHp > 0) unit.maxHp = maxHp
      const maxMp = Number(update.maxMpAfter)
      if (Number.isFinite(maxMp) && maxMp >= 0) unit.maxMp = maxMp
      if (typeof update.aliveAfter === "boolean") {
        unit.alive = update.aliveAfter
      }
      if (typeof update.corpseConsumed === "boolean") {
        unit.corpseConsumed = update.corpseConsumed
      }
      const position = String(update.position || "").trim()
      if (position) unit.position = position
      const basePosition = String(update.basePosition || "").trim()
      if (basePosition) unit.basePosition = basePosition
      applyUnitSidePayload(unit, update as Record<string, unknown>, true)
      const stats = toNumberRecordLoose((update as Record<string, unknown>).stats)
      if (stats) {
        unit.stats = stats
        unit.attributes = { ...stats }
      }
      const baseStats = toNumberRecordLoose((update as Record<string, unknown>).baseStats)
      if (baseStats) unit.baseStats = baseStats
      const statModifiers = toNumberRecordLoose((update as Record<string, unknown>).statModifiers)
      if (statModifiers) unit.statModifiers = statModifiers
      const statSources = toNestedNumberRecordLoose((update as Record<string, unknown>).statSources)
      if (statSources) unit.statSources = statSources
      const buffs = toBattleBuffs((update as Record<string, unknown>).buffs)
      if (buffs) unit.buffs = buffs
    }
    const summary = (data.summary || {}) as Record<string, unknown>
    const actor = ((summary.actor || {}) as Record<string, unknown>)
    if (ownerUnit && actor) {
      const hp = Number(actor.hp)
      if (Number.isFinite(hp)) ownerUnit.hp = hp
      const mp = Number(actor.mp)
      if (Number.isFinite(mp)) ownerUnit.mp = mp
      if (typeof actor.alive === "boolean") ownerUnit.alive = actor.alive
    }
  }

  function applyShadowCrossSideMigration(unit: BattleUnit, active: boolean) {
    if (!unit) return
    const rememberedSide = normalizeBattleSideLoose((unit as Record<string, unknown>)._shadowCrossOriginalSide)
    const currentSide = normalizeBattleSideLoose(unit.side)
    const baseSide = normalizeBattleSideLoose(unit.baseSide)
    const originSide = normalizeBattleSideLoose(unit.originSide)
    if (active) {
      const fallbackSide = currentSide || baseSide || originSide || "player"
      ;(unit as Record<string, unknown>)._shadowCrossOriginalSide = fallbackSide
      unit.side = "enemy"
      return
    }
    const restoreSide = rememberedSide || baseSide || originSide || (currentSide === "enemy" ? "player" : currentSide) || "player"
    unit.side = restoreSide
    delete (unit as Record<string, unknown>)._shadowCrossOriginalSide
  }

  function toCooldownTurns(value: unknown) {
    const parsed = Number(value)
    if (!Number.isFinite(parsed)) return 0
    return Math.max(0, Math.trunc(parsed))
  }

  function toBattleBuffs(raw: unknown): BattleUnit["buffs"] | null {
    if (!Array.isArray(raw)) return null
    const rows = raw
      .map((item) => {
        const row = (item && typeof item === "object" ? item : {}) as Record<string, unknown>
        const buffId = String(row.buffId || "").trim()
        if (!buffId) return null
        let remainTurns = Number(row.remainTurns)
        if (!Number.isFinite(remainTurns)) {
          const legacyRemainMs = Number(row.remainMs)
          if (Number.isFinite(legacyRemainMs)) {
            if (legacyRemainMs < 0) {
              remainTurns = -1
            } else if (legacyRemainMs > 0) {
              remainTurns = Math.max(1, Math.ceil(legacyRemainMs / 1000))
            } else {
              remainTurns = 0
            }
          }
        }
        const payload = row.payload && typeof row.payload === "object" && !Array.isArray(row.payload)
          ? (row.payload as Record<string, unknown>)
          : {}
        const triggered = Number(row.triggered)
        return {
          buffId,
          remainTurns: Number.isFinite(remainTurns) ? Math.trunc(remainTurns) : undefined,
          payload,
          triggered: Number.isFinite(triggered) && triggered >= 0 ? Math.trunc(triggered) : undefined
        }
      })
      .filter((item): item is NonNullable<typeof item> => Boolean(item))
    return rows
  }

  function normalizedUnitCooldownMap(unit: BattleUnit) {
    const cooldowns: Record<string, number> = {}
    const fromMap = unit.skillCooldowns || {}
    for (const [rawSkillID, rawCooldown] of Object.entries(fromMap)) {
      const skillID = String(rawSkillID || "").trim()
      if (!skillID) continue
      cooldowns[skillID] = toCooldownTurns(rawCooldown)
    }
    const skills = Array.isArray(unit.skills) ? unit.skills : []
    for (const skill of skills) {
      const skillID = String(skill.id || "").trim()
      if (!skillID) continue
      if (Object.prototype.hasOwnProperty.call(cooldowns, skillID)) continue
      cooldowns[skillID] = toCooldownTurns(skill.cooldownRemain)
    }
    return cooldowns
  }

  function resolveCastSkillID(steps: NonNullable<BattleRunEventData["steps"]>) {
    for (const step of steps) {
      const kind = String(step.kind || "").trim().toLowerCase()
      if (kind !== "cast") continue
      const skillID = String(step.skill?.skillId || "").trim()
      if (!skillID) continue
      return skillID
    }
    return ""
  }

  function resolveSkillTotalCooldown(unit: BattleUnit, skillID: string) {
    const normalizedSkillID = String(skillID || "").trim()
    if (!normalizedSkillID) return 0
    const skills = Array.isArray(unit.skills) ? unit.skills : []
    const matched = skills.find((skill) => String(skill.id || "").trim() === normalizedSkillID)
    if (matched) {
      const cooldownTurns = toCooldownTurns(matched.cooldownTurns ?? matched.cooldown)
      if (cooldownTurns > 0) return cooldownTurns
    }
    return toCooldownTurns(getSkillDefinition(normalizedSkillID)?.cooldown)
  }

  function applyTimelineCooldownDelta(
    ownerUnit: BattleUnit,
    steps: NonNullable<BattleRunEventData["steps"]>,
    authoritativeCooldowns?: Record<string, number>,
    authoritativeTotals?: Record<string, number>,
    authoritativeItemCooldowns?: Record<string, number>
  ) {
    if (authoritativeItemCooldowns !== undefined) {
      ownerUnit.itemCooldowns = { ...authoritativeItemCooldowns }
      const items = Array.isArray(ownerUnit.battleItems) ? ownerUnit.battleItems : []
      ownerUnit.battleItems = items.map((item) => {
        const itemID = String(item.itemId || "").trim()
        if (!itemID || !Object.prototype.hasOwnProperty.call(authoritativeItemCooldowns, itemID)) {
          return { ...item }
        }
        return { ...item, cooldownRemain: authoritativeItemCooldowns[itemID] }
      })
    }
    if (authoritativeCooldowns !== undefined) {
      const cooldowns = { ...authoritativeCooldowns }
      ownerUnit.skillCooldowns = cooldowns
      if (authoritativeTotals !== undefined) {
        ownerUnit.skillCooldownTurns = { ...authoritativeTotals }
      }
      const skills = Array.isArray(ownerUnit.skills) ? ownerUnit.skills : []
      ownerUnit.skills = skills.map((skill) => {
        const skillID = String(skill.id || "").trim()
        if (!skillID) return { ...skill }
        const next = { ...skill }
        if (Object.prototype.hasOwnProperty.call(cooldowns, skillID)) {
          next.cooldownRemain = cooldowns[skillID]
        }
        if (authoritativeTotals && Object.prototype.hasOwnProperty.call(authoritativeTotals, skillID)) {
          next.cooldown = authoritativeTotals[skillID]
          next.cooldownTurns = authoritativeTotals[skillID]
        }
        return next
      })
      return
    }

    const cooldowns = normalizedUnitCooldownMap(ownerUnit)
    const castSkillID = resolveCastSkillID(steps)
    if (castSkillID) {
      cooldowns[castSkillID] = resolveSkillTotalCooldown(ownerUnit, castSkillID)
    }
    for (const [skillID, remain] of Object.entries(cooldowns)) {
      cooldowns[skillID] = Math.max(0, toCooldownTurns(remain) - 1)
    }
    ownerUnit.skillCooldowns = cooldowns
    const skills = Array.isArray(ownerUnit.skills) ? ownerUnit.skills : []
    if (skills.length > 0) {
      ownerUnit.skills = skills.map((skill) => {
        const skillID = String(skill.id || "").trim()
        if (!skillID) return { ...skill }
        const cooldownRemain = Object.prototype.hasOwnProperty.call(cooldowns, skillID)
          ? cooldowns[skillID]
          : toCooldownTurns(skill.cooldownRemain)
        return {
          ...skill,
          cooldownRemain
        }
      })
    }
  }

  function applyTimelineSummonStep(next: BattleRunStatePayload, unitIndex: Map<string, BattleUnit>, step: NonNullable<BattleRunEventData["steps"]>[number]) {
    const summon = (step.summon || {}) as Record<string, unknown>
    const unitID = String(summon.unitId || "").trim()
    if (!unitID || unitIndex.has(unitID)) return
    const side = String(summon.side || "player").trim().toLowerCase()
    const summonTemplateUnitId = String(summon.templateUnitId || "").trim() || undefined
    const unit: BattleUnit = {
      unitId: unitID,
      side,
      name: String(summon.name || "").trim(),
      nameKey: String(summon.nameKey || "").trim(),
      position: String(summon.position || "vanguard").trim(),
      hp: Number(summon.hp || 0),
      mp: Number(summon.mp || 0),
      maxHp: Number(summon.maxHp || summon.hp || 0),
      maxMp: Number(summon.maxMp || summon.mp || 0),
      alive: true,
      skillIds: [],
      skillCooldowns: {},
      buffs: [],
      summonTemplateUnitId,
    }
    if (side === "enemy") {
      next.enemies = [...next.enemies, unit]
    } else {
      next.players = [...next.players, unit]
    }
    unitIndex.set(unitID, unit)
  }

  function clearEnteringWarnTimer() {
    if (enteringWarnTimer) {
      clearTimeout(enteringWarnTimer)
      enteringWarnTimer = null
    }
  }

  function enteringWarnDelayMs(detail: BattleRoomDetail) {
    const world = worlds.value.find((item) => item.id === detail.room.dungeonId)
    const entryMs = Math.max(0, Number(world?.entryDurationSeconds || 0)) * 1000
    const graceMs = 10000
    const thresholdMs = entryMs + graceMs
    const startedAt = detail.room.enteringStartedAt ? new Date(detail.room.enteringStartedAt).getTime() : 0
    if (!Number.isFinite(startedAt) || startedAt <= 0) return thresholdMs
    const elapsed = Date.now() - startedAt
    return Math.max(0, thresholdMs - elapsed)
  }

  function scheduleEnteringDelayedWarning(detail: BattleRoomDetail) {
    clearEnteringWarnTimer()
    if (detail.room.status !== "entering") {
      return
    }
    if (enteringWarnedRoomId.value === detail.room.id) {
      return
    }
    const delay = enteringWarnDelayMs(detail)
    enteringWarnTimer = setTimeout(() => {
      if (myRoom.value?.room.id !== detail.room.id || myRoom.value?.room.status !== "entering") return
      enteringWarnedRoomId.value = detail.room.id
      ElMessage.warning(t("game.battleEnteringDelayed"))
    }, delay)
  }

  function getTrackedRoomDetail(roomID: number | null) {
    if (!roomID) return null
    if (selectedRoom.value?.room.id === roomID) return selectedRoom.value
    if (myRoomDetail.value?.room.id === roomID) return myRoomDetail.value
    return null
  }

  function resolveWorldMaxTeamSize(worldID: string) {
    const normalized = String(worldID || "").trim()
    if (!normalized) return 3
    const world = worlds.value.find((item) => item.id === normalized)
    return Math.max(1, Math.trunc(Number(world?.maxTeamSize || 3)))
  }

  function upsertPublicRoom(room: BattleRoom) {
    const list = publicRooms.value.slice()
    const idx = list.findIndex((item) => item.id === room.id)
    if (idx >= 0) {
      list[idx] = { ...list[idx], ...room }
    } else {
      list.unshift(room)
    }
    publicRooms.value = list
  }

  onMounted(() => {
    store.restoreToken()
    registerWSBindings()
    if (!runBatchWatchTimer) {
      runBatchWatchTimer = setInterval(monitorRunBatchStall, 3000)
    }
  })

  onUnmounted(() => {
    clearEnteringWarnTimer()
    if (runBatchWatchTimer) {
      clearInterval(runBatchWatchTimer)
      runBatchWatchTimer = null
    }
    for (const unbind of wsUnbinders.splice(0)) {
      unbind()
    }
  })

  watch(worlds, (next) => {
    if (next.length === 0) {
      selectedWorldID.value = ""
      return
    }
    if (selectedWorldID.value && !next.some((world) => world.id === selectedWorldID.value)) {
      selectedWorldID.value = ""
    }
  }, { immediate: true })

  watch(selectedWorldID, () => {
    const worldID = selectedWorldID.value.trim()
    if (worldID && !createRoomName.value.trim()) {
      createRoomName.value = selectedWorld.value?.label || selectedWorldID.value
    }
    if (store.token) {
      void loadRooms()
    }
  })

  async function loadRooms() {
    if (!store.token) {
      roomsLoading.value = false
      return
    }
    const requestSeq = ++loadRoomsRequestSeq
    roomsLoading.value = true
    const query = selectedWorldID.value ? `?dungeonId=${encodeURIComponent(selectedWorldID.value.trim())}` : ""
    try {
      const data = await apiGet<BattleRoomListPayload>(`/api/battle/rooms${query}`, store.token)
      if (requestSeq !== loadRoomsRequestSeq) return
      publicRooms.value = data.items || []
      myRoom.value = data.mine || null
      if (myRoom.value?.room?.id) {
        chatStore.syncBattleRoomChannel(myRoom.value.room.id, myRoom.value.room.roomName)
      } else {
        chatStore.clearBattleRoomChannels()
      }
      if (myRoom.value?.room.status !== "entering") {
        enteringWarnedRoomId.value = null
      }
      if (myRoom.value) {
        scheduleEnteringDelayedWarning(myRoom.value)
      } else {
        clearEnteringWarnTimer()
      }
      realtimeStore.setBattleRoomStatus(myRoom.value?.room?.status || null)

      const selectedID = selectedRoomID.value
      const selectedStillVisible = !!selectedID && (
        myRoom.value?.room.id === selectedID || publicRooms.value.some((room) => room.id === selectedID)
      )

      if (!selectedStillVisible) {
        if (myRoom.value?.room.id) {
          selectedRoomID.value = myRoom.value.room.id
          selectedRoom.value = myRoom.value
          await loadRun(myRoom.value.room.id)
          return
        }
        if (publicRooms.value.length > 0) {
          selectedRoomID.value = publicRooms.value[0].id
          await loadRoomDetail(publicRooms.value[0].id)
          return
        }
        selectedRoomID.value = null
        selectedRoom.value = null
        clearBattleState()
        return
      }

      if (selectedRoomID.value) {
        if (myRoom.value?.room.id === selectedRoomID.value) {
          selectedRoom.value = myRoom.value
          if (myRoom.value.room.status === "playing") {
            await loadRun(selectedRoomID.value)
          } else {
            clearBattleState()
          }
        } else {
          await loadRoomDetail(selectedRoomID.value)
        }
      } else {
        selectedRoom.value = null
        clearBattleState()
      }
    } catch (err) {
      if (requestSeq === loadRoomsRequestSeq) {
        ElMessage.error(resolveErrorMessage(t, err, "game.battleRoomLoadFailed"))
      }
    } finally {
      if (requestSeq === loadRoomsRequestSeq) {
        roomsLoading.value = false
      }
    }
  }

  async function loadRoomDetail(roomID: number) {
    if (!store.token || roomID <= 0) return
    try {
      const detail = await apiGet<BattleRoomDetail>(`/api/battle/rooms/${roomID}`, store.token)
      selectedRoom.value = detail
      selectedRoomID.value = roomID
      await loadRun(roomID)
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.battleRoomLoadFailed"))
    }
  }

  function loadRun(roomID: number, expectedRunID = Number(myRoom.value?.room.currentRunId || 0)): Promise<void> {
    if (!store.token || roomID <= 0) {
      runLoading.value = false
      return Promise.resolve()
    }
    const myRoomID = Number(myRoom.value?.room.id || 0)
    if (!myRoomID || myRoomID !== roomID) {
      clearBattleState()
      return Promise.resolve()
    }
    if (runLoadFlight?.roomID === roomID && runLoadFlight.runID === expectedRunID) {
      return runLoadFlight.promise
    }
    const promise = loadRunOnce(roomID)
    runLoadFlight = { roomID, runID: expectedRunID, promise }
    void promise.then(
      () => {
        if (runLoadFlight?.promise === promise) runLoadFlight = null
      },
      () => {
        if (runLoadFlight?.promise === promise) runLoadFlight = null
      }
    )
    return promise
  }

  async function loadRunOnce(roomID: number) {
    const requestSeq = ++loadRunRequestSeq
    runLoading.value = true
    try {
      const payload = await apiGet<BattleRunPayload>(`/api/battle/rooms/${roomID}/run`, store.token)
      if (requestSeq !== loadRunRequestSeq) return
      const roomStatus = String(selectedRoom.value?.room.status || "")
      const canKeepRunningState = roomStatus === "playing" && runState.value?.roomId === roomID
      if (payload.run?.state) {
        const state = payload.run.state as unknown as Record<string, unknown>
        const parsedState = parseRunStateLoose(roomID, state)
        const hasRenderableState = parsedState.players.length > 0 || parsedState.enemies.length > 0 || Number(parsedState.seq || 0) > 0
        if (!hasRenderableState && canKeepRunningState) return
        const preserveCurrentState = shouldPreserveBattleRunSnapshot({
          currentRunId: Number(selectedRun.value?.id || 0),
          currentStatus: String(selectedRun.value?.status || ""),
          currentSeq: Number(runState.value?.seq || 0),
          incomingRunId: Number(payload.run.id || 0),
          incomingStatus: String(payload.run.status || ""),
          incomingSeq: Number(parsedState.seq || 0)
        })
        if (!preserveCurrentState) {
          selectedRun.value = payload.run
          runState.value = parsedState
          runEvents.value = Array.isArray(state.events)
            ? state.events
                .slice(-120)
                .map((event) => {
                  const type = String(event.type || "").trim()
                  return {
                    ...event,
                    data: normalizeBattleEventData(type, event.data || {})
                  }
                })
            : []
        }
        if (payload.run.status === "playing" && hasRenderableState) {
          completeBattleTransition(payload.run.id)
        }
      } else if (!canKeepRunningState) {
        selectedRun.value = null
        runState.value = null
        runEvents.value = []
      }
    } catch (err) {
      if (requestSeq === loadRunRequestSeq) {
        // Keep the last playable snapshot and transition marker. A websocket
        // batch can still advance the run after this HTTP resync fails.
        ElMessage.error(resolveErrorMessage(t, err, "game.battleRoomLoadFailed"))
      }
    } finally {
      if (requestSeq === loadRunRequestSeq) {
        runLoading.value = false
        clearRunBootstrapState(roomID)
      }
    }
  }

  function registerWSBindings() {
    wsUnbinders.push(bind("battle.room.updated", (payload) => {
      const data = payload as BattleRoomDetail
      if (!data?.room?.id) return
      const prevSelectedStatus = selectedRoom.value?.room.id === data.room.id ? String(selectedRoom.value?.room.status || "") : ""
      if (selectedRoomID.value === data.room.id) {
        selectedRoom.value = data
      }
      if (myRoom.value?.room.id === data.room.id) {
        myRoom.value = data
      }
      if (selectedRoomID.value === data.room.id && prevSelectedStatus !== "playing" && data.room.status === "playing") {
        void loadRun(data.room.id)
      }
      upsertPublicRoom(data.room)
    }))

    wsUnbinders.push(bind("battle.room.self", (payload) => {
      const data = payload as BattleRoomDetail
      if (!data?.room?.id) {
        const prevMyRoomID = Number(myRoom.value?.room.id || 0)
        myRoom.value = null
        realtimeStore.setBattleRoomStatus(null)
        clearEnteringWarnTimer()
        if (prevMyRoomID > 0 && selectedRoomID.value === prevMyRoomID) {
          selectedRoomID.value = null
          selectedRoom.value = null
          clearBattleState()
        }
        return
      }
      const prevStatus = myRoom.value?.room.status || ""
      realtimeStore.setBattleRoomStatus(data.room.status || null)
      myRoom.value = data
      if (prevStatus !== "entering" && data.room.status === "entering") {
        enteringWarnedRoomId.value = null
        scheduleEnteringDelayedWarning(data)
        ElMessage.info(t("game.battleEnteringArea"))
      } else if (prevStatus === "entering" && data.room.status === "playing") {
        clearEnteringWarnTimer()
        void loadRun(data.room.id)
      } else if (data.room.status !== "entering") {
        clearEnteringWarnTimer()
      }
      if (selectedRoomID.value === data.room.id) {
        selectedRoom.value = data
      }
      if (!selectedRoomID.value) {
        selectedRoomID.value = data.room.id
        selectedRoom.value = data
        void loadRun(data.room.id)
      }
    }))

    wsUnbinders.push(bind("battle.run.batch", (payload) => {
      const data = payload as BattleRunBatchPayload
      if (battlePayloadStreamKind(data) !== "room") return
      const roomID = Number(data?.m || 0)
      if (!roomID) return
      const myRoomID = Number(myRoom.value?.room.id || 0)
      if (myRoomID > 0 && roomID !== myRoomID) return
      const runID = Number(data?.r || 0)
      if (!runID) return
      const runSeqBeforeBatch = Number(runState.value?.seq || 0)
      lastRunBatchAtMs = Date.now()
      lastRunCompiledAtMs = 0
      const hasSnapshot = hasBatchSnapshot(data)
      const snapshotState = hasSnapshot ? createRunStateFromBatchSnapshot(roomID, data?.n) : null
      let usedSnapshotAsBase = false
      if (!hasRunMainState(roomID, runID)) {
        if (!hasSnapshot) {
          requestRunStateResync(roomID, runID)
          return
        }
        const baseState = snapshotState || createRunStateFromBatchSnapshot(roomID, data?.n)
        selectedRun.value = {
          id: runID,
          roomId: roomID,
          wave: Number(baseState.wave || runState.value?.wave || 1),
          status: "playing",
          state: baseState
        }
        runState.value = baseState
        runEvents.value = []
        usedSnapshotAsBase = true
      } else if (!runState.value) {
        if (!hasSnapshot) {
          requestRunStateResync(roomID, runID)
          return
        }
        runState.value = snapshotState || createRunStateFromBatchSnapshot(roomID, data?.n)
        usedSnapshotAsBase = true
      } else if (snapshotState && Number(snapshotState.seq || 0) >= Number(runState.value.seq || 0)) {
        runState.value = mergeSnapshotIntoRunState(runState.value, snapshotState)
        usedSnapshotAsBase = true
      }
      const nextEvents: BattleRunEventPayload[] = Array.isArray(data.v)
        ? data.v.map((event) => {
          const type = expandBattleEventType(event.e)
          return {
            seq: Number(event.q || 0),
            type,
            ts: Number(Number(data.s || 0) + Number(event.o || 0)),
            data: normalizeBattleEventData(type, event.d || {})
          }
        })
        : []
      if (!runState.value) {
        requestRunStateResync(roomID, runID)
        return
      }
      // Core NATS delivery is at-least-once when publish confirmation times out.
      // Apply only events newer than the authoritative in-memory cursor, while
      // retaining the full batch below for replay de-duplication.
      let eventsToApply = filterBatchEventsAfterSnapshot(nextEvents, Number(runState.value.seq || 0))
      if (!usedSnapshotAsBase && nextEvents.length > 0) {
        const minIncomingSeq = Math.min(...nextEvents.map((event) => Number(event.seq || 0)).filter((seq) => Number.isFinite(seq) && seq > 0))
        if (Number.isFinite(minIncomingSeq) && minIncomingSeq > Number(runState.value.seq || 0) + 1) {
          if (hasSnapshot) {
            runState.value = createRunStateFromBatchSnapshot(roomID, data.n)
            eventsToApply = filterBatchEventsAfterSnapshot(nextEvents, Number(runState.value.seq || 0))
          } else {
            requestRunStateResync(roomID, runID)
            // Do not drop this batch even when seq has a gap:
            // keep replay continuity and let async resync correct drift.
            eventsToApply = nextEvents
          }
        }
      }
      if (eventsToApply.length > 0) {
        runState.value = applyEventDeltas(runState.value, eventsToApply)
      }
      if (nextEvents.length > 0) {
        runEvents.value = mergeReplayEvents(runEvents.value, nextEvents)
      }
      if (selectedRun.value?.id === runID && selectedRun.value.status === "compiled") {
        selectedRun.value = { ...selectedRun.value, status: "playing" }
      }
      completeBattleTransition(runID)
      // console.log("[battle.run.batch][parsed]", {
      //   roomID,
      //   runID,
      //   hasSnapshot,
      //   usedSnapshotAsBase,
      //   snapshotSeq: Number(snapshotState?.seq || 0),
      //   runSeqBeforeBatch,
      //   runSeqAfterBatch: Number(runState.value?.seq || 0),
      //   rawEventCount: nextEvents.length,
      //   appliedEventCount: eventsToApply.length,
      //   nextEvents,
      //   eventsToApply
      // })
    }))

    wsUnbinders.push(bind("battle.run.started", (payload) => {
      const data = payload as { roomId?: number; runId?: number }
      if (battlePayloadStreamKind(payload) !== "room") return
      const roomID = Number(data?.roomId || 0)
      const runID = Number(data?.runId || 0)
      if (!roomID || !runID) return
      if (selectedRoomID.value === roomID) {
        const currentRunID = Number(selectedRun.value?.id || 0)
        if (currentRunID === runID && selectedRun.value?.status !== "compiled") return
        const nextWait = markBattleRunPreparing(
          interBattleWait.value,
          Date.now(),
          runID,
          currentRunID
        )
        if (nextWait === interBattleWait.value) return
        interBattleWait.value = nextWait
        void loadRun(roomID, runID)
      }
    }))

    wsUnbinders.push(bind("battle.run.compiled", (payload) => {
      const data = payload as { roomId?: number; runId?: number }
      if (battlePayloadStreamKind(payload) !== "room") return
      if (!data?.roomId) return
      if (selectedRoomID.value !== data.roomId) return
      lastRunCompiledAtMs = Date.now()
      void loadRun(data.roomId, Number(data.runId || 0))
    }))

    wsUnbinders.push(bind("battle.run.preparing_next", (payload) => {
      const data = payload as { roomId?: number }
      if (!data?.roomId || selectedRoomID.value !== data.roomId) return
      // Keep replay continuity for the ending wave.
      // Room/run snapshots will be refreshed by subsequent ws push events.
    }))

    wsUnbinders.push(bind("battle.run.finished", (payload) => {
      const data = payload as { roomId?: number; runId?: number; interBattleIntervalMs?: number }
      if (battlePayloadStreamKind(payload) !== "room") return
      const roomID = Number(data?.roomId || 0)
      if (!roomID || selectedRoomID.value !== roomID) return
      const currentRunID = Number(selectedRun.value?.id || 0)
      const finishedRunID = Number(data.runId || currentRunID || 0)
      if (currentRunID > 0 && finishedRunID > 0 && currentRunID !== finishedRunID) return
      // Keep final replay continuity: room/run updates are handled by subsequent ws events.
      interBattleWait.value = {
        finishedAtMs: Date.now(),
        intervalMs: Math.max(0, Number(data.interBattleIntervalMs || 0)),
        finishedRunId: Math.max(0, finishedRunID),
        pendingRunId: 0
      }
    }))

    wsUnbinders.push(bind("battle.room.kicked", (payload) => {
      const data = payload as { roomId?: number; kickedUserId?: number; kickedBy?: number }
      if (!data?.roomId) return
      if (store.userId > 0 && Number(data.kickedUserId || 0) > 0 && Number(data.kickedUserId || 0) !== store.userId) return
      chatStore.clearBattleRoomChannels()
      if (selectedRoomID.value === data.roomId) {
        selectedRoomID.value = null
        selectedRoom.value = null
        clearBattleState()
      }
      if (myRoom.value?.room.id === data.roomId) {
        myRoom.value = null
        realtimeStore.setBattleRoomStatus(null)
      }
      kickedRoomID.value = data.roomId
      void loadRooms()
      ElMessage.warning(t("game.battleRoomKickedNotice"))
    }))

    wsUnbinders.push(bind("fog_mode_disabled", (payload) => {
      const data = payload as { roomId?: number; ownerId?: number; reason?: string }
      if (!data?.roomId) return
      if (myRoom.value?.room.id !== data.roomId && selectedRoomID.value !== data.roomId) return
      const isOwner = store.userId > 0 && Number(data.ownerId || 0) === store.userId
      ElNotification({
        type: "warning",
        title: t("game.fogMode"),
        message: isOwner
          ? t("game.fogModeDisabledInsufficientItems")
          : t("game.fogModeDisabledNotice"),
        duration: 6000
      })
    }))
  }

  async function createRoom() {
    if (!store.token) return null
    if (!selectedWorldID.value.trim()) {
      ElMessage.warning(t("game.battleAreaMissing"))
      return null
    }
    if (!createRoomName.value.trim()) {
      ElMessage.warning(t("game.battleRoomName"))
      return null
    }
    try {
      const detail = await apiPost<BattleRoomDetail>("/api/battle/rooms", store.token, {
        dungeonId: selectedWorldID.value.trim(),
        roomName: createRoomName.value.trim(),
        autoReenterOnDefeat: createAutoReenterOnDefeat.value,
        isPublic: createIsPublic.value,
        isFogMode: createIsFogMode.value,
        position: createPosition.value,
        difficultyLevel: createDifficultyLevel.value
      })
      selectedRoomID.value = detail.room.id
      selectedRoom.value = detail
      myRoom.value = detail
      realtimeStore.setBattleRoomStatus(detail.room.status || null)
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
      return detail
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
      return null
    }
  }

  async function joinRoom(roomID: number) {
    if (!store.token || roomID <= 0) return null
    const room = publicRooms.value.find((item) => item.id === roomID)
    const maxTeamSize = resolveWorldMaxTeamSize(room?.dungeonId || "")
    if (room && Number(room.memberCount || 0) >= maxTeamSize) {
      ElMessage.warning(t("game.battleRoomMaxTeamSizeExceeded", { max: maxTeamSize }))
      return null
    }
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/join`, store.token, { position: "vanguard" })
      selectedRoomID.value = roomID
      selectedRoom.value = detail
      myRoom.value = detail
      realtimeStore.setBattleRoomStatus(detail.room.status || null)
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
      return detail
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
      return null
    }
  }

  async function leaveRoom(roomID: number | null = selectedRoomID.value) {
    if (!store.token || !roomID) return
    try {
      await apiPost(`/api/battle/rooms/${roomID}/leave`, store.token, {})
      if (selectedRoomID.value === roomID) {
        selectedRoomID.value = null
        selectedRoom.value = null
        clearBattleState()
      }
      if (myRoom.value?.room.id === roomID) {
        myRoom.value = null
        realtimeStore.setBattleRoomStatus(null)
      }
      chatStore.clearBattleRoomChannels()
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  async function toggleReady(roomID: number | null = selectedRoomID.value) {
    if (!store.token || !roomID) return
    const detail = getTrackedRoomDetail(roomID)
    if (!detail) return
    const member = detail.members.find((item) => item.userId === store.userId) || null
    if (!member) return
    try {
      const ready = !member.isReady
      const payload = await apiPost<{ room: BattleRoomDetail }>(`/api/battle/rooms/${roomID}/ready`, store.token, { ready })
      if (selectedRoomID.value === roomID) {
        selectedRoom.value = payload.room
      }
      if (myRoom.value?.room.id === roomID) {
        myRoom.value = payload.room
      }
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  async function updateRoomPosition(position: string, roomID: number | null = selectedRoomID.value ?? myRoomDetail.value?.room.id ?? null) {
    if (!store.token || !roomID) return
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/position`, store.token, { position })
      if (selectedRoomID.value === roomID) {
        selectedRoom.value = detail
      }
      if (myRoom.value?.room.id === roomID) {
        myRoom.value = detail
      }
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  async function addRoomKitty(kittyID: number, position = "vanguard", roomID: number | null = myRoomDetail.value?.room.id ?? selectedRoomID.value) {
    if (!store.token || !roomID || kittyID <= 0) return null
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/kitty/add`, store.token, {
        kittyId: kittyID,
        position
      })
      if (selectedRoomID.value === roomID) selectedRoom.value = detail
      if (myRoom.value?.room.id === roomID) myRoom.value = detail
      await loadRooms()
      const updatedKitty = (detail.units || []).find((unit) => unit.unitType === "kitty" && Number(unit.kittyId || 0) === kittyID)
      const requestedPosition = normalizeBattlePosition(position)
      const resolvedPosition = normalizeBattlePosition(updatedKitty?.position)
      if (
        updatedKitty?.positionLockedByPersonalityId &&
        resolvedPosition &&
        requestedPosition &&
        resolvedPosition !== requestedPosition
      ) {
        const personality = personalityLabel(
          updatedKitty.positionLockedByPersonalityId,
          updatedKitty.positionLockedByPersonalityKey
        )
        ElMessage.warning(t("game.battleRoomKittyPositionLockedToast", {
          personality,
          position: positionLabel(resolvedPosition)
        }))
      } else {
        ElMessage.success(t("game.saveSuccess"))
      }
      return detail
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
      return null
    }
  }

  async function removeRoomKitty(kittyID: number, roomID: number | null = myRoomDetail.value?.room.id ?? selectedRoomID.value) {
    if (!store.token || !roomID || kittyID <= 0) return null
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/kitty/remove`, store.token, {
        kittyId: kittyID
      })
      if (selectedRoomID.value === roomID) selectedRoom.value = detail
      if (myRoom.value?.room.id === roomID) myRoom.value = detail
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
      return detail
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
      return null
    }
  }

  async function endBattle(roomID: number | null = selectedRoomID.value) {
    if (!store.token || !roomID) return
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/end`, store.token, {})
      if (selectedRoomID.value === roomID) {
        selectedRoom.value = detail
      }
      if (myRoom.value?.room.id === roomID) {
        myRoom.value = detail
      }
      realtimeStore.setBattleRoomStatus(detail.room.status || null)
      await loadRun(roomID)
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  async function applyRoomConfig(roomID: number | null = selectedRoom.value?.room.id ?? myRoomDetail.value?.room.id ?? null) {
    if (!store.token || !roomID) return
    const detail = getTrackedRoomDetail(roomID)
    if (!detail) return
    const maxTeamSize = resolveWorldMaxTeamSize(detail.room.dungeonId)
    if (detail.members.length > maxTeamSize) {
      ElMessage.error(t("game.battleRoomMaxTeamSizeExceeded", { max: maxTeamSize }))
      return
    }
    try {
      const next = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/config`, store.token, {
        dungeonId: detail.room.dungeonId,
        roomName: detail.room.roomName,
        autoReenterOnDefeat: detail.room.autoReenterOnDefeat,
        disableEntryRound: detail.room.disableEntryRound,
        isPublic: detail.room.isPublic,
        isFogMode: detail.room.isFogMode,
        difficultyLevel: detail.room.difficultyLevel
      })
      if (selectedRoomID.value === roomID) {
        selectedRoom.value = next
      }
      if (myRoom.value?.room.id === roomID) myRoom.value = next
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  function selectRoom(roomID: number) {
    if (!roomID) return
    if (selectedRoomID.value === roomID) {
      if (!selectedRoom.value && myRoom.value?.room.id === roomID) {
        selectedRoom.value = myRoom.value
      }
      const currentRunID = Number(myRoom.value?.room.id === roomID ? myRoom.value.room.currentRunId || 0 : 0)
      const selectedRunID = Number(selectedRun.value?.id || 0)
      if (currentRunID > 0 && (currentRunID !== selectedRunID || !runState.value)) {
        void loadRun(roomID)
      }
      return
    }
    selectedRoomID.value = roomID
    clearBattleState()
    if (myRoom.value?.room.id === roomID) {
      selectedRoom.value = myRoom.value
      if (myRoom.value.room.status === "playing") {
        void loadRun(roomID)
      }
      return
    }
    selectedRoom.value = null
    void loadRoomDetail(roomID)
  }

  function selectMyRoom() {
    if (!myRoom.value?.room.id) return
    selectRoom(myRoom.value.room.id)
  }

  async function kickRoomMember(targetUserID: number, roomID: number | null = myRoomDetail.value?.room.id ?? selectedRoomID.value) {
    if (!store.token || !roomID || targetUserID <= 0) return
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/kick`, store.token, { targetUserId: targetUserID })
      if (selectedRoomID.value === roomID) {
        selectedRoom.value = detail
      }
      if (myRoom.value?.room.id === roomID) {
        myRoom.value = detail
      }
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  async function transferRoomOwner(targetUserID: number, roomID: number | null = myRoomDetail.value?.room.id ?? selectedRoomID.value) {
    if (!store.token || !roomID || targetUserID <= 0) return
    try {
      const detail = await apiPost<BattleRoomDetail>(`/api/battle/rooms/${roomID}/owner/transfer`, store.token, { targetUserId: targetUserID })
      if (selectedRoomID.value === roomID) {
        selectedRoom.value = detail
      }
      if (myRoom.value?.room.id === roomID) {
        myRoom.value = detail
      }
      await loadRooms()
      ElMessage.success(t("game.saveSuccess"))
    } catch (err) {
      ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
    }
  }

  return {
    selectedWorldID,
    selectedWorld,
    selectedWorldCosts,
    worlds,
    createRoomName,
    createAutoReenterOnDefeat,
    createIsPublic,
    createIsFogMode,
    createPosition,
    createDifficultyLevel,
    publicRooms,
    myRoom,
    myRoomDetail,
    selectedRoomID,
    selectedRoom,
    selectedRun,
    runState,
    runEvents,
    roomsLoading,
    runLoading,
    kickedRoomID,
    interBattleWait,
    isRoomOwner,
    isMyRoomOwner,
    isInSelectedRoom,
    myMember,
    myRoomMember,
    roomStatusLabel,
    runStatusLabel,
    positionLabel,
    loadRooms,
    createRoom,
    joinRoom,
    leaveRoom,
    toggleReady,
    updateRoomPosition,
    addRoomKitty,
    removeRoomKitty,
    kickRoomMember,
    transferRoomOwner,
    endBattle,
    applyRoomConfig,
    selectRoom,
    selectMyRoom,
    resolveWorldMaxTeamSize
  }
}
