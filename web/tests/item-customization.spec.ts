import { describe, expect, it } from "vitest"
import {
  isValidItemCustomName,
  itemCustomLabelClass,
  itemCustomLabelClassForColor,
  itemCustomLabelColor,
  itemCustomName
} from "@services/item-customization"

describe("item customization", () => {
  it("normalizes a custom name and supported label color", () => {
    const customData = { name: "  小金  ", labelColor: " SKY " }
    expect(itemCustomName(customData)).toBe("小金")
    expect(itemCustomLabelColor(customData)).toBe("sky")
    expect(itemCustomLabelClass(customData)).toBe("item-custom-label item-custom-label--sky")
    expect(itemCustomLabelClassForColor("bad class")).toBe("item-custom-label item-custom-label--gold")
  })

  it("keeps the rename validator aligned with the server character contract", () => {
    for (const name of ["月影🌙", "hello✨", "剑🧙‍♂️", "护符👍🏽", "abc_123"]) {
      expect(isValidItemCustomName(name), name).toBe(true)
    }
    for (const name of ["", "hello world", "hello!", "<script>", "\u200d", "abc\u200d", "👨‍A"]) {
      expect(isValidItemCustomName(name), name).toBe(false)
    }
  })
})
