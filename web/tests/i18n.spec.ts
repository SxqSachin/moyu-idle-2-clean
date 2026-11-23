import { describe, expect, it } from "vitest"
import zhAuth from "../src/locales/auth/zh-CN"
import enAuth from "../src/locales/auth/en-US"
import zhCore from "../src/locales/core/zh-CN"
import enCore from "../src/locales/core/en-US"
import zhGameData from "../src/locales/game-data/zh-CN"
import zhTwGameData from "../src/locales/game-data/zh-TW"
import enGameData from "../src/locales/game-data/en-US"
import jaGameData from "../src/locales/game-data/ja"
import koGameData from "../src/locales/game-data/ko"
import ruGameData from "../src/locales/game-data/ru"
import zhGame from "../src/locales/game/zh-CN"
import zhTwGame from "../src/locales/game/zh-TW"
import enGame from "../src/locales/game/en-US"
import jaGame from "../src/locales/game/ja"
import koGame from "../src/locales/game/ko"
import ruGame from "../src/locales/game/ru"
import zhHome from "../src/locales/home/zh-CN"
import enHome from "../src/locales/home/en-US"
import zhTutorial from "../src/locales/tutorial/zh-CN"
import enTutorial from "../src/locales/tutorial/en-US"
import { ensureLocaleNamespaces, getLocaleNamespacesForPath, getRouteLocaleNamespaces, isLocaleNamespaceLoaded, setAppLocale } from "../src/i18n"

function collectLeafKeys(obj: Record<string, unknown>, prefix = ""): string[] {
  const keys: string[] = []
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === "object" && !Array.isArray(value)) {
      keys.push(...collectLeafKeys(value as Record<string, unknown>, fullKey))
      continue
    }
    keys.push(fullKey)
  }
  return keys
}

function expectSyncedKeys(left: Record<string, unknown>, right: Record<string, unknown>) {
  expect(collectLeafKeys(left).sort()).toEqual(collectLeafKeys(right).sort())
}

function expectLocaleBlock(locale: Record<string, unknown>, path: string) {
  let cursor: unknown = locale
  for (const part of path.split(".")) {
    expect(cursor && typeof cursor === "object" && part in cursor).toBe(true)
    cursor = (cursor as Record<string, unknown>)[part]
  }
}

describe("i18n locale contract", () => {
  it("keeps zh-CN and en-US namespace keys in sync", () => {
    expectSyncedKeys(zhCore, enCore)
    expectSyncedKeys(zhHome, enHome)
    expectSyncedKeys(zhAuth, enAuth)
    expectSyncedKeys(zhGame, enGame)
    expectSyncedKeys(zhGameData, enGameData)
    expectSyncedKeys(zhTutorial, enTutorial)
  })

  it("describes battle-start hooks as recurring every wave", () => {
    expect(zhGame.game.itemDetailBattleHookTrigger.on_battle_start).toBe("每波战斗开始时")
    expect(zhTwGame.game.itemDetailBattleHookTrigger.on_battle_start).toBe("每波戰鬥開始時")
    expect(enGame.game.itemDetailBattleHookTrigger.on_battle_start).toBe("At the start of every wave")
  })

  it("keeps recurring battle-start summon copy aligned with runtime", () => {
    expect(zhGameData.item.rock_golem_silkwood_staff.effect).toContain("每波战斗开始时")
    expect(zhGameData.item.berry_birds_beloved_staff.effect).toContain("每波战斗开始时")
    expect(zhGameData.item.king_of_amusement_park.effect).toContain("每波战斗开始时")
    expect(enGameData.item.rock_golem_silkwood_staff.effect).toContain("start of every wave")
    expect(enGameData.item.berry_birds_beloved_staff.effect).toContain("Surviving birds carry into later waves")
    expect(enGameData.item.king_of_amusement_park.effect).toContain("start of every wave")
  })

  it("loads locale namespaces incrementally", async () => {
    await ensureLocaleNamespaces("zh-CN", ["core"])
    expect(isLocaleNamespaceLoaded("zh-CN", "core")).toBe(true)
    await setAppLocale("zh-CN", ["core", "home"])
    expect(isLocaleNamespaceLoaded("zh-CN", "home")).toBe(true)
  })

  it("loads gameData before entering routes that render generated data translations", () => {
    expect(getLocaleNamespacesForPath("/game/home")).toEqual(["core", "game", "gameData"])
    expect(getLocaleNamespacesForPath("/game/tasks")).toEqual(["core", "game", "gameData"])
    expect(getLocaleNamespacesForPath("/game/achievements")).toEqual(["core", "game", "gameData"])
    expect(getLocaleNamespacesForPath("/game/alchemy")).toEqual(["core", "game", "gameData"])
    expect(getLocaleNamespacesForPath("/auth")).toEqual(["core", "auth"])
    expect(getRouteLocaleNamespaces("/game/achievements", ["core", "game"])).toEqual(["core", "game", "gameData"])
  })

  it("keeps migrated game page locale blocks available for every locale", () => {
    for (const locale of [zhGame, zhTwGame, enGame, jaGame, koGame, ruGame]) {
      expectLocaleBlock(locale, "achievements.title")
      expectLocaleBlock(locale, "achievements.claim")
      expectLocaleBlock(locale, "achievements.stageCompact")
      expectLocaleBlock(locale, "achievements.generatedTitle.complete_action")
      expectLocaleBlock(locale, "achievements.generatedTitle.catch_fish")
      expectLocaleBlock(locale, "achievements.condition.catch_fish")
      expectLocaleBlock(locale, "achievements.subcategory.otherFishing")
      expectLocaleBlock(locale, "alchemy.convert.title")
      expectLocaleBlock(locale, "alchemy.convert.failedOutcome")
      expectLocaleBlock(locale, "alchemy.bench.title")
      expectLocaleBlock(locale, "alchemy.codex.open")
      expectLocaleBlock(locale, "game.onboardingPrompt.title")
      expectLocaleBlock(locale, "game.onboardingPrompt.skipHint")
    }
  })

  it("keeps title locale blocks available for every game data locale", () => {
    for (const locale of [zhGameData, zhTwGameData, enGameData, jaGameData, koGameData, ruGameData]) {
      expectLocaleBlock(locale, "title_category.special.name")
      expectLocaleBlock(locale, "title_prefix.angry.name")
      expectLocaleBlock(locale, "title_prefix.cat_slave.name")
      expectLocaleBlock(locale, "title_prefix.next_time_sure.name")
      expectLocaleBlock(locale, "title_prefix.grind_king.name")
      expectLocaleBlock(locale, "title_suffix.cat_lord.name")
      expectLocaleBlock(locale, "title_suffix.boss_slayer.name")
    }
  })
})
