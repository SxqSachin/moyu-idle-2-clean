import { describe, expect, it } from "vitest"
import { getGameDataTable } from "@services/game-data-runtime"
import { fishingPondZone } from "@services/fishing"

type Species = { id: string; rarity: string }
type Building = { id: string; levels: Array<{ level: number; bonuses: Record<string, number> }> }

describe("fish pond catalog contracts", () => {
  it("assigns every fish species a game rarity", () => {
    const allowed = new Set(["common", "uncommon", "rare", "epic", "legendary", "mythic"])
    const species = getGameDataTable<Species>("fishing_species")
    expect(species.length).toBeGreaterThan(0)
    expect(species.filter((row) => !allowed.has(row.rarity)).map((row) => row.id)).toEqual([])
  })

  it("keeps level one at twelve and adds three slots to later pond levels", () => {
    const pond = getGameDataTable<Building>("buildings").find((row) => row.id === "fish_pond")
    expect(pond?.levels.map((row) => row.bonuses.fishCapacity)).toEqual([12, 21, 27, 33, 39])
    expect(pond?.levels.map((row) => row.bonuses.nurseryCapacity)).toEqual([12, 12, 12, 12, 12])
  })

  it("treats legacy pond fish as main-pond residents", () => {
    expect(fishingPondZone({})).toBe("main")
    expect(fishingPondZone({ pondZone: "nursery" })).toBe("nursery")
  })
})
