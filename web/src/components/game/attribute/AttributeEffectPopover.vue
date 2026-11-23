<template>
  <el-popover
    v-if="hasTooltipContent"
    :placement="placement"
    trigger="hover"
    :width="width"
    :show-after="380"
  >
    <template #reference>
      <span class="inline-flex cursor-help">
        <slot />
      </span>
    </template>
    <div class="space-y-1 text-xs whitespace-pre-wrap">
      <div v-if="desc">
        <div class="font-semibold">{{ t("game.attrTooltipDescTitle") }}</div>
        <div class="app-muted">{{ desc }}</div>
      </div>
      <div v-if="impactLines.length > 0" class="flex flex-col gap-1">
        <div class="font-semibold">{{ t("game.attrTooltipImpactTitle") }}</div>
        <div>
          <div>{{ t("game.attrPerPointImpactTitle") }}</div>
          <div v-for="str in impactLines" :key="str">- {{ str }}</div>
        </div>
      </div>
    </div>
  </el-popover>
  <slot v-else />
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { useAttributeStats } from "@/composables/game/attributes/useAttributeStats"
import { buildBaseAttrImpactMap } from "@services/attribute-derived-impact"
import { formatAttributeValue, getAttributeDefinition } from "@services/attribute-display"

const categoryWeight: Record<string, number> = { battle: 0, work: 1, other: 2 }

const props = defineProps<{
  attrKey: string
  placement?: "top" | "bottom" | "left" | "right" | "top-start" | "top-end" | "bottom-start" | "bottom-end" | "left-start" | "left-end" | "right-start" | "right-end"
  width?: number
}>()

const placement = computed(() => props.placement || "top")
const width = computed(() => props.width || 260)

const { t } = useI18n()
const { formula } = useAttributeStats()

const key = computed(() => String(props.attrKey || "").trim())

const desc = computed(() => {
  if (!key.value) return ""
  const messageKey = `attr.${key.value}.desc`
  const localized = t(messageKey)
  if (localized === messageKey || localized.trim() === "") return ""
  return localized
})

const impactLines = computed(() => {
  if (!key.value) return []
  const impactMapByBase = buildBaseAttrImpactMap(formula.value?.derived)
  const impacts = [...(impactMapByBase[key.value] || [])].sort((a, b) => {
    const targetA = getAttributeDefinition(a.targetAttrKey)
    const targetB = getAttributeDefinition(b.targetAttrKey)
    const aw = categoryWeight[targetA?.category || "other"] ?? 99
    const bw = categoryWeight[targetB?.category || "other"] ?? 99
    if (aw !== bw) return aw - bw
    const ao = Number(targetA?.order || 0)
    const bo = Number(targetB?.order || 0)
    if (ao !== bo) return ao - bo
    return a.targetAttrKey.localeCompare(b.targetAttrKey)
  })

  return impacts.map((impact) => {
    const targetDef = getAttributeDefinition(impact.targetAttrKey)
    const targetNameKey =
      (typeof targetDef?.nameKey === "string" && targetDef.nameKey) ||
      `attr.${impact.targetAttrKey}.name`
    const targetLocalized = t(targetNameKey)
    const targetLabel =
      targetLocalized !== targetNameKey && targetLocalized.trim() !== ""
        ? targetLocalized
        : impact.targetAttrKey
    const impactValue = formatAttributeValue(
      impact.deltaPerPoint,
      targetDef?.unit === "percent" ? "percent" : "number",
      true
    )
    return t("game.attrPerPointImpactLine", { target: targetLabel, value: impactValue })
  })
})

const hasTooltipContent = computed(() => Boolean(desc.value) || impactLines.value.length > 0)
</script>
