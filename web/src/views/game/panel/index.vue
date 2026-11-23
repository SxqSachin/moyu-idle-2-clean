<template>
  <div class="page-shell">
    <ActionPanel v-if="isActionMainType" :menu="currentTab" :title-key="panelTitleKey" />
    <el-card v-else-if="isDictionaryTab" class="app-card app-card--no-border game-page-frame" shadow="never">
      <template #header>
        <GamePageHeader
          :title="panelTitle"
          :visual-src="panelVisual?.src || ''"
          :visual-tone="panelVisual?.tone || 'green'"
          :visual-key="currentTab"
        />
      </template>
      <DictionaryPanel />
    </el-card>
    <template v-else-if="isInstructionsTab">
      <el-card class="app-card app-card--no-border game-page-frame" shadow="never">
        <template #header>
          <GamePageHeader
            :title="panelTitle"
            :visual-src="panelVisual?.src || ''"
            :visual-tone="panelVisual?.tone || 'green'"
            :visual-key="currentTab"
          />
        </template>
        <InstructionsContent></InstructionsContent>
      </el-card>
    </template>
    <template v-else-if="isUpdateLogTab">
      <UpdateLogPanel />
    </template>
    <el-card v-else-if="isSettingsTab" class="app-card app-card--no-border" shadow="never" data-tutorial="settings-panel">
      <template #header>
        <GamePageHeader :title="panelTitle" tutorial-id="settings" />
      </template>
      <PlayerSettingsPanel />
    </el-card>
    <el-card v-else-if="isCatteryTab" class="app-card app-card--no-border game-page-frame" shadow="never">
      <template #header>
        <GamePageHeader
          :title="panelTitle"
          tutorial-id="cattery"
          :visual-src="panelVisual?.src || ''"
          :visual-tone="panelVisual?.tone || 'green'"
          :visual-key="currentTab"
        />
      </template>
      <CatteryPanel />
    </el-card>
    <el-card v-else-if="isGameGuideTab" class="app-card app-card--no-border game-page-frame" shadow="never" data-tutorial="gameguide-panel">
      <template #header>
        <GamePageHeader
          :title="panelTitle"
          :visual-src="panelVisual?.src || ''"
          :visual-tone="panelVisual?.tone || 'green'"
          :visual-key="currentTab"
        />
      </template>
      <TutorialGuidePanel />
    </el-card>
    <el-card v-else class="app-card app-card--no-border game-page-frame" shadow="never">
      <template #header>
        <GamePageHeader
          :title="panelTitle"
          :visual-src="panelVisual?.src || ''"
          :visual-tone="panelVisual?.tone || 'green'"
          :visual-key="currentTab"
        />
      </template>
      <p class="app-muted">{{ t("game.pendingBackend") }}</p>
      <el-button class="mt-4" type="primary" disabled>
        {{ t("game.pendingBackendAction") }}
      </el-button>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useRoute } from "vue-router"
import { useI18n } from "vue-i18n"
import ActionPanel from "@components/game/action/ActionPanel.vue"
import DictionaryPanel from "@components/game/panel/DictionaryPanel.vue"
import InstructionsContent from "@components/common/InstructionsContent.vue"
import PlayerSettingsPanel from "@components/game/panel/PlayerSettingsPanel.vue"
import UpdateLogPanel from "@components/game/panel/UpdateLogPanel.vue"
import CatteryPanel from "@components/game/panel/CatteryPanel.vue"
import TutorialGuidePanel from "@/tutorial/components/TutorialGuidePanel.vue"
import { listActionDefinitions, listActionMainTypeDefinitions } from "@services/action-definition"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { resolveGamePageVisual } from "@services/game-page-visuals"

const route = useRoute()
const { t } = useI18n()
const actionMainTypeSet = ref(new Set<string>())

const panelTitleKeyMap: Record<string, string> = {
  friend: "game.menu.friend",
  farmingPlant: "game.menu.farmingPlant",
  fishing: "game.menu.fishing",
  cattery: "game.menu.cattery",
  tavern: "game.menu.tavern",
  building: "game.menu.building",
  village: "game.menu.village",
  shop: "game.menu.shop",
  auction: "game.menu.auction",
  playerShop: "game.menu.playerShop",
  guild: "game.menu.guild",
  expedition: "game.menu.expedition",
  enhance: "game.menu.enhance",
  dismantle: "game.menu.dismantle",
  mysterious: "game.menu.mysterious",
  rankingList: "game.menu.rankingList",
  cheaterList: "game.menu.cheaterList",
  gameguide: "game.menu.gameguide",
  dictionary: "game.menu.dictionary",
  wiki: "game.menu.wiki",
  instructions: "game.menu.instructions",
  updateLog: "game.menu.updateLog",
  report: "game.menu.report",
  gm: "game.menu.gm",
  sponsor: "game.menu.sponsor",
  fishingAction: "action_main_type.fishing.name",
  farmingAnimal: "action_main_type.farmingAnimal.name",
  cooking: "action_main_type.cooking.name",
  mining: "action_main_type.mining.name",
  forging: "action_main_type.forging.name",
  sewing: "action_main_type.sewing.name",
  specialManufacture: "action_main_type.specialManufacture.name",
  selfProfile: "game.userMenu.profile",
  bindEmail: "game.userMenu.bindEmail",
  resetPassword: "game.userMenu.resetPassword",
  switchAccount: "game.userMenu.switchAccount",
  settings: "game.userMenu.settings"
}

const currentTab = computed(() => {
  const rawTab = Array.isArray(route.params.tab) ? route.params.tab[0] : route.params.tab
  return typeof rawTab === "string" ? rawTab : ""
})

const isActionMainType = computed(() => actionMainTypeSet.value.has(currentTab.value))
const isDictionaryTab = computed(() => currentTab.value === "dictionary")
const isInstructionsTab = computed(() => currentTab.value === "instructions")
const isUpdateLogTab = computed(() => currentTab.value === "updateLog")
const isSettingsTab = computed(() => currentTab.value === "settings")
const isCatteryTab = computed(() => currentTab.value === "cattery")
const isGameGuideTab = computed(() => currentTab.value === "gameguide")
const panelVisual = computed(() => resolveGamePageVisual(currentTab.value))

const panelTitleKey = computed(() => {
  const key = panelTitleKeyMap[currentTab.value]
  if (key) return key
  if (isActionMainType.value) return `action_main_type.${currentTab.value}.name`
  return "game.pendingPanelTitle"
})

const panelTitle = computed(() => {
  const tab = currentTab.value
  const key = panelTitleKeyMap[tab]
  if (!key) return t("game.pendingPanelTitle")
  return t(key)
})

onMounted(() => {
  loadActionMainTypes()
})

function loadActionMainTypes() {
  const next = new Set<string>()
  const mainTypes = listActionMainTypeDefinitions()
  for (const entry of mainTypes) {
    if (entry?.id) next.add(entry.id)
  }
  if (next.size === 0) {
    const actions = listActionDefinitions()
    for (const action of actions) {
      const mainType = action.mainType || action.menu
      if (mainType) next.add(mainType)
    }
  }
  actionMainTypeSet.value = next
}
</script>
