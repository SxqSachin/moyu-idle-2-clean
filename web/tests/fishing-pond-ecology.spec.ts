import { describe, expect, it } from "vitest"
import {
  canFishingPondFishPredate,
  fishingPondPredationRisks,
  mergeFishingPondEcosystem
} from "../src/services/fishing-pond-ecology"
import type { FishingPondFish, FishingPondPredation } from "../src/services/fishing"

function pondFish(instanceId: number, lengthCm: number, options: {
  predation?: FishingPondPredation
  satiety?: number
  lifeState?: "alive" | "dead"
} = {}): FishingPondFish {
  return {
    instanceId,
    fish: {
      speciesId: `fish-${instanceId}`,
      ageDays: 100,
      sex: "male",
      lengthCm,
      weightKg: 1,
      condition: 1,
      health: 1,
      waterId: "test-water",
      spotId: "test-spot",
      caughtAt: "2026-09-02T00:00:00Z",
      rulesetVersion: "fishing-v1",
      lifeState: options.lifeState || "alive",
      satiety: options.satiety ?? 100
    },
    dietClass: "protein",
    secondaryFeedClasses: [],
    maturityAgeDays: 30,
    expectedLifespanDays: 1000,
    mature: true,
    predation: options.predation
  }
}

const predatorRule: FishingPondPredation = {
  maxPreyLengthRatio: 0.4,
  huntBelowSatiety: 60,
  dailyHuntChance: 0.5
}

describe("fishing pond ecology", () => {
  it("uses the predator length rule and excludes dead fish", () => {
    const predator = pondFish(1, 50, { predation: predatorRule, satiety: 30 })
    expect(canFishingPondFishPredate(predator, pondFish(2, 20))).toBe(true)
    expect(canFishingPondFishPredate(predator, pondFish(3, 20.1))).toBe(false)
    expect(canFishingPondFishPredate(predator, pondFish(4, 10, { lifeState: "dead" }))).toBe(false)
  })

  it("reports potential relationships separately from active hunger", () => {
    const fullPredator = pondFish(1, 50, { predation: predatorRule, satiety: 60 })
    const hungryPredator = pondFish(2, 60, { predation: predatorRule, satiety: 59 })
    const prey = pondFish(3, 18)
    const risks = fishingPondPredationRisks([fullPredator, hungryPredator, prey])

    expect(risks.map((risk) => [risk.predator.instanceId, risk.prey.instanceId, risk.active])).toEqual([
      [1, 3, false],
      [2, 3, true]
    ])
  })

  it("merges selected fish without duplicating an existing instance", () => {
    const original = pondFish(1, 50, { predation: predatorRule })
    const updated = pondFish(1, 55, { predation: predatorRule })
    const prey = pondFish(2, 10)

    expect(mergeFishingPondEcosystem([original], [updated, prey])).toEqual([updated, prey])
  })
})
