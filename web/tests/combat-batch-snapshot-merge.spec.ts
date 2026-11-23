import { describe, expect, it } from "vitest"
import {
  applyBattlefieldTargetResult,
  expandSnapshotUnits,
  filterBatchEventsAfterSnapshot,
  mergeReplayEvents,
  normalizeTimelineNodeData,
  parseBattlefieldEffectStates
} from "@/composables/game/combat/useCombatPage"

describe("combat batch snapshot merge", () => {
  it("drops events already covered by snapshot seq", () => {
    const events = [
      { seq: 98, type: "battle.timeline.node" },
      { seq: 99, type: "battle.timeline.node" },
      { seq: 100, type: "battle.wave.spawn" },
      { seq: 101, type: "battle.timeline.node" }
    ]
    const filtered = filterBatchEventsAfterSnapshot(events, 100)
    expect(filtered).toEqual([{ seq: 101, type: "battle.timeline.node" }])
  })

  it("keeps all events when snapshot seq is empty", () => {
    const events = [
      { seq: 1, type: "battle.wave.spawn" },
      { seq: 2, type: "battle.timeline.node" }
    ]
    const filtered = filterBatchEventsAfterSnapshot(events, 0)
    expect(filtered).toEqual(events)
  })

  it("keeps snapshot-covered batch events for replay while de-duplicating older seq", () => {
    const current = [{ seq: 9, type: "battle.timeline.node" }]
    const incoming = [
      { seq: 9, type: "battle.timeline.node" },
      { seq: 10, type: "battle.timeline.node" },
      { seq: 11, type: "battle.timeline.node" }
    ]

    expect(mergeReplayEvents(current, incoming)).toEqual([
      { seq: 9, type: "battle.timeline.node" },
      { seq: 10, type: "battle.timeline.node" },
      { seq: 11, type: "battle.timeline.node" }
    ])
  })

  it("decodes full and compact battlefield effect snapshots", () => {
    expect(parseBattlefieldEffectStates([{
      id: "meteor_corridor_starburst",
      remaining: 6,
      progress: 2,
      goal: 3,
      windows: { timekeeper_exposed: 1 }
    }])).toEqual([{
      id: "meteor_corridor_starburst",
      remaining: 6,
      progress: 2,
      goal: 3,
      windows: { timekeeper_exposed: 1 }
    }])
    expect(parseBattlefieldEffectStates([{
      i: "meteor_corridor_starburst",
      r: 5,
      p: 1,
      g: 2,
      w: { timekeeper_exposed: 2 }
    }])).toEqual([{
      id: "meteor_corridor_starburst",
      remaining: 5,
      progress: 1,
      goal: 2,
      windows: { timekeeper_exposed: 2 }
    }])
  })

  it("de-duplicates repeated or out-of-order battlefield events by sequence", () => {
    const current = [{ seq: 20, type: "battle.field.tick" }]
    const incoming = [
      { seq: 19, type: "battle.field.tick" },
      { seq: 20, type: "battle.field.tick" },
      { seq: 21, type: "battle.field.trigger" },
      { seq: 22, type: "battle.field.window" }
    ]
    expect(mergeReplayEvents(current, incoming)).toEqual([
      { seq: 20, type: "battle.field.tick" },
      { seq: 21, type: "battle.field.trigger" },
      { seq: 22, type: "battle.field.window" }
    ])
  })

  it("applies authoritative audit buffs and cooldown snapshots idempotently", () => {
    const unit = {
      hp: 900,
      alive: true,
      buffs: [],
      skillCooldowns: { spell: 1, basic_attack: 0 },
      skills: [
        { id: "spell", cooldownRemain: 1 },
        { id: "basic_attack", cooldownRemain: 0 }
      ]
    }
    const result = {
      afterHp: 880,
      alive: true,
      buffs: [{ buffId: "silence", remainTurns: 1, payload: {} }],
      skillCooldowns: { spell: 2, basic_attack: 0 }
    }

    applyBattlefieldTargetResult(unit, result)
    applyBattlefieldTargetResult(unit, result)

    expect(unit).toMatchObject({
      hp: 880,
      alive: true,
      buffs: [{ buffId: "silence", remainTurns: 1 }],
      skillCooldowns: { spell: 2, basic_attack: 0 },
      skills: [
        { id: "spell", cooldownRemain: 2 },
        { id: "basic_attack", cooldownRemain: 0 }
      ]
    })
  })

  it("preserves specialization contribution sources from compact timeline metadata", () => {
    const normalized = normalizeTimelineNodeData({
      w: 1,
      o: { u: "player-1" },
      p: [{
        k: "d",
        s: "player-1",
        v: 120,
        t: [{ u: "enemy-1", b: 200, h: 80 }],
        me: {
          ra: 0.2,
          sourceKind: "specialization",
          nodeId: "strength_root_break_stance",
          sourceNameKey: "specialization.node.strength_root_break_stance.name",
          hookId: "strength_break_momentum_release",
          mutationId: "mutation_strength_scaling_append",
          hm: 0.625,
          rs: [{
            kind: "specialization",
            id: "strength_root_break_stance",
            nodeId: "strength_root_break_stance",
            sourceNameKey: "specialization.node.strength_root_break_stance.name",
            runtimeDamageAmp: 0.2
          }],
          ms: [{
            mutationId: "mutation_strength_scaling_append",
            sourceKind: "specialization",
            sourceId: "strength_root_break_stance",
            sourceNameKey: "specialization.node.strength_root_break_stance.name"
          }]
        }
      }]
    })

    expect(normalized.steps?.[0]?.meta).toMatchObject({
      sourceKind: "specialization",
      nodeId: "strength_root_break_stance",
      sourceNameKey: "specialization.node.strength_root_break_stance.name",
      hookId: "strength_break_momentum_release",
      mutationId: "mutation_strength_scaling_append",
      hitDamageMultiplier: 0.625,
      runtimeDamageAmpSources: [{
        kind: "specialization",
        nodeId: "strength_root_break_stance",
        sourceNameKey: "specialization.node.strength_root_break_stance.name",
        runtimeDamageAmp: 0.2
      }],
      mutationSources: [{
        mutationId: "mutation_strength_scaling_append",
        sourceKind: "specialization",
        sourceId: "strength_root_break_stance",
        sourceNameKey: "specialization.node.strength_root_break_stance.name"
      }]
    })
  })

  it("preserves formal extra-turn and authoritative cooldown state", () => {
    const normalized = normalizeTimelineNodeData({
      w: 2,
      o: "player-1",
      et: true,
      tk: "extra",
      sc: { spell: 1, basic_attack: 0 },
      st: { spell: 2, basic_attack: 0 },
      ic: { potion: 0 },
      p: [{ k: "c", s: "player-1", i: "basic_attack" }]
    })

    expect(normalized).toMatchObject({
      extraTurn: true,
      turnKind: "extra",
      skillCooldowns: { spell: 1, basic_attack: 0 },
      skillCooldownTurns: { spell: 2, basic_attack: 0 },
      itemCooldowns: { potion: 0 }
    })
  })

  it("restores effective cooldown totals from compact snapshots", () => {
    const units = expandSnapshotUnits([{
      u: "player-1",
      s: "p",
      h: 100,
      m: 80,
      x: 100,
      y: 80,
      i: ["spell", "basic_attack"],
      d: { spell: 1 },
      st: { spell: 2, basic_attack: 0 }
    }], "player")

    expect(units[0]).toMatchObject({
      skillCooldowns: { spell: 1 },
      skillCooldownTurns: { spell: 2, basic_attack: 0 },
      skills: [
        { id: "spell", cooldown: 2, cooldownTurns: 2, cooldownRemain: 1 },
        { id: "basic_attack", cooldown: 0, cooldownTurns: 0, cooldownRemain: 0 }
      ]
    })
  })

  it("preserves charged buff progress in snapshots and timeline updates", () => {
    const units = expandSnapshotUnits([{
      u: "enemy-1",
      s: "e",
      h: 100,
      m: 20,
      x: 100,
      y: 20,
      f: [{ i: "parade_three_act_curtain", t: 99, g: 1 }]
    }], "enemy")
    expect(units[0]?.buffs).toEqual([{
      buffId: "parade_three_act_curtain",
      remainTurns: 99,
      payload: {},
      triggered: 1
    }])

    const normalized = normalizeTimelineNodeData({
      p: [{
        k: "bc",
        s: "player-1",
        bi: "parade_three_act_curtain",
        bt: 99,
        bg: 2,
        t: [{ u: "enemy-1" }]
      }]
    })
    expect(normalized.steps?.[0]).toMatchObject({
      kind: "buff_charge",
      buffId: "parade_three_act_curtain",
      buffDurationTurns: 99,
      buffTriggered: 2,
      targets: [{ unitId: "enemy-1" }]
    })
  })

  it("preserves resource and stat audit details used by the text battle log", () => {
    const normalized = normalizeTimelineNodeData({
      p: [
        {
          k: "r",
          s: "player-1",
          amount: -10,
          resource: "mp",
          t: [{ u: "player-1", beforeMp: 40, m: 30 }]
        },
        {
          k: "stat_recalc",
          s: "player-1",
          reason: "buff_attached",
          t: [{
            u: "player-1",
            stats: { attack: 120 },
            baseStats: { attack: 100 },
            statModifiers: { attack: 20 },
            statSources: { attack: { buff: 20 } },
            buffs: [{ buffId: "battle_focus", remainTurns: 2 }]
          }]
        }
      ]
    })

    expect(normalized.steps?.[0]).toMatchObject({
      kind: "resource",
      amount: -10,
      resource: "mp",
      targets: [{ unitId: "player-1", mpBefore: 40, mpAfter: 30 }]
    })
    expect(normalized.steps?.[1]).toMatchObject({
      kind: "stat_recalc",
      reason: "buff_attached",
      targets: [{
        unitId: "player-1",
        stats: { attack: 120 },
        baseStats: { attack: 100 },
        statModifiers: { attack: 20 },
        statSources: { attack: { buff: 20 } },
        buffs: [{ buffId: "battle_focus", remainTurns: 2 }]
      }]
    })
  })
})
