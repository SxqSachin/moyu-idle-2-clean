<template>
  <el-popover
    v-model:visible="popoverActive"
    placement="top"
    :width="450"
    :trigger="supportsHover ? 'hover' : 'manual'"
    :persistent="false"
    :show-after="supportsHover ? 200 : 0"
    @show="onPopoverShow"
    @hide="onPopoverHide"
  >
    <template #reference>
      <el-tag
        type="primary"
        class="lp-suppress"
        :disable-transitions="true"
        @mouseenter="primePopover"
        @focusin="primePopover"
        @pointerdown="longPress.onPointerDown"
        @pointermove="longPress.onPointerMove"
        @pointerup="longPress.onPointerUp"
        @pointercancel="longPress.onPointerUp"
        @pointerleave="longPress.onPointerUp"
        @contextmenu="longPress.onContextMenu"
      >
        <div class="item-quick-ref">
          <TypeIcon :type="iconType" :id="iconID" :alt="itemLabel" :size="22" :fallback="fallbackIcon" />
          <span class="item-quick-ref-name" :class="instanceCustomLabelClass">{{ itemLabel }}</span>
          <span v-if="count" class="item-quick-ref-count">: {{ countDisplay }}</span>
          <slot></slot>
        </div>
      </el-tag>
    </template>
    <ItemPopoverContent
      v-if="popoverLoaded"
      :item-id="itemId"
      :instance-id="instanceId"
      :active="popoverActive"
      :fix-height="true"
    />
  </el-popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import { useLongPress } from "@composables/game/useLongPress"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import ItemPopoverContent from "@components/game/item/ItemPopoverContent.vue"
import { getLocalizedItemName } from "@services/item-label"
import { itemCustomLabelClass, itemCustomName } from "@services/item-customization"
import { getLocalizedRuntimeSkillName } from "@services/skill-definition"
import { parseIconId, resolveIconIdByItemId, resolveItemFallbackIcon } from "@services/item-icons"
import { useInventoryStore } from "@stores/inventory"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import { useItemPopoverDialogStore } from "@stores/item-popover-dialog"
import { formatNumberWithThousands, formatQuantityDisplay } from "@utils/number-format"

const props = withDefaults(
  defineProps<{
    itemId: string
    instanceId?: number | null
    count?: number
    normalNumberFormat?: boolean
  }>(),
  {
    instanceId: null,
    count: undefined,
    normalNumberFormat: false
  }
)

const { t } = useI18n()
const inventoryStore = useInventoryStore()
const instanceDetailStore = useItemInstanceDetailStore()
const popoverActive = ref(false)
const popoverLoaded = ref(false)
const { supportsHover, isTouchLike } = useDevice()
const dialogStore = useItemPopoverDialogStore()
const longPress = useLongPress(openMobilePopover, { enabled: isTouchLike })
const iconInfo = computed(() => parseIconId(resolveIconIdByItemId(props.itemId)))
const iconType = computed(() => iconInfo.value?.type || "item")
const iconID = computed(() => iconInfo.value?.id || props.itemId)
const fallbackIcon = computed(() => resolveItemFallbackIcon(props.itemId))
const originalItemLabel = computed(() => getLocalizedItemName(t, props.itemId, { fallbackText: t("game.unnamedItem") }))
const instanceCustomData = computed(() => {
  const instanceId = Number(props.instanceId || 0)
  if (instanceId <= 0) return null
  const inventoryInstance = inventoryStore.instances.find((item) => item.instanceId === instanceId)
  if (inventoryInstance?.customData) return inventoryInstance.customData
  const detail = instanceDetailStore.get(instanceId)
  return detail?.instance?.customData || null
})
const instanceCustomName = computed(() => itemCustomName(instanceCustomData.value))
const instanceCustomLabelClass = computed(() => itemCustomLabelClass(instanceCustomData.value))
const forgedSkillBookLabel = computed(() => {
  const instanceId = Number(props.instanceId || 0)
  if (instanceId <= 0) return ""
  const book = inventoryStore.instancesById.get(instanceId)?.forgedSkillBook
  if (!book) return ""
  const runtimeSkill = book.compiledSkill || (book.nameParams ? { id: book.customSkillId, nameParams: book.nameParams } : null)
  const skillName = getLocalizedRuntimeSkillName(t, book.customSkillId, runtimeSkill)
  return t("skillForge.book.displayName", { skill: skillName })
})
const itemLabel = computed(() => {
  if (forgedSkillBookLabel.value) return forgedSkillBookLabel.value
  if (!instanceCustomName.value) return originalItemLabel.value
  return `${instanceCustomName.value}（${originalItemLabel.value}）`
})
const normalizedCount = computed(() => {
  if (typeof props.count === "number") return props.count
  return 0
})
const countDisplay = computed(() => {
  if (props.normalNumberFormat) return formatNumberWithThousands(normalizedCount.value)
  return formatQuantityDisplay(normalizedCount.value)
})

function onPopoverShow() {
  if (!supportsHover.value) return
  popoverLoaded.value = true
  popoverActive.value = true
}

function onPopoverHide() {
  popoverActive.value = false
}

function primePopover() {
  if (!supportsHover.value) return
  popoverLoaded.value = true
}

function openMobilePopover() {
  popoverLoaded.value = true
  dialogStore.open({
    itemId: props.itemId,
    instanceId: props.instanceId
  })
}
</script>

<style scoped>
.item-quick-ref {
  display: inline-flex;
  align-items: center;
  justify-content: start;
}
</style>
