import { describe, expect, it } from "vitest"
import { createI18n } from "vue-i18n"
import enUS from "../src/locales/game-data/en-US"
import ja from "../src/locales/game-data/ja"
import ko from "../src/locales/game-data/ko"
import ru from "../src/locales/game-data/ru"
import zhCN from "../src/locales/game-data/zh-CN"
import zhTW from "../src/locales/game-data/zh-TW"
import zhCNGame from "../src/locales/game/zh-CN"
import {
  evaluateAmountFormulaPreview,
  localizeAmountFormulaText,
  localizeBasicAttackFormulaText,
  localizeEffectAttr,
  localizeTargetCountMaxText,
  localizeTargetCountText
} from "../src/services/effect-formula-display"

const legacyKey = "magicial_flat_defense_penetration"
const aliasKey = "magical_flat_defense_penetration"

describe("effect formula attribute localization", () => {
  it.each([
    ["zh-CN", zhCN],
    ["zh-TW", zhTW],
    ["en-US", enUS],
    ["ja", ja],
    ["ko", ko],
    ["ru", ru]
  ] as const)("localizes both spellings using the real %s catalog", (locale, messages) => {
    const { t } = createI18n({
      legacy: false,
      locale,
      fallbackLocale: false,
      missingWarn: false,
      messages: { [locale]: messages }
    }).global
    const expected = messages.attr[legacyKey].name

    expect(expected).not.toBe(legacyKey)
    expect(expected).not.toBe("")
    expect(localizeEffectAttr(legacyKey, t)).toBe(expected)
    expect(localizeEffectAttr(aliasKey, t)).toBe(expected)
  })

  it("renders mixed formula spellings without changing terms or preview values", () => {
    const { t } = createI18n({
      legacy: false,
      locale: "zh-CN",
      missingWarn: false,
      messages: { "zh-CN": { ...zhCNGame, ...zhCN } }
    }).global
    const formula = Object.freeze({
      baseAdd: 0,
      terms: [
        Object.freeze({ attrKey: "dexterity", coefficient: 0.25 }),
        Object.freeze({ attrKey: legacyKey, coefficient: 0.25 }),
        Object.freeze({ attrKey: aliasKey, coefficient: 0.25 })
      ]
    })
    const attributes = { dexterity: 100, [legacyKey]: 40, [aliasKey]: 80 }

    expect(localizeAmountFormulaText(formula.baseAdd, formula.terms, t))
      .toBe("0.25×敏捷 + 0.25×魔法伤害穿透 + 0.25×魔法伤害穿透")
    expect(localizeBasicAttackFormulaText(formula.terms, null, t))
      .not.toMatch(/magicial|magical|attr\./)
    expect(evaluateAmountFormulaPreview(formula, attributes)).toBe(55)
    expect(formula.terms.map((term) => term.attrKey)).toEqual(["dexterity", legacyKey, aliasKey])

    const targeting = { base: 4, formula: [{ attrKey: "dexterity", coefficient: 0.01 }], max: 6 }
    expect(localizeTargetCountText(targeting, t)).toBe("4 + 敏捷×0.01（向下取整）")
    expect(localizeTargetCountMaxText(targeting, t)).toBe("6")
  })

  it("preserves unknown attribute fallback", () => {
    expect(localizeEffectAttr("unknown_stat", (key) => key)).toBe("unknown_stat")
  })
})
