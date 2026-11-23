<template>
  <div class="mine-view space-y-3" data-tutorial="combat-tab-mine">
    <el-empty v-if="!myRoomDetail" :description="t('game.battleMyRoomEmpty')">
      <el-button type="primary" @click="setActiveTab('area')">
        {{ t("game.battleAreaTab") }}
      </el-button>
    </el-empty>

    <div v-else class="space-y-3">
      <div>
        <div class="flex flex-col gap-2 md:flex-row flex-wrap items-center md:justify-between mb-2">
          <div class="flex flex-row flex-wrap items-center gap-2">
            <div class="font-medium">{{ myRoomDetail.room.roomName }}</div>
            <el-tag size="small">{{ roomStatusLabel(myRoomDetail.room.status) }}</el-tag>
            <el-tag v-if="myRoomDetail.room.status === 'playing'" size="small" type="warning">
              {{ t("game.battleCurrentWave") }}: {{ myRoomWave }}
            </el-tag>
          </div>

          <div class="flex items-center gap-2">
            <el-button v-if="isMyRoomOwner" size="small" :disabled="!canEditRoomConfig" @click="openRoomEditDialog">
              {{ t("game.battleEditRoomInfo") }}
            </el-button>
            <el-button size="small" type="primary" @click="goToBattleAndSelectMyRoom">
              {{ t("game.battleViewBattle") }}
            </el-button>
          </div>
        </div>

        <!-- Room info panel: left = facts, right = badges + fog cost -->
        <div class="mine-room-info">
          <!-- Left: 3 key facts — dungeon / difficulty / max team size -->
          <div class="mine-room-info__fields">
            <span class="mine-room-info__label">{{ t("game.battleRoomDungeonName") }}</span>
            <span class="mine-room-info__value">{{ currentWorldLabel }}</span>

            <span class="mine-room-info__label">{{ t("game.battleDifficultyLevel") }}</span>
            <span class="mine-room-info__value">{{ difficultyLevelLabel(myRoomDetail.room.difficultyLevel) }}</span>

            <span class="mine-room-info__label">{{ t("game.battleRoomMaxTeamSizeLabel") }}</span>
            <span class="mine-room-info__value">{{ currentWorldMaxTeamSize }}</span>
          </div>

          <!-- Right: status tags + feature toggles (only active) + fog cost -->
          <div class="mine-room-info__aside">
            <div class="mine-room-info__badges">
              <!-- Visibility always shown -->
              <el-tag
                size="small"
                :type="myRoomDetail.room.isPublic ? 'primary' : 'info'"
                :disable-transitions="true"
              >
                {{ myRoomDetail.room.isPublic ? t("game.battleRoomPublic") : t("game.battleRoomPrivate") }}
              </el-tag>
              <!-- Optional features — only shown when active -->
              <el-tag v-if="myRoomDetail.room.autoReenterOnDefeat" size="small" type="success" :disable-transitions="true">
                {{ t("game.battleRoomAutoReenterOnDefeat") }}
              </el-tag>
              <el-tag v-if="myRoomDetail.room.disableEntryRound" size="small" type="warning" :disable-transitions="true">
                {{ t("game.battleRoomDisableEntryRound") }}
              </el-tag>
              <el-tag v-if="myRoomDetail.room.isFogMode" size="small" type="info" :disable-transitions="true">
                🌫️ {{ t("game.fogMode") }}
              </el-tag>
            </div>

            <!-- Fog entry cost — only when fog mode on -->
            <div v-if="myRoomDetail.room.isFogMode && currentFogMapItemId" class="mine-room-info__fog-cost">
              <span class="mine-room-info__label">{{ t("game.fogMapItemCost") }}</span>
              <QuickRefWithRequire
                v-if="isMyRoomOwner"
                :item-id="currentFogMapItemId"
                :required-count="1"
              />
              <QuickRef
                v-else
                :item-id="currentFogMapItemId"
              />
            </div>
          </div>
        </div>
      </div>

      <div>
        <div class="mb-2 flex items-center justify-between gap-2">
          <div class="text-xs">{{ t("game.battleRoomMembersTitle") }}</div>
          <div class="text-xs app-muted">{{ battleUnitCount }} / {{ currentWorldMaxTeamSize }}</div>
        </div>
        <div class="grid gap-2 sm:grid-cols-2">
          <template v-for="slot in roomMemberSlots" :key="slot.key">
            <BattleRoomMemberCard
              v-if="slot.member"
              :member="slot.member"
              :my-user-id="store.userId"
              :is-owner="isMyRoomOwner"
              :room-status="myRoomDetail.room.status"
              :position-label="positionLabel"
              @update-position="updateUnitPosition"
              @kick="kickMember"
              @remove-kitty="removeKittyFromRoom"
            >
              <el-tag
                v-if="slot.member.unitType === 'player' && slot.member.userId === myRoomDetail.room.ownerId"
                size="small"
                type="success"
                class="absolute top-2 right-2"
              >
                {{ t("game.battleRoomOwnerTag") }}
              </el-tag>
            </BattleRoomMemberCard>
            <button
              v-else
              type="button"
              class="flex min-h-24 w-full flex-col items-center justify-center rounded border border-dashed border-[var(--app-border)] bg-[var(--app-surface)] px-3 py-4 text-xs text-[var(--app-text)] transition hover:border-[var(--el-color-primary)] hover:bg-[var(--el-color-primary-light-9)] disabled:cursor-not-allowed disabled:opacity-70"
              :disabled="!canEditRoomConfig"
              @click="openKittyPickerDialog"
            >
              <span class="text-lg leading-none">+</span>
              <span class="mt-1">{{ t("game.battleRoomEmptySlot") }}</span>
              <span class="mt-1 app-muted">{{ t("game.battleRoomEmptySlotHint") }}</span>
            </button>
          </template>
        </div>
      </div>

	      <div class="mine-room-actions flex flex-wrap justify-start">
	        <el-button type="danger" :disabled="myRoomDetail.room.status === 'recovering'" @click="leaveRoom(myRoomDetail.room.id)">
	          {{ t("game.battleLeaveRoom") }}
	        </el-button>
        <el-button type="warning" @click="endBattle(myRoomDetail.room.id)">
          {{ t("game.battleEndRoom") }}
        </el-button>
	        <el-button type="primary" v-if="myRoomMember" @click="toggleReady(myRoomDetail.room.id)">
	          {{ myRoomMember.isReady ? t("game.battleCancelReady") : t("game.battleReady") }}
	        </el-button>
	      </div>
	      <div v-if="myRoomRecoveringProgress" class="mt-2 space-y-1">
	        <div class="text-xs app-muted">
	          {{ t("game.battleProgressRecovering") }} · {{ t("game.battleProgressRemaining", { time: myRoomRecoveringProgress.remainingLabel }) }}
	        </div>
	        <el-progress :percentage="myRoomRecoveringProgress.percentage" :stroke-width="10" :show-text="false" />
	      </div>
	      <div v-if="myRoomDetail.room.status === 'recovering'" class="mt-2 text-xs app-muted">
	        {{ t("game.battleRecoveringLeaveBlocked") }}
	        <span v-if="myRoomRecoveryEndsAtLabel">（{{ t("game.battleRecoveringUntil") }}：{{ myRoomRecoveryEndsAtLabel }}）</span>
	      </div>
	    </div>

    <GameDialog v-model="roomEditVisible" :title="t('game.battleEditRoomInfo')" width="560px">
      <div v-if="myRoomDetail">
        <el-form label-position="top" class="game-form">
          <fieldset class="game-form__section">
            <legend class="game-form__legend">{{ t("game.battleRoomEdit") }}</legend>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <el-form-item :label="t('game.battleRoomName')">
              <el-input v-model="myRoomDetail.room.roomName" />
            </el-form-item>
            <el-form-item :label="t('game.battleRoomDungeon')">
              <el-select v-model="myRoomDetail.room.dungeonId" class="w-full" filterable>
                <el-option v-for="world in worlds" :key="`room-world-${world.id}`" :label="world.label" :value="world.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('game.battleDifficultyLevel')">
              <DifficultySelector v-model="myRoomDetail.room.difficultyLevel" :world-difficulty-settings="editWorldDifficultySettings" />
            </el-form-item>
            <el-form-item :label="t('game.battleRoomVisibility')">
              <el-switch
                v-model="myRoomDetail.room.isPublic"
                :active-text="t('game.battleRoomPublic')"
                :inactive-text="t('game.battleRoomPrivate')"
              />
            </el-form-item>
            <el-form-item :label="t('game.battleRoomAutoReenterOnDefeat')">
              <el-switch
                v-model="myRoomDetail.room.autoReenterOnDefeat"
                :active-text="t('game.switchOn')"
                :inactive-text="t('game.switchOff')"
              />
            </el-form-item>
            <el-form-item :label="t('game.battleRoomDisableEntryRound')">
              <el-switch
                v-model="myRoomDetail.room.disableEntryRound"
                :active-text="t('game.switchOn')"
                :inactive-text="t('game.switchOff')"
              />
            </el-form-item>
            <el-form-item :label="t('game.fogMode')">
              <el-switch
                v-model="myRoomDetail.room.isFogMode"
                :active-text="t('game.switchOn')"
                :inactive-text="t('game.switchOff')"
              />
            </el-form-item>
          </div>
          </fieldset>

          <fieldset class="game-form__section">
            <legend class="game-form__legend">{{ t("game.battleTransferOwner") }}</legend>
            <el-form-item>
              <el-select v-model="transferTargetUserID" class="w-full" clearable>
                <el-option
                  v-for="member in transferCandidates"
                  :key="`transfer-${member.userId}`"
                  :label="transferCandidateLabel(member)"
                  :value="member.userId"
                />
              </el-select>
            </el-form-item>
          </fieldset>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="roomEditVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="warning" :disabled="!transferTargetUserID || !canEditRoomConfig" @click="submitTransferOwner">{{ t("game.battleTransferOwner") }}</el-button>
        <el-button type="primary" :disabled="!canEditRoomConfig" @click="submitRoomConfig">{{ t("game.battleSaveRoomConfig") }}</el-button>
      </template>
    </GameDialog>

    <BattleRoomKittyPickerDialog
      v-model="kittyPickerVisible"
      :title="t('game.battleRoomKittyPickerTitle')"
      :kitties="kittyPickerItems"
      :position-options="positionOptions"
      :position-label="positionLabel"
      @add-kitty="addKittyFromPicker"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { ElMessageBox } from "element-plus"
import BattleRoomMemberCard from "@components/game/combat/BattleRoomMemberCard.vue"
import BattleRoomKittyPickerDialog from "@components/game/combat/BattleRoomKittyPickerDialog.vue"
import { useGameStore } from "@stores/game"
import { usePlayerStore } from "@stores/player"
import DifficultySelector from "@components/game/combat/DifficultySelector.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import { useCombatViewContext } from "../useCombatViewContext"
import GameDialog from "@components/game/common/GameDialog.vue"

const { t } = useI18n()
const store = useGameStore()
const playerStore = usePlayerStore()
const {
  worlds,
  myRoomDetail,
  selectedRoom,
  selectedRun,
  runState,
  isMyRoomOwner,
  myRoomMember,
  roomStatusLabel,
  positionLabel,
  toggleReady,
  updateRoomPosition,
  addRoomKitty,
  removeRoomKitty,
  kickRoomMember,
  transferRoomOwner,
  leaveRoom,
  endBattle,
  applyRoomConfig,
  selectMyRoom,
  setActiveTab,
  resolveWorldMaxTeamSize
} = useCombatViewContext()

const roomEditVisible = ref(false)
const kittyPickerVisible = ref(false)
const transferTargetUserID = ref<number | null>(null)
const nowMs = ref(Date.now())
const kittyPositions = ref<Record<number, string>>({})
const positionOptions = ["vanguard", "midguard", "rearguard"]
const currentWave = computed(() => Number(runState.value?.wave || selectedRun.value?.wave || 1))
const myRoomWave = computed(() => {
  if (!myRoomDetail.value) return 1
  if (selectedRoom.value?.room.id === myRoomDetail.value.room.id) {
    return currentWave.value
  }
  const completedRuns = Number(myRoomDetail.value.room.completedRuns || 0)
  return Math.max(1, completedRuns + 1)
})
const transferCandidates = computed(() => {
  if (!myRoomDetail.value) return []
  return myRoomDetail.value.members.filter((member) => member.userId !== store.userId)
})
const canEditRoomConfig = computed(() => {
  const status = String(myRoomDetail.value?.room.status || "")
  return status === "idle" || status === "preparing"
})
const currentWorldMaxTeamSize = computed(() => {
  const dungeonID = String(myRoomDetail.value?.room.dungeonId || "").trim()
  return resolveWorldMaxTeamSize(dungeonID)
})
const currentWorldLabel = computed(() => {
  const dungeonID = String(myRoomDetail.value?.room.dungeonId || "").trim()
  return worlds.value.find((w) => w.id === dungeonID)?.label || dungeonID
})
function difficultyLevelLabel(level: number): string {
  if (level === 0) return t("game.battleDifficultyNone")
  if (level < 1 || level > 10) return t("game.battleDifficultyNormal")
  return t(`game.difficulty.level${level}`)
}
const battleUnitCount = computed(() => myRoomDetail.value?.units?.length || myRoomDetail.value?.members.length || 0)
const currentFogMapItemId = computed(() => {
  if (!myRoomDetail.value?.room.isFogMode) return ""
  const dungeonId = myRoomDetail.value.room.dungeonId
  return worlds.value.find((w) => w.id === dungeonId)?.fogMapItemId || ""
})
const editWorldDifficultySettings = computed(() => {
  const dungeonId = myRoomDetail.value?.room.dungeonId
  if (!dungeonId) return null
  const w = worlds.value.find((world) => world.id === dungeonId)
  return w?.difficultySettings || null
})
const roomDisplayUnits = computed(() => {
  if (!myRoomDetail.value) return []
  const units = Array.isArray(myRoomDetail.value.units) ? myRoomDetail.value.units : []
  if (units.length > 0) {
    return units.map((unit) => ({
      key: `${unit.unitType}:${unit.ownerUserId}:${unit.unitId}`,
      userId: Number(unit.ownerUserId || 0),
      ownerUserId: Number(unit.ownerUserId || 0),
      username: String(unit.username || ""),
      unitType: unit.unitType,
      kittyName: String(unit.kittyName || ""),
      kittyId: Number(unit.kittyId || 0),
      position: String(unit.position || "vanguard"),
      positionLockedByPersonalityId: String(unit.positionLockedByPersonalityId || ""),
      positionLockedByPersonalityKey: String(unit.positionLockedByPersonalityKey || ""),
      isReady: Boolean(unit.isReady)
    }))
  }
  return myRoomDetail.value.members.map((member) => ({
    key: `player:${member.userId}`,
    userId: member.userId,
    ownerUserId: member.userId,
    username: member.username,
    unitType: "player" as const,
    kittyName: "",
    kittyId: 0,
    position: member.position,
    isReady: member.isReady
  }))
})
const roomMemberSlots = computed(() => {
  const maxTeamSize = Math.max(1, currentWorldMaxTeamSize.value)
  const occupied = roomDisplayUnits.value.slice(0, maxTeamSize)
  return Array.from({ length: maxTeamSize }, (_, index) => ({
    key: `room-slot-${index}`,
    member: occupied[index] || null
  }))
})
const myKittyUnitIDs = computed(() => new Set((myRoomDetail.value?.units || [])
  .filter((unit) => unit.ownerUserId === store.userId && unit.unitType === "kitty" && Number(unit.kittyId || 0) > 0)
  .map((unit) => Number(unit.kittyId || 0))))
const kittyPickerItems = computed(() => {
  return (playerStore.kitties || [])
    .map((kitty) => {
      const kittyID = Number(kitty.id || 0)
      const status = String(kitty.status || "idle")
      return {
        id: kittyID,
        name: String(kitty.name || `#${kittyID}`),
        level: Number(kitty.level || 1),
        status,
        statusLabel: kittyStatusLabel(status),
        inRoom: isKittyInRoom(kittyID),
        disabledReason: kittyDisabledReason(kittyID, status),
        defaultPosition: kittyPositions.value[kittyID] || "vanguard"
      }
    })
    .filter((kitty) => kitty.id > 0)
})
const myRoomRecoveryEndsAtLabel = computed(() => {
  const raw = myRoomDetail.value?.room.recoveryEndsAt
  if (!raw) return ""
  const value = new Date(raw)
  if (Number.isNaN(value.getTime())) return ""
  return value.toLocaleTimeString()
})
const myRoomRecoveringProgress = computed(() => {
  const room = myRoomDetail.value?.room
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
let progressTicker: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  progressTicker = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})

onBeforeUnmount(() => {
  if (progressTicker) {
    clearInterval(progressTicker)
    progressTicker = null
  }
})

function transferCandidateLabel(member: { userId: number; username?: string }) {
  const username = String(member.username || "").trim()
  if (username) return username
  return `${t("game.playerLabel")} ${member.userId}`
}

function updateMyPosition(roomID: number, position: string) {
  if (!roomID) return
  void updateRoomPosition(position, roomID)
}

function isKittyInRoom(kittyID: number) {
  return myKittyUnitIDs.value.has(kittyID)
}

function kittyStatusLabel(status: string) {
  if (status === "in_combat") return t("game.battleRoomKittyStatusInCombat")
  if (status === "in_life_queue") return t("game.battleRoomKittyStatusInLifeQueue")
  return t("game.battleRoomKittyStatusIdle")
}

function kittyDisabledReason(kittyID: number, status: string) {
  if (!canEditRoomConfig.value) return "房间进入战斗流程后不能调整猫咪编队"
  if (isKittyInRoom(kittyID)) return t("game.battleRoomKittyInRoomTag")
  if (battleUnitCount.value >= currentWorldMaxTeamSize.value) return "房间战斗单位已满"
  if (status === "in_combat") return "该猫咪已在战斗中"
  return ""
}

async function addKittyToRoom(kittyID: number, status: string, position?: string) {
  if (!myRoomDetail.value?.room.id || kittyID <= 0) return
  if (status === "in_life_queue") {
    try {
      await ElMessageBox.confirm("加入战斗将暂停该猫咪当前生活队列任务，离开战斗后会继续处理。", "确认加入战斗", {
        confirmButtonText: "确认",
        cancelButtonText: t("game.cancel"),
        type: "warning"
      })
    } catch {
      return
    }
  }
  const nextPosition = String(position || kittyPositions.value[kittyID] || "vanguard")
  kittyPositions.value = {
    ...kittyPositions.value,
    [kittyID]: nextPosition
  }
  const detail = await addRoomKitty(kittyID, nextPosition, myRoomDetail.value.room.id)
  if (detail) {
    kittyPickerVisible.value = false
  }
}

function removeKittyFromRoom(kittyID: number) {
  if (!myRoomDetail.value?.room.id || kittyID <= 0) return
  void removeRoomKitty(kittyID, myRoomDetail.value.room.id)
}

function goToBattleAndSelectMyRoom() {
  if (!myRoomDetail.value?.room.id) return
  selectMyRoom()
  setActiveTab("battle")
}

function kickMember(userID: number) {
  if (!myRoomDetail.value?.room.id || userID <= 0) return
  void kickRoomMember(userID, myRoomDetail.value.room.id)
}

function updateUnitPosition(payload: { position: string; unitType: "player" | "kitty"; kittyId?: number }) {
  if (!myRoomDetail.value?.room.id) return
  if (payload.unitType === "kitty") {
    const kittyID = Number(payload.kittyId || 0)
    if (kittyID <= 0) return
    kittyPositions.value = {
      ...kittyPositions.value,
      [kittyID]: payload.position
    }
    void addRoomKitty(kittyID, payload.position, myRoomDetail.value.room.id)
    return
  }
  updateMyPosition(myRoomDetail.value.room.id, payload.position)
}

function openKittyPickerDialog() {
  kittyPickerVisible.value = true
}

function addKittyFromPicker(payload: { kittyId: number; status: string; position: string }) {
  void addKittyToRoom(payload.kittyId, payload.status, payload.position)
}

function openRoomEditDialog() {
  if (!canEditRoomConfig.value) return
  transferTargetUserID.value = null
  roomEditVisible.value = true
}

function submitRoomConfig() {
  if (!myRoomDetail.value?.room.id || !canEditRoomConfig.value) return
  void applyRoomConfig(myRoomDetail.value.room.id)
  roomEditVisible.value = false
}

function submitTransferOwner() {
  if (!myRoomDetail.value?.room.id || !transferTargetUserID.value || !canEditRoomConfig.value) return
  void transferRoomOwner(transferTargetUserID.value, myRoomDetail.value.room.id)
  roomEditVisible.value = false
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
</script>

<style scoped>
/* Room info panel — left/right split */
.mine-room-info {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface) 94%, var(--app-bg));
  color: var(--app-text);
  padding: 10px 14px;
  display: flex;
  gap: 16px;
  font-size: 13px;
}

/* Left: attribute grid — label auto-sizes, value follows immediately */
.mine-room-info__fields {
  display: grid;
  grid-template-columns: auto auto;
  column-gap: 10px;
  row-gap: 5px;
  align-content: start;
  flex: 0 0 auto;
}

.mine-room-info__label {
  color: var(--app-muted);
  white-space: nowrap;
}

.mine-room-info__value {
  font-weight: 500;
  white-space: nowrap;
}

/* Right: badges + fog cost, stacked vertically */
.mine-room-info__aside {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-end;
  justify-content: flex-start;
}

.mine-room-info__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  justify-content: flex-end;
}

.mine-room-info__fog-cost {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

@media (max-width: 767px) {
  .mine-view {
    padding-bottom: calc(74px + env(safe-area-inset-bottom));
  }

  .mine-room-info {
    flex-direction: column;
    gap: 10px;
    padding: 10px;
  }

  .mine-room-info__fields {
    grid-template-columns: minmax(0, auto) minmax(0, 1fr);
    width: 100%;
  }

  .mine-room-info__value {
    min-width: 0;
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .mine-room-info__aside {
    align-items: flex-start;
  }

  .mine-room-info__badges {
    justify-content: flex-start;
  }

  .mine-room-actions {
    position: sticky;
    bottom: 0;
    z-index: 5;
    gap: 8px;
    margin: 8px -10px calc(-10px - env(safe-area-inset-bottom));
    padding: 10px 10px calc(10px + env(safe-area-inset-bottom));
    border-top: 1px solid var(--app-border);
    background:
      linear-gradient(
        180deg,
        color-mix(in srgb, var(--app-surface) 84%, transparent),
        var(--app-surface) 28%
      );
    box-shadow: 0 -8px 22px color-mix(in srgb, #000 10%, transparent);
    backdrop-filter: blur(8px);
  }

  .mine-room-actions:deep(.el-button) {
    flex: 1 1 104px;
    min-width: 0;
    margin-left: 0;
  }

  .mine-room-actions:deep(.el-button + .el-button) {
    margin-left: 0;
  }
}
</style>
