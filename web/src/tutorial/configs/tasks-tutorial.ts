import type { TutorialConfig } from "../types"

export const tasksTutorial: TutorialConfig = {
  id: "tasks",
  nameKey: "tutorial.tasks.name",
  descriptionKey: "tutorial.tasks.description",
  icon: "game-icons:scroll-unfurled",
  version: 4,
  category: "progression",
  order: 4,
  prerequisites: ["actions"],
  steps: [
    {
      id: "tasks-page",
      target: { route: "/game/tasks", selector: "[data-tutorial='tasks-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.tasks.page.title", descriptionKey: "tutorial.tasks.page.desc", side: "bottom" },
    },
    {
      id: "tasks-reputation",
      target: { selector: "[data-tutorial='tasks-reputation'], [data-tutorial='tasks-page']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.tasks.reputation.title", descriptionKey: "tutorial.tasks.reputation.desc", side: "bottom" },
    },
    {
      id: "tasks-list",
      target: { selector: "[data-tutorial='tasks-list']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.tasks.taskList.title", descriptionKey: "tutorial.tasks.taskList.desc", side: "bottom" },
    },
    {
      id: "tasks-rumors",
      target: { selector: "[data-tutorial='tasks-list']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.tasks.rumors.title", descriptionKey: "tutorial.tasks.rumors.desc", side: "bottom" },
    },
  ],
}
