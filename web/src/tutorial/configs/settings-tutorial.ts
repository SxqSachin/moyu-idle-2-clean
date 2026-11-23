import type { TutorialConfig } from "../types"
import { clickTab } from "../helpers"

export const settingsTutorial: TutorialConfig = {
  id: "settings",
  nameKey: "tutorial.settings.name",
  descriptionKey: "tutorial.settings.description",
  icon: "game-icons:gear-hammer",
  version: 4,
  category: "social",
  order: 2,
  steps: [
    {
      id: "settings-page",
      target: { route: "/game/panel/settings", selector: "[data-tutorial='settings-panel']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.settings.page.title", descriptionKey: "tutorial.settings.page.desc", side: "bottom" },
    },
    {
      id: "settings-tabs",
      target: { selector: "[data-tutorial='settings-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.settings.options.title", descriptionKey: "tutorial.settings.options.desc", side: "top" },
    },
    {
      id: "settings-title",
      target: {
        selector: "[data-tutorial='settings-title']",
        waitForMs: 4000,
        setup: async () => {
          await clickTab("[data-tutorial='settings-tabs']", 3)
        },
      },
      popover: { titleKey: "tutorial.settings.title.title", descriptionKey: "tutorial.settings.title.desc", side: "top" },
    },
  ],
}
