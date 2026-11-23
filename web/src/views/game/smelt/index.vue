<template>
  <div class="page-shell space-y-4">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never">
      <template #header>
        <GamePageHeader
          title-key="game.smelt"
          :visual-src="smeltVisual.src"
          :visual-tone="smeltVisual.tone"
          visual-key="smelt"
        >
          <template #actions>
            <el-button size="small" @click="refresh">{{ t("game.refresh") }}</el-button>
          </template>
        </GamePageHeader>
      </template>

      <AffixSmeltPanel @submitted="refresh" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import AffixSmeltPanel from "@components/game/equipment-affix/AffixSmeltPanel.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const { t } = useI18n()
const smeltVisual = gamePageVisuals.smelt
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()

onMounted(() => {
  void initialize()
})

async function initialize() {
  gameStore.restoreToken()
  if (!gameStore.token) return
  await inventoryStore.initialize(gameStore.token)
}

async function refresh() {
  if (!gameStore.token) return
  await inventoryStore.initialize(gameStore.token, true)
}
</script>
