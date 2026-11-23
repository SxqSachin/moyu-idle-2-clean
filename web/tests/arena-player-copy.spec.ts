import { describe, expect, it } from "vitest"
import enGame from "../src/locales/game/en-US"
import jaGame from "../src/locales/game/ja"
import koGame from "../src/locales/game/ko"
import ruGame from "../src/locales/game/ru"
import zhGame from "../src/locales/game/zh-CN"
import zhTwGame from "../src/locales/game/zh-TW"
import enTutorial from "../src/locales/tutorial/en-US"
import zhTutorial from "../src/locales/tutorial/zh-CN"

function collectStrings(value: unknown): string[] {
  if (typeof value === "string") return [value]
  if (!value || typeof value !== "object") return []
  return Object.values(value).flatMap(collectStrings)
}

describe("arena player-facing copy", () => {
  it("describes asynchronous matches without exposing service internals", () => {
    expect(zhGame.game.arena.challengeQueued).toBe("比赛已安排，稍后可查看结果")
    expect(enGame.game.arena.challengeQueued).toBe("Match scheduled. Check back soon for the result.")

    const localeCopy = [
      collectStrings(zhGame.game.arena).join(" "),
      collectStrings(zhTwGame.game.arena).join(" "),
      collectStrings(enGame.game.arena).join(" "),
      collectStrings(jaGame.game.arena).join(" "),
      collectStrings(koGame.game.arena).join(" "),
      collectStrings(ruGame.game.arena).join(" "),
    ].join(" ")

    expect(localeCopy).not.toMatch(/战斗服务|戰鬥服務|battle service|戦闘サービス|전투 서비스|боевому сервису/i)
    expect(localeCopy).not.toMatch(/阵容快照|陣容快照|lineup snapshot|スナップショット|편성 스냅샷|снимок состава/i)
    expect(localeCopy).not.toMatch(/伤害钩子|傷害鉤子|damage hooks?|ダメージフック|피해 훅|триггеры урона/i)
    expect(localeCopy).not.toMatch(/处理失败|處理失敗|processing failed|処理失敗|처리 실패|ошибка обработки/i)
  })

  it("keeps the match rules while removing implementation language from the tutorial", () => {
    const zhArena = collectStrings(zhTutorial.tutorial.arena).join(" ")
    const enArena = collectStrings(enTutorial.tutorial.arena).join(" ")

    expect(zhArena).toContain("最多累计 10 次")
    expect(zhArena).toContain("每 10 分钟恢复 1 次")
    expect(enArena).toContain("Up to 10 match entries")
    expect(enArena).toContain("one recovers every 10 minutes")

    expect(zhArena).not.toMatch(/战斗服务|独立战斗流|固定种子|阵容快照|钩子|机器人|测试账号|稳定账号顺序|样本外/)
    expect(enArena).not.toMatch(/battle service|independent stream|deterministic replays?|lineup snapshot|hooks?|\bbots?\b|test accounts?|stable account order|outside that sample/i)
  })
})
