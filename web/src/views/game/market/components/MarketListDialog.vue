<template>
  <GameDialog
    :model-value="visible"
    :title="t('game.marketListDialogTitle')"
    width="420px"
    align-center
    :draggable="true"
    @update:model-value="onVisibleChange"
  >
    <div data-tutorial="market-list-dialog">
      <el-form label-position="top" class="game-form">
        <el-form-item :label="t('game.marketDialogItemLabel')">
          <div class="w-full">
            <div v-if="selectedOption?.itemId" class="mb-2">
              <QuickRef :item-id="selectedOption.itemId" />
            </div>
            <div v-else class="mb-2 text-sm app-muted">{{ t("game.marketDialogNoItemSelected") }}</div>
            <el-select v-if="showItemSelector" v-model="draft.selectedKey" class="w-full" :placeholder="t('game.marketSelectItem')">
              <el-option v-for="option in options" :key="option.key" :label="option.label" :value="option.key" />
            </el-select>
          </div>
        </el-form-item>

        <el-form-item :label="t('game.marketDialogUnitPrice')">
          <div class="flex items-center gap-1 w-full">
            <el-button size="small" @click="$emit('adjust-price', 'half')">÷2</el-button>
            <el-button size="small" @click="$emit('adjust-price', 'minus')">-</el-button>
            <el-input-number v-model="draft.price" :min="Math.max(1, minPrice)" :max="1000000000000" class="flex-1" :controls="false" />
            <el-button size="small" @click="$emit('adjust-price', 'plus')">+</el-button>
            <el-button size="small" @click="$emit('adjust-price', 'double')">x2</el-button>
          </div>
        </el-form-item>

        <el-form-item v-if="selectedOption?.mode === 'stack'" :label="t('game.marketQuantity')">
          <div class="w-full space-y-2">
            <el-input-number
              :controls="false"
              v-model="draft.quantity"
              :min="1"
              :max="Math.max(1, selectedOption?.maxQuantity || 1)"
              class="w-full!"
            >
              <template #suffix>
                <span class="text-xs app-muted">/ {{ selectedOption?.maxQuantity || 1 }}</span>
              </template>
            </el-input-number>
            <div class="flex flex-wrap gap-1 justify-end">
              <el-button size="small" @click="$emit('quick-quantity', 10)">+10</el-button>
              <el-button size="small" @click="$emit('quick-quantity', 100)">+100</el-button>
              <el-button size="small" @click="$emit('quick-quantity', Math.floor((selectedOption?.maxQuantity || 1) * 0.5))">50%</el-button>
              <el-button size="small" @click="$emit('quick-quantity', selectedOption?.maxQuantity || 1)">100%</el-button>
            </div>
          </div>
        </el-form-item>

        <div class="game-form__section text-xs space-y-1">
          <div class="flex justify-between">
            <span class="app-muted">{{ t("game.marketDialogGrossIncome") }}</span>
            <span>{{ formatNumberWithThousands(grossIncome) }} {{ t("game.gold") }}</span>
          </div>
          <div class="flex justify-between">
            <span class="app-muted">{{ t("game.marketDialogFeeEstimate") }}</span>
            <span>{{ formatNumberWithThousands(feeEstimate) }} {{ t("game.gold") }}</span>
          </div>
          <div class="flex justify-between font-semibold">
            <span>{{ t("game.marketDialogNetIncome") }}</span>
            <span style="color: var(--game-gold)">{{ formatNumberWithThousands(netIncome) }} {{ t("game.gold") }}</span>
          </div>
        </div>

        <div class="text-xs app-muted">
          {{ t("game.marketListDialogNotice") }}
        </div>
      </el-form>
    </div>
    <template #footer>
      <el-button @click="onVisibleChange(false)">{{ t("game.cancel") }}</el-button>
      <el-button type="primary" :disabled="confirmDisabled" @click="$emit('confirm')">{{ t("game.confirm") }}</el-button>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import QuickRef from "@components/game/item/QuickRef.vue"
import { formatNumberWithThousands } from "@utils/number-format"
import GameDialog from "@components/game/common/GameDialog.vue"

type ListableOption = {
  key: string
  itemId: string
  label: string
  mode: "stack"
  maxQuantity: number
}

defineProps<{
  visible: boolean
  draft: { selectedKey: string; price: number; quantity: number }
  options: ListableOption[]
  selectedOption: ListableOption | null
  showItemSelector: boolean
  minPrice: number
  grossIncome: number
  feeEstimate: number
  netIncome: number
  confirmDisabled?: boolean
}>()

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void
  (e: "adjust-price", op: "half" | "minus" | "plus" | "double"): void
  (e: "quick-quantity", value: number): void
  (e: "confirm"): void
}>()

const { t } = useI18n()

function onVisibleChange(value: boolean) {
  emit("update:visible", value)
}
</script>
