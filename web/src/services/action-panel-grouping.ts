import type { ActionDefinition, ActionSubTypeDefinition } from "@game-data/game-data"

type ActionGroup = { id: string; nameKey?: string; actions: ActionDefinition[] }
export type ActionPanelTabGroup = ActionGroup & { sections?: ActionGroup[] }

const fishCuisineTab = { id: "fish_cuisine", nameKey: "game.fishCuisine.category" }
const fishCuisineSections = [
  { id: "cooking_grilled_fish", nameKey: "game.fishCuisine.grilled_fish" },
  { id: "cooking_fish_soup", nameKey: "game.fishCuisine.fish_soup" }
]

function presentationTab(id: string, nameKey?: string) {
  return fishCuisineSections.some((section) => section.id === id) ? fishCuisineTab : { id, nameKey }
}

// Presentation grouping must not rewrite subtypes used by duration and effect selectors.
export function buildActionPanelTabGroups(actions: ActionDefinition[], definitions: ActionSubTypeDefinition[]): ActionPanelTabGroup[] {
  const byId = new Map<string, ActionDefinition[]>()
  for (const action of actions) {
    const { id } = presentationTab(action.subType || "common")
    const items = byId.get(id) || []
    items.push(action)
    byId.set(id, items)
  }
  const groups: ActionPanelTabGroup[] = []
  const seen = new Set<string>()
  for (const definition of definitions) {
    const tab = presentationTab(definition.id, definition.nameKey)
    const items = byId.get(tab.id)
    if (!items?.length || seen.has(tab.id)) continue
    groups.push({ ...tab, actions: items })
    seen.add(tab.id)
  }
  for (const [id, items] of byId) {
    if (seen.has(id)) continue
    groups.push({ ...(id === fishCuisineTab.id ? fishCuisineTab : { id }), actions: items })
  }
  return groups.map((group) => group.id !== fishCuisineTab.id ? group : {
    ...group,
    sections: fishCuisineSections.map((section) => ({
      ...section,
      actions: group.actions.filter((action) => action.subType === section.id)
    })).filter((section) => section.actions.length > 0)
  })
}
