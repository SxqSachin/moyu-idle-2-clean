import { describe, expect, it, vi } from "vitest"
import { createApp, h, ref } from "vue"
import InstanceMarketFields from "../src/views/game/market/components/InstanceMarketFields.vue"

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => `${key}${params ? JSON.stringify(params) : ''}`,
    te: () => false,
    locale: ref("zh-CN")
  })
}))

describe("market effect presentation", () => {
  it("uses the shared preview bullets with lifetime first, rules and exact variant values", () => {
    const root = document.createElement("div")
    const app = createApp({ render: () => h(InstanceMarketFields, { fields: [{
      labelKey: "effect", format: "operation_effect", value: {
        kind: "alchemy.infusion_bulk_capacity_floor_tradeoff",
        params: { extraMaterialQuantity: 72, positiveFloorReductionRate: 0.116201, successRatePenalty: 0.017, materialTier: 1 },
        lifetime: { kind: "charges", charges: 2 }
      }
    }] }) })
    try {
      app.mount(root)
      const lines = [...root.querySelectorAll('.effect-summary li')].map(row => row.textContent)
      expect(lines.length).toBeGreaterThanOrEqual(4)
      expect(lines[0]).toContain('game.fishCuisine.effectCharges')
      expect(root.textContent).toContain('11.6201')
      expect(root.textContent).toContain('game.fishCuisine.successPenalty')
      expect(root.textContent).toContain('game.fishCuisine.basicLimit')
      expect(root.querySelectorAll('.effect-summary__dot')).toHaveLength(lines.length)
    } finally { app.unmount() }
  })
})
