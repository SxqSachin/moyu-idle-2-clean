<template>
  <div
    class="queue-card"
    :class="cardClass"
    tabindex="0"
    @click="handleSelect"
    @keydown.enter.prevent="handleRunNow"
  >
    <div class="queue-card__top">
      <div class="queue-card__identity flex min-w-0 items-center gap-3">
        <div v-if="isFishingCard" class="queue-card__fishing-icon">
          <Icon icon="game-icons:fishing-hook" aria-hidden="true" />
        </div>
        <div v-else>
          <LabeledFrameCard
            hide-label
            :width="42"
            :height="42"
            :rarity-color="rarityColor"
          >
            <TypeIcon :type="card.iconType" :id="card.iconId" :alt="card.label" :size="18" :fallback="card.iconFallback" />
          </LabeledFrameCard>
        </div>
        <div class="queue-card__copy flex min-w-0 flex-1 flex-col">
          <div class="queue-card__title-line flex items-center gap-2">
            <div class="queue-card__title truncate text-sm font-semibold text-[var(--app-text)]">
              {{ card.label }}
            </div>
            <span class="queue-card__status" :class="statusClass">
              {{ cardStatusLabel }}
            </span>
          </div>
          <div class="queue-card__meta-line mt-1 flex min-w-0 items-center gap-2 text-xs text-[var(--app-muted)]">
            <span v-if="isFishingCard">{{ fishingMetaLabel }}</span>
            <template v-else>
              <span>{{ card.doneCount }} / {{ card.totalCount }}</span>
              <span>·</span>
              <span class="queue-card__eta">{{ etaLabel }}</span>
            </template>
          </div>
        </div>
      </div>

      <el-button
        v-if="card.status !== 'pending'"
        size="small"
        type="danger"
        plain
        :disabled="isKittyCombatPaused"
        @click.stop="$emit('cancel', card.id)"
      >
        {{ cancelLabel }}
      </el-button>
        <div v-if="card.status === 'pending'" class="queue-card__pending-actions flex flex-wrap items-center justify-between gap-2 md:justify-end">
          <el-button class="ml-0!" size="small" :disabled="isKittyCombatPaused || !canMoveTop" @click.stop="$emit('move-top', card.id)">
            {{ t("game.queueMoveTop") }}
          </el-button>
          <el-button class="ml-0!" size="small" :disabled="isKittyCombatPaused || !canMoveBottom" @click.stop="$emit('move-bottom', card.id)">
            {{ t("game.queueMoveBottom") }}
          </el-button>
          <el-button class="ml-0!" size="small" :disabled="isKittyCombatPaused || !canMoveUp" @click.stop="$emit('move-up', card.id)">
            {{ t("game.queueMoveUp") }}
          </el-button>
          <el-button class="ml-0!" size="small" :disabled="isKittyCombatPaused || !canMoveDown" @click.stop="$emit('move-down', card.id)">
            {{ t("game.queueMoveDown") }}
          </el-button>
          <el-button class="ml-0!" size="small" type="primary" :disabled="isKittyCombatPaused" @click.stop="$emit('run-now', card.id)">
            {{ t("game.queueRunNow") }}
          </el-button>

          <el-button class="ml-0!" size="small" type="danger" plain :disabled="isKittyCombatPaused" @click.stop="$emit('cancel', card.id)">
            {{ cancelLabel }}
          </el-button>
        </div>
    </div>

    <div>
      <slot name="content" :card="card" />
    </div>

    <div v-if="showProgress" class="queue-card__progress-block">
      <div class="queue-progress">
        <div class="queue-progress__bar" :style="progressBarStyle" />
      </div>
      <div class="mt-2 flex items-center justify-center text-xs text-[var(--app-muted)]">
        <!-- <span>{{ progressPctText }}</span> -->
        <span>{{ currentRemainingText }}</span>
      </div>
    </div>
    <div v-if="isKittyCombatPaused" class="queue-card__combat-overlay">
      <div class="queue-card__combat-overlay-text">{{ t("game.queueKittyCombatPausedHint") }}</div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import type { QueueCard } from "./types"
import { formatCountdownMillisecondsOneDecimal, formatDurationI18n } from "@/utils/duration-format"
import { getActionDefinition } from "@/services/action-definition"
import { getRarityDefinition } from "@/services/item-definition"
import { useRealtimeStore } from "@stores/realtime"

const realtimeStore = useRealtimeStore()

function adjustedNow(): number {
  return Date.now() - (realtimeStore.serverClockSkewMs ?? 0)
}

const props = defineProps<{
  card: QueueCard
  canMoveTop: boolean
  canMoveBottom: boolean
  canMoveUp: boolean
  canMoveDown: boolean
  selected: boolean
}>()

const emit = defineEmits<{
  (e: "select", queueID: number): void
  (e: "move-top", queueID: number): void
  (e: "move-bottom", queueID: number): void
  (e: "move-up", queueID: number): void
  (e: "move-down", queueID: number): void
  (e: "run-now", queueID: number): void
  (e: "cancel", queueID: number): void
}>()

const { t, locale } = useI18n()

const nowMs = ref(Date.now())
const animationBasePct = ref(0)
const animationStartMs = ref(Date.now())
const animationDurationMs = ref(120)
let nowTimer: number | undefined

function clampPct(value: number): number {
  return Math.max(0, Math.min(100, value))
}

function resetAnimationAnchor() {
  const now = adjustedNow()
  nowMs.value = now
  animationStartMs.value = now
  animationDurationMs.value = Math.max(120, props.card.runEndsAtMs - now)
  if (
    props.card.status === "running" &&
    props.card.runStartAtMs > 0 &&
    props.card.runEndsAtMs > props.card.runStartAtMs
  ) {
    const totalMs = props.card.runEndsAtMs - props.card.runStartAtMs
    animationBasePct.value = clampPct(((now - props.card.runStartAtMs) / totalMs) * 100)
  } else {
    animationBasePct.value = clampPct(props.card.progressPct)
  }
}

function startNowTimer() {
  if (nowTimer !== undefined) {
    return
  }
  nowTimer = window.setInterval(() => {
    nowMs.value = adjustedNow()
  }, 120)
}

function stopNowTimer() {
  if (nowTimer === undefined) {
    return
  }
  window.clearInterval(nowTimer)
  nowTimer = undefined
}

watch(
  () => [props.card.status, props.card.progressPct, props.card.runEndsAtMs, realtimeStore.serverClockSkewMs],
  () => {
    resetAnimationAnchor()
    if (props.card.status === "running") {
      startNowTimer()
      return
    }
    stopNowTimer()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  stopNowTimer()
})

const liveProgressPct = computed(() => {
  if (props.card.status !== "running") {
    return clampPct(props.card.progressPct)
  }

  const elapsedMs = Math.max(0, nowMs.value - animationStartMs.value)
  const ratio = Math.max(0, Math.min(1, elapsedMs / animationDurationMs.value))
  const pct = animationBasePct.value + (100 - animationBasePct.value) * ratio
  return clampPct(pct)
})

const progressBarStyle = computed(() => ({
  width: `${liveProgressPct.value}%`
}))

const progressPctText = computed(() => `${Math.round(liveProgressPct.value)}%`)
const isFishingCard = computed(() => props.card.actionId === "fishing.trip" || !!props.card.display?.fishing)
const isContinuousFishing = computed(
  () => isFishingCard.value && Number(props.card.display?.fishing?.plannedDurationSeconds || 0) <= 0
)
const showProgress = computed(
  () => props.card.status === "running" && (!isFishingCard.value || !isContinuousFishing.value)
)
const isKittyCombatPaused = computed(
  () => props.card.actorType === "kitty" && props.card.statusLabelKey === "game.queueStatus.in_combat"
)

const currentRemainingText = computed(() => {
  if (props.card.status !== "running") return ""
  const remainingMs = Math.max(0, (props.card.runEndsAtMs || 0) - nowMs.value)
  if (isFishingCard.value) {
    return t("game.fishing.queueRemaining", {
      time: formatDurationI18n(Math.ceil(remainingMs / 1000), t)
    })
  }
  return formatCountdownMillisecondsOneDecimal(remainingMs, t)
})

const statusLabel = computed(() => t(props.card.statusLabelKey ?? `game.queueStatus.${props.card.status}`))
const cardStatusLabel = computed(() => {
  if (props.card.statusLabelKey === "game.queueStatus.in_combat") return statusLabel.value
  if (!isFishingCard.value) return statusLabel.value
  if (props.card.status === "pending") return t("game.fishing.queueWaiting")
  const phase = String(props.card.display?.fishing?.phase || "waiting")
  const phaseKey = phase === "bite"
    ? "phaseBite"
    : phase === "fighting"
      ? "phaseFighting"
      : phase === "landed"
        ? "phaseLanded"
        : phase === "resting"
          ? "phaseResting"
          : phase === "finished"
            ? "phaseFinished"
            : "phaseWaiting"
  return t(`game.fishing.${phaseKey}`)
})

const etaLabel = computed(() => {
  if (props.card.etaUnknown) return "—"
  const etaMs = props.card.status === "running" ? props.card.runEndsAtMs : props.card.startAt
  return new Date(etaMs).toLocaleString()
})
const fishingMetaLabel = computed(() => {
  const fishing = props.card.display?.fishing
  if (!fishing) return ""
  if (fishing.plannedDurationSeconds <= 0) return t("game.fishing.queueContinuous")
  if (props.card.status === "running") {
    const endAt = Number(fishing.plannedEndAtMs || props.card.runEndsAtMs || 0)
    return t("game.fishing.queueEndsAt", {
      time: endAt > 0 ? new Date(endAt).toLocaleTimeString(locale.value, { hour: "2-digit", minute: "2-digit" }) : "—"
    })
  }
  return t("game.fishing.queuePlannedDuration", {
    duration: formatDurationI18n(fishing.plannedDurationSeconds, t)
  })
})
const cancelLabel = computed(() => {
  if (!isFishingCard.value) return t("game.queueCancel")
  return props.card.status === "running"
    ? t("game.fishing.stop")
    : t("game.fishing.queueCancelPending")
})

const actionDef = computed(() => getActionDefinition(props.card.actionId))
const rarityColor = computed(() =>{
  if (!actionDef.value) return "#9CA3AF"

  return getRarityDefinition(actionDef.value?.rarity)?.color || "#9CA3AF"
});

  

const cardClass = computed(() => ({
  "queue-card--running": props.card.status === "running",
  "queue-card--pending": props.card.status === "pending",
  "queue-card--fishing": isFishingCard.value,
  "queue-card--selected": props.selected
}))

const iconClass = computed(() => ({
  "queue-card__icon--running": props.card.status === "running",
  "queue-card__icon--pending": props.card.status === "pending"
}))

const statusClass = computed(() => ({
  "queue-card__status--running": props.card.status === "running",
  "queue-card__status--pending": props.card.status === "pending",
  "queue-card__status--combat": props.card.statusLabelKey === "game.queueStatus.in_combat"
}))

function handleSelect() {
  if (isKittyCombatPaused.value) return
  emit("select", props.card.id)
}

function handleRunNow() {
  if (isKittyCombatPaused.value) return
  emit("run-now", props.card.id)
}
</script>

<style scoped>
.queue-card {
  position: relative;
  overflow: hidden;
  border-radius: 18px;
  border: 1px solid color-mix(in srgb, var(--app-border) 80%, transparent);
  background: color-mix(in srgb, var(--app-surface) 98%, transparent);
  padding: 16px;
  transition: transform 150ms ease, border-color 150ms ease, box-shadow 150ms ease;
}

.queue-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 20px rgba(15, 23, 42, 0.06);
}

.queue-card--selected {
  border-color: color-mix(in srgb, var(--el-color-primary) 65%, transparent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--el-color-primary) 60%, transparent) inset;
}

.queue-card--running {
  border-color: color-mix(in srgb, #60a5fa 55%, transparent);
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.98), rgba(224, 231, 255, 0.92));
}

.queue-card--fishing.queue-card--running {
  border-color: color-mix(in srgb, #178081 58%, transparent);
  background: linear-gradient(135deg, color-mix(in srgb, #edf9f7 84%, var(--app-surface)), color-mix(in srgb, #f4f7ff 82%, var(--app-surface)));
}

.dark .queue-card--running {
  background: linear-gradient(135deg, rgba(30, 41, 59, 0.95), rgba(17, 24, 39, 0.95));
}

.dark .queue-card--fishing.queue-card--running {
  background: linear-gradient(135deg, color-mix(in srgb, #123f40 38%, var(--app-surface)), color-mix(in srgb, #263553 34%, var(--app-surface)));
}

.queue-card__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.queue-card__identity {
  flex: 1 1 22rem;
  min-width: min(100%, 18rem);
}

.queue-card__copy {
  min-width: 0;
}

.queue-card__meta-line {
  overflow: hidden;
  white-space: nowrap;
}

.queue-card__eta {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.queue-card__pending-actions {
  flex: 0 1 auto;
  max-width: 100%;
}

.queue-card__fishing-icon {
  width: 42px;
  height: 42px;
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, #178081 32%, var(--app-border));
  border-radius: 6px;
  color: #126f70;
  background: color-mix(in srgb, #d9f2ee 72%, var(--app-surface));
}

.queue-card__fishing-icon svg {
  width: 22px;
  height: 22px;
}

.queue-card__icon {
  width: 42px;
  height: 42px;
  flex: none;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.queue-card__icon--running {
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.2), rgba(129, 140, 248, 0.24));
}

.queue-card__icon--pending {
  background: linear-gradient(135deg, rgba(229, 231, 235, 0.9), rgba(209, 213, 219, 0.72));
}

.queue-card__status {
  flex: none;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 12px;
  line-height: 18px;
}

.queue-card__status--running {
  background: rgba(59, 130, 246, 0.12);
  color: rgb(29, 78, 216);
}

.queue-card--fishing .queue-card__status--running {
  background: color-mix(in srgb, #178081 13%, transparent);
  color: #126f70;
}

.queue-card__status--pending {
  background: rgba(107, 114, 128, 0.12);
  color: rgb(75, 85, 99);
}

.queue-card__status--combat {
  background: rgba(249, 115, 22, 0.16);
  color: rgb(194, 65, 12);
}

.queue-card__content {
  margin-top: 12px;
}

.queue-card__progress-block {
  margin-top: 14px;
}

.queue-card__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 14px;
  flex-wrap: wrap;
}

.queue-progress {
  height: 8px;
  width: 100%;
  border-radius: 999px;
  overflow: hidden;
  background: color-mix(in srgb, var(--app-muted) 20%, transparent);
}

.queue-progress__bar {
  height: 100%;
  width: 0;
  border-radius: 999px;
  background: #818cf8;
  transition: width 120ms linear;
}

.queue-card--fishing .queue-progress__bar {
  background: #178081;
}

.queue-card__combat-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 18px;
  backdrop-filter: blur(4px);
  background: rgba(17, 24, 39, 0.22);
  z-index: 10;
}

.queue-card__combat-overlay-text {
  border-radius: 999px;
  padding: 6px 12px;
  background: rgba(17, 24, 39, 0.65);
  color: #fff;
  font-size: 12px;
  line-height: 1;
}

@media (max-width: 640px) {
  .queue-card {
    padding: 14px;
  }

  .queue-card--fishing :deep(.el-button) {
    min-height: 44px;
  }

  .queue-card--fishing .queue-card__title-line {
    align-items: flex-start;
    flex-direction: column;
    gap: 4px;
  }

  .queue-card--fishing .queue-card__title {
    overflow: visible;
    white-space: normal;
  }

  .queue-card__top {
    flex-direction: column;
    align-items: stretch;
  }

  .queue-card__identity {
    flex: none;
    min-width: 0;
    width: 100%;
  }

  .queue-card__pending-actions {
    width: 100%;
  }

  .queue-card__actions {
    justify-content: stretch;
  }

  .queue-card__actions :deep(.el-button) {
    flex: 1 1 0;
  }
}
</style>
