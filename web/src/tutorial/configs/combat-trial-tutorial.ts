import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

async function openCombatTrialTab(index: number): Promise<void> {
  await ensureTabIndex("[data-tutorial='combat-trial-tabs']", index, 6000)
}

export const combatTrialTutorial: TutorialConfig = {
  id: "combat-trial",
  nameKey: "tutorial.combatTrial.name",
  descriptionKey: "tutorial.combatTrial.description",
  icon: "lucide:target",
  version: 1,
  category: "progression",
  order: 6,
  prerequisites: ["combat"],
  steps: [
    {
      id: "combat-trial-page",
      target: { route: "/game/combat-trial", selector: "[data-tutorial='combat-trial-page']", setup: () => openCombatTrialTab(0), waitForMs: 6000 },
      popover: { titleKey: "tutorial.combatTrial.page.title", descriptionKey: "tutorial.combatTrial.page.desc", side: "bottom" },
    },
    {
      id: "combat-trial-tabs",
      target: { selector: "[data-tutorial='combat-trial-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.combatTrial.tabs.title", descriptionKey: "tutorial.combatTrial.tabs.desc", side: "bottom" },
    },
    {
      id: "combat-trial-projects",
      target: { selector: "[data-tutorial='combat-trial-projects']", setup: () => openCombatTrialTab(0), waitForMs: 6000, optional: true },
      popover: { titleKey: "tutorial.combatTrial.projects.title", descriptionKey: "tutorial.combatTrial.projects.desc", side: "bottom" },
    },
    {
      id: "combat-trial-rules",
      target: { selector: "[data-tutorial='combat-trial-rules']", setup: () => openCombatTrialTab(0), waitForMs: 6000, optional: true },
      popover: { titleKey: "tutorial.combatTrial.rules.title", descriptionKey: "tutorial.combatTrial.rules.desc", side: "top" },
    },
    {
      id: "combat-trial-lineup",
      target: { selector: "[data-tutorial='combat-trial-lineup']", setup: () => openCombatTrialTab(0), waitForMs: 6000, optional: true },
      popover: { titleKey: "tutorial.combatTrial.lineup.title", descriptionKey: "tutorial.combatTrial.lineup.desc", side: "top" },
    },
    {
      id: "combat-trial-start",
      target: { selector: "[data-tutorial='combat-trial-start']", setup: () => openCombatTrialTab(0), waitForMs: 6000, optional: true },
      popover: { titleKey: "tutorial.combatTrial.start.title", descriptionKey: "tutorial.combatTrial.start.desc", side: "top" },
    },
    {
      id: "combat-trial-leaderboard",
      target: { selector: "[data-tutorial='combat-trial-leaderboard']", setup: () => openCombatTrialTab(1), waitForMs: 6000 },
      popover: { titleKey: "tutorial.combatTrial.leaderboard.title", descriptionKey: "tutorial.combatTrial.leaderboard.desc", side: "bottom" },
    },
    {
      id: "combat-trial-history",
      target: { selector: "[data-tutorial='combat-trial-history']", setup: () => openCombatTrialTab(2), waitForMs: 6000 },
      popover: { titleKey: "tutorial.combatTrial.history.title", descriptionKey: "tutorial.combatTrial.history.desc", side: "bottom" },
    },
  ],
}
