import { beforeEach, describe, expect, it, vi } from "vitest"
import { createApp, defineComponent, h, nextTick } from "vue"
import ItemBrowserPanel, { type ItemBrowserItem } from "../src/components/game/item/ItemBrowserPanel.vue"
import {
  getItemBrowserSecondaryCategoryEnabled,
  resetItemBrowserSecondaryCategorySettingsForTest,
  setItemBrowserSecondaryCategoryEnabled
} from "../src/services/item-browser-secondary-category"
import { buildFishingTackleSecondarySections } from "../src/services/fishing-tackle-grouping"
import { getItemTypes, isItemFixedMarketListable, listItemDefinitions } from "../src/services/item-definition"
import { usesInstanceMarket } from "../src/services/instance-market"

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => {
      const labels: Record<string, string> = {
        "game.fishing.rod": "Fishing rod group",
        "game.fishing.reel": "Fishing reel group",
        "game.fishing.bait": "Fishing bait group"
      }
      return labels[key] || key
    },
    te: () => false
  })
}))

const passthrough = defineComponent({
  name: "Passthrough",
  inheritAttrs: false,
  setup(_, { slots }) {
    return () => h("div", slots.default?.())
  }
})

const dropdownStub = defineComponent({
  name: "DropdownStub",
  inheritAttrs: false,
  setup(_, { slots }) {
    return () => h("div", [slots.default?.(), slots.dropdown?.()])
  }
})

function registerPanelStubs(app: ReturnType<typeof createApp>) {
  for (const name of ["el-input", "el-tabs", "el-button", "el-icon", "el-switch", "el-divider"]) {
    app.component(name, passthrough)
  }
  app.component("el-tab-pane", defineComponent({
    props: ["name", "label"],
    setup(props) {
      return () => h("div", { "data-category": props.name }, props.label)
    }
  }))
  app.component("el-dropdown", dropdownStub)
  app.component("el-dropdown-menu", passthrough)
  app.component("el-dropdown-item", passthrough)
}

function mountPanel(items: ItemBrowserItem[], extraProps: Record<string, unknown> = {}) {
  const container = document.createElement("div")
  document.body.appendChild(container)
  const app = createApp({
    render: () =>
      h(ItemBrowserPanel, {
        items,
        emptyText: "empty",
        showSearch: false,
        ...extraProps
      })
  })
  registerPanelStubs(app)
  app.mount(container)
  return {
    container,
    unmount() {
      app.unmount()
      container.remove()
    }
  }
}

describe("item browser secondary category settings", () => {
  beforeEach(() => {
    localStorage.clear()
    resetItemBrowserSecondaryCategorySettingsForTest()
  })

  it("stores and restores per-category values from localStorage", () => {
    expect(getItemBrowserSecondaryCategoryEnabled("equipment")).toBe(false)
    expect(getItemBrowserSecondaryCategoryEnabled("skillbook")).toBe(false)
    expect(getItemBrowserSecondaryCategoryEnabled("fishing_gear")).toBe(true)

    setItemBrowserSecondaryCategoryEnabled("equipment", true)
    setItemBrowserSecondaryCategoryEnabled("skillbook", false)
    setItemBrowserSecondaryCategoryEnabled("fishing_gear", false)

    expect(getItemBrowserSecondaryCategoryEnabled("equipment")).toBe(true)
    expect(getItemBrowserSecondaryCategoryEnabled("skillbook")).toBe(false)
    expect(getItemBrowserSecondaryCategoryEnabled("fishing_gear")).toBe(false)
    expect(JSON.parse(localStorage.getItem("item-browser:secondary-category") || "{}")).toEqual({
      equipment: true,
      skillbook: false,
      fishing_gear: false
    })

    resetItemBrowserSecondaryCategorySettingsForTest()
    expect(getItemBrowserSecondaryCategoryEnabled("fishing_gear")).toBe(false)
  })

  it("falls back to category defaults for malformed cache", () => {
    localStorage.setItem("item-browser:secondary-category", "{not-json")

    expect(getItemBrowserSecondaryCategoryEnabled("equipment")).toBe(false)
    expect(getItemBrowserSecondaryCategoryEnabled("fishing_gear")).toBe(true)
  })

  it("enables fishing grouping when an older cache has no fishing setting", () => {
    localStorage.setItem("item-browser:secondary-category", JSON.stringify({ equipment: true }))

    expect(getItemBrowserSecondaryCategoryEnabled("equipment")).toBe(true)
    expect(getItemBrowserSecondaryCategoryEnabled("fishing_gear")).toBe(true)
  })
})

describe("ItemBrowserPanel secondary grouping", () => {
  beforeEach(() => {
    localStorage.clear()
    resetItemBrowserSecondaryCategorySettingsForTest()
  })

  it("derives a fish tab from the generated market catalog without a hardcoded category", async () => {
    const items = listItemDefinitions()
      .filter((item) => isItemFixedMarketListable(item.id) || usesInstanceMarket(item.id))
      .map((item) => ({ key: item.id, itemId: item.id, itemTypes: getItemTypes(item.id) }))
    const view = mountPanel(items)
    try {
      await nextTick()
      expect(view.container.querySelector('[data-category="fish"]')).not.toBeNull()
      expect(view.container.querySelector('[data-category="fishing_gear"]')).not.toBeNull()
      expect(items.find((item) => item.itemId === "fish_catch")?.itemTypes).toEqual(["fish"])
    } finally {
      view.unmount()
    }
  })

  it("renders flat equipment items when secondary grouping is disabled", async () => {
    const view = mountPanel([
      { key: "wood_sword", itemId: "wood_sword", itemTypes: ["equipment"] },
      { key: "wood_staff", itemId: "wood_staff", itemTypes: ["equipment"] }
    ])

    await nextTick()

    expect(view.container.textContent || "").not.toContain("right_hand")
    view.unmount()
  })

  it("renders slot and weapon-class sections when secondary grouping is enabled", async () => {
    setItemBrowserSecondaryCategoryEnabled("equipment", true)
    const view = mountPanel([
      { key: "wood_sword", itemId: "wood_sword", itemTypes: ["equipment"] },
      { key: "wood_staff", itemId: "wood_staff", itemTypes: ["equipment"] }
    ])

    await nextTick()

    const text = view.container.textContent || ""
    expect(text).toContain("right_hand")
    expect(text).toContain("sword")
    expect(text).toContain("staff")
    view.unmount()
  })

  it("renders grouped sections when override is enabled even if local setting is off", async () => {
    const view = mountPanel(
      [
        { key: "wood_sword", itemId: "wood_sword", itemTypes: ["equipment"] },
        { key: "wood_staff", itemId: "wood_staff", itemTypes: ["equipment"] }
      ],
      {
        secondaryCategoryOverrideEnabled: true,
        secondaryCategoryToggleVisible: false
      }
    )

    await nextTick()

    const text = view.container.textContent || ""
    expect(text).toContain("right_hand")
    expect(text).toContain("sword")
    expect(text).toContain("staff")
    view.unmount()
  })

  it("renders fishing tackle slot sections by default", async () => {
    const view = mountPanel([
      { key: "rod", itemId: "basic_fishing_rod", itemTypes: ["fishing_gear"] },
      { key: "reel", itemId: "basic_spinning_reel", itemTypes: ["fishing_gear"] },
      { key: "bait", itemId: "earthworm_bait", itemTypes: ["fishing_gear"] }
    ])

    await nextTick()

    const text = view.container.textContent || ""
    expect(text).toContain("Fishing rod group")
    expect(text).toContain("Fishing reel group")
    expect(text).toContain("Fishing bait group")
    view.unmount()
  })

  it("uses fishing tackle data as the secondary category authority", () => {
    const items = [
      { itemId: "earthworm_bait" },
      { itemId: "reed_float" },
      { itemId: "river_sinker" },
      { itemId: "basic_fishing_rod" },
      { itemId: "unknown_fishing_gear" }
    ]
    const sections = buildFishingTackleSecondarySections({
      items,
      getItemId: (item) => item.itemId,
      t: (key) => {
        const labels: Record<string, string> = {
          "game.fishing.float": "Fishing float group",
          "game.fishing.sinker": "Fishing sinker group"
        }
        return labels[key] || key
      }
    })

    expect(sections.map((section) => section.id)).toEqual(["rod", "float", "sinker", "bait", "other"])
    expect(sections.find((section) => section.id === "rod")?.items).toEqual([{ itemId: "basic_fishing_rod" }])
    expect(sections.find((section) => section.id === "float")?.label).toBe("Fishing float group")
    expect(sections.find((section) => section.id === "sinker")?.label).toBe("Fishing sinker group")
    expect(sections.find((section) => section.id === "other")?.items).toEqual([{ itemId: "unknown_fishing_gear" }])
  })
})
