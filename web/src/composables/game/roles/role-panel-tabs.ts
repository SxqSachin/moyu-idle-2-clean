import type { RoleType } from "./useCharacterTarget"

export const rolePanelTabs = ["attributes", "equipment", "skills", "specialization", "battle-items", "loadouts"] as const
export const kittyRolePanelTabs = ["attributes", "equipment", "skills"] as const

export type RolePanelTab = (typeof rolePanelTabs)[number]

export function isRolePanelTab(value: unknown): value is RolePanelTab {
  return typeof value === "string" && rolePanelTabs.includes(value as RolePanelTab)
}

export function normalizeRolePanelTabForRole(tab: RolePanelTab, roleType: RoleType): RolePanelTab {
  if (roleType === "kitty" && !kittyRolePanelTabs.includes(tab as (typeof kittyRolePanelTabs)[number])) {
    return "attributes"
  }
  return tab
}
