type RoleType = "player" | "kitty"
type CharacterTab = "attributes" | "equipment" | "skills" | "specialization" | "battle-items" | "loadouts"

type CharacterTarget = {
  targetRoleType: RoleType | null
  targetKittyId: number
  targetTab: CharacterTab | ""
}

const state: CharacterTarget = {
  targetRoleType: null,
  targetKittyId: 0,
  targetTab: ""
}

export type { RoleType, CharacterTab }

export function openCharacterTarget(payload: {
  roleType: RoleType
  kittyId?: number
  tab: CharacterTab
}) {
  state.targetRoleType = payload.roleType
  state.targetKittyId = payload.roleType === "kitty" ? Math.max(0, Math.trunc(Number(payload.kittyId || 0))) : 0
  state.targetTab = payload.tab
}

export function consumeCharacterTarget(): CharacterTarget {
  const snapshot: CharacterTarget = {
    targetRoleType: state.targetRoleType,
    targetKittyId: state.targetKittyId,
    targetTab: state.targetTab
  }
  state.targetRoleType = null
  state.targetKittyId = 0
  state.targetTab = ""
  return snapshot
}
