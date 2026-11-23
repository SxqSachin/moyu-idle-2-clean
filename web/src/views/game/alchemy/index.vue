<template>
  <div class="page-shell space-y-4" :class="{ 'alchemy-page--skill-forge': activeTab === 'skill-forge' }">
    <GamePageFrame
      class="alchemy-page-card"
      title-key="game.menuGroup.alchemy"
      :tutorial-id="activeAlchemyTutorialId"
      :visual-src="alchemyVisual.src"
      :visual-tone="alchemyVisual.tone"
      visual-key="alchemy"
      data-tutorial="alchemy-page"
    >
      <template #actions>
        <el-button size="small" @click="handbookVisible = true">{{ t("alchemy.codex.open") }}</el-button>
        <el-button size="small" @click="refreshAll">{{ t("game.refresh") }}</el-button>
      </template>

      <el-tabs v-model="activeTab" data-tutorial="alchemy-tabs" @tab-change="handleTabChange">
        <el-tab-pane :label="t('alchemy.convert.title')" name="convert">
          <div v-if="activeTab === 'convert'" data-tutorial="alchemy-convert">
            <AlchemyConvertPanel :preferred-instance-id="preferredInstanceId" @submitted="refreshDiscoveries" />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('alchemy.bench.title')" name="bench">
          <div v-if="activeTab === 'bench'" data-tutorial="alchemy-bench">
            <AlchemyBenchPanel :combo-fill-request="comboFillRequest" @submitted="refreshDiscoveries" />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('alchemy.inherit.title')" name="inherit">
          <div v-if="activeTab === 'inherit'" data-tutorial="alchemy-inherit">
            <AlchemyInheritPanel @submitted="refreshAll" />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('alchemy.transmute.title')" name="transmute">
          <div v-if="activeTab === 'transmute'" data-tutorial="alchemy-transmute">
            <AlchemyTransmutePanel :preferred-instance-id="preferredInstanceId" @submitted="refreshDiscoveries" />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('alchemy.smelt.title')" name="smelt">
          <div v-if="activeTab === 'smelt'" data-tutorial="alchemy-smelt">
            <AffixSmeltPanel @submitted="refreshDiscoveries" />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('skillForge.title')" name="skill-forge">
          <div v-if="activeTab === 'skill-forge'" class="skill-forge-tab" data-tutorial="alchemy-skill-forge">
            <SkillForgePanel />
          </div>
        </el-tab-pane>
      </el-tabs>
    </GamePageFrame>

    <AlchemyHandbookDrawer v-model="handbookVisible" :discoveries="discoveries" @select-combo="handleSelectComboFromHandbook" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { fetchAlchemyDiscoveries, type AlchemyDiscoveries } from "@services/alchemy-data"
import AlchemyConvertPanel from "@components/game/alchemy/AlchemyConvertPanel.vue"
import AlchemyBenchPanel from "@components/game/alchemy/AlchemyBenchPanel.vue"
import AlchemyInheritPanel from "@components/game/alchemy/AlchemyInheritPanel.vue"
import AlchemyTransmutePanel from "@components/game/alchemy/AlchemyTransmutePanel.vue"
import AlchemyHandbookDrawer from "@components/game/alchemy/AlchemyHandbookDrawer.vue"
import AffixSmeltPanel from "@components/game/equipment-affix/AffixSmeltPanel.vue"
import SkillForgePanel from "@components/game/alchemy/SkillForgePanel.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

type AlchemyTab = "convert" | "bench" | "inherit" | "transmute" | "smelt" | "skill-forge"

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const alchemyVisual = gamePageVisuals.alchemy

const activeTab = ref<AlchemyTab>("convert")
const handbookVisible = ref(false)
const discoveries = ref<AlchemyDiscoveries | null>(null)
const comboFillNonce = ref(0)
const comboFillRequest = ref<{ id: string; nonce: number } | null>(null)
const activeAlchemyTutorialId = computed(() => `alchemy-${activeTab.value}`)
const preferredInstanceId = computed(() => {
  const raw = Array.isArray(route.query.instanceId) ? route.query.instanceId[0] : route.query.instanceId
  const value = Math.trunc(Number(raw || 0))
  return Number.isFinite(value) && value > 0 ? value : null
})

onMounted(() => {
  syncTabFromRoute()
  void initialize()
})

watch(
  () => route.query.tab,
  () => {
    syncTabFromRoute()
  }
)

function syncTabFromRoute() {
  const raw = typeof route.query.tab === "string" ? route.query.tab : ""
  if (raw === "convert" || raw === "bench" || raw === "inherit" || raw === "transmute" || raw === "smelt" || raw === "skill-forge") {
    activeTab.value = raw
  }
}

function handleTabChange(name: string | number) {
  if (name === "convert" || name === "bench" || name === "inherit" || name === "transmute" || name === "smelt" || name === "skill-forge") {
    activeTab.value = name
    void router.replace({ path: "/game/alchemy", query: { ...route.query, tab: name } })
  }
}

async function initialize() {
  gameStore.restoreToken()
  if (!gameStore.token) return
  await Promise.all([inventoryStore.initialize(gameStore.token), refreshDiscoveries()])
}

async function refreshDiscoveries() {
  if (!gameStore.token) return
  discoveries.value = await fetchAlchemyDiscoveries(gameStore.token)
}

async function refreshAll() {
  if (!gameStore.token) return
  await Promise.all([inventoryStore.initialize(gameStore.token, true), refreshDiscoveries()])
}

function handleSelectComboFromHandbook(comboId: string) {
  handbookVisible.value = false
  if (activeTab.value !== "bench") {
    handleTabChange("bench")
  }
  comboFillNonce.value += 1
  comboFillRequest.value = { id: comboId, nonce: comboFillNonce.value }
}
</script>

<style scoped>
.alchemy-page-card {
  overflow: visible;
}

.alchemy-page-card :deep(.el-card__body) {
  overflow: visible;
}

.alchemy-page-card :deep(.el-tabs__content),
.alchemy-page-card :deep(.el-tab-pane) {
  overflow: visible;
}

:global(.game-content-scroll:has(.alchemy-page--skill-forge)) {
  overflow: hidden !important;
}

:global(.game-route-stage:has(.alchemy-page--skill-forge)),
:global(.game-route-page:has(.alchemy-page--skill-forge)) {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.alchemy-page--skill-forge {
  height: 100%;
  min-height: 0 !important;
  overflow: hidden;
}

.alchemy-page--skill-forge .alchemy-page-card {
  height: 100%;
  min-height: 0 !important;
  overflow: hidden;
}

.alchemy-page--skill-forge .alchemy-page-card :deep(> .el-card__header) {
  flex: 0 0 auto;
}

.alchemy-page--skill-forge .alchemy-page-card :deep(> .el-card__body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.alchemy-page--skill-forge .alchemy-page-card :deep(.el-tabs) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.alchemy-page--skill-forge .alchemy-page-card :deep(.el-tabs__header) {
  flex: 0 0 auto;
}

.alchemy-page--skill-forge .alchemy-page-card :deep(.el-tabs__content),
.alchemy-page--skill-forge .alchemy-page-card :deep(.el-tab-pane),
.skill-forge-tab {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}
</style>
