import type { TutorialConfig } from "../types"
import { ensureTabIndex, waitForSelector } from "../helpers"

const AUCTION_TUTORIAL_OPEN_CREATE_DIALOG_EVENT = "auction:tutorial-open-create-dialog"
const AUCTION_TUTORIAL_CLOSE_DIALOGS_EVENT = "auction:tutorial-close-dialogs"

async function closeAuctionDialogs(): Promise<void> {
  window.dispatchEvent(new CustomEvent(AUCTION_TUTORIAL_CLOSE_DIALOGS_EVENT))
}

async function openAuctionPublicTab(): Promise<void> {
  await closeAuctionDialogs()
  await ensureTabIndex("[data-tutorial='auction-tabs']", 0)
}

async function openAuctionCreateDialog(): Promise<void> {
  await closeAuctionDialogs()
  window.dispatchEvent(new CustomEvent(AUCTION_TUTORIAL_OPEN_CREATE_DIALOG_EVENT))
  await waitForSelector("[data-tutorial='auction-create-item']", 5000)
}

export const auctionTutorial: TutorialConfig = {
  id: "auction",
  nameKey: "tutorial.auction.name",
  descriptionKey: "tutorial.auction.description",
  icon: "mingcute:auction-line",
  version: 6,
  category: "economy",
  order: 4,
  prerequisites: ["warehouse"],
  steps: [
    {
      id: "auction-page",
      target: { route: "/game/auction", selector: "[data-tutorial='auction-page']", waitForMs: 6000, setup: openAuctionPublicTab },
      popover: { titleKey: "tutorial.auction.header.title", descriptionKey: "tutorial.auction.header.desc", side: "bottom" },
    },
    {
      id: "auction-filters",
      target: { selector: "[data-tutorial='auction-filters']", waitForMs: 6000, setup: closeAuctionDialogs },
      popover: { titleKey: "tutorial.auction.filters.title", descriptionKey: "tutorial.auction.filters.desc", side: "bottom" },
    },
    {
      id: "auction-create-item",
      target: {
        selector: "[data-tutorial='auction-create-item']",
        waitForMs: 5000,
        optional: true,
        setup: openAuctionCreateDialog,
      },
      popover: { titleKey: "tutorial.auction.createItem.title", descriptionKey: "tutorial.auction.createItem.desc", side: "left" },
    },
    {
      id: "auction-create-pricing",
      target: {
        selector: "[data-tutorial='auction-create-pricing']",
        waitForMs: 5000,
        optional: true,
      },
      popover: { titleKey: "tutorial.auction.createPricing.title", descriptionKey: "tutorial.auction.createPricing.desc", side: "left" },
    },
    {
      id: "auction-create-buyout",
      target: {
        selector: "[data-tutorial='auction-create-buyout'], [data-tutorial='auction-create-settlement']",
        waitForMs: 5000,
        optional: true,
      },
      popover: { titleKey: "tutorial.auction.createBuyout.title", descriptionKey: "tutorial.auction.createBuyout.desc", side: "left" },
    },
    {
      id: "auction-list",
      target: { selector: "[data-tutorial='auction-list'], [data-tutorial='auction-tabs']", waitForMs: 6000, optional: true, setup: closeAuctionDialogs },
      popover: { titleKey: "tutorial.auction.publicTab.title", descriptionKey: "tutorial.auction.publicTab.desc", side: "bottom" },
    },
    {
      id: "auction-mine",
      target: {
        selector: "[data-tutorial='auction-tabs']",
        waitForMs: 4000,
        setup: async () => {
          await closeAuctionDialogs()
          await ensureTabIndex("[data-tutorial='auction-tabs']", 1)
        },
      },
      popover: { titleKey: "tutorial.auction.myListings.title", descriptionKey: "tutorial.auction.myListings.desc", side: "bottom" },
    },
  ],
}
