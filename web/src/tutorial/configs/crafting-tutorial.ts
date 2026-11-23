import type { TutorialConfig } from "../types"

export const craftingTutorial: TutorialConfig = {
  id: "crafting",
  nameKey: "tutorial.crafting.name",
  descriptionKey: "tutorial.crafting.description",
  icon: "game-icons:anvil",
  version: 3,
  category: "progression",
  order: 8,
  prerequisites: ["actions"],
  steps: [
    {
      id: "crafting-panel",
      target: { route: "/game/crafting", selector: "[data-tutorial='action-panel']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.crafting.page.title", descriptionKey: "tutorial.crafting.page.desc", side: "bottom" },
    },
    {
      id: "crafting-recipes",
      target: { selector: "[data-tutorial='action-cards'], .action-card", waitForMs: 6000 },
      popover: { titleKey: "tutorial.crafting.recipes.title", descriptionKey: "tutorial.crafting.recipes.desc", side: "top" },
    },
  ],
}
