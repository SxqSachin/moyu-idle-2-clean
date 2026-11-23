<template>
  <el-popover
    v-model:visible="popoverActive"
    placement="top"
    :width="360"
    :trigger="supportsHover ? 'hover' : 'manual'"
    :persistent="true"
    :teleported="false"
    :hide-after="supportsHover ? 120 : 0"
    :show-after="supportsHover ? 200 : 0"
    @show="onPopoverShow"
    @hide="onPopoverHide"
  >
    <template #reference>
      <el-tag
        class="buff-effect-quick-ref-tag"
        :disable-transitions="true"
        effect="plain"
        size="small"
        @mouseenter="primePopover"
        @focusin="primePopover"
        @click="onTagClick"
      >
        {{ labelText }}
      </el-tag>
    </template>
    <BuffEffectQuickRefContent :effect-kind="effectKind" />
  </el-popover>

  <GameDialog
    v-model="mobilePopoverVisible"
    class="dialog-mobile-windowed buff-effect-quick-ref-dialog"
    :title="labelText"
    :width="dialogWidth"
  >
    <BuffEffectQuickRefContent :effect-kind="effectKind" />
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import BuffEffectQuickRefContent from "@components/game/buff/BuffEffectQuickRefContent.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = defineProps<{
  effectKind: string
}>()

const { t } = useI18n()
const { supportsHover, isTouchLike, isMobile } = useDevice()
const popoverActive = ref(false)
const mobilePopoverVisible = ref(false)

const normalizedEffectKind = computed(() => String(props.effectKind || "").trim())
const labelText = computed(() => {
  const key = `game.buffEffectKind_${normalizedEffectKind.value}`
  const localized = t(key)
  return localized === key ? normalizedEffectKind.value : localized
})
const dialogWidth = computed(() => (isMobile.value ? "92%" : "420px"))

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
</script>

<style scoped>
.buff-effect-quick-ref-tag {
  cursor: default;
  border-color: #8b5cf6;
  color: #6d28d9;
  vertical-align: middle;
}

.buff-effect-quick-ref-dialog :deep(.el-dialog__body) {
  padding-top: 12px;
}
</style>
