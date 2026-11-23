import { describe, expect, it } from "vitest"
import { buildBaseAttrImpactMap, projectDerivedFinalStats } from "../src/services/attribute-derived-impact"

describe("attribute derived impact parser", () => {
  it("parses spaced expressions and multi-term formulas", () => {
    const result = buildBaseAttrImpactMap({
      physics_damage: "strength * 0.6 + dexterity * 0.4",
      attack_speed: "dexterity*1"
    })

    expect(result).toEqual({
      strength: [{ targetAttrKey: "physics_damage", deltaPerPoint: 0.6 }],
      dexterity: [
        { targetAttrKey: "physics_damage", deltaPerPoint: 0.4 },
        { targetAttrKey: "attack_speed", deltaPerPoint: 1 }
      ]
    })
  })

  it("skips invalid terms without throwing", () => {
    const result = buildBaseAttrImpactMap({
      hp: "stamina*2 + invalid + agility*abc",
      mp: "intelligence*1"
    })

    expect(result).toEqual({
      stamina: [{ targetAttrKey: "hp", deltaPerPoint: 2 }],
      intelligence: [{ targetAttrKey: "mp", deltaPerPoint: 1 }]
    })
  })

  it("returns empty map when formula snapshot is absent", () => {
    expect(buildBaseAttrImpactMap(undefined)).toEqual({})
    expect(buildBaseAttrImpactMap(null)).toEqual({})
  })

  it("projects derived stats by overwriting existing attributes", () => {
    const result = projectDerivedFinalStats(
      {
        hp: 100,
        stamina: 10,
        strength: 2,
        level: 8
      },
      {
        hp: "stamina*9 + strength*1.2"
      }
    )
    expect(result).toEqual({
      hp: 192.4,
      stamina: 10,
      strength: 2,
      level: 8
    })
  })

  it("projects formulas that create new target attributes", () => {
    const result = projectDerivedFinalStats(
      {
        dexterity: 20
      },
      {
        speed: "dexterity*0.45"
      }
    )
    expect(result).toEqual({
      dexterity: 20,
      speed: 9
    })
  })

  it("keeps base stats when formula snapshot is absent", () => {
    const stats = { hp: 33, mp: 11 }
    expect(projectDerivedFinalStats(stats, undefined)).toEqual(stats)
    expect(projectDerivedFinalStats(stats, null)).toEqual(stats)
  })

  it("ignores invalid formula terms without mutating other stats", () => {
    const result = projectDerivedFinalStats(
      {
        hp: 100,
        stamina: 8
      },
      {
        hp: "stamina*9 + invalid + agility*abc",
        mp: "  "
      }
    )
    expect(result).toEqual({
      hp: 172,
      stamina: 8
    })
  })
})
