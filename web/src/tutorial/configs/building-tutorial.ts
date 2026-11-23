import type { TutorialConfig } from "../types"

export const buildingTutorial: TutorialConfig = {
  id: "building",
  nameKey: "tutorial.building.name",
  descriptionKey: "tutorial.building.description",
  icon: "game-icons:house",
  version: 3,
  category: "progression",
  order: 5,
  prerequisites: ["actions"],
  steps: [
    {
      id: "building-page",
      target: { route: "/game/building", selector: "[data-tutorial='building-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.building.page.title", descriptionKey: "tutorial.building.page.desc", side: "bottom" },
    },
    {
      id: "building-actor",
      target: { selector: "[data-tutorial='building-actor']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.building.actor.title", descriptionKey: "tutorial.building.actor.desc", side: "bottom" },
    },
    {
      id: "building-list",
      target: { selector: "[data-tutorial='building-list']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.building.buildList.title", descriptionKey: "tutorial.building.buildList.desc", side: "bottom" },
    },
  ],
}
