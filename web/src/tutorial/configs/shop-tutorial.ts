import type { TutorialConfig } from "../types"

export const shopTutorial: TutorialConfig = {
  id: "shop",
  nameKey: "tutorial.shop.name",
  descriptionKey: "tutorial.shop.description",
  icon: "game-icons:shopping-bag",
  version: 3,
  category: "economy",
  order: 1,
  prerequisites: ["actions"],
  steps: [
    {
      id: "shop-page",
      target: { route: "/game/shop", selector: "[data-tutorial='shop-page']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.shop.page.title", descriptionKey: "tutorial.shop.page.desc", side: "bottom" },
    },
    {
      id: "shop-offers",
      target: { selector: "[data-tutorial='shop-offers'], [data-tutorial='shop-toolbar']", waitForMs: 6000, optional: true },
      popover: { titleKey: "tutorial.shop.items.title", descriptionKey: "tutorial.shop.items.desc", side: "bottom" },
    },
  ],
}
