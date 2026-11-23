import { describe, expect, it } from "vitest"
import zhCN from "@/locales/game/zh-CN"
import enUS from "@/locales/game/en-US"
import zhTW from "@/locales/game/zh-TW"
import ja from "@/locales/game/ja"
import ko from "@/locales/game/ko"
import ru from "@/locales/game/ru"

describe("instance market message namespace", () => {
  it("registers every locale under game, not at the document root", () => {
    for (const messages of [zhCN,enUS,zhTW,ja,ko,ru]) {
      expect(messages.game.instanceMarket.variants).toBeTruthy()
      expect(messages.game.instanceMarket.retry).toBeTruthy()
      expect(Object.keys(messages.game.instanceMarket).sort()).toEqual(Object.keys(zhCN.game.instanceMarket).sort())
    }
  })
})
