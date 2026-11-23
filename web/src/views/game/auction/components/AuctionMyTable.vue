<template>
  <el-table :data="rows" size="small" border>
    <el-table-column :label="t('game.marketItem')" min-width="180">
      <template #default="{ row }">
        <span v-if="auctionCustomName(row)">
          <span :class="auctionCustomLabelClass(row)">{{ auctionCustomName(row) }}</span>
          <span>（{{ itemLabel(row.itemId) }}）</span>
        </span>
        <span v-else>{{ itemLabel(row.itemId) }}</span>
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketQuantity')" prop="quantity" width="100" />
    <el-table-column :label="t('game.auctionCurrentPrice')" width="130">
      <template #default="{ row }">
        {{ row.currentPrice > 0 ? formatNumberWithThousands(row.currentPrice) : t("game.auctionNoBid") }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.auctionEndsAt')" width="170">
      <template #default="{ row }">
        {{ formatEndsAt(row.endsAt) }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.remainingTime')" width="130">
      <template #default="{ row }">
        {{ formatRemaining(row.endsAt) }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketStatus')" width="110">
      <template #default="{ row }">
        {{ auctionStatusLabel(row.status) }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketAction')" min-width="140">
      <template #default="{ row }">
        <div class="flex items-center gap-2">
          <el-button text @click="$emit('open-detail', row.id)">{{ t("game.marketDetail") }}</el-button>
          <el-button v-if="row.status === 'active'" type="warning" @click="$emit('cancel', row.id)">
            {{ t("game.auctionCancelAction") }}
          </el-button>
          <span v-else class="app-muted">-</span>
        </div>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import { getLocalizedItemName } from "@services/item-label"
import { itemCustomLabelClass, itemCustomName } from "@services/item-customization"
import { formatNumberWithThousands } from "@utils/number-format"
import type { ItemCustomization } from "@/types/equipment"

type AuctionListing = {
  id: number
  itemId: string
  instanceCustomData?: ItemCustomization
  quantity: number
  currentPrice: number
  status: string
  endsAt: string
}

defineProps<{
  rows: AuctionListing[]
  formatEndsAt: (value: string) => string
  formatRemaining: (value: string) => string
  auctionStatusLabel: (status: string) => string
}>()

defineEmits<{
  (e: "cancel", auctionId: number): void
  (e: "open-detail", auctionId: number): void
}>()

const { t } = useI18n()

function itemLabel(itemId: string) {
  return getLocalizedItemName(t, itemId, { fallbackText: t("game.unnamedItem") })
}

function auctionCustomName(row: AuctionListing) {
  return itemCustomName(row.instanceCustomData)
}

function auctionCustomLabelClass(row: AuctionListing) {
  return itemCustomLabelClass(row.instanceCustomData)
}
</script>
