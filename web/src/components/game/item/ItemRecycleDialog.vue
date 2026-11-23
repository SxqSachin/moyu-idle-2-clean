<template>
  <GameDialog
    v-model="visible"
    :title="t('game.recycleDialogTitle')"
    width="420px"
  >
    <div class="space-y-3">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-sm">{{ t("game.recycleQuantityLabel") }}</span>
        <el-input-number v-model="quantityProxy" :min="1" :max="maxQuantity" class="w-full" />
      </div>

      <div class="rounded border border-[var(--app-border)] p-3 text-xs app-muted space-y-2">
        <div class="flex items-center gap-2">
          <span>{{ t("game.recycleTargetLabel") }}:</span>
          <QuickRef :item-id="itemId" :count="quantityProxy" />
        </div>
        <div>{{ t("game.recycleBaseValueLabel") }}: <QuickRef item-id="gold_coin" :count="props.baseValue" /></div>
        <div>{{ t("game.recycleCoefficientLabel") }}: x{{ recycleCoefficient }}</div>
        <div>{{ t("game.recycleUnitPriceLabel") }}: <QuickRef item-id="gold_coin" :count="recycleUnitPrice" /></div>
        <div class="flex items-center gap-2">
          <span>{{ t("game.recycleTotalGoldLabel") }}:</span>
          <QuickRef item-id="gold_coin" :count="recycleTotalGold" />
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
import QuickRef from "@components/game/item/QuickRef.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    itemId: string
    quantity: number
    maxQuantity: number
    baseValue: number
    recycleCoefficient: number
  }>(),
  {
    itemId: "",
    quantity: 1,
    maxQuantity: 1,
    baseValue: 0,
    recycleCoefficient: 1
  }
)

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "update:quantity", value: number): void
  (e: "confirm", quantity: number): void
}>()

const { t } = useI18n()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value)
})

const quantityProxy = computed({
  get: () => clampQuantity(props.quantity),
  set: (value: number) => emit("update:quantity", clampQuantity(value))
})

const recycleUnitPrice = computed(() => Math.max(0, Math.floor(props.baseValue * props.recycleCoefficient)))
const recycleTotalGold = computed(() => recycleUnitPrice.value * quantityProxy.value)
const canConfirm = computed(() => props.itemId && props.maxQuantity > 0 && quantityProxy.value >= 1 && quantityProxy.value <= props.maxQuantity && recycleTotalGold.value > 0)

function clampQuantity(value: number) {
  const quantity = Number(value)
  if (!Number.isFinite(quantity)) return 1
  const floor = Math.max(1, Math.floor(quantity))
  return Math.min(floor, Math.max(1, Math.floor(props.maxQuantity)))
}

function confirm() {
  if (!canConfirm.value) return
  emit("confirm", quantityProxy.value)
}
</script>
