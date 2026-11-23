import { describe, expect, it } from "vitest"
import { buildInsufficientActionCosts } from "@/composables/game/action/action-cost-availability"

describe("action-cost-availability", () => {
  it("keeps the cost insufficient when no binding is counted", () => {
    const rows = buildInsufficientActionCosts(
      { copper_dagger: 1 },
      1,
      (itemId) => {
        if (itemId === "copper_dagger") return 0
        return 0
      }
    )
    expect(rows).toEqual([{ itemId: "copper_dagger", required: 1, owned: 0 }])
  })

  it("counts the bound enhanced item toward the required amount", () => {
    const rows = buildInsufficientActionCosts(
      { copper_dagger: 1 },
      1,
      (itemId) => {
        if (itemId === "copper_dagger") return 1
        return 0
      }
    )
    expect(rows).toEqual([])
  })

  it("scales the required amount by execution count", () => {
    const rows = buildInsufficientActionCosts(
      { copper_dagger: 2 },
      2,
      (itemId) => {
        if (itemId === "copper_dagger") return 3
        return 0
      }
    )
    expect(rows).toEqual([{ itemId: "copper_dagger", required: 4, owned: 3 }])
  })
})
