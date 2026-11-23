import type { TutorialConfig } from "../types"
import { ensureActionDetailDialogOpen, navigateToAvailableActionPanel } from "../helpers"

export const onboardingTutorial: TutorialConfig = {
  id: "onboarding",
  nameKey: "tutorial.onboarding.name",
  descriptionKey: "tutorial.onboarding.description",
  icon: "material-symbols:explore-outline-rounded",
  version: 7,
  category: "core",
  order: 0,
  steps: [
    {
      id: "onboarding-welcome",
      target: { route: "/game/home" },
      popover: { titleKey: "tutorial.onboarding.welcome.title", descriptionKey: "tutorial.onboarding.welcome.desc", side: "over" },
    },
    {
      id: "onboarding-home",
      target: { route: "/game/home", selector: "[data-tutorial='home-queues']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.home.title", descriptionKey: "tutorial.onboarding.home.desc", side: "bottom" },
    },
    {
      id: "onboarding-action-page",
      target: {
        selector: "[data-tutorial='action-panel']",
        waitForMs: 12000,
        setup: async () => {
          await navigateToAvailableActionPanel("collecting")
        },
      },
      popover: { titleKey: "tutorial.onboarding.actionPage.title", descriptionKey: "tutorial.onboarding.actionPage.desc", side: "bottom" },
    },
    {
      id: "onboarding-action-detail",
      target: {
        selector: "[data-tutorial='action-detail-tabs']",
        waitForMs: 10000,
        setup: async () => {
          await ensureActionDetailDialogOpen()
        },
      },
      popover: { titleKey: "tutorial.onboarding.actionDetail.title", descriptionKey: "tutorial.onboarding.actionDetail.desc", side: "bottom" },
    },
    {
      id: "onboarding-warehouse",
      target: { route: "/game/warehouse", selector: "[data-tutorial='warehouse-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.warehouse.title", descriptionKey: "tutorial.onboarding.warehouse.desc", side: "bottom" },
    },
    {
      id: "onboarding-details",
      target: { route: "/game/panel/dictionary", selector: ".page-shell [data-tutorial='item-browser']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.details.title", descriptionKey: "tutorial.onboarding.details.desc", side: "top" },
      mobile: {
        popover: { descriptionKey: "tutorial.onboarding.details.descMobile" },
      },
    },
    {
      id: "onboarding-character",
      target: { route: "/game/attributes", selector: "[data-tutorial='character-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.character.title", descriptionKey: "tutorial.onboarding.character.desc", side: "bottom" },
    },
    {
      id: "onboarding-next-goal",
      target: { route: "/game/tasks", selector: "[data-tutorial='tasks-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.nextGoal.title", descriptionKey: "tutorial.onboarding.nextGoal.desc", side: "bottom" },
    },
    {
      id: "onboarding-achievements",
      target: { route: "/game/achievements", selector: "[data-tutorial='achievements-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.achievements.title", descriptionKey: "tutorial.onboarding.achievements.desc", side: "bottom" },
    },
    {
      id: "onboarding-building",
      target: { route: "/game/building", selector: "[data-tutorial='building-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.building.title", descriptionKey: "tutorial.onboarding.building.desc", side: "bottom" },
    },
    {
      id: "onboarding-trade",
      target: { route: "/game/shop", selector: "[data-tutorial='shop-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.trade.title", descriptionKey: "tutorial.onboarding.trade.desc", side: "bottom" },
    },
    {
      id: "onboarding-combat",
      target: { route: "/game/combat", selector: "[data-tutorial='combat-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.combat.title", descriptionKey: "tutorial.onboarding.combat.desc", side: "bottom" },
    },
    {
      id: "onboarding-enhance",
      target: { route: "/game/enhance", selector: "[data-tutorial='enhance-target'], [data-tutorial='enhance-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.enhance.title", descriptionKey: "tutorial.onboarding.enhance.desc", side: "bottom" },
    },
    {
      id: "onboarding-affix",
      target: {
        route: "/game/affix",
        selector: "[data-tutorial='affix-tabs'] .el-tabs__item.is-active, [data-tutorial='affix-tabs'] .el-tabs__item, [data-tutorial='affix-target']",
        waitForMs: 8000,
      },
      popover: { titleKey: "tutorial.onboarding.affix.title", descriptionKey: "tutorial.onboarding.affix.desc", side: "bottom" },
    },
    {
      id: "onboarding-alchemy",
      target: { route: "/game/alchemy", selector: "[data-tutorial='alchemy-tabs'], [data-tutorial='alchemy-page']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.alchemy.title", descriptionKey: "tutorial.onboarding.alchemy.desc", side: "bottom" },
    },
    {
      id: "onboarding-guide",
      target: { route: "/game/panel/gameguide", selector: "[data-tutorial='gameguide-panel']", waitForMs: 8000 },
      popover: { titleKey: "tutorial.onboarding.guide.title", descriptionKey: "tutorial.onboarding.guide.desc", side: "bottom" },
    },
  ],
}
