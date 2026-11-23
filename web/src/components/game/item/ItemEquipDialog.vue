<template>
  <GameDialog
    v-model="visible"
    :title="t('game.equipDialogTitle')"
    width="420px"
  >
    <div class="space-y-3">
      <div v-if="showTargetSelect && targetOptions.length > 1" class="space-y-2">
        <div class="text-sm">{{ t("game.itemOperationTargetLabel") }}</div>
        <el-select v-model="selectedTargetProxy" class="w-full">
          <el-option
            v-for="target in targetOptions"
            :key="target.key"
            :label="target.label"
            :value="target.key"
          />
        </el-select>
      </div>
      <div class="text-sm">{{ t("game.equipSlotLabel") }}</div>
      <el-select v-model="selectedSlotProxy" class="w-full">
        <el-option v-for="slot in slots" :key="slot" :label="localizeEquipSlot(slot)" :value="slot" />
      </el-select>
      <div v-if="requirementRows.length > 0" class="space-y-1 rounded border border-[var(--app-border)] px-3 py-2">
        <div class="text-sm">{{ t("game.statusRequirements") }}</div>
        <div
          v-for="row in requirementRows"
          :key="row.attr"
          class="flex items-center justify-between text-xs"
          :class="row.met ? 'text-emerald-300' : 'text-rose-300'"
        >
          <status-require-tag :attr-key="row.attr" :required-value="row.required" :current-value="row.current"></status-require-tag>
          <!-- <span>{{ row.name }}</span>
          <span>{{ Number(row.current.toFixed(2)) }} / {{ row.required }}</span> -->
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">{{ t("game.cancel") }}</el-button>
      <el-button type="primary" :disabled="!canConfirm" @click="confirm">{{ t("game.confirm") }}</el-button>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import StatusRequireTag from "../attribute/StatusRequireTag.vue";
import GameDialog from "@components/game/common/GameDialog.vue"

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    slots: string[]
    selectedSlot: string
    selectedTarget?: string
    targetOptions?: Array<{ key: string; label: string }>
    showTargetSelect?: boolean
    requirementRows?: Array<{ attr: string; name: string; current: number; required: number; met: boolean }>
    disabled?: boolean
  }>(),
  {
    selectedSlot: "",
    selectedTarget: "player",
    targetOptions: () => [],
    showTargetSelect: false,
    requirementRows: () => [],
    disabled: false
  }
)

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "update:selectedSlot", value: string): void
  (e: "update:selectedTarget", value: string): void
  (e: "confirm", slot: string): void
}>()

const { t } = useI18n()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value)
})

const selectedSlotProxy = computed({
  get: () => props.selectedSlot,
  set: (value: string) => emit("update:selectedSlot", value)
})

const selectedTargetProxy = computed({
  get: () => props.selectedTarget || "player",
  set: (value: string) => emit("update:selectedTarget", value)
})

const canConfirm = computed(() => !props.disabled && props.slots.includes(selectedSlotProxy.value))

function confirm() {
  if (!canConfirm.value) return
  emit("confirm", selectedSlotProxy.value)
}

function localizeEquipSlot(slot: string) {
  const key = `game.equipSlot.${slot}`
  const localized = t(key)
  return localized === key ? slot : localized
}
</script>
