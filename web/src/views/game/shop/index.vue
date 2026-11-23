<template>
  <div class="page-shell">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never" data-tutorial="shop-page">
      <template #header>
        <GamePageHeader
          title-key="game.menu.shop"
          tutorial-id="shop"
          :visual-src="shopVisual.src"
          :visual-tone="shopVisual.tone"
          visual-key="shop"
        >
          <template #actions>
            <el-button size="small" @click="loadShopOffers">{{ t("game.marketRefresh") }}</el-button>
          </template>
        </GamePageHeader>
      </template>
      <div v-if="shopOffers.length === 0" class="text-sm app-muted">
        {{ t("game.shopEmpty") }}
      </div>
      <div v-else class="space-y-3" data-tutorial="shop-offers">
        <el-card v-for="offer in shopOffers" :key="offer.id" class="app-card">
          <div class="flex items-center justify-between gap-4 text-sm">
            <div>
              <div>
                <QuickRef :item-id="offer.itemId" />
              </div>
              <div class="app-muted">
                {{ t("game.shopSellPrice") }} {{ offer.shopSellPrice }} / {{ t("game.shopBuyPrice") }} {{ offer.shopBuyPrice }}
              </div>
            </div>
            <div class="flex gap-2">
              <el-button size="small" @click="shopBuy(offer.id)">{{ t("game.marketBuyOne") }}</el-button>
              <el-button size="small" :disabled="getInventoryQuantity(offer.itemId) < 1" @click="shopSell(offer.itemId)">
                {{ t("game.marketSellOne") }}
              </el-button>
            </div>
          </div>
        </el-card>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiGet, apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import QuickRef from "@/components/game/item/QuickRef.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

type ShopOffer = { id: number; itemId: string; shopBuyPrice: number; shopSellPrice: number }

const { t } = useI18n()
const shopVisual = gamePageVisuals.shop
const store = useGameStore()
const inventoryStore = useInventoryStore()
const { getInventoryQuantity } = useInventorySummary()
const shopOffers = ref<ShopOffer[]>([])

onMounted(() => {
  store.restoreToken()
  if (!store.token) return
  void inventoryStore.initialize(store.token)
  void loadShopOffers()
})

async function loadShopOffers() {
  if (!store.token) return
  try {
    const data = await apiGet<{ offers: ShopOffer[] }>("/api/shop/offers", store.token)
    shopOffers.value = data.offers || []
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.shopLoadFailed"))
  }
}

async function shopBuy(offerID: number) {
  if (!store.token) return
  try {
    await apiPost("/api/shop/buy", store.token, { offerId: offerID, quantity: 1 })
    ElMessage.success(t("game.shopBuySuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.shopBuyFailed"))
  }
}

async function shopSell(itemID: string) {
  if (!store.token) return
  if (getInventoryQuantity(itemID) < 1) {
    ElMessage.warning(t("game.shopSellInsufficientInventory"))
    return
  }
  try {
    await apiPost("/api/shop/sell", store.token, { itemId: itemID, quantity: 1 })
    ElMessage.success(t("game.shopSellSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.shopSellFailed"))
  }
}
</script>
