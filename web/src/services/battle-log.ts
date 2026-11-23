import { getLocalizedItemName } from './item-label'

export type BattleLogCategory = "action" | "damage" | "support" | "status" | "field" | "system"

export type BattleLogEntityReference = {
  kind: "unit" | "skill" | "item" | "buff"
  id: string
  label?: string
  enemyId?: string
  remainTurns?: number
  payload?: Record<string, unknown>
  stackCount?: number
}

export type BattleLogDetail = {
  text: string
  tone?: "default" | "danger" | "success" | "warning" | "info"
}

export type BattleLogLine = {
  id: string
  kind: string
  category: BattleLogCategory
  icon: string
  phase?: string
  messageKey: string
  values: Record<string, string | number>
  refs: Record<string, BattleLogEntityReference>
  details: BattleLogDetail[]
  relatedRefs: BattleLogEntityReference[]
}

export type BattleLogGroup = {
  id: string
  runId: number
  seq: number
  ts: number
  wave: number
  eventType: string
  actor?: BattleLogEntityReference
  preparationTurn: boolean
  extraTurn: boolean
  turnKind: string
  lines: BattleLogLine[]
}

export type BattleLogEvent = {
  seq?: number
  type?: string
  ts?: number
  data?: Record<string, unknown>
}

export type BattleLogCompressionResult = {
  groups: BattleLogGroup[]
  removedLineCount: number
  firstWave: number
  lastWave: number
}

type Translate = (key: string, params?: Record<string, unknown>) => string

export type BattleLogBuildOptions = {
  runId: number
  t: Translate
  resolveUnit: (unitId: string, snapshot?: Record<string, unknown>) => BattleLogEntityReference
  positionLabel: (position: string) => string
  sideLabel: (side: string) => string
  damageTypeLabel: (damageType: string) => string
  resourceLabel: (resource: string) => string
  resolveFieldEffectName: (effectId: string) => string
  resolveSourceLabel: (meta: Record<string, unknown>) => string
  formatStatModifier: (key: string, value: number) => string
}

type StepContext = {
  ownerId: string
  lineSerial: number
  currentRefByPhase: Map<string, BattleLogEntityReference>
}

export function battleLogEventKey(runId: number, event: BattleLogEvent): string {
  return `${Math.max(0, Math.trunc(Number(runId) || 0))}:${Math.trunc(Number(event.seq) || 0)}:${String(event.type || "").trim()}`
}

export function formatBattleLogNumber(raw: unknown): string {
  const value = Number(raw)
  if (!Number.isFinite(value)) return "0"
  const rounded = Math.round(value * 100) / 100
  if (Number.isInteger(rounded)) return String(rounded)
  return rounded.toFixed(2).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
}

export function compressBattleLogGroups(
  groups: BattleLogGroup[],
  maxDetailedLines = 1000,
  compressionBatchLines = 50
): BattleLogCompressionResult {
  const maxLines = Math.max(1, Math.trunc(Number(maxDetailedLines) || 1))
  const totalLines = groups.reduce((total, group) => total + group.lines.length, 0)
  if (totalLines <= maxLines) {
    return { groups, removedLineCount: 0, firstWave: 0, lastWave: 0 }
  }

  const batchLines = Math.max(1, Math.trunc(Number(compressionBatchLines) || 1))
  let linesToRemove = totalLines - Math.max(1, maxLines - batchLines)
  let removedLineCount = 0
  let firstWave = 0
  let lastWave = 0
  const nextGroups = groups.slice()

  while (linesToRemove > 0 && nextGroups.length > 0) {
    const group = nextGroups[0]
    if (!group || group.lines.length === 0) {
      nextGroups.shift()
      continue
    }

    const wholeGroupCanBeRemoved = group.lines.length <= linesToRemove && nextGroups.length > 1
    const removableLines = wholeGroupCanBeRemoved
      ? group.lines.length
      : Math.min(linesToRemove, Math.max(0, group.lines.length - 1))
    if (removableLines <= 0) break

    const wave = Math.max(1, Math.trunc(Number(group.wave) || 1))
    if (firstWave === 0) firstWave = wave
    lastWave = wave
    removedLineCount += removableLines
    linesToRemove -= removableLines

    if (wholeGroupCanBeRemoved) {
      nextGroups.shift()
    } else {
      nextGroups[0] = { ...group, lines: group.lines.slice(removableLines) }
    }
  }

  return { groups: nextGroups, removedLineCount, firstWave, lastWave }
}

export function buildBattleLogGroup(event: BattleLogEvent, options: BattleLogBuildOptions): BattleLogGroup | null {
  const eventType = String(event.type || "").trim()
  if (!eventType) return null
  const data = asRecord(event.data)
  const seq = Math.trunc(Number(event.seq) || 0)
  const wave = Math.max(0, Math.trunc(Number(data.wave ?? data.w) || 0))
  const ownerId = normalizedId(
    asRecord(data.turnOwner).unitId ?? data.actorId ?? data.o
  )
  const group: BattleLogGroup = {
    id: battleLogEventKey(options.runId, event),
    runId: Math.max(0, Math.trunc(Number(options.runId) || 0)),
    seq,
    ts: Math.trunc(Number(event.ts) || 0),
    wave,
    eventType,
    actor: ownerId ? options.resolveUnit(ownerId) : undefined,
    preparationTurn: Boolean(data.preparationTurn ?? data.pr),
    extraTurn: Boolean(data.extraTurn ?? data.et),
    turnKind: String(data.turnKind ?? data.tk ?? "").trim(),
    lines: []
  }
  const context: StepContext = {
    ownerId,
    lineSerial: 0,
    currentRefByPhase: new Map()
  }

  switch (eventType) {
    case "battle.timeline.node":
      group.lines = buildStepLines(normalizeEventSteps(eventType, data), group, context, options)
      break
    case "battle.turn.action":
    case "battle.turn.item_use":
    case "battle.turn.regen":
      group.lines = buildStepLines(normalizeEventSteps(eventType, data), group, context, options)
      break
    case "battle.wave.spawn":
      group.lines = buildWaveSpawnLines(data, group, context, options)
      break
    case "battle.wave.win":
      group.lines.push(makeLine(group, context, {
        kind: "wave_win",
        category: "system",
        icon: "lucide:flag",
        messageKey: "game.battleLogWaveWin",
        values: {
          wave: Math.max(1, wave),
          completed: Math.max(0, Math.trunc(Number(data.completedBattles) || 0))
        }
      }))
      for (const summary of Object.values(asRecord(data.settlementSummary))) {
        const rows = asRecord(summary).kittyExtraLoot
        if (!Array.isArray(rows)) continue
        for (const raw of rows) {
          const row = asRecord(raw)
          group.lines.push(makeLine(group, context, {
            kind: 'kitty_extra_loot', category: 'system', icon: 'lucide:package-plus',
            messageKey: 'game.furniture.lootLog',
            values: { name: String(row.name || ''), result: options.t(row.triggered ? 'game.furniture.triggered' : 'game.furniture.missed') },
            relatedRefs: Object.entries(asRecord(row.items)).filter(([, n]) => Number(n) > 0).map(([id, n]) => ({ kind: 'item', id, label: `${getLocalizedItemName(options.t, id)} ×${n}` }))
          }))
        }
      }
      break
    case "battle.failed":
      group.lines.push(makeLine(group, context, {
        kind: "battle_failed",
        category: "system",
        icon: "lucide:circle-x",
        messageKey: "game.battleLogBattleFailed",
        values: { reason: String(data.reason || "").trim() }
      }))
      break
    case "battle.ended":
      group.lines.push(makeLine(group, context, {
        kind: "battle_ended",
        category: "system",
        icon: "lucide:flag-off",
        messageKey: "game.battleLogBattleEnded",
        values: { reason: String(data.reason || "").trim() }
      }))
      break
    case "battle.field.tick":
      group.lines = buildFieldTickLines(data, group, context, options)
      break
    case "battle.field.trigger":
      group.lines = buildFieldTriggerLines(data, group, context, options)
      break
    case "battle.field.window":
      group.lines = buildFieldWindowLines(data, group, context, options)
      break
    default:
      group.lines.push(makeLine(group, context, {
        kind: "event",
        category: "system",
        icon: "lucide:activity",
        messageKey: "game.battleLogGenericEvent",
        values: { event: eventType }
      }))
  }

  if (group.lines.length === 0) return null
  return group
}

function buildWaveSpawnLines(
  data: Record<string, unknown>,
  group: BattleLogGroup,
  context: StepContext,
  options: BattleLogBuildOptions
): BattleLogLine[] {
  const players = recordArray(data.players)
  const enemies = recordArray(data.enemies)
  const lines: BattleLogLine[] = [makeLine(group, context, {
    kind: "wave_start",
    category: "system",
    icon: "lucide:flag",
    messageKey: "game.battleLogWaveStart",
    values: {
      wave: Math.max(1, group.wave),
      players: players.length,
      enemies: enemies.length
    }
  })]

  for (const unit of players) {
    const unitId = normalizedId(unit.unitId)
    if (!unitId) continue
    lines.push(makeLine(group, context, {
      kind: "unit_enter",
      category: "system",
      icon: "lucide:shield",
      messageKey: "game.battleLogAllyEntered",
      refs: { unit: options.resolveUnit(unitId, unit) },
      values: { position: options.positionLabel(String(unit.position || "")) },
      details: unitSnapshotDetails(unit, options)
    }))
  }
  for (const unit of enemies) {
    const unitId = normalizedId(unit.unitId)
    if (!unitId) continue
    lines.push(makeLine(group, context, {
      kind: "unit_enter",
      category: "system",
      icon: "game-icons:crossed-swords",
      messageKey: "game.battleLogEnemyEntered",
      refs: { unit: options.resolveUnit(unitId, unit) },
      values: { position: options.positionLabel(String(unit.position || "")) },
      details: unitSnapshotDetails(unit, options)
    }))
  }

  const startActions = recordArray(data.battleStartActions)
  if (startActions.length > 0) {
    lines.push(...buildStepLines(startActions.map((step) => ({ ...step, phase: "action" })), group, context, options))
  }
  return lines
}

function unitSnapshotDetails(unit: Record<string, unknown>, options: BattleLogBuildOptions): BattleLogDetail[] {
  const details: BattleLogDetail[] = []
  const hp = finiteNumber(unit.hp)
  const maxHp = finiteNumber(unit.maxHp)
  const mp = finiteNumber(unit.mp)
  const maxMp = finiteNumber(unit.maxMp)
  if (hp !== null || maxHp !== null) {
    details.push({ text: options.t("game.battleLogDetailCurrentHp", {
      value: formatBattleLogNumber(hp ?? 0),
      max: formatBattleLogNumber(maxHp ?? 0)
    }) })
  }
  if (mp !== null || maxMp !== null) {
    details.push({ text: options.t("game.battleLogDetailCurrentMp", {
      value: formatBattleLogNumber(mp ?? 0),
      max: formatBattleLogNumber(maxMp ?? 0)
    }) })
  }
  return details
}

function buildFieldTickLines(
  data: Record<string, unknown>,
  group: BattleLogGroup,
  context: StepContext,
  options: BattleLogBuildOptions
): BattleLogLine[] {
  const effectId = normalizedId(data.effectId ?? data.id)
  const state = asRecord(data.state)
  return [makeLine(group, context, {
    kind: "field_tick",
    category: "field",
    icon: "game-icons:stopwatch",
    messageKey: "game.battleLogFieldTick",
    values: {
      field: options.resolveFieldEffectName(effectId),
      remaining: Math.max(0, Math.trunc(Number(state.remaining) || 0)),
      progress: Math.max(0, Math.trunc(Number(state.progress) || 0)),
      goal: Math.max(0, Math.trunc(Number(state.goal) || 0))
    }
  })]
}

function buildFieldTriggerLines(
  data: Record<string, unknown>,
  group: BattleLogGroup,
  context: StepContext,
  options: BattleLogBuildOptions
): BattleLogLine[] {
  const effectId = normalizedId(data.effectId ?? data.id)
  const field = options.resolveFieldEffectName(effectId)
  const lines = [makeLine(group, context, {
    kind: "field_trigger",
    category: "field",
    icon: "game-icons:falling-star",
    messageKey: "game.battleLogFieldTriggered",
    values: { field }
  })]
  for (const [index, target] of recordArray(data.targets).entries()) {
    const targetId = normalizedId(target.unitId ?? target.targetId)
    if (!targetId) continue
    const amount = finiteNumber(target.damage)
    const outcomeId = normalizedId(target.outcomeId)
    const details = hpDetails(target, options)
    if (outcomeId) {
      const relatedRefs: BattleLogEntityReference[] = []
      const statusId = normalizedId(target.statusId)
      const durationTurns = finiteNumber(target.durationTurns)
      if (statusId) {
        const statusState = recordArray(target.buffs).find((buff) => normalizedId(buff.buffId ?? buff.id) === statusId)
        const remainTurns = finiteNumber(statusState?.remainTurns ?? statusState?.durationTurns ?? durationTurns)
        const payload = asRecord(statusState?.payload)
        const triggered = finiteNumber(statusState?.triggered ?? statusState?.stackCount)
        relatedRefs.push({
          kind: "buff",
          id: statusId,
          ...(remainTurns !== null ? { remainTurns: Math.trunc(remainTurns) } : {}),
          ...(Object.keys(payload).length > 0 ? { payload } : {}),
          ...(triggered !== null && triggered > 1 ? { stackCount: Math.trunc(triggered) } : {})
        })
      }
      if (durationTurns !== null && durationTurns > 0) {
        details.push({ text: options.t("game.battleLogDetailDuration", {
          turns: Math.max(1, Math.trunc(durationTurns))
        }) })
      }
      for (const change of recordArray(target.cooldownChanges)) {
        const skillId = normalizedId(change.skillId)
        if (skillId) relatedRefs.push({ kind: "skill", id: skillId })
        const before = finiteNumber(change.before)
        const after = finiteNumber(change.after)
        if (before === null && after === null) continue
        details.push({ text: options.t("game.battleLogDetailCooldown", {
          before: Math.max(0, Math.trunc(before ?? 0)),
          after: Math.max(0, Math.trunc(after ?? 0))
        }) })
      }
      const outcomeKey = effectId ? `battlefield_effect.${effectId}.outcomes.${outcomeId}` : ""
      const localized = outcomeKey ? options.t(outcomeKey) : ""
      const outcome = localized && localized !== outcomeKey ? localized : outcomeId
      lines.push(makeLine(group, context, {
        kind: "field_outcome",
        category: "field",
        icon: "lucide:sparkles",
        messageKey: "game.battleLogFieldOutcome",
        refs: { target: options.resolveUnit(targetId) },
        values: { field, outcome },
        details,
        relatedRefs
      }, index))
      continue
    }
    lines.push(makeLine(group, context, {
      kind: "field_damage",
      category: "damage",
      icon: "game-icons:impact-point",
      messageKey: "game.battleLogFieldDamage",
      refs: { target: options.resolveUnit(targetId) },
      values: { field, amount: formatBattleLogNumber(Math.abs(amount ?? 0)) },
      details: withDefeatedDetail(details, target, options)
    }, index))
  }
  return lines
}

function buildFieldWindowLines(
  data: Record<string, unknown>,
  group: BattleLogGroup,
  context: StepContext,
  options: BattleLogBuildOptions
): BattleLogLine[] {
  const effectId = normalizedId(data.effectId ?? data.id)
  const status = String(data.status || "tick").trim().toLowerCase()
  const messageKey = status === "opened"
    ? "game.battleLogFieldWindowOpened"
    : status === "closed"
      ? "game.battleLogFieldWindowClosed"
      : "game.battleLogFieldWindowTick"
  return [makeLine(group, context, {
    kind: "field_window",
    category: "field",
    icon: "lucide:timer-reset",
    messageKey,
    values: {
      field: options.resolveFieldEffectName(effectId),
      window: String(data.windowId || "").trim(),
      remaining: Math.max(0, Math.trunc(Number(data.remaining) || 0))
    }
  })]
}

function buildStepLines(
  steps: Record<string, unknown>[],
  group: BattleLogGroup,
  context: StepContext,
  options: BattleLogBuildOptions
): BattleLogLine[] {
  const lines: BattleLogLine[] = []
  for (const step of steps) {
    const kind = String(step.kind ?? step.k ?? "").trim().toLowerCase()
    if (!kind) continue
    const phase = normalizedPhase(step.phase ?? step.ph)
    const phaseKey = phase || "action"
    const sourceId = stepSourceId(step, context.ownerId)
    const sourceRef = sourceId ? options.resolveUnit(sourceId) : undefined
    const targets = stepTargets(step)
    const skillId = normalizedId(asRecord(step.skill).skillId ?? step.skillId ?? step.i)
    const buffId = normalizedId(step.buffId ?? step.buffID ?? step.stateId ?? step.bi)

    if (kind === "cast") {
      if (!skillId) continue
      const isItem = phase === "item"
      const entity: BattleLogEntityReference = { kind: isItem ? "item" : "skill", id: skillId }
      context.currentRefByPhase.set(phaseKey, entity)
      lines.push(makeLine(group, context, {
        kind,
        category: "action",
        icon: isItem ? "game-icons:potion-ball" : "game-icons:magic-swirl",
        phase,
        messageKey: isItem ? "game.battleLogUseItem" : "game.battleLogCast",
        refs: {
          ...(sourceRef ? { source: sourceRef } : {}),
          [isItem ? "item" : "skill"]: entity
        }
      }))
      continue
    }

    const currentRef = context.currentRefByPhase.get(phaseKey)
    const relatedRefs = collectRelatedRefs(step, currentRef)
    const stepMeta = asRecord(step.meta ?? step.me)
    const sourceLabel = options.resolveSourceLabel(stepMeta)

    if (kind === "damage" || kind === "damage_redirect") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        const amount = resolvedHpAmount("damage", step, target)
        const critical = Boolean(target.crit ?? step.crit ?? step.c)
        const damageType = String(step.damageType ?? target.damageType ?? step.dt ?? "").trim().toLowerCase()
        const details = withDamageDetails(hpDetails(target, options), step, target, sourceLabel, options)
        lines.push(makeLine(group, context, {
          kind,
          category: "damage",
          icon: damageType === "magical" || damageType === "magic" ? "game-icons:fairy-wand" : "game-icons:gladius",
          phase,
          messageKey: kind === "damage_redirect" ? "game.battleLogDamageRedirect" : "game.battleLogDamage",
          refs: {
            ...(sourceRef ? { source: sourceRef } : {}),
            target: options.resolveUnit(targetId)
          },
          values: {
            amount: formatBattleLogNumber(Math.abs(amount)),
            damageType: options.damageTypeLabel(damageType),
            critical: critical ? options.t("game.battleLogCriticalSuffix") : ""
          },
          details: withDefeatedDetail(details, target, options),
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "heal") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        const amount = resolvedHpAmount("heal", step, target)
        const details = hpDetails(target, options)
        if (sourceLabel) details.push({ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" })
        lines.push(makeLine(group, context, {
          kind,
          category: "support",
          icon: "game-icons:heart-plus",
          phase,
          messageKey: "game.battleLogHeal",
          refs: {
            ...(sourceRef ? { source: sourceRef } : {}),
            target: options.resolveUnit(targetId)
          },
          values: { amount: formatBattleLogNumber(Math.abs(amount)) },
          details,
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "resource") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        const amount = finiteNumber(step.amount ?? step.v ?? target.amount) ?? 0
        const resource = inferResourceKind(step, target)
        const before = resource === "hp"
          ? finiteNumber(target.hpBefore ?? target.beforeHp ?? target.b)
          : finiteNumber(target.mpBefore ?? target.beforeMp)
        const after = resource === "hp"
          ? finiteNumber(target.hpAfter ?? target.afterHp ?? target.hp)
          : finiteNumber(target.mpAfter ?? target.afterMp ?? target.mp)
        const details: BattleLogDetail[] = []
        const resolvedBefore = before ?? (after !== null ? after - amount : null)
        if (resolvedBefore !== null && after !== null) {
          details.push({ text: options.t("game.battleLogDetailResource", {
            resource: options.resourceLabel(resource),
            before: formatBattleLogNumber(resolvedBefore),
            after: formatBattleLogNumber(after)
          }) })
        } else if (after !== null) {
          details.push({ text: options.t("game.battleLogDetailResourceAfter", {
            resource: options.resourceLabel(resource),
            value: formatBattleLogNumber(after)
          }) })
        }
        if (sourceLabel) details.push({ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" })
        lines.push(makeLine(group, context, {
          kind,
          category: "support",
          icon: resource === "hp" ? "game-icons:heart-plus" : "game-icons:embrassed-energy",
          phase,
          messageKey: amount < 0 ? "game.battleLogResourceLoss" : "game.battleLogResourceGain",
          refs: { target: options.resolveUnit(targetId) },
          values: {
            amount: formatBattleLogNumber(Math.abs(amount)),
            resource: options.resourceLabel(resource)
          },
          details,
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "buff" || kind === "buff_charge" || kind === "state_detach") {
      if (!buffId) continue
      const remainTurns = finiteNumber(step.buffDurationTurns ?? step.bt)
      const triggered = finiteNumber(step.buffTriggered ?? step.bg)
      const payload = asRecord(step.buffPayload ?? step.bp)
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        const buffRef: BattleLogEntityReference = {
          kind: "buff",
          id: buffId,
          remainTurns: remainTurns === null ? undefined : Math.trunc(remainTurns),
          payload: Object.keys(payload).length > 0 ? payload : undefined,
          stackCount: triggered !== null && triggered > 1 ? Math.trunc(triggered) : undefined
        }
        const details: BattleLogDetail[] = []
        if (remainTurns !== null) {
          details.push({ text: remainTurns < 0
            ? options.t("game.battleUnitDurationInfinite")
            : options.t("game.battleLogDetailDuration", { turns: Math.trunc(remainTurns) }) })
        }
        if (triggered !== null) {
          details.push({ text: options.t("game.battleLogDetailProgress", { value: Math.trunc(triggered) }) })
        }
        if (sourceLabel) details.push({ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" })
        lines.push(makeLine(group, context, {
          kind,
          category: "status",
          icon: kind === "state_detach" ? "game-icons:broken-shield" : "game-icons:aura",
          phase,
          messageKey: kind === "state_detach"
            ? "game.battleLogBuffRemoved"
            : kind === "buff_charge"
              ? "game.battleLogBuffProgress"
              : "game.battleLogBuffApplied",
          refs: {
            ...(sourceRef ? { source: sourceRef } : {}),
            target: options.resolveUnit(targetId),
            buff: buffRef
          },
          values: { progress: Math.max(0, Math.trunc(triggered ?? 0)) },
          details,
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "summon") {
      const summon = asRecord(step.summon ?? step.q)
      const summonId = normalizedId(summon.unitId ?? summon.u ?? targets[0]?.unitId)
      if (!summonId) continue
      const details = unitSnapshotDetails({
        hp: summon.hp ?? summon.h,
        maxHp: summon.maxHp ?? summon.x,
        mp: summon.mp ?? summon.m,
        maxMp: summon.maxMp ?? summon.y
      }, options)
      const side = String(summon.side ?? summon.s ?? "").trim()
      const position = String(summon.position ?? summon.p ?? "").trim()
      if (side) details.push({ text: options.t("game.battleLogDetailSide", { side: options.sideLabel(side) }) })
      if (position) details.push({ text: options.t("game.battleLogDetailPosition", { position: options.positionLabel(position) }) })
      lines.push(makeLine(group, context, {
        kind,
        category: "status",
        icon: "game-icons:teleport",
        phase,
        messageKey: "game.battleLogSummon",
        refs: {
          ...(sourceRef ? { source: sourceRef } : {}),
          summon: options.resolveUnit(summonId, summon)
        },
        details,
        relatedRefs
      }))
      continue
    }

    if (kind === "summon_expire") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        lines.push(makeLine(group, context, {
          kind,
          category: "status",
          icon: "game-icons:teleport",
          phase,
          messageKey: "game.battleLogSummonExpired",
          refs: { unit: options.resolveUnit(targetId) },
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "position_move") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        const from = String(target.fromPosition ?? target.f ?? "").trim()
        const to = String(target.toPosition ?? target.position ?? target.t ?? target.p ?? "").trim()
        lines.push(makeLine(group, context, {
          kind,
          category: "status",
          icon: "lucide:move-right",
          phase,
          messageKey: "game.battleLogPositionMove",
          refs: { unit: options.resolveUnit(targetId) },
          values: {
            from: options.positionLabel(from),
            to: options.positionLabel(to)
          },
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "corpse_focus" || kind === "corpse_consume") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        lines.push(makeLine(group, context, {
          kind,
          category: "status",
          icon: "game-icons:death-skull",
          phase,
          messageKey: kind === "corpse_focus" ? "game.battleLogCorpseFocused" : "game.battleLogCorpseConsumed",
          refs: {
            ...(sourceRef ? { source: sourceRef } : {}),
            target: options.resolveUnit(targetId)
          },
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    if (kind === "trigger") {
      const interruptedSkill = normalizedId(stepMeta.interruptedSkillId)
      const interruptedActor = normalizedId(stepMeta.interruptedActorId ?? targets[0]?.unitId ?? step.targetId)
      if (skillId === "equipment_domination" && interruptedSkill && interruptedActor) {
        lines.push(makeLine(group, context, {
          kind, category: "action", icon: "lucide:shield-ban", phase,
          messageKey: "game.battleLogSkillInterrupted",
          refs: {
            ...(sourceRef ? {source: sourceRef} : {}),
            skill: {kind: "skill", id: skillId},
            target: options.resolveUnit(interruptedActor),
            interruptedSkill: {kind: "skill", id: interruptedSkill}
          }, relatedRefs
        }))
        continue
      }
      lines.push(makeLine(group, context, {
        kind,
        category: "action",
        icon: "lucide:zap",
        phase,
        messageKey: skillId ? "game.battleLogSkillTriggered" : "game.battleLogEffectTriggered",
        refs: {
          ...(sourceRef ? { source: sourceRef } : {}),
          ...(skillId ? { skill: { kind: "skill", id: skillId } as BattleLogEntityReference } : {})
        },
        values: { effect: String(step.reason || kind) },
        details: sourceLabel ? [{ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" }] : [],
        relatedRefs
      }))
      continue
    }

    if (kind === "extra_turn") {
      const targetId = normalizedId(targets[0]?.unitId ?? targets[0]?.targetId ?? sourceId)
      if (!targetId) continue
      lines.push(makeLine(group, context, {
        kind,
        category: "action",
        icon: "lucide:rotate-cw",
        phase,
        messageKey: "game.battleLogExtraTurn",
        refs: { unit: options.resolveUnit(targetId) },
        relatedRefs
      }))
      continue
    }

    if (kind === "stat_recalc") {
      for (const [targetIndex, target] of targets.entries()) {
        const targetId = normalizedId(target.unitId ?? target.targetId)
        if (!targetId) continue
        const modifiers = asRecord(target.statModifiers ?? target.deltaStats)
        const details = Object.entries(modifiers).flatMap(([key, raw]) => {
          const value = finiteNumber(raw)
          return value === null ? [] : [{ text: options.formatStatModifier(key, value), tone: "info" as const }]
        })
        if (sourceLabel) details.push({ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" })
        lines.push(makeLine(group, context, {
          kind,
          category: "status",
          icon: "lucide:sliders-horizontal",
          phase,
          messageKey: "game.battleLogStatsRecalculated",
          refs: { target: options.resolveUnit(targetId) },
          details,
          relatedRefs
        }, targetIndex))
      }
      continue
    }

    const fallbackTargetId = normalizedId(targets[0]?.unitId ?? targets[0]?.targetId)
    lines.push(makeLine(group, context, {
      kind,
      category: "status",
      icon: "lucide:activity",
      phase,
      messageKey: "game.battleLogEffectTriggered",
      refs: {
        ...(sourceRef ? { source: sourceRef } : {}),
        ...(fallbackTargetId ? { target: options.resolveUnit(fallbackTargetId) } : {})
      },
      values: { effect: kind },
      details: sourceLabel ? [{ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" }] : [],
      relatedRefs
    }))
  }
  return lines
}

function normalizeEventSteps(eventType: string, data: Record<string, unknown>): Record<string, unknown>[] {
  if (eventType === "battle.timeline.node") {
    return recordArray(data.steps ?? data.p)
  }
  if (eventType === "battle.turn.regen") {
    return recordArray(data.actions).map((step) => ({ ...step, phase: "regen" }))
  }
  const ownerId = normalizedId(data.actorId)
  const phase = eventType === "battle.turn.item_use" ? "item" : "action"
  const castId = normalizedId(eventType === "battle.turn.item_use" ? data.itemId : data.skillId)
  const steps: Record<string, unknown>[] = []
  if (castId) {
    steps.push({ kind: "cast", sourceId: ownerId, skillId: castId, phase })
  }
  for (const step of recordArray(data.actionLog)) {
    steps.push({ ...step, phase })
  }
  if (eventType === "battle.turn.action") {
    for (const hook of recordArray(data.hooks)) {
      const sourceId = normalizedId(hook.sourceId) || ownerId
      const skillId = normalizedId(hook.skillId)
      steps.push({ kind: "trigger", sourceId, skillId, targetId: hook.targetId, phase })
      if (skillId) steps.push({ kind: "cast", sourceId, skillId, phase })
      for (const effect of recordArray(hook.effects)) {
        steps.push({ ...effect, phase })
      }
    }
  }
  return steps
}

function makeLine(
  group: BattleLogGroup,
  context: StepContext,
  input: {
    kind: string
    category: BattleLogCategory
    icon: string
    phase?: string
    messageKey: string
    values?: Record<string, string | number>
    refs?: Record<string, BattleLogEntityReference>
    details?: BattleLogDetail[]
    relatedRefs?: BattleLogEntityReference[]
  },
  suffix = 0
): BattleLogLine {
  context.lineSerial += 1
  return {
    id: `${group.id}:${context.lineSerial}:${suffix}`,
    kind: input.kind,
    category: input.category,
    icon: input.icon,
    phase: input.phase,
    messageKey: input.messageKey,
    values: input.values || {},
    refs: input.refs || {},
    details: input.details || [],
    relatedRefs: dedupeRefs(input.relatedRefs || [])
  }
}

function stepSourceId(step: Record<string, unknown>, ownerId: string): string {
  return normalizedId(
    asRecord(step.source).unitId ?? step.visualSourceId ?? step.lineSourceId ?? step.sourceId ?? step.s ?? ownerId
  )
}

function stepTargets(step: Record<string, unknown>): Record<string, unknown>[] {
  const rows = recordArray(step.targets ?? step.t)
  if (rows.length > 0) return rows
  const targetId = normalizedId(step.targetId)
  if (!targetId) return []
  return [{ ...step, unitId: targetId }]
}

function hpDetails(target: Record<string, unknown>, options: BattleLogBuildOptions): BattleLogDetail[] {
  const before = finiteNumber(target.hpBefore ?? target.beforeHp ?? target.b)
  const after = finiteNumber(target.hpAfter ?? target.afterHp ?? target.hp ?? target.h)
  if (before === null && after === null) return []
  return [{ text: options.t("game.battleLogDetailHp", {
    before: formatBattleLogNumber(before ?? 0),
    after: formatBattleLogNumber(after ?? 0)
  }) }]
}

function withDamageDetails(
  details: BattleLogDetail[],
  step: Record<string, unknown>,
  target: Record<string, unknown>,
  sourceLabel: string,
  options: BattleLogBuildOptions
): BattleLogDetail[] {
  const output = details.slice()
  const meta = Object.keys(asRecord(step.meta ?? step.me)).length > 0
    ? asRecord(step.meta ?? step.me)
    : asRecord(target.meta)
  const hitIndex = finiteNumber(meta.hitIndex ?? meta.hi)
  const hitCount = finiteNumber(meta.hitCount ?? meta.hc)
  if (hitIndex !== null && hitCount !== null && hitCount > 1) {
    output.push({ text: options.t("game.battleLogDetailHit", {
      index: Math.max(1, Math.trunc(hitIndex)),
      count: Math.max(1, Math.trunc(hitCount))
    }), tone: "warning" })
  }
  const criticalMultiplier = finiteNumber(meta.criticalMultiplier ?? meta.cm)
  if (criticalMultiplier !== null && criticalMultiplier > 1) {
    output.push({ text: options.t("game.battleLogDetailCriticalMultiplier", {
      value: formatBattleLogNumber(criticalMultiplier)
    }), tone: "warning" })
  }
  const runtimeAmp = finiteNumber(meta.runtimeDamageAmp ?? meta.ra)
  if (runtimeAmp !== null && runtimeAmp > 0) {
    output.push({ text: options.t("game.battleLogDetailDamageAmp", {
      value: formatBattleLogNumber(runtimeAmp * 100)
    }), tone: "warning" })
  }
  if (Boolean(meta.corpseExplosion ?? meta.ce)) {
    output.push({ text: options.t("game.battleLogDetailCorpseExplosion"), tone: "danger" })
  }
  if (sourceLabel) {
    output.push({ text: options.t("game.battleLogDetailSource", { source: sourceLabel }), tone: "info" })
  }
  return output
}

function withDefeatedDetail(
  details: BattleLogDetail[],
  target: Record<string, unknown>,
  options: BattleLogBuildOptions
): BattleLogDetail[] {
  const alive = target.aliveAfter ?? target.afterAlive ?? target.alive ?? target.l
  if (alive !== false) return details
  return [...details, { text: options.t("game.battleLogDetailDefeated"), tone: "danger" }]
}

function resolvedHpAmount(
  kind: "damage" | "heal",
  step: Record<string, unknown>,
  target: Record<string, unknown>
): number {
  const direct = finiteNumber(step.amount ?? step.v ?? target.amount)
  if (direct !== null && direct !== 0) return direct
  const before = finiteNumber(target.hpBefore ?? target.beforeHp ?? target.b)
  const after = finiteNumber(target.hpAfter ?? target.afterHp ?? target.hp ?? target.h)
  if (before === null || after === null) return 0
  return kind === "heal" ? Math.max(0, after - before) : Math.max(0, before - after)
}

function inferResourceKind(step: Record<string, unknown>, target: Record<string, unknown>): string {
  const explicit = String(step.resource ?? target.resource ?? "").trim().toLowerCase()
  if (explicit === "hp" || explicit === "mp") return explicit
  const beforeHp = finiteNumber(target.hpBefore ?? target.beforeHp ?? target.b)
  const afterHp = finiteNumber(target.hpAfter ?? target.afterHp ?? target.hp ?? target.h)
  if (beforeHp !== null && afterHp !== null && beforeHp !== afterHp) return "hp"
  return "mp"
}

function collectRelatedRefs(
  step: Record<string, unknown>,
  currentRef?: BattleLogEntityReference
): BattleLogEntityReference[] {
  const refs: BattleLogEntityReference[] = []
  if (currentRef) refs.push(currentRef)
  const meta = asRecord(step.meta ?? step.me)
  const directItemId = normalizedId(meta.sourceItemId)
  if (directItemId) refs.push({ kind: "item", id: directItemId })
  for (const source of [...recordArray(meta.runtimeDamageAmpSources), ...recordArray(meta.mutationSources)]) {
    const sourceItemId = normalizedId(source.sourceItemId)
    if (sourceItemId) refs.push({ kind: "item", id: sourceItemId })
  }
  return dedupeRefs(refs)
}

function dedupeRefs(refs: BattleLogEntityReference[]): BattleLogEntityReference[] {
  const seen = new Set<string>()
  const output: BattleLogEntityReference[] = []
  for (const ref of refs) {
    const id = normalizedId(ref.id)
    if (!id) continue
    const key = `${ref.kind}:${id}`
    if (seen.has(key)) continue
    seen.add(key)
    output.push({ ...ref, id })
  }
  return output
}

function normalizedPhase(raw: unknown): string {
  const phase = String(raw || "").trim().toLowerCase()
  if (phase === "r") return "regen"
  if (phase === "i") return "item"
  if (phase === "a") return "action"
  if (phase === "regen" || phase === "item" || phase === "action") return phase
  return ""
}

function normalizedId(raw: unknown): string {
  const value = String(raw || "").trim()
  if (!value || value === "<nil>" || value === "nil" || value === "null") return ""
  return value
}

function finiteNumber(raw: unknown): number | null {
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}

function asRecord(raw: unknown): Record<string, unknown> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  return raw as Record<string, unknown>
}

function recordArray(raw: unknown): Record<string, unknown>[] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    const row = asRecord(entry)
    return Object.keys(row).length > 0 ? [row] : []
  })
}
