<template>
  <GameDialog
    :model-value="visible"
    :title="t('game.marketBuyOrderDialogTitle')"
    width="420px"
    @update:model-value="onVisibleChange"
  >
    <div class="space-y-3">
      <div class="flex items-center gap-2 text-sm">
        <span>{{ t("game.marketDialogItemLabel") }}:</span>
        <QuickRef v-if="draft.itemId" :item-id="draft.itemId" />
        <span v-else class="app-muted">{{ t("game.marketDialogNoItemSelected") }}</span>
      </div>
      <el-select
        v-if="showItemSelector"
        v-model="draft.itemId"
        class="w-full"
        :placeholder="t('game.marketSelectItem')"
        :disabled="disableItemSelection"
      >
        <el-option
          v-for="option in itemOptions"
          :key="`buy-order-dialog-${option}`"
          :label="itemLabel(option)"
          :value="option"
        />
      </el-select>
      <div class="flex items-center gap-2">
        <span class="text-sm w-24">{{ t("game.marketDialogUnitPrice") }}</span>
        <div class="flex items-center gap-1">
          <el-button class="m-0!" size="small" @click="$emit('adjust-price', 'half')">÷2</el-button>
          <el-button class="m-0!" size="small" @click="$emit('adjust-price', 'minus')">-</el-button>
          <el-input-number v-model="draft.price" :min="Math.max(1, minPrice)" :max="1000000000000" class="w-full" :controls="false" />
          <el-button class="m-0!" size="small" @click="$emit('adjust-price', 'plus')">+</el-button>
          <el-button class="m-0!" size="small" @click="$emit('adjust-price', 'double')">x2</el-button>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-sm w-24">{{ t("game.marketQuantity") }}</span>
        <el-input-number
            :controls="false" 
         v-model="draft.quantity" :min="1" :max="1000000" class="w-full!">
        </el-input-number>
      </div>
      <div class="rounded border border-[var(--app-border)] p-2 text-xs app-muted space-y-1">
        <div>{{ t("game.marketDialogTotalPayment") }}: {{ formatNumberWithThousands(totalPayment) }} {{ t("game.gold") }}</div>
        <div>{{ t("game.marketDialogFeeEstimate") }}: {{ formatNumberWithThousands(feeEstimate) }} {{ t("game.gold") }}</div>
        <div>{{ t("game.marketDialogFrozenPayment") }}: {{ formatNumberWithThousands(frozenTotal) }} {{ t("game.gold") }}</div>
      </div>
      <div class="text-xs app-muted">
        {{ t("game.marketBuyDialogNotice") }}
      </div>
    </div>
    <template #footer>
      <el-button @click="onVisibleChange(false)">{{ t("game.cancel") }}</el-button>
      <el-button type="primary" @click="$emit('confirm')">{{ t("game.confirm") }}</el-button>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import QuickRef from "@components/game/item/QuickRef.vue"
import { formatNumberWithThousands } from "@utils/number-format"
import GameDialog from "@components/game/common/GameDialog.vue"

defineProps<{
  visible: boolean
  draft: { itemId: string; price: number; quantity: number }
  itemOptions: string[]
  showItemSelector: boolean
  disableItemSelection: boolean
  minPrice: number
  totalPayment: number
  feeEstimate: number
  frozenTotal: number
  itemLabel: (itemID: string) => string
}>()

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void
  (e: "adjust-price", op: "half" | "minus" | "plus" | "double"): void
  (e: "confirm"): void
}>()

const { t } = useI18n()

function onVisibleChange(value: boolean) {
  emit("update:visible", value)
}
</script>
