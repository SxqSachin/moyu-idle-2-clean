import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import {
  activeCombatTrialKittySelections,
  combatTrialMetricValue,
  isCombatTrialLineupSelectionValid,
  isCombatTrialMetricAvailableForEndReason,
  normalizeCombatTrialMetrics,
  type CombatTrialKitty,
  type CombatTrialLineupSelection
} from "../src/services/combat-trial"
import { battlePayloadStreamKind, parseRunStateLoose } from "../src/composables/game/combat/useCombatPage"
import { applyArenaReplayEventBatch, cloneArenaBattleState } from "../src/services/arena-replay-state"
import {
  combatTrialEnUS,
  combatTrialJa,
  combatTrialKo,
  combatTrialRu,
  combatTrialZhCN,
  combatTrialZhTW
} from "../src/locales/game/combat-trial"
import { useChatStore } from "../src/stores/chat"

const kitties: CombatTrialKitty[] = [
  { id: 1, name: "Mochi", level: 10, status: "idle", positionLocked: "midguard", positionLockedByPersonalityId: "calm", positionLockedByPersonalityKey: "", },
  { id: 2, name: "Pepper", level: 12, status: "idle", positionLocked: "", positionLockedByPersonalityId: "", positionLockedByPersonalityKey: "", }
]

describe("combat trial contracts", () => {
  beforeEach(() => setActivePinia(createPinia()))

  it("normalizes generic metrics without binding the protocol to damage fields", () => {
    expect(normalizeCombatTrialMetrics({ damage_total: 123.5, healing_total: 44, invalid: -1, nope: "x" })).toEqual({
      damage_total: 123.5,
      healing_total: 44
    })
    expect(combatTrialMetricValue({ damage_max_segment: 77 }, "damage_max_segment")).toBe(77)
    expect(isCombatTrialMetricAvailableForEndReason({ eligibleEndReasons: ["enemy_team_defeated"] }, "enemy_team_defeated")).toBe(true)
    expect(isCombatTrialMetricAvailableForEndReason({ eligibleEndReasons: ["enemy_team_defeated", "all_waves_cleared"] }, "all_waves_cleared")).toBe(true)
    expect(isCombatTrialMetricAvailableForEndReason({ eligibleEndReasons: ["enemy_team_defeated"] }, "action_budget_reached")).toBe(false)
    expect(isCombatTrialMetricAvailableForEndReason({ eligibleEndReasons: ["enemy_team_defeated"] }, "")).toBe(false)
    expect(isCombatTrialMetricAvailableForEndReason({ eligibleEndReasons: [] }, "")).toBe(true)
  })

  it("drives shared-position validation from the project lineup rules", () => {
    const selection: CombatTrialLineupSelection = {
      playerPosition: "vanguard",
      formatId: "standard",
      kitties: [{ kittyId: 1, position: "midguard" }, { kittyId: 2, position: "rearguard" }]
    }
    const sharedFormat = { lineupRules: { positionPolicy: "shared" } }
    const distinctFormat = { lineupRules: { positionPolicy: "distinct" } }
    expect(isCombatTrialLineupSelectionValid(selection, kitties, sharedFormat)).toBe(true)
    expect(activeCombatTrialKittySelections(selection)).toHaveLength(2)
    expect(isCombatTrialLineupSelectionValid({ ...selection, playerPosition: "midguard" }, kitties, sharedFormat)).toBe(true)
    expect(isCombatTrialLineupSelectionValid({ ...selection, playerPosition: "midguard" }, kitties, distinctFormat)).toBe(false)
    expect(isCombatTrialLineupSelectionValid({ ...selection, kitties: [{ kittyId: 1, position: "rearguard" }] }, kitties, sharedFormat)).toBe(false)
    expect(isCombatTrialLineupSelectionValid({ ...selection, kitties: [{ kittyId: 99, position: "midguard" }] }, kitties, sharedFormat)).toBe(false)
  })

  it("keeps shared-position and critical-resistance rules localized in every supported language", () => {
    for (const locale of [combatTrialEnUS, combatTrialZhCN, combatTrialZhTW, combatTrialJa, combatTrialKo, combatTrialRu]) {
      expect(locale.lineupHintShared).toBeTruthy()
      expect(locale.lineupHintDistinct).toBeTruthy()
      expect(locale.rulePositionShared).toBeTruthy()
      expect(locale.rulePositionDistinct).toBeTruthy()
      expect(locale.criticalDamageResist).toBeTruthy()
      expect(locale.formats.standard.desc).toContain("50")
      expect(locale.formats.gauntlet.name).toBeTruthy()
      expect(locale.formats.gauntlet.desc).toContain("N20")
      expect(locale.formats.recovery.name).toBeTruthy()
      expect(locale.metrics.wavesCleared.name).toBeTruthy()
      expect(locale.metrics.actionsToVictory.name).toBeTruthy()
      expect(locale.metrics.effectiveHealing.name).toBeTruthy()
      expect(locale.objectives.woundedPatient).toBeTruthy()
      expect(locale.objectives.recoveryClockVanguard).toBeTruthy()
      expect(locale.objectives.recoveryClockMidguard).toBeTruthy()
      expect(locale.objectives.recoveryClockRearguard).toBeTruthy()
      expect(locale.endReason.allWavesCleared).toBeTruthy()
      expect(locale.waveProgress).toContain("{current}")
      expect(locale.waveProgress).toContain("{count}")
      expect(locale.waveNavigator).toBeTruthy()
      expect(locale.previousWave).toBeTruthy()
      expect(locale.nextWave).toBeTruthy()
    }
  })

  it("recognizes the third battle stream and parses its compact snapshot", () => {
    expect(battlePayloadStreamKind({ k: "combat_trial", m: 9 })).toBe("combat_trial")
    const state = parseRunStateLoose(9, {
      w: 1,
      q: 3,
      p: [],
      e: [{ u: "combat_trial:dummy:1", t: "trial_dummy", s: 1, p: 0, h: 1_000_000, x: 1_000_000, m: 0, y: 0, a: true }],
      ctf: {
        id: "standard",
        nameKey: "game.combatTrial.formats.standard.name",
        icon: "lucide:target",
        actionBudget: 20,
        metrics: [{ id: "damage_total" }],
        encounter: { kind: "unit_roster", units: [{ id: "front_dummy", kind: "training_target", healthMode: "immutable_full" }] }
      },
      ct: { actionCount: 8, actionBudget: 20, metrics: { damage_total: 456 } }
    })
    expect(state.combatTrialFormat?.id).toBe("standard")
    expect(state.combatTrial?.actionCount).toBe(8)
    expect(state.combatTrial?.metrics.damage_total).toBe(456)
    expect(state.combatTrialFormat?.encounter?.units?.[0]?.healthMode).toBe("immutable_full")
    expect(state.enemies[0]?.unitType).toBe("trial_dummy")
  })

  it("keeps wave rosters and generic progress fields through replay parsing", () => {
    const state = parseRunStateLoose(10, {
      w: 2,
      q: 7,
      p: [],
      e: [],
      ctf: {
        id: "gauntlet",
        nameKey: "game.combatTrial.formats.gauntlet.name",
        icon: "lucide:repeat-2",
        actionBudget: 400,
        metrics: [{ id: "waves_cleared" }, { id: "actions_to_victory", rankDirection: "asc" }],
        encounter: {
          kind: "wave_roster",
          waves: [{ id: "first", nameKey: "game.combatTrial.formats.gauntlet.waves.meadowPatrol", units: [{ id: "slime", kind: "authored_enemy", healthMode: "normal" }] }]
        }
      },
      ct: {
        actionCount: 9,
        actionBudget: 400,
        encounterKind: "wave_roster",
        currentWave: 2,
        waveCount: 5,
        wavesCleared: 1,
        objectiveUnitsDefeated: 3,
        metrics: { waves_cleared: 1, actions_to_victory: 0 }
      }
    })
    expect(state.combatTrialFormat?.encounter?.waves?.[0]?.units[0]?.id).toBe("slime")
    expect(state.combatTrial?.currentWave).toBe(2)
    expect(state.combatTrial?.waveCount).toBe(5)
    expect(state.combatTrial?.objectiveUnitsDefeated).toBe(3)

    const cloned = cloneArenaBattleState(state)
    expect(cloned.combatTrialFormat?.encounter?.waves?.[0]).not.toBe(state.combatTrialFormat?.encounter?.waves?.[0])
    const advanced = applyArenaReplayEventBatch(cloned, [{
      seq: 8,
      ts: 100,
      type: "battle.wave.spawn",
      data: { combatTrial: { actionCount: 12, actionBudget: 400, currentWave: 3, waveCount: 5, wavesCleared: 2, objectiveUnitsDefeated: 6, metrics: { waves_cleared: 2, actions_to_victory: 0 } } }
    }])
    expect(advanced.combatTrial?.currentWave).toBe(3)
    expect(advanced.combatTrial?.wavesCleared).toBe(2)
  })

  it("keeps a combat trial replay as a typed chat attachment", () => {
    const store = useChatStore()
    store.applyConfig({ channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }] })
    store.appendIncomingMessage({
      id: 8,
      channel: "world",
      userId: 2,
      username: "B",
      content: "",
      ts: 100,
      attachment: {
        type: "combat_trial_replay",
        battleKind: "combat_trial",
        token: "t".repeat(64),
        runId: 42,
        createdBy: 2,
        createdByName: "B",
        outcome: "action_budget_reached",
        metrics: { damage_total: 900 }
      }
    })
    const attachment = store.messagesByChannel.world[0]?.attachment
    expect(attachment?.type).toBe("combat_trial_replay")
    if (attachment?.type === "combat_trial_replay") {
      expect(attachment.runId).toBe(42)
      expect(attachment.metrics.damage_total).toBe(900)
    }
  })
})
