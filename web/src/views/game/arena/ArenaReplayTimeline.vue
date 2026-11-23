<template>
  <section class="replay-turn-rail" :aria-label="t('game.arena.replayTurns')">
    <header class="replay-turn-rail__header">
      <div>
        <span><Icon icon="lucide:list-tree" /></span>
        <div>
          <strong>{{ t("game.arena.replayTurns") }}</strong>
          <small>{{ t("game.arena.replayTurnCount", { count: turns.length }) }}</small>
        </div>
      </div>
      <el-segmented v-model="filter" size="small" :options="filterOptions" />
      <div class="replay-turn-rail__scrubber">
        <span>0</span>
        <el-slider
          :model-value="currentTurn"
          :min="0"
          :max="Math.max(1, turns.length)"
          :step="1"
          :show-tooltip="false"
          :disabled="turns.length <= 0"
          :aria-label="t('game.arena.replayTurnScrubber')"
          @input="handleScrub"
          @change="handleScrub"
        />
        <strong>{{ currentTurn }} / {{ turns.length }}</strong>
      </div>
    </header>

    <div v-if="visibleTurns.length > 0" ref="turnListRef" class="replay-turn-rail__list">
      <button
        v-for="turn in visibleTurns"
        :key="turn.index"
        :data-turn="turn.index"
        type="button"
        class="replay-turn-entry"
        :class="[
          `replay-turn-entry--${turn.side || 'neutral'}`,
          { 'replay-turn-entry--active': turn.index === currentTurn }
        ]"
        :aria-current="turn.index === currentTurn ? 'step' : undefined"
        @click="emit('select', turn.index)"
      >
        <span class="replay-turn-entry__index">{{ String(turn.index).padStart(2, "0") }}</span>
        <span class="replay-turn-entry__main">
          <span class="replay-turn-entry__actor">
            <Icon :icon="turn.side === 'enemy' ? 'lucide:shield' : 'lucide:swords'" />
            <strong>{{ actorLabel(turn.actorName) }}</strong>
          </span>
          <small>{{ skillLabel(turn) }}</small>
        </span>
        <span class="replay-turn-entry__metrics">
          <span v-if="turn.damage > 0" class="replay-turn-entry__damage"><Icon icon="lucide:burst" />{{ formatNumber(turn.damage) }}</span>
          <span v-if="turn.healing > 0" class="replay-turn-entry__healing"><Icon icon="lucide:heart-pulse" />{{ formatNumber(turn.healing) }}</span>
          <span v-if="turn.knockouts > 0" class="replay-turn-entry__ko"><Icon icon="lucide:circle-x" />{{ turn.knockouts }}</span>
          <span v-else-if="turn.criticalHits > 0" class="replay-turn-entry__critical"><Icon icon="lucide:crosshair" />{{ turn.criticalHits }}</span>
        </span>
        <span v-if="turn.keyMoment" class="replay-turn-entry__key" :title="t('game.arena.replayKeyMoment')">
          <Icon icon="lucide:bookmark" />
        </span>
      </button>
    </div>

    <div v-else class="replay-turn-rail__empty">
      <Icon icon="lucide:scan-search" />
      <span>{{ t(filter === "key" ? "game.arena.replayNoKeyTurns" : "game.arena.replayNoTurns") }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { getLocalizedSkillName } from "@services/skill-definition"
import { formatNumberWithThousands } from "@utils/number-format"
import type { ArenaReplayTurnSummary } from "@services/arena-replay-analysis"

const props = defineProps<{
  turns: ArenaReplayTurnSummary[]
  currentTurn: number
}>()

const emit = defineEmits<{
  (event: "select", turn: number): void
  (event: "scrub", turn: number): void
}>()

const { t } = useI18n()
const filter = ref<"all" | "key">("all")
const turnListRef = ref<HTMLElement | null>(null)
const filterOptions = computed(() => [
  { label: t("game.arena.replayAllTurns"), value: "all" },
  { label: t("game.arena.replayKeyTurns"), value: "key" }
])
const visibleTurns = computed(() => filter.value === "key"
  ? props.turns.filter((turn) => turn.keyMoment)
  : props.turns)

watch(
  () => props.currentTurn,
  async (turn) => {
    if (turn <= 0) return
    await nextTick()
    const node = turnListRef.value?.querySelector<HTMLElement>(`[data-turn="${turn}"]`)
    node?.scrollIntoView({ block: "nearest", inline: "nearest" })
  }
)

function skillLabel(turn: ArenaReplayTurnSummary): string {
  if (!turn.skillId && !turn.skillNameFallback) return t("game.arena.replayNoAction")
  return getLocalizedSkillName(t, turn.skillId, turn.skillNameFallback || turn.skillId)
}

function actorLabel(raw: string): string {
  const name = String(raw || "").trim()
  if (!name) return t("game.arena.unknownUnit")
  const translated = t(name)
  return translated && translated !== name ? translated : name
}

function formatNumber(value: number): string {
  return formatNumberWithThousands(Math.round(Number(value) || 0))
}

function handleScrub(raw: number | number[]) {
  const value = Array.isArray(raw) ? Number(raw[0] || 0) : Number(raw || 0)
  emit("scrub", value)
}
</script>

<style scoped>
.replay-turn-rail {
  min-width: 0;
  height: min(650px, calc(100vh - 310px));
  min-height: 430px;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: var(--el-bg-color);
}

.replay-turn-rail__header {
  display: grid;
  gap: 9px;
  padding: 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: color-mix(in srgb, var(--el-fill-color-light) 68%, var(--el-bg-color));
}

.replay-turn-rail__header > div:first-child {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.replay-turn-rail__header > div:first-child > span {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  flex: none;
  border-radius: 4px;
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 10%, transparent);
}

.replay-turn-rail__header > div:first-child > div {
  min-width: 0;
  display: grid;
  gap: 1px;
}

.replay-turn-rail__header strong {
  font-size: 0.78rem;
}

.replay-turn-rail__header small {
  color: var(--app-muted);
  font-size: 0.64rem;
}

.replay-turn-rail__header :deep(.el-segmented) {
  width: 100%;
}

.replay-turn-rail__scrubber {
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(70px, 1fr) auto;
  align-items: center;
  gap: 8px;
}

.replay-turn-rail__scrubber > span,
.replay-turn-rail__scrubber > strong {
  color: var(--app-muted);
  font-size: 0.61rem;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  white-space: nowrap;
}

.replay-turn-rail__list {
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.replay-turn-entry {
  position: relative;
  box-sizing: border-box;
  width: 100%;
  min-height: 64px;
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) auto;
  align-items: center;
  gap: 7px;
  padding: 8px 9px;
  border: 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
  color: var(--el-text-color-primary);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 0.14s ease, box-shadow 0.14s ease;
}

.replay-turn-entry:hover {
  background: var(--el-fill-color-light);
}

.replay-turn-entry:focus-visible {
  z-index: 1;
  outline: 2px solid color-mix(in srgb, var(--el-color-primary) 70%, transparent);
  outline-offset: -2px;
}

.replay-turn-entry--active {
  background: color-mix(in srgb, var(--el-color-primary) 8%, var(--el-bg-color));
  box-shadow: inset 3px 0 var(--el-color-primary);
}

.replay-turn-entry--enemy.replay-turn-entry--active {
  background: color-mix(in srgb, var(--el-color-warning) 9%, var(--el-bg-color));
  box-shadow: inset 3px 0 var(--el-color-warning);
}

.replay-turn-entry__index {
  color: var(--app-muted);
  font-size: 0.66rem;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
}

.replay-turn-entry__main {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.replay-turn-entry__actor {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 5px;
}

.replay-turn-entry__actor svg {
  flex: none;
  color: var(--el-color-primary);
  font-size: 0.72rem;
}

.replay-turn-entry--enemy .replay-turn-entry__actor svg {
  color: var(--el-color-warning-dark-2);
}

.replay-turn-entry__actor strong,
.replay-turn-entry__main small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.replay-turn-entry__actor strong {
  font-size: 0.72rem;
}

.replay-turn-entry__main small {
  color: var(--app-muted);
  font-size: 0.62rem;
}

.replay-turn-entry__metrics {
  display: grid;
  justify-items: end;
  gap: 2px;
  font-size: 0.61rem;
  font-variant-numeric: tabular-nums;
}

.replay-turn-entry__metrics > span {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.replay-turn-entry__damage,
.replay-turn-entry__ko {
  color: var(--el-color-danger);
}

.replay-turn-entry__healing {
  color: var(--el-color-success);
}

.replay-turn-entry__critical {
  color: var(--el-color-warning-dark-2);
}

.replay-turn-entry__key {
  position: absolute;
  top: 4px;
  right: 4px;
  color: var(--el-color-warning-dark-2);
  font-size: 0.62rem;
}

.replay-turn-rail__empty {
  min-height: 180px;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 7px;
  color: var(--app-muted);
  font-size: 0.7rem;
}

.replay-turn-rail__empty svg {
  font-size: 1.1rem;
}

@media (max-width: 1180px) {
  .replay-turn-rail {
    height: auto;
    min-height: 0;
    grid-template-rows: auto auto;
  }

  .replay-turn-rail__header {
    grid-template-columns: minmax(0, 1fr) minmax(180px, 0.65fr);
    align-items: center;
  }

  .replay-turn-rail__scrubber {
    grid-column: 1 / -1;
  }

  .replay-turn-rail__list {
    display: flex;
    overflow-x: auto;
    overflow-y: hidden;
  }

  .replay-turn-entry {
    width: 218px;
    min-width: 218px;
    border-right: 1px solid var(--el-border-color-extra-light);
    border-bottom: 0;
  }

  .replay-turn-entry--active,
  .replay-turn-entry--enemy.replay-turn-entry--active {
    box-shadow: inset 0 -3px var(--el-color-primary);
  }
}

@media (max-width: 560px) {
  .replay-turn-rail__header {
    grid-template-columns: minmax(0, 1fr);
  }

  .replay-turn-entry {
    width: 196px;
    min-width: 196px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .replay-turn-entry {
    transition: none;
  }
}
</style>
