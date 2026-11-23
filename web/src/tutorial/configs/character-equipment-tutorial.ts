import type { TutorialConfig } from "../types"
import { ensureTabIndex } from "../helpers"

const CHARACTER_TUTORIAL_OPEN_TAB_EVENT = "character:tutorial-open-tab"

async function openCharacterEquipmentTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_OPEN_TAB_EVENT, { detail: { tab: "equipment" } }))
  await ensureTabIndex("[data-tutorial='character-tabs']", 1)
}

export const characterEquipmentTutorial: TutorialConfig = {
  id: "character-equipment",
  nameKey: "tutorial.characterEquipment.name",
  descriptionKey: "tutorial.characterEquipment.description",
  icon: "game-icons:chest-armor",
  version: 4,
  category: "core",
  order: 4,
  prerequisites: ["character"],
  steps: [
    {
      id: "equipment-tab",
      target: {
        route: "/game/attributes",
        selector: "[data-tutorial='character-tabs']",
        waitForMs: 6000,
        setup: openCharacterEquipmentTab,
      },
      popover: { titleKey: "tutorial.characterEquipment.switchTab.title", descriptionKey: "tutorial.characterEquipment.switchTab.desc", side: "bottom" },
    },
    {
      id: "equipment-panel",
      target: { selector: "[data-tutorial='equipment-panel']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.characterEquipment.grid.title", descriptionKey: "tutorial.characterEquipment.grid.desc", side: "bottom" },
    },
    {
      id: "equipment-type-tabs",
      target: { selector: "[data-tutorial='equipment-type-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterEquipment.workBattleTabs.title", descriptionKey: "tutorial.characterEquipment.workBattleTabs.desc", side: "bottom" },
    },
    {
      id: "equipment-main-slots",
      target: { selector: "[data-tutorial='equipment-main-slots']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterEquipment.mainSlots.title", descriptionKey: "tutorial.characterEquipment.mainSlots.desc", side: "bottom" },
    },
    {
      id: "equipment-accessory-slots",
      target: { selector: "[data-tutorial='equipment-accessory-slots']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.characterEquipment.accessoryDivider.title", descriptionKey: "tutorial.characterEquipment.accessoryDivider.desc", side: "top" },
    },
  ],
}
