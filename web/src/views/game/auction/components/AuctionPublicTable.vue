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
    <el-table-column :label="t('game.marketSeller')" min-width="120">
      <template #default="{ row }">
        {{ row.sellerUsername || t("game.auctionUnknownUser") }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketQuantity')" prop="quantity" width="100" />
    <el-table-column :label="t('game.auctionStartPrice')" width="120">
      <template #default="{ row }">
        {{ formatNumberWithThousands(row.startPrice) }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.auctionCurrentPrice')" width="120">
      <template #default="{ row }">
        {{ row.currentPrice > 0 ? formatNumberWithThousands(row.currentPrice) : t("game.auctionNoBid") }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.auctionBuyoutPrice')" width="130">
      <template #default="{ row }">
        {{ row.buyoutPrice ? formatNumberWithThousands(row.buyoutPrice) : t("game.auctionNoBuyout") }}
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
    <el-table-column :label="t('game.marketAction')" min-width="230">
      <template #default="{ row }">
        <div v-if="row.status === 'active' && row.sellerId !== userId" class="flex items-center gap-2">
          <el-input-number v-model="bidDraftByAuction[row.id]" :min="Math.max(row.minNextBid || row.startPrice, row.currentPrice + 1)" :max="1000000000000" />
          <el-button @click="$emit('adjust-bid', row.id, 1)">+1</el-button>
          <el-button @click="$emit('adjust-bid', row.id, 10)">+10</el-button>
          <el-button type="success" @click="$emit('place-bid', row.id)">{{ t("game.auctionBidAction") }}</el-button>
          <el-button text @click="$emit('open-bid-history', row.id)">{{ t("game.auctionBidHistory") }}</el-button>
          <el-button text @click="$emit('open-detail', row.id)">{{ t("game.marketDetail") }}</el-button>
        </div>
        <div v-else class="flex items-center gap-2">
          <span class="app-muted">-</span>
          <el-button text @click="$emit('open-bid-history', row.id)">{{ t("game.auctionBidHistory") }}</el-button>
          <el-button text @click="$emit('open-detail', row.id)">{{ t("game.marketDetail") }}</el-button>
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
  sellerId: number
  sellerUsername?: string
  itemId: string
  instanceCustomData?: ItemCustomization
  quantity: number
  minNextBid?: number
  startPrice: number
  currentPrice: number
  buyoutPrice?: number | null
  status: string
  endsAt: string
}

defineProps<{
  rows: AuctionListing[]
  userId: number
  bidDraftByAuction: Record<number, number>
  formatEndsAt: (value: string) => string
  formatRemaining: (value: string) => string
  auctionStatusLabel: (status: string) => string
}>()

defineEmits<{
  (e: "adjust-bid", auctionId: number, delta: number): void
  (e: "place-bid", auctionId: number): void
  (e: "open-bid-history", auctionId: number): void
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
