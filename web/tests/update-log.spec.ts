import { describe, expect, it } from "vitest"
import { updateLogTimeline } from "../src/data/update-log"
import { getEnemyDefinition } from "../src/services/enemy-definition"
import { getItemDefinition } from "../src/services/item-definition"
import { getSkillDefinition } from "../src/services/skill-definition"
import { parseUpdateLogLineSegments } from "../src/services/update-log-content"

const QUICK_REF_PATTERN = /\b(item|skill|enemy):([A-Za-z0-9_](?:[A-Za-z0-9_.-]*[A-Za-z0-9_])?)/g
function itemReferences(text: string): string[] {
  return [...text.matchAll(/\bitem:([A-Za-z0-9_](?:[A-Za-z0-9_.-]*[A-Za-z0-9_])?)/g)]
    .map((match) => match[1])
}

describe("update log", () => {
  it("publishes complete pond candidates and shared fish filters in both locales", () => {
    const entry = updateLogTimeline.find(candidate => candidate.id === "2026-09-07-fish-filters")
    expect(updateLogTimeline[0]).toBe(entry)
    expect(entry?.items["zh-CN"].join("\n")).toContain("跨页选择")
    expect(entry?.items["zh-CN"].join("\n")).toContain("变异筛选")
    expect(entry?.items["en-US"].join("\n")).toContain("mutation filters")
  })
  it("publishes the new equipment skills in both locales", () => {
    const entry = updateLogTimeline.find(candidate => candidate.id === "2026-09-07-equipment-skills")
    expect(updateLogTimeline).toContain(entry)
    expect(entry?.items["zh-CN"].join("\n")).toContain("统御印记")
    expect(entry?.items["en-US"].join("\n")).toContain("Martial Chain")
  })
  it("publishes the September 6 fix roundup in both locales without duplicate lines", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-06")
    expect(updateLogTimeline).toContain(entry)
    expect(entry?.date).toBe("2026-09-06")

    const topics = [
      ["支持的钓法", "supported methods"],
      ["逐件检查鱼竿、鱼线轮", "checks each rod and reel"],
      ["离屏或页面进入后台", "off-screen or the page is in the background"],
      ["背景拉伸变形", "stretch out of proportion"],
      ["最多显示两行", "up to two lines"],
      ["鱼汤最多储备24小时", "Fish soups can hold up to 24 hours"],
      ["批量食用", "batch eating"],
      ["家具材料标签过高", "furniture material tags"],
    ]
    const zh = entry?.items["zh-CN"].join("\n") ?? ""
    const en = entry?.items["en-US"].join("\n") ?? ""
    for (const [zhTopic, enTopic] of topics) {
      expect(zh).toContain(zhTopic)
      expect(en).toContain(enTopic)
    }
    expect(entry?.items["zh-CN"].length).toBe(entry?.items["en-US"].length)
    for (const locale of ["zh-CN", "en-US"] as const) {
      const lines = entry?.items[locale] ?? []
      expect(new Set(lines).size).toBe(lines.length)
      const menus = lines.flatMap(parseUpdateLogLineSegments).filter((segment) => segment.kind === "menu")
      expect(menus.map((segment) => segment.key)).toEqual(["fishing", "fishing", "pond", "pond"])
    }
  })

  it("publishes kitty fishing requirement fixes in both locales", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-06")
    expect(entry?.items["zh-CN"].join("\n")).toContain("不再被玩家自身的钓鱼等级阻挡")
    expect(entry?.items["zh-CN"].join("\n")).toContain("按猫咪自身属性显示")
    expect(entry?.items["en-US"].join("\n")).toContain("no longer blocked by the player's Fishing level")
    expect(entry?.items["en-US"].join("\n")).toContain("kitty's own attributes")
  })

  it("publishes the specialization state refresh fix in both locales", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-04")
    const zh = entry?.items["zh-CN"].join("\n") ?? ""
    const en = entry?.items["en-US"].join("\n") ?? ""

    expect(zh).toContain("一直无法分配专精")
    expect(en).toContain("remain unallocatable")
  })

  it("publishes specialization refresh and success-reporting fixes in both locales", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-05")
    expect(entry?.items["zh-CN"].join("\n")).toContain("不会再误报操作失败")
    expect(entry?.items["en-US"].join("\n")).toContain("Successful changes are no longer reported as failed")
  })

  it("publishes the wave-trigger fixes in both locales", () => {
    const latest = updateLogTimeline.find((candidate) => candidate.id === "2026-08-29")
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-08-15")
    const zh = latest?.items["zh-CN"].join("\n") ?? ""
    const en = latest?.items["en-US"].join("\n") ?? ""

    const recurringSummonItems = [
      "rock_golem_silkwood_staff",
      "berry_birds_beloved_staff",
      "king_of_amusement_park",
    ]
    expect(itemReferences(zh)).toEqual(recurringSummonItems)
    expect(itemReferences(en)).toEqual(recurringSummonItems)
    expect(zh).toContain("每个敌人刷新波次")
    expect(zh).toContain("错误的整场一次上限")
    expect(en).toContain("every enemy-refresh wave")
    expect(en).toContain("incorrect once-per-battle limit")
    expect(entry?.items["zh-CN"].join("\n")).toContain("流星回廊")
    expect(entry?.items["zh-CN"].join("\n")).toContain("禁魔图书馆")
    expect(entry?.items["zh-CN"].join("\n")).toContain("item:stellar_testament_pendant")
    expect(entry?.items["zh-CN"].join("\n")).toContain("item:five_tick_starfall_lunar_veil_robe")
    expect(entry?.items["zh-CN"].join("\n")).toContain("技能炼成")
    expect(entry?.items["en-US"].join("\n")).toContain("Meteor Corridor")
    expect(entry?.items["en-US"].join("\n")).toContain("Forbidden Library")
    expect(entry?.items["en-US"].join("\n")).toContain("item:five_tick_starfall_lunar_veil_robe")
    expect(entry?.items["en-US"].join("\n")).toContain("Skill Forge")
    expect(entry?.items["en-US"].join("\n")).toContain("mythic-material parameters now require 15 items per batch")
  })

  it("publishes the kitty fishing and equipment synchronization fixes", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-03")
    const zh = entry?.items["zh-CN"].join("\n") ?? ""
    const en = entry?.items["en-US"].join("\n") ?? ""

    expect(zh).toContain("穿脱装备")
    expect(zh).toContain("应用套装")
    expect(zh).toContain("调整专精")
    expect(zh).toContain("不会再卡住或提前结束")
    expect(zh).toContain("装备槽现在会立即正确更新")
    expect(en).toContain("active or queued")
    expect(en).toContain("no longer gets stuck or ends early")
    expect(en).toContain("equipment slots now update immediately")
  })

  it("publishes recent player-facing changes without the alchemy cap adjustment", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-03")
    const zh = entry?.items["zh-CN"].join("\n") ?? ""
    const en = entry?.items["en-US"].join("\n") ?? ""

    expect(zh).toContain("最近20次钓行")
    expect(zh).toContain("捕获见闻")
    expect(zh).toContain("扔石头和打水漂")
    expect(zh).toContain("其他玩家资料加入池塘页签")
    expect(zh).toContain("特殊异变")
    expect(zh).toContain("聊天消息可以附带")
    expect(zh).toContain("虚拟滚动")
    expect(zh).toContain("误退出登录")
    expect(en).toContain("latest 20 trips")
    expect(en).toContain("catch milestones")
    expect(en).toContain("throwing a stone, and skipping stones")
    expect(en).toContain("Player profiles now include a pond tab")
    expect(en).toContain("special mutations")
    expect(en).toContain("Chat messages can include")
    expect(en).toContain("virtual scrolling")
    expect(en).toContain("no longer signs you out")
    expect(zh).not.toMatch(/炼金.{0,12}上限|上限.{0,12}炼金/)
    expect(en).not.toMatch(/alchemy.{0,20}(cap|limit)|(cap|limit).{0,20}alchemy/i)
  })

  it("publishes the applause crown cooldown adjustment in both locales", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-02.c")
    const zh = entry?.items["zh-CN"].join("\n") ?? ""
    const en = entry?.items["en-US"].join("\n") ?? ""

    expect(itemReferences(zh)).toContain("full_house_applause_crown")
    expect(itemReferences(en)).toContain("full_house_applause_crown")
    expect(zh).toContain("余音未歇")
    expect(zh).toContain("本次额外回合计入冷却")
    expect(en).toContain("Lingering Ovation")
    expect(en).toContain("granted extra turn counts toward the cooldown")
  })

  it("publishes the unified fishing angler flow in both locales", () => {
    const entry = updateLogTimeline.find((candidate) => candidate.id === "2026-09-02.d")
    const zh = entry?.items["zh-CN"].join("\n") ?? ""
    const en = entry?.items["en-US"].join("\n") ?? ""

    expect(zh).toContain("当前钓手")
    expect(zh).toContain("角色名 - 我")
    expect(zh).toContain("不再重复选择角色")
    expect(en).toContain("current angler")
    expect(en).toContain("character name - Me")
    expect(en).toContain("reuses the header selection")
  })

  it("only publishes quick references that resolve to game data", () => {
    for (const entryID of ["2026-09-03", "2026-09-02.c", "2026-08-29", "2026-08-15"]) {
      const entry = updateLogTimeline.find((candidate) => candidate.id === entryID)
      const text = [...(entry?.items["zh-CN"] ?? []), ...(entry?.items["en-US"] ?? [])].join("\n")
      const references = [...text.matchAll(QUICK_REF_PATTERN)]

      expect(references.length).toBeGreaterThan(0)
      for (const [, type, id] of references) {
        const definition = type === "item"
          ? getItemDefinition(id)
          : type === "skill"
            ? getSkillDefinition(id)
            : getEnemyDefinition(id)
        expect(definition, `${entryID} ${type}:${id}`).toBeTruthy()
      }
    }
  })

  it("resolves allowlisted menu links and preserves unknown menu keys as text", () => {
    const linkedMenus = [
      { entryID: "2026-09-02.a", locale: "zh-CN" as const, key: "pond", label: "池塘", to: "/game/pond" },
      { entryID: "2026-09-01", locale: "zh-CN" as const, key: "combatTrial", label: "猫猫试炼", to: "/game/combat-trial" },
      { entryID: "2026-09-01", locale: "zh-CN" as const, key: "fishing", label: "钓鱼", to: "/game/fishing" },
      { entryID: "2026-08-30", locale: "zh-CN" as const, key: "arena", label: "猫村联赛", to: "/game/arena" },
      { entryID: "2026-09-02.a", locale: "en-US" as const, key: "pond", label: "Pond", to: "/game/pond" },
      { entryID: "2026-09-01", locale: "en-US" as const, key: "combatTrial", label: "Cat Trial", to: "/game/combat-trial" },
      { entryID: "2026-09-01", locale: "en-US" as const, key: "fishing", label: "Fishing", to: "/game/fishing" },
      { entryID: "2026-08-30", locale: "en-US" as const, key: "arena", label: "Cat Village League", to: "/game/arena" },
    ]

    for (const expected of linkedMenus) {
      const entry = updateLogTimeline.find((candidate) => candidate.id === expected.entryID)
      const line = entry?.items[expected.locale].find((candidate) => candidate.includes(`menu:${expected.key}`))
      const menuSegment = parseUpdateLogLineSegments(line || "").find((segment) => segment.kind === "menu")

      expect(menuSegment).toEqual({
        kind: "menu",
        key: expected.key,
        label: expected.label,
        to: expected.to,
      })
    }

    const unknownLink = "New: [Unknown menu](menu:notRegistered)"
    expect(parseUpdateLogLineSegments(unknownLink)).toEqual([{ kind: "text", text: unknownLink }])
  })

  it("keeps existing quick references when a line also contains a menu link", () => {
    expect(parseUpdateLogLineSegments("Added item:rock_golem_silkwood_staff, then [Fishing](menu:fishing)")).toEqual([
      { kind: "text", text: "Added " },
      { kind: "item", id: "rock_golem_silkwood_staff" },
      { kind: "text", text: ", then " },
      { kind: "menu", key: "fishing", label: "Fishing", to: "/game/fishing" },
    ])
  })
})
