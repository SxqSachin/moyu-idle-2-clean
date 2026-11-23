<template>
  <el-popover
    v-model:visible="popoverActive"
    placement="top"
    :width="450"
    trigger="hover"
    :persistent="false"
    :show-after="200"
    @show="onPopoverShow"
    @hide="onPopoverHide"
  >
    <template #reference>
      <el-tag
        :type="hasEnoughInInventory ? 'primary' : 'danger'"
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
          <span class="item-quick-ref-name">{{ itemLabel }}</span>
          <span class="item-quick-ref-count">: {{ formatNumberWithThousands(requiredCount) }}({{ formatNumberWithThousands(inventoryCount) }})</span>
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
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import ItemPopoverContent from "@components/game/item/ItemPopoverContent.vue"
import { useDevice } from "@composables/game/useDevice"
import { useLongPress } from "@composables/game/useLongPress"
import { getLocalizedItemName } from "@services/item-label"
import { parseIconId, resolveIconIdByItemId, resolveItemFallbackIcon } from "@services/item-icons"
import { useItemPopoverDialogStore } from "@stores/item-popover-dialog"
import { formatNumberWithThousands } from "@utils/number-format"
import { useInventorySummary } from "@/composables/game/useInventorySummary"

const props = withDefaults(
  defineProps<{
    itemId: string
    instanceId?: number | null
    requiredCount: number
    availableCount?: number
  }>(),
  {
    instanceId: null,
    count: 0,
  }
)

const { t } = useI18n()
const popoverActive = ref(false)
const popoverLoaded = ref(false)
const { supportsHover, isTouchLike } = useDevice()
const dialogStore = useItemPopoverDialogStore()
const longPress = useLongPress(openMobilePopover, { enabled: isTouchLike })
const iconInfo = computed(() => parseIconId(resolveIconIdByItemId(props.itemId)))
const iconType = computed(() => iconInfo.value?.type || "item")
const iconID = computed(() => iconInfo.value?.id || props.itemId)
const fallbackIcon = computed(() => resolveItemFallbackIcon(props.itemId))
const itemLabel = computed(() => getLocalizedItemName(t, props.itemId, { fallbackText: t("game.unnamedItem") }))

const { getInventoryQuantity }= useInventorySummary();

const inventoryCount = computed(() => {
  if (props.availableCount !== undefined) return props.availableCount
  return  getInventoryQuantity(props.itemId)
});

const hasEnoughInInventory = computed(() => {
  return inventoryCount.value >= props.requiredCount
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
