import type { TutorialConfig } from "../types"

export const homeTutorial: TutorialConfig = {
  id: "home",
  nameKey: "tutorial.home.name",
  descriptionKey: "tutorial.home.description",
  icon: "game-icons:home-garage",
  version: 3,
  category: "core",
  order: 8,
  prerequisites: ["actions"],
  steps: [
    {
      id: "home-queues",
      target: { route: "/game/home", selector: "[data-tutorial='home-queues']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.home.queue.title", descriptionKey: "tutorial.home.queue.desc", side: "bottom" },
    },
    {
      id: "queue-lane",
      target: { selector: "[data-tutorial='queue-lane']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.home.kittyLane.title", descriptionKey: "tutorial.home.kittyLane.desc", side: "top" },
    },
  ],
}
