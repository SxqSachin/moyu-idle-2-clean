import type { TutorialConfig } from "../types"
import { hasOwnedKitties } from "../helpers"
import { ensureTabIndex, waitForSelector } from "../helpers"

const CHARACTER_TUTORIAL_RESET_EVENT = "character:tutorial-reset"
const CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT = "character:tutorial-open-allocate"

async function resetCharacterTutorial(): Promise<void> {
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_RESET_EVENT))
  await ensureTabIndex("[data-tutorial='character-tabs']", 0)
  await waitForSelector("[data-tutorial='attribute-stats']", 5000)
}

async function showBattleAttributeTab(): Promise<void> {
  await resetCharacterTutorial()
  await ensureTabIndex("[data-tutorial='attribute-group-tabs']", 1)
}

async function openAttributeAllocateDialog(): Promise<void> {
  await showBattleAttributeTab()
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT))
  await waitForSelector("[data-tutorial='attribute-allocate-dialog']", 5000)
}

export const characterTutorial: TutorialConfig = {
  id: "character",
  nameKey: "tutorial.character.name",
  descriptionKey: "tutorial.character.description",
  icon: "game-icons:character",
  version: 4,
  category: "core",
  order: 1,
  steps: [
    {
      id: "character-page",
      target: { route: "/game/attributes", selector: "[data-tutorial='character-page']", waitForMs: 6000, setup: resetCharacterTutorial },
      popover: { titleKey: "tutorial.character.page.title", descriptionKey: "tutorial.character.page.desc", side: "bottom" },
    },
    {
      id: "role-selector",
      target: { selector: "[data-tutorial='role-selector']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.character.roleSelector.title", descriptionKey: "tutorial.character.roleSelector.desc", side: "right" },
      when: () => hasOwnedKitties(),
    },
    {
      id: "role-selector-no-kitty",
      target: { selector: "[data-tutorial='role-selector']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.character.roleSelectorNoKitty.title", descriptionKey: "tutorial.character.roleSelectorNoKitty.desc", side: "right" },
      when: () => !hasOwnedKitties(),
    },
    {
      id: "character-tabs",
      target: { selector: "[data-tutorial='character-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.character.tabs.title", descriptionKey: "tutorial.character.tabs.desc", side: "bottom" },
    },
    {
      id: "character-attribute-panel",
      target: { selector: "[data-tutorial='attribute-stats']", waitForMs: 6000, setup: resetCharacterTutorial },
      popover: { titleKey: "tutorial.character.attributePanel.title", descriptionKey: "tutorial.character.attributePanel.desc", side: "bottom" },
    },
    {
      id: "character-attribute-card",
      target: { selector: "[data-tutorial='attribute-primary-card'], [data-tutorial='attribute-stat-card']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.character.attributeCard.title", descriptionKey: "tutorial.character.attributeCard.desc", side: "bottom" },
    },
    {
      id: "character-battle-attributes",
      target: { selector: "[data-tutorial='battle-stats'], [data-tutorial='attribute-group-tabs']", waitForMs: 6000, setup: showBattleAttributeTab },
      popover: { titleKey: "tutorial.character.battleAttributes.title", descriptionKey: "tutorial.character.battleAttributes.desc", side: "bottom" },
    },
    {
      id: "character-allocate-button",
      target: { selector: "[data-tutorial='attribute-allocate-button']", waitForMs: 5000, optional: true, setup: showBattleAttributeTab },
      popover: { titleKey: "tutorial.character.allocateButton.title", descriptionKey: "tutorial.character.allocateButton.desc", side: "left" },
    },
    {
      id: "character-allocate-dialog",
      target: { selector: "[data-tutorial='attribute-allocate-dialog']", waitForMs: 5000, optional: true, setup: openAttributeAllocateDialog },
      popover: { titleKey: "tutorial.character.allocateDialog.title", descriptionKey: "tutorial.character.allocateDialog.desc", side: "left" },
    },
  ],
}
