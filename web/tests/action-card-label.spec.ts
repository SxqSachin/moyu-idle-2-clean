import { describe, expect, it, vi } from "vitest"
import { createApp, h } from "vue"
import ActionCard from "../src/components/game/action/ActionCard.vue"

vi.mock("vue-i18n", () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock("@components/game/icon/TypeIcon.vue", () => ({
  default: { template: "<span />" }
}))

describe("action card label", () => {
  it("uses two-line labels while preserving custom label classes and selection", () => {
    const label = "Extraordinarily long cooking action name"
    const onSelect = vi.fn()
    const container = document.createElement("div")
    const app = createApp({
      render: () => h(ActionCard, {
        actionId: "test-action",
        label,
        labelClass: "custom-label",
        requirementCount: 0,
        outputCount: 1,
        onSelect
      })
    })
    app.mount(container)
    try {
      const name = container.querySelector(".item-card-name")
      expect(name?.textContent).toBe(label)
      expect(name?.classList.contains("item-card-name--two-lines")).toBe(true)
      expect(name?.classList.contains("custom-label")).toBe(true)
      container.querySelector<HTMLElement>(".action-card")?.click()
      expect(onSelect).toHaveBeenCalledOnce()
    } finally {
      app.unmount()
    }
  })
})
