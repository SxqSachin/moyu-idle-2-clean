<template>
  <div class="page-shell flex flex-col gap-4">
    <GamePageFrame
      title-key="game.menu.homepage"
      tutorial-id="home"
      :visual-src="homeVisual.src"
      :visual-tone="homeVisual.tone"
      visual-key="homepage"
      data-tutorial="home-page"
    >
      <div class="flex flex-col gap-4" data-tutorial="home-queues">
        <ActionQueuePanel
          :cards="playerQueueCards"
          :title="t('game.queueLanePlayer')"
          :subtitle="t('game.homeQueueHint')"
          @move="moveQueueItem"
          @run-now="runQueueItemNow"
          @cancel="cancelQueueItem"
          @clear-pending="clearPendingQueue"
        />

        <ActionQueuePanel
          v-for="lane in kittyLanes"
          :key="`kitty-lane-${lane.actorId}`"
          :cards="lane.cards"
          :title="t('game.queueLaneKitty', { name: lane.name })"
          :subtitle="t('game.homeQueueHint')"
          @move="moveQueueItem"
          @run-now="runQueueItemNow"
          @cancel="cancelQueueItem"
          @clear-pending="clearPendingQueue"
        />
      </div>
    </GamePageFrame>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { getGameDataChunk, getGameDataTable } from "@services/game-data-runtime"
import { send } from "@services/ws"
import { parseIconId, resolveFallbackIconByIconId } from "@services/item-icons"
import { useRealtimeStore } from "@stores/realtime"
import { usePlayerStore } from "@stores/player"
import ActionQueuePanel from "@components/game/queue/ActionQueuePanel.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"
import { localizedFishingText, type FishingSpot } from "@services/fishing"
import type { QueueCard, QueueCardDisplayFishing } from "@components/game/queue/types"

type ActionDef = {
  id: string
  nameKey?: string
  iconId?: string
  durationS: number
}

const { t, locale } = useI18n()
const homeVisual = gamePageVisuals.home
const realtimeStore = useRealtimeStore()
const playerStore = usePlayerStore()
const actionsByID = ref<Record<string, ActionDef>>({})
const fishingSpotsByID = new Map(
  getGameDataTable<FishingSpot>("fishing_spots").map((spot) => [spot.id, spot])
)
const kittyStatusByID = computed(() => {
  const map = new Map<number, string>()
  for (const kitty of playerStore.kitties || []) {
    const kittyID = Math.max(1, Math.floor(Number(kitty.id || 0)))
    if (!Number.isFinite(kittyID) || kittyID <= 0) continue
    map.set(kittyID, String(kitty.status || "").toLowerCase().trim())
  }
  return map
})

const playerQueueCards = computed<QueueCard[]>(() => {
  const items = realtimeStore.actionQueue.filter((item) => normalizeActorType(item.actorType) === "player")
  return buildQueueCardsForLane(items)
})

const kittyLanes = computed<Array<{ actorId: number; name: string; cards: QueueCard[] }>>(() => {
  const byActor = new Map<number, typeof realtimeStore.actionQueue>()
  for (const item of realtimeStore.actionQueue) {
    if (normalizeActorType(item.actorType) !== "kitty") continue
    const actorId = Math.max(1, Math.floor(Number(item.actorId || 0)))
    if (!byActor.has(actorId)) {
      byActor.set(actorId, [])
    }
    byActor.get(actorId)!.push(item)
  }
  const lanes: Array<{ actorId: number; name: string; cards: QueueCard[] }> = []
  const seen = new Set<number>()
  for (const kitty of playerStore.kitties || []) {
    const actorId = Math.max(1, Math.floor(Number(kitty.id || 0)))
    if (!Number.isFinite(actorId) || actorId <= 0 || seen.has(actorId)) continue
    seen.add(actorId)
    lanes.push({
      actorId,
      name: kitty.name,
      cards: buildQueueCardsForLane(byActor.get(actorId) || [])
    })
  }
  for (const [actorId, items] of byActor.entries()) {
    if (seen.has(actorId)) continue
    lanes.push({
      actorId,
      name: `#${actorId}`,
      cards: buildQueueCardsForLane(items)
    })
  }
  lanes.sort((a, b) => a.actorId - b.actorId)
  return lanes
})

function buildQueueCardsForLane(items: typeof realtimeStore.actionQueue): QueueCard[] {
  const snapshotNowMs = Date.now() - (realtimeStore.serverClockSkewMs ?? 0)
  let cursorMs = snapshotNowMs
  let cursorKnown = true
  return items
    .filter((item) => item.status === "pending" || item.status === "running")
    .map((item) => {
      const normalizedActorType = normalizeActorType(item.actorType)
      const normalizedActorID = Math.max(0, Math.floor(Number(item.actorId || 0)))
      const action = actionsByID.value[item.actionId]
      const fishing = item.display?.fishing
      const displayDurationMs = Number(item.display?.effectiveDurationMs || 0)
      const plannedFishingDurationMs = Math.max(0, Number(fishing?.plannedDurationSeconds || 0) * 1000)
      const durationMs = Math.max(
        1000,
        plannedFishingDurationMs || displayDurationMs || (action?.durationS || 1) * 1000
      )
      const remainingCount = Math.max(0, item.totalCount - item.doneCount)
      let remainingMs = 0
      let startAt = cursorKnown ? cursorMs : 0
      let progressPct = 0
      let runStartAtMs = 0
      let runEndsAtMs = 0
      let etaUnknown = !cursorKnown
      let progressToken = `${item.id}-${item.status}-${item.doneCount}`
      if (item.status === "running" && fishing) {
        const phaseEndsAtMs = Math.max(
          0,
          Number(fishing.phaseEndsAtMs || 0),
          item.runEndsAt ? new Date(item.runEndsAt).getTime() : 0
        )
        runStartAtMs = Number(fishing.startedAtMs || 0) ||
          (item.runStartAt ? new Date(item.runStartAt).getTime() : 0)
        startAt = snapshotNowMs
        if (Number(fishing.plannedEndAtMs || 0) > 0) {
          const plannedEndAtMs = Number(fishing.plannedEndAtMs)
          runEndsAtMs = snapshotNowMs >= plannedEndAtMs
            ? Math.max(plannedEndAtMs, phaseEndsAtMs)
            : plannedEndAtMs
          if (runStartAtMs <= 0 || runStartAtMs >= runEndsAtMs) {
            runStartAtMs = runEndsAtMs - Math.max(1000, plannedFishingDurationMs)
          }
          const totalMs = Math.max(1000, runEndsAtMs - runStartAtMs)
          remainingMs = Math.max(0, runEndsAtMs - snapshotNowMs)
          progressPct = Math.max(0, Math.min(100, ((snapshotNowMs - runStartAtMs) / totalMs) * 100))
          cursorMs = snapshotNowMs + remainingMs
        } else {
          runEndsAtMs = phaseEndsAtMs || snapshotNowMs
          remainingMs = 0
          progressPct = 0
          cursorKnown = false
        }
        progressToken = `${item.id}-${item.status}-${fishing.phase}-${runStartAtMs}-${runEndsAtMs}`
      } else if (item.status === "running") {
        runEndsAtMs = item.runEndsAt ? new Date(item.runEndsAt).getTime() : snapshotNowMs
        runStartAtMs = item.runStartAt ? new Date(item.runStartAt).getTime() : runEndsAtMs - durationMs
        if (runStartAtMs >= runEndsAtMs) {
          runStartAtMs = runEndsAtMs - durationMs
        }
        const runWindowDurationMs = Math.max(1000, runEndsAtMs - runStartAtMs)
        const currentRunRemainingMs = Math.max(0, runEndsAtMs - snapshotNowMs)
        const currentRunElapsedMs = Math.max(0, snapshotNowMs - runStartAtMs)
        remainingMs = currentRunRemainingMs + Math.max(0, remainingCount - 1) * runWindowDurationMs
        startAt = snapshotNowMs
        progressPct = Math.max(0, Math.min(100, (currentRunElapsedMs / runWindowDurationMs) * 100))
        progressToken = `${item.id}-${item.status}-${item.doneCount}-${runStartAtMs}-${runEndsAtMs}`
        cursorMs = startAt + remainingMs
      } else {
        remainingMs = fishing && fishing.plannedDurationSeconds === 0 ? 0 : remainingCount * durationMs
        if (fishing && fishing.plannedDurationSeconds === 0) {
          cursorKnown = false
        } else if (!etaUnknown) {
          cursorMs = startAt + remainingMs
        }
      }
      return {
        id: item.id,
        actorType: normalizedActorType,
        actorId: normalizedActorID,
        actionId: item.actionId,
        label: actionLabel(item.actionId, item.display?.building?.buildingId, fishing, item.status),
        iconType: actionIcon(item.actionId).type,
        iconId: actionIcon(item.actionId).id,
        iconFallback: actionIconFallback(item.actionId),
        status: item.status,
        statusLabelKey:
          normalizedActorType === "kitty" &&
          item.status !== "done" &&
          item.status !== "failed" &&
          kittyStatusByID.value.get(normalizedActorID) === "in_combat"
            ? "game.queueStatus.in_combat"
            : undefined,
        totalCount: item.totalCount,
        doneCount: item.doneCount,
        remainingMs,
        startAt,
        progressPct,
        runStartAtMs,
        runEndsAtMs,
        progressToken,
        etaUnknown,
        display: item.display
      }
    })
}

onMounted(() => {
  void loadCatalog()
})

async function loadCatalog() {
  const data = getCatalogActions(getGameDataChunk("actions"))
  const map: Record<string, ActionDef> = {}
  for (const item of data) {
    map[item.id] = item
  }
  actionsByID.value = map
}

function getCatalogActions(catalog: unknown): ActionDef[] {
  if (!catalog || typeof catalog !== "object") return []
  const actions = (catalog as { actions?: unknown }).actions
  if (!Array.isArray(actions)) return []
  return actions
    .filter((item): item is ActionDef => !!item && typeof item === "object" && typeof (item as ActionDef).id === "string")
    .map((item) => ({
      id: item.id,
      nameKey: typeof item.nameKey === "string" ? item.nameKey : undefined,
      iconId: typeof item.iconId === "string" ? item.iconId : undefined,
      durationS: typeof item.durationS === "number" ? item.durationS : 1
    }))
}

function actionLabel(
  actionID: string,
  buildingID?: string,
  fishing?: QueueCardDisplayFishing,
  status?: string
): string {
  if (actionID === "fishing.trip" || fishing) {
    const spot = fishing ? fishingSpotsByID.get(fishing.spotId) : undefined
    const spotName = localizedFishingText(spot?.names, locale.value) || t("game.fishing.spot")
    return status === "running"
      ? t("game.fishing.queueTitleRunning", { spot: spotName })
      : t("game.fishing.queueTitlePending", { spot: spotName })
  }
  if (actionID === "building_upgrade" && buildingID) {
    const buildingNameKey = `building.${buildingID}.name`
    const buildingName = t(buildingNameKey)
    if (buildingName !== buildingNameKey) {
      return buildingName
    }
    return buildingID
  }
  const action = actionsByID.value[actionID]
  const alchemyLabel = alchemyActionLabel(actionID)
  if (alchemyLabel) return alchemyLabel
  if (!action?.nameKey) {
    const fallbackKey = `game.queueAction_${actionID}`
    const fallback = t(fallbackKey)
    return fallback === fallbackKey ? actionID : fallback
  }
  const localized = t(action.nameKey)
  return localized === action.nameKey ? actionID : localized
}

function alchemyActionLabel(actionID: string): string {
  switch (actionID) {
    case "alchemy.convert":
      return t("alchemy.convert.title")
    case "alchemy.bench":
      return t("alchemy.bench.title")
    case "alchemy.inherit":
      return t("alchemy.inherit.title")
    case "alchemy.transmute":
      return t("alchemy.transmute.title")
    case "alchemy.smelt":
      return t("alchemy.smelt.title")
    default:
      return ""
  }
}

function actionIcon(actionID: string): { type: string; id: string } {
  const action = actionsByID.value[actionID]
  const parsed = parseIconId(action?.iconId || `action.${actionID}`)
  return {
    type: parsed?.type || "action",
    id: parsed?.id || actionID
  }
}

function actionIconFallback(actionID: string): string {
  const action = actionsByID.value[actionID]
  if (action?.iconId) return resolveFallbackIconByIconId(action.iconId, "⚙️")
  switch (actionID) {
    case "alchemy.convert":
      return "⚗️"
    case "alchemy.bench":
      return "🧪"
    case "alchemy.inherit":
      return "🧬"
    case "alchemy.transmute":
      return "💰"
    case "alchemy.smelt":
      return "🔥"
    default:
      return "⚙️"
  }
}

function moveQueueItem(payload: { queueId: number; direction: "up" | "down" | "top" | "bottom" }) {
  send("action.queue.move", { queueId: payload.queueId, moveTarget: payload.direction })
}

function runQueueItemNow(queueID: number) {
  send("action.queue.run_now", { queueId: queueID })
}

function cancelQueueItem(queueID: number) {
  send("action.cancel", { queueId: queueID })
}

function clearPendingQueue(queueIDs: number[]) {
  for (const queueID of queueIDs) {
    cancelQueueItem(queueID)
  }
}

function normalizeActorType(raw: unknown): "player" | "kitty" {
  return String(raw || "").toLowerCase().trim() === "kitty" ? "kitty" : "player"
}
</script>
