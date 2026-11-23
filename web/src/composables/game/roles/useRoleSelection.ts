import { ref, watch, type Ref } from "vue"

type KittyNode = { id?: number }

export function useRoleSelection(kitties: Ref<KittyNode[]>) {
  const selectedRoleType = ref<"player" | "kitty">("player")
  const selectedKittyId = ref(0)

  watch(
    kitties,
    (next) => {
      if (selectedRoleType.value !== "kitty") return
      if (next.some((row) => Number(row.id || 0) === selectedKittyId.value)) return
      if (next.length > 0) {
        selectedKittyId.value = Number(next[0]?.id || 0)
      } else {
        selectedRoleType.value = "player"
        selectedKittyId.value = 0
      }
    },
    { immediate: true }
  )

  function selectPlayerRole() {
    selectedRoleType.value = "player"
  }

  function selectKittyRole(kittyId: number) {
    if (!Number.isFinite(kittyId) || kittyId <= 0) return false
    selectedKittyId.value = kittyId
    selectedRoleType.value = "kitty"
    return true
  }

  return {
    selectedRoleType,
    selectedKittyId,
    selectPlayerRole,
    selectKittyRole
  }
}
