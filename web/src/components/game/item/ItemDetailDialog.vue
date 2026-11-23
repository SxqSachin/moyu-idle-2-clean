<template>
  <GameDialog
    v-model="visible"
    class="item-detail-dialog"
    :title="t('game.itemDetailTitle')"
    :width="dialogWidth"
  >
    <div data-tutorial="warehouse-item-detail-dialog">
      <ItemDetailPanel
        :item-id="itemId"
        :instance-id="instanceId"
        :active="visible"
        @updated="emit('updated')"
        @request-close="visible = false"
      />
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import ItemDetailPanel from "./ItemDetailPanel.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = defineProps<{
  modelValue: boolean
  itemId: string
  instanceId?: number | null
}>()

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "updated"): void
}>()

const { t } = useI18n()
const { isMobile } = useDevice()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value)
})

const dialogWidth = computed(() => {
  if (isMobile.value) return "92%"
  return "480px"
})
</script>
