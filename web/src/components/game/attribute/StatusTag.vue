<template>
  <AttributeEffectPopover :attr-key="props.attrKey">
    <el-tag class="status-tag" type="primary" :disable-transitions="true">
      <span class="flex flex-row items-center">
        <AttributeIcon :attr-key="props.attrKey" :alt="label" :size="14" :fallback="fallbackIcon" />
        <span>{{ label }}</span>
        <span v-if="value">: {{ displayValue }}</span>
      </span>
    </el-tag>
  </AttributeEffectPopover>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import AttributeEffectPopover from "@components/game/attribute/AttributeEffectPopover.vue"
import { getAttributeDefinition, getAttributeUnit, formatAttributeValue } from "@services/attribute-display"

const props = defineProps<{
  attrKey: string
  value?: number
  signed?: boolean
}>()

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
const displayValue = computed(() => {
  const unit = getAttributeUnit(props.attrKey)
  return formatAttributeValue(Number(props.value || 0), unit, Boolean(props.signed))
})
</script>

<style scoped>
.status-tag {
  max-width: 100%;
}
</style>
