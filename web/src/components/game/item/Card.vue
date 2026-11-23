<template>
  <el-popover
    v-if="popoverEnabled"
    v-model:visible="popoverActive"
    placement="top"
    :width="450"
    trigger="manual"
    :persistent="false"
    @show="onPopoverShow"
    @hide="onPopoverHide"
  >
    <template #reference>
      <ItemCardFace
        v-bind="faceProps"
        class="lp-suppress"
        @click="onCardClick"
        @mouseenter="showPopoverFromReference"
        @mousemove="showPopoverFromReferenceIntent"
        @mouseleave="hidePopoverFromReference"
        @focusin="showPopoverFromReferenceByFocus"
        @pointerdown="longPress.onPointerDown"
        @pointermove="longPress.onPointerMove"
        @pointerup="longPress.onPointerUp"
        @pointercancel="longPress.onPointerUp"
        @pointerleave="longPress.onPointerUp"
        @contextmenu="longPress.onContextMenu"
      />
    </template>
    <div
      v-if="popoverLoaded"
      ref="popoverContentEl"
      @mouseenter="showPopoverFromContent"
      @mouseleave="hidePopoverFromContent"
    >
      <ItemPopoverContent
        :item-id="itemId"
        :instance-id="instanceId"
        :public-instance-preview="instancePreview || undefined"
        :requirement-attributes="requirementAttributes"
        :active="popoverActive"
        :fix-height="true"
      />
    </div>
  </el-popover>
  <ItemCardFace
    v-else
    v-bind="faceProps"
    class="lp-suppress"
    @click="onCardClick"
    @mouseenter="showPopoverFromReference"
    @mousemove="showPopoverFromReferenceIntent"
    @mouseleave="hidePopoverFromReference"
    @focusin="showPopoverFromReferenceByFocus"
    @pointerdown="longPress.onPointerDown"
    @pointermove="longPress.onPointerMove"
    @pointerup="longPress.onPointerUp"
    @pointercancel="longPress.onPointerUp"
    @pointerleave="longPress.onPointerUp"
    @contextmenu="longPress.onContextMenu"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import ItemCardFace from "@components/game/item/ItemCardFace.vue"
import ItemPopoverContent from "@components/game/item/ItemPopoverContent.vue"
import { useDevice } from "@composables/game/useDevice"
import { useLongPress } from "@composables/game/useLongPress"
import { getEquipmentTemplateDefinition, getItemDefinition, getRarityDefinition } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { parseIconId, resolveIconIdByItemId, resolveItemFallbackIcon } from "@services/item-icons"
import { getAffixSourceBadgeDefinitions } from "@services/affix-source-badges"
import { getLocalizedRuntimeSkillName, getSkillDefinition, getSkillTypeDefinition } from "@services/skill-definition"
import { itemCustomLabelClass, itemCustomName } from "@services/item-customization"
import { getFishSpeciesDisplay, localizedFishText } from "@services/fish-instance-display"
import { useInventoryStore } from "@stores/inventory"
import { useItemPopoverDialogStore, type ItemPopoverDialogPublicInstancePreview } from "@stores/item-popover-dialog"
import { useEquipmentStore } from "@stores/equipment"
import type { EquipmentAffixSlot, FishInstanceData } from "@/types/equipment"

type CardInstancePreview = ItemPopoverDialogPublicInstancePreview & {
  affixes?: EquipmentAffixSlot[]
  fishData?: FishInstanceData
}

const POPOVER_SHOW_DELAY = 400
const POPOVER_HIDE_DELAY = 300
const HOVER_INTENT_MOVE_GRACE_MS = POPOVER_SHOW_DELAY + 200
const HOVER_INTENT_SCROLL_SUPPRESS_MS = 500

let hoverIntentTrackingInitialized = false
let lastGlobalPointerMoveAt = 0
let lastGlobalScrollAt = 0

function markGlobalPointerMove(event?: Event) {
  if (typeof PointerEvent !== "undefined" && event instanceof PointerEvent && event.pointerType !== "mouse") return
  lastGlobalPointerMoveAt = Date.now()
}

function markGlobalScroll() {
  lastGlobalScrollAt = Date.now()
}

function ensureHoverIntentTracking() {
  if (hoverIntentTrackingInitialized || typeof window === "undefined") return
  hoverIntentTrackingInitialized = true
  window.addEventListener("pointermove", markGlobalPointerMove, { capture: true, passive: true })
  window.addEventListener("mousemove", markGlobalPointerMove, { capture: true, passive: true })
  window.addEventListener("scroll", markGlobalScroll, { capture: true, passive: true })
  window.addEventListener("wheel", markGlobalScroll, { capture: true, passive: true })
}

function hasRecentPointerHoverIntent() {
  const now = Date.now()
  if (lastGlobalPointerMoveAt <= 0) return false
  if (lastGlobalPointerMoveAt < lastGlobalScrollAt) return false
  if (now - lastGlobalPointerMoveAt > HOVER_INTENT_MOVE_GRACE_MS) return false
  if (now - lastGlobalScrollAt <= HOVER_INTENT_SCROLL_SUPPRESS_MS && lastGlobalPointerMoveAt <= lastGlobalScrollAt) return false
  return true
}

const props = withDefaults(
  defineProps<{
    itemId: string
    instanceId?: number | null
    instancePreview?: CardInstancePreview | null
    quantity?: number
    hideBorder?: boolean
    hideLabel?: boolean
    showZeroQuantity?: boolean
    previewEnabled?: boolean
    width?: number
    height?: number
    iconSize?: number
    requirementAttributes?: Record<string, number> | null
  }>(),
  {
    instanceId: null,
    instancePreview: null,
    quantity: 0,
    showZeroQuantity: false,
    previewEnabled: true,
    requirementAttributes: null,
  }
)
const emit = defineEmits<{
  (e: "click", event: MouseEvent): void
}>()

const { t, locale } = useI18n()
const inventoryStore = useInventoryStore()
const equipmentStore = useEquipmentStore()
const popoverActive = ref(false)
const popoverLoaded = ref(false)
const popoverEnabled = ref(false)
const popoverContentEl = ref<HTMLElement | null>(null)
let pendingShowTimer: ReturnType<typeof setTimeout> | null = null
let pendingHideTimer: ReturnType<typeof setTimeout> | null = null
let referenceHovered = false
let contentHovered = false
let globalDismissListenersAttached = false
const { supportsHover, isTouchLike } = useDevice()
const dialogStore = useItemPopoverDialogStore()
const previewInteractionEnabled = computed(() => props.previewEnabled && isTouchLike.value)
const longPress = useLongPress(openMobilePopover, { enabled: previewInteractionEnabled })

const pIconSize = computed(() => props.iconSize || 32)
const iconInfo = computed(() => parseIconId(resolveIconIdByItemId(props.itemId)))
const iconType = computed(() => iconInfo.value?.type || "item")
const iconID = computed(() => iconInfo.value?.id || props.itemId)
const fallbackIcon = computed(() => resolveItemFallbackIcon(props.itemId))
const originalItemLabel = computed(() => getLocalizedItemName(t, props.itemId, { fallbackText: t("game.unnamedItem") }))
const itemDefinition = computed(() => getItemDefinition(props.itemId))
const equipmentTemplate = computed(() => {
  if (!itemDefinition.value?.isEquipment) return null
  return getEquipmentTemplateDefinition(props.itemId)
})
const isSkillbook = computed(() => {
  const item = itemDefinition.value
  if (!item) return false
  const itemTypes = Array.isArray(item.type) ? item.type : []
  return itemTypes.includes("skillbook")
})
const skillbookOverlaySkillID = computed(() => {
  if (!isSkillbook.value) return ""
  return String(itemDefinition.value?.useGrantSkillId || "").trim()
})
const skillbookOverlayIcon = computed(() => {
  if (!isSkillbook.value) return ""
  const skillID = skillbookOverlaySkillID.value
  if (!skillID) return ""
  return String(getSkillDefinition(skillID)?.icon || "").trim()
})
const skillbookCategoryColor = computed(() => {
  if (!isSkillbook.value) return ""
  const categoryID = String(getSkillDefinition(skillbookOverlaySkillID.value)?.category || "").trim()
  if (!categoryID) return ""
  return String(getSkillTypeDefinition(categoryID)?.color || "").trim()
})
const skillbookOverlayStyle = computed(() => {
  const color = skillbookCategoryColor.value || "#FFFFFF"
  return {
    color,
    border: `1px solid ${color}66`,
    backgroundColor: "rgba(0, 0, 0, 0.55)",
    // transform: "skew(35deg, 325deg) translate(2px, -2px)"
  }
})

const inventoryInstance = computed(() => {
  if (props.instanceId === null || props.instanceId === undefined) return null
  return inventoryStore.instancesById.get(props.instanceId) || null
})
const fishData = computed(() => props.instancePreview?.fishData || inventoryInstance.value?.fishData || null)
const fishSpecies = computed(() => getFishSpeciesDisplay(fishData.value?.speciesId || ""))
const fishSpeciesName = computed(() => localizedFishText(fishSpecies.value?.names, locale.value, originalItemLabel.value))
const itemRarityColor = computed(() => {
  const rarityId = fishSpecies.value?.rarity || getItemDefinition(props.itemId)?.rarity || "common"
  return getRarityDefinition(rarityId)?.color || "#9CA3AF"
})
const customItemName = computed(() => {
  const fromPreview = itemCustomName(props.instancePreview?.customData)
  if (fromPreview) return fromPreview
  return itemCustomName(inventoryInstance.value?.customData)
})
const customLabelClass = computed(() => {
  if (itemCustomName(props.instancePreview?.customData)) return itemCustomLabelClass(props.instancePreview?.customData)
  return itemCustomLabelClass(inventoryInstance.value?.customData)
})
const forgedSkillBookLabel = computed(() => {
  const book = inventoryInstance.value?.forgedSkillBook
  if (!book) return ""
  const runtimeSkill = book.compiledSkill || (book.nameParams ? { id: book.customSkillId, nameParams: book.nameParams } : null)
  const skillName = getLocalizedRuntimeSkillName(t, book.customSkillId, runtimeSkill)
  return t("skillForge.book.displayName", { skill: skillName })
})
const itemLabel = computed(() => fishData.value ? customItemName.value || fishSpeciesName.value : forgedSkillBookLabel.value || customItemName.value || originalItemLabel.value)
const previewAffixes = computed(() => {
  if (Array.isArray(props.instancePreview?.affixes)) return props.instancePreview?.affixes || []
  return inventoryInstance.value?.affixes || []
})
const sourceBadges = computed(() => {
  const affixes = previewAffixes.value || []
  return getAffixSourceBadgeDefinitions(affixes).map((badge) => ({ ...badge, title: t(badge.titleKey) }))
})

const templateAffixes = computed(() => {
  if (!itemDefinition.value?.isEquipment || !equipmentTemplate.value) {
    return[]
  }

  if ((equipmentTemplate.value.defaultAffixSlots?.length ?? 0) > 0) {
    return equipmentTemplate.value.defaultAffixSlots || []
  }

  return []
})

const enhanceLevelBadge = computed(() => {
  const instanceID = Number(props.instanceId || 0)
  if (instanceID <= 0) return null

  if (Number(props.instancePreview?.enhanceLevel || 0) > 0) {
    return Number(props.instancePreview?.enhanceLevel || 0)
  }
  if (inventoryInstance.value && Number(inventoryInstance.value.enhanceLevel || 0) > 0) {
    return Number(inventoryInstance.value.enhanceLevel)
  }
  const equippedInstance = equipmentStore.equippedItems.find((item) => item.instanceId === instanceID)
  if (equippedInstance && Number(equippedInstance.enhanceLevel || 0) > 0) {
    return Number(equippedInstance.enhanceLevel)
  }
  return null
})

const faceProps = computed(() => ({
  itemLabel: itemLabel.value,
  itemRarityColor: itemRarityColor.value,
  iconType: iconType.value,
  iconID: iconID.value,
  fallbackIcon: fallbackIcon.value,
  pIconSize: pIconSize.value,
  isSkillbook: isSkillbook.value,
  skillbookOverlayIcon: skillbookOverlayIcon.value,
  skillbookOverlayStyle: skillbookOverlayStyle.value,
  quantity: props.quantity,
  showZeroQuantity: props.showZeroQuantity,
  enhanceLevelBadge: enhanceLevelBadge.value,
  sourceBadges: sourceBadges.value,
  previewAffixes: previewAffixes.value,
  templateAffixes: templateAffixes.value,
  customIconSrc: fishSpecies.value?.iconPath || "",
  fishData: fishData.value,
  labelClass: customLabelClass.value,
  instanceId: props.instanceId,
  width: props.width,
  height: props.height,
  hideBorder: props.hideBorder,
  hideLabel: props.hideLabel
}))

// 桌面端首次悬停/聚焦时才挂载 el-popover，避免开局批量实例化数百个 popover。
// 移动端（!supportsHover）不会触发，el-popover 永不挂载，交互走 longPress -> dialogStore。
function showPopoverFromReference() {
  if (!props.previewEnabled || !supportsHover.value) return
  ensureHoverIntentTracking()
  referenceHovered = true
  cancelPendingHide()
  if (hasRecentPointerHoverIntent()) {
    schedulePopoverShow()
  }
}

function showPopoverFromReferenceIntent(event: MouseEvent) {
  if (!props.previewEnabled || !supportsHover.value) return
  ensureHoverIntentTracking()
  markGlobalPointerMove(event)
  referenceHovered = true
  cancelPendingHide()
  schedulePopoverShow()
}

function showPopoverFromReferenceByFocus() {
  if (!props.previewEnabled || !supportsHover.value) return
  referenceHovered = true
  cancelPendingHide()
  schedulePopoverShow({ bypassHoverIntent: true })
}

function schedulePopoverShow(options: { bypassHoverIntent?: boolean } = {}) {
  if (!props.previewEnabled || !supportsHover.value) return
  if (!options.bypassHoverIntent && !hasRecentPointerHoverIntent()) return
  popoverLoaded.value = true
  popoverEnabled.value = true
  if (pendingShowTimer !== null) return
  attachGlobalDismissListeners()
  pendingShowTimer = setTimeout(() => {
    pendingShowTimer = null
    if (!referenceHovered || (!options.bypassHoverIntent && !hasRecentPointerHoverIntent())) {
      if (!popoverActive.value) detachGlobalDismissListeners()
      return
    }
    popoverActive.value = true
    attachGlobalDismissListeners()
  }, POPOVER_SHOW_DELAY)
}

function hidePopoverFromReference() {
  referenceHovered = false
  schedulePopoverHide()
}

function showPopoverFromContent() {
  if (!supportsHover.value) return
  contentHovered = true
  cancelPendingHide()
}

function hidePopoverFromContent() {
  contentHovered = false
  schedulePopoverHide()
}

function cancelPendingShow() {
  if (pendingShowTimer === null) return
  clearTimeout(pendingShowTimer)
  pendingShowTimer = null
}

function cancelPendingHide() {
  if (pendingHideTimer === null) return
  clearTimeout(pendingHideTimer)
  pendingHideTimer = null
}

function schedulePopoverHide() {
  cancelPendingShow()
  cancelPendingHide()
  pendingHideTimer = setTimeout(() => {
    pendingHideTimer = null
    if (referenceHovered || contentHovered) return
    popoverActive.value = false
    detachGlobalDismissListeners()
  }, POPOVER_HIDE_DELAY)
}

function closePopover() {
  referenceHovered = false
  contentHovered = false
  cancelPendingShow()
  cancelPendingHide()
  popoverActive.value = false
  detachGlobalDismissListeners()
}

function handleGlobalPopoverDismiss(event: Event) {
  if ((event.type === "scroll" || event.type === "wheel") && isEventFromPopoverContent(event)) return
  closePopover()
}

function isEventFromPopoverContent(event: Event) {
  if (!contentHovered || !popoverContentEl.value) return false
  const target = event.target
  return target instanceof Node && popoverContentEl.value.contains(target)
}

function attachGlobalDismissListeners() {
  if (globalDismissListenersAttached || typeof window === "undefined") return
  globalDismissListenersAttached = true
  window.addEventListener("scroll", handleGlobalPopoverDismiss, true)
  window.addEventListener("wheel", handleGlobalPopoverDismiss, true)
  window.addEventListener("blur", handleGlobalPopoverDismiss)
}

function detachGlobalDismissListeners() {
  if (!globalDismissListenersAttached || typeof window === "undefined") return
  globalDismissListenersAttached = false
  window.removeEventListener("scroll", handleGlobalPopoverDismiss, true)
  window.removeEventListener("wheel", handleGlobalPopoverDismiss, true)
  window.removeEventListener("blur", handleGlobalPopoverDismiss)
}

onBeforeUnmount(() => {
  closePopover()
})

watch(
  () => [props.itemId, props.instanceId],
  () => closePopover()
)

function onCardClick(event: MouseEvent) {
  if (longPress.consumeTriggeredClick(event)) return
  closePopover()
  emit("click", event)
}

function onPopoverShow() {
  if (!supportsHover.value) return
  popoverLoaded.value = true
  popoverActive.value = true
  attachGlobalDismissListeners()
}

function onPopoverHide() {
  popoverActive.value = false
  detachGlobalDismissListeners()
}

function openMobilePopover() {
  if (!props.previewEnabled) return
  popoverLoaded.value = true
  dialogStore.open({
    itemId: props.itemId,
    instanceId: props.instanceId,
    publicInstancePreview: props.instancePreview || undefined,
    requirementAttributes: props.requirementAttributes || undefined
  })
}
</script>
