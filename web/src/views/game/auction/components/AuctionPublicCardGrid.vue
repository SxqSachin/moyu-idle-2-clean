<template>
  <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
    <el-card
      v-for="row in rows"
      :key="row.id"
      class="app-card auction-public-card cursor-pointer"
      shadow="hover"
      @click="$emit('open-detail', row.id)"
    >
      <div class="flex h-full flex-col gap-3">
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0">
            <div class="text-xs app-muted">{{ t("game.marketSeller") }}</div>
            <div class="truncate text-sm font-medium">{{ row.sellerUsername || t("game.auctionUnknownUser") }}</div>
            <div v-if="row.status === 'active' && row.currentBidderId" class="truncate text-xs app-muted">
              {{ t("game.auctionCurrentHighestBidder") }}: {{ row.currentBidderUsername || t("game.auctionUnknownUser") }}
            </div>
            <div v-else-if="row.status !== 'active' && row.currentBidderId" class="truncate text-xs app-muted">
              {{ t("game.auctionBuyer") }}: {{ row.currentBidderUsername || t("game.auctionUnknownUser") }}
            </div>
          </div>
          <el-tag size="small" type="info">{{ listingKindLabel(row.listingKind) }}</el-tag>
        </div>

        <div class="flex items-center gap-3">
          <Card
            :item-id="row.itemId"
            :instance-id="row.instanceId || undefined"
            :instance-preview="toInstancePreview(row)"
            :quantity="row.quantity"
          />
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-medium">
              <template v-if="auctionCustomName(row)">
                <span :class="auctionCustomLabelClass(row)">{{ auctionCustomName(row) }}</span>
                <span>（{{ itemLabel(row.itemId) }}）</span>
              </template>
              <template v-else>{{ itemLabel(row.itemId) }}</template>
            </div>
            <div class="text-xs app-muted">{{ t("game.marketQuantity") }}: {{ row.quantity }}</div>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2 text-sm">
          <div>
            <div class="app-muted text-xs">{{ t("game.auctionCurrentPrice") }}</div>
            <QuickRef v-if="row.currentPrice > 0" item-id="gold_coin" :count="row.currentPrice" normal-number-format />
            <div v-else>{{ t("game.auctionNoBid") }}</div>
          </div>
          <div>
            <div class="app-muted text-xs">{{ t("game.auctionMinBid") }}</div>
            <QuickRef item-id="gold_coin" :count="row.minNextBid" normal-number-format />
          </div>
          <div>
            <div class="app-muted text-xs">{{ t("game.auctionBuyoutPrice") }}</div>
            <QuickRef v-if="row.buyoutPrice && row.buyoutPrice > 0" item-id="gold_coin" :count="row.buyoutPrice" normal-number-format />
            <div v-else>{{ t("game.auctionNoBuyout") }}</div>
          </div>
          <div>
            <div class="app-muted text-xs">{{ t("game.marketStatus") }}</div>
            <div>{{ auctionStatusLabel(row.status) }}</div>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2 text-sm">
          <div>
            <div class="app-muted text-xs">{{ t("game.auctionEndsAt") }}</div>
            <div class="truncate">{{ formatEndsAt(row.endsAt) }}</div>
          </div>
          <div>
            <div class="app-muted text-xs">{{ t("game.remainingTime") }}</div>
            <div>{{ formatRemaining(row.endsAt) }}</div>
          </div>
        </div>

        <div class="mt-auto flex justify-end">
          <el-button type="primary" text @click.stop="$emit('open-detail', row.id)">
            {{ t("game.marketDetail") }}
          </el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import Card from "@components/game/item/Card.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import { getLocalizedItemName } from "@services/item-label"
import { itemCustomLabelClass, itemCustomName } from "@services/item-customization"
import type { EquipmentAffixSlot, FishInstanceData, ItemCustomization } from "@/types/equipment"

type AuctionListing = {
  id: number
  sellerId: number
  sellerUsername?: string
  listingKind: "stack" | "instance"
  itemId: string
  instanceId?: number | null
  instanceCustomData?: ItemCustomization
  fishData?: FishInstanceData | null
  instanceEnhanceLevel?: number | null
  instanceBaseSnapshot?: Record<string, unknown> | null
  instanceAffixes?: EquipmentAffixSlot[] | null
  instanceDurability?: number | null
  instanceMaxDurability?: number | null
  quantity: number
  minNextBid: number
  currentBidderId?: number | null
  currentBidderUsername?: string
  currentPrice: number
  buyoutPrice?: number | null
  status: string
  endsAt: string
}

const props = defineProps<{
  rows: AuctionListing[]
  formatEndsAt: (value: string) => string
  formatRemaining: (value: string) => string
  auctionStatusLabel: (status: string) => string
}>()

defineEmits<{
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

function listingKindLabel(listingKind: string) {
  if (listingKind === "instance") return t("game.marketInstanceListing")
  return t("game.marketStackListing")
}

function toInstancePreview(row: AuctionListing) {
  if (!row.instanceId && !auctionCustomName(row) && !row.fishData) return null
  return {
    customData: row.instanceCustomData || {},
    enhanceLevel: Number(row.instanceEnhanceLevel || 0),
    baseSnapshot: row.instanceBaseSnapshot || {},
    affixes: row.instanceAffixes || [],
    durability: Number(row.instanceDurability || 0),
    maxDurability: Number(row.instanceMaxDurability || 0),
    fishData: row.fishData || undefined
  }
}
</script>
