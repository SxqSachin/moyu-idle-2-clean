import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

async function openCatteryKittiesTab(): Promise<void> {
  await ensureTabIndex("[data-tutorial='cattery-tabs']", 0)
}

export const catteryTutorial: TutorialConfig = {
  id: "cattery",
  nameKey: "tutorial.cattery.name",
  descriptionKey: "tutorial.cattery.description",
  icon: "game-icons:cat",
  version: 6,
  category: "social",
  order: 1,
  prerequisites: ["character"],
  steps: [
    {
      id: "cattery-page",
      target: { route: "/game/panel/cattery", selector: "[data-tutorial='cattery-page']", waitForMs: 6000, setup: openCatteryKittiesTab },
      popover: { titleKey: "tutorial.cattery.page.title", descriptionKey: "tutorial.cattery.page.desc", side: "bottom" },
    },
    {
      id: "cattery-summary",
      target: { selector: "[data-tutorial='cattery-summary']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.cattery.summary.title", descriptionKey: "tutorial.cattery.summary.desc", side: "bottom" },
    },
    {
      id: "cattery-tabs",
      target: { selector: "[data-tutorial='cattery-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.cattery.tabs.title", descriptionKey: "tutorial.cattery.tabs.desc", side: "bottom" },
    },
    {
      id: "cattery-kitties",
      target: { selector: "[data-tutorial='cattery-kitties']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.cattery.kitties.title", descriptionKey: "tutorial.cattery.kitties.desc", side: "bottom" },
    },
    {
      id: "cattery-journal",
      target: {
        selector: "[data-tutorial='cattery-journal'], [data-tutorial='cattery-tabs']",
        waitForMs: 4000,
        optional: true,
        setup: async () => {
          await ensureTabIndex("[data-tutorial='cattery-tabs']", 2)
        },
      },
      popover: { titleKey: "tutorial.cattery.journal.title", descriptionKey: "tutorial.cattery.journal.desc", side: "bottom" },
    },
  ],
}
