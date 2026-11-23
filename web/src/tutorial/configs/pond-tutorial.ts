import type { TutorialConfig } from "../types"

export const pondTutorial: TutorialConfig = {
  id: "pond",
  nameKey: "tutorial.pond.name",
  descriptionKey: "tutorial.pond.description",
  icon: "game-icons:fish-eggs",
  version: 1,
  category: "progression",
  order: 9,
  prerequisites: ["fishing"],
  steps: [
    {
      id: "pond-page",
      target: { route: "/game/pond", selector: "[data-tutorial='pond-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.pond.page.title", descriptionKey: "tutorial.pond.page.desc", side: "bottom" },
    },
    {
      id: "pond-time",
      target: { selector: "[data-tutorial='pond-status']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.pond.time.title", descriptionKey: "tutorial.pond.time.desc", side: "bottom" },
    },
    {
      id: "pond-water",
      target: { selector: "[data-tutorial='pond-water']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.pond.water.title", descriptionKey: "tutorial.pond.water.desc", side: "bottom" },
    },
    {
      id: "pond-residents",
      target: { selector: "[data-tutorial='pond-residents']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.pond.residents.title", descriptionKey: "tutorial.pond.residents.desc", side: "bottom" },
    },
    {
      id: "pond-feed",
      target: { selector: "[data-tutorial='pond-feed']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.pond.feed.title", descriptionKey: "tutorial.pond.feed.desc", side: "left" },
    },
    {
      id: "pond-breeding",
      target: { selector: "[data-tutorial='pond-breeding']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.pond.breeding.title", descriptionKey: "tutorial.pond.breeding.desc", side: "left" },
    },
    {
      id: "pond-events",
      target: { selector: "[data-tutorial='pond-events']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.pond.events.title", descriptionKey: "tutorial.pond.events.desc", side: "left" },
    },
  ],
}
