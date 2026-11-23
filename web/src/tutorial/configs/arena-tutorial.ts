import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

async function openArenaTab(index: number): Promise<void> {
  await ensureTabIndex("[data-tutorial='arena-tabs']", index, 6000)
}

export const arenaTutorial: TutorialConfig = {
  id: "arena",
  nameKey: "tutorial.arena.name",
  descriptionKey: "tutorial.arena.description",
  icon: "game-icons:cat",
  version: 6,
  category: "progression",
  order: 4,
  prerequisites: ["combat"],
  steps: [
    {
      id: "arena-page",
      target: { route: "/game/arena", selector: "[data-tutorial='arena-overview']", waitForMs: 6000, setup: () => openArenaTab(0) },
      popover: { titleKey: "tutorial.arena.page.title", descriptionKey: "tutorial.arena.page.desc", side: "bottom" },
    },
    {
      id: "arena-targets",
      target: { selector: "[data-tutorial='arena-targets']", waitForMs: 5000, setup: () => openArenaTab(0) },
      popover: { titleKey: "tutorial.arena.targets.title", descriptionKey: "tutorial.arena.targets.desc", side: "top" },
    },
    {
      id: "arena-formation",
      target: { selector: "[data-tutorial='arena-defense']", waitForMs: 5000, setup: () => openArenaTab(1) },
      popover: { titleKey: "tutorial.arena.formation.title", descriptionKey: "tutorial.arena.formation.desc", side: "top" },
    },
    {
      id: "arena-format-policy",
      target: { selector: "[data-tutorial='arena-format-policy']", waitForMs: 5000, setup: () => openArenaTab(1) },
      popover: { titleKey: "tutorial.arena.formatPolicy.title", descriptionKey: "tutorial.arena.formatPolicy.desc", side: "top" },
    },
    {
      id: "arena-update-team",
      target: { selector: "[data-tutorial='arena-defense-submit']", waitForMs: 5000, setup: () => openArenaTab(1) },
      popover: { titleKey: "tutorial.arena.updateTeam.title", descriptionKey: "tutorial.arena.updateTeam.desc", side: "top" },
    },
    {
      id: "arena-records",
      target: { selector: "[data-tutorial='arena-records']", waitForMs: 5000, setup: () => openArenaTab(2) },
      popover: { titleKey: "tutorial.arena.records.title", descriptionKey: "tutorial.arena.records.desc", side: "top" },
    },
    {
      id: "arena-leaderboard",
      target: { selector: "[data-tutorial='arena-leaderboard']", waitForMs: 5000, setup: () => openArenaTab(3) },
      popover: { titleKey: "tutorial.arena.leaderboard.title", descriptionKey: "tutorial.arena.leaderboard.desc", side: "top" },
    },
    {
      id: "arena-shop",
      target: { selector: "[data-tutorial='arena-shop']", waitForMs: 5000, setup: () => openArenaTab(4) },
      popover: { titleKey: "tutorial.arena.shop.title", descriptionKey: "tutorial.arena.shop.desc", side: "top" },
    },
  ],
}
