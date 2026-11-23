import type { TutorialConfig } from "../types"
import { ensureActionDetailDialogOpen, navigateToAvailableActionPanel, switchActionDetailTab } from "../helpers"

export const actionsTutorial: TutorialConfig = {
  id: "actions",
  nameKey: "tutorial.actions.name",
  descriptionKey: "tutorial.actions.description",
  icon: "game-icons:running-shoe",
  version: 5,
  category: "core",
  order: 2,
  steps: [
    {
      id: "action-panel",
      target: {
        selector: "[data-tutorial='action-panel']",
        waitForMs: 12000,
        setup: async () => {
          await navigateToAvailableActionPanel("collecting")
        },
      },
      popover: { titleKey: "tutorial.actions.actionPanel.title", descriptionKey: "tutorial.actions.actionPanel.desc", side: "bottom" },
    },
    {
      id: "action-tabs",
      target: { selector: "[data-tutorial='action-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.actions.actionTabs.title", descriptionKey: "tutorial.actions.actionTabs.desc", side: "bottom" },
    },
    {
      id: "action-cards",
      target: { selector: "[data-tutorial='action-cards'], .action-card", waitForMs: 6000 },
      popover: { titleKey: "tutorial.actions.actionCards.title", descriptionKey: "tutorial.actions.actionCards.desc", side: "top" },
    },
    {
      id: "action-detail",
      target: {
        selector: "[data-tutorial='action-detail-tabs']",
        waitForMs: 10000,
        setup: async () => {
          await ensureActionDetailDialogOpen()
        },
      },
      popover: { titleKey: "tutorial.actions.detailDialog.title", descriptionKey: "tutorial.actions.detailDialog.desc", side: "bottom" },
    },
    {
      id: "action-requirements",
      target: { selector: "[data-tutorial='action-detail-costs'], [data-tutorial='action-detail-overview']", optional: true },
      popover: { titleKey: "tutorial.actions.costs.title", descriptionKey: "tutorial.actions.costs.desc", side: "bottom" },
    },
    {
      id: "action-rewards",
      target: { selector: "[data-tutorial='action-detail-rewards'], [data-tutorial='action-detail-overview']", optional: true },
      popover: { titleKey: "tutorial.actions.rewards.title", descriptionKey: "tutorial.actions.rewards.desc", side: "top" },
    },
    {
      id: "action-submit",
      target: { selector: "[data-tutorial='action-submit-buttons']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.actions.enqueue.title", descriptionKey: "tutorial.actions.enqueue.desc", side: "top" },
    },
    {
      id: "action-quick-arrange",
      target: {
        selector: ".action-detail-quick-arrange-pane, [data-tutorial='action-detail-tabs']",
        setup: async () => {
          await ensureActionDetailDialogOpen()
          await switchActionDetailTab(1)
        },
      },
      popover: { titleKey: "tutorial.actions.quickArrange.title", descriptionKey: "tutorial.actions.quickArrange.desc", side: "bottom" },
    },
    {
      id: "action-set-count",
      target: {
        selector: "[data-tutorial='action-detail-count']",
        waitForMs: 6000,
        setup: async () => {
          await ensureActionDetailDialogOpen()
          await switchActionDetailTab(0)
        },
      },
      interactive: true,
      popover: { titleKey: "tutorial.actions.setCount.title", descriptionKey: "tutorial.actions.setCount.desc", side: "top" },
    },
    {
      id: "action-execute-now",
      target: {
        selector: "[data-tutorial='action-detail-run-now']",
        waitForMs: 6000,
        setup: async () => {
          await ensureActionDetailDialogOpen()
          await switchActionDetailTab(0)
        },
      },
      interactive: true,
      advanceOnTargetClick: true,
      popover: { titleKey: "tutorial.actions.executeNow.title", descriptionKey: "tutorial.actions.executeNow.desc", side: "top" },
    },
    {
      id: "action-home-queue",
      target: { route: "/game/home", selector: "[data-tutorial='home-queues']", waitForMs: 8000, optional: true },
      popover: { titleKey: "tutorial.actions.homeQueue.title", descriptionKey: "tutorial.actions.homeQueue.desc", side: "bottom" },
    },
    {
      id: "action-offline",
      target: { selector: "[data-tutorial='queue-lane'], [data-tutorial='home-queues']", waitForMs: 4000, optional: true },
      popover: { titleKey: "tutorial.actions.offline.title", descriptionKey: "tutorial.actions.offline.desc", side: "top" },
    },
  ],
}
