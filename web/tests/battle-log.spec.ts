import { describe, expect, it } from "vitest"
import { normalizeTimelineNodeData } from "../src/composables/game/combat/useCombatPage"
import {
  battleLogEventKey,
  buildBattleLogGroup,
  compressBattleLogGroups,
  formatBattleLogNumber,
  type BattleLogBuildOptions
} from "@/services/battle-log"

function buildOptions(runId = 7): BattleLogBuildOptions {
  return {
    runId,
    t: (key, params = {}) => `${key}:${Object.entries(params).map(([name, value]) => `${name}=${String(value)}`).join(",")}`,
    resolveUnit: (unitId, snapshot) => ({
      kind: "unit",
      id: unitId,
      label: String(snapshot?.name || unitId),
      enemyId: unitId.startsWith("enemy:") ? "training_dummy" : undefined
    }),
    positionLabel: (position) => `position:${position}`,
    sideLabel: (side) => `side:${side}`,
    damageTypeLabel: (damageType) => `damage:${damageType}`,
    resourceLabel: (resource) => resource.toUpperCase(),
    resolveFieldEffectName: (effectId) => `field:${effectId}`,
    resolveSourceLabel: (meta) => String(meta.sourceNameKey || ""),
    formatStatModifier: (key, value) => `${key}:${value}`
  }
}

describe("battle log formatter", () => {
  it("preserves paid domination and its interrupted skill through compact replay normalization", () => {
    const data = normalizeTimelineNodeData({p:[
      {k:"r",s:"player:1",i:"equipment_domination",resource:"hp",v:-15,t:[{u:"player:1",b:100,h:85,l:true}]},
      {k:"r",s:"player:1",i:"equipment_domination",resource:"mp",v:-7,t:[{u:"player:1",beforeMp:50,m:43}]},
      {k:"t",s:"player:1",i:"equipment_domination",t:[{u:"enemy:1"}],me:{interruptedSkillId:"power_strike",interruptedActorId:"enemy:1"}}
    ]})
    const group = buildBattleLogGroup({seq:1,type:"battle.timeline.node",ts:1,data},buildOptions())
    const losses = group?.lines.filter(row=>row.messageKey==="game.battleLogResourceLoss")
    expect(losses?.map(row=>row.values)).toEqual([{amount:"15",resource:"HP"},{amount:"7",resource:"MP"}])
    expect(losses?.[1].details.some(row=>row.text.includes("before=50,after=43"))).toBe(true)
    expect(group?.lines.some(row=>row.messageKey==="game.battleLogResourceGain")).toBe(false)
    const interruption = group?.lines.find(row=>row.messageKey==="game.battleLogSkillInterrupted")
    expect(interruption?.refs.interruptedSkill.id).toBe("power_strike")
    expect(interruption?.refs.target.id).toBe("enemy:1")
  })
  it("names the skill and actor stopped by domination", () => {
    const group = buildBattleLogGroup({seq:1,type:"battle.turn.action",ts:1,data:{actorId:"enemy:1",skillId:"power_strike",actionLog:[
      {kind:"trigger",sourceId:"player:1",targetId:"enemy:1",skillId:"equipment_domination",meta:{interruptedSkillId:"power_strike"}}
    ]}},buildOptions())
    const line = group?.lines.find(row=>row.messageKey==="game.battleLogSkillInterrupted")
    expect(line?.refs.target.id).toBe("enemy:1")
    expect(line?.refs.interruptedSkill.id).toBe("power_strike")
    expect(line?.refs.skill.id).toBe("equipment_domination")
  })
  it("turns an authoritative timeline node into detailed, linked log lines", () => {
    const group = buildBattleLogGroup({
      seq: 42,
      type: "battle.timeline.node",
      ts: 1_700_000_000_000,
      data: {
        wave: 3,
        turnOwner: { unitId: "player:1" },
        preparationTurn: true,
        extraTurn: true,
        steps: [
          { kind: "cast", phase: "item", source: { unitId: "player:1" }, skill: { skillId: "healing_pill" } },
          {
            kind: "resource",
            phase: "item",
            source: { unitId: "player:1" },
            resource: "mp",
            amount: -12,
            targets: [{ unitId: "player:1", mpBefore: 50, mpAfter: 38 }]
          },
          { kind: "cast", phase: "action", source: { unitId: "player:1" }, skill: { skillId: "meteor_slash" } },
          {
            kind: "damage",
            phase: "action",
            source: { unitId: "player:1" },
            amount: 120.125,
            damageType: "physical",
            targets: [{ unitId: "enemy:99:training_dummy", hpBefore: 200, hpAfter: 79.875, crit: true, aliveAfter: false }],
            meta: {
              hitIndex: 2,
              hitCount: 3,
              criticalMultiplier: 1.75,
              runtimeDamageAmp: 0.2,
              sourceNameKey: "specialization.node.test.name",
              sourceItemId: "damage_charm"
            }
          },
          {
            kind: "buff",
            source: { unitId: "player:1" },
            buffId: "burning",
            buffDurationTurns: 2,
            targets: [{ unitId: "enemy:99:training_dummy" }]
          },
          {
            kind: "summon",
            source: { unitId: "player:1" },
            summon: { unitId: "enemy:99:training_dummy", name: "Dummy", side: "player", position: "vanguard", hp: 90, maxHp: 90 }
          },
          {
            kind: "stat_recalc",
            source: { unitId: "player:1" },
            targets: [{ unitId: "player:1", statModifiers: { attack: 15, crit: 0.1 } }]
          }
        ]
      }
    }, buildOptions())

    expect(group).not.toBeNull()
    expect(group).toMatchObject({
      id: "7:42:battle.timeline.node",
      runId: 7,
      wave: 3,
      preparationTurn: true,
      extraTurn: true,
      actor: { id: "player:1" }
    })
    expect(group?.lines.map((line) => line.kind)).toEqual([
      "cast",
      "resource",
      "cast",
      "damage",
      "buff",
      "summon",
      "stat_recalc"
    ])

    const resource = group?.lines.find((line) => line.kind === "resource")
    expect(resource).toMatchObject({
      messageKey: "game.battleLogResourceLoss",
      values: { amount: "12", resource: "MP" },
      refs: { target: { id: "player:1" } }
    })
    expect(resource?.details[0]?.text).toContain("before=50,after=38")
    expect(resource?.relatedRefs).toEqual([{ kind: "item", id: "healing_pill" }])

    const damage = group?.lines.find((line) => line.kind === "damage")
    expect(damage).toMatchObject({
      category: "damage",
      values: { amount: "120.13", damageType: "damage:physical" },
      refs: { source: { id: "player:1" }, target: { enemyId: "training_dummy" } }
    })
    expect(damage?.details.map((detail) => detail.text)).toEqual(expect.arrayContaining([
      expect.stringContaining("before=200,after=79.88"),
      expect.stringContaining("index=2,count=3"),
      expect.stringContaining("value=1.75"),
      expect.stringContaining("value=20"),
      expect.stringContaining("game.battleLogDetailDefeated")
    ]))
    expect(damage?.relatedRefs).toEqual([
      { kind: "skill", id: "meteor_slash" },
      { kind: "item", id: "damage_charm" }
    ])

    const buff = group?.lines.find((line) => line.kind === "buff")
    expect(buff?.refs.buff).toMatchObject({ kind: "buff", id: "burning", remainTurns: 2 })
    const summon = group?.lines.find((line) => line.kind === "summon")
    expect(summon?.refs.summon).toMatchObject({ id: "enemy:99:training_dummy", label: "Dummy" })
    const recalc = group?.lines.find((line) => line.kind === "stat_recalc")
    expect(recalc?.details.map((detail) => detail.text)).toEqual(["attack:15", "crit:0.1"])
  })

  it("supports field events and legacy item-use packets", () => {
    const field = buildBattleLogGroup({
      seq: 8,
      type: "battle.field.trigger",
      data: {
        wave: 2,
        effectId: "falling_stars",
        targets: [
          { unitId: "enemy:1:training_dummy", damage: 33, beforeHp: 50, afterHp: 17 },
          {
            unitId: "player:1",
            outcomeId: "silence",
            statusId: "silence",
            durationTurns: 1,
            buffs: [{ buffId: "silence", remainTurns: 1, payload: { blocked: true }, triggered: 2 }]
          },
          {
            unitId: "player:2",
            outcomeId: "sealed",
            cooldownChanges: [{ skillId: "arcane_missile", before: 1, after: 2 }]
          }
        ]
      }
    }, buildOptions(9))
    expect(field?.lines.map((line) => line.kind)).toEqual([
      "field_trigger",
      "field_damage",
      "field_outcome",
      "field_outcome"
    ])
    expect(field?.lines[1]).toMatchObject({
      messageKey: "game.battleLogFieldDamage",
      values: { field: "field:falling_stars", amount: "33" }
    })
    expect(field?.lines[2]?.relatedRefs).toEqual([
      { kind: "buff", id: "silence", remainTurns: 1, payload: { blocked: true }, stackCount: 2 }
    ])
    expect(field?.lines[2]?.details[0]?.text).toContain("turns=1")
    expect(field?.lines[3]?.relatedRefs).toEqual([
      { kind: "skill", id: "arcane_missile" }
    ])
    expect(field?.lines[3]?.details[0]?.text).toContain("before=1,after=2")

    const item = buildBattleLogGroup({
      seq: 9,
      type: "battle.turn.item_use",
      data: {
        wave: 2,
        actorId: "player:1",
        itemId: "healing_pill",
        actionLog: [{ kind: "heal", targetId: "player:1", amount: 25, beforeHp: 50, afterHp: 75 }]
      }
    }, buildOptions(9))
    expect(item?.lines.map((line) => line.messageKey)).toEqual([
      "game.battleLogUseItem",
      "game.battleLogHeal"
    ])
    expect(item?.lines[1]?.relatedRefs).toEqual([{ kind: "item", id: "healing_pill" }])
  })

  it("uses stable event keys and compact number formatting", () => {
    expect(battleLogEventKey(12, { seq: 4, type: "battle.wave.spawn" })).toBe("12:4:battle.wave.spawn")
    expect(formatBattleLogNumber(12.3456)).toBe("12.35")
    expect(formatBattleLogNumber(Number.NaN)).toBe("0")
  })

  it("compresses old detail lines in batches while keeping recent groups intact", () => {
    const groups = Array.from({ length: 110 }, (_, index) => buildBattleLogGroup({
      seq: index + 1,
      type: "battle.wave.win",
      data: { wave: index + 1, completedBattles: index + 1 }
    }, buildOptions(12))).filter((group) => group !== null)

    const compressed = compressBattleLogGroups(groups, 100, 10)

    expect(compressed).toMatchObject({
      removedLineCount: 20,
      firstWave: 1,
      lastWave: 20
    })
    expect(compressed.groups).toHaveLength(90)
    expect(compressed.groups[0]?.wave).toBe(21)
    expect(compressed.groups.at(-1)?.wave).toBe(110)
  })
})
