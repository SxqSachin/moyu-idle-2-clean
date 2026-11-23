import type { TutorialConfig } from "../types"
import { ensureTabIndex, waitForSelector } from "../helpers"

const MARKET_TUTORIAL_OPEN_LIST_DIALOG_EVENT = "market:tutorial-open-list-dialog"
const MARKET_TUTORIAL_OPEN_BUY_DIALOG_EVENT = "market:tutorial-open-buy-dialog"
const MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT = "market:tutorial-close-dialogs"

async function openMarketCatalogTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT))
  await ensureTabIndex("[data-tutorial='market-tabs']", 0)
}

async function openMarketTradeTab(): Promise<void> {
  window.dispatchEvent(new CustomEvent(MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT))
  await ensureTabIndex("[data-tutorial='market-tabs']", 1)
}

async function openMarketListDialog(): Promise<void> {
  await openMarketTradeTab()
  window.dispatchEvent(new CustomEvent(MARKET_TUTORIAL_OPEN_LIST_DIALOG_EVENT))
  await waitForSelector("[data-tutorial='market-list-dialog']", 5000)
}

async function openMarketBuyDialog(): Promise<void> {
  await openMarketTradeTab()
  window.dispatchEvent(new CustomEvent(MARKET_TUTORIAL_OPEN_BUY_DIALOG_EVENT))
  await waitForSelector("[data-tutorial='market-buy-dialog']", 5000)
}

export const marketTutorial: TutorialConfig = {
  id: "market",
  nameKey: "tutorial.market.name",
  descriptionKey: "tutorial.market.description",
  icon: "game-icons:shop",
  version: 6,
  category: "economy",
  order: 3,
  prerequisites: ["warehouse"],
  steps: [
    {
      id: "market-page",
      target: { route: "/game/market", selector: "[data-tutorial='market-page']", waitForMs: 6000, setup: openMarketCatalogTab },
      popover: { titleKey: "tutorial.market.catalog.title", descriptionKey: "tutorial.market.catalog.desc", side: "bottom" },
    },
    {
      id: "market-catalog",
      target: { selector: "[data-tutorial='market-catalog']", waitForMs: 6000 },
      popover: { titleKey: "tutorial.market.browse.title", descriptionKey: "tutorial.market.browse.desc", side: "bottom" },
    },
    {
      id: "market-trade",
      target: {
        selector: "[data-tutorial='market-trade'], [data-tutorial='market-tabs']",
        waitForMs: 4000,
        setup: openMarketTradeTab,
      },
      popover: { titleKey: "tutorial.market.trade.title", descriptionKey: "tutorial.market.trade.desc", side: "bottom" },
    },
    {
      id: "market-list-dialog",
      target: {
        selector: "[data-tutorial='market-list-dialog']",
        waitForMs: 5000,
        setup: openMarketListDialog,
      },
      popover: { titleKey: "tutorial.market.listDialog.title", descriptionKey: "tutorial.market.listDialog.desc", side: "left" },
    },
    {
      id: "market-buy-dialog",
      target: {
        selector: "[data-tutorial='market-buy-dialog']",
        waitForMs: 5000,
        setup: openMarketBuyDialog,
      },
      popover: { titleKey: "tutorial.market.buyDialog.title", descriptionKey: "tutorial.market.buyDialog.desc", side: "left" },
    },
    {
      id: "market-history",
      target: {
        selector: "[data-tutorial='market-history'], [data-tutorial='market-tabs']",
        waitForMs: 4000,
        setup: async () => {
          window.dispatchEvent(new CustomEvent(MARKET_TUTORIAL_CLOSE_DIALOGS_EVENT))
          await ensureTabIndex("[data-tutorial='market-tabs']", 2)
        },
      },
      popover: { titleKey: "tutorial.market.history.title", descriptionKey: "tutorial.market.history.desc", side: "bottom" },
    },
  ],
}
