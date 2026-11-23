import { describe, expect, it } from "vitest"
import { ref } from "vue"
import { useAttributeRows } from "../src/composables/game/attributes/useAttributeRows"

describe("useAttributeRows", () => {
  it("shows per-point derived impacts for static base attributes", () => {
    const attributes = ref({
      dexterity: {
        value: 10,
        unit: "number",
        category: "battle",
        growthType: "static",
        order: 1,
      },
      speed: {
        value: 10,
        unit: "number",
        category: "battle",
        growthType: "static",
        order: 2,
      },
      crit_chance: {
        value: 0.025,
        unit: "percent",
        category: "battle",
        growthType: "static",
        order: 3,
      },
    })
    const attributeProgress = ref({})
    const formula = ref({
      derived: {
        speed: "dexterity*1",
        crit_chance: "dexterity*0.0025",
      },
    })
    const t = (key: string, params?: Record<string, string | number>) => {
      if (key === "game.attrPerPointImpactTitle") return "Per +1 impact:"
      if (key === "game.attrPerPointImpactLine") return `${params?.target} ${params?.value}`
      return key
    }

    const { attrRows } = useAttributeRows(attributes, attributeProgress, formula, t)
    const dexterity = attrRows.value.find((row) => row.key === "dexterity")

    expect(dexterity?.impactLines).toHaveLength(2)
    expect(dexterity?.compositionLines).toHaveLength(0)
    expect(dexterity?.breakdownLines).toContain("game.attr.speed +1")
    expect(dexterity?.breakdownLines).toContain("game.attr.crit_chance +0.25%")
  })

  it("uses attribute value for static rows even when progress is stale", () => {
    const attributes = ref({
      intelligence: {
        value: 110,
        unit: "number",
        category: "battle",
        growthType: "static",
        order: 1,
      },
    })
    const attributeProgress = ref({
      intelligence: {
        value: 10,
        level: 1,
        exp: 0,
        nextExp: 100,
      },
    })
    const formula = ref({})
    const t = (key: string) => key

    const { attrRows } = useAttributeRows(attributes, attributeProgress, formula, t)
    const intelligence = attrRows.value.find((row) => row.key === "intelligence")

    expect(intelligence?.rawValue).toBe(110)
    expect(intelligence?.value).toBe("110")
  })
})
