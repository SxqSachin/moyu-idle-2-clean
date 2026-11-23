import type { ArenaFormatRule } from "@services/arena"
import {
  formatEffectNumber,
  formatEffectPercent,
  type TranslateFn
} from "@services/effect-formula-display"

export type ArenaFormatRuleLine = {
  key: string
  text: string
}

export type ArenaFormatRuleView = {
  key: string
  event: string
  target: string
  effects: ArenaFormatRuleLine[]
  settlement: ArenaFormatRuleLine[]
}

export function buildArenaFormatRuleViews(
  rules: ArenaFormatRule[] | null | undefined,
  t: TranslateFn
): ArenaFormatRuleView[] {
  return (Array.isArray(rules) ? rules : []).map((rule, index) => ({
    key: `${index}-${rule.event}-${rule.effectKind}`,
    event: arenaFormatEventLabel(rule, t),
    target: arenaFormatTargetLabel(rule, t),
    effects: arenaFormatEffectLines(rule, t),
    settlement: arenaFormatSettlementLines(rule, t)
  }))
}

function arenaFormatEventLabel(rule: ArenaFormatRule, t: TranslateFn): string {
  if (rule.event === "lineup_snapshot") return t("game.arena.formatRuleDetails.eventLineupSnapshot")
  if (rule.event === "battle_start") return t("game.arena.formatRuleDetails.eventBattleStart")
  if (rule.event === "after_summon") return t("game.arena.formatRuleDetails.eventAfterSummon")
  return String(rule.event || "-")
}

function arenaFormatTargetLabel(rule: ArenaFormatRule, t: TranslateFn): string {
  if (rule.targetScope === "player_units") {
    return t("game.arena.formatRuleDetails.targetPlayerUnits")
  }
  if (rule.targetScope === "eligible_side_units") {
    return t("game.arena.formatRuleDetails.targetWarmupEligibleSide")
  }
  if (rule.targetScope === "formal_units_by_side") {
    return t("game.arena.formatRuleDetails.targetFormalUnitsBySide")
  }
  if (rule.targetScope === "all_units") {
    return t(rule.includeSummons
      ? "game.arena.formatRuleDetails.targetAllUnitsIncludingSummons"
      : "game.arena.formatRuleDetails.targetAllUnitsFormal")
  }
  if (rule.targetScope === "all_alive_units") {
    return t(rule.includeSummons
      ? "game.arena.formatRuleDetails.targetAllAliveIncludingSummons"
      : "game.arena.formatRuleDetails.targetAllAliveFormal")
  }
  return String(rule.targetScope || "-")
}

function arenaFormatEffectLines(rule: ArenaFormatRule, t: TranslateFn): ArenaFormatRuleLine[] {
  if (rule.effectKind === "warmup_buff") {
    return arenaWarmupLines(rule, t)
  }
  if (rule.effectKind === "matchup_hp_buff") {
    return arenaMatchupHpLines(rule, t)
  }
  if (rule.effectKind === "specialization_policy") {
    return [{
      key: "specialization-policy",
      text: t(rule.specializationEnabled === false
        ? "game.arena.formatRuleDetails.specializationDisabled"
        : "game.arena.formatRuleDetails.specializationEnabled")
    }]
  }
  if (rule.effectKind === "turn_barrier") {
    return [{
      key: "turn-barrier",
      text: t("game.arena.formatRuleDetails.turnBarrier", {
        charges: Math.max(0, Math.trunc(Number(rule.charges || 0)))
      })
    }]
  }
  if (rule.effectKind !== "damage") {
    return [{ key: "unknown-effect", text: String(rule.effectKind || "-") }]
  }

  const amount = rule.amount
  const multiplier = finiteNonNegative(amount?.multiplier)
  const flat = finiteNonNegative(amount?.flat)
  const percent = formatEffectPercent(multiplier)
  const flatText = formatEffectNumber(flat, 2)
  const damageType = localizeDamageType(rule.damageType, t)
  const values = { percent, flat: flatText, damageType }
  const lines: ArenaFormatRuleLine[] = []
  if (multiplier > 0 && flat > 0) {
    lines.push({ key: "damage", text: t("game.arena.formatRuleDetails.damageMaxHpFlat", values) })
  } else if (multiplier > 0) {
    lines.push({ key: "damage", text: t("game.arena.formatRuleDetails.damageMaxHp", values) })
  } else {
    lines.push({ key: "damage", text: t("game.arena.formatRuleDetails.damageFlat", values) })
  }

  const minimum = finiteNonNegative(amount?.minimum)
  const maximum = finiteNonNegative(amount?.maximum)
  if (minimum > 0) {
    lines.push({
      key: "minimum",
      text: t("game.arena.formatRuleDetails.damageMinimum", { value: formatEffectNumber(minimum, 2) })
    })
  }
  if (maximum > 0) {
    lines.push({
      key: "maximum",
      text: t("game.arena.formatRuleDetails.damageMaximum", { value: formatEffectNumber(maximum, 2) })
    })
  }
  return lines
}

function arenaFormatSettlementLines(rule: ArenaFormatRule, t: TranslateFn): ArenaFormatRuleLine[] {
  if (rule.effectKind === "matchup_hp_buff") {
    return compactSettlement([
      { key: "formal-only", text: t("game.arena.formatRuleDetails.formalOnly") },
      { key: "summons-no-inherit", text: t("game.arena.formatRuleDetails.summonsDoNotInherit") },
      { key: "player-levels-only", text: t("game.arena.formatRuleDetails.playerLevelsOnly") }
    ])
  }
  if (rule.effectKind === "warmup_buff") {
    return compactSettlement([
      { key: "warmup-independent", text: t("game.arena.formatRuleDetails.warmupIndependent") },
      { key: "warmup-turns", text: t("game.arena.formatRuleDetails.warmupNormalActionsOnly") },
      { key: "warmup-summons", text: t("game.arena.formatRuleDetails.warmupSummonsFresh") }
    ])
  }
  if (rule.effectKind === "specialization_policy") {
    return compactSettlement([
      { key: "snapshot-policy", text: t("game.arena.formatRuleDetails.policyFrozenWithLineup") },
      { key: "kitty-personality", text: t("game.arena.formatRuleDetails.kittyPersonalityUnaffected") }
    ])
  }
  if (rule.effectKind === "turn_barrier") {
    return [{ key: "turn-barrier-rule", text: t("game.arena.formatRuleDetails.turnBarrierRule") }]
  }
  if (rule.effectKind !== "damage") return []
  const lines: ArenaFormatRuleLine[] = []
  if (rule.simultaneous) {
    lines.push({ key: "simultaneous", text: t("game.arena.formatRuleDetails.simultaneous") })
  }
  if (rule.skipFinalDamageModifiers) {
    lines.push({ key: "skip-final-modifiers", text: t("game.arena.formatRuleDetails.skipFinalDamageModifiers") })
  }
  if (!rule.dispatchHooks) {
    lines.push({ key: "no-hooks", text: t("game.arena.formatRuleDetails.noHooks") })
  }
  return compactSettlement(lines)
}

function arenaWarmupLines(rule: ArenaFormatRule, t: TranslateFn): ArenaFormatRuleLine[] {
  const warmup = rule.warmup
  const gap = positiveInteger(warmup?.higherSideGapExceeds)
  const alwaysAbove = positiveInteger(warmup?.alwaysAboveLevel)
  const normalActions = positiveInteger(warmup?.normalActions)
  const start = finiteNonNegative(warmup?.startMultiplier)
  const recovery = finiteNonNegative(warmup?.recoveryPerNormalAction)
  const maximum = finiteNonNegative(warmup?.maximumMultiplier)
  if (!gap || !alwaysAbove || !normalActions || !recovery || !maximum) {
    return [{ key: "invalid-warmup", text: t("game.arena.formatRuleDetails.warmup") }]
  }
  return [
    {
      key: "warmup-eligibility",
      text: t("game.arena.formatRuleDetails.warmupEligibility", {
        gap: formatEffectNumber(gap, 0),
        level: formatEffectNumber(alwaysAbove, 0)
      })
    },
    {
      key: "warmup-stages",
      text: normalActions <= 4
        ? t("game.arena.formatRuleDetails.warmupStages", {
            values: warmupStageValues(start, recovery, maximum, normalActions)
          })
        : t("game.arena.formatRuleDetails.warmupProgression", {
            start: formatEffectPercent(start),
            recovery: formatEffectPercent(recovery),
            maximum: formatEffectPercent(maximum),
            actions: normalActions
          })
    }
  ]
}

function arenaMatchupHpLines(rule: ArenaFormatRule, t: TranslateFn): ArenaFormatRuleLine[] {
  const matchup = rule.matchupHp
  const threshold = positiveInteger(matchup?.bothAtLeastLevel)
  const bothBonus = finiteNonNegative(matchup?.bothSidesFlatBonus)
  const gapBonus = finiteNonNegative(matchup?.lowerSideBonusPerLevel)
  if (!threshold || !bothBonus || !gapBonus) {
    return [{ key: "invalid-matchup-hp", text: t("game.arena.formatRuleDetails.matchupHp") }]
  }
  return [
    {
      key: "matchup-high-level",
      text: t("game.arena.formatRuleDetails.matchupHpHighLevel", {
        level: formatEffectNumber(threshold, 0),
        value: formatEffectNumber(bothBonus, 0)
      })
    },
    {
      key: "matchup-gap",
      text: t("game.arena.formatRuleDetails.matchupHpLowerSide", {
        value: formatEffectNumber(gapBonus, 0)
      })
    }
  ]
}

function warmupStageValues(start: number, recovery: number, maximum: number, normalActions: number): string {
  return Array.from({ length: normalActions + 1 }, (_, index) => (
    formatEffectPercent(Math.min(maximum, start + recovery * index))
  )).join(" → ")
}

function compactSettlement(lines: ArenaFormatRuleLine[]): ArenaFormatRuleLine[] {
  if (lines.length === 0) return []
  return [{
    key: lines.map((line) => line.key).join("-"),
    text: lines.map((line) => line.text).join(" · ")
  }]
}

function localizeDamageType(raw: string | undefined, t: TranslateFn): string {
  const type = String(raw || "physical").trim().toLowerCase() || "physical"
  const key = `game.damageType.${type}`
  const localized = t(key)
  return localized === key ? type : localized
}

function finiteNonNegative(raw: unknown): number {
  const value = Number(raw)
  return Number.isFinite(value) ? Math.max(0, value) : 0
}

function positiveInteger(raw: unknown): number {
  const value = Math.trunc(Number(raw))
  return Number.isFinite(value) ? Math.max(0, value) : 0
}
