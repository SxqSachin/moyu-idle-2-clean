import { describe, expect, it } from "vitest"
import zhTutorial from "../src/locales/tutorial/zh-CN"
import enTutorial from "../src/locales/tutorial/en-US"
import { getAllTutorialIds, loadTutorialConfig } from "../src/tutorial/configs"
import { getTutorialGuidePlacement, recommendedTutorialIds } from "../src/tutorial/guide"

function expectLocaleBlock(locale: Record<string, unknown>, path: string) {
  let cursor: unknown = locale
  for (const part of path.split(".")) {
    expect(cursor && typeof cursor === "object" && part in cursor).toBe(true)
    cursor = (cursor as Record<string, unknown>)[part]
  }
}

describe("tutorial contract", () => {
  it("registers the basic onboarding flow and recommended path tutorials", () => {
    const ids = getAllTutorialIds()

    expect(ids).toContain("onboarding")
    expect(ids).toContain("interactions")
    for (const id of recommendedTutorialIds) {
      expect(ids).toContain(id)
    }
  })

  it("places the basic onboarding entries at the top of the guide", () => {
    expect(getTutorialGuidePlacement("onboarding")).toEqual({ groupId: "recommended", order: 1 })
    expect(getTutorialGuidePlacement("interactions")).toEqual({ groupId: "recommended", order: 2 })
  })

  it("keeps onboarding locale blocks available", () => {
    for (const locale of [zhTutorial, enTutorial]) {
      expectLocaleBlock(locale, "tutorial.guide.recommendedTitle")
      expectLocaleBlock(locale, "tutorial.guide.recommendedDesc")
      expectLocaleBlock(locale, "tutorial.guideGroup.recommended")
      expectLocaleBlock(locale, "tutorial.onboarding.name")
      expectLocaleBlock(locale, "tutorial.onboarding.welcome.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.home.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.details.descMobile")
      expectLocaleBlock(locale, "tutorial.onboarding.nextGoal.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.achievements.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.building.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.trade.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.combat.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.enhance.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.affix.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.alchemy.desc")
      expectLocaleBlock(locale, "tutorial.onboarding.guide.desc")
    }
  })

  it("registers a complete skill forge tutorial in both locales", async () => {
    expect(getAllTutorialIds()).toContain("alchemy-skill-forge")
    expect(getTutorialGuidePlacement("alchemy-skill-forge")).toEqual({ groupId: "growth", order: 8 })

    const config = await loadTutorialConfig("alchemy-skill-forge")
    expect(config?.steps.map((step) => step.id)).toEqual(expect.arrayContaining([
      "alchemy-skill-forge-overview",
      "alchemy-skill-forge-source",
      "alchemy-skill-forge-materials",
      "alchemy-skill-forge-graph",
      "alchemy-skill-forge-preview",
      "alchemy-skill-forge-submit",
    ]))

    for (const locale of [zhTutorial, enTutorial]) {
      expectLocaleBlock(locale, "tutorial.alchemySkillForge.name")
      expectLocaleBlock(locale, "tutorial.alchemySkillForge.source.desc")
      expectLocaleBlock(locale, "tutorial.alchemySkillForge.materials.desc")
      expectLocaleBlock(locale, "tutorial.alchemySkillForge.graph.desc")
      expectLocaleBlock(locale, "tutorial.alchemySkillForge.preview.desc")
      expectLocaleBlock(locale, "tutorial.alchemySkillForge.submit.desc")
    }
  })

  it("registers the cat formation tutorial with compensation and replay guidance", async () => {
    expect(getAllTutorialIds()).toContain("arena")
    expect(getTutorialGuidePlacement("arena")).toEqual({ groupId: "combat", order: 2 })

    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined }),
    })
    const config = await loadTutorialConfig("arena")
    expect(config?.steps.map((step) => step.id)).toEqual([
      "arena-page",
      "arena-targets",
      "arena-formation",
      "arena-format-policy",
      "arena-update-team",
      "arena-records",
      "arena-leaderboard",
      "arena-shop",
    ])

    for (const locale of [zhTutorial, enTutorial]) {
      expectLocaleBlock(locale, "tutorial.arena.name")
      expectLocaleBlock(locale, "tutorial.arena.formation.desc")
      expectLocaleBlock(locale, "tutorial.arena.formatPolicy.desc")
      expectLocaleBlock(locale, "tutorial.arena.updateTeam.desc")
      expectLocaleBlock(locale, "tutorial.arena.records.desc")
      expectLocaleBlock(locale, "tutorial.arena.leaderboard.desc")
      expectLocaleBlock(locale, "tutorial.arena.shop.desc")
    }
  })

  it("registers the complete fishing tutorial in the action guide", async () => {
    expect(getAllTutorialIds()).toContain("fishing")
    expect(getTutorialGuidePlacement("fishing")).toEqual({ groupId: "actions", order: 3 })

    const config = await loadTutorialConfig("fishing")
    expect(config?.steps.map((step) => step.id)).toEqual([
      "fishing-page",
      "fishing-tabs",
      "fishing-waters",
      "fishing-trip",
      "fishing-loadouts",
      "fishing-catches",
      "fishing-history",
      "fishing-tournaments",
    ])

    for (const locale of [zhTutorial, enTutorial]) {
      expectLocaleBlock(locale, "tutorial.fishing.name")
      expectLocaleBlock(locale, "tutorial.fishing.waters.desc")
      expectLocaleBlock(locale, "tutorial.fishing.trip.desc")
      expectLocaleBlock(locale, "tutorial.fishing.loadouts.desc")
      expectLocaleBlock(locale, "tutorial.fishing.catches.desc")
      expectLocaleBlock(locale, "tutorial.fishing.history.desc")
      expectLocaleBlock(locale, "tutorial.fishing.tournaments.desc")
    }
  })

  it("registers a dedicated pond tutorial with the accelerated time contract", async () => {
    expect(getAllTutorialIds()).toContain("pond")
    expect(getTutorialGuidePlacement("pond")).toEqual({ groupId: "actions", order: 4 })

    const config = await loadTutorialConfig("pond")
    expect(config?.steps.map((step) => step.id)).toEqual([
      "pond-page",
      "pond-time",
      "pond-water",
      "pond-residents",
      "pond-feed",
      "pond-breeding",
      "pond-events",
    ])

    for (const locale of [zhTutorial, enTutorial]) {
      expectLocaleBlock(locale, "tutorial.pond.name")
      expectLocaleBlock(locale, "tutorial.pond.time.desc")
      expectLocaleBlock(locale, "tutorial.pond.residents.desc")
      expectLocaleBlock(locale, "tutorial.pond.feed.desc")
      expectLocaleBlock(locale, "tutorial.pond.breeding.desc")
      expectLocaleBlock(locale, "tutorial.pond.events.desc")
    }

    expect(zhTutorial.tutorial.pond.time.desc).toContain("现实 1 天 = 生物成长 365 天")
    expect(enTutorial.tutorial.pond.time.desc).toContain("1 real day equals 365 days")
  })

  it("registers the cat trial tutorial with format, lineup, and replay guidance", async () => {
    expect(getAllTutorialIds()).toContain("combat-trial")
    expect(getTutorialGuidePlacement("combat-trial")).toEqual({ groupId: "combat", order: 3 })

    const config = await loadTutorialConfig("combat-trial")
    expect(config?.steps.map((step) => step.id)).toEqual([
      "combat-trial-page",
      "combat-trial-tabs",
      "combat-trial-projects",
      "combat-trial-rules",
      "combat-trial-lineup",
      "combat-trial-start",
      "combat-trial-leaderboard",
      "combat-trial-history",
    ])

    for (const locale of [zhTutorial, enTutorial]) {
      expectLocaleBlock(locale, "tutorial.combatTrial.name")
      expectLocaleBlock(locale, "tutorial.combatTrial.projects.desc")
      expectLocaleBlock(locale, "tutorial.combatTrial.rules.desc")
      expectLocaleBlock(locale, "tutorial.combatTrial.lineup.desc")
      expectLocaleBlock(locale, "tutorial.combatTrial.start.desc")
      expectLocaleBlock(locale, "tutorial.combatTrial.leaderboard.desc")
      expectLocaleBlock(locale, "tutorial.combatTrial.history.desc")
    }
  })
})
