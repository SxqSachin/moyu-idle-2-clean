import { describe, expect, it } from "vitest"
import zhCN from "../src/locales/game/zh-CN"
import zhTW from "../src/locales/game/zh-TW"
import enUS from "../src/locales/game/en-US"
import ja from "../src/locales/game/ja"
import ko from "../src/locales/game/ko"
import ru from "../src/locales/game/ru"

describe("destructive recycle locale integration", () => {
  it.each([zhCN,zhTW,enUS,ja,ko,ru])("publishes all confirmation keys under game", (messages) => {
    const copy = messages.game.destructiveRecycle
    expect(Object.keys(copy).sort()).toEqual(Object.keys(enUS.game.destructiveRecycle).sort())
    expect(copy.acknowledge).toContain("{items}")
    expect(copy.acknowledge).toContain("{affixes}")
    expect(copy.acknowledge).toContain("{enhanced}")
    expect(copy.fee).toContain("{percent}")
    for (const value of Object.values(copy)) expect(value.length).toBeGreaterThan(0)
  })
})
