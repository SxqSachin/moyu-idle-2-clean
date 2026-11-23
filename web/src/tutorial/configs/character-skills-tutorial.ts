import type { TutorialConfig } from "../types"
import { ensureTabIndex, waitForSelector } from "../helpers"

const SKILLS_TUTORIAL_OPEN_RULE_EVENT = "character-skills:tutorial-open-rule"
const CHARACTER_TUTORIAL_OPEN_TAB_EVENT = "character:tutorial-open-tab"

async function openCharacterSkillsTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(CHARACTER_TUTORIAL_OPEN_TAB_EVENT, { detail: { tab: "skills" } }))
  await ensureTabIndex("[data-tutorial='character-tabs']", 2)
  await waitForSelector("[data-tutorial='skills-panel']", 6000)
  await ensureTabIndex("[data-tutorial='skills-tabs']", 0)
}

async function openDisabledSkillsTab(): Promise<void> {
  await openCharacterSkillsTab()
  await ensureTabIndex("[data-tutorial='skills-tabs']", 1)
}

async function openSkillRuleDialog(): Promise<void> {
  await openCharacterSkillsTab()
  window.dispatchEvent(new CustomEvent(SKILLS_TUTORIAL_OPEN_RULE_EVENT))
  await waitForSelector("[data-tutorial='skills-rule-dialog']", 5000)
}

export const characterSkillsTutorial: TutorialConfig = {
  id: "character-skills",
  nameKey: "tutorial.characterSkills.name",
  descriptionKey: "tutorial.characterSkills.description",
  icon: "game-icons:skills",
  version: 4,
  category: "core",
  order: 5,
  prerequisites: ["character"],
  steps: [
    {
      id: "skills-tab",
      target: {
        route: "/game/attributes",
        selector: "[data-tutorial='character-tabs']",
        waitForMs: 6000,
        setup: openCharacterSkillsTab,
      },
      popover: { titleKey: "tutorial.characterSkills.switchTab.title", descriptionKey: "tutorial.characterSkills.switchTab.desc", side: "bottom" },
    },
    {
      id: "skills-panel",
      target: { selector: "[data-tutorial='skills-panel']", waitForMs: 6000, setup: openCharacterSkillsTab },
      popover: { titleKey: "tutorial.characterSkills.enabledTab.title", descriptionKey: "tutorial.characterSkills.enabledTab.desc", side: "bottom" },
    },
    {
      id: "skills-slot-rule",
      target: { selector: "[data-tutorial='skills-slot-rule']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.characterSkills.slotRule.title", descriptionKey: "tutorial.characterSkills.slotRule.desc", side: "bottom" },
    },
    {
      id: "skills-enabled-list",
      target: { selector: "[data-tutorial='skills-enabled-list'], [data-tutorial='skills-tabs']", waitForMs: 5000 },
      popover: { titleKey: "tutorial.characterSkills.enabledList.title", descriptionKey: "tutorial.characterSkills.enabledList.desc", side: "bottom" },
    },
    {
      id: "skills-drag-order",
      target: { selector: "[data-tutorial='skills-drag-handle'], [data-tutorial='skills-enabled-card'], [data-tutorial='skills-enabled-list']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.characterSkills.dragOrder.title", descriptionKey: "tutorial.characterSkills.dragOrder.desc", side: "bottom" },
    },
    {
      id: "skills-rule-button",
      target: { selector: "[data-tutorial='skills-rule-button'], [data-tutorial='skills-enabled-card'], [data-tutorial='skills-enabled-list']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.characterSkills.ruleButton.title", descriptionKey: "tutorial.characterSkills.ruleButton.desc", side: "bottom" },
    },
    {
      id: "skills-disable-button",
      target: { selector: "[data-tutorial='skills-disable-button'], [data-tutorial='skills-enabled-card'], [data-tutorial='skills-enabled-list']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.characterSkills.disableButton.title", descriptionKey: "tutorial.characterSkills.disableButton.desc", side: "bottom" },
    },
    {
      id: "skills-disabled-tab",
      target: { selector: "[data-tutorial='skills-disabled-list'], [data-tutorial='skills-tabs']", waitForMs: 5000, setup: openDisabledSkillsTab },
      popover: { titleKey: "tutorial.characterSkills.disabledTab.title", descriptionKey: "tutorial.characterSkills.disabledTab.desc", side: "bottom" },
    },
    {
      id: "skills-requirements",
      target: { selector: "[data-tutorial='skills-requirements'], [data-tutorial='skills-disabled-card'], [data-tutorial='skills-disabled-list']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.characterSkills.requirements.title", descriptionKey: "tutorial.characterSkills.requirements.desc", side: "bottom" },
    },
    {
      id: "skills-enable-button",
      target: { selector: "[data-tutorial='skills-enable-button'], [data-tutorial='skills-disabled-card'], [data-tutorial='skills-disabled-list']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.characterSkills.enableButton.title", descriptionKey: "tutorial.characterSkills.enableButton.desc", side: "bottom" },
    },
    {
      id: "skills-rule-dialog",
      target: { selector: "[data-tutorial='skills-rule-dialog']", waitForMs: 5000, optional: true, setup: openSkillRuleDialog },
      popover: { titleKey: "tutorial.characterSkills.ruleDialog.title", descriptionKey: "tutorial.characterSkills.ruleDialog.desc", side: "left" },
    },
    {
      id: "skills-rule-editor",
      target: { selector: "[data-tutorial='skills-rule-editor'], [data-tutorial='skills-rule-dialog']", waitForMs: 5000, optional: true },
      popover: { titleKey: "tutorial.characterSkills.ruleEditor.title", descriptionKey: "tutorial.characterSkills.ruleEditor.desc", side: "left" },
    },
  ],
}
