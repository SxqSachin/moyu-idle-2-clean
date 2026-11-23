<template>
  <div class="page-shell combat-page-shell space-y-4">
    <GamePageFrame
      class="combat-page-card"
      title-key="game.menu.battle"
      tutorial-id="combat"
      :visual-src="combatVisual.src"
      :visual-tone="combatVisual.tone"
      visual-key="battle"
      data-tutorial="combat-page"
    >
      <div v-if="showInitialCombatLoading" class="combat-page-loading">
        <div class="combat-page-loading__indicator" role="status" aria-live="polite">
          <span class="combat-page-loading__spinner" aria-hidden="true"></span>
          <span>{{ t("game.battleBattleLoading") }}</span>
        </div>
      </div>
      <el-tabs v-else v-model="activeTab" class="battle-tabs" data-tutorial="combat-tabs" @tab-change="handleTabChange">
        <el-tab-pane :label="t('game.battleAreaTab')" name="area" :lazy="true">
          <AreaView />
        </el-tab-pane>
        <el-tab-pane :label="t('game.battleRoomListTab')" name="rooms" :lazy="true">
          <RoomsView />
        </el-tab-pane>
        <el-tab-pane :label="t('game.battleMyRoomTab')" name="mine" :lazy="true">
          <MineView />
        </el-tab-pane>
        <el-tab-pane :label="t('game.battleCombatTab')" name="battle" :lazy="true">
          <BattleView />
        </el-tab-pane>
        <el-tab-pane :label="t('game.battleHelpTab')" name="help" :lazy="true">
          <HelpView />
        </el-tab-pane>
      </el-tabs>
    </GamePageFrame>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useCombatPage } from "@/composables/game/combat/useCombatPage"
import { useTutorialStore } from "@/tutorial"
import { acquireBattleStream } from "@services/battle-stream"
import AreaView from "./views/AreaView.vue"
import RoomsView from "./views/RoomsView.vue"
import MineView from "./views/MineView.vue"
import BattleView from "./views/BattleView.vue"
import HelpView from "./views/HelpView.vue"
import { provideCombatViewContext, type BattleTabName } from "./useCombatViewContext"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const { t } = useI18n()
const activeTab = ref<BattleTabName>("area")
const combatPage = useCombatPage()
const tutorialStore = useTutorialStore()
const combatVisual = gamePageVisuals.combat
const initialRoomsLoaded = ref(false)
let releaseRoomBattleStream: (() => void) | null = null
const showInitialCombatLoading = computed(() => !initialRoomsLoaded.value)

function setActiveTab(tab: BattleTabName) {
  activeTab.value = tab
}

provideCombatViewContext({
  ...combatPage,
  activeTab,
  setActiveTab
})

watch(
  () => combatPage.kickedRoomID.value,
  (roomID) => {
    if (!roomID || roomID <= 0) return
    setActiveTab("rooms")
  }
)

watch(activeTab, () => {
  if (activeTab.value === "battle") {
    combatPage.selectMyRoom()
    if (!releaseRoomBattleStream) releaseRoomBattleStream = acquireBattleStream("room")
  } else if (releaseRoomBattleStream) {
    releaseRoomBattleStream()
    releaseRoomBattleStream = null
  }
}, { immediate: true })

function handleTabChange(name: string | number) {
  const tab = String(name || "") as BattleTabName
  if (tab === "area" || tab === "rooms" || tab === "mine" || tab === "battle" || tab === "help") {
    setActiveTab(tab)
  }
}

onMounted(async () => {
  try {
    await combatPage.loadRooms()
  } finally {
    initialRoomsLoaded.value = true
  }
  // While the combat tutorial walks each tab, suppress the auto-jump to the
  // battle/my-room tab so the guided sequence isn't yanked off course.
  if (tutorialStore.activeTutorialId === "combat") {
    return
  }
  const runStatus = String(combatPage.selectedRun.value?.status || "")
  const hasRunningBattle = runStatus === "compiled" || runStatus === "playing"
  const hasMyRoom = Boolean(combatPage.myRoomDetail.value?.room.id)
  if (hasRunningBattle) {
    setActiveTab("battle")
    return
  }
  if (hasMyRoom) {
    setActiveTab("mine")
    return
  }
  setActiveTab("area")
})

onUnmounted(() => {
  releaseRoomBattleStream?.()
  releaseRoomBattleStream = null
})
</script>

<style scoped>
.combat-page-shell {
  min-height: 0;
}

.combat-page-card {
  min-height: 0;
}

.combat-page-loading {
  min-height: 260px;
  display: grid;
  place-items: center;
}

.combat-page-loading__indicator {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--app-muted);
  font-size: 13px;
  line-height: 1.5;
}

.combat-page-loading__spinner {
  width: 18px;
  height: 18px;
  box-sizing: border-box;
  border: 2px solid color-mix(in srgb, var(--app-muted) 24%, transparent);
  border-top-color: var(--el-color-primary);
  border-radius: 999px;
  animation: combat-page-loading-spin 0.8s linear infinite;
}

@keyframes combat-page-loading-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 767px) {
  .combat-page-shell {
    min-height: 100%;
    padding-bottom: calc(10px + env(safe-area-inset-bottom));
  }

  .combat-page-card {
    display: flex;
    flex-direction: column;
  }

  .combat-page-card:deep(.el-card__body) {
    min-height: 0;
    padding: 10px;
  }

  .battle-tabs:deep(.el-tabs__header) {
    margin-bottom: 10px;
  }

  .battle-tabs:deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
    overflow-y: hidden;
    -webkit-overflow-scrolling: touch;
  }

  .battle-tabs:deep(.el-tabs__content),
  .battle-tabs:deep(.el-tab-pane) {
    min-height: 0;
    overflow: visible;
  }
}
</style>
