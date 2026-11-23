<template>
  <AttributesStatsTab
    :rows="visibleRows"
    :basic-attack="basicAttack"
    :unit-attributes="unitAttributes"
    :visible-group-keys="visibleGroupKeys"
    :split-work-sections-into-two-columns="splitWorkSectionsIntoTwoColumns"
    @refresh="$emit('refresh')"
    @open-allocate-dialog="$emit('open-allocate-dialog')"
  />
</template>

<script setup lang="ts">
import { computed, toRef } from "vue"
import { useI18n } from "vue-i18n"
import { useRoleAttributeModel } from "@/composables/game/roles/useRoleAttributeModel"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import type { RoleKey } from "@stores/role-state"
import AttributesStatsTab from "@components/game/attributes/AttributesStatsTab.vue"

type FormulaSnapshot = {
  derived?: Record<string, string>
}

const props = withDefaults(defineProps<{
  roleKey: RoleKey
  formula?: FormulaSnapshot
  basicAttack?: Record<string, unknown> | null
  unitAttributesOverride?: Record<string, number> | null
  visibleGroupKeys?: Array<"work" | "battle">
  splitWorkSectionsIntoTwoColumns?: boolean
  rowFilter?: (row: AttributeRow) => boolean
}>(), {
  formula: () => ({}),
  basicAttack: null,
  unitAttributesOverride: null,
  visibleGroupKeys: () => ["work", "battle"],
  splitWorkSectionsIntoTwoColumns: false,
  rowFilter: undefined
})

defineEmits<{
  (e: "refresh"): void
  (e: "open-allocate-dialog"): void
}>()

const { t } = useI18n()
const roleKeyRef = toRef(props, "roleKey")
const formulaRef = computed(() => props.formula || {})
const { attrRows, unitAttributes: roleUnitAttributes } = useRoleAttributeModel(roleKeyRef, formulaRef, t)
const visibleRows = computed(() => {
  if (!props.rowFilter) return attrRows.value
  return attrRows.value.filter(props.rowFilter)
})
const unitAttributes = computed(() => props.unitAttributesOverride || roleUnitAttributes.value)
</script>
