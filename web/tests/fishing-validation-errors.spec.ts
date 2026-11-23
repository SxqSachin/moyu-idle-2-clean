import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { describe, expect, it } from "vitest"
import { createI18n } from "vue-i18n"
import { APIClientError } from "../src/services/api"
import { resolveErrorI18nKey, resolveErrorMessage } from "../src/services/error-messages"
import { fishingValidationMessages } from "../src/services/fishing-validation-errors"
import zhCN from "../src/locales/game/zh-CN"
import enUS from "../src/locales/game/en-US"
import zhTW from "../src/locales/game/zh-TW"
import ja from "../src/locales/game/ja"
import ko from "../src/locales/game/ko"
import ru from "../src/locales/game/ru"

describe("fishing validation feedback", () => {
  const i18n = createI18n({ legacy: false, locale: "zh-CN", messages: { "zh-CN": zhCN } })

  it.each([
    ["WS_FISHING_SETTINGS_SUPPLY_REQUIRED_LEADER", "出发至少需要携带 1 件前导线"],
    ["WS_FISHING_SETTINGS_SUPPLY_UNEXPECTED_HOOK", "当前钓法不使用鱼钩"],
    ["WS_FISHING_SETTINGS_COMPONENT_INCOMPATIBLE_ROD", "钓组中的鱼竿与槽位或当前钓法不匹配"],
    ["FISHING_SETTINGS_COMPONENT_REQUIRED_FLOAT_OR_SINKER", "钓组尚未配置浮漂或铅坠"],
    ["WS_FISHING_SETTINGS_BOTTOM_LAYER_MISMATCH", "底钓只能选择“自动”或“近底”水层"],
    ["WS_FISHING_SETTINGS_STANDARD_MISMATCH", "这套钓组不符合本期标准组的指定渔具要求"]
  ])("localizes %s and preserves the diagnostic code", (code, text) => {
    for (const error of [{ code }, new APIClientError("server message", { code, status: 400 }), new Error(`${code}: server message`)]) {
      const message = resolveErrorMessage(i18n.global.t, error, "game.fishing.tripStartFailed")
      expect(message).toContain(text)
      expect(message).toContain(`[${code}]`)
      expect(message).not.toContain("{field}")
    }
  })

  it.each(["WS_FISHING_INVALID", "FISHING_INVALID"])("gives feedback instructions for legacy %s", (code) => {
    const message = resolveErrorMessage(i18n.global.t, { code }, "game.fishing.tripStartFailed")
    expect(message).toContain("截图反馈")
    expect(message).toContain(code)
  })

  it("has a localized message and field label for every public code in every locale", () => {
    for (const messages of [zhCN, enUS, zhTW, ja, ko, ru]) {
      const locale = createI18n({ legacy: false, locale: "test", messages: { test: messages } })
      for (const [code, details] of Object.entries(fishingValidationMessages)) {
        expect(resolveErrorI18nKey({ code }, "fallback")).toBe(details.key)
        expect(locale.global.te(details.key), details.key).toBe(true)
        if (details.fieldKey) expect(locale.global.te(details.fieldKey), details.fieldKey).toBe(true)
      }
    }
  })

  it("covers every backend validation reason", () => {
    const source = readFileSync(resolve(process.cwd(), "../server/internal/store/store_fishing_validation.go"), "utf8")
    const reasons = [...source.matchAll(/"([A-Z_]+)":\s*"/g)].map((match) => match[1])
    expect(reasons.length).toBeGreaterThan(20)
    const frontendKeys = new Set(Object.values(fishingValidationMessages).map((message) => message.key))
    for (const reason of reasons) expect(frontendKeys.has(`game.fishing.validation_${reason}`), reason).toBe(true)
  })

  it("localizes equipment requirements and attribute-loading feedback in every locale", () => {
    for (const messages of [zhCN, enUS, zhTW, ja, ko, ru]) {
      const locale = createI18n({ legacy: false, locale: "test", messages: { test: messages } })
      for (const suffix of ["tripEquipmentRequirement", "tripEquipmentMismatchHint", "tripEquipmentLoading", "tripEquipmentLoadFailed"]) {
        const key = `game.fishing.${suffix}`
        expect(locale.global.te(key), key).toBe(true)
        const message = locale.global.t(key, { name: "Test Angler", item: "Test Rod", attribute: "Fishing", required: 8, current: 1 })
        expect(message).toContain("Test Angler")
        expect(message).not.toContain("{")
      }
    }
  })
})
