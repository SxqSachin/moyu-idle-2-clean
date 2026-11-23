<template>
  <AttributeEffectPopover :attr-key="props.attrKey" :placement="placement" :width="popoverWidth">
    <el-tag class="status-require-tag" :type="passed ? 'success' : 'danger'" :disable-transitions="true">
      <span class="flex flex-row items-center">
        <AttributeIcon :attr-key="props.attrKey" :alt="label" :size="14" :fallback="fallbackIcon" />
        <span>{{ label }}</span>
        <span>: {{ requiredValue }} ({{ selfValue }})</span>
      </span>
    </el-tag>
  </AttributeEffectPopover>
</template>

<script setup lang="ts">
import { computed, unref } from "vue"
import { useI18n } from "vue-i18n"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import AttributeEffectPopover from "@components/game/attribute/AttributeEffectPopover.vue"
import { getAttributeDefinition } from "@services/attribute-display"
import { useAttributeStats } from "@/composables/game/attributes/useAttributeStats"

const { attributes: attr } = useAttributeStats();

const props = defineProps<{
  attrKey: string
  requiredValue: number
  currentValue?: number
  placement?: "top" | "bottom" | "left" | "right" | "top-start" | "top-end" | "bottom-start" | "bottom-end" | "left-start" | "left-end" | "right-start" | "right-end"
  popoverWidth?: number
}>()

const placement = computed(() => props.placement || "top")
const popoverWidth = computed(() => props.popoverWidth || 260)

const selfValue = computed(() => {
  if (props.currentValue !== undefined) {
    return props.currentValue
  }
  const node = unref(attr)?.[props.attrKey]
  if (!node || typeof node.value !== "number" || !Number.isFinite(node.value)) {
    return 0
  }
  return node.value
});

const passed = computed(() => {
  return selfValue.value >= Number(props.requiredValue || 0)
})

const { t } = useI18n()
const definition = computed(() => getAttributeDefinition(props.attrKey))
const nameKey = computed(() => {
  const fromDef = definition.value?.nameKey
  if (typeof fromDef === "string" && fromDef.trim() !== "") return fromDef
  return `attr.${props.attrKey}.name`
})
const label = computed(() => {
  const localized = t(nameKey.value)
  if (localized !== nameKey.value && localized.trim() !== "") return localized
  return props.attrKey
})
const fallbackIcon = computed(() => {
  const emoji = definition.value?.emojiIcon
  if (typeof emoji === "string" && emoji.trim() !== "") return emoji
  return "📈"
})
</script>

<style scoped>
.status-require-tag {
  max-width: 100%;
}
</style>
