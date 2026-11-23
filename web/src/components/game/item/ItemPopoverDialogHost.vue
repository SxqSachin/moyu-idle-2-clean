<template>
  <GameDialog
    v-model="dialogVisible"
    class="item-popover-dialog dialog-mobile-windowed"
    :width="dialogWidth"
    destroy-on-close
  >
    <ItemPopoverContent
      :item-id="dialogStore.itemId"
      :instance-id="dialogStore.instanceId"
      :public-instance-preview="dialogStore.publicInstancePreview || undefined"
      :requirement-attributes="dialogStore.requirementAttributes || undefined"
      :active="dialogVisible"
      :fix-height="true"
    />
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useDevice } from "@composables/game/useDevice"
import ItemPopoverContent from "@components/game/item/ItemPopoverContent.vue"
import { useItemPopoverDialogStore } from "@stores/item-popover-dialog"
import GameDialog from "@components/game/common/GameDialog.vue"

const dialogStore = useItemPopoverDialogStore()
const { isMobile } = useDevice()

const dialogVisible = computed({
  get() {
    return dialogStore.visible
  },
  set(value: boolean) {
    if (!value) dialogStore.close()
  }
})

const dialogWidth = computed(() => {
  if (isMobile.value) return "92%"
  return "520px"
})
</script>

<style scoped>
.item-popover-dialog :deep(.el-dialog__body) {
  padding-top: 12px;
}

.item-popover-dialog :deep(.el-dialog__headerbtn) {
  top: 4px;
  right: 4px;
}
</style>
