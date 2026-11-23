import type { TutorialConfig } from "../types"

/**
 * Basic interaction primer, anchored on the in-game Dictionary
 * (/game/panel/dictionary). The dictionary lists real items / skills / affixes
 * that expose the floating detail view, so the user can practice the gesture on
 * actual elements: hover on desktop, long-press on mobile (the mobile override
 * swaps the wording). Targets use stable `.page-shell` selectors plus the
 * existing `item-browser` hook — no business component is modified.
 */
export const interactionsTutorial: TutorialConfig = {
  id: "interactions",
  nameKey: "tutorial.interactions.name",
  descriptionKey: "tutorial.interactions.description",
  icon: "game-icons:click",
  version: 2,
  category: "core",
  order: 0,
  steps: [
    {
      id: "interactions-intro",
      target: {
        route: "/game/panel/dictionary",
        selector: ".page-shell .el-card__header",
        waitForMs: 8000,
      },
      popover: {
        titleKey: "tutorial.interactions.intro.title",
        descriptionKey: "tutorial.interactions.intro.desc",
        side: "bottom",
      },
    },
    {
      id: "interactions-view",
      target: {
        selector: ".page-shell [data-tutorial='item-browser']",
        waitForMs: 6000,
      },
      popover: {
        titleKey: "tutorial.interactions.view.title",
        descriptionKey: "tutorial.interactions.view.desc",
        side: "top",
      },
      mobile: {
        popover: { descriptionKey: "tutorial.interactions.view.descMobile" },
      },
    },
    {
      id: "interactions-where",
      target: {
        selector: ".page-shell .el-tabs__nav",
        waitForMs: 6000,
      },
      popover: {
        titleKey: "tutorial.interactions.where.title",
        descriptionKey: "tutorial.interactions.where.desc",
        side: "bottom",
      },
    },
  ],
}
