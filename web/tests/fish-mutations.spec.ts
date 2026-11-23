import { describe, expect, it } from "vitest"
import {
  fishMutationClassNames,
  fishMutationIds,
  resolveFishMutations
} from "../src/services/fish-mutations"
import type { FishingMutationDefinition } from "../src/services/fishing"

const definitions: FishingMutationDefinition[] = [
  {
    id: "prismatic",
    nameKey: "fish.prismatic.name",
    descriptionKey: "fish.prismatic.desc",
    visualEffect: "prismatic",
    badgeIcon: "lucide:gem",
    appearanceChance: 0.01,
    inheritanceChance: 0.1,
    lifespanMultiplier: 1.8,
    saleValueMultiplier: 2,
    alchemy: { primaryValueMultiplier: 1.5, bonusAttributes: [] }
  },
  {
    id: "aurora",
    nameKey: "fish.aurora.name",
    descriptionKey: "fish.aurora.desc",
    visualEffect: "rainbow",
    badgeIcon: "lucide:rainbow",
    appearanceChance: 0.01,
    inheritanceChance: 0.1,
    lifespanMultiplier: 1.4,
    saleValueMultiplier: 2.5,
    alchemy: { primaryValueMultiplier: 1.2, bonusAttributes: [] }
  }
]

describe("fish mutations", () => {
  it("normalizes and deduplicates mutations stored in genes", () => {
    expect(fishMutationIds({ genes: { mutations: [" Prismatic ", "prismatic", "RAINBOW"] } })).toEqual([
      "prismatic",
      "rainbow"
    ])
  })

  it("supports the public projection and resolves DSL presentation", () => {
    const fish = { mutations: ["aurora", "unknown effect"] }
    expect(resolveFishMutations(fish, definitions)).toEqual([
      expect.objectContaining({ id: "aurora", visualEffect: "rainbow", badgeIcon: "lucide:rainbow", nameKey: "fish.aurora.name" }),
      expect.objectContaining({ id: "unknown-effect", visualEffect: "unknown-effect", badgeIcon: "lucide:sparkles" })
    ])
    expect(fishMutationClassNames(fish, definitions)).toEqual([
      "has-fish-mutation",
      "has-stacked-fish-mutations",
      "fish-mutation--rainbow",
      "fish-mutation--unknown-effect"
    ])
  })
})
