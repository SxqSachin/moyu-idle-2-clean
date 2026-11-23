<template>
  <component :is="gameShellComponent" v-if="isGameDataLoaded && localeMessagesReady && gameShellComponent" />
  <main v-else class="game-boot">
    <section class="game-boot__panel">
      <div class="game-boot__eyebrow">{{ t("app.title") }}</div>
      <h1 class="game-boot__title">{{ t("boot.title") }}</h1>
      <div class="game-boot__loading">
        <span class="game-boot__spinner"></span>
        <p class="game-boot__status">{{ statusText }}</p>
      </div>

      <div class="game-boot__progress" :aria-label="t('boot.progressLabel')">
        <div class="game-boot__progress-bar" :style="{ width: `${progressPercent}%` }" />
      </div>
      <div class="game-boot__meta">
        <span>{{ completedText }}</span>
        <span>{{ progressPercent }}%</span>
      </div>

      <button v-if="gameDataLoadState.phase === 'error'" type="button" class="game-boot__retry" @click="retry">
        {{ t("boot.retry") }}
      </button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, shallowRef, watch, type Component } from "vue"
import { useRoute } from "vue-router"
import { useI18n } from "vue-i18n"
import { preloadGameRouteViewForPath, warmGameRouteViews } from "@/router/game-route-loaders"
import i18n, { GameBackgroundLocaleNamespaces, ensureLocaleNamespaces, getLocaleNamespacesForPath, normalizeSupportedLocale } from "@/i18n"
import { gameDataLoadState, isGameDataLoaded, loadAllGameData, resetGameDataRuntime } from "@services/game-data-runtime"

const route = useRoute()
const { t } = useI18n()
const gameShellComponent = shallowRef<Component | null>(null)
const localeMessagesReady = shallowRef(false)
let gameShellPromise: Promise<void> | null = null

const progressPercent = computed(() => {
  if (gameDataLoadState.loaded) return 100
  if (gameDataLoadState.totalFiles <= 0) return gameDataLoadState.phase === "manifest" ? 5 : 0
  if (gameDataLoadState.totalBytes > 0) {
    return Math.max(1, Math.min(99, Math.round((gameDataLoadState.loadedBytes / gameDataLoadState.totalBytes) * 100)))
  }
  const fileWeight = gameDataLoadState.completedFiles / gameDataLoadState.totalFiles
  return Math.max(1, Math.min(99, Math.round(fileWeight * 100)))
})

const completedText = computed(() => {
  if (gameDataLoadState.totalFiles <= 0) return t("boot.preparingManifest")
  return `${gameDataLoadState.completedFiles}/${gameDataLoadState.totalFiles}`
})

const statusText = computed(() => {
  if (gameDataLoadState.loaded && !localeMessagesReady.value) return t("boot.loadingLocale")
  if (gameDataLoadState.loaded && !gameShellComponent.value) return t("boot.loadingShell")
  if (gameDataLoadState.phase === "manifest") return t("boot.manifest")
  if (gameDataLoadState.phase === "download") {
    return t("boot.downloading", { label: gameDataLoadState.currentFileLabel || gameDataLoadState.currentFileId || t("boot.defaultResource") })
  }
  if (gameDataLoadState.phase === "parse") return t("boot.parsing", { label: gameDataLoadState.currentFileLabel || gameDataLoadState.currentFileId })
  if (gameDataLoadState.phase === "ready") return t("boot.ready")
  if (gameDataLoadState.phase === "error") return t("boot.error", { error: gameDataLoadState.error })
  return t("boot.preparing")
})

onMounted(() => {
  void loadAllGameData()
  void loadBootLocaleMessages()
})

watch(
  () => [isGameDataLoaded.value, localeMessagesReady.value] as const,
  ([dataLoaded, localeLoaded]) => {
    if (dataLoaded && localeLoaded) void loadGameShell()
  },
  { immediate: true }
)

function retry() {
  resetGameDataRuntime()
  void loadAllGameData()
}

async function loadGameShell() {
  if (gameShellComponent.value) return
  if (!gameShellPromise) {
    gameShellPromise = import("@views/game/shell/index.vue")
      .then(async (module) => {
        await preloadGameRouteViewForPath(route.path)
        gameShellComponent.value = module.default
        scheduleRouteWarmup()
      })
      .finally(() => {
        gameShellPromise = null
      })
  }
  await gameShellPromise
}

function scheduleRouteWarmup() {
  window.setTimeout(() => {
    void warmGameRouteViews()
  }, 500)
}

async function loadBootLocaleMessages() {
  const initialLocale = normalizeSupportedLocale(i18n.global.locale.value)
  await ensureLocaleNamespaces(initialLocale, getLocaleNamespacesForPath(route.path))
  localeMessagesReady.value = true
  void ensureLocaleNamespaces(initialLocale, GameBackgroundLocaleNamespaces).catch(() => undefined)
}
</script>

<style scoped>
.game-boot {
  min-height: 100vh;
  min-height: 100svh;
  display: grid;
  place-items: center;
  padding: 28px;
  box-sizing: border-box;
  background: rgb(0 0 0 / 92%);
  color: #ffffff;
}

.game-boot__panel {
  width: min(360px, 100%);
  display: grid;
  gap: 11px;
}

.game-boot__eyebrow {
  color: rgb(255 255 255 / 48%);
  font-size: 11px;
  line-height: 1.4;
}

.game-boot__title {
  margin: 0;
  font-size: 18px;
  line-height: 1.35;
  font-weight: 560;
  color: #ffffff;
}

.game-boot__loading {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 18px;
  margin-top: 2px;
}

.game-boot__spinner {
  width: 12px;
  height: 12px;
  box-sizing: border-box;
  flex: 0 0 auto;
  border: 2px solid rgb(255 255 255 / 20%);
  border-top-color: rgb(255 255 255 / 82%);
  border-radius: 999px;
  animation: game-boot-spin 0.9s linear infinite;
}

.game-boot__status {
  min-width: 0;
  margin: 0;
  color: rgb(255 255 255 / 58%);
  font-size: 12px;
  line-height: 1.5;
}

.game-boot__progress {
  height: 3px;
  overflow: hidden;
  background: rgb(255 255 255 / 12%);
}

.game-boot__progress-bar {
  height: 100%;
  background: rgb(255 255 255 / 78%);
  transition: width 0.18s ease;
}

.game-boot__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-top: 2px;
  color: rgb(255 255 255 / 44%);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.game-boot__retry {
  justify-self: start;
  margin-top: 8px;
  border: 1px solid rgb(255 255 255 / 22%);
  border-radius: 4px;
  background: rgb(255 255 255 / 8%);
  color: #ffffff;
  padding: 6px 12px;
  font-size: 12px;
  line-height: 1.4;
  cursor: pointer;
}

@media (prefers-reduced-motion: reduce) {
  .game-boot__spinner {
    animation: none;
  }
}

@keyframes game-boot-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
