import type { TutorialConfig } from "../types"
import { switchCombatTab, waitForSelector } from "../helpers"

const COMBAT_TUTORIAL_OPEN_CREATE_ROOM_EVENT = "combat:tutorial-open-create-room"
const COMBAT_TUTORIAL_CLOSE_DIALOGS_EVENT = "combat:tutorial-close-dialogs"

async function closeCombatDialogs(): Promise<void> {
  window.dispatchEvent(new CustomEvent(COMBAT_TUTORIAL_CLOSE_DIALOGS_EVENT))
}

async function openCombatAreaTab(): Promise<void> {
  await closeCombatDialogs()
  await switchCombatTab(0)
}

async function openCombatCreateRoom(): Promise<void> {
  await openCombatAreaTab()
  window.dispatchEvent(new CustomEvent(COMBAT_TUTORIAL_OPEN_CREATE_ROOM_EVENT))
  await waitForSelector("[data-tutorial='combat-create-room-dialog']", 5000)
}

export const combatTutorial: TutorialConfig = {
  id: "combat",
  nameKey: "tutorial.combat.name",
  descriptionKey: "tutorial.combat.description",
  icon: "game-icons:crossed-swords",
  version: 6,
  category: "progression",
  order: 3,
  prerequisites: ["character"],
  steps: [
    {
      id: "combat-page",
      target: { route: "/game/combat", selector: "[data-tutorial='combat-page']", waitForMs: 6000, setup: openCombatAreaTab },
      popover: { titleKey: "tutorial.combat.page.title", descriptionKey: "tutorial.combat.page.desc", side: "bottom" },
    },
    {
      id: "combat-tabs",
      target: { selector: "[data-tutorial='combat-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.combat.tabs.title", descriptionKey: "tutorial.combat.tabs.desc", side: "bottom" },
    },
    {
      id: "combat-area",
      target: {
        selector: "[data-tutorial='combat-tab-area']",
        waitForMs: 6000,
        optional: true,
        setup: openCombatAreaTab,
      },
      popover: { titleKey: "tutorial.combat.area.title", descriptionKey: "tutorial.combat.area.desc", side: "top" },
    },
    {
      id: "combat-create-room-basic",
      target: {
        selector: "[data-tutorial='combat-create-room-basic'], [data-tutorial='combat-create-room-dialog']",
        waitForMs: 5000,
        optional: true,
        setup: openCombatCreateRoom,
      },
      popover: { titleKey: "tutorial.combat.createRoomBasic.title", descriptionKey: "tutorial.combat.createRoomBasic.desc", side: "left" },
    },
    {
      id: "combat-create-room-difficulty",
      target: {
        selector: "[data-tutorial='combat-create-room-difficulty'], [data-tutorial='combat-create-room-area']",
        waitForMs: 5000,
        optional: true,
      },
      popover: { titleKey: "tutorial.combat.createRoomDifficulty.title", descriptionKey: "tutorial.combat.createRoomDifficulty.desc", side: "left" },
    },
    {
      id: "combat-create-room-options",
      target: {
        selector: "[data-tutorial='combat-create-room-position'], [data-tutorial='combat-create-room-options']",
        waitForMs: 5000,
        optional: true,
      },
      popover: { titleKey: "tutorial.combat.createRoomOptions.title", descriptionKey: "tutorial.combat.createRoomOptions.desc", side: "left" },
    },
    {
      id: "combat-rooms",
      target: {
        selector: "[data-tutorial='combat-tab-rooms']",
        waitForMs: 6000,
        optional: true,
        setup: async () => {
          await closeCombatDialogs()
          await switchCombatTab(1)
        },
      },
      popover: { titleKey: "tutorial.combat.rooms.title", descriptionKey: "tutorial.combat.rooms.desc", side: "top" },
    },
    {
      id: "combat-mine",
      target: {
        selector: "[data-tutorial='combat-tab-mine']",
        waitForMs: 6000,
        optional: true,
        setup: async () => {
          await closeCombatDialogs()
          await switchCombatTab(2)
        },
      },
      popover: { titleKey: "tutorial.combat.mine.title", descriptionKey: "tutorial.combat.mine.desc", side: "top" },
    },
    {
      id: "combat-battle",
      target: {
        selector: "[data-tutorial='combat-tab-battle']",
        waitForMs: 6000,
        optional: true,
        setup: async () => {
          await closeCombatDialogs()
          await switchCombatTab(3)
        },
      },
      popover: { titleKey: "tutorial.combat.battle.title", descriptionKey: "tutorial.combat.battle.desc", side: "top" },
    },
    {
      id: "combat-help",
      target: {
        selector: "[data-tutorial='combat-tab-help']",
        waitForMs: 6000,
        optional: true,
        setup: async () => {
          await closeCombatDialogs()
          await switchCombatTab(4)
        },
      },
      popover: { titleKey: "tutorial.combat.help.title", descriptionKey: "tutorial.combat.help.desc", side: "top" },
    },
  ],
}
