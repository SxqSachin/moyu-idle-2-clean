<template>
  <div
    class="queue-shell p-4! overflow-hidden"
    :class="{ 'queue-shell--fishing': isFishingLane }"
    data-tutorial="queue-lane"
  >
    <div class="flex flex-row justify-between items-center mb-2">
      <div class="flex min-w-0 items-center gap-3">
        <div class="queue-shell__badge">
          <Icon v-if="isFishingLane" icon="game-icons:fishing-hook" aria-hidden="true" />
          <span v-else>⚡</span>
        </div>
        <div class="min-w-0 flex flex-col gap-0 md:gap-2">
          <h3 class="truncate text-md font-semibold tracking-tight text-[var(--app-text)]">
            {{ title }}
          </h3>
          <span class="flex flex-row items-center md:hidden mt-1 text-sm text-[var(--app-muted)]">
            <span class="text-[12px]! md:text-[14px]! mr-1">{{ etaHeading }}: </span>
            <span class="text-[12px]! md:text-[14px]!">{{ etaLabel }}</span>
          </span>
        </div>
      </div>

      <div class="queue-shell__meta-item hidden md:flex flex-col">
        <span class="queue-shell__meta-label text-[10px]! md:text-[12px]!">{{ etaHeading }}</span>
        <span class="queue-shell__meta-value text-[12px]! md:text-[14px]!">{{ etaLabel }}</span>
      </div>
    </div>

    <div v-if="cards.length" class="queue-shell__body pt-2">
      <div v-if="executingCard" class="queue-shell__running-sticky">
        <ActionQueueItemCard
          :card="executingCard"
          :can-move-top="false"
          :can-move-bottom="false"
          :can-move-up="false"
          :can-move-down="false"
          selected
          @select="selectCard"
          @move-up="$emit('move', { queueId: $event, direction: 'up' })"
          @move-down="$emit('move', { queueId: $event, direction: 'down' })"
          @run-now="$emit('run-now', $event)"
          @cancel="$emit('cancel', $event)"
        >
          <template #content="{ card: slotCard }">
            <QueueCardContentRenderer :card="slotCard" />
          </template>
        </ActionQueueItemCard>
      </div>

      <div v-if="pendingCards.length" class="space-y-3">
        <ActionQueueItemCard
          v-for="card in pendingCards"
          :key="card.id"
          :card="card"
          :can-move-top="canMove(card.id, 'top')"
          :can-move-bottom="canMove(card.id, 'bottom')"
          :can-move-up="canMove(card.id, 'up')"
          :can-move-down="canMove(card.id, 'down')"
          :selected="selectedPendingID === card.id"
          @select="selectCard"
          @move-top="$emit('move', { queueId: $event, direction: 'top' })"
          @move-bottom="$emit('move', { queueId: $event, direction: 'bottom' })"
          @move-up="$emit('move', { queueId: $event, direction: 'up' })"
          @move-down="$emit('move', { queueId: $event, direction: 'down' })"
          @run-now="$emit('run-now', $event)"
          @cancel="$emit('cancel', $event)"
        >
          <template #content="{ card: slotCard }">
            <QueueCardContentRenderer :card="slotCard" />
          </template>
        </ActionQueueItemCard>
      </div>

      <div v-if="!executingCard && !pendingCards.length" class="queue-shell__empty">
        {{ t("game.actionEmpty") }}
      </div>
    </div>

    <div v-else class="queue-shell__empty-panel">
      <div class="queue-shell__empty-icon">💤</div>
      <div class="text-center">
        <div class="text-base font-medium text-[var(--app-text)]">{{ t("game.actionEmpty") }}</div>
        <!-- <div class="mt-1 text-sm text-[var(--app-muted)]">{{ subtitle || t("game.homeQueueHint") }}</div> -->
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import ActionQueueItemCard from "@components/game/queue/ActionQueueItemCard.vue"
import QueueCardContentRenderer from "@components/game/queue/QueueCardContentRenderer.vue"
import type { QueueCard } from "@components/game/queue/types"

const props = defineProps<{
  cards: QueueCard[]
  title: string
  subtitle?: string
}>()

defineEmits<{
  (e: "move", payload: { queueId: number; direction: "up" | "down" | "top" | "bottom" }): void
  (e: "run-now", queueId: number): void
  (e: "cancel", queueId: number): void
  (e: "clear-pending", queueIDs: number[]): void
}>()

const { t, locale } = useI18n()

const selectedPendingID = ref<number | null>(null)

const executingCard = computed(() => props.cards.find((card) => card.status === "running") || null)
const pendingCards = computed(() => props.cards.filter((card) => card.status === "pending"))
const runningCount = computed(() => (executingCard.value ? 1 : 0))
const isFishingLane = computed(() => props.cards.some(
  (card) => card.actionId === "fishing.trip" || !!card.display?.fishing
))
const hasOpenEndedFishing = computed(() => props.cards.some(
  (card) =>
    (card.actionId === "fishing.trip" || !!card.display?.fishing) &&
    Number(card.display?.fishing?.plannedDurationSeconds || 0) <= 0
))
const etaAt = computed(() => {
  if (!props.cards.length) return null
  const lastCard = props.cards[props.cards.length - 1]
  if (lastCard.etaUnknown) return null
  return lastCard.startAt + lastCard.remainingMs
})
const etaLabel = computed(() => {
  if (hasOpenEndedFishing.value) return t("game.fishing.queueContinuous")
  if (!etaAt.value) return "—"
  if (isFishingLane.value) {
    return new Date(etaAt.value).toLocaleTimeString(locale.value, { hour: "2-digit", minute: "2-digit" })
  }
  return new Date(etaAt.value).toLocaleString(locale.value)
})
const etaHeading = computed(() => isFishingLane.value ? t("game.fishing.queueReelIn") : t("game.queueEta"))

function canMove(queueID: number, direction: "up" | "down" | "top" | "bottom") {
  const index = pendingCards.value.findIndex((card) => card.id === queueID)
  if (index < 0) return false
  if (direction === "top") return index > 0
  if (direction === "bottom") return index < pendingCards.value.length - 1
  if (direction === "up") return true
  return index < pendingCards.value.length - 1
}

function selectCard(queueID: number) {
  if (!pendingCards.value.some((card) => card.id === queueID)) return
  selectedPendingID.value = queueID
}

</script>

<style scoped>
.queue-shell {
  border-radius: 24px;
  border: 1px solid color-mix(in srgb, var(--app-border) 72%, transparent);
  background:
    linear-gradient(135deg, color-mix(in srgb, #ffffff 92%, transparent), color-mix(in srgb, #eef4ff 58%, transparent));
  box-shadow: 0 20px 45px rgba(17, 24, 39, 0.08);
  padding: 24px;
}

.dark .queue-shell {
  background:
    linear-gradient(135deg, color-mix(in srgb, #1f2937 94%, transparent), color-mix(in srgb, #111827 72%, transparent));
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.28);
}

.queue-shell__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.queue-shell__badge {
  width: 40px;
  height: 40px;
  flex: none;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.2), rgba(168, 85, 247, 0.22));
  font-size: 18px;
}

.queue-shell__badge svg {
  width: 21px;
  height: 21px;
}

.queue-shell--fishing .queue-shell__badge {
  color: #126f70;
  background: color-mix(in srgb, #d9f2ee 78%, var(--app-surface));
}

.queue-shell__meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, auto));
  gap: 10px;
  align-items: stretch;
}

.queue-shell__meta-item {
  /* min-width: 92px; */
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--app-border) 72%, transparent);
  background: color-mix(in srgb, var(--app-surface) 88%, transparent);
  padding: 8px 12px;
}

.queue-shell__meta-label {
  display: block;
  font-size: 12px;
  color: var(--app-muted);
  line-height: 1.2;
}

.queue-shell__meta-value {
  display: block;
  margin-top: 4px;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
}

.queue-shell__body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: min(72vh, 760px);
  overflow-y: auto;
  padding-right: 4px;
}

.queue-shell__running-sticky {
  position: sticky;
  top: -8px;
  z-index: 2;
  padding-bottom: 4px;
  background: color-mix(in srgb, var(--app-surface) 88%, transparent);
}

.queue-shell__empty-panel {
  min-height: 180px;
  border-radius: 20px;
  border: 1px dashed color-mix(in srgb, var(--app-border) 72%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: color-mix(in srgb, var(--app-surface) 92%, transparent);
}

.queue-shell__empty-icon {
  width: 56px;
  height: 56px;
  border-radius: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  background: color-mix(in srgb, var(--app-surface-elevated) 100%, transparent);
}

.queue-shell__empty {
  padding: 18px 0 4px;
  text-align: center;
  font-size: 14px;
  color: var(--app-muted);
}

@media (max-width: 640px) {
  .queue-shell {
    padding: 18px;
    border-radius: 20px;
  }

  /* .queue-shell__head {
    flex-direction: column;
  } */

  .queue-shell__meta {
    width: 100%;
    grid-template-columns: 1fr;
  }

  .queue-shell__meta-item {
    min-width: 0;
  }

  .queue-shell__body {
    max-height: min(68vh, 640px);
  }

  .queue-shell__running-sticky {
    background: color-mix(in srgb, var(--app-surface) 92%, transparent);
  }
}
</style>
