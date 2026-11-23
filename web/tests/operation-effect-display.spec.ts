import { describe, expect, it } from "vitest"
import { createI18n } from "vue-i18n"
import zhCN from "@/locales/game/zh-CN"
import enUS from "@/locales/game/en-US"
import { getFishFoodEffectDefinition, listFishFoodRecipes } from "@services/fish-food"
import { operationEffectLifetimeText, operationEffectModifierLines } from "@services/operation-effect-display"

const t = createI18n({ legacy: false, locale: "zh-CN", messages: { "zh-CN": zhCN } }).global.t
const charges = { kind: "charges", charges: 12 } as const

describe("food lifetime operation wording", () => {
  it("names the exact point-gold operation instead of a generic eligible operation", () => {
    expect(operationEffectLifetimeText({ kind: "alchemy.transmute_gold_bonus", params: {}, lifetime: charges }, t))
      .toBe("接下来 12 次点金生效")
    expect(operationEffectLifetimeText({ kind: "alchemy.transmute_duration", params: {}, lifetime: charges }, t))
      .toBe("接下来 12 次点金生效")
    expect(operationEffectModifierLines("alchemy.transmute_gold_bonus", { bonusRate: .04 }, t))
      .toEqual(["点金额外金币 +4%"])
  })

  it.each([
    [1, "普通或罕见"], [2, "稀有或史诗"], [0, "史诗及以下"]
  ])("uses the frozen material tier %s", (materialTier, scope) => {
    expect(operationEffectLifetimeText({ kind: "alchemy.infusion_bulk_capacity_floor_tradeoff", params: { materialTier }, lifetime: charges }, t))
      .toBe(`接下来 12 次含${scope}材料的灌注生效`)
  })

  it("counts casts rather than catches and keeps duration eligibility at operation start", () => {
    expect(operationEffectLifetimeText({ kind: "fishing_cast.landing_bonus", params: {}, lifetime: charges }, t))
      .toBe("接下来 12 次钓鱼抛竿生效")
    expect(operationEffectLifetimeText({ kind: "alchemy.transmute_gold_bonus", params: {}, lifetime: { kind: "duration", durationSeconds: 600 } }, t))
      .toBe("食用后 10分钟 内开始的点金生效")
  })

  it("covers every current recipe in both locale families", () => {
    const recipes = listFishFoodRecipes()
    expect(recipes.length).toBeGreaterThanOrEqual(22)
    for (const [locale, messages] of [["zh-CN", zhCN], ["en-US", enUS]] as const) {
      const translate = createI18n({ legacy: false, locale, messages: { [locale]: messages } }).global.t
      for (const recipe of recipes) {
        const definition = getFishFoodEffectDefinition(recipe)!
        const text = operationEffectLifetimeText(definition, translate)
        expect(text, recipe.id).not.toBe("")
        expect(text).not.toMatch(/符合条件的操作|eligible operations|game\.fishCuisine|\{operation\}/)
      }
    }
  })

  it("does not invent an eligibility scope for an unknown business effect", () => {
    expect(operationEffectLifetimeText({ kind: "future.effect", params: {}, lifetime: charges }, t)).toBe("")
  })
})
