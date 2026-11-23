<template>
  <div class="battle-view space-y-3" data-tutorial="combat-tab-battle">
    <!-- <div class="text-sm app-muted">{{ t("game.battleBattleSubtitle") }}</div> -->

      <div v-if="showInitialBattleLoading" class="battle-loading-placeholder" role="status" aria-live="polite">
        <span class="battle-loading-spinner" aria-hidden="true"></span>
        <span>{{ t("game.battleBattleLoading") }}</span>
      </div>
      <el-empty v-else-if="!selectedRoom" :description="t('game.battleBattleNoSelection')" />
      <template v-else>
	        <div v-if="!embedded" class="rounded border border-[var(--app-border)] p-3 space-y-3">
	          <div class="flex flex-wrap items-center gap-2">
	            <el-tag size="small">{{ roomStatusLabel(selectedRoom.room.status) }}</el-tag>
	            <el-tag v-if="selectedRoom.room.status === 'playing'" size="small" type="warning">
	              {{ t("game.battleCurrentWave") }} {{ currentWave }}
	            </el-tag>
	            <el-tag v-if="selectedRun" size="small" type="success">{{ runStatusLabel(selectedRun.status) }}</el-tag>
	          </div>
	          <div v-if="enteringProgress" class="space-y-1">
	            <div class="text-xs app-muted">
	              {{ t("game.battleProgressEntering") }} · {{ t("game.battleProgressRemaining", { time: enteringProgress.remainingLabel }) }}
	            </div>
	            <el-progress :percentage="enteringProgress.percentage" :stroke-width="10" :show-text="false" />
	          </div>
	          <div v-if="recoveringProgress" class="space-y-1">
	            <div class="text-xs app-muted">
	              {{ t("game.battleProgressRecovering") }} · {{ t("game.battleProgressRemaining", { time: recoveringProgress.remainingLabel }) }}
	            </div>
	            <el-progress :percentage="recoveringProgress.percentage" :stroke-width="10" :show-text="false" status="warning" />
	          </div>
	          <div class="text-xs app-muted">
	            {{ t("game.battleRoomAutoReenterOnDefeat") }}：{{ selectedRoom.room.autoReenterOnDefeat ? t("game.switchOn") : t("game.switchOff") }} ·
	            <span v-if="selectedRoom.room.status === 'recovering'">
	              {{ t("game.battleRecoveringUntil") }}：{{ recoveryEndsAtLabel }} ·
	            </span>
            {{ t("game.battleRoomVisibility") }}：{{ selectedRoom.room.isPublic ? t("game.battleRoomPublic") : t("game.battleRoomPrivate") }}
          </div>
        </div>

        <el-tabs v-model="battleDetailTab" class="battle-detail-tabs">
          <el-tab-pane :label="t('game.battleDetailAnimationTab')" name="animation">
        <div v-if="shouldRenderBattleCanvas" ref="battleCanvasRef" class="battle-canvas relative rounded border border-[var(--app-border)] p-3 min-h-[360px] overflow-hidden">
          <template v-if="interBattleState">
            <div class="space-y-2 mb-4">
              <div class="flex items-center gap-2 text-xs app-muted" :class="{ 'battle-intermission-loading': interBattleState.phase === 'preparing' }" role="status" aria-live="polite">
                <span v-if="interBattleState.phase === 'preparing'" class="battle-loading-spinner" aria-hidden="true"></span>
                {{ interBattleState.phase === "preparing" ? t("game.battleBattleLoading") : t("game.battleInterBattleWaiting", { time: interBattleState.remainingLabel }) }}
              </div>
              <div v-if="interBattleState.phase === 'waiting'" class="battle-intermission-progress h-[3px] overflow-hidden rounded-full bg-[var(--el-fill-color-darker)]">
                <div
                  class="h-full rounded-full bg-[var(--el-color-primary)] opacity-40"
                  :style="{ width: `${interBattleState.percentage}%`, transition: 'width 0.1s linear' }"
                />
              </div>
            </div>
          </template>
          <template v-else>
            <div
              v-if="battlefieldEffectRows.length > 0"
              class="battlefield-effect-strip mb-3"
              :class="{ 'battlefield-effect-strip--triggered': battlefieldTriggerActive }"
              role="status"
              aria-live="polite"
            >
              <div
                v-for="effect in battlefieldEffectRows"
                :key="effect.id"
                class="battlefield-effect-row"
                :class="{ 'battlefield-effect-row--warning': effect.displayMode === 'countdown' && effect.remaining <= 3 }"
              >
                <Icon :icon="effect.icon" class="battlefield-effect-icon" aria-hidden="true" />
                <span class="battlefield-effect-name">{{ effect.name }}</span>
                <strong v-if="effect.displayMode === 'countdown'" class="battlefield-effect-count">{{ effect.remaining }} → 0</strong>
                <span class="battlefield-effect-progress">
                  {{ t(effect.displayMode === "progress" ? "game.battleFieldCurrentProgress" : "game.battleFieldRoundProgress", { progress: effect.progress, goal: effect.goal }) }}
                </span>
                <span v-if="effect.exposedRemaining > 0" class="battlefield-effect-window">
                  {{ t("game.battleFieldTimekeeperExposed") }} {{ effect.exposedRemaining }} → 0
                </span>
                <span v-if="battlefieldTriggerActive && battlefieldTriggerSummary" class="battlefield-effect-trigger-summary">
                  {{ battlefieldTriggerSummary }}
                </span>
              </div>
            </div>
            <div v-if="preparationTurnOrderEntries.length > 0" class="flex flex-row flex-wrap items-center gap-2 mb-2">
              <div class="text-xs">{{ t("game.battlePreparationTurnOrder") }}: </div>
              <div class="flex flex-wrap gap-1.5">
                <el-tag
                  v-for="entry in preparationTurnOrderEntries"
                  :key="`prep-turn-order-${entry.unitId}`"
                  :disable-transitions="true"
                  type="success"
                  effect="plain"
                  :class="[
                    'battle-turn-order-tag',
                    hoveredTurnOrderUnitId === entry.unitId ? 'battle-turn-order-tag-hovered' : ''
                  ]"
                  :style="entry.isAlive ? undefined : { opacity: '0.45', filter: 'grayscale(1)' }"
                  @mouseenter="hoveredTurnOrderUnitId = entry.unitId"
                  @mouseleave="hoveredTurnOrderUnitId = ''"
                >
                  {{ entry.name }}
                </el-tag>
              </div>
            </div>
            <div class="flex flex-row flex-wrap items-center gap-2">
              <div class="text-xs">{{ t("game.battleTurnOrder") }}: </div>
              <div v-if="turnOrderEntries.length > 0" class="flex flex-wrap gap-1.5">
                <el-tag
                  v-for="entry in turnOrderEntries"
                  :key="`turn-order-${entry.unitId}`"
                  :disable-transitions="true"
                  :type="entry.isActive && entry.isAlive ? 'warning' : 'info'"
                  :effect="entry.isActive && entry.isAlive ? 'dark' : 'plain'"
                  :class="[
                    'battle-turn-order-tag',
                    hoveredTurnOrderUnitId === entry.unitId ? 'battle-turn-order-tag-hovered' : ''
                  ]"
                  :style="entry.isAlive ? undefined : { opacity: '0.45', filter: 'grayscale(1)' }"
                  @mouseenter="hoveredTurnOrderUnitId = entry.unitId"
                  @mouseleave="hoveredTurnOrderUnitId = ''"
                >
                  {{ entry.name }}
                </el-tag>
              </div>
              <div v-else class="text-xs app-muted">{{ t("game.battleTurnOrderEmpty") }}</div>
            </div>
          </template>
          <el-divider class="my-4!"></el-divider>
          <!-- <div class="mb-2 text-xs app-muted">{{ t("game.battleBattleProcess") }}</div> -->
          <div class="grid gap-3 md:grid-cols-1">
            <div class="battle-side-section">
              <div class="battle-side-label app-muted">{{ t("game.battleBattlePlayers") }}</div>
              <div class="space-y-2 w-full">
                <template v-for="lane in playerLaneOrder" :key="`player-lane-${lane}`">
                  <div v-if="playerLaneSlots[lane].length" class="relative rounded border border-[var(--app-border)] p-2 pt-5">
                    <div class="mb-1 text-[11px] app-muted absolute top-1 left-2">{{ positionLabel(lane) }}</div>
                    <div class="flex flex-wrap flex-row items-center justify-center gap-1.5">
                      <BattleUnitCard
                        v-for="slot in playerLaneSlots[lane]"
                        :key="slot.key"
                        :unit-id="String(slot.unit.unitId || slot.key)"
                        :unit="slot.unit"
                        :resolve-name="resolveUnitName"
                        :is-active="isActiveUnit(slot)"
                        :is-hovered="hoveredTurnOrderUnitId === String(slot.unit.unitId || slot.key)"
                        :immutable-health="isImmutableCombatTrialUnit(slot.unit)"
                      />
                    </div>
                  </div>
                </template>
              </div>
            </div>
            <el-divider class="my-2!"></el-divider>
            <div class="battle-side-section">
              <div class="battle-side-label app-muted">{{ t("game.battleBattleEnemies") }}</div>
              <div class="space-y-2 w-full">
                <template v-for="lane in enemyLaneOrder" :key="`enemy-lane-${lane}`">
                  <div v-if="enemyLaneSlots[lane].length" class="relative rounded border border-[var(--app-border)] p-2 pt-5">
                    <div class="mb-1 text-[11px] app-muted absolute top-1 left-2">{{ positionLabel(lane) }}</div>
                    <div class="flex flex-wrap flex-row items-center justify-center gap-1.5">
                      <BattleUnitCard
                        v-for="slot in enemyLaneSlots[lane]"
                        :key="slot.key"
                      :unit-id="String(slot.unit.unitId || slot.key)"
                      :unit="slot.unit"
                      :resolve-name="resolveUnitName"
                      :is-active="isActiveUnit(slot)"
                      :is-hovered="hoveredTurnOrderUnitId === String(slot.unit.unitId || slot.key)"
                      :immutable-health="isImmutableCombatTrialUnit(slot.unit)"
                    />
                  </div>
                </div>
                </template>
              </div>
            </div>
          </div>

          <svg class="pointer-events-none absolute inset-0 h-full w-full">
            <g v-for="trajectory in battleTrajectories" :key="trajectory.key">
              <line
                :ref="(el) => setLineRef(trajectory.key, el as SVGLineElement | null)"
                :x1="trajectory.x1"
                :y1="trajectory.y1"
                :x2="trajectory.x1"
                :y2="trajectory.y1"
                :stroke="trajectory.color"
                stroke-width="2.4"
                stroke-linecap="round"
                stroke-opacity="0.88"
                :stroke-dasharray="trajectory.dashArray"
              />
            </g>
          </svg>
          <div
            v-for="floatLabel in battleFloatLabels"
            :key="floatLabel.key"
            :ref="(el) => setFloatLabelRef(floatLabel.key, el as HTMLDivElement | null)"
            :class="[
              'pointer-events-none absolute select-none text-sm font-semibold',
              floatLabel.className || ''
            ]"
            :style="{
              left: `${floatLabel.x}px`,
              top: `${floatLabel.y}px`,
              color: floatLabel.color
            }"
          >
            <Icon v-if="floatLabel.icon" :icon="floatLabel.icon" class="mr-1 inline-block align-[-0.12em] text-[1em]" />
            <span>{{ floatLabel.text }}</span>
            <span v-if="floatLabel.suffixText" class="battle-float-label-suffix">{{ floatLabel.suffixText }}</span>
          </div>

          <!-- <div
            v-if="currentActionText"
            class="pointer-events-none absolute left-1/2 top-2 -translate-x-1/2 rounded bg-black/50 px-2 py-1 text-xs text-white"
          >
            {{ currentActionText }}
          </div> -->
        </div>
        <div v-else-if="showBattleLoadingPlaceholder" class="battle-loading-placeholder" role="status" aria-live="polite">
          <span class="battle-loading-spinner" aria-hidden="true"></span>
          <span>{{ t("game.battleBattleLoading") }}</span>
        </div>
        <div v-else-if="showNoRunPlaceholder">
          <el-empty :description="t('game.battleBattleNoRun')" />
        </div>
          </el-tab-pane>
          <el-tab-pane :label="t('game.battleDetailTextTab')" name="text">
            <BattleTextLog
              :room-id="Number(selectedRoom.room.id || 0)"
              :run-id="Number(selectedRun?.id || 0)"
              :events="runEvents"
              :units="battleLogUnits"
              :active="battleDetailTab === 'text'"
            />
          </el-tab-pane>
        </el-tabs>
      </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import gsap from "gsap"
import { getLocalizedSkillName, getSkillDefinition } from "@/services/skill-definition"
import { getLocalizedBuffName } from "@/services/buff-definition"
import { buildBattleSpecializationSourceLabel } from "@/services/battle-specialization-source-display"
import { BATTLE_CHAIN_FLOAT_JITTER_SCALE, INTERVAL_BETWEEN_ITEM_AND_ACTION, INTERVAL_BETWEEN_MULTI_HIT_DAMAGE } from "@/config/shared-global-config"
import BattleUnitCard from "@/components/game/combat/BattleUnitCard.vue"
import BattleTextLog from "@/components/game/combat/BattleTextLog.vue"
import { useCombatViewContext } from "../useCombatViewContext"
import type { BattleViewSource } from "../useCombatViewContext"

type BattleUnit = {
  [key: string]: unknown
  unitId?: string | number
  enemyId?: string
  side?: string
  originSide?: string
  baseSide?: string
  name?: string
  nameKey?: string
  position?: string
  hp?: number
  maxHp?: number
  mp?: number
  maxMp?: number
  alive?: boolean
  corpseConsumed?: boolean
  rewardEligible?: boolean
  attributes?: Record<string, number>
  stats?: Record<string, number>
  skillIds?: string[]
  skillCooldowns?: Record<string, number>
  equipmentInstanceIds?: number[]
  buffs?: Array<{ buffId?: string; remainTurns?: number; payload?: Record<string, unknown>; triggered?: number }>
}
type BattleRunState = {
  seq?: number
  wave?: number
  players?: BattleUnit[]
  enemies?: BattleUnit[]
  order?: unknown[]
  preparationOrder?: unknown[]
  battlefieldEffects?: Array<{
    id?: string
    remaining?: number
    progress?: number
    goal?: number
    windows?: Record<string, number>
  }>
  combatTrialFormat?: {
    encounter?: {
      units?: Array<{ id?: string; healthMode?: string }>
    }
  }
}
type BattleRoomProgress = {
  percentage: number
  remainingLabel: string
}
type InterBattleState =
  | { phase: "preparing" }
  | { phase: "waiting"; percentage: number; remainingLabel: string }
type BattleTrajectory = {
  key: string
  x1: number
  y1: number
  x2: number
  y2: number
  durationMs: number
  delayMs: number
  color: string
  dashArray?: string
}
type BattleFloatLabel = {
  key: string
  x: number
  y: number
  text: string
  suffixText?: string
  icon?: string
  color: string
  className?: string
  isCritical?: boolean
  delayMs: number
  durationMs: number
}
type ReplayActionEvent = {
  seq?: number
  type?: string
  data?: Record<string, unknown>
  replayKey: string
  nextDelayMs?: number
  parentSeq?: number
}
type BattlePoint = { x: number; y: number }
type BattleSlot = { key: string; unit: BattleUnit }
type LaneKey = "vanguard" | "midguard" | "rearguard"

const SHADOW_CROSS_BUFF_ID = "basic_left_dagger_shadow_cross"

const props = withDefaults(defineProps<{
  source?: BattleViewSource
  embedded?: boolean
  animateEvents?: boolean
  playbackRate?: number
}>(), {
  embedded: false,
  animateEvents: true,
  playbackRate: 1
})

const { t } = useI18n()
const battleDetailTab = ref<"animation" | "text">("animation")
const {
  worlds,
  selectedRoom,
  selectedRun,
  runState,
  runEvents,
  roomsLoading,
  runLoading,
  roomStatusLabel,
  runStatusLabel,
  positionLabel,
  selectRoom,
  interBattleWait
} = props.source || useCombatViewContext()
const embedded = computed(() => props.embedded)
const playbackRate = computed(() => Math.max(0.5, Math.min(2, Number(props.playbackRate) || 1)))

const battleCanvasRef = ref<HTMLElement | null>(null)
const lineRefs = new Map<string, SVGLineElement>()
const floatLabelRefs = new Map<string, HTMLDivElement>()
const animationHandles: gsap.core.Tween[] = []
const moveAnimationHandles: gsap.core.Tween[] = []
const pendingMovementUnitIDsBySeq = new Map<number, Set<string>>()
let lastRenderedPosMap = new Map<string, BattlePoint>()
const roomStatus = computed(() => String(selectedRoom.value?.room.status || ""))
const shouldRenderBattleCanvas = computed(() => roomStatus.value === "playing" && Boolean(renderRunState.value))
const isBattleStateSyncing = computed(() => roomStatus.value === "playing" && !renderRunState.value)
const showInitialBattleLoading = computed(() => !selectedRoom.value && roomsLoading.value)
const showBattleLoadingPlaceholder = computed(() =>
  !shouldRenderBattleCanvas.value && (roomsLoading.value || runLoading.value || isBattleStateSyncing.value)
)
const transientVisualUnitIDs = ref<Set<string>>(new Set())
const showNoRunPlaceholder = computed(() => {
  if (roomStatus.value === "entering" || roomStatus.value === "recovering") return false
  if (shouldRenderBattleCanvas.value || showBattleLoadingPlaceholder.value) return false
  return true
})
const battleAllUnits = computed<BattleUnit[]>(() => {
  const units = [...(renderRunState.value?.players || []), ...(renderRunState.value?.enemies || [])]
  const dedup = new Map<string, BattleUnit>()
  for (const unit of units) {
    const unitID = String(unit.unitId || "").trim()
    if (!unitID) continue
    if (unit.corpseConsumed === true && !transientVisualUnitIDs.value.has(unitID)) continue
    if (!dedup.has(unitID)) dedup.set(unitID, unit)
  }
  return [...dedup.values()]
})
const battleLogUnits = computed<BattleUnit[]>(() => [
  ...(renderRunState.value?.players || []),
  ...(renderRunState.value?.enemies || [])
])
const battlePlayers = computed<BattleUnit[]>(() => battleAllUnits.value.filter((unit) => resolveDisplaySide(unit) === "player"))
const battleEnemies = computed<BattleUnit[]>(() => battleAllUnits.value.filter((unit) => resolveDisplaySide(unit) === "enemy"))
const nowMs = ref(Date.now())
const currentWave = computed(() => Number(renderRunState.value?.wave || selectedRun.value?.wave || 1))
const battlefieldTriggerUntilMs = ref(0)
const battlefieldTriggerSummary = ref("")
const battlefieldTriggerActive = computed(() => nowMs.value < battlefieldTriggerUntilMs.value)
const battlefieldEffectRows = computed(() => {
  const worldID = String(selectedRoom.value?.room.dungeonId || "").trim()
  const world = worlds.value.find((item) => item.id === worldID)
  const definitions = new Map((world?.battlefieldEffects || []).map((effect) => [effect.id, effect]))
  return (renderRunState.value?.battlefieldEffects || []).flatMap((state) => {
    const id = String(state.id || "").trim()
    if (!id) return []
    const definition = definitions.get(id)
    const nameKey = String(definition?.nameKey || "").trim()
    const translated = nameKey ? t(nameKey) : ""
    return [{
      id,
      name: translated && translated !== nameKey ? translated : id,
      icon: String(definition?.icon || "game-icons:falling-star"),
      displayMode: definition?.displayMode === "progress" ? "progress" as const : "countdown" as const,
      remaining: Math.max(0, Math.trunc(Number(state.remaining) || 0)),
      progress: Math.max(0, Math.trunc(Number(state.progress) || 0)),
      goal: Math.max(1, Math.trunc(Number(state.goal) || 1)),
      exposedRemaining: Math.max(0, Math.trunc(Number(state.windows?.timekeeper_exposed) || 0))
    }]
  })
})
const recoveryEndsAtLabel = computed(() => {
  const raw = selectedRoom.value?.room.recoveryEndsAt
  if (!raw) return "-"
  const value = new Date(raw)
  if (Number.isNaN(value.getTime())) return "-"
  return value.toLocaleTimeString()
})
const enteringProgress = computed<BattleRoomProgress | null>(() => {
  const room = selectedRoom.value?.room
  if (!room || room.status !== "entering") return null
  const world = worlds.value.find((item) => item.id === room.dungeonId)
  const durationMs = Math.max(0, Math.trunc(Number(world?.entryDurationSeconds || 0) * 1000))
  const startedAtMs = parseTimestampMs(room.enteringStartedAt)
  if (durationMs <= 0 || startedAtMs <= 0) return null
  const elapsedMs = Math.max(0, nowMs.value - startedAtMs)
  const cappedElapsedMs = Math.min(durationMs, elapsedMs)
  return {
    percentage: toProgressPercent(cappedElapsedMs, durationMs),
    remainingLabel: formatDurationLabel(Math.max(0, durationMs - elapsedMs))
  }
})
const recoveringProgress = computed<BattleRoomProgress | null>(() => {
  const room = selectedRoom.value?.room
  if (!room || room.status !== "recovering") return null
  const startedAtMs = parseTimestampMs(room.recoveringStartedAt)
  const endsAtMs = parseTimestampMs(room.recoveryEndsAt)
  if (endsAtMs <= 0) return null
  const remainingMs = Math.max(0, endsAtMs - nowMs.value)
  const durationMs = startedAtMs > 0 && endsAtMs > startedAtMs ? (endsAtMs - startedAtMs) : 0
  const elapsedMs = durationMs > 0 ? Math.max(0, nowMs.value - startedAtMs) : 0
  return {
    percentage: durationMs > 0 ? toProgressPercent(Math.min(durationMs, elapsedMs), durationMs) : (remainingMs <= 0 ? 100 : 0),
    remainingLabel: formatDurationLabel(remainingMs)
  }
})
const interBattleState = computed<InterBattleState | null>(() => {
  const wait = interBattleWait.value
  const runIsPreparing = selectedRun.value?.status === "compiled"
  if ((!wait && !runIsPreparing) || roomStatus.value !== "playing") return null
  if (!wait) return { phase: "preparing" }
  const elapsedMs = Math.max(0, nowMs.value - wait.finishedAtMs)
  const remainingMs = Math.max(0, wait.intervalMs - elapsedMs)
  if (wait.pendingRunId > 0 || remainingMs <= 0 || runIsPreparing) {
    return { phase: "preparing" }
  }
  return {
    phase: "waiting",
    percentage: toProgressPercent(Math.min(wait.intervalMs, elapsedMs), wait.intervalMs),
    remainingLabel: formatDurationLabel(remainingMs)
  }
})
const playerLaneOrder: LaneKey[] = ["rearguard", "midguard", "vanguard"]
const enemyLaneOrder: LaneKey[] = ["vanguard", "midguard", "rearguard"]
const unitStableSeqByID = new Map<string, number>()
const laneEntrySeqByKey = new Map<string, number>()
let nextUnitStableSeq = 1
let nextLaneEntrySeq = 1
const unitNameByID = computed(() => {
  const map = new Map<string, string>()
  for (const unit of [...battlePlayers.value, ...battleEnemies.value]) {
    const unitID = String(unit.unitId || "").trim()
    if (!unitID) continue
    map.set(unitID, resolveUnitName(unit as Record<string, unknown>))
  }
  return map
})
const unitAliveByID = computed(() => {
  const map = new Map<string, boolean>()
  for (const unit of [...battlePlayers.value, ...battleEnemies.value]) {
    const unitID = String(unit.unitId || "").trim()
    if (!unitID) continue
    map.set(unitID, unit.alive !== false)
  }
  return map
})

const replayQueue = ref<ReplayActionEvent[]>([])
const currentAction = ref<ReplayActionEvent | null>(null)
const currentBatchActions = ref<ReplayActionEvent[]>([])
const battleTrajectories = ref<BattleTrajectory[]>([])
const battleFloatLabels = ref<BattleFloatLabel[]>([])
const hoveredTurnOrderUnitId = ref("")
const replayRunID = ref(0)
const replayEventSerial = ref(0)
const lastQueuedRunEventIndex = ref(-1)
const pendingReplayKey = ref("")
const pendingReplayAttempts = ref(0)
let replayQueueTimer: ReturnType<typeof setTimeout> | null = null
let progressTicker: ReturnType<typeof setInterval> | null = null
const displayedRunState = ref<BattleRunState | null>(null)
const hasPendingDisplayStateSync = ref(false)
const renderRunState = computed<BattleRunState | null>(() => displayedRunState.value || (runState.value as BattleRunState | null))
const immutableCombatTrialUnitIDs = computed(() => {
  const state = renderRunState.value
  if (!state?.combatTrialFormat) return new Set<string>()
  const ids = (state?.combatTrialFormat?.encounter?.units || [])
    .filter((unit) => String(unit.healthMode || "").trim() === "immutable_full")
    .map((unit) => String(unit.id || "").trim())
    .filter(Boolean)
    .map((id) => `combat_trial:opponent:${id}`)
  return new Set(ids)
})

function isImmutableCombatTrialUnit(unit: BattleUnit): boolean {
  return immutableCombatTrialUnitIDs.value.has(String(unit.unitId || "").trim())
}

function isReplayPacingInProgress() {
  return replayQueueTimer !== null || replayQueue.value.length > 0
}

function syncDisplayedRunState() {
  displayedRunState.value = (runState.value as BattleRunState | null) || null
  hasPendingDisplayStateSync.value = false
}

function reconcileDisplayedRunState() {
  if (isReplayPacingInProgress()) {
    hasPendingDisplayStateSync.value = true
    return
  }
  syncDisplayedRunState()
}

const currentActionText = computed(() => {
  const action = currentAction.value
  if (!action) return ""
  const actionType = String(action.type || "").trim()
  if (actionType === "battle.timeline.node") {
    const owner = (action.data?.turnOwner || {}) as Record<string, unknown>
    const actor = String(owner.unitId || "")
    const steps = Array.isArray(action.data?.steps) ? action.data.steps : []
    const castStep = steps.find((step) => String((step as Record<string, unknown>)?.kind || "").trim() === "cast") as Record<string, unknown> | undefined
    const skill = String(((castStep?.skill as Record<string, unknown> | undefined)?.skillName) || ((castStep?.skill as Record<string, unknown> | undefined)?.skillId) || "")
    return `${actor} -> ${skill}`
  }
  const actor = String(action.data?.actorId || "")
  const skill = String(action.data?.skillName || action.data?.skillId || "")
  return `${actor} -> ${skill}`
})

const activeActorID = computed(() => {
  const action = currentAction.value
  if (!action) return ""
  if (String(action.type || "").trim() === "battle.timeline.node") {
    return String((action.data?.turnOwner as Record<string, unknown> | undefined)?.unitId || "").trim()
  }
  return String(action.data?.actorId || "").trim()
})

function clearReplayTimer() {
  if (replayQueueTimer === null) return
  clearTimeout(replayQueueTimer)
  replayQueueTimer = null
}

function resetReplayQueue() {
  clearReplayTimer()
  replayQueue.value = []
  currentAction.value = null
  currentBatchActions.value = []
  transientVisualUnitIDs.value = new Set()
  battleTrajectories.value = []
  battleFloatLabels.value = []
  lastQueuedRunEventIndex.value = -1
  pendingReplayKey.value = ""
  pendingReplayAttempts.value = 0
  pendingMovementUnitIDsBySeq.clear()
  clearMoveAnimations()
  syncDisplayedRunState()
}

function isReplayActionEventType(type: string) {
  return type === "battle.timeline.node" || type === "battle.turn.action" || type === "battle.turn.item_use"
}

function replayDurationMs(action: ReplayActionEvent) {
  const serverDelayMs = Number(action.nextDelayMs || 0)
  if (String(action.type || "").trim() !== "battle.timeline.node") return 560
  const steps = Array.isArray(action.data?.steps) ? action.data.steps : []
  let cumulativeDelay = 0
  let maxVisualEnd = 0
  let previousStep: Record<string, unknown> | null = null
  for (const rawStep of steps) {
    const step = rawStep as Record<string, unknown>
    cumulativeDelay += resolveReplayStepDelay(step, previousStep)
    previousStep = step
    const kind = String(step.kind || "").trim().toLowerCase()
    if (!["damage", "heal", "resource", "buff", "buff_charge", "state_detach", "corpse_focus"].includes(kind)) continue
    const targets = Array.isArray(step.targets) ? step.targets : []
    if (targets.length <= 0) continue
    maxVisualEnd = Math.max(maxVisualEnd, cumulativeDelay + 620)
  }
  if (maxVisualEnd <= 0) {
    maxVisualEnd = steps.length > 0 ? cumulativeDelay + 520 : 560
  }
  const visualDuration = Math.max(560, Math.min(4200, maxVisualEnd + 120))
  if (serverDelayMs > 0) {
    return Math.max(560, Math.min(visualDuration, serverDelayMs))
  }
  return visualDuration
}

function replayBatchDurationMs(actions: ReplayActionEvent[]) {
  if (!Array.isArray(actions) || actions.length <= 0) return 560
  return Math.max(...actions.map((action) => replayDurationMs(action)))
}

function collectReplayUnitIDs(action: ReplayActionEvent): string[] {
  const ids = new Set<string>()
  if (!action.data) return []
  const actionType = String(action.type || "").trim()
  if (actionType === "battle.timeline.node") {
    const ownerID = normalizeReplayUnitID((action.data.turnOwner as Record<string, unknown> | undefined)?.unitId)
    if (ownerID) ids.add(ownerID)
    const steps = Array.isArray(action.data.steps) ? action.data.steps : []
    for (const rawStep of steps) {
      const step = rawStep as Record<string, unknown>
      const source = (step.source || {}) as Record<string, unknown>
      const sourceID = normalizeReplayUnitID(source.unitId)
      if (sourceID) ids.add(sourceID)
      const targets = Array.isArray(step.targets) ? step.targets : []
      for (const rawTarget of targets) {
        const target = rawTarget as Record<string, unknown>
        const targetID = normalizeReplayUnitID(target.unitId || target.targetId)
        if (targetID) ids.add(targetID)
      }
    }
    return [...ids]
  }
  const actorID = normalizeReplayUnitID(action.data.actorId)
  if (actorID) ids.add(actorID)
  const targets = Array.isArray(action.data.targets) ? action.data.targets : []
  for (const rawTarget of targets) {
    const target = rawTarget as Record<string, unknown>
    const targetID = normalizeReplayUnitID(target.targetId || target.unitId)
    if (targetID) ids.add(targetID)
  }
  return [...ids]
}

function collectReplayVisualUnitIDs(actions: ReplayActionEvent[]): Set<string> {
  const ids = new Set<string>()
  for (const action of actions) {
    for (const unitID of collectReplayUnitIDs(action)) {
      ids.add(unitID)
    }
  }
  return ids
}

function normalizeReplayUnitID(raw: unknown): string {
  const unitID = String(raw || "").trim()
  if (!unitID) return ""
  const lower = unitID.toLowerCase()
  if (lower === "<nil>" || lower === "nil" || lower === "null") return ""
  return unitID
}

function canRenderReplayAction(action: ReplayActionEvent) {
  const posMap = collectUnitPositionMap()
  if (!posMap || posMap.size <= 0) return false
  const requiredIDs = collectReplayUnitIDs(action)
  if (requiredIDs.length <= 0) return true
  return requiredIDs.every((unitID) => posMap.has(unitID))
}

function pumpReplayQueue() {
  if (replayQueueTimer !== null || replayQueue.value.length <= 0) return
  const nextAction = replayQueue.value[0]
  if (!nextAction) return
  transientVisualUnitIDs.value = collectReplayVisualUnitIDs([nextAction])
  if (!canRenderReplayAction(nextAction)) {
    if (pendingReplayKey.value === nextAction.replayKey) {
      pendingReplayAttempts.value += 1
    } else {
      pendingReplayKey.value = nextAction.replayKey
      pendingReplayAttempts.value = 1
    }
    if (pendingReplayAttempts.value <= 8) {
      replayQueueTimer = setTimeout(() => {
        replayQueueTimer = null
        pumpReplayQueue()
      }, 48)
      return
    }
  }
  replayQueue.value.shift()
  pendingReplayKey.value = ""
  pendingReplayAttempts.value = 0
  // Collect synced summon actions that share parentSeq with the main action.
  // They play simultaneously: all trajectories rendered in one batch.
  const mainSeq = nextAction.seq
  const batch: ReplayActionEvent[] = [nextAction]
  while (replayQueue.value.length > 0) {
    const child = replayQueue.value[0]
    if (!child || child.parentSeq !== mainSeq) break
    batch.push(replayQueue.value.shift()!)
  }
  transientVisualUnitIDs.value = collectReplayVisualUnitIDs(batch)
  if (hasPendingDisplayStateSync.value) {
    syncDisplayedRunState()
  }
  currentAction.value = nextAction
  currentBatchActions.value = batch
  replayQueueTimer = setTimeout(() => {
    replayQueueTimer = null
    transientVisualUnitIDs.value = new Set()
    if (hasPendingDisplayStateSync.value) {
      syncDisplayedRunState()
    }
    pumpReplayQueue()
  }, replayBatchDurationMs(batch) / playbackRate.value)
}

function normalizeBattleSide(raw: unknown): "player" | "enemy" | "" {
  const side = String(raw || "").trim().toLowerCase()
  if (side === "player" || side === "enemy") return side
  return ""
}

function normalizeLaneKey(raw: unknown): LaneKey | "" {
  const lane = String(raw || "").trim().toLowerCase()
  if (lane === "vanguard" || lane === "midguard" || lane === "rearguard") return lane
  return ""
}

function ensureUnitStableSeq(unitID: string): number {
  const normalized = String(unitID || "").trim()
  if (!normalized) return Number.MAX_SAFE_INTEGER
  const existing = unitStableSeqByID.get(normalized)
  if (typeof existing === "number") return existing
  const assigned = nextUnitStableSeq++
  unitStableSeqByID.set(normalized, assigned)
  return assigned
}

function ensureLaneEntrySeq(unitID: string, lane: LaneKey): number {
  const normalized = String(unitID || "").trim()
  if (!normalized) return Number.MAX_SAFE_INTEGER
  const key = `${normalized}:${lane}`
  const existing = laneEntrySeqByKey.get(key)
  if (typeof existing === "number") return existing
  const assigned = nextLaneEntrySeq++
  laneEntrySeqByKey.set(key, assigned)
  return assigned
}

function resolveDisplaySide(unit: BattleUnit): "player" | "enemy" {
  const explicit = normalizeBattleSide(unit.side)
  if (explicit) return explicit
  const origin = normalizeBattleSide(unit.originSide)
  if (origin) return origin
  const base = normalizeBattleSide(unit.baseSide)
  if (base) return base
  return "player"
}

function collectMoveUnitIDs(action: ReplayActionEvent): Set<string> {
  const ids = new Set<string>()
  if (String(action.type || "").trim() !== "battle.timeline.node" || !action.data) return ids
  const steps = Array.isArray(action.data.steps) ? action.data.steps : []
  for (const rawStep of steps) {
    const step = rawStep as Record<string, unknown>
    const kind = String(step.kind || "").trim().toLowerCase()
    const buffID = String(step.buffId || "").trim()
    const targets = Array.isArray(step.targets) ? step.targets : []
    if (kind === "position_move") {
      for (const rawTarget of targets) {
        const target = rawTarget as Record<string, unknown>
        const targetID = String(target.unitId || target.targetId || "").trim()
        if (targetID) ids.add(targetID)
      }
      continue
    }
    if (kind === "stat_recalc") {
      for (const rawTarget of targets) {
        const target = rawTarget as Record<string, unknown>
        const targetID = String(target.unitId || target.targetId || "").trim()
        if (!targetID) continue
        const fromPosition = String(target.fromPosition || "").trim().toLowerCase()
        const toPosition = String(target.toPosition || target.position || "").trim().toLowerCase()
        if (fromPosition && toPosition && fromPosition !== toPosition) {
          ids.add(targetID)
        }
      }
      continue
    }
    if ((kind === "buff" || kind === "state_detach") && buffID === SHADOW_CROSS_BUFF_ID) {
      for (const rawTarget of targets) {
        const target = rawTarget as Record<string, unknown>
        const targetID = String(target.unitId || target.targetId || "").trim()
        if (targetID) ids.add(targetID)
      }
    }
  }
  return ids
}

function rememberPendingMoveIDs(seq: number, moveIDs: Set<string>) {
  if (!Number.isFinite(seq) || seq <= 0 || moveIDs.size <= 0) return
  const existing = pendingMovementUnitIDsBySeq.get(seq) || new Set<string>()
  for (const unitID of moveIDs) {
    existing.add(unitID)
  }
  pendingMovementUnitIDsBySeq.set(seq, existing)
}

watch(
  () => Number(selectedRun.value?.id || 0),
  (nextRunID) => {
    if (replayRunID.value === nextRunID) return
    replayRunID.value = nextRunID
    unitStableSeqByID.clear()
    laneEntrySeqByKey.clear()
    nextUnitStableSeq = 1
    nextLaneEntrySeq = 1
    resetReplayQueue()
  },
  { immediate: true }
)

watch(
  runEvents,
  (nextEvents) => {
    if (!Array.isArray(nextEvents)) return
    if (nextEvents.length <= 0) {
      resetReplayQueue()
      return
    }
    if (!props.animateEvents) {
      resetReplayQueue()
      lastQueuedRunEventIndex.value = nextEvents.length - 1
      syncDisplayedRunState()
      return
    }
    if (nextEvents.length - 1 < lastQueuedRunEventIndex.value) {
      resetReplayQueue()
    }
    for (let idx = lastQueuedRunEventIndex.value + 1; idx < nextEvents.length; idx += 1) {
      const event = nextEvents[idx]
      const eventType = String(event.type || "").trim()
      const eventData = (event.data || {}) as Record<string, unknown>
      if (eventType === "battle.field.trigger") {
        const effectID = String(eventData.effectId || eventData.id || "").trim()
        const targets = Array.isArray(eventData.targets) ? eventData.targets : []
        battlefieldTriggerSummary.value = targets.flatMap((target) => {
          if (!target || typeof target !== "object" || Array.isArray(target)) return []
          const row = target as Record<string, unknown>
          const unitID = String(row.unitId || row.targetId || "").trim()
          const unit = battleAllUnits.value.find((entry) => String(entry.unitId || "").trim() === unitID)
          const unitName = unit ? resolveUnitName(unit as Record<string, unknown>) : unitID
          const outcomeID = String(row.outcomeId || "").trim()
          const outcomeKey = effectID && outcomeID ? `battlefield_effect.${effectID}.outcomes.${outcomeID}` : ""
          const translatedOutcome = outcomeKey ? t(outcomeKey) : ""
          const outcome = translatedOutcome && translatedOutcome !== outcomeKey ? translatedOutcome : outcomeID
          if (!unitName || !outcome) return []
          return [`${unitName} · ${outcome}`]
        }).join("  ")
        battlefieldTriggerUntilMs.value = Date.now() + 1600
      }
      if (!isReplayActionEventType(eventType)) continue
      let nextDelayMs = 0
      for (let nextIdx = idx + 1; nextIdx < nextEvents.length; nextIdx += 1) {
        const nextType = String(nextEvents[nextIdx]?.type || "").trim()
        if (!isReplayActionEventType(nextType)) continue
        const currentTs = Number(event.ts || 0)
        const nextTs = Number(nextEvents[nextIdx]?.ts || 0)
        if (Number.isFinite(currentTs) && Number.isFinite(nextTs) && nextTs > currentTs) {
          nextDelayMs = nextTs - currentTs
        }
        if (
          nextDelayMs <= 0 &&
          eventType === "battle.turn.item_use" &&
          nextType === "battle.turn.action"
        ) {
          nextDelayMs = INTERVAL_BETWEEN_ITEM_AND_ACTION
        }
        break
      }
      replayQueue.value.push({
        seq: Number(event.seq || 0),
        type: eventType,
        data: (event.data || {}) as Record<string, unknown>,
        replayKey: `${replayRunID.value}-${Number(event.seq || 0)}-${idx}-${replayEventSerial.value}`,
        nextDelayMs,
        parentSeq: eventData.parentSeq != null ? Number(eventData.parentSeq) : undefined
      })
      const queuedAction = replayQueue.value[replayQueue.value.length - 1]
      if (queuedAction) {
        rememberPendingMoveIDs(Number(queuedAction.seq || 0), collectMoveUnitIDs(queuedAction))
      }
      replayEventSerial.value += 1
    }
    lastQueuedRunEventIndex.value = nextEvents.length - 1
    pumpReplayQueue()
  },
  { immediate: true, flush: "post" }
)

watch(
  () => props.animateEvents,
  (enabled) => {
    if (enabled) return
    resetReplayQueue()
    lastQueuedRunEventIndex.value = Math.max(-1, runEvents.value.length - 1)
  },
  { immediate: true, flush: "post" }
)

watch(
  runState,
  () => reconcileDisplayedRunState(),
  { immediate: true, deep: false }
)

watch(
  () => Number(renderRunState.value?.seq || 0),
  async (nextSeq) => {
    await nextTick()
    const latestPosMap = collectUnitPositionMap() || new Map<string, BattlePoint>()
    if (lastRenderedPosMap.size <= 0) {
      lastRenderedPosMap = latestPosMap
      return
    }
    const pendingMoveIDs = pendingMovementUnitIDsBySeq.get(nextSeq)
    if (!pendingMoveIDs || pendingMoveIDs.size <= 0) {
      lastRenderedPosMap = latestPosMap
      return
    }
    for (const unitID of pendingMoveIDs) {
      const from = lastRenderedPosMap.get(unitID)
      const to = latestPosMap.get(unitID)
      if (!from || !to) continue
      animateUnitMoveByID(unitID, from, to)
    }
    pendingMovementUnitIDsBySeq.delete(nextSeq)
    for (const key of [...pendingMovementUnitIDsBySeq.keys()]) {
      if (key < nextSeq - 2) pendingMovementUnitIDsBySeq.delete(key)
    }
    lastRenderedPosMap = latestPosMap
  },
  { immediate: true, flush: "post" }
)

const turnOrderEntries = computed(() => {
  const allUnits = [...battlePlayers.value, ...battleEnemies.value]
  const unitByID = new Map<string, BattleUnit>()
  for (const unit of allUnits) {
    const unitID = String(unit.unitId || "").trim()
    if (!unitID) continue
    unitByID.set(unitID, unit)
  }
  const seen = new Set<string>()
  const ordered: Array<{ unitId: string; name: string; isActive: boolean; isAlive: boolean }> = []
  const orderFromState = Array.isArray(renderRunState.value?.order) ? renderRunState.value?.order || [] : []
  for (const rawID of orderFromState) {
    const unitID = String(rawID || "").trim()
    if (!unitID || seen.has(unitID)) continue
    if (!unitByID.has(unitID)) continue
    seen.add(unitID)
    const unit = unitByID.get(unitID)
    if (!unit) continue
    ordered.push({
      unitId: unitID,
      name: resolveUnitName(unit as Record<string, unknown>),
      isActive: unitID === activeActorID.value,
      isAlive: unit.alive !== false
    })
  }
  return ordered
})

const preparationTurnOrderEntries = computed(() => {
  const order = Array.isArray(renderRunState.value?.preparationOrder)
    ? renderRunState.value?.preparationOrder || []
    : []
  if (order.length <= 0) return []
  const rows: Array<{ unitId: string; name: string; isAlive: boolean }> = []
  const seen = new Set<string>()
  for (const rawID of order) {
    const unitID = String(rawID || "").trim()
    if (!unitID || seen.has(unitID)) continue
    seen.add(unitID)
    rows.push({
      unitId: unitID,
      name: unitNameByID.value.get(unitID) || unitID,
      isAlive: unitAliveByID.value.get(unitID) !== false
    })
  }
  return rows
})

watch(turnOrderEntries, (entries) => {
  if (!hoveredTurnOrderUnitId.value) return
  const inTurnOrder = entries.some((entry) => entry.unitId === hoveredTurnOrderUnitId.value)
  const inPreparationOrder = preparationTurnOrderEntries.value.some((entry) => entry.unitId === hoveredTurnOrderUnitId.value)
  if (!inTurnOrder && !inPreparationOrder) {
    hoveredTurnOrderUnitId.value = ""
  }
})

function laneSlots(units: BattleUnit[], order: LaneKey[], side: "player" | "enemy"): Record<LaneKey, BattleSlot[]> {
  const byLane: Record<LaneKey, BattleSlot[]> = { vanguard: [], midguard: [], rearguard: [] }
  order.forEach((lane) => {
    const laneUnits = units
      .filter((unit) => normalizeLaneKey(unit.position) === lane)
      .sort((left, right) => {
        const leftID = String(left.unitId || "").trim()
        const rightID = String(right.unitId || "").trim()
        const leftLaneSeq = ensureLaneEntrySeq(leftID, lane)
        const rightLaneSeq = ensureLaneEntrySeq(rightID, lane)
        if (leftLaneSeq !== rightLaneSeq) return leftLaneSeq - rightLaneSeq
        const leftStableSeq = ensureUnitStableSeq(leftID)
        const rightStableSeq = ensureUnitStableSeq(rightID)
        if (leftStableSeq !== rightStableSeq) return leftStableSeq - rightStableSeq
        return leftID.localeCompare(rightID)
      })
    laneUnits.forEach((unit, localIdx) => {
      byLane[lane].push({
        key: `${String(unit.unitId || `${side}-${lane}-${localIdx}`)}`,
        unit
      })
    })
  })
  return byLane
}

const playerLaneSlots = computed(() => laneSlots(battlePlayers.value, playerLaneOrder, "player"))
const enemyLaneSlots = computed(() => laneSlots(battleEnemies.value, enemyLaneOrder, "enemy"))

function collectUnitPositionMap() {
  const container = battleCanvasRef.value
  if (!container) return null
  const containerRect = container.getBoundingClientRect()
  const posMap = new Map<string, { x: number; y: number }>()
  const unitNodes = container.querySelectorAll<HTMLElement>("[data-battle-unit-id]")
  for (const node of unitNodes) {
    const unitID = String(node.dataset.battleUnitId || "").trim()
    if (!unitID) continue
    const rect = node.getBoundingClientRect()
    posMap.set(unitID, {
      x: rect.left - containerRect.left + rect.width / 2,
      y: rect.top - containerRect.top + rect.height / 2
    })
  }
  return posMap
}

function animateUnitMoveByID(unitID: string, from: BattlePoint, to: BattlePoint) {
  const deltaX = from.x - to.x
  const deltaY = from.y - to.y
  if (Math.abs(deltaX) < 0.5 && Math.abs(deltaY) < 0.5) return
  // 用户要求取消位移过渡：保持即时落位，不做 tween。
}

function clearMoveAnimations() {
  for (const handle of moveAnimationHandles.splice(0)) {
    handle.kill()
  }
  const container = battleCanvasRef.value
  if (!container) return
  const unitNodes = container.querySelectorAll<HTMLElement>("[data-battle-unit-id]")
  for (const node of unitNodes) {
    gsap.killTweensOf(node)
    gsap.set(node, { clearProps: "transform,zIndex,willChange" })
  }
}

function buildTrajectories(action: ReplayActionEvent, posMap: Map<string, { x: number; y: number }>): BattleTrajectory[] {
  if (!action.data) return []
  const actionType = String(action.type || "").trim()
  if (actionType === "battle.timeline.node") {
    const steps = Array.isArray(action.data.steps) ? action.data.steps : []
    const ownerID = String((action.data.turnOwner as Record<string, unknown> | undefined)?.unitId || "").trim()
    const lines: BattleTrajectory[] = []
    let cumulativeDelay = 0
    let previousStep: Record<string, unknown> | null = null
    for (const [stepIdx, rawStep] of steps.entries()) {
      const step = rawStep as Record<string, unknown>
      const kind = String(step.kind || "").trim().toLowerCase()
      cumulativeDelay += resolveReplayStepDelay(step, previousStep)
      previousStep = step
      if (!["damage", "heal", "resource", "buff", "corpse_focus"].includes(kind)) continue
      const source = (step.source || {}) as Record<string, unknown>
      const sourceID = String(source.unitId || ownerID).trim()
      const from = posMap.get(sourceID)
      if (!from) continue
      const display = (step.display || {}) as Record<string, unknown>
      const lineColor = String(display.lineColor || defaultLineColor(kind))
      const lineStyle = String(display.lineStyle || "solid")
      const dashArray = lineStyle === "dashed" ? "6 4" : undefined
      const targets = Array.isArray(step.targets) ? step.targets : []
      for (const [targetIdx, rawTarget] of targets.entries()) {
        const target = rawTarget as Record<string, unknown>
        const targetID = String(target.unitId || target.targetId || "").trim()
        if (!targetID) continue
        const to = posMap.get(targetID)
        if (!to) continue
        lines.push({
          key: `${action.replayKey}-line-${stepIdx}-${targetIdx}`,
          x1: from.x,
          y1: from.y,
          x2: to.x,
          y2: to.y,
          durationMs: 520,
          delayMs: cumulativeDelay,
          color: lineColor,
          dashArray
        })
      }
    }
    return lines
  }

  const actorID = String(action.data.actorId || "")
  const targets = Array.isArray(action.data.targets) ? action.data.targets : []
  if (!actorID || targets.length === 0) return []
  const from = posMap.get(actorID)
  if (!from) return []
  const lines: BattleTrajectory[] = []
  for (const [idx, target] of targets.entries()) {
    const targetID = String(target.targetId || "")
    const to = posMap.get(targetID)
    if (!to) continue
    lines.push({
      key: `${action.replayKey}-line-${idx}`,
      x1: from.x,
      y1: from.y,
      x2: to.x,
      y2: to.y,
      durationMs: 560,
      delayMs: 0,
      color: "#ef4444"
    })
  }
  return lines
}

function buildFloatLabels(action: ReplayActionEvent, posMap: Map<string, { x: number; y: number }>): BattleFloatLabel[] {
  if (!action.data || String(action.type || "").trim() !== "battle.timeline.node") return []
  const steps = Array.isArray(action.data.steps) ? action.data.steps : []
  const labels: BattleFloatLabel[] = []
  const ownerID = String((action.data.turnOwner as Record<string, unknown> | undefined)?.unitId || "").trim()
  let cumulativeDelay = 0
  let previousStep: Record<string, unknown> | null = null
  const buffLabelCountByTarget = new Map<string, number>()
  for (const [stepIdx, rawStep] of steps.entries()) {
    const step = rawStep as Record<string, unknown>
    const kind = String(step.kind || "").trim().toLowerCase()
    cumulativeDelay += resolveReplayStepDelay(step, previousStep)
    previousStep = step
    if (kind === "cast") {
      const source = (step.source || {}) as Record<string, unknown>
      const sourceID = String(source.unitId || ownerID).trim()
      if (!sourceID) continue
      const pos = posMap.get(sourceID)
      if (!pos) continue
      const skillLabel = resolveSkillFloatLabel(step)
      if (!skillLabel.text) continue
      const display = (step.display || {}) as Record<string, unknown>
      const phase = String(step.phase || "").trim().toLowerCase()
      const isItemCast = phase === "item"
      labels.push({
        key: `${action.replayKey}-skill-${stepIdx}`,
        x: pos.x,
        y: pos.y - (isItemCast ? 52 : 36),
        text: skillLabel.text,
        suffixText: resolveSpecializationSourceLabel(step) || undefined,
        icon: skillLabel.icon || undefined,
        color: String(display.skillTextColor || "var(--battle-skill-float-text-color)"),
        className: isItemCast ? "battle-skill-float-label battle-item-float-label" : "battle-skill-float-label",
        delayMs: cumulativeDelay,
        durationMs: 840
      })
      continue
    }

    if (kind === "buff" || kind === "buff_charge" || kind === "state_detach") {
      const targets = Array.isArray(step.targets) ? step.targets : []
      const buffLabel = resolveBuffFloatLabel(step)
      if (!buffLabel) continue
      for (const [targetIdx, rawTarget] of targets.entries()) {
        const target = rawTarget as Record<string, unknown>
        const targetID = String(target.unitId || target.targetId || "").trim()
        if (!targetID) continue
        const pos = posMap.get(targetID)
        if (!pos) continue
        const isGain = kind === "buff"
        const buffIdx = buffLabelCountByTarget.get(targetID) ?? 0
        buffLabelCountByTarget.set(targetID, buffIdx + 1)
        const jitterX = buffIdx % 2 === 0 ? 0 : (buffIdx % 4 < 2 ? 10 : -10)
        labels.push({
          key: `${action.replayKey}-status-${stepIdx}-${targetIdx}`,
          x: pos.x - 56 + jitterX,
          y: pos.y - 28 - buffIdx * 20,
          text: `${isGain ? "+" : "-"}${buffLabel}`,
          suffixText: resolveSpecializationSourceLabel(step, target) || undefined,
          color: "var(--battle-status-float-color)",
          className: isGain ? "battle-status-float-label battle-status-float-label--gain" : "battle-status-float-label battle-status-float-label--loss",
          delayMs: cumulativeDelay + buffIdx * 60,
          durationMs: 700
        })
      }
      continue
    }
    if (kind !== "damage" && kind !== "heal") continue
    const display = (step.display || {}) as Record<string, unknown>
    const targets = Array.isArray(step.targets) ? step.targets : []
    for (const [targetIdx, rawTarget] of targets.entries()) {
      const target = rawTarget as Record<string, unknown>
      const targetID = String(target.unitId || target.targetId || "").trim()
      if (!targetID) continue
      const pos = posMap.get(targetID)
      if (!pos) continue
      const amount = resolvedStepAmount(kind, step, target)
      if (!(amount > 0)) continue
      const isCritical = resolveStepCritical(kind, step, target)
      const color = String(display.textColor || (kind === "damage" ? resolveDamageFloatColor(step, target) : defaultFloatColor(kind)))
      const damageType = kind === "damage" ? resolveDamageType(step, target) : ""
      const icon = kind === "damage" ? resolveDamageTypeIcon(damageType) : ""
      const classNames: string[] = []
      if (kind === "damage") {
        classNames.push("battle-damage-float-label")
        if (damageType === "physical") classNames.push("battle-damage-float-label--physical")
        if (damageType === "magical") classNames.push("battle-damage-float-label--magical")
      }
      if (isCritical) classNames.push("battle-crit-float-label")
      const suffixText = [
        kind === "damage" ? resolveDamageFloatSuffix(step, target, amount, isCritical) : "",
        resolveSpecializationSourceLabel(step, target)
      ].filter(Boolean).join(" ")
      const hasRuntimeAmp = kind === "damage" && hasRuntimeDamageAmp(step, target)
      const durationMs = (isCritical || hasRuntimeAmp) ? 960 : 620
      const chainJitter = kind === "damage" ? resolveDamageChainFloatJitter(step) : { x: 0, y: 0 }
      labels.push({
        key: `${action.replayKey}-float-${stepIdx}-${targetIdx}`,
        x: pos.x + chainJitter.x,
        y: pos.y - 22 + chainJitter.y,
        text: `${kind === "heal" ? "+" : "-"}${formatFloatingNumber(amount)}${isCritical ? "!" : ""}`,
        suffixText: suffixText || undefined,
        icon: icon || undefined,
        color,
        className: classNames.join(" "),
        isCritical,
        delayMs: cumulativeDelay,
        durationMs
      })
    }
  }
  return labels
}

function resolveDamageChainFloatJitter(step: Record<string, unknown>): { x: number; y: number } {
  const meta = (step.meta || {}) as Record<string, unknown>
  const chainId = String(meta.chainId || "").trim()
  const hitIndex = Number(meta.hitIndex)
  const hitCount = Number(meta.hitCount)
  if (!chainId || !Number.isFinite(hitIndex) || !Number.isFinite(hitCount) || hitCount <= 1 || hitIndex <= 1) {
    return { x: 0, y: 0 }
  }
  const normalizedHitIndex = Math.max(1, Math.trunc(hitIndex))
  const slot = (normalizedHitIndex - 1) % 6
  const ring = Math.floor((normalizedHitIndex - 1) / 6) + 1
  const angle = (slot / 6) * Math.PI * 2
  const radius = 4 + ring * 4
  const x = Math.round(Math.cos(angle) * radius * BATTLE_CHAIN_FLOAT_JITTER_SCALE)
  const y = Math.round((-Math.sin(angle) * radius - ring * 2) * BATTLE_CHAIN_FLOAT_JITTER_SCALE)
  return { x, y }
}

function resolveReplayStepDelay(step: Record<string, unknown>, previousStep: Record<string, unknown> | null) {
  let delay = 0
  const kind = String(step.kind || "").trim().toLowerCase()
  if (shouldApplyReplayStepDelay(kind)) {
    const stepDelay = Number(step.delayMs || 0)
    if (Number.isFinite(stepDelay) && stepDelay > 0) delay += stepDelay
  }
  if (delay <= 0 && previousStep && shouldInjectItemActionInterval(previousStep, step)) {
    delay += INTERVAL_BETWEEN_ITEM_AND_ACTION
  }
  if (delay <= 0 && previousStep && shouldInjectMultiHitDamageInterval(previousStep, step)) {
    delay += INTERVAL_BETWEEN_MULTI_HIT_DAMAGE
  }
  return delay
}

function shouldApplyReplayStepDelay(kind: string) {
  // Keep timeline pacing on cast/move/summon only. Damage/buff trigger delays cause visual stutter
  // because unit state is already applied before float labels are animated.
  return ["cast", "position_move", "summon"].includes(kind)
}

function shouldInjectItemActionInterval(previousStep: Record<string, unknown>, currentStep: Record<string, unknown>) {
  const previousPhase = String(previousStep.phase || "").trim().toLowerCase()
  const currentPhase = String(currentStep.phase || "").trim().toLowerCase()
  if (previousPhase !== "item" || currentPhase !== "action") return false
  const currentKind = String(currentStep.kind || "").trim().toLowerCase()
  return currentKind === "cast"
}

function shouldInjectMultiHitDamageInterval(previousStep: Record<string, unknown>, currentStep: Record<string, unknown>) {
  const previousKind = String(previousStep.kind || "").trim().toLowerCase()
  const currentKind = String(currentStep.kind || "").trim().toLowerCase()
  if (previousKind !== "damage" || currentKind !== "damage") return false
  const previousSourceID = String(((previousStep.source || {}) as Record<string, unknown>).unitId || "").trim()
  const currentSourceID = String(((currentStep.source || {}) as Record<string, unknown>).unitId || "").trim()
  const previousTargetID = String((((Array.isArray(previousStep.targets) ? previousStep.targets[0] : {}) as Record<string, unknown>).unitId || ((Array.isArray(previousStep.targets) ? previousStep.targets[0] : {}) as Record<string, unknown>).targetId || "")).trim()
  const currentTargetID = String((((Array.isArray(currentStep.targets) ? currentStep.targets[0] : {}) as Record<string, unknown>).unitId || ((Array.isArray(currentStep.targets) ? currentStep.targets[0] : {}) as Record<string, unknown>).targetId || "")).trim()
  if (!previousSourceID || !currentSourceID || previousSourceID !== currentSourceID) return false
  if (!previousTargetID || !currentTargetID || previousTargetID !== currentTargetID) return false
  const previousMeta = (previousStep.meta || {}) as Record<string, unknown>
  const currentMeta = (currentStep.meta || {}) as Record<string, unknown>
  const previousHitIndex = Number(previousMeta.hitIndex)
  const currentHitIndex = Number(currentMeta.hitIndex)
  const previousHitCount = Number(previousMeta.hitCount)
  const currentHitCount = Number(currentMeta.hitCount)
  if (
    Number.isFinite(previousHitIndex) &&
    Number.isFinite(currentHitIndex) &&
    Number.isFinite(previousHitCount) &&
    Number.isFinite(currentHitCount) &&
    currentHitCount > 1 &&
    currentHitIndex > 1 &&
    previousHitCount === currentHitCount &&
    previousHitIndex === currentHitIndex - 1
  ) {
    return true
  }

  // Fallback for compact timeline payloads where hitIndex/hitCount is not serialized.
  // Treat consecutive same-source/same-target damage packets in action phase as multi-hit chain.
  const previousPhase = String(previousStep.phase || "").trim().toLowerCase()
  const currentPhase = String(currentStep.phase || "").trim().toLowerCase()
  if (previousPhase !== "action" || currentPhase !== "action") return false
  const previousDamageType = String(previousStep.damageType || "").trim().toLowerCase()
  const currentDamageType = String(currentStep.damageType || "").trim().toLowerCase()
  if (previousDamageType !== currentDamageType) return false
  const previousTargets = Array.isArray(previousStep.targets) ? previousStep.targets : []
  const currentTargets = Array.isArray(currentStep.targets) ? currentStep.targets : []
  if (previousTargets.length !== 1 || currentTargets.length !== 1) return false
  const previousAmount = Number(previousStep.amount)
  const currentAmount = Number(currentStep.amount)
  if (Number.isFinite(previousAmount) && Number.isFinite(currentAmount) && Math.abs(previousAmount - currentAmount) > 0.0001) return false
  return true
}

function resolveSkillFloatLabel(step: Record<string, unknown>) {
  const skill = (step.skill || {}) as Record<string, unknown>
  const skillID = String(skill.skillId || "").trim()
  const skillName = String(skill.skillName || "").trim()
  let text = ""
  let icon = ""
  if (skillID) {
    const localized = getLocalizedSkillName(t, skillID, skillName || skillID)
    if (localized && localized.trim()) text = localized.trim()
    icon = String(getSkillDefinition(skillID)?.icon || "").trim()
  }
  if (!text && skillName) text = skillName
  if (!text) text = skillID
  return { text, icon }
}

function resolveBuffFloatLabel(step: Record<string, unknown>) {
  const buffID = String(step.buffId || step.buffID || step.stateId || "").trim()
  if (!buffID) return ""
  return getLocalizedBuffName(t, buffID, buffID).trim()
}

watch(currentAction, async (action) => {
  battleTrajectories.value = []
  battleFloatLabels.value = []
  if (!action) return
  await nextTick()
  let posMap = collectUnitPositionMap()
  if (!posMap || posMap.size <= 0) {
    await nextTick()
    posMap = collectUnitPositionMap()
  }
  if (!posMap || posMap.size <= 0) return
  // Render trajectories and labels from all actions in the batch
  // so synced summons animate simultaneously with the main actor.
  const batch = currentBatchActions.value.length > 1 ? currentBatchActions.value : [action]
  let trajectories: BattleTrajectory[] = []
  let labels: BattleFloatLabel[] = []
  for (const batchAction of batch) {
    trajectories = trajectories.concat(buildTrajectories(batchAction, posMap))
    labels = labels.concat(buildFloatLabels(batchAction, posMap))
  }
  battleTrajectories.value = trajectories
  battleFloatLabels.value = labels
  currentBatchActions.value = []
})

watch(battleTrajectories, async (next) => {
  for (const handle of animationHandles.splice(0)) {
    handle.kill()
  }
  if (next.length <= 0) return
  await nextTick()
  for (const trajectory of next) {
    const line = lineRefs.get(trajectory.key)
    if (!line) continue
    gsap.set(line, {
      attr: { x2: trajectory.x1, y2: trajectory.y1 },
      opacity: 0.95
    })
    const duration = Math.max(0.05, trajectory.durationMs / 1000 / playbackRate.value)
    animationHandles.push(gsap.to(line, {
      duration,
      ease: "power2.out",
      delay: Math.max(0, trajectory.delayMs / 1000 / playbackRate.value),
      attr: { x2: trajectory.x2, y2: trajectory.y2 }
    }))
    animationHandles.push(gsap.to(line, {
      duration: 0.2 / playbackRate.value,
      delay: Math.max(0, trajectory.delayMs / 1000 / playbackRate.value) + duration,
      opacity: 0
    }))
  }
})

watch(battleFloatLabels, async (next) => {
  if (next.length <= 0) return
  await nextTick()
  for (const floatLabel of next) {
    const node = floatLabelRefs.get(floatLabel.key)
    if (!node) continue
    gsap.set(node, {
      xPercent: -50,
      yPercent: -50,
      y: 0,
      opacity: 0,
      scale: 0.96
    })
    const delaySec = Math.max(0, floatLabel.delayMs / 1000 / playbackRate.value)
    const durationSec = Math.max(0.06, floatLabel.durationMs / 1000 / playbackRate.value)
    if (floatLabel.isCritical) {
      animationHandles.push(gsap.to(node, {
        duration: Math.max(0.1, durationSec * 0.2),
        delay: delaySec,
        opacity: 1,
        y: -14,
        scale: 1.22,
        ease: "back.out(2.8)"
      }))
      animationHandles.push(gsap.to(node, {
        duration: Math.max(0.08, durationSec * 0.16),
        delay: delaySec + Math.max(0.1, durationSec * 0.2),
        keyframes: [
          { x: -2 },
          { x: 2 },
          { x: -1 },
          { x: 1 },
          { x: 0 }
        ],
        ease: "power1.out"
      }))
      animationHandles.push(gsap.to(node, {
        duration: Math.max(0.14, durationSec * 0.64),
        delay: delaySec + Math.max(0.1, durationSec * 0.36),
        opacity: 0,
        y: -36,
        scale: 0.98,
        ease: "power2.in"
      }))
      continue
    }
    animationHandles.push(gsap.to(node, {
      duration: durationSec * 0.36,
      delay: delaySec,
      opacity: 1,
      y: -9,
      scale: 1.04,
      ease: "power2.out"
    }))
    animationHandles.push(gsap.to(node, {
      duration: Math.max(0.08, durationSec * 0.18),
      delay: delaySec + Math.max(0.06, durationSec * 0.1),
      keyframes: [
        { x: -1 },
        { x: 1 },
        { x: 0 }
      ],
      ease: "power1.out"
    }))
    animationHandles.push(gsap.to(node, {
      duration: durationSec * 0.64,
      delay: delaySec + durationSec * 0.36,
      opacity: 0,
      y: -24,
      scale: 0.98,
      ease: "power1.in"
    }))
  }
})

onMounted(() => {
  progressTicker = setInterval(() => {
    nowMs.value = Date.now()
  }, 100)
})

onBeforeUnmount(() => {
  clearReplayTimer()
  if (progressTicker) {
    clearInterval(progressTicker)
    progressTicker = null
  }
  for (const handle of animationHandles.splice(0)) {
    handle.kill()
  }
  clearMoveAnimations()
})

function defaultLineColor(kind: string) {
  switch (kind) {
    case "heal":
      return "#22c55e"
    case "resource":
      return "#38bdf8"
    case "buff":
      return "#a78bfa"
    case "corpse_focus":
      return "#f59e0b"
    case "damage":
    default:
      return "#ef4444"
  }
}

function defaultFloatColor(kind: string) {
  if (kind === "heal") return "#22c55e"
  return "#ef4444"
}

function resolveDamageType(step: Record<string, unknown>, target: Record<string, unknown>) {
  const raw = String(step.damageType || target.damageType || "").trim().toLowerCase()
  if (raw === "physical") return "physical"
  if (raw === "magical" || raw === "magic") return "magical"
  return ""
}

function resolveDamageFloatColor(step: Record<string, unknown>, target: Record<string, unknown>) {
  const damageType = resolveDamageType(step, target)
  if (damageType === "physical") return "#f97316"
  if (damageType === "magical") return "#60a5fa"
  return "#ef4444"
}

function resolveDamageTypeIcon(damageType: string) {
  if (damageType === "physical") return "game-icons:gladius"
  if (damageType === "magical") return "game-icons:fairy-wand"
  return ""
}

function resolvedStepAmount(kind: string, step: Record<string, unknown>, target: Record<string, unknown>) {
  const amount = Number(step.amount || target.amount || 0)
  if (Number.isFinite(amount) && amount > 0) return amount
  const hpBefore = Number(target.hpBefore)
  const hpAfter = Number(target.hpAfter)
  if (Number.isFinite(hpBefore) && Number.isFinite(hpAfter)) {
    const delta = kind === "heal" ? (hpAfter - hpBefore) : (hpBefore - hpAfter)
    if (delta > 0) return delta
  }
  return 0
}

function formatFloatingNumber(value: number) {
  if (!Number.isFinite(value)) return "0"
  const rounded = Math.round(value * 100) / 100
  if (Number.isInteger(rounded)) return String(rounded)
  return rounded.toFixed(2).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
}

function resolveStepCritical(kind: string, step: Record<string, unknown>, target: Record<string, unknown>) {
  if (kind !== "damage") return false
  if (typeof step.crit === "boolean") return step.crit
  if (typeof target.crit === "boolean") return target.crit
  return false
}

function resolveDamageFloatSuffix(
  step: Record<string, unknown>,
  target: Record<string, unknown>,
  finalAmount: number,
  isCritical: boolean
) {
  const meta = resolveStepMeta(step, target)
  const parts: string[] = []
  const runtimeAmp = Number(meta.runtimeDamageAmp ?? 0)
  if (Number.isFinite(runtimeAmp) && runtimeAmp > 0 && finalAmount > 0) {
    const ampExtra = finalAmount - (finalAmount / (1 + runtimeAmp))
    if (ampExtra > 0) {
      parts.push(`(+${formatFloatingNumber(ampExtra)})`)
    }
  }
  const criticalMultiplier = Number(meta.criticalMultiplier ?? 0)
  if (isCritical && Number.isFinite(criticalMultiplier) && criticalMultiplier > 1) {
    parts.push(`(x${formatFloatingNumber(criticalMultiplier)})`)
  }
  return parts.join(" ")
}

function hasRuntimeDamageAmp(step: Record<string, unknown>, target: Record<string, unknown>) {
  const meta = resolveStepMeta(step, target)
  const runtimeAmp = Number(meta.runtimeDamageAmp ?? 0)
  return Number.isFinite(runtimeAmp) && runtimeAmp > 0
}

function resolveStepMeta(step: Record<string, unknown>, target: Record<string, unknown>) {
  const stepMeta = step.meta
  if (stepMeta && typeof stepMeta === "object" && !Array.isArray(stepMeta)) {
    return stepMeta as Record<string, unknown>
  }
  const targetMeta = target.meta
  if (targetMeta && typeof targetMeta === "object" && !Array.isArray(targetMeta)) {
    return targetMeta as Record<string, unknown>
  }
  return {}
}

function resolveSpecializationSourceLabel(step: Record<string, unknown>, target: Record<string, unknown> = {}) {
  return buildBattleSpecializationSourceLabel(resolveStepMeta(step, target), t)
}

function parseTimestampMs(raw: unknown) {
  if (!raw) return 0
  const value = new Date(String(raw))
  const ms = value.getTime()
  if (!Number.isFinite(ms) || Number.isNaN(ms)) return 0
  return ms
}

function toProgressPercent(doneMs: number, totalMs: number) {
  if (!Number.isFinite(doneMs) || !Number.isFinite(totalMs) || totalMs <= 0) return 0
  const ratio = Math.max(0, Math.min(1, doneMs / totalMs))
  return Math.round(ratio * 1000) / 10
}

function formatDurationLabel(ms: number) {
  const safeMs = Math.max(0, Math.trunc(Number(ms) || 0))
  const totalSeconds = Math.ceil(safeMs / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes <= 0) return `${seconds}s`
  return `${minutes}m ${String(seconds).padStart(2, "0")}s`
}

function setLineRef(key: string, node: SVGLineElement | null) {
  if (!node) {
    lineRefs.delete(key)
    return
  }
  lineRefs.set(key, node)
}

function setFloatLabelRef(key: string, node: HTMLDivElement | null) {
  if (!node) {
    floatLabelRefs.delete(key)
    return
  }
  floatLabelRefs.set(key, node)
}

function isActiveUnit(slot: BattleSlot) {
  if (!activeActorID.value) return false
  return String(slot.unit.unitId || slot.key).trim() === activeActorID.value
}

function resolveUnitName(unit: Record<string, unknown>): string {
  const nameKey = String(unit.nameKey || "").trim()
  if (nameKey) {
    const localized = t(nameKey)
    if (localized && localized !== nameKey) return localized
  }
  if (String(unit.side || "") !== "enemy") {
    const candidates = [unit.name, unit.playerName, unit.nickname, unit.displayName]
    for (const candidate of candidates) {
      const name = String(candidate || "").trim()
      if (name) return name
    }
    return t("game.playerLabel")
  }
  const enemyName = String(unit.name || "").trim()
  if (enemyName) return enemyName
  return "-"
}

function refreshBattleState() {
  if (!selectedRoom.value?.room.id) return
  void selectRoom(selectedRoom.value.room.id)
}
</script>

<style scoped>
.battle-detail-tabs {
  min-width: 0;
}

.battle-detail-tabs:deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.battle-detail-tabs:deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background-color: var(--app-border);
}

.battle-detail-tabs:deep(.el-tabs__item) {
  height: 36px;
  padding: 0 16px;
  font-size: 13px;
  font-weight: 650;
}

.battle-detail-tabs:deep(.el-tabs__content),
.battle-detail-tabs:deep(.el-tab-pane) {
  min-width: 0;
}

.battlefield-effect-strip {
  width: 100%;
  border: 1px solid color-mix(in srgb, #b7791f 55%, var(--app-border));
  border-radius: 6px;
  background: color-mix(in srgb, #171717 82%, #b7791f 18%);
  color: #f8fafc;
  overflow: hidden;
}

.battlefield-effect-row {
  min-height: 34px;
  display: grid;
  grid-template-columns: 18px minmax(6.5rem, auto) auto minmax(5.5rem, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.25;
}

.battlefield-effect-row--warning {
  background: color-mix(in srgb, #7f1d1d 42%, transparent);
}

.battlefield-effect-icon {
  width: 16px;
  height: 16px;
  color: #f6c453;
}

.battlefield-effect-name,
.battlefield-effect-count {
  white-space: nowrap;
}

.battlefield-effect-count {
  color: #fde68a;
  font-variant-numeric: tabular-nums;
}

.battlefield-effect-progress {
  min-width: 0;
  color: #cbd5e1;
}

.battlefield-effect-window {
  white-space: nowrap;
  color: #7dd3fc;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.battlefield-effect-trigger-summary {
  min-width: 0;
  color: var(--el-color-warning-light-3);
  font-weight: 700;
  overflow-wrap: anywhere;
}

.battlefield-effect-strip--triggered {
  animation: battlefield-trigger-flash 0.9s ease-out;
}

@keyframes battlefield-trigger-flash {
  0% { box-shadow: 0 0 0 2px rgba(251, 191, 36, 0.9), inset 0 0 28px rgba(248, 113, 113, 0.65); }
  100% { box-shadow: 0 0 0 0 transparent, inset 0 0 0 transparent; }
}

.battle-turn-order-tag {
  transition: box-shadow 0.18s ease, border-color 0.18s ease;
}

.battle-loading-placeholder {
  min-height: 168px;
  display: grid;
  place-items: center;
  grid-auto-flow: column;
  justify-content: center;
  gap: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  color: var(--app-muted);
  font-size: 13px;
  line-height: 1.5;
}

.battle-loading-spinner {
  width: 18px;
  height: 18px;
  box-sizing: border-box;
  border: 2px solid color-mix(in srgb, var(--app-muted) 24%, transparent);
  border-top-color: var(--el-color-primary);
  border-radius: 999px;
  animation: battle-loading-spin 0.8s linear infinite;
}

@keyframes battle-loading-spin {
  to {
    transform: rotate(360deg);
  }
}

.battle-turn-order-tag-hovered {
  box-shadow: 0 0 0 1px rgba(56, 189, 248, 0.5), 0 0 10px rgba(56, 189, 248, 0.32);
}

.battle-turn-order-tag-hovered:deep(.el-tag__content) {
  color: #0369a1;
}

.battle-side-section {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 8px;
  width: 100%;
}

.battle-side-label {
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

@media (min-width: 768px) {
  .battle-side-section {
    grid-template-columns: minmax(3.5rem, 3.5rem) minmax(0, 1fr);
    align-items: center;
    gap: 10px;
  }

  .battle-side-label {
    overflow-wrap: break-word;
    font-size: 13px;
  }
}

.battle-skill-float-label {
  --battle-skill-float-text-color: #0f172a;
  border: 1px solid color-mix(in srgb, var(--app-border) 86%, #ffffff 14%);
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 11px;
  letter-spacing: 0.01em;
  line-height: 1;
  color: var(--battle-skill-float-text-color);
  text-shadow: 0 1px 1px rgba(255, 255, 255, 0.5);
  background: color-mix(in srgb, #ffffff 92%, var(--app-surface) 8%);
  box-shadow: 0 4px 12px color-mix(in srgb, #0f172a 16%, transparent);
}

.battle-crit-float-label {
  font-size: 18px;
  font-weight: 800;
  letter-spacing: 0.015em;
  text-shadow:
    0 0 8px rgba(251, 146, 60, 0.32),
    0 2px 10px rgba(15, 23, 42, 0.38);
}

.battle-damage-float-label {
  font-size: 16px;
  text-shadow: 0 1px 6px rgba(15, 23, 42, 0.18);
}

.battle-float-label-suffix {
  margin-left: 6px;
  font-size: 0.72em;
  font-weight: 600;
  opacity: 0.88;
}

.battle-damage-float-label--physical {
  text-shadow:
    0 0 8px rgba(249, 115, 22, 0.24),
    0 2px 8px rgba(124, 45, 18, 0.24);
}

.battle-damage-float-label--magical {
  text-shadow:
    0 0 8px rgba(96, 165, 250, 0.24),
    0 2px 8px rgba(30, 58, 138, 0.24);
}

.battle-status-float-label {
  --battle-status-float-color: #334155;
  border: 1px solid color-mix(in srgb, var(--app-border) 86%, #ffffff 14%);
  border-radius: 999px;
  padding: 2px 7px;
  line-height: 1;
  background: color-mix(in srgb, #ffffff 92%, var(--app-surface) 8%);
  box-shadow: 0 2px 8px color-mix(in srgb, #0f172a 12%, transparent);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.01em;
  text-shadow: 0 1px 4px rgba(15, 23, 42, 0.15);
  opacity: 0.88;
}

.battle-status-float-label--gain {
  --battle-status-float-color: #7c3aed;
}

.battle-status-float-label--loss {
  --battle-status-float-color: #dc2626;
}

.battle-item-float-label {
  box-shadow: 0 6px 16px color-mix(in srgb, #0f172a 18%, transparent);
}

:global(.dark) .battle-skill-float-label {
  --battle-skill-float-text-color: #fef3c7;
  border: 1px solid rgba(251, 191, 36, 0.55);
  text-shadow: 0 1px 2px rgba(2, 6, 23, 0.6);
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.88), rgba(30, 41, 59, 0.76));
  box-shadow: 0 4px 14px rgba(2, 6, 23, 0.34);
}

:global(.dark) .battle-status-float-label {
  --battle-status-float-color: #cbd5e1;
  border: 1px solid rgba(251, 191, 36, 0.55);
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.88), rgba(30, 41, 59, 0.76));
  box-shadow: 0 4px 14px rgba(2, 6, 23, 0.34);
  text-shadow: 0 1px 8px rgba(2, 6, 23, 0.52);
}

:global(.dark) .battle-status-float-label--gain {
  --battle-status-float-color: #c4b5fd;
}

:global(.dark) .battle-status-float-label--loss {
  --battle-status-float-color: #fda4af;
}

@media (max-width: 767px) {
  .battle-view {
    padding-bottom: calc(12px + env(safe-area-inset-bottom));
  }

  .battle-canvas {
    min-height: clamp(260px, calc(100dvh - 260px), 320px);
    padding: 10px;
  }

  .battle-canvas:deep(.el-divider) {
    margin-top: 10px !important;
    margin-bottom: 10px !important;
  }

  .battlefield-effect-row {
    grid-template-columns: 16px minmax(0, 1fr) auto;
    gap: 6px;
    padding: 7px 8px;
  }

  .battlefield-effect-progress {
    grid-column: 2 / 4;
  }

  .battlefield-effect-window {
    grid-column: 2 / 4;
    white-space: normal;
  }

  .battlefield-effect-trigger-summary {
    grid-column: 2 / 4;
  }
}
</style>
