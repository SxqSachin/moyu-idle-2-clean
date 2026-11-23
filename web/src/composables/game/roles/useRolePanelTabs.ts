import { computed, ref, type Ref } from "vue"
import { normalizeRolePanelTabForRole, type RolePanelTab } from "./role-panel-tabs"

export function useRolePanelTabs(selectedRoleType: Ref<"player" | "kitty">) {
  const sharedTab = ref<RolePanelTab>("attributes")

  const activeTab = computed<RolePanelTab>({
    get() {
      return normalizeRolePanelTabForRole(sharedTab.value, selectedRoleType.value)
    },
    set(value) {
      sharedTab.value = normalizeRolePanelTabForRole(value, selectedRoleType.value)
    }
  })

  function normalizeForRole() {
    sharedTab.value = normalizeRolePanelTabForRole(sharedTab.value, selectedRoleType.value)
  }

  return {
    activeTab,
    sharedTab,
    normalizeForRole
  }
}
