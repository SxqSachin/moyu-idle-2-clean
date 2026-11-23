import type { TutorialConfig } from "../types"
import { ensureTabIndex, waitForSelector } from "../helpers"

const CHARACTER_TUTORIAL_RESET_EVENT = "character:tutorial-reset"
const CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT = "character:tutorial-open-allocate"

async function resetCharacterAttributesTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_RESET_EVENT))
  await ensureTabIndex("[data-tutorial='character-tabs']", 0)
  await waitForSelector("[data-tutorial='attribute-stats']", 5000)
}

async function showBattleAttributeTab(): Promise<void> {
  await resetCharacterAttributesTab()
  await ensureTabIndex("[data-tutorial='attribute-group-tabs']", 1)
}

async function openAttributeAllocateDialog(): Promise<void> {
  await showBattleAttributeTab()
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT))
  await waitForSelector("[data-tutorial='attribute-allocate-dialog']", 5000)
}

export const characterAttributesTutorial: TutorialConfig = {
  id: "character-attributes",
  nameKey: "tutorial.characterAttributes.name",
  descriptionKey: "tutorial.characterAttributes.description",
  icon: "game-icons:muscle-up",
  version: 5,
  category: "core",
  order: 3,
  prerequisites: ["character"],
  steps: [
    {
      id: "attributes-tab",
      target: {
        route: "/game/attributes",
        selector: "[data-tutorial='character-tabs']",
        waitForMs: 6000,
        setup: resetCharacterAttributesTab,
      },
      popover: { titleKey: "tutorial.characterAttributes.switchTab.title", descriptionKey: "tutorial.characterAttributes.switchTab.desc", side: "bottom" },
    },
    {
      id: "stats-overview",
      target: { selector: "[data-tutorial='attribute-stats']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.characterAttributes.statsOverview.title", descriptionKey: "tutorial.characterAttributes.statsOverview.desc", side: "bottom" },
    },
    {
      id: "attribute-card-detail",
      target: { selector: "[data-tutorial='attribute-primary-card'], [data-tutorial='attribute-stat-card']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.characterAttributes.attributeCard.title", descriptionKey: "tutorial.characterAttributes.attributeCard.desc", side: "bottom" },
    },
    {
      id: "growth-path",
      target: { selector: "[data-tutorial='attribute-stats'], [data-tutorial='character-tabs']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.characterAttributes.growthPath.title", descriptionKey: "tutorial.characterAttributes.growthPath.desc", side: "bottom" },
    },
    {
      id: "battle-stats",
      target: { selector: "[data-tutorial='battle-stats'], [data-tutorial='attribute-stats']", optional: true, setup: showBattleAttributeTab },
      popover: { titleKey: "tutorial.characterAttributes.basicAttack.title", descriptionKey: "tutorial.characterAttributes.basicAttack.desc", side: "bottom" },
    },
    {
      id: "allocate-bp",
      target: { selector: "[data-tutorial='attribute-allocate-button']", waitForMs: 5000, optional: true, setup: showBattleAttributeTab },
      popover: { titleKey: "tutorial.characterAttributes.allocateBp.title", descriptionKey: "tutorial.characterAttributes.allocateBp.desc", side: "left" },
    },
    {
      id: "allocate-bp-dialog",
      target: { selector: "[data-tutorial='attribute-allocate-dialog']", waitForMs: 5000, optional: true, setup: openAttributeAllocateDialog },
      popover: { titleKey: "tutorial.characterAttributes.allocateBpDialog.title", descriptionKey: "tutorial.characterAttributes.allocateBpDialog.desc", side: "left" },
    },
  ],
}
