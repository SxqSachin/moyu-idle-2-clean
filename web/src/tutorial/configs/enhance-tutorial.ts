import type { TutorialConfig } from "../types"

export const enhanceTutorial: TutorialConfig = {
  id: "enhance",
  nameKey: "tutorial.enhance.name",
  descriptionKey: "tutorial.enhance.description",
  icon: "game-icons:upgrade",
  version: 3,
  category: "progression",
  order: 1,
  prerequisites: ["warehouse"],
  steps: [
    {
      id: "enhance-page",
      target: { route: "/game/enhance", selector: "[data-tutorial='enhance-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.enhance.page.title", descriptionKey: "tutorial.enhance.page.desc", side: "bottom" },
    },
    {
      id: "enhance-target",
      target: { selector: "[data-tutorial='enhance-target']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.enhance.select.title", descriptionKey: "tutorial.enhance.select.desc", side: "bottom" },
    },
    {
      id: "enhance-settings",
      target: { selector: "[data-tutorial='enhance-settings'], [data-tutorial='enhance-target']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.enhance.settings.title", descriptionKey: "tutorial.enhance.settings.desc", side: "bottom" },
    },
    {
      id: "enhance-preview",
      target: { selector: "[data-tutorial='enhance-preview'], [data-tutorial='enhance-target']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.enhance.preview.title", descriptionKey: "tutorial.enhance.preview.desc", side: "bottom" },
    },
    {
      id: "enhance-actions",
      target: { selector: "[data-tutorial='enhance-actions'], [data-tutorial='enhance-page']", optional: true },
      popover: { titleKey: "tutorial.enhance.enhanceBtn.title", descriptionKey: "tutorial.enhance.enhanceBtn.desc", side: "top" },
    },
  ],
}
