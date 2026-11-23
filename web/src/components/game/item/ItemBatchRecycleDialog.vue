<template>
  <GameDialog
    v-model="visible"
    :title="t('game.batchRecycleConfirmTitle')"
    width="480px"
    class="dialog-mobile-windowed"
    :close-on-click-modal="!submitting"
    :close-on-press-escape="!submitting"
  >
    <div class="space-y-3">
      <div class="text-xs app-muted mb-2">{{ t('game.batchRecycleSelected', { n: items.length }) }}</div>

      <div class="batch-recycle-list">
        <div
          v-for="item in previewItems"
          :key="item.instanceId"
          class="batch-recycle-row"
        >
          <div class="batch-recycle-row__card">
            <Card :item-id="item.templateItemId" :instance-id="item.instanceId" :width="40" :height="40" hide-label />
          </div>
          <div class="batch-recycle-row__name text-sm">
            {{ getLocalizedItemName(t, item.templateItemId, { fallbackText: item.templateItemId }) }}
            <span class="app-muted text-xs"> ×{{ item.quantity }}</span>
          </div>
          <div class="batch-recycle-row__value text-sm">
            <QuickRef item-id="gold_coin" :count="item.totalGold" />
          </div>
        </div>
      </div>

      <div class="batch-recycle-total">
        <span class="font-semibold text-sm">{{ t('game.batchRecycleTotal') }}</span>
        <QuickRef item-id="gold_coin" :count="totalGoldPreview" />
      </div>

      <div v-if="nonRecyclableCount > 0" class="text-xs text-yellow-500">
        {{ t('game.batchRecycleSkipped', { n: nonRecyclableCount }) }}
      </div>
    </div>

    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">{{ t('game.cancel') }}</el-button>
      <el-button
        type="warning"
        :loading="submitting"
        :disabled="previewItems.length === 0"
        @click="confirmRecycle"
      >
        {{ t('game.batchRecycleConfirm') }}
      </el-button>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getItemDefinition } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { getRecycleTemplateMatchKind } from "@composables/game/useInventorySummary"
import { useGameStore } from "@stores/game"
import type { InventoryInstance } from "@/types/equipment"
import Card from "@components/game/item/Card.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = defineProps<{
  modelValue: boolean
  items: InventoryInstance[]
}>()

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "done"): void
}>()

const { t } = useI18n()
const gameStore = useGameStore()

const submitting = ref(false)

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => {
    if (!submitting.value) emit("update:modelValue", value)
  }
})

type PreviewItem = {
  instanceId: number
  templateItemId: string
  quantity: number
  unitPrice: number
  totalGold: number
}

const previewItems = computed<PreviewItem[]>(() => {
  const cfg = gameStore.recycleConfig
  const result: PreviewItem[] = []
  for (const inst of props.items) {
    const matchKind = getRecycleTemplateMatchKind(inst)
    if (matchKind === "not_recyclable") continue
    const base = getItemDefinition(inst.templateItemId)?.economyBaseValue ?? 0
    if (base <= 0) continue
    const coefficient =
      matchKind === "affix_mismatch_only"
        ? cfg.goldCoefficient + cfg.affixMismatchExtraCoefficient - 1
        : cfg.goldCoefficient
    if (coefficient <= 0) continue
    const unitPrice = Math.floor(base * coefficient)
    const totalGold = Math.floor(unitPrice * inst.quantity)
    if (unitPrice <= 0 || totalGold <= 0) continue
    result.push({ instanceId: inst.instanceId, templateItemId: inst.templateItemId, quantity: inst.quantity, unitPrice, totalGold })
  }
  return result
})

const nonRecyclableCount = computed(() => props.items.length - previewItems.value.length)

const totalGoldPreview = computed(() =>
  previewItems.value.reduce((sum, item) => sum + item.totalGold, 0)
)

async function confirmRecycle() {
  if (submitting.value || previewItems.value.length === 0) return
  submitting.value = true
  try {
    const payload = await apiPost<{ ok: boolean; results: unknown[]; totalGold: number }>(
      "/api/items/recycle/batch",
      gameStore.token,
      { items: previewItems.value.map((item) => ({ instanceId: item.instanceId, quantity: item.quantity })) }
    )
    ElMessage.success(t("game.batchRecycleSuccess", { gold: payload.totalGold ?? 0 }))
    emit("done")
    emit("update:modelValue", false)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.recycleFailed"))
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.batch-recycle-list {
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  padding: 4px 0;
}

.batch-recycle-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
}

.batch-recycle-row:not(:last-child) {
  border-bottom: 1px solid var(--app-border);
}

.batch-recycle-row__card { flex-shrink: 0; }

.batch-recycle-row__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.batch-recycle-row__value { flex-shrink: 0; }

.batch-recycle-total {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 4px 0;
  border-top: 1px solid var(--app-border);
}
</style>
