<template>
  <div class="page-shell">
    <GamePageFrame
      title-key="game.auctionTitle"
      tutorial-id="auction"
      :visual-src="auctionVisual.src"
      :visual-tone="auctionVisual.tone"
      visual-key="auction"
      data-tutorial="auction-page"
    >
      <template #actions>
        <el-tooltip :content="t('game.auctionReputationRequired', { current: repLevel })" :disabled="canAuction">
          <el-button size="small" type="primary" :disabled="!canAuction" @click="openCreateDialog">{{ t("game.auctionCreateAction") }}</el-button>
        </el-tooltip>
      </template>
      <div class="mb-2 text-sm app-muted">{{ t("game.auctionHint") }}</div>
      <div class="mb-4 text-xs app-muted">
        {{ t("game.marketDialogFeeEstimate") }}: {{ auctionFeeRateBps / 100 }}% ·
        {{ t("game.auctionAntiSnipeHint", { trigger: antiSnipeTriggerSeconds / 60, extend: antiSnipeExtendSeconds / 60 }) }}
      </div>

      <el-alert
        v-if="!canAuction"
        type="warning"
        :closable="false"
        show-icon
        class="mb-4"
      >
        <span>{{ t("game.auctionReputationRequired", { required: requiredRepLevel, current: repLevel }) }}</span>
        <router-link to="/game/tasks" class="rep-upgrade-link">{{ t("game.auctionGoImprove") }}</router-link>
      </el-alert>
      <el-tabs v-model="activeTab" data-tutorial="auction-tabs">
        <el-tab-pane :label="t('game.auctionPublicListings')" name="public">

          <div class="mb-4 flex flex-wrap items-center gap-2" data-tutorial="auction-filters">
            <el-select v-model="filterItemId" clearable style="max-width: 260px" :placeholder="t('game.marketFilterByItem')">
              <el-option v-for="itemId in auctionFilterOptions" :key="`auction-filter-${itemId}`" :label="itemLabel(itemId)" :value="itemId" />
            </el-select>
            <el-select v-model="publicStatusFilter" style="max-width: 220px" :placeholder="t('game.auctionPublicFilterPlaceholder')">
              <el-option
                v-for="option in publicStatusFilterOptions"
                :key="`auction-status-filter-${option.value}`"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
            <el-input v-model="searchKeyword" clearable style="max-width: 240px" :placeholder="t('game.marketSearchPlaceholder')" />
            <el-button @click="loadAll">{{ t("game.marketRefresh") }}</el-button>
          </div>

          <div v-if="filteredPublicAuctions.length === 0" class="text-sm app-muted">{{ t("game.auctionNoListings") }}</div>
          <div v-else data-tutorial="auction-list">
            <AuctionPublicCardGrid
              :rows="filteredPublicAuctions"
              :format-ends-at="formatEndsAt"
              :format-remaining="formatRemaining"
              :auction-status-label="auctionStatusLabel"
              @open-detail="openAuctionDetail"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('game.auctionMyListings')" name="mine">
          <div v-if="filteredMyAuctions.length === 0" class="text-sm app-muted">{{ t("game.auctionNoListings") }}</div>
          <AuctionMyTable
            v-else
            :rows="filteredMyAuctions"
            :format-ends-at="formatEndsAt"
            :format-remaining="formatRemaining"
            :auction-status-label="auctionStatusLabel"
            @cancel="cancelAuction"
            @open-detail="openAuctionDetail"
          />
        </el-tab-pane>
        <el-tab-pane :label="t('game.auctionDetailTab')" name="detail">
          <div v-if="!selectedAuctionDetail" class="text-sm app-muted">{{ t("game.auctionDetailEmpty") }}</div>
          <div v-else class="space-y-3">
            <el-card class="app-card">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div class="flex min-w-0 items-center gap-3">
                  <Card
                    :item-id="selectedAuctionDetail.itemId"
                    :instance-id="selectedAuctionDetail.instanceId || undefined"
                    :instance-preview="toInstancePreview(selectedAuctionDetail)"
                    :quantity="selectedAuctionDetail.quantity"
                  />
                  <div class="min-w-0 space-y-2">
                    <div class="truncate text-lg font-semibold">{{ auctionItemLabel(selectedAuctionDetail) }}</div>
                    <div class="grid gap-x-6 gap-y-1 text-sm app-muted sm:grid-cols-2">
                      <div>
                        {{ t("game.marketSeller") }}: <span class="font-medium text-[var(--app-text)]">{{ selectedAuctionDetail.sellerUsername || t("game.auctionUnknownUser") }}</span>
                      </div>
                      <div>
                        {{ t("game.marketQuantity") }}: <span class="font-medium text-[var(--app-text)]">{{ selectedAuctionDetail.quantity }}</span>
                      </div>
                      <div>
                        {{ t("game.marketStatus") }}:
                        <el-tag size="small" :type="selectedAuctionDetail.status === 'active' ? 'success' : 'info'">
                          {{ auctionStatusLabel(selectedAuctionDetail.status) }}
                        </el-tag>
                      </div>
                      <div v-if="selectedAuctionDetail.status === 'active' && selectedAuctionDetail.currentBidderId">
                        {{ t("game.auctionCurrentHighestBidder") }}:
                        <span class="font-medium text-[var(--app-text)]">{{ selectedAuctionDetail.currentBidderUsername || t("game.auctionUnknownUser") }}</span>
                      </div>
                      <div v-else-if="selectedAuctionDetail.status !== 'active' && selectedAuctionDetail.currentBidderId">
                        {{ t("game.auctionBuyer") }}: <span class="font-medium text-[var(--app-text)]">{{ selectedAuctionDetail.currentBidderUsername || t("game.auctionUnknownUser") }}</span>
                      </div>
                    </div>
                    <div class="text-xs app-muted">{{ t("game.marketDialogFeeEstimate") }}: {{ selectedAuctionDetail.feeRateBps || auctionFeeRateBps }} bps</div>
                  </div>
                </div>
                <div class="text-right">
                  <div class="text-xs app-muted">{{ t("game.auctionEndsAt") }}</div>
                  <div class="text-base font-medium">{{ formatEndsAt(selectedAuctionDetail.endsAt) }}</div>
                  <div class="text-xs app-muted">{{ t("game.remainingTime") }}: {{ formatRemaining(selectedAuctionDetail.endsAt) }}</div>
                </div>
              </div>
            </el-card>

            <div class="grid gap-3 xl:grid-cols-3">
              <el-card class="app-card">
                <div class="space-y-3">
                  <div class="flex flex-row items-center gap-2">
                    <div class="text-sm font-medium">{{ t("game.auctionStartPrice") }}:</div>
                    <QuickRef item-id="gold_coin" :count="selectedAuctionDetail.startPrice" normal-number-format/>
                  </div>
                  <div class="flex flex-row items-center gap-2">
                    <div class="text-sm font-medium">{{ t("game.auctionCurrentPrice") }}:</div>
                    <QuickRef v-if="selectedAuctionDetail.currentPrice > 0" item-id="gold_coin" :count="selectedAuctionDetail.currentPrice" normal-number-format/>
                    <div v-else class="text-sm app-muted">{{ t("game.auctionNoBid") }}</div>
                  </div>
                  <div class="flex flex-row items-center gap-2">
                    <div class="text-sm font-medium">{{ t("game.auctionMinBid") }}:</div>
                    <QuickRef item-id="gold_coin" :count="selectedAuctionDetail.minNextBid" normal-number-format/>
                  </div>
                  <div class="flex flex-row items-center gap-2" v-if="selectedAuctionDetail.buyoutPrice && selectedAuctionDetail.buyoutPrice > 0">
                    <div class="text-sm font-medium">{{ t("game.auctionBuyoutPrice") }}:</div>
                    <QuickRef item-id="gold_coin" :count="selectedAuctionDetail.buyoutPrice" normal-number-format />
                  </div>
                </div>
              </el-card>

              <el-card class="app-card">
                <div class="space-y-3">
                  <div class="text-sm font-medium">{{ t("game.auctionMinBidIncrement") }}</div>
                  <QuickRef item-id="gold_coin" :count="Math.max(1, selectedAuctionDetail.minBidIncrement || 1)" normal-number-format/>
                  <div class="text-sm font-medium">{{ t("game.marketDialogFeeEstimate") }}</div>
                  <div class="text-sm app-muted">{{ selectedAuctionDetail.feeRateBps || auctionFeeRateBps }} bps</div>
                  <div class="text-sm font-medium">{{ t("game.auctionAntiSnipeHint", { trigger: antiSnipeTriggerSeconds / 60, extend: antiSnipeExtendSeconds / 60 }) }}</div>
                </div>
              </el-card>

              <el-card class="app-card" v-if="selectedAuctionDetail.status === 'active'">
                <div class="space-y-3">
                  <div class="flex items-center justify-between gap-2">
                    <div>
                      <div class="text-sm font-medium">{{ t("game.auctionBidAction") }}</div>
                      <div class="text-xs app-muted">{{ t("game.auctionMinBid") }}: {{ formatNumberWithThousands(selectedAuctionDetail.minNextBid) }}</div>
                    </div>
                    <el-button @click="openBidHistory(selectedAuctionDetail.id)">{{ t("game.auctionBidHistory") }}</el-button>
                  </div>
                  <div class="space-y-3">
                    <div class="flex flex-wrap items-center gap-2">
                      <el-input-number
                        class="w-full! md:w-56!"
                        v-model="bidDraftByAuction[selectedAuctionDetail.id]"
                        :min="Math.max(1, selectedAuctionDetail.minNextBid)"
                        :max="1000000000000"
                      />
                      <el-button
                        v-if="selectedAuctionDetail.buyoutPrice && selectedAuctionDetail.buyoutPrice > 0"
                        class="ml-0!"
                        @click="bidDraftByAuction[selectedAuctionDetail.id] = Math.max(selectedAuctionDetail.minNextBid, selectedAuctionDetail.buyoutPrice)"
                      >
                        {{ t("game.auctionFillBuyoutBid") }}
                      </el-button>
                    </div>
                    <div class="flex flex-wrap items-center gap-3 w-full justify-start">
                      <el-button class="ml-0!" @click="adjustBidPercent(selectedAuctionDetail.id, 0.1)">+10%</el-button>
                      <el-button class="ml-0!" @click="adjustBidPercent(selectedAuctionDetail.id, 0.2)">+20%</el-button>
                      <el-button class="ml-0!" @click="adjustBidPercent(selectedAuctionDetail.id, 0.5)">+50%</el-button>
                    </div>
                  </div>
                  <div class="flex flex-wrap items-center gap-2">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="text-sm">{{ t("game.marketPrice") }}</span>
                      <QuickRefWithRequire item-id="gold_coin" :required-count="Math.max(1, bidDraftByAuction[selectedAuctionDetail.id] || selectedAuctionDetail.minNextBid)" />
                    </div>
                  </div>

                  <el-tooltip
                    :content="t('game.auctionReputationRequired', { current: repLevel })"
                    :disabled="canAuction"
                  >
                    <el-button
                    class="w-full md:w-24!"
                      type="success"
                      :disabled="!canAuction || selectedAuctionDetail.sellerId === store.userId"
                      @click="placeBid(selectedAuctionDetail.id)"
                    >
                      {{ t("game.auctionBidAction") }}
                    </el-button>
                  </el-tooltip>
                </div>
              </el-card>

              <el-card v-if="selectedAuctionDetail.status === 'active' && selectedAuctionDetail.sellerId === store.userId" class="app-card">
                <div class="flex h-full flex-col justify-between gap-3">
                  <div class="space-y-2">
                    <div class="text-sm font-medium">{{ t("game.auctionCancelAction") }}</div>
                    <div class="text-xs app-muted">
                      <span>{{ t("game.auctionCancelFee") }}: </span>
                      <QuickRefWithRequire v-if="selectedCancelFee > 0" item-id="gold_coin" :required-count="selectedCancelFee" />
                      <span v-else>0</span>
                    </div>
                  </div>
                  <el-button class="w-full! md:w-56!" type="warning" @click="cancelAuction(selectedAuctionDetail.id)">
                    {{ t("game.auctionCancelAction") }}
                  </el-button>
                </div>
              </el-card>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </GamePageFrame>

    <AuctionCreateDialog
      :visible="createDialogVisible"
      :draft="createDraft"
      :selected-instance-item-id="selectedInstanceItemId"
      :min-start-price="createMinStartPrice"
      :min-bid-increment="createMinBidIncrement"
      :max-bid-increment="createMaxBidIncrement"
      :min-buyout-price="createMinBuyout"
      :fee-rate-bps="auctionFeeRateBps"
      :estimated-fee="createFeeEstimate"
      :estimated-seller-amount="createSellerAmount"
      :can-submit="canCreateAuction"
      @update:visible="createDialogVisible = $event"
      @open-instance-selector="openInstanceSelector"
      @confirm="createAuction"
    />

    <InventoryInstanceSelectorDialog
      v-model="instanceSelectorVisible"
      :title="t('game.marketSelectEquipment')"
      :instance-ids="instanceAuctionInstanceIDs"
      :selected-instance-id="createDraft.instanceId || null"
      :allow-clear="false"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="onConfirmSelectInstance"
    />

    <GameDialog v-model="bidHistoryVisible" :title="t('game.auctionBidHistory')" width="500px">
      <div v-if="bidHistory.length === 0" class="text-sm app-muted">{{ t("game.auctionBidHistoryEmpty") }}</div>
      <el-table v-else :data="bidHistory" size="small" border>
        <el-table-column :label="t('game.marketSeller')" width="140">
          <template #default="{ row }">{{ row.bidderUsername || t("game.auctionUnknownUser") }}</template>
        </el-table-column>
        <el-table-column :label="t('game.marketPrice')" width="120">
          <template #default="{ row }">
            {{ formatNumberWithThousands(row.bidAmount) }}
          </template>
        </el-table-column>
        <el-table-column :label="t('game.auctionBidTime')">
          <template #default="{ row }">
            {{ formatEndsAt(row.createdAt) }}
          </template>
        </el-table-column>
      </el-table>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from "vue"
import { useI18n } from "vue-i18n"
import Card from "@components/game/item/Card.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import AuctionCreateDialog from "@views/game/auction/components/AuctionCreateDialog.vue"
import AuctionPublicCardGrid from "@views/game/auction/components/AuctionPublicCardGrid.vue"
import AuctionMyTable from "@views/game/auction/components/AuctionMyTable.vue"
import { useAuctionPage } from "@/composables/game/auction/useAuctionPage"
import { formatNumberWithThousands } from "@/utils/number-format"
import { itemCustomName } from "@services/item-customization"
import type { EquipmentAffixSlot, FishInstanceData, ItemCustomization } from "@/types/equipment"
import GameDialog from "@components/game/common/GameDialog.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const AUCTION_TUTORIAL_OPEN_CREATE_DIALOG_EVENT = "auction:tutorial-open-create-dialog"
const AUCTION_TUTORIAL_CLOSE_DIALOGS_EVENT = "auction:tutorial-close-dialogs"

const { t } = useI18n()
const auctionVisual = gamePageVisuals.auction
const {
  store,
  activeTab,
  auctionFilterOptions,
  filterItemId,
  publicStatusFilter,
  searchKeyword,
  activeAuctionCount,
  myActiveAuctionCount,
  highestBidPrice,
  selectedAuctionDetail,
  createDraft,
  createDialogVisible,
  instanceSelectorVisible,
  instanceAuctionInstanceIDs,
  selectedInstanceItemId,
  createMinStartPrice,
  createMinBidIncrement,
  createMaxBidIncrement,
  createMinBuyout,
  createFeeEstimate,
  createSellerAmount,
  auctionFeeRateBps,
  antiSnipeTriggerSeconds,
  antiSnipeExtendSeconds,
  selectedCancelFee,
  canCreateAuction,
  canAuction,
  repLevel,
  requiredRepLevel,
  filteredPublicAuctions,
  filteredMyAuctions,
  bidDraftByAuction,
  bidHistoryVisible,
  bidHistory,
  itemLabel,
  formatEndsAt,
  formatRemaining,
  auctionStatusLabel,
  auctionItemLabel,
  loadAll,
  openCreateDialog,
  openInstanceSelector,
  onConfirmSelectInstance,
  createAuction,
  placeBid,
  cancelAuction,
  adjustBidPercent,
  openBidHistory,
  openAuctionDetail
} = useAuctionPage()

const publicStatusFilterOptions = computed(() => [
  { value: "participating", label: t("game.auctionPublicFilterParticipating") },
  { value: "sold", label: t("game.auctionPublicFilterSold") },
  { value: "expired", label: t("game.auctionPublicFilterExpired") },
  { value: "all", label: t("game.auctionPublicFilterAll") }
])

function handleAuctionTutorialOpenCreateDialog() {
  activeTab.value = "public"
  openCreateDialog()
}

function closeAuctionTutorialDialogs() {
  createDialogVisible.value = false
  instanceSelectorVisible.value = false
  bidHistoryVisible.value = false
}

onMounted(() => {
  window.addEventListener(AUCTION_TUTORIAL_OPEN_CREATE_DIALOG_EVENT, handleAuctionTutorialOpenCreateDialog)
  window.addEventListener(AUCTION_TUTORIAL_CLOSE_DIALOGS_EVENT, closeAuctionTutorialDialogs)
})

onBeforeUnmount(() => {
  window.removeEventListener(AUCTION_TUTORIAL_OPEN_CREATE_DIALOG_EVENT, handleAuctionTutorialOpenCreateDialog)
  window.removeEventListener(AUCTION_TUTORIAL_CLOSE_DIALOGS_EVENT, closeAuctionTutorialDialogs)
})

type AuctionDetailInstancePreviewSource = {
  instanceId?: number | null
  instanceCustomData?: ItemCustomization
  fishData?: FishInstanceData | null
  instanceEnhanceLevel?: number | null
  instanceBaseSnapshot?: Record<string, unknown> | null
  instanceAffixes?: EquipmentAffixSlot[] | null
  instanceDurability?: number | null
  instanceMaxDurability?: number | null
}

function toInstancePreview(row: AuctionDetailInstancePreviewSource | null | undefined) {
  if (!row?.instanceId && !itemCustomName(row?.instanceCustomData) && !row?.fishData) return null
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

<style scoped>
.rep-upgrade-link {
  margin-left: 12px;
  color: var(--el-color-primary);
  font-weight: 500;
  text-decoration: none;
}
.rep-upgrade-link:hover {
  text-decoration: underline;
}
</style>
