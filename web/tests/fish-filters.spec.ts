import { describe, expect, it } from "vitest"
import { defaultFishFilters, filterAndSortFish, fishFilterRarities } from "@services/fish-filters"
import { getGameDataTable } from "@services/game-data-runtime"
import type { FishingSpecies } from "@services/fishing"

const species = getGameDataTable<FishingSpecies>("fishing_species")
const rows = [
  { id: 1, name: "小金", speciesId: species[0].id, health: .9, weightKg: 1, lengthCm: 20, genes: { mutations: ["prismatic", "shimmering"] } },
  { id: 2, name: "Little Rainbow", speciesId: species[1].id, health: .79, weightKg: 2, lengthCm: 30, mutations: ["rainbow"] },
  { id: 3, name: "Plain", speciesId: species[0].id, health: .8, weightKg: 3, lengthCm: 40 }
]
const context = {
  fish: (row: typeof rows[number]) => row, name: (row: typeof rows[number]) => row.name,
  speciesName: (id: string) => id, speciesById: new Map(species.map((row) => [row.id, row])),
  instanceId: (row: typeof rows[number]) => row.id, locale: "zh-CN"
}

describe("shared fish filters", () => {
  it("offers every catalog fish rarity independent of owned fish", () => {
    expect(fishFilterRarities()).toEqual(["common", "uncommon", "rare", "epic", "legendary", "mythic"])
  })
  it("matches each stacked mutation and legacy mutation data", () => {
    for (const mutation of ["prismatic", "shimmering"]) {
      expect(filterAndSortFish(rows, { ...defaultFishFilters(), mutation }, context).map((row) => row.id)).toEqual([1])
    }
    expect(filterAndSortFish(rows, { ...defaultFishFilters(), mutation: "rainbow" }, context).map((row) => row.id)).toEqual([2])
    expect(filterAndSortFish(rows, { ...defaultFishFilters(), mutation: "none" }, context).map((row) => row.id)).toEqual([3])
    expect(filterAndSortFish(rows, { ...defaultFishFilters(), mutation: "any" }, context)).toHaveLength(2)
  })
  it("combines name, species, rarity, health and mutation before sorting", () => {
    const filters = { ...defaultFishFilters(), search: " 小金 ", species: species[0].id, rarity: species[0].rarity, health: "healthy" as const, mutation: "shimmering" }
    expect(filterAndSortFish(rows, filters, context).map((row) => row.id)).toEqual([1])
    expect(filterAndSortFish(rows, { ...filters, health: "attention" }, context)).toEqual([])
    expect(filterAndSortFish(rows, { ...defaultFishFilters(), search: "rainBOW" }, context).map((row) => row.id)).toEqual([2])
    expect(filterAndSortFish(rows, { ...defaultFishFilters(), health: "healthy", sort: "weight", direction: "asc" }, context).map((row) => row.id)).toEqual([1, 3])
    expect(rows.map((row) => row.id)).toEqual([1, 2, 3])
  })
  it("preserves the server's default order and reverses it only on request", () => {
    expect(filterAndSortFish(rows, defaultFishFilters("recent"), context).map((row) => row.id)).toEqual([1, 2, 3])
    expect(filterAndSortFish(rows, { ...defaultFishFilters("recent"), direction: "asc" }, context).map((row) => row.id)).toEqual([3, 2, 1])
  })
})
