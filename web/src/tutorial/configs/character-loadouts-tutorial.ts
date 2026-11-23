import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

const CHARACTER_TUTORIAL_OPEN_TAB_EVENT = "character:tutorial-open-tab"

async function openCharacterLoadoutsTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_OPEN_TAB_EVENT, { detail: { tab: "loadouts" } }))
  await ensureTabIndex("[data-tutorial='character-tabs']", 4)
}

export const characterLoadoutsTutorial: TutorialConfig = {
  id: "character-loadouts",
  nameKey: "tutorial.characterLoadouts.name",
  descriptionKey: "tutorial.characterLoadouts.description",
  icon: "game-icons:swap-bag",
  version: 4,
  category: "core",
  order: 7,
  prerequisites: ["character"],
  steps: [
    {
      id: "loadouts-tab",
      target: {
        route: "/game/attributes",
        selector: "[data-tutorial='character-tabs']",
        waitForMs: 6000,
        setup: openCharacterLoadoutsTab,
      },
      popover: { titleKey: "tutorial.characterLoadouts.switchTab.title", descriptionKey: "tutorial.characterLoadouts.switchTab.desc", side: "bottom" },
    },
    {
      id: "loadouts-content",
      target: { selector: "[data-tutorial='loadouts-panel']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterLoadouts.content.title", descriptionKey: "tutorial.characterLoadouts.content.desc", side: "bottom" },
    },
    {
      id: "loadouts-battle",
      target: { selector: "[data-tutorial='loadouts-battle']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterLoadouts.battle.title", descriptionKey: "tutorial.characterLoadouts.battle.desc", side: "bottom" },
    },
    {
      id: "loadouts-work",
      target: {
        selector: "[data-tutorial='loadouts-work']",
        waitForMs: 4000,
        setup: async () => {
          await ensureTabIndex("[data-tutorial='loadouts-type-tabs']", 1)
        },
      },
      popover: { titleKey: "tutorial.characterLoadouts.work.title", descriptionKey: "tutorial.characterLoadouts.work.desc", side: "bottom" },
    },
    {
      id: "loadouts-work-bindings",
      target: { selector: "[data-tutorial='loadouts-work-bindings']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.characterLoadouts.bindings.title", descriptionKey: "tutorial.characterLoadouts.bindings.desc", side: "top" },
    },
  ],
}
