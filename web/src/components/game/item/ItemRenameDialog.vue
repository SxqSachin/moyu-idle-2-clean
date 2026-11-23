<template>
  <GameDialog
    v-model="visible"
    :title="t('game.renameItem')"
    width="360px"
    class="dialog-mobile-windowed"
    :close-on-click-modal="!submitting"
  >
    <div class="rename-dialog-body">
      <el-input
        v-model="draft"
        :maxlength="ITEM_RENAME_MAX_NAME_RUNES"
        show-word-limit
        :disabled="submitting"
        @keyup.enter="confirmRename"
      />
      <div v-if="nameError" class="text-xs text-red-500">{{ nameError }}</div>
      <div class="rename-color-block">
        <div class="rename-cost-label">{{ t("game.itemRenameColor") }}</div>
        <div class="rename-color-list">
          <button
            v-for="color in ITEM_RENAME_LABEL_COLORS"
            :key="color"
            class="rename-color-swatch"
            :class="[itemCustomLabelClassForColor(color), labelColor === color ? 'is-selected' : '']"
            type="button"
            :disabled="submitting"
            @click="labelColor = color"
          >
            {{ normalizedDraft || t("game.renameItem") }}
          </button>
        </div>
      </div>
      <div class="rename-cost-block">
        <div class="rename-cost-label">{{ t("game.itemRenameCost") }}</div>
        <div class="rename-cost-list">
          <QuickRefWithRequire
            v-for="entry in costEntries"
            :key="entry.itemId"
            :item-id="entry.itemId"
            :required-count="entry.count"
          />
        </div>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <el-button :disabled="submitting" @click="visible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="confirmRename">
          {{ t("game.confirm") }}
        </el-button>
      </div>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { ITEM_RENAME_COST, ITEM_RENAME_LABEL_COLORS, ITEM_RENAME_MAX_NAME_RUNES } from "@/config/shared-global-config"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import type { ItemCustomization, ItemInstance } from "@/types/equipment"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import {
  isValidItemCustomName,
  itemCustomLabelClassForColor,
  itemCustomLabelColor,
  itemCustomName
} from "@services/item-customization"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import GameDialog from "@components/game/common/GameDialog.vue"
import QuickRefWithRequire from "./QuickRefWithRequire.vue"

const props = withDefaults(defineProps<{
  modelValue: boolean
  instanceId?: number | null
  customData?: ItemCustomization | null
}>(), {
  instanceId: null,
  customData: null
})

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void
  (event: "renamed", instance: ItemInstance): void
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const instanceDetailStore = useItemInstanceDetailStore()
const { getInventoryQuantity } = useInventorySummary()
const submitting = ref(false)
const draft = ref("")
const labelColor = ref(ITEM_RENAME_LABEL_COLORS[0] || "gold")

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value)
})
const costEntries = computed(() =>
  Object.entries(ITEM_RENAME_COST)
    .filter(([itemId, count]) => itemId.trim() !== "" && Number(count) > 0)
    .map(([itemId, count]) => ({
      itemId,
      count: Math.max(1, Math.floor(Number(count) || 1)),
      owned: getInventoryQuantity(itemId)
    }))
)
const hasEnoughCost = computed(() => costEntries.value.length > 0 && costEntries.value.every((entry) => entry.owned >= entry.count))
const normalizedDraft = computed(() => draft.value.trim())
const nameError = computed(() => {
  const name = normalizedDraft.value
  if (!name) return t("game.itemRenameNameRequired")
  if (Array.from(name).length > ITEM_RENAME_MAX_NAME_RUNES) return t("game.itemRenameNameTooLong", { max: ITEM_RENAME_MAX_NAME_RUNES })
  if (!isValidItemCustomName(name)) return t("game.itemRenameNameInvalid")
  if (name === itemCustomName(props.customData) && labelColor.value === itemCustomLabelColor(props.customData)) return t("game.itemRenameSameName")
  return ""
})
const canSubmit = computed(() =>
  Number(props.instanceId || 0) > 0 &&
  !!gameStore.token &&
  !submitting.value &&
  !nameError.value &&
  hasEnoughCost.value
)

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    draft.value = itemCustomName(props.customData)
    const currentColor = itemCustomLabelColor(props.customData)
    labelColor.value = ITEM_RENAME_LABEL_COLORS.includes(currentColor)
      ? currentColor
      : (ITEM_RENAME_LABEL_COLORS[0] || "gold")
  }
)

async function confirmRename() {
  const instanceId = Number(props.instanceId || 0)
  if (!canSubmit.value || !gameStore.token || instanceId <= 0) return
  submitting.value = true
  try {
    const result = await apiPost<{ instance: ItemInstance }>("/api/items/rename", gameStore.token, {
      instanceId,
      name: normalizedDraft.value,
      labelColor: labelColor.value
    })
    instanceDetailStore.invalidate(instanceId)
    emit("renamed", result.instance)
    ElMessage.success(t("game.itemRenameSuccess"))
    visible.value = false
    try {
      await inventoryStore.refreshAll(gameStore.token)
    } catch {
      ElMessage.warning(t("game.inventoryLoadFailed"))
    }
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.itemRenameFailed"))
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.rename-dialog-body { display: grid; gap: 12px; }
.rename-cost-block, .rename-color-block { display: grid; gap: 8px; }
.rename-cost-label { color: var(--app-muted); font-size: 12px; line-height: 1.35; }
.rename-cost-list, .rename-color-list { display: flex; flex-wrap: wrap; gap: 8px; }
.rename-color-swatch { min-width: 64px; max-width: 112px; height: 30px; padding: 0 10px; overflow: hidden; border: 1px solid var(--app-border); border-radius: 6px; background: var(--app-surface-elevated); cursor: pointer; text-overflow: ellipsis; white-space: nowrap; }
.rename-color-swatch.is-selected { border-color: var(--item-custom-label-color); box-shadow: 0 0 0 2px color-mix(in srgb, var(--item-custom-label-color) 26%, transparent); }
.rename-color-swatch:disabled { cursor: not-allowed; opacity: .68; }
</style>
