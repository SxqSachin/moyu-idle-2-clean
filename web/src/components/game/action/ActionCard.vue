<template>
  <LabeledFrameCard
    class="action-card"
    :label="label"
    :label-lines="2"
    :rarity-color="props.rarityColor"
    :width="frameWidth"
    :height="frameHeight"
    :label-class="labelClass"
    @click="$emit('select')"
  >
    <template #default>
      <TypeIcon :type="iconType" :id="iconID" :alt="label" :size="iconSize" :fallback="iconFallback" />
    </template>
  </LabeledFrameCard>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import LabeledFrameCard from "@components/game/common/LabeledFrameCard.vue"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import { parseIconId, resolveFallbackIconByIconId } from "@services/item-icons"
import { formatDurationSecondsOneDecimal } from "@/utils/duration-format"

const props = defineProps<{
  actionId: string
  iconId?: string
  rarityColor?: string
  label: string
  description?: string
  durationS?: number
  requirementCount: number
  outputCount: number
  frameWidth?: number | string
  frameHeight?: number | string
  iconSize?: number
  labelClass?: string
}>()

defineEmits<{
  (e: "select"): void
}>()

const { t } = useI18n()

const iconKey = computed(() => props.iconId || `action.${props.actionId}`)
const parsedIcon = computed(() => parseIconId(iconKey.value))
const iconType = computed(() => parsedIcon.value?.type || "action")
const iconID = computed(() => parsedIcon.value?.id || props.actionId)
const iconFallback = computed(() => resolveFallbackIconByIconId(iconKey.value, "❓️"))
const iconSize = computed(() => props.iconSize ?? 42)
const frameWidth = computed(() => props.frameWidth ?? 64)
const frameHeight = computed(() => props.frameHeight ?? 64)
const labelClass = computed(() => props.labelClass ?? "text-sm")
const durationText = computed(() =>
  Number.isFinite(props.durationS) && Number(props.durationS || 0) > 0
    ? formatDurationSecondsOneDecimal(Number(props.durationS || 0), t)
    : ""
)
</script>
