import { describe, expect, it } from "vitest"
import { operationEffectEligibility, operationEffectMatchesQuery } from "@services/operation-effect-context"
import { primaryWorkTypes, workMealDuration, workMealModifiers, workMealPlanSeconds } from "@services/work-meal"
import type { OperationEffectView } from "@services/operation-effects"

const at = "2026-09-05T00:00:00.000Z"
const meal = (overrides: Partial<OperationEffectView> = {}): OperationEffectView => ({
  grantId: 1, definitionId: "patient", rulesetVersion: "test", domain: "life_action", kind: "life_action.patient_harvest",
  nameKey: "game.fishCuisine.names.patient_dual_grill", descKey: "", targetKind: "player_and_kitties",
  lifetimeKind: "charges", validFrom: at, initialCharges: 2, remainingCharges: 2,
  selector: { mainTypes: [...primaryWorkTypes] }, params: { chance: .08, durationPenaltyRate: .04 }, ...overrides
})

describe("primary work meals", () => {
  it("only advertises the three work categories for players and assistants", () => {
    for (const actorKind of ["player", "kitty"] as const) for (const mainType of [...primaryWorkTypes, "cooking", "alchemy", "fishing"]) {
      const context = { domain: "life_action" as const, mainType, mainOutput: "wood", actorKind, actorId: 2 }
      const want = primaryWorkTypes.some(type => type === mainType)
      expect(operationEffectMatchesQuery(meal(), context)).toBe(want)
      expect(operationEffectEligibility(meal(), context).applicable).toBe(want)
      expect(operationEffectEligibility(meal(), { ...context, mainOutput: undefined }).applicable).toBe(false)
    }
  })
  it("keeps benefit and penalty inseparable and rejects invalid or overlapping data", () => {
    expect(workMealModifiers([meal()])).toEqual({ chance: .08, durationPenaltyRate: .04, durationReductionRate: 0, experienceBonusRate: 0 })
    for (const effects of [[meal(), meal()], [meal({ params: { chance: .10 } })], [meal({ params: { chance: .1101, durationPenaltyRate: .04 } })]]) {
      expect(workMealModifiers(effects)).toEqual({ chance: 0, durationPenaltyRate: 0, durationReductionRate: 0, experienceBonusRate: 0 })
    }
  })
  it("uses the duration floor before applying the cost and retains millisecond precision", () => {
    expect(workMealDuration(1, .04)).toBe(2.08)
    expect(workMealDuration(10, .04)).toBe(10.4)
    expect(workMealDuration(16.001, .04)).toBe(16.642)
    expect(workMealDuration(16.001, 0)).toBe(16.001)
    expect(workMealDuration(2,0,.08)).toBe(2)
    expect(workMealDuration(4,0,.02)).toBe(3.92)
  })
  it("estimates only covered starts, including the last start before real-time expiry", () => {
    const now = Date.parse(at)
    expect(workMealPlanSeconds(4, 10, [meal()], now)).toBe(40.8)
    for (const [seconds, want] of [[0, 40], [10.4, 40.4], [20, 40.8]]) {
      const soup = meal({ lifetimeKind: "duration", validUntil: new Date(now + seconds * 1000).toISOString() })
      expect(workMealPlanSeconds(4, 10, [soup], now)).toBe(want)
    }
    expect(workMealPlanSeconds(4, 10, [], now)).toBe(40)
    expect(workMealPlanSeconds(4,10,[meal({kind:'life_action.focused_harvest',params:{chance:.01,durationReductionRate:.03}})],now)).toBe(39.4)
  })
})
