import type { TutorialConfig } from "../types"
import { delay, waitForSelector } from "../helpers"

const WAREHOUSE_TUTORIAL_CLOSE_DETAIL_EVENT = "warehouse:tutorial-close-detail"

function visibleElement(selector: string): HTMLElement | null {
  const elements = Array.from(document.querySelectorAll<HTMLElement>(selector))
  return elements.find((element) => element.getClientRects().length > 0) || null
}

async function switchWarehouseTab(tab: "inventory" | "archived" | "log" | "collectibles"): Promise<void> {
  window.dispatchEvent(new CustomEvent(WAREHOUSE_TUTORIAL_CLOSE_DETAIL_EVENT))
  window.dispatchEvent(new CustomEvent("item-browser:tutorial-close-tools", { detail: { scope: "warehouse-inventory" } }))
  window.dispatchEvent(new CustomEvent("item-browser:tutorial-close-tools", { detail: { scope: "warehouse-archive" } }))
  window.dispatchEvent(new CustomEvent("warehouse:tutorial-switch-tab", { detail: { tab } }))
  await delay(250)
}

async function closeWarehouseItemDetail(): Promise<void> {
  window.dispatchEvent(new CustomEvent(WAREHOUSE_TUTORIAL_CLOSE_DETAIL_EVENT))
  await delay(150)
}

async function openWarehouseItemDetail(): Promise<void> {
  await switchWarehouseTab("inventory")
  window.dispatchEvent(new CustomEvent("warehouse:tutorial-open-detail"))
  await waitForSelector("[data-tutorial='warehouse-item-detail-dialog']", 5000)
}

async function openWarehouseToolMenu(): Promise<void> {
  await switchWarehouseTab("inventory")
  window.dispatchEvent(new CustomEvent("item-browser:tutorial-open-tools", { detail: { scope: "warehouse-inventory" } }))
  await delay(350)
}

async function ensureWarehouseToolMenuOpen(): Promise<void> {
  await openWarehouseToolMenu()
  if (visibleElement("[data-tutorial='warehouse-batch-archive'], [data-tutorial='warehouse-batch-recycle']")) return
  const trigger = document.querySelector<HTMLElement>("[data-tutorial='warehouse-items'] [data-tutorial='item-browser-tools-trigger']")
  trigger?.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, view: window }))
  await delay(350)
}

export const warehouseTutorial: TutorialConfig = {
  id: "warehouse",
  nameKey: "tutorial.warehouse.name",
  descriptionKey: "tutorial.warehouse.description",
  icon: "game-icons:warehouse",
  version: 10,
  category: "economy",
  order: 2,
  prerequisites: ["actions"],
  steps: [
    {
      id: "warehouse-page",
      target: { route: "/game/warehouse", selector: "[data-tutorial='warehouse-page']", waitForMs: 6000, setup: () => switchWarehouseTab("inventory") },
      popover: { titleKey: "tutorial.warehouse.page.title", descriptionKey: "tutorial.warehouse.page.desc", side: "bottom" },
    },
    {
      id: "warehouse-tabs",
      target: { selector: "[data-tutorial='warehouse-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.warehouse.inventory.title", descriptionKey: "tutorial.warehouse.inventory.desc", side: "bottom" },
    },
    {
      id: "warehouse-browser",
      target: { selector: "[data-tutorial='warehouse-items'], [data-tutorial='item-browser']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.warehouse.browser.title", descriptionKey: "tutorial.warehouse.browser.desc", side: "bottom" },
    },
    {
      id: "warehouse-item-detail",
      target: {
        selector: "[data-tutorial='warehouse-item-detail-dialog']",
        waitForMs: 5000,
        optional: true,
        setup: openWarehouseItemDetail,
      },
      popover: { titleKey: "tutorial.warehouse.itemDetail.title", descriptionKey: "tutorial.warehouse.itemDetail.desc", side: "right" },
    },
    {
      id: "warehouse-filtering",
      target: { selector: "[data-tutorial='item-browser-toolbar']", waitForMs: 4000, setup: closeWarehouseItemDetail },
      popover: { titleKey: "tutorial.warehouse.filtering.title", descriptionKey: "tutorial.warehouse.filtering.desc", side: "bottom" },
    },
    {
      id: "warehouse-tools",
      target: {
        selector: "[data-tutorial='warehouse-batch-archive'], [data-tutorial='warehouse-batch-recycle']",
        waitForMs: 4000,
        setup: ensureWarehouseToolMenuOpen,
      },
      popover: { titleKey: "tutorial.warehouse.tools.title", descriptionKey: "tutorial.warehouse.tools.desc", side: "left" },
    },
    {
      id: "warehouse-batch-archive",
      target: {
        selector: "[data-tutorial='warehouse-batch-archive']",
        waitForMs: 4000,
        setup: ensureWarehouseToolMenuOpen,
      },
      popover: { titleKey: "tutorial.warehouse.batchArchive.title", descriptionKey: "tutorial.warehouse.batchArchive.desc", side: "left" },
    },
    {
      id: "warehouse-batch-recycle",
      target: {
        selector: "[data-tutorial='warehouse-batch-recycle']",
        waitForMs: 4000,
        setup: ensureWarehouseToolMenuOpen,
      },
      popover: { titleKey: "tutorial.warehouse.batchRecycle.title", descriptionKey: "tutorial.warehouse.batchRecycle.desc", side: "left" },
    },
    {
      id: "warehouse-archive",
      target: {
        selector: "[data-tutorial='warehouse-archive'], [data-tutorial='warehouse-tabs']",
        waitForMs: 5000,
        setup: async () => {
          await switchWarehouseTab("archived")
        },
      },
      popover: { titleKey: "tutorial.warehouse.archive.title", descriptionKey: "tutorial.warehouse.archive.desc", side: "bottom" },
    },
    {
      id: "warehouse-log",
      target: {
        selector: "[data-tutorial='warehouse-log'], [data-tutorial='warehouse-tabs']",
        waitForMs: 5000,
        setup: async () => {
          await switchWarehouseTab("log")
        },
      },
      popover: { titleKey: "tutorial.warehouse.log.title", descriptionKey: "tutorial.warehouse.log.desc", side: "bottom" },
    },
  ],
}
