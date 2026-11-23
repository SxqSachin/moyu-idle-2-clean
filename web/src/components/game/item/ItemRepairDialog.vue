<template>
  <GameDialog
    v-model="visible"
    :title="t('game.repairDialogTitle')"
    width="420px"
  >
    <div class="text-sm">{{ t("game.operationConfirmPrompt") }}</div>
    <template #footer>
      <el-button @click="visible = false">{{ t("game.cancel") }}</el-button>
      <el-button type="primary" @click="confirm">{{ t("game.confirm") }}</el-button>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "confirm"): void
}>()

const { t } = useI18n()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value)
})

function confirm() {
  emit("confirm")
}
</script>
