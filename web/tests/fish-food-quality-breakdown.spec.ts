import { afterEach, describe, expect, it } from "vitest"
import { createApp, h, type App } from "vue"
import { createI18n } from "vue-i18n"
import FishFoodQualityBreakdown from "@components/game/item/FishFoodQualityBreakdown.vue"
import { fishCuisineZhCN } from "@/locales/game/fish-cuisine"

let app: App | undefined
let host: HTMLElement | undefined
afterEach(() => { app?.unmount(); host?.remove() })

describe("fish food quality detail content", () => {
  it.each([false, true])("keeps scores and parameter weights without the removed rule paragraphs (compact=%s)", (compact) => {
    host = document.createElement("div")
    document.body.append(host)
    app = createApp({ render: () => h(FishFoodQualityBreakdown, {
      compact,
      quality: { lengthScore: 0.49, weightScore: 0.31, traitAverage: -0.06, parameterMultipliers: { extraMaterialQuantity: 0.999 } },
      scales: { extraMaterialQuantity: { length: 0.15, weight: 0.45, traits: 0.1, min: 10, max: 80 } }
    }) })
    app.use(createI18n({ legacy: false, locale: "zh-CN", messages: { "zh-CN": { game: { fishCuisine: fishCuisineZhCN } } } }))
    app.mount(host)
    expect(host.textContent).toContain("长度分位 49%")
    expect(host.textContent).toContain("同长度体重分位 31%")
    expect(host.textContent).toContain("特质均值 -0.06")
    expect(host.textContent).toContain("×0.9990")
    expect(host.textContent).toContain("长度 15% · 体重 45% · 特质 10%")
    expect(host.textContent).not.toContain("闪光")
    expect(host.textContent).not.toContain("15 分钟")
    expect(host.querySelector(".fish-food-breakdown--compact") !== null).toBe(compact)
  })
})
