import { computed, type Ref } from "vue"
import { useAttributeRows } from "@/composables/game/attributes/useAttributeRows"
import { useRoleStateStore, type RoleKey } from "@stores/role-state"

type FormulaSnapshot = {
  derived?: Record<string, string>
}

type TranslateFn = (key: string, params?: Record<string, string | number>) => string

export function useRoleAttributeModel(
  roleKey: Ref<RoleKey>,
  formula: Ref<FormulaSnapshot>,
  t: TranslateFn
) {
  const roleStateStore = useRoleStateStore()
  const roleState = computed(() => roleStateStore.ensureRole(roleKey.value))
  const attributes = computed(() => roleState.value.attributes)
  const attributeProgress = computed(() => roleState.value.attributeProgress)
  const unitAttributes = computed<Record<string, number>>(() => roleStateStore.roleUnitAttributes(roleKey.value))
  const { attrRows } = useAttributeRows(attributes, attributeProgress, formula, t)

  return {
    roleState,
    attributes,
    attributeProgress,
    unitAttributes,
    attrRows
  }
}
