import { describe, expect, it } from "vitest"
import {
  activeArenaKittySelections,
  arenaCooldownSecondsRemaining,
  arenaPositionOrder,
  formatArenaCooldown,
  isArenaChallengeTerminal,
  isArenaLineupSelectionValid,
  projectArenaChallengeEntries
} from "../src/services/arena"
import { battlePayloadStreamKind, normalizeTimelineNodeData, parseRunStateLoose } from "../src/composables/game/combat/useCombatPage"
import { buildArenaReplayAnalysis, type ArenaReplayQueuedEvent } from "../src/services/arena-replay-analysis"
import type { BattleRunStatePayload } from "../src/composables/game/combat/useCombatPage"
import { applyArenaReplayEventBatch, cloneArenaBattleState } from "../src/services/arena-replay-state"
import { buildArenaFormatRuleViews } from "../src/services/arena-format-display"

describe("feline trials contracts", () => {
  it("rounds cooldowns up and formats the countdown without unlocking early", () => {
    const now = Date.parse("2026-08-30T12:00:00.000Z")
    expect(arenaCooldownSecondsRemaining("2026-08-30T12:05:00.001Z", now)).toBe(301)
    expect(arenaCooldownSecondsRemaining("2026-08-30T12:00:00.000Z", now)).toBe(0)
    expect(arenaCooldownSecondsRemaining(null, now)).toBe(0)
    expect(formatArenaCooldown(301)).toBe("05:01")
    expect(formatArenaCooldown(3661)).toBe("1:01:01")
  })

  it("projects lazily recovered match entries while the page stays open", () => {
    const entries = {
      serverTime: "2026-08-30T12:00:00.000Z",
      remaining: 2,
      capacity: 10,
      recoverySeconds: 600,
      nextRecoveryAt: "2026-08-30T12:10:00.000Z",
      opponentCooldownSeconds: 900
    }
    expect(projectArenaChallengeEntries(entries, Date.parse("2026-08-30T12:25:00.000Z"))).toEqual({
      remaining: 4,
      capacity: 10,
      secondsToNext: 300,
      nextRecoveryAtMs: Date.parse("2026-08-30T12:30:00.000Z")
    })
    expect(projectArenaChallengeEntries(entries, Date.parse("2026-08-30T14:00:00.000Z"))).toEqual({
      remaining: 10,
      capacity: 10,
      secondsToNext: 0,
      nextRecoveryAtMs: null
    })
  })

  it("keeps the three formation lanes in battle order", () => {
    expect(["rearguard", "vanguard", "midguard"].sort((left, right) => arenaPositionOrder(left) - arenaPositionOrder(right)))
      .toEqual(["vanguard", "midguard", "rearguard"])
  })

  it("polls only queued and running challenges", () => {
    expect(isArenaChallengeTerminal("queued")).toBe(false)
    expect(isArenaChallengeTerminal("running")).toBe(false)
    expect(isArenaChallengeTerminal("attacker_win")).toBe(true)
    expect(isArenaChallengeTerminal("defender_win")).toBe(true)
    expect(isArenaChallengeTerminal("failed")).toBe(true)
  })

  it("allows a player to publish with zero, one, or two distinct kitties", () => {
    expect(isArenaLineupSelectionValid({ playerPosition: "vanguard", formatId: "standard", kitties: [] })).toBe(true)
    expect(isArenaLineupSelectionValid({
      playerPosition: "vanguard", formatId: "standard",
      kitties: [{ kittyId: 10, position: "midguard" }]
    })).toBe(true)
    expect(isArenaLineupSelectionValid({
      playerPosition: "vanguard", formatId: "three_layer_boxes",
      kitties: [{ kittyId: 10, position: "midguard" }, { kittyId: 11, position: "rearguard" }]
    })).toBe(true)
    expect(isArenaLineupSelectionValid({
      playerPosition: "vanguard", formatId: "standard",
      kitties: [{ kittyId: 10, position: "midguard" }, { kittyId: 10, position: "rearguard" }]
    })).toBe(false)
    expect(isArenaLineupSelectionValid({
      playerPosition: "vanguard", formatId: "standard",
      kitties: [{ kittyId: 10, position: "vanguard" }]
    })).toBe(true)
    expect(isArenaLineupSelectionValid({
      playerPosition: "vanguard", formatId: "standard",
      kitties: [{ kittyId: 10, position: "vanguard" }, { kittyId: 11, position: "vanguard" }]
    })).toBe(true)
  })

  it("omits empty editor slots from the published kitty selection", () => {
    expect(activeArenaKittySelections({
      playerPosition: "vanguard", formatId: "standard",
      kitties: [{ kittyId: 10, position: "midguard" }, { kittyId: 0, position: "rearguard" }]
    })).toEqual([{ kittyId: 10, position: "midguard" }])
  })

  it("requires an authored format id on every published formation", () => {
    expect(isArenaLineupSelectionValid({ playerPosition: "vanguard", formatId: "", kitties: [] })).toBe(false)
    expect(isArenaLineupSelectionValid({ playerPosition: "vanguard", formatId: "Natural_Talents", kitties: [] })).toBe(true)
  })

  it("translates standard matchup HP, slow start, and specialization policy into concise player copy", () => {
    const translations: Record<string, string> = {
      "game.arena.formatRuleDetails.eventBattleStart": "战斗开始",
      "game.arena.formatRuleDetails.eventLineupSnapshot": "冻结阵容",
      "game.arena.formatRuleDetails.targetFormalUnitsBySide": "双方正式单位",
      "game.arena.formatRuleDetails.targetWarmupEligibleSide": "慢热一方全体",
      "game.arena.formatRuleDetails.targetPlayerUnits": "双方玩家",
      "game.arena.formatRuleDetails.matchupHpHighLevel": "双方>={level} +{value}",
      "game.arena.formatRuleDetails.matchupHpLowerSide": "低方每级 +{value}",
      "game.arena.formatRuleDetails.warmupEligibility": "差>{gap} 或等级>{level}",
      "game.arena.formatRuleDetails.warmupStages": "输出阶段 {values}",
      "game.arena.formatRuleDetails.warmupProgression": "初始 {start}，每次恢复 {recovery}，最高 {maximum}，{actions} 次结束",
      "game.arena.formatRuleDetails.warmupFormula": "不应显示 min({maximum}, {start} + n × {recovery})",
      "game.arena.formatRuleDetails.formalOnly": "仅正式单位",
      "game.arena.formatRuleDetails.summonsDoNotInherit": "召唤不继承",
      "game.arena.formatRuleDetails.playerLevelsOnly": "只看玩家等级",
      "game.arena.formatRuleDetails.warmupIndependent": "独立进度且不可驱散",
      "game.arena.formatRuleDetails.warmupNormalActionsOnly": "仅正常行动推进",
      "game.arena.formatRuleDetails.warmupSummonsFresh": "召唤物从第一档开始",
      "game.arena.formatRuleDetails.specializationDisabled": "双方不载入专精",
      "game.arena.formatRuleDetails.policyFrozenWithLineup": "随阵容冻结",
      "game.arena.formatRuleDetails.kittyPersonalityUnaffected": "猫咪性格不变"
    }
    const translate = (key: string, params: Record<string, unknown> = {}) => Object.entries(params)
      .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), translations[key] || key)
    const common = { skipFinalDamageModifiers: false, dispatchHooks: false, simultaneous: false }
    const views = buildArenaFormatRuleViews([{
      ...common,
      event: "battle_start",
      effectKind: "matchup_hp_buff",
      targetScope: "formal_units_by_side",
      includeSummons: false,
      buffId: "arena_matchup_hp_adjustment",
      payloadKey: "hpBonus",
      matchupHp: {
        levelSource: "player_battle_level",
        bothAtLeastLevel: 400,
        bothSidesFlatBonus: 50000,
        lowerSideBonusPerLevel: 100
      }
    }, {
      ...common,
      event: "battle_start",
      effectKind: "warmup_buff",
      targetScope: "eligible_side_units",
      includeSummons: true,
      buffId: "arena_output_warmup",
      warmup: {
        levelSource: "player_battle_level",
        higherSideGapExceeds: 100,
        alwaysAboveLevel: 400,
        normalActions: 2,
        startMultiplier: 0.4,
        recoveryPerNormalAction: 0.3,
        maximumMultiplier: 1,
        outputs: ["damage", "heal"]
      }
    }, {
      ...common,
      event: "lineup_snapshot",
      effectKind: "specialization_policy",
      targetScope: "player_units",
      includeSummons: false,
      specializationEnabled: false
    }], translate)

    expect(views[0]?.effects.map((line) => line.text)).toEqual([
      "双方>=400 +50000", "低方每级 +100"
    ])
    expect(views[0]?.settlement.map((line) => line.text)).toEqual(["仅正式单位 · 召唤不继承 · 只看玩家等级"])
    expect(views[1]?.effects.map((line) => line.text)).toEqual([
      "差>100 或等级>400", "输出阶段 40% → 70% → 100%"
    ])
    expect(views[1]?.settlement.map((line) => line.text)).toEqual(["独立进度且不可驱散 · 仅正常行动推进 · 召唤物从第一档开始"])
    expect(views[2]).toMatchObject({ event: "冻结阵容", target: "双方玩家" })
    expect(views[2]?.effects[0]?.text).toBe("双方不载入专精")
    expect(views[2]?.settlement.map((line) => line.text)).toEqual(["随阵容冻结 · 猫咪性格不变"])

    const visibleText = views.flatMap((view) => [
      view.event,
      view.target,
      ...view.effects.map((line) => line.text),
      ...view.settlement.map((line) => line.text)
    ]).join(" ")
    expect(visibleText).not.toMatch(/\b(?:floor|max|min|clamp)\s*\(/i)
  })

  it("translates frozen format damage values without exposing implementation formulas", () => {
    const translations: Record<string, string> = {
      "game.arena.formatRuleDetails.eventAfterSummon": "召唤后",
      "game.arena.formatRuleDetails.targetAllAliveIncludingSummons": "所有存活单位",
      "game.arena.formatRuleDetails.damageMaxHpFlat": "伤害 {percent} + {flat}（{damageType}）",
      "game.arena.formatRuleDetails.damageFormula": "不应显示 clamp({formula})",
      "game.arena.formatRuleDetails.damageMinimum": "最低 {value}",
      "game.arena.formatRuleDetails.simultaneous": "同时结算",
      "game.arena.formatRuleDetails.skipFinalDamageModifiers": "忽略最终增减",
      "game.arena.formatRuleDetails.noHooks": "不触发钩子",
      "game.damageType.physical": "物理"
    }
    const translate = (key: string, params: Record<string, unknown> = {}) => Object.entries(params)
      .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), translations[key] || key)

    const views = buildArenaFormatRuleViews([{
      event: "after_summon",
      effectKind: "damage",
      targetScope: "all_alive_units",
      includeSummons: true,
      damageType: "physical",
      skipFinalDamageModifiers: true,
      dispatchHooks: false,
      simultaneous: true,
      amount: {
        basis: "target_max_hp",
        multiplier: 0.04,
        flat: 30,
        minimum: 1
      }
    }], translate)

    expect(views[0]).toMatchObject({ event: "召唤后", target: "所有存活单位" })
    expect(views[0]?.effects.map((line) => line.text)).toEqual([
      "伤害 4% + 30（物理）",
      "最低 1"
    ])
    expect(views[0]?.settlement.map((line) => line.text)).toEqual(["同时结算 · 忽略最终增减 · 不触发钩子"])
    expect(JSON.stringify(views)).not.toMatch(/\b(?:floor|max|min|clamp)\s*\(/i)
  })

  it("keeps trial and ordinary battle event streams independent", () => {
    expect(battlePayloadStreamKind({ k: "arena", m: 12 })).toBe("arena")
    expect(battlePayloadStreamKind({ battleKind: "arena", roomId: 12 })).toBe("arena")
    expect(battlePayloadStreamKind({ m: 12 })).toBe("room")
  })

  it("preserves authored trial end-condition labels in compact replay snapshots", () => {
    const state = parseRunStateLoose(0, {
      ctf: {
        id: "boss_rush",
        nameKey: "game.combatTrial.formats.bossRush.name",
        icon: "lucide:crown",
        actionBudget: 30,
        metrics: [{ id: "actions_to_victory" }],
        endConditions: [{
          kind: "enemy_team_defeated",
          reason: "boss_defeated",
          labelKey: "game.combatTrial.endReason.bossDefeated"
        }]
      }
    })

    expect(state.combatTrialFormat?.endConditions).toEqual([{
      kind: "enemy_team_defeated",
      reason: "boss_defeated",
      labelKey: "game.combatTrial.endReason.bossDefeated"
    }])
    expect(cloneArenaBattleState(state).combatTrialFormat?.endConditions).toEqual(state.combatTrialFormat?.endConditions)
  })

  it("groups synchronized child actions into their parent tactical turn", () => {
    const snapshot: BattleRunStatePayload = {
      roomId: 99,
      wave: 1,
      completedBattles: 0,
      seq: 0,
      players: [
        { unitId: "attacker", name: "Attacker", side: "player", hp: 100, maxHp: 100, alive: true },
        { unitId: "summon", name: "Summon", side: "player", hp: 40, maxHp: 40, alive: true }
      ],
      enemies: [{ unitId: "defender", name: "Defender", side: "enemy", hp: 100, maxHp: 100, alive: true }]
    }
    const events: ArenaReplayQueuedEvent[] = [
      { offsetMs: 0, event: { seq: 1, type: "battle.wave.spawn", data: {} } },
      {
        offsetMs: 100,
        event: {
          seq: 10,
          type: "battle.timeline.node",
          data: {
            wave: 1,
            turnOwner: { unitId: "attacker", side: "player" },
            steps: [
              { kind: "cast", phase: "action", skill: { skillId: "slash", skillName: "Slash" } },
              { kind: "damage", amount: 40, crit: true, source: { unitId: "attacker", side: "player" }, targets: [{ unitId: "defender", hpBefore: 100, hpAfter: 60, aliveAfter: true }] }
            ]
          }
        }
      },
      {
        offsetMs: 100,
        event: {
          seq: 11,
          type: "battle.timeline.node",
          data: {
            parentSeq: 10,
            turnOwner: { unitId: "summon", side: "player" },
            steps: [
              { kind: "damage", amount: 60, source: { unitId: "summon", side: "player" }, targets: [{ unitId: "defender", hpBefore: 60, hpAfter: 0, aliveAfter: false }] }
            ]
          }
        }
      },
      {
        offsetMs: 800,
        event: {
          seq: 12,
          type: "battle.timeline.node",
          data: {
            turnOwner: { unitId: "defender", side: "enemy" },
            steps: [
              { kind: "heal", source: { unitId: "defender", side: "enemy" }, targets: [{ unitId: "defender", hpBefore: 20, hpAfter: 70, aliveAfter: true }] }
            ]
          }
        }
      }
    ]

    const analysis = buildArenaReplayAnalysis(events, snapshot)

    expect(analysis.turns).toHaveLength(2)
    expect(analysis.turns[0]).toMatchObject({
      eventStart: 1,
      eventEnd: 3,
      actorName: "Attacker",
      skillId: "slash",
      syncedTurn: true,
      damage: 100,
      criticalHits: 1,
      knockouts: 1,
      keyMoment: true
    })
    expect(analysis.turns[1]).toMatchObject({ eventStart: 3, eventEnd: 4, healing: 50 })
    expect(analysis.player).toMatchObject({ damage: 100, criticalHits: 1, knockouts: 1 })
    expect(analysis.enemy.healing).toBe(50)
    expect(analysis.actors.find((actor) => actor.actorId === "summon")).toMatchObject({ damage: 60, turns: 1 })
  })

  it("uses authoritative HP deltas when a replay step omits its amount", () => {
    const analysis = buildArenaReplayAnalysis([{
      offsetMs: 0,
      event: {
        seq: 1,
        type: "battle.timeline.node",
        data: {
          turnOwner: { unitId: "player", side: "player" },
          steps: [{
            kind: "damage",
            source: { unitId: "player", side: "player" },
            targets: [{ unitId: "enemy", hpBefore: 91, hpAfter: 66, aliveAfter: true }]
          }]
        }
      }
    }], {
      roomId: 1,
      wave: 1,
      completedBattles: 0,
      seq: 0,
      players: [{ unitId: "player", name: "Player", side: "player" }],
      enemies: [{ unitId: "enemy", name: "Enemy", side: "enemy" }]
    })

    expect(analysis.turns[0]?.damage).toBe(25)
  })

  it("rebuilds cooldowns, buffs, stat snapshots, and summons without mutating the saved snapshot", () => {
    const snapshot: BattleRunStatePayload = {
      roomId: 7,
      wave: 1,
      completedBattles: 0,
      seq: 0,
      players: [{
        unitId: "player",
        name: "Player",
        side: "player",
        position: "vanguard",
        hp: 100,
        maxHp: 100,
        alive: true,
        skillCooldowns: {},
        buffs: [{ buffId: "old_guard", remainTurns: 2, payload: { source: "snapshot" } }]
      }],
      enemies: [{ unitId: "enemy", name: "Enemy", side: "enemy", position: "vanguard", hp: 90, maxHp: 90, alive: true }]
    }

    const rebuilt = applyArenaReplayEventBatch(snapshot, [{
      seq: 10,
      type: "battle.timeline.node",
      data: {
        wave: 2,
        completedBattles: 1,
        turnOwner: { unitId: "player", side: "player" },
        skillCooldowns: { power_strike: 2 },
        skillCooldownTurns: { power_strike: 4 },
        itemCooldowns: { healing_broth: 1 },
        summary: { actor: { hp: 82, mp: 30, alive: true } },
        unitUpdates: [{ unitId: "enemy", stats: { defense: 44 }, position: "midguard" }],
        steps: [
          {
            kind: "buff",
            buffId: "focused",
            buffDurationTurns: 3,
            buffPayload: { source: "specialization" },
            targets: [{ unitId: "player" }]
          },
          {
            kind: "summon",
            summon: { unitId: "echo", name: "Echo", side: "player", position: "rearguard", hp: 30, maxHp: 30, alive: true },
            targets: []
          }
        ]
      }
    }])

    expect(rebuilt).toMatchObject({ wave: 2, completedBattles: 1, seq: 10 })
    expect(rebuilt.players.find((unit) => unit.unitId === "player")).toMatchObject({
      hp: 82,
      mp: 30,
      skillCooldowns: { power_strike: 2 },
      skillCooldownTurns: { power_strike: 4 },
      itemCooldowns: { healing_broth: 1 }
    })
    expect(rebuilt.players.find((unit) => unit.unitId === "player")?.buffs).toEqual([
      { buffId: "old_guard", remainTurns: 2, payload: { source: "snapshot" } },
      { buffId: "focused", remainTurns: 3, triggered: undefined, payload: { source: "specialization" } }
    ])
    expect(rebuilt.enemies[0]).toMatchObject({ position: "midguard", stats: { defense: 44 } })
    expect(rebuilt.players.find((unit) => unit.unitId === "echo")).toMatchObject({ position: "rearguard", hp: 30 })
    expect(snapshot).toMatchObject({ wave: 1, completedBattles: 0, seq: 0 })
    expect(snapshot.players[0]?.buffs).toEqual([{ buffId: "old_guard", remainTurns: 2, payload: { source: "snapshot" } }])
    expect(snapshot.players[0]?.skillCooldowns).toEqual({})
  })

  it("rebuilds generic combat trial progress from compact timeline nodes", () => {
    const snapshot: BattleRunStatePayload = {
      roomId: 31,
      wave: 1,
      completedBattles: 0,
      seq: 0,
      players: [{ unitId: "player", name: "Player", side: "player", hp: 100, maxHp: 100, alive: true }],
      enemies: [{ unitId: "dummy", name: "Dummy", side: "enemy", hp: 100, maxHp: 100, alive: true }],
      combatTrial: { progressVersion: 1, actionCount: 0, actionBudget: 20, metrics: { damage_total: 0, damage_max_segment: 0 } }
    }
    const data = normalizeTimelineNodeData({
      w: 1,
      o: "player",
      p: [],
      combatTrial: {
        progressVersion: 1,
        actionCount: 1,
        actionBudget: 20,
        metrics: { damage_total: 103, damage_max_segment: 103 }
      }
    })
    const rebuilt = applyArenaReplayEventBatch(snapshot, [{ seq: 4, type: "battle.timeline.node", data }])

    expect(rebuilt.combatTrial).toEqual({
      progressVersion: 1,
      progressSeq: 4,
      actionCount: 1,
      actionBudget: 20,
      endReason: undefined,
      metrics: { damage_total: 103, damage_max_segment: 103 }
    })
    expect(snapshot.combatTrial?.metrics.damage_total).toBe(0)
  })

  it("carries a frozen match format through replay snapshots and wave events", () => {
    const snapshot: BattleRunStatePayload = {
      roomId: 17,
      wave: 1,
      completedBattles: 0,
      seq: 0,
      players: [{ unitId: "player", name: "Player", side: "player", hp: 100, maxHp: 100, alive: true }],
      enemies: [{ unitId: "enemy", name: "Enemy", side: "enemy", hp: 100, maxHp: 100, alive: true }],
      arenaFormat: {
        id: "standard",
        nameKey: "game.arena.formats.standard.name",
        shortDescKey: "game.arena.formats.standard.shortDesc",
        descKey: "game.arena.formats.standard.desc",
        icon: "lucide:circle-off",
        standard: true
      }
    }
    const rebuilt = applyArenaReplayEventBatch(snapshot, [{
      seq: 1,
      type: "battle.wave.spawn",
      data: {
        players: snapshot.players,
        enemies: snapshot.enemies,
        arenaFormat: {
          id: "three_layer_boxes",
          nameKey: "game.arena.formats.three_layer_boxes.name",
          shortDescKey: "game.arena.formats.three_layer_boxes.shortDesc",
          descKey: "game.arena.formats.three_layer_boxes.desc",
          icon: "game-icons:cardboard-box",
          standard: false,
          definitionDigest: "frozen-digest"
        }
      }
    }])

    expect(rebuilt.arenaFormat).toMatchObject({ id: "three_layer_boxes", definitionDigest: "frozen-digest" })
    expect(snapshot.arenaFormat?.id).toBe("standard")
  })
})
