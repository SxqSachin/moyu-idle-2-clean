<template>
  <GameDialog :model-value="visible" :title="t('game.auctionCreateTitle')" width="500px" @update:model-value="$emit('update:visible', $event)">
    <el-form label-position="top" class="game-form" data-tutorial="auction-create-dialog">
      <fieldset class="game-form__section" data-tutorial="auction-create-item">
        <legend class="game-form__legend">{{ t("game.marketSelectedItem") }}</legend>
        <el-form-item :label="t('game.marketDialogItemLabel')">
          <div>
            <Card v-if="selectedInstanceItemId" :item-id="selectedInstanceItemId" :instance-id="draft.instanceId" @click="$emit('open-instance-selector')" />
            <div v-else class="text-sm app-muted">{{ t("game.marketDialogNoItemSelected") }}</div>
          </div>
        </el-form-item>
      </fieldset>

      <fieldset class="game-form__section" data-tutorial="auction-create-pricing">
        <legend class="game-form__legend">{{ t("game.marketPrice") }}</legend>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <el-form-item :label="t('game.auctionStartPrice')">
            <el-input-number v-model="draft.startPrice" :min="Math.max(1, minStartPrice + 1)" :max="1000000000000" class="w-full" />
          </el-form-item>
          <el-form-item :label="t('game.auctionMinBidIncrement')">
            <el-input-number
              v-model="draft.minBidIncrement"
              :min="minBidIncrement"
              :max="maxBidIncrement"
              :step="1"
              :precision="0"
              class="w-full"
            />
          </el-form-item>
        </div>
        <div class="text-xs app-muted mb-2">
          {{ t("game.auctionMinBidIncrementRangeHint") }}:
          <QuickRef item-id="gold_coin" :count="minBidIncrement" normal-number-format />
          <span> ~ </span>
          <QuickRef item-id="gold_coin" :count="maxBidIncrement" normal-number-format />
        </div>
        <el-form-item data-tutorial="auction-create-buyout">
          <template #label>
            <span class="flex items-center gap-1">
              {{ t("game.auctionBuyoutPrice") }}
              <el-tooltip :content="t('game.auctionBuyoutHint')" placement="top">
                <span class="inline-flex cursor-help text-[var(--app-muted)]"><el-icon><Icon icon="ep:question-filled" /></el-icon></span>
              </el-tooltip>
            </span>
          </template>
          <div class="flex items-center gap-2 w-full">
            <el-input-number v-model="draft.buyoutPrice" :min="0" :max="1000000000000" class="flex-1" />
            <el-button size="small" @click="draft.buyoutPrice = 0">{{ t("game.auctionBuyoutUnset") }}</el-button>
          </div>
        </el-form-item>
        <div class="text-xs app-muted">
          {{ t("game.auctionMinBuyoutPrice") }}:
          <QuickRef item-id="gold_coin" :count="minBuyoutPrice" normal-number-format />
        </div>
      </fieldset>

      <fieldset class="game-form__section" data-tutorial="auction-create-settlement">
        <legend class="game-form__legend">{{ t("game.auctionDurationHours") }}</legend>
        <el-form-item :label="t('game.auctionDurationHours')">
          <el-input-number v-model="draft.durationHours" :min="4" :max="72" :step="1" :precision="0" class="w-56!" />
        </el-form-item>

        <div class="game-form__section text-xs space-y-1 mt-3">
          <div class="flex justify-between">
            <span class="app-muted">{{ t("game.auctionStartPrice") }} {{ t("game.marketFloorHint") }}</span>
            <QuickRef item-id="gold_coin" :count="minStartPrice + 1" normal-number-format />
          </div>
          <div class="flex justify-between">
            <span class="app-muted">{{ t("game.marketDialogFeeEstimate") }} ({{ feeRateBps / 100 }}%)</span>
            <QuickRef item-id="gold_coin" :count="estimatedFee" normal-number-format />
          </div>
          <div class="flex justify-between font-semibold">
            <span>{{ t("game.marketDialogNetIncome") }}</span>
            <span style="color: var(--game-gold)">
              <QuickRef item-id="gold_coin" :count="estimatedSellerAmount" normal-number-format />
            </span>
          </div>
        </div>
      </fieldset>
    </el-form>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">{{ t("game.cancel") }}</el-button>
      <el-button type="primary" :disabled="!canSubmit" @click="$emit('confirm')">{{ t("game.auctionCreateAction") }}</el-button>
    </template>
  </GameDialog>

</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import Card from "@components/game/item/Card.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

defineProps<{
  visible: boolean
  draft: {
    instanceId: number
    startPrice: number
    minBidIncrement: number
    buyoutPrice: number
    durationHours: number
  }
  selectedInstanceItemId: string
  minStartPrice: number
  minBidIncrement: number
  maxBidIncrement: number
  minBuyoutPrice: number
  feeRateBps: number
  estimatedFee: number
  estimatedSellerAmount: number
  canSubmit: boolean
}>()

defineEmits<{
  (e: "update:visible", value: boolean): void
  (e: "open-instance-selector"): void
  (e: "confirm"): void
}>()

const { t } = useI18n()
</script>
