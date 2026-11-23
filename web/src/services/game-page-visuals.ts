export type GamePageVisualTone = "green" | "blue" | "amber" | "violet" | "rose" | "cyan" | "flame"

export type GamePageVisualDefinition = {
  src: string
  tone: GamePageVisualTone
}

const MENU_ACTION_BASE = "/images/game/menu-actions"

function menuAction(name: string, tone: GamePageVisualTone): GamePageVisualDefinition {
  return { src: `${MENU_ACTION_BASE}/${name}.png`, tone }
}

export const gamePageVisuals = {
  achievements: menuAction("achievements", "green"),
  alchemy: menuAction("alchemy", "violet"),
  affix: menuAction("affix", "violet"),
  auction: menuAction("auction", "amber"),
  arena: menuAction("battle", "flame"),
  battle: menuAction("battle", "flame"),
  building: menuAction("building", "green"),
  cattery: menuAction("cattery", "green"),
  character: menuAction("character", "green"),
  cheaterList: menuAction("mysterious", "rose"),
  collecting: menuAction("collecting", "green"),
  combat: menuAction("battle", "flame"),
  community: menuAction("community", "cyan"),
  cooking: menuAction("cooking", "green"),
  crafting: menuAction("crafting", "green"),
  dictionary: menuAction("dictionary", "cyan"),
  enhance: menuAction("enhance", "violet"),
  exercise: menuAction("character", "green"),
  explore: menuAction("mining", "green"),
  exploration: menuAction("mining", "green"),
  farmingAnimal: menuAction("farming_animal", "green"),
  farmingPlant: menuAction("farmingPlant", "green"),
  farming_animal: menuAction("farming_animal", "green"),
  feedback: menuAction("feedback", "cyan"),
  fishing: menuAction("fishing", "green"),
  forging: menuAction("forging", "green"),
  gameguide: menuAction("gameguide", "cyan"),
  home: menuAction("homepage", "green"),
  homepage: menuAction("homepage", "green"),
  instructions: menuAction("instructions", "cyan"),
  inventory: menuAction("inventory", "green"),
  market: menuAction("market", "amber"),
  mining: menuAction("mining", "green"),
  mysterious: menuAction("mysterious", "violet"),
  quest: menuAction("quest", "green"),
  sewing: menuAction("sewing", "green"),
  shop: menuAction("shop", "amber"),
  smelt: menuAction("forging", "green"),
  specialManufacture: menuAction("crafting", "green"),
  tasks: menuAction("quest", "green"),
  training: menuAction("character", "green"),
  updateLog: menuAction("updateLog", "cyan"),
  warehouse: menuAction("inventory", "green"),
  wardrobe: menuAction("mysterious", "rose"),
  work: menuAction("collecting", "green")
} satisfies Record<string, GamePageVisualDefinition>

export function resolveGamePageVisual(key: string): GamePageVisualDefinition | null {
  return gamePageVisuals[key as keyof typeof gamePageVisuals] || null
}
