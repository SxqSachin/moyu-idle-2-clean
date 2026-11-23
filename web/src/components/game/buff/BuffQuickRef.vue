<template>
  <el-popover
    v-model:visible="popoverActive"
    placement="top"
    :width="340"
    :trigger="supportsHover ? 'hover' : 'manual'"
    :persistent="true"
    :teleported="teleported"
    :hide-after="supportsHover ? 120 : 0"
    :show-after="supportsHover ? 200 : 0"
    popper-class="buff-quick-ref-popper"
    @show="onPopoverShow"
    @hide="onPopoverHide"
  >
    <template #reference>
      <el-tag
        :class="['buff-quick-ref-tag px-1! relative', iconOnly ? 'buff-quick-ref-tag--icon-only' : '']"
        :disable-transitions="true"
        effect="plain"
        :style="tagStyle"
        size="small"
        @mouseenter="primePopover"
        @focusin="primePopover"
        @click="onTagClick"
      >
        <Icon v-if="buffIcon" :icon="buffIcon" class="inline-block align-[-0.12em]" />
        <span v-if="!iconOnly">{{ buffName }}</span>
        <span v-if="showStackCount" class="buff-stack-badge">{{ stackCount }}</span>
        <!-- <span v-if="!iconOnly && remainLabel">({{ remainLabel }})</span> -->
      </el-tag>
    </template>
    <div class="space-y-2 text-xs">
      <div class="font-medium flex items-center gap-1">
        <Icon v-if="buffIcon" :icon="buffIcon" class="inline-block align-[-0.12em]" />
        <span>{{ buffName }}</span>
      </div>
      <div class="app-muted">{{ kindLabel }}</div>
      <div class="my-1">
        {{ t(`${definition?.descKey}`)}}
      </div>
      <div class="app-muted">{{ remainLabel }}</div>
      <div v-if="showStackCount" class="app-muted">{{ t("game.buffStackCount", { count: stackCount }) }}</div>
      <div v-if="effectLines.length > 0" class="space-y-1">
        <div class="text-[11px] uppercase tracking-wide app-muted">{{ t("game.buffEffectsTitle") }}</div>
        <div v-for="line in effectLines" :key="line.key" class="buff-effect-line">
          <span>{{ line.label }}</span>
          <BuffQuickRef
            v-for="nextBuffId in getNestedBuffIds(line.buffIds)"
            :key="`${line.key}-${nextBuffId}`"
            :buff-id="nextBuffId"
            :remain-turns="-1"
            :depth="nestedDepth"
            :ancestor-buff-ids="nestedAncestorBuffIds"
          />
          <span v-if="line.suffix">{{ line.suffix }}</span>
        </div>
      </div>
      <!-- <div v-if="payloadText && Object.keys(payloadText).length">{{ payloadText }}</div> -->
    </div>
  </el-popover>
  <GameDialog v-model="mobilePopoverVisible" class="dialog-mobile-windowed buff-quick-ref-dialog" :width="dialogWidth">
    <div class="space-y-2 text-xs">
      <div class="font-medium flex items-center gap-1">
        <Icon v-if="buffIcon" :icon="buffIcon" class="inline-block align-[-0.12em]" />
        <span>{{ buffName }}</span>
      </div>
      <div class="app-muted">{{ kindLabel }}</div>
      <div class="my-1">
        {{ t(`${definition?.descKey}`)}}
      </div>
      <div class="app-muted">{{ remainLabel }}</div>
      <div v-if="showStackCount" class="app-muted">{{ t("game.buffStackCount", { count: stackCount }) }}</div>
      <div v-if="effectLines.length > 0" class="space-y-1">
        <div class="text-[11px] uppercase tracking-wide app-muted">{{ t("game.buffEffectsTitle") }}</div>
        <div v-for="line in effectLines" :key="line.key" class="buff-effect-line">
          <span>{{ line.label }}</span>
          <BuffQuickRef
            v-for="nextBuffId in getNestedBuffIds(line.buffIds)"
            :key="`${line.key}-${nextBuffId}`"
            :buff-id="nextBuffId"
            :remain-turns="-1"
            :depth="nestedDepth"
            :ancestor-buff-ids="nestedAncestorBuffIds"
          />
          <span v-if="line.suffix">{{ line.suffix }}</span>
        </div>
      </div>
      <!-- <div v-if="payloadText">{{ payloadText }}</div> -->
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { useDevice } from "@composables/game/useDevice"
import { getBuffDefinition, getBuffEffectDescriptions, getBuffTypeDefinition, getLocalizedBuffKindLabel, getLocalizedBuffName } from "@services/buff-definition"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = withDefaults(defineProps<{
  buffId: string
  remainTurns?: number
  payload?: Record<string, unknown>
  payloadFormula?: Record<string, unknown>
  iconOnly?: boolean
  stackCount?: number
  teleported?: boolean
  depth?: number
  ancestorBuffIds?: string[]
}>(), {
  iconOnly: false,
  stackCount: 1,
  teleported: false,
  depth: 0,
  ancestorBuffIds: () => []
})

const { t } = useI18n()
const { supportsHover, isTouchLike, isMobile } = useDevice()
const popoverActive = ref(false)
const mobilePopoverVisible = ref(false)

const definition = computed(() => getBuffDefinition(props.buffId))
const typeDefinition = computed(() => getBuffTypeDefinition(definition.value?.type || ""))
const buffName = computed(() => getLocalizedBuffName(t, props.buffId, t("game.buffNameFallback")))
const buffIcon = computed(() => String(definition.value?.icon || "").trim())
const kind = computed(() => definition.value?.kind === "debuff" ? "debuff" : "buff")
const iconOnly = computed(() => Boolean(props.iconOnly))
const showStackCount = computed(() => Number(props.stackCount) > 1)
const teleported = computed(() => Boolean(props.teleported))
const kindLabel = computed(() => getLocalizedBuffKindLabel(t, kind.value))
const remainLabel = computed(() => {
  const remainTurns = Number(props.remainTurns)
  if (Number.isFinite(remainTurns) && remainTurns >= 0) {
    return t("game.battleUnitDurationTurns", { turns: Math.max(0, Math.trunc(remainTurns)) })
  }
  const definitionTurns = Number(definition.value?.durationTurns)
  if (definition.value?.permanent !== true && Number.isFinite(definitionTurns) && definitionTurns > 0) {
    return t("game.battleUnitDurationTurns", { turns: Math.trunc(definitionTurns) })
  }
  return t("game.battleUnitDurationInfinite")
})
const payloadText = computed(() => {
  if (!props.payload || typeof props.payload !== "object") return ""
  return JSON.stringify(props.payload)
})
const effectLines = computed(() => getBuffEffectDescriptions(t, props.buffId, props.payload, props.payloadFormula))
const normalizedBuffId = computed(() => String(props.buffId || "").trim())
const normalizedAncestorBuffIds = computed(() => props.ancestorBuffIds.map((item) => String(item || "").trim()).filter(Boolean))
const nestedDepth = computed(() => Math.max(0, Math.trunc(Number(props.depth) || 0)) + 1)
const nestedAncestorBuffIds = computed(() => [...normalizedAncestorBuffIds.value, normalizedBuffId.value].filter(Boolean))
const tagStyle = computed(() => {
  const color = typeDefinition.value?.color || (kind.value === "debuff" ? "#f56c6c" : "#67c23a")
  return { borderColor: color, color }
})
const dialogWidth = computed(() => {
  if (isMobile.value) return "92%"
  return "420px"
})

function onPopoverShow() {
  if (!supportsHover.value) return
  popoverActive.value = true
}

function onPopoverHide() {
  popoverActive.value = false
}

function primePopover() {
  if (!supportsHover.value) return
}

function onTagClick() {
  if (!isTouchLike.value) return
  mobilePopoverVisible.value = true
}

function getNestedBuffIds(buffIds?: string[]) {
  if (nestedDepth.value > 2) return []
  const blocked = new Set([...normalizedAncestorBuffIds.value, normalizedBuffId.value])
  return (Array.isArray(buffIds) ? buffIds : [])
    .map((item) => String(item || "").trim())
    .filter((item, index, array) => item && !blocked.has(item) && array.indexOf(item) === index)
}
</script>

<style scoped>
.buff-quick-ref-tag {
  cursor: default;
}

.buff-quick-ref-tag--icon-only {
  padding: 2px 6px;
}

.buff-quick-ref-dialog :deep(.el-dialog__body) {
  padding-top: 12px;
}

.buff-stack-badge {
  position: absolute;
  bottom: -4px;
  right: -4px;
  min-width: 14px;
  height: 14px;
  padding: 0 2px;
  font-size: 10px;
  line-height: 14px;
  text-align: center;
  color: currentColor;
  background-color: rgba(255, 255, 255, 0.45);
  border-radius: 7px;
  font-weight: 700;
  pointer-events: none;
  opacity: 0.65;
}

.buff-effect-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}
</style>
