import type {
  BattleRunEventPayload,
  BattleRunStatePayload,
  BattleUnit
} from "@composables/game/combat/useCombatPage"

export type ArenaReplaySide = "player" | "enemy" | ""

export type ArenaReplayQueuedEvent = {
  offsetMs: number
  event: BattleRunEventPayload
}

export type ArenaReplayTurnSummary = {
  index: number
  eventStart: number
  eventEnd: number
  seq: number
  offsetMs: number
  actorId: string
  actorName: string
  side: ArenaReplaySide
  skillId: string
  skillNameFallback: string
  wave: number
  preparationTurn: boolean
  extraTurn: boolean
  syncedTurn: boolean
  damage: number
  healing: number
  criticalHits: number
  knockouts: number
  statusChanges: number
  keyMoment: boolean
}

export type ArenaReplayActorContribution = {
  actorId: string
  actorName: string
  side: ArenaReplaySide
  turns: number
  damage: number
  healing: number
  criticalHits: number
  knockouts: number
}

export type ArenaReplaySideContribution = {
  damage: number
  healing: number
  criticalHits: number
  knockouts: number
}

export type ArenaReplayAnalysis = {
  turns: ArenaReplayTurnSummary[]
  actors: ArenaReplayActorContribution[]
  player: ArenaReplaySideContribution
  enemy: ArenaReplaySideContribution
}

type MutableTurn = Omit<ArenaReplayTurnSummary, "index" | "eventEnd" | "keyMoment">
type MutableContribution = ArenaReplayActorContribution

const STATUS_STEP_KINDS = new Set(["buff", "buff_charge", "state_detach", "position_move", "stat_recalc"])

export function buildArenaReplayAnalysis(
  events: ArenaReplayQueuedEvent[],
  snapshot: BattleRunStatePayload | null
): ArenaReplayAnalysis {
  const rows = Array.isArray(events) ? events : []
  const units = collectReplayUnits(rows, snapshot)
  const drafts: Array<{ turn: MutableTurn; nodes: ArenaReplayQueuedEvent[] }> = []

  for (let eventIndex = 0; eventIndex < rows.length; eventIndex += 1) {
    const row = rows[eventIndex]
    if (String(row?.event?.type || "").trim() !== "battle.timeline.node") continue
    const data = asRecord(row.event.data)
    const parentSeq = positiveInteger(data.parentSeq)
    const current = drafts[drafts.length - 1]
    const currentSeq = current?.turn.seq || 0
    const belongsToCurrent = Boolean(current && parentSeq > 0 && parentSeq === currentSeq)

    if (belongsToCurrent && current) {
      current.nodes.push(row)
      current.turn.syncedTurn = true
      continue
    }

    const owner = asRecord(data.turnOwner)
    const actorId = normalizedId(owner.unitId)
    const actor = units.get(actorId)
    const cast = findPrimaryCast([row])
    drafts.push({
      nodes: [row],
      turn: {
        eventStart: eventIndex,
        seq: positiveInteger(row.event.seq),
        offsetMs: Math.max(0, Math.trunc(Number(row.offsetMs || 0))),
        actorId,
        actorName: resolveUnitName(actor, actorId),
        side: normalizeSide(owner.side) || normalizeSide(actor?.side),
        skillId: cast.skillId,
        skillNameFallback: cast.skillName,
        wave: positiveInteger(data.wave),
        preparationTurn: Boolean(data.preparationTurn),
        extraTurn: Boolean(data.extraTurn),
        syncedTurn: false,
        damage: 0,
        healing: 0,
        criticalHits: 0,
        knockouts: 0,
        statusChanges: 0
      }
    })
  }

  const actorContributions = new Map<string, MutableContribution>()
  const replayKnockouts = new Set<string>()
  const turns = drafts.map((draft, draftIndex) => {
    const eventEnd = drafts[draftIndex + 1]?.turn.eventStart ?? rows.length
    const turnRows = rows.slice(draft.turn.eventStart, eventEnd)
    const turnStats = summarizeRows(turnRows, units, actorContributions, replayKnockouts)
    const cast = findPrimaryCast(draft.nodes)
    return {
      ...draft.turn,
      index: draftIndex + 1,
      eventEnd,
      skillId: cast.skillId || draft.turn.skillId,
      skillNameFallback: cast.skillName || draft.turn.skillNameFallback,
      damage: turnStats.damage,
      healing: turnStats.healing,
      criticalHits: turnStats.criticalHits,
      knockouts: turnStats.knockouts,
      statusChanges: turnStats.statusChanges,
      keyMoment: false
    }
  })

  const maxDamage = Math.max(0, ...turns.map((turn) => turn.damage))
  const maxHealing = Math.max(0, ...turns.map((turn) => turn.healing))
  for (const turn of turns) {
    turn.keyMoment = turn.knockouts > 0 ||
      turn.criticalHits > 0 ||
      turn.extraTurn ||
      turn.syncedTurn ||
      turn.statusChanges >= 3 ||
      (maxDamage > 0 && turn.damage >= maxDamage * 0.68) ||
      (maxHealing > 0 && turn.healing >= maxHealing * 0.68)
  }

  const actors = [...actorContributions.values()]
    .filter((actor) => actor.actorId)
    .sort((left, right) => {
      const leftImpact = left.damage + left.healing
      const rightImpact = right.damage + right.healing
      return rightImpact - leftImpact || right.turns - left.turns || left.actorName.localeCompare(right.actorName)
    })

  return {
    turns,
    actors,
    player: summarizeSide(actors, "player"),
    enemy: summarizeSide(actors, "enemy")
  }
}

function collectReplayUnits(events: ArenaReplayQueuedEvent[], snapshot: BattleRunStatePayload | null): Map<string, BattleUnit> {
  const units = new Map<string, BattleUnit>()
  const addUnits = (entries: unknown) => {
    if (!Array.isArray(entries)) return
    for (const raw of entries) {
      const unit = asRecord(raw) as BattleUnit
      const unitId = normalizedId(unit.unitId)
      if (unitId) units.set(unitId, unit)
    }
  }
  addUnits(snapshot?.players)
  addUnits(snapshot?.enemies)
  for (const row of events) {
    const data = asRecord(row.event.data)
    addUnits(data.players)
    addUnits(data.enemies)
    for (const rawStep of asArray(data.steps)) {
      const summon = asRecord(asRecord(rawStep).summon) as BattleUnit
      const summonId = normalizedId(summon.unitId)
      if (summonId) units.set(summonId, summon)
    }
  }
  return units
}

function summarizeRows(
  rows: ArenaReplayQueuedEvent[],
  units: Map<string, BattleUnit>,
  contributions: Map<string, MutableContribution>,
  replayKnockouts: Set<string>
): ArenaReplaySideContribution & { statusChanges: number } {
  const total = { damage: 0, healing: 0, criticalHits: 0, knockouts: 0, statusChanges: 0 }

  for (const row of rows) {
    if (String(row.event.type || "").trim() !== "battle.timeline.node") continue
    const data = asRecord(row.event.data)
    const owner = asRecord(data.turnOwner)
    const ownerId = normalizedId(owner.unitId)
    if (ownerId) ensureContribution(contributions, units, ownerId, normalizeSide(owner.side)).turns += 1

    for (const rawStep of asArray(data.steps)) {
      const step = asRecord(rawStep)
      const kind = String(step.kind || "").trim().toLowerCase()
      const source = asRecord(step.source)
      const sourceId = normalizedId(source.unitId) || ownerId
      const actor = sourceId ? ensureContribution(contributions, units, sourceId, normalizeSide(source.side) || normalizeSide(owner.side)) : null
      const targets = asArray(step.targets).map(asRecord)
      const targetRows = targets.length > 0 ? targets : [{}]

      if (kind === "damage" || kind === "heal") {
        for (const target of targetRows) {
          const amount = resolveStepAmount(kind, step, target)
          if (kind === "damage") {
            total.damage += amount
            if (actor) actor.damage += amount
            if (resolveCritical(step, target)) {
              total.criticalHits += 1
              if (actor) actor.criticalHits += 1
            }
            const targetId = normalizedId(target.unitId ?? target.targetId)
            if (isKnockout(target) && (!targetId || !replayKnockouts.has(targetId))) {
              if (targetId) replayKnockouts.add(targetId)
              total.knockouts += 1
              if (actor) actor.knockouts += 1
            }
          } else {
            total.healing += amount
            if (actor) actor.healing += amount
          }
        }
      }

      if (STATUS_STEP_KINDS.has(kind)) {
        total.statusChanges += Math.max(1, targets.length)
      }
    }
  }

  return total
}

function findPrimaryCast(rows: ArenaReplayQueuedEvent[]): { skillId: string; skillName: string } {
  let fallback = { skillId: "", skillName: "" }
  for (const row of rows) {
    const steps = asArray(asRecord(row.event.data).steps).map(asRecord)
    for (const step of steps) {
      if (String(step.kind || "").trim().toLowerCase() !== "cast") continue
      const skill = asRecord(step.skill)
      const candidate = {
        skillId: String(skill.skillId || "").trim(),
        skillName: String(skill.skillName || "").trim()
      }
      if (String(step.phase || "").trim().toLowerCase() === "action") return candidate
      if (!fallback.skillId && !fallback.skillName) fallback = candidate
    }
  }
  return fallback
}

function ensureContribution(
  contributions: Map<string, MutableContribution>,
  units: Map<string, BattleUnit>,
  actorId: string,
  fallbackSide: ArenaReplaySide
): MutableContribution {
  const existing = contributions.get(actorId)
  if (existing) return existing
  const unit = units.get(actorId)
  const contribution: MutableContribution = {
    actorId,
    actorName: resolveUnitName(unit, actorId),
    side: normalizeSide(unit?.side) || fallbackSide,
    turns: 0,
    damage: 0,
    healing: 0,
    criticalHits: 0,
    knockouts: 0
  }
  contributions.set(actorId, contribution)
  return contribution
}

function summarizeSide(actors: ArenaReplayActorContribution[], side: ArenaReplaySide): ArenaReplaySideContribution {
  return actors
    .filter((actor) => actor.side === side)
    .reduce((total, actor) => ({
      damage: total.damage + actor.damage,
      healing: total.healing + actor.healing,
      criticalHits: total.criticalHits + actor.criticalHits,
      knockouts: total.knockouts + actor.knockouts
    }), { damage: 0, healing: 0, criticalHits: 0, knockouts: 0 })
}

function resolveStepAmount(kind: string, step: Record<string, unknown>, target: Record<string, unknown>): number {
  const amount = Number(step.amount ?? target.amount ?? 0)
  if (Number.isFinite(amount) && amount > 0) return amount
  const hpBefore = Number(target.hpBefore)
  const hpAfter = Number(target.hpAfter)
  if (!Number.isFinite(hpBefore) || !Number.isFinite(hpAfter)) return 0
  return Math.max(0, kind === "heal" ? hpAfter - hpBefore : hpBefore - hpAfter)
}

function resolveCritical(step: Record<string, unknown>, target: Record<string, unknown>): boolean {
  if (typeof target.crit === "boolean") return target.crit
  return step.crit === true
}

function isKnockout(target: Record<string, unknown>): boolean {
  if (target.aliveAfter !== false) return false
  const hpBefore = Number(target.hpBefore)
  return !Number.isFinite(hpBefore) || hpBefore > 0
}

function resolveUnitName(unit: BattleUnit | undefined, fallback: string): string {
  return String(unit?.name || unit?.nameKey || fallback || "").trim()
}

function normalizeSide(raw: unknown): ArenaReplaySide {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "player" || value === "enemy") return value
  return ""
}

function normalizedId(raw: unknown): string {
  const value = String(raw || "").trim()
  return value === "<nil>" || value === "nil" || value === "null" ? "" : value
}

function positiveInteger(raw: unknown): number {
  const value = Math.trunc(Number(raw || 0))
  return Number.isFinite(value) && value > 0 ? value : 0
}

function asRecord(raw: unknown): Record<string, unknown> {
  return raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
}

function asArray(raw: unknown): unknown[] {
  return Array.isArray(raw) ? raw : []
}
