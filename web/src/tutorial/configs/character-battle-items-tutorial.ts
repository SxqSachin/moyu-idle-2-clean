import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

const CHARACTER_TUTORIAL_OPEN_TAB_EVENT = "character:tutorial-open-tab"

async function openCharacterBattleItemsTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_OPEN_TAB_EVENT, { detail: { tab: "battle-items" } }))
  await ensureTabIndex("[data-tutorial='character-tabs']", 3)
}

export const characterBattleItemsTutorial: TutorialConfig = {
  id: "character-battle-items",
  nameKey: "tutorial.characterBattleItems.name",
  descriptionKey: "tutorial.characterBattleItems.description",
  icon: "game-icons:potion-ball",
  version: 4,
  category: "core",
  order: 6,
  prerequisites: ["character"],
  steps: [
    {
      id: "battle-items-tab",
      target: {
        route: "/game/attributes",
        selector: "[data-tutorial='character-tabs']",
        waitForMs: 6000,
        setup: openCharacterBattleItemsTab,
      },
      popover: { titleKey: "tutorial.characterBattleItems.switchTab.title", descriptionKey: "tutorial.characterBattleItems.switchTab.desc", side: "bottom" },
    },
    {
      id: "battle-items-content",
      target: { selector: "[data-tutorial='battle-items-panel']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterBattleItems.content.title", descriptionKey: "tutorial.characterBattleItems.content.desc", side: "bottom" },
    },
    {
      id: "battle-items-capacity",
      target: { selector: "[data-tutorial='battle-items-capacity']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterBattleItems.capacity.title", descriptionKey: "tutorial.characterBattleItems.capacity.desc", side: "bottom" },
    },
    {
      id: "battle-items-slots",
      target: { selector: "[data-tutorial='battle-items-slots']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterBattleItems.slots.title", descriptionKey: "tutorial.characterBattleItems.slots.desc", side: "top" },
    },
  ],
}
