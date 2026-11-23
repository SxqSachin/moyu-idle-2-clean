import type { TutorialConfig } from "../types"

export const affixTutorial: TutorialConfig = {
  id: "affix",
  nameKey: "tutorial.affix.name",
  descriptionKey: "tutorial.affix.description",
  icon: "game-icons:sparkling-sabre",
  version: 3,
  category: "progression",
  order: 2,
  prerequisites: ["warehouse"],
  steps: [
    {
      id: "affix-page",
      target: { route: "/game/affix", selector: "[data-tutorial='affix-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.affix.page.title", descriptionKey: "tutorial.affix.page.desc", side: "bottom" },
    },
    {
      id: "affix-target",
      target: { selector: "[data-tutorial='affix-target'], [data-tutorial='affix-tabs']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.affix.target.title", descriptionKey: "tutorial.affix.target.desc", side: "bottom" },
    },
    {
      id: "affix-tabs",
      target: { selector: "[data-tutorial='affix-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.affix.tabs.title", descriptionKey: "tutorial.affix.tabs.desc", side: "bottom" },
    },
    {
      id: "affix-operation",
      target: { selector: "[data-tutorial='affix-operation'], [data-tutorial='affix-tabs']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.affix.operation.title", descriptionKey: "tutorial.affix.operation.desc", side: "top" },
    },
  ],
}
