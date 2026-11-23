<template>
  <div class="page-shell combat-trial-page-shell">
    <GamePageFrame
      title-key="game.menu.combatTrial"
      icon="lucide:target"
      tutorial-id="combat-trial"
      :visual-src="trialVisual.src"
      :visual-tone="trialVisual.tone"
      visual-key="combat-trial"
      data-tutorial="combat-trial-page"
    >
      <div class="combat-trial-page" v-loading="overviewLoading">
        <el-tabs v-model="activeTab" class="trial-tabs" data-tutorial="combat-trial-tabs">
          <el-tab-pane name="trial">
            <template #label><span class="trial-tab-label"><Icon icon="lucide:swords" />{{ t("game.combatTrial.tabs.trial") }}</span></template>
            <div v-if="overview && activeFormat" class="trial-workspace">
              <section class="trial-launch-summary">
                <div class="trial-launch-heading">
                  <span><Icon icon="lucide:trophy" />{{ t("game.combatTrial.personalBests") }}</span>
                  <small>{{ t("game.combatTrial.currentVersionOnly") }}</small>
                </div>
                <div class="trial-launch-bests">
                  <div v-for="metric in rankedMetrics" :key="metric.id" class="trial-launch-best" :class="`trial-launch-best--${metric.aggregation}`">
                    <span><Icon :icon="metric.aggregation === 'max' ? 'lucide:zap' : metric.rankDirection === 'asc' ? 'lucide:gauge' : 'lucide:sigma'" />{{ metricLabel(metric) }}</span>
                    <strong>{{ formatMetricValue(personalBestValue(metric.id), metric) }}</strong>
                  </div>
                </div>
                <div class="trial-launch-rule">
                  <Icon :icon="activeFormat.icon || 'lucide:target'" />
                  <span>
                    <strong>{{ localized(activeFormat.nameKey, activeFormat.id) }}</strong>
                    <small>{{ t("game.combatTrial.ruleActionBudget", { count: activeFormat.actionBudget }) }}</small>
                  </span>
                </div>
                <div class="trial-launch-actions">
                  <span v-if="overview.activeRun" class="trial-active-run"><Icon icon="lucide:loader-circle" />{{ activeRunLabel }}</span>
                  <span v-else-if="!selectionValid" class="trial-invalid-lineup"><Icon icon="lucide:triangle-alert" />{{ t("game.combatTrial.lineupInvalid") }}</span>
                  <span v-else class="trial-ranked-note"><Icon icon="lucide:badge-check" />{{ t("game.combatTrial.rankedNote") }}</span>
                  <el-button type="primary" class="trial-launch-start" data-tutorial="combat-trial-start" :loading="starting" :disabled="!selectionValid || Boolean(overview.activeRun)" @click="startTrial">
                    <Icon icon="lucide:play" />{{ t("game.combatTrial.start") }}
                  </el-button>
                </div>
              </section>

              <section class="trial-projects" data-tutorial="combat-trial-projects">
                <header class="trial-projects__heading">
                  <span class="trial-projects__heading-icon"><Icon icon="lucide:layout-grid" /></span>
                  <div>
                    <span>{{ t("game.combatTrial.formatMode") }}</span>
                    <small>{{ t("game.combatTrial.formatHint") }}</small>
                  </div>
                  <el-tooltip :content="t('game.combatTrial.refresh')" placement="top">
                    <el-button circle :loading="refreshing" :aria-label="t('game.combatTrial.refresh')" @click="refreshCurrentTab"><Icon icon="lucide:refresh-cw" /></el-button>
                  </el-tooltip>
                </header>
                <div class="trial-project-list" role="radiogroup" :aria-label="t('game.combatTrial.formatMode')">
                  <article
                    v-for="format in overview.formats"
                    :key="format.id"
                    class="trial-project-card"
                    :class="{ 'is-selected': selection.formatId === format.id }"
                    role="radio"
                    tabindex="0"
                    :aria-checked="selection.formatId === format.id"
                    aria-controls="combat-trial-project-detail"
                    :aria-label="`${localized(format.nameKey, format.id)}。${localized(format.shortDescKey, format.id)}`"
                    @click="selectFormat(format.id)"
                    @keydown.enter.prevent="selectFormat(format.id)"
                    @keydown.space.prevent="selectFormat(format.id)"
                  >
                    <header class="trial-project-card__header">
                      <span class="trial-project-card__icon"><Icon :icon="format.icon || 'lucide:target'" /></span>
                      <span class="trial-project-card__copy">
                        <strong>{{ localized(format.nameKey, format.id) }}</strong>
                        <small>{{ localized(format.shortDescKey, format.id) }}</small>
                      </span>
                      <el-tooltip :content="t('game.combatTrial.leaderboard')" placement="top">
                        <el-button circle class="trial-project-card__ranking" :aria-label="t('game.combatTrial.leaderboard')" @click.stop="openProjectLeaderboard(format.id)">
                          <Icon icon="lucide:trophy" />
                        </el-button>
                      </el-tooltip>
                      <span class="trial-project-card__state"><Icon :icon="selection.formatId === format.id ? 'lucide:check' : 'lucide:chevron-right'" /></span>
                    </header>
                  </article>
                </div>
                <div id="combat-trial-project-detail" :key="activeFormat.id" class="trial-project-detail" data-tutorial="combat-trial-rules" aria-live="polite">
                  <header class="trial-project-detail__header">
                    <span class="trial-project-detail__icon"><Icon :icon="activeFormat.icon || 'lucide:target'" /></span>
                    <span class="trial-project-detail__copy">
                      <strong>{{ localized(activeFormat.nameKey, activeFormat.id) }}</strong>
                      <small>{{ localized(activeFormat.descKey, activeFormat.id) }}</small>
                    </span>
                    <span class="trial-project-detail__count">
                      <Icon :icon="formatEncounterIcon(activeFormat)" />
                      {{ formatEncounterCountLabel(activeFormat) }}
                    </span>
                  </header>
                  <div class="trial-project-detail__meta">
                    <span><Icon icon="lucide:swords" />{{ t("game.combatTrial.ruleActionBudget", { count: activeFormat.actionBudget }) }}</span>
                    <span><Icon icon="lucide:rows-3" />{{ lineupPositionRule(activeFormat) }}</span>
                    <span v-for="metric in rankedMetricsForFormat(activeFormat)" :key="metric.id" class="trial-project-detail__metric">
                      <Icon :icon="metric.aggregation === 'max' ? 'lucide:zap' : metric.rankDirection === 'asc' ? 'lucide:gauge' : 'lucide:sigma'" />
                      {{ metricLabel(metric) }}
                    </span>
                  </div>
                  <div class="trial-project-targets" :class="{ 'trial-project-targets--waves': encounterWavesForFormat(activeFormat).length }">
                    <div v-if="encounterWavesForFormat(activeFormat).length" class="trial-wave-browser">
                      <header class="trial-wave-browser__header">
                        <span class="trial-wave-browser__heading">
                          <Icon :icon="formatEncounterIcon(activeFormat)" />
                          <span><strong>{{ t("game.combatTrial.opponents") }}</strong><small>{{ formatEncounterCountLabel(activeFormat) }}</small></span>
                        </span>
                        <span class="trial-wave-browser__controls">
                          <strong>{{ t("game.combatTrial.waveProgress", { current: selectedWaveIndexForFormat(activeFormat) + 1, count: encounterWavesForFormat(activeFormat).length }) }}</strong>
                          <button
                            type="button"
                            :title="t('game.combatTrial.previousWave')"
                            :aria-label="t('game.combatTrial.previousWave')"
                            :disabled="selectedWaveIndexForFormat(activeFormat) === 0"
                            @click="stepSelectedWave(activeFormat, -1)"
                          >
                            <Icon icon="lucide:chevron-left" />
                          </button>
                          <button
                            type="button"
                            :title="t('game.combatTrial.nextWave')"
                            :aria-label="t('game.combatTrial.nextWave')"
                            :disabled="selectedWaveIndexForFormat(activeFormat) >= encounterWavesForFormat(activeFormat).length - 1"
                            @click="stepSelectedWave(activeFormat, 1)"
                          >
                            <Icon icon="lucide:chevron-right" />
                          </button>
                        </span>
                      </header>
                      <div class="trial-project-waves" :aria-label="t('game.combatTrial.waveNavigator')">
                        <button
                          v-for="(wave, waveIndex) in encounterWavesForFormat(activeFormat)"
                          :id="waveButtonID(activeFormat.id, waveIndex)"
                          :key="wave.id"
                          type="button"
                          class="trial-project-wave"
                          :class="{ 'is-selected': selectedWaveIndexForFormat(activeFormat) === waveIndex }"
                          :title="waveButtonLabel(activeFormat, wave, waveIndex)"
                          :aria-label="waveButtonLabel(activeFormat, wave, waveIndex)"
                          :aria-pressed="selectedWaveIndexForFormat(activeFormat) === waveIndex"
                          @click="selectWave(activeFormat, waveIndex)"
                        >
                          <span class="trial-project-wave__index">{{ waveIndex + 1 }}</span>
                          <span class="trial-project-wave__copy">
                            <strong>{{ encounterWaveLabel(wave, waveIndex) }}</strong>
                            <small>{{ waveDifficultyLabel(activeFormat, wave) }} · {{ t("game.combatTrial.waveTargets", { count: objectiveUnitsForWave(wave).length }) }}</small>
                          </span>
                        </button>
                      </div>
                      <div class="trial-wave-detail">
                        <div class="trial-wave-detail__summary">
                          <span><Icon icon="game-icons:crossed-swords" /></span>
                          <div>
                            <small>{{ t("game.combatTrial.waveNumber", { count: selectedWaveIndexForFormat(activeFormat) + 1 }) }}</small>
                            <strong>{{ selectedWaveLabel(activeFormat) }}</strong>
                            <small>{{ selectedWaveMeta(activeFormat) }}</small>
                          </div>
                        </div>
                        <div class="trial-wave-detail__units">
                          <button
                            v-for="unit in selectedObjectiveUnitsForFormat(activeFormat)"
                            :key="unit.id"
                            type="button"
                            class="trial-wave-unit"
                            :title="encounterUnitLabel(unit)"
                            :aria-label="`${encounterUnitLabel(unit)}。${t('game.combatTrial.rules')}`"
                            @click="openTargetDetail(activeFormat, unit)"
                          >
                            <Icon :icon="encounterUnitIcon(unit)" />
                            <span><strong>{{ encounterUnitLabel(unit) }}</strong><small>{{ positionLabel(unit.position) }}</small></span>
                            <Icon icon="lucide:chevron-right" />
                          </button>
                        </div>
                      </div>
                    </div>
                    <template v-else>
                      <div class="trial-project-targets__label">
                        <span><Icon :icon="formatEncounterIcon(activeFormat)" />{{ t("game.combatTrial.opponents") }}</span>
                        <strong>{{ formatEncounterCountLabel(activeFormat) }}</strong>
                      </div>
                      <div class="trial-project-targets__list">
                        <button
                          v-for="unit in encounterUnitsForFormat(activeFormat)"
                          :key="unit.id"
                          type="button"
                          class="trial-project-target"
                          :aria-label="`${encounterUnitLabel(unit)}。${t('game.combatTrial.rules')}`"
                          @click="openTargetDetail(activeFormat, unit)"
                        >
                          <span><Icon :icon="encounterUnitIcon(unit)" /></span>
                          <strong>{{ encounterUnitLabel(unit) }}</strong>
                          <small>{{ positionLabel(unit.position) }}</small>
                          <Icon class="trial-project-target__info" icon="lucide:info" />
                        </button>
                      </div>
                    </template>
                  </div>
                </div>
              </section>

              <section class="trial-preparation" data-tutorial="combat-trial-lineup">
                <div class="trial-party">
                  <header class="trial-party-heading">
                    <div>
                      <strong>{{ t("game.combatTrial.lineup") }}</strong>
                      <small>{{ t(lineupHintKey) }}</small>
                    </div>
                    <span>{{ formationCount }} / 3</span>
                  </header>
                  <div class="trial-party-rows">
                    <div class="trial-party-row">
                      <div class="trial-party-member">
                        <span><Icon icon="lucide:user" /></span>
                        <div><strong>{{ playerDisplayName }}</strong><small>{{ t("game.combatTrial.currentPlayer") }}</small></div>
                      </div>
                      <span class="trial-party-row__spacer" />
                      <el-segmented v-model="selection.playerPosition" :options="positionOptions" />
                    </div>
                    <div v-for="slot in 2" :key="slot" class="trial-party-row">
                      <div class="trial-party-member">
                        <span><Icon icon="game-icons:cat" /></span>
                        <div><strong>{{ selectedKitty(slot - 1)?.name || t("game.combatTrial.kittySlot", { slot }) }}</strong><small>{{ selectedKitty(slot - 1) ? t("game.combatTrial.kitty") : t("game.combatTrial.noKitty") }}</small></div>
                      </div>
                      <el-select v-model="selection.kitties[slot - 1].kittyId" :placeholder="t('game.combatTrial.noKitty')" @change="handleKittyChange(slot - 1)">
                        <el-option :label="t('game.combatTrial.noKitty')" :value="0" />
                        <el-option v-for="kitty in overview.kitties" :key="kitty.id" :label="`${kitty.name} · Lv.${kitty.level}`" :value="kitty.id" :disabled="kittySelectedElsewhere(kitty.id, slot - 1)" />
                      </el-select>
                      <el-segmented v-model="selection.kitties[slot - 1].position" :options="positionOptions" :disabled="selection.kitties[slot - 1].kittyId <= 0 || Boolean(selectedKitty(slot - 1)?.positionLocked)" />
                      <el-tooltip v-if="selectedKitty(slot - 1)?.positionLocked" :content="t('game.combatTrial.positionLocked')" placement="top">
                        <span class="trial-position-lock"><Icon icon="lucide:lock-keyhole" /></span>
                      </el-tooltip>
                    </div>
                  </div>
                </div>
              </section>
            </div>
          </el-tab-pane>

          <el-tab-pane name="leaderboard">
            <template #label><span class="trial-tab-label"><Icon icon="lucide:trophy" />{{ t("game.combatTrial.tabs.leaderboard") }}</span></template>
            <section class="trial-pane" data-tutorial="combat-trial-leaderboard" v-loading="leaderboardLoading">
              <header class="trial-pane-toolbar">
                <div>
                  <h2>{{ t("game.combatTrial.leaderboard") }}</h2>
                  <span>{{ t("game.combatTrial.leaderboardPrivacy") }}</span>
                </div>
                <div class="trial-leaderboard-controls">
                  <el-select :model-value="selection.formatId" @change="handleLeaderboardFormatChange">
                    <el-option v-for="format in overview?.formats || []" :key="format.id" :label="localized(format.nameKey, format.id)" :value="format.id" />
                  </el-select>
                  <el-segmented v-model="leaderboardMetricID" :options="metricOptions" />
                </div>
              </header>
              <div v-if="leaderboard?.self" class="trial-self-rank">
                <span>{{ t("game.combatTrial.yourRank") }}</span>
                <strong>#{{ leaderboard.self.rank }}</strong>
                <span>{{ formatMetricByID(leaderboard.self.value, leaderboardMetricID) }}</span>
              </div>
              <div class="trial-table-wrap">
                <table class="trial-table">
                  <thead>
                    <tr>
                      <th>{{ t("game.combatTrial.rank") }}</th>
                      <th>{{ t("game.combatTrial.player") }}</th>
                      <th>{{ selectedMetricLabel }}</th>
                      <th v-for="metricId in secondaryMetricIDs" :key="metricId">{{ metricLabelByID(metricId) }}</th>
                      <th>{{ t("game.combatTrial.kittyCount") }}</th>
                      <th>{{ t("game.combatTrial.achievedAt") }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="entry in leaderboard?.entries || []" :key="entry.rank" :class="{ 'is-self': leaderboard?.self?.rank === entry.rank }">
                      <td><strong>#{{ entry.rank }}</strong></td>
                      <td>{{ entry.username }}</td>
                      <td class="is-primary-metric">{{ formatMetricByID(entry.value, leaderboardMetricID) }}</td>
                      <td v-for="metricId in secondaryMetricIDs" :key="metricId">{{ formatMetricByID(metricValue(entry.metrics, metricId), metricId) }}</td>
                      <td><Icon icon="game-icons:cat" />{{ entry.kittyCount }}</td>
                      <td>{{ formatDate(entry.achievedAt) }}</td>
                    </tr>
                  </tbody>
                </table>
                <el-empty v-if="!leaderboardLoading && !(leaderboard?.entries.length)" :description="t('game.combatTrial.leaderboardEmpty')" />
              </div>
            </section>
          </el-tab-pane>

          <el-tab-pane name="history">
            <template #label><span class="trial-tab-label"><Icon icon="lucide:history" />{{ t("game.combatTrial.tabs.history") }}</span></template>
            <section class="trial-pane" data-tutorial="combat-trial-history" v-loading="historyLoading">
              <header class="trial-pane-toolbar">
                <div>
                  <h2>{{ t("game.combatTrial.history") }}</h2>
                  <span>{{ t("game.combatTrial.historyLimit", { count: overview?.rules.historyKeepCount || 50 }) }}</span>
                </div>
              </header>
              <div class="trial-history-list">
                <article v-for="run in runs" :key="run.id" class="trial-history-row">
                  <div class="trial-history-id">
                    <strong>#{{ run.id }}</strong>
                    <span :class="`is-${run.status}`">{{ runStatusLabel(run) }}</span>
                    <small>{{ runFormatLabel(run) }}</small>
                    <small>{{ formatDate(run.completedAt || run.createdAt) }}</small>
                  </div>
                  <div class="trial-history-metrics">
                    <div v-for="metric in runMetricEntries(run)" :key="metric.id" class="trial-history-score">
                      <span>{{ metricLabelByID(metric.id, run) }}</span>
                      <strong>{{ formatMetricByID(metric.value, metric.id, run) }}</strong>
                    </div>
                  </div>
                  <div class="trial-history-actions-count">
                    <span>{{ t("game.combatTrial.actionProgress") }}</span>
                    <strong>{{ run.actionCount }} / {{ run.actionBudget }}</strong>
                  </div>
                  <div class="trial-history-buttons">
                    <el-tooltip :content="t('game.combatTrial.watchReplay')" placement="top">
                      <el-button circle :disabled="run.status !== 'completed'" :aria-label="t('game.combatTrial.watchReplay')" @click="openReplay(run.id)">
                        <Icon icon="lucide:play" />
                      </el-button>
                    </el-tooltip>
                    <el-tooltip :content="t('game.combatTrial.shareReplay')" placement="top">
                      <el-button circle :loading="sharingRunID === run.id" :disabled="run.status !== 'completed'" :aria-label="t('game.combatTrial.shareReplay')" @click="shareReplay(run)">
                        <Icon icon="ep:chat-dot-square" />
                      </el-button>
                    </el-tooltip>
                  </div>
                </article>
                <el-empty v-if="!historyLoading && runs.length === 0" :description="t('game.combatTrial.historyEmpty')" />
              </div>
            </section>
          </el-tab-pane>
        </el-tabs>
      </div>
    </GamePageFrame>
    <CombatTrialTargetDetailDialog
      v-model="targetDetailVisible"
      :format="targetDetailFormat"
      :unit="targetDetailUnit"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import CombatTrialTargetDetailDialog from "./CombatTrialTargetDetailDialog.vue"
import { apiGet, apiPost } from "@services/api"
import {
  activeCombatTrialKittySelections,
  isCombatTrialLineupSelectionValid,
  isCombatTrialMetricAvailableForEndReason,
  isCombatTrialRunTerminal,
  normalizeCombatTrialMetrics,
  type CombatTrialEncounterWave,
  type CombatTrialEncounterUnit,
  type CombatTrialFormat,
  type CombatTrialKitty,
  type CombatTrialLeaderboardPayload,
  type CombatTrialLineupSelection,
  type CombatTrialMetric,
  type CombatTrialOverview,
  type CombatTrialPosition,
  type CombatTrialReplaySharePayload,
  type CombatTrialRun,
  type CombatTrialRunsPayload
} from "@services/combat-trial"
import {
  cancelPreparedCombatTrialReplay,
  openCombatTrialReplay,
  prepareCombatTrialReplayPlayback
} from "@services/combat-trial-replay-viewer"
import { resolveErrorMessage } from "@services/error-messages"
import { gamePageVisuals } from "@services/game-page-visuals"
import { useChatStore } from "@stores/chat"
import { useGameStore } from "@stores/game"
import { formatNumberWithThousands } from "@utils/number-format"

type TrialTab = "trial" | "leaderboard" | "history"

const positions: CombatTrialPosition[] = ["vanguard", "midguard", "rearguard"]
const { t, locale } = useI18n()
const store = useGameStore()
const chatStore = useChatStore()
const trialVisual = gamePageVisuals.combat
const activeTab = ref<TrialTab>("trial")
const overview = ref<CombatTrialOverview | null>(null)
const overviewLoading = ref(true)
const refreshing = ref(false)
const starting = ref(false)
const historyLoading = ref(false)
const runs = ref<CombatTrialRun[]>([])
const leaderboardLoading = ref(false)
const leaderboard = ref<CombatTrialLeaderboardPayload | null>(null)
const leaderboardMetricID = ref("")
const sharingRunID = ref(0)
const targetDetailVisible = ref(false)
const targetDetailFormat = ref<CombatTrialFormat | null>(null)
const targetDetailUnit = ref<CombatTrialEncounterUnit | null>(null)
const selectedWaveIndices = reactive<Record<string, number>>({})
const selection = reactive<CombatTrialLineupSelection>({
  playerPosition: "vanguard",
  formatId: "standard",
  kitties: [
    { kittyId: 0, position: "midguard" },
    { kittyId: 0, position: "rearguard" }
  ]
})
let historyTimer: ReturnType<typeof setTimeout> | null = null
let overviewRequestSerial = 0
let leaderboardRequestSerial = 0

const activeFormat = computed<CombatTrialFormat | null>(() => {
  const formats = overview.value?.formats || []
  return formats.find((format) => format.id === selection.formatId) || formats[0] || null
})
const playerDisplayName = computed(() => store.username || t("game.combatTrial.currentPlayer"))
const positionOptions = computed(() => positions.map((position) => ({ label: positionLabel(position), value: position })))
const selectionValid = computed(() => overview.value ? isCombatTrialLineupSelectionValid(selection, overview.value.kitties, activeFormat.value) : false)
const rankedMetrics = computed(() => (activeFormat.value?.metrics || []).filter((metric) => metric.ranked !== false))
const metricOptions = computed(() => rankedMetrics.value.map((metric) => ({ label: metricLabel(metric), value: metric.id })))
const secondaryMetricIDs = computed(() => rankedMetrics.value.filter((metric) => metric.id !== leaderboardMetricID.value).map((metric) => metric.id))
const selectedMetricLabel = computed(() => metricLabelByID(leaderboardMetricID.value))
const formationCount = computed(() => 1 + activeCombatTrialKittySelections(selection).length)
const lineupHintKey = computed(() => activeFormat.value?.lineupRules?.positionPolicy === "distinct"
  ? "game.combatTrial.lineupHintDistinct"
  : "game.combatTrial.lineupHintShared")
const activeRunLabel = computed(() => overview.value?.activeRun?.status === "running"
  ? t("game.combatTrial.runRunning", { id: overview.value.activeRun.id })
  : t("game.combatTrial.runQueued", { id: overview.value?.activeRun?.id || "-" }))

onMounted(() => {
  store.restoreToken()
  void loadOverview()
})

onBeforeUnmount(clearHistoryPoll)

watch(activeTab, (tab) => {
  if (tab === "leaderboard") void loadLeaderboard()
  if (tab === "history") void loadHistory()
})

watch(leaderboardMetricID, () => {
  if (activeTab.value === "leaderboard") void loadLeaderboard()
})

async function loadOverview(silent = false, formatId = "") {
  if (!store.token) return
  const serial = ++overviewRequestSerial
  const hydrateFromServer = overview.value === null
  if (!silent) overviewLoading.value = true
  try {
    const query = formatId ? `?formatId=${encodeURIComponent(formatId)}` : ""
    const payload = await apiGet<CombatTrialOverview>(`/api/combat-trials/overview${query}`, store.token)
    if (serial !== overviewRequestSerial) return
    overview.value = payload
    if (hydrateFromServer) hydrateSelection(payload.selection)
    const format = payload.formats.find((entry) => entry.id === selection.formatId) || payload.formats[0]
    const rankable = (format?.metrics || []).filter((metric) => metric.ranked !== false)
    if (format && !rankable.some((metric) => metric.id === leaderboardMetricID.value)) {
      leaderboardMetricID.value = rankable[0]?.id || ""
    }
    if (payload.activeRun && !isCombatTrialRunTerminal(payload.activeRun.status)) scheduleHistoryPoll()
  } catch (error) {
    if (serial === overviewRequestSerial && !silent) ElMessage.error(resolveErrorMessage(t, error, "game.combatTrial.error.overviewFailed"))
  } finally {
    if (serial === overviewRequestSerial) overviewLoading.value = false
  }
}

async function handleFormatChange() {
  leaderboard.value = null
  const firstMetric = rankedMetrics.value[0]?.id || ""
  leaderboardMetricID.value = firstMetric
  await loadOverview(true, selection.formatId)
  if (activeTab.value === "leaderboard") await loadLeaderboard()
}

function selectFormat(formatId: string) {
  if (selection.formatId === formatId) return
  selection.formatId = formatId
  void handleFormatChange()
}

function handleLeaderboardFormatChange(raw: unknown) {
  const formatId = String(raw || "").trim()
  if (formatId) selectFormat(formatId)
}

async function openProjectLeaderboard(formatId: string) {
  if (selection.formatId !== formatId) {
    selection.formatId = formatId
    await handleFormatChange()
  }
  activeTab.value = "leaderboard"
}

function openTargetDetail(format: CombatTrialFormat, unit: CombatTrialEncounterUnit) {
  targetDetailFormat.value = format
  targetDetailUnit.value = unit
  targetDetailVisible.value = true
}

function hydrateSelection(saved: CombatTrialLineupSelection) {
  selection.playerPosition = normalizePosition(saved?.playerPosition) || "vanguard"
  selection.formatId = String(saved?.formatId || "standard").trim() || "standard"
  const selected = Array.isArray(saved?.kitties) ? saved.kitties.slice(0, 2) : []
  selection.kitties[0] = selected[0] ? { kittyId: Number(selected[0].kittyId) || 0, position: normalizePosition(selected[0].position) || "midguard" } : { kittyId: 0, position: "midguard" }
  selection.kitties[1] = selected[1] ? { kittyId: Number(selected[1].kittyId) || 0, position: normalizePosition(selected[1].position) || "rearguard" } : { kittyId: 0, position: "rearguard" }
  normalizeEditorPositions()
}

async function startTrial() {
  if (!store.token || !selectionValid.value || overview.value?.activeRun || starting.value) return
  starting.value = true
  try {
    await prepareCombatTrialReplayPlayback()
    const payload = await apiPost<{ battleKind: "combat_trial"; run: CombatTrialRun }>("/api/combat-trials/runs", store.token, {
      selection: serializedSelection()
    })
    if (overview.value) overview.value.activeRun = payload.run
    openCombatTrialReplay({ runId: payload.run.id, live: true })
    ElMessage.success(t("game.combatTrial.runQueuedToast"))
    scheduleHistoryPoll()
  } catch (error) {
    cancelPreparedCombatTrialReplay()
    ElMessage.error(resolveErrorMessage(t, error, "game.combatTrial.error.runCreateFailed"))
    void loadOverview(true, selection.formatId)
  } finally {
    starting.value = false
  }
}

async function loadHistory(silent = false) {
  if (!store.token || historyLoading.value) return
  if (!silent) historyLoading.value = true
  try {
    const payload = await apiGet<CombatTrialRunsPayload>("/api/combat-trials/runs", store.token)
    runs.value = payload.runs.slice(0, payload.historyKeepCount || 50)
    if (runs.value.some((run) => !isCombatTrialRunTerminal(run.status))) scheduleHistoryPoll()
    else clearHistoryPoll()
  } catch (error) {
    if (!silent) ElMessage.error(resolveErrorMessage(t, error, "game.combatTrial.error.historyFailed"))
  } finally {
    historyLoading.value = false
  }
}

async function loadLeaderboard() {
  if (!store.token || !activeFormat.value) return
  const serial = ++leaderboardRequestSerial
  const formatID = activeFormat.value.id
  const metricID = leaderboardMetricID.value
  leaderboardLoading.value = true
  try {
    const payload = await apiGet<CombatTrialLeaderboardPayload>(
      `/api/combat-trials/leaderboard?formatId=${encodeURIComponent(formatID)}&metricId=${encodeURIComponent(metricID)}`,
      store.token
    )
    if (serial === leaderboardRequestSerial) leaderboard.value = payload
  } catch (error) {
    if (serial === leaderboardRequestSerial) ElMessage.error(resolveErrorMessage(t, error, "game.combatTrial.error.leaderboardFailed"))
  } finally {
    if (serial === leaderboardRequestSerial) leaderboardLoading.value = false
  }
}

async function refreshCurrentTab() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    await loadOverview(true, selection.formatId)
    if (activeTab.value === "history") await loadHistory(true)
    if (activeTab.value === "leaderboard") await loadLeaderboard()
  } finally {
    refreshing.value = false
  }
}

function scheduleHistoryPoll() {
  clearHistoryPoll()
  historyTimer = setTimeout(async () => {
    await Promise.all([loadOverview(true, selection.formatId), loadHistory(true)])
    if (overview.value?.activeRun) scheduleHistoryPoll()
  }, 1200)
}

function clearHistoryPoll() {
  if (historyTimer) clearTimeout(historyTimer)
  historyTimer = null
}

async function shareReplay(run: CombatTrialRun) {
  if (!store.token || run.status !== "completed" || sharingRunID.value > 0) return
  sharingRunID.value = run.id
  try {
    const payload = await apiPost<CombatTrialReplaySharePayload>(`/api/combat-trials/runs/${run.id}/share`, store.token)
    const channel = chatStore.canSendToChannel(chatStore.activeChannel)
      ? chatStore.activeChannel
      : chatStore.channels.find((item) => item.playerSendable)?.id || chatStore.defaultChannel
    if (!chatStore.canSendToChannel(channel)) throw new Error("no sendable chat channel")
    chatStore.setActiveChannel(channel)
    chatStore.setPanelOpen(true)
    const outcomeLabelKey = formatForRun(run)?.endConditions?.find((condition) => condition.reason === run.endReason)?.labelKey
    chatStore.setPendingAttachment({
      type: "combat_trial_replay",
      battleKind: "combat_trial",
      token: payload.share.token,
      runId: payload.share.runId,
      createdBy: payload.share.createdBy,
      createdByName: payload.share.createdByName,
      outcome: payload.share.endReason,
      ...(outcomeLabelKey ? { outcomeLabelKey } : {}),
      metrics: payload.share.metrics
    })
    ElMessage.success(t("game.combatTrial.shareReplaySuccess"))
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.combatTrial.error.replayShareFailed"))
  } finally {
    sharingRunID.value = 0
  }
}

function openReplay(runId: number) {
  openCombatTrialReplay({ runId })
}

function serializedSelection(): CombatTrialLineupSelection {
  return {
    playerPosition: selection.playerPosition,
    formatId: selection.formatId,
    kitties: activeCombatTrialKittySelections(selection)
  }
}

function selectedKitty(index: number): CombatTrialKitty | undefined {
  const kittyId = Number(selection.kitties[index]?.kittyId || 0)
  return overview.value?.kitties.find((kitty) => kitty.id === kittyId)
}

function handleKittyChange(index: number) {
  const kitty = selectedKitty(index)
  const locked = normalizePosition(kitty?.positionLocked)
  if (locked) selection.kitties[index].position = locked
  normalizeEditorPositions()
}

function normalizeEditorPositions() {
  for (let index = 0; index < selection.kitties.length; index += 1) {
    const locked = normalizePosition(selectedKitty(index)?.positionLocked)
    if (locked) selection.kitties[index].position = locked
  }
}

function kittySelectedElsewhere(kittyId: number, index: number): boolean {
  return selection.kitties.some((entry, entryIndex) => entryIndex !== index && entry.kittyId === kittyId)
}

function personalBestValue(metricId: string): number | null {
  return overview.value?.personalBests.find((best) => best.metricId === metricId)?.value ?? null
}

function metricValue(metrics: unknown, metricId: string): number | null {
  const normalized = normalizeCombatTrialMetrics(metrics)
  return Object.prototype.hasOwnProperty.call(normalized, metricId) ? normalized[metricId] : null
}

function runMetricEntries(run: CombatTrialRun): Array<{ id: string; value: number | null }> {
  const metrics = normalizeCombatTrialMetrics(run.metrics)
  const definitions = formatForRun(run)?.metrics || []
  const ids = definitions.map((metric) => metric.id)
  const seen = new Set(ids)
  for (const metricId of Object.keys(metrics).sort()) {
    if (!seen.has(metricId)) ids.push(metricId)
  }
  return ids.map((id) => {
    const metric = definitions.find((entry) => entry.id === id)
    const available = !metric || isCombatTrialMetricAvailableForEndReason(metric, run.endReason)
    return { id, value: available ? metrics[id] || 0 : null }
  })
}

function metricLabel(metric: CombatTrialMetric): string {
  return localized(metric.shortNameKey || metric.nameKey, metric.id)
}

function formatForRun(run: CombatTrialRun): CombatTrialFormat | NonNullable<CombatTrialRun["format"]> | null {
  if (run.format?.definitionDigest === run.formatDefinitionDigest) return run.format
  return overview.value?.formats.find((format) => format.definitionDigest === run.formatDefinitionDigest)
    || null
}

function metricLabelByID(metricId: string, run?: CombatTrialRun): string {
  const metric = (run ? formatForRun(run) : activeFormat.value)?.metrics.find((entry) => entry.id === metricId)
  return metric ? metricLabel(metric) : metricId
}

function encounterUnitLabel(unit: CombatTrialEncounterUnit): string {
  return localized(unit.nameKey, unit.enemyId || unit.id)
}

function rankedMetricsForFormat(format: CombatTrialFormat): CombatTrialMetric[] {
  return (format.metrics || []).filter((metric) => metric.ranked !== false)
}

function encounterUnitsForFormat(format: CombatTrialFormat): CombatTrialEncounterUnit[] {
  if (format.encounter?.units?.length) return format.encounter.units.filter((unit) => unit.objectiveTarget !== false)
  const dummy = format.dummy
  return (dummy?.positions || []).map((position, index) => ({
    id: `legacy_dummy_${index + 1}`,
    kind: "training_target",
    nameKey: `game.combatTrial.dummies.${position}`,
    position,
    behavior: "passive",
    healthMode: "immutable_full",
    objectiveTarget: true,
    defenseSource: dummy?.defenseSource,
    defense: dummy?.defense,
    maxHp: dummy?.maxHp,
    damageResist: dummy?.damageResist,
    criticalDamageResist: dummy?.criticalDamageResist
  }))
}

function encounterWavesForFormat(format: CombatTrialFormat): CombatTrialEncounterWave[] {
  return format.encounter?.waves || []
}

function objectiveUnitsForWave(wave: CombatTrialEncounterWave): CombatTrialEncounterUnit[] {
  return wave.units.filter((unit) => unit.objectiveTarget !== false)
}

function selectedWaveIndexForFormat(format: CombatTrialFormat): number {
  const lastIndex = encounterWavesForFormat(format).length - 1
  if (lastIndex < 0) return 0
  const selected = Math.trunc(Number(selectedWaveIndices[format.id] ?? 0))
  return Math.min(Math.max(selected, 0), lastIndex)
}

function selectedWaveForFormat(format: CombatTrialFormat): CombatTrialEncounterWave | null {
  return encounterWavesForFormat(format)[selectedWaveIndexForFormat(format)] || null
}

function selectedObjectiveUnitsForFormat(format: CombatTrialFormat): CombatTrialEncounterUnit[] {
  const wave = selectedWaveForFormat(format)
  return wave ? objectiveUnitsForWave(wave) : []
}

function selectedWaveLabel(format: CombatTrialFormat): string {
  const index = selectedWaveIndexForFormat(format)
  const wave = encounterWavesForFormat(format)[index]
  return wave ? encounterWaveLabel(wave, index) : ""
}

function selectedWaveMeta(format: CombatTrialFormat): string {
  const wave = selectedWaveForFormat(format)
  if (!wave) return ""
  return `${waveDifficultyLabel(format, wave)} · ${t("game.combatTrial.waveTargets", { count: objectiveUnitsForWave(wave).length })}`
}

function waveButtonLabel(format: CombatTrialFormat, wave: CombatTrialEncounterWave, index: number): string {
  return `${t("game.combatTrial.waveNumber", { count: index + 1 })}。${encounterWaveLabel(wave, index)}。${waveDifficultyLabel(format, wave)}。${t("game.combatTrial.waveTargets", { count: objectiveUnitsForWave(wave).length })}`
}

function selectWave(format: CombatTrialFormat, index: number) {
  const lastIndex = encounterWavesForFormat(format).length - 1
  if (lastIndex < 0) return
  const selected = Math.min(Math.max(Math.trunc(index), 0), lastIndex)
  selectedWaveIndices[format.id] = selected
  scrollWaveIntoView(format.id, selected)
}

function stepSelectedWave(format: CombatTrialFormat, offset: number) {
  selectWave(format, selectedWaveIndexForFormat(format) + offset)
}

function waveButtonID(formatID: string, index: number): string {
  const safeFormatID = encodeURIComponent(formatID)
  return `combat-trial-wave-${safeFormatID}-${index}`
}

function scrollWaveIntoView(formatID: string, index: number) {
  if (typeof window === "undefined") return
  window.requestAnimationFrame(() => {
    const button = document.getElementById(waveButtonID(formatID, index))
    if (!button) return
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches
    button.scrollIntoView({ behavior: reduceMotion ? "auto" : "smooth", block: "nearest", inline: "center" })
  })
}

function encounterWaveLabel(wave: CombatTrialEncounterWave, index: number): string {
  return localized(wave.nameKey, t("game.combatTrial.waveNumber", { count: index + 1 }))
}

function waveDifficultyLabel(format: CombatTrialFormat, wave: CombatTrialEncounterWave): string {
  const level = Math.max(0, Math.trunc(Number(wave.enemyStatProfile?.difficultyLevel ?? format.encounter?.enemyStatProfile?.difficultyLevel ?? 0)))
  return level > 0 ? `N${level}` : t("game.combatTrial.active")
}

function formatEncounterCountLabel(format: CombatTrialFormat): string {
  const waveCount = encounterWavesForFormat(format).length
  return waveCount > 0 ? t("game.combatTrial.waveCount", { count: waveCount }) : String(encounterUnitsForFormat(format).length)
}

function encounterUnitIcon(unit: CombatTrialEncounterUnit): string {
  if (unit.kind === "training_target") return "lucide:target"
  if (unit.kind === "scripted_ally") return "lucide:heart-handshake"
  return "game-icons:crossed-swords"
}

function formatEncounterIcon(format: CombatTrialFormat): string {
  if (encounterWavesForFormat(format).length > 0) return "lucide:repeat-2"
  const units = encounterUnitsForFormat(format)
  if (units.some((unit) => unit.kind === "scripted_ally")) return "lucide:heart-pulse"
  return units.length > 0 && units.every((unit) => unit.kind === "training_target")
    ? "lucide:target"
    : "game-icons:crossed-swords"
}

function formatMetricValue(value: number | null | undefined, metric: CombatTrialMetric): string {
  if (value === null || value === undefined) return "-"
  if (metric.valueFormat === "integer") return formatNumber(Math.trunc(Number(value) || 0))
  return formatNumber(value)
}

function formatMetricByID(value: number | null | undefined, metricId: string, run?: CombatTrialRun): string {
  const metric = (run ? formatForRun(run) : activeFormat.value)?.metrics.find((entry) => entry.id === metricId)
  if (metric) return formatMetricValue(value, metric)
  return value === null || value === undefined ? "-" : formatNumber(value)
}

function positionLabel(position: CombatTrialPosition): string {
  return t(`game.combatTrial.position.${position}`)
}

function lineupPositionRule(format: CombatTrialFormat): string {
  return format.lineupRules?.positionPolicy === "distinct"
    ? t("game.combatTrial.rulePositionDistinct")
    : t("game.combatTrial.rulePositionShared")
}

function normalizePosition(raw: unknown): CombatTrialPosition | "" {
  const value = String(raw || "").trim().toLowerCase()
  return positions.includes(value as CombatTrialPosition) ? value as CombatTrialPosition : ""
}

function localized(key: string | undefined, fallback: string): string {
  const normalized = String(key || "").trim()
  if (!normalized) return fallback
  const translated = t(normalized)
  return translated && translated !== normalized ? translated : fallback
}

function runStatusLabel(run: CombatTrialRun): string {
  if (run.status === "queued") return t("game.combatTrial.status.queued")
  if (run.status === "running") return t("game.combatTrial.status.running")
  if (run.status === "failed") return t("game.combatTrial.status.failed")
  const condition = formatForRun(run)?.endConditions?.find((entry) => entry.reason === run.endReason)
  if (condition?.labelKey) return localized(condition.labelKey, t("game.combatTrial.status.completed"))
  if (run.endReason === "team_defeated") return t("game.combatTrial.endReason.teamDefeated")
  if (run.endReason === "enemy_team_defeated") return t("game.combatTrial.endReason.enemyTeamDefeated")
  if (run.endReason === "all_waves_cleared") return t("game.combatTrial.endReason.allWavesCleared")
  if (run.endReason === "action_budget_reached") return t("game.combatTrial.endReason.actionBudgetReached")
  return t("game.combatTrial.status.completed")
}

function runFormatLabel(run: CombatTrialRun): string {
  const format = formatForRun(run)
  return format ? localized(format.nameKey, format.id) : run.formatId
}

function formatDate(raw: string | undefined): string {
  const date = new Date(String(raw || ""))
  if (Number.isNaN(date.getTime())) return "-"
  return new Intl.DateTimeFormat(locale.value, { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(date)
}

function formatNumber(value: number): string {
  return formatNumberWithThousands(Math.round(Number(value) || 0))
}

</script>

<style scoped src="./index.css"></style>
