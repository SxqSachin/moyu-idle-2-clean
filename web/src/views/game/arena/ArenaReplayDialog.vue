<template>
  <GameDialog
    :model-value="modelValue"
    class="arena-replay-dialog"
    width="min(1380px, 97vw)"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <template #header>
      <div class="arena-replay-header">
        <span class="arena-kind-mark"><Icon :icon="isCombatTrial ? 'lucide:target' : 'game-icons:cat'" />{{ t(isCombatTrial ? "game.combatTrial.kind" : "game.arena.kind") }}</span>
        <div class="arena-replay-header__copy">
          <h2>{{ t(isCombatTrial ? "game.combatTrial.replayTitle" : "game.arena.replayTitle", { id: activeChallengeId || "-" }) }}</h2>
        </div>
      </div>
    </template>

    <div class="arena-replay-body">
      <div v-if="loading && !runState" class="arena-replay-loading" role="status" aria-live="polite">
        <span class="arena-replay-spinner" aria-hidden="true" />
        <span>{{ t("game.arena.replayLoading") }}</span>
      </div>

      <el-empty v-else-if="errorMessage && !runState" :description="errorMessage" />

      <template v-else-if="runState">
        <div class="arena-replay-summary">
          <el-tag :type="outcomeTagType" effect="dark">{{ outcomeLabel }}</el-tag>
          <el-tooltip :content="activeArenaFormatDescription" placement="bottom">
            <span
              class="arena-replay-format"
              :class="{ 'arena-replay-format--standard': activeArenaFormat.standard }"
            >
              <Icon :icon="activeArenaFormat.icon" />
              {{ activeArenaFormatName }}
            </span>
          </el-tooltip>
          <span v-if="live && !playbackComplete" class="arena-replay-live">
            <i aria-hidden="true" />{{ t("game.arena.replayLive") }}
          </span>
          <span>{{ t("game.arena.replayStep", { current: playedEventCount, total: totalEventCount }) }}</span>
        </div>

        <div v-if="isCombatTrial" class="combat-trial-replay-metrics">
          <div v-for="metric in activeTrialMetricDefinitions" :key="metric.id">
            <span>{{ trialMetricLabel(metric) }}</span>
            <strong>{{ trialMetricValueLabel(metric) }}</strong>
          </div>
          <div>
            <span>{{ t("game.combatTrial.actionProgress") }}</span>
            <strong>{{ activeTrialActionCount }} <small>/ {{ activeTrialActionBudget }}</small></strong>
          </div>
        </div>

        <section class="arena-replay-scoreboard" :aria-label="t('game.arena.replayTeamStatus')">
          <div class="arena-replay-team arena-replay-team--attacker">
            <div class="arena-replay-team__heading">
              <span><Icon icon="lucide:swords" />{{ t(isCombatTrial ? "game.combatTrial.challengers" : "game.arena.attacker") }}</span>
              <strong>{{ t("game.arena.replayAliveCount", { alive: attackerStatus.alive, total: attackerStatus.total }) }}</strong>
            </div>
            <div class="arena-replay-team__hp">
              <i :style="{ width: `${attackerStatus.percent}%` }" />
            </div>
            <small>{{ formatNumber(attackerStatus.hp) }} / {{ formatNumber(attackerStatus.maxHp) }} HP</small>
          </div>

          <div class="arena-replay-scoreboard__turn">
            <span>{{ t("game.arena.replayCurrentTurn") }}</span>
            <strong>{{ currentTurnNumber }} <small>/ {{ replayAnalysis.turns.length }}</small></strong>
          </div>

          <div class="arena-replay-team arena-replay-team--defender">
            <div class="arena-replay-team__heading">
              <span><Icon :icon="isCombatTrial ? 'lucide:target' : 'lucide:shield'" />{{ t(isCombatTrial ? "game.combatTrial.opponents" : "game.arena.defender") }}</span>
              <strong>{{ t("game.arena.replayAliveCount", { alive: defenderStatus.alive, total: defenderStatus.total }) }}</strong>
            </div>
            <div class="arena-replay-team__hp">
              <i :style="{ width: `${defenderStatus.percent}%` }" />
            </div>
            <small v-if="isCombatTrial && trialOpponentHealthLocked">{{ t("game.combatTrial.immutable") }}</small>
            <small v-else>{{ formatNumber(defenderStatus.hp) }} / {{ formatNumber(defenderStatus.maxHp) }} HP</small>
          </div>
        </section>

        <nav class="arena-replay-controls" :aria-label="t('game.arena.replayControls')">
          <div class="arena-replay-controls__buttons">
            <el-tooltip :content="t('game.arena.replayJumpStart')" placement="top">
              <el-button circle :disabled="currentTurnNumber <= 0" :aria-label="t('game.arena.replayJumpStart')" @click="seekToStart">
                <Icon icon="lucide:skip-back" />
              </el-button>
            </el-tooltip>
            <el-tooltip :content="t('game.arena.replayPrevious')" placement="top">
              <el-button circle :disabled="currentTurnNumber <= 0" :aria-label="t('game.arena.replayPrevious')" @click="playPreviousTurn">
                <Icon icon="lucide:step-back" />
              </el-button>
            </el-tooltip>
            <el-tooltip :content="t(isPlaying ? 'game.arena.replayPause' : 'game.arena.replayPlay')" placement="top">
              <el-button
                class="arena-replay-controls__primary"
                type="primary"
                circle
                :disabled="totalEventCount <= 0"
                :aria-label="t(isPlaying ? 'game.arena.replayPause' : 'game.arena.replayPlay')"
                @click="togglePlayback"
              >
                <Icon :icon="isPlaying ? 'lucide:pause' : 'lucide:play'" />
              </el-button>
            </el-tooltip>
            <el-tooltip :content="t('game.arena.replayNext')" placement="top">
              <el-button circle :disabled="currentTurnNumber >= replayAnalysis.turns.length" :aria-label="t('game.arena.replayNext')" @click="playNextTurn">
                <Icon icon="lucide:step-forward" />
              </el-button>
            </el-tooltip>
            <el-tooltip :content="t('game.arena.replayJumpEnd')" placement="top">
              <el-button circle :disabled="currentTurnNumber >= replayAnalysis.turns.length" :aria-label="t('game.arena.replayJumpEnd')" @click="seekToEnd">
                <Icon icon="lucide:skip-forward" />
              </el-button>
            </el-tooltip>
          </div>

          <div class="arena-replay-controls__scrubber">
            <span>{{ currentTurnLabel }}</span>
            <el-slider
              :model-value="currentTurnNumber"
              :min="0"
              :max="Math.max(1, replayAnalysis.turns.length)"
              :step="1"
              :show-tooltip="false"
              :disabled="replayAnalysis.turns.length <= 0"
              @input="handleTurnSliderInput"
              @change="handleTurnSliderChange"
            />
          </div>

          <div class="arena-replay-controls__speed">
            <span>{{ t("game.arena.replaySpeed") }}</span>
            <el-segmented v-model="playbackSpeed" size="small" :options="playbackSpeedOptions" />
          </div>
        </nav>

        <div class="arena-replay-workspace">
          <section class="arena-replay-battlefield" :aria-label="t('game.arena.replayBattlefield')">
            <header>
              <div>
                <Icon icon="lucide:scan-line" />
                <strong>{{ t("game.arena.replayBattlefield") }}</strong>
              </div>
              <span v-if="selectedTurn">{{ selectedActorLabel }} · {{ selectedTurnLabel }}</span>
              <span v-else>{{ t("game.arena.replayInitialState") }}</span>
            </header>
            <BattleView
              :key="battleViewKey"
              :source="battleViewSource"
              :animate-events="animateBattleEvents"
              :playback-rate="playbackSpeed"
              embedded
            />
          </section>

          <ArenaReplayTimeline
            class="arena-replay-timeline"
            :turns="replayAnalysis.turns"
            :current-turn="currentTurnNumber"
            @select="playTurn"
            @scrub="handleTurnSliderInput"
          />

          <ArenaReplayInspector class="arena-replay-inspector" :analysis="replayAnalysis" :selected-turn="selectedTurn" :battle-kind="props.battleKind" />
        </div>
      </template>

      <el-empty v-else :description="t('game.arena.replayUnavailable')" />
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import GameDialog from "@components/game/common/GameDialog.vue"
import { getLocalizedSkillName } from "@services/skill-definition"
import { formatNumberWithThousands } from "@utils/number-format"
import { apiGet } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { bind } from "@services/ws"
import {
  battlePayloadStreamKind,
  expandBattleEventType,
  normalizeBattleEventData,
  parseRunStateLoose,
  type BattleCombatTrialMetricState,
  type BattleRunBatchPayload,
  type BattleRunEventPayload,
  type BattleRunStatePayload,
  type BattleUnit
} from "@composables/game/combat/useCombatPage"
import type { BattleViewSource } from "@views/game/combat/useCombatViewContext"
import BattleView from "@views/game/combat/views/BattleView.vue"
import ArenaReplayInspector from "./ArenaReplayInspector.vue"
import ArenaReplayTimeline from "./ArenaReplayTimeline.vue"
import type { ArenaReplayPayload, ArenaSharedReplayPayload } from "@services/arena"
import {
  isCombatTrialMetricAvailableForEndReason,
  type CombatTrialReplayPayload,
  type CombatTrialSharedReplayPayload
} from "@services/combat-trial"
import {
  buildArenaReplayAnalysis,
  type ArenaReplayQueuedEvent,
  type ArenaReplayTurnSummary
} from "@services/arena-replay-analysis"
import { applyArenaReplayEventBatch, cloneArenaBattleState } from "@services/arena-replay-state"
import { useGameStore } from "@stores/game"

type QueuedArenaEvent = ArenaReplayQueuedEvent

type LiveReplayCache = {
  key: string
  challengeId: number
  snapshot: BattleRunStatePayload | null
  events: QueuedArenaEvent[]
  outcome: string
  finished: boolean
  touchedAt: number
}

const props = withDefaults(defineProps<{
  modelValue: boolean
  battleKind?: "arena" | "combat_trial"
  challengeId?: number
  runId?: number
  shareToken?: string
  live?: boolean
  version?: number
}>(), {
  battleKind: "arena",
  challengeId: 0,
  runId: 0,
  shareToken: "",
  live: false,
  version: 0
})

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void
}>()

const { t } = useI18n()
const store = useGameStore()
const isCombatTrial = computed(() => props.battleKind === "combat_trial")
const activeStreamKind = computed(() => isCombatTrial.value ? "combat_trial" : "arena")
const loading = ref(false)
const errorMessage = ref("")
const activeChallengeId = ref(0)
const outcome = ref("")
const replayMetrics = ref<Record<string, number>>({})
const replayActionCount = ref(0)
const replayActionBudget = ref(0)
const runState = ref<BattleRunStatePayload | null>(null)
const runEvents = ref<BattleRunEventPayload[]>([])
const selectedRun = ref<Record<string, unknown> | null>(null)
const selectedRoom = ref<Record<string, unknown> | null>(null)
const roomsLoading = ref(false)
const runLoading = computed(() => loading.value && !runState.value)
const worlds = ref<never[]>([])
const interBattleWait = ref(null)
const battleViewKey = ref(0)
const animateBattleEvents = ref(true)
const isPlaying = ref(true)
const playbackSpeed = ref(1)
const playedEventCount = ref(0)
const totalEventCount = ref(0)
const timelineRevision = ref(0)
const replayCaches = new Map<string, LiveReplayCache>()
const wsUnbinders: Array<() => void> = []
let activeCache: LiveReplayCache | null = null
let playbackCursor = 0
let playbackTimer: ReturnType<typeof setTimeout> | null = null
let fallbackTimer: ReturnType<typeof setTimeout> | null = null
let requestSerial = 0
let fallbackAttempts = 0
let navigationSerial = 0
let scrubFrame = 0
let pendingScrubTurn = 0

const battleViewSource = {
  worlds,
  selectedRoom,
  selectedRun,
  runState,
  runEvents,
  roomsLoading,
  runLoading,
  roomStatusLabel: () => "",
  runStatusLabel: () => "",
  positionLabel: (position: string) => t(`${isCombatTrial.value ? "game.combatTrial" : "game.arena"}.position.${position}`),
  selectRoom: async () => undefined,
  interBattleWait
} as unknown as BattleViewSource

const playbackComplete = computed(() => Boolean(outcome.value && totalEventCount.value > 0 && playedEventCount.value >= totalEventCount.value))
const replayAnalysis = computed(() => {
  timelineRevision.value
  return buildArenaReplayAnalysis(activeCache?.events || [], activeCache?.snapshot || null)
})
const currentTurnNumber = computed(() => {
  let current = 0
  for (const turn of replayAnalysis.value.turns) {
    if (playedEventCount.value <= turn.eventStart) break
    current = turn.index
  }
  return current
})
const selectedTurn = computed<ArenaReplayTurnSummary | null>(() => replayAnalysis.value.turns[currentTurnNumber.value - 1] || null)
const currentTurnLabel = computed(() => currentTurnNumber.value > 0
  ? t("game.arena.replayTurnNumber", { turn: currentTurnNumber.value })
  : t("game.arena.replayInitialState"))
const selectedTurnLabel = computed(() => {
  const turn = selectedTurn.value
  if (!turn) return t("game.arena.replayNoAction")
  if (!turn.skillId && !turn.skillNameFallback) return t("game.arena.replayNoAction")
  return getLocalizedSkillName(t, turn.skillId, turn.skillNameFallback || turn.skillId)
})
const selectedActorLabel = computed(() => {
  const name = String(selectedTurn.value?.actorName || "").trim()
  if (!name) return t("game.arena.unknownUnit")
  const translated = t(name)
  return translated && translated !== name ? translated : name
})
const attackerStatus = computed(() => summarizeTeam(runState.value?.players || []))
const defenderStatus = computed(() => summarizeTeam(runState.value?.enemies || []))
const playbackSpeedOptions = computed(() => [
  { label: "0.5x", value: 0.5 },
  { label: "1x", value: 1 },
  { label: "2x", value: 2 }
])
const visibleOutcome = computed(() => props.live && !playbackComplete.value ? "" : outcome.value)
const activeArenaFormat = computed(() => {
  if (isCombatTrial.value && runState.value?.combatTrialFormat) {
    return { ...runState.value.combatTrialFormat, standard: runState.value.combatTrialFormat.id === "standard" }
  }
  return runState.value?.arenaFormat || {
  id: "standard",
  nameKey: isCombatTrial.value ? "game.combatTrial.formats.standard.name" : "game.arena.formats.standard.name",
  shortDescKey: isCombatTrial.value ? "game.combatTrial.formats.standard.shortDesc" : "game.arena.formats.standard.shortDesc",
  descKey: isCombatTrial.value ? "game.combatTrial.formats.standard.desc" : "game.arena.formats.standard.desc",
  icon: isCombatTrial.value ? "lucide:target" : "lucide:circle-off",
  standard: true
  }
})
const activeArenaFormatName = computed(() => t(activeArenaFormat.value.nameKey))
const activeArenaFormatDescription = computed(() => t(activeArenaFormat.value.descKey || activeArenaFormat.value.nameKey))
const activeTrialMetrics = computed(() => {
  const progress = runState.value?.combatTrial
  if ((progress?.progressVersion || 0) > 0) return progress?.metrics || {}
  if (!props.live && Object.keys(replayMetrics.value).length > 0) return replayMetrics.value
  const liveMetrics = runState.value?.combatTrial?.metrics
  return liveMetrics && Object.keys(liveMetrics).length > 0 ? liveMetrics : replayMetrics.value
})
const activeTrialMetricDefinitions = computed<BattleCombatTrialMetricState[]>(() => {
  const metrics = runState.value?.combatTrialFormat?.metrics || []
  if (metrics.length > 0) return metrics
  return Object.keys(activeTrialMetrics.value).sort().map((id) => ({ id }))
})
const authoredTrialOutcomeLabel = computed(() => {
  const condition = runState.value?.combatTrialFormat?.endConditions?.find((entry) => entry.reason === visibleOutcome.value)
  const key = String(condition?.labelKey || "").trim()
  if (!key) return ""
  const translated = t(key)
  return translated && translated !== key ? translated : ""
})
const trialOpponentHealthLocked = computed(() => {
  const units = runState.value?.combatTrialFormat?.encounter?.units || []
  return units.length > 0 && units.every((unit) => unit.healthMode === "immutable_full")
})
const activeTrialActionCount = computed(() => {
  const progress = runState.value?.combatTrial
  if ((progress?.progressVersion || 0) > 0) return progress?.actionCount || 0
  return props.live
    ? progress?.actionCount ?? replayActionCount.value
    : replayActionCount.value || progress?.actionCount || 0
})
const activeTrialActionBudget = computed(() => {
  const formatBudget = "actionBudget" in activeArenaFormat.value ? Number(activeArenaFormat.value.actionBudget) : 0
  return runState.value?.combatTrial?.actionBudget || replayActionBudget.value || formatBudget
})
const outcomeLabel = computed(() => isCombatTrial.value
  ? authoredTrialOutcomeLabel.value
    || (visibleOutcome.value === "action_budget_reached"
    ? t("game.combatTrial.endReason.actionBudgetReached")
    : visibleOutcome.value === "team_defeated"
      ? t("game.combatTrial.endReason.teamDefeated")
      : visibleOutcome.value === "enemy_team_defeated"
        ? t("game.combatTrial.endReason.enemyTeamDefeated")
        : visibleOutcome.value === "failed"
          ? t("game.combatTrial.failed")
          : visibleOutcome.value
            ? t("game.combatTrial.status.completed")
            : t("game.combatTrial.replayInProgress"))
  : visibleOutcome.value === "attacker_win"
  ? t("game.arena.attackerWin")
  : visibleOutcome.value === "defender_win"
    ? t("game.arena.defenderWin")
    : visibleOutcome.value === "failed"
      ? t("game.arena.failed")
      : t("game.arena.replayInProgress"))
const outcomeTagType = computed(() => isCombatTrial.value
  ? visibleOutcome.value === "failed" ? "danger" : visibleOutcome.value ? "success" : "info"
  : visibleOutcome.value === "attacker_win" ? "success" : visibleOutcome.value === "defender_win" ? "warning" : visibleOutcome.value === "failed" ? "danger" : "info")

function trialMetricLabel(metric: BattleCombatTrialMetricState): string {
  const key = String(metric.shortNameKey || metric.nameKey || "").trim()
  if (!key) return metric.id
  const translated = t(key)
  return translated && translated !== key ? translated : metric.id
}

function trialMetricValueLabel(metric: BattleCombatTrialMetricState): string {
  if (visibleOutcome.value && !isCombatTrialMetricAvailableForEndReason(metric, visibleOutcome.value)) return "-"
  return formatNumber(activeTrialMetrics.value[metric.id] || 0)
}

watch(
  () => [props.modelValue, props.battleKind, props.challengeId, props.runId, props.shareToken, props.live, props.version] as const,
  ([visible]) => {
    if (!visible) {
      stopPlayback()
      return
    }
    void openRequestedReplay()
  },
  { immediate: true }
)

watch(playbackSpeed, () => {
  if (!isPlaying.value || !playbackTimer) return
  clearPlaybackTimer()
  pumpPlayback()
})

onMounted(() => {
  wsUnbinders.push(bind("battle.run.batch", cacheArenaBatch))
  wsUnbinders.push(bind("battle.run.started", cacheArenaLifecycle))
  wsUnbinders.push(bind("battle.run.compiled", cacheArenaLifecycle))
  wsUnbinders.push(bind("battle.run.finished", cacheArenaFinished))
})

onBeforeUnmount(() => {
  stopPlayback()
  for (const unbind of wsUnbinders.splice(0)) unbind()
})

function ensureCache(challengeId: number): LiveReplayCache {
  const key = `${activeStreamKind.value}:${challengeId}`
  const existing = replayCaches.get(key)
  if (existing) {
    existing.touchedAt = Date.now()
    return existing
  }
  const cache: LiveReplayCache = {
    key,
    challengeId,
    snapshot: null,
    events: [],
    outcome: "",
    finished: false,
    touchedAt: Date.now()
  }
  replayCaches.set(key, cache)
  pruneCaches()
  return cache
}

function pruneCaches() {
  const entries = [...replayCaches.values()].sort((a, b) => b.touchedAt - a.touchedAt)
  for (const row of entries.slice(6)) replayCaches.delete(row.key)
}

function cacheArenaLifecycle(payload: unknown) {
  if (battlePayloadStreamKind(payload) !== activeStreamKind.value) return
  const data = payload as { challengeId?: number; roomId?: number; runId?: number }
  const challengeId = Math.max(0, Math.trunc(Number(data.challengeId || data.roomId || data.runId || 0)))
  if (!challengeId) return
  ensureCache(challengeId)
}

function cacheArenaFinished(payload: unknown) {
  if (battlePayloadStreamKind(payload) !== activeStreamKind.value) return
  const data = payload as { challengeId?: number; roomId?: number; runId?: number; outcome?: string }
  const challengeId = Math.max(0, Math.trunc(Number(data.challengeId || data.roomId || data.runId || 0)))
  if (!challengeId) return
  const cache = ensureCache(challengeId)
  cache.finished = true
  cache.outcome = String(data.outcome || cache.outcome || "")
  if (activeCache === cache) {
    outcome.value = cache.outcome
    if (cache.outcome === "failed" && !cache.snapshot) {
      loading.value = false
      errorMessage.value = t(isCombatTrial.value ? "game.combatTrial.replayUnavailable" : "game.arena.replayUnavailable")
      if (fallbackTimer) clearTimeout(fallbackTimer)
      fallbackTimer = null
      return
    }
    pumpPlayback()
  }
}

function cacheArenaBatch(payload: unknown) {
  if (battlePayloadStreamKind(payload) !== activeStreamKind.value) return
  const data = payload as BattleRunBatchPayload
  const challengeId = Math.max(0, Math.trunc(Number(data.m || data.r || 0)))
  if (!challengeId) return
  const cache = ensureCache(challengeId)
  if (data.n) {
    const snapshot = parseRunStateLoose(challengeId, data.n as Record<string, unknown>)
    if (!cache.snapshot || snapshot.seq >= cache.snapshot.seq) {
      cache.snapshot = snapshot
      if (activeCache === cache) timelineRevision.value += 1
    }
  }
  mergeQueuedEvents(cache, (Array.isArray(data.v) ? data.v : []).map((raw) => {
    const eventType = expandBattleEventType(raw.e)
    return {
      offsetMs: Math.max(0, Math.trunc(Number(raw.o || 0))),
      event: {
        seq: Math.max(0, Math.trunc(Number(raw.q || 0))),
        type: eventType,
        ts: Number(data.s || Date.now()) + Number(raw.o || 0),
        data: normalizeBattleEventData(eventType, raw.d || {})
      }
    }
  }))
  if (activeCache === cache) {
    initializeFromCache()
    pumpPlayback()
  }
}

function mergeQueuedEvents(cache: LiveReplayCache, incoming: QueuedArenaEvent[]) {
  if (incoming.length === 0) return
  const existing = new Set(cache.events.map((row) => row.event.seq))
  for (const row of incoming) {
    if (row.event.seq <= 0 || existing.has(row.event.seq)) continue
    existing.add(row.event.seq)
    cache.events.push(row)
  }
  cache.events.sort((a, b) => a.offsetMs - b.offsetMs || a.event.seq - b.event.seq)
  if (activeCache === cache) {
    totalEventCount.value = cache.events.length
    timelineRevision.value += 1
  }
}

async function openRequestedReplay() {
  stopPlayback()
  const serial = ++requestSerial
  loading.value = true
  errorMessage.value = ""
  outcome.value = ""
  replayMetrics.value = {}
  replayActionCount.value = 0
  replayActionBudget.value = 0
  runState.value = null
  runEvents.value = []
  selectedRoom.value = null
  selectedRun.value = null
  playedEventCount.value = 0
  totalEventCount.value = 0
  playbackCursor = 0
  isPlaying.value = true
  playbackSpeed.value = 1
  animateBattleEvents.value = true
  fallbackAttempts = 0
  activeCache = null
  navigationSerial += 1
  timelineRevision.value += 1
  battleViewKey.value += 1

  const challengeId = Math.max(0, Math.trunc(Number(isCombatTrial.value ? props.runId : props.challengeId || 0)))
  if (props.shareToken.trim()) {
    try {
      const endpoint = isCombatTrial.value
        ? `/api/combat-trials/replays/shared/${encodeURIComponent(props.shareToken.trim())}`
        : `/api/arena/replays/shared/${encodeURIComponent(props.shareToken.trim())}`
      const shared = await apiGet<ArenaSharedReplayPayload | CombatTrialSharedReplayPayload>(endpoint, store.token)
      if (serial !== requestSerial) return
      hydrateCacheFromReplay(shared.payload)
      startCachePlayback()
    } catch (error) {
      if (serial !== requestSerial) return
      errorMessage.value = resolveErrorMessage(t, error, isCombatTrial.value ? "game.combatTrial.error.replaySharedFailed" : "game.arena.error.replaySharedFailed")
    } finally {
      if (serial === requestSerial) loading.value = false
    }
    return
  }

  if (!challengeId) {
    errorMessage.value = t(isCombatTrial.value ? "game.combatTrial.replayUnavailable" : "game.arena.replayUnavailable")
    loading.value = false
    return
  }
  activeChallengeId.value = challengeId
  activeCache = ensureCache(challengeId)
  initializeFromCache()
  if (runState.value) {
    loading.value = false
    pumpPlayback()
  }
  await loadParticipantReplay(serial, Boolean(props.live))
}

async function loadParticipantReplay(serial: number, retry: boolean) {
  try {
    const endpoint = isCombatTrial.value
      ? `/api/combat-trials/runs/${activeChallengeId.value}/replay`
      : `/api/arena/challenges/${activeChallengeId.value}/replay`
    const payload = await apiGet<ArenaReplayPayload | CombatTrialReplayPayload>(endpoint, store.token)
    if (serial !== requestSerial) return
    hydrateCacheFromReplay(payload)
    initializeFromCache()
    loading.value = false
    pumpPlayback()
  } catch (error) {
    if (serial !== requestSerial) return
    if (retry) {
      fallbackAttempts += 1
      if (fallbackAttempts >= 120) {
        loading.value = false
        errorMessage.value = resolveErrorMessage(t, error, isCombatTrial.value ? "game.combatTrial.error.replayFailed" : "game.arena.error.replayFailed")
        return
      }
      fallbackTimer = setTimeout(() => void loadParticipantReplay(serial, true), 700)
      return
    }
    loading.value = false
    errorMessage.value = resolveErrorMessage(t, error, isCombatTrial.value ? "game.combatTrial.error.replayFailed" : "game.arena.error.replayFailed")
  }
}

function hydrateCacheFromReplay(payload: ArenaReplayPayload | CombatTrialReplayPayload) {
  const trialPayload = payload as CombatTrialReplayPayload
  const arenaPayload = payload as ArenaReplayPayload
  const challengeId = Math.max(1, Math.trunc(Number(trialPayload.runId || arenaPayload.challengeId || activeChallengeId.value || 0)))
  activeChallengeId.value = challengeId
  const cache = ensureCache(challengeId)
  cache.snapshot = parseRunStateLoose(challengeId, payload.initialState)
  cache.outcome = String(trialPayload.endReason || arenaPayload.outcome || "")
  cache.finished = payload.status === "completed" || payload.status === "attacker_win" || payload.status === "defender_win" || payload.status === "failed"
  replayMetrics.value = { ...(trialPayload.metrics || {}) }
  replayActionCount.value = Math.max(0, Math.trunc(Number(trialPayload.actionCount) || 0))
  replayActionBudget.value = Math.max(0, Math.trunc(Number(trialPayload.actionBudget) || 0))
  mergeQueuedEvents(cache, (Array.isArray(payload.timeline) ? payload.timeline : []).map((entry) => {
    const eventType = String(entry.payload?.eventType || "").trim()
    const eventData = { ...(entry.payload?.data || {}) }
    if (entry.parentSeq) eventData.parentSeq = entry.parentSeq
    if (entry.groupId) eventData.batchGroup = entry.groupId
    return {
      offsetMs: Math.max(0, Math.trunc(Number(entry.offsetMs || 0))),
      event: {
        seq: Math.max(0, Math.trunc(Number(entry.seq || 0))),
        type: eventType,
        ts: Number(entry.payload?.ts || 0),
        data: normalizeBattleEventData(eventType, eventData)
      }
    }
  }))
  activeCache = cache
  timelineRevision.value += 1
  outcome.value = cache.outcome
}

function startCachePlayback() {
  initializeFromCache()
  loading.value = false
  pumpPlayback()
}

function initializeFromCache() {
  if (!activeCache?.snapshot || runState.value) return
  activeChallengeId.value = activeCache.challengeId
  outcome.value = activeCache.outcome
  runState.value = cloneArenaBattleState(activeCache.snapshot)
  selectedRoom.value = {
    room: {
      id: activeCache.challengeId,
      status: "playing",
      dungeonId: "",
      autoReenterOnDefeat: false,
      isPublic: false
    },
    members: []
  }
  selectedRun.value = {
    id: activeCache.challengeId,
    roomId: activeCache.challengeId,
    wave: runState.value.wave,
    status: "playing",
    state: runState.value
  }
  totalEventCount.value = activeCache.events.length
  void nextTick().then(() => pumpPlayback())
}

function pumpPlayback() {
  if (playbackTimer || !isPlaying.value || !props.modelValue || !activeCache || !runState.value) return
  if (playbackCursor >= activeCache.events.length) {
    totalEventCount.value = activeCache.events.length
    if (activeCache.finished) {
      outcome.value = activeCache.outcome
      isPlaying.value = false
      if (selectedRun.value) selectedRun.value = { ...selectedRun.value, status: "ended" }
    }
    return
  }
  const offset = activeCache.events[playbackCursor]?.offsetMs ?? 0
  const batch: BattleRunEventPayload[] = []
  while (playbackCursor < activeCache.events.length && activeCache.events[playbackCursor]?.offsetMs === offset) {
    const row = activeCache.events[playbackCursor]
    if (row) batch.push(row.event)
    playbackCursor += 1
  }
  if (batch.length > 0) {
    runEvents.value = [...runEvents.value, ...batch]
    runState.value = applyArenaReplayEventBatch(runState.value, batch)
    playedEventCount.value = runEvents.value.length
    totalEventCount.value = Math.max(totalEventCount.value, activeCache.events.length)
  }
  const nextOffset = activeCache.events[playbackCursor]?.offsetMs
  const nextDelay = Number.isFinite(nextOffset)
    ? Math.max(50, Math.min(2000, Number(nextOffset) - offset))
    : 600
  playbackTimer = setTimeout(() => {
    playbackTimer = null
    pumpPlayback()
  }, nextDelay / playbackSpeed.value)
}

function togglePlayback() {
  if (isPlaying.value) {
    isPlaying.value = false
    clearPlaybackTimer()
    return
  }
  if (playbackCursor >= (activeCache?.events.length || 0)) {
    void restartPlayback()
    return
  }
  isPlaying.value = true
  pumpPlayback()
}

async function restartPlayback() {
  const serial = ++navigationSerial
  isPlaying.value = false
  await rebuildToEventCount(0, serial)
  if (serial !== navigationSerial) return
  isPlaying.value = true
  pumpPlayback()
}

async function seekToStart() {
  await seekToTurn(0)
}

async function seekToEnd() {
  await seekToTurn(replayAnalysis.value.turns.length)
}

async function playPreviousTurn() {
  if (currentTurnNumber.value <= 1) {
    await seekToTurn(0)
    return
  }
  await playTurn(currentTurnNumber.value - 1)
}

async function playNextTurn() {
  await playTurn(Math.min(replayAnalysis.value.turns.length, currentTurnNumber.value + 1))
}

async function playTurn(rawTurn: number) {
  const turnNumber = Math.max(1, Math.min(replayAnalysis.value.turns.length, Math.trunc(Number(rawTurn) || 0)))
  const turn = replayAnalysis.value.turns[turnNumber - 1]
  if (!turn || !activeCache?.snapshot) return
  const serial = ++navigationSerial
  isPlaying.value = false
  clearPlaybackTimer()
  await rebuildToEventCount(turn.eventStart, serial)
  if (serial !== navigationSerial || !activeCache || !runState.value) return
  const rows = activeCache.events.slice(turn.eventStart, turn.eventEnd)
  const events = rows.map((row) => row.event)
  runEvents.value = [...runEvents.value, ...events]
  runState.value = applyArenaReplayEventBatch(runState.value, events)
  playbackCursor = turn.eventEnd
  playedEventCount.value = turn.eventEnd
  syncSelectedRunState()
}

async function seekToTurn(rawTurn: number) {
  const turnNumber = Math.max(0, Math.min(replayAnalysis.value.turns.length, Math.trunc(Number(rawTurn) || 0)))
  const eventCount = turnNumber <= 0 ? 0 : replayAnalysis.value.turns[turnNumber - 1]?.eventEnd || 0
  const serial = ++navigationSerial
  isPlaying.value = false
  clearPlaybackTimer()
  await rebuildToEventCount(eventCount, serial)
}

function handleTurnSliderChange(raw: number | number[]) {
  if (scrubFrame) window.cancelAnimationFrame(scrubFrame)
  scrubFrame = 0
  void seekToTurn(normalizeTurnSliderValue(raw))
}

function handleTurnSliderInput(raw: number | number[]) {
  pendingScrubTurn = normalizeTurnSliderValue(raw)
  if (scrubFrame) return
  scrubFrame = window.requestAnimationFrame(() => {
    scrubFrame = 0
    void seekToTurn(pendingScrubTurn)
  })
}

function normalizeTurnSliderValue(raw: number | number[]): number {
  return Array.isArray(raw) ? Number(raw[0] || 0) : Number(raw || 0)
}

async function rebuildToEventCount(rawEventCount: number, serial: number) {
  if (!activeCache?.snapshot) return
  const eventCount = Math.max(0, Math.min(activeCache.events.length, Math.trunc(Number(rawEventCount) || 0)))
  const events = activeCache.events.slice(0, eventCount).map((row) => row.event)
  animateBattleEvents.value = false
  playbackCursor = eventCount
  playedEventCount.value = eventCount
  totalEventCount.value = activeCache.events.length
  runEvents.value = events
  runState.value = applyArenaReplayEventBatch(activeCache.snapshot, events)
  syncSelectedRunState()
  battleViewKey.value += 1
  await nextTick()
  if (serial !== navigationSerial) return
  animateBattleEvents.value = true
  await nextTick()
}

function syncSelectedRunState() {
  if (!selectedRun.value || !runState.value) return
  const ended = Boolean(activeCache?.finished && playbackCursor >= (activeCache?.events.length || 0))
  selectedRun.value = {
    ...selectedRun.value,
    wave: runState.value.wave,
    status: ended ? "ended" : "playing",
    state: runState.value
  }
}

function stopPlayback() {
  requestSerial += 1
  navigationSerial += 1
  isPlaying.value = false
  clearPlaybackTimer()
  if (scrubFrame) window.cancelAnimationFrame(scrubFrame)
  scrubFrame = 0
  if (fallbackTimer) clearTimeout(fallbackTimer)
  fallbackTimer = null
}

function clearPlaybackTimer() {
  if (playbackTimer) clearTimeout(playbackTimer)
  playbackTimer = null
}

function summarizeTeam(units: BattleUnit[]) {
  const rows = Array.isArray(units) ? units : []
  const hp = rows.reduce((total, unit) => total + Math.max(0, Number(unit.hp) || 0), 0)
  const maxHp = rows.reduce((total, unit) => total + Math.max(0, Number(unit.maxHp) || 0), 0)
  return {
    hp,
    maxHp,
    alive: rows.filter((unit) => unit.alive !== false && Number(unit.hp || 0) > 0).length,
    total: rows.length,
    percent: maxHp > 0 ? Math.max(0, Math.min(100, Math.round((hp / maxHp) * 100))) : 0
  }
}

function formatNumber(value: number) {
  return formatNumberWithThousands(Math.round(Number(value) || 0))
}

</script>

<style scoped>
.arena-replay-body {
  --replay-attacker: #2563eb;
  --replay-defender: #b96b08;
  --replay-danger: #c2413b;
  --replay-success: #168447;
  min-height: 520px;
}

.arena-replay-header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.arena-replay-header__copy {
  min-width: 0;
}

.arena-replay-header h2 {
  margin: 0;
  font-size: 1.05rem;
  letter-spacing: 0;
}

.arena-kind-mark {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: none;
  padding: 5px 8px;
  border: 1px solid color-mix(in srgb, var(--el-color-warning) 58%, transparent);
  border-radius: 4px;
  color: var(--el-color-warning-dark-2);
  background: color-mix(in srgb, var(--el-color-warning) 11%, transparent);
  font-size: 0.75rem;
  font-weight: 700;
}

.arena-replay-loading {
  min-height: 440px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--app-muted);
  font-size: 0.86rem;
}

.arena-replay-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid color-mix(in srgb, var(--app-muted) 24%, transparent);
  border-top-color: var(--el-color-primary);
  border-radius: 50%;
  animation: arena-replay-spin 0.8s linear infinite;
}

.arena-replay-summary {
  min-height: 34px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding-bottom: 8px;
  color: var(--app-muted);
  font-size: 0.72rem;
}

.combat-trial-replay-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  margin-bottom: 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-bottom: 0;
  background: color-mix(in srgb, var(--el-fill-color-light) 45%, var(--el-bg-color));
}

.combat-trial-replay-metrics > div {
  min-width: 0;
  display: grid;
  gap: 3px;
  padding: 8px 12px;
  border-right: 1px solid var(--el-border-color-lighter);
}

.combat-trial-replay-metrics > div:last-child {
  border-right: 0;
}

.combat-trial-replay-metrics span,
.combat-trial-replay-metrics small {
  color: var(--app-muted);
  font-size: 0.66rem;
}

.combat-trial-replay-metrics strong {
  overflow: hidden;
  color: var(--el-text-color-primary);
  font-size: 1rem;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-replay-live {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.arena-replay-format {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 6px;
  border: 1px solid color-mix(in srgb, var(--replay-defender) 34%, var(--el-border-color-lighter));
  border-radius: 3px;
  color: var(--replay-defender);
  background: color-mix(in srgb, var(--replay-defender) 7%, transparent);
  font-weight: 700;
  line-height: 1.2;
}

.arena-replay-format--standard {
  border-color: var(--el-border-color-lighter);
  color: var(--app-muted);
  background: var(--el-fill-color-light);
}

.arena-replay-live i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--el-color-danger);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--el-color-danger) 14%, transparent);
}

.arena-replay-scoreboard {
  min-height: 78px;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 112px minmax(0, 1fr);
  align-items: stretch;
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px 6px 0 0;
  background: var(--el-bg-color);
}

.arena-replay-team {
  min-width: 0;
  display: grid;
  align-content: center;
  gap: 6px;
  padding: 10px 14px;
}

.arena-replay-team--attacker {
  border-left: 3px solid var(--replay-attacker);
}

.arena-replay-team--defender {
  border-right: 3px solid var(--replay-defender);
}

.arena-replay-team__heading {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.arena-replay-team__heading span,
.arena-replay-team__heading strong {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.69rem;
}

.arena-replay-team--attacker .arena-replay-team__heading span {
  color: var(--replay-attacker);
}

.arena-replay-team--defender .arena-replay-team__heading span {
  color: var(--replay-defender);
}

.arena-replay-team__heading strong {
  color: var(--el-text-color-primary);
}

.arena-replay-team__hp {
  height: 5px;
  overflow: hidden;
  background: var(--el-fill-color-darker);
}

.arena-replay-team__hp i {
  display: block;
  height: 100%;
  transition: width 0.2s ease;
}

.arena-replay-team--attacker .arena-replay-team__hp i {
  background: var(--replay-attacker);
}

.arena-replay-team--defender .arena-replay-team__hp i {
  margin-left: auto;
  background: var(--replay-defender);
}

.arena-replay-team small {
  color: var(--app-muted);
  font-size: 0.61rem;
  font-variant-numeric: tabular-nums;
}

.arena-replay-team--defender small {
  text-align: right;
}

.arena-replay-scoreboard__turn {
  display: grid;
  place-items: center;
  align-content: center;
  gap: 2px;
  border-inline: 1px solid var(--el-border-color-lighter);
  background: color-mix(in srgb, var(--el-fill-color-light) 72%, var(--el-bg-color));
  text-align: center;
}

.arena-replay-scoreboard__turn > span {
  color: var(--app-muted);
  font-size: 0.62rem;
}

.arena-replay-scoreboard__turn > strong {
  color: var(--el-text-color-primary);
  font-size: 1.14rem;
  font-variant-numeric: tabular-nums;
}

.arena-replay-scoreboard__turn small {
  color: var(--app-muted);
  font-size: 0.68rem;
}

.arena-replay-controls {
  min-height: 58px;
  display: grid;
  grid-template-columns: auto minmax(220px, 1fr) auto;
  align-items: center;
  gap: 16px;
  padding: 8px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-top: 0;
  background: color-mix(in srgb, var(--el-fill-color-light) 48%, var(--el-bg-color));
}

.arena-replay-controls__buttons {
  display: flex;
  align-items: center;
  gap: 5px;
}

.arena-replay-controls__buttons :deep(.el-button + .el-button) {
  margin-left: 0;
}

.arena-replay-controls__primary {
  margin-inline: 2px;
}

.arena-replay-controls__scrubber {
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(100px, 1fr);
  align-items: center;
  gap: 12px;
}

.arena-replay-controls__scrubber > span,
.arena-replay-controls__speed > span {
  color: var(--app-muted);
  font-size: 0.64rem;
  white-space: nowrap;
}

.arena-replay-controls__speed {
  display: flex;
  align-items: center;
  gap: 8px;
}

.arena-replay-controls__speed :deep(.el-segmented) {
  min-width: 142px;
}

.arena-replay-workspace {
  display: grid;
  grid-template-columns: 236px minmax(560px, 1fr) 236px;
  grid-template-areas: "timeline battlefield inspector";
  align-items: start;
  gap: 10px;
  padding-top: 10px;
}

.arena-replay-battlefield {
  grid-area: battlefield;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: var(--el-bg-color);
}

.arena-replay-timeline {
  grid-area: timeline;
}

.arena-replay-inspector {
  grid-area: inspector;
}

.arena-replay-battlefield > header {
  min-height: 42px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 7px 11px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: color-mix(in srgb, var(--el-fill-color-light) 68%, var(--el-bg-color));
}

.arena-replay-battlefield > header > div {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--replay-attacker);
}

.arena-replay-battlefield > header strong {
  color: var(--el-text-color-primary);
  font-size: 0.75rem;
}

.arena-replay-battlefield > header > span {
  overflow: hidden;
  color: var(--app-muted);
  font-size: 0.63rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-replay-battlefield :deep(.battle-view) {
  box-sizing: border-box;
  padding: 0 10px 10px;
}

.arena-replay-battlefield :deep(.battle-detail-tabs > .el-tabs__header) {
  margin-bottom: 10px;
}

.arena-replay-battlefield :deep(.battle-detail-tabs > .el-tabs__header .el-tabs__item) {
  height: 38px;
  font-size: 0.72rem;
}

@keyframes arena-replay-spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 1180px) {
  .arena-replay-workspace {
    grid-template-columns: minmax(0, 1fr) 250px;
    grid-template-areas:
      "battlefield inspector"
      "timeline timeline";
  }
}

@media (max-width: 920px) {
  .arena-replay-workspace {
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas:
      "battlefield"
      "timeline"
      "inspector";
  }

  .arena-replay-controls {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .arena-replay-controls__scrubber {
    grid-column: 1 / -1;
    grid-row: 2;
  }

  .arena-replay-controls__speed {
    justify-self: end;
  }
}

@media (max-width: 620px) {
  .arena-replay-body,
  .arena-replay-loading {
    min-height: 360px;
  }

  .arena-replay-scoreboard {
    grid-template-columns: minmax(0, 1fr) 68px minmax(0, 1fr);
  }

  .combat-trial-replay-metrics {
    grid-template-columns: minmax(0, 1fr);
  }

  .combat-trial-replay-metrics > div {
    border-right: 0;
    border-bottom: 1px solid var(--el-border-color-lighter);
  }

  .combat-trial-replay-metrics > div:last-child {
    border-bottom: 0;
  }

  .arena-replay-team {
    padding-inline: 8px;
  }

  .arena-replay-team__heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .arena-replay-team__heading strong {
    font-size: 0.59rem;
  }

  .arena-replay-team small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .arena-replay-controls {
    grid-template-columns: minmax(0, 1fr);
    gap: 9px;
  }

  .arena-replay-controls__buttons {
    justify-content: center;
  }

  .arena-replay-controls__scrubber {
    grid-column: auto;
    grid-row: auto;
  }

  .arena-replay-controls__speed {
    width: 100%;
    justify-content: space-between;
  }

  .arena-replay-controls__speed :deep(.el-segmented) {
    min-width: min(190px, 68vw);
  }

  .arena-replay-battlefield > header {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .arena-replay-battlefield :deep(.battle-view) {
    padding-inline: 6px;
  }

}

@media (prefers-reduced-motion: reduce) {
  .arena-replay-team__hp i {
    transition: none;
  }
}
</style>
