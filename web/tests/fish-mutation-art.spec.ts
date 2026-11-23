import { afterEach, describe, expect, it } from "vitest"
import { createApp, h, nextTick, reactive, type App } from "vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import type { FishingMutationDefinition } from "@services/fishing"

let app: App | undefined
let host: HTMLElement

afterEach(() => {
  app?.unmount()
  app = undefined
  host.remove()
})

function mount(props: Record<string, unknown>) {
  host = document.createElement("div")
  document.body.append(host)
  app = createApp({ render: () => h(FishMutationArt, { src: "/fish.png", ...props }) })
  app.mount(host)
}

describe("fish mutation thumbnail rendering", () => {
  it("does not allocate invisible effect layers for ordinary fish", () => {
    mount({ fish: {} })
    expect(host.querySelector(".fish-mutation-art")!.children).toHaveLength(1)
    expect(host.querySelector("img")!.getAttribute("src")).toBe("/fish.png")
    expect(host.querySelector(".is-static")).toBeNull()
  })

  it("renders only the effect selected by the DSL rather than the mutation id", () => {
    mount({
      fish: { mutations: ["aurora"] },
      mutations: [{ id: "aurora", visualEffect: "rainbow" } as FishingMutationDefinition]
    })
    expect(host.querySelector(".fish-mutation-art__rainbow")).not.toBeNull()
    expect(host.querySelector(".fish-mutation-art__prism, .fish-mutation-art__glints")).toBeNull()
    expect(host.querySelector(".fish-mutation-art")!.getAttribute("data-fish-mutations")).toBe("aurora")
  })

  it("preserves all mutation markings in static mode and updates when props change", async () => {
    const props = reactive({
      fish: { mutations: ["prismatic", "rainbow", "shimmering"] },
      animated: false
    })
    mount(props)
    expect(host.querySelector(".is-static.has-stacked-fish-mutations")).not.toBeNull()
    expect(host.querySelectorAll(".fish-mutation-art__glints i")).toHaveLength(5)
    expect(host.querySelector(".fish-mutation-art__prism")).not.toBeNull()
    expect(host.querySelector(".fish-mutation-art__rainbow")).not.toBeNull()
    props.animated = true
    props.fish.mutations = ["prismatic"]
    await nextTick()
    expect(host.querySelector(".is-static, .has-stacked-fish-mutations")).toBeNull()
    expect(host.querySelector(".fish-mutation-art__prism")).not.toBeNull()
    expect(host.querySelector(".fish-mutation-art__rainbow, .fish-mutation-art__glints")).toBeNull()
  })
})
