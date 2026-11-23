<template>
  <div class="space-y-3" data-tutorial="combat-tab-area">
    <!-- <div class="text-sm app-muted">{{ t("game.battleAreaSubtitle") }}</div> -->

    <el-card class="app-card app-card--no-border" shadow="never">
      <template #header>
        <div class="flex items-center justify-between gap-2">
          <span class="font-medium">{{ t("game.battleAreaTitle") }}</span>
          <el-button size="small" @click="loadRooms">{{ t("game.battleAreaRefresh") }}</el-button>
        </div>
      </template>

      <el-empty v-if="worlds.length === 0" :description="t('game.battleAreaNoData')" />
      <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <el-card
          v-for="world in worlds"
          :key="world.id"
          shadow="hover"
          class="cursor-pointer transition-all"
          body-class="h-full flex flex-col justify-between gap-3"
          @click="openWorldDetail(world.id)"
        >
          <div class="space-y-2">
            <div class="flex items-center justify-between gap-2">
              <div class="truncate font-medium">{{ world.label }}</div>
              <!-- <el-tag size="small" type="info">{{ world.id }}</el-tag> -->
            </div>
            <div class="text-xs app-muted">
              {{ t("game.battleAreaNormalEnemyCount") }}: {{ world.normalEnemyCountRange.min }}~{{ world.normalEnemyCountRange.max }}
            </div>
            <div class="text-xs app-muted">
              {{ t("game.battleAreaEntryDuration") }}: {{ world.entryDurationSeconds }}s
            </div>
            <div class="text-xs app-muted">
              {{ t("game.battleRoomMaxTeamSizeLabel") }}: {{ world.maxTeamSize }}
            </div>
          </div>

          <div class="flex items-center gap-2">
            <el-button size="small" class="w-full" @click.stop="openWorldDetail(world.id)">
              {{ t("game.detail") }}
            </el-button>
            <el-button
              size="small"
              type="primary"
              class="w-full"
              data-tutorial="combat-create-room-entry"
              @click.stop="openCreateDialog(world.id)"
            >
              {{ t("game.battleCreateRoom") }}
            </el-button>
          </div>
        </el-card>
      </div>
    </el-card>

    <GameDialog v-model="worldDetailVisible" :title="detailWorld?.label || t('game.battleAreaTitle')" width="700px">
      <div v-if="detailWorld" class="space-y-3">
        <p v-if="detailWorld.description" class="whitespace-pre-line text-sm leading-6 app-muted">
          {{ detailWorld.description }}
        </p>

        <div class="grid gap-2 text-xs md:grid-cols-2">
          <div class="rounded border border-[var(--app-border)] px-3 py-2">
            <div class="app-muted">{{ t("game.battleAreaNormalEnemyCount") }}</div>
            <div class="mt-1 font-medium">{{ detailWorld.normalEnemyCountRange.min }}~{{ detailWorld.normalEnemyCountRange.max }}</div>
          </div>
          <div class="rounded border border-[var(--app-border)] px-3 py-2">
            <div class="app-muted">{{ t("game.battleAreaExtraEnemyPerPlayer") }}</div>
            <div class="mt-1 font-medium">{{ detailWorld.extraEnemyPerPlayer.min }}~{{ detailWorld.extraEnemyPerPlayer.max }}</div>
          </div>
          <div class="rounded border border-[var(--app-border)] px-3 py-2">
            <div class="app-muted">{{ t("game.battleAreaEntryDuration") }}</div>
            <div class="mt-1 font-medium">{{ detailWorld.entryDurationSeconds }}s</div>
          </div>
          <div class="rounded border border-[var(--app-border)] px-3 py-2">
            <div class="app-muted">{{ t("game.battleRoomMaxTeamSizeLabel") }}</div>
            <div class="mt-1 font-medium">{{ detailWorld.maxTeamSize }}</div>
          </div>
        </div>

        <div>
          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaEnemyIds") }}</div>
          <div class="flex flex-wrap gap-2">
            <EnemyUnitQuickRef
              v-for="enemyId in detailWorld.enemyIds"
              :key="`enemy-${enemyId}`"
              :enemy-id="enemyId"
              :difficulty-context="detailWorldDifficultyContext"
            />
            <span v-if="detailWorld.enemyIds.length === 0" class="text-xs app-muted">-</span>
          </div>
        </div>

        <div>
          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaSpecialEnemyIds") }}</div>
          <div class="flex flex-wrap gap-2">
            <EnemyUnitQuickRef
              v-for="enemyId in detailWorld.specialEnemyIds"
              :key="`special-${enemyId}`"
              :enemy-id="enemyId"
              :difficulty-context="detailWorldDifficultyContext"
            />
            <span v-if="detailWorld.specialEnemyIds.length === 0" class="text-xs app-muted">-</span>
          </div>
        </div>

	        <div>
	          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaSpecialWaves") }}</div>
	          <div class="flex flex-wrap gap-2">
	            <el-tag v-for="wave in detailWorld.specialWaves" :key="`wave-${wave}`" size="small" type="success">{{ wave }}</el-tag>
	            <el-tag
	              v-if="detailWorld.specialWaveEvery > 0"
	              size="small"
	              type="warning"
	            >
	              {{ t("game.battleAreaSpecialWavePeriodic", { every: detailWorld.specialWaveEvery, min: detailWorld.specialWaveMinCount, max: detailWorld.specialWaveMaxCount }) }}
	            </el-tag>
	            <span v-if="detailWorld.specialWaves.length === 0 && detailWorld.specialWaveEvery <= 0" class="text-xs app-muted">-</span>
	          </div>
	        </div>

        <div v-if="detailWorld.waveSchedules.length > 0">
          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaWaveSchedules") }}</div>
          <div class="space-y-2">
            <div
              v-for="schedule in detailWorld.waveSchedules"
              :key="`wave-schedule-${schedule.id}`"
              class="border-b border-[var(--app-border)] pb-2 text-xs last:border-b-0 last:pb-0"
            >
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium">{{ schedule.label }}</span>
                <span class="app-muted">{{ t("game.battleAreaWaveScheduleInterval", { every: schedule.every }) }}</span>
                <span class="app-muted">{{ t("game.battleAreaWaveScheduleCount", { min: schedule.minCount, max: schedule.maxCount }) }}</span>
              </div>
              <div v-if="schedule.description" class="mt-1 app-muted">{{ schedule.description }}</div>
              <div class="mt-2 flex flex-wrap items-center gap-2">
                <div
                  v-for="choice in schedule.enemyChoices"
                  :key="`wave-schedule-${schedule.id}-${choice.enemyId}`"
                  class="flex items-center gap-1"
                >
                  <EnemyUnitQuickRef :enemy-id="choice.enemyId" :difficulty-context="detailWorldDifficultyContext" />
                  <span class="app-muted">{{ t("game.battleAreaWaveScheduleWeight", { weight: formatWeight(choice.weight) }) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="detailWorld.triggerEncounters.length > 0">
          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaTriggerEncounters") }}</div>
          <div class="space-y-2">
            <div
              v-for="encounter in detailWorld.triggerEncounters"
              :key="`trigger-encounter-${encounter.id}`"
              class="rounded border border-[var(--app-border)] px-3 py-2 text-xs"
            >
              <div class="flex flex-wrap items-center gap-2">
                <span class="app-muted">{{ t("game.triggerItemInfo") }}</span>
                <QuickRef :item-id="encounter.itemId" />
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-2">
                <span class="app-muted">{{ t("game.triggerItemEnemies") }}</span>
                <EnemyUnitQuickRef
                  v-for="enemyId in encounter.enemyIds"
                  :key="`trigger-encounter-${encounter.id}-${enemyId}`"
                  :enemy-id="enemyId"
                  :difficulty-context="detailWorldDifficultyContext"
                />
              </div>
              <div class="mt-2 flex flex-wrap gap-2">
                <el-tag v-if="encounter.minWave !== undefined" size="small" type="warning">
                  {{ t("game.triggerItemMinWave") }}: {{ encounter.minWave }}
                </el-tag>
                <el-tag v-if="encounter.intervalWaves !== undefined" size="small" type="info">
                  {{ t("game.triggerItemIntervalWaves") }}: {{ encounter.intervalWaves }}
                </el-tag>
                <el-tag v-if="encounter.perWaveChance !== undefined" size="small" type="success">
                  {{ t("game.triggerItemChance") }}: {{ Math.round(encounter.perWaveChance * 100) }}%
                </el-tag>
              </div>
            </div>
          </div>
        </div>

        <div>
	          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaCustomLineupConf") }}</div>
	          <div class="space-y-2">
	            <div
	              v-for="lineup in detailWorld.customLineupConf"
	              :key="`lineup-${lineup.id}`"
	              class="rounded border border-[var(--app-border)] px-3 py-2 text-xs"
	            >
	              <div class="flex flex-wrap items-center gap-2">
	                <span class="font-medium">{{ lineup.label }}</span>
	                <el-tag size="small" :type="lineup.isSpecialWaveLineup ? 'warning' : 'info'">
	                  {{ lineup.isSpecialWaveLineup ? t("game.battleAreaCustomLineupSpecialWave") : t("game.battleAreaCustomLineupNormalWave") }}
	                </el-tag>
	                <el-tag size="small" type="success">{{ t("game.battleAreaCustomLineupPercent", { percent: lineup.percent }) }}</el-tag>
	              </div>
	              <div class="mt-2 app-muted">{{ t("game.battleAreaCustomLineupLineup") }}</div>
	              <div class="mt-1 flex flex-wrap gap-2">
	                <EnemyUnitQuickRef
	                  v-for="enemyId in lineup.lineup"
	                  :key="`lineup-unit-${lineup.id}-${enemyId}`"
	                  :enemy-id="enemyId"
	                  :difficulty-context="detailWorldDifficultyContext"
	                />
	                <span v-if="lineup.lineup.length === 0" class="text-xs app-muted">-</span>
	              </div>
	              <div class="mt-2 app-muted">{{ t("game.battleAreaCustomLineupExtraDrops") }}</div>
	              <div class="mt-1 flex flex-wrap gap-2">
	                <el-tag
	                  v-for="(rule, itemId) in lineup.drops.items"
	                  :key="`lineup-drop-${lineup.id}-${itemId}`"
	                  size="small"
	                  type="warning"
	                >
	                  {{ t("game.battleAreaCustomLineupDropRule", { itemId, min: rule.min, max: rule.max, percent: rule.percent }) }}
	                </el-tag>
	                <span v-if="Object.keys(lineup.drops.items).length === 0" class="text-xs app-muted">-</span>
	              </div>
	            </div>
	            <span v-if="detailWorld.customLineupConf.length === 0" class="text-xs app-muted">-</span>
	          </div>
	        </div>

	        <div>
	          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaEntryCosts") }}</div>
          <div class="flex flex-wrap gap-2">
            <el-tag v-for="([itemId, amount]) in detailWorldCosts" :key="`cost-${itemId}`" size="small" type="warning">
              {{ itemId }} x{{ amount }}
            </el-tag>
            <span v-if="detailWorldCosts.length === 0" class="text-xs app-muted">-</span>
          </div>
        </div>

        <div v-if="detailWorldWaveDrops.length > 0">
          <div class="mb-2 text-xs app-muted">{{ t("game.battleAreaWaveDrops") }}</div>
          <div class="flex flex-wrap gap-2">
            <div
              v-for="([itemId, rule]) in detailWorldWaveDrops"
              :key="`wave-drop-${itemId}`"
              class="flex items-center gap-1"
            >
              <QuickRef :item-id="itemId" :count="rule.min === rule.max ? rule.min : undefined" />
              <span class="text-[10px] app-muted">{{ Math.round(rule.percent * 100) }}%</span>
            </div>
          </div>
        </div>
      </div>
    </GameDialog>

    <GameDialog v-model="createDialogVisible" :title="t('game.battleAreaCreateTitle')" width="560px">

          <fieldset class="game-form__section" data-tutorial="combat-create-room-dialog">
            <legend class="game-form__legend">{{ t("game.battleRoomEdit") }}</legend>
      <el-form label-position="top" class="game-form">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <el-form-item :label="t('game.battleRoomName')" data-tutorial="combat-create-room-basic">
            <el-input v-model="createRoomName" />
          </el-form-item>
          <el-form-item :label="t('game.battleAreaSelectLabel')" data-tutorial="combat-create-room-area">
            <el-select v-model="selectedWorldID" class="w-full" filterable :placeholder="t('game.battleAreaSelectPlaceholder')">
              <el-option v-for="world in worlds" :key="world.id" :label="world.label" :value="world.id" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('game.battleDifficultyLevel')" data-tutorial="combat-create-room-difficulty">
            <DifficultySelector v-model="createDifficultyLevel" :world-difficulty-settings="currentWorldDifficultySettings" />
          </el-form-item>
          <el-form-item :label="t('game.battleRoomPosition')" data-tutorial="combat-create-room-position">
            <el-select v-model="createPosition" class="w-full">
              <el-option :label="t('game.battlePositionVanguard')" value="vanguard" />
              <el-option :label="t('game.battlePositionMidguard')" value="midguard" />
              <el-option :label="t('game.battlePositionRearguard')" value="rearguard" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('game.battleRoomVisibility')" data-tutorial="combat-create-room-options">
            <el-switch v-model="createIsPublic" :active-text="t('game.battleRoomPublic')" :inactive-text="t('game.battleRoomPrivate')" />
          </el-form-item>
          <el-form-item :label="t('game.battleRoomAutoReenterOnDefeat')">
            <el-switch
              v-model="createAutoReenterOnDefeat"
              :active-text="t('game.switchOn')"
              :inactive-text="t('game.switchOff')"
            />
          </el-form-item>
        </div>
      </el-form>
      </fieldset>
      <template #footer>
        <el-button @click="createDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" @click="submitCreateRoom">{{ t("game.battleCreateRoom") }}</el-button>
      </template>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import DifficultySelector from "@components/game/combat/DifficultySelector.vue"
import { useCombatViewContext } from "../useCombatViewContext"
import GameDialog from "@components/game/common/GameDialog.vue"

const { t } = useI18n()
const COMBAT_TUTORIAL_OPEN_CREATE_ROOM_EVENT = "combat:tutorial-open-create-room"
const COMBAT_TUTORIAL_CLOSE_DIALOGS_EVENT = "combat:tutorial-close-dialogs"

const {
  selectedWorldID,
  worlds,
  createRoomName,
  createAutoReenterOnDefeat,
  createIsPublic,
  createPosition,
  createDifficultyLevel,
  loadRooms,
  createRoom,
  setActiveTab
} = useCombatViewContext()

const currentWorldDifficultySettings = computed(() => {
  const wid = selectedWorldID.value.trim()
  if (!wid) return null
  const w = worlds.value.find((world) => world.id === wid)
  return w?.difficultySettings || null
})

const worldDetailVisible = ref(false)
const createDialogVisible = ref(false)
const detailWorldID = ref("")

const detailWorld = computed(() => worlds.value.find((world) => world.id === detailWorldID.value) || null)
const detailWorldDifficultyContext = computed(() => {
  const world = detailWorld.value
  if (!world) return null
  return {
    worldId: world.id,
    worldLabel: world.label,
    worldDifficultySettings: world.difficultySettings
  }
})
const detailWorldCosts = computed(() => Object.entries(detailWorld.value?.entryCosts || {}).sort(([a], [b]) => a.localeCompare(b)))
const detailWorldWaveDrops = computed(() => Object.entries(detailWorld.value?.waveDrops || {}).sort(([a], [b]) => a.localeCompare(b)))

function formatWeight(weight: number): string {
  const value = Number(weight)
  if (!Number.isFinite(value)) return "0"
  return value.toFixed(4).replace(/\.?0+$/, "")
}

function openWorldDetail(worldID: string) {
  detailWorldID.value = worldID
  worldDetailVisible.value = true
}

function openCreateDialog(worldID: string) {
  selectedWorldID.value = worldID
  const world = worlds.value.find((item) => item.id === worldID)
  if (!createRoomName.value.trim()) {
    createRoomName.value = world?.label || worldID
  }
  createDialogVisible.value = true
}

function handleCombatTutorialOpenCreateRoom() {
  worldDetailVisible.value = false
  const worldID = selectedWorldID.value || worlds.value[0]?.id || ""
  if (!worldID) return
  openCreateDialog(worldID)
}

function closeCombatTutorialDialogs() {
  worldDetailVisible.value = false
  createDialogVisible.value = false
}

onMounted(() => {
  window.addEventListener(COMBAT_TUTORIAL_OPEN_CREATE_ROOM_EVENT, handleCombatTutorialOpenCreateRoom)
  window.addEventListener(COMBAT_TUTORIAL_CLOSE_DIALOGS_EVENT, closeCombatTutorialDialogs)
})

onBeforeUnmount(() => {
  window.removeEventListener(COMBAT_TUTORIAL_OPEN_CREATE_ROOM_EVENT, handleCombatTutorialOpenCreateRoom)
  window.removeEventListener(COMBAT_TUTORIAL_CLOSE_DIALOGS_EVENT, closeCombatTutorialDialogs)
})

async function submitCreateRoom() {
  const detail = await createRoom()
  if (!detail) return
  createDialogVisible.value = false
  setActiveTab("mine")
}
</script>
