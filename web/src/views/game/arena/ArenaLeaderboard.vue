<template>
  <section class="arena-leaderboard-pane" data-tutorial="arena-leaderboard">
    <div class="arena-leaderboard-toolbar">
      <span><Icon icon="lucide:users-round" />{{ t("game.arena.leaderboardPlayers", { count: formatNumber(entries.length) }) }}</span>
      <el-tooltip :content="t('game.arena.refreshLeaderboard')" placement="top">
        <el-button circle :loading="loading" :aria-label="t('game.arena.refreshLeaderboard')" @click="loadLeaderboard">
          <Icon icon="lucide:refresh-cw" />
        </el-button>
      </el-tooltip>
    </div>

    <div v-if="!loading && !selfEntry" class="arena-leaderboard-self arena-leaderboard-self--empty">
      <span class="arena-leaderboard-self__icon"><Icon icon="lucide:circle-dashed" /></span>
      <div>
        <strong>{{ t("game.arena.leaderboardUnranked") }}</strong>
        <small>{{ t("game.arena.leaderboardEligibility") }}</small>
      </div>
    </div>

    <div v-else-if="selfEntry && !selfInPage" class="arena-leaderboard-self">
      <span class="arena-leaderboard-self__rank">#{{ formatNumber(selfEntry.rank) }}</span>
      <div>
        <strong>{{ t("game.arena.leaderboardMyRank") }}</strong>
        <small>{{ selfEntry.username }}</small>
      </div>
      <div class="arena-leaderboard-self__rating">
        <span>{{ t("game.arena.rating") }}</span>
        <strong>{{ formatNumber(selfEntry.rating) }}</strong>
      </div>
    </div>

    <div v-loading="loading" class="arena-leaderboard-list">
      <el-empty v-if="!loading && entries.length === 0" :description="t('game.arena.leaderboardEmpty')" />
      <article
        v-for="entry in entries"
        :key="entry.userId"
        class="arena-leaderboard-row"
        :class="[
          `arena-leaderboard-row--rank-${Math.min(entry.rank, 4)}`,
          { 'arena-leaderboard-row--self': entry.userId === selfEntry?.userId }
        ]"
      >
        <div class="arena-leaderboard-rank" :aria-label="t('game.arena.leaderboardRank', { rank: entry.rank })">
          <Icon v-if="entry.rank === 1" icon="lucide:crown" />
          <Icon v-else-if="entry.rank <= 3" icon="lucide:medal" />
          <strong>{{ entry.rank }}</strong>
        </div>

        <div class="arena-leaderboard-player">
          <UserName :user-id="entry.userId" :username="entry.username" plain />
          <el-tag v-if="entry.userId === selfEntry?.userId" size="small" effect="plain">{{ t("game.arena.leaderboardMe") }}</el-tag>
          <small v-if="entry.activeDefense">{{ t("game.arena.battleLevelShort", { level: entry.battleLevel }) }}</small>
        </div>

        <div class="arena-leaderboard-rating">
          <span>{{ t("game.arena.rating") }}</span>
          <strong>{{ formatNumber(entry.rating) }}</strong>
        </div>

        <div class="arena-leaderboard-records">
          <div>
            <span>{{ t("game.arena.leaderboardAttack") }}</span>
            <strong>{{ entry.attackWins }} <small>/</small> {{ entry.attackLosses }}</strong>
          </div>
          <div>
            <span>{{ t("game.arena.leaderboardDefense") }}</span>
            <strong>{{ entry.defenseWins }} <small>/</small> {{ entry.defenseLosses }}</strong>
          </div>
        </div>

        <span class="arena-leaderboard-status" :class="{ 'arena-leaderboard-status--active': entry.activeDefense }">
          <Icon :icon="entry.activeDefense ? 'lucide:shield-check' : 'lucide:shield-minus'" />
          {{ entry.activeDefense ? t("game.arena.leaderboardDefending") : t("game.arena.leaderboardNotDefending") }}
        </span>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import UserName from "@components/common/UserName.vue"
import { apiGet } from "@services/api"
import type { ArenaLeaderboardEntry, ArenaLeaderboardPayload } from "@services/arena"
import { resolveErrorMessage } from "@services/error-messages"
import { useGameStore } from "@stores/game"
import { formatNumberWithThousands } from "@utils/number-format"

const props = withDefaults(defineProps<{
  active?: boolean
  refreshVersion?: number
}>(), {
  active: false,
  refreshVersion: 0
})

const { t } = useI18n()
const store = useGameStore()
const entries = ref<ArenaLeaderboardEntry[]>([])
const selfEntry = ref<ArenaLeaderboardEntry | null>(null)
const loading = ref(false)
let loaded = false

const selfInPage = computed(() => Boolean(selfEntry.value && entries.value.some((entry) => entry.userId === selfEntry.value?.userId)))

watch(
  () => [props.active, props.refreshVersion] as const,
  ([active], previous) => {
    if (!active) return
    const refreshChanged = Boolean(previous && props.refreshVersion !== previous[1])
    const becameActive = Boolean(previous && !previous[0])
    if (!loaded || refreshChanged || becameActive) void loadLeaderboard()
  },
  { immediate: true }
)

async function loadLeaderboard() {
  if (!store.token || loading.value) return
  loading.value = true
  try {
    const payload = await apiGet<ArenaLeaderboardPayload>("/api/arena/leaderboard", store.token)
    entries.value = payload.entries
    selfEntry.value = payload.self
    loaded = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.leaderboardFailed"))
  } finally {
    loading.value = false
  }
}

function formatNumber(value: number) {
  return formatNumberWithThousands(Math.trunc(Number(value) || 0))
}
</script>

<style scoped>
.arena-leaderboard-pane {
  display: grid;
  align-content: start;
  gap: 12px;
  min-height: 380px;
}

.arena-leaderboard-toolbar {
  min-height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.arena-leaderboard-toolbar > span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 0.76rem;
}

.arena-leaderboard-self {
  min-height: 54px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border: 1px solid color-mix(in srgb, #2563eb 32%, var(--el-border-color));
  border-left: 3px solid #2563eb;
  border-radius: 5px;
  background: color-mix(in srgb, #2563eb 7%, var(--el-bg-color));
}

.arena-leaderboard-self--empty {
  grid-template-columns: auto minmax(0, 1fr);
  border-color: var(--el-border-color-lighter);
  border-left-color: var(--el-text-color-placeholder);
  background: var(--el-fill-color-lighter);
}

.arena-leaderboard-self__icon,
.arena-leaderboard-self__rank {
  min-width: 36px;
  display: grid;
  place-items: center;
  color: #2563eb;
  font-size: 0.9rem;
  font-weight: 800;
}

.arena-leaderboard-self > div {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.arena-leaderboard-self small,
.arena-leaderboard-self__rating span {
  color: var(--app-muted);
  font-size: 0.7rem;
}

.arena-leaderboard-self__rating {
  text-align: right;
}

.arena-leaderboard-list {
  min-height: 260px;
  border-block: 1px solid var(--el-border-color-lighter);
}

.arena-leaderboard-row {
  min-width: 0;
  min-height: 68px;
  display: grid;
  grid-template-columns: 58px minmax(180px, 1fr) 120px minmax(220px, 0.8fr) 126px;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}

.arena-leaderboard-row:last-child {
  border-bottom: 0;
}

.arena-leaderboard-row--self {
  box-shadow: inset 3px 0 #2563eb;
  background: color-mix(in srgb, #2563eb 5%, var(--el-bg-color));
}

.arena-leaderboard-rank {
  height: 42px;
  display: grid;
  grid-template-columns: 18px 1fr;
  place-items: center;
  gap: 3px;
  border-right: 1px solid var(--el-border-color-lighter);
  color: var(--app-muted);
}

.arena-leaderboard-row--rank-1 .arena-leaderboard-rank {
  color: #b77900;
}

.arena-leaderboard-row--rank-2 .arena-leaderboard-rank {
  color: #687385;
}

.arena-leaderboard-row--rank-3 .arena-leaderboard-rank {
  color: #a85928;
}

.arena-leaderboard-rank strong {
  font-size: 0.98rem;
}

.arena-leaderboard-player {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 7px;
}

.arena-leaderboard-player :deep(button) {
  min-width: 0;
  overflow: hidden;
  font-size: 0.88rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-leaderboard-player small {
  margin-left: auto;
  color: var(--app-muted);
  font-size: 0.69rem;
  white-space: nowrap;
}

.arena-leaderboard-rating,
.arena-leaderboard-records > div {
  display: grid;
  gap: 2px;
}

.arena-leaderboard-rating span,
.arena-leaderboard-records span {
  color: var(--app-muted);
  font-size: 0.68rem;
}

.arena-leaderboard-rating strong {
  color: #2563eb;
  font-size: 1rem;
}

.arena-leaderboard-records {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.arena-leaderboard-records strong {
  font-size: 0.78rem;
}

.arena-leaderboard-records small {
  color: var(--app-muted);
  font-size: 0.68rem;
  font-weight: 500;
}

.arena-leaderboard-status {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 5px;
  color: var(--app-muted);
  font-size: 0.72rem;
  white-space: nowrap;
}

.arena-leaderboard-status--active {
  color: #168447;
}

@media (max-width: 900px) {
  .arena-leaderboard-row {
    grid-template-columns: 50px minmax(0, 1fr) 110px;
  }

  .arena-leaderboard-records {
    grid-column: 2 / 3;
  }

  .arena-leaderboard-status {
    grid-column: 3;
    grid-row: 2;
  }
}

@media (max-width: 600px) {
  .arena-leaderboard-row {
    grid-template-columns: 44px minmax(0, 1fr) auto;
    gap: 8px;
    padding-inline: 8px;
  }

  .arena-leaderboard-rank {
    grid-template-columns: 1fr;
  }

  .arena-leaderboard-rank svg {
    display: none;
  }

  .arena-leaderboard-player small {
    display: none;
  }

  .arena-leaderboard-rating {
    text-align: right;
  }

  .arena-leaderboard-records {
    grid-column: 2 / 4;
  }

  .arena-leaderboard-status {
    display: none;
  }

  .arena-leaderboard-self {
    padding-inline: 8px;
  }
}
</style>
