import { describe, expect, it } from "vitest"
import { expandSnapshotUnits } from "@/composables/game/combat/useCombatPage"

describe("combat snapshot skill levels", () => {
  it("hydrates skill level from compact snapshot level map", () => {
    const units = expandSnapshotUnits([
      {
        u: "player:1",
        h: 100,
        m: 50,
        x: 120,
        y: 60,
        i: ["fireball", "heal"],
        d: { fireball: 2 },
        g: { fireball: 4, heal: 2 }
      }
    ], "player")

    expect(units).toHaveLength(1)
    expect(units[0].skills).toEqual([
      { id: "fireball", level: 4, cooldownRemain: 2 },
      { id: "heal", level: 2, cooldownRemain: 0 }
    ])
  })

  it("falls back to level 1 when snapshot has no level map", () => {
    const units = expandSnapshotUnits([
      {
        u: "player:2",
        h: 80,
        m: 30,
        x: 100,
        y: 40,
        i: ["slash"]
      }
    ], "player")

    expect(units).toHaveLength(1)
    expect(units[0].skills).toEqual([
      { id: "slash", level: 1, cooldownRemain: 0 }
    ])
  })
})
