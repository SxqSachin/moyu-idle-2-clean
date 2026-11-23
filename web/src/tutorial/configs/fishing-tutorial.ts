import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

async function openFishingTab(index: number): Promise<void> {
  await ensureTabIndex("[data-tutorial='fishing-tabs']", index, 6000)
}

export const fishingTutorial: TutorialConfig = {
  id: "fishing",
  nameKey: "tutorial.fishing.name",
  descriptionKey: "tutorial.fishing.description",
  icon: "game-icons:fishing-hook",
  version: 2,
  category: "progression",
  order: 8,
  prerequisites: ["actions"],
  steps: [
    {
      id: "fishing-page",
      target: { route: "/game/fishing", selector: "[data-tutorial='fishing-page']", setup: () => openFishingTab(0), waitForMs: 6000 },
      popover: { titleKey: "tutorial.fishing.page.title", descriptionKey: "tutorial.fishing.page.desc", side: "bottom" },
    },
    {
      id: "fishing-tabs",
      target: { selector: "[data-tutorial='fishing-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.fishing.tabs.title", descriptionKey: "tutorial.fishing.tabs.desc", side: "bottom" },
    },
    {
      id: "fishing-waters",
      target: { selector: "[data-tutorial='fishing-waters']", setup: () => openFishingTab(0), waitForMs: 6000, optional: true },
      popover: { titleKey: "tutorial.fishing.waters.title", descriptionKey: "tutorial.fishing.waters.desc", side: "bottom" },
    },
    {
      id: "fishing-trip",
      target: { selector: "[data-tutorial='fishing-trip-start']", setup: () => openFishingTab(0), waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.fishing.trip.title", descriptionKey: "tutorial.fishing.trip.desc", side: "top" },
    },
    {
      id: "fishing-loadouts",
      target: { selector: "[data-tutorial='fishing-loadouts']", setup: () => openFishingTab(1), waitForMs: 5000 },
      popover: { titleKey: "tutorial.fishing.loadouts.title", descriptionKey: "tutorial.fishing.loadouts.desc", side: "bottom" },
    },
    {
      id: "fishing-catches",
      target: { selector: "[data-tutorial='fishing-catches']", setup: () => openFishingTab(2), waitForMs: 5000 },
      popover: { titleKey: "tutorial.fishing.catches.title", descriptionKey: "tutorial.fishing.catches.desc", side: "bottom" },
    },
    {
      id: "fishing-history",
      target: { selector: "[data-tutorial='fishing-history']", setup: () => openFishingTab(3), waitForMs: 5000 },
      popover: { titleKey: "tutorial.fishing.history.title", descriptionKey: "tutorial.fishing.history.desc", side: "bottom" },
    },
    {
      id: "fishing-tournaments",
      target: { selector: "[data-tutorial='fishing-tournaments']", setup: () => openFishingTab(4), waitForMs: 6000 },
      popover: { titleKey: "tutorial.fishing.tournaments.title", descriptionKey: "tutorial.fishing.tournaments.desc", side: "bottom" },
    },
  ],
}
