<template>
  <GameDialog
    :model-value="modelValue"
    :title="title"
    width="560px"
    @update:model-value="(value) => emit('update:modelValue', value)"
  >
    <div class="space-y-2">
      <div
        v-if="kitties.length <= 0"
        class="rounded border border-dashed border-[var(--app-border)] p-3 text-xs app-muted"
      >
        {{ t("game.catteryNoKitty") }}
      </div>
      <div v-else class="max-h-[420px] space-y-2 overflow-auto pr-1">
        <div
          v-for="kitty in kitties"
          :key="`room-kitty-picker-${kitty.id}`"
          class="rounded border border-[var(--app-border)] bg-[var(--app-surface)] p-3"
        >
          <div class="flex items-start justify-between gap-2">
            <div>
              <div class="font-medium">{{ kitty.name || `#${kitty.id}` }}</div>
              <div class="mt-1 text-xs app-muted">Lv.{{ kitty.level || 1 }} · {{ kitty.statusLabel }}</div>
            </div>
            <el-tag v-if="kitty.inRoom" size="small" type="success">
              {{ t("game.battleRoomKittyInRoomTag") }}
            </el-tag>
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <el-select
              :model-value="resolvedPosition(kitty)"
              size="small"
              class="w-36"
              :disabled="kitty.inRoom"
              @update:model-value="(value) => updatePosition(kitty.id, String(value || 'vanguard'))"
            >
              <el-option
                v-for="option in positionOptions"
                :key="`kitty-picker-position-${kitty.id}-${option}`"
                :label="positionLabel(option)"
                :value="option"
              />
            </el-select>
            <el-button
              size="small"
              type="primary"
              :disabled="kitty.inRoom || Boolean(kitty.disabledReason)"
              @click="emit('add-kitty', { kittyId: kitty.id, status: kitty.status, position: resolvedPosition(kitty) })"
            >
              {{ t("game.battleRoomAddKittyAction") }}
            </el-button>
          </div>
          <div v-if="kitty.disabledReason" class="mt-2 text-xs app-muted">
            {{ kitty.disabledReason }}
          </div>
        </div>
      </div>
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import GameDialog from "@components/game/common/GameDialog.vue"

type KittyPickerItem = {
  id: number
  name: string
  level: number
  status: string
  statusLabel: string
  inRoom: boolean
  disabledReason: string
  defaultPosition: string
}

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  kitties: KittyPickerItem[]
  positionOptions: string[]
  positionLabel: (position: string) => string
}>(), {
  title: ""
})

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "add-kitty", payload: { kittyId: number; status: string; position: string }): void
}>()

const { t } = useI18n()
const positions = ref<Record<number, string>>({})

watch(
  () => props.kitties,
  (rows) => {
    const next = { ...positions.value }
    for (const kitty of rows) {
      if (!next[kitty.id]) {
        next[kitty.id] = kitty.defaultPosition || "vanguard"
      }
    }
    positions.value = next
  },
  { immediate: true, deep: true }
)

function updatePosition(kittyID: number, position: string) {
  positions.value = {
    ...positions.value,
    [kittyID]: position || "vanguard"
  }
}

function resolvedPosition(kitty: KittyPickerItem) {
  const current = positions.value[kitty.id]
  if (current) return current
  return kitty.defaultPosition || "vanguard"
}
</script>
