import type { TutorialConfig } from "../types"

export const achievementsTutorial: TutorialConfig = {
  id: "achievements",
  nameKey: "tutorial.achievements.name",
  descriptionKey: "tutorial.achievements.description",
  icon: "game-icons:trophy",
  version: 2,
  category: "progression",
  order: 6,
  steps: [
    {
      id: "achievements-page",
      target: { route: "/game/achievements", selector: "[data-tutorial='achievements-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.achievements.page.title", descriptionKey: "tutorial.achievements.page.desc", side: "bottom" },
    },
    {
      id: "achievements-stats",
      target: { selector: "[data-tutorial='achievements-stats']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.achievements.stats.title", descriptionKey: "tutorial.achievements.stats.desc", side: "bottom" },
    },
    {
      id: "achievements-tabs",
      target: { selector: "[data-tutorial='achievements-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.achievements.tabs.title", descriptionKey: "tutorial.achievements.tabs.desc", side: "top" },
    },
    {
      id: "achievements-categories",
      target: { selector: "[data-tutorial='achievements-categories']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.achievements.categories.title", descriptionKey: "tutorial.achievements.categories.desc", side: "bottom" },
    },
  ],
}
