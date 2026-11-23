import { describe, expect, it } from "vitest"
import {
  compareFishSize,
  FISH_STANDARD_LENGTH_RANGE_RATIO,
  fishStandardMeasurements
} from "../src/services/fish-size-comparison"

const species = { minLengthCm: 10, maxLengthCm: 50, shapeFactor: 1.55 }

describe("fish size comparison", () => {
  it("uses the same 30% range anchor as the standard sale specimen", () => {
    expect(FISH_STANDARD_LENGTH_RANGE_RATIO).toBe(0.30)
    const standard = fishStandardMeasurements(species)
    expect(standard?.lengthCm).toBe(22)
    expect(standard?.weightKg).toBeCloseTo(0.165044, 6)
  })

  it("hides the marker at or below the species standard", () => {
    expect(compareFishSize("length", 21.9, species)).toBeNull()
    expect(compareFishSize("length", 22, species)).toBeNull()
    expect(compareFishSize("weight", 0.16, species)).toBeNull()
  })

  it("adds arrows as the excess grows", () => {
    expect(compareFishSize("length", 22.1, species)).toMatchObject({ arrowCount: 1, excessPercent: 1 })
    expect(compareFishSize("length", 26.4, species)).toMatchObject({ arrowCount: 2, excessPercent: 20 })
    expect(compareFishSize("length", 33, species)).toMatchObject({ arrowCount: 3, excessPercent: 50 })
  })

  it("compares weight against the standard weight and rejects invalid catalog data", () => {
    expect(compareFishSize("weight", 0.2, species)).toMatchObject({ arrowCount: 2, excessPercent: 21 })
    expect(compareFishSize("length", 30, { ...species, shapeFactor: 0 })).toBeNull()
    expect(compareFishSize("length", Number.NaN, species)).toBeNull()
  })
})
