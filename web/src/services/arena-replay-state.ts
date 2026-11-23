import type {
  BattleCombatTrialFormatState,
  BattleRunEventPayload,
  BattleRunStatePayload,
  BattleUnit,
  BattlefieldEffectState
} from "@composables/game/combat/useCombatPage"

export function cloneArenaBattleState(state: BattleRunStatePayload): BattleRunStatePayload {
  return {
    ...state,
    order: [...(state.order || [])],
    preparationOrder: [...(state.preparationOrder || [])],
    players: state.players.map(cloneArenaBattleUnit),
    enemies: state.enemies.map(cloneArenaBattleUnit),
    battlefieldEffects: (state.battlefieldEffects || []).map(cloneBattlefieldEffect),
    arenaFormat: state.arenaFormat ? { ...state.arenaFormat } : undefined,
    combatTrialFormat: cloneCombatTrialFormat(state.combatTrialFormat),
    combatTrial: state.combatTrial ? { ...state.combatTrial, metrics: { ...state.combatTrial.metrics } } : undefined
  }
}

export function applyArenaReplayEventBatch(
  current: BattleRunStatePayload,
  events: BattleRunEventPayload[]
): BattleRunStatePayload {
  const next = cloneArenaBattleState(current)
  for (const event of events) applyArenaReplayEvent(next, event)
  return next
}

function cloneArenaBattleUnit(unit: BattleUnit): BattleUnit {
  return {
    ...unit,
    stats: cloneNumberRecord(unit.stats),
    attributes: cloneNumberRecord(unit.attributes),
    baseStats: cloneNumberRecord(unit.baseStats),
    statModifiers: cloneNumberRecord(unit.statModifiers),
    statSources: cloneNestedNumberRecord(unit.statSources),
    skillIds: Array.isArray(unit.skillIds) ? [...unit.skillIds] : undefined,
    skillCooldowns: cloneNumberRecord(unit.skillCooldowns),
    skillCooldownTurns: cloneNumberRecord(unit.skillCooldownTurns),
    itemCooldowns: cloneNumberRecord(unit.itemCooldowns),
    itemQuantities: cloneNumberRecord(unit.itemQuantities),
    skills: Array.isArray(unit.skills) ? unit.skills.map((skill) => ({ ...skill })) : undefined,
    battleItems: Array.isArray(unit.battleItems) ? unit.battleItems.map((item) => ({ ...item })) : undefined,
    equipmentInstanceIds: Array.isArray(unit.equipmentInstanceIds) ? [...unit.equipmentInstanceIds] : undefined,
    buffs: Array.isArray(unit.buffs) ? unit.buffs.map(cloneBattleBuff) : [],
    basicAttack: unit.basicAttack ? {
      ...unit.basicAttack,
      formula: Array.isArray(unit.basicAttack.formula) ? unit.basicAttack.formula.map((term) => ({ ...term })) : undefined
    } : undefined
  }
}

function cloneBattlefieldEffect(effect: BattlefieldEffectState): BattlefieldEffectState {
  return { ...effect, windows: { ...(effect.windows || {}) } }
}

function cloneCombatTrialFormat(format: BattleCombatTrialFormatState | undefined): BattleCombatTrialFormatState | undefined {
  if (!format) return undefined
  return {
    ...format,
    metrics: format.metrics.map((metric) => ({ ...metric })),
    endConditions: format.endConditions?.map((condition) => ({ ...condition })),
    encounter: format.encounter
      ? {
          ...format.encounter,
          units: format.encounter.units?.map((unit) => ({ ...unit })),
          waves: format.encounter.waves?.map((wave) => ({ ...wave, units: wave.units.map((unit) => ({ ...unit })) }))
        }
      : undefined
  }
}

function cloneBattleBuff(buff: NonNullable<BattleUnit["buffs"]>[number]) {
  return { ...buff, payload: { ...(buff.payload || {}) } }
}

function applyArenaReplayEvent(state: BattleRunStatePayload, event: BattleRunEventPayload) {
  const type = String(event.type || "").trim()
  const data = asRecord(event.data)
  state.seq = Math.max(state.seq, Math.trunc(Number(event.seq || 0)))
  if (Array.isArray(data.order)) state.order = data.order.map(String)
  if (Array.isArray(data.preparationOrder)) state.preparationOrder = data.preparationOrder.map(String)
  setPositiveInteger(state, "wave", data.wave)
  setNonNegativeInteger(state, "completedBattles", data.completedBattles)
  applyCombatTrialProgress(state, data.combatTrial, event.seq)

  if (type === "battle.wave.spawn") {
    if (Array.isArray(data.players)) state.players = data.players.map((unit) => cloneArenaBattleUnit(unit as BattleUnit))
    if (Array.isArray(data.enemies)) state.enemies = data.enemies.map((unit) => cloneArenaBattleUnit(unit as BattleUnit))
    if (Array.isArray(data.battlefieldEffects)) {
      state.battlefieldEffects = data.battlefieldEffects.map((effect) => cloneBattlefieldEffect(effect as BattlefieldEffectState))
    }
    if (data.arenaFormat && typeof data.arenaFormat === "object" && !Array.isArray(data.arenaFormat)) {
      state.arenaFormat = { ...(data.arenaFormat as NonNullable<BattleRunStatePayload["arenaFormat"]>) }
    }
    if (data.combatTrialFormat && typeof data.combatTrialFormat === "object" && !Array.isArray(data.combatTrialFormat)) {
      state.combatTrialFormat = cloneCombatTrialFormat(data.combatTrialFormat as BattleCombatTrialFormatState)
    }
    return
  }

  if (state.combatTrial) {
    const actionCount = Number(data.combatTrialActionCount ?? data.actionCount)
    const actionBudget = Number(data.combatTrialActionBudget ?? data.actionBudget)
    if (Number.isFinite(actionCount)) state.combatTrial.actionCount = Math.max(0, Math.trunc(actionCount))
    if (Number.isFinite(actionBudget)) state.combatTrial.actionBudget = Math.max(0, Math.trunc(actionBudget))
    if (data.reason !== undefined && type === "battle.ended") state.combatTrial.endReason = String(data.reason || "")
    if (data.metrics && typeof data.metrics === "object" && !Array.isArray(data.metrics)) {
      state.combatTrial.metrics = toNumberRecord(data.metrics)
    }
  }

  const ownerId = normalizedId(asRecord(data.turnOwner).unitId)
  if (ownerId) {
    applyArenaUnitUpdate(state, { unitId: ownerId, ...asRecord(asRecord(data.summary).actor) })
    applyArenaActorCooldowns(state, ownerId, data)
  }

  for (const update of asArray(data.unitUpdates)) applyArenaUnitUpdate(state, asRecord(update))
  for (const rawStep of asArray(data.steps)) applyArenaStep(state, asRecord(rawStep))
  for (const target of asArray(data.targets)) applyArenaUnitUpdate(state, asRecord(target))

  if (Array.isArray(data.battlefieldEffects)) {
    state.battlefieldEffects = data.battlefieldEffects.map((effect) => cloneBattlefieldEffect(effect as BattlefieldEffectState))
  }
}

function applyCombatTrialProgress(state: BattleRunStatePayload, raw: unknown, eventSeq: number) {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return
  const row = raw as Record<string, unknown>
  const actionCount = Math.max(0, Math.trunc(Number(row.actionCount) || 0))
  const actionBudget = Math.max(0, Math.trunc(Number(row.actionBudget) || 0))
  const progressVersion = Math.max(0, Math.trunc(Number(row.progressVersion) || 0)) || undefined
  const progress: NonNullable<BattleRunStatePayload["combatTrial"]> = {
    progressVersion,
    progressSeq: progressVersion ? Math.max(0, Math.trunc(Number(eventSeq) || 0)) : undefined,
    actionCount,
    actionBudget,
    endReason: String(row.endReason || "").trim() || undefined,
    metrics: toNumberRecord(row.metrics)
  }
  if (row.encounterKind !== undefined) progress.encounterKind = String(row.encounterKind || "").trim() || undefined
  if (row.currentWave !== undefined) progress.currentWave = Math.max(0, Math.trunc(Number(row.currentWave) || 0)) || undefined
  if (row.waveCount !== undefined) progress.waveCount = Math.max(0, Math.trunc(Number(row.waveCount) || 0)) || undefined
  if (row.wavesCleared !== undefined) progress.wavesCleared = Math.max(0, Math.trunc(Number(row.wavesCleared) || 0))
  if (row.objectiveUnitsDefeated !== undefined) progress.objectiveUnitsDefeated = Math.max(0, Math.trunc(Number(row.objectiveUnitsDefeated) || 0))
  state.combatTrial = progress
}

function applyArenaActorCooldowns(state: BattleRunStatePayload, ownerId: string, data: Record<string, unknown>) {
  const unit = findArenaUnit(state, ownerId)
  if (!unit) return
  if (data.skillCooldowns !== undefined) unit.skillCooldowns = toNumberRecord(data.skillCooldowns)
  if (data.skillCooldownTurns !== undefined) unit.skillCooldownTurns = toNumberRecord(data.skillCooldownTurns)
  if (data.itemCooldowns !== undefined) unit.itemCooldowns = toNumberRecord(data.itemCooldowns)
}

function applyArenaStep(state: BattleRunStatePayload, step: Record<string, unknown>) {
  const kind = String(step.kind || "").trim().toLowerCase()
  const summon = asRecord(step.summon) as BattleUnit
  if (kind === "summon" && normalizedId(summon.unitId)) addArenaSummon(state, summon)

  const targets = asArray(step.targets).map(asRecord)
  for (const target of targets) applyArenaUnitUpdate(state, target)

  if (kind === "summon_expire") {
    for (const target of targets) removeArenaUnit(state, normalizedId(target.unitId ?? target.targetId))
    return
  }

  const buffId = String(step.buffId || step.buffID || step.stateId || "").trim()
  if (!buffId) return
  for (const target of targets) {
    const unit = findArenaUnit(state, normalizedId(target.unitId ?? target.targetId))
    if (!unit) continue
    if (kind === "state_detach") {
      unit.buffs = (unit.buffs || []).filter((buff) => String(buff.buffId || "").trim() !== buffId)
      continue
    }
    if (kind !== "buff" && kind !== "buff_charge") continue
    const buffs = unit.buffs || []
    const index = buffs.findIndex((buff) => String(buff.buffId || "").trim() === buffId)
    const nextBuff = {
      ...(index >= 0 ? buffs[index] : {}),
      buffId,
      remainTurns: finiteInteger(step.buffDurationTurns, index >= 0 ? buffs[index]?.remainTurns : undefined),
      triggered: finiteInteger(step.buffTriggered, index >= 0 ? buffs[index]?.triggered : undefined),
      payload: { ...(index >= 0 ? buffs[index]?.payload || {} : {}), ...asRecord(step.buffPayload) }
    }
    if (index >= 0) buffs[index] = nextBuff
    else buffs.push(nextBuff)
    unit.buffs = buffs
  }
}

function applyArenaUnitUpdate(state: BattleRunStatePayload, update: Record<string, unknown>) {
  const unitId = normalizedId(update.unitId ?? update.targetId)
  const unit = findArenaUnit(state, unitId)
  if (!unit) return
  setFinite(unit, "hp", update.hpAfter ?? update.afterHp ?? update.hp)
  setFinite(unit, "mp", update.mpAfter ?? update.afterMp ?? update.mp)
  setFinite(unit, "maxHp", update.maxHpAfter ?? update.maxHp)
  setFinite(unit, "maxMp", update.maxMpAfter ?? update.maxMp)

  const alive = update.aliveAfter ?? update.afterAlive ?? update.alive
  if (alive !== undefined) unit.alive = Boolean(alive)
  const corpseConsumed = update.corpseConsumed
  if (corpseConsumed !== undefined) unit.corpseConsumed = Boolean(corpseConsumed)

  for (const key of ["position", "basePosition", "baseSide", "perspectiveSide"] as const) {
    const value = String(update[key] ?? (key === "position" ? update.toPosition : "") ?? "").trim()
    if (value) unit[key] = value
  }

  for (const key of ["stats", "attributes", "baseStats", "statModifiers"] as const) {
    if (update[key] !== undefined) unit[key] = toNumberRecord(update[key])
  }
  if (update.statSources !== undefined) unit.statSources = toNestedNumberRecord(update.statSources)
  if (Array.isArray(update.buffs)) unit.buffs = update.buffs.map((buff) => cloneBattleBuff(buff as NonNullable<BattleUnit["buffs"]>[number]))
  if (update.skillCooldowns !== undefined) unit.skillCooldowns = toNumberRecord(update.skillCooldowns)
  if (update.skillCooldownTurns !== undefined) unit.skillCooldownTurns = toNumberRecord(update.skillCooldownTurns)
  if (update.itemCooldowns !== undefined) unit.itemCooldowns = toNumberRecord(update.itemCooldowns)

  const side = normalizeSide(update.side)
  if (side) {
    unit.side = side
    relocateArenaUnit(state, unit)
  }
}

function findArenaUnit(state: BattleRunStatePayload, unitId: string): BattleUnit | undefined {
  if (!unitId) return undefined
  return [...state.players, ...state.enemies].find((unit) => normalizedId(unit.unitId) === unitId)
}

function addArenaSummon(state: BattleRunStatePayload, summon: BattleUnit) {
  const unitId = normalizedId(summon.unitId)
  if (!unitId || findArenaUnit(state, unitId)) return
  const unit = cloneArenaBattleUnit({ ...summon, unitId, side: normalizeSide(summon.side) || "player", alive: summon.alive !== false })
  if (unit.side === "enemy") state.enemies.push(unit)
  else state.players.push(unit)
}

function removeArenaUnit(state: BattleRunStatePayload, unitId: string) {
  if (!unitId) return
  state.players = state.players.filter((unit) => normalizedId(unit.unitId) !== unitId)
  state.enemies = state.enemies.filter((unit) => normalizedId(unit.unitId) !== unitId)
}

function relocateArenaUnit(state: BattleRunStatePayload, unit: BattleUnit) {
  const unitId = normalizedId(unit.unitId)
  if (!unitId) return
  removeArenaUnit(state, unitId)
  if (unit.side === "enemy") state.enemies.push(unit)
  else state.players.push(unit)
}

function setFinite(unit: BattleUnit, key: "hp" | "mp" | "maxHp" | "maxMp", raw: unknown) {
  if (raw === undefined || raw === null) return
  const value = Number(raw)
  if (Number.isFinite(value)) unit[key] = value
}

function setPositiveInteger(state: BattleRunStatePayload, key: "wave", raw: unknown) {
  const value = Math.trunc(Number(raw))
  if (Number.isFinite(value) && value > 0) state[key] = value
}

function setNonNegativeInteger(state: BattleRunStatePayload, key: "completedBattles", raw: unknown) {
  const value = Math.trunc(Number(raw))
  if (Number.isFinite(value) && value >= 0) state[key] = value
}

function finiteInteger(raw: unknown, fallback: number | undefined): number | undefined {
  if (raw === undefined || raw === null) return fallback
  const value = Math.trunc(Number(raw))
  return Number.isFinite(value) ? value : fallback
}

function cloneNumberRecord(raw: Record<string, number> | undefined): Record<string, number> | undefined {
  return raw ? { ...raw } : undefined
}

function cloneNestedNumberRecord(raw: Record<string, Record<string, number>> | undefined) {
  if (!raw) return undefined
  return Object.fromEntries(Object.entries(raw).map(([key, value]) => [key, { ...value }]))
}

function toNumberRecord(raw: unknown): Record<string, number> {
  const output: Record<string, number> = {}
  for (const [key, value] of Object.entries(asRecord(raw))) {
    const parsed = Number(value)
    if (key && Number.isFinite(parsed)) output[key] = parsed
  }
  return output
}

function toNestedNumberRecord(raw: unknown): Record<string, Record<string, number>> {
  const output: Record<string, Record<string, number>> = {}
  for (const [key, value] of Object.entries(asRecord(raw))) {
    if (key) output[key] = toNumberRecord(value)
  }
  return output
}

function normalizeSide(raw: unknown): "player" | "enemy" | "" {
  const side = String(raw || "").trim().toLowerCase()
  if (side === "player" || side === "enemy") return side
  if (side === "0") return "player"
  if (side === "1") return "enemy"
  return ""
}

function normalizedId(raw: unknown): string {
  const value = String(raw || "").trim()
  return value === "<nil>" || value === "nil" || value === "null" ? "" : value
}

function asRecord(raw: unknown): Record<string, unknown> {
  return raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
}

function asArray(raw: unknown): unknown[] {
  return Array.isArray(raw) ? raw : []
}
