import { afterEach, describe, expect, it } from "vitest"
import { createApp, h, type App } from "vue"
import { createI18n } from "vue-i18n"
import FishFoodInstanceInfo from "@components/game/item/FishFoodInstanceInfo.vue"
import zhCN from "@/locales/game/zh-CN"
import { getFishFoodEffectDefinition, getFishFoodRecipeByActionId } from "@services/fish-food"

let app: App | undefined
let host: HTMLElement | undefined
afterEach(() => { app?.unmount(); host?.remove() })

function mount(baseSnapshot?: unknown, itemId = "mossfire_dual_fish_grill") {
  host = document.createElement("div")
  document.body.append(host)
  app = createApp({render: () => h(FishFoodInstanceInfo, {itemId,baseSnapshot})})
  app.use(createI18n({legacy:false,locale:"zh-CN",messages:{"zh-CN":zhCN}}))
  app.mount(host)
  return host
}

describe("food detail values", () => {
  it("shows template numbers and lifetime without a custom snapshot", () => {
    const el = mount()
    expect(el.textContent).toContain("+50")
    expect(el.textContent).toContain("12%")
    expect(el.textContent).toContain("接下来 1 次含普通或罕见材料的灌注生效")
    expect(el.textContent).toContain("食用时选择玩家、指定猫咪或全体")
    expect(el.querySelector("details")).toBeNull()
  })
  it("shows the specific point-gold scope in standard food details", () => {
    const el = mount(undefined, "sunhoney_gold_grill")
    expect(el.querySelector(".fish-food-instance-info__lifetime")?.textContent)
      .toContain("接下来 12 次点金生效")
    expect(el.textContent).not.toMatch(/首领战利品|向下取整|商店出售/)
  })
  it("shows timed food scope without implying settlement must finish before expiry", () => {
    const el = mount(undefined, "sunhoney_gold_soup")
    expect(el.querySelector(".fish-food-instance-info__lifetime")?.textContent)
      .toContain("食用后 10分钟 内开始的点金生效")
  })
  it("shows frozen custom numbers outside the collapsed ingredient details", () => {
    const def = getFishFoodEffectDefinition(getFishFoodRecipeByActionId("cook_mossfire_dual_fish_grill")!)!
    const el = mount({operationEffect:{schemaVersion:2,definitionId:def.id,rulesetVersion:def.rulesetVersion,params:{extraMaterialQuantity:127,positiveFloorReductionRate:0.08},lifetime:{kind:"charges",charges:2}},fishFood:{schemaVersion:1,quality:{multiplier:1.1,lengthScore:0.6,weightScore:0.7,traitAverage:0,mutationCount:1,parameterMultipliers:{extraMaterialQuantity:2.54}},ingredients:[{speciesId:"crucian_carp",lengthCm:18,weightKg:0.09,mutations:["prismatic"]}]}})
    expect(el.querySelector(".fish-food-instance-info__effects")?.textContent).toContain("+127")
    expect(el.querySelector(".fish-food-instance-info__effects")?.textContent).toContain("8%")
    expect(el.querySelector(".fish-food-instance-info__lifetime")?.textContent).toContain("2 次")
    const details = el.querySelector("details")!
    expect(details.open).toBe(false)
    expect(details.querySelector(".fish-food-instance-info__effects")).toBeNull()
  })
})
