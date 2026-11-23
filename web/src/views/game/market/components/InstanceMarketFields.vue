<template>
  <div class="variant-fields">
    <template v-for="(field, index) in fields" :key="index">
      <template v-if="field.format === 'operation_effect'">
        <OperationEffectSummary :lines="effectLines(field.value)" />
      </template>
      <dl v-else class="variant-field"><dt>{{ t(field.labelKey) }}</dt><dd>{{ valueText(field) }}</dd></dl>
    </template>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from "vue-i18n"
import OperationEffectSummary from "@components/game/operation-effect/OperationEffectSummary.vue"
import type { MarketField } from "@services/instance-market"
import type { OperationEffectDefinitionView } from "@services/operation-effects"
import { operationEffectLifetimeText, operationEffectModifierLines } from "@services/operation-effect-display"
defineProps<{ fields: MarketField[] }>()
const { t, te, locale } = useI18n()
function valueText(field: MarketField) {
  if (typeof field.value === "number") return new Intl.NumberFormat(locale.value, { maximumFractionDigits: 12, style: field.format === "percent" ? "percent" : "decimal" }).format(field.value)
  if (field.value && typeof field.value === "object") {
    const value = field.value as Record<string, unknown>
    return String(value[locale.value] ?? value[locale.value.startsWith("zh") ? "zhCN" : "enUS"] ?? Object.values(value)[0] ?? "")
  }
  const value = String(field.value ?? "")
  if (["male", "female"].includes(value)) return t(`game.instanceMarket.${value}`)
  return te(value) ? t(value) : value
}
function effectLines(value: unknown) {
  const def = value as OperationEffectDefinitionView
  if (!def?.kind || !def?.params || !def?.lifetime) return []
  return [operationEffectLifetimeText(def, t), ...operationEffectModifierLines(def.kind, def.params, t, true, 12)].filter(Boolean)
}
</script>
<style scoped>
.variant-fields { display: grid; gap: 6px; margin: 0; font-size: 13px; line-height: 1.5; }
.variant-field { display: flex; justify-content: space-between; gap: 12px; margin: 0; }
dt { color: var(--el-text-color-secondary); }
dd { margin: 0; text-align: right; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
.variant-effect { overflow-wrap: anywhere; }
</style>
