import router from "@/router"
import type { TutorialConfig } from "../types"

type AlchemyTab = "convert" | "bench" | "inherit" | "transmute" | "smelt" | "skill-forge"

async function showAlchemyTab(tab: AlchemyTab) {
  await router.replace({ path: "/game/alchemy", query: { tab } })
}

function moduleTutorial(
  id: string,
  nameKey: string,
  descriptionKey: string,
  tab: AlchemyTab,
  steps: TutorialConfig["steps"]
): TutorialConfig {
  return {
    id,
    nameKey,
    descriptionKey,
    icon: "game-icons:chemical-drop",
    version: 7,
    category: "progression",
    order: 9,
    steps: [
      {
        id: `${id}-page`,
        target: { route: "/game/alchemy", selector: "[data-tutorial='alchemy-page']", setup: () => showAlchemyTab(tab), waitForMs: 6000 },
        popover: { titleKey: "tutorial.alchemy.page.title", descriptionKey: "tutorial.alchemy.page.desc", side: "bottom" },
      },
      ...steps,
    ],
  }
}

export const alchemyTutorial: TutorialConfig = {
  id: "alchemy",
  nameKey: "tutorial.alchemy.name",
  descriptionKey: "tutorial.alchemy.description",
  icon: "game-icons:chemical-drop",
  version: 7,
  category: "progression",
  order: 9,
  steps: [
    {
      id: "alchemy-page",
      target: { route: "/game/alchemy", selector: "[data-tutorial='alchemy-page']", setup: () => showAlchemyTab("convert"), waitForMs: 6000 },
      popover: { titleKey: "tutorial.alchemy.page.title", descriptionKey: "tutorial.alchemy.page.desc", side: "bottom" },
    },
    {
      id: "alchemy-tabs",
      target: { selector: "[data-tutorial='alchemy-tabs']", waitForMs: 4000 },
      popover: { titleKey: "tutorial.alchemy.tabs.title", descriptionKey: "tutorial.alchemy.tabs.desc", side: "bottom" },
    },
  ],
}

export const alchemyConvertTutorial = moduleTutorial(
  "alchemy-convert",
  "tutorial.alchemyConvert.name",
  "tutorial.alchemyConvert.description",
  "convert",
  [
    {
      id: "alchemy-convert",
      target: { selector: "[data-tutorial='alchemy-convert']", setup: () => showAlchemyTab("convert"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyConvert.overview.title", descriptionKey: "tutorial.alchemyConvert.overview.desc", side: "bottom" },
    },
    {
      id: "alchemy-convert-source",
      target: { selector: "[data-tutorial='alchemy-convert-source']", setup: () => showAlchemyTab("convert"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyConvert.source.title", descriptionKey: "tutorial.alchemyConvert.source.desc", side: "bottom" },
    },
    {
      id: "alchemy-convert-outcomes",
      target: { selector: "[data-tutorial='alchemy-convert-outcomes']", setup: () => showAlchemyTab("convert"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyConvert.outcomes.title", descriptionKey: "tutorial.alchemyConvert.outcomes.desc", side: "bottom" },
    },
    {
      id: "alchemy-convert-submit",
      target: { selector: "[data-tutorial='alchemy-convert-submit']", setup: () => showAlchemyTab("convert"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyConvert.submit.title", descriptionKey: "tutorial.alchemyConvert.submit.desc", side: "top" },
    },
  ]
)

export const alchemyBenchTutorial = moduleTutorial(
  "alchemy-bench",
  "tutorial.alchemyBench.name",
  "tutorial.alchemyBench.description",
  "bench",
  [
    {
      id: "alchemy-bench",
      target: { selector: "[data-tutorial='alchemy-bench']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.overview.title", descriptionKey: "tutorial.alchemyBench.overview.desc", side: "bottom" },
    },
    {
      id: "alchemy-bench-carrier",
      target: { selector: "[data-tutorial='alchemy-bench-carrier']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.carrier.title", descriptionKey: "tutorial.alchemyBench.carrier.desc", side: "bottom" },
    },
    {
      id: "alchemy-bench-medium",
      target: { selector: "[data-tutorial='alchemy-bench-medium']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.medium.title", descriptionKey: "tutorial.alchemyBench.medium.desc", side: "bottom" },
    },
    {
      id: "alchemy-bench-materials",
      target: { selector: "[data-tutorial='alchemy-bench-materials']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.materials.title", descriptionKey: "tutorial.alchemyBench.materials.desc", side: "bottom" },
    },
    {
      id: "alchemy-bench-details",
      target: { selector: "[data-tutorial='alchemy-bench-preview']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.details.title", descriptionKey: "tutorial.alchemyBench.details.desc", side: "bottom" },
    },
    {
      id: "alchemy-bench-preview",
      target: { selector: "[data-tutorial='alchemy-bench-preview']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.preview.title", descriptionKey: "tutorial.alchemyBench.preview.desc", side: "bottom" },
    },
    {
      id: "alchemy-bench-submit",
      target: { selector: "[data-tutorial='alchemy-bench-submit']", setup: () => showAlchemyTab("bench"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyBench.submit.title", descriptionKey: "tutorial.alchemyBench.submit.desc", side: "top" },
    },
  ]
)

export const alchemyInheritTutorial = moduleTutorial(
  "alchemy-inherit",
  "tutorial.alchemyInherit.name",
  "tutorial.alchemyInherit.description",
  "inherit",
  [
    {
      id: "alchemy-inherit",
      target: { selector: "[data-tutorial='alchemy-inherit']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.overview.title", descriptionKey: "tutorial.alchemyInherit.overview.desc", side: "bottom" },
    },
    {
      id: "alchemy-inherit-setup",
      target: { selector: "[data-tutorial='alchemy-inherit-setup']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.setup.title", descriptionKey: "tutorial.alchemyInherit.setup.desc", side: "bottom" },
    },
    {
      id: "alchemy-inherit-medium",
      target: { selector: "[data-tutorial='alchemy-inherit-medium']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.medium.title", descriptionKey: "tutorial.alchemyInherit.medium.desc", side: "bottom" },
    },
    {
      id: "alchemy-inherit-carrier",
      target: { selector: "[data-tutorial='alchemy-inherit-carrier']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.carrier.title", descriptionKey: "tutorial.alchemyInherit.carrier.desc", side: "bottom" },
    },
    {
      id: "alchemy-inherit-summary",
      target: { selector: "[data-tutorial='alchemy-inherit-summary']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.summary.title", descriptionKey: "tutorial.alchemyInherit.summary.desc", side: "bottom" },
    },
    {
      id: "alchemy-inherit-sources",
      target: { selector: "[data-tutorial='alchemy-inherit-sources']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.sources.title", descriptionKey: "tutorial.alchemyInherit.sources.desc", side: "bottom" },
    },
    {
      id: "alchemy-inherit-submit",
      target: { selector: "[data-tutorial='alchemy-inherit-submit']", setup: () => showAlchemyTab("inherit"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyInherit.submit.title", descriptionKey: "tutorial.alchemyInherit.submit.desc", side: "top" },
    },
  ]
)

export const alchemyTransmuteTutorial = moduleTutorial(
  "alchemy-transmute",
  "tutorial.alchemyTransmute.name",
  "tutorial.alchemyTransmute.description",
  "transmute",
  [
    {
      id: "alchemy-transmute",
      target: { selector: "[data-tutorial='alchemy-transmute']", setup: () => showAlchemyTab("transmute"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyTransmute.overview.title", descriptionKey: "tutorial.alchemyTransmute.overview.desc", side: "bottom" },
    },
    {
      id: "alchemy-transmute-target",
      target: { selector: "[data-tutorial='alchemy-transmute-target']", setup: () => showAlchemyTab("transmute"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyTransmute.target.title", descriptionKey: "tutorial.alchemyTransmute.target.desc", side: "bottom" },
    },
    {
      id: "alchemy-transmute-rewards",
      target: { selector: "[data-tutorial='alchemy-transmute-rewards']", setup: () => showAlchemyTab("transmute"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyTransmute.rewards.title", descriptionKey: "tutorial.alchemyTransmute.rewards.desc", side: "bottom" },
    },
    {
      id: "alchemy-transmute-submit",
      target: { selector: "[data-tutorial='alchemy-transmute-submit']", setup: () => showAlchemyTab("transmute"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemyTransmute.submit.title", descriptionKey: "tutorial.alchemyTransmute.submit.desc", side: "top" },
    },
  ]
)

export const alchemySmeltTutorial = moduleTutorial(
  "alchemy-smelt",
  "tutorial.alchemySmelt.name",
  "tutorial.alchemySmelt.description",
  "smelt",
  [
    {
      id: "alchemy-smelt",
      target: { selector: "[data-tutorial='alchemy-smelt']", setup: () => showAlchemyTab("smelt"), waitForMs: 5000 },
      popover: { titleKey: "tutorial.alchemySmelt.overview.title", descriptionKey: "tutorial.alchemySmelt.overview.desc", side: "bottom" },
    },
  ]
)

export const alchemySkillForgeTutorial = moduleTutorial(
  "alchemy-skill-forge",
  "tutorial.alchemySkillForge.name",
  "tutorial.alchemySkillForge.description",
  "skill-forge",
  [
    {
      id: "alchemy-skill-forge-overview",
      target: { selector: "[data-tutorial='alchemy-skill-forge']", setup: () => showAlchemyTab("skill-forge"), waitForMs: 6000 },
      popover: { titleKey: "tutorial.alchemySkillForge.overview.title", descriptionKey: "tutorial.alchemySkillForge.overview.desc", side: "bottom" },
    },
    {
      id: "alchemy-skill-forge-source",
      target: { selector: "[data-tutorial='skill-forge-source']", setup: () => showAlchemyTab("skill-forge"), waitForMs: 6000 },
      popover: { titleKey: "tutorial.alchemySkillForge.source.title", descriptionKey: "tutorial.alchemySkillForge.source.desc", side: "bottom" },
    },
    {
      id: "alchemy-skill-forge-materials",
      target: { selector: "[data-tutorial='skill-forge-materials']", setup: () => showAlchemyTab("skill-forge"), waitForMs: 3000, optional: true },
      popover: { titleKey: "tutorial.alchemySkillForge.materials.title", descriptionKey: "tutorial.alchemySkillForge.materials.desc", side: "right" },
      mobile: { target: { selector: "[data-tutorial='skill-forge-panel']" }, popover: { side: "over" } },
    },
    {
      id: "alchemy-skill-forge-graph",
      target: { selector: "[data-tutorial='skill-forge-graph']", setup: () => showAlchemyTab("skill-forge"), waitForMs: 3000, optional: true },
      popover: { titleKey: "tutorial.alchemySkillForge.graph.title", descriptionKey: "tutorial.alchemySkillForge.graph.desc", side: "bottom" },
      mobile: { target: { selector: "[data-tutorial='skill-forge-panel']" }, popover: { side: "over" } },
    },
    {
      id: "alchemy-skill-forge-preview",
      target: { selector: "[data-tutorial='skill-forge-preview']", setup: () => showAlchemyTab("skill-forge"), waitForMs: 3000, optional: true },
      popover: { titleKey: "tutorial.alchemySkillForge.preview.title", descriptionKey: "tutorial.alchemySkillForge.preview.desc", side: "left" },
      mobile: { target: { selector: "[data-tutorial='skill-forge-panel']" }, popover: { side: "over" } },
    },
    {
      id: "alchemy-skill-forge-submit",
      target: { selector: "[data-tutorial='skill-forge-submit']", setup: () => showAlchemyTab("skill-forge"), waitForMs: 3000, optional: true },
      popover: { titleKey: "tutorial.alchemySkillForge.submit.title", descriptionKey: "tutorial.alchemySkillForge.submit.desc", side: "top" },
      mobile: { target: { selector: "[data-tutorial='skill-forge-panel']" }, popover: { side: "over" } },
    },
  ]
)
